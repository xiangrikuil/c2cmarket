package postgres

import (
	"testing"

	"c2c-market/backend/internal/module/apiintent"
)

func TestAPIPurchaseIntentNotificationTargetURLUsesReceiverPerspective(t *testing.T) {
	intent := apiintent.Intent{
		ID:          "intent-123",
		BuyerUserID: "buyer-123",
		OwnerUserID: "owner-123",
	}

	if got := apiPurchaseIntentNotificationTargetURL(intent, intent.OwnerUserID); got != "/merchant/api-orders/intent-123" {
		t.Fatalf("expected merchant API intent route, got %q", got)
	}
	if got := apiPurchaseIntentNotificationTargetURL(intent, intent.BuyerUserID); got != "/my/api-orders/intent-123" {
		t.Fatalf("expected buyer API intent route, got %q", got)
	}
}
