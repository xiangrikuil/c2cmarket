package postgres

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/module/operationaudit"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestOperationAuditSingleSourcePlansIntegration(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("C2C_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	store, err := Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer store.Close()
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin explain fixture transaction: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	now := time.Now().UTC().Truncate(time.Microsecond)
	fixture := seedOperationAuditPlanFixture(t, ctx, tx, now)
	if _, err := tx.Exec(ctx, `
		ANALYZE users, admin_audit_logs, moderation_audit_logs, domain_events,
		        api_order_events, contact_access_logs,
		        api_purchase_intent_contact_access_logs,
		        api_order_payment_instruction_access_logs,
		        api_probe_connection_events
	`); err != nil {
		t.Fatalf("analyze explain fixture: %v", err)
	}

	sources := []operationAuditPlanSource{
		{operationaudit.SourceAdmin, "admin_audit_logs", "ix_admin_audit_logs_operation_cursor", "ix_admin_audit_logs_operation_actor_cursor", "ix_admin_audit_logs_operation_target_cursor"},
		{operationaudit.SourceModeration, "moderation_audit_logs", "ix_moderation_audit_logs_operation_cursor", "ix_moderation_audit_logs_actor", "ix_moderation_audit_logs_object"},
		{operationaudit.SourceDomain, "domain_events", "ix_domain_events_operation_cursor", "ix_domain_events_operation_actor_cursor", "ix_domain_events_operation_target_cursor"},
		{operationaudit.SourceAPIOrder, "api_order_events", "ix_api_order_events_operation_cursor", "ix_api_order_events_operation_actor_cursor", "ix_api_order_events_operation_target_cursor"},
		{operationaudit.SourceContactSessionAccess, "contact_access_logs", "ix_contact_access_logs_operation_cursor", "ix_contact_access_logs_operation_actor_cursor", "ix_contact_access_logs_session_accessed"},
		{operationaudit.SourceAPIIntentContactAccess, "api_purchase_intent_contact_access_logs", "ix_api_intent_contact_access_operation_cursor", "ix_api_intent_contact_access_logs_viewer_accessed", "ix_api_intent_contact_access_operation_target_cursor"},
		{operationaudit.SourceAPIOrderAccess, "api_order_payment_instruction_access_logs", "ix_api_order_access_operation_cursor", "ix_api_order_access_operation_actor_cursor", "ix_api_order_payment_instruction_logs_order"},
		{operationaudit.SourceProbe, "api_probe_connection_events", "ix_api_probe_connection_events_time", "ix_api_probe_connection_events_actor_time", "ix_api_probe_connection_events_target_time"},
	}
	for _, source := range sources {
		queries := []struct {
			name          string
			query         operationaudit.Query
			expectedIndex string
		}{
			{
				name: "time",
				query: operationaudit.Query{
					SourceKind: source.sourceKind,
					From:       now.Add(-time.Hour), To: now, Limit: 20,
				},
				expectedIndex: source.timeIndex,
			},
			{
				name: "actor",
				query: operationaudit.Query{
					SourceKind: source.sourceKind, ActorUserID: fixture.actorID,
					From: now.Add(-time.Hour), To: now, Limit: 20,
				},
				expectedIndex: source.actorIndex,
			},
			{
				name: "target",
				query: operationaudit.Query{
					SourceKind: source.sourceKind, TargetID: fixture.targetBySource[source.sourceKind],
					From: now.Add(-time.Hour), To: now, Limit: 20,
				},
				expectedIndex: source.targetIndex,
			},
		}
		for _, item := range queries {
			t.Run(source.sourceKind+"/"+item.name, func(t *testing.T) {
				plan := explainOperationAuditPlan(t, ctx, tx, item.query)
				assertOperationAuditPlan(t, plan, source.table, item.expectedIndex, item.query.Limit+1)
			})
		}
	}
}

type operationAuditPlanSource struct {
	sourceKind  string
	table       string
	timeIndex   string
	actorIndex  string
	targetIndex string
}

type operationAuditPlanFixture struct {
	actorID               string
	otherActorID          string
	targetBySource        map[string]string
	otherContactSessionID string
	otherIntentID         string
	otherOrderID          string
}

type operationAuditExplainEnvelope struct {
	Plan          operationAuditExplainNode `json:"Plan"`
	ExecutionTime float64                   `json:"Execution Time"`
}

type operationAuditExplainNode struct {
	NodeType     string                      `json:"Node Type"`
	RelationName string                      `json:"Relation Name"`
	IndexName    string                      `json:"Index Name"`
	ActualRows   float64                     `json:"Actual Rows"`
	Plans        []operationAuditExplainNode `json:"Plans"`
}

func explainOperationAuditPlan(t *testing.T, ctx context.Context, tx pgx.Tx, query operationaudit.Query) operationAuditExplainEnvelope {
	t.Helper()
	statement, args := buildOperationAuditQuery(query)
	var raw []byte
	if err := tx.QueryRow(ctx, "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+statement, args...).Scan(&raw); err != nil {
		t.Fatalf("explain operation audit query: %v", err)
	}
	var envelopes []operationAuditExplainEnvelope
	if err := json.Unmarshal(raw, &envelopes); err != nil {
		t.Fatalf("decode operation audit plan: %v", err)
	}
	if len(envelopes) != 1 {
		t.Fatalf("unexpected operation audit plan envelope count: %d", len(envelopes))
	}
	return envelopes[0]
}

func assertOperationAuditPlan(
	t *testing.T,
	plan operationAuditExplainEnvelope,
	table string,
	expectedIndex string,
	maximumRows int,
) {
	t.Helper()
	if plan.Plan.ActualRows > float64(maximumRows) {
		t.Fatalf("operation audit plan returned %.0f rows, want at most %d", plan.Plan.ActualRows, maximumRows)
	}
	if plan.ExecutionTime > 250 {
		t.Fatalf("operation audit plan execution took %.3fms, local budget is 250ms", plan.ExecutionTime)
	}
	var foundIndex bool
	usedIndexes := make([]string, 0, 2)
	var walk func(operationAuditExplainNode)
	walk = func(node operationAuditExplainNode) {
		if node.IndexName != "" {
			usedIndexes = append(usedIndexes, node.IndexName)
		}
		if node.IndexName == expectedIndex {
			foundIndex = true
		}
		if node.RelationName == table && node.NodeType == "Seq Scan" {
			t.Errorf("operation audit source %s used an unbounded sequential scan", table)
		}
		if strings.Contains(node.NodeType, "Sort") && node.ActualRows > float64(maximumRows) {
			t.Errorf("operation audit sort processed %.0f rows, want at most %d", node.ActualRows, maximumRows)
		}
		for _, child := range node.Plans {
			walk(child)
		}
	}
	walk(plan.Plan)
	if !foundIndex {
		t.Errorf("operation audit source %s did not activate expected index %s; used %v", table, expectedIndex, usedIndexes)
	}
}

func seedOperationAuditPlanFixture(t *testing.T, ctx context.Context, tx pgx.Tx, now time.Time) operationAuditPlanFixture {
	fixture := seedOperationAuditPlanBaseFixture(t, ctx, tx, now)
	seedOperationAuditPlanRows(
		t,
		ctx,
		tx,
		fixture.actorID,
		fixture.otherActorID,
		fixture.targetBySource,
		fixture.otherContactSessionID,
		fixture.otherIntentID,
		fixture.otherOrderID,
		now,
	)
	return fixture
}

func seedOperationAuditPlanBaseFixture(t *testing.T, ctx context.Context, tx pgx.Tx, now time.Time) operationAuditPlanFixture {
	t.Helper()
	actorID := uuid.NewString()
	otherActorID := uuid.NewString()
	if _, err := tx.Exec(ctx, `
		INSERT INTO users (id, username, display_name, account_status, created_at, updated_at)
		VALUES ($1, $2, 'Audit plan actor', 'active', $4, $4),
		       ($3, $5, 'Audit plan other actor', 'active', $4, $4)
	`, actorID, "audit-plan-"+actorID, otherActorID, now, "audit-plan-"+otherActorID); err != nil {
		t.Fatalf("insert explain actors: %v", err)
	}

	targets := map[string]string{
		operationaudit.SourceAdmin:                  uuid.NewString(),
		operationaudit.SourceModeration:             uuid.NewString(),
		operationaudit.SourceDomain:                 uuid.NewString(),
		operationaudit.SourceProbe:                  uuid.NewString(),
		operationaudit.SourceContactSessionAccess:   uuid.NewString(),
		operationaudit.SourceAPIIntentContactAccess: uuid.NewString(),
		operationaudit.SourceAPIOrder:               uuid.NewString(),
		operationaudit.SourceAPIOrderAccess:         "",
	}
	otherContactSessionID := uuid.NewString()
	otherIntentID := uuid.NewString()
	otherOrderID := uuid.NewString()
	targets[operationaudit.SourceAPIOrderAccess] = targets[operationaudit.SourceAPIOrder]

	ownerContactID := uuid.NewString()
	buyerContactID := uuid.NewString()
	ownerContactVersionID := uuid.NewString()
	buyerContactVersionID := uuid.NewString()
	serviceID := uuid.NewString()
	if _, err := tx.Exec(ctx, `
		INSERT INTO contact_methods (
		  id, user_id, type, label, is_default, enabled, created_at, updated_at
		) VALUES
		  ($1, $2, 'linuxdo', 'linux.do owner', true, true, $5, $5),
		  ($3, $4, 'linuxdo', 'linux.do buyer', true, true, $5, $5)
	`, ownerContactID, otherActorID, buyerContactID, actorID, now); err != nil {
		t.Fatalf("insert explain contact methods: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO contact_method_versions (
		  id, contact_method_id, owner_user_id, value_ciphertext, value_nonce,
		  masked_value, value_fingerprint, encryption_key_version, fingerprint_key_version, created_at
		) VALUES
		  ($1, $2, $3, decode('0102', 'hex'), decode('000000000000000000000000', 'hex'),
		   'owner', $8, 'test-v1', 'test-v1', $7),
		  ($4, $5, $6, decode('0102', 'hex'), decode('000000000000000000000000', 'hex'),
		   'buyer', $9, 'test-v1', 'test-v1', $7)
	`, ownerContactVersionID, ownerContactID, otherActorID,
		buyerContactVersionID, buyerContactID, actorID, now,
		"audit-plan-owner-"+ownerContactVersionID, "audit-plan-buyer-"+buyerContactVersionID); err != nil {
		t.Fatalf("insert explain contact versions: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE contact_methods
		SET current_version_id = CASE id WHEN $1::uuid THEN $2::uuid ELSE $4::uuid END
		WHERE id IN ($1::uuid, $3::uuid)
	`, ownerContactID, ownerContactVersionID, buyerContactID, buyerContactVersionID); err != nil {
		t.Fatalf("activate explain contact versions: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO api_services (
		  id, owner_user_id, merchant_identity_mode, owner_contact_method_id,
		  title, short_description, distribution_system, billing_mode,
		  declared_cny_per_usd_allowance, declared_max_usd_allowance_per_intent,
		  available_usd_allowance, quota_expires_at,
		  minimum_intent_cny, maximum_intent_cny, usage_visibility,
		  review_status, publication_status, moderation_status,
		  accepting_orders, payment_window_minutes,
		  declared_ttft_band, declared_max_concurrency, performance_confirmed_at,
		  prompt_audit_enabled, created_at, updated_at, version
		) VALUES (
		  $1, $2, 'public_profile', $3,
		  'Audit plan service', 'Operation-audit EXPLAIN fixture', 'sub2api', 'metered_usd_quota',
		  1, 1000, 1000, $4::timestamptz + interval '30 days',
		  1, 1000, 'offsite_panel_readonly',
		  'approved', 'online', 'clear', true, 10,
		  'under_1s', 20, $4, false, $4, $4, 1
		)
	`, serviceID, otherActorID, ownerContactID, now); err != nil {
		t.Fatalf("insert explain API service: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO api_service_access_modes (api_service_id, access_mode, public_note)
		VALUES ($1, 'buyer_dedicated_sub_key', 'Audit plan fixture')
	`, serviceID); err != nil {
		t.Fatalf("insert explain API access mode: %v", err)
	}
	insertIntent := `
		INSERT INTO api_purchase_intents (
		  id, api_service_id, api_service_owner_user_id, buyer_user_id, owner_user_id,
		  buyer_contact_method_id, buyer_contact_method_version_id,
		  owner_contact_method_id, owner_contact_method_version_id,
		  status, requested_cny_amount, requested_usd_allowance, selected_access_mode,
		  service_version_snapshot, service_title_snapshot,
		  distribution_system_snapshot, billing_mode_snapshot,
		  buyer_contact_type_snapshot, buyer_contact_label_snapshot,
		  owner_contact_type_snapshot, owner_contact_label_snapshot,
		  minimum_intent_cny_snapshot, pricing_snapshot, contacted_at, created_at, updated_at
		) VALUES (
		  $1, $2, $3, $4, $3,
		  $5, $6, $7, $8,
		  'ordered', 10, 10, 'buyer_dedicated_sub_key',
		  1, 'Audit plan service', 'sub2api', 'metered_usd_quota',
		  'linuxdo', 'linux.do buyer', 'linuxdo', 'linux.do owner',
		  1, '{}'::jsonb, $9, $9, $9
	)`
	for _, intentID := range []string{targets[operationaudit.SourceAPIIntentContactAccess], otherIntentID} {
		if _, err := tx.Exec(ctx, insertIntent, intentID, serviceID, otherActorID, actorID,
			buyerContactID, buyerContactVersionID, ownerContactID, ownerContactVersionID, now); err != nil {
			t.Fatalf("insert explain API intent: %v", err)
		}
	}
	insertOrder := `
		INSERT INTO api_orders (
		  id, api_purchase_intent_id, api_service_id, buyer_user_id, seller_user_id,
		  status, dispute_status, service_title_snapshot, service_version_snapshot,
		  billing_mode_snapshot, requested_usd_allowance_snapshot, cny_per_usd_allowance_snapshot,
		  amount, currency, selected_payment_method,
		  payment_window_minutes_snapshot, payment_expires_at,
		  payment_instructions_snapshot, created_at, updated_at, order_no
		) VALUES (
		  $1, $2, $3, $4, $5,
		  'pending_payment', 'none', 'Audit plan service', 1,
		  'metered_usd_quota', 10, 1,
		  10, 'CNY', 'wechat', 10, $6::timestamptz + interval '2 hours',
		  'Audit plan fixture', $6, $6, $7
	)`
	if _, err := tx.Exec(ctx, insertOrder, targets[operationaudit.SourceAPIOrder], targets[operationaudit.SourceAPIIntentContactAccess], serviceID, actorID, otherActorID, now, "API-20260813-AAAAAAAAAA"); err != nil {
		t.Fatalf("insert explain target API order: %v", err)
	}
	if _, err := tx.Exec(ctx, insertOrder, otherOrderID, otherIntentID, serviceID, actorID, otherActorID, now, "API-20260813-BBBBBBBBBB"); err != nil {
		t.Fatalf("insert explain other API order: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO contact_sessions (id, buyer_user_id, seller_user_id, opens_at, ends_at, status, created_at)
		VALUES ($1, $2, $3, $4, $4::timestamptz + interval '1 day', 'open', $4),
		       ($5, $2, $3, $4, $4::timestamptz + interval '1 day', 'open', $4)
	`, targets[operationaudit.SourceContactSessionAccess], actorID, otherActorID, now, otherContactSessionID); err != nil {
		t.Fatalf("insert explain contact sessions: %v", err)
	}

	return operationAuditPlanFixture{
		actorID:               actorID,
		otherActorID:          otherActorID,
		targetBySource:        targets,
		otherContactSessionID: otherContactSessionID,
		otherIntentID:         otherIntentID,
		otherOrderID:          otherOrderID,
	}
}

func seedOperationAuditPlanRows(
	t *testing.T,
	ctx context.Context,
	tx pgx.Tx,
	actorID string,
	otherActorID string,
	targets map[string]string,
	otherContactSessionID string,
	otherIntentID string,
	otherOrderID string,
	now time.Time,
) {
	t.Helper()
	statements := []struct {
		name string
		sql  string
		args []any
	}{
		{
			name: "admin",
			sql: `
				INSERT INTO admin_audit_logs (
				  id, admin_user_id, action, target_type, target_id, request_id, created_at
				)
				SELECT gen_random_uuid(),
				       CASE WHEN n % 97 = 0 THEN $1::uuid ELSE $2::uuid END,
				       CASE n % 4
				         WHEN 0 THEN 'student_institution_domain.created'
				         WHEN 1 THEN 'student_institution_domain.updated'
				         WHEN 2 THEN 'api_service.approved'
				         ELSE 'user.account_status_changed'
				       END,
				       CASE WHEN n % 4 IN (0, 1) THEN 'student_institution_domain'
				            WHEN n % 4 = 2 THEN 'api_service'
				            ELSE 'user'
				       END,
				       CASE WHEN n % 101 = 0 THEN $3::uuid ELSE gen_random_uuid() END,
				       'audit-plan-admin-' || n::text,
				       $4::timestamptz - (n::text || ' seconds')::interval
				FROM generate_series(1, 20000) AS series(n)
			`,
			args: []any{actorID, otherActorID, targets[operationaudit.SourceAdmin], now},
		},
		{
			name: "moderation",
			sql: `
				INSERT INTO moderation_audit_logs (
				  id, actor_admin_id, action, object_type, object_id, request_id, created_at
				)
				SELECT gen_random_uuid(),
				       CASE WHEN n % 97 = 0 THEN $1::uuid ELSE $2::uuid END,
				       'triage', 'report',
				       CASE WHEN n % 101 = 0 THEN $3::uuid ELSE gen_random_uuid() END,
				       'audit-plan-moderation-' || n::text,
				       $4::timestamptz - (n::text || ' seconds')::interval
				FROM generate_series(1, 20000) AS series(n)
			`,
			args: []any{actorID, otherActorID, targets[operationaudit.SourceModeration], now},
		},
		{
			name: "domain",
			sql: `
				INSERT INTO domain_events (
				  id, aggregate_type, aggregate_id, event_type, actor_user_id,
				  actor_kind, aggregate_version, request_id, created_at
				)
				SELECT gen_random_uuid(), 'contact_method',
				       CASE WHEN n % 101 = 0 THEN $3::uuid ELSE gen_random_uuid() END,
				       'contact_method.created',
				       CASE WHEN n % 97 = 0 THEN $1::uuid ELSE $2::uuid END,
				       'user', n,
				       'audit-plan-domain-' || n::text,
				       $4::timestamptz - (n::text || ' seconds')::interval
				FROM generate_series(1, 20000) AS series(n)
			`,
			args: []any{actorID, otherActorID, targets[operationaudit.SourceDomain], now},
		},
		{
			name: "API order",
			sql: `
				INSERT INTO api_order_events (
				  id, api_order_id, actor_user_id, event_type, request_id, created_at
				)
				SELECT gen_random_uuid(),
				       CASE WHEN n % 101 = 0 THEN $3::uuid ELSE $4::uuid END,
				       CASE WHEN n % 97 = 0 THEN $1::uuid ELSE $2::uuid END,
				       'api_order.payment_confirmed',
				       'audit-plan-order-' || n::text,
				       $5::timestamptz - (n::text || ' seconds')::interval
				FROM generate_series(1, 20000) AS series(n)
			`,
			args: []any{actorID, otherActorID, targets[operationaudit.SourceAPIOrder], otherOrderID, now},
		},
		{
			name: "contact-session access",
			sql: `
				INSERT INTO contact_access_logs (
				  id, contact_session_id, viewer_user_id, accessed_at, request_id
				)
				SELECT gen_random_uuid(),
				       CASE WHEN n % 101 = 0 THEN $3::uuid ELSE $4::uuid END,
				       CASE WHEN n % 97 = 0 THEN $1::uuid ELSE $2::uuid END,
				       $5::timestamptz - (n::text || ' seconds')::interval,
				       'audit-plan-contact-' || n::text
				FROM generate_series(1, 20000) AS series(n)
			`,
			args: []any{actorID, otherActorID, targets[operationaudit.SourceContactSessionAccess], otherContactSessionID, now},
		},
		{
			name: "API intent contact access",
			sql: `
				INSERT INTO api_purchase_intent_contact_access_logs (
				  id, api_purchase_intent_id, viewer_user_id,
				  viewed_contact_owner_side, request_id, accessed_at
				)
				SELECT gen_random_uuid(),
				       CASE WHEN n % 101 = 0 THEN $3::uuid ELSE $4::uuid END,
				       CASE WHEN n % 97 = 0 THEN $1::uuid ELSE $2::uuid END,
				       'merchant', 'audit-plan-intent-' || n::text,
				       $5::timestamptz - (n::text || ' seconds')::interval
				FROM generate_series(1, 20000) AS series(n)
			`,
			args: []any{actorID, otherActorID, targets[operationaudit.SourceAPIIntentContactAccess], otherIntentID, now},
		},
		{
			name: "API order access",
			sql: `
				INSERT INTO api_order_payment_instruction_access_logs (
				  id, api_order_id, buyer_user_id, request_id, accessed_at
				)
				SELECT gen_random_uuid(),
				       CASE WHEN n % 101 = 0 THEN $3::uuid ELSE $4::uuid END,
				       CASE WHEN n % 97 = 0 THEN $1::uuid ELSE $2::uuid END,
				       'audit-plan-order-access-' || n::text,
				       $5::timestamptz - (n::text || ' seconds')::interval
				FROM generate_series(1, 20000) AS series(n)
			`,
			args: []any{actorID, otherActorID, targets[operationaudit.SourceAPIOrderAccess], otherOrderID, now},
		},
		{
			name: "probe",
			sql: `
				INSERT INTO api_probe_connection_events (
				  id, target_connection_id, owner_user_id, actor_user_id, actor_kind,
				  action, changed_fields, request_id, occurred_at, created_at
				)
				SELECT gen_random_uuid(),
				       CASE WHEN n % 101 = 0 THEN $3::uuid ELSE gen_random_uuid() END,
				       $2::uuid,
				       CASE WHEN n % 97 = 0 THEN $1::uuid ELSE $2::uuid END,
				       'user', 'created', ARRAY['name']::text[],
				       'audit-plan-probe-' || n::text,
				       $4::timestamptz - (n::text || ' seconds')::interval,
				       $4::timestamptz - (n::text || ' seconds')::interval
				FROM generate_series(1, 20000) AS series(n)
			`,
			args: []any{actorID, otherActorID, targets[operationaudit.SourceProbe], now},
		},
	}
	for _, statement := range statements {
		if _, err := tx.Exec(ctx, statement.sql, statement.args...); err != nil {
			t.Fatalf("seed %s explain rows: %v", statement.name, err)
		}
	}
}

func TestOperationAuditMixedSourceCursorIntegration(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("C2C_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	store, err := Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer store.Close()
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin mixed-source transaction: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	occurredAt := time.Now().UTC().Truncate(time.Microsecond)
	fixture := seedOperationAuditPlanBaseFixture(t, ctx, tx, occurredAt)
	requestKey := "audit-integration-" + uuid.NewString()
	eventIDs := map[string]string{
		operationaudit.SourceAdmin:                  uuid.NewString(),
		operationaudit.SourceModeration:             uuid.NewString(),
		operationaudit.SourceDomain:                 uuid.NewString(),
		operationaudit.SourceAPIOrder:               uuid.NewString(),
		operationaudit.SourceContactSessionAccess:   uuid.NewString(),
		operationaudit.SourceAPIIntentContactAccess: uuid.NewString(),
		operationaudit.SourceAPIOrderAccess:         uuid.NewString(),
		operationaudit.SourceProbe:                  uuid.NewString(),
	}
	unknownEventID := uuid.NewString()
	duplicateDomainEventID := uuid.NewString()
	legacyProbeEventID := uuid.NewString()
	if _, err := tx.Exec(ctx, `
		INSERT INTO admin_audit_logs (
		  id, admin_user_id, action, target_type, target_id, reason,
		  before_json, after_json, request_id, created_at
		) VALUES (
		  $1, $2, 'user.account_status_changed', 'user', $3,
		  'private moderation reason', '{"secret":"before"}', '{"secret":"after"}', $4, $5
		)
	`, eventIDs[operationaudit.SourceAdmin], fixture.actorID,
		fixture.targetBySource[operationaudit.SourceAdmin], requestKey+"-admin", occurredAt); err != nil {
		t.Fatalf("insert admin event: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO moderation_audit_logs (
		  id, actor_admin_id, action, object_type, object_id,
		  before_json, after_json, reason_internal, request_id, created_at
		) VALUES (
		  $1, $2, 'triage', 'report', $3,
		  '{"secret":"before"}', '{"secret":"after"}', 'private reason', $4, $5
		)
	`, eventIDs[operationaudit.SourceModeration], fixture.actorID,
		fixture.targetBySource[operationaudit.SourceModeration], requestKey+"-moderation", occurredAt); err != nil {
		t.Fatalf("insert moderation event: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO domain_events (
		  id, aggregate_type, aggregate_id, event_type, actor_user_id,
		  actor_kind, aggregate_version, request_id, metadata_json, created_at
		) VALUES
		  ($1, 'user', $2, 'user.student_identity_assigned', $3, 'user', 1, $4, '{"email":"must-not-leak@example.edu"}', $5),
		  ($6, 'user', $2, 'private.secret', $3, 'user', 2, $7, '{"token":"must-not-leak"}', $5),
		  ($8, 'user', $2, 'user.account_status_changed', $3, 'admin', 3, $9, '{}', $5)
	`, eventIDs[operationaudit.SourceDomain], fixture.targetBySource[operationaudit.SourceDomain],
		fixture.actorID, requestKey+"-domain", occurredAt,
		unknownEventID, requestKey+"-unknown", duplicateDomainEventID, requestKey+"-duplicate"); err != nil {
		t.Fatalf("insert domain events: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO api_order_events (
		  id, api_order_id, actor_user_id, event_type, request_id, created_at
		) VALUES ($1, $2, $3, 'api_order.payment_confirmed', $4, $5)
	`, eventIDs[operationaudit.SourceAPIOrder], fixture.targetBySource[operationaudit.SourceAPIOrder],
		fixture.actorID, requestKey+"-api-order", occurredAt); err != nil {
		t.Fatalf("insert API-order event: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO contact_access_logs (
		  id, contact_session_id, viewer_user_id, accessed_at, request_id
		) VALUES ($1, $2, $3, $4, $5)
	`, eventIDs[operationaudit.SourceContactSessionAccess], fixture.targetBySource[operationaudit.SourceContactSessionAccess],
		fixture.actorID, occurredAt, requestKey+"-contact-session"); err != nil {
		t.Fatalf("insert contact-session access event: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO api_purchase_intent_contact_access_logs (
		  id, api_purchase_intent_id, viewer_user_id,
		  viewed_contact_owner_side, request_id, accessed_at
		) VALUES ($1, $2, $3, 'merchant', $4, $5)
	`, eventIDs[operationaudit.SourceAPIIntentContactAccess], fixture.targetBySource[operationaudit.SourceAPIIntentContactAccess],
		fixture.actorID, requestKey+"-intent-contact", occurredAt); err != nil {
		t.Fatalf("insert intent-contact access event: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO api_order_payment_instruction_access_logs (
		  id, api_order_id, buyer_user_id, request_id, accessed_at
		) VALUES ($1, $2, $3, $4, $5)
	`, eventIDs[operationaudit.SourceAPIOrderAccess], fixture.targetBySource[operationaudit.SourceAPIOrderAccess],
		fixture.actorID, requestKey+"-order-access", occurredAt); err != nil {
		t.Fatalf("insert order-access event: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO api_probe_connection_events (
		  id, target_connection_id, owner_user_id, actor_user_id, actor_kind,
		  action, changed_fields, request_id, occurred_at, created_at
		) VALUES ($1, $2, $3, $3, 'user', 'created', ARRAY['name'], $4, $5, $5)
	`, eventIDs[operationaudit.SourceProbe], fixture.targetBySource[operationaudit.SourceProbe],
		fixture.actorID, requestKey+"-probe", occurredAt); err != nil {
		t.Fatalf("insert probe event: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO api_probe_connection_model_changes (
		  id, connection_id, changed_by_user_id, new_measurement_version,
		  new_model, new_protocol, environment, changed_at, created_at
		) VALUES ($1, NULL, $2, 1, 'gpt-test', 'openai_responses_v1', 'test', $3, $3)
	`, legacyProbeEventID, fixture.actorID, occurredAt); err != nil {
		t.Fatalf("legacy model-change view must remain writable: %v", err)
	}
	var legacyAction, legacyOwnerID, legacyRequestID string
	if err := tx.QueryRow(ctx, `
		SELECT action, owner_user_id::text, request_id
		FROM api_probe_connection_events
		WHERE id = $1
	`, legacyProbeEventID).Scan(&legacyAction, &legacyOwnerID, &legacyRequestID); err != nil {
		t.Fatalf("read migrated legacy event: %v", err)
	}
	if legacyAction != "model_changed" || legacyOwnerID != fixture.actorID || !strings.HasPrefix(legacyRequestID, "probe-event-") {
		t.Fatalf("unexpected legacy event projection action=%s owner=%s request=%s", legacyAction, legacyOwnerID, legacyRequestID)
	}
	if _, err := tx.Exec(ctx, `SAVEPOINT append_only_check`); err != nil {
		t.Fatalf("create append-only savepoint: %v", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM api_probe_connection_events WHERE id = $1`, legacyProbeEventID); err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Fatalf("probe event delete must be rejected by append-only ledger: %v", err)
	}
	if _, err := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT append_only_check`); err != nil {
		t.Fatalf("restore append-only savepoint: %v", err)
	}

	const limit = 3
	var cursor *operationaudit.CursorPosition
	items := make([]operationaudit.Entry, 0, len(eventIDs))
	pageCount := 0
	for {
		pageCount++
		page, appErr := listOperationAudit(ctx, tx, operationaudit.Query{
			From: occurredAt.Add(-time.Minute), To: occurredAt.Add(time.Minute),
			Search: strings.ToLower(requestKey), Limit: limit, Cursor: cursor,
		})
		if appErr != nil {
			t.Fatalf("query operation page %d: %v", pageCount, appErr)
		}
		hasMore := len(page) > limit
		if hasMore {
			page = page[:limit]
		}
		items = append(items, page...)
		if !hasMore {
			break
		}
		last := page[len(page)-1]
		cursor = &operationaudit.CursorPosition{
			OccurredAt: last.CreatedAt,
			SourceKind: last.SourceKind,
			EventID:    last.SourceEventID,
		}
		if pageCount > 8 {
			t.Fatal("mixed-source cursor did not terminate")
		}
	}
	if pageCount < 3 {
		t.Fatalf("mixed-source cursor used %d pages, want at least 3", pageCount)
	}
	if len(items) != len(eventIDs) {
		t.Fatalf("mixed-source cursor returned %d rows, want %d: %+v", len(items), len(eventIDs), items)
	}
	seenSources := make(map[string]bool, len(eventIDs))
	seenEvents := make(map[string]bool, len(eventIDs))
	for index, item := range items {
		if index > 0 && !operationAuditEntryBefore(items[index-1], item) {
			t.Fatalf("mixed-source order broke between %+v and %+v", items[index-1], item)
		}
		if seenEvents[item.SourceEventID] {
			t.Fatalf("duplicate event across cursor pages: %s", item.SourceEventID)
		}
		seenEvents[item.SourceEventID] = true
		seenSources[item.SourceKind] = true
		if want := eventIDs[item.SourceKind]; want == "" || want != item.SourceEventID {
			t.Fatalf("unexpected event for source %s: got %s want %s", item.SourceKind, item.SourceEventID, want)
		}
		if strings.Contains(item.RequestID, "private") || strings.Contains(item.ActorUsername, "@") {
			t.Fatalf("unsafe data reached projection: %+v", item)
		}
	}
	for sourceKind := range eventIDs {
		if !seenSources[sourceKind] {
			t.Errorf("mixed-source cursor omitted %s", sourceKind)
		}
	}
	if seenEvents[unknownEventID] || seenEvents[duplicateDomainEventID] {
		t.Fatalf("unknown action or dual-written compatibility event reached projection: %+v", items)
	}
}

func operationAuditEntryBefore(left, right operationaudit.Entry) bool {
	if !left.CreatedAt.Equal(right.CreatedAt) {
		return left.CreatedAt.After(right.CreatedAt)
	}
	if left.SourceKind != right.SourceKind {
		return left.SourceKind > right.SourceKind
	}
	return left.SourceEventID > right.SourceEventID
}
