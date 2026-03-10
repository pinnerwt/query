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

-- name: CreateComboMeal :one
INSERT INTO combo_meals (restaurant_id, name, description, price)
VALUES (@restaurant_id, @name, @description, @price)
RETURNING *;

-- name: CreateComboMealGroup :one
INSERT INTO combo_meal_groups (combo_meal_id, name, min_choices, max_choices, sort_order)
VALUES (@combo_meal_id, @name, @min_choices, @max_choices, @sort_order)
RETURNING *;

-- name: CreateComboMealGroupOption :one
INSERT INTO combo_meal_group_options (group_id, menu_item_id, item_name, price_adjustment, sort_order)
VALUES (@group_id, @menu_item_id, @item_name, @price_adjustment, @sort_order)
RETURNING *;

-- name: CreateAddOn :one
INSERT INTO add_ons (restaurant_id, name, price)
VALUES (@restaurant_id, @name, @price)
RETURNING *;

-- name: ListAddOnsByRestaurant :many
SELECT * FROM add_ons WHERE restaurant_id = $1 ORDER BY name;

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

-- name: DeleteComboMealsByRestaurant :exec
DELETE FROM combo_meals WHERE restaurant_id = $1;

-- name: ListComboMealsByRestaurant :many
SELECT * FROM combo_meals WHERE restaurant_id = $1 ORDER BY name;

-- name: ListComboMealGroupsByComboMeal :many
SELECT * FROM combo_meal_groups WHERE combo_meal_id = $1 ORDER BY sort_order;

-- name: ListComboMealGroupOptionsByGroup :many
SELECT * FROM combo_meal_group_options WHERE group_id = $1 ORDER BY sort_order;

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
