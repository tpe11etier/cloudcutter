package elastic

import (
	"math"
	"sync"
	"time"
)

// RateLimiter handles rate limiting for Elasticsearch requests
type RateLimiter struct {
	lastRequest     time.Time
	retryAfter      time.Duration
	initialDelay    time.Duration
	maxDelay        time.Duration
	retryMultiplier float64
	mu              sync.RWMutex
}

// NewRateLimiter creates a new rate limiter with default settings
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		lastRequest:     time.Now(),
		retryAfter:      time.Millisecond * 100, // Default 100ms delay for backward compatibility
		initialDelay:    time.Millisecond * 100,
		maxDelay:        5 * time.Second,
		retryMultiplier: 2.0,
	}
}

// NewRateLimiterWithConfig creates a new rate limiter with configuration
func NewRateLimiterWithConfig(config *RateLimitConfig) *RateLimiter {
	return &RateLimiter{
		lastRequest:     time.Now(),
		retryAfter:      config.InitialRetryDelay,
		initialDelay:    config.InitialRetryDelay,
		maxDelay:        config.MaxRetryDelay,
		retryMultiplier: config.RetryMultiplier,
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
	// Exponential backoff with configurable max delay
	r.retryAfter = time.Duration(math.Min(
		float64(r.retryAfter)*r.retryMultiplier,
		float64(r.maxDelay),
	))
}

// Reset resets the backoff time to default
func (r *RateLimiter) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.retryAfter = r.initialDelay
}

// GetRetryAfter returns current retry delay
func (r *RateLimiter) GetRetryAfter() time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.retryAfter
}
