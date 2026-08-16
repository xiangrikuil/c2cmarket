package server

import (
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/module/report"
)

func TestSelfSupplementProjectionExposesCapabilityWithoutInternalModerationData(t *testing.T) {
	now := time.Date(2026, 8, 3, 9, 0, 0, 0, time.UTC)
	userID := "20000000-0000-4000-8000-000000000001"
	item := report.Report{
		ID: "40000000-0000-4000-8000-000000000001", ReporterUserID: userID,
		Status: report.ReportStatusNeedsInfo, OpenInfoRequestID: "50000000-0000-4000-8000-000000000001",
		InfoRequestedFromID: userID, AdminReason: "internal-only reason", HandledByAdminID: "10000000-0000-4000-8000-000000000001",
		Supplements: []report.InfoSupplement{{ID: "60000000-0000-4000-8000-000000000001", Body: "admin-only supplement body", SubmittedByUserID: userID, CreatedAt: now}},
		CreatedAt:   now, UpdatedAt: now, Version: 2,
	}
	response := toMyReportResponse(item, userID)
	if response.CanSupplement == nil || !*response.CanSupplement || response.OpenInfoRequestID != item.OpenInfoRequestID {
		t.Fatalf("self capability projection missing: %+v", response)
	}
	completion, appErr := supplementCompletionBuilder(userID)(report.MutationResult{Report: &item})
	if appErr != nil {
		t.Fatalf("build supplement completion: %v", appErr)
	}
	body := string(completion.Body)
	for _, forbidden := range []string{"internal-only reason", "admin-only supplement body", "supplements", "adminReason", "handledByAdminId", "requestedFromUserId", "actorUserId", "requestId"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("self response leaked %q: %s", forbidden, body)
		}
	}
}

func TestAdminDetailProjectionIncludesSubmittedSupplements(t *testing.T) {
	now := time.Date(2026, 8, 3, 9, 30, 0, 0, time.UTC)
	item := report.DisputeCase{
		ID:                         "40000000-0000-4000-8000-000000000001",
		PlatformInterventionReason: "卖家拒绝申请，请平台核对交付事实。",
		Supplements: []report.InfoSupplement{{
			ID: "60000000-0000-4000-8000-000000000001", InfoRequestID: "50000000-0000-4000-8000-000000000001",
			SubmittedByUserID: "30000000-0000-4000-8000-000000000001", SubmittedByUsername: "buyer", SubmittedByName: "Buyer",
			Body: "订单状态与付款记录时间不一致，请复核。", CreatedAt: now,
		}},
	}
	response := toDisputeResponse(item, true)
	if len(response.Supplements) != 1 {
		t.Fatalf("admin supplement projection missing: %+v", response)
	}
	if response.PlatformInterventionReason != item.PlatformInterventionReason {
		t.Fatalf("admin platform intervention reason missing: %+v", response)
	}
	supplement := response.Supplements[0]
	if supplement.Body != item.Supplements[0].Body || supplement.SubmittedByUserID != item.Supplements[0].SubmittedByUserID || supplement.CreatedAt != now.Format(time.RFC3339) {
		t.Fatalf("admin supplement projection incomplete: %+v", supplement)
	}
	self := toMyDisputeResponse(item, item.Supplements[0].SubmittedByUserID)
	if len(self.Supplements) != 0 {
		t.Fatalf("self projection leaked admin supplements: %+v", self.Supplements)
	}
}

func TestSelfDisputeSupplementCapabilityIsLimitedToDesignatedParticipant(t *testing.T) {
	item := report.DisputeCase{
		ID: "40000000-0000-4000-8000-000000000001", Status: report.DisputeStatusWaitingInfo,
		OpenInfoRequestID: "50000000-0000-4000-8000-000000000001", InfoRequestedFromID: "30000000-0000-4000-8000-000000000001",
	}
	designated := toMyDisputeResponse(item, item.InfoRequestedFromID)
	other := toMyDisputeResponse(item, "20000000-0000-4000-8000-000000000001")
	if designated.CanSupplement == nil || !*designated.CanSupplement || designated.OpenInfoRequestID == "" {
		t.Fatalf("designated participant capability missing: %+v", designated)
	}
	if other.CanSupplement == nil || *other.CanSupplement || other.OpenInfoRequestID != "" {
		t.Fatalf("non-designated participant received supplement capability: %+v", other)
	}
}
