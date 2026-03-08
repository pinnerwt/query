package seed

import (
	"context"
	"encoding/json"
	"fmt"
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
	cellResults, stats, err := DiscoverSweep(context.Background(), client, SweepConfig{
		CenterLat: 25.0,
		CenterLng: 121.0,
		Radius:    100,
		SubRadius: 100,
		PlaceType: "restaurant",
	})
	require.NoError(t, err)
	assert.Greater(t, len(cellResults), 0)
	assert.Greater(t, stats.CellsProbed, 0)
	assert.Greater(t, stats.NewPlaces, 0)

	// Flatten places across all cells to verify count
	var totalPlaces int
	for _, cell := range cellResults {
		totalPlaces += len(cell.Places)
		assert.Greater(t, cell.Radius, 0.0)
	}
	assert.Greater(t, totalPlaces, 0)
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
