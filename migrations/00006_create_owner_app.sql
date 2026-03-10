-- +goose Up

-- 1. Enable PostGIS
CREATE EXTENSION IF NOT EXISTS postgis;

-- 2. Create owners table
CREATE TABLE owners (
    id            BIGSERIAL PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    name          TEXT NOT NULL DEFAULT '',
    phone         TEXT,
    is_verified   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 3. Insert system owner for migrated data
INSERT INTO owners (id, email, password_hash, name, is_verified)
VALUES (1, 'system@query.local', '', 'System', TRUE);
SELECT setval('owners_id_seq', 1);

-- 4. Create restaurants table
CREATE TABLE restaurants (
    id               BIGSERIAL PRIMARY KEY,
    owner_id         BIGINT NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
    name             TEXT NOT NULL,
    slug             TEXT NOT NULL UNIQUE,
    address          TEXT,
    location         geography(Point, 4326),
    phone_number     TEXT,
    website          TEXT,
    google_place_id  TEXT UNIQUE,
    logo_url         TEXT,
    cover_image_url  TEXT,
    dine_in          BOOLEAN NOT NULL DEFAULT TRUE,
    takeout          BOOLEAN NOT NULL DEFAULT FALSE,
    delivery         BOOLEAN NOT NULL DEFAULT FALSE,
    minimum_spend    INTEGER,
    is_published     BOOLEAN NOT NULL DEFAULT FALSE,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_restaurants_owner_id ON restaurants(owner_id);
CREATE INDEX idx_restaurants_location ON restaurants USING GIST(location);
CREATE INDEX idx_restaurants_slug ON restaurants(slug);

-- 5. Migrate data: preserve restaurant_details.id as restaurants.id
--    so menu tables don't need value updates, only FK re-pointing.
INSERT INTO restaurants (
    id, owner_id, name, slug, address, location,
    phone_number, website, google_place_id,
    dine_in, takeout, delivery, minimum_spend,
    is_published, created_at, updated_at
)
SELECT
    rd.id,
    1,
    p.name,
    'r-' || rd.id,
    p.address,
    CASE WHEN p.latitude IS NOT NULL AND p.longitude IS NOT NULL
         THEN ST_SetSRID(ST_MakePoint(p.longitude, p.latitude), 4326)::geography
         ELSE NULL END,
    p.phone_number,
    p.website,
    p.google_place_id,
    rd.dine_in,
    rd.takeout,
    rd.delivery,
    rd.minimum_spend,
    TRUE,
    rd.created_at,
    rd.updated_at
FROM restaurant_details rd
JOIN places p ON p.id = rd.place_id;

SELECT setval('restaurants_id_seq', greatest(COALESCE((SELECT MAX(id) FROM restaurants), 0), 1));

-- 6. Drop old FKs, add new FKs pointing to restaurants(id)
ALTER TABLE menu_categories DROP CONSTRAINT menu_categories_restaurant_id_fkey;
ALTER TABLE menu_categories ADD CONSTRAINT menu_categories_restaurant_id_fkey
    FOREIGN KEY (restaurant_id) REFERENCES restaurants(id) ON DELETE CASCADE;

ALTER TABLE menu_items DROP CONSTRAINT menu_items_restaurant_id_fkey;
ALTER TABLE menu_items ADD CONSTRAINT menu_items_restaurant_id_fkey
    FOREIGN KEY (restaurant_id) REFERENCES restaurants(id) ON DELETE CASCADE;

ALTER TABLE combo_meals DROP CONSTRAINT combo_meals_restaurant_id_fkey;
ALTER TABLE combo_meals ADD CONSTRAINT combo_meals_restaurant_id_fkey
    FOREIGN KEY (restaurant_id) REFERENCES restaurants(id) ON DELETE CASCADE;

ALTER TABLE add_ons DROP CONSTRAINT add_ons_restaurant_id_fkey;
ALTER TABLE add_ons ADD CONSTRAINT add_ons_restaurant_id_fkey
    FOREIGN KEY (restaurant_id) REFERENCES restaurants(id) ON DELETE CASCADE;

-- 7. Create restaurant_hours (replaces place_opening_hours for owner path)
CREATE TABLE restaurant_hours (
    id            BIGSERIAL PRIMARY KEY,
    restaurant_id BIGINT NOT NULL REFERENCES restaurants(id) ON DELETE CASCADE,
    day_of_week   SMALLINT NOT NULL CHECK (day_of_week BETWEEN 0 AND 6),
    open_time     TIME,
    close_time    TIME,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_restaurant_hours_restaurant_id ON restaurant_hours(restaurant_id);

-- 8. Create orders + order_items
CREATE TABLE orders (
    id            BIGSERIAL PRIMARY KEY,
    restaurant_id BIGINT NOT NULL REFERENCES restaurants(id) ON DELETE CASCADE,
    status        TEXT NOT NULL DEFAULT 'pending'
                  CHECK (status IN ('pending', 'confirmed', 'preparing', 'completed', 'cancelled')),
    table_label   TEXT,
    total_amount  INTEGER NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_orders_restaurant_id ON orders(restaurant_id);
CREATE INDEX idx_orders_status ON orders(status);

CREATE TABLE order_items (
    id           BIGSERIAL PRIMARY KEY,
    order_id     BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    menu_item_id BIGINT REFERENCES menu_items(id) ON DELETE SET NULL,
    item_name    TEXT NOT NULL,
    quantity     INTEGER NOT NULL DEFAULT 1,
    unit_price   INTEGER NOT NULL,
    notes        TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_order_items_order_id ON order_items(order_id);

-- 9. Create menu_photo_uploads
CREATE TABLE menu_photo_uploads (
    id            BIGSERIAL PRIMARY KEY,
    restaurant_id BIGINT NOT NULL REFERENCES restaurants(id) ON DELETE CASCADE,
    file_path     TEXT NOT NULL,
    file_name     TEXT NOT NULL,
    ocr_status    TEXT NOT NULL DEFAULT 'pending'
                  CHECK (ocr_status IN ('pending', 'processing', 'completed', 'failed')),
    ocr_text      TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_menu_photo_uploads_restaurant_id ON menu_photo_uploads(restaurant_id);

-- +goose Down

DROP TABLE IF EXISTS menu_photo_uploads;
DROP TABLE IF EXISTS order_items;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS restaurant_hours;

-- Restore old FKs (only works if no new restaurants were created outside migration)
ALTER TABLE menu_categories DROP CONSTRAINT menu_categories_restaurant_id_fkey;
ALTER TABLE menu_categories ADD CONSTRAINT menu_categories_restaurant_id_fkey
    FOREIGN KEY (restaurant_id) REFERENCES restaurant_details(id) ON DELETE CASCADE;

ALTER TABLE menu_items DROP CONSTRAINT menu_items_restaurant_id_fkey;
ALTER TABLE menu_items ADD CONSTRAINT menu_items_restaurant_id_fkey
    FOREIGN KEY (restaurant_id) REFERENCES restaurant_details(id) ON DELETE CASCADE;

ALTER TABLE combo_meals DROP CONSTRAINT combo_meals_restaurant_id_fkey;
ALTER TABLE combo_meals ADD CONSTRAINT combo_meals_restaurant_id_fkey
    FOREIGN KEY (restaurant_id) REFERENCES restaurant_details(id) ON DELETE CASCADE;

ALTER TABLE add_ons DROP CONSTRAINT add_ons_restaurant_id_fkey;
ALTER TABLE add_ons ADD CONSTRAINT add_ons_restaurant_id_fkey
    FOREIGN KEY (restaurant_id) REFERENCES restaurant_details(id) ON DELETE CASCADE;

DROP TABLE IF EXISTS restaurants;
DROP TABLE IF EXISTS owners;

DROP EXTENSION IF EXISTS postgis;
