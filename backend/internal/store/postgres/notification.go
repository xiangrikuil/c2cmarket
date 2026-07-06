package postgres

import (
	"context"
	"errors"
	"net/http"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/notification"

	"github.com/jackc/pgx/v5"
)

func (s *Store) ListNotifications(ctx context.Context, userID string, page domain.PageRequest) (domain.Page[notification.Notification], *domain.AppError) {
	if s == nil || s.pool == nil {
		return domain.Page[notification.Notification]{}, internalStoreError()
	}
	page = normalizePageRequest(page)
	position, appErr := decodeKeysetCursor(page.Cursor)
	if appErr != nil {
		return domain.Page[notification.Notification]{}, appErr
	}
	limit := page.Limit + 1
	var rows pgx.Rows
	var err error
	if page.Cursor == "" {
		rows, err = s.pool.Query(ctx, notificationSelectSQL+`
			WHERE user_id = $1
			ORDER BY created_at DESC, id DESC
			LIMIT $2
		`, userID, limit)
	} else {
		rows, err = s.pool.Query(ctx, notificationSelectSQL+`
			WHERE user_id = $1
			  AND (created_at, id) < ($2, $3::uuid)
			ORDER BY created_at DESC, id DESC
			LIMIT $4
		`, userID, position.Time, position.ID, limit)
	}
	if err != nil {
		return domain.Page[notification.Notification]{}, internalStoreError()
	}
	defer rows.Close()
	items, appErr := scanNotifications(rows)
	if appErr != nil {
		return domain.Page[notification.Notification]{}, appErr
	}
	return pageFromItems(items, page, func(item notification.Notification) (time.Time, string) { return item.CreatedAt, item.ID }), nil
}

func (s *Store) UnreadNotificationCount(ctx context.Context, userID string) (int, *domain.AppError) {
	if s == nil || s.pool == nil {
		return 0, internalStoreError()
	}
	var count int
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*)::int
		FROM notifications
		WHERE user_id = $1 AND read_at IS NULL
	`, userID).Scan(&count); err != nil {
		return 0, internalStoreError()
	}
	return count, nil
}

func (s *Store) MarkNotificationRead(ctx context.Context, userID, notificationID string, now time.Time) (notification.Notification, *domain.AppError) {
	if s == nil || s.pool == nil {
		return notification.Notification{}, internalStoreError()
	}
	item, err := scanNotification(ctx, s.pool, notificationSelectSQL+`
		WHERE id = $1 AND user_id = $2
	`, notificationID, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return notification.Notification{}, notificationNotFound()
	}
	if err != nil {
		return notification.Notification{}, internalStoreError()
	}
	if item.ReadAt != nil {
		return item, nil
	}
	updated, err := scanNotification(ctx, s.pool, `
		UPDATE notifications
		SET read_at = $3
		WHERE id = $1 AND user_id = $2
		RETURNING
			id::text, user_id::text, type, title, body, target_type, target_id::text,
			target_url, source_event_type, COALESCE(source_event_id::text, ''),
			read_at, created_at
	`, notificationID, userID, now)
	if errors.Is(err, pgx.ErrNoRows) {
		return notification.Notification{}, notificationNotFound()
	}
	if err != nil {
		return notification.Notification{}, internalStoreError()
	}
	return updated, nil
}

func (s *Store) MarkAllNotificationsRead(ctx context.Context, userID string, now time.Time) (notification.ReadAllResult, *domain.AppError) {
	if s == nil || s.pool == nil {
		return notification.ReadAllResult{}, internalStoreError()
	}
	var count int
	if err := s.pool.QueryRow(ctx, `
		WITH updated AS (
			UPDATE notifications
			SET read_at = $2
			WHERE user_id = $1 AND read_at IS NULL
			RETURNING id
		)
		SELECT count(*)::int FROM updated
	`, userID, now).Scan(&count); err != nil {
		return notification.ReadAllResult{}, internalStoreError()
	}
	items, appErr := s.listAllNotifications(ctx, userID)
	if appErr != nil {
		return notification.ReadAllResult{}, appErr
	}
	return notification.ReadAllResult{Count: count, Items: items}, nil
}

func (s *Store) listAllNotifications(ctx context.Context, userID string) ([]notification.Notification, *domain.AppError) {
	rows, err := s.pool.Query(ctx, notificationSelectSQL+`
		WHERE user_id = $1
		ORDER BY created_at DESC, id DESC
	`, userID)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return scanNotifications(rows)
}

func scanNotifications(rows pgx.Rows) ([]notification.Notification, *domain.AppError) {
	items := []notification.Notification{}
	for rows.Next() {
		item, err := scanNotificationRow(rows)
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

func scanNotification(ctx context.Context, q queryer, sql string, args ...any) (notification.Notification, error) {
	return scanNotificationRow(q.QueryRow(ctx, sql, args...))
}

func scanNotificationRow(row scanner) (notification.Notification, error) {
	var item notification.Notification
	err := row.Scan(
		&item.ID,
		&item.UserID,
		&item.Type,
		&item.Title,
		&item.Body,
		&item.TargetType,
		&item.TargetID,
		&item.TargetURL,
		&item.SourceEventType,
		&item.SourceEventID,
		&item.ReadAt,
		&item.CreatedAt,
	)
	return item, err
}

func notificationNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Notification not found", "通知不存在。")
}

const notificationSelectSQL = `
SELECT
	id::text, user_id::text, type, title, body, target_type, target_id::text,
	target_url, source_event_type, COALESCE(source_event_id::text, ''),
	read_at, created_at
FROM notifications`
