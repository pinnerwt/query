# Place Seeding Step 1 (Discovery) Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a CLI tool that discovers places from the Google Places API (New) `searchNearby` using a grid sweep algorithm, storing results in a staging table for later detail fetching.

**Architecture:** New migration adds `discovery_queries` and `place_discoveries` tables. sqlc generates Go code for those tables. A `internal/seed` package implements grid geometry and Google API calls. A `cmd/seed/main.go` CLI drives the sweep. The Google API client is injected as an interface for testing.

**Tech Stack:** Go 1.25, PostgreSQL, sqlc, goose, testcontainers-go, `net/http` + `encoding/json` (no external Google SDK), `flag` for CLI.

---

### Task 1: Migration — `discovery_queries` and `place_discoveries` tables

**Files:**
- Create: `migrations/00004_create_place_discoveries.sql`

**Step 1: Write the migration file**

```sql
-- +goose Up

CREATE TABLE discovery_queries (
    id           BIGSERIAL PRIMARY KEY,
    latitude     DOUBLE PRECISION NOT NULL,
    longitude    DOUBLE PRECISION NOT NULL,
    radius       DOUBLE PRECISION NOT NULL,
    place_type   TEXT NOT NULL,
    result_count INTEGER NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE place_discoveries (
    id              BIGSERIAL PRIMARY KEY,
    query_id        BIGINT REFERENCES discovery_queries(id) ON DELETE SET NULL,
    google_place_id TEXT NOT NULL UNIQUE,
    name            TEXT,
    address         TEXT,
    latitude        DOUBLE PRECISION,
    longitude       DOUBLE PRECISION,
    place_types     TEXT[] DEFAULT '{}',
    status          TEXT NOT NULL DEFAULT 'pending',
    discovered_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_place_discoveries_status ON place_discoveries(status);
CREATE INDEX idx_place_discoveries_query_id ON place_discoveries(query_id);

-- +goose Down

DROP TABLE IF EXISTS place_discoveries;
DROP TABLE IF EXISTS discovery_queries;
```

**Step 2: Write the schema integration test**

Create test in `tests/schema_discoveries_test.go`:

```go
package tests

import (
	"context"
	"testing"

	"github.com/pinnertw/query/internal/db/dbtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoveryQueriesTable(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()

	var id int64
	err := conn.QueryRow(ctx, `
		INSERT INTO discovery_queries (latitude, longitude, radius, place_type, result_count)
		VALUES (25.033, 121.565, 500.0, 'restaurant', 15)
		RETURNING id
	`).Scan(&id)
	require.NoError(t, err)
	assert.Positive(t, id)

	var lat, lng, radius float64
	var placeType string
	var resultCount int
	err = conn.QueryRow(ctx, `
		SELECT latitude, longitude, radius, place_type, result_count
		FROM discovery_queries WHERE id = $1
	`, id).Scan(&lat, &lng, &radius, &placeType, &resultCount)
	require.NoError(t, err)
	assert.InDelta(t, 25.033, lat, 0.001)
	assert.InDelta(t, 121.565, lng, 0.001)
	assert.InDelta(t, 500.0, radius, 0.1)
	assert.Equal(t, "restaurant", placeType)
	assert.Equal(t, 15, resultCount)
}

func TestPlaceDiscoveriesTable(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()

	// Insert a discovery query
	var queryID int64
	err := conn.QueryRow(ctx, `
		INSERT INTO discovery_queries (latitude, longitude, radius, place_type)
		VALUES (25.033, 121.565, 500.0, 'restaurant')
		RETURNING id
	`).Scan(&queryID)
	require.NoError(t, err)

	// Insert a discovery
	var id int64
	err = conn.QueryRow(ctx, `
		INSERT INTO place_discoveries (query_id, google_place_id, name, address, latitude, longitude, place_types, status)
		VALUES ($1, 'ChIJtest123', 'Test Place', '123 Test St', 25.033, 121.565, ARRAY['restaurant', 'food'], 'pending')
		RETURNING id
	`, queryID).Scan(&id)
	require.NoError(t, err)
	assert.Positive(t, id)

	// Verify data
	var googlePlaceID, name, status string
	var placeTypes []string
	err = conn.QueryRow(ctx, `
		SELECT google_place_id, name, status, place_types
		FROM place_discoveries WHERE id = $1
	`, id).Scan(&googlePlaceID, &name, &status, &placeTypes)
	require.NoError(t, err)
	assert.Equal(t, "ChIJtest123", googlePlaceID)
	assert.Equal(t, "Test Place", name)
	assert.Equal(t, "pending", status)
	assert.Equal(t, []string{"restaurant", "food"}, placeTypes)
}

func TestPlaceDiscoveriesUniqueGoogleID(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()

	_, err := conn.Exec(ctx, `
		INSERT INTO place_discoveries (google_place_id, name)
		VALUES ('unique_disco_test', 'First')
	`)
	require.NoError(t, err)

	_, err = conn.Exec(ctx, `
		INSERT INTO place_discoveries (google_place_id, name)
		VALUES ('unique_disco_test', 'Second')
	`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unique")
}

func TestPlaceDiscoveriesQueryFKSetNull(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()

	var queryID int64
	err := conn.QueryRow(ctx, `
		INSERT INTO discovery_queries (latitude, longitude, radius, place_type)
		VALUES (25.0, 121.0, 500.0, 'restaurant')
		RETURNING id
	`).Scan(&queryID)
	require.NoError(t, err)

	var discoID int64
	err = conn.QueryRow(ctx, `
		INSERT INTO place_discoveries (query_id, google_place_id, name)
		VALUES ($1, 'fk_test_place', 'FK Test')
		RETURNING id
	`, queryID).Scan(&discoID)
	require.NoError(t, err)

	// Delete the query — discovery should remain with NULL query_id
	_, err = conn.Exec(ctx, `DELETE FROM discovery_queries WHERE id = $1`, queryID)
	require.NoError(t, err)

	var nullQueryID *int64
	err = conn.QueryRow(ctx, `
		SELECT query_id FROM place_discoveries WHERE id = $1
	`, discoID).Scan(&nullQueryID)
	require.NoError(t, err)
	assert.Nil(t, nullQueryID)
}
```

**Step 3: Run tests to verify they pass**

Run: `go test ./tests -run TestDiscovery -v && go test ./tests -run TestPlaceDiscoveries -v`
Expected: All PASS

**Step 4: Commit**

```bash
git add migrations/00004_create_place_discoveries.sql tests/schema_discoveries_test.go
git commit -m "Add discovery_queries and place_discoveries tables with tests"
```

---

### Task 2: sqlc queries for discovery tables

**Files:**
- Create: `internal/db/queries/discoveries.sql`
- Regenerate: `internal/db/generated/` (via `sqlc generate`)

**Step 1: Write the sqlc queries**

```sql
-- name: InsertDiscoveryQuery :one
INSERT INTO discovery_queries (latitude, longitude, radius, place_type, result_count)
VALUES (@latitude, @longitude, @radius, @place_type, @result_count)
RETURNING *;

-- name: InsertDiscovery :exec
INSERT INTO place_discoveries (query_id, google_place_id, name, address, latitude, longitude, place_types)
VALUES (@query_id, @google_place_id, @name, @address, @latitude, @longitude, @place_types)
ON CONFLICT (google_place_id) DO NOTHING;

-- name: ListPendingDiscoveries :many
SELECT * FROM place_discoveries WHERE status = 'pending' ORDER BY discovered_at;

-- name: MarkDiscoveryFetched :exec
UPDATE place_discoveries SET status = 'fetched' WHERE google_place_id = $1;

-- name: CountDiscoveriesByStatus :one
SELECT
    COUNT(*) FILTER (WHERE status = 'pending') AS pending,
    COUNT(*) FILTER (WHERE status = 'fetched') AS fetched,
    COUNT(*) AS total
FROM place_discoveries;
```

**Step 2: Run sqlc generate**

Run: `sqlc generate`
Expected: No errors. New file `internal/db/generated/discoveries.sql.go` created.

**Step 3: Write the sqlc query integration test**

Add to `tests/queries_discoveries_test.go`:

```go
package tests

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pinnertw/query/internal/db/dbtest"
	db "github.com/pinnertw/query/internal/db/generated"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInsertDiscoveryQueryAndDiscovery(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()
	q := db.New(conn)

	// Insert a discovery query
	dq, err := q.InsertDiscoveryQuery(ctx, db.InsertDiscoveryQueryParams{
		Latitude:    25.033,
		Longitude:   121.565,
		Radius:      500.0,
		PlaceType:   "restaurant",
		ResultCount: 5,
	})
	require.NoError(t, err)
	assert.Positive(t, dq.ID)
	assert.Equal(t, "restaurant", dq.PlaceType)

	// Insert a discovery
	err = q.InsertDiscovery(ctx, db.InsertDiscoveryParams{
		QueryID:       pgtype.Int8{Int64: dq.ID, Valid: true},
		GooglePlaceID: "ChIJ_insert_test",
		Name:          pgtype.Text{String: "Test Restaurant", Valid: true},
		Address:       pgtype.Text{String: "456 Test St", Valid: true},
		Latitude:      pgtype.Float8{Float64: 25.034, Valid: true},
		Longitude:     pgtype.Float8{Float64: 121.566, Valid: true},
		PlaceTypes:    []string{"restaurant", "food"},
	})
	require.NoError(t, err)

	// Insert duplicate — should be silently skipped
	err = q.InsertDiscovery(ctx, db.InsertDiscoveryParams{
		QueryID:       pgtype.Int8{Int64: dq.ID, Valid: true},
		GooglePlaceID: "ChIJ_insert_test",
		Name:          pgtype.Text{String: "Different Name", Valid: true},
	})
	require.NoError(t, err) // no error, just skipped

	// List pending — should have 1
	pending, err := q.ListPendingDiscoveries(ctx)
	require.NoError(t, err)
	assert.Len(t, pending, 1)
	assert.Equal(t, "ChIJ_insert_test", pending[0].GooglePlaceID)

	// Mark fetched
	err = q.MarkDiscoveryFetched(ctx, "ChIJ_insert_test")
	require.NoError(t, err)

	// List pending — should be empty now
	pending, err = q.ListPendingDiscoveries(ctx)
	require.NoError(t, err)
	assert.Len(t, pending, 0)

	// Count by status
	counts, err := q.CountDiscoveriesByStatus(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(0), counts.Pending)
	assert.Equal(t, int64(1), counts.Fetched)
	assert.Equal(t, int64(1), counts.Total)
}
```

**Step 4: Run tests**

Run: `go test ./tests -run TestInsertDiscoveryQueryAndDiscovery -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/db/queries/discoveries.sql internal/db/generated/ tests/queries_discoveries_test.go
git commit -m "Add sqlc queries for discovery tables with tests"
```

---

### Task 3: Grid geometry helpers

**Files:**
- Create: `internal/seed/geo.go`
- Test: `internal/seed/geo_test.go`

**Step 1: Write failing tests for grid generation and distance**

Create `internal/seed/geo_test.go`:

```go
package seed

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHaversineDistance(t *testing.T) {
	// Taipei 101 to Taipei Main Station ≈ 3.2 km
	dist := HaversineDistance(25.0340, 121.5645, 25.0478, 121.5170)
	assert.InDelta(t, 3200, dist, 200) // within 200m tolerance
}

func TestHaversineDistanceSamePoint(t *testing.T) {
	dist := HaversineDistance(25.0, 121.0, 25.0, 121.0)
	assert.Equal(t, 0.0, dist)
}

func TestGenerateGridPoints(t *testing.T) {
	// Center at (0, 0), radius 1000m, sub-radius 500m
	points := GenerateGridPoints(0, 0, 1000, 500)

	// Should have multiple points
	assert.Greater(t, len(points), 1)

	// All points should be within radius + sub-radius of center
	for _, p := range points {
		dist := HaversineDistance(0, 0, p.Lat, p.Lng)
		assert.LessOrEqual(t, dist, 1000.0+500.0,
			"point (%.4f, %.4f) is %.0fm from center", p.Lat, p.Lng, dist)
	}

	// Center point should be included
	hasCenter := false
	for _, p := range points {
		if math.Abs(p.Lat) < 0.0001 && math.Abs(p.Lng) < 0.0001 {
			hasCenter = true
			break
		}
	}
	assert.True(t, hasCenter, "grid should include the center point")
}

func TestGenerateGridPointsSmallRadius(t *testing.T) {
	// When sub-radius >= radius, should still have at least the center
	points := GenerateGridPoints(25.0, 121.0, 100, 500)
	assert.GreaterOrEqual(t, len(points), 1)
}

func TestSubdivideCell(t *testing.T) {
	center := GridPoint{Lat: 25.0, Lng: 121.0}
	subRadius := 500.0
	children := SubdivideCell(center, subRadius)

	assert.Len(t, children, 4)

	// Each child should be roughly subRadius/2 from center
	for _, child := range children {
		dist := HaversineDistance(center.Lat, center.Lng, child.Point.Lat, child.Point.Lng)
		assert.InDelta(t, subRadius/2, dist, subRadius/2+50)
	}

	// Each child should have half the parent radius
	assert.InDelta(t, subRadius/2, children[0].Radius, 1.0)
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/seed -run TestHaversine -v`
Expected: FAIL — package doesn't exist yet

**Step 3: Implement the geo helpers**

Create `internal/seed/geo.go`:

```go
package seed

import "math"

const earthRadiusMeters = 6_371_000.0

// GridPoint is a lat/lng coordinate.
type GridPoint struct {
	Lat float64
	Lng float64
}

// GridCell is a point with a search radius.
type GridCell struct {
	Point  GridPoint
	Radius float64
}

// HaversineDistance returns the distance in meters between two lat/lng points.
func HaversineDistance(lat1, lng1, lat2, lng2 float64) float64 {
	dLat := toRad(lat2 - lat1)
	dLng := toRad(lng2 - lng1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusMeters * c
}

// GenerateGridPoints generates a square grid of points spaced 2*subRadius*0.8 apart,
// clipped to a circle of the given radius around (centerLat, centerLng).
func GenerateGridPoints(centerLat, centerLng, radius, subRadius float64) []GridPoint {
	spacing := 2 * subRadius * 0.8
	// Convert spacing to approximate degrees
	latStep := spacing / 111_320.0
	lngStep := spacing / (111_320.0 * math.Cos(toRad(centerLat)))

	clipDist := radius + subRadius

	// How many steps in each direction
	nLat := int(math.Ceil(clipDist / spacing))
	nLng := int(math.Ceil(clipDist / spacing))

	var points []GridPoint
	for i := -nLat; i <= nLat; i++ {
		for j := -nLng; j <= nLng; j++ {
			lat := centerLat + float64(i)*latStep
			lng := centerLng + float64(j)*lngStep
			if HaversineDistance(centerLat, centerLng, lat, lng) <= clipDist {
				points = append(points, GridPoint{Lat: lat, Lng: lng})
			}
		}
	}
	return points
}

// SubdivideCell splits a cell into 4 child cells, each with half the radius.
func SubdivideCell(center GridPoint, radius float64) []GridCell {
	halfR := radius / 2
	offset := halfR / 111_320.0
	lngOffset := halfR / (111_320.0 * math.Cos(toRad(center.Lat)))

	return []GridCell{
		{Point: GridPoint{Lat: center.Lat + offset, Lng: center.Lng + lngOffset}, Radius: halfR},
		{Point: GridPoint{Lat: center.Lat + offset, Lng: center.Lng - lngOffset}, Radius: halfR},
		{Point: GridPoint{Lat: center.Lat - offset, Lng: center.Lng + lngOffset}, Radius: halfR},
		{Point: GridPoint{Lat: center.Lat - offset, Lng: center.Lng - lngOffset}, Radius: halfR},
	}
}

func toRad(deg float64) float64 {
	return deg * math.Pi / 180
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/seed -run Test -v`
Expected: All PASS

**Step 5: Commit**

```bash
git add internal/seed/geo.go internal/seed/geo_test.go
git commit -m "Add grid geometry helpers with haversine and subdivision"
```

---

### Task 4: Google Places API client

**Files:**
- Create: `internal/seed/places_api.go`
- Test: `internal/seed/places_api_test.go`

**Step 1: Write the failing test with a mock HTTP server**

Create `internal/seed/places_api_test.go`:

```go
package seed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchNearbyProbe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "test-api-key", r.Header.Get("X-Goog-Api-Key"))
		assert.Equal(t, "places.id", r.Header.Get("X-Goog-FieldMask"))

		var body SearchNearbyRequest
		err := json.NewDecoder(r.Body).Decode(&body)
		require.NoError(t, err)
		assert.Equal(t, []string{"restaurant"}, body.IncludedTypes)
		assert.InDelta(t, 25.033, body.LocationRestriction.Circle.Center.Latitude, 0.001)

		resp := SearchNearbyResponse{
			Places: []PlaceResult{
				{ID: "places/ChIJ1"},
				{ID: "places/ChIJ2"},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewPlacesClient("test-api-key", WithBaseURL(server.URL))
	results, err := client.SearchNearbyProbe(context.Background(), 25.033, 121.565, 500, "restaurant")
	require.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "places/ChIJ1", results[0].ID)
}

func TestSearchNearbyBasic(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.Header.Get("X-Goog-FieldMask"), "places.displayName")

		resp := SearchNearbyResponse{
			Places: []PlaceResult{
				{
					ID:               "places/ChIJ1",
					DisplayName:      &DisplayName{Text: "Test Restaurant"},
					FormattedAddress: "123 Test St",
					Location:         &LatLng{Latitude: 25.034, Longitude: 121.566},
					Types:            []string{"restaurant", "food"},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewPlacesClient("test-api-key", WithBaseURL(server.URL))
	results, err := client.SearchNearbyBasic(context.Background(), 25.033, 121.565, 500, "restaurant")
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "Test Restaurant", results[0].DisplayName.Text)
	assert.Equal(t, "123 Test St", results[0].FormattedAddress)
}

func TestSearchNearbyAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"error": {"message": "API key invalid"}}`))
	}))
	defer server.Close()

	client := NewPlacesClient("bad-key", WithBaseURL(server.URL))
	_, err := client.SearchNearbyProbe(context.Background(), 25.0, 121.0, 500, "restaurant")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}

func TestSearchNearbyEmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(SearchNearbyResponse{})
	}))
	defer server.Close()

	client := NewPlacesClient("test-key", WithBaseURL(server.URL))
	results, err := client.SearchNearbyProbe(context.Background(), 25.0, 121.0, 500, "restaurant")
	require.NoError(t, err)
	assert.Len(t, results, 0)
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/seed -run TestSearchNearby -v`
Expected: FAIL — types don't exist yet

**Step 3: Implement the Places API client**

Create `internal/seed/places_api.go`:

```go
package seed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const defaultBaseURL = "https://places.googleapis.com/v1"

// PlacesClient calls the Google Places API (New).
type PlacesClient struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

type PlacesClientOption func(*PlacesClient)

func WithBaseURL(url string) PlacesClientOption {
	return func(c *PlacesClient) { c.baseURL = url }
}

func NewPlacesClient(apiKey string, opts ...PlacesClientOption) *PlacesClient {
	c := &PlacesClient{
		apiKey:  apiKey,
		baseURL: defaultBaseURL,
		http:    http.DefaultClient,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// SearchNearbyRequest is the JSON body for searchNearby.
type SearchNearbyRequest struct {
	IncludedTypes       []string            `json:"includedTypes"`
	MaxResultCount      int                 `json:"maxResultCount"`
	LocationRestriction LocationRestriction `json:"locationRestriction"`
}

type LocationRestriction struct {
	Circle CircleRestriction `json:"circle"`
}

type CircleRestriction struct {
	Center LatLng  `json:"center"`
	Radius float64 `json:"radius"`
}

type LatLng struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// SearchNearbyResponse is the JSON response from searchNearby.
type SearchNearbyResponse struct {
	Places []PlaceResult `json:"places"`
}

type PlaceResult struct {
	ID               string       `json:"id"`
	DisplayName      *DisplayName `json:"displayName,omitempty"`
	FormattedAddress string       `json:"formattedAddress,omitempty"`
	Location         *LatLng      `json:"location,omitempty"`
	Types            []string     `json:"types,omitempty"`
}

type DisplayName struct {
	Text string `json:"text"`
}

// SearchNearbyProbe calls searchNearby with only places.id in the field mask (free).
// Returns the list of place results (ID only).
func (c *PlacesClient) SearchNearbyProbe(ctx context.Context, lat, lng, radius float64, placeType string) ([]PlaceResult, error) {
	return c.searchNearby(ctx, lat, lng, radius, placeType, "places.id")
}

// SearchNearbyBasic calls searchNearby with free Basic fields.
func (c *PlacesClient) SearchNearbyBasic(ctx context.Context, lat, lng, radius float64, placeType string) ([]PlaceResult, error) {
	return c.searchNearby(ctx, lat, lng, radius, placeType,
		"places.id,places.displayName,places.formattedAddress,places.location,places.types")
}

func (c *PlacesClient) searchNearby(ctx context.Context, lat, lng, radius float64, placeType, fieldMask string) ([]PlaceResult, error) {
	reqBody := SearchNearbyRequest{
		IncludedTypes:  []string{placeType},
		MaxResultCount: 20,
		LocationRestriction: LocationRestriction{
			Circle: CircleRestriction{
				Center: LatLng{Latitude: lat, Longitude: lng},
				Radius: radius,
			},
		},
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/places:searchNearby", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Goog-Api-Key", c.apiKey)
	req.Header.Set("X-Goog-FieldMask", fieldMask)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("searchNearby returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result SearchNearbyResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result.Places, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/seed -run TestSearchNearby -v`
Expected: All PASS

**Step 5: Commit**

```bash
git add internal/seed/places_api.go internal/seed/places_api_test.go
git commit -m "Add Google Places API client with probe and basic search"
```

---

### Task 5: Grid sweep orchestrator

**Files:**
- Create: `internal/seed/discover.go`
- Test: `internal/seed/discover_test.go`

**Step 1: Write the failing test**

Create `internal/seed/discover_test.go`:

```go
package seed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverSweep(t *testing.T) {
	var mu sync.Mutex
	var calls []SearchNearbyRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body SearchNearbyRequest
		json.NewDecoder(r.Body).Decode(&body)

		mu.Lock()
		calls = append(calls, body)
		mu.Unlock()

		fieldMask := r.Header.Get("X-Goog-FieldMask")

		var resp SearchNearbyResponse
		if fieldMask == "places.id" {
			// Probe: return 3 results (not saturated)
			resp = SearchNearbyResponse{
				Places: []PlaceResult{
					{ID: "places/ChIJ1"},
					{ID: "places/ChIJ2"},
					{ID: "places/ChIJ3"},
				},
			}
		} else {
			// Basic: return full data
			resp = SearchNearbyResponse{
				Places: []PlaceResult{
					{ID: "places/ChIJ1", DisplayName: &DisplayName{Text: "Place 1"}, Types: []string{"restaurant"}, Location: &LatLng{Latitude: 25.0, Longitude: 121.0}},
					{ID: "places/ChIJ2", DisplayName: &DisplayName{Text: "Place 2"}, Types: []string{"restaurant"}, Location: &LatLng{Latitude: 25.1, Longitude: 121.1}},
					{ID: "places/ChIJ3", DisplayName: &DisplayName{Text: "Place 3"}, Types: []string{"restaurant"}, Location: &LatLng{Latitude: 25.2, Longitude: 121.2}},
				},
			}
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewPlacesClient("test-key", WithBaseURL(server.URL))

	// Use a very small radius so we get few grid cells
	results, stats, err := DiscoverSweep(context.Background(), client, SweepConfig{
		CenterLat: 25.0,
		CenterLng: 121.0,
		Radius:    100,
		SubRadius: 100,
		PlaceType: "restaurant",
	})
	require.NoError(t, err)
	assert.Greater(t, len(results), 0)
	assert.Greater(t, stats.CellsProbed, 0)
	assert.Greater(t, stats.NewPlaces, 0)
}

func TestDiscoverSweepSaturation(t *testing.T) {
	callCount := 0
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		currentCall := callCount
		mu.Unlock()

		fieldMask := r.Header.Get("X-Goog-FieldMask")
		var resp SearchNearbyResponse

		if fieldMask == "places.id" && currentCall == 1 {
			// First probe: saturated (20 results)
			places := make([]PlaceResult, 20)
			for i := range places {
				places[i] = PlaceResult{ID: fmt.Sprintf("places/ChIJ_%d", i)}
			}
			resp = SearchNearbyResponse{Places: places}
		} else if fieldMask == "places.id" {
			// Sub-cell probes: not saturated
			resp = SearchNearbyResponse{
				Places: []PlaceResult{{ID: "places/ChIJ_sub"}},
			}
		} else {
			// Basic fetch
			resp = SearchNearbyResponse{
				Places: []PlaceResult{
					{ID: "places/ChIJ_sub", DisplayName: &DisplayName{Text: "Sub Place"}, Types: []string{"restaurant"}},
				},
			}
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewPlacesClient("test-key", WithBaseURL(server.URL))

	_, stats, err := DiscoverSweep(context.Background(), client, SweepConfig{
		CenterLat: 25.0,
		CenterLng: 121.0,
		Radius:    50,
		SubRadius: 50,
		PlaceType: "restaurant",
	})
	require.NoError(t, err)
	// Should have subdivided, so more than 1 cell probed
	assert.Greater(t, stats.CellsProbed, 1)
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/seed -run TestDiscoverSweep -v`
Expected: FAIL — `DiscoverSweep` doesn't exist

**Step 3: Implement the sweep orchestrator**

Create `internal/seed/discover.go`:

```go
package seed

import (
	"context"
	"fmt"
	"strings"
)

const maxResults = 20

// SweepConfig holds parameters for a grid sweep.
type SweepConfig struct {
	CenterLat float64
	CenterLng float64
	Radius    float64
	SubRadius float64
	PlaceType string
}

// SweepStats tracks sweep progress.
type SweepStats struct {
	CellsProbed int
	NewPlaces   int
	Duplicates  int
}

// DiscoverSweep runs a grid sweep for a single place type, returning deduplicated results.
func DiscoverSweep(ctx context.Context, client *PlacesClient, cfg SweepConfig) ([]PlaceResult, SweepStats, error) {
	grid := GenerateGridPoints(cfg.CenterLat, cfg.CenterLng, cfg.Radius, cfg.SubRadius)

	seen := make(map[string]bool)
	var allResults []PlaceResult
	var stats SweepStats

	for _, point := range grid {
		results, err := sweepCell(ctx, client, GridCell{Point: point, Radius: cfg.SubRadius}, cfg.PlaceType, seen, &stats)
		if err != nil {
			return nil, stats, err
		}
		allResults = append(allResults, results...)
	}

	return allResults, stats, nil
}

func sweepCell(ctx context.Context, client *PlacesClient, cell GridCell, placeType string, seen map[string]bool, stats *SweepStats) ([]PlaceResult, error) {
	// Probe (free)
	probeResults, err := client.SearchNearbyProbe(ctx, cell.Point.Lat, cell.Point.Lng, cell.Radius, placeType)
	if err != nil {
		return nil, fmt.Errorf("probe at (%.4f, %.4f): %w", cell.Point.Lat, cell.Point.Lng, err)
	}
	stats.CellsProbed++

	if len(probeResults) == 0 {
		return nil, nil
	}

	// Saturated — subdivide
	if len(probeResults) >= maxResults {
		fmt.Printf("  Cell (%.4f, %.4f) r=%.0fm: saturated (%d), subdividing\n",
			cell.Point.Lat, cell.Point.Lng, cell.Radius, len(probeResults))

		var allResults []PlaceResult
		children := SubdivideCell(cell.Point, cell.Radius)
		for _, child := range children {
			results, err := sweepCell(ctx, client, child, placeType, seen, stats)
			if err != nil {
				return nil, err
			}
			allResults = append(allResults, results...)
		}
		return allResults, nil
	}

	// Not saturated — fetch basic fields
	basicResults, err := client.SearchNearbyBasic(ctx, cell.Point.Lat, cell.Point.Lng, cell.Radius, placeType)
	if err != nil {
		return nil, fmt.Errorf("basic fetch at (%.4f, %.4f): %w", cell.Point.Lat, cell.Point.Lng, err)
	}

	var newResults []PlaceResult
	for _, r := range basicResults {
		id := normalizeID(r.ID)
		if seen[id] {
			stats.Duplicates++
			continue
		}
		seen[id] = true
		stats.NewPlaces++
		newResults = append(newResults, r)
	}

	fmt.Printf("  Cell (%.4f, %.4f) r=%.0fm: %d places (%d new, %d dupes)\n",
		cell.Point.Lat, cell.Point.Lng, cell.Radius, len(basicResults), len(newResults), len(basicResults)-len(newResults))

	return newResults, nil
}

// normalizeID strips the "places/" prefix from the Google Place ID.
func normalizeID(id string) string {
	return strings.TrimPrefix(id, "places/")
}
```

**Step 4: Add missing import to test file**

The saturation test uses `fmt.Sprintf` — add `"fmt"` to its imports.

**Step 5: Run tests to verify they pass**

Run: `go test ./internal/seed -run TestDiscoverSweep -v`
Expected: All PASS

**Step 6: Commit**

```bash
git add internal/seed/discover.go internal/seed/discover_test.go
git commit -m "Add grid sweep orchestrator with saturation subdivision"
```

---

### Task 6: CLI entry point

**Files:**
- Create: `cmd/seed/main.go`

**Step 1: Implement the CLI**

Create `cmd/seed/main.go`:

```go
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/pinnertw/query/internal/seed"

	db "github.com/pinnertw/query/internal/db/generated"
	"github.com/jackc/pgx/v5/pgtype"
)

func main() {
	lat := flag.Float64("lat", 0, "Center latitude (required)")
	lng := flag.Float64("lng", 0, "Center longitude (required)")
	radius := flag.Float64("radius", 0, "Search radius in meters (required)")
	typesStr := flag.String("types", "", "Comma-separated place types in sweep order (required)")
	subRadius := flag.Float64("sub-radius", 0, "Grid cell radius in meters (default: radius/3)")
	apiKey := flag.String("api-key", "", "Google API key (or set GOOGLE_API_KEY env var)")
	dbURL := flag.String("db", "", "PostgreSQL connection string (or set DATABASE_URL env var)")

	flag.Parse()

	// Resolve API key
	key := *apiKey
	if key == "" {
		key = os.Getenv("GOOGLE_API_KEY")
	}
	if key == "" {
		log.Fatal("--api-key or GOOGLE_API_KEY is required")
	}

	// Resolve DB URL
	connStr := *dbURL
	if connStr == "" {
		connStr = os.Getenv("DATABASE_URL")
	}
	if connStr == "" {
		log.Fatal("--db or DATABASE_URL is required")
	}

	// Validate required flags
	if *lat == 0 && *lng == 0 {
		log.Fatal("--lat and --lng are required")
	}
	if *radius <= 0 {
		log.Fatal("--radius must be positive")
	}
	if *typesStr == "" {
		log.Fatal("--types is required")
	}

	types := strings.Split(*typesStr, ",")
	sr := *subRadius
	if sr <= 0 {
		sr = *radius / 3
	}

	ctx := context.Background()

	// Connect to DB
	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer conn.Close(ctx)

	q := db.New(conn)
	client := seed.NewPlacesClient(key)

	var totalNew, totalDupes int

	for _, placeType := range types {
		placeType = strings.TrimSpace(placeType)
		fmt.Printf("Sweeping type: %s\n", placeType)

		results, stats, err := seed.DiscoverSweep(ctx, client, seed.SweepConfig{
			CenterLat: *lat,
			CenterLng: *lng,
			Radius:    *radius,
			SubRadius: sr,
			PlaceType: placeType,
		})
		if err != nil {
			log.Fatalf("Sweep failed for type %s: %v", placeType, err)
		}

		// Store results in DB
		for _, r := range results {
			googlePlaceID := strings.TrimPrefix(r.ID, "places/")

			name := pgtype.Text{}
			if r.DisplayName != nil {
				name = pgtype.Text{String: r.DisplayName.Text, Valid: true}
			}

			address := pgtype.Text{}
			if r.FormattedAddress != "" {
				address = pgtype.Text{String: r.FormattedAddress, Valid: true}
			}

			var lat, lng pgtype.Float8
			if r.Location != nil {
				lat = pgtype.Float8{Float64: r.Location.Latitude, Valid: true}
				lng = pgtype.Float8{Float64: r.Location.Longitude, Valid: true}
			}

			dq, err := q.InsertDiscoveryQuery(ctx, db.InsertDiscoveryQueryParams{
				Latitude:    r.Location.Latitude,
				Longitude:   r.Location.Longitude,
				Radius:      sr,
				PlaceType:   placeType,
				ResultCount: int32(len(results)),
			})
			if err != nil {
				log.Printf("Warning: failed to insert discovery query: %v", err)
			}

			err = q.InsertDiscovery(ctx, db.InsertDiscoveryParams{
				QueryID:       pgtype.Int8{Int64: dq.ID, Valid: true},
				GooglePlaceID: googlePlaceID,
				Name:          name,
				Address:       address,
				Latitude:      lat,
				Longitude:     lng,
				PlaceTypes:    r.Types,
			})
			if err != nil {
				log.Printf("Warning: failed to insert discovery %s: %v", googlePlaceID, err)
			}
		}

		fmt.Printf("Type %s: %d new, %d dupes\n\n", placeType, stats.NewPlaces, stats.Duplicates)
		totalNew += stats.NewPlaces
		totalDupes += stats.Duplicates
	}

	fmt.Printf("Total: %d new places discovered, %d duplicates skipped\n", totalNew, totalDupes)
}
```

**Step 2: Verify it builds**

Run: `go build ./cmd/seed`
Expected: No errors

**Step 3: Commit**

```bash
git add cmd/seed/main.go
git commit -m "Add seed CLI for place discovery sweep"
```

---

### Task 7: Store discovery queries per grid cell (not per result)

The CLI in Task 6 stores one `discovery_query` per result, but the design calls for one per grid cell. This task refactors the orchestrator to return cell-level query info alongside results.

**Files:**
- Modify: `internal/seed/discover.go`
- Modify: `cmd/seed/main.go`

**Step 1: Add CellResult type to discover.go**

Add a `CellResult` struct that pairs a cell's query parameters with its results:

```go
// CellResult holds the results of a single grid cell sweep.
type CellResult struct {
	Lat       float64
	Lng       float64
	Radius    float64
	PlaceType string
	Places    []PlaceResult
}
```

Update `DiscoverSweep` to return `[]CellResult` instead of `[]PlaceResult`.

**Step 2: Update cmd/seed/main.go**

Store one `discovery_query` per `CellResult`, then link each place to that query.

**Step 3: Run all tests**

Run: `go test ./internal/seed/... -v && go build ./cmd/seed`
Expected: All PASS, build succeeds

**Step 4: Commit**

```bash
git add internal/seed/discover.go cmd/seed/main.go
git commit -m "Store discovery queries per grid cell instead of per result"
```

---

### Task 8: End-to-end integration test

**Files:**
- Create: `tests/seed_integration_test.go`

**Step 1: Write the integration test**

This test uses testcontainers for DB + httptest for Google API mock, and runs the full discovery flow:

```go
package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pinnertw/query/internal/db/dbtest"
	db "github.com/pinnertw/query/internal/db/generated"
	"github.com/pinnertw/query/internal/seed"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoverySweepIntegration(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()
	q := db.New(conn)

	// Mock Google API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fieldMask := r.Header.Get("X-Goog-FieldMask")
		if fieldMask == "places.id" {
			json.NewEncoder(w).Encode(seed.SearchNearbyResponse{
				Places: []seed.PlaceResult{
					{ID: "places/ChIJ_int_1"},
					{ID: "places/ChIJ_int_2"},
				},
			})
		} else {
			json.NewEncoder(w).Encode(seed.SearchNearbyResponse{
				Places: []seed.PlaceResult{
					{ID: "places/ChIJ_int_1", DisplayName: &seed.DisplayName{Text: "Integration Place 1"}, Types: []string{"restaurant"}, Location: &seed.LatLng{Latitude: 25.0, Longitude: 121.0}, FormattedAddress: "1 Test St"},
					{ID: "places/ChIJ_int_2", DisplayName: &seed.DisplayName{Text: "Integration Place 2"}, Types: []string{"restaurant", "cafe"}, Location: &seed.LatLng{Latitude: 25.1, Longitude: 121.1}, FormattedAddress: "2 Test St"},
				},
			})
		}
	}))
	defer server.Close()

	client := seed.NewPlacesClient("test-key", seed.WithBaseURL(server.URL))

	results, stats, err := seed.DiscoverSweep(ctx, client, seed.SweepConfig{
		CenterLat: 25.0,
		CenterLng: 121.0,
		Radius:    100,
		SubRadius: 100,
		PlaceType: "restaurant",
	})
	require.NoError(t, err)
	assert.Greater(t, stats.NewPlaces, 0)

	// Store in DB
	for _, r := range results {
		err := q.InsertDiscovery(ctx, db.InsertDiscoveryParams{
			GooglePlaceID: seed.NormalizeID(r.ID),
			Name:          pgtype.Text{String: r.DisplayName.Text, Valid: true},
			PlaceTypes:    r.Types,
		})
		require.NoError(t, err)
	}

	// Verify DB state
	pending, err := q.ListPendingDiscoveries(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(pending), 2)

	counts, err := q.CountDiscoveriesByStatus(ctx)
	require.NoError(t, err)
	assert.Equal(t, counts.Pending, counts.Total)
}
```

**Step 2: Export NormalizeID**

In `internal/seed/discover.go`, rename `normalizeID` to `NormalizeID` (export it).

**Step 3: Run the integration test**

Run: `go test ./tests -run TestDiscoverySweepIntegration -v`
Expected: PASS

**Step 4: Run all tests to verify nothing is broken**

Run: `go test ./... -v`
Expected: All PASS

**Step 5: Commit**

```bash
git add tests/seed_integration_test.go internal/seed/discover.go
git commit -m "Add end-to-end integration test for discovery sweep"
```
