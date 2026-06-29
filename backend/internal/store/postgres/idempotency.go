package postgres

import (
	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"net/http"
	"time"
)

func (s *Store) BeginIdempotency(ctx context.Context, entry idempotency.Entry) (*idempotency.Entry, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rollback(ctx, tx)

	var existing idempotency.Entry
	err = tx.QueryRow(ctx, `
		SELECT user_id::text, route_key, idempotency_key, request_hash, status, COALESCE(response_status, 0),
		       COALESCE(response_content_type, ''), COALESCE(response_body_json, 'null'::jsonb), COALESCE(resource_type, ''),
		       COALESCE(resource_id::text, ''), created_at, completed_at, expires_at
		FROM idempotency_keys
		WHERE user_id = $1 AND route_key = $2 AND idempotency_key = $3
		FOR UPDATE
	`, entry.UserID, entry.RouteKey, entry.Key).Scan(
		&existing.UserID,
		&existing.RouteKey,
		&existing.Key,
		&existing.RequestHash,
		&existing.State,
		&existing.Status,
		&existing.ContentType,
		&existing.Body,
		&existing.ResourceType,
		&existing.ResourceID,
		&existing.CreatedAt,
		&existing.CompletedAt,
		&existing.ExpiresAt,
	)
	if err == nil {
		if existing.RequestHash != entry.RequestHash {
			if err := tx.Commit(ctx); err != nil {
				return nil, internalStoreError()
			}
			return nil, domain.NewError(http.StatusConflict, domain.CodeIdempotencyKeyReused, "Idempotency key reused", "同一个 Idempotency-Key 不能用于不同请求。")
		}
		if existing.State == "completed" {
			if err := tx.Commit(ctx); err != nil {
				return nil, internalStoreError()
			}
			return &existing, nil
		}
		if entry.CreatedAt.After(existing.ExpiresAt) {
			_, err = tx.Exec(ctx, `
				UPDATE idempotency_keys
				SET request_hash = $4,
				    status = 'processing',
				    response_status = NULL,
				    response_content_type = NULL,
				    response_body_json = NULL,
				    response_body_cache_allowed = true,
				    resource_type = NULL,
				    resource_id = NULL,
				    completed_at = NULL,
				    created_at = $5,
				    expires_at = $6
				WHERE user_id = $1 AND route_key = $2 AND idempotency_key = $3
			`, entry.UserID, entry.RouteKey, entry.Key, entry.RequestHash, entry.CreatedAt, entry.ExpiresAt)
			if err != nil {
				return nil, internalStoreError()
			}
			if err := tx.Commit(ctx); err != nil {
				return nil, internalStoreError()
			}
			return &entry, nil
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, internalStoreError()
		}
		return nil, domain.NewError(http.StatusConflict, domain.CodeIdempotencyInProgress, "Idempotency request in progress", "相同幂等请求仍在处理中。")
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, internalStoreError()
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO idempotency_keys (user_id, route_key, idempotency_key, request_hash, status, created_at, expires_at)
		VALUES ($1, $2, $3, $4, 'processing', $5, $6)
	`, entry.UserID, entry.RouteKey, entry.Key, entry.RequestHash, entry.CreatedAt, entry.ExpiresAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, domain.NewError(http.StatusConflict, domain.CodeIdempotencyInProgress, "Idempotency request in progress", "相同幂等请求仍在处理中。")
		}
		return nil, internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, internalStoreError()
	}
	return &entry, nil
}

func (s *Store) CleanupExpiredIdempotency(ctx context.Context, before time.Time) *domain.AppError {
	if s == nil || s.pool == nil {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
		DELETE FROM idempotency_keys
		WHERE status = 'processing' AND expires_at < $1
	`, before)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) CompleteIdempotency(ctx context.Context, entry *idempotency.Entry, status int, contentType string, body []byte, resourceType, resourceID string, completedAt time.Time) *domain.AppError {
	if s == nil || s.pool == nil || entry == nil {
		return nil
	}
	var bodyJSON json.RawMessage
	if len(body) > 0 {
		bodyJSON = append(bodyJSON, body...)
	} else {
		bodyJSON = json.RawMessage(`null`)
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE idempotency_keys
		SET status = 'completed',
		    response_status = $4,
		    response_content_type = $5,
		    response_body_json = $6,
		    response_body_cache_allowed = true,
		    resource_type = $7,
		    resource_id = $8,
		    completed_at = $9
		WHERE user_id = $1 AND route_key = $2 AND idempotency_key = $3
	`, entry.UserID, entry.RouteKey, entry.Key, status, contentType, bodyJSON, resourceType, nullUUID(resourceID), completedAt)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) CancelIdempotency(ctx context.Context, entry *idempotency.Entry) *domain.AppError {
	if s == nil || s.pool == nil || entry == nil {
		return nil
	}
	_, err := s.pool.Exec(ctx, `
		DELETE FROM idempotency_keys
		WHERE user_id = $1 AND route_key = $2 AND idempotency_key = $3 AND status = 'processing'
	`, entry.UserID, entry.RouteKey, entry.Key)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func lockProcessingIdempotencyInTx(ctx context.Context, tx pgx.Tx, entry idempotency.Entry) (idempotency.Entry, *domain.AppError) {
	var existing idempotency.Entry
	err := tx.QueryRow(ctx, `
		SELECT user_id::text, route_key, idempotency_key, request_hash, status, COALESCE(response_status, 0),
		       COALESCE(response_content_type, ''), COALESCE(response_body_json, 'null'::jsonb), COALESCE(resource_type, ''),
		       COALESCE(resource_id::text, ''), created_at, completed_at, expires_at
		FROM idempotency_keys
		WHERE user_id = $1 AND route_key = $2 AND idempotency_key = $3
		FOR UPDATE
	`, entry.UserID, entry.RouteKey, entry.Key).Scan(
		&existing.UserID,
		&existing.RouteKey,
		&existing.Key,
		&existing.RequestHash,
		&existing.State,
		&existing.Status,
		&existing.ContentType,
		&existing.Body,
		&existing.ResourceType,
		&existing.ResourceID,
		&existing.CreatedAt,
		&existing.CompletedAt,
		&existing.ExpiresAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return idempotency.Entry{}, internalStoreError()
	}
	if err != nil {
		return idempotency.Entry{}, internalStoreError()
	}
	if existing.RequestHash != entry.RequestHash {
		return idempotency.Entry{}, domain.NewError(http.StatusConflict, domain.CodeIdempotencyKeyReused, "Idempotency key reused", "同一个 Idempotency-Key 不能用于不同请求。")
	}
	if existing.State != "processing" {
		return idempotency.Entry{}, domain.NewError(http.StatusConflict, domain.CodeIdempotencyInProgress, "Idempotency request in progress", "相同幂等请求仍在处理中。")
	}
	if entry.CreatedAt.After(existing.ExpiresAt) {
		_, err = tx.Exec(ctx, `
			UPDATE idempotency_keys
			SET request_hash = $4,
			    status = 'processing',
			    response_status = NULL,
			    response_content_type = NULL,
			    response_body_json = NULL,
			    response_body_cache_allowed = true,
			    resource_type = NULL,
			    resource_id = NULL,
			    completed_at = NULL,
			    created_at = $5,
			    expires_at = $6
			WHERE user_id = $1 AND route_key = $2 AND idempotency_key = $3
		`, entry.UserID, entry.RouteKey, entry.Key, entry.RequestHash, entry.CreatedAt, entry.ExpiresAt)
		if err != nil {
			return idempotency.Entry{}, internalStoreError()
		}
		return entry, nil
	}
	return existing, nil
}

func completeIdempotencyInTx(ctx context.Context, tx pgx.Tx, entry idempotency.Entry, completion idempotency.Completion, completedAt time.Time) *domain.AppError {
	var bodyJSON any
	cacheBody := !completion.SkipBodyCache
	if !completion.SkipBodyCache {
		raw := json.RawMessage(`null`)
		if len(completion.Body) > 0 {
			raw = append(json.RawMessage(nil), completion.Body...)
		}
		bodyJSON = raw
	}
	_, err := tx.Exec(ctx, `
		UPDATE idempotency_keys
		SET status = 'completed',
		    response_status = $4,
		    response_content_type = $5,
		    response_body_json = $6,
		    response_body_cache_allowed = $10,
		    resource_type = $7,
		    resource_id = $8,
		    completed_at = $9
		WHERE user_id = $1
		  AND route_key = $2
		  AND idempotency_key = $3
		  AND status = 'processing'
	`, entry.UserID, entry.RouteKey, entry.Key, completion.Status, completion.ContentType, bodyJSON,
		completion.ResourceType, nullUUID(completion.ResourceID), completedAt, cacheBody)
	if err != nil {
		return internalStoreError()
	}
	return nil
}
