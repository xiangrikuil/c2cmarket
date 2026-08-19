package postgres

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/module/apimarket"

	"github.com/google/uuid"
)

func TestPostgresPublicAPIMarketDerivedSortsPaginate(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	suffix := strings.ReplaceAll(uuid.NewString()[:8], "-", "")

	ownerIDs := make([]string, 0, 4)
	serviceIDs := make([]string, 0, 4)
	for index := 0; index < 4; index++ {
		owner, appErr := store.EnsureUser(ctx, fmt.Sprintf("api-sort-%s-%d", suffix, index), false, now)
		if appErr != nil {
			t.Fatalf("ensure sort owner %d: %v", index, appErr)
		}
		ownerIDs = append(ownerIDs, owner.ID)
		serviceIDs = append(serviceIDs, seedPublicAPISortService(t, store, owner.ID, now, index))
	}

	t.Cleanup(func() {
		for _, ownerID := range ownerIDs {
			_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_quota_inventory_units WHERE batch_id IN (SELECT id FROM api_quota_batches WHERE owner_user_id = $1)`, ownerID)
			_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_quota_allocations WHERE owner_user_id = $1`, ownerID)
			_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_quota_sale_rounds WHERE owner_user_id = $1`, ownerID)
			_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_quota_offers WHERE owner_user_id = $1`, ownerID)
			_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_quota_batches WHERE owner_user_id = $1`, ownerID)
			_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_service_payment_options WHERE api_service_id IN (SELECT id FROM api_services WHERE owner_user_id = $1)`, ownerID)
			_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_services WHERE owner_user_id = $1`, ownerID)
			_, _ = store.pool.Exec(context.Background(), `DELETE FROM user_reputation_states WHERE user_id = $1`, ownerID)
			_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_probe_connections WHERE owner_user_id = $1`, ownerID)
			_, _ = store.pool.Exec(context.Background(), `UPDATE contact_methods SET current_version_id = NULL WHERE user_id = $1`, ownerID)
			_, _ = store.pool.Exec(context.Background(), `DELETE FROM contact_method_versions WHERE owner_user_id = $1`, ownerID)
			_, _ = store.pool.Exec(context.Background(), `DELETE FROM contact_methods WHERE user_id = $1`, ownerID)
			_, _ = store.pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, ownerID)
		}
	})

	for _, sortMode := range []string{
		apimarket.PublicServiceSortRecommended,
		apimarket.PublicServiceSortReputationDesc,
		apimarket.PublicServiceSortCompletedDesc,
		apimarket.PublicServiceSortResponseFast,
	} {
		t.Run(sortMode, func(t *testing.T) {
			var rawSortValue string
			if err := store.pool.QueryRow(context.Background(), `SELECT `+apiServiceSortExpression("api_services", sortMode)+` FROM api_services LIMIT 1`).Scan(&rawSortValue); err != nil {
				t.Fatalf("evaluate service sort expression: %v", err)
			}
			items := collectPublicAPIServiceIntegrationPages(t, store, apimarket.PublicServiceFilter{Sort: sortMode}, 2)
			assertPublicAPIServicePageCoverage(t, items, serviceIDs)
			for index := 1; index < len(items); index++ {
				previous := items[index-1]
				current := items[index]
				comparison := integrationDecimal(t, previous.PublicSortValue).Cmp(integrationDecimal(t, current.PublicSortValue))
				if comparison > 0 || comparison == 0 && previous.ID >= current.ID {
					t.Fatalf("derived service keyset order broke at %d: %+v then %+v", index, previous, current)
				}
			}
			if sortMode == apimarket.PublicServiceSortRecommended || sortMode == apimarket.PublicServiceSortReputationDesc {
				lastValue := integrationDecimal(t, items[len(items)-1].PublicSortValue)
				firstValue := integrationDecimal(t, items[0].PublicSortValue)
				missingDataLast := lastValue.Cmp(firstValue) > 0
				if sortMode == apimarket.PublicServiceSortReputationDesc {
					missingDataLast = lastValue.Cmp(integrationDecimal(t, "1000000000000000")) == 0
				}
				if items[0].ID != serviceIDs[0] || !missingDataLast {
					t.Fatalf("reputation ordering did not put high-trust data first and missing data last: first=%s(%s) last=%s(%s)", items[0].ID, items[0].PublicSortValue, items[len(items)-1].ID, items[len(items)-1].PublicSortValue)
				}
			}
		})
	}
}

func TestPostgresPublicAPIQuotaDerivedSortsPaginate(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	suffix := strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	owner, appErr := store.EnsureUser(ctx, "api-quota-sort-"+suffix, false, now)
	if appErr != nil {
		t.Fatalf("ensure quota sort owner: %v", appErr)
	}
	serviceID := seedPublicAPISortService(t, store, owner.ID, now, 3)
	t.Cleanup(func() {
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_quota_inventory_units WHERE batch_id IN (SELECT id FROM api_quota_batches WHERE owner_user_id = $1)`, owner.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_quota_allocations WHERE owner_user_id = $1`, owner.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_quota_sale_rounds WHERE owner_user_id = $1`, owner.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_quota_offers WHERE owner_user_id = $1`, owner.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_quota_batches WHERE owner_user_id = $1`, owner.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_service_payment_options WHERE api_service_id = $1`, serviceID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_services WHERE id = $1`, serviceID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_probe_connections WHERE owner_user_id = $1`, owner.ID)
		_, _ = store.pool.Exec(context.Background(), `UPDATE contact_methods SET current_version_id = NULL WHERE user_id = $1`, owner.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM contact_method_versions WHERE owner_user_id = $1`, owner.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM contact_methods WHERE user_id = $1`, owner.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, owner.ID)
	})

	assertPublicQuotaFiltersBeforePagination(t, store, owner.ID, serviceID, now)
}

func TestPostgresPublicAPIPackageDiscoveryFiltersBeforePagination(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	suffix := strings.ReplaceAll(uuid.NewString()[:8], "-", "")

	type catalogModel struct {
		id       string
		modelKey string
		provider string
	}
	catalogModels := make([]catalogModel, 0, 3)
	rows, err := store.pool.Query(ctx, `
		SELECT model.id::text, model.model_key, provider.display_name
		FROM api_model_catalog model
		JOIN api_model_providers provider ON provider.id = model.provider_id
		WHERE model.status = 'active' AND provider.status = 'active'
		ORDER BY model.sort_order, model.id
		LIMIT 3
	`)
	if err != nil {
		t.Fatalf("query package discovery catalog models: %v", err)
	}
	for rows.Next() {
		var model catalogModel
		if err := rows.Scan(&model.id, &model.modelKey, &model.provider); err != nil {
			rows.Close()
			t.Fatalf("scan package discovery catalog model: %v", err)
		}
		catalogModels = append(catalogModels, model)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate package discovery catalog rows: %v", err)
	}
	if len(catalogModels) != 3 {
		t.Fatalf("package discovery test requires three active catalog models, got %d", len(catalogModels))
	}

	ownerIDs := make([]string, 0, 4)
	serviceIDs := make([]string, 0, 4)
	for index := 0; index < 4; index++ {
		owner, appErr := store.EnsureUser(ctx, fmt.Sprintf("api-package-discovery-%s-%d", suffix, index), false, now)
		if appErr != nil {
			t.Fatalf("ensure package discovery owner %d: %v", index, appErr)
		}
		ownerIDs = append(ownerIDs, owner.ID)
		serviceID := seedPublicAPISortService(t, store, owner.ID, now, index)
		serviceIDs = append(serviceIDs, serviceID)
		if _, err := store.pool.Exec(ctx, `
			UPDATE api_services
			SET billing_mode = 'fixed_package', usage_visibility = 'fixed_package_only',
			    available_usd_allowance = NULL
			WHERE id = $1
		`, serviceID); err != nil {
			t.Fatalf("mark package discovery service %d fixed: %v", index, err)
		}
	}
	t.Cleanup(func() {
		for _, ownerID := range ownerIDs {
			_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_service_payment_options WHERE api_service_id IN (SELECT id FROM api_services WHERE owner_user_id = $1)`, ownerID)
			_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_services WHERE owner_user_id = $1`, ownerID)
			_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_probe_connections WHERE owner_user_id = $1`, ownerID)
			_, _ = store.pool.Exec(context.Background(), `UPDATE contact_methods SET current_version_id = NULL WHERE user_id = $1`, ownerID)
			_, _ = store.pool.Exec(context.Background(), `DELETE FROM contact_method_versions WHERE owner_user_id = $1`, ownerID)
			_, _ = store.pool.Exec(context.Background(), `DELETE FROM contact_methods WHERE user_id = $1`, ownerID)
			_, _ = store.pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, ownerID)
		}
	})

	seedPackage := func(serviceIndex int, models []catalogModel, enabled []bool, stock int) {
		t.Helper()
		packageID := uuid.NewString()
		if _, err := store.pool.Exec(ctx, `
			INSERT INTO api_service_packages (
				id, api_service_id, name, price_cny, duration_days, description,
				panel_allowance, stock_total, stock_available, enabled, created_at, updated_at
			) VALUES ($1, $2, $3, 10, 7, '公开套餐发现集成测试', 10, 2, $4, true, $5, $5)
		`, packageID, serviceIDs[serviceIndex], fmt.Sprintf("公开套餐 %d", serviceIndex), stock, now); err != nil {
			t.Fatalf("seed discovery package %d: %v", serviceIndex, err)
		}
		for modelIndex, model := range models {
			serviceModelID := uuid.NewString()
			if _, err := store.pool.Exec(ctx, `
				INSERT INTO api_service_models (
					id, api_service_id, distribution_system, model_catalog_id,
					model_key_snapshot, provider_snapshot, merchant_multiplier,
					enabled, created_at, updated_at
				) VALUES ($1, $2, 'sub2api', $3, $4, $5, 1, $6, $7, $7)
			`, serviceModelID, serviceIDs[serviceIndex], model.id, model.modelKey, model.provider, enabled[modelIndex], now); err != nil {
				t.Fatalf("seed discovery service model %d/%d: %v", serviceIndex, modelIndex, err)
			}
			if _, err := store.pool.Exec(ctx, `
				INSERT INTO api_service_package_models (
					api_service_package_id, api_service_model_id, api_service_id, created_at
				) VALUES ($1, $2, $3, $4)
			`, packageID, serviceModelID, serviceIDs[serviceIndex], now); err != nil {
				t.Fatalf("seed discovery package model %d/%d: %v", serviceIndex, modelIndex, err)
			}
		}
	}
	seedPackage(0, []catalogModel{catalogModels[0]}, []bool{false}, 2)
	seedPackage(1, catalogModels[:2], []bool{true, true}, 2)
	seedPackage(2, []catalogModel{catalogModels[2]}, []bool{true}, 0)
	seedPackage(3, []catalogModel{catalogModels[0]}, []bool{true}, 2)

	items := collectPublicAPIServiceIntegrationPages(t, store, apimarket.PublicServiceFilter{
		BillingMode: apimarket.ServiceBillingModeFixedPackage,
		PackageModelCatalogIDs: []string{
			catalogModels[0].id,
			catalogModels[1].id,
		},
	}, 1)
	assertPublicAPIServicePageCoverage(t, items, []string{serviceIDs[1], serviceIDs[3]})

	availability, appErr := store.ListPublicAPIPackageFilterAvailability(ctx)
	if appErr != nil {
		t.Fatalf("list package filter availability: %v", appErr)
	}
	availableModels := make(map[string]struct{}, len(availability.Facts))
	availableDurations := make(map[int]struct{}, len(availability.Facts))
	for _, fact := range availability.Facts {
		availableModels[fact.ModelCatalogID] = struct{}{}
		availableDurations[fact.DurationDays] = struct{}{}
	}
	for _, model := range catalogModels[:2] {
		if _, ok := availableModels[model.id]; !ok {
			t.Fatalf("available package model %s is missing from filter options: %+v", model.id, availability)
		}
	}
	if _, ok := availableModels[catalogModels[2].id]; ok {
		t.Fatalf("sold-out-only model must be absent from filter options: %+v", availability)
	}
	if len(availableDurations) != 1 {
		t.Fatalf("unexpected package duration options: %+v", availability.Facts)
	}

	if _, appErr := store.GetPublicAPIService(ctx, serviceIDs[2]); appErr == nil || appErr.Status != http.StatusNotFound {
		t.Fatalf("sold-out package detail must be hidden, got %+v", appErr)
	}
	if _, appErr := store.GetPublicAPIService(ctx, serviceIDs[0]); appErr == nil || appErr.Status != 404 {
		t.Fatalf("package backed only by disabled service models must be hidden, got %+v", appErr)
	}
}

func seedPublicAPISortService(t *testing.T, store *Store, ownerID string, now time.Time, index int) string {
	t.Helper()
	ctx := context.Background()
	contactID := uuid.NewString()
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO contact_methods (id, user_id, type, label, usage_scopes, is_default, enabled, created_at, updated_at)
		VALUES ($1, $2, 'linuxdo', 'linux.do', ARRAY['api_merchant']::text[], true, true, $3, $3)
	`, contactID, ownerID, now); err != nil {
		t.Fatalf("seed sort contact %d: %v", index, err)
	}
	seedContactVersionForTest(t, ctx, store.pool, contactID, ownerID, now)
	connectionID := uuid.NewString()
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO api_probe_connections (
			id, owner_user_id, name, base_url, normalized_base_url,
			credential_ciphertext, credential_nonce, credential_key_version,
			credential_cipher_format, credential_fingerprint,
			probe_model, probe_protocol, enabled, verification_status, verified_at,
			measurement_version, version, created_at, updated_at
		) VALUES (
			$1, $2, $3, 'https://api-sort.example.com/v1', 'https://api-sort.example.com/v1',
			decode('0102', 'hex'), decode('000000000000000000000000', 'hex'), 'test-v1',
			'test-v1', decode('0304', 'hex'), 'gpt-5-mini', 'openai_responses_v1',
			true, 'verified', $4, 1, 1, $4, $4
		)
	`, connectionID, ownerID, fmt.Sprintf("排序探针 %d", index), now); err != nil {
		t.Fatalf("seed sort probe %d: %v", index, err)
	}
	serviceID := uuid.NewString()
	updatedAt := now.Add(-time.Duration(index) * time.Minute)
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO api_services (
			id, owner_user_id, merchant_identity_mode, owner_contact_method_id,
			title, short_description, distribution_system, billing_mode,
			declared_cny_per_usd_allowance, declared_max_usd_allowance_per_intent,
			available_usd_allowance, quota_expires_at, minimum_intent_cny,
			maximum_intent_cny, usage_visibility, review_status, publication_status,
			moderation_status, accepting_orders, payment_window_minutes,
			declared_ttft_band, declared_max_concurrency, performance_confirmed_at,
			prompt_audit_enabled, probe_connection_id, created_at, updated_at, version
		) VALUES (
			$1, $2, 'public_profile', $3, $4, '排序分页集成夹具', 'sub2api',
			'metered_usd_quota', 1, 1000, 1000, $5, 1, 1000,
			'offsite_panel_readonly', 'approved', 'online', 'clear', true, 10,
			'under_1s', 20, $6, false, $7, $6, $8, 1
		)
	`, serviceID, ownerID, contactID, fmt.Sprintf("排序 API 服务 %d", index), now.Add(30*24*time.Hour), now, connectionID, updatedAt); err != nil {
		t.Fatalf("seed sort service %d: %v", index, err)
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO api_service_payment_options (id, api_service_id, payment_method, enabled, payment_instructions, created_at, updated_at, version)
		VALUES ($1, $2, 'wechat', true, '集成测试付款说明', $3, $3, 1)
	`, uuid.NewString(), serviceID, now); err != nil {
		t.Fatalf("seed sort payment option %d: %v", index, err)
	}
	if index == 0 || index == 1 {
		tier := "high_trust"
		if index == 1 {
			tier = "normal"
		}
		if _, err := store.pool.Exec(ctx, `
			INSERT INTO user_reputation_states (
				user_id, role, scope, tier, state, confidence, rule_version, metrics_json,
				warnings_json, badges_json, progress_json, tier_entered_at, state_entered_at,
				calculated_at
			) VALUES ($1, 'seller', 'api', $2, 'active', 'high', 'sort-test-v1',
				'{"weightedRating": 4.8, "verifiedReviewCount": 10}'::jsonb,
				'[]'::jsonb, '[]'::jsonb, '[]'::jsonb, $3, $3, $3)
		`, ownerID, tier, now); err != nil {
			t.Fatalf("seed sort reputation %d: %v", index, err)
		}
	}
	return serviceID
}
