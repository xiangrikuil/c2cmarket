package server

import (
	"net/http"
	"time"

	"c2c-market/backend/internal/module/notification"

	"github.com/go-chi/chi/v5"
)

type notificationDTO struct {
	ID         string  `json:"id"`
	Type       string  `json:"type"`
	Title      string  `json:"title"`
	Detail     string  `json:"detail"`
	TargetType string  `json:"targetType"`
	TargetID   string  `json:"targetId"`
	To         string  `json:"to"`
	Unread     bool    `json:"unread"`
	ReadAt     *string `json:"readAt"`
	CreatedAt  string  `json:"createdAt"`
	Time       string  `json:"time"`
}

type notificationUnreadCountDTO struct {
	Count int `json:"count"`
}

type notificationReadAllDTO struct {
	Count int               `json:"count"`
	Items []notificationDTO `json:"items"`
}

func (s *Server) handleMyNotifications(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items, appErr := s.app.MyNotifications(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toNotificationDTOs(items))
}

func (s *Server) handleNotificationUnreadCount(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	count, appErr := s.app.MyNotificationUnreadCount(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, notificationUnreadCountDTO{Count: count})
}

func (s *Server) handleMarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if _, appErr := decodeStrictJSONOnly[emptyRequest](r); appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	item, appErr := s.app.MarkNotificationRead(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toNotificationDTO(item))
}

func (s *Server) handleMarkAllNotificationsRead(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if _, appErr := decodeStrictJSONOnly[emptyRequest](r); appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	result, appErr := s.app.MarkAllNotificationsRead(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, notificationReadAllDTO{
		Count: result.Count,
		Items: toNotificationDTOs(result.Items),
	})
}

func toNotificationDTOs(items []notification.Notification) []notificationDTO {
	result := make([]notificationDTO, 0, len(items))
	for _, item := range items {
		result = append(result, toNotificationDTO(item))
	}
	return result
}

func toNotificationDTO(item notification.Notification) notificationDTO {
	var readAt *string
	if item.ReadAt != nil {
		value := item.ReadAt.UTC().Format(time.RFC3339)
		readAt = &value
	}
	createdAt := item.CreatedAt.UTC().Format(time.RFC3339)
	to := notificationTargetURL(item)
	return notificationDTO{
		ID:         item.ID,
		Type:       notificationCategory(item),
		Title:      item.Title,
		Detail:     item.Body,
		TargetType: item.TargetType,
		TargetID:   item.TargetID,
		To:         to,
		Unread:     item.ReadAt == nil,
		ReadAt:     readAt,
		CreatedAt:  createdAt,
		Time:       createdAt,
	}
}

func notificationTargetURL(item notification.Notification) string {
	if item.TargetURL != "" {
		return item.TargetURL
	}
	switch item.TargetType {
	case "api_purchase_intent":
		if item.SourceEventType == "api_purchase_intent.created" || item.SourceEventType == "api_purchase_intent.buyer_cancelled" {
			return "/merchant/api-orders"
		}
		return "/my/api-orders/" + item.TargetID
	case "carpool_application":
		return "/my/rides/" + item.TargetID
	case "carpool_membership":
		return "/my/rides"
	case "official_price_lead":
		return "/official-prices"
	case "feedback_ticket":
		return "/my/feedback/" + item.TargetID
	default:
		return "/my/notifications"
	}
}

func notificationCategory(item notification.Notification) string {
	switch item.TargetType {
	case "api_purchase_intent":
		return "API 意向"
	case "carpool_application", "carpool_membership":
		return "上车申请"
	case "official_price_lead":
		return "审核结果"
	case "feedback_ticket":
		return "问题反馈"
	case "demand":
		return "求车需求"
	default:
		if item.Type != "" {
			return item.Type
		}
		return "管理操作"
	}
}
