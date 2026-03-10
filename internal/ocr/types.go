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
