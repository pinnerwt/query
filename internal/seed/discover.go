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
