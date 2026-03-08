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
