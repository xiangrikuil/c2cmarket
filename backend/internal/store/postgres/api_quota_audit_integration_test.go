package postgres

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apiquota"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAPIQuotaAuditAndIdempotencyAreAtomic(t *testing.T) {
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
		t.Fatalf("refusing to run quota audit integration test against non-dedicated database %q", databaseName)
	}

	now := time.Date(2026, 8, 13, 11, 0, 0, 0, time.UTC)
	sellerID := uuid.NewString()
	contactID := uuid.NewString()
	buyerID := uuid.NewString()
	buyerContactID := uuid.NewString()
	serviceID := uuid.NewString()
	seedOrderableQuotaServiceForTest(t, ctx, pool, sellerID, contactID, buyerID, buyerContactID, serviceID, now)
	t.Cleanup(func() { cleanupQuotaServiceForTest(t, context.Background(), pool, sellerID, buyerID) })

	store := &Store{pool: pool}
	manager := apiquota.NewManager(store, func() time.Time { return now })
	owner := auth.User{ID: sellerID}
	batchBuilder := func(batch apiquota.Batch) (idempotency.Completion, *domain.AppError) {
		return idempotency.Completion{
			Status: http.StatusCreated, ContentType: "application/json", Body: []byte(`{"kind":"batch"}`),
			ResourceType: "api_quota_batch", ResourceID: batch.ID,
		}, nil
	}
	batchInput := apiquota.CreateBatchInput{
		APIServiceID: serviceID, SourceType: apiquota.SourceTypeOther,
		SourceLabel:               "private-source-label-must-not-enter-audit",
		DeclaredTotalUSDAllowance: "600", SaleCutoffAt: now.Add(5 * time.Hour),
		ExpiresAt: now.Add(6 * time.Hour), SourceConfirmedAt: now,
		RequestID: "quota-audit-batch-create",
	}
	created, appErr := manager.CreateBatchWithIdempotency(ctx, owner, "quota-batch-create", "batch-key", "batch-hash", batchInput, batchBuilder)
	if appErr != nil {
		t.Fatalf("create quota batch: %v", appErr)
	}
	batchID := created.ResourceID
	replayed, appErr := manager.CreateBatchWithIdempotency(ctx, owner, "quota-batch-create", "batch-key", "batch-hash", batchInput, batchBuilder)
	if appErr != nil || replayed.ResourceID != batchID || !equivalentJSONBodies(created.Body, replayed.Body) {
		t.Fatalf("replay quota batch: completion=%+v error=%v", replayed, appErr)
	}

	offerBuilder := func(offer apiquota.Offer) (idempotency.Completion, *domain.AppError) {
		return idempotency.Completion{
			Status: http.StatusCreated, ContentType: "application/json", Body: []byte(`{"kind":"offer"}`),
			ResourceType: "api_quota_offer", ResourceID: offer.ID,
		}, nil
	}
	offerInput := apiquota.CreateOfferInput{
		BatchID: batchID, Name: "$50 审计额度包", USDAllowance: "50", PriceCNY: "5", ModelMultiplier: "1",
		QuotaUsagePolicy: integrationQuotaUsagePolicy(), DeliveryMode: apiquota.DeliveryModeManual,
		DeliveryETAMinutes: 10, SaleMode: apiquota.SaleModeContinuous, ContinuousCopies: 10,
		RequestID: "quota-audit-offer-create",
	}
	createdOffer, appErr := manager.CreateOfferWithIdempotency(ctx, owner, "quota-offer-create", "offer-key", "offer-hash", offerInput, offerBuilder)
	if appErr != nil {
		t.Fatalf("create quota offer: %v", appErr)
	}
	offerID := createdOffer.ResourceID

	publishBuilder := func(batch apiquota.Batch) (idempotency.Completion, *domain.AppError) {
		return idempotency.Completion{
			Status: http.StatusOK, ContentType: "application/json", Body: []byte(`{"kind":"published"}`),
			ResourceType: "api_quota_batch", ResourceID: batch.ID,
		}, nil
	}
	publishInput := apiquota.BatchActionInput{BatchID: batchID, ExpectedVersion: 1, RequestID: "quota-audit-batch-publish"}
	published, appErr := manager.PublishBatchWithIdempotency(ctx, owner, "quota-batch-publish", "publish-key", "publish-hash", publishInput, publishBuilder)
	if appErr != nil {
		t.Fatalf("publish quota batch: %v", appErr)
	}
	publishReplay, appErr := manager.PublishBatchWithIdempotency(ctx, owner, "quota-batch-publish", "publish-key", "publish-hash", publishInput, publishBuilder)
	if appErr != nil || !equivalentJSONBodies(published.Body, publishReplay.Body) {
		t.Fatalf("replay published batch after state transition: completion=%+v error=%v", publishReplay, appErr)
	}
	// 批次发布后创建规格已不再合法；已完成的创建请求仍必须先命中幂等结果。
	offerReplay, appErr := manager.CreateOfferWithIdempotency(ctx, owner, "quota-offer-create", "offer-key", "offer-hash", offerInput, offerBuilder)
	if appErr != nil || offerReplay.ResourceID != offerID || !equivalentJSONBodies(createdOffer.Body, offerReplay.Body) {
		t.Fatalf("replay offer creation after batch publication: completion=%+v error=%v", offerReplay, appErr)
	}

	failedInput := batchInput
	failedInput.RequestID = "quota-audit-builder-failure"
	_, appErr = manager.CreateBatchWithIdempotency(
		ctx, owner, "quota-batch-create-failure", "failure-key", "failure-hash", failedInput,
		func(apiquota.Batch) (idempotency.Completion, *domain.AppError) {
			return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Encoding failed", "测试响应编码失败。")
		},
	)
	if appErr == nil || appErr.Code != domain.CodeInternalError {
		t.Fatalf("expected completion builder failure, got %#v", appErr)
	}

	var batches, offers, completed, failed int
	if err := pool.QueryRow(ctx, `SELECT count(*)::int FROM api_quota_batches WHERE owner_user_id = $1`, sellerID).Scan(&batches); err != nil {
		t.Fatalf("count batches: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*)::int FROM api_quota_offers WHERE owner_user_id = $1`, sellerID).Scan(&offers); err != nil {
		t.Fatalf("count offers: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*)::int FROM idempotency_keys WHERE user_id = $1 AND status = 'completed'`, sellerID).Scan(&completed); err != nil {
		t.Fatalf("count completed idempotency rows: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*)::int FROM idempotency_keys WHERE user_id = $1 AND status = 'failed'`, sellerID).Scan(&failed); err != nil {
		t.Fatalf("count failed idempotency rows: %v", err)
	}
	if batches != 1 || offers != 1 || completed != 3 || failed != 1 {
		t.Fatalf("atomic row counts batches=%d offers=%d completed=%d failed=%d", batches, offers, completed, failed)
	}

	var batchCreated, batchPublished, offerCreated, offerPublished int
	var metadata string
	if err := pool.QueryRow(ctx, `
		SELECT
		  count(*) FILTER (WHERE event_type = 'api_quota_batch.created')::int,
		  count(*) FILTER (WHERE event_type = 'api_quota_batch.published')::int,
		  count(*) FILTER (WHERE event_type = 'api_quota_offer.created')::int,
		  count(*) FILTER (WHERE event_type = 'api_quota_offer.published')::int,
		  COALESCE(string_agg(metadata_json::text, ' '), '')
		FROM domain_events
		WHERE actor_user_id = $1 AND aggregate_id IN ($2, $3)
	`, sellerID, batchID, offerID).Scan(&batchCreated, &batchPublished, &offerCreated, &offerPublished, &metadata); err != nil {
		t.Fatalf("read quota audit events: %v", err)
	}
	if batchCreated != 1 || batchPublished != 1 || offerCreated != 1 || offerPublished != 1 {
		t.Fatalf("unexpected quota audit counts: batch created=%d published=%d; offer created=%d published=%d", batchCreated, batchPublished, offerCreated, offerPublished)
	}
	if strings.Contains(metadata, batchInput.SourceLabel) {
		t.Fatalf("quota audit metadata leaked source label: %s", metadata)
	}
}

func equivalentJSONBodies(left []byte, right []byte) bool {
	var leftValue any
	var rightValue any
	if err := json.Unmarshal(left, &leftValue); err != nil {
		return false
	}
	if err := json.Unmarshal(right, &rightValue); err != nil {
		return false
	}
	return reflect.DeepEqual(leftValue, rightValue)
}
