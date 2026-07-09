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

func TestSubmitDeliveryAcceptsStructuredCredentialAndRejectsUnsafeFields(t *testing.T) {
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

	rejected := []DeliveryCredentialInput{
		{
			DeliveryKind: DeliveryKindAPIKeyEndpoint,
			APIBaseURL:   "https://example.com/api/v1/client/subscribe?token=abc",
			APIKey:       "sk-proj-test",
		},
		{
			DeliveryKind: DeliveryKindAPIKeyEndpoint,
			APIBaseURL:   "https://api.example.com/v1",
			APIKey:       "cookie=abc",
		},
		{
			DeliveryKind: DeliveryKindAPIKeyEndpoint,
			APIBaseURL:   "https://api.example.com/v1",
			APIKey:       "sk-proj-test",
			Instructions: "Authorization: Bearer sk-test",
		},
	}
	for i, credential := range rejected {
		_, appErr := service.SubmitDeliveryWithIdempotency(context.Background(), "seller-1", "api-order-submit-delivery", "reject-"+string(rune('a'+i)), "hash-"+string(rune('a'+i)), ActionInput{
			OrderID:            order.ID,
			DeliveryCredential: credential,
			ExpectedVersion:    1,
			RequestID:          "reject",
		}, testAPIOrderCompletion)
		if appErr == nil || appErr.Code != domain.CodeSecretContentDetected {
			t.Fatalf("expected credential to be rejected as secret content, got %v", appErr)
		}
	}

	allowed := []DeliveryCredentialInput{
		{
			DeliveryKind: DeliveryKindAPIKeyEndpoint,
			APIBaseURL:   "https://api.example.com/v1",
			APIKey:       "sk-proj-test",
			Instructions: "买家专属、可撤销；后续更换请站外联系。",
		},
		{
			DeliveryKind:  DeliveryKindLoginAccount,
			PanelLoginURL: "https://panel.example.com/login",
			Username:      "buyer-demo",
			Password:      "initial-password-123",
			Instructions:  "首次登录后请按面板提示完成设置。",
		},
	}
	for i, credential := range allowed {
		working := order
		working.ID = "allowed-" + string(rune('a'+i))
		service.orders[working.ID] = working
		completion, appErr := service.SubmitDeliveryWithIdempotency(context.Background(), "seller-1", "api-order-submit-delivery", "allow-"+string(rune('a'+i)), "hash-allow-"+string(rune('a'+i)), ActionInput{
			OrderID:            working.ID,
			DeliveryCredential: credential,
			ExpectedVersion:    1,
			RequestID:          "allow",
		}, testAPIOrderCompletion)
		if appErr != nil {
			t.Fatalf("expected credential to be allowed, got %v", appErr)
		}
		if completion.Status != http.StatusOK {
			t.Fatalf("unexpected completion for credential: %+v", completion)
		}
		stored := service.orders[working.ID]
		if stored.DeliveryNote == "" || stored.DeliveryCredential == nil {
			t.Fatalf("expected delivery summary and credential on order: %+v", stored)
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

func TestNewOrderRejectsLegacyUSDTPaymentOption(t *testing.T) {
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	service := testOrderableService(now)
	service.PaymentOptions = append(service.PaymentOptions, apimarket.PaymentOption{
		ID:                  "payment-usdt",
		PaymentMethod:       "usdt",
		Enabled:             true,
		PaymentInstructions: "TRC20 地址站外确认。",
		CreatedAt:           now,
		UpdatedAt:           now,
		Version:             1,
	})
	intent := apiintent.Intent{
		ID:                  "intent-1",
		APIServiceID:        "service-1",
		BuyerUserID:         "buyer-1",
		OwnerUserID:         "seller-1",
		Status:              apiintent.StatusOpen,
		RequestedCNYAmount:  "16.00",
		SelectedAccessMode:  "buyer_dedicated_sub_key",
		BillingModeSnapshot: apimarket.ServiceBillingModeMetered,
	}

	_, appErr := NewOrder(CreateInput{
		IntentID:      "intent-1",
		BuyerUserID:   "buyer-1",
		PaymentMethod: "usdt",
		RequestID:     "create-1",
	}, intent, service, now)
	if appErr == nil || appErr.Status != http.StatusUnprocessableEntity || len(appErr.FieldErrors) != 1 || appErr.FieldErrors[0].Field != "paymentMethod" {
		t.Fatalf("expected legacy USDT payment method to be rejected, got %v", appErr)
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
			ID:                   "payment-1",
			PaymentMethod:        apimarket.PaymentMethodWechat,
			Enabled:              true,
			PaymentInstructions:  "站外确认付款说明。",
			PaymentQRCodeDataURL: "data:image/png;base64,ZmFrZS1xcg==",
			CreatedAt:            now,
			UpdatedAt:            now,
			Version:              1,
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
