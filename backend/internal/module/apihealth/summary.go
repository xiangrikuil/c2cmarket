package apihealth

import (
	"sort"
	"strconv"
	"time"
)

func SlotStart(at time.Time) time.Time {
	return at.UTC().Truncate(ProbeSlotDuration)
}

func BuildSummary(config *Config, samples []Sample, now time.Time) Summary {
	if config == nil {
		return noSampleSummary(AvailabilityUnconfigured, nil, TransportSecurityUnknown, now)
	}
	model := config.Model
	transportSecurity := TargetTransportSecurity(config.BaseURL)
	if !config.Enabled {
		return noSampleSummary(AvailabilityDisabled, &model, transportSecurity, now)
	}
	if !IsAuthorized(*config) {
		return noSampleSummary(AvailabilityUnauthorized, &model, transportSecurity, now)
	}

	currentSlot := SlotStart(now)
	windowStart := currentSlot.Add(-(SummarySlotCount - 1) * ProbeSlotDuration)
	bySlot := make(map[time.Time]Sample, SummarySlotCount)
	final := make([]Sample, 0, SummarySlotCount)
	for _, sample := range samples {
		slot := sample.SlotStartedAt.UTC()
		if sample.MeasurementVersion != config.MeasurementVersion || sample.ProbeModelSnapshot != config.Model ||
			(sample.Status != SampleStatusSucceeded && sample.Status != SampleStatusFailed) ||
			slot.Before(windowStart) || slot.After(currentSlot) {
			continue
		}
		previous, exists := bySlot[slot]
		if !exists || sample.CreatedAt.After(previous.CreatedAt) {
			bySlot[slot] = sample
		}
	}
	for _, sample := range bySlot {
		final = append(final, sample)
	}
	sort.Slice(final, func(i, j int) bool { return final[i].SlotStartedAt.Before(final[j].SlotStartedAt) })

	summary := Summary{
		State: HealthStateNoSample, AvailabilityReason: AvailabilityInsufficient,
		TransportSecurity: transportSecurity, ProbeModel: &model,
	}
	summary.Samples = buildHealthSlots(bySlot, currentSlot)
	summary.TotalSamples = len(final)
	for _, sample := range final {
		if sample.Status == SampleStatusSucceeded {
			summary.SuccessfulSamples++
		}
		if sample.FinishedAt != nil && (summary.LastSampledAt == nil || sample.FinishedAt.After(*summary.LastSampledAt)) {
			value := sample.FinishedAt.UTC()
			summary.LastSampledAt = &value
		}
	}
	if len(final) < MinimumFinalSamples {
		return summary
	}
	if summary.LastSampledAt == nil || now.UTC().Sub(*summary.LastSampledAt) > SummaryStaleAfter {
		summary.AvailabilityReason = AvailabilityStale
		return summary
	}

	rateTenths := (summary.SuccessfulSamples*1000 + len(final)/2) / len(final)
	rate := strconv.Itoa(rateTenths/10) + "." + strconv.Itoa(rateTenths%10)
	summary.SuccessRatePercent = &rate
	summary.MedianTTFTMS = medianSuccessfulTTFT(final)
	summary.AvailabilityReason = ""
	recentTwoFailed := len(final) >= 2 && final[len(final)-1].Status == SampleStatusFailed && final[len(final)-2].Status == SampleStatusFailed
	switch {
	case rateTenths < 800 || recentTwoFailed:
		summary.State = HealthStateAbnormal
	case rateTenths < 950 || (summary.MedianTTFTMS != nil && *summary.MedianTTFTMS > 3000):
		summary.State = HealthStateFluctuating
	default:
		summary.State = HealthStateNormal
	}
	return summary
}

func noSampleSummary(reason string, model *string, transportSecurity string, now time.Time) Summary {
	return Summary{
		State: HealthStateNoSample, AvailabilityReason: reason,
		TransportSecurity: transportSecurity, ProbeModel: model,
		Samples: buildHealthSlots(nil, SlotStart(now)),
	}
}

func buildHealthSlots(samples map[time.Time]Sample, currentSlot time.Time) []HealthSlot {
	result := make([]HealthSlot, 0, SummarySlotCount)
	for index := SummarySlotCount - 1; index >= 0; index-- {
		slot := currentSlot.Add(-time.Duration(index) * ProbeSlotDuration)
		state := SlotStateNoSample
		if sample, ok := samples[slot]; ok {
			switch sample.Status {
			case SampleStatusFailed:
				state = SlotStateAbnormal
			case SampleStatusSucceeded:
				state = SlotStateFluctuating
				if sample.TTFTMS != nil && *sample.TTFTMS <= 3000 {
					state = SlotStateSmooth
				}
			}
		}
		result = append(result, HealthSlot{SlotStartedAt: slot, State: state})
	}
	return result
}

func medianSuccessfulTTFT(samples []Sample) *int {
	values := make([]int, 0, len(samples))
	for _, sample := range samples {
		if sample.Status == SampleStatusSucceeded && sample.TTFTMS != nil {
			values = append(values, *sample.TTFTMS)
		}
	}
	if len(values) == 0 {
		return nil
	}
	sort.Ints(values)
	median := values[len(values)/2]
	if len(values)%2 == 0 {
		median = (values[len(values)/2-1] + values[len(values)/2] + 1) / 2
	}
	return &median
}
