package postgres

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/contact"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/reputation"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Store) CreateContactMethod(ctx context.Context, input contact.ContactMethodInput, method contact.ContactMethod, version contact.ContactMethodVersion) *domain.AppError {
	if s == nil || s.pool == nil || s.contactCodec == nil {
		return internalStoreError()
	}
	encoded, err := s.contactCodec.encode(input.Value, version.ID, contactFieldMethodValue)
	if err != nil {
		return internalStoreError()
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return internalStoreError()
	}
	defer rollback(ctx, tx)

	if appErr := createContactMethodInTx(ctx, tx, input, method, version, encoded); appErr != nil {
		return appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return internalStoreError()
	}
	return nil
}

func createContactMethodInTx(ctx context.Context, tx pgx.Tx, input contact.ContactMethodInput, method contact.ContactMethod, version contact.ContactMethodVersion, encoded encodedContactValue) *domain.AppError {
	_, err := tx.Exec(ctx, `
		INSERT INTO contact_methods (
			id, user_id, type, label, current_version_id, is_default, enabled, verified_at, created_at, updated_at, version
		)
		VALUES ($1, $2, $3, $4, NULL, $5, $6, $7, $8, $9, $10)
	`, method.ID, method.UserID, method.Type, method.Label, false, method.Enabled, method.VerifiedAt, method.CreatedAt, method.UpdatedAt, method.Version)
	if err != nil {
		if isUniqueViolationOnConstraint(err, "ux_contact_methods_one_enabled_wechat") {
			return contact.DuplicateEnabledWechatError()
		}
		return internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO contact_method_versions (
			id, contact_method_id, owner_user_id, value_ciphertext, value_nonce,
			masked_value, value_fingerprint, encryption_key_version, fingerprint_key_version,
			encryption_format, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, version.ID, version.ContactMethodID, version.OwnerUserID, encoded.Ciphertext, encoded.Nonce,
		version.MaskedValue, encoded.Fingerprint, encoded.EncryptionKeyVersion, encoded.FingerprintKeyVersion,
		encoded.CipherFormat, version.CreatedAt)
	if err != nil {
		return internalStoreError()
	}
	_, err = tx.Exec(ctx, `UPDATE contact_methods SET current_version_id = $2 WHERE id = $1`, method.ID, version.ID)
	if err != nil {
		return internalStoreError()
	}
	if method.IsDefault {
		if appErr := clearContactDefaultsInTx(ctx, tx, method.UserID, method.ID, method.UpdatedAt, input.RequestID); appErr != nil {
			return appErr
		}
		if _, err = tx.Exec(ctx, `UPDATE contact_methods SET is_default = true WHERE id = $1 AND user_id = $2`, method.ID, method.UserID); err != nil {
			return internalStoreError()
		}
	}
	return insertContactMethodEvent(ctx, tx, method, "contact_method.created", input.RequestID, []string{"type", "label", "value", "verifiedAt", "isDefault", "enabled"}, method.UpdatedAt)
}

func (s *Store) CreateContactMethodWithIdempotency(ctx context.Context, entry idempotency.Entry, input contact.ContactMethodInput, method contact.ContactMethod, version contact.ContactMethodVersion, buildCompletion contact.MethodCompletionBuilder) (contact.ContactMethod, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil || s.contactCodec == nil {
		return contact.ContactMethod{}, idempotency.Completion{}, internalStoreError()
	}
	encoded, err := s.contactCodec.encode(input.Value, version.ID, contactFieldMethodValue)
	if err != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, appErr
	}
	if appErr := createContactMethodInTx(ctx, tx, input, method, version, encoded); appErr != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(method)
	if appErr != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, method.UpdatedAt); appErr != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, internalStoreError()
	}
	return method, completion, nil
}

func (s *Store) ListContactMethods(ctx context.Context, userID string) ([]contact.ContactMethod, *domain.AppError) {
	if s == nil || s.pool == nil || s.contactCodec == nil {
		return nil, internalStoreError()
	}
	rows, err := s.pool.Query(ctx, `
		SELECT m.id::text, m.user_id::text, m.type, m.label, COALESCE(v.masked_value, ''),
		       v.value_ciphertext, v.value_nonce, COALESCE(v.encryption_key_version, ''),
		       COALESCE(v.encryption_format, ''), m.enabled, m.is_default, m.verified_at,
		       COALESCE(m.current_version_id::text, ''), m.created_at, m.updated_at, m.version
		FROM contact_methods m
		JOIN contact_method_versions v ON v.id = m.current_version_id
		WHERE m.user_id = $1 AND m.enabled = true
		ORDER BY m.is_default DESC, m.updated_at DESC
	`, userID)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return s.scanContactMethodsWithValues(rows)
}

func (s *Store) UpdateContactMethod(ctx context.Context, input contact.UpdateContactMethodInput, method contact.ContactMethod, version contact.ContactMethodVersion) (contact.ContactMethod, *domain.AppError) {
	if s == nil || s.pool == nil || s.contactCodec == nil {
		return contact.ContactMethod{}, internalStoreError()
	}
	encoded, err := s.contactCodec.encode(input.Value, version.ID, contactFieldMethodValue)
	if err != nil {
		return contact.ContactMethod{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return contact.ContactMethod{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	method, appErr := s.updateContactMethodInTx(ctx, tx, input, method, version, encoded)
	if appErr != nil {
		return contact.ContactMethod{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return contact.ContactMethod{}, internalStoreError()
	}
	return method, nil
}

func (s *Store) UpdateContactMethodWithIdempotency(ctx context.Context, entry idempotency.Entry, input contact.UpdateContactMethodInput, method contact.ContactMethod, version contact.ContactMethodVersion, buildCompletion contact.MethodCompletionBuilder) (contact.ContactMethod, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil || s.contactCodec == nil {
		return contact.ContactMethod{}, idempotency.Completion{}, internalStoreError()
	}
	encoded, err := s.contactCodec.encode(input.Value, version.ID, contactFieldMethodValue)
	if err != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, appErr
	}
	method, appErr = s.updateContactMethodInTx(ctx, tx, input, method, version, encoded)
	if appErr != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(method)
	if appErr != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, method.UpdatedAt); appErr != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, internalStoreError()
	}
	return method, completion, nil
}

func (s *Store) updateContactMethodInTx(ctx context.Context, tx pgx.Tx, input contact.UpdateContactMethodInput, method contact.ContactMethod, version contact.ContactMethodVersion, encoded encodedContactValue) (contact.ContactMethod, *domain.AppError) {
	var current contact.ContactMethod
	var currentFingerprint string
	err := tx.QueryRow(ctx, `
		SELECT m.id::text, m.user_id::text, m.type, m.label, m.enabled, m.is_default, m.verified_at,
		       COALESCE(m.current_version_id::text, ''), m.created_at, m.updated_at, m.version,
		       COALESCE(v.value_fingerprint, '')
		FROM contact_methods m
		JOIN contact_method_versions v ON v.id = m.current_version_id
		WHERE m.id = $1 AND m.user_id = $2 AND m.enabled = true
		FOR UPDATE
	`, input.MethodID, input.UserID).Scan(
		&current.ID,
		&current.UserID,
		&current.Type,
		&current.Label,
		&current.Enabled,
		&current.IsDefault,
		&current.VerifiedAt,
		&current.CurrentVersionID,
		&current.CreatedAt,
		&current.UpdatedAt,
		&current.Version,
		&currentFingerprint,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return contact.ContactMethod{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Contact method not found", "联系方式不存在。")
	}
	if err != nil {
		return contact.ContactMethod{}, internalStoreError()
	}
	method.ID = current.ID
	method.UserID = current.UserID
	method.CreatedAt = current.CreatedAt
	method.Version = current.Version + 1
	method.DisplayValue = input.Value
	valueChanged := input.Type != current.Type || subtle.ConstantTimeCompare([]byte(currentFingerprint), []byte(encoded.Fingerprint)) != 1
	if valueChanged {
		version.ContactMethodID = current.ID
		version.OwnerUserID = current.UserID
		method.CurrentVersionID = version.ID
		method.VerifiedAt = input.VerifiedAt
	} else {
		method.CurrentVersionID = current.CurrentVersionID
		method.VerifiedAt = current.VerifiedAt
		if input.VerifiedAt != nil {
			verifiedAt := *input.VerifiedAt
			method.VerifiedAt = &verifiedAt
		}
	}

	if valueChanged {
		_, err = tx.Exec(ctx, `
			INSERT INTO contact_method_versions (
				id, contact_method_id, owner_user_id, value_ciphertext, value_nonce,
				masked_value, value_fingerprint, encryption_key_version, fingerprint_key_version,
				encryption_format, created_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		`, version.ID, version.ContactMethodID, version.OwnerUserID, encoded.Ciphertext, encoded.Nonce,
			version.MaskedValue, encoded.Fingerprint, encoded.EncryptionKeyVersion, encoded.FingerprintKeyVersion,
			encoded.CipherFormat, version.CreatedAt)
		if err != nil {
			return contact.ContactMethod{}, internalStoreError()
		}

		_, err = tx.Exec(ctx, `
			UPDATE contact_method_versions
			SET retired_at = $3
			WHERE id = $1 AND owner_user_id = $2
		`, current.CurrentVersionID, current.UserID, method.UpdatedAt)
		if err != nil {
			return contact.ContactMethod{}, internalStoreError()
		}
	}
	if method.IsDefault {
		if appErr := clearContactDefaultsInTx(ctx, tx, method.UserID, method.ID, method.UpdatedAt, input.RequestID); appErr != nil {
			return contact.ContactMethod{}, appErr
		}
	}

	_, err = tx.Exec(ctx, `
		UPDATE contact_methods
		SET type = $3, label = $4, current_version_id = $5, is_default = $6,
		    enabled = $7, verified_at = $8, updated_at = $9, version = $10
		WHERE id = $1 AND user_id = $2
	`, method.ID, method.UserID, method.Type, method.Label, method.CurrentVersionID, method.IsDefault,
		method.Enabled, method.VerifiedAt, method.UpdatedAt, method.Version)
	if err != nil {
		if isUniqueViolationOnConstraint(err, "ux_contact_methods_one_enabled_wechat") {
			return contact.ContactMethod{}, contact.DuplicateEnabledWechatError()
		}
		return contact.ContactMethod{}, internalStoreError()
	}
	eventType := "contact_method.updated"
	if current.Enabled && !method.Enabled {
		eventType = "contact_method.disabled"
	}
	if appErr := insertContactMethodEvent(ctx, tx, method, eventType, input.RequestID, contactMethodChangedFields(current, method, valueChanged), method.UpdatedAt); appErr != nil {
		return contact.ContactMethod{}, appErr
	}
	return method, nil
}

func (s *Store) DeleteContactMethod(ctx context.Context, userID, methodID, requestID string, now time.Time) (contact.ContactMethod, *domain.AppError) {
	if s == nil || s.pool == nil {
		return contact.ContactMethod{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return contact.ContactMethod{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	method, appErr := deleteContactMethodInTx(ctx, tx, userID, methodID, requestID, now)
	if appErr != nil {
		return contact.ContactMethod{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return contact.ContactMethod{}, internalStoreError()
	}
	return method, nil
}

func (s *Store) DeleteContactMethodWithIdempotency(ctx context.Context, entry idempotency.Entry, userID, methodID, requestID string, now time.Time, buildCompletion contact.MethodCompletionBuilder) (contact.ContactMethod, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return contact.ContactMethod{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, appErr
	}
	method, appErr := deleteContactMethodInTx(ctx, tx, userID, methodID, requestID, now)
	if appErr != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(method)
	if appErr != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, internalStoreError()
	}
	return method, completion, nil
}

func deleteContactMethodInTx(ctx context.Context, tx pgx.Tx, userID, methodID, requestID string, now time.Time) (contact.ContactMethod, *domain.AppError) {
	var currentType string
	if err := tx.QueryRow(ctx, `
		SELECT type
		FROM contact_methods
		WHERE id = $1 AND user_id = $2 AND enabled = true
		FOR UPDATE
	`, methodID, userID).Scan(&currentType); errors.Is(err, pgx.ErrNoRows) {
		return contact.ContactMethod{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Contact method not found", "联系方式不存在。")
	} else if err != nil {
		return contact.ContactMethod{}, internalStoreError()
	}
	if currentType == "linuxdo" {
		return contact.ContactMethod{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Contact method protected", "linux.do 绑定联系方式不能删除。")
	}

	var method contact.ContactMethod
	err := tx.QueryRow(ctx, `
		UPDATE contact_methods
		SET enabled = false, is_default = false, updated_at = $3, version = version + 1
		WHERE id = $1 AND user_id = $2 AND enabled = true
		RETURNING id::text, user_id::text, type, label, '', enabled, is_default, verified_at,
		          COALESCE(current_version_id::text, ''), created_at, updated_at, version
	`, methodID, userID, now).Scan(
		&method.ID,
		&method.UserID,
		&method.Type,
		&method.Label,
		&method.MaskedValue,
		&method.Enabled,
		&method.IsDefault,
		&method.VerifiedAt,
		&method.CurrentVersionID,
		&method.CreatedAt,
		&method.UpdatedAt,
		&method.Version,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return contact.ContactMethod{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Contact method not found", "联系方式不存在。")
	}
	if err != nil {
		return contact.ContactMethod{}, internalStoreError()
	}
	if appErr := insertContactMethodEvent(ctx, tx, method, "contact_method.disabled", requestID, []string{"enabled", "isDefault"}, method.UpdatedAt); appErr != nil {
		return contact.ContactMethod{}, appErr
	}
	return method, nil
}

func (s *Store) SetDefaultContactMethod(ctx context.Context, userID, methodID, requestID string, now time.Time) (contact.ContactMethod, *domain.AppError) {
	if s == nil || s.pool == nil || s.contactCodec == nil {
		return contact.ContactMethod{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return contact.ContactMethod{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	method, appErr := s.setDefaultContactMethodInTx(ctx, tx, userID, methodID, requestID, now)
	if appErr != nil {
		return contact.ContactMethod{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return contact.ContactMethod{}, internalStoreError()
	}
	return method, nil
}

func (s *Store) SetDefaultContactMethodWithIdempotency(ctx context.Context, entry idempotency.Entry, userID, methodID, requestID string, now time.Time, buildCompletion contact.MethodCompletionBuilder) (contact.ContactMethod, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil || s.contactCodec == nil {
		return contact.ContactMethod{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, appErr
	}
	method, appErr := s.setDefaultContactMethodInTx(ctx, tx, userID, methodID, requestID, now)
	if appErr != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(method)
	if appErr != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, internalStoreError()
	}
	return method, completion, nil
}

func (s *Store) setDefaultContactMethodInTx(ctx context.Context, tx pgx.Tx, userID, methodID, requestID string, now time.Time) (contact.ContactMethod, *domain.AppError) {
	var exists bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM contact_methods WHERE id = $1 AND user_id = $2 AND enabled = true)`, methodID, userID).Scan(&exists)
	if err != nil {
		return contact.ContactMethod{}, internalStoreError()
	}
	if !exists {
		return contact.ContactMethod{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Contact method not found", "联系方式不存在。")
	}
	if appErr := clearContactDefaultsInTx(ctx, tx, userID, methodID, now, requestID); appErr != nil {
		return contact.ContactMethod{}, appErr
	}
	_, err = tx.Exec(ctx, `
		UPDATE contact_methods
		SET is_default = true, updated_at = $3, version = version + 1
		WHERE user_id = $1 AND id = $2 AND enabled = true
	`, userID, methodID, now)
	if err != nil {
		return contact.ContactMethod{}, internalStoreError()
	}
	method, appErr := s.getContactMethodWithValue(ctx, tx, userID, methodID)
	if appErr != nil {
		return contact.ContactMethod{}, appErr
	}
	if appErr := insertContactMethodEvent(ctx, tx, method, "contact_method.default_changed", requestID, []string{"isDefault"}, method.UpdatedAt); appErr != nil {
		return contact.ContactMethod{}, appErr
	}
	return method, nil
}

func (s *Store) CreateContactEmailVerificationCode(ctx context.Context, challenge contact.ContactEmailVerificationCode) *domain.AppError {
	if s == nil || s.pool == nil || s.contactCodec == nil {
		return internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return internalStoreError()
	}
	defer rollback(ctx, tx)

	var methodType, currentVersionID string
	err = tx.QueryRow(ctx, `
		SELECT type, current_version_id::text
		FROM contact_methods
		WHERE id = $1 AND user_id = $2 AND enabled = true
		FOR UPDATE
	`, challenge.ContactMethodID, challenge.UserID).Scan(&methodType, &currentVersionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return contactMethodNotFoundError()
	}
	if err != nil {
		return internalStoreError()
	}
	if methodType != "email" {
		return contactEmailMethodRequiredError()
	}
	method, appErr := s.getContactMethodWithValue(ctx, tx, challenge.UserID, challenge.ContactMethodID)
	if appErr != nil {
		return appErr
	}
	if currentVersionID != challenge.ContactMethodVersionID || !strings.EqualFold(strings.TrimSpace(method.DisplayValue), strings.TrimSpace(challenge.Email)) {
		return contactEmailChangedError()
	}
	if _, err = tx.Exec(ctx, `
		UPDATE email_verification_codes
		SET consumed_at = $2
		WHERE contact_method_id = $1
		  AND purpose = 'contact_email'
		  AND consumed_at IS NULL
	`, challenge.ContactMethodID, challenge.CreatedAt); err != nil {
		return internalStoreError()
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO email_verification_codes (
			user_id, email, purpose, code_hash, expires_at, created_at,
			contact_method_id, contact_method_version_id
		)
		VALUES ($1, lower($2), 'contact_email', $3, $4, $5, $6, $7)
	`, challenge.UserID, challenge.Email, challenge.CodeHash, challenge.ExpiresAt, challenge.CreatedAt,
		challenge.ContactMethodID, challenge.ContactMethodVersionID); err != nil {
		return internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) ConfirmContactEmailVerificationWithIdempotency(ctx context.Context, entry idempotency.Entry, input contact.ConfirmContactEmailInput, buildCompletion contact.MethodCompletionBuilder) (contact.ContactMethod, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil || s.contactCodec == nil || buildCompletion == nil {
		return contact.ContactMethod{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	// 联系方式行是父级锁；发码与确认必须统一先锁联系方式、再锁挑战，避免并发互等。
	var methodType, currentVersionID string
	err = tx.QueryRow(ctx, `
		SELECT type, current_version_id::text
		FROM contact_methods
		WHERE id = $1 AND user_id = $2 AND enabled = true
		FOR UPDATE
	`, input.ContactMethodID, input.UserID).Scan(&methodType, &currentVersionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return contact.ContactMethod{}, idempotency.Completion{}, contactMethodNotFoundError()
	}
	if err != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, internalStoreError()
	}

	var codeID, storedHash, challengeVersionID, challengeEmail string
	var expiresAt time.Time
	var attemptCount int
	err = tx.QueryRow(ctx, `
		SELECT id::text, code_hash, contact_method_version_id::text, email, expires_at, attempt_count
		FROM email_verification_codes
		WHERE user_id = $1
		  AND contact_method_id = $2
		  AND purpose = 'contact_email'
		  AND consumed_at IS NULL
		ORDER BY created_at DESC
		LIMIT 1
		FOR UPDATE
	`, input.UserID, input.ContactMethodID).Scan(
		&codeID, &storedHash, &challengeVersionID, &challengeEmail, &expiresAt, &attemptCount,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return contact.ContactMethod{}, idempotency.Completion{}, invalidContactEmailVerificationStoreError()
	}
	if err != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, internalStoreError()
	}
	if methodType != "email" || challengeVersionID != currentVersionID || challengeVersionID != input.ContactMethodVersionID ||
		!strings.EqualFold(challengeEmail, input.Email) {
		if appErr := consumeContactEmailChallengeInTx(ctx, tx, codeID, input.Now); appErr != nil {
			return contact.ContactMethod{}, idempotency.Completion{}, appErr
		}
		if err := tx.Commit(ctx); err != nil {
			return contact.ContactMethod{}, idempotency.Completion{}, internalStoreError()
		}
		return contact.ContactMethod{}, idempotency.Completion{}, invalidContactEmailVerificationStoreError()
	}
	if !input.Now.Before(expiresAt) || attemptCount >= contact.ContactEmailVerificationMaxAttempts {
		if appErr := consumeContactEmailChallengeInTx(ctx, tx, codeID, input.Now); appErr != nil {
			return contact.ContactMethod{}, idempotency.Completion{}, appErr
		}
		if err := tx.Commit(ctx); err != nil {
			return contact.ContactMethod{}, idempotency.Completion{}, internalStoreError()
		}
		return contact.ContactMethod{}, idempotency.Completion{}, invalidContactEmailVerificationStoreError()
	}
	if subtle.ConstantTimeCompare([]byte(storedHash), []byte(strings.TrimSpace(input.CodeHash))) != 1 {
		if _, err := tx.Exec(ctx, `
			UPDATE email_verification_codes
			SET attempt_count = attempt_count + 1,
			    consumed_at = CASE WHEN attempt_count + 1 >= $3 THEN $2 ELSE consumed_at END
			WHERE id = $1
		`, codeID, input.Now, contact.ContactEmailVerificationMaxAttempts); err != nil {
			return contact.ContactMethod{}, idempotency.Completion{}, internalStoreError()
		}
		if err := tx.Commit(ctx); err != nil {
			return contact.ContactMethod{}, idempotency.Completion{}, internalStoreError()
		}
		return contact.ContactMethod{}, idempotency.Completion{}, invalidContactEmailVerificationStoreError()
	}

	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, appErr
	}
	commandTag, err := tx.Exec(ctx, `
		UPDATE contact_methods
		SET verified_at = $3, updated_at = $3, version = version + 1
		WHERE id = $1 AND user_id = $2 AND enabled = true AND current_version_id = $4
	`, input.ContactMethodID, input.UserID, input.Now, input.ContactMethodVersionID)
	if err != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, internalStoreError()
	}
	if commandTag.RowsAffected() != 1 {
		return contact.ContactMethod{}, idempotency.Completion{}, invalidContactEmailVerificationStoreError()
	}
	if appErr := consumeContactEmailChallengeInTx(ctx, tx, codeID, input.Now); appErr != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, appErr
	}
	method, appErr := s.getContactMethodWithValue(ctx, tx, input.UserID, input.ContactMethodID)
	if appErr != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, appErr
	}
	if appErr := insertContactMethodEvent(ctx, tx, method, "contact_method.verified", input.RequestID, []string{"verifiedAt"}, input.Now); appErr != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(method)
	if appErr != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, input.Now); appErr != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return contact.ContactMethod{}, idempotency.Completion{}, internalStoreError()
	}
	return method, completion, nil
}

func consumeContactEmailChallengeInTx(ctx context.Context, tx pgx.Tx, codeID string, consumedAt time.Time) *domain.AppError {
	if _, err := tx.Exec(ctx, `
		UPDATE email_verification_codes
		SET consumed_at = COALESCE(consumed_at, $2)
		WHERE id = $1
	`, codeID, consumedAt); err != nil {
		return internalStoreError()
	}
	return nil
}

func invalidContactEmailVerificationStoreError() *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeVerificationCodeInvalid, "Verification code invalid", "验证码无效或已过期。", "code", "invalid", "验证码无效或已过期。")
}

func contactEmailMethodRequiredError() *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Email contact required", "当前联系方式不是邮箱。", "contactMethodId", "not_email", "请选择邮箱联系方式。")
}

func contactEmailChangedError() *domain.AppError {
	return domain.NewError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Contact email changed", "邮箱联系方式已变化，请重新获取验证码。")
}

func contactMethodNotFoundError() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Contact method not found", "联系方式不存在。")
}

func contactMethodChangedFields(before, after contact.ContactMethod, valueChanged bool) []string {
	fields := make([]string, 0, 6)
	if before.Type != after.Type {
		fields = append(fields, "type")
	}
	if before.Label != after.Label {
		fields = append(fields, "label")
	}
	if valueChanged {
		fields = append(fields, "value")
	}
	if (before.VerifiedAt == nil) != (after.VerifiedAt == nil) || (before.VerifiedAt != nil && !before.VerifiedAt.Equal(*after.VerifiedAt)) {
		fields = append(fields, "verifiedAt")
	}
	if before.IsDefault != after.IsDefault {
		fields = append(fields, "isDefault")
	}
	if before.Enabled != after.Enabled {
		fields = append(fields, "enabled")
	}
	return fields
}

func (s *Store) CreateContactSession(ctx context.Context, input contact.CreateContactSessionInput, session contact.ContactSession, now time.Time) (contact.ContactSession, *domain.AppError) {
	if s == nil || s.pool == nil {
		return contact.ContactSession{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return contact.ContactSession{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	_, buyerVersion, appErr := lockTransactionContactVersionForOwner(ctx, tx, input.BuyerContactMethodID, input.BuyerUserID, "buyerContactMethodId", "买家交易联系方式不可用。")
	if appErr != nil {
		return contact.ContactSession{}, appErr
	}
	_, sellerVersion, appErr := lockTransactionContactVersionForOwner(ctx, tx, input.SellerContactMethodID, input.SellerUserID, "sellerContactMethodId", "卖家交易联系方式不可用。")
	if appErr != nil {
		return contact.ContactSession{}, appErr
	}

	session.BuyerVersionID = buyerVersion.ID
	session.SellerVersionID = sellerVersion.ID
	_, err = tx.Exec(ctx, `
		INSERT INTO contact_sessions (id, buyer_user_id, seller_user_id, opens_at, ends_at, status, created_at)
		VALUES ($1, $2, $3, $4, $5, 'open', $6)
	`, session.ID, session.BuyerUserID, session.SellerUserID, session.OpensAt, session.EndsAt, now)
	if err != nil {
		return contact.ContactSession{}, internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO contact_session_items (contact_session_id, subject_user_id, side, contact_method_version_id, created_at)
		VALUES ($1, $2, 'buyer', $3, $4),
		       ($1, $5, 'seller', $6, $4)
	`, session.ID, session.BuyerUserID, session.BuyerVersionID, now, session.SellerUserID, session.SellerVersionID)
	if err != nil {
		return contact.ContactSession{}, internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return contact.ContactSession{}, internalStoreError()
	}
	return session, nil
}

func (s *Store) ReadContactSession(ctx context.Context, sessionID, viewerUserID, requestID string, now time.Time) (contact.ContactSessionView, *domain.AppError) {
	if s == nil || s.pool == nil || s.contactCodec == nil {
		return contact.ContactSessionView{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return contact.ContactSessionView{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	var buyerUserID, sellerUserID, status string
	var endsAt *time.Time
	err = tx.QueryRow(ctx, `
		SELECT buyer_user_id::text, seller_user_id::text, status, ends_at
		FROM contact_sessions
		WHERE id = $1
		FOR UPDATE
	`, sessionID).Scan(&buyerUserID, &sellerUserID, &status, &endsAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return contact.ContactSessionView{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Contact session not found", "联系窗口不存在。")
	}
	if err != nil {
		return contact.ContactSessionView{}, internalStoreError()
	}
	if viewerUserID != buyerUserID && viewerUserID != sellerUserID {
		return contact.ContactSessionView{}, domain.NewError(http.StatusForbidden, domain.CodeContactAccessForbidden, "Contact access forbidden", "你不是该联系窗口参与方。")
	}
	expired := endsAt != nil && !now.Before(*endsAt)
	if status != "open" || expired {
		if status == "open" && expired {
			_, _ = tx.Exec(ctx, `UPDATE contact_sessions SET status = 'expired' WHERE id = $1 AND status = 'open'`, sessionID)
		}
		return contact.ContactSessionView{}, domain.NewError(http.StatusConflict, domain.CodeContactWindowExpired, "Contact window expired", "联系窗口已过期。")
	}

	rows, err := tx.Query(ctx, `
		SELECT i.side, i.subject_user_id::text, m.type, m.label, v.id::text,
		       v.value_ciphertext, v.value_nonce, v.encryption_key_version, v.encryption_format,
		       v.masked_value
		FROM contact_session_items i
		JOIN contact_method_versions v ON v.id = i.contact_method_version_id
		JOIN contact_methods m ON m.id = v.contact_method_id
		WHERE i.contact_session_id = $1
		ORDER BY CASE i.side WHEN 'buyer' THEN 1 ELSE 2 END
	`, sessionID)
	if err != nil {
		return contact.ContactSessionView{}, internalStoreError()
	}
	defer rows.Close()

	items := make([]contact.ContactItemView, 0, 2)
	for rows.Next() {
		var item contact.ContactItemView
		var ciphertext, nonce []byte
		var recordID, keyVersion, cipherFormat string
		if err := rows.Scan(&item.Side, &item.SubjectID, &item.Type, &item.Label, &recordID,
			&ciphertext, &nonce, &keyVersion, &cipherFormat, &item.MaskedValue); err != nil {
			return contact.ContactSessionView{}, internalStoreError()
		}
		value, err := s.contactCodec.decode(ciphertext, nonce, keyVersion, cipherFormat, recordID, contactFieldMethodValue)
		if err != nil {
			return contact.ContactSessionView{}, internalStoreError()
		}
		item.Value = value
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return contact.ContactSessionView{}, internalStoreError()
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO contact_access_logs (contact_session_id, viewer_user_id, accessed_at, request_id)
		VALUES ($1, $2, $3, $4)
	`, sessionID, viewerUserID, now, requestID)
	if err != nil {
		return contact.ContactSessionView{}, internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return contact.ContactSessionView{}, internalStoreError()
	}
	return contact.ContactSessionView{
		SessionID: sessionID,
		EndsAt:    endsAt,
		Items:     items,
	}, nil
}

func (s *Store) ContactSessionViewerRole(ctx context.Context, sessionID, viewerUserID string) (string, *domain.AppError) {
	if s == nil || s.pool == nil {
		return "", internalStoreError()
	}
	var buyerUserID, sellerUserID string
	err := s.pool.QueryRow(ctx, `
		SELECT buyer_user_id::text, seller_user_id::text
		FROM contact_sessions
		WHERE id = $1
	`, sessionID).Scan(&buyerUserID, &sellerUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Contact session not found", "联系窗口不存在。")
	}
	if err != nil {
		return "", internalStoreError()
	}
	switch viewerUserID {
	case buyerUserID:
		return reputation.RoleBuyer, nil
	case sellerUserID:
		return reputation.RoleSeller, nil
	default:
		return "", domain.NewError(http.StatusForbidden, domain.CodeContactAccessForbidden, "Contact access forbidden", "你不是该联系窗口参与方。")
	}
}

func (s *Store) ContactAccessLogCount(ctx context.Context, sessionID string) (int, *domain.AppError) {
	if s == nil || s.pool == nil {
		return 0, internalStoreError()
	}
	var count int
	err := s.pool.QueryRow(ctx, `
		SELECT count(*)::int
		FROM contact_access_logs
		WHERE contact_session_id = $1
	`, sessionID).Scan(&count)
	if err != nil {
		return 0, internalStoreError()
	}
	return count, nil
}

func clearContactDefaultsInTx(ctx context.Context, tx pgx.Tx, userID, exceptMethodID string, now time.Time, requestID string) *domain.AppError {
	rows, err := tx.Query(ctx, `
		UPDATE contact_methods
		SET is_default = false, updated_at = $3, version = version + 1
		WHERE user_id = $1 AND id <> $2 AND is_default = true
		RETURNING id::text, user_id::text, version, updated_at
	`, userID, exceptMethodID, now)
	if err != nil {
		return internalStoreError()
	}
	methods := make([]contact.ContactMethod, 0, 1)
	for rows.Next() {
		method := contact.ContactMethod{}
		if err := rows.Scan(&method.ID, &method.UserID, &method.Version, &method.UpdatedAt); err != nil {
			rows.Close()
			return internalStoreError()
		}
		methods = append(methods, method)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return internalStoreError()
	}
	rows.Close()
	for _, method := range methods {
		if appErr := insertContactMethodEvent(ctx, tx, method, "contact_method.default_changed", requestID, []string{"isDefault"}, now); appErr != nil {
			return appErr
		}
	}
	return nil
}

func insertContactMethodEvent(ctx context.Context, tx pgx.Tx, method contact.ContactMethod, eventType, requestID string, changedFields []string, now time.Time) *domain.AppError {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		requestID = "unknown"
	}
	metadata, err := json.Marshal(map[string]any{"changedFields": changedFields})
	if err != nil {
		return internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO domain_events (
			id, aggregate_type, aggregate_id, event_type, actor_user_id, actor_kind,
			aggregate_version, request_id, metadata_json, created_at
		)
		VALUES ($1, 'contact_method', $2, $3, $4, 'user', $5, $6, $7, $8)
	`, uuid.NewString(), method.ID, eventType, method.UserID, method.Version, requestID, metadata, now)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func lockContactVersionForOwner(ctx context.Context, q queryer, methodID, ownerID, detail string) (contact.ContactMethod, contact.ContactMethodVersion, *domain.AppError) {
	var method contact.ContactMethod
	var version contact.ContactMethodVersion
	err := q.QueryRow(ctx, `
		SELECT m.id::text, m.user_id::text, m.type, m.label, m.enabled,
		       m.is_default, m.verified_at, m.created_at, m.updated_at, m.version,
		       v.id::text, v.contact_method_id::text, v.owner_user_id::text, v.masked_value
		FROM contact_methods m
		JOIN contact_method_versions v
		  ON v.id = m.current_version_id
		 AND v.contact_method_id = m.id
		 AND v.owner_user_id = m.user_id
		WHERE m.id = $1
		  AND m.user_id = $2
		  AND m.enabled = true
		  AND m.current_version_id IS NOT NULL
		  AND v.retired_at IS NULL
		  AND v.destroyed_at IS NULL
		FOR UPDATE
	`, methodID, ownerID).Scan(
		&method.ID,
		&method.UserID,
		&method.Type,
		&method.Label,
		&method.Enabled,
		&method.IsDefault,
		&method.VerifiedAt,
		&method.CreatedAt,
		&method.UpdatedAt,
		&method.Version,
		&version.ID,
		&version.ContactMethodID,
		&version.OwnerUserID,
		&version.MaskedValue,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return contact.ContactMethod{}, contact.ContactMethodVersion{}, domain.NewError(http.StatusUnprocessableEntity, domain.CodeContactMethodNotOwned, "Contact method not owned", detail)
	}
	if err != nil {
		return contact.ContactMethod{}, contact.ContactMethodVersion{}, internalStoreError()
	}
	method.CurrentVersionID = version.ID
	return method, version, nil
}

func lockTransactionContactVersionForOwner(ctx context.Context, q queryer, methodID, ownerID, field, detail string) (contact.ContactMethod, contact.ContactMethodVersion, *domain.AppError) {
	method, version, appErr := lockContactVersionForOwner(ctx, q, methodID, ownerID, detail)
	if appErr != nil || !contact.TransactionContactEligible(method) {
		return contact.ContactMethod{}, contact.ContactMethodVersion{}, contact.TransactionContactRequiredError(field, detail)
	}
	return method, version, nil
}

func getContactMethod(ctx context.Context, q queryer, userID, methodID string) (contact.ContactMethod, *domain.AppError) {
	var method contact.ContactMethod
	err := q.QueryRow(ctx, `
		SELECT m.id::text, m.user_id::text, m.type, m.label, COALESCE(v.masked_value, ''), m.enabled,
		       m.is_default, m.verified_at, COALESCE(m.current_version_id::text, ''), m.created_at, m.updated_at, m.version
		FROM contact_methods m
		LEFT JOIN contact_method_versions v ON v.id = m.current_version_id
		WHERE m.id = $1 AND m.user_id = $2 AND m.enabled = true
	`, methodID, userID).Scan(
		&method.ID,
		&method.UserID,
		&method.Type,
		&method.Label,
		&method.MaskedValue,
		&method.Enabled,
		&method.IsDefault,
		&method.VerifiedAt,
		&method.CurrentVersionID,
		&method.CreatedAt,
		&method.UpdatedAt,
		&method.Version,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return contact.ContactMethod{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Contact method not found", "联系方式不存在。")
	}
	if err != nil {
		return contact.ContactMethod{}, internalStoreError()
	}
	return method, nil
}

func (s *Store) getContactMethodWithValue(ctx context.Context, q queryer, userID, methodID string) (contact.ContactMethod, *domain.AppError) {
	var method contact.ContactMethod
	var ciphertext, nonce []byte
	var keyVersion, cipherFormat string
	err := q.QueryRow(ctx, `
		SELECT m.id::text, m.user_id::text, m.type, m.label, COALESCE(v.masked_value, ''),
		       v.value_ciphertext, v.value_nonce, COALESCE(v.encryption_key_version, ''),
		       COALESCE(v.encryption_format, ''), m.enabled, m.is_default, m.verified_at,
		       COALESCE(m.current_version_id::text, ''), m.created_at, m.updated_at, m.version
		FROM contact_methods m
		LEFT JOIN contact_method_versions v ON v.id = m.current_version_id
		WHERE m.id = $1 AND m.user_id = $2 AND m.enabled = true
	`, methodID, userID).Scan(
		&method.ID,
		&method.UserID,
		&method.Type,
		&method.Label,
		&method.MaskedValue,
		&ciphertext,
		&nonce,
		&keyVersion,
		&cipherFormat,
		&method.Enabled,
		&method.IsDefault,
		&method.VerifiedAt,
		&method.CurrentVersionID,
		&method.CreatedAt,
		&method.UpdatedAt,
		&method.Version,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return contact.ContactMethod{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Contact method not found", "联系方式不存在。")
	}
	if err != nil {
		return contact.ContactMethod{}, internalStoreError()
	}
	if len(ciphertext) > 0 {
		value, err := s.contactCodec.decode(ciphertext, nonce, keyVersion, cipherFormat, method.CurrentVersionID, contactFieldMethodValue)
		if err != nil {
			return contact.ContactMethod{}, internalStoreError()
		}
		method.DisplayValue = value
	}
	return method, nil
}

func scanContactMethods(rows pgx.Rows) ([]contact.ContactMethod, *domain.AppError) {
	methods := []contact.ContactMethod{}
	for rows.Next() {
		var method contact.ContactMethod
		if err := rows.Scan(
			&method.ID,
			&method.UserID,
			&method.Type,
			&method.Label,
			&method.MaskedValue,
			&method.Enabled,
			&method.IsDefault,
			&method.VerifiedAt,
			&method.CurrentVersionID,
			&method.CreatedAt,
			&method.UpdatedAt,
			&method.Version,
		); err != nil {
			return nil, internalStoreError()
		}
		methods = append(methods, method)
	}
	if err := rows.Err(); err != nil {
		return nil, internalStoreError()
	}
	return methods, nil
}

func (s *Store) scanContactMethodsWithValues(rows pgx.Rows) ([]contact.ContactMethod, *domain.AppError) {
	methods := []contact.ContactMethod{}
	for rows.Next() {
		var method contact.ContactMethod
		var ciphertext, nonce []byte
		var keyVersion, cipherFormat string
		if err := rows.Scan(
			&method.ID,
			&method.UserID,
			&method.Type,
			&method.Label,
			&method.MaskedValue,
			&ciphertext,
			&nonce,
			&keyVersion,
			&cipherFormat,
			&method.Enabled,
			&method.IsDefault,
			&method.VerifiedAt,
			&method.CurrentVersionID,
			&method.CreatedAt,
			&method.UpdatedAt,
			&method.Version,
		); err != nil {
			return nil, internalStoreError()
		}
		if len(ciphertext) > 0 {
			value, err := s.contactCodec.decode(ciphertext, nonce, keyVersion, cipherFormat, method.CurrentVersionID, contactFieldMethodValue)
			if err != nil {
				return nil, internalStoreError()
			}
			method.DisplayValue = value
		}
		methods = append(methods, method)
	}
	if err := rows.Err(); err != nil {
		return nil, internalStoreError()
	}
	return methods, nil
}
