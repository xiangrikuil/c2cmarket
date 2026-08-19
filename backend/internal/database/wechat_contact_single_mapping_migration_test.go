package database

import (
	"os"
	"strings"
	"testing"
)

func TestWechatContactSingleMappingMigrationContract(t *testing.T) {
	t.Parallel()

	up, err := os.ReadFile("../../migrations/000115_wechat_contact_single_mapping.up.sql")
	if err != nil {
		t.Fatalf("read up migration: %v", err)
	}
	down, err := os.ReadFile("../../migrations/000115_wechat_contact_single_mapping.down.sql")
	if err != nil {
		t.Fatalf("read down migration: %v", err)
	}

	upSQL := string(up)
	for _, required := range []string{
		"UPDATE contact_methods",
		"ARRAY['carpool_owner', 'api_merchant', 'buyer', 'dispute']::text[]",
		"WHERE type = 'wechat'",
		"CREATE UNIQUE INDEX ux_contact_methods_one_enabled_wechat",
		"ON contact_methods(user_id)",
		"WHERE type = 'wechat' AND enabled = true",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("up migration missing %q", required)
		}
	}
	for _, historicalTable := range []string{
		"contact_method_versions",
		"api_purchase_intents",
		"api_purchase_intent_owner_contact_snapshots",
		"carpool_contact_snapshots",
	} {
		if strings.Contains(upSQL, "UPDATE "+historicalTable) || strings.Contains(upSQL, "DELETE FROM "+historicalTable) {
			t.Fatalf("migration must preserve historical contact evidence in %s", historicalTable)
		}
	}
	if strings.TrimSpace(string(down)) != "DROP INDEX IF EXISTS ux_contact_methods_one_enabled_wechat;" {
		t.Fatal("down migration must remove only the enabled WeChat uniqueness index")
	}
}
