package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	app "c2c-market/backend/internal/module/core"
	"c2c-market/backend/internal/store/postgres"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresAdminOfficialPriceRecordFlow(t *testing.T) {
	databaseURL := os.Getenv("C2C_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	store, err := postgres.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer store.Close()
	pool := openTestPool(t, databaseURL)
	defer pool.Close()

	server := NewServer(app.NewServiceWithPersistence(store))
	adminSession := createSession(t, server, "pg-admin", true)

	suffix := time.Now().Format("150405.000000000")
	body := adminRecordPayload("pg-record-"+suffix, "799.00", "0.12210000")
	first := createAdminOfficialPriceRecordWithBody(t, server, adminSession, body, "pg-record-"+suffix)
	second := createAdminOfficialPriceRecordWithBody(t, server, adminSession, body, "pg-record-"+suffix)

	if first.ID == "" || first.ID != second.ID {
		t.Fatalf("expected postgres idempotent replay, got %q and %q", first.ID, second.ID)
	}
	if first.NormalizedMonthlyCNY != "97.56" {
		t.Fatalf("expected normalized price 97.56, got %q", first.NormalizedMonthlyCNY)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/official-prices", nil)
	listResponse := httptest.NewRecorder()
	server.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list public prices status %d body %s", listResponse.Code, listResponse.Body.String())
	}
	if !strings.Contains(listResponse.Body.String(), first.ID) {
		t.Fatalf("expected approved record in public price list")
	}

	detailRequest := httptest.NewRequest(http.MethodGet, "/api/v1/official-prices/"+first.ID, nil)
	detailResponse := httptest.NewRecorder()
	server.ServeHTTP(detailResponse, detailRequest)
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("get public price status %d body %s", detailResponse.Code, detailResponse.Body.String())
	}
	if !strings.Contains(detailResponse.Body.String(), `"normalizedMonthlyCny":"97.56"`) {
		t.Fatalf("expected normalized price in public detail, got %s", detailResponse.Body.String())
	}

	assertOfficialPriceRecordAdminSideEffects(t, databaseURL, first.ID, "official_price_record.created", "official_price_record.create", 1)
}

func TestPostgresProductCatalogReadAPIs(t *testing.T) {
	databaseURL := os.Getenv("C2C_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	store, err := postgres.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer store.Close()

	server := NewServer(app.NewServiceWithPersistence(store))

	categoryRequest := httptest.NewRequest(http.MethodGet, "/api/v1/product-categories", nil)
	categoryResponse := httptest.NewRecorder()
	server.ServeHTTP(categoryResponse, categoryRequest)
	if categoryResponse.Code != http.StatusOK {
		t.Fatalf("list categories status %d body %s", categoryResponse.Code, categoryResponse.Body.String())
	}
	if !strings.Contains(categoryResponse.Body.String(), `"code":"gpt"`) || !strings.Contains(categoryResponse.Body.String(), `"code":"claude"`) {
		t.Fatalf("expected seeded categories, got %s", categoryResponse.Body.String())
	}

	plansRequest := httptest.NewRequest(http.MethodGet, "/api/v1/product-plans?category=gpt", nil)
	plansResponse := httptest.NewRecorder()
	server.ServeHTTP(plansResponse, plansRequest)
	if plansResponse.Code != http.StatusOK {
		t.Fatalf("list plans status %d body %s", plansResponse.Code, plansResponse.Body.String())
	}
	body := plansResponse.Body.String()
	if !strings.Contains(body, `"slug":"chatgpt-pro-20x-web"`) {
		t.Fatalf("expected GPT plans, got %s", body)
	}
	if !strings.Contains(body, `"publishPolicy":"allowed"`) || !strings.Contains(body, `"riskAckRequired":true`) {
		t.Fatalf("expected policy fields in plans, got %s", body)
	}

	detailRequest := httptest.NewRequest(http.MethodGet, "/api/v1/product-plans/00000000-0000-0000-0000-000000000303", nil)
	detailResponse := httptest.NewRecorder()
	server.ServeHTTP(detailResponse, detailRequest)
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("get plan status %d body %s", detailResponse.Code, detailResponse.Body.String())
	}
	if !strings.Contains(detailResponse.Body.String(), `"accessMode":"personal_account_cost_share"`) {
		t.Fatalf("expected plan detail policy fields, got %s", detailResponse.Body.String())
	}
}

func TestPostgresAPIServiceFlow(t *testing.T) {
	databaseURL := os.Getenv("C2C_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	store, err := postgres.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer store.Close()

	server := NewServer(app.NewServiceWithPersistence(store))
	suffix := time.Now().Format("150405.000000000")
	ownerSession := createLinuxDoSession(t, server, "pg-api-owner-"+suffix)
	adminSession := createSession(t, server, "pg-api-admin-"+suffix, true)
	ownerContact := createContactMethod(t, server, ownerSession, "telegram", "PG API Owner "+suffix, "@pg_api_owner_"+suffix)

	service := createAPIService(t, server, ownerSession, ownerContact.ID, "pg-api-create-"+suffix)
	if service.ReviewStatus != app.APIServiceReviewStatusDraft || service.Version != 1 {
		t.Fatalf("unexpected postgres API service draft: %+v", service)
	}
	if service.SourceURL != "https://linux.do/t/api-service/123" {
		t.Fatalf("expected postgres API service source URL round trip, got %+v", service)
	}
	assertAPIServiceChildren(t, databaseURL, service.ID, 1, 1, 0)

	updated := updateAPIService(t, server, ownerSession, service.ID, service.Version, apiServicePayloadWithModel(ownerContact.ID, "00000000-0000-0000-0000-000000000a02"), "pg-api-update-"+suffix)
	if updated.Version != 2 || len(updated.Models) != 1 || updated.Models[0].ModelCatalogID != "00000000-0000-0000-0000-000000000a02" {
		t.Fatalf("unexpected updated API service: %+v", updated)
	}

	submitted := ownerAPIServiceAction(t, server, ownerSession, updated.ID, "submit-review", updated.Version, "pg-api-submit-"+suffix)
	if submitted.ReviewStatus != app.APIServiceReviewStatusApproved || submitted.PublicationStatus != app.APIServicePublicationStatusOffline {
		t.Fatalf("expected auto-approved offline API service, got %+v", submitted)
	}
	published := ownerAPIServiceAction(t, server, ownerSession, submitted.ID, "publish", submitted.Version, "pg-api-publish-"+suffix)
	publishedReplay := ownerAPIServiceAction(t, server, ownerSession, submitted.ID, "publish", submitted.Version, "pg-api-publish-"+suffix)
	if published.ID != publishedReplay.ID || published.Version != publishedReplay.Version {
		t.Fatalf("expected publish idempotent replay, got %+v and %+v", published, publishedReplay)
	}
	if published.PublicationStatus != app.APIServicePublicationStatusOnline || published.ModerationStatus != app.APIServiceModerationStatusClear {
		t.Fatalf("unexpected published API service: %+v", published)
	}
	assertPublicAPIServiceVisible(t, server, published.ID, ownerContact.ID, true)
	assertAPIServicePublicPredicateCount(t, databaseURL, published.ID, 1)

	reviewService := createAPIService(t, server, ownerSession, ownerContact.ID, "pg-api-manual-review-create-"+suffix)
	forcedPendingVersion := forceAPIServicePendingReview(t, databaseURL, reviewService.ID)
	changesRequested := adminAPIServiceAction(t, server, adminSession, reviewService.ID, "request-changes", forcedPendingVersion, "pg-api-request-changes-"+suffix)
	if changesRequested.ReviewStatus != app.APIServiceReviewStatusChangesRequested {
		t.Fatalf("unexpected changes-requested API service: %+v", changesRequested)
	}
	resubmitted := ownerAPIServiceAction(t, server, ownerSession, changesRequested.ID, "submit-review", changesRequested.Version, "pg-api-resubmit-"+suffix)
	if resubmitted.ReviewStatus != app.APIServiceReviewStatusApproved || resubmitted.PublicationStatus != app.APIServicePublicationStatusOffline {
		t.Fatalf("unexpected resubmitted API service: %+v", resubmitted)
	}

	paused := ownerAPIServiceAction(t, server, ownerSession, published.ID, "pause", published.Version, "pg-api-pause-"+suffix)
	if paused.PublicationStatus != app.APIServicePublicationStatusOwnerPaused {
		t.Fatalf("unexpected paused API service: %+v", paused)
	}
	assertPublicAPIServiceVisible(t, server, published.ID, ownerContact.ID, false)

	resumed := ownerAPIServiceAction(t, server, ownerSession, paused.ID, "resume", paused.Version, "pg-api-resume-"+suffix)
	if resumed.PublicationStatus != app.APIServicePublicationStatusOnline {
		t.Fatalf("unexpected resumed API service: %+v", resumed)
	}
	assertPublicAPIServiceVisible(t, server, resumed.ID, ownerContact.ID, true)

	suspended := adminAPIServiceAction(t, server, adminSession, resumed.ID, "suspend", resumed.Version, "pg-api-suspend-"+suffix)
	if suspended.ModerationStatus != app.APIServiceModerationStatusAdminSuspended {
		t.Fatalf("unexpected suspended API service: %+v", suspended)
	}
	assertPublicAPIServiceVisible(t, server, resumed.ID, ownerContact.ID, false)

	restored := adminAPIServiceAction(t, server, adminSession, suspended.ID, "restore", suspended.Version, "pg-api-restore-"+suffix)
	if restored.ModerationStatus != app.APIServiceModerationStatusClear {
		t.Fatalf("unexpected restored API service: %+v", restored)
	}
	assertPublicAPIServiceVisible(t, server, restored.ID, ownerContact.ID, true)

	removed := adminAPIServiceAction(t, server, adminSession, restored.ID, "remove", restored.Version, "pg-api-remove-"+suffix)
	if removed.ModerationStatus != app.APIServiceModerationStatusRemoved || removed.PublicationStatus != app.APIServicePublicationStatusArchived {
		t.Fatalf("unexpected removed API service: %+v", removed)
	}
	assertPublicAPIServiceVisible(t, server, removed.ID, ownerContact.ID, false)
	assertAPIServiceIdempotencyCache(t, databaseURL, ownerSession.userID, published.ID, "publish", "pg-api-publish-"+suffix, app.APIServicePublicationStatusOnline)

	rejectedService := createAPIService(t, server, ownerSession, ownerContact.ID, "pg-api-reject-create-"+suffix)
	forcedRejectedVersion := forceAPIServicePendingReview(t, databaseURL, rejectedService.ID)
	rejected := adminAPIServiceAction(t, server, adminSession, rejectedService.ID, "reject", forcedRejectedVersion, "pg-api-reject-"+suffix)
	if rejected.ReviewStatus != app.APIServiceReviewStatusRejected || rejected.PublicationStatus != app.APIServicePublicationStatusOffline {
		t.Fatalf("unexpected rejected API service: %+v", rejected)
	}
	assertPublicAPIServiceVisible(t, server, rejected.ID, ownerContact.ID, false)

	revisionService := createAPIService(t, server, ownerSession, ownerContact.ID, "pg-api-revision-create-"+suffix)
	revisionSubmitted := ownerAPIServiceAction(t, server, ownerSession, revisionService.ID, "submit-review", revisionService.Version, "pg-api-revision-submit-"+suffix)
	revisionPublished := ownerAPIServiceAction(t, server, ownerSession, revisionSubmitted.ID, "publish", revisionSubmitted.Version, "pg-api-revision-publish-"+suffix)
	revision := ownerAPIServiceAction(t, server, ownerSession, revisionPublished.ID, "start-revision", revisionPublished.Version, "pg-api-start-revision-"+suffix)
	if revision.ReviewStatus != app.APIServiceReviewStatusChangesRequested || revision.PublicationStatus != app.APIServicePublicationStatusOffline {
		t.Fatalf("unexpected API service revision state: %+v", revision)
	}
	assertPublicAPIServiceVisible(t, server, revision.ID, ownerContact.ID, false)
}

func TestPostgresAPIServiceIntegrityConstraints(t *testing.T) {
	databaseURL := os.Getenv("C2C_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	store, err := postgres.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer store.Close()

	server := NewServer(app.NewServiceWithPersistence(store))
	suffix := time.Now().Format("150405.000000000")
	ownerSession := createSession(t, server, "pg-api-integrity-owner-"+suffix, false)
	otherSession := createSession(t, server, "pg-api-integrity-other-"+suffix, false)
	ownerContact := createContactMethod(t, server, ownerSession, "telegram", "PG API Integrity Owner "+suffix, "@pg_api_integrity_owner_"+suffix)
	otherContact := createContactMethod(t, server, otherSession, "telegram", "PG API Integrity Other "+suffix, "@pg_api_integrity_other_"+suffix)

	service := createAPIService(t, server, ownerSession, ownerContact.ID, "pg-api-integrity-create-"+suffix)
	pool := openTestPool(t, databaseURL)
	defer pool.Close()

	_, err = pool.Exec(ctx, `
		INSERT INTO api_service_models (
			api_service_id, distribution_system, model_catalog_id, model_name_snapshot,
			provider_snapshot, capabilities_snapshot, merchant_multiplier, enabled
		)
		VALUES ($1, 'sub2api', '00000000-0000-0000-0000-000000000a02',
		        'GPT-4.1 mini', 'OpenAI', ARRAY['text'], 1.2000, true)
	`, service.ID)
	assertPostgresConstraintError(t, err, "Sub2API model multiplier must be fixed to 1")

	_, err = pool.Exec(ctx, `
		UPDATE api_services
		SET owner_contact_method_id = $2
		WHERE id = $1
	`, service.ID, otherContact.ID)
	assertPostgresConstraintError(t, err, "API service contact method must belong to owner")
}

func TestPostgresAPIPurchaseIntentFlow(t *testing.T) {
	databaseURL := os.Getenv("C2C_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	store, err := postgres.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer store.Close()

	server := NewServer(app.NewServiceWithPersistence(store))
	suffix := time.Now().Format("150405.000000000")
	ownerSession := createLinuxDoSession(t, server, "pg-api-intent-owner-"+suffix)
	buyerSession := createSession(t, server, "pg-api-intent-buyer-"+suffix, false)
	adminSession := createSession(t, server, "pg-api-intent-admin-"+suffix, true)
	ownerContact := createContactMethod(t, server, ownerSession, "telegram", "PG API Intent Owner "+suffix, "@pg_api_intent_owner_"+suffix)
	buyerContact := createContactMethod(t, server, buyerSession, "telegram", "PG API Intent Buyer "+suffix, "@pg_api_intent_buyer_"+suffix)

	service := createAPIService(t, server, ownerSession, ownerContact.ID, "pg-api-intent-service-create-"+suffix)
	submitted := ownerAPIServiceAction(t, server, ownerSession, service.ID, "submit-review", service.Version, "pg-api-intent-service-submit-"+suffix)
	published := ownerAPIServiceAction(t, server, ownerSession, submitted.ID, "publish", submitted.Version, "pg-api-intent-service-publish-"+suffix)

	first := createAPIPurchaseIntent(t, server, buyerSession, published.ID, buyerContact.ID, "pg-api-intent-create-"+suffix)
	second := createAPIPurchaseIntent(t, server, buyerSession, published.ID, buyerContact.ID, "pg-api-intent-create-"+suffix)
	if first.ID == "" || first.ID != second.ID {
		t.Fatalf("expected idempotent API purchase intent replay, got %+v and %+v", first, second)
	}
	if first.Status != app.APIPurchaseIntentStatusOpen ||
		first.RequestedCNYAmount != "16.00" ||
		first.RequestedUSDAllowance != "20.000000" ||
		first.DeclaredCNYPerUSDAllowanceSnapshot != "0.8000" ||
		first.DeclaredMaxUSDAllowancePerIntentSnapshot != "20.000000" ||
		first.PricingSnapshot == "" {
		t.Fatalf("unexpected postgres API purchase intent: %+v", first)
	}
	if first.MerchantContact == nil || first.MerchantContact.Value != "@pg_api_intent_owner_"+suffix {
		t.Fatalf("expected merchant contact in postgres API intent create: %+v", first.MerchantContact)
	}
	if first.SelectedAccessMode != "buyer_dedicated_sub_key" || first.OwnerUserID != "" || first.OwnerContactMethodID != "" || first.MerchantContact.Type != "telegram" {
		t.Fatalf("postgres create response leaked owner identity or missed frozen access/contact data: %+v", first)
	}
	if first.BuyerContact != nil {
		t.Fatalf("create response must not include buyer contact: %+v", first.BuyerContact)
	}
	assertAPIPurchaseIntentSideEffects(t, databaseURL, first.ID, 1)
	assertAPIPurchaseIntentIdempotencyCache(t, databaseURL, buyerSession.userID, published.ID, first.ID, "create", "pg-api-intent-create-"+suffix, app.APIPurchaseIntentStatusOpen, "@pg_api_intent_owner_"+suffix)

	buyerDetail := getAPIPurchaseIntent(t, server, buyerSession, "me", first.ID)
	if buyerDetail.ID != first.ID || buyerDetail.MerchantContact == nil || buyerDetail.MerchantContact.Value != "@pg_api_intent_owner_"+suffix {
		t.Fatalf("unexpected buyer API purchase intent detail: %+v", buyerDetail)
	}
	ownerDetail := getAPIPurchaseIntent(t, server, ownerSession, "owner", first.ID)
	if ownerDetail.ID != first.ID || ownerDetail.BuyerContactMethodID != buyerContact.ID || ownerDetail.OwnerContactMethodID != "" || ownerDetail.BuyerContact == nil || ownerDetail.BuyerContact.Value != "@pg_api_intent_buyer_"+suffix {
		t.Fatalf("unexpected owner API purchase intent detail: %+v", ownerDetail)
	}
	assertAPIPurchaseIntentContactAccessLogs(t, databaseURL, first.ID, 1, 1)
	if _, err := poolExec(databaseURL, `
		UPDATE contact_methods
		SET label = label || ' changed after intent'
		WHERE id IN ($1, $2)
	`, ownerContact.ID, buyerContact.ID); err != nil {
		t.Fatalf("mutate contact labels after API intent: %v", err)
	}
	labelReplay := createAPIPurchaseIntent(t, server, buyerSession, published.ID, buyerContact.ID, "pg-api-intent-create-"+suffix)
	labelBuyerDetail := getAPIPurchaseIntent(t, server, buyerSession, "me", first.ID)
	labelOwnerDetail := getAPIPurchaseIntent(t, server, ownerSession, "owner", first.ID)
	if labelReplay.MerchantContact.Label != first.MerchantContact.Label || labelBuyerDetail.MerchantContact.Label != first.MerchantContact.Label {
		t.Fatalf("merchant contact label drifted after mutable method edit: create=%+v replay=%+v detail=%+v", first.MerchantContact, labelReplay.MerchantContact, labelBuyerDetail.MerchantContact)
	}
	if labelOwnerDetail.BuyerContact.Label != ownerDetail.BuyerContact.Label {
		t.Fatalf("buyer contact label drifted after mutable method edit: before=%+v after=%+v", ownerDetail.BuyerContact, labelOwnerDetail.BuyerContact)
	}
	adminDetail := getAPIPurchaseIntent(t, server, adminSession, "admin", first.ID)
	if adminDetail.ID != first.ID || adminDetail.MerchantContact != nil || adminDetail.BuyerContact != nil {
		t.Fatalf("unexpected admin API purchase intent detail: %+v", adminDetail)
	}

	contacted := ownerAPIPurchaseIntentAction(t, server, ownerSession, first.ID, "mark-contacted", first.Version, "pg-api-intent-contacted-"+suffix, `{}`)
	contactedReplay := ownerAPIPurchaseIntentAction(t, server, ownerSession, first.ID, "mark-contacted", first.Version, "pg-api-intent-contacted-"+suffix, `{}`)
	if contacted.Status != app.APIPurchaseIntentStatusContacted || contacted.ContactedAt == nil || contacted.Version != contactedReplay.Version {
		t.Fatalf("unexpected contacted API purchase intent: %+v replay %+v", contacted, contactedReplay)
	}
	assertAPIPurchaseIntentActionSideEffects(t, databaseURL, first.ID, app.APIPurchaseIntentStatusContacted, "api_purchase_intent.contacted", 1)
	assertAPIPurchaseIntentIdempotencyCache(t, databaseURL, ownerSession.userID, first.ID, first.ID, "mark-contacted", "pg-api-intent-contacted-"+suffix, app.APIPurchaseIntentStatusContacted, "")

	closed := ownerAPIPurchaseIntentAction(t, server, ownerSession, first.ID, "close", contacted.Version, "pg-api-intent-close-"+suffix, `{"reason":"双方已站外完成确认，关闭本次意向。"}`)
	if closed.Status != app.APIPurchaseIntentStatusOwnerClosed || closed.OwnerClosedAt == nil || closed.OwnerCloseReason == "" {
		t.Fatalf("unexpected closed API purchase intent: %+v", closed)
	}
	assertAPIPurchaseIntentActionSideEffects(t, databaseURL, first.ID, app.APIPurchaseIntentStatusOwnerClosed, "api_purchase_intent.owner_closed", 1)

	secondIntent := createAPIPurchaseIntent(t, server, buyerSession, published.ID, buyerContact.ID, "pg-api-intent-create-after-close-"+suffix)
	pausedService := ownerAPIServiceAction(t, server, ownerSession, published.ID, "pause", published.Version, "pg-api-intent-service-pause-"+suffix)
	if pausedService.PublicationStatus != app.APIServicePublicationStatusOwnerPaused {
		t.Fatalf("expected service paused, got %+v", pausedService)
	}
	pausedReplay := createAPIPurchaseIntent(t, server, buyerSession, published.ID, buyerContact.ID, "pg-api-intent-create-after-close-"+suffix)
	if pausedReplay.ID != secondIntent.ID || pausedReplay.MerchantContact == nil || pausedReplay.MerchantContact.Value != secondIntent.MerchantContact.Value {
		t.Fatalf("idempotent replay after service pause lost frozen contact: %+v", pausedReplay)
	}
}

func TestPostgresAPIOrderReleasesPurchaseIntent(t *testing.T) {
	databaseURL := os.Getenv("C2C_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	store, err := postgres.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer store.Close()
	pool := openTestPool(t, databaseURL)
	defer pool.Close()

	server := NewServer(app.NewServiceWithPersistence(store))
	suffix := time.Now().Format("150405.000000000")
	ownerSession := createLinuxDoSession(t, server, "pg-api-order-release-owner-"+suffix)
	buyerSession := createSession(t, server, "pg-api-order-release-buyer-"+suffix, false)
	ownerContact := createContactMethod(t, server, ownerSession, "telegram", "PG API Order Release Owner "+suffix, "@pg_api_order_release_owner_"+suffix)
	buyerContact := createContactMethod(t, server, buyerSession, "telegram", "PG API Order Release Buyer "+suffix, "@pg_api_order_release_buyer_"+suffix)

	service := createAPIService(t, server, ownerSession, ownerContact.ID, "pg-api-order-release-service-create-"+suffix)
	submitted := ownerAPIServiceAction(t, server, ownerSession, service.ID, "submit-review", service.Version, "pg-api-order-release-service-submit-"+suffix)
	published := ownerAPIServiceAction(t, server, ownerSession, submitted.ID, "publish", submitted.Version, "pg-api-order-release-service-publish-"+suffix)
	orderable := updateAPIServiceOrderSettings(t, server, ownerSession, published.ID, published.Version, true, "pg-api-order-release-service-settings-"+suffix)

	firstIntent := createAPIPurchaseIntent(t, server, buyerSession, orderable.ID, buyerContact.ID, "pg-api-order-release-intent-one-"+suffix)
	firstOrder := createAPIOrder(t, server, buyerSession, firstIntent.ID, "wechat", "pg-api-order-release-order-one-"+suffix)
	if firstOrder.RequestedUSDAllowanceSnapshot != "20.000000" || firstOrder.CNYPerUSDAllowanceSnapshot != "0.8000" || firstOrder.PricingSnapshot == "" {
		t.Fatalf("expected frozen quota pricing on first order, got %+v", firstOrder)
	}
	var availableAfterFirst string
	if err := pool.QueryRow(ctx, `SELECT available_usd_allowance::text FROM api_services WHERE id = $1`, orderable.ID).Scan(&availableAfterFirst); err != nil {
		t.Fatalf("read available allowance after first order: %v", err)
	}
	if availableAfterFirst != "80.000000" {
		t.Fatalf("expected 80.000000 allowance after first reservation, got %s", availableAfterFirst)
	}
	firstOrderedIntent := getAPIPurchaseIntent(t, server, buyerSession, "me", firstIntent.ID)
	if firstOrderedIntent.Status != app.APIPurchaseIntentStatusOrdered || firstOrderedIntent.Version != firstIntent.Version+1 {
		t.Fatalf("expected first order to release purchase intent, got %+v", firstOrderedIntent)
	}

	secondIntent := createAPIPurchaseIntent(t, server, buyerSession, orderable.ID, buyerContact.ID, "pg-api-order-release-intent-two-"+suffix)
	if secondIntent.ID == firstIntent.ID || secondIntent.Status != app.APIPurchaseIntentStatusOpen {
		t.Fatalf("expected a new active intent after first order, got first=%+v second=%+v", firstIntent, secondIntent)
	}
	secondOrder := createAPIOrder(t, server, buyerSession, secondIntent.ID, "wechat", "pg-api-order-release-order-two-"+suffix)
	if firstOrder.ID == secondOrder.ID || secondOrder.APIPurchaseIntentID != secondIntent.ID {
		t.Fatalf("expected a second order for the new intent, got first=%+v second=%+v", firstOrder, secondOrder)
	}
	secondOrderedIntent := getAPIPurchaseIntent(t, server, buyerSession, "me", secondIntent.ID)
	if secondOrderedIntent.Status != app.APIPurchaseIntentStatusOrdered {
		t.Fatalf("expected second order to release purchase intent, got %+v", secondOrderedIntent)
	}
	var availableAfterSecond string
	if err := pool.QueryRow(ctx, `SELECT available_usd_allowance::text FROM api_services WHERE id = $1`, orderable.ID).Scan(&availableAfterSecond); err != nil {
		t.Fatalf("read available allowance after second order: %v", err)
	}
	if availableAfterSecond != "60.000000" {
		t.Fatalf("expected 60.000000 allowance after two reservations, got %s", availableAfterSecond)
	}

	cancelled := apiOrderAction(t, server, buyerSession, "me", secondOrder.ID, "cancel", secondOrder.Version, "pg-api-order-release-cancel-two-"+suffix, `{"reason":"取消未付款订单以验证额度释放。"}`)
	if cancelled.Status != "cancelled" {
		t.Fatalf("expected second order cancelled, got %+v", cancelled)
	}
	var availableAfterCancellation string
	if err := pool.QueryRow(ctx, `SELECT available_usd_allowance::text FROM api_services WHERE id = $1`, orderable.ID).Scan(&availableAfterCancellation); err != nil {
		t.Fatalf("read available allowance after cancellation: %v", err)
	}
	if availableAfterCancellation != "80.000000" {
		t.Fatalf("expected 80.000000 allowance after cancellation release, got %s", availableAfterCancellation)
	}
}

func TestPostgresAPIPurchaseIntentIntegrityConstraints(t *testing.T) {
	databaseURL := os.Getenv("C2C_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	store, err := postgres.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer store.Close()

	server := NewServer(app.NewServiceWithPersistence(store))
	suffix := time.Now().Format("150405.000000000")
	ownerSession := createLinuxDoSession(t, server, "pg-api-intent-integrity-owner-"+suffix)
	buyerSession := createSession(t, server, "pg-api-intent-integrity-buyer-"+suffix, false)
	otherSession := createSession(t, server, "pg-api-intent-integrity-other-"+suffix, false)
	ownerContact := createContactMethod(t, server, ownerSession, "telegram", "PG API Intent Integrity Owner "+suffix, "@pg_api_intent_integrity_owner_"+suffix)
	buyerContact := createContactMethod(t, server, buyerSession, "telegram", "PG API Intent Integrity Buyer "+suffix, "@pg_api_intent_integrity_buyer_"+suffix)
	otherContact := createContactMethod(t, server, otherSession, "telegram", "PG API Intent Integrity Other "+suffix, "@pg_api_intent_integrity_other_"+suffix)

	service := createAPIService(t, server, ownerSession, ownerContact.ID, "pg-api-intent-integrity-service-create-"+suffix)
	pausedIntent := newJSONRequest(http.MethodPost, "/api/v1/api-services/"+service.ID+"/purchase-intents", apiPurchaseIntentPayload(buyerContact.ID))
	addAuth(pausedIntent, buyerSession, "pg-api-intent-paused-"+suffix)
	pausedResponse := httptest.NewRecorder()
	server.ServeHTTP(pausedResponse, pausedIntent)
	if pausedResponse.Code != http.StatusNotFound {
		t.Fatalf("expected draft service to reject purchase intent as not found, got %d body %s", pausedResponse.Code, pausedResponse.Body.String())
	}

	submitted := ownerAPIServiceAction(t, server, ownerSession, service.ID, "submit-review", service.Version, "pg-api-intent-integrity-submit-"+suffix)
	published := ownerAPIServiceAction(t, server, ownerSession, submitted.ID, "publish", submitted.Version, "pg-api-intent-integrity-publish-"+suffix)

	selfIntent := newJSONRequest(http.MethodPost, "/api/v1/api-services/"+published.ID+"/purchase-intents", apiPurchaseIntentPayload(ownerContact.ID))
	addAuth(selfIntent, ownerSession, "pg-api-intent-self-"+suffix)
	selfResponse := httptest.NewRecorder()
	server.ServeHTTP(selfResponse, selfIntent)
	if selfResponse.Code != http.StatusConflict {
		t.Fatalf("expected owner self-intent conflict, got %d body %s", selfResponse.Code, selfResponse.Body.String())
	}
	assertProblemCode(t, selfResponse, "INVALID_STATE_TRANSITION")

	wrongContact := newJSONRequest(http.MethodPost, "/api/v1/api-services/"+published.ID+"/purchase-intents", apiPurchaseIntentPayload(otherContact.ID))
	addAuth(wrongContact, buyerSession, "pg-api-intent-wrong-contact-"+suffix)
	wrongContactResponse := httptest.NewRecorder()
	server.ServeHTTP(wrongContactResponse, wrongContact)
	if wrongContactResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected wrong buyer contact validation error, got %d body %s", wrongContactResponse.Code, wrongContactResponse.Body.String())
	}
	assertProblemCode(t, wrongContactResponse, "CONTACT_METHOD_NOT_OWNED")

	tooHigh := newJSONRequest(http.MethodPost, "/api/v1/api-services/"+published.ID+"/purchase-intents", strings.Replace(apiPurchaseIntentPayload(buyerContact.ID), `"requestedUsdAllowance":"20.000000"`, `"requestedUsdAllowance":"21.000000"`, 1))
	addAuth(tooHigh, buyerSession, "pg-api-intent-too-high-"+suffix)
	tooHighResponse := httptest.NewRecorder()
	server.ServeHTTP(tooHighResponse, tooHigh)
	if tooHighResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected too-high allowance validation error, got %d body %s", tooHighResponse.Code, tooHighResponse.Body.String())
	}

	active := createAPIPurchaseIntent(t, server, buyerSession, published.ID, buyerContact.ID, "pg-api-intent-active-"+suffix)
	duplicate := newJSONRequest(http.MethodPost, "/api/v1/api-services/"+published.ID+"/purchase-intents", apiPurchaseIntentPayload(buyerContact.ID))
	addAuth(duplicate, buyerSession, "pg-api-intent-active-duplicate-"+suffix)
	duplicateResponse := httptest.NewRecorder()
	server.ServeHTTP(duplicateResponse, duplicate)
	if duplicateResponse.Code != http.StatusConflict {
		t.Fatalf("expected duplicate active API intent conflict, got %d body %s active %+v", duplicateResponse.Code, duplicateResponse.Body.String(), active)
	}
	assertProblemCode(t, duplicateResponse, "ACTIVE_API_INTENT_EXISTS")

	pool := openTestPool(t, databaseURL)
	defer pool.Close()
	now := time.Now().UTC()
	err = insertRawAPIPurchaseIntent(t, pool, published.ID, published.Version, ownerSession.userID, otherSession.userID, ownerContact.ID, buyerContact.ID, now)
	assertPostgresConstraintError(t, err, "API purchase intent buyer contact owner mismatch must be rejected")

	err = insertRawAPIPurchaseIntent(t, pool, published.ID, published.Version, ownerSession.userID, otherSession.userID, otherContact.ID, otherContact.ID, now)
	assertPostgresConstraintError(t, err, "API purchase intent owner contact owner mismatch must be rejected")

	otherSecondContact := createContactMethod(t, server, otherSession, "telegram", "PG API Intent Integrity Other Alt "+suffix, "@pg_api_intent_integrity_other_alt_"+suffix)
	ownerSecondContact := createContactMethod(t, server, ownerSession, "telegram", "PG API Intent Integrity Owner Alt "+suffix, "@pg_api_intent_integrity_owner_alt_"+suffix)
	err = insertRawAPIPurchaseIntentWithVersionOverride(t, pool, published.ID, published.Version, ownerSession.userID, otherSession.userID, ownerContact.ID, otherContact.ID, currentContactVersionID(t, pool, otherSecondContact.ID), currentContactVersionID(t, pool, ownerContact.ID), now)
	assertPostgresConstraintError(t, err, "API purchase intent buyer method/version mismatch must be rejected")
	err = insertRawAPIPurchaseIntentWithVersionOverride(t, pool, published.ID, published.Version, ownerSession.userID, otherSession.userID, ownerContact.ID, otherContact.ID, currentContactVersionID(t, pool, otherContact.ID), currentContactVersionID(t, pool, ownerSecondContact.ID), now)
	assertPostgresConstraintError(t, err, "API purchase intent owner method/version mismatch must be rejected")

	secondOwnerSession := createLinuxDoSession(t, server, "pg-api-intent-integrity-owner-two-"+suffix)
	secondOwnerContact := createContactMethod(t, server, secondOwnerSession, "telegram", "PG API Intent Integrity Owner Two "+suffix, "@pg_api_intent_integrity_owner_two_"+suffix)
	secondService := createAPIService(t, server, secondOwnerSession, secondOwnerContact.ID, "pg-api-intent-integrity-second-service-create-"+suffix)
	secondSubmitted := ownerAPIServiceAction(t, server, secondOwnerSession, secondService.ID, "submit-review", secondService.Version, "pg-api-intent-integrity-second-submit-"+suffix)
	secondPublished := ownerAPIServiceAction(t, server, secondOwnerSession, secondSubmitted.ID, "publish", secondSubmitted.Version, "pg-api-intent-integrity-second-publish-"+suffix)
	reusedKey := newJSONRequest(http.MethodPost, "/api/v1/api-services/"+secondPublished.ID+"/purchase-intents", apiPurchaseIntentPayload(buyerContact.ID))
	addAuth(reusedKey, buyerSession, "pg-api-intent-active-"+suffix)
	reusedKeyResponse := httptest.NewRecorder()
	server.ServeHTTP(reusedKeyResponse, reusedKey)
	if reusedKeyResponse.Code != http.StatusConflict {
		t.Fatalf("expected same API intent idempotency key across services to conflict, got %d body %s", reusedKeyResponse.Code, reusedKeyResponse.Body.String())
	}
	assertProblemCode(t, reusedKeyResponse, "IDEMPOTENCY_KEY_REUSED")
}

func TestPostgresIdempotencyProcessingReplay(t *testing.T) {
	databaseURL := os.Getenv("C2C_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	store, err := postgres.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer store.Close()

	now := time.Now().UTC()
	user, appErr := store.EnsureUser(ctx, "pg-idem-"+now.Format("150405.000000000"), false, now)
	if appErr != nil {
		t.Fatalf("ensure user: %v", appErr)
	}
	entry := app.IdempotencyEntry{
		UserID:      user.ID,
		RouteKey:    "POST /api/v1/test-processing",
		Key:         "processing-" + now.Format("150405.000000000"),
		RequestHash: "same-request-hash",
		State:       "processing",
		CreatedAt:   now,
		ExpiresAt:   now.Add(24 * time.Hour),
	}
	if _, appErr := store.BeginIdempotency(ctx, entry); appErr != nil {
		t.Fatalf("begin first idempotency entry: %v", appErr)
	}
	if _, appErr := store.BeginIdempotency(ctx, entry); appErr == nil || appErr.Code != domain.CodeIdempotencyInProgress {
		t.Fatalf("expected %s, got %#v", domain.CodeIdempotencyInProgress, appErr)
	}
}

func TestPostgresContactSessionFlow(t *testing.T) {
	databaseURL := os.Getenv("C2C_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	store, err := postgres.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer store.Close()

	server := NewServer(app.NewServiceWithPersistence(store))
	suffix := time.Now().Format("150405.000000000")
	buyerSession := createSession(t, server, "pg-contact-buyer-"+suffix, false)
	sellerSession := createSession(t, server, "pg-contact-seller-"+suffix, false)
	buyerContact := createContactMethod(t, server, buyerSession, "telegram", "Buyer PG TG "+suffix, "@pg_buyer_"+suffix)
	sellerContact := createContactMethod(t, server, sellerSession, "telegram", "Seller PG TG "+suffix, "@pg_seller_"+suffix)

	request := newJSONRequest(http.MethodPost, "/api/v1/dev/contact-sessions", `{
		"sellerUsername":"pg-contact-seller-`+suffix+`",
		"buyerContactMethodId":"`+buyerContact.ID+`",
		"sellerContactMethodId":"`+sellerContact.ID+`",
		"durationSeconds":1
	}`)
	addAuth(request, buyerSession, "pg-contact-session-"+suffix)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create contact session status %d body %s", response.Code, response.Body.String())
	}
	var created contactSessionResponse
	if err := json.NewDecoder(response.Body).Decode(&created); err != nil {
		t.Fatalf("decode contact session: %v", err)
	}

	read := httptest.NewRequest(http.MethodGet, "/api/v1/contact-sessions/"+created.ID+"/contacts", nil)
	addCookie(read, buyerSession.cookie)
	readResponse := httptest.NewRecorder()
	server.ServeHTTP(readResponse, read)
	if readResponse.Code != http.StatusOK {
		t.Fatalf("read contact status %d body %s", readResponse.Code, readResponse.Body.String())
	}
	if got := readResponse.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected no-store, got %q", got)
	}
	if !strings.Contains(readResponse.Body.String(), "@pg_seller_"+suffix) {
		t.Fatalf("expected seller contact in participant response")
	}
	if count := storeAccessLogCount(t, store, created.ID); count != 1 {
		t.Fatalf("expected one contact access log, got %d", count)
	}
	assertContactCiphertextDoesNotContain(t, databaseURL, "@pg_seller_"+suffix)

	time.Sleep(1100 * time.Millisecond)
	expired := httptest.NewRequest(http.MethodGet, "/api/v1/contact-sessions/"+created.ID+"/contacts", nil)
	addCookie(expired, buyerSession.cookie)
	expiredResponse := httptest.NewRecorder()
	server.ServeHTTP(expiredResponse, expired)
	if expiredResponse.Code != http.StatusConflict {
		t.Fatalf("expected expired contact status %d, got %d body %s", http.StatusConflict, expiredResponse.Code, expiredResponse.Body.String())
	}
	if strings.Contains(expiredResponse.Body.String(), "@pg_seller_"+suffix) {
		t.Fatalf("expired response must not include full contact value")
	}
}

func TestPostgresContactIntegrityConstraints(t *testing.T) {
	databaseURL := os.Getenv("C2C_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	store, err := postgres.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer store.Close()

	server := NewServer(app.NewServiceWithPersistence(store))
	suffix := time.Now().Format("150405.000000000")
	userSession := createSession(t, server, "pg-integrity-user-"+suffix, false)
	otherSession := createSession(t, server, "pg-integrity-other-"+suffix, false)
	buyerSession := createSession(t, server, "pg-integrity-buyer-"+suffix, false)
	sellerSession := createSession(t, server, "pg-integrity-seller-"+suffix, false)

	contactA := createContactMethod(t, server, userSession, "telegram", "Integrity A "+suffix, "@integrity_a_"+suffix)
	contactB := createContactMethod(t, server, userSession, "telegram", "Integrity B "+suffix, "@integrity_b_"+suffix)
	otherContact := createContactMethod(t, server, otherSession, "telegram", "Integrity Other "+suffix, "@integrity_other_"+suffix)
	buyerContact := createContactMethod(t, server, buyerSession, "telegram", "Integrity Buyer "+suffix, "@integrity_buyer_"+suffix)
	sellerContact := createContactMethod(t, server, sellerSession, "telegram", "Integrity Seller "+suffix, "@integrity_seller_"+suffix)

	pool := openTestPool(t, databaseURL)
	defer pool.Close()

	versionB := currentContactVersionID(t, pool, contactB.ID)
	_, err = pool.Exec(ctx, `
		UPDATE contact_methods
		SET current_version_id = $2
		WHERE id = $1
	`, contactA.ID, versionB)
	assertPostgresConstraintError(t, err, "cross contact current_version_id must be rejected")

	sessionID := createRawContactSession(t, pool, buyerSession.userID, sellerSession.userID, time.Now().UTC())
	otherVersionID := currentContactVersionID(t, pool, otherContact.ID)
	_, err = pool.Exec(ctx, `
		INSERT INTO contact_session_items (contact_session_id, subject_user_id, side, contact_method_version_id)
		VALUES ($1, $2, 'seller', $3)
	`, sessionID, otherSession.userID, otherVersionID)
	assertPostgresConstraintError(t, err, "third-party contact session item must be rejected")

	sellerVersionID := currentContactVersionID(t, pool, sellerContact.ID)
	_, err = pool.Exec(ctx, `
		INSERT INTO contact_session_items (contact_session_id, subject_user_id, side, contact_method_version_id)
		VALUES ($1, $2, 'buyer', $3)
	`, sessionID, sellerSession.userID, sellerVersionID)
	assertPostgresConstraintError(t, err, "wrong side contact session item must be rejected")

	buyerVersionID := currentContactVersionID(t, pool, buyerContact.ID)
	if _, err = pool.Exec(ctx, `
		INSERT INTO contact_session_items (contact_session_id, subject_user_id, side, contact_method_version_id)
		VALUES ($1, $2, 'buyer', $3)
	`, sessionID, buyerSession.userID, buyerVersionID); err != nil {
		t.Fatalf("valid buyer contact session item should be accepted: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO contact_sessions (buyer_user_id, seller_user_id, opens_at, ends_at, status)
		VALUES ($1, $2, $3, $3, 'open')
	`, buyerSession.userID, sellerSession.userID, time.Now().UTC())
	assertPostgresConstraintError(t, err, "non-positive contact window must be rejected")
}

func TestPostgresCarpoolMembershipIntegrityConstraints(t *testing.T) {
	databaseURL := os.Getenv("C2C_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	store, err := postgres.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer store.Close()

	server := NewServer(app.NewServiceWithPersistence(store))
	suffix := time.Now().Format("150405.000000000")
	ownerSession := createLinuxDoSession(t, server, "pg-member-owner-"+suffix)
	buyerSession := createSession(t, server, "pg-member-buyer-"+suffix, false)
	otherSession := createSession(t, server, "pg-member-other-"+suffix, false)
	ownerContact := createContactMethod(t, server, ownerSession, "telegram", "PG Member Owner "+suffix, "@pg_member_owner_"+suffix)
	buyerContact := createContactMethod(t, server, buyerSession, "telegram", "PG Member Buyer "+suffix, "@pg_member_buyer_"+suffix)

	listing := createCarpool(t, server, ownerSession, ownerContact.ID, "pg-member-create-"+suffix)
	published := submitCarpoolReview(t, server, ownerSession, listing.ID, listing.Version, "pg-member-submit-"+suffix)
	application := createCarpoolApplication(t, server, buyerSession, published.ID, buyerContact.ID, "pg-member-apply-"+suffix)
	accepted := acceptCarpoolApplication(t, server, ownerSession, application.ID, application.Version, "pg-member-accept-"+suffix)

	pool := openTestPool(t, databaseURL)
	defer pool.Close()

	_, err = pool.Exec(ctx, `
		INSERT INTO carpool_join_confirmations (
			carpool_application_id, actor_user_id, actor_role, confirmed_at, request_id
		)
		VALUES ($1, $2, 'buyer', now(), 'wrong-buyer')
	`, accepted.ID, otherSession.userID)
	assertPostgresConstraintError(t, err, "wrong buyer join confirmation actor must be rejected")

	_, err = pool.Exec(ctx, `
		INSERT INTO carpool_memberships (
			carpool_listing_id, carpool_application_id, buyer_user_id, owner_user_id,
			product_plan_id, status, seat_count, price_monthly_cny_snapshot,
			policy_version_snapshot, risk_notice_code_snapshot, joined_at
		)
		SELECT carpool_listing_id, id, buyer_user_id, owner_user_id,
		       product_plan_id, 'active', seat_count, price_monthly_cny_snapshot,
		       policy_version_snapshot, risk_notice_code_snapshot, now()
		FROM carpool_applications
		WHERE id = $1
	`, accepted.ID)
	assertPostgresConstraintError(t, err, "membership before joined application must be rejected")

	buyerConfirmed := confirmCarpoolJoin(t, server, buyerSession, "me", accepted.ID, accepted.Version, "pg-member-buyer-confirm-"+suffix)
	joined := confirmCarpoolJoin(t, server, ownerSession, "owner", accepted.ID, buyerConfirmed.Version, "pg-member-owner-confirm-"+suffix)
	membership := firstCarpoolMembership(t, server, ownerSession, "owner", joined.ID)
	_, err = pool.Exec(ctx, `
		INSERT INTO carpool_completion_confirmations (
			carpool_membership_id, actor_user_id, actor_role, confirmed_at, request_id
		)
		VALUES ($1, $2, 'buyer', now(), 'wrong-completion-buyer')
	`, membership.ID, otherSession.userID)
	assertPostgresConstraintError(t, err, "wrong buyer completion confirmation actor must be rejected")
}

func TestPostgresOfficialPriceAdminRecordSideEffectsAreIdempotent(t *testing.T) {
	databaseURL := os.Getenv("C2C_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	store, err := postgres.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer store.Close()

	server := NewServer(app.NewServiceWithPersistence(store))
	suffix := time.Now().Format("150405.000000000")
	adminSession := createSession(t, server, "pg-side-admin-"+suffix, true)
	body := adminRecordPayload("pg-side-record-"+suffix, "799.00", "0.12210000")

	first := createAdminOfficialPriceRecordWithBody(t, server, adminSession, body, "pg-side-record-"+suffix)
	second := createAdminOfficialPriceRecordWithBody(t, server, adminSession, body, "pg-side-record-"+suffix)
	if first.ID != second.ID {
		t.Fatalf("expected idempotent replay record, got %s and %s", first.ID, second.ID)
	}
	assertOfficialPriceRecordAdminSideEffects(t, databaseURL, first.ID, "official_price_record.created", "official_price_record.create", 1)
	assertOfficialPriceRecordIdempotencyCache(t, databaseURL, adminSession.userID, "pg-side-record-"+suffix, first.ID)
}

func TestPostgresCarpoolApplicationFlow(t *testing.T) {
	databaseURL := os.Getenv("C2C_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	store, err := postgres.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer store.Close()

	server := NewServer(app.NewServiceWithPersistence(store))
	suffix := time.Now().Format("150405.000000000")
	ownerSession := createLinuxDoSession(t, server, "pg-carpool-owner-"+suffix)
	buyerSession := createSession(t, server, "pg-carpool-buyer-"+suffix, false)
	ownerContact := createContactMethod(t, server, ownerSession, "telegram", "PG Owner Carpool TG "+suffix, "@pg_owner_carpool_"+suffix)
	buyerContact := createContactMethod(t, server, buyerSession, "telegram", "PG Buyer Carpool TG "+suffix, "@pg_buyer_carpool_"+suffix)

	unboundOwner := createSession(t, server, "pg-carpool-unbound-owner-"+suffix, false)
	unboundContact := createContactMethod(t, server, unboundOwner, "telegram", "PG Unbound Carpool Owner "+suffix, "@pg_unbound_carpool_owner_"+suffix)
	unboundListing := createCarpool(t, server, unboundOwner, unboundContact.ID, "pg-carpool-unbound-create-"+suffix)
	unboundPublish := newJSONRequest(http.MethodPost, "/api/v1/carpools/"+unboundListing.ID+"/submit-review", `{}`)
	addAuth(unboundPublish, unboundOwner, "pg-carpool-unbound-publish-"+suffix)
	unboundPublish.Header.Set("If-Match", `"`+strconv.FormatInt(unboundListing.Version, 10)+`"`)
	unboundResponse := httptest.NewRecorder()
	server.ServeHTTP(unboundResponse, unboundPublish)
	if unboundResponse.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected postgres unbound linux.do publish failure, got %d body %s", unboundResponse.Code, unboundResponse.Body.String())
	}
	assertProblemCode(t, unboundResponse, "VALIDATION_FAILED")

	listing := createCarpool(t, server, ownerSession, ownerContact.ID, "pg-carpool-create-"+suffix)
	published := submitCarpoolReview(t, server, ownerSession, listing.ID, listing.Version, "pg-carpool-submit-review-"+suffix)
	if published.Status != app.CarpoolListingStatusActive || published.AvailableSeats != 1 {
		t.Fatalf("expected one available seat after publish, got %+v", published)
	}
	if published.ServiceMultiplier != "1.3500" || published.MonthlyQuotaAmount != "200.00" || published.QuotaLabel != "额度" || published.QuotaUnit != "USD" || published.QuotaPeriod != "monthly" {
		t.Fatalf("expected postgres multiplier and quota fields after publish, got %+v", published)
	}
	if published.RegionCode != "other" || published.RegionName != "印度区" {
		t.Fatalf("expected postgres region fields after publish, got %+v", published)
	}
	if published.DistributionMethod != "sub2api" || published.DistributionMethodNote != "Sub2API 托管管理，具体方式站外确认。" || !published.ProvidesAdminAccount {
		t.Fatalf("expected postgres distribution/admin account fields after publish, got %+v", published)
	}
	selfApplicationRequest := newJSONRequest(http.MethodPost, "/api/v1/carpools/"+published.ID+"/applications", carpoolApplicationPayload(ownerContact.ID))
	addAuth(selfApplicationRequest, ownerSession, "pg-carpool-apply-own-"+suffix)
	selfApplicationResponse := httptest.NewRecorder()
	server.ServeHTTP(selfApplicationResponse, selfApplicationRequest)
	if selfApplicationResponse.Code != http.StatusConflict {
		t.Fatalf("expected postgres own carpool application conflict, got %d body %s", selfApplicationResponse.Code, selfApplicationResponse.Body.String())
	}
	if !strings.Contains(selfApplicationResponse.Body.String(), "不能申请自己的车源") {
		t.Fatalf("expected postgres own carpool application detail, got %s", selfApplicationResponse.Body.String())
	}
	assertProblemCode(t, selfApplicationResponse, "INVALID_STATE_TRANSITION")

	application := createCarpoolApplication(t, server, buyerSession, published.ID, buyerContact.ID, "pg-carpool-apply-"+suffix)
	assertCarpoolApplicationCreatedOwnerNotification(t, databaseURL, application.ID, ownerSession.userID, 1)
	first := acceptCarpoolApplication(t, server, ownerSession, application.ID, application.Version, "pg-carpool-accept-"+suffix)
	second := acceptCarpoolApplication(t, server, ownerSession, application.ID, application.Version, "pg-carpool-accept-"+suffix)
	if first.ContactSessionID == "" || first.ContactSessionID != second.ContactSessionID {
		t.Fatalf("expected idempotent contact session, got %+v and %+v", first, second)
	}
	assertCarpoolApplicationSideEffects(t, databaseURL, application.ID, first.ContactSessionID, 1)
	assertCarpoolAcceptIdempotencyCache(t, databaseURL, ownerSession.userID, application.ID, "pg-carpool-accept-"+suffix, first.ContactSessionID)

	adminSession := createSession(t, server, "pg-carpool-admin-"+suffix, true)
	reviewListing := createCarpool(t, server, ownerSession, ownerContact.ID, "pg-carpool-legacy-review-create-"+suffix)
	forcedPendingVersion := forceCarpoolPendingReview(t, databaseURL, reviewListing.ID)
	changesRequested := reviewCarpool(t, server, adminSession, reviewListing.ID, "request-changes", forcedPendingVersion, "pg-carpool-request-changes-"+suffix)
	if changesRequested.Status != app.CarpoolListingStatusChangesRequested {
		t.Fatalf("unexpected legacy changes-requested listing: %+v", changesRequested)
	}
	republished := submitCarpoolReview(t, server, ownerSession, changesRequested.ID, changesRequested.Version, "pg-carpool-republish-"+suffix)
	if republished.Status != app.CarpoolListingStatusActive {
		t.Fatalf("unexpected republished listing: %+v", republished)
	}
	rejectedListing := createCarpool(t, server, ownerSession, ownerContact.ID, "pg-carpool-legacy-reject-create-"+suffix)
	rejectedPendingVersion := forceCarpoolPendingReview(t, databaseURL, rejectedListing.ID)
	rejected := reviewCarpool(t, server, adminSession, rejectedListing.ID, "reject", rejectedPendingVersion, "pg-carpool-reject-"+suffix)
	if rejected.Status != app.CarpoolListingStatusRejected {
		t.Fatalf("unexpected legacy rejected listing: %+v", rejected)
	}
	approvedListing := createCarpool(t, server, ownerSession, ownerContact.ID, "pg-carpool-legacy-approve-create-"+suffix)
	approvedPendingVersion := forceCarpoolPendingReview(t, databaseURL, approvedListing.ID)
	approvedLegacy := reviewCarpool(t, server, adminSession, approvedListing.ID, "approve", approvedPendingVersion, "pg-carpool-approve-legacy-"+suffix)
	if approvedLegacy.Status != app.CarpoolListingStatusActive {
		t.Fatalf("unexpected legacy approved listing: %+v", approvedLegacy)
	}

	readContact := httptest.NewRequest(http.MethodGet, "/api/v1/contact-sessions/"+first.ContactSessionID+"/contacts", nil)
	addCookie(readContact, buyerSession.cookie)
	readResponse := httptest.NewRecorder()
	server.ServeHTTP(readResponse, readContact)
	if readResponse.Code != http.StatusOK {
		t.Fatalf("read carpool contact status %d body %s", readResponse.Code, readResponse.Body.String())
	}
	if got := readResponse.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected no-store, got %q", got)
	}
	if !strings.Contains(readResponse.Body.String(), "@pg_owner_carpool_"+suffix) {
		t.Fatalf("expected owner contact in accepted carpool contact response")
	}

	buyerConfirmed := confirmCarpoolJoin(t, server, buyerSession, "me", first.ID, first.Version, "pg-carpool-buyer-confirm-"+suffix)
	if buyerConfirmed.Status != app.CarpoolApplicationStatusAcceptedReserved || buyerConfirmed.BuyerConfirmedAt == nil {
		t.Fatalf("unexpected postgres buyer-confirmed application: %+v", buyerConfirmed)
	}
	joined := confirmCarpoolJoin(t, server, ownerSession, "owner", first.ID, buyerConfirmed.Version, "pg-carpool-owner-confirm-"+suffix)
	if joined.Status != app.CarpoolApplicationStatusJoined || joined.JoinedAt == nil || joined.ReservationExpiresAt != nil {
		t.Fatalf("unexpected postgres joined application: %+v", joined)
	}
	assertCarpoolJoinSideEffects(t, databaseURL, application.ID, buyerSession.userID, ownerSession.userID, 1)
	assertCarpoolJoinIdempotencyCache(t, databaseURL, buyerSession.userID, application.ID, "pg-carpool-buyer-confirm-"+suffix, app.CarpoolApplicationStatusAcceptedReserved)
	assertCarpoolJoinIdempotencyCache(t, databaseURL, ownerSession.userID, application.ID, "pg-carpool-owner-confirm-"+suffix, app.CarpoolApplicationStatusJoined)

	membership := firstCarpoolMembership(t, server, ownerSession, "owner", application.ID)
	if membership.Status != app.CarpoolMembershipStatusActive {
		t.Fatalf("unexpected postgres active membership: %+v", membership)
	}
	buyerCompleted := confirmCarpoolMembershipComplete(t, server, buyerSession, "me", membership.ID, membership.Version, "pg-carpool-buyer-complete-"+suffix)
	if buyerCompleted.Status != app.CarpoolMembershipStatusActive || buyerCompleted.BuyerCompletedAt == nil {
		t.Fatalf("unexpected postgres buyer-completed membership: %+v", buyerCompleted)
	}
	ownerCompleted := confirmCarpoolMembershipComplete(t, server, ownerSession, "owner", membership.ID, buyerCompleted.Version, "pg-carpool-owner-complete-"+suffix)
	if ownerCompleted.Status != app.CarpoolMembershipStatusCompleted || ownerCompleted.CompletedAt == nil || ownerCompleted.EndedAt == nil {
		t.Fatalf("unexpected postgres completed membership: %+v", ownerCompleted)
	}
	assertCarpoolMembershipCompletionSideEffects(t, databaseURL, membership.ID, buyerSession.userID, ownerSession.userID, 1)
	assertCarpoolMembershipIdempotencyCache(t, databaseURL, buyerSession.userID, membership.ID, "pg-carpool-buyer-complete-"+suffix, app.CarpoolMembershipStatusActive)
	assertCarpoolMembershipIdempotencyCache(t, databaseURL, ownerSession.userID, membership.ID, "pg-carpool-owner-complete-"+suffix, app.CarpoolMembershipStatusCompleted)
}

func storeAccessLogCount(t *testing.T, store *postgres.Store, sessionID string) int {
	t.Helper()
	count, appErr := store.ContactAccessLogCount(context.Background(), sessionID)
	if appErr != nil {
		t.Fatalf("count access logs: %v", appErr)
	}
	return count
}

func assertContactCiphertextDoesNotContain(t *testing.T, databaseURL, plaintext string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect for ciphertext query: %v", err)
	}
	defer pool.Close()
	rows, err := pool.Query(ctx, `
		SELECT encode(value_ciphertext, 'escape')
		FROM contact_method_versions
		WHERE masked_value LIKE '@p%'
	`)
	if err != nil {
		t.Fatalf("query ciphertext: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var encoded string
		if err := rows.Scan(&encoded); err != nil {
			t.Fatalf("scan ciphertext: %v", err)
		}
		if strings.Contains(encoded, plaintext) {
			t.Fatalf("ciphertext unexpectedly contains plaintext contact value")
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("ciphertext rows: %v", err)
	}
}

func openTestPool(t *testing.T, databaseURL string) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("ping postgres: %v", err)
	}
	return pool
}

func currentContactVersionID(t *testing.T, pool *pgxpool.Pool, methodID string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), `
		SELECT current_version_id::text
		FROM contact_methods
		WHERE id = $1
	`, methodID).Scan(&id); err != nil {
		t.Fatalf("query current version for %s: %v", methodID, err)
	}
	return id
}

func createRawContactSession(t *testing.T, pool *pgxpool.Pool, buyerID, sellerID string, now time.Time) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(context.Background(), `
		INSERT INTO contact_sessions (buyer_user_id, seller_user_id, opens_at, ends_at, status)
		VALUES ($1, $2, $3, $4, 'open')
		RETURNING id::text
	`, buyerID, sellerID, now, now.Add(10*time.Minute)).Scan(&id); err != nil {
		t.Fatalf("insert raw contact session: %v", err)
	}
	return id
}

func insertRawAPIPurchaseIntent(t *testing.T, pool *pgxpool.Pool, serviceID string, serviceVersion int64, ownerID, buyerID, ownerContactID, buyerContactID string, now time.Time) error {
	t.Helper()
	buyerVersionID := currentContactVersionID(t, pool, buyerContactID)
	ownerVersionID := currentContactVersionID(t, pool, ownerContactID)
	return insertRawAPIPurchaseIntentWithVersionOverride(t, pool, serviceID, serviceVersion, ownerID, buyerID, ownerContactID, buyerContactID, buyerVersionID, ownerVersionID, now)
}

func insertRawAPIPurchaseIntentWithVersionOverride(t *testing.T, pool *pgxpool.Pool, serviceID string, serviceVersion int64, ownerID, buyerID, ownerContactID, buyerContactID, buyerVersionID, ownerVersionID string, now time.Time) error {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO api_purchase_intents (
			api_service_id,
			api_service_owner_user_id,
			buyer_user_id,
			owner_user_id,
			buyer_contact_method_id,
			buyer_contact_method_version_id,
			owner_contact_method_id,
			owner_contact_method_version_id,
			status,
			requested_cny_amount,
			requested_usd_allowance,
			selected_access_mode,
			service_version_snapshot,
			service_title_snapshot,
			distribution_system_snapshot,
			billing_mode_snapshot,
			buyer_contact_type_snapshot,
			buyer_contact_label_snapshot,
			owner_contact_type_snapshot,
			owner_contact_label_snapshot,
			declared_cny_per_usd_allowance_snapshot,
			declared_max_usd_allowance_per_intent_snapshot,
			minimum_intent_cny_snapshot,
			maximum_intent_cny_snapshot,
			pricing_snapshot,
			created_at,
			updated_at
		)
		VALUES (
			$1, $2, $3, $2, $4, $5, $6, $7,
			'open', 16.00, 20.000000, 'buyer_dedicated_sub_key', $8, 'Sub2API raw intent',
			'sub2api', 'metered_usd_quota',
			'telegram', 'Raw buyer contact', 'telegram', 'Raw owner contact',
			0.8000, 20.000000, 10.00,
			200.00, '{}'::jsonb, $9, $9
		)
	`, serviceID, ownerID, buyerID, buyerContactID, buyerVersionID, ownerContactID, ownerVersionID, serviceVersion, now)
	return err
}

func poolExec(databaseURL, sql string, args ...any) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return 0, err
	}
	defer pool.Close()
	tag, err := pool.Exec(ctx, sql, args...)
	return tag.RowsAffected(), err
}

func assertPublicAPIServiceVisible(t *testing.T, server http.Handler, serviceID, ownerContactID string, wantVisible bool) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/api-services/"+serviceID, nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if wantVisible {
		if response.Code != http.StatusOK {
			t.Fatalf("expected public API service visible, got %d body %s", response.Code, response.Body.String())
		}
		assertPublicAPIServiceBody(t, response.Body.String(), ownerContactID)
		return
	}
	if response.Code != http.StatusNotFound {
		t.Fatalf("expected public API service hidden, got %d body %s", response.Code, response.Body.String())
	}
}

func assertAPIServiceChildren(t *testing.T, databaseURL, serviceID string, wantAccessModes, wantModels, wantPackages int) {
	t.Helper()
	pool := openTestPool(t, databaseURL)
	defer pool.Close()
	checks := map[string]struct {
		query string
		want  int
	}{
		"access modes": {
			query: `SELECT count(*)::int FROM api_service_access_modes WHERE api_service_id = $1`,
			want:  wantAccessModes,
		},
		"models": {
			query: `SELECT count(*)::int FROM api_service_models WHERE api_service_id = $1`,
			want:  wantModels,
		},
		"packages": {
			query: `SELECT count(*)::int FROM api_service_packages WHERE api_service_id = $1`,
			want:  wantPackages,
		},
	}
	for label, check := range checks {
		var count int
		if err := pool.QueryRow(context.Background(), check.query, serviceID).Scan(&count); err != nil {
			t.Fatalf("count API service %s: %v", label, err)
		}
		if count != check.want {
			t.Fatalf("expected %d API service %s, got %d", check.want, label, count)
		}
	}
}

func assertAPIServicePublicPredicateCount(t *testing.T, databaseURL, serviceID string, want int) {
	t.Helper()
	pool := openTestPool(t, databaseURL)
	defer pool.Close()
	var count int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*)::int
		FROM api_services
		WHERE id = $1
		  AND review_status = 'approved'
		  AND publication_status = 'online'
		  AND moderation_status = 'clear'
	`, serviceID).Scan(&count); err != nil {
		t.Fatalf("count public API service predicate: %v", err)
	}
	if count != want {
		t.Fatalf("expected public API service predicate count %d, got %d", want, count)
	}
}

func forceAPIServicePendingReview(t *testing.T, databaseURL, serviceID string) int64 {
	t.Helper()
	pool := openTestPool(t, databaseURL)
	defer pool.Close()
	var version int64
	err := pool.QueryRow(context.Background(), `
		UPDATE api_services
		SET review_status = 'pending_review',
		    publication_status = 'offline',
		    approved_by_admin_id = NULL,
		    approved_at = NULL,
		    updated_at = now(),
		    version = version + 1
		WHERE id = $1
		RETURNING version
	`, serviceID).Scan(&version)
	if err != nil {
		t.Fatalf("force API service pending review: %v", err)
	}
	return version
}

func forceCarpoolPendingReview(t *testing.T, databaseURL, listingID string) int64 {
	t.Helper()
	pool := openTestPool(t, databaseURL)
	defer pool.Close()
	var version int64
	err := pool.QueryRow(context.Background(), `
		UPDATE carpool_listings
		SET status = 'pending_review',
		    reviewed_by_admin_id = NULL,
		    reviewed_at = NULL,
		    review_reason = NULL,
		    updated_at = now(),
		    version = version + 1
		WHERE id = $1
		RETURNING version
	`, listingID).Scan(&version)
	if err != nil {
		t.Fatalf("force carpool pending review: %v", err)
	}
	return version
}

func assertAPIServiceIdempotencyCache(t *testing.T, databaseURL, userID, serviceID, action, key, expectedStatus string) {
	t.Helper()
	pool := openTestPool(t, databaseURL)
	defer pool.Close()

	routeKey := "POST /api/v1/owner/api-services/{id}/" + action + ":" + serviceID
	var status string
	var resourceType string
	var resourceID string
	var bodyText string
	if err := pool.QueryRow(context.Background(), `
		SELECT status, resource_type, resource_id::text, response_body_json::text
		FROM idempotency_keys
		WHERE user_id = $1 AND route_key = $2 AND idempotency_key = $3
	`, userID, routeKey, key).Scan(&status, &resourceType, &resourceID, &bodyText); err != nil {
		t.Fatalf("query API service idempotency cache: %v", err)
	}
	if status != "completed" || resourceType != "api_service" || resourceID != serviceID {
		t.Fatalf("unexpected API service idempotency cache: status=%s resource=%s %s", status, resourceType, resourceID)
	}
	if !strings.Contains(bodyText, expectedStatus) {
		t.Fatalf("cached API service response does not include status %s: %s", expectedStatus, bodyText)
	}
}

func assertAPIPurchaseIntentSideEffects(t *testing.T, databaseURL, intentID string, want int) {
	t.Helper()
	pool := openTestPool(t, databaseURL)
	defer pool.Close()

	checks := map[string]string{
		"intent": `
			SELECT count(*)::int
			FROM api_purchase_intents
			WHERE id = $1
			  AND status = 'open'
		`,
		"events": `
			SELECT count(*)::int
			FROM domain_events
			WHERE aggregate_type = 'api_purchase_intent'
			  AND aggregate_id = $1
			  AND event_type = 'api_purchase_intent.created'
		`,
		"notifications": `
			SELECT count(*)::int
			FROM notifications
			WHERE target_type = 'api_purchase_intent'
			  AND target_id = $1
			  AND type = 'api_purchase_intent.created'
		`,
	}
	for name, query := range checks {
		var count int
		if err := pool.QueryRow(context.Background(), query, intentID).Scan(&count); err != nil {
			t.Fatalf("count %s side effects: %v", name, err)
		}
		if count != want {
			t.Fatalf("expected %d %s side effects, got %d", want, name, count)
		}
	}
	assertAPIPurchaseIntentCreatesNoContactSession(t, pool, intentID)
}

func assertAPIPurchaseIntentActionSideEffects(t *testing.T, databaseURL, intentID, status, eventType string, want int) {
	t.Helper()
	pool := openTestPool(t, databaseURL)
	defer pool.Close()

	checks := map[string]string{
		"intent_status": `
			SELECT count(*)::int
			FROM api_purchase_intents
			WHERE id = $1 AND status = $2
		`,
		"events": `
			SELECT count(*)::int
			FROM domain_events
			WHERE aggregate_type = 'api_purchase_intent'
			  AND aggregate_id = $1
			  AND event_type = $2
		`,
		"notifications": `
			SELECT count(*)::int
			FROM notifications
			WHERE target_type = 'api_purchase_intent'
			  AND target_id = $1
			  AND type = $2
		`,
	}
	for name, query := range checks {
		var count int
		var err error
		if name == "intent_status" {
			err = pool.QueryRow(context.Background(), query, intentID, status).Scan(&count)
		} else {
			err = pool.QueryRow(context.Background(), query, intentID, eventType).Scan(&count)
		}
		if err != nil {
			t.Fatalf("count %s API intent action side effects: %v", name, err)
		}
		if count != want {
			t.Fatalf("expected %d %s API intent action side effects, got %d", want, name, count)
		}
	}
}

func assertAPIPurchaseIntentIdempotencyCache(t *testing.T, databaseURL, userID, routeResourceID, intentID, action, key, expectedStatus, forbiddenValue string) {
	t.Helper()
	pool := openTestPool(t, databaseURL)
	defer pool.Close()

	routeKey := "POST /api/v1/api-services/{id}/purchase-intents"
	if action == "mark-contacted" || action == "close" {
		routeKey = "POST /api/v1/owner/api-purchase-intents/{id}/" + action
	} else if action == "cancel" {
		routeKey = "POST /api/v1/me/api-purchase-intents/{id}/cancel"
	}
	var status string
	var resourceType string
	var resourceID string
	var bodyText string
	var cacheAllowed bool
	if err := pool.QueryRow(context.Background(), `
		SELECT status, resource_type, resource_id::text, COALESCE(response_body_json::text, ''), response_body_cache_allowed
		FROM idempotency_keys
		WHERE user_id = $1 AND route_key = $2 AND idempotency_key = $3
	`, userID, routeKey, key).Scan(&status, &resourceType, &resourceID, &bodyText, &cacheAllowed); err != nil {
		t.Fatalf("query API purchase intent idempotency cache: %v", err)
	}
	if status != "completed" || resourceType != "api_purchase_intent" || resourceID != intentID {
		t.Fatalf("unexpected API purchase intent idempotency cache: status=%s resource=%s %s", status, resourceType, resourceID)
	}
	if forbiddenValue != "" {
		if cacheAllowed || strings.Contains(bodyText, forbiddenValue) {
			t.Fatalf("API intent create idempotency cache leaked contact: cacheAllowed=%v body=%s", cacheAllowed, bodyText)
		}
		return
	}
	if !cacheAllowed || !strings.Contains(bodyText, expectedStatus) {
		t.Fatalf("cached API purchase intent response does not include status %s: cacheAllowed=%v body=%s", expectedStatus, cacheAllowed, bodyText)
	}
}

func assertAPIPurchaseIntentContactAccessLogs(t *testing.T, databaseURL, intentID string, wantMerchantAtLeast, wantBuyerAtLeast int) {
	t.Helper()
	pool := openTestPool(t, databaseURL)
	defer pool.Close()

	rows, err := pool.Query(context.Background(), `
		SELECT viewed_contact_owner_side, count(*)::int
		FROM api_purchase_intent_contact_access_logs
		WHERE api_purchase_intent_id = $1
		GROUP BY viewed_contact_owner_side
	`, intentID)
	if err != nil {
		t.Fatalf("query API purchase intent contact access logs: %v", err)
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var side string
		var count int
		if err := rows.Scan(&side, &count); err != nil {
			t.Fatalf("scan API purchase intent contact access log count: %v", err)
		}
		counts[side] = count
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate API purchase intent contact access logs: %v", err)
	}
	if counts["merchant"] < wantMerchantAtLeast || counts["buyer"] < wantBuyerAtLeast {
		t.Fatalf("expected API purchase intent contact access logs merchant>=%d buyer>=%d, got %+v", wantMerchantAtLeast, wantBuyerAtLeast, counts)
	}
}

func assertAPIPurchaseIntentCreatesNoContactSession(t *testing.T, pool *pgxpool.Pool, intentID string) {
	t.Helper()
	var hasContactSessionColumns bool
	if err := pool.QueryRow(context.Background(), `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_name = 'api_purchase_intents'
			  AND column_name IN ('contact_session_id', 'contact_opens_at', 'contact_expires_at')
		)
	`).Scan(&hasContactSessionColumns); err != nil {
		t.Fatalf("check API intent contact-session columns: %v", err)
	}
	if hasContactSessionColumns {
		t.Fatalf("api_purchase_intents still has API contact-session columns")
	}
	var sessionCount int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*)::int
		FROM contact_sessions s
		JOIN api_purchase_intents i
		  ON i.buyer_user_id = s.buyer_user_id
		 AND i.owner_user_id = s.seller_user_id
		 AND s.created_at >= i.created_at - interval '1 second'
		 AND s.created_at <= i.created_at + interval '1 second'
		WHERE i.id = $1
	`, intentID).Scan(&sessionCount); err != nil {
		t.Fatalf("count API intent contact sessions: %v", err)
	}
	if sessionCount != 0 {
		t.Fatalf("API purchase intent created %d contact session rows", sessionCount)
	}
}

func assertPostgresConstraintError(t *testing.T, err error, label string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected postgres constraint error, got nil", label)
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("%s: expected postgres error, got %v", label, err)
	}
	switch pgErr.Code {
	case "23503", "23505", "23514":
	default:
		t.Fatalf("%s: expected postgres constraint code, got %s: %v", label, pgErr.Code, err)
	}
}

func assertOfficialPriceRecordAdminSideEffects(t *testing.T, databaseURL, recordID, eventType, auditAction string, want int) {
	t.Helper()
	pool := openTestPool(t, databaseURL)
	defer pool.Close()

	checks := map[string]string{
		"records": `
			SELECT count(*)::int
			FROM official_price_records
			WHERE id = $1
		`,
		"events": `
			SELECT count(*)::int
			FROM domain_events
			WHERE aggregate_type = 'official_price_record'
			  AND aggregate_id = $1
			  AND event_type = $2
		`,
		"audit": `
			SELECT count(*)::int
			FROM admin_audit_logs
			WHERE target_type = 'official_price_record'
			  AND target_id = $1
			  AND action = $2
		`,
	}
	for name, query := range checks {
		var count int
		var err error
		if name == "records" {
			err = pool.QueryRow(context.Background(), query, recordID).Scan(&count)
		} else if name == "events" {
			err = pool.QueryRow(context.Background(), query, recordID, eventType).Scan(&count)
		} else {
			err = pool.QueryRow(context.Background(), query, recordID, auditAction).Scan(&count)
		}
		if err != nil {
			t.Fatalf("count %s side effects: %v", name, err)
		}
		if count != want {
			t.Fatalf("expected %d %s side effects, got %d", want, name, count)
		}
	}
}

func assertOfficialPriceRecordIdempotencyCache(t *testing.T, databaseURL, adminUserID, key, recordID string) {
	t.Helper()
	pool := openTestPool(t, databaseURL)
	defer pool.Close()

	routeKey := "POST /api/v1/admin/official-price-records"
	var status string
	var responseStatus int
	var contentType string
	var bodyText string
	var resourceType string
	var resourceID string
	var completed bool
	if err := pool.QueryRow(context.Background(), `
		SELECT status, response_status, response_content_type, response_body_json::text,
		       resource_type, resource_id::text, completed_at IS NOT NULL
		FROM idempotency_keys
		WHERE user_id = $1 AND route_key = $2 AND idempotency_key = $3
	`, adminUserID, routeKey, key).Scan(
		&status,
		&responseStatus,
		&contentType,
		&bodyText,
		&resourceType,
		&resourceID,
		&completed,
	); err != nil {
		t.Fatalf("query approval idempotency cache: %v", err)
	}
	if status != "completed" || !completed {
		t.Fatalf("expected completed idempotency cache, got status=%q completed=%v", status, completed)
	}
	if responseStatus != http.StatusOK || contentType != "application/json; charset=utf-8" {
		t.Fatalf("unexpected cached response metadata: status=%d contentType=%q", responseStatus, contentType)
	}
	if resourceType != "official_price_record" || resourceID != recordID {
		t.Fatalf("unexpected cached resource: %s %s", resourceType, resourceID)
	}
	if !strings.Contains(bodyText, recordID) {
		t.Fatalf("cached response body does not include record id %s: %s", recordID, bodyText)
	}
}

func assertCarpoolApplicationSideEffects(t *testing.T, databaseURL, applicationID, contactSessionID string, want int) {
	t.Helper()
	pool := openTestPool(t, databaseURL)
	defer pool.Close()

	checks := map[string]string{
		"application_contact_session": `
			SELECT count(*)::int
			FROM carpool_applications
			WHERE id = $1
			  AND contact_session_id = $2
			  AND status = 'accepted_reserved'
			  AND reservation_expires_at IS NOT NULL
			  AND reservation_expires_at > decided_at
		`,
		"application_session_participants": `
			SELECT count(*)::int
			FROM carpool_applications application
			JOIN contact_sessions session ON session.id = application.contact_session_id
			WHERE application.id = $1
			  AND application.contact_session_id = $2
			  AND session.buyer_user_id = application.buyer_user_id
			  AND session.seller_user_id = application.owner_user_id
		`,
		"contact_session": `
			SELECT count(*)::int
			FROM contact_sessions
			WHERE id = $1 AND status = 'open' AND ends_at > opens_at
		`,
		"contact_items": `
			SELECT count(*)::int
			FROM contact_session_items
			WHERE contact_session_id = $1
		`,
		"events": `
			SELECT count(*)::int
			FROM domain_events
			WHERE aggregate_type = 'carpool_application'
			  AND aggregate_id = $1
			  AND event_type = 'carpool_application.accepted'
		`,
		"notifications": `
			SELECT count(*)::int
			FROM notifications
			WHERE target_type = 'carpool_application'
			  AND target_id = $1
			  AND type = 'carpool_application.accepted'
		`,
	}
	for name, query := range checks {
		var count int
		var err error
		switch name {
		case "application_contact_session", "application_session_participants":
			err = pool.QueryRow(context.Background(), query, applicationID, contactSessionID).Scan(&count)
		case "contact_session", "contact_items":
			err = pool.QueryRow(context.Background(), query, contactSessionID).Scan(&count)
			if name == "contact_items" {
				if err != nil {
					t.Fatalf("count %s: %v", name, err)
				}
				if count != 2 {
					t.Fatalf("expected two contact session items, got %d", count)
				}
				continue
			}
		default:
			err = pool.QueryRow(context.Background(), query, applicationID).Scan(&count)
		}
		if err != nil {
			t.Fatalf("count %s side effects: %v", name, err)
		}
		if count != want {
			t.Fatalf("expected %d %s side effects, got %d", want, name, count)
		}
	}
}

func assertCarpoolAcceptIdempotencyCache(t *testing.T, databaseURL, ownerUserID, applicationID, key, contactSessionID string) {
	t.Helper()
	pool := openTestPool(t, databaseURL)
	defer pool.Close()

	routeKey := "POST /api/v1/owner/carpool-applications/{id}/accept:" + applicationID
	var status string
	var resourceType string
	var resourceID string
	var bodyText string
	if err := pool.QueryRow(context.Background(), `
		SELECT status, resource_type, resource_id::text, response_body_json::text
		FROM idempotency_keys
		WHERE user_id = $1 AND route_key = $2 AND idempotency_key = $3
	`, ownerUserID, routeKey, key).Scan(&status, &resourceType, &resourceID, &bodyText); err != nil {
		t.Fatalf("query carpool accept idempotency cache: %v", err)
	}
	if status != "completed" || resourceType != "carpool_application" || resourceID != applicationID {
		t.Fatalf("unexpected carpool idempotency cache: status=%s resource=%s %s", status, resourceType, resourceID)
	}
	if !strings.Contains(bodyText, contactSessionID) {
		t.Fatalf("cached carpool accept response does not include contact session %s: %s", contactSessionID, bodyText)
	}
}

func assertCarpoolApplicationCreatedOwnerNotification(t *testing.T, databaseURL, applicationID, ownerUserID string, want int) {
	t.Helper()
	pool := openTestPool(t, databaseURL)
	defer pool.Close()

	var count int
	var targetURL string
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*)::int, COALESCE(max(target_url), '')
		FROM notifications
		WHERE user_id = $1
		  AND target_type = 'carpool_application'
		  AND target_id = $2
		  AND type = 'carpool_application.created'
		  AND read_at IS NULL
	`, ownerUserID, applicationID).Scan(&count, &targetURL); err != nil {
		t.Fatalf("query owner application notification: %v", err)
	}
	if count != want {
		t.Fatalf("expected %d owner application notification, got %d", want, count)
	}
	expectedURL := "/merchant/carpool-applications/" + applicationID
	if want > 0 && targetURL != expectedURL {
		t.Fatalf("expected owner notification target URL %q, got %q", expectedURL, targetURL)
	}
}

func assertCarpoolJoinSideEffects(t *testing.T, databaseURL, applicationID, buyerUserID, ownerUserID string, want int) {
	t.Helper()
	pool := openTestPool(t, databaseURL)
	defer pool.Close()

	checks := map[string]string{
		"buyer_confirmation": `
			SELECT count(*)::int
			FROM carpool_join_confirmations
			WHERE carpool_application_id = $1
			  AND actor_user_id = $2
			  AND actor_role = 'buyer'
		`,
		"owner_confirmation": `
			SELECT count(*)::int
			FROM carpool_join_confirmations
			WHERE carpool_application_id = $1
			  AND actor_user_id = $2
			  AND actor_role = 'owner'
		`,
		"membership": `
			SELECT count(*)::int
			FROM carpool_memberships membership
			JOIN carpool_applications application ON application.id = membership.carpool_application_id
			JOIN carpool_listings listing ON listing.id = membership.carpool_listing_id
			WHERE application.id = $1
			  AND application.status = 'joined'
			  AND application.joined_at IS NOT NULL
			  AND membership.status = 'active'
			  AND membership.joined_at = application.joined_at
			  AND listing.active_buyer_members >= 1
		`,
		"joined_event": `
			SELECT count(*)::int
			FROM domain_events
			WHERE aggregate_type = 'carpool_application'
			  AND aggregate_id = $1
			  AND event_type = 'carpool_application.joined'
		`,
	}
	for name, query := range checks {
		var count int
		var err error
		switch name {
		case "buyer_confirmation":
			err = pool.QueryRow(context.Background(), query, applicationID, buyerUserID).Scan(&count)
		case "owner_confirmation":
			err = pool.QueryRow(context.Background(), query, applicationID, ownerUserID).Scan(&count)
		default:
			err = pool.QueryRow(context.Background(), query, applicationID).Scan(&count)
		}
		if err != nil {
			t.Fatalf("count %s side effects: %v", name, err)
		}
		if count != want {
			t.Fatalf("expected %d %s side effects, got %d", want, name, count)
		}
	}
}

func assertCarpoolJoinIdempotencyCache(t *testing.T, databaseURL, userID, applicationID, key, expectedStatus string) {
	t.Helper()
	pool := openTestPool(t, databaseURL)
	defer pool.Close()

	routeKey := "POST /api/v1/me/carpool-applications/{id}/confirm-join:" + applicationID
	if expectedStatus == app.CarpoolApplicationStatusJoined {
		routeKey = "POST /api/v1/owner/carpool-applications/{id}/confirm-join:" + applicationID
	}
	var status string
	var resourceType string
	var resourceID string
	var bodyText string
	if err := pool.QueryRow(context.Background(), `
		SELECT status, resource_type, resource_id::text, response_body_json::text
		FROM idempotency_keys
		WHERE user_id = $1 AND route_key = $2 AND idempotency_key = $3
	`, userID, routeKey, key).Scan(&status, &resourceType, &resourceID, &bodyText); err != nil {
		t.Fatalf("query carpool join idempotency cache: %v", err)
	}
	if status != "completed" || resourceType != "carpool_application" || resourceID != applicationID {
		t.Fatalf("unexpected carpool join idempotency cache: status=%s resource=%s %s", status, resourceType, resourceID)
	}
	if !strings.Contains(bodyText, expectedStatus) {
		t.Fatalf("cached carpool join response does not include status %s: %s", expectedStatus, bodyText)
	}
}

func assertCarpoolMembershipCompletionSideEffects(t *testing.T, databaseURL, membershipID, buyerUserID, ownerUserID string, want int) {
	t.Helper()
	pool := openTestPool(t, databaseURL)
	defer pool.Close()

	checks := map[string]string{
		"buyer_completion": `
			SELECT count(*)::int
			FROM carpool_completion_confirmations
			WHERE carpool_membership_id = $1
			  AND actor_user_id = $2
			  AND actor_role = 'buyer'
		`,
		"owner_completion": `
			SELECT count(*)::int
			FROM carpool_completion_confirmations
			WHERE carpool_membership_id = $1
			  AND actor_user_id = $2
			  AND actor_role = 'owner'
		`,
		"membership_completed": `
			SELECT count(*)::int
			FROM carpool_memberships membership
			JOIN carpool_listings listing ON listing.id = membership.carpool_listing_id
			WHERE membership.id = $1
			  AND membership.status = 'completed'
			  AND membership.ended_at IS NOT NULL
			  AND membership.ended_reason <> ''
			  AND listing.active_buyer_members = 0
		`,
		"completed_event": `
			SELECT count(*)::int
			FROM domain_events
			WHERE aggregate_type = 'carpool_membership'
			  AND aggregate_id = $1
			  AND event_type = 'carpool_membership.completed'
		`,
	}
	for name, query := range checks {
		var count int
		var err error
		switch name {
		case "buyer_completion":
			err = pool.QueryRow(context.Background(), query, membershipID, buyerUserID).Scan(&count)
		case "owner_completion":
			err = pool.QueryRow(context.Background(), query, membershipID, ownerUserID).Scan(&count)
		default:
			err = pool.QueryRow(context.Background(), query, membershipID).Scan(&count)
		}
		if err != nil {
			t.Fatalf("count %s side effects: %v", name, err)
		}
		if count != want {
			t.Fatalf("expected %d %s side effects, got %d", want, name, count)
		}
	}
}

func assertCarpoolMembershipIdempotencyCache(t *testing.T, databaseURL, userID, membershipID, key, expectedStatus string) {
	t.Helper()
	pool := openTestPool(t, databaseURL)
	defer pool.Close()

	routeKey := "POST /api/v1/me/carpool-memberships/{id}/confirm-complete:" + membershipID
	if expectedStatus == app.CarpoolMembershipStatusCompleted {
		routeKey = "POST /api/v1/owner/carpool-memberships/{id}/confirm-complete:" + membershipID
	}
	var status string
	var resourceType string
	var resourceID string
	var bodyText string
	if err := pool.QueryRow(context.Background(), `
		SELECT status, resource_type, resource_id::text, response_body_json::text
		FROM idempotency_keys
		WHERE user_id = $1 AND route_key = $2 AND idempotency_key = $3
	`, userID, routeKey, key).Scan(&status, &resourceType, &resourceID, &bodyText); err != nil {
		t.Fatalf("query carpool membership idempotency cache: %v", err)
	}
	if status != "completed" || resourceType != "carpool_membership" || resourceID != membershipID {
		t.Fatalf("unexpected carpool membership idempotency cache: status=%s resource=%s %s", status, resourceType, resourceID)
	}
	if !strings.Contains(bodyText, expectedStatus) {
		t.Fatalf("cached carpool membership response does not include status %s: %s", expectedStatus, bodyText)
	}
}
