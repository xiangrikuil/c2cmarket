package database

import (
	"os"
	"strings"
	"testing"
)

func TestOptionalTransactionContactsMigrationContract(t *testing.T) {
	t.Parallel()

	up, err := os.ReadFile("../../migrations/000117_optional_transaction_contacts.up.sql")
	if err != nil {
		t.Fatalf("read up migration: %v", err)
	}
	down, err := os.ReadFile("../../migrations/000117_optional_transaction_contacts.down.sql")
	if err != nil {
		t.Fatalf("read down migration: %v", err)
	}

	upSQL := string(up)
	for _, required := range []string{
		"DROP CONSTRAINT IF EXISTS ck_contact_methods_wechat_all_usage_scopes",
		"DROP CONSTRAINT IF EXISTS ck_contact_methods_usage_scopes",
		"DROP COLUMN IF EXISTS usage_scopes",
		"DROP FUNCTION IF EXISTS canonical_contact_usage_scopes(text[])",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("up migration missing %q", required)
		}
	}
	for _, preserved := range []string{
		"contact_method_versions",
		"contact_sessions",
		"contact_session_items",
		"ux_contact_methods_one_enabled_wechat",
	} {
		if strings.Contains(upSQL, "DROP TABLE "+preserved) || strings.Contains(upSQL, "DROP INDEX "+preserved) {
			t.Fatalf("up migration must preserve %s", preserved)
		}
	}

	downSQL := string(down)
	for _, required := range []string{
		"CREATE FUNCTION canonical_contact_usage_scopes",
		"ADD COLUMN usage_scopes text[]",
		"ALTER COLUMN usage_scopes SET NOT NULL",
		"ADD CONSTRAINT ck_contact_methods_usage_scopes",
		"ADD CONSTRAINT ck_contact_methods_wechat_all_usage_scopes",
	} {
		if !strings.Contains(downSQL, required) {
			t.Fatalf("down migration missing %q", required)
		}
	}
}
