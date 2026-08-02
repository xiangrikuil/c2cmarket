package postgres

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apipromotion"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/promotionreward"

	"github.com/google/uuid"
)

func TestPromotionRewardPostgresLifecycle(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("C2C_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not configured")
	}

	ctx := context.Background()
	store, err := Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer store.Close()
	requireAuthIdentityTestDatabase(t, store)

	now := time.Date(2026, 8, 2, 4, 0, 0, 0, time.UTC)
	before, appErr := store.GetAdminPromotionRewardCampaign(ctx)
	if appErr != nil {
		t.Fatalf("read original campaign: %v", appErr)
	}
	if _, err := store.pool.Exec(ctx, `
		UPDATE promotion_reward_campaigns
		SET program_enabled = true, welcome_enabled = true, referral_enabled = true,
		    starts_at = $1, ends_at = $2, promotion_duration_hours = 24,
		    coupon_valid_days = 30, reward_delay_hours = 72,
		    inviter_monthly_limit = 10, rules_text = 'PostgreSQL integration test',
		    updated_at = $3, version = version + 1
		WHERE code = $4
	`, now.Add(-time.Hour), now.AddDate(0, 1, 0), now, promotionreward.CampaignCodeAPIServiceReferralV1); err != nil {
		t.Fatalf("enable promotion reward campaign: %v", err)
	}

	suffix := strings.ToLower(strings.ReplaceAll(uuid.NewString()[:8], "-", ""))
	userIDs := make([]string, 0, 3)
	defer func() {
		restorePromotionRewardCampaign(t, store, before)
		cleanupPromotionRewardIntegrationFixture(t, store, userIDs)
	}()

	inviterResult, appErr := store.UpsertOAuthUser(ctx, auth.OAuthProfile{
		Provider: "linux_do", Subject: "reward-inviter-" + suffix,
		Username: "reward-inviter-" + suffix, DisplayName: "邀请用户",
		LinuxDoUserID: "reward-linux-inviter-" + suffix,
	}, now)
	if appErr != nil {
		t.Fatalf("create inviter: %v", appErr)
	}
	userIDs = append(userIDs, inviterResult.User.ID)
	summary, appErr := store.GetReferralSummary(ctx, inviterResult.User.ID, now)
	if appErr != nil {
		t.Fatalf("create inviter code: %v", appErr)
	}
	if promotionreward.CanonicalReferralCode(summary.Code) == "" {
		t.Fatalf("expected canonical referral code, got %q", summary.Code)
	}
	repeatedSummary, appErr := store.GetReferralSummary(ctx, inviterResult.User.ID, now.Add(time.Minute))
	if appErr != nil || repeatedSummary.Code != summary.Code {
		t.Fatalf("referral code must remain stable: first=%q second=%q error=%v", summary.Code, repeatedSummary.Code, appErr)
	}

	inviteeResult, appErr := store.UpsertOAuthUser(ctx, auth.OAuthProfile{
		Provider: "linux_do", Subject: "reward-invitee-" + suffix,
		Username: "reward-invitee-" + suffix, DisplayName: "受邀用户",
		LinuxDoUserID: "reward-linux-invitee-" + suffix, ReferralCode: strings.ToLower(summary.Code),
	}, now.Add(2*time.Minute))
	if appErr != nil || !inviteeResult.Created {
		t.Fatalf("create referred OAuth user: result=%+v error=%v", inviteeResult, appErr)
	}
	userIDs = append(userIDs, inviteeResult.User.ID)

	loggedInAgain, appErr := store.UpsertOAuthUser(ctx, auth.OAuthProfile{
		Provider: "linux_do", Subject: "reward-invitee-" + suffix,
		Username: "changed-name-" + suffix, ReferralCode: summary.Code,
	}, now.Add(3*time.Minute))
	if appErr != nil || loggedInAgain.Created || loggedInAgain.User.ID != inviteeResult.User.ID {
		t.Fatalf("existing OAuth login changed referral ownership: result=%+v error=%v", loggedInAgain, appErr)
	}

	invalidResult, appErr := store.UpsertOAuthUser(ctx, auth.OAuthProfile{
		Provider: "linux_do", Subject: "reward-invalid-" + suffix,
		Username: "reward-invalid-" + suffix, LinuxDoUserID: "reward-linux-invalid-" + suffix,
		ReferralCode: "INVALID!",
	}, now.Add(4*time.Minute))
	if appErr != nil || !invalidResult.Created {
		t.Fatalf("invalid referral code blocked OAuth creation: result=%+v error=%v", invalidResult, appErr)
	}
	userIDs = append(userIDs, invalidResult.User.ID)
	var invalidRelations int
	if err := store.pool.QueryRow(ctx, `SELECT count(*)::int FROM referral_relations WHERE invitee_user_id = $1`, invalidResult.User.ID).Scan(&invalidRelations); err != nil || invalidRelations != 0 {
		t.Fatalf("invalid referral code created relation: count=%d error=%v", invalidRelations, err)
	}

	var relationID string
	if err := store.pool.QueryRow(ctx, `
		SELECT id::text FROM referral_relations
		WHERE inviter_user_id = $1 AND invitee_user_id = $2 AND status = 'bound'
	`, inviterResult.User.ID, inviteeResult.User.ID).Scan(&relationID); err != nil {
		t.Fatalf("read bound referral relation: %v", err)
	}

	serviceID := uuid.NewString()
	seedPromotionRewardService(t, store, inviteeResult.User.ID, serviceID, now.Add(5*time.Minute))
	var waitGroup sync.WaitGroup
	failures := make(chan string, 2)
	for range 2 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			tx, beginErr := store.pool.Begin(ctx)
			if beginErr != nil {
				failures <- beginErr.Error()
				return
			}
			defer rollback(ctx, tx)
			if qualifyErr := qualifyPromotionRewardsForAPIServiceInTx(ctx, tx, serviceID, now.Add(5*time.Minute)); qualifyErr != nil {
				failures <- qualifyErr.Error()
				return
			}
			if commitErr := tx.Commit(ctx); commitErr != nil {
				failures <- commitErr.Error()
			}
		}()
	}
	waitGroup.Wait()
	close(failures)
	for failure := range failures {
		t.Fatalf("concurrent reward qualification failed: %s", failure)
	}

	var relationStatus string
	var relationVersion int64
	var couponCount int
	if err := store.pool.QueryRow(ctx, `SELECT status, version FROM referral_relations WHERE id = $1`, relationID).Scan(&relationStatus, &relationVersion); err != nil || relationStatus != promotionreward.ReferralStatusRewarded {
		t.Fatalf("referral relation not rewarded: status=%q error=%v", relationStatus, err)
	}
	if err := store.pool.QueryRow(ctx, `
		SELECT count(*)::int FROM promotion_coupons
		WHERE user_id IN ($1, $2)
	`, inviterResult.User.ID, inviteeResult.User.ID).Scan(&couponCount); err != nil || couponCount != 3 {
		t.Fatalf("qualification must create welcome and one reward per side: count=%d error=%v", couponCount, err)
	}

	initialCoupons, appErr := store.ListUserPromotionCoupons(ctx, inviteeResult.User.ID, promotionreward.CouponQuery{Page: 1, Limit: 20, Status: promotionreward.CouponStatusAll}, now.Add(6*time.Minute))
	if appErr != nil {
		t.Fatalf("list initial invitee coupons: %v", appErr)
	}
	var welcomeCoupon, inviteeCoupon promotionreward.Coupon
	for _, coupon := range initialCoupons.Items {
		switch coupon.SourceType {
		case promotionreward.CouponSourceWelcome:
			welcomeCoupon = coupon
		case promotionreward.CouponSourceReferralInvitee:
			inviteeCoupon = coupon
		}
	}
	if welcomeCoupon.Status != promotionreward.CouponStatusUsed || welcomeCoupon.ActivationID == "" {
		t.Fatalf("welcome coupon did not auto-activate: %+v", welcomeCoupon)
	}
	if inviteeCoupon.Status != promotionreward.CouponStatusPending {
		t.Fatalf("referral invitee coupon should be pending: %+v", inviteeCoupon)
	}

	publicItems, appErr := store.ListPublicAPIPromotions(ctx, apipromotion.PlacementAPIMarketTop, now.Add(time.Hour))
	if appErr != nil {
		t.Fatalf("list public welcome reward: %v", appErr)
	}
	assertPublicRewardProjection(t, publicItems, welcomeCoupon.ActivationID, welcomeCoupon.ID, serviceID)

	applyNow := now.Add(73 * time.Hour)
	availableCoupons, appErr := store.ListUserPromotionCoupons(ctx, inviteeResult.User.ID, promotionreward.CouponQuery{Page: 1, Limit: 20, Status: promotionreward.CouponStatusAvailable}, applyNow)
	if appErr != nil || len(availableCoupons.Items) != 1 || availableCoupons.Items[0].ID != inviteeCoupon.ID {
		t.Fatalf("delayed coupon did not become available: coupons=%+v error=%v", availableCoupons.Items, appErr)
	}

	idempotencyService := idempotency.NewService(store, func() time.Time { return applyNow })
	rewardService := promotionreward.NewService(store, idempotencyService, func() time.Time { return applyNow })
	adminUser := inviterResult.User
	adminUser.IsAdmin = true
	campaign, appErr := store.GetAdminPromotionRewardCampaign(ctx)
	if appErr != nil {
		t.Fatalf("read campaign before idempotent update: %v", appErr)
	}
	campaignBuilder := func(item promotionreward.Campaign) (idempotency.Completion, *domain.AppError) {
		body, _ := json.Marshal(map[string]any{"id": item.ID, "version": item.Version})
		return idempotency.Completion{Status: http.StatusOK, ContentType: "application/json", Body: body, ResourceType: "promotion_reward_campaign", ResourceID: item.ID}, nil
	}
	campaignInput := promotionreward.UpdateCampaignInput{
		ProgramEnabled: campaign.ProgramEnabled, WelcomeEnabled: campaign.WelcomeEnabled,
		ReferralEnabled: campaign.ReferralEnabled, StartsAt: campaign.StartsAt, EndsAt: campaign.EndsAt,
		PromotionDurationHours: campaign.PromotionDurationHours, CouponValidDays: campaign.CouponValidDays,
		RewardDelayHours: campaign.RewardDelayHours, InviterMonthlyLimit: campaign.InviterMonthlyLimit,
		RulesText: campaign.RulesText, ExpectedVersion: campaign.Version,
		Reason: "idempotent campaign integration", RequestID: "promotion-campaign-update",
	}
	campaignKey := "promotion-campaign-update-" + suffix
	campaignCompletion, appErr := rewardService.UpdateAdminCampaignWithIdempotency(ctx, adminUser, "PATCH /test/promotion-reward-campaign", campaignKey, "campaign-hash", campaignInput, campaignBuilder)
	if appErr != nil || campaignCompletion.ResourceID != campaign.ID {
		t.Fatalf("update campaign idempotently: completion=%+v error=%v", campaignCompletion, appErr)
	}
	campaignReplay, appErr := rewardService.UpdateAdminCampaignWithIdempotency(ctx, adminUser, "PATCH /test/promotion-reward-campaign", campaignKey, "campaign-hash", campaignInput, campaignBuilder)
	if appErr != nil || campaignReplay.ResourceID != campaign.ID || normalizedPromotionRewardJSON(campaignReplay.Body) != normalizedPromotionRewardJSON(campaignCompletion.Body) {
		t.Fatalf("replay campaign update: first=%s replay=%s completion=%+v error=%v", campaignCompletion.Body, campaignReplay.Body, campaignReplay, appErr)
	}
	completionBuilder := func(coupon promotionreward.Coupon) (idempotency.Completion, *domain.AppError) {
		body, _ := json.Marshal(map[string]string{"id": coupon.ID})
		return idempotency.Completion{Status: http.StatusOK, ContentType: "application/json", Body: body, ResourceType: "promotion_coupon", ResourceID: coupon.ID}, nil
	}
	key := "promotion-reward-apply-" + suffix
	completion, appErr := rewardService.ApplyCouponWithIdempotency(ctx, inviteeResult.User, "POST /test/promotion-coupon/apply", key, "request-hash", promotionreward.ApplyCouponInput{
		CouponID: inviteeCoupon.ID, APIServiceID: serviceID, RequestID: "promotion-reward-integration",
	}, completionBuilder)
	if appErr != nil || completion.ResourceID != inviteeCoupon.ID {
		t.Fatalf("apply delayed coupon: completion=%+v error=%v", completion, appErr)
	}
	replayed, appErr := rewardService.ApplyCouponWithIdempotency(ctx, inviteeResult.User, "POST /test/promotion-coupon/apply", key, "request-hash", promotionreward.ApplyCouponInput{
		CouponID: inviteeCoupon.ID, APIServiceID: serviceID,
	}, completionBuilder)
	if appErr != nil || replayed.ResourceID != inviteeCoupon.ID {
		t.Fatalf("replay applied coupon: completion=%+v error=%v", replayed, appErr)
	}

	appliedCoupons, appErr := store.ListUserPromotionCoupons(ctx, inviteeResult.User.ID, promotionreward.CouponQuery{Page: 1, Limit: 20, Status: promotionreward.CouponStatusUsed}, applyNow)
	if appErr != nil {
		t.Fatalf("list used coupons: %v", appErr)
	}
	var appliedCoupon promotionreward.Coupon
	for _, coupon := range appliedCoupons.Items {
		if coupon.ID == inviteeCoupon.ID {
			appliedCoupon = coupon
		}
	}
	if appliedCoupon.ActivationID == "" || appliedCoupon.Version <= inviteeCoupon.Version {
		t.Fatalf("coupon activation facts missing: before=%+v after=%+v", inviteeCoupon, appliedCoupon)
	}
	publicItems, appErr = store.ListPublicAPIPromotions(ctx, apipromotion.PlacementAPIMarketTop, applyNow.Add(time.Minute))
	if appErr != nil {
		t.Fatalf("list public referral reward: %v", appErr)
	}
	assertPublicRewardProjection(t, publicItems, appliedCoupon.ActivationID, appliedCoupon.ID, serviceID)

	operatorStartsAt := applyNow.Add(time.Minute)
	operatorEndsAt := operatorStartsAt.Add(time.Hour)
	availability, appErr := store.GetAPIPromotionAvailability(ctx, apipromotion.AvailabilityInput{
		APIServiceID: serviceID,
		Placement:    apipromotion.PlacementAPIMarketTop,
		StartsAt:     operatorStartsAt,
		EndsAt:       operatorEndsAt,
	}, applyNow)
	if appErr != nil || !availability.SameServiceOverlap {
		t.Fatalf("active reward must block administrator overlap: availability=%+v error=%v", availability, appErr)
	}
	_, appErr = store.CreateAPIPromotion(ctx, apipromotion.CreateInput{
		APIServiceID: serviceID,
		Placement:    apipromotion.PlacementAPIMarketTop,
		StartsAt:     operatorStartsAt,
		EndsAt:       operatorEndsAt,
		Reason:       "reward overlap integration",
		AdminUserID:  adminUser.ID,
		RequestID:    "promotion-operator-overlap",
	}, applyNow)
	if appErr == nil || appErr.Status != http.StatusConflict {
		t.Fatalf("expected administrator overlap conflict, got %v", appErr)
	}

	grantKey := "promotion-reward-grant-" + suffix
	grantInput := promotionreward.GrantCouponInput{
		UserID: inviteeResult.User.ID, DurationHours: 24, ValidDays: 30,
		Reason: "overlap integration", RequestID: "promotion-reward-grant",
	}
	grantCompletion, appErr := rewardService.GrantAdminCouponWithIdempotency(ctx, adminUser, "POST /test/promotion-coupons/grant", grantKey, "grant-hash", grantInput, completionBuilder)
	if appErr != nil || grantCompletion.ResourceID == "" {
		t.Fatalf("grant overlap coupon: completion=%+v error=%v", grantCompletion, appErr)
	}
	grantReplay, appErr := rewardService.GrantAdminCouponWithIdempotency(ctx, adminUser, "POST /test/promotion-coupons/grant", grantKey, "grant-hash", grantInput, completionBuilder)
	if appErr != nil || grantReplay.ResourceID != grantCompletion.ResourceID {
		t.Fatalf("replay coupon grant: completion=%+v error=%v", grantReplay, appErr)
	}
	availableCoupons, appErr = store.ListUserPromotionCoupons(ctx, inviteeResult.User.ID, promotionreward.CouponQuery{Page: 1, Limit: 20, Status: promotionreward.CouponStatusAvailable}, applyNow)
	if appErr != nil || len(availableCoupons.Items) != 1 {
		t.Fatalf("list granted coupon: coupons=%+v error=%v", availableCoupons.Items, appErr)
	}
	_, appErr = rewardService.ApplyCouponWithIdempotency(ctx, inviteeResult.User, "POST /test/promotion-coupon/apply", "promotion-overlap-"+suffix, "overlap-hash", promotionreward.ApplyCouponInput{
		CouponID: availableCoupons.Items[0].ID, APIServiceID: serviceID,
	}, completionBuilder)
	if appErr == nil || appErr.Status != http.StatusConflict {
		t.Fatalf("expected overlapping promotion conflict, got %v", appErr)
	}

	revokeCouponInput := promotionreward.RevokeCouponInput{
		CouponID:        appliedCoupon.ID,
		ExpectedVersion: appliedCoupon.Version, Reason: "revoke integration", RequestID: "promotion-reward-revoke",
	}
	revokeCouponKey := "promotion-reward-revoke-" + suffix
	revokeCompletion, appErr := rewardService.RevokeAdminCouponWithIdempotency(ctx, adminUser, "POST /test/promotion-coupons/revoke", revokeCouponKey, "revoke-hash", revokeCouponInput, completionBuilder)
	if appErr != nil || revokeCompletion.ResourceID != appliedCoupon.ID {
		t.Fatalf("revoke active reward: completion=%+v error=%v", revokeCompletion, appErr)
	}
	revokeReplay, appErr := rewardService.RevokeAdminCouponWithIdempotency(ctx, adminUser, "POST /test/promotion-coupons/revoke", revokeCouponKey, "revoke-hash", revokeCouponInput, completionBuilder)
	if appErr != nil || revokeReplay.ResourceID != appliedCoupon.ID {
		t.Fatalf("replay active reward revoke: completion=%+v error=%v", revokeReplay, appErr)
	}
	publicItems, appErr = store.ListPublicAPIPromotions(ctx, apipromotion.PlacementAPIMarketTop, applyNow.Add(time.Hour+time.Minute))
	if appErr != nil {
		t.Fatalf("list public rewards after revoke: %v", appErr)
	}
	for _, item := range publicItems {
		if item.Kind == apipromotion.KindReward && item.ID == appliedCoupon.ActivationID {
			t.Fatalf("revoked reward remained public: %+v", item)
		}
	}
	referralBuilder := func(item promotionreward.ReferralRecord) (idempotency.Completion, *domain.AppError) {
		body, _ := json.Marshal(map[string]any{"id": item.ID, "version": item.Version})
		return idempotency.Completion{Status: http.StatusOK, ContentType: "application/json", Body: body, ResourceType: "referral_relation", ResourceID: item.ID}, nil
	}
	revokeReferralInput := promotionreward.RevokeReferralInput{
		ReferralID: relationID, ExpectedVersion: relationVersion,
		Reason: "revoke referral integration", RequestID: "promotion-referral-revoke",
	}
	revokeReferralKey := "promotion-referral-revoke-" + suffix
	referralCompletion, appErr := rewardService.RevokeAdminReferralWithIdempotency(ctx, adminUser, "POST /test/referrals/revoke", revokeReferralKey, "referral-revoke-hash", revokeReferralInput, referralBuilder)
	if appErr != nil || referralCompletion.ResourceID != relationID {
		t.Fatalf("revoke referral idempotently: completion=%+v error=%v", referralCompletion, appErr)
	}
	referralReplay, appErr := rewardService.RevokeAdminReferralWithIdempotency(ctx, adminUser, "POST /test/referrals/revoke", revokeReferralKey, "referral-revoke-hash", revokeReferralInput, referralBuilder)
	if appErr != nil || referralReplay.ResourceID != relationID {
		t.Fatalf("replay referral revoke: completion=%+v error=%v", referralReplay, appErr)
	}
}

func seedPromotionRewardService(t *testing.T, store *Store, ownerUserID, serviceID string, now time.Time) {
	t.Helper()
	ctx := context.Background()
	contactID := uuid.NewString()
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO contact_methods (id, user_id, type, label, is_default, enabled, created_at, updated_at)
		VALUES ($1, $2, 'linuxdo', 'linux.do', true, true, $3, $3)
	`, contactID, ownerUserID, now); err != nil {
		t.Fatalf("seed promotion contact: %v", err)
	}
	seedContactVersionForTest(t, ctx, store.pool, contactID, ownerUserID, now)
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO api_services (
			id, owner_user_id, merchant_identity_mode, owner_contact_method_id,
			title, short_description, distribution_system, billing_mode,
			minimum_intent_cny, maximum_intent_cny, usage_visibility,
			review_status, publication_status, moderation_status,
			accepting_orders, payment_window_minutes,
			declared_ttft_band, declared_max_concurrency, performance_confirmed_at,
			approved_at, created_at, updated_at, version
		) VALUES (
			$1, $2, 'public_profile', $3, '推广权益测试服务', 'PostgreSQL integration service',
			'sub2api', 'manual_usage_check', 1, 1000, 'none',
			'approved', 'online', 'clear', true, 10,
			'under_1s', 20, $4, $4, $4, $4, 1
		)
	`, serviceID, ownerUserID, contactID, now); err != nil {
		t.Fatalf("seed promotion API service: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO api_service_access_modes (api_service_id, access_mode, public_note)
		VALUES ($1, 'buyer_dedicated_sub_key', '买家专属子 Key')
	`, serviceID); err != nil {
		t.Fatalf("seed promotion access mode: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO api_service_payment_options (
			id, api_service_id, payment_method, enabled, payment_instructions,
			created_at, updated_at, version
		) VALUES ($1, $2, 'wechat', true, '站外确认', $3, $3, 1)
	`, uuid.NewString(), serviceID, now); err != nil {
		t.Fatalf("seed promotion payment option: %v", err)
	}
}

func assertPublicRewardProjection(t *testing.T, items []apipromotion.Promotion, activationID, couponID, serviceID string) {
	t.Helper()
	for _, item := range items {
		if item.Kind != apipromotion.KindReward || item.APIServiceID != serviceID {
			continue
		}
		if item.ID != activationID || item.ID == couponID || item.CreatedByAdminID != "" {
			t.Fatalf("public reward leaked internal identity: %+v", item)
		}
		return
	}
	t.Fatalf("public reward %s for service %s not found: %+v", activationID, serviceID, items)
}

func normalizedPromotionRewardJSON(body []byte) string {
	var value any
	if err := json.Unmarshal(body, &value); err != nil {
		return string(body)
	}
	normalized, err := json.Marshal(value)
	if err != nil {
		return string(body)
	}
	return string(normalized)
}

func cleanupPromotionRewardIntegrationFixture(t *testing.T, store *Store, userIDs []string) {
	t.Helper()
	if len(userIDs) == 0 {
		return
	}
	ctx := context.Background()
	statements := []string{
		`DELETE FROM idempotency_keys WHERE user_id = ANY($1::uuid[])`,
		`DELETE FROM notifications WHERE user_id = ANY($1::uuid[])`,
		`DELETE FROM domain_events WHERE actor_user_id = ANY($1::uuid[]) OR aggregate_id IN (SELECT id FROM promotion_coupons WHERE user_id = ANY($1::uuid[]))`,
		`DELETE FROM admin_audit_logs WHERE admin_user_id = ANY($1::uuid[]) OR target_id IN (SELECT id FROM promotion_coupons WHERE user_id = ANY($1::uuid[]))`,
		`DELETE FROM promotion_coupons WHERE user_id = ANY($1::uuid[])`,
		`DELETE FROM referral_relations WHERE inviter_user_id = ANY($1::uuid[]) OR invitee_user_id = ANY($1::uuid[])`,
		`DELETE FROM referral_codes WHERE user_id = ANY($1::uuid[])`,
		`DELETE FROM api_service_payment_options WHERE api_service_id IN (SELECT id FROM api_services WHERE owner_user_id = ANY($1::uuid[]))`,
		`DELETE FROM api_service_access_modes WHERE api_service_id IN (SELECT id FROM api_services WHERE owner_user_id = ANY($1::uuid[]))`,
		`DELETE FROM api_services WHERE owner_user_id = ANY($1::uuid[])`,
		`UPDATE contact_methods SET current_version_id = NULL WHERE user_id = ANY($1::uuid[])`,
		`DELETE FROM contact_method_versions WHERE owner_user_id = ANY($1::uuid[])`,
		`DELETE FROM contact_methods WHERE user_id = ANY($1::uuid[])`,
		`DELETE FROM linux_do_bindings WHERE user_id = ANY($1::uuid[])`,
		`DELETE FROM auth_identities WHERE user_id = ANY($1::uuid[])`,
		`DELETE FROM users WHERE id = ANY($1::uuid[])`,
	}
	for _, statement := range statements {
		if _, err := store.pool.Exec(ctx, statement, userIDs); err != nil {
			t.Errorf("cleanup promotion reward fixture: %v", err)
		}
	}
}

func restorePromotionRewardCampaign(t *testing.T, store *Store, campaign promotionreward.Campaign) {
	t.Helper()
	if _, err := store.pool.Exec(context.Background(), `
		UPDATE promotion_reward_campaigns
		SET program_enabled = $2, welcome_enabled = $3, referral_enabled = $4,
		    starts_at = $5, ends_at = $6, promotion_duration_hours = $7,
		    coupon_valid_days = $8, reward_delay_hours = $9,
		    inviter_monthly_limit = $10, rules_text = $11,
		    created_by_admin_id = NULLIF($12, '')::uuid,
		    updated_by_admin_id = NULLIF($13, '')::uuid,
		    created_at = $14, updated_at = $15, version = $16
		WHERE id = $1
	`, campaign.ID, campaign.ProgramEnabled, campaign.WelcomeEnabled, campaign.ReferralEnabled,
		campaign.StartsAt, campaign.EndsAt, campaign.PromotionDurationHours, campaign.CouponValidDays,
		campaign.RewardDelayHours, campaign.InviterMonthlyLimit, campaign.RulesText,
		campaign.CreatedByAdminID, campaign.UpdatedByAdminID, campaign.CreatedAt, campaign.UpdatedAt, campaign.Version); err != nil {
		t.Errorf("restore promotion reward campaign: %v", err)
	}
}
