package middleware

import (
	"context"
	"sync"
	"time"
)

const (
	defaultRateLimitMaxKeys        = 100_000
	defaultRateLimitCleanupDivisor = 2
)

type RateLimitDecision struct {
	Allowed          bool
	RetryAfter       time.Duration
	CapacityExceeded bool
}

type RateLimiterConfig struct {
	Window          time.Duration
	MaxKeys         int
	CleanupInterval time.Duration
}

type RateLimiterStats struct {
	MaxKeys              int
	ActiveKeys           int
	AllowedTotal         uint64
	LimitedTotal         uint64
	CapacityLimitedTotal uint64
	ExpiredTotal         uint64
}

type RateLimiter struct {
	mu              sync.Mutex
	window          time.Duration
	maxKeys         int
	cleanupInterval time.Duration
	nextCleanup     time.Time
	now             func() time.Time
	entries         map[string]rateLimitEntry
	allowedTotal    uint64
	limitedTotal    uint64
	capacityLimited uint64
	expiredTotal    uint64
	lifecycleMu     sync.Mutex
	cleanupStarted  bool
	cleanupClosed   bool
	stopCleanup     chan struct{}
	cleanupDone     chan struct{}
}

type rateLimitEntry struct {
	WindowStart time.Time
	Count       int
}

func NewRateLimiter(window time.Duration) *RateLimiter {
	return NewRateLimiterWithClock(window, time.Now)
}

func NewRateLimiterWithClock(window time.Duration, now func() time.Time) *RateLimiter {
	return NewRateLimiterWithConfig(RateLimiterConfig{Window: window}, now)
}

func NewRateLimiterWithConfig(cfg RateLimiterConfig, now func() time.Time) *RateLimiter {
	if now == nil {
		now = time.Now
	}
	if cfg.Window <= 0 {
		cfg.Window = time.Minute
	}
	if cfg.MaxKeys <= 0 {
		cfg.MaxKeys = defaultRateLimitMaxKeys
	}
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = cfg.Window / defaultRateLimitCleanupDivisor
		if cfg.CleanupInterval <= 0 {
			cfg.CleanupInterval = time.Second
		}
	}
	current := now()
	return &RateLimiter{
		window:          cfg.Window,
		maxKeys:         cfg.MaxKeys,
		cleanupInterval: cfg.CleanupInterval,
		nextCleanup:     current.Add(cfg.CleanupInterval),
		now:             now,
		entries:         map[string]rateLimitEntry{},
		stopCleanup:     make(chan struct{}),
		cleanupDone:     make(chan struct{}),
	}
}

func (l *RateLimiter) Allow(key string, limit int) RateLimitDecision {
	if l == nil || limit <= 0 || key == "" {
		return RateLimitDecision{Allowed: true}
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	if !now.Before(l.nextCleanup) {
		l.cleanupLocked(now)
		l.nextCleanup = now.Add(l.cleanupInterval)
	}
	entry, found := l.entries[key]
	if found && now.Sub(entry.WindowStart) >= l.window {
		l.entries[key] = rateLimitEntry{WindowStart: now, Count: 1}
		l.allowedTotal++
		return RateLimitDecision{Allowed: true}
	}
	if !found {
		if len(l.entries) >= l.maxKeys {
			l.limitedTotal++
			l.capacityLimited++
			return RateLimitDecision{
				Allowed:          false,
				RetryAfter:       l.window,
				CapacityExceeded: true,
			}
		}
		l.entries[key] = rateLimitEntry{WindowStart: now, Count: 1}
		l.allowedTotal++
		return RateLimitDecision{Allowed: true}
	}
	if entry.Count >= limit {
		retryAfter := l.window - now.Sub(entry.WindowStart)
		if retryAfter <= 0 {
			retryAfter = time.Second
		}
		l.limitedTotal++
		return RateLimitDecision{Allowed: false, RetryAfter: retryAfter}
	}
	entry.Count++
	l.entries[key] = entry
	l.allowedTotal++
	return RateLimitDecision{Allowed: true}
}

func (l *RateLimiter) Start(ctx context.Context) {
	if l == nil {
		return
	}
	l.lifecycleMu.Lock()
	if l.cleanupStarted || l.cleanupClosed {
		l.lifecycleMu.Unlock()
		return
	}
	l.cleanupStarted = true
	l.lifecycleMu.Unlock()

	go func() {
		defer close(l.cleanupDone)
		ticker := time.NewTicker(l.cleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-l.stopCleanup:
				return
			case <-ticker.C:
				l.CleanupExpired()
			}
		}
	}()
}

func (l *RateLimiter) Close() {
	if l == nil {
		return
	}
	l.lifecycleMu.Lock()
	if l.cleanupClosed {
		l.lifecycleMu.Unlock()
		return
	}
	l.cleanupClosed = true
	started := l.cleanupStarted
	if started {
		close(l.stopCleanup)
	}
	l.lifecycleMu.Unlock()
	if started {
		<-l.cleanupDone
	}
}

func (l *RateLimiter) CleanupExpired() int {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	removed := l.cleanupLocked(l.now())
	l.nextCleanup = l.now().Add(l.cleanupInterval)
	return removed
}

func (l *RateLimiter) Stats() RateLimiterStats {
	if l == nil {
		return RateLimiterStats{}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return RateLimiterStats{
		MaxKeys:              l.maxKeys,
		ActiveKeys:           len(l.entries),
		AllowedTotal:         l.allowedTotal,
		LimitedTotal:         l.limitedTotal,
		CapacityLimitedTotal: l.capacityLimited,
		ExpiredTotal:         l.expiredTotal,
	}
}

func (l *RateLimiter) cleanupLocked(now time.Time) int {
	removed := 0
	for key, entry := range l.entries {
		if !entry.WindowStart.Add(l.window).After(now) {
			delete(l.entries, key)
			removed++
		}
	}
	l.expiredTotal += uint64(removed)
	return removed
}
