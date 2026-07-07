package modelaudit

import (
	"strings"
	"testing"
)

func TestRiskAggregatorAppliesWeightsAndHighRiskOverride(t *testing.T) {
	aggregator := DefaultRiskAggregator()
	result := aggregator.Aggregate([]ProbeScore{
		{Probe: ProbeKBF, Risk: RiskHigh, Score: 0.8, Confidence: 0.8},
		{Probe: ProbeActiveFingerprint, Risk: RiskHigh, Score: 0.7, Confidence: 0.7},
		{Probe: ProbeRandomFingerprint, Risk: RiskConsistent, Score: 0.05, Confidence: 0.9},
	})

	if result.Risk != RiskHigh {
		t.Fatalf("expected KBF + active high risk override, got %+v", result)
	}
}

func TestRiskAggregatorReturnsInsufficientDataForLowConfidence(t *testing.T) {
	aggregator := DefaultRiskAggregator()
	result := aggregator.Aggregate([]ProbeScore{
		{Probe: ProbeRandomFingerprint, Risk: RiskInsufficientData, Score: 0.2, Confidence: 0.1},
		{Probe: ProbeLogprobTracking, Risk: RiskNotApplicable, Score: 0, Confidence: 0},
	})

	if result.Risk != RiskInsufficientData {
		t.Fatalf("expected insufficient data, got %+v", result)
	}
}

func TestReportBuilderIncludesEvidenceAndCaveat(t *testing.T) {
	report := BuildAuditReport(AuditReportInput{
		RunID:            "run-1",
		TargetID:         "target-1",
		TargetName:       "Example Relay",
		ClaimedModel:     "gpt-example",
		Mode:             AuditModeStandard,
		OverallRisk:      RiskSuspicious,
		OverallScore:     0.42,
		OverallConfidence: 0.78,
		ProbeScores: []ProbeScore{{
			Probe:      ProbeRandomFingerprint,
			Risk:       RiskSuspicious,
			Score:      0.42,
			Confidence: 0.8,
			Evidence:   map[string]any{"js_distance": 0.27},
		}},
	})

	if report.RiskLevel != RiskSuspicious || len(report.ProbeScores) != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
	if !strings.Contains(report.Markdown, "统计风险审计") {
		t.Fatalf("markdown must include statistical-risk caveat, got:\n%s", report.Markdown)
	}
	if strings.Contains(report.Summary, "假") || strings.Contains(strings.ToLower(report.Summary), "fake") {
		t.Fatalf("summary must not make absolute fake/real claims: %q", report.Summary)
	}
}
