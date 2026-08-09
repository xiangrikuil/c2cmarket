package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/report"

	"github.com/jackc/pgx/v5"
)

func TestResolveAPIIntentTargetCanonicalization(t *testing.T) {
	queryer := fakeReportQueryer{
		apiIntents: map[string]fakeAPIIntentRow{
			"intent-with-order": {
				title:         "Sub2API 服务",
				status:        "open",
				ownerID:       "merchant-1",
				ownerUsername: "merchant",
				buyerID:       "buyer-1",
				buyerUsername: "buyer",
				orderID:       "order-1",
				orderStatus:   "pending_payment",
			},
			"intent-only": {
				title:         "Sub2API 服务",
				status:        "open",
				ownerID:       "merchant-1",
				ownerUsername: "merchant",
				buyerID:       "buyer-1",
				buyerUsername: "buyer",
			},
		},
	}

	withOrder, found, appErr := resolveAPIIntentTarget(context.Background(), queryer, report.CreateReportInput{
		ReporterUserID: "buyer-1",
		TargetType:     report.TargetAPIPurchaseIntent,
		TargetID:       "intent-with-order",
	})
	if appErr != nil {
		t.Fatalf("resolve intent with order: %v", appErr)
	}
	if !found {
		t.Fatalf("expected intent with order to be found")
	}
	if withOrder.CanonicalTargetType != report.TargetAPIOrder || withOrder.CanonicalTargetID != "order-1" {
		t.Fatalf("expected intent to canonicalize to order, got %+v", withOrder)
	}
	if withOrder.ReporterRole != "buyer" || withOrder.ReportedUserID != "merchant-1" || !withOrder.HasOrder {
		t.Fatalf("unexpected participant snapshot for intent with order: %+v", withOrder)
	}

	withoutOrder, found, appErr := resolveAPIIntentTarget(context.Background(), queryer, report.CreateReportInput{
		ReporterUserID: "merchant-1",
		TargetType:     report.TargetAPIPurchaseIntent,
		TargetID:       "intent-only",
	})
	if appErr != nil {
		t.Fatalf("resolve intent without order: %v", appErr)
	}
	if !found {
		t.Fatalf("expected intent without order to be found")
	}
	if withoutOrder.CanonicalTargetType != report.TargetAPIPurchaseIntent || withoutOrder.CanonicalTargetID != "intent-only" {
		t.Fatalf("expected intent without order to remain canonical intent, got %+v", withoutOrder)
	}
	if withoutOrder.ReporterRole != "merchant" || withoutOrder.ReportedUserID != "buyer-1" || withoutOrder.HasOrder {
		t.Fatalf("unexpected participant snapshot for intent without order: %+v", withoutOrder)
	}
}

func TestResolveCarpoolApplicationTargetCanonicalization(t *testing.T) {
	queryer := fakeReportQueryer{
		carpoolApplications: map[string]fakeCarpoolApplicationRow{
			"application-with-membership": {
				title:            "ChatGPT Plus 车",
				status:           "joined",
				ownerID:          "owner-1",
				ownerUsername:    "owner",
				buyerID:          "buyer-1",
				buyerUsername:    "buyer",
				membershipID:     "membership-1",
				membershipStatus: "active",
			},
			"application-only": {
				title:         "ChatGPT Plus 车",
				status:        "pending",
				ownerID:       "owner-1",
				ownerUsername: "owner",
				buyerID:       "buyer-1",
				buyerUsername: "buyer",
			},
		},
	}

	withMembership, found, appErr := resolveCarpoolApplicationTarget(context.Background(), queryer, report.CreateReportInput{
		ReporterUserID: "owner-1",
		TargetType:     report.TargetCarpoolApplication,
		TargetID:       "application-with-membership",
	})
	if appErr != nil {
		t.Fatalf("resolve application with membership: %v", appErr)
	}
	if !found {
		t.Fatalf("expected application with membership to be found")
	}
	if withMembership.CanonicalTargetType != report.TargetCarpoolMembership || withMembership.CanonicalTargetID != "membership-1" {
		t.Fatalf("expected application to canonicalize to membership, got %+v", withMembership)
	}
	if withMembership.ReporterRole != "owner" || withMembership.ReportedUserID != "buyer-1" || !withMembership.HasMembership {
		t.Fatalf("unexpected participant snapshot for application with membership: %+v", withMembership)
	}

	withoutMembership, found, appErr := resolveCarpoolApplicationTarget(context.Background(), queryer, report.CreateReportInput{
		ReporterUserID: "buyer-1",
		TargetType:     report.TargetCarpoolApplication,
		TargetID:       "application-only",
	})
	if appErr != nil {
		t.Fatalf("resolve application without membership: %v", appErr)
	}
	if !found {
		t.Fatalf("expected application without membership to be found")
	}
	if withoutMembership.CanonicalTargetType != report.TargetCarpoolApplication || withoutMembership.CanonicalTargetID != "application-only" {
		t.Fatalf("expected application without membership to remain canonical application, got %+v", withoutMembership)
	}
	if withoutMembership.ReporterRole != "buyer" || withoutMembership.ReportedUserID != "owner-1" || withoutMembership.HasMembership {
		t.Fatalf("unexpected participant snapshot for application without membership: %+v", withoutMembership)
	}
}

func TestResolveReportTargetRejectsUnauthorizedAndSelfReport(t *testing.T) {
	queryer := fakeReportQueryer{
		users: map[string]string{"alice": "user-1"},
		apiOrders: map[string]fakeAPIOrderRow{
			"order-1": {
				title:         "API 订单",
				status:        "pending_payment",
				ownerID:       "merchant-1",
				ownerUsername: "merchant",
				buyerID:       "buyer-1",
				buyerUsername: "buyer",
			},
		},
	}

	_, _, appErr := resolveAPIOrderTarget(context.Background(), queryer, report.CreateReportInput{
		ReporterUserID: "stranger-1",
		TargetType:     report.TargetAPIOrder,
		TargetID:       "order-1",
	})
	if appErr == nil || appErr.Code != domain.CodePermissionDenied {
		t.Fatalf("expected non-participant to be rejected, got %v", appErr)
	}

	_, appErr = resolveReportTarget(context.Background(), queryer, report.CreateReportInput{
		ReporterUserID:   "user-1",
		ReporterUsername: "alice",
		TargetType:       report.TargetPublicUser,
		TargetID:         "alice",
		ReportedUsername: "alice",
	})
	if appErr == nil || appErr.Code != domain.CodePermissionDenied {
		t.Fatalf("expected public profile self-report to be rejected, got %v", appErr)
	}
}

func TestBuildReportTargetSnapshotIsPublicSafeContext(t *testing.T) {
	snapshot, appErr := buildReportTargetSnapshot(report.CreateReportInput{
		TargetType: report.TargetAPIPurchaseIntent,
		TargetID:   "intent-1",
	}, reportTargetResolution{
		TargetLabel:         "API 购买意向",
		CanonicalTargetType: report.TargetAPIOrder,
		CanonicalTargetID:   "order-1",
		ReportedUsername:    "merchant",
		ReporterRole:        "buyer",
		RespondentUserID:    "merchant-1",
		RespondentUsername:  "merchant",
		Participants: []reportTargetParticipant{
			{Role: "merchant", UserID: "merchant-1", Username: "merchant"},
			{Role: "buyer", UserID: "buyer-1", Username: "buyer"},
		},
		BusinessStatus: "intent:open order:pending_payment",
		HasOrder:       true,
	})
	if appErr != nil {
		t.Fatalf("build snapshot: %v", appErr)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(snapshot), &payload); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if payload["canonicalTargetType"] != report.TargetAPIOrder || payload["canonicalTargetId"] != "order-1" {
		t.Fatalf("snapshot missed canonical target: %v", payload)
	}
	if payload["submittedTargetType"] != report.TargetAPIPurchaseIntent || payload["submittedTargetId"] != "intent-1" {
		t.Fatalf("snapshot missed submitted target: %v", payload)
	}
	if payload["primaryRespondentUserId"] != "merchant-1" || payload["primaryRespondentUsername"] != "merchant" {
		t.Fatalf("snapshot missed primary respondent: %v", payload)
	}
	participants, ok := payload["participants"].([]any)
	if !ok || len(participants) != 2 {
		t.Fatalf("snapshot missed participants: %v", payload)
	}
	if payload["containsContactValue"] != false {
		t.Fatalf("snapshot must explicitly mark contact values absent: %v", payload)
	}
	for _, forbidden := range []string{"contactValue", "paymentCredential", "apiKey", "token", "password", "cookie", "session"} {
		if _, ok := payload[forbidden]; ok {
			t.Fatalf("snapshot contains forbidden field %q: %v", forbidden, payload)
		}
	}
}

func TestReportMigrationKeepsAuditAndDuplicateContracts(t *testing.T) {
	path := filepath.Join("..", "..", "..", "migrations", "000022_reports_disputes_appeals.up.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	sql := string(data)
	for _, required := range []string{
		"public_result_code text NOT NULL DEFAULT 'no_action'",
		"CREATE TABLE moderation_audit_logs",
		"actor_admin_id uuid NOT NULL REFERENCES users(id)",
		"CREATE UNIQUE INDEX ux_reports_active_canonical_target",
		"ON reports(reporter_user_id, canonical_target_type, canonical_target_id)",
		"WHERE status IN ('submitted', 'triaged', 'needs_info', 'dispute_opened')",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration missing required contract %q", required)
		}
	}
	if strings.Contains(sql, "reporter_id, canonical_target_type") {
		t.Fatalf("migration must use reporter_user_id, not reporter_id")
	}
}

func TestModerationInfoRequestMigrationKeepsAuthorizationAndImmutableSupplementContracts(t *testing.T) {
	path := filepath.Join("..", "..", "..", "migrations", "000077_moderation_info_requests.up.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read moderation info request migration: %v", err)
	}
	sql := string(data)
	for _, required := range []string{
		"CREATE TABLE moderation_info_requests",
		"requested_from_user_id uuid NOT NULL REFERENCES users(id)",
		"requested_by_admin_id uuid NOT NULL REFERENCES users(id)",
		"status text NOT NULL CHECK (status IN ('open', 'answered', 'cancelled'))",
		"CREATE UNIQUE INDEX ux_moderation_info_requests_open_report",
		"CREATE UNIQUE INDEX ux_moderation_info_requests_open_dispute",
		"CREATE TABLE moderation_info_supplements",
		"info_request_id uuid NOT NULL UNIQUE REFERENCES moderation_info_requests(id) ON DELETE RESTRICT",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("moderation info request migration missing %q", required)
		}
	}
	if strings.Contains(sql, "UPDATE moderation_info_supplements") {
		t.Fatal("supplements must be append-only and never updated")
	}
}

func TestReportSchemaUpgradeMigrationAlignsLegacyDatabases(t *testing.T) {
	path := filepath.Join("..", "..", "..", "migrations", "000048_report_schema_upgrade.up.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read upgrade migration: %v", err)
	}
	sql := string(data)
	for _, required := range []string{
		"ADD COLUMN IF NOT EXISTS canonical_target_type text",
		"ADD COLUMN IF NOT EXISTS canonical_target_id text",
		"ADD COLUMN IF NOT EXISTS target_snapshot_json jsonb",
		"SET reason_code = 'other'",
		"row_number() OVER",
		"SET status = 'closed'",
		"CREATE UNIQUE INDEX IF NOT EXISTS ux_reports_active_canonical_target",
		"ADD COLUMN IF NOT EXISTS public_result_code text NOT NULL DEFAULT 'no_action'",
		"CREATE TABLE IF NOT EXISTS moderation_audit_logs",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("upgrade migration missing required contract %q", required)
		}
	}

	downPath := filepath.Join("..", "..", "..", "migrations", "000048_report_schema_upgrade.down.sql")
	downData, err := os.ReadFile(downPath)
	if err != nil {
		t.Fatalf("read upgrade down migration: %v", err)
	}
	downSQL := string(downData)
	for _, forbidden := range []string{"DROP COLUMN", "DROP TABLE"} {
		if strings.Contains(downSQL, forbidden) {
			t.Fatalf("upgrade down migration must preserve baseline-owned objects, found %q", forbidden)
		}
	}
}

func TestAppealApprovalChecksOutcomeSubjectBeforeReversal(t *testing.T) {
	path := filepath.Join("report.go")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read report store: %v", err)
	}
	source := string(data)
	start := strings.Index(source, "func reverseReputationOutcomeForApprovedAppeal")
	if start < 0 {
		t.Fatal("appeal reversal function start not found")
	}
	end := strings.Index(source[start:], "func updateDisputeAdminInTx")
	if end < 0 {
		t.Fatal("appeal reversal function boundaries not found")
	}
	section := source[start : start+end]
	guard := strings.Index(section, "report.ValidateAppealOutcomeSubject")
	outcomeUpdate := strings.Index(section, "UPDATE dispute_reputation_outcomes")
	restrictionUpdate := strings.Index(section, "UPDATE user_restrictions")
	if guard < 0 || outcomeUpdate < 0 || restrictionUpdate < 0 {
		t.Fatalf("appeal reversal guard or mutations missing: guard=%d outcome=%d restriction=%d", guard, outcomeUpdate, restrictionUpdate)
	}
	if guard > outcomeUpdate || guard > restrictionUpdate {
		t.Fatal("appeal subject guard must run before outcome and restriction mutations")
	}
}

func TestDisputeResolutionSynchronizesAPIOrderProjectionInsideAdminTransaction(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("report.go"))
	if err != nil {
		t.Fatalf("read report store: %v", err)
	}
	source := string(data)
	for _, required := range []string{
		"closeAPIOrderDisputeProjectionInTx(ctx, tx, item, input, now)",
		"SELECT id::text, status, dispute_status",
		"WHERE id::text = $1",
		"AND dispute_case_id = $2",
		"SET dispute_status = $2",
		"version = version + 1",
		"apiorder.EventDisputeClosed",
		"orderStatus, orderStatus, \"纠纷已结案\"",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("dispute projection transaction is missing %q", required)
		}
	}
}

func TestCreateAppealSerializesSubmittedDuplicateCheck(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("report.go"))
	if err != nil {
		t.Fatalf("read report store: %v", err)
	}
	source := string(data)
	start := strings.Index(source, "func createAppealInTx")
	if start < 0 {
		t.Fatal("appeal create function start not found")
	}
	end := strings.Index(source[start:], "func lockUndestroyedAPIOrderCredentialForModeration")
	if end < 0 {
		t.Fatal("appeal create function boundaries not found")
	}
	section := source[start : start+end]
	lock := strings.Index(section, "pg_advisory_xact_lock")
	duplicateCheck := strings.Index(section, "SELECT EXISTS")
	insert := strings.Index(section, "INSERT INTO appeals")
	if lock < 0 || duplicateCheck < 0 || insert < 0 {
		t.Fatalf("appeal duplicate serialization missing: lock=%d check=%d insert=%d", lock, duplicateCheck, insert)
	}
	if lock > duplicateCheck || duplicateCheck > insert {
		t.Fatal("appeal advisory lock and submitted duplicate check must run before insert")
	}
	credentialGuard := strings.Index(section, "lockUndestroyedAPIOrderCredentialForModeration")
	if credentialGuard < 0 || credentialGuard > insert {
		t.Fatalf("API order appeal lifecycle guard must run before insert: guard=%d insert=%d", credentialGuard, insert)
	}
	for _, required := range []string{"status = 'submitted'", "dispute_case_id = $2", "report_id = $2", "dispute_case_id IS NULL"} {
		if !strings.Contains(section, required) {
			t.Fatalf("appeal duplicate check missing %q", required)
		}
	}

	helperStart := start + end
	helperEnd := strings.Index(source[helperStart:], "func updateAppealAdminInTx")
	if helperEnd < 0 {
		t.Fatal("credential lifecycle helper boundaries not found")
	}
	helperSection := source[helperStart : helperStart+helperEnd]
	credentialLock := strings.Index(helperSection, "apiOrderCredentialLifecycleLockPrefix")
	destroyedCheck := strings.Index(helperSection, "destroyed_at IS NOT NULL")
	if credentialLock < 0 || destroyedCheck < 0 || credentialLock > destroyedCheck {
		t.Fatalf("API order lifecycle helper must lock before checking destruction: lock=%d destroyed_check=%d", credentialLock, destroyedCheck)
	}
}

func TestInfoSupplementLocksParentCaseBeforeInformationRequest(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("report.go"))
	if err != nil {
		t.Fatalf("read report store: %v", err)
	}
	source := string(data)
	start := strings.Index(source, "func submitInfoSupplementInTx")
	if start < 0 {
		t.Fatal("supplement transaction function start not found")
	}
	end := strings.Index(source[start:], "func cancelOpenInfoRequests")
	if end < 0 {
		t.Fatal("supplement transaction function boundaries not found")
	}
	section := source[start : start+end]
	parentLock := strings.Index(section, "FOR UPDATE OF r")
	requestLock := strings.Index(section, "FOR UPDATE OF mir")
	if parentLock < 0 || requestLock < 0 || parentLock > requestLock {
		t.Fatalf("supplement lock order must be parent case before information request: parent=%d request=%d", parentLock, requestLock)
	}
}

func TestOpenDisputeFromReportSerializesCredentialLifecycle(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("report.go"))
	if err != nil {
		t.Fatalf("read report store: %v", err)
	}
	source := string(data)
	start := strings.Index(source, "func updateReportAdminInTx")
	if start < 0 {
		t.Fatal("report admin transaction function start not found")
	}
	end := strings.Index(source[start:], "func createAppealInTx")
	if end < 0 {
		t.Fatal("report admin transaction function boundaries not found")
	}
	section := source[start : start+end]
	lifecycleLock := strings.Index(section, "lockUndestroyedAPIOrderCredentialForModeration")
	disputeInsert := strings.Index(section, "openDisputeFromReport")
	if lifecycleLock < 0 || disputeInsert < 0 || lifecycleLock > disputeInsert {
		t.Fatalf("API order report dispute must lock credential lifecycle before creating the dispute: lock=%d insert=%d", lifecycleLock, disputeInsert)
	}
}

func TestCreateAppealLocksDisputeBeforeAuthorization(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("report.go"))
	if err != nil {
		t.Fatalf("read report store: %v", err)
	}
	source := string(data)
	start := strings.Index(source, "func createAppealInTx")
	if start < 0 {
		t.Fatal("appeal create function start not found")
	}
	end := strings.Index(source[start:], "func updateAppealAdminInTx")
	if end < 0 {
		t.Fatal("appeal create function end not found")
	}
	section := source[start : start+end]
	disputeLock := strings.Index(section, "FOR UPDATE OF d")
	authorization := strings.Index(section, "report.ResolveAppealSource")
	if disputeLock < 0 || authorization < 0 {
		t.Fatalf("appeal dispute lock or authorization missing: lock=%d authorization=%d", disputeLock, authorization)
	}
	if disputeLock > authorization {
		t.Fatal("dispute must be locked before appeal authorization reads its subject")
	}
}

func TestDisputeParticipantMutationLocksCaseBeforeOrderAndCompletesIdempotencyLast(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("report.go"))
	if err != nil {
		t.Fatalf("read report store: %v", err)
	}
	source := string(data)
	start := strings.Index(source, "func (s *Store) UpdateDisputeParticipantWithIdempotency")
	if start < 0 {
		t.Fatal("participant dispute transaction function start not found")
	}
	end := strings.Index(source[start:], "func (s *Store) applyDisputeParticipantActionInTx")
	if end < 0 {
		t.Fatal("participant dispute transaction function end not found")
	}
	section := source[start : start+end]
	disputeLock := strings.Index(section, "FOR UPDATE OF d")
	orderLock := strings.Index(section, "s.getAPIOrder(ctx, tx, item.TargetID, true, false)")
	mutation := strings.Index(section, "s.applyDisputeParticipantActionInTx")
	completion := strings.Index(section, "completeIdempotencyInTx")
	commit := strings.Index(section, "tx.Commit")
	if disputeLock < 0 || orderLock < 0 || mutation < 0 || completion < 0 || commit < 0 {
		t.Fatalf("participant transaction contract is incomplete: dispute=%d order=%d mutation=%d completion=%d commit=%d", disputeLock, orderLock, mutation, completion, commit)
	}
	if !(disputeLock < orderLock && orderLock < mutation && mutation < completion && completion < commit) {
		t.Fatalf("participant transaction lock/write order drifted: dispute=%d order=%d mutation=%d completion=%d commit=%d", disputeLock, orderLock, mutation, completion, commit)
	}
	if !strings.Contains(section, "item.TargetType != report.TargetAPIOrder") || !strings.Contains(section, "return report.DisputeCase{}, idempotency.Completion{}, disputeNotFound()") {
		t.Fatal("non-API disputes must remain hidden from API-order negotiation mutations")
	}
}

func TestEnsureNoActiveReportForCanonicalTarget(t *testing.T) {
	queryer := fakeReportQueryer{
		activeReports: map[string]string{
			"reporter-1|api_order|order-1": "report-1",
		},
	}

	appErr := ensureNoActiveReportForCanonicalTarget(context.Background(), queryer, "reporter-1", report.TargetAPIOrder, "order-1")
	if appErr == nil || appErr.Code != domain.CodeActiveReportExists {
		t.Fatalf("expected duplicate active report rejection, got %v", appErr)
	}

	if appErr := ensureNoActiveReportForCanonicalTarget(context.Background(), queryer, "reporter-1", report.TargetAPIOrder, "order-2"); appErr != nil {
		t.Fatalf("expected different canonical target to be allowed, got %v", appErr)
	}
}

type fakeReportQueryer struct {
	users               map[string]string
	apiIntents          map[string]fakeAPIIntentRow
	apiOrders           map[string]fakeAPIOrderRow
	carpoolApplications map[string]fakeCarpoolApplicationRow
	carpoolMemberships  map[string]fakeCarpoolMembershipRow
	activeReports       map[string]string
}

type fakeAPIIntentRow struct {
	title         string
	status        string
	ownerID       string
	ownerUsername string
	buyerID       string
	buyerUsername string
	orderID       string
	orderStatus   string
}

type fakeAPIOrderRow struct {
	canonicalID   string
	title         string
	status        string
	ownerID       string
	ownerUsername string
	buyerID       string
	buyerUsername string
}

type fakeCarpoolApplicationRow struct {
	title            string
	status           string
	ownerID          string
	ownerUsername    string
	buyerID          string
	buyerUsername    string
	membershipID     string
	membershipStatus string
}

type fakeCarpoolMembershipRow struct {
	title         string
	status        string
	ownerID       string
	ownerUsername string
	buyerID       string
	buyerUsername string
}

func (q fakeReportQueryer) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	switch {
	case strings.Contains(sql, "FROM users WHERE username"):
		username := fmt.Sprint(args[0])
		userID, ok := q.users[username]
		if !ok {
			return fakeReportRow{err: pgx.ErrNoRows}
		}
		return fakeReportRow{values: []any{userID}}
	case strings.Contains(sql, "FROM api_purchase_intents i"):
		id := fmt.Sprint(args[0])
		row, ok := q.apiIntents[id]
		if !ok {
			return fakeReportRow{err: pgx.ErrNoRows}
		}
		return fakeReportRow{values: []any{
			row.title,
			row.status,
			row.ownerID,
			row.ownerUsername,
			row.buyerID,
			row.buyerUsername,
			row.orderID,
			row.orderStatus,
		}}
	case strings.Contains(sql, "FROM api_orders o"):
		id := fmt.Sprint(args[0])
		row, ok := q.apiOrders[id]
		if !ok {
			return fakeReportRow{err: pgx.ErrNoRows}
		}
		canonicalID := row.canonicalID
		if canonicalID == "" {
			canonicalID = id
		}
		return fakeReportRow{values: []any{
			canonicalID,
			row.title,
			row.status,
			row.ownerID,
			row.ownerUsername,
			row.buyerID,
			row.buyerUsername,
		}}
	case strings.Contains(sql, "FROM carpool_applications a"):
		id := fmt.Sprint(args[0])
		row, ok := q.carpoolApplications[id]
		if !ok {
			return fakeReportRow{err: pgx.ErrNoRows}
		}
		return fakeReportRow{values: []any{
			row.title,
			row.status,
			row.ownerID,
			row.ownerUsername,
			row.buyerID,
			row.buyerUsername,
			row.membershipID,
			row.membershipStatus,
		}}
	case strings.Contains(sql, "FROM carpool_memberships m"):
		id := fmt.Sprint(args[0])
		row, ok := q.carpoolMemberships[id]
		if !ok {
			return fakeReportRow{err: pgx.ErrNoRows}
		}
		return fakeReportRow{values: []any{
			row.title,
			row.status,
			row.ownerID,
			row.ownerUsername,
			row.buyerID,
			row.buyerUsername,
		}}
	case strings.Contains(sql, "FROM reports"):
		key := fmt.Sprintf("%s|%s|%s", args[0], args[1], args[2])
		reportID, ok := q.activeReports[key]
		if !ok {
			return fakeReportRow{err: pgx.ErrNoRows}
		}
		return fakeReportRow{values: []any{reportID}}
	default:
		return fakeReportRow{err: fmt.Errorf("unexpected query: %s", sql)}
	}
}

type fakeReportRow struct {
	values []any
	err    error
}

func (r fakeReportRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return fmt.Errorf("scan destination count %d does not match value count %d", len(dest), len(r.values))
	}
	for i, value := range r.values {
		switch target := dest[i].(type) {
		case *string:
			*target = fmt.Sprint(value)
		case *bool:
			v, ok := value.(bool)
			if !ok {
				return fmt.Errorf("value %d is %T, not bool", i, value)
			}
			*target = v
		default:
			return fmt.Errorf("unsupported scan target %T", dest[i])
		}
	}
	return nil
}
