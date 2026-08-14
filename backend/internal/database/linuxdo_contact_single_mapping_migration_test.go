package database

import (
	"os"
	"strings"
	"testing"
)

func TestLinuxDoContactSingleMappingMigrationContract(t *testing.T) {
	t.Parallel()

	up, err := os.ReadFile("../../migrations/000090_linuxdo_contact_single_mapping.up.sql")
	if err != nil {
		t.Fatalf("read up migration: %v", err)
	}
	down, err := os.ReadFile("../../migrations/000090_linuxdo_contact_single_mapping.down.sql")
	if err != nil {
		t.Fatalf("read down migration: %v", err)
	}

	upSQL := string(up)
	for _, required := range []string{
		"ORDER BY is_default DESC, created_at ASC, id ASC",
		"UPDATE api_services service",
		"DELETE FROM api_service_contact_methods selected",
		"INSERT INTO api_service_contact_methods",
		"UPDATE carpool_listings listing",
		"UPDATE carpool_applications application",
		"SET enabled = false",
		"CREATE UNIQUE INDEX ux_contact_methods_one_enabled_linuxdo",
		"WHERE type = 'linuxdo' AND enabled = true",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("up migration missing %q", required)
		}
	}
	for _, historicalTable := range []string{
		"api_purchase_intents",
		"api_purchase_intent_owner_contact_snapshots",
		"contact_method_versions",
	} {
		if strings.Contains(upSQL, "UPDATE "+historicalTable) || strings.Contains(upSQL, "DELETE FROM "+historicalTable) {
			t.Fatalf("migration must preserve historical contact evidence in %s", historicalTable)
		}
	}
	if !strings.Contains(string(down), "DROP INDEX IF EXISTS ux_contact_methods_one_enabled_linuxdo") {
		t.Fatal("down migration must remove only the enabled linux.do uniqueness index")
	}
}
