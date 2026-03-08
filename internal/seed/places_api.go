package seed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
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
		http:    &http.Client{Timeout: 30 * time.Second},
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
