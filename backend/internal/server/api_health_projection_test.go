package server

import (
	"context"
	"net/http"
	"testing"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apihealth"
)

func TestLoadAPIHealthSummariesDeduplicatesAndFailsOpen(t *testing.T) {
	t.Parallel()
	health := &failingAPIHealthSummaryService{apiHealthRouteService: &apiHealthRouteService{}}
	server := &Server{apiHealth: health}

	summaries := server.loadAPIHealthSummaries(context.Background(), []string{"service-a", " service-a ", "", "service-b"})
	if len(health.requestedServiceIDs) != 2 || health.requestedServiceIDs[0] != "service-a" || health.requestedServiceIDs[1] != "service-b" {
		t.Fatalf("summary IDs were not deduplicated: %v", health.requestedServiceIDs)
	}
	for _, serviceID := range []string{"service-a", "service-b"} {
		summary := summaries[serviceID]
		if summary.State != apihealth.HealthStateNoSample || summary.AvailabilityReason != apihealth.AvailabilityTemporarilyUnavailable || len(summary.Samples) != apihealth.SummarySlotCount {
			t.Fatalf("unexpected fail-open summary for %s: %+v", serviceID, summary)
		}
	}
}

type failingAPIHealthSummaryService struct {
	*apiHealthRouteService
	requestedServiceIDs []string
}

func (service *failingAPIHealthSummaryService) Summaries(_ context.Context, serviceIDs []string) (map[string]apihealth.Summary, *domain.AppError) {
	service.requestedServiceIDs = append([]string(nil), serviceIDs...)
	return nil, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "探针汇总暂时不可用。")
}
