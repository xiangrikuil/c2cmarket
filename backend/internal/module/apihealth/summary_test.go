package apihealth

import (
	"testing"
	"time"
)

func TestBuildSummaryUsesFirstAttemptSuccesses(t *testing.T) {
	now := time.Date(2026, 8, 8, 10, 4, 0, 0, time.UTC)
	connection := verifiedConnection(now.Add(-time.Hour))
	connection.MeasurementVersion = 2
	samples := []Sample{
		finalSample(connection, SlotStart(now).Add(-10*time.Minute), SampleStatusSucceeded),
		finalSample(connection, SlotStart(now).Add(-5*time.Minute), SampleStatusSucceeded),
		finalSample(connection, SlotStart(now), SampleStatusSucceeded),
	}
	summary := BuildSummary(&connection, samples, now)
	if summary.State != HealthStateNormal || summary.SuccessRatePercent == nil || *summary.SuccessRatePercent != "100.0" {
		t.Fatalf("summary=%+v", summary)
	}
	if len(summary.Samples) != SummarySlotCount || summary.Samples[len(summary.Samples)-1].State != SlotStateSmooth {
		t.Fatalf("unexpected slots: %+v", summary.Samples)
	}
}

func TestBuildSummaryIgnoresOldMeasurementVersionAndDetectsRecentFailures(t *testing.T) {
	now := time.Date(2026, 8, 8, 10, 4, 0, 0, time.UTC)
	connection := verifiedConnection(now.Add(-time.Hour))
	connection.MeasurementVersion = 3
	old := finalSample(connection, SlotStart(now).Add(-15*time.Minute), SampleStatusSucceeded)
	old.MeasurementVersion = 2
	samples := []Sample{
		old,
		finalSample(connection, SlotStart(now).Add(-10*time.Minute), SampleStatusSucceeded),
		finalSample(connection, SlotStart(now).Add(-5*time.Minute), SampleStatusFailed),
		finalSample(connection, SlotStart(now), SampleStatusFailed),
	}
	summary := BuildSummary(&connection, samples, now)
	if summary.TotalSamples != 3 || summary.State != HealthStateAbnormal {
		t.Fatalf("summary=%+v", summary)
	}
}

func TestBuildSummarySeparatesStabilityFinalSuccessAndFirstAttemptTTFT(t *testing.T) {
	now := time.Date(2026, 8, 8, 10, 49, 0, 0, time.UTC)
	connection := verifiedConnection(now.Add(-time.Hour))
	samples := make([]Sample, 0, 10)
	for index := 0; index < 8; index++ {
		ttft := 100 + index*10
		samples = append(samples, outcomeSample(connection, SlotStart(now).Add(time.Duration(index-9)*ProbeSlotDuration), OutcomeFirstSuccess, &ttft))
	}
	samples = append(samples,
		outcomeSample(connection, SlotStart(now).Add(-ProbeSlotDuration), OutcomeRetryRecovered, nil),
		outcomeSample(connection, SlotStart(now), OutcomeFinalFailure, nil),
	)

	summary := BuildSummary(&connection, samples, now)

	if summary.StabilityPercent == nil || *summary.StabilityPercent != "80.0" || summary.FinalSuccessPercent == nil || *summary.FinalSuccessPercent != "90.0" {
		t.Fatalf("unexpected success rates: %+v", summary)
	}
	if summary.RetryRecoveries != 1 || summary.FinalFailures != 1 || summary.AverageTTFTMS == nil || *summary.AverageTTFTMS != 135 {
		t.Fatalf("unexpected counts/TTFT: %+v", summary)
	}
	if len(summary.HourlyBuckets) != 24 || summary.TheoreticalSlots != 286 {
		t.Fatalf("unexpected 24-hour shape: buckets=%d slots=%d", len(summary.HourlyBuckets), summary.TheoreticalSlots)
	}
}

func TestBuildSummaryCoverageUsesOnlyReachedSlotsInCurrentHour(t *testing.T) {
	now := time.Date(2026, 8, 8, 10, 4, 0, 0, time.UTC)
	connection := verifiedConnection(now.Add(-time.Hour))
	summary := BuildSummary(&connection, []Sample{
		outcomeSample(connection, SlotStart(now), OutcomeFirstSuccess, intPointer(100)),
	}, now)

	if summary.TheoreticalSlots != 277 || summary.CoveragePercent != "0.4" {
		t.Fatalf("unexpected reached-slot coverage: slots=%d coverage=%s", summary.TheoreticalSlots, summary.CoveragePercent)
	}
}

func TestBuildSummaryMarksCurrentHourRedForConsecutiveFailuresAcrossHourBoundary(t *testing.T) {
	now := time.Date(2026, 8, 8, 11, 4, 0, 0, time.UTC)
	connection := verifiedConnection(now.Add(-time.Hour))
	samples := make([]Sample, 0, 13)
	for minute := 0; minute <= 50; minute += 5 {
		ttft := 100
		samples = append(samples, outcomeSample(connection, time.Date(2026, 8, 8, 10, minute, 0, 0, time.UTC), OutcomeFirstSuccess, &ttft))
	}
	samples = append(samples,
		outcomeSample(connection, time.Date(2026, 8, 8, 10, 55, 0, 0, time.UTC), OutcomeFinalFailure, nil),
		outcomeSample(connection, time.Date(2026, 8, 8, 11, 0, 0, 0, time.UTC), OutcomeFinalFailure, nil),
	)

	summary := BuildSummary(&connection, samples, now)
	previous := summary.HourlyBuckets[len(summary.HourlyBuckets)-2]
	current := summary.HourlyBuckets[len(summary.HourlyBuckets)-1]
	if previous.State != SlotStateFluctuating || current.State != SlotStateAbnormal {
		t.Fatalf("unexpected cross-hour states: previous=%+v current=%+v", previous, current)
	}
}

func TestBuildSummaryKeepsSlowSuccessGreenWithoutPublishedRuleOutcome(t *testing.T) {
	now := time.Date(2026, 8, 8, 10, 14, 0, 0, time.UTC)
	connection := verifiedConnection(now.Add(-time.Hour))
	ttft := 9000
	samples := []Sample{
		outcomeSample(connection, SlotStart(now).Add(-10*time.Minute), OutcomeFirstSuccess, &ttft),
		outcomeSample(connection, SlotStart(now).Add(-5*time.Minute), OutcomeFirstSuccess, &ttft),
		outcomeSample(connection, SlotStart(now), OutcomeFirstSuccess, &ttft),
	}

	summary := BuildSummary(&connection, samples, now)
	if summary.State != HealthStateNormal || summary.HourlyBuckets[len(summary.HourlyBuckets)-1].State != SlotStateSmooth {
		t.Fatalf("unpublished slow threshold changed health color: %+v", summary)
	}
}

func TestBuildSummaryKeepsKnownCostWhenRetryUsageIsUnknown(t *testing.T) {
	now := time.Date(2026, 8, 8, 10, 14, 0, 0, time.UTC)
	connection := verifiedConnection(now.Add(-time.Hour))
	sample := outcomeSample(connection, SlotStart(now), OutcomeRetryRecovered, nil)
	sample.AttemptCount = 2
	sample.BaseCostUSD = "0.0010000000"
	sample.RetryCostUSD = ""
	sample.UsageComplete = false

	summary := BuildSummary(&connection, []Sample{sample}, now)
	if summary.Cost.KnownBaseCostUSD != "0.0010000000" || summary.Cost.KnownRetryCostUSD != "0.0000000000" ||
		!summary.Cost.HasUnknownUsage || summary.Cost.KnownUsageSamples != 0 || summary.Cost.ProjectedDailyCostUSD != "" {
		t.Fatalf("unexpected mixed known/unknown cost summary: %+v", summary.Cost)
	}
}

func finalSample(connection Connection, slot time.Time, status string) Sample {
	finished := slot.Add(time.Second)
	duration := 25
	outcome := OutcomeFirstSuccess
	if status == SampleStatusFailed {
		outcome = OutcomeFinalFailure
	}
	return Sample{
		ID: slot.String(), ConnectionID: connection.ID, MeasurementVersion: connection.MeasurementVersion,
		SlotStartedAt: slot, Status: status, Outcome: outcome, AttemptCount: 1,
		FirstAttemptTTFTMS: &duration, FirstAttemptTotalDurationMS: &duration, TotalDurationMS: &duration,
		StartedAt: slot, FinishedAt: &finished, CreatedAt: slot,
	}
}

func outcomeSample(connection Connection, slot time.Time, outcome string, ttft *int) Sample {
	status := SampleStatusSucceeded
	if outcome == OutcomeFinalFailure {
		status = SampleStatusFailed
	}
	finished := slot.Add(time.Second)
	duration := 1000
	return Sample{
		ID: slot.String(), ConnectionID: connection.ID, MeasurementVersion: connection.MeasurementVersion,
		SlotStartedAt: slot, Status: status, Outcome: outcome, AttemptCount: 1,
		FirstAttemptTTFTMS: ttft, FirstAttemptTotalDurationMS: &duration, TotalDurationMS: &duration,
		StartedAt: slot, FinishedAt: &finished, CreatedAt: slot,
	}
}

func intPointer(value int) *int { return &value }
