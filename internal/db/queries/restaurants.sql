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
