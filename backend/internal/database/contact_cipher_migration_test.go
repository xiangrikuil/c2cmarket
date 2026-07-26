package database

import (
	"strings"
	"testing"
)

func TestContactCipherAADMigrationContract(t *testing.T) {
	t.Parallel()

	upSQL := readMigrationForTest(t, "000064_contact_cipher_aad.up.sql")
	for _, required := range []string{
		"ALTER TABLE contact_method_versions",
		"ALTER TABLE model_audit_targets",
		"ALTER TABLE api_order_delivery_credentials",
		"ALTER TABLE api_quota_credentials",
		"legacy_no_aad_v1",
		"aad_v1",
		"ck_contact_method_versions_encryption_format",
		"ck_model_audit_targets_encryption_format",
		"ck_api_order_delivery_credentials_encryption_format",
		"ck_api_quota_credentials_encryption_format",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("contact cipher migration missing %q", required)
		}
	}

	downSQL := readMigrationForTest(t, "000064_contact_cipher_aad.down.sql")
	for _, required := range []string{
		"DROP COLUMN IF EXISTS encryption_format",
		"DROP COLUMN IF EXISTS api_key_encryption_format",
		"DROP COLUMN IF EXISTS secret_encryption_format",
	} {
		if !strings.Contains(downSQL, required) {
			t.Fatalf("contact cipher rollback missing %q", required)
		}
	}
}
