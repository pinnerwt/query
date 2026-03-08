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

-- name: GetPlace :one
SELECT * FROM places WHERE id = $1;

-- name: GetPlaceByGoogleID :one
SELECT * FROM places WHERE google_place_id = $1;

-- name: ListPlacesByType :many
SELECT * FROM places WHERE @place_type::text = ANY(place_types) ORDER BY name;
