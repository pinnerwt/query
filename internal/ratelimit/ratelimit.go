package ratelimit

import (
	"net/http"
	"sync"
	"time"
)

// SlidingWindow implements a per-key sliding window rate limiter.
type SlidingWindow struct {
	mu      sync.Mutex
	windows map[string][]time.Time
	limit   int
	window  time.Duration
}

// NewSlidingWindow creates a rate limiter allowing `limit` requests per `window` per key.
func NewSlidingWindow(limit int, window time.Duration) *SlidingWindow {
	return &SlidingWindow{
		windows: make(map[string][]time.Time),
		limit:   limit,
		window:  window,
	}
}

// Allow checks if a request from the given key is allowed.
func (sw *SlidingWindow) Allow(key string) bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-sw.window)

	// Prune expired entries
	timestamps := sw.windows[key]
	start := 0
	for start < len(timestamps) && timestamps[start].Before(cutoff) {
		start++
	}
	timestamps = timestamps[start:]

	if len(timestamps) >= sw.limit {
		sw.windows[key] = timestamps
		return false
	}

	sw.windows[key] = append(timestamps, now)
	return true
}

// Middleware wraps an HTTP handler with per-IP rate limiting.
func (sw *SlidingWindow) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip = r.RemoteAddr
		}

		if !sw.Allow(ip) {
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}
