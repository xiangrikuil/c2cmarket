package server

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/apipromotion"
	"c2c-market/backend/internal/module/idempotency"
)

func TestPublicAPIPromotionResponseDoesNotExposeAdminFacts(t *testing.T) {
	now := time.Date(2026, 8, 2, 4, 0, 0, 0, time.UTC)
	stoppedAt := now.Add(time.Hour)
	responses := toPublicAPIPromotionResponses([]apipromotion.Promotion{{
		ID:               "10000000-0000-4000-8000-000000000001",
		Placement:        apipromotion.PlacementAPIMarketTop,
		StartsAt:         now,
		EndsAt:           now.Add(7 * 24 * time.Hour),
		CreatedReason:    "内部运营理由",
		CreatedByAdminID: "20000000-0000-4000-8000-000000000001",
		StoppedAt:        &stoppedAt,
		StoppedReason:    "内部停止理由",
		Service: apimarket.Service{
			ID:        "30000000-0000-4000-8000-000000000001",
			Title:     "公开服务",
			CreatedAt: now,
			UpdatedAt: now,
		},
	}})
	payload, err := json.Marshal(responses)
	if err != nil {
		t.Fatalf("marshal public promotion response: %v", err)
	}
	text := string(payload)
	for _, forbidden := range []string{"createdReason", "createdByAdminId", "stoppedReason", "内部运营理由", "内部停止理由"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("public promotion response exposed %q: %s", forbidden, text)
		}
	}
	if !strings.Contains(text, `"label":"推广"`) {
		t.Fatalf("public promotion response omitted disclosure label: %s", text)
	}
}

func TestAPIPromotionAvailabilityResponsePreservesDecisionFacts(t *testing.T) {
	response := toAPIPromotionAvailabilityResponse(apipromotion.Availability{
		Eligibility: apipromotion.Eligibility{
			Configurable:       true,
			Displayable:        false,
			WarningReasons:     []string{"需要人工复核。"},
			SuppressionReasons: []string{"服务当前暂停接单。"},
		},
		OverlappingCampaigns: 1,
		Capacity:             3,
		RemainingCapacity:    2,
		SameServiceOverlap:   true,
	})

	if !response.Eligibility.Configurable || response.Eligibility.Displayable {
		t.Fatalf("eligibility flags changed in HTTP projection: %#v", response)
	}
	if response.OverlappingCampaigns != 1 || response.Capacity != 3 || response.RemainingCapacity != 2 || !response.SameServiceOverlap {
		t.Fatalf("capacity facts changed in HTTP projection: %#v", response)
	}
	if len(response.Eligibility.WarningReasons) != 1 || len(response.Eligibility.SuppressionReasons) != 1 {
		t.Fatalf("eligibility reasons changed in HTTP projection: %#v", response)
	}
}

func TestRestoreAPIPromotionETagFromCachedResponse(t *testing.T) {
	completion := idempotency.Completion{Body: []byte(`{"version":4}`)}
	restoreAPIPromotionETag(&completion)
	if completion.Headers["ETag"] != `"4"` {
		t.Fatalf("expected replay ETag to be restored, got %#v", completion.Headers)
	}
}
