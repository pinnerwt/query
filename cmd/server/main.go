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

	type menuItem struct {
		Name        string `json:"name"`
		Price       int32  `json:"price"`
		Description string `json:"description,omitempty"`
	}

	type category struct {
		Name  string     `json:"name"`
		Items []menuItem `json:"items"`
	}

	catIdx := make(map[int64]int)
	var out []category
	for _, c := range categories {
		catIdx[c.ID] = len(out)
		out = append(out, category{Name: c.Name})
	}

	for _, it := range items {
		mi := menuItem{
			Name:        it.Name,
			Price:       it.Price,
			Description: it.Description.String,
		}
		if it.CategoryID.Valid {
			if idx, ok := catIdx[it.CategoryID.Int64]; ok {
				out[idx].Items = append(out[idx].Items, mi)
				continue
			}
		}
		// Uncategorized
		if len(out) == 0 || out[len(out)-1].Name != "其他" {
			out = append(out, category{Name: "其他"})
		}
		out[len(out)-1].Items = append(out[len(out)-1].Items, mi)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
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
.container { max-width: 480px; margin: 0 auto; min-height: 100vh; background: #fff; }
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
.menu-item { display: flex; justify-content: space-between; align-items: flex-start; padding: 14px 20px; border-bottom: 1px solid #f0f0f0; }
.menu-item:last-child { border-bottom: none; }
.item-info { flex: 1; }
.item-name { font-size: 16px; font-weight: 500; }
.item-desc { font-size: 12px; color: #999; margin-top: 2px; }
.item-price { font-size: 16px; font-weight: 700; color: #e74c3c; white-space: nowrap; margin-left: 16px; }
.item-price::before { content: "NT$"; font-size: 12px; font-weight: 400; }

.loading { text-align: center; padding: 40px; color: #999; }
.empty { text-align: center; padding: 60px 20px; color: #999; }
</style>
</head>
<body>
<div class="container">
  <header>
    <div class="back" id="back" onclick="showList()">← 返回餐廳列表</div>
    <h1 id="title">餐廳菜單</h1>
  </header>
  <div id="list" class="restaurant-list">
    <div class="loading">載入中...</div>
  </div>
  <div id="menu" class="menu-view"></div>
</div>
<script>
const listEl = document.getElementById('list');
const menuEl = document.getElementById('menu');
const titleEl = document.getElementById('title');
const backEl = document.getElementById('back');

async function loadRestaurants() {
  const res = await fetch('/api/restaurants');
  const data = await res.json();
  if (!data || data.length === 0) {
    listEl.innerHTML = '<div class="empty">目前沒有菜單資料</div>';
    return;
  }
  listEl.innerHTML = data.map(r =>
    '<div class="restaurant-item" onclick="showMenu(' + r.restaurant_id + ',\'' + esc(r.name) + '\')">' +
    '<div class="name">' + esc(r.name) + '</div>' +
    (r.address ? '<div class="addr">' + esc(r.address) + '</div>' : '') +
    '</div>'
  ).join('');
}

async function showMenu(rid, name) {
  listEl.style.display = 'none';
  menuEl.style.display = 'block';
  menuEl.innerHTML = '<div class="loading">載入菜單...</div>';
  titleEl.textContent = name;
  backEl.style.display = 'block';

  const res = await fetch('/api/menu?restaurant_id=' + rid);
  const data = await res.json();
  if (!data || data.length === 0) {
    menuEl.innerHTML = '<div class="empty">此餐廳尚無菜單</div>';
    return;
  }
  menuEl.innerHTML = data.filter(cat => cat.items && cat.items.length > 0).map(cat =>
    '<div class="category">' +
    '<div class="category-header">' + esc(cat.name) + '</div>' +
    cat.items.map(it =>
      '<div class="menu-item">' +
      '<div class="item-info">' +
      '<div class="item-name">' + esc(it.name) + '</div>' +
      (it.description ? '<div class="item-desc">' + esc(it.description) + '</div>' : '') +
      '</div>' +
      (it.price > 0 ? '<div class="item-price">' + it.price + '</div>' : '') +
      '</div>'
    ).join('') +
    '</div>'
  ).join('');
}

function showList() {
  listEl.style.display = 'block';
  menuEl.style.display = 'none';
  titleEl.textContent = '餐廳菜單';
  backEl.style.display = 'none';
}

function esc(s) {
  const d = document.createElement('div');
  d.textContent = s;
  return d.innerHTML;
}

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
