package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apihealth"
	app "c2c-market/backend/internal/module/core"
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

func TestAPIHealthProjectionExposesOnlyTransportSecurity(t *testing.T) {
	for _, transportSecurity := range []string{apihealth.TransportSecurityHTTPS, apihealth.TransportSecurityHTTP} {
		transportSecurity := transportSecurity
		t.Run(transportSecurity, func(t *testing.T) {
			t.Parallel()
			response := toAPIServiceHealthSummaryResponse(apihealth.Summary{
				State: apihealth.HealthStateNoSample, AvailabilityReason: apihealth.AvailabilityInsufficient,
				TransportSecurity: transportSecurity,
			})
			if response.TransportSecurity == nil || *response.TransportSecurity != transportSecurity {
				t.Fatalf("unexpected transport security projection: %+v", response)
			}
			body, err := json.Marshal(response)
			if err != nil {
				t.Fatalf("marshal health summary: %v", err)
			}
			if string(body) == "" || strings.Contains(string(body), "api.example.com") || strings.Contains(string(body), "baseUrl") {
				t.Fatalf("public health projection exposed target details: %s", body)
			}
		})
	}
}

func TestAPIHealthProjectionUsesNullForUnknownTransport(t *testing.T) {
	response := toAPIServiceHealthSummaryResponse(apihealth.TemporarilyUnavailableSummary(time.Now()))
	if response.TransportSecurity != nil {
		t.Fatalf("unknown transport must be public null: %+v", response)
	}
	body, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal health summary: %v", err)
	}
	if !strings.Contains(string(body), `"transportSecurity":null`) {
		t.Fatalf("unknown transport did not marshal as null: %s", body)
	}
}

func TestOwnerAPIServiceListHealthSummariesFailOpenInOneBatch(t *testing.T) {
	now := time.Date(2026, 8, 5, 8, 0, 0, 0, time.UTC)
	health := &failingAPIHealthSummaryService{apiHealthRouteService: &apiHealthRouteService{}}
	server := NewServer(
		app.NewServiceWithClock(func() time.Time { return now }),
		ServerOptions{EnableDevAuth: true, APIHealth: health},
	)
	ownerSession := createLinuxDoSession(t, server, "api-health-owner-list")
	ownerContact := createContactMethod(t, server, ownerSession, "telegram", "API Health Owner", "@api_health_owner")
	first := createAPIService(t, server, ownerSession, ownerContact.ID, "api-health-owner-list-first")
	second := createAPIService(t, server, ownerSession, ownerContact.ID, "api-health-owner-list-second")

	request := httptest.NewRequest(http.MethodGet, "/api/v1/owner/api-services?salesView=all", nil)
	addCookie(request, ownerSession.cookie)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("owner API service list status %d body %s", response.Code, response.Body.String())
	}
	var payload listResponse[createdAPIService]
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode owner API service list: %v", err)
	}
	if health.callCount != 1 {
		t.Fatalf("expected one health summary batch, got %d", health.callCount)
	}
	wantServiceIDs := map[string]bool{first.ID: true, second.ID: true}
	if len(health.requestedServiceIDs) != len(wantServiceIDs) {
		t.Fatalf("unexpected health summary IDs: %v", health.requestedServiceIDs)
	}
	for _, serviceID := range health.requestedServiceIDs {
		if !wantServiceIDs[serviceID] {
			t.Fatalf("unexpected health summary service ID %q", serviceID)
		}
	}
	if len(payload.Items) != 2 {
		t.Fatalf("expected two owner services, got %+v", payload.Items)
	}
	for _, item := range payload.Items {
		if item.HealthSummary.State != apihealth.HealthStateNoSample ||
			item.HealthSummary.AvailabilityReason == nil ||
			*item.HealthSummary.AvailabilityReason != apihealth.AvailabilityTemporarilyUnavailable ||
			len(item.HealthSummary.Samples) != apihealth.SummarySlotCount {
			t.Fatalf("unexpected fail-open health summary for %s: %+v", item.ID, item.HealthSummary)
		}
	}
}

type failingAPIHealthSummaryService struct {
	*apiHealthRouteService
	requestedServiceIDs []string
	callCount           int
}

func (service *failingAPIHealthSummaryService) Summaries(_ context.Context, serviceIDs []string) (map[string]apihealth.Summary, *domain.AppError) {
	service.callCount++
	service.requestedServiceIDs = append([]string(nil), serviceIDs...)
	return nil, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "探针汇总暂时不可用。")
}
