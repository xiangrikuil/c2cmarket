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

func TestOrderableReasonsIncludesExpiredQuota(t *testing.T) {
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	expiredAt := now.Add(-time.Minute)
	service := Service{
		OwnerContactMethodID: "contact-1",
		BillingMode:         ServiceBillingModeMetered,
		QuotaExpiresAt:      &expiredAt,
		AcceptingOrders:     true,
		PaymentWindowMinutes: 10,
		ReviewStatus:        ServiceReviewStatusApproved,
		PublicationStatus:   ServicePublicationStatusOnline,
		ModerationStatus:    ServiceModerationStatusClear,
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
