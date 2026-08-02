package server

import (
	"testing"
	"time"

	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/reputation"
)

func TestAPIOrderResponseIncludesBuyerAndSellerReputation(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	buyer := testReputationSnapshot("buyer-1", reputation.RoleBuyer, reputation.ScopeAPI)
	seller := testReputationSnapshot("seller-1", reputation.RoleSeller, reputation.ScopeAPI)
	completionRate := 0.75
	seller.Metrics.RoleCompletionRate = &completionRate
	response := toAPIOrderResponse(apiorder.Order{
		ID:               "order-1",
		BuyerUserID:      "buyer-1",
		SellerUserID:     "seller-1",
		BuyerReputation:  &buyer,
		SellerReputation: &seller,
		PaymentExpiresAt: now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}, false, false)

	if response.BuyerReputation == nil || response.BuyerReputation.Role != reputation.RoleBuyer {
		t.Fatalf("buyer reputation missing: %#v", response.BuyerReputation)
	}
	if response.SellerReputation == nil || response.SellerReputation.Role != reputation.RoleSeller {
		t.Fatalf("seller reputation missing: %#v", response.SellerReputation)
	}
	if response.SellerReputation.RoleCompletionRate == nil || *response.SellerReputation.RoleCompletionRate != completionRate {
		t.Fatalf("seller completion rate missing: %#v", response.SellerReputation)
	}
}
