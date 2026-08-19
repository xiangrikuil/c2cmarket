package postgres

import (
	"strings"
	"testing"
)

func TestNotificationSelectProjectsOnlyOutstandingDisputeActions(t *testing.T) {
	requiredFragments := []string{
		"dispute.remedy_claimed",
		"dispute.remedy_confirmation_due",
		"dispute.active = true",
		"remedy.status = 'claimed_fulfilled'",
		"remedy.beneficiary_user_id = notification.user_id",
		"remedy.confirmation_due_at > now()",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(notificationSelectSQL, fragment) {
			t.Fatalf("notification action projection missing %q", fragment)
		}
	}
}
