package auth

import "testing"

func TestVerificationCodeHashBindsPurposeSubjectAndEmail(t *testing.T) {
	pepper := []byte("fixed-test-pepper")
	base := VerificationCodeHash(pepper, "email_registration", "", "Student@Example.edu", "123456")
	if base != VerificationCodeHash(pepper, "email_registration", "", "student@example.edu", "123456") {
		t.Fatal("email canonicalization changed the digest")
	}
	variants := []string{
		VerificationCodeHash(pepper, "bind_email", "", "student@example.edu", "123456"),
		VerificationCodeHash(pepper, "email_registration", "user-id", "student@example.edu", "123456"),
		VerificationCodeHash(pepper, "email_registration", "", "other@example.edu", "123456"),
		VerificationCodeHash(pepper, "email_registration", "", "student@example.edu", "654321"),
	}
	for _, variant := range variants {
		if variant == base {
			t.Fatal("purpose/subject/email/code variant must produce a different digest")
		}
	}
}
