package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/middleware"
	"c2c-market/backend/internal/module/auth"
	core "c2c-market/backend/internal/module/core"
	"c2c-market/backend/internal/module/report"
)

type disputeContinuityRouteService struct {
	ApplicationService
	actor           auth.BusinessActor
	supplementActor auth.BusinessActor
	dispute         report.DisputeCase
}

func (s *disputeContinuityRouteService) GetRestrictedBusinessSession(_ context.Context, sessionID string) (auth.User, auth.RestrictedBusinessSession, *domain.AppError) {
	if sessionID != "restricted-dispute-session" {
		return auth.User{}, auth.RestrictedBusinessSession{}, domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session expired", "受限业务会话已失效。")
	}
	return auth.User{ID: s.actor.UserID, Username: "restricted-buyer", DisplayName: "Restricted Buyer", Status: auth.AccountStatusBanned}, auth.RestrictedBusinessSession{
		UserID:                 s.actor.UserID,
		GovernanceActionID:     s.actor.GovernanceActionID,
		GovernanceVersion:      s.actor.GovernanceVersion,
		RestrictionEffectiveAt: s.actor.RestrictionEffectiveAt,
	}, nil
}

func (s *disputeContinuityRouteService) GetRestrictedBusinessSessionWithCSRF(ctx context.Context, sessionID, csrfToken string) (auth.User, auth.RestrictedBusinessSession, *domain.AppError) {
	if csrfToken != "restricted-csrf" {
		return auth.User{}, auth.RestrictedBusinessSession{}, domain.NewError(http.StatusForbidden, domain.CodeCSRFTokenInvalid, "CSRF token invalid", "受限业务 CSRF token 无效或缺失。")
	}
	return s.GetRestrictedBusinessSession(ctx, sessionID)
}

func (s *disputeContinuityRouteService) SubmitInfoSupplementForActorWithIdempotency(_ context.Context, actor auth.BusinessActor, _ string, _ string, _ string, _ report.SupplementInput, buildCompletion report.SupplementCompletionBuilder) (core.IdempotencyCompletion, *domain.AppError) {
	s.supplementActor = actor
	return buildCompletion(report.MutationResult{Dispute: &s.dispute})
}

func (s *disputeContinuityRouteService) DisputeForActor(_ context.Context, actor auth.BusinessActor, _ string) (report.DisputeCase, *domain.AppError) {
	s.actor = actor
	return s.dispute, nil
}

func TestRestrictedDisputeSupplementCarriesDisplayIdentity(t *testing.T) {
	now := time.Date(2026, 8, 14, 14, 0, 0, 0, time.UTC)
	service := &disputeContinuityRouteService{
		ApplicationService: core.NewServiceWithClock(func() time.Time { return now }),
		actor: auth.BusinessActor{
			UserID: "10000000-0000-4000-8000-000000000031", GovernanceActionID: "20000000-0000-4000-8000-000000000031",
			GovernanceVersion: 2, RestrictionEffectiveAt: now.Add(-time.Hour),
		},
		dispute: report.DisputeCase{
			ID: "30000000-0000-4000-8000-000000000031", PrimaryUserID: "10000000-0000-4000-8000-000000000031",
			Status: report.DisputeStatusWaitingInfo, OpenInfoRequestID: "40000000-0000-4000-8000-000000000031",
			InfoRequestedFromID: "10000000-0000-4000-8000-000000000031", CreatedAt: now, UpdatedAt: now,
		},
	}
	handler := NewServer(service, ServerOptions{EnableDevAuth: true})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/me/disputes/"+service.dispute.ID+"/supplements", strings.NewReader(`{"openInfoRequestId":"40000000-0000-4000-8000-000000000031","body":"补充材料"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Idempotency-Key", "restricted-dispute-supplement")
	request.Header.Set(middleware.SessionAudienceHeaderName, auth.SessionAudienceRestrictedBusiness)
	request.Header.Set(middleware.RestrictedBusinessCSRFHeaderName, "restricted-csrf")
	request.AddCookie(&http.Cookie{Name: middleware.RestrictedBusinessSessionCookieName, Value: "restricted-dispute-session"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("restricted dispute supplement status=%d body=%s", response.Code, response.Body.String())
	}
	if service.supplementActor.Username != "restricted-buyer" || service.supplementActor.DisplayName != "Restricted Buyer" {
		t.Fatalf("restricted supplement display identity lost: %+v", service.supplementActor)
	}
}

func TestRestrictedDisputeDetailCarriesGovernanceIdentity(t *testing.T) {
	now := time.Date(2026, 8, 14, 13, 0, 0, 0, time.UTC)
	service := &disputeContinuityRouteService{
		ApplicationService: core.NewServiceWithClock(func() time.Time { return now }),
		actor: auth.BusinessActor{
			UserID:                 "10000000-0000-4000-8000-000000000021",
			GovernanceActionID:     "20000000-0000-4000-8000-000000000021",
			GovernanceVersion:      5,
			RestrictionEffectiveAt: now.Add(-time.Hour),
		},
		dispute: report.DisputeCase{
			ID:                 "30000000-0000-4000-8000-000000000021",
			TargetType:         report.TargetAPIOrder,
			TargetID:           "40000000-0000-4000-8000-000000000021",
			PrimaryUserID:      "10000000-0000-4000-8000-000000000021",
			CounterpartyUserID: "10000000-0000-4000-8000-000000000022",
			Status:             report.DisputeStatusNegotiating,
			OpenedAt:           now.Add(-2 * time.Hour),
			CreatedAt:          now.Add(-2 * time.Hour),
			UpdatedAt:          now,
			Version:            2,
		},
	}
	handler := NewServer(service, ServerOptions{EnableDevAuth: true})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me/disputes/"+service.dispute.ID, nil)
	request.Header.Set(middleware.SessionAudienceHeaderName, auth.SessionAudienceRestrictedBusiness)
	request.AddCookie(&http.Cookie{Name: middleware.RestrictedBusinessSessionCookieName, Value: "restricted-dispute-session"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("restricted dispute detail status=%d body=%s", response.Code, response.Body.String())
	}
	if service.actor.Audience != auth.SessionAudienceRestrictedBusiness || service.actor.GovernanceActionID != "20000000-0000-4000-8000-000000000021" || service.actor.GovernanceVersion != 5 || !service.actor.RestrictionEffectiveAt.Equal(now.Add(-time.Hour)) {
		t.Fatalf("restricted dispute actor context lost: %+v", service.actor)
	}
}
