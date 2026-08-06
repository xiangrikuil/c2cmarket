package postgres

import (
	"encoding/json"
	"testing"
	"time"

	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/apiquota"
)

func TestAPIQuotaSnapshotFreezesCommercialFacts(t *testing.T) {
	expiresAt := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	promptAuditEnabled := false
	snapshot, appErr := buildAPIQuotaSnapshot(apiQuotaOrderContext{
		OfferID:                  "offer-1",
		BatchID:                  "batch-1",
		OfferName:                "Claude 额度",
		USDAllowance:             "10.000000",
		PriceCNY:                 "8.00",
		CNYPerUSD:                "0.800000",
		ModelMultiplier:          "1.0000",
		SaleMode:                 apiquota.SaleModeContinuous,
		SaleCutoffAt:             expiresAt.Add(-time.Hour),
		ExpiresAt:                expiresAt,
		DistributionSystem:       apimarket.ServiceDistributionSub2API,
		AccountPoolType:          apimarket.AccountPoolCustom,
		AccountPoolCustomName:    "Claude Max",
		MerchantRefundCommitment: true,
		DeclaredTTFTBand:         "1_to_3s",
		DeclaredMaxConcurrency:   12,
		PerformanceConfirmedAt:   timePointer(expiresAt.Add(-24 * time.Hour)),
		PromptAuditEnabled:       &promptAuditEnabled,
		DeliveryETAMinutes:       15,
		DeliveryMode:             "manual",
		ServiceTitle:             "Claude API",
		ServiceVersion:           3,
	}, apiquota.SaleRound{})
	if appErr != nil {
		t.Fatalf("build API quota snapshot: %v", appErr)
	}

	var payload struct {
		AccountPoolType             string    `json:"accountPoolType"`
		AccountPoolLabel            string    `json:"accountPoolLabel"`
		MerchantRefundCommitment    bool      `json:"merchantRefundCommitment"`
		MerchantRefundPolicyVersion string    `json:"merchantRefundPolicyVersion"`
		DeclaredMaxConcurrency      int       `json:"declaredMaxConcurrency"`
		PromptAuditEnabled          *bool     `json:"promptAuditEnabled"`
		ServiceValidityExpiresAt    time.Time `json:"serviceValidityExpiresAt"`
	}
	if err := json.Unmarshal([]byte(snapshot), &payload); err != nil {
		t.Fatalf("decode API quota snapshot: %v", err)
	}
	if payload.AccountPoolType != apimarket.AccountPoolCustom || payload.AccountPoolLabel != "Claude Max" || !payload.MerchantRefundCommitment || payload.MerchantRefundPolicyVersion != apimarket.MerchantRefundPolicyVersion || payload.DeclaredMaxConcurrency != 12 || payload.PromptAuditEnabled == nil || *payload.PromptAuditEnabled || !payload.ServiceValidityExpiresAt.Equal(expiresAt) {
		t.Fatalf("unexpected API quota commercial facts snapshot: %+v", payload)
	}
}

func TestAPIQuotaSnapshotPreservesHistoricalNullCommercialFacts(t *testing.T) {
	expiresAt := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	snapshot, appErr := buildAPIQuotaSnapshot(apiQuotaOrderContext{
		OfferID:      "offer-1",
		BatchID:      "batch-1",
		OfferName:    "历史额度",
		SaleCutoffAt: expiresAt.Add(-time.Hour),
		ExpiresAt:    expiresAt,
	}, apiquota.SaleRound{})
	if appErr != nil {
		t.Fatalf("build historical API quota snapshot: %v", appErr)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(snapshot), &payload); err != nil {
		t.Fatalf("decode historical API quota snapshot: %v", err)
	}
	for _, key := range []string{"accountPoolType", "accountPoolLabel", "declaredMaxConcurrency", "promptAuditEnabled"} {
		value, exists := payload[key]
		if !exists || value != nil {
			t.Fatalf("expected explicit null %s, got exists=%v value=%v", key, exists, value)
		}
	}
}

func TestAPIQuotaOrderAllowsHistoricalNullPromptAuditDeclaration(t *testing.T) {
	item := apiQuotaOrderContext{DeclaredMaxConcurrency: 1, PromptAuditEnabled: nil}
	if appErr := validateAPIQuotaOrderServiceDeclaration(item); appErr != nil {
		t.Fatalf("expected historical null prompt audit declaration to remain purchasable, got %v", appErr)
	}

	item.DeclaredMaxConcurrency = 0
	if appErr := validateAPIQuotaOrderServiceDeclaration(item); appErr == nil {
		t.Fatal("expected missing maximum concurrency to remain invalid")
	}
}
