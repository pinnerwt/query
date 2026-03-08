package tests

import (
	"context"
	"testing"

	"github.com/pinnertw/query/internal/db/dbtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiscoveryQueriesTable(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()

	var id int64
	err := conn.QueryRow(ctx, `
		INSERT INTO discovery_queries (latitude, longitude, radius, place_type, result_count)
		VALUES (25.033, 121.565, 500.0, 'restaurant', 15)
		RETURNING id
	`).Scan(&id)
	require.NoError(t, err)
	assert.Positive(t, id)

	var lat, lng, radius float64
	var placeType string
	var resultCount int
	err = conn.QueryRow(ctx, `
		SELECT latitude, longitude, radius, place_type, result_count
		FROM discovery_queries WHERE id = $1
	`, id).Scan(&lat, &lng, &radius, &placeType, &resultCount)
	require.NoError(t, err)
	assert.InDelta(t, 25.033, lat, 0.001)
	assert.InDelta(t, 121.565, lng, 0.001)
	assert.InDelta(t, 500.0, radius, 0.1)
	assert.Equal(t, "restaurant", placeType)
	assert.Equal(t, 15, resultCount)
}

func TestPlaceDiscoveriesTable(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()

	// Insert a discovery query
	var queryID int64
	err := conn.QueryRow(ctx, `
		INSERT INTO discovery_queries (latitude, longitude, radius, place_type)
		VALUES (25.033, 121.565, 500.0, 'restaurant')
		RETURNING id
	`).Scan(&queryID)
	require.NoError(t, err)

	// Insert a discovery
	var id int64
	err = conn.QueryRow(ctx, `
		INSERT INTO place_discoveries (query_id, google_place_id, name, address, latitude, longitude, place_types, status)
		VALUES ($1, 'ChIJtest123', 'Test Place', '123 Test St', 25.033, 121.565, ARRAY['restaurant', 'food'], 'pending')
		RETURNING id
	`, queryID).Scan(&id)
	require.NoError(t, err)
	assert.Positive(t, id)

	// Verify data
	var googlePlaceID, name, status string
	var placeTypes []string
	err = conn.QueryRow(ctx, `
		SELECT google_place_id, name, status, place_types
		FROM place_discoveries WHERE id = $1
	`, id).Scan(&googlePlaceID, &name, &status, &placeTypes)
	require.NoError(t, err)
	assert.Equal(t, "ChIJtest123", googlePlaceID)
	assert.Equal(t, "Test Place", name)
	assert.Equal(t, "pending", status)
	assert.Equal(t, []string{"restaurant", "food"}, placeTypes)
}

func TestPlaceDiscoveriesUniqueGoogleID(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()

	_, err := conn.Exec(ctx, `
		INSERT INTO place_discoveries (google_place_id, name)
		VALUES ('unique_disco_test', 'First')
	`)
	require.NoError(t, err)

	_, err = conn.Exec(ctx, `
		INSERT INTO place_discoveries (google_place_id, name)
		VALUES ('unique_disco_test', 'Second')
	`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unique")
}

func TestPlaceDiscoveriesQueryFKSetNull(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()

	var queryID int64
	err := conn.QueryRow(ctx, `
		INSERT INTO discovery_queries (latitude, longitude, radius, place_type)
		VALUES (25.0, 121.0, 500.0, 'restaurant')
		RETURNING id
	`).Scan(&queryID)
	require.NoError(t, err)

	var discoID int64
	err = conn.QueryRow(ctx, `
		INSERT INTO place_discoveries (query_id, google_place_id, name)
		VALUES ($1, 'fk_test_place', 'FK Test')
		RETURNING id
	`, queryID).Scan(&discoID)
	require.NoError(t, err)

	// Delete the query — discovery should remain with NULL query_id
	_, err = conn.Exec(ctx, `DELETE FROM discovery_queries WHERE id = $1`, queryID)
	require.NoError(t, err)

	var nullQueryID *int64
	err = conn.QueryRow(ctx, `
		SELECT query_id FROM place_discoveries WHERE id = $1
	`, discoID).Scan(&nullQueryID)
	require.NoError(t, err)
	assert.Nil(t, nullQueryID)
}
