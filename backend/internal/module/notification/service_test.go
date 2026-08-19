package notification

import (
	"context"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
)

func TestCompleteDisputeActionsDowngradesOutstandingMemoryNotifications(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	dueAt := now.Add(24 * time.Hour)
	service := NewService(nil, func() time.Time { return now })
	service.Add(Notification{
		ID: "todo", UserID: "buyer", TargetType: "dispute", TargetID: "dispute-1",
		SourceEventType: "dispute.remedy_claimed", ActionRequired: true, ActionDueAt: &dueAt,
	})
	service.Add(Notification{
		ID: "other", UserID: "buyer", TargetType: "dispute", TargetID: "dispute-2",
		SourceEventType: "dispute.remedy_claimed", ActionRequired: true, ActionDueAt: &dueAt,
	})

	service.CompleteDisputeActions("dispute-1")
	items, appErr := service.List(context.Background(), "buyer", domain.PageRequest{Limit: 20})
	if appErr != nil {
		t.Fatalf("list notifications: %+v", appErr)
	}
	statusByID := make(map[string]bool, len(items.Items))
	for _, item := range items.Items {
		statusByID[item.ID] = item.ActionRequired
	}
	if statusByID["todo"] {
		t.Fatal("completed dispute notification must no longer require action")
	}
	if !statusByID["other"] {
		t.Fatal("unrelated dispute notification must remain actionable")
	}
}

func TestMemoryNotificationActionExpiresAtDeadline(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	dueAt := now.Add(time.Hour)
	service := NewService(nil, func() time.Time { return now })
	service.Add(Notification{
		ID: "todo", UserID: "buyer", TargetType: "dispute", TargetID: "dispute-1",
		SourceEventType: "dispute.remedy_claimed", ActionRequired: true, ActionDueAt: &dueAt,
	})
	now = dueAt

	items, appErr := service.List(context.Background(), "buyer", domain.PageRequest{Limit: 20})
	if appErr != nil {
		t.Fatalf("list notifications: %+v", appErr)
	}
	if len(items.Items) != 1 || items.Items[0].ActionRequired {
		t.Fatalf("deadline must remove the action requirement: %+v", items.Items)
	}
}
