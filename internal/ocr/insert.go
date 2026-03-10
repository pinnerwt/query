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
