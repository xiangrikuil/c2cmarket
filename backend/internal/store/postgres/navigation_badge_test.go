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

func TestNavigationBadgeCountsOnlyUserSupportActions(t *testing.T) {
	normalizedSQL := strings.Join(strings.Fields(navigationBadgeSummarySQL), " ")
	for _, want := range []string{
		"FROM feedback_tickets WHERE submitter_user_id = $1 AND latest_admin_update_at IS NOT NULL",
		"FROM feedback_tickets WHERE submitter_user_id = $1 AND ( status = 'needs_user_info' OR ( latest_admin_update_at IS NOT NULL",
		"FROM moderation_info_requests WHERE requested_from_user_id = $1 AND status = 'open'",
		"(support_counts.feedback_actions + support_counts.moderation_actions)::int AS support_action_count",
	} {
		if !strings.Contains(normalizedSQL, want) {
			t.Fatalf("navigation badge query is missing support action fact %q: %s", want, normalizedSQL)
		}
	}
}

func TestNavigationBadgeCountsOnlyTargetedStrongAnnouncements(t *testing.T) {
	normalizedSQL := strings.Join(strings.Fields(navigationBadgeSummarySQL), " ")
	for _, want := range []string{
		"a.level IN ('important', 'critical')",
		"a.expire_at IS NULL OR a.expire_at > $2",
		"a.audience_json->>'type' = 'all' OR EXISTS",
		"recipient.user_id = $1 AND recipient.announcement_version = a.version",
	} {
		if !strings.Contains(normalizedSQL, want) {
			t.Fatalf("navigation badge query is missing strong-announcement boundary %q: %s", want, normalizedSQL)
		}
	}
}

func TestNavigationBadgeUsesLightweightCarpoolActions(t *testing.T) {
	normalizedSQL := strings.Join(strings.Fields(navigationBadgeSummarySQL), " ")
	for _, obsolete := range []string{"carpool_join_confirmations", "carpool_completion_confirmations", "accepted_reserved", "reservation_expires_at", "join_confirmation_deadline"} {
		if strings.Contains(normalizedSQL, obsolete) {
			t.Fatalf("navigation badge query still uses obsolete carpool workflow %q: %s", obsolete, normalizedSQL)
		}
	}
	if !strings.Contains(normalizedSQL, "application.status = 'pending_owner') AS merchant_carpool_actions") {
		t.Fatalf("navigation badge query must count pending owner applications: %s", normalizedSQL)
	}
}

func TestNavigationBadgeCountsActiveAPIOrderDisputesAsDistinctOrderActions(t *testing.T) {
	normalizedSQL := strings.Join(strings.Fields(navigationBadgeSummarySQL), " ")
	activeStatuses := "dispute_status IN ( 'negotiating', 'pending_seller_response', 'pending_applicant_decision', 'open', 'awaiting_fulfillment', 'fulfillment_confirmation' )"
	for _, want := range []string{
		"status = 'delivery_submitted' OR " + activeStatuses,
		activeStatuses + ") AS buyer_api_order_disputes",
		"status IN ('payment_submitted', 'paid_confirmed') OR " + activeStatuses,
		activeStatuses + ") AS merchant_api_order_disputes",
	} {
		if !strings.Contains(normalizedSQL, want) {
			t.Fatalf("navigation badge query is missing dispute action boundary %q: %s", want, normalizedSQL)
		}
	}
	for _, terminal := range []string{"dispute_status = 'none'", "dispute_status = 'closed'"} {
		if strings.Contains(normalizedSQL, terminal) {
			t.Fatalf("navigation badge query must not count terminal dispute projection %q: %s", terminal, normalizedSQL)
		}
	}
}
