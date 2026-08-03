package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const adminUserDirectoryWhereSQL = `
WHERE ($1 = '' OR lower(u.username || ' ' || u.display_name) LIKE '%' || lower($1) || '%')
  AND ($2 = 'all' OR u.account_status = $2)
  AND (
    $3 = 'all'
    OR ($3 = 'admin' AND EXISTS (
      SELECT 1 FROM user_permissions permission
      WHERE permission.user_id = u.id AND permission.permission = 'admin'
    ))
    OR ($3 = 'user' AND NOT EXISTS (
      SELECT 1 FROM user_permissions permission
      WHERE permission.user_id = u.id AND permission.permission = 'admin'
    ))
  )
  AND (
    $4 = 'all'
    OR ($4 = 'bound' AND binding.user_id IS NOT NULL)
    OR ($4 = 'unbound' AND binding.user_id IS NULL)
  )`

func (s *Store) ListAdminUsers(ctx context.Context, query auth.AdminUserDirectoryQuery) (auth.AdminUserDirectory, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.AdminUserDirectory{}, internalStoreError()
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return auth.AdminUserDirectory{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	args := []any{query.Search, query.Status, query.Role, query.LinuxDo}
	var totalItems int
	err = tx.QueryRow(ctx, `
		SELECT count(*)::int
		FROM users u
		LEFT JOIN linux_do_bindings binding ON binding.user_id = u.id
		`+adminUserDirectoryWhereSQL, args...).Scan(&totalItems)
	if err != nil {
		return auth.AdminUserDirectory{}, internalStoreError()
	}

	var summary auth.AdminUserDirectorySummary
	err = tx.QueryRow(ctx, `
		SELECT count(*)::int,
		       count(*) FILTER (WHERE EXISTS (
		         SELECT 1 FROM user_permissions permission
		         WHERE permission.user_id = u.id AND permission.permission = 'admin'
		       ))::int,
		       count(*) FILTER (WHERE binding.user_id IS NOT NULL)::int,
		       count(*) FILTER (WHERE u.account_status = 'active')::int,
		       count(*) FILTER (WHERE u.account_status = 'suspended')::int,
		       count(*) FILTER (WHERE u.account_status = 'banned')::int,
		       count(*) FILTER (WHERE u.account_status = 'archived')::int
		FROM users u
		LEFT JOIN linux_do_bindings binding ON binding.user_id = u.id
	`).Scan(
		&summary.TotalUsers,
		&summary.AdminUsers,
		&summary.LinuxDoBoundUsers,
		&summary.ActiveUsers,
		&summary.SuspendedUsers,
		&summary.BannedUsers,
		&summary.ArchivedUsers,
	)
	if err != nil {
		return auth.AdminUserDirectory{}, internalStoreError()
	}

	rows, err := tx.Query(ctx, `
		SELECT u.id::text,
		       u.username,
		       u.display_name,
		       u.account_status,
		       EXISTS (
		         SELECT 1 FROM user_permissions permission
		         WHERE permission.user_id = u.id AND permission.permission = 'admin'
		       ) AS is_admin,
		       binding.user_id IS NOT NULL AS linux_do_bound,
		       binding.trust_level,
		       u.created_at,
		       u.updated_at,
		       u.last_active_at,
		       u.version
		FROM users u
		LEFT JOIN linux_do_bindings binding ON binding.user_id = u.id
		`+adminUserDirectoryWhereSQL+`
		ORDER BY `+adminUserDirectoryOrderBy(query.Sort)+`
		LIMIT $5 OFFSET $6
	`, query.Search, query.Status, query.Role, query.LinuxDo, query.Limit, (query.Page-1)*query.Limit)
	if err != nil {
		return auth.AdminUserDirectory{}, internalStoreError()
	}
	defer rows.Close()
	items := []auth.AdminUser{}
	for rows.Next() {
		var item auth.AdminUser
		if err := rows.Scan(
			&item.ID,
			&item.Username,
			&item.DisplayName,
			&item.Status,
			&item.IsAdmin,
			&item.LinuxDoBound,
			&item.TrustLevel,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.LastActiveAt,
			&item.Version,
		); err != nil {
			return auth.AdminUserDirectory{}, internalStoreError()
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return auth.AdminUserDirectory{}, internalStoreError()
	}
	totalPages := 0
	if totalItems > 0 {
		totalPages = (totalItems + query.Limit - 1) / query.Limit
	}
	directory := auth.AdminUserDirectory{
		Items: items,
		Pagination: auth.AdminUserPagination{
			Page:       query.Page,
			Limit:      query.Limit,
			TotalItems: totalItems,
			TotalPages: totalPages,
		},
		Summary: summary,
	}
	if err := tx.Commit(ctx); err != nil {
		return auth.AdminUserDirectory{}, internalStoreError()
	}
	return directory, nil
}

func adminUserDirectoryOrderBy(value string) string {
	switch value {
	case auth.AdminUserSortCreatedAsc:
		return "u.created_at ASC, u.id ASC"
	case auth.AdminUserSortActiveDesc:
		return "u.last_active_at DESC NULLS LAST, u.id DESC"
	case auth.AdminUserSortUsernameAsc:
		return "u.username ASC, u.id ASC"
	case auth.AdminUserSortUsernameDesc:
		return "u.username DESC, u.id DESC"
	default:
		return "u.created_at DESC, u.id DESC"
	}
}

func (s *Store) AdminUserDetail(ctx context.Context, userID string) (auth.AdminUserDetail, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.AdminUserDetail{}, internalStoreError()
	}
	return loadAdminUserDetail(ctx, s.pool, userID, time.Now())
}

func loadAdminUserDetail(ctx context.Context, q queryer, userID string, now time.Time) (auth.AdminUserDetail, *domain.AppError) {
	var detail auth.AdminUserDetail
	var linuxDoUsername *string
	var linuxDoTrustLevel *int
	var linuxDoBoundAt *time.Time
	var linuxDoLastSyncedAt *time.Time
	err := q.QueryRow(ctx, `
		SELECT u.id::text,
		       u.username,
		       u.display_name,
		       u.account_status,
		       EXISTS (
		         SELECT 1 FROM user_permissions permission
		         WHERE permission.user_id = u.id AND permission.permission = 'admin'
		       ) AS is_admin,
		       binding.user_id IS NOT NULL AS linux_do_bound,
		       binding.trust_level,
		       u.created_at,
		       u.updated_at,
		       u.last_active_at,
		       u.version,
		       binding.linux_do_username,
		       binding.trust_level,
		       binding.bound_at,
		       binding.last_synced_at,
		       u.email_verified_at IS NOT NULL AS email_verified,
		       EXISTS (
		         SELECT 1 FROM user_password_credentials credential
		         WHERE credential.user_id = u.id
		       ) AS backup_password_configured,
		       (
		         SELECT count(*)::int
		         FROM auth_sessions session
		         WHERE session.user_id = u.id
		           AND session.revoked_at IS NULL
		           AND session.expires_at > $2
		           AND session.absolute_expires_at > $2
		       ) AS active_session_count,
		       (
		         SELECT max(COALESCE(session.last_seen_at, session.renewed_at, session.created_at))
		         FROM auth_sessions session
		         WHERE session.user_id = u.id
		           AND session.revoked_at IS NULL
		           AND session.expires_at > $2
		           AND session.absolute_expires_at > $2
		       ) AS latest_session_activity_at,
		       (
		         SELECT count(*)::int
		         FROM users admin_user
		         WHERE admin_user.account_status = 'active'
		           AND EXISTS (
		             SELECT 1 FROM user_permissions permission
		             WHERE permission.user_id = admin_user.id AND permission.permission = 'admin'
		           )
		       ) AS active_admin_count,
		       (
		         SELECT count(*)::int FROM carpool_listings listing
		         WHERE listing.owner_user_id = u.id AND listing.status = 'active'
		       ) AS active_carpool_listings,
		       (
		         SELECT count(*)::int FROM api_services service
		         WHERE service.owner_user_id = u.id AND service.publication_status = 'online'
		       ) AS online_api_services,
		       (
		         SELECT count(*)::int FROM carpool_applications application
		         WHERE (application.buyer_user_id = u.id OR application.owner_user_id = u.id)
		           AND application.status IN ('pending_owner', 'accepted_reserved')
		       ) AS open_carpool_applications,
		       (
		         SELECT count(*)::int FROM api_orders order_row
		         WHERE (order_row.buyer_user_id = u.id OR order_row.seller_user_id = u.id)
		           AND (order_row.status NOT IN ('completed', 'cancelled') OR order_row.dispute_status = 'open')
		       ) AS open_api_orders,
		       (
		         SELECT count(*)::int FROM dispute_cases dispute
		         WHERE (dispute.primary_user_id = u.id OR dispute.counterparty_user_id = u.id)
		           AND dispute.status IN ('open', 'waiting_info')
		       ) AS open_disputes
		FROM users u
		LEFT JOIN linux_do_bindings binding ON binding.user_id = u.id
		WHERE u.id = $1
	`, userID, now).Scan(
		&detail.User.ID,
		&detail.User.Username,
		&detail.User.DisplayName,
		&detail.User.Status,
		&detail.User.IsAdmin,
		&detail.User.LinuxDoBound,
		&detail.User.TrustLevel,
		&detail.User.CreatedAt,
		&detail.User.UpdatedAt,
		&detail.User.LastActiveAt,
		&detail.User.Version,
		&linuxDoUsername,
		&linuxDoTrustLevel,
		&linuxDoBoundAt,
		&linuxDoLastSyncedAt,
		&detail.EmailVerified,
		&detail.BackupPasswordConfigured,
		&detail.ActiveSessionCount,
		&detail.LatestSessionActivityAt,
		&detail.ActiveAdminCount,
		&detail.ImpactPreview.ActiveCarpoolListings,
		&detail.ImpactPreview.OnlineAPIServices,
		&detail.ImpactPreview.OpenCarpoolApplications,
		&detail.ImpactPreview.OpenAPIOrders,
		&detail.ImpactPreview.OpenDisputes,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.AdminUserDetail{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "User not found", "用户不存在。")
	}
	if err != nil {
		return auth.AdminUserDetail{}, internalStoreError()
	}
	detail.ImpactPreview.ActiveSessions = detail.ActiveSessionCount
	detail.LinuxDoBinding.Bound = detail.User.LinuxDoBound
	if linuxDoUsername != nil {
		detail.LinuxDoBinding.Username = *linuxDoUsername
	}
	if linuxDoTrustLevel != nil {
		detail.LinuxDoBinding.TrustLevel = *linuxDoTrustLevel
	}
	detail.LinuxDoBinding.BoundAt = linuxDoBoundAt
	detail.LinuxDoBinding.LastSyncedAt = linuxDoLastSyncedAt

	providerRows, err := queryRows(ctx, q, `
		SELECT provider, created_at, last_login_at
		FROM auth_identities
		WHERE user_id = $1
		ORDER BY created_at ASC, provider ASC
	`, userID)
	if err != nil {
		return auth.AdminUserDetail{}, internalStoreError()
	}
	detail.Providers = []auth.AdminAuthProvider{}
	for providerRows.Next() {
		var provider auth.AdminAuthProvider
		if err := providerRows.Scan(&provider.Provider, &provider.CreatedAt, &provider.LastLoginAt); err != nil {
			providerRows.Close()
			return auth.AdminUserDetail{}, internalStoreError()
		}
		detail.Providers = append(detail.Providers, provider)
	}
	if err := providerRows.Err(); err != nil {
		providerRows.Close()
		return auth.AdminUserDetail{}, internalStoreError()
	}
	providerRows.Close()

	auditRows, err := queryRows(ctx, q, `
		SELECT audit.id::text,
		       audit.admin_user_id::text,
		       admin.username,
		       audit.action,
		       COALESCE(audit.reason, ''),
		       audit.before_json,
		       audit.after_json,
		       audit.request_id,
		       audit.created_at
		FROM admin_audit_logs audit
		JOIN users admin ON admin.id = audit.admin_user_id
		WHERE audit.target_type = 'user'
		  AND audit.target_id = $1
		  AND audit.action IN ('user.account_status_changed', 'user.admin_permission_changed')
		ORDER BY audit.created_at DESC, audit.id DESC
		LIMIT 20
	`, userID)
	if err != nil {
		return auth.AdminUserDetail{}, internalStoreError()
	}
	detail.RecentAuditEntries = []auth.AdminAccountAuditEntry{}
	for auditRows.Next() {
		var entry auth.AdminAccountAuditEntry
		var beforeJSON []byte
		var afterJSON []byte
		if err := auditRows.Scan(
			&entry.ID,
			&entry.AdminUserID,
			&entry.AdminUsername,
			&entry.Action,
			&entry.Reason,
			&beforeJSON,
			&afterJSON,
			&entry.RequestID,
			&entry.CreatedAt,
		); err != nil {
			auditRows.Close()
			return auth.AdminUserDetail{}, internalStoreError()
		}
		applyAdminAccountAuditProjection(&entry, beforeJSON, afterJSON)
		detail.RecentAuditEntries = append(detail.RecentAuditEntries, entry)
	}
	if err := auditRows.Err(); err != nil {
		auditRows.Close()
		return auth.AdminUserDetail{}, internalStoreError()
	}
	auditRows.Close()
	return detail, nil
}

type adminAccountAuditProjection struct {
	AccountStatus string `json:"accountStatus"`
	IsAdmin       *bool  `json:"isAdmin"`
}

func applyAdminAccountAuditProjection(entry *auth.AdminAccountAuditEntry, beforeJSON, afterJSON []byte) {
	var before adminAccountAuditProjection
	var after adminAccountAuditProjection
	_ = json.Unmarshal(beforeJSON, &before)
	_ = json.Unmarshal(afterJSON, &after)
	entry.BeforeStatus = before.AccountStatus
	entry.AfterStatus = after.AccountStatus
	entry.BeforeIsAdmin = before.IsAdmin
	entry.AfterIsAdmin = after.IsAdmin
}

func (s *Store) UpdateAdminUserStatusWithIdempotency(ctx context.Context, entry idempotency.Entry, input auth.AdminUserStatusInput, now time.Time, buildCompletion auth.AdminUserCompletionBuilder) (auth.AdminUserMutationResult, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.AdminUserMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return auth.AdminUserMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	existingEntry, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return auth.AdminUserMutationResult{}, idempotency.Completion{}, appErr
	}
	if appErr := lockAccountGovernanceUser(ctx, tx, input.TargetUserID); appErr != nil {
		return auth.AdminUserMutationResult{}, idempotency.Completion{}, appErr
	}
	if appErr := lockAdminUserGovernanceTables(ctx, tx); appErr != nil {
		return auth.AdminUserMutationResult{}, idempotency.Completion{}, appErr
	}
	current, appErr := lockAdminUserForGovernance(ctx, tx, input.TargetUserID)
	if appErr != nil {
		return auth.AdminUserMutationResult{}, idempotency.Completion{}, appErr
	}
	if current.Version != input.ExpectedVersion {
		return auth.AdminUserMutationResult{}, idempotency.Completion{}, adminUserStoreVersionConflict()
	}
	if !auth.AllowedAdminUserStatusTransition(current.Status, input.Status) {
		return auth.AdminUserMutationResult{}, idempotency.Completion{}, adminUserStoreInvalidTransition("当前账号状态不能执行该变更。")
	}
	if current.IsAdmin && current.Status == auth.AccountStatusActive && input.Status != auth.AccountStatusActive {
		activeAdmins, appErr := activeAdminCountInTx(ctx, tx)
		if appErr != nil {
			return auth.AdminUserMutationResult{}, idempotency.Completion{}, appErr
		}
		if activeAdmins <= 1 {
			return auth.AdminUserMutationResult{}, idempotency.Completion{}, adminUserStoreInvalidTransition("不能停用最后一个有效管理员账号。")
		}
	}
	before := adminAccountAuditProjection{AccountStatus: current.Status, IsAdmin: boolPointerForStore(current.IsAdmin)}
	err = tx.QueryRow(ctx, `
		UPDATE users
		SET account_status = $2,
		    updated_at = $3,
		    version = version + 1
		WHERE id = $1
		RETURNING account_status, updated_at, version
	`, current.ID, input.Status, now).Scan(&current.Status, &current.UpdatedAt, &current.Version)
	if err != nil {
		return auth.AdminUserMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	if before.AccountStatus == auth.AccountStatusActive && current.Status != auth.AccountStatusActive {
		if _, err := tx.Exec(ctx, `
			UPDATE auth_sessions
			SET revoked_at = COALESCE(revoked_at, $2),
			    updated_at = $2
			WHERE user_id = $1 AND revoked_at IS NULL
		`, current.ID, now); err != nil {
			return auth.AdminUserMutationResult{}, idempotency.Completion{}, internalStoreError()
		}
	}
	after := adminAccountAuditProjection{AccountStatus: current.Status, IsAdmin: boolPointerForStore(current.IsAdmin)}
	if appErr := insertAdminUserGovernanceSideEffects(ctx, tx, current, input.AdminUserID, "user.account_status_changed", input.Reason, input.RequestID, before, after, now); appErr != nil {
		return auth.AdminUserMutationResult{}, idempotency.Completion{}, appErr
	}
	detail, appErr := loadAdminUserDetail(ctx, tx, current.ID, now)
	if appErr != nil {
		return auth.AdminUserMutationResult{}, idempotency.Completion{}, appErr
	}
	result := auth.AdminUserMutationResult{Detail: detail}
	completion, appErr := buildCompletion(result)
	if appErr != nil {
		return auth.AdminUserMutationResult{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existingEntry, completion, now); appErr != nil {
		return auth.AdminUserMutationResult{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return auth.AdminUserMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	return result, completion, nil
}

func (s *Store) UpdateAdminUserPermissionWithIdempotency(ctx context.Context, entry idempotency.Entry, input auth.AdminUserPermissionInput, now time.Time, buildCompletion auth.AdminUserCompletionBuilder) (auth.AdminUserMutationResult, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return auth.AdminUserMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return auth.AdminUserMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	existingEntry, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return auth.AdminUserMutationResult{}, idempotency.Completion{}, appErr
	}
	if appErr := lockAdminUserGovernanceTables(ctx, tx); appErr != nil {
		return auth.AdminUserMutationResult{}, idempotency.Completion{}, appErr
	}
	current, appErr := lockAdminUserForGovernance(ctx, tx, input.TargetUserID)
	if appErr != nil {
		return auth.AdminUserMutationResult{}, idempotency.Completion{}, appErr
	}
	if current.Version != input.ExpectedVersion {
		return auth.AdminUserMutationResult{}, idempotency.Completion{}, adminUserStoreVersionConflict()
	}
	if current.IsAdmin == input.Grant {
		return auth.AdminUserMutationResult{}, idempotency.Completion{}, adminUserStoreInvalidTransition("账号管理员权限没有变化。")
	}
	if input.Grant && current.Status != auth.AccountStatusActive {
		return auth.AdminUserMutationResult{}, idempotency.Completion{}, adminUserStoreInvalidTransition("只能向有效账号授予管理员权限。")
	}
	if !input.Grant && current.Status == auth.AccountStatusActive {
		activeAdmins, appErr := activeAdminCountInTx(ctx, tx)
		if appErr != nil {
			return auth.AdminUserMutationResult{}, idempotency.Completion{}, appErr
		}
		if activeAdmins <= 1 {
			return auth.AdminUserMutationResult{}, idempotency.Completion{}, adminUserStoreInvalidTransition("不能撤销最后一个有效管理员的权限。")
		}
	}
	before := adminAccountAuditProjection{AccountStatus: current.Status, IsAdmin: boolPointerForStore(current.IsAdmin)}
	if input.Grant {
		_, err = tx.Exec(ctx, `
			INSERT INTO user_permissions (user_id, permission)
			VALUES ($1, 'admin')
		`, current.ID)
	} else {
		_, err = tx.Exec(ctx, `
			DELETE FROM user_permissions
			WHERE user_id = $1 AND permission = 'admin'
		`, current.ID)
	}
	if err != nil {
		return auth.AdminUserMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	current.IsAdmin = input.Grant
	err = tx.QueryRow(ctx, `
		UPDATE users
		SET updated_at = $2,
		    version = version + 1
		WHERE id = $1
		RETURNING updated_at, version
	`, current.ID, now).Scan(&current.UpdatedAt, &current.Version)
	if err != nil {
		return auth.AdminUserMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	after := adminAccountAuditProjection{AccountStatus: current.Status, IsAdmin: boolPointerForStore(current.IsAdmin)}
	if appErr := insertAdminUserGovernanceSideEffects(ctx, tx, current, input.AdminUserID, "user.admin_permission_changed", input.Reason, input.RequestID, before, after, now); appErr != nil {
		return auth.AdminUserMutationResult{}, idempotency.Completion{}, appErr
	}
	detail, appErr := loadAdminUserDetail(ctx, tx, current.ID, now)
	if appErr != nil {
		return auth.AdminUserMutationResult{}, idempotency.Completion{}, appErr
	}
	result := auth.AdminUserMutationResult{Detail: detail}
	completion, appErr := buildCompletion(result)
	if appErr != nil {
		return auth.AdminUserMutationResult{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existingEntry, completion, now); appErr != nil {
		return auth.AdminUserMutationResult{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return auth.AdminUserMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	return result, completion, nil
}

func lockAdminUserGovernanceTables(ctx context.Context, tx pgx.Tx) *domain.AppError {
	if _, err := tx.Exec(ctx, `LOCK TABLE users, user_permissions IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return internalStoreError()
	}
	return nil
}

func lockAdminUserForGovernance(ctx context.Context, tx pgx.Tx, userID string) (auth.AdminUser, *domain.AppError) {
	var item auth.AdminUser
	err := tx.QueryRow(ctx, `
		SELECT u.id::text,
		       u.username,
		       u.display_name,
		       u.account_status,
		       EXISTS (
		         SELECT 1 FROM user_permissions permission
		         WHERE permission.user_id = u.id AND permission.permission = 'admin'
		       ) AS is_admin,
		       u.created_at,
		       u.updated_at,
		       u.last_active_at,
		       u.version
		FROM users u
		WHERE u.id = $1
		FOR UPDATE
	`, userID).Scan(
		&item.ID,
		&item.Username,
		&item.DisplayName,
		&item.Status,
		&item.IsAdmin,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.LastActiveAt,
		&item.Version,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.AdminUser{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "User not found", "用户不存在。")
	}
	if err != nil {
		return auth.AdminUser{}, internalStoreError()
	}
	return item, nil
}

func activeAdminCountInTx(ctx context.Context, tx pgx.Tx) (int, *domain.AppError) {
	var count int
	if err := tx.QueryRow(ctx, `
		SELECT count(*)::int
		FROM users user_account
		JOIN user_permissions permission
		  ON permission.user_id = user_account.id
		 AND permission.permission = 'admin'
		WHERE user_account.account_status = 'active'
	`).Scan(&count); err != nil {
		return 0, internalStoreError()
	}
	return count, nil
}

func insertAdminUserGovernanceSideEffects(ctx context.Context, tx pgx.Tx, user auth.AdminUser, adminUserID, eventType, reason, requestID string, before, after adminAccountAuditProjection, now time.Time) *domain.AppError {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		requestID = "unknown"
	}
	beforeJSON, err := json.Marshal(before)
	if err != nil {
		return internalStoreError()
	}
	afterJSON, err := json.Marshal(after)
	if err != nil {
		return internalStoreError()
	}
	metadata, err := json.Marshal(map[string]any{
		"accountStatus": user.Status,
		"isAdmin":       user.IsAdmin,
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
		VALUES ($1, 'user', $2, $3, $4, 'admin', $5, $6, $7, $8)
	`, eventID, user.ID, eventType, adminUserID, user.Version, requestID, metadata, now); err != nil {
		return internalStoreError()
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO admin_audit_logs (
			admin_user_id, action, target_type, target_id, reason, before_json,
			after_json, request_id, created_at
		)
		VALUES ($1, $2, 'user', $3, $4, $5, $6, $7, $8)
	`, adminUserID, eventType, user.ID, reason, beforeJSON, afterJSON, requestID, now); err != nil {
		return internalStoreError()
	}
	title := "账号状态已更新"
	body := "管理员已更新你的账号状态，请在账号页面查看当前状态。"
	if eventType == "user.admin_permission_changed" {
		title = "管理员权限已更新"
		body = "管理员已更新你的管理员权限，请重新进入管理台确认当前权限。"
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO notifications (
			user_id, type, title, body, target_type, target_id, target_url,
			source_event_type, source_event_id, dedupe_key, created_at
		)
		VALUES ($1, $2, $3, $4, 'user', $1, '/my/profile', $2, $5, $6, $7)
		ON CONFLICT (user_id, dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING
	`, user.ID, eventType, title, body, eventID, "user:"+user.ID+":"+eventType+":"+strconv.FormatInt(user.Version, 10), now); err != nil {
		return internalStoreError()
	}
	return nil
}

func boolPointerForStore(value bool) *bool {
	return &value
}

func adminUserStoreVersionConflict() *domain.AppError {
	return domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "账号信息已更新，请刷新后重试。")
}

func adminUserStoreInvalidTransition(detail string) *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", detail)
}
