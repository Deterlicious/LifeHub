package httpapi

import (
	"sync"
	"time"
)

const maximumRateLimitKeys = 10_000

type rateLimitWindow struct {
	startedAt time.Time
	count     int
}

type fixedWindowLimiter struct {
	mu      sync.Mutex
	entries map[string]rateLimitWindow
	limit   int
	window  time.Duration
	clock   func() time.Time
}

func newFixedWindowLimiter(limit int, window time.Duration, clock func() time.Time) *fixedWindowLimiter {
	return &fixedWindowLimiter{
		entries: make(map[string]rateLimitWindow),
		limit:   limit,
		window:  window,
		clock:   clock,
	}
}

func (limiter *fixedWindowLimiter) Allow(key string) (allowed bool, remaining int, retryAfter time.Duration) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	now := limiter.clock()
	entry, exists := limiter.entries[key]
	if !exists || now.Sub(entry.startedAt) >= limiter.window || now.Before(entry.startedAt) {
		if len(limiter.entries) >= maximumRateLimitKeys {
			limiter.removeExpired(now)
		}
		if !exists && len(limiter.entries) >= maximumRateLimitKeys {
			key = "_overflow"
			entry, exists = limiter.entries[key]
		}
		if !exists || now.Sub(entry.startedAt) >= limiter.window || now.Before(entry.startedAt) {
			entry = rateLimitWindow{startedAt: now}
		}
	}
	entry.count++
	limiter.entries[key] = entry
	remaining = limiter.limit - entry.count
	if remaining < 0 {
		remaining = 0
	}
	retryAfter = limiter.window - now.Sub(entry.startedAt)
	if retryAfter < time.Second {
		retryAfter = time.Second
	}
	return entry.count <= limiter.limit, remaining, retryAfter
}

func (limiter *fixedWindowLimiter) removeExpired(now time.Time) {
	for key, entry := range limiter.entries {
		if now.Sub(entry.startedAt) >= limiter.window || now.Before(entry.startedAt) {
			delete(limiter.entries, key)
		}
	}
}
