package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/pinnertw/query/internal/auth"
	"github.com/pinnertw/query/internal/db/dbtest"
	db "github.com/pinnertw/query/internal/db/generated"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthIntegration(t *testing.T) {
	conn := dbtest.SetupTestDB(t)
	ctx := context.Background()
	q := db.New(conn)
	secret := []byte("integration-test-secret")

	t.Run("register and login flow", func(t *testing.T) {
		// Hash password
		hash, err := auth.HashPassword("mypassword")
		require.NoError(t, err)

		// Create owner
		owner, err := q.CreateOwner(ctx, db.CreateOwnerParams{
			Email:        "test@example.com",
			PasswordHash: hash,
			Name:         "Test Owner",
		})
		require.NoError(t, err)
		assert.Equal(t, "test@example.com", owner.Email)
		assert.Equal(t, "Test Owner", owner.Name)

		// Verify email uniqueness
		_, err = q.CreateOwner(ctx, db.CreateOwnerParams{
			Email:        "test@example.com",
			PasswordHash: hash,
			Name:         "Duplicate",
		})
		require.Error(t, err)

		// Login: find owner by email
		found, err := q.GetOwnerByEmail(ctx, "test@example.com")
		require.NoError(t, err)
		assert.Equal(t, owner.ID, found.ID)

		// Verify password
		err = auth.CheckPassword(found.PasswordHash, "mypassword")
		require.NoError(t, err)

		// Generate token
		token, err := auth.GenerateToken(found.ID, secret, 24*time.Hour)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		// Validate token
		ownerID, err := auth.ValidateToken(token, secret)
		require.NoError(t, err)
		assert.Equal(t, found.ID, ownerID)

		// Get owner by ID (for /me endpoint)
		me, err := q.GetOwnerByID(ctx, ownerID)
		require.NoError(t, err)
		assert.Equal(t, "test@example.com", me.Email)
	})

	t.Run("auth middleware protects endpoints", func(t *testing.T) {
		hash, err := auth.HashPassword("pass")
		require.NoError(t, err)
		owner, err := q.CreateOwner(ctx, db.CreateOwnerParams{
			Email:        "middleware@test.com",
			PasswordHash: hash,
			Name:         "MW Test",
		})
		require.NoError(t, err)

		token, err := auth.GenerateToken(owner.ID, secret, 24*time.Hour)
		require.NoError(t, err)

		// Build a protected handler
		protected := auth.Middleware(secret, false, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ownerID, ok := auth.OwnerIDFromContext(r.Context())
			if !ok {
				http.Error(w, "no owner", 500)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]int64{"owner_id": ownerID})
		}))

		// With valid cookie
		req := httptest.NewRequest("GET", "/api/auth/me", nil)
		req.AddCookie(&http.Cookie{Name: "session", Value: token})
		w := httptest.NewRecorder()
		protected.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), "owner_id")

		// Without token
		req2 := httptest.NewRequest("GET", "/api/auth/me", nil)
		w2 := httptest.NewRecorder()
		protected.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusUnauthorized, w2.Code)
	})

	t.Run("register endpoint simulation", func(t *testing.T) {
		// Simulate POST /api/auth/register
		type registerReq struct {
			Email    string `json:"email"`
			Password string `json:"password"`
			Name     string `json:"name"`
		}
		body := registerReq{
			Email:    "new@example.com",
			Password: "strongpass123",
			Name:     "New Owner",
		}
		bodyJSON, _ := json.Marshal(body)

		// Parse request
		var reg registerReq
		err := json.Unmarshal(bodyJSON, &reg)
		require.NoError(t, err)

		// Hash and create
		hash, err := auth.HashPassword(reg.Password)
		require.NoError(t, err)

		owner, err := q.CreateOwner(ctx, db.CreateOwnerParams{
			Email:        reg.Email,
			PasswordHash: hash,
			Name:         reg.Name,
		})
		require.NoError(t, err)

		// Generate token
		token, err := auth.GenerateToken(owner.ID, secret, 24*time.Hour)
		require.NoError(t, err)

		// Verify token works
		id, err := auth.ValidateToken(token, secret)
		require.NoError(t, err)
		assert.Equal(t, owner.ID, id)
	})

	t.Run("login endpoint simulation", func(t *testing.T) {
		// Register first
		hash, err := auth.HashPassword("logintest")
		require.NoError(t, err)
		_, err = q.CreateOwner(ctx, db.CreateOwnerParams{
			Email:        "login@test.com",
			PasswordHash: hash,
			Name:         "Login Test",
		})
		require.NoError(t, err)

		// Login
		type loginReq struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		bodyJSON, _ := json.Marshal(loginReq{Email: "login@test.com", Password: "logintest"})

		var login loginReq
		err = json.Unmarshal(bodyJSON, &login)
		require.NoError(t, err)

		owner, err := q.GetOwnerByEmail(ctx, login.Email)
		require.NoError(t, err)

		err = auth.CheckPassword(owner.PasswordHash, login.Password)
		require.NoError(t, err)

		token, err := auth.GenerateToken(owner.ID, secret, 24*time.Hour)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		// Wrong password
		err = auth.CheckPassword(owner.PasswordHash, "wrong")
		assert.Error(t, err)
	})

	_ = strings.NewReader // used for body building
}
