package server

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
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
)

func TestPostgresAPIQuotaHTTPFlow(t *testing.T) {
	databaseURL := os.Getenv("C2C_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	store, err := postgres.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer store.Close()
	pool := openTestPool(t, databaseURL)
	defer pool.Close()

	server := NewServer(app.NewServiceWithPersistence(store))
	suffix := time.Now().UTC().Format("150405.000000000")
	owner := createLinuxDoSession(t, server, "pg-quota-owner-"+suffix)
	buyer := createSession(t, server, "pg-quota-buyer-"+suffix, false)
	secondBuyer := createSession(t, server, "pg-quota-second-buyer-"+suffix, false)
	ownerContact := createContactMethod(t, server, owner, "telegram", "额度包卖家 "+suffix, "@pg_quota_owner_"+strings.ReplaceAll(suffix, ".", "_"))
	buyerContact := createContactMethod(t, server, buyer, "telegram", "额度包买家 "+suffix, "@pg_quota_buyer_"+strings.ReplaceAll(suffix, ".", "_"))
	secondBuyerContact := createContactMethod(t, server, secondBuyer, "telegram", "额度包买家二 "+suffix, "@pg_quota_buyer_two_"+strings.ReplaceAll(suffix, ".", "_"))

	now := time.Now().UTC()
	serviceBody := strings.Replace(apiServicePayload(ownerContact.ID, "1.0000"), `"accessModes":`, `
		"declaredTtftBand":"under_1s",
		"declaredMaxConcurrency":24,
		"performanceConfirmedAt":"`+now.Add(-time.Minute).Format(time.RFC3339)+`",
		"accessModes":`, 1)
	service := createAPIServiceWithPayload(t, server, owner, serviceBody, "pg-quota-service-create-"+suffix)
	submitted := ownerAPIServiceAction(t, server, owner, service.ID, "submit-review", service.Version, "pg-quota-service-submit-"+suffix)
	publishedService := ownerAPIServiceAction(t, server, owner, submitted.ID, "publish", submitted.Version, "pg-quota-service-publish-"+suffix)
	orderableService := updateAPIServiceOrderSettings(t, server, owner, publishedService.ID, publishedService.Version, true, "pg-quota-service-settings-"+suffix)

	batch := createQuotaBatchHTTP(t, server, owner, orderableService.ID, now, "pg-quota-batch-"+suffix)
	continuous := createQuotaOfferHTTP(t, server, owner, batch.ID, `{
		"name":"$50 全天额度包",
		"usdAllowance":"50",
		"priceCny":"5.00",
		"modelMultiplier":"1.0000",
		"quotaUsagePolicy":{"fiveHour":{"mode":"limited","amountUsd":"5.000000"},"daily":{"mode":"unlimited"}},
		"deliveryMode":"manual",
		"deliveryEtaMinutes":10,
		"saleMode":"continuous",
		"continuousCopies":1,
		"sortOrder":10
	}`, "pg-quota-continuous-"+suffix)
	scheduled := createQuotaOfferHTTP(t, server, owner, batch.ID, `{
		"name":"$100 整点额度包",
		"usdAllowance":"100",
		"priceCny":"9.00",
		"modelMultiplier":"1.0000",
		"quotaUsagePolicy":{"fiveHour":{"mode":"limited","amountUsd":"10.000000"},"daily":{"mode":"limited","amountUsd":"50.000000"}},
		"deliveryMode":"manual",
		"deliveryEtaMinutes":10,
		"saleMode":"scheduled",
		"continuousCopies":0,
		"sortOrder":20
	}`, "pg-quota-scheduled-"+suffix)
	preimported := createQuotaOfferHTTP(t, server, owner, batch.ID, `{
		"name":"$20 预导入额度包",
		"usdAllowance":"20",
		"priceCny":"2.00",
		"modelMultiplier":"1.0000",
		"quotaUsagePolicy":{"fiveHour":{"mode":"unlimited"},"daily":{"mode":"limited","amountUsd":"20.000000"}},
		"deliveryMode":"preimported",
		"deliveryEtaMinutes":5,
		"saleMode":"continuous",
		"continuousCopies":1,
		"sortOrder":30
	}`, "pg-quota-preimported-"+suffix)
	round := createQuotaRoundHTTP(t, server, owner, batch.ID, scheduled.ID, now, "pg-quota-round-"+suffix)

	secret := "sk-pg-quota-secret-" + strings.ReplaceAll(suffix, ".", "")
	imported := importQuotaCredentialHTTP(t, server, owner, preimported.ID, secret, "pg-quota-import-"+suffix)
	if imported.Imported != 1 || imported.Summary.Available != 1 {
		t.Fatalf("unexpected credential import result: %+v", imported)
	}
	summary := getQuotaCredentialSummaryHTTP(t, server, owner, preimported.ID)
	if summary.Available != 1 {
		t.Fatalf("unexpected credential summary: %+v", summary)
	}

	publishedBatch := quotaBatchActionHTTP(t, server, owner, batch.ID, "publish", batch.Version, "pg-quota-publish-"+suffix)
	if publishedBatch.Status != "published" {
		t.Fatalf("expected published quota batch, got %+v", publishedBatch)
	}

	ownerListRequest := httptest.NewRequest(http.MethodGet, "/api/v1/owner/api-services/"+orderableService.ID+"/quota-batches", nil)
	addCookie(ownerListRequest, owner.cookie)
	ownerListResponse := httptest.NewRecorder()
	server.ServeHTTP(ownerListResponse, ownerListRequest)
	if ownerListResponse.Code != http.StatusOK || !strings.Contains(ownerListResponse.Body.String(), batch.ID) {
		t.Fatalf("owner quota batch list status %d body %s", ownerListResponse.Code, ownerListResponse.Body.String())
	}

	publicListRequest := httptest.NewRequest(http.MethodGet, "/api/v1/api-quota-offers?oneMultiplier=true&onlyOrderable=false", nil)
	publicListResponse := httptest.NewRecorder()
	server.ServeHTTP(publicListResponse, publicListRequest)
	if publicListResponse.Code != http.StatusOK {
		t.Fatalf("public quota list status %d body %s", publicListResponse.Code, publicListResponse.Body.String())
	}
	publicBody := publicListResponse.Body.String()
	for _, expected := range []string{
		continuous.ID,
		scheduled.ID,
		preimported.ID,
		`"quotaUsagePolicy":{"fiveHour":{"mode":"limited","amountUsd":"5.000000"},"daily":{"mode":"unlimited","amountUsd":null}`,
		`"quotaUsagePolicy":{"fiveHour":{"mode":"limited","amountUsd":"10.000000"},"daily":{"mode":"limited","amountUsd":"50.000000"}`,
		`"quotaUsagePolicy":{"fiveHour":{"mode":"unlimited","amountUsd":null},"daily":{"mode":"limited","amountUsd":"20.000000"}`,
		`"healthSummary":{"state":"no_sample","availabilityReason":"unconfigured"`,
	} {
		if !strings.Contains(publicBody, expected) {
			t.Fatalf("public quota list missing %q: %s", expected, publicBody)
		}
	}
	for _, retiredField := range []string{`"declaredTtftBand"`, `"performanceConfirmedAt"`, `"performanceDisclaimer"`} {
		if strings.Contains(publicBody, retiredField) {
			t.Fatalf("public quota list exposed retired merchant performance field %q: %s", retiredField, publicBody)
		}
	}
	if strings.Contains(publicBody, secret) {
		t.Fatalf("public quota list exposed imported credential")
	}

	continuousDetail := getPublicQuotaOfferHTTP(t, server, continuous.ID)
	if !continuousDetail.IsOrderable || continuousDetail.AvailableCopies != 1 || continuousDetail.PriceCNY != "5.00" || continuousDetail.USDAllowance != "50.000000" {
		t.Fatalf("unexpected continuous quota detail: %+v", continuousDetail)
	}

	order := createQuotaOrderHTTP(t, server, buyer, continuous.ID, "", buyerContact.ID, "pg-quota-order-"+suffix, http.StatusCreated)
	if order.PurchaseKind != "limited_quota_offer" || order.APIQuotaOfferID != continuous.ID || order.QuotaOfferNameSnapshot != "$50 全天额度包" || order.QuotaUSDAllowanceSnapshot != "50.000000" || order.QuotaPriceCNYSnapshot != "5.00" || order.QuotaModelMultiplierSnapshot != "1.0000" || order.PaymentWindowMinutesSnapshot != 10 {
		t.Fatalf("unexpected quota order snapshot: %+v", order)
	}
	replayed := createQuotaOrderHTTP(t, server, buyer, continuous.ID, "", buyerContact.ID, "pg-quota-order-"+suffix, http.StatusCreated)
	if replayed.ID != order.ID {
		t.Fatalf("expected idempotent quota order replay, got %s and %s", order.ID, replayed.ID)
	}

	soldOutRequest := newQuotaOrderRequest(continuous.ID, "", secondBuyerContact.ID)
	addAuth(soldOutRequest, secondBuyer, "pg-quota-sold-out-"+suffix)
	soldOutResponse := httptest.NewRecorder()
	server.ServeHTTP(soldOutResponse, soldOutRequest)
	if soldOutResponse.Code != http.StatusConflict {
		t.Fatalf("sold out quota status %d body %s", soldOutResponse.Code, soldOutResponse.Body.String())
	}
	assertProblemCode(t, soldOutResponse, domain.CodeAPIQuotaSoldOut)

	notStartedRequest := newQuotaOrderRequest(scheduled.ID, round.ID, secondBuyerContact.ID)
	addAuth(notStartedRequest, secondBuyer, "pg-quota-not-started-"+suffix)
	notStartedResponse := httptest.NewRecorder()
	server.ServeHTTP(notStartedResponse, notStartedRequest)
	if notStartedResponse.Code != http.StatusConflict {
		t.Fatalf("not-started quota status %d body %s", notStartedResponse.Code, notStartedResponse.Body.String())
	}
	assertProblemCode(t, notStartedResponse, domain.CodeAPIQuotaNotStarted)

	activeNow := time.Now().UTC()
	if _, err := pool.Exec(context.Background(), `UPDATE api_quota_sale_rounds SET starts_at = $2, ends_at = $3 WHERE id = $1`, round.ID, activeNow.Add(-time.Minute), activeNow.Add(10*time.Minute)); err != nil {
		t.Fatalf("activate quota round: %v", err)
	}
	scheduledOrder := createQuotaOrderHTTP(t, server, secondBuyer, scheduled.ID, round.ID, secondBuyerContact.ID, "pg-quota-round-order-"+suffix, http.StatusCreated)
	if scheduledOrder.PaymentWindowMinutesSnapshot != 5 || scheduledOrder.APIQuotaSaleRoundID != round.ID {
		t.Fatalf("unexpected scheduled quota order: %+v", scheduledOrder)
	}
	cancelled := apiOrderAction(t, server, secondBuyer, "buyer", scheduledOrder.ID, "cancel", scheduledOrder.Version, "pg-quota-round-cancel-"+suffix, `{"reason":"买家取消本轮额度包订单。"}`)
	if cancelled.Status != "cancelled" {
		t.Fatalf("expected cancelled scheduled order, got %+v", cancelled)
	}

	roundLimitRequest := newQuotaOrderRequest(scheduled.ID, round.ID, secondBuyerContact.ID)
	addAuth(roundLimitRequest, secondBuyer, "pg-quota-round-limit-"+suffix)
	roundLimitResponse := httptest.NewRecorder()
	server.ServeHTTP(roundLimitResponse, roundLimitRequest)
	if roundLimitResponse.Code != http.StatusConflict {
		t.Fatalf("round-limit quota status %d body %s", roundLimitResponse.Code, roundLimitResponse.Body.String())
	}
	assertProblemCode(t, roundLimitResponse, domain.CodeAPIQuotaBuyerRoundLimit)
}

func createQuotaBatchHTTP(t *testing.T, server http.Handler, owner testSession, serviceID string, now time.Time, key string) apiQuotaBatchResponse {
	t.Helper()
	request := newJSONRequest(http.MethodPost, "/api/v1/owner/api-services/"+serviceID+"/quota-batches", `{
		"sourceType":"sub2api",
		"sourceLabel":"",
		"declaredTotalUsdAllowance":"1000",
		"saleCutoffAt":"`+now.Add(4*time.Hour).Format(time.RFC3339)+`",
		"expiresAt":"`+now.Add(5*time.Hour).Format(time.RFC3339)+`",
		"sourceConfirmedAt":"`+now.Add(-time.Minute).Format(time.RFC3339)+`"
	}`)
	addAuth(request, owner, key)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create quota batch status %d body %s", response.Code, response.Body.String())
	}
	var payload apiQuotaBatchResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode quota batch: %v", err)
	}
	return payload
}

func createQuotaOfferHTTP(t *testing.T, server http.Handler, owner testSession, batchID, body, key string) apiQuotaOfferResponse {
	t.Helper()
	request := newJSONRequest(http.MethodPost, "/api/v1/owner/api-quota-batches/"+batchID+"/offers", body)
	addAuth(request, owner, key)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create quota offer status %d body %s", response.Code, response.Body.String())
	}
	var payload apiQuotaOfferResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode quota offer: %v", err)
	}
	return payload
}

func createQuotaRoundHTTP(t *testing.T, server http.Handler, owner testSession, batchID, offerID string, now time.Time, key string) apiQuotaRoundResponse {
	t.Helper()
	request := newJSONRequest(http.MethodPost, "/api/v1/owner/api-quota-batches/"+batchID+"/rounds", `{
		"name":"下一整点放量",
		"startsAt":"`+now.Add(10*time.Minute).Format(time.RFC3339)+`",
		"endsAt":"`+now.Add(20*time.Minute).Format(time.RFC3339)+`",
		"offers":[{"offerId":"`+offerID+`","copies":1}]
	}`)
	addAuth(request, owner, key)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("create quota round status %d body %s", response.Code, response.Body.String())
	}
	var payload apiQuotaRoundResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode quota round: %v", err)
	}
	return payload
}

func importQuotaCredentialHTTP(t *testing.T, server http.Handler, owner testSession, offerID, secret, key string) apiQuotaCredentialImportResponse {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("deliveryKind", "api_key_endpoint"); err != nil {
		t.Fatalf("write delivery kind: %v", err)
	}
	part, err := writer.CreateFormFile("file", "credentials.csv")
	if err != nil {
		t.Fatalf("create CSV part: %v", err)
	}
	if _, err := part.Write([]byte("api_base_url,api_key,instructions\nhttps://api.example.com/v1," + secret + ",buyer-only\n")); err != nil {
		t.Fatalf("write CSV part: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/owner/api-quota-offers/"+offerID+"/credentials/import", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	addAuth(request, owner, key)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("import quota credentials status %d body %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "private, no-store" || strings.Contains(response.Body.String(), secret) {
		t.Fatalf("credential import response leaked cacheable secret data: headers=%v body=%s", response.Header(), response.Body.String())
	}
	var payload apiQuotaCredentialImportResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode credential import: %v", err)
	}
	return payload
}

func getQuotaCredentialSummaryHTTP(t *testing.T, server http.Handler, owner testSession, offerID string) apiQuotaCredentialSummaryResponse {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/owner/api-quota-offers/"+offerID+"/credentials/summary", nil)
	addCookie(request, owner.cookie)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("quota credential summary status %d body %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("expected no-store credential summary, got %q", response.Header().Get("Cache-Control"))
	}
	var payload apiQuotaCredentialSummaryResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode credential summary: %v", err)
	}
	return payload
}

func quotaBatchActionHTTP(t *testing.T, server http.Handler, owner testSession, batchID, action string, version int64, key string) apiQuotaBatchResponse {
	t.Helper()
	request := newJSONRequest(http.MethodPost, "/api/v1/owner/api-quota-batches/"+batchID+"/"+action, `{}`)
	addAuth(request, owner, key)
	request.Header.Set("If-Match", `"`+strconv.FormatInt(version, 10)+`"`)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("quota batch %s status %d body %s", action, response.Code, response.Body.String())
	}
	var payload apiQuotaBatchResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode quota batch action: %v", err)
	}
	return payload
}

func getPublicQuotaOfferHTTP(t *testing.T, server http.Handler, offerID string) publicAPIQuotaOfferResponse {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/api-quota-offers/"+offerID, nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("public quota offer status %d body %s", response.Code, response.Body.String())
	}
	var payload publicAPIQuotaOfferResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode public quota offer: %v", err)
	}
	return payload
}

func newQuotaOrderRequest(offerID, roundID, contactID string) *http.Request {
	return newJSONRequest(http.MethodPost, "/api/v1/api-quota-offers/"+offerID+"/orders", `{
		"saleRoundId":"`+roundID+`",
		"buyerContactMethodId":"`+contactID+`",
		"selectedAccessMode":"buyer_dedicated_sub_key",
		"paymentMethod":"wechat",
		"buyerNote":"固定额度包订单"
	}`)
}

func createQuotaOrderHTTP(t *testing.T, server http.Handler, buyer testSession, offerID, roundID, contactID, key string, expectedStatus int) apiOrderResponse {
	t.Helper()
	request := newQuotaOrderRequest(offerID, roundID, contactID)
	addAuth(request, buyer, key)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != expectedStatus {
		t.Fatalf("create quota order status %d body %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("expected no-store quota order, got %q", response.Header().Get("Cache-Control"))
	}
	var payload apiOrderResponse
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode quota order: %v", err)
	}
	if response.Header().Get("Location") != "/api/v1/me/api-orders/"+payload.ID {
		t.Fatalf("unexpected quota order location %q", response.Header().Get("Location"))
	}
	return payload
}
