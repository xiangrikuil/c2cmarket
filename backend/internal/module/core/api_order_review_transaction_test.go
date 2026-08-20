package core

import (
	"testing"

	"c2c-market/backend/internal/module/apiorder"
)

func TestReviewTransactionFromAPIOrderPreservesParticipantUsernames(t *testing.T) {
	t.Parallel()

	transaction := reviewTransactionFromAPIOrder(apiorder.Order{
		BuyerUserID:    "78902aff-0000-0000-0000-000000000000",
		BuyerUsername:  "lin_buyer",
		SellerUserID:   "5999642f-0000-0000-0000-000000000000",
		SellerUsername: "api_merchant",
	})

	if transaction.BuyerUsername != "lin_buyer" || transaction.SellerUsername != "api_merchant" {
		t.Fatalf("participant usernames were not preserved: %#v", transaction)
	}
	if transaction.BuyerUserID != "78902aff-0000-0000-0000-000000000000" || transaction.SellerUserID != "5999642f-0000-0000-0000-000000000000" {
		t.Fatalf("participant IDs changed: %#v", transaction)
	}
}
