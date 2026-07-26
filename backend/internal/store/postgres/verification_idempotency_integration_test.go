package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/profile"

	"github.com/google/uuid"
)

func TestPostgresEmailVerificationChallengeLifecycle(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	user, appErr := store.EnsureUser(ctx, "verification-"+strings.ToLower(uuid.NewString()[:20]), false, now)
	if appErr != nil {
		t.Fatalf("ensure verification user: %v", appErr)
	}
	t.Cleanup(func() {
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM email_verification_codes WHERE user_id = $1`, user.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID)
	})

	service := profile.NewServiceWithOptions(store, func() time.Time { return now }, profile.NewDevelopmentEmailSender(), profile.ServiceOptions{
		EmailVerificationPepper: "postgres-test-email-verification-pepper",
	})
	first, appErr := service.StartEmailVerification(ctx, user, profile.EmailVerificationStartInput{Email: "first@example.com"})
	if appErr != nil {
		t.Fatalf("start first verification: %v", appErr)
	}
	var storedDigest string
	if err := store.pool.QueryRow(ctx, `
		SELECT code_hash
		FROM email_verification_codes
		WHERE user_id = $1 AND consumed_at IS NULL
	`, user.ID).Scan(&storedDigest); err != nil {
		t.Fatalf("read stored verification digest: %v", err)
	}
	bareInput := user.ID + ":first@example.com:" + first.DevCode
	bareDigest := sha256.Sum256([]byte(bareInput))
	if storedDigest == hex.EncodeToString(bareDigest[:]) {
		t.Fatal("PostgreSQL stored an unkeyed verification digest")
	}

	second, appErr := service.StartEmailVerification(ctx, user, profile.EmailVerificationStartInput{Email: "second@example.com"})
	if appErr != nil {
		t.Fatalf("start replacement verification: %v", appErr)
	}
	if _, appErr := service.ConfirmEmailVerification(ctx, user, profile.EmailVerificationConfirmInput{
		Email: "first@example.com",
		Code:  first.DevCode,
	}); appErr == nil {
		t.Fatal("replaced PostgreSQL challenge unexpectedly succeeded")
	}
	for attempt := 1; attempt <= profile.EmailVerificationMaxAttempts; attempt++ {
		if _, appErr := service.ConfirmEmailVerification(ctx, user, profile.EmailVerificationConfirmInput{
			Email: "second@example.com",
			Code:  "000000",
		}); appErr == nil {
			t.Fatalf("wrong PostgreSQL verification attempt %d unexpectedly succeeded", attempt)
		}
	}
	if _, appErr := service.ConfirmEmailVerification(ctx, user, profile.EmailVerificationConfirmInput{
		Email: "second@example.com",
		Code:  second.DevCode,
	}); appErr == nil {
		t.Fatal("locked PostgreSQL challenge accepted the correct code")
	}
	var attempts int
	var consumedAt *time.Time
	if err := store.pool.QueryRow(ctx, `
		SELECT attempt_count, consumed_at
		FROM email_verification_codes
		WHERE user_id = $1 AND email = 'second@example.com'
		ORDER BY created_at DESC
		LIMIT 1
	`, user.ID).Scan(&attempts, &consumedAt); err != nil {
		t.Fatalf("read locked verification challenge: %v", err)
	}
	if attempts != profile.EmailVerificationMaxAttempts || consumedAt == nil {
		t.Fatalf("unexpected locked verification state attempts=%d consumed_at=%v", attempts, consumedAt)
	}

	now = now.Add(time.Minute)
	concurrent, appErr := service.StartEmailVerification(ctx, user, profile.EmailVerificationStartInput{Email: "concurrent@example.com"})
	if appErr != nil {
		t.Fatalf("start concurrent verification: %v", appErr)
	}
	var wait sync.WaitGroup
	var mu sync.Mutex
	successes := 0
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, appErr := service.ConfirmEmailVerification(ctx, user, profile.EmailVerificationConfirmInput{
				Email: "concurrent@example.com",
				Code:  concurrent.DevCode,
			})
			if appErr == nil {
				mu.Lock()
				successes++
				mu.Unlock()
			}
		}()
	}
	wait.Wait()
	if successes != 1 {
		t.Fatalf("expected one concurrent PostgreSQL confirmation, got %d", successes)
	}

	now = now.Add(time.Minute)
	expiring, appErr := service.StartEmailVerification(ctx, user, profile.EmailVerificationStartInput{Email: "expired@example.com"})
	if appErr != nil {
		t.Fatalf("start expiring verification: %v", appErr)
	}
	now = now.Add(profile.EmailVerificationLifetime)
	if _, appErr := service.ConfirmEmailVerification(ctx, user, profile.EmailVerificationConfirmInput{
		Email: "expired@example.com",
		Code:  expiring.DevCode,
	}); appErr == nil {
		t.Fatal("expired PostgreSQL challenge unexpectedly succeeded")
	}
}

func TestPostgresIdempotencyLifecycleBoundsBodiesAndRejectsStaleGeneration(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	user, appErr := store.EnsureUser(ctx, "idempotency-"+strings.ToLower(uuid.NewString()[:20]), false, now)
	if appErr != nil {
		t.Fatalf("ensure idempotency user: %v", appErr)
	}
	t.Cleanup(func() {
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM idempotency_keys WHERE user_id = $1`, user.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID)
	})

	service := idempotency.NewService(store, func() time.Time { return now })
	stale, appErr := service.Begin(ctx, user.ID, "POST /resource", "bounded-key", "hash-1")
	if appErr != nil {
		t.Fatalf("begin bounded idempotency: %v", appErr)
	}
	body := make([]byte, idempotency.MaxCachedResponseBodySize+1)
	resourceID := uuid.NewString()
	if appErr := service.Complete(ctx, stale, 201, "application/json", body, "resource", resourceID); appErr != nil {
		t.Fatalf("complete bounded idempotency: %v", appErr)
	}

	var state string
	var bodyCached bool
	var bodyIsNull bool
	var expiresAt time.Time
	if err := store.pool.QueryRow(ctx, `
		SELECT status, response_body_cache_allowed, response_body_json IS NULL, expires_at
		FROM idempotency_keys
		WHERE user_id = $1 AND route_key = $2 AND idempotency_key = $3
	`, user.ID, stale.RouteKey, stale.Key).Scan(&state, &bodyCached, &bodyIsNull, &expiresAt); err != nil {
		t.Fatalf("read bounded idempotency row: %v", err)
	}
	if state != "completed" || bodyCached || !bodyIsNull || !expiresAt.Equal(now.Add(idempotency.CompletedRetention)) {
		t.Fatalf("unexpected bounded idempotency row state=%s cached=%t null=%t expires=%s", state, bodyCached, bodyIsNull, expiresAt)
	}
	replay, appErr := service.Begin(ctx, user.ID, stale.RouteKey, stale.Key, stale.RequestHash)
	if appErr != nil || replay.State != "completed" || replay.BodyCacheAllowed {
		t.Fatalf("unexpected bounded idempotency replay entry=%+v err=%v", replay, appErr)
	}
	if completion := idempotency.CompletionFromEntry(replay); completion.Status != 409 || !strings.Contains(string(completion.Body), domain.CodeIdempotencyResultNotReplayable) {
		t.Fatalf("unexpected non-replayable completion: %+v", completion)
	}

	now = now.Add(idempotency.CompletedRetention)
	replacement, appErr := service.Begin(ctx, user.ID, stale.RouteKey, stale.Key, "hash-2")
	if appErr != nil {
		t.Fatalf("reuse expired completed key: %v", appErr)
	}
	if appErr := service.Complete(ctx, stale, 201, "application/json", []byte(`{"stale":true}`), "resource", resourceID); appErr == nil || appErr.Code != domain.CodeIdempotencyInProgress {
		t.Fatalf("expected stale PostgreSQL completion rejection, got %v", appErr)
	}
	service.Cancel(ctx, stale)
	var storedHash string
	if err := store.pool.QueryRow(ctx, `
		SELECT status, request_hash
		FROM idempotency_keys
		WHERE user_id = $1 AND route_key = $2 AND idempotency_key = $3
	`, user.ID, replacement.RouteKey, replacement.Key).Scan(&state, &storedHash); err != nil {
		t.Fatalf("read replacement idempotency row: %v", err)
	}
	if state != "processing" || storedHash != replacement.RequestHash {
		t.Fatalf("stale generation changed replacement state=%s hash=%s", state, storedHash)
	}

	now = now.Add(time.Minute)
	failed, appErr := service.Begin(ctx, user.ID, "POST /resource", "failed-key", "failed-hash")
	if appErr != nil {
		t.Fatalf("begin failed idempotency: %v", appErr)
	}
	service.Cancel(ctx, failed)
	now = now.Add(time.Millisecond)
	retry, appErr := service.Begin(ctx, user.ID, failed.RouteKey, failed.Key, failed.RequestHash)
	if appErr != nil || retry.State != "processing" {
		t.Fatalf("retry failed idempotency entry=%+v err=%v", retry, appErr)
	}
}
