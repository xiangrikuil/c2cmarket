package postgres

import (
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/module/operationaudit"
)

func TestOperationAuditQueryUsesAllAuthoritiesAndNeverProjectsSensitiveColumns(t *testing.T) {
	query := operationaudit.Query{
		From:   time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		To:     time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC),
		Search: "safe", Limit: 20,
		Cursor: &operationaudit.CursorPosition{
			OccurredAt: time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
			SourceKind: operationaudit.SourceDomain,
			EventID:    "10000000-0000-4000-8000-000000000001",
		},
	}
	statement, _ := buildOperationAuditQuery(query)
	for _, table := range []string{
		"admin_audit_logs", "moderation_audit_logs", "domain_events", "api_order_events",
		"contact_access_logs", "api_purchase_intent_contact_access_logs",
		"api_order_payment_instruction_access_logs", "api_probe_connection_events",
	} {
		if !strings.Contains(statement, table) {
			t.Fatalf("query missing authority %s", table)
		}
	}
	for _, forbidden := range []string{
		"reason_internal", "event.reason", "event.note", "before_json", "after_json",
		"metadata_json", "event.credential", "event.payment_instructions", "request_body", "cookie",
	} {
		if strings.Contains(strings.ToLower(statement), forbidden) {
			t.Fatalf("query references forbidden data %q", forbidden)
		}
	}
	if !strings.Contains(statement, "event.actor_kind = $") || strings.Count(statement, "LIMIT") != len(operationaudit.SourceKinds)+1 {
		t.Fatal("query must enforce the action/actor allowlist and bound every source branch before the final page")
	}
	for _, fragment := range []string{
		"ORDER BY event.created_at DESC, event.id DESC",
		"ORDER BY event.accessed_at DESC, event.id DESC",
		"ORDER BY event.occurred_at DESC, event.id DESC",
	} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("query missing source-local bounded order %q", fragment)
		}
	}
}

func TestOperationAuditQueryPushesSourceAndResourceFiltersIntoBranch(t *testing.T) {
	query := operationaudit.Query{
		SourceKind:  operationaudit.SourceAPIOrder,
		Action:      "api_order.payment_confirmed",
		ActorKind:   operationaudit.ActorUser,
		ActorUserID: "10000000-0000-4000-8000-000000000001",
		TargetType:  "api_order",
		TargetID:    "20000000-0000-4000-8000-000000000001",
		From:        time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		To:          time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC),
		Limit:       10,
	}
	statement, args := buildOperationAuditQuery(query)
	if !strings.Contains(statement, "FROM api_order_events event") || strings.Contains(statement, "FROM admin_audit_logs event") {
		t.Fatalf("source filter must omit unrelated physical branches: %s", statement)
	}
	for _, fragment := range []string{
		"event.event_type = $", "event.actor_user_id = $", "'api_order' = $", "event.api_order_id = $",
	} {
		if !strings.Contains(statement, fragment) {
			t.Fatalf("query missing pushed filter %q", fragment)
		}
	}
	if len(args) == 0 {
		t.Fatal("expected bound query arguments")
	}
}

func TestOperationAuditQueryExcludesUnknownAndImpossibleActorActions(t *testing.T) {
	query := operationaudit.Query{
		From:  time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		To:    time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC),
		Limit: 20,
	}
	statement, args := buildOperationAuditQuery(query)
	values := stringArguments(args)
	if strings.Contains(values, "\nprivate.secret\n") {
		t.Fatal("unknown action must not enter the registry CTE")
	}
	if !strings.Contains(statement, "event.actor_kind = $") {
		t.Fatal("actor kind must be part of the physical allowlist predicate")
	}
	if !strings.Contains(values, "\napi_order.auto_completed\n") || !strings.Contains(values, "\nsystem\n") {
		t.Fatal("system API-order action must be represented in the allowlist")
	}
}

func TestOperationAuditQueryPushesDomainAndOutcomeIntoEligibleBranches(t *testing.T) {
	query := operationaudit.Query{
		Domain:  operationaudit.DomainInstitution,
		Outcome: operationaudit.OutcomeSucceeded,
		From:    time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		To:      time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC),
		Limit:   20,
	}
	statement, args := buildOperationAuditQuery(query)
	if !strings.Contains(statement, "FROM admin_audit_logs event") {
		t.Fatal("institution/succeeded must keep its admin authority")
	}
	for _, unrelated := range []string{
		"moderation_audit_logs", "domain_events", "api_order_events",
		"contact_access_logs", "api_purchase_intent_contact_access_logs",
		"api_order_payment_instruction_access_logs", "api_probe_connection_events",
	} {
		if strings.Contains(statement, unrelated) {
			t.Fatalf("domain/outcome filter must prune unrelated source %s", unrelated)
		}
	}
	values := stringArguments(args)
	if !strings.Contains(values, "\nstudent_institution_domain.created\n") ||
		strings.Contains(values, "\nstudent_institution_domain.updated\n") {
		t.Fatalf("outcome filter did not reduce the physical tuple allowlist: %s", values)
	}
	if strings.Contains(statement, "projected.domain") || strings.Contains(statement, "projected.outcome") {
		t.Fatal("domain/outcome filtering must not wait until after the UNION")
	}
}

func TestOperationAuditSearchRemainsInsideBoundedSourceBranch(t *testing.T) {
	query := operationaudit.Query{
		SourceKind: operationaudit.SourceDomain,
		From:       time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		To:         time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC),
		Search:     "carpool",
		Limit:      20,
	}
	statement, _ := buildOperationAuditQuery(query)
	searchPosition := strings.Index(statement, "strpos(lower(")
	branchLimitPosition := strings.Index(statement, "ORDER BY event.created_at DESC, event.id DESC")
	if searchPosition < 0 || branchLimitPosition < 0 || searchPosition > branchLimitPosition {
		t.Fatalf("search must run under the source time/cursor predicates before its bounded order: %s", statement)
	}
	if strings.Count(statement, "LIMIT") != 2 {
		t.Fatalf("single-source search must have a branch and final bound: %s", statement)
	}
}

func stringArguments(args []any) string {
	values := make([]string, 0, len(args))
	for _, value := range args {
		if text, ok := value.(string); ok {
			values = append(values, text)
		}
	}
	return "\n" + strings.Join(values, "\n") + "\n"
}
