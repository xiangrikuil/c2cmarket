package idempotency

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
)

func TestServiceBeginHandlesCompletedReplayBodyConflictAndProcessingExpiry(t *testing.T) {
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	service := NewService(nil, func() time.Time { return now })
	ctx := context.Background()

	entry, appErr := service.Begin(ctx, "user-1", "POST /resource", "key-1", "hash-1")
	if appErr != nil {
		t.Fatalf("begin first: %v", appErr)
	}
	if appErr := service.Complete(ctx, entry, 201, "application/json", []byte(`{"ok":true}`), "resource", "res-1"); appErr != nil {
		t.Fatalf("complete: %v", appErr)
	}
	replay, appErr := service.Begin(ctx, "user-1", "POST /resource", "key-1", "hash-1")
	if appErr != nil {
		t.Fatalf("begin replay: %v", appErr)
	}
	if replay.State != "completed" || replay.Status != 201 || string(replay.Body) != `{"ok":true}` {
		t.Fatalf("unexpected replay entry: %+v body %s", replay, string(replay.Body))
	}
	if _, appErr := service.Begin(ctx, "user-1", "POST /resource", "key-1", "hash-2"); appErr == nil || appErr.Code != domain.CodeIdempotencyKeyReused {
		t.Fatalf("expected body conflict, got %v", appErr)
	}

	processing, appErr := service.Begin(ctx, "user-1", "POST /other", "key-2", "hash-3")
	if appErr != nil {
		t.Fatalf("begin processing: %v", appErr)
	}
	if _, appErr := service.Begin(ctx, "user-1", "POST /other", "key-2", "hash-3"); appErr == nil || appErr.Code != domain.CodeIdempotencyInProgress {
		t.Fatalf("expected in progress before expiry, got %v", appErr)
	}
	now = processing.ExpiresAt.Add(time.Second)
	retry, appErr := service.Begin(ctx, "user-1", "POST /other", "key-2", "hash-3")
	if appErr != nil {
		t.Fatalf("expected expired processing to retry: %v", appErr)
	}
	if retry.State != "processing" || !retry.CreatedAt.Equal(now) || !retry.ExpiresAt.After(now) {
		t.Fatalf("unexpected retry entry: %+v", retry)
	}
}

func TestServiceFailedRequestCanRetryAndRemainsBoundToHash(t *testing.T) {
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	service := NewService(nil, func() time.Time { return now })
	ctx := context.Background()

	entry, appErr := service.Begin(ctx, "user-1", "POST /resource", "key-1", "hash-1")
	if appErr != nil {
		t.Fatalf("begin: %v", appErr)
	}
	service.Cancel(ctx, entry)
	stored := service.entries[entryMapKey(entry.UserID, entry.RouteKey, entry.Key)]
	if stored.State != "failed" || stored.CompletedAt == nil || !stored.ExpiresAt.Equal(now.Add(FailedRetention)) {
		t.Fatalf("unexpected failed entry: %+v", stored)
	}
	if _, appErr := service.Begin(ctx, "user-1", "POST /resource", "key-1", "hash-2"); appErr == nil || appErr.Code != domain.CodeIdempotencyKeyReused {
		t.Fatalf("expected failed key hash conflict, got %v", appErr)
	}
	retry, appErr := service.Begin(ctx, "user-1", "POST /resource", "key-1", "hash-1")
	if appErr != nil || retry.State != "processing" {
		t.Fatalf("expected same failed request to retry, entry=%+v err=%v", retry, appErr)
	}
}

func TestServiceCompletedExpiryAllowsNewRequestHash(t *testing.T) {
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	service := NewService(nil, func() time.Time { return now })
	ctx := context.Background()

	entry, appErr := service.Begin(ctx, "user-1", "POST /resource", "key-1", "hash-1")
	if appErr != nil {
		t.Fatalf("begin: %v", appErr)
	}
	if appErr := service.Complete(ctx, entry, 201, "application/json", []byte(`{"ok":true}`), "resource", "res-1"); appErr != nil {
		t.Fatalf("complete: %v", appErr)
	}
	now = now.Add(CompletedRetention)
	reused, appErr := service.Begin(ctx, "user-1", "POST /resource", "key-1", "hash-2")
	if appErr != nil || reused.State != "processing" || reused.RequestHash != "hash-2" {
		t.Fatalf("expected expired completed key reuse, entry=%+v err=%v", reused, appErr)
	}
}

func TestServiceOversizedResponseStaysAuthoritativeWithoutBody(t *testing.T) {
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	service := NewService(nil, func() time.Time { return now })
	ctx := context.Background()
	entry, appErr := service.Begin(ctx, "user-1", "POST /resource", "key-1", "hash-1")
	if appErr != nil {
		t.Fatalf("begin: %v", appErr)
	}
	body := make([]byte, MaxCachedResponseBodySize+1)
	if appErr := service.Complete(ctx, entry, 201, "application/json", body, "resource", "res-1"); appErr != nil {
		t.Fatalf("complete: %v", appErr)
	}
	replay, appErr := service.Begin(ctx, "user-1", "POST /resource", "key-1", "hash-1")
	if appErr != nil {
		t.Fatalf("replay: %v", appErr)
	}
	if replay.BodyCacheAllowed || len(replay.Body) != 0 || replay.State != "completed" {
		t.Fatalf("oversized response was cached or lost authority: %+v", replay)
	}
	completion := CompletionFromEntry(replay)
	if completion.Status != http.StatusConflict || !strings.Contains(string(completion.Body), domain.CodeIdempotencyResultNotReplayable) {
		t.Fatalf("unexpected uncached replay completion: %+v body=%s", completion, completion.Body)
	}
}

func TestServiceSupersededGenerationCannotCompleteOrCancelReplacement(t *testing.T) {
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	service := NewService(nil, func() time.Time { return now })
	ctx := context.Background()

	stale, appErr := service.Begin(ctx, "user-1", "POST /resource", "key-1", "hash-1")
	if appErr != nil {
		t.Fatalf("begin stale generation: %v", appErr)
	}
	now = now.Add(ProcessingLifetime)
	replacement, appErr := service.Begin(ctx, "user-1", "POST /resource", "key-1", "hash-2")
	if appErr != nil {
		t.Fatalf("begin replacement generation: %v", appErr)
	}

	if appErr := service.Complete(ctx, stale, 201, "application/json", []byte(`{"stale":true}`), "resource", "res-1"); appErr == nil || appErr.Code != domain.CodeIdempotencyInProgress {
		t.Fatalf("expected stale completion rejection, got %v", appErr)
	}
	service.Cancel(ctx, stale)

	stored := service.entries[entryMapKey(replacement.UserID, replacement.RouteKey, replacement.Key)]
	if stored.State != "processing" || stored.RequestHash != replacement.RequestHash || !stored.CreatedAt.Equal(replacement.CreatedAt) {
		t.Fatalf("stale generation changed replacement: %+v", stored)
	}
}
