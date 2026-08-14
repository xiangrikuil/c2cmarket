package database

import (
	"os"
	"strings"
	"testing"
)

func TestDynamicCatalogLifecycleMigrationUsesCurrentSchema(t *testing.T) {
	up, err := os.ReadFile("../../migrations/000097_dynamic_catalog_lifecycle.up.sql")
	if err != nil {
		t.Fatalf("read migration 97 up: %v", err)
	}
	down, err := os.ReadFile("../../migrations/000097_dynamic_catalog_lifecycle.down.sql")
	if err != nil {
		t.Fatalf("read migration 97 down: %v", err)
	}

	upSQL := string(up)
	if strings.Contains(upSQL, "model_key, display_name") || strings.Contains(upSQL, "display_name = EXCLUDED.display_name") {
		t.Fatal("migration 97 must not use api_model_catalog.display_name removed by migration 81")
	}
	if strings.Contains(upSQL, "00000000-0000-0000-0000-000000000501") {
		t.Fatal("migration 97 must not reuse the historical other-custom product plan ID")
	}
	if !strings.Contains(upSQL, "00000000-0000-0000-0000-000000000601") {
		t.Fatal("migration 97 must use the dedicated Grok product plan ID")
	}
	if count := strings.Count(upSQL, "CREATE UNIQUE INDEX ux_api_order_catalog_risk_holds_active"); count != 1 {
		t.Fatalf("active risk-hold index must be declared once, got %d", count)
	}
	for _, required := range []string{
		"DROP CONSTRAINT IF EXISTS api_model_providers_provider_category_check",
		"status IN ('active', 'deprecated', 'blocked')",
		"CREATE TABLE api_order_catalog_risk_holds",
		"DROP COLUMN active",
		"GROUP BY lower(btrim(code)) HAVING count(*) > 1",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("migration 97 up missing %q", required)
		}
	}
	if !strings.Contains(string(down), "cannot roll back dynamic catalog lifecycle") {
		t.Fatal("migration 97 down must refuse lossy rollback")
	}
	if !strings.Contains(string(down), "SET provider_category = 'other' WHERE code = 'xai'") {
		t.Fatal("migration 97 down must map xAI to the historical open provider category")
	}
	for _, required := range []string{
		"ALTER TABLE product_categories DROP CONSTRAINT ck_product_categories_code",
		"ALTER TABLE api_model_providers DROP CONSTRAINT ck_api_model_providers_code",
	} {
		if !strings.Contains(string(down), required) {
			t.Fatalf("migration 97 down missing %q", required)
		}
	}
}
