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
	if !sessionCookie.Secure || !sessionCookie.HttpOnly || sessionCookie.SameSite != http.SameSiteLaxMode || sessionCookie.MaxAge != 7*24*60*60 {
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

func TestOAuthCallbackRedirectsToConfiguredFrontendOrigin(t *testing.T) {
	tests := []struct {
		name     string
		returnTo string
		want     string
	}{
		{name: "preserves safe frontend path", returnTo: "/market?tab=api", want: "https://staging.c2cmarket.shop/market?tab=api"},
		{name: "rejects protocol relative target", returnTo: "//evil.example/path", want: "https://staging.c2cmarket.shop/"},
		{name: "rejects absolute target", returnTo: "https://evil.example/path", want: "https://staging.c2cmarket.shop/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer(app.NewService(), ServerOptions{
				FrontendOrigin: "https://staging.c2cmarket.shop",
				OAuth:          OAuthOptions{ProviderMode: "fake"},
			})
			state := "oauth-state"
			request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/callback?state="+state+"&code=test-user", nil)
			request.AddCookie(&http.Cookie{Name: oauthStateCookieName, Value: state + "|" + tt.returnTo})
			response := httptest.NewRecorder()

			server.ServeHTTP(response, request)

			if response.Code != http.StatusFound {
				t.Fatalf("expected redirect, got %d body %s", response.Code, response.Body.String())
			}
			if location := response.Header().Get("Location"); location != tt.want {
				t.Fatalf("expected redirect to %q, got %q", tt.want, location)
			}
		})
	}
}

func TestRateLimitedEndpointReturnsProblem429(t *testing.T) {
	server := &Server{
		app:              app.NewService(),
		rateLimiter:      middleware.NewRateLimiter(time.Minute),
		clientIPResolver: middleware.NewClientIPResolver(false, nil),
	}
	handler := server.limitHandler("test_rate_limit", 1, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	wrapped := middleware.WithRequestID(
		middleware.WithClientIP(server.clientIPResolver, http.HandlerFunc(handler)),
	)
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

func TestRateLimitSeparatesAuthenticatedUserAndSharedIPBudgets(t *testing.T) {
	service := app.NewService()
	server := &Server{
		app:         service,
		rateLimiter: middleware.NewRateLimiter(time.Minute),
	}
	_, firstSession, appErr := service.CreateDevSession(context.Background(), "rate-user-one", false)
	if appErr != nil {
		t.Fatalf("create first session: %v", appErr)
	}
	_, secondSession, appErr := service.CreateDevSession(context.Background(), "rate-user-two", false)
	if appErr != nil {
		t.Fatalf("create second session: %v", appErr)
	}
	_, thirdSession, appErr := service.CreateDevSession(context.Background(), "rate-user-three", false)
	if appErr != nil {
		t.Fatalf("create third session: %v", appErr)
	}
	handler := server.limitHandlerByActor("test_actor_rate_limit", 2, 1, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	wrapped := middleware.WithRequestID(http.HandlerFunc(handler))

	request := func(sessionID, remoteAddr string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(http.MethodPost, "/test-actor-rate-limit", nil)
		r.RemoteAddr = remoteAddr
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: sessionID})
		response := httptest.NewRecorder()
		wrapped.ServeHTTP(response, r)
		return response
	}

	if response := request(firstSession.ID, "203.0.113.10:4001"); response.Code != http.StatusNoContent {
		t.Fatalf("first buyer expected success, got %d body %s", response.Code, response.Body.String())
	}
	if response := request(firstSession.ID, "203.0.113.11:4002"); response.Code != http.StatusTooManyRequests {
		t.Fatalf("same buyer must hit user limit across IPs, got %d body %s", response.Code, response.Body.String())
	}
	if response := request(secondSession.ID, "203.0.113.10:4003"); response.Code != http.StatusNoContent {
		t.Fatalf("second buyer on shared IP expected success, got %d body %s", response.Code, response.Body.String())
	}
	if response := request(thirdSession.ID, "203.0.113.10:4004"); response.Code != http.StatusTooManyRequests {
		t.Fatalf("shared IP must eventually hit its independent limit, got %d body %s", response.Code, response.Body.String())
	}
}

func TestJSONRequestBodyStrictParsingFailures(t *testing.T) {
	server := newTestServer(time.Now())
	cases := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "empty body", body: "", wantStatus: http.StatusBadRequest},
		{name: "malformed json", body: `{"username":`, wantStatus: http.StatusBadRequest},
		{name: "multiple json objects", body: `{"username":"admin","password":"password"} {}`, wantStatus: http.StatusBadRequest},
		{name: "oversized json body", body: `{"username":"` + strings.Repeat("a", 1<<20) + `"}`, wantStatus: http.StatusRequestEntityTooLarge},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := newJSONRequest(http.MethodPost, "/api/v1/auth/password/login", tc.body)
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)

			if response.Code != tc.wantStatus {
				t.Fatalf("expected status %d, got %d body %s", tc.wantStatus, response.Code, response.Body.String())
			}
			assertProblemCode(t, response, domain.CodeValidationFailed)
		})
	}
}

func TestRateLimitIgnoresForgedForwardingHeadersByDefault(t *testing.T) {
	server := &Server{
		app:              app.NewService(),
		rateLimiter:      middleware.NewRateLimiter(time.Minute),
		clientIPResolver: middleware.NewClientIPResolver(false, nil),
	}
	handler := server.limitHandler("test_forged_xff", 1, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	wrapped := middleware.WithRequestID(
		middleware.WithClientIP(server.clientIPResolver, http.HandlerFunc(handler)),
	)

	for i, forwardedFor := range []string{"198.51.100.10", "198.51.100.11"} {
		request := httptest.NewRequest(http.MethodGet, "/test-forged-xff", nil)
		request.RemoteAddr = "203.0.113.10:4321"
		request.Header.Set("X-Forwarded-For", forwardedFor)
		request.Header.Set("X-Real-IP", forwardedFor)
		response := httptest.NewRecorder()
		wrapped.ServeHTTP(response, request)

		if i == 0 && response.Code != http.StatusNoContent {
			t.Fatalf("first request expected no content, got %d body %s", response.Code, response.Body.String())
		}
		if i == 1 {
			if response.Code != http.StatusTooManyRequests {
				t.Fatalf("forged XFF must not bypass rate limit, got %d body %s", response.Code, response.Body.String())
			}
			assertProblemCode(t, response, domain.CodeRateLimited)
		}
	}
}

func TestTrustedProxyForwardingHeadersAffectRateLimitOnlyForTrustedPeer(t *testing.T) {
	server := &Server{
		app:              app.NewService(),
		rateLimiter:      middleware.NewRateLimiter(time.Minute),
		clientIPResolver: middleware.NewClientIPResolver(true, []string{"10.0.0.0/24"}),
	}
	handler := server.limitHandler("test_trusted_xff", 1, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	wrapped := middleware.WithRequestID(
		middleware.WithClientIP(server.clientIPResolver, http.HandlerFunc(handler)),
	)

	for _, forwardedFor := range []string{"198.51.100.10", "198.51.100.11"} {
		request := httptest.NewRequest(http.MethodGet, "/test-trusted-xff", nil)
		request.RemoteAddr = "10.0.0.9:4321"
		request.Header.Set("X-Forwarded-For", forwardedFor)
		response := httptest.NewRecorder()
		wrapped.ServeHTTP(response, request)
		if response.Code != http.StatusNoContent {
			t.Fatalf("trusted proxy request for %s expected no content, got %d body %s", forwardedFor, response.Code, response.Body.String())
		}
	}

	untrustedFirst := httptest.NewRequest(http.MethodGet, "/test-trusted-xff", nil)
	untrustedFirst.RemoteAddr = "203.0.113.10:4321"
	untrustedFirst.Header.Set("X-Forwarded-For", "198.51.100.12")
	untrustedFirstResponse := httptest.NewRecorder()
	wrapped.ServeHTTP(untrustedFirstResponse, untrustedFirst)
	if untrustedFirstResponse.Code != http.StatusNoContent {
		t.Fatalf("untrusted first request expected no content, got %d body %s", untrustedFirstResponse.Code, untrustedFirstResponse.Body.String())
	}

	untrustedSecond := httptest.NewRequest(http.MethodGet, "/test-trusted-xff", nil)
	untrustedSecond.RemoteAddr = "203.0.113.10:4321"
	untrustedSecond.Header.Set("X-Forwarded-For", "198.51.100.13")
	untrustedSecondResponse := httptest.NewRecorder()
	wrapped.ServeHTTP(untrustedSecondResponse, untrustedSecond)
	if untrustedSecondResponse.Code != http.StatusTooManyRequests {
		t.Fatalf("untrusted peer must ignore XFF and rate-limit remote address, got %d body %s", untrustedSecondResponse.Code, untrustedSecondResponse.Body.String())
	}
	assertProblemCode(t, untrustedSecondResponse, domain.CodeRateLimited)
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
