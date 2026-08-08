package postgres

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/catalog"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/jackc/pgx/v5"
)

func (s *Store) AdminApplyAPIModelSyncWithIdempotency(
	ctx context.Context,
	entry idempotency.Entry,
	input catalog.APIModelSyncMutationInput,
	now time.Time,
	buildCompletion catalog.APIModelSyncCompletionBuilder,
) (catalog.APIModelBulkMutationResult, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil || buildCompletion == nil {
		return catalog.APIModelBulkMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return catalog.APIModelBulkMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existingEntry, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return catalog.APIModelBulkMutationResult{}, idempotency.Completion{}, appErr
	}
	items := append([]catalog.APIModelSyncSelection(nil), input.Items...)
	sort.Slice(items, func(i, j int) bool {
		if items[i].ProviderID != items[j].ProviderID {
			return items[i].ProviderID < items[j].ProviderID
		}
		return items[i].ModelKey < items[j].ModelKey
	})
	if appErr := validateAPIModelSyncItemsInTx(ctx, tx, items); appErr != nil {
		return catalog.APIModelBulkMutationResult{}, idempotency.Completion{}, appErr
	}

	var maxSortOrder int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(sort_order), 0) FROM api_model_catalog`).Scan(&maxSortOrder); err != nil {
		return catalog.APIModelBulkMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	nextSortOrder := (maxSortOrder/10 + 1) * 10
	result := catalog.APIModelBulkMutationResult{IDs: make([]string, 0, len(items))}
	for _, item := range items {
		if item.Status == catalog.APIModelSyncStatusNew {
			modelID, appErr := insertSyncedAPIModelInTx(ctx, tx, item, nextSortOrder, now)
			if appErr != nil {
				return catalog.APIModelBulkMutationResult{}, idempotency.Completion{}, appErr
			}
			nextSortOrder += 10
			result.Created++
			result.IDs = append(result.IDs, modelID)
			continue
		}
		if appErr := updateSyncedAPIModelPriceInTx(ctx, tx, item, now); appErr != nil {
			return catalog.APIModelBulkMutationResult{}, idempotency.Completion{}, appErr
		}
		result.Updated++
		result.IDs = append(result.IDs, item.LocalModelID)
	}

	completion, appErr := buildCompletion(result)
	if appErr != nil {
		return catalog.APIModelBulkMutationResult{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existingEntry, completion, now); appErr != nil {
		return catalog.APIModelBulkMutationResult{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return catalog.APIModelBulkMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	return result, completion, nil
}

func (s *Store) AdminSetAPIModelsActiveWithIdempotency(
	ctx context.Context,
	entry idempotency.Entry,
	input catalog.APIModelBulkStatusMutationInput,
	now time.Time,
	buildCompletion catalog.APIModelSyncCompletionBuilder,
) (catalog.APIModelBulkMutationResult, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil || buildCompletion == nil {
		return catalog.APIModelBulkMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return catalog.APIModelBulkMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existingEntry, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return catalog.APIModelBulkMutationResult{}, idempotency.Completion{}, appErr
	}
	modelIDs := append([]string(nil), input.ModelIDs...)
	lockedIDs, appErr := lockAPIModelsInTx(ctx, tx, modelIDs)
	if appErr != nil {
		return catalog.APIModelBulkMutationResult{}, idempotency.Completion{}, appErr
	}
	if len(lockedIDs) != len(modelIDs) {
		return catalog.APIModelBulkMutationResult{}, idempotency.Completion{}, apiModelNotFound()
	}

	result := catalog.APIModelBulkMutationResult{IDs: modelIDs}
	commandTag, err := tx.Exec(ctx, `
		UPDATE api_model_catalog
		SET active = $2, updated_at = $3
		WHERE id = ANY($1::uuid[]) AND active IS DISTINCT FROM $2
	`, modelIDs, input.Active, now)
	if err != nil {
		return catalog.APIModelBulkMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	result.Changed = int(commandTag.RowsAffected())
	completion, appErr := buildCompletion(result)
	if appErr != nil {
		return catalog.APIModelBulkMutationResult{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existingEntry, completion, now); appErr != nil {
		return catalog.APIModelBulkMutationResult{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return catalog.APIModelBulkMutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	return result, completion, nil
}

func validateAPIModelSyncItemsInTx(ctx context.Context, tx pgx.Tx, items []catalog.APIModelSyncSelection) *domain.AppError {
	providerCodes := make(map[string]string)
	for _, item := range items {
		code, ok := providerCodes[item.ProviderID]
		if !ok {
			err := tx.QueryRow(ctx, `
				SELECT code
				FROM api_model_providers
				WHERE id = $1
				FOR SHARE
			`, item.ProviderID).Scan(&code)
			if errors.Is(err, pgx.ErrNoRows) {
				return apiModelSyncVersionConflict()
			}
			if err != nil {
				return internalStoreError()
			}
			providerCodes[item.ProviderID] = code
		}
		if code != item.ProviderCode {
			return apiModelSyncVersionConflict()
		}
		if item.Status == catalog.APIModelSyncStatusNew {
			var existingID string
			err := tx.QueryRow(ctx, `
				SELECT id::text
				FROM api_model_catalog
				WHERE model_key = $1
				FOR UPDATE
			`, item.ModelKey).Scan(&existingID)
			if err == nil {
				return apiModelSyncVersionConflict()
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return internalStoreError()
			}
			continue
		}

		var providerID, modelKey string
		err := tx.QueryRow(ctx, `
			SELECT provider_id::text, model_key
			FROM api_model_catalog
			WHERE id = $1
			FOR UPDATE
		`, item.LocalModelID).Scan(&providerID, &modelKey)
		if errors.Is(err, pgx.ErrNoRows) {
			return apiModelSyncVersionConflict()
		}
		if err != nil {
			return internalStoreError()
		}
		if providerID != item.ProviderID || modelKey != item.ModelKey {
			return apiModelSyncVersionConflict()
		}
		var currentPriceVersionID string
		err = tx.QueryRow(ctx, `
			SELECT id::text
			FROM api_model_price_versions
			WHERE model_catalog_id = $1 AND valid_to IS NULL
			ORDER BY valid_from DESC
			LIMIT 1
			FOR UPDATE
		`, item.LocalModelID).Scan(&currentPriceVersionID)
		if errors.Is(err, pgx.ErrNoRows) || currentPriceVersionID != item.LocalPriceVersionID {
			return apiModelSyncVersionConflict()
		}
		if err != nil {
			return internalStoreError()
		}
	}
	return nil
}

func insertSyncedAPIModelInTx(ctx context.Context, tx pgx.Tx, item catalog.APIModelSyncSelection, sortOrder int, now time.Time) (string, *domain.AppError) {
	var modelID string
	err := tx.QueryRow(ctx, `
		INSERT INTO api_model_catalog (
		  provider_id, model_key, capabilities, active, sort_order, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id::text
	`, item.ProviderID, item.ModelKey, item.Capabilities, item.Active, sortOrder, now).Scan(&modelID)
	if err != nil {
		if isUniqueViolation(err) {
			return "", apiModelSyncVersionConflict()
		}
		return "", internalStoreError()
	}
	form := catalog.APIModelInput{
		ProviderID: item.ProviderID, ModelKey: item.ModelKey, Capabilities: item.Capabilities,
		InputTokenPrice: item.InputPricePerMillion, CachedInputTokenPrice: item.CachedInputPricePerMillion,
		OutputTokenPrice: item.OutputPricePerMillion, SourceURL: item.SourceURL,
		SourceVersion: item.SourceVersion, Active: item.Active, SortOrder: sortOrder,
	}
	if appErr := insertAPIModelPriceVersionAt(ctx, tx, modelID, form, now); appErr != nil {
		return "", appErr
	}
	return modelID, nil
}

func updateSyncedAPIModelPriceInTx(ctx context.Context, tx pgx.Tx, item catalog.APIModelSyncSelection, now time.Time) *domain.AppError {
	commandTag, err := tx.Exec(ctx, `
		UPDATE api_model_price_versions
		SET valid_to = $2
		WHERE id = $1 AND valid_to IS NULL
	`, item.LocalPriceVersionID, now)
	if err != nil {
		return internalStoreError()
	}
	if commandTag.RowsAffected() != 1 {
		return apiModelSyncVersionConflict()
	}
	form := catalog.APIModelInput{
		ProviderID: item.ProviderID, ModelKey: item.ModelKey, Capabilities: item.Capabilities,
		InputTokenPrice: item.InputPricePerMillion, CachedInputTokenPrice: item.CachedInputPricePerMillion,
		OutputTokenPrice: item.OutputPricePerMillion, SourceURL: item.SourceURL,
		SourceVersion: item.SourceVersion,
	}
	if appErr := insertAPIModelPriceVersionAt(ctx, tx, item.LocalModelID, form, now); appErr != nil {
		return appErr
	}
	if _, err := tx.Exec(ctx, `UPDATE api_model_catalog SET updated_at = $2 WHERE id = $1`, item.LocalModelID, now); err != nil {
		return internalStoreError()
	}
	return nil
}

func lockAPIModelsInTx(ctx context.Context, tx pgx.Tx, modelIDs []string) (map[string]bool, *domain.AppError) {
	rows, err := tx.Query(ctx, `
		SELECT id::text
		FROM api_model_catalog
		WHERE id = ANY($1::uuid[])
		ORDER BY id
		FOR UPDATE
	`, modelIDs)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	locked := make(map[string]bool, len(modelIDs))
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, internalStoreError()
		}
		locked[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, internalStoreError()
	}
	return locked, nil
}

func apiModelSyncVersionConflict() *domain.AppError {
	return domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Model catalog changed", "模型目录已变化，请重新获取同步预览。")
}
