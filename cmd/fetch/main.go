package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/pinnertw/query/internal/db/generated"
	"github.com/pinnertw/query/internal/seed"
)

func main() {
	apiKey := flag.String("api-key", "", "Google API key (or set GOOGLE_API_KEY env var)")
	dbURL := flag.String("db", "", "PostgreSQL connection string (or set DATABASE_URL env var)")
	photosDir := flag.String("photos-dir", "photos", "Directory to save photos")
	maxPhotos := flag.Int("max-photos", 3, "Max photos to download per place")
	flag.Parse()

	key := *apiKey
	if key == "" {
		key = os.Getenv("GOOGLE_API_KEY")
	}
	if key == "" {
		log.Fatal("--api-key or GOOGLE_API_KEY is required")
	}

	connStr := *dbURL
	if connStr == "" {
		connStr = os.Getenv("DATABASE_URL")
	}
	if connStr == "" {
		log.Fatal("--db or DATABASE_URL is required")
	}

	ctx := context.Background()

	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer conn.Close(ctx)

	q := db.New(conn)
	client := seed.NewPlacesClient(key)

	// Load all discovery queries
	queries, err := q.ListDiscoveryQueries(ctx)
	if err != nil {
		log.Fatalf("Failed to list discovery queries: %v", err)
	}
	fmt.Printf("Found %d discovery queries to replay\n", len(queries))

	// Track which places we've already promoted (across queries)
	promoted := make(map[string]bool)

	var totalPromoted, totalPhotos int

	for i, dq := range queries {
		fmt.Printf("\n[%d/%d] Replaying query (%.4f, %.4f) r=%.0fm type=%s ...\n",
			i+1, len(queries), dq.Latitude, dq.Longitude, dq.Radius, dq.PlaceType)

		results, err := client.SearchNearbyAdvanced(ctx, dq.Latitude, dq.Longitude, dq.Radius, dq.PlaceType)
		if err != nil {
			log.Printf("  Warning: query failed: %v", err)
			continue
		}

		for _, r := range results {
			placeID := seed.NormalizeID(r.ID)
			if promoted[placeID] {
				continue
			}
			promoted[placeID] = true

			place, err := promotePlace(ctx, q, r, placeID)
			if err != nil {
				log.Printf("  Warning: failed to promote %s: %v", placeID, err)
				continue
			}

			// Opening hours
			if r.RegularOpeningHours != nil {
				if err := saveOpeningHours(ctx, q, place.ID, r.RegularOpeningHours); err != nil {
					log.Printf("  Warning: failed to save hours for %s: %v", placeID, err)
				}
			}

			// Photos
			photosDownloaded := 0
			for j, photo := range r.Photos {
				if j >= *maxPhotos {
					break
				}
				localPath, err := client.DownloadPhoto(ctx, photo.Name, *photosDir)
				if err != nil {
					log.Printf("  Warning: failed to download photo for %s: %v", placeID, err)
					continue
				}
				err = q.InsertPlacePhoto(ctx, db.InsertPlacePhotoParams{
					PlaceID:              place.ID,
					GooglePhotoReference: pgtype.Text{String: photo.Name, Valid: true},
					Url:                  pgtype.Text{String: localPath, Valid: true},
					Width:                pgtype.Int4{Int32: int32(photo.WidthPx), Valid: photo.WidthPx > 0},
					Height:               pgtype.Int4{Int32: int32(photo.HeightPx), Valid: photo.HeightPx > 0},
				})
				if err != nil {
					log.Printf("  Warning: failed to save photo record for %s: %v", placeID, err)
					continue
				}
				photosDownloaded++
			}
			totalPhotos += photosDownloaded

			name := placeID
			if r.DisplayName != nil {
				name = r.DisplayName.Text
			}
			fmt.Printf("  Promoted: %s (rating=%.1f, %d reviews, %d photos)\n",
				name, r.Rating, r.UserRatingCount, photosDownloaded)

			// Mark as fetched in staging table
			if err := q.MarkDiscoveryFetched(ctx, placeID); err != nil {
				log.Printf("  Warning: failed to mark %s as fetched: %v", placeID, err)
			}
			totalPromoted++
		}
	}

	fmt.Printf("\nDone: %d places promoted, %d photos downloaded\n", totalPromoted, totalPhotos)
}

func promotePlace(ctx context.Context, q *db.Queries, r seed.PlaceResult, placeID string) (db.Place, error) {
	name := placeID
	if r.DisplayName != nil {
		name = r.DisplayName.Text
	}

	var lat, lng pgtype.Float8
	if r.Location != nil {
		lat = pgtype.Float8{Float64: r.Location.Latitude, Valid: true}
		lng = pgtype.Float8{Float64: r.Location.Longitude, Valid: true}
	}

	var rating pgtype.Numeric
	if r.Rating > 0 {
		// Convert float to numeric: rating is like 4.6
		rating.Valid = true
		rating.Int = big.NewInt(int64(r.Rating * 10))
		rating.Exp = -1
	}

	priceLevel := parsePriceLevel(r.PriceLevel)

	// Delete existing hours/photos before re-inserting (idempotent)
	existing, err := q.GetPlaceByGoogleID(ctx, placeID)
	if err == nil {
		_ = q.DeleteOpeningHours(ctx, existing.ID)
		_ = q.DeletePlacePhotos(ctx, existing.ID)
	}

	return q.UpsertPlace(ctx, db.UpsertPlaceParams{
		GooglePlaceID: placeID,
		Name:          name,
		Address:       pgtype.Text{String: r.FormattedAddress, Valid: r.FormattedAddress != ""},
		Latitude:      lat,
		Longitude:     lng,
		PhoneNumber:   pgtype.Text{String: r.NationalPhoneNumber, Valid: r.NationalPhoneNumber != ""},
		Website:       pgtype.Text{String: r.WebsiteURI, Valid: r.WebsiteURI != ""},
		GoogleMapsUrl: pgtype.Text{String: r.GoogleMapsURI, Valid: r.GoogleMapsURI != ""},
		Rating:        rating,
		TotalRatings:  pgtype.Int4{Int32: int32(r.UserRatingCount), Valid: r.UserRatingCount > 0},
		PriceLevel:    priceLevel,
		PlaceTypes:    r.Types,
	})
}

func saveOpeningHours(ctx context.Context, q *db.Queries, placeID int64, hours *seed.OpeningHours) error {
	for _, p := range hours.Periods {
		openTime := pgtype.Time{
			Microseconds: int64(p.Open.Hour)*3600000000 + int64(p.Open.Minute)*60000000,
			Valid:        true,
		}
		closeTime := pgtype.Time{
			Microseconds: int64(p.Close.Hour)*3600000000 + int64(p.Close.Minute)*60000000,
			Valid:        true,
		}
		err := q.InsertOpeningHour(ctx, db.InsertOpeningHourParams{
			PlaceID:   placeID,
			DayOfWeek: int16(p.Open.Day),
			OpenTime:  openTime,
			CloseTime: closeTime,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func parsePriceLevel(s string) pgtype.Int2 {
	switch s {
	case "PRICE_LEVEL_FREE":
		return pgtype.Int2{Int16: 0, Valid: true}
	case "PRICE_LEVEL_INEXPENSIVE":
		return pgtype.Int2{Int16: 1, Valid: true}
	case "PRICE_LEVEL_MODERATE":
		return pgtype.Int2{Int16: 2, Valid: true}
	case "PRICE_LEVEL_EXPENSIVE":
		return pgtype.Int2{Int16: 3, Valid: true}
	case "PRICE_LEVEL_VERY_EXPENSIVE":
		return pgtype.Int2{Int16: 4, Valid: true}
	default:
		return pgtype.Int2{}
	}
}

// priceLevelString converts price level int back to display string.
func priceLevelString(level pgtype.Int2) string {
	if !level.Valid {
		return ""
	}
	symbols := strings.Repeat("$", int(level.Int16)+1)
	return symbols
}
