package apihealth

import (
	"math"
	"math/big"
	"sort"
	"strconv"
	"time"
)

func SlotStart(at time.Time) time.Time {
	return at.UTC().Truncate(ProbeSlotDuration)
}

func SummaryStart(at time.Time) time.Time {
	return at.UTC().Truncate(time.Hour).Add(-(SummarySlotCount - 1) * time.Hour)
}

func theoreticalSlots(windowStart, now time.Time) int {
	lastSlot := SlotStart(now)
	if lastSlot.Before(windowStart) {
		return 0
	}
	count := int(lastSlot.Sub(windowStart)/ProbeSlotDuration) + 1
	if count > SummaryTheoreticalSamples {
		return SummaryTheoreticalSamples
	}
	return count
}

func BuildSummary(connection *Connection, samples []Sample, now time.Time) Summary {
	if connection == nil {
		return noSampleSummary(AvailabilityUnconfigured, TransportSecurityUnknown, now)
	}
	transportSecurity := TargetTransportSecurity(connection.BaseURL)
	if !connection.Enabled {
		return connectionNoSampleSummary(connection, AvailabilityDisabled, transportSecurity, now)
	}
	if connection.VerificationStatus != VerificationVerified || connection.VerifiedAt == nil || connection.ProbeModel == "" || connection.ProbeProtocol == "" {
		return connectionNoSampleSummary(connection, AvailabilityUnverified, transportSecurity, now)
	}

	windowStart := SummaryStart(now)
	windowEnd := now.UTC()
	bySlot := make(map[time.Time]Sample, SummaryTheoreticalSamples)
	for _, sample := range samples {
		slot := sample.SlotStartedAt.UTC()
		if sample.MeasurementVersion != connection.MeasurementVersion ||
			(sample.Status != SampleStatusSucceeded && sample.Status != SampleStatusFailed) ||
			slot.After(windowEnd) {
			continue
		}
		previous, exists := bySlot[slot]
		if !exists || sample.CreatedAt.After(previous.CreatedAt) {
			bySlot[slot] = sample
		}
	}
	ordered := make([]Sample, 0, len(bySlot))
	for _, sample := range bySlot {
		ordered = append(ordered, sample)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].SlotStartedAt.Before(ordered[j].SlotStartedAt) })

	summary := connectionNoSampleSummary(connection, AvailabilityInsufficient, transportSecurity, now)
	summary.HourlyBuckets = buildHourlyBuckets(ordered, windowStart, windowEnd)
	summary.Samples = legacyHealthSlots(summary.HourlyBuckets)
	ttfts := make([]int, 0, len(ordered))
	var knownBaseCost, knownRetryCost big.Rat
	for _, sample := range ordered {
		if sample.SlotStartedAt.Before(windowStart) || sample.SlotStartedAt.After(windowEnd) {
			continue
		}
		summary.CompletedCycles++
		switch sample.Outcome {
		case OutcomeFirstSuccess:
			summary.FirstAttemptSuccesses++
		case OutcomeFirstSuccessSlow:
			summary.FirstAttemptSuccesses++
		case OutcomeRetryRecovered:
			summary.RetryRecoveries++
		case OutcomeFinalFailure:
			summary.FinalFailures++
		}
		if (sample.Outcome == OutcomeFirstSuccess || sample.Outcome == OutcomeFirstSuccessSlow) && sample.FirstAttemptTTFTMS != nil {
			ttfts = append(ttfts, *sample.FirstAttemptTTFTMS)
		}
		if sample.FinishedAt != nil && (summary.LastSampledAt == nil || sample.FinishedAt.After(*summary.LastSampledAt)) {
			value := sample.FinishedAt.UTC()
			summary.LastSampledAt = &value
		}
		baseCostKnown := sample.BaseCostUSD != "" && addDecimal(&knownBaseCost, sample.BaseCostUSD)
		retryCostKnown := sample.AttemptCount < 2
		if sample.AttemptCount >= 2 {
			retryCostKnown = sample.RetryCostUSD != "" && addDecimal(&knownRetryCost, sample.RetryCostUSD)
		}
		if sample.UsageComplete && baseCostKnown && retryCostKnown {
			summary.Cost.KnownUsageSamples++
		} else {
			summary.Cost.HasUnknownUsage = true
		}
	}

	summary.TotalSamples = summary.CompletedCycles
	summary.SuccessfulSamples = summary.FirstAttemptSuccesses
	summary.TheoreticalSlots = theoreticalSlots(windowStart, now)
	summary.CoveragePercent = percent(summary.CompletedCycles, summary.TheoreticalSlots)
	summary.StabilityPercent = percentagePointer(summary.FirstAttemptSuccesses, summary.CompletedCycles)
	summary.SuccessRatePercent = summary.StabilityPercent
	summary.FinalSuccessPercent = percentagePointer(summary.FirstAttemptSuccesses+summary.RetryRecoveries, summary.CompletedCycles)
	if len(ttfts) > 0 {
		sort.Ints(ttfts)
		average := roundedAverage(ttfts)
		summary.AverageTTFTMS = &average
		summary.P50TTFTMS = percentile(ttfts, 50)
		summary.P95TTFTMS = percentile(ttfts, 95)
	}
	summary.Cost.KnownBaseCostUSD = knownBaseCost.FloatString(10)
	summary.Cost.KnownRetryCostUSD = knownRetryCost.FloatString(10)
	if summary.Cost.KnownUsageSamples > 0 {
		total := new(big.Rat).Add(&knownBaseCost, &knownRetryCost)
		total.Mul(total, big.NewRat(SummaryTheoreticalSamples, int64(summary.Cost.KnownUsageSamples)))
		summary.Cost.ProjectedDailyCostUSD = total.FloatString(10)
	}
	if connection.ProbeModelChangedAt != nil && now.UTC().Sub(*connection.ProbeModelChangedAt) < ModelChangeNoticeDuration {
		summary.ProbeModelChangedAt = connection.ProbeModelChangedAt
	}
	if summary.CompletedCycles < MinimumFinalSamples {
		summary.AccumulatingSamples = true
		return summary
	}
	if summary.LastSampledAt == nil || now.UTC().Sub(*summary.LastSampledAt) > SummaryStaleAfter {
		summary.AvailabilityReason = AvailabilityStale
		return summary
	}
	summary.AvailabilityReason = ""
	summary.State = HealthStateNormal
	if hasAbnormalCurrentState(summary.HourlyBuckets) || ratioBelow(summary.FirstAttemptSuccesses+summary.RetryRecoveries, summary.CompletedCycles, 80) {
		summary.State = HealthStateAbnormal
	} else if summary.RetryRecoveries > 0 || summary.FinalFailures > 0 || hasSlowOutcome(ordered, windowStart, windowEnd) {
		summary.State = HealthStateFluctuating
	}
	return summary
}

func buildHourlyBuckets(samples []Sample, windowStart, windowEnd time.Time) []HourlyBucket {
	buckets := make([]HourlyBucket, SummarySlotCount)
	indexByHour := make(map[time.Time]int, SummarySlotCount)
	for index := 0; index < SummarySlotCount; index++ {
		hour := windowStart.Add(time.Duration(index) * time.Hour)
		buckets[index] = HourlyBucket{HourStartedAt: hour, State: SlotStateNoSample}
		indexByHour[hour] = index
	}
	previousFailure := false
	for _, sample := range samples {
		isFailure := sample.Outcome == OutcomeFinalFailure
		if sample.SlotStartedAt.Before(windowStart) {
			previousFailure = isFailure
			continue
		}
		if sample.SlotStartedAt.After(windowEnd) {
			continue
		}
		hour := sample.SlotStartedAt.UTC().Truncate(time.Hour)
		index, exists := indexByHour[hour]
		if !exists {
			continue
		}
		bucket := &buckets[index]
		bucket.CompletedCycles++
		switch sample.Outcome {
		case OutcomeFirstSuccess:
			bucket.FirstAttemptSuccesses++
		case OutcomeFirstSuccessSlow:
			bucket.FirstAttemptSuccesses++
			bucket.SlowSuccesses++
		case OutcomeRetryRecovered:
			bucket.RetryRecoveries++
		case OutcomeFinalFailure:
			bucket.FinalFailures++
		}
		if isFailure && previousFailure {
			bucket.State = SlotStateAbnormal
		}
		previousFailure = isFailure
	}
	for index := range buckets {
		bucket := &buckets[index]
		if bucket.CompletedCycles == 0 {
			continue
		}
		bucket.FinalSuccessPercent = percentagePointer(bucket.FirstAttemptSuccesses+bucket.RetryRecoveries, bucket.CompletedCycles)
		if bucket.State == SlotStateAbnormal || ratioBelow(bucket.FirstAttemptSuccesses+bucket.RetryRecoveries, bucket.CompletedCycles, 80) {
			bucket.State = SlotStateAbnormal
		} else if bucket.RetryRecoveries > 0 || bucket.FinalFailures > 0 || bucket.SlowSuccesses > 0 {
			bucket.State = SlotStateFluctuating
		} else {
			bucket.State = SlotStateSmooth
		}
		values := make([]int, 0, bucket.FirstAttemptSuccesses)
		for _, sample := range samples {
			if sample.SlotStartedAt.UTC().Truncate(time.Hour) == bucket.HourStartedAt && sample.FirstAttemptTTFTMS != nil && (sample.Outcome == OutcomeFirstSuccess || sample.Outcome == OutcomeFirstSuccessSlow) {
				values = append(values, *sample.FirstAttemptTTFTMS)
			}
		}
		if len(values) > 0 {
			average := roundedAverage(values)
			bucket.AverageTTFTMS = &average
		}
	}
	return buckets
}

func connectionNoSampleSummary(connection *Connection, reason, transport string, now time.Time) Summary {
	summary := noSampleSummary(reason, transport, now)
	summary.ProbeModel = connection.ProbeModel
	summary.ProbeProtocol = connection.ProbeProtocol
	summary.ProbeEnvironment = connection.ProbeEnvironment
	if connection.ProbeModelChangedAt != nil && now.UTC().Sub(*connection.ProbeModelChangedAt) < ModelChangeNoticeDuration {
		summary.ProbeModelChangedAt = connection.ProbeModelChangedAt
	}
	return summary
}

func noSampleSummary(reason, transportSecurity string, now time.Time) Summary {
	buckets := make([]HourlyBucket, SummarySlotCount)
	start := SummaryStart(now)
	for index := range buckets {
		buckets[index] = HourlyBucket{HourStartedAt: start.Add(time.Duration(index) * time.Hour), State: SlotStateNoSample}
	}
	return Summary{
		State: HealthStateNoSample, AvailabilityReason: reason, TransportSecurity: transportSecurity,
		CoveragePercent: "0.0", TheoreticalSlots: theoreticalSlots(start, now), HourlyBuckets: buckets,
		Samples: legacyHealthSlots(buckets),
	}
}

func legacyHealthSlots(buckets []HourlyBucket) []HealthSlot {
	result := make([]HealthSlot, 0, len(buckets))
	for _, bucket := range buckets {
		result = append(result, HealthSlot{SlotStartedAt: bucket.HourStartedAt, State: bucket.State})
	}
	return result
}

func percentagePointer(numerator, denominator int) *string {
	if denominator <= 0 {
		return nil
	}
	value := percent(numerator, denominator)
	return &value
}

func percent(numerator, denominator int) string {
	if denominator <= 0 {
		return "0.0"
	}
	tenths := (numerator*1000 + denominator/2) / denominator
	return strconv.Itoa(tenths/10) + "." + strconv.Itoa(tenths%10)
}

func ratioBelow(numerator, denominator, threshold int) bool {
	return denominator > 0 && numerator*100 < denominator*threshold
}

func percentile(sortedValues []int, value int) *int {
	if len(sortedValues) == 0 {
		return nil
	}
	index := int(math.Ceil(float64(value)*float64(len(sortedValues))/100.0)) - 1
	if index < 0 {
		index = 0
	}
	result := sortedValues[index]
	return &result
}

func roundedAverage(values []int) int {
	if len(values) == 0 {
		return 0
	}
	total := 0
	for _, value := range values {
		total += value
	}
	return (total + len(values)/2) / len(values)
}

func addDecimal(total *big.Rat, value string) bool {
	parsed, ok := decimalRat(value)
	if !ok {
		return false
	}
	total.Add(total, parsed)
	return true
}

func hasSlowOutcome(samples []Sample, start, end time.Time) bool {
	for _, sample := range samples {
		if !sample.SlotStartedAt.Before(start) && !sample.SlotStartedAt.After(end) && sample.Outcome == OutcomeFirstSuccessSlow {
			return true
		}
	}
	return false
}

func hasAbnormalCurrentState(buckets []HourlyBucket) bool {
	for index := len(buckets) - 1; index >= 0; index-- {
		if buckets[index].CompletedCycles == 0 {
			continue
		}
		return buckets[index].State == SlotStateAbnormal
	}
	return false
}
