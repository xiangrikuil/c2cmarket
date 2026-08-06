package reputation

import (
	"math"
	"testing"
	"time"
)

func TestEvaluateSnapshotTierAndConfidenceBoundaries(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	rules := CurrentRules()
	tests := []struct {
		name        string
		completions int
		tier        string
		confidence  string
	}{
		{name: "zero", completions: 0, tier: TierInsufficient, confidence: ConfidenceLow},
		{name: "three", completions: 3, tier: TierNormal, confidence: ConfidenceMedium},
		{name: "nine", completions: 9, tier: TierNormal, confidence: ConfidenceMedium},
		{name: "ten", completions: 10, tier: TierReliable, confidence: ConfidenceHigh},
		{name: "thirty", completions: 30, tier: TierReliable, confidence: ConfidenceHigh},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			snapshot := EvaluateSnapshot(
				SnapshotKey{UserID: "user-1", Role: RoleSeller, Scope: ScopeOverall},
				ScopeFacts{CompletedCount: test.completions},
				nil,
				now,
				rules,
			)
			if snapshot.Tier != test.tier || snapshot.Confidence != test.confidence {
				t.Fatalf("got tier=%q confidence=%q", snapshot.Tier, snapshot.Confidence)
			}
		})
	}
}

func TestEvaluateSnapshotCancellationRateBoundaries(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	rules := CurrentRules()
	tests := []struct {
		name         string
		completions  int
		faults       int
		unknown      int
		expectedTier string
		expectedRate float64
	}{
		{name: "ten completions no fault", completions: 10, expectedTier: TierReliable, expectedRate: 0},
		{name: "ten completions one fault", completions: 10, faults: 1, expectedTier: TierNormal, expectedRate: 1.0 / 11.0},
		{name: "nineteen completions one fault is five percent", completions: 19, faults: 1, expectedTier: TierReliable, expectedRate: 0.05},
		{name: "unknown does not enter denominator", completions: 10, unknown: 20, expectedTier: TierReliable, expectedRate: 0},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			snapshot := EvaluateSnapshot(
				SnapshotKey{UserID: "user-1", Role: RoleBuyer, Scope: ScopeAPI},
				ScopeFacts{
					CompletedCount:                         test.completions,
					RoleResponsibilityCancellationCount:    test.faults,
					UnknownResponsibilityCancellationCount: test.unknown,
				},
				nil,
				now,
				rules,
			)
			if snapshot.Tier != test.expectedTier {
				t.Fatalf("expected tier %q, got %q", test.expectedTier, snapshot.Tier)
			}
			if snapshot.Metrics.RoleFaultCancelRate == nil ||
				math.Abs(*snapshot.Metrics.RoleFaultCancelRate-test.expectedRate) > 0.000001 {
				t.Fatalf("unexpected fault rate: %#v", snapshot.Metrics.RoleFaultCancelRate)
			}
		})
	}
}

func TestEvaluateSnapshotZeroDenominatorReturnsNullRates(t *testing.T) {
	t.Parallel()

	snapshot := EvaluateSnapshot(
		SnapshotKey{UserID: "user-1", Role: RoleBuyer, Scope: ScopeCarpool},
		ScopeFacts{UnknownResponsibilityCancellationCount: 2},
		nil,
		time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC),
		CurrentRules(),
	)
	if snapshot.Metrics.RoleCompletionRate != nil || snapshot.Metrics.RoleFaultCancelRate != nil {
		t.Fatalf("zero denominator must return null rates: %#v", snapshot.Metrics)
	}
	for _, item := range snapshot.Progress {
		if item.Code == "fault_cancel_rate" && item.RemainingValue != nil {
			t.Fatalf("undefined cancellation rate must not invent a completion count: %#v", item)
		}
	}
}

func TestEvaluateSnapshotFaultCancellationProgressUsesRequiredCleanCompletions(t *testing.T) {
	t.Parallel()

	snapshot := EvaluateSnapshot(
		SnapshotKey{UserID: "user-1", Role: RoleBuyer, Scope: ScopeAPI},
		ScopeFacts{
			CompletedCount:                      10,
			RoleResponsibilityCancellationCount: 1,
		},
		nil,
		time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC),
		CurrentRules(),
	)
	for _, item := range snapshot.Progress {
		if item.Code != "fault_cancel_rate" {
			continue
		}
		if item.Status != ProgressNotMet || item.RemainingValue == nil || *item.RemainingValue != 9 {
			t.Fatalf("expected nine clean completions to reach five percent, got %#v", item)
		}
		return
	}
	t.Fatal("missing responsibility cancellation progress")
}

func TestEvaluateSnapshotBayesianRatingUsesPlatformPrior(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	snapshot := EvaluateSnapshot(
		SnapshotKey{UserID: "user-1", Role: RoleSeller, Scope: ScopeOverall},
		ScopeFacts{
			CompletedCount:           30,
			CompletedCountLast90Days: 3,
			VerifiedReviewCount:      1,
			RatingSum:                5,
			PlatformReviewCount:      100,
			PlatformAverageRating:    4.2,
		},
		nil,
		now,
		CurrentRules(),
	)
	expected := (5.0 + 5.0*4.2) / 6.0
	if snapshot.Metrics.WeightedRating == nil ||
		math.Abs(*snapshot.Metrics.WeightedRating-expected) > 0.000001 {
		t.Fatalf("expected weighted rating %.6f, got %#v", expected, snapshot.Metrics.WeightedRating)
	}
	if snapshot.Tier != TierReliable {
		t.Fatalf("one five-star review must not enter high trust, got %q", snapshot.Tier)
	}

	neutral := EvaluateSnapshot(
		SnapshotKey{UserID: "user-2", Role: RoleSeller, Scope: ScopeOverall},
		ScopeFacts{VerifiedReviewCount: 1, RatingSum: 5, PlatformReviewCount: 19, PlatformAverageRating: 4.9},
		nil,
		now,
		CurrentRules(),
	)
	expectedNeutral := (5.0 + 5.0*4.0) / 6.0
	if neutral.Metrics.WeightedRating == nil ||
		math.Abs(*neutral.Metrics.WeightedRating-expectedNeutral) > 0.000001 {
		t.Fatalf("platform sample below 20 must use neutral prior: %#v", neutral.Metrics.WeightedRating)
	}
}

func TestEvaluateSnapshotRiskOverridesReliableContinuity(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	reliableSince := now.Add(-100 * 24 * time.Hour)
	previous := ReputationSnapshot{
		Tier:          TierHighTrust,
		State:         StateActive,
		RuleVersion:   RuleVersion,
		ReliableSince: &reliableSince,
	}
	facts := ScopeFacts{
		CompletedCount:           30,
		CompletedCountLast90Days: 3,
		VerifiedReviewCount:      10,
		RatingSum:                50,
		PlatformReviewCount:      20,
		PlatformAverageRating:    4,
		ActiveRestrictionCount:   1,
	}
	snapshot := EvaluateSnapshot(SnapshotKey{UserID: "user-1", Role: RoleSeller, Scope: ScopeAPI}, facts, &previous, now, CurrentRules())
	if snapshot.State != StateRestricted || snapshot.Tier != TierNormal {
		t.Fatalf("risk must override tier display inputs: tier=%q state=%q", snapshot.Tier, snapshot.State)
	}
	if snapshot.ReliableSince != nil {
		t.Fatal("restriction must reset reliable continuity")
	}
	for _, item := range snapshot.Progress {
		if item.Status != ProgressUnavailable && item.Status != ProgressBlocked {
			t.Fatalf("active progress must be blocked while restricted: %#v", item)
		}
	}
}

func TestEvaluateSnapshotSourceAuthorMismatchCausesSellerCaution(t *testing.T) {
	t.Parallel()

	snapshot := EvaluateSnapshot(
		SnapshotKey{UserID: "user-1", Role: RoleSeller, Scope: ScopeCarpool},
		ScopeFacts{
			CompletedCount:       10,
			SourceAuthorMismatch: true,
			SourceAuthorVerification: SourceAuthorAggregate{
				State:  SourceAggregateMismatch,
				Counts: SourceAuthorStatusCounts{Total: 1, Mismatch: 1},
			},
		},
		nil,
		time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC),
		CurrentRules(),
	)
	if snapshot.State != StateCaution {
		t.Fatalf("expected source mismatch caution, got %q", snapshot.State)
	}
	if len(snapshot.Warnings) != 1 || snapshot.Warnings[0] != warningSourceMismatch {
		t.Fatalf("expected source mismatch warning, got %#v", snapshot.Warnings)
	}
	if snapshot.Metrics.SourceAuthorVerification.State != SourceAggregateMismatch {
		t.Fatalf("expected mismatch aggregate, got %#v", snapshot.Metrics.SourceAuthorVerification)
	}
}

func TestEvaluateSnapshotReliableContinuityAndRecentCompletion(t *testing.T) {
	t.Parallel()

	rules := CurrentRules()
	start := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	key := SnapshotKey{UserID: "user-1", Role: RoleSeller, Scope: ScopeOverall}
	facts := ScopeFacts{
		CompletedCount:           30,
		CompletedCountLast90Days: 3,
		VerifiedReviewCount:      10,
		RatingSum:                50,
		PlatformReviewCount:      20,
		PlatformAverageRating:    4,
	}
	first := EvaluateSnapshot(key, facts, nil, start, rules)
	if first.Tier != TierReliable || first.ReliableSince == nil {
		t.Fatalf("expected reliable continuity to start: %#v", first)
	}
	atBoundary := start.Add(rules.ReliableContinuity)
	high := EvaluateSnapshot(key, facts, &first, atBoundary, rules)
	if high.Tier != TierHighTrust {
		t.Fatalf("expected high trust at continuity boundary, got %q", high.Tier)
	}

	noRecent := facts
	noRecent.CompletedCountLast90Days = 0
	waited := EvaluateSnapshot(key, noRecent, &high, atBoundary.Add(365*24*time.Hour), rules)
	if waited.Tier != TierReliable {
		t.Fatalf("waiting without recent completion must not retain high trust, got %q", waited.Tier)
	}

	interruptedFacts := facts
	interruptedFacts.UnresolvedDisputeCount = 1
	interrupted := EvaluateSnapshot(key, interruptedFacts, &high, atBoundary.Add(time.Hour), rules)
	if interrupted.ReliableSince != nil || interrupted.State != StateCaution {
		t.Fatalf("caution must interrupt continuity: %#v", interrupted)
	}
	resumedAt := atBoundary.Add(2 * time.Hour)
	resumed := EvaluateSnapshot(key, facts, &interrupted, resumedAt, rules)
	if resumed.ReliableSince == nil || !resumed.ReliableSince.Equal(resumedAt) {
		t.Fatalf("continuity must restart after interruption: %#v", resumed.ReliableSince)
	}
}

func TestEvaluateSnapshotPassiveReviewProgressCannotSolicitReviews(t *testing.T) {
	t.Parallel()

	snapshot := EvaluateSnapshot(
		SnapshotKey{UserID: "user-1", Role: RoleSeller, Scope: ScopeOverall},
		ScopeFacts{CompletedCount: 10, VerifiedReviewCount: 7, RatingSum: 33},
		nil,
		time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC),
		CurrentRules(),
	)
	for _, code := range []string{"verified_reviews", "weighted_rating"} {
		found := false
		for _, item := range snapshot.Progress {
			if item.Code != code {
				continue
			}
			found = true
			if item.Status != ProgressUnavailable || item.RemainingValue != nil ||
				item.ActionLabel != nil || item.ActionURL != nil {
				t.Fatalf("passive review evidence must not become a task: %#v", item)
			}
		}
		if !found {
			t.Fatalf("missing passive evidence %q", code)
		}
	}
}

func TestSnapshotIsValidCoversRuleDirtySourceAndTimeBoundaries(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	sourceAt := now.Add(-time.Minute)
	next := now.Add(time.Minute)
	snapshot := ReputationSnapshot{
		RuleVersion:         RuleVersion,
		SourceDataUpdatedAt: &sourceAt,
		NextRecalculationAt: &next,
	}
	facts := ScopeFacts{SourceDataUpdatedAt: &sourceAt}
	if !SnapshotIsValid(snapshot, facts, now, RuleVersion) {
		t.Fatal("expected current snapshot to be valid")
	}
	if SnapshotIsValid(snapshot, facts, next, RuleVersion) {
		t.Fatal("time boundary must invalidate at equality")
	}
	newSource := now
	if SnapshotIsValid(snapshot, ScopeFacts{SourceDataUpdatedAt: &newSource}, now, RuleVersion) {
		t.Fatal("new source facts must invalidate snapshot")
	}
	dirtyAt := now
	snapshot.DirtyAt = &dirtyAt
	if SnapshotIsValid(snapshot, facts, now, RuleVersion) {
		t.Fatal("dirty snapshot must be invalid")
	}
	snapshot.DirtyAt = nil
	if SnapshotIsValid(snapshot, facts, now, "reputation-v1") {
		t.Fatal("rule version change must invalidate snapshot")
	}
}
