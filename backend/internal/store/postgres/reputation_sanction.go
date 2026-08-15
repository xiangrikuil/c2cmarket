package postgres

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/report"
	"c2c-market/backend/internal/module/reputation"

	"github.com/jackc/pgx/v5"
)

func (s *Store) GetAPIOrderSanctionRecommendation(ctx context.Context, disputeCaseID string, now time.Time) (reputation.APIOrderSanctionRecommendation, *domain.AppError) {
	if s == nil || s.pool == nil {
		return reputation.APIOrderSanctionRecommendation{}, internalStoreError()
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return reputation.APIOrderSanctionRecommendation{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	recommendation, appErr := loadAPIOrderSanctionRecommendation(ctx, tx, disputeCaseID, now, false)
	if appErr != nil {
		return reputation.APIOrderSanctionRecommendation{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return reputation.APIOrderSanctionRecommendation{}, internalStoreError()
	}
	return recommendation, nil
}

func (s *Store) ApplyAPIOrderSanctionWithIdempotency(ctx context.Context, entry idempotency.Entry, input reputation.ApplyAPIOrderSanctionInput, now time.Time, buildCompletion reputation.GovernanceCompletionBuilder) (reputation.GovernanceMutationResult, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, appErr
	}
	recommendation, appErr := loadAPIOrderSanctionRecommendation(ctx, tx, input.DisputeCaseID, now, true)
	if appErr != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, appErr
	}
	if !recommendation.Eligible {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, apiOrderSanctionUnavailable()
	}
	if recommendation.SubjectUserVersion != input.ExpectedUserVersion {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, reputationVersionConflict()
	}
	if recommendation.AlreadyApplied {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, apiOrderSanctionAlreadyApplied()
	}

	endsAt := now.AddDate(0, 0, recommendation.RecommendedDays)
	restriction, err := scanUserRestriction(tx.QueryRow(ctx, `
		INSERT INTO user_restrictions (
		  user_id, restriction_type, reason, starts_at, ends_at, created_by_admin_id,
		  created_at, role_scope, action_code, reason_code, public_reason,
		  source_dispute_outcome_id, source_dispute_remedy_id, updated_at, version
		)
		VALUES (
		  $1, $2, $3, $4, $5, $6,
		  $4, $7, $8, $9, $10,
		  $11, $12, $4, 1
		)
		RETURNING `+userRestrictionReturningColumns+`
	`, recommendation.SubjectUserID, reputation.RestrictionTypeAPIOrderRemedyOverdue,
		strings.TrimSpace(input.InternalReason), now, endsAt, input.AdminUserID,
		reputation.RoleSeller, reputation.ActionAPIServicePublish,
		reputation.ReasonCodeAPIOrderRemedyOverdue,
		reputation.APIOrderSanctionPublicReason(recommendation.RecommendedDays),
		recommendation.OutcomeID, recommendation.RemedyID))
	if isUniqueViolation(err) {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, apiOrderSanctionAlreadyApplied()
	}
	if err != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	if err := tx.QueryRow(ctx, `
		UPDATE users
		SET version = version + 1,
		    updated_at = $3
		WHERE id = $1
		  AND version = $2
		RETURNING version
	`, recommendation.SubjectUserID, input.ExpectedUserVersion, now).Scan(&restriction.UserVersion); errors.Is(err, pgx.ErrNoRows) {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, reputationVersionConflict()
	} else if err != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	if appErr := insertReputationGovernanceEvent(ctx, tx, "restriction", restriction.ID, "restriction_created", input.AdminUserID, nil, restriction, input.InternalReason, input.RequestID, now); appErr != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, appErr
	}
	if appErr := insertAPIOrderSanctionNotificationInTx(ctx, tx, restriction, now); appErr != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, appErr
	}
	result := reputation.GovernanceMutationResult{Restriction: &restriction}
	completion, appErr := buildCompletion(result)
	if appErr != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return reputation.GovernanceMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	return result, completion, nil
}

func (s *Store) ListActiveRestrictions(ctx context.Context, userID string, now time.Time) ([]reputation.UserRestriction, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+userRestrictionColumns+`
		FROM user_restrictions
		WHERE user_id = $1
		  AND revoked_at IS NULL
		  AND starts_at <= $2
		  AND (ends_at IS NULL OR $2 < ends_at)
		ORDER BY ends_at ASC NULLS LAST, starts_at DESC, id DESC
	`, userID, now)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	items := make([]reputation.UserRestriction, 0)
	for rows.Next() {
		item, scanErr := scanUserRestriction(rows)
		if scanErr != nil {
			return nil, internalStoreError()
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		return nil, internalStoreError()
	}
	return items, nil
}

func loadAPIOrderSanctionRecommendation(ctx context.Context, q queryer, disputeCaseID string, now time.Time, lock bool) (reputation.APIOrderSanctionRecommendation, *domain.AppError) {
	recommendation := reputation.APIOrderSanctionRecommendation{DisputeCaseID: disputeCaseID}
	lockClause := ""
	if lock {
		lockClause = " FOR UPDATE"
	}
	var targetType, targetID string
	err := q.QueryRow(ctx, `
		SELECT target_type, target_id, version
		FROM dispute_cases
		WHERE id = $1`+lockClause, disputeCaseID).Scan(&targetType, &targetID, &recommendation.DisputeVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return reputation.APIOrderSanctionRecommendation{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Dispute not found", "纠纷不存在。")
	}
	if err != nil {
		return reputation.APIOrderSanctionRecommendation{}, internalStoreError()
	}
	if targetType != report.TargetAPIOrder {
		recommendation.ReasonCode = "api_order_required"
		return recommendation, nil
	}

	var remedyLatenessStatus, responsibleUserID string
	var overdueAt *time.Time
	var latenessReversedAt *time.Time
	err = q.QueryRow(ctx, `
		SELECT id::text, lateness_status, responsible_user_id::text, lateness_decided_at, lateness_reversed_at
		FROM api_order_dispute_remedies
		WHERE dispute_case_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT 1`+lockClause, disputeCaseID).Scan(
		&recommendation.RemedyID,
		&remedyLatenessStatus,
		&responsibleUserID,
		&overdueAt,
		&latenessReversedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		recommendation.ReasonCode = "overdue_remedy_required"
		return recommendation, nil
	}
	if err != nil {
		return reputation.APIOrderSanctionRecommendation{}, internalStoreError()
	}

	var outcomeStatus, responsibility string
	err = q.QueryRow(ctx, `
		SELECT id::text, subject_user_id::text, status, responsibility
		FROM dispute_reputation_outcomes
		WHERE dispute_case_id = $1
		  AND subject_user_id = $2
		  AND status = 'active'`+lockClause, disputeCaseID, responsibleUserID).Scan(
		&recommendation.OutcomeID,
		&recommendation.SubjectUserID,
		&outcomeStatus,
		&responsibility,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		recommendation.ReasonCode = "active_outcome_required"
		return recommendation, nil
	}
	if err != nil {
		return reputation.APIOrderSanctionRecommendation{}, internalStoreError()
	}

	var sellerUserID string
	err = q.QueryRow(ctx, `
		SELECT seller_user_id::text
		FROM api_orders
		WHERE id::text = $1`+lockClause, targetID).Scan(&sellerUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		recommendation.ReasonCode = "api_order_required"
		return recommendation, nil
	}
	if err != nil {
		return reputation.APIOrderSanctionRecommendation{}, internalStoreError()
	}
	if err := q.QueryRow(ctx, `SELECT version FROM users WHERE id = $1`+lockClause, recommendation.SubjectUserID).Scan(&recommendation.SubjectUserVersion); err != nil {
		return reputation.APIOrderSanctionRecommendation{}, internalStoreError()
	}

	existingRestriction, err := scanUserRestriction(q.QueryRow(ctx, `
		SELECT `+userRestrictionColumns+`
		FROM user_restrictions
		WHERE source_dispute_remedy_id = $1
		LIMIT 1`+lockClause, recommendation.RemedyID))
	if err == nil {
		recommendation.AlreadyApplied = true
		recommendation.ExistingRestriction = &existingRestriction
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return reputation.APIOrderSanctionRecommendation{}, internalStoreError()
	}

	switch {
	case remedyLatenessStatus != report.RemedyLatenessLateConfirmed || latenessReversedAt != nil || overdueAt == nil:
		recommendation.ReasonCode = "overdue_remedy_required"
		return recommendation, nil
	case outcomeStatus != reputation.OutcomeStatusActive:
		recommendation.ReasonCode = "active_outcome_required"
		return recommendation, nil
	case !faultResponsibilityValue(responsibility):
		recommendation.ReasonCode = "responsible_outcome_required"
		return recommendation, nil
	case recommendation.SubjectUserID != responsibleUserID || responsibleUserID != sellerUserID:
		recommendation.ReasonCode = "responsible_seller_required"
		return recommendation, nil
	}

	windowStart := now.AddDate(0, 0, -reputation.APIOrderSanctionWindowDays)
	if err := q.QueryRow(ctx, `
		SELECT count(*)
		FROM api_order_dispute_remedies remedy
		JOIN dispute_cases dispute
		  ON dispute.id = remedy.dispute_case_id
		 AND dispute.target_type = 'api_order'
		JOIN api_orders order_row
		  ON order_row.id::text = dispute.target_id
		WHERE remedy.lateness_status = 'late_confirmed'
		  AND remedy.lateness_reversed_at IS NULL
		  AND remedy.lateness_decided_at >= $1
		  AND remedy.responsible_user_id = $2
		  AND order_row.seller_user_id = remedy.responsible_user_id
	`, windowStart, recommendation.SubjectUserID).Scan(&recommendation.ConfirmedBreaches180Days); err != nil {
		return reputation.APIOrderSanctionRecommendation{}, internalStoreError()
	}
	recommendation.Eligible = true
	recommendation.ReasonCode = "eligible"
	recommendation.RecommendedDays = reputation.RecommendedAPIOrderSanctionDays(recommendation.ConfirmedBreaches180Days)
	return recommendation, nil
}

func faultResponsibilityValue(value string) bool {
	return value == reputation.ResponsibilityResponsible || value == reputation.ResponsibilityShared
}

func apiOrderSanctionUnavailable() *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Sanction unavailable", "当前纠纷不满足 API 卖家逾期处罚条件。")
}

func apiOrderSanctionAlreadyApplied() *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Sanction already applied", "该逾期整改已经创建过处罚限制。")
}

func insertAPIOrderSanctionNotificationInTx(ctx context.Context, tx pgx.Tx, restriction reputation.UserRestriction, now time.Time) *domain.AppError {
	dedupeKey := "reputation_restriction:" + restriction.SourceDisputeRemedyID
	if _, err := tx.Exec(ctx, `
		INSERT INTO notifications (
		  user_id, type, title, body, target_type, target_id, target_url,
		  source_event_type, source_event_id, dedupe_key, created_at
		)
		VALUES (
		  $1, 'reputation', 'API 服务限制已生效', $2, 'reputation_restriction', $3,
		  '/my/reputation', 'reputation.restriction_created', NULL, $4, $5
		)
		ON CONFLICT (user_id, dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING
	`, restriction.UserID, restriction.PublicReason, restriction.ID, dedupeKey, now); err != nil {
		return internalStoreError()
	}
	return nil
}
