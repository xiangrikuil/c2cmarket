package postgres

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/carpool"
	"c2c-market/backend/internal/module/catalog"
	"c2c-market/backend/internal/module/contact"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/google/uuid"
)

func TestPostgresCarpoolListingAuditAndIdempotencyAreAtomic(t *testing.T) {
	databaseURL := strings.TrimSpace(os.Getenv("C2C_TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("C2C_TEST_DATABASE_URL is not configured")
	}
	ctx := context.Background()
	store, err := Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	t.Cleanup(store.Close)

	userID := uuid.NewString()
	buyerUserID := uuid.NewString()
	username := "carpool-audit-" + strings.ToLower(uuid.NewString()[:8])
	buyerUsername := "carpool-buyer-audit-" + strings.ToLower(uuid.NewString()[:8])
	if _, err := store.pool.Exec(ctx, `
		INSERT INTO users (id, username, display_name, account_status, created_at, updated_at)
		VALUES ($1, $2, 'Carpool Audit Test', 'active', now(), now()),
		       ($3, $4, 'Carpool Buyer Audit Test', 'active', now(), now())
	`, userID, username, buyerUserID, buyerUsername); err != nil {
		t.Fatalf("insert carpool audit user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = store.pool.Exec(context.Background(), `
			DELETE FROM notifications WHERE user_id IN ($1, $2);
			DELETE FROM domain_events
			WHERE actor_user_id IN ($1, $2)
			   OR aggregate_id IN (SELECT id FROM carpool_applications WHERE buyer_user_id = $2 OR owner_user_id = $1)
			   OR aggregate_id IN (SELECT id FROM carpool_listings WHERE owner_user_id = $1);
			DELETE FROM idempotency_keys WHERE user_id IN ($1, $2);
			DELETE FROM carpool_listings WHERE owner_user_id = $1;
			UPDATE contact_methods SET current_version_id = NULL WHERE user_id IN ($1, $2);
			DELETE FROM contact_method_versions WHERE owner_user_id IN ($1, $2);
			DELETE FROM contact_methods WHERE user_id IN ($1, $2);
			DELETE FROM users WHERE id IN ($1, $2);
		`, userID, buyerUserID)
	})

	now := time.Date(2026, 8, 12, 14, 0, 0, 0, time.UTC)
	nowFn := func() time.Time { return now }
	contactService := contact.NewService(store, nowFn)
	ownerContact, appErr := contactService.CreateMethod(ctx, contact.ContactMethodInput{
		UserID: userID, Type: "wechat", Label: "拼车微信", Value: "audit-owner-contact",
		UsageScopes: []string{contact.UsageScopeCarpoolOwner}, Enabled: true, RequestID: "owner-contact-create",
	})
	if appErr != nil {
		t.Fatalf("create owner contact: %v", appErr)
	}
	buyerOnlyContact, appErr := contactService.CreateMethod(ctx, contact.ContactMethodInput{
		UserID: userID, Type: "telegram", Label: "买家联系", Value: "audit-buyer-contact",
		UsageScopes: []string{contact.UsageScopeBuyer}, Enabled: true, RequestID: "buyer-contact-create",
	})
	if appErr != nil {
		t.Fatalf("create buyer contact: %v", appErr)
	}
	applicationBuyerContact, appErr := contactService.CreateMethod(ctx, contact.ContactMethodInput{
		UserID: buyerUserID, Type: "telegram", Label: "申请人联系", Value: "audit-application-buyer-contact",
		UsageScopes: []string{contact.UsageScopeBuyer}, Enabled: true, RequestID: "application-buyer-contact-create",
	})
	if appErr != nil {
		t.Fatalf("create application buyer contact: %v", appErr)
	}
	idempotencyService := idempotency.NewService(store, nowFn)
	service := carpool.NewService(store, catalog.NewService(store, idempotencyService, nil, nowFn), contactService, idempotencyService, nowFn)
	owner := auth.User{ID: userID}
	input := validCarpoolListingAuditInput(ownerContact.ID, "carpool-create-request")
	completionBuilder := func(listing carpool.Listing) (idempotency.Completion, *domain.AppError) {
		return idempotency.Completion{
			Status: 201, ContentType: "application/json", Body: []byte(`{"created":true}`),
			ResourceType: "carpool_listing", ResourceID: listing.ID,
		}, nil
	}
	listing, _, created, appErr := service.CreateListingWithIdempotency(ctx, owner, "carpool-create", "create-key", "create-hash", input, completionBuilder)
	if appErr != nil || !created {
		t.Fatalf("create listing: listing=%+v created=%t error=%v", listing, created, appErr)
	}
	_, _, created, appErr = service.CreateListingWithIdempotency(ctx, owner, "carpool-create", "create-key", "create-hash", input, completionBuilder)
	if appErr != nil || created {
		t.Fatalf("replay listing create: created=%t error=%v", created, appErr)
	}
	var eventCount, completedCount int
	var eventType, requestID, metadata string
	if err := store.pool.QueryRow(ctx, `
		SELECT count(*)::int, min(event_type), min(request_id), min(metadata_json::text)
		FROM domain_events
		WHERE aggregate_type = 'carpool_listing' AND aggregate_id = $1
	`, listing.ID).Scan(&eventCount, &eventType, &requestID, &metadata); err != nil {
		t.Fatalf("read listing audit event: %v", err)
	}
	if eventCount != 1 || eventType != "carpool_listing.created" || requestID != "carpool-create-request" {
		t.Fatalf("unexpected listing audit event: count=%d type=%q request=%q", eventCount, eventType, requestID)
	}
	if strings.Contains(metadata, "audit-owner-contact") {
		t.Fatalf("listing audit metadata leaked contact value: %s", metadata)
	}
	if err := store.pool.QueryRow(ctx, `
		SELECT count(*)::int FROM idempotency_keys
		WHERE user_id = $1 AND route_key = 'carpool-create' AND idempotency_key = 'create-key' AND status = 'completed'
	`, userID).Scan(&completedCount); err != nil || completedCount != 1 {
		t.Fatalf("completed idempotency count=%d error=%v", completedCount, err)
	}

	now = now.Add(time.Minute)
	updateInput := validCarpoolListingUpdateAuditInput(listing, ownerContact.ID, "carpool-update-request")
	updated, _, changed, appErr := service.UpdateListingWithIdempotency(ctx, owner, "carpool-update", "update-key", "update-hash", updateInput, completionBuilder)
	if appErr != nil || !changed || updated.Version != listing.Version+1 {
		t.Fatalf("update listing atomically: listing=%+v changed=%t error=%v", updated, changed, appErr)
	}
	if _, _, changed, appErr = service.UpdateListingWithIdempotency(ctx, owner, "carpool-update", "update-key", "update-hash", updateInput, completionBuilder); appErr != nil || changed {
		t.Fatalf("replay listing update: changed=%t error=%v", changed, appErr)
	}
	rollbackUpdate := validCarpoolListingUpdateAuditInput(updated, ownerContact.ID, "carpool-update-builder-failure")
	rollbackUpdate.Title = "不应持久化的车源标题"
	if _, _, _, appErr = service.UpdateListingWithIdempotency(
		ctx, owner, "carpool-update-builder-failure", "update-builder-failure-key", "update-builder-failure-hash", rollbackUpdate,
		func(carpool.Listing) (idempotency.Completion, *domain.AppError) {
			return idempotency.Completion{}, domain.NewError(500, domain.CodeInternalError, "Encoding failed", "测试响应编码失败。")
		},
	); appErr == nil || appErr.Code != domain.CodeInternalError {
		t.Fatalf("expected listing update completion builder failure, got %#v", appErr)
	}
	persisted, appErr := service.MyListing(ctx, owner, updated.ID)
	if appErr != nil || persisted.Version != updated.Version || persisted.Title != updated.Title {
		t.Fatalf("failed listing update was not rolled back: listing=%+v error=%v", persisted, appErr)
	}

	now = now.Add(time.Minute)
	boundOwner := owner
	boundOwner.LinuxDoBinding = &auth.LinuxDoBinding{Bound: true, LinuxDoUserID: "audit-linuxdo", LinuxDoUsername: "audit-owner"}
	published, _, changed, appErr := service.SubmitListingForReviewWithIdempotency(ctx, boundOwner, "carpool-submit", "submit-key", "submit-hash", carpool.SubmitListingReviewInput{
		ListingID: updated.ID, ExpectedVersion: updated.Version, RequestID: "carpool-publish-request",
	}, completionBuilder)
	if appErr != nil || !changed || published.Status != carpool.ListingStatusActive {
		t.Fatalf("publish listing atomically: listing=%+v changed=%t error=%v", published, changed, appErr)
	}

	now = now.Add(time.Minute)
	applicationBuilder := func(application carpool.Application) (idempotency.Completion, *domain.AppError) {
		return idempotency.Completion{
			Status: 201, ContentType: "application/json", Body: []byte(`{"application":true}`),
			ResourceType: "carpool_application", ResourceID: application.ID,
		}, nil
	}
	buyer := auth.User{ID: buyerUserID, LinuxDoBinding: &auth.LinuxDoBinding{Bound: true, LinuxDoUserID: "audit-buyer-linuxdo", LinuxDoUsername: "audit-buyer"}}
	applicationInput := carpool.CreateApplicationInput{
		ListingID: published.ID, BuyerContactMethodID: applicationBuyerContact.ID, RequestID: "carpool-application-create-request",
	}
	application, _, created, appErr := service.CreateApplicationWithIdempotency(
		ctx, buyer, "carpool-application-create", "application-create-key", "application-create-hash", applicationInput, applicationBuilder,
	)
	if appErr != nil || !created || application.Status != carpool.ApplicationStatusPendingOwner {
		t.Fatalf("create application atomically: application=%+v created=%t error=%v", application, created, appErr)
	}
	if _, _, created, appErr = service.CreateApplicationWithIdempotency(
		ctx, buyer, "carpool-application-create", "application-create-key", "application-create-hash", applicationInput, applicationBuilder,
	); appErr != nil || created {
		t.Fatalf("replay application create: created=%t error=%v", created, appErr)
	}

	rejectInput := carpool.RejectApplicationInput{
		ApplicationID: application.ID, OwnerUserID: userID, Reason: "不符合拼车要求", ExpectedVersion: application.Version,
		RequestID: "carpool-application-reject-request",
	}
	if _, _, _, appErr = service.RejectApplicationWithIdempotency(
		ctx, userID, "carpool-application-reject-failure", "application-reject-failure-key", "application-reject-failure-hash", rejectInput,
		func(carpool.Application) (idempotency.Completion, *domain.AppError) {
			return idempotency.Completion{}, domain.NewError(500, domain.CodeInternalError, "Encoding failed", "测试响应编码失败。")
		},
	); appErr == nil || appErr.Code != domain.CodeInternalError {
		t.Fatalf("expected application reject builder failure, got %#v", appErr)
	}
	pending, appErr := service.OwnerApplication(ctx, owner, application.ID)
	if appErr != nil || pending.Status != carpool.ApplicationStatusPendingOwner || pending.Version != application.Version {
		t.Fatalf("failed application rejection was not rolled back: application=%+v error=%v", pending, appErr)
	}
	rejected, _, changed, appErr := service.RejectApplicationWithIdempotency(
		ctx, userID, "carpool-application-reject", "application-reject-key", "application-reject-hash", rejectInput, applicationBuilder,
	)
	if appErr != nil || !changed || rejected.Status != carpool.ApplicationStatusRejected {
		t.Fatalf("reject application atomically: application=%+v changed=%t error=%v", rejected, changed, appErr)
	}
	if _, _, changed, appErr = service.RejectApplicationWithIdempotency(
		ctx, userID, "carpool-application-reject", "application-reject-key", "application-reject-hash", rejectInput, applicationBuilder,
	); appErr != nil || changed {
		t.Fatalf("replay application rejection: changed=%t error=%v", changed, appErr)
	}
	var applicationActions []string
	applicationRows, err := store.pool.Query(ctx, `
		SELECT event_type FROM domain_events
		WHERE aggregate_type = 'carpool_application' AND aggregate_id = $1
		ORDER BY aggregate_version
	`, application.ID)
	if err != nil {
		t.Fatalf("query application lifecycle events: %v", err)
	}
	for applicationRows.Next() {
		var action string
		if err := applicationRows.Scan(&action); err != nil {
			applicationRows.Close()
			t.Fatalf("scan application lifecycle action: %v", err)
		}
		applicationActions = append(applicationActions, action)
	}
	applicationRows.Close()
	if want := []string{"carpool_application.created", "carpool_application.rejected"}; !slices.Equal(applicationActions, want) {
		t.Fatalf("application lifecycle actions = %v, want %v", applicationActions, want)
	}

	now = now.Add(time.Minute)
	joinedInput := applicationInput
	joinedInput.RequestID = "carpool-application-joined-create-request"
	joined, _, created, appErr := service.CreateApplicationWithIdempotency(
		ctx, buyer, "carpool-application-joined-create", "application-joined-create-key", "application-joined-create-hash", joinedInput, applicationBuilder,
	)
	if appErr != nil || !created {
		t.Fatalf("create joined application: application=%+v created=%t error=%v", joined, created, appErr)
	}
	joined, _, changed, appErr = service.AcceptApplicationWithIdempotency(
		ctx, userID, "carpool-application-accept", "application-accept-key", "application-accept-hash",
		carpool.AcceptApplicationInput{
			ApplicationID: joined.ID, OwnerUserID: userID, ExpectedVersion: joined.Version,
			RequestID: "carpool-application-accept-request",
		}, applicationBuilder,
	)
	if appErr != nil || !changed || joined.Status != carpool.ApplicationStatusJoined || joined.JoinedAt == nil {
		t.Fatalf("accept joined application: application=%+v changed=%t error=%v", joined, changed, appErr)
	}
	var joinedStatus, joinedActorKind string
	var joinedActorID *string
	if err := store.pool.QueryRow(ctx, `
		SELECT metadata_json->>'status', actor_kind, actor_user_id::text
		FROM domain_events
		WHERE aggregate_type = 'carpool_application'
		  AND aggregate_id = $1
		  AND event_type = 'carpool_application.joined'
	`, joined.ID).Scan(&joinedStatus, &joinedActorKind, &joinedActorID); err != nil {
		t.Fatalf("read joined event: %v", err)
	}
	if joinedStatus != carpool.ApplicationStatusJoined || joinedActorKind != "user" || joinedActorID == nil || *joinedActorID != userID {
		t.Fatalf("unexpected joined event: status=%q actorKind=%q actorID=%v", joinedStatus, joinedActorKind, joinedActorID)
	}

	now = now.Add(time.Minute)
	adminName := "carpool-admin-" + strings.ToLower(uuid.NewString()[:8])
	admin, appErr := store.EnsureUser(ctx, adminName, true, now)
	if appErr != nil {
		t.Fatalf("ensure carpool audit admin: %v", appErr)
	}
	t.Cleanup(func() {
		_, _ = store.pool.Exec(context.Background(), `
			DELETE FROM domain_events WHERE actor_user_id = $1;
			DELETE FROM idempotency_keys WHERE user_id = $1;
			DELETE FROM user_permissions WHERE user_id = $1;
			DELETE FROM users WHERE id = $1;
		`, admin.ID)
	})
	currentListing, appErr := service.AdminListing(ctx, admin, published.ID)
	if appErr != nil {
		t.Fatalf("read listing before governance removal: %v", appErr)
	}
	paused, _, changed, appErr := service.UpdateListingReviewStatusWithIdempotency(ctx, admin, "carpool-pause", "pause-key", "pause-hash", carpool.ReviewInput{
		ListingID: published.ID, Action: "pause", Status: carpool.ListingStatusPaused, Reason: "审计暂停", ExpectedVersion: currentListing.Version, RequestID: "carpool-pause-request",
	}, completionBuilder)
	if appErr != nil || !changed || paused.Status != currentListing.Status || paused.GovernanceStatus != "removed" {
		t.Fatalf("pause listing atomically: listing=%+v changed=%t error=%v", paused, changed, appErr)
	}
	if _, _, changed, appErr = service.UpdateListingReviewStatusWithIdempotency(ctx, admin, "carpool-pause", "pause-key", "pause-hash", carpool.ReviewInput{
		ListingID: published.ID, Action: "pause", Status: carpool.ListingStatusPaused, Reason: "审计暂停", ExpectedVersion: currentListing.Version, RequestID: "carpool-pause-request",
	}, completionBuilder); appErr != nil || changed {
		t.Fatalf("replay listing pause: changed=%t error=%v", changed, appErr)
	}
	var lifecycleActions []string
	rows, err := store.pool.Query(ctx, `
		SELECT event_type FROM domain_events
		WHERE aggregate_type = 'carpool_listing' AND aggregate_id = $1
		ORDER BY aggregate_version
	`, listing.ID)
	if err != nil {
		t.Fatalf("query listing lifecycle events: %v", err)
	}
	for rows.Next() {
		var action string
		if err := rows.Scan(&action); err != nil {
			rows.Close()
			t.Fatalf("scan listing lifecycle action: %v", err)
		}
		lifecycleActions = append(lifecycleActions, action)
	}
	rows.Close()
	wantLifecycle := []string{"carpool_listing.created", "carpool_listing.updated", "carpool_listing.published", "carpool_listing.paused"}
	if !slices.Equal(lifecycleActions, wantLifecycle) {
		t.Fatalf("listing lifecycle actions = %v, want %v", lifecycleActions, wantLifecycle)
	}

	invalidInput := validCarpoolListingAuditInput(buyerOnlyContact.ID, "invalid-scope-request")
	_, _, _, appErr = service.CreateListingWithIdempotency(ctx, owner, "carpool-create-invalid", "invalid-key", "invalid-hash", invalidInput, completionBuilder)
	if appErr == nil || appErr.Code != domain.CodeContactMethodNotOwned {
		t.Fatalf("buyer-only contact was accepted for listing: %#v", appErr)
	}
	var invalidListings, invalidEvents int
	if err := store.pool.QueryRow(ctx, `SELECT count(*)::int FROM carpool_listings WHERE owner_user_id = $1 AND id <> $2`, userID, listing.ID).Scan(&invalidListings); err != nil {
		t.Fatalf("count invalid listings: %v", err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT count(*)::int FROM domain_events WHERE aggregate_type = 'carpool_listing' AND actor_user_id = $1 AND aggregate_id <> $2`, userID, listing.ID).Scan(&invalidEvents); err != nil {
		t.Fatalf("count invalid listing events: %v", err)
	}
	if invalidListings != 0 || invalidEvents != 0 {
		t.Fatalf("failed scoped mutation leaked rows: listings=%d events=%d", invalidListings, invalidEvents)
	}

	rollbackInput := validCarpoolListingAuditInput(ownerContact.ID, "builder-failure-request")
	_, _, _, appErr = service.CreateListingWithIdempotency(
		ctx, owner, "carpool-create-builder-failure", "builder-failure-key", "builder-failure-hash", rollbackInput,
		func(carpool.Listing) (idempotency.Completion, *domain.AppError) {
			return idempotency.Completion{}, domain.NewError(500, domain.CodeInternalError, "Encoding failed", "测试响应编码失败。")
		},
	)
	if appErr == nil || appErr.Code != domain.CodeInternalError {
		t.Fatalf("expected completion builder failure, got %#v", appErr)
	}
	if err := store.pool.QueryRow(ctx, `SELECT count(*)::int FROM carpool_listings WHERE owner_user_id = $1 AND id <> $2`, userID, listing.ID).Scan(&invalidListings); err != nil {
		t.Fatalf("count builder-failure listings: %v", err)
	}
	if err := store.pool.QueryRow(ctx, `SELECT count(*)::int FROM domain_events WHERE aggregate_type = 'carpool_listing' AND actor_user_id = $1 AND aggregate_id <> $2`, userID, listing.ID).Scan(&invalidEvents); err != nil {
		t.Fatalf("count builder-failure events: %v", err)
	}
	if invalidListings != 0 || invalidEvents != 0 {
		t.Fatalf("completion builder failure leaked rows: listings=%d events=%d", invalidListings, invalidEvents)
	}
}

func validCarpoolListingAuditInput(contactMethodID, requestID string) carpool.CreateListingInput {
	followsOfficialReset := true
	supportsMainlandDirect := true
	return carpool.CreateListingInput{
		ProductPlanID: "00000000-0000-0000-0000-000000000401", OwnerContactMethodID: contactMethodID,
		CycleTerm: carpool.CycleTermInput{BillingPeriod: "monthly", NoticeDays: 7, ExitPolicy: "提前确认退出。", UsageRules: "仅限本人使用。"},
		Title:     "Claude Pro 拼车", Summary: "社区拼车测试。", AccessArrangement: "席位安排站外确认。",
		DistributionMethod: carpool.ListingDistributionMethodSub2API, DistributionMethodNote: "Sub2API 托管。",
		ProvidesAdminAccount: true, RegionCode: "us", RegionName: "美国区", PriceMonthlyCNY: "20.00",
		ServiceMultiplier: "1.0000", DailyQuotaAmount: "5.000000", WeeklyQuotaAmount: "20.000000",
		FollowsOfficialQuotaReset: &followsOfficialReset, VPSRegion: "香港",
		SupportsMainlandChinaDirectConnection: &supportsMainlandDirect, OpeningChannelCode: carpool.ListingOpeningChannelWeb,
		PaymentMethodCode: carpool.ListingPaymentMethodUCard, BuyerSeatCapacity: 2, RequestID: requestID,
	}
}

func validCarpoolListingUpdateAuditInput(listing carpool.Listing, contactMethodID, requestID string) carpool.UpdateListingInput {
	input := validCarpoolListingAuditInput(contactMethodID, requestID)
	return carpool.UpdateListingInput{
		ListingID: listing.ID, ProductPlanID: input.ProductPlanID, OwnerContactMethodID: input.OwnerContactMethodID,
		CycleTerm: input.CycleTerm, Title: input.Title + "（更新）", Summary: input.Summary, AccessArrangement: input.AccessArrangement,
		DistributionMethod: input.DistributionMethod, DistributionMethodNote: input.DistributionMethodNote,
		ProvidesAdminAccount: input.ProvidesAdminAccount, RegionCode: input.RegionCode, RegionName: input.RegionName,
		SourceURL: input.SourceURL, PriceMonthlyCNY: input.PriceMonthlyCNY, ServiceMultiplier: input.ServiceMultiplier,
		DailyQuotaAmount: input.DailyQuotaAmount, WeeklyQuotaAmount: input.WeeklyQuotaAmount,
		FollowsOfficialQuotaReset: input.FollowsOfficialQuotaReset, VPSRegion: input.VPSRegion,
		SupportsMainlandChinaDirectConnection: input.SupportsMainlandChinaDirectConnection,
		OpeningChannelCode:                    input.OpeningChannelCode, CustomOpeningChannel: input.CustomOpeningChannel,
		PaymentMethodCode: input.PaymentMethodCode, CustomPaymentMethod: input.CustomPaymentMethod,
		BuyerSeatCapacity: input.BuyerSeatCapacity, OfflineOccupiedSeats: input.OfflineOccupiedSeats,
		RiskAcknowledgement: input.RiskAcknowledgement, ExpectedVersion: listing.Version, RequestID: requestID,
	}
}
