package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/report"
)

func TestAccountAppealOAuthUsesIsolatedFixedExpirySession(t *testing.T) {
	now := time.Date(2026, 8, 3, 8, 0, 0, 0, time.UTC)
	server := newTestServer(now)
	start := httptest.NewRequest(http.MethodGet, "/api/v1/auth/account-appeal/start?returnTo=//evil.example", nil)
	startResponse := httptest.NewRecorder()
	server.ServeHTTP(startResponse, start)
	if startResponse.Code != http.StatusOK {
		t.Fatalf("account appeal OAuth start status %d body %s", startResponse.Code, startResponse.Body.String())
	}
	var startPayload oauthStartResponse
	if err := json.NewDecoder(startResponse.Body).Decode(&startPayload); err != nil {
		t.Fatalf("decode account appeal OAuth start: %v", err)
	}
	if !strings.Contains(startPayload.AuthorizationURL, "/api/v1/auth/oauth/callback?") {
		t.Fatalf("unexpected account appeal authorization URL %q", startPayload.AuthorizationURL)
	}
	var stateCookie *http.Cookie
	for _, cookie := range startResponse.Result().Cookies() {
		if cookie.Name == oauthStateCookieName {
			stateCookie = cookie
		}
	}
	if stateCookie == nil {
		t.Fatal("account appeal OAuth start did not issue state cookie")
	}
	statePayload, ok := decodeOAuthStateCookie(stateCookie.Value)
	if !ok || statePayload.Purpose != oauthPurposeAccountAppeal || statePayload.ReturnTo != accountAppealFrontendPath || statePayload.InviteCode != "" {
		t.Fatalf("unexpected account appeal OAuth state: %+v valid=%t", statePayload, ok)
	}

	admin := createSession(t, server, "account-appeal-admin", true)
	member := createLinuxDoSession(t, server, "account-appeal-member")

	suspend := newJSONRequest(http.MethodPost, "/api/v1/admin/users/"+member.userID+"/status", `{"status":"suspended","reason":"测试受限账号申诉入口"}`)
	suspend.Header.Set("If-Match", `"1"`)
	addAuth(suspend, admin, "account-appeal-suspend")
	suspendResponse := httptest.NewRecorder()
	server.ServeHTTP(suspendResponse, suspend)
	if suspendResponse.Code != http.StatusOK {
		t.Fatalf("suspend account status %d body %s", suspendResponse.Code, suspendResponse.Body.String())
	}

	state := "account-appeal-state"
	callback := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/callback?state="+url.QueryEscape(state)+"&code=account-appeal-member", nil)
	callback.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: encodeOAuthStateCookie(oauthStateCookiePayload{
		State:    state,
		ReturnTo: accountAppealFrontendPath,
		Purpose:  oauthPurposeAccountAppeal,
	})})
	callbackResponse := httptest.NewRecorder()
	server.ServeHTTP(callbackResponse, callback)
	if callbackResponse.Code != http.StatusFound {
		t.Fatalf("account appeal callback status %d body %s", callbackResponse.Code, callbackResponse.Body.String())
	}
	if location := callbackResponse.Header().Get("Location"); location != "/account-appeal?accountAppealOutcome=verified" {
		t.Fatalf("account appeal callback location = %q", location)
	}

	var appealCookie *http.Cookie
	for _, cookie := range callbackResponse.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			t.Fatal("account appeal callback must not issue an ordinary session cookie")
		}
		if cookie.Name == accountAppealCookieName && cookie.Value != "" {
			appealCookie = cookie
		}
	}
	if appealCookie == nil {
		t.Fatal("account appeal callback did not issue the dedicated cookie")
	}
	if appealCookie.Path != accountAppealCookiePath || !appealCookie.HttpOnly || appealCookie.SameSite != http.SameSiteLaxMode || appealCookie.MaxAge != 15*60 {
		t.Fatalf("unexpected account appeal cookie attributes: %+v", appealCookie)
	}
	if !appealCookie.Expires.Equal(now.Add(15 * time.Minute)) {
		t.Fatalf("account appeal cookie expires at %s, want %s", appealCookie.Expires, now.Add(15*time.Minute))
	}

	firstSession := readAccountAppealSession(t, server, appealCookie)
	secondSession := readAccountAppealSession(t, server, appealCookie)
	if firstSession.CSRFToken == secondSession.CSRFToken {
		t.Fatal("account appeal session read must rotate its dedicated CSRF token")
	}
	if firstSession.ExpiresAt != secondSession.ExpiresAt || firstSession.ExpiresAt != now.Add(15*time.Minute).Format(time.RFC3339) {
		t.Fatalf("account appeal session read renewed expiry: first=%s second=%s", firstSession.ExpiresAt, secondSession.ExpiresAt)
	}

	ordinarySession := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	ordinarySession.AddCookie(appealCookie)
	ordinarySessionResponse := httptest.NewRecorder()
	server.ServeHTTP(ordinarySessionResponse, ordinarySession)
	if ordinarySessionResponse.Code != http.StatusUnauthorized {
		t.Fatalf("dedicated cookie authenticated ordinary route: %d body %s", ordinarySessionResponse.Code, ordinarySessionResponse.Body.String())
	}

	wrongCSRFHeader := newJSONRequest(http.MethodPost, "/api/v1/account-appeal/appeals", `{"statement":"请复核账号治理处理依据。"}`)
	wrongCSRFHeader.AddCookie(appealCookie)
	wrongCSRFHeader.Header.Set(csrfHeaderName, secondSession.CSRFToken)
	wrongCSRFHeader.Header.Set("Idempotency-Key", "account-appeal-wrong-csrf-header")
	wrongCSRFResponse := httptest.NewRecorder()
	server.ServeHTTP(wrongCSRFResponse, wrongCSRFHeader)
	if wrongCSRFResponse.Code != http.StatusForbidden {
		t.Fatalf("ordinary CSRF header authorized account appeal mutation: %d body %s", wrongCSRFResponse.Code, wrongCSRFResponse.Body.String())
	}
}

func TestAccountAppealOAuthUsesGenericIneligibleRedirect(t *testing.T) {
	server := newTestServer(time.Date(2026, 8, 3, 8, 0, 0, 0, time.UTC))

	for _, code := range []string{"unknown-account-appeal-user", "active-account-appeal-user"} {
		if strings.HasPrefix(code, "active") {
			createLinuxDoSession(t, server, code)
		}
		state := "state-" + code
		request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/callback?state="+url.QueryEscape(state)+"&code="+url.QueryEscape(code), nil)
		request.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: encodeOAuthStateCookie(oauthStateCookiePayload{
			State: state, Purpose: oauthPurposeAccountAppeal, ReturnTo: accountAppealFrontendPath,
		})})
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request)
		if response.Code != http.StatusFound || response.Header().Get("Location") != "/account-appeal?accountAppealOutcome=ineligible" {
			t.Fatalf("%s account appeal result status=%d location=%q body=%s", code, response.Code, response.Header().Get("Location"), response.Body.String())
		}
		for _, cookie := range response.Result().Cookies() {
			if cookie.Name == sessionCookieName || (cookie.Name == accountAppealCookieName && cookie.Value != "") {
				t.Fatalf("ineligible account appeal issued an authentication cookie: %+v", cookie)
			}
		}
	}
}

func TestAccountAppealCookieIsSecureInProduction(t *testing.T) {
	now := time.Date(2026, 8, 3, 8, 0, 0, 0, time.UTC)
	response := httptest.NewRecorder()
	server := &Server{cookieSecure: true}
	server.setAccountAppealCookie(response, auth.AccountAppealSession{
		ID:        "appeal_session_test",
		CreatedAt: now,
		ExpiresAt: now.Add(15 * time.Minute),
	})
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].Secure || cookies[0].Path != accountAppealCookiePath {
		t.Fatalf("production account appeal cookie attributes: %+v", cookies)
	}
}

func TestAccountGovernanceAppealCompletionIsSelfSafe(t *testing.T) {
	createdAt := time.Date(2026, 8, 3, 8, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Minute)
	completion, appErr := accountGovernanceAppealCompletionBuilder(report.Appeal{
		ID:                "appeal-1",
		AppellantUserID:   "user-1",
		AppellantUsername: "restricted-user",
		AppellantName:     "Restricted User",
		TargetType:        report.TargetAccountGovernance,
		TargetID:          "user-1",
		Title:             "账号治理申诉",
		Statement:         "仅管理员可见的申诉陈述",
		Status:            report.AppealStatusSubmitted,
		AdminReason:       "仅管理员可见的处理理由",
		HandledByAdminID:  "admin-1",
		CreatedAt:         createdAt,
		UpdatedAt:         updatedAt,
		Version:           2,
	})
	if appErr != nil {
		t.Fatalf("build account-governance appeal completion: %v", appErr)
	}
	if completion.Status != http.StatusCreated || completion.Headers["ETag"] != `"2"` {
		t.Fatalf("unexpected account-governance completion metadata: %+v", completion)
	}

	var payload map[string]any
	if err := json.Unmarshal(completion.Body, &payload); err != nil {
		t.Fatalf("decode account-governance appeal completion: %v", err)
	}
	want := map[string]any{
		"id":         "appeal-1",
		"targetType": report.TargetAccountGovernance,
		"targetId":   "user-1",
		"title":      "账号治理申诉",
		"status":     report.AppealStatusSubmitted,
		"createdAt":  createdAt.Format(time.RFC3339),
		"updatedAt":  updatedAt.Format(time.RFC3339),
		"version":    float64(2),
	}
	if len(payload) != len(want) {
		t.Fatalf("account-governance completion exposed unexpected fields: %#v", payload)
	}
	for key, wantValue := range want {
		if got := payload[key]; got != wantValue {
			t.Fatalf("account-governance completion field %s = %#v, want %#v", key, got, wantValue)
		}
	}
}

func readAccountAppealSession(t *testing.T, server http.Handler, cookie *http.Cookie) accountAppealSessionResponse {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/account-appeal/session", nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("account appeal session status %d body %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("account appeal session cache control = %q", response.Header().Get("Cache-Control"))
	}
	var payload accountAppealSessionResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode account appeal session: %v", err)
	}
	return payload
}
