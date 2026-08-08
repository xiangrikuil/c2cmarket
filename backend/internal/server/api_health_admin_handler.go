package server

import (
	"net/http"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apihealth"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"
)

type apiProbeLatencyRuleRequest struct {
	Model         string `json:"model"`
	Protocol      string `json:"protocol"`
	Environment   string `json:"environment"`
	SlowTtftMs    int    `json:"slowTtftMs"`
	HardTimeoutMs int    `json:"hardTimeoutMs"`
}

type apiProbeCalibrationResponse struct {
	Model                string    `json:"model"`
	Protocol             string    `json:"protocol"`
	Environment          string    `json:"environment"`
	EnvironmentLabel     string    `json:"environmentLabel"`
	ObservationStartedAt time.Time `json:"observationStartedAt"`
	ObservationEndedAt   time.Time `json:"observationEndedAt"`
	CompleteCalendarDays int       `json:"completeCalendarDays"`
	ConnectionCount      int       `json:"connectionCount"`
	SampleCount          int64     `json:"sampleCount"`
	P50TtftMs            *int      `json:"p50TtftMs"`
	P90TtftMs            *int      `json:"p90TtftMs"`
	P95TtftMs            *int      `json:"p95TtftMs"`
	P99TtftMs            *int      `json:"p99TtftMs"`
	Ready                bool      `json:"ready"`
}

type apiProbeLatencyRulePreviewResponse struct {
	Calibration        apiProbeCalibrationResponse `json:"calibration"`
	SlowTtftMs         int                         `json:"slowTtftMs"`
	HardTimeoutMs      int                         `json:"hardTimeoutMs"`
	SlowSampleCount    int64                       `json:"slowSampleCount"`
	SlowPercent        string                      `json:"slowPercent"`
	OverTimeoutCount   int64                       `json:"overTimeoutCount"`
	OverTimeoutPercent string                      `json:"overTimeoutPercent"`
}

type apiProbeLatencyRuleResponse struct {
	ID                   string     `json:"id"`
	Model                string     `json:"model"`
	Protocol             string     `json:"protocol"`
	Environment          string     `json:"environment"`
	EnvironmentLabel     string     `json:"environmentLabel"`
	Version              int64      `json:"version"`
	SlowTtftMs           int        `json:"slowTtftMs"`
	HardTimeoutMs        int        `json:"hardTimeoutMs"`
	ObservationStartedAt time.Time  `json:"observationStartedAt"`
	ObservationEndedAt   time.Time  `json:"observationEndedAt"`
	CompleteCalendarDays int        `json:"completeCalendarDays"`
	ConnectionCount      int        `json:"connectionCount"`
	SampleCount          int64      `json:"sampleCount"`
	P50TtftMs            *int       `json:"p50TtftMs"`
	P90TtftMs            *int       `json:"p90TtftMs"`
	P95TtftMs            *int       `json:"p95TtftMs"`
	P99TtftMs            *int       `json:"p99TtftMs"`
	Status               string     `json:"status"`
	PublishedAt          time.Time  `json:"publishedAt"`
	SupersededAt         *time.Time `json:"supersededAt"`
}

func (server *Server) handleAdminAPIProbeLatencyCalibration(w http.ResponseWriter, request *http.Request) {
	setAPIHealthPrivateHeaders(w)
	_, appErr := server.requireAPIHealthAdmin(w, request, false)
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	model, protocol, environment := probeLatencyDimension(request.URL.Query().Get("model"), request.URL.Query().Get("protocol"), request.URL.Query().Get("environment"))
	calibration, appErr := server.adminAPIHealth.ProbeCalibration(request.Context(), model, protocol, environment)
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toAPIProbeCalibrationResponse(calibration))
}

func (server *Server) handlePreviewAdminAPIProbeLatencyRule(w http.ResponseWriter, request *http.Request) {
	setAPIHealthPrivateHeaders(w)
	_, appErr := server.requireAPIHealthAdmin(w, request, true)
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	input, appErr := decodeStrictJSONOnly[apiProbeLatencyRuleRequest](request)
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	model, protocol, environment := probeLatencyDimension(input.Model, input.Protocol, input.Environment)
	preview, appErr := server.adminAPIHealth.PreviewLatencyRule(request.Context(), model, protocol, environment, input.SlowTtftMs, input.HardTimeoutMs)
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toAPIProbeLatencyRulePreviewResponse(preview))
}

func (server *Server) handlePublishAdminAPIProbeLatencyRule(w http.ResponseWriter, request *http.Request) {
	setAPIHealthPrivateHeaders(w)
	admin, appErr := server.requireAPIHealthAdmin(w, request, true)
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	body, input, appErr := decodeStrictJSON[apiProbeLatencyRuleRequest](request)
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	model, protocol, environment := probeLatencyDimension(input.Model, input.Protocol, input.Environment)
	routeKey := "POST /api/v1/admin/api-health/latency-rules"
	server.withAPIHealthIdempotency(w, request, admin.ID, routeKey, body, func() (idempotency.Completion, *domain.AppError) {
		rule, appErr := server.adminAPIHealth.PublishLatencyRule(request.Context(), admin, model, protocol, environment, input.SlowTtftMs, input.HardTimeoutMs)
		if appErr != nil {
			return idempotency.Completion{}, appErr
		}
		return apiHealthIdempotencyCompletion(http.StatusCreated, toAPIProbeLatencyRuleResponse(rule), rule.Version, rule.ID)
	})
}

func (server *Server) handleAdminAPIProbeLatencyRules(w http.ResponseWriter, request *http.Request) {
	setAPIHealthPrivateHeaders(w)
	_, appErr := server.requireAPIHealthAdmin(w, request, false)
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	rules, appErr := server.adminAPIHealth.LatencyRules(request.Context())
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	items := make([]apiProbeLatencyRuleResponse, 0, len(rules))
	for _, rule := range rules {
		items = append(items, toAPIProbeLatencyRuleResponse(rule))
	}
	writeJSON(w, http.StatusOK, listResponse[apiProbeLatencyRuleResponse]{Items: items})
}

func (server *Server) requireAPIHealthAdmin(w http.ResponseWriter, request *http.Request, csrf bool) (auth.User, *domain.AppError) {
	var user auth.User
	var appErr *domain.AppError
	if csrf {
		user, _, appErr = server.requireSessionAndCSRF(w, request)
	} else {
		user, _, appErr = server.requireSession(w, request)
	}
	if appErr != nil {
		return auth.User{}, appErr
	}
	if !user.IsAdmin {
		return auth.User{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	if server.adminAPIHealth == nil {
		return auth.User{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "API health calibration unavailable", "探针校准服务暂时不可用。")
	}
	if appErr = server.requireAPIHealthService(); appErr != nil {
		return auth.User{}, appErr
	}
	return user, nil
}

func probeLatencyDimension(model, protocol, environment string) (string, string, string) {
	model = strings.TrimSpace(model)
	if model == "" {
		model = apihealth.DefaultGPTProbeModel
	}
	protocol = strings.TrimSpace(protocol)
	if protocol == "" {
		protocol = apihealth.ProtocolResponsesV1
	}
	environment = strings.TrimSpace(environment)
	if environment == "" {
		environment = apihealth.ProbeEnvironmentUSWestV1
	}
	return model, protocol, environment
}

func toAPIProbeCalibrationResponse(value apihealth.Calibration) apiProbeCalibrationResponse {
	return apiProbeCalibrationResponse{
		Model: value.Model, Protocol: value.Protocol, Environment: value.Environment, EnvironmentLabel: "平台美西",
		ObservationStartedAt: value.ObservationStartedAt, ObservationEndedAt: value.ObservationEndedAt,
		CompleteCalendarDays: value.CompleteCalendarDays, ConnectionCount: value.ConnectionCount,
		SampleCount: value.SampleCount, P50TtftMs: value.P50TTFTMS, P90TtftMs: value.P90TTFTMS,
		P95TtftMs: value.P95TTFTMS, P99TtftMs: value.P99TTFTMS, Ready: value.Ready,
	}
}

func toAPIProbeLatencyRulePreviewResponse(value apihealth.LatencyRulePreview) apiProbeLatencyRulePreviewResponse {
	return apiProbeLatencyRulePreviewResponse{
		Calibration: toAPIProbeCalibrationResponse(value.Calibration), SlowTtftMs: value.SlowTTFTMS,
		HardTimeoutMs: value.HardTimeoutMS, SlowSampleCount: value.SlowSampleCount,
		SlowPercent: value.SlowPercent, OverTimeoutCount: value.OverTimeoutCount,
		OverTimeoutPercent: value.OverTimeoutPercent,
	}
}

func toAPIProbeLatencyRuleResponse(value apihealth.LatencyRule) apiProbeLatencyRuleResponse {
	return apiProbeLatencyRuleResponse{
		ID: value.ID, Model: value.Model, Protocol: value.Protocol, Environment: value.Environment,
		EnvironmentLabel: "平台美西", Version: value.Version, SlowTtftMs: value.SlowTTFTMS,
		HardTimeoutMs: value.HardTimeoutMS, ObservationStartedAt: value.ObservationStartedAt,
		ObservationEndedAt: value.ObservationEndedAt, CompleteCalendarDays: value.CompleteCalendarDays,
		ConnectionCount: value.ConnectionCount, SampleCount: value.SampleCount,
		P50TtftMs: value.P50TTFTMS, P90TtftMs: value.P90TTFTMS, P95TtftMs: value.P95TTFTMS,
		P99TtftMs: value.P99TTFTMS, Status: value.Status, PublishedAt: value.PublishedAt,
		SupersededAt: value.SupersededAt,
	}
}
