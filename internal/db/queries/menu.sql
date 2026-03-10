-- name: DeleteMenuItemsByRestaurant :exec
DELETE FROM menu_items WHERE restaurant_id = $1;

-- name: DeleteMenuCategoriesByRestaurant :exec
DELETE FROM menu_categories WHERE restaurant_id = $1;

-- name: CreateMenuCategory :one
INSERT INTO menu_categories (restaurant_id, name, sort_order)
VALUES (@restaurant_id, @name, @sort_order)
RETURNING *;

-- name: ListMenuCategoriesByRestaurant :many
SELECT * FROM menu_categories WHERE restaurant_id = $1 ORDER BY sort_order;

-- name: CreateMenuItem :one
INSERT INTO menu_items (restaurant_id, category_id, name, description, price, photo_url)
VALUES (@restaurant_id, @category_id, @name, @description, @price, @photo_url)
RETURNING *;

-- name: ListMenuItemsByRestaurant :many
SELECT * FROM menu_items WHERE restaurant_id = $1 ORDER BY name;

-- name: UpdateMenuItemPrice :one
UPDATE menu_items SET price = @price, updated_at = NOW()
WHERE id = @id
RETURNING *;

-- name: CreatePriceTier :one
INSERT INTO menu_item_price_tiers (menu_item_id, label, quantity, price, sort_order)
VALUES (@menu_item_id, @label, @quantity, @price, @sort_order)
RETURNING *;

-- name: ListPriceTiersByMenuItem :many
SELECT * FROM menu_item_price_tiers WHERE menu_item_id = $1 ORDER BY sort_order;

-- name: ListPriceTiersByRestaurant :many
SELECT pt.* FROM menu_item_price_tiers pt
JOIN menu_items mi ON mi.id = pt.menu_item_id
WHERE mi.restaurant_id = $1
ORDER BY pt.menu_item_id, pt.sort_order;

-- name: DeletePriceTiersByMenuItem :exec
DELETE FROM menu_item_price_tiers WHERE menu_item_id = $1;

-- name: GetMenuItemByID :one
SELECT * FROM menu_items WHERE id = $1;

-- name: UpdateMenuCategory :one
UPDATE menu_categories SET name = @name, sort_order = @sort_order
WHERE id = @id
RETURNING *;

-- name: DeleteMenuCategory :exec
DELETE FROM menu_categories WHERE id = $1;

-- name: UpdateMenuItem :one
UPDATE menu_items SET
    name = @name,
    description = @description,
    price = @price,
    category_id = @category_id,
    is_available = @is_available,
    updated_at = NOW()
WHERE id = @id
RETURNING *;

-- name: DeleteMenuItem :exec
DELETE FROM menu_items WHERE id = $1;

-- name: CreateOptionGroup :one
INSERT INTO menu_item_option_groups (menu_item_id, name, min_choices, max_choices, sort_order)
VALUES (@menu_item_id, @name, @min_choices, @max_choices, @sort_order)
RETURNING *;

-- name: ListOptionGroupsByMenuItem :many
SELECT * FROM menu_item_option_groups WHERE menu_item_id = $1 ORDER BY sort_order;

-- name: ListOptionGroupsByRestaurant :many
SELECT og.* FROM menu_item_option_groups og
JOIN menu_items mi ON mi.id = og.menu_item_id
WHERE mi.restaurant_id = $1
ORDER BY og.menu_item_id, og.sort_order;

-- name: DeleteOptionGroupsByMenuItem :exec
DELETE FROM menu_item_option_groups WHERE menu_item_id = $1;

-- name: CreateOptionChoice :one
INSERT INTO menu_item_option_choices (group_id, name, price_adjustment, sort_order)
VALUES (@group_id, @name, @price_adjustment, @sort_order)
RETURNING *;

-- name: ListOptionChoicesByGroup :many
SELECT * FROM menu_item_option_choices WHERE group_id = $1 ORDER BY sort_order;

-- name: ListOptionChoicesByRestaurant :many
SELECT oc.* FROM menu_item_option_choices oc
JOIN menu_item_option_groups og ON og.id = oc.group_id
JOIN menu_items mi ON mi.id = og.menu_item_id
WHERE mi.restaurant_id = $1
ORDER BY oc.group_id, oc.sort_order;
