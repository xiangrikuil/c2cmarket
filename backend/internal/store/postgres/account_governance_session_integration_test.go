package postgres

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/google/uuid"
)

func TestPostgresAccountGovernanceOAuthSessionsAndAdminReauthentication(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 13, 16, 0, 0, 0, time.UTC)
	suffix := strings.ToLower(uuid.NewString()[:8])

	restrictedProfile := auth.OAuthProfile{
		Provider: "linux_do", Subject: "restricted-subject-" + suffix,
		Username: "restricted-oauth-" + suffix, LinuxDoUserID: "restricted-subject-" + suffix,
		LinuxDoUsername: "restricted-oauth-" + suffix,
	}
	restrictedResult, appErr := store.UpsertOAuthUser(ctx, restrictedProfile, now.Add(-time.Hour))
	if appErr != nil {
		t.Fatalf("seed restricted OAuth identity: %v", appErr)
	}
	restrictedUserID := restrictedResult.User.ID
	restrictionActionID := uuid.NewString()
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO account_governance_actions (
			id, target_user_id, action_type, status, governance_version,
			reason_code, public_reason, effective_at, is_indefinite,
			actor_user_id, request_id, created_at, updated_at
		)
		VALUES ($1, $2, 'suspend', 'effective', 2,
		        'MANUAL', '集成测试暂停', $3, true,
		        NULL, $4, $3, $3)
	`, restrictionActionID, restrictedUserID, now.Add(-30*time.Minute), "restrict-"+suffix); err != nil {
		t.Fatalf("insert restriction action: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		UPDATE users
		SET account_status = 'suspended', governance_version = 2,
		    current_governance_action_id = $2, updated_at = $3, version = version + 1
		WHERE id = $1
	`, restrictedUserID, restrictionActionID, now.Add(-30*time.Minute)); err != nil {
		t.Fatalf("activate restriction action: %v", err)
	}

	adminProfile := auth.OAuthProfile{
		Provider: "linux_do", Subject: "admin-subject-" + suffix,
		Username: "oauth-admin-" + suffix, LinuxDoUserID: "admin-subject-" + suffix,
		LinuxDoUsername: "oauth-admin-" + suffix,
	}
	adminResult, appErr := store.UpsertOAuthUser(ctx, adminProfile, now.Add(-time.Hour))
	if appErr != nil {
		t.Fatalf("seed OAuth-only administrator: %v", appErr)
	}
	adminUserID := adminResult.User.ID
	if _, err := store.pool.Exec(ctx, `INSERT INTO user_permissions (user_id, permission) VALUES ($1, 'admin')`, adminUserID); err != nil {
		t.Fatalf("grant fixture administrator: %v", err)
	}
	adminRawSession := "admin-session-" + suffix
	adminSessionHash := accountAppealTestHash(adminRawSession)
	if appErr := store.CreateSession(ctx, adminUserID, adminSessionHash, accountAppealTestHash("admin-csrf-"+suffix), now.Add(time.Hour), now.Add(24*time.Hour), now); appErr != nil {
		t.Fatalf("create administrator session: %v", appErr)
	}
	targetUser, appErr := store.EnsureUser(ctx, "admin-target-"+suffix, false, now)
	if appErr != nil {
		t.Fatalf("ensure administrator grant target: %v", appErr)
	}
	secondTarget, appErr := store.EnsureUser(ctx, "admin-second-target-"+suffix, false, now)
	if appErr != nil {
		t.Fatalf("ensure second administrator grant target: %v", appErr)
	}
	userIDs := []string{restrictedUserID, adminUserID, targetUser.ID, secondTarget.ID}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM notifications WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM admin_audit_logs WHERE admin_user_id = ANY($1::uuid[]) OR target_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM domain_events WHERE actor_user_id = ANY($1::uuid[]) OR aggregate_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM idempotency_keys WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM account_appeal_sessions WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM restricted_business_sessions WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM account_governance_expiry_jobs WHERE target_user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `UPDATE users SET current_governance_action_id = NULL WHERE id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM account_governance_actions WHERE target_user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM auth_sessions WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM promotion_coupons WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM referral_relations WHERE inviter_user_id = ANY($1::uuid[]) OR invitee_user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM referral_codes WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM user_activity_daily WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM user_registration_attributions WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM linux_do_bindings WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM auth_identities WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM user_permissions WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM restricted_business_oauth_states WHERE state_hash LIKE $1`, "%"+suffix+"%")
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM account_appeal_oauth_states WHERE state_hash LIKE $1`, "%"+suffix+"%")
	})

	restrictedState := "restricted-state-" + suffix
	appealState := "appeal-state-" + suffix
	if appErr := store.StartRestrictedBusinessOAuth(ctx, restrictedState, now.Add(10*time.Minute), now); appErr != nil {
		t.Fatalf("start restricted OAuth state: %v", appErr)
	}
	if appErr := store.StartAccountAppealOAuth(ctx, appealState, now.Add(10*time.Minute), now); appErr != nil {
		t.Fatalf("start account appeal OAuth state: %v", appErr)
	}
	if _, _, appErr := store.CompleteAccountAppealOAuth(ctx, restrictedState, restrictedProfile, "cross-appeal-session-"+suffix, "cross-appeal-csrf-"+suffix, now.Add(15*time.Minute), now); appErr == nil || appErr.Code != domain.CodeCSRFTokenInvalid {
		t.Fatalf("account appeal consumed restricted state: %v", appErr)
	}
	if _, _, appErr := store.CompleteRestrictedBusinessOAuth(ctx, appealState, restrictedProfile, "cross-restricted-session-"+suffix, "cross-restricted-csrf-"+suffix, now.Add(24*time.Hour), now); appErr == nil || appErr.Code != domain.CodeCSRFTokenInvalid {
		t.Fatalf("restricted flow consumed account appeal state: %v", appErr)
	}
	user, restrictedSession, appErr := store.CompleteRestrictedBusinessOAuth(ctx, restrictedState, restrictedProfile, "restricted-session-"+suffix, "restricted-csrf-"+suffix, now.Add(24*time.Hour), now)
	if appErr != nil || user.ID != restrictedUserID || restrictedSession.GovernanceVersion != 2 || restrictedSession.GovernanceActionID != restrictionActionID {
		t.Fatalf("complete restricted OAuth session user=%+v session=%+v err=%v", user, restrictedSession, appErr)
	}
	if _, _, appErr := store.CompleteRestrictedBusinessOAuth(ctx, restrictedState, restrictedProfile, "replay-session-"+suffix, "replay-csrf-"+suffix, now.Add(24*time.Hour), now); appErr == nil || appErr.Code != domain.CodeCSRFTokenInvalid {
		t.Fatalf("restricted state replay was accepted: %v", appErr)
	}
	if _, appealSession, appErr := store.CompleteAccountAppealOAuth(ctx, appealState, restrictedProfile, "appeal-session-"+suffix, "appeal-csrf-"+suffix, now.Add(15*time.Minute), now); appErr != nil || appealSession.UserID != restrictedUserID {
		t.Fatalf("complete account appeal OAuth session=%+v err=%v", appealSession, appErr)
	}

	unknownState := "unknown-state-" + suffix
	unknownProfile := auth.OAuthProfile{Provider: "linux_do", Subject: "unknown-subject-" + suffix, Username: "unknown-" + suffix}
	if appErr := store.StartRestrictedBusinessOAuth(ctx, unknownState, now.Add(10*time.Minute), now); appErr != nil {
		t.Fatalf("start unknown restricted OAuth state: %v", appErr)
	}
	if _, _, appErr := store.CompleteRestrictedBusinessOAuth(ctx, unknownState, unknownProfile, "unknown-session-"+suffix, "unknown-csrf-"+suffix, now.Add(24*time.Hour), now); appErr == nil || appErr.Code != domain.CodeAccountRestricted {
		t.Fatalf("unknown restricted identity was accepted: %v", appErr)
	}
	var unknownUsers, unknownIdentities, unknownSessions int
	if err := store.pool.QueryRow(ctx, `SELECT count(*)::int FROM users WHERE username = $1`, unknownProfile.Username).Scan(&unknownUsers); err != nil {
		t.Fatalf("count unknown users: %v", err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT count(*)::int FROM auth_identities WHERE provider = 'linux_do' AND provider_subject = $1`, unknownProfile.Subject).Scan(&unknownIdentities); err != nil {
		t.Fatalf("count unknown identities: %v", err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT count(*)::int FROM restricted_business_sessions WHERE session_token_hash = $1`, "unknown-session-"+suffix).Scan(&unknownSessions); err != nil {
		t.Fatalf("count unknown sessions: %v", err)
	}
	if unknownUsers != 0 || unknownIdentities != 0 || unknownSessions != 0 {
		t.Fatalf("unknown restricted identity created auth facts users=%d identities=%d sessions=%d", unknownUsers, unknownIdentities, unknownSessions)
	}

	nextActionID := uuid.NewString()
	if _, err := store.pool.Exec(ctx, `UPDATE account_governance_actions SET status = 'superseded', superseded_at = $2, updated_at = $2 WHERE id = $1`, restrictionActionID, now.Add(time.Minute)); err != nil {
		t.Fatalf("supersede restricted action: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO account_governance_actions (
			id, target_user_id, action_type, status, governance_version,
			reason_code, public_reason, effective_at, is_indefinite,
			supersedes_action_id, actor_user_id, request_id, created_at, updated_at
		)
		VALUES ($1, $2, 'ban', 'effective', 3, 'MANUAL', '集成测试封禁', $3, false, $4, NULL, $5, $3, $3)
	`, nextActionID, restrictedUserID, now.Add(time.Minute), restrictionActionID, "ban-"+suffix); err != nil {
		t.Fatalf("insert replacement governance action: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `UPDATE users SET account_status = 'banned', governance_version = 3, current_governance_action_id = $2 WHERE id = $1`, restrictedUserID, nextActionID); err != nil {
		t.Fatalf("activate replacement governance action: %v", err)
	}
	if _, _, appErr := store.GetRestrictedBusinessSession(ctx, "restricted-session-"+suffix, now.Add(time.Minute)); appErr == nil || appErr.Code != domain.CodeSessionExpired {
		t.Fatalf("superseded restricted session remained readable: %v", appErr)
	}
	if _, _, appErr := store.RotateRestrictedBusinessSessionCSRF(ctx, "restricted-session-"+suffix, "rotated-csrf-"+suffix, now.Add(time.Minute)); appErr == nil || appErr.Code != domain.CodeSessionExpired {
		t.Fatalf("superseded restricted session CSRF rotated: %v", appErr)
	}

	adminState := "admin-reauth-state-" + suffix
	if appErr := store.StartAdminReauthenticationOAuth(ctx, adminSessionHash, adminState, auth.OAuthPurposeGrantAdminReauthentication, now.Add(10*time.Minute), now); appErr != nil {
		t.Fatalf("start administrator OAuth reauthentication: %v", appErr)
	}
	wrongAdminProfile := adminProfile
	wrongAdminProfile.Subject = "other-admin-subject-" + suffix
	if _, appErr := store.CompleteAdminReauthenticationOAuth(ctx, adminSessionHash, adminState, wrongAdminProfile, now, now.Add(10*time.Minute)); appErr == nil || appErr.Code != domain.CodeRecentReauthenticationRequired {
		t.Fatalf("mismatched administrator OAuth identity was accepted: %v", appErr)
	}
	grant, appErr := store.CompleteAdminReauthenticationOAuth(ctx, adminSessionHash, adminState, adminProfile, now, now.Add(10*time.Minute))
	if appErr != nil || grant.AdminUserID != adminUserID || grant.Method != auth.AdminReauthenticationMethodLinuxDoOAuth {
		t.Fatalf("complete administrator OAuth reauthentication grant=%+v err=%v", grant, appErr)
	}
	if _, appErr := store.CompleteAdminReauthenticationOAuth(ctx, adminSessionHash, adminState, adminProfile, now, now.Add(10*time.Minute)); appErr == nil || appErr.Code != domain.CodeRecentReauthenticationRequired {
		t.Fatalf("administrator OAuth state replay was accepted: %v", appErr)
	}

	adminUser, appErr := store.UserByID(ctx, adminUserID)
	if appErr != nil {
		t.Fatalf("reload administrator: %v", appErr)
	}
	idempotencyService := idempotency.NewService(store, func() time.Time { return now })
	authService := auth.NewServiceWithRegistrationEmailSenderAndIdempotency(store, func() time.Time { return now }, nil, idempotencyService)
	targetDetail, appErr := authService.AdminUser(ctx, adminUser, targetUser.ID)
	if appErr != nil {
		t.Fatalf("load administrator grant target: %v", appErr)
	}
	completionBuilder := func(result auth.AdminUserMutationResult) (idempotency.Completion, *domain.AppError) {
		return idempotency.Completion{Status: http.StatusOK, ContentType: "application/json", Body: []byte(`{}`), ResourceType: "user", ResourceID: result.Detail.User.ID}, nil
	}
	if _, appErr := authService.UpdateAdminUserPermissionWithIdempotency(ctx, adminUser,
		"POST /api/v1/admin/users/{id}/admin-permission:"+targetUser.ID,
		"stale-version-"+suffix, "stale-version-hash-"+suffix,
		auth.AdminUserPermissionInput{TargetUserID: targetUser.ID, Grant: true, ExpectedVersion: targetDetail.User.Version + 1, Reason: "验证失败不消耗重验", AdminSessionTokenHash: adminRawSession, RequestID: "stale-version-" + suffix}, completionBuilder,
	); appErr == nil || appErr.Code != domain.CodeVersionConflict {
		t.Fatalf("stale administrator grant did not fail with version conflict: %v", appErr)
	}
	if _, appErr := authService.UpdateAdminUserPermissionWithIdempotency(ctx, adminUser,
		"POST /api/v1/admin/users/{id}/admin-permission:"+targetUser.ID,
		"grant-admin-"+suffix, "grant-admin-hash-"+suffix,
		auth.AdminUserPermissionInput{TargetUserID: targetUser.ID, Grant: true, ExpectedVersion: targetDetail.User.Version, Reason: "验证一次性重验授权", AdminSessionTokenHash: adminRawSession, RequestID: "grant-admin-" + suffix}, completionBuilder,
	); appErr != nil {
		t.Fatalf("consume administrator reauthentication grant: %v", appErr)
	}
	secondDetail, appErr := authService.AdminUser(ctx, adminUser, secondTarget.ID)
	if appErr != nil {
		t.Fatalf("load second administrator grant target: %v", appErr)
	}
	if _, appErr := authService.UpdateAdminUserPermissionWithIdempotency(ctx, adminUser,
		"POST /api/v1/admin/users/{id}/admin-permission:"+secondTarget.ID,
		"replay-grant-"+suffix, "replay-grant-hash-"+suffix,
		auth.AdminUserPermissionInput{TargetUserID: secondTarget.ID, Grant: true, ExpectedVersion: secondDetail.User.Version, Reason: "验证重验授权不可重放", AdminSessionTokenHash: adminRawSession, RequestID: "replay-grant-" + suffix}, completionBuilder,
	); appErr == nil || appErr.Code != domain.CodeRecentReauthenticationRequired {
		t.Fatalf("consumed administrator grant was reused: %v", appErr)
	}
}

func TestPostgresAccountGovernanceSuspensionExpiryIsExactAndIdempotent(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 13, 18, 0, 0, 0, time.UTC)
	suffix := strings.ToLower(uuid.NewString()[:8])
	admin, appErr := store.EnsureUser(ctx, "expiry-admin-"+suffix, true, now.Add(-time.Hour))
	if appErr != nil {
		t.Fatalf("ensure expiry administrator: %v", appErr)
	}
	restoredUser, appErr := store.EnsureUser(ctx, "expiry-restored-"+suffix, false, now.Add(-time.Hour))
	if appErr != nil {
		t.Fatalf("ensure expiry restore user: %v", appErr)
	}
	supersededUser, appErr := store.EnsureUser(ctx, "expiry-superseded-"+suffix, false, now.Add(-time.Hour))
	if appErr != nil {
		t.Fatalf("ensure expiry superseded user: %v", appErr)
	}
	userIDs := []string{admin.ID, restoredUser.ID, supersededUser.ID}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM notifications WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM domain_events WHERE actor_user_id = ANY($1::uuid[]) OR aggregate_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM account_governance_expiry_jobs WHERE target_user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM restricted_business_sessions WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `UPDATE users SET current_governance_action_id = NULL WHERE id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM account_governance_actions WHERE target_user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM auth_sessions WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM user_permissions WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = ANY($1::uuid[])`, userIDs)
	})

	insertSuspension := func(userID string, version int64, expiresAt time.Time, label string) string {
		t.Helper()
		actionID := uuid.NewString()
		jobID := uuid.NewString()
		effectiveAt := now.Add(-time.Hour)
		if _, err := store.pool.Exec(ctx, `
			INSERT INTO account_governance_actions (
				id, target_user_id, action_type, status, governance_version,
				reason_code, public_reason, effective_at, expires_at, is_indefinite,
				actor_user_id, request_id, created_at, updated_at
			)
			VALUES ($1, $2, 'suspend', 'effective', $3, 'MANUAL', $4, $5, $6, false, $7, $8, $5, $5)
		`, actionID, userID, version, "暂停 "+label, effectiveAt, expiresAt, admin.ID, "suspend-"+label+"-"+suffix); err != nil {
			t.Fatalf("insert %s suspension: %v", label, err)
		}
		if _, err := store.pool.Exec(ctx, `UPDATE users SET account_status = 'suspended', governance_version = $2, current_governance_action_id = $3, version = version + 1 WHERE id = $1`, userID, version, actionID); err != nil {
			t.Fatalf("activate %s suspension: %v", label, err)
		}
		if _, err := store.pool.Exec(ctx, `
			INSERT INTO account_governance_expiry_jobs (
				id, target_user_id, suspension_action_id, expected_governance_version,
				expected_expires_at, available_at, created_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $5, $6, $6)
		`, jobID, userID, actionID, version, expiresAt, effectiveAt); err != nil {
			t.Fatalf("insert %s expiry job: %v", label, err)
		}
		return actionID
	}
	restoredActionID := insertSuspension(restoredUser.ID, 2, now.Add(-time.Minute), "restored")
	supersededActionID := insertSuspension(supersededUser.ID, 2, now.Add(-time.Minute), "superseded")
	oldOAuthStateCreatedAt := now.Add(-31 * 24 * time.Hour)
	oldOAuthStateConsumedAt := oldOAuthStateCreatedAt.Add(time.Minute)
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO restricted_business_oauth_states (state_hash, created_at, expires_at, consumed_at)
		VALUES ($1, $2::timestamptz, $2::timestamptz + interval '10 minutes', $3::timestamptz)
	`, "expiry-restricted-oauth-"+suffix, oldOAuthStateCreatedAt, oldOAuthStateConsumedAt); err != nil {
		t.Fatalf("insert expired restricted OAuth state: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO account_appeal_oauth_states (state_hash, created_at, expires_at, consumed_at)
		VALUES ($1, $2::timestamptz, $2::timestamptz + interval '10 minutes', $3::timestamptz)
	`, "expiry-appeal-oauth-"+suffix, oldOAuthStateCreatedAt, oldOAuthStateConsumedAt); err != nil {
		t.Fatalf("insert expired account appeal OAuth state: %v", err)
	}
	var adminSessionID string
	adminSessionHash := "expiry-admin-session-" + suffix
	if appErr := store.CreateSession(ctx, admin.ID, adminSessionHash, "expiry-admin-csrf-"+suffix, now.Add(24*time.Hour), now.Add(30*24*time.Hour), now); appErr != nil {
		t.Fatalf("create administrator OAuth-state session: %v", appErr)
	}
	if err := store.pool.QueryRow(ctx, `SELECT id::text FROM auth_sessions WHERE session_token_hash = $1`, adminSessionHash).Scan(&adminSessionID); err != nil {
		t.Fatalf("read administrator OAuth-state session: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO admin_reauthentication_oauth_states (
			admin_user_id, auth_session_id, state_hash, purpose,
			created_at, expires_at, consumed_at
		)
		VALUES ($1, $2, $3, 'grant_admin_reauth', $4::timestamptz, $4::timestamptz + interval '10 minutes', $5::timestamptz)
	`, admin.ID, adminSessionID, "expiry-admin-oauth-"+suffix, oldOAuthStateCreatedAt, oldOAuthStateConsumedAt); err != nil {
		t.Fatalf("insert expired administrator OAuth state: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `UPDATE account_governance_actions SET status = 'superseded', superseded_at = $2, updated_at = $2 WHERE id = $1`, supersededActionID, now.Add(-30*time.Minute)); err != nil {
		t.Fatalf("supersede old suspension: %v", err)
	}
	banActionID := uuid.NewString()
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO account_governance_actions (
			id, target_user_id, action_type, status, governance_version,
			reason_code, public_reason, effective_at, is_indefinite,
			supersedes_action_id, actor_user_id, request_id, created_at, updated_at
		)
		VALUES ($1, $2, 'ban', 'effective', 3, 'MANUAL', '后续封禁', $3, false, $4, $5, $6, $3, $3)
	`, banActionID, supersededUser.ID, now.Add(-30*time.Minute), supersededActionID, admin.ID, "ban-expiry-"+suffix); err != nil {
		t.Fatalf("insert replacement ban: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `UPDATE users SET account_status = 'banned', governance_version = 3, current_governance_action_id = $2 WHERE id = $1`, supersededUser.ID, banActionID); err != nil {
		t.Fatalf("activate replacement ban: %v", err)
	}

	policy := lifecycleCredentialPolicy()
	result, appErr := store.RunDataLifecycle(ctx, now, 100, policy)
	if appErr != nil {
		t.Fatalf("run governance suspension expiry: %v", appErr)
	}
	if result.GovernanceSuspensionsRestored != 1 || result.GovernanceExpiryJobsSuperseded != 1 || result.GovernanceOAuthStatesDeleted != 3 {
		t.Fatalf("unexpected suspension expiry result: %+v", result)
	}
	var restoredStatus, restoredCurrentAction, oldActionStatus, restoredJobStatus string
	var restoredGovernanceVersion int64
	var restoredIsAdmin bool
	if err := store.pool.QueryRow(ctx, `
		SELECT user_account.account_status, user_account.governance_version,
		       user_account.current_governance_action_id::text,
		       EXISTS(SELECT 1 FROM user_permissions permission WHERE permission.user_id = user_account.id AND permission.permission = 'admin'),
		       old_action.status, expiry_job.status
		FROM users user_account
		JOIN account_governance_actions old_action ON old_action.id = $2
		JOIN account_governance_expiry_jobs expiry_job ON expiry_job.suspension_action_id = old_action.id
		WHERE user_account.id = $1
	`, restoredUser.ID, restoredActionID).Scan(&restoredStatus, &restoredGovernanceVersion, &restoredCurrentAction, &restoredIsAdmin, &oldActionStatus, &restoredJobStatus); err != nil {
		t.Fatalf("read restored suspension state: %v", err)
	}
	if restoredStatus != auth.AccountStatusActive || restoredGovernanceVersion != 3 || restoredCurrentAction == restoredActionID || restoredIsAdmin || oldActionStatus != "superseded" || restoredJobStatus != "restored" {
		t.Fatalf("unexpected restored suspension status=%s governance=%d action=%s admin=%t old=%s job=%s", restoredStatus, restoredGovernanceVersion, restoredCurrentAction, restoredIsAdmin, oldActionStatus, restoredJobStatus)
	}
	var supersededStatus, supersededJobStatus string
	if err := store.pool.QueryRow(ctx, `
		SELECT user_account.account_status, expiry_job.status
		FROM users user_account
		JOIN account_governance_expiry_jobs expiry_job ON expiry_job.target_user_id = user_account.id
		WHERE user_account.id = $1
	`, supersededUser.ID).Scan(&supersededStatus, &supersededJobStatus); err != nil {
		t.Fatalf("read superseded suspension state: %v", err)
	}
	if supersededStatus != auth.AccountStatusBanned || supersededJobStatus != "noop_superseded" {
		t.Fatalf("old expiry job changed later governance status=%s job=%s", supersededStatus, supersededJobStatus)
	}
	var restorationNotifications int
	if err := store.pool.QueryRow(ctx, `SELECT count(*)::int FROM notifications WHERE user_id = $1 AND type = 'user.account_suspension_expired'`, restoredUser.ID).Scan(&restorationNotifications); err != nil {
		t.Fatalf("count restoration notifications: %v", err)
	}
	if restorationNotifications != 1 {
		t.Fatalf("expected one restoration notification, got %d", restorationNotifications)
	}
	replay, appErr := store.RunDataLifecycle(ctx, now.Add(time.Minute), 100, policy)
	if appErr != nil {
		t.Fatalf("rerun governance suspension expiry: %v", appErr)
	}
	if replay.GovernanceSuspensionsRestored != 0 || replay.GovernanceExpiryJobsSuperseded != 0 {
		t.Fatalf("governance expiry rerun was not idempotent: %+v", replay)
	}
	if err := store.pool.QueryRow(ctx, `SELECT count(*)::int FROM notifications WHERE user_id = $1 AND type = 'user.account_suspension_expired'`, restoredUser.ID).Scan(&restorationNotifications); err != nil || restorationNotifications != 1 {
		t.Fatalf("rerun duplicated restoration notification count=%d err=%v", restorationNotifications, err)
	}
}
