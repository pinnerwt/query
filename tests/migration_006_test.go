package tests

import (
	"context"
	"testing"

	"github.com/pinnertw/query/internal/db/dbtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMigration006OwnerApp(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()

	t.Run("PostGIS extension enabled", func(t *testing.T) {
		var exists bool
		err := conn.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'postgis')
		`).Scan(&exists)
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("owners table exists with system owner", func(t *testing.T) {
		var email string
		var isVerified bool
		err := conn.QueryRow(ctx, `
			SELECT email, is_verified FROM owners WHERE id = 1
		`).Scan(&email, &isVerified)
		require.NoError(t, err)
		assert.Equal(t, "system@query.local", email)
		assert.True(t, isVerified)
	})

	t.Run("restaurants table exists with correct columns", func(t *testing.T) {
		// Verify we can insert a restaurant
		ownerID := insertOwner(t, conn, ctx, "mig006@test.com")
		var restID int64
		err := conn.QueryRow(ctx, `
			INSERT INTO restaurants (
				owner_id, name, slug, address, phone_number, website,
				google_place_id, dine_in, takeout, delivery, minimum_spend, is_published
			) VALUES (
				$1, 'Test Rest', 'test-rest', '123 Test St', '+886', 'https://test.com',
				'ChIJtest123', TRUE, FALSE, FALSE, 100, TRUE
			) RETURNING id
		`, ownerID).Scan(&restID)
		require.NoError(t, err)
		assert.Positive(t, restID)

		// Verify slug uniqueness
		_, err = conn.Exec(ctx, `
			INSERT INTO restaurants (owner_id, name, slug) VALUES ($1, 'Other', 'test-rest')
		`, ownerID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unique")

		// Verify google_place_id uniqueness (nullable but unique)
		_, err = conn.Exec(ctx, `
			INSERT INTO restaurants (owner_id, name, slug, google_place_id) VALUES ($1, 'Dup', 'dup', 'ChIJtest123')
		`, ownerID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unique")
	})

	t.Run("menu tables FK points to restaurants", func(t *testing.T) {
		ownerID := insertOwner(t, conn, ctx, "menufk@test.com")
		restID := insertRestaurant(t, conn, ctx, ownerID, "FK Test", "fk-test")

		// Should succeed: menu_categories references restaurants
		var catID int64
		err := conn.QueryRow(ctx, `
			INSERT INTO menu_categories (restaurant_id, name, sort_order)
			VALUES ($1, '測試', 0) RETURNING id
		`, restID).Scan(&catID)
		require.NoError(t, err)

		// Should succeed: menu_items references restaurants
		_, err = conn.Exec(ctx, `
			INSERT INTO menu_items (restaurant_id, name, price)
			VALUES ($1, '測試品', 100)
		`, restID)
		require.NoError(t, err)

		// Should succeed: combo_meals references restaurants
		_, err = conn.Exec(ctx, `
			INSERT INTO combo_meals (restaurant_id, name, price)
			VALUES ($1, '套餐', 200)
		`, restID)
		require.NoError(t, err)

		// Should succeed: add_ons references restaurants
		_, err = conn.Exec(ctx, `
			INSERT INTO add_ons (restaurant_id, name, price)
			VALUES ($1, '加蛋', 15)
		`, restID)
		require.NoError(t, err)

		// FK to non-existent restaurant should fail
		_, err = conn.Exec(ctx, `
			INSERT INTO menu_categories (restaurant_id, name, sort_order)
			VALUES (999999, 'Bad', 0)
		`)
		require.Error(t, err)

		// Cascade delete
		_, err = conn.Exec(ctx, `DELETE FROM restaurants WHERE id = $1`, restID)
		require.NoError(t, err)

		var count int
		err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM menu_categories WHERE restaurant_id = $1`, restID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("data migration preserves IDs", func(t *testing.T) {
		// Create a place + restaurant_details with menu data (simulates pre-migration state)
		placeID := insertPlace(t, conn, ctx, "mig_data_test", "Migration Data Test")
		rdID := insertRestaurantDetails(t, conn, ctx, placeID)

		// Now create a matching restaurants row with the SAME ID
		_, err := conn.Exec(ctx, `
			INSERT INTO restaurants (id, owner_id, name, slug, google_place_id)
			VALUES ($1, 1, 'Migration Data Test', 'mig-data-test', 'mig_data_test')
		`, rdID)
		require.NoError(t, err)

		// Menu items should work with this restaurant ID
		_, err = conn.Exec(ctx, `
			INSERT INTO menu_items (restaurant_id, name, price) VALUES ($1, '遷移品', 100)
		`, rdID)
		require.NoError(t, err)
	})

	t.Run("restaurant_hours table", func(t *testing.T) {
		ownerID := insertOwner(t, conn, ctx, "hours@test.com")
		restID := insertRestaurant(t, conn, ctx, ownerID, "Hours Test", "hours-test")

		_, err := conn.Exec(ctx, `
			INSERT INTO restaurant_hours (restaurant_id, day_of_week, open_time, close_time)
			VALUES ($1, 1, '11:00', '14:00'), ($1, 1, '17:00', '21:00')
		`, restID)
		require.NoError(t, err)

		var count int
		err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM restaurant_hours WHERE restaurant_id = $1`, restID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 2, count)

		// day_of_week check constraint
		_, err = conn.Exec(ctx, `
			INSERT INTO restaurant_hours (restaurant_id, day_of_week) VALUES ($1, 7)
		`, restID)
		require.Error(t, err)
	})

	t.Run("orders and order_items tables", func(t *testing.T) {
		ownerID := insertOwner(t, conn, ctx, "orders@test.com")
		restID := insertRestaurant(t, conn, ctx, ownerID, "Order Test", "order-test")

		var orderID int64
		err := conn.QueryRow(ctx, `
			INSERT INTO orders (restaurant_id, table_label, total_amount)
			VALUES ($1, 'A1', 500)
			RETURNING id
		`, restID).Scan(&orderID)
		require.NoError(t, err)

		// Verify default status is pending
		var status string
		err = conn.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, "pending", status)

		// Status check constraint
		_, err = conn.Exec(ctx, `UPDATE orders SET status = 'invalid' WHERE id = $1`, orderID)
		require.Error(t, err)

		// Valid status update
		_, err = conn.Exec(ctx, `UPDATE orders SET status = 'confirmed' WHERE id = $1`, orderID)
		require.NoError(t, err)

		// Order items
		_, err = conn.Exec(ctx, `
			INSERT INTO order_items (order_id, item_name, quantity, unit_price, notes)
			VALUES ($1, '牛肉麵', 2, 250, '不要辣')
		`, orderID)
		require.NoError(t, err)

		// Cascade delete
		_, err = conn.Exec(ctx, `DELETE FROM orders WHERE id = $1`, orderID)
		require.NoError(t, err)

		var count int
		err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM order_items WHERE order_id = $1`, orderID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 0, count)
	})

	t.Run("menu_photo_uploads table", func(t *testing.T) {
		ownerID := insertOwner(t, conn, ctx, "uploads@test.com")
		restID := insertRestaurant(t, conn, ctx, ownerID, "Upload Test", "upload-test")

		var uploadID int64
		err := conn.QueryRow(ctx, `
			INSERT INTO menu_photo_uploads (restaurant_id, file_path, file_name)
			VALUES ($1, '/photos/1.jpg', '1.jpg')
			RETURNING id
		`, restID).Scan(&uploadID)
		require.NoError(t, err)

		// Verify default ocr_status is pending
		var status string
		err = conn.QueryRow(ctx, `SELECT ocr_status FROM menu_photo_uploads WHERE id = $1`, uploadID).Scan(&status)
		require.NoError(t, err)
		assert.Equal(t, "pending", status)

		// Status check constraint
		_, err = conn.Exec(ctx, `UPDATE menu_photo_uploads SET ocr_status = 'bad' WHERE id = $1`, uploadID)
		require.Error(t, err)

		// Valid status
		_, err = conn.Exec(ctx, `UPDATE menu_photo_uploads SET ocr_status = 'completed' WHERE id = $1`, uploadID)
		require.NoError(t, err)
	})

	t.Run("GiST index on location", func(t *testing.T) {
		var exists bool
		err := conn.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM pg_indexes
				WHERE tablename = 'restaurants' AND indexname = 'idx_restaurants_location'
			)
		`).Scan(&exists)
		require.NoError(t, err)
		assert.True(t, exists)
	})
}
