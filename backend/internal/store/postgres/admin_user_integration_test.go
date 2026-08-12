package postgres

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/google/uuid"
)

func TestPostgresAdminUserDirectoryAndGovernance(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	suffix := strings.ToLower(uuid.NewString()[:8])
	adminUser, appErr := store.EnsureUser(ctx, "admin-directory-"+suffix, true, now)
	if appErr != nil {
		t.Fatalf("ensure admin: %v", appErr)
	}
	targetUser, appErr := store.EnsureUser(ctx, "target-directory-"+suffix, false, now.Add(time.Minute))
	if appErr != nil {
		t.Fatalf("ensure target: %v", appErr)
	}
	userIDs := []string{adminUser.ID, targetUser.ID}
	for index := 0; index < 23; index++ {
		user, appErr := store.EnsureUser(ctx, "page-"+suffix+"-"+string(rune('a'+index)), false, now.Add(time.Duration(index+2)*time.Minute))
		if appErr != nil {
			t.Fatalf("ensure page fixture %d: %v", index, appErr)
		}
		userIDs = append(userIDs, user.ID)
	}
	t.Cleanup(func() {
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM notifications WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM admin_audit_logs WHERE target_id = ANY($1::uuid[]) OR admin_user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM domain_events WHERE aggregate_id = ANY($1::uuid[]) OR actor_user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM idempotency_keys WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM auth_sessions WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM user_permissions WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM users WHERE id = ANY($1::uuid[])`, userIDs)
	})

	directory, appErr := store.ListAdminUsers(ctx, auth.AdminUserDirectoryQuery{
		Page:    2,
		Limit:   20,
		Search:  suffix,
		Status:  auth.AdminUserStatusAll,
		Role:    auth.AdminUserRoleAll,
		LinuxDo: auth.AdminUserLinuxDoAll,
		Sort:    auth.AdminUserSortUsernameAsc,
	})
	if appErr != nil {
		t.Fatalf("list directory: %v", appErr)
	}
	if len(directory.Items) != 5 || directory.Pagination.TotalItems != 25 || directory.Pagination.TotalPages != 2 {
		t.Fatalf("unexpected bounded directory: %+v", directory)
	}
	if directory.Summary.TotalUsers < 25 || directory.Summary.AdminUsers < 1 {
		t.Fatalf("unexpected authoritative summary: %+v", directory.Summary)
	}

	if appErr := store.CreateSession(ctx, targetUser.ID, "admin-user-session-"+suffix, "admin-user-csrf-"+suffix, now.Add(24*time.Hour), now.Add(30*24*time.Hour), now); appErr != nil {
		t.Fatalf("create target session: %v", appErr)
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO admin_audit_logs (
			admin_user_id, action, target_type, target_id, reason, request_id, created_at
		)
		VALUES ($1, 'reputation.recalculated', 'user', $2, '不属于账号治理详情', $3, $4)
	`, adminUser.ID, targetUser.ID, "unrelated-admin-audit-"+suffix, now); err != nil {
		t.Fatalf("insert unrelated user audit: %v", err)
	}
	idempotencyService := idempotency.NewService(store, func() time.Time { return now })
	authService := auth.NewServiceWithRegistrationEmailSenderAndIdempotency(store, func() time.Time { return now }, nil, idempotencyService)
	detail, appErr := authService.AdminUser(ctx, adminUser, targetUser.ID)
	if appErr != nil {
		t.Fatalf("load target detail: %v", appErr)
	}
	if len(detail.RecentAuditEntries) != 0 {
		t.Fatalf("account detail exposed unrelated user audit entries: %+v", detail.RecentAuditEntries)
	}
	completionBuilder := func(result auth.AdminUserMutationResult) (idempotency.Completion, *domain.AppError) {
		body, err := json.Marshal(map[string]any{"id": result.Detail.User.ID, "version": result.Detail.User.Version})
		if err != nil {
			t.Fatalf("marshal completion: %v", err)
		}
		return idempotency.Completion{Status: http.StatusOK, ContentType: "application/json", Body: body, ResourceType: "user", ResourceID: result.Detail.User.ID}, nil
	}
	completion, appErr := authService.UpdateAdminUserStatusWithIdempotency(
		ctx,
		adminUser,
		"POST /api/v1/admin/users/{id}/status:"+targetUser.ID,
		"suspend-"+suffix,
		"request-hash-"+suffix,
		auth.AdminUserStatusInput{
			TargetUserID:    targetUser.ID,
			Status:          auth.AccountStatusSuspended,
			ExpectedVersion: detail.User.Version,
			Reason:          "PostgreSQL 集成核查",
			RequestID:       "postgres-admin-user-" + suffix,
		},
		completionBuilder,
	)
	if appErr != nil || completion.Status != http.StatusOK {
		t.Fatalf("suspend target completion=%+v err=%v", completion, appErr)
	}

	updated, appErr := store.AdminUserDetail(ctx, targetUser.ID)
	if appErr != nil {
		t.Fatalf("load updated target: %v", appErr)
	}
	if updated.User.Status != auth.AccountStatusSuspended || updated.User.Version != detail.User.Version+1 || updated.ActiveSessionCount != 0 || len(updated.RecentAuditEntries) != 1 {
		t.Fatalf("unexpected updated target: %+v", updated)
	}
	var revokedSessions, events, audits, notifications, completedKeys int
	if err := store.pool.QueryRow(ctx, `SELECT count(*)::int FROM auth_sessions WHERE user_id = $1 AND revoked_at IS NOT NULL`, targetUser.ID).Scan(&revokedSessions); err != nil {
		t.Fatalf("count revoked sessions: %v", err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT count(*)::int FROM domain_events WHERE aggregate_type = 'user' AND aggregate_id = $1`, targetUser.ID).Scan(&events); err != nil {
		t.Fatalf("count domain events: %v", err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT count(*)::int FROM admin_audit_logs WHERE target_type = 'user' AND target_id = $1 AND action = 'user.account_status_changed'`, targetUser.ID).Scan(&audits); err != nil {
		t.Fatalf("count audit logs: %v", err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT count(*)::int FROM notifications WHERE user_id = $1 AND target_type = 'user'`, targetUser.ID).Scan(&notifications); err != nil {
		t.Fatalf("count notifications: %v", err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT count(*)::int FROM idempotency_keys WHERE user_id = $1 AND status = 'completed'`, adminUser.ID).Scan(&completedKeys); err != nil {
		t.Fatalf("count completed idempotency keys: %v", err)
	}
	if revokedSessions != 1 || events != 1 || audits != 1 || notifications != 1 || completedKeys != 1 {
		t.Fatalf("missing transactional side effects sessions=%d events=%d audits=%d notifications=%d idempotency=%d", revokedSessions, events, audits, notifications, completedKeys)
	}

	auditPage, appErr := store.ListAdminAuditLogs(ctx, auth.AdminAuditLogFilter{
		Action:      "user.account_status_changed",
		TargetType:  "user",
		ActorUserID: adminUser.ID,
		TargetID:    targetUser.ID,
		Search:      "PostgreSQL 集成核查",
	}, domain.PageRequest{Limit: 1})
	if appErr != nil || len(auditPage.Items) != 1 || auditPage.NextCursor != nil {
		t.Fatalf("unexpected filtered audit page: %+v err=%v", auditPage, appErr)
	}
	auditItem := auditPage.Items[0]
	if auditItem.ActorUsername != adminUser.Username || auditItem.BeforeStatus == nil || *auditItem.BeforeStatus != auth.AccountStatusActive || auditItem.AfterStatus == nil || *auditItem.AfterStatus != auth.AccountStatusSuspended {
		t.Fatalf("unexpected safe audit projection: %+v", auditItem)
	}
	firstAuditPage, appErr := store.ListAdminAuditLogs(ctx, auth.AdminAuditLogFilter{TargetType: "user", TargetID: targetUser.ID}, domain.PageRequest{Limit: 1})
	if appErr != nil || len(firstAuditPage.Items) != 1 || firstAuditPage.NextCursor == nil {
		t.Fatalf("unexpected first audit cursor page: %+v err=%v", firstAuditPage, appErr)
	}
	secondAuditPage, appErr := store.ListAdminAuditLogs(ctx, auth.AdminAuditLogFilter{TargetType: "user", TargetID: targetUser.ID}, domain.PageRequest{Limit: 1, Cursor: *firstAuditPage.NextCursor})
	if appErr != nil || len(secondAuditPage.Items) != 1 || secondAuditPage.NextCursor != nil || secondAuditPage.Items[0].ID == firstAuditPage.Items[0].ID {
		t.Fatalf("unexpected second audit cursor page: %+v err=%v", secondAuditPage, appErr)
	}

	secondAdmin, appErr := store.EnsureUser(ctx, "second-admin-"+suffix, true, now.Add(time.Hour))
	if appErr != nil {
		t.Fatalf("ensure second admin: %v", appErr)
	}
	userIDs = append(userIDs, secondAdmin.ID)
	secondAdminDetail, appErr := authService.AdminUser(ctx, adminUser, secondAdmin.ID)
	if appErr != nil {
		t.Fatalf("load second admin: %v", appErr)
	}
	if _, appErr := authService.UpdateAdminUserPermissionWithIdempotency(
		ctx,
		adminUser,
		"POST /api/v1/admin/users/{id}/admin-permission:"+secondAdmin.ID,
		"demote-"+suffix,
		"demote-hash-"+suffix,
		auth.AdminUserPermissionInput{
			TargetUserID:    secondAdmin.ID,
			Grant:           false,
			ExpectedVersion: secondAdminDetail.User.Version,
			Reason:          "结束集成测试值班",
			RequestID:       "postgres-demote-" + suffix,
		},
		completionBuilder,
	); appErr != nil {
		t.Fatalf("demote second admin: %v", appErr)
	}
	if _, appErr := authService.UpdateAdminUserStatusWithIdempotency(
		ctx,
		auth.User{ID: secondAdmin.ID, IsAdmin: true},
		"POST /api/v1/admin/users/{id}/status:"+adminUser.ID,
		"last-admin-"+suffix,
		"last-admin-hash-"+suffix,
		auth.AdminUserStatusInput{
			TargetUserID:    adminUser.ID,
			Status:          auth.AccountStatusArchived,
			ExpectedVersion: 1,
			Reason:          "验证最后管理员保护",
			RequestID:       "postgres-last-admin-" + suffix,
		},
		completionBuilder,
	); appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("expected PostgreSQL last-admin protection, got %v", appErr)
	}
}

func TestPostgresStudentOnlyAccountCannotReceiveAdminPermission(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 13, 9, 0, 0, 0, time.UTC)
	suffix := strings.ToLower(uuid.NewString()[:8])
	adminUser, appErr := store.EnsureUser(ctx, "student-admin-guard-"+suffix, true, now)
	if appErr != nil {
		t.Fatalf("ensure admin: %v", appErr)
	}

	var studentUserID string
	if err := store.pool.QueryRow(ctx, `
		INSERT INTO users (username, display_name, account_status, created_at, updated_at)
		VALUES ($1, $1, 'active', $2, $2)
		RETURNING id::text
	`, "student-admin-target-"+suffix, now).Scan(&studentUserID); err != nil {
		t.Fatalf("insert student user: %v", err)
	}
	domainID := uuid.NewString()
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO student_institution_domains (
			id, domain, institution_name, enabled, created_by_admin_id, updated_by_admin_id,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, true, $4, $4, $5, $5)
	`, domainID, "admin-guard-"+suffix+".example.edu", "Admin Guard University", adminUser.ID, now); err != nil {
		t.Fatalf("insert student institution: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO student_email_claims (
			id, user_id, normalized_email, institution_domain_id, claimed_at
		)
		VALUES ($1, $2, $3, $4, $5)
	`, uuid.NewString(), studentUserID, "student-"+suffix+"@admin-guard-"+suffix+".example.edu", domainID, now); err != nil {
		t.Fatalf("insert student claim: %v", err)
	}

	idempotencyService := idempotency.NewService(store, func() time.Time { return now })
	authService := auth.NewServiceWithRegistrationEmailSenderAndIdempotency(store, func() time.Time { return now }, nil, idempotencyService)
	detail, appErr := authService.AdminUser(ctx, adminUser, studentUserID)
	if appErr != nil {
		t.Fatalf("load student account: %v", appErr)
	}
	completionBuilder := func(result auth.AdminUserMutationResult) (idempotency.Completion, *domain.AppError) {
		body, err := json.Marshal(map[string]any{"id": result.Detail.User.ID, "version": result.Detail.User.Version})
		if err != nil {
			t.Fatalf("marshal completion: %v", err)
		}
		return idempotency.Completion{
			Status: http.StatusOK, ContentType: "application/json", Body: body,
			ResourceType: "user", ResourceID: result.Detail.User.ID,
		}, nil
	}
	key := "student-admin-grant-" + suffix
	if _, appErr := authService.UpdateAdminUserPermissionWithIdempotency(
		ctx,
		adminUser,
		"POST /api/v1/admin/users/{id}/admin-permission:"+studentUserID,
		key,
		"student-admin-grant-hash-"+suffix,
		auth.AdminUserPermissionInput{
			TargetUserID: studentUserID, Grant: true, ExpectedVersion: detail.User.Version,
			Reason: "验证高校邮箱账号管理员边界", RequestID: "student-admin-grant-" + suffix,
		},
		completionBuilder,
	); appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("expected student-only admin grant rejection, got %v", appErr)
	}

	var permissions, audits, events, completedKeys int
	if err := store.pool.QueryRow(ctx, `SELECT count(*)::int FROM user_permissions WHERE user_id = $1 AND permission = 'admin'`, studentUserID).Scan(&permissions); err != nil {
		t.Fatalf("count student permissions: %v", err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT count(*)::int FROM admin_audit_logs WHERE target_type = 'user' AND target_id = $1 AND action = 'user.admin_permission_changed'`, studentUserID).Scan(&audits); err != nil {
		t.Fatalf("count student admin audits: %v", err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT count(*)::int FROM domain_events WHERE aggregate_type = 'user' AND aggregate_id = $1 AND event_type = 'user.admin_permission_changed'`, studentUserID).Scan(&events); err != nil {
		t.Fatalf("count student admin events: %v", err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT count(*)::int FROM idempotency_keys WHERE user_id = $1 AND idempotency_key = $2 AND status = 'completed'`, adminUser.ID, key).Scan(&completedKeys); err != nil {
		t.Fatalf("count completed student admin keys: %v", err)
	}
	if permissions != 0 || audits != 0 || events != 0 || completedKeys != 0 {
		t.Fatalf("rejected grant left side effects permissions=%d audits=%d events=%d completed=%d", permissions, audits, events, completedKeys)
	}

	studentUser, appErr := store.UserByID(ctx, studentUserID)
	if appErr != nil {
		t.Fatalf("reload student account: %v", appErr)
	}
	if studentUser.IsAdmin || auth.HasCapability(studentUser, auth.CapabilityAdminAccess) {
		t.Fatalf("student-only account gained admin authority: %+v", studentUser)
	}
}

func TestPostgresAdminGrantAndLinuxDoLinkUseOneLockOrder(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	suffix := strings.ToLower(uuid.NewString()[:8])
	adminUser, appErr := store.EnsureUser(ctx, "link-admin-guard-"+suffix, true, now)
	if appErr != nil {
		t.Fatalf("ensure admin: %v", appErr)
	}

	var studentUserID string
	studentUsername := "link-student-target-" + suffix
	if err := store.pool.QueryRow(ctx, `
		INSERT INTO users (username, display_name, account_status, created_at, updated_at)
		VALUES ($1, $1, 'active', $2, $2)
		RETURNING id::text
	`, studentUsername, now).Scan(&studentUserID); err != nil {
		t.Fatalf("insert student user: %v", err)
	}
	domainID := uuid.NewString()
	domainValue := "link-guard-" + suffix + ".example.edu"
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO student_institution_domains (
			id, domain, institution_name, enabled, created_by_admin_id, updated_by_admin_id,
			created_at, updated_at
		)
		VALUES ($1, $2, $3, true, $4, $4, $5, $5)
	`, domainID, domainValue, "Link Guard University", adminUser.ID, now); err != nil {
		t.Fatalf("insert student institution: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO student_email_claims (
			id, user_id, normalized_email, institution_domain_id, claimed_at
		)
		VALUES ($1, $2, $3, $4, $5)
	`, uuid.NewString(), studentUserID, "student@"+domainValue, domainID, now); err != nil {
		t.Fatalf("insert student claim: %v", err)
	}

	sessionHash := "link-session-hash-" + suffix
	stateHash := "link-state-hash-" + suffix
	createdAt := now.Add(-time.Minute)
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO auth_sessions (
			user_id, session_token_hash, csrf_token_hash, expires_at, renewed_at,
			absolute_expires_at, created_at, updated_at, last_seen_at,
			password_reauthenticated_at, oauth_link_state_hash,
			oauth_link_state_purpose, oauth_link_state_expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $5, $5, $5, $7, $8, 'link_linuxdo', $9)
	`, studentUserID, sessionHash, "link-csrf-hash-"+suffix, now.Add(time.Hour), createdAt,
		now.Add(24*time.Hour), now, stateHash, now.Add(10*time.Minute)); err != nil {
		t.Fatalf("insert link session: %v", err)
	}

	const blockerLockKey int64 = 73429108341
	if _, err := store.pool.Exec(ctx, `
		CREATE OR REPLACE FUNCTION test_block_linuxdo_binding_insert()
		RETURNS trigger
		LANGUAGE plpgsql
		AS $function$
		BEGIN
		  PERFORM pg_advisory_xact_lock(73429108341);
		  RETURN NEW;
		END;
		$function$;
		DROP TRIGGER IF EXISTS test_block_linuxdo_binding_insert ON linux_do_bindings;
		CREATE TRIGGER test_block_linuxdo_binding_insert
		BEFORE INSERT ON linux_do_bindings
		FOR EACH ROW
		EXECUTE FUNCTION test_block_linuxdo_binding_insert()
	`); err != nil {
		t.Fatalf("install linux.do binding blocker: %v", err)
	}
	t.Cleanup(func() {
		_, _ = store.pool.Exec(context.Background(), `DROP TRIGGER IF EXISTS test_block_linuxdo_binding_insert ON linux_do_bindings`)
		_, _ = store.pool.Exec(context.Background(), `DROP FUNCTION IF EXISTS test_block_linuxdo_binding_insert()`)
	})

	blocker, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin link blocker: %v", err)
	}
	blockerReleased := false
	t.Cleanup(func() {
		if !blockerReleased {
			_ = blocker.Rollback(context.Background())
		}
	})
	if _, err := blocker.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, blockerLockKey); err != nil {
		t.Fatalf("acquire link blocker: %v", err)
	}

	type linkResult struct {
		user   auth.User
		appErr *domain.AppError
	}
	linkResults := make(chan linkResult, 1)
	go func() {
		user, linkErr := store.CompleteOAuthLink(
			context.Background(), sessionHash, stateHash,
			auth.OAuthProfile{
				Provider: "linux_do", Subject: "link-subject-" + suffix,
				Username: studentUsername, LinuxDoUserID: "link-user-" + suffix,
				LinuxDoUsername: studentUsername, TrustLevel: 1,
			},
			"replacement-session-hash-"+suffix, "replacement-csrf-hash-"+suffix,
			now.Add(time.Hour), now.Add(24*time.Hour), now,
		)
		linkResults <- linkResult{user: user, appErr: linkErr}
	}()
	waitForPostgresLockWait(t, store, "%INSERT INTO linux_do_bindings%", 5*time.Second)

	idempotencyService := idempotency.NewService(store, func() time.Time { return now })
	authService := auth.NewServiceWithRegistrationEmailSenderAndIdempotency(store, func() time.Time { return now }, nil, idempotencyService)
	detail, appErr := authService.AdminUser(ctx, adminUser, studentUserID)
	if appErr != nil {
		t.Fatalf("load student account: %v", appErr)
	}
	completionBuilder := func(result auth.AdminUserMutationResult) (idempotency.Completion, *domain.AppError) {
		return idempotency.Completion{
			Status: http.StatusOK, ContentType: "application/json", Body: []byte(`{}`),
			ResourceType: "user", ResourceID: result.Detail.User.ID,
		}, nil
	}
	grantResults := make(chan *domain.AppError, 1)
	go func() {
		_, grantErr := authService.UpdateAdminUserPermissionWithIdempotency(
			context.Background(), adminUser,
			"POST /api/v1/admin/users/{id}/admin-permission:"+studentUserID,
			"concurrent-link-grant-"+suffix, "concurrent-link-grant-hash-"+suffix,
			auth.AdminUserPermissionInput{
				TargetUserID: studentUserID, Grant: true, ExpectedVersion: detail.User.Version,
				Reason: "验证绑定与授权锁顺序", RequestID: "concurrent-link-grant-" + suffix,
			},
			completionBuilder,
		)
		grantResults <- grantErr
	}()
	waitForPostgresLockWait(t, store, "%FROM users u%FOR UPDATE%", 5*time.Second)

	if err := blocker.Commit(ctx); err != nil {
		t.Fatalf("release link blocker: %v", err)
	}
	blockerReleased = true

	var linked linkResult
	select {
	case linked = <-linkResults:
	case <-time.After(5 * time.Second):
		t.Fatal("linux.do link did not finish after releasing blocker")
	}
	if linked.appErr != nil || linked.user.LinuxDoBinding == nil || !linked.user.LinuxDoBinding.Bound {
		t.Fatalf("linux.do link failed during concurrent grant: user=%+v error=%v", linked.user, linked.appErr)
	}
	select {
	case grantErr := <-grantResults:
		if grantErr == nil || grantErr.Code != domain.CodeVersionConflict {
			t.Fatalf("concurrent stale admin grant should return a retryable version conflict, got %v", grantErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("admin grant did not finish after linux.do link")
	}

	linkedUser, appErr := store.UserByID(ctx, studentUserID)
	if appErr != nil {
		t.Fatalf("reload linked account: %v", appErr)
	}
	if linkedUser.LinuxDoBinding == nil || !linkedUser.LinuxDoBinding.Bound || linkedUser.IsAdmin || auth.HasCapability(linkedUser, auth.CapabilityAdminAccess) {
		t.Fatalf("stale grant should roll back without losing the completed link: %+v", linkedUser)
	}

	freshDetail, appErr := authService.AdminUser(ctx, adminUser, studentUserID)
	if appErr != nil {
		t.Fatalf("reload linked account for retry: %v", appErr)
	}
	_, appErr = authService.UpdateAdminUserPermissionWithIdempotency(
		ctx, adminUser,
		"POST /api/v1/admin/users/{id}/admin-permission:"+studentUserID,
		"concurrent-link-grant-retry-"+suffix, "concurrent-link-grant-retry-hash-"+suffix,
		auth.AdminUserPermissionInput{
			TargetUserID: studentUserID, Grant: true, ExpectedVersion: freshDetail.User.Version,
			Reason: "绑定完成后按最新版本重试授权", RequestID: "concurrent-link-grant-retry-" + suffix,
		},
		completionBuilder,
	)
	if appErr != nil {
		t.Fatalf("retry admin grant after linux.do link: %v", appErr)
	}

	finalUser, appErr := store.UserByID(ctx, studentUserID)
	if appErr != nil {
		t.Fatalf("reload linked administrator: %v", appErr)
	}
	if finalUser.LinuxDoBinding == nil || !finalUser.LinuxDoBinding.Bound || !finalUser.IsAdmin || !auth.HasCapability(finalUser, auth.CapabilityAdminAccess) {
		t.Fatalf("retry did not preserve linked administrator facts: %+v", finalUser)
	}
}

func TestPostgresAdminGovernanceDoesNotDeadlockWithOrdinaryUserUpdate(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	suffix := strings.ToLower(uuid.NewString()[:8])
	adminUser, appErr := store.EnsureUser(ctx, "ordinary-update-admin-"+suffix, true, now)
	if appErr != nil {
		t.Fatalf("ensure admin: %v", appErr)
	}
	targetUser, appErr := store.EnsureUser(ctx, "ordinary-update-target-"+suffix, false, now)
	if appErr != nil {
		t.Fatalf("ensure target: %v", appErr)
	}
	userIDs := []string{adminUser.ID, targetUser.ID}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM notifications WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM admin_audit_logs WHERE target_id = ANY($1::uuid[]) OR admin_user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM domain_events WHERE aggregate_id = ANY($1::uuid[]) OR actor_user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM idempotency_keys WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM user_permissions WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = ANY($1::uuid[])`, userIDs)
	})

	idempotencyService := idempotency.NewService(store, func() time.Time { return now })
	authService := auth.NewServiceWithRegistrationEmailSenderAndIdempotency(store, func() time.Time { return now }, nil, idempotencyService)
	detail, appErr := authService.AdminUser(ctx, adminUser, targetUser.ID)
	if appErr != nil {
		t.Fatalf("load target account: %v", appErr)
	}

	blocker, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin target-row blocker: %v", err)
	}
	blockerReleased := false
	t.Cleanup(func() {
		if !blockerReleased {
			_ = blocker.Rollback(context.Background())
		}
	})
	if _, err := blocker.Exec(ctx, `SELECT id FROM users WHERE id = $1 FOR UPDATE`, targetUser.ID); err != nil {
		t.Fatalf("lock target row: %v", err)
	}

	completionBuilder := func(result auth.AdminUserMutationResult) (idempotency.Completion, *domain.AppError) {
		return idempotency.Completion{
			Status: http.StatusOK, ContentType: "application/json", Body: []byte(`{}`),
			ResourceType: "user", ResourceID: result.Detail.User.ID,
		}, nil
	}
	governanceResults := make(chan *domain.AppError, 1)
	go func() {
		_, governanceErr := authService.UpdateAdminUserPermissionWithIdempotency(
			context.Background(), adminUser,
			"POST /api/v1/admin/users/{id}/admin-permission:"+targetUser.ID,
			"ordinary-update-grant-"+suffix, "ordinary-update-grant-hash-"+suffix,
			auth.AdminUserPermissionInput{
				TargetUserID: targetUser.ID, Grant: true, ExpectedVersion: detail.User.Version,
				Reason: "验证普通用户更新与管理员治理锁顺序", RequestID: "ordinary-update-grant-" + suffix,
			},
			completionBuilder,
		)
		governanceResults <- governanceErr
	}()
	waitForPostgresLockWait(t, store, "%FROM users u%FOR UPDATE%", 5*time.Second)

	updatedDisplayName := "ordinary-update-completed-" + suffix
	ordinaryUpdateResults := make(chan error, 1)
	go func() {
		_, updateErr := store.pool.Exec(context.Background(), `
			UPDATE users
			SET display_name = $2,
			    updated_at = $3,
			    version = version + 1
			WHERE id = $1
		`, targetUser.ID, updatedDisplayName, now.Add(time.Minute))
		ordinaryUpdateResults <- updateErr
	}()
	waitForPostgresLockWait(t, store, "%UPDATE users%SET display_name%", 5*time.Second)

	if err := blocker.Commit(ctx); err != nil {
		t.Fatalf("release target-row blocker: %v", err)
	}
	blockerReleased = true
	select {
	case governanceErr := <-governanceResults:
		if governanceErr != nil {
			t.Fatalf("administrator governance failed after ordinary update acquired its table lock: %v", governanceErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("administrator governance did not finish after target-row blocker release")
	}
	select {
	case updateErr := <-ordinaryUpdateResults:
		if updateErr != nil {
			t.Fatalf("ordinary user update failed after governance commit: %v", updateErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ordinary user update did not finish after governance commit")
	}

	var displayName string
	var version int64
	var isAdmin bool
	if err := store.pool.QueryRow(ctx, `
		SELECT u.display_name,
		       u.version,
		       EXISTS(SELECT 1 FROM user_permissions permission WHERE permission.user_id = u.id AND permission.permission = 'admin')
		FROM users u
		WHERE u.id = $1
	`, targetUser.ID).Scan(&displayName, &version, &isAdmin); err != nil {
		t.Fatalf("reload concurrent update result: %v", err)
	}
	if displayName != updatedDisplayName || version != detail.User.Version+2 || !isAdmin {
		t.Fatalf("concurrent operations lost an update: displayName=%q version=%d isAdmin=%t", displayName, version, isAdmin)
	}
}

func waitForPostgresLockWait(t *testing.T, store *Store, queryPattern string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var waiting bool
		if err := store.pool.QueryRow(context.Background(), `
			SELECT EXISTS (
			  SELECT 1
			  FROM pg_stat_activity
			  WHERE datname = current_database()
			    AND wait_event_type = 'Lock'
			    AND query LIKE $1
			)
		`, queryPattern).Scan(&waiting); err != nil {
			t.Fatalf("inspect PostgreSQL lock wait: %v", err)
		}
		if waiting {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for PostgreSQL query matching %q to block", queryPattern)
}
