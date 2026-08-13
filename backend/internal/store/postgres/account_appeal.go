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

const accountGovernanceAdvisoryLockPrefix = "account_governance:"

func (s *Store) ResolveExistingOAuthUser(ctx context.Context, provider, subject string) (auth.User, bool, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.User{}, false, internalStoreError()
	}
	provider = auth.CanonicalOAuthProvider(provider)
	subject = auth.CanonicalOAuthSubject(subject)
	if provider == "" || subject == "" {
		return auth.User{}, false, nil
	}
	user, found, err := oauthUserByIdentity(ctx, s.pool, provider, subject, false)
	if err != nil {
		return auth.User{}, false, internalStoreError()
	}
	return user, found, nil
}

func (s *Store) CompleteAccountAppealOAuth(
	ctx context.Context,
	stateHash string,
	profile auth.OAuthProfile,
	sessionTokenHash, csrfTokenHash string,
	sessionExpiresAt, now time.Time,
) (auth.User, auth.AccountAppealSession, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.User{}, auth.AccountAppealSession{}, internalStoreError()
	}
	provider := auth.CanonicalOAuthProvider(profile.Provider)
	subject := auth.CanonicalOAuthSubject(profile.Subject)
	if !auth.IsLinuxDoProvider(provider) || subject == "" {
		return auth.User{}, auth.AccountAppealSession{}, accountAppealStoreIneligibleError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return auth.User{}, auth.AccountAppealSession{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	if appErr := consumeGovernanceOAuthState(ctx, tx, "account_appeal_oauth_states", stateHash, now); appErr != nil {
		return auth.User{}, auth.AccountAppealSession{}, appErr
	}

	var userID string
	err = tx.QueryRow(ctx, `
		SELECT user_id::text
		FROM auth_identities
		WHERE provider = $1 AND provider_subject = $2
	`, provider, subject).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := tx.Commit(ctx); err != nil {
			return auth.User{}, auth.AccountAppealSession{}, internalStoreError()
		}
		return auth.User{}, auth.AccountAppealSession{}, accountAppealStoreIneligibleError()
	}
	if err != nil {
		return auth.User{}, auth.AccountAppealSession{}, internalStoreError()
	}
	if appErr := lockAccountGovernanceUser(ctx, tx, userID); appErr != nil {
		return auth.User{}, auth.AccountAppealSession{}, appErr
	}
	user, found, err := oauthUserByIdentity(ctx, tx, provider, subject, true)
	if err != nil {
		return auth.User{}, auth.AccountAppealSession{}, internalStoreError()
	}
	if !found || !isAccountAppealStatus(user.Status) {
		if err := tx.Commit(ctx); err != nil {
			return auth.User{}, auth.AccountAppealSession{}, internalStoreError()
		}
		return auth.User{}, auth.AccountAppealSession{}, accountAppealStoreIneligibleError()
	}
	if _, err := tx.Exec(ctx, `
		UPDATE account_appeal_sessions
		SET revoked_at = COALESCE(revoked_at, $2)
		WHERE user_id = $1 AND revoked_at IS NULL
	`, user.ID, now); err != nil {
		return auth.User{}, auth.AccountAppealSession{}, internalStoreError()
	}
	var session auth.AccountAppealSession
	err = tx.QueryRow(ctx, `
		INSERT INTO account_appeal_sessions (
			user_id, session_token_hash, csrf_token_hash, created_at, expires_at
		)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id::text, user_id::text, created_at, expires_at, revoked_at
	`, user.ID, strings.TrimSpace(sessionTokenHash), strings.TrimSpace(csrfTokenHash), now, sessionExpiresAt).Scan(
		&session.ID,
		&session.UserID,
		&session.CreatedAt,
		&session.ExpiresAt,
		&session.RevokedAt,
	)
	if err != nil {
		return auth.User{}, auth.AccountAppealSession{}, internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return auth.User{}, auth.AccountAppealSession{}, internalStoreError()
	}
	return auth.HydrateCapabilities(user), session, nil
}

func (s *Store) CreateAccountAppealSession(ctx context.Context, userID, sessionTokenHash, csrfTokenHash string, expiresAt, now time.Time) (auth.User, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.User{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return auth.User{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	if appErr := lockAccountGovernanceUser(ctx, tx, userID); appErr != nil {
		return auth.User{}, appErr
	}
	user, found, err := accountAppealUserByID(ctx, tx, userID, true)
	if err != nil {
		return auth.User{}, internalStoreError()
	}
	if !found || !isAccountAppealStatus(user.Status) {
		return auth.User{}, accountAppealStoreIneligibleError()
	}
	if _, err := tx.Exec(ctx, `
		UPDATE account_appeal_sessions
		SET revoked_at = $2
		WHERE user_id = $1 AND revoked_at IS NULL
	`, user.ID, now); err != nil {
		return auth.User{}, internalStoreError()
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO account_appeal_sessions (
			user_id, session_token_hash, csrf_token_hash, created_at, expires_at
		)
		VALUES ($1, $2, $3, $4, $5)
	`, user.ID, strings.TrimSpace(sessionTokenHash), strings.TrimSpace(csrfTokenHash), now, expiresAt); err != nil {
		return auth.User{}, internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return auth.User{}, internalStoreError()
	}
	return user, nil
}

func (s *Store) RotateAccountAppealSessionCSRF(ctx context.Context, sessionTokenHash, csrfTokenHash string, now time.Time) (auth.User, auth.AccountAppealSession, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.User{}, auth.AccountAppealSession{}, internalStoreError()
	}
	var user auth.User
	var session auth.AccountAppealSession
	err := s.pool.QueryRow(ctx, `
		UPDATE account_appeal_sessions session
		SET csrf_token_hash = $2
		FROM users user_account
		WHERE session.session_token_hash = $1
		  AND session.user_id = user_account.id
		  AND session.revoked_at IS NULL
		  AND session.expires_at > $3
		  AND user_account.account_status IN ('suspended', 'banned')
		RETURNING user_account.id::text,
		          user_account.analytics_user_id::text,
		          user_account.username,
		          user_account.display_name,
		          user_account.account_status,
		          EXISTS (
		            SELECT 1
		            FROM user_permissions permission
		            WHERE permission.user_id = user_account.id
		              AND permission.permission = 'admin'
		          ) AS is_admin,
		          session.id::text,
		          session.user_id::text,
		          session.created_at,
		          session.expires_at,
		          session.revoked_at
	`, strings.TrimSpace(sessionTokenHash), strings.TrimSpace(csrfTokenHash), now).Scan(
		&user.ID,
		&user.AnalyticsUserID,
		&user.Username,
		&user.DisplayName,
		&user.Status,
		&user.IsAdmin,
		&session.ID,
		&session.UserID,
		&session.CreatedAt,
		&session.ExpiresAt,
		&session.RevokedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, auth.AccountAppealSession{}, accountAppealStoreSessionExpiredError()
	}
	if err != nil {
		return auth.User{}, auth.AccountAppealSession{}, internalStoreError()
	}
	return user, session, nil
}

func (s *Store) GetAccountAppealSessionWithCSRF(ctx context.Context, sessionTokenHash, csrfTokenHash string, now time.Time) (auth.User, auth.AccountAppealSession, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.User{}, auth.AccountAppealSession{}, internalStoreError()
	}
	var user auth.User
	var session auth.AccountAppealSession
	err := s.pool.QueryRow(ctx, `
		SELECT user_account.id::text,
		       user_account.analytics_user_id::text,
		       user_account.username,
		       user_account.display_name,
		       user_account.account_status,
		       user_account.governance_version,
		       COALESCE(user_account.current_governance_action_id::text, ''),
		       user_account.security_locked_at,
		       EXISTS (
		         SELECT 1
		         FROM user_permissions permission
		         WHERE permission.user_id = user_account.id
		           AND permission.permission = 'admin'
		       ) AS is_admin,
		       session.id::text,
		       session.user_id::text,
		       session.created_at,
		       session.expires_at,
		       session.revoked_at
		FROM account_appeal_sessions session
		JOIN users user_account ON user_account.id = session.user_id
		WHERE session.session_token_hash = $1
		  AND session.csrf_token_hash = $2
		  AND session.revoked_at IS NULL
		  AND session.expires_at > $3
		  AND user_account.account_status IN ('suspended', 'banned')
	`, strings.TrimSpace(sessionTokenHash), strings.TrimSpace(csrfTokenHash), now).Scan(
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
		&session.CreatedAt,
		&session.ExpiresAt,
		&session.RevokedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, auth.AccountAppealSession{}, accountAppealStoreCSRFError()
	}
	if err != nil {
		return auth.User{}, auth.AccountAppealSession{}, internalStoreError()
	}
	return user, session, nil
}

func lockAccountGovernanceUser(ctx context.Context, tx pgx.Tx, userID string) *domain.AppError {
	if _, err := tx.Exec(ctx, `
		SELECT pg_advisory_xact_lock(
			hashtextextended($1 || $2::uuid::text, 0)
		)
	`, accountGovernanceAdvisoryLockPrefix, strings.TrimSpace(userID)); err != nil {
		return internalStoreError()
	}
	return nil
}

func accountAppealUserByID(ctx context.Context, q queryer, userID string, lock bool) (auth.User, bool, error) {
	lockClause := ""
	if lock {
		lockClause = " FOR UPDATE OF user_account"
	}
	var user auth.User
	err := q.QueryRow(ctx, `
		SELECT user_account.id::text,
		       user_account.analytics_user_id::text,
		       user_account.username,
		       user_account.display_name,
		       user_account.account_status,
		       EXISTS (
		         SELECT 1
		         FROM user_permissions permission
		         WHERE permission.user_id = user_account.id
		           AND permission.permission = 'admin'
		       ) AS is_admin
		FROM users user_account
		WHERE user_account.id = $1
	`+lockClause, strings.TrimSpace(userID)).Scan(
		&user.ID,
		&user.AnalyticsUserID,
		&user.Username,
		&user.DisplayName,
		&user.Status,
		&user.IsAdmin,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, false, nil
	}
	if err != nil {
		return auth.User{}, false, err
	}
	return user, true, nil
}

func isAccountAppealStatus(status string) bool {
	return status == auth.AccountStatusSuspended || status == auth.AccountStatusBanned
}

func accountAppealStoreIneligibleError() *domain.AppError {
	return domain.NewError(http.StatusForbidden, domain.CodeAccountAppealIneligible, "Account appeal unavailable", "当前身份无法使用账号申诉验证。")
}

func accountAppealStoreSessionExpiredError() *domain.AppError {
	return domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Account appeal session expired", "账号申诉验证已过期，请重新验证。")
}

func accountAppealStoreCSRFError() *domain.AppError {
	return domain.NewError(http.StatusForbidden, domain.CodeCSRFTokenInvalid, "Account appeal CSRF token invalid", "账号申诉 CSRF token 无效或缺失。")
}
