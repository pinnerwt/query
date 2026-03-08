package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pinnertw/query/internal/db/dbtest"
	db "github.com/pinnertw/query/internal/db/generated"
	"github.com/pinnertw/query/internal/seed"
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

	cellResults, stats, err := seed.DiscoverSweep(ctx, client, seed.SweepConfig{
		CenterLat: 25.0,
		CenterLng: 121.0,
		Radius:    100,
		SubRadius: 100,
		PlaceType: "restaurant",
	})
	require.NoError(t, err)
	assert.Greater(t, stats.NewPlaces, 0)

	// Store in DB — one query per cell, places linked to their cell's query
	for _, cell := range cellResults {
		dq, err := q.InsertDiscoveryQuery(ctx, db.InsertDiscoveryQueryParams{
			Latitude:    cell.Lat,
			Longitude:   cell.Lng,
			Radius:      cell.Radius,
			PlaceType:   "restaurant",
			ResultCount: int32(len(cell.Places)),
		})
		require.NoError(t, err)

		for _, r := range cell.Places {
			var lat, lng pgtype.Float8
			if r.Location != nil {
				lat = pgtype.Float8{Float64: r.Location.Latitude, Valid: true}
				lng = pgtype.Float8{Float64: r.Location.Longitude, Valid: true}
			}
			var addr pgtype.Text
			if r.FormattedAddress != "" {
				addr = pgtype.Text{String: r.FormattedAddress, Valid: true}
			}

			err := q.InsertDiscovery(ctx, db.InsertDiscoveryParams{
				QueryID:       pgtype.Int8{Int64: dq.ID, Valid: true},
				GooglePlaceID: seed.NormalizeID(r.ID),
				Name:          pgtype.Text{String: r.DisplayName.Text, Valid: true},
				Address:       addr,
				Latitude:      lat,
				Longitude:     lng,
				PlaceTypes:    r.Types,
			})
			require.NoError(t, err)
		}
	}

	// Verify DB state
	pending, err := q.ListPendingDiscoveries(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(pending), 2)

	counts, err := q.CountDiscoveriesByStatus(ctx)
	require.NoError(t, err)
	assert.Equal(t, counts.Pending, counts.Total)
}
