package modelaudit

import "sort"

type RandomFingerprintFeatureSet struct {
	Categorical []CategoricalFingerprintFeature
	Binary      []BinarySequenceFeature
}

type CategoricalFingerprintFeature struct {
	PromptID    string
	N           int
	Counts      map[string]int
	Values      []string
	InvalidRate float64
}

type BinarySequenceFeature struct {
	PromptID    string
	NSequences  int
	TotalBits   int
	POne        float64
	PZero       float64
	BigramProbs map[string]float64
	InvalidRate float64
}

func ScoreRandomFingerprint(baseline, target RandomFingerprintFeatureSet) ProbeScore {
	sampleCount := randomSampleCount(target)
	if sampleCount < 40 {
		return ProbeScore{
			Probe:      ProbeRandomFingerprint,
			Risk:       RiskInsufficientData,
			Confidence: clamp01(float64(sampleCount) / 40),
			Score:      0,
			Evidence:   map[string]any{"sample_count": sampleCount},
		}
	}

	distances := []float64{}
	evidence := map[string]any{"sample_count": sampleCount}
	baselineByPrompt := mapCategoricalFeatures(baseline.Categorical)
	for _, targetFeature := range target.Categorical {
		baselineFeature, ok := baselineByPrompt[targetFeature.PromptID]
		if !ok {
			continue
		}
		values := mergedValues(baselineFeature, targetFeature)
		baselineProb := categoricalProbabilities(baselineFeature, values)
		targetProb := categoricalProbabilities(targetFeature, values)
		distance := JensenShannonDistance(baselineProb, targetProb)
		distances = append(distances, distance)
		evidence[targetFeature.PromptID] = map[string]any{
			"js_distance":        distance,
			"baseline_mode":      categoricalMode(baselineFeature),
			"target_mode":        categoricalMode(targetFeature),
			"target_invalid_rate": targetFeature.InvalidRate,
		}
	}
	binaryByPrompt := mapBinaryFeatures(baseline.Binary)
	for _, targetFeature := range target.Binary {
		baselineFeature, ok := binaryByPrompt[targetFeature.PromptID]
		if !ok {
			continue
		}
		bitDistance := JensenShannonDistance([]float64{baselineFeature.PZero, baselineFeature.POne}, []float64{targetFeature.PZero, targetFeature.POne})
		distances = append(distances, bitDistance)
		evidence[targetFeature.PromptID] = map[string]any{
			"bit_js_distance": bitDistance,
			"baseline_p_one":  baselineFeature.POne,
			"target_p_one":    targetFeature.POne,
		}
	}
	if len(distances) == 0 {
		return ProbeScore{
			Probe:      ProbeRandomFingerprint,
			Risk:       RiskInsufficientData,
			Confidence: 0.2,
			Score:      0,
			Evidence:   evidence,
		}
	}
	distance := mean(distances)
	score := clamp01(distance / 0.35)
	risk := RiskConsistent
	if distance >= 0.25 {
		risk = RiskHigh
	} else if distance >= 0.12 {
		risk = RiskSuspicious
	}
	evidence["distance"] = distance
	return ProbeScore{
		Probe:      ProbeRandomFingerprint,
		Risk:       risk,
		Confidence: clamp01(float64(sampleCount) / 160),
		Score:      score,
		Evidence:   evidence,
	}
}

func randomSampleCount(featureSet RandomFingerprintFeatureSet) int {
	total := 0
	for _, feature := range featureSet.Categorical {
		total += feature.N
	}
	for _, feature := range featureSet.Binary {
		total += feature.NSequences
	}
	return total
}

func mapCategoricalFeatures(features []CategoricalFingerprintFeature) map[string]CategoricalFingerprintFeature {
	out := map[string]CategoricalFingerprintFeature{}
	for _, feature := range features {
		out[feature.PromptID] = feature
	}
	return out
}

func mapBinaryFeatures(features []BinarySequenceFeature) map[string]BinarySequenceFeature {
	out := map[string]BinarySequenceFeature{}
	for _, feature := range features {
		out[feature.PromptID] = feature
	}
	return out
}

func mergedValues(a, b CategoricalFingerprintFeature) []string {
	seen := map[string]bool{}
	for _, value := range a.Values {
		seen[value] = true
	}
	for _, value := range b.Values {
		seen[value] = true
	}
	for value := range a.Counts {
		seen[value] = true
	}
	for value := range b.Counts {
		seen[value] = true
	}
	values := make([]string, 0, len(seen))
	for value := range seen {
		values = append(values, value)
	}
	sort.Strings(values)
	return values
}

func categoricalProbabilities(feature CategoricalFingerprintFeature, values []string) []float64 {
	alpha := 0.5
	total := 0
	for _, count := range feature.Counts {
		total += count
	}
	out := make([]float64, 0, len(values))
	denominator := float64(total) + alpha*float64(len(values))
	if denominator == 0 {
		denominator = 1
	}
	for _, value := range values {
		out = append(out, (float64(feature.Counts[value])+alpha)/denominator)
	}
	return out
}

func categoricalMode(feature CategoricalFingerprintFeature) string {
	mode := ""
	modeCount := -1
	for value, count := range feature.Counts {
		if count > modeCount || (count == modeCount && value < mode) {
			mode = value
			modeCount = count
		}
	}
	return mode
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	return sum / float64(len(values))
}
