package database

import (
	"strings"
	"testing"
)

func TestAPIOrderPublicNumberMigrationContract(t *testing.T) {
	t.Parallel()

	upSQL := readMigrationForTest(t, "000075_api_order_public_numbers.up.sql")
	for _, required := range []string{
		"ADD COLUMN order_no text",
		"created_at AT TIME ZONE 'Asia/Shanghai'",
		"ABCDEFGHJKMNPQRSTUVWXYZ23456789",
		"ALTER COLUMN order_no SET NOT NULL",
		"ADD CONSTRAINT ck_api_orders_order_no_format",
		"ADD CONSTRAINT ux_api_orders_order_no UNIQUE (order_no)",
		"CREATE TRIGGER trg_api_orders_order_no_immutable",
		"NEW.order_no IS DISTINCT FROM OLD.order_no",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("API order number migration missing %q", required)
		}
	}

	downSQL := readMigrationForTest(t, "000075_api_order_public_numbers.down.sql")
	for _, required := range []string{
		"DROP TRIGGER IF EXISTS trg_api_orders_order_no_immutable ON api_orders",
		"DROP FUNCTION IF EXISTS preserve_api_order_no()",
		"DROP COLUMN IF EXISTS order_no",
	} {
		if !strings.Contains(downSQL, required) {
			t.Fatalf("API order number rollback missing %q", required)
		}
	}
}
