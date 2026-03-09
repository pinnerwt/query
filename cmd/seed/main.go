package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/pinnertw/query/internal/db/generated"
	"github.com/pinnertw/query/internal/seed"
)

// checkpoint stores resume state per place type.
type checkpoint struct {
	CompletedGridCells map[int]bool `json:"completed_grid_cells"`
}

func loadCheckpoint(path string) (map[string]checkpoint, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]checkpoint), nil
		}
		return nil, err
	}
	var cp map[string]checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, err
	}
	return cp, nil
}

func saveCheckpoint(path string, cp map[string]checkpoint) error {
	data, err := json.Marshal(cp)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func main() {
	lat := flag.Float64("lat", math.NaN(), "Center latitude (required)")
	lng := flag.Float64("lng", math.NaN(), "Center longitude (required)")
	radius := flag.Float64("radius", 0, "Search radius in meters (required)")
	typesStr := flag.String("types", "", "Comma-separated place types in sweep order (required)")
	subRadius := flag.Float64("sub-radius", 0, "Grid cell radius in meters (default: radius/3, max 50000)")
	apiKey := flag.String("api-key", "", "Google API key (or set GOOGLE_API_KEY env var)")
	dbURL := flag.String("db", "", "PostgreSQL connection string (or set DATABASE_URL env var)")
	lang := flag.String("lang", "zh-TW", "Language code for API results")
	checkpointFile := flag.String("checkpoint", "seed_checkpoint.json", "Checkpoint file for resuming sweeps")

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
	if math.IsNaN(*lat) || math.IsNaN(*lng) {
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
	if sr > 50000 {
		sr = 50000
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
	client.SetLanguage(*lang)

	// Load checkpoint
	cp, err := loadCheckpoint(*checkpointFile)
	if err != nil {
		log.Fatalf("Failed to load checkpoint: %v", err)
	}

	// Preload seen place IDs from DB for dedup
	existingIDs, err := q.ListDiscoveredPlaceIDs(ctx)
	if err != nil {
		log.Fatalf("Failed to load existing discoveries: %v", err)
	}
	seen := make(map[string]bool, len(existingIDs))
	for _, id := range existingIDs {
		seen[id] = true
	}
	fmt.Printf("Loaded %d existing place IDs from DB, checkpoint from %s\n", len(seen), *checkpointFile)

	var totalNew, totalDupes int

	for _, placeType := range types {
		placeType = strings.TrimSpace(placeType)
		fmt.Printf("\nSweeping type: %s\n", placeType)

		typeCp, exists := cp[placeType]
		if !exists {
			typeCp = checkpoint{CompletedGridCells: make(map[int]bool)}
		}
		if len(typeCp.CompletedGridCells) > 0 {
			fmt.Printf("  Resuming: %d grid cells already completed\n", len(typeCp.CompletedGridCells))
		}

		// Callback: save results to DB and update checkpoint after each grid cell
		onCellDone := func(gridIndex int, results []seed.CellResult, saturated bool) {
			for _, cell := range results {
				dq, err := q.InsertDiscoveryQuery(ctx, db.InsertDiscoveryQueryParams{
					Latitude:    cell.Lat,
					Longitude:   cell.Lng,
					Radius:      cell.Radius,
					PlaceType:   placeType,
					ResultCount: int32(len(cell.Places)),
				})
				if err != nil {
					log.Printf("Warning: failed to insert discovery query: %v", err)
					continue
				}

				for _, r := range cell.Places {
					googlePlaceID := seed.NormalizeID(r.ID)

					name := pgtype.Text{}
					if r.DisplayName != nil {
						name = pgtype.Text{String: r.DisplayName.Text, Valid: true}
					}

					address := pgtype.Text{}
					if r.FormattedAddress != "" {
						address = pgtype.Text{String: r.FormattedAddress, Valid: true}
					}

					var placeLat, placeLng pgtype.Float8
					if r.Location != nil {
						placeLat = pgtype.Float8{Float64: r.Location.Latitude, Valid: true}
						placeLng = pgtype.Float8{Float64: r.Location.Longitude, Valid: true}
					}

					err = q.InsertDiscovery(ctx, db.InsertDiscoveryParams{
						QueryID:       pgtype.Int8{Int64: dq.ID, Valid: true},
						GooglePlaceID: googlePlaceID,
						Name:          name,
						Address:       address,
						Latitude:      placeLat,
						Longitude:     placeLng,
						PlaceTypes:    r.Types,
					})
					if err != nil {
						log.Printf("Warning: failed to insert discovery %s: %v", googlePlaceID, err)
					}
				}
			}

			// Only checkpoint if no sub-cells were saturated (all data captured)
			if saturated {
				fmt.Printf("  Grid cell %d: had saturated sub-cells, will retry on next run\n", gridIndex+1)
			} else {
				typeCp.CompletedGridCells[gridIndex] = true
				cp[placeType] = typeCp
				if err := saveCheckpoint(*checkpointFile, cp); err != nil {
					log.Printf("Warning: failed to save checkpoint: %v", err)
				}
			}
		}

		_, stats, err := seed.DiscoverSweep(ctx, client, seed.SweepConfig{
			CenterLat: *lat,
			CenterLng: *lng,
			Radius:    *radius,
			SubRadius: sr,
			PlaceType: placeType,
		}, &seed.SweepOpts{
			Seen:          seen,
			SkipGridCells: typeCp.CompletedGridCells,
			OnCellDone:    onCellDone,
		})
		if err != nil {
			log.Fatalf("Sweep failed for type %s: %v", placeType, err)
		}

		fmt.Printf("Type %s: %d new, %d dupes\n", placeType, stats.NewPlaces, stats.Duplicates)
		totalNew += stats.NewPlaces
		totalDupes += stats.Duplicates
	}

	fmt.Printf("\nTotal: %d new places discovered, %d duplicates skipped\n", totalNew, totalDupes)
}
