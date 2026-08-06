package promotionreward

import (
	"context"
	"net/http"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"
)

type fakeRepository struct {
	config    PublicConfig
	summary   ReferralSummary
	coupons   CouponPage
	campaign  Campaign
	lastQuery CouponQuery
}

func (f *fakeRepository) GetPromotionRewardPublicConfig(context.Context, time.Time) (PublicConfig, *domain.AppError) {
	return f.config, nil
}
func (f *fakeRepository) GetReferralSummary(context.Context, string, time.Time) (ReferralSummary, *domain.AppError) {
	return f.summary, nil
}
func (f *fakeRepository) ListUserPromotionCoupons(_ context.Context, _ string, query CouponQuery, _ time.Time) (CouponPage, *domain.AppError) {
	f.lastQuery = query
	return f.coupons, nil
}
func (f *fakeRepository) ApplyPromotionCouponWithIdempotency(context.Context, idempotency.Entry, ApplyCouponInput, time.Time, CouponCompletionBuilder) (Coupon, idempotency.Completion, *domain.AppError) {
	return Coupon{}, idempotency.Completion{}, nil
}
func (f *fakeRepository) GetAdminPromotionRewardCampaign(context.Context) (Campaign, *domain.AppError) {
	return f.campaign, nil
}
func (f *fakeRepository) UpdateAdminPromotionRewardCampaign(context.Context, UpdateCampaignInput, time.Time) (Campaign, *domain.AppError) {
	return f.campaign, nil
}
func (f *fakeRepository) UpdateAdminPromotionRewardCampaignWithIdempotency(_ context.Context, _ idempotency.Entry, _ UpdateCampaignInput, _ time.Time, build CampaignCompletionBuilder) (Campaign, idempotency.Completion, *domain.AppError) {
	completion, appErr := build(f.campaign)
	return f.campaign, completion, appErr
}
func (f *fakeRepository) ListAdminReferrals(context.Context, ReferralQuery) (ReferralPage, *domain.AppError) {
	return ReferralPage{}, nil
}
func (f *fakeRepository) RevokeAdminReferral(context.Context, RevokeReferralInput, time.Time) (ReferralRecord, *domain.AppError) {
	return ReferralRecord{}, nil
}
func (f *fakeRepository) RevokeAdminReferralWithIdempotency(_ context.Context, _ idempotency.Entry, _ RevokeReferralInput, _ time.Time, build ReferralCompletionBuilder) (ReferralRecord, idempotency.Completion, *domain.AppError) {
	item := ReferralRecord{}
	completion, appErr := build(item)
	return item, completion, appErr
}
func (f *fakeRepository) ListAdminPromotionCoupons(context.Context, CouponQuery, time.Time) (CouponPage, *domain.AppError) {
	return CouponPage{}, nil
}
func (f *fakeRepository) GrantAdminPromotionCoupon(context.Context, GrantCouponInput, time.Time) (Coupon, *domain.AppError) {
	return Coupon{}, nil
}
func (f *fakeRepository) GrantAdminPromotionCouponWithIdempotency(_ context.Context, _ idempotency.Entry, _ GrantCouponInput, _ time.Time, build CouponCompletionBuilder) (Coupon, idempotency.Completion, *domain.AppError) {
	item := Coupon{}
	completion, appErr := build(item)
	return item, completion, appErr
}
func (f *fakeRepository) RevokeAdminPromotionCoupon(context.Context, RevokeCouponInput, time.Time) (Coupon, *domain.AppError) {
	return Coupon{}, nil
}
func (f *fakeRepository) RevokeAdminPromotionCouponWithIdempotency(_ context.Context, _ idempotency.Entry, _ RevokeCouponInput, _ time.Time, build CouponCompletionBuilder) (Coupon, idempotency.Completion, *domain.AppError) {
	item := Coupon{}
	completion, appErr := build(item)
	return item, completion, appErr
}

func TestEffectiveCouponStatus(t *testing.T) {
	now := time.Date(2026, 8, 2, 8, 0, 0, 0, time.UTC)
	available := Coupon{StoredStatus: CouponStatusPending, AvailableAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour)}
	if got := EffectiveCouponStatus(available, now); got != CouponStatusAvailable {
		t.Fatalf("available status = %q", got)
	}
	pending := available
	pending.AvailableAt = now.Add(time.Minute)
	if got := EffectiveCouponStatus(pending, now); got != CouponStatusPending {
		t.Fatalf("pending status = %q", got)
	}
	expired := available
	expired.ExpiresAt = now
	if got := EffectiveCouponStatus(expired, now); got != CouponStatusExpired {
		t.Fatalf("expired status = %q", got)
	}
	used := available
	used.StoredStatus = CouponStatusUsed
	if got := EffectiveCouponStatus(used, now); got != CouponStatusUsed {
		t.Fatalf("used status = %q", got)
	}
}

func TestCanonicalReferralCode(t *testing.T) {
	if got := CanonicalReferralCode(" 2abcde89 "); got != "2ABCDE89" {
		t.Fatalf("canonical code = %q", got)
	}
	for _, value := range []string{"", "ABCDEFG", "ABC0EFGH", "ABCIEFGH", "ABCDEFGHI"} {
		if got := CanonicalReferralCode(value); got != "" {
			t.Fatalf("invalid code %q normalized to %q", value, got)
		}
	}
}

func TestMyReferralHonorsFeatureFlagsAndRemovesRiskData(t *testing.T) {
	user := auth.User{ID: "user-1"}
	repo := &fakeRepository{summary: ReferralSummary{Campaign: PublicConfig{}, Records: []ReferralRecord{{RiskFlags: []string{"shared_ip"}}}}}
	service := NewService(repo, nil, time.Now)
	if _, appErr := service.MyReferral(context.Background(), user); appErr == nil || appErr.Code != domain.CodeFeatureDisabled {
		t.Fatalf("expected feature disabled, got %#v", appErr)
	}
	repo.summary.Campaign = PublicConfig{ProgramEnabled: true, ReferralEnabled: true}
	result, appErr := service.MyReferral(context.Background(), user)
	if appErr != nil {
		t.Fatalf("my referral: %v", appErr)
	}
	if len(result.Records[0].RiskFlags) != 0 {
		t.Fatalf("risk flags leaked: %+v", result.Records[0])
	}
}

func TestCouponQueryDefaultsAndValidation(t *testing.T) {
	repo := &fakeRepository{coupons: CouponPage{Items: []Coupon{}}}
	service := NewService(repo, nil, time.Now)
	if _, appErr := service.MyCoupons(context.Background(), auth.User{ID: "user-1"}, CouponQuery{}); appErr != nil {
		t.Fatalf("my coupons: %v", appErr)
	}
	if repo.lastQuery.Page != 1 || repo.lastQuery.Limit != 20 || repo.lastQuery.Status != CouponStatusAll {
		t.Fatalf("query not normalized: %+v", repo.lastQuery)
	}
	if _, appErr := service.MyCoupons(context.Background(), auth.User{ID: "user-1"}, CouponQuery{Limit: 101}); appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("expected limit validation, got %#v", appErr)
	}
}

func TestAdminCampaignRequiresAdminAndValidContract(t *testing.T) {
	service := NewService(&fakeRepository{}, nil, time.Now)
	if _, appErr := service.AdminCampaign(context.Background(), auth.User{ID: "user-1"}); appErr == nil || appErr.Code != domain.CodePermissionDenied {
		t.Fatalf("expected permission error, got %#v", appErr)
	}
	input := UpdateCampaignInput{ExpectedVersion: 1, StartsAt: time.Now(), PromotionDurationHours: 24, CouponValidDays: 30, RewardDelayHours: 72, InviterMonthlyLimit: 10, RulesText: "活动规则", Reason: "更新活动"}
	if _, appErr := service.UpdateAdminCampaign(context.Background(), auth.User{ID: "admin", IsAdmin: true}, input); appErr != nil {
		t.Fatalf("update campaign: %v", appErr)
	}
	input.PromotionDurationHours = 0
	if _, appErr := service.UpdateAdminCampaign(context.Background(), auth.User{ID: "admin", IsAdmin: true}, input); appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("expected duration validation, got %#v", appErr)
	}
}

func TestAdminIdempotentMutationsRequireIdempotencyKey(t *testing.T) {
	now := time.Date(2026, 8, 2, 8, 0, 0, 0, time.UTC)
	service := NewService(&fakeRepository{}, idempotency.NewService(nil, func() time.Time { return now }), func() time.Time { return now })
	admin := auth.User{ID: "admin-1", IsAdmin: true}
	completionBuilder := func(Coupon) (idempotency.Completion, *domain.AppError) {
		return idempotency.Completion{}, nil
	}
	campaignBuilder := func(Campaign) (idempotency.Completion, *domain.AppError) {
		return idempotency.Completion{}, nil
	}
	referralBuilder := func(ReferralRecord) (idempotency.Completion, *domain.AppError) {
		return idempotency.Completion{}, nil
	}

	tests := []struct {
		name string
		run  func() *domain.AppError
	}{
		{
			name: "campaign update",
			run: func() *domain.AppError {
				_, appErr := service.UpdateAdminCampaignWithIdempotency(context.Background(), admin, "PATCH /campaign", "", "campaign-hash", UpdateCampaignInput{
					ExpectedVersion: 1, StartsAt: now, PromotionDurationHours: 24,
					CouponValidDays: 30, RewardDelayHours: 72, InviterMonthlyLimit: 10,
					RulesText: "活动规则", Reason: "更新活动",
				}, campaignBuilder)
				return appErr
			},
		},
		{
			name: "referral revoke",
			run: func() *domain.AppError {
				_, appErr := service.RevokeAdminReferralWithIdempotency(context.Background(), admin, "POST /referral/revoke", "", "referral-hash", RevokeReferralInput{
					ReferralID: "referral-1", ExpectedVersion: 1, Reason: "撤销邀请",
				}, referralBuilder)
				return appErr
			},
		},
		{
			name: "coupon grant",
			run: func() *domain.AppError {
				_, appErr := service.GrantAdminCouponWithIdempotency(context.Background(), admin, "POST /coupon/grant", "", "grant-hash", GrantCouponInput{
					UserID: "user-1", DurationHours: 24, ValidDays: 30, Reason: "人工补发",
				}, completionBuilder)
				return appErr
			},
		},
		{
			name: "coupon revoke",
			run: func() *domain.AppError {
				_, appErr := service.RevokeAdminCouponWithIdempotency(context.Background(), admin, "POST /coupon/revoke", "", "revoke-hash", RevokeCouponInput{
					CouponID: "coupon-1", ExpectedVersion: 1, Reason: "撤销权益",
				}, completionBuilder)
				return appErr
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			appErr := test.run()
			if appErr == nil || appErr.Code != domain.CodeValidationFailed || appErr.Status != http.StatusBadRequest {
				t.Fatalf("expected missing idempotency key validation error, got %#v", appErr)
			}
		})
	}
}
