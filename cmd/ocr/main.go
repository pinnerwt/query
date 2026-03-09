package main

import (
	"bytes"
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
)

const defaultPrompt = "這是一張餐廳菜單照片。請識別每個菜品名稱和對應的價格，以JSON array格式輸出，每個元素包含 name 和 price 欄位。只輸出JSON，不要其他文字。"

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

func main() {
	ollamaURL := flag.String("ollama", "http://127.0.0.1:11434", "Ollama API base URL")
	model := flag.String("model", "glm-ocr", "Model name")
	prompt := flag.String("prompt", defaultPrompt, "Prompt for menu extraction")
	flag.Parse()

	if flag.NArg() < 1 {
		log.Fatal("Usage: ocr [--ollama URL] [--model NAME] <image_or_directory>")
	}
	target := flag.Arg(0)

	info, err := os.Stat(target)
	if err != nil {
		log.Fatalf("Cannot access %s: %v", target, err)
	}

	var files []string
	if info.IsDir() {
		entries, _ := os.ReadDir(target)
		for _, e := range entries {
			name := strings.ToLower(e.Name())
			if strings.HasSuffix(name, ".jpg") || strings.HasSuffix(name, ".jpeg") || strings.HasSuffix(name, ".png") {
				files = append(files, filepath.Join(target, e.Name()))
			}
		}
	} else {
		files = []string{target}
	}

	if len(files) == 0 {
		log.Fatal("No image files found")
	}

	for _, f := range files {
		fmt.Printf("=== %s ===\n", f)
		result, err := ocrImage(*ollamaURL, *model, *prompt, f)
		if err != nil {
			log.Printf("Error processing %s: %v", f, err)
			continue
		}
		fmt.Println(result)
		fmt.Println()
	}
}

func ocrImage(baseURL, model, prompt, imagePath string) (string, error) {
	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("read image: %w", err)
	}

	imgB64 := base64.StdEncoding.EncodeToString(imgData)

	reqBody := ollamaRequest{
		Model:  model,
		Stream: false,
		Messages: []ollamaMessage{{
			Role:    "user",
			Content: prompt,
			Images:  []string{imgB64},
		}},
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
