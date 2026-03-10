package slug

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)

// Generate creates a URL-safe slug from a restaurant name with a random suffix.
func Generate(name string) string {
	// Normalize unicode
	s := norm.NFKD.String(name)

	// Keep only ASCII letters and digits
	var b strings.Builder
	for _, r := range s {
		if r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '-') {
			b.WriteRune(unicode.ToLower(r))
		}
	}

	slug := strings.TrimSpace(b.String())
	slug = nonAlphaNum.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")

	// For CJK names that produce empty slugs, use "restaurant"
	if slug == "" {
		slug = "restaurant"
	}

	// Append random suffix for uniqueness
	suffix := randomHex(4)
	return slug + "-" + suffix
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
