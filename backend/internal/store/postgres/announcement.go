package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/announcement"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (s *Store) UserAnnouncements(ctx context.Context, userID string, now time.Time) ([]announcement.Announcement, *domain.AppError) {
	items, appErr := s.queryAnnouncements(ctx, userID, `
		SELECT `+announcementColumns+`
		FROM announcements a
		LEFT JOIN announcement_receipts r ON r.announcement_id = a.id AND r.user_id = $1
		WHERE a.audience_json->>'type' = 'all'
		   OR EXISTS (
		     SELECT 1 FROM announcement_recipients recipient
		     WHERE recipient.announcement_id = a.id
		       AND recipient.user_id = $1
		       AND recipient.announcement_version = a.version
		   )
		ORDER BY a.publish_at DESC
	`, userID)
	if appErr != nil {
		return nil, appErr
	}
	return filterUserVisible(items, now, ""), nil
}

func (s *Store) PublicActiveAnnouncements(ctx context.Context, channel string, now time.Time) ([]announcement.Announcement, *domain.AppError) {
	items, appErr := s.queryAnnouncements(ctx, "", `
		SELECT `+announcementColumns+`
		FROM announcements a
		LEFT JOIN announcement_receipts r ON false
		WHERE a.audience_json->>'type' = 'all'
		  AND array_position(a.channels, $1) IS NOT NULL
		ORDER BY a.publish_at DESC
	`, channel)
	if appErr != nil {
		return nil, appErr
	}
	result := make([]announcement.Announcement, 0, len(items))
	for _, item := range items {
		if announcement.DisplayStatus(item, now) == announcement.StatusPublished {
			item.Receipt = nil
			result = append(result, item)
		}
	}
	announcement.SortByDeliveryPriority(result)
	return result, nil
}

func (s *Store) ActiveAnnouncements(ctx context.Context, userID, channel string, now time.Time) ([]announcement.Announcement, *domain.AppError) {
	items, appErr := s.UserAnnouncements(ctx, userID, now)
	if appErr != nil {
		return nil, appErr
	}
	result := make([]announcement.Announcement, 0, len(items))
	for _, item := range items {
		if announcement.DisplayStatus(item, now) == announcement.StatusPublished &&
			(strings.TrimSpace(channel) == "" || announcement.HasChannel(item, channel)) {
			result = append(result, item)
		}
	}
	announcement.SortByDeliveryPriority(result)
	return result, nil
}

func (s *Store) HomeAnnouncement(ctx context.Context, userID string, now time.Time) (*announcement.Announcement, *domain.AppError) {
	items, appErr := s.queryAnnouncements(ctx, userID, `
		SELECT `+announcementColumns+`
		FROM announcements a
		LEFT JOIN announcement_receipts r ON r.announcement_id = a.id AND r.user_id = $1
		WHERE array_position(a.channels, 'home_banner') IS NOT NULL
		  AND (
		    a.audience_json->>'type' = 'all'
		    OR EXISTS (
		      SELECT 1 FROM announcement_recipients recipient
		      WHERE recipient.announcement_id = a.id AND recipient.user_id = $1
		        AND recipient.announcement_version = a.version
		    )
		  )
		ORDER BY a.publish_at DESC
	`, userID)
	if appErr != nil {
		return nil, appErr
	}
	candidates := make([]announcement.Announcement, 0, len(items))
	for _, item := range items {
		if announcement.DisplayStatus(item, now) == announcement.StatusPublished && !announcement.IsDismissed(item) {
			candidates = append(candidates, item)
		}
	}
	announcement.SortForHome(candidates)
	if len(candidates) == 0 {
		return nil, nil
	}
	return &candidates[0], nil
}

func (s *Store) UserAnnouncementBySlug(ctx context.Context, userID, slug string, now time.Time) (announcement.Announcement, *domain.AppError) {
	item, err := s.scanAnnouncement(ctx, s.pool, `
		SELECT `+announcementColumns+`
		FROM announcements a
		LEFT JOIN announcement_receipts r ON r.announcement_id = a.id AND r.user_id = $1
		WHERE a.slug = $2
		  AND (
		    a.audience_json->>'type' = 'all'
		    OR EXISTS (
		      SELECT 1 FROM announcement_recipients recipient
		      WHERE recipient.announcement_id = a.id AND recipient.user_id = $1
		        AND recipient.announcement_version = a.version
		    )
		  )
	`, userID, strings.TrimSpace(strings.ToLower(slug)))
	if errors.Is(err, pgx.ErrNoRows) || !announcement.IsUserVisible(item, now) {
		return announcement.Announcement{}, announcementNotFound()
	}
	if err != nil {
		return announcement.Announcement{}, internalStoreError()
	}
	return item, nil
}

func (s *Store) AnnouncementUnreadCount(ctx context.Context, userID string, importantOnly bool, now time.Time) (int, *domain.AppError) {
	items, appErr := s.UserAnnouncements(ctx, userID, now)
	if appErr != nil {
		return 0, appErr
	}
	count := 0
	for _, item := range items {
		if importantOnly && item.Level != announcement.LevelImportant && item.Level != announcement.LevelCritical {
			continue
		}
		if announcement.IsUnread(item) {
			count++
		}
	}
	return count, nil
}

func (s *Store) UpsertReceipt(ctx context.Context, input announcement.ReceiptInput, now time.Time) (announcement.Receipt, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return announcement.Receipt{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	item, err := s.scanAnnouncement(ctx, tx, `
		SELECT `+announcementColumns+`
		FROM announcements a
		LEFT JOIN announcement_receipts r ON r.announcement_id = a.id AND r.user_id = $1
		WHERE a.id = $2
		  AND (
		    a.audience_json->>'type' = 'all'
		    OR EXISTS (
		      SELECT 1 FROM announcement_recipients recipient
		      WHERE recipient.announcement_id = a.id AND recipient.user_id = $1
		        AND recipient.announcement_version = a.version
		    )
		  )
		FOR UPDATE OF a
	`, input.UserID, input.AnnouncementID)
	if errors.Is(err, pgx.ErrNoRows) || !announcement.IsUserVisible(item, now) {
		return announcement.Receipt{}, announcementNotFound()
	}
	if err != nil {
		return announcement.Receipt{}, internalStoreError()
	}

	firstSeenExpr := "COALESCE(announcement_receipts.first_seen_at, EXCLUDED.first_seen_at)"
	readExpr := "announcement_receipts.read_at"
	dismissedExpr := "announcement_receipts.dismissed_at"
	acknowledgedExpr := "announcement_receipts.acknowledged_at"
	switch input.Action {
	case "seen":
	case "read":
		readExpr = "EXCLUDED.read_at"
	case "dismiss":
		if !item.IsDismissible {
			return announcement.Receipt{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Announcement cannot be dismissed", "该公告不允许关闭。")
		}
		dismissedExpr = "EXCLUDED.dismissed_at"
	case "acknowledge":
		if item.Level != announcement.LevelCritical || !item.RequiresAck {
			return announcement.Receipt{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Announcement acknowledgement not required", "该公告不需要确认知悉。")
		}
		acknowledgedExpr = "EXCLUDED.acknowledged_at"
	default:
		return announcement.Receipt{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid announcement receipt action", "公告 receipt 动作不支持。")
	}

	var receipt announcement.Receipt
	err = tx.QueryRow(ctx, `
			INSERT INTO announcement_receipts (
			  announcement_id, user_id, announcement_version, first_seen_at, read_at, dismissed_at, acknowledged_at, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (announcement_id, user_id) DO UPDATE
		SET announcement_version = EXCLUDED.announcement_version,
		    first_seen_at = CASE
		      WHEN announcement_receipts.announcement_version = EXCLUDED.announcement_version THEN `+firstSeenExpr+`
		      ELSE EXCLUDED.first_seen_at
		    END,
		    read_at = CASE
		      WHEN announcement_receipts.announcement_version = EXCLUDED.announcement_version THEN `+readExpr+`
		      ELSE EXCLUDED.read_at
		    END,
		    dismissed_at = CASE
		      WHEN announcement_receipts.announcement_version = EXCLUDED.announcement_version THEN `+dismissedExpr+`
		      ELSE EXCLUDED.dismissed_at
		    END,
		    acknowledged_at = CASE
		      WHEN announcement_receipts.announcement_version = EXCLUDED.announcement_version THEN `+acknowledgedExpr+`
		      ELSE EXCLUDED.acknowledged_at
		    END,
		    updated_at = EXCLUDED.updated_at
		RETURNING announcement_id::text, announcement_version, first_seen_at, read_at, dismissed_at, acknowledged_at
	`, input.AnnouncementID, input.UserID, item.Version, nullableFirstSeenTime(input.Action, now), nullableActionTime(input.Action, "read", now), nullableActionTime(input.Action, "dismiss", now), nullableActionTime(input.Action, "acknowledge", now), now).Scan(
		&receipt.AnnouncementID,
		&receipt.AnnouncementVersion,
		&receipt.FirstSeenAt,
		&receipt.ReadAt,
		&receipt.DismissedAt,
		&receipt.AcknowledgedAt,
	)
	if err != nil {
		return announcement.Receipt{}, internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return announcement.Receipt{}, internalStoreError()
	}
	return receipt, nil
}

func (s *Store) AdminAnnouncements(ctx context.Context, now time.Time) ([]announcement.Announcement, *domain.AppError) {
	return s.queryAnnouncements(ctx, "", `
		SELECT `+announcementColumns+`
		FROM announcements a
		LEFT JOIN announcement_receipts r ON false
		ORDER BY a.publish_at DESC
	`)
}

func (s *Store) AdminAnnouncementByID(ctx context.Context, id string, now time.Time) (announcement.Announcement, *domain.AppError) {
	item, err := s.scanAnnouncement(ctx, s.pool, `
		SELECT `+announcementColumns+`
		FROM announcements a
		LEFT JOIN announcement_receipts r ON false
		WHERE a.id = $1
	`, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return announcement.Announcement{}, announcementNotFound()
	}
	if err != nil {
		return announcement.Announcement{}, internalStoreError()
	}
	return item, nil
}

func (s *Store) CreateAnnouncement(ctx context.Context, input announcement.CreateInput, now time.Time) (announcement.Announcement, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return announcement.Announcement{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	item, appErr := s.insertAnnouncement(ctx, tx, input.Form, input.OperatorID, input.OperatorID, announcement.StatusDraft, uniqueSlugBase(input.Form.Title), now)
	if appErr != nil {
		return announcement.Announcement{}, appErr
	}
	if appErr := insertAnnouncementAudit(ctx, tx, announcement.AuditCreated, item, input.OperatorID, input.OperatorName, "创建公告草稿", now); appErr != nil {
		return announcement.Announcement{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return announcement.Announcement{}, internalStoreError()
	}
	return item, nil
}

func (s *Store) UpdateAnnouncement(ctx context.Context, input announcement.UpdateInput, now time.Time) (announcement.Announcement, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return announcement.Announcement{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	before, err := s.scanAnnouncement(ctx, tx, `
		SELECT `+announcementColumns+`
		FROM announcements a
		LEFT JOIN announcement_receipts r ON false
		WHERE a.id = $1
		FOR UPDATE OF a
	`, input.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return announcement.Announcement{}, announcementNotFound()
	}
	if err != nil {
		return announcement.Announcement{}, internalStoreError()
	}
	contentUpdatedAt := before.ContentUpdatedAt
	if announcement.UserVisibleContentChanged(before, input.Form) {
		contentUpdatedAt = now
	}
	deliveryChanged := announcement.DeliveryRevisionChanged(before, input.Form)
	audienceJSON, err := json.Marshal(input.Form.Audience)
	if err != nil {
		return announcement.Announcement{}, internalStoreError()
	}
	item, err := s.scanAnnouncement(ctx, tx, `
		UPDATE announcements
		SET title = $2, summary = $3, content_markdown = $4, category = $5, level = $6,
		    channels = $7, audience_json = $8, is_pinned = $9, is_dismissible = $10, requires_ack = $11, cta_label = $12, cta_url = $13,
		    publish_at = $14, expire_at = $15, updated_by_user_id = $16, updated_at = $17,
		    content_updated_at = $18, version = version + CASE WHEN $19 THEN 1 ELSE 0 END
		WHERE id = $1
		RETURNING `+announcementReturningColumns+`
	`, input.ID, strings.TrimSpace(input.Form.Title), strings.TrimSpace(input.Form.Summary), strings.TrimSpace(input.Form.ContentMarkdown),
		input.Form.Category, input.Form.Level, input.Form.Channels, audienceJSON, input.Form.IsPinned, input.Form.IsDismissible, input.Form.RequiresAck,
		nullText(input.Form.CTALabel), nullText(input.Form.CTAURL), input.Form.PublishAt, input.Form.ExpireAt,
		input.OperatorID, now, contentUpdatedAt, deliveryChanged)
	if err != nil {
		return announcement.Announcement{}, internalStoreError()
	}
	beforeStatus := announcement.DisplayStatus(before, now)
	if beforeStatus == announcement.StatusPublished || beforeStatus == announcement.StatusScheduled {
		if deliveryChanged {
			if appErr := s.rebuildAnnouncementRecipients(ctx, tx, item, now); appErr != nil {
				return announcement.Announcement{}, appErr
			}
		}
	}
	if beforeStatus == announcement.StatusPublished {
		if appErr := insertAnnouncementAudit(ctx, tx, announcement.AuditUpdated, item, input.OperatorID, input.OperatorName, "编辑已发布公告", now); appErr != nil {
			return announcement.Announcement{}, appErr
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return announcement.Announcement{}, internalStoreError()
	}
	return item, nil
}

func (s *Store) PublishAnnouncement(ctx context.Context, input announcement.ActionInput, now time.Time) (announcement.Announcement, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return announcement.Announcement{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	current, err := s.scanAnnouncement(ctx, tx, `
		SELECT `+announcementColumns+`
		FROM announcements a
		LEFT JOIN announcement_receipts r ON false
		WHERE a.id = $1
		FOR UPDATE OF a
	`, input.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return announcement.Announcement{}, announcementNotFound()
	}
	if err != nil {
		return announcement.Announcement{}, internalStoreError()
	}

	status, publishAt, appErr := announcement.ResolvePublishTransition(current, now)
	if appErr != nil {
		return announcement.Announcement{}, appErr
	}
	reason := "立即发布公告"
	if status == announcement.StatusScheduled {
		reason = "设置未来发布时间"
	}
	item, err := s.scanAnnouncement(ctx, tx, `
		UPDATE announcements
		SET status = $2, publish_at = $3, updated_by_user_id = $4, updated_at = $5,
		    version = version + 1
		WHERE id = $1
		RETURNING `+announcementReturningColumns+`
	`, input.ID, status, publishAt, input.OperatorID, now)
	if err != nil {
		return announcement.Announcement{}, internalStoreError()
	}
	if appErr := s.rebuildAnnouncementRecipients(ctx, tx, item, now); appErr != nil {
		return announcement.Announcement{}, appErr
	}
	if appErr := insertAnnouncementAudit(ctx, tx, announcement.AuditPublished, item, input.OperatorID, input.OperatorName, reason, now); appErr != nil {
		return announcement.Announcement{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return announcement.Announcement{}, internalStoreError()
	}
	return item, nil
}

func (s *Store) OfflineAnnouncement(ctx context.Context, input announcement.ActionInput, now time.Time) (announcement.Announcement, *domain.AppError) {
	if strings.TrimSpace(input.Reason) == "" {
		return announcement.Announcement{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Reason required", "下线公告必须填写原因。", "reason", "required", "下线公告必须填写原因。")
	}
	current, appErr := s.AdminAnnouncementByID(ctx, input.ID, now)
	if appErr != nil {
		return announcement.Announcement{}, appErr
	}
	status := announcement.DisplayStatus(current, now)
	if status != announcement.StatusPublished && status != announcement.StatusScheduled {
		return announcement.Announcement{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid announcement state", "只有发布中或待发布公告可以下线。")
	}
	return s.updateAnnouncementStatusWithAudit(ctx, input, announcement.StatusOffline, announcement.AuditOfflined, input.Reason, now)
}

func (s *Store) DuplicateAnnouncement(ctx context.Context, input announcement.ActionInput, now time.Time) (announcement.Announcement, *domain.AppError) {
	source, appErr := s.AdminAnnouncementByID(ctx, input.ID, now)
	if appErr != nil {
		return announcement.Announcement{}, appErr
	}
	form := announcement.FormInput{
		Title:           source.Title + " 副本",
		Summary:         source.Summary,
		ContentMarkdown: source.ContentMarkdown,
		Category:        source.Category,
		Level:           source.Level,
		Channels:        source.Channels,
		IsPinned:        source.IsPinned,
		IsDismissible:   source.IsDismissible,
		RequiresAck:     source.RequiresAck,
		CTALabel:        source.CTALabel,
		CTAURL:          source.CTAURL,
		PublishAt:       source.PublishAt,
		ExpireAt:        source.ExpireAt,
		Audience:        source.Audience,
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return announcement.Announcement{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	item, appErr := s.insertAnnouncement(ctx, tx, form, input.OperatorID, input.OperatorID, announcement.StatusDraft, uniqueSlugBase(form.Title), now)
	if appErr != nil {
		return announcement.Announcement{}, appErr
	}
	if appErr := insertAnnouncementAudit(ctx, tx, announcement.AuditDuplicated, item, input.OperatorID, input.OperatorName, "复制自 "+source.Title, now); appErr != nil {
		return announcement.Announcement{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return announcement.Announcement{}, internalStoreError()
	}
	return item, nil
}

func (s *Store) AnnouncementAuditLogs(ctx context.Context, now time.Time) ([]announcement.AuditLog, *domain.AppError) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, action, announcement_id::text, announcement_title,
		       COALESCE(operator_user_id::text, ''), operator_name, COALESCE(reason, ''), created_at
		FROM announcement_audit_logs
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	logs := []announcement.AuditLog{}
	for rows.Next() {
		var log announcement.AuditLog
		if err := rows.Scan(&log.ID, &log.Action, &log.AnnouncementID, &log.AnnouncementTitle, &log.OperatorID, &log.OperatorName, &log.Reason, &log.CreatedAt); err != nil {
			return nil, internalStoreError()
		}
		logs = append(logs, log)
	}
	if rows.Err() != nil {
		return nil, internalStoreError()
	}
	return logs, nil
}

func (s *Store) updateAnnouncementStatusWithAudit(ctx context.Context, input announcement.ActionInput, status, action, reason string, now time.Time) (announcement.Announcement, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return announcement.Announcement{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	item, err := s.scanAnnouncement(ctx, tx, `
		UPDATE announcements
		SET status = $2, updated_by_user_id = $3, updated_at = $4
		WHERE id = $1
		RETURNING `+announcementReturningColumns+`
	`, input.ID, status, input.OperatorID, now)
	if errors.Is(err, pgx.ErrNoRows) {
		return announcement.Announcement{}, announcementNotFound()
	}
	if err != nil {
		return announcement.Announcement{}, internalStoreError()
	}
	if appErr := insertAnnouncementAudit(ctx, tx, action, item, input.OperatorID, input.OperatorName, reason, now); appErr != nil {
		return announcement.Announcement{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return announcement.Announcement{}, internalStoreError()
	}
	return item, nil
}

func (s *Store) insertAnnouncement(ctx context.Context, q queryer, form announcement.FormInput, createdBy, updatedBy, status, slugBase string, now time.Time) (announcement.Announcement, *domain.AppError) {
	slug := slugBase
	for i := 2; ; i++ {
		item, err := s.scanAnnouncement(ctx, q, `
			INSERT INTO announcements (
			  slug, title, summary, content_markdown, category, level, status, channels,
			  audience_json, is_pinned, is_dismissible, requires_ack, cta_label, cta_url, publish_at, expire_at,
			  created_by_user_id, updated_by_user_id, created_at, updated_at, content_updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $19, $19)
			RETURNING `+announcementReturningColumns+`
		`, slug, strings.TrimSpace(form.Title), strings.TrimSpace(form.Summary), strings.TrimSpace(form.ContentMarkdown),
			form.Category, form.Level, status, form.Channels, mustJSON(form.Audience), form.IsPinned, form.IsDismissible, form.RequiresAck,
			nullText(form.CTALabel), nullText(form.CTAURL), form.PublishAt, form.ExpireAt, createdBy, updatedBy, now)
		if isUniqueViolation(err) {
			slug = slugBase + "-" + strconv.Itoa(i)
			continue
		}
		if err != nil {
			return announcement.Announcement{}, internalStoreError()
		}
		return item, nil
	}
}

func (s *Store) rebuildAnnouncementRecipients(ctx context.Context, tx pgx.Tx, item announcement.Announcement, now time.Time) *domain.AppError {
	if _, err := tx.Exec(ctx, `DELETE FROM announcement_recipients WHERE announcement_id = $1`, item.ID); err != nil {
		return internalStoreError()
	}

	switch item.Audience.Type {
	case "", announcement.AudienceAll:
		return nil
	case announcement.AudienceSpecificUsers:
		userIDs := make([]uuid.UUID, 0, len(item.Audience.UserIDs))
		for _, value := range item.Audience.UserIDs {
			parsed, err := uuid.Parse(value)
			if err != nil {
				return announcementAudienceFieldError("audience.userIds", "指定用户 ID 格式无效。")
			}
			userIDs = append(userIDs, parsed)
		}
		var eligible int
		if err := tx.QueryRow(ctx, `
			SELECT count(*)::int
			FROM users
			WHERE id = ANY($1::uuid[]) AND account_status <> 'archived'
		`, userIDs).Scan(&eligible); err != nil {
			return internalStoreError()
		}
		if eligible != len(userIDs) {
			return announcementAudienceFieldError("audience.userIds", "指定用户不存在或已归档。")
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO announcement_recipients (announcement_id, user_id, announcement_version, snapshotted_at)
			SELECT $1, id, $2, $3 FROM users WHERE id = ANY($4::uuid[])
		`, item.ID, item.Version, now, userIDs); err != nil {
			return internalStoreError()
		}
		return nil
	case announcement.AudienceRoles:
		buyer := stringSliceContains(item.Audience.Roles, announcement.AudienceRoleBuyer)
		merchant := stringSliceContains(item.Audience.Roles, announcement.AudienceRoleMerchant)
		admin := stringSliceContains(item.Audience.Roles, announcement.AudienceRoleAdmin)
		if _, err := tx.Exec(ctx, `
			INSERT INTO announcement_recipients (announcement_id, user_id, announcement_version, snapshotted_at)
			SELECT $1, u.id, $2, $3
			FROM users u
			WHERE u.account_status <> 'archived'
			  AND (
			    ($4 AND (
			      EXISTS (SELECT 1 FROM linux_do_bindings binding WHERE binding.user_id = u.id)
			      OR EXISTS (SELECT 1 FROM student_email_claims claim WHERE claim.user_id = u.id)
			    ))
			    OR ($5 AND EXISTS (SELECT 1 FROM linux_do_bindings binding WHERE binding.user_id = u.id))
			    OR ($6
			      AND EXISTS (SELECT 1 FROM user_permissions permission WHERE permission.user_id = u.id AND permission.permission = 'admin')
			      AND (
			        NOT EXISTS (SELECT 1 FROM student_email_claims claim WHERE claim.user_id = u.id)
			        OR EXISTS (SELECT 1 FROM linux_do_bindings binding WHERE binding.user_id = u.id)
			      )
			    )
			  )
		`, item.ID, item.Version, now, buyer, merchant, admin); err != nil {
			return internalStoreError()
		}
		return nil
	default:
		return announcementAudienceFieldError("audience.type", "公告受众类型不支持。")
	}
}

func announcementAudienceFieldError(field, message string) *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Announcement audience validation failed", "公告受众校验失败。", field, "invalid", message)
}

func stringSliceContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func mustJSON(value any) []byte {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return encoded
}

func (s *Store) queryAnnouncements(ctx context.Context, userID string, sql string, args ...any) ([]announcement.Announcement, *domain.AppError) {
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	items := []announcement.Announcement{}
	for rows.Next() {
		item, err := scanAnnouncementRow(rows)
		if err != nil {
			return nil, internalStoreError()
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		return nil, internalStoreError()
	}
	return items, nil
}

func (s *Store) scanAnnouncement(ctx context.Context, q queryer, sql string, args ...any) (announcement.Announcement, error) {
	return scanAnnouncementRow(q.QueryRow(ctx, sql, args...))
}

func scanAnnouncementRow(row scanner) (announcement.Announcement, error) {
	var item announcement.Announcement
	var audienceText string
	var receiptID *string
	var receiptVersion *int64
	var firstSeenAt *time.Time
	var readAt *time.Time
	var dismissedAt *time.Time
	var acknowledgedAt *time.Time
	var ctaLabel *string
	var ctaURL *string
	var createdBy *string
	var updatedBy *string
	err := row.Scan(
		&item.ID, &item.Slug, &item.Title, &item.Summary, &item.ContentMarkdown,
		&item.Category, &item.Level, &item.Status, &item.Channels, &audienceText,
		&item.IsPinned, &item.IsDismissible, &item.RequiresAck, &ctaLabel, &ctaURL, &item.PublishAt, &item.ExpireAt,
		&item.ContentUpdatedAt, &item.Version, &createdBy, &updatedBy, &item.CreatedAt, &item.UpdatedAt,
		&receiptID, &receiptVersion, &firstSeenAt, &readAt, &dismissedAt, &acknowledgedAt,
	)
	if err != nil {
		return announcement.Announcement{}, err
	}
	item.CTALabel = stringFromPtr(ctaLabel)
	item.CTAURL = stringFromPtr(ctaURL)
	item.CreatedBy = stringFromPtr(createdBy)
	item.UpdatedBy = stringFromPtr(updatedBy)
	if err := json.Unmarshal([]byte(audienceText), &item.Audience); err != nil {
		item.Audience = announcement.Audience{Type: "all"}
	}
	if receiptID != nil && receiptVersion != nil {
		item.Receipt = &announcement.Receipt{
			AnnouncementID:      *receiptID,
			AnnouncementVersion: *receiptVersion,
			FirstSeenAt:         firstSeenAt,
			ReadAt:              readAt,
			DismissedAt:         dismissedAt,
			AcknowledgedAt:      acknowledgedAt,
		}
	}
	return item, nil
}

func filterUserVisible(items []announcement.Announcement, now time.Time, channel string) []announcement.Announcement {
	result := make([]announcement.Announcement, 0, len(items))
	for _, item := range items {
		if announcement.IsUserVisible(item, now) && (channel == "" || announcement.HasChannel(item, channel)) {
			result = append(result, item)
		}
	}
	return result
}

func insertAnnouncementAudit(ctx context.Context, q queryer, action string, item announcement.Announcement, operatorID, operatorName, reason string, now time.Time) *domain.AppError {
	_, err := q.(interface {
		Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	}).Exec(ctx, `
		INSERT INTO announcement_audit_logs (
		  action, announcement_id, announcement_title, operator_user_id, operator_name, reason, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, action, item.ID, item.Title, nullUUID(operatorID), strings.TrimSpace(operatorName), nullText(reason), now)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func nullableActionTime(action, target string, now time.Time) any {
	if action == target {
		return now
	}
	return nil
}

func nullableFirstSeenTime(action string, now time.Time) any {
	if action == "seen" || action == "dismiss" || action == "acknowledge" {
		return now
	}
	return nil
}

func announcementNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Announcement not found", "公告不存在或当前不可见。")
}

func uniqueSlugBase(title string) string {
	value := strings.ToLower(strings.TrimSpace(title))
	value = announcementSlugPattern.ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "announcement"
	}
	return value
}

func stringFromPtr(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

const announcementColumns = `
	a.id::text, a.slug, a.title, a.summary, a.content_markdown,
	a.category, a.level, a.status, a.channels, a.audience_json::text,
	a.is_pinned, a.is_dismissible, a.requires_ack, a.cta_label, a.cta_url, a.publish_at, a.expire_at,
	a.content_updated_at, a.version, a.created_by_user_id::text, a.updated_by_user_id::text, a.created_at, a.updated_at,
	r.announcement_id::text, r.announcement_version, r.first_seen_at, r.read_at, r.dismissed_at, r.acknowledged_at
`

const announcementReturningColumns = `
	id::text, slug, title, summary, content_markdown,
	category, level, status, channels, audience_json::text,
	is_pinned, is_dismissible, requires_ack, cta_label, cta_url, publish_at, expire_at,
	content_updated_at, version, created_by_user_id::text, updated_by_user_id::text, created_at, updated_at,
	NULL::text, NULL::bigint, NULL::timestamptz, NULL::timestamptz, NULL::timestamptz, NULL::timestamptz
`

var announcementSlugPattern = regexp.MustCompile(`[^a-z0-9\p{Han}]+`)
