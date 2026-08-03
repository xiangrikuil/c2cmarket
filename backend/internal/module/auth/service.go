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
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"

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
	SessionIdleLifetime                    = 7 * 24 * time.Hour
	SessionRenewalInterval                 = 24 * time.Hour
	SessionAbsoluteLifetime                = 30 * 24 * time.Hour
	AccountAppealSessionLifetime           = 15 * time.Minute
)

type Service struct {
	mu                          sync.Mutex
	now                         func() time.Time
	repo                        Repository
	idempotency                 *idempotency.Service
	registrationEmailSender     RegistrationEmailSender
	users                       map[string]User
	adminUsers                  map[string]AdminUser
	adminAuditEntries           map[string][]AdminAccountAuditEntry
	usersByUsername             map[string]string
	usersByVerifiedEmail        map[string]string
	oauthUserIDs                map[string]string
	adminBootstrapRuns          map[string]adminBootstrapRun
	sessions                    map[string]Session
	accountAppealSessions       map[string]AccountAppealSession
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
	return NewServiceWithRegistrationEmailSenderAndIdempotency(repo, now, nil, nil)
}

func NewServiceWithRegistrationEmailSender(repo Repository, now func() time.Time, registrationEmailSender RegistrationEmailSender) *Service {
	return NewServiceWithRegistrationEmailSenderAndIdempotency(repo, now, registrationEmailSender, nil)
}

func NewServiceWithRegistrationEmailSenderAndIdempotency(repo Repository, now func() time.Time, registrationEmailSender RegistrationEmailSender, idempotencyService *idempotency.Service) *Service {
	if now == nil {
		now = time.Now
	}
	if idempotencyService == nil {
		idempotencyService = idempotency.NewService(nil, now)
	}
	service := &Service{
		now:                         now,
		repo:                        repo,
		idempotency:                 idempotencyService,
		registrationEmailSender:     registrationEmailSender,
		users:                       make(map[string]User),
		adminUsers:                  make(map[string]AdminUser),
		adminAuditEntries:           make(map[string][]AdminAccountAuditEntry),
		usersByUsername:             make(map[string]string),
		usersByVerifiedEmail:        make(map[string]string),
		oauthUserIDs:                make(map[string]string),
		adminBootstrapRuns:          make(map[string]adminBootstrapRun),
		sessions:                    make(map[string]Session),
		accountAppealSessions:       make(map[string]AccountAppealSession),
		emailRegistrationCodes:      make(map[string]emailRegistrationChallenge),
		passwordCredentialsByUserID: make(map[string]PasswordCredential),
	}
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
		session := newSession(user.ID, now)
		if appErr := s.persistSession(ctx, session, now); appErr != nil {
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
	session := newSession(user.ID, now)
	s.sessions[session.ID] = session
	return user, session, nil
}

func (s *Service) LoginWithOAuthProfile(ctx context.Context, profile OAuthProfile) (User, Session, *domain.AppError) {
	profile.Provider = CanonicalOAuthProvider(profile.Provider)
	profile.Subject = CanonicalOAuthSubject(profile.Subject)
	rawUsername := strings.TrimSpace(profile.Username)
	profile.Username = OAuthUsernameCandidate(profile.Username, profile.Provider, profile.Subject, 0)
	if profile.Provider == "" || profile.Subject == "" || rawUsername == "" {
		return User{}, Session{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid OAuth profile", "OAuth 用户资料不完整。", "profile", "required", "OAuth 用户资料不完整。")
	}
	if profile.DisplayName == "" {
		profile.DisplayName = profile.Username
	}
	if profile.TrustLevel <= 0 {
		profile.TrustLevel = 1
	}
	profile.Attribution = NormalizeRegistrationAttribution(profile.Attribution)

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
		identityKey := OAuthIdentityKey(profile.Provider, profile.Subject)
		userID := s.oauthUserIDs[identityKey]
		if userID != "" {
			user = s.users[userID]
		} else {
			for attempt := 0; ; attempt++ {
				candidate := OAuthUsernameCandidate(profile.Username, profile.Provider, profile.Subject, attempt)
				if s.usersByUsername[candidate] != "" {
					continue
				}
				user = User{
					ID:              uuid.NewString(),
					AnalyticsUserID: uuid.NewString(),
					Username:        candidate,
					DisplayName:     candidate,
					Status:          "active",
				}
				s.users[user.ID] = user
				s.usersByUsername[candidate] = user.ID
				s.oauthUserIDs[identityKey] = user.ID
				created = true
				break
			}
		}
		user.DisplayName = strings.TrimSpace(profile.DisplayName)
		if user.DisplayName == "" {
			user.DisplayName = user.Username
		}
		if IsLinuxDoProvider(profile.Provider) {
			boundAt := now
			if user.LinuxDoBinding != nil && !user.LinuxDoBinding.BoundAt.IsZero() {
				boundAt = user.LinuxDoBinding.BoundAt
			}
			user.LinuxDoBinding = &LinuxDoBinding{
				Bound:           true,
				LinuxDoUserID:   valueOrDefault(profile.LinuxDoUserID, profile.Subject),
				LinuxDoUsername: valueOrDefault(profile.LinuxDoUsername, profile.Username),
				TrustLevel:      profile.TrustLevel,
				AvatarURL:       valueOrDefault(profile.LinuxDoAvatarURL, profile.AvatarURL),
				BoundAt:         boundAt,
				LastSyncedAt:    now,
			}
		}
		s.users[user.ID] = user
		s.mu.Unlock()
	}
	if user.Status != "active" {
		return User{}, Session{}, domain.NewError(http.StatusForbidden, domain.CodeAccountRestricted, "Account restricted", "当前账号不可执行该操作。")
	}
	session := newSession(user.ID, now)
	session.NewRegistration = created
	if s.repo != nil {
		if appErr := s.persistSession(ctx, session, now); appErr != nil {
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

func (s *Service) StartAccountAppealSession(ctx context.Context, profile OAuthProfile) (User, AccountAppealSession, *domain.AppError) {
	provider := CanonicalOAuthProvider(profile.Provider)
	subject := CanonicalOAuthSubject(profile.Subject)
	if provider == "" || subject == "" {
		return User{}, AccountAppealSession{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid OAuth profile", "OAuth 用户资料不完整。", "profile", "required", "OAuth 用户资料不完整。")
	}
	if !IsLinuxDoProvider(provider) {
		return User{}, AccountAppealSession{}, accountAppealIneligibleError()
	}

	now := s.now()
	session := AccountAppealSession{
		ID:        newSecret("appeal_sess"),
		CSRFToken: newSecret("appeal_csrf"),
		CreatedAt: now,
		ExpiresAt: now.Add(AccountAppealSessionLifetime),
	}
	if s.repo != nil {
		user, found, appErr := s.repo.ResolveExistingOAuthUser(ctx, provider, subject)
		if appErr != nil {
			return User{}, AccountAppealSession{}, appErr
		}
		if !found || !eligibleAccountAppealStatus(user.Status) {
			return User{}, AccountAppealSession{}, accountAppealIneligibleError()
		}
		user, appErr = s.repo.CreateAccountAppealSession(
			ctx,
			user.ID,
			hashOpaqueToken(session.ID),
			hashOpaqueToken(session.CSRFToken),
			session.ExpiresAt,
			now,
		)
		if appErr != nil {
			return User{}, AccountAppealSession{}, appErr
		}
		session.UserID = user.ID
		return user, session, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	userID := s.oauthUserIDs[OAuthIdentityKey(provider, subject)]
	user := s.users[userID]
	if user.ID == "" || !eligibleAccountAppealStatus(user.Status) {
		return User{}, AccountAppealSession{}, accountAppealIneligibleError()
	}
	for sessionID, existing := range s.accountAppealSessions {
		if existing.UserID != user.ID || existing.RevokedAt != nil {
			continue
		}
		revokedAt := now
		existing.RevokedAt = &revokedAt
		s.accountAppealSessions[sessionID] = existing
	}
	session.UserID = user.ID
	s.accountAppealSessions[session.ID] = session
	return user, session, nil
}

func (s *Service) GetAccountAppealSession(ctx context.Context, sessionID string) (User, AccountAppealSession, *domain.AppError) {
	csrfToken := newSecret("appeal_csrf")
	if s.repo != nil {
		user, session, appErr := s.repo.RotateAccountAppealSessionCSRF(ctx, hashOpaqueToken(sessionID), hashOpaqueToken(csrfToken), s.now())
		if appErr != nil {
			return User{}, AccountAppealSession{}, appErr
		}
		session.ID = sessionID
		session.CSRFToken = csrfToken
		return user, session, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	user, session, appErr := s.accountAppealSessionLocked(sessionID, "", false)
	if appErr != nil {
		return User{}, AccountAppealSession{}, appErr
	}
	session.CSRFToken = csrfToken
	s.accountAppealSessions[sessionID] = session
	return user, session, nil
}

func (s *Service) GetAccountAppealSessionWithCSRF(ctx context.Context, sessionID, csrfToken string) (User, AccountAppealSession, *domain.AppError) {
	if s.repo != nil {
		user, session, appErr := s.repo.GetAccountAppealSessionWithCSRF(ctx, hashOpaqueToken(sessionID), hashOpaqueToken(csrfToken), s.now())
		if appErr != nil {
			return User{}, AccountAppealSession{}, appErr
		}
		session.ID = sessionID
		return user, session, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.accountAppealSessionLocked(sessionID, csrfToken, true)
}

func (s *Service) accountAppealSessionLocked(sessionID, csrfToken string, requireCSRF bool) (User, AccountAppealSession, *domain.AppError) {
	session, ok := s.accountAppealSessions[sessionID]
	if !ok {
		if requireCSRF {
			return User{}, AccountAppealSession{}, accountAppealCSRFError()
		}
		return User{}, AccountAppealSession{}, accountAppealSessionExpiredError()
	}
	if requireCSRF && subtle.ConstantTimeCompare([]byte(session.CSRFToken), []byte(csrfToken)) != 1 {
		return User{}, AccountAppealSession{}, accountAppealCSRFError()
	}
	if session.RevokedAt != nil {
		return User{}, AccountAppealSession{}, accountAppealSessionRevokedError()
	}
	if !s.now().Before(session.ExpiresAt) {
		return User{}, AccountAppealSession{}, accountAppealSessionExpiredError()
	}
	user := s.users[session.UserID]
	if user.ID == "" || !eligibleAccountAppealStatus(user.Status) {
		return User{}, AccountAppealSession{}, accountAppealSessionExpiredError()
	}
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
	session := newSession(credential.User.ID, now)
	if s.repo != nil {
		if appErr := s.persistSession(ctx, session, now); appErr != nil {
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
	if run, ok := s.adminBootstrapRuns[InitialAdminBootstrapKey]; ok {
		user := s.users[run.UserID]
		credential := s.passwordCredentialsByUserID[run.UserID]
		if user.ID == "" ||
			run.Username != user.Username ||
			user.Status != "active" ||
			!user.IsAdmin ||
			s.usersByUsername[run.Username] != run.UserID ||
			credential.User.ID != user.ID {
			return BootstrapAdminResult{}, AdminBootstrapInconsistentError()
		}
		if username != run.Username {
			return BootstrapAdminResult{}, AdminBootstrapConflictError()
		}
		return BootstrapAdminResult{User: user}, nil
	}
	for _, user := range s.users {
		if user.IsAdmin {
			return BootstrapAdminResult{}, AdminBootstrapConflictError()
		}
	}
	if s.usersByUsername[username] != "" {
		return BootstrapAdminResult{}, AdminBootstrapConflictError()
	}
	user := User{
		ID:              uuid.NewString(),
		AnalyticsUserID: uuid.NewString(),
		Username:        username,
		DisplayName:     username,
		IsAdmin:         true,
		Status:          "active",
	}
	s.users[user.ID] = user
	s.usersByUsername[user.Username] = user.ID
	credential.User = user
	s.passwordCredentialsByUserID[user.ID] = credential
	s.adminBootstrapRuns[InitialAdminBootstrapKey] = adminBootstrapRun{
		UserID:   user.ID,
		Username: user.Username,
	}
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
	now := s.now()
	if !now.Before(session.ExpiresAt) || !now.Before(session.AbsoluteExpiresAt) {
		return User{}, Session{}, domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session expired", "当前会话已过期。")
	}
	user, ok := s.users[session.UserID]
	if !ok || user.Status != "active" {
		return User{}, Session{}, domain.NewError(http.StatusForbidden, domain.CodeAccountRestricted, "Account restricted", "当前账号不可执行该操作。")
	}
	return user, session, nil
}

func (s *Service) RenewSession(ctx context.Context, sessionID string) (Session, bool, *domain.AppError) {
	now := s.now()
	targetExpiresAt := now.Add(SessionIdleLifetime)
	renewBefore := now.Add(-SessionRenewalInterval)
	if s.repo != nil {
		expiresAt, renewed, appErr := s.repo.RenewSession(ctx, hashOpaqueToken(sessionID), now, targetExpiresAt, renewBefore)
		if appErr != nil || !renewed {
			return Session{}, renewed, appErr
		}
		return Session{ID: sessionID, ExpiresAt: expiresAt, RenewedAt: now}, true, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[sessionID]
	if !ok || session.RevokedAt != nil || !now.Before(session.ExpiresAt) || !now.Before(session.AbsoluteExpiresAt) {
		return Session{}, false, nil
	}
	if session.RenewedAt.After(renewBefore) {
		return Session{}, false, nil
	}
	if targetExpiresAt.After(session.AbsoluteExpiresAt) {
		targetExpiresAt = session.AbsoluteExpiresAt
	}
	session.RenewedAt = now
	session.ExpiresAt = targetExpiresAt
	s.sessions[sessionID] = session
	return session, true, nil
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

func (s *Service) AdminUsers(ctx context.Context, user User, query AdminUserDirectoryQuery) (AdminUserDirectory, *domain.AppError) {
	if !user.IsAdmin {
		return AdminUserDirectory{}, adminPermissionRequired()
	}
	query, appErr := normalizeAdminUserDirectoryQuery(query)
	if appErr != nil {
		return AdminUserDirectory{}, appErr
	}
	if s.repo != nil {
		return s.repo.ListAdminUsers(ctx, query)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	items := make([]AdminUser, 0, len(s.users))
	summary := AdminUserDirectorySummary{}
	for _, item := range s.users {
		adminUser := s.adminUserLocked(item)
		addAdminUserSummary(&summary, adminUser)
		if adminUserMatchesQuery(adminUser, query) {
			items = append(items, adminUser)
		}
	}
	sortAdminUsers(items, query.Sort)
	totalItems := len(items)
	totalPages := 0
	if totalItems > 0 {
		totalPages = (totalItems + query.Limit - 1) / query.Limit
	}
	start := (query.Page - 1) * query.Limit
	if start >= totalItems {
		items = []AdminUser{}
	} else {
		end := min(start+query.Limit, totalItems)
		items = append([]AdminUser(nil), items[start:end]...)
	}
	return AdminUserDirectory{
		Items: items,
		Pagination: AdminUserPagination{
			Page:       query.Page,
			Limit:      query.Limit,
			TotalItems: totalItems,
			TotalPages: totalPages,
		},
		Summary: summary,
	}, nil
}

func (s *Service) AdminUser(ctx context.Context, user User, userID string) (AdminUserDetail, *domain.AppError) {
	if !user.IsAdmin {
		return AdminUserDetail{}, adminPermissionRequired()
	}
	userID = strings.TrimSpace(userID)
	if _, err := uuid.Parse(userID); err != nil {
		return AdminUserDetail{}, adminUserValidationError("id", "用户 ID 格式不正确。")
	}
	if s.repo != nil {
		detail, appErr := s.repo.AdminUserDetail(ctx, userID)
		if appErr != nil {
			return AdminUserDetail{}, appErr
		}
		return decorateAdminUserDetail(detail, user.ID), nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	detail, appErr := s.adminUserDetailLocked(userID)
	if appErr != nil {
		return AdminUserDetail{}, appErr
	}
	return decorateAdminUserDetail(detail, user.ID), nil
}

func (s *Service) UpdateAdminUserStatusWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input AdminUserStatusInput, buildCompletion AdminUserCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if !user.IsAdmin {
		return idempotency.Completion{}, adminPermissionRequired()
	}
	input.AdminUserID = strings.TrimSpace(user.ID)
	input.TargetUserID = strings.TrimSpace(input.TargetUserID)
	input.Status = strings.TrimSpace(input.Status)
	input.Reason = strings.TrimSpace(input.Reason)
	if appErr := validateAdminUserStatusInput(input); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if buildCompletion == nil {
		return idempotency.Completion{}, adminUserInternalError()
	}
	buildCompletion = decorateAdminUserCompletionBuilder(input.AdminUserID, buildCompletion)
	entry, appErr := s.idempotency.Begin(ctx, input.AdminUserID, routeKey, strings.TrimSpace(key), requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}
	if s.repo != nil {
		_, completion, appErr := s.repo.UpdateAdminUserStatusWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}
	result, appErr := s.updateAdminUserStatusMemory(input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	return s.completeAdminUserMemoryMutation(ctx, entry, result, buildCompletion)
}

func (s *Service) UpdateAdminUserPermissionWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input AdminUserPermissionInput, buildCompletion AdminUserCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if !user.IsAdmin {
		return idempotency.Completion{}, adminPermissionRequired()
	}
	input.AdminUserID = strings.TrimSpace(user.ID)
	input.TargetUserID = strings.TrimSpace(input.TargetUserID)
	input.Reason = strings.TrimSpace(input.Reason)
	if appErr := validateAdminUserPermissionInput(input); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if buildCompletion == nil {
		return idempotency.Completion{}, adminUserInternalError()
	}
	buildCompletion = decorateAdminUserCompletionBuilder(input.AdminUserID, buildCompletion)
	entry, appErr := s.idempotency.Begin(ctx, input.AdminUserID, routeKey, strings.TrimSpace(key), requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}
	if s.repo != nil {
		_, completion, appErr := s.repo.UpdateAdminUserPermissionWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}
	result, appErr := s.updateAdminUserPermissionMemory(input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	return s.completeAdminUserMemoryMutation(ctx, entry, result, buildCompletion)
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

func newSession(userID string, now time.Time) Session {
	return Session{
		ID:                newSecret("sess"),
		UserID:            userID,
		CSRFToken:         newSecret("csrf"),
		ExpiresAt:         now.Add(SessionIdleLifetime),
		RenewedAt:         now,
		AbsoluteExpiresAt: now.Add(SessionAbsoluteLifetime),
	}
}

func (s *Service) persistSession(ctx context.Context, session Session, now time.Time) *domain.AppError {
	return s.repo.CreateSession(
		ctx,
		session.UserID,
		hashOpaqueToken(session.ID),
		hashOpaqueToken(session.CSRFToken),
		session.ExpiresAt,
		session.AbsoluteExpiresAt,
		now,
	)
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
		ID:              uuid.NewString(),
		AnalyticsUserID: uuid.NewString(),
		Username:        username,
		DisplayName:     username,
		IsAdmin:         isAdmin,
		Status:          "active",
	}
	s.users[user.ID] = user
	s.usersByUsername[username] = user.ID
	return user
}

func normalizeAdminUserDirectoryQuery(query AdminUserDirectoryQuery) (AdminUserDirectoryQuery, *domain.AppError) {
	query.Search = strings.TrimSpace(query.Search)
	query.Status = strings.TrimSpace(query.Status)
	query.Role = strings.TrimSpace(query.Role)
	query.LinuxDo = strings.TrimSpace(query.LinuxDo)
	query.Sort = strings.TrimSpace(query.Sort)
	if query.Page == 0 {
		query.Page = 1
	}
	if query.Limit == 0 {
		query.Limit = 20
	}
	if query.Status == "" {
		query.Status = AdminUserStatusAll
	}
	if query.Role == "" {
		query.Role = AdminUserRoleAll
	}
	if query.LinuxDo == "" {
		query.LinuxDo = AdminUserLinuxDoAll
	}
	if query.Sort == "" {
		query.Sort = AdminUserSortCreatedDesc
	}
	if query.Page < 1 {
		return AdminUserDirectoryQuery{}, adminUserValidationError("page", "页码必须是正整数。")
	}
	if query.Limit != 20 && query.Limit != 50 && query.Limit != 100 {
		return AdminUserDirectoryQuery{}, adminUserValidationError("limit", "每页数量仅支持 20、50 或 100。")
	}
	if utf8.RuneCountInString(query.Search) > 100 {
		return AdminUserDirectoryQuery{}, adminUserValidationError("search", "搜索内容最多 100 字。")
	}
	if !stringIn(query.Status, AdminUserStatusAll, AccountStatusActive, AccountStatusSuspended, AccountStatusBanned, AccountStatusArchived) {
		return AdminUserDirectoryQuery{}, adminUserValidationError("status", "账号状态筛选值不受支持。")
	}
	if !stringIn(query.Role, AdminUserRoleAll, AdminUserRoleAdmin, AdminUserRoleUser) {
		return AdminUserDirectoryQuery{}, adminUserValidationError("role", "账号角色筛选值不受支持。")
	}
	if !stringIn(query.LinuxDo, AdminUserLinuxDoAll, AdminUserLinuxDoBound, AdminUserLinuxDoUnbound) {
		return AdminUserDirectoryQuery{}, adminUserValidationError("linuxDo", "linux.do 绑定筛选值不受支持。")
	}
	if !stringIn(query.Sort, AdminUserSortCreatedDesc, AdminUserSortCreatedAsc, AdminUserSortActiveDesc, AdminUserSortUsernameAsc, AdminUserSortUsernameDesc) {
		return AdminUserDirectoryQuery{}, adminUserValidationError("sort", "排序方式不受支持。")
	}
	return query, nil
}

func validateAdminUserStatusInput(input AdminUserStatusInput) *domain.AppError {
	if input.AdminUserID == "" {
		return domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。")
	}
	if input.TargetUserID == "" {
		return adminUserValidationError("id", "必须提供目标用户 ID。")
	}
	if input.TargetUserID == input.AdminUserID {
		return domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Self management forbidden", "不能修改自己的账号状态。")
	}
	if !stringIn(input.Status, AccountStatusActive, AccountStatusSuspended, AccountStatusBanned, AccountStatusArchived) {
		return adminUserValidationError("status", "目标账号状态不受支持。")
	}
	return validateAdminUserMutationReason(input.ExpectedVersion, input.Reason)
}

func validateAdminUserPermissionInput(input AdminUserPermissionInput) *domain.AppError {
	if input.AdminUserID == "" {
		return domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。")
	}
	if input.TargetUserID == "" {
		return adminUserValidationError("id", "必须提供目标用户 ID。")
	}
	if input.TargetUserID == input.AdminUserID {
		return domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Self management forbidden", "不能修改自己的管理员权限。")
	}
	return validateAdminUserMutationReason(input.ExpectedVersion, input.Reason)
}

func validateAdminUserMutationReason(expectedVersion int64, reason string) *domain.AppError {
	if expectedVersion < 1 {
		return adminUserValidationError("version", "必须提供有效账号版本。")
	}
	if strings.TrimSpace(reason) == "" {
		return adminUserValidationError("reason", "账号治理操作必须填写原因。")
	}
	if utf8.RuneCountInString(reason) > 500 {
		return adminUserValidationError("reason", "操作原因最多 500 字。")
	}
	return nil
}

func (s *Service) adminUserLocked(user User) AdminUser {
	item, exists := s.adminUsers[user.ID]
	if !exists {
		now := s.now()
		item = AdminUser{
			ID:        user.ID,
			CreatedAt: now,
			UpdatedAt: now,
			Version:   1,
		}
	}
	item.Username = user.Username
	item.DisplayName = user.DisplayName
	item.IsAdmin = user.IsAdmin
	item.Status = user.Status
	item.LinuxDoBound = user.LinuxDoBinding != nil && user.LinuxDoBinding.Bound
	item.TrustLevel = nil
	if item.LinuxDoBound {
		value := user.LinuxDoBinding.TrustLevel
		item.TrustLevel = &value
	}
	s.adminUsers[user.ID] = item
	return item
}

func addAdminUserSummary(summary *AdminUserDirectorySummary, user AdminUser) {
	summary.TotalUsers++
	if user.IsAdmin {
		summary.AdminUsers++
	}
	if user.LinuxDoBound {
		summary.LinuxDoBoundUsers++
	}
	switch user.Status {
	case AccountStatusActive:
		summary.ActiveUsers++
	case AccountStatusSuspended:
		summary.SuspendedUsers++
	case AccountStatusBanned:
		summary.BannedUsers++
	case AccountStatusArchived:
		summary.ArchivedUsers++
	}
}

func adminUserMatchesQuery(user AdminUser, query AdminUserDirectoryQuery) bool {
	if query.Status != AdminUserStatusAll && user.Status != query.Status {
		return false
	}
	if query.Role == AdminUserRoleAdmin && !user.IsAdmin {
		return false
	}
	if query.Role == AdminUserRoleUser && user.IsAdmin {
		return false
	}
	if query.LinuxDo == AdminUserLinuxDoBound && !user.LinuxDoBound {
		return false
	}
	if query.LinuxDo == AdminUserLinuxDoUnbound && user.LinuxDoBound {
		return false
	}
	search := strings.ToLower(query.Search)
	return search == "" || strings.Contains(strings.ToLower(user.Username+" "+user.DisplayName), search)
}

func sortAdminUsers(items []AdminUser, sortValue string) {
	sort.SliceStable(items, func(i, j int) bool {
		left, right := items[i], items[j]
		switch sortValue {
		case AdminUserSortCreatedAsc:
			if !left.CreatedAt.Equal(right.CreatedAt) {
				return left.CreatedAt.Before(right.CreatedAt)
			}
		case AdminUserSortActiveDesc:
			if left.LastActiveAt == nil || right.LastActiveAt == nil {
				if left.LastActiveAt != right.LastActiveAt {
					return left.LastActiveAt != nil
				}
			} else if !left.LastActiveAt.Equal(*right.LastActiveAt) {
				return left.LastActiveAt.After(*right.LastActiveAt)
			}
		case AdminUserSortUsernameAsc:
			if left.Username != right.Username {
				return left.Username < right.Username
			}
		case AdminUserSortUsernameDesc:
			if left.Username != right.Username {
				return left.Username > right.Username
			}
		default:
			if !left.CreatedAt.Equal(right.CreatedAt) {
				return left.CreatedAt.After(right.CreatedAt)
			}
		}
		return left.ID < right.ID
	})
}

func (s *Service) adminUserDetailLocked(userID string) (AdminUserDetail, *domain.AppError) {
	user, ok := s.users[userID]
	if !ok {
		return AdminUserDetail{}, adminUserNotFound()
	}
	item := s.adminUserLocked(user)
	detail := AdminUserDetail{
		User:               item,
		Providers:          []AdminAuthProvider{},
		RecentAuditEntries: append([]AdminAccountAuditEntry(nil), s.adminAuditEntries[userID]...),
		ActiveAdminCount:   s.activeAdminCountLocked(),
	}
	if user.LinuxDoBinding != nil && user.LinuxDoBinding.Bound {
		boundAt := user.LinuxDoBinding.BoundAt
		lastSyncedAt := user.LinuxDoBinding.LastSyncedAt
		detail.LinuxDoBinding = AdminLinuxDoBinding{
			Bound:        true,
			Username:     user.LinuxDoBinding.LinuxDoUsername,
			TrustLevel:   user.LinuxDoBinding.TrustLevel,
			BoundAt:      &boundAt,
			LastSyncedAt: &lastSyncedAt,
		}
	}
	for _, verifiedUserID := range s.usersByVerifiedEmail {
		if verifiedUserID == userID {
			detail.EmailVerified = true
			break
		}
	}
	detail.BackupPasswordConfigured = s.passwordCredentialsByUserID[userID].User.ID != ""
	for _, session := range s.sessions {
		if session.UserID != userID || session.RevokedAt != nil || !s.now().Before(session.ExpiresAt) || !s.now().Before(session.AbsoluteExpiresAt) {
			continue
		}
		detail.ActiveSessionCount++
		latest := session.RenewedAt
		if detail.LatestSessionActivityAt == nil || latest.After(*detail.LatestSessionActivityAt) {
			detail.LatestSessionActivityAt = &latest
		}
	}
	detail.ImpactPreview.ActiveSessions = detail.ActiveSessionCount
	return detail, nil
}

func (s *Service) updateAdminUserStatusMemory(input AdminUserStatusInput) (AdminUserMutationResult, *domain.AppError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[input.TargetUserID]
	if !ok {
		return AdminUserMutationResult{}, adminUserNotFound()
	}
	current := s.adminUserLocked(user)
	if current.Version != input.ExpectedVersion {
		return AdminUserMutationResult{}, adminUserVersionConflict()
	}
	if !AllowedAdminUserStatusTransition(current.Status, input.Status) {
		return AdminUserMutationResult{}, adminUserInvalidTransition("当前账号状态不能执行该变更。")
	}
	if current.IsAdmin && current.Status == AccountStatusActive && input.Status != AccountStatusActive && s.activeAdminCountLocked() <= 1 {
		return AdminUserMutationResult{}, adminUserInvalidTransition("不能停用最后一个有效管理员账号。")
	}
	beforeStatus := current.Status
	now := s.now()
	user.Status = input.Status
	s.users[user.ID] = user
	current.Status = input.Status
	current.Version++
	current.UpdatedAt = now
	s.adminUsers[user.ID] = current
	if beforeStatus == AccountStatusActive && input.Status != AccountStatusActive {
		for id, session := range s.sessions {
			if session.UserID == user.ID && session.RevokedAt == nil {
				revokedAt := now
				session.RevokedAt = &revokedAt
				s.sessions[id] = session
			}
		}
	}
	s.appendAdminAuditEntryLocked(user.ID, AdminAccountAuditEntry{
		ID:           uuid.NewString(),
		AdminUserID:  input.AdminUserID,
		Action:       "user.account_status_changed",
		Reason:       input.Reason,
		BeforeStatus: beforeStatus,
		AfterStatus:  input.Status,
		RequestID:    input.RequestID,
		CreatedAt:    now,
	})
	detail, appErr := s.adminUserDetailLocked(user.ID)
	return AdminUserMutationResult{Detail: detail}, appErr
}

func (s *Service) updateAdminUserPermissionMemory(input AdminUserPermissionInput) (AdminUserMutationResult, *domain.AppError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[input.TargetUserID]
	if !ok {
		return AdminUserMutationResult{}, adminUserNotFound()
	}
	current := s.adminUserLocked(user)
	if current.Version != input.ExpectedVersion {
		return AdminUserMutationResult{}, adminUserVersionConflict()
	}
	if current.IsAdmin == input.Grant {
		return AdminUserMutationResult{}, adminUserInvalidTransition("账号管理员权限没有变化。")
	}
	if input.Grant && current.Status != AccountStatusActive {
		return AdminUserMutationResult{}, adminUserInvalidTransition("只能向有效账号授予管理员权限。")
	}
	if !input.Grant && current.Status == AccountStatusActive && s.activeAdminCountLocked() <= 1 {
		return AdminUserMutationResult{}, adminUserInvalidTransition("不能撤销最后一个有效管理员的权限。")
	}
	beforeIsAdmin := current.IsAdmin
	now := s.now()
	user.IsAdmin = input.Grant
	s.users[user.ID] = user
	current.IsAdmin = input.Grant
	current.Version++
	current.UpdatedAt = now
	s.adminUsers[user.ID] = current
	s.appendAdminAuditEntryLocked(user.ID, AdminAccountAuditEntry{
		ID:            uuid.NewString(),
		AdminUserID:   input.AdminUserID,
		Action:        "user.admin_permission_changed",
		Reason:        input.Reason,
		BeforeIsAdmin: boolPointer(beforeIsAdmin),
		AfterIsAdmin:  boolPointer(input.Grant),
		RequestID:     input.RequestID,
		CreatedAt:     now,
	})
	detail, appErr := s.adminUserDetailLocked(user.ID)
	return AdminUserMutationResult{Detail: detail}, appErr
}

func (s *Service) completeAdminUserMemoryMutation(ctx context.Context, entry *idempotency.Entry, result AdminUserMutationResult, buildCompletion AdminUserCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	completion, appErr := buildCompletion(result)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	if appErr := s.idempotency.Complete(ctx, entry, completion.Status, completion.ContentType, completion.Body, completion.ResourceType, completion.ResourceID); appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	return completion, nil
}

func decorateAdminUserCompletionBuilder(adminUserID string, buildCompletion AdminUserCompletionBuilder) AdminUserCompletionBuilder {
	return func(result AdminUserMutationResult) (idempotency.Completion, *domain.AppError) {
		result.Detail = decorateAdminUserDetail(result.Detail, adminUserID)
		return buildCompletion(result)
	}
}

func decorateAdminUserDetail(detail AdminUserDetail, adminUserID string) AdminUserDetail {
	detail.ImpactPreview.ActiveSessions = detail.ActiveSessionCount
	isActive := detail.User.Status == AccountStatusActive
	detail.AccountCapabilities = AdminUserAccountCapabilities{
		CanLogin:                        isActive,
		PubliclyVisible:                 isActive,
		CanPublish:                      isActive,
		CanCreateOrders:                 isActive,
		CanRevealContact:                isActive,
		CanAccessHistoricalTransactions: true,
	}
	detail.AvailableActions = adminUserGovernanceActions(detail, strings.TrimSpace(adminUserID))
	return detail
}

func adminUserGovernanceActions(detail AdminUserDetail, adminUserID string) []AdminUserGovernanceAction {
	actions := make([]AdminUserGovernanceAction, 0, 4)
	for _, targetStatus := range []string{AccountStatusActive, AccountStatusSuspended, AccountStatusBanned, AccountStatusArchived} {
		if !AllowedAdminUserStatusTransition(detail.User.Status, targetStatus) {
			continue
		}
		action := AdminUserGovernanceAction{
			Action:               adminUserStatusAction(targetStatus),
			Kind:                 "status",
			TargetStatus:         targetStatus,
			Allowed:              true,
			Severity:             adminUserStatusActionSeverity(targetStatus),
			RequiresReason:       true,
			RequiresConfirmation: true,
		}
		applyAdminUserActionBlock(&action, detail, adminUserID)
		actions = append(actions, action)
	}

	targetIsAdmin := !detail.User.IsAdmin
	permissionAction := AdminUserGovernanceAction{
		Action:               AdminUserActionGrantAdmin,
		Kind:                 "permission",
		TargetIsAdmin:        boolPointer(targetIsAdmin),
		Allowed:              true,
		Severity:             "normal",
		RequiresReason:       true,
		RequiresConfirmation: true,
	}
	if detail.User.IsAdmin {
		permissionAction.Action = AdminUserActionRevokeAdmin
		permissionAction.Severity = "danger"
	}
	applyAdminUserActionBlock(&permissionAction, detail, adminUserID)
	actions = append(actions, permissionAction)
	return actions
}

func applyAdminUserActionBlock(action *AdminUserGovernanceAction, detail AdminUserDetail, adminUserID string) {
	if detail.User.ID == adminUserID {
		blockAdminUserAction(action, "SELF_TARGET", "不能修改自己的账号状态或管理员权限。")
		return
	}
	deactivatesLastAdmin := action.Kind == "status" && detail.User.Status == AccountStatusActive && action.TargetStatus != AccountStatusActive
	revokesLastAdmin := action.Action == AdminUserActionRevokeAdmin && detail.User.Status == AccountStatusActive
	if detail.User.IsAdmin && detail.ActiveAdminCount <= 1 && (deactivatesLastAdmin || revokesLastAdmin) {
		blockAdminUserAction(action, "LAST_ACTIVE_ADMIN", "不能停用最后一个有效管理员或撤销其权限。")
		return
	}
	if action.Action == AdminUserActionGrantAdmin && detail.User.Status != AccountStatusActive {
		blockAdminUserAction(action, "ACCOUNT_NOT_ACTIVE", "只能向正常状态的账号授予管理员权限。")
	}
}

func blockAdminUserAction(action *AdminUserGovernanceAction, code, reason string) {
	action.Allowed = false
	action.BlockedCode = code
	action.BlockedReason = reason
}

func adminUserStatusAction(status string) string {
	switch status {
	case AccountStatusSuspended:
		return AdminUserActionSuspend
	case AccountStatusBanned:
		return AdminUserActionBan
	case AccountStatusArchived:
		return AdminUserActionArchive
	default:
		return AdminUserActionRestore
	}
}

func adminUserStatusActionSeverity(status string) string {
	switch status {
	case AccountStatusSuspended:
		return "warning"
	case AccountStatusBanned, AccountStatusArchived:
		return "danger"
	default:
		return "normal"
	}
}

func (s *Service) activeAdminCountLocked() int {
	count := 0
	for _, user := range s.users {
		if user.IsAdmin && user.Status == AccountStatusActive {
			count++
		}
	}
	return count
}

func (s *Service) appendAdminAuditEntryLocked(userID string, entry AdminAccountAuditEntry) {
	if admin := s.users[entry.AdminUserID]; admin.ID != "" {
		entry.AdminUsername = admin.Username
	}
	entries := append([]AdminAccountAuditEntry{entry}, s.adminAuditEntries[userID]...)
	if len(entries) > 20 {
		entries = entries[:20]
	}
	s.adminAuditEntries[userID] = entries
}

func AllowedAdminUserStatusTransition(current, next string) bool {
	switch current {
	case AccountStatusActive:
		return stringIn(next, AccountStatusSuspended, AccountStatusBanned, AccountStatusArchived)
	case AccountStatusSuspended:
		return stringIn(next, AccountStatusActive, AccountStatusBanned, AccountStatusArchived)
	case AccountStatusBanned:
		return stringIn(next, AccountStatusActive, AccountStatusArchived)
	case AccountStatusArchived:
		return next == AccountStatusActive
	default:
		return false
	}
}

func stringIn(value string, values ...string) bool {
	for _, candidate := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func boolPointer(value bool) *bool {
	return &value
}

func adminPermissionRequired() *domain.AppError {
	return domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
}

func adminUserValidationError(field, detail string) *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "User management validation failed", detail, field, "invalid", detail)
}

func adminUserNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "User not found", "用户不存在。")
}

func adminUserVersionConflict() *domain.AppError {
	return domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "账号信息已更新，请刷新后重试。")
}

func adminUserInvalidTransition(detail string) *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", detail)
}

func adminUserInternalError() *domain.AppError {
	return domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "账号治理响应编码失败。")
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
	if len([]rune(password)) > 32 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Password too long", "备用密码最多 32 个字符。", "newPassword", "too_long", "备用密码最多 32 个字符。")
	}
	hasLetter, hasDigit, hasSymbol := passwordComposition(password)
	if !hasLetter || !hasDigit || !hasSymbol {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Password too weak", "备用密码需同时包含字母、数字和特殊字符。", "newPassword", "composition_required", "需同时包含字母、数字和特殊字符。")
	}
	return nil
}

func passwordComposition(password string) (bool, bool, bool) {
	var hasLetter bool
	var hasDigit bool
	var hasSymbol bool
	for _, r := range password {
		isLetter := (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
		isDigit := r >= '0' && r <= '9'
		switch {
		case isLetter:
			hasLetter = true
		case isDigit:
			hasDigit = true
		case !unicode.IsSpace(r):
			hasSymbol = true
		}
	}
	return hasLetter, hasDigit, hasSymbol
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

func eligibleAccountAppealStatus(status string) bool {
	return status == AccountStatusSuspended || status == AccountStatusBanned
}

func accountAppealIneligibleError() *domain.AppError {
	return domain.NewError(http.StatusForbidden, domain.CodeAccountAppealIneligible, "Account appeal unavailable", "当前身份无法使用账号申诉验证。")
}

func accountAppealSessionExpiredError() *domain.AppError {
	return domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Account appeal session expired", "账号申诉验证已过期，请重新验证。")
}

func accountAppealSessionRevokedError() *domain.AppError {
	return domain.NewError(http.StatusUnauthorized, domain.CodeSessionRevoked, "Account appeal session revoked", "账号申诉验证已失效，请重新验证。")
}

func accountAppealCSRFError() *domain.AppError {
	return domain.NewError(http.StatusForbidden, domain.CodeCSRFTokenInvalid, "Account appeal CSRF token invalid", "账号申诉 CSRF token 无效或缺失。")
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
