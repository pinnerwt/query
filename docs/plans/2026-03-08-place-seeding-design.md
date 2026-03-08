# Place Seeding Design — Layer 1

## Goal

Populate the Layer 1 tables (`places`, `place_opening_hours`, `place_photos`) with data from Google Maps, starting with restaurants. The process is cost-oriented and split into two steps.

## Two-Step Process

### Step 1: Discovery (free)

Use the Google Places API (New) `searchNearby` to discover places within a radius around a center point. Only free/Basic fields are requested. Results are stored in a staging table for later detailed fetch.

### Step 2: Detail Fetch (paid, ~$0.035/query)

Replay saved discovery queries with an advanced field mask to get hours, photos, and other detailed data. Promote results from the staging table into the full `places` + related tables.

## Schema

### `discovery_queries` table

Stores the search parameters used during discovery so they can be replayed in Step 2.

```sql
CREATE TABLE discovery_queries (
    id           BIGSERIAL PRIMARY KEY,
    latitude     DOUBLE PRECISION NOT NULL,
    longitude    DOUBLE PRECISION NOT NULL,
    radius       DOUBLE PRECISION NOT NULL,
    place_type   TEXT NOT NULL,
    result_count INTEGER NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### `place_discoveries` table

Staging table for discovered places. Lightweight records with only free fields.

```sql
CREATE TABLE place_discoveries (
    id              BIGSERIAL PRIMARY KEY,
    query_id        BIGINT REFERENCES discovery_queries(id) ON DELETE SET NULL,
    google_place_id TEXT NOT NULL UNIQUE,
    name            TEXT,
    address         TEXT,
    latitude        DOUBLE PRECISION,
    longitude       DOUBLE PRECISION,
    place_types     TEXT[] DEFAULT '{}',
    status          TEXT NOT NULL DEFAULT 'pending',  -- 'pending' | 'fetched'
    discovered_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

## Grid Sweep Algorithm (Step 1)

1. **Input**: center (lat, lng), radius R, ordered list of types, sub-radius r (default R/3).
2. **Generate grid**: Square grid of points spaced `2r * 0.8` apart, clipped to circle R.
3. **For each type in order** (broad types first, sub-types later):
   - For each grid cell:
     - Probe with `places.id` only (free). If 0 results, skip.
     - If 20 results (saturated), subdivide into 4 smaller cells and recurse.
     - If 1-19 results, fetch with basic free fields (`id`, `displayName`, `types`, `location`, `formattedAddress`).
     - Save query parameters to `discovery_queries`.
     - Insert results into `place_discoveries` with `ON CONFLICT (google_place_id) DO NOTHING`.
4. **Report**: Per-type summary of cells probed, new places discovered, duplicates skipped.

Broad types (e.g. `restaurant`) are swept first. Sub-types (e.g. `sushi_restaurant`) are swept after; most results will already exist and get deduplicated.

## Detail Fetch (Step 2)

Replay each row from `discovery_queries` with an advanced field mask to retrieve hours, photos, and other paid fields. Match results by `google_place_id` to the staging table, then promote into the full Layer 1 tables (`places`, `place_opening_hours`, `place_photos`). Mark promoted discoveries as `status = 'fetched'`.

## CLI Interface

```
go run ./cmd/seed discover \
  --lat 25.033 \
  --lng 121.565 \
  --radius 2000 \
  --types restaurant,cafe,bar \
  --sub-radius 500 \
  --api-key $GOOGLE_API_KEY
```

**Flags:**
- `--lat`, `--lng` (required): Center point.
- `--radius` (required): Search radius in meters.
- `--types` (required): Comma-separated ordered list of Google place types.
- `--sub-radius` (optional, default radius/3): Grid cell radius.
- `--api-key` (required): Google API key, or read from `GOOGLE_API_KEY` env var.

**Output**: Progress logs to stdout with per-type and total summaries.

## Project Structure

**New files:**
- `migrations/00004_create_place_discoveries.sql` — staging table + discovery queries migration
- `internal/db/queries/discoveries.sql` — sqlc queries for staging table
- `cmd/seed/main.go` — CLI entry point with flag parsing
- `internal/seed/discover.go` — grid sweep logic, Google API client
- `internal/seed/geo.go` — grid generation, haversine distance helpers

**Dependencies:**
- `net/http` + `encoding/json` for Google API calls (no external SDK)
- `flag` package for CLI

**Testing:**
- Integration tests for staging table schema + queries (testcontainers)
- Unit tests for grid generation geometry (no DB needed)
- Mock HTTP server for Google API call tests

## Google Places API Details

- Endpoint: `POST https://places.googleapis.com/v1/places:searchNearby`
- Auth: `X-Goog-Api-Key` header
- Field mask: `X-Goog-FieldMask` header
- Free Basic fields: `places.id`, `places.displayName`, `places.types`, `places.location`, `places.formattedAddress`
- Max 20 results per call, no pagination token
- `includedTypes` filters by Google's predefined Table A place types
