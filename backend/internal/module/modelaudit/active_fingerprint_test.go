package modelaudit

import "testing"

func TestActiveFingerprintScoreSeparatesFeatureDrift(t *testing.T) {
	baseline := []ActiveFingerprintFeature{
		{PromptID: "active_json_strict_v1", LengthChars: 24, FormatCompliance: 1, NumericValue: 42, CategoricalValue: "json"},
		{PromptID: "active_refusal_boundary_v1", LengthChars: 34, FormatCompliance: 0.9, Refusal: true, CategoricalValue: "refusal"},
		{PromptID: "active_numeric_precision_v1", LengthChars: 3, FormatCompliance: 1, NumericValue: 437, CategoricalValue: "integer"},
	}
	consistent := []ActiveFingerprintFeature{
		{PromptID: "active_json_strict_v1", LengthChars: 25, FormatCompliance: 0.98, NumericValue: 42, CategoricalValue: "json"},
		{PromptID: "active_refusal_boundary_v1", LengthChars: 35, FormatCompliance: 0.85, Refusal: true, CategoricalValue: "refusal"},
		{PromptID: "active_numeric_precision_v1", LengthChars: 3, FormatCompliance: 1, NumericValue: 437, CategoricalValue: "integer"},
	}
	drifted := []ActiveFingerprintFeature{
		{PromptID: "active_json_strict_v1", LengthChars: 120, FormatCompliance: 0.1, NumericValue: 0, CategoricalValue: "paragraph"},
		{PromptID: "active_refusal_boundary_v1", LengthChars: 12, FormatCompliance: 0.2, Refusal: false, CategoricalValue: "answer", CodeStyleDistance: 1},
		{PromptID: "active_numeric_precision_v1", LengthChars: 10, FormatCompliance: 0.2, NumericValue: 0, CategoricalValue: "float", CodeStyleDistance: 1},
	}

	consistentScore := ScoreActiveFingerprint(consistent, baseline)
	if consistentScore.Risk != RiskConsistent {
		t.Fatalf("expected consistent features, got %+v", consistentScore)
	}

	driftedScore := ScoreActiveFingerprint(drifted, baseline)
	if driftedScore.Risk != RiskHigh {
		t.Fatalf("expected high-risk feature drift, got %+v", driftedScore)
	}
	if driftedScore.Score <= consistentScore.Score {
		t.Fatalf("expected drifted score > consistent score, got drifted=%+v consistent=%+v", driftedScore, consistentScore)
	}
}

func TestActiveFingerprintRequiresMatchedBaseline(t *testing.T) {
	score := ScoreActiveFingerprint([]ActiveFingerprintFeature{
		{PromptID: "active_json_strict_v1", LengthChars: 24, FormatCompliance: 1},
		{PromptID: "active_numeric_precision_v1", LengthChars: 3, FormatCompliance: 1},
		{PromptID: "active_length_control_v1", LengthChars: 8, FormatCompliance: 1},
	}, []ActiveFingerprintFeature{
		{PromptID: "different_prompt", LengthChars: 24, FormatCompliance: 1},
	})

	if score.Risk != RiskInsufficientData {
		t.Fatalf("expected insufficient data for unmatched baseline, got %+v", score)
	}
}
