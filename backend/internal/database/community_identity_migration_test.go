package database

import (
	"strings"
	"testing"
)

func TestCommunityIdentityMigrationIsIndependentAndSoftRevocable(t *testing.T) {
	t.Parallel()

	upSQL := readMigrationForTest(t, "000113_community_identity.up.sql")
	for _, required := range []string{
		"CREATE TABLE user_community_identities",
		"identity_type text NOT NULL CHECK (identity_type IN ('FOUNDING_USER', 'BETA_CONTRIBUTOR'))",
		"source text NOT NULL CHECK (source IN ('AUTO', 'ADMIN', 'BACKFILL'))",
		"UNIQUE (user_id, identity_type)",
		"revoked_at timestamptz",
		"CHECK ((source = 'ADMIN' AND granted_by IS NOT NULL",
		"CREATE INDEX ix_user_community_identities_user_active",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("community identity migration missing %q", required)
		}
	}

	for _, forbidden := range []string{
		"profile.badges",
		"user_reputation",
	} {
		if strings.Contains(upSQL, forbidden) {
			t.Fatalf("community identity migration must remain independent from %q", forbidden)
		}
	}

	downSQL := readMigrationForTest(t, "000113_community_identity.down.sql")
	if !strings.Contains(downSQL, "DROP TABLE IF EXISTS user_community_identities") {
		t.Fatal("community identity rollback must remove only its own table")
	}
}
