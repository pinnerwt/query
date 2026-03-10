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
	_ = q.DeleteComboMealsByRestaurant(ctx, restaurantID)

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
		}
	}

	// Insert combo meals
	for _, combo := range menu.Combos {
		cm, err := q.CreateComboMeal(ctx, db.CreateComboMealParams{
			RestaurantID: restaurantID,
			Name:         combo.Name,
			Description:  pgtype.Text{String: combo.Description, Valid: combo.Description != ""},
			Price:        int32(combo.Price),
		})
		if err != nil {
			return fmt.Errorf("create combo %q: %w", combo.Name, err)
		}

		for gi, group := range combo.Groups {
			cg, err := q.CreateComboMealGroup(ctx, db.CreateComboMealGroupParams{
				ComboMealID: cm.ID,
				Name:        group.Name,
				MinChoices:  int32(group.MinChoices),
				MaxChoices:  int32(group.MaxChoices),
				SortOrder:   int32(gi),
			})
			if err != nil {
				return fmt.Errorf("create combo group %q: %w", group.Name, err)
			}

			for oi, opt := range group.Options {
				_, err := q.CreateComboMealGroupOption(ctx, db.CreateComboMealGroupOptionParams{
					GroupID:         cg.ID,
					MenuItemID:      pgtype.Int8{},
					ItemName:        pgtype.Text{String: opt.Name, Valid: opt.Name != ""},
					PriceAdjustment: int32(opt.PriceAdjustment),
					SortOrder:       int32(oi),
				})
				if err != nil {
					return fmt.Errorf("create combo option %q: %w", opt.Name, err)
				}
			}
		}
	}

	return nil
}
