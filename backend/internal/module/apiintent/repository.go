package apiintent

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

type Repository interface {
	CreateAPIPurchaseIntentWithIdempotency(ctx context.Context, entry idempotency.Entry, input CreateIntentInput, now time.Time, buildCompletion CompletionBuilder) (Intent, idempotency.Completion, *domain.AppError)
	ListAPIPurchaseIntentsByBuyer(ctx context.Context, buyerUserID string, now time.Time) ([]Intent, *domain.AppError)
	GetAPIPurchaseIntentForBuyer(ctx context.Context, buyerUserID, intentID string, now time.Time) (Intent, *domain.AppError)
	GetAPIPurchaseIntentForBuyerWithMerchantContact(ctx context.Context, buyerUserID, intentID, requestID string, now time.Time) (Intent, *domain.AppError)
	ListAPIPurchaseIntentsByOwner(ctx context.Context, ownerUserID string, now time.Time) ([]Intent, *domain.AppError)
	GetAPIPurchaseIntentForOwner(ctx context.Context, ownerUserID, intentID string, now time.Time) (Intent, *domain.AppError)
	GetAPIPurchaseIntentForOwnerWithBuyerContact(ctx context.Context, ownerUserID, intentID, requestID string, now time.Time) (Intent, *domain.AppError)
	ListAdminAPIPurchaseIntents(ctx context.Context, now time.Time) ([]Intent, *domain.AppError)
	GetAdminAPIPurchaseIntent(ctx context.Context, intentID string, now time.Time) (Intent, *domain.AppError)
	CancelAPIPurchaseIntentWithIdempotency(ctx context.Context, entry idempotency.Entry, input ActionInput, now time.Time, buildCompletion CompletionBuilder) (Intent, idempotency.Completion, *domain.AppError)
	MarkAPIPurchaseIntentContactedWithIdempotency(ctx context.Context, entry idempotency.Entry, input ActionInput, now time.Time, buildCompletion CompletionBuilder) (Intent, idempotency.Completion, *domain.AppError)
	CloseAPIPurchaseIntentWithIdempotency(ctx context.Context, entry idempotency.Entry, input ActionInput, now time.Time, buildCompletion CompletionBuilder) (Intent, idempotency.Completion, *domain.AppError)
}
