package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// SaveUpload saves an uploaded file to menu_photos/{restaurantID}/{filename}.
func SaveUpload(baseDir string, restaurantID int64, filename string, r io.Reader) (string, error) {
	dir := filepath.Join(baseDir, fmt.Sprintf("%d", restaurantID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create dir: %w", err)
	}

	path := filepath.Join(dir, filename)
	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	return path, nil
}
