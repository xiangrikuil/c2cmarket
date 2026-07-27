package apiquota

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/idempotency"
)

type Repository interface {
	CreateAPIQuotaBatch(ctx context.Context, batch Batch) (Batch, *domain.AppError)
	GetAPIQuotaBatchForOwner(ctx context.Context, ownerUserID, batchID string) (Batch, *domain.AppError)
	ListAPIQuotaBatchesForOwner(ctx context.Context, ownerUserID, apiServiceID string, page domain.PageRequest) (domain.Page[Batch], *domain.AppError)
	CreateAPIQuotaOffer(ctx context.Context, offer Offer, continuousCopies int, now time.Time) (Offer, *domain.AppError)
	GetAPIQuotaOfferForOwner(ctx context.Context, ownerUserID, offerID string) (Offer, *domain.AppError)
	ListAPIQuotaOffersForBatch(ctx context.Context, ownerUserID, batchID string) ([]Offer, *domain.AppError)
	CreateAPIQuotaSaleRound(ctx context.Context, round SaleRound, requested []RoundOfferInput, now time.Time) (SaleRound, *domain.AppError)
	ListAPIQuotaSaleRoundsForBatch(ctx context.Context, ownerUserID, batchID string) ([]SaleRound, *domain.AppError)
	PublishAPIQuotaBatch(ctx context.Context, input BatchActionInput, now time.Time) (Batch, *domain.AppError)
	UpdateAPIQuotaBatchStatus(ctx context.Context, input BatchActionInput, action string, now time.Time) (Batch, *domain.AppError)
	ListPublicAPIQuotaOffers(ctx context.Context, filter PublicOfferFilter, page domain.PageRequest, now time.Time) (domain.Page[OfferCard], *domain.AppError)
	GetPublicAPIQuotaOffer(ctx context.Context, offerID string, now time.Time) (OfferCard, *domain.AppError)
	CreateAPIQuotaOrderWithIdempotency(ctx context.Context, entry idempotency.Entry, input CreateOrderInput, now time.Time, buildCompletion apiorder.CompletionBuilder) (apiorder.Order, idempotency.Completion, *domain.AppError)
	GetAPIQuotaOrderForBuyer(ctx context.Context, buyerUserID, orderID string, now time.Time) (apiorder.Order, *domain.AppError)
	ImportAPIQuotaCredentials(ctx context.Context, ownerUserID, offerID string, rows []CredentialImportRow, now time.Time) (CredentialSummary, *domain.AppError)
	GetAPIQuotaCredentialSummary(ctx context.Context, ownerUserID, offerID string) (CredentialSummary, *domain.AppError)
	CreateSystemRushOfferWithIdempotency(ctx context.Context, entry idempotency.Entry, publication RushOfferPublication, credentials []CredentialImportRow, now time.Time, buildCompletion RushOfferCompletionBuilder) (RushOfferPublication, idempotency.Completion, *domain.AppError)
}
