package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
)

func TestAdminUserDirectoryRouteUsesServerPaginationAndGlobalSummary(t *testing.T) {
	server := newTestServer(time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC))
	adminSession := createSession(t, server, "directory-route-admin", true)
	for index := 0; index < 25; index++ {
		createSession(t, server, "directory-route-member-"+strconv.Itoa(index), false)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?page=2&limit=20&role=user&sort=username_asc", nil)
	addCookie(request, adminSession.cookie)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("list directory status %d body %s", response.Code, response.Body.String())
	}
	var payload adminUserDirectoryResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode directory: %v", err)
	}
	if len(payload.Items) != 5 || payload.Pagination.TotalItems != 25 || payload.Pagination.TotalPages != 2 {
		t.Fatalf("unexpected server page: %+v", payload)
	}
	if payload.Summary.TotalUsers != 26 || payload.Summary.AdminUsers != 1 || payload.Summary.ActiveUsers != 26 {
		t.Fatalf("unexpected global summary: %+v", payload.Summary)
	}

	invalidRequest := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?page=not-a-number", nil)
	addCookie(invalidRequest, adminSession.cookie)
	invalidResponse := httptest.NewRecorder()
	server.ServeHTTP(invalidResponse, invalidRequest)
	if invalidResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid query status %d body %s", invalidResponse.Code, invalidResponse.Body.String())
	}
	assertProblemCode(t, invalidResponse, domain.CodeValidationFailed)
}

func TestAdminUserDetailAndGovernanceRoutes(t *testing.T) {
	server := newTestServer(time.Date(2026, 8, 1, 11, 0, 0, 0, time.UTC))
	adminSession := createSession(t, server, "governance-route-admin", true)
	memberSession := createSession(t, server, "governance-route-member", false)

	detailRequest := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/"+memberSession.userID, nil)
	addCookie(detailRequest, adminSession.cookie)
	detailResponse := httptest.NewRecorder()
	server.ServeHTTP(detailResponse, detailRequest)
	if detailResponse.Code != http.StatusOK || detailResponse.Header().Get("ETag") != `"1"` {
		t.Fatalf("detail status %d etag %q body %s", detailResponse.Code, detailResponse.Header().Get("ETag"), detailResponse.Body.String())
	}
	var detail adminUserDetailResponse
	if err := json.NewDecoder(detailResponse.Body).Decode(&detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if detail.User.ID != memberSession.userID || detail.Sessions.ActiveCount != 1 || detail.EmailVerified || detail.BackupPasswordConfigured {
		t.Fatalf("unexpected detail: %+v", detail)
	}
	if len(detail.AvailableActions) != 4 || detail.ImpactPreview.ActiveSessions != 1 || !detail.AccountCapabilities.CanLogin || !detail.AccountCapabilities.CanAccessHistoricalTransactions {
		t.Fatalf("missing governance contract: %+v", detail)
	}
	for _, forbidden := range []string{"passwordHash", "providerSubject", "sessionToken", "csrfToken", "ipAddress", "device"} {
		if strings.Contains(detailResponse.Body.String(), forbidden) {
			t.Fatalf("detail leaked %s: %s", forbidden, detailResponse.Body.String())
		}
	}

	missingVersion := newJSONRequest(http.MethodPost, "/api/v1/admin/users/"+memberSession.userID+"/status", `{"status":"suspended","reason":"异常登录核查"}`)
	addAuth(missingVersion, adminSession, "missing-version")
	missingVersionResponse := httptest.NewRecorder()
	server.ServeHTTP(missingVersionResponse, missingVersion)
	if missingVersionResponse.Code != http.StatusPreconditionRequired {
		t.Fatalf("missing If-Match status %d body %s", missingVersionResponse.Code, missingVersionResponse.Body.String())
	}
	assertProblemCode(t, missingVersionResponse, domain.CodePreconditionRequired)

	missingReason := newJSONRequest(http.MethodPost, "/api/v1/admin/users/"+memberSession.userID+"/status", `{"status":"suspended","reason":""}`)
	addAuth(missingReason, adminSession, "missing-reason")
	missingReason.Header.Set("If-Match", `"1"`)
	missingReasonResponse := httptest.NewRecorder()
	server.ServeHTTP(missingReasonResponse, missingReason)
	if missingReasonResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing reason status %d body %s", missingReasonResponse.Code, missingReasonResponse.Body.String())
	}

	path := "/api/v1/admin/users/" + memberSession.userID + "/status"
	body := `{"status":"suspended","reason":"异常登录核查"}`
	update := newJSONRequest(http.MethodPost, path, body)
	addAuth(update, adminSession, "suspend-member")
	update.Header.Set("If-Match", `"1"`)
	updateResponse := httptest.NewRecorder()
	server.ServeHTTP(updateResponse, update)
	if updateResponse.Code != http.StatusOK || updateResponse.Header().Get("ETag") != `"2"` {
		t.Fatalf("status update %d etag %q body %s", updateResponse.Code, updateResponse.Header().Get("ETag"), updateResponse.Body.String())
	}
	updateBody := updateResponse.Body.String()
	var updated adminUserDetailResponse
	if err := json.Unmarshal([]byte(updateBody), &updated); err != nil {
		t.Fatalf("decode update: %v", err)
	}
	if updated.User.AccountStatus != "suspended" || updated.User.Version != 2 || updated.Sessions.ActiveCount != 0 || len(updated.RecentAuditEntries) != 1 {
		t.Fatalf("unexpected updated account: %+v", updated)
	}
	if updated.AccountCapabilities.CanLogin || updated.AccountCapabilities.CanPublish || !updated.AccountCapabilities.CanAccessHistoricalTransactions {
		t.Fatalf("unexpected suspended capabilities: %+v", updated.AccountCapabilities)
	}

	replay := newJSONRequest(http.MethodPost, path, body)
	addAuth(replay, adminSession, "suspend-member")
	replay.Header.Set("If-Match", `"1"`)
	replayResponse := httptest.NewRecorder()
	server.ServeHTTP(replayResponse, replay)
	if replayResponse.Code != http.StatusOK || replayResponse.Body.String() != updateBody {
		t.Fatalf("unexpected idempotent replay status %d body %s", replayResponse.Code, replayResponse.Body.String())
	}

	revokedSessionRequest := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	addCookie(revokedSessionRequest, memberSession.cookie)
	revokedSessionResponse := httptest.NewRecorder()
	server.ServeHTTP(revokedSessionResponse, revokedSessionRequest)
	if revokedSessionResponse.Code != http.StatusUnauthorized {
		t.Fatalf("revoked account session status %d body %s", revokedSessionResponse.Code, revokedSessionResponse.Body.String())
	}
	assertProblemCode(t, revokedSessionResponse, domain.CodeSessionRevoked)

	selfPermission := newJSONRequest(http.MethodPost, "/api/v1/admin/users/"+adminSession.userID+"/admin-permission", `{"isAdmin":false,"reason":"自操作"}`)
	addAuth(selfPermission, adminSession, "self-permission")
	selfPermission.Header.Set("If-Match", `"1"`)
	selfPermissionResponse := httptest.NewRecorder()
	server.ServeHTTP(selfPermissionResponse, selfPermission)
	if selfPermissionResponse.Code != http.StatusForbidden {
		t.Fatalf("self permission status %d body %s", selfPermissionResponse.Code, selfPermissionResponse.Body.String())
	}
	assertProblemCode(t, selfPermissionResponse, domain.CodePermissionDenied)
}

func TestAdminAuditLogRouteReadsGlobalSafeProjection(t *testing.T) {
	server := newTestServer(time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC))
	adminSession := createSession(t, server, "audit-route-admin", true)
	memberSession := createSession(t, server, "audit-route-member", false)

	update := newJSONRequest(http.MethodPost, "/api/v1/admin/users/"+memberSession.userID+"/status", `{"status":"suspended","reason":"审计路由核查"}`)
	addAuth(update, adminSession, "audit-route-update")
	update.Header.Set("If-Match", `"1"`)
	updateResponse := httptest.NewRecorder()
	server.ServeHTTP(updateResponse, update)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("seed audit status %d body %s", updateResponse.Code, updateResponse.Body.String())
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-logs?limit=1&action=user.account_status_changed&targetType=user&actorUserId="+adminSession.userID+"&targetId="+memberSession.userID, nil)
	addCookie(request, adminSession.cookie)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("audit list status %d body %s", response.Code, response.Body.String())
	}
	var payload listResponse[adminOperationAuditEntryDTO]
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode audit list: %v", err)
	}
	if len(payload.Items) != 1 {
		t.Fatalf("unexpected audit list: %+v", payload)
	}
	item := payload.Items[0]
	if item.ActorUsername == nil || *item.ActorUsername != "audit-route-admin" ||
		item.ActorUserID == nil || *item.ActorUserID != adminSession.userID ||
		item.SourceKind != "admin" || item.Domain != "account" || item.ActorKind != "admin" ||
		item.TargetID != memberSession.userID || item.ActionLabel != "账号状态变更" ||
		item.Outcome != "status_changed" || item.Summary != "管理员变更了账号状态" ||
		item.DetailPath != nil {
		t.Fatalf("unexpected audit projection: %+v", item)
	}
	for _, forbidden := range []string{"审计路由核查", "reason", "before_json", "after_json", "beforeJson", "afterJson", "password", "csrfToken"} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Fatalf("audit response leaked %s: %s", forbidden, response.Body.String())
		}
	}

	nonAdmin := createSession(t, server, "audit-route-reader", false)
	request = httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-logs", nil)
	addCookie(request, nonAdmin.cookie)
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("non-admin audit status %d body %s", response.Code, response.Body.String())
	}
}

func TestAdminUserPermissionRouteGrantsAdministratorAccess(t *testing.T) {
	server := newTestServer(time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC))
	adminSession := createSession(t, server, "permission-route-admin", true)
	memberSession := createSession(t, server, "permission-route-member", false)
	request := newJSONRequest(http.MethodPost, "/api/v1/admin/users/"+memberSession.userID+"/admin-permission", `{"isAdmin":true,"reason":"加入值班管理员"}`)
	addAuth(request, adminSession, "grant-admin")
	request.Header.Set("If-Match", `"1"`)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("ETag") != `"2"` {
		t.Fatalf("grant permission status %d etag %q body %s", response.Code, response.Header().Get("ETag"), response.Body.String())
	}
	var detail adminUserDetailResponse
	if err := json.NewDecoder(response.Body).Decode(&detail); err != nil {
		t.Fatalf("decode permission response: %v", err)
	}
	if !detail.User.IsAdmin || detail.User.Version != 2 || len(detail.RecentAuditEntries) != 1 {
		t.Fatalf("unexpected permission detail: %+v", detail)
	}
}

func TestAdminUserPermissionRouteRequiresExplicitPermissionValue(t *testing.T) {
	server := newTestServer(time.Date(2026, 8, 1, 12, 30, 0, 0, time.UTC))
	adminSession := createSession(t, server, "permission-required-admin", true)
	memberSession := createSession(t, server, "permission-required-member", false)
	request := newJSONRequest(http.MethodPost, "/api/v1/admin/users/"+memberSession.userID+"/admin-permission", `{"reason":"加入值班管理员"}`)
	addAuth(request, adminSession, "missing-admin-permission-value")
	request.Header.Set("If-Match", `"1"`)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing isAdmin status %d body %s", response.Code, response.Body.String())
	}
	assertProblemCode(t, response, domain.CodeValidationFailed)
}
