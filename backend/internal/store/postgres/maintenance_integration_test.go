package postgres

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/maintenance"

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
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM email_verification_codes WHERE id = ANY($1::uuid[])`, []string{verificationExpired, verificationActive, verificationRecentlyConsumed})
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
		SessionRetention:            7 * 24 * time.Hour,
		EmailVerificationRetention:  24 * time.Hour,
		ReadNotificationRetention:   90 * 24 * time.Hour,
		UnreadNotificationRetention: 365 * 24 * time.Hour,
		DomainEventRetention:        365 * 24 * time.Hour,
	}
	result, appErr := store.RunDataLifecycle(ctx, now, 2, policy)
	if appErr != nil {
		t.Fatalf("run lifecycle maintenance: %v", appErr)
	}
	if !result.LockAcquired ||
		result.SessionsDeleted != 1 ||
		result.VerificationCodesDeleted != 1 ||
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
		SessionRetention:            time.Hour,
		EmailVerificationRetention:  time.Hour,
		ReadNotificationRetention:   time.Hour,
		UnreadNotificationRetention: 2 * time.Hour,
		DomainEventRetention:        time.Hour,
	})
	if appErr != nil {
		t.Fatalf("run lifecycle while locked: %v", appErr)
	}
	if result.LockAcquired {
		t.Fatalf("maintenance acquired an already-held advisory lock: %+v", result)
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
	t.Helper()
	if _, err := store.pool.Exec(context.Background(), `
		INSERT INTO email_verification_codes (
			id, user_id, email, purpose, code_hash, expires_at, consumed_at, created_at
		)
		VALUES ($1, $2, $3, 'bind_email', $4, $5, $6, $7)
	`, id, userID, email, "hash-"+id, expiresAt, consumedAt, createdAt); err != nil {
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
