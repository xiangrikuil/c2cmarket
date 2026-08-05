package server

import (
	"net/http"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apihealth"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/go-chi/chi/v5"
)

type adminAPIHealthProbeDecisionRequest struct {
	Reason string `json:"reason"`
}

type adminAPIHealthProbeResponse struct {
	ID                  string     `json:"id"`
	APIServiceID        string     `json:"apiServiceId"`
	ServiceTitle        string     `json:"serviceTitle"`
	OwnerUserID         string     `json:"ownerUserId"`
	OwnerUsername       string     `json:"ownerUsername"`
	OwnerDisplayName    string     `json:"ownerDisplayName"`
	Protocol            string     `json:"protocol"`
	NormalizedOrigin    string     `json:"normalizedOrigin"`
	Model               string     `json:"model"`
	Enabled             bool       `json:"enabled"`
	AuthorizationStatus string     `json:"authorizationStatus"`
	AuthorizationMethod *string    `json:"authorizationMethod"`
	VerifiedOrigin      *string    `json:"verifiedOrigin"`
	VerifiedAt          *time.Time `json:"verifiedAt"`
	ApprovedAt          *time.Time `json:"approvedAt"`
	RejectionReason     *string    `json:"rejectionReason"`
	Version             int64      `json:"version"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

func (s *Server) handleAdminAPIHealthProbes(w http.ResponseWriter, r *http.Request) {
	setAPIHealthPrivateHeaders(w)
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	pageRequest, appErr := parsePageRequest(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if appErr = s.requireAPIHealthService(); appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	page, appErr := s.apiHealth.AdminConfigs(r.Context(), user, r.URL.Query().Get("status"), pageRequest)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items := make([]adminAPIHealthProbeResponse, 0, len(page.Items))
	for _, config := range page.Items {
		items = append(items, toAdminAPIHealthProbeResponse(config))
	}
	writeJSON(w, http.StatusOK, listResponse[adminAPIHealthProbeResponse]{Items: items, NextCursor: page.NextCursor})
}

func (s *Server) handleApproveAdminAPIHealthProbe(w http.ResponseWriter, r *http.Request) {
	s.handleAdminAPIHealthProbeDecision(w, r, true)
}

func (s *Server) handleRejectAdminAPIHealthProbe(w http.ResponseWriter, r *http.Request) {
	s.handleAdminAPIHealthProbeDecision(w, r, false)
}

func (s *Server) handleAdminAPIHealthProbeDecision(w http.ResponseWriter, r *http.Request, approve bool) {
	setAPIHealthPrivateHeaders(w)
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, request, appErr := decodeStrictJSON[adminAPIHealthProbeDecisionRequest](r)
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
	configID := chi.URLParam(r, "id")
	action := "reject"
	if approve {
		action = "approve"
	}
	routeKey := "POST /api/v1/admin/api-service-health-probes/{id}/" + action + ":" + configID
	s.withAPIHealthIdempotency(w, r, user.ID, routeKey, body, func() (idempotency.Completion, *domain.AppError) {
		config, appErr := s.apiHealth.AdminDecision(r.Context(), user, configID, version, approve, request.Reason)
		if appErr != nil {
			return idempotency.Completion{}, appErr
		}
		return apiHealthIdempotencyCompletion(
			http.StatusOK,
			toAdminAPIHealthProbeResponse(config),
			config.Version,
			config.ID,
		)
	})
}

func toAdminAPIHealthProbeResponse(config apihealth.Config) adminAPIHealthProbeResponse {
	return adminAPIHealthProbeResponse{
		ID: config.ID, APIServiceID: config.APIServiceID, ServiceTitle: config.ServiceTitle,
		OwnerUserID: config.OwnerUserID, OwnerUsername: config.OwnerUsername, OwnerDisplayName: config.OwnerDisplayName,
		Protocol: config.Protocol, NormalizedOrigin: config.NormalizedOrigin, Model: config.Model, Enabled: config.Enabled,
		AuthorizationStatus: config.AuthorizationStatus, AuthorizationMethod: apiHealthStringPointer(config.AuthorizationMethod),
		VerifiedOrigin: apiHealthStringPointer(config.VerifiedOrigin), VerifiedAt: config.VerifiedAt,
		ApprovedAt: config.ApprovedAt, RejectionReason: apiHealthStringPointer(config.RejectionReason),
		Version: config.Version, UpdatedAt: config.UpdatedAt,
	}
}
