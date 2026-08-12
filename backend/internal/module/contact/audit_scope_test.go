package contact

import (
	"context"
	"slices"
	"testing"
	"time"
)

func TestContactUsageScopeAndAuditEventsStayAlignedInMemory(t *testing.T) {
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	service := NewService(nil, func() time.Time { return now })
	method, appErr := service.CreateMethod(context.Background(), ContactMethodInput{
		UserID: "buyer", Type: "wechat", Label: "微信", Value: "sensitive-wechat-id",
		UsageScopes: []string{UsageScopeBuyer}, Enabled: true, RequestID: "create-request",
	})
	if appErr != nil {
		t.Fatalf("create contact method: %v", appErr)
	}
	if _, _, ok := service.VersionForOwnerAndScope(method.ID, "buyer", UsageScopeBuyer); !ok {
		t.Fatal("buyer-scoped method was not accepted for buyer usage")
	}
	if _, _, ok := service.VersionForOwnerAndScope(method.ID, "buyer", UsageScopeCarpoolOwner); ok {
		t.Fatal("buyer-scoped method was accepted for carpool owner usage")
	}

	now = now.Add(time.Minute)
	method, appErr = service.UpdateMethod(context.Background(), UpdateContactMethodInput{
		UserID: "buyer", MethodID: method.ID, Type: "wechat", Label: "备用微信", Value: "next-sensitive-id",
		UsageScopes: []string{UsageScopeBuyer, UsageScopeDispute}, Enabled: true, RequestID: "update-request",
	})
	if appErr != nil {
		t.Fatalf("update contact method: %v", appErr)
	}
	now = now.Add(time.Minute)
	if method, appErr = service.SetDefaultMethodWithRequestID(context.Background(), "buyer", method.ID, "default-request"); appErr != nil {
		t.Fatalf("set default contact method: %v", appErr)
	}
	now = now.Add(time.Minute)
	if method, appErr = service.VerifyMethodWithRequestID(context.Background(), "buyer", method.ID, "verify-request"); appErr != nil {
		t.Fatalf("verify contact method: %v", appErr)
	}
	now = now.Add(time.Minute)
	if _, appErr = service.DeleteMethodWithRequestID(context.Background(), "buyer", method.ID, "disable-request"); appErr != nil {
		t.Fatalf("disable contact method: %v", appErr)
	}

	events := service.MethodAuditEvents()
	actions := make([]string, 0, len(events))
	for _, event := range events {
		if event.ActorUserID != "buyer" || event.RequestID == "" || len(event.ChangedFields) == 0 {
			t.Fatalf("unsafe or incomplete contact audit event: %+v", event)
		}
		actions = append(actions, event.EventType)
	}
	want := []string{
		"contact_method.created",
		"contact_method.updated",
		"contact_method.default_changed",
		"contact_method.verified",
		"contact_method.disabled",
	}
	if !slices.Equal(actions, want) {
		t.Fatalf("audit actions = %v, want %v", actions, want)
	}
}
