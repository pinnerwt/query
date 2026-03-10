package auth

import (
	"context"
	"net/http"
	"time"
)

type contextKey string

const ownerIDKey contextKey = "owner_id"

const rotationInterval = 1 * time.Hour

// Middleware extracts and validates the session cookie, setting owner_id in context.
// If the token is older than rotationInterval, it issues a fresh token via Set-Cookie.
func Middleware(secret []byte, secure bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(cookieName)
		if err != nil || cookie.Value == "" {
			http.Error(w, `{"error":"missing session"}`, http.StatusUnauthorized)
			return
		}

		tokenStr := cookie.Value
		ownerID, err := ValidateToken(tokenStr, secret)
		if err != nil {
			http.Error(w, `{"error":"invalid session"}`, http.StatusUnauthorized)
			return
		}

		// Rotate token if older than rotationInterval
		if iat, err := TokenIssuedAt(tokenStr, secret); err == nil {
			if time.Since(iat) > rotationInterval {
				if newToken, err := GenerateToken(ownerID, secret, 24*time.Hour); err == nil {
					SetSessionCookie(w, newToken, secure)
				}
			}
		}

		ctx := context.WithValue(r.Context(), ownerIDKey, ownerID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// OwnerIDFromContext extracts the authenticated owner ID from the request context.
func OwnerIDFromContext(ctx context.Context) (int64, bool) {
	id, ok := ctx.Value(ownerIDKey).(int64)
	return id, ok
}
