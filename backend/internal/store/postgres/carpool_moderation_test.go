package postgres

import (
	"testing"

	"c2c-market/backend/internal/module/carpool"
)

func TestCanRequestChangesForPendingAndActiveCarpoolListings(t *testing.T) {
	for _, status := range []string{carpool.ListingStatusPendingReview, carpool.ListingStatusActive} {
		if !canUpdateCarpoolListingStatus(status, carpool.ListingStatusChangesRequested, "request_changes") {
			t.Fatalf("request_changes should be allowed from %q", status)
		}
	}

	for _, status := range []string{carpool.ListingStatusDraft, carpool.ListingStatusChangesRequested, carpool.ListingStatusPaused, carpool.ListingStatusRejected, carpool.ListingStatusRemoved} {
		if canUpdateCarpoolListingStatus(status, carpool.ListingStatusChangesRequested, "request_changes") {
			t.Fatalf("request_changes should not be allowed from %q", status)
		}
	}
}
