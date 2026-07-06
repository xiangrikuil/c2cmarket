package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/report"

	"github.com/go-chi/chi/v5"
)

type createReportRequest struct {
	TargetType       string `json:"targetType"`
	TargetID         string `json:"targetId"`
	TargetLabel      string `json:"targetLabel"`
	ReportedUsername string `json:"reportedUsername"`
	ReasonCode       string `json:"reasonCode"`
	Title            string `json:"title"`
	Description      string `json:"description"`
}

type createAppealRequest struct {
	ReportID   string `json:"reportId"`
	DisputeID  string `json:"disputeId"`
	TargetType string `json:"targetType"`
	TargetID   string `json:"targetId"`
	Title      string `json:"title"`
	Statement  string `json:"statement"`
}

type reportActionRequest struct {
	Reason           string `json:"reason"`
	PublicSummary    string `json:"publicSummary"`
	PublicResultCode string `json:"publicResultCode"`
	PublicResult     string `json:"publicResult"`
}

type reportResponse struct {
	ID                  string  `json:"id"`
	ReporterUserID      string  `json:"reporterUserId,omitempty"`
	ReporterUsername    string  `json:"reporterUsername"`
	ReporterName        string  `json:"reporterName"`
	TargetType          string  `json:"targetType"`
	TargetID            string  `json:"targetId"`
	CanonicalTargetType string  `json:"canonicalTargetType"`
	CanonicalTargetID   string  `json:"canonicalTargetId"`
	TargetLabel         string  `json:"targetLabel"`
	TargetSnapshotJSON  string  `json:"targetSnapshotJson,omitempty"`
	ReportedUsername    string  `json:"reportedUsername"`
	ReasonCode          string  `json:"reasonCode"`
	Title               string  `json:"title"`
	Description         string  `json:"description,omitempty"`
	Status              string  `json:"status"`
	AdminReason         string  `json:"adminReason,omitempty"`
	HandledByAdminID    string  `json:"handledByAdminId,omitempty"`
	HandledAt           *string `json:"handledAt,omitempty"`
	DisputeID           string  `json:"disputeId,omitempty"`
	CreatedAt           string  `json:"createdAt"`
	UpdatedAt           string  `json:"updatedAt"`
	Version             int64   `json:"version"`
}

type disputeResponse struct {
	ID                   string  `json:"id"`
	ReportID             string  `json:"reportId,omitempty"`
	TargetType           string  `json:"targetType"`
	TargetID             string  `json:"targetId"`
	TargetLabel          string  `json:"targetLabel"`
	PrimaryUserID        string  `json:"primaryUserId,omitempty"`
	PrimaryUsername      string  `json:"primaryUsername"`
	PrimaryDisplayName   string  `json:"primaryDisplayName"`
	CounterpartyUserID   string  `json:"counterpartyUserId,omitempty"`
	CounterpartyUsername string  `json:"counterpartyUsername"`
	CounterpartyName     string  `json:"counterpartyName"`
	Status               string  `json:"status"`
	PublicSummary        string  `json:"publicSummary"`
	PublicResultCode     string  `json:"publicResultCode"`
	PublicResult         string  `json:"publicResult"`
	AdminReason          string  `json:"adminReason,omitempty"`
	OpenedByAdminID      string  `json:"openedByAdminId,omitempty"`
	OpenedAt             string  `json:"openedAt"`
	ResolvedAt           *string `json:"resolvedAt,omitempty"`
	ClosedAt             *string `json:"closedAt,omitempty"`
	CreatedAt            string  `json:"createdAt"`
	UpdatedAt            string  `json:"updatedAt"`
	Version              int64   `json:"version"`
}

type appealResponse struct {
	ID                string  `json:"id"`
	AppellantUserID   string  `json:"appellantUserId,omitempty"`
	AppellantUsername string  `json:"appellantUsername"`
	AppellantName     string  `json:"appellantName"`
	ReportID          string  `json:"reportId,omitempty"`
	DisputeID         string  `json:"disputeId,omitempty"`
	TargetType        string  `json:"targetType"`
	TargetID          string  `json:"targetId"`
	Title             string  `json:"title"`
	Statement         string  `json:"statement,omitempty"`
	Status            string  `json:"status"`
	AdminReason       string  `json:"adminReason,omitempty"`
	HandledByAdminID  string  `json:"handledByAdminId,omitempty"`
	HandledAt         *string `json:"handledAt,omitempty"`
	CreatedAt         string  `json:"createdAt"`
	UpdatedAt         string  `json:"updatedAt"`
	Version           int64   `json:"version"`
}

type publicDisputeResponse struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	Type       string `json:"type"`
	Result     string `json:"result"`
	HandledAt  string `json:"handledAt"`
	Unresolved bool   `json:"unresolved"`
}

type adminMutationResponse struct {
	Report  *reportResponse  `json:"report,omitempty"`
	Dispute *disputeResponse `json:"dispute,omitempty"`
	Appeal  *appealResponse  `json:"appeal,omitempty"`
}

func (s *Server) handleCreateReport(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[createReportRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	completion, appErr := s.app.CreateReportWithIdempotency(
		r.Context(),
		user,
		"POST /api/v1/reports",
		r.Header.Get("Idempotency-Key"),
		requestHash(r.Method, "POST /api/v1/reports", body),
		report.CreateReportInput{
			TargetType:       req.TargetType,
			TargetID:         req.TargetID,
			TargetLabel:      req.TargetLabel,
			ReportedUsername: req.ReportedUsername,
			ReasonCode:       req.ReasonCode,
			Title:            req.Title,
			Description:      req.Description,
		},
		reportCompletionBuilder(http.StatusCreated, false),
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleMyReports(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items, appErr := s.app.MyReports(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toReportResponses(items, false))
}

func (s *Server) handleCreateAppeal(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[createAppealRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	routeKey := "POST /api/v1/me/appeals"
	completion, appErr := s.app.CreateAppealWithIdempotency(
		r.Context(),
		user,
		routeKey,
		r.Header.Get("Idempotency-Key"),
		requestHash(r.Method, routeKey, body),
		report.CreateAppealInput{
			ReportID:   req.ReportID,
			DisputeID:  req.DisputeID,
			TargetType: req.TargetType,
			TargetID:   req.TargetID,
			Title:      req.Title,
			Statement:  req.Statement,
		},
		appealCompletionBuilder(http.StatusCreated, false),
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleMyAppeals(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items, appErr := s.app.MyAppeals(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toAppealResponses(items, false))
}

func (s *Server) handlePublicUserDisputes(w http.ResponseWriter, r *http.Request) {
	items, appErr := s.app.PublicUserDisputes(r.Context(), chi.URLParam(r, "username"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[publicDisputeResponse]{Items: toPublicDisputeResponses(items)})
}

func (s *Server) handleAdminReports(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	pageRequest, appErr := parsePageRequest(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items, appErr := s.app.AdminReports(r.Context(), user, pageRequest)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePageJSON(w, domain.Page[reportResponse]{
		Items:      toReportResponses(items.Items, true),
		NextCursor: items.NextCursor,
	})
}

func (s *Server) handleAdminReport(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	item, appErr := s.app.AdminReport(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, item.Version)
	writeJSON(w, http.StatusOK, toReportResponse(item, true))
}

func (s *Server) handleTriageReport(w http.ResponseWriter, r *http.Request) {
	s.handleAdminReportAction(w, r, "triage")
}

func (s *Server) handleRequestReportInfo(w http.ResponseWriter, r *http.Request) {
	s.handleAdminReportAction(w, r, "request_info")
}

func (s *Server) handleRejectReport(w http.ResponseWriter, r *http.Request) {
	s.handleAdminReportAction(w, r, "reject")
}

func (s *Server) handleOpenReportDispute(w http.ResponseWriter, r *http.Request) {
	s.handleAdminReportAction(w, r, "open_dispute")
}

func (s *Server) handleCloseReport(w http.ResponseWriter, r *http.Request) {
	s.handleAdminReportAction(w, r, "close")
}

func (s *Server) handleAdminReportAction(w http.ResponseWriter, r *http.Request, action string) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[reportActionRequest](r)
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
	routeKey := "POST /api/v1/admin/reports/{id}/" + action + ":" + id
	completion, appErr := s.app.AdminReportActionWithIdempotency(r.Context(), user, routeKey, r.Header.Get("Idempotency-Key"), requestHash(r.Method, routeKey, body), report.AdminActionInput{
		ID:               id,
		Action:           action,
		Reason:           req.Reason,
		PublicSummary:    req.PublicSummary,
		PublicResultCode: req.PublicResultCode,
		PublicResult:     req.PublicResult,
		ExpectedVersion:  version,
		RequestID:        requestIDFrom(r),
	}, adminMutationCompletionBuilder)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleAdminDisputes(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items, appErr := s.app.AdminDisputes(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toDisputeResponses(items, true))
}

func (s *Server) handleAdminDispute(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	item, appErr := s.app.AdminDispute(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, item.Version)
	writeJSON(w, http.StatusOK, toDisputeResponse(item, true))
}

func (s *Server) handleRequestDisputeInfo(w http.ResponseWriter, r *http.Request) {
	s.handleAdminDisputeAction(w, r, "request_info")
}

func (s *Server) handleResolveDispute(w http.ResponseWriter, r *http.Request) {
	s.handleAdminDisputeAction(w, r, "resolve")
}

func (s *Server) handleCloseDispute(w http.ResponseWriter, r *http.Request) {
	s.handleAdminDisputeAction(w, r, "close")
}

func (s *Server) handleAdminDisputeAction(w http.ResponseWriter, r *http.Request, action string) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[reportActionRequest](r)
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
	routeKey := "POST /api/v1/admin/disputes/{id}/" + action + ":" + id
	completion, appErr := s.app.AdminDisputeActionWithIdempotency(r.Context(), user, routeKey, r.Header.Get("Idempotency-Key"), requestHash(r.Method, routeKey, body), report.AdminActionInput{
		ID:               id,
		Action:           action,
		Reason:           req.Reason,
		PublicSummary:    req.PublicSummary,
		PublicResultCode: req.PublicResultCode,
		PublicResult:     req.PublicResult,
		ExpectedVersion:  version,
		RequestID:        requestIDFrom(r),
	}, adminMutationCompletionBuilder)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleAdminAppeals(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items, appErr := s.app.AdminAppeals(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toAppealResponses(items, true))
}

func (s *Server) handleAdminAppeal(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	item, appErr := s.app.AdminAppeal(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, item.Version)
	writeJSON(w, http.StatusOK, toAppealResponse(item, true))
}

func (s *Server) handleApproveAppeal(w http.ResponseWriter, r *http.Request) {
	s.handleAdminAppealAction(w, r, "approve")
}

func (s *Server) handleRejectAppeal(w http.ResponseWriter, r *http.Request) {
	s.handleAdminAppealAction(w, r, "reject")
}

func (s *Server) handleAdminAppealAction(w http.ResponseWriter, r *http.Request, action string) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[reportActionRequest](r)
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
	routeKey := "POST /api/v1/admin/appeals/{id}/" + action + ":" + id
	completion, appErr := s.app.AdminAppealActionWithIdempotency(r.Context(), user, routeKey, r.Header.Get("Idempotency-Key"), requestHash(r.Method, routeKey, body), report.AdminActionInput{
		ID:              id,
		Action:          action,
		Reason:          req.Reason,
		ExpectedVersion: version,
		RequestID:       requestIDFrom(r),
	}, adminMutationCompletionBuilder)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func reportCompletionBuilder(status int, includeAdmin bool) report.ReportCompletionBuilder {
	return func(item report.Report) (idempotency.Completion, *domain.AppError) {
		body, err := json.Marshal(toReportResponse(item, includeAdmin))
		if err != nil {
			return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
		}
		return idempotency.Completion{
			Status:       status,
			ContentType:  "application/json; charset=utf-8",
			Body:         body,
			ResourceType: "report",
			ResourceID:   item.ID,
			Headers:      map[string]string{"ETag": `"` + strconv.FormatInt(item.Version, 10) + `"`},
		}, nil
	}
}

func appealCompletionBuilder(status int, includeAdmin bool) report.AppealCompletionBuilder {
	return func(item report.Appeal) (idempotency.Completion, *domain.AppError) {
		body, err := json.Marshal(toAppealResponse(item, includeAdmin))
		if err != nil {
			return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
		}
		return idempotency.Completion{
			Status:       status,
			ContentType:  "application/json; charset=utf-8",
			Body:         body,
			ResourceType: "appeal",
			ResourceID:   item.ID,
			Headers:      map[string]string{"ETag": `"` + strconv.FormatInt(item.Version, 10) + `"`},
		}, nil
	}
}

func adminMutationCompletionBuilder(result report.MutationResult) (idempotency.Completion, *domain.AppError) {
	payload := toAdminMutationResponse(result)
	body, err := json.Marshal(payload)
	if err != nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	headers := map[string]string{}
	resourceType := "moderation_action"
	resourceID := ""
	if result.Report != nil {
		resourceType = "report"
		resourceID = result.Report.ID
		headers["ETag"] = `"` + strconv.FormatInt(result.Report.Version, 10) + `"`
	}
	if result.Dispute != nil {
		resourceType = "dispute"
		resourceID = result.Dispute.ID
		headers["ETag"] = `"` + strconv.FormatInt(result.Dispute.Version, 10) + `"`
	}
	if result.Appeal != nil {
		resourceType = "appeal"
		resourceID = result.Appeal.ID
		headers["ETag"] = `"` + strconv.FormatInt(result.Appeal.Version, 10) + `"`
	}
	return idempotency.Completion{
		Status:       http.StatusOK,
		ContentType:  "application/json; charset=utf-8",
		Body:         body,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Headers:      headers,
	}, nil
}

func toReportResponses(items []report.Report, includeAdmin bool) []reportResponse {
	result := make([]reportResponse, 0, len(items))
	for _, item := range items {
		result = append(result, toReportResponse(item, includeAdmin))
	}
	return result
}

func toReportResponse(item report.Report, includeAdmin bool) reportResponse {
	response := reportResponse{
		ID:                  item.ID,
		ReporterUsername:    item.ReporterUsername,
		ReporterName:        item.ReporterName,
		TargetType:          item.TargetType,
		TargetID:            item.TargetID,
		CanonicalTargetType: item.CanonicalTargetType,
		CanonicalTargetID:   item.CanonicalTargetID,
		TargetLabel:         item.TargetLabel,
		ReportedUsername:    item.ReportedUsername,
		ReasonCode:          item.ReasonCode,
		Title:               item.Title,
		Status:              item.Status,
		HandledAt:           formatOptionalTime(item.HandledAt),
		DisputeID:           item.DisputeID,
		CreatedAt:           item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:           item.UpdatedAt.UTC().Format(time.RFC3339),
		Version:             item.Version,
	}
	if includeAdmin {
		response.ReporterUserID = item.ReporterUserID
		response.Description = item.Description
		response.TargetSnapshotJSON = item.TargetSnapshotJSON
		response.AdminReason = item.AdminReason
		response.HandledByAdminID = item.HandledByAdminID
	}
	return response
}

func toDisputeResponses(items []report.DisputeCase, includeAdmin bool) []disputeResponse {
	result := make([]disputeResponse, 0, len(items))
	for _, item := range items {
		result = append(result, toDisputeResponse(item, includeAdmin))
	}
	return result
}

func toDisputeResponse(item report.DisputeCase, includeAdmin bool) disputeResponse {
	response := disputeResponse{
		ID:                   item.ID,
		ReportID:             item.ReportID,
		TargetType:           item.TargetType,
		TargetID:             item.TargetID,
		TargetLabel:          item.TargetLabel,
		PrimaryUsername:      item.PrimaryUsername,
		PrimaryDisplayName:   item.PrimaryDisplayName,
		CounterpartyUsername: item.CounterpartyUsername,
		CounterpartyName:     item.CounterpartyName,
		Status:               item.Status,
		PublicSummary:        item.PublicSummary,
		PublicResultCode:     item.PublicResultCode,
		PublicResult:         item.PublicResult,
		OpenedAt:             item.OpenedAt.UTC().Format(time.RFC3339),
		ResolvedAt:           formatOptionalTime(item.ResolvedAt),
		ClosedAt:             formatOptionalTime(item.ClosedAt),
		CreatedAt:            item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:            item.UpdatedAt.UTC().Format(time.RFC3339),
		Version:              item.Version,
	}
	if includeAdmin {
		response.PrimaryUserID = item.PrimaryUserID
		response.CounterpartyUserID = item.CounterpartyUserID
		response.AdminReason = item.AdminReason
		response.OpenedByAdminID = item.OpenedByAdminID
	}
	return response
}

func toAppealResponses(items []report.Appeal, includeAdmin bool) []appealResponse {
	result := make([]appealResponse, 0, len(items))
	for _, item := range items {
		result = append(result, toAppealResponse(item, includeAdmin))
	}
	return result
}

func toAppealResponse(item report.Appeal, includeAdmin bool) appealResponse {
	response := appealResponse{
		ID:                item.ID,
		AppellantUsername: item.AppellantUsername,
		AppellantName:     item.AppellantName,
		ReportID:          item.ReportID,
		DisputeID:         item.DisputeID,
		TargetType:        item.TargetType,
		TargetID:          item.TargetID,
		Title:             item.Title,
		Status:            item.Status,
		HandledAt:         formatOptionalTime(item.HandledAt),
		CreatedAt:         item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:         item.UpdatedAt.UTC().Format(time.RFC3339),
		Version:           item.Version,
	}
	if includeAdmin {
		response.AppellantUserID = item.AppellantUserID
		response.Statement = item.Statement
		response.AdminReason = item.AdminReason
		response.HandledByAdminID = item.HandledByAdminID
	}
	return response
}

func toPublicDisputeResponses(items []report.PublicDispute) []publicDisputeResponse {
	result := make([]publicDisputeResponse, 0, len(items))
	for _, item := range items {
		result = append(result, publicDisputeResponse{
			ID:         item.ID,
			Username:   item.Username,
			Type:       item.Type,
			Result:     item.Result,
			HandledAt:  item.HandledAt.UTC().Format("2006-01-02"),
			Unresolved: item.Unresolved,
		})
	}
	return result
}

func toAdminMutationResponse(result report.MutationResult) adminMutationResponse {
	response := adminMutationResponse{}
	if result.Report != nil {
		item := toReportResponse(*result.Report, true)
		response.Report = &item
	}
	if result.Dispute != nil {
		item := toDisputeResponse(*result.Dispute, true)
		response.Dispute = &item
	}
	if result.Appeal != nil {
		item := toAppealResponse(*result.Appeal, true)
		response.Appeal = &item
	}
	return response
}
