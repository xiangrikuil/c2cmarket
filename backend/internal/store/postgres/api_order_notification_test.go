package postgres

import (
	"strings"
	"testing"

	"c2c-market/backend/internal/module/apiorder"
)

func TestAPIOrderNotificationMatrix(t *testing.T) {
	order := apiorder.Order{
		ID:           "11111111-1111-4111-8111-111111111111",
		BuyerUserID:  "22222222-2222-4222-8222-222222222222",
		SellerUserID: "33333333-3333-4333-8333-333333333333",
	}
	tests := []struct {
		name      string
		eventType string
		actorID   string
		wantUser  string
		wantURL   string
	}{
		{name: "payment submitted", eventType: apiorder.EventPaymentSubmitted, actorID: order.BuyerUserID, wantUser: order.SellerUserID, wantURL: "/merchant/api-orders/" + order.ID},
		{name: "payment issue", eventType: apiorder.EventPaymentIssueReported, actorID: order.SellerUserID, wantUser: order.BuyerUserID, wantURL: "/my/api-orders/" + order.ID},
		{name: "buyer cancelled", eventType: apiorder.EventCancelled, actorID: order.BuyerUserID, wantUser: order.SellerUserID, wantURL: "/merchant/api-orders/" + order.ID},
		{name: "payment confirmed", eventType: apiorder.EventPaymentConfirmed, actorID: order.SellerUserID, wantUser: order.BuyerUserID, wantURL: "/my/api-orders/" + order.ID},
		{name: "delivery submitted", eventType: apiorder.EventDeliverySubmitted, actorID: order.SellerUserID, wantUser: order.BuyerUserID, wantURL: "/my/api-orders/" + order.ID},
		{name: "completed", eventType: apiorder.EventCompleted, actorID: order.BuyerUserID, wantUser: order.SellerUserID, wantURL: "/merchant/api-orders/" + order.ID},
		{name: "timeout", eventType: apiorder.EventPaymentTimeoutCancelled, wantUser: order.BuyerUserID, wantURL: "/my/api-orders/" + order.ID},
		{name: "buyer dispute", eventType: apiorder.EventDisputeOpened, actorID: order.BuyerUserID, wantUser: order.SellerUserID, wantURL: "/merchant/api-orders/" + order.ID},
		{name: "seller dispute", eventType: apiorder.EventDisputeOpened, actorID: order.SellerUserID, wantUser: order.BuyerUserID, wantURL: "/my/api-orders/" + order.ID},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec, ok := apiOrderNotificationFor(order, tt.actorID, tt.eventType)
			if !ok {
				t.Fatalf("expected notification for %s", tt.eventType)
			}
			if spec.RecipientUserID != tt.wantUser || spec.TargetURL != tt.wantURL {
				t.Fatalf("unexpected recipient/target: %#v", spec)
			}
			joined := strings.ToLower(spec.Title + " " + spec.Body)
			for _, forbidden := range []string{"api key", "password", "token", "session", "付款摘要", "二维码"} {
				if strings.Contains(joined, forbidden) {
					t.Fatalf("notification copy contains forbidden material %q: %q", forbidden, joined)
				}
			}
		})
	}
}

func TestAPIOrderNotificationMatrixSkipsCreationAndInvalidDisputeActor(t *testing.T) {
	order := apiorder.Order{ID: "order", BuyerUserID: "buyer", SellerUserID: "seller"}
	if _, ok := apiOrderNotificationFor(order, order.BuyerUserID, apiorder.EventCreated); ok {
		t.Fatal("order creation must not duplicate the purchase-intent notification")
	}
	if _, ok := apiOrderNotificationFor(order, "other", apiorder.EventDisputeOpened); ok {
		t.Fatal("unknown dispute actor must not create a notification")
	}
}
