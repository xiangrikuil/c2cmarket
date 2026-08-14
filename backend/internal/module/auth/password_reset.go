package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"

	"github.com/google/uuid"
)

const (
	PasswordResetPurpose      = "password_reset"
	PasswordResetCodeLifetime = 15 * time.Minute
	PasswordResetMaxAttempts  = 5
	passwordResetDummyUserID  = "00000000-0000-0000-0000-000000000000"
)

type PasswordResetStartInput struct {
	Email     string
	RequestID string
}

type PasswordResetStartResult struct {
	Accepted bool
}

type PasswordResetConfirmInput struct {
	Email       string
	Code        string
	NewPassword string
	RequestID   string
}

type PasswordResetSubject struct {
	UserID   string
	Eligible bool
}

type PasswordResetRepository interface {
	PasswordResetSubject(ctx context.Context, normalizedEmail string) (PasswordResetSubject, *domain.AppError)
	ReplacePasswordResetChallenge(ctx context.Context, normalizedEmail, expectedUserID, codeHash string, expiresAt, now time.Time) (bool, *domain.AppError)
	ConfirmPasswordReset(ctx context.Context, input PasswordResetConfirmInput, expectedUserID, codeHash string, credential PasswordCredential, now time.Time) *domain.AppError
}

type PasswordResetEmailSender interface {
	SendPasswordResetCode(ctx context.Context, toEmail, code string, expiresAt time.Time) *domain.AppError
}

type PasswordResetDeliveryRecorder interface {
	RecordPasswordResetDelivery(outcome string)
}

type passwordResetChallenge struct {
	ID        string
	UserID    string
	Email     string
	CodeHash  string
	CreatedAt time.Time
	ExpiresAt time.Time
	Attempts  int
	Consumed  bool
}

func (s *Service) SetPasswordResetDeliveryRecorder(recorder PasswordResetDeliveryRecorder) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.passwordResetDeliveryRecorder = recorder
	s.mu.Unlock()
}

func (s *Service) StartPasswordReset(ctx context.Context, input PasswordResetStartInput) (PasswordResetStartResult, *domain.AppError) {
	email, appErr := normalizePasswordResetEmail(input.Email)
	if appErr != nil {
		return PasswordResetStartResult{}, appErr
	}
	now := s.now()
	subject, appErr := s.passwordResetSubject(ctx, email)
	if appErr != nil {
		return PasswordResetStartResult{}, appErr
	}
	code := newVerificationCode()
	userID := subject.UserID
	if userID == "" {
		userID = passwordResetDummyUserID
	}
	codeHash := s.passwordResetCodeHash(userID, email, code)
	created, appErr := s.replacePasswordResetChallenge(ctx, email, subject, codeHash, now)
	if appErr != nil {
		return PasswordResetStartResult{}, appErr
	}
	result := PasswordResetStartResult{Accepted: true}
	if !created {
		return result, nil
	}
	sender, ok := s.registrationEmailSender.(PasswordResetEmailSender)
	if !ok || sender == nil {
		s.recordPasswordResetDelivery("failed")
		log.Printf("password_reset_email_delivery_failed request_id=%s user_id=%s code=EMAIL_SENDER_UNAVAILABLE", safeAuthLogValue(input.RequestID), safeAuthLogValue(subject.UserID))
		return result, nil
	}
	if sendErr := sender.SendPasswordResetCode(ctx, email, code, now.Add(PasswordResetCodeLifetime)); sendErr != nil {
		s.recordPasswordResetDelivery("failed")
		log.Printf("password_reset_email_delivery_failed request_id=%s user_id=%s code=%s", safeAuthLogValue(input.RequestID), safeAuthLogValue(subject.UserID), safeAuthLogValue(sendErr.Code))
		return result, nil
	}
	s.recordPasswordResetDelivery("sent")
	return result, nil
}

func (s *Service) ConfirmPasswordReset(ctx context.Context, input PasswordResetConfirmInput) *domain.AppError {
	email, appErr := normalizePasswordResetEmail(input.Email)
	if appErr != nil {
		return verificationCodeInvalidError()
	}
	input.Email = email
	input.Code = strings.TrimSpace(input.Code)
	if !verificationCodePattern.MatchString(input.Code) {
		return verificationCodeInvalidError()
	}
	if appErr := validateNewPassword(input.NewPassword); appErr != nil {
		return appErr
	}
	subject, appErr := s.passwordResetSubject(ctx, email)
	if appErr != nil {
		return appErr
	}
	userID := subject.UserID
	if userID == "" {
		userID = passwordResetDummyUserID
	}
	codeHash := s.passwordResetCodeHash(userID, email, input.Code)
	credential := newPasswordCredential(User{ID: userID}, input.NewPassword)
	credential.User.ID = subject.UserID
	if repo, ok := s.repo.(PasswordResetRepository); ok {
		return repo.ConfirmPasswordReset(ctx, input, subject.UserID, codeHash, credential, s.now())
	}
	if s.repo != nil {
		return internalAuthDependencyError()
	}
	return s.confirmPasswordResetMemory(input, subject, codeHash, credential)
}

func (s *Service) passwordResetSubject(ctx context.Context, email string) (PasswordResetSubject, *domain.AppError) {
	if repo, ok := s.repo.(PasswordResetRepository); ok {
		return repo.PasswordResetSubject(ctx, email)
	}
	if s.repo != nil {
		return PasswordResetSubject{}, internalAuthDependencyError()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	claim, ok := s.studentClaimsByEmail[email]
	if !ok {
		return PasswordResetSubject{}, nil
	}
	user := s.users[claim.UserID]
	return PasswordResetSubject{UserID: user.ID, Eligible: passwordResetEligible(user)}, nil
}

func (s *Service) replacePasswordResetChallenge(ctx context.Context, email string, subject PasswordResetSubject, codeHash string, now time.Time) (bool, *domain.AppError) {
	if repo, ok := s.repo.(PasswordResetRepository); ok {
		return repo.ReplacePasswordResetChallenge(ctx, email, subject.UserID, codeHash, now.Add(PasswordResetCodeLifetime), now)
	}
	if s.repo != nil {
		return false, internalAuthDependencyError()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, challenge := range s.passwordResetCodes {
		if challenge.Email == email && !challenge.Consumed {
			challenge.Consumed = true
			s.passwordResetCodes[id] = challenge
		}
	}
	claim, ok := s.studentClaimsByEmail[email]
	user := s.users[claim.UserID]
	if !ok || user.ID != subject.UserID || !passwordResetEligible(user) {
		return false, nil
	}
	challenge := passwordResetChallenge{
		ID: uuid.NewString(), UserID: user.ID, Email: email, CodeHash: codeHash,
		CreatedAt: now, ExpiresAt: now.Add(PasswordResetCodeLifetime),
	}
	s.passwordResetCodes[challenge.ID] = challenge
	return true, nil
}

func (s *Service) confirmPasswordResetMemory(input PasswordResetConfirmInput, subject PasswordResetSubject, codeHash string, credential PasswordCredential) *domain.AppError {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	var challengeID string
	var challenge passwordResetChallenge
	for id, candidate := range s.passwordResetCodes {
		if candidate.Email == input.Email && !candidate.Consumed && candidate.CreatedAt.After(challenge.CreatedAt) {
			challengeID, challenge = id, candidate
		}
	}
	user := s.users[subject.UserID]
	if challengeID == "" {
		return verificationCodeInvalidError()
	}
	if challenge.UserID != subject.UserID || !passwordResetEligible(user) || !now.Before(challenge.ExpiresAt) || challenge.Attempts >= PasswordResetMaxAttempts {
		challenge.Consumed = true
		s.passwordResetCodes[challengeID] = challenge
		return verificationCodeInvalidError()
	}
	if subtle.ConstantTimeCompare([]byte(challenge.CodeHash), []byte(codeHash)) != 1 {
		challenge.Attempts++
		challenge.Consumed = challenge.Attempts >= PasswordResetMaxAttempts
		s.passwordResetCodes[challengeID] = challenge
		return verificationCodeInvalidError()
	}
	challenge.Consumed = true
	s.passwordResetCodes[challengeID] = challenge
	credential.User = user
	s.passwordCredentialsByUserID[user.ID] = credential
	for id, session := range s.sessions {
		if session.UserID == user.ID && session.RevokedAt == nil {
			session.RevokedAt = &now
			s.sessions[id] = session
		}
	}
	for id, session := range s.restrictedBusinessSessions {
		if session.UserID == user.ID && session.RevokedAt == nil {
			session.RevokedAt = &now
			s.restrictedBusinessSessions[id] = session
		}
	}
	return nil
}

func (s *Service) passwordResetCodeHash(userID, email, code string) string {
	s.mu.Lock()
	pepper := append([]byte(nil), s.emailVerificationPepper...)
	s.mu.Unlock()
	mac := hmac.New(sha256.New, pepper)
	_, _ = mac.Write([]byte(PasswordResetPurpose))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(userID))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(email))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(code))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *Service) recordPasswordResetDelivery(outcome string) {
	s.mu.Lock()
	recorder := s.passwordResetDeliveryRecorder
	s.mu.Unlock()
	if recorder != nil {
		recorder.RecordPasswordResetDelivery(outcome)
	}
}

func normalizePasswordResetEmail(value string) (string, *domain.AppError) {
	raw := strings.TrimSpace(value)
	address, err := mail.ParseAddress(raw)
	if raw == "" || len([]rune(raw)) > 254 || strings.Count(raw, "@") != 1 || err != nil || address.Address != raw {
		return "", domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Email invalid", "请输入有效的学校邮箱。", "email", "invalid", "请输入有效的学校邮箱。")
	}
	return strings.ToLower(raw), nil
}

func passwordResetEligible(user User) bool {
	if user.ID == "" || user.SecurityLockedAt != nil || user.Status == AccountStatusArchived {
		return false
	}
	return user.Status == AccountStatusActive || user.Status == AccountStatusSuspended || user.Status == AccountStatusBanned
}

func safeAuthLogValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return value
}
