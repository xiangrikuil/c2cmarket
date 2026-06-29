package officialprice

import "testing"

func TestMarkLowestReferencesIgnoresDeprecatedGroupingFields(t *testing.T) {
	seatCount := 6
	commitmentMonths := 12
	records := []Record{
		{
			ID:                   "expensive",
			ProductPlanID:        "plan-chatgpt-pro",
			RegionCode:           "ph",
			Channel:              "web",
			OpeningMethod:        "official_web",
			Status:               RecordStatusActive,
			BillingPeriod:        "monthly",
			CommitmentMonths:     &commitmentMonths,
			PriceUnit:            "per_account",
			SeatCount:            &seatCount,
			Quantity:             9,
			TaxIncluded:          true,
			NormalizedMonthlyCNY: "120.00",
		},
		{
			ID:                   "cheap",
			ProductPlanID:        "plan-chatgpt-pro",
			RegionCode:           "ph",
			Channel:              "web",
			OpeningMethod:        "official_web",
			Status:               RecordStatusActive,
			BillingPeriod:        "monthly",
			PriceUnit:            "per_account",
			Quantity:             1,
			TaxIncluded:          true,
			NormalizedMonthlyCNY: "95.00",
		},
		{
			ID:                   "other-region",
			ProductPlanID:        "plan-chatgpt-pro",
			RegionCode:           "hk",
			Channel:              "web",
			OpeningMethod:        "official_web",
			Status:               RecordStatusActive,
			BillingPeriod:        "monthly",
			PriceUnit:            "per_account",
			Quantity:             1,
			TaxIncluded:          true,
			NormalizedMonthlyCNY: "110.00",
		},
	}

	markLowestReferences(records)

	if records[0].IsLowestReference {
		t.Fatalf("expensive record with deprecated grouping fields must not be lowest reference")
	}
	if !records[1].IsLowestReference {
		t.Fatalf("cheap same-group record must be lowest reference")
	}
	if !records[2].IsLowestReference {
		t.Fatalf("record in another public reference group must be lowest reference for its group")
	}
}
