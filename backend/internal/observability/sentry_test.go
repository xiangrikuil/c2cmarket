package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/middleware"
	"github.com/getsentry/sentry-go"
)

type recordingSentryTransport struct {
	events []*sentry.Event
}

func (transport *recordingSentryTransport) Flush(time.Duration) bool              { return true }
func (transport *recordingSentryTransport) FlushWithContext(context.Context) bool { return true }
func (transport *recordingSentryTransport) Configure(sentry.ClientOptions)        {}
func (transport *recordingSentryTransport) Close()                                {}
func (transport *recordingSentryTransport) SendEvent(event *sentry.Event) {
	transport.events = append(transport.events, event)
}

func TestSanitizeSentryEventRemovesRequestAndUserData(t *testing.T) {
	event := &sentry.Event{
		User: sentry.User{Email: "student@example.test", IPAddress: "127.0.0.1"},
		Request: &sentry.Request{
			URL:         "https://api.c2cmarket.shop/api/v1/search?q=private",
			QueryString: "q=private",
			Data:        `{"password":"secret"}`,
			Cookies:     "session=secret",
			Headers:     map[string]string{"Authorization": "Bearer secret"},
			Env:         map[string]string{"REMOTE_ADDR": "127.0.0.1"},
		},
		Breadcrumbs: []*sentry.Breadcrumb{{
			Data: map[string]interface{}{
				"method":        "GET",
				"authorization": "Bearer secret",
				"request_body":  `{"password":"secret"}`,
			},
		}},
	}

	sanitized := sanitizeSentryEvent(event, nil)
	if !sanitized.User.IsEmpty() {
		t.Fatalf("Sentry user data was not removed: %+v", sanitized.User)
	}
	if sanitized.Request.URL != "" || sanitized.Request.QueryString != "" || sanitized.Request.Data != "" ||
		sanitized.Request.Cookies != "" || sanitized.Request.Headers != nil || sanitized.Request.Env != nil {
		t.Fatalf("Sentry request data was not removed: %+v", sanitized.Request)
	}
	if _, exists := sanitized.Breadcrumbs[0].Data["authorization"]; exists {
		t.Fatal("authorization breadcrumb was not removed")
	}
	if _, exists := sanitized.Breadcrumbs[0].Data["request_body"]; exists {
		t.Fatal("request body breadcrumb was not removed")
	}
	if sanitized.Breadcrumbs[0].Data["method"] != "GET" {
		t.Fatal("non-sensitive breadcrumb data should be preserved")
	}
}

func TestCaptureServerProblemReportsOnlyRedacted5xx(t *testing.T) {
	transport := &recordingSentryTransport{}
	client, err := sentry.NewClient(sentry.ClientOptions{
		Dsn:              "https://public@example.ingest.sentry.io/123",
		AttachStacktrace: true,
		Transport:        transport,
		BeforeSend:       sanitizeSentryEvent,
	})
	if err != nil {
		t.Fatalf("create Sentry client: %v", err)
	}
	hub := sentry.NewHub(client, sentry.NewScope())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/private-resource?token=secret", nil)
	request = request.WithContext(sentry.SetHubOnContext(request.Context(), hub))
	request = request.WithContext(middleware.WithRequestIDContext(request.Context(), "req_sentry_test"))

	captureServerProblem(request, domain.NewError(http.StatusBadRequest, "INVALID_INPUT", "Invalid", "private detail"))
	if len(transport.events) != 0 {
		t.Fatalf("expected 4xx to be ignored, captured %d events", len(transport.events))
	}

	captureServerProblem(request, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal", "private database detail"))
	if len(transport.events) != 1 {
		t.Fatalf("expected one 5xx event, captured %d", len(transport.events))
	}
	event := transport.events[0]
	if event.Message != "HTTP server error: INTERNAL_ERROR" {
		t.Fatalf("unexpected sanitized message: %q", event.Message)
	}
	if event.Tags["request_id"] != "req_sentry_test" || event.Tags["status"] != "500" {
		t.Fatalf("missing bounded Sentry tags: %+v", event.Tags)
	}
	if event.Request != nil && (event.Request.URL != "" || event.Request.QueryString != "" || event.Request.Data != "") {
		t.Fatalf("request data leaked into Sentry event: %+v", event.Request)
	}
}
