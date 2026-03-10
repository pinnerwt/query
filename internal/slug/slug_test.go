package slug

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{"english", "My Restaurant", "my-restaurant-"},
		{"chinese only", "好吃餐廳", "restaurant-"},
		{"mixed", "Cafe 123 好", "cafe-123-"},
		{"special chars", "Bob's Burgers!!!", "bobs-burgers-"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slug := Generate(tt.input)
			assert.Contains(t, slug, tt.contains)
			// Should have random suffix (8 hex chars)
			assert.Regexp(t, `[a-z0-9-]+-[a-f0-9]{8}$`, slug)
		})
	}
}

func TestGenerate_unique(t *testing.T) {
	s1 := Generate("Test")
	s2 := Generate("Test")
	assert.NotEqual(t, s1, s2, "two slugs from same input should differ")
}
