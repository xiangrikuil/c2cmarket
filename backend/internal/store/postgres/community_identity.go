package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/communityidentity"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Store) GrantFounding(ctx context.Context, input communityidentity.GrantFoundingInput, now time.Time) (communityidentity.Identity, bool, *domain.AppError) {
	if s == nil || s.pool == nil {
		return communityidentity.Identity{}, false, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return communityidentity.Identity{}, false, internalStoreError()
	}
	defer rollback(ctx, tx)

	item, created, appErr := insertCommunityIdentity(ctx, tx, input.UserID, communityidentity.IdentityTypeFoundingUser, input.Source, input.QualifiedAt, "", "", now)
	if appErr != nil {
		return communityidentity.Identity{}, false, appErr
	}
	if created {
		if appErr := insertCommunityIdentityNotification(ctx, tx, item, communityIdentityGrantRequestID(item), now); appErr != nil {
			return communityidentity.Identity{}, false, appErr
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return communityidentity.Identity{}, false, internalStoreError()
	}
	return item, created, nil
}

func (s *Store) GrantAdmin(ctx context.Context, input communityidentity.GrantAdminInput, now time.Time) (communityidentity.Identity, bool, *domain.AppError) {
	if s == nil || s.pool == nil {
		return communityidentity.Identity{}, false, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return communityidentity.Identity{}, false, internalStoreError()
	}
	defer rollback(ctx, tx)

	item, created, appErr := insertCommunityIdentity(ctx, tx, input.TargetUserID, input.Type, communityidentity.SourceAdmin, time.Time{}, input.AdminUserID, input.Reason, now)
	if appErr != nil {
		return communityidentity.Identity{}, false, appErr
	}
	if created {
		if appErr := insertCommunityIdentityNotification(ctx, tx, item, input.RequestID, now); appErr != nil {
			return communityidentity.Identity{}, false, appErr
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return communityidentity.Identity{}, false, internalStoreError()
	}
	return item, created, nil
}

func (s *Store) Revoke(ctx context.Context, input communityidentity.RevokeInput, now time.Time) (communityidentity.Identity, *domain.AppError) {
	if s == nil || s.pool == nil {
		return communityidentity.Identity{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return communityidentity.Identity{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	item, err := scanCommunityIdentity(tx.QueryRow(ctx, `
		UPDATE user_community_identities
		SET revoked_at = $3,
		    revoked_by = $4,
		    revoke_reason = $5,
		    updated_at = $3
		WHERE user_id = $1 AND identity_type = $2 AND revoked_at IS NULL
		RETURNING id::text, user_id::text, identity_type, source, qualified_at, granted_at,
		          COALESCE(granted_by::text, ''), COALESCE(grant_reason, ''), revoked_at,
		          COALESCE(revoked_by::text, ''), COALESCE(revoke_reason, ''), created_at, updated_at
	`, input.TargetUserID, input.Type, now, input.AdminUserID, input.Reason))
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return communityidentity.Identity{}, internalStoreError()
	}
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM user_community_identities
				WHERE user_id = $1 AND identity_type = $2
			)
		`, input.TargetUserID, input.Type).Scan(&exists); err != nil {
			return communityidentity.Identity{}, internalStoreError()
		}
		if !exists {
			return communityidentity.Identity{}, communityIdentityNotFoundError()
		}
		return communityidentity.Identity{}, communityIdentityDuplicateError()
	}
	if err := tx.Commit(ctx); err != nil {
		return communityidentity.Identity{}, internalStoreError()
	}
	return item, nil
}

func (s *Store) ListForUser(ctx context.Context, userID string, includeRevoked bool) ([]communityidentity.Identity, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, user_id::text, identity_type, source, qualified_at, granted_at,
		       COALESCE(granted_by::text, ''), COALESCE(grant_reason, ''), revoked_at,
		       COALESCE(revoked_by::text, ''), COALESCE(revoke_reason, ''), created_at, updated_at
		FROM user_community_identities
		WHERE user_id = $1 AND ($2 OR revoked_at IS NULL)
		ORDER BY CASE WHEN identity_type = 'BETA_CONTRIBUTOR' THEN 0 ELSE 1 END, granted_at ASC
	`, userID, includeRevoked)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	items := make([]communityidentity.Identity, 0)
	for rows.Next() {
		item, err := scanCommunityIdentity(rows)
		if err != nil {
			return nil, internalStoreError()
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, internalStoreError()
	}
	return items, nil
}

func (s *Store) BackfillFounding(ctx context.Context, cutoff, now time.Time) (int, *domain.AppError) {
	if s == nil || s.pool == nil {
		return 0, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, internalStoreError()
	}
	defer rollback(ctx, tx)

	rows, err := tx.Query(ctx, `
		WITH eligible AS (
			SELECT u.id,
			       LEAST(l.bound_at, u.email_verified_at) AS qualified_at
			FROM users u
			LEFT JOIN linux_do_bindings l ON l.user_id = u.id
			WHERE u.account_status = 'active'
			  AND NOT EXISTS (
				SELECT 1 FROM user_permissions p
				WHERE p.user_id = u.id AND p.permission = 'admin'
			  )
			  AND (l.bound_at IS NOT NULL OR u.email_verified_at IS NOT NULL)
			  AND LEAST(l.bound_at, u.email_verified_at) <= $1
		)
		INSERT INTO user_community_identities (
			user_id, identity_type, source, qualified_at, granted_at, created_at, updated_at
		)
		SELECT id, 'FOUNDING_USER', 'BACKFILL', qualified_at, $2, $2, $2
		FROM eligible
		ON CONFLICT (user_id, identity_type) DO NOTHING
		RETURNING id::text, user_id::text, identity_type, source, qualified_at, granted_at,
		          COALESCE(granted_by::text, ''), COALESCE(grant_reason, ''), revoked_at,
		          COALESCE(revoked_by::text, ''), COALESCE(revoke_reason, ''), created_at, updated_at
	`, cutoff, now)
	if err != nil {
		return 0, internalStoreError()
	}
	items := make([]communityidentity.Identity, 0)
	for rows.Next() {
		item, scanErr := scanCommunityIdentity(rows)
		if scanErr != nil {
			rows.Close()
			return 0, internalStoreError()
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, internalStoreError()
	}
	rows.Close()
	for _, item := range items {
		if appErr := insertCommunityIdentityNotification(ctx, tx, item, communityIdentityGrantRequestID(item), now); appErr != nil {
			return 0, appErr
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, internalStoreError()
	}
	return len(items), nil
}

func insertCommunityIdentity(ctx context.Context, tx pgx.Tx, userID string, identityType communityidentity.IdentityType, source communityidentity.Source, qualifiedAt time.Time, grantedBy, grantReason string, now time.Time) (communityidentity.Identity, bool, *domain.AppError) {
	item, err := scanCommunityIdentity(tx.QueryRow(ctx, `
		INSERT INTO user_community_identities (
			user_id, identity_type, source, qualified_at, granted_at, granted_by, grant_reason, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $5, $5)
		ON CONFLICT (user_id, identity_type) DO NOTHING
		RETURNING id::text, user_id::text, identity_type, source, qualified_at, granted_at,
		          COALESCE(granted_by::text, ''), COALESCE(grant_reason, ''), revoked_at,
		          COALESCE(revoked_by::text, ''), COALESCE(revoke_reason, ''), created_at, updated_at
	`, userID, identityType, source, nullableTime(qualifiedAt), now, nullUUID(grantedBy), nullText(grantReason)))
	if err == nil {
		return item, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		if isForeignKeyViolation(err) {
			return communityidentity.Identity{}, false, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "User not found", "目标用户不存在。")
		}
		return communityidentity.Identity{}, false, internalStoreError()
	}
	existing, err := scanCommunityIdentity(tx.QueryRow(ctx, `
		SELECT id::text, user_id::text, identity_type, source, qualified_at, granted_at,
		       COALESCE(granted_by::text, ''), COALESCE(grant_reason, ''), revoked_at,
		       COALESCE(revoked_by::text, ''), COALESCE(revoke_reason, ''), created_at, updated_at
		FROM user_community_identities
		WHERE user_id = $1 AND identity_type = $2
	`, userID, identityType))
	if err != nil {
		return communityidentity.Identity{}, false, internalStoreError()
	}
	return existing, false, nil
}

func insertCommunityIdentityNotification(ctx context.Context, tx pgx.Tx, item communityidentity.Identity, requestID string, now time.Time) *domain.AppError {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		requestID = communityIdentityGrantRequestID(item)
	}
	actorKind := "system"
	if item.GrantedBy != "" {
		actorKind = "admin"
	}
	metadata, err := json.Marshal(map[string]string{
		"identityType": string(item.Type),
		"source":       string(item.Source),
	})
	if err != nil {
		return internalStoreError()
	}
	eventID := uuid.NewString()
	if _, err := tx.Exec(ctx, `
		INSERT INTO domain_events (
			id, aggregate_type, aggregate_id, event_type, actor_user_id, actor_kind,
			aggregate_version, request_id, metadata_json, created_at
		)
		VALUES ($1, 'community_identity', $2, $3, $4, $5, 1, $6, $7, $8)
	`, eventID, item.ID, communityidentity.NotificationEventType, nullUUID(item.GrantedBy), actorKind, requestID, metadata, now); err != nil {
		return internalStoreError()
	}
	if _, err := tx.Exec(ctx, `
			INSERT INTO notifications (
			user_id, type, title, body, target_type, target_id, target_url,
			source_event_type, source_event_id, dedupe_key, created_at
		)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
			ON CONFLICT (user_id, dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING
		`, item.UserID, communityidentity.NotificationType, "获得社区身份", "你已获得社区身份「"+identityName(item.Type)+"」。该身份记录参与经历，不代表交易信用认证、平台担保或服务能力评价。", "community_identity", item.ID, "/my/profile", communityidentity.NotificationEventType, eventID, "community_identity:"+item.ID, now); err != nil {
		return internalStoreError()
	}
	return nil
}

func communityIdentityGrantRequestID(item communityidentity.Identity) string {
	return "community-identity-" + strings.ToLower(string(item.Source))
}

func scanCommunityIdentity(row scanner) (communityidentity.Identity, error) {
	var item communityidentity.Identity
	var identityType, source string
	err := row.Scan(
		&item.ID, &item.UserID, &identityType, &source, &item.QualifiedAt, &item.GrantedAt,
		&item.GrantedBy, &item.GrantReason, &item.RevokedAt, &item.RevokedBy, &item.RevokeReason,
		&item.CreatedAt, &item.UpdatedAt,
	)
	item.Type = communityidentity.IdentityType(identityType)
	item.Source = communityidentity.Source(source)
	return item, err
}

func identityName(identityType communityidentity.IdentityType) string {
	definition, _ := communityidentity.Definition(identityType)
	return definition.Name
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func communityIdentityNotFoundError() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Community identity not found", "社区身份不存在。")
}

func communityIdentityDuplicateError() *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Community identity already revoked", "该社区身份已经撤销。")
}
