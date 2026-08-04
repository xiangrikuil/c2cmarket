package postgres

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/report"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestPostgresModerationInfoSupplementLifecycle(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)

	admin, appErr := store.EnsureUser(ctx, "supplement-admin-"+strings.ToLower(uuid.NewString()[:8]), true, now)
	if appErr != nil {
		t.Fatalf("ensure supplement admin: %v", appErr)
	}
	reporter, appErr := store.EnsureUser(ctx, "supplement-reporter-"+strings.ToLower(uuid.NewString()[:8]), false, now)
	if appErr != nil {
		t.Fatalf("ensure supplement reporter: %v", appErr)
	}
	other, appErr := store.EnsureUser(ctx, "supplement-other-"+strings.ToLower(uuid.NewString()[:8]), false, now)
	if appErr != nil {
		t.Fatalf("ensure supplement other user: %v", appErr)
	}
	admin.IsAdmin = true
	admin.Status = auth.AccountStatusActive
	reporter.Status = auth.AccountStatusActive
	other.Status = auth.AccountStatusActive

	reportID := uuid.NewString()
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO reports (
			id, reporter_user_id, target_type, target_id, canonical_target_type, canonical_target_id,
			target_label, reported_user_id, reported_username, reason_code, title, description,
			status, created_at, updated_at, version
		)
		VALUES ($1, $2, 'public_user', $3, 'public_user', $3, 'Supplement target', $4, $5,
		        'other', 'Supplement lifecycle', 'Supplement lifecycle facts.', 'submitted', $6, $6, 1)
	`, reportID, reporter.ID, other.Username, other.ID, other.Username, now); err != nil {
		t.Fatalf("seed supplement report: %v", err)
	}

	userIDs := []string{admin.ID, reporter.ID, other.ID}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM notifications WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM domain_events WHERE actor_user_id = ANY($1::uuid[])`, userIDs)
		deleteModerationInfoSupplementsForTest(t, store, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM moderation_info_requests WHERE report_id = $1`, reportID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM dispute_events WHERE actor_user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM moderation_audit_logs WHERE object_type = 'report' AND object_id = $1`, reportID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM idempotency_keys WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM reports WHERE id = $1`, reportID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM user_permissions WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = ANY($1::uuid[])`, userIDs)
	})

	service := report.NewService(store, idempotency.NewService(store, func() time.Time { return now }), func() time.Time { return now })
	requestRoute := "POST /api/v1/admin/reports/{id}/request_info:" + reportID
	requestInput := report.AdminActionInput{
		ID: reportID, Action: "request_info", Reason: "请补充脱敏事实。", RequestedFromID: reporter.ID,
		ExpectedVersion: 1, RequestID: "supplement-request",
	}

	if _, err := store.pool.Exec(ctx, `UPDATE users SET account_status = 'suspended' WHERE id = $1`, reporter.ID); err != nil {
		t.Fatalf("suspend requested reporter: %v", err)
	}
	_, appErr = service.AdminReportActionWithIdempotency(ctx, admin, requestRoute, "inactive-request", "inactive-request-hash", requestInput, moderationSupplementIntegrationCompletion)
	if appErr == nil || appErr.Code != domain.CodePermissionDenied {
		t.Fatalf("inactive requested reporter was accepted: %#v", appErr)
	}
	var status string
	var version int64
	if err := store.pool.QueryRow(ctx, `SELECT status, version FROM reports WHERE id = $1`, reportID).Scan(&status, &version); err != nil {
		t.Fatalf("read report after rejected request: %v", err)
	}
	if status != report.ReportStatusSubmitted || version != 1 {
		t.Fatalf("inactive request must roll back report mutation: status=%s version=%d", status, version)
	}

	if _, err := store.pool.Exec(ctx, `UPDATE users SET account_status = 'active' WHERE id = $1`, reporter.ID); err != nil {
		t.Fatalf("reactivate requested reporter: %v", err)
	}
	_, appErr = service.AdminReportActionWithIdempotency(ctx, admin, requestRoute, "active-request", "active-request-hash", requestInput, moderationSupplementIntegrationCompletion)
	if appErr != nil {
		t.Fatalf("request report supplement: %v", appErr)
	}
	requested, appErr := store.GetAdminReport(ctx, reportID)
	if appErr != nil || requested.OpenInfoRequestID == "" || requested.InfoRequestedFromID != reporter.ID {
		t.Fatalf("open information request missing: report=%+v err=%v", requested, appErr)
	}

	supplementRoute := "POST /api/v1/me/reports/{id}/supplements:" + reportID
	supplementInput := report.SupplementInput{
		EntityType: report.InfoRequestEntityReport, EntityID: reportID, InfoRequestID: requested.OpenInfoRequestID,
		Body: "订单页面时间与站外付款记录不一致，请复核。", RequestID: "supplement-submit",
	}
	_, appErr = service.SubmitInfoSupplementWithIdempotency(ctx, other, supplementRoute, "wrong-user", "wrong-user-hash", supplementInput, moderationSupplementIntegrationCompletion)
	if appErr == nil || appErr.Code != domain.CodeObjectNotFound {
		t.Fatalf("non-designated user supplement was not hidden: %#v", appErr)
	}

	first, appErr := service.SubmitInfoSupplementWithIdempotency(ctx, reporter, supplementRoute, "accepted-supplement", "accepted-supplement-hash", supplementInput, moderationSupplementIntegrationCompletion)
	if appErr != nil {
		t.Fatalf("submit designated supplement: %v", appErr)
	}
	replay, appErr := service.SubmitInfoSupplementWithIdempotency(ctx, reporter, supplementRoute, "accepted-supplement", "accepted-supplement-hash", supplementInput, moderationSupplementIntegrationCompletion)
	if appErr != nil || normalizedIntegrationJSON(replay.Body) != normalizedIntegrationJSON(first.Body) {
		t.Fatalf("supplement replay mismatch: first=%s replay=%s err=%v", first.Body, replay.Body, appErr)
	}

	adminDetail, appErr := store.GetAdminReport(ctx, reportID)
	if appErr != nil || len(adminDetail.Supplements) != 1 {
		t.Fatalf("admin supplement projection missing: report=%+v err=%v", adminDetail, appErr)
	}
	if adminDetail.Status != report.ReportStatusNeedsInfo || adminDetail.OpenInfoRequestID != "" || adminDetail.Supplements[0].Body != supplementInput.Body {
		t.Fatalf("supplement changed case state or projection: %+v", adminDetail)
	}
	var requestStatus string
	var supplementCount int
	if err := store.pool.QueryRow(ctx, `
		SELECT request.status, count(supplement.id)
		FROM moderation_info_requests request
		LEFT JOIN moderation_info_supplements supplement ON supplement.info_request_id = request.id
		WHERE request.id = $1
		GROUP BY request.status
	`, requested.OpenInfoRequestID).Scan(&requestStatus, &supplementCount); err != nil {
		t.Fatalf("read durable supplement state: %v", err)
	}
	if requestStatus != report.InfoRequestStatusAnswered || supplementCount != 1 {
		t.Fatalf("unexpected durable supplement state: status=%s count=%d", requestStatus, supplementCount)
	}
	var supplementID string
	var storedBody string
	if err := store.pool.QueryRow(ctx, `
		SELECT id, body
		FROM moderation_info_supplements
		WHERE info_request_id = $1
	`, requested.OpenInfoRequestID).Scan(&supplementID, &storedBody); err != nil {
		t.Fatalf("read stored supplement: %v", err)
	}
	if storedBody != supplementInput.Body {
		t.Fatalf("unexpected stored supplement body: %q", storedBody)
	}
	_, err := store.pool.Exec(ctx, `UPDATE moderation_info_supplements SET body = 'rewritten' WHERE id = $1`, supplementID)
	assertModerationSupplementMutationRejected(t, err, "update")
	_, err = store.pool.Exec(ctx, `DELETE FROM moderation_info_supplements WHERE id = $1`, supplementID)
	assertModerationSupplementMutationRejected(t, err, "delete")
	if err := store.pool.QueryRow(ctx, `SELECT body FROM moderation_info_supplements WHERE id = $1`, supplementID).Scan(&storedBody); err != nil {
		t.Fatalf("read supplement after rejected mutations: %v", err)
	}
	if storedBody != supplementInput.Body {
		t.Fatalf("rejected mutations changed supplement body: %q", storedBody)
	}
	var notificationBody string
	if err := store.pool.QueryRow(ctx, `
		SELECT body FROM notifications
		WHERE user_id = $1 AND source_event_type = 'moderation.info_supplemented'
	`, admin.ID).Scan(&notificationBody); err != nil {
		t.Fatalf("read requesting-admin notification: %v", err)
	}
	if strings.Contains(notificationBody, supplementInput.Body) {
		t.Fatalf("supplement body leaked into notification: %q", notificationBody)
	}
}

func assertModerationSupplementMutationRejected(t *testing.T, err error, operation string) {
	t.Helper()
	if err == nil {
		t.Fatalf("append-only supplement %s unexpectedly succeeded", operation)
	}
	var postgresErr *pgconn.PgError
	if !errors.As(err, &postgresErr) || postgresErr.Code != "55000" {
		t.Fatalf("append-only supplement %s returned unexpected error: %v", operation, err)
	}
}

func deleteModerationInfoSupplementsForTest(t *testing.T, store *Store, submittedByUserIDs []string) {
	t.Helper()
	ctx := context.Background()
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		t.Errorf("begin moderation supplement cleanup: %v", err)
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `ALTER TABLE moderation_info_supplements DISABLE TRIGGER trg_moderation_info_supplements_append_only`); err != nil {
		t.Errorf("disable moderation supplement append-only trigger for cleanup: %v", err)
		return
	}
	if _, err := tx.Exec(ctx, `DELETE FROM moderation_info_supplements WHERE submitted_by_user_id = ANY($1::uuid[])`, submittedByUserIDs); err != nil {
		t.Errorf("delete moderation supplement fixtures: %v", err)
		return
	}
	if _, err := tx.Exec(ctx, `ALTER TABLE moderation_info_supplements ENABLE TRIGGER trg_moderation_info_supplements_append_only`); err != nil {
		t.Errorf("restore moderation supplement append-only trigger after cleanup: %v", err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		t.Errorf("commit moderation supplement cleanup: %v", err)
	}
}

func TestPostgresSupplementAndAdminCloseUseParentFirstLockOrder(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 3, 11, 0, 0, 0, time.UTC)

	admin, appErr := store.EnsureUser(ctx, "lock-admin-"+strings.ToLower(uuid.NewString()[:8]), true, now)
	if appErr != nil {
		t.Fatalf("ensure lock admin: %v", appErr)
	}
	reporter, appErr := store.EnsureUser(ctx, "lock-reporter-"+strings.ToLower(uuid.NewString()[:8]), false, now)
	if appErr != nil {
		t.Fatalf("ensure lock reporter: %v", appErr)
	}
	admin.IsAdmin = true
	admin.Status = auth.AccountStatusActive
	reporter.Status = auth.AccountStatusActive
	reportID := uuid.NewString()
	infoRequestID := uuid.NewString()
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO reports (
			id, reporter_user_id, target_type, target_id, canonical_target_type, canonical_target_id,
			target_label, reported_username, reason_code, title, description, status,
			admin_reason, handled_by_admin_id, handled_at, created_at, updated_at, version
		)
		VALUES ($1, $2, 'public_user', $3, 'public_user', $3, 'Lock target', 'lock-target',
		        'other', 'Lock order', 'Lock order facts.', 'needs_info', 'Need facts', $4, $5, $5, $5, 2)
	`, reportID, reporter.ID, "lock-target-"+uuid.NewString(), admin.ID, now); err != nil {
		t.Fatalf("seed lock-order report: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO moderation_info_requests (
			id, entity_type, report_id, requested_from_user_id, requested_by_admin_id,
			internal_reason, status, requested_at, created_at
		)
		VALUES ($1, 'report', $2, $3, $4, 'Need lock-order facts', 'open', $5, $5)
	`, infoRequestID, reportID, reporter.ID, admin.ID, now); err != nil {
		t.Fatalf("seed lock-order information request: %v", err)
	}

	userIDs := []string{admin.ID, reporter.ID}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM notifications WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM domain_events WHERE actor_user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM moderation_info_supplements WHERE submitted_by_user_id = $1`, reporter.ID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM moderation_info_requests WHERE id = $1`, infoRequestID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM idempotency_keys WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM reports WHERE id = $1`, reportID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM user_permissions WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = ANY($1::uuid[])`, userIDs)
	})

	adminTx, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin admin lock-order transaction: %v", err)
	}
	defer func() { _ = adminTx.Rollback(context.Background()) }()
	if _, err := adminTx.Exec(ctx, `SET LOCAL lock_timeout = '500ms'`); err != nil {
		t.Fatalf("set lock-order timeout: %v", err)
	}
	if _, err := adminTx.Exec(ctx, `SELECT id FROM reports WHERE id = $1 FOR UPDATE`, reportID); err != nil {
		t.Fatalf("lock parent report: %v", err)
	}

	service := report.NewService(store, idempotency.NewService(store, func() time.Time { return now }), func() time.Time { return now })
	resultCh := make(chan *domain.AppError, 1)
	go func() {
		_, submitErr := service.SubmitInfoSupplementWithIdempotency(
			context.Background(), reporter, "POST /api/v1/me/reports/{id}/supplements:"+reportID,
			"lock-order-supplement", "lock-order-supplement-hash",
			report.SupplementInput{
				EntityType: report.InfoRequestEntityReport, EntityID: reportID, InfoRequestID: infoRequestID,
				Body: "Lock ordering supplement facts.", RequestID: "lock-order-supplement",
			}, moderationSupplementIntegrationCompletion,
		)
		resultCh <- submitErr
	}()

	deadline := time.Now().Add(2 * time.Second)
	blocked := false
	for time.Now().Before(deadline) {
		if err := store.pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM pg_stat_activity
				WHERE datname = current_database()
				  AND wait_event_type = 'Lock'
				  AND query LIKE '%FOR UPDATE OF r%'
			)
		`).Scan(&blocked); err != nil {
			t.Fatalf("inspect lock-order waiter: %v", err)
		}
		if blocked {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !blocked {
		t.Fatal("supplement submission did not block on the parent report lock")
	}
	if _, err := adminTx.Exec(ctx, `
		UPDATE moderation_info_requests
		SET status = 'cancelled', cancelled_at = $2
		WHERE id = $1 AND status = 'open'
	`, infoRequestID, now); err != nil {
		t.Fatalf("parent-first admin path could not lock child request: %v", err)
	}
	if _, err := adminTx.Exec(ctx, `UPDATE reports SET status = 'closed', updated_at = $2, version = version + 1 WHERE id = $1`, reportID, now); err != nil {
		t.Fatalf("close parent report: %v", err)
	}
	if err := adminTx.Commit(ctx); err != nil {
		t.Fatalf("commit parent-first admin path: %v", err)
	}
	select {
	case submitErr := <-resultCh:
		if submitErr == nil || submitErr.Code != domain.CodeObjectNotFound {
			t.Fatalf("supplement did not observe the closed parent/request: %#v", submitErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("supplement remained blocked after parent transaction committed")
	}
}

func moderationSupplementIntegrationCompletion(result report.MutationResult) (idempotency.Completion, *domain.AppError) {
	resourceID := ""
	if result.Report != nil {
		resourceID = result.Report.ID
	}
	if result.Dispute != nil {
		resourceID = result.Dispute.ID
	}
	return idempotency.Completion{
		Status: http.StatusOK, ContentType: "application/json", Body: []byte(`{"ok":true}`),
		ResourceType: "moderation_case", ResourceID: resourceID,
	}, nil
}
