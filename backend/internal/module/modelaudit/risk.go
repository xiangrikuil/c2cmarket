package modelaudit

type RiskAggregator struct {
	weights map[ProbeName]float64
}

func DefaultRiskAggregator() RiskAggregator {
	return RiskAggregator{weights: map[ProbeName]float64{
		ProbeRandomFingerprint: 0.20,
		ProbeActiveFingerprint: 0.20,
		ProbeKBF:               0.25,
		ProbeModelEquality:     0.15,
		ProbeLogprobTracking:   0.08,
		ProbeBorderInputChange: 0.07,
		ProbeBillingLatency:    0.05,
	}}
}

func (a RiskAggregator) Aggregate(scores []ProbeScore) AggregatedRisk {
	if len(scores) == 0 {
		return AggregatedRisk{Risk: RiskInsufficientData}
	}
	if hasHighRisk(scores, ProbeKBF) && hasHighRisk(scores, ProbeActiveFingerprint) {
		return AggregatedRisk{Risk: RiskHigh, Score: maxScore(scores), Confidence: meanConfidence(scores)}
	}
	weightedScore := 0.0
	weightedConfidence := 0.0
	totalWeight := 0.0
	for _, score := range scores {
		if score.Risk == RiskNotApplicable {
			continue
		}
		weight := a.weights[score.Probe]
		if weight == 0 {
			weight = 0.05
		}
		confidence := clamp01(score.Confidence)
		if score.Risk == RiskInsufficientData {
			confidence *= 0.35
		}
		weightedScore += clamp01(score.Score) * weight * confidence
		weightedConfidence += weight * confidence
		totalWeight += weight
	}
	if totalWeight == 0 || weightedConfidence == 0 {
		return AggregatedRisk{Risk: RiskInsufficientData}
	}
	overallScore := weightedScore / weightedConfidence
	overallConfidence := clamp01(weightedConfidence / totalWeight)
	risk := RiskConsistent
	if overallConfidence < 0.35 {
		risk = RiskInsufficientData
	} else if overallScore >= 0.55 {
		risk = RiskHigh
	} else if overallScore >= 0.25 {
		risk = RiskSuspicious
	}
	return AggregatedRisk{
		Risk:       risk,
		Score:      overallScore,
		Confidence: overallConfidence,
	}
}

func hasHighRisk(scores []ProbeScore, probe ProbeName) bool {
	for _, score := range scores {
		if score.Probe == probe && score.Risk == RiskHigh && score.Confidence >= 0.5 {
			return true
		}
	}
	return false
}

func maxScore(scores []ProbeScore) float64 {
	max := 0.0
	for _, score := range scores {
		if score.Score > max {
			max = score.Score
		}
	}
	return clamp01(max)
}

func meanConfidence(scores []ProbeScore) float64 {
	values := make([]float64, 0, len(scores))
	for _, score := range scores {
		if score.Risk != RiskNotApplicable {
			values = append(values, score.Confidence)
		}
	}
	return clamp01(mean(values))
}
