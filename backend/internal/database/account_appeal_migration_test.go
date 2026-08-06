package database

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAccountAppealMigrationDefinesDedicatedSessionAndAppealBoundary(t *testing.T) {
	up := readAccountAppealMigration(t, "000078_account_appeal_sessions.up.sql")
	for _, required := range []string{
		"CREATE TABLE account_appeal_sessions",
		"session_token_hash text NOT NULL UNIQUE",
		"csrf_token_hash text NOT NULL",
		"expires_at = created_at + interval '15 minutes'",
		"ix_account_appeal_sessions_lifecycle",
		"target_type = 'account_governance'",
		"report_id IS NULL",
		"dispute_case_id IS NULL",
		"target_id = appellant_user_id::text",
		"ux_appeals_submitted_account_governance",
		"WHERE target_type = 'account_governance' AND status = 'submitted'",
	} {
		if !strings.Contains(up, required) {
			t.Fatalf("account appeal migration is missing %q", required)
		}
	}
	for _, forbidden := range []string{"access_token", "refresh_token", "raw_profile", "userinfo_json"} {
		if strings.Contains(strings.ToLower(up), forbidden) {
			t.Fatalf("account appeal migration contains forbidden provider credential field %q", forbidden)
		}
	}

	down := readAccountAppealMigration(t, "000078_account_appeal_sessions.down.sql")
	for _, required := range []string{
		"WHERE target_type = 'account_governance'",
		"cannot roll back account appeal sessions after account-governance appeals exist",
		"USING ERRCODE = '55000'",
		"DROP TABLE IF EXISTS account_appeal_sessions",
		"CHECK (report_id IS NOT NULL OR dispute_case_id IS NOT NULL)",
	} {
		if !strings.Contains(down, required) {
			t.Fatalf("account appeal rollback is missing %q", required)
		}
	}
}

func readAccountAppealMigration(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
	if err != nil {
		t.Fatalf("read migration %s: %v", name, err)
	}
	return string(data)
}
