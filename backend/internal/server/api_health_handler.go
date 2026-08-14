package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apihealth"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/go-chi/chi/v5"
)

type apiProbeConnectionRequest struct {
	Name                    string  `json:"name"`
	BaseURL                 string  `json:"baseUrl"`
	Credential              *string `json:"credential"`
	ProbeModel              string  `json:"probeModel"`
	PreflightToken          string  `json:"preflightToken"`
	Enabled                 *bool   `json:"enabled"`
	AcknowledgeInsecureHTTP bool    `json:"acknowledgeInsecureHttp"`
}

type apiProbeConnectionServiceResponse struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type ownerAPIProbeConnectionResponse struct {
	ID                         string                              `json:"id"`
	Name                       string                              `json:"name"`
	BaseURL                    string                              `json:"baseUrl"`
	NormalizedBaseURL          string                              `json:"normalizedBaseUrl"`
	CredentialConfigured       bool                                `json:"credentialConfigured"`
	Enabled                    bool                                `json:"enabled"`
	VerificationStatus         string                              `json:"verificationStatus"`
	VerifiedAt                 *time.Time                          `json:"verifiedAt"`
	LastVerificationErrorCode  *string                             `json:"lastVerificationErrorCode"`
	ProbeModel                 *string                             `json:"probeModel"`
	ProbeProtocol              *string                             `json:"probeProtocol"`
	AvailableModels            []string                            `json:"availableModels"`
	ProbeEnvironment           string                              `json:"probeEnvironment"`
	ProbeModelChangedAt        *time.Time                          `json:"probeModelChangedAt"`
	DailyBaseCostUpperBoundUSD *string                             `json:"dailyBaseCostUpperBoundUsd"`
	PriceUnavailable           bool                                `json:"priceUnavailable"`
	MeasurementVersion         int64                               `json:"measurementVersion"`
	Version                    int64                               `json:"version"`
	ReferencedServices         []apiProbeConnectionServiceResponse `json:"referencedServices"`
	HealthSummary              apiServiceHealthSummaryResponse     `json:"healthSummary"`
	CreatedAt                  time.Time                           `json:"createdAt"`
	UpdatedAt                  time.Time                           `json:"updatedAt"`
}

type apiProbeConnectionPreflightResponse struct {
	ErrorCode                  *string  `json:"errorCode"`
	AvailableModels            []string `json:"availableModels"`
	ProbeModel                 *string  `json:"probeModel"`
	ProbeProtocol              *string  `json:"probeProtocol"`
	ProbeEnvironment           string   `json:"probeEnvironment"`
	DailyBaseCostUpperBoundUSD *string  `json:"dailyBaseCostUpperBoundUsd"`
	PriceUnavailable           bool     `json:"priceUnavailable"`
	PreflightToken             *string  `json:"preflightToken"`
}

func (server *Server) handlePreflightOwnerAPIProbeConnection(w http.ResponseWriter, request *http.Request) {
	setAPIHealthPrivateHeaders(w)
	user, _, appErr := server.requireSessionAndCSRF(w, request)
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	if !requireCapability(w, request, user, auth.CapabilityAPIProbeManage) {
		return
	}
	input, appErr := decodeStrictJSONOnly[apiProbeConnectionRequest](request)
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	if appErr = server.requireAPIHealthService(); appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	result, appErr := server.apiHealth.PreflightOwnerConnection(request.Context(), user, toAPIProbeConnectionInput(input))
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toAPIProbeConnectionPreflightResponse(result))
}

func (server *Server) handlePreflightExistingOwnerAPIProbeConnection(w http.ResponseWriter, request *http.Request) {
	setAPIHealthPrivateHeaders(w)
	user, _, appErr := server.requireSessionAndCSRF(w, request)
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	if !requireCapability(w, request, user, auth.CapabilityAPIProbeManage) {
		return
	}
	input, appErr := decodeStrictJSONOnly[apiProbeConnectionRequest](request)
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(request)
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	if appErr = server.requireAPIHealthService(); appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	result, appErr := server.apiHealth.PreflightExistingOwnerConnection(request.Context(), user, chi.URLParam(request, "id"), toAPIProbeConnectionInput(input), version)
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toAPIProbeConnectionPreflightResponse(result))
}

func (server *Server) handleOwnerAPIProbeConnections(w http.ResponseWriter, request *http.Request) {
	setAPIHealthPrivateHeaders(w)
	user, _, appErr := server.requireSession(w, request)
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	if !requireCapability(w, request, user, auth.CapabilityAPIProbeManage) {
		return
	}
	if appErr = server.requireAPIHealthService(); appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	connections, appErr := server.apiHealth.OwnerConnections(request.Context(), user)
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	items := make([]ownerAPIProbeConnectionResponse, 0, len(connections))
	for _, connection := range connections {
		items = append(items, toOwnerAPIProbeConnectionResponse(connection))
	}
	writeJSON(w, http.StatusOK, listResponse[ownerAPIProbeConnectionResponse]{Items: items})
}

func (server *Server) handleOwnerAPIProbeConnection(w http.ResponseWriter, request *http.Request) {
	setAPIHealthPrivateHeaders(w)
	user, _, appErr := server.requireSession(w, request)
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	if !requireCapability(w, request, user, auth.CapabilityAPIProbeManage) {
		return
	}
	if appErr = server.requireAPIHealthService(); appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	connection, found, appErr := server.apiHealth.OwnerConnection(request.Context(), user, chi.URLParam(request, "id"))
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	if !found {
		writeProblem(w, request, apiHealthProbeNotFoundError())
		return
	}
	setETag(w, connection.Version)
	writeJSON(w, http.StatusOK, toOwnerAPIProbeConnectionResponse(connection))
}

func (server *Server) handleCreateOwnerAPIProbeConnection(w http.ResponseWriter, request *http.Request) {
	setAPIHealthPrivateHeaders(w)
	user, _, appErr := server.requireSessionAndCSRF(w, request)
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	if !requireCapability(w, request, user, auth.CapabilityAPIProbeManage) {
		return
	}
	body, input, appErr := decodeStrictJSON[apiProbeConnectionRequest](request)
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	if input.Enabled == nil {
		writeProblem(w, request, probeEnabledRequiredError())
		return
	}
	if appErr = server.requireAPIHealthService(); appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	routeKey := "POST /api/v1/owner/api-probe-connections"
	completion, appErr := server.apiHealth.CreateOwnerConnectionWithIdempotency(
		request.Context(), user, routeKey, request.Header.Get("Idempotency-Key"),
		requestHash(request.Method, routeKey, body), toAPIProbeConnectionInput(input), requestIDFrom(request),
		func(connection apihealth.Connection) (idempotency.Completion, *domain.AppError) {
			return apiHealthIdempotencyCompletion(http.StatusCreated, toOwnerAPIProbeConnectionResponse(connection), connection.Version, connection.ID)
		},
	)
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	writeAPIHealthMutationCompletion(w, completion)
}

func (server *Server) handleUpdateOwnerAPIProbeConnection(w http.ResponseWriter, request *http.Request) {
	setAPIHealthPrivateHeaders(w)
	user, _, appErr := server.requireSessionAndCSRF(w, request)
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	if !requireCapability(w, request, user, auth.CapabilityAPIProbeManage) {
		return
	}
	body, input, appErr := decodeStrictJSON[apiProbeConnectionRequest](request)
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	if input.Enabled == nil {
		writeProblem(w, request, probeEnabledRequiredError())
		return
	}
	version, appErr := requireIfMatchVersion(request)
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	if appErr = server.requireAPIHealthService(); appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	connectionID := chi.URLParam(request, "id")
	routeKey := "PUT /api/v1/owner/api-probe-connections/{id}"
	completion, appErr := server.apiHealth.UpdateOwnerConnectionWithIdempotency(
		request.Context(), user, routeKey, request.Header.Get("Idempotency-Key"),
		apiHealthMutationRequestHash(request, routeKey, body, connectionID, version), connectionID,
		toAPIProbeConnectionInput(input), version, requestIDFrom(request),
		func(connection apihealth.Connection) (idempotency.Completion, *domain.AppError) {
			return apiHealthIdempotencyCompletion(http.StatusOK, toOwnerAPIProbeConnectionResponse(connection), connection.Version, connection.ID)
		},
	)
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	writeAPIHealthMutationCompletion(w, completion)
}

func (server *Server) handleDeleteOwnerAPIProbeConnection(w http.ResponseWriter, request *http.Request) {
	setAPIHealthPrivateHeaders(w)
	user, _, appErr := server.requireSessionAndCSRF(w, request)
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	if !requireCapability(w, request, user, auth.CapabilityAPIProbeManage) {
		return
	}
	version, appErr := requireIfMatchVersion(request)
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	if appErr = server.requireAPIHealthService(); appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	connectionID := chi.URLParam(request, "id")
	routeKey := "DELETE /api/v1/owner/api-probe-connections/{id}"
	completion, appErr := server.apiHealth.DeleteOwnerConnectionWithIdempotency(
		request.Context(), user, routeKey, request.Header.Get("Idempotency-Key"),
		apiHealthMutationRequestHash(request, routeKey, nil, connectionID, version), connectionID,
		version, requestIDFrom(request), func(connection apihealth.Connection) (idempotency.Completion, *domain.AppError) {
			return idempotency.Completion{
				Status: http.StatusNoContent, ResourceType: "api_probe_connection", ResourceID: connection.ID,
			}, nil
		},
	)
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	writeAPIHealthMutationCompletion(w, completion)
}

func (server *Server) handleVerifyOwnerAPIProbeConnection(w http.ResponseWriter, request *http.Request) {
	setAPIHealthPrivateHeaders(w)
	user, _, appErr := server.requireSessionAndCSRF(w, request)
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	if !requireCapability(w, request, user, auth.CapabilityAPIProbeManage) {
		return
	}
	version, appErr := requireIfMatchVersion(request)
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	if appErr = server.requireAPIHealthService(); appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	connectionID := chi.URLParam(request, "id")
	routeKey := "POST /api/v1/owner/api-probe-connections/{id}/verify"
	completion, appErr := server.apiHealth.VerifyOwnerConnectionWithIdempotency(
		request.Context(), user, routeKey, request.Header.Get("Idempotency-Key"),
		apiHealthMutationRequestHash(request, routeKey, nil, connectionID, version), connectionID,
		version, requestIDFrom(request), func(connection apihealth.Connection) (idempotency.Completion, *domain.AppError) {
			return apiHealthIdempotencyCompletion(http.StatusOK, toOwnerAPIProbeConnectionResponse(connection), connection.Version, connection.ID)
		},
	)
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	writeAPIHealthMutationCompletion(w, completion)
}

func apiHealthMutationRequestHash(request *http.Request, routeKey string, body []byte, connectionID string, version int64) string {
	prefix := []byte(connectionID + "\n" + strconv.FormatInt(version, 10) + "\n")
	payload := make([]byte, 0, len(prefix)+len(body))
	payload = append(payload, prefix...)
	payload = append(payload, body...)
	return requestHash(request.Method, routeKey, payload)
}

func writeAPIHealthMutationCompletion(w http.ResponseWriter, completion idempotency.Completion) {
	if completion.Status == http.StatusNoContent {
		completion.ContentType = ""
		completion.Body = nil
	}
	restoreAPIHealthETag(&completion)
	writeNoStoreIdempotencyCompletion(w, completion)
}

func (server *Server) withAPIHealthIdempotency(
	w http.ResponseWriter,
	request *http.Request,
	userID string,
	routeKey string,
	body []byte,
	run func() (idempotency.Completion, *domain.AppError),
) {
	entry, appErr := server.app.BeginIdempotency(request.Context(), userID, routeKey, request.Header.Get("Idempotency-Key"), requestHash(request.Method, routeKey, body))
	if appErr != nil {
		writeProblem(w, request, appErr)
		return
	}
	if entry.State == "completed" {
		completion := idempotency.CompletionFromEntry(entry)
		restoreAPIHealthETag(&completion)
		writeNoStoreIdempotencyCompletion(w, completion)
		return
	}
	completion, appErr := run()
	if appErr != nil {
		server.app.CancelIdempotency(request.Context(), entry)
		writeProblem(w, request, appErr)
		return
	}
	if appErr := server.app.CompleteIdempotency(request.Context(), entry, completion.Status, completion.ContentType, completion.Body, completion.ResourceType, completion.ResourceID); appErr != nil {
		server.app.CancelIdempotency(request.Context(), entry)
		writeProblem(w, request, appErr)
		return
	}
	writeNoStoreIdempotencyCompletion(w, completion)
}

func apiHealthIdempotencyCompletion(status int, payload any, version int64, resourceID string) (idempotency.Completion, *domain.AppError) {
	body, err := json.Marshal(payload)
	if err != nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	return idempotency.Completion{
		Status: status, ContentType: "application/json; charset=utf-8", Body: body,
		ResourceType: "api_probe_connection", ResourceID: resourceID,
		Headers: map[string]string{"ETag": `"` + strconv.FormatInt(version, 10) + `"`},
	}, nil
}

func restoreAPIHealthETag(completion *idempotency.Completion) {
	if completion == nil || len(completion.Body) == 0 || completion.Status < 200 || completion.Status >= 300 {
		return
	}
	var payload struct {
		Version int64 `json:"version"`
	}
	if err := json.Unmarshal(completion.Body, &payload); err != nil || payload.Version <= 0 {
		return
	}
	if completion.Headers == nil {
		completion.Headers = make(map[string]string)
	}
	completion.Headers["ETag"] = `"` + strconv.FormatInt(payload.Version, 10) + `"`
}

func toAPIProbeConnectionInput(request apiProbeConnectionRequest) apihealth.ConnectionInput {
	return apihealth.ConnectionInput{
		Name: request.Name, BaseURL: request.BaseURL, Credential: request.Credential, ProbeModel: request.ProbeModel,
		PreflightToken: request.PreflightToken, Enabled: request.Enabled != nil && *request.Enabled,
		AcknowledgeInsecureHTTP: request.AcknowledgeInsecureHTTP,
	}
}

func toOwnerAPIProbeConnectionResponse(connection apihealth.Connection) ownerAPIProbeConnectionResponse {
	references := make([]apiProbeConnectionServiceResponse, 0, len(connection.References))
	for _, reference := range connection.References {
		references = append(references, apiProbeConnectionServiceResponse{ID: reference.ID, Title: reference.Title})
	}
	dailyCost := apiHealthOptionalString(apihealth.DailyBaseCostUpperBoundUSD(connection.Price))
	return ownerAPIProbeConnectionResponse{
		ID: connection.ID, Name: connection.Name, BaseURL: connection.BaseURL, NormalizedBaseURL: connection.NormalizedBaseURL,
		CredentialConfigured: connection.CredentialConfigured, Enabled: connection.Enabled,
		VerificationStatus: connection.VerificationStatus, VerifiedAt: connection.VerifiedAt,
		LastVerificationErrorCode: apiHealthStringPointer(connection.LastVerificationErrorCode),
		ProbeModel:                apiHealthOptionalString(connection.ProbeModel), ProbeProtocol: apiHealthOptionalString(connection.ProbeProtocol),
		AvailableModels: append([]string(nil), connection.AvailableModels...), ProbeEnvironment: connection.ProbeEnvironment,
		ProbeModelChangedAt: connection.ProbeModelChangedAt, DailyBaseCostUpperBoundUSD: dailyCost,
		PriceUnavailable:   dailyCost == nil,
		MeasurementVersion: connection.MeasurementVersion, Version: connection.Version,
		ReferencedServices: references, HealthSummary: toAPIServiceHealthSummaryResponse(connection.HealthSummary),
		CreatedAt: connection.CreatedAt, UpdatedAt: connection.UpdatedAt,
	}
}

func toAPIProbeConnectionPreflightResponse(result apihealth.PreflightResult) apiProbeConnectionPreflightResponse {
	return apiProbeConnectionPreflightResponse{
		ErrorCode:       apiHealthStringPointer(result.Verification.ErrorCode),
		AvailableModels: append([]string(nil), result.Verification.AvailableModels...),
		ProbeModel:      apiHealthOptionalString(result.Verification.ProbeModel), ProbeProtocol: apiHealthOptionalString(result.Verification.ProbeProtocol),
		ProbeEnvironment:           apihealth.ProbeEnvironmentUSWestV1,
		DailyBaseCostUpperBoundUSD: apiHealthOptionalString(result.DailyBaseCostUpperBoundUSD),
		PriceUnavailable:           result.PriceUnavailable,
		PreflightToken:             apiHealthOptionalString(result.PreflightToken),
	}
}

func (server *Server) requireAPIHealthService() *domain.AppError {
	if server.apiHealth != nil {
		return nil
	}
	return domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "API health unavailable", "探针服务暂时不可用。")
}

func apiHealthProbeNotFoundError() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Probe connection not found", "探针连接不存在。")
}

func probeEnabledRequiredError() *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Probe enabled state required", "必须明确指定是否启用探针连接。", "enabled", "required", "必须提供 enabled。")
}

func setAPIHealthPrivateHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "private, no-store")
}

func apiHealthStringPointer(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}
