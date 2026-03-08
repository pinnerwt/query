package main

import (
	"context"
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

func main() {
	lat := flag.Float64("lat", math.NaN(), "Center latitude (required)")
	lng := flag.Float64("lng", math.NaN(), "Center longitude (required)")
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

		cellResults, stats, err := seed.DiscoverSweep(ctx, client, seed.SweepConfig{
			CenterLat: *lat,
			CenterLng: *lng,
			Radius:    *radius,
			SubRadius: sr,
			PlaceType: placeType,
		})
		if err != nil {
			log.Fatalf("Sweep failed for type %s: %v", placeType, err)
		}

		// Store results in DB — one discovery_query per cell
		for _, cell := range cellResults {
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

		fmt.Printf("Type %s: %d new, %d dupes\n\n", placeType, stats.NewPlaces, stats.Duplicates)
		totalNew += stats.NewPlaces
		totalDupes += stats.Duplicates
	}

	fmt.Printf("Total: %d new places discovered, %d duplicates skipped\n", totalNew, totalDupes)
}
