package tests

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/pinnertw/query/internal/db/dbtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlacesTable(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()

	// Insert a place with all columns populated
	var id int64
	err := conn.QueryRow(ctx, `
		INSERT INTO places (
			google_place_id, name, address, latitude, longitude,
			plus_code, phone_number, website, google_maps_url,
			rating, total_ratings, price_level, place_types, reservation_url
		) VALUES (
			'ChIJN1t_tDeuEmsRUsoyG83frY4', 'Test Restaurant', '123 Test St',
			25.0330, 121.5654, '8Q65+XX', '+886212345678',
			'https://example.com', 'https://maps.google.com/?cid=123',
			4.5, 150, 2, ARRAY['restaurant', 'food'], 'https://reserve.example.com'
		) RETURNING id
	`).Scan(&id)
	require.NoError(t, err)
	assert.Positive(t, id)

	// Read it back and verify all columns
	var (
		googlePlaceID  string
		name           string
		address        *string
		latitude       *float64
		longitude      *float64
		plusCode        *string
		phoneNumber    *string
		website        *string
		googleMapsURL  *string
		rating         *float64
		totalRatings   *int
		priceLevel     *int16
		placeTypes     []string
		reservationURL *string
	)
	err = conn.QueryRow(ctx, `
		SELECT google_place_id, name, address, latitude, longitude,
		       plus_code, phone_number, website, google_maps_url,
		       rating, total_ratings, price_level, place_types, reservation_url
		FROM places WHERE id = $1
	`, id).Scan(
		&googlePlaceID, &name, &address, &latitude, &longitude,
		&plusCode, &phoneNumber, &website, &googleMapsURL,
		&rating, &totalRatings, &priceLevel, &placeTypes, &reservationURL,
	)
	require.NoError(t, err)

	assert.Equal(t, "ChIJN1t_tDeuEmsRUsoyG83frY4", googlePlaceID)
	assert.Equal(t, "Test Restaurant", name)
	assert.Equal(t, "123 Test St", *address)
	assert.InDelta(t, 25.0330, *latitude, 0.0001)
	assert.InDelta(t, 121.5654, *longitude, 0.0001)
	assert.Equal(t, "8Q65+XX", *plusCode)
	assert.Equal(t, "+886212345678", *phoneNumber)
	assert.Equal(t, "https://example.com", *website)
	assert.Equal(t, "https://maps.google.com/?cid=123", *googleMapsURL)
	assert.InDelta(t, 4.5, *rating, 0.01)
	assert.Equal(t, 150, *totalRatings)
	assert.Equal(t, int16(2), *priceLevel)
	assert.Equal(t, []string{"restaurant", "food"}, placeTypes)
	assert.Equal(t, "https://reserve.example.com", *reservationURL)
}

func TestPlaceOpeningHours(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()

	// Create a place
	var placeID int64
	err := conn.QueryRow(ctx, `
		INSERT INTO places (google_place_id, name)
		VALUES ('place_hours_test', 'Hours Test Place')
		RETURNING id
	`).Scan(&placeID)
	require.NoError(t, err)

	// Insert multiple periods for the same day (lunch + dinner)
	_, err = conn.Exec(ctx, `
		INSERT INTO place_opening_hours (place_id, day_of_week, open_time, close_time)
		VALUES
			($1, 1, '11:00', '14:00'),
			($1, 1, '17:00', '21:00'),
			($1, 2, '11:00', '21:00')
	`, placeID)
	require.NoError(t, err)

	// Verify count
	var count int
	err = conn.QueryRow(ctx, `
		SELECT COUNT(*) FROM place_opening_hours WHERE place_id = $1
	`, placeID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	// Verify FK cascade on place delete
	_, err = conn.Exec(ctx, `DELETE FROM places WHERE id = $1`, placeID)
	require.NoError(t, err)

	err = conn.QueryRow(ctx, `
		SELECT COUNT(*) FROM place_opening_hours WHERE place_id = $1
	`, placeID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestPlacePhotos(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()

	// Create a place
	var placeID int64
	err := conn.QueryRow(ctx, `
		INSERT INTO places (google_place_id, name)
		VALUES ('place_photos_test', 'Photos Test Place')
		RETURNING id
	`).Scan(&placeID)
	require.NoError(t, err)

	// Insert photos
	_, err = conn.Exec(ctx, `
		INSERT INTO place_photos (place_id, google_photo_reference, url, width, height)
		VALUES
			($1, 'ref_abc123', 'https://photos.example.com/1.jpg', 800, 600),
			($1, 'ref_def456', 'https://photos.example.com/2.jpg', 1024, 768)
	`, placeID)
	require.NoError(t, err)

	// Verify count
	var count int
	err = conn.QueryRow(ctx, `
		SELECT COUNT(*) FROM place_photos WHERE place_id = $1
	`, placeID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Verify FK cascade on place delete
	_, err = conn.Exec(ctx, `DELETE FROM places WHERE id = $1`, placeID)
	require.NoError(t, err)

	err = conn.QueryRow(ctx, `
		SELECT COUNT(*) FROM place_photos WHERE place_id = $1
	`, placeID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestPlacesUniqueGoogleId(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()

	// Insert first place
	_, err := conn.Exec(ctx, `
		INSERT INTO places (google_place_id, name)
		VALUES ('unique_test_id', 'First Place')
	`)
	require.NoError(t, err)

	// Insert duplicate google_place_id — should fail
	_, err = conn.Exec(ctx, `
		INSERT INTO places (google_place_id, name)
		VALUES ('unique_test_id', 'Second Place')
	`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unique")
}

// insertPlace is a helper to create a place and return its ID.
func insertPlace(t *testing.T, conn *pgx.Conn, ctx context.Context, googlePlaceID, name string) int64 {
	t.Helper()
	var id int64
	err := conn.QueryRow(ctx, `
		INSERT INTO places (google_place_id, name)
		VALUES ($1, $2)
		RETURNING id
	`, googlePlaceID, name).Scan(&id)
	require.NoError(t, err)
	return id
}

// insertRestaurantDetails is a helper to create restaurant_details and return its ID.
func insertRestaurantDetails(t *testing.T, conn *pgx.Conn, ctx context.Context, placeID int64) int64 {
	t.Helper()
	var id int64
	err := conn.QueryRow(ctx, `
		INSERT INTO restaurant_details (place_id)
		VALUES ($1)
		RETURNING id
	`, placeID).Scan(&id)
	require.NoError(t, err)
	return id
}

// insertOwner creates an owner and returns its ID.
func insertOwner(t *testing.T, conn *pgx.Conn, ctx context.Context, email string) int64 {
	t.Helper()
	var id int64
	err := conn.QueryRow(ctx, `
		INSERT INTO owners (email, password_hash, name)
		VALUES ($1, '$2a$10$dummy', 'Test Owner')
		RETURNING id
	`, email).Scan(&id)
	require.NoError(t, err)
	return id
}

// insertRestaurant creates a restaurant (for new schema) and returns its ID.
func insertRestaurant(t *testing.T, conn *pgx.Conn, ctx context.Context, ownerID int64, name, slug string) int64 {
	t.Helper()
	var id int64
	err := conn.QueryRow(ctx, `
		INSERT INTO restaurants (owner_id, name, slug)
		VALUES ($1, $2, $3)
		RETURNING id
	`, ownerID, name, slug).Scan(&id)
	require.NoError(t, err)
	return id
}

func TestRestaurantDetails(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()

	placeID := insertPlace(t, conn, ctx, "rest_details_test", "Details Test Place")

	// Insert restaurant_details with all fields
	var id int64
	err := conn.QueryRow(ctx, `
		INSERT INTO restaurant_details (
			place_id, minimum_spend, time_limit_minutes,
			dine_in, takeout, delivery
		) VALUES ($1, 200, 90, TRUE, TRUE, FALSE)
		RETURNING id
	`, placeID).Scan(&id)
	require.NoError(t, err)
	assert.Positive(t, id)

	// Read back and verify
	var (
		minSpend  *int
		timeLimit *int
		dineIn    bool
		takeout   bool
		delivery  bool
	)
	err = conn.QueryRow(ctx, `
		SELECT minimum_spend, time_limit_minutes, dine_in, takeout, delivery
		FROM restaurant_details WHERE id = $1
	`, id).Scan(&minSpend, &timeLimit, &dineIn, &takeout, &delivery)
	require.NoError(t, err)
	assert.Equal(t, 200, *minSpend)
	assert.Equal(t, 90, *timeLimit)
	assert.True(t, dineIn)
	assert.True(t, takeout)
	assert.False(t, delivery)

	// 1:1 constraint — duplicate place_id should fail
	_, err = conn.Exec(ctx, `
		INSERT INTO restaurant_details (place_id) VALUES ($1)
	`, placeID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unique")
}

func TestRestaurantHoursOverride(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()

	placeID := insertPlace(t, conn, ctx, "rest_hours_test", "Hours Override Place")
	restID := insertRestaurantDetails(t, conn, ctx, placeID)

	// Insert last_order overrides for different days
	_, err := conn.Exec(ctx, `
		INSERT INTO restaurant_hours_override (restaurant_id, day_of_week, override_type, override_time)
		VALUES
			($1, 1, 'last_order', '20:30'),
			($1, 2, 'last_order', '20:30'),
			($1, 1, 'last_entry', '20:00')
	`, restID)
	require.NoError(t, err)

	// Verify unique constraint on (restaurant_id, day_of_week, override_type)
	_, err = conn.Exec(ctx, `
		INSERT INTO restaurant_hours_override (restaurant_id, day_of_week, override_type, override_time)
		VALUES ($1, 1, 'last_order', '21:00')
	`, restID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unique")

	// Verify FK cascade — delete restaurant_details cascades to overrides
	_, err = conn.Exec(ctx, `DELETE FROM restaurant_details WHERE id = $1`, restID)
	require.NoError(t, err)

	var count int
	err = conn.QueryRow(ctx, `
		SELECT COUNT(*) FROM restaurant_hours_override WHERE restaurant_id = $1
	`, restID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestRestaurantCascadeFromPlace(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()

	placeID := insertPlace(t, conn, ctx, "rest_cascade_test", "Cascade Test Place")
	restID := insertRestaurantDetails(t, conn, ctx, placeID)

	// Delete the parent place — restaurant_details should cascade-delete
	_, err := conn.Exec(ctx, `DELETE FROM places WHERE id = $1`, placeID)
	require.NoError(t, err)

	var count int
	err = conn.QueryRow(ctx, `
		SELECT COUNT(*) FROM restaurant_details WHERE id = $1
	`, restID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestMenuCategories(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, conn, ctx, "menucat@test.com")
	restID := insertRestaurant(t, conn, ctx, ownerID, "Category Test Place", "cat-test")

	// Insert categories with sort_order
	_, err := conn.Exec(ctx, `
		INSERT INTO menu_categories (restaurant_id, name, sort_order)
		VALUES
			($1, '主餐', 1),
			($1, '飲料', 2),
			($1, '甜點', 3)
	`, restID)
	require.NoError(t, err)

	var count int
	err = conn.QueryRow(ctx, `
		SELECT COUNT(*) FROM menu_categories WHERE restaurant_id = $1
	`, restID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	// Verify FK cascade from restaurants
	_, err = conn.Exec(ctx, `DELETE FROM restaurants WHERE id = $1`, restID)
	require.NoError(t, err)

	err = conn.QueryRow(ctx, `
		SELECT COUNT(*) FROM menu_categories WHERE restaurant_id = $1
	`, restID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestMenuItems(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, conn, ctx, "menuitem@test.com")
	restID := insertRestaurant(t, conn, ctx, ownerID, "Item Test Place", "item-test")

	// Create a category
	var catID int64
	err := conn.QueryRow(ctx, `
		INSERT INTO menu_categories (restaurant_id, name)
		VALUES ($1, '主餐')
		RETURNING id
	`, restID).Scan(&catID)
	require.NoError(t, err)

	// Insert a menu item with price as integer (TWD)
	var itemID int64
	err = conn.QueryRow(ctx, `
		INSERT INTO menu_items (restaurant_id, category_id, name, description, price, photo_url)
		VALUES ($1, $2, '牛肉麵', 'Beef noodle soup', 250, 'https://photos.example.com/beef.jpg')
		RETURNING id
	`, restID, catID).Scan(&itemID)
	require.NoError(t, err)

	// Verify price stored as int
	var price int
	err = conn.QueryRow(ctx, `
		SELECT price FROM menu_items WHERE id = $1
	`, itemID).Scan(&price)
	require.NoError(t, err)
	assert.Equal(t, 250, price)

	// Verify category ON DELETE SET NULL
	_, err = conn.Exec(ctx, `DELETE FROM menu_categories WHERE id = $1`, catID)
	require.NoError(t, err)

	var categoryID *int64
	err = conn.QueryRow(ctx, `
		SELECT category_id FROM menu_items WHERE id = $1
	`, itemID).Scan(&categoryID)
	require.NoError(t, err)
	assert.Nil(t, categoryID)
}


func TestPriceTiers(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()

	ownerID := insertOwner(t, conn, ctx, "tier@test.com")
	restID := insertRestaurant(t, conn, ctx, ownerID, "Tier Test Place", "tier-test")

	// Create a menu item
	var itemID int64
	err := conn.QueryRow(ctx, `
		INSERT INTO menu_items (restaurant_id, name, price)
		VALUES ($1, '法國生蠔', 688)
		RETURNING id
	`, restID).Scan(&itemID)
	require.NoError(t, err)

	// Insert price tiers
	_, err = conn.Exec(ctx, `
		INSERT INTO menu_item_price_tiers (menu_item_id, label, quantity, price, sort_order)
		VALUES ($1, '2入', 2, 688, 0), ($1, '6入', 6, 1680, 1), ($1, '12入', 12, 3280, 2)
	`, itemID)
	require.NoError(t, err)

	// Verify tiers
	var count int
	err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM menu_item_price_tiers WHERE menu_item_id = $1`, itemID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	// Verify cascade delete
	_, err = conn.Exec(ctx, `DELETE FROM menu_items WHERE id = $1`, itemID)
	require.NoError(t, err)

	err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM menu_item_price_tiers WHERE menu_item_id = $1`, itemID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

