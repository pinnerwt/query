-- +goose Up

CREATE TABLE menu_item_price_tiers (
    id           BIGSERIAL PRIMARY KEY,
    menu_item_id BIGINT NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
    label        TEXT NOT NULL,
    quantity     INTEGER NOT NULL DEFAULT 1,
    price        INTEGER NOT NULL,
    sort_order   INTEGER NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_menu_item_price_tiers_menu_item_id ON menu_item_price_tiers(menu_item_id);

-- +goose Down

DROP TABLE IF EXISTS menu_item_price_tiers;
