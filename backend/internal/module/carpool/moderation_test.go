package carpool

import "testing"

func TestCanRequestChangesForPendingAndActiveListings(t *testing.T) {
	for _, status := range []string{ListingStatusPendingReview, ListingStatusActive} {
		if !canUpdateListingStatus(status, ListingStatusChangesRequested, "request_changes") {
			t.Fatalf("request_changes should be allowed from %q", status)
		}
	}

	for _, status := range []string{ListingStatusDraft, ListingStatusChangesRequested, ListingStatusPaused, ListingStatusRejected, ListingStatusRemoved} {
		if canUpdateListingStatus(status, ListingStatusChangesRequested, "request_changes") {
			t.Fatalf("request_changes should not be allowed from %q", status)
		}
	}
}
