package elastic

import (
	"math"
	"sync"
	"time"
)

type RateLimiter struct {
	lastRequest     time.Time
	retryAfter      time.Duration
	initialDelay    time.Duration
	maxDelay        time.Duration
	retryMultiplier float64
	mu              sync.Mutex
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		lastRequest:     time.Now(),
		retryAfter:      time.Millisecond * 100,
		initialDelay:    time.Millisecond * 100,
		maxDelay:        5 * time.Second,
		retryMultiplier: 2.0,
	}
}

func (r *RateLimiter) Wait() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if elapsed := time.Since(r.lastRequest); elapsed < r.retryAfter {
		time.Sleep(r.retryAfter - elapsed)
	}
	r.lastRequest = time.Now()
}

func (r *RateLimiter) HandleTooManyRequests() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.retryAfter = time.Duration(math.Min(
		float64(r.retryAfter)*r.retryMultiplier,
		float64(r.maxDelay),
	))
}

func (r *RateLimiter) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.retryAfter = r.initialDelay
}

func (r *RateLimiter) GetRetryAfter() time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.retryAfter
}
