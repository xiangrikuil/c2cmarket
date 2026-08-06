package database

import (
	"strings"
	"testing"
)

func TestAPIPromotionMigrationContract(t *testing.T) {
	t.Parallel()

	upSQL := readMigrationForTest(t, "000071_api_service_promotions.up.sql")
	for _, required := range []string{
		"CREATE TABLE api_service_promotions",
		"api_service_id uuid NOT NULL REFERENCES api_services(id)",
		"placement text NOT NULL CHECK (placement IN ('api_market_top'))",
		"ends_at > starts_at",
		"created_reason text NOT NULL CHECK (trim(created_reason) <> '' AND char_length(created_reason) <= 500)",
		"stopped_by_admin_id IS NULL AND stopped_reason = ''",
		"stopped_by_admin_id IS NOT NULL AND trim(stopped_reason) <> '' AND char_length(stopped_reason) <= 500",
		"version bigint NOT NULL DEFAULT 1 CHECK (version > 0)",
		"ix_api_service_promotions_placement_period",
		"ix_api_service_promotions_service_period",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("API promotion migration missing %q", required)
		}
	}

	downSQL := readMigrationForTest(t, "000071_api_service_promotions.down.sql")
	if !strings.Contains(downSQL, "DROP TABLE IF EXISTS api_service_promotions") {
		t.Fatal("API promotion rollback must remove only the promotion table")
	}
}

func TestAPIServiceCommercialFactsMigrationContract(t *testing.T) {
	t.Parallel()

	upSQL := readMigrationForTest(t, "000072_api_service_commercial_facts.up.sql")
	for _, required := range []string{
		"RENAME COLUMN recommended_concurrency TO declared_max_concurrency",
		"RENAME COLUMN quota_recommended_concurrency_snapshot",
		"TO quota_declared_max_concurrency_snapshot",
		"ADD COLUMN account_pool_type text",
		"ADD COLUMN account_pool_custom_name text",
		"ADD COLUMN merchant_refund_commitment boolean NOT NULL DEFAULT false",
		"account_pool_type IN ('gpt_pro_20x', 'gpt_pro_5x', 'gpt_plus', 'custom')",
		"char_length(trim(account_pool_custom_name)) BETWEEN 2 AND 40",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("API service commercial facts migration missing %q", required)
		}
	}

	downSQL := readMigrationForTest(t, "000072_api_service_commercial_facts.down.sql")
	for _, required := range []string{
		"DROP COLUMN IF EXISTS merchant_refund_commitment",
		"TO quota_recommended_concurrency_snapshot",
		"RENAME COLUMN declared_max_concurrency TO recommended_concurrency",
	} {
		if !strings.Contains(downSQL, required) {
			t.Fatalf("API service commercial facts rollback missing %q", required)
		}
	}
}
