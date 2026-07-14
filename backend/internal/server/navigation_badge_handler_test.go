package server

import (
	"testing"
	"time"

	"c2c-market/backend/internal/module/navigationbadge"
)

func TestNavigationBadgeSummaryDTOKeepsAdminOptional(t *testing.T) {
	summary := navigationbadge.Summary{
		GeneratedAt:                 time.Date(2026, 7, 11, 8, 0, 0, 0, time.UTC),
		NotificationUnread:          3,
		ImportantAnnouncementUnread: 2,
		FeedbackUnread:              1,
		Buyer:                       navigationbadge.RoleActions{CarpoolActions: 4, APIOrderActions: 5},
		Merchant:                    navigationbadge.RoleActions{CarpoolActions: 6, APIOrderActions: 7},
	}
	dto := toNavigationBadgeSummaryDTO(summary)
	if dto.Admin != nil {
		t.Fatalf("non-admin DTO exposed admin counts: %#v", dto.Admin)
	}
	if dto.GeneratedAt != "2026-07-11T08:00:00Z" || dto.NotificationUnread != 3 || dto.Buyer.APIOrderActions != 5 {
		t.Fatalf("unexpected navigation badge DTO: %#v", dto)
	}

	summary.Admin = &navigationbadge.AdminCounts{Total: 15, OfficialPrices: 1, Carpools: 2, APIServices: 3, FeedbackTickets: 4, Reports: 5}
	dto = toNavigationBadgeSummaryDTO(summary)
	if dto.Admin == nil || dto.Admin.Total != 15 || dto.Admin.Reports != 5 {
		t.Fatalf("admin DTO missing counts: %#v", dto.Admin)
	}
}
