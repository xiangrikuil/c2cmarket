package server

import (
	"testing"

	"c2c-market/backend/internal/module/notification"
)

func TestNotificationTargetURLPrefersPersistedTargetURL(t *testing.T) {
	item := notification.Notification{
		TargetType: "carpool_application",
		TargetID:   "application-123",
		TargetURL:  "/merchant/carpool-applications/application-123",
	}

	if got := notificationTargetURL(item); got != item.TargetURL {
		t.Fatalf("expected persisted target URL %q, got %q", item.TargetURL, got)
	}
}

func TestNotificationTargetURLFallsBackForLegacyCarpoolApplicationRows(t *testing.T) {
	item := notification.Notification{
		TargetType: "carpool_application",
		TargetID:   "application-123",
	}

	if got := notificationTargetURL(item); got != "/my/rides/application-123" {
		t.Fatalf("expected legacy buyer carpool route, got %q", got)
	}
}

func TestNotificationTargetURLAndCategoryForAPIOrder(t *testing.T) {
	item := notification.Notification{
		TargetType: "api_order",
		TargetID:   "order-123",
	}

	if got := notificationTargetURL(item); got != "/my/api-orders/order-123" {
		t.Fatalf("expected buyer API order fallback route, got %q", got)
	}
	if got := notificationCategory(item); got != "API 订单" {
		t.Fatalf("expected API order category, got %q", got)
	}
}

func TestDisputeNotificationCategoryUsesCurrentActionRequirement(t *testing.T) {
	item := notification.Notification{
		TargetType:      "dispute",
		TargetID:        "dispute-123",
		SourceEventType: "dispute.remedy_claimed",
		ActionRequired:  true,
	}

	if got := notificationCategory(item); got != "交易待办" {
		t.Fatalf("expected pending dispute action category, got %q", got)
	}
	item.ActionRequired = false
	if got := notificationCategory(item); got != "交易通知" {
		t.Fatalf("expected completed dispute action to become a transaction notice, got %q", got)
	}
	if got := notificationTargetURL(item); got != "/my/disputes/dispute-123" {
		t.Fatalf("expected dispute detail route, got %q", got)
	}
}

func TestNotificationTargetURLFallsBackToPerspectiveListForAPIPurchaseIntent(t *testing.T) {
	tests := []struct {
		name            string
		sourceEventType string
		want            string
	}{
		{name: "seller receives creation", sourceEventType: "api_purchase_intent.created", want: "/merchant/api-orders"},
		{name: "seller receives buyer cancellation", sourceEventType: "api_purchase_intent.buyer_cancelled", want: "/merchant/api-orders"},
		{name: "buyer receives contact update", sourceEventType: "api_purchase_intent.contacted", want: "/my/api-orders"},
		{name: "buyer receives owner closure", sourceEventType: "api_purchase_intent.owner_closed", want: "/my/api-orders"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := notification.Notification{
				TargetType:      "api_purchase_intent",
				TargetID:        "intent-123",
				SourceEventType: tt.sourceEventType,
			}
			if got := notificationTargetURL(item); got != tt.want {
				t.Fatalf("expected API order list fallback route %q, got %q", tt.want, got)
			}
		})
	}
}
