package server

import (
	"net/http"
	"time"

	"c2c-market/backend/internal/module/navigationbadge"
)

type navigationBadgeRoleDTO struct {
	CarpoolActions  int `json:"carpoolActions"`
	APIOrderActions int `json:"apiOrderActions"`
}

type navigationBadgeAdminDTO struct {
	Total           int `json:"total"`
	OfficialPrices  int `json:"officialPrices"`
	Carpools        int `json:"carpools"`
	APIServices     int `json:"apiServices"`
	FeedbackTickets int `json:"feedbackTickets"`
	Reports         int `json:"reports"`
}

type navigationBadgeSummaryDTO struct {
	GeneratedAt                 string                   `json:"generatedAt"`
	NotificationUnread          int                      `json:"notificationUnread"`
	ImportantAnnouncementUnread int                      `json:"importantAnnouncementUnread"`
	FeedbackUnread              int                      `json:"feedbackUnread"`
	Buyer                       navigationBadgeRoleDTO   `json:"buyer"`
	Merchant                    navigationBadgeRoleDTO   `json:"merchant"`
	Admin                       *navigationBadgeAdminDTO `json:"admin"`
}

func (s *Server) handleNavigationBadges(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	summary, appErr := s.navigationBadges.Get(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toNavigationBadgeSummaryDTO(summary))
}

func toNavigationBadgeSummaryDTO(summary navigationbadge.Summary) navigationBadgeSummaryDTO {
	result := navigationBadgeSummaryDTO{
		GeneratedAt:                 summary.GeneratedAt.UTC().Format(time.RFC3339),
		NotificationUnread:          summary.NotificationUnread,
		ImportantAnnouncementUnread: summary.ImportantAnnouncementUnread,
		FeedbackUnread:              summary.FeedbackUnread,
		Buyer: navigationBadgeRoleDTO{
			CarpoolActions:  summary.Buyer.CarpoolActions,
			APIOrderActions: summary.Buyer.APIOrderActions,
		},
		Merchant: navigationBadgeRoleDTO{
			CarpoolActions:  summary.Merchant.CarpoolActions,
			APIOrderActions: summary.Merchant.APIOrderActions,
		},
	}
	if summary.Admin != nil {
		result.Admin = &navigationBadgeAdminDTO{
			Total:           summary.Admin.Total,
			OfficialPrices:  summary.Admin.OfficialPrices,
			Carpools:        summary.Admin.Carpools,
			APIServices:     summary.Admin.APIServices,
			FeedbackTickets: summary.Admin.FeedbackTickets,
			Reports:         summary.Admin.Reports,
		}
	}
	return result
}
