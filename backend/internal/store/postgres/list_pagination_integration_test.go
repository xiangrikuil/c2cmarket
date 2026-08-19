package postgres

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/apiquota"
	"c2c-market/backend/internal/module/carpool"

	"github.com/google/uuid"
)

func TestPostgresBusinessListFiltersAndSortsBeforePagination(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	suffix := strings.ReplaceAll(uuid.NewString()[:8], "-", "")

	owner, appErr := store.EnsureUser(ctx, "pagination-"+suffix, false, now)
	if appErr != nil {
		t.Fatalf("ensure pagination owner: %v", appErr)
	}
	contactID := uuid.NewString()
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO contact_methods (id, user_id, type, label, usage_scopes, is_default, enabled, created_at, updated_at)
		VALUES ($1, $2, 'linuxdo', 'linux.do', ARRAY['carpool_owner', 'api_merchant', 'buyer', 'dispute']::text[], true, true, $3, $3)
	`, contactID, owner.ID, now); err != nil {
		t.Fatalf("seed pagination contact: %v", err)
	}
	seedContactVersionForTest(t, ctx, store.pool, contactID, owner.ID, now)
	t.Cleanup(func() {
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_quota_inventory_units WHERE batch_id IN (SELECT id FROM api_quota_batches WHERE owner_user_id = $1)`, owner.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_quota_allocations WHERE owner_user_id = $1`, owner.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_quota_sale_rounds WHERE owner_user_id = $1`, owner.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_quota_offers WHERE owner_user_id = $1`, owner.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_quota_batches WHERE owner_user_id = $1`, owner.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_service_payment_options WHERE api_service_id IN (SELECT id FROM api_services WHERE owner_user_id = $1)`, owner.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_services WHERE owner_user_id = $1`, owner.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_probe_connections WHERE owner_user_id = $1`, owner.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM carpool_listings WHERE owner_user_id = $1`, owner.ID)
		_, _ = store.pool.Exec(context.Background(), `UPDATE contact_methods SET current_version_id = NULL WHERE user_id = $1`, owner.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM contact_method_versions WHERE owner_user_id = $1`, owner.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM contact_methods WHERE user_id = $1`, owner.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, owner.ID)
	})

	var productPlanID string
	if err := store.pool.QueryRow(ctx, `SELECT id::text FROM product_plans ORDER BY created_at, id LIMIT 1`).Scan(&productPlanID); err != nil {
		t.Fatalf("read seeded product plan: %v", err)
	}

	listingIDs := make([]string, 0, 28)
	for index := 0; index < 28; index++ {
		listingID := uuid.NewString()
		listingIDs = append(listingIDs, listingID)
		status := carpool.ListingStatusActive
		title := fmt.Sprintf("分页车源 %02d", index)
		if index >= 25 {
			status = carpool.ListingStatusPaused
			title = fmt.Sprintf("较旧异常车源 %02d", index)
		}
		updatedAt := now.Add(-time.Duration(index) * time.Minute)
		if _, err := store.pool.Exec(ctx, `
			INSERT INTO carpool_listings (
				id, owner_user_id, product_plan_id, owner_contact_method_id,
				title, summary, access_arrangement,
				distribution_method, distribution_method_note, provides_admin_account,
				region_code, region_name, price_monthly_cny, service_multiplier,
				daily_quota_amount, weekly_quota_amount, follows_official_quota_reset,
				vps_region, supports_mainland_china_direct_connection,
				opening_channel_code, payment_method_code,
				quota_label, quota_unit, quota_period,
				buyer_seat_capacity, active_buyer_members, status,
				policy_version, risk_ack_required, created_at, updated_at, version
			) VALUES (
				$1, $2, $3, $4,
				$5, 'PostgreSQL pagination integration fixture', '站外确认',
				'other', '站外确认', false,
				'us', '美国区', $6, 1,
				10, 100, true,
				'美国', true,
				'web', 'credit_card',
				'额度', 'USD', 'monthly',
				$7, $8, $9,
				1, false, $10, $10, 1
			)
		`, listingID, owner.ID, productPlanID, contactID, title,
			fmt.Sprintf("%d.00", index%7+1), 3+index%5, index%2, status, updatedAt); err != nil {
			t.Fatalf("seed carpool listing %d: %v", index, err)
		}
	}

	exceptions, appErr := store.ListAdminCarpoolListings(ctx, carpool.ListingFilter{View: carpool.ListingViewExceptions}, domain.PageRequest{Limit: 20})
	if appErr != nil {
		t.Fatalf("list carpool exceptions: %v", appErr)
	}
	if len(exceptions.Items) != 3 || exceptions.NextCursor != nil {
		t.Fatalf("expected all older exceptions on the first filtered page, got %+v", exceptions)
	}

	searchPage, appErr := store.ListAdminCarpoolListings(ctx, carpool.ListingFilter{Query: "较旧异常车源 27"}, domain.PageRequest{Limit: 1})
	if appErr != nil || len(searchPage.Items) != 1 || searchPage.Items[0].Title != "较旧异常车源 27" {
		t.Fatalf("search must run before pagination: page=%+v error=%v", searchPage, appErr)
	}

	priceItems := collectCarpoolIntegrationPages(t, store, carpool.ListingFilter{Sort: carpool.ListingSortPriceAsc}, 6)
	assertCarpoolPageCoverage(t, priceItems, listingIDs)
	for index := 1; index < len(priceItems); index++ {
		left := integrationDecimal(t, priceItems[index-1].PriceMonthlyCNY)
		right := integrationDecimal(t, priceItems[index].PriceMonthlyCNY)
		if comparison := left.Cmp(right); comparison > 0 || comparison == 0 && priceItems[index-1].ID >= priceItems[index].ID {
			t.Fatalf("price keyset order broke at %d: %+v then %+v", index, priceItems[index-1], priceItems[index])
		}
	}

	seatItems := collectCarpoolIntegrationPages(t, store, carpool.ListingFilter{Sort: carpool.ListingSortSeatsDesc}, 5)
	assertCarpoolPageCoverage(t, seatItems, listingIDs)
	for index := 1; index < len(seatItems); index++ {
		left := seatItems[index-1]
		right := seatItems[index]
		if left.AvailableSeats < right.AvailableSeats || left.AvailableSeats == right.AvailableSeats && left.ID <= right.ID {
			t.Fatalf("seat keyset order broke at %d: %+v then %+v", index, left, right)
		}
	}

	serviceIDs := make([]string, 0, 27)
	for index := 0; index < 27; index++ {
		serviceID := uuid.NewString()
		serviceIDs = append(serviceIDs, serviceID)
		reviewStatus := apimarket.ServiceReviewStatusApproved
		publicationStatus := apimarket.ServicePublicationStatusOnline
		moderationStatus := apimarket.ServiceModerationStatusClear
		title := fmt.Sprintf("分页 API 服务 %02d", index)
		if index >= 25 {
			moderationStatus = apimarket.ServiceModerationStatusAdminSuspended
			title = fmt.Sprintf("较旧异常 API 服务 %02d", index)
		}
		updatedAt := now.Add(-time.Duration(index) * time.Minute)
		if _, err := store.pool.Exec(ctx, `
			INSERT INTO api_services (
				id, owner_user_id, merchant_identity_mode, owner_contact_method_id,
				title, short_description, distribution_system, billing_mode,
				declared_cny_per_usd_allowance, declared_max_usd_allowance_per_intent,
				available_usd_allowance, quota_expires_at,
				minimum_intent_cny, maximum_intent_cny, usage_visibility,
				review_status, publication_status, moderation_status,
				accepting_orders, payment_window_minutes,
				declared_ttft_band, declared_max_concurrency, performance_confirmed_at,
				prompt_audit_enabled, created_at, updated_at, version
			) VALUES (
				$1, $2, 'public_profile', $3,
				$4, 'PostgreSQL pagination integration fixture', 'sub2api', 'metered_usd_quota',
				1, 1000, 1000, $5,
				1, 1000, 'offsite_panel_readonly',
				$6, $7, $8,
				true, 10,
				'under_1s', 20, $9,
				false, $9, $9, 1
			)
		`, serviceID, owner.ID, contactID, title, now.AddDate(0, 1, 0),
			reviewStatus, publicationStatus, moderationStatus, updatedAt); err != nil {
			t.Fatalf("seed API service %d: %v", index, err)
		}
	}

	serviceExceptions, appErr := store.ListAdminAPIServices(ctx, apimarket.AdminServiceFilter{View: apimarket.AdminServiceViewExceptions}, domain.PageRequest{Limit: 20})
	if appErr != nil {
		t.Fatalf("list API service exceptions: %v", appErr)
	}
	if len(serviceExceptions.Items) != 2 || serviceExceptions.NextCursor != nil {
		t.Fatalf("expected all older API exceptions on the first filtered page, got %+v", serviceExceptions)
	}

	serviceSearch, appErr := store.ListAdminAPIServices(ctx, apimarket.AdminServiceFilter{Query: "较旧异常 API 服务 26"}, domain.PageRequest{Limit: 1})
	if appErr != nil || len(serviceSearch.Items) != 1 || serviceSearch.Items[0].Title != "较旧异常 API 服务 26" {
		t.Fatalf("API service search must run before pagination: page=%+v error=%v", serviceSearch, appErr)
	}

	allServices := collectAPIServiceIntegrationPages(t, store, apimarket.AdminServiceFilter{}, 7)
	if len(allServices) != len(serviceIDs) {
		t.Fatalf("API service page coverage = %d, want %d", len(allServices), len(serviceIDs))
	}
	seenServices := make(map[string]struct{}, len(allServices))
	for index, service := range allServices {
		if _, exists := seenServices[service.ID]; exists {
			t.Fatalf("duplicate API service across page boundary: %s", service.ID)
		}
		seenServices[service.ID] = struct{}{}
		if index > 0 {
			previous := allServices[index-1]
			if previous.UpdatedAt.Before(service.UpdatedAt) || previous.UpdatedAt.Equal(service.UpdatedAt) && previous.ID <= service.ID {
				t.Fatalf("API service keyset order broke at %d: %+v then %+v", index, previous, service)
			}
		}
	}

	fixedServiceID := serviceIDs[24]
	modelCatalogID := seedPublicServicePaginationFilters(t, store, owner.ID, serviceIDs, now)
	assertPublicServiceFiltersBeforePagination(t, store, fixedServiceID, modelCatalogID)
	assertPublicServiceSortPagination(t, store, serviceIDs[:20], serviceIDs[20:25], modelCatalogID)
	assertPublicQuotaFiltersBeforePagination(t, store, owner.ID, serviceIDs[0], now)
	assertAdminAPIOrderFiltersAndPagination(t, store, owner.ID, contactID, serviceIDs[0], now)
}

func assertAdminAPIOrderFiltersAndPagination(t *testing.T, store *Store, sellerID, sellerContactID, serviceID string, now time.Time) {
	t.Helper()
	ctx := context.Background()
	suffix := strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	buyer, appErr := store.EnsureUser(ctx, "order-pagination-buyer-"+suffix, false, now)
	if appErr != nil {
		t.Fatalf("ensure order pagination buyer: %v", appErr)
	}
	buyerContactID := uuid.NewString()
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO contact_methods (id, user_id, type, label, usage_scopes, is_default, enabled, created_at, updated_at)
		VALUES ($1, $2, 'linuxdo', 'linux.do', ARRAY['buyer']::text[], true, true, $3, $3)
	`, buyerContactID, buyer.ID, now); err != nil {
		t.Fatalf("seed order pagination buyer contact: %v", err)
	}
	seedContactVersionForTest(t, ctx, store.pool, buyerContactID, buyer.ID, now)
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO api_service_access_modes (api_service_id, access_mode, public_note)
		VALUES ($1, 'buyer_dedicated_sub_key', '站外确认')
		ON CONFLICT (api_service_id, access_mode) DO NOTHING
	`, serviceID); err != nil {
		t.Fatalf("seed order pagination access mode: %v", err)
	}

	var buyerContactVersionID, sellerContactVersionID string
	if err := store.pool.QueryRow(ctx, `SELECT current_version_id::text FROM contact_methods WHERE id = $1`, buyerContactID).Scan(&buyerContactVersionID); err != nil {
		t.Fatalf("read buyer contact version: %v", err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT current_version_id::text FROM contact_methods WHERE id = $1`, sellerContactID).Scan(&sellerContactVersionID); err != nil {
		t.Fatalf("read seller contact version: %v", err)
	}
	t.Cleanup(func() {
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_orders WHERE buyer_user_id = $1`, buyer.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM api_purchase_intents WHERE buyer_user_id = $1`, buyer.ID)
		_, _ = store.pool.Exec(context.Background(), `UPDATE contact_methods SET current_version_id = NULL WHERE user_id = $1`, buyer.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM contact_method_versions WHERE owner_user_id = $1`, buyer.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM contact_methods WHERE user_id = $1`, buyer.ID)
		_, _ = store.pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, buyer.ID)
	})

	orderIDs := make([]string, 0, 27)
	var deepOrderNo, deepAmount string
	for index := 0; index < 27; index++ {
		intentID := uuid.NewString()
		orderID := uuid.NewString()
		createdAt := now.Add(-time.Duration(index) * time.Minute)
		amount := fmt.Sprintf("%d.00", index%5+1)
		if _, err := store.pool.Exec(ctx, `
			INSERT INTO api_purchase_intents (
				id, api_service_id, api_service_owner_user_id, buyer_user_id, owner_user_id,
				buyer_contact_method_id, buyer_contact_method_version_id,
				owner_contact_method_id, owner_contact_method_version_id,
				status, requested_cny_amount, requested_usd_allowance, selected_access_mode,
				service_version_snapshot, service_title_snapshot,
				distribution_system_snapshot, billing_mode_snapshot,
				buyer_contact_type_snapshot, buyer_contact_label_snapshot,
				owner_contact_type_snapshot, owner_contact_label_snapshot,
				minimum_intent_cny_snapshot, pricing_snapshot, contacted_at, created_at, updated_at
			) VALUES (
				$1, $2, $3, $4, $3,
				$5, $6, $7, $8,
				'ordered', $9, 20, 'buyer_dedicated_sub_key',
				1, $10, 'sub2api', 'metered_usd_quota',
				'linuxdo', 'linux.do', 'linuxdo', 'linux.do',
				1, '{}'::jsonb, $11, $11, $11
			)
		`, intentID, serviceID, sellerID, buyer.ID, buyerContactID, buyerContactVersionID,
			sellerContactID, sellerContactVersionID, amount, fmt.Sprintf("监管分页 API 服务 %02d", index), createdAt); err != nil {
			t.Fatalf("seed order pagination intent %d: %v", index, err)
		}
		orderNo, err := apiorder.GenerateOrderNo(createdAt)
		if err != nil {
			t.Fatalf("generate order pagination number %d: %v", index, err)
		}
		if _, err := store.pool.Exec(ctx, `
			INSERT INTO api_orders (
				id, api_purchase_intent_id, api_service_id, buyer_user_id, seller_user_id,
				status, dispute_status, service_title_snapshot, service_version_snapshot,
				billing_mode_snapshot, requested_usd_allowance_snapshot, cny_per_usd_allowance_snapshot,
				amount, currency, selected_payment_method,
				payment_window_minutes_snapshot, payment_expires_at,
				payment_instructions_snapshot, created_at, updated_at, order_no
			) VALUES (
				$1, $2, $3, $4, $5,
				'pending_payment', 'none', $6, 1,
				'metered_usd_quota', 20, 1,
				$7, 'CNY', 'wechat',
				10, $8, '站外确认付款', $9, $9, $10
			)
		`, orderID, intentID, serviceID, buyer.ID, sellerID, fmt.Sprintf("监管分页 API 服务 %02d", index), amount, now.Add(2*time.Hour), createdAt, orderNo); err != nil {
			t.Fatalf("seed order pagination order %d: %v", index, err)
		}
		orderIDs = append(orderIDs, orderID)
		if index == 26 {
			deepOrderNo = orderNo
			deepAmount = amount
		}
	}

	first, appErr := store.ListAdminAPIOrders(ctx, apiorder.AdminOrderFilter{}, domain.PageRequest{Limit: 20}, now)
	if appErr != nil || len(first.Items) != 20 || first.NextCursor == nil {
		t.Fatalf("admin order first page: page=%+v error=%v", first, appErr)
	}
	for _, order := range first.Items {
		if order.OrderNo == deepOrderNo {
			t.Fatalf("deep order unexpectedly appeared on first page: %s", deepOrderNo)
		}
	}
	deepQuery := strings.ToLower(strings.ReplaceAll(deepOrderNo, "-", ""))
	deepPage, appErr := store.ListAdminAPIOrders(ctx, apiorder.AdminOrderFilter{
		Query:        deepQuery,
		Statuses:     []string{apiorder.StatusPendingPayment},
		BuyerUserID:  buyer.ID,
		SellerUserID: sellerID,
		APIServiceID: serviceID,
		Dispute:      apiorder.AdminOrderDisputeNone,
		MinAmount:    deepAmount,
		MaxAmount:    deepAmount,
	}, domain.PageRequest{Limit: 1}, now)
	if appErr != nil || len(deepPage.Items) != 1 || deepPage.Items[0].OrderNo != deepOrderNo || deepPage.NextCursor != nil {
		t.Fatalf("admin order filters must run before pagination: page=%+v error=%v", deepPage, appErr)
	}

	for _, sortMode := range []string{
		apiorder.AdminOrderSortUpdatedDesc,
		apiorder.AdminOrderSortCreatedDesc,
		apiorder.AdminOrderSortAmountDesc,
		apiorder.AdminOrderSortAmountAsc,
	} {
		items := collectAdminAPIOrderIntegrationPages(t, store, apiorder.AdminOrderFilter{BuyerUserID: buyer.ID, Sort: sortMode}, 6, now)
		assertAdminAPIOrderPageCoverage(t, items, orderIDs, sortMode)
	}

	amountFirst, appErr := store.ListAdminAPIOrders(ctx, apiorder.AdminOrderFilter{BuyerUserID: buyer.ID, Sort: apiorder.AdminOrderSortAmountAsc}, domain.PageRequest{Limit: 1}, now)
	if appErr != nil || amountFirst.NextCursor == nil {
		t.Fatalf("admin amount cursor precondition: page=%+v error=%v", amountFirst, appErr)
	}
	if _, appErr := store.ListAdminAPIOrders(ctx, apiorder.AdminOrderFilter{BuyerUserID: buyer.ID, Sort: apiorder.AdminOrderSortCreatedDesc}, domain.PageRequest{Limit: 1, Cursor: *amountFirst.NextCursor}, now); appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("sort-mismatched admin order cursor error = %+v", appErr)
	}
}

func collectAdminAPIOrderIntegrationPages(t *testing.T, store *Store, filter apiorder.AdminOrderFilter, limit int, now time.Time) []apiorder.Order {
	t.Helper()
	var items []apiorder.Order
	var cursor string
	for {
		page, appErr := store.ListAdminAPIOrders(context.Background(), filter, domain.PageRequest{Limit: limit, Cursor: cursor}, now)
		if appErr != nil {
			t.Fatalf("collect admin API order page: %v", appErr)
		}
		items = append(items, page.Items...)
		if page.NextCursor == nil {
			return items
		}
		if *page.NextCursor == cursor {
			t.Fatalf("admin API order cursor repeated: %q", cursor)
		}
		cursor = *page.NextCursor
	}
}

func assertAdminAPIOrderPageCoverage(t *testing.T, items []apiorder.Order, expectedIDs []string, sortMode string) {
	t.Helper()
	if len(items) != len(expectedIDs) {
		t.Fatalf("admin API order page coverage = %d, want %d for %s", len(items), len(expectedIDs), sortMode)
	}
	seen := make(map[string]struct{}, len(items))
	for index, order := range items {
		if _, exists := seen[order.ID]; exists {
			t.Fatalf("duplicate admin API order across %s page boundary: %s", sortMode, order.ID)
		}
		seen[order.ID] = struct{}{}
		if index == 0 {
			continue
		}
		previous := items[index-1]
		switch sortMode {
		case apiorder.AdminOrderSortAmountAsc:
			comparison := apiorder.CompareAdminOrderAmounts(previous.Amount, order.Amount)
			if comparison > 0 || comparison == 0 && previous.ID >= order.ID {
				t.Fatalf("amount ascending keyset order broke at %d: %+v then %+v", index, previous, order)
			}
		case apiorder.AdminOrderSortAmountDesc:
			comparison := apiorder.CompareAdminOrderAmounts(previous.Amount, order.Amount)
			if comparison < 0 || comparison == 0 && previous.ID <= order.ID {
				t.Fatalf("amount descending keyset order broke at %d: %+v then %+v", index, previous, order)
			}
		case apiorder.AdminOrderSortCreatedDesc:
			if previous.CreatedAt.Before(order.CreatedAt) || previous.CreatedAt.Equal(order.CreatedAt) && previous.ID <= order.ID {
				t.Fatalf("created descending keyset order broke at %d: %+v then %+v", index, previous, order)
			}
		default:
			if previous.UpdatedAt.Before(order.UpdatedAt) || previous.UpdatedAt.Equal(order.UpdatedAt) && previous.ID <= order.ID {
				t.Fatalf("updated descending keyset order broke at %d: %+v then %+v", index, previous, order)
			}
		}
	}
}

func seedPublicServicePaginationFilters(t *testing.T, store *Store, ownerID string, serviceIDs []string, now time.Time) string {
	t.Helper()
	ctx := context.Background()
	if len(serviceIDs) < 25 {
		t.Fatal("pagination service fixture requires at least 25 services")
	}
	connectionID := uuid.NewString()
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO api_probe_connections (
			id, owner_user_id, name, base_url, normalized_base_url,
			credential_ciphertext, credential_nonce, credential_key_version,
			credential_cipher_format, credential_fingerprint,
			probe_model, probe_protocol,
			enabled, verification_status, verified_at,
			measurement_version, version, created_at, updated_at
		) VALUES (
			$1, $2, '分页筛选探针', 'https://pagination.example.com/v1', 'https://pagination.example.com/v1',
			decode('0102', 'hex'), decode('000000000000000000000000', 'hex'), 'test-v1',
			'test-v1', decode('0304', 'hex'),
			'gpt-5-mini', 'openai_responses_v1',
			true, 'verified', $3, 1, 1, $3, $3
		)
	`, connectionID, ownerID, now); err != nil {
		t.Fatalf("seed pagination probe connection: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `UPDATE api_services SET probe_connection_id = $1 WHERE owner_user_id = $2`, connectionID, ownerID); err != nil {
		t.Fatalf("bind pagination probe connection: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO api_service_payment_options (
			id, api_service_id, payment_method, enabled, payment_instructions,
			created_at, updated_at, version
		)
		SELECT gen_random_uuid(), id, 'wechat', true, '站外确认', $2, $2, 1
		FROM api_services
		WHERE owner_user_id = $1
	`, ownerID, now); err != nil {
		t.Fatalf("seed pagination payment options: %v", err)
	}

	var modelCatalogID, modelKey, provider string
	if err := store.pool.QueryRow(ctx, `
		SELECT model.id::text, model.model_key, provider.display_name
		FROM api_model_catalog model
		JOIN api_model_providers provider ON provider.id = model.provider_id
		WHERE model.status = 'active'
		  AND provider.status = 'active'
		ORDER BY model.sort_order, model.id
		LIMIT 1
	`).Scan(&modelCatalogID, &modelKey, &provider); err != nil {
		t.Fatalf("read pagination model catalog: %v", err)
	}
	fixedServiceIDs := serviceIDs[20:25]
	if _, err := store.pool.Exec(ctx, `
		UPDATE api_services
		SET billing_mode = 'fixed_package',
		    usage_visibility = 'fixed_package_only',
		    available_usd_allowance = NULL
		WHERE id = ANY($1::uuid[])
	`, fixedServiceIDs); err != nil {
		t.Fatalf("mark pagination fixed-package services: %v", err)
	}
	packagePrices := []string{"10", "10", "20", "30", "5"}
	for index, serviceID := range fixedServiceIDs {
		serviceModelID := uuid.NewString()
		packageID := uuid.NewString()
		if _, err := store.pool.Exec(ctx, `
			INSERT INTO api_service_models (
				id, api_service_id, distribution_system, model_catalog_id,
				model_key_snapshot, provider_snapshot, merchant_multiplier,
				enabled, created_at, updated_at
			) VALUES ($1, $2, 'sub2api', $3, $4, $5, 1, true, $6, $6)
		`, serviceModelID, serviceID, modelCatalogID, modelKey, provider, now); err != nil {
			t.Fatalf("seed pagination service model %d: %v", index, err)
		}
		if _, err := store.pool.Exec(ctx, `
			INSERT INTO api_service_packages (
				id, api_service_id, name, price_cny, duration_days, description,
				panel_allowance, stock_total, stock_available, enabled, created_at, updated_at
			) VALUES ($1, $2, $3, $4, 7, '分页筛选回归', 10, 2, 2, true, $5, $5)
		`, packageID, serviceID, fmt.Sprintf("分页固定套餐 %02d", index), packagePrices[index], now); err != nil {
			t.Fatalf("seed pagination package %d: %v", index, err)
		}
		if _, err := store.pool.Exec(ctx, `
			INSERT INTO api_service_package_models (
				api_service_package_id, api_service_model_id, api_service_id, created_at
			) VALUES ($1, $2, $3, $4)
		`, packageID, serviceModelID, serviceID, now); err != nil {
			t.Fatalf("seed pagination package model %d: %v", index, err)
		}
	}
	return modelCatalogID
}

func assertPublicServiceFiltersBeforePagination(t *testing.T, store *Store, fixedServiceID, modelCatalogID string) {
	t.Helper()
	page, appErr := store.ListPublicAPIServices(context.Background(), apimarket.PublicServiceFilter{
		BillingMode:           apimarket.ServiceBillingModeFixedPackage,
		PackageModelCatalogID: modelCatalogID,
		PackageDurationDays:   7,
		PackagePriceCNYMax:    "5",
	}, domain.PageRequest{Limit: 1})
	if appErr != nil || len(page.Items) != 1 || page.Items[0].ID != fixedServiceID || page.NextCursor != nil {
		t.Fatalf("fixed-package filters must run before pagination: page=%+v error=%v", page, appErr)
	}
}

func assertPublicServiceSortPagination(t *testing.T, store *Store, meteredServiceIDs, fixedServiceIDs []string, modelCatalogID string) {
	t.Helper()
	tests := []struct {
		name        string
		filter      apimarket.PublicServiceFilter
		expectedIDs []string
		value       func(apimarket.Service) string
	}{
		{
			name: "metered unit price",
			filter: apimarket.PublicServiceFilter{
				BillingMode: apimarket.ServiceBillingModeMetered,
				Sort:        apimarket.PublicServiceSortPriceAsc,
			},
			expectedIDs: meteredServiceIDs,
			value:       func(item apimarket.Service) string { return item.DeclaredCNYPerUSDAllowance },
		},
		{
			name: "metered minimum purchase",
			filter: apimarket.PublicServiceFilter{
				BillingMode: apimarket.ServiceBillingModeMetered,
				Sort:        apimarket.PublicServiceSortMinimumPurchaseAsc,
			},
			expectedIDs: meteredServiceIDs,
			value:       func(item apimarket.Service) string { return item.MinimumIntentCNY },
		},
		{
			name: "fixed package price",
			filter: apimarket.PublicServiceFilter{
				BillingMode:           apimarket.ServiceBillingModeFixedPackage,
				PackageModelCatalogID: modelCatalogID,
				PackageDurationDays:   7,
				Sort:                  apimarket.PublicServiceSortPackagePriceAsc,
			},
			expectedIDs: fixedServiceIDs,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := test.value
			if value == nil {
				value = func(item apimarket.Service) string { return minimumPackagePriceForFilter(item, test.filter) }
			}
			items := collectPublicAPIServiceIntegrationPages(t, store, test.filter, 2)
			assertPublicAPIServicePageCoverage(t, items, test.expectedIDs)
			for index := 1; index < len(items); index++ {
				left := integrationDecimal(t, value(items[index-1]))
				right := integrationDecimal(t, value(items[index]))
				if comparison := left.Cmp(right); comparison > 0 || comparison == 0 && items[index-1].ID >= items[index].ID {
					t.Fatalf("scalar keyset order broke at %d: %+v then %+v", index, items[index-1], items[index])
				}
			}
		})
	}
}

func assertPublicQuotaFiltersBeforePagination(t *testing.T, store *Store, ownerID, serviceID string, now time.Time) {
	t.Helper()
	ctx := context.Background()
	batchID := uuid.NewString()
	roundID := uuid.NewString()
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO api_quota_batches (
			id, api_service_id, owner_user_id, source_type, status,
			declared_total_usd_allowance, unallocated_usd_allowance,
			sale_cutoff_at, expires_at, source_confirmed_at, published_at,
			created_at, updated_at, version
		) VALUES ($1, $2, $3, 'sub2api', 'published', 1000, 0, $4, $5, $6, $6, $6, $6, 1)
	`, batchID, serviceID, ownerID, now.Add(5*time.Hour), now.Add(6*time.Hour), now); err != nil {
		t.Fatalf("seed pagination quota batch: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO api_quota_sale_rounds (
			id, batch_id, api_service_id, owner_user_id, system_slot_key,
			name, starts_at, ends_at, status, created_at, updated_at, version
		) VALUES ($1, $2, $3, $4, '2026-08-09@20:00', '分页系统场次', $5, $6, 'scheduled', $7, $7, 1)
	`, roundID, batchID, serviceID, ownerID, now.Add(-time.Minute), now.Add(30*time.Minute), now); err != nil {
		t.Fatalf("seed pagination quota round: %v", err)
	}
	offerIDs := make([]string, 0, 22)
	for index := 0; index < 21; index++ {
		offerID := uuid.NewString()
		offerIDs = append(offerIDs, offerID)
		updatedAt := now.Add(-time.Duration(index) * time.Second)
		if _, err := store.pool.Exec(ctx, `
			INSERT INTO api_quota_offers (
				id, batch_id, api_service_id, owner_user_id, distribution_system,
				name, usd_allowance, price_cny, model_multiplier,
				delivery_mode, delivery_eta_minutes, sale_mode, status,
				sort_order, published_at, created_at, updated_at, version
			) VALUES ($1, $2, $3, $4, 'sub2api', $5, 10, 5, 1, 'manual', 10, 'scheduled', 'published', $6, $7, $7, $7, 1)
		`, offerID, batchID, serviceID, ownerID, fmt.Sprintf("系统场次额度包 %02d", index), index, updatedAt); err != nil {
			t.Fatalf("seed pagination system offer %d: %v", index, err)
		}
		if _, err := store.pool.Exec(ctx, `
			INSERT INTO api_quota_allocations (
				id, batch_id, offer_id, api_service_id, owner_user_id,
				sale_round_id, sale_mode, copy_limit, allocated_usd_allowance,
				status, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, 'scheduled', 1, 10, 'active', $7, $7)
		`, uuid.NewString(), batchID, offerID, serviceID, ownerID, roundID, updatedAt); err != nil {
			t.Fatalf("seed pagination system allocation %d: %v", index, err)
		}
	}

	continuousOfferID := uuid.NewString()
	offerIDs = append(offerIDs, continuousOfferID)
	continuousUpdatedAt := now.Add(-time.Minute)
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO api_quota_offers (
			id, batch_id, api_service_id, owner_user_id, distribution_system,
			name, usd_allowance, price_cny, model_multiplier,
			delivery_mode, delivery_eta_minutes, sale_mode, status,
			sort_order, published_at, created_at, updated_at, version
		) VALUES ($1, $2, $3, $4, 'sub2api', '深页搜索命中额度包', 10, 5, 1, 'manual', 10, 'continuous', 'published', 100, $5, $5, $5, 1)
	`, continuousOfferID, batchID, serviceID, ownerID, continuousUpdatedAt); err != nil {
		t.Fatalf("seed pagination continuous offer: %v", err)
	}
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO api_quota_allocations (
			id, batch_id, offer_id, api_service_id, owner_user_id,
			sale_mode, copy_limit, allocated_usd_allowance,
			status, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, 'continuous', 1, 10, 'active', $6, $6)
	`, uuid.NewString(), batchID, continuousOfferID, serviceID, ownerID, continuousUpdatedAt); err != nil {
		t.Fatalf("seed pagination continuous allocation: %v", err)
	}

	unfiltered, appErr := store.ListPublicAPIQuotaOffers(ctx, apiquota.PublicOfferFilter{}, domain.PageRequest{Limit: 1}, now)
	if appErr != nil || len(unfiltered.Items) != 1 || unfiltered.Items[0].ID == continuousOfferID {
		t.Fatalf("quota pagination precondition failed: page=%+v error=%v", unfiltered, appErr)
	}
	searchPage, appErr := store.ListPublicAPIQuotaOffers(ctx, apiquota.PublicOfferFilter{Search: "深页搜索命中"}, domain.PageRequest{Limit: 1}, now)
	if appErr != nil || len(searchPage.Items) != 1 || searchPage.Items[0].ID != continuousOfferID || searchPage.NextCursor != nil {
		t.Fatalf("quota search must run before pagination: page=%+v error=%v", searchPage, appErr)
	}
	excludedPage, appErr := store.ListPublicAPIQuotaOffers(ctx, apiquota.PublicOfferFilter{ExcludeSystemSlots: true}, domain.PageRequest{Limit: 1}, now)
	if appErr != nil || len(excludedPage.Items) != 1 || excludedPage.Items[0].ID != continuousOfferID || excludedPage.NextCursor != nil {
		t.Fatalf("system-slot exclusion must run before pagination: page=%+v error=%v", excludedPage, appErr)
	}
	for _, sortMode := range []string{
		apiquota.PublicOfferSortRecommended,
		apiquota.PublicOfferSortReputationDesc,
		apiquota.PublicOfferSortCompletedDesc,
		apiquota.PublicOfferSortResponseFast,
		apiquota.PublicOfferSortUnitPriceAsc,
		apiquota.PublicOfferSortAllowanceDesc,
		apiquota.PublicOfferSortDeliveryAsc,
	} {
		t.Run(sortMode, func(t *testing.T) {
			items := collectPublicAPIQuotaIntegrationPages(t, store, apiquota.PublicOfferFilter{Search: "额度包", Sort: sortMode}, 5, now)
			assertPublicAPIQuotaPageCoverage(t, items, offerIDs)
			for index := 1; index < len(items); index++ {
				previous := items[index-1]
				current := items[index]
				switch sortMode {
				case apiquota.PublicOfferSortUnitPriceAsc:
					comparison := integrationDecimal(t, previous.CNYPerUSD).Cmp(integrationDecimal(t, current.CNYPerUSD))
					if comparison > 0 || comparison == 0 && previous.ID >= current.ID {
						t.Fatalf("unit-price keyset order broke at %d: %+v then %+v", index, previous, current)
					}
				case apiquota.PublicOfferSortAllowanceDesc:
					comparison := integrationDecimal(t, previous.USDAllowance).Cmp(integrationDecimal(t, current.USDAllowance))
					if comparison < 0 || comparison == 0 && previous.ID <= current.ID {
						t.Fatalf("allowance keyset order broke at %d: %+v then %+v", index, previous, current)
					}
				case apiquota.PublicOfferSortDeliveryAsc:
					if previous.DeliveryETAMinutes > current.DeliveryETAMinutes || previous.DeliveryETAMinutes == current.DeliveryETAMinutes && previous.ID >= current.ID {
						t.Fatalf("delivery keyset order broke at %d: %+v then %+v", index, previous, current)
					}
				case apiquota.PublicOfferSortRecommended, apiquota.PublicOfferSortReputationDesc,
					apiquota.PublicOfferSortCompletedDesc, apiquota.PublicOfferSortResponseFast:
					comparison := integrationDecimal(t, previous.PublicSortValue).Cmp(integrationDecimal(t, current.PublicSortValue))
					if comparison > 0 || comparison == 0 && previous.ID >= current.ID {
						t.Fatalf("derived keyset order broke at %d: %+v then %+v", index, previous, current)
					}
				}
			}
		})
	}
}

func collectCarpoolIntegrationPages(t *testing.T, store *Store, filter carpool.ListingFilter, limit int) []carpool.Listing {
	t.Helper()
	var items []carpool.Listing
	var cursor string
	for {
		page, appErr := store.ListAdminCarpoolListings(context.Background(), filter, domain.PageRequest{Limit: limit, Cursor: cursor})
		if appErr != nil {
			t.Fatalf("collect carpool page: %v", appErr)
		}
		items = append(items, page.Items...)
		if page.NextCursor == nil {
			return items
		}
		if *page.NextCursor == cursor {
			t.Fatalf("carpool cursor repeated: %q", cursor)
		}
		cursor = *page.NextCursor
	}
}

func collectAPIServiceIntegrationPages(t *testing.T, store *Store, filter apimarket.AdminServiceFilter, limit int) []apimarket.Service {
	t.Helper()
	var items []apimarket.Service
	var cursor string
	for {
		page, appErr := store.ListAdminAPIServices(context.Background(), filter, domain.PageRequest{Limit: limit, Cursor: cursor})
		if appErr != nil {
			t.Fatalf("collect API service page: %v", appErr)
		}
		items = append(items, page.Items...)
		if page.NextCursor == nil {
			return items
		}
		if *page.NextCursor == cursor {
			t.Fatalf("API service cursor repeated: %q", cursor)
		}
		cursor = *page.NextCursor
	}
}

func collectPublicAPIServiceIntegrationPages(t *testing.T, store *Store, filter apimarket.PublicServiceFilter, limit int) []apimarket.Service {
	t.Helper()
	var items []apimarket.Service
	var cursor string
	for {
		page, appErr := store.ListPublicAPIServices(context.Background(), filter, domain.PageRequest{Limit: limit, Cursor: cursor})
		if appErr != nil {
			t.Fatalf("collect public API service page: %v", appErr)
		}
		items = append(items, page.Items...)
		if page.NextCursor == nil {
			return items
		}
		if *page.NextCursor == cursor {
			t.Fatalf("public API service cursor repeated: %q", cursor)
		}
		cursor = *page.NextCursor
	}
}

func collectPublicAPIQuotaIntegrationPages(t *testing.T, store *Store, filter apiquota.PublicOfferFilter, limit int, now time.Time) []apiquota.OfferCard {
	t.Helper()
	var items []apiquota.OfferCard
	var cursor string
	for {
		page, appErr := store.ListPublicAPIQuotaOffers(context.Background(), filter, domain.PageRequest{Limit: limit, Cursor: cursor}, now)
		if appErr != nil {
			t.Fatalf("collect public API quota page: %v", appErr)
		}
		items = append(items, page.Items...)
		if page.NextCursor == nil {
			return items
		}
		if *page.NextCursor == cursor {
			t.Fatalf("public API quota cursor repeated: %q", cursor)
		}
		cursor = *page.NextCursor
	}
}

func assertCarpoolPageCoverage(t *testing.T, items []carpool.Listing, expectedIDs []string) {
	t.Helper()
	if len(items) != len(expectedIDs) {
		t.Fatalf("carpool page coverage = %d, want %d", len(items), len(expectedIDs))
	}
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if _, exists := seen[item.ID]; exists {
			t.Fatalf("duplicate carpool listing across page boundary: %s", item.ID)
		}
		seen[item.ID] = struct{}{}
	}
	for _, id := range expectedIDs {
		if _, exists := seen[id]; !exists {
			t.Fatalf("carpool listing omitted across page boundaries: %s", id)
		}
	}
}

func assertPublicAPIServicePageCoverage(t *testing.T, items []apimarket.Service, expectedIDs []string) {
	t.Helper()
	if len(items) != len(expectedIDs) {
		t.Fatalf("public API service page coverage = %d, want %d", len(items), len(expectedIDs))
	}
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if _, exists := seen[item.ID]; exists {
			t.Fatalf("duplicate public API service across page boundary: %s", item.ID)
		}
		seen[item.ID] = struct{}{}
	}
	for _, id := range expectedIDs {
		if _, exists := seen[id]; !exists {
			t.Fatalf("public API service omitted across page boundaries: %s", id)
		}
	}
}

func assertPublicAPIQuotaPageCoverage(t *testing.T, items []apiquota.OfferCard, expectedIDs []string) {
	t.Helper()
	if len(items) != len(expectedIDs) {
		t.Fatalf("public API quota page coverage = %d, want %d", len(items), len(expectedIDs))
	}
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if _, exists := seen[item.ID]; exists {
			t.Fatalf("duplicate public API quota offer across page boundary: %s", item.ID)
		}
		seen[item.ID] = struct{}{}
	}
	for _, id := range expectedIDs {
		if _, exists := seen[id]; !exists {
			t.Fatalf("public API quota offer omitted across page boundaries: %s", id)
		}
	}
}

func integrationDecimal(t *testing.T, value string) *big.Rat {
	t.Helper()
	parsed, ok := new(big.Rat).SetString(value)
	if !ok {
		t.Fatalf("parse decimal %q", value)
	}
	return parsed
}
