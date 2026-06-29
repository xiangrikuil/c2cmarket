package postgres

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/carpool"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/review"

	"github.com/jackc/pgx/v5"
)

func (s *Store) ListMyReviewCenterRows(ctx context.Context, userID string) ([]review.ReviewCenterRow, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	rows, err := s.pool.Query(ctx, reviewCenterRowsSQL+` ORDER BY updated_at DESC`, userID)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return scanReviewCenterRows(rows)
}

func (s *Store) UpsertCarpoolReviewWithIdempotency(ctx context.Context, entry idempotency.Entry, input review.SubmitReviewInput, now time.Time, buildCompletion review.CompletionBuilder) (review.MutationResult, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return review.MutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	if appErr := validateReviewStoreInput(input); appErr != nil {
		return review.MutationResult{}, idempotency.Completion{}, appErr
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return review.MutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return review.MutationResult{}, idempotency.Completion{}, appErr
	}
	membership, listingTitle, ownerUsername, ownerDisplayName, appErr := lockCompletedCarpoolMembershipForReview(ctx, tx, input)
	if appErr != nil {
		return review.MutationResult{}, idempotency.Completion{}, appErr
	}
	item, appErr := upsertCarpoolReviewInTx(ctx, tx, membership, input, now)
	if appErr != nil {
		return review.MutationResult{}, idempotency.Completion{}, appErr
	}
	result := review.MutationResult{
		Row: reviewCenterRowFromReview(item, listingTitle, ownerUsername, ownerDisplayName),
	}
	completion, appErr := buildCompletion(result)
	if appErr != nil {
		return review.MutationResult{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return review.MutationResult{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return review.MutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	return result, completion, nil
}

func (s *Store) ListPublicUserReviews(ctx context.Context, username string) ([]review.PublicReview, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	username = strings.TrimSpace(strings.ToLower(username))
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1 AND account_status = 'active')`, username).Scan(&exists); err != nil {
		return nil, internalStoreError()
	}
	if !exists {
		return nil, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Profile not found", "公开主页不存在。")
	}
	rows, err := s.pool.Query(ctx, `
		SELECT r.id::text,
		       u.username,
		       r.updated_at,
		       l.title,
		       r.rating,
		       r.tags,
		       r.note
		FROM carpool_reviews r
		JOIN users u ON u.id = r.reviewee_user_id
		JOIN carpool_memberships m ON m.id = r.source_id AND r.source_type = 'carpool_membership'
		JOIN carpool_listings l ON l.id = m.carpool_listing_id
		WHERE u.username = $1
		  AND u.account_status = 'active'
		  AND m.status = 'completed'
		ORDER BY r.updated_at DESC
	`, username)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	items := []review.PublicReview{}
	for rows.Next() {
		var item review.PublicReview
		if err := rows.Scan(&item.ID, &item.Username, &item.Date, &item.ServiceType, &item.Rating, &item.Tags, &item.Note); err != nil {
			return nil, internalStoreError()
		}
		item.Verified = true
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, internalStoreError()
	}
	return items, nil
}

func lockCompletedCarpoolMembershipForReview(ctx context.Context, tx pgx.Tx, input review.SubmitReviewInput) (carpool.Membership, string, string, string, *domain.AppError) {
	var membership carpool.Membership
	var listingTitle string
	var ownerUsername string
	var ownerDisplayName string
	err := tx.QueryRow(ctx, `
		SELECT `+reviewCarpoolMembershipColumns+`,
		       l.title,
		       owner.username,
		       owner.display_name
		FROM carpool_memberships
		JOIN carpool_listings l ON l.id = carpool_memberships.carpool_listing_id
		JOIN users owner ON owner.id = carpool_memberships.owner_user_id
		WHERE carpool_memberships.id = $1
		FOR UPDATE OF carpool_memberships
	`, input.SourceID).Scan(
		&membership.ID,
		&membership.CarpoolListingID,
		&membership.CarpoolApplicationID,
		&membership.CycleTermID,
		&membership.BuyerUserID,
		&membership.OwnerUserID,
		&membership.ProductPlanID,
		&membership.Status,
		&membership.SeatCount,
		&membership.PriceMonthlyCNY,
		&membership.PolicyVersionSnapshot,
		&membership.RiskNoticeCode,
		&membership.JoinedAt,
		&membership.BuyerCompletedAt,
		&membership.OwnerCompletedAt,
		&membership.CompletedAt,
		&membership.EndedAt,
		&membership.EndedReason,
		&membership.EndedByUserID,
		&membership.CreatedAt,
		&membership.UpdatedAt,
		&membership.Version,
		&listingTitle,
		&ownerUsername,
		&ownerDisplayName,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return carpool.Membership{}, "", "", "", domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool membership not found", "已完成拼车成员关系不存在。")
	}
	if err != nil {
		return carpool.Membership{}, "", "", "", internalStoreError()
	}
	if membership.BuyerUserID != input.ReviewerUserID {
		return carpool.Membership{}, "", "", "", domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool membership not found", "已完成拼车成员关系不存在。")
	}
	if membership.Status != carpool.MembershipStatusCompleted {
		return carpool.Membership{}, "", "", "", domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "只能评价已完成的拼车成员关系。")
	}
	return membership, listingTitle, ownerUsername, ownerDisplayName, nil
}

func upsertCarpoolReviewInTx(ctx context.Context, tx pgx.Tx, membership carpool.Membership, input review.SubmitReviewInput, now time.Time) (review.Review, *domain.AppError) {
	var item review.Review
	err := tx.QueryRow(ctx, `
		INSERT INTO carpool_reviews (
			source_type, source_id, reviewer_user_id, reviewee_user_id,
			reviewer_role, reviewee_role, rating, tags, note, created_at, updated_at
		)
		VALUES ('carpool_membership', $1, $2, $3, 'buyer', 'owner', $4, $5, $6, $7, $7)
		ON CONFLICT (source_type, source_id, reviewer_user_id) DO UPDATE
		SET reviewee_user_id = EXCLUDED.reviewee_user_id,
		    rating = EXCLUDED.rating,
		    tags = EXCLUDED.tags,
		    note = EXCLUDED.note,
		    updated_at = EXCLUDED.updated_at
		RETURNING id::text, source_type, source_id::text, reviewer_user_id::text, reviewee_user_id::text,
		          reviewer_role, reviewee_role, rating, tags, note, created_at, updated_at
	`, membership.ID, input.ReviewerUserID, membership.OwnerUserID, input.Rating, normalizeReviewTags(input.Tags), strings.TrimSpace(input.Note), now).Scan(
		&item.ID,
		&item.SourceType,
		&item.SourceID,
		&item.ReviewerUserID,
		&item.RevieweeUserID,
		&item.ReviewerRole,
		&item.RevieweeRole,
		&item.Rating,
		&item.Tags,
		&item.Note,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err != nil {
		return review.Review{}, internalStoreError()
	}
	return item, nil
}

func reviewCenterRowFromReview(item review.Review, listingTitle, ownerUsername, ownerDisplayName string) review.ReviewCenterRow {
	return review.ReviewCenterRow{
		ID:                   item.ID,
		SourceType:           item.SourceType,
		SourceID:             item.SourceID,
		Target:               listingTitle,
		CounterpartyUsername: ownerUsername,
		CounterpartyName:     strings.TrimSpace(ownerDisplayName),
		Status:               review.StatusReviewed,
		Rating:               item.Rating,
		Tags:                 append([]string{}, item.Tags...),
		Note:                 item.Note,
		CreatedAt:            item.CreatedAt,
		UpdatedAt:            item.UpdatedAt,
	}
}

func scanReviewCenterRows(rows pgx.Rows) ([]review.ReviewCenterRow, *domain.AppError) {
	items := []review.ReviewCenterRow{}
	for rows.Next() {
		var item review.ReviewCenterRow
		if err := rows.Scan(
			&item.ID,
			&item.SourceType,
			&item.SourceID,
			&item.Target,
			&item.CounterpartyUsername,
			&item.CounterpartyName,
			&item.Status,
			&item.Rating,
			&item.Tags,
			&item.Note,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, internalStoreError()
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, internalStoreError()
	}
	return items, nil
}

func validateReviewStoreInput(input review.SubmitReviewInput) *domain.AppError {
	if strings.TrimSpace(input.SourceID) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Review validation failed", "必须提供评价来源。", "sourceId", "required", "必须提供评价来源。")
	}
	if input.Rating < 1 || input.Rating > 5 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Review validation failed", "评分必须在 1-5 分之间。", "rating", "invalid", "评分必须在 1-5 分之间。")
	}
	tags := normalizeReviewTags(input.Tags)
	if len(tags) > 5 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Review validation failed", "体验标签最多 5 个。", "tags", "too_many", "体验标签最多 5 个。")
	}
	for _, tag := range tags {
		if utf8.RuneCountInString(tag) > 16 {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Review validation failed", "单个体验标签最多 16 字。", "tags", "too_long", "单个体验标签最多 16 字。")
		}
		if storeLooksLikeSecret(tag) {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeSecretContentDetected, "Secret content detected", "不能在评价中填写、粘贴或上传任何凭据。", "tags", "secret_content", "不能包含密码、API Key、Token、Session、Cookie 或恢复码。")
		}
	}
	note := strings.TrimSpace(input.Note)
	if note == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Review validation failed", "评价说明不能为空。", "note", "required", "评价说明不能为空。")
	}
	if utf8.RuneCountInString(note) > 600 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Review validation failed", "评价说明最多 600 字。", "note", "too_long", "评价说明最多 600 字。")
	}
	if storeLooksLikeSecret(note) {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeSecretContentDetected, "Secret content detected", "不能在评价中填写、粘贴或上传任何凭据。", "note", "secret_content", "不能包含密码、API Key、Token、Session、Cookie 或恢复码。")
	}
	return nil
}

func normalizeReviewTags(tags []string) []string {
	result := make([]string, 0, len(tags))
	seen := map[string]struct{}{}
	for _, tag := range tags {
		value := strings.TrimSpace(tag)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

const reviewCenterRowsSQL = `
WITH completed_memberships AS (
	SELECT
		m.id,
		m.buyer_user_id,
		m.owner_user_id,
		COALESCE(m.ended_at, m.updated_at) AS completed_sort_at,
		l.title,
		owner.username AS owner_username,
		owner.display_name AS owner_display_name
	FROM carpool_memberships m
	JOIN carpool_listings l ON l.id = m.carpool_listing_id
	JOIN users owner ON owner.id = m.owner_user_id
	WHERE m.buyer_user_id = $1
	  AND m.status = 'completed'
)
SELECT
	COALESCE(r.id::text, 'review-carpool-membership-' || m.id::text) AS id,
	'carpool_membership' AS source_type,
	m.id::text AS source_id,
	m.title AS target,
	m.owner_username AS counterparty_username,
	m.owner_display_name AS counterparty_name,
	CASE WHEN r.id IS NULL THEN 'reviewable' ELSE 'reviewed' END AS status,
	COALESCE(r.rating, 0) AS rating,
	COALESCE(r.tags, '{}'::text[]) AS tags,
	COALESCE(r.note, '') AS note,
	COALESCE(r.created_at, m.completed_sort_at) AS created_at,
	COALESCE(r.updated_at, m.completed_sort_at) AS updated_at
FROM completed_memberships m
LEFT JOIN carpool_reviews r
  ON r.source_type = 'carpool_membership'
 AND r.source_id = m.id
 AND r.reviewer_user_id = m.buyer_user_id`

const reviewCarpoolMembershipColumns = `
	carpool_memberships.id::text, carpool_memberships.carpool_listing_id::text, carpool_memberships.carpool_application_id::text, COALESCE(carpool_memberships.cycle_term_id::text, ''), carpool_memberships.buyer_user_id::text,
	carpool_memberships.owner_user_id::text, carpool_memberships.product_plan_id::text, carpool_memberships.status, carpool_memberships.seat_count,
	carpool_memberships.price_monthly_cny_snapshot::text, carpool_memberships.policy_version_snapshot, COALESCE(carpool_memberships.risk_notice_code_snapshot, ''),
	carpool_memberships.joined_at,
	(SELECT confirmed_at FROM carpool_completion_confirmations WHERE carpool_membership_id = carpool_memberships.id AND actor_role = 'buyer') AS buyer_completed_at,
	(SELECT confirmed_at FROM carpool_completion_confirmations WHERE carpool_membership_id = carpool_memberships.id AND actor_role = 'owner') AS owner_completed_at,
	CASE WHEN carpool_memberships.status = 'completed' THEN carpool_memberships.ended_at ELSE NULL END AS completed_at,
	carpool_memberships.ended_at, carpool_memberships.ended_reason, COALESCE(carpool_memberships.ended_by_user_id::text, ''),
	carpool_memberships.created_at, carpool_memberships.updated_at, carpool_memberships.version
`
