package apiorder

import (
	"testing"
	"time"
)

func TestEvaluateSellerCommerceKeepsSingleOrdinaryDisputeAtOrderLevel(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	dueAt := now.Add(time.Hour)
	status := EvaluateSellerCommerce([]DisputeCommerceFact{{
		DisputeID: "dispute-1", OrderID: "order-1", OrderNo: "API-1", APIServiceID: "service-1",
		BuyerUserID: "buyer-1", DisputeStatus: DisputeStatusFulfillmentConfirmation, DueAt: &dueAt,
	}}, now)
	if status.Level != CommerceRestrictionNormal || status.BlockingDisputeCount != 0 || status.ActiveBuyerCount != 1 {
		t.Fatalf("unexpected commerce status: %#v", status)
	}
	if status.BlocksService("service-1") {
		t.Fatal("single dispute waiting for buyer confirmation must not block the service")
	}
}

func TestEvaluateSellerCommerceUsesDistinctBuyersPerService(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	facts := []DisputeCommerceFact{
		{DisputeID: "d1", OrderID: "o1", APIServiceID: "s1", BuyerUserID: "b1", DisputeStatus: DisputeStatusOpen},
		{DisputeID: "d2", OrderID: "o2", APIServiceID: "s1", BuyerUserID: "b1", DisputeStatus: DisputeStatusPendingApplicantDecision},
		{DisputeID: "d3", OrderID: "o3", APIServiceID: "s1", BuyerUserID: "b2", DisputeStatus: DisputeStatusFulfillmentConfirmation},
	}
	status := EvaluateSellerCommerce(facts, now)
	if status.Level != CommerceRestrictionService || !status.BlocksService("s1") || status.BlocksService("s2") {
		t.Fatalf("unexpected service restriction: %#v", status)
	}
	if status.ActiveBuyerCount != 2 || len(status.AffectedServiceIDs) != 1 || status.AffectedServiceIDs[0] != "s1" {
		t.Fatalf("buyers were not deduplicated: %#v", status)
	}
}

func TestEvaluateSellerCommerceEscalatesThreeBuyersAcrossServices(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	status := EvaluateSellerCommerce([]DisputeCommerceFact{
		{DisputeID: "d1", OrderID: "o1", APIServiceID: "s1", BuyerUserID: "b1", DisputeStatus: DisputeStatusOpen},
		{DisputeID: "d2", OrderID: "o2", APIServiceID: "s2", BuyerUserID: "b2", DisputeStatus: DisputeStatusFulfillmentConfirmation},
		{DisputeID: "d3", OrderID: "o3", APIServiceID: "s3", BuyerUserID: "b3", DisputeStatus: DisputeStatusPendingApplicantDecision},
	}, now)
	if status.Level != CommerceRestrictionAccount || !status.BlocksService("unrelated") || status.BlockingDisputeCount != 3 {
		t.Fatalf("unexpected account restriction: %#v", status)
	}
}

func TestEvaluateSellerCommerceSeparatesResponseAndFulfillmentOverdue(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	overdue := now.Add(-time.Second)
	response := EvaluateSellerCommerce([]DisputeCommerceFact{{
		DisputeID: "d1", OrderID: "o1", APIServiceID: "s1", BuyerUserID: "b1",
		DisputeStatus: DisputeStatusPendingSellerResponse, DueAt: &overdue,
	}}, now)
	if response.Level != CommerceRestrictionService || !response.BlocksService("s1") || response.BlocksService("s2") {
		t.Fatalf("seller response overdue must only block its service: %#v", response)
	}
	fulfillment := EvaluateSellerCommerce([]DisputeCommerceFact{{
		DisputeID: "d2", OrderID: "o2", APIServiceID: "s1", BuyerUserID: "b1",
		DisputeStatus: DisputeStatusAwaitingFulfillment, DueAt: &overdue,
	}}, now)
	if fulfillment.Level != CommerceRestrictionAccount || !fulfillment.BlocksService("s2") {
		t.Fatalf("remedy fulfillment overdue must block the account: %#v", fulfillment)
	}
}

func TestEvaluateSellerCommerceIgnoresClosedAndIncompleteFacts(t *testing.T) {
	status := EvaluateSellerCommerce([]DisputeCommerceFact{
		{DisputeID: "closed", OrderID: "o1", APIServiceID: "s1", BuyerUserID: "b1", DisputeStatus: DisputeStatusClosed},
		{DisputeID: "missing-order", APIServiceID: "s1", BuyerUserID: "b2", DisputeStatus: DisputeStatusOpen},
	}, time.Now())
	if status.Level != CommerceRestrictionNormal || status.ActiveDisputeCount != 0 {
		t.Fatalf("inactive or incomplete facts must not count: %#v", status)
	}
}
