-- name: InsertDiscoveryQuery :one
INSERT INTO discovery_queries (latitude, longitude, radius, place_type, result_count)
VALUES (@latitude, @longitude, @radius, @place_type, @result_count)
RETURNING *;

-- name: InsertDiscovery :exec
INSERT INTO place_discoveries (query_id, google_place_id, name, address, latitude, longitude, place_types)
VALUES (@query_id, @google_place_id, @name, @address, @latitude, @longitude, @place_types)
ON CONFLICT (google_place_id) DO NOTHING;

-- name: ListPendingDiscoveries :many
SELECT * FROM place_discoveries WHERE status = 'pending' ORDER BY discovered_at;

-- name: MarkDiscoveryFetched :exec
UPDATE place_discoveries SET status = 'fetched' WHERE google_place_id = $1;

-- name: ListDiscoveryQueries :many
SELECT * FROM discovery_queries ORDER BY id;

-- name: CountDiscoveriesByStatus :one
SELECT
    COUNT(*) FILTER (WHERE status = 'pending') AS pending,
    COUNT(*) FILTER (WHERE status = 'fetched') AS fetched,
    COUNT(*) AS total
FROM place_discoveries;

-- name: ListDiscoveredPlaceIDs :many
SELECT google_place_id FROM place_discoveries;

-- name: ListCompletedGridCells :many
SELECT DISTINCT latitude, longitude, radius, place_type
FROM discovery_queries;
