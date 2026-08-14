package observability

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/middleware"

	"github.com/getsentry/sentry-go"
	"github.com/getsentry/sentry-go/http"
	"github.com/go-chi/chi/v5"
)

type SentryOptions struct {
	Enabled          bool
	DSN              string
	Environment      string
	Release          string
	TracesSampleRate float64
}

func InitSentry(options SentryOptions) (func(), error) {
	if !options.Enabled {
		return func() {}, nil
	}
	off := &sentry.KeyValueCollectionBehavior{Mode: sentry.CollectionOff}
	err := sentry.Init(sentry.ClientOptions{
		Dsn:              strings.TrimSpace(options.DSN),
		Environment:      strings.TrimSpace(options.Environment),
		Release:          strings.TrimSpace(options.Release),
		AttachStacktrace: true,
		EnableTracing:    options.TracesSampleRate > 0,
		TracesSampleRate: options.TracesSampleRate,
		SendDefaultPII:   false,
		DataCollection: &sentry.DataCollection{
			UserInfo:    sentry.Set(false),
			Cookies:     off,
			HTTPHeaders: &sentry.HeaderCollectionConfig{Request: off, Response: off},
			HTTPBodies:  []sentry.BodyType{},
			QueryParams: off,
		},
		BeforeSend:            sanitizeSentryEvent,
		BeforeSendTransaction: sanitizeSentryEvent,
		BeforeBreadcrumb:      sanitizeSentryBreadcrumb,
	})
	if err != nil {
		return nil, err
	}
	return func() {
		sentry.Flush(2 * time.Second)
	}, nil
}

func WithSentry(next http.Handler) http.Handler {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hub := sentry.GetHubFromContext(r.Context()); hub != nil {
			hub.Scope().SetTag("request_id", middleware.RequestIDFromRequest(r))
		}
		next.ServeHTTP(w, r)
		if transaction := sentry.SpanFromContext(r.Context()); transaction != nil {
			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = "unmatched"
			}
			transaction.Name = r.Method + " " + route
			transaction.Source = sentry.SourceRoute
		}
	})
	return sentryhttp.New(sentryhttp.Options{Repanic: true}).Handle(inner)
}

func captureServerProblem(r *http.Request, err error) {
	if r == nil || err == nil {
		return
	}
	status := http.StatusInternalServerError
	code := domain.CodeInternalError
	var appErr *domain.AppError
	if errors.As(err, &appErr) {
		status = appErr.Status
		code = strings.TrimSpace(appErr.Code)
		if status == 0 {
			status = http.StatusInternalServerError
		}
	}
	if status < http.StatusInternalServerError {
		return
	}
	if code == "" {
		code = domain.CodeInternalError
	}
	hub := sentry.GetHubFromContext(r.Context())
	if hub == nil || hub.Client() == nil {
		return
	}
	hub.WithScope(func(scope *sentry.Scope) {
		scope.SetLevel(sentry.LevelError)
		scope.SetTag("request_id", middleware.RequestIDFromRequest(r))
		scope.SetTag("status", strconv.Itoa(status))
		scope.SetTag("error_code", code)
		hub.CaptureMessage("HTTP server error: " + code)
	})
}

func sanitizeSentryEvent(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	event.User = sentry.User{}
	if event.Request != nil {
		event.Request.URL = ""
		event.Request.QueryString = ""
		event.Request.Data = ""
		event.Request.Cookies = ""
		event.Request.Headers = nil
		event.Request.Env = nil
	}
	for _, breadcrumb := range event.Breadcrumbs {
		sanitizeSentryBreadcrumb(breadcrumb, nil)
	}
	return event
}

func sanitizeSentryBreadcrumb(breadcrumb *sentry.Breadcrumb, _ *sentry.BreadcrumbHint) *sentry.Breadcrumb {
	if breadcrumb == nil || breadcrumb.Data == nil {
		return breadcrumb
	}
	for key := range breadcrumb.Data {
		normalized := strings.ToLower(key)
		if strings.Contains(normalized, "authorization") ||
			strings.Contains(normalized, "cookie") ||
			strings.Contains(normalized, "csrf") ||
			strings.Contains(normalized, "token") ||
			strings.Contains(normalized, "password") ||
			strings.Contains(normalized, "secret") ||
			strings.Contains(normalized, "body") ||
			strings.Contains(normalized, "payload") ||
			strings.Contains(normalized, "email") ||
			strings.Contains(normalized, "phone") ||
			strings.Contains(normalized, "contact") {
			delete(breadcrumb.Data, key)
		}
	}
	return breadcrumb
}
