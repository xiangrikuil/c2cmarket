package modelaudit

import (
	"math"
	"math/rand"
	"strings"
)

type MMDResult struct {
	MMD2   float64 `json:"mmd2"`
	PValue float64 `json:"pValue"`
}

func JensenShannonDistance(p, q []float64) float64 {
	n := len(p)
	if len(q) < n {
		n = len(q)
	}
	if n == 0 {
		return 0
	}
	pn := normalizeDistribution(p[:n])
	qn := normalizeDistribution(q[:n])
	m := make([]float64, n)
	for i := 0; i < n; i++ {
		m[i] = (pn[i] + qn[i]) / 2
	}
	jsd := 0.5*klDivergence(pn, m) + 0.5*klDivergence(qn, m)
	if jsd <= 0 {
		return 0
	}
	return math.Sqrt(jsd)
}

func TotalVariationDistance(p, q []float64) float64 {
	n := len(p)
	if len(q) < n {
		n = len(q)
	}
	if n == 0 {
		return 0
	}
	pn := normalizeDistribution(p[:n])
	qn := normalizeDistribution(q[:n])
	sum := 0.0
	for i := 0; i < n; i++ {
		sum += math.Abs(pn[i] - qn[i])
	}
	return sum / 2
}

func CosineDistance(a, b []float64) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if n == 0 {
		return 1
	}
	var dot, an, bn float64
	for i := 0; i < n; i++ {
		dot += a[i] * b[i]
		an += a[i] * a[i]
		bn += b[i] * b[i]
	}
	if an == 0 || bn == 0 {
		return 1
	}
	return clamp01(1 - dot/(math.Sqrt(an)*math.Sqrt(bn)))
}

func Entropy(values []float64) float64 {
	sum := 0.0
	for _, value := range values {
		if value > 0 {
			sum += value
		}
	}
	if sum == 0 {
		return 0
	}
	entropy := 0.0
	for _, value := range values {
		if value <= 0 {
			continue
		}
		p := value / sum
		entropy -= p * math.Log(p)
	}
	if entropy < 0.0000000001 {
		return 0
	}
	return entropy
}

func PermutationTestMMD(reference, target []string, iterations int) MMDResult {
	observed := mmd2(reference, target)
	if iterations <= 0 {
		iterations = 100
	}
	if len(reference) < 2 || len(target) < 2 {
		return MMDResult{MMD2: observed, PValue: 1}
	}
	combined := append(append([]string{}, reference...), target...)
	refN := len(reference)
	rng := rand.New(rand.NewSource(17))
	count := 0
	for i := 0; i < iterations; i++ {
		shuffled := append([]string{}, combined...)
		rng.Shuffle(len(shuffled), func(i, j int) {
			shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
		})
		permuted := mmd2(shuffled[:refN], shuffled[refN:])
		if permuted >= observed {
			count++
		}
	}
	return MMDResult{
		MMD2:   observed,
		PValue: float64(1+count) / float64(iterations+1),
	}
}

func mmd2(x, y []string) float64 {
	if len(x) < 2 || len(y) < 2 {
		return 0
	}
	xx := 0.0
	for i := range x {
		for j := range x {
			if i != j {
				xx += ngramKernel(x[i], x[j])
			}
		}
	}
	yy := 0.0
	for i := range y {
		for j := range y {
			if i != j {
				yy += ngramKernel(y[i], y[j])
			}
		}
	}
	xy := 0.0
	for _, left := range x {
		for _, right := range y {
			xy += ngramKernel(left, right)
		}
	}
	return xx/float64(len(x)*(len(x)-1)) + yy/float64(len(y)*(len(y)-1)) - 2*xy/float64(len(x)*len(y))
}

func ngramKernel(a, b string) float64 {
	left := textNgramCounts(a)
	right := textNgramCounts(b)
	var dot, an, bn float64
	for key, value := range left {
		dot += value * right[key]
		an += value * value
	}
	for _, value := range right {
		bn += value * value
	}
	if an == 0 || bn == 0 {
		return 0
	}
	return dot / (math.Sqrt(an) * math.Sqrt(bn))
}

func textNgramCounts(value string) map[string]float64 {
	counts := map[string]float64{}
	normalized := strings.ToLower(strings.TrimSpace(value))
	for _, n := range []int{3, 4, 5} {
		runes := []rune(normalized)
		if len(runes) < n {
			continue
		}
		for i := 0; i+n <= len(runes); i++ {
			counts[string(runes[i:i+n])]++
		}
	}
	return counts
}

func normalizeDistribution(values []float64) []float64 {
	out := make([]float64, len(values))
	sum := 0.0
	for i, value := range values {
		if value > 0 {
			out[i] = value
			sum += value
		}
	}
	if sum == 0 {
		if len(values) == 0 {
			return out
		}
		for i := range out {
			out[i] = 1 / float64(len(out))
		}
		return out
	}
	for i := range out {
		out[i] /= sum
	}
	return out
}

func klDivergence(p, q []float64) float64 {
	sum := 0.0
	for i := range p {
		if p[i] <= 0 {
			continue
		}
		if q[i] <= 0 {
			continue
		}
		sum += p[i] * math.Log(p[i]/q[i])
	}
	return sum
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
