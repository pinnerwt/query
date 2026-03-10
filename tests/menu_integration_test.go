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

func createTestRestaurant(t *testing.T, ctx context.Context, q *db.Queries) db.Restaurant {
	t.Helper()
	hash, err := auth.HashPassword("test")
	require.NoError(t, err)
	owner, err := q.CreateOwner(ctx, db.CreateOwnerParams{
		Email:        "menu-" + slug.Generate("test") + "@test.com",
		PasswordHash: hash,
		Name:         "Menu Test Owner",
	})
	require.NoError(t, err)
	rest, err := q.CreateRestaurant(ctx, db.CreateRestaurantParams{
		OwnerID: owner.ID,
		Name:    "Menu Test Restaurant",
		Slug:    slug.Generate("Menu Test"),
		DineIn:  true,
	})
	require.NoError(t, err)
	return rest
}

func TestMenuCategoryCRUD(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()
	q := db.New(conn)
	rest := createTestRestaurant(t, ctx, q)

	t.Run("create and list categories", func(t *testing.T) {
		cat1, err := q.CreateMenuCategory(ctx, db.CreateMenuCategoryParams{
			RestaurantID: rest.ID,
			Name:         "主食",
			SortOrder:    1,
		})
		require.NoError(t, err)
		assert.Equal(t, "主食", cat1.Name)
		assert.Equal(t, int32(1), cat1.SortOrder)

		_, err = q.CreateMenuCategory(ctx, db.CreateMenuCategoryParams{
			RestaurantID: rest.ID,
			Name:         "飲料",
			SortOrder:    2,
		})
		require.NoError(t, err)

		cats, err := q.ListMenuCategoriesByRestaurant(ctx, rest.ID)
		require.NoError(t, err)
		assert.Len(t, cats, 2)
		assert.Equal(t, "主食", cats[0].Name)
		assert.Equal(t, "飲料", cats[1].Name)
	})

	t.Run("update category", func(t *testing.T) {
		cat, err := q.CreateMenuCategory(ctx, db.CreateMenuCategoryParams{
			RestaurantID: rest.ID,
			Name:         "Before",
			SortOrder:    10,
		})
		require.NoError(t, err)

		updated, err := q.UpdateMenuCategory(ctx, db.UpdateMenuCategoryParams{
			ID:        cat.ID,
			Name:      "After",
			SortOrder: 20,
		})
		require.NoError(t, err)
		assert.Equal(t, "After", updated.Name)
		assert.Equal(t, int32(20), updated.SortOrder)
	})

	t.Run("delete category", func(t *testing.T) {
		cat, err := q.CreateMenuCategory(ctx, db.CreateMenuCategoryParams{
			RestaurantID: rest.ID,
			Name:         "ToDelete",
			SortOrder:    99,
		})
		require.NoError(t, err)

		err = q.DeleteMenuCategory(ctx, cat.ID)
		require.NoError(t, err)

		// Verify deleted
		cats, err := q.ListMenuCategoriesByRestaurant(ctx, rest.ID)
		require.NoError(t, err)
		for _, c := range cats {
			assert.NotEqual(t, cat.ID, c.ID)
		}
	})
}

func TestMenuItemCRUD(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()
	q := db.New(conn)
	rest := createTestRestaurant(t, ctx, q)

	cat, err := q.CreateMenuCategory(ctx, db.CreateMenuCategoryParams{
		RestaurantID: rest.ID,
		Name:         "主食",
		SortOrder:    1,
	})
	require.NoError(t, err)

	t.Run("create and get item", func(t *testing.T) {
		item, err := q.CreateMenuItem(ctx, db.CreateMenuItemParams{
			RestaurantID: rest.ID,
			CategoryID:   pgtype.Int8{Int64: cat.ID, Valid: true},
			Name:         "牛肉麵",
			Description:  pgtype.Text{String: "紅燒牛肉麵", Valid: true},
			Price:        180,
		})
		require.NoError(t, err)
		assert.Equal(t, "牛肉麵", item.Name)
		assert.Equal(t, int32(180), item.Price)
		assert.True(t, item.IsAvailable)

		got, err := q.GetMenuItemByID(ctx, item.ID)
		require.NoError(t, err)
		assert.Equal(t, item.ID, got.ID)
		assert.Equal(t, "牛肉麵", got.Name)
	})

	t.Run("update item", func(t *testing.T) {
		item, err := q.CreateMenuItem(ctx, db.CreateMenuItemParams{
			RestaurantID: rest.ID,
			CategoryID:   pgtype.Int8{Int64: cat.ID, Valid: true},
			Name:         "Original",
			Price:        100,
		})
		require.NoError(t, err)

		updated, err := q.UpdateMenuItem(ctx, db.UpdateMenuItemParams{
			ID:          item.ID,
			Name:        "Updated",
			Description: pgtype.Text{String: "new desc", Valid: true},
			Price:       150,
			CategoryID:  pgtype.Int8{Int64: cat.ID, Valid: true},
			IsAvailable: false,
		})
		require.NoError(t, err)
		assert.Equal(t, "Updated", updated.Name)
		assert.Equal(t, int32(150), updated.Price)
		assert.False(t, updated.IsAvailable)
	})

	t.Run("delete item", func(t *testing.T) {
		item, err := q.CreateMenuItem(ctx, db.CreateMenuItemParams{
			RestaurantID: rest.ID,
			CategoryID:   pgtype.Int8{Int64: cat.ID, Valid: true},
			Name:         "ToDelete",
			Price:        50,
		})
		require.NoError(t, err)

		err = q.DeleteMenuItem(ctx, item.ID)
		require.NoError(t, err)

		_, err = q.GetMenuItemByID(ctx, item.ID)
		assert.Error(t, err)
	})

	t.Run("list items by restaurant", func(t *testing.T) {
		// Clear existing
		err := q.DeleteMenuItemsByRestaurant(ctx, rest.ID)
		require.NoError(t, err)

		_, err = q.CreateMenuItem(ctx, db.CreateMenuItemParams{
			RestaurantID: rest.ID,
			CategoryID:   pgtype.Int8{Int64: cat.ID, Valid: true},
			Name:         "A Item",
			Price:        100,
		})
		require.NoError(t, err)
		_, err = q.CreateMenuItem(ctx, db.CreateMenuItemParams{
			RestaurantID: rest.ID,
			CategoryID:   pgtype.Int8{Int64: cat.ID, Valid: true},
			Name:         "B Item",
			Price:        200,
		})
		require.NoError(t, err)

		items, err := q.ListMenuItemsByRestaurant(ctx, rest.ID)
		require.NoError(t, err)
		assert.Len(t, items, 2)
	})

	t.Run("price tiers", func(t *testing.T) {
		item, err := q.CreateMenuItem(ctx, db.CreateMenuItemParams{
			RestaurantID: rest.ID,
			CategoryID:   pgtype.Int8{Int64: cat.ID, Valid: true},
			Name:         "Tiered Item",
			Price:        0,
		})
		require.NoError(t, err)

		_, err = q.CreatePriceTier(ctx, db.CreatePriceTierParams{
			MenuItemID: item.ID,
			Label:      "小",
			Quantity:    1,
			Price:      80,
			SortOrder:  1,
		})
		require.NoError(t, err)
		_, err = q.CreatePriceTier(ctx, db.CreatePriceTierParams{
			MenuItemID: item.ID,
			Label:      "大",
			Quantity:    1,
			Price:      120,
			SortOrder:  2,
		})
		require.NoError(t, err)

		tiers, err := q.ListPriceTiersByMenuItem(ctx, item.ID)
		require.NoError(t, err)
		assert.Len(t, tiers, 2)
		assert.Equal(t, "小", tiers[0].Label)
		assert.Equal(t, "大", tiers[1].Label)

		// Delete tiers
		err = q.DeletePriceTiersByMenuItem(ctx, item.ID)
		require.NoError(t, err)
		tiers, err = q.ListPriceTiersByMenuItem(ctx, item.ID)
		require.NoError(t, err)
		assert.Len(t, tiers, 0)
	})
}

func TestMenuBulkReplace(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()
	q := db.New(conn)
	rest := createTestRestaurant(t, ctx, q)

	// Create initial menu
	cat, err := q.CreateMenuCategory(ctx, db.CreateMenuCategoryParams{
		RestaurantID: rest.ID,
		Name:         "Old Category",
		SortOrder:    1,
	})
	require.NoError(t, err)
	_, err = q.CreateMenuItem(ctx, db.CreateMenuItemParams{
		RestaurantID: rest.ID,
		CategoryID:   pgtype.Int8{Int64: cat.ID, Valid: true},
		Name:         "Old Item",
		Price:        100,
	})
	require.NoError(t, err)

	// Bulk replace: delete all, re-insert
	err = q.DeleteMenuItemsByRestaurant(ctx, rest.ID)
	require.NoError(t, err)
	err = q.DeleteMenuCategoriesByRestaurant(ctx, rest.ID)
	require.NoError(t, err)

	newCat, err := q.CreateMenuCategory(ctx, db.CreateMenuCategoryParams{
		RestaurantID: rest.ID,
		Name:         "New Category",
		SortOrder:    1,
	})
	require.NoError(t, err)
	_, err = q.CreateMenuItem(ctx, db.CreateMenuItemParams{
		RestaurantID: rest.ID,
		CategoryID:   pgtype.Int8{Int64: newCat.ID, Valid: true},
		Name:         "New Item A",
		Price:        200,
	})
	require.NoError(t, err)
	_, err = q.CreateMenuItem(ctx, db.CreateMenuItemParams{
		RestaurantID: rest.ID,
		CategoryID:   pgtype.Int8{Int64: newCat.ID, Valid: true},
		Name:         "New Item B",
		Price:        300,
	})
	require.NoError(t, err)

	// Verify
	cats, err := q.ListMenuCategoriesByRestaurant(ctx, rest.ID)
	require.NoError(t, err)
	assert.Len(t, cats, 1)
	assert.Equal(t, "New Category", cats[0].Name)

	items, err := q.ListMenuItemsByRestaurant(ctx, rest.ID)
	require.NoError(t, err)
	assert.Len(t, items, 2)
}

func TestMenuPhotoDeleteByID(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()
	q := db.New(conn)
	rest := createTestRestaurant(t, ctx, q)

	// Create two photos
	p1, err := q.CreateMenuPhotoUpload(ctx, db.CreateMenuPhotoUploadParams{
		RestaurantID: rest.ID,
		FilePath:     "/tmp/photo1.jpg",
		FileName:     "photo1.jpg",
	})
	require.NoError(t, err)
	p2, err := q.CreateMenuPhotoUpload(ctx, db.CreateMenuPhotoUploadParams{
		RestaurantID: rest.ID,
		FilePath:     "/tmp/photo2.jpg",
		FileName:     "photo2.jpg",
	})
	require.NoError(t, err)

	// GetMenuPhotoUploadByID returns the correct photo
	got, err := q.GetMenuPhotoUploadByID(ctx, p1.ID)
	require.NoError(t, err)
	assert.Equal(t, p1.ID, got.ID)
	assert.Equal(t, "photo1.jpg", got.FileName)
	assert.Equal(t, rest.ID, got.RestaurantID)

	// Delete first photo by ID
	err = q.DeleteMenuPhotoUploadByID(ctx, p1.ID)
	require.NoError(t, err)

	// Verify only second photo remains
	photos, err := q.ListMenuPhotoUploadsByRestaurant(ctx, rest.ID)
	require.NoError(t, err)
	assert.Len(t, photos, 1)
	assert.Equal(t, p2.ID, photos[0].ID)

	// GetMenuPhotoUploadByID on deleted photo returns error
	_, err = q.GetMenuPhotoUploadByID(ctx, p1.ID)
	assert.Error(t, err)
}
