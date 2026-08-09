package database

import (
	"strings"
	"testing"
)

func TestAPIOrderDisputeNegotiationMigrationDefinesImmutableMessagesAndBilateralProposals(t *testing.T) {
	upSQL := readMigrationForTest(t, "000085_api_order_dispute_negotiation.up.sql")
	for _, required := range []string{
		"issue_code text NOT NULL",
		"requested_resolution text NOT NULL",
		"CREATE TABLE api_order_dispute_messages",
		"trg_api_order_dispute_messages_append_only",
		"CREATE TABLE api_order_dispute_settlement_proposals",
		"ux_api_order_dispute_proposals_pending",
		"accepted_by_user_id IS NULL OR accepted_by_user_id <> proposed_by_user_id",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("API order dispute negotiation migration missing %q", required)
		}
	}
	if strings.Contains(strings.ToUpper(upSQL), "UPDATE DISPUTE_CASES") {
		t.Fatal("negotiation migration must not rewrite existing dispute cases")
	}

	downSQL := readMigrationForTest(t, "000085_api_order_dispute_negotiation.down.sql")
	for _, required := range []string{
		"DROP TABLE IF EXISTS api_order_dispute_settlement_proposals",
		"DROP TABLE IF EXISTS api_order_dispute_messages",
		"DROP COLUMN IF EXISTS requested_amount_cny",
	} {
		if !strings.Contains(downSQL, required) {
			t.Fatalf("API order dispute negotiation rollback missing %q", required)
		}
	}
}
