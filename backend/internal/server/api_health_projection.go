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

type apiServiceHealthSummaryResponse struct {
	State              string                           `json:"state"`
	AvailabilityReason *string                          `json:"availabilityReason"`
	TransportSecurity  *string                          `json:"transportSecurity"`
	SuccessRatePercent *string                          `json:"successRatePercent"`
	SuccessfulSamples  int                              `json:"successfulSamples"`
	TotalSamples       int                              `json:"totalSamples"`
	LastSampledAt      *string                          `json:"lastSampledAt"`
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
		samples = append(samples, apiServiceHealthSampleResponse{
			SlotStartedAt: sample.SlotStartedAt.UTC().Format(time.RFC3339),
			State:         sample.State,
		})
	}
	var transportSecurity *string
	if summary.TransportSecurity == apihealth.TransportSecurityHTTPS || summary.TransportSecurity == apihealth.TransportSecurityHTTP {
		transportSecurity = &summary.TransportSecurity
	}
	return apiServiceHealthSummaryResponse{
		State: summary.State, AvailabilityReason: availabilityReason, TransportSecurity: transportSecurity,
		SuccessRatePercent: summary.SuccessRatePercent, SuccessfulSamples: summary.SuccessfulSamples,
		TotalSamples: summary.TotalSamples, LastSampledAt: lastSampledAt, Samples: samples,
	}
}
