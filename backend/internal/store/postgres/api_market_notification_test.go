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

	if got := apiPurchaseIntentNotificationTargetURL(intent, intent.OwnerUserID); got != "/merchant/api-orders" {
		t.Fatalf("expected merchant API order list route, got %q", got)
	}
	if got := apiPurchaseIntentNotificationTargetURL(intent, intent.BuyerUserID); got != "/my/api-orders" {
		t.Fatalf("expected buyer API order list route, got %q", got)
	}
}
