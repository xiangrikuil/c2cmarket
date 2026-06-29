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
