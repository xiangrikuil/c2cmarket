package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apihealth"
	"c2c-market/backend/internal/module/auth"
	app "c2c-market/backend/internal/module/core"
)

type apiHealthRouteService struct {
	config                apihealth.Config
	found                 bool
	putInput              apihealth.ConfigInput
	putExpectedVersion    int64
	deleteExpectedVersion int64
	challengeMethod       string
	challengeVersion      int64
	challengeCalls        int
	verifyVersion         int64
	verifyCalls           int
	adminStatus           string
	adminPage             domain.PageRequest
	adminApprove          bool
	adminReason           string
	adminVersion          int64
	adminDecisionCalls    int
}

func (s *apiHealthRouteService) OwnerConfig(context.Context, auth.User, string) (apihealth.Config, bool, *domain.AppError) {
	return s.config, s.found, nil
}

func (s *apiHealthRouteService) PutOwnerConfig(_ context.Context, _ auth.User, _ string, input apihealth.ConfigInput, expectedVersion int64) (apihealth.Config, *domain.AppError) {
	s.putInput = input
	s.putExpectedVersion = expectedVersion
	config := s.config
	config.Version = expectedVersion + 1
	config.CredentialConfigured = input.Credential != nil
	config.Enabled = input.Enabled
	config.BaseURL = input.BaseURL
	config.Model = input.Model
	return config, nil
}

func (s *apiHealthRouteService) DeleteOwnerConfig(_ context.Context, _ auth.User, _ string, expectedVersion int64) *domain.AppError {
	s.deleteExpectedVersion = expectedVersion
	return nil
}

func (s *apiHealthRouteService) CreateChallenge(_ context.Context, _ auth.User, _ string, method string, expectedVersion int64) (apihealth.Challenge, *domain.AppError) {
	s.challengeMethod = method
	s.challengeVersion = expectedVersion
	s.challengeCalls++
	return apihealth.Challenge{
		Token: "one-time-token", Method: method, DNSRecordName: "_c2cmarket-probe.example.com",
		ExpiresAt: time.Date(2026, 8, 4, 5, 30, 0, 0, time.UTC), ConfigVersion: expectedVersion + 1,
	}, nil
}

func (s *apiHealthRouteService) VerifyChallenge(_ context.Context, _ auth.User, _ string, expectedVersion int64) (apihealth.Config, *domain.AppError) {
	s.verifyVersion = expectedVersion
	s.verifyCalls++
	config := s.config
	config.AuthorizationStatus = apihealth.AuthorizationVerified
	config.AuthorizationMethod = apihealth.AuthorizationMethodDNSTXT
	config.Version = expectedVersion + 1
	return config, nil
}

func (s *apiHealthRouteService) AdminConfigs(_ context.Context, user auth.User, status string, page domain.PageRequest) (domain.Page[apihealth.Config], *domain.AppError) {
	if !user.IsAdmin {
		return domain.Page[apihealth.Config]{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Forbidden", "需要管理员权限。")
	}
	s.adminStatus = status
	s.adminPage = page
	next := "next-probe-page"
	return domain.Page[apihealth.Config]{Items: []apihealth.Config{s.config}, NextCursor: &next}, nil
}

func (s *apiHealthRouteService) AdminDecision(_ context.Context, user auth.User, _ string, expectedVersion int64, approve bool, reason string) (apihealth.Config, *domain.AppError) {
	if !user.IsAdmin {
		return apihealth.Config{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Forbidden", "需要管理员权限。")
	}
	s.adminApprove = approve
	s.adminReason = reason
	s.adminVersion = expectedVersion
	s.adminDecisionCalls++
	config := s.config
	config.Version = expectedVersion + 1
	if approve {
		config.AuthorizationStatus = apihealth.AuthorizationApproved
	} else {
		config.AuthorizationStatus = apihealth.AuthorizationRejected
	}
	return config, nil
}

func (s *apiHealthRouteService) Summaries(context.Context, []string) (map[string]apihealth.Summary, *domain.AppError) {
	return map[string]apihealth.Summary{}, nil
}

func TestOwnerAPIHealthProbeRoutesKeepCredentialWriteOnly(t *testing.T) {
	now := time.Date(2026, 8, 4, 5, 0, 0, 0, time.UTC)
	health := &apiHealthRouteService{found: true, config: testAPIHealthConfig(now)}
	handler := NewServer(app.NewServiceWithClock(func() time.Time { return now }), ServerOptions{
		EnableDevAuth: true,
		APIHealth:     health,
	})
	owner := createSession(t, handler, "probe-owner", false)
	path := "/api/v1/owner/api-services/" + health.config.APIServiceID + "/health-probe"

	read := httptest.NewRequest(http.MethodGet, path, nil)
	addCookie(read, owner.cookie)
	readResponse := httptest.NewRecorder()
	handler.ServeHTTP(readResponse, read)
	if readResponse.Code != http.StatusOK || readResponse.Header().Get("ETag") != `"4"` {
		t.Fatalf("owner probe read status=%d etag=%q body=%s", readResponse.Code, readResponse.Header().Get("ETag"), readResponse.Body.String())
	}
	assertAPIHealthPrivateNoStore(t, readResponse)
	if body := readResponse.Body.String(); !strings.Contains(body, `"credentialConfigured":true`) || strings.Contains(body, "credential_ciphertext") || strings.Contains(body, "fingerprint") || strings.Contains(body, `"credential":`) {
		t.Fatalf("owner probe response leaked or omitted credential state: %s", body)
	}

	putBody := `{"baseUrl":"https://example.com/v1","model":"gpt-5.1","credential":"sk-owner-secret","enabled":true,"acknowledgeInsecureHttp":false}`
	missingCSRF := newJSONRequest(http.MethodPut, path, putBody)
	addCookie(missingCSRF, owner.cookie)
	missingCSRF.Header.Set("If-Match", `"4"`)
	missingCSRFResponse := httptest.NewRecorder()
	handler.ServeHTTP(missingCSRFResponse, missingCSRF)
	if missingCSRFResponse.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF status=%d body=%s", missingCSRFResponse.Code, missingCSRFResponse.Body.String())
	}
	assertAPIHealthPrivateNoStore(t, missingCSRFResponse)

	missingVersion := newJSONRequest(http.MethodPut, path, putBody)
	addAuth(missingVersion, owner, "unused")
	missingVersionResponse := httptest.NewRecorder()
	handler.ServeHTTP(missingVersionResponse, missingVersion)
	if missingVersionResponse.Code != http.StatusPreconditionRequired {
		t.Fatalf("missing If-Match status=%d body=%s", missingVersionResponse.Code, missingVersionResponse.Body.String())
	}

	put := newJSONRequest(http.MethodPut, path, putBody)
	addAuth(put, owner, "unused")
	put.Header.Set("If-Match", `"4"`)
	putResponse := httptest.NewRecorder()
	handler.ServeHTTP(putResponse, put)
	if putResponse.Code != http.StatusOK || putResponse.Header().Get("ETag") != `"5"` {
		t.Fatalf("owner probe put status=%d etag=%q body=%s", putResponse.Code, putResponse.Header().Get("ETag"), putResponse.Body.String())
	}
	assertAPIHealthPrivateNoStore(t, putResponse)
	if health.putInput.Credential == nil || *health.putInput.Credential != "sk-owner-secret" || health.putExpectedVersion != 4 {
		t.Fatalf("unexpected owner probe input: input=%+v version=%d", health.putInput, health.putExpectedVersion)
	}
	if strings.Contains(putResponse.Body.String(), "sk-owner-secret") || strings.Contains(putResponse.Body.String(), `"credential":`) {
		t.Fatalf("owner probe credential was echoed: %s", putResponse.Body.String())
	}

	challenge := newJSONRequest(http.MethodPost, path+"/challenges", `{"method":"dns_txt"}`)
	addAuth(challenge, owner, "unused")
	challenge.Header.Set("If-Match", `"5"`)
	challengeResponse := httptest.NewRecorder()
	handler.ServeHTTP(challengeResponse, challenge)
	if challengeResponse.Code != http.StatusCreated || challengeResponse.Header().Get("ETag") != `"6"` || !strings.Contains(challengeResponse.Body.String(), "one-time-token") {
		t.Fatalf("challenge status=%d etag=%q body=%s", challengeResponse.Code, challengeResponse.Header().Get("ETag"), challengeResponse.Body.String())
	}
	if health.challengeMethod != apihealth.AuthorizationMethodDNSTXT || health.challengeVersion != 5 {
		t.Fatalf("unexpected challenge input method=%q version=%d", health.challengeMethod, health.challengeVersion)
	}
	challengeReplay := newJSONRequest(http.MethodPost, path+"/challenges", `{"method":"dns_txt"}`)
	addAuth(challengeReplay, owner, "unused")
	challengeReplay.Header.Set("If-Match", `"5"`)
	challengeReplayResponse := httptest.NewRecorder()
	handler.ServeHTTP(challengeReplayResponse, challengeReplay)
	if challengeReplayResponse.Code != http.StatusCreated || challengeReplayResponse.Header().Get("ETag") != `"6"` ||
		challengeReplayResponse.Body.String() != challengeResponse.Body.String() || health.challengeCalls != 1 {
		t.Fatalf("challenge replay status=%d etag=%q calls=%d body=%s", challengeReplayResponse.Code, challengeReplayResponse.Header().Get("ETag"), health.challengeCalls, challengeReplayResponse.Body.String())
	}

	verify := newJSONRequest(http.MethodPost, path+"/verify", "")
	addAuth(verify, owner, "unused")
	verify.Header.Set("If-Match", `"6"`)
	verifyResponse := httptest.NewRecorder()
	handler.ServeHTTP(verifyResponse, verify)
	if verifyResponse.Code != http.StatusOK || verifyResponse.Header().Get("ETag") != `"7"` || strings.Contains(verifyResponse.Body.String(), "one-time-token") {
		t.Fatalf("verify status=%d etag=%q body=%s", verifyResponse.Code, verifyResponse.Header().Get("ETag"), verifyResponse.Body.String())
	}
	verifyReplay := newJSONRequest(http.MethodPost, path+"/verify", "")
	addAuth(verifyReplay, owner, "unused")
	verifyReplay.Header.Set("If-Match", `"6"`)
	verifyReplayResponse := httptest.NewRecorder()
	handler.ServeHTTP(verifyReplayResponse, verifyReplay)
	if verifyReplayResponse.Code != http.StatusOK || verifyReplayResponse.Header().Get("ETag") != `"7"` ||
		verifyReplayResponse.Body.String() != verifyResponse.Body.String() || health.verifyCalls != 1 {
		t.Fatalf("verify replay status=%d etag=%q calls=%d body=%s", verifyReplayResponse.Code, verifyReplayResponse.Header().Get("ETag"), health.verifyCalls, verifyReplayResponse.Body.String())
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, path, nil)
	addAuth(deleteRequest, owner, "unused")
	deleteRequest.Header.Set("If-Match", `"7"`)
	deleteResponse := httptest.NewRecorder()
	handler.ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusNoContent || health.deleteExpectedVersion != 7 {
		t.Fatalf("delete status=%d version=%d body=%s", deleteResponse.Code, health.deleteExpectedVersion, deleteResponse.Body.String())
	}
	assertAPIHealthPrivateNoStore(t, deleteResponse)
}

func TestOwnerAPIHealthProbePassesInsecureHTTPAcknowledgement(t *testing.T) {
	now := time.Date(2026, 8, 5, 5, 0, 0, 0, time.UTC)
	health := &apiHealthRouteService{found: true, config: testAPIHealthConfig(now)}
	handler := NewServer(app.NewServiceWithClock(func() time.Time { return now }), ServerOptions{
		EnableDevAuth: true,
		APIHealth:     health,
	})
	owner := createSession(t, handler, "http-probe-owner", false)
	path := "/api/v1/owner/api-services/" + health.config.APIServiceID + "/health-probe"
	request := newJSONRequest(http.MethodPut, path, `{"baseUrl":"http://api.example.com","model":"gpt-5-mini","credential":"low-quota-key","enabled":true,"acknowledgeInsecureHttp":true}`)
	addAuth(request, owner, "unused")
	request.Header.Set("If-Match", `"4"`)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("HTTP probe PUT status=%d body=%s", response.Code, response.Body.String())
	}
	if !health.putInput.AcknowledgeInsecureHTTP || health.putInput.BaseURL != "http://api.example.com" {
		t.Fatalf("HTTP acknowledgement was not passed to service: %+v", health.putInput)
	}
	if body := response.Body.String(); strings.Contains(body, "acknowledgeInsecureHttp") || strings.Contains(body, "low-quota-key") {
		t.Fatalf("request-only HTTP acknowledgement or credential leaked into response: %s", body)
	}
}

func TestOwnerAPIHealthProbeFirstPutAcceptsVersionZero(t *testing.T) {
	now := time.Date(2026, 8, 4, 5, 0, 0, 0, time.UTC)
	health := &apiHealthRouteService{config: testAPIHealthConfig(now)}
	handler := NewServer(app.NewServiceWithClock(func() time.Time { return now }), ServerOptions{EnableDevAuth: true, APIHealth: health})
	owner := createSession(t, handler, "new-probe-owner", false)
	path := "/api/v1/owner/api-services/" + health.config.APIServiceID + "/health-probe"
	read := httptest.NewRequest(http.MethodGet, path, nil)
	addCookie(read, owner.cookie)
	readResponse := httptest.NewRecorder()
	handler.ServeHTTP(readResponse, read)
	if readResponse.Code != http.StatusNotFound {
		t.Fatalf("unconfigured read status=%d body=%s", readResponse.Code, readResponse.Body.String())
	}
	assertAPIHealthPrivateNoStore(t, readResponse)

	request := newJSONRequest(http.MethodPut, path, `{"baseUrl":"https://example.com/v1","model":"gpt-5","credential":"sk-first","enabled":false}`)
	addAuth(request, owner, "unused")
	request.Header.Set("If-Match", `"0"`)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("ETag") != `"1"` || health.putExpectedVersion != 0 {
		t.Fatalf("first put status=%d etag=%q version=%d body=%s", response.Code, response.Header().Get("ETag"), health.putExpectedVersion, response.Body.String())
	}
}

func TestAdminAPIHealthProbeRoutesReturnSafeReviewProjection(t *testing.T) {
	now := time.Date(2026, 8, 4, 5, 0, 0, 0, time.UTC)
	health := &apiHealthRouteService{config: testAPIHealthConfig(now)}
	handler := NewServer(app.NewServiceWithClock(func() time.Time { return now }), ServerOptions{EnableDevAuth: true, APIHealth: health})
	member := createSession(t, handler, "probe-review-member", false)
	admin := createSession(t, handler, "probe-review-admin", true)
	listPath := "/api/v1/admin/api-service-health-probes?status=pending&limit=5"

	forbidden := httptest.NewRequest(http.MethodGet, listPath, nil)
	addCookie(forbidden, member.cookie)
	forbiddenResponse := httptest.NewRecorder()
	handler.ServeHTTP(forbiddenResponse, forbidden)
	if forbiddenResponse.Code != http.StatusForbidden {
		t.Fatalf("non-admin list status=%d body=%s", forbiddenResponse.Code, forbiddenResponse.Body.String())
	}
	assertAPIHealthPrivateNoStore(t, forbiddenResponse)

	list := httptest.NewRequest(http.MethodGet, listPath, nil)
	addCookie(list, admin.cookie)
	adminListResponse := httptest.NewRecorder()
	handler.ServeHTTP(adminListResponse, list)
	if adminListResponse.Code != http.StatusOK || health.adminStatus != apihealth.AuthorizationPending || health.adminPage.Limit != 5 {
		t.Fatalf("admin list status=%d query=%q page=%+v body=%s", adminListResponse.Code, health.adminStatus, health.adminPage, adminListResponse.Body.String())
	}
	assertAPIHealthPrivateNoStore(t, adminListResponse)
	var payload listResponse[adminAPIHealthProbeResponse]
	if err := json.NewDecoder(adminListResponse.Body).Decode(&payload); err != nil || len(payload.Items) != 1 {
		t.Fatalf("decode admin probe list: items=%+v err=%v", payload.Items, err)
	}
	if item := payload.Items[0]; item.ServiceTitle != "Managed OpenAI API" || item.OwnerUsername != "probe-owner" || item.OwnerDisplayName != "Probe Owner" || item.NormalizedOrigin != "https://example.com:443" {
		t.Fatalf("unexpected admin review projection: %+v", item)
	}
	if body := adminListResponse.Body.String(); strings.Contains(body, "credential") || strings.Contains(body, "fingerprint") || strings.Contains(body, `"baseUrl"`) {
		t.Fatalf("admin review projection exposed private config fields: %s", body)
	}

	decisionPath := "/api/v1/admin/api-service-health-probes/" + health.config.ID + "/approve"
	missingCSRF := newJSONRequest(http.MethodPost, decisionPath, `{"reason":"exact origin reviewed"}`)
	addCookie(missingCSRF, admin.cookie)
	missingCSRF.Header.Set("If-Match", `"4"`)
	missingCSRFResponse := httptest.NewRecorder()
	handler.ServeHTTP(missingCSRFResponse, missingCSRF)
	if missingCSRFResponse.Code != http.StatusForbidden {
		t.Fatalf("admin decision missing CSRF status=%d body=%s", missingCSRFResponse.Code, missingCSRFResponse.Body.String())
	}

	approve := newJSONRequest(http.MethodPost, decisionPath, `{"reason":"exact origin reviewed"}`)
	addAuth(approve, admin, "unused")
	approve.Header.Set("If-Match", `"4"`)
	approveResponse := httptest.NewRecorder()
	handler.ServeHTTP(approveResponse, approve)
	if approveResponse.Code != http.StatusOK || approveResponse.Header().Get("ETag") != `"5"` || !health.adminApprove || health.adminVersion != 4 || health.adminReason != "exact origin reviewed" {
		t.Fatalf("approve status=%d etag=%q approve=%v version=%d reason=%q body=%s", approveResponse.Code, approveResponse.Header().Get("ETag"), health.adminApprove, health.adminVersion, health.adminReason, approveResponse.Body.String())
	}
	assertAPIHealthPrivateNoStore(t, approveResponse)
	approveReplay := newJSONRequest(http.MethodPost, decisionPath, `{"reason":"exact origin reviewed"}`)
	addAuth(approveReplay, admin, "unused")
	approveReplay.Header.Set("If-Match", `"4"`)
	approveReplayResponse := httptest.NewRecorder()
	handler.ServeHTTP(approveReplayResponse, approveReplay)
	if approveReplayResponse.Code != http.StatusOK || approveReplayResponse.Header().Get("ETag") != `"5"` ||
		approveReplayResponse.Body.String() != approveResponse.Body.String() || health.adminDecisionCalls != 1 {
		t.Fatalf("approve replay status=%d etag=%q calls=%d body=%s", approveReplayResponse.Code, approveReplayResponse.Header().Get("ETag"), health.adminDecisionCalls, approveReplayResponse.Body.String())
	}

	reject := newJSONRequest(http.MethodPost, "/api/v1/admin/api-service-health-probes/"+health.config.ID+"/reject", `{"reason":"origin ownership not established"}`)
	addAuth(reject, admin, "unused")
	reject.Header.Set("If-Match", `"5"`)
	rejectResponse := httptest.NewRecorder()
	handler.ServeHTTP(rejectResponse, reject)
	if rejectResponse.Code != http.StatusOK || rejectResponse.Header().Get("ETag") != `"6"` || health.adminApprove || health.adminVersion != 5 {
		t.Fatalf("reject status=%d etag=%q approve=%v version=%d body=%s", rejectResponse.Code, rejectResponse.Header().Get("ETag"), health.adminApprove, health.adminVersion, rejectResponse.Body.String())
	}
}

func testAPIHealthConfig(now time.Time) apihealth.Config {
	return apihealth.Config{
		ID: "11111111-1111-4111-8111-111111111111", APIServiceID: "22222222-2222-4222-8222-222222222222",
		OwnerUserID: "33333333-3333-4333-8333-333333333333", ServiceTitle: "Managed OpenAI API",
		OwnerUsername: "probe-owner", OwnerDisplayName: "Probe Owner",
		Protocol: apihealth.ProtocolOpenAIChatCompletionsV1, BaseURL: "https://example.com/v1",
		NormalizedOrigin: "https://example.com:443", Model: "gpt-5", CredentialConfigured: true,
		Enabled: true, AuthorizationStatus: apihealth.AuthorizationPending, MeasurementVersion: 1,
		Version: 4, CreatedAt: now.Add(-time.Hour), UpdatedAt: now,
	}
}

func assertAPIHealthPrivateNoStore(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	if value := response.Header().Get("Cache-Control"); value != "private, no-store" {
		t.Fatalf("expected private no-store, got %q", value)
	}
}
