package idempotency

import (
	"context"
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
