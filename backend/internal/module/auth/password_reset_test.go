package auth

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
)

type passwordResetTestSender struct {
	mu       sync.Mutex
	messages []passwordResetTestMessage
	fail     bool
}

type passwordResetTestMessage struct {
	email     string
	code      string
	expiresAt time.Time
}

func (*passwordResetTestSender) SendVerificationCode(context.Context, string, string, time.Time) *domain.AppError {
	return nil
}

func (*passwordResetTestSender) SendRegistrationSuccess(context.Context, string, string, string, time.Time) *domain.AppError {
	return nil
}

func (*passwordResetTestSender) ExposeDevCode() bool { return false }

func (s *passwordResetTestSender) SendPasswordResetCode(_ context.Context, email, code string, expiresAt time.Time) *domain.AppError {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail {
		return domain.NewError(503, domain.CodeExternalSourceUnavailable, "Email unavailable", "邮件服务暂时不可用。")
	}
	s.messages = append(s.messages, passwordResetTestMessage{email: email, code: code, expiresAt: expiresAt})
	return nil
}

func (s *passwordResetTestSender) lastMessage(t *testing.T) passwordResetTestMessage {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.messages) == 0 {
		t.Fatal("password reset email was not sent")
	}
	return s.messages[len(s.messages)-1]
}

type passwordResetTestRecorder struct {
	mu       sync.Mutex
	outcomes []string
}

func (r *passwordResetTestRecorder) RecordPasswordResetDelivery(outcome string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.outcomes = append(r.outcomes, outcome)
}

func TestPasswordResetMemoryLifecycle(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 14, 9, 0, 0, 0, time.UTC)
	sender := &passwordResetTestSender{}
	recorder := &passwordResetTestRecorder{}
	service := NewServiceWithRegistrationEmailSender(nil, func() time.Time { return now }, sender)
	service.SetPasswordResetDeliveryRecorder(recorder)

	user := User{ID: "student-1", Username: "student-one", Status: AccountStatusSuspended, LinuxDoBinding: &LinuxDoBinding{Bound: true, LinuxDoUserID: "linuxdo-1"}}
	claim := StudentEmailClaim{ID: "claim-1", UserID: user.ID, NormalizedEmail: "student@example.edu"}
	service.users[user.ID] = user
	service.studentClaimsByEmail[claim.NormalizedEmail] = claim
	service.studentClaimsByUserID[user.ID] = claim
	service.passwordCredentialsByUserID[user.ID] = newPasswordCredential(user, "Old-password-1!")
	service.sessions["normal-1"] = Session{ID: "normal-1", UserID: user.ID}
	service.restrictedBusinessSessions["restricted-1"] = RestrictedBusinessSession{ID: "restricted-1", UserID: user.ID}
	service.accountAppealSessions["appeal-1"] = AccountAppealSession{ID: "appeal-1", UserID: user.ID}

	started, appErr := service.StartPasswordReset(ctx, PasswordResetStartInput{Email: " Student@EXAMPLE.EDU ", RequestID: "req-reset-start"})
	if appErr != nil || !started.Accepted {
		t.Fatalf("StartPasswordReset result=%+v error=%v", started, appErr)
	}
	message := sender.lastMessage(t)
	if message.email != claim.NormalizedEmail || message.expiresAt.Sub(now) != PasswordResetCodeLifetime {
		t.Fatalf("unexpected reset message: %+v", message)
	}
	if len(message.code) != 6 {
		t.Fatalf("reset code length = %d", len(message.code))
	}

	if appErr := service.ConfirmPasswordReset(ctx, PasswordResetConfirmInput{Email: claim.NormalizedEmail, Code: message.code, NewPassword: "New-password-2!", RequestID: "req-reset-confirm"}); appErr != nil {
		t.Fatalf("ConfirmPasswordReset error=%v", appErr)
	}
	credential := service.passwordCredentialsByUserID[user.ID]
	if matches, _ := passwordCredentialMatches(credential, "New-password-2!"); !matches || credential.Algorithm != PasswordAlgorithmArgon2IDV1 {
		t.Fatalf("new password credential was not stored: %+v", credential)
	}
	if matches, _ := passwordCredentialMatches(credential, "Old-password-1!"); matches {
		t.Fatal("old password still matches after reset")
	}
	if service.sessions["normal-1"].RevokedAt == nil || service.restrictedBusinessSessions["restricted-1"].RevokedAt == nil {
		t.Fatal("normal and restricted-business sessions were not revoked")
	}
	if service.accountAppealSessions["appeal-1"].RevokedAt != nil {
		t.Fatal("account-appeal session was revoked")
	}
	if service.users[user.ID].Status != AccountStatusSuspended {
		t.Fatal("password reset changed account governance status")
	}
	if appErr := service.ConfirmPasswordReset(ctx, PasswordResetConfirmInput{Email: claim.NormalizedEmail, Code: message.code, NewPassword: "Another-password-3!"}); appErr == nil || appErr.Code != domain.CodeVerificationCodeInvalid {
		t.Fatalf("consumed reset code was reusable: %v", appErr)
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if len(recorder.outcomes) != 1 || recorder.outcomes[0] != "sent" {
		t.Fatalf("delivery outcomes=%v", recorder.outcomes)
	}
}

func TestPasswordResetStartDoesNotEnumerateIneligibleAccounts(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
	sender := &passwordResetTestSender{}
	service := NewServiceWithRegistrationEmailSender(nil, func() time.Time { return now }, sender)
	lockedAt := now

	for _, fixture := range []struct {
		user  User
		email string
	}{
		{user: User{ID: "archived", Status: AccountStatusArchived}, email: "archived@example.edu"},
		{user: User{ID: "locked", Status: AccountStatusActive, SecurityLockedAt: &lockedAt}, email: "locked@example.edu"},
	} {
		service.users[fixture.user.ID] = fixture.user
		service.studentClaimsByEmail[fixture.email] = StudentEmailClaim{ID: "claim-" + fixture.user.ID, UserID: fixture.user.ID, NormalizedEmail: fixture.email}
	}

	for _, email := range []string{"missing@example.edu", "archived@example.edu", "locked@example.edu"} {
		result, appErr := service.StartPasswordReset(ctx, PasswordResetStartInput{Email: email})
		if appErr != nil || !result.Accepted {
			t.Fatalf("StartPasswordReset(%q) result=%+v error=%v", email, result, appErr)
		}
	}
	sender.mu.Lock()
	defer sender.mu.Unlock()
	if len(sender.messages) != 0 {
		t.Fatalf("ineligible accounts received reset messages: %+v", sender.messages)
	}
}

func TestPasswordResetConsumesChallengeAfterFiveInvalidCodes(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 14, 11, 0, 0, 0, time.UTC)
	sender := &passwordResetTestSender{}
	service := NewServiceWithRegistrationEmailSender(nil, func() time.Time { return now }, sender)
	user := User{ID: "student-attempts", Username: "student-attempts", Status: AccountStatusActive}
	email := "attempts@example.edu"
	service.users[user.ID] = user
	service.studentClaimsByEmail[email] = StudentEmailClaim{ID: "claim-attempts", UserID: user.ID, NormalizedEmail: email}

	if _, appErr := service.StartPasswordReset(ctx, PasswordResetStartInput{Email: email}); appErr != nil {
		t.Fatalf("StartPasswordReset error=%v", appErr)
	}
	message := sender.lastMessage(t)
	wrongCode := "000000"
	if message.code == wrongCode {
		wrongCode = "000001"
	}
	for attempt := 1; attempt <= PasswordResetMaxAttempts; attempt++ {
		appErr := service.ConfirmPasswordReset(ctx, PasswordResetConfirmInput{Email: email, Code: wrongCode, NewPassword: "New-password-2!"})
		if appErr == nil || appErr.Code != domain.CodeVerificationCodeInvalid {
			t.Fatalf("attempt %d error=%v", attempt, appErr)
		}
	}
	if appErr := service.ConfirmPasswordReset(ctx, PasswordResetConfirmInput{Email: email, Code: message.code, NewPassword: "New-password-2!"}); appErr == nil || appErr.Code != domain.CodeVerificationCodeInvalid {
		t.Fatalf("correct code succeeded after attempts were exhausted: %v", appErr)
	}
	for _, challenge := range service.passwordResetCodes {
		if !challenge.Consumed || challenge.Attempts != PasswordResetMaxAttempts {
			t.Fatalf("challenge was not exhausted: %+v", challenge)
		}
	}
}

func TestPasswordResetConsumesChallengeWhenAccountBecomesIneligible(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 8, 14, 11, 30, 0, 0, time.UTC)
	sender := &passwordResetTestSender{}
	service := NewServiceWithRegistrationEmailSender(nil, func() time.Time { return now }, sender)
	user := User{ID: "student-newly-archived", Username: "student-newly-archived", Status: AccountStatusActive}
	email := "newly-archived@example.edu"
	service.users[user.ID] = user
	service.studentClaimsByEmail[email] = StudentEmailClaim{ID: "claim-newly-archived", UserID: user.ID, NormalizedEmail: email}

	if _, appErr := service.StartPasswordReset(ctx, PasswordResetStartInput{Email: email}); appErr != nil {
		t.Fatalf("StartPasswordReset error=%v", appErr)
	}
	message := sender.lastMessage(t)
	user.Status = AccountStatusArchived
	service.users[user.ID] = user
	if appErr := service.ConfirmPasswordReset(ctx, PasswordResetConfirmInput{Email: email, Code: message.code, NewPassword: "New-password-2!"}); appErr == nil || appErr.Code != domain.CodeVerificationCodeInvalid {
		t.Fatalf("newly archived account reset error=%v", appErr)
	}
	for _, challenge := range service.passwordResetCodes {
		if !challenge.Consumed {
			t.Fatalf("newly archived account retained active challenge: %+v", challenge)
		}
	}
}

func TestPasswordResetHashIsBoundToPurposeUserAndEmail(t *testing.T) {
	service := NewService(nil, time.Now)
	code := "123456"
	resetHash := service.passwordResetCodeHash("user-1", "student@example.edu", code)
	for label, candidate := range map[string]string{
		"user":    service.passwordResetCodeHash("user-2", "student@example.edu", code),
		"email":   service.passwordResetCodeHash("user-1", "other@example.edu", code),
		"purpose": VerificationCodeHash(service.emailVerificationPepper, EmailRegistrationPurpose, "user-1", "student@example.edu", code),
	} {
		if candidate == resetHash {
			t.Fatalf("%s was not bound into reset hash", label)
		}
	}
}

func TestPasswordResetEmailNormalizationRejectsNonCanonicalAddresses(t *testing.T) {
	for _, value := range []string{
		"",
		"not-an-email",
		"Student <student@example.edu>",
		strings.Repeat("a", 243) + "@example.edu",
	} {
		if _, appErr := normalizePasswordResetEmail(value); appErr == nil || appErr.Code != domain.CodeValidationFailed {
			t.Fatalf("normalizePasswordResetEmail(%q) error=%v", value, appErr)
		}
	}

	normalized, appErr := normalizePasswordResetEmail(" Student@EXAMPLE.EDU ")
	if appErr != nil || normalized != "student@example.edu" {
		t.Fatalf("normalized email=%q error=%v", normalized, appErr)
	}
}
