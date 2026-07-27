package postgres

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestPostgresContactReencryptDryRunAndApply(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("C2C_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	const (
		legacyKey  = "contact-reencrypt-legacy-key"
		currentKey = "contact-reencrypt-current-key"
		secret     = "test-api-key-value"
	)
	store, err := ConnectWithContactCrypto(ctx, databaseURL, ContactCryptoConfig{
		EncryptionKeyVersion:  "test-v2",
		FingerprintKeyVersion: "test-v2",
		EncryptionKeys: map[string]string{
			"test-v1": legacyKey,
			"test-v2": currentKey,
		},
		FingerprintKeys: map[string]string{
			"test-v1": "contact-reencrypt-legacy-fingerprint",
			"test-v2": "contact-reencrypt-current-fingerprint",
		},
	})
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer store.Close()

	cursor := "ffffffff-ffff-ffff-fffe-000000000000"
	targetID := "ffffffff-ffff-ffff-fffe-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	ciphertext, nonce := legacyContactCiphertextForTest(t, legacyKey, secret)
	_, err = store.pool.Exec(ctx, `
		INSERT INTO model_audit_targets (
			id, name, base_url, provider_type, api_key_ciphertext, api_key_nonce,
			api_key_fingerprint, api_key_key_version, api_key_encryption_format,
			claimed_model, enabled
		) VALUES ($1, 'reencrypt-test', 'https://api.example.com', 'openai_compatible',
			$2, $3, 'legacy-fingerprint', 'test-v1', 'legacy_no_aad_v1', 'test-model', true)
	`, targetID, ciphertext, nonce)
	if err != nil {
		t.Fatalf("insert legacy cipher fixture; apply migration 64 first: %v", err)
	}
	defer func() {
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM model_audit_targets WHERE id = $1`, targetID)
	}()

	dryRun, err := store.ReencryptContactCipherBatch(ctx, ContactReencryptOptions{
		Kind:      ContactReencryptKindModelAudit,
		Cursor:    cursor,
		BatchSize: 1000,
		DryRun:    true,
	})
	if err != nil {
		t.Fatalf("dry-run re-encryption: %v", err)
	}
	if dryRun.Eligible < 1 || dryRun.Reencrypted != 0 {
		t.Fatalf("unexpected dry-run result: %+v", dryRun)
	}
	var keyVersion, cipherFormat string
	if err := store.pool.QueryRow(ctx, `
		SELECT api_key_key_version, api_key_encryption_format
		FROM model_audit_targets WHERE id = $1
	`, targetID).Scan(&keyVersion, &cipherFormat); err != nil {
		t.Fatalf("read dry-run fixture: %v", err)
	}
	if keyVersion != "test-v1" || cipherFormat != contactCipherFormatLegacy {
		t.Fatalf("dry-run mutated fixture: version=%q format=%q", keyVersion, cipherFormat)
	}

	applied, err := store.ReencryptContactCipherBatch(ctx, ContactReencryptOptions{
		Kind:      ContactReencryptKindModelAudit,
		Cursor:    cursor,
		BatchSize: 1000,
		DryRun:    false,
	})
	if err != nil {
		t.Fatalf("apply re-encryption: %v", err)
	}
	if applied.Reencrypted < 1 {
		t.Fatalf("expected fixture re-encrypted: %+v", applied)
	}
	target, decoded, appErr := store.GetModelAuditTargetSecret(ctx, targetID)
	if appErr != nil {
		t.Fatalf("read re-encrypted target: %v", appErr)
	}
	if target.ID != targetID || decoded != secret {
		t.Fatalf("unexpected re-encrypted target id=%q decoded_match=%t", target.ID, decoded == secret)
	}
}

func legacyContactCiphertextForTest(t *testing.T, key, plaintext string) ([]byte, []byte) {
	t.Helper()
	derived := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(derived[:])
	if err != nil {
		t.Fatalf("new legacy cipher: %v", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("new legacy GCM: %v", err)
	}
	nonce := make([]byte, aead.NonceSize())
	return aead.Seal(nil, nonce, []byte(plaintext), nil), nonce
}
