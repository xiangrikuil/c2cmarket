package postgres

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
)

func (s *Store) ListAdminAuditLogs(ctx context.Context, filter auth.AdminAuditLogFilter, page domain.PageRequest) (domain.Page[auth.AdminAuditLog], *domain.AppError) {
	if s == nil || s.pool == nil {
		return domain.Page[auth.AdminAuditLog]{}, internalStoreError()
	}
	page = normalizePageRequest(page)
	position, appErr := decodeKeysetCursor(page.Cursor)
	if appErr != nil {
		return domain.Page[auth.AdminAuditLog]{}, appErr
	}

	conditions := []string{}
	args := []any{}
	addArgument := func(value any) string {
		args = append(args, value)
		return "$" + strconv.Itoa(len(args))
	}
	if value := strings.TrimSpace(filter.Action); value != "" {
		conditions = append(conditions, "audit.action = "+addArgument(value))
	}
	if value := strings.TrimSpace(filter.TargetType); value != "" {
		conditions = append(conditions, "audit.target_type = "+addArgument(value))
	}
	if value := strings.TrimSpace(filter.ActorUserID); value != "" {
		conditions = append(conditions, "audit.admin_user_id = "+addArgument(value)+"::uuid")
	}
	if value := strings.TrimSpace(filter.TargetID); value != "" {
		conditions = append(conditions, "audit.target_id = "+addArgument(value)+"::uuid")
	}
	if value := strings.ToLower(strings.TrimSpace(filter.Search)); value != "" {
		placeholder := addArgument(value)
		conditions = append(conditions, `(
			strpos(lower(audit.id::text), `+placeholder+`) > 0 OR
			strpos(lower(audit.action), `+placeholder+`) > 0 OR
			strpos(lower(audit.target_type), `+placeholder+`) > 0 OR
			strpos(lower(audit.target_id::text), `+placeholder+`) > 0 OR
			strpos(lower(admin.username), `+placeholder+`) > 0 OR
			strpos(lower(COALESCE(audit.reason, '')), `+placeholder+`) > 0 OR
			strpos(lower(audit.request_id), `+placeholder+`) > 0
		)`)
	}
	if page.Cursor != "" {
		timePlaceholder := addArgument(position.Time)
		idPlaceholder := addArgument(position.ID)
		conditions = append(conditions, "(audit.created_at, audit.id) < ("+timePlaceholder+", "+idPlaceholder+"::uuid)")
	}

	query := `
		SELECT audit.id::text,
		       audit.admin_user_id::text,
		       admin.username,
		       audit.action,
		       audit.target_type,
		       audit.target_id::text,
		       COALESCE(audit.reason, ''),
		       audit.request_id,
		       audit.before_json,
		       audit.after_json,
		       audit.created_at
		FROM admin_audit_logs audit
		JOIN users admin ON admin.id = audit.admin_user_id`
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY audit.created_at DESC, audit.id DESC"
	query += " LIMIT " + addArgument(page.Limit+1)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return domain.Page[auth.AdminAuditLog]{}, internalStoreError()
	}
	defer rows.Close()
	items := make([]auth.AdminAuditLog, 0, page.Limit+1)
	for rows.Next() {
		var item auth.AdminAuditLog
		var beforeJSON []byte
		var afterJSON []byte
		if err := rows.Scan(
			&item.ID,
			&item.ActorUserID,
			&item.ActorUsername,
			&item.Action,
			&item.TargetType,
			&item.TargetID,
			&item.Reason,
			&item.RequestID,
			&beforeJSON,
			&afterJSON,
			&item.CreatedAt,
		); err != nil {
			return domain.Page[auth.AdminAuditLog]{}, internalStoreError()
		}
		item.BeforeStatus = adminAuditStatus(beforeJSON)
		item.AfterStatus = adminAuditStatus(afterJSON)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return domain.Page[auth.AdminAuditLog]{}, internalStoreError()
	}
	return pageFromItems(items, page, func(item auth.AdminAuditLog) (time.Time, string) {
		return item.CreatedAt, item.ID
	}), nil
}

func adminAuditStatus(body []byte) *string {
	if len(body) == 0 {
		return nil
	}
	var values map[string]any
	if err := json.Unmarshal(body, &values); err != nil {
		return nil
	}
	for _, key := range []string{"status", "accountStatus", "reviewStatus", "publicationStatus"} {
		value, ok := values[key].(string)
		value = strings.TrimSpace(value)
		if ok && value != "" {
			return &value
		}
	}
	return nil
}
