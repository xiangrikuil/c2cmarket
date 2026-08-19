package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/evidence"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/report"
	"c2c-market/backend/internal/module/reputation"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type reportTargetResolution struct {
	TargetLabel         string
	CanonicalTargetType string
	CanonicalTargetID   string
	ReportedUserID      string
	ReportedUsername    string
	ReporterRole        string
	RespondentUserID    string
	RespondentUsername  string
	Participants        []reportTargetParticipant
	BusinessStatus      string
	HasOrder            bool
	HasMembership       bool
}

type reportTargetParticipant struct {
	Role     string `json:"role"`
	UserID   string `json:"userId"`
	Username string `json:"username"`
}

func (s *Store) CreateReportWithIdempotency(ctx context.Context, entry idempotency.Entry, input report.CreateReportInput, now time.Time, buildCompletion report.ReportCompletionBuilder) (report.Report, idempotency.Completion, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return report.Report{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return report.Report{}, idempotency.Completion{}, appErr
	}
	item, appErr := createReportInTx(ctx, tx, input, now)
	if appErr != nil {
		return report.Report{}, idempotency.Completion{}, appErr
	}
	if appErr := insertDisputeEvent(ctx, tx, "report", item.ID, "submitted", input.ReporterUserID, "user", "用户提交举报", false, "", now); appErr != nil {
		return report.Report{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(item)
	if appErr != nil {
		return report.Report{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return report.Report{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return report.Report{}, idempotency.Completion{}, internalStoreError()
	}
	return item, completion, nil
}

func (s *Store) ListReportsByUser(ctx context.Context, userID string) ([]report.Report, *domain.AppError) {
	rows, err := s.pool.Query(ctx, reportSelectSQL+` WHERE r.reporter_user_id = $1 ORDER BY r.updated_at DESC`, userID)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return scanReports(rows)
}

func (s *Store) ListAdminReports(ctx context.Context, page domain.PageRequest) (domain.Page[report.Report], *domain.AppError) {
	page = normalizePageRequest(page)
	position, appErr := decodeKeysetCursor(page.Cursor)
	if appErr != nil {
		return domain.Page[report.Report]{}, appErr
	}
	limit := page.Limit + 1
	var rows pgx.Rows
	var err error
	if page.Cursor == "" {
		rows, err = s.pool.Query(ctx, reportSelectSQL+` ORDER BY r.updated_at DESC, r.id DESC LIMIT $1`, limit)
	} else {
		rows, err = s.pool.Query(ctx, reportSelectSQL+` WHERE (r.updated_at, r.id) < ($1, $2::uuid) ORDER BY r.updated_at DESC, r.id DESC LIMIT $3`, position.Time, position.ID, limit)
	}
	if err != nil {
		return domain.Page[report.Report]{}, internalStoreError()
	}
	defer rows.Close()
	items, appErr := scanReports(rows)
	if appErr != nil {
		return domain.Page[report.Report]{}, appErr
	}
	return pageFromItems(items, page, func(item report.Report) (time.Time, string) { return item.UpdatedAt, item.ID }), nil
}

func (s *Store) GetAdminReport(ctx context.Context, id string) (report.Report, *domain.AppError) {
	item, err := scanReport(ctx, s.pool, reportSelectSQL+` WHERE r.id = $1`, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return report.Report{}, reportNotFound()
	}
	if err != nil {
		return report.Report{}, internalStoreError()
	}
	item.Supplements, err = listAdminInfoSupplements(ctx, s.pool, report.InfoRequestEntityReport, id)
	if err != nil {
		return report.Report{}, internalStoreError()
	}
	return item, nil
}

func (s *Store) UpdateReportAdminWithIdempotency(ctx context.Context, entry idempotency.Entry, input report.AdminActionInput, now time.Time, buildCompletion report.AdminCompletionBuilder) (report.MutationResult, idempotency.Completion, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return report.MutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return report.MutationResult{}, idempotency.Completion{}, appErr
	}
	result, appErr := updateReportAdminInTx(ctx, tx, input, now)
	if appErr != nil {
		return report.MutationResult{}, idempotency.Completion{}, appErr
	}
	if input.Action == "request_info" {
		request, appErr := createInfoRequestInTx(ctx, tx, input, result, now)
		if appErr != nil {
			return report.MutationResult{}, idempotency.Completion{}, appErr
		}
		result.Report.OpenInfoRequestID = request.ID
		result.Report.InfoRequestedFromID = request.RequestedFromID
		if appErr := insertInfoRequestOpenedSideEffects(ctx, tx, request, input.RequestID, now); appErr != nil {
			return report.MutationResult{}, idempotency.Completion{}, appErr
		}
	} else if result.Report != nil {
		if appErr := cancelOpenInfoRequests(ctx, tx, report.InfoRequestEntityReport, result.Report.ID, now); appErr != nil {
			return report.MutationResult{}, idempotency.Completion{}, appErr
		}
	}
	if appErr := insertDisputeEvent(ctx, tx, "report", input.ID, input.Action, input.AdminUserID, "admin", input.Reason, input.Action == "open_dispute", input.RequestID, now); appErr != nil {
		return report.MutationResult{}, idempotency.Completion{}, appErr
	}
	if result.Dispute != nil {
		if appErr := insertDisputeEvent(ctx, tx, "dispute", result.Dispute.ID, "opened", input.AdminUserID, "admin", input.Reason, true, input.RequestID, now); appErr != nil {
			return report.MutationResult{}, idempotency.Completion{}, appErr
		}
	}
	completion, appErr := buildCompletion(result)
	if appErr != nil {
		return report.MutationResult{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return report.MutationResult{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return report.MutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	return result, completion, nil
}

func (s *Store) SubmitInfoSupplementWithIdempotency(ctx context.Context, entry idempotency.Entry, input report.SupplementInput, now time.Time, buildCompletion report.SupplementCompletionBuilder) (report.MutationResult, idempotency.Completion, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return report.MutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return report.MutationResult{}, idempotency.Completion{}, appErr
	}
	if input.ActorAudience == auth.SessionAudienceRestrictedBusiness {
		if appErr := lockAccountGovernanceUser(ctx, tx, input.SubmittingUserID); appErr != nil {
			return report.MutationResult{}, idempotency.Completion{}, appErr
		}
	}
	result, request, appErr := submitInfoSupplementInTx(ctx, tx, input, now)
	if appErr != nil {
		return report.MutationResult{}, idempotency.Completion{}, appErr
	}
	if appErr := insertInfoSupplementSideEffects(ctx, tx, request, input.RequestID, now); appErr != nil {
		return report.MutationResult{}, idempotency.Completion{}, appErr
	}
	if len(input.EvidenceAssetIDs) > 0 {
		if result.Dispute == nil || result.Dispute.TargetType != report.TargetAPIOrder {
			return report.MutationResult{}, idempotency.Completion{}, evidenceValidationError("evidenceAssetIds", "unsupported_target", "图片证据只能用于 API 订单纠纷补件。")
		}
		var supplementID string
		if err := tx.QueryRow(ctx, `SELECT id::text FROM moderation_info_supplements WHERE info_request_id = $1`, request.ID).Scan(&supplementID); err != nil {
			return report.MutationResult{}, idempotency.Completion{}, internalStoreError()
		}
		if appErr := bindEvidenceAssetsInTx(ctx, tx, evidence.BindingInput{
			AssetIDs: input.EvidenceAssetIDs, APIOrderID: result.Dispute.APIOrderID, DisputeCaseID: result.Dispute.ID,
			UploaderID: input.SubmittingUserID, Visibility: evidence.VisibilitySubmitterAdmin,
			Usage: evidence.UsageInfoSupplement, SourceType: evidence.SourceInfoSupplement, SourceID: supplementID,
		}, now); appErr != nil {
			return report.MutationResult{}, idempotency.Completion{}, appErr
		}
	}
	completion, appErr := buildCompletion(result)
	if appErr != nil {
		return report.MutationResult{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return report.MutationResult{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return report.MutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	return result, completion, nil
}

func (s *Store) CreateAppealWithIdempotency(ctx context.Context, entry idempotency.Entry, input report.CreateAppealInput, now time.Time, buildCompletion report.AppealCompletionBuilder) (report.Appeal, idempotency.Completion, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return report.Appeal{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return report.Appeal{}, idempotency.Completion{}, appErr
	}
	item, appErr := createAppealInTx(ctx, tx, input, now)
	if appErr != nil {
		return report.Appeal{}, idempotency.Completion{}, appErr
	}
	if appErr := insertDisputeEvent(ctx, tx, "appeal", item.ID, "submitted", input.AppellantUserID, "user", "用户提交申诉", false, "", now); appErr != nil {
		return report.Appeal{}, idempotency.Completion{}, appErr
	}
	if len(input.EvidenceAssetIDs) > 0 {
		if strings.TrimSpace(item.DisputeID) == "" {
			return report.Appeal{}, idempotency.Completion{}, evidenceValidationError("evidenceAssetIds", "unsupported_target", "图片证据只能用于 API 订单纠纷申诉。")
		}
		var apiOrderID string
		if err := tx.QueryRow(ctx, `SELECT COALESCE(api_order_id::text, '') FROM dispute_cases WHERE id = $1`, item.DisputeID).Scan(&apiOrderID); err != nil {
			return report.Appeal{}, idempotency.Completion{}, internalStoreError()
		}
		if apiOrderID == "" {
			return report.Appeal{}, idempotency.Completion{}, evidenceValidationError("evidenceAssetIds", "unsupported_target", "图片证据只能用于 API 订单纠纷申诉。")
		}
		if appErr := bindEvidenceAssetsInTx(ctx, tx, evidence.BindingInput{
			AssetIDs: input.EvidenceAssetIDs, APIOrderID: apiOrderID, DisputeCaseID: item.DisputeID,
			UploaderID: input.AppellantUserID, Visibility: evidence.VisibilityAppellantAdmin,
			Usage: evidence.UsageAppeal, SourceType: evidence.SourceAppeal, SourceID: item.ID,
		}, now); appErr != nil {
			return report.Appeal{}, idempotency.Completion{}, appErr
		}
	}
	completion, appErr := buildCompletion(item)
	if appErr != nil {
		return report.Appeal{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return report.Appeal{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return report.Appeal{}, idempotency.Completion{}, internalStoreError()
	}
	return item, completion, nil
}

func (s *Store) CreateAccountGovernanceAppealWithIdempotency(ctx context.Context, entry idempotency.Entry, input report.CreateAccountGovernanceAppealInput, now time.Time, buildCompletion report.AppealCompletionBuilder) (report.Appeal, idempotency.Completion, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return report.Appeal{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return report.Appeal{}, idempotency.Completion{}, appErr
	}
	item, appErr := createAccountGovernanceAppealInTx(ctx, tx, input, now)
	if appErr != nil {
		return report.Appeal{}, idempotency.Completion{}, appErr
	}
	if appErr := insertDisputeEvent(ctx, tx, "appeal", item.ID, "submitted", input.AppellantUserID, "user", "用户提交账号治理申诉", false, "", now); appErr != nil {
		return report.Appeal{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(item)
	if appErr != nil {
		return report.Appeal{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return report.Appeal{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return report.Appeal{}, idempotency.Completion{}, internalStoreError()
	}
	return item, completion, nil
}

func (s *Store) ListAppealsByUser(ctx context.Context, userID string) ([]report.Appeal, *domain.AppError) {
	rows, err := s.pool.Query(ctx, appealSelectSQL+` WHERE a.appellant_user_id = $1 ORDER BY a.updated_at DESC`, userID)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	items, appErr := scanAppeals(rows)
	if appErr != nil {
		return nil, appErr
	}
	for index := range items {
		items[index].Evidence, err = listAppealEvidenceReferences(ctx, s.pool, items[index].ID)
		if err != nil {
			return nil, internalStoreError()
		}
	}
	return items, nil
}

func (s *Store) ListAdminAppeals(ctx context.Context) ([]report.Appeal, *domain.AppError) {
	rows, err := s.pool.Query(ctx, appealSelectSQL+` ORDER BY a.updated_at DESC`)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	items, appErr := scanAppeals(rows)
	if appErr != nil {
		return nil, appErr
	}
	for index := range items {
		items[index].Evidence, err = listAppealEvidenceReferences(ctx, s.pool, items[index].ID)
		if err != nil {
			return nil, internalStoreError()
		}
	}
	return items, nil
}

func (s *Store) GetAdminAppeal(ctx context.Context, id string) (report.Appeal, *domain.AppError) {
	item, err := scanAppeal(ctx, s.pool, appealSelectSQL+` WHERE a.id = $1`, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return report.Appeal{}, appealNotFound()
	}
	if err != nil {
		return report.Appeal{}, internalStoreError()
	}
	item.Evidence, err = listAppealEvidenceReferences(ctx, s.pool, item.ID)
	if err != nil {
		return report.Appeal{}, internalStoreError()
	}
	return item, nil
}

func (s *Store) UpdateAppealAdminWithIdempotency(ctx context.Context, entry idempotency.Entry, input report.AdminActionInput, now time.Time, buildCompletion report.AdminCompletionBuilder) (report.MutationResult, idempotency.Completion, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return report.MutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return report.MutationResult{}, idempotency.Completion{}, appErr
	}
	result, appErr := updateAppealAdminInTx(ctx, tx, input, now)
	if appErr != nil {
		return report.MutationResult{}, idempotency.Completion{}, appErr
	}
	if appErr := insertDisputeEvent(ctx, tx, "appeal", input.ID, input.Action, input.AdminUserID, "admin", input.Reason, false, input.RequestID, now); appErr != nil {
		return report.MutationResult{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(result)
	if appErr != nil {
		return report.MutationResult{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return report.MutationResult{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return report.MutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	return result, completion, nil
}

func (s *Store) ListAdminDisputes(ctx context.Context) ([]report.DisputeCase, *domain.AppError) {
	rows, err := s.pool.Query(ctx, disputeSelectSQL+`
		WHERE NOT (
			d.active = true
			AND d.status IN ('pending_seller_response', 'pending_applicant_decision', 'voluntary_fulfillment')
		)
		ORDER BY d.updated_at DESC`)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return scanDisputes(rows)
}

func (s *Store) ListDisputesByUser(ctx context.Context, userID string) ([]report.DisputeCase, *domain.AppError) {
	rows, err := s.pool.Query(ctx, disputeSelectSQL+`
		WHERE d.primary_user_id = $1
		   OR d.counterparty_user_id = $1
		   OR d.subject_user_id = $1
		ORDER BY d.updated_at DESC`, userID)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return scanDisputes(rows)
}

func (s *Store) ListDisputesForActor(ctx context.Context, actor auth.BusinessActor) ([]report.DisputeCase, *domain.AppError) {
	if actor.Audience == auth.SessionAudienceNormal {
		return s.ListDisputesByUser(ctx, actor.UserID)
	}
	where, args, ok := restrictedDisputeWhere(actor, "")
	if !ok {
		return nil, disputeNotFound()
	}
	rows, err := s.pool.Query(ctx, disputeSelectSQL+where+` ORDER BY d.updated_at DESC`, args...)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return scanDisputes(rows)
}

func (s *Store) GetAdminDispute(ctx context.Context, id string) (report.DisputeCase, *domain.AppError) {
	item, err := scanDispute(ctx, s.pool, disputeSelectSQL+` WHERE d.id = $1`, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return report.DisputeCase{}, disputeNotFound()
	}
	if err != nil {
		return report.DisputeCase{}, internalStoreError()
	}
	if appErr := loadAPIOrderDisputeNegotiation(ctx, s.pool, &item); appErr != nil {
		return report.DisputeCase{}, appErr
	}
	item.Supplements, err = listAdminInfoSupplements(ctx, s.pool, report.InfoRequestEntityDispute, id)
	if err != nil {
		return report.DisputeCase{}, internalStoreError()
	}
	return item, nil
}

func (s *Store) GetDisputeForParticipant(ctx context.Context, id, userID string) (report.DisputeCase, *domain.AppError) {
	item, err := scanDispute(ctx, s.pool, disputeSelectSQL+`
		WHERE d.id = $1
		  AND (d.primary_user_id = $2 OR d.counterparty_user_id = $2 OR d.subject_user_id = $2)
	`, id, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return report.DisputeCase{}, disputeNotFound()
	}
	if err != nil {
		return report.DisputeCase{}, internalStoreError()
	}
	if appErr := loadAPIOrderDisputeNegotiation(ctx, s.pool, &item); appErr != nil {
		return report.DisputeCase{}, appErr
	}
	return item, nil
}

func (s *Store) GetDisputeForActor(ctx context.Context, actor auth.BusinessActor, id string) (report.DisputeCase, *domain.AppError) {
	if actor.Audience == auth.SessionAudienceNormal {
		return s.GetDisputeForParticipant(ctx, id, actor.UserID)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return report.DisputeCase{}, internalStoreError()
	}
	defer rollback(ctx, tx)
	if appErr := lockAccountGovernanceUser(ctx, tx, actor.UserID); appErr != nil {
		return report.DisputeCase{}, appErr
	}
	item, err := scanDispute(ctx, tx, disputeSelectSQL+` WHERE d.id = $1 FOR UPDATE OF d`, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return report.DisputeCase{}, disputeNotFound()
	}
	if err != nil {
		return report.DisputeCase{}, internalStoreError()
	}
	if appErr := authorizeRestrictedDisputeInTx(ctx, tx, actor, item); appErr != nil {
		return report.DisputeCase{}, appErr
	}
	if appErr := loadAPIOrderDisputeNegotiation(ctx, tx, &item); appErr != nil {
		return report.DisputeCase{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return report.DisputeCase{}, internalStoreError()
	}
	return item, nil
}

func restrictedDisputeWhere(actor auth.BusinessActor, disputeID string) (string, []any, bool) {
	if actor.Audience != auth.SessionAudienceRestrictedBusiness || actor.UserID == "" || actor.GovernanceActionID == "" || actor.GovernanceVersion < 1 || actor.RestrictionEffectiveAt.IsZero() {
		return "", nil, false
	}
	where := ` WHERE (d.primary_user_id = $1 OR d.counterparty_user_id = $1 OR d.subject_user_id = $1)
		AND d.created_at <= $4
		AND EXISTS (
			SELECT 1
			FROM account_governance_resource_dispositions disposition
			JOIN account_governance_disposition_actions link ON link.disposition_id = disposition.id
			JOIN account_governance_actions action ON action.id = link.governance_action_id
			JOIN users user_account ON user_account.id = action.target_user_id
			WHERE disposition.resource_type = d.target_type
			  AND disposition.resource_id::text = d.target_id
			  AND disposition.result = 'preserved'
			  AND link.governance_action_id = $2
			  AND action.target_user_id = $1
			  AND action.governance_version = $3
			  AND action.effective_at = $4
			  AND action.status = 'effective'
			  AND user_account.account_status IN ('suspended', 'banned')
			  AND user_account.security_locked_at IS NULL
			  AND user_account.current_governance_action_id = action.id
			  AND user_account.governance_version = action.governance_version
		)`
	args := []any{actor.UserID, actor.GovernanceActionID, actor.GovernanceVersion, actor.RestrictionEffectiveAt}
	if strings.TrimSpace(disputeID) != "" {
		where += ` AND d.id = $5`
		args = append(args, disputeID)
	}
	return where, args, true
}

func authorizeRestrictedDisputeInTx(ctx context.Context, tx pgx.Tx, actor auth.BusinessActor, item report.DisputeCase) *domain.AppError {
	if !isStoredDisputeParticipant(item, actor.UserID) || item.CreatedAt.After(actor.RestrictionEffectiveAt) {
		return disputeNotFound()
	}
	var authorized bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM users user_account
			JOIN account_governance_actions action ON action.id = user_account.current_governance_action_id
			JOIN account_governance_disposition_actions link ON link.governance_action_id = action.id
			JOIN account_governance_resource_dispositions disposition ON disposition.id = link.disposition_id
			WHERE user_account.id = $1
			  AND user_account.account_status IN ('suspended', 'banned')
			  AND user_account.security_locked_at IS NULL
			  AND user_account.current_governance_action_id = $2
			  AND user_account.governance_version = $3
			  AND action.status = 'effective'
			  AND action.effective_at = $4
			  AND disposition.resource_type = $5
			  AND disposition.resource_id = $6
			  AND disposition.result = 'preserved'
		)
	`, actor.UserID, actor.GovernanceActionID, actor.GovernanceVersion, actor.RestrictionEffectiveAt, item.TargetType, item.TargetID).Scan(&authorized); err != nil {
		return internalStoreError()
	}
	if !authorized {
		return disputeNotFound()
	}
	return nil
}

func (s *Store) UpdateDisputeParticipantWithIdempotency(ctx context.Context, entry idempotency.Entry, input report.DisputeParticipantActionInput, now time.Time, buildCompletion report.DisputeParticipantCompletionBuilder) (report.DisputeCase, idempotency.Completion, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return report.DisputeCase{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return report.DisputeCase{}, idempotency.Completion{}, appErr
	}
	if input.ActorAudience == auth.SessionAudienceRestrictedBusiness {
		if appErr := lockAccountGovernanceUser(ctx, tx, input.ActorUserID); appErr != nil {
			return report.DisputeCase{}, idempotency.Completion{}, appErr
		}
	}
	item, err := scanDispute(ctx, tx, disputeSelectSQL+`
		WHERE d.id = $1
		  AND (d.primary_user_id = $2 OR d.counterparty_user_id = $2 OR d.subject_user_id = $2)
		FOR UPDATE OF d
	`, input.DisputeID, input.ActorUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return report.DisputeCase{}, idempotency.Completion{}, disputeNotFound()
	}
	if err != nil {
		return report.DisputeCase{}, idempotency.Completion{}, internalStoreError()
	}
	if input.ActorAudience == auth.SessionAudienceRestrictedBusiness {
		actor := auth.BusinessActor{UserID: input.ActorUserID, Audience: input.ActorAudience, GovernanceActionID: input.GovernanceActionID, GovernanceVersion: input.GovernanceVersion, RestrictionEffectiveAt: input.RestrictionEffectiveAt}
		if appErr := authorizeRestrictedDisputeInTx(ctx, tx, actor, item); appErr != nil {
			return report.DisputeCase{}, idempotency.Completion{}, appErr
		}
	} else if input.ActorAudience != "" && input.ActorAudience != auth.SessionAudienceNormal {
		return report.DisputeCase{}, idempotency.Completion{}, disputeNotFound()
	}
	if item.TargetType != report.TargetAPIOrder {
		return report.DisputeCase{}, idempotency.Completion{}, disputeNotFound()
	}
	order, err := s.getAPIOrder(ctx, tx, item.TargetID, true, false)
	if errors.Is(err, pgx.ErrNoRows) {
		return report.DisputeCase{}, idempotency.Completion{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "纠纷关联的 API 订单状态不一致。")
	}
	if err != nil {
		return report.DisputeCase{}, idempotency.Completion{}, internalStoreError()
	}
	if order.DisputeCaseID != item.ID {
		return report.DisputeCase{}, idempotency.Completion{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "纠纷关联的 API 订单状态不一致。")
	}

	if appErr := s.applyDisputeParticipantActionInTx(ctx, tx, &item, &order, input, now); appErr != nil {
		return report.DisputeCase{}, idempotency.Completion{}, appErr
	}
	if appErr := loadAPIOrderDisputeNegotiation(ctx, tx, &item); appErr != nil {
		return report.DisputeCase{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(item)
	if appErr != nil {
		return report.DisputeCase{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return report.DisputeCase{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return report.DisputeCase{}, idempotency.Completion{}, internalStoreError()
	}
	return item, completion, nil
}

func (s *Store) applyDisputeParticipantActionInTx(ctx context.Context, tx pgx.Tx, item *report.DisputeCase, order *apiorder.Order, input report.DisputeParticipantActionInput, now time.Time) *domain.AppError {
	if item == nil || order == nil {
		return internalStoreError()
	}
	switch input.Action {
	case report.DisputeActionSellerDecision:
		if !item.Active || item.Status != report.DisputeStatusPendingSellerResponse || item.CounterpartyUserID != input.ActorUserID || item.SellerDecision != "" || item.DueAt == nil {
			return participantDisputeInvalidState("当前售后申请不能再由卖家处理。")
		}
		responseDueAt := *item.DueAt
		responseLate := !now.Before(responseDueAt)
		if input.Decision == report.SellerDecisionRejected {
			applicantDueAt := now.Add(report.DisputeApplicantDecisionWindow)
			if err := tx.QueryRow(ctx, `
				UPDATE dispute_cases
				SET status = 'pending_applicant_decision', seller_decision = $2,
				    seller_decision_reason = $3, seller_decided_by_user_id = $4,
				    seller_decided_at = $5, seller_response_late = $6,
				    applicant_decision_due_at = $7, next_actor = 'applicant', due_at = $7,
				    public_result = '卖家已拒绝申请，等待买家决定是否申请平台介入',
				    updated_at = $5, version = version + 1
				WHERE id = $1 AND status = 'pending_seller_response' AND seller_decision = ''
				RETURNING status, seller_decision, seller_decision_reason,
				          seller_decided_by_user_id::text, seller_decided_at, seller_response_late,
				          applicant_decision_due_at, next_actor, due_at, public_result, updated_at, version
			`, item.ID, input.Decision, strings.TrimSpace(input.Reason), input.ActorUserID, now, responseLate, applicantDueAt).Scan(
				&item.Status, &item.SellerDecision, &item.SellerDecisionReason,
				&item.SellerDecidedByUserID, &item.SellerDecidedAt, &item.SellerResponseLate,
				&item.ApplicantDecisionDueAt, &item.NextActor, &item.DueAt, &item.PublicResult, &item.UpdatedAt, &item.Version,
			); err != nil {
				return internalStoreError()
			}
			if appErr := setLockedAPIOrderDisputeProjectionInTx(ctx, tx, order, apiorder.DisputeStatusPendingApplicantDecision, input.ActorUserID, apiorder.EventDisputeOpened, "卖家已拒绝售后申请", input.RequestID, now); appErr != nil {
				return appErr
			}
		} else {
			remedyID := uuid.NewString()
			remedyDueAt := now.Add(report.VoluntaryRemedyFulfillmentWindow)
			if _, err := tx.Exec(ctx, `
				INSERT INTO api_order_dispute_remedies (
					id, dispute_case_id, action, amount_cny, currency,
					responsible_user_id, beneficiary_user_id, instructions, status, due_at,
					created_by_admin_id, created_request_id, source, created_at, updated_at, version
				)
				VALUES ($1, $2, $3, $4, 'CNY', $5, $6, $7, 'pending', $8, NULL, $9, 'seller_acceptance', $10, $10, 1)
			`, remedyID, item.ID, item.RequestedResolution, nullNumeric(item.RequestedAmountCNY), item.CounterpartyUserID,
				item.PrimaryUserID, strings.TrimSpace(input.Reason), remedyDueAt, strings.TrimSpace(input.RequestID), now); err != nil {
				return internalStoreError()
			}
			if err := tx.QueryRow(ctx, `
				UPDATE dispute_cases
				SET status = 'voluntary_fulfillment', seller_decision = $2,
				    seller_decision_reason = $3, seller_decided_by_user_id = $4,
				    seller_decided_at = $5, seller_response_late = $6,
				    next_actor = 'responsible_party', due_at = $7,
				    public_result = '卖家已同意申请，等待卖家履行',
				    updated_at = $5, version = version + 1
				WHERE id = $1 AND status = 'pending_seller_response' AND seller_decision = ''
				RETURNING status, seller_decision, seller_decision_reason,
				          seller_decided_by_user_id::text, seller_decided_at, seller_response_late,
				          next_actor, due_at, public_result, updated_at, version
			`, item.ID, input.Decision, strings.TrimSpace(input.Reason), input.ActorUserID, now, responseLate, remedyDueAt).Scan(
				&item.Status, &item.SellerDecision, &item.SellerDecisionReason,
				&item.SellerDecidedByUserID, &item.SellerDecidedAt, &item.SellerResponseLate,
				&item.NextActor, &item.DueAt, &item.PublicResult, &item.UpdatedAt, &item.Version,
			); err != nil {
				return internalStoreError()
			}
			order.ActiveRemedyAction = item.RequestedResolution
			if appErr := setLockedAPIOrderDisputeProjectionInTx(ctx, tx, order, apiorder.DisputeStatusAwaitingFulfillment, input.ActorUserID, apiorder.EventDisputeRemedyAwaiting, "卖家已同意售后申请，等待履行", input.RequestID, now); appErr != nil {
				return appErr
			}
		}
		if appErr := bindEvidenceAssetsInTx(ctx, tx, evidence.BindingInput{
			AssetIDs: input.EvidenceAssetIDs, APIOrderID: order.ID, DisputeCaseID: item.ID,
			UploaderID: input.ActorUserID, Visibility: evidence.VisibilityParticipantsAdmin,
			Usage: evidence.UsageFormalResponse, SourceType: evidence.SourceDisputeCase, SourceID: item.ID,
		}, now); appErr != nil {
			return appErr
		}
		if appErr := insertDisputeEvent(ctx, tx, "dispute", item.ID, "seller_"+input.Decision, input.ActorUserID, "user", input.Reason, true, input.RequestID, now); appErr != nil {
			return appErr
		}
		title := "卖家已处理售后申请"
		message := item.PublicResult
		return insertDisputeNotifications(ctx, tx, item.ID, "dispute.seller_"+input.Decision, title, message, item.ID+":seller_"+input.Decision, now, item.PrimaryUserID)

	case report.DisputeActionRequestPlatformIntervention:
		if !item.Active || item.PrimaryUserID != input.ActorUserID {
			return participantDisputeInvalidState("当前售后申请不能申请平台介入。")
		}
		allowed := false
		switch item.Status {
		case report.DisputeStatusPendingSellerResponse:
			allowed = item.DueAt != nil && !now.Before(*item.DueAt)
		case report.DisputeStatusPendingApplicantDecision:
			allowed = item.ApplicantDecisionDueAt != nil && now.Before(*item.ApplicantDecisionDueAt)
		case report.DisputeStatusVoluntaryFulfillment:
			remedy, appErr := lockActiveDisputeRemedyInTx(ctx, tx, item.ID)
			if appErr != nil {
				return appErr
			}
			allowed = remedy.Status == report.RemedyStatusClaimedFulfilled || !now.Before(remedy.DueAt)
			if allowed {
				targetStatus := report.RemedyStatusCancelled
				if remedy.Status == report.RemedyStatusClaimedFulfilled {
					targetStatus = report.RemedyStatusContested
				}
				if _, err := tx.Exec(ctx, `
					UPDATE api_order_dispute_remedies
					SET status = $2, response_note = $3,
					    contested_at = CASE WHEN $2 = 'contested' THEN $4 ELSE contested_at END,
					    response_request_id = $5, updated_at = $4, version = version + 1
					WHERE id = $1
				`, remedy.ID, targetStatus, strings.TrimSpace(input.Reason), now, strings.TrimSpace(input.RequestID)); err != nil {
					return internalStoreError()
				}
			}
		}
		if !allowed {
			return participantDisputeInvalidState("当前售后申请不能申请平台介入。")
		}
		if err := tx.QueryRow(ctx, `
			UPDATE dispute_cases
			SET status = 'open', requested_platform_action = 'platform_intervention',
			    escalated_by_user_id = $2, escalated_at = $3,
			    next_actor = 'admin', due_at = NULL, public_result = '买家已申请平台介入',
			    updated_at = $3, version = version + 1
			WHERE id = $1 AND active = true
			RETURNING status, requested_platform_action, escalated_by_user_id::text, escalated_at,
			          next_actor, due_at, public_result, updated_at, version
		`, item.ID, input.ActorUserID, now).Scan(
			&item.Status, &item.RequestedPlatformAction, &item.EscalatedByUserID, &item.EscalatedAt,
			&item.NextActor, &item.DueAt, &item.PublicResult, &item.UpdatedAt, &item.Version,
		); err != nil {
			return internalStoreError()
		}
		item.PlatformInterventionReason = strings.TrimSpace(input.Reason)
		order.ActiveRemedyAction = ""
		if appErr := setLockedAPIOrderDisputeProjectionInTx(ctx, tx, order, apiorder.DisputeStatusOpen, input.ActorUserID, apiorder.EventDisputeOpened, "买家已申请平台介入", input.RequestID, now); appErr != nil {
			return appErr
		}
		if appErr := bindEvidenceAssetsInTx(ctx, tx, evidence.BindingInput{
			AssetIDs: input.EvidenceAssetIDs, APIOrderID: order.ID, DisputeCaseID: item.ID,
			UploaderID: input.ActorUserID, Visibility: evidence.VisibilityParticipantsAdmin,
			Usage: evidence.UsagePlatformEscalation, SourceType: evidence.SourceDisputeCase, SourceID: item.ID,
		}, now); appErr != nil {
			return appErr
		}
		if appErr := insertDisputeEvent(ctx, tx, "dispute", item.ID, "platform_intervention_requested", input.ActorUserID, "user", input.Reason, true, input.RequestID, now); appErr != nil {
			return appErr
		}
		return insertDisputeNotifications(ctx, tx, item.ID, "dispute.platform_intervention_requested", "买家已申请平台介入", "售后申请已进入平台审核。", item.ID+":platform_intervention", now, item.CounterpartyUserID)

	case report.DisputeActionRespond:
		if !item.Active || item.Status != report.DisputeStatusOpen || item.CounterpartyUserID != input.ActorUserID || item.RespondedAt != nil || item.NextActor != report.DisputeNextActorRespondent || item.DueAt == nil || !now.Before(*item.DueAt) {
			return participantDisputeInvalidState("只有被申请方可以提交一次正式答复。")
		}
		if err := tx.QueryRow(ctx, `
			UPDATE dispute_cases
			SET respondent_response = $2, responded_by_user_id = $3, responded_at = $4,
			    next_actor = 'admin', due_at = NULL, public_result = '双方材料已提交，等待平台审核',
			    updated_at = $4, version = version + 1
			WHERE id = $1 AND responded_at IS NULL
			RETURNING respondent_response, responded_by_user_id::text, responded_at,
			          next_actor, due_at, public_result, updated_at, version
		`, item.ID, strings.TrimSpace(input.Body), input.ActorUserID, now).Scan(
			&item.RespondentResponse, &item.RespondedByUserID, &item.RespondedAt,
			&item.NextActor, &item.DueAt, &item.PublicResult, &item.UpdatedAt, &item.Version,
		); err != nil {
			return internalStoreError()
		}
		if appErr := bindEvidenceAssetsInTx(ctx, tx, evidence.BindingInput{
			AssetIDs: input.EvidenceAssetIDs, APIOrderID: order.ID, DisputeCaseID: item.ID,
			UploaderID: input.ActorUserID, Visibility: evidence.VisibilityParticipantsAdmin,
			Usage: evidence.UsageFormalResponse, SourceType: evidence.SourceDisputeCase, SourceID: item.ID,
		}, now); appErr != nil {
			return appErr
		}
		if appErr := insertDisputeEvent(ctx, tx, "dispute", item.ID, "respondent_answered", input.ActorUserID, "user", input.Body, true, input.RequestID, now); appErr != nil {
			return appErr
		}
		return insertDisputeNotifications(ctx, tx, item.ID, "dispute.respondent_answered", "被申请方已正式答复", "案件材料已齐，等待平台审核。", item.ID+":responded", now, item.PrimaryUserID)

	case report.DisputeActionWithdraw:
		if !item.Active || item.PrimaryUserID != input.ActorUserID || (item.Status != report.DisputeStatusPendingSellerResponse && item.Status != report.DisputeStatusPendingApplicantDecision) {
			return participantDisputeInvalidState("只有申请人可以在卖家接受或平台介入前撤回申请。")
		}
		var hasRemedy bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM api_order_dispute_remedies WHERE dispute_case_id = $1)`, item.ID).Scan(&hasRemedy); err != nil {
			return internalStoreError()
		}
		if hasRemedy {
			return participantDisputeInvalidState("平台已作出裁决，申请人不能撤回或标记线下解决。")
		}
		targetStatus := report.DisputeStatusWithdrawn
		publicResult := "申请人已撤回售后申请"
		eventType := "withdrawn"
		if _, err := tx.Exec(ctx, `
			UPDATE moderation_info_requests
			SET status = 'cancelled', updated_at = $2, version = version + 1
			WHERE dispute_case_id = $1 AND status = 'open'
		`, item.ID, now); err != nil {
			return internalStoreError()
		}
		if err := tx.QueryRow(ctx, `
			UPDATE dispute_cases
			SET status = $2, active = false, public_result = $3, closed_at = $4,
			    next_actor = 'none', due_at = NULL, updated_at = $4, version = version + 1
			WHERE id = $1
			RETURNING status, active, public_result, closed_at, next_actor, due_at, updated_at, version
		`, item.ID, targetStatus, publicResult, now).Scan(
			&item.Status, &item.Active, &item.PublicResult, &item.ClosedAt,
			&item.NextActor, &item.DueAt, &item.UpdatedAt, &item.Version,
		); err != nil {
			return internalStoreError()
		}
		if appErr := setLockedAPIOrderDisputeProjectionInTx(ctx, tx, order, apiorder.DisputeStatusClosed, input.ActorUserID, apiorder.EventDisputeClosed, publicResult, input.RequestID, now); appErr != nil {
			return appErr
		}
		if appErr := insertDisputeEvent(ctx, tx, "dispute", item.ID, eventType, input.ActorUserID, "user", input.Reason, true, input.RequestID, now); appErr != nil {
			return appErr
		}
		return insertDisputeNotifications(ctx, tx, item.ID, "dispute."+eventType, "平台处理申请已结束", publicResult, item.ID+":"+eventType, now, item.CounterpartyUserID)

	case report.DisputeRemedyActionClaim:
		if (item.Status != report.DisputeStatusResolved && item.Status != report.DisputeStatusVoluntaryFulfillment) || order.DisputeStatus != apiorder.DisputeStatusAwaitingFulfillment {
			return participantDisputeInvalidState("当前纠纷没有待履行的整改要求。")
		}
		remedy, appErr := lockActiveDisputeRemedyInTx(ctx, tx, item.ID)
		if appErr != nil {
			return appErr
		}
		if remedy.Status != report.RemedyStatusPending || remedy.ResponsibleUserID != input.ActorUserID {
			return participantDisputeInvalidState("只有整改责任方可以声明已履行。")
		}
		confirmationWindow := report.RemedyConfirmationWindow
		if remedy.Source == report.RemedySourceSellerAcceptance {
			confirmationWindow = report.VoluntaryRemedyConfirmationWindow
		}
		confirmationDueAt := now.Add(confirmationWindow)
		if _, err := tx.Exec(ctx, `
			UPDATE api_order_dispute_remedies
			SET status = 'claimed_fulfilled', claim_note = $2, claimed_at = $3,
			    confirmation_due_at = $4, claim_request_id = $5,
			    lateness_status = CASE
			        WHEN $3 >= due_at AND lateness_status IN ('late_confirmed', 'late_excused') THEN lateness_status
			        WHEN $3 < due_at THEN 'on_time'
			        ELSE 'late_unreviewed'
			    END,
			    late_at = CASE WHEN $3 < due_at THEN late_at ELSE COALESCE(late_at, due_at) END,
			    claimed_late = $3 >= due_at,
			    updated_at = $3, version = version + 1
			WHERE id = $1
		`, remedy.ID, strings.TrimSpace(input.Note), now, confirmationDueAt, strings.TrimSpace(input.RequestID)); err != nil {
			return internalStoreError()
		}
		if appErr := bindEvidenceAssetsInTx(ctx, tx, evidence.BindingInput{
			AssetIDs: input.EvidenceAssetIDs, APIOrderID: order.ID, DisputeCaseID: item.ID,
			UploaderID: input.ActorUserID, Visibility: evidence.VisibilityParticipantsAdmin,
			Usage: evidence.UsageRemedyClaim, SourceType: evidence.SourceDisputeRemedy, SourceID: remedy.ID,
		}, now); appErr != nil {
			return appErr
		}
		if err := tx.QueryRow(ctx, `
			UPDATE dispute_cases
			SET public_result = '责任方已声明履行，等待对方确认',
			    next_actor = 'counterparty', due_at = $3,
			    updated_at = $2, version = version + 1
			WHERE id = $1
			RETURNING public_result, next_actor, due_at, updated_at, version
		`, item.ID, now, confirmationDueAt).Scan(&item.PublicResult, &item.NextActor, &item.DueAt, &item.UpdatedAt, &item.Version); err != nil {
			return internalStoreError()
		}
		if appErr := setLockedAPIOrderDisputeProjectionInTx(ctx, tx, order, apiorder.DisputeStatusFulfillmentConfirmation, input.ActorUserID, apiorder.EventDisputeRemedyClaimed, "责任方已声明履行，等待对方确认", input.RequestID, now); appErr != nil {
			return appErr
		}
		if appErr := insertDisputeEvent(ctx, tx, "dispute", item.ID, "remedy_claimed_fulfilled", input.ActorUserID, "user", input.Note, true, input.RequestID, now); appErr != nil {
			return appErr
		}
		confirmationHours := strconv.Itoa(int(confirmationWindow.Hours()))
		return insertDisputeNotifications(ctx, tx, item.ID, "dispute.remedy_claimed", "履行声明待确认", "责任方已声明履行，请在 "+confirmationHours+" 小时内确认是否收到或完成。", remedy.ID+":claimed", now, remedy.BeneficiaryUserID)

	case report.DisputeRemedyActionConfirm, report.DisputeRemedyActionContest:
		if (item.Status != report.DisputeStatusResolved && item.Status != report.DisputeStatusVoluntaryFulfillment) || order.DisputeStatus != apiorder.DisputeStatusFulfillmentConfirmation {
			return participantDisputeInvalidState("当前纠纷没有待确认的履行声明。")
		}
		remedy, appErr := lockActiveDisputeRemedyInTx(ctx, tx, item.ID)
		if appErr != nil {
			return appErr
		}
		if remedy.Status != report.RemedyStatusClaimedFulfilled || remedy.BeneficiaryUserID != input.ActorUserID {
			return participantDisputeInvalidState("只有整改受益方可以确认或否认履行结果。")
		}
		if remedy.ConfirmationDueAt != nil && !now.Before(*remedy.ConfirmationDueAt) {
			if _, err := tx.Exec(ctx, `
				UPDATE api_order_dispute_remedies
				SET status = 'confirmation_expired', confirmation_expired_at = $2,
				    response_note = $3, response_request_id = $4,
				    updated_at = $2, version = version + 1
				WHERE id = $1 AND status = 'claimed_fulfilled'
			`, remedy.ID, now, report.RemedyConfirmationExpiredNote, strings.TrimSpace(input.RequestID)); err != nil {
				return internalStoreError()
			}
			publicResult := report.RemedyConfirmationExpiredPublicResult
			if remedy.Source == report.RemedySourceSellerAcceptance {
				publicResult = "买家在确认期内未提出异议，售后申请已中性结束"
			}
			if _, err := tx.Exec(ctx, `UPDATE dispute_cases SET public_result = $2 WHERE id = $1`, item.ID, publicResult); err != nil {
				return internalStoreError()
			}
			item.PublicResult = publicResult
			var finalizeErr *domain.AppError
			if remedy.Source == report.RemedySourceSellerAcceptance {
				finalizeErr = finalizeStoredVoluntaryDisputeInTx(ctx, tx, item, order, "voluntary_confirmation_no_objection", apiorder.CommercialOutcomeClosedUnverified, now)
			} else {
				finalizeErr = finalizeStoredDisputeInTx(ctx, tx, item, order, "remedy_confirmation_expired", nil, apiorder.CommercialOutcomeClosedUnverified, now)
			}
			if finalizeErr != nil {
				return finalizeErr
			}
			order.UpdatedAt = now
			order.Version++
			if appErr := updateAPIOrderInTx(ctx, tx, *order); appErr != nil {
				return appErr
			}
			if order.CommercialOutcomeUpdatedAt != nil {
				if appErr := refreshMutableAPIOrderReviewsInTx(ctx, tx, order.ID, order.CommercialOutcome, *order.CommercialOutcomeUpdatedAt, now); appErr != nil {
					return appErr
				}
			}
			if appErr := insertAPIOrderEventInTx(ctx, tx, *order, "", apiorder.EventDisputeClosed, order.Status, order.Status, "确认期限已到，流程中性结案；平台未核验到账或履约事实。", input.RequestID, now); appErr != nil {
				return appErr
			}
			if appErr := insertDisputeEvent(ctx, tx, "dispute", item.ID, "remedy_confirmation_expired", "", "system", report.RemedyConfirmationExpiredNote, true, input.RequestID, now); appErr != nil {
				return appErr
			}
			return insertDisputeNotifications(ctx, tx, item.ID, "dispute.remedy_confirmation_expired", "整改确认期已结束", "对方未在期限内反馈，流程已中性结案；平台未核验到账或履约事实。", remedy.ID+":confirmation_expired", now, remedy.ResponsibleUserID, remedy.BeneficiaryUserID)
		}
		if input.Action == report.DisputeRemedyActionContest {
			if remedy.Source == report.RemedySourceSellerAcceptance {
				return participantDisputeInvalidState("卖家自愿履行存在异议时，请申请平台介入。")
			}
			if _, err := tx.Exec(ctx, `
				UPDATE api_order_dispute_remedies
				SET status = 'contested', response_note = $2, contested_at = $3,
				    response_request_id = $4, updated_at = $3, version = version + 1
				WHERE id = $1
			`, remedy.ID, strings.TrimSpace(input.Reason), now, strings.TrimSpace(input.RequestID)); err != nil {
				return internalStoreError()
			}
			if appErr := bindEvidenceAssetsInTx(ctx, tx, evidence.BindingInput{
				AssetIDs: input.EvidenceAssetIDs, APIOrderID: order.ID, DisputeCaseID: item.ID,
				UploaderID: input.ActorUserID, Visibility: evidence.VisibilityParticipantsAdmin,
				Usage: evidence.UsageRemedyContest, SourceType: evidence.SourceDisputeRemedy, SourceID: remedy.ID,
			}, now); appErr != nil {
				return appErr
			}
			if err := tx.QueryRow(ctx, `
				UPDATE dispute_cases
				SET status = 'open', public_result = '履行结果有异议，平台重新审核中',
				    resolved_at = NULL, next_actor = 'admin', due_at = NULL,
				    updated_at = $2, version = version + 1
				WHERE id = $1
				RETURNING status, public_result, resolved_at, next_actor, due_at, updated_at, version
			`, item.ID, now).Scan(&item.Status, &item.PublicResult, &item.ResolvedAt, &item.NextActor, &item.DueAt, &item.UpdatedAt, &item.Version); err != nil {
				return internalStoreError()
			}
			if appErr := setLockedAPIOrderDisputeProjectionInTx(ctx, tx, order, apiorder.DisputeStatusOpen, input.ActorUserID, apiorder.EventDisputeRemedyContested, "履行结果有异议，平台重新审核", input.RequestID, now); appErr != nil {
				return appErr
			}
			if appErr := insertDisputeEvent(ctx, tx, "dispute", item.ID, "remedy_contested", input.ActorUserID, "user", input.Reason, true, input.RequestID, now); appErr != nil {
				return appErr
			}
			return insertDisputeNotifications(ctx, tx, item.ID, "dispute.remedy_contested", "整改结果已申请平台复核", "对方反馈未收到或未完成，纠纷已重新进入平台审核。", remedy.ID+":contested", now, remedy.ResponsibleUserID)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE api_order_dispute_remedies
			SET status = 'confirmed', response_note = $2, confirmed_at = $3,
			    response_request_id = $4, updated_at = $3, version = version + 1
			WHERE id = $1
		`, remedy.ID, strings.TrimSpace(input.Reason), now, strings.TrimSpace(input.RequestID)); err != nil {
			return internalStoreError()
		}
		if _, err := tx.Exec(ctx, `UPDATE dispute_cases SET public_result = '对方已确认整改履行完成' WHERE id = $1`, item.ID); err != nil {
			return internalStoreError()
		}
		item.PublicResult = "对方已确认整改履行完成"
		var finalizeErr *domain.AppError
		if remedy.Source == report.RemedySourceSellerAcceptance {
			finalizeErr = finalizeStoredVoluntaryDisputeInTx(ctx, tx, item, order, "voluntary_fulfillment_confirmed", commercialOutcomeForRemedy(remedy.Action), now)
		} else {
			finalizeErr = finalizeStoredDisputeInTx(ctx, tx, item, order, "remedy_confirmed", nil, commercialOutcomeForRemedy(remedy.Action), now)
		}
		if finalizeErr != nil {
			return finalizeErr
		}
		order.UpdatedAt = now
		order.Version++
		if appErr := updateAPIOrderInTx(ctx, tx, *order); appErr != nil {
			return appErr
		}
		if order.CommercialOutcomeUpdatedAt != nil {
			if appErr := refreshMutableAPIOrderReviewsInTx(ctx, tx, order.ID, order.CommercialOutcome, *order.CommercialOutcomeUpdatedAt, now); appErr != nil {
				return appErr
			}
		}
		if appErr := insertAPIOrderEventInTx(ctx, tx, *order, input.ActorUserID, apiorder.EventDisputeClosed, order.Status, order.Status, "对方已确认整改履行完成", input.RequestID, now); appErr != nil {
			return appErr
		}
		if appErr := insertDisputeEvent(ctx, tx, "dispute", item.ID, "remedy_confirmed", input.ActorUserID, "user", input.Reason, true, input.RequestID, now); appErr != nil {
			return appErr
		}
		return insertDisputeNotifications(ctx, tx, item.ID, "dispute.remedy_confirmed", "整改结果已由对方确认", "对方已确认整改履行完成，纠纷已结案。", remedy.ID+":confirmed", now, remedy.ResponsibleUserID)
	default:
		return participantDisputeInvalidState("纠纷参与方动作不支持。")
	}
}

func setLockedAPIOrderDisputeProjectionInTx(ctx context.Context, tx pgx.Tx, order *apiorder.Order, status, actorUserID, eventType, note, requestID string, now time.Time) *domain.AppError {
	if order == nil || !apiorder.IsDisputeActive(order.DisputeStatus) {
		return participantDisputeInvalidState("纠纷关联的 API 订单状态不一致。")
	}
	if status == apiorder.DisputeStatusClosed {
		order.LatestDisputeCaseID = order.DisputeCaseID
		order.DisputeCaseID = ""
		order.DisputeStatus = apiorder.DisputeStatusNone
		order.ActiveRemedyAction = ""
	} else {
		order.DisputeStatus = status
	}
	order.UpdatedAt = now
	order.Version++
	if appErr := updateAPIOrderInTx(ctx, tx, *order); appErr != nil {
		return appErr
	}
	if order.CommercialOutcomeUpdatedAt != nil {
		if appErr := refreshMutableAPIOrderReviewsInTx(ctx, tx, order.ID, order.CommercialOutcome, *order.CommercialOutcomeUpdatedAt, now); appErr != nil {
			return appErr
		}
	}
	return insertAPIOrderEventInTx(ctx, tx, *order, actorUserID, eventType, order.Status, order.Status, note, requestID, now)
}

func finalizeStoredDisputeInTx(ctx context.Context, tx pgx.Tx, item *report.DisputeCase, order *apiorder.Order, finalReason string, affectedUserIDs []string, commercialOutcome string, now time.Time) *domain.AppError {
	if item == nil || order == nil {
		return internalStoreError()
	}
	if len(affectedUserIDs) == 0 {
		if item.SubjectUserID != "" {
			affectedUserIDs = []string{item.SubjectUserID}
		} else {
			affectedUserIDs = []string{item.PrimaryUserID, item.CounterpartyUserID}
		}
	}
	expiresAt := now.Add(report.DisputeAppealWindow)
	if err := tx.QueryRow(ctx, `
		UPDATE dispute_cases
		SET status = 'closed', active = false, closed_at = $2, final_reason = $3,
		    appeal_expires_at = $4, adversely_affected_user_ids = $5::uuid[],
		    next_actor = 'none', due_at = NULL,
		    updated_at = $2, version = version + 1
		WHERE id = $1
		RETURNING status, active, closed_at, final_reason, appeal_expires_at,
		          adversely_affected_user_ids::text[], next_actor, due_at, updated_at, version
	`, item.ID, now, finalReason, expiresAt, affectedUserIDs).Scan(
		&item.Status, &item.Active, &item.ClosedAt, &item.FinalReason, &item.AppealExpiresAt,
		&item.AdverselyAffectedIDs, &item.NextActor, &item.DueAt, &item.UpdatedAt, &item.Version,
	); err != nil {
		return internalStoreError()
	}
	order.LatestDisputeCaseID = item.ID
	order.DisputeCaseID = ""
	order.DisputeStatus = apiorder.DisputeStatusNone
	order.ActiveRemedyAction = ""
	if commercialOutcome != "" {
		order.CommercialOutcome = commercialOutcome
		order.CommercialOutcomeUpdatedAt = &now
	}
	return nil
}

func finalizeStoredVoluntaryDisputeInTx(ctx context.Context, tx pgx.Tx, item *report.DisputeCase, order *apiorder.Order, finalReason, commercialOutcome string, now time.Time) *domain.AppError {
	if item == nil || order == nil {
		return internalStoreError()
	}
	if err := tx.QueryRow(ctx, `
		UPDATE dispute_cases
		SET status = 'closed', active = false, closed_at = $2, final_reason = $3,
		    appeal_expires_at = NULL, adversely_affected_user_ids = '{}'::uuid[],
		    next_actor = 'none', due_at = NULL,
		    updated_at = $2, version = version + 1
		WHERE id = $1
		RETURNING status, active, closed_at, final_reason, appeal_expires_at,
		          adversely_affected_user_ids::text[], next_actor, due_at, updated_at, version
	`, item.ID, now, finalReason).Scan(
		&item.Status, &item.Active, &item.ClosedAt, &item.FinalReason, &item.AppealExpiresAt,
		&item.AdverselyAffectedIDs, &item.NextActor, &item.DueAt, &item.UpdatedAt, &item.Version,
	); err != nil {
		return internalStoreError()
	}
	order.LatestDisputeCaseID = item.ID
	order.DisputeCaseID = ""
	order.DisputeStatus = apiorder.DisputeStatusNone
	order.ActiveRemedyAction = ""
	if commercialOutcome != "" {
		order.CommercialOutcome = commercialOutcome
		order.CommercialOutcomeUpdatedAt = &now
	}
	return nil
}

func participantDisputeInvalidState(detail string) *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", detail)
}

func lockActiveDisputeRemedyInTx(ctx context.Context, tx pgx.Tx, disputeID string) (report.DisputeRemedy, *domain.AppError) {
	return scanLockedDisputeRemedy(tx.QueryRow(ctx, `
		SELECT id::text, dispute_case_id::text, action, COALESCE(amount_cny::text, ''), currency,
		       responsible_user_id::text, beneficiary_user_id::text, instructions, status, due_at,
		       claimed_at, confirmation_due_at, confirmed_at, contested_at,
		       confirmation_expired_at, lateness_status, late_at, lateness_decided_at,
		       COALESCE(lateness_decided_by_admin_id::text, ''), lateness_reason,
		       lateness_reversed_at, COALESCE(lateness_reversed_by_admin_id::text, ''),
		       COALESCE(lateness_reversal_appeal_id::text, ''), lateness_reversal_reason, claimed_late,
		       claim_note, response_note,
		       COALESCE(created_by_admin_id::text, ''), source, COALESCE(settlement_proposal_id::text, ''),
		       created_at, updated_at, version
		FROM api_order_dispute_remedies
		WHERE dispute_case_id = $1 AND status IN ('pending', 'claimed_fulfilled')
		FOR UPDATE
	`, disputeID), "当前纠纷没有进行中的整改要求。")
}

func lockLatestDisputeRemedyInTx(ctx context.Context, tx pgx.Tx, disputeID string) (report.DisputeRemedy, *domain.AppError) {
	return scanLockedDisputeRemedy(tx.QueryRow(ctx, `
		SELECT id::text, dispute_case_id::text, action, COALESCE(amount_cny::text, ''), currency,
		       responsible_user_id::text, beneficiary_user_id::text, instructions, status, due_at,
		       claimed_at, confirmation_due_at, confirmed_at, contested_at,
		       confirmation_expired_at, lateness_status, late_at, lateness_decided_at,
		       COALESCE(lateness_decided_by_admin_id::text, ''), lateness_reason,
		       lateness_reversed_at, COALESCE(lateness_reversed_by_admin_id::text, ''),
		       COALESCE(lateness_reversal_appeal_id::text, ''), lateness_reversal_reason, claimed_late,
		       claim_note, response_note,
		       COALESCE(created_by_admin_id::text, ''), source, COALESCE(settlement_proposal_id::text, ''),
		       created_at, updated_at, version
		FROM api_order_dispute_remedies
		WHERE dispute_case_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT 1
		FOR UPDATE
	`, disputeID), "当前纠纷没有可裁定逾期状态的整改要求。")
}

func scanLockedDisputeRemedy(row scanner, notFoundMessage string) (report.DisputeRemedy, *domain.AppError) {
	var item report.DisputeRemedy
	err := row.Scan(
		&item.ID, &item.DisputeCaseID, &item.Action, &item.AmountCNY, &item.Currency,
		&item.ResponsibleUserID, &item.BeneficiaryUserID, &item.Instructions, &item.Status, &item.DueAt,
		&item.ClaimedAt, &item.ConfirmationDueAt, &item.ConfirmedAt, &item.ContestedAt,
		&item.ConfirmationExpiredAt, &item.LatenessStatus, &item.LateAt, &item.LatenessDecidedAt,
		&item.LatenessDecidedByAdminID, &item.LatenessReason, &item.LatenessReversedAt,
		&item.LatenessReversedByAdminID, &item.LatenessReversalAppealID, &item.LatenessReversalReason, &item.ClaimedLate,
		&item.ClaimNote, &item.ResponseNote, &item.CreatedByAdminID, &item.Source, &item.SettlementProposalID,
		&item.CreatedAt, &item.UpdatedAt, &item.Version,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return report.DisputeRemedy{}, participantDisputeInvalidState(notFoundMessage)
	}
	if err != nil {
		return report.DisputeRemedy{}, internalStoreError()
	}
	return item, nil
}

func loadAPIOrderDisputeNegotiation(ctx context.Context, q queryer, item *report.DisputeCase) *domain.AppError {
	if item == nil || item.TargetType != report.TargetAPIOrder {
		return nil
	}
	messages, err := listAPIOrderDisputeMessages(ctx, q, item.ID)
	if err != nil {
		return internalStoreError()
	}
	proposals, err := listAPIOrderDisputeProposals(ctx, q, item.ID)
	if err != nil {
		return internalStoreError()
	}
	item.Messages = messages
	item.SettlementProposals = proposals
	remedies, err := listAPIOrderDisputeRemedies(ctx, q, item.ID)
	if err != nil {
		return internalStoreError()
	}
	item.Remedies = remedies
	references, err := listEvidenceReferences(ctx, q, item.ID)
	if err != nil {
		return internalStoreError()
	}
	item.Evidence = references
	if err := q.QueryRow(ctx, `
		SELECT reason
		FROM dispute_events
		WHERE entity_type = 'dispute'
		  AND entity_id = $1
		  AND action = 'platform_intervention_requested'
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`, item.ID).Scan(&item.PlatformInterventionReason); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return internalStoreError()
	}
	return nil
}

func listAPIOrderDisputeRemedies(ctx context.Context, q queryer, disputeID string) ([]report.DisputeRemedy, error) {
	rows, err := queryRows(ctx, q, `
		SELECT id::text, dispute_case_id::text, action, COALESCE(amount_cny::text, ''), currency,
		       responsible_user_id::text, beneficiary_user_id::text, instructions, status, due_at,
		       claimed_at, confirmation_due_at, confirmed_at, contested_at,
		       confirmation_expired_at, lateness_status, late_at, lateness_decided_at,
		       COALESCE(lateness_decided_by_admin_id::text, ''), lateness_reason,
		       lateness_reversed_at, COALESCE(lateness_reversed_by_admin_id::text, ''),
		       COALESCE(lateness_reversal_appeal_id::text, ''), lateness_reversal_reason, claimed_late,
		       claim_note, response_note,
		       COALESCE(created_by_admin_id::text, ''), source, COALESCE(settlement_proposal_id::text, ''),
		       created_at, updated_at, version
		FROM api_order_dispute_remedies
		WHERE dispute_case_id = $1
		ORDER BY created_at DESC, id DESC
	`, disputeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]report.DisputeRemedy, 0)
	for rows.Next() {
		var item report.DisputeRemedy
		if err := rows.Scan(
			&item.ID, &item.DisputeCaseID, &item.Action, &item.AmountCNY, &item.Currency,
			&item.ResponsibleUserID, &item.BeneficiaryUserID, &item.Instructions, &item.Status, &item.DueAt,
			&item.ClaimedAt, &item.ConfirmationDueAt, &item.ConfirmedAt, &item.ContestedAt,
			&item.ConfirmationExpiredAt, &item.LatenessStatus, &item.LateAt, &item.LatenessDecidedAt,
			&item.LatenessDecidedByAdminID, &item.LatenessReason, &item.LatenessReversedAt,
			&item.LatenessReversedByAdminID, &item.LatenessReversalAppealID, &item.LatenessReversalReason, &item.ClaimedLate,
			&item.ClaimNote, &item.ResponseNote, &item.CreatedByAdminID, &item.Source, &item.SettlementProposalID,
			&item.CreatedAt, &item.UpdatedAt, &item.Version,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func listAPIOrderDisputeMessages(ctx context.Context, q queryer, disputeID string) ([]report.DisputeMessage, error) {
	rows, err := queryRows(ctx, q, `
		SELECT id::text, dispute_case_id::text, sender_user_id::text, body, created_at
		FROM api_order_dispute_messages
		WHERE dispute_case_id = $1
		ORDER BY created_at, id
	`, disputeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]report.DisputeMessage, 0)
	for rows.Next() {
		var item report.DisputeMessage
		if err := rows.Scan(&item.ID, &item.DisputeCaseID, &item.SenderUserID, &item.Body, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func listAPIOrderDisputeProposals(ctx context.Context, q queryer, disputeID string) ([]report.SettlementProposal, error) {
	rows, err := queryRows(ctx, q, `
		SELECT id::text, dispute_case_id::text, proposed_by_user_id::text, resolution,
		       COALESCE(amount_cny::text, ''), terms, fulfillment_required,
		       COALESCE(responsible_user_id::text, ''), COALESCE(beneficiary_user_id::text, ''), due_at, status,
		       COALESCE(accepted_by_user_id::text, ''), accepted_at,
		       COALESCE(rejected_by_user_id::text, ''), rejected_at,
		       superseded_reason, created_at, updated_at, version
		FROM api_order_dispute_settlement_proposals
		WHERE dispute_case_id = $1
		ORDER BY created_at DESC, id DESC
	`, disputeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]report.SettlementProposal, 0)
	for rows.Next() {
		var item report.SettlementProposal
		if err := rows.Scan(
			&item.ID, &item.DisputeCaseID, &item.ProposedByUserID, &item.Resolution,
			&item.AmountCNY, &item.Terms, &item.FulfillmentRequired, &item.ResponsibleUserID,
			&item.BeneficiaryUserID, &item.DueAt, &item.Status, &item.AcceptedByUserID,
			&item.AcceptedAt, &item.RejectedByUserID, &item.RejectedAt,
			&item.SupersededReason, &item.CreatedAt, &item.UpdatedAt, &item.Version,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) UpdateDisputeAdminWithIdempotency(ctx context.Context, entry idempotency.Entry, input report.AdminActionInput, now time.Time, buildCompletion report.AdminCompletionBuilder) (report.MutationResult, idempotency.Completion, *domain.AppError) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return report.MutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return report.MutationResult{}, idempotency.Completion{}, appErr
	}
	result, appErr := updateDisputeAdminInTx(ctx, tx, input, now)
	if appErr != nil {
		return report.MutationResult{}, idempotency.Completion{}, appErr
	}
	if input.Action == "request_info" {
		request, appErr := createInfoRequestInTx(ctx, tx, input, result, now)
		if appErr != nil {
			return report.MutationResult{}, idempotency.Completion{}, appErr
		}
		result.Dispute.OpenInfoRequestID = request.ID
		result.Dispute.InfoRequestedFromID = request.RequestedFromID
		if appErr := insertInfoRequestOpenedSideEffects(ctx, tx, request, input.RequestID, now); appErr != nil {
			return report.MutationResult{}, idempotency.Completion{}, appErr
		}
	} else if result.Dispute != nil {
		if appErr := cancelOpenInfoRequests(ctx, tx, report.InfoRequestEntityDispute, result.Dispute.ID, now); appErr != nil {
			return report.MutationResult{}, idempotency.Completion{}, appErr
		}
	}
	publicEvent := input.Action == "resolve" || input.Action == "close" || input.Action == "confirm_lateness" || input.Action == "excuse_lateness"
	if appErr := insertDisputeEvent(ctx, tx, "dispute", input.ID, input.Action, input.AdminUserID, "admin", input.Reason, publicEvent, input.RequestID, now); appErr != nil {
		return report.MutationResult{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(result)
	if appErr != nil {
		return report.MutationResult{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return report.MutationResult{}, idempotency.Completion{}, appErr
	}
	if err := tx.Commit(ctx); err != nil {
		return report.MutationResult{}, idempotency.Completion{}, internalStoreError()
	}
	return result, completion, nil
}

func (s *Store) ListPublicUserDisputes(ctx context.Context, username string) ([]report.PublicDispute, *domain.AppError) {
	if appErr := ensurePublicUserExists(ctx, s.pool, username); appErr != nil {
		return nil, appErr
	}
	rows, err := s.pool.Query(ctx, `
		SELECT d.id::text,
		       u.username,
		       d.public_summary,
		       d.public_result,
		       COALESCE(d.resolved_at, d.closed_at, d.updated_at) AS handled_at,
		       d.status IN ('negotiating', 'open', 'waiting_info') AS unresolved
		FROM dispute_cases d
		JOIN users u ON u.id = d.subject_user_id
		WHERE u.username = $1
		  AND u.account_status = 'active'
		ORDER BY handled_at DESC
	`, strings.TrimSpace(strings.ToLower(username)))
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	items := []report.PublicDispute{}
	for rows.Next() {
		var item report.PublicDispute
		if err := rows.Scan(&item.ID, &item.Username, &item.Type, &item.Result, &item.HandledAt, &item.Unresolved); err != nil {
			return nil, internalStoreError()
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, internalStoreError()
	}
	return items, nil
}

func (s *Store) PublicUserDisputeStats(ctx context.Context, username string, now time.Time) (report.PublicStats, *domain.AppError) {
	if appErr := ensurePublicUserExists(ctx, s.pool, username); appErr != nil {
		return report.PublicStats{}, appErr
	}
	var stats report.PublicStats
	if err := s.pool.QueryRow(ctx, `
		SELECT
		  COUNT(*) FILTER (WHERE d.status IN ('negotiating', 'open', 'waiting_info'))::int,
		  COUNT(*) FILTER (
		    WHERE d.status IN ('resolved', 'closed')
		      AND COALESCE(d.resolved_at, d.closed_at, d.updated_at) >= $2
		  )::int
		FROM dispute_cases d
		JOIN users u ON u.id = d.subject_user_id
		WHERE u.username = $1
		  AND u.account_status = 'active'
	`, strings.TrimSpace(strings.ToLower(username)), now.AddDate(0, 0, -90)).Scan(&stats.UnresolvedCount, &stats.ResolvedLast90Days); err != nil {
		return report.PublicStats{}, internalStoreError()
	}
	return stats, nil
}

func createReportInTx(ctx context.Context, tx pgx.Tx, input report.CreateReportInput, now time.Time) (report.Report, *domain.AppError) {
	input.TargetType = strings.TrimSpace(strings.ToLower(input.TargetType))
	input.ReasonCode = strings.TrimSpace(strings.ToLower(input.ReasonCode))
	input.ReportedUsername = strings.TrimSpace(strings.ToLower(input.ReportedUsername))
	resolution, appErr := resolveReportTarget(ctx, tx, input)
	if appErr != nil {
		return report.Report{}, appErr
	}
	if strings.TrimSpace(input.TargetLabel) != "" {
		resolution.TargetLabel = strings.TrimSpace(input.TargetLabel)
	}
	if resolution.ReportedUsername == "" {
		resolution.ReportedUsername = strings.TrimSpace(strings.ToLower(input.ReportedUsername))
	}
	snapshotJSON, appErr := buildReportTargetSnapshot(input, resolution)
	if appErr != nil {
		return report.Report{}, appErr
	}
	if appErr := ensureNoActiveReportForCanonicalTarget(ctx, tx, input.ReporterUserID, resolution.CanonicalTargetType, resolution.CanonicalTargetID); appErr != nil {
		return report.Report{}, appErr
	}
	item, err := scanReport(ctx, tx, `
		INSERT INTO reports (
			reporter_user_id, target_type, target_id, canonical_target_type, canonical_target_id,
			target_label, target_snapshot_json, reported_user_id, reported_username,
			reason_code, title, description, status, created_at, updated_at, version
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8, $9, $10, $11, $12, 'submitted', $13, $13, 1)
		RETURNING `+reportReturningColumns+`
	`, input.ReporterUserID, input.TargetType, strings.TrimSpace(input.TargetID), resolution.CanonicalTargetType, resolution.CanonicalTargetID,
		resolution.TargetLabel, snapshotJSON, nullUUID(resolution.ReportedUserID), resolution.ReportedUsername,
		input.ReasonCode, strings.TrimSpace(input.Title), strings.TrimSpace(input.Description), now)
	if err != nil {
		if isUniqueViolationOnConstraint(err, "ux_reports_active_canonical_target") {
			return report.Report{}, activeReportExists()
		}
		return report.Report{}, internalStoreError()
	}
	return item, nil
}

func createInfoRequestInTx(ctx context.Context, tx pgx.Tx, input report.AdminActionInput, result report.MutationResult, now time.Time) (report.InfoRequest, *domain.AppError) {
	requestedFromID := strings.TrimSpace(input.RequestedFromID)
	entityType := ""
	var reportID any
	var disputeID any
	if result.Report != nil {
		entityType = report.InfoRequestEntityReport
		reportID = result.Report.ID
		if requestedFromID != result.Report.ReporterUserID {
			return report.InfoRequest{}, infoRequestPermissionDenied()
		}
	} else if result.Dispute != nil {
		entityType = report.InfoRequestEntityDispute
		disputeID = result.Dispute.ID
		if !isStoredDisputeParticipant(*result.Dispute, requestedFromID) {
			return report.InfoRequest{}, infoRequestPermissionDenied()
		}
	} else {
		return report.InfoRequest{}, internalStoreError()
	}
	var active bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND account_status = 'active')`, requestedFromID).Scan(&active); err != nil {
		return report.InfoRequest{}, internalStoreError()
	}
	if !active {
		return report.InfoRequest{}, infoRequestPermissionDenied()
	}
	var item report.InfoRequest
	err := tx.QueryRow(ctx, `
		INSERT INTO moderation_info_requests (
			entity_type, report_id, dispute_case_id, requested_from_user_id, requested_by_admin_id,
			internal_reason, status, requested_at, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, 'open', $7, $7)
		RETURNING id::text, entity_type, COALESCE(report_id::text, dispute_case_id::text),
		          requested_from_user_id::text, requested_by_admin_id::text, internal_reason,
		          status, requested_at, answered_at, cancelled_at
	`, entityType, reportID, disputeID, requestedFromID, input.AdminUserID, strings.TrimSpace(input.Reason), now).Scan(
		&item.ID, &item.EntityType, &item.EntityID, &item.RequestedFromID, &item.RequestedByAdminID,
		&item.InternalReason, &item.Status, &item.RequestedAt, &item.AnsweredAt, &item.CancelledAt,
	)
	if err != nil {
		if isUniqueViolationOnConstraint(err, "ux_moderation_info_requests_open_report") || isUniqueViolationOnConstraint(err, "ux_moderation_info_requests_open_dispute") {
			return report.InfoRequest{}, reportInvalidState("该案件已有待补充请求。")
		}
		return report.InfoRequest{}, internalStoreError()
	}
	return item, nil
}

func submitInfoSupplementInTx(ctx context.Context, tx pgx.Tx, input report.SupplementInput, now time.Time) (report.MutationResult, report.InfoRequest, *domain.AppError) {
	var result report.MutationResult
	switch input.EntityType {
	case report.InfoRequestEntityReport:
		current, err := scanReport(ctx, tx, reportSelectSQL+` WHERE r.id = $1 FOR UPDATE OF r`, input.EntityID)
		if errors.Is(err, pgx.ErrNoRows) || err == nil && (current.ReporterUserID != input.SubmittingUserID || current.Status != report.ReportStatusNeedsInfo) {
			return report.MutationResult{}, report.InfoRequest{}, infoRequestNotFound()
		}
		if err != nil {
			return report.MutationResult{}, report.InfoRequest{}, internalStoreError()
		}
		result.Report = &current
	case report.InfoRequestEntityDispute:
		current, err := scanDispute(ctx, tx, disputeSelectSQL+` WHERE d.id = $1 FOR UPDATE OF d`, input.EntityID)
		if errors.Is(err, pgx.ErrNoRows) || err == nil && (!isStoredDisputeParticipant(current, input.SubmittingUserID) || current.Status != report.DisputeStatusWaitingInfo) {
			return report.MutationResult{}, report.InfoRequest{}, infoRequestNotFound()
		}
		if err != nil {
			return report.MutationResult{}, report.InfoRequest{}, internalStoreError()
		}
		if input.ActorAudience == auth.SessionAudienceRestrictedBusiness {
			actor := auth.BusinessActor{UserID: input.SubmittingUserID, Audience: input.ActorAudience, GovernanceActionID: input.GovernanceActionID, GovernanceVersion: input.GovernanceVersion, RestrictionEffectiveAt: input.RestrictionEffectiveAt}
			if appErr := authorizeRestrictedDisputeInTx(ctx, tx, actor, current); appErr != nil {
				return report.MutationResult{}, report.InfoRequest{}, appErr
			}
		} else if input.ActorAudience != "" && input.ActorAudience != auth.SessionAudienceNormal {
			return report.MutationResult{}, report.InfoRequest{}, disputeNotFound()
		}
		result.Dispute = &current
	default:
		return report.MutationResult{}, report.InfoRequest{}, infoRequestNotFound()
	}

	var request report.InfoRequest
	err := tx.QueryRow(ctx, `
		SELECT mir.id::text, mir.entity_type, COALESCE(mir.report_id::text, mir.dispute_case_id::text),
		       mir.requested_from_user_id::text, mir.requested_by_admin_id::text, mir.internal_reason,
		       mir.status, mir.requested_at, mir.answered_at, mir.cancelled_at
		FROM moderation_info_requests mir
		JOIN users requested_user ON requested_user.id = mir.requested_from_user_id
		WHERE mir.id = $1
		  AND mir.entity_type = $2
		  AND COALESCE(mir.report_id::text, mir.dispute_case_id::text) = $3
		  AND mir.requested_from_user_id = $4
		  AND mir.status = 'open'
		  AND (
		    requested_user.account_status = 'active'
		    OR ($5 = 'restricted_business' AND requested_user.account_status IN ('suspended', 'banned') AND requested_user.security_locked_at IS NULL)
		  )
		FOR UPDATE OF mir
	`, input.InfoRequestID, input.EntityType, input.EntityID, input.SubmittingUserID, input.ActorAudience).Scan(
		&request.ID, &request.EntityType, &request.EntityID, &request.RequestedFromID, &request.RequestedByAdminID,
		&request.InternalReason, &request.Status, &request.RequestedAt, &request.AnsweredAt, &request.CancelledAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return report.MutationResult{}, report.InfoRequest{}, infoRequestNotFound()
	}
	if err != nil {
		return report.MutationResult{}, report.InfoRequest{}, internalStoreError()
	}
	switch input.EntityType {
	case report.InfoRequestEntityReport:
		item, err := scanReport(ctx, tx, `
			UPDATE reports SET updated_at = $2, version = version + 1 WHERE id = $1
			RETURNING `+reportReturningColumns+`
		`, result.Report.ID, now)
		if err != nil {
			return report.MutationResult{}, report.InfoRequest{}, internalStoreError()
		}
		result.Report = &item
	case report.InfoRequestEntityDispute:
		item, err := scanDispute(ctx, tx, `
			UPDATE dispute_cases
			SET status = 'open', next_actor = 'admin', due_at = NULL,
			    public_result = '补充材料已提交，等待平台审核', updated_at = $2, version = version + 1
			WHERE id = $1
			RETURNING `+disputeReturningColumns+`
		`, result.Dispute.ID, now)
		if err != nil {
			return report.MutationResult{}, report.InfoRequest{}, internalStoreError()
		}
		result.Dispute = &item
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO moderation_info_supplements (info_request_id, submitted_by_user_id, body, created_at)
		VALUES ($1, $2, $3, $4)
	`, request.ID, input.SubmittingUserID, strings.TrimSpace(input.Body), now); err != nil {
		return report.MutationResult{}, report.InfoRequest{}, internalStoreError()
	}
	if _, err := tx.Exec(ctx, `
		UPDATE moderation_info_requests
		SET status = 'answered', answered_at = $2
		WHERE id = $1 AND status = 'open'
	`, request.ID, now); err != nil {
		return report.MutationResult{}, report.InfoRequest{}, internalStoreError()
	}
	request.Status = report.InfoRequestStatusAnswered
	request.AnsweredAt = &now
	if result.Report != nil {
		result.Report.OpenInfoRequestID = ""
		result.Report.InfoRequestedFromID = ""
	}
	if result.Dispute != nil {
		result.Dispute.OpenInfoRequestID = ""
		result.Dispute.InfoRequestedFromID = ""
	}
	return result, request, nil
}

func cancelOpenInfoRequests(ctx context.Context, tx pgx.Tx, entityType, entityID string, now time.Time) *domain.AppError {
	column := "report_id"
	if entityType == report.InfoRequestEntityDispute {
		column = "dispute_case_id"
	}
	if _, err := tx.Exec(ctx, `UPDATE moderation_info_requests SET status = 'cancelled', cancelled_at = $2 WHERE `+column+` = $1 AND status = 'open'`, entityID, now); err != nil {
		return internalStoreError()
	}
	return nil
}

func isStoredDisputeParticipant(item report.DisputeCase, userID string) bool {
	userID = strings.TrimSpace(userID)
	return userID != "" && (item.PrimaryUserID == userID || item.CounterpartyUserID == userID || item.SubjectUserID == userID)
}

func updateReportAdminInTx(ctx context.Context, tx pgx.Tx, input report.AdminActionInput, now time.Time) (report.MutationResult, *domain.AppError) {
	current, err := scanReport(ctx, tx, reportSelectSQL+` WHERE r.id = $1 FOR UPDATE OF r`, input.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return report.MutationResult{}, reportNotFound()
	}
	if err != nil {
		return report.MutationResult{}, internalStoreError()
	}
	if input.ExpectedVersion > 0 && current.Version != input.ExpectedVersion {
		return report.MutationResult{}, versionConflict()
	}
	switch input.Action {
	case "triage":
		if current.Status != report.ReportStatusSubmitted {
			return report.MutationResult{}, reportInvalidState("只有新提交的举报可以标记分诊。")
		}
		updated, appErr := updateReportStatus(ctx, tx, current.ID, report.ReportStatusTriaged, input.AdminUserID, input.Reason, now)
		if appErr != nil {
			return report.MutationResult{}, appErr
		}
		if appErr := insertReportModerationAuditLog(ctx, tx, input, current, updated, now); appErr != nil {
			return report.MutationResult{}, appErr
		}
		return report.MutationResult{Report: &updated}, nil
	case "request_info":
		if current.Status != report.ReportStatusSubmitted && current.Status != report.ReportStatusTriaged {
			return report.MutationResult{}, reportInvalidState("只有新提交或已分诊的举报可以要求补充信息。")
		}
		updated, appErr := updateReportStatus(ctx, tx, current.ID, report.ReportStatusNeedsInfo, input.AdminUserID, input.Reason, now)
		if appErr != nil {
			return report.MutationResult{}, appErr
		}
		if appErr := insertReportModerationAuditLog(ctx, tx, input, current, updated, now); appErr != nil {
			return report.MutationResult{}, appErr
		}
		return report.MutationResult{Report: &updated}, nil
	case "reject":
		if !canFinishReport(current.Status) {
			return report.MutationResult{}, reportInvalidState("当前举报不能拒绝。")
		}
		updated, appErr := updateReportStatus(ctx, tx, current.ID, report.ReportStatusRejected, input.AdminUserID, input.Reason, now)
		if appErr != nil {
			return report.MutationResult{}, appErr
		}
		if appErr := insertReportModerationAuditLog(ctx, tx, input, current, updated, now); appErr != nil {
			return report.MutationResult{}, appErr
		}
		return report.MutationResult{Report: &updated}, nil
	case "close":
		if !canFinishReport(current.Status) {
			return report.MutationResult{}, reportInvalidState("当前举报不能关闭。")
		}
		updated, appErr := updateReportStatus(ctx, tx, current.ID, report.ReportStatusClosed, input.AdminUserID, input.Reason, now)
		if appErr != nil {
			return report.MutationResult{}, appErr
		}
		if appErr := insertReportModerationAuditLog(ctx, tx, input, current, updated, now); appErr != nil {
			return report.MutationResult{}, appErr
		}
		return report.MutationResult{Report: &updated}, nil
	case "open_dispute":
		if !canOpenDisputeFromReport(current.Status) {
			return report.MutationResult{}, reportInvalidState("当前举报不能打开纠纷。")
		}
		if current.CanonicalTargetType == report.TargetAPIOrder {
			if appErr := lockUndestroyedAPIOrderCredentialForModeration(ctx, tx, current.CanonicalTargetID, "订单交付凭据已按保留规则销毁，无法再打开纠纷。"); appErr != nil {
				return report.MutationResult{}, appErr
			}
		}
		dispute, appErr := openDisputeFromReport(ctx, tx, current, input, now)
		if appErr != nil {
			return report.MutationResult{}, appErr
		}
		updated, appErr := updateReportStatusWithDispute(ctx, tx, current.ID, dispute.ID, input.AdminUserID, input.Reason, now)
		if appErr != nil {
			return report.MutationResult{}, appErr
		}
		if appErr := insertReportModerationAuditLog(ctx, tx, input, current, updated, now); appErr != nil {
			return report.MutationResult{}, appErr
		}
		return report.MutationResult{Report: &updated, Dispute: &dispute}, nil
	default:
		return report.MutationResult{}, reportInvalidState("举报处理动作不支持。")
	}
}

func createAppealInTx(ctx context.Context, tx pgx.Tx, input report.CreateAppealInput, now time.Time) (report.Appeal, *domain.AppError) {
	var sourceReport *report.Report
	var sourceDispute *report.DisputeCase
	reportID := strings.TrimSpace(input.ReportID)
	disputeID := strings.TrimSpace(input.DisputeID)
	if reportID != "" {
		item, err := scanReport(ctx, tx, reportSelectSQL+` WHERE r.id = $1`, reportID)
		if errors.Is(err, pgx.ErrNoRows) {
			return report.Appeal{}, appealSourceNotFound()
		}
		if err != nil {
			return report.Appeal{}, internalStoreError()
		}
		sourceReport = &item
	}
	if disputeID != "" {
		item, err := scanDispute(ctx, tx, disputeSelectSQL+` WHERE d.id = $1 FOR UPDATE OF d`, disputeID)
		if errors.Is(err, pgx.ErrNoRows) {
			return report.Appeal{}, appealSourceNotFound()
		}
		if err != nil {
			return report.Appeal{}, internalStoreError()
		}
		sourceDispute = &item
	}
	source, appErr := report.ResolveAppealSourceAt(input.AppellantUserID, sourceReport, sourceDispute, now)
	if appErr != nil {
		return report.Appeal{}, appErr
	}
	if source.TargetType == report.TargetAPIOrder {
		if appErr := lockUndestroyedAPIOrderCredentialForModeration(ctx, tx, source.TargetID, "订单交付凭据已按保留规则销毁，无法再提交申诉。"); appErr != nil {
			return report.Appeal{}, appErr
		}
	}
	sourceKind := "report"
	sourceID := reportID
	if disputeID != "" {
		sourceKind = "dispute"
		sourceID = disputeID
	}
	lockKey := "appeal:" + strings.TrimSpace(input.AppellantUserID) + ":" + sourceKind + ":" + sourceID
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey); err != nil {
		return report.Appeal{}, internalStoreError()
	}
	var submittedExists bool
	if disputeID != "" {
		err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM appeals
				WHERE appellant_user_id = $1
				  AND dispute_case_id = $2
				  AND status = 'submitted'
			)
		`, input.AppellantUserID, disputeID).Scan(&submittedExists)
		if err != nil {
			return report.Appeal{}, internalStoreError()
		}
	} else {
		err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM appeals
				WHERE appellant_user_id = $1
				  AND report_id = $2
				  AND dispute_case_id IS NULL
				  AND status = 'submitted'
			)
		`, input.AppellantUserID, reportID).Scan(&submittedExists)
		if err != nil {
			return report.Appeal{}, internalStoreError()
		}
	}
	if appErr := report.ValidateNoSubmittedAppeal(submittedExists); appErr != nil {
		return report.Appeal{}, appErr
	}
	item, err := scanAppeal(ctx, tx, `
		INSERT INTO appeals (
			appellant_user_id, report_id, dispute_case_id, target_type, target_id, title, statement,
			status, created_at, updated_at, version
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'submitted', $8, $8, 1)
		RETURNING `+appealReturningColumns+`
	`, input.AppellantUserID, nullUUID(reportID), nullUUID(disputeID), source.TargetType, source.TargetID,
		strings.TrimSpace(input.Title), strings.TrimSpace(input.Statement), now)
	if errors.Is(err, pgx.ErrNoRows) {
		return report.Appeal{}, appealSourceNotFound()
	}
	if err != nil {
		return report.Appeal{}, internalStoreError()
	}
	return item, nil
}

func createAccountGovernanceAppealInTx(ctx context.Context, tx pgx.Tx, input report.CreateAccountGovernanceAppealInput, now time.Time) (report.Appeal, *domain.AppError) {
	appellantUserID := strings.TrimSpace(input.AppellantUserID)
	if appErr := lockAccountGovernanceUser(ctx, tx, appellantUserID); appErr != nil {
		return report.Appeal{}, appErr
	}
	appellant, found, err := accountAppealUserByID(ctx, tx, appellantUserID, true)
	if err != nil {
		return report.Appeal{}, internalStoreError()
	}
	if !found || !isAccountAppealStatus(appellant.Status) {
		return report.Appeal{}, accountAppealStoreIneligibleError()
	}
	canonicalUserID := appellant.ID

	var submittedExists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM appeals
			WHERE appellant_user_id = $1
			  AND target_type = 'account_governance'
			  AND status = 'submitted'
		)
	`, canonicalUserID).Scan(&submittedExists); err != nil {
		return report.Appeal{}, internalStoreError()
	}
	if appErr := report.ValidateNoSubmittedAccountGovernanceAppeal(submittedExists); appErr != nil {
		return report.Appeal{}, appErr
	}

	item, err := scanAppeal(ctx, tx, `
		INSERT INTO appeals (
			appellant_user_id, report_id, dispute_case_id, target_type, target_id, title, statement,
			status, created_at, updated_at, version
		)
		VALUES ($1, NULL, NULL, 'account_governance', $1::uuid::text, '账号治理申诉', $2,
		        'submitted', $3, $3, 1)
		RETURNING `+appealReturningColumns+`
	`, canonicalUserID, strings.TrimSpace(input.Statement), now)
	if err != nil {
		return report.Appeal{}, internalStoreError()
	}
	return item, nil
}

func lockUndestroyedAPIOrderCredentialForModeration(ctx context.Context, tx pgx.Tx, orderID, destroyedDetail string) *domain.AppError {
	if _, err := tx.Exec(ctx, `
		SELECT pg_advisory_xact_lock(hashtextextended($1 || $2::uuid::text, 0))
	`, apiOrderCredentialLifecycleLockPrefix, strings.TrimSpace(orderID)); err != nil {
		return internalStoreError()
	}
	var credentialDestroyed bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM api_order_delivery_credentials
			WHERE api_order_id = $1
			  AND destroyed_at IS NOT NULL
		)
	`, orderID).Scan(&credentialDestroyed); err != nil {
		return internalStoreError()
	}
	if credentialDestroyed {
		return reportInvalidState(destroyedDetail)
	}
	return nil
}

func updateAppealAdminInTx(ctx context.Context, tx pgx.Tx, input report.AdminActionInput, now time.Time) (report.MutationResult, *domain.AppError) {
	current, err := scanAppeal(ctx, tx, appealSelectSQL+` WHERE a.id = $1 FOR UPDATE OF a`, input.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return report.MutationResult{}, appealNotFound()
	}
	if err != nil {
		return report.MutationResult{}, internalStoreError()
	}
	if input.ExpectedVersion > 0 && current.Version != input.ExpectedVersion {
		return report.MutationResult{}, versionConflict()
	}
	if current.Status != report.AppealStatusSubmitted {
		return report.MutationResult{}, reportInvalidState("只有待处理申诉可以审核。")
	}
	next := report.AppealStatusRejected
	if input.Action == "approve" {
		next = report.AppealStatusApproved
	}
	item, err := scanAppeal(ctx, tx, `
		UPDATE appeals
		SET status = $2,
		    admin_reason = $3,
		    handled_by_admin_id = $4,
		    handled_at = $5,
		    updated_at = $5,
		    version = version + 1
		WHERE id = $1
		RETURNING `+appealReturningColumns+`
	`, current.ID, next, strings.TrimSpace(input.Reason), input.AdminUserID, now)
	if err != nil {
		return report.MutationResult{}, internalStoreError()
	}
	if appErr := insertAppealModerationAuditLog(ctx, tx, input, current, item, now); appErr != nil {
		return report.MutationResult{}, appErr
	}
	if input.Action == "approve" {
		if appErr := reverseReputationOutcomeForApprovedAppeal(ctx, tx, item, input, now); appErr != nil {
			return report.MutationResult{}, appErr
		}
	}
	return report.MutationResult{Appeal: &item}, nil
}

func reverseReputationOutcomeForApprovedAppeal(ctx context.Context, tx pgx.Tx, appeal report.Appeal, input report.AdminActionInput, now time.Time) *domain.AppError {
	if appeal.TargetType == report.TargetAccountGovernance {
		return nil
	}
	disputeID := strings.TrimSpace(appeal.DisputeID)
	if disputeID == "" && strings.TrimSpace(appeal.ReportID) != "" {
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(dispute_case_id::text, '')
			FROM reports
			WHERE id = $1
		`, appeal.ReportID).Scan(&disputeID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return internalStoreError()
		}
	}
	if disputeID == "" {
		return nil
	}
	var adverselyAffected bool
	if err := tx.QueryRow(ctx, `
		SELECT $2::uuid = ANY(adversely_affected_user_ids)
		FROM dispute_cases
		WHERE id = $1
		FOR UPDATE
	`, disputeID, appeal.AppellantUserID).Scan(&adverselyAffected); err != nil {
		return internalStoreError()
	}
	if appErr := report.ValidateAppealAdverseSubject(adverselyAffected); appErr != nil {
		return appErr
	}

	beforeOutcome, err := scanDisputeOutcome(tx.QueryRow(ctx, `
		SELECT `+disputeOutcomeReturningColumns+`
		FROM dispute_reputation_outcomes
		WHERE dispute_case_id = $1
		  AND subject_user_id = $2
		  AND status = 'active'
		FOR UPDATE
	`, disputeID, appeal.AppellantUserID))
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return internalStoreError()
	}
	reversalReason := nonEmpty(input.Reason, "申诉批准，反转关联信誉裁定。")
	if err == nil {
		afterOutcome, updateErr := scanDisputeOutcome(tx.QueryRow(ctx, `
			UPDATE dispute_reputation_outcomes
			SET status = 'reversed', reversed_at = $2, reversed_by_admin_id = $3,
			    reversal_appeal_id = $4, reversal_reason = $5,
			    updated_at = $2, version = version + 1
			WHERE id = $1
			RETURNING `+disputeOutcomeReturningColumns+`
		`, beforeOutcome.ID, now, input.AdminUserID, appeal.ID, reversalReason))
		if updateErr != nil {
			return internalStoreError()
		}
		if appErr := insertReputationGovernanceEvent(ctx, tx, "outcome", afterOutcome.ID, "outcome_reversed", input.AdminUserID, beforeOutcome, afterOutcome, reversalReason, input.RequestID, now); appErr != nil {
			return appErr
		}
		if appErr := revokeAppealSubjectRestrictions(ctx, tx, afterOutcome.ID, appeal.AppellantUserID, input, reversalReason, now); appErr != nil {
			return appErr
		}
	}

	commandTag, err := tx.Exec(ctx, `
		UPDATE api_order_dispute_remedies
		SET lateness_reversed_at = $3, lateness_reversed_by_admin_id = $4,
		    lateness_reversal_appeal_id = $5, lateness_reversal_reason = $6,
		    updated_at = $3, version = version + 1
		WHERE dispute_case_id = $1
		  AND responsible_user_id = $2
		  AND lateness_status = 'late_confirmed'
		  AND lateness_reversed_at IS NULL
	`, disputeID, appeal.AppellantUserID, now, input.AdminUserID, appeal.ID, reversalReason)
	if err != nil {
		return internalStoreError()
	}
	if commandTag.RowsAffected() > 0 {
		if appErr := insertDisputeEvent(ctx, tx, "dispute", disputeID, "remedy_lateness_reversed", input.AdminUserID, "admin", reversalReason, false, input.RequestID, now); appErr != nil {
			return appErr
		}
	}
	return nil
}

func revokeAppealSubjectRestrictions(ctx context.Context, tx pgx.Tx, outcomeID, appellantUserID string, input report.AdminActionInput, reversalReason string, now time.Time) *domain.AppError {
	rows, err := tx.Query(ctx, `
		SELECT `+userRestrictionColumns+`
		FROM user_restrictions
		WHERE source_dispute_outcome_id = $1
		  AND user_id = $2
		  AND revoked_at IS NULL
		ORDER BY id
		FOR UPDATE
	`, outcomeID, appellantUserID)
	if err != nil {
		return internalStoreError()
	}
	restrictions := []reputation.UserRestriction{}
	for rows.Next() {
		item, scanErr := scanUserRestriction(rows)
		if scanErr != nil {
			rows.Close()
			return internalStoreError()
		}
		restrictions = append(restrictions, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return internalStoreError()
	}
	rows.Close()
	for _, before := range restrictions {
		after, updateErr := scanUserRestriction(tx.QueryRow(ctx, `
			UPDATE user_restrictions
			SET revoked_at = $2, revoked_by_admin_id = $3, revocation_reason = $4,
			    updated_at = $2, version = version + 1
			WHERE id = $1
			RETURNING `+userRestrictionReturningColumns+`
		`, before.ID, now, input.AdminUserID, reversalReason))
		if updateErr != nil {
			return internalStoreError()
		}
		if appErr := insertReputationGovernanceEvent(ctx, tx, "restriction", after.ID, "restriction_revoked", input.AdminUserID, before, after, reversalReason, input.RequestID, now); appErr != nil {
			return appErr
		}
	}
	return nil
}

func updateDisputeAdminInTx(ctx context.Context, tx pgx.Tx, input report.AdminActionInput, now time.Time) (report.MutationResult, *domain.AppError) {
	current, err := scanDispute(ctx, tx, disputeSelectSQL+` WHERE d.id = $1 FOR UPDATE OF d`, input.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return report.MutationResult{}, disputeNotFound()
	}
	if err != nil {
		return report.MutationResult{}, internalStoreError()
	}
	if input.ExpectedVersion > 0 && current.Version != input.ExpectedVersion {
		return report.MutationResult{}, versionConflict()
	}
	next := current.Status
	resolvedAt := current.ResolvedAt
	closedAt := current.ClosedAt
	active := current.Active
	finalReason := current.FinalReason
	appealExpiresAt := current.AppealExpiresAt
	affectedUserIDs := append([]string{}, current.AdverselyAffectedIDs...)
	nextActor := current.NextActor
	dueAt := current.DueAt
	adminReason := strings.TrimSpace(input.Reason)
	switch input.Action {
	case "request_info":
		if current.Status != report.DisputeStatusOpen {
			return report.MutationResult{}, reportInvalidState("只有打开中的纠纷可以要求补充信息。")
		}
		next = report.DisputeStatusWaitingInfo
		nextActor = report.DisputeNextActorRespondent
		if input.RequestedFromID == current.PrimaryUserID {
			nextActor = report.DisputeNextActorApplicant
		}
		requestDueAt := now.Add(report.DisputeInfoRequestWindow)
		dueAt = &requestDueAt
	case "resolve":
		if current.Status != report.DisputeStatusOpen && current.Status != report.DisputeStatusWaitingInfo {
			return report.MutationResult{}, reportInvalidState("当前纠纷不能标记处理完成。")
		}
		next = report.DisputeStatusResolved
		if current.TargetType == report.TargetAPIOrder && input.Remedy == nil {
			next = report.DisputeStatusClosed
			closedAt = &now
		}
		resolvedAt = &now
		if input.Remedy != nil {
			nextActor = report.DisputeNextActorResponsibleParty
			dueAt = &input.Remedy.DueAt
		} else {
			nextActor = report.DisputeNextActorNone
			dueAt = nil
		}
	case "close":
		if current.Status == report.DisputeStatusClosed {
			return report.MutationResult{}, reportInvalidState("纠纷已关闭。")
		}
		if current.TargetType == report.TargetAPIOrder {
			var activeRemedyID string
			err := tx.QueryRow(ctx, `
				SELECT id::text
				FROM api_order_dispute_remedies
				WHERE dispute_case_id = $1 AND status IN ('pending', 'claimed_fulfilled')
				FOR UPDATE
			`, current.ID).Scan(&activeRemedyID)
			if err == nil {
				return report.MutationResult{}, reportInvalidState("当前纠纷仍有进行中的整改要求，不能直接关闭。")
			}
			if !errors.Is(err, pgx.ErrNoRows) {
				return report.MutationResult{}, internalStoreError()
			}
		}
		next = report.DisputeStatusClosed
		closedAt = &now
		nextActor = report.DisputeNextActorNone
		dueAt = nil
	case "confirm_lateness", "excuse_lateness":
		if current.TargetType != report.TargetAPIOrder || (current.Status != report.DisputeStatusResolved && current.Status != report.DisputeStatusClosed) {
			return report.MutationResult{}, reportInvalidState("当前纠纷没有可裁定逾期状态的整改要求。")
		}
		remedy, appErr := lockLatestDisputeRemedyInTx(ctx, tx, current.ID)
		if appErr != nil {
			return report.MutationResult{}, appErr
		}
		latenessStatus := remedy.LatenessStatus
		lateAt := remedy.LateAt
		if (latenessStatus == "" || latenessStatus == report.RemedyLatenessNotDue) && !now.Before(remedy.DueAt) {
			latenessStatus = report.RemedyLatenessLateUnreviewed
			value := remedy.DueAt
			lateAt = &value
		}
		if latenessStatus != report.RemedyLatenessLateUnreviewed || lateAt == nil {
			return report.MutationResult{}, reportInvalidState("当前整改没有待裁定的客观逾期事实。")
		}
		decidedStatus := report.RemedyLatenessLateExcused
		if input.Action == "confirm_lateness" {
			decidedStatus = report.RemedyLatenessLateConfirmed
		}
		if _, err := tx.Exec(ctx, `
			UPDATE api_order_dispute_remedies
			SET lateness_status = $2, late_at = $3,
			    lateness_decided_at = $4, lateness_decided_by_admin_id = $5,
			    lateness_reason = $6, updated_at = $4, version = version + 1
			WHERE id = $1
		`, remedy.ID, decidedStatus, lateAt, now, input.AdminUserID, strings.TrimSpace(input.Reason)); err != nil {
			return report.MutationResult{}, internalStoreError()
		}
		adminReason = current.AdminReason
		if appErr := insertDisputeNotifications(ctx, tx, current.ID, "dispute.remedy_lateness_decided", "平台已完成整改逾期裁定", "平台已记录整改逾期裁定结果。", remedy.ID+":"+decidedStatus, now, remedy.ResponsibleUserID, remedy.BeneficiaryUserID); appErr != nil {
			return report.MutationResult{}, appErr
		}
	default:
		return report.MutationResult{}, reportInvalidState("纠纷处理动作不支持。")
	}
	if next == report.DisputeStatusClosed {
		active = false
	}
	if !active && (next == report.DisputeStatusResolved || next == report.DisputeStatusClosed) && finalReason == "" {
		switch input.Action {
		case "resolve":
			finalReason = "admin_resolved_no_remedy"
		case "close":
			finalReason = "admin_closed"
		default:
			finalReason = "dispute_finalized"
		}
		var appErr *domain.AppError
		affectedUserIDs, appErr = report.ResolveAdverselyAffectedUsers(current, input.AdverselyAffectedUserIDs)
		if appErr != nil {
			return report.MutationResult{}, appErr
		}
		expiresAt := now.Add(report.DisputeAppealWindow)
		appealExpiresAt = &expiresAt
	}
	item, err := scanDispute(ctx, tx, `
		UPDATE dispute_cases
		SET status = $2,
		    public_summary = $3,
		    public_result_code = $4,
		    public_result = $5,
		    admin_reason = $6,
		    resolved_at = $7,
		    closed_at = $8,
		    active = $9,
		    final_reason = $10,
		    appeal_expires_at = $11,
		    adversely_affected_user_ids = $12::uuid[],
		    next_actor = $13,
		    due_at = $14,
		    updated_at = $15,
		    version = version + 1
		WHERE id = $1
		RETURNING `+disputeReturningColumns+`
		`, current.ID, next, nonEmpty(input.PublicSummary, current.PublicSummary), nonEmpty(input.PublicResultCode, current.PublicResultCode, report.PublicResultNoAction),
		nonEmpty(input.PublicResult, current.PublicResult), adminReason, resolvedAt, closedAt,
		active, finalReason, appealExpiresAt, affectedUserIDs, nextActor, dueAt, now)
	if err != nil {
		return report.MutationResult{}, internalStoreError()
	}
	if item.TargetType == report.TargetAPIOrder {
		switch input.Action {
		case "resolve":
			if input.Remedy == nil {
				if appErr := setAPIOrderDisputeProjectionInTx(ctx, tx, item, input, apiorder.DisputeStatusClosed, apiorder.EventDisputeClosed, "平台裁决无需整改，纠纷已结案", now); appErr != nil {
					return report.MutationResult{}, appErr
				}
			} else {
				if appErr := createAPIOrderDisputeRemedyInTx(ctx, tx, item, input, now); appErr != nil {
					return report.MutationResult{}, appErr
				}
				if appErr := setAPIOrderDisputeProjectionInTx(ctx, tx, item, input, apiorder.DisputeStatusAwaitingFulfillment, apiorder.EventDisputeRemedyAwaiting, "平台已裁决，等待责任方履行", now); appErr != nil {
					return report.MutationResult{}, appErr
				}
			}
		case "close":
			if appErr := setAPIOrderDisputeProjectionInTx(ctx, tx, item, input, apiorder.DisputeStatusClosed, apiorder.EventDisputeClosed, item.PublicResult, now); appErr != nil {
				return report.MutationResult{}, appErr
			}
		}
		if appErr := loadAPIOrderDisputeNegotiation(ctx, tx, &item); appErr != nil {
			return report.MutationResult{}, appErr
		}
	}
	if appErr := insertDisputeModerationAuditLog(ctx, tx, input, current, item, now); appErr != nil {
		return report.MutationResult{}, appErr
	}
	return report.MutationResult{Dispute: &item}, nil
}

func createAPIOrderDisputeRemedyInTx(ctx context.Context, tx pgx.Tx, dispute report.DisputeCase, input report.AdminActionInput, now time.Time) *domain.AppError {
	if input.Remedy == nil || !input.Remedy.DueAt.After(now) {
		return reportInvalidState("整改期限必须晚于当前时间。")
	}
	if !isStoredDisputeParticipant(dispute, input.Remedy.ResponsibleUserID) {
		return reportInvalidState("整改责任方必须是当前 API 订单纠纷参与者。")
	}
	beneficiaryID := dispute.PrimaryUserID
	if beneficiaryID == input.Remedy.ResponsibleUserID {
		beneficiaryID = dispute.CounterpartyUserID
	}
	if beneficiaryID == "" || beneficiaryID == input.Remedy.ResponsibleUserID {
		return reportInvalidState("整改要求缺少有效受益方。")
	}
	var orderAmount string
	var deliverySubmittedAt *time.Time
	if err := tx.QueryRow(ctx, `
		SELECT amount::text, delivery_submitted_at
		FROM api_orders
		WHERE id::text = $1 AND dispute_case_id = $2
		FOR UPDATE
	`, dispute.TargetID, dispute.ID).Scan(&orderAmount, &deliverySubmittedAt); errors.Is(err, pgx.ErrNoRows) {
		return reportInvalidState("纠纷关联的 API 订单不存在或关联不一致。")
	} else if err != nil {
		return internalStoreError()
	}
	if appErr := apiorder.ValidateDisputeResolutionForOrder(apiorder.Order{Amount: orderAmount, DeliverySubmittedAt: deliverySubmittedAt}, input.Remedy.Action, input.Remedy.AmountCNY); appErr != nil {
		return appErr
	}
	remedyID := uuid.NewString()
	if _, err := tx.Exec(ctx, `
		INSERT INTO api_order_dispute_remedies (
			id, dispute_case_id, action, amount_cny, currency,
			responsible_user_id, beneficiary_user_id, instructions, status, due_at,
			lateness_status, source, created_by_admin_id, created_request_id, created_at, updated_at, version
		)
		VALUES ($1, $2, $3, $4, 'CNY', $5, $6, $7, 'pending', $8, 'not_due', 'admin_decision', $9, $10, $11, $11, 1)
	`, remedyID, dispute.ID, input.Remedy.Action, nullNumeric(input.Remedy.AmountCNY),
		input.Remedy.ResponsibleUserID, beneficiaryID, strings.TrimSpace(input.Remedy.Instructions),
		input.Remedy.DueAt, input.AdminUserID, strings.TrimSpace(input.RequestID), now); err != nil {
		if isUniqueViolationOnConstraint(err, "ux_api_order_dispute_remedies_active") {
			return reportInvalidState("当前纠纷已有进行中的整改要求。")
		}
		return internalStoreError()
	}
	if appErr := insertDisputeEvent(ctx, tx, "dispute", dispute.ID, "remedy_created", input.AdminUserID, "admin", input.Remedy.Instructions, true, input.RequestID, now); appErr != nil {
		return appErr
	}
	return insertDisputeNotifications(ctx, tx, dispute.ID, "dispute.remedy_created", "平台已下达整改要求", "平台已作出裁决，请按整改要求和期限完成履行。", remedyID+":created", now, input.Remedy.ResponsibleUserID, beneficiaryID)
}

func setAPIOrderDisputeProjectionInTx(ctx context.Context, tx pgx.Tx, dispute report.DisputeCase, input report.AdminActionInput, targetStatus, eventType, note string, now time.Time) *domain.AppError {
	var orderID string
	var orderStatus string
	var currentStatus string
	err := tx.QueryRow(ctx, `
			SELECT id::text, status, dispute_status
			FROM api_orders
			WHERE id::text = $1
			  AND dispute_case_id = $2
			FOR UPDATE
		`, dispute.TargetID, dispute.ID).Scan(&orderID, &orderStatus, &currentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return reportInvalidState("纠纷关联的 API 订单不存在或关联不一致。")
	}
	if err != nil {
		return internalStoreError()
	}
	if currentStatus == targetStatus && targetStatus != apiorder.DisputeStatusClosed {
		return nil
	}
	if !apiorder.IsDisputeActive(currentStatus) {
		return reportInvalidState("纠纷关联的 API 订单状态不一致，无法结案。")
	}
	nextStatus := targetStatus
	activeCaseID := dispute.ID
	activeRemedyAction := ""
	commercialOutcome := ""
	var commercialOutcomeAt *time.Time
	if targetStatus == apiorder.DisputeStatusAwaitingFulfillment && input.Remedy != nil {
		activeRemedyAction = input.Remedy.Action
	}
	if targetStatus == apiorder.DisputeStatusClosed {
		nextStatus = apiorder.DisputeStatusNone
		activeCaseID = ""
		commercialOutcome = apiorder.CommercialOutcomeClosedUnverified
		commercialOutcomeAt = &now
	}
	commandTag, err := tx.Exec(ctx, `
		UPDATE api_orders
		SET dispute_status = $2,
		    dispute_case_id = NULLIF($3, '')::uuid,
		    latest_dispute_case_id = $4,
		    active_remedy_action = $5,
		    commercial_outcome = COALESCE(NULLIF($6, ''), commercial_outcome),
		    commercial_outcome_updated_at = COALESCE($7, commercial_outcome_updated_at),
		    updated_at = $8,
		    version = version + 1
		WHERE id = $1
		  AND dispute_status = $9
	`, orderID, nextStatus, activeCaseID, dispute.ID, activeRemedyAction, commercialOutcome, commercialOutcomeAt, now, currentStatus)
	if err != nil {
		return internalStoreError()
	}
	if commandTag.RowsAffected() != 1 {
		return versionConflict()
	}
	return insertAPIOrderEventInTx(ctx, tx, apiorder.Order{ID: orderID}, input.AdminUserID, eventType, orderStatus, orderStatus, note, input.RequestID, now)
}

func updateReportStatus(ctx context.Context, tx pgx.Tx, id, status, adminID, reason string, now time.Time) (report.Report, *domain.AppError) {
	item, err := scanReport(ctx, tx, `
		UPDATE reports
		SET status = $2,
		    admin_reason = $3,
		    handled_by_admin_id = $4,
		    handled_at = $5,
		    updated_at = $5,
		    version = version + 1
		WHERE id = $1
		RETURNING `+reportReturningColumns+`
	`, id, status, strings.TrimSpace(reason), adminID, now)
	if err != nil {
		return report.Report{}, internalStoreError()
	}
	return item, nil
}

func updateReportStatusWithDispute(ctx context.Context, tx pgx.Tx, id, disputeID, adminID, reason string, now time.Time) (report.Report, *domain.AppError) {
	item, err := scanReport(ctx, tx, `
		UPDATE reports
		SET status = 'dispute_opened',
		    dispute_case_id = $2,
		    admin_reason = $3,
		    handled_by_admin_id = $4,
		    handled_at = $5,
		    updated_at = $5,
		    version = version + 1
		WHERE id = $1
		RETURNING `+reportReturningColumns+`
	`, id, disputeID, strings.TrimSpace(reason), adminID, now)
	if err != nil {
		return report.Report{}, internalStoreError()
	}
	return item, nil
}

func openDisputeFromReport(ctx context.Context, tx pgx.Tx, source report.Report, input report.AdminActionInput, now time.Time) (report.DisputeCase, *domain.AppError) {
	var counterpartyID any
	if strings.TrimSpace(source.ReportedUsername) != "" {
		userID, appErr := userIDForUsername(ctx, tx, source.ReportedUsername)
		if appErr != nil {
			return report.DisputeCase{}, appErr
		}
		if userID != "" {
			counterpartyID = userID
		}
	}
	item, err := scanDispute(ctx, tx, `
		INSERT INTO dispute_cases (
			report_id, target_type, target_id, api_order_id, active, target_label, primary_user_id, counterparty_user_id,
			subject_user_id,
			status, public_summary, public_result_code, public_result, admin_reason, opened_by_admin_id, opened_at,
			created_at, updated_at, version
		)
			VALUES ($1, $2, $3::text, CASE WHEN $2 = 'api_order' THEN $3::text::uuid ELSE NULL END, $2 = 'api_order', $4, $5, $6, $6, 'open', $7, $8, $9, $10, $11, $12, $12, $12, 1)
		RETURNING `+disputeReturningColumns+`
	`, source.ID, nonEmpty(source.CanonicalTargetType, source.TargetType), nonEmpty(source.CanonicalTargetID, source.TargetID), source.TargetLabel, source.ReporterUserID, counterpartyID,
		strings.TrimSpace(input.PublicSummary), nonEmpty(input.PublicResultCode, report.PublicResultNoAction),
		nonEmpty(input.PublicResult, "已进入人工处理中"), strings.TrimSpace(input.Reason), input.AdminUserID, now)
	if err != nil {
		return report.DisputeCase{}, internalStoreError()
	}
	return item, nil
}

func resolveReportTarget(ctx context.Context, q queryer, input report.CreateReportInput) (reportTargetResolution, *domain.AppError) {
	targetID := strings.TrimSpace(input.TargetID)
	switch input.TargetType {
	case report.TargetPublicUser:
		username := strings.TrimSpace(strings.ToLower(input.ReportedUsername))
		if username == "" {
			username = strings.TrimSpace(strings.ToLower(targetID))
		}
		userID, appErr := userIDForUsername(ctx, q, username)
		if appErr != nil {
			return reportTargetResolution{}, appErr
		}
		if userID == "" {
			return reportTargetResolution{}, publicProfileNotFound()
		}
		if input.ReporterUserID == userID || strings.EqualFold(input.ReporterUsername, username) {
			return reportTargetResolution{}, selfReportForbidden()
		}
		return reportTargetResolution{
			TargetLabel:         "公开主页 @" + username,
			CanonicalTargetType: report.TargetPublicUser,
			CanonicalTargetID:   userID,
			ReportedUserID:      userID,
			ReportedUsername:    username,
			ReporterRole:        "reporter",
			RespondentUserID:    userID,
			RespondentUsername:  username,
			Participants: []reportTargetParticipant{{
				Role:     "reported_user",
				UserID:   userID,
				Username: username,
			}},
			BusinessStatus: "active",
		}, nil
	case report.TargetContactSnapshot:
		return resolveContactSnapshotTarget(ctx, q, input)
	case report.TargetCarpoolApplication:
		resolution, found, appErr := resolveCarpoolApplicationTarget(ctx, q, input)
		if appErr != nil {
			return reportTargetResolution{}, appErr
		}
		if !found {
			return reportTargetResolution{}, targetNotFound()
		}
		return resolution, nil
	case report.TargetCarpoolMembership:
		return resolveCarpoolMembershipTarget(ctx, q, input)
	case report.TargetAPIPurchaseIntent:
		resolution, found, appErr := resolveAPIIntentTarget(ctx, q, input)
		if appErr != nil {
			return reportTargetResolution{}, appErr
		}
		if !found {
			return reportTargetResolution{}, targetNotFound()
		}
		return resolution, nil
	case report.TargetAPIOrder:
		resolution, found, appErr := resolveAPIOrderTarget(ctx, q, input)
		if appErr != nil {
			return reportTargetResolution{}, appErr
		}
		if !found {
			return reportTargetResolution{}, targetNotFound()
		}
		return resolution, nil
	default:
		return reportTargetResolution{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Report validation failed", "举报目标类型不支持。", "targetType", "invalid", "举报目标类型不支持。")
	}
}

func resolveContactSnapshotTarget(ctx context.Context, q queryer, input report.CreateReportInput) (reportTargetResolution, *domain.AppError) {
	if resolution, found, appErr := resolveCarpoolApplicationTarget(ctx, q, input); appErr != nil || found {
		return resolution, appErr
	}
	if resolution, found, appErr := resolveAPIOrderTarget(ctx, q, input); appErr != nil || found {
		return resolution, appErr
	}
	if resolution, found, appErr := resolveAPIIntentTarget(ctx, q, input); appErr != nil || found {
		return resolution, appErr
	}
	return reportTargetResolution{}, targetNotFound()
}

func resolveCarpoolApplicationTarget(ctx context.Context, q queryer, input report.CreateReportInput) (reportTargetResolution, bool, *domain.AppError) {
	targetID := strings.TrimSpace(input.TargetID)
	var title, status, ownerID, ownerUsername, buyerID, buyerUsername, membershipID, membershipStatus string
	err := q.QueryRow(ctx, `
		SELECT a.listing_title_snapshot, a.status, owner.id::text, owner.username,
		       buyer.id::text, buyer.username, COALESCE(m.id::text, ''), COALESCE(m.status, '')
		FROM carpool_applications a
		JOIN users owner ON owner.id = a.owner_user_id
		JOIN users buyer ON buyer.id = a.buyer_user_id
		LEFT JOIN carpool_memberships m ON m.carpool_application_id = a.id
		WHERE a.id = $1
	`, targetID).Scan(&title, &status, &ownerID, &ownerUsername, &buyerID, &buyerUsername, &membershipID, &membershipStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return reportTargetResolution{}, false, nil
	}
	if err != nil {
		return reportTargetResolution{}, true, internalStoreError()
	}
	reporterRole, respondentID, respondentUsername, appErr := participantRole(input.ReporterUserID, ownerID, ownerUsername, buyerID, buyerUsername, "owner", "buyer")
	if appErr != nil {
		return reportTargetResolution{}, true, appErr
	}
	canonicalType := report.TargetCarpoolApplication
	canonicalID := targetID
	if membershipID != "" {
		canonicalType = report.TargetCarpoolMembership
		canonicalID = membershipID
	}
	return reportTargetResolution{
		TargetLabel:         nonEmpty(input.TargetLabel, title, "拼车申请"),
		CanonicalTargetType: canonicalType,
		CanonicalTargetID:   canonicalID,
		ReportedUserID:      respondentID,
		ReportedUsername:    respondentUsername,
		ReporterRole:        reporterRole,
		RespondentUserID:    respondentID,
		RespondentUsername:  respondentUsername,
		Participants:        reportParticipants("owner", ownerID, ownerUsername, "buyer", buyerID, buyerUsername),
		BusinessStatus:      joinedStatus("application", status, "membership", membershipStatus),
		HasMembership:       membershipID != "",
	}, true, nil
}

func resolveCarpoolMembershipTarget(ctx context.Context, q queryer, input report.CreateReportInput) (reportTargetResolution, *domain.AppError) {
	targetID := strings.TrimSpace(input.TargetID)
	var title, status, ownerID, ownerUsername, buyerID, buyerUsername string
	err := q.QueryRow(ctx, `
		SELECT l.title, m.status, owner.id::text, owner.username, buyer.id::text, buyer.username
		FROM carpool_memberships m
		JOIN carpool_listings l ON l.id = m.carpool_listing_id
		JOIN users owner ON owner.id = m.owner_user_id
		JOIN users buyer ON buyer.id = m.buyer_user_id
		WHERE m.id = $1
	`, targetID).Scan(&title, &status, &ownerID, &ownerUsername, &buyerID, &buyerUsername)
	if errors.Is(err, pgx.ErrNoRows) {
		return reportTargetResolution{}, targetNotFound()
	}
	if err != nil {
		return reportTargetResolution{}, internalStoreError()
	}
	reporterRole, respondentID, respondentUsername, appErr := participantRole(input.ReporterUserID, ownerID, ownerUsername, buyerID, buyerUsername, "owner", "buyer")
	if appErr != nil {
		return reportTargetResolution{}, appErr
	}
	return reportTargetResolution{
		TargetLabel:         nonEmpty(input.TargetLabel, title, "拼车成员关系"),
		CanonicalTargetType: report.TargetCarpoolMembership,
		CanonicalTargetID:   targetID,
		ReportedUserID:      respondentID,
		ReportedUsername:    respondentUsername,
		ReporterRole:        reporterRole,
		RespondentUserID:    respondentID,
		RespondentUsername:  respondentUsername,
		Participants:        reportParticipants("owner", ownerID, ownerUsername, "buyer", buyerID, buyerUsername),
		BusinessStatus:      status,
		HasMembership:       true,
	}, nil
}

func resolveAPIIntentTarget(ctx context.Context, q queryer, input report.CreateReportInput) (reportTargetResolution, bool, *domain.AppError) {
	targetID := strings.TrimSpace(input.TargetID)
	var title, status, ownerID, ownerUsername, buyerID, buyerUsername, orderID, orderStatus string
	err := q.QueryRow(ctx, `
		SELECT i.service_title_snapshot, i.status, owner.id::text, owner.username,
		       buyer.id::text, buyer.username, COALESCE(o.id::text, ''), COALESCE(o.status, '')
		FROM api_purchase_intents i
		JOIN users owner ON owner.id = i.owner_user_id
		JOIN users buyer ON buyer.id = i.buyer_user_id
		LEFT JOIN api_orders o ON o.api_purchase_intent_id = i.id
		WHERE i.id = $1
	`, targetID).Scan(&title, &status, &ownerID, &ownerUsername, &buyerID, &buyerUsername, &orderID, &orderStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return reportTargetResolution{}, false, nil
	}
	if err != nil {
		return reportTargetResolution{}, true, internalStoreError()
	}
	reporterRole, respondentID, respondentUsername, appErr := participantRole(input.ReporterUserID, ownerID, ownerUsername, buyerID, buyerUsername, "merchant", "buyer")
	if appErr != nil {
		return reportTargetResolution{}, true, appErr
	}
	canonicalType := report.TargetAPIPurchaseIntent
	canonicalID := targetID
	if orderID != "" {
		canonicalType = report.TargetAPIOrder
		canonicalID = orderID
	}
	return reportTargetResolution{
		TargetLabel:         nonEmpty(input.TargetLabel, title, "API 购买意向"),
		CanonicalTargetType: canonicalType,
		CanonicalTargetID:   canonicalID,
		ReportedUserID:      respondentID,
		ReportedUsername:    respondentUsername,
		ReporterRole:        reporterRole,
		RespondentUserID:    respondentID,
		RespondentUsername:  respondentUsername,
		Participants:        reportParticipants("merchant", ownerID, ownerUsername, "buyer", buyerID, buyerUsername),
		BusinessStatus:      joinedStatus("intent", status, "order", orderStatus),
		HasOrder:            orderID != "",
	}, true, nil
}

func resolveAPIOrderTarget(ctx context.Context, q queryer, input report.CreateReportInput) (reportTargetResolution, bool, *domain.AppError) {
	targetID := strings.TrimSpace(input.TargetID)
	var canonicalID, title, status, ownerID, ownerUsername, buyerID, buyerUsername string
	err := q.QueryRow(ctx, `
			SELECT o.id::text, o.service_title_snapshot, o.status, owner.id::text, owner.username, buyer.id::text, buyer.username
			FROM api_orders o
			JOIN users owner ON owner.id = o.seller_user_id
			JOIN users buyer ON buyer.id = o.buyer_user_id
			WHERE o.id = $1
		`, targetID).Scan(&canonicalID, &title, &status, &ownerID, &ownerUsername, &buyerID, &buyerUsername)
	if errors.Is(err, pgx.ErrNoRows) {
		return reportTargetResolution{}, false, nil
	}
	if err != nil {
		return reportTargetResolution{}, true, internalStoreError()
	}
	reporterRole, respondentID, respondentUsername, appErr := participantRole(input.ReporterUserID, ownerID, ownerUsername, buyerID, buyerUsername, "merchant", "buyer")
	if appErr != nil {
		return reportTargetResolution{}, true, appErr
	}
	return reportTargetResolution{
		TargetLabel:         nonEmpty(input.TargetLabel, title, "API 订单"),
		CanonicalTargetType: report.TargetAPIOrder,
		CanonicalTargetID:   canonicalID,
		ReportedUserID:      respondentID,
		ReportedUsername:    respondentUsername,
		ReporterRole:        reporterRole,
		RespondentUserID:    respondentID,
		RespondentUsername:  respondentUsername,
		Participants:        reportParticipants("merchant", ownerID, ownerUsername, "buyer", buyerID, buyerUsername),
		BusinessStatus:      status,
		HasOrder:            true,
	}, true, nil
}

func participantRole(reporterID, ownerID, ownerUsername, buyerID, buyerUsername, ownerRole, buyerRole string) (string, string, string, *domain.AppError) {
	switch reporterID {
	case ownerID:
		return ownerRole, buyerID, buyerUsername, nil
	case buyerID:
		return buyerRole, ownerID, ownerUsername, nil
	default:
		return "", "", "", reportPermissionDenied()
	}
}

func reportParticipants(firstRole, firstUserID, firstUsername, secondRole, secondUserID, secondUsername string) []reportTargetParticipant {
	return []reportTargetParticipant{
		{Role: firstRole, UserID: firstUserID, Username: firstUsername},
		{Role: secondRole, UserID: secondUserID, Username: secondUsername},
	}
}

func buildReportTargetSnapshot(input report.CreateReportInput, resolution reportTargetResolution) (string, *domain.AppError) {
	payload := map[string]any{
		"submittedTargetType":       strings.TrimSpace(input.TargetType),
		"submittedTargetId":         strings.TrimSpace(input.TargetID),
		"canonicalTargetType":       resolution.CanonicalTargetType,
		"canonicalTargetId":         resolution.CanonicalTargetID,
		"targetLabel":               resolution.TargetLabel,
		"reportedUsername":          resolution.ReportedUsername,
		"reporterRole":              resolution.ReporterRole,
		"primaryRespondentUserId":   resolution.RespondentUserID,
		"primaryRespondentUsername": resolution.RespondentUsername,
		"participants":              resolution.Participants,
		"businessStatus":            resolution.BusinessStatus,
		"hasOrder":                  resolution.HasOrder,
		"hasMembership":             resolution.HasMembership,
		"containsContactValue":      false,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", internalStoreError()
	}
	return string(data), nil
}

func ensureNoActiveReportForCanonicalTarget(ctx context.Context, q queryer, reporterID, targetType, targetID string) *domain.AppError {
	var existingID string
	err := q.QueryRow(ctx, `
		SELECT id::text
		FROM reports
		WHERE reporter_user_id = $1
		  AND canonical_target_type = $2
		  AND canonical_target_id = $3
		  AND status IN ('submitted', 'triaged', 'needs_info', 'dispute_opened')
		LIMIT 1
	`, reporterID, targetType, targetID).Scan(&existingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return internalStoreError()
	}
	return activeReportExists()
}

func joinedStatus(firstLabel, firstValue, secondLabel, secondValue string) string {
	firstValue = strings.TrimSpace(firstValue)
	secondValue = strings.TrimSpace(secondValue)
	if secondValue == "" {
		return firstValue
	}
	return firstLabel + ":" + firstValue + " " + secondLabel + ":" + secondValue
}

func userIDForUsername(ctx context.Context, q queryer, username string) (string, *domain.AppError) {
	username = strings.TrimSpace(strings.ToLower(username))
	if username == "" {
		return "", nil
	}
	var userID string
	err := q.QueryRow(ctx, `SELECT id::text FROM users WHERE username = $1 AND account_status = 'active'`, username).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", internalStoreError()
	}
	return userID, nil
}

func ensurePublicUserExists(ctx context.Context, q queryer, username string) *domain.AppError {
	userID, appErr := userIDForUsername(ctx, q, username)
	if appErr != nil {
		return appErr
	}
	if userID == "" {
		return publicProfileNotFound()
	}
	return nil
}

func insertInfoRequestOpenedSideEffects(ctx context.Context, tx pgx.Tx, request report.InfoRequest, requestID string, now time.Time) *domain.AppError {
	eventID := uuid.NewString()
	requestID = nonEmpty(requestID, "unknown")
	metadata, err := json.Marshal(map[string]string{"entityType": request.EntityType, "status": report.InfoRequestStatusOpen})
	if err != nil {
		return internalStoreError()
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO domain_events (id, aggregate_type, aggregate_id, event_type, actor_user_id, actor_kind, aggregate_version, request_id, metadata_json, created_at)
		VALUES ($1, 'moderation_info_request', $2, 'moderation.info_requested', $3, 'admin', 1, $4, $5, $6)
	`, eventID, request.ID, request.RequestedByAdminID, requestID, metadata, now); err != nil {
		return internalStoreError()
	}
	targetURL := "/my/reports/report/" + request.EntityID
	if request.EntityType == report.InfoRequestEntityDispute {
		targetURL = "/my/disputes/" + request.EntityID
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO notifications (
			user_id, type, title, body, target_type, target_id, target_url,
			source_event_type, source_event_id, dedupe_key, created_at
		)
		VALUES ($1, 'moderation_info_requested', '平台需要你补充案件材料',
		        '请提交脱敏事实说明，不要包含联系方式或任何凭据。', $2, $3, $4,
		        'moderation.info_requested', $5, $6, $7)
		ON CONFLICT (user_id, dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING
	`, request.RequestedFromID, request.EntityType, request.EntityID, targetURL, eventID, "moderation_info_request:"+request.ID+":opened", now); err != nil {
		return internalStoreError()
	}
	return nil
}

func insertInfoSupplementSideEffects(ctx context.Context, tx pgx.Tx, request report.InfoRequest, requestID string, now time.Time) *domain.AppError {
	eventID := uuid.NewString()
	requestID = nonEmpty(requestID, "unknown")
	metadata, err := json.Marshal(map[string]string{"entityType": request.EntityType, "status": report.InfoRequestStatusAnswered})
	if err != nil {
		return internalStoreError()
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO domain_events (id, aggregate_type, aggregate_id, event_type, actor_user_id, actor_kind, aggregate_version, request_id, metadata_json, created_at)
		VALUES ($1, 'moderation_info_request', $2, 'moderation.info_supplemented', $3, 'user', 2, $4, $5, $6)
	`, eventID, request.ID, request.RequestedFromID, requestID, metadata, now); err != nil {
		return internalStoreError()
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO notifications (
			user_id, type, title, body, target_type, target_id, target_url,
			source_event_type, source_event_id, dedupe_key, created_at
		)
		VALUES ($1, 'moderation_info_supplemented', '用户已补充案件材料',
		        '用户已提交脱敏补充说明，请重新查看案件。', $2, $3, '/admin/reports',
		        'moderation.info_supplemented', $4, $5, $6)
		ON CONFLICT (user_id, dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING
	`, request.RequestedByAdminID, request.EntityType, request.EntityID, eventID, "moderation_info_request:"+request.ID+":answered", now); err != nil {
		return internalStoreError()
	}
	return nil
}

func insertDisputeEvent(ctx context.Context, tx pgx.Tx, entityType, entityID, action, actorID, actorRole, reason string, public bool, requestID string, now time.Time) *domain.AppError {
	_, err := tx.Exec(ctx, `
		INSERT INTO dispute_events (entity_type, entity_id, action, actor_user_id, actor_role, reason, public, request_id, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, entityType, entityID, action, nullUUID(actorID), actorRole, strings.TrimSpace(reason), public, strings.TrimSpace(requestID), now)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func insertDisputeNotifications(ctx context.Context, tx pgx.Tx, disputeID, eventType, title, body, dedupeSuffix string, now time.Time, userIDs ...string) *domain.AppError {
	for _, userID := range userIDs {
		userID = strings.TrimSpace(userID)
		if userID == "" {
			continue
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO notifications (
				id, user_id, type, title, body, target_type, target_id, target_url,
				source_event_type, dedupe_key, created_at
			)
			VALUES ($1, $2, $3, $4, $5, 'dispute', $6, $7, $3, $8, $9)
			ON CONFLICT (user_id, dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING
		`, uuid.NewString(), userID, eventType, title, body, disputeID, "/my/disputes/"+disputeID,
			"api-order-dispute-remedy:"+disputeID+":"+dedupeSuffix, now); err != nil {
			return internalStoreError()
		}
	}
	return nil
}

func insertReportModerationAuditLog(ctx context.Context, tx pgx.Tx, input report.AdminActionInput, before, after report.Report, now time.Time) *domain.AppError {
	return insertModerationAuditLog(ctx, tx, input, "report", after.ID, after.ID, "", "", reportAuditPayload(before), reportAuditPayload(after), now)
}

func insertDisputeModerationAuditLog(ctx context.Context, tx pgx.Tx, input report.AdminActionInput, before, after report.DisputeCase, now time.Time) *domain.AppError {
	return insertModerationAuditLog(ctx, tx, input, "dispute_case", after.ID, after.ReportID, after.ID, "", disputeAuditPayload(before), disputeAuditPayload(after), now)
}

func insertAppealModerationAuditLog(ctx context.Context, tx pgx.Tx, input report.AdminActionInput, before, after report.Appeal, now time.Time) *domain.AppError {
	return insertModerationAuditLog(ctx, tx, input, "appeal", after.ID, after.ReportID, after.DisputeID, after.ID, appealAuditPayload(before), appealAuditPayload(after), now)
}

func insertModerationAuditLog(ctx context.Context, tx pgx.Tx, input report.AdminActionInput, objectType, objectID, basisReportID, basisDisputeID, basisAppealID string, beforePayload, afterPayload map[string]any, now time.Time) *domain.AppError {
	beforeJSON, err := json.Marshal(beforePayload)
	if err != nil {
		return internalStoreError()
	}
	afterJSON, err := json.Marshal(afterPayload)
	if err != nil {
		return internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO moderation_audit_logs (
			actor_admin_id, action, object_type, object_id,
			basis_report_id, basis_dispute_case_id, basis_appeal_id,
			before_json, after_json, reason_internal, request_id, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9::jsonb, $10, $11, $12)
	`, input.AdminUserID, input.Action, objectType, objectID, nullUUID(basisReportID), nullUUID(basisDisputeID), nullUUID(basisAppealID),
		string(beforeJSON), string(afterJSON), strings.TrimSpace(input.Reason), strings.TrimSpace(input.RequestID), now)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func reportAuditPayload(item report.Report) map[string]any {
	return map[string]any{
		"id":                  item.ID,
		"status":              item.Status,
		"version":             item.Version,
		"canonicalTargetType": item.CanonicalTargetType,
		"canonicalTargetId":   item.CanonicalTargetID,
		"disputeId":           item.DisputeID,
		"handledAt":           item.HandledAt,
	}
}

func disputeAuditPayload(item report.DisputeCase) map[string]any {
	return map[string]any{
		"id":               item.ID,
		"reportId":         item.ReportID,
		"status":           item.Status,
		"version":          item.Version,
		"targetType":       item.TargetType,
		"targetId":         item.TargetID,
		"subjectUserId":    item.SubjectUserID,
		"publicSummary":    item.PublicSummary,
		"publicResultCode": item.PublicResultCode,
		"publicResult":     item.PublicResult,
		"resolvedAt":       item.ResolvedAt,
		"closedAt":         item.ClosedAt,
	}
}

func appealAuditPayload(item report.Appeal) map[string]any {
	return map[string]any{
		"id":        item.ID,
		"reportId":  item.ReportID,
		"disputeId": item.DisputeID,
		"status":    item.Status,
		"version":   item.Version,
		"handledAt": item.HandledAt,
	}
}

func scanReports(rows pgx.Rows) ([]report.Report, *domain.AppError) {
	items := []report.Report{}
	for rows.Next() {
		item, err := scanReportRow(rows)
		if err != nil {
			return nil, internalStoreError()
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, internalStoreError()
	}
	return items, nil
}

func scanReport(ctx context.Context, q queryer, sql string, args ...any) (report.Report, error) {
	return scanReportRow(q.QueryRow(ctx, sql, args...))
}

func scanReportRow(row scanner) (report.Report, error) {
	var item report.Report
	err := row.Scan(
		&item.ID,
		&item.ReporterUserID,
		&item.ReporterUsername,
		&item.ReporterName,
		&item.TargetType,
		&item.TargetID,
		&item.CanonicalTargetType,
		&item.CanonicalTargetID,
		&item.TargetLabel,
		&item.TargetSnapshotJSON,
		&item.ReportedUsername,
		&item.ReasonCode,
		&item.Title,
		&item.Description,
		&item.Status,
		&item.AdminReason,
		&item.HandledByAdminID,
		&item.HandledAt,
		&item.DisputeID,
		&item.OpenInfoRequestID,
		&item.InfoRequestedFromID,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.Version,
	)
	return item, err
}

func listAdminInfoSupplements(ctx context.Context, q rowQueryer, entityType, entityID string) ([]report.InfoSupplement, error) {
	rows, err := q.Query(ctx, `
		SELECT supplement.id::text,
		       supplement.info_request_id::text,
		       supplement.submitted_by_user_id::text,
		       submitter.username,
		       submitter.display_name,
		       supplement.body,
		       supplement.created_at
		FROM moderation_info_supplements supplement
		JOIN moderation_info_requests info_request ON info_request.id = supplement.info_request_id
		JOIN users submitter ON submitter.id = supplement.submitted_by_user_id
		WHERE info_request.entity_type = $1
		  AND (($1 = 'report' AND info_request.report_id = $2::uuid)
		    OR ($1 = 'dispute' AND info_request.dispute_case_id = $2::uuid))
		ORDER BY supplement.created_at ASC, supplement.id ASC
	`, entityType, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]report.InfoSupplement, 0)
	for rows.Next() {
		var item report.InfoSupplement
		if err := rows.Scan(
			&item.ID,
			&item.InfoRequestID,
			&item.SubmittedByUserID,
			&item.SubmittedByUsername,
			&item.SubmittedByName,
			&item.Body,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanDisputes(rows pgx.Rows) ([]report.DisputeCase, *domain.AppError) {
	items := []report.DisputeCase{}
	for rows.Next() {
		item, err := scanDisputeRow(rows)
		if err != nil {
			return nil, internalStoreError()
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, internalStoreError()
	}
	return items, nil
}

func scanDispute(ctx context.Context, q queryer, sql string, args ...any) (report.DisputeCase, error) {
	return scanDisputeRow(q.QueryRow(ctx, sql, args...))
}

func scanDisputeRow(row scanner) (report.DisputeCase, error) {
	var item report.DisputeCase
	err := row.Scan(
		&item.ID,
		&item.ReportID,
		&item.TargetType,
		&item.TargetID,
		&item.APIOrderID,
		&item.Active,
		&item.TargetLabel,
		&item.PrimaryUserID,
		&item.PrimaryUsername,
		&item.PrimaryDisplayName,
		&item.CounterpartyUserID,
		&item.CounterpartyUsername,
		&item.CounterpartyName,
		&item.SubjectUserID,
		&item.SubjectUsername,
		&item.SubjectName,
		&item.Status,
		&item.IssueCode,
		&item.RequestedResolution,
		&item.RequestedAmountCNY,
		&item.IssueOccurredAt,
		&item.PublicSummary,
		&item.PublicResultCode,
		&item.PublicResult,
		&item.AdminReason,
		&item.OpenedByAdminID,
		&item.OpenedAt,
		&item.ResolvedAt,
		&item.ClosedAt,
		&item.FinalReason,
		&item.AppealExpiresAt,
		&item.AdverselyAffectedIDs,
		&item.NegotiationChannels,
		&item.NegotiationEndedConfirmed,
		&item.NegotiationSummary,
		&item.RequestedPlatformAction,
		&item.EscalatedByUserID,
		&item.EscalatedAt,
		&item.NextActor,
		&item.DueAt,
		&item.FactSnapshotJSON,
		&item.ApplicantStatement,
		&item.RespondentResponse,
		&item.RespondedByUserID,
		&item.RespondedAt,
		&item.SellerDecision,
		&item.SellerDecisionReason,
		&item.SellerDecidedByUserID,
		&item.SellerDecidedAt,
		&item.SellerResponseLate,
		&item.ApplicantDecisionDueAt,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.Version,
		&item.OpenInfoRequestID,
		&item.InfoRequestedFromID,
	)
	return item, err
}

func scanAppeals(rows pgx.Rows) ([]report.Appeal, *domain.AppError) {
	items := []report.Appeal{}
	for rows.Next() {
		item, err := scanAppealRow(rows)
		if err != nil {
			return nil, internalStoreError()
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, internalStoreError()
	}
	return items, nil
}

func scanAppeal(ctx context.Context, q queryer, sql string, args ...any) (report.Appeal, error) {
	return scanAppealRow(q.QueryRow(ctx, sql, args...))
}

func scanAppealRow(row scanner) (report.Appeal, error) {
	var item report.Appeal
	err := row.Scan(
		&item.ID,
		&item.AppellantUserID,
		&item.AppellantUsername,
		&item.AppellantName,
		&item.ReportID,
		&item.DisputeID,
		&item.TargetType,
		&item.TargetID,
		&item.Title,
		&item.Statement,
		&item.Status,
		&item.AdminReason,
		&item.HandledByAdminID,
		&item.HandledAt,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.Version,
	)
	return item, err
}

func reportNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Report not found", "举报记录不存在。")
}

func disputeNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Dispute not found", "纠纷记录不存在。")
}

func appealNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Appeal not found", "申诉记录不存在。")
}

func appealSourceNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Appeal source not found", "关联举报或纠纷不存在。")
}

func publicProfileNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Profile not found", "公开主页不存在。")
}

func targetNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Report target not found", "举报目标不存在或不可见。")
}

func reportPermissionDenied() *domain.AppError {
	return domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "你没有权限举报该对象。")
}

func infoRequestPermissionDenied() *domain.AppError {
	return domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "只能指定该案件中的有效参与者补充信息。")
}

func infoRequestNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Information request not found", "补充请求不存在、已失效或不属于当前用户。")
}

func selfReportForbidden() *domain.AppError {
	return domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "不能举报自己。")
}

func activeReportExists() *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeActiveReportExists, "Active report exists", "你已对该对象提交过进行中的举报或人工介入申请。")
}

func reportInvalidState(detail string) *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid report state", detail)
}

func versionConflict() *domain.AppError {
	return domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
}

func canFinishReport(status string) bool {
	switch status {
	case report.ReportStatusSubmitted, report.ReportStatusTriaged, report.ReportStatusNeedsInfo:
		return true
	default:
		return false
	}
}

func canOpenDisputeFromReport(status string) bool {
	switch status {
	case report.ReportStatusSubmitted, report.ReportStatusTriaged, report.ReportStatusNeedsInfo:
		return true
	default:
		return false
	}
}

func nonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func commercialOutcomeForRemedy(action string) string {
	switch action {
	case apiorder.DisputeResolutionFullRefund:
		return apiorder.CommercialOutcomeFullRefund
	case apiorder.DisputeResolutionPartialRefund:
		return apiorder.CommercialOutcomePartialRefund
	case apiorder.DisputeResolutionContinueFulfillment:
		return apiorder.CommercialOutcomeContinuedFulfillment
	default:
		return apiorder.CommercialOutcomeClosedUnverified
	}
}

const reportSelectSQL = `
	SELECT ` + reportColumns + `
	FROM reports r
	JOIN users reporter ON reporter.id = r.reporter_user_id
	LEFT JOIN users reported ON reported.id = r.reported_user_id`

const reportColumns = `
	r.id::text,
	r.reporter_user_id::text,
	reporter.username,
	reporter.display_name,
	r.target_type,
	r.target_id,
	r.canonical_target_type,
	r.canonical_target_id,
	r.target_label,
	r.target_snapshot_json::text,
	COALESCE(NULLIF(r.reported_username, ''), reported.username, ''),
	r.reason_code,
	r.title,
	r.description,
	r.status,
	r.admin_reason,
	COALESCE(r.handled_by_admin_id::text, ''),
	r.handled_at,
	COALESCE(r.dispute_case_id::text, ''),
	COALESCE((SELECT info.id::text FROM moderation_info_requests info WHERE info.report_id = r.id AND info.status = 'open' LIMIT 1), ''),
	COALESCE((SELECT info.requested_from_user_id::text FROM moderation_info_requests info WHERE info.report_id = r.id AND info.status = 'open' LIMIT 1), ''),
	r.created_at,
	r.updated_at,
	r.version`

const reportReturningColumns = `
	reports.id::text,
	reports.reporter_user_id::text,
	(SELECT username FROM users WHERE users.id = reports.reporter_user_id),
	(SELECT display_name FROM users WHERE users.id = reports.reporter_user_id),
	reports.target_type,
	reports.target_id,
	reports.canonical_target_type,
	reports.canonical_target_id,
	reports.target_label,
	reports.target_snapshot_json::text,
	COALESCE(NULLIF(reports.reported_username, ''), (SELECT username FROM users WHERE users.id = reports.reported_user_id), ''),
	reports.reason_code,
	reports.title,
	reports.description,
	reports.status,
	reports.admin_reason,
	COALESCE(reports.handled_by_admin_id::text, ''),
	reports.handled_at,
	COALESCE(reports.dispute_case_id::text, ''),
	COALESCE((SELECT info.id::text FROM moderation_info_requests info WHERE info.report_id = reports.id AND info.status = 'open' LIMIT 1), ''),
	COALESCE((SELECT info.requested_from_user_id::text FROM moderation_info_requests info WHERE info.report_id = reports.id AND info.status = 'open' LIMIT 1), ''),
	reports.created_at,
	reports.updated_at,
	reports.version`

const disputeSelectSQL = `
	SELECT ` + disputeColumns + `
	FROM dispute_cases d
	JOIN users primary_user ON primary_user.id = d.primary_user_id
	LEFT JOIN users counterparty_user ON counterparty_user.id = d.counterparty_user_id
	LEFT JOIN users subject_user ON subject_user.id = d.subject_user_id`

const disputeColumns = `
	d.id::text,
	COALESCE(d.report_id::text, ''),
	d.target_type,
	d.target_id,
	COALESCE(d.api_order_id::text, ''),
	d.active,
	d.target_label,
	d.primary_user_id::text,
	primary_user.username,
	primary_user.display_name,
	COALESCE(d.counterparty_user_id::text, ''),
	COALESCE(counterparty_user.username, ''),
	COALESCE(counterparty_user.display_name, ''),
	COALESCE(d.subject_user_id::text, ''),
	COALESCE(subject_user.username, ''),
	COALESCE(subject_user.display_name, ''),
	d.status,
	d.issue_code,
	d.requested_resolution,
	COALESCE(d.requested_amount_cny::text, ''),
	d.issue_occurred_at,
	d.public_summary,
	d.public_result_code,
	d.public_result,
	d.admin_reason,
	d.opened_by_admin_id::text,
	d.opened_at,
	d.resolved_at,
	d.closed_at,
	d.final_reason,
	d.appeal_expires_at,
	d.adversely_affected_user_ids::text[],
	d.negotiation_channels::text[],
	d.negotiation_ended_confirmed,
	d.negotiation_summary,
	d.requested_platform_action,
	COALESCE(d.escalated_by_user_id::text, ''),
	d.escalated_at,
	d.next_actor,
	d.due_at,
	d.fact_snapshot::text,
	d.applicant_statement,
	d.respondent_response,
	COALESCE(d.responded_by_user_id::text, ''),
	d.responded_at,
	d.seller_decision,
	d.seller_decision_reason,
	COALESCE(d.seller_decided_by_user_id::text, ''),
	d.seller_decided_at,
	d.seller_response_late,
	d.applicant_decision_due_at,
	d.created_at,
	d.updated_at,
	d.version,
	COALESCE((SELECT info.id::text FROM moderation_info_requests info WHERE info.dispute_case_id = d.id AND info.status = 'open' LIMIT 1), ''),
	COALESCE((SELECT info.requested_from_user_id::text FROM moderation_info_requests info WHERE info.dispute_case_id = d.id AND info.status = 'open' LIMIT 1), '')`

const disputeReturningColumns = `
	dispute_cases.id::text,
	COALESCE(dispute_cases.report_id::text, ''),
	dispute_cases.target_type,
	dispute_cases.target_id,
	COALESCE(dispute_cases.api_order_id::text, ''),
	dispute_cases.active,
	dispute_cases.target_label,
	dispute_cases.primary_user_id::text,
	(SELECT username FROM users WHERE users.id = dispute_cases.primary_user_id),
	(SELECT display_name FROM users WHERE users.id = dispute_cases.primary_user_id),
	COALESCE(dispute_cases.counterparty_user_id::text, ''),
	COALESCE((SELECT username FROM users WHERE users.id = dispute_cases.counterparty_user_id), ''),
	COALESCE((SELECT display_name FROM users WHERE users.id = dispute_cases.counterparty_user_id), ''),
	COALESCE(dispute_cases.subject_user_id::text, ''),
	COALESCE((SELECT username FROM users WHERE users.id = dispute_cases.subject_user_id), ''),
	COALESCE((SELECT display_name FROM users WHERE users.id = dispute_cases.subject_user_id), ''),
	dispute_cases.status,
	dispute_cases.issue_code,
	dispute_cases.requested_resolution,
	COALESCE(dispute_cases.requested_amount_cny::text, ''),
	dispute_cases.issue_occurred_at,
	dispute_cases.public_summary,
	dispute_cases.public_result_code,
	dispute_cases.public_result,
	dispute_cases.admin_reason,
	dispute_cases.opened_by_admin_id::text,
	dispute_cases.opened_at,
	dispute_cases.resolved_at,
	dispute_cases.closed_at,
	dispute_cases.final_reason,
	dispute_cases.appeal_expires_at,
	dispute_cases.adversely_affected_user_ids::text[],
	dispute_cases.negotiation_channels::text[],
	dispute_cases.negotiation_ended_confirmed,
	dispute_cases.negotiation_summary,
	dispute_cases.requested_platform_action,
	COALESCE(dispute_cases.escalated_by_user_id::text, ''),
	dispute_cases.escalated_at,
	dispute_cases.next_actor,
	dispute_cases.due_at,
	dispute_cases.fact_snapshot::text,
	dispute_cases.applicant_statement,
	dispute_cases.respondent_response,
	COALESCE(dispute_cases.responded_by_user_id::text, ''),
	dispute_cases.responded_at,
	dispute_cases.seller_decision,
	dispute_cases.seller_decision_reason,
	COALESCE(dispute_cases.seller_decided_by_user_id::text, ''),
	dispute_cases.seller_decided_at,
	dispute_cases.seller_response_late,
	dispute_cases.applicant_decision_due_at,
	dispute_cases.created_at,
	dispute_cases.updated_at,
	dispute_cases.version,
	COALESCE((SELECT info.id::text FROM moderation_info_requests info WHERE info.dispute_case_id = dispute_cases.id AND info.status = 'open' LIMIT 1), ''),
	COALESCE((SELECT info.requested_from_user_id::text FROM moderation_info_requests info WHERE info.dispute_case_id = dispute_cases.id AND info.status = 'open' LIMIT 1), '')`

const appealSelectSQL = `
	SELECT ` + appealColumns + `
	FROM appeals a
	JOIN users appellant ON appellant.id = a.appellant_user_id`

const appealColumns = `
	a.id::text,
	a.appellant_user_id::text,
	appellant.username,
	appellant.display_name,
	COALESCE(a.report_id::text, ''),
	COALESCE(a.dispute_case_id::text, ''),
	a.target_type,
	a.target_id,
	a.title,
	a.statement,
	a.status,
	a.admin_reason,
	COALESCE(a.handled_by_admin_id::text, ''),
	a.handled_at,
	a.created_at,
	a.updated_at,
	a.version`

const appealReturningColumns = `
	appeals.id::text,
	appeals.appellant_user_id::text,
	(SELECT username FROM users WHERE users.id = appeals.appellant_user_id),
	(SELECT display_name FROM users WHERE users.id = appeals.appellant_user_id),
	COALESCE(appeals.report_id::text, ''),
	COALESCE(appeals.dispute_case_id::text, ''),
	appeals.target_type,
	appeals.target_id,
	appeals.title,
	appeals.statement,
	appeals.status,
	appeals.admin_reason,
	COALESCE(appeals.handled_by_admin_id::text, ''),
	appeals.handled_at,
	appeals.created_at,
	appeals.updated_at,
	appeals.version`
