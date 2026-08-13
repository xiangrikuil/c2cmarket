package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/middleware"
	"c2c-market/backend/internal/module/core"
)

func TestBusinessActorDefaultsToNormalAndNeverFallsBackToRestrictedCookie(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	service := core.NewServiceWithClock(func() time.Time { return now })
	handler := NewServer(service, ServerOptions{EnableDevAuth: true})
	server := &Server{app: service}
	normal := createSession(t, handler, "actor-normal", false)
	request := httptest.NewRequest(http.MethodGet, "/shared", nil)
	addCookie(request, normal.cookie)
	actor, appErr := server.requireBusinessActor(request, true, false)
	if appErr != nil || actor.Audience != "normal" || actor.UserID != normal.userID {
		t.Fatalf("normal actor mismatch actor=%+v error=%v", actor, appErr)
	}

	restrictedOnly := httptest.NewRequest(http.MethodGet, "/shared", nil)
	restrictedOnly.AddCookie(&http.Cookie{Name: middleware.RestrictedBusinessSessionCookieName, Value: "restricted-only"})
	if _, appErr := server.requireBusinessActor(restrictedOnly, true, false); appErr == nil || appErr.Code != domain.CodeSessionExpired {
		t.Fatalf("default audience fell back to restricted cookie: %v", appErr)
	}
}

func TestBusinessActorRejectsUnapprovedOrUnknownAudience(t *testing.T) {
	server := &Server{app: core.NewServiceWithClock(func() time.Time { return time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC) })}
	request := httptest.NewRequest(http.MethodGet, "/shared", nil)
	request.Header.Set(middleware.SessionAudienceHeaderName, "restricted_business")
	if _, appErr := server.requireBusinessActor(request, false, false); appErr == nil || appErr.Code != domain.CodePermissionDenied {
		t.Fatalf("restricted audience accepted by normal-only route: %v", appErr)
	}

	request = httptest.NewRequest(http.MethodGet, "/shared", nil)
	request.Header.Set(middleware.SessionAudienceHeaderName, "account_appeal")
	if _, appErr := server.requireBusinessActor(request, true, false); appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("unknown shared-route audience accepted: %v", appErr)
	}
}
