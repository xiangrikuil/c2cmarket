package apimarket

import (
	"sort"
	"strings"
)

const (
	AdminServiceViewPublic     = "public"
	AdminServiceViewExceptions = "exceptions"
	AdminServiceRiskHigh       = "high"
)

type AdminServiceFilter struct {
	Query    string
	View     string
	Statuses []string
	Risk     string
}

func filterAdminServices(items []Service, filter AdminServiceFilter) []Service {
	statuses := adminServiceStringSet(filter.Statuses)
	query := strings.ToLower(strings.TrimSpace(filter.Query))
	view := strings.TrimSpace(filter.View)
	risk := strings.TrimSpace(filter.Risk)
	filtered := make([]Service, 0, len(items))
	for _, item := range items {
		status := adminServiceStatus(item)
		isPublic := status == "online"
		isException := status == "pending" || status == "changes_requested" || status == "suspended" || status == "rejected" || status == "removed"
		if view == AdminServiceViewPublic && !isPublic || view == AdminServiceViewExceptions && !isException {
			continue
		}
		if len(statuses) > 0 {
			if _, exists := statuses[status]; !exists {
				continue
			}
		}
		if risk == AdminServiceRiskHigh && item.ModerationStatus == ServiceModerationStatusClear && item.UnresolvedDisputes == 0 {
			continue
		}
		if !containsAdminServiceText(query, item.ID, item.Title, item.ShortDescription, item.MerchantDisplayName, item.OwnerUserID, item.ModerationReason) {
			continue
		}
		filtered = append(filtered, item)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if !filtered[i].UpdatedAt.Equal(filtered[j].UpdatedAt) {
			return filtered[i].UpdatedAt.After(filtered[j].UpdatedAt)
		}
		return filtered[i].ID > filtered[j].ID
	})
	return filtered
}

func adminServiceStatus(item Service) string {
	if item.ModerationStatus == ServiceModerationStatusRemoved {
		return "removed"
	}
	if item.ModerationStatus == ServiceModerationStatusAdminSuspended {
		return "suspended"
	}
	if item.ReviewStatus == ServiceReviewStatusPendingReview {
		return "pending"
	}
	if item.ReviewStatus == ServiceReviewStatusChangesRequested {
		return "changes_requested"
	}
	if item.ReviewStatus == ServiceReviewStatusRejected {
		return "rejected"
	}
	if item.ReviewStatus == ServiceReviewStatusApproved && item.PublicationStatus == ServicePublicationStatusOnline {
		return "online"
	}
	if item.ReviewStatus == ServiceReviewStatusApproved && item.PublicationStatus == ServicePublicationStatusOwnerPaused {
		return "paused"
	}
	if item.ReviewStatus == ServiceReviewStatusApproved {
		return "approved"
	}
	return "draft"
}

func adminServiceStringSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			set[value] = struct{}{}
		}
	}
	return set
}

func containsAdminServiceText(query string, values ...string) bool {
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
