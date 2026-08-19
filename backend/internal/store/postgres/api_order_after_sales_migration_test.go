package postgres

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAPIOrderAfterSalesMigrationsPreserveSelectedContracts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		version  string
		name     string
		required []string
		forbid   []string
	}{
		{
			version: "000099", name: "api_order_active_dispute_history",
			required: []string{"ux_dispute_cases_one_active_api_order", "latest_dispute_case_id", "mutual_agreement"},
		},
		{
			version: "000100", name: "api_order_remedy_progress_lateness",
			required: []string{"late_unreviewed", "late_confirmed", "late_excused", "lateness_reversed_at IS NULL"},
		},
		{
			version: "000101", name: "api_order_appeal_governance",
			required: []string{"final_reason", "appeal_expires_at", "adversely_affected_user_ids"},
		},
		{
			version: "000102", name: "api_order_commercial_outcome_reviews",
			required: []string{"commercial_outcome", "active API-order dispute pauses review", "transaction_terminal_at + interval '14 days'"},
			forbid:   []string{"GREATEST(", "quota_replacement_expires_at"},
		},
		{
			version: "000103", name: "api_order_deadline_facts_reminders",
			required: []string{"merchant_confirm_overdue_at", "delivery_overdue_at", "delivery_due_reminded_at"},
		},
		{
			version: "000104", name: "api_order_quota_validity_issue",
			required: []string{"quota_validity_issue_at", "delivery_insufficient"},
			forbid:   []string{"replacement", "api_order_delivery_credentials"},
		},
	}

	for _, test := range tests {
		t.Run(test.version, func(t *testing.T) {
			base := filepath.Join("..", "..", "..", "migrations", test.version+"_"+test.name)
			up, err := os.ReadFile(base + ".up.sql")
			if err != nil {
				t.Fatalf("read up migration: %v", err)
			}
			if _, err := os.Stat(base + ".down.sql"); err != nil {
				t.Fatalf("read down migration: %v", err)
			}
			source := string(up)
			for _, expected := range test.required {
				if !strings.Contains(source, expected) {
					t.Fatalf("up migration missing %q", expected)
				}
			}
			for _, forbidden := range test.forbid {
				if strings.Contains(source, forbidden) {
					t.Fatalf("up migration contains rejected contract %q", forbidden)
				}
			}
		})
	}
}
