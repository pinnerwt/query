package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
        {"name": "item name", "price": 100, "description": "optional description"}
      ]
    }
  ]
}

Rules:
- price is in TWD (New Taiwan Dollars), as an integer (no decimals)
- If an item has no price, set price to 0
- If there are no clear categories, use "其他" as the category name
- Merge duplicate items (same name) keeping the first occurrence
- description is optional, omit or set to "" if none
- Do NOT include any text outside the JSON object

Raw OCR text:
`

type ollamaRequest struct {
	Model    string          `json:"model"`
	Stream   bool            `json:"stream"`
	Messages []ollamaMessage `json:"messages"`
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
}

type menuCategory struct {
	Name  string     `json:"name"`
	Items []menuItem `json:"items"`
}

type menuItem struct {
	Name        string `json:"name"`
	Price       int    `json:"price"`
	Description string `json:"description,omitempty"`
}

func main() {
	ollamaURL := flag.String("ollama", "http://127.0.0.1:11434", "Ollama API base URL")
	model := flag.String("model", "glm-ocr", "Model name for OCR")
	normalizeModel := flag.String("normalize-model", "qwen3.5:9b", "Model name for normalization (text-only)")
	maxPhotos := flag.Int("max-photos", 3, "Max photos to OCR (reduces duplicates)")
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

	// Limit photos to reduce duplicate content
	if len(files) > *maxPhotos {
		files = files[:*maxPhotos]
	}

	// Pass 1: OCR all images
	var allOCRText strings.Builder
	for i, f := range files {
		fmt.Printf("[%d/%d] OCR: %s ...\n", i+1, len(files), filepath.Base(f))
		text, err := ocrImage(*ollamaURL, *model, f)
		if err != nil {
			log.Printf("  Warning: OCR failed: %v", err)
			continue
		}
		allOCRText.WriteString(fmt.Sprintf("--- Photo %d ---\n", i+1))
		allOCRText.WriteString(text)
		allOCRText.WriteString("\n\n")
	}

	rawText := allOCRText.String()
	if strings.TrimSpace(rawText) == "" {
		log.Fatal("No text extracted from any photos")
	}
	fmt.Printf("\n=== Raw OCR text (%d chars) ===\n%s\n", len(rawText), rawText)

	// Pass 2: Normalize into structured JSON
	fmt.Println("=== Normalizing into structured menu ===")
	menu, err := normalizeMenu(*ollamaURL, *normalizeModel, rawText)
	if err != nil {
		log.Fatalf("Normalization failed: %v", err)
	}

	// Print results
	totalItems := 0
	for _, cat := range menu.Categories {
		fmt.Printf("\n[%s]\n", cat.Name)
		for _, item := range cat.Items {
			fmt.Printf("  %s — %d元\n", item.Name, item.Price)
			totalItems++
		}
	}
	fmt.Printf("\nTotal: %d categories, %d items\n", len(menu.Categories), totalItems)

	if *dryRun {
		fmt.Println("\n(dry-run mode, not writing to database)")
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
	if err := insertMenu(ctx, q, googlePlaceID, menu); err != nil {
		log.Fatalf("Failed to insert menu: %v", err)
	}
	fmt.Println("\nMenu saved to database!")
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

func ocrImage(baseURL, model, imagePath string) (string, error) {
	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}

	imgB64 := base64.StdEncoding.EncodeToString(imgData)

	return ollamaChat(baseURL, model, ocrPrompt, []string{imgB64})
}

func normalizeMenu(baseURL, model, rawText string) (*menuData, error) {
	prompt := normalizePrompt + rawText

	result, err := ollamaChat(baseURL, model, prompt, nil)
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

	var menu menuData
	if err := json.Unmarshal([]byte(result), &menu); err != nil {
		return nil, fmt.Errorf("parse menu JSON: %w\nraw response:\n%s", err, result)
	}

	return &menu, nil
}

func ollamaChat(baseURL, model, prompt string, images []string) (string, error) {
	msg := ollamaMessage{
		Role:    "user",
		Content: prompt,
		Images:  images,
	}

	reqBody := ollamaRequest{
		Model:    model,
		Stream:   false,
		Messages: []ollamaMessage{msg},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	resp, err := http.Post(baseURL+"/api/chat", "application/json", bytes.NewReader(jsonBody))
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
			_, err := q.CreateMenuItem(ctx, db.CreateMenuItemParams{
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
		}
	}

	return nil
}
