-- +goose Up

CREATE TABLE restaurant_details (
    id                 BIGSERIAL PRIMARY KEY,
    place_id           BIGINT NOT NULL UNIQUE REFERENCES places(id) ON DELETE CASCADE,
    minimum_spend      INTEGER,
    time_limit_minutes INTEGER,
    dine_in            BOOLEAN NOT NULL DEFAULT TRUE,
    takeout            BOOLEAN NOT NULL DEFAULT FALSE,
    delivery           BOOLEAN NOT NULL DEFAULT FALSE,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE restaurant_hours_override (
    id             BIGSERIAL PRIMARY KEY,
    restaurant_id  BIGINT NOT NULL REFERENCES restaurant_details(id) ON DELETE CASCADE,
    day_of_week    SMALLINT NOT NULL CHECK (day_of_week BETWEEN 0 AND 6),
    override_type  TEXT NOT NULL,
    override_time  TIME NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (restaurant_id, day_of_week, override_type)
);

CREATE INDEX idx_restaurant_hours_override_restaurant_id ON restaurant_hours_override(restaurant_id);

-- +goose Down

DROP TABLE IF EXISTS restaurant_hours_override;
DROP TABLE IF EXISTS restaurant_details;
