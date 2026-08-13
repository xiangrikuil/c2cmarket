package apiorder

import (
	"testing"
	"time"
)

func TestCompletedOrderCanReportDuringGracePeriod(t *testing.T) {
	validityExpiresAt := time.Date(2026, 8, 10, 8, 0, 0, 0, time.UTC)
	order := Order{
		Status:                 StatusCompleted,
		DisputeStatus:          DisputeStatusNone,
		QuotaExpiresAtSnapshot: &validityExpiresAt,
	}

	projection := WithAfterSalesProjection(order, validityExpiresAt.Add(23*time.Hour))
	if !projection.CanOpenDispute || projection.AfterSalesExpiresAt == nil || !projection.AfterSalesExpiresAt.Equal(validityExpiresAt.Add(24*time.Hour)) {
		t.Fatalf("expected completed order to remain reportable during grace period: %+v", projection)
	}

	occurredAt, appErr := ValidateDisputeOccurrence(order, validityExpiresAt.Add(-time.Minute).Format(time.RFC3339), validityExpiresAt.Add(23*time.Hour))
	if appErr != nil || occurredAt == nil {
		t.Fatalf("expected in-validity occurrence to be accepted: occurredAt=%v err=%v", occurredAt, appErr)
	}
}

func TestCompletedOrderAfterSalesValidationRejectsInvalidOccurrence(t *testing.T) {
	validityExpiresAt := time.Date(2026, 8, 10, 8, 0, 0, 0, time.UTC)
	order := Order{
		Status:                 StatusCompleted,
		DisputeStatus:          DisputeStatusNone,
		QuotaExpiresAtSnapshot: &validityExpiresAt,
	}
	now := validityExpiresAt.Add(23 * time.Hour)

	if _, appErr := ValidateDisputeOccurrence(order, "", now); appErr == nil || len(appErr.FieldErrors) == 0 || appErr.FieldErrors[0].Field != "issueOccurredAt" {
		t.Fatalf("expected completed order to require occurrence time: %+v", appErr)
	}
	if _, appErr := ValidateDisputeOccurrence(order, validityExpiresAt.Add(time.Second).Format(time.RFC3339), now); appErr == nil || len(appErr.FieldErrors) == 0 || appErr.FieldErrors[0].Code != "after_validity" {
		t.Fatalf("expected occurrence after validity to be rejected: %+v", appErr)
	}
	if _, appErr := ValidateDisputeOccurrence(order, validityExpiresAt.Format(time.RFC3339), validityExpiresAt.Add(24*time.Hour)); appErr == nil {
		t.Fatal("expected grace-period boundary to be expired")
	}
}

func TestValidityExpiresAtFallsBackToPricingSnapshot(t *testing.T) {
	want := time.Date(2026, 8, 12, 4, 30, 0, 0, time.UTC)
	order := Order{PricingSnapshot: `{"serviceValidityExpiresAt":"2026-08-12T04:30:00Z"}`}
	got := ValidityExpiresAt(order)
	if got == nil || !got.Equal(want) {
		t.Fatalf("unexpected pricing validity: got=%v want=%v", got, want)
	}
}

func TestOrderDeadlineAndLatePaymentProjectionBoundaries(t *testing.T) {
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	merchantDue := now.Add(time.Minute)
	deliveryDue := now.Add(2 * time.Minute)
	cancelledAt := now.Add(-24 * time.Hour)

	merchant := WithAfterSalesProjection(Order{Status: StatusPaymentSubmitted, MerchantConfirmDueAt: &merchantDue}, merchantDue)
	if !merchant.MerchantConfirmOverdue {
		t.Fatal("merchant confirmation must become overdue exactly at the deadline")
	}
	delivery := WithAfterSalesProjection(Order{Status: StatusPaidConfirmed, DeliveryDueAt: &deliveryDue}, deliveryDue)
	if !delivery.DeliveryOverdue {
		t.Fatal("delivery must become overdue exactly at the deadline")
	}
	late := WithAfterSalesProjection(Order{Status: StatusCancelled, CancelReason: "payment_timeout", CancelledAt: &cancelledAt}, now.Add(-time.Nanosecond))
	if !late.CanReportLatePayment {
		t.Fatal("late payment must remain reportable immediately before 24 hours")
	}
	expired := WithAfterSalesProjection(Order{Status: StatusCancelled, CancelReason: "payment_timeout", CancelledAt: &cancelledAt}, now)
	if expired.CanReportLatePayment {
		t.Fatal("late payment reporting must close exactly at 24 hours")
	}
}
