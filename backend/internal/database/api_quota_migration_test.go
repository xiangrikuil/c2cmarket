package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAPIQuotaMigrationsPreserveInventoryAndCredentialContracts(t *testing.T) {
	t.Parallel()

	quotaSQL := readMigrationForTest(t, "000054_api_quota_offers.up.sql")
	for _, required := range []string{
		"conname = 'ck_api_service_models_sub2api_multiplier'",
		"CHECK (distribution_system <> 'sub2api' OR merchant_multiplier = 1.0000)",
		"purchase_kind text NOT NULL DEFAULT 'api_service'",
		"DROP CONSTRAINT IF EXISTS api_purchase_intents_check3",
		"CHECK (sale_cutoff_at <= expires_at - interval '1 hour')",
		"CREATE TABLE api_quota_inventory_units",
		"DEFAULT 'planned' CHECK (status IN ('planned', 'active', 'closed'))",
		"WHERE status = 'available'",
		"CREATE TABLE api_quota_round_claims",
		"UNIQUE (sale_round_id, buyer_user_id)",
		"quota_performance_unverified_snapshot = true",
	} {
		if !strings.Contains(quotaSQL, required) {
			t.Fatalf("quota migration missing required contract %q", required)
		}
	}
	if strings.Contains(quotaSQL, "distribution_system <> 'sub2api' OR model_multiplier = 1.0000") {
		t.Fatal("quota migration must not force Sub2API offers to 1.0000x")
	}
	quotaDownSQL := readMigrationForTest(t, "000054_api_quota_offers.down.sql")
	if !strings.Contains(quotaDownSQL, "DROP CONSTRAINT IF EXISTS ck_api_service_models_sub2api_multiplier") {
		t.Fatal("quota rollback must restore the canonical migration 53 constraint state")
	}

	credentialSQL := readMigrationForTest(t, "000055_api_quota_credentials.up.sql")
	for _, required := range []string{
		"CREATE TABLE api_quota_credentials",
		"api_key_ciphertext bytea",
		"password_ciphertext bytea",
		"secret_fingerprint bytea NOT NULL",
		"CREATE UNIQUE INDEX ux_api_quota_credentials_seller_fingerprint",
		"CREATE UNIQUE INDEX ux_api_quota_credentials_reserved_order",
		"quota_delivery_mode_snapshot = 'preimported' AND api_quota_credential_id IS NOT NULL",
	} {
		if !strings.Contains(credentialSQL, required) {
			t.Fatalf("credential migration missing required contract %q", required)
		}
	}

	for _, forbidden := range []string{
		"api_key text",
		"password text",
		"credential_json",
	} {
		if strings.Contains(credentialSQL, forbidden) {
			t.Fatalf("credential migration contains plaintext contract %q", forbidden)
		}
	}
}

func TestPublishedMigrationHistoryPrecedesAPIQuotaMigrations(t *testing.T) {
	t.Parallel()

	limitedPackageSQL := readMigrationForTest(t, "000051_api_limited_packages.up.sql")
	for _, required := range []string{
		"ADD COLUMN panel_allowance numeric(18,6)",
		"CREATE TABLE api_service_package_models",
		"ADD COLUMN package_stock_reserved boolean NOT NULL DEFAULT false",
	} {
		if !strings.Contains(limitedPackageSQL, required) {
			t.Fatalf("published migration 51 missing required contract %q", required)
		}
	}
	if strings.Contains(limitedPackageSQL, "CREATE TABLE api_quota_batches") {
		t.Fatal("published migration 51 must not be reused for API quota offers")
	}

	intentCleanupSQL := readMigrationForTest(t, "000052_api_purchase_intent_ordered_constraint_cleanup.up.sql")
	for _, required := range []string{
		"DROP CONSTRAINT IF EXISTS api_purchase_intents_check3",
		"status = 'ordered'",
		"ADD CONSTRAINT ck_api_intent_status_timestamps",
	} {
		if !strings.Contains(intentCleanupSQL, required) {
			t.Fatalf("published migration 52 missing required contract %q", required)
		}
	}
	if strings.Contains(intentCleanupSQL, "CREATE TABLE api_quota_credentials") {
		t.Fatal("published migration 52 must not be reused for API quota credentials")
	}
}

func TestAPIQuotaDependentMigrationsExist(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"000051_api_limited_packages.up.sql",
		"000051_api_limited_packages.down.sql",
		"000052_api_purchase_intent_ordered_constraint_cleanup.up.sql",
		"000052_api_purchase_intent_ordered_constraint_cleanup.down.sql",
		"000053_auth_session_renewal.up.sql",
		"000053_auth_session_renewal.down.sql",
		"000054_api_quota_offers.up.sql",
		"000054_api_quota_offers.down.sql",
		"000055_api_quota_credentials.up.sql",
		"000055_api_quota_credentials.down.sql",
		"000056_api_quota_system_slots.up.sql",
		"000056_api_quota_system_slots.down.sql",
		"000057_reputation_transaction_exclusions.up.sql",
		"000057_reputation_transaction_exclusions.down.sql",
		"000058_reputation_governance.up.sql",
		"000058_reputation_governance.down.sql",
		"000059_transaction_reviews.up.sql",
		"000059_transaction_reviews.down.sql",
		"000060_reputation_engine.up.sql",
		"000060_reputation_engine.down.sql",
		"000061_source_author_verification.up.sql",
		"000061_source_author_verification.down.sql",
		"000062_auth_identity_bootstrap_hardening.up.sql",
		"000062_auth_identity_bootstrap_hardening.down.sql",
		"000063_verification_data_lifecycle.up.sql",
		"000063_verification_data_lifecycle.down.sql",
		"000064_contact_cipher_aad.up.sql",
		"000064_contact_cipher_aad.down.sql",
		"000065_remove_demands.up.sql",
		"000065_remove_demands.down.sql",
		"000066_api_service_multiplier_reconciliation.up.sql",
		"000066_api_service_multiplier_reconciliation.down.sql",
		"000067_api_account_payment_settings.up.sql",
		"000067_api_account_payment_settings.down.sql",
	} {
		if _, err := os.Stat(filepath.Join("..", "..", "migrations", name)); err != nil {
			t.Fatalf("migration file %s is unavailable: %v", name, err)
		}
	}
}

func TestAPIAccountPaymentSettingsMigrationKeepsOneEnabledMethod(t *testing.T) {
	t.Parallel()

	upSQL := readMigrationForTest(t, "000067_api_account_payment_settings.up.sql")
	for _, required := range []string{
		"CREATE TABLE api_payment_account_options",
		"PRIMARY KEY (user_id, payment_method)",
		"ux_api_payment_account_options_one_enabled",
		"WHERE enabled = true",
		"ux_api_service_payment_options_one_enabled",
		"row_number() OVER",
		"ranked_enabled.enabled_rank > 1",
		"ranked_owner_options",
		"option_row.payment_qr_code_data_url IS NOT NULL",
		"trim(option_row.payment_qr_code_data_url) <> ''",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("account payment settings migration missing %q", required)
		}
	}

	downSQL := readMigrationForTest(t, "000067_api_account_payment_settings.down.sql")
	for _, required := range []string{
		"DROP TABLE IF EXISTS api_payment_account_options",
		"DROP INDEX IF EXISTS ux_api_service_payment_options_one_enabled",
	} {
		if !strings.Contains(downSQL, required) {
			t.Fatalf("account payment settings rollback missing %q", required)
		}
	}
}

func TestAPIServiceMultiplierReconciliationIsAdditive(t *testing.T) {
	t.Parallel()

	upSQL := readMigrationForTest(t, "000066_api_service_multiplier_reconciliation.up.sql")
	for _, required := range []string{
		"pg_get_constraintdef(oid)",
		"conname = 'ck_api_service_models_sub2api_multiplier'",
		"merchant_multiplier[[:space:]]*=",
		"([^0-9.]|$)",
		"~*",
		"ALTER TABLE api_service_models DROP CONSTRAINT",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("multiplier reconciliation migration missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"UPDATE api_service_models",
		"DELETE FROM api_service_models",
		"ILIKE '%merchant_multiplier = 1%'",
	} {
		if strings.Contains(upSQL, forbidden) {
			t.Fatalf("multiplier reconciliation must not rewrite business rows with %q", forbidden)
		}
	}

	downSQL := readMigrationForTest(t, "000066_api_service_multiplier_reconciliation.down.sql")
	for _, required := range []string{
		"ADD CONSTRAINT ck_api_service_models_sub2api_multiplier",
		"CHECK (distribution_system <> 'sub2api' OR merchant_multiplier = 1.0000)",
	} {
		if !strings.Contains(downSQL, required) {
			t.Fatalf("multiplier reconciliation rollback missing %q", required)
		}
	}
}

func TestAPIQuotaSystemSlotMigrationIsAdditive(t *testing.T) {
	t.Parallel()

	sql := readMigrationForTest(t, "000056_api_quota_system_slots.up.sql")
	for _, required := range []string{
		"ADD COLUMN system_slot_key text",
		"ck_api_quota_sale_rounds_system_slot_key",
		"ix_api_quota_sale_rounds_system_slot",
		"WHERE system_slot_key IS NOT NULL",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("system slot migration missing %q", required)
		}
	}
}

func TestAuthSessionRenewalMigrationDefinesExpiryBoundaries(t *testing.T) {
	t.Parallel()

	sql := readMigrationForTest(t, "000053_auth_session_renewal.up.sql")
	for _, required := range []string{
		"ADD COLUMN renewed_at timestamptz",
		"ADD COLUMN absolute_expires_at timestamptz",
		"ADD COLUMN updated_at timestamptz",
		"expires_at = LEAST(expires_at, created_at + interval '30 days')",
		"absolute_expires_at = created_at + interval '30 days'",
		"CHECK (",
		"expires_at <= absolute_expires_at",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("auth session renewal migration missing %q", required)
		}
	}
}

func readMigrationForTest(t *testing.T, name string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
	if err != nil {
		t.Fatalf("read migration %s: %v", name, err)
	}
	return string(data)
}
