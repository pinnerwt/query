package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/image/draw"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/pinnertw/query/internal/db/generated"
)

const ocrPrompt = "OCR this menu image. Extract all text exactly as shown."

const normalizePrompt = `You are given raw OCR text from a restaurant menu photo. Parse it into structured JSON.

Output ONLY valid JSON with this exact schema, no other text:
{
  "categories": [
    {
      "name": "category name",
      "items": [
        {
          "name": "item name",
          "price": 100,
          "description": "optional description",
          "price_tiers": [
            {"label": "2入", "quantity": 2, "price": 688},
            {"label": "6入", "quantity": 6, "price": 1680}
          ]
        }
      ]
    }
  ],
  "combos": [
    {
      "name": "combo name",
      "price": 198,
      "description": "what is included",
      "groups": [
        {
          "name": "group name",
          "min_choices": 1,
          "max_choices": 1,
          "options": [
            {"name": "option name", "price_adjustment": 0}
          ]
        }
      ]
    }
  ]
}

Rules:
- Use Chinese (Traditional) for all item names and category names. If the menu has both Chinese and English/Japanese names for the same item, use the Chinese name only.
- Ignore items that only appear in English, Japanese, or Korean without a Chinese equivalent.
- price is in TWD (New Taiwan Dollars), as an integer (no decimals)
- ONLY include items where you are confident the price matches the item. If a price seems misaligned or uncertain, skip that item rather than guess wrong.
- If an item has no price at all, set price to -1 (unknown/not shown)
- If the menu explicitly says "時價" (market price) for an item, set price to -2
- If there are no clear categories, use "其他" as the category name
- Merge duplicate items (same name) keeping the first occurrence
- description is optional, omit or set to "" if none
- Do NOT include any text outside the JSON object
- If an item has multiple prices for different quantities (e.g. "Two/NT$688, Six/NT$1,680"), use price_tiers array. Set item price to the lowest tier price.
- If an item has only one price, omit price_tiers (do NOT create a single-entry price_tiers array).
- If the menu has set meals/combos with chooseable options (e.g. "choose a soup", "pick a main"), add them to the combos array with groups and options.
- combos is optional — omit if no set meals are detected.
- price_adjustment in combo options is the extra cost on top of the combo base price (0 if no upcharge).

Raw OCR text:
`

type ollamaRequest struct {
	Model    string          `json:"model"`
	Stream   bool            `json:"stream"`
	Messages []ollamaMessage `json:"messages"`
	Format   string          `json:"format,omitempty"`
}

type ollamaMessage struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"`
}

type ollamaResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type menuData struct {
	Categories []menuCategory `json:"categories"`
	Combos     []menuCombo    `json:"combos,omitempty"`
}

type menuCategory struct {
	Name  string     `json:"name"`
	Items []menuItem `json:"items"`
}

type menuItem struct {
	Name        string      `json:"name"`
	Price       int         `json:"price"`
	Description string      `json:"description,omitempty"`
	PriceTiers  []priceTier `json:"price_tiers,omitempty"`
}

type priceTier struct {
	Label    string `json:"label"`
	Quantity int    `json:"quantity"`
	Price    int    `json:"price"`
}

type menuCombo struct {
	Name        string       `json:"name"`
	Price       int          `json:"price"`
	Description string       `json:"description,omitempty"`
	Groups      []comboGroup `json:"groups,omitempty"`
}

type comboGroup struct {
	Name       string        `json:"name"`
	MinChoices int           `json:"min_choices"`
	MaxChoices int           `json:"max_choices"`
	Options    []comboOption `json:"options"`
}

type comboOption struct {
	Name            string `json:"name"`
	PriceAdjustment int    `json:"price_adjustment"`
}

func main() {
	ollamaURL := flag.String("ollama", "http://127.0.0.1:11434", "Ollama API base URL")
	model := flag.String("model", "glm-ocr-gpu", "Model name for OCR")
	normalizeModel := flag.String("normalize-model", "qwen3.5:9b", "Model name for normalization (text-only)")
	normalizeURL := flag.String("normalize-url", "", "OpenAI-compatible API base URL for normalization (if different from --ollama)")
	ocrTimeout := flag.Duration("ocr-timeout", 30*time.Second, "Timeout per OCR image (skip image on timeout)")
	ocrWorkers := flag.Int("ocr-workers", 1, "OCR concurrency (1 = sequential, safer; >1 = parallel)")
	normWorkers := flag.Int("norm-workers", 0, "Normalization mode: 0 = combined (default), N>0 = per-photo with N parallel workers")
	maxPhotos := flag.Int("max-photos", 0, "Max photos to OCR (0 = all)")
	maxDim := flag.Int("max-dim", 800, "Resize images so longest side is at most this many pixels (0 = no resize)")
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
	files, err := findImages(photosDir)
	if err != nil || len(files) == 0 {
		log.Fatalf("No menu photos found in %s", photosDir)
	}
	fmt.Printf("Found %d menu photos for %s\n", len(files), googlePlaceID)

	// Deduplicate similar photos
	if *dedup {
		t0dedup := time.Now()
		files = deduplicateImages(files)
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
			text, err := ocrImage(*ollamaURL, *model, file, *maxDim, *ocrTimeout)
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
	var menu *menuData

	if *normWorkers <= 0 {
		// Combined mode: concatenate all texts, one normalization call
		fmt.Println("=== Normalizing combined text ===")
		combined := allOCRText.String()
		var err error
		menu, err = normalizeMenu(normURL, *normalizeModel, combined, useOpenAI)
		if err != nil {
			log.Fatalf("Normalization failed (%s): %v", time.Since(normStart).Round(time.Millisecond), err)
		}
	} else {
		// Per-photo mode: normalize each photo independently with N workers
		fmt.Printf("=== Normalizing %d photos (%d workers) ===\n", len(photoTexts), *normWorkers)
		type normResult struct {
			menu *menuData
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
				m, err := normalizeMenu(normURL, *normalizeModel, txt, useOpenAI)
				normResults[idx] = normResult{menu: m, dur: time.Since(t0), err: err}
			}(i, text)
		}
		normWG.Wait()

		var menus []*menuData
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
		menu = mergeMenus(menus)
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

	if len(menu.Combos) > 0 {
		fmt.Printf("\n=== Combos ===\n")
		for _, combo := range menu.Combos {
			fmt.Printf("\n[%s] %d元 — %s\n", combo.Name, combo.Price, combo.Description)
			for _, g := range combo.Groups {
				fmt.Printf("  %s (choose %d-%d):\n", g.Name, g.MinChoices, g.MaxChoices)
				for _, o := range g.Options {
					adj := ""
					if o.PriceAdjustment != 0 {
						adj = fmt.Sprintf(" (+%d)", o.PriceAdjustment)
					}
					fmt.Printf("    - %s%s\n", o.Name, adj)
				}
			}
		}
	}

	fmt.Printf("\nTotal: %d categories, %d items\n", len(menu.Categories), totalItems)

	if *dryRun {
		fmt.Printf("\n(dry-run mode, not writing to database) (total: %s)\n", time.Since(totalStart).Round(time.Millisecond))
		return
	}

	// Write to database
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer conn.Close(ctx)

	q := db.New(conn)
	t1 := time.Now()
	if err := insertMenu(ctx, q, googlePlaceID, menu); err != nil {
		log.Fatalf("Failed to insert menu: %v", err)
	}
	fmt.Printf("\nMenu saved to database! (insert: %s, total: %s)\n",
		time.Since(t1).Round(time.Millisecond),
		time.Since(totalStart).Round(time.Millisecond))
}

// fixInvalidEscapes removes backslashes before characters that are not valid
// JSON escape sequences. Some models produce \é or \開 inside strings.
func fixInvalidEscapes(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inString := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '"' && (i == 0 || s[i-1] != '\\') {
			inString = !inString
		}
		if inString && ch == '\\' && i+1 < len(s) {
			next := s[i+1]
			// Valid JSON escapes: " \ / b f n r t u
			switch next {
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't', 'u':
				b.WriteByte(ch)
			default:
				// Skip the invalid backslash
				continue
			}
		} else {
			b.WriteByte(ch)
		}
	}
	return b.String()
}

// mergeMenus combines per-photo menuData results into a single menu.
// Categories with the same name are merged, duplicate items are deduped by name.
func mergeMenus(menus []*menuData) *menuData {
	type catEntry struct {
		cat      menuCategory
		itemSeen map[string]bool
	}

	catMap := make(map[string]int) // name -> index in cats
	var cats []catEntry

	for _, m := range menus {
		for _, cat := range m.Categories {
			idx, exists := catMap[cat.Name]
			if !exists {
				idx = len(cats)
				catMap[cat.Name] = idx
				cats = append(cats, catEntry{
					cat:      menuCategory{Name: cat.Name},
					itemSeen: make(map[string]bool),
				})
			}
			for _, item := range cat.Items {
				if !cats[idx].itemSeen[item.Name] {
					cats[idx].itemSeen[item.Name] = true
					cats[idx].cat.Items = append(cats[idx].cat.Items, item)
				}
			}
		}
	}

	result := &menuData{}
	for _, e := range cats {
		result.Categories = append(result.Categories, e.cat)
	}

	comboSeen := make(map[string]bool)
	for _, m := range menus {
		for _, combo := range m.Combos {
			if !comboSeen[combo.Name] {
				comboSeen[combo.Name] = true
				result.Combos = append(result.Combos, combo)
			}
		}
	}

	return result
}

func findImages(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		name := strings.ToLower(e.Name())
		if strings.HasSuffix(name, ".jpg") || strings.HasSuffix(name, ".jpeg") || strings.HasSuffix(name, ".png") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	return files, nil
}

func ocrImage(baseURL, model, imagePath string, maxDim int, timeout time.Duration) (string, error) {
	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}

	if maxDim > 0 {
		resized, err := resizeImage(imgData, maxDim)
		if err != nil {
			// Fall back to original if resize fails
			log.Printf("  Warning: resize failed, using original: %v", err)
		} else {
			imgData = resized
		}
	}

	imgB64 := base64.StdEncoding.EncodeToString(imgData)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return ollamaChatCtx(ctx, baseURL, model, ocrPrompt, []string{imgB64}, "")
}

func normalizeMenu(baseURL, model, rawText string, useOpenAI bool) (*menuData, error) {
	prompt := normalizePrompt + rawText

	var result string
	var err error
	if useOpenAI {
		result, err = openaiChat(baseURL, model, prompt)
	} else {
		result, err = ollamaChat(baseURL, model, prompt, nil, "")
	}
	if err != nil {
		return nil, err
	}

	// Strip markdown code fences if present
	result = strings.TrimSpace(result)
	if strings.HasPrefix(result, "```") {
		lines := strings.Split(result, "\n")
		// Remove first line (```json) and last line (```)
		if len(lines) > 2 {
			result = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	// Fix invalid JSON escape sequences (e.g. \é, \開) produced by some models
	result = fixInvalidEscapes(result)

	var menu menuData
	if err := json.Unmarshal([]byte(result), &menu); err != nil {
		return nil, fmt.Errorf("parse menu JSON: %w\nraw response:\n%s", err, result)
	}

	return &menu, nil
}

func ollamaChat(baseURL, model, prompt string, images []string, format string) (string, error) {
	return ollamaChatCtx(context.Background(), baseURL, model, prompt, images, format)
}

func ollamaChatCtx(ctx context.Context, baseURL, model, prompt string, images []string, format string) (string, error) {
	msg := ollamaMessage{
		Role:    "user",
		Content: prompt,
		Images:  images,
	}

	reqBody := ollamaRequest{
		Model:    model,
		Stream:   false,
		Messages: []ollamaMessage{msg},
		Format:   format,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/api/chat", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama returned %d: %s", resp.StatusCode, string(body))
	}

	var ollamaResp ollamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	return ollamaResp.Message.Content, nil
}

// resizeImage downscales a JPEG/PNG so the longest side is at most maxDim pixels.
// Returns the re-encoded JPEG bytes. If the image is already small enough, returns original.
func resizeImage(data []byte, maxDim int) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	// Already small enough
	if w <= maxDim && h <= maxDim {
		return data, nil
	}

	// Calculate new dimensions preserving aspect ratio
	var newW, newH int
	if w > h {
		newW = maxDim
		newH = int(math.Round(float64(h) * float64(maxDim) / float64(w)))
	} else {
		newH = maxDim
		newW = int(math.Round(float64(w) * float64(maxDim) / float64(h)))
	}

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.BiLinear.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 85}); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}

	return buf.Bytes(), nil
}

// deduplicateImages removes near-duplicate photos using a simple average hash.
// It downscales each image to 8x8 grayscale, computes a 64-bit hash, and skips
// any image whose hamming distance to a previously seen hash is <= 10.
func deduplicateImages(files []string) []string {
	type hashEntry struct {
		hash uint64
		file string
	}

	var seen []hashEntry
	var result []string

	for _, f := range files {
		h, err := imageHash(f)
		if err != nil {
			// Can't hash — keep it
			result = append(result, f)
			continue
		}

		isDup := false
		for _, s := range seen {
			if hammingDistance(h, s.hash) <= 10 {
				isDup = true
				break
			}
		}

		if !isDup {
			seen = append(seen, hashEntry{hash: h, file: f})
			result = append(result, f)
		}
	}

	return result
}

// imageHash computes a simple average hash: resize to 8x8 grayscale,
// set bits where pixel > mean.
func imageHash(path string) (uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return 0, err
	}

	// Resize to 8x8
	dst := image.NewGray(image.Rect(0, 0, 8, 8))
	draw.BiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)

	// Compute mean
	var sum float64
	for _, p := range dst.Pix {
		sum += float64(p)
	}
	mean := sum / 64.0

	// Build hash
	var hash uint64
	for i, p := range dst.Pix {
		if float64(p) > mean {
			hash |= 1 << uint(i)
		}
	}

	return hash, nil
}

func hammingDistance(a, b uint64) int {
	x := a ^ b
	count := 0
	for x != 0 {
		count++
		x &= x - 1
	}
	return count
}

type openaiRequest struct {
	Model    string          `json:"model"`
	Stream   bool            `json:"stream"`
	Messages []openaiMessage `json:"messages"`
}

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func openaiChat(baseURL, model, prompt string) (string, error) {
	reqBody := openaiRequest{
		Model:  model,
		Stream: false,
		Messages: []openaiMessage{
			{Role: "user", Content: prompt},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	resp, err := http.Post(baseURL+"/v1/chat/completions", "application/json", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("openai request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai returned %d: %s", resp.StatusCode, string(body))
	}

	var oaiResp openaiResponse
	if err := json.Unmarshal(body, &oaiResp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if len(oaiResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return oaiResp.Choices[0].Message.Content, nil
}

func insertMenu(ctx context.Context, q *db.Queries, googlePlaceID string, menu *menuData) error {
	// Look up place
	place, err := q.GetPlaceByGoogleID(ctx, googlePlaceID)
	if err != nil {
		return fmt.Errorf("place %s not found: %w", googlePlaceID, err)
	}

	// Ensure restaurant_details row exists
	restaurant, err := q.UpsertRestaurantDetails(ctx, place.ID)
	if err != nil {
		return fmt.Errorf("upsert restaurant details: %w", err)
	}

	// Clear existing menu data (idempotent)
	_ = q.DeleteMenuItemsByRestaurant(ctx, restaurant.ID)
	_ = q.DeleteMenuCategoriesByRestaurant(ctx, restaurant.ID)
	_ = q.DeleteComboMealsByRestaurant(ctx, restaurant.ID)

	// Insert categories and items
	for i, cat := range menu.Categories {
		category, err := q.CreateMenuCategory(ctx, db.CreateMenuCategoryParams{
			RestaurantID: restaurant.ID,
			Name:         cat.Name,
			SortOrder:    int32(i),
		})
		if err != nil {
			return fmt.Errorf("create category %q: %w", cat.Name, err)
		}

		for _, item := range cat.Items {
			mi, err := q.CreateMenuItem(ctx, db.CreateMenuItemParams{
				RestaurantID: restaurant.ID,
				CategoryID:   pgtype.Int8{Int64: category.ID, Valid: true},
				Name:         item.Name,
				Description:  pgtype.Text{String: item.Description, Valid: item.Description != ""},
				Price:        int32(item.Price),
				PhotoUrl:     pgtype.Text{},
			})
			if err != nil {
				return fmt.Errorf("create item %q: %w", item.Name, err)
			}

			for j, tier := range item.PriceTiers {
				_, err := q.CreatePriceTier(ctx, db.CreatePriceTierParams{
					MenuItemID: mi.ID,
					Label:      tier.Label,
					Quantity:   int32(tier.Quantity),
					Price:      int32(tier.Price),
					SortOrder:  int32(j),
				})
				if err != nil {
					return fmt.Errorf("create price tier %q for item %q: %w", tier.Label, item.Name, err)
				}
			}
		}
	}

	// Insert combo meals
	for _, combo := range menu.Combos {
		cm, err := q.CreateComboMeal(ctx, db.CreateComboMealParams{
			RestaurantID: restaurant.ID,
			Name:         combo.Name,
			Description:  pgtype.Text{String: combo.Description, Valid: combo.Description != ""},
			Price:        int32(combo.Price),
		})
		if err != nil {
			return fmt.Errorf("create combo %q: %w", combo.Name, err)
		}

		for gi, group := range combo.Groups {
			cg, err := q.CreateComboMealGroup(ctx, db.CreateComboMealGroupParams{
				ComboMealID: cm.ID,
				Name:        group.Name,
				MinChoices:  int32(group.MinChoices),
				MaxChoices:  int32(group.MaxChoices),
				SortOrder:   int32(gi),
			})
			if err != nil {
				return fmt.Errorf("create combo group %q: %w", group.Name, err)
			}

			for oi, opt := range group.Options {
				_, err := q.CreateComboMealGroupOption(ctx, db.CreateComboMealGroupOptionParams{
					GroupID:         cg.ID,
					MenuItemID:      pgtype.Int8{},
					ItemName:        pgtype.Text{String: opt.Name, Valid: opt.Name != ""},
					PriceAdjustment: int32(opt.PriceAdjustment),
					SortOrder:       int32(oi),
				})
				if err != nil {
					return fmt.Errorf("create combo option %q: %w", opt.Name, err)
				}
			}
		}
	}

	return nil
}
