package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/middleware"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/carpool"
	core "c2c-market/backend/internal/module/core"
)

type carpoolContinuityRouteService struct {
	ApplicationService
	actor auth.BusinessActor
	role  string
}

func (s *carpoolContinuityRouteService) GetRestrictedBusinessSession(_ context.Context, sessionID string) (auth.User, auth.RestrictedBusinessSession, *domain.AppError) {
	if sessionID != "restricted-carpool-session" {
		return auth.User{}, auth.RestrictedBusinessSession{}, domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session expired", "受限业务会话已失效。")
	}
	return auth.User{ID: s.actor.UserID, Status: auth.AccountStatusSuspended}, auth.RestrictedBusinessSession{
		UserID:                 s.actor.UserID,
		GovernanceActionID:     s.actor.GovernanceActionID,
		GovernanceVersion:      s.actor.GovernanceVersion,
		RestrictionEffectiveAt: s.actor.RestrictionEffectiveAt,
	}, nil
}

func (s *carpoolContinuityRouteService) CarpoolApplicationsForActor(_ context.Context, actor auth.BusinessActor, participantRole string) ([]carpool.Application, *domain.AppError) {
	s.actor = actor
	s.role = participantRole
	return []carpool.Application{}, nil
}

func (s *carpoolContinuityRouteService) CarpoolMembershipsForActor(_ context.Context, _ auth.BusinessActor, _ string) ([]carpool.Membership, *domain.AppError) {
	return []carpool.Membership{}, nil
}

func TestRestrictedCarpoolOwnerListCarriesGovernanceIdentity(t *testing.T) {
	now := time.Date(2026, 8, 14, 11, 0, 0, 0, time.UTC)
	service := &carpoolContinuityRouteService{
		ApplicationService: core.NewServiceWithClock(func() time.Time { return now }),
		actor: auth.BusinessActor{
			UserID:                 "10000000-0000-4000-8000-000000000011",
			GovernanceActionID:     "20000000-0000-4000-8000-000000000011",
			GovernanceVersion:      4,
			RestrictionEffectiveAt: now.Add(-time.Hour),
		},
	}
	handler := NewServer(service, ServerOptions{EnableDevAuth: true})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/owner/carpool-applications", nil)
	request.Header.Set(middleware.SessionAudienceHeaderName, auth.SessionAudienceRestrictedBusiness)
	request.AddCookie(&http.Cookie{Name: middleware.RestrictedBusinessSessionCookieName, Value: "restricted-carpool-session"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("restricted owner list status=%d body=%s", response.Code, response.Body.String())
	}
	if service.actor.Audience != auth.SessionAudienceRestrictedBusiness || service.actor.GovernanceActionID != "20000000-0000-4000-8000-000000000011" || service.actor.GovernanceVersion != 4 || !service.actor.RestrictionEffectiveAt.Equal(now.Add(-time.Hour)) || service.role != carpool.JoinActorOwner {
		t.Fatalf("restricted carpool actor context lost: actor=%+v role=%s", service.actor, service.role)
	}
}

func TestRestrictedCarpoolSessionCannotConfirmJoin(t *testing.T) {
	service := core.NewServiceWithClock(func() time.Time { return time.Date(2026, 8, 14, 11, 0, 0, 0, time.UTC) })
	handler := NewServer(service, ServerOptions{EnableDevAuth: true})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/me/carpool-applications/30000000-0000-4000-8000-000000000011/confirm-join", nil)
	request.Header.Set(middleware.SessionAudienceHeaderName, auth.SessionAudienceRestrictedBusiness)
	request.AddCookie(&http.Cookie{Name: middleware.RestrictedBusinessSessionCookieName, Value: "restricted-only"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("restricted session reached confirm-join status=%d body=%s", response.Code, response.Body.String())
	}
}
