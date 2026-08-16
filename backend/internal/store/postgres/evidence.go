package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/evidence"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Store) CreateReadyAssets(ctx context.Context, assets []evidence.Asset) *domain.AppError {
	if len(assets) == 0 || len(assets) > evidence.MaxFilesPerUpload {
		return evidenceValidationError("files", "invalid_count", "每次必须上传 1 至 3 张图片。")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return internalStoreError()
	}
	defer rollback(ctx, tx)

	orderID := strings.TrimSpace(assets[0].APIOrderID)
	uploaderID := strings.TrimSpace(assets[0].UploaderUserID)
	var participant bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM api_orders
			WHERE id = $1 AND (buyer_user_id = $2 OR seller_user_id = $2)
		)
	`, orderID, uploaderID).Scan(&participant); err != nil {
		return internalStoreError()
	}
	if !participant {
		return evidenceNotFound()
	}
	for _, asset := range assets {
		if asset.APIOrderID != orderID || asset.UploaderUserID != uploaderID || !evidence.IsAllowedKind(asset.Kind) ||
			asset.Status != "ready" || asset.ObjectKey == "" || asset.ReadyAt == nil || asset.UnboundExpiresAt == nil {
			return evidenceValidationError("files", "invalid", "图片证据元数据无效。")
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO api_order_evidence_assets (
				id, api_order_id, uploader_user_id, kind, object_key, output_mime,
				byte_size, width, height, sha256, scan_status, status,
				ready_at, unbound_expires_at, created_at, updated_at, version
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'passed', 'ready', $11, $12, $13, $13, 1)
		`, asset.ID, asset.APIOrderID, asset.UploaderUserID, asset.Kind, asset.ObjectKey, asset.OutputMIME,
			asset.ByteSize, asset.Width, asset.Height, asset.SHA256[:], *asset.ReadyAt, *asset.UnboundExpiresAt, asset.CreatedAt); err != nil {
			return internalStoreError()
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) AuthorizedAsset(ctx context.Context, assetID, viewerUserID string, admin bool) (evidence.Asset, *domain.AppError) {
	return authorizedEvidenceAsset(ctx, s.pool, assetID, viewerUserID, admin)
}

func authorizedEvidenceAsset(ctx context.Context, q queryer, assetID, viewerUserID string, admin bool) (evidence.Asset, *domain.AppError) {
	var item evidence.Asset
	var sha []byte
	err := q.QueryRow(ctx, `
		SELECT asset.id::text, asset.api_order_id::text, asset.uploader_user_id::text,
		       asset.kind, asset.object_key, asset.output_mime, asset.byte_size,
		       asset.width, asset.height, asset.sha256, asset.status, asset.created_at,
		       asset.ready_at, asset.unbound_expires_at, asset.destroyed_at, asset.destroy_reason,
		       asset.version
		FROM api_order_evidence_assets asset
		JOIN api_order_evidence_bindings binding ON binding.asset_id = asset.id
		JOIN api_orders order_row ON order_row.id = asset.api_order_id
		WHERE asset.id = $1
		  AND asset.status = 'ready'
		  AND asset.scan_status = 'passed'
		  AND asset.object_key IS NOT NULL
		  AND (
		    $3::boolean
		    OR (binding.visibility = 'participants_admin' AND $2::uuid IN (order_row.buyer_user_id, order_row.seller_user_id))
		    OR (binding.visibility IN ('submitter_admin', 'appellant_admin') AND asset.uploader_user_id = $2)
		  )
	`, assetID, nullUUID(viewerUserID), admin).Scan(
		&item.ID, &item.APIOrderID, &item.UploaderUserID, &item.Kind, &item.ObjectKey,
		&item.OutputMIME, &item.ByteSize, &item.Width, &item.Height, &sha, &item.Status,
		&item.CreatedAt, &item.ReadyAt, &item.UnboundExpiresAt, &item.DestroyedAt, &item.DestroyReason,
		&item.Version,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return evidence.Asset{}, evidenceNotFound()
	}
	if err != nil || len(sha) != len(item.SHA256) {
		return evidence.Asset{}, internalStoreError()
	}
	copy(item.SHA256[:], sha)
	return item, nil
}

func (s *Store) QuarantineAssetWithIdempotency(
	ctx context.Context,
	entry idempotency.Entry,
	input evidence.AdminQuarantineInput,
	now time.Time,
	buildCompletion evidence.AdminQuarantineCompletionBuilder,
) (evidence.AdminQuarantineResult, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return evidence.AdminQuarantineResult{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return evidence.AdminQuarantineResult{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	existingEntry, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return evidence.AdminQuarantineResult{}, idempotency.Completion{}, appErr
	}

	var assetID, disputeCaseID, status string
	var version int64
	err = tx.QueryRow(ctx, `
		SELECT asset.id::text, binding.dispute_case_id::text, asset.status, asset.version
		FROM api_order_evidence_assets asset
		JOIN api_order_evidence_bindings binding ON binding.asset_id = asset.id
		JOIN dispute_cases dispute ON dispute.id = binding.dispute_case_id
		WHERE asset.id = $1 AND dispute.target_type = 'api_order'
		FOR UPDATE OF asset, binding, dispute
	`, input.AssetID).Scan(&assetID, &disputeCaseID, &status, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return evidence.AdminQuarantineResult{}, idempotency.Completion{}, evidenceNotFound()
	}
	if err != nil {
		return evidence.AdminQuarantineResult{}, idempotency.Completion{}, internalStoreError()
	}
	if version != input.ExpectedVersion {
		return evidence.AdminQuarantineResult{}, idempotency.Completion{}, evidenceVersionConflict()
	}
	if status != "ready" {
		return evidence.AdminQuarantineResult{}, idempotency.Completion{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Evidence cannot be quarantined", "图片证据已隔离或已进入销毁流程。")
	}

	expiresAt := now.Add(evidence.QuarantineRetention)
	result := evidence.AdminQuarantineResult{ID: assetID, Status: "quarantined", QuarantinedExpiresAt: expiresAt}
	err = tx.QueryRow(ctx, `
		UPDATE api_order_evidence_assets
		SET scan_status = 'rejected', status = 'quarantined', ready_at = NULL,
		    unbound_expires_at = NULL, quarantined_expires_at = $2,
		    updated_at = $1, version = version + 1
		WHERE id = $3 AND status = 'ready' AND version = $4
		RETURNING version
	`, now, expiresAt, assetID, input.ExpectedVersion).Scan(&result.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return evidence.AdminQuarantineResult{}, idempotency.Completion{}, evidenceVersionConflict()
	}
	if err != nil {
		return evidence.AdminQuarantineResult{}, idempotency.Completion{}, internalStoreError()
	}
	if appErr := insertEvidenceQuarantineAudit(ctx, tx, input, disputeCaseID, result, now); appErr != nil {
		return evidence.AdminQuarantineResult{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(result)
	if appErr != nil {
		return evidence.AdminQuarantineResult{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existingEntry, completion, now); appErr != nil {
		return evidence.AdminQuarantineResult{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return evidence.AdminQuarantineResult{}, idempotency.Completion{}, internalStoreError()
	}
	return result, completion, nil
}

func insertEvidenceQuarantineAudit(ctx context.Context, tx pgx.Tx, input evidence.AdminQuarantineInput, disputeCaseID string, result evidence.AdminQuarantineResult, now time.Time) *domain.AppError {
	metadata, err := json.Marshal(map[string]any{
		"disputeCaseId": disputeCaseID,
		"status":        result.Status,
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
		VALUES ($1, 'api_order_evidence', $2, 'api_order_evidence.quarantined', $3, 'admin', $4, $5, $6, $7)
	`, eventID, result.ID, input.AdminUserID, result.Version, input.RequestID, metadata, now); err != nil {
		return internalStoreError()
	}
	beforeJSON, err := json.Marshal(map[string]any{"id": result.ID, "status": "ready", "version": result.Version - 1})
	if err != nil {
		return internalStoreError()
	}
	afterJSON, err := json.Marshal(map[string]any{
		"id": result.ID, "status": result.Status, "version": result.Version,
		"quarantinedExpiresAt": result.QuarantinedExpiresAt,
	})
	if err != nil {
		return internalStoreError()
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO admin_audit_logs (
			admin_user_id, action, target_type, target_id, reason,
			before_json, after_json, request_id, created_at
		)
		VALUES ($1, 'api_order_evidence.quarantined', 'api_order_evidence', $2, $3, $4, $5, $6, $7)
	`, input.AdminUserID, result.ID, input.Reason, beforeJSON, afterJSON, input.RequestID, now); err != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) ClaimDestroyCandidates(ctx context.Context, now time.Time, batchSize int) ([]evidence.DestroyCandidate, *domain.AppError) {
	if batchSize < 1 {
		return nil, evidenceValidationError("batchSize", "invalid", "清理批次必须大于零。")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rollback(ctx, tx)
	items, appErr := claimEvidenceDestroyCandidatesInTx(ctx, tx, now, batchSize)
	if appErr != nil {
		return nil, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, internalStoreError()
	}
	return items, nil
}

func claimEvidenceDestroyCandidatesInTx(ctx context.Context, tx pgx.Tx, now time.Time, batchSize int) ([]evidence.DestroyCandidate, *domain.AppError) {
	if batchSize < 1 {
		return nil, evidenceValidationError("batchSize", "invalid", "清理批次必须大于零。")
	}
	rows, err := tx.Query(ctx, `
		WITH candidates AS (
			SELECT asset.id, asset.object_key,
			       CASE
			         WHEN asset.status = 'destroy_pending' THEN asset.destroy_reason
			         WHEN asset.status = 'quarantined' THEN 'quarantine_retention_expired'
			         WHEN binding.asset_id IS NULL THEN 'unbound_retention_expired'
			         ELSE 'terminal_retention_expired'
			       END AS reason
			FROM api_order_evidence_assets asset
			LEFT JOIN api_order_evidence_bindings binding ON binding.asset_id = asset.id
			LEFT JOIN dispute_cases dispute ON dispute.id = binding.dispute_case_id
			WHERE asset.object_key IS NOT NULL
			  AND (
			    asset.status = 'destroy_pending'
			    OR (asset.status = 'ready' AND binding.asset_id IS NULL AND asset.unbound_expires_at <= $1)
			    OR (asset.status = 'quarantined' AND asset.quarantined_expires_at <= $1)
			    OR (
			      asset.status = 'ready' AND binding.asset_id IS NOT NULL
			      AND dispute.active = false
			      AND NOT EXISTS (
			        SELECT 1 FROM api_order_dispute_remedies remedy
			        WHERE remedy.dispute_case_id = dispute.id
			          AND remedy.status IN ('pending', 'claimed_fulfilled')
			      )
			      AND NOT EXISTS (
			        SELECT 1 FROM appeals appeal
			        WHERE appeal.dispute_case_id = dispute.id AND appeal.status = 'submitted'
			      )
			      AND GREATEST(
			        COALESCE(dispute.closed_at, dispute.resolved_at, dispute.updated_at),
			        COALESCE((
			          SELECT max(COALESCE(appeal.handled_at, appeal.updated_at))
			          FROM appeals appeal WHERE appeal.dispute_case_id = dispute.id
			        ), '-infinity'::timestamptz)
			      ) <= $1 - interval '90 days'
			    )
			  )
			ORDER BY COALESCE(asset.destroy_requested_at, asset.unbound_expires_at, asset.quarantined_expires_at, asset.created_at), asset.id
			LIMIT $2
			FOR UPDATE OF asset SKIP LOCKED
		), updated AS (
			UPDATE api_order_evidence_assets asset
			SET status = 'destroy_pending', destroy_requested_at = COALESCE(asset.destroy_requested_at, $1),
			    destroy_reason = candidates.reason, updated_at = $1, version = version + 1
			FROM candidates
			WHERE asset.id = candidates.id
			RETURNING asset.id::text, asset.object_key
		)
		SELECT id, object_key FROM updated
	`, now, batchSize)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	items := make([]evidence.DestroyCandidate, 0)
	for rows.Next() {
		var item evidence.DestroyCandidate
		if err := rows.Scan(&item.ID, &item.ObjectKey); err != nil {
			return nil, internalStoreError()
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, internalStoreError()
	}
	return items, nil
}

func (s *Store) MarkDestroyed(ctx context.Context, assetID string, now time.Time) *domain.AppError {
	command, err := s.pool.Exec(ctx, `
		UPDATE api_order_evidence_assets
		SET status = 'destroyed', object_key = NULL, destroyed_at = $2,
		    updated_at = $2, version = version + 1
		WHERE id = $1 AND status = 'destroy_pending' AND destroyed_at IS NULL
	`, assetID, now)
	if err != nil {
		return internalStoreError()
	}
	if command.RowsAffected() != 1 {
		return evidenceNotFound()
	}
	return nil
}

func evidenceNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Evidence not found", "图片证据不存在或当前账号无权查看。")
}

func evidenceValidationError(field, code, message string) *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid evidence", message, field, code, message)
}

func evidenceVersionConflict() *domain.AppError {
	return domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "图片证据状态已变化，请刷新后重试。")
}
