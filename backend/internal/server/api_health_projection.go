package server

import (
	"context"
	"strings"
	"time"

	"c2c-market/backend/internal/module/apihealth"
)

type apiServiceHealthSampleResponse struct {
	SlotStartedAt string `json:"slotStartedAt"`
	State         string `json:"state"`
}

type apiServiceHealthHourlyBucketResponse struct {
	HourStartedAt         string  `json:"hourStartedAt"`
	State                 string  `json:"state"`
	CompletedCycles       int     `json:"completedCycles"`
	FirstAttemptSuccesses int     `json:"firstAttemptSuccesses"`
	RetryRecoveries       int     `json:"retryRecoveries"`
	FinalFailures         int     `json:"finalFailures"`
	SlowSuccesses         int     `json:"slowSuccesses"`
	FinalSuccessPercent   *string `json:"finalSuccessPercent"`
	AverageTtftMs         *int    `json:"averageTtftMs"`
}

type apiServiceHealthCostResponse struct {
	KnownBaseCostUSD      string `json:"knownBaseCostUsd"`
	KnownRetryCostUSD     string `json:"knownRetryCostUsd"`
	ProjectedDailyCostUSD string `json:"projectedDailyCostUsd"`
	HasUnknownUsage       bool   `json:"hasUnknownUsage"`
	KnownUsageSamples     int    `json:"knownUsageSamples"`
}

type apiServiceHealthSummaryResponse struct {
	State                 string                                 `json:"state"`
	AvailabilityReason    *string                                `json:"availabilityReason"`
	TransportSecurity     *string                                `json:"transportSecurity"`
	StabilityPercent      *string                                `json:"stabilityPercent"`
	FinalSuccessPercent   *string                                `json:"finalSuccessPercent"`
	CoveragePercent       string                                 `json:"coveragePercent"`
	CompletedCycles       int                                    `json:"completedCycles"`
	TheoreticalSlots      int                                    `json:"theoreticalSlots"`
	FirstAttemptSuccesses int                                    `json:"firstAttemptSuccesses"`
	RetryRecoveries       int                                    `json:"retryRecoveries"`
	FinalFailures         int                                    `json:"finalFailures"`
	AverageTtftMs         *int                                   `json:"averageTtftMs"`
	P50TtftMs             *int                                   `json:"p50TtftMs"`
	P95TtftMs             *int                                   `json:"p95TtftMs"`
	LastSampledAt         *string                                `json:"lastSampledAt"`
	ProbeModel            *string                                `json:"probeModel"`
	ProbeProtocol         *string                                `json:"probeProtocol"`
	ProbeEnvironment      *string                                `json:"probeEnvironment"`
	ProbeEnvironmentLabel *string                                `json:"probeEnvironmentLabel"`
	ProbeModelChangedAt   *string                                `json:"probeModelChangedAt"`
	AccumulatingSamples   bool                                   `json:"accumulatingSamples"`
	HourlyBuckets         []apiServiceHealthHourlyBucketResponse `json:"hourlyBuckets"`
	Cost                  apiServiceHealthCostResponse           `json:"cost"`
	// Legacy aliases are temporary and mirror the new first-attempt metrics.
	SuccessRatePercent *string                          `json:"successRatePercent"`
	SuccessfulSamples  int                              `json:"successfulSamples"`
	TotalSamples       int                              `json:"totalSamples"`
	Samples            []apiServiceHealthSampleResponse `json:"samples"`
}

func (s *Server) loadAPIHealthSummaries(ctx context.Context, serviceIDs []string) map[string]apihealth.Summary {
	unique := make([]string, 0, len(serviceIDs))
	seen := make(map[string]struct{}, len(serviceIDs))
	for _, serviceID := range serviceIDs {
		serviceID = strings.TrimSpace(serviceID)
		if serviceID == "" {
			continue
		}
		if _, exists := seen[serviceID]; exists {
			continue
		}
		seen[serviceID] = struct{}{}
		unique = append(unique, serviceID)
	}
	if len(unique) == 0 {
		return map[string]apihealth.Summary{}
	}
	now := time.Now().UTC()
	if s == nil || s.apiHealth == nil {
		return fallbackAPIHealthSummaries(unique, now, false)
	}
	summaries, appErr := s.apiHealth.Summaries(ctx, unique)
	if appErr != nil {
		return fallbackAPIHealthSummaries(unique, now, true)
	}
	if summaries == nil {
		summaries = make(map[string]apihealth.Summary, len(unique))
	}
	for _, serviceID := range unique {
		if _, exists := summaries[serviceID]; !exists {
			summaries[serviceID] = apihealth.BuildSummary(nil, nil, now)
		}
	}
	return summaries
}

func fallbackAPIHealthSummaries(serviceIDs []string, now time.Time, temporarilyUnavailable bool) map[string]apihealth.Summary {
	result := make(map[string]apihealth.Summary, len(serviceIDs))
	for _, serviceID := range serviceIDs {
		if temporarilyUnavailable {
			result[serviceID] = apihealth.TemporarilyUnavailableSummary(now)
		} else {
			result[serviceID] = apihealth.BuildSummary(nil, nil, now)
		}
	}
	return result
}

func toAPIServiceHealthSummaryResponse(summary apihealth.Summary) apiServiceHealthSummaryResponse {
	var availabilityReason *string
	if summary.AvailabilityReason != "" {
		value := summary.AvailabilityReason
		availabilityReason = &value
	}
	var lastSampledAt *string
	if summary.LastSampledAt != nil {
		value := summary.LastSampledAt.UTC().Format(time.RFC3339)
		lastSampledAt = &value
	}
	samples := make([]apiServiceHealthSampleResponse, 0, len(summary.Samples))
	for _, sample := range summary.Samples {
		samples = append(samples, apiServiceHealthSampleResponse{SlotStartedAt: sample.SlotStartedAt.UTC().Format(time.RFC3339), State: sample.State})
	}
	buckets := make([]apiServiceHealthHourlyBucketResponse, 0, len(summary.HourlyBuckets))
	for _, bucket := range summary.HourlyBuckets {
		buckets = append(buckets, apiServiceHealthHourlyBucketResponse{
			HourStartedAt: bucket.HourStartedAt.UTC().Format(time.RFC3339), State: bucket.State,
			CompletedCycles: bucket.CompletedCycles, FirstAttemptSuccesses: bucket.FirstAttemptSuccesses,
			RetryRecoveries: bucket.RetryRecoveries, FinalFailures: bucket.FinalFailures,
			SlowSuccesses: bucket.SlowSuccesses, FinalSuccessPercent: bucket.FinalSuccessPercent,
			AverageTtftMs: bucket.AverageTTFTMS,
		})
	}
	var transportSecurity *string
	if summary.TransportSecurity == apihealth.TransportSecurityHTTPS || summary.TransportSecurity == apihealth.TransportSecurityHTTP {
		transportSecurity = &summary.TransportSecurity
	}
	probeModel := apiHealthOptionalString(summary.ProbeModel)
	probeProtocol := apiHealthOptionalString(summary.ProbeProtocol)
	probeEnvironment := apiHealthOptionalString(summary.ProbeEnvironment)
	var environmentLabel *string
	if summary.ProbeEnvironment == apihealth.ProbeEnvironmentUSWestV1 {
		value := "平台美西"
		environmentLabel = &value
	}
	var modelChangedAt *string
	if summary.ProbeModelChangedAt != nil {
		value := summary.ProbeModelChangedAt.UTC().Format(time.RFC3339)
		modelChangedAt = &value
	}
	return apiServiceHealthSummaryResponse{
		State: summary.State, AvailabilityReason: availabilityReason, TransportSecurity: transportSecurity,
		StabilityPercent: summary.StabilityPercent, FinalSuccessPercent: summary.FinalSuccessPercent,
		CoveragePercent: summary.CoveragePercent, CompletedCycles: summary.CompletedCycles,
		TheoreticalSlots: summary.TheoreticalSlots, FirstAttemptSuccesses: summary.FirstAttemptSuccesses,
		RetryRecoveries: summary.RetryRecoveries, FinalFailures: summary.FinalFailures,
		AverageTtftMs: summary.AverageTTFTMS, P50TtftMs: summary.P50TTFTMS, P95TtftMs: summary.P95TTFTMS,
		LastSampledAt: lastSampledAt, ProbeModel: probeModel, ProbeProtocol: probeProtocol,
		ProbeEnvironment: probeEnvironment, ProbeEnvironmentLabel: environmentLabel,
		ProbeModelChangedAt: modelChangedAt, AccumulatingSamples: summary.AccumulatingSamples,
		HourlyBuckets: buckets,
		Cost: apiServiceHealthCostResponse{
			KnownBaseCostUSD: summary.Cost.KnownBaseCostUSD, KnownRetryCostUSD: summary.Cost.KnownRetryCostUSD,
			ProjectedDailyCostUSD: summary.Cost.ProjectedDailyCostUSD,
			HasUnknownUsage:       summary.Cost.HasUnknownUsage, KnownUsageSamples: summary.Cost.KnownUsageSamples,
		},
		SuccessRatePercent: summary.StabilityPercent, SuccessfulSamples: summary.FirstAttemptSuccesses,
		TotalSamples: summary.CompletedCycles, Samples: samples,
	}
}

func apiHealthOptionalString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	copy := value
	return &copy
}
