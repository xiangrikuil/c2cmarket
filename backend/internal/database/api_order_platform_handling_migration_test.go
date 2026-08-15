package database

import (
	"os"
	"strings"
	"testing"
)

func TestAPIOrderPlatformHandlingMigrationContract(t *testing.T) {
	up, err := os.ReadFile("../../migrations/000107_api_order_platform_handling_simplification.up.sql")
	if err != nil {
		t.Fatalf("read migration 107 up: %v", err)
	}
	down, err := os.ReadFile("../../migrations/000107_api_order_platform_handling_simplification.down.sql")
	if err != nil {
		t.Fatalf("read migration 107 down: %v", err)
	}
	upSQL := string(up)
	downSQL := string(down)
	for _, required := range []string{
		"next_actor text NOT NULL DEFAULT 'none'",
		"fact_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb",
		"respondent_response text NOT NULL DEFAULT ''",
		"status IN ('open', 'waiting_info', 'resolved', 'closed', 'withdrawn', 'self_resolved')",
		"superseded_reason = 'platform_flow_simplified'",
		"'formal_response'",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("migration 107 up missing %q", required)
		}
	}
	for _, required := range []string{
		"cannot roll back migration 107 after direct platform-handling data exists",
		"applicant_statement <> ''",
		"fact_snapshot <> '{}'::jsonb",
		"usage = 'formal_response'",
		"SET status = 'pending'",
		"SET status = 'negotiating'",
	} {
		if !strings.Contains(downSQL, required) {
			t.Fatalf("migration 107 down missing %q", required)
		}
	}
}
