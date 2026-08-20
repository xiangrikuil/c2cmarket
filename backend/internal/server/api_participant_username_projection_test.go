package server

import (
	"testing"

	"c2c-market/backend/internal/module/apiintent"
	"c2c-market/backend/internal/module/apiorder"
)

func TestAPIPurchaseIntentResponsesExposeViewSpecificUsernames(t *testing.T) {
	intent := apiintent.Intent{
		BuyerUserID:   "buyer-id",
		BuyerUsername: "lin_buyer",
		OwnerUserID:   "owner-id",
		OwnerUsername: "api_merchant",
	}

	buyer := toBuyerAPIPurchaseIntentListItemResponse(intent)
	if buyer.OwnerUsername != "api_merchant" {
		t.Fatalf("buyer owner username = %q", buyer.OwnerUsername)
	}
	owner := toOwnerAPIPurchaseIntentListItemResponse(intent)
	if owner.BuyerUsername != "lin_buyer" {
		t.Fatalf("owner buyer username = %q", owner.BuyerUsername)
	}
	admin := toAdminAPIPurchaseIntentDetailResponse(intent)
	if admin.BuyerUsername != "lin_buyer" || admin.OwnerUsername != "api_merchant" {
		t.Fatalf("admin participant usernames = %+v", admin)
	}
}

func TestAPIOrderResponsesExposeViewSpecificUsernames(t *testing.T) {
	order := apiorder.Order{
		BuyerUserID:    "buyer-id",
		BuyerUsername:  "lin_buyer",
		SellerUserID:   "seller-id",
		SellerUsername: "api_merchant",
	}

	buyer := toAPIOrderResponse(order, false, false)
	if buyer.SellerUsername != "api_merchant" || buyer.BuyerUsername != "" {
		t.Fatalf("buyer response participant usernames = %+v", buyer)
	}
	owner := toAPIOrderResponse(order, true, false)
	if owner.BuyerUsername != "lin_buyer" || owner.SellerUsername != "" {
		t.Fatalf("owner response participant usernames = %+v", owner)
	}
	admin := toAdminAPIOrderResponse(order)
	if admin.BuyerUsername != "lin_buyer" || admin.SellerUsername != "api_merchant" {
		t.Fatalf("admin response participant usernames = %+v", admin)
	}
}
