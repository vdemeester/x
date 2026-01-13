package pr

import (
	"math"
	"sync"
	"time"
)

// RateLimiter implements exponential backoff for API requests
type RateLimiter struct {
	mu           sync.Mutex
	baseDelay    time.Duration // Initial delay between requests
	backoffRate  float64       // Exponential backoff multiplier
	maxDelay     time.Duration // Maximum delay cap
	requestCount int           // Number of requests made
}

// NewRateLimiter creates a new rate limiter with exponential backoff
func NewRateLimiter(baseDelay time.Duration, backoffRate float64, maxDelay time.Duration) *RateLimiter {
	return &RateLimiter{
		baseDelay:    baseDelay,
		backoffRate:  backoffRate,
		maxDelay:     maxDelay,
		requestCount: 0,
	}
}

// Wait blocks until the appropriate delay has passed based on request count
func (rl *RateLimiter) Wait() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if rl.shouldDelay() {
		delay := rl.calculateDelay()
		time.Sleep(delay)
	}
}

// recordRequest increments the request counter
func (rl *RateLimiter) recordRequest() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.requestCount++
}

// shouldDelay returns true if we should delay before the next request
func (rl *RateLimiter) shouldDelay() bool {
	// No delay on first request
	return rl.requestCount > 0
}

// calculateDelay computes the delay using exponential backoff
func (rl *RateLimiter) calculateDelay() time.Duration {
	// Exponential backoff: baseDelay * (backoffRate ^ (requestCount - 1))
	exponent := float64(rl.requestCount - 1)
	multiplier := math.Pow(rl.backoffRate, exponent)
	delay := time.Duration(float64(rl.baseDelay) * multiplier)

	// Cap at maxDelay
	if delay > rl.maxDelay {
		delay = rl.maxDelay
	}

	return delay
}

// Reset resets the request counter (useful for testing or new batches)
func (rl *RateLimiter) Reset() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.requestCount = 0
}
