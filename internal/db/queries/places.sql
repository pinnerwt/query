-- name: CreatePlace :one
INSERT INTO places (
    google_place_id, name, address, latitude, longitude,
    plus_code, phone_number, website, google_maps_url,
    rating, total_ratings, price_level, place_types, reservation_url
) VALUES (
    @google_place_id, @name, @address, @latitude, @longitude,
    @plus_code, @phone_number, @website, @google_maps_url,
    @rating, @total_ratings, @price_level, @place_types, @reservation_url
) RETURNING *;

-- name: ListAllPlaces :many
SELECT * FROM places ORDER BY name;

-- name: GetPlace :one
SELECT * FROM places WHERE id = $1;

-- name: GetPlaceByGoogleID :one
SELECT * FROM places WHERE google_place_id = $1;

-- name: ListPlacesByType :many
SELECT * FROM places WHERE @place_type::text = ANY(place_types) ORDER BY name;

-- name: UpsertPlace :one
INSERT INTO places (
    google_place_id, name, address, latitude, longitude,
    phone_number, website, google_maps_url,
    rating, total_ratings, price_level, place_types
) VALUES (
    @google_place_id, @name, @address, @latitude, @longitude,
    @phone_number, @website, @google_maps_url,
    @rating, @total_ratings, @price_level, @place_types
)
ON CONFLICT (google_place_id) DO UPDATE SET
    name = EXCLUDED.name,
    address = EXCLUDED.address,
    latitude = EXCLUDED.latitude,
    longitude = EXCLUDED.longitude,
    phone_number = EXCLUDED.phone_number,
    website = EXCLUDED.website,
    google_maps_url = EXCLUDED.google_maps_url,
    rating = EXCLUDED.rating,
    total_ratings = EXCLUDED.total_ratings,
    price_level = EXCLUDED.price_level,
    place_types = EXCLUDED.place_types,
    updated_at = NOW()
RETURNING *;

-- name: InsertOpeningHour :exec
INSERT INTO place_opening_hours (place_id, day_of_week, open_time, close_time)
VALUES (@place_id, @day_of_week, @open_time, @close_time);

-- name: DeleteOpeningHours :exec
DELETE FROM place_opening_hours WHERE place_id = $1;

-- name: InsertPlacePhoto :exec
INSERT INTO place_photos (place_id, google_photo_reference, url, width, height)
VALUES (@place_id, @google_photo_reference, @url, @width, @height);

-- name: DeletePlacePhotos :exec
DELETE FROM place_photos WHERE place_id = $1;
