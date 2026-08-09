package postgres

import (
	"context"
	"errors"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/maintenance"
	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/report"

	"github.com/jackc/pgx/v5"
)

const dataLifecycleAdvisoryLockID int64 = 0x4332434d4b544c46

const apiOrderCredentialLifecycleLockPrefix = "api_order_credential_lifecycle:"

func (s *Store) RunDataLifecycle(ctx context.Context, now time.Time, batchSize int, policy maintenance.Policy) (maintenance.Result, *domain.AppError) {
	if s == nil || s.pool == nil {
		return maintenance.Result{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return maintenance.Result{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	result := maintenance.Result{}
	if err := tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock($1)`, dataLifecycleAdvisoryLockID).Scan(&result.LockAcquired); err != nil {
		return maintenance.Result{}, internalStoreError()
	}
	if !result.LockAcquired {
		if err := tx.Commit(ctx); err != nil {
			return maintenance.Result{}, internalStoreError()
		}
		return result, nil
	}
	result.DisputeRemedyConfirmationsExpired, err = expireDisputeRemedyConfirmationsInTx(ctx, tx, now, batchSize)
	if err != nil {
		return maintenance.Result{}, internalStoreError()
	}

	result.APIOrdersPaymentExpired, result.APIOrderReviewReminders, result.APIOrdersAutoCompleted, err = s.materializeAPIOrdersForMaintenanceInTx(ctx, tx, now, batchSize)
	if err != nil {
		return maintenance.Result{}, internalStoreError()
	}
	result.APIOrderCredentialsDestroyed, result.APIQuotaCredentialsDestroyed, err = destroyCompletedAPIOrderCredentialsInTx(
		ctx,
		tx,
		now,
		now.Add(-policy.APIDeliveryCredentialRetention),
		batchSize,
	)
	if err != nil {
		return maintenance.Result{}, internalStoreError()
	}
	retiredCredentialsDestroyed, err := destroyRetiredAPIQuotaCredentialsInTx(
		ctx,
		tx,
		now,
		now.Add(-policy.APIDeliveryCredentialRetention),
		batchSize,
	)
	if err != nil {
		return maintenance.Result{}, internalStoreError()
	}
	result.APIQuotaCredentialsDestroyed += retiredCredentialsDestroyed

	result.SessionsDeleted, err = execMaintenanceBatch(ctx, tx, `
		WITH candidates AS (
			SELECT id
			FROM auth_sessions
			WHERE (revoked_at IS NOT NULL AND revoked_at < $1)
			   OR (revoked_at IS NULL AND expires_at < $1)
			ORDER BY COALESCE(revoked_at, expires_at), id
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		DELETE FROM auth_sessions target
		USING candidates
		WHERE target.id = candidates.id
	`, now.Add(-policy.SessionRetention), batchSize)
	if err != nil {
		return maintenance.Result{}, internalStoreError()
	}

	result.AccountAppealSessionsDeleted, err = execMaintenanceBatch(ctx, tx, `
		WITH candidates AS (
			SELECT id
			FROM account_appeal_sessions
			WHERE (revoked_at IS NOT NULL AND revoked_at < $1)
			   OR (revoked_at IS NULL AND expires_at < $1)
			ORDER BY COALESCE(revoked_at, expires_at), id
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		DELETE FROM account_appeal_sessions target
		USING candidates
		WHERE target.id = candidates.id
	`, now.Add(-policy.SessionRetention), batchSize)
	if err != nil {
		return maintenance.Result{}, internalStoreError()
	}

	result.VerificationCodesDeleted, err = execMaintenanceBatch(ctx, tx, `
		WITH candidates AS (
			SELECT id
			FROM email_verification_codes
			WHERE (consumed_at IS NOT NULL AND consumed_at < $1)
			   OR (consumed_at IS NULL AND expires_at < $1)
			ORDER BY COALESCE(consumed_at, expires_at), id
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		DELETE FROM email_verification_codes target
		USING candidates
		WHERE target.id = candidates.id
	`, now.Add(-policy.EmailVerificationRetention), batchSize)
	if err != nil {
		return maintenance.Result{}, internalStoreError()
	}

	result.IdempotencyEntriesDeleted, err = execMaintenanceBatch(ctx, tx, `
		WITH candidates AS (
			SELECT id
			FROM idempotency_keys
			WHERE expires_at <= $1
			ORDER BY expires_at, id
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		DELETE FROM idempotency_keys target
		USING candidates
		WHERE target.id = candidates.id
	`, now, batchSize)
	if err != nil {
		return maintenance.Result{}, internalStoreError()
	}

	result.APIProbeSamplesDeleted, err = execMaintenanceBatch(ctx, tx, `
		WITH candidates AS (
			SELECT id
			FROM api_probe_connection_samples
			WHERE status IN ('succeeded', 'failed')
			  AND finished_at < $1
			ORDER BY finished_at, id
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		DELETE FROM api_probe_connection_samples target
		USING candidates
		WHERE target.id = candidates.id
	`, now.Add(-policy.APIProbeSampleRetention), batchSize)
	if err != nil {
		return maintenance.Result{}, internalStoreError()
	}

	result.ContactSessionsExpired, err = execMaintenanceBatch(ctx, tx, `
		WITH candidates AS (
			SELECT id
			FROM contact_sessions
			WHERE status = 'open'
			  AND ends_at <= $1
			ORDER BY ends_at, id
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		UPDATE contact_sessions target
		SET status = 'expired'
		FROM candidates
		WHERE target.id = candidates.id
	`, now, batchSize)
	if err != nil {
		return maintenance.Result{}, internalStoreError()
	}

	result.NotificationsDeleted, err = execMaintenanceBatch(ctx, tx, `
		WITH candidates AS (
			SELECT id
			FROM notifications
			WHERE (read_at IS NOT NULL AND created_at < $1)
			   OR (read_at IS NULL AND created_at < $2)
			ORDER BY created_at, id
			LIMIT $3
			FOR UPDATE SKIP LOCKED
		)
		DELETE FROM notifications target
		USING candidates
		WHERE target.id = candidates.id
	`, now.Add(-policy.ReadNotificationRetention), now.Add(-policy.UnreadNotificationRetention), batchSize)
	if err != nil {
		return maintenance.Result{}, internalStoreError()
	}

	result.DomainEventsDeleted, err = execMaintenanceBatch(ctx, tx, `
		WITH candidates AS (
			SELECT event.id
			FROM domain_events event
			WHERE event.created_at < $1
			  AND NOT EXISTS (
			    SELECT 1
			    FROM notifications notification
			    WHERE notification.source_event_id = event.id
			  )
			ORDER BY event.created_at, event.id
			LIMIT $2
			FOR UPDATE OF event SKIP LOCKED
		)
		DELETE FROM domain_events target
		USING candidates
		WHERE target.id = candidates.id
	`, now.Add(-policy.DomainEventRetention), batchSize)
	if err != nil {
		return maintenance.Result{}, internalStoreError()
	}

	if err := tx.Commit(ctx); err != nil {
		return maintenance.Result{}, internalStoreError()
	}
	return result, nil
}

type expiredDisputeRemedyCandidate struct {
	RemedyID      string
	DisputeID     string
	OrderID       string
	OrderStatus   string
	ResponsibleID string
	BeneficiaryID string
}

func expireDisputeRemedyConfirmationsInTx(ctx context.Context, tx pgx.Tx, now time.Time, batchSize int) (int64, error) {
	rows, err := tx.Query(ctx, `
		SELECT remedy.id::text, dispute.id::text, api_order.id::text, api_order.status,
		       remedy.responsible_user_id::text, remedy.beneficiary_user_id::text
		FROM api_order_dispute_remedies remedy
		JOIN dispute_cases dispute ON dispute.id = remedy.dispute_case_id
		JOIN api_orders api_order
		  ON api_order.id::text = dispute.target_id
		 AND api_order.dispute_case_id = dispute.id
		WHERE remedy.status = 'claimed_fulfilled'
		  AND remedy.confirmation_due_at <= $1
		  AND dispute.status = 'resolved'
		  AND api_order.dispute_status = 'fulfillment_confirmation'
		ORDER BY remedy.confirmation_due_at, remedy.id
		LIMIT $2
		FOR UPDATE OF remedy, dispute, api_order SKIP LOCKED
	`, now, batchSize)
	if err != nil {
		return 0, err
	}
	candidates := make([]expiredDisputeRemedyCandidate, 0)
	for rows.Next() {
		var candidate expiredDisputeRemedyCandidate
		if err := rows.Scan(&candidate.RemedyID, &candidate.DisputeID, &candidate.OrderID, &candidate.OrderStatus, &candidate.ResponsibleID, &candidate.BeneficiaryID); err != nil {
			rows.Close()
			return 0, err
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()

	for _, candidate := range candidates {
		requestID := "remedy-confirmation-timeout:" + candidate.RemedyID
		if _, err := tx.Exec(ctx, `
			UPDATE api_order_dispute_remedies
			SET status = 'confirmation_expired', confirmation_expired_at = $2,
			    response_note = $3,
			    response_request_id = $4, updated_at = $2, version = version + 1
			WHERE id = $1 AND status = 'claimed_fulfilled'
		`, candidate.RemedyID, now, report.RemedyConfirmationExpiredNote, requestID); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE dispute_cases
			SET status = 'closed', public_result = $2,
			    closed_at = $3, updated_at = $3, version = version + 1
			WHERE id = $1 AND status = 'resolved'
		`, candidate.DisputeID, report.RemedyConfirmationExpiredPublicResult, now); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE api_orders
			SET dispute_status = 'closed', updated_at = $2, version = version + 1
			WHERE id = $1 AND dispute_status = 'fulfillment_confirmation'
		`, candidate.OrderID, now); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO dispute_events (
				entity_type, entity_id, action, actor_user_id, actor_role, reason, public, request_id, created_at
			)
			VALUES ('dispute', $1, 'remedy_confirmation_expired', NULL, 'system',
			        '对方未在确认期限内反馈；平台未核验到账或履约事实。', true, $2, $3)
		`, candidate.DisputeID, requestID, now); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO api_order_events (
				id, api_order_id, actor_user_id, event_type, from_status, to_status, note, request_id, created_at
			)
			VALUES (gen_random_uuid(), $1, NULL, $2, $3, $3,
			        '确认期限已到，流程中性结案；平台未核验到账或履约事实。', $4, $5)
			ON CONFLICT (api_order_id, event_type, request_id) DO NOTHING
		`, candidate.OrderID, apiorder.EventDisputeClosed, candidate.OrderStatus, requestID, now); err != nil {
			return 0, err
		}
		if appErr := insertDisputeNotifications(ctx, tx, candidate.DisputeID, "dispute.remedy_confirmation_expired", "整改确认期已结束", "对方未在期限内反馈，流程已中性结案；平台未核验到账或履约事实。", candidate.RemedyID+":confirmation_expired", now, candidate.ResponsibleID, candidate.BeneficiaryID); appErr != nil {
			return 0, errors.New(appErr.Detail)
		}
	}
	return int64(len(candidates)), nil
}

func destroyCompletedAPIOrderCredentialsInTx(ctx context.Context, tx pgx.Tx, now, cutoff time.Time, batchSize int) (int64, int64, error) {
	var orderCredentialsDestroyed int64
	var quotaCredentialsDestroyed int64
	err := tx.QueryRow(ctx, `
		WITH order_locked AS MATERIALIZED (
			SELECT credential.id, order_row.id AS api_order_id, order_row.api_quota_credential_id,
			       GREATEST(
			         order_row.completed_at,
			         COALESCE(order_row.package_expires_at, order_row.completed_at),
			         COALESCE(order_row.quota_expires_at_snapshot, order_row.completed_at),
			         COALESCE(order_row.delivery_submitted_at, order_row.completed_at)
			       ) AS retention_anchor
			FROM api_order_delivery_credentials credential
			JOIN api_orders order_row ON order_row.id = credential.api_order_id
			WHERE credential.destroyed_at IS NULL
			  AND order_row.status = 'completed'
			  AND order_row.completed_at IS NOT NULL
			  AND GREATEST(
			        order_row.completed_at,
			        COALESCE(order_row.package_expires_at, order_row.completed_at),
			        COALESCE(order_row.quota_expires_at_snapshot, order_row.completed_at),
			        COALESCE(order_row.delivery_submitted_at, order_row.completed_at)
			      ) <= $1
			  AND NOT EXISTS (
			    SELECT 1
			    FROM dispute_cases dispute
			    WHERE dispute.target_type = 'api_order'
			      AND dispute.target_id = order_row.id::text
			      AND dispute.status IN ('negotiating', 'open', 'waiting_info')
			  )
			  AND NOT EXISTS (
			    SELECT 1
			    FROM appeals appeal
			    LEFT JOIN dispute_cases appeal_dispute ON appeal_dispute.id = appeal.dispute_case_id
			    LEFT JOIN reports appeal_report ON appeal_report.id = appeal.report_id
			    WHERE appeal.status = 'submitted'
			      AND (
			        (appeal_dispute.target_type = 'api_order' AND appeal_dispute.target_id = order_row.id::text)
			        OR (
			          appeal_report.canonical_target_type = 'api_order'
			          AND appeal_report.canonical_target_id = order_row.id::text
			        )
			      )
			  )
			ORDER BY retention_anchor, credential.id
			LIMIT $2
			FOR UPDATE OF credential, order_row SKIP LOCKED
		), quota_locked AS MATERIALIZED (
			SELECT quota.id
			FROM api_quota_credentials quota
			JOIN order_locked candidate
			  ON candidate.api_quota_credential_id = quota.id
			FOR UPDATE OF quota SKIP LOCKED
		), fully_locked AS MATERIALIZED (
			SELECT order_locked.*
			FROM order_locked
			WHERE order_locked.api_quota_credential_id IS NULL
			   OR EXISTS (
			     SELECT 1
			     FROM quota_locked
			     WHERE quota_locked.id = order_locked.api_quota_credential_id
			   )
		), locked_candidates AS MATERIALIZED (
			SELECT fully_locked.*,
			       pg_try_advisory_xact_lock(
			         hashtextextended($4 || fully_locked.api_order_id::text, 0)
			       ) AS lifecycle_lock_acquired
			FROM fully_locked
		), candidates AS (
			SELECT locked_candidates.id, locked_candidates.api_quota_credential_id
			FROM locked_candidates
			WHERE locked_candidates.lifecycle_lock_acquired
			  AND NOT EXISTS (
			    SELECT 1
			    FROM dispute_cases dispute
			    WHERE dispute.target_type = 'api_order'
			      AND dispute.target_id = locked_candidates.api_order_id::text
			      AND dispute.status IN ('negotiating', 'open', 'waiting_info')
			  )
			  AND NOT EXISTS (
			    SELECT 1
			    FROM appeals appeal
			    LEFT JOIN dispute_cases appeal_dispute ON appeal_dispute.id = appeal.dispute_case_id
			    LEFT JOIN reports appeal_report ON appeal_report.id = appeal.report_id
			    WHERE appeal.status = 'submitted'
			      AND (
			        (
			          appeal_dispute.target_type = 'api_order'
			          AND appeal_dispute.target_id = locked_candidates.api_order_id::text
			        )
			        OR (
			          appeal_report.canonical_target_type = 'api_order'
			          AND appeal_report.canonical_target_id = locked_candidates.api_order_id::text
			        )
			      )
			  )
		), destroyed_quota AS (
			UPDATE api_quota_credentials target
			SET api_base_url = NULL,
			    panel_login_url = NULL,
			    username = NULL,
			    instructions = NULL,
			    api_key_ciphertext = NULL,
			    api_key_nonce = NULL,
			    password_ciphertext = NULL,
			    password_nonce = NULL,
			    secret_fingerprint = NULL,
			    destroyed_at = $3,
			    destroy_reason = 'retention_expired',
			    updated_at = $3
			FROM candidates
			WHERE target.id = candidates.api_quota_credential_id
			  AND target.status = 'delivered'
			  AND target.destroyed_at IS NULL
			RETURNING target.id
		), destroyed_orders AS (
			UPDATE api_order_delivery_credentials target
			SET api_base_url = NULL,
			    panel_login_url = NULL,
			    username = NULL,
			    instructions = NULL,
			    api_key_ciphertext = NULL,
			    api_key_nonce = NULL,
			    password_ciphertext = NULL,
			    password_nonce = NULL,
			    destroyed_at = $3,
			    destroy_reason = 'retention_expired'
			FROM candidates
			WHERE target.id = candidates.id
			  AND target.destroyed_at IS NULL
			  AND (
			    candidates.api_quota_credential_id IS NULL
			    OR EXISTS (
			      SELECT 1
			      FROM destroyed_quota
			      WHERE destroyed_quota.id = candidates.api_quota_credential_id
			    )
			  )
			RETURNING candidates.api_quota_credential_id
		)
		SELECT (SELECT count(*) FROM destroyed_orders),
		       (SELECT count(*) FROM destroyed_quota)
	`, cutoff, batchSize, now, apiOrderCredentialLifecycleLockPrefix).Scan(&orderCredentialsDestroyed, &quotaCredentialsDestroyed)
	return orderCredentialsDestroyed, quotaCredentialsDestroyed, err
}

func destroyRetiredAPIQuotaCredentialsInTx(ctx context.Context, tx pgx.Tx, now, cutoff time.Time, batchSize int) (int64, error) {
	return execMaintenanceBatch(ctx, tx, `
		WITH candidates AS (
			SELECT id
			FROM api_quota_credentials
			WHERE status = 'retired'
			  AND destroyed_at IS NULL
			  AND retired_at <= $1
			ORDER BY retired_at, id
			LIMIT $2
			FOR UPDATE SKIP LOCKED
		)
		UPDATE api_quota_credentials target
		SET api_base_url = NULL,
		    panel_login_url = NULL,
		    username = NULL,
		    instructions = NULL,
		    api_key_ciphertext = NULL,
		    api_key_nonce = NULL,
		    password_ciphertext = NULL,
		    password_nonce = NULL,
		    secret_fingerprint = NULL,
		    destroyed_at = $3,
		    destroy_reason = 'retired_unused',
		    updated_at = $3
		FROM candidates
		WHERE target.id = candidates.id
		  AND target.destroyed_at IS NULL
	`, cutoff, batchSize, now)
}

func execMaintenanceBatch(ctx context.Context, tx pgx.Tx, query string, args ...any) (int64, error) {
	commandTag, err := tx.Exec(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return commandTag.RowsAffected(), nil
}

func (s *Store) materializeAPIOrdersForMaintenanceInTx(ctx context.Context, tx pgx.Tx, now time.Time, batchSize int) (int64, int64, int64, error) {
	rows, err := tx.Query(ctx, `
		SELECT id::text
		FROM api_orders
		WHERE (status = 'pending_payment' AND payment_expires_at <= $1)
		   OR (
		     status = 'delivery_submitted'
		     AND dispute_status IN ('none', 'closed')
		     AND delivery_review_expires_at <= $2
		   )
		ORDER BY COALESCE(delivery_review_expires_at, payment_expires_at), id
		LIMIT $3
		FOR UPDATE SKIP LOCKED
	`, now, now.Add(apiorder.DeliveryReviewReminderLead), batchSize)
	if err != nil {
		return 0, 0, 0, err
	}
	ids := make([]string, 0, batchSize)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, 0, 0, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, 0, 0, err
	}
	rows.Close()

	var paymentExpired int64
	var reviewReminders int64
	var autoCompleted int64
	for _, id := range ids {
		result, appErr := s.materializeExpiredAPIOrderInTx(ctx, tx, id, now)
		if appErr != nil {
			return 0, 0, 0, errors.New(appErr.Detail)
		}
		if result.PaymentTimeoutCancelled {
			paymentExpired++
		}
		if result.DeliveryReviewReminded {
			reviewReminders++
		}
		if result.AutoCompleted {
			autoCompleted++
		}
	}
	return paymentExpired, reviewReminders, autoCompleted, nil
}
