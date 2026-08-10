package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"sync"
	"testing"
	"time"

	app "c2c-market/backend/internal/module/core"
	"c2c-market/backend/internal/store/postgres"
)

func TestPostgresAPIPromotionCapacityAndLifecycle(t *testing.T) {
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
	ownerSession := createLinuxDoSession(t, server, "pg-promotion-owner-"+suffix)
	adminSession := createSession(t, server, "pg-promotion-admin-"+suffix, true)
	ownerContact := createContactMethod(t, server, ownerSession, "telegram", "PG Promotion Owner "+suffix, "@pg_promotion_owner_"+suffix)

	services := make([]createdAPIService, 0, 4)
	for index := 0; index < 4; index++ {
		keyPrefix := fmt.Sprintf("pg-promotion-%s-%d", suffix, index)
		service := createPostgresAPIService(t, databaseURL, server, ownerSession, ownerContact.ID, keyPrefix+"-create")
		service = ownerAPIServiceAction(t, server, ownerSession, service.ID, "submit-review", service.Version, keyPrefix+"-submit")
		service = ownerAPIServiceAction(t, server, ownerSession, service.ID, "publish", service.Version, keyPrefix+"-publish")
		service = updateAPIServiceOrderSettings(t, server, ownerSession, service.ID, service.Version, true, keyPrefix+"-orders")
		services = append(services, service)
	}

	startsAt := time.Now().UTC().Add(-time.Minute)
	endsAt := startsAt.Add(7 * 24 * time.Hour)
	first := createAPIPromotionForTest(t, server, adminSession, services[0].ID, startsAt, endsAt, "pg-promotion-first-"+suffix, http.StatusCreated)
	duplicate := createAPIPromotionForTest(t, server, adminSession, services[0].ID, startsAt, endsAt, "pg-promotion-duplicate-"+suffix, http.StatusConflict)
	if duplicate.ID != "" {
		t.Fatalf("overlapping promotion unexpectedly created: %+v", duplicate)
	}

	second := createAPIPromotionForTest(t, server, adminSession, services[1].ID, startsAt, endsAt, "pg-promotion-second-"+suffix, http.StatusCreated)
	type createResult struct {
		status int
		item   adminAPIPromotionResponse
	}
	results := make(chan createResult, 2)
	var waitGroup sync.WaitGroup
	for index := 2; index < 4; index++ {
		waitGroup.Add(1)
		go func(index int) {
			defer waitGroup.Done()
			item, status := createAPIPromotionRequestForTest(server, adminSession, services[index].ID, startsAt, endsAt, fmt.Sprintf("pg-promotion-concurrent-%s-%d", suffix, index))
			results <- createResult{status: status, item: item}
		}(index)
	}
	waitGroup.Wait()
	close(results)

	statuses := make([]int, 0, 2)
	var third adminAPIPromotionResponse
	for result := range results {
		statuses = append(statuses, result.status)
		if result.status == http.StatusCreated {
			third = result.item
		}
	}
	sort.Ints(statuses)
	if len(statuses) != 2 || statuses[0] != http.StatusCreated || statuses[1] != http.StatusConflict {
		t.Fatalf("expected one concurrent create and one capacity conflict, got %v", statuses)
	}

	publicItems := listPublicAPIPromotionsForTest(t, server)
	if len(publicItems) != 3 {
		t.Fatalf("expected three public promotions, got %d", len(publicItems))
	}
	for _, item := range publicItems {
		if item.Label != "推广" || item.Service.ID == "" {
			t.Fatalf("unexpected public promotion projection: %+v", item)
		}
	}

	stopAPIPromotionForTest(t, server, adminSession, first.ID, first.Version, "运营排期提前结束。", "pg-promotion-stop-"+suffix)
	publicItems = listPublicAPIPromotionsForTest(t, server)
	if len(publicItems) != 2 {
		t.Fatalf("expected stopped promotion to leave public pool, got %d items", len(publicItems))
	}

	pool := openTestPool(t, databaseURL)
	defer pool.Close()
	var auditCount int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*)::int
		FROM admin_audit_logs
		WHERE target_type = 'api_service_promotion'
		  AND target_id = $1
		  AND action IN ('api_service_promotion.created', 'api_service_promotion.stopped')
	`, first.ID).Scan(&auditCount); err != nil {
		t.Fatalf("query promotion audit count: %v", err)
	}
	if auditCount != 2 {
		t.Fatalf("expected create and stop audit records, got %d", auditCount)
	}
	if second.ID == "" || third.ID == "" {
		t.Fatalf("expected capacity-filling promotions, got second=%q third=%q", second.ID, third.ID)
	}

	staggeredBase := endsAt.Add(time.Hour)
	for index := 0; index < 3; index++ {
		staggeredStart := staggeredBase.Add(time.Duration(index) * time.Hour)
		createAPIPromotionForTest(
			t,
			server,
			adminSession,
			services[index].ID,
			staggeredStart,
			staggeredStart.Add(time.Hour),
			fmt.Sprintf("pg-promotion-staggered-%s-%d", suffix, index),
			http.StatusCreated,
		)
	}
	spanningStart := staggeredBase.Add(-30 * time.Minute)
	spanningEnd := staggeredBase.Add(3*time.Hour + 30*time.Minute)
	availability := getAPIPromotionAvailabilityForTest(t, server, adminSession, services[3].ID, spanningStart, spanningEnd, "pg-promotion-availability-"+suffix)
	if availability.OverlappingCampaigns != 1 || availability.RemainingCapacity != 2 || availability.SameServiceOverlap {
		t.Fatalf("expected staggered schedules to use peak occupancy 1 with two slots remaining, got %+v", availability)
	}

	spanningKey := "pg-promotion-spanning-" + suffix
	spanning, status, etag := createAPIPromotionResponseForTest(server, adminSession, services[3].ID, spanningStart, spanningEnd, spanningKey)
	if status != http.StatusCreated || etag != `"1"` {
		t.Fatalf("create spanning promotion status=%d etag=%q item=%+v", status, etag, spanning)
	}
	replayed, replayStatus, replayETag := createAPIPromotionResponseForTest(server, adminSession, services[3].ID, spanningStart, spanningEnd, spanningKey)
	if replayStatus != http.StatusCreated || replayed.ID != spanning.ID || replayETag != etag {
		t.Fatalf("create replay status=%d etag=%q item=%+v, expected id=%q etag=%q", replayStatus, replayETag, replayed, spanning.ID, etag)
	}

	stopped, stoppedETag := stopAPIPromotionResponseForTest(t, server, adminSession, spanning.ID, spanning.Version, "运营排期提前结束。", "pg-promotion-spanning-stop-"+suffix)
	if stoppedETag != `"2"` {
		t.Fatalf("stop spanning promotion etag=%q item=%+v", stoppedETag, stopped)
	}
	replayedStop, replayedStopETag := stopAPIPromotionResponseForTest(t, server, adminSession, spanning.ID, spanning.Version, "运营排期提前结束。", "pg-promotion-spanning-stop-"+suffix)
	if replayedStop.ID != stopped.ID || replayedStop.Version != stopped.Version || replayedStopETag != stoppedETag {
		t.Fatalf("stop replay etag=%q item=%+v, expected etag=%q item=%+v", replayedStopETag, replayedStop, stoppedETag, stopped)
	}

	var spanningAuditCount int
	if err := pool.QueryRow(context.Background(), `
		SELECT count(*)::int
		FROM admin_audit_logs
		WHERE target_type = 'api_service_promotion'
		  AND target_id = $1
		  AND action IN ('api_service_promotion.created', 'api_service_promotion.stopped')
	`, spanning.ID).Scan(&spanningAuditCount); err != nil {
		t.Fatalf("query spanning promotion audit count: %v", err)
	}
	if spanningAuditCount != 2 {
		t.Fatalf("expected one create and one stop audit after idempotent replays, got %d", spanningAuditCount)
	}
}

func createAPIPromotionForTest(t *testing.T, server http.Handler, session testSession, serviceID string, startsAt, endsAt time.Time, key string, expectedStatus int) adminAPIPromotionResponse {
	t.Helper()
	item, status := createAPIPromotionRequestForTest(server, session, serviceID, startsAt, endsAt, key)
	if status != expectedStatus {
		t.Fatalf("create API promotion status %d, expected %d", status, expectedStatus)
	}
	return item
}

func createAPIPromotionRequestForTest(server http.Handler, session testSession, serviceID string, startsAt, endsAt time.Time, key string) (adminAPIPromotionResponse, int) {
	item, status, _ := createAPIPromotionResponseForTest(server, session, serviceID, startsAt, endsAt, key)
	return item, status
}

func createAPIPromotionResponseForTest(server http.Handler, session testSession, serviceID string, startsAt, endsAt time.Time, key string) (adminAPIPromotionResponse, int, string) {
	body := fmt.Sprintf(`{"apiServiceId":%q,"placement":"api_market_top","startsAt":%q,"endsAt":%q,"reason":"管理员运营排期测试。"}`,
		serviceID,
		startsAt.Format(time.RFC3339),
		endsAt.Format(time.RFC3339),
	)
	request := newJSONRequest(http.MethodPost, "/api/v1/admin/api-service-promotions", body)
	addAuth(request, session, key)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		return adminAPIPromotionResponse{}, response.Code, response.Header().Get("ETag")
	}
	var item adminAPIPromotionResponse
	if err := json.NewDecoder(response.Body).Decode(&item); err != nil {
		return adminAPIPromotionResponse{}, http.StatusInternalServerError, response.Header().Get("ETag")
	}
	return item, response.Code, response.Header().Get("ETag")
}

func getAPIPromotionAvailabilityForTest(t *testing.T, server http.Handler, session testSession, serviceID string, startsAt, endsAt time.Time, key string) apiPromotionAvailabilityResponse {
	t.Helper()
	query := url.Values{
		"apiServiceId": {serviceID},
		"placement":    {"api_market_top"},
		"startsAt":     {startsAt.Format(time.RFC3339)},
		"endsAt":       {endsAt.Format(time.RFC3339)},
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/api-service-promotions/availability?"+query.Encode(), nil)
	addAuth(request, session, key)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("get API promotion availability status %d body %s", response.Code, response.Body.String())
	}
	var availability apiPromotionAvailabilityResponse
	if err := json.NewDecoder(response.Body).Decode(&availability); err != nil {
		t.Fatalf("decode API promotion availability: %v", err)
	}
	return availability
}

func listPublicAPIPromotionsForTest(t *testing.T, server http.Handler) []publicAPIPromotionResponse {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/api-service-promotions?placement=api_market_top", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("list public API promotions status %d body %s", response.Code, response.Body.String())
	}
	var payload listResponse[publicAPIPromotionResponse]
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("decode public API promotions: %v", err)
	}
	return payload.Items
}

func stopAPIPromotionForTest(t *testing.T, server http.Handler, session testSession, promotionID string, version int64, reason, key string) adminAPIPromotionResponse {
	t.Helper()
	item, _ := stopAPIPromotionResponseForTest(t, server, session, promotionID, version, reason, key)
	return item
}

func stopAPIPromotionResponseForTest(t *testing.T, server http.Handler, session testSession, promotionID string, version int64, reason, key string) (adminAPIPromotionResponse, string) {
	t.Helper()
	request := newJSONRequest(http.MethodPost, "/api/v1/admin/api-service-promotions/"+promotionID+"/stop", fmt.Sprintf(`{"reason":%q}`, reason))
	addAuth(request, session, key)
	request.Header.Set("If-Match", fmt.Sprintf(`"%d"`, version))
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("stop API promotion status %d body %s", response.Code, response.Body.String())
	}
	var item adminAPIPromotionResponse
	if err := json.NewDecoder(response.Body).Decode(&item); err != nil {
		t.Fatalf("decode stopped API promotion: %v", err)
	}
	if item.Status != "stopped" {
		t.Fatalf("expected stopped status, got %+v", item)
	}
	return item, response.Header().Get("ETag")
}
