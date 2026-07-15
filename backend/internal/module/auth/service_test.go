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
	adminUsers  []AdminUser

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

func (f *fakeAuthRepository) BootstrapAdminPassword(_ context.Context, credential PasswordCredential, _ time.Time) (BootstrapAdminResult, *domain.AppError) {
	if f.credential.User.IsAdmin && f.credential.User.ID != "" {
		return BootstrapAdminResult{}, nil
	}
	credential.User.ID = "bootstrap-admin"
	credential.User.IsAdmin = true
	credential.User.Status = "active"
	f.credential = credential
	return BootstrapAdminResult{User: credential.User, Created: true}, nil
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

func (f *fakeAuthRepository) ListAdminUsers(context.Context) ([]AdminUser, *domain.AppError) {
	return f.adminUsers, nil
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
	if credential.User.Username == "" {
		switch credential.User.ID {
		case f.credential.User.ID:
			credential.User = f.credential.User
		case f.user.ID:
			credential.User = f.user
		}
	}
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

func boundAdminUserForTest() User {
	return User{
		ID:          "user-admin",
		Username:    "admin",
		DisplayName: "C2CMarket Admin",
		IsAdmin:     true,
		Status:      "active",
		LinuxDoBinding: &LinuxDoBinding{
			Bound: true,
		},
	}
}

func boundUserForTest() User {
	return User{
		ID:       "user-oauth",
		Username: "oauth-user",
		Status:   "active",
		LinuxDoBinding: &LinuxDoBinding{
			Bound: true,
		},
	}
}

func argon2idCredentialForTest(user User, password string) PasswordCredential {
	credential := newPasswordCredential(user, password)
	credential.User = user
	return credential
}

func legacyCredentialForTest(user User, password string) PasswordCredential {
	salt := "test-salt"
	return PasswordCredential{
		User:      user,
		Algorithm: PasswordAlgorithmSHA256SaltedV1,
		Salt:      salt,
		Hash:      legacyPasswordHash(salt, password),
	}
}

func TestLoginWithArgon2idPasswordCreatesSession(t *testing.T) {
	repo := &fakeAuthRepository{
		credential: argon2idCredentialForTest(boundAdminUserForTest(), "unit-test-password"),
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

func TestLoginWithLegacyPasswordRehashesCredential(t *testing.T) {
	repo := &fakeAuthRepository{
		credential: legacyCredentialForTest(boundAdminUserForTest(), "unit-test-password"),
	}
	legacySalt := repo.credential.Salt
	legacyHash := repo.credential.Hash
	service := NewService(repo, func() time.Time { return time.Date(2026, 6, 23, 12, 0, 0, 0, time.UTC) })

	_, session, appErr := service.LoginWithPassword(context.Background(), "admin", "unit-test-password")
	if appErr != nil {
		t.Fatalf("legacy login with password: %v", appErr)
	}
	if session.ID == "" {
		t.Fatalf("expected session after legacy login")
	}
	if repo.credential.Algorithm != PasswordAlgorithmArgon2IDV1 {
		t.Fatalf("expected legacy credential to rehash to argon2id, got %+v", repo.credential)
	}
	if repo.credential.Salt == legacySalt || repo.credential.Hash == legacyHash {
		t.Fatalf("expected rehash to replace salt/hash")
	}
	if matched, needsRehash := passwordCredentialMatches(repo.credential, "unit-test-password"); !matched || needsRehash {
		t.Fatalf("expected rehashed credential to verify without another rehash")
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
		credential: argon2idCredentialForTest(boundAdminUserForTest(), "unit-test-password"),
	}
	original := repo.credential
	service := NewService(repo, time.Now)

	_, _, appErr := service.LoginWithPassword(context.Background(), "admin", "wrong-password")
	if appErr == nil || appErr.Code != domain.CodeInvalidCredentials {
		t.Fatalf("expected invalid credentials, got %v", appErr)
	}
	if repo.session.ID != "" {
		t.Fatalf("invalid password must not create session: %+v", repo.session)
	}
	if repo.credential.Algorithm != original.Algorithm || repo.credential.Salt != original.Salt || repo.credential.Hash != original.Hash {
		t.Fatalf("invalid password must not rehash credential: before=%+v after=%+v", original, repo.credential)
	}
}

func TestLoginWithPasswordRequiresLinuxDoBinding(t *testing.T) {
	user := User{ID: "user-email", Username: "email-user", Status: "active"}
	repo := &fakeAuthRepository{
		credential: argon2idCredentialForTest(user, "unit-test-password"),
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

func TestValidateNewPasswordRequiresLengthAndComposition(t *testing.T) {
	tests := []struct {
		name     string
		password string
		reason   string
	}{
		{name: "too short", password: "Aa1!", reason: "too_short"},
		{name: "too long", password: "Password1!Password1!Password1!Long", reason: "too_long"},
		{name: "missing digit", password: "Password!", reason: "composition_required"},
		{name: "missing symbol", password: "Password1", reason: "composition_required"},
		{name: "space is not a symbol", password: "Password1 ", reason: "composition_required"},
		{name: "missing letter", password: "12345678!", reason: "composition_required"},
		{name: "ascii letter required", password: "密码123456!", reason: "composition_required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appErr := validateNewPassword(tt.password)
			if appErr == nil || len(appErr.FieldErrors) != 1 || appErr.FieldErrors[0].Code != tt.reason {
				t.Fatalf("expected reason %s, got %v", tt.reason, appErr)
			}
		})
	}

	if appErr := validateNewPassword("Password1!"); appErr != nil {
		t.Fatalf("expected valid password, got %v", appErr)
	}
}

func TestSetPasswordCreatesCredentialWithoutCurrentPasswordForLinuxDoBoundUser(t *testing.T) {
	repo := &fakeAuthRepository{
		user: boundUserForTest(),
	}
	service := NewService(repo, time.Now)

	appErr := service.SetPassword(context.Background(), SetPasswordInput{
		UserID:      "user-oauth",
		NewPassword: "unit-test-password-1!",
	})
	if appErr != nil {
		t.Fatalf("set password: %v", appErr)
	}
	if repo.credential.User.ID != "user-oauth" || repo.credential.Hash == "" || repo.credential.Salt == "" {
		t.Fatalf("expected credential upsert, got %+v", repo.credential)
	}
	if repo.credential.Algorithm != PasswordAlgorithmArgon2IDV1 {
		t.Fatalf("expected argon2id credential, got %+v", repo.credential)
	}
}

func TestSetPasswordRequiresLinuxDoBinding(t *testing.T) {
	repo := &fakeAuthRepository{
		user: User{ID: "user-email", Username: "email-user", Status: "active"},
	}
	service := NewService(repo, time.Now)

	appErr := service.SetPassword(context.Background(), SetPasswordInput{
		UserID:      "user-email",
		NewPassword: "unit-test-password-1!",
	})
	if appErr == nil || appErr.Code != domain.CodeLinuxDoBindingRequired {
		t.Fatalf("expected linux.do binding required, got %v", appErr)
	}
	if repo.credential.User.ID != "" {
		t.Fatalf("unbound user must not get password credential: %+v", repo.credential)
	}
}

func TestSetPasswordRequiresCurrentPasswordWhenConfigured(t *testing.T) {
	user := boundUserForTest()
	repo := &fakeAuthRepository{
		credential: legacyCredentialForTest(user, "unit-test-password"),
	}
	legacyHash := repo.credential.Hash
	service := NewService(repo, time.Now)

	appErr := service.SetPassword(context.Background(), SetPasswordInput{
		UserID:      "user-oauth",
		NewPassword: "new-unit-test-password-1!",
	})
	if appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("expected current password validation error, got %v", appErr)
	}
	appErr = service.SetPassword(context.Background(), SetPasswordInput{
		UserID:          "user-oauth",
		CurrentPassword: "unit-test-password",
		NewPassword:     "new-unit-test-password-1!",
	})
	if appErr != nil {
		t.Fatalf("change password: %v", appErr)
	}
	if repo.credential.Hash == legacyHash {
		t.Fatalf("expected changed password hash")
	}
	if repo.credential.Algorithm != PasswordAlgorithmArgon2IDV1 {
		t.Fatalf("expected changed password to use argon2id, got %+v", repo.credential)
	}
}

func TestBootstrapAdminCreatesFirstAdminCredential(t *testing.T) {
	service := NewService(nil, func() time.Time {
		return time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC)
	})

	result, appErr := service.BootstrapAdmin(context.Background(), BootstrapAdminInput{
		Username: "Admin Root",
		Password: "bootstrap-password-1!",
	})
	if appErr != nil {
		t.Fatalf("bootstrap admin: %v", appErr)
	}
	if !result.Created || result.User.Username != "admin-root" || !result.User.IsAdmin {
		t.Fatalf("unexpected bootstrap result: %+v", result)
	}

	user, session, appErr := service.LoginWithPassword(context.Background(), "admin-root", "bootstrap-password-1!")
	if appErr != nil {
		t.Fatalf("login with bootstrapped admin: %v", appErr)
	}
	if !user.IsAdmin || session.ID == "" {
		t.Fatalf("unexpected bootstrapped admin login: user=%+v session=%+v", user, session)
	}
}

func TestBootstrapAdminDoesNotOverwriteExistingAdminCredential(t *testing.T) {
	service := NewService(nil, time.Now)

	first, appErr := service.BootstrapAdmin(context.Background(), BootstrapAdminInput{
		Username: "admin",
		Password: "first-bootstrap-password-1!",
	})
	if appErr != nil || !first.Created {
		t.Fatalf("first bootstrap admin result=%+v err=%v", first, appErr)
	}
	second, appErr := service.BootstrapAdmin(context.Background(), BootstrapAdminInput{
		Username: "admin",
		Password: "second-bootstrap-password-2!",
	})
	if appErr != nil {
		t.Fatalf("second bootstrap admin: %v", appErr)
	}
	if second.Created {
		t.Fatalf("second bootstrap must not overwrite existing admin credential: %+v", second)
	}

	if _, _, appErr := service.LoginWithPassword(context.Background(), "admin", "first-bootstrap-password-1!"); appErr != nil {
		t.Fatalf("first bootstrap password should still work: %v", appErr)
	}
	if _, _, appErr := service.LoginWithPassword(context.Background(), "admin", "second-bootstrap-password-2!"); appErr == nil || appErr.Code != domain.CodeInvalidCredentials {
		t.Fatalf("second bootstrap password must not work, got %v", appErr)
	}
}
