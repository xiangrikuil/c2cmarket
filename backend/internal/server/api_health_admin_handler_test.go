package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apihealth"
	"c2c-market/backend/internal/module/auth"
	app "c2c-market/backend/internal/module/core"
)

func TestAdminAPIHealthCalibrationPreviewAndPublishRoutes(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	adminHealth := &adminAPIHealthRouteService{calibration: apihealth.Calibration{
		Model: apihealth.DefaultGPTProbeModel, Protocol: apihealth.ProtocolResponsesV1,
		Environment: apihealth.ProbeEnvironmentUSWestV1, CompleteCalendarDays: 7,
		ConnectionCount: 5, SampleCount: 9000, Ready: true,
	}}
	handler := NewServer(app.NewServiceWithClock(func() time.Time { return now }), ServerOptions{
		EnableDevAuth: true, APIHealth: &apiHealthRouteService{}, AdminAPIHealth: adminHealth,
	})
	admin := createSession(t, handler, "api-health-admin", true)

	calibrationRequest := httptest.NewRequest(http.MethodGet, "/api/v1/admin/api-health/latency-calibration", nil)
	addCookie(calibrationRequest, admin.cookie)
	calibrationResponse := httptest.NewRecorder()
	handler.ServeHTTP(calibrationResponse, calibrationRequest)
	if calibrationResponse.Code != http.StatusOK || !strings.Contains(calibrationResponse.Body.String(), `"ready":true`) {
		t.Fatalf("calibration status=%d body=%s", calibrationResponse.Code, calibrationResponse.Body.String())
	}

	body := `{"model":"gpt-5.6-luna","protocol":"openai_responses_v1","environment":"us-west-v1","slowTtftMs":5000,"hardTimeoutMs":10000}`
	previewRequest := newJSONRequest(http.MethodPost, "/api/v1/admin/api-health/latency-rules/preview", body)
	addAuth(previewRequest, admin, "preview-latency")
	previewResponse := httptest.NewRecorder()
	handler.ServeHTTP(previewResponse, previewRequest)
	if previewResponse.Code != http.StatusOK || adminHealth.previewCalls != 1 {
		t.Fatalf("preview status=%d calls=%d body=%s", previewResponse.Code, adminHealth.previewCalls, previewResponse.Body.String())
	}

	publishRequest := newJSONRequest(http.MethodPost, "/api/v1/admin/api-health/latency-rules", body)
	addAuth(publishRequest, admin, "publish-latency")
	publishResponse := httptest.NewRecorder()
	handler.ServeHTTP(publishResponse, publishRequest)
	if publishResponse.Code != http.StatusCreated || adminHealth.publishCalls != 1 {
		t.Fatalf("publish status=%d calls=%d body=%s", publishResponse.Code, adminHealth.publishCalls, publishResponse.Body.String())
	}
}

type adminAPIHealthRouteService struct {
	calibration  apihealth.Calibration
	previewCalls int
	publishCalls int
}

func (service *adminAPIHealthRouteService) ProbeCalibration(context.Context, string, string, string) (apihealth.Calibration, *domain.AppError) {
	return service.calibration, nil
}

func (service *adminAPIHealthRouteService) PreviewLatencyRule(_ context.Context, model, protocol, environment string, slowTTFTMS, hardTimeoutMS int) (apihealth.LatencyRulePreview, *domain.AppError) {
	service.previewCalls++
	return apihealth.LatencyRulePreview{
		Calibration: service.calibration, SlowTTFTMS: slowTTFTMS, HardTimeoutMS: hardTimeoutMS,
		SlowSampleCount: 10, SlowPercent: "1.0", OverTimeoutCount: 1, OverTimeoutPercent: "0.1",
	}, nil
}

func (service *adminAPIHealthRouteService) PublishLatencyRule(_ context.Context, admin auth.User, model, protocol, environment string, slowTTFTMS, hardTimeoutMS int) (apihealth.LatencyRule, *domain.AppError) {
	service.publishCalls++
	return apihealth.LatencyRule{
		ID: "00000000-0000-0000-0000-000000000882", Model: model, Protocol: protocol,
		Environment: environment, Version: 1, SlowTTFTMS: slowTTFTMS, HardTimeoutMS: hardTimeoutMS,
		ObservationStartedAt: service.calibration.ObservationStartedAt,
		ObservationEndedAt:   service.calibration.ObservationEndedAt,
		CompleteCalendarDays: service.calibration.CompleteCalendarDays,
		ConnectionCount:      service.calibration.ConnectionCount, SampleCount: service.calibration.SampleCount,
		Status: "active", PublishedByAdminID: admin.ID, PublishedAt: time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC),
	}, nil
}

func (service *adminAPIHealthRouteService) LatencyRules(context.Context) ([]apihealth.LatencyRule, *domain.AppError) {
	return nil, nil
}
