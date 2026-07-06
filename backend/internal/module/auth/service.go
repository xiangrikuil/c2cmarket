package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"sync"
	"time"

	"c2c-market/backend/internal/domain"

	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"
)

const (
	PasswordAlgorithmArgon2IDV1            = "argon2id_v1"
	PasswordAlgorithmSHA256SaltedV1        = "sha256_salted_v1"
	argon2idV1MemoryKB              uint32 = 64 * 1024
	argon2idV1Iterations            uint32 = 3
	argon2idV1Parallelism           uint8  = 1
	argon2idV1KeyLength             uint32 = 32
)

type Service struct {
	mu                          sync.Mutex
	now                         func() time.Time
	repo                        Repository
	registrationEmailSender     RegistrationEmailSender
	users                       map[string]User
	usersByUsername             map[string]string
	usersByVerifiedEmail        map[string]string
	sessions                    map[string]Session
	emailRegistrationCodes      map[string]emailRegistrationChallenge
	passwordCredentialsByUserID map[string]PasswordCredential
}

type RegistrationEmailSender interface {
	SendVerificationCode(ctx context.Context, toEmail, code string, expiresAt time.Time) *domain.AppError
	SendRegistrationSuccess(ctx context.Context, toEmail, username, displayName string, registeredAt time.Time) *domain.AppError
	ExposeDevCode() bool
}

type emailRegistrationChallenge struct {
	Email     string
	CodeHash  string
	ExpiresAt time.Time
	Consumed  bool
}

func NewService(repo Repository, now func() time.Time) *Service {
	return NewServiceWithRegistrationEmailSender(repo, now, nil)
}

func NewServiceWithRegistrationEmailSender(repo Repository, now func() time.Time, registrationEmailSender RegistrationEmailSender) *Service {
	if now == nil {
		now = time.Now
	}
	service := &Service{
		now:                         now,
		repo:                        repo,
		registrationEmailSender:     registrationEmailSender,
		users:                       make(map[string]User),
		usersByUsername:             make(map[string]string),
		usersByVerifiedEmail:        make(map[string]string),
		sessions:                    make(map[string]Session),
		emailRegistrationCodes:      make(map[string]emailRegistrationChallenge),
		passwordCredentialsByUserID: make(map[string]PasswordCredential),
	}
	service.ensureUserLocked("admin", true)
	service.ensureUserLocked("buyer", false)
	service.ensureUserLocked("seller", false)
	return service
}

func (s *Service) CreateDevSession(ctx context.Context, username string, isAdmin bool) (User, Session, *domain.AppError) {
	username = normalizeUsername(username)
	if username == "" {
		username = "buyer"
	}

	if s.repo != nil {
		now := s.now()
		user, appErr := s.repo.EnsureUser(ctx, username, isAdmin, now)
		if appErr != nil {
			return User{}, Session{}, appErr
		}
		session := Session{
			ID:        newSecret("sess"),
			UserID:    user.ID,
			CSRFToken: newSecret("csrf"),
			ExpiresAt: now.Add(24 * time.Hour),
		}
		if appErr := s.repo.CreateSession(ctx, user.ID, hashOpaqueToken(session.ID), hashOpaqueToken(session.CSRFToken), session.ExpiresAt, now); appErr != nil {
			return User{}, Session{}, appErr
		}
		return user, session, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	user := s.ensureUserLocked(username, isAdmin)
	if isAdmin && !user.IsAdmin {
		user.IsAdmin = true
		s.users[user.ID] = user
	}

	now := s.now()
	session := Session{
		ID:        newSecret("sess"),
		UserID:    user.ID,
		CSRFToken: newSecret("csrf"),
		ExpiresAt: now.Add(24 * time.Hour),
	}
	s.sessions[session.ID] = session
	return user, session, nil
}

func (s *Service) LoginWithOAuthProfile(ctx context.Context, profile OAuthProfile) (User, Session, *domain.AppError) {
	profile.Provider = strings.TrimSpace(profile.Provider)
	profile.Subject = strings.TrimSpace(profile.Subject)
	profile.Username = normalizeUsername(profile.Username)
	if profile.Provider == "" || profile.Subject == "" || profile.Username == "" {
		return User{}, Session{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid OAuth profile", "OAuth 用户资料不完整。", "profile", "required", "OAuth 用户资料不完整。")
	}
	if profile.DisplayName == "" {
		profile.DisplayName = profile.Username
	}
	if profile.TrustLevel <= 0 {
		profile.TrustLevel = 1
	}

	now := s.now()
	var user User
	var created bool
	if s.repo != nil {
		result, appErr := s.repo.UpsertOAuthUser(ctx, profile, now)
		if appErr != nil {
			return User{}, Session{}, appErr
		}
		user = result.User
		created = result.Created
	} else {
		s.mu.Lock()
		_, existed := s.usersByUsername[profile.Username]
		user = s.ensureUserLocked(profile.Username, profile.GrantAdmin)
		created = !existed
		user.DisplayName = strings.TrimSpace(profile.DisplayName)
		if profile.GrantAdmin {
			user.IsAdmin = true
		}
		user.LinuxDoBinding = &LinuxDoBinding{
			Bound:           true,
			LinuxDoUserID:   valueOrDefault(profile.LinuxDoUserID, profile.Subject),
			LinuxDoUsername: valueOrDefault(profile.LinuxDoUsername, profile.Username),
			TrustLevel:      profile.TrustLevel,
			AvatarURL:       valueOrDefault(profile.LinuxDoAvatarURL, profile.AvatarURL),
			BoundAt:         now,
			LastSyncedAt:    now,
		}
		s.users[user.ID] = user
		s.mu.Unlock()
	}
	session := Session{
		ID:        newSecret("sess"),
		UserID:    user.ID,
		CSRFToken: newSecret("csrf"),
		ExpiresAt: now.Add(24 * time.Hour),
	}
	if s.repo != nil {
		if appErr := s.repo.CreateSession(ctx, user.ID, hashOpaqueToken(session.ID), hashOpaqueToken(session.CSRFToken), session.ExpiresAt, now); appErr != nil {
			return User{}, Session{}, appErr
		}
	} else {
		s.mu.Lock()
		s.sessions[session.ID] = session
		s.mu.Unlock()
	}
	s.sendRegistrationSuccessIfNeeded(ctx, created, user, profile.Email, now)
	return user, session, nil
}

func (s *Service) sendRegistrationSuccessIfNeeded(ctx context.Context, created bool, user User, email string, registeredAt time.Time) {
	if !created || s.registrationEmailSender == nil {
		return
	}
	email = normalizeRegistrationEmail(email)
	if email == "" {
		log.Printf("注册成功邮件跳过：OAuth userinfo 未返回有效邮箱 user_id=%s", user.ID)
		return
	}
	if appErr := s.registrationEmailSender.SendRegistrationSuccess(ctx, email, user.Username, user.DisplayName, registeredAt); appErr != nil {
		log.Printf("注册成功邮件发送失败 user_id=%s code=%s title=%s", user.ID, appErr.Code, appErr.Title)
	}
}

func (s *Service) StartEmailRegistration(ctx context.Context, input EmailRegistrationStartInput) (EmailRegistrationChallenge, *domain.AppError) {
	_ = ctx
	_ = input
	return EmailRegistrationChallenge{}, emailRegistrationDisabledError()
}

func (s *Service) ConfirmEmailRegistration(ctx context.Context, input EmailRegistrationConfirmInput) (User, Session, *domain.AppError) {
	_ = ctx
	_ = input
	return User{}, Session{}, emailRegistrationDisabledError()
}

func (s *Service) LoginWithPassword(ctx context.Context, username, password string) (User, Session, *domain.AppError) {
	username = normalizeUsername(username)
	password = strings.TrimSpace(password)
	if username == "" || password == "" {
		return User{}, Session{}, domain.NewError(http.StatusUnauthorized, domain.CodeInvalidCredentials, "Invalid credentials", "用户名或密码不正确。")
	}
	var credential PasswordCredential
	var appErr *domain.AppError
	if s.repo != nil {
		credential, appErr = s.repo.PasswordCredential(ctx, username)
		if appErr != nil {
			return User{}, Session{}, appErr
		}
	} else {
		s.mu.Lock()
		userID := s.usersByUsername[username]
		credential = s.passwordCredentialsByUserID[userID]
		user := s.users[userID]
		if user.ID == "" || credential.User.ID == "" {
			s.mu.Unlock()
			return User{}, Session{}, domain.NewError(http.StatusUnauthorized, domain.CodeInvalidCredentials, "Invalid credentials", "用户名或密码不正确。")
		}
		credential.User = user
		s.mu.Unlock()
	}
	matched, needsRehash := passwordCredentialMatches(credential, password)
	if !matched {
		return User{}, Session{}, domain.NewError(http.StatusUnauthorized, domain.CodeInvalidCredentials, "Invalid credentials", "用户名或密码不正确。")
	}
	if credential.User.Status != "active" {
		return User{}, Session{}, domain.NewError(http.StatusForbidden, domain.CodeAccountRestricted, "Account restricted", "当前账号不可执行该操作。")
	}
	if appErr := requireNativePasswordUser(credential.User); appErr != nil {
		return User{}, Session{}, appErr
	}

	now := s.now()
	if needsRehash {
		if appErr := s.rehashPasswordCredential(ctx, credential, password, now); appErr != nil {
			return User{}, Session{}, appErr
		}
	}
	session := Session{
		ID:        newSecret("sess"),
		UserID:    credential.User.ID,
		CSRFToken: newSecret("csrf"),
		ExpiresAt: now.Add(24 * time.Hour),
	}
	if s.repo != nil {
		if appErr := s.repo.CreateSession(ctx, credential.User.ID, hashOpaqueToken(session.ID), hashOpaqueToken(session.CSRFToken), session.ExpiresAt, now); appErr != nil {
			return User{}, Session{}, appErr
		}
	} else {
		s.mu.Lock()
		s.sessions[session.ID] = session
		s.mu.Unlock()
	}
	return credential.User, session, nil
}

func (s *Service) BootstrapAdmin(ctx context.Context, input BootstrapAdminInput) (BootstrapAdminResult, *domain.AppError) {
	username := normalizeUsername(input.Username)
	if username == "" {
		username = "admin"
	}
	password := strings.TrimSpace(input.Password)
	if password == "" {
		return BootstrapAdminResult{}, nil
	}
	if !usernamePattern.MatchString(username) {
		return BootstrapAdminResult{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid username", "管理员用户名格式不正确。", "username", "invalid", "用户名只能包含 3-24 位字母、数字、下划线或连字符。")
	}
	if appErr := validateNewPassword(password); appErr != nil {
		return BootstrapAdminResult{}, appErr
	}

	credential := newPasswordCredential(User{
		Username:    username,
		DisplayName: username,
		IsAdmin:     true,
		Status:      "active",
	}, password)
	if s.repo != nil {
		return s.repo.BootstrapAdminPassword(ctx, credential, s.now())
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hasAdminPasswordCredentialLocked() {
		return BootstrapAdminResult{}, nil
	}
	user := s.ensureUserLocked(username, true)
	credential.User = user
	s.passwordCredentialsByUserID[user.ID] = credential
	return BootstrapAdminResult{User: user, Created: true}, nil
}

func (s *Service) SetPassword(ctx context.Context, input SetPasswordInput) *domain.AppError {
	input.UserID = strings.TrimSpace(input.UserID)
	input.CurrentPassword = strings.TrimSpace(input.CurrentPassword)
	input.NewPassword = strings.TrimSpace(input.NewPassword)
	if input.UserID == "" {
		return domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。")
	}
	if err := validateNewPassword(input.NewPassword); err != nil {
		return err
	}
	var credential PasswordCredential
	var user User
	var appErr *domain.AppError
	if s.repo != nil {
		user, appErr = s.repo.UserByID(ctx, input.UserID)
		if appErr != nil {
			return appErr
		}
		if appErr := requireNativePasswordUser(user); appErr != nil {
			return appErr
		}
		credential, appErr = s.repo.PasswordCredentialByUserID(ctx, input.UserID)
		if appErr != nil && appErr.Code != domain.CodeObjectNotFound {
			return appErr
		}
	} else {
		s.mu.Lock()
		user = s.users[input.UserID]
		if user.ID == "" {
			s.mu.Unlock()
			return domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。")
		}
		if appErr := requireNativePasswordUser(user); appErr != nil {
			s.mu.Unlock()
			return appErr
		}
		credential = s.passwordCredentialsByUserID[input.UserID]
		if credential.User.ID == "" {
			appErr = domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Password credential not found", "尚未设置备用密码。")
		} else {
			credential.User = user
		}
		s.mu.Unlock()
	}
	if appErr == nil {
		if input.CurrentPassword == "" {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Current password required", "修改备用密码必须输入当前密码。", "currentPassword", "required", "必须输入当前密码。")
		}
		matched, _ := passwordCredentialMatches(credential, input.CurrentPassword)
		if !matched {
			return domain.NewError(http.StatusUnauthorized, domain.CodeInvalidCredentials, "Invalid credentials", "当前密码不正确。")
		}
	}
	next := newPasswordCredential(User{ID: input.UserID}, input.NewPassword)
	if s.repo != nil {
		return s.repo.UpsertPasswordCredential(ctx, next, s.now())
	}
	s.mu.Lock()
	next.User = user
	s.passwordCredentialsByUserID[input.UserID] = next
	s.mu.Unlock()
	return nil
}

func (s *Service) PasswordConfigured(ctx context.Context, userID string) (bool, *domain.AppError) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return false, domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。")
	}
	if s.repo != nil {
		_, appErr := s.repo.PasswordCredentialByUserID(ctx, userID)
		if appErr != nil {
			if appErr.Code == domain.CodeObjectNotFound {
				return false, nil
			}
			return false, appErr
		}
		return true, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	credential := s.passwordCredentialsByUserID[userID]
	return credential.User.ID != "", nil
}

func (s *Service) GetSession(ctx context.Context, sessionID string) (User, Session, *domain.AppError) {
	if s.repo != nil {
		return s.repo.GetSession(ctx, hashOpaqueToken(sessionID), s.now())
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return User{}, Session{}, domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。")
	}
	if session.RevokedAt != nil {
		return User{}, Session{}, domain.NewError(http.StatusUnauthorized, domain.CodeSessionRevoked, "Session revoked", "当前会话已退出。")
	}
	if !s.now().Before(session.ExpiresAt) {
		return User{}, Session{}, domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session expired", "当前会话已过期。")
	}
	user, ok := s.users[session.UserID]
	if !ok || user.Status != "active" {
		return User{}, Session{}, domain.NewError(http.StatusForbidden, domain.CodeAccountRestricted, "Account restricted", "当前账号不可执行该操作。")
	}
	return user, session, nil
}

func (s *Service) GetSessionWithCSRF(ctx context.Context, sessionID, csrfToken string) (User, Session, *domain.AppError) {
	if s.repo != nil {
		return s.repo.GetSessionWithCSRF(ctx, hashOpaqueToken(sessionID), hashOpaqueToken(csrfToken), s.now())
	}
	user, session, appErr := s.GetSession(ctx, sessionID)
	if appErr != nil {
		return User{}, Session{}, appErr
	}
	if csrfToken != session.CSRFToken {
		return User{}, Session{}, domain.NewError(http.StatusForbidden, domain.CodeCSRFTokenInvalid, "CSRF token invalid", "CSRF token 无效或缺失。")
	}
	return user, session, nil
}

func (s *Service) RefreshSessionCSRF(ctx context.Context, sessionID string) (string, *domain.AppError) {
	csrfToken := newSecret("csrf")
	if s.repo != nil {
		if appErr := s.repo.RefreshSessionCSRF(ctx, hashOpaqueToken(sessionID), hashOpaqueToken(csrfToken), s.now()); appErr != nil {
			return "", appErr
		}
		return csrfToken, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		return "", domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。")
	}
	session.CSRFToken = csrfToken
	s.sessions[sessionID] = session
	return csrfToken, nil
}

func (s *Service) Logout(ctx context.Context, sessionID string) {
	if s.repo != nil {
		_ = s.repo.RevokeSession(ctx, hashOpaqueToken(sessionID), s.now())
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return
	}
	now := s.now()
	session.RevokedAt = &now
	s.sessions[sessionID] = session
}

func (s *Service) rehashPasswordCredential(ctx context.Context, credential PasswordCredential, password string, now time.Time) *domain.AppError {
	next := newPasswordCredential(User{ID: credential.User.ID}, password)
	if s.repo != nil {
		return s.repo.UpsertPasswordCredential(ctx, next, now)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if user := s.users[credential.User.ID]; user.ID != "" {
		next.User = user
	}
	s.passwordCredentialsByUserID[credential.User.ID] = next
	return nil
}

func (s *Service) hasAdminPasswordCredentialLocked() bool {
	for userID, credential := range s.passwordCredentialsByUserID {
		if credential.User.ID == "" {
			continue
		}
		user := s.users[userID]
		if user.IsAdmin {
			return true
		}
	}
	return false
}

func (s *Service) ensureUserLocked(username string, isAdmin bool) User {
	username = normalizeUsername(username)
	if id := s.usersByUsername[username]; id != "" {
		user := s.users[id]
		if isAdmin && !user.IsAdmin {
			user.IsAdmin = true
			s.users[id] = user
		}
		return user
	}
	user := User{
		ID:          uuid.NewString(),
		Username:    username,
		DisplayName: username,
		IsAdmin:     isAdmin,
		Status:      "active",
	}
	s.users[user.ID] = user
	s.usersByUsername[username] = user.ID
	return user
}

func normalizeUsername(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "-")
	return value
}

func normalizeRegistrationEmail(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	address, err := mail.ParseAddress(value)
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(address.Address))
}

func hashOpaqueToken(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func passwordCredentialMatches(credential PasswordCredential, password string) (bool, bool) {
	switch credential.Algorithm {
	case PasswordAlgorithmArgon2IDV1:
		return argon2idPasswordHashMatches(credential.Salt, password, credential.Hash), false
	case PasswordAlgorithmSHA256SaltedV1:
		return legacyPasswordHashMatches(credential.Salt, password, credential.Hash), true
	default:
		return false, false
	}
}

func newPasswordCredential(user User, password string) PasswordCredential {
	salt := newPasswordSalt()
	return PasswordCredential{
		User:      user,
		Algorithm: PasswordAlgorithmArgon2IDV1,
		Salt:      salt,
		Hash:      argon2idPasswordHash(salt, password),
	}
}

func argon2idPasswordHashMatches(salt, password, expectedHash string) bool {
	saltBytes, err := hex.DecodeString(strings.TrimSpace(salt))
	if err != nil || len(saltBytes) == 0 {
		return false
	}
	expected, err := hex.DecodeString(strings.TrimSpace(expectedHash))
	if err != nil || len(expected) != int(argon2idV1KeyLength) {
		return false
	}
	actual := argon2.IDKey([]byte(password), saltBytes, argon2idV1Iterations, argon2idV1MemoryKB, argon2idV1Parallelism, argon2idV1KeyLength)
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func argon2idPasswordHash(salt, password string) string {
	saltBytes, err := hex.DecodeString(strings.TrimSpace(salt))
	if err != nil || len(saltBytes) == 0 {
		panic("invalid argon2id password salt")
	}
	sum := argon2.IDKey([]byte(password), saltBytes, argon2idV1Iterations, argon2idV1MemoryKB, argon2idV1Parallelism, argon2idV1KeyLength)
	return hex.EncodeToString(sum)
}

func legacyPasswordHashMatches(salt, password, expectedHash string) bool {
	actual := legacyPasswordHash(salt, password)
	return subtle.ConstantTimeCompare([]byte(actual), []byte(strings.TrimSpace(expectedHash))) == 1
}

func legacyPasswordHash(salt, password string) string {
	sum := sha256.Sum256([]byte(salt + ":" + password))
	return hex.EncodeToString(sum[:])
}

func validateNewPassword(password string) *domain.AppError {
	if len([]rune(password)) < 8 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Password too short", "备用密码至少 8 个字符。", "newPassword", "too_short", "备用密码至少 8 个字符。")
	}
	if len([]rune(password)) > 128 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Password too long", "备用密码最多 128 个字符。", "newPassword", "too_long", "备用密码最多 128 个字符。")
	}
	return nil
}

func requireLinuxDoBoundUser(user User) *domain.AppError {
	if user.LinuxDoBinding == nil || !user.LinuxDoBinding.Bound {
		return linuxDoBindingRequiredError()
	}
	return nil
}

func requireNativePasswordUser(user User) *domain.AppError {
	if user.IsAdmin {
		return nil
	}
	return requireLinuxDoBoundUser(user)
}

func emailRegistrationDisabledError() *domain.AppError {
	return domain.NewError(http.StatusForbidden, domain.CodeEmailRegistrationDisabled, "Email registration disabled", "第一版本仅支持 linux.do OAuth 注册和登录。")
}

func linuxDoBindingRequiredError() *domain.AppError {
	return domain.NewError(http.StatusForbidden, domain.CodeLinuxDoBindingRequired, "linux.do binding required", "第一版本仅支持已绑定 linux.do 的账号使用备用密码。")
}

func newPasswordSalt() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf[:])
}

func newSecret(prefix string) string {
	var buf [24]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(buf[:])
}

func valueOrDefault(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return strings.TrimSpace(fallback)
	}
	return value
}

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,24}$`)
