package postgres

import (
	"context"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/evidence"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type evidenceIntegrationFixture struct {
	buyerID      string
	sellerID     string
	adminID      string
	outsiderID   string
	orderID      string
	otherOrderID string
	caseID       string
	otherCaseID  string
	appealID     string
	supplementID string
}

func TestPostgresEvidenceAuthorizationBindingAndRollback(t *testing.T) {
	pool := connectEvidenceIntegrationPool(t)
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	fixture := seedEvidenceIntegrationFixture(t, tx, now)

	participantAsset := insertReadyEvidenceAsset(t, tx, fixture.orderID, fixture.buyerID, now, now.Add(time.Hour))
	if appErr := bindEvidenceAssetsInTx(ctx, tx, evidence.BindingInput{
		AssetIDs: []string{participantAsset}, APIOrderID: fixture.orderID, DisputeCaseID: fixture.caseID,
		UploaderID: fixture.buyerID, Visibility: evidence.VisibilityParticipantsAdmin,
		Usage: evidence.UsageDisputeInitial, SourceType: evidence.SourceDisputeCase, SourceID: fixture.caseID,
	}, now); appErr != nil {
		t.Fatalf("bind participant evidence: %v", appErr)
	}

	for name, test := range map[string]struct {
		viewer string
		admin  bool
		want   bool
	}{
		"buyer":    {viewer: fixture.buyerID, want: true},
		"seller":   {viewer: fixture.sellerID, want: true},
		"admin":    {viewer: fixture.adminID, admin: true, want: true},
		"outsider": {viewer: fixture.outsiderID, want: false},
	} {
		t.Run("participants visibility "+name, func(t *testing.T) {
			_, appErr := authorizedEvidenceAsset(ctx, tx, participantAsset, test.viewer, test.admin)
			if test.want && appErr != nil {
				t.Fatalf("expected authorized read: %v", appErr)
			}
			if !test.want && (appErr == nil || appErr.Status != http.StatusNotFound || appErr.Code != domain.CodeObjectNotFound) {
				t.Fatalf("expected hidden evidence, got %#v", appErr)
			}
		})
	}

	appealAsset := insertReadyEvidenceAsset(t, tx, fixture.orderID, fixture.buyerID, now, now.Add(time.Hour))
	if appErr := bindEvidenceAssetsInTx(ctx, tx, evidence.BindingInput{
		AssetIDs: []string{appealAsset}, APIOrderID: fixture.orderID, DisputeCaseID: fixture.caseID,
		UploaderID: fixture.buyerID, Visibility: evidence.VisibilityAppellantAdmin,
		Usage: evidence.UsageAppeal, SourceType: evidence.SourceAppeal, SourceID: fixture.appealID,
	}, now); appErr != nil {
		t.Fatalf("bind appeal evidence: %v", appErr)
	}
	if _, appErr := authorizedEvidenceAsset(ctx, tx, appealAsset, fixture.buyerID, false); appErr != nil {
		t.Fatalf("appellant read: %v", appErr)
	}
	if _, appErr := authorizedEvidenceAsset(ctx, tx, appealAsset, fixture.sellerID, false); appErr == nil || appErr.Status != http.StatusNotFound {
		t.Fatalf("seller must not read appellant-only evidence: %#v", appErr)
	}
	if _, appErr := authorizedEvidenceAsset(ctx, tx, appealAsset, fixture.adminID, true); appErr != nil {
		t.Fatalf("admin appeal read: %v", appErr)
	}

	supplementAsset := insertReadyEvidenceAsset(t, tx, fixture.orderID, fixture.sellerID, now, now.Add(time.Hour))
	if appErr := bindEvidenceAssetsInTx(ctx, tx, evidence.BindingInput{
		AssetIDs: []string{supplementAsset}, APIOrderID: fixture.orderID, DisputeCaseID: fixture.caseID,
		UploaderID: fixture.sellerID, Visibility: evidence.VisibilitySubmitterAdmin,
		Usage: evidence.UsageInfoSupplement, SourceType: evidence.SourceInfoSupplement, SourceID: fixture.supplementID,
	}, now); appErr != nil {
		t.Fatalf("bind supplement evidence: %v", appErr)
	}
	if _, appErr := authorizedEvidenceAsset(ctx, tx, supplementAsset, fixture.sellerID, false); appErr != nil {
		t.Fatalf("supplement submitter read: %v", appErr)
	}
	if _, appErr := authorizedEvidenceAsset(ctx, tx, supplementAsset, fixture.buyerID, false); appErr == nil || appErr.Status != http.StatusNotFound {
		t.Fatalf("other participant must not read submitter-only evidence: %#v", appErr)
	}

	messageID := uuid.NewString()
	if _, err := tx.Exec(ctx, `
		INSERT INTO api_order_dispute_messages (id, dispute_case_id, sender_user_id, body, request_id, created_at)
		VALUES ($1, $2, $3, 'message evidence', $4, $5)
	`, messageID, fixture.caseID, fixture.buyerID, uuid.NewString(), now); err != nil {
		t.Fatal(err)
	}
	messageAsset := insertReadyEvidenceAsset(t, tx, fixture.orderID, fixture.buyerID, now, now.Add(time.Hour))
	if appErr := bindEvidenceAssetsInTx(ctx, tx, evidence.BindingInput{
		AssetIDs: []string{messageAsset}, APIOrderID: fixture.orderID, DisputeCaseID: fixture.caseID,
		UploaderID: fixture.buyerID, Visibility: evidence.VisibilityParticipantsAdmin,
		Usage: evidence.UsageMessage, SourceType: evidence.SourceDisputeMessage, SourceID: messageID,
	}, now); appErr != nil {
		t.Fatalf("bind message evidence: %v", appErr)
	}

	remedyID := uuid.NewString()
	if _, err := tx.Exec(ctx, `
		INSERT INTO api_order_dispute_remedies (
			id, dispute_case_id, action, responsible_user_id, beneficiary_user_id,
			instructions, status, due_at, claimed_at, confirmation_due_at,
			created_by_admin_id, created_at, updated_at
		) VALUES ($1, $2, 'full_refund', $3, $4, 'refund outside platform', 'claimed_fulfilled',
		          $5, $6, $6::timestamptz + interval '48 hours', $7, $6, $6)
	`, remedyID, fixture.caseID, fixture.sellerID, fixture.buyerID, now.Add(time.Hour), now, fixture.adminID); err != nil {
		t.Fatal(err)
	}
	for _, usage := range []string{evidence.UsageRemedyClaim, evidence.UsageRemedyContest} {
		assetID := insertReadyEvidenceAsset(t, tx, fixture.orderID, fixture.sellerID, now, now.Add(time.Hour))
		if appErr := bindEvidenceAssetsInTx(ctx, tx, evidence.BindingInput{
			AssetIDs: []string{assetID}, APIOrderID: fixture.orderID, DisputeCaseID: fixture.caseID,
			UploaderID: fixture.sellerID, Visibility: evidence.VisibilityParticipantsAdmin,
			Usage: usage, SourceType: evidence.SourceDisputeRemedy, SourceID: remedyID,
		}, now); appErr != nil {
			t.Fatalf("bind %s evidence: %v", usage, appErr)
		}
	}

	crossOrder := insertReadyEvidenceAsset(t, tx, fixture.otherOrderID, fixture.buyerID, now, now.Add(time.Hour))
	assertEvidenceBindConflict(t, bindEvidenceAssetsInTx(ctx, tx, evidence.BindingInput{
		AssetIDs: []string{crossOrder}, APIOrderID: fixture.orderID, DisputeCaseID: fixture.caseID,
		UploaderID: fixture.buyerID, Visibility: evidence.VisibilityParticipantsAdmin,
		Usage: evidence.UsageDisputeInitial, SourceType: evidence.SourceDisputeCase, SourceID: fixture.caseID,
	}, now))

	crossUser := insertReadyEvidenceAsset(t, tx, fixture.orderID, fixture.sellerID, now, now.Add(time.Hour))
	assertEvidenceBindConflict(t, bindEvidenceAssetsInTx(ctx, tx, evidence.BindingInput{
		AssetIDs: []string{crossUser}, APIOrderID: fixture.orderID, DisputeCaseID: fixture.caseID,
		UploaderID: fixture.buyerID, Visibility: evidence.VisibilityParticipantsAdmin,
		Usage: evidence.UsageDisputeInitial, SourceType: evidence.SourceDisputeCase, SourceID: fixture.caseID,
	}, now))

	assertEvidenceBindConflict(t, bindEvidenceAssetsInTx(ctx, tx, evidence.BindingInput{
		AssetIDs: []string{participantAsset}, APIOrderID: fixture.orderID, DisputeCaseID: fixture.caseID,
		UploaderID: fixture.buyerID, Visibility: evidence.VisibilityParticipantsAdmin,
		Usage: evidence.UsageDisputeInitial, SourceType: evidence.SourceDisputeCase, SourceID: fixture.caseID,
	}, now))

	duplicate := insertReadyEvidenceAsset(t, tx, fixture.orderID, fixture.buyerID, now, now.Add(time.Hour))
	if appErr := bindEvidenceAssetsInTx(ctx, tx, evidence.BindingInput{
		AssetIDs: []string{duplicate, duplicate}, APIOrderID: fixture.orderID, DisputeCaseID: fixture.caseID,
		UploaderID: fixture.buyerID, Visibility: evidence.VisibilityParticipantsAdmin,
		Usage: evidence.UsageDisputeInitial, SourceType: evidence.SourceDisputeCase, SourceID: fixture.caseID,
	}, now); appErr == nil || appErr.Status != http.StatusUnprocessableEntity {
		t.Fatalf("duplicate input must fail validation: %#v", appErr)
	}

	wrongSource := insertReadyEvidenceAsset(t, tx, fixture.orderID, fixture.buyerID, now, now.Add(time.Hour))
	assertEvidenceBindConflict(t, bindEvidenceAssetsInTx(ctx, tx, evidence.BindingInput{
		AssetIDs: []string{wrongSource}, APIOrderID: fixture.orderID, DisputeCaseID: fixture.caseID,
		UploaderID: fixture.buyerID, Visibility: evidence.VisibilityParticipantsAdmin,
		Usage: evidence.UsageDisputeInitial, SourceType: evidence.SourceDisputeCase, SourceID: fixture.otherCaseID,
	}, now))

	crossDisputeMessageID := uuid.NewString()
	if _, err := tx.Exec(ctx, `
		INSERT INTO api_order_dispute_messages (id, dispute_case_id, sender_user_id, body, request_id, created_at)
		VALUES ($1, $2, $3, 'cross-dispute evidence source', $4, $5)
	`, crossDisputeMessageID, fixture.otherCaseID, fixture.buyerID, uuid.NewString(), now); err != nil {
		t.Fatal(err)
	}
	databaseGuardAsset := insertReadyEvidenceAsset(t, tx, fixture.orderID, fixture.buyerID, now, now.Add(time.Hour))
	if _, err := tx.Exec(ctx, "SAVEPOINT evidence_source_integrity"); err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO api_order_evidence_bindings (
			asset_id, dispute_case_id, visibility, usage, source_type, source_id,
			dispute_message_id, created_at
		) VALUES ($1, $2, 'participants_admin', 'message', 'dispute_message', $3, $3, $4)
	`, databaseGuardAsset, fixture.caseID, crossDisputeMessageID, now)
	if err == nil {
		t.Fatal("database accepted an evidence source from another dispute")
	}
	if _, rollbackErr := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT evidence_source_integrity"); rollbackErr != nil {
		t.Fatalf("restore transaction after expected source-integrity rejection: %v", rollbackErr)
	}

	rollbackAsset := insertReadyEvidenceAsset(t, tx, fixture.orderID, fixture.buyerID, now, now.Add(time.Hour))
	if _, err := tx.Exec(ctx, "SAVEPOINT evidence_mutation"); err != nil {
		t.Fatal(err)
	}
	rollbackMessageID := uuid.NewString()
	if _, err := tx.Exec(ctx, `INSERT INTO api_order_dispute_messages (id, dispute_case_id, sender_user_id, body, request_id, created_at) VALUES ($1, $2, $3, 'rollback message', $4, $5)`, rollbackMessageID, fixture.caseID, fixture.buyerID, "rollback-"+rollbackMessageID, now); err != nil {
		t.Fatal(err)
	}
	if appErr := bindEvidenceAssetsInTx(ctx, tx, evidence.BindingInput{
		AssetIDs: []string{rollbackAsset}, APIOrderID: fixture.orderID, DisputeCaseID: fixture.caseID,
		UploaderID: fixture.buyerID, Visibility: evidence.VisibilityParticipantsAdmin,
		Usage: evidence.UsageMessage, SourceType: evidence.SourceDisputeMessage, SourceID: rollbackMessageID,
	}, now); appErr != nil {
		t.Fatalf("bind rollback evidence: %v", appErr)
	}
	if _, err := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT evidence_mutation"); err != nil {
		t.Fatal(err)
	}
	var bindingCount, messageCount int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM api_order_evidence_bindings WHERE asset_id = $1`, rollbackAsset).Scan(&bindingCount); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM api_order_dispute_messages WHERE id = $1`, rollbackMessageID).Scan(&messageCount); err != nil {
		t.Fatal(err)
	}
	if bindingCount != 0 || messageCount != 0 {
		t.Fatalf("mutation rollback leaked state binding=%d message=%d", bindingCount, messageCount)
	}
}

func TestPostgresEvidenceQuarantineIsAtomicUnreadableAndSecretSafe(t *testing.T) {
	pool := connectEvidenceIntegrationPool(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	setupTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = setupTx.Rollback(context.Background()) })
	fixture := seedEvidenceIntegrationFixture(t, setupTx, now)
	assetID := insertReadyEvidenceAsset(t, setupTx, fixture.orderID, fixture.buyerID, now, now.Add(time.Hour))
	rollbackAssetID := insertReadyEvidenceAsset(t, setupTx, fixture.orderID, fixture.buyerID, now, now.Add(time.Hour))
	for _, id := range []string{assetID, rollbackAssetID} {
		if appErr := bindEvidenceAssetsInTx(ctx, setupTx, evidence.BindingInput{
			AssetIDs: []string{id}, APIOrderID: fixture.orderID, DisputeCaseID: fixture.caseID,
			UploaderID: fixture.buyerID, Visibility: evidence.VisibilityParticipantsAdmin,
			Usage: evidence.UsageDisputeInitial, SourceType: evidence.SourceDisputeCase, SourceID: fixture.caseID,
		}, now); appErr != nil {
			_ = setupTx.Rollback(ctx)
			t.Fatalf("bind quarantine evidence: %v", appErr)
		}
	}
	if err := setupTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupEvidenceIntegrationFixture(t, pool, fixture) })

	store := &Store{pool: pool}
	if _, appErr := store.AuthorizedAsset(ctx, assetID, fixture.adminID, true); appErr != nil {
		t.Fatalf("bound evidence must be readable before quarantine: %v", appErr)
	}
	entry := beginEvidenceQuarantineIdempotency(t, store, fixture.adminID, now, "quarantine-success")
	result, completion, appErr := store.QuarantineAssetWithIdempotency(ctx, entry, evidence.AdminQuarantineInput{
		AssetID: assetID, AdminUserID: fixture.adminID, ExpectedVersion: 1,
		Reason: "图片包含未遮挡的完整账号信息。", RequestID: "quarantine-request-success",
	}, now, func(result evidence.AdminQuarantineResult) (idempotency.Completion, *domain.AppError) {
		return idempotency.Completion{
			Status: http.StatusOK, ContentType: "application/json",
			Body: []byte(`{"status":"quarantined"}`), ResourceType: "api_order_evidence", ResourceID: result.ID,
		}, nil
	})
	if appErr != nil {
		t.Fatalf("quarantine evidence: %v", appErr)
	}
	if result.Status != "quarantined" || result.Version != 2 || !result.QuarantinedExpiresAt.Equal(now.Add(evidence.QuarantineRetention)) || completion.ResourceID != assetID {
		t.Fatalf("unexpected quarantine result=%+v completion=%+v", result, completion)
	}
	var status, scanStatus string
	var version int64
	var expiresAt time.Time
	if err := pool.QueryRow(ctx, `
		SELECT status, scan_status, version, quarantined_expires_at
		FROM api_order_evidence_assets WHERE id = $1
	`, assetID).Scan(&status, &scanStatus, &version, &expiresAt); err != nil {
		t.Fatal(err)
	}
	if status != "quarantined" || scanStatus != "rejected" || version != 2 || !expiresAt.Equal(now.Add(7*24*time.Hour)) {
		t.Fatalf("unexpected persisted quarantine status=%s scan=%s version=%d expires=%s", status, scanStatus, version, expiresAt)
	}
	if _, appErr := store.AuthorizedAsset(ctx, assetID, fixture.adminID, true); appErr == nil || appErr.Code != domain.CodeObjectNotFound {
		t.Fatalf("quarantined evidence remained readable: %#v", appErr)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE api_order_evidence_assets
		SET status = 'destroy_pending', destroy_requested_at = $2, destroy_reason = 'quarantine_retention_expired'
		WHERE id = $1
	`, assetID, now.Add(evidence.QuarantineRetention)); err != nil {
		t.Fatal(err)
	}
	if _, appErr := store.AuthorizedAsset(ctx, assetID, fixture.adminID, true); appErr == nil || appErr.Code != domain.CodeObjectNotFound {
		t.Fatalf("quarantined evidence became readable while deletion was pending: %#v", appErr)
	}
	references, err := listEvidenceReferences(ctx, pool, fixture.caseID)
	if err != nil {
		t.Fatal(err)
	}
	for _, reference := range references {
		if reference.ID == assetID {
			t.Fatalf("quarantined evidence remained in active DTO references: %+v", reference)
		}
	}

	var eventPayload, auditPayload, cachedBody string
	if err := pool.QueryRow(ctx, `SELECT metadata_json::text FROM domain_events WHERE aggregate_type = 'api_order_evidence' AND aggregate_id = $1`, assetID).Scan(&eventPayload); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT concat_ws(' ', COALESCE(reason, ''), COALESCE(before_json::text, ''), COALESCE(after_json::text, ''))
		FROM admin_audit_logs WHERE action = 'api_order_evidence.quarantined' AND target_id = $1
	`, assetID).Scan(&auditPayload); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT COALESCE(response_body_json::text, '') FROM idempotency_keys
		WHERE user_id = $1 AND route_key = $2 AND idempotency_key = $3
	`, entry.UserID, entry.RouteKey, entry.Key).Scan(&cachedBody); err != nil {
		t.Fatal(err)
	}
	objectKey := "private/" + assetID + ".png"
	for name, payload := range map[string]string{"event": eventPayload, "audit": auditPayload, "idempotency": cachedBody} {
		for _, forbidden := range []string{objectKey, "private/", "objectKey", "contentPath"} {
			if strings.Contains(payload, forbidden) {
				t.Fatalf("%s payload leaked %q: %s", name, forbidden, payload)
			}
		}
	}

	rollbackEntry := beginEvidenceQuarantineIdempotency(t, store, fixture.adminID, now.Add(time.Minute), "quarantine-rollback")
	_, _, appErr = store.QuarantineAssetWithIdempotency(ctx, rollbackEntry, evidence.AdminQuarantineInput{
		AssetID: rollbackAssetID, AdminUserID: fixture.adminID, ExpectedVersion: 1,
		Reason: "用于验证事务回滚。", RequestID: "quarantine-request-rollback",
	}, now.Add(time.Minute), func(evidence.AdminQuarantineResult) (idempotency.Completion, *domain.AppError) {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Completion failed", "完成响应构造失败。")
	})
	if appErr == nil || appErr.Code != domain.CodeInternalError {
		t.Fatalf("expected completion failure, got %#v", appErr)
	}
	readable, readErr := store.AuthorizedAsset(ctx, rollbackAssetID, fixture.adminID, true)
	if readErr != nil || readable.Status != "ready" || readable.Version != 1 {
		t.Fatalf("rollback must leave evidence readable: asset=%+v error=%v", readable, readErr)
	}
	var rollbackAuditCount int
	if err := pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM domain_events WHERE aggregate_type = 'api_order_evidence' AND aggregate_id = $1)
			     + (SELECT count(*) FROM admin_audit_logs WHERE action = 'api_order_evidence.quarantined' AND target_id = $1)
	`, rollbackAssetID).Scan(&rollbackAuditCount); err != nil {
		t.Fatal(err)
	}
	if rollbackAuditCount != 0 {
		t.Fatalf("failed quarantine leaked event or audit rows: %d", rollbackAuditCount)
	}
}

func beginEvidenceQuarantineIdempotency(t *testing.T, store *Store, adminID string, now time.Time, suffix string) idempotency.Entry {
	t.Helper()
	entry := idempotency.Entry{
		UserID: adminID, RouteKey: "/api/v1/admin/dispute-evidence/{id}/quarantine",
		Key: suffix + "-" + uuid.NewString(), RequestHash: "hash-" + suffix,
		State: "processing", CreatedAt: now, ExpiresAt: now.Add(idempotency.ProcessingLifetime),
	}
	created, appErr := store.BeginIdempotency(context.Background(), entry)
	if appErr != nil {
		t.Fatalf("begin quarantine idempotency: %v", appErr)
	}
	return *created
}

func TestPostgresEvidenceCaseLimitSerializesConcurrentBindings(t *testing.T) {
	pool := connectEvidenceIntegrationPool(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	setupTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = setupTx.Rollback(context.Background()) })
	fixture := seedEvidenceIntegrationFixture(t, setupTx, now)
	assetIDs := make([]string, 0, evidence.MaxAssetsPerCase+1)
	for range evidence.MaxAssetsPerCase + 1 {
		assetIDs = append(assetIDs, insertReadyEvidenceAsset(t, setupTx, fixture.orderID, fixture.buyerID, now, now.Add(time.Hour)))
	}
	for start := 0; start < evidence.MaxAssetsPerCase-1; start += evidence.MaxFilesPerUpload {
		end := min(start+evidence.MaxFilesPerUpload, evidence.MaxAssetsPerCase-1)
		if appErr := bindEvidenceAssetsInTx(ctx, setupTx, evidence.BindingInput{
			AssetIDs: assetIDs[start:end], APIOrderID: fixture.orderID, DisputeCaseID: fixture.caseID,
			UploaderID: fixture.buyerID, Visibility: evidence.VisibilityParticipantsAdmin,
			Usage: evidence.UsageDisputeInitial, SourceType: evidence.SourceDisputeCase, SourceID: fixture.caseID,
		}, now); appErr != nil {
			_ = setupTx.Rollback(ctx)
			t.Fatalf("seed evidence boundary: %v", appErr)
		}
	}
	if err := setupTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cleanupEvidenceIntegrationFixture(t, pool, fixture) })

	type bindingResult struct {
		appErr *domain.AppError
		err    error
	}
	results := make(chan bindingResult, 2)
	for _, assetID := range assetIDs[evidence.MaxAssetsPerCase-1:] {
		go func(assetID string) {
			tx, beginErr := pool.Begin(ctx)
			if beginErr != nil {
				results <- bindingResult{err: beginErr}
				return
			}
			appErr := bindEvidenceAssetsInTx(ctx, tx, evidence.BindingInput{
				AssetIDs: []string{assetID}, APIOrderID: fixture.orderID, DisputeCaseID: fixture.caseID,
				UploaderID: fixture.buyerID, Visibility: evidence.VisibilityParticipantsAdmin,
				Usage: evidence.UsageDisputeInitial, SourceType: evidence.SourceDisputeCase, SourceID: fixture.caseID,
			}, now)
			if appErr != nil {
				_ = tx.Rollback(ctx)
				results <- bindingResult{appErr: appErr}
				return
			}
			results <- bindingResult{err: tx.Commit(ctx)}
		}(assetID)
	}

	successes, caseLimitFailures := 0, 0
	for range 2 {
		result := <-results
		if result.err != nil {
			t.Fatalf("concurrent evidence binding: %v", result.err)
		}
		if result.appErr == nil {
			successes++
			continue
		}
		if result.appErr.Status == http.StatusUnprocessableEntity && strings.Contains(result.appErr.Detail, "20") {
			caseLimitFailures++
			continue
		}
		t.Fatalf("unexpected concurrent evidence error: %#v", result.appErr)
	}
	var bindingCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM api_order_evidence_bindings WHERE dispute_case_id = $1`, fixture.caseID).Scan(&bindingCount); err != nil {
		t.Fatal(err)
	}
	if successes != 1 || caseLimitFailures != 1 || bindingCount != evidence.MaxAssetsPerCase {
		t.Fatalf("case limit was not serialized: successes=%d caseLimitFailures=%d bindings=%d", successes, caseLimitFailures, bindingCount)
	}
}

func TestPostgresEvidenceCleanupExactBoundariesAndActiveHolds(t *testing.T) {
	pool := connectEvidenceIntegrationPool(t)
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	fixture := seedEvidenceIntegrationFixture(t, tx, now)

	want := make(map[string]bool)
	labels := make(map[string]string)
	unboundExact := insertReadyEvidenceAsset(t, tx, fixture.orderID, fixture.buyerID, now.Add(-24*time.Hour), now)
	want[unboundExact] = true
	labels[unboundExact] = "unbound_exact"
	unboundBefore := insertReadyEvidenceAsset(t, tx, fixture.orderID, fixture.buyerID, now.Add(-24*time.Hour), now.Add(time.Microsecond))
	labels[unboundBefore] = "unbound_before"
	quarantineExact := insertQuarantinedEvidenceAsset(t, tx, fixture.orderID, fixture.buyerID, now.Add(-7*24*time.Hour), now)
	want[quarantineExact] = true
	labels[quarantineExact] = "quarantine_exact"
	quarantineBefore := insertQuarantinedEvidenceAsset(t, tx, fixture.orderID, fixture.buyerID, now.Add(-7*24*time.Hour), now.Add(time.Microsecond))
	labels[quarantineBefore] = "quarantine_before"

	terminalExactCase := insertEvidenceCase(t, tx, fixture, now.Add(-180*24*time.Hour), false)
	terminalExactAppeal := insertEvidenceAppeal(t, tx, fixture.buyerID, terminalExactCase, fixture.orderID, "approved", now.Add(-90*24*time.Hour))
	if terminalExactAppeal == "" {
		t.Fatal("expected handled appeal")
	}
	terminalExact := insertBoundLifecycleEvidence(t, tx, fixture, terminalExactCase, now)
	want[terminalExact] = true
	labels[terminalExact] = "terminal_exact"

	terminalBeforeCase := insertEvidenceCase(t, tx, fixture, now.Add(-180*24*time.Hour), false)
	insertEvidenceAppeal(t, tx, fixture.buyerID, terminalBeforeCase, fixture.orderID, "approved", now.Add(-90*24*time.Hour).Add(time.Microsecond))
	terminalBefore := insertBoundLifecycleEvidence(t, tx, fixture, terminalBeforeCase, now)
	labels[terminalBefore] = "terminal_before"
	earlierAppealCase := insertEvidenceCase(t, tx, fixture, now.Add(-90*24*time.Hour).Add(time.Microsecond), false)
	insertEvidenceAppeal(t, tx, fixture.buyerID, earlierAppealCase, fixture.orderID, "approved", now.Add(-180*24*time.Hour))
	earlierAppeal := insertBoundLifecycleEvidence(t, tx, fixture, earlierAppealCase, now)
	labels[earlierAppeal] = "terminal_after_case_with_earlier_appeal"

	activeCase := insertEvidenceCase(t, tx, fixture, now.Add(-180*24*time.Hour), true)
	activeAsset := insertBoundLifecycleEvidence(t, tx, fixture, activeCase, now)
	labels[activeAsset] = "active_case"

	remedyCase := insertEvidenceCase(t, tx, fixture, now.Add(-180*24*time.Hour), false)
	if _, err := tx.Exec(ctx, `
		INSERT INTO api_order_dispute_remedies (
			id, dispute_case_id, action, responsible_user_id, beneficiary_user_id,
			instructions, status, due_at, created_by_admin_id, created_at, updated_at
		) VALUES ($1, $2, 'full_refund', $3, $4, 'refund outside platform', 'pending', $5, $6, $7, $7)
	`, uuid.NewString(), remedyCase, fixture.sellerID, fixture.buyerID, now.Add(time.Hour), fixture.adminID, now.Add(-180*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	remedyAsset := insertBoundLifecycleEvidence(t, tx, fixture, remedyCase, now)
	labels[remedyAsset] = "pending_remedy"

	appealCase := insertEvidenceCase(t, tx, fixture, now.Add(-180*24*time.Hour), false)
	insertEvidenceAppeal(t, tx, fixture.buyerID, appealCase, fixture.orderID, "submitted", now.Add(-180*24*time.Hour))
	appealAsset := insertBoundLifecycleEvidence(t, tx, fixture, appealCase, now)
	labels[appealAsset] = "submitted_appeal"

	candidates, appErr := claimEvidenceDestroyCandidatesInTx(ctx, tx, now, 100)
	if appErr != nil {
		t.Fatalf("claim cleanup candidates: %v", appErr)
	}
	got := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		got[candidate.ID] = true
	}
	if len(got) != len(want) {
		gotLabels := make([]string, 0, len(got))
		for assetID := range got {
			gotLabels = append(gotLabels, labels[assetID])
		}
		sort.Strings(gotLabels)
		t.Fatalf("unexpected candidates got=%v want=%v", gotLabels, []string{"quarantine_exact", "terminal_exact", "unbound_exact"})
	}
	for assetID := range want {
		if !got[assetID] {
			t.Fatalf("missing exact-boundary candidate %s: %v", assetID, got)
		}
	}
	if _, appErr := authorizedEvidenceAsset(ctx, tx, terminalExact, fixture.buyerID, false); appErr == nil || appErr.Status != http.StatusNotFound {
		t.Fatalf("terminal evidence remained readable while deletion was pending: %#v", appErr)
	}
}

func connectEvidenceIntegrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := strings.TrimSpace(os.Getenv("C2C_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not configured")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func cleanupEvidenceIntegrationFixture(t *testing.T, pool *pgxpool.Pool, fixture evidenceIntegrationFixture) {
	t.Helper()
	ctx := context.Background()
	_, _ = pool.Exec(ctx, `
		UPDATE api_order_evidence_assets
		SET status = 'destroyed', object_key = NULL,
		    destroy_requested_at = COALESCE(destroy_requested_at, now()),
		    destroyed_at = COALESCE(destroyed_at, now()),
		    destroy_reason = CASE WHEN destroy_reason = '' THEN 'integration_test_cleanup' ELSE destroy_reason END,
		    updated_at = now(), version = version + 1
		WHERE api_order_id IN ($1, $2) AND status <> 'destroyed'
	`, fixture.orderID, fixture.otherOrderID)
	_, _ = pool.Exec(ctx, `DELETE FROM idempotency_keys WHERE user_id = $1`, fixture.adminID)
	_, _ = pool.Exec(ctx, `DELETE FROM admin_audit_logs WHERE admin_user_id = $1`, fixture.adminID)
	_, _ = pool.Exec(ctx, `DELETE FROM domain_events WHERE actor_user_id = $1 OR (aggregate_type = 'api_order_evidence' AND aggregate_id IN (SELECT id FROM api_order_evidence_assets WHERE api_order_id IN ($2, $3)))`, fixture.adminID, fixture.orderID, fixture.otherOrderID)
	deleteEvidenceBindingsForTest(t, pool, `DELETE FROM api_order_evidence_bindings WHERE dispute_case_id IN ($1, $2)`, fixture.caseID, fixture.otherCaseID)
	_, _ = pool.Exec(ctx, `DELETE FROM api_order_evidence_assets WHERE api_order_id IN ($1, $2)`, fixture.orderID, fixture.otherOrderID)
	_, _ = pool.Exec(ctx, `DELETE FROM appeals WHERE dispute_case_id IN ($1, $2)`, fixture.caseID, fixture.otherCaseID)
	_, _ = pool.Exec(ctx, `DELETE FROM moderation_info_supplements WHERE id = $1`, fixture.supplementID)
	_, _ = pool.Exec(ctx, `DELETE FROM moderation_info_requests WHERE dispute_case_id IN ($1, $2)`, fixture.caseID, fixture.otherCaseID)
	_, _ = pool.Exec(ctx, `DELETE FROM dispute_cases WHERE id IN ($1, $2)`, fixture.caseID, fixture.otherCaseID)
	_, _ = pool.Exec(ctx, `DELETE FROM api_orders WHERE id IN ($1, $2)`, fixture.orderID, fixture.otherOrderID)
	_, _ = pool.Exec(ctx, `DELETE FROM api_purchase_intents WHERE buyer_user_id = $1 AND owner_user_id = $2`, fixture.buyerID, fixture.sellerID)
	_, _ = pool.Exec(ctx, `DELETE FROM api_service_access_modes WHERE api_service_id IN (SELECT id FROM api_services WHERE owner_user_id = $1)`, fixture.sellerID)
	_, _ = pool.Exec(ctx, `DELETE FROM api_services WHERE owner_user_id = $1`, fixture.sellerID)
	_, _ = pool.Exec(ctx, `UPDATE contact_methods SET current_version_id = NULL WHERE user_id IN ($1, $2)`, fixture.buyerID, fixture.sellerID)
	_, _ = pool.Exec(ctx, `DELETE FROM contact_method_versions WHERE owner_user_id IN ($1, $2)`, fixture.buyerID, fixture.sellerID)
	_, _ = pool.Exec(ctx, `DELETE FROM contact_methods WHERE user_id IN ($1, $2)`, fixture.buyerID, fixture.sellerID)
	_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id IN ($1, $2, $3, $4)`, fixture.buyerID, fixture.sellerID, fixture.adminID, fixture.outsiderID)
}

func deleteEvidenceBindingsForTest(t *testing.T, pool *pgxpool.Pool, query string, args ...any) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Errorf("begin evidence binding cleanup: %v", err)
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `ALTER TABLE api_order_evidence_bindings DISABLE TRIGGER trg_api_order_evidence_bindings_append_only`); err != nil {
		t.Errorf("disable evidence binding append-only trigger for cleanup: %v", err)
		return
	}
	if _, err := tx.Exec(ctx, query, args...); err != nil {
		t.Errorf("delete evidence binding fixtures: %v", err)
		return
	}
	if _, err := tx.Exec(ctx, `ALTER TABLE api_order_evidence_bindings ENABLE TRIGGER trg_api_order_evidence_bindings_append_only`); err != nil {
		t.Errorf("restore evidence binding append-only trigger after cleanup: %v", err)
		return
	}
	if err := tx.Commit(ctx); err != nil {
		t.Errorf("commit evidence binding cleanup: %v", err)
	}
}

func seedEvidenceIntegrationFixture(t *testing.T, tx pgx.Tx, now time.Time) evidenceIntegrationFixture {
	t.Helper()
	fixture := evidenceIntegrationFixture{
		buyerID: uuid.NewString(), sellerID: uuid.NewString(), adminID: uuid.NewString(), outsiderID: uuid.NewString(),
	}
	for index, userID := range []string{fixture.buyerID, fixture.sellerID, fixture.adminID, fixture.outsiderID} {
		if _, err := tx.Exec(context.Background(), `INSERT INTO users (id, username, display_name, account_status, created_at, updated_at) VALUES ($1, $2, $2, 'active', $3, $3)`, userID, "evidence-user-"+uuid.NewString()[:8]+string(rune('a'+index)), now); err != nil {
			t.Fatal(err)
		}
	}
	sellerContact, sellerVersion := insertEvidenceContact(t, tx, fixture.sellerID, now)
	buyerContact, buyerVersion := insertEvidenceContact(t, tx, fixture.buyerID, now)
	serviceID := uuid.NewString()
	if _, err := tx.Exec(context.Background(), `
		INSERT INTO api_services (
			id, owner_user_id, merchant_identity_mode, owner_contact_method_id,
			title, short_description, distribution_system, billing_mode,
			minimum_intent_cny, usage_visibility, review_status, publication_status,
			moderation_status, accepting_orders, created_at, updated_at
		) VALUES ($1, $2, 'public_profile', $3, 'Evidence service', 'Evidence integration service',
		          'sub2api', 'manual_usage_check', 1, 'none', 'approved', 'online', 'clear', true, $4, $4)
	`, serviceID, fixture.sellerID, sellerContact, now); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(context.Background(), `INSERT INTO api_service_access_modes (api_service_id, access_mode, public_note) VALUES ($1, 'buyer_dedicated_sub_key', 'Buyer-dedicated access')`, serviceID); err != nil {
		t.Fatal(err)
	}
	fixture.orderID = insertEvidenceOrder(t, tx, serviceID, fixture, sellerContact, sellerVersion, buyerContact, buyerVersion, now)
	fixture.otherOrderID = insertEvidenceOrder(t, tx, serviceID, fixture, sellerContact, sellerVersion, buyerContact, buyerVersion, now.Add(time.Second))
	fixture.caseID = insertEvidenceCase(t, tx, fixture, now.Add(-time.Hour), false)
	fixture.otherCaseID = insertEvidenceCaseForOrder(t, tx, fixture, fixture.otherOrderID, now.Add(-time.Hour), false)
	fixture.appealID = insertEvidenceAppeal(t, tx, fixture.buyerID, fixture.caseID, fixture.orderID, "submitted", now)

	requestID := uuid.NewString()
	fixture.supplementID = uuid.NewString()
	if _, err := tx.Exec(context.Background(), `
		INSERT INTO moderation_info_requests (
			id, entity_type, dispute_case_id, requested_from_user_id, requested_by_admin_id,
			internal_reason, status, requested_at, answered_at, created_at
		) VALUES ($1, 'dispute', $2, $3, $4, 'Need evidence', 'answered', $5, $5, $5)
	`, requestID, fixture.caseID, fixture.sellerID, fixture.adminID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(context.Background(), `INSERT INTO moderation_info_supplements (id, info_request_id, submitted_by_user_id, body, created_at) VALUES ($1, $2, $3, 'Supplement evidence', $4)`, fixture.supplementID, requestID, fixture.sellerID, now); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func insertEvidenceContact(t *testing.T, tx pgx.Tx, userID string, now time.Time) (string, string) {
	t.Helper()
	contactID, versionID := uuid.NewString(), uuid.NewString()
	if _, err := tx.Exec(context.Background(), `INSERT INTO contact_methods (id, user_id, type, label, is_default, enabled, created_at, updated_at) VALUES ($1, $2, 'linuxdo', 'linux.do', true, true, $3, $3)`, contactID, userID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(context.Background(), `
		INSERT INTO contact_method_versions (
			id, contact_method_id, owner_user_id, value_ciphertext, value_nonce,
			masked_value, value_fingerprint, encryption_key_version, fingerprint_key_version, created_at
		) VALUES ($1, $2, $3, decode('0102', 'hex'), decode('000000000000000000000000', 'hex'),
		          'linux.do user', $4, 'test-v1', 'test-v1', $5)
	`, versionID, contactID, userID, "fingerprint-"+versionID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(context.Background(), `UPDATE contact_methods SET current_version_id = $2 WHERE id = $1`, contactID, versionID); err != nil {
		t.Fatal(err)
	}
	return contactID, versionID
}

func insertEvidenceOrder(t *testing.T, tx pgx.Tx, serviceID string, fixture evidenceIntegrationFixture, sellerContact, sellerVersion, buyerContact, buyerVersion string, now time.Time) string {
	t.Helper()
	intentID, orderID := uuid.NewString(), uuid.NewString()
	if _, err := tx.Exec(context.Background(), `
		INSERT INTO api_purchase_intents (
			id, api_service_id, api_service_owner_user_id, buyer_user_id, owner_user_id,
			buyer_contact_method_id, buyer_contact_method_version_id,
			owner_contact_method_id, owner_contact_method_version_id,
			status, requested_cny_amount, selected_access_mode, service_version_snapshot,
			service_title_snapshot, distribution_system_snapshot, billing_mode_snapshot,
			buyer_contact_type_snapshot, buyer_contact_label_snapshot,
			owner_contact_type_snapshot, owner_contact_label_snapshot,
			minimum_intent_cny_snapshot, pricing_snapshot, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $3, $5, $6, $7, $8, 'ordered', 20,
		          'buyer_dedicated_sub_key', 1, 'Evidence service', 'sub2api', 'manual_usage_check',
		          'linuxdo', 'linux.do', 'linuxdo', 'linux.do', 1, '{}'::jsonb, $9, $9)
	`, intentID, serviceID, fixture.sellerID, fixture.buyerID, buyerContact, buyerVersion, sellerContact, sellerVersion, now); err != nil {
		t.Fatal(err)
	}
	orderNo, err := apiorder.GenerateOrderNo(now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(context.Background(), `
		INSERT INTO api_orders (
			id, api_purchase_intent_id, api_service_id, buyer_user_id, seller_user_id,
			status, service_title_snapshot, service_version_snapshot, billing_mode_snapshot,
			amount, currency, selected_payment_method, payment_window_minutes_snapshot,
			payment_expires_at, payment_instructions_snapshot, created_at, updated_at, order_no
		) VALUES ($1, $2, $3, $4, $5, 'pending_payment', 'Evidence service', 1,
		          'manual_usage_check', 20, 'CNY', 'wechat', 10, $6, 'Offsite payment', $7, $7, $8)
	`, orderID, intentID, serviceID, fixture.buyerID, fixture.sellerID, now.Add(time.Hour), now, orderNo); err != nil {
		t.Fatal(err)
	}
	return orderID
}

func insertEvidenceCase(t *testing.T, tx pgx.Tx, fixture evidenceIntegrationFixture, anchor time.Time, active bool) string {
	t.Helper()
	return insertEvidenceCaseForOrder(t, tx, fixture, fixture.orderID, anchor, active)
}

func insertEvidenceCaseForOrder(t *testing.T, tx pgx.Tx, fixture evidenceIntegrationFixture, orderID string, anchor time.Time, active bool) string {
	t.Helper()
	caseID := uuid.NewString()
	status := "closed"
	var finalReason any = "evidence_test_closed"
	var appealExpiresAt any = anchor.Add(30 * 24 * time.Hour)
	var affected any = []string{fixture.buyerID}
	var closedAt any = anchor
	if active {
		status = "open"
		finalReason, appealExpiresAt, affected, closedAt = "", nil, []string{}, nil
	}
	if _, err := tx.Exec(context.Background(), `
		INSERT INTO dispute_cases (
			id, target_type, target_id, api_order_id, active, target_label,
			primary_user_id, counterparty_user_id, subject_user_id, status,
			public_summary, public_result, opened_by_admin_id, opened_at, closed_at,
			final_reason, appeal_expires_at, adversely_affected_user_ids,
			created_at, updated_at
		) VALUES ($1, 'api_order', $2::uuid::text, $2::uuid, $3, 'Evidence order', $4, $5, $4, $6,
		          'Evidence dispute', 'Evidence result', $7, $8, $9, $10, $11, $12, $8, $8)
	`, caseID, orderID, active, fixture.buyerID, fixture.sellerID, status, fixture.adminID, anchor, closedAt, finalReason, appealExpiresAt, affected); err != nil {
		t.Fatal(err)
	}
	return caseID
}

func insertEvidenceAppeal(t *testing.T, tx pgx.Tx, appellantID, caseID, orderID, status string, at time.Time) string {
	t.Helper()
	appealID := uuid.NewString()
	var handledBy, handledAt any
	if status != "submitted" {
		handledBy = appellantID
		handledAt = at
	}
	if _, err := tx.Exec(context.Background(), `
		INSERT INTO appeals (
			id, appellant_user_id, dispute_case_id, target_type, target_id,
			title, statement, status, handled_by_admin_id, handled_at, created_at, updated_at
		) VALUES ($1, $2, $3, 'api_order', $4, 'Evidence appeal', 'Evidence appeal statement',
		          $5, $6, $7, $8, $8)
	`, appealID, appellantID, caseID, orderID, status, handledBy, handledAt, at); err != nil {
		t.Fatal(err)
	}
	return appealID
}

func insertReadyEvidenceAsset(t *testing.T, tx pgx.Tx, orderID, uploaderID string, createdAt, expiresAt time.Time) string {
	t.Helper()
	assetID := uuid.NewString()
	if _, err := tx.Exec(context.Background(), `
		INSERT INTO api_order_evidence_assets (
			id, api_order_id, uploader_user_id, kind, object_key, output_mime,
			byte_size, width, height, sha256, scan_status, status,
			ready_at, unbound_expires_at, created_at, updated_at
		) VALUES ($1, $2, $3, 'api_error', $4, 'image/png', 1, 1, 1, $5,
		          'passed', 'ready', $6, $7, $6, $6)
	`, assetID, orderID, uploaderID, "private/"+assetID+".png", make([]byte, 32), createdAt, expiresAt); err != nil {
		t.Fatal(err)
	}
	return assetID
}

func insertQuarantinedEvidenceAsset(t *testing.T, tx pgx.Tx, orderID, uploaderID string, createdAt, expiresAt time.Time) string {
	t.Helper()
	assetID := uuid.NewString()
	if _, err := tx.Exec(context.Background(), `
		INSERT INTO api_order_evidence_assets (
			id, api_order_id, uploader_user_id, kind, object_key, output_mime,
			byte_size, width, height, sha256, scan_status, status,
			quarantined_expires_at, created_at, updated_at
		) VALUES ($1, $2, $3, 'api_error', $4, 'image/png', 1, 1, 1, $5,
		          'rejected', 'quarantined', $6, $7, $7)
	`, assetID, orderID, uploaderID, "private/"+assetID+".png", make([]byte, 32), expiresAt, createdAt); err != nil {
		t.Fatal(err)
	}
	return assetID
}

func insertBoundLifecycleEvidence(t *testing.T, tx pgx.Tx, fixture evidenceIntegrationFixture, caseID string, now time.Time) string {
	t.Helper()
	assetID := insertReadyEvidenceAsset(t, tx, fixture.orderID, fixture.buyerID, now.Add(-180*24*time.Hour), now.Add(time.Hour))
	if appErr := bindEvidenceAssetsInTx(context.Background(), tx, evidence.BindingInput{
		AssetIDs: []string{assetID}, APIOrderID: fixture.orderID, DisputeCaseID: caseID,
		UploaderID: fixture.buyerID, Visibility: evidence.VisibilityParticipantsAdmin,
		Usage: evidence.UsageDisputeInitial, SourceType: evidence.SourceDisputeCase, SourceID: caseID,
	}, now.Add(-180*24*time.Hour)); appErr != nil {
		t.Fatalf("bind lifecycle evidence: %v", appErr)
	}
	return assetID
}

func assertEvidenceBindConflict(t *testing.T, appErr *domain.AppError) {
	t.Helper()
	if appErr == nil || appErr.Status != http.StatusConflict || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("expected evidence bind conflict, got %#v", appErr)
	}
}
