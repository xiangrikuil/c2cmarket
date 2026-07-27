package idempotency

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
)

type Repository interface {
	BeginIdempotency(ctx context.Context, entry Entry) (*Entry, *domain.AppError)
	CompleteIdempotency(ctx context.Context, entry *Entry, completion Completion, completedAt time.Time) *domain.AppError
	CancelIdempotency(ctx context.Context, entry *Entry, failedAt time.Time) *domain.AppError
}
