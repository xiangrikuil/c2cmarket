package postgres

import (
	"context"
	"errors"
	"net/http"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/favorite"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/jackc/pgx/v5"
)

func (s *Store) ListFavorites(ctx context.Context, userID string) ([]favorite.ListItem, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	rows, err := s.pool.Query(ctx, favoriteListBaseSQL+` ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return scanFavoriteList(rows)
}

func (s *Store) IsFavorite(ctx context.Context, userID, targetType, targetID string) (bool, *domain.AppError) {
	if s == nil || s.pool == nil {
		return false, internalStoreError()
	}
	var exists bool
	if err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM favorites
			WHERE user_id = $1 AND target_type = $2 AND target_id = $3
		)
	`, userID, targetType, targetID).Scan(&exists); err != nil {
		return false, internalStoreError()
	}
	return exists, nil
}

func (s *Store) CreateFavoriteWithIdempotency(ctx context.Context, entry idempotency.Entry, userID, targetType, targetID string, now time.Time, buildCompletion favorite.CompletionBuilder) (favorite.MutationResult, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return favorite.MutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return favorite.MutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return favorite.MutationResult{}, idempotency.Completion{}, appErr
	}
	if appErr := ensureFavoriteTargetPublic(ctx, tx, targetType, targetID); appErr != nil {
		return favorite.MutationResult{}, idempotency.Completion{}, appErr
	}
	result, appErr := upsertFavoriteInTx(ctx, tx, userID, targetType, targetID, now)
	if appErr != nil {
		return favorite.MutationResult{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(result)
	if appErr != nil {
		return favorite.MutationResult{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return favorite.MutationResult{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return favorite.MutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	return result, completion, nil
}

func (s *Store) DeleteFavorite(ctx context.Context, userID, targetType, targetID string) (favorite.MutationResult, *domain.AppError) {
	if s == nil || s.pool == nil {
		return favorite.MutationResult{}, internalStoreError()
	}
	_, err := s.pool.Exec(ctx, `
		DELETE FROM favorites
		WHERE user_id = $1 AND target_type = $2 AND target_id = $3
	`, userID, targetType, targetID)
	if err != nil {
		return favorite.MutationResult{}, internalStoreError()
	}
	return favorite.MutationResult{Favorited: false}, nil
}

func ensureFavoriteTargetPublic(ctx context.Context, q queryer, targetType, targetID string) *domain.AppError {
	var exists bool
	var err error
	switch targetType {
	case favorite.TargetCarpool:
		err = q.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM carpool_listings
				WHERE id = $1 AND status = 'active'
			)
		`, targetID).Scan(&exists)
	case favorite.TargetAPIService:
		err = q.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM api_services
				WHERE id = $1
				  AND `+publicAPIServiceOrderablePredicate("api_services")+`
			)
		`, targetID).Scan(&exists)
	default:
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Favorite validation failed", "收藏类型不支持。", "targetType", "invalid", "收藏类型不支持。")
	}
	if err != nil {
		return internalStoreError()
	}
	if !exists {
		return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Favorite target not found", "收藏目标不存在或当前不可见。")
	}
	return nil
}

func upsertFavoriteInTx(ctx context.Context, tx pgx.Tx, userID, targetType, targetID string, now time.Time) (favorite.MutationResult, *domain.AppError) {
	var id string
	err := tx.QueryRow(ctx, `
		INSERT INTO favorites (user_id, target_type, target_id, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, target_type, target_id) DO UPDATE
		SET created_at = favorites.created_at
		RETURNING id::text
	`, userID, targetType, targetID, now).Scan(&id)
	if err != nil {
		return favorite.MutationResult{}, internalStoreError()
	}
	item, err := scanFavoriteItem(ctx, tx, userID, targetType, targetID)
	if errors.Is(err, pgx.ErrNoRows) {
		return favorite.MutationResult{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Favorite target not found", "收藏目标不存在或当前不可见。")
	}
	if err != nil {
		return favorite.MutationResult{}, internalStoreError()
	}
	item.ID = id
	return favorite.MutationResult{Favorited: true, Favorite: &item}, nil
}

func scanFavoriteItem(ctx context.Context, q queryer, userID, targetType, targetID string) (favorite.ListItem, error) {
	return scanFavoriteRow(q.QueryRow(ctx, favoriteListBaseSQL+`
		AND target_type = $2 AND target_id = $3
	`, userID, targetType, targetID))
}

func scanFavoriteList(rows pgx.Rows) ([]favorite.ListItem, *domain.AppError) {
	items := []favorite.ListItem{}
	for rows.Next() {
		item, err := scanFavoriteRow(rows)
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

func scanFavoriteRow(row scanner) (favorite.ListItem, error) {
	var item favorite.ListItem
	err := row.Scan(
		&item.ID,
		&item.UserID,
		&item.TargetType,
		&item.TargetID,
		&item.CreatedAt,
		&item.Title,
		&item.Subtitle,
		&item.Status,
		&item.To,
	)
	return item, err
}

var favoriteListBaseSQL = `
WITH visible_favorites AS (
	SELECT
		f.id::text,
		f.user_id::text,
		f.target_type,
		f.target_id::text,
		f.created_at,
		l.title,
		('车源 · 月费 ¥' || l.price_monthly_cny::text) AS subtitle,
		l.status,
		('/carpools/' || l.id::text) AS target_to
	FROM favorites f
	JOIN carpool_listings l
	  ON f.target_type = 'carpool'
	 AND f.target_id = l.id
	 AND l.status = 'active'
	WHERE f.user_id = $1
	UNION ALL
	SELECT
		f.id::text,
		f.user_id::text,
		f.target_type,
		f.target_id::text,
		f.created_at,
		s.title,
		('API 服务 · ' || COALESCE(mp.display_name, u.display_name, u.username)) AS subtitle,
		s.publication_status AS status,
		('/api-market/' || s.id::text) AS target_to
	FROM favorites f
	JOIN api_services s
	  ON f.target_type = 'api_service'
	 AND f.target_id = s.id
	 AND ` + publicAPIServiceOrderablePredicate("s") + `
	JOIN users u ON u.id = s.owner_user_id
	LEFT JOIN merchant_profiles mp ON mp.id = s.merchant_profile_id AND mp.owner_user_id = s.owner_user_id
	WHERE f.user_id = $1
)
SELECT id, user_id, target_type, target_id, created_at, title, subtitle, status, target_to
FROM visible_favorites
WHERE true`
