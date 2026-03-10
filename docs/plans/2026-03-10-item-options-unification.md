# Item Options & Combo Unification Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Unify combo meals into regular menu items with option groups, so items like Kae-Dama can have selectable options (e.g. noodle firmness) while staying in their category.

**Architecture:** Two new tables (`menu_item_option_groups`, `menu_item_option_choices`) replace the three combo tables. Migration moves existing combo data into menu items with option groups. All layers updated: SQL queries, OCR types/insert/prompt, server API, public menu HTML, frontend types/editor.

**Tech Stack:** Go 1.25, PostgreSQL/PostGIS, sqlc, Preact + TypeScript, testcontainers-go

---

### Task 1: Migration — new tables, combo data migration, drop combo tables

**Files:**
- Create: `migrations/00007_item_option_groups.sql`

**Step 1: Write the migration**

```sql
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
```

**Step 2: Commit**

```bash
git add migrations/00007_item_option_groups.sql
git commit -m "feat: add migration 00007 — item option groups, combo unification"
```

---

### Task 2: SQL queries — replace combo queries with option group queries

**Files:**
- Modify: `internal/db/queries/menu.sql`

**Step 1: Update the queries file**

Remove these combo-related queries:
- `CreateComboMeal` (line 28-31)
- `CreateComboMealGroup` (line 33-36)
- `CreateComboMealGroupOption` (line 38-41)
- `CreateAddOn` (line 43-46)
- `ListAddOnsByRestaurant` (line 48-49)
- `DeleteComboMealsByRestaurant` (line 68-69)
- `ListComboMealsByRestaurant` (line 71-72)
- `ListComboMealGroupsByComboMeal` (line 74-75)
- `ListComboMealGroupOptionsByGroup` (line 77-78)

Add these new queries:

```sql
-- name: CreateOptionGroup :one
INSERT INTO menu_item_option_groups (menu_item_id, name, min_choices, max_choices, sort_order)
VALUES (@menu_item_id, @name, @min_choices, @max_choices, @sort_order)
RETURNING *;

-- name: ListOptionGroupsByMenuItem :many
SELECT * FROM menu_item_option_groups WHERE menu_item_id = $1 ORDER BY sort_order;

-- name: ListOptionGroupsByRestaurant :many
SELECT og.* FROM menu_item_option_groups og
JOIN menu_items mi ON mi.id = og.menu_item_id
WHERE mi.restaurant_id = $1
ORDER BY og.menu_item_id, og.sort_order;

-- name: DeleteOptionGroupsByMenuItem :exec
DELETE FROM menu_item_option_groups WHERE menu_item_id = $1;

-- name: CreateOptionChoice :one
INSERT INTO menu_item_option_choices (group_id, name, price_adjustment, sort_order)
VALUES (@group_id, @name, @price_adjustment, @sort_order)
RETURNING *;

-- name: ListOptionChoicesByGroup :many
SELECT * FROM menu_item_option_choices WHERE group_id = $1 ORDER BY sort_order;

-- name: ListOptionChoicesByRestaurant :many
SELECT oc.* FROM menu_item_option_choices oc
JOIN menu_item_option_groups og ON og.id = oc.group_id
JOIN menu_items mi ON mi.id = og.menu_item_id
WHERE mi.restaurant_id = $1
ORDER BY oc.group_id, oc.sort_order;
```

**Step 2: Run sqlc generate**

```bash
sqlc generate
```

Expected: generates updated Go code in `internal/db/generated/` with new query methods and without combo methods.

**Step 3: Commit**

```bash
git add internal/db/queries/menu.sql internal/db/generated/
git commit -m "feat: replace combo queries with option group queries, sqlc generate"
```

---

### Task 3: Schema test — verify new tables and combo migration

**Files:**
- Modify: `tests/menu_integration_test.go`

**Step 1: Write failing test for option group CRUD**

Add to `tests/menu_integration_test.go`:

```go
func TestOptionGroupCRUD(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()
	q := db.New(conn)
	rest := createTestRestaurant(t, ctx, q)

	cat, err := q.CreateMenuCategory(ctx, db.CreateMenuCategoryParams{
		RestaurantID: rest.ID,
		Name:         "Side Menu",
		SortOrder:    0,
	})
	require.NoError(t, err)

	item, err := q.CreateMenuItem(ctx, db.CreateMenuItemParams{
		RestaurantID: rest.ID,
		CategoryID:   pgtype.Int8{Int64: cat.ID, Valid: true},
		Name:         "Kae-Dama",
		Price:        60,
	})
	require.NoError(t, err)

	t.Run("create and list option groups", func(t *testing.T) {
		og, err := q.CreateOptionGroup(ctx, db.CreateOptionGroupParams{
			MenuItemID: item.ID,
			Name:       "Noodle Firmness",
			MinChoices: 1,
			MaxChoices: 1,
			SortOrder:  0,
		})
		require.NoError(t, err)
		assert.Equal(t, "Noodle Firmness", og.Name)
		assert.Equal(t, int32(1), og.MinChoices)

		groups, err := q.ListOptionGroupsByMenuItem(ctx, item.ID)
		require.NoError(t, err)
		assert.Len(t, groups, 1)

		// Also test restaurant-level listing
		allGroups, err := q.ListOptionGroupsByRestaurant(ctx, rest.ID)
		require.NoError(t, err)
		assert.Len(t, allGroups, 1)
	})

	t.Run("create and list option choices", func(t *testing.T) {
		og, err := q.CreateOptionGroup(ctx, db.CreateOptionGroupParams{
			MenuItemID: item.ID,
			Name:       "Firmness",
			MinChoices: 1,
			MaxChoices: 1,
			SortOrder:  1,
		})
		require.NoError(t, err)

		choices := []string{"Extra Firm", "Firm", "Medium", "Soft", "Extra Soft"}
		for i, name := range choices {
			_, err := q.CreateOptionChoice(ctx, db.CreateOptionChoiceParams{
				GroupID:         og.ID,
				Name:            name,
				PriceAdjustment: 0,
				SortOrder:       int32(i),
			})
			require.NoError(t, err)
		}

		opts, err := q.ListOptionChoicesByGroup(ctx, og.ID)
		require.NoError(t, err)
		assert.Len(t, opts, 5)
		assert.Equal(t, "Extra Firm", opts[0].Name)
		assert.Equal(t, "Extra Soft", opts[4].Name)

		// Restaurant-level listing
		allOpts, err := q.ListOptionChoicesByRestaurant(ctx, rest.ID)
		require.NoError(t, err)
		assert.Len(t, allOpts, 5)
	})

	t.Run("cascade delete on item removes option groups", func(t *testing.T) {
		item2, err := q.CreateMenuItem(ctx, db.CreateMenuItemParams{
			RestaurantID: rest.ID,
			CategoryID:   pgtype.Int8{Int64: cat.ID, Valid: true},
			Name:         "Temp Item",
			Price:        100,
		})
		require.NoError(t, err)

		og, err := q.CreateOptionGroup(ctx, db.CreateOptionGroupParams{
			MenuItemID: item2.ID,
			Name:       "Size",
			MinChoices: 1,
			MaxChoices: 1,
			SortOrder:  0,
		})
		require.NoError(t, err)

		_, err = q.CreateOptionChoice(ctx, db.CreateOptionChoiceParams{
			GroupID:         og.ID,
			Name:            "Large",
			PriceAdjustment: 20,
			SortOrder:       0,
		})
		require.NoError(t, err)

		err = q.DeleteMenuItem(ctx, item2.ID)
		require.NoError(t, err)

		groups, err := q.ListOptionGroupsByMenuItem(ctx, item2.ID)
		require.NoError(t, err)
		assert.Len(t, groups, 0)
	})
}
```

**Step 2: Run test to verify it passes**

```bash
go test ./tests -run TestOptionGroupCRUD -v
```

Expected: PASS — the migration creates the tables, sqlc generated the query methods.

**Step 3: Commit**

```bash
git add tests/menu_integration_test.go
git commit -m "test: add option group CRUD integration tests"
```

---

### Task 4: OCR types — replace combo types with option groups on MenuItem

**Files:**
- Modify: `internal/ocr/types.go`

**Step 1: Replace types**

Replace the entire file content with:

```go
package ocr

// MenuData represents a structured restaurant menu.
type MenuData struct {
	Categories []MenuCategory `json:"categories"`
}

// MenuCategory is a group of related menu items.
type MenuCategory struct {
	Name  string     `json:"name"`
	Items []MenuItem `json:"items"`
}

// MenuItem is a single dish or drink.
type MenuItem struct {
	Name         string        `json:"name"`
	Price        int           `json:"price"`
	Description  string        `json:"description,omitempty"`
	PriceTiers   []PriceTier   `json:"price_tiers,omitempty"`
	OptionGroups []OptionGroup `json:"option_groups,omitempty"`
}

// PriceTier represents a quantity-based price option.
type PriceTier struct {
	Label    string `json:"label"`
	Quantity int    `json:"quantity"`
	Price    int    `json:"price"`
}

// OptionGroup is a set of choices on a menu item (e.g. noodle firmness, spice level).
type OptionGroup struct {
	Name       string         `json:"name"`
	MinChoices int            `json:"min_choices"`
	MaxChoices int            `json:"max_choices"`
	Options    []OptionChoice `json:"options"`
}

// OptionChoice is a single selectable option within a group.
type OptionChoice struct {
	Name            string `json:"name"`
	PriceAdjustment int    `json:"price_adjustment"`
}
```

**Step 2: Commit**

```bash
git add internal/ocr/types.go
git commit -m "refactor: replace combo types with option groups on MenuItem"
```

---

### Task 5: OCR insert — replace combo insertion with option group insertion

**Files:**
- Modify: `internal/ocr/insert.go`

**Step 1: Update InsertMenu**

Replace the entire file content with:

```go
package ocr

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
	db "github.com/pinnertw/query/internal/db/generated"
)

// InsertMenu writes a structured menu to the database for the given restaurant.
// It clears existing menu data and replaces it entirely.
func InsertMenu(ctx context.Context, q *db.Queries, restaurantID int64, menu *MenuData) error {
	// Clear existing menu data (idempotent)
	_ = q.DeleteMenuItemsByRestaurant(ctx, restaurantID)
	_ = q.DeleteMenuCategoriesByRestaurant(ctx, restaurantID)

	// Insert categories and items
	for i, cat := range menu.Categories {
		category, err := q.CreateMenuCategory(ctx, db.CreateMenuCategoryParams{
			RestaurantID: restaurantID,
			Name:         cat.Name,
			SortOrder:    int32(i),
		})
		if err != nil {
			return fmt.Errorf("create category %q: %w", cat.Name, err)
		}

		for _, item := range cat.Items {
			mi, err := q.CreateMenuItem(ctx, db.CreateMenuItemParams{
				RestaurantID: restaurantID,
				CategoryID:   pgtype.Int8{Int64: category.ID, Valid: true},
				Name:         item.Name,
				Description:  pgtype.Text{String: item.Description, Valid: item.Description != ""},
				Price:        int32(item.Price),
				PhotoUrl:     pgtype.Text{},
			})
			if err != nil {
				return fmt.Errorf("create item %q: %w", item.Name, err)
			}

			for j, tier := range item.PriceTiers {
				_, err := q.CreatePriceTier(ctx, db.CreatePriceTierParams{
					MenuItemID: mi.ID,
					Label:      tier.Label,
					Quantity:   int32(tier.Quantity),
					Price:      int32(tier.Price),
					SortOrder:  int32(j),
				})
				if err != nil {
					return fmt.Errorf("create price tier %q for item %q: %w", tier.Label, item.Name, err)
				}
			}

			for gi, og := range item.OptionGroups {
				group, err := q.CreateOptionGroup(ctx, db.CreateOptionGroupParams{
					MenuItemID: mi.ID,
					Name:       og.Name,
					MinChoices: int32(og.MinChoices),
					MaxChoices: int32(og.MaxChoices),
					SortOrder:  int32(gi),
				})
				if err != nil {
					return fmt.Errorf("create option group %q for item %q: %w", og.Name, item.Name, err)
				}

				for oi, opt := range og.Options {
					_, err := q.CreateOptionChoice(ctx, db.CreateOptionChoiceParams{
						GroupID:         group.ID,
						Name:            opt.Name,
						PriceAdjustment: int32(opt.PriceAdjustment),
						SortOrder:       int32(oi),
					})
					if err != nil {
						return fmt.Errorf("create option %q in group %q: %w", opt.Name, og.Name, err)
					}
				}
			}
		}
	}

	return nil
}
```

**Step 2: Commit**

```bash
git add internal/ocr/insert.go
git commit -m "refactor: replace combo insertion with option group insertion"
```

---

### Task 6: OCR normalization prompt and MergeMenus — update for option groups

**Files:**
- Modify: `internal/ocr/pipeline.go`

**Step 1: Update NormalizePrompt**

Replace the `NormalizePrompt` constant (lines 24-81) — change the JSON schema to put option groups on items instead of separate combos. The new prompt:

```go
const NormalizePrompt = `You are given raw OCR text from a restaurant menu photo. Parse it into structured JSON.

Output ONLY valid JSON with this exact schema, no other text:
{
  "categories": [
    {
      "name": "category name",
      "items": [
        {
          "name": "item name",
          "price": 100,
          "description": "optional description",
          "price_tiers": [
            {"label": "2入", "quantity": 2, "price": 688},
            {"label": "6入", "quantity": 6, "price": 1680}
          ],
          "option_groups": [
            {
              "name": "group name",
              "min_choices": 1,
              "max_choices": 1,
              "options": [
                {"name": "option name", "price_adjustment": 0}
              ]
            }
          ]
        }
      ]
    }
  ]
}

Rules:
- Use Chinese (Traditional) for all item names and category names. If the menu has both Chinese and English/Japanese names for the same item, use the Chinese name only.
- Ignore items that only appear in English, Japanese, or Korean without a Chinese equivalent.
- price is in TWD (New Taiwan Dollars), as an integer (no decimals)
- ONLY include items where you are confident the price matches the item. If a price seems misaligned or uncertain, skip that item rather than guess wrong.
- If an item has no price at all, set price to -1 (unknown/not shown)
- If the menu explicitly says "時價" (market price) for an item, set price to -2
- If there are no clear categories, use "其他" as the category name
- Merge duplicate items (same name) keeping the first occurrence
- description is optional, omit or set to "" if none
- Do NOT include any text outside the JSON object
- If an item has multiple prices for different quantities (e.g. "Two/NT$688, Six/NT$1,680"), use price_tiers array. Set item price to the lowest tier price.
- If an item has only one price, omit price_tiers (do NOT create a single-entry price_tiers array).
- If an item has selectable options (e.g. firmness, spice level, size, toppings, soup base), add option_groups on that item with min_choices/max_choices and options.
- option_groups is optional — omit if the item has no selectable options.
- price_adjustment in options is the extra cost on top of the item base price (0 if no upcharge).
- Set meals / combos with chooseable components should be regular items with option_groups, NOT a separate structure.

Raw OCR text:
`
```

**Step 2: Update MergeMenus to remove combo merging**

Replace MergeMenus (lines 139-184) — remove the combo deduplication section:

```go
func MergeMenus(menus []*MenuData) *MenuData {
	type catEntry struct {
		cat      MenuCategory
		itemSeen map[string]bool
	}

	catMap := make(map[string]int)
	var cats []catEntry

	for _, m := range menus {
		for _, cat := range m.Categories {
			idx, exists := catMap[cat.Name]
			if !exists {
				idx = len(cats)
				catMap[cat.Name] = idx
				cats = append(cats, catEntry{
					cat:      MenuCategory{Name: cat.Name},
					itemSeen: make(map[string]bool),
				})
			}
			for _, item := range cat.Items {
				if !cats[idx].itemSeen[item.Name] {
					cats[idx].itemSeen[item.Name] = true
					cats[idx].cat.Items = append(cats[idx].cat.Items, item)
				}
			}
		}
	}

	result := &MenuData{}
	for _, e := range cats {
		result.Categories = append(result.Categories, e.cat)
	}

	return result
}
```

**Step 3: Commit**

```bash
git add internal/ocr/pipeline.go
git commit -m "refactor: update OCR prompt and MergeMenus for option groups"
```

---

### Task 7: Fix compilation — update all Go code referencing combo types

**Files:**
- Modify: `cmd/server/main.go` (buildMenuJSON, lines 1177-1216)
- Modify: any other files referencing combo types

**Step 1: Update buildMenuJSON in `cmd/server/main.go`**

Replace lines 1117-1217 (the entire `buildMenuJSON` function). The new version removes combo fetching and adds option group fetching:

```go
func (s *server) buildMenuJSON(ctx context.Context, restaurantID int64) (map[string]interface{}, error) {
	categories, err := s.q.ListMenuCategoriesByRestaurant(ctx, restaurantID)
	if err != nil {
		return nil, err
	}

	items, err := s.q.ListMenuItemsByRestaurant(ctx, restaurantID)
	if err != nil {
		return nil, err
	}

	priceTiers, err := s.q.ListPriceTiersByRestaurant(ctx, restaurantID)
	if err != nil {
		return nil, err
	}

	optionGroups, err := s.q.ListOptionGroupsByRestaurant(ctx, restaurantID)
	if err != nil {
		return nil, err
	}

	optionChoices, err := s.q.ListOptionChoicesByRestaurant(ctx, restaurantID)
	if err != nil {
		return nil, err
	}

	// Build price tier map: item_id -> tiers
	tierMap := make(map[int64][]map[string]interface{})
	for _, pt := range priceTiers {
		tierMap[pt.MenuItemID] = append(tierMap[pt.MenuItemID], map[string]interface{}{
			"label":    pt.Label,
			"quantity": pt.Quantity,
			"price":    pt.Price,
		})
	}

	// Build option choice map: group_id -> choices
	choiceMap := make(map[int64][]map[string]interface{})
	for _, oc := range optionChoices {
		choiceMap[oc.GroupID] = append(choiceMap[oc.GroupID], map[string]interface{}{
			"name":       oc.Name,
			"adjustment": oc.PriceAdjustment,
		})
	}

	// Build option group map: item_id -> groups
	ogMap := make(map[int64][]map[string]interface{})
	for _, og := range optionGroups {
		ogMap[og.MenuItemID] = append(ogMap[og.MenuItemID], map[string]interface{}{
			"name":    og.Name,
			"min":     og.MinChoices,
			"max":     og.MaxChoices,
			"options": choiceMap[og.ID],
		})
	}

	type categoryOut struct {
		Name  string                   `json:"name"`
		Items []map[string]interface{} `json:"items"`
	}
	catIdx := make(map[int64]int)
	cats := make([]categoryOut, 0)
	for _, c := range categories {
		catIdx[c.ID] = len(cats)
		cats = append(cats, categoryOut{Name: c.Name})
	}

	for _, it := range items {
		mi := map[string]interface{}{
			"id":    it.ID,
			"name":  it.Name,
			"price": it.Price,
		}
		if it.Description.Valid && it.Description.String != "" {
			mi["description"] = it.Description.String
		}
		if tiers, ok := tierMap[it.ID]; ok {
			mi["price_tiers"] = tiers
		}
		if groups, ok := ogMap[it.ID]; ok {
			mi["option_groups"] = groups
		}
		if it.CategoryID.Valid {
			if idx, ok := catIdx[it.CategoryID.Int64]; ok {
				cats[idx].Items = append(cats[idx].Items, mi)
				continue
			}
		}
		if len(cats) == 0 || cats[len(cats)-1].Name != "其他" {
			cats = append(cats, categoryOut{Name: "其他"})
		}
		cats[len(cats)-1].Items = append(cats[len(cats)-1].Items, mi)
	}

	return map[string]interface{}{
		"categories": cats,
	}, nil
}
```

**Step 2: Build to verify compilation**

```bash
go build ./cmd/server && go build ./cmd/ocr && go build ./cmd/seed
```

Expected: successful compilation. If there are other files referencing combo types (e.g. `cmd/ocr/main.go`), fix those too — they should just work since `MenuData` no longer has `Combos` field and we removed combo queries.

**Step 3: Run existing tests**

```bash
go test ./tests -v
```

Expected: all tests pass. The `TestMenuBulkReplace` test uses `DeleteComboMealsByRestaurant` — that query no longer exists, but `InsertMenu` no longer calls it either. Check if `TestMenuBulkReplace` needs updating (it doesn't call combo queries directly).

**Step 4: Commit**

```bash
git add cmd/server/main.go
git commit -m "refactor: buildMenuJSON returns option groups on items, no more combos"
```

---

### Task 8: Test InsertMenu with option groups

**Files:**
- Modify: `tests/menu_integration_test.go`

**Step 1: Write test for InsertMenu with option groups**

```go
func TestInsertMenuWithOptionGroups(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()
	q := db.New(conn)
	rest := createTestRestaurant(t, ctx, q)

	menu := &ocr.MenuData{
		Categories: []ocr.MenuCategory{
			{
				Name: "Side Menu",
				Items: []ocr.MenuItem{
					{
						Name:  "Kae-Dama",
						Price: 60,
						OptionGroups: []ocr.OptionGroup{
							{
								Name:       "Noodle Firmness",
								MinChoices: 1,
								MaxChoices: 1,
								Options: []ocr.OptionChoice{
									{Name: "Extra Firm", PriceAdjustment: 0},
									{Name: "Firm", PriceAdjustment: 0},
									{Name: "Medium", PriceAdjustment: 0},
									{Name: "Soft", PriceAdjustment: 0},
									{Name: "Extra Soft", PriceAdjustment: 0},
								},
							},
						},
					},
					{
						Name:  "Extra Sliced Pork",
						Price: 65,
						Description: "3 pieces",
					},
				},
			},
		},
	}

	err := ocr.InsertMenu(ctx, q, rest.ID, menu)
	require.NoError(t, err)

	// Verify categories
	cats, err := q.ListMenuCategoriesByRestaurant(ctx, rest.ID)
	require.NoError(t, err)
	assert.Len(t, cats, 1)
	assert.Equal(t, "Side Menu", cats[0].Name)

	// Verify items
	items, err := q.ListMenuItemsByRestaurant(ctx, rest.ID)
	require.NoError(t, err)
	assert.Len(t, items, 2)

	// Find the Kae-Dama item
	var kaeDamaID int64
	for _, it := range items {
		if it.Name == "Kae-Dama" {
			kaeDamaID = it.ID
			break
		}
	}
	require.NotZero(t, kaeDamaID)

	// Verify option groups
	groups, err := q.ListOptionGroupsByMenuItem(ctx, kaeDamaID)
	require.NoError(t, err)
	assert.Len(t, groups, 1)
	assert.Equal(t, "Noodle Firmness", groups[0].Name)
	assert.Equal(t, int32(1), groups[0].MinChoices)
	assert.Equal(t, int32(1), groups[0].MaxChoices)

	// Verify option choices
	choices, err := q.ListOptionChoicesByGroup(ctx, groups[0].ID)
	require.NoError(t, err)
	assert.Len(t, choices, 5)
	assert.Equal(t, "Extra Firm", choices[0].Name)
	assert.Equal(t, int32(0), choices[0].PriceAdjustment)

	// Verify idempotent replace clears old data
	menu2 := &ocr.MenuData{
		Categories: []ocr.MenuCategory{
			{Name: "New", Items: []ocr.MenuItem{{Name: "Simple", Price: 100}}},
		},
	}
	err = ocr.InsertMenu(ctx, q, rest.ID, menu2)
	require.NoError(t, err)

	items2, err := q.ListMenuItemsByRestaurant(ctx, rest.ID)
	require.NoError(t, err)
	assert.Len(t, items2, 1)
	assert.Equal(t, "Simple", items2[0].Name)

	groups2, err := q.ListOptionGroupsByRestaurant(ctx, rest.ID)
	require.NoError(t, err)
	assert.Len(t, groups2, 0)
}
```

Add this import to the test file:
```go
"github.com/pinnertw/query/internal/ocr"
```

**Step 2: Run tests**

```bash
go test ./tests -run TestInsertMenuWithOptionGroups -v
```

Expected: PASS

**Step 3: Commit**

```bash
git add tests/menu_integration_test.go
git commit -m "test: add InsertMenu with option groups integration test"
```

---

### Task 9: Frontend types — replace combo types with option groups

**Files:**
- Modify: `frontend/src/lib/api.ts`

**Step 1: Update TypeScript types**

In `frontend/src/lib/api.ts`:

Replace `ComboMeal`, `ComboGroup`, `ComboOption` interfaces (lines 68-88) with:

```typescript
export interface OptionGroup {
  name: string;
  min_choices: number;
  max_choices: number;
  options: OptionChoice[];
}

export interface OptionChoice {
  name: string;
  price_adjustment: number;
}
```

Update `MenuItem` (lines 52-60) to add option_groups:

```typescript
export interface MenuItem {
  id: number;
  name: string;
  description: string;
  price: number;
  is_available: boolean;
  category_id: number;
  price_tiers?: PriceTier[];
  option_groups?: OptionGroup[];
}
```

Update `MenuData` (lines 90-93) to remove combos:

```typescript
export interface MenuData {
  categories: MenuCategory[];
}
```

**Step 2: TypeScript check**

```bash
cd frontend && npx tsc --noEmit
```

Expected: errors in MenuEditor.tsx if it references combos (it probably doesn't reference them in the editor based on the code seen, but check). Fix any errors.

**Step 3: Commit**

```bash
git add frontend/src/lib/api.ts
git commit -m "refactor: replace combo types with option groups on MenuItem"
```

---

### Task 10: MenuEditor — add option group editing UI

**Files:**
- Modify: `frontend/src/pages/MenuEditor.tsx`

**Step 1: Update MenuEditor**

Key changes needed in `MenuEditor.tsx`:

1. Update the `handleSave` call — `menu` no longer has `combos`
2. Update the `useEffect` that loads menu — don't expect combos
3. Add option group editing in the item edit form (after the price input)

In the item edit form section (around lines 326-334), add option group editing after the price field:

```tsx
{/* Option Groups */}
{item.option_groups?.map((og, ogIdx) => (
  <div key={ogIdx} class="mt-3 p-3 bg-slate-50 rounded-lg border border-slate-200">
    <div class="flex items-center gap-2 mb-2">
      <input
        class={inputClass}
        value={og.name}
        placeholder="Group name (e.g. Noodle Firmness)"
        onInput={(e) => {
          const val = (e.target as HTMLInputElement).value;
          setMenu((prev) => {
            const cats = [...prev.categories];
            const items = [...cats[catIdx].items];
            const groups = [...(items[itemIdx].option_groups || [])];
            groups[ogIdx] = { ...groups[ogIdx], name: val };
            items[itemIdx] = { ...items[itemIdx], option_groups: groups };
            cats[catIdx] = { ...cats[catIdx], items };
            return { ...prev, categories: cats };
          });
        }}
      />
      <span class="text-xs text-slate-400 whitespace-nowrap">
        {og.min_choices === og.max_choices ? `選 ${og.min_choices}` : `${og.min_choices}-${og.max_choices}`}
      </span>
      <button
        class="text-red-400 hover:text-red-600 text-sm"
        onClick={() => {
          setMenu((prev) => {
            const cats = [...prev.categories];
            const items = [...cats[catIdx].items];
            const groups = (items[itemIdx].option_groups || []).filter((_, i) => i !== ogIdx);
            items[itemIdx] = { ...items[itemIdx], option_groups: groups.length ? groups : undefined };
            cats[catIdx] = { ...cats[catIdx], items };
            return { ...prev, categories: cats };
          });
        }}
      >✕</button>
    </div>
    <div class="flex flex-wrap gap-1">
      {og.options.map((opt, optIdx) => (
        <span key={optIdx} class="inline-flex items-center gap-1 bg-white border border-slate-200 rounded px-2 py-0.5 text-sm">
          <input
            class="border-none bg-transparent text-sm w-20 p-0 focus:outline-none"
            value={opt.name}
            onInput={(e) => {
              const val = (e.target as HTMLInputElement).value;
              setMenu((prev) => {
                const cats = [...prev.categories];
                const items = [...cats[catIdx].items];
                const groups = [...(items[itemIdx].option_groups || [])];
                const options = [...groups[ogIdx].options];
                options[optIdx] = { ...options[optIdx], name: val };
                groups[ogIdx] = { ...groups[ogIdx], options };
                items[itemIdx] = { ...items[itemIdx], option_groups: groups };
                cats[catIdx] = { ...cats[catIdx], items };
                return { ...prev, categories: cats };
              });
            }}
          />
          {opt.price_adjustment !== 0 && (
            <span class="text-xs text-amber-600">+{opt.price_adjustment}</span>
          )}
          <button
            class="text-red-300 hover:text-red-500 text-xs"
            onClick={() => {
              setMenu((prev) => {
                const cats = [...prev.categories];
                const items = [...cats[catIdx].items];
                const groups = [...(items[itemIdx].option_groups || [])];
                const options = groups[ogIdx].options.filter((_, i) => i !== optIdx);
                groups[ogIdx] = { ...groups[ogIdx], options };
                items[itemIdx] = { ...items[itemIdx], option_groups: groups };
                cats[catIdx] = { ...cats[catIdx], items };
                return { ...prev, categories: cats };
              });
            }}
          >✕</button>
        </span>
      ))}
      <button
        class="text-xs text-amber-600 hover:text-amber-700 px-2 py-0.5 border border-dashed border-amber-300 rounded"
        onClick={() => {
          setMenu((prev) => {
            const cats = [...prev.categories];
            const items = [...cats[catIdx].items];
            const groups = [...(items[itemIdx].option_groups || [])];
            const options = [...groups[ogIdx].options, { name: "", price_adjustment: 0 }];
            groups[ogIdx] = { ...groups[ogIdx], options };
            items[itemIdx] = { ...items[itemIdx], option_groups: groups };
            cats[catIdx] = { ...cats[catIdx], items };
            return { ...prev, categories: cats };
          });
        }}
      >+ 選項</button>
    </div>
  </div>
))}
<button
  class="text-xs text-slate-400 hover:text-slate-600 mt-2"
  onClick={() => {
    setMenu((prev) => {
      const cats = [...prev.categories];
      const items = [...cats[catIdx].items];
      const groups = [...(items[itemIdx].option_groups || []), {
        name: "",
        min_choices: 1,
        max_choices: 1,
        options: [{ name: "", price_adjustment: 0 }],
      }];
      items[itemIdx] = { ...items[itemIdx], option_groups: groups };
      cats[catIdx] = { ...cats[catIdx], items };
      return { ...prev, categories: cats };
    });
  }}
>+ 選項群組</button>
```

Also update the initial menu load (around line 33) to not expect combos:
```typescript
const safe = { categories: data?.categories || [] };
```

And the OCR handler (around line 137):
```typescript
const safe = { categories: result?.categories || [] };
```

**Step 2: TypeScript check**

```bash
cd frontend && npx tsc --noEmit
```

Expected: PASS

**Step 3: Commit**

```bash
git add frontend/src/pages/MenuEditor.tsx
git commit -m "feat: add option group editing UI in MenuEditor, remove combos"
```

---

### Task 11: Public menu page — render option groups with selection UI

**Files:**
- Modify: `cmd/server/main.go` (publicMenuHTML template, lines 1291-1368)

**Step 1: Update the public menu HTML/JS**

The key changes to the JavaScript in the template:
1. When an item has `option_groups`, show a selection modal instead of directly adding to cart
2. `addToCart` needs to handle selected options (stored in notes, price adjustments added)
3. Cart items need to track selected options for display

Update the render function and add option selection logic. Replace the `<script>` section (lines 1325-1366):

```javascript
var restaurant = %s;
var menuData = %s;
var slug = "%s";
var cart = [];
function esc(s){var d=document.createElement('div');d.textContent=s;return d.innerHTML;}
function formatPrice(p){if(p===-1)return'未知';if(p===-2)return'時價';return'NT$'+p;}
function render(){
var cats=menuData.categories||[];
var html=cats.filter(function(c){return c.items&&c.items.length>0;}).map(function(cat){
return '<div class="category"><div class="category-header">'+esc(cat.name)+'</div>'+
cat.items.map(function(it){
var hasOpts=it.option_groups&&it.option_groups.length>0;
return '<div class="menu-item"><div class="item-info"><div class="item-name">'+esc(it.name)+'</div>'+
(it.description?'<div class="item-desc">'+esc(it.description)+'</div>':'')+
(hasOpts?'<div class="item-desc" style="color:#e67e22">有選項</div>':'')+
'</div><span class="item-price">'+formatPrice(it.price)+'</span>'+
(it.price>=0?'<button class="add-btn" onclick="handleAdd('+it.id+')">+</button>':'')+
'</div>';}).join('')+'</div>';}).join('');
document.getElementById('menu').innerHTML=html;
}
var allItems={};
(menuData.categories||[]).forEach(function(c){c.items.forEach(function(it){allItems[it.id]=it;});});
function handleAdd(id){
var it=allItems[id];
if(!it)return;
if(!it.option_groups||it.option_groups.length===0){addToCart(id,it.name,it.price,'');return;}
showOptionModal(it);
}
function showOptionModal(it){
var bg=document.createElement('div');
bg.style.cssText='position:fixed;top:0;left:0;width:100%%;height:100%%;background:rgba(0,0,0,0.5);z-index:100;display:flex;align-items:flex-end;justify-content:center;';
var box=document.createElement('div');
box.style.cssText='background:#fff;width:100%%;max-width:480px;border-radius:16px 16px 0 0;padding:20px;max-height:70vh;overflow-y:auto;';
var h='<div style="font-weight:700;font-size:18px;margin-bottom:12px;">'+esc(it.name)+' — '+formatPrice(it.price)+'</div>';
it.option_groups.forEach(function(og,gi){
var req=og.min_choices>0?' <span style="color:#e74c3c;font-size:12px;">必選</span>':'';
h+='<div style="font-weight:600;margin:10px 0 6px;">'+esc(og.name)+req+'</div>';
var isRadio=og.min_choices===1&&og.max_choices===1;
og.options.forEach(function(opt,oi){
var type=isRadio?'radio':'checkbox';
var nm='og'+gi;
var adj=opt.price_adjustment?(' (+'+opt.price_adjustment+')'):'';
h+='<label style="display:block;padding:8px 0;border-bottom:1px solid #f0f0f0;cursor:pointer;"><input type="'+type+'" name="'+nm+'" data-gi="'+gi+'" data-oi="'+oi+'" data-adj="'+opt.price_adjustment+'" style="margin-right:8px;">'+esc(opt.name)+adj+'</label>';
});
});
h+='<button id="confirmAdd" style="width:100%%;padding:14px;background:#e74c3c;color:#fff;border:none;border-radius:10px;font-size:16px;font-weight:700;cursor:pointer;margin-top:16px;">加入</button>';
box.innerHTML=h;
bg.appendChild(box);
document.body.appendChild(bg);
bg.onclick=function(e){if(e.target===bg){document.body.removeChild(bg);}};
document.getElementById('confirmAdd').onclick=function(){
var adj=0,notes=[];
it.option_groups.forEach(function(og,gi){
var checks=box.querySelectorAll('input[data-gi="'+gi+'"]:checked');
checks.forEach(function(el){var oi=parseInt(el.getAttribute('data-oi'));adj+=parseInt(el.getAttribute('data-adj'));notes.push(og.name+': '+og.options[oi].name);});
});
addToCart(it.id,it.name,it.price+adj,notes.join(', '));
document.body.removeChild(bg);
};
}
function addToCart(id,name,price,notes){
cart.push({id:id,name:name,price:price,qty:1,notes:notes});updateCart();
}
function updateCart(){
var count=0,total=0;
for(var i=0;i<cart.length;i++){count+=cart[i].qty;total+=cart[i].price*cart[i].qty;}
var bar=document.getElementById('cartBar');
if(count===0){bar.classList.remove('visible');return;}
bar.classList.add('visible');
document.getElementById('cartCount').textContent=count+' 項';
document.getElementById('cartTotal').textContent='NT$'+total;
bar.onclick=function(){submitOrder();};
}
function submitOrder(){
var items=cart.map(function(c){return{menu_item_id:c.id,item_name:c.name,quantity:c.qty,unit_price:c.price,notes:c.notes};});
fetch('/api/public/orders/'+slug,{method:'POST',headers:{'Content-Type':'application/json'},
body:JSON.stringify({items:items,table_label:''})})
.then(function(r){return r.json();})
.then(function(d){alert('訂單已送出！訂單編號: '+d.id);cart=[];updateCart();});
}
render();
```

Key changes from original:
- Items with option_groups show "有選項" label
- `handleAdd()` checks for option groups and shows modal
- `showOptionModal()` renders radio (pick-one) or checkbox (pick-many) based on min/max
- Cart stores notes with selected options
- Each add creates a new cart entry (not merged by id) since options may differ
- `submitOrder()` sends notes field with selections

**Step 2: Build and verify**

```bash
go build ./cmd/server
```

Expected: PASS

**Step 3: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: public menu page renders option groups with selection modal"
```

---

### Task 12: Frontend build and full test run

**Files:** (no new changes, just verification)

**Step 1: Build frontend**

```bash
cd frontend && npm run build
```

Expected: PASS

**Step 2: Build server with embedded frontend**

```bash
go build ./cmd/server
```

Expected: PASS

**Step 3: Run all tests**

```bash
go test ./tests -v
```

Expected: all tests pass including the new option group tests.

**Step 4: TypeScript check**

```bash
cd frontend && npx tsc --noEmit
```

Expected: PASS

**Step 5: Commit if any fixes needed, otherwise done**

---

## Task Dependency Graph

```
Task 1 (migration) → Task 2 (queries + sqlc) → Task 3 (schema test)
                                               → Task 4 (OCR types)
                                               → Task 5 (OCR insert)
Task 4 + 5 → Task 6 (OCR prompt + merge)
Task 2 → Task 7 (server buildMenuJSON + fix compilation)
Task 7 → Task 8 (InsertMenu test)
Task 4 → Task 9 (frontend types)
Task 9 → Task 10 (MenuEditor UI)
Task 7 → Task 11 (public menu HTML)
All → Task 12 (full verification)
```

Tasks 4+5 can run in parallel with Task 7. Tasks 9-10 can run in parallel with 11.
