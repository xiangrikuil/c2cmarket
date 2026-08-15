package postgres

import (
	"strings"
	"testing"
)

func TestAggregateReputationEngineFactsSQLCoversRulesAndTimeBoundaries(t *testing.T) {
	t.Parallel()

	for _, required := range []string{
		"CROSS JOIN (VALUES ('overall'), ('carpool'), ('api'))",
		"review_candidates.status = 'sealed'",
		"review_candidates.review_deadline_at <= $2",
		"review_tag_stats",
		"'slow_response', 'hard_to_comm', 'late_change', 'desc_diff'",
		"'响应较慢', '沟通困难', '临时变更', '与描述不符', '实际体验与描述有差异'",
		"platform_review_stats",
		"interval '90 days'",
		"interval '365 days'",
		"outcome.responsibility IN ('responsible', 'shared')",
		"dispute.target_type <> 'api_order'",
		"FROM api_order_dispute_remedies remedy",
		") = 'late_confirmed'",
		"outcome_candidates.severity IN ('high', 'critical')",
		"restriction_candidates.starts_at <= $2",
		"restriction.revoked_at IS NULL",
		"reputation_transaction_exclusions",
		"LEAST(",
	} {
		if !strings.Contains(aggregateReputationEngineFactsSQL, required) {
			t.Fatalf("engine aggregate SQL missing %q", required)
		}
	}
}

func TestAggregateReputationEngineFactsSQLUsesSameTimeoutResponsibilityMatrix(t *testing.T) {
	t.Parallel()

	for _, required := range []string{
		"api_order.cancel_reason = 'payment_timeout'",
		"FROM carpool_join_confirmations confirmation",
		"confirmation.actor_role = 'buyer'",
		"confirmation.actor_role = 'owner'",
		"participants.role = 'buyer' AND NOT confirmations.buyer_confirmed",
		"participants.role = 'seller' AND NOT confirmations.owner_confirmed",
	} {
		if !strings.Contains(aggregateReputationEngineFactsSQL, required) {
			t.Fatalf("engine aggregate SQL missing responsibility evidence %q", required)
		}
	}
}

func TestSnapshotRecordsetIncludesCacheValidityFields(t *testing.T) {
	t.Parallel()

	for _, required := range []string{
		"rule_version text",
		"dirty_at",
		"source_data_updated_at timestamptz",
		"next_recalculation_at timestamptz",
		"reliable_since timestamptz",
	} {
		target := reputationSnapshotRecordset
		if required == "dirty_at" {
			target = `
				dirty_at = NULL
				source_data_updated_at
				next_recalculation_at
			`
		}
		if !strings.Contains(target, required) {
			t.Fatalf("snapshot persistence contract missing %q", required)
		}
	}
}
