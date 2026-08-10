package postgres

import (
	"strings"
	"testing"
)

func TestNavigationBadgeCountsActionableAPIServiceExceptions(t *testing.T) {
	normalizedSQL := strings.Join(strings.Fields(navigationBadgeSummarySQL), " ")
	want := "FROM api_services WHERE review_status = 'pending_review' OR moderation_status = 'admin_suspended'"
	if !strings.Contains(normalizedSQL, want) {
		t.Fatalf("navigation badge query does not count both actionable API service exceptions: %s", normalizedSQL)
	}
}
