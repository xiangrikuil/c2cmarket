package apiquota

import (
	"context"
	"math/big"
	"net/http"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/contact"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/reputation"

	"github.com/google/uuid"
)

const performanceDisclaimer = "商户自报，平台未测速"

type Manager struct {
	repo          Repository
	idempotency   *idempotency.Service
	now           func() time.Time
	actionChecker interface {
		CheckActionAllowed(context.Context, string, string, string) *domain.AppError
	}
}

func (m *Manager) SetActionChecker(checker interface {
	CheckActionAllowed(context.Context, string, string, string) *domain.AppError
}) {
	m.actionChecker = checker
}

func NewManager(repo Repository, now func() time.Time) *Manager {
	if now == nil {
		now = time.Now
	}
	var idempotencyRepo idempotency.Repository
	if candidate, ok := repo.(idempotency.Repository); ok {
		idempotencyRepo = candidate
	}
	return &Manager{repo: repo, idempotency: idempotency.NewService(idempotencyRepo, now), now: now}
}

func (m *Manager) beginQuotaIdempotency(ctx context.Context, userID, routeKey, key, requestHash string) (*idempotency.Entry, idempotency.Completion, *domain.AppError) {
	entry, appErr := m.idempotency.Begin(ctx, userID, routeKey, key, requestHash)
	if appErr != nil {
		return nil, idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return nil, idempotency.CompletionFromEntry(entry), nil
	}
	return entry, idempotency.Completion{}, nil
}

func (m *Manager) CreateOrderWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input CreateOrderInput, buildCompletion apiorder.CompletionBuilder) (idempotency.Completion, *domain.AppError) {
	input.BuyerUserID = strings.TrimSpace(userID)
	if appErr := validateCreateOrderInput(input); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if buildCompletion == nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	entry, appErr := m.idempotency.Begin(ctx, userID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		if entry.ResourceType == "api_order" && entry.ResourceID != "" {
			order, replayErr := m.repo.GetAPIQuotaOrderForBuyer(ctx, userID, entry.ResourceID, m.now().UTC())
			if replayErr != nil {
				return idempotency.Completion{}, replayErr
			}
			return buildCompletion(order)
		}
		return idempotency.CompletionFromEntry(entry), nil
	}
	if m.actionChecker != nil {
		offer, appErr := m.repo.GetPublicAPIQuotaOffer(ctx, input.OfferID, m.now().UTC())
		if appErr != nil {
			m.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		if appErr := m.actionChecker.CheckActionAllowed(ctx, offer.OwnerUserID, reputation.RoleSeller, reputation.ActionAPIServicePublish); appErr != nil {
			m.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
	}
	_, completion, appErr := m.repo.CreateAPIQuotaOrderWithIdempotency(ctx, *entry, input, m.now().UTC(), buildCompletion)
	if appErr != nil {
		m.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	return completion, nil
}

func (m *Manager) ImportCredentials(ctx context.Context, user auth.User, input CredentialImportInput) (CredentialImportResult, *domain.AppError) {
	if _, err := uuid.Parse(strings.TrimSpace(input.OfferID)); err != nil {
		return CredentialImportResult{}, fieldError("offerId", "必须选择有效的额度包。")
	}
	rows, appErr := ParseCredentialCSV(input.CSV, strings.TrimSpace(input.DeliveryKind))
	if appErr != nil {
		return CredentialImportResult{}, appErr
	}
	summary, appErr := m.repo.ImportAPIQuotaCredentials(ctx, user.ID, strings.TrimSpace(input.OfferID), input.RequestID, rows, m.now().UTC())
	if appErr != nil {
		return CredentialImportResult{}, appErr
	}
	return CredentialImportResult{Imported: len(rows), Summary: summary}, nil
}

func (m *Manager) ImportCredentialsWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input CredentialImportInput, buildCompletion CredentialImportCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if buildCompletion == nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	if _, err := uuid.Parse(strings.TrimSpace(input.OfferID)); err != nil {
		return idempotency.Completion{}, fieldError("offerId", "必须选择有效的额度包。")
	}
	rows, appErr := ParseCredentialCSV(input.CSV, strings.TrimSpace(input.DeliveryKind))
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	entry, replay, appErr := m.beginQuotaIdempotency(ctx, user.ID, routeKey, key, requestHash)
	if appErr != nil || entry == nil {
		return replay, appErr
	}
	_, completion, appErr := m.repo.ImportAPIQuotaCredentialsWithIdempotency(ctx, *entry, user.ID, strings.TrimSpace(input.OfferID), input.RequestID, rows, m.now().UTC(), buildCompletion)
	if appErr != nil {
		m.idempotency.Cancel(ctx, entry)
	}
	return completion, appErr
}

func (m *Manager) CredentialSummary(ctx context.Context, user auth.User, offerID string) (CredentialSummary, *domain.AppError) {
	if _, err := uuid.Parse(strings.TrimSpace(offerID)); err != nil {
		return CredentialSummary{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Quota offer not found", "额度包不存在。")
	}
	return m.repo.GetAPIQuotaCredentialSummary(ctx, user.ID, strings.TrimSpace(offerID))
}

func (m *Manager) CreateBatch(ctx context.Context, user auth.User, input CreateBatchInput) (Batch, *domain.AppError) {
	input.OwnerUserID = user.ID
	if appErr := validateCreateBatchInput(input, m.now()); appErr != nil {
		return Batch{}, appErr
	}
	now := m.now().UTC()
	batch := Batch{
		ID:                        uuid.NewString(),
		APIServiceID:              strings.TrimSpace(input.APIServiceID),
		OwnerUserID:               user.ID,
		SourceType:                strings.TrimSpace(input.SourceType),
		SourceLabel:               strings.TrimSpace(input.SourceLabel),
		Status:                    BatchStatusDraft,
		DeclaredTotalUSDAllowance: decimalStringMust(input.DeclaredTotalUSDAllowance, 6),
		UnallocatedUSDAllowance:   decimalStringMust(input.DeclaredTotalUSDAllowance, 6),
		SaleCutoffAt:              input.SaleCutoffAt.UTC(),
		ExpiresAt:                 input.ExpiresAt.UTC(),
		SourceConfirmedAt:         input.SourceConfirmedAt.UTC(),
		CreatedAt:                 now,
		UpdatedAt:                 now,
		Version:                   1,
	}
	return m.repo.CreateAPIQuotaBatch(ctx, batch, input.RequestID)
}

func (m *Manager) CreateBatchWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input CreateBatchInput, buildCompletion BatchCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if buildCompletion == nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	input.OwnerUserID = user.ID
	entry, replay, appErr := m.beginQuotaIdempotency(ctx, user.ID, routeKey, key, requestHash)
	if appErr != nil || entry == nil {
		return replay, appErr
	}
	if appErr := validateCreateBatchInput(input, m.now()); appErr != nil {
		m.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	now := m.now().UTC()
	batch := Batch{
		ID: uuid.NewString(), APIServiceID: strings.TrimSpace(input.APIServiceID), OwnerUserID: user.ID,
		SourceType: strings.TrimSpace(input.SourceType), SourceLabel: strings.TrimSpace(input.SourceLabel), Status: BatchStatusDraft,
		DeclaredTotalUSDAllowance: decimalStringMust(input.DeclaredTotalUSDAllowance, 6), UnallocatedUSDAllowance: decimalStringMust(input.DeclaredTotalUSDAllowance, 6),
		SaleCutoffAt: input.SaleCutoffAt.UTC(), ExpiresAt: input.ExpiresAt.UTC(), SourceConfirmedAt: input.SourceConfirmedAt.UTC(),
		CreatedAt: now, UpdatedAt: now, Version: 1,
	}
	_, completion, appErr := m.repo.CreateAPIQuotaBatchWithIdempotency(ctx, *entry, batch, input.RequestID, now, buildCompletion)
	if appErr != nil {
		m.idempotency.Cancel(ctx, entry)
	}
	return completion, appErr
}

func (m *Manager) OwnerBatches(ctx context.Context, user auth.User, apiServiceID string, page domain.PageRequest) (domain.Page[Batch], *domain.AppError) {
	apiServiceID = strings.TrimSpace(apiServiceID)
	if _, err := uuid.Parse(apiServiceID); err != nil {
		return domain.Page[Batch]{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "API service not found", "API 服务不存在。")
	}
	return m.repo.ListAPIQuotaBatchesForOwner(ctx, user.ID, apiServiceID, page)
}

func (m *Manager) CreateOffer(ctx context.Context, user auth.User, input CreateOfferInput) (Offer, *domain.AppError) {
	input.OwnerUserID = user.ID
	batch, appErr := m.repo.GetAPIQuotaBatchForOwner(ctx, user.ID, strings.TrimSpace(input.BatchID))
	if appErr != nil {
		return Offer{}, appErr
	}
	if appErr := validateCreateOfferInput(input, batch); appErr != nil {
		return Offer{}, appErr
	}
	now := m.now().UTC()
	offer := Offer{
		ID:                 uuid.NewString(),
		BatchID:            batch.ID,
		APIServiceID:       batch.APIServiceID,
		OwnerUserID:        user.ID,
		DistributionSystem: batch.DistributionSystem,
		Name:               strings.TrimSpace(input.Name),
		USDAllowance:       decimalStringMust(input.USDAllowance, 6),
		PriceCNY:           decimalStringMust(input.PriceCNY, 2),
		CNYPerUSD:          divideDecimal(input.PriceCNY, input.USDAllowance, 6),
		ModelMultiplier:    decimalStringMust(input.ModelMultiplier, 4),
		QuotaUsagePolicy:   apimarket.NormalizeQuotaUsagePolicy(input.QuotaUsagePolicy),
		DeliveryMode:       strings.TrimSpace(input.DeliveryMode),
		DeliveryETAMinutes: input.DeliveryETAMinutes,
		SaleMode:           strings.TrimSpace(input.SaleMode),
		Status:             OfferStatusDraft,
		SortOrder:          input.SortOrder,
		CreatedAt:          now,
		UpdatedAt:          now,
		Version:            1,
	}
	return m.repo.CreateAPIQuotaOffer(ctx, offer, input.ContinuousCopies, input.RequestID, now)
}

func (m *Manager) CreateOfferWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input CreateOfferInput, buildCompletion OfferCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if buildCompletion == nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	input.OwnerUserID = user.ID
	entry, replay, appErr := m.beginQuotaIdempotency(ctx, user.ID, routeKey, key, requestHash)
	if appErr != nil || entry == nil {
		return replay, appErr
	}
	batch, appErr := m.repo.GetAPIQuotaBatchForOwner(ctx, user.ID, strings.TrimSpace(input.BatchID))
	if appErr != nil {
		m.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	if appErr := validateCreateOfferInput(input, batch); appErr != nil {
		m.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	now := m.now().UTC()
	offer := Offer{
		ID: uuid.NewString(), BatchID: batch.ID, APIServiceID: batch.APIServiceID, OwnerUserID: user.ID, DistributionSystem: batch.DistributionSystem,
		Name: strings.TrimSpace(input.Name), USDAllowance: decimalStringMust(input.USDAllowance, 6), PriceCNY: decimalStringMust(input.PriceCNY, 2),
		CNYPerUSD: divideDecimal(input.PriceCNY, input.USDAllowance, 6), ModelMultiplier: decimalStringMust(input.ModelMultiplier, 4),
		QuotaUsagePolicy: apimarket.NormalizeQuotaUsagePolicy(input.QuotaUsagePolicy), DeliveryMode: strings.TrimSpace(input.DeliveryMode), DeliveryETAMinutes: input.DeliveryETAMinutes,
		SaleMode: strings.TrimSpace(input.SaleMode), Status: OfferStatusDraft, SortOrder: input.SortOrder, CreatedAt: now, UpdatedAt: now, Version: 1,
	}
	_, completion, appErr := m.repo.CreateAPIQuotaOfferWithIdempotency(ctx, *entry, offer, input.ContinuousCopies, input.RequestID, now, buildCompletion)
	if appErr != nil {
		m.idempotency.Cancel(ctx, entry)
	}
	return completion, appErr
}

func (m *Manager) OwnerOffers(ctx context.Context, user auth.User, batchID string) ([]Offer, *domain.AppError) {
	if _, err := uuid.Parse(strings.TrimSpace(batchID)); err != nil {
		return nil, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Quota batch not found", "额度批次不存在。")
	}
	return m.repo.ListAPIQuotaOffersForBatch(ctx, user.ID, strings.TrimSpace(batchID))
}

func (m *Manager) CreateRound(ctx context.Context, user auth.User, input CreateRoundInput) (SaleRound, *domain.AppError) {
	input.OwnerUserID = user.ID
	batch, appErr := m.repo.GetAPIQuotaBatchForOwner(ctx, user.ID, strings.TrimSpace(input.BatchID))
	if appErr != nil {
		return SaleRound{}, appErr
	}
	offers, appErr := m.repo.ListAPIQuotaOffersForBatch(ctx, user.ID, batch.ID)
	if appErr != nil {
		return SaleRound{}, appErr
	}
	if appErr := validateCreateRoundInput(input, batch, offers, m.now()); appErr != nil {
		return SaleRound{}, appErr
	}
	now := m.now().UTC()
	round := SaleRound{
		ID:           uuid.NewString(),
		BatchID:      batch.ID,
		APIServiceID: batch.APIServiceID,
		OwnerUserID:  user.ID,
		Name:         strings.TrimSpace(input.Name),
		StartsAt:     input.StartsAt.UTC(),
		EndsAt:       input.EndsAt.UTC(),
		Status:       RoundStatusScheduled,
		CreatedAt:    now,
		UpdatedAt:    now,
		Version:      1,
	}
	return m.repo.CreateAPIQuotaSaleRound(ctx, round, input.Offers, input.RequestID, now)
}

func (m *Manager) CreateRoundWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input CreateRoundInput, buildCompletion SaleRoundCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if buildCompletion == nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	input.OwnerUserID = user.ID
	entry, replay, appErr := m.beginQuotaIdempotency(ctx, user.ID, routeKey, key, requestHash)
	if appErr != nil || entry == nil {
		return replay, appErr
	}
	batch, appErr := m.repo.GetAPIQuotaBatchForOwner(ctx, user.ID, strings.TrimSpace(input.BatchID))
	if appErr != nil {
		m.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	offers, appErr := m.repo.ListAPIQuotaOffersForBatch(ctx, user.ID, batch.ID)
	if appErr != nil {
		m.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	if appErr := validateCreateRoundInput(input, batch, offers, m.now()); appErr != nil {
		m.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	now := m.now().UTC()
	round := SaleRound{
		ID: uuid.NewString(), BatchID: batch.ID, APIServiceID: batch.APIServiceID, OwnerUserID: user.ID,
		Name: strings.TrimSpace(input.Name), StartsAt: input.StartsAt.UTC(), EndsAt: input.EndsAt.UTC(), Status: RoundStatusScheduled,
		CreatedAt: now, UpdatedAt: now, Version: 1,
	}
	_, completion, appErr := m.repo.CreateAPIQuotaSaleRoundWithIdempotency(ctx, *entry, round, input.Offers, input.RequestID, now, buildCompletion)
	if appErr != nil {
		m.idempotency.Cancel(ctx, entry)
	}
	return completion, appErr
}

func (m *Manager) OwnerRounds(ctx context.Context, user auth.User, batchID string) ([]SaleRound, *domain.AppError) {
	if _, err := uuid.Parse(strings.TrimSpace(batchID)); err != nil {
		return nil, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Quota batch not found", "额度批次不存在。")
	}
	return m.repo.ListAPIQuotaSaleRoundsForBatch(ctx, user.ID, strings.TrimSpace(batchID))
}

func (m *Manager) ConfirmRoundFulfillmentWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input SaleRoundActionInput, buildCompletion SaleRoundCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if buildCompletion == nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	if _, err := uuid.Parse(strings.TrimSpace(input.SaleRoundID)); err != nil {
		return idempotency.Completion{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Quota sale round not found", "放量轮次不存在。")
	}
	input.OwnerUserID = user.ID
	entry, replay, appErr := m.beginQuotaIdempotency(ctx, user.ID, routeKey, key, requestHash)
	if appErr != nil || entry == nil {
		return replay, appErr
	}
	_, completion, appErr := m.repo.ConfirmAPIQuotaSaleRoundFulfillmentWithIdempotency(ctx, *entry, input, m.now().UTC(), buildCompletion)
	if appErr != nil {
		m.idempotency.Cancel(ctx, entry)
	}
	return completion, appErr
}

func (m *Manager) PublishBatch(ctx context.Context, user auth.User, input BatchActionInput) (Batch, *domain.AppError) {
	input.OwnerUserID = user.ID
	if appErr := m.checkSellerPublishAllowed(ctx, user.ID); appErr != nil {
		return Batch{}, appErr
	}
	batch, appErr := m.repo.GetAPIQuotaBatchForOwner(ctx, user.ID, strings.TrimSpace(input.BatchID))
	if appErr != nil {
		return Batch{}, appErr
	}
	if appErr := validatePublishableBatch(batch, m.now()); appErr != nil {
		return Batch{}, appErr
	}
	return m.repo.PublishAPIQuotaBatch(ctx, input, m.now().UTC())
}

func (m *Manager) PublishBatchWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input BatchActionInput, buildCompletion BatchCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if buildCompletion == nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	input.OwnerUserID = user.ID
	entry, replay, appErr := m.beginQuotaIdempotency(ctx, user.ID, routeKey, key, requestHash)
	if appErr != nil || entry == nil {
		return replay, appErr
	}
	if appErr := m.checkSellerPublishAllowed(ctx, user.ID); appErr != nil {
		m.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	batch, appErr := m.repo.GetAPIQuotaBatchForOwner(ctx, user.ID, strings.TrimSpace(input.BatchID))
	if appErr != nil {
		m.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	if appErr := validatePublishableBatch(batch, m.now()); appErr != nil {
		m.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	_, completion, appErr := m.repo.PublishAPIQuotaBatchWithIdempotency(ctx, *entry, input, m.now().UTC(), buildCompletion)
	if appErr != nil {
		m.idempotency.Cancel(ctx, entry)
	}
	return completion, appErr
}

func (m *Manager) UpdateBatchStatus(ctx context.Context, user auth.User, input BatchActionInput, action string) (Batch, *domain.AppError) {
	input.OwnerUserID = user.ID
	action = strings.TrimSpace(action)
	if action != "pause" && action != "resume" && action != "archive" {
		return Batch{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid action", "额度批次操作无效。", "action", "invalid", "额度批次操作无效。")
	}
	if action == "resume" {
		if appErr := m.checkSellerPublishAllowed(ctx, user.ID); appErr != nil {
			return Batch{}, appErr
		}
	}
	if action == "pause" || action == "archive" {
		rounds, appErr := m.repo.ListAPIQuotaSaleRoundsForBatch(ctx, user.ID, strings.TrimSpace(input.BatchID))
		if appErr != nil {
			return Batch{}, appErr
		}
		now := m.now().UTC()
		for _, round := range rounds {
			if strings.TrimSpace(round.SystemSlotKey) != "" && !now.Before(round.StartsAt.Add(-systemSlotRegistration)) {
				return Batch{}, domain.NewError(
					http.StatusConflict,
					domain.CodeInvalidStateTransition,
					"System sale slot locked",
					"固定场次已进入开抢前 1 小时锁定期，卖家不能暂停或归档。",
				)
			}
		}
	}
	return m.repo.UpdateAPIQuotaBatchStatus(ctx, input, action, m.now().UTC())
}

func (m *Manager) UpdateBatchStatusWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input BatchActionInput, action string, buildCompletion BatchCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if buildCompletion == nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	input.OwnerUserID = user.ID
	action = strings.TrimSpace(action)
	if action != "pause" && action != "resume" && action != "archive" {
		return idempotency.Completion{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid action", "额度批次操作无效。", "action", "invalid", "额度批次操作无效。")
	}
	entry, replay, appErr := m.beginQuotaIdempotency(ctx, user.ID, routeKey, key, requestHash)
	if appErr != nil || entry == nil {
		return replay, appErr
	}
	if action == "resume" {
		if appErr := m.checkSellerPublishAllowed(ctx, user.ID); appErr != nil {
			m.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
	}
	if action == "pause" || action == "archive" {
		rounds, appErr := m.repo.ListAPIQuotaSaleRoundsForBatch(ctx, user.ID, strings.TrimSpace(input.BatchID))
		if appErr != nil {
			m.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		now := m.now().UTC()
		for _, round := range rounds {
			if strings.TrimSpace(round.SystemSlotKey) != "" && !now.Before(round.StartsAt.Add(-systemSlotRegistration)) {
				m.idempotency.Cancel(ctx, entry)
				return idempotency.Completion{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "System sale slot locked", "固定场次已进入开抢前 1 小时锁定期，卖家不能暂停或归档。")
			}
		}
	}
	_, completion, appErr := m.repo.UpdateAPIQuotaBatchStatusWithIdempotency(ctx, *entry, input, action, m.now().UTC(), buildCompletion)
	if appErr != nil {
		m.idempotency.Cancel(ctx, entry)
	}
	return completion, appErr
}

func (m *Manager) PublicOffers(ctx context.Context, filter PublicOfferFilter, page domain.PageRequest) (domain.Page[OfferCard], *domain.AppError) {
	if filter.DistributionSystem != "" && filter.DistributionSystem != "sub2api" && filter.DistributionSystem != "new_api_proxy" && filter.DistributionSystem != "other" {
		return domain.Page[OfferCard]{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid distribution system", "接入系统筛选无效。", "distributionSystem", "invalid", "接入系统筛选无效。")
	}
	filter.Search = strings.TrimSpace(filter.Search)
	if len([]rune(filter.Search)) > 100 {
		return domain.Page[OfferCard]{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Search query too long", "搜索关键词不能超过 100 个字符。", "search", "max_length", "搜索关键词不能超过 100 个字符。")
	}
	filter.MaxMultiplier = strings.TrimSpace(filter.MaxMultiplier)
	if filter.MaxMultiplier != "" {
		if _, ok := positiveDecimal(filter.MaxMultiplier); !ok {
			return domain.Page[OfferCard]{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Multiplier filter invalid", "倍率上限必须是大于 0 的数字。", "maxMultiplier", "invalid", "请输入大于 0 的数字。")
		}
	}
	filter.SaleMode = strings.TrimSpace(filter.SaleMode)
	if filter.SaleMode != "" && filter.SaleMode != SaleModeContinuous && filter.SaleMode != SaleModeScheduled {
		return domain.Page[OfferCard]{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Sale mode filter invalid", "销售方式筛选无效。", "saleMode", "invalid", "销售方式筛选无效。")
	}
	if sortMode := strings.TrimSpace(filter.Sort); sortMode != "" && sortMode != PublicOfferSortUpdatedDesc &&
		sortMode != PublicOfferSortRecommended && sortMode != PublicOfferSortReputationDesc &&
		sortMode != PublicOfferSortCompletedDesc && sortMode != PublicOfferSortResponseFast &&
		sortMode != PublicOfferSortUnitPriceAsc && sortMode != PublicOfferSortAllowanceDesc && sortMode != PublicOfferSortDeliveryAsc {
		return domain.Page[OfferCard]{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Sort invalid", "排序方式无效。", "sort", "invalid", "排序方式无效。")
	}
	if strings.TrimSpace(filter.SystemSlotKey) != "" {
		slot, appErr := ResolveSystemSaleSlot(filter.SystemSlotKey, m.now())
		if appErr != nil {
			return domain.Page[OfferCard]{}, appErr
		}
		filter.SystemSlotKey = slot.Key
	}
	result, appErr := m.repo.ListPublicAPIQuotaOffers(ctx, filter, page, m.now().UTC())
	if appErr != nil {
		return domain.Page[OfferCard]{}, appErr
	}
	for index := range result.Items {
		result.Items[index] = WithOrderability(result.Items[index], m.now())
	}
	return result, nil
}

func (m *Manager) PublicOrderableOfferCount(ctx context.Context) (int, *domain.AppError) {
	if m.repo == nil {
		return 0, nil
	}
	repo, ok := m.repo.(PublicOfferInventoryRepository)
	if !ok {
		return 0, domain.NewError(http.StatusServiceUnavailable, domain.CodeInternalError, "API quota inventory count unavailable", "限量额度包数量暂时不可用。")
	}
	return repo.CountPublicOrderableAPIQuotaOffers(ctx, m.now().UTC())
}

func (filter PublicOfferFilter) NormalizedSort() string {
	switch strings.TrimSpace(filter.Sort) {
	case PublicOfferSortRecommended, PublicOfferSortReputationDesc, PublicOfferSortCompletedDesc,
		PublicOfferSortResponseFast, PublicOfferSortUnitPriceAsc, PublicOfferSortAllowanceDesc, PublicOfferSortDeliveryAsc:
		return strings.TrimSpace(filter.Sort)
	default:
		return PublicOfferSortUpdatedDesc
	}
}

func (m *Manager) SystemSaleSlots() []SystemSaleSlot {
	return SystemSaleSlots(m.now())
}

func (m *Manager) CreateRushOfferWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input CreateRushOfferInput, buildCompletion RushOfferCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if m.repo == nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "额度包存储不可用。")
	}
	if buildCompletion == nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	entry, replay, appErr := m.beginQuotaIdempotency(ctx, user.ID, routeKey, key, requestHash)
	if appErr != nil || entry == nil {
		return replay, appErr
	}
	now := m.now()
	slot, appErr := ResolveOpenSystemSaleSlot(input.SlotKey, now)
	if appErr != nil {
		m.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	totalAllowance, ok := allocationAmount(input.USDAllowance, input.Copies)
	if !ok || input.Copies > maxRushOfferCopies {
		m.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, fieldError("copies", "可售份数必须在 1 到 10 之间。")
	}
	batchInput := CreateBatchInput{
		APIServiceID: input.APIServiceID, SourceType: input.SourceType, SourceLabel: input.SourceLabel,
		DeclaredTotalUSDAllowance: totalAllowance, SaleCutoffAt: slot.EndsAt,
		ExpiresAt: input.ExpiresAt, SourceConfirmedAt: input.SourceConfirmedAt,
	}
	if appErr := validateCreateBatchInput(batchInput, now); appErr != nil {
		m.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	if input.ExpiresAt.Before(slot.EndsAt.Add(time.Hour)) {
		m.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, fieldError("expiresAt", "额度失效时间必须至少晚于场次结束 1 小时。")
	}
	offerInput := CreateOfferInput{
		Name: input.Name, USDAllowance: input.USDAllowance, PriceCNY: input.PriceCNY,
		ModelMultiplier: input.ModelMultiplier, QuotaUsagePolicy: input.QuotaUsagePolicy, DeliveryMode: input.DeliveryMode,
		DeliveryETAMinutes: input.DeliveryETAMinutes, SaleMode: SaleModeScheduled,
	}
	if appErr := validateCreateOfferInput(offerInput, Batch{Status: BatchStatusDraft}); appErr != nil {
		m.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	if appErr := validateRushOfferCredentials(input); appErr != nil {
		m.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}

	nowUTC := now.UTC()
	batchID := uuid.NewString()
	offerID := uuid.NewString()
	roundID := uuid.NewString()
	allocationID := uuid.NewString()
	publication := RushOfferPublication{
		RequestID: input.RequestID,
		Batch: Batch{
			ID: batchID, APIServiceID: strings.TrimSpace(input.APIServiceID), OwnerUserID: user.ID,
			SourceType: strings.TrimSpace(input.SourceType), SourceLabel: strings.TrimSpace(input.SourceLabel),
			Status: BatchStatusDraft, DeclaredTotalUSDAllowance: totalAllowance,
			UnallocatedUSDAllowance: totalAllowance, SaleCutoffAt: slot.EndsAt,
			ExpiresAt: input.ExpiresAt.UTC(), SourceConfirmedAt: input.SourceConfirmedAt.UTC(),
			CreatedAt: nowUTC, UpdatedAt: nowUTC, Version: 1,
		},
		Offer: Offer{
			ID: offerID, BatchID: batchID, APIServiceID: strings.TrimSpace(input.APIServiceID), OwnerUserID: user.ID,
			Name: strings.TrimSpace(input.Name), USDAllowance: decimalStringMust(input.USDAllowance, 6),
			PriceCNY: decimalStringMust(input.PriceCNY, 2), CNYPerUSD: divideDecimal(input.PriceCNY, input.USDAllowance, 6),
			ModelMultiplier: decimalStringMust(input.ModelMultiplier, 4), DeliveryMode: input.DeliveryMode,
			QuotaUsagePolicy:   apimarket.NormalizeQuotaUsagePolicy(input.QuotaUsagePolicy),
			DeliveryETAMinutes: input.DeliveryETAMinutes, SaleMode: SaleModeScheduled,
			Status: OfferStatusDraft, CreatedAt: nowUTC, UpdatedAt: nowUTC, Version: 1,
		},
		Round: SaleRound{
			ID: roundID, BatchID: batchID, APIServiceID: strings.TrimSpace(input.APIServiceID), OwnerUserID: user.ID,
			SystemSlotKey: slot.Key, Name: slot.Key, StartsAt: slot.StartsAt, EndsAt: slot.EndsAt,
			Status: RoundStatusScheduled, CreatedAt: nowUTC, UpdatedAt: nowUTC, Version: 1,
			Allocations: []Allocation{{
				ID: allocationID, BatchID: batchID, OfferID: offerID, APIServiceID: strings.TrimSpace(input.APIServiceID),
				OwnerUserID: user.ID, SaleRoundID: roundID, SaleMode: SaleModeScheduled,
				CopyLimit: input.Copies, AllocatedUSDAllowance: totalAllowance, Status: "planned",
				CreatedAt: nowUTC, UpdatedAt: nowUTC,
			}},
		},
		CredentialImported: len(input.CredentialRows),
	}

	if appErr := m.checkSellerPublishAllowed(ctx, user.ID); appErr != nil {
		m.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	_, completion, appErr := m.repo.CreateSystemRushOfferWithIdempotency(ctx, *entry, publication, input.CredentialRows, nowUTC, buildCompletion)
	if appErr != nil {
		m.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	return completion, nil
}

func (m *Manager) checkSellerPublishAllowed(ctx context.Context, userID string) *domain.AppError {
	if m.actionChecker == nil {
		return nil
	}
	return m.actionChecker.CheckActionAllowed(ctx, userID, reputation.RoleSeller, reputation.ActionAPIServicePublish)
}

func (m *Manager) PublicOffer(ctx context.Context, offerID string) (OfferCard, *domain.AppError) {
	if _, err := uuid.Parse(strings.TrimSpace(offerID)); err != nil {
		return OfferCard{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Quota offer not found", "额度包不存在。")
	}
	offer, appErr := m.repo.GetPublicAPIQuotaOffer(ctx, offerID, m.now().UTC())
	if appErr != nil {
		return OfferCard{}, appErr
	}
	return WithOrderability(offer, m.now()), nil
}

func WithOrderability(card OfferCard, now time.Time) OfferCard {
	now = now.UTC()
	card.PerformanceDisclaimer = performanceDisclaimer
	card.IsOrderable = false

	switch {
	case !card.ServiceOrderable:
		card.OrderabilityCode = OrderabilityServiceUnavailable
		card.OrderabilityReason = "关联 API 服务当前不可接单。"
	case card.BatchStatus != BatchStatusPublished:
		card.OrderabilityCode = OrderabilityBatchPaused
		card.OrderabilityReason = "额度批次当前未开放销售。"
	case card.Status != OfferStatusPublished:
		card.OrderabilityCode = OrderabilityOfferPaused
		card.OrderabilityReason = "额度包当前未开放销售。"
	case !now.Before(card.ExpiresAt) || !now.Before(card.SaleCutoffAt):
		card.OrderabilityCode = OrderabilityBatchExpired
		card.OrderabilityReason = "额度包已超过最晚下单时间。"
	case card.SaleMode == SaleModeScheduled && card.CurrentRound == nil && card.NextRound != nil:
		card.OrderabilityCode = OrderabilityNotStarted
		card.OrderabilityReason = "本轮尚未开始。"
	case card.SaleMode == SaleModeScheduled && card.CurrentRound == nil:
		card.OrderabilityCode = OrderabilityRoundEnded
		card.OrderabilityReason = "当前没有可购买的放量轮次。"
	case card.SaleMode == SaleModeScheduled && card.CurrentRound.SystemSlotKey != "" && card.CurrentRound.FulfillmentConfirmedAt == nil:
		card.OrderabilityCode = OrderabilityConfirmationMissing
		card.OrderabilityReason = "卖家尚未确认本场可按时履约。"
	case card.AvailableCopies < 1:
		card.OrderabilityCode = OrderabilitySoldOut
		card.OrderabilityReason = "当前库存已售罄。"
	case card.DeliveryMode == DeliveryModePreimported && card.CredentialAvailableCopies < card.AvailableCopies:
		card.OrderabilityCode = OrderabilityCredentialShortage
		card.OrderabilityReason = "可分配交付凭据不足。"
	default:
		card.IsOrderable = true
		card.OrderabilityCode = OrderabilityOrderable
		card.OrderabilityReason = "当前可购买。"
	}
	return card
}

func validateCreateBatchInput(input CreateBatchInput, now time.Time) *domain.AppError {
	if _, err := uuid.Parse(strings.TrimSpace(input.APIServiceID)); err != nil {
		return fieldError("apiServiceId", "必须选择有效的 API 服务。")
	}
	sourceType := strings.TrimSpace(input.SourceType)
	if sourceType != SourceTypeSub2API && sourceType != SourceTypeNewAPIProxy && sourceType != SourceTypeSelfHosted && sourceType != SourceTypeOther {
		return fieldError("sourceType", "额度来源类型无效。")
	}
	if sourceType == SourceTypeOther && strings.TrimSpace(input.SourceLabel) == "" {
		return fieldError("sourceLabel", "其他额度来源必须填写公开说明。")
	}
	if sourceType != SourceTypeOther && strings.TrimSpace(input.SourceLabel) != "" {
		return fieldError("sourceLabel", "仅其他额度来源可以填写自定义说明。")
	}
	if _, ok := positiveDecimal(input.DeclaredTotalUSDAllowance); !ok {
		return fieldError("declaredTotalUsdAllowance", "卖家声明美元额度必须大于 0。")
	}
	now = now.UTC()
	if !input.ExpiresAt.After(now) {
		return fieldError("expiresAt", "额度失效时间必须晚于当前时间。")
	}
	if input.SaleCutoffAt.After(input.ExpiresAt.Add(-time.Hour)) {
		return fieldError("saleCutoffAt", "最晚下单时间不能晚于额度失效前 1 小时。")
	}
	if !input.SaleCutoffAt.After(now) {
		return fieldError("saleCutoffAt", "最晚下单时间必须晚于当前时间。")
	}
	if input.SourceConfirmedAt.IsZero() || input.SourceConfirmedAt.After(now.Add(time.Minute)) {
		return fieldError("sourceConfirmedAt", "额度最近确认时间无效。")
	}
	return nil
}

func validateCreateOfferInput(input CreateOfferInput, batch Batch) *domain.AppError {
	if batch.Status != BatchStatusDraft {
		return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "只有草稿额度批次可以新增规格。")
	}
	if strings.TrimSpace(input.Name) == "" {
		return fieldError("name", "额度包名称不能为空。")
	}
	if _, ok := positiveDecimal(input.USDAllowance); !ok {
		return fieldError("usdAllowance", "美元额度必须大于 0。")
	}
	if _, ok := positiveDecimal(input.PriceCNY); !ok {
		return fieldError("priceCny", "人民币总价必须大于 0。")
	}
	if _, ok := positiveDecimal(input.ModelMultiplier); !ok {
		return fieldError("modelMultiplier", "模型倍率必须大于 0。")
	}
	if appErr := apimarket.ValidateQuotaUsagePolicy(input.QuotaUsagePolicy, "quotaUsagePolicy", false); appErr != nil {
		return appErr
	}
	if input.DeliveryMode == DeliveryModePreimported {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Preimported delivery retired for new offers", "新额度包只支持卖家手工交付。", "deliveryMode", "new_preimported_not_allowed", "请选择卖家手工交付。")
	}
	if input.DeliveryMode != DeliveryModeManual {
		return fieldError("deliveryMode", "交付模式无效。")
	}
	if input.DeliveryETAMinutes < 1 || input.DeliveryETAMinutes > 10 {
		return fieldError("deliveryEtaMinutes", "预计交付时间必须在 1 到 10 分钟之间。")
	}
	if input.SaleMode != SaleModeContinuous && input.SaleMode != SaleModeScheduled {
		return fieldError("saleMode", "销售模式无效。")
	}
	if input.SaleMode == SaleModeContinuous && input.ContinuousCopies < 1 {
		return fieldError("continuousCopies", "全天可买额度包必须配置至少 1 份库存。")
	}
	if input.SaleMode == SaleModeScheduled && input.ContinuousCopies != 0 {
		return fieldError("continuousCopies", "定时额度包的份数必须配置在放量轮次中。")
	}
	return nil
}

func validateCreateRoundInput(input CreateRoundInput, batch Batch, offers []Offer, now time.Time) *domain.AppError {
	if batch.Status != BatchStatusDraft {
		return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "只有草稿额度批次可以新增放量轮次。")
	}
	if strings.TrimSpace(input.Name) == "" {
		return fieldError("name", "放量轮次名称不能为空。")
	}
	if !input.StartsAt.After(now.UTC()) {
		return fieldError("startsAt", "放量开始时间必须晚于当前时间。")
	}
	if !input.EndsAt.After(input.StartsAt) {
		return fieldError("endsAt", "放量结束时间必须晚于开始时间。")
	}
	if input.EndsAt.After(batch.SaleCutoffAt) {
		return fieldError("endsAt", "放量结束时间不能晚于批次最晚下单时间。")
	}
	if len(input.Offers) == 0 {
		return fieldError("offers", "放量轮次至少需要一个额度规格。")
	}
	byID := make(map[string]Offer, len(offers))
	for _, offer := range offers {
		byID[offer.ID] = offer
	}
	seen := make(map[string]struct{}, len(input.Offers))
	for index, requested := range input.Offers {
		offer, ok := byID[strings.TrimSpace(requested.OfferID)]
		if !ok || offer.SaleMode != SaleModeScheduled {
			return fieldError("offers", "放量轮次包含无效的定时额度规格。")
		}
		if requested.Copies < 1 {
			return fieldError("offers", "每个额度规格的放量份数必须大于 0。")
		}
		if _, duplicate := seen[offer.ID]; duplicate {
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Duplicate offer", "同一轮次不能重复配置额度规格。", "offers", "duplicate", "第 "+integerText(index+1)+" 项额度规格重复。")
		}
		seen[offer.ID] = struct{}{}
	}
	return nil
}

func validatePublishableBatch(batch Batch, now time.Time) *domain.AppError {
	if batch.Status != BatchStatusDraft {
		return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前额度批次不能发布。")
	}
	if !batch.ServiceOrderable {
		return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "API service unavailable", "关联 API 服务当前不可接单。")
	}
	if batch.DeclaredMaxConcurrency < 1 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Maximum concurrency required", "发布额度包前必须填写商户声明最大并发。", "declaredMaxConcurrency", "required", "请输入大于 0 的最大并发。")
	}
	if batch.PromptAuditEnabled == nil {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Prompt audit selection required", "发布额度包前必须声明是否开启提示词审计。", "promptAuditEnabled", "required", "请选择是否开启提示词审计。")
	}
	if !now.UTC().Before(batch.SaleCutoffAt) || !now.UTC().Before(batch.ExpiresAt) {
		return domain.NewError(http.StatusConflict, domain.CodeAPIQuotaBatchExpired, "Quota batch expired", "额度批次已超过最晚下单时间。")
	}
	return nil
}

func validateCreateOrderInput(input CreateOrderInput) *domain.AppError {
	if _, err := uuid.Parse(strings.TrimSpace(input.OfferID)); err != nil {
		return fieldError("offerId", "必须选择有效的额度包。")
	}
	if strings.TrimSpace(input.SaleRoundID) != "" {
		if _, err := uuid.Parse(strings.TrimSpace(input.SaleRoundID)); err != nil {
			return fieldError("saleRoundId", "放量轮次无效。")
		}
	}
	buyerContactMethodID := strings.TrimSpace(input.BuyerContactMethodID)
	if buyerContactMethodID == "" {
		return contact.TransactionContactRequiredError("buyerContactMethodId", "请选择有效的买家交易联系方式。")
	}
	if _, err := uuid.Parse(buyerContactMethodID); err != nil {
		return fieldError("buyerContactMethodId", "交易联系方式无效，请刷新后重试。")
	}
	if strings.TrimSpace(input.SelectedAccessMode) == "" {
		return fieldError("selectedAccessMode", "必须选择接入方式。")
	}
	if strings.TrimSpace(input.PaymentMethod) != "wechat" && strings.TrimSpace(input.PaymentMethod) != "alipay" {
		return fieldError("paymentMethod", "付款方式无效。")
	}
	if appErr := validateOptionalNonSecretText("buyerNote", input.BuyerNote); appErr != nil {
		return appErr
	}
	return nil
}

func fieldError(field, message string) *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Validation failed", message, field, "invalid", message)
}

func validateOptionalNonSecretText(field, value string) *domain.AppError {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if len(value) > 4000 || strings.ContainsAny(value, "\x00") {
		return fieldError(field, "文本内容无效或过长。")
	}
	if domain.LooksLikeSecretContent(value) {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeSecretContentDetected, "Secret content detected", "不能在订单备注中填写、粘贴或上传任何凭据。", field, "secret_content", "不能包含 API Key、密码、Token、Session 或 Cookie。")
	}
	return nil
}

func positiveDecimal(value string) (*big.Rat, bool) {
	rat, ok := new(big.Rat).SetString(strings.TrimSpace(value))
	if !ok || rat.Sign() <= 0 {
		return nil, false
	}
	return rat, true
}

func decimalStringMust(value string, places int) string {
	rat, _ := positiveDecimal(value)
	return rat.FloatString(places)
}

func divideDecimal(numerator, denominator string, places int) string {
	left, _ := positiveDecimal(numerator)
	right, _ := positiveDecimal(denominator)
	return new(big.Rat).Quo(left, right).FloatString(places)
}

func integerText(value int) string {
	return new(big.Int).SetInt64(int64(value)).String()
}

const maxRushOfferCopies = 10

func allocationAmount(allowance string, copies int) (string, bool) {
	value, ok := positiveDecimal(allowance)
	if !ok || copies < 1 {
		return "", false
	}
	return new(big.Rat).Mul(value, new(big.Rat).SetInt64(int64(copies))).FloatString(6), true
}

func validateRushOfferCredentials(input CreateRushOfferInput) *domain.AppError {
	if input.DeliveryMode == DeliveryModeManual {
		if strings.TrimSpace(input.DeliveryKind) != "" || len(input.CredentialRows) != 0 {
			return fieldError("file", "卖家手工交付不需要上传凭据 CSV。")
		}
		return nil
	}
	if input.DeliveryKind != apiorder.DeliveryKindAPIKeyEndpoint && input.DeliveryKind != apiorder.DeliveryKindLoginAccount {
		return fieldError("deliveryKind", "预导入交付必须选择有效的凭据模板。")
	}
	if len(input.CredentialRows) < input.Copies {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Credential inventory insufficient", "预导入凭据数量必须覆盖全部可售份数。", "file", "insufficient", "CSV 中的有效凭据数量不能少于可售份数。")
	}
	for _, row := range input.CredentialRows {
		if row.DeliveryKind != input.DeliveryKind {
			return fieldError("file", "CSV 凭据类型与所选交付模板不一致。")
		}
	}
	return nil
}
