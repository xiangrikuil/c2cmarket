package apimarket

import (
	"testing"
	"time"
)

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
