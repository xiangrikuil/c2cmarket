package modelaudit

import "testing"

func TestJensenShannonDistanceRanksDistributionDrift(t *testing.T) {
	same := JensenShannonDistance([]float64{0.5, 0.5}, []float64{0.5, 0.5})
	if same != 0 {
		t.Fatalf("same distributions should have zero distance, got %v", same)
	}

	similar := JensenShannonDistance([]float64{0.55, 0.45}, []float64{0.5, 0.5})
	opposite := JensenShannonDistance([]float64{1, 0}, []float64{0, 1})
	if !(opposite > similar && similar > same) {
		t.Fatalf("expected opposite > similar > same, got opposite=%v similar=%v same=%v", opposite, similar, same)
	}
}

func TestEntropyHandlesSmoothedProbabilities(t *testing.T) {
	entropy := Entropy([]float64{0.5, 0.5})
	if entropy < 0.69 || entropy > 0.70 {
		t.Fatalf("unexpected entropy for fair binary distribution: %v", entropy)
	}

	if got := Entropy([]float64{1, 0}); got != 0 {
		t.Fatalf("certain distribution should have zero entropy, got %v", got)
	}
}

func TestPermutationTestMMDDetectsDifferentTextDistributions(t *testing.T) {
	reference := []string{"alpha beta", "alpha gamma", "beta gamma", "alpha alpha"}
	target := []string{"zulu xray", "xray yankee", "zulu zulu", "yankee xray"}

	result := PermutationTestMMD(reference, target, 80)
	if result.MMD2 <= 0 {
		t.Fatalf("expected positive MMD2 for different distributions, got %+v", result)
	}
	if result.PValue >= 0.10 {
		t.Fatalf("expected low p-value for different distributions, got %+v", result)
	}

	same := PermutationTestMMD(reference, append([]string{}, reference...), 40)
	if same.MMD2 > 0.000001 {
		t.Fatalf("same distributions should be near zero, got %+v", same)
	}
}
