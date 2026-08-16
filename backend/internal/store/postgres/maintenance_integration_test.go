package postgres

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/maintenance"
	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/report"

	"github.com/google/uuid"
)

func TestPostgresDataLifecycleAppliesRetentionAndPreservesAuditHistory(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)

	buyer, appErr := store.EnsureUser(ctx, "lifecycle-"+strings.ToLower(uuid.NewString()[:20]), false, now)
	if appErr != nil {
		t.Fatalf("ensure lifecycle buyer: %v", appErr)
	}
	seller, appErr := store.EnsureUser(ctx, "lifecycle-"+strings.ToLower(uuid.NewString()[:20]), false, now)
	if appErr != nil {
		t.Fatalf("ensure lifecycle seller: %v", appErr)
	}

	sessionExpired := uuid.NewString()
	sessionActive := uuid.NewString()
	sessionRecentlyRevoked := uuid.NewString()
	verificationExpired := uuid.NewString()
	passwordResetExpired := uuid.NewString()
	verificationActive := uuid.NewString()
	verificationRecentlyConsumed := uuid.NewString()
	contactSessionID := uuid.NewString()
	accessLogID := uuid.NewString()
	adminAuditID := uuid.NewString()
	moderationAuditID := uuid.NewString()
	oldReadEventID := uuid.NewString()
	oldUnreadEventID := uuid.NewString()
	oldReferencedEventID := uuid.NewString()
	oldReadNotificationID := uuid.NewString()
	oldUnreadNotificationID := uuid.NewString()
	recentNotificationID := uuid.NewString()
	idempotencyIDs := []string{uuid.NewString(), uuid.NewString(), uuid.NewString()}

	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM contact_access_logs WHERE id = $1`, accessLogID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM contact_sessions WHERE id = $1`, contactSessionID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM notifications WHERE id = ANY($1::uuid[])`, []string{oldReadNotificationID, oldUnreadNotificationID, recentNotificationID})
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM domain_events WHERE id = ANY($1::uuid[])`, []string{oldReadEventID, oldUnreadEventID, oldReferencedEventID})
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM idempotency_keys WHERE id = ANY($1::uuid[])`, idempotencyIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM email_verification_codes WHERE id = ANY($1::uuid[])`, []string{verificationExpired, passwordResetExpired, verificationActive, verificationRecentlyConsumed})
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM auth_sessions WHERE id = ANY($1::uuid[])`, []string{sessionExpired, sessionActive, sessionRecentlyRevoked})
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM moderation_audit_logs WHERE id = $1`, moderationAuditID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM admin_audit_logs WHERE id = $1`, adminAuditID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = ANY($1::uuid[])`, []string{buyer.ID, seller.ID})
	})

	insertLifecycleSession(t, store, sessionExpired, buyer.ID, "expired-"+uuid.NewString(), now.Add(-20*24*time.Hour), now.Add(-8*24*time.Hour), nil)
	insertLifecycleSession(t, store, sessionActive, buyer.ID, "active-"+uuid.NewString(), now.Add(-time.Hour), now.Add(24*time.Hour), nil)
	recentlyRevokedAt := now.Add(-24 * time.Hour)
	insertLifecycleSession(t, store, sessionRecentlyRevoked, buyer.ID, "revoked-"+uuid.NewString(), now.Add(-20*24*time.Hour), now.Add(-8*24*time.Hour), &recentlyRevokedAt)

	insertLifecycleVerification(t, store, verificationExpired, seller.ID, "expired@example.com", now.Add(-72*time.Hour), now.Add(-48*time.Hour), nil)
	insertLifecycleVerificationForPurpose(t, store, passwordResetExpired, seller.ID, "reset-expired@example.com", "password_reset", now.Add(-72*time.Hour), now.Add(-48*time.Hour), nil)
	insertLifecycleVerification(t, store, verificationActive, buyer.ID, "active@example.com", now.Add(-time.Hour), now.Add(time.Hour), nil)
	recentlyConsumedAt := now.Add(-time.Hour)
	insertLifecycleVerification(t, store, verificationRecentlyConsumed, buyer.ID, "consumed@example.com", now.Add(-72*time.Hour), now.Add(-48*time.Hour), &recentlyConsumedAt)

	for index, id := range idempotencyIDs {
		createdAt := now.Add(-time.Duration(index+3) * time.Hour)
		expiresAt := now.Add(-time.Duration(index+1) * time.Hour)
		if _, err := store.pool.Exec(ctx, `
			INSERT INTO idempotency_keys (
				id, user_id, route_key, idempotency_key, request_hash, status,
				response_body_cache_allowed, created_at, expires_at
			)
			VALUES ($1, $2, $3, $4, $5, 'processing', false, $6, $7)
		`, id, buyer.ID, "POST /lifecycle", "key-"+id, "hash-"+id, createdAt, expiresAt); err != nil {
			t.Fatalf("insert expired idempotency row %d: %v", index, err)
		}
	}

	if _, err := store.pool.Exec(ctx, `
		INSERT INTO contact_sessions (id, buyer_user_id, seller_user_id, opens_at, ends_at, status, created_at)
		VALUES ($1, $2, $3, $4, $5, 'open', $4)
	`, contactSessionID, buyer.ID, seller.ID, now.Add(-2*time.Hour), now.Add(-time.Hour)); err != nil {
		t.Fatalf("insert expired contact session: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO contact_access_logs (id, contact_session_id, viewer_user_id, accessed_at, request_id)
		VALUES ($1, $2, $3, $4, $5)
	`, accessLogID, contactSessionID, buyer.ID, now.Add(-90*time.Minute), "lifecycle-access-"+accessLogID); err != nil {
		t.Fatalf("insert contact access log: %v", err)
	}

	insertLifecycleEvent(t, store, oldReadEventID, buyer.ID, now.Add(-400*24*time.Hour))
	insertLifecycleEvent(t, store, oldUnreadEventID, buyer.ID, now.Add(-399*24*time.Hour))
	insertLifecycleEvent(t, store, oldReferencedEventID, buyer.ID, now.Add(-398*24*time.Hour))
	readAt := now.Add(-99 * 24 * time.Hour)
	insertLifecycleNotification(t, store, oldReadNotificationID, buyer.ID, oldReadEventID, now.Add(-100*24*time.Hour), &readAt)
	insertLifecycleNotification(t, store, oldUnreadNotificationID, buyer.ID, oldUnreadEventID, now.Add(-366*24*time.Hour), nil)
	insertLifecycleNotification(t, store, recentNotificationID, buyer.ID, oldReferencedEventID, now.Add(-24*time.Hour), nil)

	if _, err := store.pool.Exec(ctx, `
		INSERT INTO admin_audit_logs (
			id, admin_user_id, action, target_type, target_id, request_id, created_at
		)
		VALUES ($1, $2, 'lifecycle_test', 'user', $3, $4, $5)
	`, adminAuditID, buyer.ID, seller.ID, "lifecycle-admin-"+adminAuditID, now.Add(-500*24*time.Hour)); err != nil {
		t.Fatalf("insert admin audit log: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO moderation_audit_logs (
			id, actor_admin_id, action, object_type, object_id, request_id, created_at
		)
		VALUES ($1, $2, 'triage', 'report', $3, $4, $5)
	`, moderationAuditID, buyer.ID, uuid.NewString(), "lifecycle-moderation-"+moderationAuditID, now.Add(-500*24*time.Hour)); err != nil {
		t.Fatalf("insert moderation audit log: %v", err)
	}

	policy := maintenance.Policy{
		SessionRetention:               7 * 24 * time.Hour,
		EmailVerificationRetention:     24 * time.Hour,
		ReadNotificationRetention:      90 * 24 * time.Hour,
		UnreadNotificationRetention:    365 * 24 * time.Hour,
		DomainEventRetention:           365 * 24 * time.Hour,
		APIDeliveryCredentialRetention: 30 * 24 * time.Hour,
		APIProbeSampleRetention:        7 * 24 * time.Hour,
	}
	result, appErr := store.RunDataLifecycle(ctx, now, 2, policy)
	if appErr != nil {
		t.Fatalf("run lifecycle maintenance: %v", appErr)
	}
	if !result.LockAcquired ||
		result.SessionsDeleted != 1 ||
		result.VerificationCodesDeleted != 2 ||
		result.IdempotencyEntriesDeleted != 2 ||
		result.ContactSessionsExpired != 1 ||
		result.NotificationsDeleted != 2 ||
		result.DomainEventsDeleted != 2 {
		t.Fatalf("unexpected lifecycle result: %+v", result)
	}

	assertLifecycleRowMissing(t, store, "auth_sessions", sessionExpired)
	assertLifecycleRowExists(t, store, "auth_sessions", sessionActive)
	assertLifecycleRowExists(t, store, "auth_sessions", sessionRecentlyRevoked)
	assertLifecycleRowMissing(t, store, "email_verification_codes", verificationExpired)
	assertLifecycleRowMissing(t, store, "email_verification_codes", passwordResetExpired)
	assertLifecycleRowExists(t, store, "email_verification_codes", verificationActive)
	assertLifecycleRowExists(t, store, "email_verification_codes", verificationRecentlyConsumed)
	assertLifecycleRowExists(t, store, "contact_access_logs", accessLogID)
	assertLifecycleRowExists(t, store, "admin_audit_logs", adminAuditID)
	assertLifecycleRowExists(t, store, "moderation_audit_logs", moderationAuditID)
	assertLifecycleRowExists(t, store, "domain_events", oldReferencedEventID)

	var contactStatus string
	if err := store.pool.QueryRow(ctx, `SELECT status FROM contact_sessions WHERE id = $1`, contactSessionID).Scan(&contactStatus); err != nil {
		t.Fatalf("read contact session status: %v", err)
	}
	if contactStatus != "expired" {
		t.Fatalf("expected contact session to expire, got %q", contactStatus)
	}

	second, appErr := store.RunDataLifecycle(ctx, now, 2, policy)
	if appErr != nil {
		t.Fatalf("run second lifecycle maintenance: %v", appErr)
	}
	if second.IdempotencyEntriesDeleted != 1 {
		t.Fatalf("expected bounded second idempotency batch, got %+v", second)
	}
	for _, id := range idempotencyIDs {
		assertLifecycleRowMissing(t, store, "idempotency_keys", id)
	}
}

func TestPostgresDataLifecycleSkipsWhenAdvisoryLockIsHeld(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	connection, err := store.pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire lock connection: %v", err)
	}
	defer connection.Release()
	tx, err := connection.Begin(ctx)
	if err != nil {
		t.Fatalf("begin lock transaction: %v", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, dataLifecycleAdvisoryLockID); err != nil {
		t.Fatalf("hold lifecycle advisory lock: %v", err)
	}

	result, appErr := store.RunDataLifecycle(ctx, time.Now(), 1, maintenance.Policy{
		SessionRetention:               time.Hour,
		EmailVerificationRetention:     time.Hour,
		ReadNotificationRetention:      time.Hour,
		UnreadNotificationRetention:    2 * time.Hour,
		DomainEventRetention:           time.Hour,
		APIDeliveryCredentialRetention: time.Hour,
		APIProbeSampleRetention:        time.Hour,
	})
	if appErr != nil {
		t.Fatalf("run lifecycle while locked: %v", appErr)
	}
	if result.LockAcquired {
		t.Fatalf("maintenance acquired an already-held advisory lock: %+v", result)
	}
}

func TestPostgresDataLifecycleClosesExpiredRemedyConfirmationNeutrally(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 9, 18, 0, 0, 0, time.UTC)
	sellerID := uuid.NewString()
	sellerContactID := uuid.NewString()
	buyerID := uuid.NewString()
	buyerContactID := uuid.NewString()
	serviceID := uuid.NewString()
	seedQuotaServiceForTest(t, ctx, store.pool, sellerID, sellerContactID, buyerID, buyerContactID, serviceID, now.Add(-24*time.Hour))
	order := insertLifecycleCompletedCredentialOrder(t, store, serviceID, sellerID, sellerContactID, buyerID, buyerContactID, now.Add(-2*time.Hour), now.Add(-3*time.Hour), "", nil)
	disputeID := insertLifecycleDispute(t, store, order.OrderID, buyerID, sellerID, report.DisputeStatusResolved, now.Add(-72*time.Hour))
	remedyID := uuid.NewString()
	t.Cleanup(func() {
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM notifications WHERE target_id = $1`, disputeID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM dispute_events WHERE entity_id = $1`, disputeID)
		cleanupLifecycleCredentialFixtures(t, context.Background(), store, sellerID, buyerID, "")
	})
	if _, err := store.pool.Exec(ctx, `
		UPDATE api_orders
		SET dispute_status = 'fulfillment_confirmation', dispute_case_id = $2, updated_at = $3
		WHERE id = $1
	`, order.OrderID, disputeID, now.Add(-49*time.Hour)); err != nil {
		t.Fatalf("attach lifecycle remedy dispute: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO api_order_dispute_remedies (
			id, dispute_case_id, action, responsible_user_id, beneficiary_user_id,
			instructions, status, due_at, claimed_at, confirmation_due_at,
			claim_note, created_by_admin_id, created_request_id, claim_request_id,
			created_at, updated_at, version
		)
		VALUES ($1, $2, 'continue_fulfillment', $3, $4,
		        '请继续完成订单交付。', 'claimed_fulfilled', $5, $6, $7,
		        '已声明继续履行。', $3, $8, $9, $10, $6, 2)
	`, remedyID, disputeID, sellerID, buyerID, now.Add(24*time.Hour), now.Add(-49*time.Hour), now,
		"remedy-created-"+remedyID, "remedy-claimed-"+remedyID, now.Add(-72*time.Hour)); err != nil {
		t.Fatalf("seed claimed remedy: %v", err)
	}

	result, appErr := store.RunDataLifecycle(ctx, now, 10, lifecycleCredentialPolicy())
	if appErr != nil {
		t.Fatalf("run remedy confirmation lifecycle: %v", appErr)
	}
	if result.DisputeRemedyConfirmationsExpired != 1 {
		t.Fatalf("expected one expired remedy confirmation, got %+v", result)
	}
	var remedyStatus, responseNote, disputeStatus, publicResult, orderDisputeStatus string
	if err := store.pool.QueryRow(ctx, `SELECT status, response_note FROM api_order_dispute_remedies WHERE id = $1`, remedyID).Scan(&remedyStatus, &responseNote); err != nil {
		t.Fatalf("read expired remedy: %v", err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT status, public_result FROM dispute_cases WHERE id = $1`, disputeID).Scan(&disputeStatus, &publicResult); err != nil {
		t.Fatalf("read closed remedy dispute: %v", err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT dispute_status FROM api_orders WHERE id = $1`, order.OrderID).Scan(&orderDisputeStatus); err != nil {
		t.Fatalf("read closed order projection: %v", err)
	}
	if remedyStatus != report.RemedyStatusConfirmationExpired || responseNote != report.RemedyConfirmationExpiredNote ||
		disputeStatus != report.DisputeStatusClosed || publicResult != report.RemedyConfirmationExpiredPublicResult ||
		orderDisputeStatus != apiorder.DisputeStatusNone {
		t.Fatalf("unexpected neutral timeout state remedy=%q note=%q dispute=%q result=%q order=%q", remedyStatus, responseNote, disputeStatus, publicResult, orderDisputeStatus)
	}
	var notificationCount int
	if err := store.pool.QueryRow(ctx, `
		SELECT COUNT(*)::int
		FROM notifications
		WHERE target_id = $1 AND source_event_type = 'dispute.remedy_confirmation_expired'
	`, disputeID).Scan(&notificationCount); err != nil {
		t.Fatalf("count remedy timeout notifications: %v", err)
	}
	if notificationCount != 2 {
		t.Fatalf("expected both participants to receive neutral timeout notification, got %d", notificationCount)
	}

	second, appErr := store.RunDataLifecycle(ctx, now, 10, lifecycleCredentialPolicy())
	if appErr != nil || second.DisputeRemedyConfirmationsExpired != 0 {
		t.Fatalf("remedy timeout rerun must be idempotent: result=%+v err=%v", second, appErr)
	}
}

func TestPostgresRemedyLatenessDecisionSurvivesLateFulfillmentClaim(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
	sellerID := uuid.NewString()
	sellerContactID := uuid.NewString()
	buyerID := uuid.NewString()
	buyerContactID := uuid.NewString()
	serviceID := uuid.NewString()
	seedQuotaServiceForTest(t, ctx, store.pool, sellerID, sellerContactID, buyerID, buyerContactID, serviceID, now.Add(-24*time.Hour))
	order := insertLifecycleCompletedCredentialOrder(t, store, serviceID, sellerID, sellerContactID, buyerID, buyerContactID, now.Add(-2*time.Hour), now.Add(-3*time.Hour), "", nil)
	disputeID := insertLifecycleDispute(t, store, order.OrderID, buyerID, sellerID, report.DisputeStatusResolved, now.Add(-24*time.Hour))
	remedyID := uuid.NewString()
	t.Cleanup(func() {
		_, _ = store.pool.Exec(context.Background(), `
			UPDATE api_orders
			SET dispute_status = 'none', dispute_case_id = NULL, active_remedy_action = ''
			WHERE id = $1
		`, order.OrderID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM notifications WHERE target_id = $1`, disputeID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM dispute_events WHERE entity_id = $1`, disputeID)
		cleanupLifecycleCredentialFixtures(t, context.Background(), store, sellerID, buyerID, "")
	})
	if _, err := store.pool.Exec(ctx, `
		UPDATE api_orders
		SET dispute_status = 'awaiting_fulfillment', dispute_case_id = $2, updated_at = $3
		WHERE id = $1
	`, order.OrderID, disputeID, now.Add(-time.Hour)); err != nil {
		t.Fatalf("attach remedy dispute: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO api_order_dispute_remedies (
			id, dispute_case_id, action, responsible_user_id, beneficiary_user_id,
			instructions, status, due_at, lateness_status, source,
			created_by_admin_id, created_request_id, created_at, updated_at, version
		)
		VALUES ($1, $2, 'full_refund', $3, $4, '请完成约定退款。', 'pending', $5,
		        'not_due', 'admin_decision', $4, $6, $7, $7, 1)
	`, remedyID, disputeID, sellerID, buyerID, now, "lateness-remedy-created-"+remedyID, now.Add(-24*time.Hour)); err != nil {
		t.Fatalf("seed pending remedy: %v", err)
	}
	var initialCommercialOutcome string
	if err := store.pool.QueryRow(ctx, `SELECT commercial_outcome FROM api_orders WHERE id = $1`, order.OrderID).Scan(&initialCommercialOutcome); err != nil {
		t.Fatalf("read initial commercial outcome: %v", err)
	}

	tx, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin lateness decision: %v", err)
	}
	result, appErr := updateDisputeAdminInTx(ctx, tx, report.AdminActionInput{
		ID: disputeID, Action: "confirm_lateness", ExpectedVersion: 1,
		AdminUserID: buyerID, Reason: "责任方未在约定期限内声明履行。", RequestID: "confirm-lateness-" + remedyID,
	}, now)
	if appErr != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("confirm remedy lateness: %v", appErr)
	}
	if result.Dispute == nil || result.Dispute.Status != report.DisputeStatusResolved || !result.Dispute.Active || result.Dispute.ClosedAt != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("lateness decision changed dispute progress: %+v", result.Dispute)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit lateness decision: %v", err)
	}

	claimAt := now.Add(time.Hour)
	tx, err = store.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin late claim: %v", err)
	}
	dispute, err := scanDispute(ctx, tx, disputeSelectSQL+` WHERE d.id = $1 FOR UPDATE OF d`, disputeID)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("lock dispute for late claim: %v", err)
	}
	storedOrder, err := store.getAPIOrder(ctx, tx, order.OrderID, true, false)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("lock order for late claim: %v", err)
	}
	if appErr := store.applyDisputeParticipantActionInTx(ctx, tx, &dispute, &storedOrder, report.DisputeParticipantActionInput{
		DisputeID: disputeID, Action: report.DisputeRemedyActionClaim, ActorUserID: sellerID,
		Note: "已在迟到裁定后补充履行。", RequestID: "late-claim-" + remedyID,
	}, claimAt); appErr != nil {
		_ = tx.Rollback(ctx)
		t.Fatalf("claim after lateness decision: %v", appErr)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit late claim: %v", err)
	}

	var remedyStatus, latenessStatus, disputeStatus, orderDisputeStatus, commercialOutcome string
	var active bool
	if err := store.pool.QueryRow(ctx, `
		SELECT r.status, r.lateness_status, d.status, d.active, o.dispute_status, o.commercial_outcome
		FROM api_order_dispute_remedies r
		JOIN dispute_cases d ON d.id = r.dispute_case_id
		JOIN api_orders o ON o.id = d.api_order_id
		WHERE r.id = $1
	`, remedyID).Scan(&remedyStatus, &latenessStatus, &disputeStatus, &active, &orderDisputeStatus, &commercialOutcome); err != nil {
		t.Fatalf("read late claim state: %v", err)
	}
	if remedyStatus != report.RemedyStatusClaimedFulfilled || latenessStatus != report.RemedyLatenessLateConfirmed ||
		disputeStatus != report.DisputeStatusResolved || !active || orderDisputeStatus != apiorder.DisputeStatusFulfillmentConfirmation ||
		commercialOutcome != initialCommercialOutcome {
		t.Fatalf("unexpected late claim state remedy=%q lateness=%q dispute=%q active=%v order=%q commercial=%q", remedyStatus, latenessStatus, disputeStatus, active, orderDisputeStatus, commercialOutcome)
	}
}

func TestPostgresDataLifecycleDestroysAPICredentialsAfterTrustedHoldsAndLatestAnchor(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	runAt := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	lateRunAt := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	oldCompletion := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	laterAnchor := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)

	sellerID := uuid.NewString()
	sellerContactID := uuid.NewString()
	buyerID := uuid.NewString()
	buyerContactID := uuid.NewString()
	serviceID := uuid.NewString()
	quotaOrderID := ""
	t.Cleanup(func() {
		cleanupLifecycleCredentialFixtures(t, context.Background(), store, sellerID, buyerID, quotaOrderID)
	})
	seedQuotaServiceForTest(t, ctx, store.pool, sellerID, sellerContactID, buyerID, buyerContactID, serviceID, oldCompletion)
	packageID := uuid.NewString()
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO api_service_packages (
			id, api_service_id, name, price_cny, duration_days, description,
			panel_allowance, stock_total, stock_available, enabled, created_at, updated_at
		)
		VALUES ($1, $2, 'Lifecycle package', 20, 30, 'Lifecycle package', 10, 1, 1, true, $3, $3)
	`, packageID, serviceID, oldCompletion); err != nil {
		t.Fatalf("seed lifecycle package: %v", err)
	}

	eligibleFirst := insertLifecycleCompletedCredentialOrder(t, store, serviceID, sellerID, sellerContactID, buyerID, buyerContactID, oldCompletion, oldCompletion.Add(-time.Hour), "", nil)
	eligibleSecond := insertLifecycleCompletedCredentialOrder(t, store, serviceID, sellerID, sellerContactID, buyerID, buyerContactID, oldCompletion.Add(24*time.Hour), oldCompletion, "", nil)
	openHold := insertLifecycleCompletedCredentialOrder(t, store, serviceID, sellerID, sellerContactID, buyerID, buyerContactID, oldCompletion.Add(48*time.Hour), oldCompletion.Add(47*time.Hour), "", nil)
	waitingHold := insertLifecycleCompletedCredentialOrder(t, store, serviceID, sellerID, sellerContactID, buyerID, buyerContactID, oldCompletion.Add(72*time.Hour), oldCompletion.Add(71*time.Hour), "", nil)
	appealHold := insertLifecycleCompletedCredentialOrder(t, store, serviceID, sellerID, sellerContactID, buyerID, buyerContactID, oldCompletion.Add(96*time.Hour), oldCompletion.Add(95*time.Hour), "", nil)
	deliveryAnchor := insertLifecycleCompletedCredentialOrder(t, store, serviceID, sellerID, sellerContactID, buyerID, buyerContactID, oldCompletion.Add(120*time.Hour), laterAnchor, "", nil)
	packageAnchor := insertLifecycleCompletedCredentialOrder(t, store, serviceID, sellerID, sellerContactID, buyerID, buyerContactID, oldCompletion.Add(144*time.Hour), oldCompletion.Add(143*time.Hour), packageID, &laterAnchor)
	quotaAnchor := insertLifecycleCompletedQuotaCredentialOrder(t, store, serviceID, sellerID, sellerContactID, buyerID, buyerContactID, oldCompletion.Add(168*time.Hour), oldCompletion.Add(167*time.Hour), laterAnchor)
	quotaOrderID = quotaAnchor.Order.OrderID
	raceHold := insertLifecycleCompletedCredentialOrder(t, store, serviceID, sellerID, sellerContactID, buyerID, buyerContactID, oldCompletion.Add(192*time.Hour), oldCompletion.Add(191*time.Hour), "", nil)

	openDisputeID := insertLifecycleDispute(t, store, openHold.OrderID, buyerID, sellerID, "open", oldCompletion)
	waitingDisputeID := insertLifecycleDispute(t, store, waitingHold.OrderID, buyerID, sellerID, "waiting_info", oldCompletion)
	appealDisputeID := insertLifecycleFinalDispute(t, store, appealHold.OrderID, buyerID, sellerID, oldCompletion)
	appealID := createLifecycleAppeal(t, store, buyerID, appealDisputeID, oldCompletion)
	if _, err := store.pool.Exec(ctx, `UPDATE appeals SET target_type = 'public_user', target_id = 'poisoned-target' WHERE id = $1`, appealID); err != nil {
		t.Fatalf("poison denormalized appeal target: %v", err)
	}
	falseHoldDisputeID := insertLifecycleFinalDispute(t, store, deliveryAnchor.OrderID, buyerID, sellerID, oldCompletion)
	falseHoldAppealID := uuid.NewString()
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO appeals (
			id, appellant_user_id, dispute_case_id, target_type, target_id,
			title, statement, status, created_at, updated_at
		)
		VALUES ($1, $2, $3, 'api_order', $4, 'Poisoned target', 'Must follow the dispute FK.', 'submitted', $5, $5)
	`, falseHoldAppealID, buyerID, falseHoldDisputeID, eligibleFirst.OrderID, oldCompletion); err != nil {
		t.Fatalf("seed poisoned false-hold appeal: %v", err)
	}

	retiredCredentialID := insertLifecycleQuotaCredential(t, store, quotaAnchor.OfferID, sellerID, "retired", "", oldCompletion)
	availableCredentialID := insertLifecycleQuotaCredential(t, store, quotaAnchor.OfferID, sellerID, "available", "", time.Time{})
	reservedCredentialID := insertLifecycleQuotaCredential(t, store, quotaAnchor.OfferID, sellerID, "reserved", eligibleSecond.OrderID, oldCompletion)

	policy := lifecycleCredentialPolicy()
	first, appErr := store.RunDataLifecycle(ctx, runAt, 1, policy)
	if appErr != nil {
		t.Fatalf("run first credential lifecycle batch: %v", appErr)
	}
	if first.APIOrderCredentialsDestroyed != 1 || first.APIQuotaCredentialsDestroyed != 1 {
		t.Fatalf("unexpected first credential batch: %+v", first)
	}
	assertLifecycleOrderCredentialState(t, store, eligibleFirst.CredentialID, true, "retention_expired")
	assertLifecycleOrderCredentialState(t, store, eligibleSecond.CredentialID, false, "")
	assertLifecycleQuotaCredentialState(t, store, retiredCredentialID, true, "retired_unused")
	assertLifecycleQuotaCredentialState(t, store, availableCredentialID, false, "")
	assertLifecycleQuotaCredentialState(t, store, reservedCredentialID, false, "")
	for _, fixture := range []lifecycleCredentialOrderFixture{openHold, waitingHold, appealHold, deliveryAnchor, packageAnchor, quotaAnchor.Order, raceHold} {
		assertLifecycleOrderCredentialState(t, store, fixture.CredentialID, false, "")
	}

	destroyedSourceDisputeID := insertLifecycleFinalDispute(t, store, eligibleFirst.OrderID, buyerID, sellerID, runAt)
	destroyedAppealTx, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin destroyed credential appeal: %v", err)
	}
	_, destroyedAppealErr := createAppealInTx(ctx, destroyedAppealTx, report.CreateAppealInput{
		AppellantUserID: buyerID,
		DisputeID:       destroyedSourceDisputeID,
		Title:           "Destroyed credential appeal",
		Statement:       "The lifecycle guard must reject this appeal.",
	}, runAt)
	_ = destroyedAppealTx.Rollback(ctx)
	if destroyedAppealErr == nil || destroyedAppealErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("destroyed credential appeal was not rejected: %#v", destroyedAppealErr)
	}
	destroyedReportID := insertLifecycleAPIOrderReport(t, store, eligibleFirst.OrderID, buyerID, sellerID, oldCompletion)
	destroyedReportTx, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin destroyed credential report dispute: %v", err)
	}
	_, destroyedReportErr := updateReportAdminInTx(ctx, destroyedReportTx, report.AdminActionInput{
		ID: destroyedReportID, AdminUserID: sellerID, Action: "open_dispute", Reason: "Open destroyed order dispute",
		PublicSummary: "Destroyed order dispute", PublicResultCode: report.PublicResultNoAction, PublicResult: "Reviewing", ExpectedVersion: 1,
	}, runAt)
	_ = destroyedReportTx.Rollback(ctx)
	if destroyedReportErr == nil || destroyedReportErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("destroyed credential report dispute was not rejected: %#v", destroyedReportErr)
	}

	second, appErr := store.RunDataLifecycle(ctx, runAt, 1, policy)
	if appErr != nil {
		t.Fatalf("run second credential lifecycle batch: %v", appErr)
	}
	if second.APIOrderCredentialsDestroyed != 1 || second.APIQuotaCredentialsDestroyed != 0 {
		t.Fatalf("unexpected second credential batch: %+v", second)
	}
	assertLifecycleOrderCredentialState(t, store, eligibleSecond.CredentialID, true, "retention_expired")

	raceDisputeID := insertLifecycleFinalDispute(t, store, raceHold.OrderID, buyerID, sellerID, runAt)
	raceTx, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin concurrent appeal: %v", err)
	}
	raceAppeal, raceAppealErr := createAppealInTx(ctx, raceTx, report.CreateAppealInput{
		AppellantUserID: buyerID,
		DisputeID:       raceDisputeID,
		Title:           "Concurrent appeal",
		Statement:       "This appeal owns the credential lifecycle lock.",
	}, runAt)
	if raceAppealErr != nil {
		_ = raceTx.Rollback(ctx)
		t.Fatalf("create concurrent appeal: %v", raceAppealErr)
	}
	lockedRun, appErr := store.RunDataLifecycle(ctx, runAt, 20, policy)
	if appErr != nil {
		_ = raceTx.Rollback(ctx)
		t.Fatalf("run lifecycle behind appeal lock: %v", appErr)
	}
	if lockedRun.APIOrderCredentialsDestroyed != 0 {
		_ = raceTx.Rollback(ctx)
		t.Fatalf("maintenance destroyed a credential behind the appeal lock: %+v", lockedRun)
	}
	assertLifecycleOrderCredentialState(t, store, raceHold.CredentialID, false, "")
	if err := raceTx.Commit(ctx); err != nil {
		t.Fatalf("commit concurrent appeal: %v", err)
	}

	reportRaceHold := insertLifecycleCompletedCredentialOrder(t, store, serviceID, sellerID, sellerContactID, buyerID, buyerContactID, oldCompletion.Add(216*time.Hour), oldCompletion.Add(215*time.Hour), "", nil)
	reportRaceID := insertLifecycleAPIOrderReport(t, store, reportRaceHold.OrderID, buyerID, sellerID, oldCompletion)
	reportRaceTx, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin concurrent report dispute: %v", err)
	}
	reportRaceResult, reportRaceErr := updateReportAdminInTx(ctx, reportRaceTx, report.AdminActionInput{
		ID: reportRaceID, AdminUserID: sellerID, Action: "open_dispute", Reason: "Open lifecycle report dispute",
		PublicSummary: "Lifecycle report dispute", PublicResultCode: report.PublicResultNoAction, PublicResult: "Reviewing", ExpectedVersion: 1,
	}, runAt)
	if reportRaceErr != nil || reportRaceResult.Dispute == nil {
		_ = reportRaceTx.Rollback(ctx)
		t.Fatalf("create concurrent report dispute: result=%+v err=%v", reportRaceResult, reportRaceErr)
	}
	reportLockedRun, appErr := store.RunDataLifecycle(ctx, runAt, 20, policy)
	if appErr != nil {
		_ = reportRaceTx.Rollback(ctx)
		t.Fatalf("run lifecycle behind report dispute lock: %v", appErr)
	}
	if reportLockedRun.APIOrderCredentialsDestroyed != 0 {
		_ = reportRaceTx.Rollback(ctx)
		t.Fatalf("maintenance destroyed a credential behind the report dispute lock: %+v", reportLockedRun)
	}
	assertLifecycleOrderCredentialState(t, store, reportRaceHold.CredentialID, false, "")
	if err := reportRaceTx.Commit(ctx); err != nil {
		t.Fatalf("commit concurrent report dispute: %v", err)
	}

	idempotent, appErr := store.RunDataLifecycle(ctx, runAt, 20, policy)
	if appErr != nil {
		t.Fatalf("rerun credential lifecycle: %v", appErr)
	}
	if idempotent.APIOrderCredentialsDestroyed != 0 || idempotent.APIQuotaCredentialsDestroyed != 0 {
		t.Fatalf("credential lifecycle rerun was not idempotent: %+v", idempotent)
	}

	if _, err := store.pool.Exec(ctx, `
		UPDATE dispute_cases
		SET status = 'closed', active = false, closed_at = $2,
		    final_reason = 'applicant_decision_expired', appeal_expires_at = NULL,
		    adversely_affected_user_ids = '{}'::uuid[],
		    next_actor = 'none', due_at = NULL, updated_at = $2
		WHERE id = ANY($1::uuid[])
		`, []string{openDisputeID, waitingDisputeID, reportRaceResult.Dispute.ID}, lateRunAt); err != nil {
		t.Fatalf("release dispute holds: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		UPDATE appeals
		SET status = 'approved', handled_by_admin_id = $2, handled_at = $3, updated_at = $3
		WHERE id = ANY($1::uuid[])
	`, []string{appealID, falseHoldAppealID, raceAppeal.ID}, sellerID, lateRunAt); err != nil {
		t.Fatalf("release appeal holds: %v", err)
	}

	late, appErr := store.RunDataLifecycle(ctx, lateRunAt, 20, policy)
	if appErr != nil {
		t.Fatalf("run lifecycle after latest anchors and holds: %v", appErr)
	}
	if late.APIOrderCredentialsDestroyed != 8 || late.APIQuotaCredentialsDestroyed != 1 {
		t.Fatalf("unexpected released credential batch: %+v", late)
	}
	for _, fixture := range []lifecycleCredentialOrderFixture{openHold, waitingHold, appealHold, deliveryAnchor, packageAnchor, quotaAnchor.Order, raceHold, reportRaceHold} {
		assertLifecycleOrderCredentialState(t, store, fixture.CredentialID, true, "retention_expired")
	}
	assertLifecycleQuotaCredentialState(t, store, quotaAnchor.CredentialID, true, "retention_expired")
	assertLifecycleQuotaCredentialState(t, store, availableCredentialID, false, "")
	assertLifecycleQuotaCredentialState(t, store, reservedCredentialID, false, "")

	finalRun, appErr := store.RunDataLifecycle(ctx, lateRunAt, 20, policy)
	if appErr != nil {
		t.Fatalf("run final credential lifecycle rerun: %v", appErr)
	}
	if finalRun.APIOrderCredentialsDestroyed != 0 || finalRun.APIQuotaCredentialsDestroyed != 0 {
		t.Fatalf("final credential lifecycle rerun was not idempotent: %+v", finalRun)
	}
}

type lifecycleCredentialOrderFixture struct {
	OrderID      string
	CredentialID string
}

type lifecycleQuotaOrderFixture struct {
	Order        lifecycleCredentialOrderFixture
	CredentialID string
	OfferID      string
}

func lifecycleCredentialPolicy() maintenance.Policy {
	return maintenance.Policy{
		SessionRetention:               30 * 24 * time.Hour,
		EmailVerificationRetention:     30 * 24 * time.Hour,
		ReadNotificationRetention:      90 * 24 * time.Hour,
		UnreadNotificationRetention:    365 * 24 * time.Hour,
		DomainEventRetention:           365 * 24 * time.Hour,
		APIDeliveryCredentialRetention: 30 * 24 * time.Hour,
		APIProbeSampleRetention:        7 * 24 * time.Hour,
	}
}

func insertLifecycleCompletedCredentialOrder(t *testing.T, store *Store, serviceID, sellerID, sellerContactID, buyerID, buyerContactID string, completedAt, deliveredAt time.Time, packageID string, packageExpiresAt *time.Time) lifecycleCredentialOrderFixture {
	t.Helper()
	ctx := context.Background()
	var sellerVersionID, buyerVersionID string
	if err := store.pool.QueryRow(ctx, `
		SELECT seller.current_version_id::text, buyer.current_version_id::text
		FROM contact_methods seller
		JOIN contact_methods buyer ON buyer.id = $2
		WHERE seller.id = $1
	`, sellerContactID, buyerContactID).Scan(&sellerVersionID, &buyerVersionID); err != nil {
		t.Fatalf("read lifecycle contact versions: %v", err)
	}
	billingMode := "manual_usage_check"
	if packageID != "" {
		billingMode = "fixed_package"
	}
	intentID := uuid.NewString()
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO api_purchase_intents (
			id, api_service_id, api_service_owner_user_id, buyer_user_id, owner_user_id,
			buyer_contact_method_id, buyer_contact_method_version_id,
			owner_contact_method_id, owner_contact_method_version_id,
			status, requested_cny_amount, selected_access_mode,
			selected_package_id, selected_package_snapshot,
			service_version_snapshot, service_title_snapshot,
			distribution_system_snapshot, billing_mode_snapshot,
			buyer_contact_type_snapshot, buyer_contact_label_snapshot,
			owner_contact_type_snapshot, owner_contact_label_snapshot,
			minimum_intent_cny_snapshot, pricing_snapshot, contacted_at, created_at, updated_at
		)
		VALUES (
			$1, $2, $3, $4, $3, $5, $6, $7, $8,
			'ordered', 20, 'buyer_dedicated_sub_key',
			NULLIF($9, '')::uuid, CASE WHEN $9 = '' THEN NULL ELSE '{}'::jsonb END,
			1, 'Lifecycle API service', 'sub2api', $10,
			'linuxdo', 'linux.do', 'linuxdo', 'linux.do', 1, '{}'::jsonb, $11, $11, $11
		)
	`, intentID, serviceID, sellerID, buyerID, buyerContactID, buyerVersionID, sellerContactID, sellerVersionID, packageID, billingMode, completedAt.Add(-4*time.Hour)); err != nil {
		t.Fatalf("seed lifecycle purchase intent: %v", err)
	}
	orderID := uuid.NewString()
	orderNo, err := apiorder.GenerateOrderNo(completedAt.Add(-4 * time.Hour))
	if err != nil {
		t.Fatalf("generate lifecycle order number: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO api_orders (
			id, api_purchase_intent_id, api_service_id, buyer_user_id, seller_user_id,
			status, service_title_snapshot, service_version_snapshot, billing_mode_snapshot,
			selected_package_id, selected_package_snapshot, package_expires_at,
			amount, currency, selected_payment_method, payment_window_minutes_snapshot,
			payment_expires_at, payment_instructions_snapshot,
			payment_summary, payment_submitted_at, paid_confirmed_at,
			delivery_note, delivery_submitted_at, delivery_review_expires_at,
			completion_source, completed_at, created_at, updated_at, order_no
		)
		VALUES (
			$1, $2, $3, $4, $5, 'completed', 'Lifecycle API service', 1, $6,
			NULLIF($7, '')::uuid, CASE WHEN $7 = '' THEN NULL ELSE '{}'::jsonb END, $8,
			20, 'CNY', 'wechat', 10, $9, 'Offsite payment',
			'Paid', $10, $11, 'Delivered', $12, $13,
			'buyer_confirmed', $14, $15, $14, $16
		)
	`, orderID, intentID, serviceID, buyerID, sellerID, billingMode, packageID, packageExpiresAt,
		completedAt.Add(-3*time.Hour), completedAt.Add(-2*time.Hour), completedAt.Add(-time.Hour), deliveredAt,
		deliveredAt.Add(24*time.Hour), completedAt, completedAt.Add(-4*time.Hour), orderNo); err != nil {
		t.Fatalf("seed lifecycle order: %v", err)
	}
	credentialID := insertLifecycleOrderCredential(t, store, orderID, sellerID, buyerID, deliveredAt)
	return lifecycleCredentialOrderFixture{OrderID: orderID, CredentialID: credentialID}
}

func insertLifecycleOrderCredential(t *testing.T, store *Store, orderID, sellerID, buyerID string, submittedAt time.Time) string {
	t.Helper()
	credentialID := uuid.NewString()
	if _, err := store.pool.Exec(context.Background(), `
		INSERT INTO api_order_delivery_credentials (
			id, api_order_id, seller_user_id, buyer_user_id, delivery_kind,
			api_base_url, instructions, api_key_ciphertext, api_key_nonce,
			secret_encryption_key_version, secret_encryption_format, submitted_at, created_at
		)
		VALUES ($1, $2, $3, $4, 'api_key_endpoint', 'https://lifecycle.example.com/v1',
		        'Lifecycle credential', decode('0102', 'hex'), decode('000000000000000000000000', 'hex'),
		        'test-v1', 'aad_v1', $5, $5)
	`, credentialID, orderID, sellerID, buyerID, submittedAt); err != nil {
		t.Fatalf("seed lifecycle order credential: %v", err)
	}
	return credentialID
}

func insertLifecycleCompletedQuotaCredentialOrder(t *testing.T, store *Store, serviceID, sellerID, sellerContactID, buyerID, buyerContactID string, completedAt, deliveredAt, quotaExpiresAt time.Time) lifecycleQuotaOrderFixture {
	t.Helper()
	ctx := context.Background()
	batchID := uuid.NewString()
	offerID := uuid.NewString()
	allocationID := uuid.NewString()
	unitID := uuid.NewString()
	createdAt := completedAt.Add(-7 * 24 * time.Hour)
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO api_quota_batches (
			id, api_service_id, owner_user_id, source_type, status,
			declared_total_usd_allowance, unallocated_usd_allowance,
			sale_cutoff_at, expires_at, source_confirmed_at, published_at,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, 'sub2api', 'published', 10, 0, $4, $5, $6, $6, $6, $6)
	`, batchID, serviceID, sellerID, quotaExpiresAt.Add(-time.Hour), quotaExpiresAt, createdAt); err != nil {
		t.Fatalf("seed lifecycle quota batch: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO api_quota_offers (
			id, batch_id, api_service_id, owner_user_id, distribution_system,
			name, usd_allowance, price_cny, model_multiplier,
			delivery_mode, delivery_eta_minutes, sale_mode, status,
			published_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, 'sub2api', 'Lifecycle quota', 10, 10, 1,
		        'preimported', 1, 'continuous', 'published', $5, $5, $5)
	`, offerID, batchID, serviceID, sellerID, createdAt); err != nil {
		t.Fatalf("seed lifecycle quota offer: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO api_quota_allocations (
			id, batch_id, offer_id, api_service_id, owner_user_id,
			sale_mode, copy_limit, allocated_usd_allowance, status, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, 'continuous', 1, 10, 'active', $6, $6)
	`, allocationID, batchID, offerID, serviceID, sellerID, createdAt); err != nil {
		t.Fatalf("seed lifecycle quota allocation: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO api_quota_inventory_units (
			id, allocation_id, batch_id, offer_id, usd_allowance, status, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, 10, 'available', $5, $5)
	`, unitID, allocationID, batchID, offerID, createdAt); err != nil {
		t.Fatalf("seed lifecycle quota inventory: %v", err)
	}
	quotaCredentialID := insertLifecycleQuotaCredential(t, store, offerID, sellerID, "available", "", createdAt)

	var sellerVersionID, buyerVersionID string
	if err := store.pool.QueryRow(ctx, `
		SELECT seller.current_version_id::text, buyer.current_version_id::text
		FROM contact_methods seller
		JOIN contact_methods buyer ON buyer.id = $2
		WHERE seller.id = $1
	`, sellerContactID, buyerContactID).Scan(&sellerVersionID, &buyerVersionID); err != nil {
		t.Fatalf("read lifecycle quota contact versions: %v", err)
	}
	intentID := uuid.NewString()
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO api_purchase_intents (
			id, api_service_id, api_service_owner_user_id, buyer_user_id, owner_user_id,
			buyer_contact_method_id, buyer_contact_method_version_id,
			owner_contact_method_id, owner_contact_method_version_id,
			status, requested_cny_amount, requested_usd_allowance, selected_access_mode,
			service_version_snapshot, service_title_snapshot,
			distribution_system_snapshot, billing_mode_snapshot,
			buyer_contact_type_snapshot, buyer_contact_label_snapshot,
			owner_contact_type_snapshot, owner_contact_label_snapshot,
			minimum_intent_cny_snapshot, pricing_snapshot, purchase_kind,
			api_quota_batch_id, api_quota_offer_id, api_quota_allocation_id,
			api_quota_inventory_unit_id, quota_offer_snapshot,
			contacted_at, created_at, updated_at
		)
		VALUES (
			$1, $2, $3, $4, $3, $5, $6, $7, $8,
			'ordered', 10, 10, 'buyer_dedicated_sub_key',
			1, 'Lifecycle quota service', 'sub2api', 'manual_usage_check',
			'linuxdo', 'linux.do', 'linuxdo', 'linux.do', 1, '{}'::jsonb, 'limited_quota_offer',
			$9, $10, $11, $12, '{}'::jsonb, $13, $13, $13
		)
	`, intentID, serviceID, sellerID, buyerID, buyerContactID, buyerVersionID, sellerContactID, sellerVersionID,
		batchID, offerID, allocationID, unitID, createdAt); err != nil {
		t.Fatalf("seed lifecycle quota intent: %v", err)
	}
	orderID := uuid.NewString()
	orderNo, err := apiorder.GenerateOrderNo(createdAt)
	if err != nil {
		t.Fatalf("generate lifecycle quota order number: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO api_orders (
			id, api_purchase_intent_id, api_service_id, buyer_user_id, seller_user_id,
			status, service_title_snapshot, service_version_snapshot, billing_mode_snapshot,
			amount, currency, selected_payment_method, payment_window_minutes_snapshot,
			payment_expires_at, payment_instructions_snapshot,
			payment_summary, payment_submitted_at, paid_confirmed_at,
			delivery_note, delivery_submitted_at, delivery_review_expires_at,
			completion_source, completed_at, created_at, updated_at, order_no,
			purchase_kind, api_quota_batch_id, api_quota_offer_id,
			api_quota_allocation_id, api_quota_inventory_unit_id, api_quota_credential_id,
			quota_offer_snapshot, quota_offer_name_snapshot,
			quota_usd_allowance_snapshot, quota_price_cny_snapshot,
			quota_cny_per_usd_snapshot, quota_model_multiplier_snapshot,
			quota_sale_cutoff_at_snapshot, quota_expires_at_snapshot,
			quota_sale_mode_snapshot, quota_distribution_system_snapshot,
			quota_ttft_band_snapshot, quota_declared_max_concurrency_snapshot,
			quota_performance_confirmed_at_snapshot, quota_performance_unverified_snapshot,
			quota_delivery_eta_minutes_snapshot, quota_delivery_mode_snapshot,
			requested_usd_allowance_snapshot, cny_per_usd_allowance_snapshot
		)
		VALUES (
			$1, $2, $3, $4, $5, 'completed', 'Lifecycle quota service', 1, 'manual_usage_check',
			10, 'CNY', 'wechat', 10, $6, 'Offsite payment',
			'Paid', $7, $8, 'Delivered', $9, $10,
			'buyer_confirmed', $11, $12, $11, $13,
			'limited_quota_offer', $14, $15, $16, $17, $18,
			'{}'::jsonb, 'Lifecycle quota', 10, 10, 1, 1,
			$19, $20, 'continuous', 'sub2api', 'under_1s', 10,
			$12, true, 1, 'preimported', 10, 1
		)
	`, orderID, intentID, serviceID, buyerID, sellerID, completedAt.Add(-3*time.Hour), completedAt.Add(-2*time.Hour),
		completedAt.Add(-time.Hour), deliveredAt, deliveredAt.Add(24*time.Hour), completedAt, createdAt, orderNo,
		batchID, offerID, allocationID, unitID, quotaCredentialID, quotaExpiresAt.Add(-time.Hour), quotaExpiresAt); err != nil {
		t.Fatalf("seed lifecycle quota order: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		UPDATE api_quota_inventory_units
		SET status = 'consumed', reserved_order_id = $2, reserved_at = $3,
		    consumed_at = $4, updated_at = $4
		WHERE id = $1
	`, unitID, orderID, completedAt.Add(-2*time.Hour), deliveredAt); err != nil {
		t.Fatalf("consume lifecycle quota inventory: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		UPDATE api_quota_credentials
		SET status = 'delivered', reserved_order_id = $2, reserved_at = $3,
		    delivered_at = $4, updated_at = $4
		WHERE id = $1
	`, quotaCredentialID, orderID, completedAt.Add(-2*time.Hour), deliveredAt); err != nil {
		t.Fatalf("deliver lifecycle quota credential: %v", err)
	}
	orderCredentialID := insertLifecycleOrderCredential(t, store, orderID, sellerID, buyerID, deliveredAt)
	return lifecycleQuotaOrderFixture{
		Order:        lifecycleCredentialOrderFixture{OrderID: orderID, CredentialID: orderCredentialID},
		CredentialID: quotaCredentialID,
		OfferID:      offerID,
	}
}

func insertLifecycleQuotaCredential(t *testing.T, store *Store, offerID, sellerID, status, reservedOrderID string, at time.Time) string {
	t.Helper()
	if at.IsZero() {
		at = time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	}
	credentialID := uuid.NewString()
	if _, err := store.pool.Exec(context.Background(), `
		INSERT INTO api_quota_credentials (
			id, api_quota_offer_id, seller_user_id, delivery_kind,
			api_base_url, instructions, api_key_ciphertext, api_key_nonce,
			secret_encryption_key_version, secret_encryption_format, secret_fingerprint,
			status, reserved_order_id, reserved_at, retired_at, created_at, updated_at
		)
		VALUES (
			$1, $2, $3, 'api_key_endpoint', 'https://quota-lifecycle.example.com/v1',
			'Lifecycle quota credential', decode('0102', 'hex'), decode('000000000000000000000000', 'hex'),
			'test-v1', 'aad_v1', decode(md5(($1::uuid)::text), 'hex'),
			$4, NULLIF($5, '')::uuid,
			CASE WHEN $4 = 'reserved' THEN $6::timestamptz ELSE NULL END,
			CASE WHEN $4 = 'retired' THEN $6::timestamptz ELSE NULL END,
			$6, $6
		)
	`, credentialID, offerID, sellerID, status, reservedOrderID, at); err != nil {
		t.Fatalf("seed lifecycle quota credential %s: %v", status, err)
	}
	return credentialID
}

func insertLifecycleDispute(t *testing.T, store *Store, orderID, buyerID, sellerID, status string, now time.Time) string {
	t.Helper()
	disputeID := uuid.NewString()
	var resolvedAt any
	var closedAt any
	if status == "resolved" {
		resolvedAt = now
	}
	if status == "closed" {
		closedAt = now
	}
	if _, err := store.pool.Exec(context.Background(), `
		INSERT INTO dispute_cases (
			id, target_type, target_id, api_order_id, active, target_label,
			primary_user_id, counterparty_user_id, subject_user_id,
			status, public_summary, public_result_code, public_result,
			admin_reason, opened_by_admin_id, opened_at, resolved_at, closed_at,
			created_at, updated_at
		)
		VALUES ($1, 'api_order', $2, $9::uuid, $5 <> 'closed', 'Lifecycle order', $3, $4, $3,
		        $5, 'Lifecycle dispute', 'no_action', 'Lifecycle result',
		        'Lifecycle reason', $4, $6, $7, $8, $6, $6)
	`, disputeID, orderID, buyerID, sellerID, status, now, resolvedAt, closedAt, orderID); err != nil {
		t.Fatalf("seed lifecycle dispute %s: %v", status, err)
	}
	return disputeID
}

func insertLifecycleFinalDispute(t *testing.T, store *Store, orderID, buyerID, sellerID string, now time.Time) string {
	t.Helper()
	disputeID := uuid.NewString()
	if _, err := store.pool.Exec(context.Background(), `
		INSERT INTO dispute_cases (
			id, target_type, target_id, api_order_id, active, target_label,
			primary_user_id, counterparty_user_id, subject_user_id,
			status, public_summary, public_result_code, public_result,
			admin_reason, opened_by_admin_id, opened_at, resolved_at,
			final_reason, appeal_expires_at, adversely_affected_user_ids,
			created_at, updated_at
		)
		VALUES (
			$1, 'api_order', $2, NULLIF($3, '')::uuid, false, 'Lifecycle order',
			$4, $5, $4, 'resolved', 'Lifecycle dispute', 'no_action', 'Lifecycle result',
			'Lifecycle reason', $5, $6, $6, 'legacy_resolved',
			$6::timestamptz + interval '30 days', ARRAY[$4::uuid], $6, $6
		)
	`, disputeID, orderID, orderID, buyerID, sellerID, now); err != nil {
		t.Fatalf("seed final lifecycle dispute: %v", err)
	}
	return disputeID
}

func insertLifecycleAPIOrderReport(t *testing.T, store *Store, orderID, buyerID, sellerID string, now time.Time) string {
	t.Helper()
	reportID := uuid.NewString()
	var sellerUsername string
	if err := store.pool.QueryRow(context.Background(), `SELECT username FROM users WHERE id = $1`, sellerID).Scan(&sellerUsername); err != nil {
		t.Fatalf("read lifecycle seller username: %v", err)
	}
	if _, err := store.pool.Exec(context.Background(), `
		INSERT INTO reports (
			id, reporter_user_id, target_type, target_id, canonical_target_type, canonical_target_id,
			target_label, reported_user_id, reported_username, reason_code, title, description,
			status, created_at, updated_at, version
		)
		VALUES ($1, $2, 'api_order', $3, 'api_order', $3, 'Lifecycle API order', $4, $5,
		        'order_delivery_dispute', 'Lifecycle order report', 'Lifecycle order report facts.',
		        'triaged', $6, $6, 1)
	`, reportID, buyerID, orderID, sellerID, sellerUsername, now); err != nil {
		t.Fatalf("seed lifecycle API order report: %v", err)
	}
	return reportID
}

func createLifecycleAppeal(t *testing.T, store *Store, appellantID, disputeID string, now time.Time) string {
	t.Helper()
	tx, err := store.pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("begin lifecycle appeal: %v", err)
	}
	item, appErr := createAppealInTx(context.Background(), tx, report.CreateAppealInput{
		AppellantUserID: appellantID,
		DisputeID:       disputeID,
		Title:           "Lifecycle appeal",
		Statement:       "Lifecycle appeal statement.",
	}, now)
	if appErr != nil {
		_ = tx.Rollback(context.Background())
		t.Fatalf("create lifecycle appeal: %v", appErr)
	}
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatalf("commit lifecycle appeal: %v", err)
	}
	return item.ID
}

func assertLifecycleOrderCredentialState(t *testing.T, store *Store, credentialID string, destroyed bool, reason string) {
	t.Helper()
	var actualDestroyed, payloadCleared bool
	var actualReason string
	if err := store.pool.QueryRow(context.Background(), `
		SELECT destroyed_at IS NOT NULL, COALESCE(destroy_reason, ''),
		       api_base_url IS NULL AND panel_login_url IS NULL
		       AND username IS NULL AND instructions IS NULL
		       AND api_key_ciphertext IS NULL AND api_key_nonce IS NULL
		       AND password_ciphertext IS NULL AND password_nonce IS NULL
		FROM api_order_delivery_credentials
		WHERE id = $1
	`, credentialID).Scan(&actualDestroyed, &actualReason, &payloadCleared); err != nil {
		t.Fatalf("read lifecycle order credential %s: %v", credentialID, err)
	}
	if actualDestroyed != destroyed || actualReason != reason || payloadCleared != destroyed {
		t.Fatalf("unexpected order credential state id=%s destroyed=%t reason=%q payload_cleared=%t", credentialID, actualDestroyed, actualReason, payloadCleared)
	}
}

func assertLifecycleQuotaCredentialState(t *testing.T, store *Store, credentialID string, destroyed bool, reason string) {
	t.Helper()
	var actualDestroyed, payloadCleared bool
	var actualReason string
	if err := store.pool.QueryRow(context.Background(), `
		SELECT destroyed_at IS NOT NULL, COALESCE(destroy_reason, ''),
		       secret_fingerprint IS NULL
		       AND api_base_url IS NULL AND panel_login_url IS NULL
		       AND username IS NULL AND instructions IS NULL
		       AND api_key_ciphertext IS NULL AND api_key_nonce IS NULL
		       AND password_ciphertext IS NULL AND password_nonce IS NULL
		FROM api_quota_credentials
		WHERE id = $1
	`, credentialID).Scan(&actualDestroyed, &actualReason, &payloadCleared); err != nil {
		t.Fatalf("read lifecycle quota credential %s: %v", credentialID, err)
	}
	if actualDestroyed != destroyed || actualReason != reason || payloadCleared != destroyed {
		t.Fatalf("unexpected quota credential state id=%s destroyed=%t reason=%q payload_cleared=%t", credentialID, actualDestroyed, actualReason, payloadCleared)
	}
}

func cleanupLifecycleCredentialFixtures(t *testing.T, ctx context.Context, store *Store, sellerID, buyerID, quotaOrderID string) {
	t.Helper()
	statements := []struct {
		query string
		args  []any
	}{
		{`DELETE FROM appeals WHERE appellant_user_id = $1`, []any{buyerID}},
		{`UPDATE api_orders SET dispute_case_id = NULL, latest_dispute_case_id = NULL, dispute_status = 'none', active_remedy_action = '' WHERE buyer_user_id = $1 AND seller_user_id = $2`, []any{buyerID, sellerID}},
		{`DELETE FROM dispute_cases WHERE primary_user_id = $1 AND counterparty_user_id = $2`, []any{buyerID, sellerID}},
		{`DELETE FROM moderation_audit_logs WHERE actor_admin_id IN ($1, $2)`, []any{sellerID, buyerID}},
		{`DELETE FROM reports WHERE reporter_user_id = $1`, []any{buyerID}},
		{`UPDATE api_orders SET quota_delivery_mode_snapshot = 'manual', api_quota_credential_id = NULL WHERE id = NULLIF($1, '')::uuid`, []any{quotaOrderID}},
		{`UPDATE api_quota_credentials SET status = CASE WHEN destroyed_at IS NULL THEN 'available' ELSE 'retired' END, reserved_order_id = NULL, reserved_at = NULL, delivered_at = NULL, retired_at = CASE WHEN destroyed_at IS NULL THEN NULL ELSE COALESCE(retired_at, now()) END, updated_at = now() WHERE seller_user_id = $1`, []any{sellerID}},
		{`UPDATE api_quota_inventory_units SET status = 'retired', reserved_order_id = NULL, reserved_at = NULL, consumed_at = NULL, retired_at = COALESCE(retired_at, now()), updated_at = now() WHERE batch_id IN (SELECT id FROM api_quota_batches WHERE owner_user_id = $1)`, []any{sellerID}},
		{`DELETE FROM api_orders WHERE buyer_user_id = $1 AND seller_user_id = $2`, []any{buyerID, sellerID}},
		{`DELETE FROM api_purchase_intents WHERE buyer_user_id = $1 AND owner_user_id = $2`, []any{buyerID, sellerID}},
		{`DELETE FROM api_quota_inventory_units WHERE batch_id IN (SELECT id FROM api_quota_batches WHERE owner_user_id = $1)`, []any{sellerID}},
		{`DELETE FROM api_quota_allocations WHERE owner_user_id = $1`, []any{sellerID}},
		{`DELETE FROM api_quota_sale_rounds WHERE owner_user_id = $1`, []any{sellerID}},
		{`DELETE FROM api_quota_credentials WHERE seller_user_id = $1`, []any{sellerID}},
		{`DELETE FROM api_quota_offers WHERE owner_user_id = $1`, []any{sellerID}},
		{`DELETE FROM api_quota_batches WHERE owner_user_id = $1`, []any{sellerID}},
		{`DELETE FROM api_service_payment_options WHERE api_service_id IN (SELECT id FROM api_services WHERE owner_user_id = $1)`, []any{sellerID}},
		{`DELETE FROM api_services WHERE owner_user_id = $1`, []any{sellerID}},
		{`UPDATE contact_methods SET current_version_id = NULL WHERE user_id IN ($1, $2)`, []any{sellerID, buyerID}},
		{`DELETE FROM contact_method_versions WHERE owner_user_id IN ($1, $2)`, []any{sellerID, buyerID}},
		{`DELETE FROM contact_methods WHERE user_id IN ($1, $2)`, []any{sellerID, buyerID}},
		{`DELETE FROM domain_events WHERE actor_user_id IN ($1, $2)`, []any{sellerID, buyerID}},
		{`DELETE FROM users WHERE id IN ($1, $2)`, []any{sellerID, buyerID}},
	}
	for _, statement := range statements {
		if _, err := store.pool.Exec(ctx, statement.query, statement.args...); err != nil {
			t.Errorf("cleanup lifecycle credential fixture: %v", err)
		}
	}
}

func connectLifecycleTestStore(t *testing.T) *Store {
	t.Helper()
	databaseURL := strings.TrimSpace(os.Getenv("C2C_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not configured")
	}
	store, err := Connect(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect lifecycle test database: %v", err)
	}
	t.Cleanup(store.Close)
	return store
}

func insertLifecycleSession(t *testing.T, store *Store, id, userID, token string, createdAt, expiresAt time.Time, revokedAt *time.Time) {
	t.Helper()
	renewedAt := createdAt.Add(time.Minute)
	absoluteExpiresAt := expiresAt.Add(time.Hour)
	if _, err := store.pool.Exec(context.Background(), `
		INSERT INTO auth_sessions (
			id, user_id, session_token_hash, csrf_token_hash, expires_at, revoked_at,
			created_at, renewed_at, absolute_expires_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $7)
	`, id, userID, token, "csrf-"+token, expiresAt, revokedAt, createdAt, renewedAt, absoluteExpiresAt); err != nil {
		t.Fatalf("insert lifecycle session %s: %v", id, err)
	}
}

func insertLifecycleVerification(t *testing.T, store *Store, id, userID, email string, createdAt, expiresAt time.Time, consumedAt *time.Time) {
	insertLifecycleVerificationForPurpose(t, store, id, userID, email, "bind_email", createdAt, expiresAt, consumedAt)
}

func insertLifecycleVerificationForPurpose(t *testing.T, store *Store, id, userID, email, purpose string, createdAt, expiresAt time.Time, consumedAt *time.Time) {
	t.Helper()
	if _, err := store.pool.Exec(context.Background(), `
		INSERT INTO email_verification_codes (
			id, user_id, email, purpose, code_hash, expires_at, consumed_at, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, id, userID, email, purpose, "hash-"+id, expiresAt, consumedAt, createdAt); err != nil {
		t.Fatalf("insert lifecycle verification %s: %v", id, err)
	}
}

func insertLifecycleEvent(t *testing.T, store *Store, id, actorUserID string, createdAt time.Time) {
	t.Helper()
	if _, err := store.pool.Exec(context.Background(), `
		INSERT INTO domain_events (
			id, aggregate_type, aggregate_id, event_type, actor_user_id, actor_kind,
			aggregate_version, request_id, created_at
		)
		VALUES ($1, 'lifecycle_test', $2, 'lifecycle.tested', $3, 'user', 1, $4, $5)
	`, id, uuid.NewString(), actorUserID, "lifecycle-event-"+id, createdAt); err != nil {
		t.Fatalf("insert lifecycle event %s: %v", id, err)
	}
}

func insertLifecycleNotification(t *testing.T, store *Store, id, userID, eventID string, createdAt time.Time, readAt *time.Time) {
	t.Helper()
	if _, err := store.pool.Exec(context.Background(), `
		INSERT INTO notifications (
			id, user_id, type, title, body, target_type, target_id, target_url,
			source_event_type, source_event_id, read_at, created_at
		)
		VALUES ($1, $2, 'lifecycle_test', 'Lifecycle', 'Lifecycle test', 'user', $2, '/me',
		        'lifecycle.tested', $3, $4, $5)
	`, id, userID, eventID, readAt, createdAt); err != nil {
		t.Fatalf("insert lifecycle notification %s: %v", id, err)
	}
}

func assertLifecycleRowExists(t *testing.T, store *Store, table, id string) {
	t.Helper()
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM ` + table + ` WHERE id = $1)`
	if err := store.pool.QueryRow(context.Background(), query, id).Scan(&exists); err != nil {
		t.Fatalf("check %s row %s: %v", table, id, err)
	}
	if !exists {
		t.Fatalf("expected %s row %s to remain", table, id)
	}
}

func assertLifecycleRowMissing(t *testing.T, store *Store, table, id string) {
	t.Helper()
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM ` + table + ` WHERE id = $1)`
	if err := store.pool.QueryRow(context.Background(), query, id).Scan(&exists); err != nil {
		t.Fatalf("check %s row %s: %v", table, id, err)
	}
	if exists {
		t.Fatalf("expected %s row %s to be removed", table, id)
	}
}
