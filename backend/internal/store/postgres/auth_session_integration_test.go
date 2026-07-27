package postgres

import (
	"context"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"c2c-market/backend/internal/module/auth"

	"github.com/google/uuid"
)

func TestPostgresSessionRenewalUpdatesExactlyOnceAtBoundary(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("C2C_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not configured")
	}

	ctx := context.Background()
	store, err := Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer store.Close()

	startedAt := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	username := "sr-" + strings.ToLower(uuid.NewString()[:20])
	user, appErr := store.EnsureUser(ctx, username, false, startedAt)
	if appErr != nil {
		t.Fatalf("ensure session test user: %v", appErr)
	}
	defer func() {
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM auth_sessions WHERE user_id = $1`, user.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID)
	}()

	tokenHash := "session-token-hash-" + uuid.NewString()
	if appErr := store.CreateSession(
		ctx,
		user.ID,
		tokenHash,
		"csrf-token-hash-"+uuid.NewString(),
		startedAt.Add(auth.SessionIdleLifetime),
		startedAt.Add(auth.SessionAbsoluteLifetime),
		startedAt,
	); appErr != nil {
		t.Fatalf("create session fixture: %v", appErr)
	}

	renewedAt := startedAt.Add(auth.SessionRenewalInterval)
	targetExpiresAt := renewedAt.Add(auth.SessionIdleLifetime)
	renewBefore := renewedAt.Add(-auth.SessionRenewalInterval)
	var renewedCount atomic.Int32
	var waitGroup sync.WaitGroup
	errors := make(chan error, 16)
	for range 16 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			_, renewed, appErr := store.RenewSession(ctx, tokenHash, renewedAt, targetExpiresAt, renewBefore)
			if appErr != nil {
				errors <- appErr
				return
			}
			if renewed {
				renewedCount.Add(1)
			}
		}()
	}
	waitGroup.Wait()
	close(errors)
	for err := range errors {
		t.Fatalf("concurrent session renewal: %v", err)
	}
	if renewedCount.Load() != 1 {
		t.Fatalf("expected exactly one renewal update, got %d", renewedCount.Load())
	}

	var storedRenewedAt time.Time
	var storedExpiresAt time.Time
	if err := store.pool.QueryRow(ctx, `
		SELECT renewed_at, expires_at
		FROM auth_sessions
		WHERE session_token_hash = $1
	`, tokenHash).Scan(&storedRenewedAt, &storedExpiresAt); err != nil {
		t.Fatalf("read renewed session: %v", err)
	}
	if !storedRenewedAt.Equal(renewedAt) || !storedExpiresAt.Equal(targetExpiresAt) {
		t.Fatalf("unexpected renewed timestamps renewed_at=%s expires_at=%s", storedRenewedAt, storedExpiresAt)
	}
}
