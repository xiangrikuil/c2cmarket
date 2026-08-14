package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"

	"github.com/jackc/pgx/v5"
)

func (s *Store) PasswordResetSubject(ctx context.Context, normalizedEmail string) (auth.PasswordResetSubject, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.PasswordResetSubject{}, internalStoreError()
	}
	var subject auth.PasswordResetSubject
	var status string
	var securityLockedAt *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT claim.user_id::text, u.account_status, u.security_locked_at
		FROM student_email_claims claim
		JOIN users u ON u.id = claim.user_id
		WHERE claim.normalized_email = $1
	`, strings.TrimSpace(strings.ToLower(normalizedEmail))).Scan(&subject.UserID, &status, &securityLockedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.PasswordResetSubject{}, nil
	}
	if err != nil {
		return auth.PasswordResetSubject{}, internalStoreError()
	}
	subject.Eligible = securityLockedAt == nil && status != auth.AccountStatusArchived && (status == auth.AccountStatusActive || status == auth.AccountStatusSuspended || status == auth.AccountStatusBanned)
	return subject, nil
}

func (s *Store) ReplacePasswordResetChallenge(ctx context.Context, normalizedEmail, expectedUserID, codeHash string, expiresAt, now time.Time) (bool, *domain.AppError) {
	if s == nil || s.pool == nil {
		return false, internalStoreError()
	}
	email := strings.TrimSpace(strings.ToLower(normalizedEmail))
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, internalStoreError()
	}
	defer rollback(ctx, tx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1 || ':' || $2, 0))`, auth.PasswordResetPurpose, email); err != nil {
		return false, internalStoreError()
	}
	if _, err := tx.Exec(ctx, `
		UPDATE email_verification_codes
		SET consumed_at = $2
		WHERE email = $1
		  AND purpose = 'password_reset'
		  AND consumed_at IS NULL
	`, email, now); err != nil {
		return false, internalStoreError()
	}
	var userID, status string
	var securityLockedAt *time.Time
	err = tx.QueryRow(ctx, `
		SELECT claim.user_id::text, u.account_status, u.security_locked_at
		FROM student_email_claims claim
		JOIN users u ON u.id = claim.user_id
		WHERE claim.normalized_email = $1
		FOR SHARE OF claim, u
	`, email).Scan(&userID, &status, &securityLockedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.Commit(ctx); err != nil {
			return false, internalStoreError()
		}
		return false, nil
	}
	if err != nil {
		return false, internalStoreError()
	}
	eligible := userID == strings.TrimSpace(expectedUserID) && securityLockedAt == nil && status != auth.AccountStatusArchived && (status == auth.AccountStatusActive || status == auth.AccountStatusSuspended || status == auth.AccountStatusBanned)
	if !eligible {
		if err := tx.Commit(ctx); err != nil {
			return false, internalStoreError()
		}
		return false, nil
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO email_verification_codes (
		  user_id, email, purpose, code_hash, expires_at, attempt_count, created_at
		)
		VALUES ($1, $2, 'password_reset', $3, $4, 0, $5)
	`, userID, email, strings.TrimSpace(codeHash), expiresAt, now); err != nil {
		return false, internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return false, internalStoreError()
	}
	return true, nil
}

func (s *Store) ConfirmPasswordReset(ctx context.Context, input auth.PasswordResetConfirmInput, expectedUserID, codeHash string, credential auth.PasswordCredential, now time.Time) *domain.AppError {
	if s == nil || s.pool == nil {
		return internalStoreError()
	}
	email := strings.TrimSpace(strings.ToLower(input.Email))
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return internalStoreError()
	}
	defer rollback(ctx, tx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1 || ':' || $2, 0))`, auth.PasswordResetPurpose, email); err != nil {
		return internalStoreError()
	}
	var codeID, userID, storedHash, status string
	var expiresAt time.Time
	var attempts int
	var securityLockedAt *time.Time
	err = tx.QueryRow(ctx, `
		SELECT code.id::text, code.user_id::text, code.code_hash, code.expires_at,
		       code.attempt_count, u.account_status, u.security_locked_at
		FROM email_verification_codes code
		JOIN student_email_claims claim
		  ON claim.user_id = code.user_id AND claim.normalized_email = code.email
		JOIN users u ON u.id = claim.user_id
		WHERE code.email = $1
		  AND code.purpose = 'password_reset'
		  AND code.consumed_at IS NULL
		ORDER BY code.created_at DESC, code.id DESC
		LIMIT 1
		FOR UPDATE OF code, claim, u
	`, email).Scan(&codeID, &userID, &storedHash, &expiresAt, &attempts, &status, &securityLockedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.VerificationCodeInvalidError()
	}
	if err != nil {
		return internalStoreError()
	}
	eligible := userID == strings.TrimSpace(expectedUserID) && securityLockedAt == nil && status != auth.AccountStatusArchived && (status == auth.AccountStatusActive || status == auth.AccountStatusSuspended || status == auth.AccountStatusBanned)
	if !eligible || !now.Before(expiresAt) || attempts >= auth.PasswordResetMaxAttempts {
		if _, err := tx.Exec(ctx, `UPDATE email_verification_codes SET consumed_at = $2 WHERE id = $1 AND consumed_at IS NULL`, codeID, now); err != nil {
			return internalStoreError()
		}
		if err := tx.Commit(ctx); err != nil {
			return internalStoreError()
		}
		return auth.VerificationCodeInvalidError()
	}
	if !constantTimeStringEqual(storedHash, codeHash) {
		attempts++
		if _, err := tx.Exec(ctx, `
			UPDATE email_verification_codes
			SET attempt_count = $2,
			    consumed_at = CASE WHEN $2 >= $3 THEN $4 ELSE NULL END
			WHERE id = $1
		`, codeID, attempts, auth.PasswordResetMaxAttempts, now); err != nil {
			return internalStoreError()
		}
		if err := tx.Commit(ctx); err != nil {
			return internalStoreError()
		}
		return auth.VerificationCodeInvalidError()
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO user_password_credentials (
		  user_id, password_algorithm, password_salt, password_hash, created_at, password_updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $5)
		ON CONFLICT (user_id) DO UPDATE
		SET password_algorithm = EXCLUDED.password_algorithm,
		    password_salt = EXCLUDED.password_salt,
		    password_hash = EXCLUDED.password_hash,
		    password_updated_at = EXCLUDED.password_updated_at
	`, userID, credential.Algorithm, credential.Salt, credential.Hash, now); err != nil {
		return internalStoreError()
	}
	result, err := tx.Exec(ctx, `
		UPDATE email_verification_codes
		SET consumed_at = $2
		WHERE id = $1 AND consumed_at IS NULL
	`, codeID, now)
	if err != nil || result.RowsAffected() != 1 {
		return internalStoreError()
	}
	if _, err := tx.Exec(ctx, `
		UPDATE auth_sessions
		SET revoked_at = COALESCE(revoked_at, $2), updated_at = $2
		WHERE user_id = $1 AND revoked_at IS NULL
	`, userID, now); err != nil {
		return internalStoreError()
	}
	if _, err := tx.Exec(ctx, `
		UPDATE restricted_business_sessions
		SET revoked_at = COALESCE(revoked_at, $2), last_seen_at = GREATEST(last_seen_at, $2)
		WHERE user_id = $1 AND revoked_at IS NULL
	`, userID, now); err != nil {
		return internalStoreError()
	}
	var nextVersion int64
	if err := tx.QueryRow(ctx, `
		UPDATE users
		SET version = version + 1, updated_at = $2
		WHERE id = $1
		RETURNING version
	`, userID, now).Scan(&nextVersion); err != nil {
		return internalStoreError()
	}
	metadata, err := json.Marshal(map[string]any{
		"credentialType":       "password",
		"sessionScopesRevoked": []string{"normal", "restricted_business"},
	})
	if err != nil {
		return internalStoreError()
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO domain_events (
		  aggregate_type, aggregate_id, event_type, actor_user_id, actor_kind,
		  aggregate_version, request_id, metadata_json, created_at
		)
		VALUES ('user', $1, 'user.password_reset_completed', $1, 'user', $2, $3, $4, $5)
	`, userID, nextVersion, strings.TrimSpace(input.RequestID), metadata, now); err != nil {
		return internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return internalStoreError()
	}
	return nil
}
