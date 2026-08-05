package postgres

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/apiquota"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAPIQuotaPostgresPublishCreatesAuthoritativeInventory(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("C2C_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer pool.Close()
	var databaseName string
	if err := pool.QueryRow(ctx, "select current_database()").Scan(&databaseName); err != nil {
		t.Fatalf("read test database name: %v", err)
	}
	if !strings.HasSuffix(databaseName, "_quota_test") {
		t.Fatalf("refusing to run inventory integration test against non-dedicated database %q", databaseName)
	}

	now := time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC)
	sellerID := uuid.NewString()
	contactID := uuid.NewString()
	buyerID := uuid.NewString()
	buyerContactID := uuid.NewString()
	serviceID := uuid.NewString()
	seedQuotaServiceForTest(t, ctx, pool, sellerID, contactID, buyerID, buyerContactID, serviceID, now)
	store := &Store{pool: pool}
	manager := apiquota.NewManager(store, func() time.Time { return now })
	user := auth.User{ID: sellerID}

	batch, appErr := manager.CreateBatch(ctx, user, apiquota.CreateBatchInput{
		APIServiceID:              serviceID,
		SourceType:                apiquota.SourceTypeSub2API,
		DeclaredTotalUSDAllowance: "600",
		SaleCutoffAt:              now.Add(5 * time.Hour),
		ExpiresAt:                 now.Add(6 * time.Hour),
		SourceConfirmedAt:         now,
	})
	if appErr != nil {
		t.Fatalf("create quota batch: %v", appErr)
	}
	offer, appErr := manager.CreateOffer(ctx, user, apiquota.CreateOfferInput{
		BatchID:            batch.ID,
		Name:               "$50 额度包",
		USDAllowance:       "50",
		PriceCNY:           "5",
		ModelMultiplier:    "1",
		QuotaUsagePolicy:   integrationQuotaUsagePolicy(),
		DeliveryMode:       apiquota.DeliveryModeManual,
		DeliveryETAMinutes: 10,
		SaleMode:           apiquota.SaleModeContinuous,
		ContinuousCopies:   10,
	})
	if appErr != nil {
		t.Fatalf("create quota offer: %v", appErr)
	}
	batch, appErr = manager.PublishBatch(ctx, user, apiquota.BatchActionInput{BatchID: batch.ID, ExpectedVersion: 1})
	if appErr != nil {
		t.Fatalf("publish quota batch: %v", appErr)
	}
	if batch.Status != apiquota.BatchStatusPublished || batch.UnallocatedUSDAllowance != "100.000000" {
		t.Fatalf("unexpected published batch: %+v", batch)
	}
	var available int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM api_quota_inventory_units
		WHERE offer_id = $1 AND status = 'available'
	`, offer.ID).Scan(&available); err != nil {
		t.Fatalf("count inventory: %v", err)
	}
	if available != 10 {
		t.Fatalf("expected 10 inventory units, got %d", available)
	}
	page, appErr := manager.PublicOffers(ctx, apiquota.PublicOfferFilter{OnlyOrderable: true}, domain.PageRequest{Limit: 20})
	if appErr != nil {
		t.Fatalf("list public offers: %v", appErr)
	}
	if len(page.Items) != 1 || page.Items[0].ID != offer.ID || page.Items[0].AvailableCopies != 10 || !page.Items[0].IsOrderable {
		t.Fatalf("unexpected public offer projection: %+v", page.Items)
	}
	if page.Items[0].QuotaUsagePolicy.FiveHour.Mode != apimarket.QuotaLimitModeLimited || page.Items[0].QuotaUsagePolicy.FiveHour.AmountUSD != "5.250000" || page.Items[0].QuotaUsagePolicy.Daily.Mode != apimarket.QuotaLimitModeUnlimited {
		t.Fatalf("unexpected public quota policy: %+v", page.Items[0].QuotaUsagePolicy)
	}

	buildCompletion := func(order apiorder.Order) (idempotency.Completion, *domain.AppError) {
		return idempotency.Completion{
			Status: 201, ContentType: "application/json", Body: []byte(`{"id":"` + order.ID + `"}`),
			ResourceType: "api_order", ResourceID: order.ID,
		}, nil
	}
	completion, appErr := manager.CreateOrderWithIdempotency(ctx, buyerID, "/api/v1/api-quota-offers/{id}/orders", "quota-order-key", "quota-order-hash", apiquota.CreateOrderInput{
		OfferID: offer.ID, BuyerContactMethodID: buyerContactID,
		SelectedAccessMode: "buyer_dedicated_sub_key", PaymentMethod: "wechat", RequestID: "quota-order-request",
	}, buildCompletion)
	if appErr != nil {
		t.Fatalf("create quota order: %v", appErr)
	}
	order, appErr := store.GetAPIOrderForBuyer(ctx, buyerID, completion.ResourceID, now)
	if appErr != nil {
		t.Fatalf("read quota order: %v", appErr)
	}
	if order.PurchaseKind != apiorder.PurchaseKindLimitedQuotaOffer || order.APIQuotaOfferID != offer.ID || order.PaymentWindowMinutesSnapshot != 10 || !order.PaymentExpiresAt.Equal(now.Add(10*time.Minute)) {
		t.Fatalf("unexpected quota order snapshot: %+v", order)
	}
	if order.QuotaUsagePolicySnapshot != page.Items[0].QuotaUsagePolicy {
		t.Fatalf("order did not freeze quota policy: order=%+v offer=%+v", order.QuotaUsagePolicySnapshot, page.Items[0].QuotaUsagePolicy)
	}
	intent, err := store.getAPIPurchaseIntent(ctx, pool, order.APIPurchaseIntentID, false)
	if err != nil {
		t.Fatalf("read quota purchase intent: %v", err)
	}
	if intent.QuotaUsagePolicySnapshot != order.QuotaUsagePolicySnapshot || !strings.Contains(intent.QuotaOfferSnapshot, `"quotaUsagePolicy"`) {
		t.Fatalf("intent did not freeze self-describing quota policy: intent=%+v snapshot=%s", intent.QuotaUsagePolicySnapshot, intent.QuotaOfferSnapshot)
	}
	replay, appErr := manager.CreateOrderWithIdempotency(ctx, buyerID, "/api/v1/api-quota-offers/{id}/orders", "quota-order-key", "quota-order-hash", apiquota.CreateOrderInput{
		OfferID: offer.ID, BuyerContactMethodID: buyerContactID,
		SelectedAccessMode: "buyer_dedicated_sub_key", PaymentMethod: "wechat", RequestID: "quota-order-request",
	}, buildCompletion)
	if appErr != nil || replay.ResourceID != order.ID {
		t.Fatalf("expected idempotent replay of %s, got %+v %v", order.ID, replay, appErr)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM api_quota_inventory_units
		WHERE offer_id = $1 AND status = 'reserved'
	`, offer.ID).Scan(&available); err != nil {
		t.Fatalf("count reserved inventory: %v", err)
	}
	if available != 1 {
		t.Fatalf("idempotent order must reserve exactly one unit, got %d", available)
	}
	actionNow := now.Add(time.Minute)
	orderService := apiorder.NewService(store, nil, nil, nil, idempotency.NewService(store, func() time.Time { return actionNow }), func() time.Time { return actionNow })
	_, appErr = orderService.CancelWithIdempotency(ctx, buyerID, "/api/v1/me/api-orders/{id}/cancel", "quota-order-cancel", "quota-order-cancel-hash", apiorder.ActionInput{
		OrderID: order.ID, Reason: "买家尚未付款，取消本次订单。", ExpectedVersion: order.Version, RequestID: "quota-order-cancel-request",
	}, buildCompletion)
	if appErr != nil {
		t.Fatalf("cancel quota order: %v", appErr)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM api_quota_inventory_units
		WHERE offer_id = $1 AND status = 'available'
	`, offer.ID).Scan(&available); err != nil {
		t.Fatalf("count released inventory: %v", err)
	}
	if available != 10 {
		t.Fatalf("pending cancellation must restore inventory while the sale window is open, got %d", available)
	}

	insufficientBatch, appErr := manager.CreateBatch(ctx, user, apiquota.CreateBatchInput{
		APIServiceID:              serviceID,
		SourceType:                apiquota.SourceTypeSub2API,
		DeclaredTotalUSDAllowance: "100",
		SaleCutoffAt:              now.Add(5 * time.Hour),
		ExpiresAt:                 now.Add(6 * time.Hour),
		SourceConfirmedAt:         now,
	})
	if appErr != nil {
		t.Fatalf("create insufficient batch: %v", appErr)
	}
	_, appErr = manager.CreateOffer(ctx, user, apiquota.CreateOfferInput{
		BatchID:            insufficientBatch.ID,
		Name:               "$50 超额计划",
		USDAllowance:       "50",
		PriceCNY:           "5",
		ModelMultiplier:    "1",
		QuotaUsagePolicy:   integrationQuotaUsagePolicy(),
		DeliveryMode:       apiquota.DeliveryModeManual,
		DeliveryETAMinutes: 10,
		SaleMode:           apiquota.SaleModeContinuous,
		ContinuousCopies:   3,
	})
	if appErr != nil {
		t.Fatalf("create insufficient offer plan: %v", appErr)
	}
	_, appErr = manager.PublishBatch(ctx, user, apiquota.BatchActionInput{BatchID: insufficientBatch.ID, ExpectedVersion: 1})
	if appErr == nil || appErr.Code != domain.CodeAPIQuotaInsufficientAllowance {
		t.Fatalf("expected insufficient allowance conflict, got %v", appErr)
	}
	var leakedUnits int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FROM api_quota_inventory_units WHERE batch_id = $1
	`, insufficientBatch.ID).Scan(&leakedUnits); err != nil {
		t.Fatalf("count rolled-back inventory: %v", err)
	}
	if leakedUnits != 0 {
		t.Fatalf("failed publish leaked %d inventory units", leakedUnits)
	}

	cleanupQuotaServiceForTest(t, ctx, pool, sellerID, buyerID)
}

func TestOwnerAPIServiceSalesProjectionTransitionsFromSellingToExpired(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("C2C_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(pool.Close)
	var databaseName string
	if err := pool.QueryRow(ctx, "select current_database()").Scan(&databaseName); err != nil {
		t.Fatalf("read test database name: %v", err)
	}
	if !strings.HasSuffix(databaseName, "_quota_test") {
		t.Fatalf("refusing to run owner sales integration test against non-dedicated database %q", databaseName)
	}

	now := time.Now().UTC().Truncate(time.Second)
	sellerID := uuid.NewString()
	contactID := uuid.NewString()
	buyerID := uuid.NewString()
	buyerContactID := uuid.NewString()
	serviceID := uuid.NewString()
	seedQuotaServiceForTest(t, ctx, pool, sellerID, contactID, buyerID, buyerContactID, serviceID, now)
	t.Cleanup(func() {
		cleanupQuotaServiceForTest(t, ctx, pool, sellerID, buyerID)
	})

	store := &Store{pool: pool}
	manager := apiquota.NewManager(store, func() time.Time { return now })
	user := auth.User{ID: sellerID}
	batch, appErr := manager.CreateBatch(ctx, user, apiquota.CreateBatchInput{
		APIServiceID:              serviceID,
		SourceType:                apiquota.SourceTypeSub2API,
		DeclaredTotalUSDAllowance: "500",
		SaleCutoffAt:              now.Add(5 * time.Hour),
		ExpiresAt:                 now.Add(6 * time.Hour),
		SourceConfirmedAt:         now,
	})
	if appErr != nil {
		t.Fatalf("create quota batch: %v", appErr)
	}
	_, appErr = manager.CreateOffer(ctx, user, apiquota.CreateOfferInput{
		BatchID:            batch.ID,
		Name:               "$50 额度包",
		USDAllowance:       "50",
		PriceCNY:           "5",
		ModelMultiplier:    "1",
		QuotaUsagePolicy:   integrationQuotaUsagePolicy(),
		DeliveryMode:       apiquota.DeliveryModeManual,
		DeliveryETAMinutes: 10,
		SaleMode:           apiquota.SaleModeContinuous,
		ContinuousCopies:   10,
	})
	if appErr != nil {
		t.Fatalf("create quota offer: %v", appErr)
	}
	_, appErr = manager.CreateOffer(ctx, user, apiquota.CreateOfferInput{
		BatchID:            batch.ID,
		Name:               "$25 额度包",
		USDAllowance:       "25",
		PriceCNY:           "3",
		ModelMultiplier:    "1",
		QuotaUsagePolicy:   integrationQuotaUsagePolicy(),
		DeliveryMode:       apiquota.DeliveryModeManual,
		DeliveryETAMinutes: 10,
		SaleMode:           apiquota.SaleModeContinuous,
		ContinuousCopies:   4,
	})
	if appErr != nil {
		t.Fatalf("create second quota offer: %v", appErr)
	}
	batch, appErr = manager.PublishBatch(ctx, user, apiquota.BatchActionInput{
		BatchID:         batch.ID,
		ExpectedVersion: batch.Version,
	})
	if appErr != nil {
		t.Fatalf("publish quota batch: %v", appErr)
	}
	if _, appErr = manager.CreateBatch(ctx, user, apiquota.CreateBatchInput{
		APIServiceID:              serviceID,
		SourceType:                apiquota.SourceTypeSub2API,
		DeclaredTotalUSDAllowance: "100",
		SaleCutoffAt:              now.Add(8 * time.Hour),
		ExpiresAt:                 now.Add(9 * time.Hour),
		SourceConfirmedAt:         now,
	}); appErr != nil {
		t.Fatalf("create second quota batch: %v", appErr)
	}

	active, appErr := store.ListAPIServicesByOwner(ctx, sellerID, apimarket.OwnerServiceFilter{
		SalesView: apimarket.OwnerSalesViewActive,
	}, domain.PageRequest{Limit: 20})
	if appErr != nil {
		t.Fatalf("list active owner API services: %v", appErr)
	}
	if len(active.Items) != 1 ||
		active.Items[0].ID != serviceID ||
		active.Items[0].SalesSummary.OverallState != apimarket.ServiceSalesStateSelling ||
		len(active.Items[0].SalesSummary.Channels) != 1 ||
		active.Items[0].SalesSummary.Channels[0].Kind != apimarket.ServiceSalesChannelLimitedQuota ||
		active.Items[0].SalesSummary.Channels[0].AvailableCopies <= 0 {
		t.Fatalf("unexpected active owner sales projection: %+v", active.Items)
	}

	if _, err := pool.Exec(ctx, `
		UPDATE api_quota_batches
		SET sale_cutoff_at = now() - interval '2 hours',
		    expires_at = now() - interval '1 hour',
		    updated_at = now()
		WHERE id = $1
	`, batch.ID); err != nil {
		t.Fatalf("expire quota batch: %v", err)
	}
	expired, appErr := store.ListAPIServicesByOwner(ctx, sellerID, apimarket.OwnerServiceFilter{
		SalesView: apimarket.OwnerSalesViewExpired,
	}, domain.PageRequest{Limit: 20})
	if appErr != nil {
		t.Fatalf("list expired owner API services: %v", appErr)
	}
	if len(expired.Items) != 1 ||
		expired.Items[0].ID != serviceID ||
		expired.Items[0].SalesSummary.OverallState != apimarket.ServiceSalesStateExpired ||
		expired.Items[0].SalesSummary.Channels[0].State != apimarket.ServiceSalesStateExpired {
		t.Fatalf("unexpected expired owner sales projection: %+v", expired.Items)
	}

	all, appErr := store.ListAPIServicesByOwner(ctx, sellerID, apimarket.OwnerServiceFilter{
		SalesView: apimarket.OwnerSalesViewAll,
	}, domain.PageRequest{Limit: 20})
	if appErr != nil || len(all.Items) != 1 || all.Items[0].ID != serviceID {
		t.Fatalf("expired service must remain reusable in all view, got %+v %v", all.Items, appErr)
	}
}

func TestAPIQuotaPostgresArchiveSystemRushRetiresUnsoldCapacity(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("C2C_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(pool.Close)
	var databaseName string
	if err := pool.QueryRow(ctx, "select current_database()").Scan(&databaseName); err != nil {
		t.Fatalf("read test database name: %v", err)
	}
	if !strings.HasSuffix(databaseName, "_quota_test") {
		t.Fatalf("refusing to run inventory integration test against non-dedicated database %q", databaseName)
	}

	slotStartsAt := time.Date(2026, 7, 25, 1, 0, 0, 0, time.UTC)
	currentTime := slotStartsAt.Add(-2 * time.Hour)
	sellerID := uuid.NewString()
	contactID := uuid.NewString()
	buyerID := uuid.NewString()
	buyerContactID := uuid.NewString()
	serviceID := uuid.NewString()
	seedQuotaServiceForTest(t, ctx, pool, sellerID, contactID, buyerID, buyerContactID, serviceID, currentTime)
	t.Cleanup(func() {
		cleanupQuotaServiceForTest(t, ctx, pool, sellerID, buyerID)
	})

	codec, err := newContactCodec(ContactCryptoConfig{
		EncryptionKey: "quota-archive-encryption", FingerprintKey: "quota-archive-fingerprint",
		EncryptionKeyVersion: "quota-archive-v1", FingerprintKeyVersion: "quota-archive-v1",
	})
	if err != nil {
		t.Fatalf("create credential codec: %v", err)
	}
	store := &Store{pool: pool, contactCodec: codec}
	manager := apiquota.NewManager(store, func() time.Time { return currentTime })
	user := auth.User{ID: sellerID}
	var publication apiquota.RushOfferPublication

	_, appErr := manager.CreateRushOfferWithIdempotency(
		ctx,
		user,
		"POST /api/v1/owner/api-services/{id}/quota-rush-offers",
		"archive-system-rush",
		"archive-system-rush-hash",
		apiquota.CreateRushOfferInput{
			APIServiceID:       serviceID,
			SourceType:         apiquota.SourceTypeSub2API,
			Name:               "$50 归档测试额度包",
			USDAllowance:       "50",
			PriceCNY:           "5",
			ModelMultiplier:    "1",
			QuotaUsagePolicy:   integrationQuotaUsagePolicy(),
			Copies:             2,
			DeliveryMode:       apiquota.DeliveryModeManual,
			DeliveryETAMinutes: 1,
			SlotKey:            "2026-07-25@09:00",
			ExpiresAt:          slotStartsAt.Add(90 * time.Minute),
			SourceConfirmedAt:  currentTime,
		},
		func(created apiquota.RushOfferPublication) (idempotency.Completion, *domain.AppError) {
			publication = created
			return idempotency.Completion{
				Status: 201, ContentType: "application/json", Body: []byte(`{"ok":true}`),
				ResourceType: "api_quota_batch", ResourceID: created.Batch.ID,
			}, nil
		},
	)
	if appErr != nil {
		t.Fatalf("create system rush offer: %v", appErr)
	}
	if publication.Batch.Status != apiquota.BatchStatusPublished {
		t.Fatalf("unexpected system rush publication: %+v", publication)
	}
	markAPIQuotaOfferAsLegacyPreimportedForTest(t, ctx, pool, publication.Offer.ID)
	credentialSummary, appErr := manager.ImportCredentials(ctx, user, apiquota.CredentialImportInput{
		OfferID:      publication.Offer.ID,
		DeliveryKind: apiorder.DeliveryKindAPIKeyEndpoint,
		CSV:          strings.NewReader("api_base_url,api_key,instructions\nhttps://one.example/v1,sk-archive-one,legacy one\nhttps://two.example/v1,sk-archive-two,legacy two\n"),
	})
	if appErr != nil || credentialSummary.Summary.Available != 2 {
		t.Fatalf("seed legacy preimported rush credentials: summary=%+v err=%v", credentialSummary, appErr)
	}

	archived, appErr := manager.UpdateBatchStatus(
		ctx,
		user,
		apiquota.BatchActionInput{BatchID: publication.Batch.ID, ExpectedVersion: publication.Batch.Version},
		"archive",
	)
	if appErr != nil {
		t.Fatalf("archive system rush offer before registration cutoff: %v", appErr)
	}
	if archived.Status != apiquota.BatchStatusArchived || archived.UnallocatedUSDAllowance != "100.000000" {
		t.Fatalf("unexpected archived batch accounting: %+v", archived)
	}

	var retiredUnits, retiredCredentials int
	var allocationStatus, returnedAllowance, roundStatus, offerStatus string
	if err := pool.QueryRow(ctx, `
		SELECT
		  (SELECT count(*) FROM api_quota_inventory_units WHERE allocation_id = $1 AND status = 'retired'),
		  (SELECT count(*) FROM api_quota_credentials WHERE api_quota_offer_id = $2 AND status = 'retired'),
		  a.status,
		  a.returned_usd_allowance::text,
		  r.status,
		  o.status
		FROM api_quota_allocations a
		JOIN api_quota_sale_rounds r ON r.id = a.sale_round_id
		JOIN api_quota_offers o ON o.id = a.offer_id
		WHERE a.id = $1
	`, publication.Round.Allocations[0].ID, publication.Offer.ID).Scan(
		&retiredUnits,
		&retiredCredentials,
		&allocationStatus,
		&returnedAllowance,
		&roundStatus,
		&offerStatus,
	); err != nil {
		t.Fatalf("read archived system rush state: %v", err)
	}
	if retiredUnits != 2 ||
		retiredCredentials != 2 ||
		allocationStatus != "closed" ||
		returnedAllowance != "100.000000" ||
		roundStatus != apiquota.RoundStatusCancelled ||
		offerStatus != apiquota.OfferStatusArchived {
		t.Fatalf(
			"unexpected archived system rush state: units=%d credentials=%d allocation=%s returned=%s round=%s offer=%s",
			retiredUnits,
			retiredCredentials,
			allocationStatus,
			returnedAllowance,
			roundStatus,
			offerStatus,
		)
	}
}

func TestAPIQuotaPostgresPublicRoundsStayOfferSpecific(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("C2C_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer pool.Close()
	var databaseName string
	if err := pool.QueryRow(ctx, "select current_database()").Scan(&databaseName); err != nil {
		t.Fatalf("read test database name: %v", err)
	}
	if !strings.HasSuffix(databaseName, "_quota_test") {
		t.Fatalf("refusing to run inventory integration test against non-dedicated database %q", databaseName)
	}

	now := time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC)
	sellerID := uuid.NewString()
	contactID := uuid.NewString()
	buyerID := uuid.NewString()
	buyerContactID := uuid.NewString()
	serviceID := uuid.NewString()
	seedQuotaServiceForTest(t, ctx, pool, sellerID, contactID, buyerID, buyerContactID, serviceID, now)
	store := &Store{pool: pool}
	manager := apiquota.NewManager(store, func() time.Time { return now })
	user := auth.User{ID: sellerID}

	batch, appErr := manager.CreateBatch(ctx, user, apiquota.CreateBatchInput{
		APIServiceID:              serviceID,
		SourceType:                apiquota.SourceTypeSub2API,
		DeclaredTotalUSDAllowance: "150",
		SaleCutoffAt:              now.Add(5 * time.Hour),
		ExpiresAt:                 now.Add(6 * time.Hour),
		SourceConfirmedAt:         now,
	})
	if appErr != nil {
		t.Fatalf("create quota batch: %v", appErr)
	}
	createScheduledOffer := func(name, allowance string) apiquota.Offer {
		t.Helper()
		offer, createErr := manager.CreateOffer(ctx, user, apiquota.CreateOfferInput{
			BatchID: batch.ID, Name: name, USDAllowance: allowance, PriceCNY: "5",
			ModelMultiplier: "1.2500", QuotaUsagePolicy: integrationQuotaUsagePolicy(), DeliveryMode: apiquota.DeliveryModeManual,
			DeliveryETAMinutes: 10, SaleMode: apiquota.SaleModeScheduled,
		})
		if createErr != nil {
			t.Fatalf("create scheduled offer: %v", createErr)
		}
		return offer
	}
	firstOffer := createScheduledOffer("$50 第一场", "50")
	secondOffer := createScheduledOffer("$100 第二场", "100")
	firstRound, appErr := manager.CreateRound(ctx, user, apiquota.CreateRoundInput{
		BatchID: batch.ID, Name: "第一场", StartsAt: now.Add(time.Hour), EndsAt: now.Add(90 * time.Minute),
		Offers: []apiquota.RoundOfferInput{{OfferID: firstOffer.ID, Copies: 1}},
	})
	if appErr != nil {
		t.Fatalf("create first round: %v", appErr)
	}
	secondRound, appErr := manager.CreateRound(ctx, user, apiquota.CreateRoundInput{
		BatchID: batch.ID, Name: "第二场", StartsAt: now.Add(30 * time.Minute), EndsAt: now.Add(45 * time.Minute),
		Offers: []apiquota.RoundOfferInput{{OfferID: secondOffer.ID, Copies: 1}},
	})
	if appErr != nil {
		t.Fatalf("create second round: %v", appErr)
	}
	if _, appErr = manager.PublishBatch(ctx, user, apiquota.BatchActionInput{BatchID: batch.ID, ExpectedVersion: 1}); appErr != nil {
		t.Fatalf("publish quota batch: %v", appErr)
	}

	page, appErr := manager.PublicOffers(ctx, apiquota.PublicOfferFilter{}, domain.PageRequest{Limit: 20})
	if appErr != nil {
		t.Fatalf("list public offers: %v", appErr)
	}
	cards := make(map[string]apiquota.OfferCard, len(page.Items))
	for _, card := range page.Items {
		cards[card.ID] = card
	}
	if cards[firstOffer.ID].NextRound == nil || cards[firstOffer.ID].NextRound.ID != firstRound.ID {
		t.Fatalf("first offer projected another offer's round: %+v", cards[firstOffer.ID].NextRound)
	}
	if cards[secondOffer.ID].NextRound == nil || cards[secondOffer.ID].NextRound.ID != secondRound.ID {
		t.Fatalf("second offer projected another offer's round: %+v", cards[secondOffer.ID].NextRound)
	}

	cleanupQuotaServiceForTest(t, ctx, pool, sellerID, buyerID)
}

func TestAPIQuotaPostgresTimeoutAndRoundRetirementReturnAllowance(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("C2C_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer pool.Close()
	var databaseName string
	if err := pool.QueryRow(ctx, "select current_database()").Scan(&databaseName); err != nil {
		t.Fatalf("read test database name: %v", err)
	}
	if !strings.HasSuffix(databaseName, "_quota_test") {
		t.Fatalf("refusing to run inventory integration test against non-dedicated database %q", databaseName)
	}

	currentTime := time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC)
	sellerID := uuid.NewString()
	contactID := uuid.NewString()
	buyerID := uuid.NewString()
	buyerContactID := uuid.NewString()
	serviceID := uuid.NewString()
	seedQuotaServiceForTest(t, ctx, pool, sellerID, contactID, buyerID, buyerContactID, serviceID, currentTime)
	store := &Store{pool: pool}
	manager := apiquota.NewManager(store, func() time.Time { return currentTime })
	user := auth.User{ID: sellerID}
	buildCompletion := func(order apiorder.Order) (idempotency.Completion, *domain.AppError) {
		return idempotency.Completion{
			Status: 201, ContentType: "application/json", Body: []byte(`{"id":"` + order.ID + `"}`),
			ResourceType: "api_order", ResourceID: order.ID,
		}, nil
	}

	expiringBatch, appErr := manager.CreateBatch(ctx, user, apiquota.CreateBatchInput{
		APIServiceID: serviceID, SourceType: apiquota.SourceTypeSub2API,
		DeclaredTotalUSDAllowance: "50", SaleCutoffAt: currentTime.Add(5 * time.Minute),
		ExpiresAt: currentTime.Add(65 * time.Minute), SourceConfirmedAt: currentTime,
	})
	if appErr != nil {
		t.Fatalf("create expiring quota batch: %v", appErr)
	}
	expiringOffer, appErr := manager.CreateOffer(ctx, user, apiquota.CreateOfferInput{
		BatchID: expiringBatch.ID, Name: "$50 临期额度包", USDAllowance: "50", PriceCNY: "5",
		ModelMultiplier: "1", QuotaUsagePolicy: integrationQuotaUsagePolicy(), DeliveryMode: apiquota.DeliveryModeManual, DeliveryETAMinutes: 10,
		SaleMode: apiquota.SaleModeContinuous, ContinuousCopies: 1,
	})
	if appErr != nil {
		t.Fatalf("create expiring quota offer: %v", appErr)
	}
	if _, appErr = manager.PublishBatch(ctx, user, apiquota.BatchActionInput{BatchID: expiringBatch.ID, ExpectedVersion: 1}); appErr != nil {
		t.Fatalf("publish expiring quota batch: %v", appErr)
	}
	completion, appErr := manager.CreateOrderWithIdempotency(ctx, buyerID, "/api/v1/api-quota-offers/{id}/orders", "expiring-order", "expiring-order-hash", apiquota.CreateOrderInput{
		OfferID: expiringOffer.ID, BuyerContactMethodID: buyerContactID,
		SelectedAccessMode: "buyer_dedicated_sub_key", PaymentMethod: "wechat", RequestID: "expiring-order-request",
	}, buildCompletion)
	if appErr != nil {
		t.Fatalf("create expiring quota order: %v", appErr)
	}
	currentTime = currentTime.Add(10 * time.Minute)
	expiredOrder, appErr := store.GetAPIOrderForBuyer(ctx, buyerID, completion.ResourceID, currentTime)
	if appErr != nil {
		t.Fatalf("materialize expired quota order: %v", appErr)
	}
	if expiredOrder.Status != apiorder.StatusCancelled || expiredOrder.CancelReason != apiorder.CancelReasonPaymentTimeout {
		t.Fatalf("expected payment timeout cancellation, got %+v", expiredOrder)
	}
	var unitStatus, unallocated, returned string
	if err := pool.QueryRow(ctx, `
		SELECT u.status, b.unallocated_usd_allowance::text, a.returned_usd_allowance::text
		FROM api_quota_inventory_units u
		JOIN api_quota_allocations a ON a.id = u.allocation_id
		JOIN api_quota_batches b ON b.id = u.batch_id
		WHERE u.id = $1
	`, expiredOrder.APIQuotaInventoryUnitID).Scan(&unitStatus, &unallocated, &returned); err != nil {
		t.Fatalf("read retired timeout inventory: %v", err)
	}
	if unitStatus != "retired" || unallocated != "50.000000" || returned != "50.000000" {
		t.Fatalf("timeout after cutoff must retire and return allowance, got status=%s unallocated=%s returned=%s", unitStatus, unallocated, returned)
	}

	currentTime = time.Date(2026, 7, 19, 10, 0, 0, 0, time.UTC)
	roundBatch, appErr := manager.CreateBatch(ctx, user, apiquota.CreateBatchInput{
		APIServiceID: serviceID, SourceType: apiquota.SourceTypeSub2API,
		DeclaredTotalUSDAllowance: "100", SaleCutoffAt: currentTime.Add(2 * time.Hour),
		ExpiresAt: currentTime.Add(3 * time.Hour), SourceConfirmedAt: currentTime,
	})
	if appErr != nil {
		t.Fatalf("create scheduled quota batch: %v", appErr)
	}
	roundOffer, appErr := manager.CreateOffer(ctx, user, apiquota.CreateOfferInput{
		BatchID: roundBatch.ID, Name: "$50 定时额度包", USDAllowance: "50", PriceCNY: "5",
		ModelMultiplier: "1", QuotaUsagePolicy: integrationQuotaUsagePolicy(), DeliveryMode: apiquota.DeliveryModeManual, DeliveryETAMinutes: 10,
		SaleMode: apiquota.SaleModeScheduled,
	})
	if appErr != nil {
		t.Fatalf("create scheduled quota offer: %v", appErr)
	}
	round, appErr := manager.CreateRound(ctx, user, apiquota.CreateRoundInput{
		BatchID: roundBatch.ID, Name: "10:20 放量", StartsAt: currentTime.Add(20 * time.Minute), EndsAt: currentTime.Add(40 * time.Minute),
		Offers: []apiquota.RoundOfferInput{{OfferID: roundOffer.ID, Copies: 2}},
	})
	if appErr != nil {
		t.Fatalf("create scheduled quota round: %v", appErr)
	}
	if _, appErr = manager.PublishBatch(ctx, user, apiquota.BatchActionInput{BatchID: roundBatch.ID, ExpectedVersion: 1}); appErr != nil {
		t.Fatalf("publish scheduled quota batch: %v", appErr)
	}
	currentTime = currentTime.Add(20 * time.Minute)
	completion, appErr = manager.CreateOrderWithIdempotency(ctx, buyerID, "/api/v1/api-quota-offers/{id}/orders", "scheduled-order", "scheduled-order-hash", apiquota.CreateOrderInput{
		OfferID: roundOffer.ID, SaleRoundID: round.ID, BuyerContactMethodID: buyerContactID,
		SelectedAccessMode: "buyer_dedicated_sub_key", PaymentMethod: "wechat", RequestID: "scheduled-order-request",
	}, buildCompletion)
	if appErr != nil {
		t.Fatalf("create scheduled quota order: %v", appErr)
	}
	currentTime = currentTime.Add(5 * time.Minute)
	if _, appErr = store.GetAPIOrderForBuyer(ctx, buyerID, completion.ResourceID, currentTime); appErr != nil {
		t.Fatalf("materialize scheduled payment timeout: %v", appErr)
	}
	var available int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM api_quota_inventory_units WHERE allocation_id = $1 AND status = 'available'`, round.Allocations[0].ID).Scan(&available); err != nil {
		t.Fatalf("count reusable round inventory: %v", err)
	}
	if available != 2 {
		t.Fatalf("timeout inside the round must restore the claimed unit, got %d available", available)
	}
	_, appErr = manager.CreateOrderWithIdempotency(ctx, buyerID, "/api/v1/api-quota-offers/{id}/orders", "scheduled-retry", "scheduled-retry-hash", apiquota.CreateOrderInput{
		OfferID: roundOffer.ID, SaleRoundID: round.ID, BuyerContactMethodID: buyerContactID,
		SelectedAccessMode: "buyer_dedicated_sub_key", PaymentMethod: "wechat", RequestID: "scheduled-retry-request",
	}, buildCompletion)
	if appErr == nil || appErr.Code != domain.CodeAPIQuotaBuyerRoundLimit {
		t.Fatalf("expected permanent buyer round claim after timeout, got %v", appErr)
	}
	currentTime = time.Date(2026, 7, 19, 10, 41, 0, 0, time.UTC)
	if appErr = store.MaterializeExpiredAPIQuotaInventory(ctx, currentTime); appErr != nil {
		t.Fatalf("retire ended round inventory: %v", appErr)
	}
	var retired int
	var allocationStatus, roundStatus string
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE u.status = 'retired'), a.status,
		       a.returned_usd_allowance::text, b.unallocated_usd_allowance::text, r.status
		FROM api_quota_allocations a
		JOIN api_quota_inventory_units u ON u.allocation_id = a.id
		JOIN api_quota_batches b ON b.id = a.batch_id
		JOIN api_quota_sale_rounds r ON r.id = a.sale_round_id
		WHERE a.id = $1
		GROUP BY a.status, a.returned_usd_allowance, b.unallocated_usd_allowance, r.status
	`, round.Allocations[0].ID).Scan(&retired, &allocationStatus, &returned, &unallocated, &roundStatus); err != nil {
		t.Fatalf("read ended round accounting: %v", err)
	}
	if retired != 2 || allocationStatus != "closed" || roundStatus != apiquota.RoundStatusClosed || returned != "100.000000" || unallocated != "100.000000" {
		t.Fatalf("unexpected ended round accounting: retired=%d allocation=%s round=%s returned=%s unallocated=%s", retired, allocationStatus, roundStatus, returned, unallocated)
	}
	var claims int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM api_quota_round_claims WHERE sale_round_id = $1 AND buyer_user_id = $2`, round.ID, buyerID).Scan(&claims); err != nil {
		t.Fatalf("count round claims: %v", err)
	}
	if claims != 1 {
		t.Fatalf("round claim must remain after timeout and retirement, got %d", claims)
	}

	cleanupQuotaServiceForTest(t, ctx, pool, sellerID, buyerID)
}

func TestAPIQuotaPostgresConfirmPaymentConsumesAndDeliversHistoricalPreimportedCredential(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("C2C_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer pool.Close()
	var databaseName string
	if err := pool.QueryRow(ctx, "select current_database()").Scan(&databaseName); err != nil {
		t.Fatalf("read test database name: %v", err)
	}
	if !strings.HasSuffix(databaseName, "_quota_test") {
		t.Fatalf("refusing to run inventory integration test against non-dedicated database %q", databaseName)
	}
	codec, err := newContactCodec(ContactCryptoConfig{
		EncryptionKey: "quota-test-encryption", FingerprintKey: "quota-test-fingerprint",
		EncryptionKeyVersion: "quota-test-v1", FingerprintKeyVersion: "quota-test-v1",
	})
	if err != nil {
		t.Fatalf("create credential codec: %v", err)
	}

	currentTime := time.Date(2026, 7, 19, 12, 0, 0, 0, time.UTC)
	sellerID := uuid.NewString()
	contactID := uuid.NewString()
	buyerID := uuid.NewString()
	buyerContactID := uuid.NewString()
	serviceID := uuid.NewString()
	seedQuotaServiceForTest(t, ctx, pool, sellerID, contactID, buyerID, buyerContactID, serviceID, currentTime)
	store := &Store{pool: pool, contactCodec: codec}
	manager := apiquota.NewManager(store, func() time.Time { return currentTime })
	user := auth.User{ID: sellerID}
	buildCompletion := func(order apiorder.Order) (idempotency.Completion, *domain.AppError) {
		return idempotency.Completion{
			Status: 200, ContentType: "application/json", Body: []byte(`{"id":"` + order.ID + `"}`),
			ResourceType: "api_order", ResourceID: order.ID,
		}, nil
	}

	batch, appErr := manager.CreateBatch(ctx, user, apiquota.CreateBatchInput{
		APIServiceID: serviceID, SourceType: apiquota.SourceTypeSub2API,
		DeclaredTotalUSDAllowance: "50", SaleCutoffAt: currentTime.Add(time.Hour),
		ExpiresAt: currentTime.Add(2 * time.Hour), SourceConfirmedAt: currentTime,
	})
	if appErr != nil {
		t.Fatalf("create preimported quota batch: %v", appErr)
	}
	offer, appErr := manager.CreateOffer(ctx, user, apiquota.CreateOfferInput{
		BatchID: batch.ID, Name: "$50 预导入额度包", USDAllowance: "50", PriceCNY: "5",
		ModelMultiplier: "1", QuotaUsagePolicy: integrationQuotaUsagePolicy(), DeliveryMode: apiquota.DeliveryModeManual, DeliveryETAMinutes: 1,
		SaleMode: apiquota.SaleModeContinuous, ContinuousCopies: 1,
	})
	if appErr != nil {
		t.Fatalf("create quota offer before legacy fixture conversion: %v", appErr)
	}
	markAPIQuotaOfferAsLegacyPreimportedForTest(t, ctx, pool, offer.ID)
	importResult, appErr := manager.ImportCredentials(ctx, user, apiquota.CredentialImportInput{
		OfferID: offer.ID, DeliveryKind: apiorder.DeliveryKindAPIKeyEndpoint,
		CSV: strings.NewReader("api_base_url,api_key,instructions\nhttps://api.example.com/v1,sk-quota-buyer-only,买家专属接入信息\n"),
	})
	if appErr != nil {
		t.Fatalf("import encrypted quota credential: %v", appErr)
	}
	if importResult.Imported != 1 || importResult.Summary.Available != 1 {
		t.Fatalf("unexpected credential import result: %+v", importResult)
	}
	_, appErr = manager.ImportCredentials(ctx, user, apiquota.CredentialImportInput{
		OfferID: offer.ID, DeliveryKind: apiorder.DeliveryKindAPIKeyEndpoint,
		CSV: strings.NewReader("api_base_url,api_key,instructions\nhttps://api.example.com/v1,sk-new-row-must-rollback,first\nhttps://api.example.com/v1,sk-quota-buyer-only,duplicate\n"),
	})
	if appErr == nil || appErr.Status != 409 {
		t.Fatalf("expected cross-file duplicate conflict, got %v", appErr)
	}
	credentialSummary, appErr := manager.CredentialSummary(ctx, user, offer.ID)
	if appErr != nil {
		t.Fatalf("read credential summary after rollback: %v", appErr)
	}
	if credentialSummary.Available != 1 {
		t.Fatalf("duplicate import must roll back every row, got %+v", credentialSummary)
	}
	if _, appErr = manager.PublishBatch(ctx, user, apiquota.BatchActionInput{BatchID: batch.ID, ExpectedVersion: 1}); appErr != nil {
		t.Fatalf("publish preimported quota batch: %v", appErr)
	}
	completion, appErr := manager.CreateOrderWithIdempotency(ctx, buyerID, "/api/v1/api-quota-offers/{id}/orders", "preimported-order", "preimported-order-hash", apiquota.CreateOrderInput{
		OfferID: offer.ID, BuyerContactMethodID: buyerContactID,
		SelectedAccessMode: "buyer_dedicated_sub_key", PaymentMethod: "wechat", RequestID: "preimported-order-request",
	}, buildCompletion)
	if appErr != nil {
		t.Fatalf("create preimported quota order: %v", appErr)
	}
	order, appErr := store.GetAPIOrderForBuyer(ctx, buyerID, completion.ResourceID, currentTime)
	if appErr != nil {
		t.Fatalf("read preimported quota order: %v", appErr)
	}
	orderService := apiorder.NewService(store, nil, nil, nil, idempotency.NewService(store, func() time.Time { return currentTime }), func() time.Time { return currentTime })
	currentTime = currentTime.Add(time.Minute)
	if _, appErr = orderService.SubmitPaymentWithIdempotency(ctx, buyerID, "/api/v1/me/api-orders/{id}/submit-payment", "preimported-payment", "preimported-payment-hash", apiorder.ActionInput{
		OrderID: order.ID, PaymentSummary: "已完成站外付款，交易尾号 1234。", ExpectedVersion: order.Version, RequestID: "preimported-payment-request",
	}, buildCompletion); appErr != nil {
		t.Fatalf("submit preimported quota payment: %v", appErr)
	}
	order, appErr = store.GetAPIOrderForSeller(ctx, sellerID, order.ID, currentTime)
	if appErr != nil {
		t.Fatalf("read submitted preimported order: %v", appErr)
	}
	currentTime = currentTime.Add(time.Minute)
	if _, appErr = orderService.ConfirmPaymentWithIdempotency(ctx, sellerID, "/api/v1/owner/api-orders/{id}/confirm-payment", "preimported-confirm", "preimported-confirm-hash", apiorder.ActionInput{
		OrderID: order.ID, ExpectedVersion: order.Version, RequestID: "preimported-confirm-request",
	}, buildCompletion); appErr != nil {
		t.Fatalf("confirm preimported quota payment: %v", appErr)
	}
	order, appErr = store.GetAPIOrderForBuyer(ctx, buyerID, order.ID, currentTime)
	if appErr != nil {
		t.Fatalf("read delivered preimported order: %v", appErr)
	}
	if order.Status != apiorder.StatusDeliverySubmitted || order.PaidConfirmedAt == nil || order.DeliverySubmittedAt == nil || order.DeliveryCredential == nil || order.DeliveryCredential.APIKey != "sk-quota-buyer-only" {
		t.Fatalf("unexpected preimported delivery result: %+v", order)
	}
	var unitStatus, credentialStatus string
	if err := pool.QueryRow(ctx, `
		SELECT u.status, c.status
		FROM api_orders o
		JOIN api_quota_inventory_units u ON u.id = o.api_quota_inventory_unit_id
		JOIN api_quota_credentials c ON c.id = o.api_quota_credential_id
		WHERE o.id = $1
	`, order.ID).Scan(&unitStatus, &credentialStatus); err != nil {
		t.Fatalf("read consumed preimported resources: %v", err)
	}
	if unitStatus != "consumed" || credentialStatus != "delivered" {
		t.Fatalf("confirmed preimported order must consume inventory and deliver credential, got unit=%s credential=%s", unitStatus, credentialStatus)
	}
	var paymentEvents, deliveryEvents int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE event_type = $2), count(*) FILTER (WHERE event_type = $3)
		FROM api_order_events WHERE api_order_id = $1
	`, order.ID, apiorder.EventPaymentConfirmed, apiorder.EventDeliverySubmitted).Scan(&paymentEvents, &deliveryEvents); err != nil {
		t.Fatalf("count automatic delivery events: %v", err)
	}
	if paymentEvents != 1 || deliveryEvents != 1 {
		t.Fatalf("expected payment and delivery events, got payment=%d delivery=%d", paymentEvents, deliveryEvents)
	}

	cleanupQuotaServiceForTest(t, ctx, pool, sellerID, buyerID)
}

func TestAPIQuotaPostgresRush1500BuyersFor1000Copies(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("C2C_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	defer pool.Close()
	var databaseName string
	if err := pool.QueryRow(ctx, "select current_database()").Scan(&databaseName); err != nil {
		t.Fatalf("read test database name: %v", err)
	}
	if !strings.HasSuffix(databaseName, "_quota_test") {
		t.Fatalf("refusing to run quota rush test against non-dedicated database %q", databaseName)
	}

	const buyerCount = 1500
	const inventoryCopies = 1000
	now := time.Date(2026, 7, 19, 9, 0, 0, 0, time.UTC)
	currentTime := now
	sellerID := uuid.NewString()
	sellerContactID := uuid.NewString()
	serviceID := uuid.NewString()
	firstBuyerID := uuid.NewString()
	firstBuyerContactID := uuid.NewString()
	seedQuotaServiceForTest(t, ctx, pool, sellerID, sellerContactID, firstBuyerID, firstBuyerContactID, serviceID, now)

	buyerIDs := make([]uuid.UUID, buyerCount)
	buyerContactIDs := make([]string, buyerCount)
	buyerIDs[0] = uuid.MustParse(firstBuyerID)
	buyerContactIDs[0] = firstBuyerContactID
	seedAPIQuotaRushBuyers(t, ctx, pool, buyerIDs, buyerContactIDs, now)
	defer cleanupAPIQuotaRushTest(t, ctx, pool, sellerID, buyerIDs)

	store := &Store{pool: pool}
	manager := apiquota.NewManager(store, func() time.Time { return currentTime })
	batch, appErr := manager.CreateBatch(ctx, auth.User{ID: sellerID}, apiquota.CreateBatchInput{
		APIServiceID: serviceID, SourceType: apiquota.SourceTypeSub2API,
		DeclaredTotalUSDAllowance: "1000", SaleCutoffAt: now.Add(2 * time.Hour),
		ExpiresAt: now.Add(3 * time.Hour), SourceConfirmedAt: now,
	})
	if appErr != nil {
		t.Fatalf("create rush quota batch: %v", appErr)
	}
	offer, appErr := manager.CreateOffer(ctx, auth.User{ID: sellerID}, apiquota.CreateOfferInput{
		BatchID: batch.ID, Name: "$1 并发额度包", USDAllowance: "1", PriceCNY: "0.10",
		ModelMultiplier: "1", QuotaUsagePolicy: integrationQuotaUsagePolicy(), DeliveryMode: apiquota.DeliveryModeManual, DeliveryETAMinutes: 10,
		SaleMode: apiquota.SaleModeScheduled,
	})
	if appErr != nil {
		t.Fatalf("create rush quota offer: %v", appErr)
	}
	round, appErr := manager.CreateRound(ctx, auth.User{ID: sellerID}, apiquota.CreateRoundInput{
		BatchID: batch.ID, Name: "1500 抢 1000", StartsAt: now.Add(time.Minute), EndsAt: now.Add(time.Hour),
		Offers: []apiquota.RoundOfferInput{{OfferID: offer.ID, Copies: inventoryCopies}},
	})
	if appErr != nil {
		t.Fatalf("create rush quota round: %v", appErr)
	}
	if _, appErr = manager.PublishBatch(ctx, auth.User{ID: sellerID}, apiquota.BatchActionInput{BatchID: batch.ID, ExpectedVersion: 1}); appErr != nil {
		t.Fatalf("publish rush quota batch: %v", appErr)
	}
	currentTime = now.Add(time.Minute)

	buildCompletion := func(order apiorder.Order) (idempotency.Completion, *domain.AppError) {
		return idempotency.Completion{
			Status: 201, ContentType: "application/json", Body: []byte(`{"id":"` + order.ID + `"}`),
			ResourceType: "api_order", ResourceID: order.ID,
		}, nil
	}
	type rushResult struct {
		code    string
		latency time.Duration
		buyerID string
		orderID string
	}
	results := make(chan rushResult, buyerCount)
	startedAt := time.Now()
	for index := 0; index < buyerCount; index++ {
		go func(index int) {
			requestStartedAt := time.Now()
			buyerID := buyerIDs[index].String()
			completion, createErr := manager.CreateOrderWithIdempotency(ctx, buyerID, "/api/v1/api-quota-offers/{id}/orders",
				fmt.Sprintf("rush-order-%d", index), fmt.Sprintf("rush-order-hash-%d", index), apiquota.CreateOrderInput{
					OfferID: offer.ID, SaleRoundID: round.ID, BuyerContactMethodID: buyerContactIDs[index],
					SelectedAccessMode: "buyer_dedicated_sub_key", PaymentMethod: "wechat", RequestID: fmt.Sprintf("rush-request-%d", index),
				}, buildCompletion)
			result := rushResult{latency: time.Since(requestStartedAt), buyerID: buyerID, orderID: completion.ResourceID}
			if createErr != nil {
				result.code = createErr.Code
			}
			results <- result
		}(index)
	}

	successes := 0
	soldOut := 0
	unexpected := map[string]int{}
	latencies := make([]time.Duration, 0, buyerCount)
	orderIDs := make(map[string]struct{}, inventoryCopies)
	successfulBuyers := make(map[string]struct{}, inventoryCopies)
	for range buyerCount {
		result := <-results
		latencies = append(latencies, result.latency)
		switch result.code {
		case "":
			successes++
			orderIDs[result.orderID] = struct{}{}
			successfulBuyers[result.buyerID] = struct{}{}
		case domain.CodeAPIQuotaSoldOut:
			soldOut++
		default:
			unexpected[result.code]++
		}
	}
	elapsed := time.Since(startedAt)
	sort.Slice(latencies, func(left, right int) bool { return latencies[left] < latencies[right] })
	percentile := func(percent int) time.Duration {
		return latencies[(len(latencies)-1)*percent/100]
	}
	t.Logf("quota rush: requests=%d success=%d sold_out=%d pool_max_conns=%d throughput=%.1f req/s p50=%s p95=%s p99=%s elapsed=%s",
		buyerCount, successes, soldOut, pool.Stat().MaxConns(), float64(buyerCount)/elapsed.Seconds(), percentile(50), percentile(95), percentile(99), elapsed)
	if successes != inventoryCopies || soldOut != buyerCount-inventoryCopies || len(unexpected) != 0 {
		t.Fatalf("unexpected rush distribution: success=%d sold_out=%d unexpected=%v", successes, soldOut, unexpected)
	}
	if len(orderIDs) != inventoryCopies || len(successfulBuyers) != inventoryCopies {
		t.Fatalf("rush created duplicate orders or buyers: orders=%d buyers=%d", len(orderIDs), len(successfulBuyers))
	}

	var reservedUnits, distinctReservedOrders, orders, claims int
	if err := pool.QueryRow(ctx, `
		SELECT count(*), count(DISTINCT reserved_order_id)
		FROM api_quota_inventory_units
		WHERE allocation_id = $1 AND status = 'reserved'
	`, round.Allocations[0].ID).Scan(&reservedUnits, &distinctReservedOrders); err != nil {
		t.Fatalf("count rush inventory: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM api_orders WHERE api_quota_sale_round_id = $1`, round.ID).Scan(&orders); err != nil {
		t.Fatalf("count rush orders: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM api_quota_round_claims WHERE sale_round_id = $1`, round.ID).Scan(&claims); err != nil {
		t.Fatalf("count rush claims: %v", err)
	}
	if reservedUnits != inventoryCopies || distinctReservedOrders != inventoryCopies || orders != inventoryCopies || claims != inventoryCopies {
		t.Fatalf("rush authority mismatch: units=%d distinct_orders=%d orders=%d claims=%d", reservedUnits, distinctReservedOrders, orders, claims)
	}
}

func seedAPIQuotaRushBuyers(t *testing.T, ctx context.Context, pool *pgxpool.Pool, buyerIDs []uuid.UUID, buyerContactIDs []string, now time.Time) {
	t.Helper()
	userRows := make([][]any, 0, len(buyerIDs)-1)
	contactRows := make([][]any, 0, len(buyerIDs)-1)
	versionRows := make([][]any, 0, len(buyerIDs)-1)
	for index := 1; index < len(buyerIDs); index++ {
		buyerIDs[index] = uuid.New()
		buyerContactIDs[index] = uuid.NewString()
		versionID := uuid.NewString()
		userRows = append(userRows, []any{buyerIDs[index], fmt.Sprintf("quota-rush-buyer-%04d", index), "并发额度买家", "active", now, now})
		contactRows = append(contactRows, []any{buyerContactIDs[index], buyerIDs[index], "linuxdo", "linux.do", true, true, now, now})
		versionRows = append(versionRows, []any{versionID, buyerContactIDs[index], buyerIDs[index], []byte{1, 2}, make([]byte, 12), "linux.do 用户",
			fmt.Sprintf("quota-rush-fingerprint-%04d", index), "test-v1", "test-v1", now})
	}
	if _, err := pool.CopyFrom(ctx, pgx.Identifier{"users"},
		[]string{"id", "username", "display_name", "account_status", "created_at", "updated_at"}, pgx.CopyFromRows(userRows)); err != nil {
		t.Fatalf("seed rush buyers: %v", err)
	}
	if _, err := pool.CopyFrom(ctx, pgx.Identifier{"contact_methods"},
		[]string{"id", "user_id", "type", "label", "is_default", "enabled", "created_at", "updated_at"}, pgx.CopyFromRows(contactRows)); err != nil {
		t.Fatalf("seed rush buyer contacts: %v", err)
	}
	if _, err := pool.CopyFrom(ctx, pgx.Identifier{"contact_method_versions"},
		[]string{"id", "contact_method_id", "owner_user_id", "value_ciphertext", "value_nonce", "masked_value", "value_fingerprint", "encryption_key_version", "fingerprint_key_version", "created_at"}, pgx.CopyFromRows(versionRows)); err != nil {
		t.Fatalf("seed rush contact versions: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE contact_methods method
		SET current_version_id = version.id
		FROM contact_method_versions version
		WHERE method.id = version.contact_method_id AND method.user_id = ANY($1::uuid[])
	`, buyerIDs[1:]); err != nil {
		t.Fatalf("activate rush contact versions: %v", err)
	}
}

func cleanupAPIQuotaRushTest(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sellerID string, buyerIDs []uuid.UUID) {
	t.Helper()
	statements := []struct {
		query string
		args  []any
	}{
		{`UPDATE api_quota_inventory_units SET status = 'available', reserved_order_id = NULL, reserved_at = NULL, updated_at = now() WHERE reserved_order_id IN (SELECT id FROM api_orders WHERE buyer_user_id = ANY($1::uuid[]))`, []any{buyerIDs}},
		{`DELETE FROM api_quota_round_claims WHERE buyer_user_id = ANY($1::uuid[])`, []any{buyerIDs}},
		{`DELETE FROM idempotency_keys WHERE user_id = $1 OR user_id = ANY($2::uuid[])`, []any{sellerID, buyerIDs}},
		{`DELETE FROM notifications WHERE user_id = $1 OR user_id = ANY($2::uuid[])`, []any{sellerID, buyerIDs}},
		{`DELETE FROM domain_events WHERE actor_user_id = $1 OR actor_user_id = ANY($2::uuid[])`, []any{sellerID, buyerIDs}},
		{`DELETE FROM api_orders WHERE buyer_user_id = ANY($1::uuid[])`, []any{buyerIDs}},
		{`DELETE FROM api_purchase_intents WHERE buyer_user_id = ANY($1::uuid[])`, []any{buyerIDs}},
		{`DELETE FROM api_quota_inventory_units WHERE batch_id IN (SELECT id FROM api_quota_batches WHERE owner_user_id = $1)`, []any{sellerID}},
		{`DELETE FROM api_quota_allocations WHERE owner_user_id = $1`, []any{sellerID}},
		{`DELETE FROM api_quota_sale_rounds WHERE owner_user_id = $1`, []any{sellerID}},
		{`DELETE FROM api_quota_offers WHERE owner_user_id = $1`, []any{sellerID}},
		{`DELETE FROM api_quota_batches WHERE owner_user_id = $1`, []any{sellerID}},
		{`DELETE FROM api_service_payment_options WHERE api_service_id IN (SELECT id FROM api_services WHERE owner_user_id = $1)`, []any{sellerID}},
		{`DELETE FROM api_services WHERE owner_user_id = $1`, []any{sellerID}},
		{`UPDATE contact_methods SET current_version_id = NULL WHERE user_id = $1 OR user_id = ANY($2::uuid[])`, []any{sellerID, buyerIDs}},
		{`DELETE FROM contact_method_versions WHERE owner_user_id = $1 OR owner_user_id = ANY($2::uuid[])`, []any{sellerID, buyerIDs}},
		{`DELETE FROM contact_methods WHERE user_id = $1 OR user_id = ANY($2::uuid[])`, []any{sellerID, buyerIDs}},
		{`DELETE FROM users WHERE id = $1 OR id = ANY($2::uuid[])`, []any{sellerID, buyerIDs}},
	}
	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement.query, statement.args...); err != nil {
			t.Errorf("cleanup quota rush fixture: %v", err)
		}
	}
}

func integrationQuotaUsagePolicy() apimarket.QuotaUsagePolicy {
	return apimarket.QuotaUsagePolicy{
		FiveHour: apimarket.QuotaUsageLimit{Mode: apimarket.QuotaLimitModeLimited, AmountUSD: "5.25"},
		Daily:    apimarket.QuotaUsageLimit{Mode: apimarket.QuotaLimitModeUnlimited},
	}
}

func seedQuotaServiceForTest(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sellerID, contactID, buyerID, buyerContactID, serviceID string, now time.Time) {
	t.Helper()
	if _, err := pool.Exec(ctx, `
		INSERT INTO users (id, username, display_name, account_status, created_at, updated_at)
		VALUES ($1, $2, '额度卖家', 'active', $3, $3)
	`, sellerID, "quota-seller-"+strings.ReplaceAll(sellerID[:8], "-", ""), now); err != nil {
		t.Fatalf("seed seller: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO contact_methods (id, user_id, type, label, is_default, enabled, created_at, updated_at)
		VALUES ($1, $2, 'linuxdo', 'linux.do', true, true, $3, $3)
	`, contactID, sellerID, now); err != nil {
		t.Fatalf("seed contact method: %v", err)
	}
	seedContactVersionForTest(t, ctx, pool, contactID, sellerID, now)
	if _, err := pool.Exec(ctx, `
		INSERT INTO users (id, username, display_name, account_status, created_at, updated_at)
		VALUES ($1, $2, '额度买家', 'active', $3, $3)
	`, buyerID, "quota-buyer-"+strings.ReplaceAll(buyerID[:8], "-", ""), now); err != nil {
		t.Fatalf("seed buyer: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO contact_methods (id, user_id, type, label, is_default, enabled, created_at, updated_at)
		VALUES ($1, $2, 'linuxdo', 'linux.do', true, true, $3, $3)
	`, buyerContactID, buyerID, now); err != nil {
		t.Fatalf("seed buyer contact method: %v", err)
	}
	seedContactVersionForTest(t, ctx, pool, buyerContactID, buyerID, now)
	if _, err := pool.Exec(ctx, `
		INSERT INTO api_services (
			id, owner_user_id, merchant_identity_mode, owner_contact_method_id,
			title, short_description, distribution_system, billing_mode,
			declared_cny_per_usd_allowance, declared_max_usd_allowance_per_intent,
			available_usd_allowance, quota_expires_at,
			minimum_intent_cny, maximum_intent_cny, usage_visibility,
			review_status, publication_status, moderation_status,
			accepting_orders, payment_window_minutes,
			declared_ttft_band, declared_max_concurrency, performance_confirmed_at,
			prompt_audit_enabled,
			created_at, updated_at, version
		) VALUES (
			$1, $2, 'public_profile', $3, 'Sub2API 短期额度', '集成测试服务',
			'sub2api', 'metered_usd_quota', 1, 1000, 1000, $5, 1, 1000, 'offsite_panel_readonly',
			'approved', 'online', 'clear', true, 10,
			'under_1s', 20, $4, false, $4, $4, 1
		)
	`, serviceID, sellerID, contactID, now, now.AddDate(1, 0, 0)); err != nil {
		t.Fatalf("seed API service: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO api_service_access_modes (api_service_id, access_mode, public_note)
		VALUES ($1, 'buyer_dedicated_sub_key', '买家专属子 Key')
	`, serviceID); err != nil {
		t.Fatalf("seed access mode: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO api_service_payment_options (
			id, api_service_id, payment_method, enabled, payment_instructions,
			created_at, updated_at, version
		) VALUES ($1, $2, 'wechat', true, '站外确认', $3, $3, 1)
	`, uuid.NewString(), serviceID, now); err != nil {
		t.Fatalf("seed payment option: %v", err)
	}
}

func markAPIQuotaOfferAsLegacyPreimportedForTest(t *testing.T, ctx context.Context, pool *pgxpool.Pool, offerID string) {
	t.Helper()
	tag, err := pool.Exec(ctx, `
		UPDATE api_quota_offers
		SET delivery_mode = 'preimported'
		WHERE id = $1
	`, offerID)
	if err != nil {
		t.Fatalf("mark legacy preimported quota offer: %v", err)
	}
	if tag.RowsAffected() != 1 {
		t.Fatalf("expected one legacy preimported quota offer, updated %d", tag.RowsAffected())
	}
}

func seedContactVersionForTest(t *testing.T, ctx context.Context, pool *pgxpool.Pool, contactID, ownerID string, now time.Time) {
	t.Helper()
	versionID := uuid.NewString()
	if _, err := pool.Exec(ctx, `
		INSERT INTO contact_method_versions (
			id, contact_method_id, owner_user_id, value_ciphertext, value_nonce,
			masked_value, value_fingerprint, encryption_key_version, fingerprint_key_version, created_at
		) VALUES ($1, $2, $3, decode('0102', 'hex'), decode('000000000000000000000000', 'hex'),
		          'linux.do 用户', $4, 'test-v1', 'test-v1', $5)
	`, versionID, contactID, ownerID, "fingerprint-"+versionID, now); err != nil {
		t.Fatalf("seed contact version: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE contact_methods SET current_version_id = $2 WHERE id = $1`, contactID, versionID); err != nil {
		t.Fatalf("activate contact version: %v", err)
	}
}

func cleanupQuotaServiceForTest(t *testing.T, ctx context.Context, pool *pgxpool.Pool, sellerID, buyerID string) {
	t.Helper()
	statements := []struct {
		query string
		args  []any
	}{
		{`UPDATE api_quota_inventory_units SET status = 'available', reserved_order_id = NULL, reserved_at = NULL, consumed_at = NULL, retired_at = NULL, updated_at = now() WHERE reserved_order_id IN (SELECT id FROM api_orders WHERE buyer_user_id = $1)`, []any{buyerID}},
		{`UPDATE api_quota_credentials SET status = 'available', reserved_order_id = NULL, reserved_at = NULL, delivered_at = NULL, retired_at = NULL, updated_at = now() WHERE reserved_order_id IN (SELECT id FROM api_orders WHERE buyer_user_id = $1)`, []any{buyerID}},
		{`DELETE FROM api_quota_round_claims WHERE buyer_user_id = $1`, []any{buyerID}},
		{`DELETE FROM idempotency_keys WHERE user_id IN ($1, $2)`, []any{sellerID, buyerID}},
		{`DELETE FROM notifications WHERE user_id IN ($1, $2)`, []any{sellerID, buyerID}},
		{`DELETE FROM domain_events WHERE actor_user_id IN ($1, $2)`, []any{sellerID, buyerID}},
		{`DELETE FROM api_orders WHERE buyer_user_id = $1`, []any{buyerID}},
		{`DELETE FROM api_purchase_intents WHERE buyer_user_id = $1`, []any{buyerID}},
		{`DELETE FROM api_quota_inventory_units WHERE batch_id IN (SELECT id FROM api_quota_batches WHERE owner_user_id = $1)`, []any{sellerID}},
		{`DELETE FROM api_quota_allocations WHERE owner_user_id = $1`, []any{sellerID}},
		{`DELETE FROM api_quota_sale_rounds WHERE owner_user_id = $1`, []any{sellerID}},
		{`DELETE FROM api_quota_credentials WHERE seller_user_id = $1`, []any{sellerID}},
		{`DELETE FROM api_quota_offers WHERE owner_user_id = $1`, []any{sellerID}},
		{`DELETE FROM api_quota_batches WHERE owner_user_id = $1`, []any{sellerID}},
		{`DELETE FROM api_service_payment_options WHERE api_service_id IN (SELECT id FROM api_services WHERE owner_user_id = $1)`, []any{sellerID}},
		{`DELETE FROM api_services WHERE owner_user_id = $1`, []any{sellerID}},
		{`UPDATE contact_methods SET current_version_id = NULL WHERE user_id IN ($1, $2)`, []any{sellerID, buyerID}},
		{`DELETE FROM contact_method_versions WHERE owner_user_id IN ($1, $2)`, []any{sellerID, buyerID}},
		{`DELETE FROM contact_methods WHERE user_id IN ($1, $2)`, []any{sellerID, buyerID}},
		{`DELETE FROM users WHERE id IN ($1, $2)`, []any{sellerID, buyerID}},
	}
	for _, statement := range statements {
		if _, err := pool.Exec(ctx, statement.query, statement.args...); err != nil {
			t.Fatalf("cleanup quota integration fixture: %v", err)
		}
	}
}
