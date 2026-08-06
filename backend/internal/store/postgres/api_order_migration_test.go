package postgres

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAPIOrderIntentOrderedConstraintCleanupMigration(t *testing.T) {
	path := filepath.Join("..", "..", "..", "migrations", "000052_api_purchase_intent_ordered_constraint_cleanup.up.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(data)
	for _, required := range []string{
		"DROP CONSTRAINT IF EXISTS api_purchase_intents_check3",
		"DROP CONSTRAINT IF EXISTS ck_api_intent_status_timestamps",
		"ADD CONSTRAINT ck_api_intent_status_timestamps",
		"status = 'ordered'",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("ordered-intent cleanup migration missing %q", required)
		}
	}

	downPath := filepath.Join("..", "..", "..", "migrations", "000052_api_purchase_intent_ordered_constraint_cleanup.down.sql")
	downData, err := os.ReadFile(downPath)
	if err != nil {
		t.Fatalf("read down migration: %v", err)
	}
	if strings.Contains(string(downData), "ADD CONSTRAINT") {
		t.Fatal("down migration must not restore the legacy constraint that rejects ordered intents")
	}
}
