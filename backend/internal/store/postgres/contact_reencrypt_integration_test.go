package postgres

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"os"
	"strings"
	"testing"
	"time"

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

func TestPostgresContactReencryptAndLifecycleSkipDestroyedOrLockedAPICredentials(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("C2C_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	const (
		legacyKey  = "api-credential-reencrypt-legacy-key"
		currentKey = "api-credential-reencrypt-current-key"
		secret     = "api-credential-reencrypt-secret"
	)
	store, err := ConnectWithContactCrypto(ctx, databaseURL, ContactCryptoConfig{
		EncryptionKeyVersion:  "test-v2",
		FingerprintKeyVersion: "test-v2",
		EncryptionKeys: map[string]string{
			"test-v1": legacyKey,
			"test-v2": currentKey,
		},
		FingerprintKeys: map[string]string{
			"test-v1": "api-credential-reencrypt-legacy-fingerprint",
			"test-v2": "api-credential-reencrypt-current-fingerprint",
		},
	})
	if err != nil {
		t.Fatalf("connect API credential re-encryption store: %v", err)
	}
	t.Cleanup(store.Close)

	completedAt := time.Date(2020, 1, 2, 12, 0, 0, 0, time.UTC)
	runAt := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	sellerID := uuid.NewString()
	sellerContactID := uuid.NewString()
	buyerID := uuid.NewString()
	buyerContactID := uuid.NewString()
	serviceID := uuid.NewString()
	quotaOrderID := ""
	t.Cleanup(func() {
		cleanupLifecycleCredentialFixtures(t, context.Background(), store, sellerID, buyerID, quotaOrderID)
	})
	seedQuotaServiceForTest(t, ctx, store.pool, sellerID, sellerContactID, buyerID, buyerContactID, serviceID, completedAt)

	destroyed := insertLifecycleCompletedQuotaCredentialOrder(
		t, store, serviceID, sellerID, sellerContactID, buyerID, buyerContactID,
		completedAt, completedAt.Add(-time.Hour), completedAt.Add(24*time.Hour),
	)
	quotaOrderID = destroyed.Order.OrderID
	if _, err := store.pool.Exec(ctx, `
		UPDATE api_order_delivery_credentials
		SET api_base_url = NULL, panel_login_url = NULL, username = NULL, instructions = NULL,
		    api_key_ciphertext = NULL, api_key_nonce = NULL,
		    password_ciphertext = NULL, password_nonce = NULL,
		    destroyed_at = $2, destroy_reason = 'retention_expired'
		WHERE id = $1
	`, destroyed.Order.CredentialID, runAt); err != nil {
		t.Fatalf("stage destroyed API order credential: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		UPDATE api_quota_credentials
		SET api_base_url = NULL, panel_login_url = NULL, username = NULL, instructions = NULL,
		    api_key_ciphertext = NULL, api_key_nonce = NULL,
		    password_ciphertext = NULL, password_nonce = NULL,
		    secret_fingerprint = NULL,
		    destroyed_at = $2, destroy_reason = 'retention_expired', updated_at = $2
		WHERE id = $1
	`, destroyed.CredentialID, runAt); err != nil {
		t.Fatalf("stage destroyed API quota credential: %v", err)
	}
	statsBeforeDestroyedDryRun := store.ContactCryptoStats()
	for _, kind := range []string{ContactReencryptKindAPIQuota, ContactReencryptKindAPIOrder} {
		result, err := store.ReencryptContactCipherBatch(ctx, ContactReencryptOptions{
			Kind: kind, BatchSize: 1000, DryRun: true,
		})
		if err != nil {
			t.Fatalf("dry-run %s with destroyed credential: %v", kind, err)
		}
		if result.Eligible != 0 {
			t.Fatalf("dry-run %s selected destroyed credential: %+v", kind, result)
		}
	}
	if statsAfterDestroyedDryRun := store.ContactCryptoStats(); statsAfterDestroyedDryRun != statsBeforeDestroyedDryRun {
		t.Fatalf("destroyed API credentials were decrypted: before=%+v after=%+v", statsBeforeDestroyedDryRun, statsAfterDestroyedDryRun)
	}

	live := insertLifecycleCompletedQuotaCredentialOrder(
		t, store, serviceID, sellerID, sellerContactID, buyerID, buyerContactID,
		completedAt.Add(48*time.Hour), completedAt.Add(47*time.Hour), completedAt.Add(72*time.Hour),
	)
	legacyCiphertext, legacyNonce := legacyContactCiphertextForTest(t, legacyKey, secret)
	if _, err := store.pool.Exec(ctx, `
		UPDATE api_order_delivery_credentials
		SET api_key_ciphertext = $2, api_key_nonce = $3,
		    secret_encryption_key_version = 'test-v1', secret_encryption_format = 'legacy_no_aad_v1'
		WHERE id = $1
	`, live.Order.CredentialID, legacyCiphertext, legacyNonce); err != nil {
		t.Fatalf("stage live legacy API order credential: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		UPDATE api_quota_credentials
		SET api_key_ciphertext = $2, api_key_nonce = $3,
		    secret_encryption_key_version = 'test-v1', secret_encryption_format = 'legacy_no_aad_v1'
		WHERE id = $1
	`, live.CredentialID, legacyCiphertext, legacyNonce); err != nil {
		t.Fatalf("stage live legacy API quota credential: %v", err)
	}

	lockTx, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin API credential row locks: %v", err)
	}
	defer func() { _ = lockTx.Rollback(context.Background()) }()
	if _, err := lockTx.Exec(ctx, `SELECT id FROM api_quota_credentials WHERE id = $1 FOR UPDATE`, live.CredentialID); err != nil {
		t.Fatalf("lock quota credential for lifecycle: %v", err)
	}
	lifecycleCtx, cancelLifecycle := context.WithTimeout(ctx, time.Second)
	lifecycleTx, err := store.pool.Begin(lifecycleCtx)
	if err != nil {
		cancelLifecycle()
		t.Fatalf("begin lifecycle behind quota lock: %v", err)
	}
	orderDestroyed, quotaDestroyed, err := destroyCompletedAPIOrderCredentialsInTx(
		lifecycleCtx, lifecycleTx, runAt, runAt.Add(-30*24*time.Hour), 10,
	)
	if err == nil {
		err = lifecycleTx.Commit(lifecycleCtx)
	} else {
		_ = lifecycleTx.Rollback(context.Background())
	}
	cancelLifecycle()
	if err != nil {
		t.Fatalf("lifecycle blocked behind quota credential row lock: %v", err)
	}
	if orderDestroyed != 0 || quotaDestroyed != 0 {
		t.Fatalf("lifecycle partially destroyed quota-backed credentials: order=%d quota=%d", orderDestroyed, quotaDestroyed)
	}
	assertLifecycleOrderCredentialState(t, store, live.Order.CredentialID, false, "")
	assertLifecycleQuotaCredentialState(t, store, live.CredentialID, false, "")

	if _, err := lockTx.Exec(ctx, `SELECT id FROM api_order_delivery_credentials WHERE id = $1 FOR UPDATE`, live.Order.CredentialID); err != nil {
		t.Fatalf("lock order credential for re-encryption: %v", err)
	}
	statsBeforeLockedDryRun := store.ContactCryptoStats()
	for _, kind := range []string{ContactReencryptKindAPIQuota, ContactReencryptKindAPIOrder} {
		result, err := store.ReencryptContactCipherBatch(ctx, ContactReencryptOptions{
			Kind: kind, BatchSize: 1000, DryRun: true,
		})
		if err != nil {
			t.Fatalf("dry-run %s behind row lock: %v", kind, err)
		}
		if result.Eligible != 0 {
			t.Fatalf("dry-run %s read a locked MVCC snapshot: %+v", kind, result)
		}
	}
	if statsAfterLockedDryRun := store.ContactCryptoStats(); statsAfterLockedDryRun != statsBeforeLockedDryRun {
		t.Fatalf("locked API credentials were decrypted: before=%+v after=%+v", statsBeforeLockedDryRun, statsAfterLockedDryRun)
	}
	if err := lockTx.Commit(ctx); err != nil {
		t.Fatalf("release API credential row locks: %v", err)
	}

	statsBeforeUnlockedDryRun := store.ContactCryptoStats()
	for _, kind := range []string{ContactReencryptKindAPIQuota, ContactReencryptKindAPIOrder} {
		result, err := store.ReencryptContactCipherBatch(ctx, ContactReencryptOptions{
			Kind: kind, BatchSize: 1000, DryRun: true,
		})
		if err != nil {
			t.Fatalf("dry-run unlocked %s: %v", kind, err)
		}
		if result.Eligible != 1 {
			t.Fatalf("dry-run unlocked %s omitted live credential: %+v", kind, result)
		}
	}
	statsAfterUnlockedDryRun := store.ContactCryptoStats()
	if statsAfterUnlockedDryRun.DecryptSuccessTotal != statsBeforeUnlockedDryRun.DecryptSuccessTotal+2 ||
		statsAfterUnlockedDryRun.DecryptFailureTotal != statsBeforeUnlockedDryRun.DecryptFailureTotal ||
		statsAfterUnlockedDryRun.UnknownKeyTotal != statsBeforeUnlockedDryRun.UnknownKeyTotal {
		t.Fatalf("unexpected unlocked API credential decrypt stats: before=%+v after=%+v", statsBeforeUnlockedDryRun, statsAfterUnlockedDryRun)
	}

	orderDestroyed, quotaDestroyed = runCredentialDestructionBatchForTest(t, store, runAt, 10)
	if orderDestroyed != 1 || quotaDestroyed != 1 {
		t.Fatalf("lifecycle did not destroy unlocked quota-backed credentials together: order=%d quota=%d", orderDestroyed, quotaDestroyed)
	}
	assertLifecycleOrderCredentialState(t, store, live.Order.CredentialID, true, "retention_expired")
	assertLifecycleQuotaCredentialState(t, store, live.CredentialID, true, "retention_expired")
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
