package middleware

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestRateLimiterAllowsWithinWindowAndRejectsOverLimit(t *testing.T) {
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	limiter := NewRateLimiterWithClock(time.Minute, func() time.Time { return now })

	if decision := limiter.Allow("ip:search:127.0.0.1", 2); !decision.Allowed {
		t.Fatalf("expected first request allowed")
	}
	if decision := limiter.Allow("ip:search:127.0.0.1", 2); !decision.Allowed {
		t.Fatalf("expected second request allowed")
	}
	if decision := limiter.Allow("ip:search:127.0.0.1", 2); decision.Allowed || decision.RetryAfter <= 0 {
		t.Fatalf("expected third request rejected with retry-after, got %+v", decision)
	}

	now = now.Add(time.Minute)
	if decision := limiter.Allow("ip:search:127.0.0.1", 2); !decision.Allowed {
		t.Fatalf("expected new window to allow request")
	}
}

func TestRateLimiterBoundsKeysAndFailsClosedAtCapacity(t *testing.T) {
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	limiter := NewRateLimiterWithConfig(RateLimiterConfig{
		Window:          time.Minute,
		MaxKeys:         2,
		CleanupInterval: 30 * time.Second,
	}, func() time.Time { return now })

	if decision := limiter.Allow("ip:login:one", 5); !decision.Allowed {
		t.Fatalf("expected first key allowed")
	}
	if decision := limiter.Allow("ip:login:two", 5); !decision.Allowed {
		t.Fatalf("expected second key allowed")
	}
	decision := limiter.Allow("ip:login:three", 5)
	if decision.Allowed || !decision.CapacityExceeded || decision.RetryAfter != time.Minute {
		t.Fatalf("expected capacity rejection, got %+v", decision)
	}

	stats := limiter.Stats()
	if stats.ActiveKeys != 2 || stats.MaxKeys != 2 || stats.CapacityLimitedTotal != 1 || stats.LimitedTotal != 1 {
		t.Fatalf("unexpected capacity stats: %+v", stats)
	}

	now = now.Add(time.Minute)
	if decision := limiter.Allow("ip:login:three", 5); !decision.Allowed {
		t.Fatalf("expected expired keys to be cleaned before capacity check: %+v", decision)
	}
	stats = limiter.Stats()
	if stats.ActiveKeys != 1 || stats.ExpiredTotal != 2 {
		t.Fatalf("unexpected cleanup stats: %+v", stats)
	}
}

func TestRateLimiterScheduledCleanup(t *testing.T) {
	var clockMu sync.Mutex
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time {
		clockMu.Lock()
		defer clockMu.Unlock()
		return now
	}
	limiter := NewRateLimiterWithConfig(RateLimiterConfig{
		Window:          time.Minute,
		MaxKeys:         10,
		CleanupInterval: 2 * time.Millisecond,
	}, clock)
	if decision := limiter.Allow("user:report:user-1", 1); !decision.Allowed {
		t.Fatalf("expected key allowed")
	}

	clockMu.Lock()
	now = now.Add(time.Minute)
	clockMu.Unlock()
	limiter.Start(context.Background())
	defer limiter.Close()

	deadline := time.Now().Add(250 * time.Millisecond)
	for limiter.Stats().ActiveKeys != 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	stats := limiter.Stats()
	if stats.ActiveKeys != 0 || stats.ExpiredTotal != 1 {
		t.Fatalf("scheduled cleanup did not remove expired key: %+v", stats)
	}
}

func TestRateLimiterConcurrentCapacityNeverExceedsBound(t *testing.T) {
	limiter := NewRateLimiterWithConfig(RateLimiterConfig{
		Window:          time.Minute,
		MaxKeys:         32,
		CleanupInterval: time.Minute,
	}, time.Now)

	var wg sync.WaitGroup
	for index := 0; index < 128; index++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			limiter.Allow("key-"+time.Unix(int64(index), 0).Format(time.RFC3339Nano), 1)
		}(index)
	}
	wg.Wait()

	stats := limiter.Stats()
	if stats.ActiveKeys != 32 || stats.CapacityLimitedTotal != 96 {
		t.Fatalf("unexpected concurrent capacity stats: %+v", stats)
	}
}
