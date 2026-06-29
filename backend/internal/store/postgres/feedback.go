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
	"c2c-market/backend/internal/module/feedback"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Store) CreateFeedbackTicketWithIdempotency(ctx context.Context, entry idempotency.Entry, input feedback.CreateInput, now time.Time, buildCompletion feedback.CompletionBuilder) (feedback.Ticket, idempotency.Completion, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return feedback.Ticket{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return feedback.Ticket{}, idempotency.Completion{}, appErr
	}
	item, appErr := createFeedbackTicketInTx(ctx, tx, input, now)
	if appErr != nil {
		return feedback.Ticket{}, idempotency.Completion{}, appErr
	}
	if appErr := insertFeedbackEvent(ctx, tx, item.ID, input.SubmitterUserID, feedback.EventSubmitted, "user", "用户提交问题反馈", "", input.RequestID, now); appErr != nil {
		return feedback.Ticket{}, idempotency.Completion{}, appErr
	}
	item, err = scanFeedbackTicket(ctx, tx, feedbackTicketSelectSQL+` WHERE ft.id = $1`, item.ID)
	if err != nil {
		return feedback.Ticket{}, idempotency.Completion{}, internalStoreError()
	}
	completion, appErr := buildCompletion(item)
	if appErr != nil {
		return feedback.Ticket{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return feedback.Ticket{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return feedback.Ticket{}, idempotency.Completion{}, internalStoreError()
	}
	return item, completion, nil
}

func (s *Store) ListFeedbackTicketsBySubmitter(ctx context.Context, submitterUserID string) ([]feedback.Ticket, *domain.AppError) {
	rows, err := s.pool.Query(ctx, feedbackTicketSelectSQL+`
		WHERE ft.submitter_user_id = $1
		ORDER BY ft.updated_at DESC
	`, submitterUserID)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return scanFeedbackTickets(rows)
}

func (s *Store) GetFeedbackTicketForSubmitter(ctx context.Context, submitterUserID, id string) (feedback.Ticket, *domain.AppError) {
	item, err := scanFeedbackTicket(ctx, s.pool, feedbackTicketSelectSQL+`
		WHERE ft.id = $1 AND ft.submitter_user_id = $2
	`, id, submitterUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return feedback.Ticket{}, feedbackNotFound()
	}
	if err != nil {
		return feedback.Ticket{}, internalStoreError()
	}
	events, appErr := listFeedbackEvents(ctx, s.pool, item.ID, false)
	if appErr != nil {
		return feedback.Ticket{}, appErr
	}
	item.Events = events
	return item, nil
}

func (s *Store) AddFeedbackSupplementWithIdempotency(ctx context.Context, entry idempotency.Entry, input feedback.SupplementInput, now time.Time, buildCompletion feedback.CompletionBuilder) (feedback.Ticket, idempotency.Completion, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return feedback.Ticket{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return feedback.Ticket{}, idempotency.Completion{}, appErr
	}
	item, appErr := updateFeedbackSupplementInTx(ctx, tx, input, now)
	if appErr != nil {
		return feedback.Ticket{}, idempotency.Completion{}, appErr
	}
	if appErr := insertFeedbackEvent(ctx, tx, item.ID, input.SubmitterUserID, feedback.EventUserSupplemented, "user", input.Message, "", input.RequestID, now); appErr != nil {
		return feedback.Ticket{}, idempotency.Completion{}, appErr
	}
	item, err = scanFeedbackTicket(ctx, tx, feedbackTicketSelectSQL+` WHERE ft.id = $1`, item.ID)
	if err != nil {
		return feedback.Ticket{}, idempotency.Completion{}, internalStoreError()
	}
	events, appErr := listFeedbackEvents(ctx, tx, item.ID, false)
	if appErr != nil {
		return feedback.Ticket{}, idempotency.Completion{}, appErr
	}
	item.Events = events
	completion, appErr := buildCompletion(item)
	if appErr != nil {
		return feedback.Ticket{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return feedback.Ticket{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return feedback.Ticket{}, idempotency.Completion{}, internalStoreError()
	}
	return item, completion, nil
}

func (s *Store) MarkFeedbackRead(ctx context.Context, submitterUserID, id string, now time.Time) (feedback.Ticket, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return feedback.Ticket{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	current, err := scanFeedbackTicket(ctx, tx, feedbackTicketSelectSQL+`
		WHERE ft.id = $1 AND ft.submitter_user_id = $2
		FOR UPDATE OF ft
	`, id, submitterUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return feedback.Ticket{}, feedbackNotFound()
	}
	if err != nil {
		return feedback.Ticket{}, internalStoreError()
	}
	if current.LatestAdminUpdateAt != nil && (current.SubmitterReadAt == nil || current.SubmitterReadAt.Before(*current.LatestAdminUpdateAt)) {
		if _, err := tx.Exec(ctx, `
			UPDATE feedback_tickets
			SET submitter_read_at = $3,
			    updated_at = $3,
			    version = version + 1
			WHERE id = $1 AND submitter_user_id = $2
		`, id, submitterUserID, now); err != nil {
			return feedback.Ticket{}, internalStoreError()
		}
		if appErr := insertFeedbackEvent(ctx, tx, id, submitterUserID, feedback.EventRead, "user", "用户已查看处理结果", "", "", now); appErr != nil {
			return feedback.Ticket{}, appErr
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE notifications
		SET read_at = COALESCE(read_at, $3)
		WHERE user_id = $1
		  AND target_type = 'feedback_ticket'
		  AND target_id = $2
		  AND read_at IS NULL
	`, submitterUserID, id, now); err != nil {
		return feedback.Ticket{}, internalStoreError()
	}
	item, err := scanFeedbackTicket(ctx, tx, feedbackTicketSelectSQL+` WHERE ft.id = $1`, id)
	if err != nil {
		return feedback.Ticket{}, internalStoreError()
	}
	events, appErr := listFeedbackEvents(ctx, tx, item.ID, false)
	if appErr != nil {
		return feedback.Ticket{}, appErr
	}
	item.Events = events
	if err := tx.Commit(ctx); err != nil {
		return feedback.Ticket{}, internalStoreError()
	}
	return item, nil
}

func (s *Store) UnreadFeedbackCount(ctx context.Context, submitterUserID string) (int, *domain.AppError) {
	var count int
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*)::int
		FROM feedback_tickets
		WHERE submitter_user_id = $1
		  AND latest_admin_update_at IS NOT NULL
		  AND (submitter_read_at IS NULL OR submitter_read_at < latest_admin_update_at)
	`, submitterUserID).Scan(&count); err != nil {
		return 0, internalStoreError()
	}
	return count, nil
}

func (s *Store) ListAdminFeedbackTickets(ctx context.Context) ([]feedback.Ticket, *domain.AppError) {
	rows, err := s.pool.Query(ctx, feedbackTicketSelectSQL+`
		ORDER BY ft.updated_at DESC
	`)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return scanFeedbackTickets(rows)
}

func (s *Store) GetAdminFeedbackTicket(ctx context.Context, id string) (feedback.Ticket, *domain.AppError) {
	item, err := scanFeedbackTicket(ctx, s.pool, feedbackTicketSelectSQL+`
		WHERE ft.id = $1
	`, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return feedback.Ticket{}, feedbackNotFound()
	}
	if err != nil {
		return feedback.Ticket{}, internalStoreError()
	}
	events, appErr := listFeedbackEvents(ctx, s.pool, item.ID, true)
	if appErr != nil {
		return feedback.Ticket{}, appErr
	}
	item.Events = events
	return item, nil
}

func (s *Store) HandleAdminFeedbackTicketWithIdempotency(ctx context.Context, entry idempotency.Entry, input feedback.AdminHandleInput, now time.Time, buildCompletion feedback.CompletionBuilder) (feedback.Ticket, idempotency.Completion, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return feedback.Ticket{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return feedback.Ticket{}, idempotency.Completion{}, appErr
	}
	item, appErr := updateFeedbackAdminHandleInTx(ctx, tx, input, now)
	if appErr != nil {
		return feedback.Ticket{}, idempotency.Completion{}, appErr
	}
	if appErr := insertFeedbackEvent(ctx, tx, item.ID, input.AdminUserID, feedback.EventAdminHandled, "admin", input.Response, input.InternalNote, input.RequestID, now); appErr != nil {
		return feedback.Ticket{}, idempotency.Completion{}, appErr
	}
	if appErr := insertFeedbackDomainEventAndNotification(ctx, tx, item, input, now); appErr != nil {
		return feedback.Ticket{}, idempotency.Completion{}, appErr
	}
	item, err = scanFeedbackTicket(ctx, tx, feedbackTicketSelectSQL+` WHERE ft.id = $1`, item.ID)
	if err != nil {
		return feedback.Ticket{}, idempotency.Completion{}, internalStoreError()
	}
	events, appErr := listFeedbackEvents(ctx, tx, item.ID, true)
	if appErr != nil {
		return feedback.Ticket{}, idempotency.Completion{}, appErr
	}
	item.Events = events
	completion, appErr := buildCompletion(item)
	if appErr != nil {
		return feedback.Ticket{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return feedback.Ticket{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return feedback.Ticket{}, idempotency.Completion{}, internalStoreError()
	}
	return item, completion, nil
}

func createFeedbackTicketInTx(ctx context.Context, tx pgx.Tx, input feedback.CreateInput, now time.Time) (feedback.Ticket, *domain.AppError) {
	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = strings.TrimSpace(input.Description)
		runes := []rune(title)
		if len(runes) > 32 {
			title = string(runes[:32])
		}
	}
	id := uuid.NewString()
	_, err := tx.Exec(ctx, `
		INSERT INTO feedback_tickets (
			id, submitter_user_id, type, impact, status, title, description,
			context_page_label, context_target_type, context_target_id, context_target_label,
			context_role_label, created_at, updated_at, version
		)
		VALUES ($1, $2, $3, $4, 'submitted', $5, $6, $7, $8, $9, $10, $11, $12, $12, 1)
	`, id, input.SubmitterUserID, input.Type, input.Impact, title, strings.TrimSpace(input.Description),
		strings.TrimSpace(input.ContextPageLabel), strings.TrimSpace(input.ContextTargetType),
		strings.TrimSpace(input.ContextTargetID), strings.TrimSpace(input.ContextTargetLabel),
		strings.TrimSpace(input.ContextRoleLabel), now)
	if err != nil {
		return feedback.Ticket{}, internalStoreError()
	}
	return feedback.Ticket{ID: id}, nil
}

func updateFeedbackSupplementInTx(ctx context.Context, tx pgx.Tx, input feedback.SupplementInput, now time.Time) (feedback.Ticket, *domain.AppError) {
	current, err := scanFeedbackTicket(ctx, tx, feedbackTicketSelectSQL+`
		WHERE ft.id = $1 AND ft.submitter_user_id = $2
		FOR UPDATE OF ft
	`, input.ID, input.SubmitterUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return feedback.Ticket{}, feedbackNotFound()
	}
	if err != nil {
		return feedback.Ticket{}, internalStoreError()
	}
	if current.Status == feedback.StatusClosed {
		return feedback.Ticket{}, feedbackInvalidState("已关闭反馈不能继续补充。")
	}
	nextStatus := current.Status
	if current.Status == feedback.StatusNeedsUserInfo {
		nextStatus = feedback.StatusSubmitted
	}
	item, err := scanFeedbackTicket(ctx, tx, `
		UPDATE feedback_tickets ft
		SET status = $2,
		    updated_at = $3,
		    version = version + 1
		WHERE ft.id = $1
		RETURNING `+feedbackTicketReturningColumns, current.ID, nextStatus, now)
	if err != nil {
		return feedback.Ticket{}, internalStoreError()
	}
	return item, nil
}

func updateFeedbackAdminHandleInTx(ctx context.Context, tx pgx.Tx, input feedback.AdminHandleInput, now time.Time) (feedback.Ticket, *domain.AppError) {
	current, err := scanFeedbackTicket(ctx, tx, feedbackTicketSelectSQL+`
		WHERE ft.id = $1
		FOR UPDATE OF ft
	`, input.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return feedback.Ticket{}, feedbackNotFound()
	}
	if err != nil {
		return feedback.Ticket{}, internalStoreError()
	}
	if input.ExpectedVersion > 0 && current.Version != input.ExpectedVersion {
		return feedback.Ticket{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if current.Status == feedback.StatusClosed {
		return feedback.Ticket{}, feedbackInvalidState("已关闭反馈不能继续处理。")
	}
	item, err := scanFeedbackTicket(ctx, tx, `
		UPDATE feedback_tickets ft
		SET status = $2,
		    admin_response = $3,
		    admin_internal_note = $4,
		    handled_by_admin_id = $5,
		    handled_at = $6,
		    latest_admin_update_at = $6,
		    submitter_read_at = NULL,
		    updated_at = $6,
		    version = version + 1
		WHERE ft.id = $1
		RETURNING `+feedbackTicketReturningColumns, current.ID, input.Status, strings.TrimSpace(input.Response),
		strings.TrimSpace(input.InternalNote), input.AdminUserID, now)
	if err != nil {
		return feedback.Ticket{}, internalStoreError()
	}
	return item, nil
}

func insertFeedbackEvent(ctx context.Context, tx pgx.Tx, ticketID, actorUserID, action, actorRole, publicMessage, internalNote, requestID string, now time.Time) *domain.AppError {
	_, err := tx.Exec(ctx, `
		INSERT INTO feedback_events (
			id, ticket_id, actor_user_id, actor_role, action, public_message,
			internal_note, request_id, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, uuid.NewString(), ticketID, nullUUID(actorUserID), actorRole, action, strings.TrimSpace(publicMessage),
		strings.TrimSpace(internalNote), strings.TrimSpace(requestID), now)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func insertFeedbackDomainEventAndNotification(ctx context.Context, tx pgx.Tx, item feedback.Ticket, input feedback.AdminHandleInput, now time.Time) *domain.AppError {
	requestID := strings.TrimSpace(input.RequestID)
	if requestID == "" {
		requestID = "unknown"
	}
	eventID := uuid.NewString()
	metadata, err := json.Marshal(map[string]string{
		"status": item.Status,
	})
	if err != nil {
		return internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO domain_events (
			id, aggregate_type, aggregate_id, event_type, actor_user_id, actor_kind,
			aggregate_version, request_id, metadata_json, created_at
		)
		VALUES ($1, 'feedback_ticket', $2, 'feedback_ticket.admin_handled', $3, 'admin', $4, $5, $6, $7)
	`, eventID, item.ID, input.AdminUserID, item.Version, requestID, metadata, now)
	if err != nil {
		return internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO notifications (
			user_id, type, title, body, target_type, target_id, target_url,
			source_event_type, source_event_id, dedupe_key, created_at
		)
		VALUES (
			$1, 'feedback_ticket_admin_handled', '你的问题反馈已有处理结果',
			$2, 'feedback_ticket', $3, $4,
			'feedback_ticket.admin_handled', $5, $6, $7
		)
		ON CONFLICT (user_id, dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING
	`, item.SubmitterUserID, strings.TrimSpace(input.Response), item.ID, "/my/feedback/"+item.ID, eventID,
		"feedback_ticket:"+item.ID+":admin_update:"+strconv.FormatInt(item.Version, 10), now)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func scanFeedbackTickets(rows pgx.Rows) ([]feedback.Ticket, *domain.AppError) {
	items := []feedback.Ticket{}
	for rows.Next() {
		item, err := scanFeedbackTicketRow(rows)
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

func scanFeedbackTicket(ctx context.Context, q queryer, sql string, args ...any) (feedback.Ticket, error) {
	return scanFeedbackTicketRow(q.QueryRow(ctx, sql, args...))
}

func scanFeedbackTicketRow(row scanner) (feedback.Ticket, error) {
	var item feedback.Ticket
	err := row.Scan(
		&item.ID,
		&item.SubmitterUserID,
		&item.SubmitterUsername,
		&item.SubmitterName,
		&item.Type,
		&item.Impact,
		&item.Status,
		&item.Title,
		&item.Description,
		&item.ContextPageLabel,
		&item.ContextTargetType,
		&item.ContextTargetID,
		&item.ContextTargetLabel,
		&item.ContextRoleLabel,
		&item.AdminResponse,
		&item.AdminInternalNote,
		&item.HandledByAdminID,
		&item.HandledByAdminName,
		&item.HandledAt,
		&item.LatestAdminUpdateAt,
		&item.SubmitterReadAt,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.Version,
	)
	return item, err
}

func listFeedbackEvents(ctx context.Context, q queryer, ticketID string, includeInternal bool) ([]feedback.Event, *domain.AppError) {
	rows, err := queryRows(ctx, q, `
		SELECT
			fe.id::text,
			fe.ticket_id::text,
			COALESCE(fe.actor_user_id::text, ''),
			COALESCE(NULLIF(u.display_name, ''), u.username, ''),
			fe.actor_role,
			fe.action,
			fe.public_message,
			CASE WHEN $2 THEN fe.internal_note ELSE '' END,
			fe.created_at
		FROM feedback_events fe
		LEFT JOIN users u ON u.id = fe.actor_user_id
		WHERE fe.ticket_id = $1
		ORDER BY fe.created_at ASC
	`, ticketID, includeInternal)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	items := []feedback.Event{}
	for rows.Next() {
		var item feedback.Event
		if err := rows.Scan(
			&item.ID,
			&item.TicketID,
			&item.ActorUserID,
			&item.ActorName,
			&item.ActorRole,
			&item.Action,
			&item.PublicMessage,
			&item.InternalNote,
			&item.CreatedAt,
		); err != nil {
			return nil, internalStoreError()
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, internalStoreError()
	}
	return items, nil
}

func feedbackNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Feedback ticket not found", "问题反馈不存在。")
}

func feedbackInvalidState(detail string) *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid feedback state", detail)
}

const feedbackTicketSelectSQL = `
SELECT ` + feedbackTicketColumns + `
FROM feedback_tickets ft
JOIN users submitter ON submitter.id = ft.submitter_user_id
LEFT JOIN users handler ON handler.id = ft.handled_by_admin_id`

const feedbackTicketColumns = `
	ft.id::text,
	ft.submitter_user_id::text,
	submitter.username,
	COALESCE(NULLIF(submitter.display_name, ''), submitter.username, ''),
	ft.type,
	ft.impact,
	ft.status,
	ft.title,
	ft.description,
	ft.context_page_label,
	ft.context_target_type,
	ft.context_target_id,
	ft.context_target_label,
	ft.context_role_label,
	ft.admin_response,
	ft.admin_internal_note,
	COALESCE(ft.handled_by_admin_id::text, ''),
	COALESCE(NULLIF(handler.display_name, ''), handler.username, ''),
	ft.handled_at,
	ft.latest_admin_update_at,
	ft.submitter_read_at,
	ft.created_at,
	ft.updated_at,
	ft.version`

const feedbackTicketReturningColumns = `
	ft.id::text,
	ft.submitter_user_id::text,
	(SELECT username FROM users WHERE id = ft.submitter_user_id),
	(SELECT COALESCE(NULLIF(display_name, ''), username, '') FROM users WHERE id = ft.submitter_user_id),
	ft.type,
	ft.impact,
	ft.status,
	ft.title,
	ft.description,
	ft.context_page_label,
	ft.context_target_type,
	ft.context_target_id,
	ft.context_target_label,
	ft.context_role_label,
	ft.admin_response,
	ft.admin_internal_note,
	COALESCE(ft.handled_by_admin_id::text, ''),
	COALESCE((SELECT COALESCE(NULLIF(display_name, ''), username, '') FROM users WHERE id = ft.handled_by_admin_id), ''),
	ft.handled_at,
	ft.latest_admin_update_at,
	ft.submitter_read_at,
	ft.created_at,
	ft.updated_at,
	ft.version`
