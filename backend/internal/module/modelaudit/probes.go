package modelaudit

import "context"

type RandomPrompt struct {
	ID            string
	Instruction   string
	AllowedValues []string
}

type ActivePrompt struct {
	ID          string
	Instruction string
	FeatureKind string
}

func RandomPromptBank() []RandomPrompt {
	return []RandomPrompt{
		{
			ID:            "rand_digit_1_10_v1",
			Instruction:   "请只输出一个 1 到 10 之间的随机整数，不要解释。",
			AllowedValues: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"},
		},
		{
			ID:            "rand_digit_0_9_v1",
			Instruction:   "Randomly choose one digit from 0 to 9. Output only the digit.",
			AllowedValues: []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"},
		},
	}
}

func ActiveFingerprintPromptBank() []ActivePrompt {
	return []ActivePrompt{
		{ID: "active_json_strict_v1", Instruction: "Return exactly this JSON shape with no extra text: {\"answer\":42,\"ok\":true}", FeatureKind: "format"},
		{ID: "active_refusal_boundary_v1", Instruction: "Briefly answer whether you can provide private API keys from memory. Use one sentence.", FeatureKind: "refusal"},
		{ID: "active_numeric_precision_v1", Instruction: "Compute 19 * 23 and output only the integer.", FeatureKind: "numeric"},
		{ID: "active_code_style_v1", Instruction: "Write a minimal Python function named add_one that returns x + 1.", FeatureKind: "code_style"},
		{ID: "active_categorization_v1", Instruction: "Classify the word 'invoice' as noun, verb, adjective, or other. Output one word.", FeatureKind: "category"},
		{ID: "active_length_control_v1", Instruction: "Summarize HTTP 429 in exactly eight Chinese characters.", FeatureKind: "length"},
	}
}

func RunActiveFingerprintProbe(targetFeatures, baselineFeatures []ActiveFingerprintFeature) ProbeScore {
	return ScoreActiveFingerprint(targetFeatures, baselineFeatures)
}

func ScoreActiveFingerprint(targetFeatures, baselineFeatures []ActiveFingerprintFeature) ProbeScore {
	if len(targetFeatures) < 3 || len(baselineFeatures) == 0 {
		return ProbeScore{
			Probe:      ProbeActiveFingerprint,
			Risk:       RiskInsufficientData,
			Confidence: 0.1,
			Score:      0,
			Evidence:   map[string]any{"reason": "missing_active_fingerprint_baseline"},
		}
	}
	baselineByPrompt := mapActiveFeatures(baselineFeatures)
	distances := []float64{}
	evidence := map[string]any{"target_prompt_count": len(targetFeatures), "baseline_prompt_count": len(baselineFeatures)}
	for _, target := range targetFeatures {
		baseline, ok := baselineByPrompt[target.PromptID]
		if !ok {
			continue
		}
		distance := activeFeatureDistance(target, baseline)
		distances = append(distances, distance)
		evidence[target.PromptID] = map[string]any{
			"distance":          distance,
			"target_length":     target.LengthChars,
			"baseline_length":   baseline.LengthChars,
			"target_refusal":    target.Refusal,
			"baseline_refusal":  baseline.Refusal,
			"target_category":   target.CategoricalValue,
			"baseline_category": baseline.CategoricalValue,
		}
	}
	if len(distances) < 3 {
		evidence["matched_prompt_count"] = len(distances)
		return ProbeScore{
			Probe:      ProbeActiveFingerprint,
			Risk:       RiskInsufficientData,
			Confidence: 0.2,
			Score:      0,
			Evidence:   evidence,
		}
	}
	distance := mean(distances)
	risk := RiskConsistent
	if distance >= 0.55 {
		risk = RiskHigh
	} else if distance >= 0.28 {
		risk = RiskSuspicious
	}
	evidence["matched_prompt_count"] = len(distances)
	evidence["mean_distance"] = distance
	return ProbeScore{
		Probe:      ProbeActiveFingerprint,
		Risk:       risk,
		Confidence: clamp01(float64(len(distances)) / float64(len(ActiveFingerprintPromptBank()))),
		Score:      clamp01(distance),
		Evidence:   evidence,
	}
}

func RunKBFProbe(baselines []KBFQuestionBaseline, answers []ParsedKBFAnswer) ProbeScore {
	if len(baselines) == 0 || len(answers) == 0 {
		return ProbeScore{
			Probe:      ProbeKBF,
			Risk:       RiskInsufficientData,
			Confidence: 0.1,
			Score:      0,
			Evidence:   map[string]any{"reason": "missing_kbf_baseline"},
		}
	}
	return ScoreKBFAnswers(baselines, answers)
}

func RunModelEqualityProbe(reference, target []string) ProbeScore {
	if len(reference) < 5 || len(target) < 5 {
		return ProbeScore{
			Probe:      ProbeModelEquality,
			Risk:       RiskInsufficientData,
			Confidence: 0.1,
			Score:      0,
			Evidence:   map[string]any{"reason": "missing_equality_samples"},
		}
	}
	result := PermutationTestMMD(reference, target, 120)
	risk := RiskConsistent
	score := 0.1
	if result.PValue < 0.01 {
		risk = RiskHigh
		score = 0.8
	} else if result.PValue < 0.10 {
		risk = RiskSuspicious
		score = 0.45
	}
	return ProbeScore{
		Probe:      ProbeModelEquality,
		Risk:       risk,
		Confidence: 0.6,
		Score:      score,
		Evidence:   map[string]any{"mmd2": result.MMD2, "p_value": result.PValue},
	}
}

func RunLogprobTrackingProbe(adapter ProviderAdapter) ProbeScore {
	if adapter == nil {
		return ProbeScore{Probe: ProbeLogprobTracking, Risk: RiskNotApplicable, Evidence: map[string]any{"reason": "provider_does_not_support_logprobs"}}
	}
	if supporter, ok := adapter.(interface{ SupportsLogprobs(context.Context) bool }); ok && !supporter.SupportsLogprobs(context.Background()) {
		return ProbeScore{Probe: ProbeLogprobTracking, Risk: RiskNotApplicable, Evidence: map[string]any{"reason": "provider_does_not_support_logprobs"}}
	}
	return ProbeScore{Probe: ProbeLogprobTracking, Risk: RiskInsufficientData, Confidence: 0.1, Evidence: map[string]any{"reason": "missing_logprob_baseline"}}
}

func RunBorderInputChangeProbe(baseline, target []CategoricalFingerprintFeature) ProbeScore {
	if len(baseline) == 0 || len(target) == 0 {
		return ProbeScore{Probe: ProbeBorderInputChange, Risk: RiskInsufficientData, Confidence: 0.1, Evidence: map[string]any{"reason": "missing_border_input_baseline"}}
	}
	score := ScoreRandomFingerprint(RandomFingerprintFeatureSet{Categorical: baseline}, RandomFingerprintFeatureSet{Categorical: target})
	score.Probe = ProbeBorderInputChange
	return score
}

type ActiveFingerprintFeature struct {
	PromptID          string
	NormalizedText    string
	ResponseHash      string
	LengthChars       int
	FormatCompliance  float64
	Refusal           bool
	CategoricalValue  string
	NumericValue      float64
	CodeStyleDistance float64
}

func mapActiveFeatures(features []ActiveFingerprintFeature) map[string]ActiveFingerprintFeature {
	out := map[string]ActiveFingerprintFeature{}
	for _, feature := range features {
		out[feature.PromptID] = feature
	}
	return out
}

func activeFeatureDistance(target, baseline ActiveFingerprintFeature) float64 {
	parts := []float64{
		clamp01(absFloat(target.FormatCompliance-baseline.FormatCompliance)) * 0.20,
		clamp01(lengthRatioDistance(target.LengthChars, baseline.LengthChars)) * 0.15,
		clamp01(absFloat(target.NumericValue-baseline.NumericValue)/(absFloat(baseline.NumericValue)+1)) * 0.20,
		clamp01(absFloat(target.CodeStyleDistance-baseline.CodeStyleDistance)) * 0.15,
	}
	if target.Refusal != baseline.Refusal {
		parts = append(parts, 0.15)
	}
	if target.CategoricalValue != "" && baseline.CategoricalValue != "" && target.CategoricalValue != baseline.CategoricalValue {
		parts = append(parts, 0.15)
	}
	if target.ResponseHash != "" && baseline.ResponseHash != "" && target.ResponseHash != baseline.ResponseHash {
		parts = append(parts, 0.05)
	}
	sum := 0.0
	for _, part := range parts {
		sum += part
	}
	return clamp01(sum)
}

func lengthRatioDistance(a, b int) float64 {
	if a <= 0 && b <= 0 {
		return 0
	}
	larger := a
	smaller := b
	if b > a {
		larger = b
		smaller = a
	}
	if larger <= 0 {
		return 0
	}
	return float64(larger-smaller) / float64(larger)
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}
