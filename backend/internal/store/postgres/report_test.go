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
		return fakeReportRow{values: []any{
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
