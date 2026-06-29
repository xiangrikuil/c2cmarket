package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/config"
	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/middleware"
	app "c2c-market/backend/internal/module/core"
)

func TestProductionSessionCookieIsSecureAndLogoutClearsWithSameAttributes(t *testing.T) {
	server := NewServer(app.NewService(), ServerOptions{
		EnableDevAuth:  true,
		AppEnv:         config.EnvProduction,
		AllowedOrigins: []string{"https://app.example"},
	})

	request := newJSONRequest(http.MethodPost, "/api/v1/auth/dev-session", `{"username":"buyer"}`)
	request.Header.Set("Origin", "https://app.example")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("dev session status %d body %s", response.Code, response.Body.String())
	}
	sessionCookie := findCookie(t, response.Result().Cookies(), sessionCookieName)
	if !sessionCookie.Secure || !sessionCookie.HttpOnly || sessionCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("unexpected production session cookie: %+v", sessionCookie)
	}
	var payload sessionResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode session: %v", err)
	}

	logout := newJSONRequest(http.MethodPost, "/api/v1/auth/logout", `{}`)
	logout.Header.Set("Origin", "https://app.example")
	logout.AddCookie(sessionCookie)
	logout.Header.Set(csrfHeaderName, payload.CSRFToken)
	logoutResponse := httptest.NewRecorder()
	server.ServeHTTP(logoutResponse, logout)
	if logoutResponse.Code != http.StatusNoContent {
		t.Fatalf("logout status %d body %s", logoutResponse.Code, logoutResponse.Body.String())
	}
	clearCookie := findCookie(t, logoutResponse.Result().Cookies(), sessionCookieName)
	if !clearCookie.Secure || !clearCookie.HttpOnly || clearCookie.SameSite != http.SameSiteLaxMode || clearCookie.MaxAge != -1 {
		t.Fatalf("unexpected production clear cookie: %+v", clearCookie)
	}
}

func TestProductionOriginRejectsUnsafeBrowserRequest(t *testing.T) {
	server := NewServer(app.NewService(), ServerOptions{
		EnableDevAuth:  true,
		AppEnv:         config.EnvProduction,
		AllowedOrigins: []string{"https://app.example"},
	})
	request := newJSONRequest(http.MethodPost, "/api/v1/auth/dev-session", `{"username":"buyer"}`)
	request.Header.Set("Origin", "https://evil.example")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden origin, got %d body %s", response.Code, response.Body.String())
	}
	assertProblemCode(t, response, domain.CodeCSRFTokenInvalid)
}

func TestRateLimitedEndpointReturnsProblem429(t *testing.T) {
	server := &Server{
		app:         app.NewService(),
		rateLimiter: middleware.NewRateLimiter(time.Minute),
	}
	handler := server.limitHandler("test_rate_limit", 1, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	wrapped := middleware.WithRequestID(http.HandlerFunc(handler))
	for i := 0; i < 2; i++ {
		request := httptest.NewRequest(http.MethodGet, "/test-rate-limit", nil)
		response := httptest.NewRecorder()
		wrapped.ServeHTTP(response, request)
		if i == 0 && response.Code != http.StatusNoContent {
			t.Fatalf("request %d expected no content, got %d body %s", i, response.Code, response.Body.String())
		}
		if i == 1 {
			if response.Code != http.StatusTooManyRequests {
				t.Fatalf("expected 429, got %d body %s", response.Code, response.Body.String())
			}
			assertProblemCode(t, response, domain.CodeRateLimited)
		}
	}
}

func TestFetchOAuthJSONRejectsOversizedBody(t *testing.T) {
	payload := strings.Repeat("x", oauthMaxResponseBodyBytes+1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(payload))
	}))
	defer upstream.Close()

	server := &Server{oauthHTTPClient: upstream.Client()}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, upstream.URL, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	var target map[string]any
	appErr := server.fetchOAuthJSON(request, &target)
	if appErr == nil || appErr.Status != http.StatusBadGateway || appErr.Code != domain.CodeInternalError {
		t.Fatalf("expected bad gateway oversized oauth response, got %v", appErr)
	}
}

func TestPaginateSliceUsesOpaqueCursorAndValidatesInput(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/search?limit=2", nil)
	page, appErr := paginateSlice(request, []int{1, 2, 3})
	if appErr != nil {
		t.Fatalf("paginate first page: %v", appErr)
	}
	if len(page.Items) != 2 || page.Items[0] != 1 || page.NextCursor == nil {
		t.Fatalf("unexpected first page: %+v", page)
	}
	next := httptest.NewRequest(http.MethodGet, "/api/v1/search?limit=2&cursor="+*page.NextCursor, nil)
	second, appErr := paginateSlice(next, []int{1, 2, 3})
	if appErr != nil {
		t.Fatalf("paginate second page: %v", appErr)
	}
	if len(second.Items) != 1 || second.Items[0] != 3 || second.NextCursor != nil {
		t.Fatalf("unexpected second page: %+v", second)
	}

	invalid := httptest.NewRequest(http.MethodGet, "/api/v1/search?limit=101", nil)
	if _, appErr := paginateSlice(invalid, []int{1}); appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("expected invalid limit error, got %v", appErr)
	}
	badCursor := httptest.NewRequest(http.MethodGet, "/api/v1/search?cursor=bad", nil)
	if _, appErr := paginateSlice(badCursor, []int{1}); appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("expected invalid cursor error, got %v", appErr)
	}
}

func TestRateLimiterKeyUsesWindow(t *testing.T) {
	now := time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC)
	limiter := middleware.NewRateLimiterWithClock(time.Minute, func() time.Time { return now })
	if decision := limiter.Allow("key", 1); !decision.Allowed {
		t.Fatalf("expected first request allowed")
	}
	if decision := limiter.Allow("key", 1); decision.Allowed {
		t.Fatalf("expected second request rejected")
	}
	now = now.Add(time.Minute)
	if decision := limiter.Allow("key", 1); !decision.Allowed {
		t.Fatalf("expected new window allowed")
	}
}

func findCookie(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("cookie %s not found in %+v", name, cookies)
	return nil
}
