package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	db "github.com/pinnertw/query/internal/db/generated"
)

func main() {
	dbURL := flag.String("db", "", "PostgreSQL connection string (or set DATABASE_URL env var)")
	addr := flag.String("addr", ":8080", "Listen address")
	flag.Parse()

	connStr := *dbURL
	if connStr == "" {
		connStr = os.Getenv("DATABASE_URL")
	}
	if connStr == "" {
		log.Fatal("--db or DATABASE_URL is required")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer pool.Close()

	q := db.New(pool)
	s := &server{q: q}

	http.HandleFunc("/", s.handleIndex)
	http.HandleFunc("/debug", s.handleDebug)
	http.HandleFunc("/api/restaurants", s.handleRestaurants)
	http.HandleFunc("/api/menu", s.handleMenu)
	http.HandleFunc("/api/places", s.handlePlaces)

	fmt.Printf("Server listening on %s\n", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

type server struct {
	q *db.Queries
}

func (s *server) handleRestaurants(w http.ResponseWriter, r *http.Request) {
	restaurants, err := s.q.ListRestaurantsWithMenus(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	type item struct {
		RestaurantID int64  `json:"restaurant_id"`
		Name         string `json:"name"`
		Address      string `json:"address"`
	}

	var out []item
	for _, r := range restaurants {
		out = append(out, item{
			RestaurantID: r.RestaurantID,
			Name:         r.Name,
			Address:      r.Address.String,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (s *server) handleMenu(w http.ResponseWriter, r *http.Request) {
	ridStr := r.URL.Query().Get("restaurant_id")
	rid, err := strconv.ParseInt(ridStr, 10, 64)
	if err != nil {
		http.Error(w, "restaurant_id required", 400)
		return
	}

	ctx := r.Context()

	categories, err := s.q.ListMenuCategoriesByRestaurant(ctx, rid)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	items, err := s.q.ListMenuItemsByRestaurant(ctx, rid)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	// Fetch all price tiers for this restaurant and build a map by menu_item_id.
	priceTiers, err := s.q.ListPriceTiersByRestaurant(ctx, rid)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	type priceTierOut struct {
		Label    string `json:"label"`
		Quantity int32  `json:"quantity"`
		Price    int32  `json:"price"`
	}

	tierMap := make(map[int64][]priceTierOut)
	for _, pt := range priceTiers {
		tierMap[pt.MenuItemID] = append(tierMap[pt.MenuItemID], priceTierOut{
			Label:    pt.Label,
			Quantity: pt.Quantity,
			Price:    pt.Price,
		})
	}

	type menuItem struct {
		ID          int64          `json:"id"`
		Name        string         `json:"name"`
		Price       int32          `json:"price"`
		Description string         `json:"description,omitempty"`
		PriceTiers  []priceTierOut `json:"price_tiers,omitempty"`
	}

	type category struct {
		Name  string     `json:"name"`
		Items []menuItem `json:"items"`
	}

	catIdx := make(map[int64]int)
	var cats []category
	for _, c := range categories {
		catIdx[c.ID] = len(cats)
		cats = append(cats, category{Name: c.Name})
	}

	for _, it := range items {
		mi := menuItem{
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
		// Uncategorized
		if len(cats) == 0 || cats[len(cats)-1].Name != "其他" {
			cats = append(cats, category{Name: "其他"})
		}
		cats[len(cats)-1].Items = append(cats[len(cats)-1].Items, mi)
	}

	// Fetch combos for this restaurant.
	comboMeals, err := s.q.ListComboMealsByRestaurant(ctx, rid)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	type comboOption struct {
		Name       string `json:"name"`
		Adjustment int32  `json:"adjustment"`
	}

	type comboGroup struct {
		Name    string        `json:"name"`
		Min     int32         `json:"min"`
		Max     int32         `json:"max"`
		Options []comboOption `json:"options"`
	}

	type comboOut struct {
		ID          int64        `json:"id"`
		Name        string       `json:"name"`
		Price       int32        `json:"price"`
		Description string       `json:"description,omitempty"`
		Groups      []comboGroup `json:"groups"`
	}

	var combos []comboOut
	for _, cm := range comboMeals {
		groups, err := s.q.ListComboMealGroupsByComboMeal(ctx, cm.ID)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		var grpOut []comboGroup
		for _, g := range groups {
			options, err := s.q.ListComboMealGroupOptionsByGroup(ctx, g.ID)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}

			var opts []comboOption
			for _, o := range options {
				name := o.ItemName.String
				opts = append(opts, comboOption{
					Name:       name,
					Adjustment: o.PriceAdjustment,
				})
			}

			grpOut = append(grpOut, comboGroup{
				Name:    g.Name,
				Min:     g.MinChoices,
				Max:     g.MaxChoices,
				Options: opts,
			})
		}

		combos = append(combos, comboOut{
			ID:          cm.ID,
			Name:        cm.Name,
			Price:       cm.Price,
			Description: cm.Description.String,
			Groups:      grpOut,
		})
	}

	type menuResponse struct {
		Categories []category `json:"categories"`
		Combos     []comboOut `json:"combos"`
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(menuResponse{
		Categories: cats,
		Combos:     combos,
	})
}

func (s *server) handlePlaces(w http.ResponseWriter, r *http.Request) {
	places, err := s.q.ListAllPlaces(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	type placeOut struct {
		ID            int64    `json:"id"`
		GooglePlaceID string   `json:"google_place_id"`
		Name          string   `json:"name"`
		Address       string   `json:"address,omitempty"`
		Latitude      float64  `json:"latitude,omitempty"`
		Longitude     float64  `json:"longitude,omitempty"`
		PhoneNumber   string   `json:"phone_number,omitempty"`
		Website       string   `json:"website,omitempty"`
		GoogleMapsURL string   `json:"google_maps_url,omitempty"`
		Rating        string   `json:"rating,omitempty"`
		TotalRatings  int32    `json:"total_ratings,omitempty"`
		PriceLevel    int16    `json:"price_level,omitempty"`
		PlaceTypes    []string `json:"place_types,omitempty"`
	}

	var out []placeOut
	for _, p := range places {
		o := placeOut{
			ID:            p.ID,
			GooglePlaceID: p.GooglePlaceID,
			Name:          p.Name,
			PlaceTypes:    p.PlaceTypes,
		}
		if p.Address.Valid {
			o.Address = p.Address.String
		}
		if p.Latitude.Valid {
			o.Latitude = p.Latitude.Float64
		}
		if p.Longitude.Valid {
			o.Longitude = p.Longitude.Float64
		}
		if p.PhoneNumber.Valid {
			o.PhoneNumber = p.PhoneNumber.String
		}
		if p.Website.Valid {
			o.Website = p.Website.String
		}
		if p.GoogleMapsUrl.Valid {
			o.GoogleMapsURL = p.GoogleMapsUrl.String
		}
		if p.Rating.Valid {
			o.Rating = p.Rating.Int.String()
			if p.Rating.Exp < 0 {
				// e.g. Int=46 Exp=-1 → "4.6"
				s := p.Rating.Int.String()
				exp := int(-p.Rating.Exp)
				if exp < len(s) {
					o.Rating = s[:len(s)-exp] + "." + s[len(s)-exp:]
				}
			}
		}
		if p.TotalRatings.Valid {
			o.TotalRatings = p.TotalRatings.Int32
		}
		if p.PriceLevel.Valid {
			o.PriceLevel = p.PriceLevel.Int16
		}
		out = append(out, o)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, indexHTML)
}

func (s *server) handleDebug(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, debugHTML)
}

const indexHTML = `<!DOCTYPE html>
<html lang="zh-TW">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>餐廳菜單</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, "Noto Sans TC", "Microsoft JhengHei", sans-serif; background: #f5f5f5; color: #333; }
.container { max-width: 480px; margin: 0 auto; min-height: 100vh; background: #fff; padding-bottom: 70px; }
header { background: #e74c3c; color: #fff; padding: 16px 20px; position: sticky; top: 0; z-index: 10; }
header h1 { font-size: 20px; }
header .back { cursor: pointer; font-size: 14px; margin-top: 4px; opacity: 0.8; display: none; }
header .back:hover { opacity: 1; }

/* Restaurant list */
.restaurant-list { padding: 8px 0; }
.restaurant-item { padding: 16px 20px; border-bottom: 1px solid #eee; cursor: pointer; transition: background 0.15s; }
.restaurant-item:hover { background: #fafafa; }
.restaurant-item .name { font-size: 17px; font-weight: 600; }
.restaurant-item .addr { font-size: 13px; color: #999; margin-top: 4px; }

/* Menu view */
.menu-view { display: none; }
.category { margin-bottom: 8px; }
.category-header { background: #fafafa; padding: 12px 20px; font-size: 15px; font-weight: 700; color: #e74c3c; border-bottom: 1px solid #eee; position: sticky; top: 56px; z-index: 5; }
.menu-item { display: flex; justify-content: space-between; align-items: center; padding: 14px 20px; border-bottom: 1px solid #f0f0f0; }
.menu-item:last-child { border-bottom: none; }
.item-info { flex: 1; min-width: 0; }
.item-name { font-size: 16px; font-weight: 500; }
.item-desc { font-size: 12px; color: #999; margin-top: 2px; }
.item-right { display: flex; align-items: center; gap: 10px; margin-left: 12px; flex-shrink: 0; }
.item-price { font-size: 16px; font-weight: 700; color: #e74c3c; white-space: nowrap; }
.item-price.unknown { color: #999; font-weight: 500; }

/* Add button */
.add-btn { width: 28px; height: 28px; border-radius: 50%; background: #e74c3c; color: #fff; border: none; font-size: 18px; line-height: 28px; text-align: center; cursor: pointer; flex-shrink: 0; display: flex; align-items: center; justify-content: center; }
.add-btn:hover { background: #c0392b; }
.add-btn:active { transform: scale(0.92); }

/* Modal overlay */
.modal-overlay { display: none; position: fixed; top: 0; left: 0; right: 0; bottom: 0; background: rgba(0,0,0,0.5); z-index: 100; justify-content: center; align-items: flex-end; }
.modal-overlay.active { display: flex; }
.modal-content { background: #fff; width: 100%; max-width: 480px; border-radius: 16px 16px 0 0; padding: 24px 20px; max-height: 80vh; overflow-y: auto; }
.modal-title { font-size: 18px; font-weight: 700; margin-bottom: 16px; }
.modal-close { position: absolute; top: 16px; right: 20px; font-size: 24px; cursor: pointer; color: #999; background: none; border: none; }

/* Tier picker */
.tier-option { display: flex; justify-content: space-between; align-items: center; padding: 14px 16px; border: 1px solid #eee; border-radius: 10px; margin-bottom: 10px; cursor: pointer; transition: background 0.15s, border-color 0.15s; }
.tier-option:hover { background: #fef5f5; border-color: #e74c3c; }
.tier-label { font-size: 16px; font-weight: 500; }
.tier-price { font-size: 16px; font-weight: 700; color: #e74c3c; }

/* Combo builder */
.combo-group-title { font-size: 15px; font-weight: 700; color: #e74c3c; margin-bottom: 4px; }
.combo-group-hint { font-size: 12px; color: #999; margin-bottom: 10px; }
.combo-option { display: flex; justify-content: space-between; align-items: center; padding: 12px 16px; border: 1px solid #eee; border-radius: 10px; margin-bottom: 8px; cursor: pointer; transition: background 0.15s, border-color 0.15s; }
.combo-option:hover { background: #fef5f5; border-color: #e74c3c; }
.combo-option.selected { background: #fef5f5; border-color: #e74c3c; }
.combo-option .check { width: 20px; height: 20px; border-radius: 50%; border: 2px solid #ccc; margin-right: 10px; display: flex; align-items: center; justify-content: center; flex-shrink: 0; }
.combo-option.selected .check { border-color: #e74c3c; background: #e74c3c; color: #fff; font-size: 12px; }
.combo-option-left { display: flex; align-items: center; }
.combo-option-adj { font-size: 14px; color: #999; }
.combo-total { font-size: 17px; font-weight: 700; color: #e74c3c; text-align: center; margin: 16px 0 8px; }
.combo-add-btn { width: 100%; padding: 14px; background: #e74c3c; color: #fff; border: none; border-radius: 10px; font-size: 16px; font-weight: 700; cursor: pointer; }
.combo-add-btn:disabled { background: #ccc; cursor: not-allowed; }
.combo-steps { display: flex; gap: 6px; margin-bottom: 16px; }
.combo-step { width: 8px; height: 8px; border-radius: 50%; background: #ddd; }
.combo-step.active { background: #e74c3c; }
.combo-step.done { background: #2ecc71; }

/* Cart bar */
.cart-bar { display: none; position: fixed; bottom: 0; left: 50%; transform: translateX(-50%); width: 100%; max-width: 480px; height: 60px; background: #333; color: #fff; z-index: 50; cursor: pointer; padding: 0 20px; align-items: center; justify-content: space-between; font-size: 16px; font-weight: 600; }
.cart-bar.visible { display: flex; }
.cart-bar-left { display: flex; align-items: center; gap: 8px; }
.cart-bar-total { font-size: 18px; font-weight: 700; }

/* Cart panel */
.cart-panel { display: none; position: fixed; bottom: 60px; left: 50%; transform: translateX(-50%); width: 100%; max-width: 480px; max-height: 60vh; background: #fff; z-index: 49; border-radius: 16px 16px 0 0; box-shadow: 0 -4px 20px rgba(0,0,0,0.15); overflow-y: auto; padding: 20px; }
.cart-panel.visible { display: block; }
.cart-panel-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.cart-panel-title { font-size: 18px; font-weight: 700; }
.cart-panel-close { font-size: 24px; cursor: pointer; color: #999; background: none; border: none; }
.cart-item { display: flex; align-items: center; padding: 12px 0; border-bottom: 1px solid #f0f0f0; }
.cart-item:last-child { border-bottom: none; }
.cart-item-info { flex: 1; min-width: 0; }
.cart-item-name { font-size: 15px; font-weight: 500; }
.cart-item-detail { font-size: 12px; color: #999; margin-top: 2px; }
.cart-item-right { display: flex; align-items: center; gap: 10px; flex-shrink: 0; }
.cart-item-subtotal { font-size: 15px; font-weight: 700; color: #e74c3c; min-width: 60px; text-align: right; }
.qty-controls { display: flex; align-items: center; gap: 0; }
.qty-btn { width: 28px; height: 28px; border: 1px solid #ddd; background: #fff; color: #333; font-size: 16px; cursor: pointer; display: flex; align-items: center; justify-content: center; }
.qty-btn:first-child { border-radius: 6px 0 0 6px; }
.qty-btn:last-child { border-radius: 0 6px 6px 0; }
.qty-btn:hover { background: #f5f5f5; }
.qty-val { width: 32px; height: 28px; border-top: 1px solid #ddd; border-bottom: 1px solid #ddd; display: flex; align-items: center; justify-content: center; font-size: 14px; font-weight: 600; }
.clear-cart-btn { width: 100%; padding: 12px; background: #fff; color: #e74c3c; border: 1px solid #e74c3c; border-radius: 10px; font-size: 14px; font-weight: 600; cursor: pointer; margin-top: 16px; }
.clear-cart-btn:hover { background: #fef5f5; }

.loading { text-align: center; padding: 40px; color: #999; }
.empty { text-align: center; padding: 60px 20px; color: #999; }
</style>
</head>
<body>
<div class="container">
  <header>
    <div class="back" id="back" onclick="showList()">&#8592; 返回餐廳列表</div>
    <h1 id="title">餐廳菜單</h1>
    <a href="/debug" style="position:absolute;top:16px;right:20px;color:rgba(255,255,255,0.5);font-size:12px;text-decoration:none;">Debug</a>
  </header>
  <div id="list" class="restaurant-list">
    <div class="loading">載入中...</div>
  </div>
  <div id="menu" class="menu-view"></div>
</div>

<!-- Modal overlay for tier picker / combo builder -->
<div class="modal-overlay" id="modalOverlay" onclick="closeModalOnBackdrop(event)">
  <div class="modal-content" id="modalContent"></div>
</div>

<!-- Cart bar -->
<div class="cart-bar" id="cartBar" onclick="toggleCartPanel()">
  <div class="cart-bar-left">
    <span id="cartBarIcon">&#128722;</span>
    <span id="cartBarCount"></span>
  </div>
  <div class="cart-bar-total" id="cartBarTotal"></div>
</div>

<!-- Cart panel -->
<div class="cart-panel" id="cartPanel">
  <div class="cart-panel-header">
    <div class="cart-panel-title">購物車</div>
    <button class="cart-panel-close" onclick="toggleCartPanel()">&#10005;</button>
  </div>
  <div id="cartPanelItems"></div>
  <button class="clear-cart-btn" onclick="clearCart()">清空購物車</button>
</div>

<script>
var listEl = document.getElementById('list');
var menuEl = document.getElementById('menu');
var titleEl = document.getElementById('title');
var backEl = document.getElementById('back');
var modalOverlay = document.getElementById('modalOverlay');
var modalContent = document.getElementById('modalContent');
var cartBar = document.getElementById('cartBar');
var cartBarCount = document.getElementById('cartBarCount');
var cartBarTotal = document.getElementById('cartBarTotal');
var cartPanel = document.getElementById('cartPanel');
var cartPanelItems = document.getElementById('cartPanelItems');

var cart = { restaurantId: null, items: [] };
var cartPanelOpen = false;
var currentMenuData = null;

function esc(s) {
  var d = document.createElement('div');
  d.textContent = s;
  return d.innerHTML;
}

function formatPrice(p) {
  if (p === -1) return '未知';
  if (p === -2) return '時價';
  return 'NT$' + p;
}

/* ---- Cart Functions ---- */
function addToCart(id, name, price, tierLabel) {
  var cartId = tierLabel ? 'item_' + id + '_tier_' + tierLabel : 'item_' + id;
  for (var i = 0; i < cart.items.length; i++) {
    if (cart.items[i].id === cartId) {
      cart.items[i].qty += 1;
      saveCart();
      renderCart();
      return;
    }
  }
  var entry = { id: cartId, name: name, price: price, qty: 1 };
  if (tierLabel) entry.tierLabel = tierLabel;
  cart.items.push(entry);
  saveCart();
  renderCart();
}

function addComboToCart(comboId, name, price, choices) {
  var cartId = 'combo_' + comboId + '_' + Date.now();
  var totalAdj = 0;
  for (var i = 0; i < choices.length; i++) {
    totalAdj += choices[i].adjustment;
  }
  cart.items.push({
    id: cartId,
    name: name,
    price: price + totalAdj,
    qty: 1,
    comboId: comboId,
    comboChoices: choices
  });
  saveCart();
  renderCart();
}

function updateQty(index, delta) {
  cart.items[index].qty += delta;
  if (cart.items[index].qty <= 0) {
    cart.items.splice(index, 1);
  }
  saveCart();
  renderCart();
}

function clearCart() {
  cart.items = [];
  saveCart();
  renderCart();
  cartPanelOpen = false;
  cartPanel.classList.remove('visible');
}

function saveCart() {
  if (cart.restaurantId !== null) {
    try {
      localStorage.setItem('cart_' + cart.restaurantId, JSON.stringify(cart.items));
    } catch(e) {}
  }
}

function loadCart(rid) {
  cart.restaurantId = rid;
  cart.items = [];
  try {
    var saved = localStorage.getItem('cart_' + rid);
    if (saved) {
      cart.items = JSON.parse(saved) || [];
    }
  } catch(e) {
    cart.items = [];
  }
  cartPanelOpen = false;
  cartPanel.classList.remove('visible');
  renderCart();
}

function getCartTotal() {
  var total = 0;
  for (var i = 0; i < cart.items.length; i++) {
    total += cart.items[i].price * cart.items[i].qty;
  }
  return total;
}

function getCartCount() {
  var count = 0;
  for (var i = 0; i < cart.items.length; i++) {
    count += cart.items[i].qty;
  }
  return count;
}

function renderCart() {
  var count = getCartCount();
  var total = getCartTotal();

  if (count === 0) {
    cartBar.classList.remove('visible');
    cartPanel.classList.remove('visible');
    cartPanelOpen = false;
    return;
  }

  cartBar.classList.add('visible');
  cartBarCount.textContent = count + ' 項商品';
  cartBarTotal.textContent = 'NT$' + total;

  /* Render panel items */
  var html = '';
  for (var i = 0; i < cart.items.length; i++) {
    var item = cart.items[i];
    var subtotal = item.price * item.qty;
    var detail = '';
    if (item.tierLabel) {
      detail = item.tierLabel;
    } else if (item.comboChoices) {
      var parts = [];
      for (var j = 0; j < item.comboChoices.length; j++) {
        parts.push(item.comboChoices[j].option);
      }
      detail = parts.join(', ');
    }
    html += '<div class="cart-item">' +
      '<div class="cart-item-info">' +
      '<div class="cart-item-name">' + esc(item.name) + '</div>' +
      (detail ? '<div class="cart-item-detail">' + esc(detail) + '</div>' : '') +
      '</div>' +
      '<div class="cart-item-right">' +
      '<div class="qty-controls">' +
      '<button class="qty-btn" onclick="updateQty(' + i + ',-1)">&#8722;</button>' +
      '<div class="qty-val">' + item.qty + '</div>' +
      '<button class="qty-btn" onclick="updateQty(' + i + ',1)">+</button>' +
      '</div>' +
      '<div class="cart-item-subtotal">NT$' + subtotal + '</div>' +
      '</div>' +
      '</div>';
  }
  cartPanelItems.innerHTML = html;
}

function toggleCartPanel() {
  cartPanelOpen = !cartPanelOpen;
  if (cartPanelOpen) {
    cartPanel.classList.add('visible');
  } else {
    cartPanel.classList.remove('visible');
  }
}

/* ---- Modal ---- */
function openModal(html) {
  modalContent.innerHTML = html;
  modalOverlay.classList.add('active');
}

function closeModal() {
  modalOverlay.classList.remove('active');
  modalContent.innerHTML = '';
}

function closeModalOnBackdrop(e) {
  if (e.target === modalOverlay) {
    closeModal();
  }
}

/* ---- Tier Picker ---- */
function openTierPicker(itemId, itemName, tiers) {
  var html = '<div style="position:relative;">' +
    '<button class="modal-close" onclick="closeModal()">&#10005;</button>' +
    '<div class="modal-title">' + esc(itemName) + '</div>';
  for (var i = 0; i < tiers.length; i++) {
    var t = tiers[i];
    html += '<div class="tier-option" onclick="selectTier(' + itemId + ',\'' + esc(itemName).replace(/'/g, "\\'") + '\',' + t.price + ',\'' + esc(t.label).replace(/'/g, "\\'") + '\')">' +
      '<span class="tier-label">' + esc(t.label) + '</span>' +
      '<span class="tier-price">NT$' + t.price + '</span>' +
      '</div>';
  }
  html += '</div>';
  openModal(html);
}

function selectTier(itemId, itemName, price, tierLabel) {
  addToCart(itemId, itemName, price, tierLabel);
  closeModal();
}

/* ---- Combo Builder ---- */
var comboState = null;

function openComboBuilder(comboIdx) {
  var combos = currentMenuData.combos || [];
  var combo = combos[comboIdx];
  if (!combo) return;

  comboState = {
    combo: combo,
    currentGroup: 0,
    selections: []
  };
  for (var i = 0; i < combo.groups.length; i++) {
    comboState.selections.push([]);
  }
  renderComboBuilder();
}

function renderComboBuilder() {
  var c = comboState;
  var combo = c.combo;
  var gi = c.currentGroup;
  var group = combo.groups[gi];

  /* Step dots */
  var dots = '';
  for (var s = 0; s < combo.groups.length; s++) {
    var cls = 'combo-step';
    if (s < gi) cls += ' done';
    if (s === gi) cls += ' active';
    dots += '<div class="' + cls + '"></div>';
  }

  /* Calculate running total */
  var runTotal = combo.price;
  for (var si = 0; si < c.selections.length; si++) {
    for (var sj = 0; sj < c.selections[si].length; sj++) {
      runTotal += c.selections[si][sj].adjustment;
    }
  }

  /* Options */
  var optHtml = '';
  for (var oi = 0; oi < group.options.length; oi++) {
    var opt = group.options[oi];
    var isSelected = false;
    for (var k = 0; k < c.selections[gi].length; k++) {
      if (c.selections[gi][k].option === opt.name) { isSelected = true; break; }
    }
    var adjText = '';
    if (opt.adjustment > 0) adjText = '+NT$' + opt.adjustment;
    else if (opt.adjustment < 0) adjText = '-NT$' + Math.abs(opt.adjustment);

    optHtml += '<div class="combo-option' + (isSelected ? ' selected' : '') + '" onclick="toggleComboOption(' + gi + ',' + oi + ')">' +
      '<div class="combo-option-left">' +
      '<div class="check">' + (isSelected ? '&#10003;' : '') + '</div>' +
      '<span>' + esc(opt.name) + '</span>' +
      '</div>' +
      (adjText ? '<span class="combo-option-adj">' + adjText + '</span>' : '') +
      '</div>';
  }

  /* Hint about min/max */
  var hint = '';
  if (group.min === group.max) {
    hint = '請選擇 ' + group.min + ' 項';
  } else {
    hint = '請選擇 ' + group.min + ' ~ ' + group.max + ' 項';
  }

  /* Check if all groups satisfied */
  var allSatisfied = true;
  for (var ag = 0; ag < combo.groups.length; ag++) {
    if (c.selections[ag].length < combo.groups[ag].min) {
      allSatisfied = false;
      break;
    }
  }

  /* Navigation buttons */
  var navHtml = '';
  if (gi > 0) {
    navHtml += '<button style="padding:10px 20px;border:1px solid #ddd;border-radius:8px;background:#fff;cursor:pointer;font-size:14px;" onclick="comboGroupNav(-1)">上一步</button>';
  }
  if (gi < combo.groups.length - 1) {
    navHtml += '<button style="padding:10px 20px;border:1px solid #ddd;border-radius:8px;background:#fff;cursor:pointer;font-size:14px;margin-left:8px;" onclick="comboGroupNav(1)"' +
      (c.selections[gi].length < group.min ? ' disabled' : '') + '>下一步</button>';
  }

  var html = '<div style="position:relative;">' +
    '<button class="modal-close" onclick="closeModal()">&#10005;</button>' +
    '<div class="modal-title">' + esc(combo.name) + '</div>' +
    (combo.description ? '<div style="font-size:13px;color:#999;margin-bottom:12px;">' + esc(combo.description) + '</div>' : '') +
    '<div class="combo-steps">' + dots + '</div>' +
    '<div class="combo-group-title">' + esc(group.name) + '</div>' +
    '<div class="combo-group-hint">' + hint + ' (已選 ' + c.selections[gi].length + ')</div>' +
    optHtml +
    '<div class="combo-total">合計 NT$' + runTotal + '</div>' +
    '<div style="display:flex;justify-content:center;gap:8px;margin-bottom:8px;">' + navHtml + '</div>' +
    '<button class="combo-add-btn" onclick="confirmCombo()"' + (allSatisfied ? '' : ' disabled') + '>加入購物車</button>' +
    '</div>';

  openModal(html);
}

function toggleComboOption(groupIdx, optIdx) {
  var c = comboState;
  var group = c.combo.groups[groupIdx];
  var opt = group.options[optIdx];

  /* Check if already selected */
  var foundIdx = -1;
  for (var i = 0; i < c.selections[groupIdx].length; i++) {
    if (c.selections[groupIdx][i].option === opt.name) {
      foundIdx = i;
      break;
    }
  }

  if (foundIdx >= 0) {
    /* Deselect */
    c.selections[groupIdx].splice(foundIdx, 1);
  } else {
    if (group.min === 1 && group.max === 1) {
      /* Single select: replace */
      c.selections[groupIdx] = [{ group: group.name, option: opt.name, adjustment: opt.adjustment }];
      /* Auto-advance if there is a next group */
      if (groupIdx < c.combo.groups.length - 1) {
        c.currentGroup = groupIdx + 1;
        renderComboBuilder();
        return;
      }
    } else {
      /* Multi select */
      if (c.selections[groupIdx].length < group.max) {
        c.selections[groupIdx].push({ group: group.name, option: opt.name, adjustment: opt.adjustment });
      }
    }
  }

  renderComboBuilder();
}

function comboGroupNav(delta) {
  comboState.currentGroup += delta;
  renderComboBuilder();
}

function confirmCombo() {
  var c = comboState;
  var allChoices = [];
  for (var i = 0; i < c.selections.length; i++) {
    for (var j = 0; j < c.selections[i].length; j++) {
      allChoices.push(c.selections[i][j]);
    }
  }
  addComboToCart(c.combo.id, c.combo.name, c.combo.price, allChoices);
  closeModal();
  comboState = null;
}

/* ---- Restaurant list & menu ---- */
async function loadRestaurants() {
  var res = await fetch('/api/restaurants');
  var data = await res.json();
  if (!data || data.length === 0) {
    listEl.innerHTML = '<div class="empty">目前沒有菜單資料</div>';
    return;
  }
  listEl.innerHTML = data.map(function(r) {
    return '<div class="restaurant-item" onclick="showMenu(' + r.restaurant_id + ',\'' + esc(r.name).replace(/'/g, "\\'") + '\')">' +
    '<div class="name">' + esc(r.name) + '</div>' +
    (r.address ? '<div class="addr">' + esc(r.address) + '</div>' : '') +
    '</div>';
  }).join('');
}

async function showMenu(rid, name) {
  listEl.style.display = 'none';
  menuEl.style.display = 'block';
  menuEl.innerHTML = '<div class="loading">載入菜單...</div>';
  titleEl.textContent = name;
  backEl.style.display = 'block';

  loadCart(rid);

  var res = await fetch('/api/menu?restaurant_id=' + rid);
  var data = await res.json();
  currentMenuData = data;
  var cats = data.categories || [];
  var combos = data.combos || [];
  if (cats.length === 0 && combos.length === 0) {
    menuEl.innerHTML = '<div class="empty">此餐廳尚無菜單</div>';
    return;
  }

  var html = cats.filter(function(cat) { return cat.items && cat.items.length > 0; }).map(function(cat) {
    return '<div class="category">' +
    '<div class="category-header">' + esc(cat.name) + '</div>' +
    cat.items.map(function(it) {
      var hasTiers = it.price_tiers && it.price_tiers.length > 0;
      var canAdd = it.price >= 0;
      var priceHtml = '';
      var btnHtml = '';

      if (hasTiers) {
        /* Find lowest price among tiers */
        var lowest = it.price_tiers[0].price;
        for (var ti = 1; ti < it.price_tiers.length; ti++) {
          if (it.price_tiers[ti].price < lowest) lowest = it.price_tiers[ti].price;
        }
        priceHtml = '<div class="item-price">NT$' + lowest + '起</div>';
        btnHtml = '<button class="add-btn" onclick="event.stopPropagation();openTierPicker(' + it.id + ',\'' + esc(it.name).replace(/'/g, "\\'") + '\',' + JSON.stringify(it.price_tiers).replace(/"/g, '&quot;') + ')">+</button>';
      } else if (it.price === -1) {
        priceHtml = '<div class="item-price unknown">未知</div>';
      } else if (it.price === -2) {
        priceHtml = '<div class="item-price unknown">時價</div>';
      } else {
        priceHtml = '<div class="item-price">NT$' + it.price + '</div>';
        btnHtml = '<button class="add-btn" onclick="event.stopPropagation();addToCart(' + it.id + ',\'' + esc(it.name).replace(/'/g, "\\'") + '\',' + it.price + ')">+</button>';
      }

      return '<div class="menu-item">' +
        '<div class="item-info">' +
        '<div class="item-name">' + esc(it.name) + '</div>' +
        (it.description ? '<div class="item-desc">' + esc(it.description) + '</div>' : '') +
        (hasTiers ? '<div class="item-desc">' + it.price_tiers.map(function(t) { return esc(t.label) + ' NT$' + t.price; }).join(' / ') + '</div>' : '') +
        '</div>' +
        '<div class="item-right">' +
        priceHtml +
        btnHtml +
        '</div>' +
        '</div>';
    }).join('') +
    '</div>';
  }).join('');

  if (combos.length > 0) {
    html += '<div class="category"><div class="category-header">套餐</div>' +
      combos.map(function(c, ci) {
        return '<div class="menu-item">' +
          '<div class="item-info">' +
          '<div class="item-name">' + esc(c.name) + '</div>' +
          (c.description ? '<div class="item-desc">' + esc(c.description) + '</div>' : '') +
          '</div>' +
          '<div class="item-right">' +
          '<div class="item-price">NT$' + c.price + '</div>' +
          '<button class="add-btn" onclick="event.stopPropagation();openComboBuilder(' + ci + ')">+</button>' +
          '</div>' +
          '</div>';
      }).join('') + '</div>';
  }

  menuEl.innerHTML = html;
}

function showList() {
  listEl.style.display = 'block';
  menuEl.style.display = 'none';
  titleEl.textContent = '餐廳菜單';
  backEl.style.display = 'none';
  cartBar.classList.remove('visible');
  cartPanel.classList.remove('visible');
  cartPanelOpen = false;
  currentMenuData = null;
}

/* Fix tier picker to handle JSON tiers passed as string */
var _origOpenTierPicker = openTierPicker;
openTierPicker = function(itemId, itemName, tiers) {
  if (typeof tiers === 'string') {
    tiers = JSON.parse(tiers);
  }
  _origOpenTierPicker(itemId, itemName, tiers);
};

loadRestaurants();
</script>
</body>
</html>`

const debugHTML = `<!DOCTYPE html>
<html lang="zh-TW">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Debug - All Places</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, "Noto Sans TC", monospace; background: #1a1a2e; color: #e0e0e0; padding: 16px; }
h1 { color: #0ff; margin-bottom: 8px; font-size: 18px; }
.stats { color: #888; margin-bottom: 16px; font-size: 13px; }
.search { width: 100%; padding: 8px 12px; margin-bottom: 16px; background: #16213e; border: 1px solid #0f3460; color: #e0e0e0; border-radius: 4px; font-size: 14px; }
table { width: 100%; border-collapse: collapse; font-size: 12px; }
th { background: #16213e; color: #0ff; padding: 8px 6px; text-align: left; position: sticky; top: 0; z-index: 1; cursor: pointer; white-space: nowrap; }
th:hover { color: #fff; }
td { padding: 6px; border-bottom: 1px solid #16213e; max-width: 200px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
tr:hover { background: #16213e; }
a { color: #53a8ff; text-decoration: none; }
a:hover { text-decoration: underline; }
.tag { background: #0f3460; padding: 1px 5px; border-radius: 3px; margin: 1px; display: inline-block; font-size: 10px; }
.rating { color: #ffd700; }
.na { color: #555; }
</style>
</head>
<body>
<h1>All Places</h1>
<div class="stats" id="stats">Loading...</div>
<input class="search" id="search" placeholder="Search name, address, type..." oninput="filter()">
<table>
<thead><tr>
<th onclick="sort('id')">ID</th>
<th onclick="sort('name')">Name</th>
<th onclick="sort('address')">Address</th>
<th onclick="sort('rating')">Rating</th>
<th>Phone</th>
<th>Types</th>
<th>Links</th>
</tr></thead>
<tbody id="tbody"></tbody>
</table>
<script>
let allPlaces = [];
let sortKey = 'name';
let sortAsc = true;

async function load() {
  const res = await fetch('/api/places');
  allPlaces = await res.json();
  document.getElementById('stats').textContent = allPlaces.length + ' places total';
  render();
}

function render() {
  const q = document.getElementById('search').value.toLowerCase();
  let filtered = allPlaces;
  if (q) {
    filtered = allPlaces.filter(p =>
      p.name.toLowerCase().includes(q) ||
      (p.address||'').toLowerCase().includes(q) ||
      (p.place_types||[]).some(t => t.includes(q))
    );
  }
  filtered.sort((a,b) => {
    let va = a[sortKey]||'', vb = b[sortKey]||'';
    if (typeof va === 'number' && typeof vb === 'number') return sortAsc ? va-vb : vb-va;
    va = String(va); vb = String(vb);
    return sortAsc ? va.localeCompare(vb) : vb.localeCompare(va);
  });
  const tbody = document.getElementById('tbody');
  tbody.innerHTML = filtered.map(p => {
    const types = (p.place_types||[]).map(t => '<span class="tag">'+esc(t)+'</span>').join(' ');
    const rating = p.rating ? '<span class="rating">'+p.rating+'</span> ('+p.total_ratings+')' : '<span class="na">-</span>';
    const links = [];
    if (p.google_maps_url) links.push('<a href="'+esc(p.google_maps_url)+'" target="_blank">Maps</a>');
    if (p.website) links.push('<a href="'+esc(p.website)+'" target="_blank">Web</a>');
    return '<tr>' +
      '<td>'+p.id+'</td>' +
      '<td title="'+esc(p.google_place_id)+'">'+esc(p.name)+'</td>' +
      '<td title="'+esc(p.address||'')+'">'+esc(p.address||'')+'</td>' +
      '<td>'+rating+'</td>' +
      '<td>'+esc(p.phone_number||'')+'</td>' +
      '<td>'+types+'</td>' +
      '<td>'+links.join(' ')+'</td>' +
      '</tr>';
  }).join('');
  document.getElementById('stats').textContent = filtered.length + ' / ' + allPlaces.length + ' places';
}

function filter() { render(); }
function sort(key) {
  if (sortKey === key) sortAsc = !sortAsc;
  else { sortKey = key; sortAsc = true; }
  render();
}
function esc(s) {
  const d = document.createElement('div');
  d.textContent = s;
  return d.innerHTML;
}
load();
</script>
</body>
</html>`
