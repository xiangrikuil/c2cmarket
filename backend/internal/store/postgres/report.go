package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apiorder"
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
	result, request, appErr := submitInfoSupplementInTx(ctx, tx, input, now)
	if appErr != nil {
		return report.MutationResult{}, idempotency.Completion{}, appErr
	}
	if appErr := insertInfoSupplementSideEffects(ctx, tx, request, input.RequestID, now); appErr != nil {
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
	return scanAppeals(rows)
}

func (s *Store) ListAdminAppeals(ctx context.Context) ([]report.Appeal, *domain.AppError) {
	rows, err := s.pool.Query(ctx, appealSelectSQL+` ORDER BY a.updated_at DESC`)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return scanAppeals(rows)
}

func (s *Store) GetAdminAppeal(ctx context.Context, id string) (report.Appeal, *domain.AppError) {
	item, err := scanAppeal(ctx, s.pool, appealSelectSQL+` WHERE a.id = $1`, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return report.Appeal{}, appealNotFound()
	}
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
	rows, err := s.pool.Query(ctx, disputeSelectSQL+` ORDER BY d.updated_at DESC`)
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
	case report.DisputeMessageActionAppend:
		if item.Status != report.DisputeStatusNegotiating && item.Status != report.DisputeStatusOpen && item.Status != report.DisputeStatusWaitingInfo {
			return participantDisputeInvalidState("当前纠纷状态不能继续留言。")
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO api_order_dispute_messages (id, dispute_case_id, sender_user_id, body, request_id, created_at)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, uuid.NewString(), item.ID, input.ActorUserID, strings.TrimSpace(input.Body), strings.TrimSpace(input.RequestID), now); err != nil {
			return internalStoreError()
		}
		if appErr := touchDisputeCaseInTx(ctx, tx, item, now); appErr != nil {
			return appErr
		}
		return insertDisputeEvent(ctx, tx, "dispute", item.ID, "message_appended", input.ActorUserID, "user", "", false, input.RequestID, now)

	case report.DisputeMessageActionPropose:
		if item.Status != report.DisputeStatusNegotiating || order.DisputeStatus != apiorder.DisputeStatusNegotiating {
			return participantDisputeInvalidState("平台已介入或纠纷已结束，不能再提交协商方案。")
		}
		if appErr := apiorder.ValidateRequestedDisputeAmount(input.Resolution, input.AmountCNY, order.Amount); appErr != nil {
			return appErr
		}
		if _, err := tx.Exec(ctx, `
			UPDATE api_order_dispute_settlement_proposals
			SET status = 'superseded', updated_at = $2, version = version + 1
			WHERE dispute_case_id = $1 AND status = 'pending'
		`, item.ID, now); err != nil {
			return internalStoreError()
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO api_order_dispute_settlement_proposals (
				id, dispute_case_id, proposed_by_user_id, resolution, amount_cny,
				terms, status, request_id, created_at, updated_at, version
			)
			VALUES ($1, $2, $3, $4, $5, $6, 'pending', $7, $8, $8, 1)
		`, uuid.NewString(), item.ID, input.ActorUserID, input.Resolution, nullNumeric(input.AmountCNY), strings.TrimSpace(input.Terms), strings.TrimSpace(input.RequestID), now); err != nil {
			return internalStoreError()
		}
		if appErr := touchDisputeCaseInTx(ctx, tx, item, now); appErr != nil {
			return appErr
		}
		return insertDisputeEvent(ctx, tx, "dispute", item.ID, "settlement_proposed", input.ActorUserID, "user", input.Terms, true, input.RequestID, now)

	case report.DisputeMessageActionConfirm, report.DisputeMessageActionReject:
		if item.Status != report.DisputeStatusNegotiating || order.DisputeStatus != apiorder.DisputeStatusNegotiating {
			return participantDisputeInvalidState("平台已介入或纠纷已结束，不能处理协商方案。")
		}
		proposal, appErr := lockSettlementProposalInTx(ctx, tx, item.ID, input.ProposalID)
		if appErr != nil {
			return appErr
		}
		if proposal.Status != report.SettlementStatusPending || proposal.ProposedByUserID == input.ActorUserID {
			return participantDisputeInvalidState("只能由另一方确认或拒绝当前待确认方案。")
		}
		if input.Action == report.DisputeMessageActionReject {
			if _, err := tx.Exec(ctx, `
				UPDATE api_order_dispute_settlement_proposals
				SET status = 'rejected', rejected_by_user_id = $2, rejected_at = $3, updated_at = $3, version = version + 1
				WHERE id = $1
			`, proposal.ID, input.ActorUserID, now); err != nil {
				return internalStoreError()
			}
			if appErr := touchDisputeCaseInTx(ctx, tx, item, now); appErr != nil {
				return appErr
			}
			return insertDisputeEvent(ctx, tx, "dispute", item.ID, "settlement_rejected", input.ActorUserID, "user", input.Reason, true, input.RequestID, now)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE api_order_dispute_settlement_proposals
			SET status = 'accepted', accepted_by_user_id = $2, accepted_at = $3, updated_at = $3, version = version + 1
			WHERE id = $1
		`, proposal.ID, input.ActorUserID, now); err != nil {
			return internalStoreError()
		}
		if err := tx.QueryRow(ctx, `
			UPDATE dispute_cases
			SET status = 'closed', public_result = '双方已确认协商方案', closed_at = $2,
			    updated_at = $2, version = version + 1
			WHERE id = $1
			RETURNING status, public_result, closed_at, updated_at, version
		`, item.ID, now).Scan(&item.Status, &item.PublicResult, &item.ClosedAt, &item.UpdatedAt, &item.Version); err != nil {
			return internalStoreError()
		}
		order.DisputeStatus = apiorder.DisputeStatusClosed
		order.UpdatedAt = now
		order.Version++
		if appErr := updateAPIOrderInTx(ctx, tx, *order); appErr != nil {
			return appErr
		}
		if appErr := insertAPIOrderEventInTx(ctx, tx, *order, input.ActorUserID, apiorder.EventDisputeClosed, order.Status, order.Status, "双方已确认协商方案", input.RequestID, now); appErr != nil {
			return appErr
		}
		return insertDisputeEvent(ctx, tx, "dispute", item.ID, "settlement_accepted", input.ActorUserID, "user", proposal.Terms, true, input.RequestID, now)

	case report.DisputeMessageActionEscalate:
		if item.Status != report.DisputeStatusNegotiating || order.DisputeStatus != apiorder.DisputeStatusNegotiating {
			return participantDisputeInvalidState("当前纠纷已由平台处理或已经结案。")
		}
		if _, err := tx.Exec(ctx, `
			UPDATE api_order_dispute_settlement_proposals
			SET status = 'superseded', updated_at = $2, version = version + 1
			WHERE dispute_case_id = $1 AND status = 'pending'
		`, item.ID, now); err != nil {
			return internalStoreError()
		}
		if err := tx.QueryRow(ctx, `
			UPDATE dispute_cases
			SET status = 'open', public_result = '平台审核中', updated_at = $2, version = version + 1
			WHERE id = $1
			RETURNING status, public_result, updated_at, version
		`, item.ID, now).Scan(&item.Status, &item.PublicResult, &item.UpdatedAt, &item.Version); err != nil {
			return internalStoreError()
		}
		order.DisputeStatus = apiorder.DisputeStatusOpen
		order.UpdatedAt = now
		order.Version++
		if appErr := updateAPIOrderInTx(ctx, tx, *order); appErr != nil {
			return appErr
		}
		if appErr := insertAPIOrderEventInTx(ctx, tx, *order, input.ActorUserID, apiorder.EventDisputeOpened, order.Status, order.Status, "已申请平台介入", input.RequestID, now); appErr != nil {
			return appErr
		}
		return insertDisputeEvent(ctx, tx, "dispute", item.ID, "escalated", input.ActorUserID, "user", input.Reason, true, input.RequestID, now)
	default:
		return participantDisputeInvalidState("纠纷协商动作不支持。")
	}
}

func touchDisputeCaseInTx(ctx context.Context, tx pgx.Tx, item *report.DisputeCase, now time.Time) *domain.AppError {
	if err := tx.QueryRow(ctx, `
		UPDATE dispute_cases
		SET updated_at = $2, version = version + 1
		WHERE id = $1
		RETURNING updated_at, version
	`, item.ID, now).Scan(&item.UpdatedAt, &item.Version); err != nil {
		return internalStoreError()
	}
	return nil
}

func participantDisputeInvalidState(detail string) *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", detail)
}

func lockSettlementProposalInTx(ctx context.Context, tx pgx.Tx, disputeID, proposalID string) (report.SettlementProposal, *domain.AppError) {
	var item report.SettlementProposal
	err := tx.QueryRow(ctx, `
		SELECT id::text, dispute_case_id::text, proposed_by_user_id::text, resolution,
		       COALESCE(amount_cny::text, ''), terms, status,
		       COALESCE(accepted_by_user_id::text, ''), accepted_at,
		       COALESCE(rejected_by_user_id::text, ''), rejected_at,
		       created_at, updated_at, version
		FROM api_order_dispute_settlement_proposals
		WHERE id = $1 AND dispute_case_id = $2
		FOR UPDATE
	`, proposalID, disputeID).Scan(
		&item.ID, &item.DisputeCaseID, &item.ProposedByUserID, &item.Resolution,
		&item.AmountCNY, &item.Terms, &item.Status, &item.AcceptedByUserID,
		&item.AcceptedAt, &item.RejectedByUserID, &item.RejectedAt,
		&item.CreatedAt, &item.UpdatedAt, &item.Version,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return report.SettlementProposal{}, disputeNotFound()
	}
	if err != nil {
		return report.SettlementProposal{}, internalStoreError()
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
	return nil
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
		       COALESCE(amount_cny::text, ''), terms, status,
		       COALESCE(accepted_by_user_id::text, ''), accepted_at,
		       COALESCE(rejected_by_user_id::text, ''), rejected_at,
		       created_at, updated_at, version
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
			&item.AmountCNY, &item.Terms, &item.Status, &item.AcceptedByUserID,
			&item.AcceptedAt, &item.RejectedByUserID, &item.RejectedAt,
			&item.CreatedAt, &item.UpdatedAt, &item.Version,
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
	publicEvent := input.Action == "resolve" || input.Action == "close"
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
		  AND requested_user.account_status = 'active'
		FOR UPDATE OF mir
	`, input.InfoRequestID, input.EntityType, input.EntityID, input.SubmittingUserID).Scan(
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
			UPDATE dispute_cases SET updated_at = $2, version = version + 1 WHERE id = $1
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
	source, appErr := report.ResolveAppealSource(input.AppellantUserID, sourceReport, sourceDispute)
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

	beforeOutcome, err := scanDisputeOutcome(tx.QueryRow(ctx, `
		SELECT `+disputeOutcomeReturningColumns+`
		FROM dispute_reputation_outcomes
		WHERE dispute_case_id = $1
		  AND status = 'active'
		FOR UPDATE
	`, disputeID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return internalStoreError()
	}
	if appErr := report.ValidateAppealOutcomeSubject(appeal, beforeOutcome.SubjectUserID); appErr != nil {
		return appErr
	}
	reversalReason := nonEmpty(input.Reason, "申诉批准，反转关联信誉裁定。")
	afterOutcome, err := scanDisputeOutcome(tx.QueryRow(ctx, `
		UPDATE dispute_reputation_outcomes
		SET status = 'reversed',
		    reversed_at = $2,
		    reversed_by_admin_id = $3,
		    reversal_appeal_id = $4,
		    reversal_reason = $5,
		    updated_at = $2,
		    version = version + 1
		WHERE id = $1
		RETURNING `+disputeOutcomeReturningColumns+`
	`, beforeOutcome.ID, now, input.AdminUserID, appeal.ID, reversalReason))
	if err != nil {
		return internalStoreError()
	}
	if appErr := insertReputationGovernanceEvent(ctx, tx, "outcome", afterOutcome.ID, "outcome_reversed", input.AdminUserID, beforeOutcome, afterOutcome, reversalReason, input.RequestID, now); appErr != nil {
		return appErr
	}

	rows, err := tx.Query(ctx, `
		SELECT `+userRestrictionColumns+`
		FROM user_restrictions
		WHERE source_dispute_outcome_id = $1
		  AND revoked_at IS NULL
		ORDER BY id
		FOR UPDATE
	`, afterOutcome.ID)
	if err != nil {
		return internalStoreError()
	}
	restrictions := []reputation.UserRestriction{}
	for rows.Next() {
		beforeRestriction, scanErr := scanUserRestriction(rows)
		if scanErr != nil {
			rows.Close()
			return internalStoreError()
		}
		restrictions = append(restrictions, beforeRestriction)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return internalStoreError()
	}
	rows.Close()

	for _, beforeRestriction := range restrictions {
		afterRestriction, err := scanUserRestriction(tx.QueryRow(ctx, `
			UPDATE user_restrictions
			SET revoked_at = $2,
			    revoked_by_admin_id = $3,
			    revocation_reason = $4,
			    updated_at = $2,
			    version = version + 1
			WHERE id = $1
			RETURNING `+userRestrictionReturningColumns+`
		`, beforeRestriction.ID, now, input.AdminUserID, reversalReason))
		if err != nil {
			return internalStoreError()
		}
		if appErr := insertReputationGovernanceEvent(ctx, tx, "restriction", afterRestriction.ID, "restriction_revoked", input.AdminUserID, beforeRestriction, afterRestriction, reversalReason, input.RequestID, now); appErr != nil {
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
	switch input.Action {
	case "request_info":
		if current.Status != report.DisputeStatusOpen {
			return report.MutationResult{}, reportInvalidState("只有打开中的纠纷可以要求补充信息。")
		}
		next = report.DisputeStatusWaitingInfo
	case "resolve":
		if current.Status != report.DisputeStatusOpen && current.Status != report.DisputeStatusWaitingInfo {
			return report.MutationResult{}, reportInvalidState("当前纠纷不能标记处理完成。")
		}
		next = report.DisputeStatusResolved
		resolvedAt = &now
	case "close":
		if current.Status == report.DisputeStatusClosed {
			return report.MutationResult{}, reportInvalidState("纠纷已关闭。")
		}
		next = report.DisputeStatusClosed
		closedAt = &now
	default:
		return report.MutationResult{}, reportInvalidState("纠纷处理动作不支持。")
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
		    updated_at = $9,
		    version = version + 1
		WHERE id = $1
		RETURNING `+disputeReturningColumns+`
	`, current.ID, next, nonEmpty(input.PublicSummary, current.PublicSummary), nonEmpty(input.PublicResultCode, current.PublicResultCode, report.PublicResultNoAction),
		nonEmpty(input.PublicResult, current.PublicResult), strings.TrimSpace(input.Reason), resolvedAt, closedAt, now)
	if err != nil {
		return report.MutationResult{}, internalStoreError()
	}
	if item.TargetType == report.TargetAPIOrder && (input.Action == "resolve" || input.Action == "close") {
		if appErr := closeAPIOrderDisputeProjectionInTx(ctx, tx, item, input, now); appErr != nil {
			return report.MutationResult{}, appErr
		}
	}
	if appErr := insertDisputeModerationAuditLog(ctx, tx, input, current, item, now); appErr != nil {
		return report.MutationResult{}, appErr
	}
	return report.MutationResult{Dispute: &item}, nil
}

func closeAPIOrderDisputeProjectionInTx(ctx context.Context, tx pgx.Tx, dispute report.DisputeCase, input report.AdminActionInput, now time.Time) *domain.AppError {
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
	if currentStatus == apiorder.DisputeStatusClosed {
		return nil
	}
	if !apiorder.IsDisputeActive(currentStatus) {
		return reportInvalidState("纠纷关联的 API 订单状态不一致，无法结案。")
	}
	commandTag, err := tx.Exec(ctx, `
		UPDATE api_orders
		SET dispute_status = $2,
		    updated_at = $3,
		    version = version + 1
		WHERE id = $1
		  AND dispute_status = $4
	`, orderID, apiorder.DisputeStatusClosed, now, currentStatus)
	if err != nil {
		return internalStoreError()
	}
	if commandTag.RowsAffected() != 1 {
		return versionConflict()
	}
	return insertAPIOrderEventInTx(ctx, tx, apiorder.Order{ID: orderID}, input.AdminUserID, apiorder.EventDisputeClosed, orderStatus, orderStatus, "纠纷已结案", input.RequestID, now)
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
			report_id, target_type, target_id, target_label, primary_user_id, counterparty_user_id,
			subject_user_id,
			status, public_summary, public_result_code, public_result, admin_reason, opened_by_admin_id, opened_at,
			created_at, updated_at, version
		)
		VALUES ($1, $2, $3, $4, $5, $6, $6, 'open', $7, $8, $9, $10, $11, $12, $12, $12, 1)
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
		targetURL = "/my/reports/dispute/" + request.EntityID
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
		&item.PublicSummary,
		&item.PublicResultCode,
		&item.PublicResult,
		&item.AdminReason,
		&item.OpenedByAdminID,
		&item.OpenedAt,
		&item.ResolvedAt,
		&item.ClosedAt,
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
	d.public_summary,
	d.public_result_code,
	d.public_result,
	d.admin_reason,
	d.opened_by_admin_id::text,
	d.opened_at,
	d.resolved_at,
	d.closed_at,
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
	dispute_cases.public_summary,
	dispute_cases.public_result_code,
	dispute_cases.public_result,
	dispute_cases.admin_reason,
	dispute_cases.opened_by_admin_id::text,
	dispute_cases.opened_at,
	dispute_cases.resolved_at,
	dispute_cases.closed_at,
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
