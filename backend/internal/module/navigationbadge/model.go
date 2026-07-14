package navigationbadge

import "time"

type RoleActions struct {
	CarpoolActions  int
	APIOrderActions int
}

type AdminCounts struct {
	Total           int
	OfficialPrices  int
	Carpools        int
	APIServices     int
	FeedbackTickets int
	Reports         int
}

func (c AdminCounts) ActionableTotal() int {
	return c.OfficialPrices + c.Carpools + c.APIServices + c.FeedbackTickets + c.Reports
}

type Summary struct {
	GeneratedAt                 time.Time
	NotificationUnread          int
	ImportantAnnouncementUnread int
	FeedbackUnread              int
	Buyer                       RoleActions
	Merchant                    RoleActions
	Admin                       *AdminCounts
}
