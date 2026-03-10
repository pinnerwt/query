package auth

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const ownerIDKey contextKey = "owner_id"

// Middleware extracts and validates the Bearer token, setting owner_id in context.
func Middleware(secret []byte, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			http.Error(w, `{"error":"missing authorization"}`, http.StatusUnauthorized)
			return
		}

		tokenStr := strings.TrimPrefix(header, "Bearer ")
		ownerID, err := ValidateToken(tokenStr, secret)
		if err != nil {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
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
