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
