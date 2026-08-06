package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/maintenance"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/google/uuid"
)

func TestPostgresAccountAppealSessionIsExistingIdentityOnlyAndFixed(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	current := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	suffix := strings.ToLower(uuid.NewString()[:8])
	profile := auth.OAuthProfile{
		Provider:         "linux_do",
		Subject:          "account-appeal-" + suffix,
		Username:         "appeal-" + suffix,
		DisplayName:      "Appeal Fixture",
		AvatarURL:        "https://example.com/account-appeal-original.png",
		LinuxDoUserID:    "account-appeal-" + suffix,
		LinuxDoUsername:  "appeal-" + suffix,
		LinuxDoAvatarURL: "https://example.com/account-appeal-original.png",
	}
	created, appErr := store.UpsertOAuthUser(ctx, profile, current.Add(-time.Hour))
	if appErr != nil {
		t.Fatalf("seed account appeal OAuth identity: %v", appErr)
	}
	userID := created.User.ID
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM account_appeal_sessions WHERE user_id = $1`, userID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM auth_sessions WHERE user_id = $1`, userID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM promotion_coupons WHERE user_id = $1`, userID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM referral_relations WHERE inviter_user_id = $1 OR invitee_user_id = $1`, userID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM referral_codes WHERE user_id = $1`, userID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM user_activity_daily WHERE user_id = $1`, userID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM user_registration_attributions WHERE user_id = $1`, userID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM linux_do_bindings WHERE user_id = $1`, userID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM auth_identities WHERE user_id = $1`, userID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM user_permissions WHERE user_id = $1`, userID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = $1`, userID)
	})
	if _, err := store.pool.Exec(ctx, `
		UPDATE users
		SET account_status = 'suspended', updated_at = $2, version = version + 1
		WHERE id = $1
	`, userID, current.Add(-30*time.Minute)); err != nil {
		t.Fatalf("restrict account appeal fixture: %v", err)
	}
	ordinarySessionHash := "ordinary-session-" + uuid.NewString()
	if appErr := store.CreateSession(
		ctx,
		userID,
		ordinarySessionHash,
		"ordinary-csrf-"+uuid.NewString(),
		current.Add(24*time.Hour),
		current.Add(30*24*time.Hour),
		current.Add(-20*time.Minute),
	); appErr != nil {
		t.Fatalf("seed ordinary session: %v", appErr)
	}

	before := readAccountAppealAuthSnapshot(t, ctx, store, userID, profile.Provider, profile.Subject)
	service := auth.NewService(store, func() time.Time { return current })
	changedProfile := profile
	changedProfile.Username = "must-not-sync"
	changedProfile.DisplayName = "Must Not Sync"
	changedProfile.AvatarURL = "https://example.com/must-not-sync.png"
	user, first, appErr := service.StartAccountAppealSession(ctx, changedProfile)
	if appErr != nil {
		t.Fatalf("start PostgreSQL account appeal session: %v", appErr)
	}
	if user.ID != userID || user.Status != auth.AccountStatusSuspended {
		t.Fatalf("unexpected restricted account projection: %+v", user)
	}
	if !first.CreatedAt.Equal(current) || !first.ExpiresAt.Equal(current.Add(auth.AccountAppealSessionLifetime)) {
		t.Fatalf("unexpected fixed account appeal lifetime: %+v", first)
	}
	after := readAccountAppealAuthSnapshot(t, ctx, store, userID, profile.Provider, profile.Subject)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("existing-identity-only appeal start mutated ordinary auth facts:\nbefore=%+v\nafter=%+v", before, after)
	}

	var storedTokenHash, storedCSRFHash string
	var storedCreatedAt, storedExpiresAt time.Time
	if err := store.pool.QueryRow(ctx, `
		SELECT session_token_hash, csrf_token_hash, created_at, expires_at
		FROM account_appeal_sessions
		WHERE user_id = $1 AND revoked_at IS NULL
	`, userID).Scan(&storedTokenHash, &storedCSRFHash, &storedCreatedAt, &storedExpiresAt); err != nil {
		t.Fatalf("read stored account appeal session: %v", err)
	}
	if storedTokenHash == first.ID || storedTokenHash != accountAppealTestHash(first.ID) {
		t.Fatalf("raw account appeal session token reached PostgreSQL: stored=%q raw=%q", storedTokenHash, first.ID)
	}
	if storedCSRFHash == first.CSRFToken || storedCSRFHash != accountAppealTestHash(first.CSRFToken) {
		t.Fatalf("raw account appeal CSRF reached PostgreSQL: stored=%q raw=%q", storedCSRFHash, first.CSRFToken)
	}
	if !storedCreatedAt.Equal(first.CreatedAt) || !storedExpiresAt.Equal(first.ExpiresAt) {
		t.Fatalf("stored account appeal lifetime drifted: created=%s expires=%s", storedCreatedAt, storedExpiresAt)
	}

	current = current.Add(time.Minute)
	_, second, appErr := service.StartAccountAppealSession(ctx, profile)
	if appErr != nil {
		t.Fatalf("replace PostgreSQL account appeal session: %v", appErr)
	}
	var totalSessions, liveSessions, revokedSessions int
	if err := store.pool.QueryRow(ctx, `
		SELECT count(*)::int,
		       count(*) FILTER (WHERE revoked_at IS NULL)::int,
		       count(*) FILTER (WHERE revoked_at IS NOT NULL)::int
		FROM account_appeal_sessions
		WHERE user_id = $1
	`, userID).Scan(&totalSessions, &liveSessions, &revokedSessions); err != nil {
		t.Fatalf("inspect replacement account appeal sessions: %v", err)
	}
	if totalSessions != 2 || liveSessions != 1 || revokedSessions != 1 {
		t.Fatalf("account appeal session replacement failed total=%d live=%d revoked=%d", totalSessions, liveSessions, revokedSessions)
	}
	if _, _, appErr := service.GetAccountAppealSession(ctx, first.ID); appErr == nil || appErr.Code != domain.CodeSessionExpired {
		t.Fatalf("replaced PostgreSQL account appeal session remained usable: %v", appErr)
	}

	oldCSRF := second.CSRFToken
	fixedExpiry := second.ExpiresAt
	_, rotated, appErr := service.GetAccountAppealSession(ctx, second.ID)
	if appErr != nil {
		t.Fatalf("rotate PostgreSQL account appeal CSRF: %v", appErr)
	}
	if rotated.CSRFToken == oldCSRF || !rotated.ExpiresAt.Equal(fixedExpiry) {
		t.Fatalf("CSRF rotation renewed or reused state: before=%+v after=%+v", second, rotated)
	}
	if _, _, appErr := service.GetAccountAppealSessionWithCSRF(ctx, second.ID, oldCSRF); appErr == nil || appErr.Code != domain.CodeCSRFTokenInvalid {
		t.Fatalf("old PostgreSQL account appeal CSRF remained valid: %v", appErr)
	}
	if _, validated, appErr := service.GetAccountAppealSessionWithCSRF(ctx, second.ID, rotated.CSRFToken); appErr != nil || !validated.ExpiresAt.Equal(fixedExpiry) {
		t.Fatalf("rotated PostgreSQL account appeal CSRF failed: session=%+v err=%v", validated, appErr)
	}
	var rotatedHash string
	var rotatedExpiresAt time.Time
	if err := store.pool.QueryRow(ctx, `
		SELECT csrf_token_hash, expires_at
		FROM account_appeal_sessions
		WHERE session_token_hash = $1
	`, accountAppealTestHash(second.ID)).Scan(&rotatedHash, &rotatedExpiresAt); err != nil {
		t.Fatalf("inspect rotated account appeal session: %v", err)
	}
	if rotatedHash != accountAppealTestHash(rotated.CSRFToken) || !rotatedExpiresAt.Equal(fixedExpiry) {
		t.Fatalf("PostgreSQL rotation contract drifted hash=%q expires=%s", rotatedHash, rotatedExpiresAt)
	}

	current = fixedExpiry
	if _, _, appErr := service.GetAccountAppealSession(ctx, second.ID); appErr == nil || appErr.Code != domain.CodeSessionExpired {
		t.Fatalf("expired PostgreSQL account appeal session remained usable: %v", appErr)
	}
	unknownProfile := auth.OAuthProfile{Provider: "linux_do", Subject: "unknown-" + suffix, Username: "unknown-" + suffix}
	unknownBefore := readUnknownAccountAppealIdentityCounts(t, ctx, store, unknownProfile)
	assertPostgresAccountAppealIneligible(t, service, unknownProfile)
	unknownAfter := readUnknownAccountAppealIdentityCounts(t, ctx, store, unknownProfile)
	if unknownBefore != unknownAfter {
		t.Fatalf("unknown account appeal identity created auth facts: before=%+v after=%+v", unknownBefore, unknownAfter)
	}
	if _, err := store.pool.Exec(ctx, `UPDATE users SET account_status = 'active' WHERE id = $1`, userID); err != nil {
		t.Fatalf("restore active eligibility fixture: %v", err)
	}
	assertPostgresAccountAppealIneligible(t, service, profile)
	if _, err := store.pool.Exec(ctx, `UPDATE users SET account_status = 'archived' WHERE id = $1`, userID); err != nil {
		t.Fatalf("archive eligibility fixture: %v", err)
	}
	assertPostgresAccountAppealIneligible(t, service, profile)
}

func TestPostgresAccountAppealSessionLifecycleCleanupIsBounded(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 3, 15, 0, 0, 0, time.UTC)
	user, appErr := store.EnsureUser(ctx, "appeal-cleanup-"+strings.ToLower(uuid.NewString()[:8]), false, now)
	if appErr != nil {
		t.Fatalf("ensure cleanup user: %v", appErr)
	}
	t.Cleanup(func() {
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM account_appeal_sessions WHERE user_id = $1`, user.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID)
	})
	for index, createdAt := range []time.Time{now.Add(-3 * time.Hour), now.Add(-2 * time.Hour), now.Add(-90 * time.Minute)} {
		if _, err := store.pool.Exec(ctx, `
			INSERT INTO account_appeal_sessions (
				user_id, session_token_hash, csrf_token_hash, created_at, expires_at
			)
			VALUES ($1, $2, $3, $4::timestamptz, $4::timestamptz + interval '15 minutes')
		`, user.ID, "cleanup-session-"+uuid.NewString(), "cleanup-csrf-"+uuid.NewString(), createdAt); err != nil {
			t.Fatalf("insert cleanup session %d: %v", index, err)
		}
	}
	recentCreatedAt := now.Add(-30 * time.Minute)
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO account_appeal_sessions (
			user_id, session_token_hash, csrf_token_hash, created_at, expires_at, revoked_at
		)
		VALUES ($1, $2, $3, $4::timestamptz, $4::timestamptz + interval '15 minutes', $5)
	`, user.ID, "cleanup-recent-session-"+uuid.NewString(), "cleanup-recent-csrf-"+uuid.NewString(), recentCreatedAt, now.Add(-20*time.Minute)); err != nil {
		t.Fatalf("insert recent revoked cleanup session: %v", err)
	}

	policy := maintenance.Policy{
		SessionRetention:               time.Hour,
		EmailVerificationRetention:     time.Hour,
		ReadNotificationRetention:      time.Hour,
		UnreadNotificationRetention:    2 * time.Hour,
		DomainEventRetention:           time.Hour,
		APIDeliveryCredentialRetention: time.Hour,
		APIProbeSampleRetention:        time.Hour,
	}
	first, appErr := store.RunDataLifecycle(ctx, now, 2, policy)
	if appErr != nil {
		t.Fatalf("run first account appeal cleanup: %v", appErr)
	}
	if first.AccountAppealSessionsDeleted != 2 {
		t.Fatalf("first account appeal cleanup was not bounded: %+v", first)
	}
	second, appErr := store.RunDataLifecycle(ctx, now, 2, policy)
	if appErr != nil {
		t.Fatalf("run second account appeal cleanup: %v", appErr)
	}
	if second.AccountAppealSessionsDeleted != 1 {
		t.Fatalf("second account appeal cleanup did not finish the bounded remainder: %+v", second)
	}
	var remaining int
	if err := store.pool.QueryRow(ctx, `SELECT count(*)::int FROM account_appeal_sessions WHERE user_id = $1`, user.ID).Scan(&remaining); err != nil {
		t.Fatalf("count retained account appeal sessions: %v", err)
	}
	if remaining != 1 {
		t.Fatalf("account appeal cleanup removed the recent revoked row: remaining=%d", remaining)
	}
}

func TestPostgresAdminStatusUsesAccountGovernanceAdvisoryLock(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 3, 16, 0, 0, 0, time.UTC)
	suffix := strings.ToLower(uuid.NewString()[:8])
	adminUser, appErr := store.EnsureUser(ctx, "appeal-lock-admin-"+suffix, true, now)
	if appErr != nil {
		t.Fatalf("ensure advisory-lock admin: %v", appErr)
	}
	targetUser, appErr := store.EnsureUser(ctx, "appeal-lock-user-"+suffix, false, now)
	if appErr != nil {
		t.Fatalf("ensure advisory-lock target: %v", appErr)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		userIDs := []string{adminUser.ID, targetUser.ID}
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM notifications WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM admin_audit_logs WHERE target_id = ANY($1::uuid[]) OR admin_user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM domain_events WHERE aggregate_id = ANY($1::uuid[]) OR actor_user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM idempotency_keys WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM user_permissions WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = ANY($1::uuid[])`, userIDs)
	})

	connection, err := store.pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire advisory lock connection: %v", err)
	}
	defer connection.Release()
	lockTx, err := connection.Begin(ctx)
	if err != nil {
		t.Fatalf("begin advisory lock transaction: %v", err)
	}
	if appErr := lockAccountGovernanceUser(ctx, lockTx, targetUser.ID); appErr != nil {
		_ = lockTx.Rollback(context.Background())
		t.Fatalf("hold account governance advisory lock: %v", appErr)
	}

	authService := auth.NewServiceWithRegistrationEmailSenderAndIdempotency(
		store,
		func() time.Time { return now },
		nil,
		idempotency.NewService(store, func() time.Time { return now }),
	)
	type mutationResult struct {
		completion idempotency.Completion
		appErr     *domain.AppError
	}
	resultCh := make(chan mutationResult, 1)
	go func() {
		completion, mutationErr := authService.UpdateAdminUserStatusWithIdempotency(
			ctx,
			adminUser,
			"POST /api/v1/admin/users/{id}/status:"+targetUser.ID,
			"appeal-lock-"+suffix,
			"appeal-lock-hash-"+suffix,
			auth.AdminUserStatusInput{
				TargetUserID:    targetUser.ID,
				Status:          auth.AccountStatusSuspended,
				ExpectedVersion: 1,
				Reason:          "验证账号治理共享锁",
				RequestID:       "account-appeal-lock-" + suffix,
			},
			func(result auth.AdminUserMutationResult) (idempotency.Completion, *domain.AppError) {
				body, marshalErr := json.Marshal(map[string]any{"id": result.Detail.User.ID})
				if marshalErr != nil {
					return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
				}
				return idempotency.Completion{
					Status:       http.StatusOK,
					ContentType:  "application/json",
					Body:         body,
					ResourceType: "user",
					ResourceID:   result.Detail.User.ID,
				}, nil
			},
		)
		resultCh <- mutationResult{completion: completion, appErr: mutationErr}
	}()

	select {
	case result := <-resultCh:
		_ = lockTx.Rollback(context.Background())
		t.Fatalf("admin status mutation bypassed account governance lock: completion=%+v err=%v", result.completion, result.appErr)
	case <-time.After(150 * time.Millisecond):
	}
	if err := lockTx.Rollback(ctx); err != nil {
		t.Fatalf("release account governance advisory lock: %v", err)
	}
	select {
	case result := <-resultCh:
		if result.appErr != nil || result.completion.Status != http.StatusOK {
			t.Fatalf("admin status mutation failed after advisory lock release: completion=%+v err=%v", result.completion, result.appErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("admin status mutation did not resume after account governance lock release")
	}
}

type accountAppealAuthSnapshot struct {
	Username             string
	DisplayName          string
	AvatarURL            string
	AccountStatus        string
	LastActiveAt         *time.Time
	UserUpdatedAt        time.Time
	UserVersion          int64
	IdentityCreatedAt    time.Time
	IdentityLastLoginAt  *time.Time
	BindingUsername      string
	BindingAvatarURL     string
	BindingLastSyncedAt  time.Time
	OrdinarySessionRows  int
	AttributionRows      int
	ActivityRows         int
	ReferralCodeRows     int
	ReferralRelationRows int
	PromotionCouponRows  int
}

func readAccountAppealAuthSnapshot(t *testing.T, ctx context.Context, store *Store, userID, provider, subject string) accountAppealAuthSnapshot {
	t.Helper()
	var snapshot accountAppealAuthSnapshot
	if err := store.pool.QueryRow(ctx, `
		SELECT username, display_name, COALESCE(avatar_url, ''), account_status,
		       last_active_at, updated_at, version
		FROM users
		WHERE id = $1
	`, userID).Scan(
		&snapshot.Username,
		&snapshot.DisplayName,
		&snapshot.AvatarURL,
		&snapshot.AccountStatus,
		&snapshot.LastActiveAt,
		&snapshot.UserUpdatedAt,
		&snapshot.UserVersion,
	); err != nil {
		t.Fatalf("read account appeal user snapshot: %v", err)
	}
	if err := store.pool.QueryRow(ctx, `
		SELECT created_at, last_login_at
		FROM auth_identities
		WHERE provider = $1 AND provider_subject = $2
	`, auth.CanonicalOAuthProvider(provider), auth.CanonicalOAuthSubject(subject)).Scan(
		&snapshot.IdentityCreatedAt,
		&snapshot.IdentityLastLoginAt,
	); err != nil {
		t.Fatalf("read account appeal identity snapshot: %v", err)
	}
	if err := store.pool.QueryRow(ctx, `
		SELECT linux_do_username, COALESCE(avatar_url, ''), last_synced_at
		FROM linux_do_bindings
		WHERE user_id = $1
	`, userID).Scan(&snapshot.BindingUsername, &snapshot.BindingAvatarURL, &snapshot.BindingLastSyncedAt); err != nil {
		t.Fatalf("read account appeal binding snapshot: %v", err)
	}
	if err := store.pool.QueryRow(ctx, `
		SELECT
		  (SELECT count(*)::int FROM auth_sessions WHERE user_id = $1),
		  (SELECT count(*)::int FROM user_registration_attributions WHERE user_id = $1),
		  (SELECT count(*)::int FROM user_activity_daily WHERE user_id = $1),
		  (SELECT count(*)::int FROM referral_codes WHERE user_id = $1),
		  (SELECT count(*)::int FROM referral_relations WHERE inviter_user_id = $1 OR invitee_user_id = $1),
		  (SELECT count(*)::int FROM promotion_coupons WHERE user_id = $1)
	`, userID).Scan(
		&snapshot.OrdinarySessionRows,
		&snapshot.AttributionRows,
		&snapshot.ActivityRows,
		&snapshot.ReferralCodeRows,
		&snapshot.ReferralRelationRows,
		&snapshot.PromotionCouponRows,
	); err != nil {
		t.Fatalf("read account appeal side-effect snapshot: %v", err)
	}
	return snapshot
}

type unknownAccountAppealIdentityCounts struct {
	Users             int
	Identities        int
	Bindings          int
	OrdinarySessions  int
	DedicatedSessions int
}

func readUnknownAccountAppealIdentityCounts(t *testing.T, ctx context.Context, store *Store, profile auth.OAuthProfile) unknownAccountAppealIdentityCounts {
	t.Helper()
	var counts unknownAccountAppealIdentityCounts
	if err := store.pool.QueryRow(ctx, `
		SELECT
		  (SELECT count(*)::int FROM users WHERE username = $1),
		  (SELECT count(*)::int FROM auth_identities WHERE provider = $2 AND provider_subject = $3),
		  (SELECT count(*)::int FROM linux_do_bindings WHERE linux_do_user_id = $3),
		  (
		    SELECT count(*)::int
		    FROM auth_sessions session
		    JOIN auth_identities identity ON identity.user_id = session.user_id
		    WHERE identity.provider = $2 AND identity.provider_subject = $3
		  ),
		  (
		    SELECT count(*)::int
		    FROM account_appeal_sessions session
		    JOIN auth_identities identity ON identity.user_id = session.user_id
		    WHERE identity.provider = $2 AND identity.provider_subject = $3
		  )
	`,
		auth.OAuthUsernameCandidate(profile.Username, profile.Provider, profile.Subject, 0),
		auth.CanonicalOAuthProvider(profile.Provider),
		auth.CanonicalOAuthSubject(profile.Subject),
	).Scan(
		&counts.Users,
		&counts.Identities,
		&counts.Bindings,
		&counts.OrdinarySessions,
		&counts.DedicatedSessions,
	); err != nil {
		t.Fatalf("read unknown account appeal identity counts: %v", err)
	}
	return counts
}

func assertPostgresAccountAppealIneligible(t *testing.T, service *auth.Service, profile auth.OAuthProfile) {
	t.Helper()
	if _, _, appErr := service.StartAccountAppealSession(context.Background(), profile); appErr == nil || appErr.Code != domain.CodeAccountAppealIneligible {
		t.Fatalf("account appeal eligibility result was distinguishable: %v", appErr)
	}
}

func accountAppealTestHash(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}
