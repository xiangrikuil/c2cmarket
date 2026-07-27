package review

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

type Repository interface {
	ResolveTransactionForReview(ctx context.Context, transactionType, transactionID, userID string) (Transaction, *domain.AppError)
	ListMyReviewCenterRows(ctx context.Context, userID string, now time.Time) ([]ReviewCenterRow, *domain.AppError)
	SaveTransactionReviewWithIdempotency(ctx context.Context, entry idempotency.Entry, transaction Transaction, input SubmitReviewInput, now time.Time, buildCompletion CompletionBuilder) (MutationResult, idempotency.Completion, *domain.AppError)
	RemoveTransactionReviewWithIdempotency(ctx context.Context, entry idempotency.Entry, input RemoveReviewInput, now time.Time, buildCompletion CompletionBuilder) (MutationResult, idempotency.Completion, *domain.AppError)
	ListPublicUserReviews(ctx context.Context, username string, now time.Time) ([]PublicReview, *domain.AppError)
}
