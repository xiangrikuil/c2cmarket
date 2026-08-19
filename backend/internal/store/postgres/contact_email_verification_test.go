package postgres

import (
	"os"
	"strings"
	"testing"
)

func TestContactEmailVerificationUsesParentBeforeChallengeLockOrder(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("contact.go")
	if err != nil {
		t.Fatalf("read contact store: %v", err)
	}
	text := string(source)
	start := strings.Index(text, "func (s *Store) ConfirmContactEmailVerificationWithIdempotency")
	if start < 0 {
		t.Fatal("contact email confirmation implementation not found")
	}
	end := strings.Index(text[start:], "func consumeContactEmailChallengeInTx")
	if end < 0 {
		t.Fatal("contact email confirmation implementation end not found")
	}
	implementation := text[start : start+end]
	methodLock := strings.Index(implementation, "FROM contact_methods")
	challengeLock := strings.Index(implementation, "FROM email_verification_codes")
	if methodLock < 0 || challengeLock < 0 || methodLock >= challengeLock {
		t.Fatal("contact email confirmation must lock the contact method before its challenge")
	}
}
