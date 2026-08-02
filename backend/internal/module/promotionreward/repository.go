package promotionreward

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

type Repository interface {
	GetPromotionRewardPublicConfig(ctx context.Context, now time.Time) (PublicConfig, *domain.AppError)
	GetReferralSummary(ctx context.Context, userID string, now time.Time) (ReferralSummary, *domain.AppError)
	ListUserPromotionCoupons(ctx context.Context, userID string, query CouponQuery, now time.Time) (CouponPage, *domain.AppError)
	ApplyPromotionCouponWithIdempotency(ctx context.Context, entry idempotency.Entry, input ApplyCouponInput, now time.Time, buildCompletion CouponCompletionBuilder) (Coupon, idempotency.Completion, *domain.AppError)

	GetAdminPromotionRewardCampaign(ctx context.Context) (Campaign, *domain.AppError)
	UpdateAdminPromotionRewardCampaign(ctx context.Context, input UpdateCampaignInput, now time.Time) (Campaign, *domain.AppError)
	UpdateAdminPromotionRewardCampaignWithIdempotency(ctx context.Context, entry idempotency.Entry, input UpdateCampaignInput, now time.Time, buildCompletion CampaignCompletionBuilder) (Campaign, idempotency.Completion, *domain.AppError)
	ListAdminReferrals(ctx context.Context, query ReferralQuery) (ReferralPage, *domain.AppError)
	RevokeAdminReferral(ctx context.Context, input RevokeReferralInput, now time.Time) (ReferralRecord, *domain.AppError)
	RevokeAdminReferralWithIdempotency(ctx context.Context, entry idempotency.Entry, input RevokeReferralInput, now time.Time, buildCompletion ReferralCompletionBuilder) (ReferralRecord, idempotency.Completion, *domain.AppError)
	ListAdminPromotionCoupons(ctx context.Context, query CouponQuery, now time.Time) (CouponPage, *domain.AppError)
	GrantAdminPromotionCoupon(ctx context.Context, input GrantCouponInput, now time.Time) (Coupon, *domain.AppError)
	GrantAdminPromotionCouponWithIdempotency(ctx context.Context, entry idempotency.Entry, input GrantCouponInput, now time.Time, buildCompletion CouponCompletionBuilder) (Coupon, idempotency.Completion, *domain.AppError)
	RevokeAdminPromotionCoupon(ctx context.Context, input RevokeCouponInput, now time.Time) (Coupon, *domain.AppError)
	RevokeAdminPromotionCouponWithIdempotency(ctx context.Context, entry idempotency.Entry, input RevokeCouponInput, now time.Time, buildCompletion CouponCompletionBuilder) (Coupon, idempotency.Completion, *domain.AppError)
}
