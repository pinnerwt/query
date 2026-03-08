-- +goose Up

CREATE TABLE places (
    id              BIGSERIAL PRIMARY KEY,
    google_place_id TEXT NOT NULL UNIQUE,
    name            TEXT NOT NULL,
    address         TEXT,
    latitude        DOUBLE PRECISION,
    longitude       DOUBLE PRECISION,
    plus_code       TEXT,
    phone_number    TEXT,
    website         TEXT,
    google_maps_url TEXT,
    rating          NUMERIC(2,1),
    total_ratings   INTEGER DEFAULT 0,
    price_level     SMALLINT,
    place_types     TEXT[] DEFAULT '{}',
    reservation_url TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE place_opening_hours (
    id          BIGSERIAL PRIMARY KEY,
    place_id    BIGINT NOT NULL REFERENCES places(id) ON DELETE CASCADE,
    day_of_week SMALLINT NOT NULL CHECK (day_of_week BETWEEN 0 AND 6),
    open_time   TIME,
    close_time  TIME,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_place_opening_hours_place_id ON place_opening_hours(place_id);

CREATE TABLE place_photos (
    id                     BIGSERIAL PRIMARY KEY,
    place_id               BIGINT NOT NULL REFERENCES places(id) ON DELETE CASCADE,
    google_photo_reference TEXT,
    url                    TEXT,
    width                  INTEGER,
    height                 INTEGER,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_place_photos_place_id ON place_photos(place_id);

-- +goose Down

DROP TABLE IF EXISTS place_photos;
DROP TABLE IF EXISTS place_opening_hours;
DROP TABLE IF EXISTS places;
