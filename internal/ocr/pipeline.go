package ocr

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strings"

	"golang.org/x/image/draw"
)

const OCRPrompt = "OCR this menu image. Extract all text exactly as shown."

const NormalizePrompt = `You are given raw OCR text from a restaurant menu photo. Parse it into structured JSON.

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

// OcrImage reads and OCRs a single image using the Ollama API.
func OcrImage(baseURL, model, imagePath string, maxDim int, timeout context.Context) (string, error) {
	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}

	if maxDim > 0 {
		resized, err := ResizeImage(imgData, maxDim)
		if err != nil {
			log.Printf("  Warning: resize failed, using original: %v", err)
		} else {
			imgData = resized
		}
	}

	imgB64 := base64.StdEncoding.EncodeToString(imgData)
	return OllamaChatCtx(timeout, baseURL, model, OCRPrompt, []string{imgB64}, "", nil)
}

// NormalizeMenu sends raw OCR text to an LLM for structured JSON extraction.
func NormalizeMenu(baseURL, model, rawText string, useOpenAI bool) (*MenuData, error) {
	prompt := NormalizePrompt + rawText

	var result string
	var err error
	if useOpenAI {
		result, err = OpenaiChat(baseURL, model, prompt)
	} else {
		thinkFalse := false
		result, err = OllamaChat(baseURL, model, prompt, nil, "", &thinkFalse)
	}
	if err != nil {
		return nil, err
	}

	// Strip markdown code fences if present
	result = strings.TrimSpace(result)
	if strings.HasPrefix(result, "```") {
		lines := strings.Split(result, "\n")
		if len(lines) > 2 {
			result = strings.Join(lines[1:len(lines)-1], "\n")
		}
	}

	result = FixInvalidEscapes(result)

	var menu MenuData
	if err := json.Unmarshal([]byte(result), &menu); err != nil {
		return nil, fmt.Errorf("parse menu JSON: %w\nraw response:\n%s", err, result)
	}

	return &menu, nil
}

// MergeMenus combines per-photo MenuData results into a single menu.
func MergeMenus(menus []*MenuData) *MenuData {
	type catEntry struct {
		cat      MenuCategory
		itemSeen map[string]bool
	}

	catMap := make(map[string]int)
	var cats []catEntry

	for _, m := range menus {
		for _, cat := range m.Categories {
			idx, exists := catMap[cat.Name]
			if !exists {
				idx = len(cats)
				catMap[cat.Name] = idx
				cats = append(cats, catEntry{
					cat:      MenuCategory{Name: cat.Name},
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

	result := &MenuData{}
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

// FixInvalidEscapes removes backslashes before characters that are not valid JSON escape sequences.
func FixInvalidEscapes(s string) string {
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
			switch next {
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't', 'u':
				b.WriteByte(ch)
			default:
				continue
			}
		} else {
			b.WriteByte(ch)
		}
	}
	return b.String()
}

// ResizeImage downscales a JPEG/PNG so the longest side is at most maxDim pixels.
func ResizeImage(data []byte, maxDim int) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	if w <= maxDim && h <= maxDim {
		return data, nil
	}

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

// DeduplicateImages removes near-duplicate photos using perceptual hashing.
func DeduplicateImages(files []string) []string {
	type hashEntry struct {
		hash uint64
		file string
	}

	var seen []hashEntry
	var result []string

	for _, f := range files {
		h, err := imageHash(f)
		if err != nil {
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

func imageHash(path string) (uint64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return 0, err
	}

	dst := image.NewGray(image.Rect(0, 0, 8, 8))
	draw.BiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)

	var sum float64
	for _, p := range dst.Pix {
		sum += float64(p)
	}
	mean := sum / 64.0

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

// FindImages returns image files in a directory.
func FindImages(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		name := strings.ToLower(e.Name())
		if strings.HasSuffix(name, ".jpg") || strings.HasSuffix(name, ".jpeg") || strings.HasSuffix(name, ".png") {
			files = append(files, dir+"/"+e.Name())
		}
	}
	return files, nil
}

// Ollama API types and helpers

type ollamaRequest struct {
	Model    string          `json:"model"`
	Stream   bool            `json:"stream"`
	Messages []ollamaMessage `json:"messages"`
	Format   string          `json:"format,omitempty"`
	Think    *bool           `json:"think,omitempty"`
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

func OllamaChat(baseURL, model, prompt string, images []string, format string, think *bool) (string, error) {
	return OllamaChatCtx(context.Background(), baseURL, model, prompt, images, format, think)
}

func OllamaChatCtx(ctx context.Context, baseURL, model, prompt string, images []string, format string, think *bool) (string, error) {
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
		Think:    think,
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

// OpenAI-compatible API types and helpers

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

func OpenaiChat(baseURL, model, prompt string) (string, error) {
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
