# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Methodology

Use test driven development for everything.

## Commands

```bash
# Run all tests
go test ./...

# Run specific test package
go test ./tests

# Run a single test
go test ./tests -run TestSchemaPlaces

# Build the server
go build ./cmd/server

# Regenerate sqlc code after changing SQL queries or migrations
sqlc generate

# Run migrations manually (normally handled by test helpers)
goose -dir migrations postgres "connection-string" up

# Start local PostgreSQL (primary + replica)
cd deploy && cp .env.example .env && docker compose up -d

# Build and run the seed CLI (Step 1: discovery, free)
go build -o seed ./cmd/seed
./seed --lat 25.033 --lng 121.565 --radius 1000 --types restaurant \
    --lang zh-TW \
    --api-key $GOOGLE_API_KEY \
    --db "postgres://query:query@localhost:5432/query?sslmode=disable"

# Build and run the fetch CLI (Step 2: detail fetch, ~$0.035/query)
go build -o fetch ./cmd/fetch
./fetch --lang zh-TW --api-key $GOOGLE_API_KEY \
    --db "postgres://query:query@localhost:5432/query?sslmode=disable"

# Build and run the scrape CLI (menu photo scraper via Google Maps)
go build -o scrape ./cmd/scrape
./scrape ChIJ41wbgbqrQjQR75mxQgbywys                # scrape by google_place_id
./scrape --proxy socks5://host:port ChIJ41wbgbqrQjQR  # with proxy
./scrape --proxy http://user:pass@host:port ChIJ...    # with authenticated proxy

# Build and run the OCR CLI (Step 4: menu extraction, two-pass)
# Requires: ollama serve with glm-ocr-gpu model, llama.cpp server with qwen3.5:27b at :8090
go build -o ocr ./cmd/ocr
./ocr --dry-run ChIJ41wbgbqrQjQR75mxQgbywys           # preview without DB write
./ocr --db "postgres://..." ChIJ41wbgbqrQjQR75mxQgbywys  # extract & save to DB
./ocr --max-photos 5 --db "postgres://..." ChIJ...     # limit photos (default: all)
./ocr --model glm-ocr --normalize-model qwen3.5:9b \
    --normalize-url "" --db "..." ChIJ...               # fallback: all-Ollama pipeline
```

## Architecture

Go 1.25.0 project — a restaurant/place database backed by PostgreSQL.

**Database-first workflow**: Define schema in `migrations/` (Goose), write queries in `internal/db/queries/*.sql`, then run `sqlc generate` to produce Go code in `internal/db/generated/`. Never edit generated files directly.

**Key layers**:
- `migrations/` — Goose migration files defining the PostgreSQL schema
- `internal/db/queries/` — Raw SQL files consumed by sqlc (one file per domain: places, restaurants, menu)
- `internal/db/generated/` — Auto-generated Go code from sqlc (models, query methods). Do not edit.
- `internal/db/dbtest/` — Test helper that spins up a PostgreSQL container via testcontainers-go and runs migrations with Goose
- `tests/` — Integration tests against real PostgreSQL containers
- `cmd/server/` — Server entry point (placeholder)
- `cmd/seed/` — CLI for Step 1 discovery: grid sweep with Google Places API, stores to staging tables
- `cmd/fetch/` — CLI for Step 2 detail fetch: replays discovery queries with advanced fields, promotes to `places`/`place_opening_hours`
- `cmd/scrape/` — CLI for scraping menu photos from Google Maps using headless Chrome (chromedp). Supports `--proxy` for SOCKS5/HTTP proxies. Forces `hl=zh-TW` so Chinese selectors work regardless of proxy region.
- `cmd/ocr/` — CLI for menu extraction with two-pass pipeline: GLM-OCR (Ollama, GPU) for raw text, then per-photo normalization via Qwen3.5 (Ollama) into structured JSON, followed by merge/dedup across photos. Includes perceptual dedup to skip near-duplicate photos and image resizing (default 800px max). Writes to `menu_categories`/`menu_items`/`menu_item_price_tiers`/`combo_meals` tables. Takes a Google Place ID, reads photos from `menu_photos/<place_id>/`.
- `internal/seed/` — Google Places API client, grid sweep logic, geo helpers

**sqlc config** (`sqlc.yaml`): Uses `pgx/v5` as the SQL package. Queries dir is `internal/db/queries`, schema dir is `migrations`, output goes to `internal/db/generated`.

**Testing**: Tests use testcontainers-go to create isolated PostgreSQL instances. The `dbtest.SetupTestDB()` helper handles container lifecycle and migration. Tests use testify for assertions.

**Schema**: Places (Google Places integration) → Restaurant details (1:1) → Menu categories → Menu items, combo meals, add-ons. All foreign keys use CASCADE DELETE. Staging tables (`discovery_queries`, `place_discoveries`) hold intermediate discovery results before promotion.

**Data pipeline** has four steps:
1. `cmd/seed` — Discover places via free Google API calls, store in staging tables. Always use `--lang zh-TW` for Chinese names/addresses.
2. `cmd/fetch` — Replay discovery queries with advanced field masks (~$0.035/query), promote to `places`/`place_opening_hours`. Always use `--lang zh-TW`.
3. `cmd/scrape` — Scrape menu photos from Google Maps "菜單" tab via headless Chrome
4. `cmd/ocr` — Two-pass menu extraction: GLM-OCR (Ollama GPU) reads photo text, then each photo is independently normalized into structured JSON by Qwen3.5 (Ollama), results are merged/deduped across photos, then inserted into `restaurant_details`/`menu_categories`/`menu_items`/`menu_item_price_tiers`/`combo_meals`

**Model setup** (RTX 3090, 24GB VRAM):
- Ollama: `ollama pull glm-ocr` then create `glm-ocr-gpu` variant with `num_ctx 8192` for GPU loading. Run `ollama serve`.
- llama.cpp: Run qwen3.5:27b GGUF on `http://127.0.0.1:8090` for normalization (OpenAI-compatible API).
- GLM-OCR on GPU: ~10s/image. On CPU: ~8.5min/image (avoid).

**Types**: Prices stored as integers (TWD). Special price values: `-1` = unknown (not shown on menu), `-2` = 時價 (market price). Nullable fields use `pgtype` types.
