package elastic

import (
	"math"
	"sync"
	"time"
)

// RateLimiter handles rate limiting for Elasticsearch requests
type RateLimiter struct {
	lastRequest time.Time
	retryAfter  time.Duration
	mu          sync.RWMutex
}

// NewRateLimiter creates a new rate limiter with default settings
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		lastRequest: time.Now(),
		retryAfter:  time.Millisecond * 100, // Start with 100ms delay
	}
}

// Wait enforces minimum delay between requests
func (r *RateLimiter) Wait() {
	r.mu.RLock()
	waitTime := time.Since(r.lastRequest)
	if waitTime < r.retryAfter {
		time.Sleep(r.retryAfter - waitTime)
	}
	r.mu.RUnlock()

	r.mu.Lock()
	r.lastRequest = time.Now()
	r.mu.Unlock()
}

// HandleTooManyRequests increases backoff time
func (r *RateLimiter) HandleTooManyRequests() {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Exponential backoff with max of 5 seconds
	r.retryAfter = time.Duration(math.Min(
		float64(r.retryAfter)*2,
		float64(5*time.Second),
	))
}

// Reset resets the backoff time to default
func (r *RateLimiter) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.retryAfter = time.Millisecond * 100
}

// GetRetryAfter returns current retry delay
func (r *RateLimiter) GetRetryAfter() time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.retryAfter
}
