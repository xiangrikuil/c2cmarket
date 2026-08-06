package database

import (
	"strings"
	"testing"
)

func TestAPIOrderDeliveryReviewMigrationContract(t *testing.T) {
	upSQL := readMigrationForTest(t, "000068_api_order_delivery_review.up.sql")
	for _, fragment := range []string{
		"delivery_review_expires_at timestamptz",
		"delivery_review_reminded_at timestamptz",
		"completion_source text",
		"now() + interval '24 hours'",
		"completion_source = 'buyer_confirmed'",
		"completion_source IN ('buyer_confirmed', 'auto_completed')",
		"ix_api_orders_delivery_review_expiry",
		"WHERE status = 'delivery_submitted'",
	} {
		if !strings.Contains(upSQL, fragment) {
			t.Fatalf("delivery review migration is missing %q", fragment)
		}
	}

	downSQL := readMigrationForTest(t, "000068_api_order_delivery_review.down.sql")
	for _, fragment := range []string{
		"DROP INDEX IF EXISTS ix_api_orders_delivery_review_expiry",
		"DROP COLUMN delivery_review_expires_at",
		"DROP COLUMN delivery_review_reminded_at",
		"DROP COLUMN completion_source",
		"ADD CONSTRAINT ck_api_orders_state_shape",
	} {
		if !strings.Contains(downSQL, fragment) {
			t.Fatalf("delivery review down migration is missing %q", fragment)
		}
	}
}
