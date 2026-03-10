package ratelimit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSlidingWindow(t *testing.T) {
	sw := NewSlidingWindow(3, 1*time.Second)

	assert.True(t, sw.Allow("a"))
	assert.True(t, sw.Allow("a"))
	assert.True(t, sw.Allow("a"))
	assert.False(t, sw.Allow("a"))

	// Different key is independent
	assert.True(t, sw.Allow("b"))
}

func TestSlidingWindow_expiry(t *testing.T) {
	sw := NewSlidingWindow(2, 50*time.Millisecond)

	assert.True(t, sw.Allow("x"))
	assert.True(t, sw.Allow("x"))
	assert.False(t, sw.Allow("x"))

	time.Sleep(60 * time.Millisecond)

	assert.True(t, sw.Allow("x"))
}
