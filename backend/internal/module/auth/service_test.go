package auth

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

type fakeAuthRepository struct {
	oauthResult          OAuthUserResult
	user                 User
	credential           PasswordCredential
	session              Session
	accountAppealSession AccountAppealSession
	adminUsers           []AdminUser
	adminDetail          AdminUserDetail
	adminAuditLogs       domain.Page[AdminAuditLog]
	lastOAuthProfile     OAuthProfile

	ensureUserCalls                  int
	createEmailRegistrationCodeCalls int
	confirmEmailRegistrationCalls    int
	createSessionCalls               int
}

func (f *fakeAuthRepository) EnsureUser(context.Context, string, bool, time.Time) (User, *domain.AppError) {
	f.ensureUserCalls++
	return User{}, domain.NewError(500, domain.CodeInternalError, "not implemented", "not implemented")
}

func (f *fakeAuthRepository) SetDevAdminPermission(_ context.Context, userID string, isAdmin bool, _ time.Time) *domain.AppError {
	if f.user.ID == userID {
		f.user.IsAdmin = isAdmin
		return nil
	}
	if f.oauthResult.User.ID == userID {
		f.oauthResult.User.IsAdmin = isAdmin
		f.user = f.oauthResult.User
		return nil
	}
	return domain.NewError(404, domain.CodeObjectNotFound, "Development persona not found", "开发身份不存在。")
}

func (f *fakeAuthRepository) UpsertOAuthUser(_ context.Context, profile OAuthProfile, _ time.Time) (OAuthUserResult, *domain.AppError) {
	f.lastOAuthProfile = profile
	return f.oauthResult, nil
}

func (f *fakeAuthRepository) ResolveExistingOAuthUser(context.Context, string, string) (User, bool, *domain.AppError) {
	return f.oauthResult.User, f.oauthResult.User.ID != "", nil
}

func (f *fakeAuthRepository) BootstrapAdminPassword(_ context.Context, credential PasswordCredential, _ time.Time) (BootstrapAdminResult, *domain.AppError) {
	if f.credential.User.IsAdmin && f.credential.User.ID != "" {
		return BootstrapAdminResult{}, nil
	}
	credential.User.ID = "bootstrap-admin"
	credential.User.IsAdmin = true
	credential.User.Status = "active"
	f.credential = credential
	return BootstrapAdminResult{User: credential.User, Created: true}, nil
}

func (f *fakeAuthRepository) UserByID(_ context.Context, userID string) (User, *domain.AppError) {
	if f.user.ID == userID {
		return f.user, nil
	}
	if f.credential.User.ID == userID {
		return f.credential.User, nil
	}
	return User{}, domain.NewError(401, domain.CodeSessionExpired, "Session required", "请先登录。")
}

func (f *fakeAuthRepository) ListAdminUsers(_ context.Context, query AdminUserDirectoryQuery) (AdminUserDirectory, *domain.AppError) {
	return AdminUserDirectory{
		Items: f.adminUsers,
		Pagination: AdminUserPagination{
			Page:       query.Page,
			Limit:      query.Limit,
			TotalItems: len(f.adminUsers),
			TotalPages: 1,
		},
	}, nil
}

func (f *fakeAuthRepository) ListAdminAuditLogs(context.Context, AdminAuditLogFilter, domain.PageRequest) (domain.Page[AdminAuditLog], *domain.AppError) {
	return f.adminAuditLogs, nil
}

func (f *fakeAuthRepository) AdminUserDetail(context.Context, string) (AdminUserDetail, *domain.AppError) {
	return f.adminDetail, nil
}

func (f *fakeAuthRepository) UpdateAdminUserStatusWithIdempotency(_ context.Context, _ idempotency.Entry, _ AdminUserStatusInput, _ time.Time, buildCompletion AdminUserCompletionBuilder) (AdminUserMutationResult, idempotency.Completion, *domain.AppError) {
	result := AdminUserMutationResult{Detail: f.adminDetail}
	completion, appErr := buildCompletion(result)
	return result, completion, appErr
}

func (f *fakeAuthRepository) UpdateAdminUserPermissionWithIdempotency(_ context.Context, _ idempotency.Entry, _ AdminUserPermissionInput, _ time.Time, buildCompletion AdminUserCompletionBuilder) (AdminUserMutationResult, idempotency.Completion, *domain.AppError) {
	result := AdminUserMutationResult{Detail: f.adminDetail}
	completion, appErr := buildCompletion(result)
	return result, completion, appErr
}

func (f *fakeAuthRepository) PasswordCredential(_ context.Context, username string) (PasswordCredential, *domain.AppError) {
	if username != f.credential.User.Username {
		return PasswordCredential{}, domain.NewError(401, domain.CodeInvalidCredentials, "Invalid credentials", "用户名或密码不正确。")
	}
	return f.credential, nil
}

func (f *fakeAuthRepository) PasswordCredentialByUserID(_ context.Context, userID string) (PasswordCredential, *domain.AppError) {
	if userID != f.credential.User.ID {
		return PasswordCredential{}, domain.NewError(404, domain.CodeObjectNotFound, "Password credential not found", "尚未设置备用密码。")
	}
	return f.credential, nil
}

func (f *fakeAuthRepository) UpsertPasswordCredential(_ context.Context, credential PasswordCredential, _ time.Time) *domain.AppError {
	if credential.User.Username == "" {
		switch credential.User.ID {
		case f.credential.User.ID:
			credential.User = f.credential.User
		case f.user.ID:
			credential.User = f.user
		}
	}
	f.credential = credential
	return nil
}

func (f *fakeAuthRepository) CreateEmailRegistrationCode(context.Context, EmailRegistrationStartInput, string, time.Time, time.Time) *domain.AppError {
	f.createEmailRegistrationCodeCalls++
	return domain.NewError(500, domain.CodeInternalError, "not implemented", "not implemented")
}

func (f *fakeAuthRepository) ConfirmEmailRegistration(context.Context, EmailRegistrationConfirmInput, string, string, string, time.Time, time.Time, time.Time) (User, *domain.AppError) {
	f.confirmEmailRegistrationCalls++
	return User{}, domain.NewError(500, domain.CodeInternalError, "not implemented", "not implemented")
}

func (f *fakeAuthRepository) CreateSession(_ context.Context, userID, sessionTokenHash, csrfTokenHash string, expiresAt, absoluteExpiresAt, now time.Time) *domain.AppError {
	f.createSessionCalls++
	f.session = Session{
		ID:                sessionTokenHash,
		UserID:            userID,
		CSRFToken:         csrfTokenHash,
		ExpiresAt:         expiresAt,
		RenewedAt:         now,
		AbsoluteExpiresAt: absoluteExpiresAt,
	}
	return nil
}

func (f *fakeAuthRepository) GetSession(context.Context, string, time.Time) (User, Session, *domain.AppError) {
	return User{}, Session{}, domain.NewError(500, domain.CodeInternalError, "not implemented", "not implemented")
}

func (f *fakeAuthRepository) GetSessionWithCSRF(context.Context, string, string, time.Time) (User, Session, *domain.AppError) {
	return User{}, Session{}, domain.NewError(500, domain.CodeInternalError, "not implemented", "not implemented")
}

func (f *fakeAuthRepository) RenewSession(context.Context, string, time.Time, time.Time, time.Time) (time.Time, bool, *domain.AppError) {
	return time.Time{}, false, nil
}

func (f *fakeAuthRepository) RefreshSessionCSRF(context.Context, string, string, time.Time) *domain.AppError {
	return domain.NewError(500, domain.CodeInternalError, "not implemented", "not implemented")
}

func (f *fakeAuthRepository) RevokeSession(context.Context, string, time.Time) *domain.AppError {
	return nil
}

func (f *fakeAuthRepository) CreateAccountAppealSession(_ context.Context, userID, sessionTokenHash, csrfTokenHash string, expiresAt, now time.Time) (User, *domain.AppError) {
	user := f.oauthResult.User
	if user.ID == "" || user.ID != userID || !eligibleAccountAppealStatus(user.Status) {
		return User{}, accountAppealIneligibleError()
	}
	f.accountAppealSession = AccountAppealSession{
		ID:        sessionTokenHash,
		UserID:    userID,
		CSRFToken: csrfTokenHash,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}
	return user, nil
}

func (f *fakeAuthRepository) RotateAccountAppealSessionCSRF(_ context.Context, sessionTokenHash, csrfTokenHash string, now time.Time) (User, AccountAppealSession, *domain.AppError) {
	if f.accountAppealSession.ID != sessionTokenHash || !now.Before(f.accountAppealSession.ExpiresAt) {
		return User{}, AccountAppealSession{}, accountAppealSessionExpiredError()
	}
	f.accountAppealSession.CSRFToken = csrfTokenHash
	return f.oauthResult.User, f.accountAppealSession, nil
}

func (f *fakeAuthRepository) GetAccountAppealSessionWithCSRF(_ context.Context, sessionTokenHash, csrfTokenHash string, now time.Time) (User, AccountAppealSession, *domain.AppError) {
	if f.accountAppealSession.ID != sessionTokenHash || f.accountAppealSession.CSRFToken != csrfTokenHash {
		return User{}, AccountAppealSession{}, accountAppealCSRFError()
	}
	if !now.Before(f.accountAppealSession.ExpiresAt) {
		return User{}, AccountAppealSession{}, accountAppealSessionExpiredError()
	}
	return f.oauthResult.User, f.accountAppealSession, nil
}

func boundAdminUserForTest() User {
	return User{
		ID:          "user-admin",
		Username:    "admin",
		DisplayName: "C2CMarket Admin",
		IsAdmin:     true,
		Status:      "active",
		LinuxDoBinding: &LinuxDoBinding{
			Bound: true,
		},
	}
}

func boundUserForTest() User {
	return User{
		ID:       "user-oauth",
		Username: "oauth-user",
		Status:   "active",
		LinuxDoBinding: &LinuxDoBinding{
			Bound: true,
		},
	}
}

func argon2idCredentialForTest(user User, password string) PasswordCredential {
	credential := newPasswordCredential(user, password)
	credential.User = user
	return credential
}

func legacyCredentialForTest(user User, password string) PasswordCredential {
	salt := "test-salt"
	return PasswordCredential{
		User:      user,
		Algorithm: PasswordAlgorithmSHA256SaltedV1,
		Salt:      salt,
		Hash:      legacyPasswordHash(salt, password),
	}
}

func adminUserCompletionForTest(result AdminUserMutationResult) (idempotency.Completion, *domain.AppError) {
	return idempotency.Completion{
		Status:       200,
		ContentType:  "application/json",
		Body:         []byte(`{"id":"` + result.Detail.User.ID + `","version":` + strconv.FormatInt(result.Detail.User.Version, 10) + `}`),
		ResourceType: "user",
		ResourceID:   result.Detail.User.ID,
	}, nil
}

func TestLoginDevPersonaIdentityPreservesCustomizedDisplayName(t *testing.T) {
	repository := &fakeAuthRepository{oauthResult: OAuthUserResult{User: User{
		ID:          "dev-buyer-id",
		Username:    "dev-buyer",
		DisplayName: "本地自定义买家",
		Status:      AccountStatusActive,
	}}}
	service := NewService(repository, time.Now)

	user, appErr := service.LoginDevPersonaIdentity(context.Background(), OAuthProfile{
		Provider: "linux_do",
		Subject:  "dev-persona-buyer",
		Username: "dev-buyer",
	}, "开发买家")
	if appErr != nil {
		t.Fatalf("login development persona: %v", appErr)
	}
	if user.Username != "dev-buyer" {
		t.Fatalf("expected fixed username, got %q", user.Username)
	}
	if repository.lastOAuthProfile.DisplayName != "本地自定义买家" {
		t.Fatalf("expected customized display name to be preserved, got %q", repository.lastOAuthProfile.DisplayName)
	}
}

func TestLoginDevPersonaIdentityRejectsOccupiedUsernameWithoutGrantingAdmin(t *testing.T) {
	service := NewService(nil, time.Now)
	occupied, _, appErr := service.CreateDevSession(context.Background(), "dev-seller", false)
	if appErr != nil {
		t.Fatalf("create occupied username: %v", appErr)
	}

	_, appErr = service.LoginDevPersonaIdentity(context.Background(), OAuthProfile{
		Provider: "linux_do",
		Subject:  "dev-persona-seller",
		Username: "dev-seller",
	}, "开发卖家")
	if appErr == nil || appErr.Status != http.StatusConflict {
		t.Fatalf("expected occupied persona username conflict, got %+v", appErr)
	}
	service.mu.Lock()
	occupiedAfter := service.users[occupied.ID]
	service.mu.Unlock()
	if occupiedAfter.IsAdmin {
		t.Fatalf("occupied account must remain non-admin: %+v", occupiedAfter)
	}
	isolated, found, appErr := service.resolveOAuthUser(context.Background(), "linux_do", "dev-persona-seller")
	if appErr != nil || !found {
		t.Fatalf("expected isolated OAuth identity, found=%v err=%v", found, appErr)
	}
	if isolated.Username == "dev-seller" || isolated.IsAdmin {
		t.Fatalf("collision identity must be isolated and non-admin: %+v", isolated)
	}
}

func TestCreateDevPersonaSessionAppliesExactAdminPermission(t *testing.T) {
	service := NewService(nil, time.Now)
	identity, appErr := service.LoginDevPersonaIdentity(context.Background(), OAuthProfile{
		Provider: "linux_do",
		Subject:  "dev-persona-buyer",
		Username: "dev-buyer",
	}, "开发买家")
	if appErr != nil {
		t.Fatalf("create development persona identity: %v", appErr)
	}

	admin, _, appErr := service.CreateDevPersonaSession(context.Background(), identity.ID, true)
	if appErr != nil || !admin.IsAdmin {
		t.Fatalf("grant development admin permission: user=%+v err=%v", admin, appErr)
	}
	buyer, _, appErr := service.CreateDevPersonaSession(context.Background(), identity.ID, false)
	if appErr != nil || buyer.IsAdmin {
		t.Fatalf("revoke development admin permission: user=%+v err=%v", buyer, appErr)
	}
}

func TestAdminUserDirectoryUsesBoundedFilteredPagesAndGlobalSummary(t *testing.T) {
	now := time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)
	service := NewService(nil, func() time.Time { return now })
	admin, _, appErr := service.CreateDevSession(context.Background(), "directory-admin", true)
	if appErr != nil {
		t.Fatalf("create admin: %v", appErr)
	}
	for index := 0; index < 25; index++ {
		username := "member-" + string(rune('a'+index))
		if _, _, appErr := service.CreateDevSession(context.Background(), username, false); appErr != nil {
			t.Fatalf("create member %d: %v", index, appErr)
		}
	}

	directory, appErr := service.AdminUsers(context.Background(), admin, AdminUserDirectoryQuery{
		Page:  2,
		Limit: 20,
		Role:  AdminUserRoleUser,
		Sort:  AdminUserSortUsernameAsc,
	})
	if appErr != nil {
		t.Fatalf("list directory: %v", appErr)
	}
	if len(directory.Items) != 5 || directory.Pagination.TotalItems != 25 || directory.Pagination.TotalPages != 2 {
		t.Fatalf("unexpected bounded page: %+v", directory)
	}
	if directory.Summary.TotalUsers != 26 || directory.Summary.AdminUsers != 1 || directory.Summary.ActiveUsers != 26 {
		t.Fatalf("unexpected global summary: %+v", directory.Summary)
	}
	if directory.Items[0].Username != "member-u" {
		t.Fatalf("unexpected page ordering: %+v", directory.Items)
	}

	filtered, appErr := service.AdminUsers(context.Background(), admin, AdminUserDirectoryQuery{
		Page:   1,
		Limit:  20,
		Search: "MEMBER-A",
	})
	if appErr != nil {
		t.Fatalf("filter directory: %v", appErr)
	}
	if filtered.Pagination.TotalItems != 1 || len(filtered.Items) != 1 || filtered.Items[0].Username != "member-a" {
		t.Fatalf("unexpected filtered page: %+v", filtered)
	}

	emptyPage, appErr := service.AdminUsers(context.Background(), admin, AdminUserDirectoryQuery{Page: 99, Limit: 20})
	if appErr != nil {
		t.Fatalf("list stale page: %v", appErr)
	}
	if len(emptyPage.Items) != 0 || emptyPage.Pagination.TotalItems != 26 || emptyPage.Pagination.TotalPages != 2 {
		t.Fatalf("unexpected stale page metadata: %+v", emptyPage)
	}
}

func TestAdminUserDirectoryRejectsInvalidQueryAndNonAdmin(t *testing.T) {
	service := NewService(nil, time.Now)
	admin, _, _ := service.CreateDevSession(context.Background(), "query-admin", true)
	member, _, _ := service.CreateDevSession(context.Background(), "query-member", false)
	if _, appErr := service.AdminUsers(context.Background(), member, AdminUserDirectoryQuery{}); appErr == nil || appErr.Code != domain.CodePermissionDenied {
		t.Fatalf("expected permission denial, got %v", appErr)
	}
	if _, appErr := service.AdminUsers(context.Background(), admin, AdminUserDirectoryQuery{Page: -1}); appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("expected page validation error, got %v", appErr)
	}
	if _, appErr := service.AdminUsers(context.Background(), admin, AdminUserDirectoryQuery{Limit: 10}); appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("expected limit validation error, got %v", appErr)
	}
}

func TestAdminAuditLogsAreFilteredBoundedAndAdminOnly(t *testing.T) {
	now := time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)
	service := NewService(nil, func() time.Time { return now })
	admin, _, _ := service.CreateDevSession(context.Background(), "audit-admin", true)
	member, _, _ := service.CreateDevSession(context.Background(), "audit-member", false)
	targetOne := "11111111-1111-4111-8111-111111111111"
	targetTwo := "22222222-2222-4222-8222-222222222222"
	before := AccountStatusActive
	after := AccountStatusSuspended
	service.adminAuditLogs = []AdminAuditLog{
		{ID: "33333333-3333-4333-8333-333333333333", ActorUserID: admin.ID, ActorUsername: admin.Username, Action: "user.account_status_changed", TargetType: "user", TargetID: targetOne, Reason: "异常登录核查", RequestID: "request-new", BeforeStatus: &before, AfterStatus: &after, CreatedAt: now},
		{ID: "22222222-2222-4222-8222-222222222222", ActorUserID: admin.ID, ActorUsername: admin.Username, Action: "official_price_record.updated", TargetType: "official_price_record", TargetID: targetTwo, Reason: "价格复核", RequestID: "request-old", CreatedAt: now.Add(-time.Minute)},
	}

	first, appErr := service.AdminAuditLogs(context.Background(), admin, AdminAuditLogFilter{ActorUserID: admin.ID}, domain.PageRequest{Limit: 1})
	if appErr != nil || len(first.Items) != 1 || first.Items[0].TargetID != targetOne || first.NextCursor == nil {
		t.Fatalf("unexpected first audit page: %+v err=%v", first, appErr)
	}
	second, appErr := service.AdminAuditLogs(context.Background(), admin, AdminAuditLogFilter{ActorUserID: admin.ID}, domain.PageRequest{Limit: 1, Cursor: *first.NextCursor})
	if appErr != nil || len(second.Items) != 1 || second.Items[0].TargetID != targetTwo || second.NextCursor != nil {
		t.Fatalf("unexpected second audit page: %+v err=%v", second, appErr)
	}
	filtered, appErr := service.AdminAuditLogs(context.Background(), admin, AdminAuditLogFilter{Action: "user.account_status_changed", TargetType: "user", TargetID: targetOne, Search: "异常登录"}, domain.PageRequest{Limit: 20})
	if appErr != nil || len(filtered.Items) != 1 || filtered.Items[0].RequestID != "request-new" {
		t.Fatalf("unexpected filtered audit page: %+v err=%v", filtered, appErr)
	}
	if _, appErr := service.AdminAuditLogs(context.Background(), member, AdminAuditLogFilter{}, domain.PageRequest{Limit: 20}); appErr == nil || appErr.Code != domain.CodePermissionDenied {
		t.Fatalf("expected audit permission denial, got %v", appErr)
	}
	if _, appErr := service.AdminAuditLogs(context.Background(), admin, AdminAuditLogFilter{TargetID: "not-a-uuid"}, domain.PageRequest{Limit: 20}); appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("expected audit target validation error, got %v", appErr)
	}
}

func TestAdminUserStatusMutationRevokesSessionsAndReplaysIdempotently(t *testing.T) {
	now := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	service := NewService(nil, func() time.Time { return now })
	admin, _, _ := service.CreateDevSession(context.Background(), "status-admin", true)
	member, memberSession, _ := service.CreateDevSession(context.Background(), "status-member", false)
	detail, appErr := service.AdminUser(context.Background(), admin, member.ID)
	if appErr != nil {
		t.Fatalf("get detail: %v", appErr)
	}
	completion, appErr := service.UpdateAdminUserStatusWithIdempotency(
		context.Background(),
		admin,
		"POST /api/v1/admin/users/{id}/status",
		"status-key",
		"status-hash",
		AdminUserStatusInput{TargetUserID: member.ID, Status: AccountStatusSuspended, ExpectedVersion: detail.User.Version, Reason: "异常登录核查", RequestID: "request-1"},
		adminUserCompletionForTest,
	)
	if appErr != nil || completion.Status != 200 {
		t.Fatalf("suspend member: completion=%+v err=%v", completion, appErr)
	}
	updated, appErr := service.AdminUser(context.Background(), admin, member.ID)
	if appErr != nil || updated.User.Status != AccountStatusSuspended || updated.User.Version != 2 || updated.ActiveSessionCount != 0 {
		t.Fatalf("unexpected updated detail: %+v err=%v", updated, appErr)
	}
	if len(updated.RecentAuditEntries) != 1 || updated.RecentAuditEntries[0].Reason != "异常登录核查" {
		t.Fatalf("missing safe audit entry: %+v", updated.RecentAuditEntries)
	}
	if _, _, appErr := service.GetSession(context.Background(), memberSession.ID); appErr == nil || appErr.Code != domain.CodeSessionRevoked {
		t.Fatalf("expected revoked session, got %v", appErr)
	}
	replay, appErr := service.UpdateAdminUserStatusWithIdempotency(
		context.Background(), admin, "POST /api/v1/admin/users/{id}/status", "status-key", "status-hash",
		AdminUserStatusInput{TargetUserID: member.ID, Status: AccountStatusSuspended, ExpectedVersion: detail.User.Version, Reason: "异常登录核查", RequestID: "request-1"},
		adminUserCompletionForTest,
	)
	if appErr != nil || string(replay.Body) != string(completion.Body) {
		t.Fatalf("unexpected replay: completion=%+v err=%v", replay, appErr)
	}
	replayedDetail, _ := service.AdminUser(context.Background(), admin, member.ID)
	if replayedDetail.User.Version != 2 || len(replayedDetail.RecentAuditEntries) != 1 {
		t.Fatalf("replay must not repeat mutation: %+v", replayedDetail)
	}
}

func TestAdminUserGovernanceRejectsSelfInvalidTransitionAndStaleVersion(t *testing.T) {
	service := NewService(nil, time.Now)
	admin, _, _ := service.CreateDevSession(context.Background(), "guard-admin", true)
	member, _, _ := service.CreateDevSession(context.Background(), "guard-member", false)
	if _, appErr := service.UpdateAdminUserStatusWithIdempotency(
		context.Background(), admin, "status", "self-key", "self-hash",
		AdminUserStatusInput{TargetUserID: admin.ID, Status: AccountStatusSuspended, ExpectedVersion: 1, Reason: "自操作"}, adminUserCompletionForTest,
	); appErr == nil || appErr.Code != domain.CodePermissionDenied {
		t.Fatalf("expected self-target denial, got %v", appErr)
	}
	if _, appErr := service.UpdateAdminUserStatusWithIdempotency(
		context.Background(), admin, "status", "noop-key", "noop-hash",
		AdminUserStatusInput{TargetUserID: member.ID, Status: AccountStatusActive, ExpectedVersion: 1, Reason: "无效变更"}, adminUserCompletionForTest,
	); appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("expected invalid transition, got %v", appErr)
	}
	if _, appErr := service.UpdateAdminUserPermissionWithIdempotency(
		context.Background(), admin, "permission", "stale-key", "stale-hash",
		AdminUserPermissionInput{TargetUserID: member.ID, Grant: true, ExpectedVersion: 2, Reason: "授予值班权限"}, adminUserCompletionForTest,
	); appErr == nil || appErr.Code != domain.CodeVersionConflict {
		t.Fatalf("expected stale version, got %v", appErr)
	}
}

func TestAdminUserPermissionMutationProtectsLastActiveAdministrator(t *testing.T) {
	service := NewService(nil, time.Now)
	admin, _, _ := service.CreateDevSession(context.Background(), "permission-admin", true)
	secondAdmin, _, _ := service.CreateDevSession(context.Background(), "permission-second", true)
	if _, appErr := service.UpdateAdminUserPermissionWithIdempotency(
		context.Background(), admin, "permission", "demote-key", "demote-hash",
		AdminUserPermissionInput{TargetUserID: secondAdmin.ID, Grant: false, ExpectedVersion: 1, Reason: "结束值班"}, adminUserCompletionForTest,
	); appErr != nil {
		t.Fatalf("demote second admin: %v", appErr)
	}
	if _, appErr := service.UpdateAdminUserStatusWithIdempotency(
		context.Background(), User{ID: secondAdmin.ID, IsAdmin: true}, "status", "last-key", "last-hash",
		AdminUserStatusInput{TargetUserID: admin.ID, Status: AccountStatusArchived, ExpectedVersion: 1, Reason: "归档旧账号"}, adminUserCompletionForTest,
	); appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("expected last-admin protection, got %v", appErr)
	}
}

func TestAdminUserDetailReturnsAuthoritativeGovernanceActions(t *testing.T) {
	service := NewService(nil, time.Now)
	admin, _, _ := service.CreateDevSession(context.Background(), "action-admin", true)
	observer, _, _ := service.CreateDevSession(context.Background(), "action-observer", true)
	member, _, _ := service.CreateDevSession(context.Background(), "action-member", false)

	memberDetail, appErr := service.AdminUser(context.Background(), admin, member.ID)
	if appErr != nil {
		t.Fatalf("load member detail: %v", appErr)
	}
	if len(memberDetail.AvailableActions) != 4 {
		t.Fatalf("expected three status actions and one permission action: %+v", memberDetail.AvailableActions)
	}
	for _, action := range memberDetail.AvailableActions {
		if !action.Allowed || !action.RequiresReason || !action.RequiresConfirmation {
			t.Fatalf("expected allowed audited member action: %+v", action)
		}
	}

	if _, appErr := service.UpdateAdminUserStatusWithIdempotency(
		context.Background(), admin, "status", "observer-suspend", "observer-suspend-hash",
		AdminUserStatusInput{TargetUserID: observer.ID, Status: AccountStatusSuspended, ExpectedVersion: 1, Reason: "暂停值班"}, adminUserCompletionForTest,
	); appErr != nil {
		t.Fatalf("suspend observer: %v", appErr)
	}
	lastAdminDetail, appErr := service.AdminUser(context.Background(), User{ID: observer.ID, IsAdmin: true}, admin.ID)
	if appErr != nil {
		t.Fatalf("load last admin detail: %v", appErr)
	}
	for _, action := range lastAdminDetail.AvailableActions {
		if action.Kind == "status" || action.Action == AdminUserActionRevokeAdmin {
			if action.Allowed || action.BlockedCode != "LAST_ACTIVE_ADMIN" {
				t.Fatalf("expected last active admin block: %+v", action)
			}
		}
	}
}

func TestLoginWithArgon2idPasswordCreatesSession(t *testing.T) {
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	repo := &fakeAuthRepository{
		credential: argon2idCredentialForTest(boundAdminUserForTest(), "unit-test-password"),
	}
	service := NewService(repo, func() time.Time { return now })

	user, session, appErr := service.LoginWithPassword(context.Background(), "admin", "unit-test-password")
	if appErr != nil {
		t.Fatalf("login with password: %v", appErr)
	}
	if user.Username != "admin" || !user.IsAdmin {
		t.Fatalf("unexpected user: %+v", user)
	}
	if session.ID == "" || session.CSRFToken == "" || session.UserID != "user-admin" {
		t.Fatalf("unexpected session: %+v", session)
	}
	if repo.session.ID == "" || repo.session.CSRFToken == "" {
		t.Fatalf("expected persisted hashed session")
	}
	if !session.ExpiresAt.Equal(now.Add(SessionIdleLifetime)) || !session.AbsoluteExpiresAt.Equal(now.Add(SessionAbsoluteLifetime)) || !session.RenewedAt.Equal(now) {
		t.Fatalf("unexpected session lifetime: %+v", session)
	}
}

func TestSessionRenewalIsThrottledAndCappedByAbsoluteExpiry(t *testing.T) {
	current := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	service := NewService(nil, func() time.Time { return current })
	_, session, appErr := service.CreateDevSession(context.Background(), "renewal-user", false)
	if appErr != nil {
		t.Fatalf("create session: %v", appErr)
	}
	if !session.ExpiresAt.Equal(current.Add(SessionIdleLifetime)) || !session.AbsoluteExpiresAt.Equal(current.Add(SessionAbsoluteLifetime)) {
		t.Fatalf("unexpected initial session: %+v", session)
	}

	current = current.Add(SessionRenewalInterval - time.Second)
	if _, renewed, appErr := service.RenewSession(context.Background(), session.ID); appErr != nil || renewed {
		t.Fatalf("session must not renew before interval: renewed=%v err=%v", renewed, appErr)
	}

	current = current.Add(time.Second)
	renewedSession, renewed, appErr := service.RenewSession(context.Background(), session.ID)
	if appErr != nil || !renewed {
		t.Fatalf("renew session: renewed=%v err=%v", renewed, appErr)
	}
	if !renewedSession.ExpiresAt.Equal(current.Add(SessionIdleLifetime)) {
		t.Fatalf("unexpected renewed expiry: %+v", renewedSession)
	}
	if _, renewed, appErr := service.RenewSession(context.Background(), session.ID); appErr != nil || renewed {
		t.Fatalf("session must not renew twice in one interval: renewed=%v err=%v", renewed, appErr)
	}

	for day := 2; day <= 29; day++ {
		current = time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC).Add(time.Duration(day) * 24 * time.Hour)
		renewedSession, renewed, appErr = service.RenewSession(context.Background(), session.ID)
		if appErr != nil || !renewed {
			t.Fatalf("renew day %d: renewed=%v err=%v", day, renewed, appErr)
		}
	}
	wantAbsoluteExpiry := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC).Add(SessionAbsoluteLifetime)
	if !renewedSession.ExpiresAt.Equal(wantAbsoluteExpiry) {
		t.Fatalf("renewal must stop at absolute expiry: got %s want %s", renewedSession.ExpiresAt, wantAbsoluteExpiry)
	}

	current = wantAbsoluteExpiry
	if _, _, appErr := service.GetSession(context.Background(), session.ID); appErr == nil || appErr.Code != domain.CodeSessionExpired {
		t.Fatalf("absolute expiry must invalidate session, got %v", appErr)
	}
}

func TestRevokedOrExpiredSessionCannotRenew(t *testing.T) {
	current := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	service := NewService(nil, func() time.Time { return current })
	_, revokedSession, appErr := service.CreateDevSession(context.Background(), "revoked-user", false)
	if appErr != nil {
		t.Fatalf("create revoked session fixture: %v", appErr)
	}
	service.Logout(context.Background(), revokedSession.ID)
	current = current.Add(SessionRenewalInterval)
	if _, renewed, appErr := service.RenewSession(context.Background(), revokedSession.ID); appErr != nil || renewed {
		t.Fatalf("revoked session must not renew: renewed=%v err=%v", renewed, appErr)
	}

	current = time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	_, expiredSession, appErr := service.CreateDevSession(context.Background(), "expired-user", false)
	if appErr != nil {
		t.Fatalf("create expired session fixture: %v", appErr)
	}
	current = current.Add(SessionIdleLifetime)
	if _, renewed, appErr := service.RenewSession(context.Background(), expiredSession.ID); appErr != nil || renewed {
		t.Fatalf("expired session must not renew: renewed=%v err=%v", renewed, appErr)
	}
}

func TestLoginWithLegacyPasswordRehashesCredential(t *testing.T) {
	repo := &fakeAuthRepository{
		credential: legacyCredentialForTest(boundAdminUserForTest(), "unit-test-password"),
	}
	legacySalt := repo.credential.Salt
	legacyHash := repo.credential.Hash
	service := NewService(repo, func() time.Time { return time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC) })

	_, session, appErr := service.LoginWithPassword(context.Background(), "admin", "unit-test-password")
	if appErr != nil {
		t.Fatalf("legacy login with password: %v", appErr)
	}
	if session.ID == "" {
		t.Fatalf("expected session after legacy login")
	}
	if repo.credential.Algorithm != PasswordAlgorithmArgon2IDV1 {
		t.Fatalf("expected legacy credential to rehash to argon2id, got %+v", repo.credential)
	}
	if repo.credential.Salt == legacySalt || repo.credential.Hash == legacyHash {
		t.Fatalf("expected rehash to replace salt/hash")
	}
	if matched, needsRehash := passwordCredentialMatches(repo.credential, "unit-test-password"); !matched || needsRehash {
		t.Fatalf("expected rehashed credential to verify without another rehash")
	}
}

type fakeRegistrationEmailSender struct {
	to          string
	username    string
	displayName string
	err         *domain.AppError
	calls       int
	codeTo      string
	code        string
}

func (f *fakeRegistrationEmailSender) SendVerificationCode(_ context.Context, toEmail, code string, _ time.Time) *domain.AppError {
	f.codeTo = toEmail
	f.code = code
	return f.err
}

func (f *fakeRegistrationEmailSender) SendRegistrationSuccess(_ context.Context, toEmail, username, displayName string, _ time.Time) *domain.AppError {
	f.calls++
	f.to = toEmail
	f.username = username
	f.displayName = displayName
	return f.err
}

func (f *fakeRegistrationEmailSender) SendCarpoolApplicationCreated(context.Context, string, string, string, time.Time) *domain.AppError {
	return nil
}

func (f *fakeRegistrationEmailSender) ExposeDevCode() bool {
	return true
}

func TestLoginWithOAuthProfileSendsRegistrationEmailForNewUserEmail(t *testing.T) {
	repo := &fakeAuthRepository{
		oauthResult: OAuthUserResult{
			User: User{
				ID:          "user-oauth",
				Username:    "oauth-user",
				DisplayName: "OAuth User",
				Status:      "active",
			},
			Created: true,
		},
	}
	sender := &fakeRegistrationEmailSender{}
	service := NewServiceWithRegistrationEmailSender(repo, func() time.Time { return time.Date(2026, 6, 26, 1, 0, 0, 0, time.UTC) }, sender)

	user, session, appErr := service.LoginWithOAuthProfile(context.Background(), OAuthProfile{
		Provider:    "linux_do",
		Subject:     "linuxdo-1",
		Username:    "oauth-user",
		DisplayName: "OAuth User",
		Email:       " OAuth.User@Example.COM ",
		TrustLevel:  3,
	})
	if appErr != nil {
		t.Fatalf("oauth login: %v", appErr)
	}
	if user.ID != "user-oauth" || session.ID == "" {
		t.Fatalf("unexpected login result user=%+v session=%+v", user, session)
	}
	if sender.calls != 1 || sender.to != "oauth.user@example.com" || sender.username != "oauth-user" || sender.displayName != "OAuth User" {
		t.Fatalf("unexpected registration email call: %+v", sender)
	}
}

func TestEmailRegistrationIsDisabled(t *testing.T) {
	repo := &fakeAuthRepository{}
	sender := &fakeRegistrationEmailSender{}
	service := NewServiceWithRegistrationEmailSender(repo, func() time.Time {
		return time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	}, sender)

	if _, appErr := service.StartEmailRegistration(context.Background(), EmailRegistrationStartInput{Email: " Test.User+Plan@Example.COM "}); appErr == nil || appErr.Code != domain.CodeEmailRegistrationDisabled {
		t.Fatalf("expected email registration disabled, got %v", appErr)
	}
	if _, _, appErr := service.ConfirmEmailRegistration(context.Background(), EmailRegistrationConfirmInput{
		Email: "test.user+plan@example.com",
		Code:  "123456",
	}); appErr == nil || appErr.Code != domain.CodeEmailRegistrationDisabled {
		t.Fatalf("expected email registration confirmation disabled, got %v", appErr)
	}
	if sender.codeTo != "" || sender.calls != 0 {
		t.Fatalf("disabled email registration must not send email: %+v", sender)
	}
	if repo.createEmailRegistrationCodeCalls != 0 || repo.confirmEmailRegistrationCalls != 0 || repo.ensureUserCalls != 0 || repo.createSessionCalls != 0 {
		t.Fatalf("disabled email registration must not write repo side effects: %+v", repo)
	}
	if repo.session.ID != "" {
		t.Fatalf("disabled email registration must not create session: %+v", repo.session)
	}
}

func TestLoginWithOAuthProfileSkipsRegistrationEmailForExistingUser(t *testing.T) {
	repo := &fakeAuthRepository{
		oauthResult: OAuthUserResult{
			User: User{
				ID:          "user-oauth",
				Username:    "oauth-user",
				DisplayName: "OAuth User",
				Status:      "active",
			},
			Created: false,
		},
	}
	sender := &fakeRegistrationEmailSender{}
	service := NewServiceWithRegistrationEmailSender(repo, time.Now, sender)

	_, _, appErr := service.LoginWithOAuthProfile(context.Background(), OAuthProfile{
		Provider:    "linux_do",
		Subject:     "linuxdo-1",
		Username:    "oauth-user",
		DisplayName: "OAuth User",
		Email:       "oauth.user@example.com",
		TrustLevel:  3,
	})
	if appErr != nil {
		t.Fatalf("oauth login: %v", appErr)
	}
	if sender.calls != 0 {
		t.Fatalf("existing user must not receive registration email: %+v", sender)
	}
}

func TestLoginWithOAuthProfileSkipsRegistrationEmailWithoutEmail(t *testing.T) {
	repo := &fakeAuthRepository{
		oauthResult: OAuthUserResult{
			User: User{
				ID:          "user-oauth",
				Username:    "oauth-user",
				DisplayName: "OAuth User",
				Status:      "active",
			},
			Created: true,
		},
	}
	sender := &fakeRegistrationEmailSender{}
	service := NewServiceWithRegistrationEmailSender(repo, time.Now, sender)

	_, _, appErr := service.LoginWithOAuthProfile(context.Background(), OAuthProfile{
		Provider:    "linux_do",
		Subject:     "linuxdo-1",
		Username:    "oauth-user",
		DisplayName: "OAuth User",
		TrustLevel:  3,
	})
	if appErr != nil {
		t.Fatalf("oauth login: %v", appErr)
	}
	if sender.calls != 0 {
		t.Fatalf("missing email must skip registration email: %+v", sender)
	}
}

func TestLoginWithOAuthProfileDoesNotFailWhenRegistrationEmailFails(t *testing.T) {
	repo := &fakeAuthRepository{
		oauthResult: OAuthUserResult{
			User: User{
				ID:          "user-oauth",
				Username:    "oauth-user",
				DisplayName: "OAuth User",
				Status:      "active",
			},
			Created: true,
		},
	}
	sender := &fakeRegistrationEmailSender{
		err: domain.NewError(502, domain.CodeInternalError, "Email send failed", "邮件发送失败，请稍后重试。"),
	}
	service := NewServiceWithRegistrationEmailSender(repo, time.Now, sender)

	user, session, appErr := service.LoginWithOAuthProfile(context.Background(), OAuthProfile{
		Provider:    "linux_do",
		Subject:     "linuxdo-1",
		Username:    "oauth-user",
		DisplayName: "OAuth User",
		Email:       "oauth.user@example.com",
		TrustLevel:  3,
	})
	if appErr != nil {
		t.Fatalf("registration email failure must not block oauth login: %v", appErr)
	}
	if user.ID == "" || session.ID == "" || sender.calls != 1 {
		t.Fatalf("unexpected login result user=%+v session=%+v sender=%+v", user, session, sender)
	}
}

func TestLoginWithOAuthProfileRejectsInactiveUserBeforeSessionCreation(t *testing.T) {
	t.Parallel()

	repo := &fakeAuthRepository{
		oauthResult: OAuthUserResult{
			User: User{
				ID:          "user-suspended",
				Username:    "suspended-user",
				DisplayName: "Suspended User",
				Status:      "suspended",
			},
		},
	}
	service := NewService(repo, time.Now)

	_, _, appErr := service.LoginWithOAuthProfile(context.Background(), OAuthProfile{
		Provider: "linuxdo",
		Subject:  "linuxdo-suspended",
		Username: "suspended-user",
	})
	if appErr == nil || appErr.Code != domain.CodeAccountRestricted {
		t.Fatalf("expected account restriction, got %#v", appErr)
	}
	if repo.createSessionCalls != 0 {
		t.Fatalf("inactive OAuth user must not create session, got %d calls", repo.createSessionCalls)
	}
	if repo.session.ID != "" {
		t.Fatalf("inactive OAuth user must not persist session: %#v", repo.session)
	}
}

func TestLoginWithPasswordRejectsInvalidPassword(t *testing.T) {
	repo := &fakeAuthRepository{
		credential: argon2idCredentialForTest(boundAdminUserForTest(), "unit-test-password"),
	}
	original := repo.credential
	service := NewService(repo, time.Now)

	_, _, appErr := service.LoginWithPassword(context.Background(), "admin", "wrong-password")
	if appErr == nil || appErr.Code != domain.CodeInvalidCredentials {
		t.Fatalf("expected invalid credentials, got %v", appErr)
	}
	if repo.session.ID != "" {
		t.Fatalf("invalid password must not create session: %+v", repo.session)
	}
	if repo.credential.Algorithm != original.Algorithm || repo.credential.Salt != original.Salt || repo.credential.Hash != original.Hash {
		t.Fatalf("invalid password must not rehash credential: before=%+v after=%+v", original, repo.credential)
	}
}

func TestLoginWithPasswordRequiresLinuxDoBinding(t *testing.T) {
	user := User{ID: "user-email", Username: "email-user", Status: "active"}
	repo := &fakeAuthRepository{
		credential: argon2idCredentialForTest(user, "unit-test-password"),
	}
	service := NewService(repo, time.Now)

	_, _, appErr := service.LoginWithPassword(context.Background(), "email-user", "unit-test-password")
	if appErr == nil || appErr.Code != domain.CodeLinuxDoBindingRequired {
		t.Fatalf("expected linux.do binding required, got %v", appErr)
	}
	if repo.session.ID != "" {
		t.Fatalf("unbound password login must not create session: %+v", repo.session)
	}
}

func TestValidateNewPasswordRequiresLengthAndComposition(t *testing.T) {
	tests := []struct {
		name     string
		password string
		reason   string
	}{
		{name: "too short", password: "Aa1!", reason: "too_short"},
		{name: "too long", password: "Password1!Password1!Password1!Long", reason: "too_long"},
		{name: "missing digit", password: "Password!", reason: "composition_required"},
		{name: "missing symbol", password: "Password1", reason: "composition_required"},
		{name: "space is not a symbol", password: "Password1 ", reason: "composition_required"},
		{name: "missing letter", password: "12345678!", reason: "composition_required"},
		{name: "ascii letter required", password: "密码123456!", reason: "composition_required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appErr := validateNewPassword(tt.password)
			if appErr == nil || len(appErr.FieldErrors) != 1 || appErr.FieldErrors[0].Code != tt.reason {
				t.Fatalf("expected reason %s, got %v", tt.reason, appErr)
			}
		})
	}

	if appErr := validateNewPassword("Password1!"); appErr != nil {
		t.Fatalf("expected valid password, got %v", appErr)
	}
}

func TestSetPasswordCreatesCredentialWithoutCurrentPasswordForLinuxDoBoundUser(t *testing.T) {
	repo := &fakeAuthRepository{
		user: boundUserForTest(),
	}
	service := NewService(repo, time.Now)

	appErr := service.SetPassword(context.Background(), SetPasswordInput{
		UserID:      "user-oauth",
		NewPassword: "unit-test-password-1!",
	})
	if appErr != nil {
		t.Fatalf("set password: %v", appErr)
	}
	if repo.credential.User.ID != "user-oauth" || repo.credential.Hash == "" || repo.credential.Salt == "" {
		t.Fatalf("expected credential upsert, got %+v", repo.credential)
	}
	if repo.credential.Algorithm != PasswordAlgorithmArgon2IDV1 {
		t.Fatalf("expected argon2id credential, got %+v", repo.credential)
	}
}

func TestSetPasswordRequiresLinuxDoBinding(t *testing.T) {
	repo := &fakeAuthRepository{
		user: User{ID: "user-email", Username: "email-user", Status: "active"},
	}
	service := NewService(repo, time.Now)

	appErr := service.SetPassword(context.Background(), SetPasswordInput{
		UserID:      "user-email",
		NewPassword: "unit-test-password-1!",
	})
	if appErr == nil || appErr.Code != domain.CodeLinuxDoBindingRequired {
		t.Fatalf("expected linux.do binding required, got %v", appErr)
	}
	if repo.credential.User.ID != "" {
		t.Fatalf("unbound user must not get password credential: %+v", repo.credential)
	}
}

func TestSetPasswordRequiresCurrentPasswordWhenConfigured(t *testing.T) {
	user := boundUserForTest()
	repo := &fakeAuthRepository{
		credential: legacyCredentialForTest(user, "unit-test-password"),
	}
	legacyHash := repo.credential.Hash
	service := NewService(repo, time.Now)

	appErr := service.SetPassword(context.Background(), SetPasswordInput{
		UserID:      "user-oauth",
		NewPassword: "new-unit-test-password-1!",
	})
	if appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("expected current password validation error, got %v", appErr)
	}
	appErr = service.SetPassword(context.Background(), SetPasswordInput{
		UserID:          "user-oauth",
		CurrentPassword: "unit-test-password",
		NewPassword:     "new-unit-test-password-1!",
	})
	if appErr != nil {
		t.Fatalf("change password: %v", appErr)
	}
	if repo.credential.Hash == legacyHash {
		t.Fatalf("expected changed password hash")
	}
	if repo.credential.Algorithm != PasswordAlgorithmArgon2IDV1 {
		t.Fatalf("expected changed password to use argon2id, got %+v", repo.credential)
	}
}

func TestBootstrapAdminCreatesFirstAdminCredential(t *testing.T) {
	service := NewService(nil, func() time.Time {
		return time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)
	})

	result, appErr := service.BootstrapAdmin(context.Background(), BootstrapAdminInput{
		Username: "Admin Root",
		Password: "bootstrap-password-1!",
	})
	if appErr != nil {
		t.Fatalf("bootstrap admin: %v", appErr)
	}
	if !result.Created || result.User.Username != "admin-root" || !result.User.IsAdmin {
		t.Fatalf("unexpected bootstrap result: %+v", result)
	}

	user, session, appErr := service.LoginWithPassword(context.Background(), "admin-root", "bootstrap-password-1!")
	if appErr != nil {
		t.Fatalf("login with bootstrapped admin: %v", appErr)
	}
	if !user.IsAdmin || session.ID == "" {
		t.Fatalf("unexpected bootstrapped admin login: user=%+v session=%+v", user, session)
	}
}

func TestBootstrapAdminDoesNotOverwriteExistingAdminCredential(t *testing.T) {
	service := NewService(nil, time.Now)

	first, appErr := service.BootstrapAdmin(context.Background(), BootstrapAdminInput{
		Username: "admin",
		Password: "first-bootstrap-password-1!",
	})
	if appErr != nil || !first.Created {
		t.Fatalf("first bootstrap admin result=%+v err=%v", first, appErr)
	}
	second, appErr := service.BootstrapAdmin(context.Background(), BootstrapAdminInput{
		Username: "admin",
		Password: "second-bootstrap-password-2!",
	})
	if appErr != nil {
		t.Fatalf("second bootstrap admin: %v", appErr)
	}
	if second.Created {
		t.Fatalf("second bootstrap must not overwrite existing admin credential: %+v", second)
	}

	if _, _, appErr := service.LoginWithPassword(context.Background(), "admin", "first-bootstrap-password-1!"); appErr != nil {
		t.Fatalf("first bootstrap password should still work: %v", appErr)
	}
	if _, _, appErr := service.LoginWithPassword(context.Background(), "admin", "second-bootstrap-password-2!"); appErr == nil || appErr.Code != domain.CodeInvalidCredentials {
		t.Fatalf("second bootstrap password must not work, got %v", appErr)
	}
}

func TestOAuthIdentityOwnershipSurvivesProviderUsernameChange(t *testing.T) {
	service := NewService(nil, time.Now)

	first, _, appErr := service.LoginWithOAuthProfile(context.Background(), OAuthProfile{
		Provider:    "linux_do",
		Subject:     "identity-rename-1",
		Username:    "first-handle",
		DisplayName: "First Name",
	})
	if appErr != nil {
		t.Fatalf("first OAuth login: %v", appErr)
	}
	second, _, appErr := service.LoginWithOAuthProfile(context.Background(), OAuthProfile{
		Provider:    "linux_do",
		Subject:     "identity-rename-1",
		Username:    "renamed-handle",
		DisplayName: "Renamed User",
	})
	if appErr != nil {
		t.Fatalf("second OAuth login: %v", appErr)
	}

	if second.ID != first.ID {
		t.Fatalf("provider username change must keep identity owner: first=%s second=%s", first.ID, second.ID)
	}
	if second.Username != first.Username {
		t.Fatalf("provider username change must preserve local username: first=%q second=%q", first.Username, second.Username)
	}
	if second.DisplayName != "Renamed User" {
		t.Fatalf("expected refreshed display name, got %q", second.DisplayName)
	}
}

func TestOAuthFirstLoginDoesNotReuseConflictingLocalUsers(t *testing.T) {
	service := NewService(nil, time.Now)
	ordinary, _, appErr := service.CreateDevSession(context.Background(), "shared-handle", false)
	if appErr != nil {
		t.Fatalf("create ordinary user: %v", appErr)
	}
	admin, _, appErr := service.CreateDevSession(context.Background(), "admin-handle", true)
	if appErr != nil {
		t.Fatalf("create admin user: %v", appErr)
	}

	oauthOrdinaryCollision, _, appErr := service.LoginWithOAuthProfile(context.Background(), OAuthProfile{
		Provider: "linux_do",
		Subject:  "identity-collision-ordinary",
		Username: ordinary.Username,
	})
	if appErr != nil {
		t.Fatalf("OAuth ordinary collision login: %v", appErr)
	}
	if oauthOrdinaryCollision.ID == ordinary.ID || oauthOrdinaryCollision.Username == ordinary.Username {
		t.Fatalf("OAuth login must create an independent handle on ordinary-user collision: ordinary=%+v oauth=%+v", ordinary, oauthOrdinaryCollision)
	}

	oauthAdminCollision, _, appErr := service.LoginWithOAuthProfile(context.Background(), OAuthProfile{
		Provider: "linux_do",
		Subject:  "identity-collision-admin",
		Username: admin.Username,
	})
	if appErr != nil {
		t.Fatalf("OAuth admin collision login: %v", appErr)
	}
	if oauthAdminCollision.ID == admin.ID || oauthAdminCollision.Username == admin.Username {
		t.Fatalf("OAuth login must create an independent handle on admin collision: admin=%+v oauth=%+v", admin, oauthAdminCollision)
	}
	if oauthAdminCollision.IsAdmin {
		t.Fatalf("normal OAuth login must never grant admin permission: %+v", oauthAdminCollision)
	}
}

func TestOAuthIdentityIsScopedByProviderAndSubject(t *testing.T) {
	service := NewService(nil, time.Now)

	linuxDo, _, appErr := service.LoginWithOAuthProfile(context.Background(), OAuthProfile{
		Provider: " Linux_Do ",
		Subject:  "shared-subject",
		Username: "provider-shared",
	})
	if appErr != nil {
		t.Fatalf("linux.do OAuth login: %v", appErr)
	}
	linuxDoAgain, _, appErr := service.LoginWithOAuthProfile(context.Background(), OAuthProfile{
		Provider: "linux_do",
		Subject:  "shared-subject",
		Username: "provider-renamed",
	})
	if appErr != nil {
		t.Fatalf("normalized linux.do OAuth login: %v", appErr)
	}
	github, _, appErr := service.LoginWithOAuthProfile(context.Background(), OAuthProfile{
		Provider: "github",
		Subject:  "shared-subject",
		Username: "provider-shared",
	})
	if appErr != nil {
		t.Fatalf("GitHub OAuth login: %v", appErr)
	}

	if linuxDoAgain.ID != linuxDo.ID {
		t.Fatalf("normalized provider key must resolve the existing identity: first=%s second=%s", linuxDo.ID, linuxDoAgain.ID)
	}
	if github.ID == linuxDo.ID {
		t.Fatalf("same subject from different providers must create different users: linux_do=%s github=%s", linuxDo.ID, github.ID)
	}
	if github.LinuxDoBinding != nil {
		t.Fatalf("non-linux.do identity must not receive linux.do binding: %+v", github.LinuxDoBinding)
	}
}

func TestOAuthConcurrentFirstLoginReturnsOneInMemoryUser(t *testing.T) {
	service := NewService(nil, time.Now)
	var waitGroup sync.WaitGroup
	userIDs := make(chan string, 16)
	failures := make(chan string, 16)

	for range 16 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			user, _, appErr := service.LoginWithOAuthProfile(context.Background(), OAuthProfile{
				Provider: "linux_do",
				Subject:  "concurrent-subject",
				Username: "concurrent-user",
			})
			if appErr != nil {
				failures <- appErr.Error()
				return
			}
			userIDs <- user.ID
		}()
	}
	waitGroup.Wait()
	close(userIDs)
	close(failures)

	for failure := range failures {
		t.Fatalf("concurrent OAuth login failed: %s", failure)
	}
	var winningUserID string
	for userID := range userIDs {
		if winningUserID == "" {
			winningUserID = userID
		}
		if userID != winningUserID {
			t.Fatalf("concurrent OAuth logins returned different users: first=%s next=%s", winningUserID, userID)
		}
	}
	if len(service.oauthUserIDs) != 1 || len(service.users) != 1 {
		t.Fatalf("concurrent OAuth login created extra state: identities=%d users=%d", len(service.oauthUserIDs), len(service.users))
	}
}

func TestOAuthUsernameCandidatesAreStableAndBounded(t *testing.T) {
	base := OAuthUsernameCandidate(" A Very Long Provider Username !!! ", "linux_do", "subject-1", 0)
	firstCollision := OAuthUsernameCandidate(base, "linux_do", "subject-1", 1)
	repeatedCollision := OAuthUsernameCandidate(base, "linux_do", "subject-1", 1)
	nextCollision := OAuthUsernameCandidate(base, "linux_do", "subject-1", 2)

	for _, candidate := range []string{base, firstCollision, nextCollision} {
		if len(candidate) > maxUsernameLength || !usernamePattern.MatchString(candidate) {
			t.Fatalf("invalid OAuth username candidate %q", candidate)
		}
	}
	if firstCollision != repeatedCollision {
		t.Fatalf("stable collision candidate changed: first=%q repeated=%q", firstCollision, repeatedCollision)
	}
	if firstCollision == nextCollision || !strings.Contains(firstCollision, "-") {
		t.Fatalf("collision attempts must be distinct and suffixed: first=%q next=%q", firstCollision, nextCollision)
	}
}

func TestBootstrapAdminFailsClosedOnUnprovenExistingState(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(*Service)
	}{
		{
			name: "occupied ordinary username",
			prepare: func(service *Service) {
				if _, _, appErr := service.CreateDevSession(context.Background(), "bootstrap-target", false); appErr != nil {
					t.Fatalf("create ordinary fixture: %v", appErr)
				}
			},
		},
		{
			name: "foreign administrator",
			prepare: func(service *Service) {
				if _, _, appErr := service.CreateDevSession(context.Background(), "foreign-admin", true); appErr != nil {
					t.Fatalf("create admin fixture: %v", appErr)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewService(nil, time.Now)
			tt.prepare(service)

			result, appErr := service.BootstrapAdmin(context.Background(), BootstrapAdminInput{
				Username: "bootstrap-target",
				Password: "bootstrap-password-1!",
			})
			if appErr == nil || appErr.Code != domain.CodeAdminBootstrapConflict {
				t.Fatalf("expected bootstrap conflict, result=%+v err=%v", result, appErr)
			}
		})
	}
}

func TestBootstrapAdminRequiresMatchingProvenanceOnRerun(t *testing.T) {
	service := NewService(nil, time.Now)
	first, appErr := service.BootstrapAdmin(context.Background(), BootstrapAdminInput{
		Username: "bootstrap-root",
		Password: "first-bootstrap-password-1!",
	})
	if appErr != nil || !first.Created {
		t.Fatalf("first bootstrap result=%+v err=%v", first, appErr)
	}

	second, appErr := service.BootstrapAdmin(context.Background(), BootstrapAdminInput{
		Username: "different-root",
		Password: "second-bootstrap-password-2!",
	})
	if appErr == nil || appErr.Code != domain.CodeAdminBootstrapConflict {
		t.Fatalf("expected bootstrap configuration conflict, result=%+v err=%v", second, appErr)
	}

	if _, _, appErr := service.LoginWithPassword(context.Background(), "bootstrap-root", "first-bootstrap-password-1!"); appErr != nil {
		t.Fatalf("proven bootstrap password should remain valid: %v", appErr)
	}
	if _, _, appErr := service.LoginWithPassword(context.Background(), "different-root", "second-bootstrap-password-2!"); appErr == nil || appErr.Code != domain.CodeInvalidCredentials {
		t.Fatalf("conflicting bootstrap must not create or update credentials, got %v", appErr)
	}
}

func TestBootstrapAdminFailsClosedWhenProvenanceStateIsDamaged(t *testing.T) {
	service := NewService(nil, time.Now)
	first, appErr := service.BootstrapAdmin(context.Background(), BootstrapAdminInput{
		Username: "bootstrap-root",
		Password: "first-bootstrap-password-1!",
	})
	if appErr != nil || !first.Created {
		t.Fatalf("first bootstrap result=%+v err=%v", first, appErr)
	}

	user := service.users[first.User.ID]
	user.IsAdmin = false
	service.users[first.User.ID] = user

	result, appErr := service.BootstrapAdmin(context.Background(), BootstrapAdminInput{
		Username: "bootstrap-root",
		Password: "first-bootstrap-password-1!",
	})
	if appErr == nil || appErr.Code != domain.CodeAdminBootstrapInconsistent {
		t.Fatalf("expected inconsistent bootstrap state, result=%+v err=%v", result, appErr)
	}
}
