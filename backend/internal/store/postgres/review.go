package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/review"

	"github.com/jackc/pgx/v5"
)

func (s *Store) ResolveTransactionForReview(ctx context.Context, transactionType, transactionID, userID string) (review.Transaction, *domain.AppError) {
	if s == nil || s.pool == nil {
		return review.Transaction{}, internalStoreError()
	}
	return resolveTransactionForReview(ctx, s.pool, transactionType, transactionID, userID, false)
}

func (s *Store) ListMyReviewCenterRows(ctx context.Context, userID string, now time.Time) ([]review.ReviewCenterRow, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rollback(ctx, tx)
	if appErr := materializeExpiredTransactionReviewsInTx(ctx, tx, userID, now); appErr != nil {
		return nil, appErr
	}
	rows, err := tx.Query(ctx, reviewCenterRowsSQL, userID, now)
	if err != nil {
		return nil, internalStoreError()
	}
	items, appErr := scanReviewCenterRows(rows)
	rows.Close()
	if appErr != nil {
		return nil, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, internalStoreError()
	}
	return items, nil
}

func (s *Store) SaveTransactionReviewWithIdempotency(ctx context.Context, entry idempotency.Entry, transaction review.Transaction, input review.SubmitReviewInput, now time.Time, buildCompletion review.CompletionBuilder) (review.MutationResult, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return review.MutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	if appErr := review.ValidateSubmitInput(input); appErr != nil {
		return review.MutationResult{}, idempotency.Completion{}, appErr
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return review.MutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existingEntry, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return review.MutationResult{}, idempotency.Completion{}, appErr
	}
	lockedTransaction, appErr := resolveTransactionForReview(ctx, tx, input.TransactionType, input.TransactionID, input.ReviewerUserID, true)
	if appErr != nil {
		return review.MutationResult{}, idempotency.Completion{}, appErr
	}
	if lockedTransaction.ID != transaction.ID || !now.Before(lockedTransaction.ReviewDeadlineAt) {
		return review.MutationResult{}, idempotency.Completion{}, reviewWindowClosedStoreError()
	}
	if lockedTransaction.ReviewPaused {
		return review.MutationResult{}, idempotency.Completion{}, reviewPausedStoreError()
	}

	current, currentFound, appErr := lockTransactionReviewForReviewer(ctx, tx, input.TransactionType, input.TransactionID, input.ReviewerUserID)
	if appErr != nil {
		return review.MutationResult{}, idempotency.Completion{}, appErr
	}
	switch input.Operation {
	case review.OperationCreate:
		if currentFound {
			return review.MutationResult{}, idempotency.Completion{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Review already exists", "该交易已提交评价；公开前请使用修改操作。")
		}
	case review.OperationEdit:
		if !currentFound {
			return review.MutationResult{}, idempotency.Completion{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Review not found", "待修改评价不存在。")
		}
	case review.OperationLegacyUpsert:
	default:
		return review.MutationResult{}, idempotency.Completion{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Review validation failed", "评价操作不受支持。", "operation", "invalid", "评价操作不受支持。")
	}
	if currentFound && (current.Status != review.StatusSealed || current.FrozenAt != nil || !now.Before(current.ReviewDeadlineAt)) {
		return review.MutationResult{}, idempotency.Completion{}, reviewFrozenStoreError()
	}

	saved, appErr := saveTransactionReviewInTx(ctx, tx, lockedTransaction, current, currentFound, input, now)
	if appErr != nil {
		return review.MutationResult{}, idempotency.Completion{}, appErr
	}
	action := "created"
	var before *review.Review
	if currentFound {
		action = "edited"
		before = &current
	}
	if appErr := insertTransactionReviewRevisionInTx(ctx, tx, saved, action, input.ReviewerUserID, "", before); appErr != nil {
		return review.MutationResult{}, idempotency.Completion{}, appErr
	}

	counterparty, counterpartyFound, appErr := lockCounterpartyTransactionReview(ctx, tx, saved)
	if appErr != nil {
		return review.MutationResult{}, idempotency.Completion{}, appErr
	}
	if counterpartyFound && counterparty.Status == review.StatusSealed {
		savedBeforePublish := saved
		counterpartyBeforePublish := counterparty
		saved, appErr = publishTransactionReviewInTx(ctx, tx, saved.ID, now)
		if appErr != nil {
			return review.MutationResult{}, idempotency.Completion{}, appErr
		}
		counterparty, appErr = publishTransactionReviewInTx(ctx, tx, counterparty.ID, now)
		if appErr != nil {
			return review.MutationResult{}, idempotency.Completion{}, appErr
		}
		if appErr := insertTransactionReviewRevisionInTx(ctx, tx, saved, "published", input.ReviewerUserID, "both_parties_submitted", &savedBeforePublish); appErr != nil {
			return review.MutationResult{}, idempotency.Completion{}, appErr
		}
		if appErr := insertTransactionReviewRevisionInTx(ctx, tx, counterparty, "published", input.ReviewerUserID, "both_parties_submitted", &counterpartyBeforePublish); appErr != nil {
			return review.MutationResult{}, idempotency.Completion{}, appErr
		}
	}

	result := review.MutationResult{Row: reviewCenterRowFromSaved(lockedTransaction, saved, now)}
	completion, appErr := buildCompletion(result)
	if appErr != nil {
		return review.MutationResult{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existingEntry, completion, now); appErr != nil {
		return review.MutationResult{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return review.MutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	return result, completion, nil
}

func (s *Store) RemoveTransactionReviewWithIdempotency(ctx context.Context, entry idempotency.Entry, input review.RemoveReviewInput, now time.Time, buildCompletion review.CompletionBuilder) (review.MutationResult, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return review.MutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return review.MutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existingEntry, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return review.MutationResult{}, idempotency.Completion{}, appErr
	}
	item, found, appErr := lockTransactionReviewByID(ctx, tx, input.ReviewID)
	if appErr != nil {
		return review.MutationResult{}, idempotency.Completion{}, appErr
	}
	if !found {
		return review.MutationResult{}, idempotency.Completion{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Review not found", "评价不存在。")
	}
	if item.Version != input.ExpectedVersion {
		return review.MutationResult{}, idempotency.Completion{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "评价已更新，请刷新后重试。")
	}
	if item.Status != review.StatusPublished || item.FrozenAt == nil {
		return review.MutationResult{}, idempotency.Completion{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "只能移除已公开评价。")
	}
	before := item
	err = tx.QueryRow(ctx, `
		UPDATE transaction_reviews
		SET status = 'removed',
		    removed_at = $2,
		    removed_by_admin_id = $3,
		    removal_reason = $4,
		    updated_at = $2,
		    version = version + 1
		WHERE id = $1
		RETURNING `+transactionReviewColumns+`
	`, item.ID, now, input.AdminUserID, input.Reason).Scan(transactionReviewScanTargets(&item)...)
	if err != nil {
		return review.MutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	if appErr := insertTransactionReviewRevisionInTx(ctx, tx, item, "removed", input.AdminUserID, input.Reason, &before); appErr != nil {
		return review.MutationResult{}, idempotency.Completion{}, appErr
	}

	result := review.MutationResult{Row: reviewCenterRowFromRemoved(item)}
	completion, appErr := buildCompletion(result)
	if appErr != nil {
		return review.MutationResult{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existingEntry, completion, now); appErr != nil {
		return review.MutationResult{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return review.MutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	return result, completion, nil
}

func (s *Store) ListPublicUserReviews(ctx context.Context, username string, now time.Time) ([]review.PublicReview, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	username = strings.TrimSpace(strings.ToLower(username))
	var userID string
	err := s.pool.QueryRow(ctx, `
		SELECT id::text
		FROM users
		WHERE username = $1
		  AND account_status = 'active'
	`, username).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Profile not found", "公开主页不存在。")
	}
	if err != nil {
		return nil, internalStoreError()
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rollback(ctx, tx)
	if appErr := materializeExpiredTransactionReviewsInTx(ctx, tx, userID, now); appErr != nil {
		return nil, appErr
	}
	rows, err := tx.Query(ctx, `
		SELECT review.id::text,
		       reviewer.username,
		       review.visible_at,
		       api_order.service_title_snapshot,
		       review.transaction_type,
		       review.reviewer_role,
		       review.reviewee_role,
		       review.rating,
		       review.tags,
		       review.note
		FROM transaction_reviews review
		JOIN users reviewer ON reviewer.id = review.reviewer_user_id
		JOIN api_orders api_order ON api_order.id = review.api_order_id
		WHERE review.reviewee_user_id = $1
		  AND review.transaction_type = 'api_order'
		  AND review.status = 'published'
		  AND NOT EXISTS (
		    SELECT 1
		    FROM reputation_transaction_exclusions exclusion
		    WHERE exclusion.transaction_type = review.transaction_type
		      AND exclusion.transaction_id = review.api_order_id
		      AND exclusion.restored_at IS NULL
		  )
		ORDER BY review.visible_at DESC, review.id DESC
	`, userID)
	if err != nil {
		return nil, internalStoreError()
	}
	items := []review.PublicReview{}
	for rows.Next() {
		var item review.PublicReview
		if err := rows.Scan(
			&item.ID,
			&item.ReviewerUsername,
			&item.Date,
			&item.ServiceType,
			&item.TransactionType,
			&item.ReviewerRole,
			&item.RevieweeRole,
			&item.Rating,
			&item.Tags,
			&item.Note,
		); err != nil {
			rows.Close()
			return nil, internalStoreError()
		}
		item.Verified = true
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, internalStoreError()
	}
	rows.Close()
	if err := tx.Commit(ctx); err != nil {
		return nil, internalStoreError()
	}
	return items, nil
}

func resolveTransactionForReview(ctx context.Context, q queryer, transactionType, transactionID, userID string, forUpdate bool) (review.Transaction, *domain.AppError) {
	transactionType = strings.TrimSpace(transactionType)
	transactionID = strings.TrimSpace(transactionID)
	userID = strings.TrimSpace(userID)
	if transactionType != review.TransactionAPIOrder {
		return review.Transaction{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Review validation failed", "交易类型必须是 api_order。", "type", "invalid", "拼车不参与公开评价。")
	}

	var transaction review.Transaction
	var status string
	var completedAt *time.Time
	lockClause := ""
	if forUpdate {
		lockClause = " FOR UPDATE"
	}
	err := q.QueryRow(ctx, `
			SELECT 'api_order',
			       api_order.id::text,
			       api_order.service_title_snapshot,
			       buyer.id::text,
			       buyer.username,
			       buyer.display_name,
			       seller.id::text,
			       seller.username,
			       seller.display_name,
			       api_order.commercial_outcome,
			       api_order.commercial_outcome_updated_at,
			       EXISTS (
			         SELECT 1
			         FROM dispute_cases dispute
			         WHERE dispute.api_order_id = api_order.id AND dispute.active = true
			       )
			FROM api_orders api_order
			JOIN users buyer ON buyer.id = api_order.buyer_user_id
			JOIN users seller ON seller.id = api_order.seller_user_id
			WHERE api_order.id = $1
			  AND $2 IN (api_order.buyer_user_id, api_order.seller_user_id)
		`+lockClause, transactionID, userID).Scan(
		&transaction.Type,
		&transaction.ID,
		&transaction.Target,
		&transaction.BuyerUserID,
		&transaction.BuyerUsername,
		&transaction.BuyerDisplayName,
		&transaction.SellerUserID,
		&transaction.SellerUsername,
		&transaction.SellerDisplayName,
		&status,
		&completedAt,
		&transaction.ReviewPaused,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return review.Transaction{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Transaction not found", "可评价交易不存在。")
	}
	if err != nil {
		return review.Transaction{}, internalStoreError()
	}
	if transactionType == review.TransactionCarpoolMembership {
		if status != "completed" || completedAt == nil {
			return review.Transaction{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "只能评价已完成交易。")
		}
	} else {
		if !review.IsReviewableAPIOrderOutcome(status) || completedAt == nil {
			return review.Transaction{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前 API 订单商业结果不能评价。")
		}
		transaction.CommercialOutcome = status
	}
	var excluded bool
	if err := q.QueryRow(ctx, `
		SELECT EXISTS (
		  SELECT 1
		  FROM reputation_transaction_exclusions
		  WHERE transaction_type = $1
		    AND transaction_id = $2
		    AND restored_at IS NULL
		)
	`, transactionType, transactionID).Scan(&excluded); err != nil {
		return review.Transaction{}, internalStoreError()
	}
	if excluded {
		return review.Transaction{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Transaction excluded", "该交易已排除信誉统计，不能评价。")
	}
	transaction.CompletedAt = *completedAt
	transaction.ReviewDeadlineAt = completedAt.Add(review.ReviewWindow)
	return transaction, nil
}

func saveTransactionReviewInTx(ctx context.Context, tx pgx.Tx, transaction review.Transaction, current review.Review, currentFound bool, input review.SubmitReviewInput, now time.Time) (review.Review, *domain.AppError) {
	reviewerRole := review.RoleBuyer
	revieweeRole := review.RoleSeller
	revieweeUserID := transaction.SellerUserID
	if input.ReviewerUserID == transaction.SellerUserID {
		reviewerRole = review.RoleSeller
		revieweeRole = review.RoleBuyer
		revieweeUserID = transaction.BuyerUserID
	}
	tags := review.NormalizeTags(input.Tags)
	note := strings.TrimSpace(input.Note)
	var item review.Review
	var err error
	if currentFound {
		err = tx.QueryRow(ctx, `
			UPDATE transaction_reviews
			SET rating = $2,
			    tags = $3,
			    note = $4,
			    updated_at = $5,
			    version = version + 1
			WHERE id = $1
			RETURNING `+transactionReviewColumns+`
		`, current.ID, input.Rating, tags, note, now).Scan(transactionReviewScanTargets(&item)...)
	} else {
		err = tx.QueryRow(ctx, `
			INSERT INTO transaction_reviews (
			  transaction_type,
			  carpool_membership_id,
			  api_order_id,
			  reviewer_user_id,
			  reviewee_user_id,
			  reviewer_role,
			  reviewee_role,
			  rating,
			  tags,
			  note,
			  status,
			  review_deadline_at,
			  created_at,
			  updated_at,
			  version
			)
			VALUES (
			  $1,
			  CASE WHEN $1 = 'carpool_membership' THEN $2::uuid ELSE NULL END,
			  CASE WHEN $1 = 'api_order' THEN $2::uuid ELSE NULL END,
			  $3,
			  $4,
			  $5,
			  $6,
			  $7,
			  $8,
			  $9,
			  'sealed',
			  $10,
			  $11,
			  $11,
			  1
			)
			RETURNING `+transactionReviewColumns+`
		`, transaction.Type, transaction.ID, input.ReviewerUserID, revieweeUserID, reviewerRole, revieweeRole, input.Rating, tags, note, transaction.ReviewDeadlineAt, now).Scan(transactionReviewScanTargets(&item)...)
	}
	if isUniqueViolation(err) {
		return review.Review{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Review already exists", "该交易已提交评价。")
	}
	if err != nil {
		return review.Review{}, internalStoreError()
	}
	return item, nil
}

func lockTransactionReviewForReviewer(ctx context.Context, tx pgx.Tx, transactionType, transactionID, reviewerUserID string) (review.Review, bool, *domain.AppError) {
	var item review.Review
	err := tx.QueryRow(ctx, `
		SELECT `+transactionReviewColumns+`
		FROM transaction_reviews
		WHERE transaction_type = $1
		  AND COALESCE(carpool_membership_id, api_order_id) = $2
		  AND reviewer_user_id = $3
		FOR UPDATE
	`, transactionType, transactionID, reviewerUserID).Scan(transactionReviewScanTargets(&item)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return review.Review{}, false, nil
	}
	if err != nil {
		return review.Review{}, false, internalStoreError()
	}
	return item, true, nil
}

func lockCounterpartyTransactionReview(ctx context.Context, tx pgx.Tx, item review.Review) (review.Review, bool, *domain.AppError) {
	transactionID := item.CarpoolMembershipID
	if item.TransactionType == review.TransactionAPIOrder {
		transactionID = item.APIOrderID
	}
	var counterparty review.Review
	err := tx.QueryRow(ctx, `
		SELECT `+transactionReviewColumns+`
		FROM transaction_reviews
		WHERE transaction_type = $1
		  AND COALESCE(carpool_membership_id, api_order_id) = $2
		  AND reviewer_user_id <> $3
		FOR UPDATE
	`, item.TransactionType, transactionID, item.ReviewerUserID).Scan(transactionReviewScanTargets(&counterparty)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return review.Review{}, false, nil
	}
	if err != nil {
		return review.Review{}, false, internalStoreError()
	}
	return counterparty, true, nil
}

func lockTransactionReviewByID(ctx context.Context, tx pgx.Tx, reviewID string) (review.Review, bool, *domain.AppError) {
	var item review.Review
	err := tx.QueryRow(ctx, `
		SELECT `+transactionReviewColumns+`
		FROM transaction_reviews
		WHERE id = $1
		FOR UPDATE
	`, reviewID).Scan(transactionReviewScanTargets(&item)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return review.Review{}, false, nil
	}
	if err != nil {
		return review.Review{}, false, internalStoreError()
	}
	return item, true, nil
}

func publishTransactionReviewInTx(ctx context.Context, tx pgx.Tx, reviewID string, visibleAt time.Time) (review.Review, *domain.AppError) {
	var item review.Review
	err := tx.QueryRow(ctx, `
		UPDATE transaction_reviews
		SET status = 'published',
		    visible_at = $2,
		    frozen_at = $2,
		    version = version + 1
		WHERE id = $1
		  AND status = 'sealed'
		RETURNING `+transactionReviewColumns+`
	`, reviewID, visibleAt).Scan(transactionReviewScanTargets(&item)...)
	if errors.Is(err, pgx.ErrNoRows) {
		return review.Review{}, reviewFrozenStoreError()
	}
	if err != nil {
		return review.Review{}, internalStoreError()
	}
	return item, nil
}

func materializeExpiredTransactionReviewsInTx(ctx context.Context, tx pgx.Tx, participantUserID string, now time.Time) *domain.AppError {
	rows, err := tx.Query(ctx, `
		SELECT `+transactionReviewColumns+`
		FROM transaction_reviews
		WHERE status = 'sealed'
		  AND review_deadline_at <= $1
		  AND (
		    transaction_type <> 'api_order'
		    OR EXISTS (
		      SELECT 1
		      FROM api_orders order_row
		      WHERE order_row.id = transaction_reviews.api_order_id
		        AND order_row.commercial_outcome IN ('normal_fulfillment', 'full_refund', 'partial_refund', 'continued_fulfillment')
		        AND order_row.commercial_outcome_updated_at IS NOT NULL
		        AND NOT EXISTS (
		          SELECT 1
		          FROM dispute_cases dispute
		          WHERE dispute.api_order_id = order_row.id AND dispute.active = true
		        )
		    )
		  )
		  AND (
		    $2::text = ''
		    OR reviewer_user_id = NULLIF($2::text, '')::uuid
		    OR reviewee_user_id = NULLIF($2::text, '')::uuid
		  )
		ORDER BY review_deadline_at, id
		FOR UPDATE SKIP LOCKED
	`, now, strings.TrimSpace(participantUserID))
	if err != nil {
		return internalStoreError()
	}
	expired := []review.Review{}
	for rows.Next() {
		var item review.Review
		if err := rows.Scan(transactionReviewScanTargets(&item)...); err != nil {
			rows.Close()
			return internalStoreError()
		}
		expired = append(expired, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return internalStoreError()
	}
	rows.Close()

	for _, before := range expired {
		after, appErr := publishTransactionReviewInTx(ctx, tx, before.ID, before.ReviewDeadlineAt)
		if appErr != nil {
			return appErr
		}
		if appErr := insertTransactionReviewRevisionInTx(ctx, tx, after, "published", "", "review_deadline_elapsed", &before); appErr != nil {
			return appErr
		}
	}
	return nil
}

func refreshMutableAPIOrderReviewsInTx(ctx context.Context, tx pgx.Tx, orderID, commercialOutcome string, commercialOutcomeAt, now time.Time) *domain.AppError {
	if !review.IsReviewableAPIOrderOutcome(commercialOutcome) {
		return nil
	}
	if _, err := tx.Exec(ctx, `
		UPDATE transaction_reviews
		SET commercial_outcome = $2,
		    review_deadline_at = $3,
		    updated_at = $4,
		    version = version + 1
		WHERE api_order_id = $1
		  AND status = 'sealed'
		  AND frozen_at IS NULL
		  AND NOT EXISTS (
		    SELECT 1 FROM dispute_cases dispute
		    WHERE dispute.api_order_id = transaction_reviews.api_order_id AND dispute.active = true
		  )
	`, orderID, commercialOutcome, review.ReviewDeadlineForAPIOrder(commercialOutcomeAt), now); err != nil {
		return internalStoreError()
	}
	return nil
}

func insertTransactionReviewRevisionInTx(ctx context.Context, tx pgx.Tx, item review.Review, action, actorUserID, reason string, before *review.Review) *domain.AppError {
	afterJSON, err := json.Marshal(transactionReviewSnapshot(item))
	if err != nil {
		return internalStoreError()
	}
	var beforeJSON any
	if before != nil {
		data, marshalErr := json.Marshal(transactionReviewSnapshot(*before))
		if marshalErr != nil {
			return internalStoreError()
		}
		beforeJSON = data
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO transaction_review_revisions (
		  transaction_review_id,
		  revision_number,
		  action,
		  actor_user_id,
		  before_snapshot,
		  after_snapshot,
		  reason,
		  created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, item.ID, item.Version, action, nullUUID(actorUserID), beforeJSON, afterJSON, nullText(reason), item.UpdatedAt)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func transactionReviewSnapshot(item review.Review) map[string]any {
	return map[string]any{
		"rating":            item.Rating,
		"tags":              append([]string{}, item.Tags...),
		"note":              item.Note,
		"status":            item.Status,
		"commercialOutcome": item.CommercialOutcome,
		"reviewDeadlineAt":  item.ReviewDeadlineAt,
		"visibleAt":         item.VisibleAt,
		"frozenAt":          item.FrozenAt,
		"removedAt":         item.RemovedAt,
		"removalReason":     item.RemovalReason,
		"version":           item.Version,
	}
}

func reviewCenterRowFromSaved(transaction review.Transaction, item review.Review, now time.Time) review.ReviewCenterRow {
	counterpartyUsername := transaction.SellerUsername
	counterpartyName := transaction.SellerDisplayName
	if item.ReviewerRole == review.RoleSeller {
		counterpartyUsername = transaction.BuyerUsername
		counterpartyName = transaction.BuyerDisplayName
	}
	submittedAt := item.CreatedAt
	return review.ReviewCenterRow{
		ID:                    item.ID,
		TransactionType:       transaction.Type,
		TransactionID:         transaction.ID,
		Direction:             review.DirectionSent,
		Target:                transaction.Target,
		CounterpartyUsername:  counterpartyUsername,
		CounterpartyName:      strings.TrimSpace(counterpartyName),
		ReviewerRole:          item.ReviewerRole,
		RevieweeRole:          item.RevieweeRole,
		Status:                item.Status,
		Visibility:            item.Status,
		CounterpartySubmitted: item.Status == review.StatusPublished,
		CanEdit:               item.Status == review.StatusSealed && !transaction.ReviewPaused && now.Before(item.ReviewDeadlineAt),
		ContentVisible:        item.Status != review.StatusRemoved,
		Rating:                item.Rating,
		Tags:                  append([]string{}, item.Tags...),
		Note:                  item.Note,
		CompletedAt:           transaction.CompletedAt,
		ReviewDeadlineAt:      item.ReviewDeadlineAt,
		CommercialOutcome:     item.CommercialOutcome,
		ReviewPaused:          transaction.ReviewPaused,
		SubmittedAt:           &submittedAt,
		VisibleAt:             item.VisibleAt,
		FrozenAt:              item.FrozenAt,
		CreatedAt:             item.CreatedAt,
		UpdatedAt:             item.UpdatedAt,
		Version:               item.Version,
	}
}

func reviewCenterRowFromRemoved(item review.Review) review.ReviewCenterRow {
	transactionID := item.CarpoolMembershipID
	if item.TransactionType == review.TransactionAPIOrder {
		transactionID = item.APIOrderID
	}
	submittedAt := item.CreatedAt
	return review.ReviewCenterRow{
		ID:               item.ID,
		TransactionType:  item.TransactionType,
		TransactionID:    transactionID,
		Direction:        review.DirectionSent,
		Status:           item.Status,
		Visibility:       review.VisibilityRemoved,
		ReviewerRole:     item.ReviewerRole,
		RevieweeRole:     item.RevieweeRole,
		CompletedAt:      item.CreatedAt,
		ReviewDeadlineAt: item.ReviewDeadlineAt,
		SubmittedAt:      &submittedAt,
		VisibleAt:        item.VisibleAt,
		FrozenAt:         item.FrozenAt,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
		Version:          item.Version,
	}
}

func scanReviewCenterRows(rows pgx.Rows) ([]review.ReviewCenterRow, *domain.AppError) {
	items := []review.ReviewCenterRow{}
	for rows.Next() {
		var item review.ReviewCenterRow
		if err := rows.Scan(
			&item.ID,
			&item.TransactionType,
			&item.TransactionID,
			&item.Direction,
			&item.Target,
			&item.CounterpartyUsername,
			&item.CounterpartyName,
			&item.ReviewerRole,
			&item.RevieweeRole,
			&item.Status,
			&item.Visibility,
			&item.CounterpartySubmitted,
			&item.CanCreate,
			&item.CanEdit,
			&item.ContentVisible,
			&item.Rating,
			&item.Tags,
			&item.Note,
			&item.CompletedAt,
			&item.ReviewDeadlineAt,
			&item.CommercialOutcome,
			&item.ReviewPaused,
			&item.SubmittedAt,
			&item.VisibleAt,
			&item.FrozenAt,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.Version,
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

func transactionReviewScanTargets(item *review.Review) []any {
	return []any{
		&item.ID,
		&item.TransactionType,
		&item.CarpoolMembershipID,
		&item.APIOrderID,
		&item.ReviewerUserID,
		&item.RevieweeUserID,
		&item.ReviewerRole,
		&item.RevieweeRole,
		&item.Rating,
		&item.Tags,
		&item.Note,
		&item.Status,
		&item.ReviewDeadlineAt,
		&item.CommercialOutcome,
		&item.VisibleAt,
		&item.FrozenAt,
		&item.RemovedAt,
		&item.RemovedByAdminID,
		&item.RemovalReason,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.Version,
	}
}

func reviewWindowClosedStoreError() *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Review window closed", "评价窗口已截止。")
}

func reviewPausedStoreError() *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Review paused", "活跃纠纷期间暂停创建、修改和公开评价。")
}

func reviewFrozenStoreError() *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Review is frozen", "评价已公开或已冻结，不能再修改。")
}

const transactionReviewColumns = `
	id::text,
	transaction_type,
	COALESCE(carpool_membership_id::text, ''),
	COALESCE(api_order_id::text, ''),
	reviewer_user_id::text,
	reviewee_user_id::text,
	reviewer_role,
	reviewee_role,
	rating,
	tags,
	note,
	status,
	review_deadline_at,
	COALESCE(commercial_outcome, ''),
	visible_at,
	frozen_at,
	removed_at,
	COALESCE(removed_by_admin_id::text, ''),
	COALESCE(removal_reason, ''),
	created_at,
	updated_at,
	version
`

const reviewCenterRowsSQL = `
WITH my_transactions AS (
  SELECT
    'api_order'::text,
    api_order.id,
    api_order.service_title_snapshot,
    api_order.buyer_user_id,
    buyer.username,
    buyer.display_name,
    api_order.seller_user_id,
    seller.username,
    seller.display_name,
	    api_order.commercial_outcome_updated_at,
	    api_order.commercial_outcome_updated_at + interval '14 days',
	    api_order.commercial_outcome,
	    EXISTS (
	      SELECT 1 FROM dispute_cases dispute
	      WHERE dispute.api_order_id = api_order.id AND dispute.active = true
	    )
  FROM api_orders api_order
  JOIN users buyer ON buyer.id = api_order.buyer_user_id
  JOIN users seller ON seller.id = api_order.seller_user_id
	  WHERE api_order.commercial_outcome IN ('normal_fulfillment', 'full_refund', 'partial_refund', 'continued_fulfillment')
	    AND api_order.commercial_outcome_updated_at IS NOT NULL
    AND $1 IN (api_order.buyer_user_id, api_order.seller_user_id)
    AND NOT EXISTS (
      SELECT 1
      FROM reputation_transaction_exclusions exclusion
      WHERE exclusion.transaction_type = 'api_order'
        AND exclusion.transaction_id = api_order.id
        AND exclusion.restored_at IS NULL
    )
),
transaction_reviews_for_me AS (
  SELECT review.*
  FROM transaction_reviews review
  JOIN my_transactions transaction
    ON transaction.transaction_type = review.transaction_type
   AND transaction.transaction_id = COALESCE(review.carpool_membership_id, review.api_order_id)
)
SELECT *
FROM (
  SELECT
    'reviewable-' || transaction.transaction_type || '-' || transaction.transaction_id::text AS id,
    transaction.transaction_type,
    transaction.transaction_id::text,
    'pending'::text AS direction,
    transaction.target,
    CASE WHEN $1 = transaction.buyer_user_id THEN transaction.seller_username ELSE transaction.buyer_username END,
    CASE WHEN $1 = transaction.buyer_user_id THEN transaction.seller_display_name ELSE transaction.buyer_display_name END,
    CASE WHEN $1 = transaction.buyer_user_id THEN 'buyer' ELSE 'seller' END,
    CASE WHEN $1 = transaction.buyer_user_id THEN 'seller' ELSE 'buyer' END,
	    CASE
	      WHEN transaction.review_paused THEN 'paused'
	      WHEN $2 < transaction.review_deadline_at THEN 'reviewable'
	      ELSE 'expired'
	    END,
    'none'::text AS visibility,
	    false AS counterparty_submitted,
	    (NOT transaction.review_paused AND $2 < transaction.review_deadline_at) AS can_create,
    false AS can_edit,
    false AS content_visible,
    0 AS rating,
    '{}'::text[] AS tags,
    ''::text AS note,
	    transaction.completed_at,
	    transaction.review_deadline_at,
	    transaction.commercial_outcome,
	    transaction.review_paused,
	    NULL::timestamptz AS submitted_at,
    NULL::timestamptz AS visible_at,
    NULL::timestamptz AS frozen_at,
    transaction.completed_at AS created_at,
    transaction.completed_at AS updated_at,
    0::bigint AS version
  FROM my_transactions transaction
  WHERE NOT EXISTS (
    SELECT 1
    FROM transaction_reviews_for_me own_review
    WHERE own_review.transaction_type = transaction.transaction_type
      AND COALESCE(own_review.carpool_membership_id, own_review.api_order_id) = transaction.transaction_id
      AND own_review.reviewer_user_id = $1
  )

  UNION ALL

  SELECT
    own_review.id::text,
    transaction.transaction_type,
    transaction.transaction_id::text,
    'sent',
    transaction.target,
    CASE WHEN $1 = transaction.buyer_user_id THEN transaction.seller_username ELSE transaction.buyer_username END,
    CASE WHEN $1 = transaction.buyer_user_id THEN transaction.seller_display_name ELSE transaction.buyer_display_name END,
    own_review.reviewer_role,
    own_review.reviewee_role,
    own_review.status,
    own_review.status,
	    own_review.status IN ('published', 'removed'),
    false,
	    (own_review.status = 'sealed' AND NOT transaction.review_paused AND $2 < own_review.review_deadline_at),
    (own_review.status <> 'removed'),
    CASE WHEN own_review.status = 'removed' THEN 0 ELSE own_review.rating END,
    CASE WHEN own_review.status = 'removed' THEN '{}'::text[] ELSE own_review.tags END,
    CASE WHEN own_review.status = 'removed' THEN '' ELSE own_review.note END,
	    transaction.completed_at,
	    own_review.review_deadline_at,
	    own_review.commercial_outcome,
	    transaction.review_paused,
	    own_review.created_at,
    own_review.visible_at,
    own_review.frozen_at,
    own_review.created_at,
    own_review.updated_at,
    own_review.version
  FROM my_transactions transaction
  JOIN transaction_reviews_for_me own_review
    ON own_review.transaction_type = transaction.transaction_type
   AND COALESCE(own_review.carpool_membership_id, own_review.api_order_id) = transaction.transaction_id
   AND own_review.reviewer_user_id = $1

  UNION ALL

  SELECT
    received_review.id::text,
    transaction.transaction_type,
    transaction.transaction_id::text,
    'received',
    transaction.target,
    CASE WHEN $1 = transaction.buyer_user_id THEN transaction.seller_username ELSE transaction.buyer_username END,
    CASE WHEN $1 = transaction.buyer_user_id THEN transaction.seller_display_name ELSE transaction.buyer_display_name END,
    received_review.reviewer_role,
    received_review.reviewee_role,
    received_review.status,
    received_review.status,
    true,
    false,
    false,
    (received_review.status = 'published'),
    CASE WHEN received_review.status = 'published' THEN received_review.rating ELSE 0 END,
    CASE WHEN received_review.status = 'published' THEN received_review.tags ELSE '{}'::text[] END,
    CASE WHEN received_review.status = 'published' THEN received_review.note ELSE '' END,
	    transaction.completed_at,
	    received_review.review_deadline_at,
	    received_review.commercial_outcome,
	    transaction.review_paused,
	    received_review.created_at,
    received_review.visible_at,
    received_review.frozen_at,
    received_review.created_at,
    received_review.updated_at,
    received_review.version
  FROM my_transactions transaction
  JOIN transaction_reviews_for_me received_review
    ON received_review.transaction_type = transaction.transaction_type
	   AND COALESCE(received_review.carpool_membership_id, received_review.api_order_id) = transaction.transaction_id
	   AND received_review.reviewee_user_id = $1
	   AND received_review.status <> 'sealed'
) center_rows
ORDER BY completed_at DESC,
         CASE direction WHEN 'pending' THEN 0 WHEN 'sent' THEN 1 ELSE 2 END,
         id DESC
`
