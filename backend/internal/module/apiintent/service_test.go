package apiintent

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

func TestCancelAndCloseIntentWithOrderReturnDedicatedConflict(t *testing.T) {
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	manager := NewManager(nil, nil, nil, nil, func() time.Time { return now })
	manager.intents["intent-1"] = Intent{
		ID:          "intent-1",
		BuyerUserID: "buyer-1",
		OwnerUserID: "seller-1",
		Status:      StatusOpen,
		CreatedAt:   now,
		UpdatedAt:   now,
		Version:     1,
	}
	manager.SetOrderExistenceChecker(staticOrderExistenceChecker(true))

	_, appErr := manager.CancelWithIdempotency(context.Background(), "buyer-1", "api-intent-cancel", "cancel-1", "hash-cancel", ActionInput{
		IntentID:        "intent-1",
		Reason:          "已进入订单流程。",
		ExpectedVersion: 1,
		RequestID:       "cancel-1",
	}, testAPIIntentCompletion)
	if appErr == nil || appErr.Status != http.StatusConflict || appErr.Code != domain.CodeAPIPurchaseIntentHasOrder {
		t.Fatalf("expected cancel dedicated conflict, got %v", appErr)
	}

	_, appErr = manager.CloseWithIdempotency(context.Background(), "seller-1", "api-intent-close", "close-1", "hash-close", ActionInput{
		IntentID:        "intent-1",
		Reason:          "已进入订单流程。",
		ExpectedVersion: 1,
		RequestID:       "close-1",
	}, testAPIIntentCompletion)
	if appErr == nil || appErr.Status != http.StatusConflict || appErr.Code != domain.CodeAPIPurchaseIntentHasOrder {
		t.Fatalf("expected close dedicated conflict, got %v", appErr)
	}
}

type staticOrderExistenceChecker bool

func (s staticOrderExistenceChecker) HasOrderForIntent(string) bool {
	return bool(s)
}

func testAPIIntentCompletion(intent Intent) (idempotency.Completion, *domain.AppError) {
	body, err := json.Marshal(intent)
	if err != nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	return idempotency.Completion{
		Status:       http.StatusOK,
		ContentType:  "application/json",
		Body:         body,
		ResourceType: "api_purchase_intent",
		ResourceID:   intent.ID,
	}, nil
}
