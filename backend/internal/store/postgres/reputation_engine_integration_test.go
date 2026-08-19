package postgres

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/reputation"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestReputationEnginePostgresSnapshotsHistoryAndDirtyTriggers(t *testing.T) {
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

	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	subjectID := seedReviewUser(t, ctx, pool, "engine-subject", now)
	otherID := seedReviewUser(t, ctx, pool, "engine-other", now)
	store := &Store{pool: pool}
	service := reputation.NewService(store, func() time.Time { return now })

	for _, userID := range []string{subjectID, otherID} {
		result, appErr := service.RecalculateUser(ctx, userID)
		if appErr != nil {
			t.Fatalf("initial reputation recalculation for %s: %v", userID, appErr)
		}
		if result.RebuiltStates != 6 {
			t.Fatalf("expected six rebuilt states for %s, got %#v", userID, result)
		}
	}
	assertReputationStateCount(t, ctx, pool, subjectID, 6)
	assertReputationStateCount(t, ctx, pool, otherID, 6)
	assertReputationHistoryCount(t, ctx, pool, subjectID, 6)
	initialHistory, appErr := service.History(ctx, subjectID, 100)
	if appErr != nil {
		t.Fatalf("read initial reputation history: %v", appErr)
	}
	if len(initialHistory) != 6 {
		t.Fatalf("expected six initial history rows, got %#v", initialHistory)
	}
	for _, item := range initialHistory {
		if item.FromTier != nil || item.FromState != nil {
			t.Fatalf("initial history must preserve null previous state: %#v", item)
		}
	}

	if _, appErr := service.RecalculateUser(ctx, subjectID); appErr != nil {
		t.Fatalf("repeat reputation recalculation: %v", appErr)
	}
	assertReputationHistoryCount(t, ctx, pool, subjectID, 6)

	bindingID := uuid.NewString()
	if _, err := pool.Exec(ctx, `
		INSERT INTO linux_do_bindings (
		  id, user_id, linux_do_user_id, linux_do_username,
		  trust_level, bound_at, last_synced_at
		)
		VALUES ($1, $2, $3, $4, 1, $5, $5)
	`, bindingID, subjectID, "engine-"+bindingID, "engine-"+bindingID[:8], now); err != nil {
		t.Fatalf("insert linux.do binding: %v", err)
	}
	assertDirtyReputationStates(t, ctx, pool, subjectID, 6)
	assertDirtyReputationStates(t, ctx, pool, otherID, 0)
	recalculateAndAssertClean(t, ctx, service, pool, subjectID)

	if _, err := pool.Exec(ctx, `
		UPDATE linux_do_bindings
		SET trust_level = 2, last_synced_at = $2
		WHERE id = $1
	`, bindingID, now.Add(time.Minute)); err != nil {
		t.Fatalf("update linux.do binding: %v", err)
	}
	assertDirtyReputationStates(t, ctx, pool, subjectID, 6)
	recalculateAndAssertClean(t, ctx, service, pool, subjectID)

	if _, err := pool.Exec(ctx, `DELETE FROM linux_do_bindings WHERE id = $1`, bindingID); err != nil {
		t.Fatalf("delete linux.do binding: %v", err)
	}
	assertDirtyReputationStates(t, ctx, pool, subjectID, 6)
	recalculateAndAssertClean(t, ctx, service, pool, subjectID)
	assertReputationHistoryCount(t, ctx, pool, subjectID, 6)

	restrictionID := uuid.NewString()
	if _, err := pool.Exec(ctx, `
		INSERT INTO user_restrictions (
		  id, user_id, restriction_type, reason, starts_at, ends_at,
		  created_by_admin_id, created_at, role_scope, action_code,
		  reason_code, public_reason, updated_at
		)
		VALUES (
		  $1, $2, 'reputation_test', '信誉引擎集成测试限制', $3, NULL,
		  $4, $3, 'buyer', 'api_order_create',
		  'engine_test', '信誉引擎集成测试限制', $3
		)
	`, restrictionID, subjectID, now, otherID); err != nil {
		t.Fatalf("insert reputation restriction: %v", err)
	}
	assertDirtyReputationStates(t, ctx, pool, subjectID, 6)
	if _, appErr := service.RecalculateUser(ctx, subjectID); appErr != nil {
		t.Fatalf("recalculate restricted user: %v", appErr)
	}

	var restrictedCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM user_reputation_states
		WHERE user_id = $1
		  AND role = 'buyer'
		  AND scope IN ('overall', 'api')
		  AND state = 'restricted'
	`, subjectID).Scan(&restrictedCount); err != nil {
		t.Fatalf("count restricted reputation states: %v", err)
	}
	if restrictedCount != 2 {
		t.Fatalf("expected buyer overall/api restriction states, got %d", restrictedCount)
	}
	assertReputationHistoryCount(t, ctx, pool, subjectID, 8)

	if _, appErr := service.RecalculateUser(ctx, subjectID); appErr != nil {
		t.Fatalf("force recalculate unchanged restricted user: %v", appErr)
	}
	assertReputationHistoryCount(t, ctx, pool, subjectID, 8)

	if _, err := pool.Exec(ctx, `
		UPDATE user_reputation_history
		SET reason_snapshot = '{}'::jsonb
		WHERE id = (
		  SELECT id
		  FROM user_reputation_history
		  WHERE user_id = $1
		  ORDER BY created_at DESC, id DESC
		  LIMIT 1
		)
	`, subjectID); err == nil {
		t.Fatal("append-only reputation history update unexpectedly succeeded")
	}
}

func TestReputationEnginePostgresAggregatesCompletedOrderReviewAndBatchConsistency(t *testing.T) {
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

	now := time.Date(2026, 7, 24, 15, 0, 0, 0, time.UTC)
	sellerID := uuid.NewString()
	sellerContactID := uuid.NewString()
	buyerID := uuid.NewString()
	buyerContactID := uuid.NewString()
	serviceID := uuid.NewString()
	seedQuotaServiceForTest(
		t,
		ctx,
		pool,
		sellerID,
		sellerContactID,
		buyerID,
		buyerContactID,
		serviceID,
		now,
	)
	orderID := seedCompletedAPIOrderForReview(
		t,
		ctx,
		pool,
		serviceID,
		sellerID,
		sellerContactID,
		buyerID,
		buyerContactID,
		now.Add(-24*time.Hour),
	)
	if _, err := pool.Exec(ctx, `
		INSERT INTO transaction_reviews (
		  transaction_type, api_order_id,
		  reviewer_user_id, reviewee_user_id, reviewer_role, reviewee_role,
		  rating, tags, note, status, review_deadline_at,
		  visible_at, frozen_at, created_at, updated_at
		)
		VALUES (
		  'api_order', $1,
		  $2, $3, 'buyer', 'seller',
		  5, ARRAY['描述真实', '响应较慢'], '信誉引擎集成测试评价。', 'published', $4,
		  $5, $5, $5, $5
		)
	`, orderID, buyerID, sellerID, now.Add(13*24*time.Hour), now.Add(-12*time.Hour)); err != nil {
		t.Fatalf("insert published reputation review: %v", err)
	}

	store := &Store{pool: pool}
	service := reputation.NewService(store, func() time.Time { return now })
	detail, appErr := service.GetUserReputation(ctx, sellerID)
	if appErr != nil {
		t.Fatalf("load seller reputation detail: %v", appErr)
	}
	detailByKey := make(map[reputation.SnapshotKey]reputation.ReputationSnapshot, len(detail))
	for _, snapshot := range detail {
		detailByKey[snapshot.Key()] = snapshot
	}
	sellerAPI := detailByKey[reputation.SnapshotKey{
		UserID: sellerID,
		Role:   reputation.RoleSeller,
		Scope:  reputation.ScopeAPI,
	}]
	if sellerAPI.Metrics.CompletedCount != 1 ||
		sellerAPI.Metrics.CompletedCountLast90Days != 1 ||
		sellerAPI.Metrics.VerifiedReviewCount != 1 ||
		sellerAPI.Metrics.RatingDistribution.Five != 1 {
		t.Fatalf("unexpected seller API reputation metrics: %#v", sellerAPI.Metrics)
	}
	if sellerAPI.Metrics.RawAverageRating == nil || *sellerAPI.Metrics.RawAverageRating != 5 {
		t.Fatalf("unexpected seller raw average rating: %#v", sellerAPI.Metrics.RawAverageRating)
	}
	if len(sellerAPI.Metrics.CommonPositiveTags) != 1 ||
		sellerAPI.Metrics.CommonPositiveTags[0].Count != 1 ||
		len(sellerAPI.Metrics.CommonNegativeTags) != 1 ||
		sellerAPI.Metrics.CommonNegativeTags[0].Tag != "响应较慢" ||
		sellerAPI.Metrics.CommonNegativeTags[0].Count != 1 {
		t.Fatalf("unexpected seller common review tags: %#v", sellerAPI.Metrics)
	}
	if sellerAPI.NextRecalculationAt == nil {
		t.Fatal("recent completion/review must provide a next recalculation boundary")
	}
	if _, appErr := service.RecalculateUser(ctx, buyerID); appErr != nil {
		t.Fatalf("build buyer reputation snapshots: %v", appErr)
	}

	batch, appErr := service.GetMany(
		ctx,
		[]string{sellerID},
		reputation.RoleSeller,
		reputation.ScopeAPI,
	)
	if appErr != nil {
		t.Fatalf("load seller reputation batch summary: %v", appErr)
	}
	batchSeller, ok := batch[sellerID]
	if !ok {
		t.Fatalf("batch reputation missing seller %s", sellerID)
	}
	if batchSeller.Tier != sellerAPI.Tier ||
		batchSeller.State != sellerAPI.State ||
		batchSeller.Confidence != sellerAPI.Confidence ||
		batchSeller.RuleVersion != sellerAPI.RuleVersion ||
		!reflect.DeepEqual(batchSeller.Metrics, sellerAPI.Metrics) {
		t.Fatalf("batch summary differs from detail: batch=%#v detail=%#v", batchSeller, sellerAPI)
	}

	adminID := seedReviewUser(t, ctx, pool, "engine-exclusion-admin", now)
	if _, err := pool.Exec(ctx, `
		INSERT INTO reputation_transaction_exclusions (
		  transaction_type, transaction_id, excluded_at, excluded_by_admin_id,
		  reason_code, reason, created_at, updated_at
		)
		VALUES (
		  'api_order', $1, $2, $3,
		  'engine_test', '信誉引擎集成测试排除', $2, $2
		)
	`, orderID, now, adminID); err != nil {
		t.Fatalf("exclude reputation order: %v", err)
	}
	assertDirtyReputationStates(t, ctx, pool, sellerID, 6)
	assertDirtyReputationStates(t, ctx, pool, buyerID, 6)

	refreshed, appErr := service.GetMany(
		ctx,
		[]string{sellerID},
		reputation.RoleSeller,
		reputation.ScopeAPI,
	)
	if appErr != nil {
		t.Fatalf("refresh excluded seller reputation: %v", appErr)
	}
	excluded := refreshed[sellerID]
	if excluded.Metrics.CompletedCount != 0 ||
		excluded.Metrics.VerifiedReviewCount != 0 ||
		excluded.Metrics.RawAverageRating != nil ||
		excluded.Metrics.WeightedRating != nil {
		t.Fatalf("excluded transaction remained in reputation: %#v", excluded.Metrics)
	}
}

func TestReputationEnginePostgresAttributesAPIOrderTimeouts(t *testing.T) {
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

	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	sellerID := uuid.NewString()
	sellerContactID := uuid.NewString()
	buyerID := uuid.NewString()
	buyerContactID := uuid.NewString()
	serviceID := uuid.NewString()
	seedQuotaServiceForTest(t, ctx, pool, sellerID, sellerContactID, buyerID, buyerContactID, serviceID, now.Add(-2*time.Hour))
	seedTimeoutAPIOrderForReputationTest(t, ctx, pool, serviceID, sellerID, sellerContactID, buyerID, buyerContactID, now)

	if _, err := pool.Exec(ctx, `
		INSERT INTO user_reputation_states (
		  user_id, role, scope, tier, state, confidence, rule_version,
		  metrics_json, warnings_json, badges_json, progress_json,
		  tier_entered_at, state_entered_at, calculated_at
		)
		VALUES (
		  $1, 'buyer', 'api', 'insufficient', 'active', 'low', 'reputation-v1',
		  '{"completedCount":99,"roleResponsibilityCancellationCount":0}'::jsonb,
		  '[]'::jsonb, '[]'::jsonb, '[]'::jsonb,
		  $2, $2, $2
		)
	`, buyerID, now.Add(-time.Hour)); err != nil {
		t.Fatalf("insert old-rule reputation snapshot: %v", err)
	}

	store := &Store{pool: pool}
	service := reputation.NewService(store, func() time.Time { return now })
	for _, userID := range []string{buyerID, sellerID} {
		if _, appErr := service.GetUserReputation(ctx, userID); appErr != nil {
			t.Fatalf("recalculate timeout reputation for %s: %v", userID, appErr)
		}
	}

	facts, appErr := store.AggregateFacts(ctx, []string{buyerID, sellerID}, now)
	if appErr != nil {
		t.Fatalf("aggregate timeout responsibility facts: %v", appErr)
	}
	assertTimeoutFacts := func(label string, value reputation.ScopeFacts, responsible, recent int) {
		t.Helper()
		if value.RoleResponsibilityCancellationCount != responsible ||
			value.RoleResponsibilityCancellationCount90d != recent ||
			value.UnknownResponsibilityCancellationCount != 0 {
			t.Fatalf("unexpected %s timeout facts: %#v", label, value)
		}
	}
	assertTimeoutFacts("buyer API", facts[buyerID].Buyer.API, 1, 1)
	assertTimeoutFacts("seller API", facts[sellerID].Seller.API, 0, 0)

	var ruleVersion string
	var completedCount, responsibleCount, unknownCount int
	var completionRate *float64
	if err := pool.QueryRow(ctx, `
		SELECT
		  rule_version,
		  (metrics_json->>'completedCount')::integer,
		  (metrics_json->>'roleResponsibilityCancellationCount')::integer,
		  (metrics_json->>'unknownResponsibilityCancellationCount')::integer,
		  (metrics_json->>'roleCompletionRate')::double precision
		FROM user_reputation_states
		WHERE user_id = $1 AND role = 'buyer' AND scope = 'api'
	`, buyerID).Scan(&ruleVersion, &completedCount, &responsibleCount, &unknownCount, &completionRate); err != nil {
		t.Fatalf("read recalculated buyer API snapshot: %v", err)
	}
	if ruleVersion != reputation.RuleVersion || completedCount != 0 || responsibleCount != 1 || unknownCount != 0 || completionRate == nil || *completionRate != 0 {
		t.Fatalf("old snapshot did not rebuild with timeout responsibility: version=%s completed=%d responsible=%d unknown=%d rate=%v", ruleVersion, completedCount, responsibleCount, unknownCount, completionRate)
	}
}

func seedTimeoutAPIOrderForReputationTest(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	serviceID string,
	sellerID string,
	sellerContactID string,
	buyerID string,
	buyerContactID string,
	now time.Time,
) {
	t.Helper()
	var sellerContactVersionID, buyerContactVersionID string
	if err := pool.QueryRow(ctx, `
		SELECT seller.current_version_id::text, buyer.current_version_id::text
		FROM contact_methods seller
		JOIN contact_methods buyer ON buyer.id = $2
		WHERE seller.id = $1
	`, sellerContactID, buyerContactID).Scan(&sellerContactVersionID, &buyerContactVersionID); err != nil {
		t.Fatalf("read timeout order contact versions: %v", err)
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
		  1, '付款超时信誉测试 API 服务',
		  'sub2api', 'manual_usage_check',
		  'linuxdo', 'linux.do', 'linuxdo', 'linux.do',
		  1, '{}'::jsonb,
		  $9, $9, $9
		)
	`, intentID, serviceID, sellerID, buyerID, buyerContactID, buyerContactVersionID, sellerContactID, sellerContactVersionID, now.Add(-time.Hour)); err != nil {
		t.Fatalf("seed timeout API purchase intent: %v", err)
	}

	orderID := uuid.NewString()
	cancelledAt := now.Add(-10 * time.Minute)
	orderCreatedAt := now.Add(-time.Hour)
	orderNo, err := apiorder.GenerateOrderNo(orderCreatedAt)
	if err != nil {
		t.Fatalf("generate timeout API order number: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO api_orders (
		  id, api_purchase_intent_id, api_service_id, buyer_user_id, seller_user_id,
		  status, service_title_snapshot, service_version_snapshot, billing_mode_snapshot,
		  amount, currency, selected_payment_method, payment_window_minutes_snapshot,
		  payment_expires_at, payment_instructions_snapshot,
		  cancelled_at, cancel_reason, created_at, updated_at, order_no
		)
		VALUES (
		  $1, $2, $3, $4, $5,
		  'cancelled', '付款超时信誉测试 API 服务', 1, 'manual_usage_check',
		  20, 'CNY', 'wechat', 10,
		  $6, '站外确认付款',
		  $7, 'payment_timeout', $8, $7, $9
		)
	`, orderID, intentID, serviceID, buyerID, sellerID, cancelledAt, cancelledAt, orderCreatedAt, orderNo); err != nil {
		t.Fatalf("seed timeout API order: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO api_order_events (
		  api_order_id, event_type, from_status, to_status, request_id, created_at
		)
		VALUES ($1, 'api_order.payment_timeout_cancelled', 'pending_payment', 'cancelled', 'reputation-timeout', $2)
	`, orderID, cancelledAt); err != nil {
		t.Fatalf("seed timeout API order event: %v", err)
	}
}

func assertReputationStateCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID string, expected int) {
	t.Helper()
	var count int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM user_reputation_states
		WHERE user_id = $1
	`, userID).Scan(&count); err != nil {
		t.Fatalf("count reputation states: %v", err)
	}
	if count != expected {
		t.Fatalf("expected %d reputation states for %s, got %d", expected, userID, count)
	}
}

func assertReputationHistoryCount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID string, expected int) {
	t.Helper()
	var count int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM user_reputation_history
		WHERE user_id = $1
	`, userID).Scan(&count); err != nil {
		t.Fatalf("count reputation history: %v", err)
	}
	if count != expected {
		t.Fatalf("expected %d reputation history rows for %s, got %d", expected, userID, count)
	}
}

func assertDirtyReputationStates(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID string, expected int) {
	t.Helper()
	var count int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM user_reputation_states
		WHERE user_id = $1
		  AND dirty_at IS NOT NULL
	`, userID).Scan(&count); err != nil {
		t.Fatalf("count dirty reputation states: %v", err)
	}
	if count != expected {
		t.Fatalf("expected %d dirty reputation states for %s, got %d", expected, userID, count)
	}
}

func recalculateAndAssertClean(
	t *testing.T,
	ctx context.Context,
	service *reputation.Service,
	pool *pgxpool.Pool,
	userID string,
) {
	t.Helper()
	if _, appErr := service.GetUserReputation(ctx, userID); appErr != nil {
		t.Fatalf("refresh dirty reputation snapshots: %v", appErr)
	}
	assertDirtyReputationStates(t, ctx, pool, userID, 0)
}
