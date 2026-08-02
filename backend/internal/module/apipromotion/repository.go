package apipromotion

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

type Repository interface {
	ListPublicAPIPromotions(ctx context.Context, placement string, now time.Time) ([]Promotion, *domain.AppError)
	ListAdminAPIPromotions(ctx context.Context, now time.Time) ([]Promotion, *domain.AppError)
	GetAPIPromotionEligibility(ctx context.Context, serviceID string, now time.Time) (Eligibility, *domain.AppError)
	GetAPIPromotionAvailability(ctx context.Context, input AvailabilityInput, now time.Time) (Availability, *domain.AppError)
	CreateAPIPromotion(ctx context.Context, input CreateInput, now time.Time) (Promotion, *domain.AppError)
	CreateAPIPromotionWithIdempotency(ctx context.Context, entry idempotency.Entry, input CreateInput, now time.Time, buildCompletion CompletionBuilder) (Promotion, idempotency.Completion, *domain.AppError)
	StopAPIPromotion(ctx context.Context, input StopInput, now time.Time) (Promotion, *domain.AppError)
	StopAPIPromotionWithIdempotency(ctx context.Context, entry idempotency.Entry, input StopInput, now time.Time, buildCompletion CompletionBuilder) (Promotion, idempotency.Completion, *domain.AppError)
}
