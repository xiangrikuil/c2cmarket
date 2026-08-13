package postgres

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"

	"github.com/jackc/pgx/v5"
)

func (s *Store) StartRestrictedBusinessOAuth(ctx context.Context, stateHash string, expiresAt, now time.Time) *domain.AppError {
	return s.startGovernanceOAuthState(ctx, "restricted_business_oauth_states", stateHash, expiresAt, now)
}

func (s *Store) StartAccountAppealOAuth(ctx context.Context, stateHash string, expiresAt, now time.Time) *domain.AppError {
	return s.startGovernanceOAuthState(ctx, "account_appeal_oauth_states", stateHash, expiresAt, now)
}

func (s *Store) startGovernanceOAuthState(ctx context.Context, table, stateHash string, expiresAt, now time.Time) *domain.AppError {
	if s == nil || s.pool == nil {
		return internalStoreError()
	}
	if table != "restricted_business_oauth_states" && table != "account_appeal_oauth_states" {
		return governanceOAuthStateStoreError()
	}
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO `+table+` (state_hash, created_at, expires_at)
		VALUES ($1, $2, $3)
	`, strings.TrimSpace(stateHash), now, expiresAt); err != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) CompleteRestrictedBusinessOAuth(
	ctx context.Context,
	stateHash string,
	profile auth.OAuthProfile,
	sessionTokenHash, csrfTokenHash string,
	sessionExpiresAt, now time.Time,
) (auth.User, auth.RestrictedBusinessSession, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.User{}, auth.RestrictedBusinessSession{}, internalStoreError()
	}
	provider := auth.CanonicalOAuthProvider(profile.Provider)
	subject := auth.CanonicalOAuthSubject(profile.Subject)
	if !auth.IsLinuxDoProvider(provider) || subject == "" {
		return auth.User{}, auth.RestrictedBusinessSession{}, restrictedBusinessSessionUnavailableError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return auth.User{}, auth.RestrictedBusinessSession{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	if appErr := consumeGovernanceOAuthState(ctx, tx, "restricted_business_oauth_states", stateHash, now); appErr != nil {
		return auth.User{}, auth.RestrictedBusinessSession{}, appErr
	}

	var userID string
	err = tx.QueryRow(ctx, `
		SELECT user_id::text
		FROM auth_identities
		WHERE provider = $1 AND provider_subject = $2
	`, provider, subject).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.Commit(ctx); err != nil {
			return auth.User{}, auth.RestrictedBusinessSession{}, internalStoreError()
		}
		return auth.User{}, auth.RestrictedBusinessSession{}, restrictedBusinessSessionUnavailableError()
	}
	if err != nil {
		return auth.User{}, auth.RestrictedBusinessSession{}, internalStoreError()
	}
	if appErr := lockAccountGovernanceUser(ctx, tx, userID); appErr != nil {
		return auth.User{}, auth.RestrictedBusinessSession{}, appErr
	}
	user, found, err := oauthUserByIdentity(ctx, tx, provider, subject, true)
	if err != nil {
		return auth.User{}, auth.RestrictedBusinessSession{}, internalStoreError()
	}
	if !found || !auth.IsRestrictedBusinessAccountStatus(user.Status) || user.SecurityLockedAt != nil ||
		strings.TrimSpace(user.CurrentGovernanceActionID) == "" || user.GovernanceVersion < 1 {
		if err := tx.Commit(ctx); err != nil {
			return auth.User{}, auth.RestrictedBusinessSession{}, internalStoreError()
		}
		return auth.User{}, auth.RestrictedBusinessSession{}, restrictedBusinessSessionUnavailableError()
	}

	var session auth.RestrictedBusinessSession
	if _, err := tx.Exec(ctx, `
		UPDATE restricted_business_sessions
		SET revoked_at = COALESCE(revoked_at, $2),
		    last_seen_at = GREATEST(last_seen_at, $2)
		WHERE user_id = $1 AND revoked_at IS NULL
	`, user.ID, now); err != nil {
		return auth.User{}, auth.RestrictedBusinessSession{}, internalStoreError()
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO restricted_business_sessions (
			user_id, session_token_hash, csrf_token_hash,
			governance_action_id, governance_version,
			restriction_effective_at, created_at, expires_at, last_seen_at
		)
		SELECT $1, $2, $3, action.id, $4, action.effective_at, $5, $6, $5
		FROM account_governance_actions action
		WHERE action.id = $7 AND action.status = 'effective'
		RETURNING id::text, user_id::text, governance_action_id::text,
		          governance_version, restriction_effective_at,
		          created_at, expires_at, revoked_at, last_seen_at
	`, user.ID, sessionTokenHash, csrfTokenHash, user.GovernanceVersion, now, sessionExpiresAt, user.CurrentGovernanceActionID).Scan(
		&session.ID,
		&session.UserID,
		&session.GovernanceActionID,
		&session.GovernanceVersion,
		&session.RestrictionEffectiveAt,
		&session.CreatedAt,
		&session.ExpiresAt,
		&session.RevokedAt,
		&session.LastSeenAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, auth.RestrictedBusinessSession{}, restrictedBusinessSessionUnavailableError()
	}
	if err != nil {
		return auth.User{}, auth.RestrictedBusinessSession{}, internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return auth.User{}, auth.RestrictedBusinessSession{}, internalStoreError()
	}
	return auth.HydrateCapabilities(user), session, nil
}

func consumeGovernanceOAuthState(ctx context.Context, tx pgx.Tx, table, stateHash string, now time.Time) *domain.AppError {
	if table != "restricted_business_oauth_states" && table != "account_appeal_oauth_states" {
		return governanceOAuthStateStoreError()
	}
	result, err := tx.Exec(ctx, `
		UPDATE `+table+`
		SET consumed_at = $2
		WHERE state_hash = $1
		  AND consumed_at IS NULL
		  AND expires_at > $2
	`, strings.TrimSpace(stateHash), now)
	if err != nil {
		return internalStoreError()
	}
	if result.RowsAffected() != 1 {
		return governanceOAuthStateStoreError()
	}
	return nil
}

func (s *Store) CreateAdminReauthenticationGrant(
	ctx context.Context,
	sessionTokenHash, purpose, method string,
	verifiedAt, expiresAt time.Time,
) (auth.AdminReauthenticationGrant, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.AdminReauthenticationGrant{}, internalStoreError()
	}
	if purpose != auth.AdminReauthenticationPurposeGrantAdmin ||
		(method != auth.AdminReauthenticationMethodPassword && method != auth.AdminReauthenticationMethodLinuxDoOAuth) {
		return auth.AdminReauthenticationGrant{}, auth.RecentReauthenticationRequiredError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return auth.AdminReauthenticationGrant{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	var grant auth.AdminReauthenticationGrant
	err = tx.QueryRow(ctx, `
		SELECT session.id::text, session.user_id::text
		FROM auth_sessions session
		JOIN users admin_user ON admin_user.id = session.user_id
		WHERE session.session_token_hash = $1
		  AND session.revoked_at IS NULL
		  AND session.expires_at > $2
		  AND session.absolute_expires_at > $2
		  AND admin_user.account_status = 'active'
		  AND admin_user.security_locked_at IS NULL
		  AND EXISTS (
		    SELECT 1 FROM user_permissions permission
		    WHERE permission.user_id = admin_user.id AND permission.permission = 'admin'
		  )
		FOR UPDATE OF session, admin_user
	`, sessionTokenHash, verifiedAt).Scan(&grant.AuthSessionID, &grant.AdminUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.AdminReauthenticationGrant{}, auth.RecentReauthenticationRequiredError()
	}
	if err != nil {
		return auth.AdminReauthenticationGrant{}, internalStoreError()
	}
	if _, err := tx.Exec(ctx, `
		UPDATE admin_reauthentication_grants
		SET revoked_at = $3
		WHERE auth_session_id = $1
		  AND purpose = $2
		  AND consumed_at IS NULL
		  AND revoked_at IS NULL
	`, grant.AuthSessionID, purpose, verifiedAt); err != nil {
		return auth.AdminReauthenticationGrant{}, internalStoreError()
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO admin_reauthentication_grants (
			admin_user_id, auth_session_id, purpose, method,
			verified_at, expires_at, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $5)
		RETURNING id::text, admin_user_id::text, auth_session_id::text,
		          purpose, method, verified_at, expires_at, consumed_at, revoked_at
	`, grant.AdminUserID, grant.AuthSessionID, purpose, method, verifiedAt, expiresAt).Scan(
		&grant.ID,
		&grant.AdminUserID,
		&grant.AuthSessionID,
		&grant.Purpose,
		&grant.Method,
		&grant.VerifiedAt,
		&grant.ExpiresAt,
		&grant.ConsumedAt,
		&grant.RevokedAt,
	)
	if err != nil {
		return auth.AdminReauthenticationGrant{}, internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return auth.AdminReauthenticationGrant{}, internalStoreError()
	}
	return grant, nil
}

func (s *Store) StartAdminReauthenticationOAuth(ctx context.Context, sessionTokenHash, stateHash, purpose string, expiresAt, now time.Time) *domain.AppError {
	if s == nil || s.pool == nil {
		return internalStoreError()
	}
	if purpose != auth.OAuthPurposeGrantAdminReauthentication {
		return auth.RecentReauthenticationRequiredError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return internalStoreError()
	}
	defer rollback(ctx, tx)
	var sessionID string
	var adminUserID string
	err = tx.QueryRow(ctx, `
		SELECT session.id::text, session.user_id::text
		FROM auth_sessions session
		JOIN users admin_user ON admin_user.id = session.user_id
		WHERE session.session_token_hash = $1
		  AND session.revoked_at IS NULL
		  AND session.expires_at > $2
		  AND session.absolute_expires_at > $2
		  AND admin_user.account_status = 'active'
		  AND admin_user.security_locked_at IS NULL
		  AND EXISTS (
		    SELECT 1 FROM user_permissions permission
		    WHERE permission.user_id = admin_user.id AND permission.permission = 'admin'
		  )
		FOR UPDATE OF session, admin_user
	`, sessionTokenHash, now).Scan(&sessionID, &adminUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.RecentReauthenticationRequiredError()
	}
	if err != nil {
		return internalStoreError()
	}
	if _, err := tx.Exec(ctx, `
		UPDATE admin_reauthentication_oauth_states
		SET consumed_at = COALESCE(consumed_at, $3)
		WHERE auth_session_id = $1 AND purpose = $2 AND consumed_at IS NULL
	`, sessionID, purpose, now); err != nil {
		return internalStoreError()
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO admin_reauthentication_oauth_states (
			admin_user_id, auth_session_id, state_hash, purpose,
			created_at, expires_at
		)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, adminUserID, sessionID, stateHash, purpose, now, expiresAt); err != nil {
		return internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) CompleteAdminReauthenticationOAuth(
	ctx context.Context,
	sessionTokenHash, stateHash string,
	profile auth.OAuthProfile,
	verifiedAt, expiresAt time.Time,
) (auth.AdminReauthenticationGrant, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.AdminReauthenticationGrant{}, internalStoreError()
	}
	provider := auth.CanonicalOAuthProvider(profile.Provider)
	subject := auth.CanonicalOAuthSubject(profile.Subject)
	if !auth.IsLinuxDoProvider(provider) || subject == "" {
		return auth.AdminReauthenticationGrant{}, auth.RecentReauthenticationRequiredError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return auth.AdminReauthenticationGrant{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	var grant auth.AdminReauthenticationGrant
	err = tx.QueryRow(ctx, `
		SELECT oauth_state.auth_session_id::text, oauth_state.admin_user_id::text
		FROM admin_reauthentication_oauth_states oauth_state
		JOIN auth_sessions session ON session.id = oauth_state.auth_session_id
		JOIN users admin_user ON admin_user.id = oauth_state.admin_user_id
		JOIN auth_identities identity
		  ON identity.user_id = admin_user.id
		 AND identity.provider = $3
		 AND identity.provider_subject = $4
		WHERE session.session_token_hash = $1
		  AND oauth_state.state_hash = $2
		  AND oauth_state.purpose = 'grant_admin_reauth'
		  AND oauth_state.consumed_at IS NULL
		  AND oauth_state.expires_at > $5
		  AND session.revoked_at IS NULL
		  AND session.expires_at > $5
		  AND session.absolute_expires_at > $5
		  AND admin_user.account_status = 'active'
		  AND admin_user.security_locked_at IS NULL
		  AND EXISTS (
		    SELECT 1 FROM user_permissions permission
		    WHERE permission.user_id = admin_user.id AND permission.permission = 'admin'
		  )
		FOR UPDATE OF oauth_state, session, admin_user
	`, sessionTokenHash, stateHash, provider, subject, verifiedAt).Scan(&grant.AuthSessionID, &grant.AdminUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.AdminReauthenticationGrant{}, auth.RecentReauthenticationRequiredError()
	}
	if err != nil {
		return auth.AdminReauthenticationGrant{}, internalStoreError()
	}
	if _, err := tx.Exec(ctx, `
		UPDATE admin_reauthentication_oauth_states
		SET consumed_at = $2
		WHERE state_hash = $1 AND consumed_at IS NULL
	`, stateHash, verifiedAt); err != nil {
		return auth.AdminReauthenticationGrant{}, internalStoreError()
	}
	if _, err := tx.Exec(ctx, `
		UPDATE admin_reauthentication_grants
		SET revoked_at = $3
		WHERE auth_session_id = $1
		  AND purpose = $2
		  AND consumed_at IS NULL
		  AND revoked_at IS NULL
	`, grant.AuthSessionID, auth.AdminReauthenticationPurposeGrantAdmin, verifiedAt); err != nil {
		return auth.AdminReauthenticationGrant{}, internalStoreError()
	}
	err = tx.QueryRow(ctx, `
		INSERT INTO admin_reauthentication_grants (
			admin_user_id, auth_session_id, purpose, method,
			verified_at, expires_at, created_at
		)
		VALUES ($1, $2, 'grant_admin', 'linux_do_oauth', $3, $4, $3)
		RETURNING id::text, admin_user_id::text, auth_session_id::text,
		          purpose, method, verified_at, expires_at, consumed_at, revoked_at
	`, grant.AdminUserID, grant.AuthSessionID, verifiedAt, expiresAt).Scan(
		&grant.ID,
		&grant.AdminUserID,
		&grant.AuthSessionID,
		&grant.Purpose,
		&grant.Method,
		&grant.VerifiedAt,
		&grant.ExpiresAt,
		&grant.ConsumedAt,
		&grant.RevokedAt,
	)
	if err != nil {
		return auth.AdminReauthenticationGrant{}, internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return auth.AdminReauthenticationGrant{}, internalStoreError()
	}
	return grant, nil
}

func (s *Store) CreateRestrictedBusinessSession(
	ctx context.Context,
	userID, sessionTokenHash, csrfTokenHash string,
	expiresAt, now time.Time,
) (auth.RestrictedBusinessSession, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.RestrictedBusinessSession{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return auth.RestrictedBusinessSession{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	var session auth.RestrictedBusinessSession
	var status string
	var securityLockedAt *time.Time
	err = tx.QueryRow(ctx, `
		SELECT u.account_status,
		       u.current_governance_action_id::text,
		       u.governance_version,
		       u.security_locked_at,
		       action.effective_at
		FROM users u
		LEFT JOIN account_governance_actions action
		  ON action.id = u.current_governance_action_id
		 AND action.status = 'effective'
		WHERE u.id = $1
		FOR UPDATE OF u
	`, userID).Scan(
		&status,
		&session.GovernanceActionID,
		&session.GovernanceVersion,
		&securityLockedAt,
		&session.RestrictionEffectiveAt,
	)
	if errors.Is(err, pgx.ErrNoRows) ||
		!auth.IsRestrictedBusinessAccountStatus(status) ||
		securityLockedAt != nil ||
		strings.TrimSpace(session.GovernanceActionID) == "" {
		return auth.RestrictedBusinessSession{}, restrictedBusinessSessionUnavailableError()
	}
	if err != nil {
		return auth.RestrictedBusinessSession{}, internalStoreError()
	}

	if _, err = tx.Exec(ctx, `
		UPDATE restricted_business_sessions
		SET revoked_at = COALESCE(revoked_at, $2),
		    last_seen_at = GREATEST(last_seen_at, $2)
		WHERE user_id = $1 AND revoked_at IS NULL
	`, userID, now); err != nil {
		return auth.RestrictedBusinessSession{}, internalStoreError()
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO restricted_business_sessions (
			user_id, session_token_hash, csrf_token_hash,
			governance_action_id, governance_version,
			restriction_effective_at, created_at, expires_at, last_seen_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $7)
		RETURNING id::text, user_id::text, governance_action_id::text,
		          governance_version, restriction_effective_at,
		          created_at, expires_at, revoked_at, last_seen_at
	`, userID, sessionTokenHash, csrfTokenHash, session.GovernanceActionID,
		session.GovernanceVersion, session.RestrictionEffectiveAt, now, expiresAt).Scan(
		&session.ID,
		&session.UserID,
		&session.GovernanceActionID,
		&session.GovernanceVersion,
		&session.RestrictionEffectiveAt,
		&session.CreatedAt,
		&session.ExpiresAt,
		&session.RevokedAt,
		&session.LastSeenAt,
	)
	if err != nil {
		return auth.RestrictedBusinessSession{}, internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return auth.RestrictedBusinessSession{}, internalStoreError()
	}
	return session, nil
}

func (s *Store) GetRestrictedBusinessSession(ctx context.Context, sessionTokenHash string, now time.Time) (auth.User, auth.RestrictedBusinessSession, *domain.AppError) {
	return s.getRestrictedBusinessSession(ctx, sessionTokenHash, "", false, now)
}

func (s *Store) GetRestrictedBusinessSessionWithCSRF(ctx context.Context, sessionTokenHash, csrfTokenHash string, now time.Time) (auth.User, auth.RestrictedBusinessSession, *domain.AppError) {
	return s.getRestrictedBusinessSession(ctx, sessionTokenHash, csrfTokenHash, true, now)
}

func (s *Store) RotateRestrictedBusinessSessionCSRF(ctx context.Context, sessionTokenHash, csrfTokenHash string, now time.Time) (auth.User, auth.RestrictedBusinessSession, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.User{}, auth.RestrictedBusinessSession{}, internalStoreError()
	}
	result, err := s.pool.Exec(ctx, `
		UPDATE restricted_business_sessions session
		SET csrf_token_hash = $2, last_seen_at = $3
		FROM users user_account, account_governance_actions action
		WHERE session.session_token_hash = $1
		  AND session.user_id = user_account.id
		  AND session.governance_action_id = action.id
		  AND session.revoked_at IS NULL
		  AND session.expires_at > $3
		  AND action.status = 'effective'
		  AND user_account.account_status IN ('suspended', 'banned')
		  AND user_account.security_locked_at IS NULL
		  AND user_account.current_governance_action_id = session.governance_action_id
		  AND user_account.governance_version = session.governance_version
	`, sessionTokenHash, csrfTokenHash, now)
	if err != nil {
		return auth.User{}, auth.RestrictedBusinessSession{}, internalStoreError()
	}
	if result.RowsAffected() != 1 {
		return auth.User{}, auth.RestrictedBusinessSession{}, restrictedBusinessSessionExpiredError()
	}
	return s.getRestrictedBusinessSession(ctx, sessionTokenHash, csrfTokenHash, true, now)
}

func (s *Store) RevokeRestrictedBusinessSession(ctx context.Context, sessionTokenHash string, revokedAt time.Time) *domain.AppError {
	if s == nil || s.pool == nil {
		return internalStoreError()
	}
	if _, err := s.pool.Exec(ctx, `
		UPDATE restricted_business_sessions
		SET revoked_at = COALESCE(revoked_at, $2),
		    last_seen_at = GREATEST(last_seen_at, $2)
		WHERE session_token_hash = $1
	`, sessionTokenHash, revokedAt); err != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) getRestrictedBusinessSession(
	ctx context.Context,
	sessionTokenHash, csrfTokenHash string,
	requireCSRF bool,
	now time.Time,
) (auth.User, auth.RestrictedBusinessSession, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.User{}, auth.RestrictedBusinessSession{}, internalStoreError()
	}
	query := `
		SELECT u.id::text, u.analytics_user_id::text, u.username, u.display_name,
		       u.account_status, u.governance_version,
		       u.current_governance_action_id::text, u.security_locked_at,
		       EXISTS (
		         SELECT 1 FROM user_permissions permission
		         WHERE permission.user_id = u.id AND permission.permission = 'admin'
		       ) AS is_admin,
		       session.id::text, session.user_id::text,
		       session.governance_action_id::text, session.governance_version,
		       session.restriction_effective_at, session.created_at,
		       session.expires_at, session.revoked_at, session.last_seen_at,
		       binding.linux_do_user_id, binding.linux_do_username,
		       binding.trust_level, binding.avatar_url,
		       binding.bound_at, binding.last_synced_at
		FROM restricted_business_sessions session
		JOIN users u ON u.id = session.user_id
		JOIN account_governance_actions action
		  ON action.id = session.governance_action_id
		LEFT JOIN linux_do_bindings binding ON binding.user_id = u.id
		WHERE session.session_token_hash = $1
		  AND session.revoked_at IS NULL
		  AND session.expires_at > $2
		  AND action.status = 'effective'
		  AND u.account_status IN ('suspended', 'banned')
		  AND u.security_locked_at IS NULL
		  AND u.current_governance_action_id = session.governance_action_id
		  AND u.governance_version = session.governance_version
	`
	args := []any{sessionTokenHash, now}
	if requireCSRF {
		query += ` AND session.csrf_token_hash = $3`
		args = append(args, csrfTokenHash)
	}
	var user auth.User
	var session auth.RestrictedBusinessSession
	var binding authLinuxDoBindingScan
	err := s.pool.QueryRow(ctx, query, args...).Scan(
		&user.ID,
		&user.AnalyticsUserID,
		&user.Username,
		&user.DisplayName,
		&user.Status,
		&user.GovernanceVersion,
		&user.CurrentGovernanceActionID,
		&user.SecurityLockedAt,
		&user.IsAdmin,
		&session.ID,
		&session.UserID,
		&session.GovernanceActionID,
		&session.GovernanceVersion,
		&session.RestrictionEffectiveAt,
		&session.CreatedAt,
		&session.ExpiresAt,
		&session.RevokedAt,
		&session.LastSeenAt,
		&binding.userID,
		&binding.username,
		&binding.trustLevel,
		&binding.avatarURL,
		&binding.boundAt,
		&binding.lastSyncedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		if requireCSRF {
			return auth.User{}, auth.RestrictedBusinessSession{}, restrictedBusinessCSRFError()
		}
		return auth.User{}, auth.RestrictedBusinessSession{}, restrictedBusinessSessionExpiredError()
	}
	if err != nil {
		return auth.User{}, auth.RestrictedBusinessSession{}, internalStoreError()
	}
	applyAuthLinuxDoBinding(&user, binding)
	if err := hydrateAuthStudentClaim(ctx, s.pool, &user); err != nil {
		return auth.User{}, auth.RestrictedBusinessSession{}, internalStoreError()
	}
	if _, err := s.pool.Exec(ctx, `
		UPDATE restricted_business_sessions
		SET last_seen_at = $2
		WHERE session_token_hash = $1
	`, sessionTokenHash, now); err != nil {
		return auth.User{}, auth.RestrictedBusinessSession{}, internalStoreError()
	}
	session.LastSeenAt = now
	return auth.HydrateCapabilities(user), session, nil
}

func restrictedBusinessSessionUnavailableError() *domain.AppError {
	return domain.NewError(http.StatusForbidden, domain.CodeAccountRestricted, "Restricted business unavailable", "当前账号没有可用的受限业务入口。")
}

func restrictedBusinessSessionExpiredError() *domain.AppError {
	return domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Restricted business session expired", "受限业务会话已失效，请重新验证身份。")
}

func restrictedBusinessCSRFError() *domain.AppError {
	return domain.NewError(http.StatusForbidden, domain.CodeCSRFTokenInvalid, "CSRF token invalid", "受限业务 CSRF token 无效或缺失。")
}

func governanceOAuthStateStoreError() *domain.AppError {
	return domain.NewError(http.StatusForbidden, domain.CodeCSRFTokenInvalid, "OAuth state invalid", "OAuth state 无效或已过期。")
}
