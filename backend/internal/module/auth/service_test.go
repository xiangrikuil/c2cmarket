package auth

import (
	"context"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
)

type fakeAuthRepository struct {
	oauthResult OAuthUserResult
	user        User
	credential  PasswordCredential
	session     Session

	ensureUserCalls                  int
	createEmailRegistrationCodeCalls int
	confirmEmailRegistrationCalls    int
	createSessionCalls               int
}

func (f *fakeAuthRepository) EnsureUser(context.Context, string, bool, time.Time) (User, *domain.AppError) {
	f.ensureUserCalls++
	return User{}, domain.NewError(500, domain.CodeInternalError, "not implemented", "not implemented")
}

func (f *fakeAuthRepository) UpsertOAuthUser(context.Context, OAuthProfile, time.Time) (OAuthUserResult, *domain.AppError) {
	return f.oauthResult, nil
}

func (f *fakeAuthRepository) UserByID(_ context.Context, userID string) (User, *domain.AppError) {
	if f.user.ID == userID {
		return f.user, nil
	}
	if f.credential.User.ID == userID {
		return f.credential.User, nil
	}
	return User{}, domain.NewError(401, domain.CodeSessionExpired, "Session required", "请先登录。")
}

func (f *fakeAuthRepository) PasswordCredential(_ context.Context, username string) (PasswordCredential, *domain.AppError) {
	if username != f.credential.User.Username {
		return PasswordCredential{}, domain.NewError(401, domain.CodeInvalidCredentials, "Invalid credentials", "用户名或密码不正确。")
	}
	return f.credential, nil
}

func (f *fakeAuthRepository) PasswordCredentialByUserID(_ context.Context, userID string) (PasswordCredential, *domain.AppError) {
	if userID != f.credential.User.ID {
		return PasswordCredential{}, domain.NewError(404, domain.CodeObjectNotFound, "Password credential not found", "尚未设置备用密码。")
	}
	return f.credential, nil
}

func (f *fakeAuthRepository) UpsertPasswordCredential(_ context.Context, credential PasswordCredential, _ time.Time) *domain.AppError {
	f.credential = credential
	return nil
}

func (f *fakeAuthRepository) CreateEmailRegistrationCode(context.Context, EmailRegistrationStartInput, string, time.Time, time.Time) *domain.AppError {
	f.createEmailRegistrationCodeCalls++
	return domain.NewError(500, domain.CodeInternalError, "not implemented", "not implemented")
}

func (f *fakeAuthRepository) ConfirmEmailRegistration(context.Context, EmailRegistrationConfirmInput, string, string, string, time.Time, time.Time) (User, *domain.AppError) {
	f.confirmEmailRegistrationCalls++
	return User{}, domain.NewError(500, domain.CodeInternalError, "not implemented", "not implemented")
}

func (f *fakeAuthRepository) CreateSession(_ context.Context, userID, sessionTokenHash, csrfTokenHash string, expiresAt, _ time.Time) *domain.AppError {
	f.createSessionCalls++
	f.session = Session{
		ID:        sessionTokenHash,
		UserID:    userID,
		CSRFToken: csrfTokenHash,
		ExpiresAt: expiresAt,
	}
	return nil
}

func (f *fakeAuthRepository) GetSession(context.Context, string, time.Time) (User, Session, *domain.AppError) {
	return User{}, Session{}, domain.NewError(500, domain.CodeInternalError, "not implemented", "not implemented")
}

func (f *fakeAuthRepository) GetSessionWithCSRF(context.Context, string, string, time.Time) (User, Session, *domain.AppError) {
	return User{}, Session{}, domain.NewError(500, domain.CodeInternalError, "not implemented", "not implemented")
}

func (f *fakeAuthRepository) RefreshSessionCSRF(context.Context, string, string, time.Time) *domain.AppError {
	return domain.NewError(500, domain.CodeInternalError, "not implemented", "not implemented")
}

func (f *fakeAuthRepository) RevokeSession(context.Context, string, time.Time) *domain.AppError {
	return nil
}

func TestLoginWithPasswordCreatesSession(t *testing.T) {
	repo := &fakeAuthRepository{
		credential: PasswordCredential{
			User: User{
				ID:          "user-admin",
				Username:    "admin",
				DisplayName: "C2CMarket Admin",
				IsAdmin:     true,
				Status:      "active",
				LinuxDoBinding: &LinuxDoBinding{
					Bound: true,
				},
			},
			Algorithm: PasswordAlgorithmSHA256SaltedV1,
			Salt:      "test-salt",
			Hash:      "d1012a27230a9cb86e493ce308459b904562e29a3fd87ec523e79f8554b96746",
		},
	}
	service := NewService(repo, func() time.Time { return time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC) })

	user, session, appErr := service.LoginWithPassword(context.Background(), "admin", "unit-test-password")
	if appErr != nil {
		t.Fatalf("login with password: %v", appErr)
	}
	if user.Username != "admin" || !user.IsAdmin {
		t.Fatalf("unexpected user: %+v", user)
	}
	if session.ID == "" || session.CSRFToken == "" || session.UserID != "user-admin" {
		t.Fatalf("unexpected session: %+v", session)
	}
	if repo.session.ID == "" || repo.session.CSRFToken == "" {
		t.Fatalf("expected persisted hashed session")
	}
}

type fakeRegistrationEmailSender struct {
	to          string
	username    string
	displayName string
	err         *domain.AppError
	calls       int
	codeTo      string
	code        string
}

func (f *fakeRegistrationEmailSender) SendVerificationCode(_ context.Context, toEmail, code string, _ time.Time) *domain.AppError {
	f.codeTo = toEmail
	f.code = code
	return f.err
}

func (f *fakeRegistrationEmailSender) SendRegistrationSuccess(_ context.Context, toEmail, username, displayName string, _ time.Time) *domain.AppError {
	f.calls++
	f.to = toEmail
	f.username = username
	f.displayName = displayName
	return f.err
}

func (f *fakeRegistrationEmailSender) SendCarpoolApplicationCreated(context.Context, string, string, string, time.Time) *domain.AppError {
	return nil
}

func (f *fakeRegistrationEmailSender) ExposeDevCode() bool {
	return true
}

func TestLoginWithOAuthProfileSendsRegistrationEmailForNewUserEmail(t *testing.T) {
	repo := &fakeAuthRepository{
		oauthResult: OAuthUserResult{
			User: User{
				ID:          "user-oauth",
				Username:    "oauth-user",
				DisplayName: "OAuth User",
				Status:      "active",
			},
			Created: true,
		},
	}
	sender := &fakeRegistrationEmailSender{}
	service := NewServiceWithRegistrationEmailSender(repo, func() time.Time { return time.Date(2026, 6, 26, 1, 0, 0, 0, time.UTC) }, sender)

	user, session, appErr := service.LoginWithOAuthProfile(context.Background(), OAuthProfile{
		Provider:    "linux_do",
		Subject:     "linuxdo-1",
		Username:    "oauth-user",
		DisplayName: "OAuth User",
		Email:       " OAuth.User@Example.COM ",
		TrustLevel:  3,
	})
	if appErr != nil {
		t.Fatalf("oauth login: %v", appErr)
	}
	if user.ID != "user-oauth" || session.ID == "" {
		t.Fatalf("unexpected login result user=%+v session=%+v", user, session)
	}
	if sender.calls != 1 || sender.to != "oauth.user@example.com" || sender.username != "oauth-user" || sender.displayName != "OAuth User" {
		t.Fatalf("unexpected registration email call: %+v", sender)
	}
}

func TestEmailRegistrationIsDisabled(t *testing.T) {
	repo := &fakeAuthRepository{}
	sender := &fakeRegistrationEmailSender{}
	service := NewServiceWithRegistrationEmailSender(repo, func() time.Time {
		return time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	}, sender)

	if _, appErr := service.StartEmailRegistration(context.Background(), EmailRegistrationStartInput{Email: " Test.User+Plan@Example.COM "}); appErr == nil || appErr.Code != domain.CodeEmailRegistrationDisabled {
		t.Fatalf("expected email registration disabled, got %v", appErr)
	}
	if _, _, appErr := service.ConfirmEmailRegistration(context.Background(), EmailRegistrationConfirmInput{
		Email: "test.user+plan@example.com",
		Code:  "123456",
	}); appErr == nil || appErr.Code != domain.CodeEmailRegistrationDisabled {
		t.Fatalf("expected email registration confirmation disabled, got %v", appErr)
	}
	if sender.codeTo != "" || sender.calls != 0 {
		t.Fatalf("disabled email registration must not send email: %+v", sender)
	}
	if repo.createEmailRegistrationCodeCalls != 0 || repo.confirmEmailRegistrationCalls != 0 || repo.ensureUserCalls != 0 || repo.createSessionCalls != 0 {
		t.Fatalf("disabled email registration must not write repo side effects: %+v", repo)
	}
	if repo.session.ID != "" {
		t.Fatalf("disabled email registration must not create session: %+v", repo.session)
	}
}

func TestLoginWithOAuthProfileSkipsRegistrationEmailForExistingUser(t *testing.T) {
	repo := &fakeAuthRepository{
		oauthResult: OAuthUserResult{
			User: User{
				ID:          "user-oauth",
				Username:    "oauth-user",
				DisplayName: "OAuth User",
				Status:      "active",
			},
			Created: false,
		},
	}
	sender := &fakeRegistrationEmailSender{}
	service := NewServiceWithRegistrationEmailSender(repo, time.Now, sender)

	_, _, appErr := service.LoginWithOAuthProfile(context.Background(), OAuthProfile{
		Provider:    "linux_do",
		Subject:     "linuxdo-1",
		Username:    "oauth-user",
		DisplayName: "OAuth User",
		Email:       "oauth.user@example.com",
		TrustLevel:  3,
	})
	if appErr != nil {
		t.Fatalf("oauth login: %v", appErr)
	}
	if sender.calls != 0 {
		t.Fatalf("existing user must not receive registration email: %+v", sender)
	}
}

func TestLoginWithOAuthProfileSkipsRegistrationEmailWithoutEmail(t *testing.T) {
	repo := &fakeAuthRepository{
		oauthResult: OAuthUserResult{
			User: User{
				ID:          "user-oauth",
				Username:    "oauth-user",
				DisplayName: "OAuth User",
				Status:      "active",
			},
			Created: true,
		},
	}
	sender := &fakeRegistrationEmailSender{}
	service := NewServiceWithRegistrationEmailSender(repo, time.Now, sender)

	_, _, appErr := service.LoginWithOAuthProfile(context.Background(), OAuthProfile{
		Provider:    "linux_do",
		Subject:     "linuxdo-1",
		Username:    "oauth-user",
		DisplayName: "OAuth User",
		TrustLevel:  3,
	})
	if appErr != nil {
		t.Fatalf("oauth login: %v", appErr)
	}
	if sender.calls != 0 {
		t.Fatalf("missing email must skip registration email: %+v", sender)
	}
}

func TestLoginWithOAuthProfileDoesNotFailWhenRegistrationEmailFails(t *testing.T) {
	repo := &fakeAuthRepository{
		oauthResult: OAuthUserResult{
			User: User{
				ID:          "user-oauth",
				Username:    "oauth-user",
				DisplayName: "OAuth User",
				Status:      "active",
			},
			Created: true,
		},
	}
	sender := &fakeRegistrationEmailSender{
		err: domain.NewError(502, domain.CodeInternalError, "Email send failed", "邮件发送失败，请稍后重试。"),
	}
	service := NewServiceWithRegistrationEmailSender(repo, time.Now, sender)

	user, session, appErr := service.LoginWithOAuthProfile(context.Background(), OAuthProfile{
		Provider:    "linux_do",
		Subject:     "linuxdo-1",
		Username:    "oauth-user",
		DisplayName: "OAuth User",
		Email:       "oauth.user@example.com",
		TrustLevel:  3,
	})
	if appErr != nil {
		t.Fatalf("registration email failure must not block oauth login: %v", appErr)
	}
	if user.ID == "" || session.ID == "" || sender.calls != 1 {
		t.Fatalf("unexpected login result user=%+v session=%+v sender=%+v", user, session, sender)
	}
}

func TestLoginWithPasswordRejectsInvalidPassword(t *testing.T) {
	repo := &fakeAuthRepository{
		credential: PasswordCredential{
			User: User{
				ID:          "user-admin",
				Username:    "admin",
				DisplayName: "C2CMarket Admin",
				IsAdmin:     true,
				Status:      "active",
				LinuxDoBinding: &LinuxDoBinding{
					Bound: true,
				},
			},
			Algorithm: PasswordAlgorithmSHA256SaltedV1,
			Salt:      "test-salt",
			Hash:      "d1012a27230a9cb86e493ce308459b904562e29a3fd87ec523e79f8554b96746",
		},
	}
	service := NewService(repo, time.Now)

	_, _, appErr := service.LoginWithPassword(context.Background(), "admin", "wrong-password")
	if appErr == nil || appErr.Code != domain.CodeInvalidCredentials {
		t.Fatalf("expected invalid credentials, got %v", appErr)
	}
	if repo.session.ID != "" {
		t.Fatalf("invalid password must not create session: %+v", repo.session)
	}
}

func TestLoginWithPasswordRequiresLinuxDoBinding(t *testing.T) {
	repo := &fakeAuthRepository{
		credential: PasswordCredential{
			User:      User{ID: "user-email", Username: "email-user", Status: "active"},
			Algorithm: PasswordAlgorithmSHA256SaltedV1,
			Salt:      "test-salt",
			Hash:      "d1012a27230a9cb86e493ce308459b904562e29a3fd87ec523e79f8554b96746",
		},
	}
	service := NewService(repo, time.Now)

	_, _, appErr := service.LoginWithPassword(context.Background(), "email-user", "unit-test-password")
	if appErr == nil || appErr.Code != domain.CodeLinuxDoBindingRequired {
		t.Fatalf("expected linux.do binding required, got %v", appErr)
	}
	if repo.session.ID != "" {
		t.Fatalf("unbound password login must not create session: %+v", repo.session)
	}
}

func TestSetPasswordCreatesCredentialWithoutCurrentPasswordForLinuxDoBoundUser(t *testing.T) {
	repo := &fakeAuthRepository{
		user: User{
			ID:       "user-oauth",
			Username: "oauth-user",
			Status:   "active",
			LinuxDoBinding: &LinuxDoBinding{
				Bound: true,
			},
		},
	}
	service := NewService(repo, time.Now)

	appErr := service.SetPassword(context.Background(), SetPasswordInput{
		UserID:      "user-oauth",
		NewPassword: "unit-test-password",
	})
	if appErr != nil {
		t.Fatalf("set password: %v", appErr)
	}
	if repo.credential.User.ID != "user-oauth" || repo.credential.Hash == "" || repo.credential.Salt == "" {
		t.Fatalf("expected credential upsert, got %+v", repo.credential)
	}
}

func TestSetPasswordRequiresLinuxDoBinding(t *testing.T) {
	repo := &fakeAuthRepository{
		user: User{ID: "user-email", Username: "email-user", Status: "active"},
	}
	service := NewService(repo, time.Now)

	appErr := service.SetPassword(context.Background(), SetPasswordInput{
		UserID:      "user-email",
		NewPassword: "unit-test-password",
	})
	if appErr == nil || appErr.Code != domain.CodeLinuxDoBindingRequired {
		t.Fatalf("expected linux.do binding required, got %v", appErr)
	}
	if repo.credential.User.ID != "" {
		t.Fatalf("unbound user must not get password credential: %+v", repo.credential)
	}
}

func TestSetPasswordRequiresCurrentPasswordWhenConfigured(t *testing.T) {
	repo := &fakeAuthRepository{
		credential: PasswordCredential{
			User: User{
				ID:       "user-oauth",
				Username: "oauth-user",
				Status:   "active",
				LinuxDoBinding: &LinuxDoBinding{
					Bound: true,
				},
			},
			Algorithm: PasswordAlgorithmSHA256SaltedV1,
			Salt:      "test-salt",
			Hash:      "d1012a27230a9cb86e493ce308459b904562e29a3fd87ec523e79f8554b96746",
		},
	}
	service := NewService(repo, time.Now)

	appErr := service.SetPassword(context.Background(), SetPasswordInput{
		UserID:      "user-oauth",
		NewPassword: "new-unit-test-password",
	})
	if appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("expected current password validation error, got %v", appErr)
	}
	appErr = service.SetPassword(context.Background(), SetPasswordInput{
		UserID:          "user-oauth",
		CurrentPassword: "unit-test-password",
		NewPassword:     "new-unit-test-password",
	})
	if appErr != nil {
		t.Fatalf("change password: %v", appErr)
	}
	if repo.credential.Hash == "d1012a27230a9cb86e493ce308459b904562e29a3fd87ec523e79f8554b96746" {
		t.Fatalf("expected changed password hash")
	}
}
