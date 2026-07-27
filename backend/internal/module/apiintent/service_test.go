package apiintent

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/contact"
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

func TestLimitedPackageIntentFreezesExactModelSnapshot(t *testing.T) {
	now := time.Date(2026, 7, 16, 9, 0, 0, 0, time.UTC)
	duration := 3
	service := limitedPackageIntentService(now, []apimarket.ServicePackage{{
		ID:             "package-1",
		Name:           "3 天 GPT-5.6 套餐",
		PriceCNY:       "9.90",
		PanelAllowance: "5.000000",
		DurationDays:   &duration,
		StockTotal:     2,
		StockAvailable: 2,
		Enabled:        true,
		Models: []apimarket.ServicePackageModel{{
			ServiceModelID:      "service-model-1",
			ModelCatalogID:      "model-1",
			ModelPriceVersionID: "price-version-1",
			ModelNameSnapshot:   "GPT-5.6",
			ProviderSnapshot:    "OpenAI",
			MerchantMultiplier:  "0.0100",
		}},
	}})
	intent, appErr := NewIntent(CreateIntentInput{
		APIServiceID:         service.ID,
		BuyerUserID:          "buyer-1",
		BuyerContactMethodID: "buyer-contact-1",
		RequestedCNYAmount:   "9.90",
		SelectedAccessMode:   "fixed_package_offsite",
		SelectedPackageID:    "package-1",
	}, service, contact.ContactMethod{Type: "telegram", Label: "买家 TG"}, contact.ContactMethodVersion{ID: "buyer-version-1"}, contact.ContactMethod{Type: "telegram", Label: "卖家 TG"}, contact.ContactMethodVersion{ID: "owner-version-1"}, now)
	if appErr != nil {
		t.Fatalf("new limited-package intent: %v", appErr)
	}
	var snapshot struct {
		PanelAllowance string `json:"panelAllowance"`
		DurationDays   int    `json:"durationDays"`
		Models         []struct {
			ModelPriceVersionID string `json:"modelPriceVersionId"`
			ModelNameSnapshot   string `json:"modelNameSnapshot"`
			MerchantMultiplier  string `json:"merchantMultiplier"`
		} `json:"models"`
	}
	if err := json.Unmarshal([]byte(intent.SelectedPackageSnapshot), &snapshot); err != nil {
		t.Fatalf("decode package snapshot: %v", err)
	}
	if snapshot.PanelAllowance != "5.000000" || snapshot.DurationDays != 3 || len(snapshot.Models) != 1 || snapshot.Models[0].ModelNameSnapshot != "GPT-5.6" || snapshot.Models[0].ModelPriceVersionID != "price-version-1" || snapshot.Models[0].MerchantMultiplier != "0.0100" {
		t.Fatalf("unexpected package snapshot: %+v", snapshot)
	}
}

func TestLimitedPackageIntentRejectsSelectedSoldOutPackage(t *testing.T) {
	now := time.Date(2026, 7, 16, 9, 0, 0, 0, time.UTC)
	duration := 3
	model := []apimarket.ServicePackageModel{{ServiceModelID: "service-model-1", ModelCatalogID: "model-1", ModelNameSnapshot: "GPT-5.6", MerchantMultiplier: "1.0000"}}
	service := limitedPackageIntentService(now, []apimarket.ServicePackage{
		{ID: "sold-out", Name: "售罄套餐", PriceCNY: "9.90", PanelAllowance: "5", DurationDays: &duration, StockTotal: 1, StockAvailable: 0, Enabled: true, Models: model},
		{ID: "available", Name: "有货套餐", PriceCNY: "19.90", PanelAllowance: "10", DurationDays: &duration, StockTotal: 1, StockAvailable: 1, Enabled: true, Models: model},
	})
	appErr := validateCreateInput(CreateIntentInput{
		APIServiceID:         service.ID,
		BuyerUserID:          "buyer-1",
		BuyerContactMethodID: "buyer-contact-1",
		RequestedCNYAmount:   "9.90",
		SelectedAccessMode:   "fixed_package_offsite",
		SelectedPackageID:    "sold-out",
	}, service)
	if appErr == nil || len(appErr.FieldErrors) != 1 || appErr.FieldErrors[0].Field != "selectedPackageId" {
		t.Fatalf("expected sold-out package rejection, got %+v", appErr)
	}
}

func limitedPackageIntentService(now time.Time, packages []apimarket.ServicePackage) apimarket.Service {
	service := apimarket.Service{
		ID:                   "service-1",
		OwnerUserID:          "seller-1",
		OwnerContactMethodID: "owner-contact-1",
		Title:                "限时套餐服务",
		DistributionSystem:   apimarket.ServiceDistributionSub2API,
		BillingMode:          apimarket.ServiceBillingModeFixedPackage,
		MinimumIntentCNY:     "1.00",
		MaximumIntentCNY:     "100.00",
		AcceptingOrders:      true,
		PaymentWindowMinutes: 10,
		ReviewStatus:         apimarket.ServiceReviewStatusApproved,
		PublicationStatus:    apimarket.ServicePublicationStatusOnline,
		ModerationStatus:     apimarket.ServiceModerationStatusClear,
		AccessModes:          []apimarket.ServiceAccessMode{{AccessMode: "fixed_package_offsite"}},
		Packages:             packages,
		PaymentOptions:       []apimarket.PaymentOption{{PaymentMethod: apimarket.PaymentMethodWechat, Enabled: true}},
		CreatedAt:            now,
		UpdatedAt:            now,
		Version:              1,
	}
	return apimarket.WithOrderability(service)
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
