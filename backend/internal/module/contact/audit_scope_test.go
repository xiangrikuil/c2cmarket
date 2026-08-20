package contact

import (
	"context"
	"slices"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

func TestTransactionContactAndAuditEventsStayAlignedInMemory(t *testing.T) {
	now := time.Date(2026, 8, 12, 12, 0, 0, 0, time.UTC)
	service := NewService(nil, func() time.Time { return now })
	method, appErr := service.CreateMethod(context.Background(), ContactMethodInput{
		UserID: "buyer", Type: "email", Label: "交易邮箱", Value: "buyer@example.com",
		Enabled: true, RequestID: "create-request",
	})
	if appErr != nil {
		t.Fatalf("create contact method: %v", appErr)
	}
	if _, _, ok := service.TransactionVersionForOwner(method.ID, "buyer"); ok {
		t.Fatal("unverified email was accepted as a transaction contact")
	}

	now = now.Add(time.Minute)
	method, appErr = service.UpdateMethod(context.Background(), UpdateContactMethodInput{
		UserID: "buyer", MethodID: method.ID, Type: "email", Label: "备用交易邮箱", Value: "next@example.com",
		Enabled: true, RequestID: "update-request",
	})
	if appErr != nil {
		t.Fatalf("update contact method: %v", appErr)
	}
	now = now.Add(time.Minute)
	if method, appErr = service.SetDefaultMethodWithRequestID(context.Background(), "buyer", method.ID, "default-request"); appErr != nil {
		t.Fatalf("set default contact method: %v", appErr)
	}
	now = now.Add(time.Minute)
	challenge, appErr := service.StartEmailVerification(context.Background(), "buyer", method.ID)
	if appErr != nil {
		t.Fatalf("start contact email verification: %v", appErr)
	}
	method, _, changed, appErr := service.ConfirmEmailVerificationWithIdempotency(
		context.Background(), "buyer", "contact-email-confirm", "contact-email-confirm-key", "contact-email-confirm-hash",
		method.ID, challenge.DevCode, "verify-request",
		func(method ContactMethod) (idempotency.Completion, *domain.AppError) {
			return idempotency.Completion{Status: 200, ContentType: "application/json", Body: []byte(`{}`), ResourceType: "contact_method", ResourceID: method.ID}, nil
		},
	)
	if appErr != nil || !changed {
		t.Fatalf("verify contact method: changed=%t error=%v", changed, appErr)
	}
	if _, _, ok := service.TransactionVersionForOwner(method.ID, "buyer"); !ok {
		t.Fatal("verified email was not accepted as a transaction contact")
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
