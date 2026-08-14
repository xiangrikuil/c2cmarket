package auth

import (
	"testing"

	"c2c-market/backend/internal/domain"
)

func TestValidatePublicUsernameIsStrictAndDoesNotRepair(t *testing.T) {
	valid := []string{"abc", "campus_user", "buyer-2026", "a_b-c"}
	for _, username := range valid {
		if err := ValidatePublicUsername(username); err != nil {
			t.Fatalf("valid username %q rejected: %v", username, err)
		}
	}
	invalid := []string{"ABCD", " user", "user ", "user name", "ab", "user@example", "用户", "a.b"}
	for _, username := range invalid {
		err := ValidatePublicUsername(username)
		if err == nil || err.Code != domain.CodeUsernameInvalid {
			t.Fatalf("invalid username %q returned %+v", username, err)
		}
	}
}

func TestValidatePublicUsernameRejectsEveryReservedWord(t *testing.T) {
	for _, username := range ReservedUsernames() {
		err := ValidatePublicUsername(username)
		if err == nil || err.Code != domain.CodeUsernameUnavailable {
			t.Fatalf("reserved username %q returned %+v", username, err)
		}
	}
}

func TestOAuthUsernameCandidateNeverAssignsReservedHandle(t *testing.T) {
	got := OAuthUsernameCandidate("admin", "linux_do", "42", 0)
	if got == "admin" || ValidatePublicUsername(got) != nil {
		t.Fatalf("OAuthUsernameCandidate(admin) = %q", got)
	}
}
