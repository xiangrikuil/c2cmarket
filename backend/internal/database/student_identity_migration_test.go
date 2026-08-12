package database

import (
	"os"
	"strings"
	"testing"
)

func TestStudentIdentityMigrationContract(t *testing.T) {
	up, err := os.ReadFile("../../migrations/000091_student_identity_and_auth_link.up.sql")
	if err != nil {
		t.Fatalf("read migration up: %v", err)
	}
	down, err := os.ReadFile("../../migrations/000091_student_identity_and_auth_link.down.sql")
	if err != nil {
		t.Fatalf("read migration down: %v", err)
	}

	upSQL := string(up)
	for _, fragment := range []string{
		"student_registration_settings",
		"VALUES ('global', false)",
		"student_institution_domains",
		"ck_student_institution_domain_canonical",
		"trg_student_institution_domain_identity_immutable",
		"student_email_claims",
		"ON DELETE RESTRICT",
		"trg_student_email_claim_append_only",
		"trg_users_student_claimed_profile_email",
		"ux_email_verification_codes_active_registration_email",
		"purpose IN ('email_registration', 'bind_email')",
		"attempt_count BETWEEN 0 AND 5",
		"password_reauthenticated_at",
		"oauth_link_state_purpose = 'link_linuxdo'",
	} {
		if !strings.Contains(upSQL, fragment) {
			t.Fatalf("migration up missing %q", fragment)
		}
	}
	settingsStart := strings.Index(upSQL, "CREATE TABLE student_registration_settings")
	settingsEnd := strings.Index(upSQL, "CREATE TABLE student_institution_domains")
	if settingsStart < 0 || settingsEnd <= settingsStart || strings.Contains(strings.ToLower(upSQL[settingsStart:settingsEnd]), "default true") {
		t.Fatal("registration singleton must not default to enabled")
	}

	downSQL := string(down)
	for _, fragment := range []string{
		"cannot roll back durable student identity after claims exist",
		"DROP COLUMN IF EXISTS password_reauthenticated_at",
		"DROP TABLE IF EXISTS student_email_claims",
	} {
		if !strings.Contains(downSQL, fragment) {
			t.Fatalf("migration down missing %q", fragment)
		}
	}
}
