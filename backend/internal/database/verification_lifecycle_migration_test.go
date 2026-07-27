package database

import (
	"strings"
	"testing"
)

func TestVerificationDataLifecycleMigrationContract(t *testing.T) {
	t.Parallel()

	upSQL := readMigrationForTest(t, "000063_verification_data_lifecycle.up.sql")
	for _, required := range []string{
		"UPDATE email_verification_codes",
		"ux_email_verification_codes_active_bind_user",
		"status IN ('processing', 'completed', 'failed')",
		"ck_idempotency_failed_response",
		"interval '15 minutes'",
		"interval '7 days'",
		"octet_length(response_body_json::text) > 65536",
		"ix_auth_sessions_expires_at",
		"ix_contact_sessions_open_ends_at",
		"ix_notifications_created_at",
		"ix_domain_events_created_at",
	} {
		if !strings.Contains(upSQL, required) {
			t.Fatalf("verification lifecycle migration missing %q", required)
		}
	}

	downSQL := readMigrationForTest(t, "000063_verification_data_lifecycle.down.sql")
	for _, required := range []string{
		"DROP INDEX IF EXISTS ux_email_verification_codes_active_bind_user",
		"DELETE FROM idempotency_keys",
		"status IN ('processing', 'completed')",
	} {
		if !strings.Contains(downSQL, required) {
			t.Fatalf("verification lifecycle rollback missing %q", required)
		}
	}
}
