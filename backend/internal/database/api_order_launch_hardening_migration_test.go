package database

import (
	"os"
	"strings"
	"testing"
)

func TestAPIOrderLaunchHardeningMigrationContract(t *testing.T) {
	up, err := os.ReadFile("../../migrations/000094_api_order_launch_hardening.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile("../../migrations/000094_api_order_launch_hardening.down.sql")
	if err != nil {
		t.Fatal(err)
	}

	upSQL := string(up)
	for _, required := range []string{
		"ADD COLUMN merchant_confirm_due_at timestamptz",
		"ADD COLUMN delivery_due_at timestamptz",
		"ADD COLUMN late_payment_status text",
		"late_payment_status IN ('not_received', 'received_refund_pending')",
		"WHERE status = 'pending_payment'",
		"ADD COLUMN fulfillment_confirmed_at timestamptz",
		"starts_at - interval '30 minutes'",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("migration missing %q", required)
		}
	}

	downSQL := string(down)
	for _, required := range []string{
		"DROP COLUMN IF EXISTS fulfillment_confirmed_at",
		"DROP COLUMN IF EXISTS late_payment_status",
		"DROP COLUMN IF EXISTS delivery_due_at",
		"DROP COLUMN IF EXISTS merchant_confirm_due_at",
	} {
		if !strings.Contains(downSQL, required) {
			t.Fatalf("down migration missing %q", required)
		}
	}
}
