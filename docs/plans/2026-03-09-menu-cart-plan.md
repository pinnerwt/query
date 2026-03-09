# Menu Cart & Multi-Price Items Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add price tiers for quantity-based pricing, expose combos via API, extend OCR to extract both, and build a client-side shopping cart.

**Architecture:** New migration for `menu_item_price_tiers` table. New SQL queries + sqlc regeneration. OCR prompt and struct changes. Server API restructured to return object with categories + combos. Frontend rewritten with cart logic.

**Tech Stack:** PostgreSQL, Goose migrations, sqlc, Go (pgx/v5), vanilla JS

---

### Task 1: Add `menu_item_price_tiers` migration

**Files:**
- Create: `migrations/00004_create_price_tiers.sql`
- Test: `tests/schema_test.go`

**Step 1: Write the migration**

Create `migrations/00004_create_price_tiers.sql`:

```sql
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
```

**Step 2: Write the failing test**

Add to `tests/schema_test.go`:

```go
func TestPriceTiers(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()

	placeID := insertPlace(t, conn, ctx, "tier_test", "Tier Test Place")
	restID := insertRestaurantDetails(t, conn, ctx, placeID)

	// Create a menu item
	var itemID int64
	err := conn.QueryRow(ctx, `
		INSERT INTO menu_items (restaurant_id, name, price)
		VALUES ($1, '法國生蠔', 688)
		RETURNING id
	`, restID).Scan(&itemID)
	require.NoError(t, err)

	// Insert price tiers
	_, err = conn.Exec(ctx, `
		INSERT INTO menu_item_price_tiers (menu_item_id, label, quantity, price, sort_order)
		VALUES ($1, '2入', 2, 688, 0), ($1, '6入', 6, 1680, 1), ($1, '12入', 12, 3280, 2)
	`, itemID)
	require.NoError(t, err)

	// Verify tiers
	var count int
	err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM menu_item_price_tiers WHERE menu_item_id = $1`, itemID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	// Verify cascade delete
	_, err = conn.Exec(ctx, `DELETE FROM menu_items WHERE id = $1`, itemID)
	require.NoError(t, err)

	err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM menu_item_price_tiers WHERE menu_item_id = $1`, itemID).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}
```

**Step 3: Run test to verify it passes**

Run: `go test ./tests -run TestPriceTiers -v`
Expected: PASS (migration auto-applied by dbtest.SetupTestDB)

**Step 4: Commit**

```bash
git add migrations/00004_create_price_tiers.sql tests/schema_test.go
git commit -m "feat: add menu_item_price_tiers table"
```

---

### Task 2: Add SQL queries for price tiers and combos

**Files:**
- Modify: `internal/db/queries/menu.sql`
- Regenerate: `internal/db/generated/` (run `~/go/bin/sqlc generate`)

**Step 1: Add new queries to `internal/db/queries/menu.sql`**

Append these queries:

```sql
-- name: CreatePriceTier :one
INSERT INTO menu_item_price_tiers (menu_item_id, label, quantity, price, sort_order)
VALUES (@menu_item_id, @label, @quantity, @price, @sort_order)
RETURNING *;

-- name: ListPriceTiersByMenuItem :many
SELECT * FROM menu_item_price_tiers WHERE menu_item_id = $1 ORDER BY sort_order;

-- name: ListPriceTiersByRestaurant :many
SELECT pt.* FROM menu_item_price_tiers pt
JOIN menu_items mi ON mi.id = pt.menu_item_id
WHERE mi.restaurant_id = $1
ORDER BY pt.menu_item_id, pt.sort_order;

-- name: DeletePriceTiersByMenuItem :exec
DELETE FROM menu_item_price_tiers WHERE menu_item_id = $1;

-- name: DeleteComboMealsByRestaurant :exec
DELETE FROM combo_meals WHERE restaurant_id = $1;

-- name: ListComboMealsByRestaurant :many
SELECT * FROM combo_meals WHERE restaurant_id = $1 ORDER BY name;

-- name: ListComboMealGroupsByComboMeal :many
SELECT * FROM combo_meal_groups WHERE combo_meal_id = $1 ORDER BY sort_order;

-- name: ListComboMealGroupOptionsByGroup :many
SELECT * FROM combo_meal_group_options WHERE group_id = $1 ORDER BY sort_order;
```

**Step 2: Regenerate sqlc**

Run: `~/go/bin/sqlc generate`
Expected: No errors. New functions appear in `internal/db/generated/menu.sql.go`.

**Step 3: Verify it compiles**

Run: `go build ./...`
Expected: No errors.

**Step 4: Commit**

```bash
git add internal/db/queries/menu.sql internal/db/generated/
git commit -m "feat: add SQL queries for price tiers and combos"
```

---

### Task 3: Extend OCR data structures and prompt

**Files:**
- Modify: `cmd/ocr/main.go`

**Step 1: Update the Go structs**

Replace the `menuData`, `menuCategory`, and `menuItem` structs in `cmd/ocr/main.go`:

```go
type menuData struct {
	Categories []menuCategory `json:"categories"`
	Combos     []menuCombo    `json:"combos,omitempty"`
}

type menuCategory struct {
	Name  string     `json:"name"`
	Items []menuItem `json:"items"`
}

type menuItem struct {
	Name        string      `json:"name"`
	Price       int         `json:"price"`
	Description string      `json:"description,omitempty"`
	PriceTiers  []priceTier `json:"price_tiers,omitempty"`
}

type priceTier struct {
	Label    string `json:"label"`
	Quantity int    `json:"quantity"`
	Price    int    `json:"price"`
}

type menuCombo struct {
	Name        string       `json:"name"`
	Price       int          `json:"price"`
	Description string       `json:"description,omitempty"`
	Groups      []comboGroup `json:"groups,omitempty"`
}

type comboGroup struct {
	Name       string        `json:"name"`
	MinChoices int           `json:"min_choices"`
	MaxChoices int           `json:"max_choices"`
	Options    []comboOption `json:"options"`
}

type comboOption struct {
	Name            string `json:"name"`
	PriceAdjustment int    `json:"price_adjustment"`
}
```

**Step 2: Update the normalization prompt**

Replace the JSON schema section in `normalizePrompt` (the part between "Output ONLY valid JSON" and "Rules:"):

```
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
          ]
        }
      ]
    }
  ],
  "combos": [
    {
      "name": "combo name",
      "price": 198,
      "description": "what is included",
      "groups": [
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
```

Add these rules to the Rules section:

```
- If an item has multiple prices for different quantities (e.g. "Two/NT$688, Six/NT$1,680"), use price_tiers array. Set item price to the lowest tier price.
- If an item has only one price, omit price_tiers (do NOT create a single-entry price_tiers array).
- If the menu has set meals/combos with chooseable options (e.g. "choose a soup", "pick a main"), add them to the combos array with groups and options.
- combos is optional — omit if no set meals are detected.
- price_adjustment in combo options is the extra cost on top of the combo base price (0 if no upcharge).
```

**Step 3: Update the dry-run output to show tiers and combos**

In the `main()` function, after the existing print loop for categories, add:

```go
	if len(menu.Combos) > 0 {
		fmt.Printf("\n=== Combos ===\n")
		for _, combo := range menu.Combos {
			fmt.Printf("\n[%s] %d元 — %s\n", combo.Name, combo.Price, combo.Description)
			for _, g := range combo.Groups {
				fmt.Printf("  %s (choose %d-%d):\n", g.Name, g.MinChoices, g.MaxChoices)
				for _, o := range g.Options {
					adj := ""
					if o.PriceAdjustment != 0 {
						adj = fmt.Sprintf(" (+%d)", o.PriceAdjustment)
					}
					fmt.Printf("    - %s%s\n", o.Name, adj)
				}
			}
		}
	}
```

Also update the existing item print to show tiers:

```go
		for _, item := range cat.Items {
			if len(item.PriceTiers) > 0 {
				tierStrs := make([]string, len(item.PriceTiers))
				for i, t := range item.PriceTiers {
					tierStrs[i] = fmt.Sprintf("%s:%d元", t.Label, t.Price)
				}
				fmt.Printf("  %s — %s\n", item.Name, strings.Join(tierStrs, " / "))
			} else {
				fmt.Printf("  %s — %d元\n", item.Name, item.Price)
			}
			totalItems++
		}
```

**Step 4: Verify it compiles**

Run: `go build ./cmd/ocr`
Expected: No errors.

**Step 5: Commit**

```bash
git add cmd/ocr/main.go
git commit -m "feat: extend OCR structs and prompt for price tiers and combos"
```

---

### Task 4: Update `insertMenu` to write price tiers and combos

**Files:**
- Modify: `cmd/ocr/main.go` (the `insertMenu` function)

**Step 1: Update `insertMenu`**

Replace the entire `insertMenu` function:

```go
func insertMenu(ctx context.Context, q *db.Queries, googlePlaceID string, menu *menuData) error {
	// Look up place
	place, err := q.GetPlaceByGoogleID(ctx, googlePlaceID)
	if err != nil {
		return fmt.Errorf("place %s not found: %w", googlePlaceID, err)
	}

	// Ensure restaurant_details row exists
	restaurant, err := q.UpsertRestaurantDetails(ctx, place.ID)
	if err != nil {
		return fmt.Errorf("upsert restaurant details: %w", err)
	}

	// Clear existing menu data (idempotent)
	_ = q.DeleteMenuItemsByRestaurant(ctx, restaurant.ID)
	_ = q.DeleteMenuCategoriesByRestaurant(ctx, restaurant.ID)
	_ = q.DeleteComboMealsByRestaurant(ctx, restaurant.ID)

	// Insert categories and items
	for i, cat := range menu.Categories {
		category, err := q.CreateMenuCategory(ctx, db.CreateMenuCategoryParams{
			RestaurantID: restaurant.ID,
			Name:         cat.Name,
			SortOrder:    int32(i),
		})
		if err != nil {
			return fmt.Errorf("create category %q: %w", cat.Name, err)
		}

		for _, item := range cat.Items {
			mi, err := q.CreateMenuItem(ctx, db.CreateMenuItemParams{
				RestaurantID: restaurant.ID,
				CategoryID:   pgtype.Int8{Int64: category.ID, Valid: true},
				Name:         item.Name,
				Description:  pgtype.Text{String: item.Description, Valid: item.Description != ""},
				Price:        int32(item.Price),
				PhotoUrl:     pgtype.Text{},
			})
			if err != nil {
				return fmt.Errorf("create item %q: %w", item.Name, err)
			}

			// Insert price tiers
			for j, tier := range item.PriceTiers {
				_, err := q.CreatePriceTier(ctx, db.CreatePriceTierParams{
					MenuItemID: mi.ID,
					Label:      tier.Label,
					Quantity:   int32(tier.Quantity),
					Price:      int32(tier.Price),
					SortOrder:  int32(j),
				})
				if err != nil {
					return fmt.Errorf("create price tier %q for %q: %w", tier.Label, item.Name, err)
				}
			}
		}
	}

	// Insert combos
	for _, combo := range menu.Combos {
		cm, err := q.CreateComboMeal(ctx, db.CreateComboMealParams{
			RestaurantID: restaurant.ID,
			Name:         combo.Name,
			Description:  pgtype.Text{String: combo.Description, Valid: combo.Description != ""},
			Price:        int32(combo.Price),
		})
		if err != nil {
			return fmt.Errorf("create combo %q: %w", combo.Name, err)
		}

		for i, group := range combo.Groups {
			g, err := q.CreateComboMealGroup(ctx, db.CreateComboMealGroupParams{
				ComboMealID: cm.ID,
				Name:        group.Name,
				MinChoices:  int32(group.MinChoices),
				MaxChoices:  int32(group.MaxChoices),
				SortOrder:   int32(i),
			})
			if err != nil {
				return fmt.Errorf("create combo group %q: %w", group.Name, err)
			}

			for j, opt := range group.Options {
				_, err := q.CreateComboMealGroupOption(ctx, db.CreateComboMealGroupOptionParams{
					GroupID:         g.ID,
					MenuItemID:      pgtype.Int8{},
					ItemName:        pgtype.Text{String: opt.Name, Valid: true},
					PriceAdjustment: int32(opt.PriceAdjustment),
					SortOrder:       int32(j),
				})
				if err != nil {
					return fmt.Errorf("create combo option %q: %w", opt.Name, err)
				}
			}
		}
	}

	return nil
}
```

**Step 2: Verify it compiles**

Run: `go build ./cmd/ocr`
Expected: No errors.

**Step 3: Commit**

```bash
git add cmd/ocr/main.go
git commit -m "feat: insertMenu writes price tiers and combos to DB"
```

---

### Task 5: Update server API to return price tiers and combos

**Files:**
- Modify: `cmd/server/main.go` (the `handleMenu` function)

**Step 1: Rewrite `handleMenu`**

Replace the entire `handleMenu` method:

```go
func (s *server) handleMenu(w http.ResponseWriter, r *http.Request) {
	ridStr := r.URL.Query().Get("restaurant_id")
	rid, err := strconv.ParseInt(ridStr, 10, 64)
	if err != nil {
		http.Error(w, "restaurant_id required", 400)
		return
	}

	categories, err := s.q.ListMenuCategoriesByRestaurant(r.Context(), rid)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	items, err := s.q.ListMenuItemsByRestaurant(r.Context(), rid)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	tiers, err := s.q.ListPriceTiersByRestaurant(r.Context(), rid)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// Build tier lookup: menu_item_id -> []tierOut
	type tierOut struct {
		Label    string `json:"label"`
		Quantity int32  `json:"quantity"`
		Price    int32  `json:"price"`
	}
	tierMap := make(map[int64][]tierOut)
	for _, t := range tiers {
		tierMap[t.MenuItemID] = append(tierMap[t.MenuItemID], tierOut{
			Label:    t.Label,
			Quantity: t.Quantity,
			Price:    t.Price,
		})
	}

	type menuItemOut struct {
		ID          int64      `json:"id"`
		Name        string     `json:"name"`
		Price       int32      `json:"price"`
		Description string     `json:"description,omitempty"`
		PriceTiers  []tierOut  `json:"price_tiers,omitempty"`
	}

	type categoryOut struct {
		Name  string        `json:"name"`
		Items []menuItemOut `json:"items"`
	}

	catIdx := make(map[int64]int)
	var cats []categoryOut
	for _, c := range categories {
		catIdx[c.ID] = len(cats)
		cats = append(cats, categoryOut{Name: c.Name})
	}

	for _, it := range items {
		mi := menuItemOut{
			ID:          it.ID,
			Name:        it.Name,
			Price:       it.Price,
			Description: it.Description.String,
			PriceTiers:  tierMap[it.ID],
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

	// Build combos
	type optionOut struct {
		Name       string `json:"name"`
		Adjustment int32  `json:"adjustment"`
	}
	type groupOut struct {
		Name    string      `json:"name"`
		Min     int32       `json:"min"`
		Max     int32       `json:"max"`
		Options []optionOut `json:"options"`
	}
	type comboOut struct {
		ID          int64      `json:"id"`
		Name        string     `json:"name"`
		Price       int32      `json:"price"`
		Description string     `json:"description,omitempty"`
		Groups      []groupOut `json:"groups"`
	}

	combos, err := s.q.ListComboMealsByRestaurant(r.Context(), rid)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	var combosOut []comboOut
	for _, cm := range combos {
		co := comboOut{
			ID:          cm.ID,
			Name:        cm.Name,
			Price:       cm.Price,
			Description: cm.Description.String,
		}

		groups, err := s.q.ListComboMealGroupsByComboMeal(r.Context(), cm.ID)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		for _, g := range groups {
			go_ := groupOut{
				Name: g.Name,
				Min:  g.MinChoices,
				Max:  g.MaxChoices,
			}

			options, err := s.q.ListComboMealGroupOptionsByGroup(r.Context(), g.ID)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}

			for _, o := range options {
				name := o.ItemName.String
				if name == "" && o.MenuItemID.Valid {
					name = fmt.Sprintf("item#%d", o.MenuItemID.Int64)
				}
				go_.Options = append(go_.Options, optionOut{
					Name:       name,
					Adjustment: o.PriceAdjustment,
				})
			}
			co.Groups = append(co.Groups, go_)
		}
		combosOut = append(combosOut, co)
	}

	type menuResponse struct {
		Categories []categoryOut `json:"categories"`
		Combos     []comboOut    `json:"combos,omitempty"`
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(menuResponse{
		Categories: cats,
		Combos:     combosOut,
	})
}
```

**Step 2: Verify it compiles**

Run: `go build ./cmd/server`
Expected: No errors.

**Step 3: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: menu API returns price tiers and combos"
```

---

### Task 6: Rewrite frontend with cart functionality

**Files:**
- Modify: `cmd/server/main.go` (the `indexHTML` constant)

**Step 1: Replace the `indexHTML` constant**

This is the largest task. Replace the entire `const indexHTML = ...` block with the new frontend that includes:

1. **Menu display** — items with "+" buttons, price tiers shown as options, combos in their own section
2. **Cart panel** — sticky bottom bar with count/total, expandable to show line items with +/- controls
3. **Tier picker** — when an item has price_tiers, tapping "+" shows a dropdown to select tier
4. **Combo builder** — tapping a combo opens inline group selection, then adds assembled combo to cart
5. **localStorage persistence** — cart saved per restaurant_id

The HTML/CSS/JS should maintain the existing visual style (480px max-width, red theme, sticky headers) and add:

**CSS additions:**
- `.add-btn` — round "+" button on each item (28px, right side of menu-item row)
- `.tier-picker` — dropdown/inline selector for price tier options
- `.combo-section` — styled section below categories for combo meals
- `.combo-builder` — inline flow for selecting combo group options
- `.cart-bar` — sticky bottom bar (60px), shows "🛒 N items — NT$total", tap to expand
- `.cart-panel` — expanded cart view with line items, +/- buttons, subtotals, clear button
- `.cart-item` — single line in expanded cart

**JS additions:**
- `cart` object: `{ restaurantId, items: [{id, name, price, qty, tierLabel?, comboChoices?}] }`
- `addToCart(itemId, name, price, tierLabel)` — add or increment item
- `addComboToCart(comboId, name, basePrice, choices)` — add assembled combo
- `removeFromCart(index)` — remove line item
- `updateQty(index, delta)` — increment/decrement
- `saveCart()` / `loadCart(rid)` — localStorage read/write
- `renderCart()` — update cart bar count/total and panel contents
- `showTierPicker(itemId, name, tiers)` — show tier selection UI
- `showComboBuilder(combo)` — step through groups, collect choices

**Key behavior:**
- Items with `price < 0` (unknown/市價) show no "+" button
- Items with `price >= 0` and no tiers: direct add on "+"
- Items with price_tiers: "+" opens tier picker, then adds selected tier
- Combos: show base price, click opens builder, must satisfy all group min/max before adding
- Cart bar always visible when cart has items
- Cart total updates live
- "返回餐廳列表" clears the displayed cart (but localStorage keeps it for when user returns)

**The API response format changed from array to object.** Update `showMenu()`:
- `const data = await res.json()` now returns `{categories: [...], combos: [...]}` instead of `[...]`
- Render `data.categories` for menu items
- Render `data.combos` for combo section (if non-empty)

**Step 2: Verify it compiles**

Run: `go build ./cmd/server`
Expected: No errors (it's just a string constant).

**Step 3: Manual test**

Run: `DATABASE_URL="postgres://query:query@localhost:5432/query?sslmode=disable" ./server -addr :8081`
Open `http://localhost:8081`, click a restaurant, verify:
- Items show "+" buttons
- Cart bar appears when items added
- Cart total is correct
- Cart persists on page refresh

**Step 4: Commit**

```bash
git add cmd/server/main.go
git commit -m "feat: frontend with shopping cart, tier picker, and combo builder"
```

---

### Task 7: Re-run OCR to populate tiers and combos

This is a manual verification task, not code.

**Step 1: Re-run OCR on 隨意鳥地方 (has price tiers)**

```bash
DATABASE_URL="postgres://query:query@localhost:5432/query?sslmode=disable"
./ocr --dry-run ChIJ1S3Z6barQjQRUnd4BwJ6NkY
```

Verify output shows price_tiers for the oyster item and any detected combos.

**Step 2: If output looks correct, run without dry-run**

```bash
./ocr --db "$DATABASE_URL" ChIJ1S3Z6barQjQRUnd4BwJ6NkY
```

**Step 3: Re-run OCR on 阿達師 (has combos/sets)**

```bash
./ocr --dry-run ChIJjTJNOfWrQjQRrYQoAWocmwE
```

Verify combos are detected. Run without dry-run if good.

**Step 4: Verify on frontend**

Open `http://localhost:8081`, check 隨意鳥地方:
- Oyster item shows tier options when "+" is clicked
- Cart correctly prices selected tiers

Check 阿達師:
- Combo section visible
- Combo builder lets you select options per group
- Cart total correct with combo price + adjustments

**Step 5: Re-run remaining restaurants**

```bash
for pid in ChIJT7dRIq6rQjQR997I-m_QXxc ChIJCwRp06SrQjQRaVxKW4WlTf8 ChIJQXcl6LarQjQRGUMnQ18F0lE; do
  ./ocr --db "$DATABASE_URL" "$pid"
done
```

**Step 6: Commit any prompt tweaks made during testing**

```bash
git add cmd/ocr/main.go
git commit -m "fix: tune OCR prompt for tier/combo extraction"
```
