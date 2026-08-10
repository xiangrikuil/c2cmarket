package database

import (
	"os"
	"strings"
	"testing"
)

func TestAPIOrderDisputeSanctionsMigrationDefinesAuditableSourceAndWindowIndex(t *testing.T) {
	t.Parallel()

	up, err := os.ReadFile("../../migrations/000088_api_order_dispute_sanctions.up.sql")
	if err != nil {
		t.Fatalf("read up migration: %v", err)
	}
	down, err := os.ReadFile("../../migrations/000088_api_order_dispute_sanctions.down.sql")
	if err != nil {
		t.Fatalf("read down migration: %v", err)
	}

	upSQL := string(up)
	for _, required := range []string{
		"source_dispute_remedy_id uuid",
		"REFERENCES api_order_dispute_remedies(id) ON DELETE RESTRICT",
		"CREATE UNIQUE INDEX ux_user_restrictions_source_dispute_remedy",
		"WHERE source_dispute_remedy_id IS NOT NULL",
		"CREATE INDEX ix_api_order_dispute_remedies_responsible_overdue",
		"responsible_user_id, overdue_at DESC, id DESC",
		"WHERE status = 'overdue'",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("up migration missing %q", required)
		}
	}
	if strings.Contains(upSQL, "ON DELETE SET NULL") {
		t.Fatal("sanction remedy evidence must not be cleared by upstream deletion")
	}
	if strings.Contains(strings.ToUpper(upSQL), "UPDATE USER_RESTRICTIONS") || strings.Contains(strings.ToUpper(upSQL), "DELETE FROM USER_RESTRICTIONS") {
		t.Fatal("sanctions migration must not rewrite existing restrictions")
	}

	downSQL := string(down)
	for _, required := range []string{
		"DROP INDEX IF EXISTS ix_api_order_dispute_remedies_responsible_overdue",
		"DROP INDEX IF EXISTS ux_user_restrictions_source_dispute_remedy",
		"DROP COLUMN IF EXISTS source_dispute_remedy_id",
	} {
		if !strings.Contains(downSQL, required) {
			t.Fatalf("down migration missing %q", required)
		}
	}
}
