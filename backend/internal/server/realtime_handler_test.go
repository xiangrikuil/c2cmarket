package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	app "c2c-market/backend/internal/module/core"
	"c2c-market/backend/internal/realtime"
)

func TestRealtimeEventsRequireSession(t *testing.T) {
	server := NewServer(app.NewService())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me/events", nil)
	response := httptest.NewRecorder()

	server.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d body %s", response.Code, response.Body.String())
	}
}

func TestRealtimeEventsFlushReadyAndUserInvalidationThroughMiddleware(t *testing.T) {
	hub := realtime.NewHub()
	defer hub.Close()
	server := NewServer(app.NewService(), ServerOptions{EnableDevAuth: true, RealtimeHub: hub})
	session := createSession(t, server, "realtime-user", false)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me/events", nil).WithContext(ctx)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: session.cookie})
	response := newRealtimeResponseRecorder()
	done := make(chan struct{})
	go func() {
		server.ServeHTTP(response, request)
		close(done)
	}()

	waitForRealtimeFlush(t, response.flushed)
	if got := response.header.Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Fatalf("content type = %q", got)
	}
	if got := response.header.Get("Cache-Control"); got != "no-cache, no-transform" {
		t.Fatalf("cache control = %q", got)
	}
	readyBody := response.bodyString()
	if !strings.Contains(readyBody, "retry: 3000") || !strings.Contains(readyBody, "event: ready") || !strings.Contains(readyBody, realtimeClientPayload) {
		t.Fatalf("missing ready event: %q", readyBody)
	}
	if strings.Contains(readyBody, session.userID) {
		t.Fatalf("SSE payload leaked routing user ID: %q", readyBody)
	}
	if !response.hasBoundedWriteDeadline() {
		t.Fatal("SSE writes did not install a bounded write deadline")
	}

	hub.PublishUser(session.userID)
	waitForRealtimeFlush(t, response.flushed)
	if body := response.bodyString(); !strings.Contains(body, "event: invalidate") {
		t.Fatalf("missing invalidate event: %q", body)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("SSE handler did not exit after request cancellation")
	}
}

type realtimeResponseRecorder struct {
	mu        sync.Mutex
	header    http.Header
	status    int
	body      strings.Builder
	flushed   chan struct{}
	deadlines []time.Time
}

func newRealtimeResponseRecorder() *realtimeResponseRecorder {
	return &realtimeResponseRecorder{
		header:  make(http.Header),
		flushed: make(chan struct{}, 4),
	}
}

func (r *realtimeResponseRecorder) Header() http.Header {
	return r.header
}

func (r *realtimeResponseRecorder) WriteHeader(status int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.status == 0 {
		r.status = status
	}
}

func (r *realtimeResponseRecorder) Write(value []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.body.Write(value)
}

func (r *realtimeResponseRecorder) Flush() {
	select {
	case r.flushed <- struct{}{}:
	default:
	}
}

func (r *realtimeResponseRecorder) SetWriteDeadline(deadline time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deadlines = append(r.deadlines, deadline)
	return nil
}

func (r *realtimeResponseRecorder) hasBoundedWriteDeadline() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, deadline := range r.deadlines {
		if !deadline.IsZero() {
			return true
		}
	}
	return false
}

func (r *realtimeResponseRecorder) bodyString() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body.String()
}

func waitForRealtimeFlush(t *testing.T, flushed <-chan struct{}) {
	t.Helper()
	select {
	case <-flushed:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for SSE flush")
	}
}
