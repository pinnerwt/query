package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const cookieName = "session"

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidToken       = errors.New("invalid token")
)

// HashPassword hashes a plaintext password with bcrypt.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword verifies a password against a bcrypt hash.
func CheckPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// GenerateToken creates a JWT for the given owner ID.
func GenerateToken(ownerID int64, secret []byte, expiry time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"owner_id": ownerID,
		"exp":      time.Now().Add(expiry).Unix(),
		"iat":      time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(secret)
}

const tokenExpiry = 24 * time.Hour

func sessionCookie(value string, maxAge int, secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     cookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   maxAge,
	}
}

// SetSessionCookie sets an HttpOnly session cookie with the JWT.
func SetSessionCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, sessionCookie(token, int(tokenExpiry.Seconds()), secure))
}

// ClearSessionCookie removes the session cookie.
func ClearSessionCookie(w http.ResponseWriter, secure bool) {
	http.SetCookie(w, sessionCookie("", -1, secure))
}

// IsSecureURL returns true if the base URL uses HTTPS.
func IsSecureURL(baseURL string) bool {
	return strings.HasPrefix(baseURL, "https://")
}

// ValidateToken parses and validates a JWT, returning the owner ID and issued-at time.
func ValidateToken(tokenString string, secret []byte) (int64, time.Time, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return secret, nil
	})
	if err != nil {
		return 0, time.Time{}, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return 0, time.Time{}, ErrInvalidToken
	}

	ownerIDFloat, ok := claims["owner_id"].(float64)
	if !ok {
		return 0, time.Time{}, ErrInvalidToken
	}

	var iat time.Time
	if iatFloat, ok := claims["iat"].(float64); ok {
		iat = time.Unix(int64(iatFloat), 0)
	}

	return int64(ownerIDFloat), iat, nil
}
