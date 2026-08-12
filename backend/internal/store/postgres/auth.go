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
		RETURNING id::text, analytics_user_id::text, username, display_name, account_status
	`, username, now).Scan(&user.ID, &user.AnalyticsUserID, &user.Username, &user.DisplayName, &user.Status)
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
	if err := insertRegistrationAttribution(ctx, tx, user.ID, "unknown", auth.RegistrationAttribution{
		SourceType:  auth.RegistrationSourceUnknown,
		Source:      auth.RegistrationSourceUnknown,
		LandingPath: "/",
	}, now); err != nil {
		return auth.User{}, internalStoreError()
	}
	devSubject := "dev-session:" + user.ID
	if _, err = tx.Exec(ctx, `
		INSERT INTO auth_identities (user_id, provider, provider_subject, created_at, last_login_at)
		VALUES ($1, 'linux_do', $2, $3, $3)
		ON CONFLICT (provider, provider_subject) DO UPDATE
		SET last_login_at = EXCLUDED.last_login_at
	`, user.ID, devSubject, now); err != nil {
		return auth.User{}, internalStoreError()
	}
	if _, err = tx.Exec(ctx, `
		INSERT INTO linux_do_bindings (
		  user_id, linux_do_user_id, linux_do_username, trust_level,
		  bound_at, last_synced_at
		)
		VALUES ($1, $2, $3, 1, $4, $4)
		ON CONFLICT (user_id) DO NOTHING
	`, user.ID, devSubject, username, now); err != nil {
		return auth.User{}, internalStoreError()
	}
	var binding authLinuxDoBindingScan
	if err = tx.QueryRow(ctx, `
		SELECT linux_do_user_id, linux_do_username, trust_level, avatar_url, bound_at, last_synced_at
		FROM linux_do_bindings
		WHERE user_id = $1
	`, user.ID).Scan(&binding.userID, &binding.username, &binding.trustLevel, &binding.avatarURL, &binding.boundAt, &binding.lastSyncedAt); err != nil {
		return auth.User{}, internalStoreError()
	}
	applyAuthLinuxDoBinding(&user, binding)

	if err := tx.Commit(ctx); err != nil {
		return auth.User{}, internalStoreError()
	}
	return auth.HydrateCapabilities(user), nil
}

func (s *Store) SetDevAdminPermission(ctx context.Context, userID string, isAdmin bool, now time.Time) *domain.AppError {
	if s == nil || s.pool == nil {
		return internalStoreError()
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Development persona not found", "开发身份不存在。")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return internalStoreError()
	}
	defer rollback(ctx, tx)

	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)`, userID).Scan(&exists); err != nil {
		return internalStoreError()
	}
	if !exists {
		return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Development persona not found", "开发身份不存在。")
	}
	if isAdmin {
		_, err = tx.Exec(ctx, `
			INSERT INTO user_permissions (user_id, permission)
			VALUES ($1, 'admin')
			ON CONFLICT DO NOTHING
		`, userID)
	} else {
		_, err = tx.Exec(ctx, `
			DELETE FROM user_permissions
			WHERE user_id = $1 AND permission = 'admin'
		`, userID)
	}
	if err != nil {
		return internalStoreError()
	}
	if _, err = tx.Exec(ctx, `UPDATE users SET updated_at = $2 WHERE id = $1`, userID, now); err != nil {
		return internalStoreError()
	}
	if err = tx.Commit(ctx); err != nil {
		return internalStoreError()
	}
	return nil
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
		SELECT u.id::text, u.analytics_user_id::text, u.username, u.display_name, u.account_status,
		       EXISTS(SELECT 1 FROM user_permissions p WHERE p.user_id = u.id AND p.permission = 'admin') AS is_admin,
		       l.linux_do_user_id, l.linux_do_username, l.trust_level, l.avatar_url, l.bound_at, l.last_synced_at
		FROM users u
		LEFT JOIN linux_do_bindings l ON l.user_id = u.id
		WHERE u.id = $1
	`, userID).Scan(
		&user.ID,
		&user.AnalyticsUserID,
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
	if err := hydrateAuthStudentClaim(ctx, s.pool, &user); err != nil {
		return auth.User{}, internalStoreError()
	}
	return user, nil
}

func (s *Store) UpsertOAuthUser(ctx context.Context, profile auth.OAuthProfile, now time.Time) (auth.OAuthUserResult, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.OAuthUserResult{}, internalStoreError()
	}
	provider := auth.CanonicalOAuthProvider(profile.Provider)
	subject := auth.CanonicalOAuthSubject(profile.Subject)
	username := auth.OAuthUsernameCandidate(profile.Username, provider, subject, 0)
	if strings.TrimSpace(profile.Username) == "" || provider == "" || subject == "" {
		return auth.OAuthUserResult{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid OAuth profile", "OAuth 用户资料不完整。", "profile", "required", "OAuth 用户资料不完整。")
	}
	displayName := strings.TrimSpace(profile.DisplayName)
	if displayName == "" {
		displayName = username
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return auth.OAuthUserResult{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	user, identityExists, err := oauthUserByIdentity(ctx, tx, provider, subject, true)
	if err != nil {
		return auth.OAuthUserResult{}, internalStoreError()
	}
	if identityExists {
		err = tx.QueryRow(ctx, `
			UPDATE users
			SET display_name = $2,
			    avatar_url = COALESCE(NULLIF($3, ''), avatar_url),
			    last_active_at = $4,
			    updated_at = $4
			WHERE id = $1
			RETURNING display_name, account_status
		`, user.ID, displayName, strings.TrimSpace(profile.AvatarURL), now).Scan(&user.DisplayName, &user.Status)
		if err != nil {
			return auth.OAuthUserResult{}, internalStoreError()
		}
		if _, err = tx.Exec(ctx, `
			UPDATE auth_identities
			SET last_login_at = $3
			WHERE provider = $1 AND provider_subject = $2
		`, provider, subject, now); err != nil {
			return auth.OAuthUserResult{}, internalStoreError()
		}
		if auth.IsLinuxDoProvider(provider) {
			user.LinuxDoBinding, err = syncLinuxDoBinding(ctx, tx, user.ID, profile, now)
			if err != nil {
				return auth.OAuthUserResult{}, internalStoreError()
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return auth.OAuthUserResult{}, internalStoreError()
		}
		return auth.OAuthUserResult{User: auth.HydrateCapabilities(user)}, nil
	}

	var createdUser auth.User
	for attempt := 0; ; attempt++ {
		candidate := auth.OAuthUsernameCandidate(username, provider, subject, attempt)
		err = tx.QueryRow(ctx, `
			INSERT INTO users (username, display_name, avatar_url, account_status, created_at, updated_at, last_active_at)
			VALUES ($1, $2, NULLIF($3, ''), 'active', $4, $4, $4)
			ON CONFLICT (username) DO NOTHING
			RETURNING id::text, analytics_user_id::text, username, display_name, account_status
		`, candidate, displayName, strings.TrimSpace(profile.AvatarURL), now).Scan(
			&createdUser.ID,
			&createdUser.AnalyticsUserID,
			&createdUser.Username,
			&createdUser.DisplayName,
			&createdUser.Status,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return auth.OAuthUserResult{}, internalStoreError()
		}
		break
	}

	var identityID string
	err = tx.QueryRow(ctx, `
		INSERT INTO auth_identities (user_id, provider, provider_subject, created_at, last_login_at)
		VALUES ($1, $2, $3, $4, $4)
		ON CONFLICT (provider, provider_subject) DO NOTHING
		RETURNING id::text
	`, createdUser.ID, provider, subject, now).Scan(&identityID)
	if errors.Is(err, pgx.ErrNoRows) {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) {
			return auth.OAuthUserResult{}, internalStoreError()
		}
		winner, found, reloadErr := oauthUserByIdentity(ctx, s.pool, provider, subject, false)
		if reloadErr != nil || !found {
			return auth.OAuthUserResult{}, internalStoreError()
		}
		return auth.OAuthUserResult{User: auth.HydrateCapabilities(winner)}, nil
	}
	if err != nil {
		return auth.OAuthUserResult{}, internalStoreError()
	}

	if auth.IsLinuxDoProvider(provider) {
		createdUser.LinuxDoBinding, err = syncLinuxDoBinding(ctx, tx, createdUser.ID, profile, now)
		if err != nil {
			return auth.OAuthUserResult{}, internalStoreError()
		}
	}
	if err := insertRegistrationAttribution(ctx, tx, createdUser.ID, "oauth_linux_do", profile.Attribution, now); err != nil {
		return auth.OAuthUserResult{}, internalStoreError()
	}
	if appErr := bindReferralRegistrationInTx(ctx, tx, createdUser.ID, profile.ReferralCode, now); appErr != nil {
		return auth.OAuthUserResult{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return auth.OAuthUserResult{}, internalStoreError()
	}
	return auth.OAuthUserResult{User: auth.HydrateCapabilities(createdUser), Created: true}, nil
}

func (s *Store) BootstrapAdminPassword(ctx context.Context, credential auth.PasswordCredential, now time.Time) (auth.BootstrapAdminResult, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.BootstrapAdminResult{}, internalStoreError()
	}
	username := strings.TrimSpace(strings.ToLower(credential.User.Username))
	if username == "" {
		return auth.BootstrapAdminResult{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid username", "管理员用户名格式不正确。", "username", "invalid", "管理员用户名格式不正确。")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return auth.BootstrapAdminResult{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	if _, err = tx.Exec(ctx, `
		LOCK TABLE admin_bootstrap_runs, users, user_permissions, user_password_credentials
		IN SHARE ROW EXCLUSIVE MODE
	`); err != nil {
		return auth.BootstrapAdminResult{}, internalStoreError()
	}

	var existing auth.User
	var usernameSnapshot string
	var hasAdmin, hasCredential bool
	err = tx.QueryRow(ctx, `
		SELECT u.id::text,
		       u.username,
		       u.display_name,
		       u.account_status,
		       r.username_snapshot,
		       EXISTS(
		         SELECT 1 FROM user_permissions p
		         WHERE p.user_id = u.id AND p.permission = 'admin'
		       ),
		       EXISTS(
		         SELECT 1 FROM user_password_credentials c
		         WHERE c.user_id = u.id
		       )
		FROM admin_bootstrap_runs r
		JOIN users u ON u.id = r.user_id
		WHERE r.bootstrap_key = $1
	`, auth.InitialAdminBootstrapKey).Scan(
		&existing.ID,
		&existing.Username,
		&existing.DisplayName,
		&existing.Status,
		&usernameSnapshot,
		&hasAdmin,
		&hasCredential,
	)
	if err == nil {
		if usernameSnapshot != existing.Username ||
			existing.Status != "active" ||
			!hasAdmin ||
			!hasCredential {
			return auth.BootstrapAdminResult{}, auth.AdminBootstrapInconsistentError()
		}
		if username != usernameSnapshot {
			return auth.BootstrapAdminResult{}, auth.AdminBootstrapConflictError()
		}
		existing.IsAdmin = true
		return auth.BootstrapAdminResult{User: auth.HydrateCapabilities(existing)}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return auth.BootstrapAdminResult{}, internalStoreError()
	}

	var adminExists, usernameExists bool
	if err = tx.QueryRow(ctx, `
		SELECT EXISTS(
		         SELECT 1 FROM user_permissions WHERE permission = 'admin'
		       ),
		       EXISTS(
		         SELECT 1 FROM users WHERE username = $1
		       )
	`, username).Scan(&adminExists, &usernameExists); err != nil {
		return auth.BootstrapAdminResult{}, internalStoreError()
	}
	if adminExists || usernameExists {
		return auth.BootstrapAdminResult{}, auth.AdminBootstrapConflictError()
	}

	displayName := strings.TrimSpace(credential.User.DisplayName)
	if displayName == "" {
		displayName = username
	}
	var user auth.User
	err = tx.QueryRow(ctx, `
		INSERT INTO users (username, display_name, account_status, created_at, updated_at)
		VALUES ($1, $2, 'active', $3, $3)
		RETURNING id::text, analytics_user_id::text, username, display_name, account_status
	`, username, displayName, now).Scan(&user.ID, &user.AnalyticsUserID, &user.Username, &user.DisplayName, &user.Status)
	if err != nil {
		if isUniqueViolation(err) {
			return auth.BootstrapAdminResult{}, auth.AdminBootstrapConflictError()
		}
		return auth.BootstrapAdminResult{}, internalStoreError()
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO user_permissions (user_id, permission)
		VALUES ($1, 'admin')
	`, user.ID)
	if err != nil {
		return auth.BootstrapAdminResult{}, internalStoreError()
	}
	if err := insertRegistrationAttribution(ctx, tx, user.ID, "unknown", auth.RegistrationAttribution{
		SourceType:  auth.RegistrationSourceUnknown,
		Source:      auth.RegistrationSourceUnknown,
		LandingPath: "/",
	}, now); err != nil {
		return auth.BootstrapAdminResult{}, internalStoreError()
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO user_password_credentials (user_id, password_algorithm, password_salt, password_hash, created_at, password_updated_at)
		VALUES ($1, $2, $3, $4, $5, $5)
	`, user.ID, strings.TrimSpace(credential.Algorithm), strings.TrimSpace(credential.Salt), strings.TrimSpace(credential.Hash), now)
	if err != nil {
		return auth.BootstrapAdminResult{}, internalStoreError()
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO admin_bootstrap_runs (bootstrap_key, user_id, username_snapshot, created_at)
		VALUES ($1, $2, $3, $4)
	`, auth.InitialAdminBootstrapKey, user.ID, user.Username, now)
	if err != nil {
		if isUniqueViolation(err) {
			return auth.BootstrapAdminResult{}, auth.AdminBootstrapConflictError()
		}
		return auth.BootstrapAdminResult{}, internalStoreError()
	}

	if err := tx.Commit(ctx); err != nil {
		return auth.BootstrapAdminResult{}, internalStoreError()
	}
	user.IsAdmin = true
	return auth.BootstrapAdminResult{User: auth.HydrateCapabilities(user), Created: true}, nil
}

func oauthUserByIdentity(ctx context.Context, q queryer, provider, subject string, lock bool) (auth.User, bool, error) {
	lockClause := ""
	if lock {
		lockClause = " FOR UPDATE OF i, u"
	}
	var user auth.User
	var binding authLinuxDoBindingScan
	err := q.QueryRow(ctx, `
		SELECT u.id::text,
		       u.analytics_user_id::text,
		       u.username,
		       u.display_name,
		       u.account_status,
		       EXISTS(
		         SELECT 1 FROM user_permissions p
		         WHERE p.user_id = u.id AND p.permission = 'admin'
		       ) AS is_admin,
		       l.linux_do_user_id,
		       l.linux_do_username,
		       l.trust_level,
		       l.avatar_url,
		       l.bound_at,
		       l.last_synced_at
		FROM auth_identities i
		JOIN users u ON u.id = i.user_id
		LEFT JOIN linux_do_bindings l ON l.user_id = u.id
		WHERE i.provider = $1 AND i.provider_subject = $2
	`+lockClause, provider, subject).Scan(
		&user.ID,
		&user.AnalyticsUserID,
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
		return auth.User{}, false, nil
	}
	if err != nil {
		return auth.User{}, false, err
	}
	applyAuthLinuxDoBinding(&user, binding)
	if err := hydrateAuthStudentClaim(ctx, q, &user); err != nil {
		return auth.User{}, false, err
	}
	return user, true, nil
}

func syncLinuxDoBinding(ctx context.Context, q queryer, userID string, profile auth.OAuthProfile, now time.Time) (*auth.LinuxDoBinding, error) {
	linuxDoUserID := strings.TrimSpace(profile.LinuxDoUserID)
	if linuxDoUserID == "" {
		linuxDoUserID = auth.CanonicalOAuthSubject(profile.Subject)
	}
	linuxDoUsername := strings.TrimSpace(profile.LinuxDoUsername)
	if linuxDoUsername == "" {
		linuxDoUsername = auth.OAuthUsernameCandidate(profile.Username, profile.Provider, profile.Subject, 0)
	}
	trustLevel := profile.TrustLevel
	if trustLevel <= 0 {
		trustLevel = 1
	}
	avatarURL := strings.TrimSpace(profile.LinuxDoAvatarURL)
	if avatarURL == "" {
		avatarURL = strings.TrimSpace(profile.AvatarURL)
	}

	var binding auth.LinuxDoBinding
	binding.Bound = true
	err := q.QueryRow(ctx, `
		INSERT INTO linux_do_bindings (
		  user_id,
		  linux_do_user_id,
		  linux_do_username,
		  trust_level,
		  avatar_url,
		  bound_at,
		  last_synced_at
		)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, $6)
		ON CONFLICT (user_id) DO UPDATE
		SET linux_do_user_id = EXCLUDED.linux_do_user_id,
		    linux_do_username = EXCLUDED.linux_do_username,
		    trust_level = EXCLUDED.trust_level,
		    avatar_url = COALESCE(EXCLUDED.avatar_url, linux_do_bindings.avatar_url),
		    last_synced_at = EXCLUDED.last_synced_at
		RETURNING linux_do_user_id,
		          linux_do_username,
		          trust_level,
		          COALESCE(avatar_url, ''),
		          bound_at,
		          last_synced_at
	`, userID, linuxDoUserID, linuxDoUsername, trustLevel, avatarURL, now).Scan(
		&binding.LinuxDoUserID,
		&binding.LinuxDoUsername,
		&binding.TrustLevel,
		&binding.AvatarURL,
		&binding.BoundAt,
		&binding.LastSyncedAt,
	)
	if err != nil {
		return nil, err
	}
	return &binding, nil
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
		SELECT u.id::text, u.analytics_user_id::text, u.username, u.display_name, u.account_status,
		       EXISTS(SELECT 1 FROM user_permissions p WHERE p.user_id = u.id AND p.permission = 'admin') AS is_admin,
		       c.password_algorithm, c.password_salt, c.password_hash,
		       l.linux_do_user_id, l.linux_do_username, l.trust_level, l.avatar_url, l.bound_at, l.last_synced_at
		FROM users u
		JOIN user_password_credentials c ON c.user_id = u.id
		LEFT JOIN linux_do_bindings l ON l.user_id = u.id
		WHERE u.username = $1
		   OR EXISTS (
		     SELECT 1
		     FROM student_email_claims claim
		     WHERE claim.user_id = u.id AND claim.normalized_email = $1
		   )
	`, username).Scan(
		&credential.User.ID,
		&credential.User.AnalyticsUserID,
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
	if err := hydrateAuthStudentClaim(ctx, s.pool, &credential.User); err != nil {
		return auth.PasswordCredential{}, internalStoreError()
	}
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
		SELECT u.id::text, u.analytics_user_id::text, u.username, u.display_name, u.account_status,
		       EXISTS(SELECT 1 FROM user_permissions p WHERE p.user_id = u.id AND p.permission = 'admin') AS is_admin,
		       c.password_algorithm, c.password_salt, c.password_hash,
		       l.linux_do_user_id, l.linux_do_username, l.trust_level, l.avatar_url, l.bound_at, l.last_synced_at
		FROM users u
		JOIN user_password_credentials c ON c.user_id = u.id
		LEFT JOIN linux_do_bindings l ON l.user_id = u.id
		WHERE u.id = $1
	`, userID).Scan(
		&credential.User.ID,
		&credential.User.AnalyticsUserID,
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
	if err := hydrateAuthStudentClaim(ctx, s.pool, &credential.User); err != nil {
		return auth.PasswordCredential{}, internalStoreError()
	}
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

func (s *Store) CreateSession(ctx context.Context, userID, sessionTokenHash, csrfTokenHash string, expiresAt, absoluteExpiresAt, now time.Time) *domain.AppError {
	if s == nil || s.pool == nil {
		return internalStoreError()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO auth_sessions (
			user_id, session_token_hash, csrf_token_hash, expires_at,
			renewed_at, absolute_expires_at, created_at, updated_at, last_seen_at
		)
		VALUES ($1, $2, $3, $4, $6, $5, $6, $6, $6)
	`, userID, sessionTokenHash, csrfTokenHash, expiresAt, absoluteExpiresAt, now)
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

func (s *Store) RenewSession(ctx context.Context, sessionTokenHash string, now, targetExpiresAt, renewBefore time.Time) (time.Time, bool, *domain.AppError) {
	if s == nil || s.pool == nil {
		return time.Time{}, false, internalStoreError()
	}
	var expiresAt time.Time
	err := s.pool.QueryRow(ctx, `
		UPDATE auth_sessions
		SET renewed_at = $2,
		    expires_at = LEAST($3, absolute_expires_at),
		    updated_at = $2,
		    last_seen_at = $2
		WHERE session_token_hash = $1
		  AND revoked_at IS NULL
		  AND expires_at > $2
		  AND absolute_expires_at > $2
		  AND renewed_at <= $4
		RETURNING expires_at
	`, sessionTokenHash, now, targetExpiresAt, renewBefore).Scan(&expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, internalStoreError()
	}
	return expiresAt, true, nil
}

func (s *Store) RefreshSessionCSRF(ctx context.Context, sessionTokenHash, csrfTokenHash string, now time.Time) *domain.AppError {
	if s == nil || s.pool == nil {
		return internalStoreError()
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE auth_sessions
		SET csrf_token_hash = $2, last_seen_at = $3, updated_at = $3
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
		SET revoked_at = $2, updated_at = $2
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
		SELECT u.id::text, u.analytics_user_id::text, u.username, u.display_name, u.account_status,
		       EXISTS(SELECT 1 FROM user_permissions p WHERE p.user_id = u.id AND p.permission = 'admin') AS is_admin,
		       s.session_token_hash, s.user_id::text, s.expires_at, s.renewed_at, s.absolute_expires_at, s.revoked_at,
		       s.password_reauthenticated_at, COALESCE(s.oauth_link_state_hash, ''),
		       COALESCE(s.oauth_link_state_purpose, ''), s.oauth_link_state_expires_at,
		       s.oauth_link_state_consumed_at,
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
		&user.AnalyticsUserID,
		&user.Username,
		&user.DisplayName,
		&user.Status,
		&user.IsAdmin,
		&session.ID,
		&session.UserID,
		&session.ExpiresAt,
		&session.RenewedAt,
		&session.AbsoluteExpiresAt,
		&session.RevokedAt,
		&session.PasswordReauthenticatedAt,
		&session.OAuthLinkStateHash,
		&session.OAuthLinkStatePurpose,
		&session.OAuthLinkStateExpiresAt,
		&session.OAuthLinkStateConsumedAt,
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
	if !now.Before(session.ExpiresAt) || !now.Before(session.AbsoluteExpiresAt) {
		return auth.User{}, auth.Session{}, domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session expired", "当前会话已过期。")
	}
	if user.Status != "active" {
		return auth.User{}, auth.Session{}, domain.NewError(http.StatusForbidden, domain.CodeAccountRestricted, "Account restricted", "当前账号不可执行该操作。")
	}
	applyAuthLinuxDoBinding(&user, binding)
	if err := hydrateAuthStudentClaim(ctx, s.pool, &user); err != nil {
		return auth.User{}, auth.Session{}, internalStoreError()
	}
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
