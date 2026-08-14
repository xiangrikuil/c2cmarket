package server

import (
	"net/http"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/operationaudit"
)

type adminOperationAuditEntryDTO struct {
	ID            string  `json:"id"`
	SourceKind    string  `json:"sourceKind"`
	Domain        string  `json:"domain"`
	ActorKind     string  `json:"actorKind"`
	ActorUserID   *string `json:"actorUserId"`
	ActorUsername *string `json:"actorUsername"`
	Action        string  `json:"action"`
	ActionLabel   string  `json:"actionLabel"`
	TargetType    string  `json:"targetType"`
	TargetID      string  `json:"targetId"`
	TargetLabel   *string `json:"targetLabel"`
	Outcome       string  `json:"outcome"`
	Summary       string  `json:"summary"`
	DetailPath    *string `json:"detailPath"`
	RequestID     string  `json:"requestId"`
	CreatedAt     string  `json:"createdAt"`
}

func (s *Server) handleAdminOperationAuditLogs(w http.ResponseWriter, r *http.Request) {
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
	values := r.URL.Query()
	page, appErr := s.operationAudit.AdminOperationAuditLogs(r.Context(), user, operationaudit.Filter{
		SourceKind:  values.Get("sourceKind"),
		Domain:      values.Get("domain"),
		Action:      values.Get("action"),
		ActorKind:   values.Get("actorKind"),
		ActorUserID: values.Get("actorUserId"),
		TargetType:  values.Get("targetType"),
		TargetID:    values.Get("targetId"),
		Outcome:     values.Get("outcome"),
		From:        values.Get("from"),
		To:          values.Get("to"),
		Search:      values.Get("search"),
		Limit:       pageRequest.Limit,
		Cursor:      pageRequest.Cursor,
	})
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items := make([]adminOperationAuditEntryDTO, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, toAdminOperationAuditEntryDTO(item))
	}
	writePageJSON(w, domain.Page[adminOperationAuditEntryDTO]{Items: items, NextCursor: page.NextCursor})
}

func toAdminOperationAuditEntryDTO(item operationaudit.Entry) adminOperationAuditEntryDTO {
	return adminOperationAuditEntryDTO{
		ID:            item.ID,
		SourceKind:    item.SourceKind,
		Domain:        item.Domain,
		ActorKind:     item.ActorKind,
		ActorUserID:   nullableOperationAuditString(item.ActorUserID),
		ActorUsername: nullableOperationAuditString(item.ActorUsername),
		Action:        item.Action,
		ActionLabel:   item.ActionLabel,
		TargetType:    item.TargetType,
		TargetID:      item.TargetID,
		TargetLabel:   nullableOperationAuditString(item.TargetLabel),
		Outcome:       item.Outcome,
		Summary:       item.Summary,
		DetailPath:    nullableOperationAuditString(item.DetailPath),
		RequestID:     item.RequestID,
		CreatedAt:     item.CreatedAt.UTC().Format(timeLayoutRFC3339),
	}
}

func nullableOperationAuditString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
