package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apihealth"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/go-chi/chi/v5"
)

type apiHealthProbeConfigRequest struct {
	BaseURL    string  `json:"baseUrl"`
	Model      string  `json:"model"`
	Credential *string `json:"credential"`
	Enabled    *bool   `json:"enabled"`
}

type apiHealthProbeChallengeRequest struct {
	Method string `json:"method"`
}

type ownerAPIHealthProbeConfigResponse struct {
	ID                   string     `json:"id"`
	APIServiceID         string     `json:"apiServiceId"`
	Protocol             string     `json:"protocol"`
	BaseURL              string     `json:"baseUrl"`
	NormalizedOrigin     string     `json:"normalizedOrigin"`
	Model                string     `json:"model"`
	CredentialConfigured bool       `json:"credentialConfigured"`
	Enabled              bool       `json:"enabled"`
	AuthorizationStatus  string     `json:"authorizationStatus"`
	AuthorizationMethod  *string    `json:"authorizationMethod"`
	VerifiedOrigin       *string    `json:"verifiedOrigin"`
	VerifiedAt           *time.Time `json:"verifiedAt"`
	ApprovedAt           *time.Time `json:"approvedAt"`
	RejectionReason      *string    `json:"rejectionReason"`
	ChallengeExpiresAt   *time.Time `json:"challengeExpiresAt"`
	MeasurementVersion   int64      `json:"measurementVersion"`
	LastConfigErrorCode  *string    `json:"lastConfigErrorCode"`
	Version              int64      `json:"version"`
	CreatedAt            time.Time  `json:"createdAt"`
	UpdatedAt            time.Time  `json:"updatedAt"`
}

type apiHealthProbeChallengeResponse struct {
	Token         string    `json:"token"`
	Method        string    `json:"method"`
	DNSRecordName *string   `json:"dnsRecordName"`
	HTTPURL       *string   `json:"httpUrl"`
	ExpiresAt     time.Time `json:"expiresAt"`
	ConfigVersion int64     `json:"configVersion"`
}

func (s *Server) handleOwnerAPIHealthProbe(w http.ResponseWriter, r *http.Request) {
	setAPIHealthPrivateHeaders(w)
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if appErr = s.requireAPIHealthService(); appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	config, found, appErr := s.apiHealth.OwnerConfig(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if !found {
		writeProblem(w, r, apiHealthProbeNotFoundError())
		return
	}
	setETag(w, config.Version)
	writeJSON(w, http.StatusOK, toOwnerAPIHealthProbeConfigResponse(config))
}

func (s *Server) handlePutOwnerAPIHealthProbe(w http.ResponseWriter, r *http.Request) {
	setAPIHealthPrivateHeaders(w)
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	request, appErr := decodeStrictJSONOnly[apiHealthProbeConfigRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if request.Enabled == nil {
		writeProblem(w, r, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Probe enabled state required", "必须明确指定是否启用探针。", "enabled", "required", "必须提供 enabled。"))
		return
	}
	version, appErr := requireIfMatchVersionAllowZero(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if appErr = s.requireAPIHealthService(); appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	config, appErr := s.apiHealth.PutOwnerConfig(r.Context(), user, chi.URLParam(r, "id"), apihealth.ConfigInput{
		BaseURL: request.BaseURL, Model: request.Model, Credential: request.Credential, Enabled: *request.Enabled,
	}, version)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, config.Version)
	writeJSON(w, http.StatusOK, toOwnerAPIHealthProbeConfigResponse(config))
}

func (s *Server) handleDeleteOwnerAPIHealthProbe(w http.ResponseWriter, r *http.Request) {
	setAPIHealthPrivateHeaders(w)
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if appErr = s.requireAPIHealthService(); appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if appErr := s.apiHealth.DeleteOwnerConfig(r.Context(), user, chi.URLParam(r, "id"), version); appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCreateOwnerAPIHealthProbeChallenge(w http.ResponseWriter, r *http.Request) {
	setAPIHealthPrivateHeaders(w)
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, request, appErr := decodeStrictJSON[apiHealthProbeChallengeRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if appErr = s.requireAPIHealthService(); appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	serviceID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/owner/api-services/{id}/health-probe/challenges:" + serviceID
	s.withAPIHealthIdempotency(w, r, user.ID, routeKey, body, func() (idempotency.Completion, *domain.AppError) {
		challenge, appErr := s.apiHealth.CreateChallenge(r.Context(), user, serviceID, request.Method, version)
		if appErr != nil {
			return idempotency.Completion{}, appErr
		}
		return apiHealthIdempotencyCompletion(
			http.StatusCreated,
			toAPIHealthProbeChallengeResponse(challenge),
			challenge.ConfigVersion,
			serviceID,
		)
	})
}

func (s *Server) handleVerifyOwnerAPIHealthProbe(w http.ResponseWriter, r *http.Request) {
	setAPIHealthPrivateHeaders(w)
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if appErr = s.requireAPIHealthService(); appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	serviceID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/owner/api-services/{id}/health-probe/verify:" + serviceID
	s.withAPIHealthIdempotency(w, r, user.ID, routeKey, nil, func() (idempotency.Completion, *domain.AppError) {
		config, appErr := s.apiHealth.VerifyChallenge(r.Context(), user, serviceID, version)
		if appErr != nil {
			return idempotency.Completion{}, appErr
		}
		return apiHealthIdempotencyCompletion(
			http.StatusOK,
			toOwnerAPIHealthProbeConfigResponse(config),
			config.Version,
			config.ID,
		)
	})
}

func (s *Server) withAPIHealthIdempotency(
	w http.ResponseWriter,
	r *http.Request,
	userID string,
	routeKey string,
	body []byte,
	run func() (idempotency.Completion, *domain.AppError),
) {
	entry, appErr := s.app.BeginIdempotency(
		r.Context(),
		userID,
		routeKey,
		r.Header.Get("Idempotency-Key"),
		requestHash(r.Method, routeKey, body),
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
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
		s.app.CancelIdempotency(r.Context(), entry)
		writeProblem(w, r, appErr)
		return
	}
	if appErr := s.app.CompleteIdempotency(
		r.Context(),
		entry,
		completion.Status,
		completion.ContentType,
		completion.Body,
		completion.ResourceType,
		completion.ResourceID,
	); appErr != nil {
		s.app.CancelIdempotency(r.Context(), entry)
		writeProblem(w, r, appErr)
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
		Status:       status,
		ContentType:  "application/json; charset=utf-8",
		Body:         body,
		ResourceType: "api_health_probe_config",
		ResourceID:   resourceID,
		Headers: map[string]string{
			"ETag": `"` + strconv.FormatInt(version, 10) + `"`,
		},
	}, nil
}

func restoreAPIHealthETag(completion *idempotency.Completion) {
	if completion == nil || len(completion.Body) == 0 || completion.Status < 200 || completion.Status >= 300 {
		return
	}
	var payload struct {
		Version       int64 `json:"version"`
		ConfigVersion int64 `json:"configVersion"`
	}
	if err := json.Unmarshal(completion.Body, &payload); err != nil {
		return
	}
	version := payload.Version
	if version == 0 {
		version = payload.ConfigVersion
	}
	if version <= 0 {
		return
	}
	if completion.Headers == nil {
		completion.Headers = make(map[string]string)
	}
	completion.Headers["ETag"] = `"` + strconv.FormatInt(version, 10) + `"`
}

func toOwnerAPIHealthProbeConfigResponse(config apihealth.Config) ownerAPIHealthProbeConfigResponse {
	return ownerAPIHealthProbeConfigResponse{
		ID: config.ID, APIServiceID: config.APIServiceID, Protocol: config.Protocol,
		BaseURL: config.BaseURL, NormalizedOrigin: config.NormalizedOrigin, Model: config.Model,
		CredentialConfigured: config.CredentialConfigured, Enabled: config.Enabled,
		AuthorizationStatus: config.AuthorizationStatus, AuthorizationMethod: apiHealthStringPointer(config.AuthorizationMethod),
		VerifiedOrigin: apiHealthStringPointer(config.VerifiedOrigin), VerifiedAt: config.VerifiedAt,
		ApprovedAt: config.ApprovedAt, RejectionReason: apiHealthStringPointer(config.RejectionReason),
		ChallengeExpiresAt: config.ChallengeExpiresAt, MeasurementVersion: config.MeasurementVersion,
		LastConfigErrorCode: apiHealthStringPointer(config.LastConfigErrorCode), Version: config.Version,
		CreatedAt: config.CreatedAt, UpdatedAt: config.UpdatedAt,
	}
}

func toAPIHealthProbeChallengeResponse(challenge apihealth.Challenge) apiHealthProbeChallengeResponse {
	return apiHealthProbeChallengeResponse{
		Token: challenge.Token, Method: challenge.Method,
		DNSRecordName: apiHealthStringPointer(challenge.DNSRecordName), HTTPURL: apiHealthStringPointer(challenge.HTTPURL),
		ExpiresAt: challenge.ExpiresAt, ConfigVersion: challenge.ConfigVersion,
	}
}

func (s *Server) requireAPIHealthService() *domain.AppError {
	if s.apiHealth != nil {
		return nil
	}
	return domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "API health unavailable", "探针服务暂时不可用。")
}

func apiHealthProbeNotFoundError() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Probe config not found", "探针配置不存在。")
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
