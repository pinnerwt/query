-- name: CreateRestaurantDetails :one
INSERT INTO restaurant_details (
    place_id, minimum_spend, time_limit_minutes,
    dine_in, takeout, delivery
) VALUES (
    @place_id, @minimum_spend, @time_limit_minutes,
    @dine_in, @takeout, @delivery
) RETURNING *;

-- name: GetRestaurantDetailsByPlaceID :one
SELECT * FROM restaurant_details WHERE place_id = $1;
