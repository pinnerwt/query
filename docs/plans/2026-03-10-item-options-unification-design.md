# Menu Item Options & Combo Unification Design

**Date:** 2026-03-10
**Status:** Approved

## Problem

Menu items like ICHIRAN's Kae-Dama need selectable options (noodle firmness: extra firm, firm, medium, soft, extra soft). The current schema has no item-level option groups. Combo meals have option groups but live in a separate section — they should be regular items with options, grouped in categories alongside other items.

## Design

### Schema (Migration 00007)

Two new tables mirroring the combo group pattern:

```sql
CREATE TABLE menu_item_option_groups (
    id BIGSERIAL PRIMARY KEY,
    menu_item_id BIGINT NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    min_choices INTEGER NOT NULL DEFAULT 1,
    max_choices INTEGER NOT NULL DEFAULT 1,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE menu_item_option_choices (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES menu_item_option_groups(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    price_adjustment INTEGER NOT NULL DEFAULT 0,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### Combo Migration (in same 00007 migration)

1. For each `combo_meal`: INSERT a `menu_item` (same restaurant_id, NULL category_id, combo's name/price/description)
2. For each `combo_meal_group`: INSERT a `menu_item_option_group` on the new item
3. For each `combo_meal_group_option`: INSERT a `menu_item_option_choice`
4. DROP `combo_meal_group_options`, `combo_meal_groups`, `combo_meals` tables

Migrated combos get NULL category_id — owners can drag them into the right category in the editor.

### API / Server

- `buildMenuJSON()`: returns `{categories: [...]}` only — no `combos` array
- Each item gains `option_groups: [{name, min, max, choices: [{name, adjustment}]}]`
- `InsertMenu()` (ocr/insert.go): writes option groups per item, remove combo insertion
- Public menu (`/r/{slug}`): render option selection UI on items with groups (radio for min=max=1, checkboxes otherwise)

### Frontend

- **Types**: remove `ComboMeal/ComboGroup/ComboOption`, add `OptionGroup/OptionChoice` on `MenuItem`
- **MenuEditor**: remove combos section, add inline option group editor on items
- **MenuData**: `{categories: [...]}` only

### OCR

- **Types**: remove `MenuCombo` etc., add `OptionGroup` on `MenuItem`
- **Normalization prompt**: output options on items instead of separate combos
- **Insert**: updated to match

### Orders

- Selected options stored as text in existing `order_items.notes` field (e.g. "Noodle Firmness: Extra Firm")
- No schema change needed for order storage

### Pricing

- Each option choice has `price_adjustment` (default 0)
- Item base price + sum of selected option adjustments = final price
- Supports both no-cost options (firmness) and paid modifiers (extra cheese +20)
