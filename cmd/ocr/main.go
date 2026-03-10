package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/pinnertw/query/internal/db/generated"
	"github.com/pinnertw/query/internal/ocr"
)

func main() {
	ollamaURL := flag.String("ollama", "http://127.0.0.1:11434", "Ollama API base URL")
	model := flag.String("model", "glm-ocr-gpu", "Model name for OCR")
	normalizeModel := flag.String("normalize-model", "qwen3.5:27b", "Model name for normalization (text-only)")
	normalizeURL := flag.String("normalize-url", "http://127.0.0.1:8090", "OpenAI-compatible API base URL for normalization (llama.cpp)")
	ocrTimeout := flag.Duration("ocr-timeout", 30*time.Second, "Timeout per OCR image (skip image on timeout)")
	ocrWorkers := flag.Int("ocr-workers", 4, "OCR concurrency (1 = sequential, safer; >1 = parallel)")
	normWorkers := flag.Int("norm-workers", 0, "Normalization mode: 0 = combined (default), N>0 = per-photo with N parallel workers")
	maxPhotos := flag.Int("max-photos", 0, "Max photos to OCR (0 = all)")
	maxDim := flag.Int("max-dim", 1600, "Resize images so longest side is at most this many pixels (0 = no resize)")
	dedup := flag.Bool("dedup", true, "Skip near-duplicate photos using perceptual hashing")
	dbURL := flag.String("db", "", "PostgreSQL connection string (or set DATABASE_URL env var)")
	dryRun := flag.Bool("dry-run", false, "Print extracted menu without writing to database")
	flag.Parse()

	if flag.NArg() < 1 {
		log.Fatal("Usage: ocr [flags] <google_place_id>")
	}
	googlePlaceID := flag.Arg(0)

	connStr := *dbURL
	if connStr == "" {
		connStr = os.Getenv("DATABASE_URL")
	}
	if !*dryRun && connStr == "" {
		log.Fatal("--db or DATABASE_URL is required (or use --dry-run)")
	}

	// Find menu photos for this place
	photosDir := filepath.Join("menu_photos", googlePlaceID)
	files, err := ocr.FindImages(photosDir)
	if err != nil || len(files) == 0 {
		log.Fatalf("No menu photos found in %s", photosDir)
	}
	fmt.Printf("Found %d menu photos for %s\n", len(files), googlePlaceID)

	// Deduplicate similar photos
	if *dedup {
		t0dedup := time.Now()
		files = ocr.DeduplicateImages(files)
		fmt.Printf("Dedup: %d unique photos (%s)\n", len(files), time.Since(t0dedup).Round(time.Millisecond))
	}

	// Limit photos
	if *maxPhotos > 0 && len(files) > *maxPhotos {
		files = files[:*maxPhotos]
	}

	// Pass 1: OCR all images
	totalStart := time.Now()
	type ocrResult struct {
		text string
		dur  time.Duration
		err  error
	}
	ocrResults := make([]ocrResult, len(files))
	var ocrWG sync.WaitGroup
	ocrSem := make(chan struct{}, *ocrWorkers)
	for i, f := range files {
		ocrWG.Add(1)
		go func(idx int, file string) {
			defer ocrWG.Done()
			ocrSem <- struct{}{}
			defer func() { <-ocrSem }()
			t0 := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), *ocrTimeout)
			defer cancel()
			text, err := ocr.OcrImage(*ollamaURL, *model, file, *maxDim, ctx)
			ocrResults[idx] = ocrResult{text: text, dur: time.Since(t0), err: err}
		}(i, f)
	}
	ocrWG.Wait()
	ocrWall := time.Since(totalStart)

	// Print OCR results in order, collect per-photo texts
	var photoTexts []string
	for i, r := range ocrResults {
		name := filepath.Base(files[i])
		if r.err != nil {
			log.Printf("[%d/%d] OCR: %s — Failed (%s): %v", i+1, len(files), name, r.dur.Round(time.Millisecond), r.err)
		} else {
			fmt.Printf("[%d/%d] OCR: %s — Done (%s, %d chars)\n", i+1, len(files), name, r.dur.Round(time.Millisecond), len(r.text))
			photoTexts = append(photoTexts, r.text)
		}
	}
	fmt.Printf("OCR: %d/%d photos in %s (wall)\n", len(photoTexts), len(files), ocrWall.Round(time.Millisecond))

	if len(photoTexts) == 0 {
		log.Fatal("No text extracted from any photos")
	}

	// Print raw OCR text
	var allOCRText strings.Builder
	for i, text := range photoTexts {
		allOCRText.WriteString(fmt.Sprintf("--- Photo %d ---\n", i+1))
		allOCRText.WriteString(text)
		allOCRText.WriteString("\n\n")
	}
	fmt.Printf("\n=== Raw OCR text (%d chars) ===\n%s\n", allOCRText.Len(), allOCRText.String())

	// Pass 2: Normalize into structured JSON
	normURL := *ollamaURL
	if *normalizeURL != "" {
		normURL = *normalizeURL
	}
	useOpenAI := *normalizeURL != ""

	normStart := time.Now()
	var menu *ocr.MenuData

	if *normWorkers <= 0 {
		fmt.Println("=== Normalizing combined text ===")
		combined := allOCRText.String()
		var err error
		menu, err = ocr.NormalizeMenu(normURL, *normalizeModel, combined, useOpenAI)
		if err != nil {
			log.Fatalf("Normalization failed (%s): %v", time.Since(normStart).Round(time.Millisecond), err)
		}
	} else {
		fmt.Printf("=== Normalizing %d photos (%d workers) ===\n", len(photoTexts), *normWorkers)
		type normResult struct {
			menu *ocr.MenuData
			dur  time.Duration
			err  error
		}
		normResults := make([]normResult, len(photoTexts))
		sem := make(chan struct{}, *normWorkers)
		var normWG sync.WaitGroup
		for i, text := range photoTexts {
			normWG.Add(1)
			go func(idx int, txt string) {
				defer normWG.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				t0 := time.Now()
				m, err := ocr.NormalizeMenu(normURL, *normalizeModel, txt, useOpenAI)
				normResults[idx] = normResult{menu: m, dur: time.Since(t0), err: err}
			}(i, text)
		}
		normWG.Wait()

		var menus []*ocr.MenuData
		for i, r := range normResults {
			if r.err != nil {
				log.Printf("  Photo %d norm failed (%s): %v", i+1, r.dur.Round(time.Millisecond), r.err)
			} else {
				items := 0
				for _, c := range r.menu.Categories {
					items += len(c.Items)
				}
				fmt.Printf("  Photo %d normalized (%s, %d items)\n", i+1, r.dur.Round(time.Millisecond), items)
				menus = append(menus, r.menu)
			}
		}

		if len(menus) == 0 {
			log.Fatal("No photos normalized successfully")
		}
		menu = ocr.MergeMenus(menus)
	}
	normWall := time.Since(normStart)
	fmt.Printf("Normalization done (%s)\n", normWall.Round(time.Millisecond))

	// Print results
	totalItems := 0
	for _, cat := range menu.Categories {
		fmt.Printf("\n[%s]\n", cat.Name)
		for _, item := range cat.Items {
			if len(item.PriceTiers) > 0 {
				tierStrs := make([]string, len(item.PriceTiers))
				for i, t := range item.PriceTiers {
					tierStrs[i] = fmt.Sprintf("%s:%d元", t.Label, t.Price)
				}
				fmt.Printf("  %s — %s\n", item.Name, strings.Join(tierStrs, " / "))
			} else {
				fmt.Printf("  %s — %d元\n", item.Name, item.Price)
			}
			totalItems++
		}
	}

	// Print items with option groups
	for _, cat := range menu.Categories {
		for _, item := range cat.Items {
			if len(item.OptionGroups) > 0 {
				for _, og := range item.OptionGroups {
					fmt.Printf("  [%s] %s (choose %d-%d):\n", item.Name, og.Name, og.MinChoices, og.MaxChoices)
					for _, o := range og.Options {
						adj := ""
						if o.PriceAdjustment != 0 {
							adj = fmt.Sprintf(" (+%d)", o.PriceAdjustment)
						}
						fmt.Printf("    - %s%s\n", o.Name, adj)
					}
				}
			}
		}
	}

	fmt.Printf("\nTotal: %d categories, %d items\n", len(menu.Categories), totalItems)

	if *dryRun {
		fmt.Printf("\n(dry-run mode, not writing to database) (total: %s)\n", time.Since(totalStart).Round(time.Millisecond))
		return
	}

	// Write to database — look up restaurant by google_place_id
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer conn.Close(ctx)

	q := db.New(conn)

	// Find the restaurant by google_place_id
	rest, err := q.GetRestaurantByGooglePlaceID(ctx, pgtype.Text{String: googlePlaceID, Valid: true})
	if err != nil {
		// Fallback: look up via places + restaurant_details for backward compatibility
		place, pErr := q.GetPlaceByGoogleID(ctx, googlePlaceID)
		if pErr != nil {
			log.Fatalf("Restaurant with google_place_id %s not found: %v", googlePlaceID, err)
		}
		rd, rdErr := q.UpsertRestaurantDetails(ctx, place.ID)
		if rdErr != nil {
			log.Fatalf("Failed to upsert restaurant details: %v", rdErr)
		}
		// Use the restaurant_details ID (which should match restaurants.id for migrated data)
		t1 := time.Now()
		if err := ocr.InsertMenu(ctx, q, rd.ID, menu); err != nil {
			log.Fatalf("Failed to insert menu: %v", err)
		}
		fmt.Printf("\nMenu saved to database! (insert: %s, total: %s)\n",
			time.Since(t1).Round(time.Millisecond),
			time.Since(totalStart).Round(time.Millisecond))
		return
	}

	t1 := time.Now()
	if err := ocr.InsertMenu(ctx, q, rest.ID, menu); err != nil {
		log.Fatalf("Failed to insert menu: %v", err)
	}
	fmt.Printf("\nMenu saved to database! (insert: %s, total: %s)\n",
		time.Since(t1).Round(time.Millisecond),
		time.Since(totalStart).Round(time.Millisecond))
}
