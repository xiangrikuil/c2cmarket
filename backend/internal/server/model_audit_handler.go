package server

import (
	"net/http"
	"time"

	"c2c-market/backend/internal/module/modelaudit"

	"github.com/go-chi/chi/v5"
)

type modelAuditTargetRequest struct {
	Name              string `json:"name"`
	BaseURL           string `json:"baseUrl"`
	ProviderType      string `json:"providerType"`
	ClaimedModel      string `json:"claimedModel"`
	APIKey            string `json:"apiKey"`
	Enabled           *bool  `json:"enabled"`
	APIServiceID      string `json:"apiServiceId"`
	APIServiceModelID string `json:"apiServiceModelId"`
}

type modelAuditTargetResponse struct {
	ID                string               `json:"id"`
	Name              string               `json:"name"`
	BaseURL           string               `json:"baseUrl"`
	ProviderType      string               `json:"providerType"`
	ClaimedModel      string               `json:"claimedModel"`
	Enabled           bool                 `json:"enabled"`
	APIServiceID      string               `json:"apiServiceId,omitempty"`
	APIServiceModelID string               `json:"apiServiceModelId,omitempty"`
	LastRiskLevel     modelaudit.RiskLevel `json:"lastRiskLevel,omitempty"`
	LastRunID         string               `json:"lastRunId,omitempty"`
	CreatedAt         string               `json:"createdAt"`
	UpdatedAt         string               `json:"updatedAt"`
}

type modelAuditBaselineRequest struct {
	BaselineName    string         `json:"baselineName"`
	SourceTargetID  string         `json:"sourceTargetId"`
	Model           string         `json:"model"`
	SourceType      string         `json:"sourceType"`
	ProbeSetVersion string         `json:"probeSetVersion"`
	ParamsJSON      map[string]any `json:"paramsJson"`
	FeatureJSON     map[string]any `json:"featureJson"`
	SampleCount     int            `json:"sampleCount"`
}

type modelAuditBaselineResponse struct {
	ID              string         `json:"id"`
	BaselineName    string         `json:"baselineName"`
	SourceTargetID  string         `json:"sourceTargetId,omitempty"`
	Model           string         `json:"model"`
	SourceType      string         `json:"sourceType"`
	ProbeSetVersion string         `json:"probeSetVersion"`
	ParamsJSON      map[string]any `json:"paramsJson"`
	FeatureJSON     map[string]any `json:"featureJson"`
	SampleCount     int            `json:"sampleCount"`
	ValidFrom       string         `json:"validFrom"`
	ValidTo         *string        `json:"validTo,omitempty"`
	CreatedAt       string         `json:"createdAt"`
}

type modelAuditRunRequest struct {
	TargetID            string          `json:"targetId"`
	BaselineID          string          `json:"baselineId"`
	ClaimedModel        string          `json:"claimedModel"`
	Mode                modelaudit.AuditMode `json:"mode"`
	EnableModelEquality bool            `json:"enableModelEquality"`
	EnableLogprobs      string          `json:"enableLogprobs"`
	StorePromptText     bool            `json:"storePromptText"`
	StoreResponseText   bool            `json:"storeResponseText"`
	ScheduledMonitorID  string          `json:"scheduledMonitorId"`
}

type modelAuditRunResponse struct {
	ID             string               `json:"id"`
	TargetID       string               `json:"targetId"`
	TargetName     string               `json:"targetName"`
	ClaimedModel   string               `json:"claimedModel"`
	BaselineID      string               `json:"baselineId,omitempty"`
	Status         modelaudit.RunStatus  `json:"status"`
	Mode           modelaudit.AuditMode  `json:"mode"`
	RiskLevel      modelaudit.RiskLevel  `json:"riskLevel,omitempty"`
	Confidence     float64              `json:"confidence"`
	OverallScore   float64              `json:"overallScore"`
	ErrorMessage   string               `json:"errorMessage,omitempty"`
	ProbeScores    []modelaudit.ProbeScore `json:"probeScores,omitempty"`
	StartedAt      *string              `json:"startedAt,omitempty"`
	FinishedAt     *string              `json:"finishedAt,omitempty"`
	CreatedAt      string               `json:"createdAt"`
}

type modelAuditMonitorRequest struct {
	TargetID   string               `json:"targetId"`
	BaselineID string               `json:"baselineId"`
	Mode       modelaudit.AuditMode `json:"mode"`
	Enabled    *bool                `json:"enabled"`
	CronSpec   string               `json:"cronSpec"`
}

type modelAuditMonitorResponse struct {
	ID         string               `json:"id"`
	TargetID   string               `json:"targetId"`
	BaselineID string               `json:"baselineId,omitempty"`
	Mode       modelaudit.AuditMode `json:"mode"`
	Enabled    bool                 `json:"enabled"`
	CronSpec   string               `json:"cronSpec,omitempty"`
	LastRunID  string               `json:"lastRunId,omitempty"`
	LastRisk   modelaudit.RiskLevel `json:"lastRisk,omitempty"`
	LastRunAt  *string              `json:"lastRunAt,omitempty"`
	CreatedAt  string               `json:"createdAt"`
	UpdatedAt  string               `json:"updatedAt"`
}

func (s *Server) handleAdminModelAuditTargets(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	targets, appErr := s.app.AdminModelAuditTargets(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[modelAuditTargetResponse]{Items: toModelAuditTargetResponses(targets)})
}

func (s *Server) handleAdminModelAuditTarget(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	target, appErr := s.app.AdminModelAuditTarget(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toModelAuditTargetResponse(target))
}

func (s *Server) handleCreateModelAuditTarget(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	req, appErr := decodeStrictJSONOnly[modelAuditTargetRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	target, appErr := s.app.CreateModelAuditTarget(r.Context(), user, modelAuditTargetInput(req))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusCreated, toModelAuditTargetResponse(target))
}

func (s *Server) handleUpdateModelAuditTarget(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	req, appErr := decodeStrictJSONOnly[modelAuditTargetRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	target, appErr := s.app.UpdateModelAuditTarget(r.Context(), user, chi.URLParam(r, "id"), modelAuditTargetInput(req))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toModelAuditTargetResponse(target))
}

func (s *Server) handleDeleteModelAuditTarget(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if appErr := s.app.DeleteModelAuditTarget(r.Context(), user, chi.URLParam(r, "id")); appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAdminModelAuditBaselines(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	baselines, appErr := s.app.AdminModelAuditBaselines(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[modelAuditBaselineResponse]{Items: toModelAuditBaselineResponses(baselines)})
}

func (s *Server) handleAdminModelAuditBaseline(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	baseline, appErr := s.app.AdminModelAuditBaseline(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toModelAuditBaselineResponse(baseline))
}

func (s *Server) handleCreateModelAuditBaseline(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	req, appErr := decodeStrictJSONOnly[modelAuditBaselineRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	baseline, appErr := s.app.CreateModelAuditBaseline(r.Context(), user, modelAuditBaselineInput(req))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusCreated, toModelAuditBaselineResponse(baseline))
}

func (s *Server) handleAdminModelAuditRuns(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	runs, appErr := s.app.AdminModelAuditRuns(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[modelAuditRunResponse]{Items: toModelAuditRunResponses(runs)})
}

func (s *Server) handleAdminModelAuditRun(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	run, appErr := s.app.AdminModelAuditRun(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toModelAuditRunResponse(run))
}

func (s *Server) handleCreateModelAuditRun(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	req, appErr := decodeStrictJSONOnly[modelAuditRunRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	run, appErr := s.app.CreateModelAuditRun(r.Context(), user, modelAuditRunInput(req))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusCreated, toModelAuditRunResponse(run))
}

func (s *Server) handleCancelModelAuditRun(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	run, appErr := s.app.CancelModelAuditRun(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toModelAuditRunResponse(run))
}

func (s *Server) handleAdminModelAuditReport(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	report, appErr := s.app.AdminModelAuditReport(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleAdminModelAuditMonitors(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	monitors, appErr := s.app.AdminModelAuditMonitors(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[modelAuditMonitorResponse]{Items: toModelAuditMonitorResponses(monitors)})
}

func (s *Server) handleCreateModelAuditMonitor(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	req, appErr := decodeStrictJSONOnly[modelAuditMonitorRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	monitor, appErr := s.app.CreateModelAuditMonitor(r.Context(), user, modelAuditMonitorInput(req))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusCreated, toModelAuditMonitorResponse(monitor))
}

func modelAuditTargetInput(req modelAuditTargetRequest) modelaudit.TargetInput {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	return modelaudit.TargetInput{
		Name:              req.Name,
		BaseURL:           req.BaseURL,
		ProviderType:      req.ProviderType,
		ClaimedModel:      req.ClaimedModel,
		APIKey:            req.APIKey,
		Enabled:           enabled,
		APIServiceID:      req.APIServiceID,
		APIServiceModelID: req.APIServiceModelID,
	}
}

func modelAuditBaselineInput(req modelAuditBaselineRequest) modelaudit.BaselineInput {
	return modelaudit.BaselineInput{
		BaselineName:    req.BaselineName,
		SourceTargetID:  req.SourceTargetID,
		Model:           req.Model,
		SourceType:      req.SourceType,
		ProbeSetVersion: req.ProbeSetVersion,
		ParamsJSON:      req.ParamsJSON,
		FeatureJSON:     req.FeatureJSON,
		SampleCount:     req.SampleCount,
	}
}

func modelAuditRunInput(req modelAuditRunRequest) modelaudit.RunInput {
	return modelaudit.RunInput{
		TargetID:            req.TargetID,
		BaselineID:          req.BaselineID,
		ClaimedModel:        req.ClaimedModel,
		Mode:                req.Mode,
		EnableModelEquality: req.EnableModelEquality,
		EnableLogprobs:      req.EnableLogprobs,
		StorePromptText:     req.StorePromptText,
		StoreResponseText:   req.StoreResponseText,
		ScheduledMonitorID:  req.ScheduledMonitorID,
	}
}

func modelAuditMonitorInput(req modelAuditMonitorRequest) modelaudit.MonitorInput {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	return modelaudit.MonitorInput{
		TargetID:   req.TargetID,
		BaselineID: req.BaselineID,
		Mode:       req.Mode,
		Enabled:    enabled,
		CronSpec:   req.CronSpec,
	}
}

func toModelAuditTargetResponses(targets []modelaudit.Target) []modelAuditTargetResponse {
	items := make([]modelAuditTargetResponse, 0, len(targets))
	for _, target := range targets {
		items = append(items, toModelAuditTargetResponse(target))
	}
	return items
}

func toModelAuditTargetResponse(target modelaudit.Target) modelAuditTargetResponse {
	return modelAuditTargetResponse{
		ID:                target.ID,
		Name:              target.Name,
		BaseURL:           target.BaseURL,
		ProviderType:      target.ProviderType,
		ClaimedModel:      target.ClaimedModel,
		Enabled:           target.Enabled,
		APIServiceID:      target.APIServiceID,
		APIServiceModelID: target.APIServiceModelID,
		LastRiskLevel:     target.LastRiskLevel,
		LastRunID:         target.LastRunID,
		CreatedAt:         target.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:         target.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toModelAuditBaselineResponses(baselines []modelaudit.Baseline) []modelAuditBaselineResponse {
	items := make([]modelAuditBaselineResponse, 0, len(baselines))
	for _, baseline := range baselines {
		items = append(items, toModelAuditBaselineResponse(baseline))
	}
	return items
}

func toModelAuditBaselineResponse(baseline modelaudit.Baseline) modelAuditBaselineResponse {
	return modelAuditBaselineResponse{
		ID:              baseline.ID,
		BaselineName:    baseline.BaselineName,
		SourceTargetID:  baseline.SourceTargetID,
		Model:           baseline.Model,
		SourceType:      baseline.SourceType,
		ProbeSetVersion: baseline.ProbeSetVersion,
		ParamsJSON:      baseline.ParamsJSON,
		FeatureJSON:     baseline.FeatureJSON,
		SampleCount:     baseline.SampleCount,
		ValidFrom:       baseline.ValidFrom.UTC().Format(time.RFC3339),
		ValidTo:         formatOptionalTime(baseline.ValidTo),
		CreatedAt:       baseline.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func toModelAuditRunResponses(runs []modelaudit.Run) []modelAuditRunResponse {
	items := make([]modelAuditRunResponse, 0, len(runs))
	for _, run := range runs {
		items = append(items, toModelAuditRunResponse(run))
	}
	return items
}

func toModelAuditRunResponse(run modelaudit.Run) modelAuditRunResponse {
	return modelAuditRunResponse{
		ID:           run.ID,
		TargetID:     run.TargetID,
		TargetName:   run.TargetName,
		ClaimedModel: run.ClaimedModel,
		BaselineID:    run.BaselineID,
		Status:       run.Status,
		Mode:         run.Mode,
		RiskLevel:    run.RiskLevel,
		Confidence:   run.Confidence,
		OverallScore: run.OverallScore,
		ErrorMessage: run.ErrorMessage,
		ProbeScores:  run.ProbeScores,
		StartedAt:    formatOptionalTime(run.StartedAt),
		FinishedAt:   formatOptionalTime(run.FinishedAt),
		CreatedAt:    run.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func toModelAuditMonitorResponses(monitors []modelaudit.Monitor) []modelAuditMonitorResponse {
	items := make([]modelAuditMonitorResponse, 0, len(monitors))
	for _, monitor := range monitors {
		items = append(items, toModelAuditMonitorResponse(monitor))
	}
	return items
}

func toModelAuditMonitorResponse(monitor modelaudit.Monitor) modelAuditMonitorResponse {
	return modelAuditMonitorResponse{
		ID:         monitor.ID,
		TargetID:   monitor.TargetID,
		BaselineID: monitor.BaselineID,
		Mode:       monitor.Mode,
		Enabled:    monitor.Enabled,
		CronSpec:   monitor.CronSpec,
		LastRunID:  monitor.LastRunID,
		LastRisk:   monitor.LastRisk,
		LastRunAt:  formatOptionalTime(monitor.LastRunAt),
		CreatedAt:  monitor.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:  monitor.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
