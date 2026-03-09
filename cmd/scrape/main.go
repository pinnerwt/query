package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

func main() {
	outDir := flag.String("out", "menu_photos", "Output directory for menu photos")
	flag.Parse()

	if flag.NArg() < 1 {
		log.Fatal("Usage: scrape [--out DIR] <google_place_id>")
	}
	placeID := flag.Arg(0)

	urls, err := scrapeMenuPhotos(placeID)
	if err != nil {
		log.Fatalf("Scrape failed: %v", err)
	}

	if len(urls) == 0 {
		fmt.Printf("No menu photos found for %s\n", placeID)
		return
	}

	// Deduplicate by base URL (before size suffix)
	seen := make(map[string]bool)
	var unique []string
	for _, u := range urls {
		base := u
		if idx := strings.LastIndex(base, "="); idx > 0 {
			base = base[:idx]
		}
		if seen[base] {
			continue
		}
		seen[base] = true
		unique = append(unique, base)
	}
	fmt.Printf("Found %d unique menu photos (from %d raw URLs)\n", len(unique), len(urls))

	// Download at full resolution
	placeDir := filepath.Join(*outDir, placeID)
	os.MkdirAll(placeDir, 0o755)

	downloaded := 0
	for i, baseURL := range unique {
		fullURL := baseURL + "=w1200"

		resp, err := http.Get(fullURL)
		if err != nil {
			log.Printf("[%d] Download failed: %v", i, err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || len(body) < 5000 {
			continue
		}

		ext := ".jpg"
		ct := resp.Header.Get("Content-Type")
		if strings.Contains(ct, "png") {
			ext = ".png"
		}

		hash := sha256.Sum256([]byte(baseURL))
		shortHash := hex.EncodeToString(hash[:8])
		destPath := filepath.Join(placeDir, fmt.Sprintf("%s%s", shortHash, ext))

		os.WriteFile(destPath, body, 0o644)
		fmt.Printf("[%d] %s (%d bytes)\n", i, destPath, len(body))
		downloaded++
	}

	fmt.Printf("\nDone! %d menu photos saved to %s\n", downloaded, placeDir)
}

func scrapeMenuPhotos(placeID string) ([]string, error) {
	url := fmt.Sprintf("https://www.google.com/maps/place/?q=place_id:%s", placeID)

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.UserAgent("Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		chromedp.WindowSize(1280, 900),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	// Navigate to place page
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Sleep(5*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("navigate: %w", err)
	}

	// Click the photo area (aria-label ends with "的相片")
	err = chromedp.Run(ctx,
		chromedp.Click(`[aria-label$="的相片"]`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("click photos: %w", err)
	}

	// Click the "菜單" (Menu) tab
	var hasMenu bool
	err = chromedp.Run(ctx,
		chromedp.Evaluate(`
			const btns = document.querySelectorAll('button');
			let found = false;
			for (const b of btns) {
				if (b.textContent.trim() === '菜單') {
					b.click();
					found = true;
					break;
				}
			}
			found;
		`, &hasMenu),
		chromedp.Sleep(3*time.Second),
	)
	if err != nil || !hasMenu {
		return nil, fmt.Errorf("no menu tab found")
	}

	// Scroll all scrollable containers to trigger lazy-loading of photo thumbnails
	for i := 0; i < 30; i++ {
		chromedp.Run(ctx,
			chromedp.Evaluate(fmt.Sprintf(`
				document.querySelectorAll('*').forEach(el => {
					if (el.scrollHeight > el.clientHeight + 50) {
						el.scrollTop = %d * 500;
					}
				});
			`, i), nil),
			chromedp.Sleep(300*time.Millisecond),
		)
	}

	// Extract all googleusercontent photo URLs from img src and background-image
	var photoURLs []string
	chromedp.Run(ctx,
		chromedp.Evaluate(`
			const urls = new Set();
			const patterns = ['/p/', 'gps-cs'];

			document.querySelectorAll('img').forEach(img => {
				if (img.src && img.src.includes('googleusercontent') &&
					patterns.some(p => img.src.includes(p))) {
					urls.add(img.src);
				}
			});

			document.querySelectorAll('div').forEach(el => {
				const bg = getComputedStyle(el).backgroundImage;
				if (bg && bg.includes('googleusercontent') &&
					patterns.some(p => bg.includes(p))) {
					const match = bg.match(/url\("?([^")\s]+)"?\)/);
					if (match) urls.add(match[1]);
				}
			});

			Array.from(urls).filter(u =>
				!u.includes('=s32') && !u.includes('=s24') &&
				!u.includes('=s48') && !u.includes('=w32')
			);
		`, &photoURLs),
	)

	return photoURLs, nil
}
