package database

import (
	"os"
	"strings"
	"testing"
)

func TestAPIOrderSellerDeliveryCompletionMigrationContract(t *testing.T) {
	up, err := os.ReadFile("../../migrations/000118_api_order_seller_delivery_completion.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := os.ReadFile("../../migrations/000118_api_order_seller_delivery_completion.down.sql")
	if err != nil {
		t.Fatal(err)
	}

	upSQL := string(up)
	for _, fragment := range []string{
		"WHERE status = 'delivery_submitted'",
		"status = 'completed'",
		"completion_source = 'seller_delivered'",
		"commercial_outcome = 'normal_fulfillment'",
		"'seller_delivered', 'remedy_confirmed'",
		"DROP INDEX IF EXISTS ix_api_orders_delivery_review_expiry",
	} {
		if !strings.Contains(upSQL, fragment) {
			t.Fatalf("up migration missing %q", fragment)
		}
	}

	downSQL := string(down)
	for _, fragment := range []string{
		"status = 'delivery_submitted'",
		"completion_source = NULL",
		"completion_source = 'buyer_confirmed'",
		"CREATE INDEX ix_api_orders_delivery_review_expiry",
	} {
		if !strings.Contains(downSQL, fragment) {
			t.Fatalf("down migration missing %q", fragment)
		}
	}
}
