package tests

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pinnertw/query/internal/auth"
	"github.com/pinnertw/query/internal/db/dbtest"
	db "github.com/pinnertw/query/internal/db/generated"
	"github.com/pinnertw/query/internal/slug"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRestaurantCRUD(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()
	q := db.New(conn)

	// Create owner
	hash, err := auth.HashPassword("test")
	require.NoError(t, err)
	owner, err := q.CreateOwner(ctx, db.CreateOwnerParams{
		Email:        "restcrud@test.com",
		PasswordHash: hash,
		Name:         "CRUD Test",
	})
	require.NoError(t, err)

	t.Run("create restaurant with slug", func(t *testing.T) {
		s := slug.Generate("好吃牛肉麵")

		rest, err := q.CreateRestaurant(ctx, db.CreateRestaurantParams{
			OwnerID:     owner.ID,
			Name:        "好吃牛肉麵",
			Slug:        s,
			Address:     pgtype.Text{String: "台北市大安區", Valid: true},
			PhoneNumber: pgtype.Text{String: "+886212345678", Valid: true},
			DineIn:      true,
			Takeout:     true,
		})
		require.NoError(t, err)
		assert.Positive(t, rest.ID)
		assert.Equal(t, "好吃牛肉麵", rest.Name)
		assert.Equal(t, s, rest.Slug)
		assert.True(t, rest.DineIn)
		assert.True(t, rest.Takeout)
		assert.False(t, rest.Delivery)
		assert.False(t, rest.IsPublished)

		// Get by ID
		got, err := q.GetRestaurantByID(ctx, rest.ID)
		require.NoError(t, err)
		assert.Equal(t, rest.Name, got.Name)

		// Get by slug
		gotSlug, err := q.GetRestaurantBySlug(ctx, s)
		require.NoError(t, err)
		assert.Equal(t, rest.ID, gotSlug.ID)
	})

	t.Run("list by owner", func(t *testing.T) {
		// Create second restaurant
		_, err := q.CreateRestaurant(ctx, db.CreateRestaurantParams{
			OwnerID: owner.ID,
			Name:    "Second Place",
			Slug:    slug.Generate("Second Place"),
			DineIn:  true,
		})
		require.NoError(t, err)

		list, err := q.ListRestaurantsByOwner(ctx, owner.ID)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(list), 2)
	})

	t.Run("update restaurant", func(t *testing.T) {
		rest, err := q.CreateRestaurant(ctx, db.CreateRestaurantParams{
			OwnerID: owner.ID,
			Name:    "Before Update",
			Slug:    slug.Generate("Before Update"),
			DineIn:  true,
		})
		require.NoError(t, err)

		updated, err := q.UpdateRestaurant(ctx, db.UpdateRestaurantParams{
			ID:      rest.ID,
			OwnerID: owner.ID,
			Name:    "After Update",
			Address: pgtype.Text{String: "New Address", Valid: true},
			DineIn:  true,
			Takeout: true,
		})
		require.NoError(t, err)
		assert.Equal(t, "After Update", updated.Name)
		assert.Equal(t, "New Address", updated.Address.String)
		assert.True(t, updated.Takeout)
	})

	t.Run("update by wrong owner fails", func(t *testing.T) {
		rest, err := q.CreateRestaurant(ctx, db.CreateRestaurantParams{
			OwnerID: owner.ID,
			Name:    "Owner Check",
			Slug:    slug.Generate("Owner Check"),
			DineIn:  true,
		})
		require.NoError(t, err)

		// Different owner_id — should return no rows
		_, err = q.UpdateRestaurant(ctx, db.UpdateRestaurantParams{
			ID:      rest.ID,
			OwnerID: 999999,
			Name:    "Hijacked",
			DineIn:  true,
		})
		assert.Error(t, err) // no rows in result set
	})

	t.Run("delete restaurant", func(t *testing.T) {
		rest, err := q.CreateRestaurant(ctx, db.CreateRestaurantParams{
			OwnerID: owner.ID,
			Name:    "To Delete",
			Slug:    slug.Generate("To Delete"),
			DineIn:  true,
		})
		require.NoError(t, err)

		err = q.DeleteRestaurant(ctx, db.DeleteRestaurantParams{
			ID:      rest.ID,
			OwnerID: owner.ID,
		})
		require.NoError(t, err)

		_, err = q.GetRestaurantByID(ctx, rest.ID)
		assert.Error(t, err) // not found
	})

	t.Run("publish toggle", func(t *testing.T) {
		rest, err := q.CreateRestaurant(ctx, db.CreateRestaurantParams{
			OwnerID: owner.ID,
			Name:    "Publish Test",
			Slug:    slug.Generate("Publish Test"),
			DineIn:  true,
		})
		require.NoError(t, err)
		assert.False(t, rest.IsPublished)

		published, err := q.UpdateRestaurantPublished(ctx, db.UpdateRestaurantPublishedParams{
			ID:          rest.ID,
			OwnerID:     owner.ID,
			IsPublished: true,
		})
		require.NoError(t, err)
		assert.True(t, published.IsPublished)
	})

	t.Run("restaurant hours", func(t *testing.T) {
		rest, err := q.CreateRestaurant(ctx, db.CreateRestaurantParams{
			OwnerID: owner.ID,
			Name:    "Hours Test",
			Slug:    slug.Generate("Hours Test"),
			DineIn:  true,
		})
		require.NoError(t, err)

		// Set hours
		err = q.InsertRestaurantHour(ctx, db.InsertRestaurantHourParams{
			RestaurantID: rest.ID,
			DayOfWeek:    1,
			OpenTime:     pgtype.Time{Microseconds: 11 * 3600 * 1e6, Valid: true},
			CloseTime:    pgtype.Time{Microseconds: 14 * 3600 * 1e6, Valid: true},
		})
		require.NoError(t, err)

		err = q.InsertRestaurantHour(ctx, db.InsertRestaurantHourParams{
			RestaurantID: rest.ID,
			DayOfWeek:    1,
			OpenTime:     pgtype.Time{Microseconds: 17 * 3600 * 1e6, Valid: true},
			CloseTime:    pgtype.Time{Microseconds: 21 * 3600 * 1e6, Valid: true},
		})
		require.NoError(t, err)

		hours, err := q.ListRestaurantHours(ctx, rest.ID)
		require.NoError(t, err)
		assert.Len(t, hours, 2)

		// Replace hours (delete + re-insert)
		err = q.DeleteRestaurantHours(ctx, rest.ID)
		require.NoError(t, err)

		hours, err = q.ListRestaurantHours(ctx, rest.ID)
		require.NoError(t, err)
		assert.Len(t, hours, 0)
	})
}
