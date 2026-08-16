package database

import (
	"os"
	"strings"
	"testing"
)

func TestAPIOrderRefundStyleDisputeMigrationContract(t *testing.T) {
	up, err := os.ReadFile("../../migrations/000108_api_order_refund_style_disputes.up.sql")
	if err != nil {
		t.Fatalf("read migration 108 up: %v", err)
	}
	down, err := os.ReadFile("../../migrations/000108_api_order_refund_style_disputes.down.sql")
	if err != nil {
		t.Fatalf("read migration 108 down: %v", err)
	}

	upSQL := string(up)
	for _, required := range []string{
		"pending_seller_response",
		"pending_applicant_decision",
		"voluntary_fulfillment",
		"seller_decision_reason",
		"seller_response_late",
		"applicant_decision_due_at",
		"seller_acceptance",
		"opened_at + interval '24 hours'",
		"ix_dispute_cases_applicant_decision_due",
		"ck_dispute_cases_platform_escalation_shape",
		"voluntary_fulfillment_confirmed",
		"voluntary_confirmation_no_objection",
		"applicant_decision_expired",
		"order_row.dispute_status IN ('open', 'negotiating')",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("migration 108 up missing %q", required)
		}
	}
	if !strings.Contains(string(down), "cannot roll back migration 108 after seller-first after-sales data exists") {
		t.Fatal("migration 108 down must reject rollback after new seller-first facts exist")
	}
}
