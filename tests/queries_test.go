package tests

import (
	"context"
	"testing"

	"math/big"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/pinnertw/query/internal/db/dbtest"
	db "github.com/pinnertw/query/internal/db/generated"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateAndGetPlace(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()
	q := db.New(conn)

	// Create a place via sqlc query
	place, err := q.CreatePlace(ctx, db.CreatePlaceParams{
		GooglePlaceID:  "sqlc_test_place_1",
		Name:           "SQLC Test Restaurant",
		Address:        pgtype.Text{String: "456 Query St", Valid: true},
		Latitude:       pgtype.Float8{Float64: 25.0410, Valid: true},
		Longitude:      pgtype.Float8{Float64: 121.5300, Valid: true},
		PlusCode:       pgtype.Text{String: "8Q65+AB", Valid: true},
		PhoneNumber:    pgtype.Text{String: "+886298765432", Valid: true},
		Website:        pgtype.Text{String: "https://sqlc-test.example.com", Valid: true},
		GoogleMapsUrl:  pgtype.Text{String: "https://maps.google.com/?cid=456", Valid: true},
		Rating:         pgtype.Numeric{Valid: true, Int: big.NewInt(43), Exp: -1},
		TotalRatings:   pgtype.Int4{Int32: 200, Valid: true},
		PriceLevel:     pgtype.Int2{Int16: 3, Valid: true},
		PlaceTypes:     []string{"restaurant", "bar"},
		ReservationUrl: pgtype.Text{String: "https://reserve.sqlc-test.com", Valid: true},
	})
	require.NoError(t, err)
	assert.Positive(t, place.ID)
	assert.Equal(t, "SQLC Test Restaurant", place.Name)

	// Get by ID
	got, err := q.GetPlace(ctx, place.ID)
	require.NoError(t, err)
	assert.Equal(t, place.GooglePlaceID, got.GooglePlaceID)
	assert.Equal(t, place.Name, got.Name)

	// Get by google_place_id
	got2, err := q.GetPlaceByGoogleID(ctx, "sqlc_test_place_1")
	require.NoError(t, err)
	assert.Equal(t, place.ID, got2.ID)
}

func TestListPlacesByType(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()
	q := db.New(conn)

	// Insert places with different types
	_, err := q.CreatePlace(ctx, db.CreatePlaceParams{
		GooglePlaceID: "type_test_1",
		Name:          "Restaurant A",
		PlaceTypes:    []string{"restaurant", "food"},
	})
	require.NoError(t, err)

	_, err = q.CreatePlace(ctx, db.CreatePlaceParams{
		GooglePlaceID: "type_test_2",
		Name:          "Cafe B",
		PlaceTypes:    []string{"cafe", "food"},
	})
	require.NoError(t, err)

	_, err = q.CreatePlace(ctx, db.CreatePlaceParams{
		GooglePlaceID: "type_test_3",
		Name:          "Bar C",
		PlaceTypes:    []string{"bar"},
	})
	require.NoError(t, err)

	// Query by type "food" — should return 2
	places, err := q.ListPlacesByType(ctx, "food")
	require.NoError(t, err)
	assert.Len(t, places, 2)

	// Query by type "bar" — should return 1
	places, err = q.ListPlacesByType(ctx, "bar")
	require.NoError(t, err)
	assert.Len(t, places, 1)
	assert.Equal(t, "Bar C", places[0].Name)
}

func TestCreateRestaurantWithMenu(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()
	q := db.New(conn)

	// Create place
	place, err := q.CreatePlace(ctx, db.CreatePlaceParams{
		GooglePlaceID: "full_flow_test",
		Name:          "Full Flow Restaurant",
	})
	require.NoError(t, err)

	// Create restaurant_details
	rest, err := q.CreateRestaurantDetails(ctx, db.CreateRestaurantDetailsParams{
		PlaceID:          place.ID,
		MinimumSpend:     pgtype.Int4{Int32: 150, Valid: true},
		TimeLimitMinutes: pgtype.Int4{Int32: 90, Valid: true},
		DineIn:           true,
		Takeout:          true,
		Delivery:         false,
	})
	require.NoError(t, err)
	assert.Positive(t, rest.ID)

	// Create category
	cat, err := q.CreateMenuCategory(ctx, db.CreateMenuCategoryParams{
		RestaurantID: rest.ID,
		Name:         "主餐",
		SortOrder:    1,
	})
	require.NoError(t, err)

	// Create menu item
	item, err := q.CreateMenuItem(ctx, db.CreateMenuItemParams{
		RestaurantID: rest.ID,
		CategoryID:   pgtype.Int8{Int64: cat.ID, Valid: true},
		Name:         "牛肉麵",
		Description:  pgtype.Text{String: "Beef noodle soup", Valid: true},
		Price:        250,
	})
	require.NoError(t, err)
	assert.Equal(t, int32(250), item.Price)

	// Create combo
	combo, err := q.CreateComboMeal(ctx, db.CreateComboMealParams{
		RestaurantID: rest.ID,
		Name:         "午間套餐",
		Description:  pgtype.Text{String: "Lunch combo", Valid: true},
		Price:        350,
	})
	require.NoError(t, err)

	// Create combo group
	group, err := q.CreateComboMealGroup(ctx, db.CreateComboMealGroupParams{
		ComboMealID: combo.ID,
		Name:        "選主餐",
		MinChoices:  1,
		MaxChoices:  1,
		SortOrder:   1,
	})
	require.NoError(t, err)

	// Create combo group option referencing menu item
	_, err = q.CreateComboMealGroupOption(ctx, db.CreateComboMealGroupOptionParams{
		GroupID:         group.ID,
		MenuItemID:      pgtype.Int8{Int64: item.ID, Valid: true},
		PriceAdjustment: 0,
		SortOrder:       1,
	})
	require.NoError(t, err)

	// Create add-on
	addon, err := q.CreateAddOn(ctx, db.CreateAddOnParams{
		RestaurantID: rest.ID,
		Name:         "加蛋",
		Price:        15,
	})
	require.NoError(t, err)
	assert.Equal(t, "加蛋", addon.Name)

	// Query full restaurant menu
	items, err := q.ListMenuItemsByRestaurant(ctx, rest.ID)
	require.NoError(t, err)
	assert.Len(t, items, 1)

	addons, err := q.ListAddOnsByRestaurant(ctx, rest.ID)
	require.NoError(t, err)
	assert.Len(t, addons, 1)
}

func TestUpdateMenuItemPrice(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()
	q := db.New(conn)

	// Setup
	place, err := q.CreatePlace(ctx, db.CreatePlaceParams{
		GooglePlaceID: "update_price_test",
		Name:          "Price Update Place",
	})
	require.NoError(t, err)

	rest, err := q.CreateRestaurantDetails(ctx, db.CreateRestaurantDetailsParams{
		PlaceID: place.ID,
		DineIn:  true,
	})
	require.NoError(t, err)

	item, err := q.CreateMenuItem(ctx, db.CreateMenuItemParams{
		RestaurantID: rest.ID,
		Name:         "拉麵",
		Price:        200,
	})
	require.NoError(t, err)
	assert.Equal(t, int32(200), item.Price)

	// Update price
	updated, err := q.UpdateMenuItemPrice(ctx, db.UpdateMenuItemPriceParams{
		ID:    item.ID,
		Price: 280,
	})
	require.NoError(t, err)
	assert.Equal(t, int32(280), updated.Price)
}
