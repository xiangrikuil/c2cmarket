package postgres

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/reputation"
)

const adminReputationEvidenceLimit = 100

type sourceAuthorAuditKey struct {
	resourceType string
	resourceID   string
}

func (s *Store) LoadAdminReputationEvidence(
	ctx context.Context,
	userID string,
	now time.Time,
) (reputation.AdminReputationEvidence, *domain.AppError) {
	if s == nil || s.pool == nil {
		return reputation.AdminReputationEvidence{}, internalStoreError()
	}
	evidence := reputation.AdminReputationEvidence{
		Restrictions:              []reputation.UserRestriction{},
		Outcomes:                  []reputation.DisputeOutcome{},
		Appeals:                   []reputation.ReputationAppeal{},
		SourceAuthorVerifications: []reputation.SourceAuthorVerificationAudit{},
	}

	restrictions, appErr := s.listAdminUserRestrictions(ctx, userID)
	if appErr != nil {
		return reputation.AdminReputationEvidence{}, appErr
	}
	outcomes, appErr := s.listAdminDisputeOutcomes(ctx, userID)
	if appErr != nil {
		return reputation.AdminReputationEvidence{}, appErr
	}
	appeals, appErr := s.listAdminReputationAppeals(ctx, userID)
	if appErr != nil {
		return reputation.AdminReputationEvidence{}, appErr
	}
	sourceAudits, appErr := s.listAdminSourceAuthorAudits(ctx, userID, now)
	if appErr != nil {
		return reputation.AdminReputationEvidence{}, appErr
	}

	evidence.Restrictions = restrictions
	evidence.Outcomes = outcomes
	evidence.Appeals = appeals
	evidence.SourceAuthorVerifications = sourceAudits
	return evidence, nil
}

func (s *Store) listAdminUserRestrictions(ctx context.Context, userID string) ([]reputation.UserRestriction, *domain.AppError) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+userRestrictionColumns+`
		FROM user_restrictions
		WHERE user_id = $1
		ORDER BY updated_at DESC, id DESC
		LIMIT $2
	`, userID, adminReputationEvidenceLimit)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()

	items := []reputation.UserRestriction{}
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

func (s *Store) listAdminDisputeOutcomes(ctx context.Context, userID string) ([]reputation.DisputeOutcome, *domain.AppError) {
	rows, err := s.pool.Query(ctx, `
		SELECT
		  `+disputeOutcomeReturningColumns+`,
		  dispute_cases.version
		FROM dispute_reputation_outcomes
		JOIN dispute_cases
		  ON dispute_cases.id = dispute_reputation_outcomes.dispute_case_id
		WHERE dispute_reputation_outcomes.subject_user_id = $1
		ORDER BY dispute_reputation_outcomes.updated_at DESC, dispute_reputation_outcomes.id DESC
		LIMIT $2
	`, userID, adminReputationEvidenceLimit)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()

	items := []reputation.DisputeOutcome{}
	for rows.Next() {
		var item reputation.DisputeOutcome
		if err := rows.Scan(
			&item.ID,
			&item.DisputeCaseID,
			&item.SubjectUserID,
			&item.Responsibility,
			&item.Severity,
			&item.RoleScope,
			&item.Status,
			&item.ReasonCode,
			&item.PublicReason,
			&item.InternalReason,
			&item.DecidedByAdminID,
			&item.DecidedAt,
			&item.ReversedAt,
			&item.ReversedByAdminID,
			&item.ReversalAppealID,
			&item.ReversalReason,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.Version,
			&item.DisputeVersion,
		); err != nil {
			return nil, internalStoreError()
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		return nil, internalStoreError()
	}
	return items, nil
}

func (s *Store) listAdminReputationAppeals(ctx context.Context, userID string) ([]reputation.ReputationAppeal, *domain.AppError) {
	rows, err := s.pool.Query(ctx, `
		SELECT
		  appeal.id::text,
		  appeal.appellant_user_id::text,
		  COALESCE(appeal.report_id::text, ''),
		  COALESCE(appeal.dispute_case_id::text, ''),
		  appeal.target_type,
		  appeal.target_id,
		  appeal.title,
		  appeal.statement,
		  appeal.status,
		  appeal.admin_reason,
		  COALESCE(appeal.handled_by_admin_id::text, ''),
		  appeal.handled_at,
		  appeal.created_at,
		  appeal.updated_at,
		  appeal.version
		FROM appeals appeal
		WHERE appeal.appellant_user_id = $1
		   OR EXISTS (
		     SELECT 1
		     FROM dispute_reputation_outcomes outcome
		     WHERE outcome.subject_user_id = $1
		       AND (
		         outcome.dispute_case_id = appeal.dispute_case_id
		         OR outcome.reversal_appeal_id = appeal.id
		       )
		   )
		ORDER BY appeal.updated_at DESC, appeal.id DESC
		LIMIT $2
	`, userID, adminReputationEvidenceLimit)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()

	items := []reputation.ReputationAppeal{}
	for rows.Next() {
		var item reputation.ReputationAppeal
		if err := rows.Scan(
			&item.ID,
			&item.AppellantUserID,
			&item.ReportID,
			&item.DisputeID,
			&item.TargetType,
			&item.TargetID,
			&item.Title,
			&item.Statement,
			&item.Status,
			&item.AdminReason,
			&item.HandledByAdminID,
			&item.HandledAt,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.Version,
		); err != nil {
			return nil, internalStoreError()
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		return nil, internalStoreError()
	}
	return items, nil
}

func (s *Store) listAdminSourceAuthorAudits(
	ctx context.Context,
	userID string,
	now time.Time,
) ([]reputation.SourceAuthorVerificationAudit, *domain.AppError) {
	rows, err := s.pool.Query(ctx, `
		SELECT resource_type, resource_id
		FROM (
		  SELECT
		    'carpool'::text AS resource_type,
		    listing.id::text AS resource_id,
		    listing.updated_at
		  FROM carpool_listings listing
		  WHERE listing.owner_user_id = $1
		    AND NULLIF(trim(listing.source_url), '') IS NOT NULL

		  UNION ALL

		  SELECT
		    'api_service'::text,
		    service.id::text,
		    service.updated_at
		  FROM api_services service
		  WHERE service.owner_user_id = $1
		    AND NULLIF(trim(service.source_url), '') IS NOT NULL
		) resources
		ORDER BY updated_at DESC, resource_type, resource_id
		LIMIT $2
	`, userID, adminReputationEvidenceLimit)
	if err != nil {
		return nil, internalStoreError()
	}

	keys := []sourceAuthorAuditKey{}
	for rows.Next() {
		var key sourceAuthorAuditKey
		if err := rows.Scan(&key.resourceType, &key.resourceID); err != nil {
			rows.Close()
			return nil, internalStoreError()
		}
		keys = append(keys, key)
	}
	if rows.Err() != nil {
		rows.Close()
		return nil, internalStoreError()
	}
	rows.Close()

	items := make([]reputation.SourceAuthorVerificationAudit, 0, len(keys))
	for _, key := range keys {
		audit, appErr := s.GetSourceAuthorVerificationAudit(ctx, key.resourceType, key.resourceID, now)
		if appErr != nil {
			return nil, appErr
		}
		items = append(items, audit)
	}
	return items, nil
}
