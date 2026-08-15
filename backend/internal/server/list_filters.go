package server

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/announcement"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/carpool"
	"c2c-market/backend/internal/module/officialprice"
	"c2c-market/backend/internal/module/review"
)

func querySet(r *http.Request, key string) map[string]struct{} {
	values := map[string]struct{}{}
	for _, value := range queryValues(r, key) {
		values[value] = struct{}{}
	}
	return values
}

func queryValues(r *http.Request, key string) []string {
	values := make([]string, 0)
	for _, value := range strings.Split(r.URL.Query().Get(key), ",") {
		if value = strings.TrimSpace(value); value != "" {
			values = append(values, value)
		}
	}
	return values
}

func parseListQueryInteger(r *http.Request, field string) (int, *domain.AppError) {
	raw := strings.TrimSpace(r.URL.Query().Get(field))
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		detail := field + " 必须是整数。"
		return 0, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "List query invalid", detail, field, "invalid", detail)
	}
	return value, nil
}

func carpoolListingFilter(r *http.Request) carpool.ListingFilter {
	return carpool.ListingFilter{
		Query:          r.URL.Query().Get("q"),
		ProductPlanIDs: queryValues(r, "productPlanIds"),
		Region:         r.URL.Query().Get("region"),
		Statuses:       queryValues(r, "statuses"),
		View:           r.URL.Query().Get("view"),
		Risk:           r.URL.Query().Get("risk"),
		Sort:           r.URL.Query().Get("sort"),
		None:           r.URL.Query().Get("none") == "1",
	}
}

func adminAPIServiceFilter(r *http.Request) apimarket.AdminServiceFilter {
	return apimarket.AdminServiceFilter{
		Query:    r.URL.Query().Get("q"),
		View:     r.URL.Query().Get("view"),
		Statuses: queryValues(r, "statuses"),
		Risk:     r.URL.Query().Get("risk"),
	}
}

func setContains(values map[string]struct{}, value string) bool {
	if len(values) == 0 {
		return true
	}
	_, ok := values[value]
	return ok
}

func containsFold(query string, values ...string) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), query) {
			return true
		}
	}
	return false
}

func parseDecimalFloat(value string) float64 {
	parsed, _ := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return parsed
}

func matchesDateRange(createdAt time.Time, value string, now time.Time) bool {
	var duration time.Duration
	switch value {
	case "today":
		duration = 24 * time.Hour
	case "7d":
		duration = 7 * 24 * time.Hour
	case "30d":
		duration = 30 * 24 * time.Hour
	default:
		return true
	}
	return !createdAt.Before(now.Add(-duration))
}

func filterOfficialPriceRecords(r *http.Request, items []officialprice.Record) []officialprice.Record {
	if r.URL.Query().Get("none") == "1" {
		return []officialprice.Record{}
	}
	planIDs := querySet(r, "productPlanIds")
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	region := strings.TrimSpace(r.URL.Query().Get("region"))
	channel := strings.TrimSpace(r.URL.Query().Get("channel"))
	openingMethod := strings.TrimSpace(r.URL.Query().Get("openingMethod"))
	source := strings.TrimSpace(r.URL.Query().Get("source"))
	query := r.URL.Query().Get("q")
	filtered := make([]officialprice.Record, 0, len(items))
	for _, item := range items {
		if !setContains(planIDs, item.ProductPlanID) || (status != "" && item.Status != status) {
			continue
		}
		if region != "" && item.RegionCode != region {
			continue
		}
		if channel != "" && !strings.Contains(strings.ToLower(item.Channel), strings.ToLower(channel)) {
			continue
		}
		if openingMethod != "" && !strings.Contains(strings.ToLower(item.OpeningMethod), strings.ToLower(openingMethod)) {
			continue
		}
		if source != "" && !strings.Contains(strings.ToLower(item.SourceURL), strings.ToLower(source)) {
			continue
		}
		if !containsFold(query, item.ID, item.ProductPlanID, item.RegionCode, item.Channel, item.OpeningMethod, item.SourceURL, item.Currency) {
			continue
		}
		filtered = append(filtered, item)
	}
	switch r.URL.Query().Get("sort") {
	case "cny_asc":
		sort.SliceStable(filtered, func(i, j int) bool {
			return parseDecimalFloat(filtered[i].NormalizedMonthlyCNY) < parseDecimalFloat(filtered[j].NormalizedMonthlyCNY)
		})
	case "updated_desc", "verified_recent", "submitted_desc":
		sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].ValidFrom.After(filtered[j].ValidFrom) })
	}
	return filtered
}

func filterCarpoolApplications(r *http.Request, items []carpool.Application, memberships []carpool.Membership) []carpool.Application {
	statuses := querySet(r, "statuses")
	carpoolID := strings.TrimSpace(r.URL.Query().Get("carpoolId"))
	query := r.URL.Query().Get("q")
	membershipByApplicationID := make(map[string]carpool.Membership, len(memberships))
	for _, membership := range memberships {
		membershipByApplicationID[membership.CarpoolApplicationID] = membership
	}
	filtered := make([]carpool.Application, 0, len(items))
	for _, item := range items {
		status := carpoolApplicationDisplayStatus(item, membershipByApplicationID[item.ID])
		if !setContains(statuses, status) || (carpoolID != "" && item.CarpoolListingID != carpoolID) {
			continue
		}
		if !containsFold(query, item.ID, item.CarpoolListingID, item.ListingTitleSnapshot, item.BuyerUserID, item.OwnerUserID) {
			continue
		}
		filtered = append(filtered, item)
	}
	sortMode := r.URL.Query().Get("sort")
	if sortMode == "created_desc" {
		sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].CreatedAt.After(filtered[j].CreatedAt) })
	} else if sortMode == "default_buyer" || sortMode == "default_owner" {
		sort.SliceStable(filtered, func(i, j int) bool {
			iStatus := carpoolApplicationDisplayStatus(filtered[i], membershipByApplicationID[filtered[i].ID])
			jStatus := carpoolApplicationDisplayStatus(filtered[j], membershipByApplicationID[filtered[j].ID])
			iAction := carpoolApplicationActionRequired(iStatus, sortMode)
			jAction := carpoolApplicationActionRequired(jStatus, sortMode)
			if iAction != jAction {
				return iAction
			}
			return filtered[i].UpdatedAt.After(filtered[j].UpdatedAt)
		})
	} else {
		sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].UpdatedAt.After(filtered[j].UpdatedAt) })
	}
	return filtered
}

func carpoolApplicationActionRequired(status, sortMode string) bool {
	if sortMode == "default_owner" {
		return status == "pending_owner" || status == "joined_pending_confirmation" || status == "pending_completion" || status == "disputed"
	}
	return status == "accepted_reserved" || status == "waiting_contact" || status == "contacted" || status == "pending_completion" || status == "disputed"
}

func carpoolApplicationDisplayStatus(application carpool.Application, membership carpool.Membership) string {
	if membership.ID != "" {
		switch membership.Status {
		case carpool.MembershipStatusCompleted:
			return "completed"
		case carpool.MembershipStatusLeft:
			return "cancelled_by_buyer"
		case carpool.MembershipStatusRemoved:
			return "cancelled_by_owner"
		case carpool.MembershipStatusActive:
			if membership.BuyerCompletedAt != nil || membership.OwnerCompletedAt != nil {
				return "pending_completion"
			}
			return "active"
		}
	}
	if application.Status == carpool.ApplicationStatusAcceptedReserved {
		if application.BuyerConfirmedAt != nil || application.OwnerConfirmedAt != nil {
			return "joined_pending_confirmation"
		}
		return "accepted_reserved"
	}
	return application.Status
}

func filterAPIOrders(r *http.Request, items []apiorder.Order) []apiorder.Order {
	statuses := querySet(r, "statuses")
	serviceID := strings.TrimSpace(r.URL.Query().Get("serviceId"))
	query := r.URL.Query().Get("q")
	dateRange := r.URL.Query().Get("dateRange")
	risk := r.URL.Query().Get("risk")
	dispute := r.URL.Query().Get("dispute")
	now := time.Now()
	filtered := make([]apiorder.Order, 0, len(items))
	for _, item := range items {
		if !setContains(statuses, item.Status) || (serviceID != "" && item.APIServiceID != serviceID) || !matchesDateRange(item.CreatedAt, dateRange, now) {
			continue
		}
		hasRiskNote := apiorder.IsDisputeActive(item.DisputeStatus) || strings.TrimSpace(item.CancelReason) != ""
		if risk == "high" && !apiorder.IsDisputeActive(item.DisputeStatus) || risk == "has_note" && !hasRiskNote {
			continue
		}
		if dispute == "active" && !apiorder.IsDisputeActive(item.DisputeStatus) || dispute == "none" && apiorder.IsDisputeActive(item.DisputeStatus) {
			continue
		}
		if !containsFold(query, item.ID, item.OrderNo, item.APIServiceID, item.ServiceTitleSnapshot, item.BuyerUserID, item.SellerUserID) {
			continue
		}
		filtered = append(filtered, item)
	}
	switch r.URL.Query().Get("sort") {
	case "amount_desc":
		sort.SliceStable(filtered, func(i, j int) bool {
			return parseDecimalFloat(filtered[i].Amount) > parseDecimalFloat(filtered[j].Amount)
		})
	case "amount_asc":
		sort.SliceStable(filtered, func(i, j int) bool {
			return parseDecimalFloat(filtered[i].Amount) < parseDecimalFloat(filtered[j].Amount)
		})
	case "created_desc":
		sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].CreatedAt.After(filtered[j].CreatedAt) })
	case "default_buyer", "default_merchant":
		sortMode := r.URL.Query().Get("sort")
		sort.SliceStable(filtered, func(i, j int) bool {
			iAction := apiOrderActionRequired(filtered[i].Status, sortMode)
			jAction := apiOrderActionRequired(filtered[j].Status, sortMode)
			if iAction != jAction {
				return iAction
			}
			return filtered[i].UpdatedAt.After(filtered[j].UpdatedAt)
		})
	default:
		sort.SliceStable(filtered, func(i, j int) bool { return filtered[i].UpdatedAt.After(filtered[j].UpdatedAt) })
	}
	return filtered
}

func validateAPIOrderListQuery(r *http.Request) *domain.AppError {
	dispute := strings.TrimSpace(r.URL.Query().Get("dispute"))
	if apiorder.IsAdminOrderDispute(dispute) {
		return nil
	}
	detail := "纠纷筛选无效。"
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "List query invalid", detail, "dispute", "invalid", detail)
}

func apiOrderActionRequired(status, sortMode string) bool {
	if sortMode == "default_merchant" {
		return status == apiorder.StatusPaymentSubmitted || status == apiorder.StatusPaidConfirmed
	}
	return status == apiorder.StatusPendingPayment || status == apiorder.StatusPaymentIssue || status == apiorder.StatusDeliverySubmitted
}

func filterReviewCenterRows(r *http.Request, items []review.ReviewCenterRow) []review.ReviewCenterRow {
	direction := strings.TrimSpace(r.URL.Query().Get("direction"))
	if direction == "" {
		return items
	}
	filtered := make([]review.ReviewCenterRow, 0, len(items))
	for _, item := range items {
		if item.Direction == direction {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func filterAdminAnnouncements(r *http.Request, items []announcement.Announcement) []announcement.Announcement {
	statusGroup := r.URL.Query().Get("statusGroup")
	query := r.URL.Query().Get("q")
	now := time.Now()
	filtered := make([]announcement.Announcement, 0, len(items))
	for _, item := range items {
		status := announcement.DisplayStatus(item, now)
		matchesStatus := statusGroup == "" || statusGroup == "all" || statusGroup == status ||
			statusGroup == "working" && (status == announcement.StatusDraft || status == announcement.StatusScheduled || status == announcement.StatusPublished) ||
			statusGroup == "history" && (status == announcement.StatusOffline || status == announcement.StatusExpired)
		if !matchesStatus || !containsFold(query, item.ID, item.Title, item.Summary, item.Category, item.Level, status, strings.Join(item.Channels, " ")) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}
