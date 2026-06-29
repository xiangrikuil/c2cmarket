package review

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

type Repository interface {
	ListMyReviewCenterRows(ctx context.Context, userID string) ([]ReviewCenterRow, *domain.AppError)
	UpsertCarpoolReviewWithIdempotency(ctx context.Context, entry idempotency.Entry, input SubmitReviewInput, now time.Time, buildCompletion CompletionBuilder) (MutationResult, idempotency.Completion, *domain.AppError)
	ListPublicUserReviews(ctx context.Context, username string) ([]PublicReview, *domain.AppError)
}
