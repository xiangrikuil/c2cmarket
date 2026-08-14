package carpool

import (
	"math/big"
	"sort"
	"strings"
)

const (
	ListingViewPublic     = "public"
	ListingViewExceptions = "exceptions"

	ListingRiskHigh = "high"

	ListingSortRecommended = "recommended"
	ListingSortUpdatedDesc = "updated_desc"
	ListingSortPriceAsc    = "price_asc"
	ListingSortSeatsDesc   = "seats_desc"
)

type ListingFilter struct {
	Query          string
	ProductPlanIDs []string
	Region         string
	Statuses       []string
	View           string
	Risk           string
	Sort           string
	None           bool
}

func (filter ListingFilter) NormalizedSort() string {
	switch strings.TrimSpace(filter.Sort) {
	case ListingSortPriceAsc:
		return ListingSortPriceAsc
	case ListingSortSeatsDesc:
		return ListingSortSeatsDesc
	default:
		return ListingSortUpdatedDesc
	}
}

func filterListings(items []Listing, filter ListingFilter) []Listing {
	if filter.None {
		return []Listing{}
	}
	planIDs := stringSet(filter.ProductPlanIDs)
	statuses := stringSet(filter.Statuses)
	query := strings.ToLower(strings.TrimSpace(filter.Query))
	region := strings.TrimSpace(filter.Region)
	view := strings.TrimSpace(filter.View)
	risk := strings.TrimSpace(filter.Risk)
	filtered := make([]Listing, 0, len(items))
	for _, item := range items {
		isPublic := item.Status == ListingStatusActive
		if view == ListingViewPublic && !isPublic || view == ListingViewExceptions && isPublic {
			continue
		}
		if !matchesStringSet(planIDs, item.ProductPlanID) || !matchesStringSet(statuses, item.Status) {
			continue
		}
		if region != "" && item.RegionName != region && item.RegionCode != region {
			continue
		}
		if risk == ListingRiskHigh && !item.RiskAckRequired && !containsListingText("风险", item.ReviewReason) {
			continue
		}
		if !containsListingText(query, item.ID, item.Title, item.Summary, item.RegionName, item.OwnerUserID, item.ReviewReason) {
			continue
		}
		filtered = append(filtered, item)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		left := filtered[i]
		right := filtered[j]
		switch filter.NormalizedSort() {
		case ListingSortPriceAsc:
			comparison := decimalValue(left.PriceMonthlyCNY).Cmp(decimalValue(right.PriceMonthlyCNY))
			if comparison != 0 {
				return comparison < 0
			}
			return left.ID < right.ID
		case ListingSortSeatsDesc:
			if left.AvailableSeats != right.AvailableSeats {
				return left.AvailableSeats > right.AvailableSeats
			}
			return left.ID > right.ID
		default:
			if !left.UpdatedAt.Equal(right.UpdatedAt) {
				return left.UpdatedAt.After(right.UpdatedAt)
			}
			return left.ID > right.ID
		}
	})
	return filtered
}

func stringSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			set[value] = struct{}{}
		}
	}
	return set
}

func matchesStringSet(values map[string]struct{}, value string) bool {
	if len(values) == 0 {
		return true
	}
	_, exists := values[value]
	return exists
}

func containsListingText(query string, values ...string) bool {
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

func decimalValue(value string) *big.Rat {
	parsed, ok := new(big.Rat).SetString(strings.TrimSpace(value))
	if !ok {
		return new(big.Rat)
	}
	return parsed
}
