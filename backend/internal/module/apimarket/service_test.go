package apimarket

import (
	"context"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/catalog"
)

type staticAPIModelResolver struct {
	models map[string]catalog.APIModelCatalog
}

func (r staticAPIModelResolver) APIModel(_ context.Context, modelID string) (catalog.APIModelCatalog, *domain.AppError) {
	return r.models[modelID], nil
}

func TestValidateCreateInputRequiresFutureQuotaExpirationForMeteredServices(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	input := validMeteredCreateInput()
	input.QuotaExpiresAt = "2026-07-07T11:59:00Z"

	err := validateCreateInput(input, now)
	if err == nil {
		t.Fatalf("expected expired quota timestamp to be rejected")
	}
	if len(err.FieldErrors) != 1 || err.FieldErrors[0].Field != "quotaExpiresAt" {
		t.Fatalf("expected quotaExpiresAt field error, got %+v", err)
	}
}

func TestValidateCreateInputAllowsOptionalLinuxDoSourceURL(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	input := validMeteredCreateInput()
	input.SourceURL = " https://linux.do/t/api-quota/123456 "

	if err := validateCreateInput(input, now); err != nil {
		t.Fatalf("expected optional linux.do source URL to be valid, got %+v", err)
	}

	input.SourceURL = ""
	if err := validateCreateInput(input, now); err != nil {
		t.Fatalf("expected empty source URL to be valid, got %+v", err)
	}
}

func TestValidateCreateInputRejectsInvalidSourceURL(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	input := validMeteredCreateInput()
	input.SourceURL = "https://example.com/post?token=secret"

	err := validateCreateInput(input, now)
	if err == nil {
		t.Fatalf("expected invalid source URL to be rejected")
	}
	if len(err.FieldErrors) != 1 || err.FieldErrors[0].Field != "sourceUrl" {
		t.Fatalf("expected sourceUrl field error, got %+v", err)
	}
}

func TestOrderableReasonsIncludesExpiredQuota(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	expiredAt := now.Add(-time.Minute)
	service := Service{
		OwnerContactMethodID:  "contact-1",
		BillingMode:           ServiceBillingModeMetered,
		AvailableUSDAllowance: "20.000000",
		QuotaExpiresAt:        &expiredAt,
		AcceptingOrders:       true,
		PaymentWindowMinutes:  10,
		ReviewStatus:          ServiceReviewStatusApproved,
		PublicationStatus:     ServicePublicationStatusOnline,
		ModerationStatus:      ServiceModerationStatusClear,
		PaymentOptions: []PaymentOption{{
			PaymentMethod: PaymentMethodWechat,
			Enabled:       true,
		}},
	}

	reasons := OrderableReasonsAt(service, now)
	if len(reasons) != 1 || reasons[0] != "quota_expired" {
		t.Fatalf("expected only quota_expired reason, got %#v", reasons)
	}
}

func TestValidateOrderSettingsRejectsUSDTPaymentMethod(t *testing.T) {
	err := validateOrderSettingsInput(UpdateOrderSettingsInput{
		AcceptingOrders:      true,
		PaymentWindowMinutes: 10,
		PaymentOptions: []PaymentOptionInput{{
			PaymentMethod:       "usdt",
			Enabled:             true,
			PaymentInstructions: "TRC20 地址站外确认。",
		}},
	})
	if err == nil {
		t.Fatalf("expected USDT payment method to be rejected")
	}
	if len(err.FieldErrors) != 1 || err.FieldErrors[0].Field != "paymentOptions.0.paymentMethod" {
		t.Fatalf("expected payment method field error, got %+v", err)
	}
}

func TestBuildPaymentOptionsSkipsDisabledEmptyInstructions(t *testing.T) {
	input := UpdateOrderSettingsInput{
		AcceptingOrders:      true,
		PaymentWindowMinutes: 10,
		PaymentOptions: []PaymentOptionInput{
			{
				PaymentMethod:        PaymentMethodWechat,
				Enabled:              true,
				PaymentInstructions:  "微信收款二维码请按商户站外确认展示。",
				PaymentQRCodeDataURL: "data:image/png;base64,ZmFrZS1xcg==",
			},
			{
				PaymentMethod:       PaymentMethodAlipay,
				Enabled:             false,
				PaymentInstructions: " ",
			},
		},
	}
	if err := validateOrderSettingsInput(input); err != nil {
		t.Fatalf("expected disabled empty payment option placeholder to be valid, got %+v", err)
	}

	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	options := buildPaymentOptions("service-1", nil, input.PaymentOptions, now)
	if len(options) != 1 {
		t.Fatalf("expected one persisted payment option, got %#v", options)
	}
	if options[0].PaymentMethod != PaymentMethodWechat || !options[0].Enabled || options[0].PaymentInstructions == "" || options[0].PaymentQRCodeDataURL == "" {
		t.Fatalf("unexpected persisted payment option: %#v", options[0])
	}
}

func TestOrderableReasonsIgnoreLegacyUSDTPaymentOption(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(time.Hour)
	service := Service{
		OwnerContactMethodID:  "contact-1",
		BillingMode:           ServiceBillingModeMetered,
		AvailableUSDAllowance: "20.000000",
		QuotaExpiresAt:        &expiresAt,
		AcceptingOrders:       true,
		PaymentWindowMinutes:  10,
		ReviewStatus:          ServiceReviewStatusApproved,
		PublicationStatus:     ServicePublicationStatusOnline,
		ModerationStatus:      ServiceModerationStatusClear,
		PaymentOptions: []PaymentOption{{
			PaymentMethod: "usdt",
			Enabled:       true,
		}},
	}

	reasons := OrderableReasonsAt(service, now)
	if len(reasons) != 1 || reasons[0] != "payment_method_required" {
		t.Fatalf("expected legacy USDT to be ignored for orderability, got %#v", reasons)
	}
}

func TestLimitedPackageBuildIgnoresCreateIDAndRetainsUpdateIDs(t *testing.T) {
	now := time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)
	resolver := staticAPIModelResolver{models: map[string]catalog.APIModelCatalog{
		"model-1": {
			ID:                         "model-1",
			DisplayName:                "GPT-5.6",
			Provider:                   "OpenAI",
			Capabilities:               []string{"text"},
			CurrentPriceVersionID:      "price-version-1",
			InputPricePerMillion:       "1.000000",
			CachedInputPricePerMillion: "0.100000",
			OutputPricePerMillion:      "8.000000",
		},
	}}
	manager := NewManager(nil, resolver, nil, func() time.Time { return now })
	input := validLimitedPackageCreateInput()
	input.Packages[0].ID = "client-supplied-id"

	created, appErr := manager.buildFromInput(context.Background(), Service{}, input)
	if appErr != nil {
		t.Fatalf("build limited package service: %v", appErr)
	}
	if created.Packages[0].ID == "client-supplied-id" || created.Packages[0].ID == "" {
		t.Fatalf("expected a server-generated package id, got %q", created.Packages[0].ID)
	}
	if created.Models[0].MerchantMultiplier != "0.0100" || created.Packages[0].Models[0].ModelNameSnapshot != "GPT-5.6" {
		t.Fatalf("expected exact model snapshot and declared multiplier, got %+v", created.Packages[0].Models)
	}

	packageID := created.Packages[0].ID
	modelID := created.Models[0].ID
	created.Packages[0].StockAvailable = 2
	input.Packages[0].ID = packageID
	input.Packages[0].StockTotal = 6
	updated, appErr := manager.buildFromInput(context.Background(), created, input)
	if appErr != nil {
		t.Fatalf("update limited package service: %v", appErr)
	}
	if updated.Packages[0].ID != packageID || updated.Models[0].ID != modelID {
		t.Fatalf("expected stable package/model ids, got package=%q model=%q", updated.Packages[0].ID, updated.Models[0].ID)
	}
	if updated.Packages[0].StockAvailable != 3 {
		t.Fatalf("expected available stock to preserve committed units, got %d", updated.Packages[0].StockAvailable)
	}
}

func TestValidateLimitedPackageRejectsUnsupportedDurationAndModelSubset(t *testing.T) {
	now := time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)
	input := validLimitedPackageCreateInput()
	unsupported := 5
	input.Packages[0].DurationDays = &unsupported
	if err := validateCreateInput(input, now); err == nil || err.FieldErrors[0].Field != "packages.0.durationDays" {
		t.Fatalf("expected unsupported duration error, got %+v", err)
	}

	input = validLimitedPackageCreateInput()
	input.Packages[0].ModelCatalogIDs = []string{"model-not-enabled"}
	if err := validateCreateInput(input, now); err == nil || err.FieldErrors[0].Field != "packages.0.modelCatalogIds.0" {
		t.Fatalf("expected package model subset error, got %+v", err)
	}
}

func validMeteredCreateInput() CreateServiceInput {
	return CreateServiceInput{
		OwnerContactMethodID:             "contact-1",
		MerchantIdentityMode:             "public_profile",
		Title:                            "GPT API quota",
		ShortDescription:                 "GPT API quota",
		DistributionSystem:               ServiceDistributionSub2API,
		BillingMode:                      ServiceBillingModeMetered,
		DeclaredCNYPerUSDAllowance:       "0.8",
		DeclaredMaxUSDAllowancePerIntent: "500",
		AvailableUSDAllowance:            "500",
		MinimumIntentCNY:                 "20",
		MaximumIntentCNY:                 "300",
		QuotaExpiresAt:                   "2026-07-08T00:00:00Z",
		UsageVisibility:                  "merchant_reported",
		AccessModes: []ServiceAccessModeInput{{
			AccessMode: "merchant_operated_endpoint",
		}},
		Models: []ServiceModelInput{{
			ModelCatalogID:     "model-1",
			MerchantMultiplier: "1.0000",
			Enabled:            true,
		}},
	}
}

func validLimitedPackageCreateInput() CreateServiceInput {
	duration := 3
	return CreateServiceInput{
		OwnerContactMethodID: "contact-1",
		MerchantIdentityMode: "public_profile",
		Title:                "GPT 限时套餐",
		ShortDescription:     "按固定价格购买限时面板额度。",
		DistributionSystem:   ServiceDistributionSub2API,
		BillingMode:          ServiceBillingModeFixedPackage,
		MinimumIntentCNY:     "9.90",
		MaximumIntentCNY:     "9.90",
		UsageVisibility:      "fixed_package_only",
		AccessModes: []ServiceAccessModeInput{{
			AccessMode: "fixed_package_offsite",
		}},
		Models: []ServiceModelInput{{
			ModelCatalogID:     "model-1",
			MerchantMultiplier: "0.01",
			Enabled:            true,
		}},
		Packages: []ServicePackageInput{{
			Name:            "3 天 GPT-5.6 套餐",
			PriceCNY:        "9.90",
			PanelAllowance:  "5.000000",
			DurationDays:    &duration,
			StockTotal:      5,
			Description:     "交付后 3 天内有效。",
			Enabled:         true,
			ModelCatalogIDs: []string{"model-1"},
		}},
	}
}
