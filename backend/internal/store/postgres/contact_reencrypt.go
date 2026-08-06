package postgres

import (
	"context"
	"fmt"
	"strings"

	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/apiquota"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	ContactReencryptKindContactMethods = "contact_methods"
	ContactReencryptKindModelAudit     = "model_audit"
	ContactReencryptKindAPIQuota       = "api_quota"
	ContactReencryptKindAPIOrder       = "api_order"
)

type ContactReencryptOptions struct {
	Kind      string
	Cursor    string
	BatchSize int
	DryRun    bool
}

type ContactReencryptResult struct {
	Kind           string `json:"kind"`
	DryRun         bool   `json:"dryRun"`
	Scanned        int    `json:"scanned"`
	Eligible       int    `json:"eligible"`
	Reencrypted    int    `json:"reencrypted"`
	NextCursor     string `json:"nextCursor,omitempty"`
	Done           bool   `json:"done"`
	CurrentVersion string `json:"currentVersion"`
}

type cipherReencryptRow struct {
	ID              string
	DeliveryKind    string
	APIBaseURL      string
	PanelLoginURL   string
	Username        string
	Ciphertext      []byte
	Nonce           []byte
	KeyVersion      string
	CipherFormat    string
	FingerprintText string
}

func (s *Store) ReencryptContactCipherBatch(ctx context.Context, options ContactReencryptOptions) (ContactReencryptResult, error) {
	result := ContactReencryptResult{
		Kind:   strings.TrimSpace(options.Kind),
		DryRun: options.DryRun,
	}
	if s == nil || s.pool == nil || s.contactCodec == nil {
		return result, fmt.Errorf("contact re-encryption store is not configured")
	}
	if options.BatchSize < 1 || options.BatchSize > 1000 {
		return result, fmt.Errorf("batch size must be between 1 and 1000")
	}
	if options.Cursor != "" {
		if _, err := uuid.Parse(options.Cursor); err != nil {
			return result, fmt.Errorf("cursor must be a UUID")
		}
	}
	switch result.Kind {
	case ContactReencryptKindContactMethods, ContactReencryptKindModelAudit, ContactReencryptKindAPIQuota, ContactReencryptKindAPIOrder:
	default:
		return result, fmt.Errorf("unsupported contact re-encryption kind %q", result.Kind)
	}
	result.CurrentVersion = s.contactCodec.encryptionKeyVersion

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return result, fmt.Errorf("begin contact re-encryption transaction: %w", err)
	}
	defer rollback(ctx, tx)

	rows, err := s.loadContactReencryptRows(ctx, tx, options)
	if err != nil {
		return result, err
	}
	result.Scanned = len(rows)
	result.Eligible = len(rows)
	if len(rows) > 0 {
		result.NextCursor = rows[len(rows)-1].ID
	}
	result.Done = len(rows) < options.BatchSize

	for _, row := range rows {
		if err := s.reencryptContactRow(ctx, tx, result.Kind, row, options.DryRun); err != nil {
			return result, err
		}
		if !options.DryRun {
			result.Reencrypted++
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return result, fmt.Errorf("commit contact re-encryption transaction: %w", err)
	}
	return result, nil
}

func (s *Store) loadContactReencryptRows(ctx context.Context, tx pgx.Tx, options ContactReencryptOptions) ([]cipherReencryptRow, error) {
	const cursorClause = `($1 = '' OR id > NULLIF($1, '')::uuid)`
	var query string
	switch options.Kind {
	case ContactReencryptKindContactMethods:
		query = `
			SELECT id::text, '', '', '', '', value_ciphertext, value_nonce,
			       encryption_key_version, encryption_format
			FROM contact_method_versions
			WHERE ` + cursorClause + `
			  AND (encryption_key_version <> $2 OR encryption_format <> $3)
			ORDER BY id
			LIMIT $4`
	case ContactReencryptKindModelAudit:
		query = `
			SELECT id::text, '', '', '', '', api_key_ciphertext, api_key_nonce,
			       api_key_key_version, api_key_encryption_format
			FROM model_audit_targets
			WHERE ` + cursorClause + `
			  AND (api_key_key_version <> $2 OR api_key_encryption_format <> $3)
			ORDER BY id
			LIMIT $4`
	case ContactReencryptKindAPIQuota:
		query = `
			SELECT id::text, delivery_kind, COALESCE(api_base_url, ''),
			       COALESCE(panel_login_url, ''), COALESCE(username, ''),
			       COALESCE(api_key_ciphertext, password_ciphertext),
			       COALESCE(api_key_nonce, password_nonce),
			       secret_encryption_key_version, secret_encryption_format
				FROM api_quota_credentials
				WHERE ` + cursorClause + `
				  AND destroyed_at IS NULL
				  AND (secret_encryption_key_version <> $2 OR secret_encryption_format <> $3)
			ORDER BY id
			LIMIT $4`
	case ContactReencryptKindAPIOrder:
		query = `
			SELECT id::text, delivery_kind, '', '', '',
			       COALESCE(api_key_ciphertext, password_ciphertext),
			       COALESCE(api_key_nonce, password_nonce),
			       secret_encryption_key_version, secret_encryption_format
				FROM api_order_delivery_credentials
				WHERE ` + cursorClause + `
				  AND destroyed_at IS NULL
				  AND (secret_encryption_key_version <> $2 OR secret_encryption_format <> $3)
			ORDER BY id
			LIMIT $4`
	}
	if !options.DryRun || options.Kind == ContactReencryptKindAPIQuota || options.Kind == ContactReencryptKindAPIOrder {
		query += " FOR UPDATE SKIP LOCKED"
	}
	dbRows, err := tx.Query(ctx, query, options.Cursor, s.contactCodec.encryptionKeyVersion, contactCipherFormatAADV1, options.BatchSize)
	if err != nil {
		return nil, fmt.Errorf("select contact re-encryption batch: %w", err)
	}
	defer dbRows.Close()

	rows := make([]cipherReencryptRow, 0, options.BatchSize)
	for dbRows.Next() {
		var row cipherReencryptRow
		if err := dbRows.Scan(
			&row.ID,
			&row.DeliveryKind,
			&row.APIBaseURL,
			&row.PanelLoginURL,
			&row.Username,
			&row.Ciphertext,
			&row.Nonce,
			&row.KeyVersion,
			&row.CipherFormat,
		); err != nil {
			return nil, fmt.Errorf("scan contact re-encryption batch: %w", err)
		}
		rows = append(rows, row)
	}
	if err := dbRows.Err(); err != nil {
		return nil, fmt.Errorf("read contact re-encryption batch: %w", err)
	}
	return rows, nil
}

func (s *Store) reencryptContactRow(ctx context.Context, tx pgx.Tx, kind string, row cipherReencryptRow, dryRun bool) error {
	sourceField, targetField, err := contactReencryptFields(kind, row.DeliveryKind)
	if err != nil {
		return err
	}
	plaintext, err := s.contactCodec.decode(row.Ciphertext, row.Nonce, row.KeyVersion, row.CipherFormat, row.ID, sourceField)
	if err != nil {
		return fmt.Errorf("decrypt %s record %s: %w", kind, row.ID, err)
	}
	if dryRun {
		return nil
	}
	encoded, err := s.contactCodec.encode(plaintext, row.ID, targetField)
	if err != nil {
		return fmt.Errorf("encrypt %s record %s: %w", kind, row.ID, err)
	}

	var command pgconn.CommandTag
	switch kind {
	case ContactReencryptKindContactMethods:
		command, err = tx.Exec(ctx, `
			UPDATE contact_method_versions
			SET value_ciphertext = $2, value_nonce = $3, value_fingerprint = $4,
			    encryption_key_version = $5, fingerprint_key_version = $6,
			    encryption_format = $7
			WHERE id = $1 AND encryption_key_version = $8 AND encryption_format = $9
		`, row.ID, encoded.Ciphertext, encoded.Nonce, encoded.Fingerprint,
			encoded.EncryptionKeyVersion, encoded.FingerprintKeyVersion, encoded.CipherFormat,
			row.KeyVersion, row.CipherFormat)
	case ContactReencryptKindModelAudit:
		command, err = tx.Exec(ctx, `
			UPDATE model_audit_targets
			SET api_key_ciphertext = $2, api_key_nonce = $3, api_key_fingerprint = $4,
			    api_key_key_version = $5, api_key_encryption_format = $6
			WHERE id = $1 AND api_key_key_version = $7 AND api_key_encryption_format = $8
		`, row.ID, encoded.Ciphertext, encoded.Nonce, encoded.Fingerprint,
			encoded.EncryptionKeyVersion, encoded.CipherFormat, row.KeyVersion, row.CipherFormat)
	case ContactReencryptKindAPIQuota:
		fingerprint := s.contactCodec.fingerprint(apiQuotaCredentialFingerprintMaterial(apiquota.CredentialImportRow{
			DeliveryKind:  row.DeliveryKind,
			APIBaseURL:    row.APIBaseURL,
			APIKey:        selectContactSecret(row.DeliveryKind, plaintext, ""),
			PanelLoginURL: row.PanelLoginURL,
			Username:      row.Username,
			Password:      selectContactSecret(row.DeliveryKind, "", plaintext),
		}))
		if row.DeliveryKind == apiorder.DeliveryKindAPIKeyEndpoint {
			command, err = tx.Exec(ctx, `
				UPDATE api_quota_credentials
				SET api_key_ciphertext = $2, api_key_nonce = $3,
				    secret_encryption_key_version = $4, secret_encryption_format = $5,
				    secret_fingerprint = decode($6, 'hex'), updated_at = now()
				WHERE id = $1 AND secret_encryption_key_version = $7 AND secret_encryption_format = $8
			`, row.ID, encoded.Ciphertext, encoded.Nonce, encoded.EncryptionKeyVersion,
				encoded.CipherFormat, fingerprint, row.KeyVersion, row.CipherFormat)
		} else {
			command, err = tx.Exec(ctx, `
				UPDATE api_quota_credentials
				SET password_ciphertext = $2, password_nonce = $3,
				    secret_encryption_key_version = $4, secret_encryption_format = $5,
				    secret_fingerprint = decode($6, 'hex'), updated_at = now()
				WHERE id = $1 AND secret_encryption_key_version = $7 AND secret_encryption_format = $8
			`, row.ID, encoded.Ciphertext, encoded.Nonce, encoded.EncryptionKeyVersion,
				encoded.CipherFormat, fingerprint, row.KeyVersion, row.CipherFormat)
		}
	case ContactReencryptKindAPIOrder:
		if row.DeliveryKind == apiorder.DeliveryKindAPIKeyEndpoint {
			command, err = tx.Exec(ctx, `
				UPDATE api_order_delivery_credentials
				SET api_key_ciphertext = $2, api_key_nonce = $3,
				    secret_encryption_key_version = $4, secret_encryption_format = $5
				WHERE id = $1 AND secret_encryption_key_version = $6 AND secret_encryption_format = $7
			`, row.ID, encoded.Ciphertext, encoded.Nonce, encoded.EncryptionKeyVersion,
				encoded.CipherFormat, row.KeyVersion, row.CipherFormat)
		} else {
			command, err = tx.Exec(ctx, `
				UPDATE api_order_delivery_credentials
				SET password_ciphertext = $2, password_nonce = $3,
				    secret_encryption_key_version = $4, secret_encryption_format = $5
				WHERE id = $1 AND secret_encryption_key_version = $6 AND secret_encryption_format = $7
			`, row.ID, encoded.Ciphertext, encoded.Nonce, encoded.EncryptionKeyVersion,
				encoded.CipherFormat, row.KeyVersion, row.CipherFormat)
		}
	}
	if err != nil {
		return fmt.Errorf("update %s record %s: %w", kind, row.ID, err)
	}
	if command.RowsAffected() != 1 {
		return fmt.Errorf("update %s record %s lost ownership", kind, row.ID)
	}
	return nil
}

func contactReencryptFields(kind, deliveryKind string) (string, string, error) {
	switch kind {
	case ContactReencryptKindContactMethods:
		return contactFieldMethodValue, contactFieldMethodValue, nil
	case ContactReencryptKindModelAudit:
		return contactFieldModelAPIKey, contactFieldModelAPIKey, nil
	case ContactReencryptKindAPIQuota:
		switch deliveryKind {
		case apiorder.DeliveryKindAPIKeyEndpoint:
			return contactFieldQuotaAPIKey, contactFieldQuotaAPIKey, nil
		case apiorder.DeliveryKindLoginAccount:
			return contactFieldQuotaPassword, contactFieldQuotaPassword, nil
		}
	case ContactReencryptKindAPIOrder:
		switch deliveryKind {
		case apiorder.DeliveryKindAPIKeyEndpoint:
			return contactFieldOrderAPIKey, contactFieldOrderAPIKey, nil
		case apiorder.DeliveryKindLoginAccount:
			return contactFieldOrderPassword, contactFieldOrderPassword, nil
		}
	}
	return "", "", fmt.Errorf("unsupported encrypted field for kind %q", kind)
}

func selectContactSecret(deliveryKind, apiKey, password string) string {
	if deliveryKind == apiorder.DeliveryKindAPIKeyEndpoint {
		return apiKey
	}
	return password
}
