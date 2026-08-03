package postgres

import (
	"strings"
	"testing"

	"c2c-market/backend/internal/module/reputation"
)

func TestAggregateReputationFactsSQLUsesTruthfulTerminalStatesAndBatchInput(t *testing.T) {
	t.Parallel()

	for _, required := range []string{
		"SELECT DISTINCT unnest($1::uuid[]) AS user_id",
		"CROSS JOIN (VALUES ('buyer'), ('seller'))",
		"CROSS JOIN (VALUES ('overall'), ('carpool'), ('api'))",
		"membership.status = 'completed'",
		"api_order.status = 'completed'",
		"api_order.completed_at >= $2",
		"application.status = 'cancelled_by_buyer'",
		"application.status = 'cancelled_by_owner'",
		"membership.ended_by_user_id = membership.buyer_user_id",
		"membership.ended_by_user_id = membership.owner_user_id",
		"event.event_type = 'api_order.cancelled'",
		"dispute.status IN ('open', 'waiting_info')",
		"ON dispute.subject_user_id = requested.user_id",
		"reputation_transaction_exclusions",
		"exclusion.restored_at IS NOT NULL",
	} {
		if !strings.Contains(aggregateReputationFactsSQL, required) {
			t.Fatalf("aggregate reputation SQL missing %q", required)
		}
	}
	for _, forbidden := range []string{
		"api_purchase_intents.status = 'completed'",
		"carpool_listings.status = 'completed'",
		"dispute.primary_user_id = requested.user_id",
		"dispute.counterparty_user_id = requested.user_id",
	} {
		if strings.Contains(aggregateReputationFactsSQL, forbidden) {
			t.Fatalf("aggregate reputation SQL contains false completion source %q", forbidden)
		}
	}
}

func TestAggregateReputationFactsSQLAttributesControllableTimeoutsToResponsibleRole(t *testing.T) {
	t.Parallel()

	for _, required := range []string{
		"api_order.cancel_reason = 'payment_timeout'",
		"FROM carpool_join_confirmations confirmation",
		"confirmation.actor_role = 'buyer'",
		"confirmation.actor_role = 'owner'",
		"participants.role = 'buyer' AND NOT confirmations.buyer_confirmed",
		"participants.role = 'seller' AND NOT confirmations.owner_confirmed",
	} {
		if !strings.Contains(aggregateReputationFactsSQL, required) {
			t.Fatalf("aggregate reputation SQL missing responsibility evidence %q", required)
		}
	}
}

func TestScopeFactsKeepsRoleAndScopeIndependent(t *testing.T) {
	t.Parallel()

	var value reputation.RawFacts
	target := scopeFacts(&value, reputation.RoleBuyer, reputation.ScopeCarpool)
	if target == nil {
		t.Fatal("expected buyer carpool scope")
	}
	target.CompletedCount = 3
	if value.Buyer.Carpool.CompletedCount != 3 {
		t.Fatalf("buyer carpool facts were not assigned: %#v", value)
	}
	if value.Buyer.API.CompletedCount != 0 || value.Seller.Carpool.CompletedCount != 0 {
		t.Fatalf("role or scope facts leaked: %#v", value)
	}
	if scopeFacts(&value, "unknown", reputation.ScopeAPI) != nil {
		t.Fatal("unknown role must not map to facts")
	}
}
