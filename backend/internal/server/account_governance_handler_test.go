package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAccountGovernanceBusinessCenterAcceptsNormalAudience(t *testing.T) {
	handler := newTestServer(time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC))
	session := createSession(t, handler, "governance-center-normal", false)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me/account-governance/business-center", nil)
	addCookie(request, session.cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("business center status %d body %s", response.Code, response.Body.String())
	}
	var payload accountGovernanceBusinessCenterDTO
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode business center: %v", err)
	}
	if payload.AccountStatus != "active" || payload.ProcessingStatus != "completed" || payload.CurrentAction != nil || payload.Items == nil {
		t.Fatalf("unexpected normal business center: %+v", payload)
	}
}

func TestAccountGovernanceBusinessCenterDoesNotFallbackAcrossAudiences(t *testing.T) {
	handler := newTestServer(time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC))
	session := createSession(t, handler, "governance-center-audience", false)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me/account-governance/business-center", nil)
	addCookie(request, session.cookie)
	request.Header.Set("X-Session-Audience", "restricted_business")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("restricted audience fell back to normal cookie status=%d body=%s", response.Code, response.Body.String())
	}
}
