package database

import (
	"os"
	"strings"
	"testing"
)

func TestContactUsageScopesMigrationContract(t *testing.T) {
	t.Parallel()

	up, err := os.ReadFile("../../migrations/000093_contact_usage_scopes.up.sql")
	if err != nil {
		t.Fatalf("read up migration: %v", err)
	}
	down, err := os.ReadFile("../../migrations/000093_contact_usage_scopes.down.sql")
	if err != nil {
		t.Fatalf("read down migration: %v", err)
	}

	upSQL := string(up)
	for _, required := range []string{
		"ADD COLUMN usage_scopes text[]",
		"UPDATE contact_methods",
		"ARRAY['buyer', 'dispute']::text[]",
		"cardinality(usage_scopes) > 0",
		"array_position(usage_scopes, NULL) IS NULL",
		"usage_scopes <@ ARRAY[",
		"usage_scopes = canonical_contact_usage_scopes(usage_scopes)",
		"SELECT DISTINCT unnest(scopes) AS scope",
		"WHEN 'carpool_owner' THEN 1",
		"WHEN 'api_merchant' THEN 2",
		"WHEN 'buyer' THEN 3",
		"WHEN 'dispute' THEN 4",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("up migration missing %q", required)
		}
	}

	downSQL := string(down)
	for _, required := range []string{
		"cannot roll back migration 93 after explicit contact usage scopes exist",
		"DROP CONSTRAINT IF EXISTS ck_contact_methods_usage_scopes",
		"DROP COLUMN IF EXISTS usage_scopes",
		"DROP FUNCTION IF EXISTS canonical_contact_usage_scopes(text[])",
	} {
		if !strings.Contains(downSQL, required) {
			t.Fatalf("down migration missing %q", required)
		}
	}
}
