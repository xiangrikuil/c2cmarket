package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/feedback"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/go-chi/chi/v5"
)

type createFeedbackRequest struct {
	Type               string `json:"type"`
	Impact             string `json:"impact"`
	Title              string `json:"title"`
	Description        string `json:"description"`
	ContextPageLabel   string `json:"contextPageLabel"`
	ContextTargetType  string `json:"contextTargetType"`
	ContextTargetID    string `json:"contextTargetId"`
	ContextTargetLabel string `json:"contextTargetLabel"`
	ContextRoleLabel   string `json:"contextRoleLabel"`
}

type feedbackSupplementRequest struct {
	Message string `json:"message"`
}

type adminFeedbackHandleRequest struct {
	Status       string `json:"status"`
	Response     string `json:"response"`
	InternalNote string `json:"internalNote"`
}

type feedbackTicketResponse struct {
	ID                  string                  `json:"id"`
	SubmitterUserID     string                  `json:"submitterUserId,omitempty"`
	SubmitterUsername   string                  `json:"submitterUsername,omitempty"`
	SubmitterName       string                  `json:"submitterName"`
	Type                string                  `json:"type"`
	Impact              string                  `json:"impact"`
	Status              string                  `json:"status"`
	Title               string                  `json:"title"`
	Description         string                  `json:"description"`
	ContextPageLabel    string                  `json:"contextPageLabel"`
	ContextTargetType   string                  `json:"contextTargetType"`
	ContextTargetID     string                  `json:"contextTargetId"`
	ContextTargetLabel  string                  `json:"contextTargetLabel"`
	ContextRoleLabel    string                  `json:"contextRoleLabel"`
	AdminResponse       string                  `json:"adminResponse,omitempty"`
	AdminInternalNote   string                  `json:"adminInternalNote,omitempty"`
	HandledByAdminID    string                  `json:"handledByAdminId,omitempty"`
	HandledByAdminName  string                  `json:"handledByAdminName,omitempty"`
	HandledAt           *string                 `json:"handledAt,omitempty"`
	LatestAdminUpdateAt *string                 `json:"latestAdminUpdateAt,omitempty"`
	SubmitterReadAt     *string                 `json:"submitterReadAt,omitempty"`
	Unread              bool                    `json:"unread"`
	CreatedAt           string                  `json:"createdAt"`
	UpdatedAt           string                  `json:"updatedAt"`
	Version             int64                   `json:"version"`
	Events              []feedbackEventResponse `json:"events,omitempty"`
}

type feedbackEventResponse struct {
	ID            string `json:"id"`
	ActorUserID   string `json:"actorUserId,omitempty"`
	ActorName     string `json:"actorName"`
	ActorRole     string `json:"actorRole"`
	Action        string `json:"action"`
	PublicMessage string `json:"publicMessage"`
	InternalNote  string `json:"internalNote,omitempty"`
	CreatedAt     string `json:"createdAt"`
}

type feedbackUnreadCountResponse struct {
	Count int `json:"count"`
}

func (s *Server) handleCreateFeedbackTicket(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[createFeedbackRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	routeKey := "POST /api/v1/me/feedback-tickets"
	completion, appErr := s.app.CreateFeedbackTicketWithIdempotency(r.Context(), user, routeKey, r.Header.Get("Idempotency-Key"), requestHash(r.Method, routeKey, body), feedback.CreateInput{
		Type:               req.Type,
		Impact:             req.Impact,
		Title:              req.Title,
		Description:        req.Description,
		ContextPageLabel:   req.ContextPageLabel,
		ContextTargetType:  req.ContextTargetType,
		ContextTargetID:    req.ContextTargetID,
		ContextTargetLabel: req.ContextTargetLabel,
		ContextRoleLabel:   req.ContextRoleLabel,
		RequestID:          requestIDFrom(r),
	}, feedbackCompletionBuilder(http.StatusCreated, false))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleMyFeedbackTickets(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items, appErr := s.app.MyFeedbackTickets(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toFeedbackTicketResponses(items, false))
}

func (s *Server) handleMyFeedbackUnreadCount(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	count, appErr := s.app.MyFeedbackUnreadCount(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, feedbackUnreadCountResponse{Count: count})
}

func (s *Server) handleMyFeedbackTicket(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	item, appErr := s.app.MyFeedbackTicket(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, item.Version)
	writeJSON(w, http.StatusOK, toFeedbackTicketResponse(item, false))
}

func (s *Server) handleAddFeedbackSupplement(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[feedbackSupplementRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	id := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/me/feedback-tickets/{id}/supplements:" + id
	completion, appErr := s.app.AddFeedbackSupplementWithIdempotency(r.Context(), user, routeKey, r.Header.Get("Idempotency-Key"), requestHash(r.Method, routeKey, body), feedback.SupplementInput{
		ID:        id,
		Message:   req.Message,
		RequestID: requestIDFrom(r),
	}, feedbackCompletionBuilder(http.StatusOK, false))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleMarkFeedbackRead(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if _, appErr := decodeStrictJSONOnly[emptyRequest](r); appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	item, appErr := s.app.MarkFeedbackRead(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, item.Version)
	writeJSON(w, http.StatusOK, toFeedbackTicketResponse(item, false))
}

func (s *Server) handleAdminFeedbackTickets(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items, appErr := s.app.AdminFeedbackTickets(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toFeedbackTicketResponses(items, true))
}

func (s *Server) handleAdminFeedbackTicket(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	item, appErr := s.app.AdminFeedbackTicket(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, item.Version)
	writeJSON(w, http.StatusOK, toFeedbackTicketResponse(item, true))
}

func (s *Server) handleAdminFeedbackHandle(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[adminFeedbackHandleRequest](r)
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
	routeKey := "POST /api/v1/admin/feedback-tickets/{id}/handle:" + id
	completion, appErr := s.app.AdminHandleFeedbackTicketWithIdempotency(r.Context(), user, routeKey, r.Header.Get("Idempotency-Key"), requestHash(r.Method, routeKey, body), feedback.AdminHandleInput{
		ID:              id,
		Status:          req.Status,
		Response:        req.Response,
		InternalNote:    req.InternalNote,
		ExpectedVersion: version,
		RequestID:       requestIDFrom(r),
	}, feedbackCompletionBuilder(http.StatusOK, true))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func feedbackCompletionBuilder(status int, includeAdmin bool) feedback.CompletionBuilder {
	return func(item feedback.Ticket) (idempotency.Completion, *domain.AppError) {
		body, err := json.Marshal(toFeedbackTicketResponse(item, includeAdmin))
		if err != nil {
			return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
		}
		return idempotency.Completion{
			Status:       status,
			ContentType:  "application/json; charset=utf-8",
			Body:         body,
			ResourceType: "feedback_ticket",
			ResourceID:   item.ID,
			Headers:      map[string]string{"ETag": `"` + strconv.FormatInt(item.Version, 10) + `"`},
		}, nil
	}
}

func toFeedbackTicketResponses(items []feedback.Ticket, includeAdmin bool) []feedbackTicketResponse {
	result := make([]feedbackTicketResponse, 0, len(items))
	for _, item := range items {
		result = append(result, toFeedbackTicketResponse(item, includeAdmin))
	}
	return result
}

func toFeedbackTicketResponse(item feedback.Ticket, includeAdmin bool) feedbackTicketResponse {
	response := feedbackTicketResponse{
		ID:                  item.ID,
		SubmitterName:       item.SubmitterName,
		Type:                item.Type,
		Impact:              item.Impact,
		Status:              item.Status,
		Title:               item.Title,
		Description:         item.Description,
		ContextPageLabel:    item.ContextPageLabel,
		ContextTargetType:   item.ContextTargetType,
		ContextTargetID:     item.ContextTargetID,
		ContextTargetLabel:  item.ContextTargetLabel,
		ContextRoleLabel:    item.ContextRoleLabel,
		AdminResponse:       item.AdminResponse,
		HandledByAdminName:  item.HandledByAdminName,
		HandledAt:           formatOptionalTime(item.HandledAt),
		LatestAdminUpdateAt: formatOptionalTime(item.LatestAdminUpdateAt),
		SubmitterReadAt:     formatOptionalTime(item.SubmitterReadAt),
		Unread:              feedbackTicketUnread(item),
		CreatedAt:           item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:           item.UpdatedAt.UTC().Format(time.RFC3339),
		Version:             item.Version,
		Events:              toFeedbackEventResponses(item.Events, includeAdmin),
	}
	if includeAdmin {
		response.SubmitterUserID = item.SubmitterUserID
		response.SubmitterUsername = item.SubmitterUsername
		response.AdminInternalNote = item.AdminInternalNote
		response.HandledByAdminID = item.HandledByAdminID
	}
	return response
}

func toFeedbackEventResponses(items []feedback.Event, includeAdmin bool) []feedbackEventResponse {
	result := make([]feedbackEventResponse, 0, len(items))
	for _, item := range items {
		response := feedbackEventResponse{
			ID:            item.ID,
			ActorUserID:   item.ActorUserID,
			ActorName:     item.ActorName,
			ActorRole:     item.ActorRole,
			Action:        item.Action,
			PublicMessage: item.PublicMessage,
			CreatedAt:     item.CreatedAt.UTC().Format(time.RFC3339),
		}
		if includeAdmin {
			response.InternalNote = item.InternalNote
		}
		result = append(result, response)
	}
	return result
}

func feedbackTicketUnread(item feedback.Ticket) bool {
	if item.LatestAdminUpdateAt == nil {
		return false
	}
	return item.SubmitterReadAt == nil || item.SubmitterReadAt.Before(*item.LatestAdminUpdateAt)
}
