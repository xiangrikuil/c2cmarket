package postgres

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/review"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestTransactionReviewPostgresLifecycle(t *testing.T) {
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
	requireReviewTestDatabase(t, ctx, pool)

	baseTime := time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC)
	currentTime := baseTime
	store := &Store{pool: pool}
	idempotencyService := idempotency.NewService(store, func() time.Time { return currentTime })
	service := review.NewService(store, idempotencyService, nil, nil, func() time.Time { return currentTime })

	sellerID := uuid.NewString()
	sellerContactID := uuid.NewString()
	buyerID := uuid.NewString()
	buyerContactID := uuid.NewString()
	serviceID := uuid.NewString()
	seedQuotaServiceForTest(t, ctx, pool, sellerID, sellerContactID, buyerID, buyerContactID, serviceID, baseTime)
	adminID := seedReviewUser(t, ctx, pool, "review-admin", baseTime)
	sellerUsername := reviewUsername(t, ctx, pool, sellerID)

	orderID := seedCompletedAPIOrderForReview(
		t,
		ctx,
		pool,
		serviceID,
		sellerID,
		sellerContactID,
		buyerID,
		buyerContactID,
		baseTime.Add(-48*time.Hour),
	)
	pending := findReviewCenterRow(t, listReviewCenter(t, ctx, service, buyerID), review.DirectionPending, orderID)
	if !pending.CanCreate || pending.ReviewerRole != review.RoleBuyer || pending.RevieweeRole != review.RoleSeller {
		t.Fatalf("unexpected buyer reviewable row: %#v", pending)
	}

	submitReviewForTest(t, ctx, service, buyerID, review.TransactionAPIOrder, orderID, review.OperationCreate, 5, []string{"沟通顺畅", "描述真实"}, "卖家说明清晰，交付过程顺畅。", "api-buyer-create")
	received := findReviewCenterRow(t, listReviewCenter(t, ctx, service, sellerID), review.DirectionReceived, orderID)
	if received.Visibility != review.VisibilitySealed || received.ContentVisible || received.Rating != 0 || len(received.Tags) != 0 || received.Note != "" {
		t.Fatalf("sealed counterparty content leaked: %#v", received)
	}
	if public := listPublicReviews(t, ctx, service, sellerUsername); hasPublicReviewForTransaction(public, review.TransactionAPIOrder, "卖家说明清晰，交付过程顺畅。") {
		t.Fatalf("sealed review leaked to public profile: %#v", public)
	}

	submitReviewForTest(t, ctx, service, buyerID, review.TransactionAPIOrder, orderID, review.OperationEdit, 4, []string{"沟通顺畅"}, "修改后的评价内容。", "api-buyer-edit")
	var buyerReviewID string
	var buyerReviewVersion int64
	if err := pool.QueryRow(ctx, `
		SELECT id::text, version
		FROM transaction_reviews
		WHERE api_order_id = $1 AND reviewer_user_id = $2
	`, orderID, buyerID).Scan(&buyerReviewID, &buyerReviewVersion); err != nil {
		t.Fatalf("read edited buyer review: %v", err)
	}
	if buyerReviewVersion != 2 {
		t.Fatalf("expected edited buyer review version 2, got %d", buyerReviewVersion)
	}

	submitReviewForTest(t, ctx, service, sellerID, review.TransactionAPIOrder, orderID, review.OperationCreate, 5, []string{"付款及时"}, "买家付款和确认都很及时。", "api-seller-create")
	var publishedCount int
	var minVisibleAt, maxVisibleAt time.Time
	if err := pool.QueryRow(ctx, `
		SELECT count(*), min(visible_at), max(visible_at)
		FROM transaction_reviews
		WHERE api_order_id = $1
		  AND status = 'published'
		  AND frozen_at = visible_at
	`, orderID).Scan(&publishedCount, &minVisibleAt, &maxVisibleAt); err != nil {
		t.Fatalf("read published review pair: %v", err)
	}
	if publishedCount != 2 || !minVisibleAt.Equal(maxVisibleAt) || !minVisibleAt.Equal(baseTime) {
		t.Fatalf("reviews were not published and frozen together: count=%d min=%s max=%s", publishedCount, minVisibleAt, maxVisibleAt)
	}

	_, appErr := service.SubmitWithIdempotency(
		ctx,
		buyerID,
		"PUT /integration/reviews",
		"api-buyer-edit-frozen",
		"hash-api-buyer-edit-frozen",
		review.SubmitReviewInput{
			TransactionType: review.TransactionAPIOrder,
			TransactionID:   orderID,
			Operation:       review.OperationEdit,
			Rating:          1,
			Tags:            []string{"响应较慢"},
			Note:            "公开后不应允许修改。",
		},
		reviewIntegrationCompletion,
	)
	if appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("published review edit must be rejected, got %#v", appErr)
	}

	public := listPublicReviews(t, ctx, service, sellerUsername)
	if !hasPublicReviewForTransaction(public, review.TransactionAPIOrder, "修改后的评价内容。") {
		t.Fatalf("published verified review missing from public profile: %#v", public)
	}
	var revisionCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM transaction_review_revisions
		WHERE transaction_review_id = $1
	`, buyerReviewID).Scan(&revisionCount); err != nil {
		t.Fatalf("count buyer review revisions: %v", err)
	}
	if revisionCount != 3 {
		t.Fatalf("expected create, edit, publish revisions, got %d", revisionCount)
	}

	var publishedVersion int64
	if err := pool.QueryRow(ctx, `SELECT version FROM transaction_reviews WHERE id = $1`, buyerReviewID).Scan(&publishedVersion); err != nil {
		t.Fatalf("read published buyer review version: %v", err)
	}
	if _, appErr := service.RemoveWithIdempotency(
		ctx,
		adminID,
		true,
		"POST /integration/admin/reviews/remove",
		"api-review-remove",
		"hash-api-review-remove",
		review.RemoveReviewInput{
			ReviewID:        buyerReviewID,
			ExpectedVersion: publishedVersion,
			Reason:          "集成测试管理员移除评价。",
		},
		reviewIntegrationCompletion,
	); appErr != nil {
		t.Fatalf("remove published review: %v", appErr)
	}
	public = listPublicReviews(t, ctx, service, sellerUsername)
	if hasPublicReviewForTransaction(public, review.TransactionAPIOrder, "修改后的评价内容。") {
		t.Fatalf("removed review remained public: %#v", public)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE transaction_review_revisions
		SET reason = '不得改写'
		WHERE transaction_review_id = $1
		  AND revision_number = 1
	`, buyerReviewID); err == nil {
		t.Fatal("append-only review revision update unexpectedly succeeded")
	}
}

func TestTransactionReviewPostgresDeadlineExclusionAndCarpoolRoles(t *testing.T) {
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
	requireReviewTestDatabase(t, ctx, pool)

	baseTime := time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC)
	currentTime := baseTime
	store := &Store{pool: pool}
	idempotencyService := idempotency.NewService(store, func() time.Time { return currentTime })
	service := review.NewService(store, idempotencyService, nil, nil, func() time.Time { return currentTime })

	sellerID := uuid.NewString()
	sellerContactID := uuid.NewString()
	buyerID := uuid.NewString()
	buyerContactID := uuid.NewString()
	serviceID := uuid.NewString()
	seedQuotaServiceForTest(t, ctx, pool, sellerID, sellerContactID, buyerID, buyerContactID, serviceID, baseTime)
	adminID := seedReviewUser(t, ctx, pool, "review-exclusion-admin", baseTime)

	expiringOrderID := seedCompletedAPIOrderForReview(
		t,
		ctx,
		pool,
		serviceID,
		sellerID,
		sellerContactID,
		buyerID,
		buyerContactID,
		baseTime.Add(-13*24*time.Hour),
	)
	submitReviewForTest(t, ctx, service, buyerID, review.TransactionAPIOrder, expiringOrderID, review.OperationCreate, 5, []string{"合作愉快"}, "等待评价窗口截止公开。", "api-expiring-create")
	currentTime = baseTime.Add(48 * time.Hour)
	received := findReviewCenterRow(t, listReviewCenter(t, ctx, service, sellerID), review.DirectionReceived, expiringOrderID)
	expectedVisibleAt := baseTime.Add(24 * time.Hour)
	if received.Status != review.StatusPublished || received.VisibleAt == nil || !received.VisibleAt.Equal(expectedVisibleAt) || received.FrozenAt == nil {
		t.Fatalf("deadline did not publish and freeze review: %#v", received)
	}
	if _, appErr := service.SubmitWithIdempotency(
		ctx,
		sellerID,
		"POST /integration/reviews",
		"api-expired-seller-create",
		"hash-api-expired-seller-create",
		review.SubmitReviewInput{
			TransactionType: review.TransactionAPIOrder,
			TransactionID:   expiringOrderID,
			Operation:       review.OperationCreate,
			Rating:          5,
			Tags:            []string{"付款及时"},
			Note:            "截止后不应允许补交。",
		},
		reviewIntegrationCompletion,
	); appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("late counterparty review must be rejected, got %#v", appErr)
	}

	currentTime = baseTime
	excludedOrderID := seedCompletedAPIOrderForReview(
		t,
		ctx,
		pool,
		serviceID,
		sellerID,
		sellerContactID,
		buyerID,
		buyerContactID,
		baseTime.Add(-24*time.Hour),
	)
	if _, err := pool.Exec(ctx, `
		INSERT INTO reputation_transaction_exclusions (
		  transaction_type, transaction_id, excluded_at, excluded_by_admin_id,
		  reason_code, reason, created_at, updated_at
		)
		VALUES ('api_order', $1, $2, $3, 'review_integration', '评价集成测试排除', $2, $2)
	`, excludedOrderID, baseTime, adminID); err != nil {
		t.Fatalf("exclude API order from reputation: %v", err)
	}
	if _, appErr := service.SubmitWithIdempotency(
		ctx,
		buyerID,
		"POST /integration/reviews",
		"api-excluded-buyer-create",
		"hash-api-excluded-buyer-create",
		review.SubmitReviewInput{
			TransactionType: review.TransactionAPIOrder,
			TransactionID:   excludedOrderID,
			Operation:       review.OperationCreate,
			Rating:          5,
			Tags:            []string{"沟通顺畅"},
			Note:            "被排除交易不应产生评价。",
		},
		reviewIntegrationCompletion,
	); appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("excluded transaction review must be rejected, got %#v", appErr)
	}

	incompleteOrderID := seedCompletedAPIOrderForReview(
		t,
		ctx,
		pool,
		serviceID,
		sellerID,
		sellerContactID,
		buyerID,
		buyerContactID,
		baseTime.Add(-12*time.Hour),
	)
	if _, err := pool.Exec(ctx, `
		UPDATE api_orders
		SET status = 'paid_confirmed',
		    delivery_note = NULL,
		    delivery_submitted_at = NULL,
		    completed_at = NULL,
		    updated_at = $2
		WHERE id = $1
	`, incompleteOrderID, baseTime); err != nil {
		t.Fatalf("prepare incomplete API order: %v", err)
	}
	if _, appErr := service.SubmitWithIdempotency(
		ctx,
		buyerID,
		"POST /integration/reviews",
		"api-incomplete-buyer-create",
		"hash-api-incomplete-buyer-create",
		review.SubmitReviewInput{
			TransactionType: review.TransactionAPIOrder,
			TransactionID:   incompleteOrderID,
			Operation:       review.OperationCreate,
			Rating:          5,
			Tags:            []string{"交付清晰"},
			Note:            "未完成订单不应产生评价。",
		},
		reviewIntegrationCompletion,
	); appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("incomplete transaction review must be rejected, got %#v", appErr)
	}

	outsiderID := seedReviewUser(t, ctx, pool, "review-outsider", baseTime)
	if _, appErr := service.SubmitWithIdempotency(
		ctx,
		outsiderID,
		"POST /integration/reviews",
		"api-outsider-create",
		"hash-api-outsider-create",
		review.SubmitReviewInput{
			TransactionType: review.TransactionAPIOrder,
			TransactionID:   expiringOrderID,
			Operation:       review.OperationCreate,
			Rating:          5,
			Tags:            []string{"合作愉快"},
			Note:            "非交易参与方不应产生评价。",
		},
		reviewIntegrationCompletion,
	); appErr == nil || appErr.Code != domain.CodeObjectNotFound {
		t.Fatalf("non-participant review must be rejected, got %#v", appErr)
	}

	membershipID := seedCompletedCarpoolMembershipForReview(
		t,
		ctx,
		pool,
		sellerID,
		sellerContactID,
		buyerID,
		buyerContactID,
		baseTime.Add(-24*time.Hour),
	)
	submitReviewForTest(t, ctx, service, sellerID, review.TransactionCarpoolMembership, membershipID, review.OperationCreate, 5, []string{"确认及时"}, "卖家评价买家。", "carpool-seller-create")
	sellerSent := findReviewCenterRow(t, listReviewCenter(t, ctx, service, sellerID), review.DirectionSent, membershipID)
	if sellerSent.ReviewerRole != review.RoleSeller || sellerSent.RevieweeRole != review.RoleBuyer {
		t.Fatalf("unexpected carpool seller direction: %#v", sellerSent)
	}
	buyerReceived := findReviewCenterRow(t, listReviewCenter(t, ctx, service, buyerID), review.DirectionReceived, membershipID)
	if buyerReceived.ContentVisible || buyerReceived.Rating != 0 || buyerReceived.Note != "" {
		t.Fatalf("sealed carpool seller review leaked to buyer: %#v", buyerReceived)
	}
	submitReviewForTest(t, ctx, service, buyerID, review.TransactionCarpoolMembership, membershipID, review.OperationCreate, 4, []string{"规则清晰"}, "买家评价卖家。", "carpool-buyer-create")

	var buyerRoleCount, sellerRoleCount int
	if err := pool.QueryRow(ctx, `
		SELECT
		  count(*) FILTER (WHERE reviewer_role = 'buyer' AND reviewee_role = 'seller'),
		  count(*) FILTER (WHERE reviewer_role = 'seller' AND reviewee_role = 'buyer')
		FROM transaction_reviews
		WHERE carpool_membership_id = $1
		  AND status = 'published'
	`, membershipID).Scan(&buyerRoleCount, &sellerRoleCount); err != nil {
		t.Fatalf("read carpool review roles: %v", err)
	}
	if buyerRoleCount != 1 || sellerRoleCount != 1 {
		t.Fatalf("expected one review in each carpool direction, buyer=%d seller=%d", buyerRoleCount, sellerRoleCount)
	}
}

func requireReviewTestDatabase(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	var databaseName string
	if err := pool.QueryRow(ctx, "select current_database()").Scan(&databaseName); err != nil {
		t.Fatalf("read test database name: %v", err)
	}
	if !strings.Contains(databaseName, "_reputation_test_") {
		t.Fatalf("refusing to run review integration test against non-dedicated database %q", databaseName)
	}
}

func seedReviewUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, prefix string, now time.Time) string {
	t.Helper()
	userID := uuid.NewString()
	username := prefix + "-" + strings.ReplaceAll(userID[:8], "-", "")
	if _, err := pool.Exec(ctx, `
		INSERT INTO users (id, username, display_name, account_status, created_at, updated_at)
		VALUES ($1, $2, $2, 'active', $3, $3)
	`, userID, username, now); err != nil {
		t.Fatalf("seed review user: %v", err)
	}
	return userID
}

func seedCompletedAPIOrderForReview(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	serviceID string,
	sellerID string,
	sellerContactID string,
	buyerID string,
	buyerContactID string,
	completedAt time.Time,
) string {
	t.Helper()
	var sellerContactVersionID, buyerContactVersionID string
	if err := pool.QueryRow(ctx, `
		SELECT seller.current_version_id::text, buyer.current_version_id::text
		FROM contact_methods seller
		JOIN contact_methods buyer ON buyer.id = $2
		WHERE seller.id = $1
	`, sellerContactID, buyerContactID).Scan(&sellerContactVersionID, &buyerContactVersionID); err != nil {
		t.Fatalf("read API order contact versions: %v", err)
	}
	intentID := uuid.NewString()
	if _, err := pool.Exec(ctx, `
		INSERT INTO api_purchase_intents (
		  id, api_service_id, api_service_owner_user_id, buyer_user_id, owner_user_id,
		  buyer_contact_method_id, buyer_contact_method_version_id,
		  owner_contact_method_id, owner_contact_method_version_id,
		  status, requested_cny_amount, selected_access_mode,
		  service_version_snapshot, service_title_snapshot,
		  distribution_system_snapshot, billing_mode_snapshot,
		  buyer_contact_type_snapshot, buyer_contact_label_snapshot,
		  owner_contact_type_snapshot, owner_contact_label_snapshot,
		  minimum_intent_cny_snapshot, pricing_snapshot,
		  contacted_at, created_at, updated_at
		)
		VALUES (
		  $1, $2, $3, $4, $3,
		  $5, $6, $7, $8,
		  'ordered', 20, 'buyer_dedicated_sub_key',
		  1, '评价集成测试 API 服务',
		  'sub2api', 'manual_usage_check',
		  'linuxdo', 'linux.do', 'linuxdo', 'linux.do',
		  1, '{}'::jsonb,
		  $9, $9, $9
		)
	`, intentID, serviceID, sellerID, buyerID, buyerContactID, buyerContactVersionID, sellerContactID, sellerContactVersionID, completedAt.Add(-4*time.Hour)); err != nil {
		t.Fatalf("seed completed API purchase intent: %v", err)
	}

	orderID := uuid.NewString()
	if _, err := pool.Exec(ctx, `
		INSERT INTO api_orders (
		  id, api_purchase_intent_id, api_service_id, buyer_user_id, seller_user_id,
		  status, service_title_snapshot, service_version_snapshot, billing_mode_snapshot,
		  amount, currency, selected_payment_method, payment_window_minutes_snapshot,
		  payment_expires_at, payment_instructions_snapshot,
		  payment_summary, payment_submitted_at, paid_confirmed_at,
		  delivery_note, delivery_submitted_at, completed_at, created_at, updated_at
		)
		VALUES (
		  $1, $2, $3, $4, $5,
		  'completed', '评价集成测试 API 服务', 1, 'manual_usage_check',
		  20, 'CNY', 'wechat', 10,
		  $6, '站外确认付款',
		  '已付款', $7, $8,
		  '已交付', $9, $10, $11, $10
		)
	`, orderID, intentID, serviceID, buyerID, sellerID,
		completedAt.Add(-3*time.Hour),
		completedAt.Add(-2*time.Hour),
		completedAt.Add(-90*time.Minute),
		completedAt.Add(-30*time.Minute),
		completedAt,
		completedAt.Add(-4*time.Hour),
	); err != nil {
		t.Fatalf("seed completed API order: %v", err)
	}
	return orderID
}

func seedCompletedCarpoolMembershipForReview(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	sellerID string,
	sellerContactID string,
	buyerID string,
	buyerContactID string,
	completedAt time.Time,
) string {
	t.Helper()
	var productPlanID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM product_plans ORDER BY created_at, id LIMIT 1`).Scan(&productPlanID); err != nil {
		t.Fatalf("read seeded product plan: %v", err)
	}
	listingID := uuid.NewString()
	if _, err := pool.Exec(ctx, `
		INSERT INTO carpool_listings (
		  id, owner_user_id, product_plan_id, title, summary, access_arrangement,
		  price_monthly_cny, buyer_seat_capacity, active_buyer_members, status,
		  policy_version, owner_contact_method_id, created_at, updated_at
		)
		VALUES ($1, $2, $3, '评价集成测试拼车', '用于双向评价集成测试', '双方站外确认',
		        20, 2, 0, 'active', 1, $4, $5, $5)
	`, listingID, sellerID, productPlanID, sellerContactID, completedAt.Add(-35*24*time.Hour)); err != nil {
		t.Fatalf("seed carpool listing: %v", err)
	}
	joinedAt := completedAt.Add(-30 * 24 * time.Hour)
	applicationID := uuid.NewString()
	if _, err := pool.Exec(ctx, `
		INSERT INTO carpool_applications (
		  id, carpool_listing_id, buyer_user_id, owner_user_id, product_plan_id,
		  buyer_contact_method_id, status, listing_title_snapshot,
		  price_monthly_cny_snapshot, policy_version_snapshot,
		  joined_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, 'joined', '评价集成测试拼车',
		        20, 1, $7, $7, $7)
	`, applicationID, listingID, buyerID, sellerID, productPlanID, buyerContactID, joinedAt); err != nil {
		t.Fatalf("seed joined carpool application: %v", err)
	}
	membershipID := uuid.NewString()
	if _, err := pool.Exec(ctx, `
		INSERT INTO carpool_memberships (
		  id, carpool_listing_id, carpool_application_id, buyer_user_id, owner_user_id,
		  product_plan_id, status, price_monthly_cny_snapshot, policy_version_snapshot,
		  joined_at, ended_at, ended_reason, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, 'completed', 20, 1,
		        $7, $8, 'completed_by_both', $7, $8)
	`, membershipID, listingID, applicationID, buyerID, sellerID, productPlanID, joinedAt, completedAt); err != nil {
		t.Fatalf("seed completed carpool membership: %v", err)
	}
	return membershipID
}

func submitReviewForTest(
	t *testing.T,
	ctx context.Context,
	service *review.Service,
	userID string,
	transactionType string,
	transactionID string,
	operation string,
	rating int,
	tags []string,
	note string,
	key string,
) {
	t.Helper()
	if _, appErr := service.SubmitWithIdempotency(
		ctx,
		userID,
		"POST /integration/reviews",
		key,
		"hash-"+key,
		review.SubmitReviewInput{
			TransactionType: transactionType,
			TransactionID:   transactionID,
			Operation:       operation,
			Rating:          rating,
			Tags:            tags,
			Note:            note,
		},
		reviewIntegrationCompletion,
	); appErr != nil {
		t.Fatalf("submit %s review %s: %v", transactionType, key, appErr)
	}
}

func listReviewCenter(t *testing.T, ctx context.Context, service *review.Service, userID string) []review.ReviewCenterRow {
	t.Helper()
	rows, appErr := service.ListMine(ctx, userID)
	if appErr != nil {
		t.Fatalf("list review center: %v", appErr)
	}
	return rows
}

func findReviewCenterRow(t *testing.T, rows []review.ReviewCenterRow, direction, transactionID string) review.ReviewCenterRow {
	t.Helper()
	for _, row := range rows {
		if row.Direction == direction && row.TransactionID == transactionID {
			return row
		}
	}
	t.Fatalf("review center row not found: direction=%s transaction=%s rows=%#v", direction, transactionID, rows)
	return review.ReviewCenterRow{}
}

func listPublicReviews(t *testing.T, ctx context.Context, service *review.Service, username string) []review.PublicReview {
	t.Helper()
	items, appErr := service.PublicForUser(ctx, username)
	if appErr != nil {
		t.Fatalf("list public reviews: %v", appErr)
	}
	return items
}

func hasPublicReviewForTransaction(items []review.PublicReview, transactionType, note string) bool {
	for _, item := range items {
		if item.TransactionType == transactionType && item.Note == note {
			return true
		}
	}
	return false
}

func reviewUsername(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID string) string {
	t.Helper()
	var username string
	if err := pool.QueryRow(ctx, `SELECT username FROM users WHERE id = $1`, userID).Scan(&username); err != nil {
		t.Fatalf("read review username: %v", err)
	}
	return username
}

func reviewIntegrationCompletion(result review.MutationResult) (idempotency.Completion, *domain.AppError) {
	return idempotency.Completion{
		Status:       http.StatusOK,
		ContentType:  "application/json",
		Body:         []byte(`{}`),
		ResourceType: "transaction_review",
		ResourceID:   result.Row.ID,
	}, nil
}
