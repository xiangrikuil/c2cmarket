package modelaudit

import "testing"

func TestParseKBFAnswerNormalizesCommonAnswerTypes(t *testing.T) {
	cases := []struct {
		name       string
		answerType KBFAnswerType
		text       string
		want       string
	}{
		{name: "year", answerType: KBFAnswerYear, text: "PEP 8 was created in 2001.", want: "2001"},
		{name: "integer with comma", answerType: KBFAnswerInteger, text: "The value is 1,024.", want: "1024"},
		{name: "float", answerType: KBFAnswerFloat, text: "Approximately 3.14159 units.", want: "3.14159"},
		{name: "short string", answerType: KBFAnswerShortString, text: "  Hello, World! ", want: "hello world"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseKBFAnswer(tc.text, tc.answerType)
			if !got.Valid || got.Value != tc.want {
				t.Fatalf("ParseKBFAnswer() = %+v, want %q", got, tc.want)
			}
		})
	}
}

func TestKBFScoreDistinguishesClaimedAndCompetitorAnswers(t *testing.T) {
	baseline := KBFQuestionBaseline{
		QuestionID:     "kbf_software_python_pep8_year_v1",
		ClaimedCounts:  map[string]int{"2001": 9, "2000": 1},
		CompetitorName: "cheap-model",
		CompetitorCounts: map[string]int{
			"1999": 8,
			"2001": 1,
		},
	}

	consistent := ScoreKBFAnswers([]KBFQuestionBaseline{baseline}, []ParsedKBFAnswer{{QuestionID: baseline.QuestionID, Value: "2001", Valid: true}})
	if consistent.Risk != RiskConsistent {
		t.Fatalf("expected consistent answer to match claimed baseline, got %+v", consistent)
	}

	substituted := ScoreKBFAnswers([]KBFQuestionBaseline{baseline}, []ParsedKBFAnswer{{QuestionID: baseline.QuestionID, Value: "1999", Valid: true}})
	if substituted.Risk != RiskHigh {
		t.Fatalf("expected competitor-like answer to be high risk, got %+v", substituted)
	}
}

func TestMixedRoutingEstimatorFlagsCompetitorLikeMinority(t *testing.T) {
	samples := []KBFItemScore{
		{QuestionID: "q1", ClaimedLikelihood: 0.9, CompetitorLikelihood: 0.05},
		{QuestionID: "q2", ClaimedLikelihood: 0.8, CompetitorLikelihood: 0.04},
		{QuestionID: "q3", ClaimedLikelihood: 0.05, CompetitorLikelihood: 0.8},
		{QuestionID: "q4", ClaimedLikelihood: 0.85, CompetitorLikelihood: 0.05},
	}

	estimate := EstimateMixedRouting(samples)
	if !estimate.Suspected {
		t.Fatalf("expected mixed routing hint, got %+v", estimate)
	}
	if estimate.EstimatedSubstitutionRate < 0.20 || estimate.EstimatedSubstitutionRate > 0.30 {
		t.Fatalf("unexpected substitution rate: %+v", estimate)
	}
}
