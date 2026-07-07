package modelaudit

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

const (
	KBFAnswerInteger     KBFAnswerType = "integer"
	KBFAnswerFloat       KBFAnswerType = "float"
	KBFAnswerYear        KBFAnswerType = "year"
	KBFAnswerDate        KBFAnswerType = "date"
	KBFAnswerShortString KBFAnswerType = "short_string"
)

type KBFAnswerType string

type ParsedKBFAnswer struct {
	QuestionID      string
	Value           string
	Valid           bool
	FormatViolation bool
	Abstain         bool
}

type KBFQuestionBaseline struct {
	QuestionID       string
	ClaimedCounts    map[string]int
	CompetitorName   string
	CompetitorCounts map[string]int
}

type KBFSeedQuestion struct {
	ID         string
	Prompt     string
	AnswerType KBFAnswerType
	Domain     string
}

type KBFItemScore struct {
	QuestionID            string
	ParsedValue           string
	MatchesClaimed        bool
	ClaimedLikelihood     float64
	ClosestCompetitor     string
	CompetitorLikelihood  float64
	LogLikelihoodRatio    float64
	CorrectAgainstCanonical bool
	FormatViolation      bool
	Abstain              bool
}

type MixedRoutingEstimate struct {
	Suspected                  bool
	EstimatedSubstitutionRate  float64
	Confidence                 float64
	CompetitorLikeSampleCount  int
	TotalSampleCount           int
}

var (
	yearPattern  = regexp.MustCompile(`\b(1[0-9]{3}|20[0-9]{2})\b`)
	intPattern   = regexp.MustCompile(`[-+]?\d[\d,]*`)
	floatPattern = regexp.MustCompile(`[-+]?\d+(?:,\d{3})*(?:\.\d+)?`)
)

func KBFSeedBank() []KBFSeedQuestion {
	return []KBFSeedQuestion{
		{ID: "kbf_software_python_pep8_year_v1", Prompt: "In what year was PEP 8 first created? Output only the year.", AnswerType: KBFAnswerYear, Domain: "software"},
		{ID: "kbf_math_known_constant_pi_v1", Prompt: "Output pi rounded to five decimal places. Output only the number.", AnswerType: KBFAnswerFloat, Domain: "math"},
		{ID: "kbf_web_http_418_phrase_v1", Prompt: "What short phrase is associated with HTTP 418? Output only the phrase.", AnswerType: KBFAnswerShortString, Domain: "web"},
	}
}

func ParseKBFAnswer(text string, answerType KBFAnswerType) ParsedKBFAnswer {
	trimmed := strings.TrimSpace(text)
	lower := strings.ToLower(trimmed)
	abstain := strings.Contains(lower, "don't know") || strings.Contains(lower, "do not know") || strings.Contains(lower, "不知道") || strings.Contains(lower, "无法确定")
	if abstain {
		return ParsedKBFAnswer{Abstain: true}
	}
	switch answerType {
	case KBFAnswerYear:
		value := yearPattern.FindString(trimmed)
		return ParsedKBFAnswer{Value: value, Valid: value != "", FormatViolation: hasExplanation(trimmed, value)}
	case KBFAnswerInteger:
		value := intPattern.FindString(trimmed)
		value = strings.ReplaceAll(value, ",", "")
		return ParsedKBFAnswer{Value: value, Valid: value != "", FormatViolation: hasExplanation(trimmed, value)}
	case KBFAnswerFloat:
		value := floatPattern.FindString(trimmed)
		value = strings.ReplaceAll(value, ",", "")
		return ParsedKBFAnswer{Value: value, Valid: value != "", FormatViolation: hasExplanation(trimmed, value)}
	case KBFAnswerDate:
		value := normalizeShortString(trimmed)
		return ParsedKBFAnswer{Value: value, Valid: value != ""}
	case KBFAnswerShortString:
		value := normalizeShortString(trimmed)
		return ParsedKBFAnswer{Value: value, Valid: value != ""}
	default:
		value := normalizeShortString(trimmed)
		return ParsedKBFAnswer{Value: value, Valid: value != ""}
	}
}

func ScoreKBFAnswers(baselines []KBFQuestionBaseline, answers []ParsedKBFAnswer) ProbeScore {
	baselineByQuestion := map[string]KBFQuestionBaseline{}
	for _, baseline := range baselines {
		baselineByQuestion[baseline.QuestionID] = baseline
	}
	items := []KBFItemScore{}
	for _, answer := range answers {
		baseline, ok := baselineByQuestion[answer.QuestionID]
		if !ok || !answer.Valid {
			continue
		}
		claimed := smoothedLikelihood(baseline.ClaimedCounts, answer.Value)
		competitor := smoothedLikelihood(baseline.CompetitorCounts, answer.Value)
		llr := math.Log(claimed / math.Max(competitor, 0.000001))
		items = append(items, KBFItemScore{
			QuestionID:           answer.QuestionID,
			ParsedValue:          answer.Value,
			MatchesClaimed:       claimed >= competitor,
			ClaimedLikelihood:    claimed,
			ClosestCompetitor:    baseline.CompetitorName,
			CompetitorLikelihood: competitor,
			LogLikelihoodRatio:   llr,
			FormatViolation:     answer.FormatViolation,
			Abstain:             answer.Abstain,
		})
	}
	if len(items) == 0 {
		return ProbeScore{Probe: ProbeKBF, Risk: RiskInsufficientData, Evidence: map[string]any{"sample_count": 0}}
	}
	claimedMatches := 0
	llrValues := make([]float64, 0, len(items))
	for _, item := range items {
		if item.MatchesClaimed {
			claimedMatches++
		}
		llrValues = append(llrValues, item.LogLikelihoodRatio)
	}
	matchRate := float64(claimedMatches) / float64(len(items))
	llrMean := mean(llrValues)
	risk := RiskSuspicious
	if len(items) < 1 {
		risk = RiskInsufficientData
	} else if matchRate >= 0.75 && llrMean > 0 {
		risk = RiskConsistent
	} else if matchRate < 0.50 || llrMean < -0.25 {
		risk = RiskHigh
	}
	return ProbeScore{
		Probe:      ProbeKBF,
		Risk:       risk,
		Confidence: clamp01(float64(len(items)) / 40),
		Score:      clamp01((1 - matchRate) + math.Max(0, -llrMean)/2),
		Evidence: map[string]any{
			"sample_count":        len(items),
			"claimed_match_rate":  matchRate,
			"kbf_llr_mean":        llrMean,
			"mixed_routing_hint":  EstimateMixedRouting(items),
		},
	}
}

func EstimateMixedRouting(samples []KBFItemScore) MixedRoutingEstimate {
	if len(samples) == 0 {
		return MixedRoutingEstimate{}
	}
	competitorLike := 0
	for _, sample := range samples {
		if sample.CompetitorLikelihood > sample.ClaimedLikelihood*2 {
			competitorLike++
		}
	}
	rate := float64(competitorLike) / float64(len(samples))
	return MixedRoutingEstimate{
		Suspected:                 competitorLike > 0 && rate >= 0.05,
		EstimatedSubstitutionRate: rate,
		Confidence:                clamp01(float64(len(samples)) / 20),
		CompetitorLikeSampleCount: competitorLike,
		TotalSampleCount:          len(samples),
	}
}

func smoothedLikelihood(counts map[string]int, value string) float64 {
	alpha := 0.5
	total := 0
	for _, count := range counts {
		total += count
	}
	k := len(counts)
	if _, ok := counts[value]; !ok {
		k++
	}
	return (float64(counts[value]) + alpha) / (float64(total) + alpha*float64(k))
}

func hasExplanation(text, value string) bool {
	if value == "" {
		return false
	}
	return strings.TrimSpace(text) != value
}

func normalizeShortString(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastSpace := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastSpace = false
			continue
		}
		if unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) {
			if !lastSpace {
				builder.WriteRune(' ')
				lastSpace = true
			}
		}
	}
	return strings.TrimSpace(builder.String())
}

func parseFloat(value string) float64 {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return parsed
}
