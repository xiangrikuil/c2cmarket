package apiorder

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apiintent"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"
)

func TestSubmitDeliveryRejectsCredentialShapedContentAndAllowsSafetyCopy(t *testing.T) {
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	service := NewService(nil, nil, nil, nil, nil, func() time.Time { return now })
	order := Order{
		ID:                           "order-1",
		APIPurchaseIntentID:          "intent-1",
		BuyerUserID:                  "buyer-1",
		SellerUserID:                 "seller-1",
		Status:                       StatusPaidConfirmed,
		DisputeStatus:                DisputeStatusNone,
		PaymentWindowMinutesSnapshot: 10,
		PaymentExpiresAt:             now.Add(10 * time.Minute),
		CreatedAt:                    now,
		UpdatedAt:                    now,
		Version:                      1,
	}
	service.orders[order.ID] = order

	rejected := []string{
		"Authorization: Bearer sk-test",
		"X-API-Key: abcdef",
		"apiKey: sk-test",
		"OPENAI_API_KEY=sk-proj-test",
		"ANTHROPIC_API_KEY=sk-ant-test",
		"vless://example",
		"clash://install-config",
		"hysteria://example",
		"hy2://example",
		"tuic://example",
		"sub://example",
		"ssr://example",
		"socks5://user:pass@example.com:1080",
		"https://example.com/api/v1/client/subscribe?token=abc",
		"https://example.com/sub?target=clash&url=xxx",
		"https://example.com/sub?url=https%3A%2F%2Fvendor.example%2Fapi%2Fv1%2Fclient%2Fsubscribe%3Ftoken%3Dabc123",
		"[订阅链接](https://example.com/sub?target=clash&url=xxx)",
		`{"delivery":"https://example.com/api/v1/client/subscribe?token=abc"}`,
		"abc.def.ghi",
		"abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJ",
	}
	for i, note := range rejected {
		_, appErr := service.SubmitDeliveryWithIdempotency(context.Background(), "seller-1", "api-order-submit-delivery", "reject-"+string(rune('a'+i)), note, ActionInput{
			OrderID:         order.ID,
			DeliveryNote:    note,
			ExpectedVersion: 1,
			RequestID:       "reject",
		}, testAPIOrderCompletion)
		if appErr == nil || appErr.Code != domain.CodeSecretContentDetected {
			t.Fatalf("expected %q to be rejected as secret content, got %v", note, appErr)
		}
	}

	allowed := []string{
		"请通过已披露联系方式继续沟通。",
		"请勿填写 token。",
		"站外确认 cookie 不在平台保存。",
		"不要在这里填写 API key。",
		"平台不会保存 API key、token 或订阅链接。",
	}
	for i, note := range allowed {
		working := order
		working.ID = "allowed-" + string(rune('a'+i))
		service.orders[working.ID] = working
		completion, appErr := service.SubmitDeliveryWithIdempotency(context.Background(), "seller-1", "api-order-submit-delivery", "allow-"+string(rune('a'+i)), note, ActionInput{
			OrderID:         working.ID,
			DeliveryNote:    note,
			ExpectedVersion: 1,
			RequestID:       "allow",
		}, testAPIOrderCompletion)
		if appErr != nil {
			t.Fatalf("expected %q to be allowed, got %v", note, appErr)
		}
		if completion.Status != http.StatusOK {
			t.Fatalf("unexpected completion for %q: %+v", note, completion)
		}
	}
}

func TestCreateOrderForSameIntentReturnsDedicatedConflict(t *testing.T) {
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	intents := &testIntentResolver{intent: apiintent.Intent{
		ID:                  "intent-1",
		APIServiceID:        "service-1",
		BuyerUserID:         "buyer-1",
		OwnerUserID:         "seller-1",
		Status:              apiintent.StatusOpen,
		RequestedCNYAmount:  "16.00",
		SelectedAccessMode:  "buyer_dedicated_sub_key",
		BillingModeSnapshot: apimarket.ServiceBillingModeMetered,
	}}
	services := &testPublicServiceResolver{service: testOrderableService(now)}
	service := NewService(nil, intents, services, nil, nil, func() time.Time { return now })

	_, appErr := service.CreateWithIdempotency(context.Background(), "buyer-1", "api-order-create", "create-1", "hash-1", CreateInput{
		IntentID:      "intent-1",
		PaymentMethod: apimarket.PaymentMethodWechat,
		RequestID:     "create-1",
	}, testAPIOrderCompletion)
	if appErr != nil {
		t.Fatalf("first create order: %v", appErr)
	}
	_, appErr = service.CreateWithIdempotency(context.Background(), "buyer-1", "api-order-create", "create-2", "hash-2", CreateInput{
		IntentID:      "intent-1",
		PaymentMethod: apimarket.PaymentMethodWechat,
		RequestID:     "create-2",
	}, testAPIOrderCompletion)
	if appErr == nil || appErr.Status != http.StatusConflict || appErr.Code != domain.CodeAPIPurchaseIntentHasOrder {
		t.Fatalf("expected dedicated order conflict, got %v", appErr)
	}
}

type testIntentResolver struct {
	intent apiintent.Intent
}

func (r *testIntentResolver) BuyerIntent(_ context.Context, user auth.User, intentID, _ string) (apiintent.Intent, *domain.AppError) {
	if r.intent.ID != intentID || r.intent.BuyerUserID != user.ID {
		return apiintent.Intent{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API purchase intent not found", "购买意向不存在。")
	}
	return r.intent, nil
}

type testPublicServiceResolver struct {
	service apimarket.Service
}

func (r *testPublicServiceResolver) PublicService(context.Context, string) (apimarket.Service, *domain.AppError) {
	return r.service, nil
}

func testOrderableService(now time.Time) apimarket.Service {
	quotaExpiresAt := now.Add(30 * 24 * time.Hour)
	return apimarket.Service{
		ID:                         "service-1",
		OwnerUserID:                "seller-1",
		OwnerContactMethodID:       "owner-contact-1",
		Title:                      "测试 API 服务",
		DistributionSystem:         apimarket.ServiceDistributionSub2API,
		BillingMode:                apimarket.ServiceBillingModeMetered,
		DeclaredCNYPerUSDAllowance: "0.8000",
		QuotaExpiresAt:             &quotaExpiresAt,
		MinimumIntentCNY:           "10.00",
		AcceptingOrders:            true,
		PaymentWindowMinutes:       10,
		ReviewStatus:               apimarket.ServiceReviewStatusApproved,
		PublicationStatus:          apimarket.ServicePublicationStatusOnline,
		ModerationStatus:           apimarket.ServiceModerationStatusClear,
		PaymentOptions: []apimarket.PaymentOption{{
			ID:                  "payment-1",
			PaymentMethod:       apimarket.PaymentMethodWechat,
			Enabled:             true,
			PaymentInstructions: "站外确认付款说明。",
			CreatedAt:           now,
			UpdatedAt:           now,
			Version:             1,
		}},
		CreatedAt: now,
		UpdatedAt: now,
		Version:   1,
	}
}

func testAPIOrderCompletion(order Order) (idempotency.Completion, *domain.AppError) {
	body, err := json.Marshal(order)
	if err != nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	return idempotency.Completion{
		Status:       http.StatusOK,
		ContentType:  "application/json",
		Body:         body,
		ResourceType: "api_order",
		ResourceID:   order.ID,
	}, nil
}
