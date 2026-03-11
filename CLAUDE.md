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

# Build all binaries
go build ./cmd/server && go build ./cmd/ocr && go build ./cmd/seed

# Build the server (includes embedded frontend)
go build ./cmd/server

# Build the frontend (must run before building server)
cd frontend && npm install && npm run build

# Regenerate sqlc code after changing SQL queries or migrations
sqlc generate

# Run migrations manually (normally handled by test helpers)
goose -dir migrations postgres "connection-string" up

# Start local PostgreSQL (primary + replica)
cd deploy && cp .env.example .env && docker compose up -d

# Run the server
./server --db "postgres://query:query@localhost:5432/query?sslmode=disable"
./server --db "..." --jwt-secret "my-secret" --base-url "https://mydomain.com" --addr ":8080"

# Build and run the seed CLI (Step 1: discovery, free)
go build -o seed ./cmd/seed

# Seed all Taiwan restaurants — main island (center=Chiayi, radius=210km, resumable)
./seed --lat 23.60 --lng 120.95 --radius 210000 --sub-radius 50000 \
    --types restaurant --lang zh-TW \
    --api-key $GOOGLE_API_KEY \
    --db "postgres://query:query@localhost:5432/query?sslmode=disable"

# Seed outlying islands (run separately after main island)
./seed --lat 23.583 --lng 119.583 --radius 30000 --sub-radius 10000 \
    --types restaurant --lang zh-TW --checkpoint seed_checkpoint_penghu.json \
    --api-key $GOOGLE_API_KEY --db "postgres://query:query@localhost:5432/query?sslmode=disable"
./seed --lat 24.455 --lng 118.381 --radius 15000 --sub-radius 5000 \
    --types restaurant --lang zh-TW --checkpoint seed_checkpoint_kinmen.json \
    --api-key $GOOGLE_API_KEY --db "postgres://query:query@localhost:5432/query?sslmode=disable"
./seed --lat 26.151 --lng 119.923 --radius 30000 --sub-radius 10000 \
    --types restaurant --lang zh-TW --checkpoint seed_checkpoint_matsu.json \
    --api-key $GOOGLE_API_KEY --db "postgres://query:query@localhost:5432/query?sslmode=disable"
./seed --lat 22.662 --lng 121.490 --radius 5000 --sub-radius 5000 \
    --types restaurant --lang zh-TW --checkpoint seed_checkpoint_green.json \
    --api-key $GOOGLE_API_KEY --db "postgres://query:query@localhost:5432/query?sslmode=disable"
./seed --lat 22.050 --lng 121.533 --radius 6000 --sub-radius 6000 \
    --types restaurant --lang zh-TW --checkpoint seed_checkpoint_orchid.json \
    --api-key $GOOGLE_API_KEY --db "postgres://query:query@localhost:5432/query?sslmode=disable"
./seed --lat 22.337 --lng 120.369 --radius 3000 --sub-radius 3000 \
    --types restaurant --lang zh-TW --checkpoint seed_checkpoint_xiaoliuqiu.json \
    --api-key $GOOGLE_API_KEY --db "postgres://query:query@localhost:5432/query?sslmode=disable"

# Seed a small area (e.g. Taipei)
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
    --normalize-url "" --db "..." ChIJ...               # fallback: all-Ollama pipeline (9b, slower)
./ocr --max-dim 800 --db "..." ChIJ...                 # lower resolution (faster, less accurate)
```

## Development Environment

Tmux session `query` has three windows:
- `query:1` (backend) — runs `./server --db "postgres://..."`. After `go build ./cmd/server`, restart here (Ctrl-C, re-run).
- `query:2` (frontend) — for frontend dev (`cd frontend && npm run dev`).
- `query:3` (llama) — llama.cpp server for OCR normalization, launched via `./llm.sh`.

## Architecture

Go 1.25.0 project — an owner-facing restaurant menu platform backed by PostgreSQL + PostGIS. Owners register, create restaurants, upload menu photos (OCR extracts structured data), and get a QR code + public ordering page.

**Database-first workflow**: Define schema in `migrations/` (Goose), write queries in `internal/db/queries/*.sql`, then run `sqlc generate` to produce Go code in `internal/db/generated/`. Never edit generated files directly.

**Key layers**:
- `migrations/` — Goose migration files (PostGIS-enabled). Migration 00006 is the owner-app migration that creates owners, restaurants, orders tables and re-points menu FKs. Migration 00007 replaces `combo_meals`/`add_ons` with `menu_item_option_groups`/`menu_item_option_choices`.
- `internal/db/queries/` — Raw SQL files consumed by sqlc (one file per domain: places, owners, owner_restaurants, menu, orders, menu_uploads)
- `internal/db/generated/` — Auto-generated Go code from sqlc (models, query methods). Do not edit.
- `internal/db/dbtest/` — Test helper that spins up a PostGIS container via testcontainers-go and runs migrations with Goose
- `tests/` — Integration tests against real PostgreSQL containers (schema, queries, auth, restaurant CRUD, menu CRUD, orders, migration)
- `cmd/server/` — HTTP server with all API endpoints: auth (JWT), restaurant CRUD, menu editor, photo upload + OCR, QR codes, ordering, public menu pages. Frontend embedded via `//go:embed`. Uses Go 1.25 ServeMux pattern matching.
- `cmd/seed/` — CLI for Step 1 discovery: hexagonal grid sweep with Google Places API, stores to staging tables. Supports checkpointing (`--checkpoint`) for resumable sweeps and auto-caps sub-radius at 50,000m (API limit). Max depth 23 (~17m radius) to avoid saturation. Rate limited to 8 req/s.
- `cmd/fetch/` — CLI for Step 2 detail fetch: replays discovery queries with advanced fields, promotes to `places`/`place_opening_hours`
- `cmd/scrape/` — CLI for scraping menu photos from Google Maps using headless Chrome (chromedp). Supports `--proxy` for SOCKS5/HTTP proxies. Forces `hl=zh-TW` so Chinese selectors work regardless of proxy region.
- `cmd/ocr/` — Thin wrapper around `internal/ocr` package. Two-pass pipeline: GLM-OCR (Ollama, GPU) for raw text, then normalization via Qwen3.5 into structured JSON.
- `internal/auth/` — bcrypt password hashing, JWT (HS256, 24h expiry) via `golang-jwt/jwt/v5`, HttpOnly session cookie auth with hourly token rotation, auth middleware
- `internal/ratelimit/` — Sliding window rate limiter (per-IP, in-memory)
- `internal/slug/` — URL-safe slug generation from restaurant names (CJK-aware, random hex suffix)
- `internal/ocr/` — OCR pipeline (types, image processing, normalization, menu DB insertion). Extracted from cmd/ocr for reuse by server.
- `internal/storage/` — File upload handling (saves to `menu_photos/{restaurant_id}/`)
- `internal/seed/` — Google Places API client, grid sweep logic, geo helpers
- `frontend/` — Preact + Vite + Tailwind CSS 4 + TypeScript SPA with SortableJS for drag-and-drop. Built to `frontend/dist/`, embedded in Go binary via `frontend_embed.go`.
  - `frontend/src/app.tsx` — Root component: AuthProvider → Login (unauthenticated) or Layout + Router (authenticated)
  - `frontend/src/lib/api.ts` — Fetch wrapper, all API calls, type definitions
  - `frontend/src/lib/auth.tsx` — AuthContext provider (HttpOnly session cookie, validates via /api/auth/me)
  - `frontend/src/components/Layout.tsx` — App shell: dark sidebar (`bg-slate-900`, w-60) with context-aware nav, mobile drawer, user profile. Wraps all authenticated routes.
  - `frontend/src/components/Toggle.tsx` — Toggle switch (replaces checkboxes)
  - `frontend/src/components/Modal.tsx` — Modal dialog with backdrop blur
  - `frontend/src/components/Skeleton.tsx` — SkeletonCard and SkeletonList loading placeholders
  - `frontend/src/pages/Login.tsx` — Split layout: amber gradient brand panel (desktop) + form card
  - `frontend/src/pages/Dashboard.tsx` — Restaurant card grid, stats bar, create modal
  - `frontend/src/pages/RestaurantEdit.tsx` — Sectioned form cards, toggle switches, publish banner, QR card
  - `frontend/src/pages/MenuEditor.tsx` — Accordion categories, inline edit (click to expand), SortableJS drag-and-drop, dashed drop zone for photo upload, floating save bar on dirty state
  - `frontend/src/pages/Orders.tsx` — Kanban board (4 columns, desktop) / filter tabs + list (mobile), relative timestamps, pulsing live indicator
- `frontend_embed.go` — Root package file with `//go:embed frontend/dist` directive

**Frontend design system**: Amber primary (`amber-600`), stone-50 body background, slate-900 sidebar, emerald for success/published states. Cards use `rounded-xl shadow-sm border-slate-100`. Inputs use `rounded-lg` with `focus:ring-2 focus:ring-amber-500/20`. All pages use functional setState to prevent stale closures, and async buttons are disabled during requests.

**Server endpoints** (`cmd/server/main.go`):
- Auth: `POST /api/auth/register` (rate-limited), `POST /api/auth/login`, `POST /api/auth/logout`, `GET /api/auth/me`
- Restaurant CRUD (auth): `POST/GET/PUT/DELETE /api/restaurants/*`, `PUT .../hours`, `PUT .../publish`
- Menu (auth): `GET/PUT /api/restaurants/{id}/menu`
- Photos + OCR (auth): `POST /api/restaurants/{id}/menu-photos`, `POST /api/restaurants/{id}/ocr`
- QR code (auth): `GET /api/restaurants/{id}/qr`
- Orders (auth): `GET /api/restaurants/{id}/orders`, `PUT .../orders/{orderId}/status`
- Public: `GET /api/public/menu/{slug}`, `POST /api/public/orders/{slug}`, `GET /api/public/orders/{slug}/{orderId}`
- Public HTML: `GET /r/{slug}` (server-rendered menu + cart + ordering)
- Frontend SPA: `/app/*` (embedded Preact app)
- Legacy: `GET /api/restaurants`, `GET /api/menu`, `GET /api/places`

**sqlc config** (`sqlc.yaml`): Uses `pgx/v5` as the SQL package. Queries dir is `internal/db/queries`, schema dir is `migrations`, output goes to `internal/db/generated`.

**Testing**: Tests use testcontainers-go to create isolated PostGIS instances. The `dbtest.SetupTestDB()` helper handles container lifecycle and migration. Tests use testify for assertions. Docker image: `postgis/postgis:16-3.4-alpine`.

**Schema**: Two paths coexist:
- Legacy: Places (Google Places) → Restaurant details (1:1) → Menu tables
- Owner app: Owners → Restaurants (with slug, hours, publish state) → Menu tables, Orders → Order items
- Menu tables (`menu_categories`, `menu_items`, `menu_item_option_groups`, `menu_item_option_choices`) have FKs into the restaurant hierarchy. Migration 00006 preserved IDs so existing data works with both paths. Migration 00007 unified combo meals and add-ons into per-item option groups/choices.

**Data pipeline** has four steps:
1. `cmd/seed` — Discover places via free Google API calls, store in staging tables. Always use `--lang zh-TW` for Chinese names/addresses.
2. `cmd/fetch` — Replay discovery queries with advanced field masks (~$0.035/query), promote to `places`/`place_opening_hours`. Always use `--lang zh-TW`.
3. `cmd/scrape` — Scrape menu photos from Google Maps "菜單" tab via headless Chrome
4. `cmd/ocr` — Two-pass menu extraction: GLM-OCR (Ollama GPU, 1600px max-dim) reads photo text, then combined text is normalized into structured JSON by Qwen3.5:27b (llama.cpp), then inserted into `restaurants`/`menu_categories`/`menu_items`/`menu_item_price_tiers`/`menu_item_option_groups`/`menu_item_option_choices`

**Model setup** (RTX 3090, 24GB VRAM):
- Ollama: `ollama pull glm-ocr` then create `glm-ocr-gpu` variant with `num_ctx 8192` for GPU loading. Run `ollama serve`.
- llama.cpp: Run qwen3.5:27b GGUF on `http://127.0.0.1:8090` for normalization (OpenAI-compatible API).
- GLM-OCR on GPU: ~10s/image. On CPU: ~8.5min/image (avoid).

**Types**: Prices stored as integers (TWD). Special price values: `-1` = unknown (not shown on menu), `-2` = 時價 (market price). Nullable fields use `pgtype` types.
