package postgres

import (
	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"net/http"
	"strings"
	"time"
)

type authLinuxDoBindingScan struct {
	userID       *string
	username     *string
	trustLevel   *int
	avatarURL    *string
	boundAt      *time.Time
	lastSyncedAt *time.Time
}

func (s *Store) EnsureUser(ctx context.Context, username string, isAdmin bool, now time.Time) (auth.User, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.User{}, internalStoreError()
	}
	username = strings.TrimSpace(strings.ToLower(username))
	if username == "" {
		username = "buyer"
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return auth.User{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	var user auth.User
	err = tx.QueryRow(ctx, `
		INSERT INTO users (username, display_name, account_status, created_at, updated_at)
		VALUES ($1, $1, 'active', $2, $2)
		ON CONFLICT (username) DO UPDATE
		SET display_name = users.display_name
		RETURNING id::text, username, display_name, account_status
	`, username, now).Scan(&user.ID, &user.Username, &user.DisplayName, &user.Status)
	if err != nil {
		return auth.User{}, internalStoreError()
	}

	if isAdmin {
		_, err = tx.Exec(ctx, `
			INSERT INTO user_permissions (user_id, permission)
			VALUES ($1, 'admin')
			ON CONFLICT DO NOTHING
		`, user.ID)
		if err != nil {
			return auth.User{}, internalStoreError()
		}
		user.IsAdmin = true
	} else {
		user.IsAdmin, err = hasAdminPermission(ctx, tx, user.ID)
		if err != nil {
			return auth.User{}, internalStoreError()
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return auth.User{}, internalStoreError()
	}
	return user, nil
}

func (s *Store) UserByID(ctx context.Context, userID string) (auth.User, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.User{}, internalStoreError()
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return auth.User{}, domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。")
	}

	var user auth.User
	var binding authLinuxDoBindingScan
	err := s.pool.QueryRow(ctx, `
		SELECT u.id::text, u.username, u.display_name, u.account_status,
		       EXISTS(SELECT 1 FROM user_permissions p WHERE p.user_id = u.id AND p.permission = 'admin') AS is_admin,
		       l.linux_do_user_id, l.linux_do_username, l.trust_level, l.avatar_url, l.bound_at, l.last_synced_at
		FROM users u
		LEFT JOIN linux_do_bindings l ON l.user_id = u.id
		WHERE u.id = $1
	`, userID).Scan(
		&user.ID,
		&user.Username,
		&user.DisplayName,
		&user.Status,
		&user.IsAdmin,
		&binding.userID,
		&binding.username,
		&binding.trustLevel,
		&binding.avatarURL,
		&binding.boundAt,
		&binding.lastSyncedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。")
	}
	if err != nil {
		return auth.User{}, internalStoreError()
	}
	applyAuthLinuxDoBinding(&user, binding)
	return user, nil
}

func (s *Store) UpsertOAuthUser(ctx context.Context, profile auth.OAuthProfile, now time.Time) (auth.OAuthUserResult, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.OAuthUserResult{}, internalStoreError()
	}
	username := strings.TrimSpace(strings.ToLower(profile.Username))
	if username == "" || strings.TrimSpace(profile.Provider) == "" || strings.TrimSpace(profile.Subject) == "" {
		return auth.OAuthUserResult{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid OAuth profile", "OAuth 用户资料不完整。", "profile", "required", "OAuth 用户资料不完整。")
	}
	displayName := strings.TrimSpace(profile.DisplayName)
	if displayName == "" {
		displayName = username
	}
	linuxDoUserID := strings.TrimSpace(profile.LinuxDoUserID)
	if linuxDoUserID == "" {
		linuxDoUserID = strings.TrimSpace(profile.Subject)
	}
	linuxDoUsername := strings.TrimSpace(profile.LinuxDoUsername)
	if linuxDoUsername == "" {
		linuxDoUsername = username
	}
	trustLevel := profile.TrustLevel
	if trustLevel <= 0 {
		trustLevel = 1
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return auth.OAuthUserResult{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	var user auth.User
	var inserted bool
	err = tx.QueryRow(ctx, `
		INSERT INTO users (username, display_name, avatar_url, account_status, created_at, updated_at, last_active_at)
		VALUES ($1, $2, NULLIF($3, ''), 'active', $4, $4, $4)
		ON CONFLICT (username) DO UPDATE
		SET display_name = EXCLUDED.display_name,
		    avatar_url = COALESCE(EXCLUDED.avatar_url, users.avatar_url),
		    last_active_at = EXCLUDED.last_active_at,
		    updated_at = EXCLUDED.updated_at
		RETURNING id::text, username, display_name, account_status, (xmax = 0) AS inserted
	`, username, displayName, strings.TrimSpace(profile.AvatarURL), now).Scan(&user.ID, &user.Username, &user.DisplayName, &user.Status, &inserted)
	if err != nil {
		return auth.OAuthUserResult{}, internalStoreError()
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO auth_identities (user_id, provider, provider_subject, created_at, last_login_at)
		VALUES ($1, $2, $3, $4, $4)
		ON CONFLICT (provider, provider_subject) DO UPDATE
		SET user_id = EXCLUDED.user_id,
		    last_login_at = EXCLUDED.last_login_at
	`, user.ID, strings.TrimSpace(profile.Provider), strings.TrimSpace(profile.Subject), now)
	if err != nil {
		return auth.OAuthUserResult{}, internalStoreError()
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO linux_do_bindings (user_id, linux_do_user_id, linux_do_username, trust_level, avatar_url, bound_at, last_synced_at)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $6)
		ON CONFLICT (user_id) DO UPDATE
		SET linux_do_user_id = EXCLUDED.linux_do_user_id,
		    linux_do_username = EXCLUDED.linux_do_username,
		    trust_level = EXCLUDED.trust_level,
		    avatar_url = COALESCE(EXCLUDED.avatar_url, linux_do_bindings.avatar_url),
		    last_synced_at = EXCLUDED.last_synced_at
	`, user.ID, linuxDoUserID, linuxDoUsername, trustLevel, strings.TrimSpace(profile.LinuxDoAvatarURL), now)
	if err != nil {
		return auth.OAuthUserResult{}, internalStoreError()
	}

	if profile.GrantAdmin {
		_, err = tx.Exec(ctx, `
			INSERT INTO user_permissions (user_id, permission)
			VALUES ($1, 'admin')
			ON CONFLICT DO NOTHING
		`, user.ID)
		if err != nil {
			return auth.OAuthUserResult{}, internalStoreError()
		}
	}
	user.IsAdmin, err = hasAdminPermission(ctx, tx, user.ID)
	if err != nil {
		return auth.OAuthUserResult{}, internalStoreError()
	}
	user.LinuxDoBinding = &auth.LinuxDoBinding{
		Bound:           true,
		LinuxDoUserID:   linuxDoUserID,
		LinuxDoUsername: linuxDoUsername,
		TrustLevel:      trustLevel,
		AvatarURL:       strings.TrimSpace(profile.LinuxDoAvatarURL),
		BoundAt:         now,
		LastSyncedAt:    now,
	}

	if err := tx.Commit(ctx); err != nil {
		return auth.OAuthUserResult{}, internalStoreError()
	}
	return auth.OAuthUserResult{User: user, Created: inserted}, nil
}

func (s *Store) PasswordCredential(ctx context.Context, username string) (auth.PasswordCredential, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.PasswordCredential{}, internalStoreError()
	}
	username = strings.TrimSpace(strings.ToLower(username))
	if username == "" {
		return auth.PasswordCredential{}, domain.NewError(http.StatusUnauthorized, domain.CodeInvalidCredentials, "Invalid credentials", "用户名或密码不正确。")
	}

	var credential auth.PasswordCredential
	var binding authLinuxDoBindingScan
	err := s.pool.QueryRow(ctx, `
		SELECT u.id::text, u.username, u.display_name, u.account_status,
		       EXISTS(SELECT 1 FROM user_permissions p WHERE p.user_id = u.id AND p.permission = 'admin') AS is_admin,
		       c.password_algorithm, c.password_salt, c.password_hash,
		       l.linux_do_user_id, l.linux_do_username, l.trust_level, l.avatar_url, l.bound_at, l.last_synced_at
		FROM users u
		JOIN user_password_credentials c ON c.user_id = u.id
		LEFT JOIN linux_do_bindings l ON l.user_id = u.id
		WHERE u.username = $1
	`, username).Scan(
		&credential.User.ID,
		&credential.User.Username,
		&credential.User.DisplayName,
		&credential.User.Status,
		&credential.User.IsAdmin,
		&credential.Algorithm,
		&credential.Salt,
		&credential.Hash,
		&binding.userID,
		&binding.username,
		&binding.trustLevel,
		&binding.avatarURL,
		&binding.boundAt,
		&binding.lastSyncedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.PasswordCredential{}, domain.NewError(http.StatusUnauthorized, domain.CodeInvalidCredentials, "Invalid credentials", "用户名或密码不正确。")
	}
	if err != nil {
		return auth.PasswordCredential{}, internalStoreError()
	}
	applyAuthLinuxDoBinding(&credential.User, binding)
	return credential, nil
}

func (s *Store) PasswordCredentialByUserID(ctx context.Context, userID string) (auth.PasswordCredential, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.PasswordCredential{}, internalStoreError()
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return auth.PasswordCredential{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Password credential not found", "尚未设置备用密码。")
	}

	var credential auth.PasswordCredential
	var binding authLinuxDoBindingScan
	err := s.pool.QueryRow(ctx, `
		SELECT u.id::text, u.username, u.display_name, u.account_status,
		       EXISTS(SELECT 1 FROM user_permissions p WHERE p.user_id = u.id AND p.permission = 'admin') AS is_admin,
		       c.password_algorithm, c.password_salt, c.password_hash,
		       l.linux_do_user_id, l.linux_do_username, l.trust_level, l.avatar_url, l.bound_at, l.last_synced_at
		FROM users u
		JOIN user_password_credentials c ON c.user_id = u.id
		LEFT JOIN linux_do_bindings l ON l.user_id = u.id
		WHERE u.id = $1
	`, userID).Scan(
		&credential.User.ID,
		&credential.User.Username,
		&credential.User.DisplayName,
		&credential.User.Status,
		&credential.User.IsAdmin,
		&credential.Algorithm,
		&credential.Salt,
		&credential.Hash,
		&binding.userID,
		&binding.username,
		&binding.trustLevel,
		&binding.avatarURL,
		&binding.boundAt,
		&binding.lastSyncedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.PasswordCredential{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Password credential not found", "尚未设置备用密码。")
	}
	if err != nil {
		return auth.PasswordCredential{}, internalStoreError()
	}
	applyAuthLinuxDoBinding(&credential.User, binding)
	return credential, nil
}

func (s *Store) UpsertPasswordCredential(ctx context.Context, credential auth.PasswordCredential, now time.Time) *domain.AppError {
	if s == nil || s.pool == nil {
		return internalStoreError()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO user_password_credentials (user_id, password_algorithm, password_salt, password_hash, created_at, password_updated_at)
		VALUES ($1, $2, $3, $4, $5, $5)
		ON CONFLICT (user_id) DO UPDATE
		SET password_algorithm = EXCLUDED.password_algorithm,
		    password_salt = EXCLUDED.password_salt,
		    password_hash = EXCLUDED.password_hash,
		    password_updated_at = EXCLUDED.password_updated_at
	`, credential.User.ID, strings.TrimSpace(credential.Algorithm), strings.TrimSpace(credential.Salt), strings.TrimSpace(credential.Hash), now)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) CreateEmailRegistrationCode(ctx context.Context, input auth.EmailRegistrationStartInput, codeHash string, expiresAt, now time.Time) *domain.AppError {
	if s == nil || s.pool == nil {
		return internalStoreError()
	}
	email := strings.TrimSpace(strings.ToLower(input.Email))
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM users
			WHERE lower(email) = lower($1)
			  AND email_verified_at IS NOT NULL
		)
	`, email).Scan(&exists)
	if err != nil {
		return internalStoreError()
	}
	if exists {
		return domain.NewFieldError(http.StatusConflict, domain.CodeValidationFailed, "Email unavailable", "该邮箱已注册。", "email", "unavailable", "该邮箱已注册。")
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO email_verification_codes (user_id, email, purpose, code_hash, expires_at, created_at)
		VALUES (NULL, lower($1), 'email_registration', $2, $3, $4)
	`, email, strings.TrimSpace(codeHash), expiresAt, now)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) ConfirmEmailRegistration(ctx context.Context, input auth.EmailRegistrationConfirmInput, codeHash, sessionTokenHash, csrfTokenHash string, sessionExpiresAt, now time.Time) (auth.User, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.User{}, internalStoreError()
	}
	email := strings.TrimSpace(strings.ToLower(input.Email))
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return auth.User{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	var codeID string
	err = tx.QueryRow(ctx, `
		SELECT id::text
		FROM email_verification_codes
		WHERE user_id IS NULL
		  AND email = lower($1)
		  AND purpose = 'email_registration'
		  AND code_hash = $2
		  AND consumed_at IS NULL
		  AND expires_at > $3
		ORDER BY created_at DESC
		LIMIT 1
		FOR UPDATE
	`, email, strings.TrimSpace(codeHash), now).Scan(&codeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, domain.NewError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Code invalid", "验证码无效或已过期。")
	}
	if err != nil {
		return auth.User{}, internalStoreError()
	}

	var exists bool
	err = tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM users
			WHERE lower(email) = lower($1)
			  AND email_verified_at IS NOT NULL
		)
	`, email).Scan(&exists)
	if err != nil {
		return auth.User{}, internalStoreError()
	}
	if exists {
		return auth.User{}, domain.NewFieldError(http.StatusConflict, domain.CodeValidationFailed, "Email unavailable", "该邮箱已注册。", "email", "unavailable", "该邮箱已注册。")
	}

	username, appErr := firstAvailableUsername(ctx, tx, input.UsernameCandidates)
	if appErr != nil {
		return auth.User{}, appErr
	}
	user := auth.User{Status: "active"}
	err = tx.QueryRow(ctx, `
		INSERT INTO users (username, display_name, email, email_verified_at, account_status, created_at, updated_at, last_active_at)
		VALUES ($1, $1, lower($2), $3, 'active', $3, $3, $3)
		RETURNING id::text, username, display_name, account_status
	`, username, email, now).Scan(&user.ID, &user.Username, &user.DisplayName, &user.Status)
	if isUniqueViolation(err) {
		return auth.User{}, domain.NewError(http.StatusConflict, domain.CodeValidationFailed, "Registration conflict", "注册信息已被占用，请重新获取验证码。")
	}
	if err != nil {
		return auth.User{}, internalStoreError()
	}

	_, err = tx.Exec(ctx, `
		UPDATE email_verification_codes
		SET consumed_at = $2
		WHERE id = $1
	`, codeID, now)
	if err != nil {
		return auth.User{}, internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO auth_sessions (user_id, session_token_hash, csrf_token_hash, expires_at, created_at, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $5)
	`, user.ID, strings.TrimSpace(sessionTokenHash), strings.TrimSpace(csrfTokenHash), sessionExpiresAt, now)
	if err != nil {
		return auth.User{}, internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return auth.User{}, internalStoreError()
	}
	return user, nil
}

func (s *Store) CreateSession(ctx context.Context, userID, sessionTokenHash, csrfTokenHash string, expiresAt, now time.Time) *domain.AppError {
	if s == nil || s.pool == nil {
		return internalStoreError()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO auth_sessions (user_id, session_token_hash, csrf_token_hash, expires_at, created_at, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $5)
	`, userID, sessionTokenHash, csrfTokenHash, expiresAt, now)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) GetSession(ctx context.Context, sessionTokenHash string, now time.Time) (auth.User, auth.Session, *domain.AppError) {
	return s.getSession(ctx, sessionTokenHash, "", false, now)
}

func (s *Store) GetSessionWithCSRF(ctx context.Context, sessionTokenHash, csrfTokenHash string, now time.Time) (auth.User, auth.Session, *domain.AppError) {
	return s.getSession(ctx, sessionTokenHash, csrfTokenHash, true, now)
}

func (s *Store) RefreshSessionCSRF(ctx context.Context, sessionTokenHash, csrfTokenHash string, now time.Time) *domain.AppError {
	if s == nil || s.pool == nil {
		return internalStoreError()
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE auth_sessions
		SET csrf_token_hash = $2, last_seen_at = $3
		WHERE session_token_hash = $1
	`, sessionTokenHash, csrfTokenHash, now)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) RevokeSession(ctx context.Context, sessionTokenHash string, revokedAt time.Time) *domain.AppError {
	if s == nil || s.pool == nil {
		return internalStoreError()
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE auth_sessions
		SET revoked_at = $2
		WHERE session_token_hash = $1
	`, sessionTokenHash, revokedAt)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) getSession(ctx context.Context, sessionTokenHash, csrfTokenHash string, requireCSRF bool, now time.Time) (auth.User, auth.Session, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.User{}, auth.Session{}, internalStoreError()
	}
	query := `
		SELECT u.id::text, u.username, u.display_name, u.account_status,
		       EXISTS(SELECT 1 FROM user_permissions p WHERE p.user_id = u.id AND p.permission = 'admin') AS is_admin,
		       s.session_token_hash, s.user_id::text, s.expires_at, s.revoked_at,
		       l.linux_do_user_id, l.linux_do_username, l.trust_level, l.avatar_url, l.bound_at, l.last_synced_at
		FROM auth_sessions s
		JOIN users u ON u.id = s.user_id
		LEFT JOIN linux_do_bindings l ON l.user_id = u.id
		WHERE s.session_token_hash = $1
	`
	args := []any{sessionTokenHash}
	if requireCSRF {
		query += ` AND s.csrf_token_hash = $2`
		args = append(args, csrfTokenHash)
	}
	var user auth.User
	var session auth.Session
	var binding authLinuxDoBindingScan
	session.ID = sessionTokenHash
	err := s.pool.QueryRow(ctx, query, args...).Scan(
		&user.ID,
		&user.Username,
		&user.DisplayName,
		&user.Status,
		&user.IsAdmin,
		&session.ID,
		&session.UserID,
		&session.ExpiresAt,
		&session.RevokedAt,
		&binding.userID,
		&binding.username,
		&binding.trustLevel,
		&binding.avatarURL,
		&binding.boundAt,
		&binding.lastSyncedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		if requireCSRF {
			return auth.User{}, auth.Session{}, domain.NewError(http.StatusForbidden, domain.CodeCSRFTokenInvalid, "CSRF token invalid", "CSRF token 无效或缺失。")
		}
		return auth.User{}, auth.Session{}, domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。")
	}
	if err != nil {
		return auth.User{}, auth.Session{}, internalStoreError()
	}
	if session.RevokedAt != nil {
		return auth.User{}, auth.Session{}, domain.NewError(http.StatusUnauthorized, domain.CodeSessionRevoked, "Session revoked", "当前会话已退出。")
	}
	if !now.Before(session.ExpiresAt) {
		return auth.User{}, auth.Session{}, domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session expired", "当前会话已过期。")
	}
	if user.Status != "active" {
		return auth.User{}, auth.Session{}, domain.NewError(http.StatusForbidden, domain.CodeAccountRestricted, "Account restricted", "当前账号不可执行该操作。")
	}
	applyAuthLinuxDoBinding(&user, binding)
	_, _ = s.pool.Exec(ctx, `UPDATE auth_sessions SET last_seen_at = $2 WHERE session_token_hash = $1`, sessionTokenHash, now)
	return user, session, nil
}

func applyAuthLinuxDoBinding(user *auth.User, binding authLinuxDoBindingScan) {
	if binding.userID == nil || binding.username == nil || binding.trustLevel == nil || binding.boundAt == nil || binding.lastSyncedAt == nil {
		return
	}
	user.LinuxDoBinding = &auth.LinuxDoBinding{
		Bound:           true,
		LinuxDoUserID:   *binding.userID,
		LinuxDoUsername: *binding.username,
		TrustLevel:      *binding.trustLevel,
		AvatarURL:       stringFromPtr(binding.avatarURL),
		BoundAt:         *binding.boundAt,
		LastSyncedAt:    *binding.lastSyncedAt,
	}
}

func hasAdminPermission(ctx context.Context, q queryer, userID string) (bool, error) {
	var exists bool
	err := q.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM user_permissions WHERE user_id = $1 AND permission = 'admin')
	`, userID).Scan(&exists)
	return exists, err
}

func firstAvailableUsername(ctx context.Context, q queryer, candidates []string) (string, *domain.AppError) {
	for _, candidate := range candidates {
		username := strings.TrimSpace(strings.ToLower(candidate))
		if username == "" {
			continue
		}
		var exists bool
		err := q.QueryRow(ctx, `
			SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)
		`, username).Scan(&exists)
		if err != nil {
			return "", internalStoreError()
		}
		if !exists {
			return username, nil
		}
	}
	return "", domain.NewError(http.StatusConflict, domain.CodeValidationFailed, "Username unavailable", "站内用户名生成失败，请稍后重试。")
}
