package postgres

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/report"
	"c2c-market/backend/internal/module/reputation"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestReputationPostgresAggregationAndExclusionAudit(t *testing.T) {
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

	var databaseName string
	if err := pool.QueryRow(ctx, "select current_database()").Scan(&databaseName); err != nil {
		t.Fatalf("read test database name: %v", err)
	}
	if !strings.Contains(databaseName, "_reputation_test_") {
		t.Fatalf("refusing to run reputation integration test against non-dedicated database %q", databaseName)
	}

	now := time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC)
	store := &Store{pool: pool}
	userID := uuid.NewString()
	facts, appErr := store.AggregateFacts(ctx, []string{userID}, now)
	if appErr != nil {
		t.Fatalf("aggregate empty reputation facts: %v", appErr)
	}
	value, ok := facts[userID]
	if !ok {
		t.Fatalf("expected requested user zero row, got %#v", facts)
	}
	if value.Buyer.Carpool.CompletedCount != 0 || value.Seller.API.UnresolvedDisputeCount != 0 {
		t.Fatalf("expected real zero facts for empty database, got %#v", value)
	}

	adminID := uuid.NewString()
	if _, err := pool.Exec(ctx, `
		INSERT INTO users (id, username, display_name, account_status, created_at, updated_at)
		VALUES ($1, $2, $2, 'active', $3, $3)
	`, adminID, "reputation-admin-"+adminID[:8], now); err != nil {
		t.Fatalf("insert admin user: %v", err)
	}
	transactionID := uuid.NewString()
	var exclusionID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO reputation_transaction_exclusions (
		  transaction_type, transaction_id, excluded_at, excluded_by_admin_id,
		  reason_code, reason, created_at, updated_at
		)
		VALUES ('api_order', $1, $2, $3, 'integration_test', '集成测试排除', $2, $2)
		RETURNING id::text
	`, transactionID, now.Add(-time.Hour), adminID).Scan(&exclusionID); err != nil {
		t.Fatalf("insert exclusion: %v", err)
	}
	var eventID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO reputation_transaction_exclusion_events (
		  exclusion_id, transaction_type, transaction_id, action,
		  actor_admin_id, reason_code, reason, created_at
		)
		VALUES ($1, 'api_order', $2, 'excluded', $3, 'integration_test', '集成测试排除', $4)
		RETURNING id::text
	`, exclusionID, transactionID, adminID, now.Add(-time.Hour)).Scan(&eventID); err != nil {
		t.Fatalf("insert exclusion event: %v", err)
	}

	restored, appErr := store.RestoreTransaction(ctx, reputation.ExclusionMutation{
		TransactionType: reputation.TransactionAPIOrder,
		TransactionID:   transactionID,
		AdminUserID:     adminID,
		ReasonCode:      "integration_restore",
		Reason:          "集成测试恢复",
	}, now)
	if appErr != nil {
		t.Fatalf("restore exclusion: %v", appErr)
	}
	if restored.RestoredAt == nil || restored.RestoredByAdminID != adminID {
		t.Fatalf("unexpected restored exclusion: %#v", restored)
	}
	var eventCount int
	if err := pool.QueryRow(ctx, `
		SELECT count(*)
		FROM reputation_transaction_exclusion_events
		WHERE exclusion_id = $1
	`, exclusionID).Scan(&eventCount); err != nil {
		t.Fatalf("count exclusion events: %v", err)
	}
	if eventCount != 2 {
		t.Fatalf("expected excluded and restored events, got %d", eventCount)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE reputation_transaction_exclusion_events
		SET reason = '不得改写'
		WHERE id = $1
	`, eventID); err == nil {
		t.Fatal("append-only exclusion event update unexpectedly succeeded")
	}
}

func TestReputationPostgresGovernanceRestrictionAndAppealReversal(t *testing.T) {
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

	var databaseName string
	if err := pool.QueryRow(ctx, "select current_database()").Scan(&databaseName); err != nil {
		t.Fatalf("read test database name: %v", err)
	}
	if !strings.Contains(databaseName, "_reputation_test_") {
		t.Fatalf("refusing to run reputation integration test against non-dedicated database %q", databaseName)
	}

	baseTime := time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC)
	currentTime := baseTime
	store := &Store{pool: pool}
	idempotencyService := idempotency.NewService(store, func() time.Time { return currentTime })
	reputationService := reputation.NewService(store, func() time.Time { return currentTime }, idempotencyService)
	reportService := report.NewService(store, idempotencyService, func() time.Time { return currentTime })

	adminID := uuid.NewString()
	subjectID := uuid.NewString()
	otherParticipantID := uuid.NewString()
	disputeID := uuid.NewString()
	for _, user := range []struct {
		id       string
		username string
	}{
		{id: adminID, username: "governance-admin-" + adminID[:8]},
		{id: subjectID, username: "governance-subject-" + subjectID[:8]},
		{id: otherParticipantID, username: "governance-other-" + otherParticipantID[:8]},
	} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO users (id, username, display_name, account_status, created_at, updated_at, version)
			VALUES ($1, $2, $2, 'active', $3, $3, 1)
		`, user.id, user.username, baseTime); err != nil {
			t.Fatalf("insert governance user %s: %v", user.username, err)
		}
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO dispute_cases (
		  id, target_type, target_id, target_label, primary_user_id, counterparty_user_id, status,
		  public_summary, public_result_code, public_result, admin_reason,
		  opened_by_admin_id, opened_at, created_at, updated_at, version
		)
		VALUES ($1, 'public_user', $2::text, '信誉治理测试用户', $2::uuid, $3::uuid, 'open',
		        '纠纷处理中', 'no_action', '尚未裁定', '',
		        $4, $5, $5, $5, 1)
	`, disputeID, subjectID, otherParticipantID, adminID, baseTime); err != nil {
		t.Fatalf("insert governance dispute: %v", err)
	}

	outcomeInput := reputation.CreateOutcomeInput{
		DisputeCaseID:   disputeID,
		SubjectUserID:   subjectID,
		Responsibility:  reputation.ResponsibilityResponsible,
		Severity:        reputation.SeverityHigh,
		RoleScope:       reputation.RoleAll,
		ReasonCode:      "integration_responsible",
		PublicReason:    "平台已确认该用户应承担主要责任。",
		InternalReason:  "PostgreSQL 信誉治理集成测试。",
		ExpectedVersion: 1,
		RequestID:       "request-governance-outcome",
	}
	_, appErr := reputationService.CreateDisputeOutcomeWithIdempotency(
		ctx,
		reputation.AdminActor{UserID: adminID, IsAdmin: true},
		"POST /integration/disputes/outcome",
		"governance-open-outcome",
		"hash-open-outcome",
		outcomeInput,
		governanceIntegrationCompletion,
	)
	if appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("open dispute must not create outcome, got %#v", appErr)
	}

	if _, err := pool.Exec(ctx, `
		UPDATE dispute_cases
		SET status = 'resolved', resolved_at = $2, updated_at = $2
		WHERE id = $1
	`, disputeID, baseTime); err != nil {
		t.Fatalf("resolve governance dispute: %v", err)
	}
	blockingAppealID := uuid.NewString()
	if _, err := pool.Exec(ctx, `
		INSERT INTO appeals (
		  id, appellant_user_id, dispute_case_id, target_type, target_id,
		  title, statement, status, created_at, updated_at, version
		)
		VALUES ($1::uuid, $2::uuid, $3::uuid, 'public_user', $2::uuid::text, '待处理申诉',
		        '裁定前必须先处理该申诉。', 'submitted', $4, $4, 1)
	`, blockingAppealID, subjectID, disputeID, baseTime); err != nil {
		t.Fatalf("insert outcome-blocking appeal: %v", err)
	}
	if _, appErr := reputationService.CreateDisputeOutcomeWithIdempotency(
		ctx,
		reputation.AdminActor{UserID: adminID, IsAdmin: true},
		"POST /integration/disputes/outcome",
		"governance-blocked-by-submitted-appeal",
		"hash-blocked-by-submitted-appeal",
		outcomeInput,
		governanceIntegrationCompletion,
	); appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("submitted appeal must block a reputation outcome, got %#v", appErr)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM appeals WHERE id = $1`, blockingAppealID); err != nil {
		t.Fatalf("remove outcome-blocking appeal fixture: %v", err)
	}
	if _, appErr := reputationService.CreateDisputeOutcomeWithIdempotency(
		ctx,
		reputation.AdminActor{UserID: adminID, IsAdmin: true},
		"POST /integration/disputes/outcome",
		"governance-resolved-outcome",
		"hash-resolved-outcome",
		outcomeInput,
		governanceIntegrationCompletion,
	); appErr != nil {
		t.Fatalf("create resolved dispute outcome: %v", appErr)
	}

	var outcomeID, outcomeStatus string
	if err := pool.QueryRow(ctx, `
		SELECT id::text, status
		FROM dispute_reputation_outcomes
		WHERE dispute_case_id = $1
	`, disputeID).Scan(&outcomeID, &outcomeStatus); err != nil {
		t.Fatalf("read dispute outcome: %v", err)
	}
	if outcomeStatus != reputation.OutcomeStatusActive {
		t.Fatalf("expected active outcome, got %q", outcomeStatus)
	}

	firstEndsAt := baseTime.Add(time.Hour)
	firstRestriction := createGovernanceRestrictionForTest(
		t,
		ctx,
		reputationService,
		adminID,
		subjectID,
		outcomeID,
		1,
		"buyer-contact",
		reputation.RoleBuyer,
		reputation.ActionContactView,
		&firstEndsAt,
	)
	if appErr := reputationService.CheckActionAllowed(ctx, subjectID, reputation.RoleBuyer, reputation.ActionContactView); appErr == nil || appErr.Code != domain.CodeReputationActionRestricted {
		t.Fatalf("active buyer contact restriction must block, got %#v", appErr)
	}
	if appErr := reputationService.CheckActionAllowed(ctx, subjectID, reputation.RoleSeller, reputation.ActionContactView); appErr != nil {
		t.Fatalf("buyer restriction must not block seller role: %#v", appErr)
	}
	currentTime = firstEndsAt
	if appErr := reputationService.CheckActionAllowed(ctx, subjectID, reputation.RoleBuyer, reputation.ActionContactView); appErr != nil {
		t.Fatalf("expired restriction must restore contact action: %#v", appErr)
	}

	currentTime = baseTime.Add(10 * time.Minute)
	manualRestriction := createGovernanceRestrictionForTest(
		t,
		ctx,
		reputationService,
		adminID,
		subjectID,
		outcomeID,
		2,
		"seller-review",
		reputation.RoleSeller,
		reputation.ActionReviewSubmit,
		nil,
	)
	if appErr := reputationService.CheckActionAllowed(ctx, subjectID, reputation.RoleSeller, reputation.ActionReviewSubmit); appErr == nil || appErr.Code != domain.CodeReputationActionRestricted {
		t.Fatalf("active seller review restriction must block, got %#v", appErr)
	}
	if _, appErr := reputationService.RevokeUserRestrictionWithIdempotency(
		ctx,
		reputation.AdminActor{UserID: adminID, IsAdmin: true},
		"POST /integration/restrictions/revoke",
		"governance-manual-revoke",
		"hash-manual-revoke",
		reputation.RevokeRestrictionInput{
			RestrictionID:   manualRestriction.ID,
			Reason:          "管理员确认该限制可以提前解除。",
			ExpectedVersion: manualRestriction.Version,
			RequestID:       "request-manual-revoke",
		},
		governanceIntegrationCompletion,
	); appErr != nil {
		t.Fatalf("revoke restriction: %v", appErr)
	}
	if appErr := reputationService.CheckActionAllowed(ctx, subjectID, reputation.RoleSeller, reputation.ActionReviewSubmit); appErr != nil {
		t.Fatalf("revoked restriction must restore review action: %#v", appErr)
	}

	appealRestriction := createGovernanceRestrictionForTest(
		t,
		ctx,
		reputationService,
		adminID,
		subjectID,
		outcomeID,
		3,
		"buyer-order",
		reputation.RoleBuyer,
		reputation.ActionAPIOrderCreate,
		nil,
	)
	if appErr := reputationService.CheckActionAllowed(ctx, subjectID, reputation.RoleBuyer, reputation.ActionAPIOrderCreate); appErr == nil {
		t.Fatal("appeal-linked restriction must block before appeal approval")
	}
	mismatchedAppealID := uuid.NewString()
	if _, err := pool.Exec(ctx, `
		INSERT INTO appeals (
		  id, appellant_user_id, dispute_case_id, target_type, target_id,
		  title, statement, status, created_at, updated_at, version
		)
		VALUES ($1, $2, $3, 'public_user', $4::text, '非裁定主体申诉',
		        '该参与者不是信誉裁定主体。', 'submitted', $5, $5, 1)
	`, mismatchedAppealID, otherParticipantID, disputeID, subjectID, currentTime); err != nil {
		t.Fatalf("insert mismatched reputation appeal: %v", err)
	}
	if _, appErr := reportService.AdminAppealActionWithIdempotency(
		ctx,
		auth.User{ID: adminID, IsAdmin: true},
		"POST /integration/appeals/approve",
		"governance-mismatched-appeal-approve",
		"hash-mismatched-appeal-approve",
		report.AdminActionInput{
			ID:              mismatchedAppealID,
			Action:          "approve",
			Reason:          "尝试批准非裁定主体的申诉。",
			ExpectedVersion: 1,
			RequestID:       "request-mismatched-appeal-approve",
		},
		reportGovernanceIntegrationCompletion,
	); appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("mismatched appellant must not reverse outcome, got %#v", appErr)
	}
	var mismatchedAppealStatus string
	var appealRestrictionRevokedAt *time.Time
	if err := pool.QueryRow(ctx, `SELECT status FROM appeals WHERE id = $1`, mismatchedAppealID).Scan(&mismatchedAppealStatus); err != nil {
		t.Fatalf("read mismatched appeal status: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM dispute_reputation_outcomes WHERE id = $1`, outcomeID).Scan(&outcomeStatus); err != nil {
		t.Fatalf("read outcome after mismatched appeal: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT revoked_at FROM user_restrictions WHERE id = $1`, appealRestriction.ID).Scan(&appealRestrictionRevokedAt); err != nil {
		t.Fatalf("read restriction after mismatched appeal: %v", err)
	}
	if mismatchedAppealStatus != report.AppealStatusSubmitted || outcomeStatus != reputation.OutcomeStatusActive || appealRestrictionRevokedAt != nil {
		t.Fatalf("mismatched appeal changed protected state: appeal=%q outcome=%q revokedAt=%v", mismatchedAppealStatus, outcomeStatus, appealRestrictionRevokedAt)
	}

	appealID := uuid.NewString()
	if _, err := pool.Exec(ctx, `
		INSERT INTO appeals (
		  id, appellant_user_id, dispute_case_id, target_type, target_id,
		  title, statement, status, created_at, updated_at, version
		)
			VALUES ($1, $2::uuid, $3, 'public_user', $2::text, '信誉裁定申诉',
		        '请求复核信誉责任与限制。', 'submitted', $4, $4, 1)
	`, appealID, subjectID, disputeID, currentTime); err != nil {
		t.Fatalf("insert reputation appeal: %v", err)
	}
	duplicateAppealTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin duplicate appeal transaction: %v", err)
	}
	_, duplicateAppealErr := createAppealInTx(ctx, duplicateAppealTx, report.CreateAppealInput{
		AppellantUserID: subjectID,
		DisputeID:       disputeID,
		Title:           "重复信誉裁定申诉",
		Statement:       "同一纠纷已有待处理申诉。",
	}, currentTime)
	if rollbackErr := duplicateAppealTx.Rollback(ctx); rollbackErr != nil {
		t.Fatalf("rollback duplicate appeal transaction: %v", rollbackErr)
	}
	if duplicateAppealErr == nil || duplicateAppealErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("postgres duplicate appeal must be rejected, got %#v", duplicateAppealErr)
	}
	currentTime = baseTime.Add(20 * time.Minute)
	if _, appErr := reportService.AdminAppealActionWithIdempotency(
		ctx,
		auth.User{ID: adminID, IsAdmin: true},
		"POST /integration/appeals/approve",
		"governance-appeal-approve",
		"hash-appeal-approve",
		report.AdminActionInput{
			ID:              appealID,
			Action:          "approve",
			Reason:          "复核后批准申诉并反转信誉裁定。",
			ExpectedVersion: 1,
			RequestID:       "request-appeal-approve",
		},
		reportGovernanceIntegrationCompletion,
	); appErr != nil {
		t.Fatalf("approve reputation appeal: %v", appErr)
	}

	var reversalAppealID string
	if err := pool.QueryRow(ctx, `
		SELECT status, reversal_appeal_id::text
		FROM dispute_reputation_outcomes
		WHERE id = $1
	`, outcomeID).Scan(&outcomeStatus, &reversalAppealID); err != nil {
		t.Fatalf("read reversed outcome: %v", err)
	}
	if outcomeStatus != reputation.OutcomeStatusReversed || reversalAppealID != appealID {
		t.Fatalf("appeal did not reverse outcome: status=%q appeal=%q", outcomeStatus, reversalAppealID)
	}
	if appErr := reputationService.CheckActionAllowed(ctx, subjectID, reputation.RoleBuyer, reputation.ActionAPIOrderCreate); appErr != nil {
		t.Fatalf("appeal approval must revoke linked restriction: %#v", appErr)
	}
	postApprovalOutcomeInput := outcomeInput
	postApprovalOutcomeInput.ExpectedVersion = 2
	postApprovalOutcomeInput.RequestID = "request-governance-post-appeal-outcome"
	if _, appErr := reputationService.CreateDisputeOutcomeWithIdempotency(
		ctx,
		reputation.AdminActor{UserID: adminID, IsAdmin: true},
		"POST /integration/disputes/outcome",
		"governance-blocked-by-approved-appeal",
		"hash-blocked-by-approved-appeal",
		postApprovalOutcomeInput,
		governanceIntegrationCompletion,
	); appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("approved appeal must block a later reputation outcome, got %#v", appErr)
	}
	for _, restrictionID := range []string{firstRestriction.ID, appealRestriction.ID} {
		var revokedAt *time.Time
		if err := pool.QueryRow(ctx, `SELECT revoked_at FROM user_restrictions WHERE id = $1`, restrictionID).Scan(&revokedAt); err != nil {
			t.Fatalf("read appeal-revoked restriction %s: %v", restrictionID, err)
		}
		if revokedAt == nil {
			t.Fatalf("appeal approval did not revoke restriction %s", restrictionID)
		}
	}

	evidence, appErr := store.LoadAdminReputationEvidence(ctx, subjectID, currentTime)
	if appErr != nil {
		t.Fatalf("load admin reputation evidence: %v", appErr)
	}
	if len(evidence.Restrictions) != 3 {
		t.Fatalf("expected three restriction audit rows, got %#v", evidence.Restrictions)
	}
	if len(evidence.Outcomes) != 1 ||
		evidence.Outcomes[0].ID != outcomeID ||
		evidence.Outcomes[0].Status != reputation.OutcomeStatusReversed {
		t.Fatalf("unexpected dispute outcome audit: %#v", evidence.Outcomes)
	}
	if len(evidence.Appeals) != 2 {
		t.Fatalf("unexpected reputation appeal audit: %#v", evidence.Appeals)
	}
	appealStatuses := make(map[string]string, len(evidence.Appeals))
	for _, item := range evidence.Appeals {
		appealStatuses[item.ID] = item.Status
	}
	if appealStatuses[appealID] != report.AppealStatusApproved || appealStatuses[mismatchedAppealID] != report.AppealStatusSubmitted {
		t.Fatalf("unexpected reputation appeal statuses: %#v", appealStatuses)
	}
	if len(evidence.SourceAuthorVerifications) != 0 {
		t.Fatalf("governance-only subject must not have source-author audits: %#v", evidence.SourceAuthorVerifications)
	}

	var governanceEventID string
	if err := pool.QueryRow(ctx, `
		SELECT id::text
		FROM reputation_governance_events
		WHERE entity_id = $1
		ORDER BY created_at, id
		LIMIT 1
	`, outcomeID).Scan(&governanceEventID); err != nil {
		t.Fatalf("read governance event: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		UPDATE reputation_governance_events
		SET reason = '不得改写'
		WHERE id = $1
	`, governanceEventID); err == nil {
		t.Fatal("append-only governance event update unexpectedly succeeded")
	}
}

func createGovernanceRestrictionForTest(
	t *testing.T,
	ctx context.Context,
	service *reputation.Service,
	adminID string,
	userID string,
	outcomeID string,
	expectedUserVersion int64,
	key string,
	role string,
	action string,
	endsAt *time.Time,
) reputation.UserRestriction {
	t.Helper()
	var created reputation.UserRestriction
	_, appErr := service.CreateUserRestrictionWithIdempotency(
		ctx,
		reputation.AdminActor{UserID: adminID, IsAdmin: true},
		"POST /integration/users/restrictions",
		"governance-restriction-"+key,
		"hash-restriction-"+key,
		reputation.CreateRestrictionInput{
			UserID:                 userID,
			RestrictionType:        key,
			RoleScope:              role,
			ActionCode:             action,
			ReasonCode:             "integration_" + strings.ReplaceAll(key, "-", "_"),
			PublicReason:           "该动作当前受信誉治理限制。",
			InternalReason:         "PostgreSQL 信誉治理限制集成测试。",
			EndsAt:                 endsAt,
			SourceDisputeOutcomeID: outcomeID,
			ExpectedUserVersion:    expectedUserVersion,
			RequestID:              "request-restriction-" + key,
		},
		func(result reputation.GovernanceMutationResult) (idempotency.Completion, *domain.AppError) {
			if result.Restriction == nil {
				return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Missing restriction", "测试限制响应为空。")
			}
			created = *result.Restriction
			return governanceIntegrationCompletion(result)
		},
	)
	if appErr != nil {
		t.Fatalf("create governance restriction %s: %v", key, appErr)
	}
	return created
}

func governanceIntegrationCompletion(result reputation.GovernanceMutationResult) (idempotency.Completion, *domain.AppError) {
	resourceType := "reputation_governance"
	resourceID := ""
	if result.Outcome != nil {
		resourceType = "dispute_reputation_outcome"
		resourceID = result.Outcome.ID
	}
	if result.Restriction != nil {
		resourceType = "user_restriction"
		resourceID = result.Restriction.ID
	}
	return idempotency.Completion{
		Status:       http.StatusOK,
		ContentType:  "application/json",
		Body:         []byte(`{}`),
		ResourceType: resourceType,
		ResourceID:   resourceID,
	}, nil
}

func reportGovernanceIntegrationCompletion(result report.MutationResult) (idempotency.Completion, *domain.AppError) {
	if result.Appeal == nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Missing appeal", "测试申诉响应为空。")
	}
	return idempotency.Completion{
		Status:       http.StatusOK,
		ContentType:  "application/json",
		Body:         []byte(`{}`),
		ResourceType: "appeal",
		ResourceID:   result.Appeal.ID,
	}, nil
}
