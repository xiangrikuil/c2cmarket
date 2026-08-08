package apihealth

import (
	"testing"
	"time"
)

func TestBuildSummaryUsesConnectionSamplesWithoutModelOrTTFT(t *testing.T) {
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

func finalSample(connection Connection, slot time.Time, status string) Sample {
	finished := slot.Add(time.Second)
	duration := 25
	return Sample{
		ID: slot.String(), ConnectionID: connection.ID, MeasurementVersion: connection.MeasurementVersion,
		SlotStartedAt: slot, Status: status, TotalDurationMS: &duration,
		StartedAt: slot, FinishedAt: &finished, CreatedAt: slot,
	}
}
