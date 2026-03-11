# Query — OCR Menu to Online Ordering

A restaurant platform that turns menu photos into a live online ordering system. Upload menu photos, extract items via OCR, edit the structured menu, then share a public ordering page with a QR code.

Built with Go, PostgreSQL + PostGIS, and a Preact frontend.

## How It Works

1. **Upload** menu photos for a restaurant
2. **OCR** extracts menu items automatically (GLM-OCR → Qwen3.5 normalization)
3. **Edit** the structured menu — categories, items, prices, option groups
4. **Publish** and share a QR code linking to the public ordering page
5. **Manage** incoming orders via a kanban board

## Prerequisites

- Go 1.25+
- Node.js 18+ and npm
- Docker (for PostgreSQL + PostGIS)
- For OCR: [Ollama](https://ollama.ai) with `glm-ocr` model, and [llama.cpp](https://github.com/ggerganov/llama.cpp) serving Qwen3.5:27b
- Optional: Google Maps API key (for discovering and scraping restaurant data)

## Quick Start

### 1. Start PostgreSQL

```bash
cd deploy
cp .env.example .env
docker compose up -d
```

This starts a PostGIS 16 instance on port 5432 (user/pass/db: `query`). Migrations run automatically on first start.

### 2. Build the frontend

```bash
cd frontend
npm install
npm run build
```

### 3. Build and run the server

```bash
go build ./cmd/server
./server --db "postgres://query:query@localhost:5432/query?sslmode=disable"
```

The app is available at `http://localhost:8080/app/`.

#### Server flags

| Flag | Default | Description |
|------|---------|-------------|
| `--db` | `DATABASE_URL` env | PostgreSQL connection string |
| `--addr` | `:8080` | Listen address |
| `--jwt-secret` | random | JWT signing secret (set for persistent sessions) |
| `--base-url` | `http://localhost:8080` | Public URL for QR codes |
| `--photos-dir` | `menu_photos` | Directory for uploaded photos |
| `--ollama` | `http://127.0.0.1:11434` | Ollama API URL |
| `--ocr-model` | `glm-ocr-gpu` | OCR model name |
| `--norm-model` | `qwen3.5:27b` | Normalization model |
| `--norm-url` | `http://127.0.0.1:8090` | llama.cpp API URL |
| `--max-dim` | `1600` | Max image dimension for OCR |

## OCR Setup

Menu OCR uses a two-pass pipeline. An NVIDIA GPU with 24GB VRAM (e.g. RTX 3090) is recommended.

### Pass 1 — GLM-OCR (text extraction)

```bash
ollama pull glm-ocr
# Create a GPU variant with larger context
ollama create glm-ocr-gpu -f - <<EOF
FROM glm-ocr
PARAMETER num_ctx 8192
EOF
ollama serve
```

### Pass 2 — Qwen3.5:27b (normalization)

Run a Qwen3.5:27b GGUF via llama.cpp on port 8090:

```bash
# See llm.sh for the exact launch command
./llm.sh
```

The normalization pass converts raw OCR text into structured JSON (categories, items, prices, option groups).

## Data Pipeline (Optional)

For bulk-importing restaurants from Google Maps:

```bash
go build -o seed ./cmd/seed
go build -o fetch ./cmd/fetch
go build -o scrape ./cmd/scrape
go build -o ocr ./cmd/ocr
```

| Step | Command | Cost | Description |
|------|---------|------|-------------|
| 1 | `seed` | Free | Discover places via Google Places API |
| 2 | `fetch` | ~$0.035/place | Fetch full details (hours, address, etc.) |
| 3 | `scrape` | Free | Scrape menu photos from Google Maps |
| 4 | `ocr` | Free (local) | Extract structured menus from photos |

Example — seed and process a small area:

```bash
export GOOGLE_API_KEY=your-key
export DB="postgres://query:query@localhost:5432/query?sslmode=disable"

# Discover restaurants in Taipei
./seed --lat 25.033 --lng 121.565 --radius 1000 \
    --types restaurant --lang zh-TW \
    --api-key $GOOGLE_API_KEY --db $DB

# Fetch full details
./fetch --lang zh-TW --api-key $GOOGLE_API_KEY --db $DB

# Scrape menu photos
./scrape ChIJ...  # pass a google_place_id

# Run OCR
./ocr --db $DB ChIJ...
```

## Development

### Run tests

Tests use testcontainers-go to spin up isolated PostGIS instances — Docker must be running.

```bash
go test ./...              # all tests
go test ./tests            # integration tests only
go test ./tests -run TestX # single test
```

### Frontend dev server

```bash
cd frontend
npm run dev
```

### Regenerate database code

After changing SQL in `internal/db/queries/` or adding migrations:

```bash
sqlc generate
```

## Project Structure

```
cmd/
  server/    HTTP server (embeds frontend)
  seed/      Google Places discovery CLI
  fetch/     Place detail fetcher CLI
  scrape/    Google Maps menu photo scraper
  ocr/       Menu OCR CLI
internal/
  auth/      JWT auth, bcrypt, session cookies
  db/        Database layer (sqlc generated)
  ocr/       OCR pipeline (GLM-OCR + Qwen normalization)
  storage/   File upload handling
  slug/      URL-safe slug generation
frontend/    Preact + Vite + Tailwind SPA
migrations/  Goose SQL migrations (PostGIS)
deploy/      Docker Compose for PostgreSQL
```
