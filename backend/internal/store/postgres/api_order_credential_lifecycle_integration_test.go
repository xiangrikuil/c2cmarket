package postgres

import (
	"context"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/report"

	"github.com/google/uuid"
)

func TestPostgresUppercaseAPIOrderModerationSharesCredentialLifecycleLock(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	completedAt := time.Date(2020, 1, 2, 12, 0, 0, 0, time.UTC)
	runAt := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)

	sellerID := uuid.NewString()
	sellerContactID := uuid.NewString()
	buyerID := uuid.NewString()
	buyerContactID := uuid.NewString()
	serviceID := uuid.NewString()
	t.Cleanup(func() {
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM appeals WHERE appellant_user_id = $1`, sellerID)
		cleanupLifecycleCredentialFixtures(t, context.Background(), store, sellerID, buyerID, "")
	})
	seedQuotaServiceForTest(t, ctx, store.pool, sellerID, sellerContactID, buyerID, buyerContactID, serviceID, completedAt)
	fixture := insertLifecycleCompletedCredentialOrder(
		t, store, serviceID, sellerID, sellerContactID, buyerID, buyerContactID,
		completedAt, completedAt.Add(-time.Hour), "", nil,
	)

	reportTx, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin uppercase API order report: %v", err)
	}
	createdReport, appErr := createReportInTx(ctx, reportTx, report.CreateReportInput{
		ReporterUserID: buyerID,
		TargetType:     report.TargetAPIOrder,
		TargetID:       strings.ToUpper(fixture.OrderID),
		ReasonCode:     "order_delivery_dispute",
		Title:          "Uppercase order report",
		Description:    "The canonical order identifier must own lifecycle coordination.",
	}, runAt)
	if appErr != nil {
		_ = reportTx.Rollback(ctx)
		t.Fatalf("create uppercase API order report: %v", appErr)
	}
	if err := reportTx.Commit(ctx); err != nil {
		t.Fatalf("commit uppercase API order report: %v", err)
	}
	if createdReport.CanonicalTargetID != fixture.OrderID {
		t.Fatalf("report target was not canonicalized by PostgreSQL: got %q want %q", createdReport.CanonicalTargetID, fixture.OrderID)
	}

	disputeTx, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin uppercase-origin report dispute: %v", err)
	}
	disputeResult, disputeErr := updateReportAdminInTx(ctx, disputeTx, report.AdminActionInput{
		ID:               createdReport.ID,
		AdminUserID:      sellerID,
		Action:           "open_dispute",
		Reason:           "Open uppercase-origin lifecycle dispute",
		PublicSummary:    "Uppercase-origin lifecycle dispute",
		PublicResultCode: report.PublicResultNoAction,
		PublicResult:     "Reviewing",
		ExpectedVersion:  1,
		RequestID:        "uppercase-origin-dispute",
	}, runAt)
	if disputeErr != nil || disputeResult.Dispute == nil {
		_ = disputeTx.Rollback(ctx)
		t.Fatalf("open uppercase-origin report dispute: result=%+v err=%v", disputeResult, disputeErr)
	}
	assertCredentialLifecycleMaintenanceBlocked(t, store, fixture.CredentialID, runAt, "dispute lock")
	if err := disputeTx.Commit(ctx); err != nil {
		t.Fatalf("commit uppercase-origin report dispute: %v", err)
	}
	assertLifecycleDisputeTarget(t, store, disputeResult.Dispute.ID, fixture.OrderID)
	assertCredentialLifecycleMaintenanceBlocked(t, store, fixture.CredentialID, runAt, "committed dispute hold")

	if _, err := store.pool.Exec(ctx, `
		UPDATE dispute_cases
		SET status = 'resolved', resolved_at = $2, updated_at = $2
		WHERE id = $1
	`, disputeResult.Dispute.ID, runAt); err != nil {
		t.Fatalf("resolve uppercase-origin lifecycle dispute: %v", err)
	}

	appealTx, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin uppercase-origin appeal: %v", err)
	}
	createdAppeal, appealErr := createAppealInTx(ctx, appealTx, report.CreateAppealInput{
		AppellantUserID: sellerID,
		DisputeID:       disputeResult.Dispute.ID,
		Title:           "Uppercase-origin appeal",
		Statement:       "The appeal must share the canonical lifecycle lock.",
	}, runAt)
	if appealErr != nil {
		_ = appealTx.Rollback(ctx)
		t.Fatalf("create uppercase-origin appeal: %v", appealErr)
	}
	assertCredentialLifecycleMaintenanceBlocked(t, store, fixture.CredentialID, runAt, "appeal lock")
	if err := appealTx.Commit(ctx); err != nil {
		t.Fatalf("commit uppercase-origin appeal: %v", err)
	}
	if createdAppeal.TargetID != fixture.OrderID {
		t.Fatalf("appeal target was not canonicalized through its dispute: got %q want %q", createdAppeal.TargetID, fixture.OrderID)
	}
	assertCredentialLifecycleMaintenanceBlocked(t, store, fixture.CredentialID, runAt, "committed appeal hold")
}

func assertCredentialLifecycleMaintenanceBlocked(t *testing.T, store *Store, credentialID string, runAt time.Time, source string) {
	t.Helper()
	orderCount, quotaCount := runCredentialDestructionBatchForTest(t, store, runAt, 10)
	if orderCount != 0 || quotaCount != 0 {
		t.Fatalf("maintenance bypassed canonical %s: order=%d quota=%d", source, orderCount, quotaCount)
	}
	assertLifecycleOrderCredentialState(t, store, credentialID, false, "")
}

func assertLifecycleDisputeTarget(t *testing.T, store *Store, disputeID, orderID string) {
	t.Helper()
	var targetID string
	if err := store.pool.QueryRow(context.Background(), `SELECT target_id FROM dispute_cases WHERE id = $1`, disputeID).Scan(&targetID); err != nil {
		t.Fatalf("read lifecycle dispute target: %v", err)
	}
	if targetID != orderID {
		t.Fatalf("dispute target was not canonicalized: got %q want %q", targetID, orderID)
	}
}

func TestPostgresAPIOrderCredentialReadSerializesWithDestruction(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	completedAt := time.Date(2020, 1, 2, 12, 0, 0, 0, time.UTC)
	runAt := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)

	sellerID := uuid.NewString()
	sellerContactID := uuid.NewString()
	buyerID := uuid.NewString()
	buyerContactID := uuid.NewString()
	serviceID := uuid.NewString()
	t.Cleanup(func() {
		cleanupLifecycleCredentialFixtures(t, context.Background(), store, sellerID, buyerID, "")
	})
	seedQuotaServiceForTest(t, ctx, store.pool, sellerID, sellerContactID, buyerID, buyerContactID, serviceID, completedAt)
	fixture := insertLifecycleCompletedCredentialOrder(
		t,
		store,
		serviceID,
		sellerID,
		sellerContactID,
		buyerID,
		buyerContactID,
		completedAt,
		completedAt.Add(-time.Hour),
		"",
		nil,
	)

	const liveSecret = "lifecycle-read-secret"
	encoded, err := store.contactCodec.encode(liveSecret, fixture.CredentialID, contactFieldOrderAPIKey)
	if err != nil {
		t.Fatalf("encode lifecycle read credential: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		UPDATE api_order_delivery_credentials
		SET api_key_ciphertext = $2,
		    api_key_nonce = $3,
		    secret_encryption_key_version = $4,
		    secret_encryption_format = $5
		WHERE id = $1
	`, fixture.CredentialID, encoded.Ciphertext, encoded.Nonce, encoded.EncryptionKeyVersion, encoded.CipherFormat); err != nil {
		t.Fatalf("replace lifecycle read credential ciphertext: %v", err)
	}

	readTx, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin credential detail read: %v", err)
	}
	defer func() { _ = readTx.Rollback(context.Background()) }()
	if err := lockAPIOrderCredentialLifecycleInTx(ctx, readTx, fixture.OrderID); err != nil {
		t.Fatalf("lock credential lifecycle for detail read: %v", err)
	}
	liveOrder, err := store.getAPIOrder(ctx, readTx, fixture.OrderID, false, true)
	if err != nil {
		t.Fatalf("read live credential detail: %v", err)
	}
	if liveOrder.DeliveryCredential == nil || liveOrder.DeliveryCredential.APIKey != liveSecret {
		t.Fatalf("live detail omitted credential before retention destruction: %+v", liveOrder.DeliveryCredential)
	}

	firstOrderCount, firstQuotaCount := runCredentialDestructionBatchForTest(t, store, runAt, 1)
	if firstOrderCount != 0 || firstQuotaCount != 0 {
		t.Fatalf("lifecycle did not skip credential behind detail lock: order=%d quota=%d", firstOrderCount, firstQuotaCount)
	}
	assertLifecycleOrderCredentialState(t, store, fixture.CredentialID, false, "")

	if err := readTx.Commit(ctx); err != nil {
		t.Fatalf("commit credential detail read: %v", err)
	}

	// Simulate a destruction transaction after its irreversible update but before
	// commit. The production buyer/seller entry points must wait for the shared
	// lifecycle advisory lock instead of decrypting their older MVCC snapshots.
	destructionTx, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin pending credential destruction: %v", err)
	}
	defer func() { _ = destructionTx.Rollback(context.Background()) }()
	if err := lockAPIOrderCredentialLifecycleInTx(ctx, destructionTx, fixture.OrderID); err != nil {
		t.Fatalf("lock pending credential destruction: %v", err)
	}
	if _, err := destructionTx.Exec(ctx, `
		UPDATE api_order_delivery_credentials
		SET api_base_url = NULL,
		    panel_login_url = NULL,
		    username = NULL,
		    instructions = NULL,
		    api_key_ciphertext = NULL,
		    api_key_nonce = NULL,
		    password_ciphertext = NULL,
		    password_nonce = NULL,
		    destroyed_at = $2,
		    destroy_reason = 'retention_expired'
		WHERE id = $1
	`, fixture.CredentialID, runAt); err != nil {
		t.Fatalf("stage pending credential destruction: %v", err)
	}

	statsBeforeDestroyedReads := store.ContactCryptoStats()
	readResults := make(chan credentialReadOutcome, 2)
	go func() {
		order, appErr := store.GetAPIOrderForBuyer(context.Background(), buyerID, fixture.OrderID, runAt)
		readResults <- credentialReadOutcome{role: "buyer", order: order, appErr: appErr}
	}()
	go func() {
		order, appErr := store.GetAPIOrderForSeller(context.Background(), sellerID, fixture.OrderID, runAt)
		readResults <- credentialReadOutcome{role: "seller", order: order, appErr: appErr}
	}()
	waitForCredentialLifecycleAdvisoryWaiters(t, store, 2, readResults)
	if err := destructionTx.Commit(ctx); err != nil {
		t.Fatalf("commit pending credential destruction: %v", err)
	}

	for range 2 {
		select {
		case result := <-readResults:
			assertDestroyedCredentialRead(t, result)
		case <-time.After(5 * time.Second):
			t.Fatal("credential detail read did not resume after destruction commit")
		}
	}
	if statsAfterDestroyedReads := store.ContactCryptoStats(); statsAfterDestroyedReads != statsBeforeDestroyedReads {
		t.Fatalf("destroyed detail attempted decryption: before=%+v after=%+v", statsBeforeDestroyedReads, statsAfterDestroyedReads)
	}
	assertLifecycleOrderCredentialState(t, store, fixture.CredentialID, true, "retention_expired")
}

type credentialReadOutcome struct {
	role   string
	order  apiorder.Order
	appErr *domain.AppError
}

func waitForCredentialLifecycleAdvisoryWaiters(t *testing.T, store *Store, minimum int, results <-chan credentialReadOutcome) {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiters int
		if err := store.pool.QueryRow(context.Background(), `
			SELECT count(*)
			FROM pg_stat_activity
			WHERE datname = current_database()
			  AND pid <> pg_backend_pid()
			  AND wait_event_type = 'Lock'
			  AND wait_event = 'advisory'
			  AND position('hashtextextended' IN query) > 0
		`).Scan(&waiters); err != nil {
			t.Fatalf("inspect credential lifecycle advisory waiters: %v", err)
		}
		if waiters >= minimum {
			return
		}
		select {
		case result := <-results:
			t.Fatalf("%s credential detail returned before destruction commit: order=%+v err=%v", result.role, result.order, result.appErr)
		case <-deadline.C:
			t.Fatalf("credential detail did not reach lifecycle advisory wait: got %d waiter(s), want %d", waiters, minimum)
		case <-ticker.C:
		}
	}
}

func assertDestroyedCredentialRead(t *testing.T, result credentialReadOutcome) {
	t.Helper()
	if result.appErr != nil {
		t.Fatalf("%s read destroyed credential detail: %v", result.role, result.appErr)
	}
	credential := result.order.DeliveryCredential
	if credential == nil || credential.DestroyedAt == nil || credential.DestroyReason != "retention_expired" {
		t.Fatalf("%s destroyed detail omitted audit projection: %+v", result.role, credential)
	}
	if credential.APIBaseURL != "" || credential.PanelLoginURL != "" || credential.Username != "" || credential.Instructions != "" || credential.APIKey != "" || credential.Password != "" {
		t.Fatalf("%s destroyed detail retained credential payload: %+v", result.role, credential)
	}
}

func runCredentialDestructionBatchForTest(t *testing.T, store *Store, now time.Time, batchSize int) (int64, int64) {
	t.Helper()
	ctx := context.Background()
	tx, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin credential destruction batch: %v", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	orderCount, quotaCount, err := destroyCompletedAPIOrderCredentialsInTx(ctx, tx, now, now.Add(-30*24*time.Hour), batchSize)
	if err != nil {
		t.Fatalf("destroy credential batch: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit credential destruction batch: %v", err)
	}
	return orderCount, quotaCount
}
