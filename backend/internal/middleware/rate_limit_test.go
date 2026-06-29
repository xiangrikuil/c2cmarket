package middleware

import (
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
