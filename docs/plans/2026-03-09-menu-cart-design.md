# Menu Cart & Multi-Price Items Design

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Support quantity-based pricing (price tiers), combo/set meals with group choices, and a client-side shopping cart for price calculation.

**Architecture:** Add a `menu_item_price_tiers` table for multi-price items. Extend OCR normalization to extract tiers and combos (best-effort). Build a frontend cart calculator using the existing combo schema.

**Tech Stack:** PostgreSQL, sqlc, Go, vanilla JS (embedded HTML)

---

## Schema Changes

### New table: `menu_item_price_tiers`

```sql
CREATE TABLE menu_item_price_tiers (
    id          BIGSERIAL PRIMARY KEY,
    menu_item_id BIGINT NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
    label       TEXT NOT NULL,        -- e.g. "2入", "6入", "一瓶"
    quantity    INTEGER NOT NULL DEFAULT 1,
    price       INTEGER NOT NULL,     -- TWD
    sort_order  INTEGER NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

- When tiers exist, `menu_items.price` stores the lowest tier price (for list display).
- Items without tiers continue using `menu_items.price` directly.

### Existing tables (no changes needed)

- `combo_meals` — base price + description
- `combo_meal_groups` — named groups with min/max choice constraints
- `combo_meal_group_options` — options with price_adjustment
- `add_ons` — standalone add-on items

## OCR Changes

### Extended normalization JSON schema

```json
{
  "categories": [
    {
      "name": "category name",
      "items": [
        {
          "name": "item name",
          "price": 688,
          "description": "optional",
          "price_tiers": [
            {"label": "2入", "quantity": 2, "price": 688},
            {"label": "6入", "quantity": 6, "price": 1680},
            {"label": "12入", "quantity": 12, "price": 3280}
          ]
        }
      ]
    }
  ],
  "combos": [
    {
      "name": "經典套餐",
      "price": 198,
      "description": "拌麵+湯品+椒麻醬+小菜",
      "groups": [
        {
          "name": "選擇麵食",
          "min_choices": 1,
          "max_choices": 1,
          "options": [
            {"name": "椒麻炸蛋麵", "price_adjustment": 0},
            {"name": "皮蛋肉醬拌麵", "price_adjustment": 0}
          ]
        }
      ]
    }
  ]
}
```

- `price_tiers` is optional — omit for single-price items.
- When `price_tiers` exists, `price` is set to the lowest tier price.
- `combos` is best-effort — OCR extracts what it can, user corrects via SQL.

### Prompt additions

- Instruct normalizer to detect quantity-based pricing patterns (e.g. "Two/NT$688 Six/NT$1,680").
- Instruct normalizer to detect set/combo meal patterns and output as `combos` array.
- Both are optional fields — no breakage if normalizer can't detect them.

### insertMenu changes

- After inserting menu items, insert price tiers into `menu_item_price_tiers`.
- After inserting categories/items, insert combos into `combo_meals` / `combo_meal_groups` / `combo_meal_group_options`.
- Clear existing combo data before re-inserting (idempotent, same as current menu items).

## Server API Changes

### `GET /api/menu?restaurant_id=N`

Response changes from array to object:

```json
{
  "categories": [
    {
      "name": "自由點餐",
      "items": [
        {
          "name": "法國頂級吉拉朵生蠔",
          "price": 688,
          "description": "",
          "price_tiers": [
            {"label": "2入", "quantity": 2, "price": 688},
            {"label": "6入", "quantity": 6, "price": 1680},
            {"label": "12入", "quantity": 12, "price": 3280}
          ]
        },
        {
          "name": "布拉塔起司",
          "price": 428,
          "description": ""
        }
      ]
    }
  ],
  "combos": [
    {
      "id": 1,
      "name": "經典套餐",
      "price": 198,
      "description": "拌麵+湯品+椒麻醬+小菜",
      "groups": [
        {
          "name": "選擇麵食",
          "min": 1,
          "max": 1,
          "options": [
            {"name": "椒麻炸蛋麵", "adjustment": 0},
            {"name": "皮蛋肉醬拌麵", "adjustment": 0}
          ]
        }
      ]
    }
  ]
}
```

### New SQL queries needed

- `ListPriceTiersByRestaurant` — all tiers for a restaurant's menu items
- `ListComboMealsByRestaurant` — combos with nested groups and options
- `DeleteComboMealsByRestaurant` — for idempotent re-insertion
- `CreatePriceTier` — insert a price tier row
- `DeletePriceTiersByRestaurant` — for idempotent re-insertion

## Frontend Cart

### Menu display changes

- Items with price tiers: show lowest price with "起" suffix (e.g. "NT$688起"). Tapping "+" opens tier picker.
- Items without tiers: direct "+" button adds to cart.
- Combos section: each combo shows base price. Tapping opens inline group selection flow (step through each group, pick within min/max constraints).

### Cart panel

- Sticky bottom bar showing item count and total.
- Tap to expand: full cart with line items, quantities (+/- buttons), tier labels, combo choices, per-line subtotals.
- "Clear cart" button.
- Cart persisted to `localStorage` per restaurant.
- Pure client-side — no server storage.

### Price calculation

- Regular items: price × quantity
- Tiered items: selected tier price × quantity
- Combos: base price + sum of selected option adjustments
- Total: sum of all line items

## Price conventions

- `-1` = unknown (rendered as "未知", cannot add to cart)
- `-2` = market price (rendered as "時價", cannot add to cart)
- `0` = free (can add to cart, contributes $0)
- `>0` = normal price in TWD
