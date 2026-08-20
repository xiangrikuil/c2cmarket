package postgres

import (
	"os"
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
		"api_order.status = 'completed'",
		"api_order.completed_at >= $2",
		"event.event_type = 'api_order.cancelled'",
		"dispute.status IN ('negotiating', 'open', 'waiting_info')",
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
		"FROM carpool_applications",
		"FROM carpool_memberships",
		"dispute.primary_user_id = requested.user_id",
		"dispute.counterparty_user_id = requested.user_id",
		"FROM carpool_join_confirmations confirmation",
		"AND false",
	} {
		if strings.Contains(aggregateReputationFactsSQL, forbidden) {
			t.Fatalf("aggregate reputation SQL contains false completion source %q", forbidden)
		}
	}
}

func TestAggregateReputationFactsSQLAttributesControllableTimeoutsToResponsibleRole(t *testing.T) {
	t.Parallel()

	for _, required := range []string{"api_order.cancel_reason = 'payment_timeout'"} {
		if !strings.Contains(aggregateReputationFactsSQL, required) {
			t.Fatalf("aggregate reputation SQL missing responsibility evidence %q", required)
		}
	}
}

func TestAggregateReputationEngineFactsSQLExcludesCarpoolSources(t *testing.T) {
	t.Parallel()

	for _, forbidden := range []string{
		"FROM carpool_applications",
		"FROM carpool_memberships",
		"WHEN 'carpool_membership'",
		"ARRAY['carpool'::text",
		"AND false",
	} {
		if strings.Contains(aggregateReputationEngineFactsSQL, forbidden) {
			t.Fatalf("reputation engine SQL contains retired carpool source %q", forbidden)
		}
	}
	for _, required := range []string{
		"review.transaction_type = 'api_order'",
		"dispute.target_type IN ('api_purchase_intent', 'api_order')",
		"restriction.action_code NOT IN ('carpool_publish', 'carpool_apply', 'carpool_accept')",
	} {
		if !strings.Contains(aggregateReputationEngineFactsSQL, required) {
			t.Fatalf("reputation engine SQL missing carpool exclusion guard %q", required)
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

func TestDisputeOutcomeSerializesWithAppealState(t *testing.T) {
	source, err := os.ReadFile("reputation.go")
	if err != nil {
		t.Fatalf("read reputation store: %v", err)
	}
	start := strings.Index(string(source), "func (s *Store) CreateDisputeOutcomeWithIdempotency")
	if start < 0 {
		t.Fatal("dispute outcome function start not found")
	}
	end := strings.Index(string(source)[start:], "func (s *Store) CreateUserRestrictionWithIdempotency")
	if end < 0 {
		t.Fatal("dispute outcome function end not found")
	}
	section := string(source)[start : start+end]
	disputeLock := strings.Index(section, "FOR UPDATE")
	remedyGuard := strings.Index(section, "FROM api_order_dispute_remedies")
	appealGuard := strings.Index(section, "status IN ('submitted', 'approved')")
	subjectUpdate := strings.Index(section, "UPDATE dispute_cases")
	outcomeInsert := strings.Index(section, "INSERT INTO dispute_reputation_outcomes")
	if disputeLock < 0 || remedyGuard < 0 || appealGuard < 0 || subjectUpdate < 0 || outcomeInsert < 0 {
		t.Fatalf("outcome serialization guard missing: disputeLock=%d remedyGuard=%d appealGuard=%d subjectUpdate=%d outcomeInsert=%d", disputeLock, remedyGuard, appealGuard, subjectUpdate, outcomeInsert)
	}
	if disputeLock > remedyGuard || remedyGuard > subjectUpdate || appealGuard > subjectUpdate || appealGuard > outcomeInsert {
		t.Fatal("dispute and latest remedy must be checked before changing the subject or creating an outcome")
	}
}

func TestSourceLinkedRestrictionRechecksAPIOrderOverdueFact(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("reputation.go")
	if err != nil {
		t.Fatalf("read reputation store: %v", err)
	}
	start := strings.Index(string(source), "func (s *Store) CreateUserRestrictionWithIdempotency")
	end := strings.Index(string(source)[start:], "func apiOrderRemedyOutcomeUnavailable")
	if start < 0 || end < 0 {
		t.Fatal("restriction function section not found")
	}
	section := string(source)[start : start+end]
	for _, required := range []string{
		"JOIN dispute_cases dispute ON dispute.id = outcome.dispute_case_id",
		"targetType == report.TargetAPIOrder",
		"apiOrderRestrictionRequiresDedicatedSanction()",
	} {
		if !strings.Contains(section, required) {
			t.Fatalf("source-linked restriction guard missing %q", required)
		}
	}
	if strings.Contains(section, "source_dispute_remedy_id") {
		t.Fatal("generic restrictions must not write API-order remedy evidence")
	}
}

func TestAPIOrderSanctionStoreKeepsEvidenceAndSideEffectsTransactional(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("reputation_sanction.go")
	if err != nil {
		t.Fatalf("read API-order sanction store: %v", err)
	}
	wholeSource := string(source)
	start := strings.Index(wholeSource, "func (s *Store) ApplyAPIOrderSanctionWithIdempotency")
	end := strings.Index(wholeSource, "func (s *Store) ListActiveRestrictions")
	if start < 0 || end <= start {
		t.Fatal("API-order sanction apply function not found")
	}
	section := wholeSource[start:end]
	for _, required := range []string{
		"lockProcessingIdempotencyInTx",
		"loadAPIOrderSanctionRecommendation(ctx, tx, input.DisputeCaseID, now, true)",
		"INSERT INTO user_restrictions",
		"source_dispute_outcome_id, source_dispute_remedy_id",
		"insertReputationGovernanceEvent",
		"insertAPIOrderSanctionNotificationInTx",
		"completeIdempotencyInTx",
		"tx.Commit(ctx)",
	} {
		if !strings.Contains(section, required) {
			t.Fatalf("API-order sanction transaction missing %q", required)
		}
	}
	lockedRecommendation := strings.Index(section, "loadAPIOrderSanctionRecommendation(ctx, tx, input.DisputeCaseID, now, true)")
	restrictionInsert := strings.Index(section, "INSERT INTO user_restrictions")
	governanceInsert := strings.Index(section, "insertReputationGovernanceEvent")
	notificationInsert := strings.Index(section, "insertAPIOrderSanctionNotificationInTx")
	idempotencyCompletion := strings.Index(section, "completeIdempotencyInTx")
	commit := strings.Index(section, "tx.Commit(ctx)")
	if lockedRecommendation < 0 || restrictionInsert < 0 || governanceInsert < 0 || notificationInsert < 0 || idempotencyCompletion < 0 || commit < 0 ||
		lockedRecommendation > restrictionInsert || restrictionInsert > governanceInsert || governanceInsert > notificationInsert || notificationInsert > idempotencyCompletion || idempotencyCompletion > commit {
		t.Fatalf("unexpected API-order sanction transaction order: recommendation=%d restriction=%d governance=%d notification=%d idempotency=%d commit=%d", lockedRecommendation, restrictionInsert, governanceInsert, notificationInsert, idempotencyCompletion, commit)
	}
	if !strings.Contains(wholeSource, "SELECT version FROM users WHERE id = $1`+lockClause") {
		t.Fatal("sanction recommendation helper must lock the subject user during apply")
	}
	if strings.Contains(wholeSource, "'reputation.restriction_created', $3") {
		t.Fatal("notification source_event_id must not reuse a restriction ID that does not reference domain_events")
	}
}

func TestAPIOrderSanctionRecommendationUsesOneFactSnapshot(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("reputation_sanction.go")
	if err != nil {
		t.Fatalf("read API-order sanction store: %v", err)
	}
	section := string(source)
	for _, required := range []string{
		"pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}",
		"loadAPIOrderSanctionRecommendation(ctx, tx, disputeCaseID, now, false)",
		"tx.Commit(ctx)",
	} {
		if !strings.Contains(section, required) {
			t.Fatalf("recommendation snapshot transaction missing %q", required)
		}
	}
	if strings.Contains(section, "loadAPIOrderSanctionRecommendation(ctx, s.pool, disputeCaseID, now, false)") {
		t.Fatal("recommendation must not read related facts through separate pool snapshots")
	}
}

func TestAPIOrderRestrictionGatesPrecedeOrderSideEffects(t *testing.T) {
	t.Parallel()

	assertGateBefore := func(file, functionStart, functionEnd, gate string, sideEffects ...string) {
		t.Helper()
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		start := strings.Index(string(source), functionStart)
		if start < 0 {
			t.Fatalf("%s missing function %q", file, functionStart)
		}
		section := string(source)[start:]
		if end := strings.Index(section[len(functionStart):], functionEnd); end >= 0 {
			section = section[:len(functionStart)+end]
		}
		gateIndex := strings.Index(section, gate)
		if gateIndex < 0 {
			t.Fatalf("%s missing restriction gate %q", file, gate)
		}
		for _, sideEffect := range sideEffects {
			sideEffectIndex := strings.Index(section, sideEffect)
			if sideEffectIndex < 0 {
				t.Fatalf("%s missing guarded side effect %q", file, sideEffect)
			}
			if gateIndex > sideEffectIndex {
				t.Fatalf("%s restriction gate follows side effect %q", file, sideEffect)
			}
		}
	}

	assertGateBefore(
		"api_order.go",
		"func (s *Store) createAPIOrderInTx",
		"func loadReadyProbeTargetInTx",
		"ensureAPIServicePublishAllowedInTx(ctx, tx, service.OwnerUserID, service.ID, now)",
		"reserveAPIOrderInventoryInTx",
		"insertAPIOrderInTx",
		"markAPIPurchaseIntentOrderedInTx",
	)
	assertGateBefore(
		"api_quota.go",
		"func (s *Store) CreateAPIQuotaOrderWithIdempotency",
		"func getAPIQuotaOrderContext",
		"ensureAPIServicePublishAllowedInTx(ctx, tx, orderContext.OwnerUserID, orderContext.APIServiceID, now)",
		"lockTransactionContactVersionForOwner",
		"claimAPIQuotaRoundAndAllocation",
		"FROM api_quota_inventory_units",
		"INSERT INTO api_purchase_intents",
		"insertAPIOrderWithNumberRetry",
	)
}

func TestAPIOrderSanctionRecommendationCountsOverdueSellerFactsWithoutOutcomeStatusDependency(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("reputation_sanction.go")
	if err != nil {
		t.Fatalf("read API-order sanction store: %v", err)
	}
	start := strings.Index(string(source), "SELECT count(*)")
	end := strings.Index(string(source)[start:], ").Scan(&recommendation.ConfirmedBreaches180Days)")
	if start < 0 || end < 0 {
		t.Fatal("sanction breach-count query not found")
	}
	section := string(source)[start : start+end]
	for _, required := range []string{
		"remedy.lateness_status = 'late_confirmed'",
		"remedy.lateness_reversed_at IS NULL",
		"remedy.lateness_decided_at >= $1",
		"remedy.responsible_user_id = $2",
		"order_row.seller_user_id = remedy.responsible_user_id",
	} {
		if !strings.Contains(section, required) {
			t.Fatalf("sanction breach-count query missing %q", required)
		}
	}
	if strings.Contains(section, "dispute_reputation_outcomes") || strings.Contains(section, "outcome.status") {
		t.Fatal("sanction count must use the remedy reversal fact without depending on outcome state")
	}
}

func TestReputationAggregationExcludesAppealReversedLateness(t *testing.T) {
	source, err := os.ReadFile("reputation.go")
	if err != nil {
		t.Fatalf("read reputation store: %v", err)
	}
	section := string(source)
	for _, required := range []string{"remedy.lateness_reversed_at IS NULL", "outcome.status = 'active'"} {
		if !strings.Contains(section, required) {
			t.Fatalf("reputation aggregation missing reversed-lateness guard %q", required)
		}
	}
}
