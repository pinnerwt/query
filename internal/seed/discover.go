package seed

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const maxResults = 20

// SweepConfig holds parameters for a grid sweep.
type SweepConfig struct {
	CenterLat float64
	CenterLng float64
	Radius    float64
	SubRadius float64
	PlaceType string
	MaxDepth  int // Maximum subdivision depth (default 12 if 0)
}

// SweepOpts holds optional parameters for resumable sweeps.
type SweepOpts struct {
	// Seen is a pre-populated set of place IDs to skip (for dedup across resumes).
	Seen map[string]bool
	// OnCellDone is called after each top-level grid cell completes.
	// gridIndex is the index into the grid array, saturated indicates if any sub-cell hit max depth.
	OnCellDone func(gridIndex int, results []CellResult, saturated bool)
	// SkipGridCells is the set of top-level grid indices to skip (already completed).
	SkipGridCells map[int]bool
}

const defaultMaxDepth = 23

// SweepStats tracks sweep progress.
type SweepStats struct {
	CellsProbed    int
	NewPlaces      int
	Duplicates     int
	SaturatedCells int
}

// CellResult holds the results of a single grid cell sweep.
type CellResult struct {
	Lat    float64
	Lng    float64
	Radius float64
	Places []PlaceResult
}

// DiscoverSweep runs a grid sweep for a single place type, returning deduplicated results per cell.
func DiscoverSweep(ctx context.Context, client *PlacesClient, cfg SweepConfig, opts *SweepOpts) ([]CellResult, SweepStats, error) {
	grid := GenerateGridPoints(cfg.CenterLat, cfg.CenterLng, cfg.Radius, cfg.SubRadius)

	maxDepth := cfg.MaxDepth
	if maxDepth <= 0 {
		maxDepth = defaultMaxDepth
	}

	seen := make(map[string]bool)
	var skipCells map[int]bool
	var onCellDone func(int, []CellResult, bool)

	if opts != nil {
		for k, v := range opts.Seen {
			seen[k] = v
		}
		skipCells = opts.SkipGridCells
		onCellDone = opts.OnCellDone
	}

	var allResults []CellResult
	var stats SweepStats

	// Rate limit to 8 req/s to stay under 600 req/min quota.
	throttle := time.Tick(125 * time.Millisecond)

	for i, point := range grid {
		if skipCells != nil && skipCells[i] {
			fmt.Printf("  Grid cell %d/%d (%.4f, %.4f): skipped (checkpoint)\n", i+1, len(grid), point.Lat, point.Lng)
			continue
		}

		fmt.Printf("  Grid cell %d/%d (%.4f, %.4f) r=%.0fm\n", i+1, len(grid), point.Lat, point.Lng, cfg.SubRadius)
		results, saturated, err := sweepCell(ctx, client, GridCell{Point: point, Radius: cfg.SubRadius}, cfg.PlaceType, seen, &stats, 0, maxDepth, throttle)
		if err != nil {
			return allResults, stats, err
		}
		allResults = append(allResults, results...)

		if onCellDone != nil {
			onCellDone(i, results, saturated)
		}
	}

	return allResults, stats, nil
}

// sweepCell returns (results, saturated, error). saturated is true if any sub-cell hit max depth.
func sweepCell(ctx context.Context, client *PlacesClient, cell GridCell, placeType string, seen map[string]bool, stats *SweepStats, depth, maxDepth int, throttle <-chan time.Time) ([]CellResult, bool, error) {
	// Rate limit: wait for throttle tick
	<-throttle
	// Probe (free)
	probeResults, err := client.SearchNearbyProbe(ctx, cell.Point.Lat, cell.Point.Lng, cell.Radius, placeType)
	if err != nil {
		return nil, false, fmt.Errorf("probe at (%.4f, %.4f): %w", cell.Point.Lat, cell.Point.Lng, err)
	}
	stats.CellsProbed++

	if len(probeResults) == 0 {
		return nil, false, nil
	}

	// Saturated — subdivide if we haven't hit max depth
	if len(probeResults) >= maxResults && depth < maxDepth {
		fmt.Printf("    Cell (%.4f, %.4f) r=%.0fm: saturated (%d), subdividing (depth %d)\n",
			cell.Point.Lat, cell.Point.Lng, cell.Radius, len(probeResults), depth)

		var allResults []CellResult
		anySaturated := false
		children := SubdivideCell(cell.Point, cell.Radius)
		for _, child := range children {
			results, sat, err := sweepCell(ctx, client, child, placeType, seen, stats, depth+1, maxDepth, throttle)
			if err != nil {
				return nil, false, err
			}
			allResults = append(allResults, results...)
			if sat {
				anySaturated = true
			}
		}
		return allResults, anySaturated, nil
	}

	if len(probeResults) >= maxResults {
		fmt.Printf("    Cell (%.4f, %.4f) r=%.0fm: saturated (%d), max depth %d reached, fetching top %d\n",
			cell.Point.Lat, cell.Point.Lng, cell.Radius, len(probeResults), maxDepth, maxResults)
		stats.SaturatedCells++
	}

	// Not saturated (or max depth reached) — fetch basic fields
	<-throttle
	basicResults, err := client.SearchNearbyBasic(ctx, cell.Point.Lat, cell.Point.Lng, cell.Radius, placeType)
	if err != nil {
		return nil, false, fmt.Errorf("basic fetch at (%.4f, %.4f): %w", cell.Point.Lat, cell.Point.Lng, err)
	}

	var newResults []PlaceResult
	for _, r := range basicResults {
		id := NormalizeID(r.ID)
		if seen[id] {
			stats.Duplicates++
			continue
		}
		seen[id] = true
		stats.NewPlaces++
		newResults = append(newResults, r)
	}

	fmt.Printf("    Cell (%.4f, %.4f) r=%.0fm: %d places (%d new, %d dupes)\n",
		cell.Point.Lat, cell.Point.Lng, cell.Radius, len(basicResults), len(newResults), len(basicResults)-len(newResults))

	cellResult := CellResult{
		Lat:    cell.Point.Lat,
		Lng:    cell.Point.Lng,
		Radius: cell.Radius,
		Places: newResults,
	}

	hitMaxDepth := len(probeResults) >= maxResults
	return []CellResult{cellResult}, hitMaxDepth, nil
}

// NormalizeID strips the "places/" prefix from the Google Place ID.
func NormalizeID(id string) string {
	return strings.TrimPrefix(id, "places/")
}
