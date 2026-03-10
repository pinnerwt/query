package ocr

// MenuData represents a structured restaurant menu.
type MenuData struct {
	Categories []MenuCategory `json:"categories"`
	Combos     []MenuCombo    `json:"combos,omitempty"`
}

// MenuCategory is a group of related menu items.
type MenuCategory struct {
	Name  string     `json:"name"`
	Items []MenuItem `json:"items"`
}

// MenuItem is a single dish or drink.
type MenuItem struct {
	Name        string      `json:"name"`
	Price       int         `json:"price"`
	Description string      `json:"description,omitempty"`
	PriceTiers  []PriceTier `json:"price_tiers,omitempty"`
}

// PriceTier represents a quantity-based price option.
type PriceTier struct {
	Label    string `json:"label"`
	Quantity int    `json:"quantity"`
	Price    int    `json:"price"`
}

// MenuCombo is a set meal with chooseable options.
type MenuCombo struct {
	Name        string       `json:"name"`
	Price       int          `json:"price"`
	Description string       `json:"description,omitempty"`
	Groups      []ComboGroup `json:"groups,omitempty"`
}

// ComboGroup is a selection group within a combo.
type ComboGroup struct {
	Name       string        `json:"name"`
	MinChoices int           `json:"min_choices"`
	MaxChoices int           `json:"max_choices"`
	Options    []ComboOption `json:"options"`
}

// ComboOption is a chooseable item within a combo group.
type ComboOption struct {
	Name            string `json:"name"`
	PriceAdjustment int    `json:"price_adjustment"`
}
