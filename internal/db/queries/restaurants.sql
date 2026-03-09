-- name: CreateRestaurantDetails :one
INSERT INTO restaurant_details (
    place_id, minimum_spend, time_limit_minutes,
    dine_in, takeout, delivery
) VALUES (
    @place_id, @minimum_spend, @time_limit_minutes,
    @dine_in, @takeout, @delivery
) RETURNING *;

-- name: UpsertRestaurantDetails :one
INSERT INTO restaurant_details (place_id)
VALUES (@place_id)
ON CONFLICT (place_id) DO UPDATE SET updated_at = NOW()
RETURNING *;

-- name: GetRestaurantDetailsByPlaceID :one
SELECT * FROM restaurant_details WHERE place_id = $1;

-- name: ListRestaurantsWithMenus :many
SELECT p.id AS place_id, p.google_place_id, p.name, p.address, rd.id AS restaurant_id
FROM places p
JOIN restaurant_details rd ON rd.place_id = p.id
WHERE EXISTS (SELECT 1 FROM menu_items mi WHERE mi.restaurant_id = rd.id)
ORDER BY p.name;
