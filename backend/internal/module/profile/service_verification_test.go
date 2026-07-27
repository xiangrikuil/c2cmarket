package profile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"testing"
	"time"

	"c2c-market/backend/internal/module/auth"
)

func TestEmailVerificationCountsFailuresAndLocksAtLimit(t *testing.T) {
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	service := newVerificationTestService(func() time.Time { return now })
	user := verificationTestUser()

	challenge, appErr := service.StartEmailVerification(context.Background(), user, EmailVerificationStartInput{Email: "user@example.com"})
	if appErr != nil {
		t.Fatalf("start verification: %v", appErr)
	}
	for attempt := 1; attempt <= EmailVerificationMaxAttempts; attempt++ {
		if _, appErr := service.ConfirmEmailVerification(context.Background(), user, EmailVerificationConfirmInput{
			Email: "user@example.com",
			Code:  "000000",
		}); appErr == nil {
			t.Fatalf("wrong code attempt %d unexpectedly succeeded", attempt)
		}
	}
	if _, appErr := service.ConfirmEmailVerification(context.Background(), user, EmailVerificationConfirmInput{
		Email: "user@example.com",
		Code:  challenge.DevCode,
	}); appErr == nil {
		t.Fatal("locked challenge accepted the correct code")
	}
}

func TestEmailVerificationNewChallengeReplacesPreviousEmail(t *testing.T) {
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	service := newVerificationTestService(func() time.Time { return now })
	user := verificationTestUser()

	first, appErr := service.StartEmailVerification(context.Background(), user, EmailVerificationStartInput{Email: "first@example.com"})
	if appErr != nil {
		t.Fatalf("start first verification: %v", appErr)
	}
	second, appErr := service.StartEmailVerification(context.Background(), user, EmailVerificationStartInput{Email: "second@example.com"})
	if appErr != nil {
		t.Fatalf("start second verification: %v", appErr)
	}
	if _, appErr := service.ConfirmEmailVerification(context.Background(), user, EmailVerificationConfirmInput{
		Email: "first@example.com",
		Code:  first.DevCode,
	}); appErr == nil {
		t.Fatal("replaced challenge unexpectedly succeeded")
	}
	updated, appErr := service.ConfirmEmailVerification(context.Background(), user, EmailVerificationConfirmInput{
		Email: "second@example.com",
		Code:  second.DevCode,
	})
	if appErr != nil {
		t.Fatalf("confirm replacement challenge: %v", appErr)
	}
	if updated.Email != "second@example.com" || updated.EmailVerifiedAt == nil {
		t.Fatalf("unexpected verified profile: %+v", updated)
	}
}

func TestEmailVerificationConcurrentConfirmationSucceedsOnce(t *testing.T) {
	now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
	service := newVerificationTestService(func() time.Time { return now })
	user := verificationTestUser()
	challenge, appErr := service.StartEmailVerification(context.Background(), user, EmailVerificationStartInput{Email: "user@example.com"})
	if appErr != nil {
		t.Fatalf("start verification: %v", appErr)
	}

	var wait sync.WaitGroup
	wait.Add(2)
	successes := 0
	var mu sync.Mutex
	for range 2 {
		go func() {
			defer wait.Done()
			_, appErr := service.ConfirmEmailVerification(context.Background(), user, EmailVerificationConfirmInput{
				Email: "user@example.com",
				Code:  challenge.DevCode,
			})
			if appErr == nil {
				mu.Lock()
				successes++
				mu.Unlock()
			}
		}()
	}
	wait.Wait()
	if successes != 1 {
		t.Fatalf("expected exactly one successful confirmation, got %d", successes)
	}
}

func TestEmailVerificationDigestUsesPepper(t *testing.T) {
	input := "user-1:user@example.com:123456"
	bare := sha256.Sum256([]byte(input))
	digest := emailCodeHash([]byte("test-email-verification-pepper-value"), "user-1", "user@example.com", "123456")
	if digest == hex.EncodeToString(bare[:]) {
		t.Fatal("verification digest must not equal bare SHA-256")
	}
	if digest == emailCodeHash([]byte("different-email-verification-pepper"), "user-1", "user@example.com", "123456") {
		t.Fatal("verification digest must depend on the configured pepper")
	}
}

func newVerificationTestService(now func() time.Time) *Service {
	return NewServiceWithOptions(nil, now, NewDevelopmentEmailSender(), ServiceOptions{
		EmailVerificationPepper: "test-email-verification-pepper-value",
	})
}

func verificationTestUser() auth.User {
	return auth.User{
		ID:          "10000000-0000-0000-0000-000000000001",
		Username:    "verification-user",
		DisplayName: "Verification User",
		Status:      "active",
	}
}
