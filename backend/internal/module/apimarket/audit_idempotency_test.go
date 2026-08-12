package apimarket

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"
)

func TestProbeBindingIdempotentReplayWritesOneSafeMemoryAuditEvent(t *testing.T) {
	now := time.Date(2026, 8, 13, 9, 0, 0, 0, time.UTC)
	manager := NewManager(nil, nil, nil, func() time.Time { return now })
	owner := auth.User{ID: "owner-1"}
	manager.services["service-1"] = Service{
		ID: "service-1", OwnerUserID: owner.ID, ProbeConnectionID: "probe-old",
		ProbeReady: true, Version: 1, CreatedAt: now, UpdatedAt: now,
	}

	builderCalls := 0
	buildCompletion := func(service Service) (idempotency.Completion, *domain.AppError) {
		builderCalls++
		return idempotency.Completion{
			Status: http.StatusOK, ContentType: "application/json",
			Body:          []byte(fmt.Sprintf(`{"version":%d,"ownerOnly":"仅在响应中可见"}`, service.Version)),
			SkipBodyCache: true, ResourceType: "api_service", ResourceID: service.ID,
		}, nil
	}
	input := UpdateProbeConnectionInput{
		ServiceID: "service-1", ProbeConnectionID: "probe-new", ExpectedVersion: 1,
		RequestID: "request-probe-rebind",
	}
	first, appErr := manager.UpdateProbeConnectionWithIdempotency(context.Background(), owner, "probe-binding", "same-key", "same-hash", input, buildCompletion)
	if appErr != nil {
		t.Fatalf("update probe binding: %v", appErr)
	}
	replay, appErr := manager.UpdateProbeConnectionWithIdempotency(context.Background(), owner, "probe-binding", "same-key", "same-hash", input, buildCompletion)
	if appErr != nil {
		t.Fatalf("replay probe binding: %v", appErr)
	}
	if string(first.Body) != string(replay.Body) || builderCalls != 2 {
		t.Fatalf("replay must rebuild the no-store owner response: first=%s replay=%s builderCalls=%d", first.Body, replay.Body, builderCalls)
	}

	events := manager.ServiceAuditEvents()
	if len(events) != 1 {
		t.Fatalf("idempotent mutation wrote %d audit events, want 1", len(events))
	}
	event := events[0]
	if event.EventType != "api_service.probe_binding_changed" || event.ActorUserID != owner.ID || event.ActorKind != "user" ||
		event.RequestID != input.RequestID || event.AggregateID != input.ServiceID || event.AggregateVersion != 2 ||
		!reflect.DeepEqual(event.ChangedFields, []string{"probe_connection"}) {
		t.Fatalf("unexpected safe audit event: %+v", event)
	}
	events[0].ChangedFields[0] = "tampered"
	if manager.ServiceAuditEvents()[0].ChangedFields[0] != "probe_connection" {
		t.Fatal("audit accessor exposed mutable internal metadata")
	}

	noOpInput := UpdateProbeConnectionInput{
		ServiceID: "service-1", ProbeConnectionID: "probe-new", ExpectedVersion: 2,
		RequestID: "request-probe-no-op",
	}
	if _, appErr := manager.UpdateProbeConnectionWithIdempotency(context.Background(), owner, "probe-binding", "no-op-key", "no-op-hash", noOpInput, buildCompletion); appErr != nil {
		t.Fatalf("same probe binding should be a successful no-op: %v", appErr)
	}
	if got := len(manager.ServiceAuditEvents()); got != 1 {
		t.Fatalf("no-op wrote an audit event; got %d total events", got)
	}
}

func TestOrderSettingsMemoryAuditStoresFieldNamesNotPaymentValues(t *testing.T) {
	now := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	manager := NewManager(nil, nil, nil, func() time.Time { return now })
	owner := auth.User{ID: "owner-2"}
	manager.services["service-2"] = Service{
		ID: "service-2", OwnerUserID: owner.ID, Version: 1, PaymentWindowMinutes: 10,
		PaymentOptions: []PaymentOption{{
			ID: "payment-1", APIServiceID: "service-2", PaymentMethod: PaymentMethodWechat,
			PaymentInstructions: "请付款后备注订单号", Version: 1, CreatedAt: now, UpdatedAt: now,
		}},
		CreatedAt: now, UpdatedAt: now,
	}

	unchanged := UpdateOrderSettingsInput{
		ServiceID: "service-2", PaymentWindowMinutes: 10, ExpectedVersion: 1,
		PaymentOptions: []PaymentOptionInput{{PaymentMethod: PaymentMethodWechat, PaymentInstructions: "请付款后备注订单号"}},
		RequestID:      "request-settings-no-op",
	}
	if _, appErr := manager.UpdateOrderSettings(context.Background(), owner, unchanged); appErr != nil {
		t.Fatalf("unchanged order settings: %v", appErr)
	}
	if got := len(manager.ServiceAuditEvents()); got != 0 {
		t.Fatalf("unchanged settings wrote %d audit events", got)
	}

	changed := unchanged
	changed.PaymentOptions = []PaymentOptionInput{{PaymentMethod: PaymentMethodWechat, PaymentInstructions: "请使用新的订单备注"}}
	changed.RequestID = "request-settings-changed"
	if _, appErr := manager.UpdateOrderSettings(context.Background(), owner, changed); appErr != nil {
		t.Fatalf("change order settings: %v", appErr)
	}
	events := manager.ServiceAuditEvents()
	if len(events) != 1 || events[0].EventType != "api_service.order_settings_changed" ||
		!reflect.DeepEqual(events[0].ChangedFields, []string{"order_settings"}) {
		t.Fatalf("unexpected settings audit event: %+v", events)
	}
	if strings.Contains(fmt.Sprint(events[0]), "请使用新的订单备注") {
		t.Fatal("audit event leaked payment instructions")
	}
}
