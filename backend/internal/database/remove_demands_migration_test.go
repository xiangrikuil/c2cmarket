package database

import (
	"strings"
	"testing"
)

func TestRemoveDemandsMigrationContract(t *testing.T) {
	t.Parallel()

	upSQL := readMigrationForTest(t, "000065_remove_demands.up.sql")
	for _, required := range []string{
		"DELETE FROM idempotency_keys",
		"resource_type = 'demand'",
		"DROP TABLE IF EXISTS demands",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("remove demands migration missing %q", required)
		}
	}

	downSQL := readMigrationForTest(t, "000065_remove_demands.down.sql")
	for _, required := range []string{
		"CREATE TABLE demands",
		"CREATE INDEX ix_demands_public_active",
		"CREATE INDEX ix_demands_publisher_updated",
		"CREATE INDEX ix_demands_admin_status_updated",
		"CREATE INDEX ix_demands_search_trgm",
	} {
		if !strings.Contains(downSQL, required) {
			t.Fatalf("remove demands rollback missing %q", required)
		}
	}
	if strings.Contains(strings.ToUpper(downSQL), "INSERT INTO DEMANDS") {
		t.Fatal("remove demands rollback must not claim to restore deleted rows")
	}
}
