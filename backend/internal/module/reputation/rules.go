package reputation

import "time"

const RuleVersion = "reputation-v2"

type RuleSet struct {
	Version                           string        `json:"version"`
	MinimumNormalCompletions          int           `json:"minimumNormalCompletions"`
	MinimumReliableCompletions        int           `json:"minimumReliableCompletions"`
	MinimumHighTrustCompletions       int           `json:"minimumHighTrustCompletions"`
	MinimumRecentHighTrustCompletions int           `json:"minimumRecentHighTrustCompletions"`
	MinimumHighTrustReviews           int           `json:"minimumHighTrustReviews"`
	MinimumReliableCompletionRate     float64       `json:"minimumReliableCompletionRate"`
	MaximumReliableFaultCancelRate    float64       `json:"maximumReliableFaultCancelRate"`
	MinimumHighTrustWeightedRating    float64       `json:"minimumHighTrustWeightedRating"`
	ReliableContinuity                time.Duration `json:"-"`
	ReliableContinuityDays            int           `json:"reliableContinuityDays"`
	BayesianPriorWeight               int           `json:"bayesianPriorWeight"`
	PlatformPriorMinimumReviews       int           `json:"platformPriorMinimumReviews"`
	NeutralPlatformAverage            float64       `json:"neutralPlatformAverage"`
	CautionRecentFaultCount           int           `json:"cautionRecentFaultCount"`
	CautionFaultCancelRate            float64       `json:"cautionFaultCancelRate"`
}

func CurrentRules() RuleSet {
	return RuleSet{
		Version:                           RuleVersion,
		MinimumNormalCompletions:          3,
		MinimumReliableCompletions:        10,
		MinimumHighTrustCompletions:       30,
		MinimumRecentHighTrustCompletions: 3,
		MinimumHighTrustReviews:           10,
		MinimumReliableCompletionRate:     0.95,
		MaximumReliableFaultCancelRate:    0.05,
		MinimumHighTrustWeightedRating:    4.6,
		ReliableContinuity:                90 * 24 * time.Hour,
		ReliableContinuityDays:            90,
		BayesianPriorWeight:               5,
		PlatformPriorMinimumReviews:       20,
		NeutralPlatformAverage:            4.0,
		CautionRecentFaultCount:           3,
		CautionFaultCancelRate:            0.20,
	}
}
