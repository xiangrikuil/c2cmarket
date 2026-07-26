package database

import (
	"strings"
	"testing"
)

func TestAuthIdentityBootstrapHardeningMigrationContract(t *testing.T) {
	t.Parallel()

	upSQL := readMigrationForTest(t, "000062_auth_identity_bootstrap_hardening.up.sql")
	for _, required := range []string{
		"CREATE TABLE admin_bootstrap_runs",
		"bootstrap_key text PRIMARY KEY",
		"user_id uuid NOT NULL UNIQUE REFERENCES users(id)",
		"username_snapshot text NOT NULL UNIQUE",
		"created_at timestamptz NOT NULL",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("auth identity hardening migration missing %q", required)
		}
	}

	downSQL := readMigrationForTest(t, "000062_auth_identity_bootstrap_hardening.down.sql")
	if !strings.Contains(downSQL, "DROP TABLE IF EXISTS admin_bootstrap_runs") {
		t.Fatal("auth identity hardening rollback must remove admin_bootstrap_runs")
	}
}
