-- +goose Up

CREATE TABLE discovery_queries (
    id           BIGSERIAL PRIMARY KEY,
    latitude     DOUBLE PRECISION NOT NULL,
    longitude    DOUBLE PRECISION NOT NULL,
    radius       DOUBLE PRECISION NOT NULL,
    place_type   TEXT NOT NULL,
    result_count INTEGER NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE place_discoveries (
    id              BIGSERIAL PRIMARY KEY,
    query_id        BIGINT REFERENCES discovery_queries(id) ON DELETE SET NULL,
    google_place_id TEXT NOT NULL UNIQUE,
    name            TEXT,
    address         TEXT,
    latitude        DOUBLE PRECISION,
    longitude       DOUBLE PRECISION,
    place_types     TEXT[] DEFAULT '{}',
    status          TEXT NOT NULL DEFAULT 'pending',
    discovered_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_place_discoveries_status ON place_discoveries(status);
CREATE INDEX idx_place_discoveries_query_id ON place_discoveries(query_id);

-- +goose Down

DROP TABLE IF EXISTS place_discoveries;
DROP TABLE IF EXISTS discovery_queries;
