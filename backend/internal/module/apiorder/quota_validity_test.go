package apiorder

import (
	"context"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
)

func TestHasMinimumDeliveryValidityBoundary(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name      string
		expiresAt *time.Time
		want      bool
	}{
		{name: "unknown validity is not rejected", want: true},
		{name: "exactly sixty minutes remains", expiresAt: validityTimePointer(now.Add(time.Hour)), want: true},
		{name: "less than sixty minutes remains", expiresAt: validityTimePointer(now.Add(time.Hour - time.Nanosecond)), want: false},
		{name: "already expired", expiresAt: validityTimePointer(now.Add(-time.Minute)), want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			order := Order{QuotaExpiresAtSnapshot: test.expiresAt}
			if got := HasMinimumDeliveryValidity(order, now); got != test.want {
				t.Fatalf("HasMinimumDeliveryValidity() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestValidateDisputeResolutionForOrderRejectsSecondDelivery(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	order := Order{Amount: "20.00", DeliverySubmittedAt: &now}

	if appErr := ValidateDisputeResolutionForOrder(order, DisputeResolutionContinueFulfillment, ""); appErr == nil {
		t.Fatal("expected continue fulfillment to be rejected after the first delivery")
	}
	if appErr := ValidateDisputeResolutionForOrder(order, DisputeResolutionFullRefund, ""); appErr != nil {
		t.Fatalf("full refund remains available after delivery: %v", appErr)
	}
}

func TestPreimportedDeliveryValidityIsCheckedBeforePaymentConfirmation(t *testing.T) {
	now := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(59 * time.Minute)
	service := NewService(nil, nil, nil, nil, nil, func() time.Time { return now })
	service.orders["order-1"] = Order{
		ID:                     "order-1",
		PurchaseKind:           PurchaseKindLimitedQuotaOffer,
		SellerUserID:           "seller-1",
		Status:                 StatusPaymentSubmitted,
		DisputeStatus:          DisputeStatusNone,
		QuotaDeliveryMode:      QuotaDeliveryModePreimported,
		QuotaExpiresAtSnapshot: &expiresAt,
		Version:                3,
	}

	order, appErr := service.updateInMemory(context.Background(), ActionInput{
		OrderID: "order-1", ActorUserID: "seller-1", RequestID: "confirm-payment-validity",
	}, "confirm_payment")
	if appErr == nil || appErr.Code != domain.CodeAPIQuotaValidityInsufficient {
		t.Fatalf("expected quota validity error, got %#v", appErr)
	}
	if order.Status != StatusPaymentSubmitted || order.PaidConfirmedAt != nil || order.DeliverySubmittedAt != nil {
		t.Fatalf("payment confirmation must not consume or deliver an invalid quota: %#v", order)
	}
	if order.QuotaValidityIssueAt == nil || order.QuotaValidityIssueReason != QuotaValidityIssueDelivery || order.Version != 4 {
		t.Fatalf("validity issue fact was not persisted: %#v", order)
	}
}

func validityTimePointer(value time.Time) *time.Time {
	return &value
}
