package postgres

import (
	"context"
	"net/http"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/evidence"

	"github.com/jackc/pgx/v5"
)

func bindEvidenceAssetsInTx(ctx context.Context, tx pgx.Tx, input evidence.BindingInput, now time.Time) *domain.AppError {
	if len(input.AssetIDs) == 0 {
		return nil
	}
	if len(input.AssetIDs) > evidence.MaxFilesPerUpload {
		return evidenceValidationError("evidenceAssetIds", "too_many", "每次最多绑定 3 张图片。")
	}
	assetIDs := make([]string, 0, len(input.AssetIDs))
	seen := make(map[string]struct{}, len(input.AssetIDs))
	for _, raw := range input.AssetIDs {
		id := strings.TrimSpace(raw)
		if id == "" {
			return evidenceValidationError("evidenceAssetIds", "invalid", "证据资产 ID 无效。")
		}
		if _, exists := seen[id]; exists {
			return evidenceValidationError("evidenceAssetIds", "duplicate", "同一张图片不能重复绑定。")
		}
		seen[id] = struct{}{}
		assetIDs = append(assetIDs, id)
	}

	var caseOrderID string
	if err := tx.QueryRow(ctx, `
		SELECT api_order_id::text
		FROM dispute_cases
		WHERE id = $1
		FOR UPDATE
	`, input.DisputeCaseID).Scan(&caseOrderID); err == pgx.ErrNoRows {
		return evidenceNotFound()
	} else if err != nil {
		return internalStoreError()
	}
	if caseOrderID != input.APIOrderID {
		return evidenceNotFound()
	}
	if appErr := validateEvidenceBindingSourceInTx(ctx, tx, input); appErr != nil {
		return appErr
	}

	var existingCount int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM api_order_evidence_bindings WHERE dispute_case_id = $1`, input.DisputeCaseID).Scan(&existingCount); err != nil {
		return internalStoreError()
	}
	if existingCount+len(assetIDs) > evidence.MaxAssetsPerCase {
		return evidenceValidationError("evidenceAssetIds", "case_limit", "单个纠纷最多保存 20 张图片证据。")
	}

	rows, err := tx.Query(ctx, `
		SELECT asset.id::text
		FROM api_order_evidence_assets asset
		LEFT JOIN api_order_evidence_bindings binding ON binding.asset_id = asset.id
		WHERE asset.id = ANY($1::uuid[])
		  AND asset.api_order_id = $2
		  AND asset.uploader_user_id = $3
		  AND asset.status = 'ready'
		  AND asset.scan_status = 'passed'
		  AND asset.unbound_expires_at > $4
		  AND binding.asset_id IS NULL
		ORDER BY asset.id
		FOR UPDATE OF asset
	`, assetIDs, input.APIOrderID, input.UploaderID, now)
	if err != nil {
		return internalStoreError()
	}
	locked := make(map[string]struct{}, len(assetIDs))
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return internalStoreError()
		}
		locked[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return internalStoreError()
	}
	rows.Close()
	if len(locked) != len(assetIDs) {
		return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Evidence cannot be bound", "图片已过期、已绑定，或不属于当前订单和提交人。")
	}

	disputeMessageID, infoSupplementID, remedyID, appealID := any(nil), any(nil), any(nil), any(nil)
	switch input.SourceType {
	case evidence.SourceDisputeMessage:
		disputeMessageID = input.SourceID
	case evidence.SourceInfoSupplement:
		infoSupplementID = input.SourceID
	case evidence.SourceDisputeRemedy:
		remedyID = input.SourceID
	case evidence.SourceAppeal:
		appealID = input.SourceID
	case evidence.SourceDisputeCase:
	default:
		return evidenceValidationError("evidenceAssetIds", "invalid_source", "证据绑定来源无效。")
	}
	for _, assetID := range assetIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO api_order_evidence_bindings (
				asset_id, dispute_case_id, visibility, usage, source_type, source_id,
				dispute_message_id, info_supplement_id, dispute_remedy_id, appeal_id, created_at
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		`, assetID, input.DisputeCaseID, input.Visibility, input.Usage, input.SourceType, input.SourceID,
			disputeMessageID, infoSupplementID, remedyID, appealID, now); err != nil {
			return internalStoreError()
		}
	}
	return nil
}

func validateEvidenceBindingSourceInTx(ctx context.Context, tx pgx.Tx, input evidence.BindingInput) *domain.AppError {
	validShape := false
	var sourceMatches bool
	switch input.SourceType {
	case evidence.SourceDisputeCase:
		validShape = (input.Usage == evidence.UsageDisputeInitial || input.Usage == evidence.UsagePlatformEscalation || input.Usage == evidence.UsageFormalResponse) && input.Visibility == evidence.VisibilityParticipantsAdmin
		sourceMatches = input.SourceID == input.DisputeCaseID
	case evidence.SourceDisputeMessage:
		validShape = input.Usage == evidence.UsageMessage && input.Visibility == evidence.VisibilityParticipantsAdmin
		if validShape {
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM api_order_dispute_messages WHERE id = $1 AND dispute_case_id = $2)`, input.SourceID, input.DisputeCaseID).Scan(&sourceMatches); err != nil {
				return internalStoreError()
			}
		}
	case evidence.SourceInfoSupplement:
		validShape = input.Usage == evidence.UsageInfoSupplement && input.Visibility == evidence.VisibilitySubmitterAdmin
		if validShape {
			if err := tx.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT 1
					FROM moderation_info_supplements supplement
					JOIN moderation_info_requests request_row ON request_row.id = supplement.info_request_id
					WHERE supplement.id = $1
					  AND request_row.entity_type = 'dispute'
					  AND request_row.dispute_case_id = $2
				)
			`, input.SourceID, input.DisputeCaseID).Scan(&sourceMatches); err != nil {
				return internalStoreError()
			}
		}
	case evidence.SourceDisputeRemedy:
		validShape = (input.Usage == evidence.UsageRemedyClaim || input.Usage == evidence.UsageRemedyContest) && input.Visibility == evidence.VisibilityParticipantsAdmin
		if validShape {
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM api_order_dispute_remedies WHERE id = $1 AND dispute_case_id = $2)`, input.SourceID, input.DisputeCaseID).Scan(&sourceMatches); err != nil {
				return internalStoreError()
			}
		}
	case evidence.SourceAppeal:
		validShape = input.Usage == evidence.UsageAppeal && input.Visibility == evidence.VisibilityAppellantAdmin
		if validShape {
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM appeals WHERE id = $1 AND dispute_case_id = $2)`, input.SourceID, input.DisputeCaseID).Scan(&sourceMatches); err != nil {
				return internalStoreError()
			}
		}
	}
	if !validShape {
		return evidenceValidationError("evidenceAssetIds", "invalid_source", "证据绑定来源无效。")
	}
	if !sourceMatches {
		return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Evidence source mismatch", "证据绑定来源不属于当前纠纷。")
	}
	return nil
}

func listEvidenceReferences(ctx context.Context, q queryer, disputeID string) ([]evidence.Reference, error) {
	rows, err := queryRows(ctx, q, `
		SELECT asset.id::text, asset.uploader_user_id::text, asset.kind, asset.output_mime,
		       asset.byte_size, asset.width, asset.height, asset.created_at, asset.version,
		       binding.visibility, binding.usage, binding.source_type, binding.source_id::text
		FROM api_order_evidence_bindings binding
		JOIN api_order_evidence_assets asset ON asset.id = binding.asset_id
		WHERE binding.dispute_case_id = $1
		  AND asset.status = 'ready'
		  AND asset.scan_status = 'passed'
		ORDER BY binding.created_at, binding.asset_id
	`, disputeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]evidence.Reference, 0)
	for rows.Next() {
		var item evidence.Reference
		if err := rows.Scan(
			&item.ID, &item.UploaderUserID, &item.Kind, &item.MIME,
			&item.ByteSize, &item.Width, &item.Height, &item.CreatedAt, &item.Version,
			&item.Visibility, &item.Usage, &item.SourceType, &item.SourceID,
		); err != nil {
			return nil, err
		}
		item.ContentPath = "/api/v1/me/dispute-evidence/" + item.ID + "/content"
		items = append(items, item)
	}
	return items, rows.Err()
}

func listAppealEvidenceReferences(ctx context.Context, q queryer, appealID string) ([]evidence.Reference, error) {
	rows, err := queryRows(ctx, q, `
		SELECT asset.id::text, asset.uploader_user_id::text, asset.kind, asset.output_mime,
		       asset.byte_size, asset.width, asset.height, asset.created_at, asset.version,
		       binding.visibility, binding.usage, binding.source_type, binding.source_id::text
		FROM api_order_evidence_bindings binding
		JOIN api_order_evidence_assets asset ON asset.id = binding.asset_id
		WHERE binding.appeal_id = $1
		  AND asset.status = 'ready'
		  AND asset.scan_status = 'passed'
		ORDER BY binding.created_at, binding.asset_id
	`, appealID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]evidence.Reference, 0)
	for rows.Next() {
		var item evidence.Reference
		if err := rows.Scan(&item.ID, &item.UploaderUserID, &item.Kind, &item.MIME,
			&item.ByteSize, &item.Width, &item.Height, &item.CreatedAt, &item.Version,
			&item.Visibility, &item.Usage, &item.SourceType, &item.SourceID); err != nil {
			return nil, err
		}
		item.ContentPath = "/api/v1/me/dispute-evidence/" + item.ID + "/content"
		items = append(items, item)
	}
	return items, rows.Err()
}
