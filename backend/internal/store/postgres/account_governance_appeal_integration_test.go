package postgres

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/report"

	"github.com/google/uuid"
)

func TestPostgresAccountGovernanceAppealLifecycle(t *testing.T) {
	store := connectLifecycleTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 3, 14, 0, 0, 0, time.UTC)
	suffix := strings.ToLower(uuid.NewString()[:8])

	admin, appErr := store.EnsureUser(ctx, "account-appeal-admin-"+suffix, true, now)
	if appErr != nil {
		t.Fatalf("ensure account appeal admin: %v", appErr)
	}
	admin.IsAdmin = true
	admin.Status = auth.AccountStatusActive

	createUser := func(label, status string) auth.User {
		t.Helper()
		user, appErr := store.EnsureUser(ctx, "account-appeal-"+label+"-"+suffix, false, now)
		if appErr != nil {
			t.Fatalf("ensure %s account appeal user: %v", label, appErr)
		}
		if _, err := store.pool.Exec(ctx, `UPDATE users SET account_status = $2 WHERE id = $1`, user.ID, status); err != nil {
			t.Fatalf("set %s account status: %v", label, err)
		}
		user.Status = status
		return user
	}

	suspended := createUser("suspended", auth.AccountStatusSuspended)
	banned := createUser("banned", auth.AccountStatusBanned)
	active := createUser("active", auth.AccountStatusActive)
	serialized := createUser("serialized", auth.AccountStatusSuspended)
	userIDs := []string{admin.ID, suspended.ID, banned.ID, active.ID, serialized.ID}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM moderation_audit_logs WHERE actor_admin_id = $1 OR basis_appeal_id IN (SELECT id FROM appeals WHERE appellant_user_id = ANY($2::uuid[]))`, admin.ID, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM dispute_events WHERE actor_user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM appeals WHERE appellant_user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM idempotency_keys WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM account_appeal_sessions WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM user_permissions WHERE user_id = ANY($1::uuid[])`, userIDs)
		_, _ = store.pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = ANY($1::uuid[])`, userIDs)
	})

	reportService := report.NewService(store, idempotency.NewService(store, func() time.Time { return now }), func() time.Time { return now })
	create := func(userID, key, hash, statement string) (idempotency.Completion, *domain.AppError) {
		return reportService.CreateAccountGovernanceAppealWithIdempotency(
			ctx,
			userID,
			"POST /api/v1/account-appeal/appeals",
			key,
			hash,
			report.CreateAccountGovernanceAppealInput{Statement: statement},
			accountGovernanceAppealCompletion,
		)
	}

	if _, appErr := create(active.ID, "active-ineligible-"+suffix, "active-ineligible-hash", "请复核当前账号治理限制。即使账号状态已经变化，也不得创建申诉。"); appErr == nil || appErr.Code != domain.CodeAccountAppealIneligible {
		t.Fatalf("active account appeal result = %#v", appErr)
	}

	first, appErr := create(suspended.ID, "suspended-create-"+suffix, "suspended-create-hash", "请复核当前账号限制所依据的事实，并重新审查处理记录。")
	if appErr != nil || first.Status != http.StatusCreated {
		t.Fatalf("create suspended account appeal completion=%+v err=%v", first, appErr)
	}
	var created report.Appeal
	if err := json.Unmarshal(first.Body, &created); err != nil {
		t.Fatalf("decode account appeal completion: %v", err)
	}
	if created.AppellantUserID != suspended.ID || created.TargetType != report.TargetAccountGovernance || created.TargetID != suspended.ID || created.ReportID != "" || created.DisputeID != "" || created.Title != "账号治理申诉" || created.Status != report.AppealStatusSubmitted {
		t.Fatalf("unexpected account-governance appeal projection: %+v", created)
	}
	var reportSourceIsNull, disputeSourceIsNull bool
	var durableTargetType, durableTargetID string
	if err := store.pool.QueryRow(ctx, `
		SELECT report_id IS NULL, dispute_case_id IS NULL, target_type, target_id
		FROM appeals
		WHERE id = $1
	`, created.ID).Scan(&reportSourceIsNull, &disputeSourceIsNull, &durableTargetType, &durableTargetID); err != nil {
		t.Fatalf("read durable account-governance appeal source: %v", err)
	}
	if !reportSourceIsNull || !disputeSourceIsNull || durableTargetType != report.TargetAccountGovernance || durableTargetID != suspended.ID {
		t.Fatalf("unexpected durable account-governance source reportNull=%t disputeNull=%t target=%s:%s", reportSourceIsNull, disputeSourceIsNull, durableTargetType, durableTargetID)
	}

	replay, appErr := create(suspended.ID, "suspended-create-"+suffix, "suspended-create-hash", "请复核当前账号限制所依据的事实，并重新审查处理记录。")
	if appErr != nil || normalizedIntegrationJSON(replay.Body) != normalizedIntegrationJSON(first.Body) {
		t.Fatalf("account appeal idempotent replay mismatch first=%s replay=%s err=%v", first.Body, replay.Body, appErr)
	}
	if _, appErr := create(suspended.ID, "suspended-duplicate-"+suffix, "suspended-duplicate-hash", "这是另一份账号治理申诉，不应在首份待处理时创建。"); appErr == nil || appErr.Code != domain.CodeInvalidStateTransition {
		t.Fatalf("duplicate submitted account appeal result = %#v", appErr)
	}

	var submittedCount, submittedEventCount int
	if err := store.pool.QueryRow(ctx, `
		SELECT count(*)::int
		FROM appeals
		WHERE appellant_user_id = $1 AND target_type = 'account_governance' AND status = 'submitted'
	`, suspended.ID).Scan(&submittedCount); err != nil {
		t.Fatalf("count submitted account appeals: %v", err)
	}
	if err := store.pool.QueryRow(ctx, `
		SELECT count(*)::int
		FROM dispute_events
		WHERE entity_type = 'appeal' AND entity_id = $1 AND action = 'submitted' AND actor_user_id = $2
	`, created.ID, suspended.ID).Scan(&submittedEventCount); err != nil {
		t.Fatalf("count submitted account appeal events: %v", err)
	}
	if submittedCount != 1 || submittedEventCount != 1 {
		t.Fatalf("unexpected durable appeal side effects submitted=%d events=%d", submittedCount, submittedEventCount)
	}

	lockTx, err := store.pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin account-governance serialization transaction: %v", err)
	}
	defer rollback(context.Background(), lockTx)
	if _, err := lockTx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('account_governance:' || $1::uuid::text, 0))`, serialized.ID); err != nil {
		t.Fatalf("acquire account-governance user lock: %v", err)
	}
	type createResult struct {
		completion idempotency.Completion
		appErr     *domain.AppError
	}
	resultCh := make(chan createResult, 1)
	go func() {
		completion, appErr := create(serialized.ID, "serialized-create-"+suffix, "serialized-create-hash", "请在并发状态变化后重新核对当前账号是否仍可申诉。")
		resultCh <- createResult{completion: completion, appErr: appErr}
	}()
	select {
	case result := <-resultCh:
		t.Fatalf("account appeal did not serialize on user advisory lock: completion=%+v err=%v", result.completion, result.appErr)
	case <-time.After(150 * time.Millisecond):
	}
	if _, err := lockTx.Exec(ctx, `UPDATE users SET account_status = 'active' WHERE id = $1`, serialized.ID); err != nil {
		t.Fatalf("update serialized account status: %v", err)
	}
	if err := lockTx.Commit(ctx); err != nil {
		t.Fatalf("commit serialized account status: %v", err)
	}
	select {
	case result := <-resultCh:
		if result.appErr == nil || result.appErr.Code != domain.CodeAccountAppealIneligible {
			t.Fatalf("serialized account appeal did not recheck current status: completion=%+v err=%#v", result.completion, result.appErr)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("serialized account appeal did not resume after user lock release")
	}
	var serializedAppeals int
	if err := store.pool.QueryRow(ctx, `SELECT count(*)::int FROM appeals WHERE appellant_user_id = $1`, serialized.ID).Scan(&serializedAppeals); err != nil || serializedAppeals != 0 {
		t.Fatalf("serialized ineligible appeal persisted count=%d err=%v", serializedAppeals, err)
	}

	bannedCompletion, appErr := create(banned.ID, "banned-create-"+suffix, "banned-create-hash", "请复核封禁账号对应的治理事实和处理依据。")
	if appErr != nil {
		t.Fatalf("create banned account appeal: %v", appErr)
	}
	var bannedAppeal report.Appeal
	if err := json.Unmarshal(bannedCompletion.Body, &bannedAppeal); err != nil {
		t.Fatalf("decode banned account appeal: %v", err)
	}

	for _, testCase := range []struct {
		name       string
		appeal     report.Appeal
		userID     string
		wantStatus string
		wantAppeal string
		action     string
		key        string
	}{
		{name: "approve", appeal: created, userID: suspended.ID, wantStatus: auth.AccountStatusSuspended, wantAppeal: report.AppealStatusApproved, action: "approve", key: "approve-" + suffix},
		{name: "reject", appeal: bannedAppeal, userID: banned.ID, wantStatus: auth.AccountStatusBanned, wantAppeal: report.AppealStatusRejected, action: "reject", key: "reject-" + suffix},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			completion, appErr := reportService.AdminAppealActionWithIdempotency(
				ctx,
				admin,
				"POST /api/v1/admin/appeals/{id}/"+testCase.action+":"+testCase.appeal.ID,
				testCase.key,
				testCase.key+"-hash",
				report.AdminActionInput{ID: testCase.appeal.ID, Action: testCase.action, Reason: "账号治理申诉人工复核完成。", ExpectedVersion: testCase.appeal.Version, RequestID: testCase.key + "-request"},
				accountGovernanceAppealAdminCompletion,
			)
			if appErr != nil || completion.Status != http.StatusOK {
				t.Fatalf("%s account appeal completion=%+v err=%v", testCase.action, completion, appErr)
			}
			var accountStatus, appealStatus string
			if err := store.pool.QueryRow(ctx, `
				SELECT user_account.account_status, appeal.status
				FROM users user_account
				JOIN appeals appeal ON appeal.id = $2
				WHERE user_account.id = $1
			`, testCase.userID, testCase.appeal.ID).Scan(&accountStatus, &appealStatus); err != nil {
				t.Fatalf("read durable state after %s: %v", testCase.action, err)
			}
			if accountStatus != testCase.wantStatus {
				t.Fatalf("%s account appeal changed account status to %s, want %s", testCase.action, accountStatus, testCase.wantStatus)
			}
			if appealStatus != testCase.wantAppeal {
				t.Fatalf("%s account appeal status = %s, want %s", testCase.action, appealStatus, testCase.wantAppeal)
			}
		})
	}
}

func accountGovernanceAppealCompletion(item report.Appeal) (idempotency.Completion, *domain.AppError) {
	body, err := json.Marshal(item)
	if err != nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "encode account appeal completion")
	}
	return idempotency.Completion{
		Status: http.StatusCreated, ContentType: "application/json", Body: body,
		ResourceType: "appeal", ResourceID: item.ID,
	}, nil
}

func accountGovernanceAppealAdminCompletion(result report.MutationResult) (idempotency.Completion, *domain.AppError) {
	if result.Appeal == nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "missing account appeal result")
	}
	body, err := json.Marshal(result.Appeal)
	if err != nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "encode account appeal admin completion")
	}
	return idempotency.Completion{
		Status: http.StatusOK, ContentType: "application/json", Body: body,
		ResourceType: "appeal", ResourceID: result.Appeal.ID,
	}, nil
}
