package database

import (
	"strings"
	"testing"
)

func TestAPIDeliveryCredentialDestructionMigrationContract(t *testing.T) {
	t.Parallel()

	upSQL := readMigrationForTest(t, "000076_api_delivery_credential_destruction.up.sql")
	for _, required := range []string{
		"ADD COLUMN destroyed_at timestamptz",
		"ADD COLUMN destroy_reason text",
		"DROP CONSTRAINT IF EXISTS api_order_delivery_credentials_check1",
		"DROP CONSTRAINT IF EXISTS api_quota_credentials_check",
		"DROP CONSTRAINT IF EXISTS api_quota_credentials_check1",
		"ALTER COLUMN secret_fingerprint DROP NOT NULL",
		"destroy_reason IN ('retention_expired', 'retired_unused')",
		"destroyed_at IS NULL",
		"secret_fingerprint IS NOT NULL",
		"octet_length(secret_fingerprint) > 0",
		"destroyed_at IS NOT NULL",
		"secret_fingerprint IS NULL",
		"api_key_ciphertext IS NULL AND api_key_nonce IS NULL",
		"password_ciphertext IS NULL AND password_nonce IS NULL",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("credential destruction migration missing %q", required)
		}
	}

	downSQL := readMigrationForTest(t, "000076_api_delivery_credential_destruction.down.sql")
	for _, required := range []string{
		"cannot roll back credential destruction after secret material has been destroyed",
		"ADD CONSTRAINT api_quota_credentials_check",
		"ADD CONSTRAINT api_quota_credentials_check1",
		"ADD CONSTRAINT api_quota_credentials_check2",
		"status = 'available' AND reserved_order_id IS NULL AND reserved_at IS NULL AND delivered_at IS NULL AND retired_at IS NULL",
		"status = 'reserved' AND reserved_order_id IS NOT NULL AND reserved_at IS NOT NULL AND delivered_at IS NULL AND retired_at IS NULL",
		"status = 'delivered' AND reserved_order_id IS NOT NULL AND reserved_at IS NOT NULL AND delivered_at IS NOT NULL AND retired_at IS NULL",
		"status = 'retired' AND delivered_at IS NULL AND retired_at IS NOT NULL",
		"ALTER COLUMN secret_fingerprint SET NOT NULL",
		"DROP COLUMN IF EXISTS destroy_reason",
		"DROP COLUMN IF EXISTS destroyed_at",
	} {
		if !strings.Contains(downSQL, required) {
			t.Fatalf("credential destruction rollback missing %q", required)
		}
	}
	if strings.Contains(downSQL, "UPDATE api_order_delivery_credentials") ||
		strings.Contains(downSQL, "UPDATE api_quota_credentials") {
		t.Fatal("credential destruction rollback must not fabricate destroyed secret material")
	}
}
