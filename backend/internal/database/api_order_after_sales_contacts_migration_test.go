package database

import (
	"os"
	"strings"
	"testing"
)

func TestAPIOrderAfterSalesContactsMigrationContract(t *testing.T) {
	up, err := os.ReadFile("../../migrations/000089_api_order_after_sales_contacts.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile("../../migrations/000089_api_order_after_sales_contacts.down.sql")
	if err != nil {
		t.Fatal(err)
	}

	upSQL := string(up)
	for _, required := range []string{
		"CREATE TABLE api_service_contact_methods",
		"UNIQUE (api_service_id, sort_order)",
		"REFERENCES api_services(id, owner_user_id) ON DELETE CASCADE",
		"REFERENCES contact_methods(id, user_id) ON DELETE RESTRICT",
		"INSERT INTO api_service_contact_methods",
		"CREATE TABLE api_purchase_intent_owner_contact_snapshots",
		"UNIQUE (api_purchase_intent_id, sort_order)",
		"REFERENCES contact_method_versions(id, contact_method_id, owner_user_id) ON DELETE RESTRICT",
		"INSERT INTO api_purchase_intent_owner_contact_snapshots",
		"ADD COLUMN issue_occurred_at timestamptz",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("migration missing %q", required)
		}
	}

	downSQL := string(down)
	for _, required := range []string{
		"DROP COLUMN IF EXISTS issue_occurred_at",
		"DROP TABLE IF EXISTS api_purchase_intent_owner_contact_snapshots",
		"DROP TABLE IF EXISTS api_service_contact_methods",
	} {
		if !strings.Contains(downSQL, required) {
			t.Fatalf("down migration missing %q", required)
		}
	}
}
