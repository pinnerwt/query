-- name: CreateRestaurant :one
INSERT INTO restaurants (
    owner_id, name, slug, address, phone_number, website,
    dine_in, takeout, delivery, minimum_spend
) VALUES (
    @owner_id, @name, @slug, @address, @phone_number, @website,
    @dine_in, @takeout, @delivery, @minimum_spend
) RETURNING *;

-- name: GetRestaurantByID :one
SELECT * FROM restaurants WHERE id = $1;

-- name: GetRestaurantBySlug :one
SELECT * FROM restaurants WHERE slug = $1;

-- name: ListRestaurantsByOwner :many
SELECT * FROM restaurants WHERE owner_id = $1 ORDER BY created_at DESC;

-- name: UpdateRestaurant :one
UPDATE restaurants SET
    name = @name,
    address = @address,
    phone_number = @phone_number,
    website = @website,
    dine_in = @dine_in,
    takeout = @takeout,
    delivery = @delivery,
    minimum_spend = @minimum_spend,
    updated_at = NOW()
WHERE id = @id AND owner_id = @owner_id
RETURNING *;

-- name: DeleteRestaurant :exec
DELETE FROM restaurants WHERE id = @id AND owner_id = @owner_id;

-- name: UpdateRestaurantPublished :one
UPDATE restaurants SET is_published = @is_published, updated_at = NOW()
WHERE id = @id AND owner_id = @owner_id
RETURNING *;

-- name: InsertRestaurantHour :exec
INSERT INTO restaurant_hours (restaurant_id, day_of_week, open_time, close_time)
VALUES (@restaurant_id, @day_of_week, @open_time, @close_time);

-- name: DeleteRestaurantHours :exec
DELETE FROM restaurant_hours WHERE restaurant_id = $1;

-- name: ListRestaurantHours :many
SELECT * FROM restaurant_hours WHERE restaurant_id = $1 ORDER BY day_of_week, open_time;

-- name: GetRestaurantByGooglePlaceID :one
SELECT * FROM restaurants WHERE google_place_id = $1;

-- name: GetPublishedRestaurantBySlug :one
SELECT * FROM restaurants WHERE slug = $1 AND is_published = TRUE;

-- name: UpdateRestaurantLocation :exec
UPDATE restaurants SET location = ST_MakePoint(@longitude::float8, @latitude::float8)::geography, updated_at = NOW()
WHERE id = @id AND owner_id = @owner_id;

-- name: GetRestaurantLocation :one
SELECT ST_Y(location::geometry) AS latitude, ST_X(location::geometry) AS longitude
FROM restaurants WHERE id = $1 AND location IS NOT NULL;
