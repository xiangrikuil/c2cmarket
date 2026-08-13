package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/middleware"
	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/auth"
	core "c2c-market/backend/internal/module/core"
)

type apiOrderContinuityRouteService struct {
	ApplicationService
	actor auth.BusinessActor
	role  string
	order apiorder.Order
}

func (s *apiOrderContinuityRouteService) GetRestrictedBusinessSession(_ context.Context, sessionID string) (auth.User, auth.RestrictedBusinessSession, *domain.AppError) {
	if sessionID != "restricted-order-session" {
		return auth.User{}, auth.RestrictedBusinessSession{}, domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session expired", "受限业务会话已失效。")
	}
	return auth.User{ID: s.actor.UserID, Status: auth.AccountStatusSuspended}, auth.RestrictedBusinessSession{
		UserID:                 s.actor.UserID,
		GovernanceActionID:     s.actor.GovernanceActionID,
		GovernanceVersion:      s.actor.GovernanceVersion,
		RestrictionEffectiveAt: s.actor.RestrictionEffectiveAt,
	}, nil
}

func (s *apiOrderContinuityRouteService) APIOrderForActor(_ context.Context, actor auth.BusinessActor, _ string, participantRole string) (apiorder.Order, *domain.AppError) {
	s.actor = actor
	s.role = participantRole
	return s.order, nil
}

func TestRestrictedAPIOrderDetailCarriesGovernanceIdentity(t *testing.T) {
	now := time.Date(2026, 8, 14, 9, 0, 0, 0, time.UTC)
	service := &apiOrderContinuityRouteService{
		ApplicationService: core.NewServiceWithClock(func() time.Time { return now }),
		actor: auth.BusinessActor{
			UserID:                 "10000000-0000-4000-8000-000000000001",
			GovernanceActionID:     "20000000-0000-4000-8000-000000000001",
			GovernanceVersion:      3,
			RestrictionEffectiveAt: now.Add(-time.Hour),
		},
		order: apiorder.Order{
			ID:                   "30000000-0000-4000-8000-000000000001",
			OrderNo:              "API202608140001",
			PurchaseKind:         apiorder.PurchaseKindAPIService,
			BuyerUserID:          "10000000-0000-4000-8000-000000000001",
			SellerUserID:         "10000000-0000-4000-8000-000000000002",
			Status:               apiorder.StatusPaidConfirmed,
			DisputeStatus:        apiorder.DisputeStatusNone,
			ServiceTitleSnapshot: "受限会话订单",
			Amount:               "10.00",
			Currency:             "CNY",
			PaymentExpiresAt:     now.Add(time.Hour),
			CreatedAt:            now.Add(-2 * time.Hour),
			UpdatedAt:            now,
			Version:              2,
		},
	}
	handler := NewServer(service, ServerOptions{EnableDevAuth: true})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me/api-orders/"+service.order.ID, nil)
	request.Header.Set(middleware.SessionAudienceHeaderName, auth.SessionAudienceRestrictedBusiness)
	request.AddCookie(&http.Cookie{Name: middleware.RestrictedBusinessSessionCookieName, Value: "restricted-order-session"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("restricted detail status=%d body=%s", response.Code, response.Body.String())
	}
	if service.actor.Audience != auth.SessionAudienceRestrictedBusiness || service.actor.GovernanceActionID != "20000000-0000-4000-8000-000000000001" || service.actor.GovernanceVersion != 3 || !service.actor.RestrictionEffectiveAt.Equal(now.Add(-time.Hour)) || service.role != "buyer" {
		t.Fatalf("restricted actor context lost: actor=%+v role=%s", service.actor, service.role)
	}
}

func TestRestrictedAPIOrderSessionCannotReadPaymentInstructions(t *testing.T) {
	service := core.NewServiceWithClock(func() time.Time { return time.Date(2026, 8, 14, 9, 0, 0, 0, time.UTC) })
	handler := NewServer(service, ServerOptions{EnableDevAuth: true})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/me/api-orders/30000000-0000-4000-8000-000000000001/payment-instructions", nil)
	request.Header.Set(middleware.SessionAudienceHeaderName, auth.SessionAudienceRestrictedBusiness)
	request.AddCookie(&http.Cookie{Name: middleware.RestrictedBusinessSessionCookieName, Value: "restricted-only"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("restricted session reached payment instructions status=%d body=%s", response.Code, response.Body.String())
	}
}
