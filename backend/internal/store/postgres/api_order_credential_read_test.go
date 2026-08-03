package postgres

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/module/apiorder"

	"github.com/jackc/pgx/v5"
)

func TestGetAPIOrderDeliveryCredentialSkipsDecryptionAfterDestruction(t *testing.T) {
	t.Parallel()

	codec, err := newContactCodec(ContactCryptoConfig{
		EncryptionKey:         "credential-read-test-encryption-key",
		FingerprintKey:        "credential-read-test-fingerprint-key",
		EncryptionKeyVersion:  "test-v1",
		FingerprintKeyVersion: "test-v1",
	})
	if err != nil {
		t.Fatalf("create contact codec: %v", err)
	}
	destroyedAt := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	submittedAt := destroyedAt.Add(-60 * 24 * time.Hour)
	store := &Store{contactCodec: codec}
	queryer := destroyedCredentialQueryer{row: destroyedCredentialRow{
		values: []any{
			"11111111-1111-1111-1111-111111111111",
			"22222222-2222-2222-2222-222222222222",
			"33333333-3333-3333-3333-333333333333",
			"44444444-4444-4444-4444-444444444444",
			apiorder.DeliveryKindAPIKeyEndpoint,
			"", "", "", "",
			[]byte("invalid-ciphertext"), []byte("invalid-nonce"), []byte(nil), []byte(nil),
			"missing-key-version", contactCipherFormatAADV1,
			submittedAt, submittedAt, destroyedAt, "retention_expired",
		},
	}}

	credential, found, appErr := store.getAPIOrderDeliveryCredential(
		context.Background(),
		queryer,
		"22222222-2222-2222-2222-222222222222",
	)
	if appErr != nil {
		t.Fatalf("read destroyed credential: %v", appErr)
	}
	if !found || credential.DestroyedAt == nil || !credential.DestroyedAt.Equal(destroyedAt) {
		t.Fatalf("destroyed audit facts missing: found=%t credential=%+v", found, credential)
	}
	if credential.APIKey != "" || credential.Password != "" || credential.DestroyReason != "retention_expired" {
		t.Fatalf("destroyed credential leaked secret or lost reason: %+v", credential)
	}
	if stats := codec.stats(); stats.DecryptSuccessTotal != 0 || stats.DecryptFailureTotal != 0 || stats.UnknownKeyTotal != 0 {
		t.Fatalf("destroyed credential attempted decryption: %+v", stats)
	}
}

type destroyedCredentialQueryer struct {
	row pgx.Row
}

func (q destroyedCredentialQueryer) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	if !strings.Contains(sql, "destroyed_at") {
		return destroyedCredentialRow{err: fmt.Errorf("credential query omits destruction state")}
	}
	return q.row
}

type destroyedCredentialRow struct {
	values []any
	err    error
}

func (r destroyedCredentialRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return fmt.Errorf("scan destination count %d does not match value count %d", len(dest), len(r.values))
	}
	for index, value := range r.values {
		switch target := dest[index].(type) {
		case *string:
			*target = value.(string)
		case *[]byte:
			*target = value.([]byte)
		case *time.Time:
			*target = value.(time.Time)
		case **time.Time:
			valueCopy := value.(time.Time)
			*target = &valueCopy
		default:
			return fmt.Errorf("unsupported scan target %T", target)
		}
	}
	return nil
}
