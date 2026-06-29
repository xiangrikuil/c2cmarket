package postgres

import (
	"context"
	"errors"
	"net/http"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/demand"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/jackc/pgx/v5"
)

func (s *Store) CreateDemand(ctx context.Context, item demand.Demand) *domain.AppError {
	if s == nil || s.pool == nil {
		return internalStoreError()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO demands (
			id, publisher_user_id, title, max_price_cny, region_code, owner_preference,
			source_url, note, status, created_at, updated_at, version
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10, $11)
	`, item.ID, item.PublisherUserID, item.Title, item.MaxPriceCNY, item.RegionCode, item.OwnerPreference,
		item.SourceURL, nullText(item.Note), item.Status, item.CreatedAt, item.Version)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) ListPublicDemands(ctx context.Context) ([]demand.Demand, *domain.AppError) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+demandColumns+`
		FROM demands d
		JOIN users u ON u.id = d.publisher_user_id
		WHERE d.status = 'active'
		ORDER BY d.updated_at DESC
	`)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return scanDemands(rows)
}

func (s *Store) GetPublicDemand(ctx context.Context, id string) (demand.Demand, *domain.AppError) {
	item, err := scanDemand(ctx, s.pool, `
		SELECT `+demandColumns+`
		FROM demands d
		JOIN users u ON u.id = d.publisher_user_id
		WHERE d.id = $1 AND d.status = 'active'
	`, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return demand.Demand{}, demandNotFound()
	}
	if err != nil {
		return demand.Demand{}, internalStoreError()
	}
	return item, nil
}

func (s *Store) ListDemandsByPublisher(ctx context.Context, publisherUserID string) ([]demand.Demand, *domain.AppError) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+demandColumns+`
		FROM demands d
		JOIN users u ON u.id = d.publisher_user_id
		WHERE d.publisher_user_id = $1
		ORDER BY d.updated_at DESC
	`, publisherUserID)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return scanDemands(rows)
}

func (s *Store) GetDemandForPublisher(ctx context.Context, publisherUserID, id string) (demand.Demand, *domain.AppError) {
	item, err := scanDemand(ctx, s.pool, `
		SELECT `+demandColumns+`
		FROM demands d
		JOIN users u ON u.id = d.publisher_user_id
		WHERE d.id = $1 AND d.publisher_user_id = $2
	`, id, publisherUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return demand.Demand{}, demandNotFound()
	}
	if err != nil {
		return demand.Demand{}, internalStoreError()
	}
	return item, nil
}

func (s *Store) ListAdminDemands(ctx context.Context) ([]demand.Demand, *domain.AppError) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+demandColumns+`
		FROM demands d
		JOIN users u ON u.id = d.publisher_user_id
		ORDER BY d.updated_at DESC
	`)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return scanDemands(rows)
}

func (s *Store) GetAdminDemand(ctx context.Context, id string) (demand.Demand, *domain.AppError) {
	item, err := scanDemand(ctx, s.pool, `
		SELECT `+demandColumns+`
		FROM demands d
		JOIN users u ON u.id = d.publisher_user_id
		WHERE d.id = $1
	`, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return demand.Demand{}, demandNotFound()
	}
	if err != nil {
		return demand.Demand{}, internalStoreError()
	}
	return item, nil
}

func (s *Store) UpdateDemandOwnerStatusWithIdempotency(ctx context.Context, entry idempotency.Entry, input demand.OwnerActionInput, now time.Time, buildCompletion demand.CompletionBuilder) (demand.Demand, idempotency.Completion, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return demand.Demand{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return demand.Demand{}, idempotency.Completion{}, appErr
	}
	item, appErr := s.updateDemandOwnerStatusInTx(ctx, tx, input, now)
	if appErr != nil {
		return demand.Demand{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(item)
	if appErr != nil {
		return demand.Demand{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return demand.Demand{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return demand.Demand{}, idempotency.Completion{}, internalStoreError()
	}
	return item, completion, nil
}

func (s *Store) UpdateDemandAdminStatusWithIdempotency(ctx context.Context, entry idempotency.Entry, input demand.AdminActionInput, now time.Time, buildCompletion demand.CompletionBuilder) (demand.Demand, idempotency.Completion, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return demand.Demand{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return demand.Demand{}, idempotency.Completion{}, appErr
	}
	item, appErr := s.updateDemandAdminStatusInTx(ctx, tx, input, now)
	if appErr != nil {
		return demand.Demand{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(item)
	if appErr != nil {
		return demand.Demand{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return demand.Demand{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return demand.Demand{}, idempotency.Completion{}, internalStoreError()
	}
	return item, completion, nil
}

func (s *Store) updateDemandOwnerStatusInTx(ctx context.Context, tx pgx.Tx, input demand.OwnerActionInput, now time.Time) (demand.Demand, *domain.AppError) {
	current, err := scanDemand(ctx, tx, `
		SELECT `+demandColumns+`
		FROM demands d
		JOIN users u ON u.id = d.publisher_user_id
		WHERE d.id = $1 AND d.publisher_user_id = $2
		FOR UPDATE
	`, input.ID, input.PublisherUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return demand.Demand{}, demandNotFound()
	}
	if err != nil {
		return demand.Demand{}, internalStoreError()
	}
	if input.ExpectedVersion > 0 && current.Version != input.ExpectedVersion {
		return demand.Demand{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	next := current.Status
	var closedAt any
	switch input.Action {
	case "close":
		if current.Status == demand.StatusClosed {
			return demand.Demand{}, demandInvalidState("需求已关闭。")
		}
		if current.Status != demand.StatusActive && current.Status != demand.StatusPendingReview && current.Status != demand.StatusChangesRequested {
			return demand.Demand{}, demandInvalidState("当前需求状态不能关闭。")
		}
		next = demand.StatusClosed
		closedAt = now
	case "reopen":
		if current.Status != demand.StatusClosed {
			return demand.Demand{}, demandInvalidState("只有已关闭需求可以重新打开。")
		}
		next = demand.StatusPendingReview
		closedAt = nil
	default:
		return demand.Demand{}, demandInvalidState("需求操作不支持。")
	}
	item, err := scanDemand(ctx, tx, `
		UPDATE demands
		SET status = $2, closed_at = $3, updated_at = $4, version = version + 1
		WHERE id = $1
		RETURNING `+demandReturningColumns+`
	`, current.ID, next, closedAt, now)
	if err != nil {
		return demand.Demand{}, internalStoreError()
	}
	return item, nil
}

func (s *Store) updateDemandAdminStatusInTx(ctx context.Context, tx pgx.Tx, input demand.AdminActionInput, now time.Time) (demand.Demand, *domain.AppError) {
	current, err := scanDemand(ctx, tx, `
		SELECT `+demandColumns+`
		FROM demands d
		JOIN users u ON u.id = d.publisher_user_id
		WHERE d.id = $1
		FOR UPDATE
	`, input.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return demand.Demand{}, demandNotFound()
	}
	if err != nil {
		return demand.Demand{}, internalStoreError()
	}
	if input.ExpectedVersion > 0 && current.Version != input.ExpectedVersion {
		return demand.Demand{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	next, appErr := demandNextAdminStatus(current.Status, input.Action)
	if appErr != nil {
		return demand.Demand{}, appErr
	}
	item, err := scanDemand(ctx, tx, `
		UPDATE demands
		SET status = $2,
		    review_reason = $3,
		    reviewed_by_admin_id = $4,
		    reviewed_at = $5,
		    closed_at = CASE WHEN $2 = 'closed' THEN closed_at ELSE NULL END,
		    updated_at = $5,
		    version = version + 1
		WHERE id = $1
		RETURNING `+demandReturningColumns+`
	`, current.ID, next, nullText(input.Reason), input.AdminUserID, now)
	if err != nil {
		return demand.Demand{}, internalStoreError()
	}
	return item, nil
}

func demandNextAdminStatus(current, action string) (string, *domain.AppError) {
	switch action {
	case "approve":
		if current != demand.StatusPendingReview && current != demand.StatusChangesRequested {
			return "", demandInvalidState("只有待审核或需修改的需求可以审核通过。")
		}
		return demand.StatusActive, nil
	case "request_changes":
		if current != demand.StatusPendingReview && current != demand.StatusActive {
			return "", demandInvalidState("当前需求状态不能要求修改。")
		}
		return demand.StatusChangesRequested, nil
	case "reject":
		if current != demand.StatusPendingReview && current != demand.StatusChangesRequested {
			return "", demandInvalidState("当前需求状态不能拒绝。")
		}
		return demand.StatusRejected, nil
	case "take_down":
		if current != demand.StatusActive {
			return "", demandInvalidState("只有匹配中的需求可以下架。")
		}
		return demand.StatusTakenDown, nil
	case "restore":
		if current != demand.StatusTakenDown {
			return "", demandInvalidState("只有已下架需求可以恢复。")
		}
		return demand.StatusActive, nil
	default:
		return "", demandInvalidState("需求审核动作不支持。")
	}
}

func scanDemands(rows pgx.Rows) ([]demand.Demand, *domain.AppError) {
	items := []demand.Demand{}
	for rows.Next() {
		item, err := scanDemandRow(rows)
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

func scanDemand(ctx context.Context, q queryer, sql string, args ...any) (demand.Demand, error) {
	return scanDemandRow(q.QueryRow(ctx, sql, args...))
}

func scanDemandRow(row scanner) (demand.Demand, error) {
	var item demand.Demand
	var maxPrice string
	err := row.Scan(
		&item.ID,
		&item.PublisherUserID,
		&item.PublisherUsername,
		&item.PublisherName,
		&item.Title,
		&maxPrice,
		&item.RegionCode,
		&item.OwnerPreference,
		&item.SourceURL,
		&item.Note,
		&item.Status,
		&item.ReviewReason,
		&item.ReviewedByAdminID,
		&item.ReviewedAt,
		&item.ClosedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.Version,
	)
	item.MaxPriceCNY = storeDecimalStringMust(maxPrice, 2)
	return item, err
}

func demandNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Demand not found", "求车需求不存在。")
}

func demandInvalidState(detail string) *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid demand state", detail)
}

const demandColumns = `
	d.id::text,
	d.publisher_user_id::text,
	u.username,
	u.display_name,
	d.title,
	d.max_price_cny::text,
	d.region_code,
	d.owner_preference,
	d.source_url,
	COALESCE(d.note, ''),
	d.status,
	COALESCE(d.review_reason, ''),
	COALESCE(d.reviewed_by_admin_id::text, ''),
	d.reviewed_at,
	d.closed_at,
	d.created_at,
	d.updated_at,
	d.version
`

const demandReturningColumns = `
	demands.id::text,
	demands.publisher_user_id::text,
	(SELECT username FROM users WHERE users.id = demands.publisher_user_id),
	(SELECT display_name FROM users WHERE users.id = demands.publisher_user_id),
	demands.title,
	demands.max_price_cny::text,
	demands.region_code,
	demands.owner_preference,
	demands.source_url,
	COALESCE(demands.note, ''),
	demands.status,
	COALESCE(demands.review_reason, ''),
	COALESCE(demands.reviewed_by_admin_id::text, ''),
	demands.reviewed_at,
	demands.closed_at,
	demands.created_at,
	demands.updated_at,
	demands.version
`
