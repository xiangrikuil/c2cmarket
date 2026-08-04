package apihealth

import (
	"testing"
	"time"
)

func TestBuildSummaryThresholdsAndMedian(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 4, 12, 4, 0, 0, time.UTC)
	config := authorizedConfig(now)
	tests := []struct {
		name      string
		samples   []Sample
		state     string
		reason    string
		rate      string
		median    int
		hasMedian bool
	}{
		{name: "insufficient", samples: finalSamples(now, []sampleValue{{true, 900}, {true, 1100}}), state: HealthStateNoSample, reason: AvailabilityInsufficient},
		{name: "normal even median rounds", samples: finalSamples(now, []sampleValue{{true, 1000}, {true, 1001}, {true, 1200}, {true, 1300}}), state: HealthStateNormal, rate: "100.0", median: 1101, hasMedian: true},
		{name: "slow fluctuates", samples: finalSamples(now, []sampleValue{{true, 3001}, {true, 3200}, {true, 3400}}), state: HealthStateFluctuating, rate: "100.0", median: 3200, hasMedian: true},
		{name: "rate fluctuates", samples: finalSamples(now, []sampleValue{{true, 900}, {true, 1000}, {true, 1100}, {true, 1200}, {true, 1300}, {true, 1400}, {true, 1500}, {true, 1600}, {true, 1700}, {false, 0}}), state: HealthStateFluctuating, rate: "90.0", median: 1300, hasMedian: true},
		{name: "two recent failures abnormal", samples: finalSamples(now, []sampleValue{{true, 900}, {true, 1000}, {true, 1100}, {true, 1200}, {true, 1300}, {true, 1400}, {true, 1500}, {true, 1600}, {false, 0}, {false, 0}}), state: HealthStateAbnormal, rate: "80.0", median: 1250, hasMedian: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			summary := BuildSummary(&config, test.samples, now)
			if summary.State != test.state || summary.AvailabilityReason != test.reason || len(summary.Samples) != SummarySlotCount {
				t.Fatalf("unexpected summary: %+v", summary)
			}
			if test.rate != "" && (summary.SuccessRatePercent == nil || *summary.SuccessRatePercent != test.rate) {
				t.Fatalf("unexpected success rate: %+v", summary.SuccessRatePercent)
			}
			if test.hasMedian && (summary.MedianTTFTMS == nil || *summary.MedianTTFTMS != test.median) {
				t.Fatalf("unexpected median: %+v", summary.MedianTTFTMS)
			}
		})
	}
}

func TestBuildSummaryAvailabilityAndFiltering(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 4, 12, 4, 0, 0, time.UTC)
	if summary := BuildSummary(nil, nil, now); summary.AvailabilityReason != AvailabilityUnconfigured {
		t.Fatalf("unexpected unconfigured summary: %+v", summary)
	}
	config := authorizedConfig(now)
	config.Enabled = false
	if summary := BuildSummary(&config, nil, now); summary.AvailabilityReason != AvailabilityDisabled {
		t.Fatalf("unexpected disabled summary: %+v", summary)
	}
	config.Enabled = true
	config.AuthorizationStatus = AuthorizationPending
	if summary := BuildSummary(&config, nil, now); summary.AvailabilityReason != AvailabilityUnauthorized {
		t.Fatalf("unexpected unauthorized summary: %+v", summary)
	}
	config = authorizedConfig(now)
	samples := finalSamples(now.Add(-15*time.Minute), []sampleValue{{true, 1000}, {true, 1100}, {true, 1200}})
	if summary := BuildSummary(&config, samples, now); summary.AvailabilityReason != AvailabilityStale || summary.SuccessRatePercent != nil {
		t.Fatalf("unexpected stale summary: %+v", summary)
	}
	wrongVersion := finalSamples(now, []sampleValue{{true, 1000}, {true, 1100}, {true, 1200}})
	for index := range wrongVersion {
		wrongVersion[index].MeasurementVersion++
	}
	if summary := BuildSummary(&config, wrongVersion, now); summary.TotalSamples != 0 || summary.AvailabilityReason != AvailabilityInsufficient {
		t.Fatalf("wrong measurement version entered summary: %+v", summary)
	}
}

type sampleValue struct {
	succeeded bool
	ttft      int
}

func authorizedConfig(now time.Time) Config {
	verifiedAt := now.Add(-time.Hour)
	return Config{
		ID: "config", APIServiceID: "service", Protocol: ProtocolOpenAIChatCompletionsV1,
		BaseURL: "https://api.example.com/v1", NormalizedOrigin: "https://api.example.com:443",
		Model: "gpt-5", Enabled: true, AuthorizationStatus: AuthorizationVerified,
		AuthorizationMethod: AuthorizationMethodDNSTXT, VerifiedOrigin: "https://api.example.com:443",
		VerifiedAt: &verifiedAt, MeasurementVersion: 1,
	}
}

func finalSamples(latest time.Time, values []sampleValue) []Sample {
	result := make([]Sample, 0, len(values))
	firstSlot := SlotStart(latest).Add(-time.Duration(len(values)-1) * ProbeSlotDuration)
	for index, value := range values {
		slot := firstSlot.Add(time.Duration(index) * ProbeSlotDuration)
		finished := slot.Add(time.Minute)
		total := 1500
		sample := Sample{
			ID: string(rune('a' + index)), APIServiceID: "service", ProbeConfigID: "config",
			MeasurementVersion: 1, ProbeModelSnapshot: "gpt-5", SlotStartedAt: slot,
			TotalDurationMS: &total, FinishedAt: &finished, CreatedAt: finished,
		}
		if value.succeeded {
			ttft := value.ttft
			sample.Status = SampleStatusSucceeded
			sample.TTFTMS = &ttft
		} else {
			sample.Status = SampleStatusFailed
			sample.ErrorCode = ErrorConnectFailed
		}
		result = append(result, sample)
	}
	return result
}
