package server

import (
	"encoding/json"
	"net/http"
	"strings"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/communityidentity"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/go-chi/chi/v5"
)

type adminCommunityIdentityListResponse struct {
	Items []adminCommunityIdentityDTO `json:"items"`
}

type adminCommunityIdentityDTO struct {
	ID           string  `json:"id"`
	Code         string  `json:"code"`
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	Source       string  `json:"source"`
	QualifiedAt  *string `json:"qualifiedAt,omitempty"`
	GrantedAt    string  `json:"grantedAt"`
	GrantedBy    string  `json:"grantedBy,omitempty"`
	GrantReason  string  `json:"grantReason,omitempty"`
	RevokedAt    *string `json:"revokedAt,omitempty"`
	RevokedBy    string  `json:"revokedBy,omitempty"`
	RevokeReason string  `json:"revokeReason,omitempty"`
}

type communityIdentityMutationRequest struct {
	IdentityType string `json:"identityType,omitempty"`
	Reason       string `json:"reason"`
}

func (s *Server) handleAdminCommunityIdentities(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items, appErr := s.communityIdentity.AdminCommunityIdentities(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	response := adminCommunityIdentityListResponse{Items: make([]adminCommunityIdentityDTO, 0, len(items))}
	for _, item := range items {
		response.Items = append(response.Items, toAdminCommunityIdentityDTO(item))
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleGrantAdminCommunityIdentity(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, request, appErr := decodeStrictJSON[communityIdentityMutationRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	targetUserID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/admin/users/{id}/community-identities:" + targetUserID
	completion, appErr := s.communityIdentity.GrantCommunityIdentityWithIdempotency(
		r.Context(), user, routeKey, r.Header.Get("Idempotency-Key"), requestHash(r.Method, routeKey, body),
		communityidentity.GrantAdminInput{
			TargetUserID: targetUserID,
			Type:         communityidentity.IdentityType(strings.TrimSpace(request.IdentityType)),
			Reason:       request.Reason,
			RequestID:    requestIDFrom(r),
		},
		communityIdentityMutationCompletionBuilder,
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleRevokeAdminCommunityIdentity(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, request, appErr := decodeStrictJSON[communityIdentityMutationRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	targetUserID := chi.URLParam(r, "id")
	identityType := communityidentity.IdentityType(strings.TrimSpace(chi.URLParam(r, "identityType")))
	routeKey := "POST /api/v1/admin/users/{id}/community-identities/{identityType}/revoke:" + targetUserID + ":" + string(identityType)
	completion, appErr := s.communityIdentity.RevokeCommunityIdentityWithIdempotency(
		r.Context(), user, routeKey, r.Header.Get("Idempotency-Key"), requestHash(r.Method, routeKey, body),
		communityidentity.RevokeInput{
			TargetUserID: targetUserID,
			Type:         identityType,
			Reason:       request.Reason,
			RequestID:    requestIDFrom(r),
		},
		communityIdentityMutationCompletionBuilder,
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func communityIdentityMutationCompletionBuilder(item communityidentity.Identity) (idempotency.Completion, *domain.AppError) {
	body, err := json.Marshal(toAdminCommunityIdentityDTO(item))
	if err != nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "社区身份响应编码失败。")
	}
	return idempotency.Completion{
		Status:       http.StatusOK,
		ContentType:  "application/json; charset=utf-8",
		Body:         body,
		ResourceType: "community_identity",
		ResourceID:   item.ID,
	}, nil
}

func toAdminCommunityIdentityDTO(item communityidentity.Identity) adminCommunityIdentityDTO {
	definition, _ := communityidentity.Definition(item.Type)
	return adminCommunityIdentityDTO{
		ID:           item.ID,
		Code:         string(item.Type),
		Name:         definition.Name,
		Description:  definition.Description,
		Source:       string(item.Source),
		QualifiedAt:  formatOptionalTime(item.QualifiedAt),
		GrantedAt:    item.GrantedAt.UTC().Format(timeLayoutRFC3339),
		GrantedBy:    item.GrantedBy,
		GrantReason:  item.GrantReason,
		RevokedAt:    formatOptionalTime(item.RevokedAt),
		RevokedBy:    item.RevokedBy,
		RevokeReason: item.RevokeReason,
	}
}
