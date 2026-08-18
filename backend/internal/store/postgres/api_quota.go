package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apiintent"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/apiquota"
	"c2c-market/backend/internal/module/contact"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var apiQuotaBatchColumns = `
	b.id::text, b.api_service_id::text, b.owner_user_id::text,
	s.title, s.distribution_system, (` + apiServiceFulfillmentReadyPredicate("s") + `),
	COALESCE(s.declared_ttft_band, ''), COALESCE(s.declared_max_concurrency, 0), s.performance_confirmed_at,
	s.prompt_audit_enabled,
	b.source_type, COALESCE(b.source_label, ''), b.status,
	b.declared_total_usd_allowance::text, b.unallocated_usd_allowance::text,
	b.sale_cutoff_at, b.expires_at, b.source_confirmed_at, b.published_at,
	b.created_at, b.updated_at, b.version
`

const apiQuotaOfferColumns = `
		o.id::text, o.batch_id::text, o.api_service_id::text, o.owner_user_id::text,
	COALESCE(o.previous_version_id::text, ''), o.distribution_system, o.name,
	o.usd_allowance::text, o.price_cny::text,
	(price_cny / usd_allowance)::numeric(18,6)::text,
	o.model_multiplier::text,
	o.five_hour_limit_mode, COALESCE(o.five_hour_limit_usd::text, ''),
	o.daily_limit_mode, COALESCE(o.daily_limit_usd::text, ''),
	o.delivery_mode, o.delivery_eta_minutes,
	o.sale_mode, o.status, o.sort_order, o.published_at,
		o.created_at, o.updated_at, o.version
`

const apiQuotaUnitPriceExpression = `(o.price_cny / o.usd_allowance)`

type quotaEventVersion struct {
	id      string
	ownerID string
	version int64
}

func executeAPIQuotaCommand[T any](
	ctx context.Context,
	s *Store,
	entry *idempotency.Entry,
	now time.Time,
	mutate func(pgx.Tx) (T, *domain.AppError),
	buildCompletion func(T) (idempotency.Completion, *domain.AppError),
) (T, idempotency.Completion, *domain.AppError) {
	var zero T
	if s == nil || s.pool == nil {
		return zero, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return zero, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	var processing idempotency.Entry
	hasProcessing := false
	if entry != nil {
		if buildCompletion == nil {
			return zero, idempotency.Completion{}, internalStoreError()
		}
		var appErr *domain.AppError
		processing, appErr = lockProcessingIdempotencyInTx(ctx, tx, *entry)
		if appErr != nil {
			return zero, idempotency.Completion{}, appErr
		}
		hasProcessing = true
	}
	result, appErr := mutate(tx)
	if appErr != nil {
		return zero, idempotency.Completion{}, appErr
	}
	completion := idempotency.Completion{}
	if hasProcessing {
		completion, appErr = buildCompletion(result)
		if appErr != nil {
			return zero, idempotency.Completion{}, appErr
		}
		if appErr := completeIdempotencyInTx(ctx, tx, processing, completion, now); appErr != nil {
			return zero, idempotency.Completion{}, appErr
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return zero, idempotency.Completion{}, internalStoreError()
	}
	return result, completion, nil
}

func (s *Store) CreateAPIQuotaBatch(ctx context.Context, batch apiquota.Batch, requestID string) (apiquota.Batch, *domain.AppError) {
	result, _, appErr := executeAPIQuotaCommand(ctx, s, nil, batch.CreatedAt, func(tx pgx.Tx) (apiquota.Batch, *domain.AppError) {
		return s.createAPIQuotaBatchInTx(ctx, tx, batch, requestID)
	}, nil)
	return result, appErr
}

func (s *Store) CreateAPIQuotaBatchWithIdempotency(ctx context.Context, entry idempotency.Entry, batch apiquota.Batch, requestID string, now time.Time, buildCompletion apiquota.BatchCompletionBuilder) (apiquota.Batch, idempotency.Completion, *domain.AppError) {
	return executeAPIQuotaCommand(ctx, s, &entry, now, func(tx pgx.Tx) (apiquota.Batch, *domain.AppError) {
		return s.createAPIQuotaBatchInTx(ctx, tx, batch, requestID)
	}, buildCompletion)
}

func (s *Store) createAPIQuotaBatchInTx(ctx context.Context, tx pgx.Tx, batch apiquota.Batch, requestID string) (apiquota.Batch, *domain.AppError) {
	_, err := tx.Exec(ctx, `
		INSERT INTO api_quota_batches (
			id, api_service_id, owner_user_id, source_type, source_label, status,
			declared_total_usd_allowance, unallocated_usd_allowance,
			sale_cutoff_at, expires_at, source_confirmed_at,
			created_at, updated_at, version
		) VALUES (
			$1, $2, $3, $4, $5, 'draft', $6, $6, $7, $8, $9, $10, $10, 1
		)
	`, batch.ID, batch.APIServiceID, batch.OwnerUserID, batch.SourceType, nullText(batch.SourceLabel),
		batch.DeclaredTotalUSDAllowance, batch.SaleCutoffAt, batch.ExpiresAt, batch.SourceConfirmedAt, batch.CreatedAt)
	if err != nil {
		return apiquota.Batch{}, mapAPIQuotaWriteError(err)
	}
	if appErr := insertAPIOperationDomainEvent(ctx, tx, "api_quota_batch", batch.ID, "api_quota_batch.created", batch.OwnerUserID, apiOperationActorUser, batch.Version, requestID, map[string]any{
		"status": batch.Status,
	}, batch.CreatedAt); appErr != nil {
		return apiquota.Batch{}, appErr
	}
	return batch, nil
}

func (s *Store) GetAPIQuotaBatchForOwner(ctx context.Context, ownerUserID, batchID string) (apiquota.Batch, *domain.AppError) {
	if s == nil || s.pool == nil {
		return apiquota.Batch{}, internalStoreError()
	}
	batch, err := getAPIQuotaBatch(ctx, s.pool, ownerUserID, batchID, false)
	if errors.Is(err, pgx.ErrNoRows) {
		return apiquota.Batch{}, quotaNotFound("额度批次不存在。")
	}
	if err != nil {
		return apiquota.Batch{}, internalStoreError()
	}
	return batch, nil
}

func (s *Store) ListAPIQuotaBatchesForOwner(ctx context.Context, ownerUserID, apiServiceID string, page domain.PageRequest) (domain.Page[apiquota.Batch], *domain.AppError) {
	if s == nil || s.pool == nil {
		return domain.Page[apiquota.Batch]{}, internalStoreError()
	}
	page = normalizePageRequest(page)
	position, appErr := decodeKeysetCursor(page.Cursor)
	if appErr != nil {
		return domain.Page[apiquota.Batch]{}, appErr
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+apiQuotaBatchColumns+`
		FROM api_quota_batches b
		JOIN api_services s ON s.id = b.api_service_id AND s.owner_user_id = b.owner_user_id
		WHERE b.owner_user_id = $1 AND b.api_service_id = $2
		  AND ($3::timestamptz IS NULL OR (b.updated_at, b.id) < ($3::timestamptz, $4::uuid))
		ORDER BY b.updated_at DESC, b.id DESC
		LIMIT $5
	`, ownerUserID, apiServiceID, nullTime(position.Time), nullUUID(position.ID), page.Limit+1)
	if err != nil {
		return domain.Page[apiquota.Batch]{}, internalStoreError()
	}
	defer rows.Close()
	items := make([]apiquota.Batch, 0, page.Limit+1)
	for rows.Next() {
		var batch apiquota.Batch
		if err := scanAPIQuotaBatch(rows, &batch); err != nil {
			return domain.Page[apiquota.Batch]{}, internalStoreError()
		}
		items = append(items, batch)
	}
	if rows.Err() != nil {
		return domain.Page[apiquota.Batch]{}, internalStoreError()
	}
	return pageFromItems(items, page, func(item apiquota.Batch) (time.Time, string) {
		return item.UpdatedAt, item.ID
	}), nil
}

func (s *Store) CreateAPIQuotaOffer(ctx context.Context, offer apiquota.Offer, continuousCopies int, requestID string, now time.Time) (apiquota.Offer, *domain.AppError) {
	result, _, appErr := executeAPIQuotaCommand(ctx, s, nil, now, func(tx pgx.Tx) (apiquota.Offer, *domain.AppError) {
		return s.createAPIQuotaOfferInTx(ctx, tx, offer, continuousCopies, requestID, now)
	}, nil)
	return result, appErr
}

func (s *Store) CreateAPIQuotaOfferWithIdempotency(ctx context.Context, entry idempotency.Entry, offer apiquota.Offer, continuousCopies int, requestID string, now time.Time, buildCompletion apiquota.OfferCompletionBuilder) (apiquota.Offer, idempotency.Completion, *domain.AppError) {
	return executeAPIQuotaCommand(ctx, s, &entry, now, func(tx pgx.Tx) (apiquota.Offer, *domain.AppError) {
		return s.createAPIQuotaOfferInTx(ctx, tx, offer, continuousCopies, requestID, now)
	}, buildCompletion)
}

func (s *Store) createAPIQuotaOfferInTx(ctx context.Context, tx pgx.Tx, offer apiquota.Offer, continuousCopies int, requestID string, now time.Time) (apiquota.Offer, *domain.AppError) {
	batch, err := getAPIQuotaBatch(ctx, tx, offer.OwnerUserID, offer.BatchID, true)
	if errors.Is(err, pgx.ErrNoRows) {
		return apiquota.Offer{}, quotaNotFound("额度批次不存在。")
	}
	if err != nil {
		return apiquota.Offer{}, internalStoreError()
	}
	if batch.Status != apiquota.BatchStatusDraft {
		return apiquota.Offer{}, invalidQuotaState("只有草稿额度批次可以新增规格。")
	}
	if batch.APIServiceID != offer.APIServiceID || batch.DistributionSystem != offer.DistributionSystem {
		return apiquota.Offer{}, internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO api_quota_offers (
			id, batch_id, api_service_id, owner_user_id, distribution_system,
			name, usd_allowance, price_cny, model_multiplier,
			five_hour_limit_mode, five_hour_limit_usd, daily_limit_mode, daily_limit_usd,
			delivery_mode, delivery_eta_minutes, sale_mode, status, sort_order,
			created_at, updated_at, version
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9,
			$10, $11, $12, $13, $14, $15, $16, 'draft', $17, $18, $18, 1
		)
	`, offer.ID, offer.BatchID, offer.APIServiceID, offer.OwnerUserID, offer.DistributionSystem,
		offer.Name, offer.USDAllowance, offer.PriceCNY, offer.ModelMultiplier,
		offer.QuotaUsagePolicy.FiveHour.Mode, nullNumeric(offer.QuotaUsagePolicy.FiveHour.AmountUSD),
		offer.QuotaUsagePolicy.Daily.Mode, nullNumeric(offer.QuotaUsagePolicy.Daily.AmountUSD),
		offer.DeliveryMode, offer.DeliveryETAMinutes, offer.SaleMode, offer.SortOrder, now)
	if err != nil {
		return apiquota.Offer{}, mapAPIQuotaWriteError(err)
	}
	if offer.SaleMode == apiquota.SaleModeContinuous {
		allocated, ok := allocationAmount(offer.USDAllowance, continuousCopies)
		if !ok {
			return apiquota.Offer{}, internalStoreError()
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO api_quota_allocations (
				id, batch_id, offer_id, api_service_id, owner_user_id,
				sale_round_id, sale_mode, copy_limit, allocated_usd_allowance,
				status, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, NULL, 'continuous', $6, $7, 'planned', $8, $8)
		`, uuid.NewString(), offer.BatchID, offer.ID, offer.APIServiceID, offer.OwnerUserID, continuousCopies, allocated, now)
		if err != nil {
			return apiquota.Offer{}, mapAPIQuotaWriteError(err)
		}
	}
	if appErr := insertAPIOperationDomainEvent(ctx, tx, "api_quota_offer", offer.ID, "api_quota_offer.created", offer.OwnerUserID, apiOperationActorUser, offer.Version, requestID, map[string]any{
		"status": offer.Status, "saleMode": offer.SaleMode, "deliveryMode": offer.DeliveryMode,
	}, now); appErr != nil {
		return apiquota.Offer{}, appErr
	}
	return offer, nil
}

func (s *Store) GetAPIQuotaOfferForOwner(ctx context.Context, ownerUserID, offerID string) (apiquota.Offer, *domain.AppError) {
	if s == nil || s.pool == nil {
		return apiquota.Offer{}, internalStoreError()
	}
	var offer apiquota.Offer
	err := scanAPIQuotaOffer(s.pool.QueryRow(ctx, `
		SELECT `+apiQuotaOfferColumns+`
		FROM api_quota_offers o
		WHERE o.owner_user_id = $1 AND o.id = $2
	`, ownerUserID, offerID), &offer)
	if errors.Is(err, pgx.ErrNoRows) {
		return apiquota.Offer{}, quotaNotFound("额度规格不存在。")
	}
	if err != nil {
		return apiquota.Offer{}, internalStoreError()
	}
	return offer, nil
}

func (s *Store) ListAPIQuotaOffersForBatch(ctx context.Context, ownerUserID, batchID string) ([]apiquota.Offer, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+apiQuotaOfferColumns+`
		FROM api_quota_offers o
		WHERE o.owner_user_id = $1 AND o.batch_id = $2
		ORDER BY o.sort_order, o.created_at, o.id
	`, ownerUserID, batchID)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return scanAPIQuotaOffers(rows)
}

func (s *Store) CreateAPIQuotaSaleRound(ctx context.Context, round apiquota.SaleRound, requested []apiquota.RoundOfferInput, requestID string, now time.Time) (apiquota.SaleRound, *domain.AppError) {
	result, _, appErr := executeAPIQuotaCommand(ctx, s, nil, now, func(tx pgx.Tx) (apiquota.SaleRound, *domain.AppError) {
		return s.createAPIQuotaSaleRoundInTx(ctx, tx, round, requested, requestID, now)
	}, nil)
	return result, appErr
}

func (s *Store) CreateAPIQuotaSaleRoundWithIdempotency(ctx context.Context, entry idempotency.Entry, round apiquota.SaleRound, requested []apiquota.RoundOfferInput, requestID string, now time.Time, buildCompletion apiquota.SaleRoundCompletionBuilder) (apiquota.SaleRound, idempotency.Completion, *domain.AppError) {
	return executeAPIQuotaCommand(ctx, s, &entry, now, func(tx pgx.Tx) (apiquota.SaleRound, *domain.AppError) {
		return s.createAPIQuotaSaleRoundInTx(ctx, tx, round, requested, requestID, now)
	}, buildCompletion)
}

func (s *Store) createAPIQuotaSaleRoundInTx(ctx context.Context, tx pgx.Tx, round apiquota.SaleRound, requested []apiquota.RoundOfferInput, requestID string, now time.Time) (apiquota.SaleRound, *domain.AppError) {
	batch, err := getAPIQuotaBatch(ctx, tx, round.OwnerUserID, round.BatchID, true)
	if errors.Is(err, pgx.ErrNoRows) {
		return apiquota.SaleRound{}, quotaNotFound("额度批次不存在。")
	}
	if err != nil {
		return apiquota.SaleRound{}, internalStoreError()
	}
	if batch.Status != apiquota.BatchStatusDraft {
		return apiquota.SaleRound{}, invalidQuotaState("只有草稿额度批次可以新增放量轮次。")
	}
	var overlaps bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM api_quota_sale_rounds
			WHERE batch_id = $1 AND status = 'scheduled'
			  AND tstzrange(starts_at, ends_at, '[)') && tstzrange($2, $3, '[)')
		)
	`, round.BatchID, round.StartsAt, round.EndsAt).Scan(&overlaps); err != nil {
		return apiquota.SaleRound{}, internalStoreError()
	}
	if overlaps {
		return apiquota.SaleRound{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Overlapping sale round", "同一额度批次的放量轮次不能重叠。", "startsAt", "overlap", "放量时段与现有轮次重叠。")
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO api_quota_sale_rounds (
			id, batch_id, api_service_id, owner_user_id, system_slot_key, name,
			starts_at, ends_at, status, created_at, updated_at, version
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'scheduled', $9, $9, 1)
	`, round.ID, round.BatchID, round.APIServiceID, round.OwnerUserID, nullText(round.SystemSlotKey), round.Name, round.StartsAt, round.EndsAt, now)
	if err != nil {
		return apiquota.SaleRound{}, mapAPIQuotaWriteError(err)
	}
	for _, item := range requested {
		var allowance string
		err := tx.QueryRow(ctx, `
			SELECT usd_allowance::text
			FROM api_quota_offers
			WHERE id = $1 AND batch_id = $2 AND owner_user_id = $3
			  AND sale_mode = 'scheduled' AND status = 'draft'
			FOR SHARE
		`, item.OfferID, round.BatchID, round.OwnerUserID).Scan(&allowance)
		if errors.Is(err, pgx.ErrNoRows) {
			return apiquota.SaleRound{}, quotaNotFound("放量轮次包含无效的额度规格。")
		}
		if err != nil {
			return apiquota.SaleRound{}, internalStoreError()
		}
		allocated, ok := allocationAmount(allowance, item.Copies)
		if !ok {
			return apiquota.SaleRound{}, internalStoreError()
		}
		allocationID := uuid.NewString()
		_, err = tx.Exec(ctx, `
			INSERT INTO api_quota_allocations (
				id, batch_id, offer_id, api_service_id, owner_user_id,
				sale_round_id, sale_mode, copy_limit, allocated_usd_allowance,
				status, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, 'scheduled', $7, $8, 'planned', $9, $9)
		`, allocationID, round.BatchID, item.OfferID, round.APIServiceID, round.OwnerUserID, round.ID, item.Copies, allocated, now)
		if err != nil {
			return apiquota.SaleRound{}, mapAPIQuotaWriteError(err)
		}
		round.Allocations = append(round.Allocations, apiquota.Allocation{
			ID: allocationID, BatchID: round.BatchID, OfferID: item.OfferID,
			APIServiceID: round.APIServiceID, OwnerUserID: round.OwnerUserID,
			SaleRoundID: round.ID, SaleMode: apiquota.SaleModeScheduled,
			CopyLimit: item.Copies, AllocatedUSDAllowance: allocated, Status: "planned",
			CreatedAt: now, UpdatedAt: now,
		})
	}
	if appErr := insertAPIOperationDomainEvent(ctx, tx, "api_quota_sale_round", round.ID, "api_quota_sale_round.created", round.OwnerUserID, apiOperationActorUser, round.Version, requestID, map[string]any{
		"status": round.Status, "systemSlot": strings.TrimSpace(round.SystemSlotKey) != "",
	}, now); appErr != nil {
		return apiquota.SaleRound{}, appErr
	}
	return round, nil
}

func (s *Store) ListAPIQuotaSaleRoundsForBatch(ctx context.Context, ownerUserID, batchID string) ([]apiquota.SaleRound, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, batch_id::text, api_service_id::text, owner_user_id::text,
		       COALESCE(system_slot_key, ''), name, starts_at, ends_at, status, fulfillment_confirmed_at, created_at, updated_at, version
		FROM api_quota_sale_rounds
		WHERE owner_user_id = $1 AND batch_id = $2
		ORDER BY starts_at, id
	`, ownerUserID, batchID)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	result := []apiquota.SaleRound{}
	for rows.Next() {
		var round apiquota.SaleRound
		if err := scanAPIQuotaRound(rows, &round); err != nil {
			return nil, internalStoreError()
		}
		allocations, appErr := listAPIQuotaAllocations(ctx, s.pool, ownerUserID, batchID, round.ID)
		if appErr != nil {
			return nil, appErr
		}
		round.Allocations = allocations
		result = append(result, round)
	}
	if rows.Err() != nil {
		return nil, internalStoreError()
	}
	return result, nil
}

func (s *Store) ConfirmAPIQuotaSaleRoundFulfillmentWithIdempotency(ctx context.Context, entry idempotency.Entry, input apiquota.SaleRoundActionInput, now time.Time, buildCompletion apiquota.SaleRoundCompletionBuilder) (apiquota.SaleRound, idempotency.Completion, *domain.AppError) {
	return executeAPIQuotaCommand(ctx, s, &entry, now, func(tx pgx.Tx) (apiquota.SaleRound, *domain.AppError) {
		var round apiquota.SaleRound
		err := tx.QueryRow(ctx, `
			SELECT id::text, batch_id::text, api_service_id::text, owner_user_id::text,
			       COALESCE(system_slot_key, ''), name, starts_at, ends_at, status,
			       fulfillment_confirmed_at, created_at, updated_at, version
			FROM api_quota_sale_rounds
			WHERE id = $1 AND owner_user_id = $2
			FOR UPDATE
		`, input.SaleRoundID, input.OwnerUserID).Scan(
			&round.ID, &round.BatchID, &round.APIServiceID, &round.OwnerUserID,
			&round.SystemSlotKey, &round.Name, &round.StartsAt, &round.EndsAt, &round.Status,
			&round.FulfillmentConfirmedAt, &round.CreatedAt, &round.UpdatedAt, &round.Version,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return apiquota.SaleRound{}, quotaNotFound("放量轮次不存在。")
		}
		if err != nil {
			return apiquota.SaleRound{}, internalStoreError()
		}
		if input.ExpectedVersion > 0 && round.Version != input.ExpectedVersion {
			return apiquota.SaleRound{}, quotaVersionConflict()
		}
		if round.SystemSlotKey == "" || round.Status != apiquota.RoundStatusScheduled || now.Before(round.StartsAt.Add(-30*time.Minute)) || !now.Before(round.StartsAt) {
			return apiquota.SaleRound{}, invalidQuotaState("仅可在平台场次开始前 30 分钟内确认履约。")
		}
		var serviceReady, batchReady bool
		if err := tx.QueryRow(ctx, `
			SELECT (`+apiServiceFulfillmentReadyPredicate("service")+`),
			       batch.status = 'published' AND batch.sale_cutoff_at > $3 AND batch.expires_at > $3
			FROM api_services service
			JOIN api_quota_batches batch ON batch.api_service_id = service.id AND batch.id = $2
			WHERE service.id = $1 AND service.owner_user_id = $4
			FOR SHARE OF service, batch
		`, round.APIServiceID, round.BatchID, now, input.OwnerUserID).Scan(&serviceReady, &batchReady); err != nil {
			return apiquota.SaleRound{}, internalStoreError()
		}
		if !serviceReady || !batchReady {
			return apiquota.SaleRound{}, invalidQuotaState("卖家账号、服务、探针、收款配置或额度批次当前不可履约。")
		}
		if appErr := ensureAPIServicePublishAllowedInTx(ctx, tx, input.OwnerUserID, now); appErr != nil {
			return apiquota.SaleRound{}, appErr
		}
		if round.FulfillmentConfirmedAt == nil {
			confirmedAt := now
			if err := tx.QueryRow(ctx, `
				UPDATE api_quota_sale_rounds
				SET fulfillment_confirmed_at = $2, updated_at = $2, version = version + 1
				WHERE id = $1
				RETURNING updated_at, version
			`, round.ID, now).Scan(&round.UpdatedAt, &round.Version); err != nil {
				return apiquota.SaleRound{}, internalStoreError()
			}
			round.FulfillmentConfirmedAt = &confirmedAt
			if appErr := insertAPIOperationDomainEvent(ctx, tx, "api_quota_sale_round", round.ID, "api_quota_sale_round.fulfillment_confirmed", input.OwnerUserID, apiOperationActorUser, round.Version, input.RequestID, map[string]any{
				"fulfillmentConfirmedAt": now,
			}, now); appErr != nil {
				return apiquota.SaleRound{}, appErr
			}
		}
		allocations, appErr := listAPIQuotaAllocations(ctx, tx, input.OwnerUserID, round.BatchID, round.ID)
		if appErr != nil {
			return apiquota.SaleRound{}, appErr
		}
		round.Allocations = allocations
		return round, nil
	}, buildCompletion)
}

func (s *Store) PublishAPIQuotaBatch(ctx context.Context, input apiquota.BatchActionInput, now time.Time) (apiquota.Batch, *domain.AppError) {
	result, _, appErr := executeAPIQuotaCommand(ctx, s, nil, now, func(tx pgx.Tx) (apiquota.Batch, *domain.AppError) {
		return publishAPIQuotaBatchInTx(ctx, tx, input, now)
	}, nil)
	return result, appErr
}

func (s *Store) PublishAPIQuotaBatchWithIdempotency(ctx context.Context, entry idempotency.Entry, input apiquota.BatchActionInput, now time.Time, buildCompletion apiquota.BatchCompletionBuilder) (apiquota.Batch, idempotency.Completion, *domain.AppError) {
	return executeAPIQuotaCommand(ctx, s, &entry, now, func(tx pgx.Tx) (apiquota.Batch, *domain.AppError) {
		return publishAPIQuotaBatchInTx(ctx, tx, input, now)
	}, buildCompletion)
}

func publishAPIQuotaBatchInTx(ctx context.Context, tx pgx.Tx, input apiquota.BatchActionInput, now time.Time) (apiquota.Batch, *domain.AppError) {
	batch, err := getAPIQuotaBatch(ctx, tx, input.OwnerUserID, input.BatchID, true)
	if errors.Is(err, pgx.ErrNoRows) {
		return apiquota.Batch{}, quotaNotFound("额度批次不存在。")
	}
	if err != nil {
		return apiquota.Batch{}, internalStoreError()
	}
	if input.ExpectedVersion > 0 && batch.Version != input.ExpectedVersion {
		return apiquota.Batch{}, quotaVersionConflict()
	}
	if appErr := ensureAPIServiceCatalogActiveInTx(ctx, tx, batch.APIServiceID); appErr != nil {
		return apiquota.Batch{}, appErr
	}
	if appErr := ensureAPIServicePublishAllowedInTx(ctx, tx, batch.OwnerUserID, now); appErr != nil {
		return apiquota.Batch{}, appErr
	}
	if batch.Status != apiquota.BatchStatusDraft {
		return apiquota.Batch{}, invalidQuotaState("当前额度批次不能发布。")
	}
	if !batch.ServiceOrderable {
		return apiquota.Batch{}, invalidQuotaState("关联 API 服务当前不可接单。")
	}
	var flexibleQuotaSaleOpen bool
	if err := tx.QueryRow(ctx, `
		SELECT billing_mode = 'metered_usd_quota'
		   AND accepting_orders = true
		   AND available_usd_allowance > 0
		   AND quota_expires_at > $2::timestamptz + interval '24 hours'
		FROM api_services
		WHERE id = $1
	`, batch.APIServiceID, now).Scan(&flexibleQuotaSaleOpen); err != nil {
		return apiquota.Batch{}, internalStoreError()
	}
	if flexibleQuotaSaleOpen {
		return apiquota.Batch{}, invalidQuotaState("请先关闭该服务的自选额度接单，再发布限量额度包。")
	}
	if !now.Before(batch.SaleCutoffAt) || !now.Before(batch.ExpiresAt) {
		return apiquota.Batch{}, domain.NewError(http.StatusConflict, domain.CodeAPIQuotaBatchExpired, "Quota batch expired", "额度批次已超过最晚下单时间。")
	}
	var allocationCount int
	var totalAllocated string
	err = tx.QueryRow(ctx, `
		SELECT count(*), COALESCE(sum(allocated_usd_allowance), 0)::text
		FROM api_quota_allocations
		WHERE batch_id = $1 AND status = 'planned'
	`, batch.ID).Scan(&allocationCount, &totalAllocated)
	if err != nil {
		return apiquota.Batch{}, internalStoreError()
	}
	if allocationCount == 0 {
		return apiquota.Batch{}, domain.NewError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Inventory required", "发布额度批次前必须配置可售份数。")
	}
	var expiredRound bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM api_quota_sale_rounds
			WHERE batch_id = $1 AND status = 'scheduled'
			  AND (starts_at <= $2 OR ends_at > $3)
		)
	`, batch.ID, now, batch.SaleCutoffAt).Scan(&expiredRound); err != nil {
		return apiquota.Batch{}, internalStoreError()
	}
	if expiredRound {
		return apiquota.Batch{}, domain.NewError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid sale round", "定时放量必须在开始前发布，且结束时间不能晚于批次最晚下单时间。")
	}
	credentialRows, err := tx.Query(ctx, `
		SELECT o.id::text, sum(a.copy_limit),
		       (SELECT count(*) FROM api_quota_credentials c WHERE c.api_quota_offer_id = o.id AND c.status = 'available')
		FROM api_quota_offers o
		JOIN api_quota_allocations a ON a.offer_id = o.id AND a.status = 'planned'
		WHERE o.batch_id = $1 AND o.delivery_mode = 'preimported'
		GROUP BY o.id
	`, batch.ID)
	if err != nil {
		return apiquota.Batch{}, internalStoreError()
	}
	for credentialRows.Next() {
		var offerID string
		var required, available int
		if err := credentialRows.Scan(&offerID, &required, &available); err != nil {
			credentialRows.Close()
			return apiquota.Batch{}, internalStoreError()
		}
		if available < required {
			credentialRows.Close()
			return apiquota.Batch{}, domain.NewError(http.StatusConflict, domain.CodeAPIQuotaCredentialUnavailable, "Credential inventory unavailable", "预导入凭据数量不足，不能按完整库存发布额度包。")
		}
	}
	if err := credentialRows.Err(); err != nil {
		credentialRows.Close()
		return apiquota.Batch{}, internalStoreError()
	}
	credentialRows.Close()
	command, err := tx.Exec(ctx, `
		UPDATE api_quota_batches
		SET unallocated_usd_allowance = unallocated_usd_allowance - $3::numeric,
		    status = 'published', published_at = $4, updated_at = $4, version = version + 1
		WHERE id = $1 AND owner_user_id = $2 AND status = 'draft'
		  AND unallocated_usd_allowance >= $3::numeric
	`, batch.ID, input.OwnerUserID, totalAllocated, now)
	if err != nil {
		return apiquota.Batch{}, internalStoreError()
	}
	if command.RowsAffected() != 1 {
		return apiquota.Batch{}, domain.NewError(http.StatusConflict, domain.CodeAPIQuotaInsufficientAllowance, "Insufficient quota allowance", "卖家声明可售美元额度不足以覆盖全部库存计划。")
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO api_quota_inventory_units (
			id, allocation_id, batch_id, offer_id, usd_allowance, status, created_at, updated_at
		)
		SELECT gen_random_uuid(), a.id, a.batch_id, a.offer_id, o.usd_allowance, 'available', $2, $2
		FROM api_quota_allocations a
		JOIN api_quota_offers o ON o.id = a.offer_id AND o.batch_id = a.batch_id
		CROSS JOIN LATERAL generate_series(1, a.copy_limit)
		WHERE a.batch_id = $1 AND a.status = 'planned'
	`, batch.ID, now)
	if err != nil {
		return apiquota.Batch{}, internalStoreError()
	}
	if _, err := tx.Exec(ctx, `
		UPDATE api_quota_allocations SET status = 'active', updated_at = $2
		WHERE batch_id = $1 AND status = 'planned'
	`, batch.ID, now); err != nil {
		return apiquota.Batch{}, internalStoreError()
	}
	offerRows, err := tx.Query(ctx, `
		UPDATE api_quota_offers
		SET status = 'published', published_at = COALESCE(published_at, $2), updated_at = $2, version = version + 1
		WHERE batch_id = $1 AND status = 'draft'
		RETURNING id::text, owner_user_id::text, version
	`, batch.ID, now)
	if err != nil {
		return apiquota.Batch{}, internalStoreError()
	}
	publishedOffers := []quotaEventVersion{}
	for offerRows.Next() {
		var item quotaEventVersion
		if err := offerRows.Scan(&item.id, &item.ownerID, &item.version); err != nil {
			offerRows.Close()
			return apiquota.Batch{}, internalStoreError()
		}
		publishedOffers = append(publishedOffers, item)
	}
	if offerRows.Err() != nil {
		offerRows.Close()
		return apiquota.Batch{}, internalStoreError()
	}
	offerRows.Close()
	for _, offer := range publishedOffers {
		if appErr := insertAPIOperationDomainEvent(ctx, tx, "api_quota_offer", offer.id, "api_quota_offer.published", offer.ownerID, apiOperationActorUser, offer.version, input.RequestID, map[string]any{
			"status": apiquota.OfferStatusPublished,
		}, now); appErr != nil {
			return apiquota.Batch{}, appErr
		}
	}
	batch, err = getAPIQuotaBatch(ctx, tx, input.OwnerUserID, input.BatchID, false)
	if err != nil {
		return apiquota.Batch{}, internalStoreError()
	}
	if appErr := insertAPIOperationDomainEvent(ctx, tx, "api_quota_batch", batch.ID, "api_quota_batch.published", batch.OwnerUserID, apiOperationActorUser, batch.Version, input.RequestID, map[string]any{
		"status": batch.Status,
	}, now); appErr != nil {
		return apiquota.Batch{}, appErr
	}
	return batch, nil
}

func (s *Store) UpdateAPIQuotaBatchStatus(ctx context.Context, input apiquota.BatchActionInput, action string, now time.Time) (apiquota.Batch, *domain.AppError) {
	result, _, appErr := executeAPIQuotaCommand(ctx, s, nil, now, func(tx pgx.Tx) (apiquota.Batch, *domain.AppError) {
		return s.updateAPIQuotaBatchStatusInTx(ctx, tx, input, action, now)
	}, nil)
	return result, appErr
}

func (s *Store) UpdateAPIQuotaBatchStatusWithIdempotency(ctx context.Context, entry idempotency.Entry, input apiquota.BatchActionInput, action string, now time.Time, buildCompletion apiquota.BatchCompletionBuilder) (apiquota.Batch, idempotency.Completion, *domain.AppError) {
	return executeAPIQuotaCommand(ctx, s, &entry, now, func(tx pgx.Tx) (apiquota.Batch, *domain.AppError) {
		return s.updateAPIQuotaBatchStatusInTx(ctx, tx, input, action, now)
	}, buildCompletion)
}

func (s *Store) updateAPIQuotaBatchStatusInTx(ctx context.Context, tx pgx.Tx, input apiquota.BatchActionInput, action string, now time.Time) (apiquota.Batch, *domain.AppError) {
	batch, err := getAPIQuotaBatch(ctx, tx, input.OwnerUserID, input.BatchID, true)
	if errors.Is(err, pgx.ErrNoRows) {
		return apiquota.Batch{}, quotaNotFound("额度批次不存在。")
	}
	if err != nil {
		return apiquota.Batch{}, internalStoreError()
	}
	if input.ExpectedVersion > 0 && batch.Version != input.ExpectedVersion {
		return apiquota.Batch{}, quotaVersionConflict()
	}
	if action == "resume" {
		if appErr := ensureAPIServiceCatalogActiveInTx(ctx, tx, batch.APIServiceID); appErr != nil {
			return apiquota.Batch{}, appErr
		}
		if appErr := ensureAPIServicePublishAllowedInTx(ctx, tx, batch.OwnerUserID, now); appErr != nil {
			return apiquota.Batch{}, appErr
		}
	}
	next := ""
	switch action {
	case "pause":
		if batch.Status == apiquota.BatchStatusPublished {
			next = apiquota.BatchStatusPaused
		}
	case "resume":
		if batch.Status == apiquota.BatchStatusPaused && now.Before(batch.SaleCutoffAt) && now.Before(batch.ExpiresAt) && batch.ServiceOrderable {
			next = apiquota.BatchStatusPublished
		}
	case "archive":
		if batch.Status != apiquota.BatchStatusArchived {
			next = apiquota.BatchStatusArchived
		}
	}
	if next == "" {
		return apiquota.Batch{}, invalidQuotaState("当前额度批次状态不能执行该操作。")
	}
	if next == apiquota.BatchStatusArchived {
		var systemRush bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM api_quota_sale_rounds
				WHERE batch_id = $1 AND system_slot_key IS NOT NULL
			)
		`, batch.ID).Scan(&systemRush); err != nil {
			return apiquota.Batch{}, internalStoreError()
		}
		if systemRush {
			var returnedAllowance string
			if err := tx.QueryRow(ctx, `
				SELECT COALESCE(sum(a.allocated_usd_allowance - a.returned_usd_allowance), 0)::text
				FROM api_quota_allocations a
				JOIN api_quota_sale_rounds r ON r.id = a.sale_round_id
				WHERE a.batch_id = $1
				  AND r.system_slot_key IS NOT NULL
				  AND a.status IN ('planned', 'active')
			`, batch.ID).Scan(&returnedAllowance); err != nil {
				return apiquota.Batch{}, internalStoreError()
			}
			if _, err := tx.Exec(ctx, `
				UPDATE api_quota_inventory_units unit
				SET status = 'retired', retired_at = $2, updated_at = $2
				FROM api_quota_allocations a
				JOIN api_quota_sale_rounds r ON r.id = a.sale_round_id
				WHERE unit.allocation_id = a.id
				  AND a.batch_id = $1
				  AND r.system_slot_key IS NOT NULL
				  AND unit.status = 'available'
			`, batch.ID, now); err != nil {
				return apiquota.Batch{}, internalStoreError()
			}
			if _, err := tx.Exec(ctx, `
				UPDATE api_quota_credentials credential
				SET status = 'retired', retired_at = $2, updated_at = $2
				FROM api_quota_offers offer
				WHERE credential.api_quota_offer_id = offer.id
				  AND offer.batch_id = $1
				  AND credential.status = 'available'
			`, batch.ID, now); err != nil {
				return apiquota.Batch{}, internalStoreError()
			}
			if _, err := tx.Exec(ctx, `
				UPDATE api_quota_allocations a
				SET returned_usd_allowance = allocated_usd_allowance,
				    status = 'closed',
				    updated_at = $2
				FROM api_quota_sale_rounds r
				WHERE a.sale_round_id = r.id
				  AND a.batch_id = $1
				  AND r.system_slot_key IS NOT NULL
				  AND a.status IN ('planned', 'active')
			`, batch.ID, now); err != nil {
				return apiquota.Batch{}, internalStoreError()
			}
			cancelledRows, err := tx.Query(ctx, `
				UPDATE api_quota_sale_rounds
				SET status = 'cancelled', updated_at = $2, version = version + 1
				WHERE batch_id = $1
				  AND system_slot_key IS NOT NULL
				  AND status = 'scheduled'
				RETURNING id::text, owner_user_id::text, version
			`, batch.ID, now)
			if err != nil {
				return apiquota.Batch{}, internalStoreError()
			}
			cancelledRounds := []quotaEventVersion{}
			for cancelledRows.Next() {
				var item quotaEventVersion
				if err := cancelledRows.Scan(&item.id, &item.ownerID, &item.version); err != nil {
					cancelledRows.Close()
					return apiquota.Batch{}, internalStoreError()
				}
				cancelledRounds = append(cancelledRounds, item)
			}
			if cancelledRows.Err() != nil {
				cancelledRows.Close()
				return apiquota.Batch{}, internalStoreError()
			}
			cancelledRows.Close()
			for _, round := range cancelledRounds {
				if appErr := insertAPIOperationDomainEvent(ctx, tx, "api_quota_sale_round", round.id, "api_quota_sale_round.cancelled", input.OwnerUserID, apiOperationActorUser, round.version, input.RequestID, map[string]any{
					"status": "cancelled",
				}, now); appErr != nil {
					return apiquota.Batch{}, appErr
				}
			}
			if _, err := tx.Exec(ctx, `
				UPDATE api_quota_batches
				SET unallocated_usd_allowance = unallocated_usd_allowance + $2::numeric
				WHERE id = $1
			`, batch.ID, returnedAllowance); err != nil {
				return apiquota.Batch{}, internalStoreError()
			}
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE api_quota_batches SET status = $3, updated_at = $4, version = version + 1
		WHERE id = $1 AND owner_user_id = $2
	`, batch.ID, input.OwnerUserID, next, now); err != nil {
		return apiquota.Batch{}, internalStoreError()
	}
	if next == apiquota.BatchStatusArchived {
		archivedRows, err := tx.Query(ctx, `
			UPDATE api_quota_offers SET status = 'archived', updated_at = $2, version = version + 1
			WHERE batch_id = $1 AND status <> 'archived'
			RETURNING id::text, owner_user_id::text, version
		`, batch.ID, now)
		if err != nil {
			return apiquota.Batch{}, internalStoreError()
		}
		archivedOffers := []quotaEventVersion{}
		for archivedRows.Next() {
			var item quotaEventVersion
			if err := archivedRows.Scan(&item.id, &item.ownerID, &item.version); err != nil {
				archivedRows.Close()
				return apiquota.Batch{}, internalStoreError()
			}
			archivedOffers = append(archivedOffers, item)
		}
		if archivedRows.Err() != nil {
			archivedRows.Close()
			return apiquota.Batch{}, internalStoreError()
		}
		archivedRows.Close()
		for _, offer := range archivedOffers {
			if appErr := insertAPIOperationDomainEvent(ctx, tx, "api_quota_offer", offer.id, "api_quota_offer.archived", offer.ownerID, apiOperationActorUser, offer.version, input.RequestID, map[string]any{
				"status": apiquota.OfferStatusArchived,
			}, now); appErr != nil {
				return apiquota.Batch{}, appErr
			}
		}
	}
	batch, err = getAPIQuotaBatch(ctx, tx, input.OwnerUserID, input.BatchID, false)
	if err != nil {
		return apiquota.Batch{}, internalStoreError()
	}
	eventType := map[string]string{
		"pause": "api_quota_batch.paused", "resume": "api_quota_batch.resumed", "archive": "api_quota_batch.archived",
	}[action]
	if appErr := insertAPIOperationDomainEvent(ctx, tx, "api_quota_batch", batch.ID, eventType, batch.OwnerUserID, apiOperationActorUser, batch.Version, input.RequestID, map[string]any{
		"status": batch.Status,
	}, now); appErr != nil {
		return apiquota.Batch{}, appErr
	}
	return batch, nil
}

func (s *Store) ListPublicAPIQuotaOffers(ctx context.Context, filter apiquota.PublicOfferFilter, page domain.PageRequest, now time.Time) (domain.Page[apiquota.OfferCard], *domain.AppError) {
	if s == nil || s.pool == nil {
		return domain.Page[apiquota.OfferCard]{}, internalStoreError()
	}
	if appErr := s.MaterializeExpiredAPIQuotaInventory(ctx, now); appErr != nil {
		return domain.Page[apiquota.OfferCard]{}, appErr
	}
	page = normalizePageRequest(page)
	sortMode := filter.NormalizedSort()
	var position keysetPosition
	var scalarPosition scalarKeysetPosition
	var appErr *domain.AppError
	if sortMode == apiquota.PublicOfferSortUpdatedDesc {
		position, appErr = decodeKeysetCursor(page.Cursor)
	} else {
		scalarPosition, appErr = decodeScalarKeysetCursor(page.Cursor, sortMode)
	}
	if appErr != nil {
		return domain.Page[apiquota.OfferCard]{}, appErr
	}
	serviceSortExpression := apiServiceSortExpression("s", sortMode)
	if page.Cursor != "" {
		switch sortMode {
		case apiquota.PublicOfferSortUnitPriceAsc, apiquota.PublicOfferSortAllowanceDesc:
			if appErr := validateNonNegativeDecimalCursor(scalarPosition); appErr != nil {
				return domain.Page[apiquota.OfferCard]{}, appErr
			}
		case apiquota.PublicOfferSortDeliveryAsc:
			value, err := strconv.Atoi(scalarPosition.Value)
			if err != nil || value < 0 {
				return domain.Page[apiquota.OfferCard]{}, invalidPageCursorError()
			}
		}
		if serviceSortExpression != "" {
			if appErr := validateNonNegativeDecimalCursor(scalarPosition); appErr != nil {
				return domain.Page[apiquota.OfferCard]{}, appErr
			}
		}
	}
	cursorCondition := `($5::timestamptz IS NULL OR (o.updated_at, o.id) < ($5::timestamptz, $6::uuid))`
	cursorValue := any(nullTime(position.Time))
	cursorID := any(nullUUID(position.ID))
	orderBy := `ORDER BY o.updated_at DESC, o.id DESC`
	switch sortMode {
	case apiquota.PublicOfferSortUnitPriceAsc:
		cursorCondition = `($5 = '' OR (` + apiQuotaUnitPriceExpression + `, o.id) > ($5::numeric, $6::uuid))`
		cursorValue, cursorID = scalarPosition.Value, nullUUID(scalarPosition.ID)
		orderBy = `ORDER BY ` + apiQuotaUnitPriceExpression + ` ASC, o.id ASC`
	case apiquota.PublicOfferSortAllowanceDesc:
		cursorCondition = `($5 = '' OR (o.usd_allowance, o.id) < ($5::numeric, $6::uuid))`
		cursorValue, cursorID = scalarPosition.Value, nullUUID(scalarPosition.ID)
		orderBy = `ORDER BY o.usd_allowance DESC, o.id DESC`
	case apiquota.PublicOfferSortDeliveryAsc:
		cursorCondition = `($5 = '' OR (o.delivery_eta_minutes, o.id) > ($5::integer, $6::uuid))`
		cursorValue, cursorID = scalarPosition.Value, nullUUID(scalarPosition.ID)
		orderBy = `ORDER BY o.delivery_eta_minutes ASC, o.id ASC`
	}
	if serviceSortExpression != "" {
		cursorCondition = `($5 = '' OR (` + serviceSortExpression + `, o.id) > ($5::numeric, $6::uuid))`
		cursorValue, cursorID = scalarPosition.Value, nullUUID(scalarPosition.ID)
		orderBy = `ORDER BY ` + serviceSortExpression + ` ASC, o.id ASC`
	}
	query := publicAPIQuotaOffersQuery
	if serviceSortExpression != "" {
		query = publicAPIQuotaOffersQueryWithSort(serviceSortExpression)
	}
	rows, err := s.pool.Query(ctx, query+`
		  AND ($2 = '' OR o.distribution_system = $2)
		  AND (NOT $3 OR o.model_multiplier = 1.0000)
		  AND (
		    $4 = '' OR EXISTS (
		      SELECT 1
		      FROM api_quota_allocations slot_allocation
		      JOIN api_quota_sale_rounds slot_round ON slot_round.id = slot_allocation.sale_round_id
		      WHERE slot_allocation.offer_id = o.id
		        AND slot_allocation.status = 'active'
		        AND slot_round.system_slot_key = $4
		    )
		  )
		  AND `+cursorCondition+`
		  AND (
		    NOT $7 OR (
		      b.status = 'published' AND o.status = 'published'
		      AND `+apiServiceFulfillmentReadyPredicate("s")+`
		      AND $1 < b.sale_cutoff_at AND $1 < b.expires_at
		      AND stock.available_copies > 0
		      AND (o.sale_mode = 'continuous' OR (current_round.id IS NOT NULL AND (current_round.system_slot_key IS NULL OR current_round.fulfillment_confirmed_at IS NOT NULL)))
		      AND (o.delivery_mode = 'manual' OR credentials.available_copies >= stock.available_copies)
		    )
		  )
		  AND (
		    $8 = '' OR o.name ILIKE '%' || $8 || '%'
		    OR s.title ILIKE '%' || $8 || '%'
		    OR (CASE WHEN s.merchant_identity_mode = 'store_alias' THEN COALESCE(mp.display_name, u.display_name) ELSE u.display_name END) ILIKE '%' || $8 || '%'
		    OR replace(o.distribution_system, '_', ' ') ILIKE '%' || $8 || '%'
		  )
		  AND (
		    NOT $9 OR NOT EXISTS (
		      SELECT 1
		      FROM api_quota_allocations system_allocation
		      JOIN api_quota_sale_rounds system_round ON system_round.id = system_allocation.sale_round_id
		      WHERE system_allocation.offer_id = o.id
		        AND system_allocation.status = 'active'
		        AND system_round.system_slot_key IS NOT NULL
		    )
		  )
		  AND (
		    $10 = '' OR EXISTS (
		      SELECT 1 FROM api_service_models selected_model
		      WHERE selected_model.api_service_id = s.id
		        AND selected_model.enabled = true
		        AND selected_model.model_catalog_id::text = $10
		    )
		  )
		  AND ($11 = '' OR o.model_multiplier <= $11::numeric)
		  AND ($12 = '' OR o.sale_mode = $12)
		`+orderBy+`
		LIMIT $13
	`, now, strings.TrimSpace(filter.DistributionSystem), filter.OnlyOneMultiplier,
		strings.TrimSpace(filter.SystemSlotKey), cursorValue, cursorID,
		filter.OnlyOrderable, strings.TrimSpace(filter.Search), filter.ExcludeSystemSlots,
		strings.TrimSpace(filter.ModelCatalogID), strings.TrimSpace(filter.MaxMultiplier), strings.TrimSpace(filter.SaleMode), page.Limit+1)
	if err != nil {
		return domain.Page[apiquota.OfferCard]{}, internalStoreError()
	}
	defer rows.Close()
	items := make([]apiquota.OfferCard, 0, page.Limit+1)
	for rows.Next() {
		var card apiquota.OfferCard
		var err error
		if serviceSortExpression != "" {
			card, err = scanAPIQuotaOfferCardWithSortValue(rows)
		} else {
			card, err = scanAPIQuotaOfferCard(rows)
		}
		if err != nil {
			return domain.Page[apiquota.OfferCard]{}, internalStoreError()
		}
		items = append(items, card)
	}
	if rows.Err() != nil {
		return domain.Page[apiquota.OfferCard]{}, internalStoreError()
	}
	switch sortMode {
	case apiquota.PublicOfferSortRecommended, apiquota.PublicOfferSortReputationDesc,
		apiquota.PublicOfferSortCompletedDesc, apiquota.PublicOfferSortResponseFast:
		return pageFromScalarItems(items, page, sortMode, func(item apiquota.OfferCard) (string, string) {
			return item.PublicSortValue, item.ID
		}), nil
	case apiquota.PublicOfferSortUnitPriceAsc:
		return pageFromScalarItems(items, page, sortMode, func(item apiquota.OfferCard) (string, string) { return item.CNYPerUSD, item.ID }), nil
	case apiquota.PublicOfferSortAllowanceDesc:
		return pageFromScalarItems(items, page, sortMode, func(item apiquota.OfferCard) (string, string) { return item.USDAllowance, item.ID }), nil
	case apiquota.PublicOfferSortDeliveryAsc:
		return pageFromScalarItems(items, page, sortMode, func(item apiquota.OfferCard) (string, string) {
			return strconv.Itoa(item.DeliveryETAMinutes), item.ID
		}), nil
	default:
		return pageFromItems(items, page, func(item apiquota.OfferCard) (time.Time, string) {
			return item.UpdatedAt, item.ID
		}), nil
	}
}

func (s *Store) GetPublicAPIQuotaOffer(ctx context.Context, offerID string, now time.Time) (apiquota.OfferCard, *domain.AppError) {
	if s == nil || s.pool == nil {
		return apiquota.OfferCard{}, internalStoreError()
	}
	if appErr := s.MaterializeExpiredAPIQuotaInventory(ctx, now); appErr != nil {
		return apiquota.OfferCard{}, appErr
	}
	card, err := scanAPIQuotaOfferCard(s.pool.QueryRow(ctx, publicAPIQuotaOffersQuery+` AND o.id = $2`, now, offerID))
	if errors.Is(err, pgx.ErrNoRows) {
		return apiquota.OfferCard{}, quotaNotFound("额度包不存在。")
	}
	if err != nil {
		return apiquota.OfferCard{}, internalStoreError()
	}
	return card, nil
}

func (s *Store) MaterializeExpiredAPIQuotaInventory(ctx context.Context, now time.Time) *domain.AppError {
	if s == nil || s.pool == nil {
		return internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return internalStoreError()
	}
	defer rollback(ctx, tx)
	if appErr := materializeExpiredAPIQuotaInventoryInTx(ctx, tx, now); appErr != nil {
		return appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return internalStoreError()
	}
	return nil
}

func materializeExpiredAPIQuotaInventoryInTx(ctx context.Context, tx pgx.Tx, now time.Time) *domain.AppError {
	expiredBatchRows, err := tx.Query(ctx, `
		WITH retired AS (
			UPDATE api_quota_inventory_units u
			SET status = 'retired', retired_at = $1, updated_at = $1
			FROM api_quota_allocations a
			JOIN api_quota_batches b ON b.id = a.batch_id
			JOIN api_quota_offers o ON o.id = a.offer_id AND o.batch_id = a.batch_id
			LEFT JOIN api_quota_sale_rounds r ON r.id = a.sale_round_id
			WHERE u.allocation_id = a.id
			  AND u.status = 'available'
			  AND a.status = 'active'
			  AND (
			    b.status = 'archived' OR o.status = 'archived'
			    OR b.sale_cutoff_at <= $1 OR b.expires_at <= $1
			    OR (a.sale_mode = 'scheduled' AND (r.status <> 'scheduled' OR r.ends_at <= $1))
			  )
			RETURNING u.allocation_id, u.batch_id, u.usd_allowance
		), by_allocation AS (
			SELECT allocation_id, sum(usd_allowance) AS returned_allowance
			FROM retired
			GROUP BY allocation_id
		), updated_allocations AS (
			UPDATE api_quota_allocations a
			SET returned_usd_allowance = a.returned_usd_allowance + x.returned_allowance,
			    updated_at = $1
			FROM by_allocation x
			WHERE a.id = x.allocation_id
			RETURNING a.id
		), by_batch AS (
			SELECT batch_id, sum(usd_allowance) AS returned_allowance
			FROM retired
			GROUP BY batch_id
		)
		UPDATE api_quota_batches b
		SET unallocated_usd_allowance = b.unallocated_usd_allowance + x.returned_allowance,
		    updated_at = $1, version = b.version + 1
		FROM by_batch x
		WHERE b.id = x.batch_id
		RETURNING b.id::text, b.owner_user_id::text, b.version
	`, now)
	if err != nil {
		return internalStoreError()
	}
	expiredBatches := []quotaEventVersion{}
	for expiredBatchRows.Next() {
		var item quotaEventVersion
		if err := expiredBatchRows.Scan(&item.id, &item.ownerID, &item.version); err != nil {
			expiredBatchRows.Close()
			return internalStoreError()
		}
		expiredBatches = append(expiredBatches, item)
	}
	if expiredBatchRows.Err() != nil {
		expiredBatchRows.Close()
		return internalStoreError()
	}
	expiredBatchRows.Close()
	for _, batch := range expiredBatches {
		if appErr := insertAPIOperationDomainEvent(ctx, tx, "api_quota_batch", batch.id, "api_quota_batch.inventory_expired", "", apiOperationActorSystem, batch.version, "system:api-quota-expiry", map[string]any{
			"status": "inventory_expired",
		}, now); appErr != nil {
			return appErr
		}
	}
	_, err = tx.Exec(ctx, `
		UPDATE api_quota_allocations a
		SET status = 'closed', updated_at = $1
		FROM api_quota_batches b, api_quota_offers o
		WHERE a.batch_id = b.id AND o.id = a.offer_id AND o.batch_id = b.id
		  AND a.status = 'active'
		  AND (
		    b.status = 'archived' OR o.status = 'archived'
		    OR b.sale_cutoff_at <= $1 OR b.expires_at <= $1
		    OR (
		      a.sale_mode = 'scheduled'
		      AND EXISTS (
		        SELECT 1 FROM api_quota_sale_rounds r
		        WHERE r.id = a.sale_round_id
		          AND (r.status <> 'scheduled' OR r.ends_at <= $1)
		      )
		    )
		  )
	`, now)
	if err != nil {
		return internalStoreError()
	}
	expiredRoundRows, err := tx.Query(ctx, `
		UPDATE api_quota_sale_rounds
		SET status = 'closed', updated_at = $1, version = version + 1
		WHERE status = 'scheduled' AND ends_at <= $1
		RETURNING id::text, owner_user_id::text, version
	`, now)
	if err != nil {
		return internalStoreError()
	}
	expiredRounds := []quotaEventVersion{}
	for expiredRoundRows.Next() {
		var item quotaEventVersion
		if err := expiredRoundRows.Scan(&item.id, &item.ownerID, &item.version); err != nil {
			expiredRoundRows.Close()
			return internalStoreError()
		}
		expiredRounds = append(expiredRounds, item)
	}
	if expiredRoundRows.Err() != nil {
		expiredRoundRows.Close()
		return internalStoreError()
	}
	expiredRoundRows.Close()
	for _, round := range expiredRounds {
		if appErr := insertAPIOperationDomainEvent(ctx, tx, "api_quota_sale_round", round.id, "api_quota_sale_round.expired", "", apiOperationActorSystem, round.version, "system:api-quota-expiry", map[string]any{
			"status": "closed",
		}, now); appErr != nil {
			return appErr
		}
	}
	return nil
}

func (s *Store) GetAPIQuotaOrderForBuyer(ctx context.Context, buyerUserID, orderID string, now time.Time) (apiorder.Order, *domain.AppError) {
	return s.GetAPIOrderForBuyer(ctx, buyerUserID, orderID, now)
}

func (s *Store) CreateSystemRushOfferWithIdempotency(ctx context.Context, entry idempotency.Entry, publication apiquota.RushOfferPublication, credentials []apiquota.CredentialImportRow, now time.Time, buildCompletion apiquota.RushOfferCompletionBuilder) (apiquota.RushOfferPublication, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil || len(publication.Round.Allocations) != 1 || (len(credentials) > 0 && s.contactCodec == nil) {
		return apiquota.RushOfferPublication{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apiquota.RushOfferPublication{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return apiquota.RushOfferPublication{}, idempotency.Completion{}, appErr
	}

	var serviceTitle, distributionSystem, declaredTTFTBand string
	var serviceOrderable bool
	var declaredMaxConcurrency int
	var performanceConfirmedAt *time.Time
	var promptAuditEnabled *bool
	err = tx.QueryRow(ctx, `
		SELECT s.title, s.distribution_system, (`+apiServiceFulfillmentReadyPredicate("s")+`),
		       COALESCE(s.declared_ttft_band, ''), COALESCE(s.declared_max_concurrency, 0),
		       s.performance_confirmed_at, s.prompt_audit_enabled
		FROM api_services s
		WHERE s.id = $1 AND s.owner_user_id = $2
		FOR UPDATE OF s
	`, publication.Batch.APIServiceID, publication.Batch.OwnerUserID).Scan(
		&serviceTitle,
		&distributionSystem,
		&serviceOrderable,
		&declaredTTFTBand,
		&declaredMaxConcurrency,
		&performanceConfirmedAt,
		&promptAuditEnabled,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return apiquota.RushOfferPublication{}, idempotency.Completion{}, quotaNotFound("API 服务不存在。")
	}
	if err != nil {
		return apiquota.RushOfferPublication{}, idempotency.Completion{}, internalStoreError()
	}
	if !serviceOrderable {
		return apiquota.RushOfferPublication{}, idempotency.Completion{}, invalidQuotaState("关联 API 服务当前不可接单。")
	}
	if appErr := ensureAPIServicePublishAllowedInTx(ctx, tx, publication.Batch.OwnerUserID, now); appErr != nil {
		return apiquota.RushOfferPublication{}, idempotency.Completion{}, appErr
	}
	if declaredMaxConcurrency < 1 {
		return apiquota.RushOfferPublication{}, idempotency.Completion{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Maximum concurrency required", "发布额度包前必须填写商户声明最大并发。", "declaredMaxConcurrency", "required", "请输入大于 0 的最大并发。")
	}
	if promptAuditEnabled == nil {
		return apiquota.RushOfferPublication{}, idempotency.Completion{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Prompt audit selection required", "发布额度包前必须声明是否开启提示词审计。", "promptAuditEnabled", "required", "请选择是否开启提示词审计。")
	}
	var flexibleQuotaSaleOpen bool
	if err := tx.QueryRow(ctx, `
		SELECT billing_mode = 'metered_usd_quota'
		   AND accepting_orders = true
		   AND available_usd_allowance > 0
		   AND quota_expires_at > $2::timestamptz + interval '24 hours'
		FROM api_services
		WHERE id = $1
	`, publication.Batch.APIServiceID, now).Scan(&flexibleQuotaSaleOpen); err != nil {
		return apiquota.RushOfferPublication{}, idempotency.Completion{}, internalStoreError()
	}
	if flexibleQuotaSaleOpen {
		return apiquota.RushOfferPublication{}, idempotency.Completion{}, invalidQuotaState("请先关闭该服务的自选额度接单，再发布限量额度包。")
	}
	if _, err := tx.Exec(ctx, `
		SELECT pg_advisory_xact_lock(hashtextextended('api-quota-system-slot:' || $1::uuid::text || ':' || $2, 0))
	`, publication.Batch.OwnerUserID, publication.Round.SystemSlotKey); err != nil {
		return apiquota.RushOfferPublication{}, idempotency.Completion{}, internalStoreError()
	}
	var allocatedCopies int
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(sum(allocation.copy_limit), 0)::integer
		FROM api_quota_allocations allocation
		JOIN api_quota_sale_rounds round ON round.id = allocation.sale_round_id
		WHERE round.owner_user_id = $1 AND round.system_slot_key = $2
		  AND round.status = 'scheduled' AND allocation.status IN ('planned', 'active')
	`, publication.Batch.OwnerUserID, publication.Round.SystemSlotKey).Scan(&allocatedCopies); err != nil {
		return apiquota.RushOfferPublication{}, idempotency.Completion{}, internalStoreError()
	}
	if allocatedCopies+publication.Round.Allocations[0].CopyLimit > 10 {
		return apiquota.RushOfferPublication{}, idempotency.Completion{}, domain.NewFieldError(http.StatusConflict, domain.CodeValidationFailed, "Rush slot copy limit exceeded", "同一卖家在同一平台场次最多发布 10 份。", "copies", "slot_limit", "本场剩余可发布份数不足。")
	}

	publication.Batch.ServiceTitle = serviceTitle
	publication.Batch.DistributionSystem = distributionSystem
	publication.Batch.ServiceOrderable = serviceOrderable
	publication.Batch.DeclaredTTFTBand = declaredTTFTBand
	publication.Batch.DeclaredMaxConcurrency = declaredMaxConcurrency
	publication.Batch.PerformanceConfirmedAt = performanceConfirmedAt
	publication.Batch.PromptAuditEnabled = promptAuditEnabled
	publication.Offer.DistributionSystem = distributionSystem

	_, err = tx.Exec(ctx, `
		INSERT INTO api_quota_batches (
			id, api_service_id, owner_user_id, source_type, source_label, status,
			declared_total_usd_allowance, unallocated_usd_allowance,
			sale_cutoff_at, expires_at, source_confirmed_at,
			created_at, updated_at, version
		) VALUES (
			$1, $2, $3, $4, $5, 'draft', $6, $6, $7, $8, $9, $10, $10, 1
		)
	`, publication.Batch.ID, publication.Batch.APIServiceID, publication.Batch.OwnerUserID,
		publication.Batch.SourceType, nullText(publication.Batch.SourceLabel),
		publication.Batch.DeclaredTotalUSDAllowance, publication.Batch.SaleCutoffAt,
		publication.Batch.ExpiresAt, publication.Batch.SourceConfirmedAt, now)
	if err != nil {
		return apiquota.RushOfferPublication{}, idempotency.Completion{}, mapAPIQuotaWriteError(err)
	}
	if appErr := insertAPIOperationDomainEvent(ctx, tx, "api_quota_batch", publication.Batch.ID, "api_quota_batch.created", publication.Batch.OwnerUserID, apiOperationActorUser, 1, publication.RequestID, map[string]any{
		"status": apiquota.BatchStatusDraft,
	}, now); appErr != nil {
		return apiquota.RushOfferPublication{}, idempotency.Completion{}, appErr
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO api_quota_offers (
			id, batch_id, api_service_id, owner_user_id, distribution_system,
			name, usd_allowance, price_cny, model_multiplier,
			five_hour_limit_mode, five_hour_limit_usd, daily_limit_mode, daily_limit_usd,
			delivery_mode, delivery_eta_minutes, sale_mode, status, sort_order,
			created_at, updated_at, version
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9,
			$10, $11, $12, $13, $14, $15, 'scheduled', 'draft', 0, $16, $16, 1
		)
	`, publication.Offer.ID, publication.Batch.ID, publication.Batch.APIServiceID,
		publication.Batch.OwnerUserID, distributionSystem, publication.Offer.Name,
		publication.Offer.USDAllowance, publication.Offer.PriceCNY, publication.Offer.ModelMultiplier,
		publication.Offer.QuotaUsagePolicy.FiveHour.Mode, nullNumeric(publication.Offer.QuotaUsagePolicy.FiveHour.AmountUSD),
		publication.Offer.QuotaUsagePolicy.Daily.Mode, nullNumeric(publication.Offer.QuotaUsagePolicy.Daily.AmountUSD),
		publication.Offer.DeliveryMode, publication.Offer.DeliveryETAMinutes, now)
	if err != nil {
		return apiquota.RushOfferPublication{}, idempotency.Completion{}, mapAPIQuotaWriteError(err)
	}
	if appErr := insertAPIOperationDomainEvent(ctx, tx, "api_quota_offer", publication.Offer.ID, "api_quota_offer.created", publication.Batch.OwnerUserID, apiOperationActorUser, 1, publication.RequestID, map[string]any{
		"status": apiquota.OfferStatusDraft, "saleMode": apiquota.SaleModeScheduled, "deliveryMode": publication.Offer.DeliveryMode,
	}, now); appErr != nil {
		return apiquota.RushOfferPublication{}, idempotency.Completion{}, appErr
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO api_quota_sale_rounds (
			id, batch_id, api_service_id, owner_user_id, system_slot_key, name,
			starts_at, ends_at, status, created_at, updated_at, version
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, 'scheduled', $9, $9, 1
		)
	`, publication.Round.ID, publication.Batch.ID, publication.Batch.APIServiceID,
		publication.Batch.OwnerUserID, publication.Round.SystemSlotKey, publication.Round.Name,
		publication.Round.StartsAt, publication.Round.EndsAt, now)
	if err != nil {
		return apiquota.RushOfferPublication{}, idempotency.Completion{}, mapAPIQuotaWriteError(err)
	}
	if appErr := insertAPIOperationDomainEvent(ctx, tx, "api_quota_sale_round", publication.Round.ID, "api_quota_sale_round.created", publication.Batch.OwnerUserID, apiOperationActorUser, 1, publication.RequestID, map[string]any{
		"status": "scheduled", "systemSlot": true,
	}, now); appErr != nil {
		return apiquota.RushOfferPublication{}, idempotency.Completion{}, appErr
	}
	allocation := publication.Round.Allocations[0]
	_, err = tx.Exec(ctx, `
		INSERT INTO api_quota_allocations (
			id, batch_id, offer_id, api_service_id, owner_user_id,
			sale_round_id, sale_mode, copy_limit, allocated_usd_allowance,
			status, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, 'scheduled', $7, $8, 'planned', $9, $9
		)
	`, allocation.ID, publication.Batch.ID, publication.Offer.ID,
		publication.Batch.APIServiceID, publication.Batch.OwnerUserID, publication.Round.ID,
		allocation.CopyLimit, allocation.AllocatedUSDAllowance, now)
	if err != nil {
		return apiquota.RushOfferPublication{}, idempotency.Completion{}, mapAPIQuotaWriteError(err)
	}
	if appErr := s.insertAPIQuotaCredentialRowsInTx(ctx, tx, publication.Batch.OwnerUserID, publication.Offer.ID, credentials, now); appErr != nil {
		return apiquota.RushOfferPublication{}, idempotency.Completion{}, appErr
	}
	if len(credentials) > 0 {
		var credentialVersion int64
		if err := tx.QueryRow(ctx, `
			UPDATE api_quota_offers SET version = version + 1, updated_at = $2
			WHERE id = $1
			RETURNING version
		`, publication.Offer.ID, now).Scan(&credentialVersion); err != nil {
			return apiquota.RushOfferPublication{}, idempotency.Completion{}, internalStoreError()
		}
		if appErr := insertAPIOperationDomainEvent(ctx, tx, "api_quota_offer", publication.Offer.ID, "api_quota_offer.credentials_imported", publication.Batch.OwnerUserID, apiOperationActorUser, credentialVersion, publication.RequestID, apiQuotaCredentialImportMetadata(len(credentials), credentials[0].DeliveryKind), now); appErr != nil {
			return apiquota.RushOfferPublication{}, idempotency.Completion{}, appErr
		}
	}

	publishedBatch, appErr := publishAPIQuotaBatchInTx(ctx, tx, apiquota.BatchActionInput{
		BatchID: publication.Batch.ID, OwnerUserID: publication.Batch.OwnerUserID, ExpectedVersion: 1, RequestID: publication.RequestID,
	}, now)
	if appErr != nil {
		return apiquota.RushOfferPublication{}, idempotency.Completion{}, appErr
	}
	publication.Batch = publishedBatch
	if err := scanAPIQuotaOffer(tx.QueryRow(ctx, `
		SELECT `+apiQuotaOfferColumns+`
		FROM api_quota_offers o
		WHERE o.id = $1 AND o.owner_user_id = $2
	`, publication.Offer.ID, publication.Batch.OwnerUserID), &publication.Offer); err != nil {
		return apiquota.RushOfferPublication{}, idempotency.Completion{}, internalStoreError()
	}
	publication.Round.Allocations[0].Status = "active"
	publication.Round.Allocations[0].AvailableCopies = publication.Round.Allocations[0].CopyLimit
	publication.Round.Allocations[0].ReturnedUSDAllowance = "0.000000"
	summary, appErr := getAPIQuotaCredentialSummary(ctx, tx, publication.Batch.OwnerUserID, publication.Offer.ID)
	if appErr != nil {
		return apiquota.RushOfferPublication{}, idempotency.Completion{}, appErr
	}
	publication.CredentialSummary = summary
	publication.CredentialImported = len(credentials)

	completion, appErr := buildCompletion(publication)
	if appErr != nil {
		return apiquota.RushOfferPublication{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return apiquota.RushOfferPublication{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return apiquota.RushOfferPublication{}, idempotency.Completion{}, internalStoreError()
	}
	return publication, completion, nil
}

func (s *Store) ImportAPIQuotaCredentials(ctx context.Context, ownerUserID, offerID, requestID string, rows []apiquota.CredentialImportRow, now time.Time) (apiquota.CredentialSummary, *domain.AppError) {
	if s == nil || s.contactCodec == nil {
		return apiquota.CredentialSummary{}, internalStoreError()
	}
	result, _, appErr := executeAPIQuotaCommand(ctx, s, nil, now, func(tx pgx.Tx) (apiquota.CredentialImportResult, *domain.AppError) {
		return s.importAPIQuotaCredentialsInTx(ctx, tx, ownerUserID, offerID, requestID, rows, now)
	}, nil)
	return result.Summary, appErr
}

func (s *Store) ImportAPIQuotaCredentialsWithIdempotency(ctx context.Context, entry idempotency.Entry, ownerUserID, offerID, requestID string, rows []apiquota.CredentialImportRow, now time.Time, buildCompletion apiquota.CredentialImportCompletionBuilder) (apiquota.CredentialImportResult, idempotency.Completion, *domain.AppError) {
	if s == nil || s.contactCodec == nil {
		return apiquota.CredentialImportResult{}, idempotency.Completion{}, internalStoreError()
	}
	return executeAPIQuotaCommand(ctx, s, &entry, now, func(tx pgx.Tx) (apiquota.CredentialImportResult, *domain.AppError) {
		return s.importAPIQuotaCredentialsInTx(ctx, tx, ownerUserID, offerID, requestID, rows, now)
	}, buildCompletion)
}

func (s *Store) importAPIQuotaCredentialsInTx(ctx context.Context, tx pgx.Tx, ownerUserID, offerID, requestID string, rows []apiquota.CredentialImportRow, now time.Time) (apiquota.CredentialImportResult, *domain.AppError) {
	if len(rows) == 0 || len(rows) > 5000 {
		return apiquota.CredentialImportResult{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Credential rows invalid", "凭据导入行数无效。", "file", "invalid", "凭据导入行数无效。")
	}
	var deliveryMode, offerStatus string
	err := tx.QueryRow(ctx, `
		SELECT delivery_mode, status
		FROM api_quota_offers
		WHERE id = $1 AND owner_user_id = $2
		FOR UPDATE
	`, offerID, ownerUserID).Scan(&deliveryMode, &offerStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return apiquota.CredentialImportResult{}, quotaNotFound("额度包不存在。")
	}
	if err != nil {
		return apiquota.CredentialImportResult{}, internalStoreError()
	}
	if deliveryMode != apiquota.DeliveryModePreimported || offerStatus == apiquota.OfferStatusArchived {
		return apiquota.CredentialImportResult{}, invalidQuotaState("当前额度包不能导入预置交付凭据。")
	}
	deliveryKind := rows[0].DeliveryKind
	var existingKind string
	err = tx.QueryRow(ctx, `
		SELECT delivery_kind
		FROM api_quota_credentials
		WHERE api_quota_offer_id = $1
		ORDER BY created_at, id
		LIMIT 1
	`, offerID).Scan(&existingKind)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return apiquota.CredentialImportResult{}, internalStoreError()
	}
	if existingKind != "" && existingKind != deliveryKind {
		return apiquota.CredentialImportResult{}, domain.NewFieldError(http.StatusConflict, domain.CodeInvalidStateTransition, "Credential type conflict", "同一额度包不能混合两种交付凭据模板。", "deliveryKind", "conflict", "同一额度包不能混合两种交付凭据模板。")
	}
	if appErr := s.insertAPIQuotaCredentialRowsInTx(ctx, tx, ownerUserID, offerID, rows, now); appErr != nil {
		return apiquota.CredentialImportResult{}, appErr
	}
	var offerVersion int64
	if err := tx.QueryRow(ctx, `
		UPDATE api_quota_offers
		SET version = version + 1, updated_at = $3
		WHERE id = $1 AND owner_user_id = $2
		RETURNING version
	`, offerID, ownerUserID, now).Scan(&offerVersion); err != nil {
		return apiquota.CredentialImportResult{}, internalStoreError()
	}
	if appErr := insertAPIOperationDomainEvent(ctx, tx, "api_quota_offer", offerID, "api_quota_offer.credentials_imported", ownerUserID, apiOperationActorUser, offerVersion, requestID, apiQuotaCredentialImportMetadata(len(rows), deliveryKind), now); appErr != nil {
		return apiquota.CredentialImportResult{}, appErr
	}
	summary, appErr := getAPIQuotaCredentialSummary(ctx, tx, ownerUserID, offerID)
	if appErr != nil {
		return apiquota.CredentialImportResult{}, appErr
	}
	return apiquota.CredentialImportResult{Imported: len(rows), Summary: summary}, nil
}

func (s *Store) insertAPIQuotaCredentialRowsInTx(ctx context.Context, tx pgx.Tx, ownerUserID, offerID string, rows []apiquota.CredentialImportRow, now time.Time) *domain.AppError {
	if len(rows) == 0 {
		return nil
	}
	deliveryKind := rows[0].DeliveryKind
	for _, row := range rows {
		if row.DeliveryKind != deliveryKind {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Credential type mixed", "一次导入不能混合两种交付凭据模板。", "file", "mixed_type", "一次导入不能混合两种交付凭据模板。")
		}
		secret := row.APIKey
		if row.DeliveryKind == apiorder.DeliveryKindLoginAccount {
			secret = row.Password
		}
		credentialID := uuid.NewString()
		fieldType := contactFieldQuotaAPIKey
		if row.DeliveryKind == apiorder.DeliveryKindLoginAccount {
			fieldType = contactFieldQuotaPassword
		}
		encoded, encodeErr := s.contactCodec.encode(secret, credentialID, fieldType)
		if encodeErr != nil {
			return internalStoreError()
		}
		var apiKeyCiphertext, apiKeyNonce, passwordCiphertext, passwordNonce []byte
		if row.DeliveryKind == apiorder.DeliveryKindAPIKeyEndpoint {
			apiKeyCiphertext = encoded.Ciphertext
			apiKeyNonce = encoded.Nonce
		} else {
			passwordCiphertext = encoded.Ciphertext
			passwordNonce = encoded.Nonce
		}
		fingerprint := s.contactCodec.fingerprint(apiQuotaCredentialFingerprintMaterial(row))
		_, err := tx.Exec(ctx, `
			INSERT INTO api_quota_credentials (
				id, api_quota_offer_id, seller_user_id, delivery_kind,
				api_base_url, panel_login_url, username, instructions,
				api_key_ciphertext, api_key_nonce, password_ciphertext, password_nonce,
				secret_encryption_key_version, secret_encryption_format, secret_fingerprint,
				created_at, updated_at
			) VALUES (
				$1, $2, $3, $4,
				$5, $6, $7, $8,
				$9, $10, $11, $12,
				$13, $14, decode($15, 'hex'),
				$16, $16
			)
		`, credentialID, offerID, ownerUserID, row.DeliveryKind,
			nullText(row.APIBaseURL), nullText(row.PanelLoginURL), nullText(row.Username), nullText(row.Instructions),
			apiKeyCiphertext, apiKeyNonce, passwordCiphertext, passwordNonce,
			encoded.EncryptionKeyVersion, encoded.CipherFormat, fingerprint, now)
		if err != nil {
			if isUniqueViolationOnConstraint(err, "ux_api_quota_credentials_seller_fingerprint") {
				return domain.NewFieldError(http.StatusConflict, domain.CodeInvalidStateTransition, "Credential already imported", "CSV 中存在卖家已经导入的凭据，本次导入未保存任何行。", "file", "duplicate", "CSV 中存在卖家已经导入的凭据，本次导入未保存任何行。")
			}
			return internalStoreError()
		}
	}
	return nil
}

func (s *Store) GetAPIQuotaCredentialSummary(ctx context.Context, ownerUserID, offerID string) (apiquota.CredentialSummary, *domain.AppError) {
	if s == nil || s.pool == nil {
		return apiquota.CredentialSummary{}, internalStoreError()
	}
	return getAPIQuotaCredentialSummary(ctx, s.pool, ownerUserID, offerID)
}

func getAPIQuotaCredentialSummary(ctx context.Context, q queryer, ownerUserID, offerID string) (apiquota.CredentialSummary, *domain.AppError) {
	var summary apiquota.CredentialSummary
	err := q.QueryRow(ctx, `
		SELECT o.id::text,
		       count(c.id) FILTER (WHERE c.status = 'available')::integer,
		       count(c.id) FILTER (WHERE c.status = 'reserved')::integer,
		       count(c.id) FILTER (WHERE c.status = 'delivered')::integer,
		       count(c.id) FILTER (WHERE c.status = 'retired')::integer
		FROM api_quota_offers o
		LEFT JOIN api_quota_credentials c ON c.api_quota_offer_id = o.id AND c.seller_user_id = o.owner_user_id
		WHERE o.id = $1 AND o.owner_user_id = $2
		GROUP BY o.id
	`, offerID, ownerUserID).Scan(&summary.OfferID, &summary.Available, &summary.Reserved, &summary.Delivered, &summary.Retired)
	if errors.Is(err, pgx.ErrNoRows) {
		return apiquota.CredentialSummary{}, quotaNotFound("额度包不存在。")
	}
	if err != nil {
		return apiquota.CredentialSummary{}, internalStoreError()
	}
	return summary, nil
}

func apiQuotaCredentialFingerprintMaterial(row apiquota.CredentialImportRow) string {
	if row.DeliveryKind == apiorder.DeliveryKindAPIKeyEndpoint {
		return row.DeliveryKind + "\x00" + row.APIKey
	}
	return row.DeliveryKind + "\x00" + row.PanelLoginURL + "\x00" + row.Username + "\x00" + row.Password
}

func (s *Store) CreateAPIQuotaOrderWithIdempotency(ctx context.Context, entry idempotency.Entry, input apiquota.CreateOrderInput, now time.Time, buildCompletion apiorder.CompletionBuilder) (apiorder.Order, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return apiorder.Order{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return apiorder.Order{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return apiorder.Order{}, idempotency.Completion{}, appErr
	}
	orderContext, appErr := getAPIQuotaOrderContext(ctx, tx, input, now)
	if appErr != nil {
		return apiorder.Order{}, idempotency.Completion{}, appErr
	}
	if appErr := ensureAPIServiceCatalogActiveInTx(ctx, tx, orderContext.APIServiceID); appErr != nil {
		return apiorder.Order{}, idempotency.Completion{}, appErr
	}
	if orderContext.OwnerUserID == input.BuyerUserID {
		return apiorder.Order{}, idempotency.Completion{}, invalidQuotaState("不能购买自己发布的额度包。")
	}
	if appErr := ensureAPIServicePublishAllowedInTx(ctx, tx, orderContext.OwnerUserID, now); appErr != nil {
		return apiorder.Order{}, idempotency.Completion{}, appErr
	}
	buyerMethod, buyerVersion, appErr := lockWechatContactVersionForOwnerAndScope(ctx, tx, input.BuyerContactMethodID, input.BuyerUserID, contact.UsageScopeBuyer, "buyerContactMethodId", "购买额度包前必须先配置微信联系方式。")
	if appErr != nil {
		return apiorder.Order{}, idempotency.Completion{}, appErr
	}
	ownerSnapshots, appErr := lockAPIServiceOwnerContactSnapshots(ctx, tx, orderContext.APIServiceID, orderContext.OwnerUserID, orderContext.OwnerContactMethodID, nil, "卖家联系方式当前不可用。")
	if appErr != nil {
		return apiorder.Order{}, idempotency.Completion{}, appErr
	}
	primaryOwnerContact := ownerSnapshots[0]
	var accessModeExists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM api_service_access_modes
			WHERE api_service_id = $1 AND access_mode = $2
		)
	`, orderContext.APIServiceID, strings.TrimSpace(input.SelectedAccessMode)).Scan(&accessModeExists); err != nil {
		return apiorder.Order{}, idempotency.Completion{}, internalStoreError()
	}
	if !accessModeExists {
		return apiorder.Order{}, idempotency.Completion{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Access mode unavailable", "选择的接入方式不可用。", "selectedAccessMode", "invalid", "选择的接入方式不可用。")
	}
	var paymentInstructions, paymentQRCode string
	err = tx.QueryRow(ctx, `
		SELECT payment_instructions, COALESCE(payment_qr_code_data_url, '')
		FROM api_service_payment_options
		WHERE api_service_id = $1 AND payment_method = $2 AND enabled = true
		  AND payment_method IN ('wechat', 'alipay')
	`, orderContext.APIServiceID, strings.TrimSpace(input.PaymentMethod)).Scan(&paymentInstructions, &paymentQRCode)
	if errors.Is(err, pgx.ErrNoRows) {
		return apiorder.Order{}, idempotency.Completion{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Payment method unavailable", "选择的付款方式不可用。", "paymentMethod", "invalid", "选择的付款方式不可用。")
	}
	if err != nil {
		return apiorder.Order{}, idempotency.Completion{}, internalStoreError()
	}
	if appErr := ensureAPIBuyerPendingCapacityInTx(ctx, tx, input.BuyerUserID, "quota", orderContext.OfferID, now); appErr != nil {
		return apiorder.Order{}, idempotency.Completion{}, appErr
	}

	allocationID, round, appErr := claimAPIQuotaRoundAndAllocation(ctx, tx, input, orderContext, now)
	if appErr != nil {
		return apiorder.Order{}, idempotency.Completion{}, appErr
	}
	var inventoryUnitID string
	err = tx.QueryRow(ctx, `
		SELECT id::text
		FROM api_quota_inventory_units
		WHERE allocation_id = $1 AND offer_id = $2 AND batch_id = $3 AND status = 'available'
		ORDER BY id
		FOR UPDATE SKIP LOCKED
		LIMIT 1
	`, allocationID, orderContext.OfferID, orderContext.BatchID).Scan(&inventoryUnitID)
	if errors.Is(err, pgx.ErrNoRows) {
		return apiorder.Order{}, idempotency.Completion{}, domain.NewError(http.StatusConflict, domain.CodeAPIQuotaSoldOut, "Quota offer sold out", "当前额度包已售罄。")
	}
	if err != nil {
		return apiorder.Order{}, idempotency.Completion{}, internalStoreError()
	}
	credentialID := ""
	if orderContext.DeliveryMode == apiquota.DeliveryModePreimported {
		err = tx.QueryRow(ctx, `
			SELECT id::text
			FROM api_quota_credentials
			WHERE api_quota_offer_id = $1 AND seller_user_id = $2 AND status = 'available'
			ORDER BY id
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		`, orderContext.OfferID, orderContext.OwnerUserID).Scan(&credentialID)
		if errors.Is(err, pgx.ErrNoRows) {
			return apiorder.Order{}, idempotency.Completion{}, domain.NewError(http.StatusConflict, domain.CodeAPIQuotaCredentialUnavailable, "Credential inventory unavailable", "当前额度包暂无可分配交付凭据。")
		}
		if err != nil {
			return apiorder.Order{}, idempotency.Completion{}, internalStoreError()
		}
	}

	intentID := uuid.NewString()
	orderID := uuid.NewString()
	snapshot, appErr := buildAPIQuotaSnapshot(orderContext, round)
	if appErr != nil {
		return apiorder.Order{}, idempotency.Completion{}, appErr
	}
	paymentWindowMinutes := 10
	if orderContext.SaleMode == apiquota.SaleModeScheduled {
		paymentWindowMinutes = 5
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO api_purchase_intents (
			id, purchase_kind, api_service_id, api_service_owner_user_id,
			buyer_user_id, owner_user_id,
			buyer_contact_method_id, buyer_contact_method_version_id,
			owner_contact_method_id, owner_contact_method_version_id,
			status, requested_cny_amount, requested_usd_allowance,
			selected_access_mode, selected_package_id, selected_package_snapshot,
			service_version_snapshot, service_title_snapshot,
			distribution_system_snapshot, billing_mode_snapshot,
			buyer_contact_type_snapshot, buyer_contact_label_snapshot,
			owner_contact_type_snapshot, owner_contact_label_snapshot,
			declared_cny_per_usd_allowance_snapshot,
			declared_max_usd_allowance_per_intent_snapshot,
			minimum_intent_cny_snapshot, maximum_intent_cny_snapshot,
			pricing_snapshot,
			five_hour_limit_mode_snapshot, five_hour_limit_usd_snapshot,
			daily_limit_mode_snapshot, daily_limit_usd_snapshot,
			buyer_note,
			api_quota_batch_id, api_quota_offer_id, api_quota_sale_round_id,
			api_quota_allocation_id, api_quota_inventory_unit_id, quota_offer_snapshot,
			created_at, updated_at, version, prompt_audit_enabled_snapshot
		) VALUES (
			$1, 'limited_quota_offer', $2, $3, $4, $3,
			$5, $6, $7, $8,
			'ordered', $9, $10,
			$11, NULL, NULL,
			$12, $13, $14, $15,
			$16, $17, $18, $19,
			$20, NULL, $21, $22,
			$23::jsonb,
			$24, $25, $26, $27,
			$28,
			$29, $30, $31, $32, $33, $23::jsonb,
				$34, $34, 1, $35
		)
	`, intentID, orderContext.APIServiceID, orderContext.OwnerUserID, input.BuyerUserID,
		buyerMethod.ID, buyerVersion.ID, primaryOwnerContact.ContactMethodID, primaryOwnerContact.ContactMethodVersionID,
		orderContext.PriceCNY, orderContext.USDAllowance, strings.TrimSpace(input.SelectedAccessMode),
		orderContext.ServiceVersion, orderContext.ServiceTitle, orderContext.DistributionSystem, orderContext.BillingMode,
		buyerMethod.Type, buyerMethod.Label, primaryOwnerContact.Type, primaryOwnerContact.Label,
		orderContext.CNYPerUSD, orderContext.MinimumIntentCNY, nullNumeric(orderContext.MaximumIntentCNY),
		snapshot,
		orderContext.QuotaUsagePolicy.FiveHour.Mode, nullNumeric(orderContext.QuotaUsagePolicy.FiveHour.AmountUSD),
		orderContext.QuotaUsagePolicy.Daily.Mode, nullNumeric(orderContext.QuotaUsagePolicy.Daily.AmountUSD),
		nullText(input.BuyerNote), orderContext.BatchID, orderContext.OfferID, nullUUID(round.ID),
		allocationID, inventoryUnitID, now, orderContext.PromptAuditEnabled)
	if err != nil {
		return apiorder.Order{}, idempotency.Completion{}, mapAPIQuotaWriteError(err)
	}
	if appErr := insertAPIPurchaseIntentOwnerContactSnapshotsInTx(ctx, tx, apiintent.Intent{
		ID:                    intentID,
		OwnerUserID:           orderContext.OwnerUserID,
		CreatedAt:             now,
		OwnerContactSnapshots: ownerSnapshots,
	}); appErr != nil {
		return apiorder.Order{}, idempotency.Completion{}, appErr
	}

	order := apiorder.Order{
		ID:                            orderID,
		PurchaseKind:                  apiorder.PurchaseKindLimitedQuotaOffer,
		APIPurchaseIntentID:           intentID,
		APIServiceID:                  orderContext.APIServiceID,
		BuyerUserID:                   input.BuyerUserID,
		SellerUserID:                  orderContext.OwnerUserID,
		Status:                        apiorder.StatusPendingPayment,
		DisputeStatus:                 apiorder.DisputeStatusNone,
		CommercialOutcome:             apiorder.CommercialOutcomePending,
		ServiceTitleSnapshot:          orderContext.ServiceTitle,
		ServiceVersionSnapshot:        orderContext.ServiceVersion,
		BillingModeSnapshot:           orderContext.BillingMode,
		RequestedUSDAllowanceSnapshot: orderContext.USDAllowance,
		CNYPerUSDAllowanceSnapshot:    orderContext.CNYPerUSD,
		PricingSnapshot:               snapshot,
		ProbeConnectionIDSnapshot:     orderContext.ProbeConnectionID,
		APIBaseURLSnapshot:            orderContext.ProbeBaseURL,
		NormalizedAPIBaseURLSnapshot:  orderContext.NormalizedProbeBaseURL,
		QuotaUsagePolicySnapshot:      orderContext.QuotaUsagePolicy,
		PromptAuditEnabledSnapshot:    orderContext.PromptAuditEnabled,
		APIQuotaBatchID:               orderContext.BatchID,
		APIQuotaOfferID:               orderContext.OfferID,
		APIQuotaSaleRoundID:           round.ID,
		APIQuotaAllocationID:          allocationID,
		APIQuotaInventoryUnitID:       inventoryUnitID,
		APIQuotaCredentialID:          credentialID,
		QuotaOfferSnapshot:            snapshot,
		QuotaOfferNameSnapshot:        orderContext.OfferName,
		QuotaUSDAllowanceSnapshot:     orderContext.USDAllowance,
		QuotaPriceCNYSnapshot:         orderContext.PriceCNY,
		QuotaCNYPerUSDSnapshot:        orderContext.CNYPerUSD,
		QuotaModelMultiplierSnapshot:  orderContext.ModelMultiplier,
		QuotaSaleCutoffAtSnapshot:     &orderContext.SaleCutoffAt,
		QuotaExpiresAtSnapshot:        &orderContext.ExpiresAt,
		QuotaSaleModeSnapshot:         orderContext.SaleMode,
		QuotaDistributionSnapshot:     orderContext.DistributionSystem,
		QuotaTTFTBandSnapshot:         orderContext.DeclaredTTFTBand,
		QuotaDeclaredMaxConcurrency:   orderContext.DeclaredMaxConcurrency,
		QuotaPerformanceConfirmedAt:   orderContext.PerformanceConfirmedAt,
		QuotaPerformanceUnverified:    true,
		QuotaDeliveryETAMinutes:       orderContext.DeliveryETAMinutes,
		QuotaDeliveryMode:             orderContext.DeliveryMode,
		Amount:                        orderContext.PriceCNY,
		Currency:                      "CNY",
		SelectedPaymentMethod:         strings.TrimSpace(input.PaymentMethod),
		PaymentWindowMinutesSnapshot:  paymentWindowMinutes,
		PaymentExpiresAt:              now.Add(time.Duration(paymentWindowMinutes) * time.Minute),
		PaymentInstructionsSnapshot:   paymentInstructions,
		PaymentQRCodeDataURLSnapshot:  paymentQRCode,
		CreatedAt:                     now,
		UpdatedAt:                     now,
		Version:                       1,
	}
	if round.ID != "" {
		order.QuotaRoundStartsAtSnapshot = &round.StartsAt
		order.QuotaRoundEndsAtSnapshot = &round.EndsAt
	}
	err = insertAPIOrderWithNumberRetry(&order, apiorder.GenerateOrderNo, func() error {
		commandTag, insertErr := tx.Exec(ctx, `
		INSERT INTO api_orders (
			id, purchase_kind, api_purchase_intent_id, api_service_id,
			buyer_user_id, seller_user_id, status, dispute_status,
			service_title_snapshot, service_version_snapshot, billing_mode_snapshot,
			requested_usd_allowance_snapshot, cny_per_usd_allowance_snapshot, pricing_snapshot,
			api_quota_batch_id, api_quota_offer_id, api_quota_sale_round_id,
			api_quota_allocation_id, api_quota_inventory_unit_id, api_quota_credential_id,
			quota_offer_snapshot, quota_offer_name_snapshot,
			quota_usd_allowance_snapshot, quota_price_cny_snapshot, quota_cny_per_usd_snapshot,
			quota_model_multiplier_snapshot, quota_sale_cutoff_at_snapshot, quota_expires_at_snapshot,
			quota_sale_mode_snapshot, quota_round_starts_at_snapshot, quota_round_ends_at_snapshot,
			quota_distribution_system_snapshot, quota_ttft_band_snapshot,
			quota_declared_max_concurrency_snapshot, quota_performance_confirmed_at_snapshot,
			quota_performance_unverified_snapshot, quota_delivery_eta_minutes_snapshot,
				quota_delivery_mode_snapshot,
				probe_connection_id_snapshot, api_base_url_snapshot, normalized_api_base_url_snapshot,
				five_hour_limit_mode_snapshot, five_hour_limit_usd_snapshot,
			daily_limit_mode_snapshot, daily_limit_usd_snapshot,
			amount, currency, selected_payment_method, payment_window_minutes_snapshot,
			payment_expires_at, payment_instructions_snapshot, payment_qr_code_data_url_snapshot,
				created_at, updated_at, version, order_no, prompt_audit_enabled_snapshot
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13, $14::jsonb,
			$15, $16, $17, $18, $19, $20,
			$14::jsonb, $21, $12, $22, $13, $23, $24, $25,
				$26, $27, $28, $29, $30, $31, $32, true, $33, $34,
				$47, $48, $49,
				$35, $36, $37, $38,
				$22, 'CNY', $39, $40, $41, $42, $43, $44, $44, 1, $45, $46
		)
		ON CONFLICT ON CONSTRAINT ux_api_orders_order_no DO NOTHING
	`, order.ID, order.PurchaseKind, order.APIPurchaseIntentID, order.APIServiceID,
			order.BuyerUserID, order.SellerUserID, order.Status, order.DisputeStatus,
			order.ServiceTitleSnapshot, order.ServiceVersionSnapshot, order.BillingModeSnapshot,
			order.RequestedUSDAllowanceSnapshot, order.CNYPerUSDAllowanceSnapshot, snapshot,
			order.APIQuotaBatchID, order.APIQuotaOfferID, nullUUID(order.APIQuotaSaleRoundID),
			order.APIQuotaAllocationID, order.APIQuotaInventoryUnitID, nullUUID(order.APIQuotaCredentialID),
			order.QuotaOfferNameSnapshot, order.QuotaPriceCNYSnapshot, order.QuotaModelMultiplierSnapshot,
			order.QuotaSaleCutoffAtSnapshot, order.QuotaExpiresAtSnapshot, order.QuotaSaleModeSnapshot,
			order.QuotaRoundStartsAtSnapshot, order.QuotaRoundEndsAtSnapshot, order.QuotaDistributionSnapshot,
			order.QuotaTTFTBandSnapshot, order.QuotaDeclaredMaxConcurrency, order.QuotaPerformanceConfirmedAt,
			order.QuotaDeliveryETAMinutes, order.QuotaDeliveryMode,
			order.QuotaUsagePolicySnapshot.FiveHour.Mode, nullNumeric(order.QuotaUsagePolicySnapshot.FiveHour.AmountUSD),
			order.QuotaUsagePolicySnapshot.Daily.Mode, nullNumeric(order.QuotaUsagePolicySnapshot.Daily.AmountUSD),
			order.SelectedPaymentMethod,
			order.PaymentWindowMinutesSnapshot, order.PaymentExpiresAt, order.PaymentInstructionsSnapshot,
			nullText(order.PaymentQRCodeDataURLSnapshot), now, order.OrderNo, order.PromptAuditEnabledSnapshot,
			nullUUID(order.ProbeConnectionIDSnapshot), nullText(order.APIBaseURLSnapshot), nullText(order.NormalizedAPIBaseURLSnapshot))
		if insertErr != nil {
			return insertErr
		}
		if commandTag.RowsAffected() == 0 {
			return errAPIOrderNumberCollision
		}
		return nil
	})
	if err != nil {
		return apiorder.Order{}, idempotency.Completion{}, mapAPIQuotaWriteError(err)
	}
	command, err := tx.Exec(ctx, `
		UPDATE api_quota_inventory_units
		SET status = 'reserved', reserved_order_id = $2, reserved_at = $3, updated_at = $3
		WHERE id = $1 AND status = 'available'
	`, inventoryUnitID, order.ID, now)
	if err != nil || command.RowsAffected() != 1 {
		return apiorder.Order{}, idempotency.Completion{}, internalStoreError()
	}
	if credentialID != "" {
		command, err = tx.Exec(ctx, `
			UPDATE api_quota_credentials
			SET status = 'reserved', reserved_order_id = $2, reserved_at = $3, updated_at = $3
			WHERE id = $1 AND status = 'available'
		`, credentialID, order.ID, now)
		if err != nil || command.RowsAffected() != 1 {
			return apiorder.Order{}, idempotency.Completion{}, internalStoreError()
		}
	}
	if round.ID != "" {
		command, err = tx.Exec(ctx, `
			UPDATE api_quota_round_claims SET api_order_id = $3
			WHERE sale_round_id = $1 AND buyer_user_id = $2 AND api_order_id IS NULL
		`, round.ID, input.BuyerUserID, order.ID)
		if err != nil || command.RowsAffected() != 1 {
			return apiorder.Order{}, idempotency.Completion{}, internalStoreError()
		}
	}
	if appErr := insertAPIOrderEventInTx(ctx, tx, order, input.BuyerUserID, apiorder.EventCreated, "", order.Status, "", input.RequestID, now); appErr != nil {
		return apiorder.Order{}, idempotency.Completion{}, appErr
	}
	if appErr := insertAPIOrderDomainEventAndNotificationInTx(ctx, tx, order, input.BuyerUserID, apiorder.EventCreated, input.RequestID, now); appErr != nil {
		return apiorder.Order{}, idempotency.Completion{}, appErr
	}
	order = apiorder.WithAfterSalesProjection(order, now)
	completion, appErr := buildCompletion(order)
	if appErr != nil {
		return apiorder.Order{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return apiorder.Order{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return apiorder.Order{}, idempotency.Completion{}, internalStoreError()
	}
	return order, completion, nil
}

type apiQuotaOrderContext struct {
	OfferID                  string
	BatchID                  string
	APIServiceID             string
	OwnerUserID              string
	OwnerContactMethodID     string
	OfferName                string
	USDAllowance             string
	PriceCNY                 string
	CNYPerUSD                string
	ModelMultiplier          string
	QuotaUsagePolicy         apimarket.QuotaUsagePolicy
	DeliveryMode             string
	DeliveryETAMinutes       int
	SaleMode                 string
	DistributionSystem       string
	OfferStatus              string
	BatchStatus              string
	SaleCutoffAt             time.Time
	ExpiresAt                time.Time
	ServiceTitle             string
	ServiceVersion           int64
	BillingMode              string
	MinimumIntentCNY         string
	MaximumIntentCNY         string
	AccountPoolType          string
	AccountPoolCustomName    string
	MerchantRefundCommitment bool
	DeclaredTTFTBand         string
	DeclaredMaxConcurrency   int
	PerformanceConfirmedAt   *time.Time
	PromptAuditEnabled       *bool
	ProbeConnectionID        string
	ProbeBaseURL             string
	NormalizedProbeBaseURL   string
	ServiceOrderable         bool
}

func getAPIQuotaOrderContext(ctx context.Context, tx pgx.Tx, input apiquota.CreateOrderInput, now time.Time) (apiQuotaOrderContext, *domain.AppError) {
	var item apiQuotaOrderContext
	err := tx.QueryRow(ctx, `
		SELECT o.id::text, o.batch_id::text, o.api_service_id::text, o.owner_user_id::text,
		       s.owner_contact_method_id::text, o.name, o.usd_allowance::text, o.price_cny::text,
		       (o.price_cny / o.usd_allowance)::numeric(18,6)::text,
		       o.model_multiplier::text,
		       o.five_hour_limit_mode, COALESCE(o.five_hour_limit_usd::text, ''),
		       o.daily_limit_mode, COALESCE(o.daily_limit_usd::text, ''),
		       o.delivery_mode, o.delivery_eta_minutes,
		       o.sale_mode, o.distribution_system, o.status, b.status,
		       b.sale_cutoff_at, b.expires_at,
		       s.title, s.version, s.billing_mode,
		       s.minimum_intent_cny::text, COALESCE(s.maximum_intent_cny::text, ''),
		       COALESCE(s.account_pool_type, ''), COALESCE(s.account_pool_custom_name, ''), s.merchant_refund_commitment,
		       COALESCE(s.declared_ttft_band, ''), COALESCE(s.declared_max_concurrency, 0),
			       s.performance_confirmed_at, s.prompt_audit_enabled,
			       probe_connection.id::text, probe_connection.base_url, probe_connection.normalized_base_url,
			       (`+apiServiceFulfillmentReadyPredicate("s")+`)
		FROM api_quota_offers o
		JOIN api_quota_batches b ON b.id = o.batch_id AND b.api_service_id = o.api_service_id AND b.owner_user_id = o.owner_user_id
			JOIN api_services s ON s.id = o.api_service_id AND s.owner_user_id = o.owner_user_id
			JOIN api_probe_connections probe_connection
			  ON probe_connection.id = s.probe_connection_id
			 AND probe_connection.owner_user_id = s.owner_user_id
		WHERE o.id = $1
			FOR SHARE OF o, b, s, probe_connection
	`, input.OfferID).Scan(
		&item.OfferID, &item.BatchID, &item.APIServiceID, &item.OwnerUserID,
		&item.OwnerContactMethodID, &item.OfferName, &item.USDAllowance, &item.PriceCNY,
		&item.CNYPerUSD, &item.ModelMultiplier,
		&item.QuotaUsagePolicy.FiveHour.Mode, &item.QuotaUsagePolicy.FiveHour.AmountUSD,
		&item.QuotaUsagePolicy.Daily.Mode, &item.QuotaUsagePolicy.Daily.AmountUSD,
		&item.DeliveryMode, &item.DeliveryETAMinutes,
		&item.SaleMode, &item.DistributionSystem, &item.OfferStatus, &item.BatchStatus,
		&item.SaleCutoffAt, &item.ExpiresAt, &item.ServiceTitle, &item.ServiceVersion,
		&item.BillingMode, &item.MinimumIntentCNY, &item.MaximumIntentCNY,
		&item.AccountPoolType, &item.AccountPoolCustomName, &item.MerchantRefundCommitment,
		&item.DeclaredTTFTBand, &item.DeclaredMaxConcurrency, &item.PerformanceConfirmedAt,
		&item.PromptAuditEnabled,
		&item.ProbeConnectionID, &item.ProbeBaseURL, &item.NormalizedProbeBaseURL,
		&item.ServiceOrderable,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return apiQuotaOrderContext{}, quotaNotFound("额度包不存在。")
	}
	if err != nil {
		return apiQuotaOrderContext{}, internalStoreError()
	}
	if item.BatchStatus != apiquota.BatchStatusPublished || item.OfferStatus != apiquota.OfferStatusPublished || !item.ServiceOrderable {
		return apiQuotaOrderContext{}, invalidQuotaState("当前额度包不可购买。")
	}
	if !now.Before(item.SaleCutoffAt) || !now.Before(item.ExpiresAt) {
		return apiQuotaOrderContext{}, domain.NewError(http.StatusConflict, domain.CodeAPIQuotaBatchExpired, "Quota batch expired", "额度包已超过最晚下单时间。")
	}
	if appErr := validateAPIQuotaOrderServiceDeclaration(item); appErr != nil {
		return apiQuotaOrderContext{}, appErr
	}
	return item, nil
}

func validateAPIQuotaOrderServiceDeclaration(item apiQuotaOrderContext) *domain.AppError {
	if item.DeclaredMaxConcurrency < 1 {
		return invalidQuotaState("额度包服务体验声明不完整。")
	}
	return nil
}

func claimAPIQuotaRoundAndAllocation(ctx context.Context, tx pgx.Tx, input apiquota.CreateOrderInput, item apiQuotaOrderContext, now time.Time) (string, apiquota.SaleRound, *domain.AppError) {
	var allocationID string
	if item.SaleMode == apiquota.SaleModeContinuous {
		if strings.TrimSpace(input.SaleRoundID) != "" {
			return "", apiquota.SaleRound{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Sale round not allowed", "全天可买额度包不需要放量轮次。", "saleRoundId", "not_allowed", "全天可买额度包不需要放量轮次。")
		}
		err := tx.QueryRow(ctx, `
			SELECT id::text FROM api_quota_allocations
			WHERE offer_id = $1 AND batch_id = $2 AND sale_round_id IS NULL AND status = 'active'
		`, item.OfferID, item.BatchID).Scan(&allocationID)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", apiquota.SaleRound{}, domain.NewError(http.StatusConflict, domain.CodeAPIQuotaSoldOut, "Quota offer sold out", "当前额度包没有可用库存。")
		}
		if err != nil {
			return "", apiquota.SaleRound{}, internalStoreError()
		}
		return allocationID, apiquota.SaleRound{}, nil
	}
	if strings.TrimSpace(input.SaleRoundID) == "" {
		return "", apiquota.SaleRound{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Sale round required", "定时额度包必须选择当前放量轮次。", "saleRoundId", "required", "必须选择当前放量轮次。")
	}
	round := apiquota.SaleRound{ID: strings.TrimSpace(input.SaleRoundID)}
	err := tx.QueryRow(ctx, `
		SELECT batch_id::text, api_service_id::text, owner_user_id::text,
		       COALESCE(system_slot_key, ''), name, starts_at, ends_at, status, fulfillment_confirmed_at, created_at, updated_at, version
		FROM api_quota_sale_rounds
		WHERE id = $1 AND batch_id = $2 AND status = 'scheduled'
		FOR SHARE
	`, round.ID, item.BatchID).Scan(
		&round.BatchID, &round.APIServiceID, &round.OwnerUserID, &round.SystemSlotKey, &round.Name,
		&round.StartsAt, &round.EndsAt, &round.Status, &round.FulfillmentConfirmedAt, &round.CreatedAt, &round.UpdatedAt, &round.Version,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apiquota.SaleRound{}, domain.NewError(http.StatusConflict, domain.CodeAPIQuotaRoundEnded, "Sale round unavailable", "放量轮次不存在或已结束。")
	}
	if err != nil {
		return "", apiquota.SaleRound{}, internalStoreError()
	}
	if now.Before(round.StartsAt) {
		return "", apiquota.SaleRound{}, domain.NewError(http.StatusConflict, domain.CodeAPIQuotaNotStarted, "Sale round not started", "本轮尚未开始。")
	}
	if !now.Before(round.EndsAt) {
		return "", apiquota.SaleRound{}, domain.NewError(http.StatusConflict, domain.CodeAPIQuotaRoundEnded, "Sale round ended", "本轮已经结束。")
	}
	if round.SystemSlotKey != "" && round.FulfillmentConfirmedAt == nil {
		return "", apiquota.SaleRound{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Fulfillment confirmation required", "卖家尚未确认本场可按时履约，当前不能下单。")
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO api_quota_round_claims (id, sale_round_id, buyer_user_id, created_at)
		VALUES ($1, $2, $3, $4)
	`, uuid.NewString(), round.ID, input.BuyerUserID, now)
	if err != nil {
		if isUniqueViolation(err) {
			return "", apiquota.SaleRound{}, domain.NewError(http.StatusConflict, domain.CodeAPIQuotaBuyerRoundLimit, "Buyer round limit reached", "同一买家在本轮所有规格合计限购 1 份。")
		}
		return "", apiquota.SaleRound{}, internalStoreError()
	}
	err = tx.QueryRow(ctx, `
		SELECT id::text FROM api_quota_allocations
		WHERE offer_id = $1 AND batch_id = $2 AND sale_round_id = $3 AND status = 'active'
	`, item.OfferID, item.BatchID, round.ID).Scan(&allocationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apiquota.SaleRound{}, domain.NewError(http.StatusConflict, domain.CodeAPIQuotaSoldOut, "Quota offer sold out", "本轮没有该额度规格的可用库存。")
	}
	if err != nil {
		return "", apiquota.SaleRound{}, internalStoreError()
	}
	return allocationID, round, nil
}

func buildAPIQuotaSnapshot(item apiQuotaOrderContext, round apiquota.SaleRound) (string, *domain.AppError) {
	payload := map[string]any{
		"offerId": item.OfferID, "batchId": item.BatchID, "name": item.OfferName,
		"usdAllowance": item.USDAllowance, "priceCny": item.PriceCNY,
		"cnyPerUsd": item.CNYPerUSD, "modelMultiplier": item.ModelMultiplier,
		"saleMode": item.SaleMode, "saleCutoffAt": item.SaleCutoffAt,
		"expiresAt": item.ExpiresAt, "distributionSystem": item.DistributionSystem,
		"declaredTtftBand":            item.DeclaredTTFTBand,
		"declaredMaxConcurrency":      nullInt(item.DeclaredMaxConcurrency),
		"promptAuditEnabled":          item.PromptAuditEnabled,
		"accountPoolType":             nullText(item.AccountPoolType),
		"accountPoolLabel":            nullText(apimarket.AccountPoolLabel(apimarket.Service{AccountPoolType: item.AccountPoolType, AccountPoolCustomName: item.AccountPoolCustomName})),
		"merchantRefundCommitment":    item.MerchantRefundCommitment,
		"merchantRefundPolicyVersion": apimarket.MerchantRefundPolicyVersion,
		"serviceValidityExpiresAt":    item.ExpiresAt,
		"quotaUsagePolicy": map[string]any{
			"fiveHour":   map[string]any{"mode": item.QuotaUsagePolicy.FiveHour.Mode, "amountUsd": nullText(item.QuotaUsagePolicy.FiveHour.AmountUSD)},
			"daily":      map[string]any{"mode": item.QuotaUsagePolicy.Daily.Mode, "amountUsd": nullText(item.QuotaUsagePolicy.Daily.AmountUSD)},
			"scope":      apimarket.QuotaLimitScopePerBuyerCredential,
			"dailyReset": apimarket.QuotaDailyResetUTCPlus8CalendarDay,
		},
		"performanceConfirmedAt": item.PerformanceConfirmedAt,
		"performanceDisclaimer":  "历史商户声明，仅用于解释成交时事实",
		"deliveryEtaMinutes":     item.DeliveryETAMinutes, "deliveryMode": item.DeliveryMode,
		"serviceTitle": item.ServiceTitle, "serviceVersion": item.ServiceVersion,
	}
	if round.ID != "" {
		payload["saleRoundId"] = round.ID
		payload["roundStartsAt"] = round.StartsAt
		payload["roundEndsAt"] = round.EndsAt
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", internalStoreError()
	}
	return string(body), nil
}

var publicAPIQuotaOffersQuery = `
	SELECT ` + apiQuotaOfferColumns + `,
	       b.status, s.title, (` + apiServiceFulfillmentReadyPredicate("s") + `),
	       CASE WHEN s.merchant_identity_mode = 'store_alias'
	            THEN COALESCE(mp.display_name, u.display_name)
	            ELSE u.display_name END,
	       CASE WHEN s.merchant_identity_mode = 'store_alias' THEN 'merchant' ELSE 'individual' END,
	       COALESCE(CASE WHEN s.merchant_identity_mode = 'store_alias'
	            THEN mp.avatar_url
	            ELSE CASE WHEN u.avatar_mode = 'custom_url'
	                 THEN u.custom_avatar_url
	                 ELSE COALESCE(l.avatar_url, u.avatar_url) END
	       END, ''),
	       EXISTS (SELECT 1 FROM linux_do_bindings ldb WHERE ldb.user_id = s.owner_user_id),
		       COALESCE(s.declared_ttft_band, ''), COALESCE(s.declared_max_concurrency, 0), s.performance_confirmed_at,
		       s.prompt_audit_enabled,
	       b.sale_cutoff_at, b.expires_at,
	       COALESCE(current_round.id::text, ''), COALESCE(current_round.system_slot_key, ''), COALESCE(current_round.name, ''), current_round.starts_at, current_round.ends_at, COALESCE(current_round.status, ''), current_round.fulfillment_confirmed_at,
	       COALESCE(next_round.id::text, ''), COALESCE(next_round.system_slot_key, ''), COALESCE(next_round.name, ''), next_round.starts_at, next_round.ends_at, COALESCE(next_round.status, ''), next_round.fulfillment_confirmed_at,
	       stock.available_copies, credentials.available_copies
	FROM api_quota_offers o
	JOIN api_quota_batches b ON b.id = o.batch_id AND b.api_service_id = o.api_service_id AND b.owner_user_id = o.owner_user_id
	JOIN api_services s ON s.id = o.api_service_id AND s.owner_user_id = o.owner_user_id
	JOIN users u ON u.id = o.owner_user_id
	LEFT JOIN merchant_profiles mp ON mp.id = s.merchant_profile_id AND mp.owner_user_id = s.owner_user_id
	LEFT JOIN linux_do_bindings l ON l.user_id = s.owner_user_id
	LEFT JOIN LATERAL (
		SELECT r.id, r.system_slot_key, r.name, r.starts_at, r.ends_at, r.status, r.fulfillment_confirmed_at
		FROM api_quota_sale_rounds r
		WHERE r.batch_id = b.id AND r.status = 'scheduled'
		  AND r.starts_at <= $1 AND r.ends_at > $1
		  AND EXISTS (
		    SELECT 1 FROM api_quota_allocations current_allocation
		    WHERE current_allocation.sale_round_id = r.id
		      AND current_allocation.offer_id = o.id
		      AND current_allocation.status = 'active'
		  )
		ORDER BY r.starts_at DESC, r.id DESC
		LIMIT 1
	) current_round ON true
	LEFT JOIN LATERAL (
		SELECT r.id, r.system_slot_key, r.name, r.starts_at, r.ends_at, r.status, r.fulfillment_confirmed_at
		FROM api_quota_sale_rounds r
		WHERE r.batch_id = b.id AND r.status = 'scheduled' AND r.starts_at > $1
		  AND EXISTS (
		    SELECT 1 FROM api_quota_allocations next_allocation
		    WHERE next_allocation.sale_round_id = r.id
		      AND next_allocation.offer_id = o.id
		      AND next_allocation.status = 'active'
		  )
		ORDER BY r.starts_at, r.id
		LIMIT 1
	) next_round ON true
	LEFT JOIN LATERAL (
		SELECT count(*)::integer AS available_copies
		FROM api_quota_allocations a
		JOIN api_quota_inventory_units unit ON unit.allocation_id = a.id AND unit.status = 'available'
		WHERE a.offer_id = o.id AND a.status = 'active'
		  AND (
		    (o.sale_mode = 'continuous' AND a.sale_round_id IS NULL)
		    OR (o.sale_mode = 'scheduled' AND a.sale_round_id = current_round.id)
		  )
	) stock ON true
	LEFT JOIN LATERAL (
		SELECT count(*)::integer AS available_copies
		FROM api_quota_credentials credential
		WHERE credential.api_quota_offer_id = o.id AND credential.status = 'available'
	) credentials ON true
	WHERE b.status IN ('published', 'paused')
	  AND o.status IN ('published', 'paused')
	  AND s.review_status = 'approved'
	  AND s.publication_status = 'online'
	  AND s.moderation_status = 'clear'
`

func publicAPIQuotaOffersQueryWithSort(sortExpression string) string {
	if strings.TrimSpace(sortExpression) == "" {
		return publicAPIQuotaOffersQuery
	}
	return strings.Replace(publicAPIQuotaOffersQuery, "\n\tFROM api_quota_offers", ",\n\t       "+sortExpression+"\n\tFROM api_quota_offers", 1)
}

func getAPIQuotaBatch(ctx context.Context, q queryer, ownerUserID, batchID string, forUpdate bool) (apiquota.Batch, error) {
	query := `
		SELECT ` + apiQuotaBatchColumns + `
		FROM api_quota_batches b
		JOIN api_services s ON s.id = b.api_service_id AND s.owner_user_id = b.owner_user_id
		WHERE b.owner_user_id = $1 AND b.id = $2
	`
	if forUpdate {
		query += " FOR UPDATE OF b"
	}
	var batch apiquota.Batch
	err := scanAPIQuotaBatch(q.QueryRow(ctx, query, ownerUserID, batchID), &batch)
	return batch, err
}

func scanAPIQuotaBatch(row scanner, batch *apiquota.Batch) error {
	return row.Scan(
		&batch.ID, &batch.APIServiceID, &batch.OwnerUserID,
		&batch.ServiceTitle, &batch.DistributionSystem, &batch.ServiceOrderable,
		&batch.DeclaredTTFTBand, &batch.DeclaredMaxConcurrency, &batch.PerformanceConfirmedAt,
		&batch.PromptAuditEnabled,
		&batch.SourceType, &batch.SourceLabel, &batch.Status,
		&batch.DeclaredTotalUSDAllowance, &batch.UnallocatedUSDAllowance,
		&batch.SaleCutoffAt, &batch.ExpiresAt, &batch.SourceConfirmedAt, &batch.PublishedAt,
		&batch.CreatedAt, &batch.UpdatedAt, &batch.Version,
	)
}

func scanAPIQuotaOffer(row scanner, offer *apiquota.Offer) error {
	return row.Scan(
		&offer.ID, &offer.BatchID, &offer.APIServiceID, &offer.OwnerUserID,
		&offer.PreviousVersionID, &offer.DistributionSystem, &offer.Name,
		&offer.USDAllowance, &offer.PriceCNY, &offer.CNYPerUSD, &offer.ModelMultiplier,
		&offer.QuotaUsagePolicy.FiveHour.Mode, &offer.QuotaUsagePolicy.FiveHour.AmountUSD,
		&offer.QuotaUsagePolicy.Daily.Mode, &offer.QuotaUsagePolicy.Daily.AmountUSD,
		&offer.DeliveryMode, &offer.DeliveryETAMinutes, &offer.SaleMode, &offer.Status,
		&offer.SortOrder, &offer.PublishedAt, &offer.CreatedAt, &offer.UpdatedAt, &offer.Version,
	)
}

func scanAPIQuotaOffers(rows pgx.Rows) ([]apiquota.Offer, *domain.AppError) {
	items := []apiquota.Offer{}
	for rows.Next() {
		var offer apiquota.Offer
		if err := scanAPIQuotaOffer(rows, &offer); err != nil {
			return nil, internalStoreError()
		}
		items = append(items, offer)
	}
	if rows.Err() != nil {
		return nil, internalStoreError()
	}
	return items, nil
}

func scanAPIQuotaRound(row scanner, round *apiquota.SaleRound) error {
	return row.Scan(
		&round.ID, &round.BatchID, &round.APIServiceID, &round.OwnerUserID,
		&round.SystemSlotKey, &round.Name, &round.StartsAt, &round.EndsAt, &round.Status, &round.FulfillmentConfirmedAt,
		&round.CreatedAt, &round.UpdatedAt, &round.Version,
	)
}

func listAPIQuotaAllocations(ctx context.Context, q queryer, ownerUserID, batchID, roundID string) ([]apiquota.Allocation, *domain.AppError) {
	rows, err := queryRows(ctx, q, `
		SELECT a.id::text, a.batch_id::text, a.offer_id::text, a.api_service_id::text, a.owner_user_id::text,
		       COALESCE(a.sale_round_id::text, ''), a.sale_mode, a.copy_limit,
		       count(*) FILTER (WHERE unit.status = 'available')::integer,
		       count(*) FILTER (WHERE unit.status = 'reserved')::integer,
		       count(*) FILTER (WHERE unit.status = 'consumed')::integer,
		       a.allocated_usd_allowance::text, a.returned_usd_allowance::text,
		       a.status, a.created_at, a.updated_at
		FROM api_quota_allocations a
		LEFT JOIN api_quota_inventory_units unit ON unit.allocation_id = a.id
		WHERE a.owner_user_id = $1 AND a.batch_id = $2
		  AND ($3 = '' OR a.sale_round_id = $3::uuid)
		GROUP BY a.id
		ORDER BY a.created_at, a.id
	`, ownerUserID, batchID, roundID)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	result := []apiquota.Allocation{}
	for rows.Next() {
		var item apiquota.Allocation
		if err := rows.Scan(
			&item.ID, &item.BatchID, &item.OfferID, &item.APIServiceID, &item.OwnerUserID,
			&item.SaleRoundID, &item.SaleMode, &item.CopyLimit,
			&item.AvailableCopies, &item.ReservedCopies, &item.ConsumedCopies,
			&item.AllocatedUSDAllowance, &item.ReturnedUSDAllowance,
			&item.Status, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, internalStoreError()
		}
		result = append(result, item)
	}
	if rows.Err() != nil {
		return nil, internalStoreError()
	}
	return result, nil
}

func scanAPIQuotaOfferCard(row scanner) (apiquota.OfferCard, error) {
	var card apiquota.OfferCard
	if err := scanAPIQuotaOfferCardValues(row, &card, nil); err != nil {
		return apiquota.OfferCard{}, err
	}
	return card, nil
}

func scanAPIQuotaOfferCardWithSortValue(row scanner) (apiquota.OfferCard, error) {
	var card apiquota.OfferCard
	if err := scanAPIQuotaOfferCardValues(row, &card, &card.PublicSortValue); err != nil {
		return apiquota.OfferCard{}, err
	}
	return card, nil
}

func scanAPIQuotaOfferCardValues(row scanner, card *apiquota.OfferCard, sortValue *string) error {
	var currentID, currentSystemSlotKey, currentName, currentStatus string
	var currentStarts, currentEnds, currentFulfillmentConfirmedAt *time.Time
	var nextID, nextSystemSlotKey, nextName, nextStatus string
	var nextStarts, nextEnds, nextFulfillmentConfirmedAt *time.Time
	destinations := []any{
		&card.ID, &card.BatchID, &card.APIServiceID, &card.OwnerUserID,
		&card.PreviousVersionID, &card.DistributionSystem, &card.Name,
		&card.USDAllowance, &card.PriceCNY, &card.CNYPerUSD, &card.ModelMultiplier,
		&card.QuotaUsagePolicy.FiveHour.Mode, &card.QuotaUsagePolicy.FiveHour.AmountUSD,
		&card.QuotaUsagePolicy.Daily.Mode, &card.QuotaUsagePolicy.Daily.AmountUSD,
		&card.DeliveryMode, &card.DeliveryETAMinutes, &card.SaleMode, &card.Status,
		&card.SortOrder, &card.PublishedAt, &card.CreatedAt, &card.UpdatedAt, &card.Version,
		&card.BatchStatus, &card.ServiceTitle, &card.ServiceOrderable,
		&card.SellerDisplayName, &card.SellerIdentityType, &card.MerchantAvatarURL, &card.SellerLinuxDOBound,
		&card.DeclaredTTFTBand, &card.DeclaredMaxConcurrency, &card.PerformanceConfirmedAt,
		&card.PromptAuditEnabled,
		&card.SaleCutoffAt, &card.ExpiresAt,
		&currentID, &currentSystemSlotKey, &currentName, &currentStarts, &currentEnds, &currentStatus, &currentFulfillmentConfirmedAt,
		&nextID, &nextSystemSlotKey, &nextName, &nextStarts, &nextEnds, &nextStatus, &nextFulfillmentConfirmedAt,
		&card.AvailableCopies, &card.CredentialAvailableCopies,
	}
	if sortValue != nil {
		destinations = append(destinations, sortValue)
	}
	err := row.Scan(destinations...)
	if err != nil {
		return err
	}
	if currentID != "" && currentStarts != nil && currentEnds != nil {
		card.CurrentRound = &apiquota.SaleRound{ID: currentID, BatchID: card.BatchID, APIServiceID: card.APIServiceID, OwnerUserID: card.OwnerUserID, SystemSlotKey: currentSystemSlotKey, Name: currentName, StartsAt: *currentStarts, EndsAt: *currentEnds, Status: currentStatus, FulfillmentConfirmedAt: currentFulfillmentConfirmedAt}
	}
	if nextID != "" && nextStarts != nil && nextEnds != nil {
		card.NextRound = &apiquota.SaleRound{ID: nextID, BatchID: card.BatchID, APIServiceID: card.APIServiceID, OwnerUserID: card.OwnerUserID, SystemSlotKey: nextSystemSlotKey, Name: nextName, StartsAt: *nextStarts, EndsAt: *nextEnds, Status: nextStatus, FulfillmentConfirmedAt: nextFulfillmentConfirmedAt}
	}
	return nil
}

func allocationAmount(allowance string, copies int) (string, bool) {
	value, ok := new(big.Rat).SetString(strings.TrimSpace(allowance))
	if !ok || value.Sign() <= 0 || copies < 1 {
		return "", false
	}
	return new(big.Rat).Mul(value, new(big.Rat).SetInt64(int64(copies))).FloatString(6), true
}

func nullTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

func quotaNotFound(detail string) *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Quota resource not found", detail)
}

func invalidQuotaState(detail string) *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", detail)
}

func quotaVersionConflict() *domain.AppError {
	return domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
}

func mapAPIQuotaWriteError(err error) *domain.AppError {
	if isUniqueViolation(err) {
		return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Quota resource conflict", "额度包配置与现有记录冲突。")
	}
	return internalStoreError()
}
