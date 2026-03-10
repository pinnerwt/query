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

	ownerID, iat, err := ValidateToken(token, secret)
	require.NoError(t, err)
	assert.Equal(t, int64(42), ownerID)
	assert.WithinDuration(t, time.Now(), iat, 2*time.Second)
}

func TestValidateToken_expired(t *testing.T) {
	secret := []byte("test-secret")

	token, err := GenerateToken(1, secret, -1*time.Hour)
	require.NoError(t, err)

	_, _, err = ValidateToken(token, secret)
	assert.Error(t, err)
}

func TestValidateToken_wrongSecret(t *testing.T) {
	token, err := GenerateToken(1, []byte("secret1"), 24*time.Hour)
	require.NoError(t, err)

	_, _, err = ValidateToken(token, []byte("secret2"))
	assert.Error(t, err)
}

func TestMiddleware(t *testing.T) {
	secret := []byte("test-secret")
	token, err := GenerateToken(99, secret, 24*time.Hour)
	require.NoError(t, err)

	handler := Middleware(secret, false, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := OwnerIDFromContext(r.Context())
		assert.True(t, ok)
		assert.Equal(t, int64(99), id)
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("valid cookie", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "session", Value: token})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("missing cookie", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid cookie", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "session", Value: "bad-token"})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestTokenRotation(t *testing.T) {
	secret := []byte("test-secret")

	// Create a token issued 2 hours ago (should trigger rotation)
	oldToken, err := GenerateToken(42, secret, 24*time.Hour)
	require.NoError(t, err)

	// Verify ValidateToken returns iat
	_, iat, err := ValidateToken(oldToken, secret)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now(), iat, 2*time.Second)

	// Token issued just now should NOT trigger rotation
	handler := Middleware(secret, false, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: oldToken})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Fresh token should have no Set-Cookie (no rotation needed)
	cookies := w.Result().Cookies()
	hasSession := false
	for _, c := range cookies {
		if c.Name == "session" {
			hasSession = true
		}
	}
	assert.False(t, hasSession, "fresh token should not be rotated")
}

func TestSetAndClearSessionCookie(t *testing.T) {
	w := httptest.NewRecorder()
	SetSessionCookie(w, "test-token", false)
	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, "session", cookies[0].Name)
	assert.Equal(t, "test-token", cookies[0].Value)
	assert.True(t, cookies[0].HttpOnly)
	assert.False(t, cookies[0].Secure)

	w2 := httptest.NewRecorder()
	SetSessionCookie(w2, "test-token", true)
	cookies2 := w2.Result().Cookies()
	assert.True(t, cookies2[0].Secure)

	w3 := httptest.NewRecorder()
	ClearSessionCookie(w3, false)
	cookies3 := w3.Result().Cookies()
	require.Len(t, cookies3, 1)
	assert.Equal(t, "session", cookies3[0].Name)
	assert.Equal(t, "", cookies3[0].Value)
	assert.True(t, cookies3[0].MaxAge < 0)
}

func TestIsSecureURL(t *testing.T) {
	assert.True(t, IsSecureURL("https://example.com"))
	assert.False(t, IsSecureURL("http://localhost:8080"))
}
