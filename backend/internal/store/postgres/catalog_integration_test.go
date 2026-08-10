package postgres

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/catalog"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/platform/modelsdev"

	"github.com/google/uuid"
)

func TestAPIModelCatalogReadQueriesMatchCurrentSchema(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("C2C_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not configured")
	}
	store, err := Connect(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect PostgreSQL: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	if _, appErr := store.AdminListAPIModelProviders(ctx); appErr != nil {
		t.Fatalf("list API model providers: %v", appErr)
	}
	if _, appErr := store.AdminListAPIModels(ctx); appErr != nil {
		t.Fatalf("list administrator API models: %v", appErr)
	}
	if _, appErr := store.ListAPIModels(ctx); appErr != nil {
		t.Fatalf("list public API models: %v", appErr)
	}
}

func TestAPIModelCatalogSyncCommitsAndReplaysWithResourceIdentity(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	suffix := strings.ToLower(uuid.NewString()[:8])
	modelKey := "catalog-sync-" + suffix

	admin, appErr := store.EnsureUser(ctx, "catalog-sync-admin-"+suffix, true, now)
	if appErr != nil {
		t.Fatalf("ensure catalog sync administrator: %v", appErr)
	}
	var providerID string
	if err := store.pool.QueryRow(ctx, `SELECT id::text FROM api_model_providers WHERE code = 'openai'`).Scan(&providerID); err != nil {
		t.Fatalf("read OpenAI provider: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM idempotency_keys WHERE user_id = $1`, admin.ID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM api_model_price_versions WHERE model_catalog_id IN (SELECT id FROM api_model_catalog WHERE model_key = $1)`, modelKey)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM api_model_catalog WHERE model_key = $1`, modelKey)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM user_permissions WHERE user_id = $1`, admin.ID)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = $1`, admin.ID)
	})

	clock := func() time.Time { return now }
	source := catalogIntegrationModelsDevSource{catalog: modelsdev.Catalog{
		"openai": {
			ID: "openai", Name: "OpenAI",
			Models: map[string]modelsdev.Model{
				modelKey: {
					ID: modelKey, LastUpdated: "2026-08-08",
					Modalities: modelsdev.Modalities{Input: []string{"text"}, Output: []string{"text"}},
					Cost:       &modelsdev.Cost{Input: "0.4", CacheRead: "0.1", Output: "1.6"},
				},
			},
		},
	}}
	service := catalog.NewService(store, idempotency.NewService(store, clock), source, clock)
	preview, appErr := service.PreviewAPIModelSync(ctx, admin, catalog.APIModelSyncPreviewInput{ProviderIDs: []string{providerID}})
	if appErr != nil {
		t.Fatalf("preview catalog sync: %v", appErr)
	}
	var candidate catalog.APIModelSyncItem
	for _, item := range preview.Items {
		if item.ModelKey == modelKey {
			candidate = item
			break
		}
	}
	if candidate.Status != catalog.APIModelSyncStatusNew || candidate.Fingerprint == "" {
		t.Fatalf("expected new catalog sync candidate, got %+v", candidate)
	}

	applyInput := catalog.APIModelSyncApplyInput{Items: []catalog.APIModelSyncSelection{{
		Fingerprint: candidate.Fingerprint, Status: candidate.Status,
		ProviderID: candidate.ProviderID, ProviderCode: candidate.ProviderCode,
		ModelKey: candidate.ModelKey, Capabilities: candidate.Capabilities,
		SourceURL: candidate.SourceURL, SourceVersion: candidate.SourceVersion,
		InputPricePerMillion:       candidate.InputPricePerMillion,
		CachedInputPricePerMillion: candidate.CachedInputPricePerMillion,
		OutputPricePerMillion:      candidate.OutputPricePerMillion,
		Active:                     false,
	}}}
	completionBuilder := func(result catalog.APIModelBulkMutationResult) (idempotency.Completion, *domain.AppError) {
		if len(result.IDs) == 0 {
			return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "API 模型目录批量操作缺少关联资源。")
		}
		body, err := json.Marshal(result)
		if err != nil {
			return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "API 模型目录响应编码失败。")
		}
		return idempotency.Completion{
			Status: http.StatusOK, ContentType: "application/json; charset=utf-8", Body: body,
			ResourceType: "api_model_catalog", ResourceID: result.IDs[0],
		}, nil
	}
	routeKey := "POST /integration/admin/api-models/models-dev/apply"
	idempotencyKey := "catalog-sync-apply-" + suffix
	requestHash := "catalog-sync-hash-" + suffix
	completion, appErr := service.ApplyAPIModelSyncWithIdempotency(ctx, admin, routeKey, idempotencyKey, requestHash, applyInput, completionBuilder)
	if appErr != nil || completion.Status != http.StatusOK || completion.ResourceID == "" {
		t.Fatalf("apply catalog sync: completion=%+v error=%v", completion, appErr)
	}

	var active bool
	var modelCount, priceVersionCount int
	if err := store.pool.QueryRow(ctx, `
		SELECT active,
		       (SELECT count(*)::int FROM api_model_catalog WHERE model_key = $1),
		       (SELECT count(*)::int FROM api_model_price_versions WHERE model_catalog_id = api_model_catalog.id)
		FROM api_model_catalog
		WHERE id = $2
	`, modelKey, completion.ResourceID).Scan(&active, &modelCount, &priceVersionCount); err != nil {
		t.Fatalf("read committed catalog sync rows: %v", err)
	}
	if active || modelCount != 1 || priceVersionCount != 1 {
		t.Fatalf("unexpected committed catalog state active=%t models=%d prices=%d", active, modelCount, priceVersionCount)
	}

	var state, resourceType, resourceID string
	if err := store.pool.QueryRow(ctx, `
		SELECT status, resource_type, resource_id::text
		FROM idempotency_keys
		WHERE user_id = $1 AND route_key = $2 AND idempotency_key = $3
	`, admin.ID, routeKey, idempotencyKey).Scan(&state, &resourceType, &resourceID); err != nil {
		t.Fatalf("read completed catalog sync idempotency: %v", err)
	}
	if state != "completed" || resourceType != "api_model_catalog" || resourceID != completion.ResourceID {
		t.Fatalf("unexpected idempotency resource state=%s resource=%s:%s", state, resourceType, resourceID)
	}

	replay, appErr := service.ApplyAPIModelSyncWithIdempotency(ctx, admin, routeKey, idempotencyKey, requestHash, applyInput, completionBuilder)
	if appErr != nil || replay.ResourceID != completion.ResourceID || normalizedIntegrationJSON(replay.Body) != normalizedIntegrationJSON(completion.Body) {
		t.Fatalf("replay catalog sync: first=%s replay=%s completion=%+v error=%v", completion.Body, replay.Body, replay, appErr)
	}
	if err := store.pool.QueryRow(ctx, `
		SELECT count(*)::int,
		       (SELECT count(*)::int FROM api_model_price_versions WHERE model_catalog_id = api_model_catalog.id)
		FROM api_model_catalog
		WHERE model_key = $1
		GROUP BY api_model_catalog.id
	`, modelKey).Scan(&modelCount, &priceVersionCount); err != nil {
		t.Fatalf("count replayed catalog sync rows: %v", err)
	}
	if modelCount != 1 || priceVersionCount != 1 {
		t.Fatalf("catalog sync replay duplicated rows models=%d prices=%d", modelCount, priceVersionCount)
	}

	activated, appErr := store.AdminSetAPIModelActive(ctx, catalog.APIModelMutationInput{ID: completion.ResourceID}, true)
	if appErr != nil || !activated.Active || activated.ID != completion.ResourceID || activated.ModelKey != modelKey {
		t.Fatalf("activate synced API model: model=%+v error=%v", activated, appErr)
	}
	deactivated, appErr := store.AdminSetAPIModelActive(ctx, catalog.APIModelMutationInput{ID: completion.ResourceID}, false)
	if appErr != nil || deactivated.Active || deactivated.ID != completion.ResourceID || deactivated.ModelKey != modelKey {
		t.Fatalf("deactivate synced API model: model=%+v error=%v", deactivated, appErr)
	}
}

type catalogIntegrationModelsDevSource struct {
	catalog modelsdev.Catalog
}

func (s catalogIntegrationModelsDevSource) Fetch(context.Context) (modelsdev.Catalog, error) {
	return s.catalog, nil
}
