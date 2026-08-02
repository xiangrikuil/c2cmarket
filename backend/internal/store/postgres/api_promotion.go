package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/apipromotion"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/jackc/pgx/v5"
)

const apiPromotionColumns = `
	id::text, api_service_id::text, placement, starts_at, ends_at,
	created_reason, created_by_admin_id::text, stopped_at,
	COALESCE(stopped_by_admin_id::text, ''), stopped_reason,
	created_at, updated_at, version
`

func (s *Store) ListPublicAPIPromotions(ctx context.Context, placement string, now time.Time) ([]apipromotion.Promotion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+apiPromotionColumns+`
		FROM api_service_promotions promotion
		WHERE promotion.placement = $1
		  AND promotion.stopped_at IS NULL
		  AND promotion.starts_at <= $2
		  AND promotion.ends_at > $2
		  AND EXISTS (
		    SELECT 1
		    FROM api_services service
		    WHERE service.id = promotion.api_service_id
		      AND `+publicAPIServiceOrderablePredicateAt("service", "$2")+`
		  )
		ORDER BY md5(promotion.id::text || (($2 AT TIME ZONE 'Asia/Shanghai')::date)::text), promotion.id
		LIMIT $3
	`, placement, now, apipromotion.APIMarketTopCapacity)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	items, appErr := scanAPIPromotions(rows)
	if appErr != nil {
		return nil, appErr
	}
	visible := make([]apipromotion.Promotion, 0, len(items))
	for i := range items {
		service, err := s.getPublicAPIService(ctx, s.pool, items[i].APIServiceID, false)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, internalStoreError()
		}
		items[i].Service = service
		items[i].Eligibility = apipromotion.Eligibility{Configurable: true, Displayable: true}
		items[i].Capacity = apipromotion.APIMarketTopCapacity
		visible = append(visible, items[i])
	}
	return visible, nil
}

func (s *Store) ListAdminAPIPromotions(ctx context.Context, now time.Time) ([]apipromotion.Promotion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+apiPromotionColumns+`
		FROM api_service_promotions
		ORDER BY starts_at DESC, created_at DESC, id DESC
	`)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	items, appErr := scanAPIPromotions(rows)
	if appErr != nil {
		return nil, appErr
	}
	for i := range items {
		service, err := s.getAPIService(ctx, s.pool, items[i].APIServiceID, false)
		if err != nil {
			return nil, internalStoreError()
		}
		eligibility, appErr := s.GetAPIPromotionEligibility(ctx, items[i].APIServiceID, now)
		if appErr != nil {
			return nil, appErr
		}
		items[i].Service = service
		items[i].Eligibility = eligibility
		items[i].Capacity = apipromotion.APIMarketTopCapacity
		items[i].OverlappingCampaigns, appErr = apiPromotionPeakOverlap(ctx, s.pool, items[i].Placement, items[i].StartsAt, items[i].EndsAt)
		if appErr != nil {
			return nil, appErr
		}
	}
	return items, nil
}

func (s *Store) GetAPIPromotionEligibility(ctx context.Context, serviceID string, now time.Time) (apipromotion.Eligibility, *domain.AppError) {
	if s == nil || s.pool == nil {
		return apipromotion.Eligibility{}, internalStoreError()
	}
	return getAPIPromotionEligibility(ctx, s.pool, serviceID, now)
}

func (s *Store) GetAPIPromotionAvailability(ctx context.Context, input apipromotion.AvailabilityInput, now time.Time) (apipromotion.Availability, *domain.AppError) {
	if s == nil || s.pool == nil {
		return apipromotion.Availability{}, internalStoreError()
	}
	eligibility, appErr := getAPIPromotionEligibility(ctx, s.pool, input.APIServiceID, now)
	if appErr != nil {
		return apipromotion.Availability{}, appErr
	}
	peak, appErr := apiPromotionPeakOverlap(ctx, s.pool, input.Placement, input.StartsAt, input.EndsAt)
	if appErr != nil {
		return apipromotion.Availability{}, appErr
	}
	sameServiceOverlap, appErr := hasSameServicePromotionOverlap(ctx, s.pool, input.APIServiceID, input.StartsAt, input.EndsAt)
	if appErr != nil {
		return apipromotion.Availability{}, appErr
	}
	remaining := apipromotion.APIMarketTopCapacity - peak
	if remaining < 0 {
		remaining = 0
	}
	return apipromotion.Availability{
		Eligibility:          eligibility,
		OverlappingCampaigns: peak,
		Capacity:             apipromotion.APIMarketTopCapacity,
		RemainingCapacity:    remaining,
		SameServiceOverlap:   sameServiceOverlap,
	}, nil
}

func getAPIPromotionEligibility(ctx context.Context, q queryer, serviceID string, now time.Time) (apipromotion.Eligibility, *domain.AppError) {
	var reviewStatus, publicationStatus, moderationStatus, accountStatus, billingMode string
	var acceptingOrders, restricted, displayable, hasPaymentOption, hasPackageStock, hasAvailableAllowance bool
	var quotaExpiresAt *time.Time
	var unresolvedDisputes int
	err := q.QueryRow(ctx, `
		SELECT service.review_status,
		       service.publication_status,
		       service.moderation_status,
		       service.accepting_orders,
		       owner.account_status,
		       service.billing_mode,
		       COALESCE(service.available_usd_allowance, 0) > 0,
		       service.quota_expires_at,
		       EXISTS (
		         SELECT 1
		         FROM user_restrictions restriction
		         WHERE restriction.user_id = service.owner_user_id
		           AND restriction.role_scope IN ('seller', 'all')
		           AND restriction.action_code IN ('api_service_publish', 'all')
		           AND restriction.revoked_at IS NULL
		           AND restriction.starts_at <= $2
		           AND (restriction.ends_at IS NULL OR restriction.ends_at > $2)
		       ),
		       EXISTS (
		         SELECT 1 FROM api_services candidate
		         WHERE candidate.id = service.id
		           AND `+publicAPIServiceOrderablePredicateAt("candidate", "$2")+`
		       ),
		       EXISTS (
		         SELECT 1
		         FROM api_service_payment_options payment
		         WHERE payment.api_service_id = service.id
		           AND payment.enabled = true
		           AND payment.payment_method IN (`+apiServiceSupportedPaymentMethodsSQL+`)
		       ),
		       EXISTS (
		         SELECT 1
		         FROM api_service_packages package_row
		         WHERE package_row.api_service_id = service.id
		           AND package_row.enabled = true
		           AND package_row.stock_available > 0
		           AND EXISTS (
		             SELECT 1 FROM api_service_package_models package_model
		             WHERE package_model.api_service_package_id = package_row.id
		           )
		       ),
		       (
		         SELECT count(*)::int
		         FROM api_orders orders
		         WHERE orders.api_service_id = service.id
		           AND orders.dispute_status = 'open'
		       )
		FROM api_services service
		JOIN users owner ON owner.id = service.owner_user_id
		WHERE service.id = $1
	`, serviceID, now).Scan(
		&reviewStatus,
		&publicationStatus,
		&moderationStatus,
		&acceptingOrders,
		&accountStatus,
		&billingMode,
		&hasAvailableAllowance,
		&quotaExpiresAt,
		&restricted,
		&displayable,
		&hasPaymentOption,
		&hasPackageStock,
		&unresolvedDisputes,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return apipromotion.Eligibility{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API service not found", "API 服务不存在。")
	}
	if err != nil {
		return apipromotion.Eligibility{}, internalStoreError()
	}

	result := apipromotion.Eligibility{Configurable: true, Displayable: displayable}
	if reviewStatus != apimarket.ServiceReviewStatusApproved {
		result.HardBlockReasons = append(result.HardBlockReasons, "服务尚未审核通过。")
	}
	if moderationStatus == apimarket.ServiceModerationStatusAdminSuspended || moderationStatus == apimarket.ServiceModerationStatusRemoved {
		result.HardBlockReasons = append(result.HardBlockReasons, "服务已被平台暂停或移除。")
	}
	if accountStatus != "active" {
		result.HardBlockReasons = append(result.HardBlockReasons, "商户账号当前不可用。")
	}
	if restricted {
		result.HardBlockReasons = append(result.HardBlockReasons, "商户存在当前生效的卖家经营限制。")
	}
	result.Configurable = len(result.HardBlockReasons) == 0
	if unresolvedDisputes > 0 {
		result.WarningReasons = append(result.WarningReasons, "该服务存在普通未解决纠纷，请人工复核后再配置。")
	}
	if !displayable {
		if publicationStatus != apimarket.ServicePublicationStatusOnline {
			result.SuppressionReasons = append(result.SuppressionReasons, "服务当前未上线。")
		}
		if !acceptingOrders {
			result.SuppressionReasons = append(result.SuppressionReasons, "服务当前暂停接单。")
		}
		if !hasPaymentOption {
			result.SuppressionReasons = append(result.SuppressionReasons, "服务没有有效付款方式。")
		}
		if billingMode == apimarket.ServiceBillingModeMetered {
			if !hasAvailableAllowance {
				result.SuppressionReasons = append(result.SuppressionReasons, "自由额度已耗尽。")
			}
			if quotaExpiresAt == nil || !quotaExpiresAt.After(now) {
				result.SuppressionReasons = append(result.SuppressionReasons, "自由额度已过期。")
			}
		}
		if billingMode == apimarket.ServiceBillingModeFixedPackage && !hasPackageStock {
			result.SuppressionReasons = append(result.SuppressionReasons, "固定套餐已售罄。")
		}
		if len(result.SuppressionReasons) == 0 && len(result.HardBlockReasons) > 0 {
			result.SuppressionReasons = append(result.SuppressionReasons, result.HardBlockReasons...)
		}
	}
	return result, nil
}

func apiPromotionPeakOverlap(ctx context.Context, q queryer, placement string, startsAt, endsAt time.Time) (int, *domain.AppError) {
	var peak int
	err := q.QueryRow(ctx, `
		WITH overlapping AS (
		  SELECT starts_at, ends_at
		  FROM api_service_promotions
		  WHERE placement = $1
		    AND stopped_at IS NULL
		    AND starts_at < $3
		    AND ends_at > $2
		), points AS (
		  SELECT $2::timestamptz AS point
		  UNION
		  SELECT starts_at
		  FROM overlapping
		  WHERE starts_at > $2 AND starts_at < $3
		)
		SELECT COALESCE(MAX((
		  SELECT count(*)
		  FROM overlapping campaign
		  WHERE campaign.starts_at <= points.point
		    AND campaign.ends_at > points.point
		)), 0)::int
		FROM points
	`, placement, startsAt, endsAt).Scan(&peak)
	if err != nil {
		return 0, internalStoreError()
	}
	return peak, nil
}

func hasSameServicePromotionOverlap(ctx context.Context, q queryer, serviceID string, startsAt, endsAt time.Time) (bool, *domain.AppError) {
	var overlaps bool
	err := q.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1
		  FROM api_service_promotions
		  WHERE api_service_id = $1
		    AND stopped_at IS NULL
		    AND starts_at < $3
		    AND ends_at > $2
		)
	`, serviceID, startsAt, endsAt).Scan(&overlaps)
	if err != nil {
		return false, internalStoreError()
	}
	return overlaps, nil
}

func (s *Store) CreateAPIPromotion(ctx context.Context, input apipromotion.CreateInput, now time.Time) (apipromotion.Promotion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return apipromotion.Promotion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apipromotion.Promotion{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	item, appErr := s.createAPIPromotionInTx(ctx, tx, input, now)
	if appErr != nil {
		return apipromotion.Promotion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return apipromotion.Promotion{}, internalStoreError()
	}
	return item, nil
}

func (s *Store) CreateAPIPromotionWithIdempotency(ctx context.Context, entry idempotency.Entry, input apipromotion.CreateInput, now time.Time, buildCompletion apipromotion.CompletionBuilder) (apipromotion.Promotion, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return apipromotion.Promotion{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apipromotion.Promotion{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return apipromotion.Promotion{}, idempotency.Completion{}, appErr
	}
	item, appErr := s.createAPIPromotionInTx(ctx, tx, input, now)
	if appErr != nil {
		return apipromotion.Promotion{}, idempotency.Completion{}, appErr
	}
	item.Status = apipromotion.StatusAt(item, now)
	completion, appErr := buildCompletion(item)
	if appErr != nil {
		return apipromotion.Promotion{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return apipromotion.Promotion{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return apipromotion.Promotion{}, idempotency.Completion{}, internalStoreError()
	}
	return item, completion, nil
}

func (s *Store) createAPIPromotionInTx(ctx context.Context, tx pgx.Tx, input apipromotion.CreateInput, now time.Time) (apipromotion.Promotion, *domain.AppError) {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, "api_service_promotion:"+input.Placement); err != nil {
		return apipromotion.Promotion{}, internalStoreError()
	}
	eligibility, appErr := getAPIPromotionEligibility(ctx, tx, input.APIServiceID, now)
	if appErr != nil {
		return apipromotion.Promotion{}, appErr
	}
	if !eligibility.Configurable {
		return apipromotion.Promotion{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Promotion unavailable", "当前 API 服务不符合推广配置条件。")
	}
	peakOverlap, appErr := apiPromotionPeakOverlap(ctx, tx, input.Placement, input.StartsAt, input.EndsAt)
	if appErr != nil {
		return apipromotion.Promotion{}, appErr
	}
	if peakOverlap >= apipromotion.APIMarketTopCapacity {
		return apipromotion.Promotion{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Promotion capacity full", "所选时间段的推广位已满。")
	}
	sameServiceOverlap, appErr := hasSameServicePromotionOverlap(ctx, tx, input.APIServiceID, input.StartsAt, input.EndsAt)
	if appErr != nil {
		return apipromotion.Promotion{}, appErr
	}
	if sameServiceOverlap {
		return apipromotion.Promotion{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Promotion overlaps", "该 API 服务在所选时间段已有推广排期。")
	}
	item, err := scanAPIPromotion(tx.QueryRow(ctx, `
		INSERT INTO api_service_promotions (
		  api_service_id, placement, starts_at, ends_at,
		  created_reason, created_by_admin_id, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
		RETURNING `+apiPromotionColumns+`
	`, input.APIServiceID, input.Placement, input.StartsAt, input.EndsAt, input.Reason, input.AdminUserID, now))
	if err != nil {
		return apipromotion.Promotion{}, internalStoreError()
	}
	item.Eligibility = eligibility
	item.OverlappingCampaigns = peakOverlap + 1
	item.Capacity = apipromotion.APIMarketTopCapacity
	service, err := s.getAPIService(ctx, tx, item.APIServiceID, false)
	if err != nil {
		return apipromotion.Promotion{}, internalStoreError()
	}
	item.Service = service
	if appErr := insertAPIPromotionAudit(ctx, tx, input.AdminUserID, "api_service_promotion.created", item.ID, input.Reason, nil, promotionAuditSnapshot(item), input.RequestID, now); appErr != nil {
		return apipromotion.Promotion{}, appErr
	}
	return item, nil
}

func (s *Store) StopAPIPromotion(ctx context.Context, input apipromotion.StopInput, now time.Time) (apipromotion.Promotion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return apipromotion.Promotion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apipromotion.Promotion{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	item, appErr := s.stopAPIPromotionInTx(ctx, tx, input, now)
	if appErr != nil {
		return apipromotion.Promotion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return apipromotion.Promotion{}, internalStoreError()
	}
	return item, nil
}

func (s *Store) StopAPIPromotionWithIdempotency(ctx context.Context, entry idempotency.Entry, input apipromotion.StopInput, now time.Time, buildCompletion apipromotion.CompletionBuilder) (apipromotion.Promotion, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return apipromotion.Promotion{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apipromotion.Promotion{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return apipromotion.Promotion{}, idempotency.Completion{}, appErr
	}
	item, appErr := s.stopAPIPromotionInTx(ctx, tx, input, now)
	if appErr != nil {
		return apipromotion.Promotion{}, idempotency.Completion{}, appErr
	}
	item.Status = apipromotion.StatusAt(item, now)
	completion, appErr := buildCompletion(item)
	if appErr != nil {
		return apipromotion.Promotion{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return apipromotion.Promotion{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return apipromotion.Promotion{}, idempotency.Completion{}, internalStoreError()
	}
	return item, completion, nil
}

func (s *Store) stopAPIPromotionInTx(ctx context.Context, tx pgx.Tx, input apipromotion.StopInput, now time.Time) (apipromotion.Promotion, *domain.AppError) {
	current, err := scanAPIPromotion(tx.QueryRow(ctx, `
		SELECT `+apiPromotionColumns+`
		FROM api_service_promotions
		WHERE id = $1
		FOR UPDATE
	`, input.PromotionID))
	if errors.Is(err, pgx.ErrNoRows) {
		return apipromotion.Promotion{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Promotion not found", "推广记录不存在。")
	}
	if err != nil {
		return apipromotion.Promotion{}, internalStoreError()
	}
	if current.Version != input.ExpectedVersion {
		return apipromotion.Promotion{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if current.StoppedAt != nil || !now.Before(current.EndsAt) {
		return apipromotion.Promotion{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Promotion cannot be stopped", "该推广记录已经停止或结束。")
	}
	item, err := scanAPIPromotion(tx.QueryRow(ctx, `
		UPDATE api_service_promotions
		SET stopped_at = $2,
		    stopped_by_admin_id = $3,
		    stopped_reason = $4,
		    updated_at = $2,
		    version = version + 1
		WHERE id = $1
		RETURNING `+apiPromotionColumns+`
	`, input.PromotionID, now, input.AdminUserID, input.Reason))
	if err != nil {
		return apipromotion.Promotion{}, internalStoreError()
	}
	service, err := s.getAPIService(ctx, tx, item.APIServiceID, false)
	if err != nil {
		return apipromotion.Promotion{}, internalStoreError()
	}
	item.Service = service
	eligibility, appErr := getAPIPromotionEligibility(ctx, tx, item.APIServiceID, now)
	if appErr != nil {
		return apipromotion.Promotion{}, appErr
	}
	item.Eligibility = eligibility
	item.Capacity = apipromotion.APIMarketTopCapacity
	if appErr := insertAPIPromotionAudit(ctx, tx, input.AdminUserID, "api_service_promotion.stopped", item.ID, input.Reason, promotionAuditSnapshot(current), promotionAuditSnapshot(item), input.RequestID, now); appErr != nil {
		return apipromotion.Promotion{}, appErr
	}
	return item, nil
}

func scanAPIPromotions(rows pgx.Rows) ([]apipromotion.Promotion, *domain.AppError) {
	items := []apipromotion.Promotion{}
	for rows.Next() {
		item, err := scanAPIPromotion(rows)
		if err != nil {
			return nil, internalStoreError()
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, internalStoreError()
	}
	return items, nil
}

func scanAPIPromotion(row scanner) (apipromotion.Promotion, error) {
	var item apipromotion.Promotion
	err := row.Scan(
		&item.ID,
		&item.APIServiceID,
		&item.Placement,
		&item.StartsAt,
		&item.EndsAt,
		&item.CreatedReason,
		&item.CreatedByAdminID,
		&item.StoppedAt,
		&item.StoppedByAdminID,
		&item.StoppedReason,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.Version,
	)
	return item, err
}

func promotionAuditSnapshot(item apipromotion.Promotion) map[string]any {
	return map[string]any{
		"id":           item.ID,
		"apiServiceId": item.APIServiceID,
		"placement":    item.Placement,
		"startsAt":     item.StartsAt,
		"endsAt":       item.EndsAt,
		"stoppedAt":    item.StoppedAt,
		"version":      item.Version,
	}
}

func insertAPIPromotionAudit(ctx context.Context, tx pgx.Tx, adminID, action, targetID, reason string, before, after map[string]any, requestID string, now time.Time) *domain.AppError {
	var beforeJSON, afterJSON []byte
	var err error
	if before != nil {
		beforeJSON, err = json.Marshal(before)
		if err != nil {
			return internalStoreError()
		}
	}
	if after != nil {
		afterJSON, err = json.Marshal(after)
		if err != nil {
			return internalStoreError()
		}
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO admin_audit_logs (
		  admin_user_id, action, target_type, target_id, reason,
		  before_json, after_json, request_id, created_at
		)
		VALUES ($1, $2, 'api_service_promotion', $3, $4, $5::jsonb, $6::jsonb, $7, $8)
	`, adminID, action, targetID, reason, nullableJSON(beforeJSON), nullableJSON(afterJSON), requestID, now)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func nullableJSON(value []byte) any {
	if len(value) == 0 {
		return nil
	}
	return string(value)
}
