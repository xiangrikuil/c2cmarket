package postgres

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/profile"

	"github.com/jackc/pgx/v5"
)

func (s *Store) GetUserProfile(ctx context.Context, userID string, now time.Time) (profile.UserProfile, *domain.AppError) {
	if s == nil || s.pool == nil {
		return profile.UserProfile{}, internalStoreError()
	}
	value, err := scanUserProfile(ctx, s.pool, `
		SELECT u.id::text, u.username, u.display_name, COALESCE(u.bio, ''),
		       COALESCE(CASE WHEN u.avatar_mode = 'custom_url' THEN u.custom_avatar_url ELSE COALESCE(l.avatar_url, u.avatar_url) END, ''),
		       COALESCE(u.custom_avatar_url, ''), COALESCE(u.email, ''), u.email_verified_at,
		       EXISTS(SELECT 1 FROM user_password_credentials pc WHERE pc.user_id = u.id) AS password_configured,
		       u.account_status, EXISTS(SELECT 1 FROM user_permissions p WHERE p.user_id = u.id AND p.permission = 'admin') AS is_admin,
		       COALESCE(u.region_code, ''), COALESCE(u.timezone, ''), u.avatar_mode, u.privacy_settings::text,
		       u.created_at, u.updated_at, u.last_active_at, u.version,
		       (l.id IS NOT NULL) AS linux_do_bound, COALESCE(l.linux_do_user_id, ''), COALESCE(l.linux_do_username, ''), COALESCE(l.avatar_url, ''),
		       COALESCE(l.trust_level, 0), l.last_synced_at
		FROM users u
		LEFT JOIN linux_do_bindings l ON l.user_id = u.id
		WHERE u.id = $1
	`, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return profile.UserProfile{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Profile not found", "个人资料不存在。")
	}
	if err != nil {
		return profile.UserProfile{}, internalStoreError()
	}
	value.UsernameCanChange = true
	return value, nil
}

func (s *Store) UpdateUserProfile(ctx context.Context, input profile.UpdateUserProfileInput, now time.Time) (profile.UserProfile, *domain.AppError) {
	if s == nil || s.pool == nil {
		return profile.UserProfile{}, internalStoreError()
	}
	privacyJSON, err := json.Marshal(input.Privacy)
	if err != nil {
		return profile.UserProfile{}, internalStoreError()
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE users
		SET username = $2,
		    display_name = $3,
		    bio = $4,
		    region_code = $5,
		    timezone = $6,
		    avatar_mode = $7,
		    custom_avatar_url = $8,
		    avatar_url = CASE WHEN $7 = 'custom_url' THEN $8 ELSE avatar_url END,
		    privacy_settings = $9::jsonb,
		    updated_at = $10,
		    version = version + 1
		WHERE id = $1
	`, input.UserID, input.Username, strings.TrimSpace(input.DisplayName), nullText(input.Bio),
		nullText(input.RegionCode), nullText(input.Timezone), input.AvatarMode, nullText(input.AvatarURL), string(privacyJSON), now)
	if isUniqueViolation(err) {
		return profile.UserProfile{}, domain.NewFieldError(http.StatusConflict, domain.CodeUsernameUnavailable, "Username unavailable", "站内用户名已被占用。", "username", "unavailable", "站内用户名已被占用。")
	}
	if err != nil {
		return profile.UserProfile{}, internalStoreError()
	}
	return s.GetUserProfile(ctx, input.UserID, now)
}

func (s *Store) CreateEmailVerificationCode(ctx context.Context, input profile.EmailVerificationStartInput, codeHash string, expiresAt, now time.Time) *domain.AppError {
	if s == nil || s.pool == nil {
		return internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return internalStoreError()
	}
	defer rollback(ctx, tx)

	var lockedUserID string
	err = tx.QueryRow(ctx, `
		SELECT id::text
		FROM users
		WHERE id = $1
		FOR UPDATE
	`, input.UserID).Scan(&lockedUserID)
	if err != nil {
		return internalStoreError()
	}

	var exists bool
	err = tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM users
			WHERE lower(email) = lower($1)
			  AND email_verified_at IS NOT NULL
			  AND id <> $2
		)
	`, strings.TrimSpace(input.Email), input.UserID).Scan(&exists)
	if err != nil {
		return internalStoreError()
	}
	if exists {
		return domain.NewFieldError(http.StatusConflict, domain.CodeValidationFailed, "Email unavailable", "该邮箱已绑定其他账号。", "email", "unavailable", "该邮箱已绑定其他账号。")
	}
	_, err = tx.Exec(ctx, `
		UPDATE email_verification_codes
		SET consumed_at = $2
		WHERE user_id = $1
		  AND purpose = 'bind_email'
		  AND consumed_at IS NULL
	`, input.UserID, now)
	if err != nil {
		return internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO email_verification_codes (user_id, email, purpose, code_hash, expires_at, created_at)
		VALUES ($1, lower($2), 'bind_email', $3, $4, $5)
	`, input.UserID, strings.TrimSpace(input.Email), strings.TrimSpace(codeHash), expiresAt, now)
	if err != nil {
		return internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) ConfirmEmailVerificationCode(ctx context.Context, input profile.EmailVerificationConfirmInput, codeHash string, now time.Time) (profile.UserProfile, *domain.AppError) {
	if s == nil || s.pool == nil {
		return profile.UserProfile{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return profile.UserProfile{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	var codeID string
	var storedHash string
	var expiresAt time.Time
	var attemptCount int
	err = tx.QueryRow(ctx, `
		SELECT id::text, code_hash, expires_at, attempt_count
		FROM email_verification_codes
		WHERE user_id = $1
		  AND email = lower($2)
		  AND purpose = 'bind_email'
		  AND consumed_at IS NULL
		ORDER BY created_at DESC
		LIMIT 1
		FOR UPDATE
	`, input.UserID, strings.TrimSpace(input.Email)).Scan(&codeID, &storedHash, &expiresAt, &attemptCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return profile.UserProfile{}, invalidEmailVerificationStoreError()
	}
	if err != nil {
		return profile.UserProfile{}, internalStoreError()
	}
	if !now.Before(expiresAt) || attemptCount >= profile.EmailVerificationMaxAttempts {
		if _, err := tx.Exec(ctx, `
			UPDATE email_verification_codes
			SET consumed_at = COALESCE(consumed_at, $2)
			WHERE id = $1
		`, codeID, now); err != nil {
			return profile.UserProfile{}, internalStoreError()
		}
		if err := tx.Commit(ctx); err != nil {
			return profile.UserProfile{}, internalStoreError()
		}
		return profile.UserProfile{}, invalidEmailVerificationStoreError()
	}
	if subtle.ConstantTimeCompare([]byte(storedHash), []byte(strings.TrimSpace(codeHash))) != 1 {
		if _, err := tx.Exec(ctx, `
			UPDATE email_verification_codes
			SET attempt_count = attempt_count + 1,
			    consumed_at = CASE
			      WHEN attempt_count + 1 >= $3 THEN $2
			      ELSE consumed_at
			    END
			WHERE id = $1
		`, codeID, now, profile.EmailVerificationMaxAttempts); err != nil {
			return profile.UserProfile{}, internalStoreError()
		}
		if err := tx.Commit(ctx); err != nil {
			return profile.UserProfile{}, internalStoreError()
		}
		return profile.UserProfile{}, invalidEmailVerificationStoreError()
	}

	var exists bool
	err = tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM users
			WHERE lower(email) = lower($1)
			  AND email_verified_at IS NOT NULL
			  AND id <> $2
		)
	`, strings.TrimSpace(input.Email), input.UserID).Scan(&exists)
	if err != nil {
		return profile.UserProfile{}, internalStoreError()
	}
	if exists {
		return profile.UserProfile{}, domain.NewFieldError(http.StatusConflict, domain.CodeValidationFailed, "Email unavailable", "该邮箱已绑定其他账号。", "email", "unavailable", "该邮箱已绑定其他账号。")
	}

	_, err = tx.Exec(ctx, `
		UPDATE users
		SET email = lower($2),
		    email_verified_at = $3,
		    updated_at = $3,
		    version = version + 1
		WHERE id = $1
	`, input.UserID, strings.TrimSpace(input.Email), now)
	if isUniqueViolation(err) {
		return profile.UserProfile{}, domain.NewFieldError(http.StatusConflict, domain.CodeValidationFailed, "Email unavailable", "该邮箱已绑定其他账号。", "email", "unavailable", "该邮箱已绑定其他账号。")
	}
	if err != nil {
		return profile.UserProfile{}, internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		UPDATE email_verification_codes
		SET consumed_at = $2
		WHERE id = $1
	`, codeID, now)
	if err != nil {
		return profile.UserProfile{}, internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return profile.UserProfile{}, internalStoreError()
	}
	return s.GetUserProfile(ctx, input.UserID, now)
}

func invalidEmailVerificationStoreError() *domain.AppError {
	return domain.NewError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Code invalid", "验证码无效或已过期。")
}

func (s *Store) GetPublicUserProfile(ctx context.Context, username string, now time.Time) (profile.PublicUserProfile, *domain.AppError) {
	if s == nil || s.pool == nil {
		return profile.PublicUserProfile{}, internalStoreError()
	}
	value, err := scanUserProfile(ctx, s.pool, `
		SELECT u.id::text, u.username, u.display_name, COALESCE(u.bio, ''),
		       COALESCE(CASE WHEN u.avatar_mode = 'custom_url' THEN u.custom_avatar_url ELSE COALESCE(l.avatar_url, u.avatar_url) END, ''),
		       COALESCE(u.custom_avatar_url, ''), COALESCE(u.email, ''), u.email_verified_at,
		       EXISTS(SELECT 1 FROM user_password_credentials pc WHERE pc.user_id = u.id) AS password_configured,
		       u.account_status, EXISTS(SELECT 1 FROM user_permissions p WHERE p.user_id = u.id AND p.permission = 'admin') AS is_admin,
		       COALESCE(u.region_code, ''), COALESCE(u.timezone, ''), u.avatar_mode, u.privacy_settings::text,
		       u.created_at, u.updated_at, u.last_active_at, u.version,
		       (l.id IS NOT NULL) AS linux_do_bound, COALESCE(l.linux_do_user_id, ''), COALESCE(l.linux_do_username, ''), COALESCE(l.avatar_url, ''),
		       COALESCE(l.trust_level, 0), l.last_synced_at
		FROM users u
		LEFT JOIN linux_do_bindings l ON l.user_id = u.id
		WHERE u.username = $1 AND u.account_status = 'active'
	`, strings.TrimSpace(strings.ToLower(username)))
	if errors.Is(err, pgx.ErrNoRows) {
		return profile.PublicUserProfile{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Profile not found", "公开主页不存在。")
	}
	if err != nil {
		return profile.PublicUserProfile{}, internalStoreError()
	}
	return profilePublicFromUser(value), nil
}

func (s *Store) GetMerchantProfile(ctx context.Context, ownerUserID string, now time.Time) (profile.MerchantProfile, *domain.AppError) {
	if s == nil || s.pool == nil {
		return profile.MerchantProfile{}, internalStoreError()
	}
	value, err := scanMerchantProfile(ctx, s.pool, `
		SELECT id::text, owner_user_id::text, slug, display_name, COALESCE(avatar_url, ''),
		       status, created_at, updated_at, version
		FROM merchant_profiles
		WHERE owner_user_id = $1 AND status <> 'archived'
	`, ownerUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return profile.MerchantProfile{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Merchant profile not found", "商户资料不存在。")
	}
	if err != nil {
		return profile.MerchantProfile{}, internalStoreError()
	}
	return value, nil
}

func (s *Store) UpsertMerchantProfile(ctx context.Context, input profile.UpsertMerchantProfileInput, now time.Time) (profile.MerchantProfile, *domain.AppError) {
	if s == nil || s.pool == nil {
		return profile.MerchantProfile{}, internalStoreError()
	}
	slug := strings.TrimSpace(strings.ToLower(input.Slug))
	if slug == "" {
		slug = strings.TrimSpace(strings.ToLower(input.DisplayName))
	}
	value, err := scanMerchantProfile(ctx, s.pool, `
		INSERT INTO merchant_profiles (owner_user_id, slug, display_name, avatar_url, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'active', $5, $5)
		ON CONFLICT (owner_user_id) DO UPDATE
		SET slug = EXCLUDED.slug,
		    display_name = EXCLUDED.display_name,
		    avatar_url = EXCLUDED.avatar_url,
		    status = 'active',
		    updated_at = EXCLUDED.updated_at,
		    version = merchant_profiles.version + 1
		RETURNING id::text, owner_user_id::text, slug, display_name, COALESCE(avatar_url, ''),
		          status, created_at, updated_at, version
	`, input.OwnerUserID, slug, strings.TrimSpace(input.DisplayName), nullText(input.AvatarURL), now)
	if isUniqueViolation(err) {
		return profile.MerchantProfile{}, domain.NewFieldError(http.StatusConflict, domain.CodeValidationFailed, "Merchant slug unavailable", "店铺别名已被占用。", "slug", "unavailable", "店铺别名已被占用。")
	}
	if err != nil {
		return profile.MerchantProfile{}, internalStoreError()
	}
	return value, nil
}

func (s *Store) GetPublicMerchantProfile(ctx context.Context, slug string, now time.Time) (profile.PublicMerchantProfile, *domain.AppError) {
	if s == nil || s.pool == nil {
		return profile.PublicMerchantProfile{}, internalStoreError()
	}
	var value profile.PublicMerchantProfile
	err := s.pool.QueryRow(ctx, `
		SELECT merchant.id::text, merchant.owner_user_id::text, merchant.slug, merchant.display_name,
		       COALESCE(merchant.avatar_url, ''), merchant.created_at, users.last_active_at,
		       (binding.id IS NOT NULL) AS linux_do_bound,
		       CASE WHEN binding.id IS NOT NULL THEN binding.trust_level ELSE NULL END
		FROM merchant_profiles merchant
		JOIN users ON users.id = merchant.owner_user_id
		LEFT JOIN linux_do_bindings binding ON binding.user_id = merchant.owner_user_id
		WHERE merchant.slug = $1
		  AND merchant.status = 'active'
		  AND users.account_status = 'active'
	`, strings.TrimSpace(strings.ToLower(slug))).Scan(
		&value.ID,
		&value.OwnerUserID,
		&value.Slug,
		&value.DisplayName,
		&value.AvatarURL,
		&value.JoinedAt,
		&value.LastActiveAt,
		&value.LinuxDoBound,
		&value.TrustLevel,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return profile.PublicMerchantProfile{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Merchant profile not found", "商户公开主页不存在。")
	}
	if err != nil {
		return profile.PublicMerchantProfile{}, internalStoreError()
	}
	value.AvatarText = firstAvatarText(value.DisplayName)
	value.Identity = "API 商户"
	return value, nil
}

func scanUserProfile(ctx context.Context, q queryer, sql string, args ...any) (profile.UserProfile, error) {
	var value profile.UserProfile
	var privacyText string
	var trustLevel int
	err := q.QueryRow(ctx, sql, args...).Scan(
		&value.ID,
		&value.Username,
		&value.DisplayName,
		&value.Bio,
		&value.AvatarURL,
		&value.CustomAvatarURL,
		&value.Email,
		&value.EmailVerifiedAt,
		&value.PasswordConfigured,
		&value.AccountStatus,
		&value.IsAdmin,
		&value.RegionCode,
		&value.Timezone,
		&value.AvatarMode,
		&privacyText,
		&value.CreatedAt,
		&value.UpdatedAt,
		&value.LastActiveAt,
		&value.Version,
		&value.LinuxDoBound,
		&value.LinuxDoUserID,
		&value.LinuxDoUsername,
		&value.LinuxDoAvatarURL,
		&trustLevel,
		&value.LinuxDoLastSyncedAt,
	)
	if err != nil {
		return profile.UserProfile{}, err
	}
	if err := json.Unmarshal([]byte(privacyText), &value.Privacy); err != nil {
		return profile.UserProfile{}, err
	}
	if value.LinuxDoBound {
		value.LinuxDoTrustLevel = &trustLevel
	}
	return value, nil
}

func scanMerchantProfile(ctx context.Context, q queryer, sql string, args ...any) (profile.MerchantProfile, error) {
	var value profile.MerchantProfile
	err := q.QueryRow(ctx, sql, args...).Scan(
		&value.ID,
		&value.OwnerUserID,
		&value.Slug,
		&value.DisplayName,
		&value.AvatarURL,
		&value.Status,
		&value.CreatedAt,
		&value.UpdatedAt,
		&value.Version,
	)
	return value, err
}

func profilePublicFromUser(value profile.UserProfile) profile.PublicUserProfile {
	createdAt := &value.CreatedAt
	if !value.Privacy.ShowCreatedAt {
		createdAt = nil
	}
	lastActiveAt := value.LastActiveAt
	if !value.Privacy.ShowLastActiveAt {
		lastActiveAt = nil
	}
	badges := []string{}
	if value.LinuxDoBound {
		badges = append(badges, "linuxdo_bound")
	}
	if value.IsAdmin {
		badges = append(badges, "admin")
	}
	return profile.PublicUserProfile{
		ID:              value.ID,
		Username:        value.Username,
		DisplayName:     value.DisplayName,
		Bio:             value.Bio,
		AvatarURL:       value.AvatarURL,
		AvatarText:      firstAvatarText(value.DisplayName),
		LinuxDoBound:    value.LinuxDoBound,
		LinuxDoUsername: value.LinuxDoUsername,
		TrustLevel:      value.LinuxDoTrustLevel,
		AccountStatus:   value.AccountStatus,
		CreatedAt:       createdAt,
		LastActiveAt:    lastActiveAt,
		Privacy:         value.Privacy,
		Badges:          badges,
	}
}

func firstAvatarText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "U"
	}
	return strings.ToUpper(string([]rune(value)[0]))
}
