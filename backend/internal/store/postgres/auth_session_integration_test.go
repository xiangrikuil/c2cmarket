package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"

	"github.com/google/uuid"
)

func TestPostgresLogoutRevokesExactlyOnceHashedSession(t *testing.T) {
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

	now := time.Date(2026, 8, 13, 1, 0, 0, 0, time.UTC)
	service := auth.NewService(store, func() time.Time { return now })
	username := "logout-" + strings.ToLower(uuid.NewString()[:12])
	user, session, appErr := service.CreateDevSession(ctx, username, false)
	if appErr != nil {
		t.Fatalf("create real postgres session: %v", appErr)
	}
	defer func() {
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM auth_sessions WHERE user_id = $1`, user.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM linux_do_bindings WHERE user_id = $1`, user.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM auth_identities WHERE user_id = $1`, user.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID)
	}()

	onceHash := testOpaqueTokenHash(session.ID)
	doubleHash := testOpaqueTokenHash(onceHash)
	if _, _, appErr := service.GetSession(ctx, session.ID); appErr != nil {
		t.Fatalf("fresh session was not readable: %v", appErr)
	}

	service.Logout(ctx, session.ID)

	var onceHashedRows int
	var revokedAt *time.Time
	if err := store.pool.QueryRow(ctx, `
		SELECT count(*), max(revoked_at)
		FROM auth_sessions
		WHERE session_token_hash = $1
	`, onceHash).Scan(&onceHashedRows, &revokedAt); err != nil {
		t.Fatalf("inspect once-hashed session row: %v", err)
	}
	if onceHashedRows != 1 || revokedAt == nil || !revokedAt.Equal(now) {
		t.Fatalf("logout did not revoke the once-hashed row: count=%d revoked_at=%v", onceHashedRows, revokedAt)
	}
	var doubleHashedRows int
	if err := store.pool.QueryRow(ctx, `SELECT count(*) FROM auth_sessions WHERE session_token_hash = $1`, doubleHash).Scan(&doubleHashedRows); err != nil {
		t.Fatalf("inspect double-hashed token: %v", err)
	}
	if doubleHashedRows != 0 {
		t.Fatalf("logout addressed a double-hashed token row: count=%d", doubleHashedRows)
	}
	if _, _, appErr := service.GetSession(ctx, session.ID); appErr == nil || appErr.Code != domain.CodeSessionRevoked {
		t.Fatalf("revoked raw token remained readable: %v", appErr)
	}
}

func testOpaqueTokenHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

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
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM linux_do_bindings WHERE user_id = $1`, user.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM auth_identities WHERE user_id = $1`, user.ID)
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
