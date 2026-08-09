package database

import (
	"strings"
	"testing"
)

func TestAPIOrderDisputeRemediesMigrationDefinesAuditableLifecycle(t *testing.T) {
	upSQL := readMigrationForTest(t, "000087_api_order_dispute_remedies.up.sql")
	for _, required := range []string{
		"CREATE TABLE api_order_dispute_remedies",
		"'claimed_fulfilled'",
		"'confirmation_expired'",
		"responsible_user_id <> beneficiary_user_id",
		"due_at > created_at",
		"status = 'overdue'\n      AND claimed_at IS NULL AND confirmation_due_at IS NULL",
		"ux_api_order_dispute_remedies_active",
		"ix_api_order_dispute_remedies_confirmation_due",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("API order dispute remedies migration missing %q", required)
		}
	}
	if strings.Contains(strings.ToUpper(upSQL), "UPDATE DISPUTE_CASES") || strings.Contains(strings.ToUpper(upSQL), "UPDATE API_ORDERS") {
		t.Fatal("remedies migration must not rewrite existing disputes or orders")
	}

	downSQL := readMigrationForTest(t, "000087_api_order_dispute_remedies.down.sql")
	if !strings.Contains(downSQL, "DROP TABLE IF EXISTS api_order_dispute_remedies") {
		t.Fatal("remedies rollback must remove its table")
	}
}
