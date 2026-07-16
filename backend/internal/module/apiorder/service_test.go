package apiorder

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
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
			Instructions: "买家专属；提交后不可修改，后续更换请站外联系。",
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

func TestPaymentIssueRequiresStructuredReasonAndReturnsToSubmitted(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	service := NewService(nil, nil, nil, nil, nil, func() time.Time {
		now = now.Add(time.Second)
		return now
	})
	service.orders["order-payment-issue"] = Order{
		ID:                 "order-payment-issue",
		BuyerUserID:        "buyer-1",
		SellerUserID:       "seller-1",
		Status:             StatusPaymentSubmitted,
		DisputeStatus:      DisputeStatusNone,
		PaymentSummary:     "付款时间 12:00，备注 API-001",
		PaymentSubmittedAt: timePointer(now),
		PaymentExpiresAt:   now.Add(10 * time.Minute),
		CreatedAt:          now,
		UpdatedAt:          now,
		Version:            2,
	}

	_, appErr := service.ReportPaymentIssueWithIdempotency(context.Background(), "seller-1", "report-payment-issue", "invalid-reason", "invalid-reason-hash", ActionInput{
		OrderID:            "order-payment-issue",
		PaymentIssueReason: "other",
		ExpectedVersion:    2,
	}, testAPIOrderCompletion)
	if appErr == nil || appErr.Status != http.StatusUnprocessableEntity {
		t.Fatalf("expected unsupported reason to be rejected, got %v", appErr)
	}

	_, appErr = service.ReportPaymentIssueWithIdempotency(context.Background(), "seller-1", "report-payment-issue", "report-issue", "report-issue-hash", ActionInput{
		OrderID:            "order-payment-issue",
		PaymentIssueReason: PaymentIssueAmountMismatch,
		PaymentIssueNote:   "实收金额与订单金额不一致。",
		ExpectedVersion:    2,
	}, testAPIOrderCompletion)
	if appErr != nil {
		t.Fatalf("report payment issue: %v", appErr)
	}
	issueOrder := service.orders["order-payment-issue"]
	if issueOrder.Status != StatusPaymentIssue || issueOrder.PaymentIssueReason != PaymentIssueAmountMismatch || issueOrder.PaymentIssueReportedAt == nil {
		t.Fatalf("expected payment issue state and metadata, got %+v", issueOrder)
	}

	_, appErr = service.SubmitPaymentWithIdempotency(context.Background(), "buyer-1", "submit-payment", "resubmit-payment", "resubmit-payment-hash", ActionInput{
		OrderID:         issueOrder.ID,
		PaymentSummary:  "实际付款 ¥10.00，付款时间 12:00，交易尾号 1234。",
		ExpectedVersion: issueOrder.Version,
	}, testAPIOrderCompletion)
	if appErr != nil {
		t.Fatalf("resubmit payment information: %v", appErr)
	}
	resubmitted := service.orders["order-payment-issue"]
	if resubmitted.Status != StatusPaymentSubmitted || resubmitted.PaymentIssueReason != "" || resubmitted.PaymentIssueNote != "" || resubmitted.PaymentIssueReportedAt != nil {
		t.Fatalf("expected resubmission to clear payment issue fields, got %+v", resubmitted)
	}
}

func timePointer(value time.Time) *time.Time {
	return &value
}

func TestCreateOrderForSameIntentReturnsDedicatedConflict(t *testing.T) {
	now := time.Date(2026, 6, 27, 12, 0, 0, 0, time.UTC)
	intents := &testIntentResolver{intent: apiintent.Intent{
		ID:                                 "intent-1",
		APIServiceID:                       "service-1",
		BuyerUserID:                        "buyer-1",
		OwnerUserID:                        "seller-1",
		Status:                             apiintent.StatusOpen,
		RequestedCNYAmount:                 "16.00",
		RequestedUSDAllowance:              "20.000000",
		DeclaredCNYPerUSDAllowanceSnapshot: "0.8000",
		SelectedAccessMode:                 "buyer_dedicated_sub_key",
		BillingModeSnapshot:                apimarket.ServiceBillingModeMetered,
	}}
	services := &testPublicServiceResolver{service: testOrderableService(now)}
	service := NewService(nil, intents, services, nil, nil, func() time.Time { return now })
	initialIntentVersion := intents.intent.Version

	_, appErr := service.CreateWithIdempotency(context.Background(), "buyer-1", "api-order-create", "create-1", "hash-1", CreateInput{
		IntentID:      "intent-1",
		PaymentMethod: apimarket.PaymentMethodWechat,
		RequestID:     "create-1",
	}, testAPIOrderCompletion)
	if appErr != nil {
		t.Fatalf("first create order: %v", appErr)
	}
	if intents.intent.Status != apiintent.StatusOrdered || intents.intent.Version != initialIntentVersion+1 {
		t.Fatalf("expected created order to mark intent ordered, got %+v", intents.intent)
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

func TestMeteredOrderReservesAndPendingCancellationReleasesAllowance(t *testing.T) {
	now := time.Date(2026, 7, 12, 1, 0, 0, 0, time.UTC)
	intents := &testIntentResolver{intent: apiintent.Intent{
		ID:                                       "intent-inventory",
		APIServiceID:                             "service-1",
		BuyerUserID:                              "buyer-1",
		OwnerUserID:                              "seller-1",
		Status:                                   apiintent.StatusOpen,
		RequestedCNYAmount:                       "10.00",
		RequestedUSDAllowance:                    "12.500000",
		DeclaredCNYPerUSDAllowanceSnapshot:       "0.8000",
		DeclaredMaxUSDAllowancePerIntentSnapshot: "20.000000",
		SelectedAccessMode:                       "buyer_dedicated_sub_key",
		BillingModeSnapshot:                      apimarket.ServiceBillingModeMetered,
	}}
	serviceRecord := testOrderableService(now)
	serviceRecord.AvailableUSDAllowance = "20.000000"
	service := NewService(nil, intents, &testPublicServiceResolver{service: serviceRecord}, nil, nil, func() time.Time { return now })

	completion, appErr := service.CreateWithIdempotency(context.Background(), "buyer-1", "api-order-create", "inventory-create", "inventory-create-hash", CreateInput{
		IntentID:      intents.intent.ID,
		PaymentMethod: apimarket.PaymentMethodWechat,
		RequestID:     "inventory-create",
	}, testAPIOrderCompletion)
	if appErr != nil {
		t.Fatalf("create metered order: %v", appErr)
	}
	if got := decimalStringOptional(service.availableAllowances[serviceRecord.ID].RatString(), 6); got != "7.500000" {
		t.Fatalf("expected 7.500000 available after reservation, got %s", got)
	}

	orderID := completion.ResourceID
	order := service.orders[orderID]
	_, appErr = service.CancelWithIdempotency(context.Background(), "buyer-1", "api-order-cancel", "inventory-cancel", "inventory-cancel-hash", ActionInput{
		OrderID:         orderID,
		Reason:          "买家尚未付款，取消本次订单。",
		ExpectedVersion: order.Version,
		RequestID:       "inventory-cancel",
	}, testAPIOrderCompletion)
	if appErr != nil {
		t.Fatalf("cancel pending metered order: %v", appErr)
	}
	if got := decimalStringOptional(service.availableAllowances[serviceRecord.ID].RatString(), 6); got != "20.000000" {
		t.Fatalf("expected allowance release to 20.000000, got %s", got)
	}
}

func TestConcurrentMeteredOrdersCannotOversellAllowance(t *testing.T) {
	now := time.Date(2026, 7, 12, 1, 0, 0, 0, time.UTC)
	intentTemplate := apiintent.Intent{
		APIServiceID:                             "service-1",
		BuyerUserID:                              "buyer-1",
		OwnerUserID:                              "seller-1",
		Status:                                   apiintent.StatusOpen,
		RequestedCNYAmount:                       "10.00",
		RequestedUSDAllowance:                    "12.500000",
		DeclaredCNYPerUSDAllowanceSnapshot:       "0.8000",
		DeclaredMaxUSDAllowancePerIntentSnapshot: "20.000000",
		SelectedAccessMode:                       "buyer_dedicated_sub_key",
		BillingModeSnapshot:                      apimarket.ServiceBillingModeMetered,
	}
	first := intentTemplate
	first.ID = "intent-concurrent-1"
	second := intentTemplate
	second.ID = "intent-concurrent-2"
	intents := &testMultiIntentResolver{intents: map[string]apiintent.Intent{
		first.ID:  first,
		second.ID: second,
	}}
	serviceRecord := testOrderableService(now)
	serviceRecord.AvailableUSDAllowance = "20.000000"
	service := NewService(nil, intents, &testPublicServiceResolver{service: serviceRecord}, nil, nil, func() time.Time { return now })

	type result struct {
		completion idempotency.Completion
		err        *domain.AppError
	}
	results := make(chan result, 2)
	var wait sync.WaitGroup
	for index, intentID := range []string{first.ID, second.ID} {
		wait.Add(1)
		go func(index int, intentID string) {
			defer wait.Done()
			completion, appErr := service.CreateWithIdempotency(context.Background(), "buyer-1", "api-order-create", fmt.Sprintf("concurrent-%d", index), fmt.Sprintf("concurrent-hash-%d", index), CreateInput{
				IntentID:      intentID,
				PaymentMethod: apimarket.PaymentMethodWechat,
				RequestID:     fmt.Sprintf("concurrent-%d", index),
			}, testAPIOrderCompletion)
			results <- result{completion: completion, err: appErr}
		}(index, intentID)
	}
	wait.Wait()
	close(results)

	successes := 0
	conflicts := 0
	for item := range results {
		if item.err == nil && item.completion.ResourceID != "" {
			successes++
			continue
		}
		if item.err != nil && item.err.Status == http.StatusConflict {
			conflicts++
		}
	}
	if successes != 1 || conflicts != 1 || len(service.orders) != 1 {
		t.Fatalf("expected one reservation and one inventory conflict, got successes=%d conflicts=%d orders=%d", successes, conflicts, len(service.orders))
	}
	if got := decimalStringOptional(service.availableAllowances[serviceRecord.ID].RatString(), 6); got != "7.500000" {
		t.Fatalf("expected 7.500000 remaining without oversell, got %s", got)
	}
}

func TestLimitedPackageStockLifecycleAndDeliveryExpiry(t *testing.T) {
	now := time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)
	first := fixedPackageIntent("intent-package-cancel")
	second := fixedPackageIntent("intent-package-paid")
	intents := &testMultiIntentResolver{intents: map[string]apiintent.Intent{
		first.ID:  first,
		second.ID: second,
	}}
	serviceRecord := testFixedPackageService(now, 1)
	service := NewService(nil, intents, &testPublicServiceResolver{service: serviceRecord}, nil, nil, func() time.Time { return now })

	firstCompletion, appErr := service.CreateWithIdempotency(context.Background(), "buyer-1", "api-order-create", "package-create-cancel", "package-create-cancel-hash", CreateInput{
		IntentID:      first.ID,
		PaymentMethod: apimarket.PaymentMethodWechat,
		RequestID:     "package-create-cancel",
	}, testAPIOrderCompletion)
	if appErr != nil {
		t.Fatalf("create cancellable package order: %v", appErr)
	}
	if service.availablePackageStock["package-1"] != 0 || !service.orders[firstCompletion.ResourceID].PackageStockReserved {
		t.Fatalf("expected package stock reservation, stock=%d order=%+v", service.availablePackageStock["package-1"], service.orders[firstCompletion.ResourceID])
	}
	firstOrder := service.orders[firstCompletion.ResourceID]
	_, appErr = service.CancelWithIdempotency(context.Background(), "buyer-1", "api-order-cancel", "package-cancel", "package-cancel-hash", ActionInput{
		OrderID:         firstOrder.ID,
		Reason:          "尚未付款，取消订单。",
		ExpectedVersion: firstOrder.Version,
		RequestID:       "package-cancel",
	}, testAPIOrderCompletion)
	if appErr != nil {
		t.Fatalf("cancel package order: %v", appErr)
	}
	if service.availablePackageStock["package-1"] != 1 || service.orders[firstOrder.ID].PackageStockReserved {
		t.Fatalf("expected cancellation to release stock exactly once")
	}

	secondCompletion, appErr := service.CreateWithIdempotency(context.Background(), "buyer-1", "api-order-create", "package-create-paid", "package-create-paid-hash", CreateInput{
		IntentID:      second.ID,
		PaymentMethod: apimarket.PaymentMethodWechat,
		RequestID:     "package-create-paid",
	}, testAPIOrderCompletion)
	if appErr != nil {
		t.Fatalf("create paid package order: %v", appErr)
	}
	paidOrder := service.orders[secondCompletion.ResourceID]
	_, appErr = service.SubmitPaymentWithIdempotency(context.Background(), "buyer-1", "api-order-submit-payment", "package-submit-payment", "package-submit-payment-hash", ActionInput{
		OrderID:         paidOrder.ID,
		PaymentSummary:  "已按订单金额付款。",
		ExpectedVersion: paidOrder.Version,
		RequestID:       "package-submit-payment",
	}, testAPIOrderCompletion)
	if appErr != nil {
		t.Fatalf("submit package payment: %v", appErr)
	}
	paidOrder = service.orders[paidOrder.ID]
	_, appErr = service.ConfirmPaymentWithIdempotency(context.Background(), "seller-1", "api-order-confirm-payment", "package-confirm-payment", "package-confirm-payment-hash", ActionInput{
		OrderID:         paidOrder.ID,
		ExpectedVersion: paidOrder.Version,
		RequestID:       "package-confirm-payment",
	}, testAPIOrderCompletion)
	if appErr != nil {
		t.Fatalf("confirm package payment: %v", appErr)
	}
	paidOrder = service.orders[paidOrder.ID]
	if service.availablePackageStock["package-1"] != 0 || paidOrder.PackageStockReserved {
		t.Fatalf("expected confirmed payment to consume stock permanently")
	}
	_, appErr = service.SubmitDeliveryWithIdempotency(context.Background(), "seller-1", "api-order-submit-delivery", "package-submit-delivery", "package-submit-delivery-hash", ActionInput{
		OrderID:         paidOrder.ID,
		ExpectedVersion: paidOrder.Version,
		RequestID:       "package-submit-delivery",
		DeliveryCredential: DeliveryCredentialInput{
			DeliveryKind: DeliveryKindAPIKeyEndpoint,
			APIBaseURL:   "https://api.example.com/v1",
			APIKey:       "sk-package-test",
		},
	}, testAPIOrderCompletion)
	if appErr != nil {
		t.Fatalf("submit package delivery: %v", appErr)
	}
	delivered := service.orders[paidOrder.ID]
	wantExpiry := now.AddDate(0, 0, 3)
	if delivered.PackageExpiresAt == nil || !delivered.PackageExpiresAt.Equal(wantExpiry) {
		t.Fatalf("expected delivery-based package expiry %s, got %v", wantExpiry, delivered.PackageExpiresAt)
	}
}

func TestConcurrentLimitedPackageOrdersCannotReserveLastStockTwice(t *testing.T) {
	now := time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC)
	first := fixedPackageIntent("intent-package-concurrent-1")
	second := fixedPackageIntent("intent-package-concurrent-2")
	intents := &testMultiIntentResolver{intents: map[string]apiintent.Intent{first.ID: first, second.ID: second}}
	service := NewService(nil, intents, &testPublicServiceResolver{service: testFixedPackageService(now, 1)}, nil, nil, func() time.Time { return now })

	results := make(chan *domain.AppError, 2)
	var wait sync.WaitGroup
	for index, intentID := range []string{first.ID, second.ID} {
		wait.Add(1)
		go func(index int, intentID string) {
			defer wait.Done()
			_, appErr := service.CreateWithIdempotency(context.Background(), "buyer-1", "api-order-create", fmt.Sprintf("package-concurrent-%d", index), fmt.Sprintf("package-concurrent-hash-%d", index), CreateInput{
				IntentID:      intentID,
				PaymentMethod: apimarket.PaymentMethodWechat,
				RequestID:     fmt.Sprintf("package-concurrent-%d", index),
			}, testAPIOrderCompletion)
			results <- appErr
		}(index, intentID)
	}
	wait.Wait()
	close(results)
	successes := 0
	conflicts := 0
	for appErr := range results {
		if appErr == nil {
			successes++
		} else if appErr.Status == http.StatusConflict {
			conflicts++
		}
	}
	if successes != 1 || conflicts != 1 || len(service.orders) != 1 || service.availablePackageStock["package-1"] != 0 {
		t.Fatalf("expected one successful last-stock reservation, successes=%d conflicts=%d orders=%d stock=%d", successes, conflicts, len(service.orders), service.availablePackageStock["package-1"])
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

type testMultiIntentResolver struct {
	mu      sync.Mutex
	intents map[string]apiintent.Intent
}

func (r *testMultiIntentResolver) MarkOrdered(intentID string) *domain.AppError {
	r.mu.Lock()
	defer r.mu.Unlock()
	intent, ok := r.intents[intentID]
	if !ok || (intent.Status != apiintent.StatusOpen && intent.Status != apiintent.StatusContacted) {
		return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前购买意向状态不能生成订单。")
	}
	intent.Status = apiintent.StatusOrdered
	intent.Version++
	r.intents[intentID] = intent
	return nil
}

func (r *testMultiIntentResolver) BuyerIntent(_ context.Context, user auth.User, intentID, _ string) (apiintent.Intent, *domain.AppError) {
	r.mu.Lock()
	defer r.mu.Unlock()
	intent, ok := r.intents[intentID]
	if !ok || intent.BuyerUserID != user.ID {
		return apiintent.Intent{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API purchase intent not found", "购买意向不存在。")
	}
	return intent, nil
}

func (r *testIntentResolver) MarkOrdered(intentID string) *domain.AppError {
	if r.intent.ID != intentID || (r.intent.Status != apiintent.StatusOpen && r.intent.Status != apiintent.StatusContacted) {
		return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前购买意向状态不能生成订单。")
	}
	r.intent.Status = apiintent.StatusOrdered
	r.intent.Version++
	return nil
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
		ID:                               "service-1",
		OwnerUserID:                      "seller-1",
		OwnerContactMethodID:             "owner-contact-1",
		Title:                            "测试 API 服务",
		DistributionSystem:               apimarket.ServiceDistributionSub2API,
		BillingMode:                      apimarket.ServiceBillingModeMetered,
		DeclaredCNYPerUSDAllowance:       "0.8000",
		DeclaredMaxUSDAllowancePerIntent: "500.000000",
		AvailableUSDAllowance:            "500.000000",
		QuotaExpiresAt:                   &quotaExpiresAt,
		MinimumIntentCNY:                 "10.00",
		AcceptingOrders:                  true,
		PaymentWindowMinutes:             10,
		ReviewStatus:                     apimarket.ServiceReviewStatusApproved,
		PublicationStatus:                apimarket.ServicePublicationStatusOnline,
		ModerationStatus:                 apimarket.ServiceModerationStatusClear,
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

func fixedPackageIntent(id string) apiintent.Intent {
	return apiintent.Intent{
		ID:                      id,
		APIServiceID:            "service-1",
		BuyerUserID:             "buyer-1",
		OwnerUserID:             "seller-1",
		Status:                  apiintent.StatusOpen,
		RequestedCNYAmount:      "9.90",
		SelectedAccessMode:      "fixed_package_offsite",
		SelectedPackageID:       "package-1",
		SelectedPackageSnapshot: `{"id":"package-1","name":"3 天套餐","priceCny":"9.90","panelAllowance":"5.000000","durationDays":3,"models":[{"serviceModelId":"service-model-1","modelCatalogId":"model-1","modelPriceVersionId":"price-version-1","modelNameSnapshot":"GPT-5.6","merchantMultiplier":"0.0100"}]}`,
		BillingModeSnapshot:     apimarket.ServiceBillingModeFixedPackage,
	}
}

func testFixedPackageService(now time.Time, stock int) apimarket.Service {
	duration := 3
	service := testOrderableService(now)
	service.BillingMode = apimarket.ServiceBillingModeFixedPackage
	service.DeclaredCNYPerUSDAllowance = ""
	service.DeclaredMaxUSDAllowancePerIntent = ""
	service.AvailableUSDAllowance = ""
	service.QuotaExpiresAt = nil
	service.AccessModes = []apimarket.ServiceAccessMode{{AccessMode: "fixed_package_offsite"}}
	service.Packages = []apimarket.ServicePackage{{
		ID:             "package-1",
		Name:           "3 天 GPT-5.6 套餐",
		PriceCNY:       "9.90",
		PanelAllowance: "5.000000",
		DurationDays:   &duration,
		StockTotal:     stock,
		StockAvailable: stock,
		Enabled:        true,
		Models: []apimarket.ServicePackageModel{{
			ServiceModelID:      "service-model-1",
			ModelCatalogID:      "model-1",
			ModelPriceVersionID: "price-version-1",
			ModelNameSnapshot:   "GPT-5.6",
			ProviderSnapshot:    "OpenAI",
			MerchantMultiplier:  "0.0100",
		}},
	}}
	return apimarket.WithOrderability(service)
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
