package database

import (
	"os"
	"strings"
	"testing"
)

func TestContactEmailVerificationMigrationContract(t *testing.T) {
	t.Parallel()

	up, err := os.ReadFile("../../migrations/000110_contact_email_verification.up.sql")
	if err != nil {
		t.Fatalf("read up migration: %v", err)
	}
	down, err := os.ReadFile("../../migrations/000110_contact_email_verification.down.sql")
	if err != nil {
		t.Fatalf("read down migration: %v", err)
	}

	for _, required := range []string{
		"'contact_email'",
		"ADD COLUMN contact_method_id uuid",
		"ADD COLUMN contact_method_version_id uuid",
		"ck_email_verification_codes_purpose_shape",
		"FOREIGN KEY (contact_method_id, user_id)",
		"FOREIGN KEY (contact_method_version_id, contact_method_id, user_id)",
		"ux_email_verification_codes_active_contact_email",
		"WHERE purpose = 'contact_email' AND consumed_at IS NULL",
	} {
		if !strings.Contains(string(up), required) {
			t.Fatalf("up migration missing %q", required)
		}
	}

	for _, required := range []string{
		"DELETE FROM email_verification_codes",
		"WHERE purpose = 'contact_email'",
		"DROP INDEX IF EXISTS ux_email_verification_codes_active_contact_email",
		"DROP COLUMN IF EXISTS contact_method_version_id",
		"DROP COLUMN IF EXISTS contact_method_id",
		"CHECK (purpose IN ('bind_email', 'password_reset', 'email_registration'))",
	} {
		if !strings.Contains(string(down), required) {
			t.Fatalf("down migration missing %q", required)
		}
	}
}
