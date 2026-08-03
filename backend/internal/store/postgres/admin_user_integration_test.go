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
