package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/demand"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/go-chi/chi/v5"
)

type createDemandRequest struct {
	Title           string `json:"title"`
	MaxPriceCNY     string `json:"maxPriceCny"`
	RegionCode      string `json:"regionCode"`
	OwnerPreference string `json:"ownerPreference"`
	SourceURL       string `json:"sourceUrl"`
	Note            string `json:"note"`
}

type demandResponse struct {
	ID                string  `json:"id"`
	PublisherUserID   string  `json:"publisherUserId,omitempty"`
	PublisherUsername string  `json:"publisherUsername"`
	PublisherName     string  `json:"publisherName"`
	Title             string  `json:"title"`
	MaxPriceCNY       string  `json:"maxPriceCny"`
	RegionCode        string  `json:"regionCode"`
	OwnerPreference   string  `json:"ownerPreference"`
	SourceURL         string  `json:"sourceUrl"`
	Note              string  `json:"note"`
	Status            string  `json:"status"`
	ReviewReason      string  `json:"reviewReason,omitempty"`
	ReviewedByAdminID string  `json:"reviewedByAdminId,omitempty"`
	ReviewedAt        *string `json:"reviewedAt,omitempty"`
	ClosedAt          *string `json:"closedAt,omitempty"`
	CreatedAt         string  `json:"createdAt"`
	UpdatedAt         string  `json:"updatedAt"`
	Version           int64   `json:"version"`
}

func (s *Server) handleDemands(w http.ResponseWriter, r *http.Request) {
	items, appErr := s.app.PublicDemands(r.Context())
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toDemandResponses(items, false, false))
}

func (s *Server) handleDemand(w http.ResponseWriter, r *http.Request) {
	item, appErr := s.app.PublicDemand(r.Context(), chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, item.Version)
	writeJSON(w, http.StatusOK, toDemandResponse(item, false, false))
}

func (s *Server) handleCreateDemand(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[createDemandRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	s.withIdempotency(w, r, user.ID, "POST /api/v1/demands", body, func() (int, any, string, string, *domain.AppError) {
		item, errApp := s.app.CreateDemand(r.Context(), user, demand.CreateInput{
			Title:           req.Title,
			MaxPriceCNY:     req.MaxPriceCNY,
			RegionCode:      req.RegionCode,
			OwnerPreference: req.OwnerPreference,
			SourceURL:       req.SourceURL,
			Note:            req.Note,
		})
		if errApp != nil {
			return 0, nil, "", "", errApp
		}
		return http.StatusCreated, toDemandResponse(item, true, false), "demand", item.ID, nil
	})
}

func (s *Server) handleMyDemands(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items, appErr := s.app.MyDemands(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toDemandResponses(items, true, false))
}

func (s *Server) handleMyDemand(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	item, appErr := s.app.MyDemand(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, item.Version)
	writeJSON(w, http.StatusOK, toDemandResponse(item, true, false))
}

func (s *Server) handleCloseDemand(w http.ResponseWriter, r *http.Request) {
	s.handleOwnerDemandAction(w, r, "close")
}

func (s *Server) handleReopenDemand(w http.ResponseWriter, r *http.Request) {
	s.handleOwnerDemandAction(w, r, "reopen")
}

func (s *Server) handleOwnerDemandAction(w http.ResponseWriter, r *http.Request, action string) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, _, appErr := decodeStrictJSON[emptyRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	id := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/me/demands/{id}/" + action + ":" + id
	input := demand.OwnerActionInput{
		ID:              id,
		PublisherUserID: user.ID,
		ExpectedVersion: version,
		RequestID:       requestIDFrom(r),
	}
	var completion idempotency.Completion
	if action == "close" {
		completion, appErr = s.app.CloseDemandWithIdempotency(r.Context(), user.ID, routeKey, r.Header.Get("Idempotency-Key"), requestHash(r.Method, routeKey, body), input, demandCompletionBuilder(true, false))
	} else {
		completion, appErr = s.app.ReopenDemandWithIdempotency(r.Context(), user.ID, routeKey, r.Header.Get("Idempotency-Key"), requestHash(r.Method, routeKey, body), input, demandCompletionBuilder(true, false))
	}
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleAdminDemands(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items, appErr := s.app.AdminDemands(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toDemandResponses(items, true, true))
}

func (s *Server) handleAdminDemand(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	item, appErr := s.app.AdminDemand(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, item.Version)
	writeJSON(w, http.StatusOK, toDemandResponse(item, true, true))
}

func (s *Server) handleApproveDemand(w http.ResponseWriter, r *http.Request) {
	s.handleAdminDemandAction(w, r, "approve")
}

func (s *Server) handleRequestChangesDemand(w http.ResponseWriter, r *http.Request) {
	s.handleAdminDemandAction(w, r, "request_changes")
}

func (s *Server) handleRejectDemand(w http.ResponseWriter, r *http.Request) {
	s.handleAdminDemandAction(w, r, "reject")
}

func (s *Server) handleTakeDownDemand(w http.ResponseWriter, r *http.Request) {
	s.handleAdminDemandAction(w, r, "take_down")
}

func (s *Server) handleRestoreDemand(w http.ResponseWriter, r *http.Request) {
	s.handleAdminDemandAction(w, r, "restore")
}

func (s *Server) handleAdminDemandAction(w http.ResponseWriter, r *http.Request, action string) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if !user.IsAdmin {
		writeProblem(w, r, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。"))
		return
	}
	body, req, appErr := decodeStrictJSON[reviewActionRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	id := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/admin/demands/{id}/" + action + ":" + id
	completion, appErr := s.app.AdminDemandActionWithIdempotency(r.Context(), user.ID, routeKey, r.Header.Get("Idempotency-Key"), requestHash(r.Method, routeKey, body), demand.AdminActionInput{
		ID:              id,
		AdminUserID:     user.ID,
		Action:          action,
		Reason:          req.Reason,
		ExpectedVersion: version,
		RequestID:       requestIDFrom(r),
	}, demandCompletionBuilder(true, true))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func demandCompletionBuilder(includeReview, includeAdmin bool) demand.CompletionBuilder {
	return func(item demand.Demand) (idempotency.Completion, *domain.AppError) {
		body, err := json.Marshal(toDemandResponse(item, includeReview, includeAdmin))
		if err != nil {
			return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
		}
		return idempotency.Completion{
			Status:       http.StatusOK,
			ContentType:  "application/json; charset=utf-8",
			Body:         body,
			ResourceType: "demand",
			ResourceID:   item.ID,
			Headers:      map[string]string{"ETag": `"` + strconv.FormatInt(item.Version, 10) + `"`},
		}, nil
	}
}

func toDemandResponses(items []demand.Demand, includeReview, includeAdmin bool) []demandResponse {
	result := make([]demandResponse, 0, len(items))
	for _, item := range items {
		result = append(result, toDemandResponse(item, includeReview, includeAdmin))
	}
	return result
}

func toDemandResponse(item demand.Demand, includeReview, includeAdmin bool) demandResponse {
	var reviewedAt *string
	if includeReview && item.ReviewedAt != nil {
		formatted := item.ReviewedAt.UTC().Format(time.RFC3339)
		reviewedAt = &formatted
	}
	var closedAt *string
	if item.ClosedAt != nil {
		formatted := item.ClosedAt.UTC().Format(time.RFC3339)
		closedAt = &formatted
	}
	response := demandResponse{
		ID:                item.ID,
		PublisherUsername: item.PublisherUsername,
		PublisherName:     item.PublisherName,
		Title:             item.Title,
		MaxPriceCNY:       item.MaxPriceCNY,
		RegionCode:        item.RegionCode,
		OwnerPreference:   item.OwnerPreference,
		SourceURL:         item.SourceURL,
		Note:              item.Note,
		Status:            item.Status,
		ReviewedAt:        reviewedAt,
		ClosedAt:          closedAt,
		CreatedAt:         item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:         item.UpdatedAt.UTC().Format(time.RFC3339),
		Version:           item.Version,
	}
	if includeReview {
		response.ReviewReason = item.ReviewReason
	}
	if includeAdmin {
		response.PublisherUserID = item.PublisherUserID
		response.ReviewedByAdminID = item.ReviewedByAdminID
	}
	return response
}
