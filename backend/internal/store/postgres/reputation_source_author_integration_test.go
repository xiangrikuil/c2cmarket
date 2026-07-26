package postgres

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/module/reputation"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSourceAuthorVerificationPostgresLifecycleAndAudit(t *testing.T) {
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
	sellerID := uuid.NewString()
	sellerContactID := uuid.NewString()
	buyerID := uuid.NewString()
	buyerContactID := uuid.NewString()
	serviceID := uuid.NewString()
	adminID := uuid.NewString()
	seedQuotaServiceForTest(t, ctx, pool, sellerID, sellerContactID, buyerID, buyerContactID, serviceID, now)
	if _, err := pool.Exec(ctx, `
		INSERT INTO users (id, username, display_name, account_status, created_at, updated_at)
		VALUES ($1, $2, '原帖核验管理员', 'active', $3, $3)
	`, adminID, "source-admin-"+strings.ReplaceAll(adminID[:8], "-", ""), now); err != nil {
		t.Fatalf("seed source verification admin: %v", err)
	}
	externalUserID := "linux-" + strings.ReplaceAll(sellerID[:8], "-", "")
	if _, err := pool.Exec(ctx, `
		INSERT INTO linux_do_bindings (
		  id, user_id, linux_do_user_id, linux_do_username,
		  trust_level, bound_at, last_synced_at
		)
		VALUES ($1, $2, $3, $3, 1, $4, $4)
	`, uuid.NewString(), sellerID, externalUserID, now); err != nil {
		t.Fatalf("seed seller linux.do binding: %v", err)
	}
	sourceURL := "https://linux.do/t/source-author/" + strings.ReplaceAll(serviceID[:8], "-", "")
	if _, err := pool.Exec(ctx, `
		UPDATE api_services
		SET source_url = $2, updated_at = $3
		WHERE id = $1
	`, serviceID, sourceURL, now); err != nil {
		t.Fatalf("set API service source URL: %v", err)
	}

	store := &Store{pool: pool}
	service := reputation.NewService(store, func() time.Time { return now })
	actor := reputation.AdminActor{UserID: adminID, IsAdmin: true}

	initial, appErr := service.GetSourceAuthorVerificationAudit(ctx, actor, reputation.SourceResourceAPIService, serviceID)
	if appErr != nil {
		t.Fatalf("read initial source verification: %v", appErr)
	}
	if initial.Verification.Status != reputation.SourceVerificationNotSubmitted ||
		initial.Verification.Version != 0 ||
		len(initial.Events) != 0 {
		t.Fatalf("unexpected initial verification: %#v", initial)
	}

	expiresAt := now.Add(time.Hour)
	verified, appErr := service.UpdateSourceAuthorVerification(ctx, actor, reputation.UpdateSourceAuthorVerificationInput{
		ResourceType:         reputation.SourceResourceAPIService,
		ResourceID:           serviceID,
		Status:               reputation.SourceVerificationVerified,
		ActualExternalUserID: externalUserID,
		VerificationMethod:   "manual_topic_review",
		ExpiresAt:            &expiresAt,
		ExpectedVersion:      0,
	})
	if appErr != nil {
		t.Fatalf("create verified source decision: %v", appErr)
	}
	if verified.Verification.Version != 1 ||
		verified.Verification.Status != reputation.SourceVerificationVerified ||
		len(verified.Events) != 1 ||
		verified.Events[0].Action != "created" {
		t.Fatalf("unexpected verified audit: %#v", verified)
	}

	facts, appErr := store.AggregateFacts(ctx, []string{sellerID, buyerID}, now)
	if appErr != nil {
		t.Fatalf("aggregate verified source facts: %v", appErr)
	}
	sellerAPI := facts[sellerID].Seller.API
	if sellerAPI.SourceAuthorVerification.State != reputation.SourceAggregateVerified ||
		sellerAPI.SourceAuthorVerification.Counts.Verified != 1 ||
		sellerAPI.NextRecalculationAt == nil ||
		!sellerAPI.NextRecalculationAt.Equal(expiresAt) {
		t.Fatalf("unexpected verified seller source facts: %#v", sellerAPI)
	}
	if facts[buyerID].Buyer.API.SourceAuthorVerification.State != reputation.SourceAggregateNotApplicable {
		t.Fatalf("buyer source aggregate must be not applicable: %#v", facts[buyerID].Buyer.API.SourceAuthorVerification)
	}

	expired, appErr := store.GetSourceAuthorVerificationAudit(
		ctx,
		reputation.SourceResourceAPIService,
		serviceID,
		expiresAt,
	)
	if appErr != nil || expired.Verification.Status != reputation.SourceVerificationExpired {
		t.Fatalf("expected effective expiry, audit=%#v err=%v", expired, appErr)
	}

	if _, appErr := service.UpdateSourceAuthorVerification(ctx, actor, reputation.UpdateSourceAuthorVerificationInput{
		ResourceType:    reputation.SourceResourceAPIService,
		ResourceID:      serviceID,
		Status:          reputation.SourceVerificationPending,
		ExpectedVersion: 0,
	}); appErr == nil || appErr.Status != http.StatusPreconditionFailed {
		t.Fatalf("expected stale version rejection, got %#v", appErr)
	}

	changedURL := sourceURL + "-changed"
	if _, err := pool.Exec(ctx, `
		UPDATE api_services
		SET source_url = $2, updated_at = $3
		WHERE id = $1
	`, serviceID, changedURL, now.Add(2*time.Hour)); err != nil {
		t.Fatalf("change API service source URL: %v", err)
	}
	drifted, appErr := service.GetSourceAuthorVerificationAudit(ctx, actor, reputation.SourceResourceAPIService, serviceID)
	if appErr != nil || drifted.Verification.Status != reputation.SourceVerificationPending {
		t.Fatalf("source URL drift must become pending, audit=%#v err=%v", drifted, appErr)
	}

	mismatch, appErr := service.UpdateSourceAuthorVerification(ctx, actor, reputation.UpdateSourceAuthorVerificationInput{
		ResourceType:         reputation.SourceResourceAPIService,
		ResourceID:           serviceID,
		Status:               reputation.SourceVerificationMismatch,
		ActualExternalUserID: "different-linux-user",
		VerificationMethod:   "manual_topic_review",
		FailureReason:        "原帖作者与资源所有者绑定身份不一致。",
		ExpectedVersion:      1,
	})
	if appErr != nil {
		t.Fatalf("save mismatch source decision: %v", appErr)
	}
	if mismatch.Verification.Version != 2 ||
		mismatch.Verification.Status != reputation.SourceVerificationMismatch ||
		len(mismatch.Events) != 2 ||
		mismatch.Events[0].Action != "updated" {
		t.Fatalf("unexpected mismatch audit: %#v", mismatch)
	}

	facts, appErr = store.AggregateFacts(ctx, []string{sellerID}, now.Add(2*time.Hour))
	if appErr != nil {
		t.Fatalf("aggregate mismatch source facts: %v", appErr)
	}
	if facts[sellerID].Seller.API.SourceAuthorVerification.State != reputation.SourceAggregateMismatch ||
		!facts[sellerID].Seller.API.SourceAuthorMismatch {
		t.Fatalf("mismatch must enter seller caution facts: %#v", facts[sellerID].Seller.API)
	}

	evidence, appErr := store.LoadAdminReputationEvidence(ctx, sellerID, now.Add(2*time.Hour))
	if appErr != nil {
		t.Fatalf("load source-author admin evidence: %v", appErr)
	}
	if len(evidence.SourceAuthorVerifications) != 1 {
		t.Fatalf("expected one source-author audit, got %#v", evidence.SourceAuthorVerifications)
	}
	sourceAudit := evidence.SourceAuthorVerifications[0]
	if sourceAudit.Verification.ResourceType != reputation.SourceResourceAPIService ||
		sourceAudit.Verification.ResourceID != serviceID ||
		sourceAudit.Verification.Status != reputation.SourceVerificationMismatch ||
		len(sourceAudit.Events) != 2 {
		t.Fatalf("unexpected source-author admin audit: %#v", sourceAudit)
	}

	if _, err := pool.Exec(ctx, `
		UPDATE source_author_verification_events
		SET failure_reason = 'rewritten'
		WHERE resource_type = 'api_service' AND resource_id = $1
	`, serviceID); err == nil {
		t.Fatal("append-only source verification event update unexpectedly succeeded")
	}
}
