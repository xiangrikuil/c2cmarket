package idempotency

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
)

type Repository interface {
	BeginIdempotency(ctx context.Context, entry Entry) (*Entry, *domain.AppError)
	CompleteIdempotency(ctx context.Context, entry *Entry, status int, contentType string, body []byte, resourceType, resourceID string, completedAt time.Time) *domain.AppError
	CancelIdempotency(ctx context.Context, entry *Entry) *domain.AppError
	CleanupExpiredIdempotency(ctx context.Context, before time.Time) *domain.AppError
}
