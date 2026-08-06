package database

import (
	"strings"
	"testing"
)

func TestAPIPromptAuditMigrationPreservesHistoricalPublishFacts(t *testing.T) {
	t.Parallel()

	upSQL := readMigrationForTest(t, "000080_api_prompt_audit_and_publish_contract.up.sql")
	for _, required := range []string{
		"ADD COLUMN prompt_audit_enabled boolean",
		"ADD COLUMN prompt_audit_enabled_snapshot boolean",
		"DROP CONSTRAINT ck_api_services_performance_declaration",
		"DROP CONSTRAINT ck_api_orders_kind_shape",
		"ADD CONSTRAINT ck_api_orders_kind_shape",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("prompt audit migration missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"UPDATE api_services",
		"UPDATE api_purchase_intents",
		"UPDATE api_orders",
		"DROP COLUMN declared_ttft_band",
		"DROP COLUMN performance_confirmed_at",
	} {
		if strings.Contains(upSQL, forbidden) {
			t.Fatalf("prompt audit migration must preserve historical data; found %q", forbidden)
		}
	}

	downSQL := readMigrationForTest(t, "000080_api_prompt_audit_and_publish_contract.down.sql")
	for _, required := range []string{
		"cannot roll back migration 80 while prompt-audit declarations or snapshots exist",
		"cannot roll back migration 80 while independent performance declarations exist",
		"cannot roll back migration 80 while limited quota orders lack historical performance confirmation",
		"ADD CONSTRAINT ck_api_services_performance_declaration",
	} {
		if !strings.Contains(downSQL, required) {
			t.Fatalf("prompt audit rollback missing %q", required)
		}
	}
}
