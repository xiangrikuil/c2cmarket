package postgres

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/maintenance"

	"github.com/jackc/pgx/v5"
)

const dataLifecycleAdvisoryLockID int64 = 0x4332434d4b544c46

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

func execMaintenanceBatch(ctx context.Context, tx pgx.Tx, query string, args ...any) (int64, error) {
	commandTag, err := tx.Exec(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return commandTag.RowsAffected(), nil
}
