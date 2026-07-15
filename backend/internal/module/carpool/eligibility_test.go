package carpool

import (
	"testing"

	"c2c-market/backend/internal/module/catalog"
)

func TestEvaluateApplicationEligibilityCoversAllCodes(t *testing.T) {
	base := EligibilityContext{
		Listing: Listing{
			OwnerUserID:       "owner-1",
			AccessArrangement: "通过官方成员邀请加入，不共享密码。",
			Status:            ListingStatusActive,
			AvailableSeats:    1,
			RiskNoticeCode:    "risk-1",
			RiskAckRequired:   true,
		},
		Plan:          catalog.ProductPlan{PublishPolicy: "allowed"},
		CurrentUserID: "buyer-1",
	}

	tests := []struct {
		name   string
		mutate func(*EligibilityContext)
		code   string
		can    bool
	}{
		{name: "eligible", code: EligibilityEligible, can: true},
		{name: "sold out", mutate: func(value *EligibilityContext) { value.Listing.AvailableSeats = 0 }, code: EligibilitySoldOut},
		{name: "paused", mutate: func(value *EligibilityContext) { value.Listing.Status = ListingStatusPaused }, code: EligibilityPaused},
		{name: "credential risk", mutate: func(value *EligibilityContext) { value.Listing.AccessArrangement = "共享 password=secret" }, code: EligibilityCredentialRisk},
		{name: "owner action required", mutate: func(value *EligibilityContext) { value.Listing.AccessArrangement = "" }, code: EligibilityOwnerActionRequired},
		{name: "already applied", mutate: func(value *EligibilityContext) { value.HasOngoingApplication = true }, code: EligibilityAlreadyApplied},
		{name: "already member", mutate: func(value *EligibilityContext) { value.HasActiveMembership = true }, code: EligibilityAlreadyMember},
		{name: "self owned", mutate: func(value *EligibilityContext) { value.CurrentUserID = "owner-1" }, code: EligibilitySelfOwned},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := base
			if test.mutate != nil {
				test.mutate(&input)
			}
			got := EvaluateApplicationEligibility(input)
			if got.Code != test.code || got.CanApply != test.can || got.Reason == "" || got.ResolutionAction == "" {
				t.Fatalf("unexpected eligibility: %+v", got)
			}
		})
	}
}

func TestEvaluateApplicationEligibilityProductBoundaryPrecedesPersonalState(t *testing.T) {
	input := EligibilityContext{
		Listing: Listing{
			OwnerUserID:       "owner-1",
			AccessArrangement: "共享 token=secret",
			Status:            ListingStatusPaused,
			AvailableSeats:    0,
		},
		Plan:                  catalog.ProductPlan{PublishPolicy: "allowed"},
		CurrentUserID:         "owner-1",
		HasOngoingApplication: true,
		HasActiveMembership:   true,
	}
	got := EvaluateApplicationEligibility(input)
	if got.Code != EligibilityCredentialRisk {
		t.Fatalf("expected credential risk to remain authoritative, got %+v", got)
	}
}
