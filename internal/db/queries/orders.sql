-- name: CreateOrder :one
INSERT INTO orders (restaurant_id, table_label, total_amount)
VALUES (@restaurant_id, @table_label, @total_amount)
RETURNING *;

-- name: CreateOrderItem :one
INSERT INTO order_items (order_id, menu_item_id, item_name, quantity, unit_price, notes)
VALUES (@order_id, @menu_item_id, @item_name, @quantity, @unit_price, @notes)
RETURNING *;

-- name: GetOrderByID :one
SELECT * FROM orders WHERE id = $1;

-- name: ListOrdersByRestaurant :many
SELECT * FROM orders WHERE restaurant_id = @restaurant_id
AND (@status::text = '' OR status = @status)
ORDER BY created_at DESC;

-- name: UpdateOrderStatus :one
UPDATE orders SET status = @status, updated_at = NOW()
WHERE id = @id
RETURNING *;

-- name: ListOrderItemsByOrder :many
SELECT * FROM order_items WHERE order_id = $1 ORDER BY id;
