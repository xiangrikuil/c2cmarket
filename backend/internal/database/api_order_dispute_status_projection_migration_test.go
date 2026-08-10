package database

import (
	"strings"
	"testing"
)

func TestAPIOrderDisputeStatusProjectionMigrationExtendsConstraintsWithoutRewritingRows(t *testing.T) {
	upSQL := readMigrationForTest(t, "000084_api_order_dispute_status_projection.up.sql")
	for _, required := range []string{
		"ck_dispute_cases_status",
		"'negotiating', 'open', 'waiting_info', 'resolved', 'closed'",
		"ck_api_orders_dispute_status",
		"'awaiting_fulfillment'",
		"'fulfillment_confirmation'",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("API order dispute status migration missing %q", required)
		}
	}
	if strings.Contains(strings.ToUpper(upSQL), "UPDATE ") {
		t.Fatal("status projection migration must not rewrite existing business rows")
	}

	downSQL := readMigrationForTest(t, "000084_api_order_dispute_status_projection.down.sql")
	for _, required := range []string{
		"CHECK (dispute_status IN ('none', 'open', 'closed'))",
		"CHECK (status IN ('open', 'waiting_info', 'resolved', 'closed'))",
	} {
		if !strings.Contains(downSQL, required) {
			t.Fatalf("API order dispute status rollback missing %q", required)
		}
	}
	if strings.Contains(strings.ToUpper(downSQL), "UPDATE ") {
		t.Fatal("status projection rollback must not rewrite existing business rows")
	}
}
