package server

import (
	"context"
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
	connection            apihealth.Connection
	found                 bool
	input                 apihealth.ConnectionInput
	expectedVersion       int64
	verifyExpectedVersion int64
	deleteExpectedVersion int64
	verifyCalls           int
}

func (service *apiHealthRouteService) OwnerConnections(context.Context, auth.User) ([]apihealth.Connection, *domain.AppError) {
	return []apihealth.Connection{service.connection}, nil
}
func (service *apiHealthRouteService) OwnerConnection(context.Context, auth.User, string) (apihealth.Connection, bool, *domain.AppError) {
	return service.connection, service.found, nil
}
func (service *apiHealthRouteService) CreateOwnerConnection(_ context.Context, _ auth.User, input apihealth.ConnectionInput) (apihealth.Connection, *domain.AppError) {
	service.input = input
	connection := service.connection
	connection.Version++
	return connection, nil
}
func (service *apiHealthRouteService) UpdateOwnerConnection(_ context.Context, _ auth.User, _ string, input apihealth.ConnectionInput, expectedVersion int64) (apihealth.Connection, *domain.AppError) {
	service.input = input
	service.expectedVersion = expectedVersion
	connection := service.connection
	connection.Name = input.Name
	connection.BaseURL = input.BaseURL
	connection.CredentialConfigured = input.Credential != nil
	connection.Enabled = input.Enabled
	connection.Version = expectedVersion + 1
	return connection, nil
}
func (service *apiHealthRouteService) VerifyOwnerConnection(_ context.Context, _ auth.User, _ string, expectedVersion int64) (apihealth.Connection, *domain.AppError) {
	service.verifyExpectedVersion = expectedVersion
	service.verifyCalls++
	connection := service.connection
	connection.Version = expectedVersion + 1
	connection.VerificationStatus = apihealth.VerificationVerified
	return connection, nil
}
func (service *apiHealthRouteService) DeleteOwnerConnection(_ context.Context, _ auth.User, _ string, expectedVersion int64) *domain.AppError {
	service.deleteExpectedVersion = expectedVersion
	return nil
}
func (service *apiHealthRouteService) Summaries(context.Context, []string) (map[string]apihealth.Summary, *domain.AppError) {
	return map[string]apihealth.Summary{}, nil
}

func TestOwnerAPIProbeConnectionRoutesKeepCredentialWriteOnly(t *testing.T) {
	now := time.Date(2026, 8, 8, 5, 0, 0, 0, time.UTC)
	health := &apiHealthRouteService{found: true, connection: testAPIProbeConnection(now)}
	handler := NewServer(app.NewServiceWithClock(func() time.Time { return now }), ServerOptions{EnableDevAuth: true, APIHealth: health})
	owner := createSession(t, handler, "probe-connection-owner", false)

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/owner/api-probe-connections", nil)
	addCookie(listRequest, owner.cookie)
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK || !strings.Contains(listResponse.Body.String(), `"credentialConfigured":true`) {
		t.Fatalf("list status=%d body=%s", listResponse.Code, listResponse.Body.String())
	}
	assertAPIHealthPrivateNoStore(t, listResponse)
	if strings.Contains(listResponse.Body.String(), "probe-secret") || strings.Contains(listResponse.Body.String(), `"credential":`) {
		t.Fatalf("list leaked credential: %s", listResponse.Body.String())
	}

	path := "/api/v1/owner/api-probe-connections/" + health.connection.ID
	putBody := `{"name":"低额度探针","baseUrl":"https://api.example.com/v1","credential":"probe-secret","enabled":true,"acknowledgeInsecureHttp":false}`
	missingCSRF := newJSONRequest(http.MethodPut, path, putBody)
	addCookie(missingCSRF, owner.cookie)
	missingCSRF.Header.Set("If-Match", `"4"`)
	missingCSRFResponse := httptest.NewRecorder()
	handler.ServeHTTP(missingCSRFResponse, missingCSRF)
	if missingCSRFResponse.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF status=%d body=%s", missingCSRFResponse.Code, missingCSRFResponse.Body.String())
	}

	put := newJSONRequest(http.MethodPut, path, putBody)
	addAuth(put, owner, "unused")
	put.Header.Set("If-Match", `"4"`)
	putResponse := httptest.NewRecorder()
	handler.ServeHTTP(putResponse, put)
	if putResponse.Code != http.StatusOK || putResponse.Header().Get("ETag") != `"5"` {
		t.Fatalf("put status=%d etag=%q body=%s", putResponse.Code, putResponse.Header().Get("ETag"), putResponse.Body.String())
	}
	if health.input.Credential == nil || *health.input.Credential != "probe-secret" || health.expectedVersion != 4 {
		t.Fatalf("unexpected input=%+v version=%d", health.input, health.expectedVersion)
	}
	if strings.Contains(putResponse.Body.String(), "probe-secret") || strings.Contains(putResponse.Body.String(), `"credential":`) {
		t.Fatalf("response leaked credential: %s", putResponse.Body.String())
	}

	verify := newJSONRequest(http.MethodPost, path+"/verify", "")
	addAuth(verify, owner, "verify-connection")
	verify.Header.Set("If-Match", `"4"`)
	verifyResponse := httptest.NewRecorder()
	handler.ServeHTTP(verifyResponse, verify)
	if verifyResponse.Code != http.StatusOK || health.verifyExpectedVersion != 4 || health.verifyCalls != 1 {
		t.Fatalf("verify status=%d version=%d calls=%d body=%s", verifyResponse.Code, health.verifyExpectedVersion, health.verifyCalls, verifyResponse.Body.String())
	}

	deleteRequest := httptest.NewRequest(http.MethodDelete, path, nil)
	addAuth(deleteRequest, owner, "unused")
	deleteRequest.Header.Set("If-Match", `"4"`)
	deleteResponse := httptest.NewRecorder()
	handler.ServeHTTP(deleteResponse, deleteRequest)
	if deleteResponse.Code != http.StatusNoContent || health.deleteExpectedVersion != 4 {
		t.Fatalf("delete status=%d version=%d", deleteResponse.Code, health.deleteExpectedVersion)
	}
}

func TestLegacyProbeAndAdminApprovalRoutesAreRemoved(t *testing.T) {
	health := &apiHealthRouteService{found: true, connection: testAPIProbeConnection(time.Now())}
	handler := NewServer(app.NewServiceWithClock(time.Now), ServerOptions{EnableDevAuth: true, APIHealth: health})
	owner := createSession(t, handler, "removed-probe-owner", false)
	for _, path := range []string{
		"/api/v1/owner/api-services/service-1/health-probe",
		"/api/v1/admin/api-service-health-probes",
	} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		addCookie(request, owner.cookie)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Fatalf("legacy path %s status=%d body=%s", path, response.Code, response.Body.String())
		}
	}
}

func testAPIProbeConnection(now time.Time) apihealth.Connection {
	verifiedAt := now.Add(-time.Minute)
	return apihealth.Connection{
		ID: "00000000-0000-0000-0000-000000000811", OwnerUserID: "owner-1", Name: "主 Sub2API",
		BaseURL: "https://api.example.com/v1", NormalizedBaseURL: "https://api.example.com/v1",
		CredentialConfigured: true, Enabled: true, VerificationStatus: apihealth.VerificationVerified,
		VerifiedAt: &verifiedAt, MeasurementVersion: 2, Version: 4,
		References: []apihealth.ServiceReference{{ID: "service-1", Title: "额度服务"}},
		CreatedAt:  now.Add(-time.Hour), UpdatedAt: now,
	}
}

func assertAPIHealthPrivateNoStore(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	if response.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("cache control=%q", response.Header().Get("Cache-Control"))
	}
}
