package postgres

import (
	"context"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/growth"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestGrowthOverviewPostgresMetricsAndDailyActivity(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("C2C_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not configured")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect growth test database: %v", err)
	}
	defer pool.Close()
	requireGrowthTestDatabase(t, ctx, pool)

	store := &Store{pool: pool}
	shanghai := time.FixedZone("Asia/Shanghai", 8*60*60)
	asOf := time.Date(2026, 8, 2, 12, 0, 0, 0, shanghai)

	empty, appErr := store.GrowthOverview(ctx, asOf, 7)
	if appErr != nil {
		t.Fatalf("load empty growth overview: %v", appErr)
	}
	if empty.GeneratedAt != asOf.UTC() || empty.Timezone != "Asia/Shanghai" || empty.WindowDays != 7 {
		t.Fatalf("unexpected empty snapshot metadata: %#v", empty)
	}
	if empty.Summary != (growth.Summary{}) {
		t.Fatalf("expected zero-valued empty summary, got %#v", empty.Summary)
	}
	assertZeroFilledGrowthSeries(t, empty, "2026-07-27", "2026-08-02")
	assertOAuthRegistrationAttributionFirstTouch(t, ctx, pool, store, asOf.Add(-60*24*time.Hour))

	cohortCreatedAt := time.Date(2026, 7, 20, 10, 0, 0, 0, shanghai)
	sellerID := uuid.NewString()
	sellerContactID := uuid.NewString()
	buyerID := uuid.NewString()
	buyerContactID := uuid.NewString()
	serviceID := uuid.NewString()
	seedQuotaServiceForTest(t, ctx, pool, sellerID, sellerContactID, buyerID, buyerContactID, serviceID, cohortCreatedAt)
	assertAPIServiceFirstPublishedAtImmutable(t, ctx, pool, serviceID, cohortCreatedAt.Add(48*time.Hour))
	seedGrowthAttribution(t, ctx, pool, sellerID, "campaign", "linux.do", "community", "summer", cohortCreatedAt)
	seedGrowthAttribution(t, ctx, pool, buyerID, "referral", "linux.do", "", "", cohortCreatedAt)

	apiCompletedAt := cohortCreatedAt.Add(10 * time.Hour)
	seedCompletedAPIOrderForReview(
		t,
		ctx,
		pool,
		serviceID,
		sellerID,
		sellerContactID,
		buyerID,
		buyerContactID,
		apiCompletedAt,
	)

	oldCreatedAt := time.Date(2026, 6, 1, 10, 0, 0, 0, shanghai)
	oldSellerID := uuid.NewString()
	oldSellerContactID := uuid.NewString()
	oldBuyerID := uuid.NewString()
	oldBuyerContactID := uuid.NewString()
	oldServiceID := uuid.NewString()
	seedQuotaServiceForTest(t, ctx, pool, oldSellerID, oldSellerContactID, oldBuyerID, oldBuyerContactID, oldServiceID, oldCreatedAt)
	membershipID := seedEndedCarpoolMembershipWithoutReview(
		t,
		ctx,
		pool,
		oldSellerID,
		oldSellerContactID,
		oldBuyerID,
		oldBuyerContactID,
		time.Date(2026, 7, 30, 12, 0, 0, 0, shanghai),
	)
	assertCarpoolFirstPublishedAtImmutable(t, ctx, pool, membershipID, oldCreatedAt.Add(48*time.Hour))

	immatureCreatedAt := time.Date(2026, 8, 2, 9, 0, 0, 0, shanghai)
	immatureUserID := seedGrowthUser(t, ctx, pool, "growth-immature", immatureCreatedAt)
	seedGrowthAttribution(t, ctx, pool, immatureUserID, "direct", "direct", "", "", immatureCreatedAt)

	if appErr := store.RecordActivity(ctx, buyerID, "2026-07-21", cohortCreatedAt.Add(24*time.Hour)); appErr != nil {
		t.Fatalf("record D1 activity: %v", appErr)
	}
	if appErr := store.RecordActivity(ctx, buyerID, "2026-07-27", cohortCreatedAt.Add(7*24*time.Hour)); appErr != nil {
		t.Fatalf("record D7 activity: %v", appErr)
	}
	firstSeenAt := time.Date(2026, 8, 2, 8, 0, 0, 0, shanghai)
	if appErr := store.RecordActivity(ctx, sellerID, "2026-08-02", firstSeenAt); appErr != nil {
		t.Fatalf("record daily activity: %v", appErr)
	}
	if appErr := store.RecordActivity(ctx, sellerID, "2026-08-02", firstSeenAt.Add(2*time.Hour)); appErr != nil {
		t.Fatalf("repeat daily activity: %v", appErr)
	}
	var activityRows int
	var storedFirstSeenAt time.Time
	if err := pool.QueryRow(ctx, `
		SELECT count(*), min(first_seen_at)
		FROM user_activity_daily
		WHERE user_id = $1 AND activity_date = '2026-08-02'
	`, sellerID).Scan(&activityRows, &storedFirstSeenAt); err != nil {
		t.Fatalf("read deduplicated activity: %v", err)
	}
	if activityRows != 1 || !storedFirstSeenAt.Equal(firstSeenAt.UTC()) {
		t.Fatalf("daily activity was not first-write-wins: rows=%d firstSeenAt=%s", activityRows, storedFirstSeenAt)
	}

	overview, appErr := store.GrowthOverview(ctx, asOf, 30)
	if appErr != nil {
		t.Fatalf("load populated growth overview: %v", appErr)
	}
	if overview.GeneratedAt != asOf.UTC() || overview.Timezone != "Asia/Shanghai" || overview.WindowDays != 30 {
		t.Fatalf("unexpected populated snapshot metadata: %#v", overview)
	}
	if overview.Summary.NewUsersToday != 1 || overview.Summary.NewUsers7d != 1 || overview.Summary.NewUsers30d != 3 || overview.Summary.NewUsersInWindow != 3 {
		t.Fatalf("unexpected registration summary: %#v", overview.Summary)
	}
	if overview.Summary.CumulativeEffectiveUsers != 5 {
		t.Fatalf("expected five effective users, got %#v", overview.Summary)
	}
	if overview.Summary.DAU != 1 || overview.Summary.WAU != 2 || overview.Summary.MAU != 2 {
		t.Fatalf("unexpected DAU/WAU/MAU: %#v", overview.Summary)
	}
	if overview.Summary.CompletedCarpoolTransactions != 0 || overview.Summary.CompletedAPITransactions != 1 {
		t.Fatalf("unexpected completed transaction totals: %#v", overview.Summary)
	}
	assertGrowthRatio(t, "summary activation", overview.Summary.ActivationRate, 1)
	assertGrowthRatio(t, "summary D1 retention", overview.Summary.D1RetentionRate, 0.5)
	assertGrowthRatio(t, "summary D7 retention", overview.Summary.D7RetentionRate, 0.5)
	assertGrowthRatio(t, "median activation hours", overview.Summary.MedianActivationHours, 3)

	if overview.Activation.CohortUsers != 2 || overview.Activation.BuyerActivatedUsers != 1 || overview.Activation.SellerActivatedUsers != 1 || overview.Activation.ActivatedUsers != 2 {
		t.Fatalf("unexpected activation counts: %#v", overview.Activation)
	}
	assertGrowthRatio(t, "buyer activation", overview.Activation.BuyerActivationRate, 0.5)
	assertGrowthRatio(t, "seller activation", overview.Activation.SellerActivationRate, 0.5)
	assertGrowthRatio(t, "overall activation", overview.Activation.ActivationRate, 1)

	assertGrowthTrendPoint(t, overview.RegistrationTrend, "2026-07-20", 2, 4)
	assertGrowthTrendPoint(t, overview.RegistrationTrend, "2026-08-02", 1, 5)
	assertGrowthActivityPoint(t, overview.ActivityTrend, "2026-07-21", 1)
	assertGrowthActivityPoint(t, overview.ActivityTrend, "2026-07-27", 1)
	assertGrowthActivityPoint(t, overview.ActivityTrend, "2026-08-02", 1)
	assertGrowthAttribution(t, overview.Attribution, "campaign", "linux.do", "community", "summer", 1, 1.0/3.0)
	assertGrowthAttribution(t, overview.Attribution, "referral", "linux.do", "", "", 1, 1.0/3.0)
	assertGrowthAttribution(t, overview.Attribution, "direct", "direct", "", "", 1, 1.0/3.0)

	mature := growthRetentionCohort(t, overview, "2026-07-20")
	if mature.RegisteredUsers != 2 || mature.D1RetainedUsers == nil || *mature.D1RetainedUsers != 1 || mature.D7RetainedUsers == nil || *mature.D7RetainedUsers != 1 {
		t.Fatalf("unexpected mature retention cohort: %#v", mature)
	}
	assertGrowthRatio(t, "mature D1 cohort", mature.D1Rate, 0.5)
	assertGrowthRatio(t, "mature D7 cohort", mature.D7Rate, 0.5)
	immature := growthRetentionCohort(t, overview, "2026-08-02")
	if immature.RegisteredUsers != 1 || immature.D1RetainedUsers != nil || immature.D1Rate != nil || immature.D7RetainedUsers != nil || immature.D7Rate != nil {
		t.Fatalf("immature retention cohort must use null rates: %#v", immature)
	}
	incompleteD1Observation := growthRetentionCohort(t, overview, "2026-08-01")
	if incompleteD1Observation.D1RetainedUsers != nil || incompleteD1Observation.D1Rate != nil {
		t.Fatalf("D1 observation day must finish before the cohort matures: %#v", incompleteD1Observation)
	}
	incompleteD7Observation := growthRetentionCohort(t, overview, "2026-07-26")
	if incompleteD7Observation.D7RetainedUsers != nil || incompleteD7Observation.D7Rate != nil {
		t.Fatalf("D7 observation day must finish before the cohort matures: %#v", incompleteD7Observation)
	}

	repeated, appErr := store.GrowthOverview(ctx, asOf, 30)
	if appErr != nil {
		t.Fatalf("repeat populated growth overview: %v", appErr)
	}
	if !reflect.DeepEqual(overview, repeated) {
		t.Fatalf("unchanged data produced different snapshots:\nfirst=%#v\nsecond=%#v", overview, repeated)
	}
}

func assertOAuthRegistrationAttributionFirstTouch(t *testing.T, ctx context.Context, pool *pgxpool.Pool, store *Store, now time.Time) {
	t.Helper()
	subject := "growth-oauth-" + uuid.NewString()
	first := auth.OAuthProfile{
		Provider:        "linux.do",
		Subject:         subject,
		Username:        "growth-oauth-user",
		DisplayName:     "增长归因用户",
		TrustLevel:      2,
		LinuxDoUserID:   subject,
		LinuxDoUsername: "growth-oauth-user",
		Attribution: auth.RegistrationAttribution{
			Source:       "linux.do",
			Medium:       "community",
			Campaign:     "phase-two",
			ReferrerHost: "linux.do",
			LandingPath:  "/carpools/private-id",
		},
	}
	created, appErr := store.UpsertOAuthUser(ctx, first, now)
	if appErr != nil || !created.Created {
		t.Fatalf("create OAuth user with attribution: result=%#v err=%v", created, appErr)
	}
	if created.User.AnalyticsUserID == "" || created.User.AnalyticsUserID == created.User.ID {
		t.Fatalf("OAuth user must receive an opaque analytics ID: %#v", created.User)
	}

	second := first
	second.Attribution = auth.RegistrationAttribution{
		Source:      "overwrite-attempt",
		Medium:      "paid",
		Campaign:    "later-login",
		LandingPath: "/api-market/private-id",
	}
	reused, appErr := store.UpsertOAuthUser(ctx, second, now.Add(time.Hour))
	if appErr != nil || reused.Created || reused.User.ID != created.User.ID {
		t.Fatalf("reuse OAuth identity: result=%#v err=%v", reused, appErr)
	}

	var method, sourceType, source, medium, campaign, landingPath string
	if err := pool.QueryRow(ctx, `
		SELECT registration_method, source_type, source, COALESCE(medium, ''),
		       COALESCE(campaign, ''), landing_path
		FROM user_registration_attributions
		WHERE user_id = $1
	`, created.User.ID).Scan(&method, &sourceType, &source, &medium, &campaign, &landingPath); err != nil {
		t.Fatalf("read OAuth first-touch attribution: %v", err)
	}
	if method != "oauth_linux_do" || sourceType != "campaign" || source != "linux.do" || medium != "community" || campaign != "phase-two" || landingPath != "/carpools/:id" {
		t.Fatalf("OAuth attribution was not first-touch: method=%q type=%q source=%q medium=%q campaign=%q landing=%q", method, sourceType, source, medium, campaign, landingPath)
	}

	if _, err := pool.Exec(ctx, `DELETE FROM auth_identities WHERE user_id = $1`, created.User.ID); err != nil {
		t.Fatalf("remove OAuth identity fixture before metric assertions: %v", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, created.User.ID); err != nil {
		t.Fatalf("remove OAuth attribution fixture before metric assertions: %v", err)
	}
}

func assertAPIServiceFirstPublishedAtImmutable(t *testing.T, ctx context.Context, pool *pgxpool.Pool, serviceID string, later time.Time) {
	t.Helper()
	var firstPublishedAt time.Time
	if err := pool.QueryRow(ctx, `SELECT first_published_at FROM api_services WHERE id = $1`, serviceID).Scan(&firstPublishedAt); err != nil {
		t.Fatalf("read API service first publication: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE api_services
		SET publication_status = 'owner_paused', first_published_at = $2, updated_at = $2
		WHERE id = $1
	`, serviceID, later); err != nil {
		t.Fatalf("pause API service while testing first publication: %v", err)
	}
	var preserved time.Time
	if err := pool.QueryRow(ctx, `SELECT first_published_at FROM api_services WHERE id = $1`, serviceID).Scan(&preserved); err != nil {
		t.Fatalf("read preserved API service first publication: %v", err)
	}
	if !preserved.Equal(firstPublishedAt) {
		t.Fatalf("API service first publication changed: before=%s after=%s", firstPublishedAt, preserved)
	}
	if _, err := pool.Exec(ctx, `UPDATE api_services SET publication_status = 'online', updated_at = $2 WHERE id = $1`, serviceID, later.Add(time.Hour)); err != nil {
		t.Fatalf("restore API service after first-publication assertion: %v", err)
	}
}

func assertCarpoolFirstPublishedAtImmutable(t *testing.T, ctx context.Context, pool *pgxpool.Pool, membershipID string, later time.Time) {
	t.Helper()
	var listingID string
	var firstPublishedAt time.Time
	if err := pool.QueryRow(ctx, `
		SELECT listing.id::text, listing.first_published_at
		FROM carpool_memberships membership
		JOIN carpool_listings listing ON listing.id = membership.carpool_listing_id
		WHERE membership.id = $1
	`, membershipID).Scan(&listingID, &firstPublishedAt); err != nil {
		t.Fatalf("read carpool first publication: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE carpool_listings
		SET governance_status = 'removed', first_published_at = $2, updated_at = $2
		WHERE id = $1
	`, listingID, later); err != nil {
		t.Fatalf("remove carpool while testing first publication: %v", err)
	}
	var preserved time.Time
	if err := pool.QueryRow(ctx, `SELECT first_published_at FROM carpool_listings WHERE id = $1`, listingID).Scan(&preserved); err != nil {
		t.Fatalf("read preserved carpool first publication: %v", err)
	}
	if !preserved.Equal(firstPublishedAt) {
		t.Fatalf("carpool first publication changed: before=%s after=%s", firstPublishedAt, preserved)
	}
	if _, err := pool.Exec(ctx, `UPDATE carpool_listings SET governance_status = 'clear', updated_at = $2 WHERE id = $1`, listingID, later.Add(time.Hour)); err != nil {
		t.Fatalf("restore carpool after first-publication assertion: %v", err)
	}
}

func requireGrowthTestDatabase(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	var databaseName string
	if err := pool.QueryRow(ctx, `SELECT current_database()`).Scan(&databaseName); err != nil {
		t.Fatalf("read growth test database name: %v", err)
	}
	if databaseName != "c2c_growth_test" {
		t.Fatalf("refusing to run growth integration test against non-dedicated database %q", databaseName)
	}
}

func seedGrowthUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, prefix string, createdAt time.Time) string {
	t.Helper()
	userID := uuid.NewString()
	username := prefix + "-" + strings.ReplaceAll(userID[:8], "-", "")
	if _, err := pool.Exec(ctx, `
		INSERT INTO users (id, username, display_name, account_status, created_at, updated_at)
		VALUES ($1, $2, $2, 'active', $3, $3)
	`, userID, username, createdAt); err != nil {
		t.Fatalf("seed growth user: %v", err)
	}
	return userID
}

func seedGrowthAttribution(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID, sourceType, source, medium, campaign string, capturedAt time.Time) {
	t.Helper()
	if _, err := pool.Exec(ctx, `
		INSERT INTO user_registration_attributions (
		  user_id, registration_method, source_type, source, medium, campaign,
		  landing_path, captured_at
		)
		VALUES ($1, 'oauth_linux_do', $2, $3, NULLIF($4, ''), NULLIF($5, ''), '/', $6)
	`, userID, sourceType, source, medium, campaign, capturedAt); err != nil {
		t.Fatalf("seed growth attribution: %v", err)
	}
}

func assertZeroFilledGrowthSeries(t *testing.T, overview growth.Overview, startDate, endDate string) {
	t.Helper()
	if len(overview.RegistrationTrend) != overview.WindowDays || len(overview.ActivityTrend) != overview.WindowDays {
		t.Fatalf(
			"expected %d zero-filled trend points, got registrations=%d activity=%d",
			overview.WindowDays,
			len(overview.RegistrationTrend),
			len(overview.ActivityTrend),
		)
	}
	if overview.RegistrationTrend[0].Date != startDate || overview.RegistrationTrend[len(overview.RegistrationTrend)-1].Date != endDate {
		t.Fatalf("unexpected registration trend bounds: %#v", overview.RegistrationTrend)
	}
	if overview.ActivityTrend[0].Date != startDate || overview.ActivityTrend[len(overview.ActivityTrend)-1].Date != endDate {
		t.Fatalf("unexpected activity trend bounds: %#v", overview.ActivityTrend)
	}
	for _, point := range overview.RegistrationTrend {
		if point.NewUsers != 0 || point.CumulativeUsers != 0 {
			t.Fatalf("expected zero-filled registration point, got %#v", point)
		}
	}
	for _, point := range overview.ActivityTrend {
		if point.ActiveUsers != 0 {
			t.Fatalf("expected zero-filled activity point, got %#v", point)
		}
	}
}

func assertGrowthRatio(t *testing.T, label string, actual *float64, expected float64) {
	t.Helper()
	if actual == nil || math.Abs(*actual-expected) > 0.000001 {
		t.Fatalf("%s: expected %.6f, got %#v", label, expected, actual)
	}
}

func assertGrowthTrendPoint(t *testing.T, points []growth.RegistrationTrendPoint, date string, newUsers, cumulativeUsers int) {
	t.Helper()
	for _, point := range points {
		if point.Date == date {
			if point.NewUsers != newUsers || point.CumulativeUsers != cumulativeUsers {
				t.Fatalf("unexpected registration trend point for %s: %#v", date, point)
			}
			return
		}
	}
	t.Fatalf("registration trend point %s is missing: %#v", date, points)
}

func assertGrowthActivityPoint(t *testing.T, points []growth.ActivityTrendPoint, date string, activeUsers int) {
	t.Helper()
	for _, point := range points {
		if point.Date == date {
			if point.ActiveUsers != activeUsers {
				t.Fatalf("unexpected activity trend point for %s: %#v", date, point)
			}
			return
		}
	}
	t.Fatalf("activity trend point %s is missing: %#v", date, points)
}

func assertGrowthAttribution(
	t *testing.T,
	groups []growth.AttributionGroup,
	sourceType, source, medium, campaign string,
	registrations int,
	share float64,
) {
	t.Helper()
	for _, group := range groups {
		if group.SourceType == sourceType && group.Source == source && group.Medium == medium && group.Campaign == campaign {
			if group.Registrations != registrations || math.Abs(group.Share-share) > 0.000001 {
				t.Fatalf("unexpected attribution group: %#v", group)
			}
			return
		}
	}
	t.Fatalf("attribution group %s/%s/%s/%s is missing: %#v", sourceType, source, medium, campaign, groups)
}

func growthRetentionCohort(t *testing.T, overview growth.Overview, date string) growth.RetentionCohort {
	t.Helper()
	for _, cohort := range overview.RetentionCohorts {
		if cohort.CohortDate == date {
			return cohort
		}
	}
	t.Fatalf("retention cohort %s is missing: %#v", date, overview.RetentionCohorts)
	return growth.RetentionCohort{}
}
