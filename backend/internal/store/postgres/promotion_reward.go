package postgres

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/promotionreward"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const referralCodeAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

const promotionRewardCampaignColumns = `
	id::text, code, program_enabled, welcome_enabled, referral_enabled,
	starts_at, ends_at, promotion_duration_hours, coupon_valid_days,
	reward_delay_hours, inviter_monthly_limit, rules_text,
	COALESCE(created_by_admin_id::text, ''), COALESCE(updated_by_admin_id::text, ''),
	created_at, updated_at, version
`

const promotionCouponColumns = `
	coupon.id::text, COALESCE(coupon.campaign_id::text, ''), coupon.user_id::text,
	COALESCE(owner.display_name, owner.username, ''), coupon.source_type,
	COALESCE(coupon.source_id::text, ''), coupon.status,
	coupon.available_at, coupon.expires_at, coupon.duration_hours,
	COALESCE(coupon.used_api_service_id::text, ''), COALESCE(service.title, ''),
	COALESCE(coupon.activation_id::text, ''), coupon.promotion_starts_at,
	coupon.promotion_ends_at, coupon.used_at, coupon.revoked_at,
	coupon.revoked_reason, COALESCE(coupon.revoked_by_admin_id::text, ''),
	COALESCE(coupon.created_by_admin_id::text, ''), coupon.grant_reason,
	coupon.created_at, coupon.updated_at, coupon.version
`

func (s *Store) GetPromotionRewardPublicConfig(ctx context.Context, now time.Time) (promotionreward.PublicConfig, *domain.AppError) {
	campaign, err := scanPromotionRewardCampaign(s.pool.QueryRow(ctx, `
		SELECT `+promotionRewardCampaignColumns+`
		FROM promotion_reward_campaigns
		WHERE code = $1
	`, promotionreward.CampaignCodeAPIServiceReferralV1))
	if errors.Is(err, pgx.ErrNoRows) {
		return promotionreward.PublicConfig{}, nil
	}
	if err != nil {
		return promotionreward.PublicConfig{}, internalStoreError()
	}
	return publicPromotionRewardConfig(campaign, now), nil
}

func (s *Store) GetReferralSummary(ctx context.Context, userID string, now time.Time) (promotionreward.ReferralSummary, *domain.AppError) {
	if s == nil || s.pool == nil {
		return promotionreward.ReferralSummary{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return promotionreward.ReferralSummary{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	campaign, err := scanPromotionRewardCampaign(tx.QueryRow(ctx, `
		SELECT `+promotionRewardCampaignColumns+`
		FROM promotion_reward_campaigns
		WHERE code = $1
		FOR UPDATE
	`, promotionreward.CampaignCodeAPIServiceReferralV1))
	if errors.Is(err, pgx.ErrNoRows) {
		return promotionreward.ReferralSummary{}, promotionRewardFeatureDisabledError()
	}
	if err != nil {
		return promotionreward.ReferralSummary{}, internalStoreError()
	}
	config := publicPromotionRewardConfig(campaign, now)
	if !config.ProgramEnabled || !config.ReferralEnabled {
		return promotionreward.ReferralSummary{Campaign: config}, nil
	}
	code, appErr := ensureReferralCodeInTx(ctx, tx, campaign.ID, userID, now)
	if appErr != nil {
		return promotionreward.ReferralSummary{}, appErr
	}
	statistics, appErr := referralStatisticsInTx(ctx, tx, campaign, userID, now)
	if appErr != nil {
		return promotionreward.ReferralSummary{}, appErr
	}
	rows, err := tx.Query(ctx, `
		SELECT relation.id::text, relation.inviter_user_id::text, relation.invitee_user_id::text,
		       COALESCE(inviter.display_name, inviter.username, ''),
		       COALESCE(invitee.display_name, invitee.username, ''),
		       relation.status, relation.bound_at, relation.qualified_at, relation.rewarded_at,
		       COALESCE(relation.qualified_api_service_id::text, ''), relation.rejected_at,
		       relation.rejected_reason, relation.revoked_at, relation.revoked_reason,
		       relation.risk_flags, relation.created_at, relation.updated_at, relation.version
		FROM referral_relations relation
		JOIN users inviter ON inviter.id = relation.inviter_user_id
		JOIN users invitee ON invitee.id = relation.invitee_user_id
		WHERE relation.inviter_user_id = $1
		ORDER BY relation.bound_at DESC, relation.id DESC
		LIMIT 100
	`, userID)
	if err != nil {
		return promotionreward.ReferralSummary{}, internalStoreError()
	}
	defer rows.Close()
	records := make([]promotionreward.ReferralRecord, 0)
	for rows.Next() {
		record, scanErr := scanReferralRecord(rows)
		if scanErr != nil {
			return promotionreward.ReferralSummary{}, internalStoreError()
		}
		record.InviterDisplayName = maskDisplayName(record.InviterDisplayName)
		record.InviteeDisplayName = maskDisplayName(record.InviteeDisplayName)
		record.RiskFlags = nil
		records = append(records, record)
	}
	if rows.Err() != nil {
		return promotionreward.ReferralSummary{}, internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return promotionreward.ReferralSummary{}, internalStoreError()
	}
	return promotionreward.ReferralSummary{Code: code, Statistics: statistics, Records: records, Campaign: config}, nil
}

func (s *Store) ListUserPromotionCoupons(ctx context.Context, userID string, query promotionreward.CouponQuery, now time.Time) (promotionreward.CouponPage, *domain.AppError) {
	return s.listPromotionCoupons(ctx, `coupon.user_id = $1`, []any{userID}, query, now)
}

func (s *Store) ApplyPromotionCouponWithIdempotency(ctx context.Context, entry idempotency.Entry, input promotionreward.ApplyCouponInput, now time.Time, buildCompletion promotionreward.CouponCompletionBuilder) (promotionreward.Coupon, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return promotionreward.Coupon{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return promotionreward.Coupon{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return promotionreward.Coupon{}, idempotency.Completion{}, appErr
	}
	coupon, err := scanPromotionCoupon(tx.QueryRow(ctx, `
		SELECT `+promotionCouponColumns+`
		FROM promotion_coupons coupon
		JOIN users owner ON owner.id = coupon.user_id
		LEFT JOIN api_services service ON service.id = coupon.used_api_service_id
		WHERE coupon.id = $1
		FOR UPDATE OF coupon
	`, input.CouponID))
	if errors.Is(err, pgx.ErrNoRows) || coupon.UserID != input.UserID {
		return promotionreward.Coupon{}, idempotency.Completion{}, promotionCouponNotFoundError()
	}
	if err != nil {
		return promotionreward.Coupon{}, idempotency.Completion{}, internalStoreError()
	}
	coupon.Status = promotionreward.EffectiveCouponStatus(coupon, now)
	if coupon.Status != promotionreward.CouponStatusAvailable {
		return promotionreward.Coupon{}, idempotency.Completion{}, promotionCouponUnavailableError(coupon.Status)
	}
	var campaignActive bool
	err = tx.QueryRow(ctx, `
		SELECT program_enabled AND starts_at <= $1 AND (ends_at IS NULL OR ends_at > $1)
		FROM promotion_reward_campaigns
		WHERE code = $2
		FOR UPDATE
	`, now, promotionreward.CampaignCodeAPIServiceReferralV1).Scan(&campaignActive)
	if errors.Is(err, pgx.ErrNoRows) || !campaignActive {
		return promotionreward.Coupon{}, idempotency.Completion{}, promotionRewardFeatureDisabledError()
	}
	if err != nil {
		return promotionreward.Coupon{}, idempotency.Completion{}, internalStoreError()
	}
	var targetOwnerID string
	err = tx.QueryRow(ctx, `
		SELECT service.owner_user_id::text
		FROM api_services service
		WHERE service.id = $1
		  AND `+publicAPIServiceOrderablePredicateAt("service", "$2")+`
		FOR UPDATE OF service
	`, input.APIServiceID, now).Scan(&targetOwnerID)
	if errors.Is(err, pgx.ErrNoRows) || targetOwnerID != input.UserID {
		return promotionreward.Coupon{}, idempotency.Completion{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "API service not eligible", "只能推广自己已审核通过且可正常接单的 API 服务。", "apiServiceId", "not_eligible", "只能选择自己当前可接单的 API 服务。")
	}
	if err != nil {
		return promotionreward.Coupon{}, idempotency.Completion{}, internalStoreError()
	}
	endsAt := now.Add(time.Duration(coupon.DurationHours) * time.Hour)
	overlap, appErr := hasAPIServicePromotionOverlapInTx(ctx, tx, input.APIServiceID, now, endsAt, "")
	if appErr != nil {
		return promotionreward.Coupon{}, idempotency.Completion{}, appErr
	}
	if overlap {
		return promotionreward.Coupon{}, idempotency.Completion{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Promotion overlaps", "该 API 服务在所选时段已有推广，请在现有推广结束后再使用。")
	}
	activationID := uuid.NewString()
	err = tx.QueryRow(ctx, `
		UPDATE promotion_coupons
		SET status = 'used', used_api_service_id = $2, activation_id = $3,
		    promotion_starts_at = $4, promotion_ends_at = $5, used_at = $4,
		    updated_at = $4, version = version + 1
		WHERE id = $1
		RETURNING version
	`, coupon.ID, input.APIServiceID, activationID, now, endsAt).Scan(&coupon.Version)
	if err != nil {
		return promotionreward.Coupon{}, idempotency.Completion{}, internalStoreError()
	}
	coupon.StoredStatus = promotionreward.CouponStatusUsed
	coupon.Status = promotionreward.CouponStatusUsed
	coupon.UsedAPIServiceID = input.APIServiceID
	coupon.ActivationID = activationID
	coupon.PromotionStartsAt = &now
	coupon.PromotionEndsAt = &endsAt
	coupon.UsedAt = &now
	coupon.UpdatedAt = now
	if appErr := insertPromotionRewardEventAndNotification(ctx, tx, coupon.UserID, "promotion_coupon", coupon.ID, "promotion_coupon.used", "user", coupon.UserID, coupon.Version, input.RequestID, "推广券已使用", "你的 API 服务已进入推广轮换池。", "/my/promotion-benefits", now); appErr != nil {
		return promotionreward.Coupon{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(coupon)
	if appErr != nil {
		return promotionreward.Coupon{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return promotionreward.Coupon{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return promotionreward.Coupon{}, idempotency.Completion{}, internalStoreError()
	}
	return coupon, completion, nil
}

func (s *Store) GetAdminPromotionRewardCampaign(ctx context.Context) (promotionreward.Campaign, *domain.AppError) {
	campaign, err := scanPromotionRewardCampaign(s.pool.QueryRow(ctx, `
		SELECT `+promotionRewardCampaignColumns+`
		FROM promotion_reward_campaigns
		WHERE code = $1
	`, promotionreward.CampaignCodeAPIServiceReferralV1))
	if errors.Is(err, pgx.ErrNoRows) {
		return promotionreward.Campaign{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Campaign not found", "推广活动不存在。")
	}
	if err != nil {
		return promotionreward.Campaign{}, internalStoreError()
	}
	return campaign, nil
}

func (s *Store) UpdateAdminPromotionRewardCampaign(ctx context.Context, input promotionreward.UpdateCampaignInput, now time.Time) (promotionreward.Campaign, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return promotionreward.Campaign{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	after, appErr := updateAdminPromotionRewardCampaignInTx(ctx, tx, input, now)
	if appErr != nil {
		return promotionreward.Campaign{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return promotionreward.Campaign{}, internalStoreError()
	}
	return after, nil
}

func (s *Store) UpdateAdminPromotionRewardCampaignWithIdempotency(ctx context.Context, entry idempotency.Entry, input promotionreward.UpdateCampaignInput, now time.Time, buildCompletion promotionreward.CampaignCompletionBuilder) (promotionreward.Campaign, idempotency.Completion, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return promotionreward.Campaign{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return promotionreward.Campaign{}, idempotency.Completion{}, appErr
	}
	after, appErr := updateAdminPromotionRewardCampaignInTx(ctx, tx, input, now)
	if appErr != nil {
		return promotionreward.Campaign{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(after)
	if appErr != nil {
		return promotionreward.Campaign{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return promotionreward.Campaign{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return promotionreward.Campaign{}, idempotency.Completion{}, internalStoreError()
	}
	return after, completion, nil
}

func updateAdminPromotionRewardCampaignInTx(ctx context.Context, tx pgx.Tx, input promotionreward.UpdateCampaignInput, now time.Time) (promotionreward.Campaign, *domain.AppError) {
	before, err := scanPromotionRewardCampaign(tx.QueryRow(ctx, `
		SELECT `+promotionRewardCampaignColumns+`
		FROM promotion_reward_campaigns
		WHERE code = $1
		FOR UPDATE
	`, promotionreward.CampaignCodeAPIServiceReferralV1))
	if errors.Is(err, pgx.ErrNoRows) {
		return promotionreward.Campaign{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Campaign not found", "推广活动不存在。")
	}
	if err != nil {
		return promotionreward.Campaign{}, internalStoreError()
	}
	if before.Version != input.ExpectedVersion {
		return promotionreward.Campaign{}, promotionRewardVersionConflictError()
	}
	after, err := scanPromotionRewardCampaign(tx.QueryRow(ctx, `
		UPDATE promotion_reward_campaigns
		SET program_enabled = $2, welcome_enabled = $3, referral_enabled = $4,
		    starts_at = $5, ends_at = $6, promotion_duration_hours = $7,
		    coupon_valid_days = $8, reward_delay_hours = $9,
		    inviter_monthly_limit = $10, rules_text = $11,
		    updated_by_admin_id = $12, updated_at = $13, version = version + 1
		WHERE id = $1
		RETURNING `+promotionRewardCampaignColumns+`
	`, before.ID, input.ProgramEnabled, input.WelcomeEnabled, input.ReferralEnabled,
		input.StartsAt, input.EndsAt, input.PromotionDurationHours, input.CouponValidDays,
		input.RewardDelayHours, input.InviterMonthlyLimit, input.RulesText,
		input.AdminUserID, now))
	if err != nil {
		return promotionreward.Campaign{}, internalStoreError()
	}
	if appErr := insertPromotionRewardAudit(ctx, tx, input.AdminUserID, "promotion_reward.campaign_updated", "promotion_reward_campaign", after.ID, input.Reason, campaignAuditSnapshot(before), campaignAuditSnapshot(after), input.RequestID, now); appErr != nil {
		return promotionreward.Campaign{}, appErr
	}
	return after, nil
}

func (s *Store) ListAdminReferrals(ctx context.Context, query promotionreward.ReferralQuery) (promotionreward.ReferralPage, *domain.AppError) {
	where := "TRUE"
	args := []any{}
	if query.Status != promotionreward.CouponStatusAll {
		args = append(args, query.Status)
		where += fmt.Sprintf(" AND relation.status = $%d", len(args))
	}
	if query.Search != "" {
		args = append(args, "%"+strings.ToLower(query.Search)+"%")
		where += fmt.Sprintf(" AND (lower(inviter.username) LIKE $%[1]d OR lower(inviter.display_name) LIKE $%[1]d OR lower(invitee.username) LIKE $%[1]d OR lower(invitee.display_name) LIKE $%[1]d)", len(args))
	}
	var total int
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*)::int
		FROM referral_relations relation
		JOIN users inviter ON inviter.id = relation.inviter_user_id
		JOIN users invitee ON invitee.id = relation.invitee_user_id
		WHERE `+where, args...).Scan(&total); err != nil {
		return promotionreward.ReferralPage{}, internalStoreError()
	}
	page, limit := normalizePromotionRewardPage(query.Page, query.Limit)
	args = append(args, limit, (page-1)*limit)
	rows, err := s.pool.Query(ctx, `
		SELECT relation.id::text, relation.inviter_user_id::text, relation.invitee_user_id::text,
		       COALESCE(inviter.display_name, inviter.username, ''), COALESCE(invitee.display_name, invitee.username, ''),
		       relation.status, relation.bound_at, relation.qualified_at, relation.rewarded_at,
		       COALESCE(relation.qualified_api_service_id::text, ''), relation.rejected_at,
		       relation.rejected_reason, relation.revoked_at, relation.revoked_reason,
		       relation.risk_flags, relation.created_at, relation.updated_at, relation.version
		FROM referral_relations relation
		JOIN users inviter ON inviter.id = relation.inviter_user_id
		JOIN users invitee ON invitee.id = relation.invitee_user_id
		WHERE `+where+`
		ORDER BY relation.bound_at DESC, relation.id DESC
		LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return promotionreward.ReferralPage{}, internalStoreError()
	}
	defer rows.Close()
	items := make([]promotionreward.ReferralRecord, 0)
	for rows.Next() {
		item, scanErr := scanReferralRecord(rows)
		if scanErr != nil {
			return promotionreward.ReferralPage{}, internalStoreError()
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		return promotionreward.ReferralPage{}, internalStoreError()
	}
	return promotionreward.ReferralPage{Items: items, Pagination: promotionRewardPagination(page, limit, total)}, nil
}

func (s *Store) RevokeAdminReferral(ctx context.Context, input promotionreward.RevokeReferralInput, now time.Time) (promotionreward.ReferralRecord, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return promotionreward.ReferralRecord{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	after, appErr := revokeAdminReferralInTx(ctx, tx, input, now)
	if appErr != nil {
		return promotionreward.ReferralRecord{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return promotionreward.ReferralRecord{}, internalStoreError()
	}
	return after, nil
}

func (s *Store) RevokeAdminReferralWithIdempotency(ctx context.Context, entry idempotency.Entry, input promotionreward.RevokeReferralInput, now time.Time, buildCompletion promotionreward.ReferralCompletionBuilder) (promotionreward.ReferralRecord, idempotency.Completion, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return promotionreward.ReferralRecord{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return promotionreward.ReferralRecord{}, idempotency.Completion{}, appErr
	}
	after, appErr := revokeAdminReferralInTx(ctx, tx, input, now)
	if appErr != nil {
		return promotionreward.ReferralRecord{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(after)
	if appErr != nil {
		return promotionreward.ReferralRecord{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return promotionreward.ReferralRecord{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return promotionreward.ReferralRecord{}, idempotency.Completion{}, internalStoreError()
	}
	return after, completion, nil
}

func revokeAdminReferralInTx(ctx context.Context, tx pgx.Tx, input promotionreward.RevokeReferralInput, now time.Time) (promotionreward.ReferralRecord, *domain.AppError) {
	before, err := getReferralRecordInTx(ctx, tx, input.ReferralID, true)
	if errors.Is(err, pgx.ErrNoRows) {
		return promotionreward.ReferralRecord{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Referral not found", "邀请关系不存在。")
	}
	if err != nil {
		return promotionreward.ReferralRecord{}, internalStoreError()
	}
	if before.Version != input.ExpectedVersion {
		return promotionreward.ReferralRecord{}, promotionRewardVersionConflictError()
	}
	if before.Status == promotionreward.ReferralStatusRevoked {
		return promotionreward.ReferralRecord{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Referral already revoked", "邀请关系已撤销。")
	}
	_, err = tx.Exec(ctx, `
		UPDATE referral_relations
		SET status = 'revoked', revoked_at = $2, revoked_reason = $3,
		    updated_at = $2, version = version + 1
		WHERE id = $1
	`, input.ReferralID, now, input.Reason)
	if err != nil {
		return promotionreward.ReferralRecord{}, internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		UPDATE promotion_coupons
		SET status = 'revoked', revoked_at = $2, revoked_reason = $3,
		    revoked_by_admin_id = $4,
		    promotion_ends_at = CASE
		      WHEN promotion_starts_at IS NOT NULL AND promotion_ends_at > $2 THEN $2
		      ELSE promotion_ends_at
		    END,
		    updated_at = $2, version = version + 1
		WHERE source_id = $1
		  AND source_type IN ('referral_inviter', 'referral_invitee')
		  AND status <> 'revoked'
	`, input.ReferralID, now, input.Reason, input.AdminUserID)
	if err != nil {
		return promotionreward.ReferralRecord{}, internalStoreError()
	}
	after, err := getReferralRecordInTx(ctx, tx, input.ReferralID, false)
	if err != nil {
		return promotionreward.ReferralRecord{}, internalStoreError()
	}
	if appErr := insertPromotionRewardAudit(ctx, tx, input.AdminUserID, "referral.revoked", "referral_relation", after.ID, input.Reason, referralAuditSnapshot(before), referralAuditSnapshot(after), input.RequestID, now); appErr != nil {
		return promotionreward.ReferralRecord{}, appErr
	}
	if appErr := insertPromotionRewardEventAndNotification(ctx, tx, after.InviterUserID, "referral_relation", after.ID, "referral.revoked", "admin", input.AdminUserID, after.Version, input.RequestID, "邀请奖励已撤销", "管理员已撤销一条邀请关系及其相关未生效权益。", "/my/promotion-benefits", now); appErr != nil {
		return promotionreward.ReferralRecord{}, appErr
	}
	return after, nil
}

func (s *Store) ListAdminPromotionCoupons(ctx context.Context, query promotionreward.CouponQuery, now time.Time) (promotionreward.CouponPage, *domain.AppError) {
	return s.listPromotionCoupons(ctx, "TRUE", nil, query, now)
}

func (s *Store) GrantAdminPromotionCoupon(ctx context.Context, input promotionreward.GrantCouponInput, now time.Time) (promotionreward.Coupon, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return promotionreward.Coupon{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	coupon, appErr := grantAdminPromotionCouponInTx(ctx, tx, input, now)
	if appErr != nil {
		return promotionreward.Coupon{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return promotionreward.Coupon{}, internalStoreError()
	}
	return coupon, nil
}

func (s *Store) GrantAdminPromotionCouponWithIdempotency(ctx context.Context, entry idempotency.Entry, input promotionreward.GrantCouponInput, now time.Time, buildCompletion promotionreward.CouponCompletionBuilder) (promotionreward.Coupon, idempotency.Completion, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return promotionreward.Coupon{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return promotionreward.Coupon{}, idempotency.Completion{}, appErr
	}
	coupon, appErr := grantAdminPromotionCouponInTx(ctx, tx, input, now)
	if appErr != nil {
		return promotionreward.Coupon{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(coupon)
	if appErr != nil {
		return promotionreward.Coupon{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return promotionreward.Coupon{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return promotionreward.Coupon{}, idempotency.Completion{}, internalStoreError()
	}
	return coupon, completion, nil
}

func grantAdminPromotionCouponInTx(ctx context.Context, tx pgx.Tx, input promotionreward.GrantCouponInput, now time.Time) (promotionreward.Coupon, *domain.AppError) {
	var accountStatus string
	if err := tx.QueryRow(ctx, `SELECT account_status FROM users WHERE id = $1 FOR UPDATE`, input.UserID).Scan(&accountStatus); errors.Is(err, pgx.ErrNoRows) {
		return promotionreward.Coupon{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "User not found", "用户不存在。")
	} else if err != nil {
		return promotionreward.Coupon{}, internalStoreError()
	}
	if accountStatus != "active" {
		return promotionreward.Coupon{}, domain.NewError(http.StatusConflict, domain.CodeAccountRestricted, "Account restricted", "当前账号状态不能领取推广权益。")
	}
	expiresAt := now.AddDate(0, 0, input.ValidDays)
	coupon, err := scanPromotionCoupon(tx.QueryRow(ctx, `
		INSERT INTO promotion_coupons (
		  user_id, source_type, status, available_at, expires_at, duration_hours,
		  created_by_admin_id, grant_reason, created_at, updated_at
		)
		VALUES ($1, 'admin_grant', 'available', $2, $3, $4, $5, $6, $2, $2)
		RETURNING id::text, '', user_id::text,
		  (SELECT COALESCE(display_name, username, '') FROM users WHERE id = user_id),
		  source_type, '', status, available_at, expires_at, duration_hours,
		  '', '', '', promotion_starts_at, promotion_ends_at, used_at, revoked_at,
		  revoked_reason, '', created_by_admin_id::text, grant_reason,
		  created_at, updated_at, version
	`, input.UserID, now, expiresAt, input.DurationHours, input.AdminUserID, input.Reason))
	if err != nil {
		return promotionreward.Coupon{}, internalStoreError()
	}
	coupon.Status = promotionreward.CouponStatusAvailable
	if appErr := insertPromotionRewardAudit(ctx, tx, input.AdminUserID, "promotion_coupon.granted", "promotion_coupon", coupon.ID, input.Reason, nil, couponAuditSnapshot(coupon), input.RequestID, now); appErr != nil {
		return promotionreward.Coupon{}, appErr
	}
	if appErr := insertPromotionRewardEventAndNotification(ctx, tx, coupon.UserID, "promotion_coupon", coupon.ID, "promotion_coupon.created", "admin", input.AdminUserID, coupon.Version, input.RequestID, "获得推广券", "管理员已向你发放一张 API 服务推广券。", "/my/promotion-benefits", now); appErr != nil {
		return promotionreward.Coupon{}, appErr
	}
	return coupon, nil
}

func (s *Store) RevokeAdminPromotionCoupon(ctx context.Context, input promotionreward.RevokeCouponInput, now time.Time) (promotionreward.Coupon, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return promotionreward.Coupon{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	after, appErr := revokeAdminPromotionCouponInTx(ctx, tx, input, now)
	if appErr != nil {
		return promotionreward.Coupon{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return promotionreward.Coupon{}, internalStoreError()
	}
	return after, nil
}

func (s *Store) RevokeAdminPromotionCouponWithIdempotency(ctx context.Context, entry idempotency.Entry, input promotionreward.RevokeCouponInput, now time.Time, buildCompletion promotionreward.CouponCompletionBuilder) (promotionreward.Coupon, idempotency.Completion, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return promotionreward.Coupon{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return promotionreward.Coupon{}, idempotency.Completion{}, appErr
	}
	after, appErr := revokeAdminPromotionCouponInTx(ctx, tx, input, now)
	if appErr != nil {
		return promotionreward.Coupon{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(after)
	if appErr != nil {
		return promotionreward.Coupon{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return promotionreward.Coupon{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return promotionreward.Coupon{}, idempotency.Completion{}, internalStoreError()
	}
	return after, completion, nil
}

func revokeAdminPromotionCouponInTx(ctx context.Context, tx pgx.Tx, input promotionreward.RevokeCouponInput, now time.Time) (promotionreward.Coupon, *domain.AppError) {
	before, err := scanPromotionCoupon(tx.QueryRow(ctx, `
		SELECT `+promotionCouponColumns+`
		FROM promotion_coupons coupon
		JOIN users owner ON owner.id = coupon.user_id
		LEFT JOIN api_services service ON service.id = coupon.used_api_service_id
		WHERE coupon.id = $1
		FOR UPDATE OF coupon
	`, input.CouponID))
	if errors.Is(err, pgx.ErrNoRows) {
		return promotionreward.Coupon{}, promotionCouponNotFoundError()
	}
	if err != nil {
		return promotionreward.Coupon{}, internalStoreError()
	}
	if before.Version != input.ExpectedVersion {
		return promotionreward.Coupon{}, promotionRewardVersionConflictError()
	}
	if before.StoredStatus == promotionreward.CouponStatusRevoked {
		return promotionreward.Coupon{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Coupon already revoked", "推广券已撤销。")
	}
	_, err = tx.Exec(ctx, `
		UPDATE promotion_coupons
		SET status = 'revoked', revoked_at = $2, revoked_reason = $3,
		    revoked_by_admin_id = $4,
		    promotion_ends_at = CASE
		      WHEN promotion_starts_at IS NOT NULL AND promotion_ends_at > $2 THEN $2
		      ELSE promotion_ends_at
		    END,
		    updated_at = $2, version = version + 1
		WHERE id = $1
	`, input.CouponID, now, input.Reason, input.AdminUserID)
	if err != nil {
		return promotionreward.Coupon{}, internalStoreError()
	}
	after, err := scanPromotionCoupon(tx.QueryRow(ctx, `
		SELECT `+promotionCouponColumns+`
		FROM promotion_coupons coupon
		JOIN users owner ON owner.id = coupon.user_id
		LEFT JOIN api_services service ON service.id = coupon.used_api_service_id
		WHERE coupon.id = $1
	`, input.CouponID))
	if err != nil {
		return promotionreward.Coupon{}, internalStoreError()
	}
	after.Status = promotionreward.CouponStatusRevoked
	if appErr := insertPromotionRewardAudit(ctx, tx, input.AdminUserID, "promotion_coupon.revoked", "promotion_coupon", after.ID, input.Reason, couponAuditSnapshot(before), couponAuditSnapshot(after), input.RequestID, now); appErr != nil {
		return promotionreward.Coupon{}, appErr
	}
	if appErr := insertPromotionRewardEventAndNotification(ctx, tx, after.UserID, "promotion_coupon", after.ID, "promotion_coupon.revoked", "admin", input.AdminUserID, after.Version, input.RequestID, "推广券已撤销", "管理员已撤销一张推广券，正在生效的推广也已停止。", "/my/promotion-benefits", now); appErr != nil {
		return promotionreward.Coupon{}, appErr
	}
	return after, nil
}

func (s *Store) listPromotionCoupons(ctx context.Context, baseWhere string, baseArgs []any, query promotionreward.CouponQuery, now time.Time) (promotionreward.CouponPage, *domain.AppError) {
	where := baseWhere
	args := append([]any(nil), baseArgs...)
	statusExpression := `CASE
	  WHEN coupon.status IN ('used', 'revoked') THEN coupon.status
	  WHEN coupon.expires_at <= $%d THEN 'expired'
	  WHEN coupon.available_at > $%d THEN 'pending'
	  ELSE 'available'
	END`
	if query.Status != promotionreward.CouponStatusAll {
		args = append(args, now)
		nowIndex := len(args)
		statusExpression = fmt.Sprintf(statusExpression, nowIndex, nowIndex)
		args = append(args, query.Status)
		where += fmt.Sprintf(" AND (%s) = $%d", statusExpression, len(args))
	}
	if query.SourceType != "" {
		args = append(args, query.SourceType)
		where += fmt.Sprintf(" AND coupon.source_type = $%d", len(args))
	}
	if query.Search != "" {
		args = append(args, "%"+strings.ToLower(query.Search)+"%")
		where += fmt.Sprintf(" AND (lower(owner.username) LIKE $%[1]d OR lower(owner.display_name) LIKE $%[1]d OR lower(COALESCE(service.title, '')) LIKE $%[1]d)", len(args))
	}
	var total int
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*)::int
		FROM promotion_coupons coupon
		JOIN users owner ON owner.id = coupon.user_id
		LEFT JOIN api_services service ON service.id = coupon.used_api_service_id
		WHERE `+where, args...).Scan(&total); err != nil {
		return promotionreward.CouponPage{}, internalStoreError()
	}
	page, limit := normalizePromotionRewardPage(query.Page, query.Limit)
	args = append(args, limit, (page-1)*limit)
	rows, err := s.pool.Query(ctx, `
		SELECT `+promotionCouponColumns+`
		FROM promotion_coupons coupon
		JOIN users owner ON owner.id = coupon.user_id
		LEFT JOIN api_services service ON service.id = coupon.used_api_service_id
		WHERE `+where+`
		ORDER BY coupon.created_at DESC, coupon.id DESC
		LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return promotionreward.CouponPage{}, internalStoreError()
	}
	defer rows.Close()
	items := make([]promotionreward.Coupon, 0)
	for rows.Next() {
		item, scanErr := scanPromotionCoupon(rows)
		if scanErr != nil {
			return promotionreward.CouponPage{}, internalStoreError()
		}
		item.Status = promotionreward.EffectiveCouponStatus(item, now)
		items = append(items, item)
	}
	if rows.Err() != nil {
		return promotionreward.CouponPage{}, internalStoreError()
	}
	return promotionreward.CouponPage{Items: items, Pagination: promotionRewardPagination(page, limit, total)}, nil
}

func bindReferralRegistrationInTx(ctx context.Context, tx pgx.Tx, inviteeUserID, rawCode string, now time.Time) *domain.AppError {
	code := promotionreward.CanonicalReferralCode(rawCode)
	if code == "" {
		return nil
	}
	var campaignID, referralCodeID, inviterUserID string
	err := tx.QueryRow(ctx, `
		SELECT campaign.id::text, code.id::text, code.user_id::text
		FROM referral_codes code
		JOIN promotion_reward_campaigns campaign ON campaign.id = code.campaign_id
		JOIN users inviter ON inviter.id = code.user_id
		WHERE code.code = $1
		  AND code.status = 'active'
		  AND campaign.code = $2
		  AND campaign.program_enabled = true
		  AND campaign.referral_enabled = true
		  AND campaign.starts_at <= $3
		  AND (campaign.ends_at IS NULL OR campaign.ends_at > $3)
		  AND inviter.account_status = 'active'
		FOR UPDATE OF campaign, code, inviter
	`, code, promotionreward.CampaignCodeAPIServiceReferralV1, now).Scan(&campaignID, &referralCodeID, &inviterUserID)
	if errors.Is(err, pgx.ErrNoRows) || inviterUserID == inviteeUserID {
		return nil
	}
	if err != nil {
		return internalStoreError()
	}
	relationID := uuid.NewString()
	var insertedID string
	err = tx.QueryRow(ctx, `
		INSERT INTO referral_relations (
		  id, campaign_id, referral_code_id, inviter_user_id, invitee_user_id,
		  status, bound_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, 'bound', $6, $6, $6)
		ON CONFLICT (invitee_user_id) DO NOTHING
		RETURNING id::text
	`, relationID, campaignID, referralCodeID, inviterUserID, inviteeUserID, now).Scan(&insertedID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return internalStoreError()
	}
	return insertPromotionRewardEventAndNotification(ctx, tx, inviterUserID, "referral_relation", insertedID, "referral.bound", "system", "", 1, "referral-registration", "好友已接受邀请", "一位新用户已通过你的邀请加入，完成首次有效发布后将发放奖励。", "/my/promotion-benefits", now)
}

func qualifyPromotionRewardsForAPIServiceInTx(ctx context.Context, tx pgx.Tx, apiServiceID string, now time.Time) *domain.AppError {
	var ownerUserID string
	var firstPublishedAt, userCreatedAt time.Time
	err := tx.QueryRow(ctx, `
		SELECT service.owner_user_id::text, service.first_published_at, owner.created_at
		FROM api_services service
		JOIN users owner ON owner.id = service.owner_user_id
		WHERE service.id = $1
		  AND service.first_published_at IS NOT NULL
		  AND `+publicAPIServiceOrderablePredicateAt("service", "$2")+`
		FOR UPDATE OF service, owner
	`, apiServiceID, now).Scan(&ownerUserID, &firstPublishedAt, &userCreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return internalStoreError()
	}
	campaign, err := scanPromotionRewardCampaign(tx.QueryRow(ctx, `
		SELECT `+promotionRewardCampaignColumns+`
		FROM promotion_reward_campaigns
		WHERE code = $1
		FOR UPDATE
	`, promotionreward.CampaignCodeAPIServiceReferralV1))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return internalStoreError()
	}
	if !campaign.ActiveAt(now) || firstPublishedAt.Before(campaign.StartsAt) || userCreatedAt.Before(campaign.StartsAt) {
		return nil
	}
	if campaign.EndsAt != nil && (!firstPublishedAt.Before(*campaign.EndsAt) || !userCreatedAt.Before(*campaign.EndsAt)) {
		return nil
	}
	if campaign.WelcomeEnabled {
		if appErr := createWelcomeCouponInTx(ctx, tx, campaign, ownerUserID, apiServiceID, now); appErr != nil {
			return appErr
		}
	}
	if !campaign.ReferralEnabled {
		return nil
	}
	var relationID, inviterUserID string
	var relationVersion int64
	err = tx.QueryRow(ctx, `
		SELECT id::text, inviter_user_id::text, version
		FROM referral_relations
		WHERE invitee_user_id = $1 AND campaign_id = $2 AND status = 'bound'
		FOR UPDATE
	`, ownerUserID, campaign.ID).Scan(&relationID, &inviterUserID, &relationVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return internalStoreError()
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, campaign.ID+":"+inviterUserID); err != nil {
		return internalStoreError()
	}
	monthStart, nextMonthStart := shanghaiMonthBounds(now)
	var inviterRewardCount int
	if err := tx.QueryRow(ctx, `
		SELECT count(*)::int
		FROM promotion_coupons
		WHERE campaign_id = $1 AND user_id = $2 AND source_type = 'referral_inviter'
		  AND created_at >= $3 AND created_at < $4
	`, campaign.ID, inviterUserID, monthStart, nextMonthStart).Scan(&inviterRewardCount); err != nil {
		return internalStoreError()
	}
	availableAt := now.Add(time.Duration(campaign.RewardDelayHours) * time.Hour)
	expiresAt := availableAt.AddDate(0, 0, campaign.CouponValidDays)
	if appErr := insertRewardCouponInTx(ctx, tx, campaign, ownerUserID, promotionreward.CouponSourceReferralInvitee, relationID, availableAt, expiresAt, now); appErr != nil {
		return appErr
	}
	riskFlags := []string{}
	if inviterRewardCount < campaign.InviterMonthlyLimit {
		if appErr := insertRewardCouponInTx(ctx, tx, campaign, inviterUserID, promotionreward.CouponSourceReferralInviter, relationID, availableAt, expiresAt, now); appErr != nil {
			return appErr
		}
	} else {
		riskFlags = append(riskFlags, "inviter_monthly_limit_reached")
	}
	newVersion := relationVersion + 1
	_, err = tx.Exec(ctx, `
		UPDATE referral_relations
		SET status = 'rewarded', qualified_at = $2, rewarded_at = $2,
		    qualified_api_service_id = $3, risk_flags = $4,
		    updated_at = $2, version = $5
		WHERE id = $1
	`, relationID, now, apiServiceID, riskFlags, newVersion)
	if err != nil {
		return internalStoreError()
	}
	if appErr := insertPromotionRewardEventAndNotification(ctx, tx, ownerUserID, "referral_relation", relationID, "referral.reward_created", "system", "", newVersion, "first-api-service", "邀请奖励已生成", "你已完成首次有效 API 服务发布，推广券将在奖励延迟期结束后可用。", "/my/promotion-benefits", now); appErr != nil {
		return appErr
	}
	return nil
}

func createWelcomeCouponInTx(ctx context.Context, tx pgx.Tx, campaign promotionreward.Campaign, userID, apiServiceID string, now time.Time) *domain.AppError {
	expiresAt := now.AddDate(0, 0, campaign.CouponValidDays)
	couponID := uuid.NewString()
	var inserted bool
	err := tx.QueryRow(ctx, `
		INSERT INTO promotion_coupons (
		  id, campaign_id, user_id, source_type, source_id, status,
		  available_at, expires_at, duration_hours, created_at, updated_at
		)
		VALUES ($1, $2, $3, 'welcome_first_api_service', $4, 'available', $5, $6, $7, $5, $5)
		ON CONFLICT (user_id) WHERE source_type = 'welcome_first_api_service' DO NOTHING
		RETURNING true
	`, couponID, campaign.ID, userID, apiServiceID, now, expiresAt, campaign.PromotionDurationHours).Scan(&inserted)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return internalStoreError()
	}
	endsAt := now.Add(time.Duration(campaign.PromotionDurationHours) * time.Hour)
	overlap, appErr := hasAPIServicePromotionOverlapInTx(ctx, tx, apiServiceID, now, endsAt, couponID)
	if appErr != nil {
		return appErr
	}
	eventType := "promotion_coupon.created"
	title := "获得新人推广券"
	body := "你的首个有效 API 服务已获得一张推广券。"
	if !overlap {
		activationID := uuid.NewString()
		_, err = tx.Exec(ctx, `
			UPDATE promotion_coupons
			SET status = 'used', used_api_service_id = $2, activation_id = $3,
			    promotion_starts_at = $4, promotion_ends_at = $5, used_at = $4,
			    updated_at = $4, version = version + 1
			WHERE id = $1
		`, couponID, apiServiceID, activationID, now, endsAt)
		if err != nil {
			return internalStoreError()
		}
		eventType = "promotion_reward.started"
		title = "新人推广已生效"
		body = "你的首个有效 API 服务已自动进入推广轮换池。"
	}
	return insertPromotionRewardEventAndNotification(ctx, tx, userID, "promotion_coupon", couponID, eventType, "system", "", map[bool]int64{true: 2, false: 1}[!overlap], "first-api-service", title, body, "/my/promotion-benefits", now)
}

func insertRewardCouponInTx(ctx context.Context, tx pgx.Tx, campaign promotionreward.Campaign, userID, sourceType, sourceID string, availableAt, expiresAt, now time.Time) *domain.AppError {
	couponID := uuid.NewString()
	status := promotionreward.CouponStatusPending
	if !now.Before(availableAt) {
		status = promotionreward.CouponStatusAvailable
	}
	var inserted bool
	err := tx.QueryRow(ctx, `
		INSERT INTO promotion_coupons (
		  id, campaign_id, user_id, source_type, source_id, status,
		  available_at, expires_at, duration_hours, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10)
		ON CONFLICT (user_id, source_type, source_id)
		  WHERE source_type IN ('referral_inviter', 'referral_invitee') DO NOTHING
		RETURNING true
	`, couponID, campaign.ID, userID, sourceType, sourceID, status, availableAt, expiresAt, campaign.PromotionDurationHours, now).Scan(&inserted)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return internalStoreError()
	}
	return insertPromotionRewardEventAndNotification(ctx, tx, userID, "promotion_coupon", couponID, "promotion_coupon.created", "system", "", 1, "referral-reward", "获得邀请推广券", "邀请任务已完成，推广券将在奖励延迟期结束后可用。", "/my/promotion-benefits", now)
}

func hasAPIServicePromotionOverlapInTx(ctx context.Context, tx pgx.Tx, apiServiceID string, startsAt, endsAt time.Time, excludeCouponID string) (bool, *domain.AppError) {
	var overlap bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1 FROM api_service_promotions promotion
		  WHERE promotion.api_service_id = $1
		    AND promotion.stopped_at IS NULL
		    AND promotion.starts_at < $3 AND promotion.ends_at > $2
		) OR EXISTS (
		  SELECT 1 FROM promotion_coupons coupon
		  WHERE coupon.used_api_service_id = $1
		    AND coupon.activation_id IS NOT NULL
		    AND coupon.status = 'used'
		    AND coupon.promotion_starts_at < $3 AND coupon.promotion_ends_at > $2
		    AND ($4::uuid IS NULL OR coupon.id <> $4::uuid)
		)
	`, apiServiceID, startsAt, endsAt, nullUUID(excludeCouponID)).Scan(&overlap)
	if err != nil {
		return false, internalStoreError()
	}
	return overlap, nil
}

func ensureReferralCodeInTx(ctx context.Context, tx pgx.Tx, campaignID, userID string, now time.Time) (string, *domain.AppError) {
	var code string
	err := tx.QueryRow(ctx, `
		SELECT code FROM referral_codes WHERE campaign_id = $1 AND user_id = $2
	`, campaignID, userID).Scan(&code)
	if err == nil {
		return code, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", internalStoreError()
	}
	for attempt := 0; attempt < 8; attempt++ {
		candidate, generateErr := generateReferralCode()
		if generateErr != nil {
			return "", internalStoreError()
		}
		err = tx.QueryRow(ctx, `
			INSERT INTO referral_codes (campaign_id, user_id, code, status, created_at, updated_at)
			VALUES ($1, $2, $3, 'active', $4, $4)
			ON CONFLICT DO NOTHING
			RETURNING code
		`, campaignID, userID, candidate, now).Scan(&code)
		if err == nil {
			return code, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return "", internalStoreError()
		}
		if err := tx.QueryRow(ctx, `SELECT code FROM referral_codes WHERE campaign_id = $1 AND user_id = $2`, campaignID, userID).Scan(&code); err == nil {
			return code, nil
		}
	}
	return "", domain.NewError(http.StatusConflict, domain.CodeValidationFailed, "Referral code unavailable", "邀请码生成失败，请稍后重试。")
}

func referralStatisticsInTx(ctx context.Context, tx pgx.Tx, campaign promotionreward.Campaign, userID string, now time.Time) (promotionreward.ReferralStatistics, *domain.AppError) {
	var result promotionreward.ReferralStatistics
	err := tx.QueryRow(ctx, `
		SELECT count(*)::int,
		       count(*) FILTER (WHERE status IN ('qualified', 'rewarded'))::int,
		       count(*) FILTER (WHERE status = 'rewarded')::int,
		       count(*) FILTER (WHERE status = 'bound')::int
		FROM referral_relations
		WHERE campaign_id = $1 AND inviter_user_id = $2
	`, campaign.ID, userID).Scan(&result.InvitedCount, &result.QualifiedCount, &result.RewardedCount, &result.PendingCount)
	if err != nil {
		return promotionreward.ReferralStatistics{}, internalStoreError()
	}
	monthStart, nextMonthStart := shanghaiMonthBounds(now)
	err = tx.QueryRow(ctx, `
		SELECT count(*)::int FROM promotion_coupons
		WHERE campaign_id = $1 AND user_id = $2 AND source_type = 'referral_inviter'
		  AND created_at >= $3 AND created_at < $4
	`, campaign.ID, userID, monthStart, nextMonthStart).Scan(&result.InviterRewardsThisMonth)
	if err != nil {
		return promotionreward.ReferralStatistics{}, internalStoreError()
	}
	result.InviterRewardsRemaining = campaign.InviterMonthlyLimit - result.InviterRewardsThisMonth
	if result.InviterRewardsRemaining < 0 {
		result.InviterRewardsRemaining = 0
	}
	return result, nil
}

func getReferralRecordInTx(ctx context.Context, tx pgx.Tx, id string, lock bool) (promotionreward.ReferralRecord, error) {
	query := `
		SELECT relation.id::text, relation.inviter_user_id::text, relation.invitee_user_id::text,
		       COALESCE(inviter.display_name, inviter.username, ''), COALESCE(invitee.display_name, invitee.username, ''),
		       relation.status, relation.bound_at, relation.qualified_at, relation.rewarded_at,
		       COALESCE(relation.qualified_api_service_id::text, ''), relation.rejected_at,
		       relation.rejected_reason, relation.revoked_at, relation.revoked_reason,
		       relation.risk_flags, relation.created_at, relation.updated_at, relation.version
		FROM referral_relations relation
		JOIN users inviter ON inviter.id = relation.inviter_user_id
		JOIN users invitee ON invitee.id = relation.invitee_user_id
		WHERE relation.id = $1`
	if lock {
		query += " FOR UPDATE OF relation"
	}
	return scanReferralRecord(tx.QueryRow(ctx, query, id))
}

func scanPromotionRewardCampaign(row scanner) (promotionreward.Campaign, error) {
	var item promotionreward.Campaign
	err := row.Scan(&item.ID, &item.Code, &item.ProgramEnabled, &item.WelcomeEnabled, &item.ReferralEnabled,
		&item.StartsAt, &item.EndsAt, &item.PromotionDurationHours, &item.CouponValidDays,
		&item.RewardDelayHours, &item.InviterMonthlyLimit, &item.RulesText,
		&item.CreatedByAdminID, &item.UpdatedByAdminID, &item.CreatedAt, &item.UpdatedAt, &item.Version)
	return item, err
}

func scanReferralRecord(row scanner) (promotionreward.ReferralRecord, error) {
	var item promotionreward.ReferralRecord
	err := row.Scan(&item.ID, &item.InviterUserID, &item.InviteeUserID,
		&item.InviterDisplayName, &item.InviteeDisplayName, &item.Status, &item.BoundAt,
		&item.QualifiedAt, &item.RewardedAt, &item.QualifiedAPIServiceID, &item.RejectedAt,
		&item.RejectedReason, &item.RevokedAt, &item.RevokedReason, &item.RiskFlags,
		&item.CreatedAt, &item.UpdatedAt, &item.Version)
	return item, err
}

func scanPromotionCoupon(row scanner) (promotionreward.Coupon, error) {
	var item promotionreward.Coupon
	err := row.Scan(&item.ID, &item.CampaignID, &item.UserID, &item.UserDisplayName,
		&item.SourceType, &item.SourceID, &item.StoredStatus, &item.AvailableAt,
		&item.ExpiresAt, &item.DurationHours, &item.UsedAPIServiceID,
		&item.UsedAPIServiceTitle, &item.ActivationID, &item.PromotionStartsAt,
		&item.PromotionEndsAt, &item.UsedAt, &item.RevokedAt, &item.RevokedReason,
		&item.RevokedByAdminID, &item.CreatedByAdminID, &item.GrantReason,
		&item.CreatedAt, &item.UpdatedAt, &item.Version)
	item.Status = item.StoredStatus
	return item, err
}

func publicPromotionRewardConfig(campaign promotionreward.Campaign, now time.Time) promotionreward.PublicConfig {
	active := campaign.ActiveAt(now)
	return promotionreward.PublicConfig{
		ProgramEnabled: active, WelcomeEnabled: active && campaign.WelcomeEnabled,
		ReferralEnabled:        active && campaign.ReferralEnabled,
		PromotionDurationHours: campaign.PromotionDurationHours, CouponValidDays: campaign.CouponValidDays,
		RewardDelayHours: campaign.RewardDelayHours, InviterMonthlyLimit: campaign.InviterMonthlyLimit,
		RulesText: campaign.RulesText, StartsAt: campaign.StartsAt, EndsAt: campaign.EndsAt,
	}
}

func generateReferralCode() (string, error) {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	for i := range bytes {
		bytes[i] = referralCodeAlphabet[int(bytes[i])%len(referralCodeAlphabet)]
	}
	return string(bytes), nil
}

func maskDisplayName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "用户"
	}
	runes := []rune(value)
	if len(runes) == 1 {
		return string(runes[0]) + "*"
	}
	if len(runes) == 2 {
		return string(runes[0]) + "*"
	}
	return string(runes[0]) + strings.Repeat("*", min(len(runes)-2, 4)) + string(runes[len(runes)-1])
}

func shanghaiMonthBounds(now time.Time) (time.Time, time.Time) {
	zone, err := time.LoadLocation(promotionreward.BusinessTimezone)
	if err != nil {
		zone = time.FixedZone(promotionreward.BusinessTimezone, 8*60*60)
	}
	local := now.In(zone)
	start := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, zone)
	return start.UTC(), start.AddDate(0, 1, 0).UTC()
}

func normalizePromotionRewardPage(page, limit int) (int, int) {
	if page < 1 {
		page = promotionreward.DefaultPage
	}
	if limit < 1 {
		limit = promotionreward.DefaultPageLimit
	}
	if limit > promotionreward.MaximumPageLimit {
		limit = promotionreward.MaximumPageLimit
	}
	return page, limit
}

func promotionRewardPagination(page, limit, total int) promotionreward.Pagination {
	totalPages := 0
	if total > 0 {
		totalPages = (total + limit - 1) / limit
	}
	return promotionreward.Pagination{Page: page, Limit: limit, TotalItems: total, TotalPages: totalPages}
}

func insertPromotionRewardEventAndNotification(ctx context.Context, tx pgx.Tx, notifyUserID, aggregateType, aggregateID, eventType, actorKind, actorUserID string, version int64, requestID, title, body, targetURL string, now time.Time) *domain.AppError {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		requestID = "unknown"
	}
	eventID := uuid.NewString()
	_, err := tx.Exec(ctx, `
		INSERT INTO domain_events (
		  id, aggregate_type, aggregate_id, event_type, actor_user_id, actor_kind,
		  aggregate_version, request_id, metadata_json, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, '{}'::jsonb, $9)
		ON CONFLICT (aggregate_type, aggregate_id, aggregate_version) DO NOTHING
	`, eventID, aggregateType, aggregateID, eventType, nullUUID(actorUserID), actorKind, version, requestID, now)
	if err != nil {
		return internalStoreError()
	}
	if strings.TrimSpace(notifyUserID) == "" {
		return nil
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO notifications (
		  user_id, type, title, body, target_type, target_id, target_url,
		  source_event_type, source_event_id, dedupe_key, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6::uuid, $7, $2, $8,
		        $5 || ':' || $6::uuid::text || ':' || $2 || ':' || $9::bigint::text, $10)
		ON CONFLICT (user_id, dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING
	`, notifyUserID, eventType, title, body, aggregateType, aggregateID, targetURL, eventID, version, now)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func insertPromotionRewardAudit(ctx context.Context, tx pgx.Tx, adminUserID, action, targetType, targetID, reason string, before, after any, requestID string, now time.Time) *domain.AppError {
	beforeJSON, err := json.Marshal(before)
	if err != nil {
		return internalStoreError()
	}
	afterJSON, err := json.Marshal(after)
	if err != nil {
		return internalStoreError()
	}
	if before == nil {
		beforeJSON = nil
	}
	if after == nil {
		afterJSON = nil
	}
	if strings.TrimSpace(requestID) == "" {
		requestID = "unknown"
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO admin_audit_logs (
		  admin_user_id, action, target_type, target_id, reason,
		  before_json, after_json, request_id, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, adminUserID, action, targetType, targetID, reason, nullableJSON(beforeJSON), nullableJSON(afterJSON), requestID, now)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func campaignAuditSnapshot(item promotionreward.Campaign) map[string]any {
	return map[string]any{"programEnabled": item.ProgramEnabled, "welcomeEnabled": item.WelcomeEnabled, "referralEnabled": item.ReferralEnabled, "startsAt": item.StartsAt, "endsAt": item.EndsAt, "promotionDurationHours": item.PromotionDurationHours, "couponValidDays": item.CouponValidDays, "rewardDelayHours": item.RewardDelayHours, "inviterMonthlyLimit": item.InviterMonthlyLimit, "rulesText": item.RulesText, "version": item.Version}
}

func referralAuditSnapshot(item promotionreward.ReferralRecord) map[string]any {
	return map[string]any{"status": item.Status, "revokedAt": item.RevokedAt, "revokedReason": item.RevokedReason, "version": item.Version}
}

func couponAuditSnapshot(item promotionreward.Coupon) map[string]any {
	return map[string]any{"status": item.StoredStatus, "userId": item.UserID, "sourceType": item.SourceType, "durationHours": item.DurationHours, "expiresAt": item.ExpiresAt, "usedApiServiceId": item.UsedAPIServiceID, "promotionEndsAt": item.PromotionEndsAt, "version": item.Version}
}

func promotionRewardFeatureDisabledError() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeFeatureDisabled, "Feature disabled", "推广权益活动当前未开启。")
}

func promotionCouponNotFoundError() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Promotion coupon not found", "推广券不存在。")
}

func promotionCouponUnavailableError(status string) *domain.AppError {
	detail := "推广券当前不可使用。"
	switch status {
	case promotionreward.CouponStatusPending:
		detail = "推广券尚未到可用时间。"
	case promotionreward.CouponStatusExpired:
		detail = "推广券已过期。"
	case promotionreward.CouponStatusUsed:
		detail = "推广券已使用。"
	case promotionreward.CouponStatusRevoked:
		detail = "推广券已撤销。"
	}
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Promotion coupon unavailable", detail)
}

func promotionRewardVersionConflictError() *domain.AppError {
	return domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "记录已更新，请刷新后重试。")
}

var _ = utf8.RuneCountInString
