package apiorder

import (
	"context"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
)

type afterSalesDisputeRecorder struct {
	input DisputeCaseInput
}

func (r *afterSalesDisputeRecorder) RegisterAPIOrderDispute(_ context.Context, input DisputeCaseInput) (DisputeProjection, *domain.AppError) {
	r.input = input
	dueAt := input.Now.Add(48 * time.Hour)
	return DisputeProjection{CaseID: "dispute-after-sales", Status: DisputeStatusOpen, NextActor: "respondent", NextUserID: input.SellerUserID, DueAt: &dueAt}, nil
}

func TestCompletedDisputeUsesAuthoritativeMutationTimeAndReturnsUpdatedProjection(t *testing.T) {
	validityExpiresAt := time.Date(2026, 8, 10, 8, 0, 0, 0, time.UTC)
	current := validityExpiresAt.Add(24*time.Hour - time.Second)
	clockCalls := 0
	recorder := &afterSalesDisputeRecorder{}
	service := NewService(nil, nil, nil, recorder, nil, func() time.Time {
		value := current.Add(time.Duration(clockCalls) * 2 * time.Second)
		clockCalls++
		return value
	})
	clockCalls = 0
	service.orders["completed-order"] = Order{
		ID:                     "completed-order",
		BuyerUserID:            "buyer-1",
		SellerUserID:           "seller-1",
		Status:                 StatusCompleted,
		DisputeStatus:          DisputeStatusNone,
		QuotaExpiresAtSnapshot: &validityExpiresAt,
		Version:                3,
	}

	order, appErr := service.updateInMemory(context.Background(), ActionInput{
		OrderID:             "completed-order",
		ActorUserID:         "buyer-1",
		IssueCode:           DisputeIssueServiceUnavailable,
		RequestedResolution: DisputeResolutionFullRefund,
		IssueOccurredAt:     validityExpiresAt.Add(-time.Minute).Format(time.RFC3339),
		Reason:              "服务在有效期结束前无法使用。",
		ExpectedVersion:     3,
		RequestID:           "completed-after-sales",
	}, "open_dispute")
	if appErr != nil {
		t.Fatalf("open completed-order dispute: %v", appErr)
	}
	if !recorder.input.Now.Equal(current) {
		t.Fatalf("expected validation and dispute record to share mutation time, calls=%d inputNow=%v", clockCalls, recorder.input.Now)
	}
	if recorder.input.IssueOccurredAt == nil || !recorder.input.IssueOccurredAt.Equal(validityExpiresAt.Add(-time.Minute)) {
		t.Fatalf("unexpected frozen issue occurrence: %v", recorder.input.IssueOccurredAt)
	}
	if order.CanOpenDispute || order.DisputeEligibilityReason != DisputeEligibilityDisputeExists {
		t.Fatalf("opened dispute returned stale eligibility: %+v", order)
	}
}
