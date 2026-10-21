package httpapi

import (
	"sync"
	"time"
)

// RateLimiter is a generic rate limiter that tracks attempts within a time window.
type RateLimiter struct {
	mu       sync.Mutex
	attempts map[string][]time.Time
	window   time.Duration
	limit    int
	maxKeys  int
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(window time.Duration, limit int) *RateLimiter {
	return &RateLimiter{
		attempts: make(map[string][]time.Time),
		window:   window,
		limit:    limit,
		maxKeys:  10000,
	}
}

// Allow returns true if the key has not exceeded the limit within the time window.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Evict expired attempts for this key first.
	existing, hasKey := rl.attempts[key]
	valid := make([]time.Time, 0, len(existing))
	for _, t := range existing {
		if now.Sub(t) < rl.window {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.limit {
		if len(valid) == 0 {
			delete(rl.attempts, key)
		} else {
			rl.attempts[key] = valid
		}
		// Clean up expired entries from other keys while we are here.
		rl.cleanupExpired(now)
		return false
	}

	// Clean up expired entries before enforcing the global key cap.
	rl.cleanupExpired(now)

	// Hard cap on total number of keys to prevent memory exhaustion.
	if !hasKey && len(rl.attempts) >= rl.maxKeys {
		return false
	}

	rl.attempts[key] = append(valid, now)
	return true
}

// cleanupExpired evicts all expired attempts regardless of current size.
func (rl *RateLimiter) cleanupExpired(now time.Time) {
	for k, times := range rl.attempts {
		valid := make([]time.Time, 0, len(times))
		for _, t := range times {
			if now.Sub(t) < rl.window {
				valid = append(valid, t)
			}
		}
		if len(valid) == 0 {
			delete(rl.attempts, k)
		} else {
			rl.attempts[k] = valid
		}
	}
}

// Clear clears the attempts for the specified key.
func (rl *RateLimiter) Clear(key string) {
	rl.mu.Lock()
	delete(rl.attempts, key)
	rl.mu.Unlock()
}
