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
	http.HandleFunc("/api/restaurants", s.handleRestaurants)
	http.HandleFunc("/api/menu", s.handleMenu)

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

	catMap := make(map[int64]*category)
	var out []category
	for _, c := range categories {
		cat := category{Name: c.Name}
		out = append(out, cat)
		catMap[c.ID] = &out[len(out)-1]
	}

	for _, it := range items {
		mi := menuItem{
			Name:        it.Name,
			Price:       it.Price,
			Description: it.Description.String,
		}
		if it.CategoryID.Valid {
			if cat, ok := catMap[it.CategoryID.Int64]; ok {
				cat.Items = append(cat.Items, mi)
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

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, indexHTML)
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
