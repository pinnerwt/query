-- +goose Up

-- New tables for item-level option groups
CREATE TABLE menu_item_option_groups (
    id BIGSERIAL PRIMARY KEY,
    menu_item_id BIGINT NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    min_choices INTEGER NOT NULL DEFAULT 1,
    max_choices INTEGER NOT NULL DEFAULT 1,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_menu_item_option_groups_menu_item_id ON menu_item_option_groups(menu_item_id);

CREATE TABLE menu_item_option_choices (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES menu_item_option_groups(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    price_adjustment INTEGER NOT NULL DEFAULT 0,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_menu_item_option_choices_group_id ON menu_item_option_choices(group_id);

-- Migrate combo meals → menu items with option groups
-- Step 1: Create menu items from combo meals (NULL category_id, placed in "其他")
INSERT INTO menu_items (restaurant_id, category_id, name, description, price, is_available)
SELECT restaurant_id, NULL, name, description, price, is_available
FROM combo_meals;

-- Step 2: Create option groups from combo meal groups
INSERT INTO menu_item_option_groups (menu_item_id, name, min_choices, max_choices, sort_order)
SELECT mi.id, cmg.name, cmg.min_choices, cmg.max_choices, cmg.sort_order
FROM combo_meal_groups cmg
JOIN combo_meals cm ON cm.id = cmg.combo_meal_id
JOIN menu_items mi ON mi.restaurant_id = cm.restaurant_id
    AND mi.name = cm.name
    AND mi.category_id IS NULL
    AND mi.price = cm.price;

-- Step 3: Create option choices from combo meal group options
INSERT INTO menu_item_option_choices (group_id, name, price_adjustment, sort_order)
SELECT miog.id, COALESCE(cmgo.item_name, ''), cmgo.price_adjustment, cmgo.sort_order
FROM combo_meal_group_options cmgo
JOIN combo_meal_groups cmg ON cmg.id = cmgo.group_id
JOIN combo_meals cm ON cm.id = cmg.combo_meal_id
JOIN menu_items mi ON mi.restaurant_id = cm.restaurant_id
    AND mi.name = cm.name
    AND mi.category_id IS NULL
    AND mi.price = cm.price
JOIN menu_item_option_groups miog ON miog.menu_item_id = mi.id
    AND miog.name = cmg.name;

-- Step 4: Drop combo tables (cascade handles FKs)
DROP TABLE combo_meal_group_options;
DROP TABLE combo_meal_groups;
DROP TABLE combo_meals;

-- Also drop unused add_ons table
DROP TABLE add_ons;

-- +goose Down

-- Recreate combo tables
CREATE TABLE combo_meals (
    id BIGSERIAL PRIMARY KEY,
    restaurant_id BIGINT NOT NULL REFERENCES restaurants(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    price INTEGER NOT NULL,
    is_available BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE combo_meal_groups (
    id BIGSERIAL PRIMARY KEY,
    combo_meal_id BIGINT NOT NULL REFERENCES combo_meals(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    min_choices INTEGER NOT NULL DEFAULT 1,
    max_choices INTEGER NOT NULL DEFAULT 1,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE combo_meal_group_options (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES combo_meal_groups(id) ON DELETE CASCADE,
    menu_item_id BIGINT REFERENCES menu_items(id) ON DELETE SET NULL,
    item_name TEXT,
    price_adjustment INTEGER NOT NULL DEFAULT 0,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE add_ons (
    id BIGSERIAL PRIMARY KEY,
    restaurant_id BIGINT NOT NULL REFERENCES restaurants(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    price INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Drop new tables
DROP TABLE menu_item_option_choices;
DROP TABLE menu_item_option_groups;
