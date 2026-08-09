package reputation

import (
	"context"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

type fakeRepository struct {
	facts             map[string]RawFacts
	aggregateIDs      []string
	excluded          ExclusionMutation
	restored          ExclusionMutation
	activeRestriction *UserRestriction
	checkedUserID     string
	checkedRole       string
	checkedAction     string
	sourceAudit       SourceAuthorVerificationAudit
	sourceUpdate      UpdateSourceAuthorVerificationInput
	sourceReadCount   int
	sourceUpdateCount int
}

func (f *fakeRepository) GetSourceAuthorVerificationAudit(_ context.Context, _, _ string, _ time.Time) (SourceAuthorVerificationAudit, *domain.AppError) {
	f.sourceReadCount++
	return f.sourceAudit, nil
}

func (f *fakeRepository) UpdateSourceAuthorVerification(_ context.Context, input UpdateSourceAuthorVerificationInput, _ time.Time) (SourceAuthorVerificationAudit, *domain.AppError) {
	f.sourceUpdateCount++
	f.sourceUpdate = input
	return f.sourceAudit, nil
}

func (f *fakeRepository) AggregateFacts(_ context.Context, userIDs []string, _ time.Time) (map[string]RawFacts, *domain.AppError) {
	f.aggregateIDs = append([]string(nil), userIDs...)
	return f.facts, nil
}

func (f *fakeRepository) ExcludeTransaction(_ context.Context, input ExclusionMutation, now time.Time) (TransactionExclusion, *domain.AppError) {
	f.excluded = input
	return TransactionExclusion{
		ID:                "exclude-1",
		TransactionType:   input.TransactionType,
		TransactionID:     input.TransactionID,
		ExcludedAt:        now,
		ExcludedByAdminID: input.AdminUserID,
		ReasonCode:        input.ReasonCode,
		Reason:            input.Reason,
	}, nil
}

func (f *fakeRepository) RestoreTransaction(_ context.Context, input ExclusionMutation, now time.Time) (TransactionExclusion, *domain.AppError) {
	f.restored = input
	return TransactionExclusion{
		ID:                "exclude-1",
		TransactionType:   input.TransactionType,
		TransactionID:     input.TransactionID,
		ExcludedAt:        now.Add(-time.Hour),
		ExcludedByAdminID: input.AdminUserID,
		ReasonCode:        input.ReasonCode,
		Reason:            input.Reason,
		RestoredAt:        &now,
		RestoredByAdminID: input.AdminUserID,
	}, nil
}

func (f *fakeRepository) CreateDisputeOutcomeWithIdempotency(context.Context, idempotency.Entry, CreateOutcomeInput, time.Time, GovernanceCompletionBuilder) (GovernanceMutationResult, idempotency.Completion, *domain.AppError) {
	return GovernanceMutationResult{}, idempotency.Completion{}, nil
}

func (f *fakeRepository) CreateUserRestrictionWithIdempotency(context.Context, idempotency.Entry, CreateRestrictionInput, time.Time, GovernanceCompletionBuilder) (GovernanceMutationResult, idempotency.Completion, *domain.AppError) {
	return GovernanceMutationResult{}, idempotency.Completion{}, nil
}

func (f *fakeRepository) RevokeUserRestrictionWithIdempotency(context.Context, idempotency.Entry, RevokeRestrictionInput, time.Time, GovernanceCompletionBuilder) (GovernanceMutationResult, idempotency.Completion, *domain.AppError) {
	return GovernanceMutationResult{}, idempotency.Completion{}, nil
}

func (f *fakeRepository) FindActiveRestriction(_ context.Context, userID, role, action string, _ time.Time) (*UserRestriction, *domain.AppError) {
	f.checkedUserID = userID
	f.checkedRole = role
	f.checkedAction = action
	return f.activeRestriction, nil
}

func TestAggregateFactsDeduplicatesUsersAndBuildsOverallScopes(t *testing.T) {
	t.Parallel()

	sourceAt := time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC)
	repo := &fakeRepository{facts: map[string]RawFacts{
		"user-1": {
			UserID: "user-1",
			Buyer: RoleFacts{
				Carpool: ScopeFacts{CompletedCount: 2, CompletedCountLast90Days: 1, RoleResponsibilityCancellationCount: 1},
				API:     ScopeFacts{CompletedCount: 3, CompletedCountLast90Days: 2, UnknownResponsibilityCancellationCount: 1, SourceDataUpdatedAt: &sourceAt},
			},
		},
	}}
	service := NewService(repo, func() time.Time { return sourceAt })

	result, appErr := service.AggregateFacts(context.Background(), []string{"user-1", " user-1 ", "", "user-2"})
	if appErr != nil {
		t.Fatalf("aggregate facts: %v", appErr)
	}
	if len(repo.aggregateIDs) != 2 || repo.aggregateIDs[0] != "user-1" || repo.aggregateIDs[1] != "user-2" {
		t.Fatalf("unexpected aggregate IDs: %#v", repo.aggregateIDs)
	}
	if result["user-1"].Buyer.Overall.CompletedCount != 5 {
		t.Fatalf("expected buyer overall completed count 5, got %d", result["user-1"].Buyer.Overall.CompletedCount)
	}
	if result["user-1"].Buyer.Overall.CompletedCountLast90Days != 3 {
		t.Fatalf("expected buyer 90-day completed count 3, got %d", result["user-1"].Buyer.Overall.CompletedCountLast90Days)
	}
	if result["user-1"].Buyer.Overall.SourceDataUpdatedAt == nil || !result["user-1"].Buyer.Overall.SourceDataUpdatedAt.Equal(sourceAt) {
		t.Fatal("expected overall source timestamp from API facts")
	}
	if result["user-2"].UserID != "user-2" || result["user-2"].Buyer.Overall.CompletedCount != 0 {
		t.Fatalf("successful repository read must expose a real zero row: %#v", result["user-2"])
	}
}

func TestAggregateFactsWithoutRepositoryReturnsUnknown(t *testing.T) {
	t.Parallel()

	service := NewService(nil, time.Now)
	result, appErr := service.AggregateFacts(context.Background(), []string{"user-1"})
	if appErr != nil {
		t.Fatalf("aggregate facts: %v", appErr)
	}
	if len(result) != 0 {
		t.Fatalf("missing repository must not fabricate zero facts: %#v", result)
	}
}

func TestAPIOrderFaultOutcomeAndSourceRestrictionRequireOverdueRemedy(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 9, 16, 0, 0, 0, time.UTC)
	service := NewService(nil, func() time.Time { return now })
	base := CreateOutcomeInput{
		DisputeCaseID:   "11111111-1111-4111-8111-111111111111",
		SubjectUserID:   "22222222-2222-4222-8222-222222222222",
		Responsibility:  ResponsibilityResponsible,
		Severity:        SeverityLow,
		RoleScope:       RoleSeller,
		ReasonCode:      "confirmed_overdue",
		PublicReason:    "平台确认责任方逾期未履行。",
		InternalReason:  "管理员已核对整改期限和履行记录。",
		AdminUserID:     "admin-1",
		ExpectedVersion: 1,
		APIOrderDispute: true,
	}
	if appErr := validateCreateOutcomeInput(base); appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("API-order fault outcome without overdue remedy must be rejected, got %+v", appErr)
	}

	base.Responsibility = ResponsibilityNotResponsible
	base.Severity = SeverityNone
	base.ReasonCode = "invalid_claim"
	nonFault, appErr := service.createDisputeOutcomeInMemory(base)
	if appErr != nil || nonFault.Outcome == nil {
		t.Fatalf("non-fault API-order outcome should remain available: result=%+v err=%+v", nonFault, appErr)
	}
	_, appErr = service.createUserRestrictionInMemory(CreateRestrictionInput{
		UserID:                 base.SubjectUserID,
		RestrictionType:        "dispute_follow_up",
		RoleScope:              RoleSeller,
		ActionCode:             ActionAPIServicePublish,
		ReasonCode:             "invalid_claim",
		PublicReason:           "测试限制。",
		InternalReason:         "测试非逾期来源不能创建限制。",
		StartsAt:               now,
		SourceDisputeOutcomeID: nonFault.Outcome.ID,
		AdminUserID:            "admin-1",
	})
	if appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("non-overdue API-order outcome must not source a restriction, got %+v", appErr)
	}

	base.DisputeCaseID = "33333333-3333-4333-8333-333333333333"
	base.Responsibility = ResponsibilityResponsible
	base.Severity = SeverityLow
	base.ReasonCode = "confirmed_overdue"
	base.RemedyOverdueFact = true
	if appErr := validateCreateOutcomeInput(base); appErr != nil {
		t.Fatalf("confirmed overdue remedy should allow fault outcome: %+v", appErr)
	}
	overdue, appErr := service.createDisputeOutcomeInMemory(base)
	if appErr != nil || overdue.Outcome == nil {
		t.Fatalf("create overdue API-order outcome: result=%+v err=%+v", overdue, appErr)
	}
	if _, appErr := service.createUserRestrictionInMemory(CreateRestrictionInput{
		UserID:                 base.SubjectUserID,
		RestrictionType:        "dispute_overdue",
		RoleScope:              RoleSeller,
		ActionCode:             ActionAPIServicePublish,
		ReasonCode:             "confirmed_overdue",
		PublicReason:           "整改逾期限制。",
		InternalReason:         "管理员确认整改逾期。",
		StartsAt:               now,
		SourceDisputeOutcomeID: overdue.Outcome.ID,
		AdminUserID:            "admin-1",
	}); appErr != nil {
		t.Fatalf("overdue API-order outcome should source a restriction: %+v", appErr)
	}
}

func TestExclusionMutationsRequireAdminAndValidateInput(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC)
	repo := &fakeRepository{}
	service := NewService(repo, func() time.Time { return now })
	transactionID := "11111111-1111-4111-8111-111111111111"

	_, appErr := service.ExcludeTransaction(context.Background(), AdminActor{UserID: "admin-1"}, ExcludeTransactionInput{
		TransactionType: TransactionAPIOrder,
		TransactionID:   transactionID,
		ReasonCode:      "test_order",
		Reason:          "测试订单",
	})
	if appErr == nil || appErr.Code != domain.CodePermissionDenied {
		t.Fatalf("expected permission error, got %#v", appErr)
	}

	exclusion, appErr := service.ExcludeTransaction(context.Background(), AdminActor{UserID: "admin-1", IsAdmin: true}, ExcludeTransactionInput{
		TransactionType: TransactionAPIOrder,
		TransactionID:   transactionID,
		ReasonCode:      "test_order",
		Reason:          " 测试订单 ",
	})
	if appErr != nil {
		t.Fatalf("exclude transaction: %v", appErr)
	}
	if exclusion.TransactionID != transactionID || repo.excluded.Reason != "测试订单" {
		t.Fatalf("unexpected exclusion mutation: %#v", repo.excluded)
	}

	_, appErr = service.RestoreTransaction(context.Background(), AdminActor{UserID: "admin-1", IsAdmin: true}, RestoreTransactionInput{
		TransactionType: "unknown",
		TransactionID:   transactionID,
		ReasonCode:      "restore",
		Reason:          "恢复",
	})
	if appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("expected validation error, got %#v", appErr)
	}
}

func TestCheckActionAllowedUsesExactRoleAndAction(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC)
	repo := &fakeRepository{
		activeRestriction: &UserRestriction{
			UserID:       "user-1",
			RoleScope:    RoleBuyer,
			ActionCode:   ActionContactView,
			PublicReason: "纠纷处理中暂不可查看联系方式。",
		},
	}
	service := NewService(repo, func() time.Time { return now })

	appErr := service.CheckActionAllowed(context.Background(), "user-1", RoleBuyer, ActionContactView)
	if appErr == nil || appErr.Code != domain.CodeReputationActionRestricted {
		t.Fatalf("expected reputation restriction, got %#v", appErr)
	}
	if appErr.Detail != "纠纷处理中暂不可查看联系方式。" {
		t.Fatalf("expected public restriction reason, got %q", appErr.Detail)
	}
	if repo.checkedUserID != "user-1" || repo.checkedRole != RoleBuyer || repo.checkedAction != ActionContactView {
		t.Fatalf("unexpected restriction lookup: user=%q role=%q action=%q", repo.checkedUserID, repo.checkedRole, repo.checkedAction)
	}
}

func TestCheckActionAllowedRestoresAfterExpiryOrRevocation(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC)
	current := now
	service := NewService(nil, func() time.Time { return current })
	endsAt := now.Add(time.Hour)
	service.restrictions["buyer-contact"] = UserRestriction{
		ID:           "buyer-contact",
		UserID:       "user-1",
		RoleScope:    RoleBuyer,
		ActionCode:   ActionContactView,
		PublicReason: "暂不可查看联系方式。",
		StartsAt:     now.Add(-time.Minute),
		EndsAt:       &endsAt,
	}

	if appErr := service.CheckActionAllowed(context.Background(), "user-1", RoleBuyer, ActionContactView); appErr == nil || appErr.Code != domain.CodeReputationActionRestricted {
		t.Fatalf("expected active restriction, got %#v", appErr)
	}
	if appErr := service.CheckActionAllowed(context.Background(), "user-1", RoleSeller, ActionContactView); appErr != nil {
		t.Fatalf("buyer-only restriction must not block seller role: %#v", appErr)
	}
	if appErr := service.CheckActionAllowed(context.Background(), "user-1", RoleBuyer, ActionReviewSubmit); appErr != nil {
		t.Fatalf("contact-only restriction must not block review action: %#v", appErr)
	}

	current = endsAt
	if appErr := service.CheckActionAllowed(context.Background(), "user-1", RoleBuyer, ActionContactView); appErr != nil {
		t.Fatalf("expired restriction must restore action: %#v", appErr)
	}

	current = now
	revokedAt := now
	item := service.restrictions["buyer-contact"]
	item.RevokedAt = &revokedAt
	service.restrictions[item.ID] = item
	if appErr := service.CheckActionAllowed(context.Background(), "user-1", RoleBuyer, ActionContactView); appErr != nil {
		t.Fatalf("revoked restriction must restore action: %#v", appErr)
	}
}

func TestSourceAuthorAggregatePriorityAndBuyerState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		role   string
		counts SourceAuthorStatusCounts
		state  string
	}{
		{name: "buyer", role: RoleBuyer, counts: SourceAuthorStatusCounts{Total: 2, Verified: 2}, state: SourceAggregateNotApplicable},
		{name: "no sources", role: RoleSeller, state: SourceAggregateNoSources},
		{name: "pending", role: RoleSeller, counts: SourceAuthorStatusCounts{Total: 2, Pending: 1, Expired: 1}, state: SourceAggregatePending},
		{name: "partial", role: RoleSeller, counts: SourceAuthorStatusCounts{Total: 2, Verified: 1, Pending: 1}, state: SourceAggregatePartial},
		{name: "verified", role: RoleSeller, counts: SourceAuthorStatusCounts{Total: 2, Verified: 2}, state: SourceAggregateVerified},
		{name: "mismatch wins", role: RoleSeller, counts: SourceAuthorStatusCounts{Total: 3, Verified: 2, Mismatch: 1}, state: SourceAggregateMismatch},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := SourceAuthorAggregateForCounts(test.role, test.counts)
			if got.State != test.state || got.Counts != test.counts {
				t.Fatalf("unexpected aggregate: %#v", got)
			}
		})
	}
}

func TestSourceAuthorAdminOperationsRequirePermissionAndPreserveVersion(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	resourceID := "11111111-1111-4111-8111-111111111111"
	repo := &fakeRepository{sourceAudit: SourceAuthorVerificationAudit{
		Verification: SourceAuthorVerification{
			ResourceType: SourceResourceCarpool,
			ResourceID:   resourceID,
			Status:       SourceVerificationPending,
			Version:      1,
		},
		Events: []SourceAuthorVerificationEvent{},
	}}
	service := NewService(repo, func() time.Time { return now })

	if _, appErr := service.GetSourceAuthorVerificationAudit(
		context.Background(),
		AdminActor{UserID: "member"},
		SourceResourceCarpool,
		resourceID,
	); appErr == nil || appErr.Code != domain.CodePermissionDenied {
		t.Fatalf("expected read permission error, got %#v", appErr)
	}
	if repo.sourceReadCount != 0 {
		t.Fatal("permission rejection must happen before repository read")
	}

	_, appErr := service.UpdateSourceAuthorVerification(
		context.Background(),
		AdminActor{UserID: " admin-1 ", IsAdmin: true},
		UpdateSourceAuthorVerificationInput{
			ResourceType:         SourceResourceCarpool,
			ResourceID:           resourceID,
			Status:               SourceVerificationVerified,
			ActualExternalUserID: " linux-user-1 ",
			VerificationMethod:   " manual_topic_review ",
			ExpectedVersion:      0,
		},
	)
	if appErr != nil {
		t.Fatalf("update source verification: %v", appErr)
	}
	if repo.sourceUpdateCount != 1 {
		t.Fatalf("expected one repository update, got %d", repo.sourceUpdateCount)
	}
	if repo.sourceUpdate.AdminUserID != "admin-1" ||
		repo.sourceUpdate.ActualExternalUserID != "linux-user-1" ||
		repo.sourceUpdate.VerificationMethod != "manual_topic_review" ||
		repo.sourceUpdate.ExpectedVersion != 0 {
		t.Fatalf("unexpected normalized update: %#v", repo.sourceUpdate)
	}
}

func TestSourceAuthorMismatchRequiresReason(t *testing.T) {
	t.Parallel()

	service := NewService(&fakeRepository{}, func() time.Time {
		return time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	})
	_, appErr := service.UpdateSourceAuthorVerification(
		context.Background(),
		AdminActor{UserID: "admin-1", IsAdmin: true},
		UpdateSourceAuthorVerificationInput{
			ResourceType:         SourceResourceAPIService,
			ResourceID:           "22222222-2222-4222-8222-222222222222",
			Status:               SourceVerificationMismatch,
			ActualExternalUserID: "other-user",
			VerificationMethod:   "manual_topic_review",
		},
	)
	if appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("expected mismatch reason validation error, got %#v", appErr)
	}
}
