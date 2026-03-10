package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	qrcode "github.com/skip2/go-qrcode"

	rootpkg "github.com/pinnertw/query"
	"github.com/pinnertw/query/internal/auth"
	db "github.com/pinnertw/query/internal/db/generated"
	"github.com/pinnertw/query/internal/ocr"
	"github.com/pinnertw/query/internal/ratelimit"
	"github.com/pinnertw/query/internal/slug"
	"github.com/pinnertw/query/internal/storage"
)

func main() {
	dbURL := flag.String("db", "", "PostgreSQL connection string (or set DATABASE_URL env var)")
	addr := flag.String("addr", ":8080", "Listen address")
	jwtSecret := flag.String("jwt-secret", "", "JWT signing secret (or set JWT_SECRET env var)")
	baseURL := flag.String("base-url", "http://localhost:8080", "Public base URL for QR codes")
	photosDir := flag.String("photos-dir", "menu_photos", "Directory for uploaded menu photos")
	ollamaURL := flag.String("ollama", "http://127.0.0.1:11434", "Ollama API base URL")
	ocrModel := flag.String("ocr-model", "glm-ocr-gpu", "OCR model name")
	normModel := flag.String("norm-model", "qwen3.5:9b", "Normalization model name")
	normURL := flag.String("norm-url", "", "OpenAI-compatible API for normalization")
	flag.Parse()

	connStr := *dbURL
	if connStr == "" {
		connStr = os.Getenv("DATABASE_URL")
	}
	if connStr == "" {
		log.Fatal("--db or DATABASE_URL is required")
	}

	secret := *jwtSecret
	if secret == "" {
		secret = os.Getenv("JWT_SECRET")
	}
	if secret == "" {
		// Generate random secret for development
		b := make([]byte, 32)
		rand.Read(b)
		secret = hex.EncodeToString(b)
		log.Printf("No JWT_SECRET set, using random secret (tokens won't survive restart)")
	}
	secretBytes := []byte(secret)

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer pool.Close()

	q := db.New(pool)
	s := &server{
		q:         q,
		secret:    secretBytes,
		baseURL:   *baseURL,
		photosDir: *photosDir,
		ollamaURL: *ollamaURL,
		ocrModel:  *ocrModel,
		normModel: *normModel,
		normURL:   *normURL,
	}

	mux := http.NewServeMux()

	// Rate limiter for registration
	regLimiter := ratelimit.NewSlidingWindow(10, 1*time.Hour)

	// Auth endpoints
	mux.Handle("POST /api/auth/register", regLimiter.Middleware(http.HandlerFunc(s.handleRegister)))
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.Handle("GET /api/auth/me", auth.Middleware(secretBytes, http.HandlerFunc(s.handleMe)))

	// Restaurant CRUD (auth required)
	mux.Handle("POST /api/restaurants", auth.Middleware(secretBytes, http.HandlerFunc(s.handleCreateRestaurant)))
	mux.Handle("GET /api/restaurants/mine", auth.Middleware(secretBytes, http.HandlerFunc(s.handleListMyRestaurants)))
	mux.Handle("GET /api/restaurants/{id}", auth.Middleware(secretBytes, http.HandlerFunc(s.handleGetRestaurant)))
	mux.Handle("PUT /api/restaurants/{id}", auth.Middleware(secretBytes, http.HandlerFunc(s.handleUpdateRestaurant)))
	mux.Handle("DELETE /api/restaurants/{id}", auth.Middleware(secretBytes, http.HandlerFunc(s.handleDeleteRestaurant)))
	mux.Handle("PUT /api/restaurants/{id}/hours", auth.Middleware(secretBytes, http.HandlerFunc(s.handleSetHours)))
	mux.Handle("PUT /api/restaurants/{id}/publish", auth.Middleware(secretBytes, http.HandlerFunc(s.handlePublish)))

	// Menu (auth required)
	mux.Handle("GET /api/restaurants/{id}/menu", auth.Middleware(secretBytes, http.HandlerFunc(s.handleGetMenu)))
	mux.Handle("PUT /api/restaurants/{id}/menu", auth.Middleware(secretBytes, http.HandlerFunc(s.handleReplaceMenu)))

	// Photo upload + OCR (auth required)
	mux.Handle("POST /api/restaurants/{id}/menu-photos", auth.Middleware(secretBytes, http.HandlerFunc(s.handleUploadPhotos)))
	mux.Handle("POST /api/restaurants/{id}/ocr", auth.Middleware(secretBytes, http.HandlerFunc(s.handleOCR)))

	// QR code (auth required)
	mux.Handle("GET /api/restaurants/{id}/qr", auth.Middleware(secretBytes, http.HandlerFunc(s.handleQR)))

	// Orders - owner endpoints (auth required)
	mux.Handle("GET /api/restaurants/{id}/orders", auth.Middleware(secretBytes, http.HandlerFunc(s.handleListOrders)))
	mux.Handle("PUT /api/restaurants/{id}/orders/{orderId}/status", auth.Middleware(secretBytes, http.HandlerFunc(s.handleUpdateOrderStatus)))

	// Public endpoints (no auth)
	mux.HandleFunc("GET /api/public/menu/{slug}", s.handlePublicMenu)
	mux.HandleFunc("POST /api/public/orders/{slug}", s.handlePublicCreateOrder)
	mux.HandleFunc("GET /api/public/orders/{slug}/{orderId}", s.handlePublicGetOrder)

	// Legacy endpoints
	mux.HandleFunc("GET /api/restaurants", s.handleLegacyRestaurants)
	mux.HandleFunc("GET /api/menu", s.handleLegacyMenu)
	mux.HandleFunc("GET /api/places", s.handlePlaces)

	// Public menu page (server-rendered)
	mux.HandleFunc("GET /r/{slug}", s.handlePublicMenuPage)

	// Embedded Preact SPA at /app/
	distFS, err := fs.Sub(rootpkg.FrontendDist, "frontend/dist")
	if err != nil {
		log.Fatalf("Failed to create sub FS: %v", err)
	}
	appHandler := http.StripPrefix("/app/", http.FileServer(http.FS(distFS)))
	mux.HandleFunc("/app/", func(w http.ResponseWriter, r *http.Request) {
		// Try to serve the actual file; if not found, serve index.html for SPA routing
		path := strings.TrimPrefix(r.URL.Path, "/app/")
		if path == "" {
			path = "index.html"
		}
		if f, err := distFS.Open(path); err == nil {
			f.Close()
			appHandler.ServeHTTP(w, r)
			return
		}
		// SPA fallback: serve index.html
		r.URL.Path = "/app/index.html"
		appHandler.ServeHTTP(w, r)
	})

	// Frontend / index
	mux.HandleFunc("GET /", s.handleIndex)
	mux.HandleFunc("GET /debug", s.handleDebug)

	fmt.Printf("Server listening on %s\n", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

type server struct {
	q         *db.Queries
	secret    []byte
	baseURL   string
	photosDir string
	ollamaURL string
	ocrModel  string
	normModel string
	normURL   string
}

// --- Auth handlers ---

func (s *server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", 400)
		return
	}
	if req.Email == "" || req.Password == "" {
		jsonError(w, "email and password required", 400)
		return
	}
	if len(req.Password) < 8 {
		jsonError(w, "password must be at least 8 characters", 400)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		jsonError(w, "internal error", 500)
		return
	}

	owner, err := s.q.CreateOwner(r.Context(), db.CreateOwnerParams{
		Email:        req.Email,
		PasswordHash: hash,
		Name:         req.Name,
	})
	if err != nil {
		if strings.Contains(err.Error(), "unique") {
			jsonError(w, "email already registered", 409)
			return
		}
		jsonError(w, "internal error", 500)
		return
	}

	token, err := auth.GenerateToken(owner.ID, s.secret, 24*time.Hour)
	if err != nil {
		jsonError(w, "internal error", 500)
		return
	}

	jsonResp(w, 201, map[string]interface{}{
		"token": token,
		"owner": map[string]interface{}{
			"id":    owner.ID,
			"email": owner.Email,
			"name":  owner.Name,
		},
	})
}

func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", 400)
		return
	}

	owner, err := s.q.GetOwnerByEmail(r.Context(), req.Email)
	if err != nil {
		jsonError(w, "invalid credentials", 401)
		return
	}

	if err := auth.CheckPassword(owner.PasswordHash, req.Password); err != nil {
		jsonError(w, "invalid credentials", 401)
		return
	}

	token, err := auth.GenerateToken(owner.ID, s.secret, 24*time.Hour)
	if err != nil {
		jsonError(w, "internal error", 500)
		return
	}

	jsonResp(w, 200, map[string]interface{}{
		"token": token,
		"owner": map[string]interface{}{
			"id":    owner.ID,
			"email": owner.Email,
			"name":  owner.Name,
		},
	})
}

func (s *server) handleMe(w http.ResponseWriter, r *http.Request) {
	ownerID, _ := auth.OwnerIDFromContext(r.Context())
	owner, err := s.q.GetOwnerByID(r.Context(), ownerID)
	if err != nil {
		jsonError(w, "owner not found", 404)
		return
	}
	jsonResp(w, 200, map[string]interface{}{
		"id":    owner.ID,
		"email": owner.Email,
		"name":  owner.Name,
	})
}

// --- Restaurant CRUD ---

func (s *server) handleCreateRestaurant(w http.ResponseWriter, r *http.Request) {
	ownerID, _ := auth.OwnerIDFromContext(r.Context())
	var req struct {
		Name    string  `json:"name"`
		Address string  `json:"address"`
		Phone   string  `json:"phone_number"`
		Website string  `json:"website"`
		DineIn  bool    `json:"dine_in"`
		Takeout bool    `json:"takeout"`
		Lat     float64 `json:"latitude"`
		Lng     float64 `json:"longitude"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", 400)
		return
	}
	if req.Name == "" {
		jsonError(w, "name required", 400)
		return
	}

	rest, err := s.q.CreateRestaurant(r.Context(), db.CreateRestaurantParams{
		OwnerID:     ownerID,
		Name:        req.Name,
		Slug:        slug.Generate(req.Name),
		Address:     pgtype.Text{String: req.Address, Valid: req.Address != ""},
		PhoneNumber: pgtype.Text{String: req.Phone, Valid: req.Phone != ""},
		Website:     pgtype.Text{String: req.Website, Valid: req.Website != ""},
		DineIn:      req.DineIn,
		Takeout:     req.Takeout,
	})
	if err != nil {
		jsonError(w, "failed to create restaurant", 500)
		return
	}

	jsonResp(w, 201, restaurantJSON(rest))
}

func (s *server) handleListMyRestaurants(w http.ResponseWriter, r *http.Request) {
	ownerID, _ := auth.OwnerIDFromContext(r.Context())
	list, err := s.q.ListRestaurantsByOwner(r.Context(), ownerID)
	if err != nil {
		jsonError(w, "internal error", 500)
		return
	}
	out := make([]map[string]interface{}, len(list))
	for i, rest := range list {
		out[i] = restaurantJSON(rest)
	}
	jsonResp(w, 200, out)
}

func (s *server) handleGetRestaurant(w http.ResponseWriter, r *http.Request) {
	ownerID, _ := auth.OwnerIDFromContext(r.Context())
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, "invalid id", 400)
		return
	}
	rest, err := s.q.GetRestaurantByID(r.Context(), id)
	if err != nil {
		jsonError(w, "not found", 404)
		return
	}
	if rest.OwnerID != ownerID {
		jsonError(w, "forbidden", 403)
		return
	}
	jsonResp(w, 200, restaurantJSON(rest))
}

func (s *server) handleUpdateRestaurant(w http.ResponseWriter, r *http.Request) {
	ownerID, _ := auth.OwnerIDFromContext(r.Context())
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, "invalid id", 400)
		return
	}
	var req struct {
		Name    string `json:"name"`
		Address string `json:"address"`
		Phone   string `json:"phone_number"`
		Website string `json:"website"`
		DineIn  bool   `json:"dine_in"`
		Takeout bool   `json:"takeout"`
		Delivery bool  `json:"delivery"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", 400)
		return
	}

	updated, err := s.q.UpdateRestaurant(r.Context(), db.UpdateRestaurantParams{
		ID:          id,
		OwnerID:     ownerID,
		Name:        req.Name,
		Address:     pgtype.Text{String: req.Address, Valid: req.Address != ""},
		PhoneNumber: pgtype.Text{String: req.Phone, Valid: req.Phone != ""},
		Website:     pgtype.Text{String: req.Website, Valid: req.Website != ""},
		DineIn:      req.DineIn,
		Takeout:     req.Takeout,
		Delivery:    req.Delivery,
	})
	if err != nil {
		jsonError(w, "not found or forbidden", 404)
		return
	}
	jsonResp(w, 200, restaurantJSON(updated))
}

func (s *server) handleDeleteRestaurant(w http.ResponseWriter, r *http.Request) {
	ownerID, _ := auth.OwnerIDFromContext(r.Context())
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, "invalid id", 400)
		return
	}
	if err := s.q.DeleteRestaurant(r.Context(), db.DeleteRestaurantParams{ID: id, OwnerID: ownerID}); err != nil {
		jsonError(w, "not found or forbidden", 404)
		return
	}
	w.WriteHeader(204)
}

func (s *server) handleSetHours(w http.ResponseWriter, r *http.Request) {
	ownerID, _ := auth.OwnerIDFromContext(r.Context())
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, "invalid id", 400)
		return
	}

	// Verify ownership
	rest, err := s.q.GetRestaurantByID(r.Context(), id)
	if err != nil || rest.OwnerID != ownerID {
		jsonError(w, "not found or forbidden", 404)
		return
	}

	var req struct {
		Hours []struct {
			DayOfWeek int    `json:"day_of_week"`
			OpenTime  string `json:"open_time"`
			CloseTime string `json:"close_time"`
		} `json:"hours"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", 400)
		return
	}

	// Replace: delete + re-insert
	_ = s.q.DeleteRestaurantHours(r.Context(), id)
	for _, h := range req.Hours {
		openT, _ := time.Parse("15:04", h.OpenTime)
		closeT, _ := time.Parse("15:04", h.CloseTime)
		_ = s.q.InsertRestaurantHour(r.Context(), db.InsertRestaurantHourParams{
			RestaurantID: id,
			DayOfWeek:    int16(h.DayOfWeek),
			OpenTime:     pgtype.Time{Microseconds: int64(openT.Hour()*3600+openT.Minute()*60) * 1e6, Valid: true},
			CloseTime:    pgtype.Time{Microseconds: int64(closeT.Hour()*3600+closeT.Minute()*60) * 1e6, Valid: true},
		})
	}

	jsonResp(w, 200, map[string]string{"status": "ok"})
}

func (s *server) handlePublish(w http.ResponseWriter, r *http.Request) {
	ownerID, _ := auth.OwnerIDFromContext(r.Context())
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, "invalid id", 400)
		return
	}
	var req struct {
		Published bool `json:"is_published"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", 400)
		return
	}
	updated, err := s.q.UpdateRestaurantPublished(r.Context(), db.UpdateRestaurantPublishedParams{
		ID: id, OwnerID: ownerID, IsPublished: req.Published,
	})
	if err != nil {
		jsonError(w, "not found or forbidden", 404)
		return
	}
	jsonResp(w, 200, restaurantJSON(updated))
}

// --- Menu ---

func (s *server) handleGetMenu(w http.ResponseWriter, r *http.Request) {
	ownerID, _ := auth.OwnerIDFromContext(r.Context())
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, "invalid id", 400)
		return
	}
	rest, err := s.q.GetRestaurantByID(r.Context(), id)
	if err != nil || rest.OwnerID != ownerID {
		jsonError(w, "not found or forbidden", 404)
		return
	}
	menu, err := s.buildMenuJSON(r.Context(), id)
	if err != nil {
		jsonError(w, "internal error", 500)
		return
	}
	jsonResp(w, 200, menu)
}

func (s *server) handleReplaceMenu(w http.ResponseWriter, r *http.Request) {
	ownerID, _ := auth.OwnerIDFromContext(r.Context())
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, "invalid id", 400)
		return
	}
	rest, err := s.q.GetRestaurantByID(r.Context(), id)
	if err != nil || rest.OwnerID != ownerID {
		jsonError(w, "not found or forbidden", 404)
		return
	}

	var menu ocr.MenuData
	if err := json.NewDecoder(r.Body).Decode(&menu); err != nil {
		jsonError(w, "invalid menu data", 400)
		return
	}

	if err := ocr.InsertMenu(r.Context(), s.q, id, &menu); err != nil {
		jsonError(w, "failed to save menu: "+err.Error(), 500)
		return
	}

	jsonResp(w, 200, map[string]string{"status": "ok"})
}

// --- Photo upload + OCR ---

func (s *server) handleUploadPhotos(w http.ResponseWriter, r *http.Request) {
	ownerID, _ := auth.OwnerIDFromContext(r.Context())
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, "invalid id", 400)
		return
	}
	rest, err := s.q.GetRestaurantByID(r.Context(), id)
	if err != nil || rest.OwnerID != ownerID {
		jsonError(w, "not found or forbidden", 404)
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB max
		jsonError(w, "invalid multipart form", 400)
		return
	}

	var uploads []map[string]interface{}
	for _, fh := range r.MultipartForm.File["photos"] {
		f, err := fh.Open()
		if err != nil {
			continue
		}
		path, err := storage.SaveUpload(s.photosDir, id, fh.Filename, f)
		f.Close()
		if err != nil {
			continue
		}

		upload, err := s.q.CreateMenuPhotoUpload(r.Context(), db.CreateMenuPhotoUploadParams{
			RestaurantID: id,
			FilePath:     path,
			FileName:     fh.Filename,
		})
		if err != nil {
			continue
		}
		uploads = append(uploads, map[string]interface{}{
			"id":        upload.ID,
			"file_name": upload.FileName,
			"status":    upload.OcrStatus,
		})
	}

	jsonResp(w, 201, map[string]interface{}{"uploads": uploads})
}

func (s *server) handleOCR(w http.ResponseWriter, r *http.Request) {
	ownerID, _ := auth.OwnerIDFromContext(r.Context())
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, "invalid id", 400)
		return
	}
	rest, err := s.q.GetRestaurantByID(r.Context(), id)
	if err != nil || rest.OwnerID != ownerID {
		jsonError(w, "not found or forbidden", 404)
		return
	}

	// Find photos for this restaurant
	dir := filepath.Join(s.photosDir, fmt.Sprintf("%d", id))
	files, err := ocr.FindImages(dir)
	if err != nil || len(files) == 0 {
		jsonError(w, "no photos found", 404)
		return
	}

	files = ocr.DeduplicateImages(files)

	// OCR all images sequentially (synchronous, ~30-60s total)
	var texts []string
	for _, f := range files {
		ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
		text, err := ocr.OcrImage(s.ollamaURL, s.ocrModel, f, 800, ctx)
		cancel()
		if err != nil {
			continue
		}
		texts = append(texts, text)
	}

	if len(texts) == 0 {
		jsonError(w, "OCR failed on all photos", 500)
		return
	}

	// Combine and normalize
	var combined strings.Builder
	for i, text := range texts {
		combined.WriteString(fmt.Sprintf("--- Photo %d ---\n", i+1))
		combined.WriteString(text)
		combined.WriteString("\n\n")
	}

	useOpenAI := s.normURL != ""
	normBase := s.ollamaURL
	if useOpenAI {
		normBase = s.normURL
	}

	menu, err := ocr.NormalizeMenu(normBase, s.normModel, combined.String(), useOpenAI)
	if err != nil {
		jsonError(w, "normalization failed: "+err.Error(), 500)
		return
	}

	// Save to DB
	if err := ocr.InsertMenu(r.Context(), s.q, id, menu); err != nil {
		jsonError(w, "failed to save menu: "+err.Error(), 500)
		return
	}

	jsonResp(w, 200, menu)
}

// --- QR Code ---

func (s *server) handleQR(w http.ResponseWriter, r *http.Request) {
	ownerID, _ := auth.OwnerIDFromContext(r.Context())
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, "invalid id", 400)
		return
	}
	rest, err := s.q.GetRestaurantByID(r.Context(), id)
	if err != nil || rest.OwnerID != ownerID {
		jsonError(w, "not found or forbidden", 404)
		return
	}

	url := fmt.Sprintf("%s/r/%s", s.baseURL, rest.Slug)
	png, err := qrcode.Encode(url, qrcode.Medium, 512)
	if err != nil {
		jsonError(w, "QR generation failed", 500)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-qr.png"`, rest.Slug))
	w.Write(png)
}

// --- Orders (owner) ---

func (s *server) handleListOrders(w http.ResponseWriter, r *http.Request) {
	ownerID, _ := auth.OwnerIDFromContext(r.Context())
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, "invalid id", 400)
		return
	}
	rest, err := s.q.GetRestaurantByID(r.Context(), id)
	if err != nil || rest.OwnerID != ownerID {
		jsonError(w, "not found or forbidden", 404)
		return
	}

	status := r.URL.Query().Get("status")
	orders, err := s.q.ListOrdersByRestaurant(r.Context(), db.ListOrdersByRestaurantParams{
		RestaurantID: id,
		Status:       status,
	})
	if err != nil {
		jsonError(w, "internal error", 500)
		return
	}

	var out []map[string]interface{}
	for _, o := range orders {
		items, _ := s.q.ListOrderItemsByOrder(r.Context(), o.ID)
		out = append(out, orderJSON(o, items))
	}
	jsonResp(w, 200, out)
}

func (s *server) handleUpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	ownerID, _ := auth.OwnerIDFromContext(r.Context())
	id, err := parseID(r, "id")
	if err != nil {
		jsonError(w, "invalid id", 400)
		return
	}
	rest, err := s.q.GetRestaurantByID(r.Context(), id)
	if err != nil || rest.OwnerID != ownerID {
		jsonError(w, "not found or forbidden", 404)
		return
	}

	orderIDStr := r.PathValue("orderId")
	orderID, err := strconv.ParseInt(orderIDStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid order id", 400)
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", 400)
		return
	}

	order, err := s.q.GetOrderByID(r.Context(), orderID)
	if err != nil || order.RestaurantID != id {
		jsonError(w, "order not found", 404)
		return
	}

	updated, err := s.q.UpdateOrderStatus(r.Context(), db.UpdateOrderStatusParams{
		ID:     orderID,
		Status: req.Status,
	})
	if err != nil {
		jsonError(w, "invalid status", 400)
		return
	}

	items, _ := s.q.ListOrderItemsByOrder(r.Context(), orderID)
	jsonResp(w, 200, orderJSON(updated, items))
}

// --- Public endpoints ---

func (s *server) handlePublicMenu(w http.ResponseWriter, r *http.Request) {
	slugStr := r.PathValue("slug")
	rest, err := s.q.GetPublishedRestaurantBySlug(r.Context(), slugStr)
	if err != nil {
		jsonError(w, "not found", 404)
		return
	}

	menu, err := s.buildMenuJSON(r.Context(), rest.ID)
	if err != nil {
		jsonError(w, "internal error", 500)
		return
	}

	result := map[string]interface{}{
		"restaurant": map[string]interface{}{
			"name":    rest.Name,
			"slug":    rest.Slug,
			"address": rest.Address.String,
		},
		"menu": menu,
	}
	jsonResp(w, 200, result)
}

func (s *server) handlePublicCreateOrder(w http.ResponseWriter, r *http.Request) {
	slugStr := r.PathValue("slug")
	rest, err := s.q.GetPublishedRestaurantBySlug(r.Context(), slugStr)
	if err != nil {
		jsonError(w, "restaurant not found", 404)
		return
	}

	var req struct {
		TableLabel string `json:"table_label"`
		Items      []struct {
			MenuItemID int64  `json:"menu_item_id"`
			ItemName   string `json:"item_name"`
			Quantity   int    `json:"quantity"`
			UnitPrice  int    `json:"unit_price"`
			Notes      string `json:"notes"`
		} `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", 400)
		return
	}
	if len(req.Items) == 0 {
		jsonError(w, "at least one item required", 400)
		return
	}

	// Compute total server-side
	var total int
	for _, item := range req.Items {
		total += item.UnitPrice * item.Quantity
	}

	order, err := s.q.CreateOrder(r.Context(), db.CreateOrderParams{
		RestaurantID: rest.ID,
		TableLabel:   pgtype.Text{String: req.TableLabel, Valid: req.TableLabel != ""},
		TotalAmount:  int32(total),
	})
	if err != nil {
		jsonError(w, "failed to create order", 500)
		return
	}

	for _, item := range req.Items {
		_, _ = s.q.CreateOrderItem(r.Context(), db.CreateOrderItemParams{
			OrderID:    order.ID,
			MenuItemID: pgtype.Int8{Int64: item.MenuItemID, Valid: item.MenuItemID > 0},
			ItemName:   item.ItemName,
			Quantity:   int32(item.Quantity),
			UnitPrice:  int32(item.UnitPrice),
			Notes:      pgtype.Text{String: item.Notes, Valid: item.Notes != ""},
		})
	}

	items, _ := s.q.ListOrderItemsByOrder(r.Context(), order.ID)
	jsonResp(w, 201, orderJSON(order, items))
}

func (s *server) handlePublicGetOrder(w http.ResponseWriter, r *http.Request) {
	orderIDStr := r.PathValue("orderId")
	orderID, err := strconv.ParseInt(orderIDStr, 10, 64)
	if err != nil {
		jsonError(w, "invalid order id", 400)
		return
	}

	order, err := s.q.GetOrderByID(r.Context(), orderID)
	if err != nil {
		jsonError(w, "order not found", 404)
		return
	}

	items, _ := s.q.ListOrderItemsByOrder(r.Context(), orderID)
	jsonResp(w, 200, orderJSON(order, items))
}

// --- Server-rendered public menu page ---

func (s *server) handlePublicMenuPage(w http.ResponseWriter, r *http.Request) {
	slugStr := r.PathValue("slug")
	rest, err := s.q.GetPublishedRestaurantBySlug(r.Context(), slugStr)
	if err != nil {
		http.Error(w, "Restaurant not found", 404)
		return
	}

	menu, _ := s.buildMenuJSON(r.Context(), rest.ID)

	menuJSON, _ := json.Marshal(menu)
	restJSON, _ := json.Marshal(map[string]interface{}{
		"id":      rest.ID,
		"name":    rest.Name,
		"slug":    rest.Slug,
		"address": rest.Address.String,
	})

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, publicMenuHTML, rest.Name, rest.Name, string(restJSON), string(menuJSON), rest.Slug)
}

// --- Legacy handlers (backward compatible) ---

func (s *server) handleLegacyRestaurants(w http.ResponseWriter, r *http.Request) {
	restaurants, err := s.q.ListRestaurantsWithMenus(r.Context())
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	type item struct {
		RestaurantID int64  `json:"restaurant_id"`
		Name         string `json:"name"`
		Address      string `json:"address"`
		Slug         string `json:"slug"`
	}

	var out []item
	for _, r := range restaurants {
		out = append(out, item{
			RestaurantID: r.RestaurantID,
			Name:         r.Name,
			Address:      r.Address.String,
			Slug:         r.Slug,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (s *server) handleLegacyMenu(w http.ResponseWriter, r *http.Request) {
	ridStr := r.URL.Query().Get("restaurant_id")
	rid, err := strconv.ParseInt(ridStr, 10, 64)
	if err != nil {
		http.Error(w, "restaurant_id required", 400)
		return
	}

	menu, err := s.buildMenuJSON(r.Context(), rid)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(menu)
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
		out = append(out, o)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, indexHTML)
}

func (s *server) handleDebug(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, debugHTML)
}

// --- Helpers ---

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

	tierMap := make(map[int64][]map[string]interface{})
	for _, pt := range priceTiers {
		tierMap[pt.MenuItemID] = append(tierMap[pt.MenuItemID], map[string]interface{}{
			"label":    pt.Label,
			"quantity": pt.Quantity,
			"price":    pt.Price,
		})
	}

	catIdx := make(map[int64]int)
	type categoryOut struct {
		Name  string                   `json:"name"`
		Items []map[string]interface{} `json:"items"`
	}
	var cats []categoryOut
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

	comboMeals, err := s.q.ListComboMealsByRestaurant(ctx, restaurantID)
	if err != nil {
		return nil, err
	}

	var combos []map[string]interface{}
	for _, cm := range comboMeals {
		groups, _ := s.q.ListComboMealGroupsByComboMeal(ctx, cm.ID)
		var grpOut []map[string]interface{}
		for _, g := range groups {
			options, _ := s.q.ListComboMealGroupOptionsByGroup(ctx, g.ID)
			var opts []map[string]interface{}
			for _, o := range options {
				opts = append(opts, map[string]interface{}{
					"name":       o.ItemName.String,
					"adjustment": o.PriceAdjustment,
				})
			}
			grpOut = append(grpOut, map[string]interface{}{
				"name":    g.Name,
				"min":     g.MinChoices,
				"max":     g.MaxChoices,
				"options": opts,
			})
		}
		combo := map[string]interface{}{
			"id":     cm.ID,
			"name":   cm.Name,
			"price":  cm.Price,
			"groups": grpOut,
		}
		if cm.Description.Valid && cm.Description.String != "" {
			combo["description"] = cm.Description.String
		}
		combos = append(combos, combo)
	}

	return map[string]interface{}{
		"categories": cats,
		"combos":     combos,
	}, nil
}

func restaurantJSON(r db.Restaurant) map[string]interface{} {
	out := map[string]interface{}{
		"id":           r.ID,
		"name":         r.Name,
		"slug":         r.Slug,
		"dine_in":      r.DineIn,
		"takeout":      r.Takeout,
		"delivery":     r.Delivery,
		"is_published": r.IsPublished,
		"created_at":   r.CreatedAt.Time.Format(time.RFC3339),
	}
	if r.Address.Valid {
		out["address"] = r.Address.String
	}
	if r.PhoneNumber.Valid {
		out["phone_number"] = r.PhoneNumber.String
	}
	if r.Website.Valid {
		out["website"] = r.Website.String
	}
	if r.GooglePlaceID.Valid {
		out["google_place_id"] = r.GooglePlaceID.String
	}
	return out
}

func orderJSON(o db.Order, items []db.OrderItem) map[string]interface{} {
	out := map[string]interface{}{
		"id":           o.ID,
		"status":       o.Status,
		"total_amount": o.TotalAmount,
		"created_at":   o.CreatedAt.Time.Format(time.RFC3339),
	}
	if o.TableLabel.Valid {
		out["table_label"] = o.TableLabel.String
	}
	var itemsOut []map[string]interface{}
	for _, it := range items {
		item := map[string]interface{}{
			"id":         it.ID,
			"item_name":  it.ItemName,
			"quantity":   it.Quantity,
			"unit_price": it.UnitPrice,
		}
		if it.Notes.Valid {
			item["notes"] = it.Notes.String
		}
		itemsOut = append(itemsOut, item)
	}
	out["items"] = itemsOut
	return out
}

func parseID(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(r.PathValue(name), 10, 64)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func jsonResp(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

// --- HTML templates ---

const publicMenuHTML = `<!DOCTYPE html>
<html lang="zh-TW">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>%s - 菜單</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, "Noto Sans TC", sans-serif; background: #f5f5f5; color: #333; }
.container { max-width: 480px; margin: 0 auto; min-height: 100vh; background: #fff; padding-bottom: 70px; }
header { background: #e74c3c; color: #fff; padding: 16px 20px; }
header h1 { font-size: 20px; }
.category { margin-bottom: 8px; }
.category-header { background: #fafafa; padding: 12px 20px; font-size: 15px; font-weight: 700; color: #e74c3c; border-bottom: 1px solid #eee; }
.menu-item { display: flex; justify-content: space-between; align-items: center; padding: 14px 20px; border-bottom: 1px solid #f0f0f0; }
.item-info { flex: 1; }
.item-name { font-size: 16px; font-weight: 500; }
.item-desc { font-size: 12px; color: #999; margin-top: 2px; }
.item-price { font-size: 16px; font-weight: 700; color: #e74c3c; }
.add-btn { width: 28px; height: 28px; border-radius: 50%%; background: #e74c3c; color: #fff; border: none; font-size: 18px; cursor: pointer; margin-left: 10px; }
.cart-bar { display: none; position: fixed; bottom: 0; left: 50%%; transform: translateX(-50%%); width: 100%%; max-width: 480px; height: 60px; background: #333; color: #fff; z-index: 50; cursor: pointer; padding: 0 20px; align-items: center; justify-content: space-between; font-size: 16px; font-weight: 600; }
.cart-bar.visible { display: flex; }
.submit-btn { width: 100%%; padding: 14px; background: #e74c3c; color: #fff; border: none; border-radius: 10px; font-size: 16px; font-weight: 700; cursor: pointer; margin-top: 12px; }
</style>
</head>
<body>
<div class="container">
<header><h1>%s</h1></header>
<div id="menu"></div>
</div>
<div class="cart-bar" id="cartBar">
<span id="cartCount"></span>
<span id="cartTotal"></span>
</div>
<script>
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
return '<div class="menu-item"><div class="item-info"><div class="item-name">'+esc(it.name)+'</div>'+
(it.description?'<div class="item-desc">'+esc(it.description)+'</div>':'')+
'</div><span class="item-price">'+formatPrice(it.price)+'</span>'+
(it.price>=0?'<button class="add-btn" onclick="addToCart('+it.id+',\''+esc(it.name).replace(/'/g,"\\'")+'\','+it.price+')">+</button>':'')+
'</div>';}).join('')+'</div>';}).join('');
document.getElementById('menu').innerHTML=html;
}
function addToCart(id,name,price){
for(var i=0;i<cart.length;i++){if(cart[i].id===id){cart[i].qty++;updateCart();return;}}
cart.push({id:id,name:name,price:price,qty:1});updateCart();
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
var items=cart.map(function(c){return{menu_item_id:c.id,item_name:c.name,quantity:c.qty,unit_price:c.price};});
fetch('/api/public/orders/'+slug,{method:'POST',headers:{'Content-Type':'application/json'},
body:JSON.stringify({items:items,table_label:''})})
.then(function(r){return r.json();})
.then(function(d){alert('訂單已送出！訂單編號: '+d.id);cart=[];updateCart();});
}
render();
</script>
</body>
</html>`

const indexHTML = `<!DOCTYPE html>
<html lang="zh-TW">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>餐廳菜單</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, "Noto Sans TC", sans-serif; background: #f5f5f5; color: #333; }
.container { max-width: 480px; margin: 0 auto; min-height: 100vh; background: #fff; }
header { background: #e74c3c; color: #fff; padding: 16px 20px; }
header h1 { font-size: 20px; }
.restaurant-item { padding: 16px 20px; border-bottom: 1px solid #eee; cursor: pointer; }
.restaurant-item:hover { background: #fafafa; }
.restaurant-item .name { font-size: 17px; font-weight: 600; }
.restaurant-item .addr { font-size: 13px; color: #999; margin-top: 4px; }
.loading { text-align: center; padding: 40px; color: #999; }
.empty { text-align: center; padding: 60px 20px; color: #999; }
</style>
</head>
<body>
<div class="container">
<header><h1>餐廳菜單</h1></header>
<div id="list"><div class="loading">載入中...</div></div>
</div>
<script>
function esc(s){var d=document.createElement('div');d.textContent=s;return d.innerHTML;}
async function load(){
var res=await fetch('/api/restaurants');
var data=await res.json();
if(!data||data.length===0){document.getElementById('list').innerHTML='<div class="empty">目前沒有菜單資料</div>';return;}
document.getElementById('list').innerHTML=data.map(function(r){
return '<div class="restaurant-item" onclick="location.href=\'/r/'+r.slug+'\'">' +
'<div class="name">'+esc(r.name)+'</div>'+
(r.address?'<div class="addr">'+esc(r.address)+'</div>':'')+
'</div>';}).join('');
}
load();
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
body { font-family: monospace; background: #1a1a2e; color: #e0e0e0; padding: 16px; }
h1 { color: #0ff; font-size: 18px; margin-bottom: 8px; }
.stats { color: #888; margin-bottom: 16px; font-size: 13px; }
table { width: 100%%; border-collapse: collapse; font-size: 12px; }
th { background: #16213e; color: #0ff; padding: 8px 6px; text-align: left; }
td { padding: 6px; border-bottom: 1px solid #16213e; }
</style>
</head>
<body>
<h1>All Places</h1>
<div class="stats" id="stats">Loading...</div>
<table>
<thead><tr><th>ID</th><th>Name</th><th>Address</th><th>Types</th></tr></thead>
<tbody id="tbody"></tbody>
</table>
<script>
function esc(s){var d=document.createElement('div');d.textContent=s;return d.innerHTML;}
async function load(){
var res=await fetch('/api/places');
var data=await res.json();
document.getElementById('stats').textContent=data.length+' places';
document.getElementById('tbody').innerHTML=data.map(function(p){
return '<tr><td>'+p.id+'</td><td>'+esc(p.name)+'</td><td>'+esc(p.address||'')+'</td><td>'+(p.place_types||[]).join(', ')+'</td></tr>';
}).join('');
}
load();
</script>
</body>
</html>`

// io is used by handleUploadPhotos through multipart form reading
var _ = io.Discard
