-- +goose Up

CREATE TABLE menu_categories (
    id            BIGSERIAL PRIMARY KEY,
    restaurant_id BIGINT NOT NULL REFERENCES restaurant_details(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    sort_order    INTEGER NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_menu_categories_restaurant_id ON menu_categories(restaurant_id);

CREATE TABLE menu_items (
    id            BIGSERIAL PRIMARY KEY,
    restaurant_id BIGINT NOT NULL REFERENCES restaurant_details(id) ON DELETE CASCADE,
    category_id   BIGINT REFERENCES menu_categories(id) ON DELETE SET NULL,
    name          TEXT NOT NULL,
    description   TEXT,
    price         INTEGER NOT NULL,
    is_available  BOOLEAN NOT NULL DEFAULT TRUE,
    photo_url     TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_menu_items_restaurant_id ON menu_items(restaurant_id);
CREATE INDEX idx_menu_items_category_id ON menu_items(category_id);

CREATE TABLE combo_meals (
    id            BIGSERIAL PRIMARY KEY,
    restaurant_id BIGINT NOT NULL REFERENCES restaurant_details(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    description   TEXT,
    price         INTEGER NOT NULL,
    is_available  BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_combo_meals_restaurant_id ON combo_meals(restaurant_id);

CREATE TABLE combo_meal_groups (
    id            BIGSERIAL PRIMARY KEY,
    combo_meal_id BIGINT NOT NULL REFERENCES combo_meals(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    min_choices   INTEGER NOT NULL DEFAULT 1,
    max_choices   INTEGER NOT NULL DEFAULT 1,
    sort_order    INTEGER NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_combo_meal_groups_combo_meal_id ON combo_meal_groups(combo_meal_id);

CREATE TABLE combo_meal_group_options (
    id               BIGSERIAL PRIMARY KEY,
    group_id         BIGINT NOT NULL REFERENCES combo_meal_groups(id) ON DELETE CASCADE,
    menu_item_id     BIGINT REFERENCES menu_items(id) ON DELETE SET NULL,
    item_name        TEXT,
    price_adjustment INTEGER NOT NULL DEFAULT 0,
    sort_order       INTEGER NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_combo_meal_group_options_group_id ON combo_meal_group_options(group_id);
CREATE INDEX idx_combo_meal_group_options_menu_item_id ON combo_meal_group_options(menu_item_id);

CREATE TABLE add_ons (
    id            BIGSERIAL PRIMARY KEY,
    restaurant_id BIGINT NOT NULL REFERENCES restaurant_details(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    price         INTEGER NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_add_ons_restaurant_id ON add_ons(restaurant_id);

-- +goose Down

DROP TABLE IF EXISTS add_ons;
DROP TABLE IF EXISTS combo_meal_group_options;
DROP TABLE IF EXISTS combo_meal_groups;
DROP TABLE IF EXISTS combo_meals;
DROP TABLE IF EXISTS menu_items;
DROP TABLE IF EXISTS menu_categories;
