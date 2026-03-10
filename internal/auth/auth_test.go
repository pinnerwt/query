package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("secret123")
	require.NoError(t, err)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, "secret123", hash)

	err = CheckPassword(hash, "secret123")
	assert.NoError(t, err)

	err = CheckPassword(hash, "wrong")
	assert.Error(t, err)
}

func TestGenerateAndValidateToken(t *testing.T) {
	secret := []byte("test-secret")

	token, err := GenerateToken(42, secret, 24*time.Hour)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	ownerID, err := ValidateToken(token, secret)
	require.NoError(t, err)
	assert.Equal(t, int64(42), ownerID)
}

func TestValidateToken_expired(t *testing.T) {
	secret := []byte("test-secret")

	token, err := GenerateToken(1, secret, -1*time.Hour)
	require.NoError(t, err)

	_, err = ValidateToken(token, secret)
	assert.Error(t, err)
}

func TestValidateToken_wrongSecret(t *testing.T) {
	token, err := GenerateToken(1, []byte("secret1"), 24*time.Hour)
	require.NoError(t, err)

	_, err = ValidateToken(token, []byte("secret2"))
	assert.Error(t, err)
}

func TestMiddleware(t *testing.T) {
	secret := []byte("test-secret")
	token, err := GenerateToken(99, secret, 24*time.Hour)
	require.NoError(t, err)

	handler := Middleware(secret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := OwnerIDFromContext(r.Context())
		assert.True(t, ok)
		assert.Equal(t, int64(99), id)
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("valid token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("missing token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", "Bearer bad-token")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
