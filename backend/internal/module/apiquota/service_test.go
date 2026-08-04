package apiquota

import (
	"context"
	"net/http"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"
)

func TestCreateBatchEnforcesOneHourHardCutoff(t *testing.T) {
	now := time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC)
	repo := &fakeRepository{}
	manager := NewManager(repo, func() time.Time { return now })

	_, appErr := manager.CreateBatch(context.Background(), auth.User{ID: "seller-1"}, CreateBatchInput{
		APIServiceID:              "10000000-0000-0000-0000-000000000001",
		SourceType:                SourceTypeSub2API,
		DeclaredTotalUSDAllowance: "1000",
		SaleCutoffAt:              now.Add(4*time.Hour + time.Minute),
		ExpiresAt:                 now.Add(5 * time.Hour),
		SourceConfirmedAt:         now,
	})
	if appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("expected hard cutoff validation, got %v", appErr)
	}
	if repo.createdBatch != nil {
		t.Fatalf("invalid batch must not reach repository")
	}
}

func TestCreateOfferAllowsSub2APIDeclaredMultiplier(t *testing.T) {
	now := time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC)
	repo := &fakeRepository{batch: validBatch(now)}
	manager := NewManager(repo, func() time.Time { return now })

	offer, appErr := manager.CreateOffer(context.Background(), auth.User{ID: "seller-1"}, CreateOfferInput{
		BatchID:            repo.batch.ID,
		Name:               "$50 额度包",
		USDAllowance:       "50",
		PriceCNY:           "5",
		ModelMultiplier:    "1.2",
		QuotaUsagePolicy:   validQuotaUsagePolicy(),
		DeliveryMode:       DeliveryModeManual,
		DeliveryETAMinutes: 10,
		SaleMode:           SaleModeContinuous,
		ContinuousCopies:   10,
	})
	if appErr != nil {
		t.Fatalf("expected Sub2API declared multiplier to pass, got %v", appErr)
	}
	if repo.createdOffer == nil || offer.ModelMultiplier != "1.2000" {
		t.Fatalf("expected declared multiplier to reach repository, got %+v", offer)
	}
}

func TestCreateRoundSupportsOneThousandCopiesAndRejectsDuplicateOffer(t *testing.T) {
	now := time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC)
	batch := validBatch(now)
	offer := validOffer(batch, SaleModeScheduled)
	repo := &fakeRepository{batch: batch, offers: []Offer{offer}}
	manager := NewManager(repo, func() time.Time { return now })

	round, appErr := manager.CreateRound(context.Background(), auth.User{ID: "seller-1"}, CreateRoundInput{
		BatchID:  batch.ID,
		Name:     "10:00 放量",
		StartsAt: now.Add(time.Hour),
		EndsAt:   now.Add(90 * time.Minute),
		Offers:   []RoundOfferInput{{OfferID: offer.ID, Copies: 1000}},
	})
	if appErr != nil {
		t.Fatalf("expected 1000-copy round to pass: %v", appErr)
	}
	if round.ID == "" || len(repo.createdRoundRequest) != 1 || repo.createdRoundRequest[0].Copies != 1000 {
		t.Fatalf("round was not persisted with 1000 copies: %+v", round)
	}

	_, appErr = manager.CreateRound(context.Background(), auth.User{ID: "seller-1"}, CreateRoundInput{
		BatchID:  batch.ID,
		Name:     "11:00 放量",
		StartsAt: now.Add(2 * time.Hour),
		EndsAt:   now.Add(150 * time.Minute),
		Offers: []RoundOfferInput{
			{OfferID: offer.ID, Copies: 10},
			{OfferID: offer.ID, Copies: 5},
		},
	})
	if appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("expected duplicate offer validation, got %v", appErr)
	}
}

func TestOfferOrderabilityUsesServerTimeAndCredentialInventory(t *testing.T) {
	now := time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC)
	card := OfferCard{
		Offer: Offer{
			Status:       OfferStatusPublished,
			SaleMode:     SaleModeScheduled,
			DeliveryMode: DeliveryModePreimported,
		},
		BatchStatus:               BatchStatusPublished,
		ServiceOrderable:          true,
		SaleCutoffAt:              now.Add(4 * time.Hour),
		ExpiresAt:                 now.Add(5 * time.Hour),
		CurrentRound:              &SaleRound{StartsAt: now.Add(-time.Minute), EndsAt: now.Add(time.Hour)},
		AvailableCopies:           10,
		CredentialAvailableCopies: 9,
	}

	card = WithOrderability(card, now)
	if card.IsOrderable || card.OrderabilityCode != OrderabilityCredentialShortage {
		t.Fatalf("expected credential shortage, got %+v", card)
	}

	card.CredentialAvailableCopies = 10
	card = WithOrderability(card, now)
	if !card.IsOrderable || card.OrderabilityCode != OrderabilityOrderable {
		t.Fatalf("expected orderable card, got %+v", card)
	}

	card = WithOrderability(card, card.SaleCutoffAt)
	if card.IsOrderable || card.OrderabilityCode != OrderabilityBatchExpired {
		t.Fatalf("expected hard cutoff to block purchase, got %+v", card)
	}
}

func TestCreateRushOfferPublishesOneManualDeliverySlot(t *testing.T) {
	now := beijingTime(t, "2026-07-24T07:00:00+08:00")
	repo := &fakeRepository{}
	manager := NewManager(repo, func() time.Time { return now })

	completion, appErr := manager.CreateRushOfferWithIdempotency(
		context.Background(),
		auth.User{ID: "10000000-0000-0000-0000-000000000010"},
		"POST /api/v1/owner/api-services/{id}/quota-rush-offers",
		"rush-manual",
		"manual-request",
		validRushOfferInput(t, now),
		testRushOfferCompletion,
	)
	if appErr != nil {
		t.Fatalf("expected manual rush offer to publish, got %v", appErr)
	}
	if completion.Status != http.StatusCreated || repo.rushCreateCalls != 1 || repo.rushPublication == nil {
		t.Fatalf("unexpected rush publication result: completion=%+v calls=%d publication=%+v", completion, repo.rushCreateCalls, repo.rushPublication)
	}
	publication := repo.rushPublication
	if publication.Batch.Status != BatchStatusDraft || publication.Batch.DeclaredTotalUSDAllowance != "150.000000" {
		t.Fatalf("unexpected generated batch: %+v", publication.Batch)
	}
	if publication.Offer.SaleMode != SaleModeScheduled || publication.Offer.DeliveryMode != DeliveryModeManual {
		t.Fatalf("unexpected generated offer: %+v", publication.Offer)
	}
	if publication.Round.SystemSlotKey != "2026-07-24@09:00" || len(publication.Round.Allocations) != 1 {
		t.Fatalf("unexpected generated round: %+v", publication.Round)
	}
	if allocation := publication.Round.Allocations[0]; allocation.CopyLimit != 3 || allocation.AllocatedUSDAllowance != "150.000000" {
		t.Fatalf("unexpected generated allocation: %+v", allocation)
	}
	if len(repo.rushCredentials) != 0 {
		t.Fatalf("manual delivery must not persist credentials")
	}
}

func TestCreateRushOfferRejectsCredentialShortageBeforeRepository(t *testing.T) {
	now := beijingTime(t, "2026-07-24T07:00:00+08:00")
	repo := &fakeRepository{}
	manager := NewManager(repo, func() time.Time { return now })
	input := validRushOfferInput(t, now)
	input.DeliveryMode = DeliveryModePreimported
	input.DeliveryKind = apiorder.DeliveryKindAPIKeyEndpoint
	input.CredentialRows = []CredentialImportRow{
		{DeliveryKind: apiorder.DeliveryKindAPIKeyEndpoint},
		{DeliveryKind: apiorder.DeliveryKindAPIKeyEndpoint},
	}

	_, appErr := manager.CreateRushOfferWithIdempotency(
		context.Background(),
		auth.User{ID: "10000000-0000-0000-0000-000000000010"},
		"POST /api/v1/owner/api-services/{id}/quota-rush-offers",
		"rush-shortage",
		"shortage-request",
		input,
		testRushOfferCompletion,
	)
	if appErr == nil || appErr.Code != domain.CodeValidationFailed || repo.rushCreateCalls != 0 {
		t.Fatalf("expected credential shortage before repository, got err=%v calls=%d", appErr, repo.rushCreateCalls)
	}
}

func TestCreateRushOfferRejectsClosedAndArbitrarySlots(t *testing.T) {
	now := beijingTime(t, "2026-07-24T08:00:00+08:00")
	tests := []struct {
		name     string
		slotKey  string
		wantCode string
	}{
		{name: "registration closed", slotKey: "2026-07-24@09:00", wantCode: domain.CodeInvalidStateTransition},
		{name: "arbitrary minute", slotKey: "2026-07-24@10:15", wantCode: domain.CodeValidationFailed},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &fakeRepository{}
			manager := NewManager(repo, func() time.Time { return now })
			input := validRushOfferInput(t, now.Add(-time.Hour))
			input.SlotKey = test.slotKey

			_, appErr := manager.CreateRushOfferWithIdempotency(
				context.Background(),
				auth.User{ID: "10000000-0000-0000-0000-000000000010"},
				"POST /api/v1/owner/api-services/{id}/quota-rush-offers",
				"rush-invalid-slot",
				test.name,
				input,
				testRushOfferCompletion,
			)
			if appErr == nil || appErr.Code != test.wantCode || repo.rushCreateCalls != 0 {
				t.Fatalf("expected slot rejection code %s, got err=%v calls=%d", test.wantCode, appErr, repo.rushCreateCalls)
			}
		})
	}
}

func TestCreateRushOfferEnforcesExpiryOneHourAfterSlotEnd(t *testing.T) {
	now := beijingTime(t, "2026-07-24T07:00:00+08:00")
	input := validRushOfferInput(t, now)
	slot, appErr := ResolveOpenSystemSaleSlot(input.SlotKey, now)
	if appErr != nil {
		t.Fatalf("resolve test slot: %v", appErr)
	}
	input.ExpiresAt = slot.EndsAt.Add(time.Hour - time.Nanosecond)
	repo := &fakeRepository{}
	manager := NewManager(repo, func() time.Time { return now })

	_, appErr = manager.CreateRushOfferWithIdempotency(
		context.Background(),
		auth.User{ID: "10000000-0000-0000-0000-000000000010"},
		"POST /api/v1/owner/api-services/{id}/quota-rush-offers",
		"rush-expiry",
		"expiry-request",
		input,
		testRushOfferCompletion,
	)
	if appErr == nil || appErr.Code != domain.CodeValidationFailed || repo.rushCreateCalls != 0 {
		t.Fatalf("expected expiry lower-bound rejection, got err=%v calls=%d", appErr, repo.rushCreateCalls)
	}
}

func TestCreateRushOfferReplaysCompletedIdempotentResponse(t *testing.T) {
	now := beijingTime(t, "2026-07-24T07:00:00+08:00")
	repo := &fakeRepository{}
	manager := NewManager(repo, func() time.Time { return now })
	input := validRushOfferInput(t, now)
	user := auth.User{ID: "10000000-0000-0000-0000-000000000010"}
	const routeKey = "POST /api/v1/owner/api-services/{id}/quota-rush-offers"

	first, firstErr := manager.CreateRushOfferWithIdempotency(context.Background(), user, routeKey, "rush-replay", "same-request", input, testRushOfferCompletion)
	second, secondErr := manager.CreateRushOfferWithIdempotency(context.Background(), user, routeKey, "rush-replay", "same-request", input, testRushOfferCompletion)
	if firstErr != nil || secondErr != nil {
		t.Fatalf("expected successful idempotent replay, got first=%v second=%v", firstErr, secondErr)
	}
	if repo.rushCreateCalls != 1 || first.Status != second.Status || string(first.Body) != string(second.Body) || first.ResourceID != second.ResourceID {
		t.Fatalf("expected cached replay, calls=%d first=%+v second=%+v", repo.rushCreateCalls, first, second)
	}
}

func TestSystemRushBatchCannotPauseOrArchiveAfterRegistrationCloses(t *testing.T) {
	startsAt := beijingTime(t, "2026-07-24T13:00:00+08:00")
	for _, action := range []string{"pause", "archive"} {
		t.Run(action, func(t *testing.T) {
			repo := &fakeRepository{
				rounds: []SaleRound{{
					SystemSlotKey: "2026-07-24@13:00",
					StartsAt:      startsAt,
				}},
			}
			manager := NewManager(repo, func() time.Time { return startsAt.Add(-time.Hour) })

			_, appErr := manager.UpdateBatchStatus(
				context.Background(),
				auth.User{ID: "seller-1"},
				BatchActionInput{BatchID: "batch-1"},
				action,
			)
			if appErr == nil || appErr.Code != domain.CodeInvalidStateTransition || appErr.Status != http.StatusConflict {
				t.Fatalf("expected system slot lock conflict, got %v", appErr)
			}
			if repo.updateStatusCalls != 0 {
				t.Fatalf("locked action must not reach repository")
			}
		})
	}
}

func TestSystemRushBatchCanArchiveBeforeRegistrationClosesAndCustomRoundsStayEditable(t *testing.T) {
	startsAt := beijingTime(t, "2026-07-24T13:00:00+08:00")
	tests := []struct {
		name string
		now  time.Time
		key  string
	}{
		{name: "system slot before cutoff", now: startsAt.Add(-time.Hour - time.Second), key: "2026-07-24@13:00"},
		{name: "historical custom round", now: startsAt, key: ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &fakeRepository{rounds: []SaleRound{{SystemSlotKey: test.key, StartsAt: startsAt}}}
			manager := NewManager(repo, func() time.Time { return test.now })
			_, appErr := manager.UpdateBatchStatus(
				context.Background(),
				auth.User{ID: "seller-1"},
				BatchActionInput{BatchID: "batch-1"},
				"archive",
			)
			if appErr != nil {
				t.Fatalf("expected archive to remain available, got %v", appErr)
			}
			if repo.updateStatusCalls != 1 {
				t.Fatalf("expected repository update, got %d", repo.updateStatusCalls)
			}
		})
	}
}

type fakeRepository struct {
	batch               Batch
	offers              []Offer
	createdBatch        *Batch
	createdOffer        *Offer
	createdRoundRequest []RoundOfferInput
	rushPublication     *RushOfferPublication
	rounds              []SaleRound
	updateStatusCalls   int
	rushCredentials     []CredentialImportRow
	rushCreateCalls     int
	idempotencyEntry    *idempotency.Entry
}

func (f *fakeRepository) CreateAPIQuotaBatch(_ context.Context, batch Batch) (Batch, *domain.AppError) {
	f.createdBatch = &batch
	f.batch = batch
	return batch, nil
}

func (f *fakeRepository) GetAPIQuotaBatchForOwner(_ context.Context, _, _ string) (Batch, *domain.AppError) {
	return f.batch, nil
}

func (f *fakeRepository) ListAPIQuotaBatchesForOwner(_ context.Context, _, _ string, _ domain.PageRequest) (domain.Page[Batch], *domain.AppError) {
	return domain.Page[Batch]{Items: []Batch{f.batch}}, nil
}

func (f *fakeRepository) CreateAPIQuotaOffer(_ context.Context, offer Offer, _ int, _ time.Time) (Offer, *domain.AppError) {
	f.createdOffer = &offer
	return offer, nil
}

func (f *fakeRepository) GetAPIQuotaOfferForOwner(_ context.Context, _, _ string) (Offer, *domain.AppError) {
	return f.offers[0], nil
}

func (f *fakeRepository) ListAPIQuotaOffersForBatch(_ context.Context, _, _ string) ([]Offer, *domain.AppError) {
	return f.offers, nil
}

func (f *fakeRepository) CreateAPIQuotaSaleRound(_ context.Context, round SaleRound, requested []RoundOfferInput, _ time.Time) (SaleRound, *domain.AppError) {
	f.createdRoundRequest = append([]RoundOfferInput(nil), requested...)
	return round, nil
}

func (f *fakeRepository) ListAPIQuotaSaleRoundsForBatch(_ context.Context, _, _ string) ([]SaleRound, *domain.AppError) {
	return f.rounds, nil
}

func (f *fakeRepository) PublishAPIQuotaBatch(_ context.Context, _ BatchActionInput, _ time.Time) (Batch, *domain.AppError) {
	return f.batch, nil
}

func (f *fakeRepository) UpdateAPIQuotaBatchStatus(_ context.Context, _ BatchActionInput, _ string, _ time.Time) (Batch, *domain.AppError) {
	f.updateStatusCalls++
	return f.batch, nil
}

func (f *fakeRepository) ListPublicAPIQuotaOffers(_ context.Context, _ PublicOfferFilter, _ domain.PageRequest, _ time.Time) (domain.Page[OfferCard], *domain.AppError) {
	return domain.Page[OfferCard]{}, nil
}

func (f *fakeRepository) GetPublicAPIQuotaOffer(_ context.Context, _ string, _ time.Time) (OfferCard, *domain.AppError) {
	return OfferCard{}, nil
}

func (f *fakeRepository) CreateAPIQuotaOrderWithIdempotency(_ context.Context, _ idempotency.Entry, _ CreateOrderInput, _ time.Time, _ apiorder.CompletionBuilder) (apiorder.Order, idempotency.Completion, *domain.AppError) {
	return apiorder.Order{}, idempotency.Completion{}, nil
}

func (f *fakeRepository) GetAPIQuotaOrderForBuyer(_ context.Context, _, _ string, _ time.Time) (apiorder.Order, *domain.AppError) {
	return apiorder.Order{}, nil
}

func (f *fakeRepository) ImportAPIQuotaCredentials(_ context.Context, _, offerID string, rows []CredentialImportRow, _ time.Time) (CredentialSummary, *domain.AppError) {
	return CredentialSummary{OfferID: offerID, Available: len(rows)}, nil
}

func (f *fakeRepository) GetAPIQuotaCredentialSummary(_ context.Context, _, offerID string) (CredentialSummary, *domain.AppError) {
	return CredentialSummary{OfferID: offerID}, nil
}

func (f *fakeRepository) CreateSystemRushOfferWithIdempotency(_ context.Context, entry idempotency.Entry, publication RushOfferPublication, credentials []CredentialImportRow, now time.Time, buildCompletion RushOfferCompletionBuilder) (RushOfferPublication, idempotency.Completion, *domain.AppError) {
	f.rushCreateCalls++
	f.rushPublication = &publication
	f.rushCredentials = append([]CredentialImportRow(nil), credentials...)
	completion, appErr := buildCompletion(publication)
	if appErr == nil {
		entry.State = "completed"
		entry.Status = completion.Status
		entry.ContentType = completion.ContentType
		entry.Body = append([]byte(nil), completion.Body...)
		entry.BodyCacheAllowed = !completion.SkipBodyCache
		entry.ResourceType = completion.ResourceType
		entry.ResourceID = completion.ResourceID
		entry.CompletedAt = &now
		f.idempotencyEntry = &entry
	}
	return publication, completion, appErr
}

func (f *fakeRepository) BeginIdempotency(_ context.Context, entry idempotency.Entry) (*idempotency.Entry, *domain.AppError) {
	if f.idempotencyEntry != nil {
		return f.idempotencyEntry, nil
	}
	f.idempotencyEntry = &entry
	return f.idempotencyEntry, nil
}

func (f *fakeRepository) CompleteIdempotency(_ context.Context, entry *idempotency.Entry, completion idempotency.Completion, completedAt time.Time) *domain.AppError {
	entry.State = "completed"
	entry.Status = completion.Status
	entry.ContentType = completion.ContentType
	entry.Body = append([]byte(nil), completion.Body...)
	entry.BodyCacheAllowed = !completion.SkipBodyCache
	entry.ResourceType = completion.ResourceType
	entry.ResourceID = completion.ResourceID
	entry.CompletedAt = &completedAt
	f.idempotencyEntry = entry
	return nil
}

func (f *fakeRepository) CancelIdempotency(_ context.Context, entry *idempotency.Entry, failedAt time.Time) *domain.AppError {
	entry.State = "failed"
	entry.CompletedAt = &failedAt
	entry.ExpiresAt = failedAt.Add(idempotency.FailedRetention)
	return nil
}

func validRushOfferInput(t *testing.T, now time.Time) CreateRushOfferInput {
	t.Helper()
	slot, appErr := ResolveOpenSystemSaleSlot("2026-07-24@09:00", now)
	if appErr != nil {
		t.Fatalf("resolve open rush slot: %v", appErr)
	}
	return CreateRushOfferInput{
		APIServiceID:       "10000000-0000-0000-0000-000000000001",
		SourceType:         SourceTypeSub2API,
		Name:               "$50 限时额度包",
		USDAllowance:       "50",
		PriceCNY:           "5",
		ModelMultiplier:    "1",
		QuotaUsagePolicy:   validQuotaUsagePolicy(),
		Copies:             3,
		DeliveryMode:       DeliveryModeManual,
		DeliveryETAMinutes: 10,
		SlotKey:            slot.Key,
		ExpiresAt:          slot.EndsAt.Add(time.Hour),
		SourceConfirmedAt:  now,
	}
}

func testRushOfferCompletion(publication RushOfferPublication) (idempotency.Completion, *domain.AppError) {
	return idempotency.Completion{
		Status:       http.StatusCreated,
		ContentType:  "application/json; charset=utf-8",
		Body:         []byte(publication.Offer.ID),
		ResourceType: "api_quota_offer",
		ResourceID:   publication.Offer.ID,
	}, nil
}

func validBatch(now time.Time) Batch {
	confirmed := now.Add(-time.Minute)
	return Batch{
		ID:                        "20000000-0000-0000-0000-000000000001",
		APIServiceID:              "10000000-0000-0000-0000-000000000001",
		OwnerUserID:               "seller-1",
		DistributionSystem:        DistributionSub2API,
		ServiceOrderable:          true,
		DeclaredTTFTBand:          "under_1s",
		DeclaredMaxConcurrency:    10,
		PerformanceConfirmedAt:    &confirmed,
		Status:                    BatchStatusDraft,
		SaleCutoffAt:              now.Add(4 * time.Hour),
		ExpiresAt:                 now.Add(5 * time.Hour),
		DeclaredTotalUSDAllowance: "50000.000000",
		UnallocatedUSDAllowance:   "50000.000000",
	}
}

func validOffer(batch Batch, saleMode string) Offer {
	return Offer{
		ID:                 "30000000-0000-0000-0000-000000000001",
		BatchID:            batch.ID,
		APIServiceID:       batch.APIServiceID,
		OwnerUserID:        batch.OwnerUserID,
		DistributionSystem: batch.DistributionSystem,
		Name:               "$50 额度包",
		USDAllowance:       "50.000000",
		PriceCNY:           "5.00",
		ModelMultiplier:    "1.0000",
		QuotaUsagePolicy:   validQuotaUsagePolicy(),
		DeliveryMode:       DeliveryModeManual,
		DeliveryETAMinutes: 10,
		SaleMode:           saleMode,
		Status:             OfferStatusDraft,
	}
}

func validQuotaUsagePolicy() apimarket.QuotaUsagePolicy {
	return apimarket.QuotaUsagePolicy{
		FiveHour: apimarket.QuotaUsageLimit{Mode: apimarket.QuotaLimitModeLimited, AmountUSD: "5"},
		Daily:    apimarket.QuotaUsageLimit{Mode: apimarket.QuotaLimitModeUnlimited},
	}
}
