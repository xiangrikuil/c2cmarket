package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	core "c2c-market/backend/internal/module/core"
	"c2c-market/backend/internal/module/operationaudit"
)

type operationAuditRouteService struct {
	ApplicationService
	filter operationaudit.Filter
	page   domain.Page[operationaudit.Entry]
}

func (s *operationAuditRouteService) AdminOperationAuditLogs(_ context.Context, _ auth.User, filter operationaudit.Filter) (domain.Page[operationaudit.Entry], *domain.AppError) {
	s.filter = filter
	return s.page, nil
}

func TestAdminOperationAuditHandlerUsesCanonicalContract(t *testing.T) {
	base := core.NewServiceWithClock(func() time.Time {
		return time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	})
	eventID := "30000000-0000-4000-8000-000000000001"
	next := "opaque-composite-cursor"
	service := &operationAuditRouteService{
		ApplicationService: base,
		page: domain.Page[operationaudit.Entry]{
			Items: []operationaudit.Entry{{
				ID:            operationaudit.SourceAPIOrder + ":" + eventID,
				SourceEventID: eventID,
				SourceKind:    operationaudit.SourceAPIOrder,
				Domain:        operationaudit.DomainAPIOrder,
				ActorKind:     operationaudit.ActorSystem,
				Action:        "api_order.auto_completed",
				ActionLabel:   "订单自动完成",
				TargetType:    "api_order",
				TargetID:      "20000000-0000-4000-8000-000000000001",
				Outcome:       operationaudit.OutcomeStatusChanged,
				Summary:       "系统自动完成了订单",
				DetailPath:    "/admin/api-orders/20000000-0000-4000-8000-000000000001",
				RequestID:     "req-safe",
				CreatedAt:     time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC),
			}},
			NextCursor: &next,
		},
	}
	handler := NewServer(service, ServerOptions{EnableDevAuth: true})
	admin := createSession(t, handler, "operation-audit-route-admin", true)
	request := httptest.NewRequest(http.MethodGet,
		"/api/v1/admin/audit-logs?sourceKind=api_order&domain=api_order&actorKind=system&outcome=status_changed&from=2026-08-01T00:00:00Z&to=2026-08-12T23:59:59Z&limit=5", nil)
	addCookie(request, admin.cookie)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("audit route status=%d body=%s", response.Code, response.Body.String())
	}
	if service.filter.SourceKind != operationaudit.SourceAPIOrder || service.filter.ActorKind != operationaudit.ActorSystem || service.filter.Limit != 5 {
		t.Fatalf("handler did not forward filters: %+v", service.filter)
	}
	var payload struct {
		Items      []adminOperationAuditEntryDTO `json:"items"`
		NextCursor *string                       `json:"nextCursor"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Items) != 1 || payload.NextCursor == nil || *payload.NextCursor != next || payload.Items[0].ActorUserID != nil || payload.Items[0].DetailPath == nil {
		t.Fatalf("unexpected canonical payload: %+v", payload)
	}
	for _, forbidden := range []string{"reason", "beforeJson", "afterJson", "metadataJson", "note"} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Fatalf("response leaked forbidden field %q: %s", forbidden, response.Body.String())
		}
	}
}
