package postgres

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/reputation"

	"github.com/jackc/pgx/v5"
)

const sourceAuthorVerificationColumns = `
	id::text, resource_type, resource_id::text, source_url,
	expected_external_user_id, actual_external_user_id, status,
	verification_method, verified_by_admin_id::text, verified_at, expires_at,
	failure_reason, created_at, updated_at, version
`

func (s *Store) GetSourceAuthorVerificationAudit(
	ctx context.Context,
	resourceType string,
	resourceID string,
	now time.Time,
) (reputation.SourceAuthorVerificationAudit, *domain.AppError) {
	if s == nil || s.pool == nil {
		return reputation.SourceAuthorVerificationAudit{}, internalStoreError()
	}
	resource, appErr := loadSourceAuthorResource(ctx, s.pool, resourceType, resourceID, false)
	if appErr != nil {
		return reputation.SourceAuthorVerificationAudit{}, appErr
	}
	verification, found, appErr := loadSourceAuthorVerification(ctx, s.pool, resourceType, resourceID)
	if appErr != nil {
		return reputation.SourceAuthorVerificationAudit{}, appErr
	}
	if !found {
		verification.ResourceType = resourceType
		verification.ResourceID = resourceID
	}
	verification = effectiveSourceAuthorVerification(verification, found, resource, now)
	events, appErr := listSourceAuthorVerificationEvents(ctx, s.pool, resourceType, resourceID)
	if appErr != nil {
		return reputation.SourceAuthorVerificationAudit{}, appErr
	}
	return reputation.SourceAuthorVerificationAudit{Verification: verification, Events: events}, nil
}

func (s *Store) applySourceAuthorFacts(
	ctx context.Context,
	userIDs []string,
	now time.Time,
	result map[string]reputation.RawFacts,
) *domain.AppError {
	for _, userID := range userIDs {
		value := result[userID]
		value.UserID = userID
		value.Buyer.Carpool.SourceAuthorVerification = reputation.SourceAuthorAggregateForCounts(reputation.RoleBuyer, reputation.SourceAuthorStatusCounts{})
		value.Buyer.API.SourceAuthorVerification = reputation.SourceAuthorAggregateForCounts(reputation.RoleBuyer, reputation.SourceAuthorStatusCounts{})
		value.Seller.Carpool.SourceAuthorVerification = reputation.SourceAuthorAggregateForCounts(reputation.RoleSeller, reputation.SourceAuthorStatusCounts{})
		value.Seller.API.SourceAuthorVerification = reputation.SourceAuthorAggregateForCounts(reputation.RoleSeller, reputation.SourceAuthorStatusCounts{})
		result[userID] = value
	}

	query := `
		WITH requested AS (
		  SELECT DISTINCT unnest($1::uuid[]) AS user_id
		),
		resources AS (
		  SELECT
		    listing.owner_user_id AS user_id,
		    'carpool'::text AS scope,
		    CASE
		      WHEN verification.id IS NULL THEN 'not_submitted'
		      WHEN verification.source_url IS DISTINCT FROM listing.source_url
		        OR verification.expected_external_user_id IS DISTINCT FROM COALESCE(binding.linux_do_user_id, '')
		      THEN 'pending'
		      WHEN verification.status = 'verified'
		        AND verification.expires_at IS NOT NULL
		        AND verification.expires_at <= $2
		      THEN 'expired'
		      ELSE verification.status
		    END AS effective_status,
		    GREATEST(
		      listing.updated_at,
		      COALESCE(binding.last_synced_at, binding.bound_at, listing.updated_at),
		      COALESCE(verification.updated_at, listing.updated_at)
		    ) AS source_updated_at,
		    CASE
		      WHEN verification.source_url = listing.source_url
		        AND verification.expected_external_user_id = COALESCE(binding.linux_do_user_id, '')
		        AND verification.status = 'verified'
		        AND verification.expires_at > $2
		      THEN verification.expires_at
		      ELSE NULL
		    END AS next_recalculation_at
		  FROM requested
		  JOIN carpool_listings listing ON listing.owner_user_id = requested.user_id
		  LEFT JOIN linux_do_bindings binding ON binding.user_id = listing.owner_user_id
		  LEFT JOIN source_author_verifications verification
		    ON verification.resource_type = 'carpool'
		   AND verification.resource_id = listing.id
		  WHERE listing.status = 'active'
		    AND NULLIF(trim(listing.source_url), '') IS NOT NULL

		  UNION ALL

		  SELECT
		    service.owner_user_id,
		    'api'::text,
		    CASE
		      WHEN verification.id IS NULL THEN 'not_submitted'
		      WHEN verification.source_url IS DISTINCT FROM service.source_url
		        OR verification.expected_external_user_id IS DISTINCT FROM COALESCE(binding.linux_do_user_id, '')
		      THEN 'pending'
		      WHEN verification.status = 'verified'
		        AND verification.expires_at IS NOT NULL
		        AND verification.expires_at <= $2
		      THEN 'expired'
		      ELSE verification.status
		    END,
		    GREATEST(
		      service.updated_at,
		      COALESCE(binding.last_synced_at, binding.bound_at, service.updated_at),
		      COALESCE(verification.updated_at, service.updated_at)
		    ),
		    CASE
		      WHEN verification.source_url = service.source_url
		        AND verification.expected_external_user_id = COALESCE(binding.linux_do_user_id, '')
		        AND verification.status = 'verified'
		        AND verification.expires_at > $2
		      THEN verification.expires_at
		      ELSE NULL
		    END
		  FROM requested
		  JOIN api_services service ON service.owner_user_id = requested.user_id
		  LEFT JOIN linux_do_bindings binding ON binding.user_id = service.owner_user_id
		  LEFT JOIN source_author_verifications verification
		    ON verification.resource_type = 'api_service'
		   AND verification.resource_id = service.id
		  WHERE ` + publicAPIServiceOrderablePredicate("service") + `
		    AND NULLIF(trim(service.source_url), '') IS NOT NULL
		)
		SELECT
		  user_id::text,
		  scope,
		  count(*)::int,
		  count(*) FILTER (WHERE effective_status = 'not_submitted')::int,
		  count(*) FILTER (WHERE effective_status = 'pending')::int,
		  count(*) FILTER (WHERE effective_status = 'verified')::int,
		  count(*) FILTER (WHERE effective_status = 'mismatch')::int,
		  count(*) FILTER (WHERE effective_status = 'expired')::int,
		  max(source_updated_at),
		  min(next_recalculation_at)
		FROM resources
		GROUP BY user_id, scope
		ORDER BY user_id, scope
	`
	rows, err := s.pool.Query(ctx, query, userIDs, now)
	if err != nil {
		return internalStoreError()
	}
	defer rows.Close()
	for rows.Next() {
		var (
			userID              string
			scope               string
			counts              reputation.SourceAuthorStatusCounts
			sourceUpdatedAt     *time.Time
			nextRecalculationAt *time.Time
		)
		if err := rows.Scan(
			&userID,
			&scope,
			&counts.Total,
			&counts.NotSubmitted,
			&counts.Pending,
			&counts.Verified,
			&counts.Mismatch,
			&counts.Expired,
			&sourceUpdatedAt,
			&nextRecalculationAt,
		); err != nil {
			return internalStoreError()
		}
		value := result[userID]
		target := scopeFacts(&value, reputation.RoleSeller, scope)
		if target == nil {
			return internalStoreError()
		}
		target.SourceAuthorVerification = reputation.SourceAuthorAggregateForCounts(reputation.RoleSeller, counts)
		target.SourceAuthorMismatch = counts.Mismatch > 0
		target.SourceDataUpdatedAt = latestTimestamp(target.SourceDataUpdatedAt, sourceUpdatedAt)
		target.NextRecalculationAt = earliestSourceTimestamp(target.NextRecalculationAt, nextRecalculationAt)
		result[userID] = value
	}
	if rows.Err() != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) UpdateSourceAuthorVerification(
	ctx context.Context,
	input reputation.UpdateSourceAuthorVerificationInput,
	now time.Time,
) (reputation.SourceAuthorVerificationAudit, *domain.AppError) {
	if s == nil || s.pool == nil {
		return reputation.SourceAuthorVerificationAudit{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return reputation.SourceAuthorVerificationAudit{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	resource, appErr := loadSourceAuthorResource(ctx, tx, input.ResourceType, input.ResourceID, true)
	if appErr != nil {
		return reputation.SourceAuthorVerificationAudit{}, appErr
	}
	if strings.TrimSpace(resource.SourceURL) == "" {
		return reputation.SourceAuthorVerificationAudit{}, sourceAuthorStoreFieldError("sourceUrl", "资源尚未填写 linux.do 原帖链接。")
	}
	current, found, appErr := loadSourceAuthorVerificationForUpdate(ctx, tx, input.ResourceType, input.ResourceID)
	if appErr != nil {
		return reputation.SourceAuthorVerificationAudit{}, appErr
	}
	currentVersion := int64(0)
	if found {
		currentVersion = current.Version
	}
	if currentVersion != input.ExpectedVersion {
		return reputation.SourceAuthorVerificationAudit{}, domain.NewError(
			http.StatusPreconditionFailed,
			domain.CodeVersionConflict,
			"Version conflict",
			"原帖作者验证版本已变化，请刷新后重试。",
		)
	}
	if appErr := validateSourceAuthorIdentityDecision(input, resource); appErr != nil {
		return reputation.SourceAuthorVerificationAudit{}, appErr
	}

	var saved reputation.SourceAuthorVerification
	action := "created"
	var fromStatus *string
	verifiedAt := sourceAuthorDecisionTime(input.Status, now)
	if found {
		action = "updated"
		value := current.Status
		fromStatus = &value
		saved, err = scanStoredSourceAuthorVerification(tx.QueryRow(ctx, `
			UPDATE source_author_verifications
			SET source_url = $2,
			    expected_external_user_id = $3,
			    actual_external_user_id = $4,
			    status = $5,
			    verification_method = $6,
			    verified_by_admin_id = $7,
			    verified_at = $8,
			    expires_at = $9,
			    failure_reason = $10,
			    updated_at = $11,
			    version = version + 1
			WHERE id = $1
			RETURNING `+sourceAuthorVerificationColumns,
			current.ID,
			resource.SourceURL,
			resource.ExpectedExternalUserID,
			input.ActualExternalUserID,
			input.Status,
			input.VerificationMethod,
			input.AdminUserID,
			verifiedAt,
			input.ExpiresAt,
			input.FailureReason,
			now,
		))
	} else {
		saved, err = scanStoredSourceAuthorVerification(tx.QueryRow(ctx, `
			INSERT INTO source_author_verifications (
			  resource_type, resource_id, source_url,
			  expected_external_user_id, actual_external_user_id, status,
			  verification_method, verified_by_admin_id, verified_at, expires_at,
			  failure_reason, created_at, updated_at, version
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $12, 1)
			RETURNING `+sourceAuthorVerificationColumns,
			input.ResourceType,
			input.ResourceID,
			resource.SourceURL,
			resource.ExpectedExternalUserID,
			input.ActualExternalUserID,
			input.Status,
			input.VerificationMethod,
			input.AdminUserID,
			verifiedAt,
			input.ExpiresAt,
			input.FailureReason,
			now,
		))
	}
	if err != nil {
		return reputation.SourceAuthorVerificationAudit{}, internalStoreError()
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO source_author_verification_events (
		  verification_id, resource_type, resource_id, action, from_status, to_status,
		  source_url, expected_external_user_id, actual_external_user_id,
		  verification_method, verified_by_admin_id, verified_at, expires_at,
		  failure_reason, version, created_at
		)
		VALUES (
		  $1, $2, $3, $4, $5, $6,
		  $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
		)
	`,
		saved.ID,
		saved.ResourceType,
		saved.ResourceID,
		action,
		fromStatus,
		saved.Status,
		saved.SourceURL,
		saved.ExpectedExternalUserID,
		saved.ActualExternalUserID,
		saved.VerificationMethod,
		saved.VerifiedByAdminID,
		saved.VerifiedAt,
		saved.ExpiresAt,
		saved.FailureReason,
		saved.Version,
		now,
	); err != nil {
		return reputation.SourceAuthorVerificationAudit{}, internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return reputation.SourceAuthorVerificationAudit{}, internalStoreError()
	}
	return s.GetSourceAuthorVerificationAudit(ctx, input.ResourceType, input.ResourceID, now)
}

type sourceAuthorResource struct {
	OwnerUserID            string
	SourceURL              string
	ExpectedExternalUserID string
}

type sourceAuthorQueryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func loadSourceAuthorResource(
	ctx context.Context,
	q sourceAuthorQueryer,
	resourceType string,
	resourceID string,
	forUpdate bool,
) (sourceAuthorResource, *domain.AppError) {
	table := "carpool_listings"
	if resourceType == reputation.SourceResourceAPIService {
		table = "api_services"
	}
	query := `
		SELECT resource.owner_user_id::text,
		       COALESCE(resource.source_url, ''),
		       COALESCE(binding.linux_do_user_id, '')
		FROM ` + table + ` resource
		LEFT JOIN linux_do_bindings binding ON binding.user_id = resource.owner_user_id
		WHERE resource.id = $1
	`
	if forUpdate {
		query += ` FOR UPDATE OF resource`
	}
	var result sourceAuthorResource
	err := q.QueryRow(ctx, query, resourceID).Scan(
		&result.OwnerUserID,
		&result.SourceURL,
		&result.ExpectedExternalUserID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return sourceAuthorResource{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Resource not found", "待验证资源不存在。")
	}
	if err != nil {
		return sourceAuthorResource{}, internalStoreError()
	}
	return result, nil
}

func loadSourceAuthorVerification(
	ctx context.Context,
	q sourceAuthorQueryer,
	resourceType string,
	resourceID string,
) (reputation.SourceAuthorVerification, bool, *domain.AppError) {
	value, err := scanStoredSourceAuthorVerification(q.QueryRow(ctx, `
		SELECT `+sourceAuthorVerificationColumns+`
		FROM source_author_verifications
		WHERE resource_type = $1 AND resource_id = $2
	`, resourceType, resourceID))
	if errors.Is(err, pgx.ErrNoRows) {
		return reputation.SourceAuthorVerification{}, false, nil
	}
	if err != nil {
		return reputation.SourceAuthorVerification{}, false, internalStoreError()
	}
	return value, true, nil
}

func loadSourceAuthorVerificationForUpdate(
	ctx context.Context,
	q sourceAuthorQueryer,
	resourceType string,
	resourceID string,
) (reputation.SourceAuthorVerification, bool, *domain.AppError) {
	value, err := scanStoredSourceAuthorVerification(q.QueryRow(ctx, `
		SELECT `+sourceAuthorVerificationColumns+`
		FROM source_author_verifications
		WHERE resource_type = $1 AND resource_id = $2
		FOR UPDATE
	`, resourceType, resourceID))
	if errors.Is(err, pgx.ErrNoRows) {
		return reputation.SourceAuthorVerification{}, false, nil
	}
	if err != nil {
		return reputation.SourceAuthorVerification{}, false, internalStoreError()
	}
	return value, true, nil
}

func scanStoredSourceAuthorVerification(row scanner) (reputation.SourceAuthorVerification, error) {
	var value reputation.SourceAuthorVerification
	var createdAt, updatedAt time.Time
	err := row.Scan(
		&value.ID,
		&value.ResourceType,
		&value.ResourceID,
		&value.SourceURL,
		&value.ExpectedExternalUserID,
		&value.ActualExternalUserID,
		&value.Status,
		&value.VerificationMethod,
		&value.VerifiedByAdminID,
		&value.VerifiedAt,
		&value.ExpiresAt,
		&value.FailureReason,
		&createdAt,
		&updatedAt,
		&value.Version,
	)
	if err == nil {
		value.CreatedAt = &createdAt
		value.UpdatedAt = &updatedAt
	}
	return value, err
}

func effectiveSourceAuthorVerification(
	value reputation.SourceAuthorVerification,
	found bool,
	resource sourceAuthorResource,
	now time.Time,
) reputation.SourceAuthorVerification {
	if !found {
		return reputation.SourceAuthorVerification{
			ResourceType:           value.ResourceType,
			ResourceID:             value.ResourceID,
			OwnerUserID:            resource.OwnerUserID,
			SourceURL:              resource.SourceURL,
			ExpectedExternalUserID: resource.ExpectedExternalUserID,
			Status:                 reputation.SourceVerificationNotSubmitted,
			Version:                0,
		}
	}
	value.OwnerUserID = resource.OwnerUserID
	if strings.TrimSpace(resource.SourceURL) == "" {
		value.SourceURL = ""
		value.ExpectedExternalUserID = resource.ExpectedExternalUserID
		value.Status = reputation.SourceVerificationNotSubmitted
		value.VerifiedAt = nil
		value.ExpiresAt = nil
		return value
	}
	if value.SourceURL != resource.SourceURL || value.ExpectedExternalUserID != resource.ExpectedExternalUserID {
		value.SourceURL = resource.SourceURL
		value.ExpectedExternalUserID = resource.ExpectedExternalUserID
		value.Status = reputation.SourceVerificationPending
		value.VerifiedAt = nil
		value.ExpiresAt = nil
		return value
	}
	if value.Status == reputation.SourceVerificationVerified &&
		value.ExpiresAt != nil &&
		!now.Before(*value.ExpiresAt) {
		value.Status = reputation.SourceVerificationExpired
	}
	return value
}

func listSourceAuthorVerificationEvents(
	ctx context.Context,
	q sourceAuthorQueryer,
	resourceType string,
	resourceID string,
) ([]reputation.SourceAuthorVerificationEvent, *domain.AppError) {
	rows, err := q.Query(ctx, `
		SELECT id::text, verification_id::text, resource_type, resource_id::text,
		       action, from_status, to_status, source_url,
		       expected_external_user_id, actual_external_user_id,
		       verification_method, verified_by_admin_id::text,
		       verified_at, expires_at, failure_reason, version, created_at
		FROM source_author_verification_events
		WHERE resource_type = $1 AND resource_id = $2
		ORDER BY created_at DESC, version DESC, id DESC
	`, resourceType, resourceID)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	events := []reputation.SourceAuthorVerificationEvent{}
	for rows.Next() {
		var event reputation.SourceAuthorVerificationEvent
		if err := rows.Scan(
			&event.ID,
			&event.VerificationID,
			&event.ResourceType,
			&event.ResourceID,
			&event.Action,
			&event.FromStatus,
			&event.ToStatus,
			&event.SourceURL,
			&event.ExpectedExternalUserID,
			&event.ActualExternalUserID,
			&event.VerificationMethod,
			&event.VerifiedByAdminID,
			&event.VerifiedAt,
			&event.ExpiresAt,
			&event.FailureReason,
			&event.Version,
			&event.CreatedAt,
		); err != nil {
			return nil, internalStoreError()
		}
		events = append(events, event)
	}
	if rows.Err() != nil {
		return nil, internalStoreError()
	}
	return events, nil
}

func validateSourceAuthorIdentityDecision(
	input reputation.UpdateSourceAuthorVerificationInput,
	resource sourceAuthorResource,
) *domain.AppError {
	if input.Status != reputation.SourceVerificationVerified &&
		input.Status != reputation.SourceVerificationMismatch {
		return nil
	}
	if strings.TrimSpace(resource.ExpectedExternalUserID) == "" {
		return sourceAuthorStoreFieldError("expectedExternalUserId", "资源所有者尚未绑定 linux.do，不能完成作者核验。")
	}
	actual := strings.TrimSpace(input.ActualExternalUserID)
	expected := strings.TrimSpace(resource.ExpectedExternalUserID)
	if input.Status == reputation.SourceVerificationVerified && actual != expected {
		return sourceAuthorStoreFieldError("actualExternalUserId", "实际 linux.do 用户 ID 与资源所有者绑定身份不一致。")
	}
	if input.Status == reputation.SourceVerificationMismatch && actual == expected {
		return sourceAuthorStoreFieldError("actualExternalUserId", "作者 ID 与绑定身份一致，不能标记为不匹配。")
	}
	return nil
}

func sourceAuthorDecisionTime(status string, now time.Time) *time.Time {
	switch status {
	case reputation.SourceVerificationVerified,
		reputation.SourceVerificationMismatch,
		reputation.SourceVerificationExpired:
		value := now
		return &value
	default:
		return nil
	}
}

func sourceAuthorStoreFieldError(field, detail string) *domain.AppError {
	return domain.NewFieldError(
		http.StatusUnprocessableEntity,
		domain.CodeValidationFailed,
		"Source author verification validation failed",
		detail,
		field,
		"invalid",
		detail,
	)
}

func earliestSourceTimestamp(left, right *time.Time) *time.Time {
	if left == nil {
		return right
	}
	if right == nil || left.Before(*right) {
		return left
	}
	return right
}
