package postgres

import (
	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/contact"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"net/http"
	"time"
)

func (s *Store) CreateContactMethod(ctx context.Context, input contact.ContactMethodInput, method contact.ContactMethod, version contact.ContactMethodVersion) *domain.AppError {
	if s == nil || s.pool == nil || s.contactCodec == nil {
		return internalStoreError()
	}
	encoded, err := s.contactCodec.encode(input.Value)
	if err != nil {
		return internalStoreError()
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return internalStoreError()
	}
	defer rollback(ctx, tx)

	_, err = tx.Exec(ctx, `
		INSERT INTO contact_methods (
			id, user_id, type, label, current_version_id, is_default, enabled, created_at, updated_at, version
		)
		VALUES ($1, $2, $3, $4, NULL, $5, $6, $7, $8, $9)
	`, method.ID, method.UserID, method.Type, method.Label, false, method.Enabled, method.CreatedAt, method.UpdatedAt, method.Version)
	if err != nil {
		return internalStoreError()
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO contact_method_versions (
			id, contact_method_id, owner_user_id, value_ciphertext, value_nonce,
			masked_value, value_fingerprint, encryption_key_version, fingerprint_key_version, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, version.ID, version.ContactMethodID, version.OwnerUserID, encoded.Ciphertext, encoded.Nonce,
		version.MaskedValue, encoded.Fingerprint, encoded.EncryptionKeyVersion, encoded.FingerprintKeyVersion, version.CreatedAt)
	if err != nil {
		return internalStoreError()
	}

	_, err = tx.Exec(ctx, `
		UPDATE contact_methods
		SET current_version_id = $2
		WHERE id = $1
	`, method.ID, version.ID)
	if err != nil {
		return internalStoreError()
	}
	if method.IsDefault {
		_, err = tx.Exec(ctx, `
			UPDATE contact_methods
			SET is_default = false, updated_at = $2, version = version + 1
			WHERE user_id = $1 AND is_default = true
		`, method.UserID, method.UpdatedAt)
		if err != nil {
			return internalStoreError()
		}
		_, err = tx.Exec(ctx, `
			UPDATE contact_methods
			SET is_default = true
			WHERE id = $1 AND user_id = $2
		`, method.ID, method.UserID)
		if err != nil {
			return internalStoreError()
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) ListContactMethods(ctx context.Context, userID string) ([]contact.ContactMethod, *domain.AppError) {
	if s == nil || s.pool == nil || s.contactCodec == nil {
		return nil, internalStoreError()
	}
	rows, err := s.pool.Query(ctx, `
		SELECT m.id::text, m.user_id::text, m.type, m.label, COALESCE(v.masked_value, ''),
		       v.value_ciphertext, v.value_nonce, m.enabled, m.is_default, m.verified_at,
		       COALESCE(m.current_version_id::text, ''), m.created_at, m.updated_at, m.version
		FROM contact_methods m
		LEFT JOIN contact_method_versions v ON v.id = m.current_version_id
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
	encoded, err := s.contactCodec.encode(input.Value)
	if err != nil {
		return contact.ContactMethod{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return contact.ContactMethod{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	var current contact.ContactMethod
	err = tx.QueryRow(ctx, `
		SELECT id::text, user_id::text, type, label, enabled, is_default, verified_at,
		       COALESCE(current_version_id::text, ''), created_at, updated_at, version
		FROM contact_methods
		WHERE id = $1 AND user_id = $2 AND enabled = true
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
	version.ContactMethodID = current.ID
	version.OwnerUserID = current.UserID
	method.CurrentVersionID = version.ID
	if input.Type != current.Type {
		method.VerifiedAt = nil
	} else {
		method.VerifiedAt = current.VerifiedAt
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO contact_method_versions (
			id, contact_method_id, owner_user_id, value_ciphertext, value_nonce,
			masked_value, value_fingerprint, encryption_key_version, fingerprint_key_version, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, version.ID, version.ContactMethodID, version.OwnerUserID, encoded.Ciphertext, encoded.Nonce,
		version.MaskedValue, encoded.Fingerprint, encoded.EncryptionKeyVersion, encoded.FingerprintKeyVersion, version.CreatedAt)
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
	if method.IsDefault {
		_, err = tx.Exec(ctx, `
			UPDATE contact_methods
			SET is_default = false, updated_at = $3, version = version + 1
			WHERE user_id = $1 AND id <> $2 AND is_default = true
		`, method.UserID, method.ID, method.UpdatedAt)
		if err != nil {
			return contact.ContactMethod{}, internalStoreError()
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
		return contact.ContactMethod{}, internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return contact.ContactMethod{}, internalStoreError()
	}
	return method, nil
}

func (s *Store) DeleteContactMethod(ctx context.Context, userID, methodID string) (contact.ContactMethod, *domain.AppError) {
	if s == nil || s.pool == nil {
		return contact.ContactMethod{}, internalStoreError()
	}
	var method contact.ContactMethod
	err := s.pool.QueryRow(ctx, `
		UPDATE contact_methods
		SET enabled = false, is_default = false, updated_at = now(), version = version + 1
		WHERE id = $1 AND user_id = $2 AND enabled = true
		RETURNING id::text, user_id::text, type, label, '', enabled, is_default, verified_at,
		          COALESCE(current_version_id::text, ''), created_at, updated_at, version
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

func (s *Store) SetDefaultContactMethod(ctx context.Context, userID, methodID string) (contact.ContactMethod, *domain.AppError) {
	if s == nil || s.pool == nil || s.contactCodec == nil {
		return contact.ContactMethod{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return contact.ContactMethod{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	var exists bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM contact_methods WHERE id = $1 AND user_id = $2 AND enabled = true)`, methodID, userID).Scan(&exists)
	if err != nil {
		return contact.ContactMethod{}, internalStoreError()
	}
	if !exists {
		return contact.ContactMethod{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Contact method not found", "联系方式不存在。")
	}
	_, err = tx.Exec(ctx, `
		UPDATE contact_methods
		SET is_default = (id = $2), updated_at = CASE WHEN id = $2 THEN now() ELSE updated_at END,
		    version = CASE WHEN id = $2 THEN version + 1 ELSE version END
		WHERE user_id = $1 AND enabled = true
	`, userID, methodID)
	if err != nil {
		return contact.ContactMethod{}, internalStoreError()
	}
	method, appErr := s.getContactMethodWithValue(ctx, tx, userID, methodID)
	if appErr != nil {
		return contact.ContactMethod{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return contact.ContactMethod{}, internalStoreError()
	}
	return method, nil
}

func (s *Store) VerifyContactMethod(ctx context.Context, userID, methodID string, verifiedAt time.Time) (contact.ContactMethod, *domain.AppError) {
	if s == nil || s.pool == nil || s.contactCodec == nil {
		return contact.ContactMethod{}, internalStoreError()
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE contact_methods
		SET verified_at = $3, updated_at = $3, version = version + 1
		WHERE id = $1 AND user_id = $2 AND enabled = true
	`, methodID, userID, verifiedAt)
	if err != nil {
		return contact.ContactMethod{}, internalStoreError()
	}
	return s.getContactMethodWithValue(ctx, s.pool, userID, methodID)
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

	_, buyerVersion, appErr := lockContactVersionForOwner(ctx, tx, input.BuyerContactMethodID, input.BuyerUserID, "买家联系方式不可用或不属于当前用户。")
	if appErr != nil {
		return contact.ContactSession{}, appErr
	}
	_, sellerVersion, appErr := lockContactVersionForOwner(ctx, tx, input.SellerContactMethodID, input.SellerUserID, "商户联系方式不可用或归属不正确。")
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
	var endsAt time.Time
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
	if status != "open" || !now.Before(endsAt) {
		if status == "open" && !now.Before(endsAt) {
			_, _ = tx.Exec(ctx, `UPDATE contact_sessions SET status = 'expired' WHERE id = $1 AND status = 'open'`, sessionID)
		}
		return contact.ContactSessionView{}, domain.NewError(http.StatusConflict, domain.CodeContactWindowExpired, "Contact window expired", "联系窗口已过期。")
	}

	rows, err := tx.Query(ctx, `
		SELECT i.side, i.subject_user_id::text, m.type, m.label, v.value_ciphertext, v.value_nonce, v.masked_value
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
		if err := rows.Scan(&item.Side, &item.SubjectID, &item.Type, &item.Label, &ciphertext, &nonce, &item.MaskedValue); err != nil {
			return contact.ContactSessionView{}, internalStoreError()
		}
		value, err := s.contactCodec.decode(ciphertext, nonce)
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
	err := q.QueryRow(ctx, `
		SELECT m.id::text, m.user_id::text, m.type, m.label, COALESCE(v.masked_value, ''),
		       v.value_ciphertext, v.value_nonce, m.enabled, m.is_default, m.verified_at,
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
		value, err := s.contactCodec.decode(ciphertext, nonce)
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
		if err := rows.Scan(
			&method.ID,
			&method.UserID,
			&method.Type,
			&method.Label,
			&method.MaskedValue,
			&ciphertext,
			&nonce,
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
			value, err := s.contactCodec.decode(ciphertext, nonce)
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
