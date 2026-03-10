package ocr

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFixDescriptionPrices(t *testing.T) {
	t.Run("converts 套餐 price to tier", func(t *testing.T) {
		menu := &MenuData{
			Categories: []MenuCategory{{
				Name: "主食",
				Items: []MenuItem{
					{Name: "椒麻雞飕排麺", Price: 188, Description: "套餐 $258"},
				},
			}},
		}
		mockLLM := func(prompt string) (string, error) {
			return `[{"name":"椒麻雞飕排麺","price":188,"description":"","price_tiers":[{"label":"單點","quantity":1,"price":188},{"label":"套餐","quantity":1,"price":258}]}]`, nil
		}
		err := FixDescriptionPrices(menu, mockLLM)
		require.NoError(t, err)
		item := menu.Categories[0].Items[0]
		assert.Empty(t, item.Description)
		assert.Len(t, item.PriceTiers, 2)
		assert.Equal(t, 188, item.PriceTiers[0].Price)
		assert.Equal(t, 258, item.PriceTiers[1].Price)
	})

	t.Run("handles multiple items", func(t *testing.T) {
		menu := &MenuData{
			Categories: []MenuCategory{{
				Name: "麵類",
				Items: []MenuItem{
					{Name: "椒麻雞飕排麺", Price: 188, Description: "套餐 $258"},
					{Name: "雪菜肉絲湯麵", Price: 138, Description: "套餐$208"},
				},
			}},
		}
		mockLLM := func(prompt string) (string, error) {
			assert.Contains(t, prompt, "椒麻雞飕排麺")
			assert.Contains(t, prompt, "雪菜肉絲湯麵")
			return `[
				{"name":"椒麻雞飕排麺","price":188,"description":"","price_tiers":[{"label":"單點","quantity":1,"price":188},{"label":"套餐","quantity":1,"price":258}]},
				{"name":"雪菜肉絲湯麵","price":138,"description":"","price_tiers":[{"label":"單點","quantity":1,"price":138},{"label":"套餐","quantity":1,"price":208}]}
			]`, nil
		}
		err := FixDescriptionPrices(menu, mockLLM)
		require.NoError(t, err)
		assert.Len(t, menu.Categories[0].Items[0].PriceTiers, 2)
		assert.Len(t, menu.Categories[0].Items[1].PriceTiers, 2)
	})

	t.Run("skips items without price indicators", func(t *testing.T) {
		menu := &MenuData{
			Categories: []MenuCategory{{
				Name: "飲料",
				Items: []MenuItem{
					{Name: "紅茶", Price: 30, Description: "冰/熱皆可"},
				},
			}},
		}
		called := false
		mockLLM := func(prompt string) (string, error) {
			called = true
			return "[]", nil
		}
		err := FixDescriptionPrices(menu, mockLLM)
		require.NoError(t, err)
		assert.False(t, called, "LLM should not be called when no price indicators found")
		assert.Equal(t, "冰/熱皆可", menu.Categories[0].Items[0].Description)
	})

	t.Run("skips items with special price values", func(t *testing.T) {
		menu := &MenuData{
			Categories: []MenuCategory{{
				Name: "海鮮",
				Items: []MenuItem{
					{Name: "龍蝦", Price: -1, Description: "套餐 $500"},
					{Name: "鮑魚", Price: -2, Description: "套餐 $800"},
				},
			}},
		}
		called := false
		mockLLM := func(prompt string) (string, error) {
			called = true
			return "[]", nil
		}
		err := FixDescriptionPrices(menu, mockLLM)
		require.NoError(t, err)
		assert.False(t, called)
		assert.Equal(t, "套餐 $500", menu.Categories[0].Items[0].Description)
	})

	t.Run("skips items already having price tiers", func(t *testing.T) {
		menu := &MenuData{
			Categories: []MenuCategory{{
				Name: "主食",
				Items: []MenuItem{
					{Name: "水餃", Price: 80, Description: "套餐 $150", PriceTiers: []PriceTier{
						{Label: "6入", Price: 80},
						{Label: "12入", Price: 150},
					}},
				},
			}},
		}
		called := false
		mockLLM := func(prompt string) (string, error) {
			called = true
			return "[]", nil
		}
		err := FixDescriptionPrices(menu, mockLLM)
		require.NoError(t, err)
		assert.False(t, called)
		assert.Equal(t, "套餐 $150", menu.Categories[0].Items[0].Description)
	})

	t.Run("preserves option_groups on patched items", func(t *testing.T) {
		menu := &MenuData{
			Categories: []MenuCategory{{
				Name: "麵類",
				Items: []MenuItem{
					{
						Name:        "椒麻雞飕排麺",
						Price:       188,
						Description: "套餐 $258",
						OptionGroups: []OptionGroup{
							{Name: "湯品更換", MinChoices: 0, MaxChoices: 1, Options: []OptionChoice{
								{Name: "牛麻辣四寶湯", PriceAdjustment: 0},
							}},
						},
					},
				},
			}},
		}
		mockLLM := func(prompt string) (string, error) {
			return `[{"name":"椒麻雞飕排麺","price":188,"description":"","price_tiers":[{"label":"單點","quantity":1,"price":188},{"label":"套餐","quantity":1,"price":258}]}]`, nil
		}
		err := FixDescriptionPrices(menu, mockLLM)
		require.NoError(t, err)
		item := menu.Categories[0].Items[0]
		assert.Len(t, item.PriceTiers, 2)
		assert.Len(t, item.OptionGroups, 1, "option_groups should be preserved")
		assert.Equal(t, "湯品更換", item.OptionGroups[0].Name)
	})

	t.Run("handles LLM error gracefully", func(t *testing.T) {
		menu := &MenuData{
			Categories: []MenuCategory{{
				Name: "主食",
				Items: []MenuItem{
					{Name: "雞排", Price: 80, Description: "套餐 $150"},
				},
			}},
		}
		mockLLM := func(prompt string) (string, error) {
			return "", fmt.Errorf("connection refused")
		}
		err := FixDescriptionPrices(menu, mockLLM)
		assert.Error(t, err)
		// Original data should be unchanged
		assert.Equal(t, "套餐 $150", menu.Categories[0].Items[0].Description)
		assert.Empty(t, menu.Categories[0].Items[0].PriceTiers)
	})

	t.Run("handles LLM count mismatch gracefully", func(t *testing.T) {
		menu := &MenuData{
			Categories: []MenuCategory{{
				Name: "主食",
				Items: []MenuItem{
					{Name: "雞排", Price: 80, Description: "套餐 $150"},
				},
			}},
		}
		mockLLM := func(prompt string) (string, error) {
			return `[]`, nil // returns 0 items instead of 1
		}
		err := FixDescriptionPrices(menu, mockLLM)
		assert.Error(t, err)
		assert.Equal(t, "套餐 $150", menu.Categories[0].Items[0].Description)
	})

	t.Run("detects 元 price indicator", func(t *testing.T) {
		menu := &MenuData{
			Categories: []MenuCategory{{
				Name: "主食",
				Items: []MenuItem{
					{Name: "牛肉麵", Price: 180, Description: "大碗 250元"},
				},
			}},
		}
		mockLLM := func(prompt string) (string, error) {
			return `[{"name":"牛肉麵","price":180,"description":"","price_tiers":[{"label":"普通","quantity":1,"price":180},{"label":"大碗","quantity":1,"price":250}]}]`, nil
		}
		err := FixDescriptionPrices(menu, mockLLM)
		require.NoError(t, err)
		item := menu.Categories[0].Items[0]
		assert.Len(t, item.PriceTiers, 2)
		assert.Equal(t, 250, item.PriceTiers[1].Price)
	})
}

func TestCleanLLMResponse(t *testing.T) {
	t.Run("strips think blocks", func(t *testing.T) {
		input := `<think>reasoning here</think>{"result": true}`
		assert.Equal(t, `{"result": true}`, cleanLLMResponse(input))
	})

	t.Run("strips multiple think blocks", func(t *testing.T) {
		input := `<think>first</think>hello<think>second</think>world`
		assert.Equal(t, "helloworld", cleanLLMResponse(input))
	})

	t.Run("strips orphaned closing tag", func(t *testing.T) {
		input := `</think>{"data": 1}`
		assert.Equal(t, `{"data": 1}`, cleanLLMResponse(input))
	})

	t.Run("strips code fences", func(t *testing.T) {
		input := "```json\n{\"a\": 1}\n```"
		assert.Equal(t, `{"a": 1}`, cleanLLMResponse(input))
	})
}
