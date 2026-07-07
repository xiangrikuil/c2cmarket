package modelaudit

import "testing"

func TestRandomFingerprintRiskSeparatesDigitPreference(t *testing.T) {
	baseline := RandomFingerprintFeatureSet{
		Categorical: []CategoricalFingerprintFeature{{
			PromptID: "rand_digit_1_10_v1",
			N:        80,
			Counts:   map[string]int{"7": 72, "4": 8},
			Values:   []string{"4", "7"},
		}},
	}
	target := RandomFingerprintFeatureSet{
		Categorical: []CategoricalFingerprintFeature{{
			PromptID: "rand_digit_1_10_v1",
			N:        80,
			Counts:   map[string]int{"7": 7, "4": 73},
			Values:   []string{"4", "7"},
		}},
	}

	score := ScoreRandomFingerprint(baseline, target)
	if score.Risk != RiskHigh {
		t.Fatalf("expected high risk for strong preference reversal, got %+v", score)
	}
	if score.Score < 0.55 {
		t.Fatalf("expected high normalized score, got %+v", score)
	}
}

func TestRandomFingerprintRiskRequiresEnoughSamples(t *testing.T) {
	score := ScoreRandomFingerprint(RandomFingerprintFeatureSet{}, RandomFingerprintFeatureSet{
		Categorical: []CategoricalFingerprintFeature{{
			PromptID: "rand_digit_1_10_v1",
			N:        12,
			Counts:   map[string]int{"7": 12},
			Values:   []string{"7"},
		}},
	})

	if score.Risk != RiskInsufficientData {
		t.Fatalf("expected insufficient data, got %+v", score)
	}
}
