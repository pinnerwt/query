package seed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const defaultMaxWidthPx = 800

// DownloadPhoto fetches a photo from Google Places API and saves it locally.
// photoName is the full resource name (e.g., "places/ChIJ.../photos/ATCDNf...").
// Returns the local file path.
func (c *PlacesClient) DownloadPhoto(ctx context.Context, photoName string, destDir string) (string, error) {
	url := fmt.Sprintf("%s/%s/media?maxWidthPx=%d&key=%s",
		c.baseURL, photoName, defaultMaxWidthPx, c.apiKey)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("create photo request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetch photo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("photo request returned %d: %s", resp.StatusCode, string(body))
	}

	// Determine file extension from content type
	ext := ".jpg"
	ct := resp.Header.Get("Content-Type")
	if strings.Contains(ct, "png") {
		ext = ".png"
	} else if strings.Contains(ct, "webp") {
		ext = ".webp"
	}

	// Build filename: placeID + short hash of photo ref to keep it short
	// photoName looks like "places/ChIJxxx/photos/ATCDyyy..."
	parts := strings.Split(photoName, "/")
	var filename string
	if len(parts) >= 4 {
		hash := sha256.Sum256([]byte(parts[3]))
		shortHash := hex.EncodeToString(hash[:8])
		filename = parts[1] + "_" + shortHash + ext
	} else {
		hash := sha256.Sum256([]byte(photoName))
		filename = hex.EncodeToString(hash[:12]) + ext
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("create photo dir: %w", err)
	}

	destPath := filepath.Join(destDir, filename)
	f, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("create photo file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", fmt.Errorf("write photo: %w", err)
	}

	return destPath, nil
}
