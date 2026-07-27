package reputation

import (
	"math"
	"time"
)

const (
	warningUnresolvedDispute   = "unresolved_dispute"
	warningRecentFaultPattern  = "recent_fault_cancellation_pattern"
	warningActiveRestriction   = "active_restriction"
	warningUnknownCancellation = "unknown_cancellation_responsibility"
	warningSourceMismatch      = "source_author_mismatch"
)

func EvaluateSnapshot(key SnapshotKey, facts ScopeFacts, previous *ReputationSnapshot, now time.Time, rules RuleSet) ReputationSnapshot {
	metrics := calculateMetrics(key.Role, facts, rules)
	state, warnings := calculateState(facts, metrics, rules)
	confidence := calculateConfidence(metrics.CompletedCount)
	reliable := qualifiesForReliable(metrics, state, rules)
	reliableSince := calculateReliableSince(previous, reliable, now, rules)
	tier := calculateTier(metrics, state, reliable, reliableSince, now, rules)

	snapshot := ReputationSnapshot{
		UserID:              key.UserID,
		Role:                key.Role,
		Scope:               key.Scope,
		Tier:                tier,
		State:               state,
		Confidence:          confidence,
		RuleVersion:         rules.Version,
		Metrics:             metrics,
		Warnings:            warnings,
		Badges:              calculateBadges(tier),
		Progress:            calculateProgress(metrics, state, reliableSince, now, rules),
		TierEnteredAt:       now,
		ReliableSince:       reliableSince,
		StateEnteredAt:      now,
		CalculatedAt:        now,
		SourceDataUpdatedAt: cloneTime(facts.SourceDataUpdatedAt),
		NextRecalculationAt: cloneTime(facts.NextRecalculationAt),
	}
	if previous != nil {
		if previous.Tier == snapshot.Tier && !previous.TierEnteredAt.IsZero() {
			snapshot.TierEnteredAt = previous.TierEnteredAt
		}
		if previous.State == snapshot.State && !previous.StateEnteredAt.IsZero() {
			snapshot.StateEnteredAt = previous.StateEnteredAt
		}
	}
	if reliableSince != nil {
		snapshot.NextRecalculationAt = earliestFuture(
			snapshot.NextRecalculationAt,
			reliableSince.Add(rules.ReliableContinuity),
			now,
		)
	}
	return snapshot
}

func SnapshotIsValid(snapshot ReputationSnapshot, facts ScopeFacts, now time.Time, ruleVersion string) bool {
	if snapshot.RuleVersion != ruleVersion || snapshot.DirtyAt != nil {
		return false
	}
	if snapshot.NextRecalculationAt != nil && !now.Before(*snapshot.NextRecalculationAt) {
		return false
	}
	if facts.SourceDataUpdatedAt != nil {
		if snapshot.SourceDataUpdatedAt == nil || snapshot.SourceDataUpdatedAt.Before(*facts.SourceDataUpdatedAt) {
			return false
		}
	}
	return true
}

func calculateMetrics(role string, facts ScopeFacts, rules RuleSet) ReputationMetrics {
	terminalCount := facts.CompletedCount + facts.RoleResponsibilityCancellationCount
	var completionRate, faultCancelRate *float64
	if terminalCount > 0 {
		completion := float64(facts.CompletedCount) / float64(terminalCount)
		fault := float64(facts.RoleResponsibilityCancellationCount) / float64(terminalCount)
		completionRate = &completion
		faultCancelRate = &fault
	}

	var rawAverage, weighted *float64
	if facts.VerifiedReviewCount > 0 {
		raw := float64(facts.RatingSum) / float64(facts.VerifiedReviewCount)
		platformAverage := rules.NeutralPlatformAverage
		if facts.PlatformReviewCount >= rules.PlatformPriorMinimumReviews {
			platformAverage = facts.PlatformAverageRating
		}
		value := (float64(facts.VerifiedReviewCount)*raw +
			float64(rules.BayesianPriorWeight)*platformAverage) /
			float64(facts.VerifiedReviewCount+rules.BayesianPriorWeight)
		rawAverage = &raw
		weighted = &value
	}
	sourceAuthorVerification := facts.SourceAuthorVerification
	if role == RoleBuyer || !validSourceAuthorAggregateState(sourceAuthorVerification.State) {
		sourceAuthorVerification = SourceAuthorAggregateForCounts(role, sourceAuthorVerification.Counts)
	}

	return ReputationMetrics{
		CompletedCount:                         facts.CompletedCount,
		CompletedCountLast90Days:               facts.CompletedCountLast90Days,
		RoleResponsibilityCancellationCount:    facts.RoleResponsibilityCancellationCount,
		UnknownResponsibilityCancellationCount: facts.UnknownResponsibilityCancellationCount,
		RoleControllableTerminalCount:          terminalCount,
		RoleCompletionRate:                     completionRate,
		RoleFaultCancelRate:                    faultCancelRate,
		VerifiedReviewCount:                    facts.VerifiedReviewCount,
		RawAverageRating:                       rawAverage,
		WeightedRating:                         weighted,
		RatingDistribution:                     facts.RatingDistribution,
		RecentReviewCount90Days:                facts.RecentReviewCount90d,
		CommonPositiveTags:                     append([]ReputationTagCount{}, facts.CommonPositiveTags...),
		CommonNegativeTags:                     append([]ReputationTagCount{}, facts.CommonNegativeTags...),
		ConfirmedFaultDisputeCount365Days:      facts.ConfirmedFaultDisputeCount365d,
		ConfirmedMajorFaultDisputeCount365Days: facts.ConfirmedMajorFaultDisputeCount365d,
		UnresolvedDisputeCount:                 facts.UnresolvedDisputeCount,
		ActiveRestrictionCount:                 facts.ActiveRestrictionCount,
		SourceAuthorVerification:               sourceAuthorVerification,
	}
}

func validSourceAuthorAggregateState(value string) bool {
	switch value {
	case SourceAggregateNotApplicable,
		SourceAggregateNoSources,
		SourceAggregatePending,
		SourceAggregatePartial,
		SourceAggregateVerified,
		SourceAggregateMismatch:
		return true
	default:
		return false
	}
}

func calculateState(facts ScopeFacts, metrics ReputationMetrics, rules RuleSet) (string, []string) {
	warnings := make([]string, 0, 5)
	if facts.UnknownResponsibilityCancellationCount > 0 {
		warnings = append(warnings, warningUnknownCancellation)
	}
	if facts.ActiveRestrictionCount > 0 {
		warnings = append(warnings, warningActiveRestriction)
		return StateRestricted, warnings
	}
	caution := false
	if facts.UnresolvedDisputeCount > 0 {
		caution = true
		warnings = append(warnings, warningUnresolvedDispute)
	}
	if facts.RoleResponsibilityCancellationCount90d >= rules.CautionRecentFaultCount &&
		metrics.RoleFaultCancelRate != nil &&
		*metrics.RoleFaultCancelRate > rules.CautionFaultCancelRate {
		caution = true
		warnings = append(warnings, warningRecentFaultPattern)
	}
	if facts.SourceAuthorMismatch {
		caution = true
		warnings = append(warnings, warningSourceMismatch)
	}
	if caution {
		return StateCaution, warnings
	}
	return StateActive, warnings
}

func calculateConfidence(completedCount int) string {
	switch {
	case completedCount < 3:
		return ConfidenceLow
	case completedCount < 10:
		return ConfidenceMedium
	default:
		return ConfidenceHigh
	}
}

func qualifiesForReliable(metrics ReputationMetrics, state string, rules RuleSet) bool {
	return state == StateActive &&
		metrics.CompletedCount >= rules.MinimumReliableCompletions &&
		metrics.RoleCompletionRate != nil &&
		*metrics.RoleCompletionRate >= rules.MinimumReliableCompletionRate &&
		metrics.RoleFaultCancelRate != nil &&
		*metrics.RoleFaultCancelRate <= rules.MaximumReliableFaultCancelRate &&
		metrics.UnresolvedDisputeCount == 0 &&
		metrics.ConfirmedMajorFaultDisputeCount365Days == 0
}

func calculateReliableSince(previous *ReputationSnapshot, reliable bool, now time.Time, rules RuleSet) *time.Time {
	if !reliable {
		return nil
	}
	if previous != nil &&
		previous.RuleVersion == rules.Version &&
		previous.State == StateActive &&
		previous.ReliableSince != nil {
		return cloneTime(previous.ReliableSince)
	}
	value := now
	return &value
}

func calculateTier(metrics ReputationMetrics, state string, reliable bool, reliableSince *time.Time, now time.Time, rules RuleSet) string {
	if metrics.CompletedCount < rules.MinimumNormalCompletions {
		return TierInsufficient
	}
	if !reliable {
		return TierNormal
	}
	if state == StateActive &&
		reliableSince != nil &&
		!now.Before(reliableSince.Add(rules.ReliableContinuity)) &&
		metrics.CompletedCount >= rules.MinimumHighTrustCompletions &&
		metrics.CompletedCountLast90Days >= rules.MinimumRecentHighTrustCompletions &&
		metrics.VerifiedReviewCount >= rules.MinimumHighTrustReviews &&
		metrics.WeightedRating != nil &&
		*metrics.WeightedRating >= rules.MinimumHighTrustWeightedRating {
		return TierHighTrust
	}
	return TierReliable
}

func calculateBadges(tier string) []string {
	switch tier {
	case TierHighTrust:
		return []string{"high_trust"}
	case TierReliable:
		return []string{"reliable"}
	default:
		return []string{}
	}
}

func calculateProgress(metrics ReputationMetrics, state string, reliableSince *time.Time, now time.Time, rules RuleSet) []ReputationProgressItem {
	targetCompletions := rules.MinimumNormalCompletions
	if metrics.CompletedCount >= rules.MinimumNormalCompletions {
		targetCompletions = rules.MinimumReliableCompletions
	}
	if metrics.CompletedCount >= rules.MinimumReliableCompletions {
		targetCompletions = rules.MinimumHighTrustCompletions
	}

	progress := []ReputationProgressItem{
		numberProgress("completed_count", "可验证完成", float64(metrics.CompletedCount), float64(targetCompletions), false),
		rateProgress("completion_rate", "完成率", metrics.RoleCompletionRate, rules.MinimumReliableCompletionRate, true),
		faultCancelRateProgress(metrics, rules.MaximumReliableFaultCancelRate),
		passiveEvidenceProgress("verified_reviews", "已验证评价", float64(metrics.VerifiedReviewCount), float64(rules.MinimumHighTrustReviews)),
		passiveEvidenceProgress("weighted_rating", "修正评分", pointerValue(metrics.WeightedRating), rules.MinimumHighTrustWeightedRating),
	}
	if state != StateActive {
		for index := range progress {
			if progress[index].Status != ProgressUnavailable {
				progress[index].Status = ProgressBlocked
			}
		}
	}
	if reliableSince != nil {
		requiredAt := reliableSince.Add(rules.ReliableContinuity)
		currentDays := now.Sub(*reliableSince).Hours() / 24
		if currentDays < 0 {
			currentDays = 0
		}
		item := numberProgress("reliable_continuity_days", "持续较可靠", currentDays, float64(rules.ReliableContinuityDays), false)
		if !now.Before(requiredAt) {
			item.Status = ProgressMet
		}
		progress = append(progress, item)
	}
	return progress
}

func numberProgress(code, label string, current, required float64, lowerIsBetter bool) ReputationProgressItem {
	status := ProgressNotMet
	remaining := math.Max(0, required-current)
	if lowerIsBetter {
		remaining = math.Max(0, current-required)
		if current <= required {
			status = ProgressMet
		}
	} else if current >= required {
		status = ProgressMet
	}
	return ReputationProgressItem{
		Code:           code,
		Label:          label,
		Status:         status,
		CurrentValue:   floatPointer(current),
		RequiredValue:  floatPointer(required),
		RemainingValue: floatPointer(remaining),
	}
}

func rateProgress(code, label string, current *float64, required float64, higherIsBetter bool) ReputationProgressItem {
	if current == nil {
		return ReputationProgressItem{
			Code:          code,
			Label:         label,
			Status:        ProgressNotMet,
			CurrentValue:  nil,
			RequiredValue: floatPointer(required),
		}
	}
	return numberProgress(code, label, *current, required, !higherIsBetter)
}

func faultCancelRateProgress(metrics ReputationMetrics, maximumRate float64) ReputationProgressItem {
	item := ReputationProgressItem{
		Code:          "fault_cancel_rate",
		Label:         "责任取消率",
		Status:        ProgressNotMet,
		CurrentValue:  cloneFloat(metrics.RoleFaultCancelRate),
		RequiredValue: floatPointer(maximumRate),
	}
	if metrics.RoleFaultCancelRate == nil || maximumRate <= 0 {
		return item
	}
	if *metrics.RoleFaultCancelRate <= maximumRate {
		item.Status = ProgressMet
		item.RemainingValue = floatPointer(0)
		return item
	}
	requiredCompletions := math.Ceil(
		float64(metrics.RoleResponsibilityCancellationCount)/maximumRate -
			float64(metrics.RoleControllableTerminalCount),
	)
	if requiredCompletions < 0 {
		requiredCompletions = 0
	}
	item.RemainingValue = floatPointer(requiredCompletions)
	return item
}

func passiveEvidenceProgress(code, label string, current, required float64) ReputationProgressItem {
	return ReputationProgressItem{
		Code:          code,
		Label:         label,
		Status:        ProgressUnavailable,
		CurrentValue:  floatPointer(current),
		RequiredValue: floatPointer(required),
	}
}

func pointerValue(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func floatPointer(value float64) *float64 {
	return &value
}

func cloneFloat(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func earliestFuture(current *time.Time, candidate, now time.Time) *time.Time {
	if !candidate.After(now) {
		return current
	}
	if current == nil || candidate.Before(*current) {
		value := candidate
		return &value
	}
	return current
}
