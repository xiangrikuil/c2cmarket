package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	app "c2c-market/backend/internal/module/core"
)

func TestAuthenticatedRequestRenewsSessionAtMostOncePerDay(t *testing.T) {
	current := time.Date(2026, 7, 21, 8, 0, 0, 0, time.UTC)
	server := NewServer(app.NewServiceWithClock(func() time.Time { return current }), ServerOptions{EnableDevAuth: true})
	session := createSession(t, server, "session-renewal-user", false)

	current = current.Add(24 * time.Hour)
	excluded := httptest.NewRequest(http.MethodGet, "/api/v1/auth/session", nil)
	addCookie(excluded, session.cookie)
	excludedResponse := httptest.NewRecorder()
	server.ServeHTTP(excludedResponse, excluded)
	if excludedResponse.Code != http.StatusOK {
		t.Fatalf("session read status %d body %s", excludedResponse.Code, excludedResponse.Body.String())
	}
	if cookie := responseSessionCookie(excludedResponse); cookie != nil {
		t.Fatalf("session read must not renew cookie: %+v", cookie)
	}

	active := httptest.NewRequest(http.MethodGet, "/api/v1/me/profile", nil)
	addCookie(active, session.cookie)
	activeResponse := httptest.NewRecorder()
	server.ServeHTTP(activeResponse, active)
	if activeResponse.Code != http.StatusOK {
		t.Fatalf("authenticated read status %d body %s", activeResponse.Code, activeResponse.Body.String())
	}
	renewedCookie := responseSessionCookie(activeResponse)
	if renewedCookie == nil {
		t.Fatal("expected authenticated read to renew session cookie")
	}
	if renewedCookie.MaxAge != 7*24*60*60 || !renewedCookie.Expires.Equal(current.Add(7*24*time.Hour)) {
		t.Fatalf("unexpected renewed cookie: %+v", renewedCookie)
	}

	repeated := httptest.NewRequest(http.MethodGet, "/api/v1/me/profile", nil)
	addCookie(repeated, session.cookie)
	repeatedResponse := httptest.NewRecorder()
	server.ServeHTTP(repeatedResponse, repeated)
	if repeatedResponse.Code != http.StatusOK {
		t.Fatalf("repeated authenticated read status %d body %s", repeatedResponse.Code, repeatedResponse.Body.String())
	}
	if cookie := responseSessionCookie(repeatedResponse); cookie != nil {
		t.Fatalf("session must not renew twice within 24 hours: %+v", cookie)
	}
}

func TestSessionRenewalRequestPolicy(t *testing.T) {
	tests := []struct {
		method string
		path   string
		want   bool
	}{
		{method: http.MethodGet, path: "/api/v1/me/profile", want: true},
		{method: http.MethodPost, path: "/api/v1/me/profile", want: true},
		{method: http.MethodOptions, path: "/api/v1/me/profile", want: false},
		{method: http.MethodGet, path: "/health", want: false},
		{method: http.MethodGet, path: "/readyz", want: false},
		{method: http.MethodGet, path: "/assets/app.js", want: false},
		{method: http.MethodPost, path: "/api/v1/auth/dev-session", want: false},
		{method: http.MethodPost, path: "/api/v1/auth/password/login", want: false},
		{method: http.MethodGet, path: "/api/v1/auth/oauth/start", want: false},
		{method: http.MethodGet, path: "/api/v1/auth/oauth/callback", want: false},
		{method: http.MethodGet, path: "/api/v1/auth/session", want: false},
		{method: http.MethodPost, path: "/api/v1/auth/session/renew", want: false},
		{method: http.MethodPost, path: "/api/v1/auth/logout", want: false},
		{method: http.MethodGet, path: "/api/v1/me/events", want: false},
		{method: http.MethodGet, path: "/api/v1/me/navigation-badges", want: false},
		{method: http.MethodGet, path: "/api/v1/me/feedback-tickets/unread-count", want: false},
		{method: http.MethodGet, path: "/api/v1/me/notifications/unread-count", want: false},
		{method: http.MethodGet, path: "/api/v1/me/announcements/unread-count", want: false},
		{method: http.MethodGet, path: "/api/v1/me/announcements/important-unread-count", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path, func(t *testing.T) {
			request := httptest.NewRequest(tt.method, tt.path, nil)
			if got := shouldRenewSessionForRequest(request); got != tt.want {
				t.Fatalf("shouldRenewSessionForRequest()=%v want %v", got, tt.want)
			}
		})
	}
}

func responseSessionCookie(response *httptest.ResponseRecorder) *http.Cookie {
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == sessionCookieName {
			return cookie
		}
	}
	return nil
}
