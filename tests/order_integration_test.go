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

func TestOrderCRUD(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()
	q := db.New(conn)
	rest := createTestRestaurant(t, ctx, q)

	// Create a menu item to reference in order
	cat, err := q.CreateMenuCategory(ctx, db.CreateMenuCategoryParams{
		RestaurantID: rest.ID,
		Name:         "Test",
		SortOrder:    1,
	})
	require.NoError(t, err)
	item, err := q.CreateMenuItem(ctx, db.CreateMenuItemParams{
		RestaurantID: rest.ID,
		CategoryID:   pgtype.Int8{Int64: cat.ID, Valid: true},
		Name:         "牛肉麵",
		Price:        180,
	})
	require.NoError(t, err)

	t.Run("create order with items", func(t *testing.T) {
		order, err := q.CreateOrder(ctx, db.CreateOrderParams{
			RestaurantID: rest.ID,
			TableLabel:   pgtype.Text{String: "A1", Valid: true},
			TotalAmount:  540,
		})
		require.NoError(t, err)
		assert.Positive(t, order.ID)
		assert.Equal(t, "pending", order.Status)
		assert.Equal(t, "A1", order.TableLabel.String)
		assert.Equal(t, int32(540), order.TotalAmount)

		// Add items
		oi1, err := q.CreateOrderItem(ctx, db.CreateOrderItemParams{
			OrderID:    order.ID,
			MenuItemID: pgtype.Int8{Int64: item.ID, Valid: true},
			ItemName:   "牛肉麵",
			Quantity:   3,
			UnitPrice:  180,
		})
		require.NoError(t, err)
		assert.Equal(t, "牛肉麵", oi1.ItemName)
		assert.Equal(t, int32(3), oi1.Quantity)

		// List items
		items, err := q.ListOrderItemsByOrder(ctx, order.ID)
		require.NoError(t, err)
		assert.Len(t, items, 1)
		assert.Equal(t, "牛肉麵", items[0].ItemName)
	})

	t.Run("get order by ID", func(t *testing.T) {
		order, err := q.CreateOrder(ctx, db.CreateOrderParams{
			RestaurantID: rest.ID,
			TotalAmount:  100,
		})
		require.NoError(t, err)

		got, err := q.GetOrderByID(ctx, order.ID)
		require.NoError(t, err)
		assert.Equal(t, order.ID, got.ID)
		assert.Equal(t, "pending", got.Status)
	})

	t.Run("update order status", func(t *testing.T) {
		order, err := q.CreateOrder(ctx, db.CreateOrderParams{
			RestaurantID: rest.ID,
			TotalAmount:  200,
		})
		require.NoError(t, err)

		// pending → confirmed
		updated, err := q.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
			ID:     order.ID,
			Status: "confirmed",
		})
		require.NoError(t, err)
		assert.Equal(t, "confirmed", updated.Status)

		// confirmed → preparing
		updated, err = q.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
			ID:     order.ID,
			Status: "preparing",
		})
		require.NoError(t, err)
		assert.Equal(t, "preparing", updated.Status)

		// preparing → completed
		updated, err = q.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
			ID:     order.ID,
			Status: "completed",
		})
		require.NoError(t, err)
		assert.Equal(t, "completed", updated.Status)
	})

	t.Run("list orders by restaurant", func(t *testing.T) {
		// Create a few orders
		_, err := q.CreateOrder(ctx, db.CreateOrderParams{
			RestaurantID: rest.ID,
			TotalAmount:  100,
		})
		require.NoError(t, err)
		o2, err := q.CreateOrder(ctx, db.CreateOrderParams{
			RestaurantID: rest.ID,
			TotalAmount:  200,
		})
		require.NoError(t, err)
		_, err = q.UpdateOrderStatus(ctx, db.UpdateOrderStatusParams{
			ID:     o2.ID,
			Status: "confirmed",
		})
		require.NoError(t, err)

		// List all
		all, err := q.ListOrdersByRestaurant(ctx, db.ListOrdersByRestaurantParams{
			RestaurantID: rest.ID,
			Status:       "",
		})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(all), 2)

		// Filter by status
		pending, err := q.ListOrdersByRestaurant(ctx, db.ListOrdersByRestaurantParams{
			RestaurantID: rest.ID,
			Status:       "pending",
		})
		require.NoError(t, err)
		for _, o := range pending {
			assert.Equal(t, "pending", o.Status)
		}

		confirmed, err := q.ListOrdersByRestaurant(ctx, db.ListOrdersByRestaurantParams{
			RestaurantID: rest.ID,
			Status:       "confirmed",
		})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(confirmed), 1)
		for _, o := range confirmed {
			assert.Equal(t, "confirmed", o.Status)
		}
	})

	t.Run("order with notes", func(t *testing.T) {
		order, err := q.CreateOrder(ctx, db.CreateOrderParams{
			RestaurantID: rest.ID,
			TotalAmount:  180,
		})
		require.NoError(t, err)

		oi, err := q.CreateOrderItem(ctx, db.CreateOrderItemParams{
			OrderID:    order.ID,
			MenuItemID: pgtype.Int8{Int64: item.ID, Valid: true},
			ItemName:   "牛肉麵",
			Quantity:   1,
			UnitPrice:  180,
			Notes:      pgtype.Text{String: "不要香菜", Valid: true},
		})
		require.NoError(t, err)
		assert.Equal(t, "不要香菜", oi.Notes.String)
	})

	t.Run("order without menu item reference", func(t *testing.T) {
		order, err := q.CreateOrder(ctx, db.CreateOrderParams{
			RestaurantID: rest.ID,
			TotalAmount:  50,
		})
		require.NoError(t, err)

		// Order item without menu_item_id (e.g., custom item)
		oi, err := q.CreateOrderItem(ctx, db.CreateOrderItemParams{
			OrderID:   order.ID,
			ItemName:  "Extra Rice",
			Quantity:  1,
			UnitPrice: 50,
		})
		require.NoError(t, err)
		assert.False(t, oi.MenuItemID.Valid)
	})
}
