package middleware

import (
	"sync"
	"time"
)

type RateLimitDecision struct {
	Allowed    bool
	RetryAfter time.Duration
}

type RateLimiter struct {
	mu      sync.Mutex
	window  time.Duration
	now     func() time.Time
	entries map[string]rateLimitEntry
}

type rateLimitEntry struct {
	WindowStart time.Time
	Count       int
}

func NewRateLimiter(window time.Duration) *RateLimiter {
	return NewRateLimiterWithClock(window, time.Now)
}

func NewRateLimiterWithClock(window time.Duration, now func() time.Time) *RateLimiter {
	if window <= 0 {
		window = time.Minute
	}
	if now == nil {
		now = time.Now
	}
	return &RateLimiter{
		window:  window,
		now:     now,
		entries: map[string]rateLimitEntry{},
	}
}

func (l *RateLimiter) Allow(key string, limit int) RateLimitDecision {
	if l == nil || limit <= 0 || key == "" {
		return RateLimitDecision{Allowed: true}
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	entry := l.entries[key]
	if entry.WindowStart.IsZero() || now.Sub(entry.WindowStart) >= l.window {
		l.entries[key] = rateLimitEntry{WindowStart: now, Count: 1}
		l.cleanupLocked(now)
		return RateLimitDecision{Allowed: true}
	}
	if entry.Count >= limit {
		retryAfter := l.window - now.Sub(entry.WindowStart)
		if retryAfter < 0 {
			retryAfter = 0
		}
		return RateLimitDecision{Allowed: false, RetryAfter: retryAfter}
	}
	entry.Count++
	l.entries[key] = entry
	return RateLimitDecision{Allowed: true}
}

func (l *RateLimiter) cleanupLocked(now time.Time) {
	threshold := now.Add(-2 * l.window)
	for key, entry := range l.entries {
		if entry.WindowStart.Before(threshold) {
			delete(l.entries, key)
		}
	}
}
