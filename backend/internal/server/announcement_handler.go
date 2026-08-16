package server

import (
	"net/http"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/announcement"

	"github.com/go-chi/chi/v5"
)

type announcementDTO struct {
	ID               string                  `json:"id"`
	Slug             string                  `json:"slug"`
	Title            string                  `json:"title"`
	Summary          string                  `json:"summary"`
	ContentMarkdown  string                  `json:"contentMarkdown"`
	Category         string                  `json:"category"`
	Level            string                  `json:"level"`
	Status           string                  `json:"status"`
	Channels         []string                `json:"channels"`
	Audience         announcementAudienceDTO `json:"audience"`
	IsPinned         bool                    `json:"isPinned"`
	IsDismissible    bool                    `json:"isDismissible"`
	RequiresAck      bool                    `json:"requiresAck"`
	CTALabel         *string                 `json:"ctaLabel,omitempty"`
	CTAURL           *string                 `json:"ctaUrl,omitempty"`
	PublishAt        string                  `json:"publishAt"`
	ExpireAt         *string                 `json:"expireAt,omitempty"`
	ContentUpdatedAt string                  `json:"contentUpdatedAt"`
	Version          int64                   `json:"version"`
	CreatedBy        string                  `json:"createdBy"`
	UpdatedBy        string                  `json:"updatedBy"`
	CreatedAt        string                  `json:"createdAt"`
	UpdatedAt        string                  `json:"updatedAt"`
	Receipt          *announcementReceiptDTO `json:"receipt,omitempty"`
}

type announcementAudienceDTO struct {
	Type    string   `json:"type"`
	Roles   []string `json:"roles,omitempty"`
	UserIDs []string `json:"userIds,omitempty"`
}

type announcementReceiptDTO struct {
	AnnouncementID      string  `json:"announcementId"`
	AnnouncementVersion int64   `json:"announcementVersion"`
	FirstSeenAt         *string `json:"firstSeenAt,omitempty"`
	ReadAt              *string `json:"readAt,omitempty"`
	DismissedAt         *string `json:"dismissedAt,omitempty"`
	AcknowledgedAt      *string `json:"acknowledgedAt,omitempty"`
}

type announcementFormRequest struct {
	Title           string                  `json:"title"`
	Summary         string                  `json:"summary"`
	ContentMarkdown string                  `json:"contentMarkdown"`
	Category        string                  `json:"category"`
	Level           string                  `json:"level"`
	Channels        []string                `json:"channels"`
	IsPinned        bool                    `json:"isPinned"`
	IsDismissible   bool                    `json:"isDismissible"`
	RequiresAck     bool                    `json:"requiresAck"`
	Audience        announcementAudienceDTO `json:"audience"`
	CTALabel        string                  `json:"ctaLabel"`
	CTAURL          string                  `json:"ctaUrl"`
	PublishAt       string                  `json:"publishAt"`
	ExpireAt        string                  `json:"expireAt"`
}

type announcementOfflineRequest struct {
	Reason string `json:"reason"`
}

type countResponse struct {
	Count int `json:"count"`
}

type publicAnnouncementDTO struct {
	ID               string                  `json:"id"`
	Slug             string                  `json:"slug"`
	Title            string                  `json:"title"`
	Summary          string                  `json:"summary"`
	ContentMarkdown  string                  `json:"contentMarkdown"`
	Category         string                  `json:"category"`
	Level            string                  `json:"level"`
	Channels         []string                `json:"channels"`
	Audience         announcementAudienceDTO `json:"audience"`
	IsPinned         bool                    `json:"isPinned"`
	IsDismissible    bool                    `json:"isDismissible"`
	RequiresAck      bool                    `json:"requiresAck"`
	CTALabel         *string                 `json:"ctaLabel,omitempty"`
	CTAURL           *string                 `json:"ctaUrl,omitempty"`
	PublishAt        string                  `json:"publishAt"`
	ExpireAt         *string                 `json:"expireAt,omitempty"`
	ContentUpdatedAt string                  `json:"contentUpdatedAt"`
	Version          int64                   `json:"version"`
}

type announcementAuditLogDTO struct {
	ID                string  `json:"id"`
	Action            string  `json:"action"`
	AnnouncementID    string  `json:"announcementId"`
	AnnouncementTitle string  `json:"announcementTitle"`
	OperatorID        string  `json:"operatorId"`
	OperatorName      string  `json:"operatorName"`
	Reason            *string `json:"reason,omitempty"`
	CreatedAt         string  `json:"createdAt"`
}

func (s *Server) handleAnnouncements(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items, appErr := s.app.UserAnnouncements(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[announcementDTO]{Items: toAnnouncementDTOs(items)})
}

func (s *Server) handleActiveAnnouncements(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items, appErr := s.app.ActiveAnnouncements(r.Context(), user, r.URL.Query().Get("channel"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[announcementDTO]{Items: toAnnouncementDTOs(items)})
}

func (s *Server) handlePublicActiveAnnouncements(w http.ResponseWriter, r *http.Request) {
	items, appErr := s.app.PublicActiveAnnouncements(r.Context(), r.URL.Query().Get("channel"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	result := make([]publicAnnouncementDTO, 0, len(items))
	for _, item := range items {
		result = append(result, toPublicAnnouncementDTO(item))
	}
	writeJSON(w, http.StatusOK, listResponse[publicAnnouncementDTO]{Items: result})
}

func (s *Server) handleHomeAnnouncement(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	item, appErr := s.app.HomeAnnouncement(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if item == nil {
		writeJSON(w, http.StatusOK, nil)
		return
	}
	writeJSON(w, http.StatusOK, toAnnouncementDTO(*item))
}

func (s *Server) handleAnnouncementDetail(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	item, appErr := s.app.UserAnnouncementBySlug(r.Context(), user, chi.URLParam(r, "slug"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toAnnouncementDTO(item))
}

func (s *Server) handleAnnouncementUnreadCount(w http.ResponseWriter, r *http.Request) {
	s.handleAnnouncementCount(w, r, false)
}

func (s *Server) handleImportantAnnouncementUnreadCount(w http.ResponseWriter, r *http.Request) {
	s.handleAnnouncementCount(w, r, true)
}

func (s *Server) handleAnnouncementCount(w http.ResponseWriter, r *http.Request, importantOnly bool) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	count, appErr := s.app.AnnouncementUnreadCount(r.Context(), user, importantOnly)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, countResponse{Count: count})
}

func (s *Server) handleMarkAnnouncementSeen(w http.ResponseWriter, r *http.Request) {
	s.handleAnnouncementReceipt(w, r, "seen")
}

func (s *Server) handleMarkAnnouncementRead(w http.ResponseWriter, r *http.Request) {
	s.handleAnnouncementReceipt(w, r, "read")
}

func (s *Server) handleDismissAnnouncement(w http.ResponseWriter, r *http.Request) {
	s.handleAnnouncementReceipt(w, r, "dismiss")
}

func (s *Server) handleAcknowledgeAnnouncement(w http.ResponseWriter, r *http.Request) {
	s.handleAnnouncementReceipt(w, r, "acknowledge")
}

func (s *Server) handleAnnouncementReceipt(w http.ResponseWriter, r *http.Request, action string) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	var receipt announcement.Receipt
	switch action {
	case "seen":
		receipt, appErr = s.app.MarkAnnouncementSeen(r.Context(), user, chi.URLParam(r, "id"))
	case "read":
		receipt, appErr = s.app.MarkAnnouncementRead(r.Context(), user, chi.URLParam(r, "id"))
	case "dismiss":
		receipt, appErr = s.app.DismissAnnouncement(r.Context(), user, chi.URLParam(r, "id"))
	case "acknowledge":
		receipt, appErr = s.app.AcknowledgeAnnouncement(r.Context(), user, chi.URLParam(r, "id"))
	}
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toAnnouncementReceiptDTO(receipt))
}

func (s *Server) handleAdminAnnouncements(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items, appErr := s.app.AdminAnnouncements(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toAdminAnnouncementDTOs(filterAdminAnnouncements(r, items)))
}

func (s *Server) handleAdminAnnouncement(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	item, appErr := s.app.AdminAnnouncement(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toAdminAnnouncementDTO(item))
}

func (s *Server) handleCreateAnnouncement(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	req, appErr := decodeStrictJSONOnly[announcementFormRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	input, appErr := announcementFormFromRequest(req)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	item, appErr := s.app.CreateAnnouncement(r.Context(), user, input)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusCreated, toAdminAnnouncementDTO(item))
}

func (s *Server) handleUpdateAnnouncement(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	req, appErr := decodeStrictJSONOnly[announcementFormRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	input, appErr := announcementFormFromRequest(req)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	item, appErr := s.app.UpdateAnnouncement(r.Context(), user, chi.URLParam(r, "id"), input)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toAdminAnnouncementDTO(item))
}

func (s *Server) handlePublishAnnouncement(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	item, appErr := s.app.PublishAnnouncement(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toAdminAnnouncementDTO(item))
}

func (s *Server) handleOfflineAnnouncement(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	req, appErr := decodeStrictJSONOnly[announcementOfflineRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	item, appErr := s.app.OfflineAnnouncement(r.Context(), user, chi.URLParam(r, "id"), req.Reason)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toAdminAnnouncementDTO(item))
}

func (s *Server) handleDuplicateAnnouncement(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	item, appErr := s.app.DuplicateAnnouncement(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusCreated, toAdminAnnouncementDTO(item))
}

func (s *Server) handleAnnouncementAuditLogs(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	logs, appErr := s.app.AnnouncementAuditLogs(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items := make([]announcementAuditLogDTO, 0, len(logs))
	for _, log := range logs {
		items = append(items, toAnnouncementAuditLogDTO(log))
	}
	writeJSON(w, http.StatusOK, listResponse[announcementAuditLogDTO]{Items: items})
}

func announcementFormFromRequest(req announcementFormRequest) (announcement.FormInput, *domain.AppError) {
	publishAt, appErr := parseRequiredTime(req.PublishAt, "publishAt")
	if appErr != nil {
		return announcement.FormInput{}, appErr
	}
	var expireAt *time.Time
	if req.ExpireAt != "" {
		parsed, appErr := parseRequiredTime(req.ExpireAt, "expireAt")
		if appErr != nil {
			return announcement.FormInput{}, appErr
		}
		expireAt = &parsed
	}
	return announcement.FormInput{
		Title:           req.Title,
		Summary:         req.Summary,
		ContentMarkdown: req.ContentMarkdown,
		Category:        req.Category,
		Level:           req.Level,
		Channels:        req.Channels,
		IsPinned:        req.IsPinned,
		IsDismissible:   req.IsDismissible,
		RequiresAck:     req.RequiresAck,
		Audience: announcement.Audience{
			Type:    req.Audience.Type,
			Roles:   req.Audience.Roles,
			UserIDs: req.Audience.UserIDs,
		},
		CTALabel:  req.CTALabel,
		CTAURL:    req.CTAURL,
		PublishAt: publishAt,
		ExpireAt:  expireAt,
	}, nil
}

func toAnnouncementDTOs(items []announcement.Announcement) []announcementDTO {
	result := make([]announcementDTO, 0, len(items))
	for _, item := range items {
		result = append(result, toAnnouncementDTO(item))
	}
	return result
}

func toAdminAnnouncementDTOs(items []announcement.Announcement) []announcementDTO {
	result := make([]announcementDTO, 0, len(items))
	for _, item := range items {
		result = append(result, toAdminAnnouncementDTO(item))
	}
	return result
}

func toAnnouncementDTO(item announcement.Announcement) announcementDTO {
	return announcementDTO{
		ID:               item.ID,
		Slug:             item.Slug,
		Title:            item.Title,
		Summary:          item.Summary,
		ContentMarkdown:  item.ContentMarkdown,
		Category:         item.Category,
		Level:            item.Level,
		Status:           item.Status,
		Channels:         item.Channels,
		Audience:         announcementAudienceDTO{Type: item.Audience.Type, Roles: item.Audience.Roles},
		IsPinned:         item.IsPinned,
		IsDismissible:    item.IsDismissible,
		RequiresAck:      item.RequiresAck,
		CTALabel:         optionalString(item.CTALabel),
		CTAURL:           optionalString(item.CTAURL),
		PublishAt:        item.PublishAt.UTC().Format(time.RFC3339),
		ExpireAt:         formatOptionalTime(item.ExpireAt),
		ContentUpdatedAt: item.ContentUpdatedAt.UTC().Format(time.RFC3339),
		Version:          item.Version,
		CreatedBy:        item.CreatedBy,
		UpdatedBy:        item.UpdatedBy,
		CreatedAt:        item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:        item.UpdatedAt.UTC().Format(time.RFC3339),
		Receipt:          toAnnouncementReceiptDTOPtr(item.Receipt),
	}
}

func toAdminAnnouncementDTO(item announcement.Announcement) announcementDTO {
	dto := toAnnouncementDTO(item)
	dto.Audience.UserIDs = item.Audience.UserIDs
	return dto
}

func toPublicAnnouncementDTO(item announcement.Announcement) publicAnnouncementDTO {
	return publicAnnouncementDTO{
		ID: item.ID, Slug: item.Slug, Title: item.Title, Summary: item.Summary,
		ContentMarkdown: item.ContentMarkdown, Category: item.Category, Level: item.Level,
		Channels: item.Channels, Audience: announcementAudienceDTO{Type: announcement.AudienceAll},
		IsPinned: item.IsPinned, IsDismissible: item.IsDismissible, RequiresAck: item.RequiresAck,
		CTALabel: optionalString(item.CTALabel), CTAURL: optionalString(item.CTAURL),
		PublishAt: item.PublishAt.UTC().Format(time.RFC3339), ExpireAt: formatOptionalTime(item.ExpireAt),
		ContentUpdatedAt: item.ContentUpdatedAt.UTC().Format(time.RFC3339), Version: item.Version,
	}
}

func toAnnouncementReceiptDTO(receipt announcement.Receipt) *announcementReceiptDTO {
	return toAnnouncementReceiptDTOPtr(&receipt)
}

func toAnnouncementReceiptDTOPtr(receipt *announcement.Receipt) *announcementReceiptDTO {
	if receipt == nil {
		return nil
	}
	return &announcementReceiptDTO{
		AnnouncementID:      receipt.AnnouncementID,
		AnnouncementVersion: receipt.AnnouncementVersion,
		FirstSeenAt:         formatOptionalTime(receipt.FirstSeenAt),
		ReadAt:              formatOptionalTime(receipt.ReadAt),
		DismissedAt:         formatOptionalTime(receipt.DismissedAt),
		AcknowledgedAt:      formatOptionalTime(receipt.AcknowledgedAt),
	}
}

func toAnnouncementAuditLogDTO(log announcement.AuditLog) announcementAuditLogDTO {
	return announcementAuditLogDTO{
		ID:                log.ID,
		Action:            log.Action,
		AnnouncementID:    log.AnnouncementID,
		AnnouncementTitle: log.AnnouncementTitle,
		OperatorID:        log.OperatorID,
		OperatorName:      log.OperatorName,
		Reason:            optionalString(log.Reason),
		CreatedAt:         log.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
