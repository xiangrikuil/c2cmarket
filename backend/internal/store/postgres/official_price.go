package postgres

import (
	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/officialprice"
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"net/http"
	"strings"
	"time"
)

func (s *Store) FindDuplicateOfficialPriceLeadID(ctx context.Context, fingerprint string) (string, *domain.AppError) {
	if s == nil || s.pool == nil {
		return "", internalStoreError()
	}
	var id string
	err := s.pool.QueryRow(ctx, `
		SELECT id::text
		FROM official_price_leads
		WHERE fingerprint = $1
		ORDER BY created_at ASC
		LIMIT 1
	`, fingerprint).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", internalStoreError()
	}
	return id, nil
}

func (s *Store) CreateOfficialPriceLead(ctx context.Context, lead officialprice.Lead) *domain.AppError {
	if s == nil || s.pool == nil {
		return internalStoreError()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO official_price_leads (
			id, submitter_user_id, product_plan_id, product_text, plan_text, region_code, channel,
			opening_method, source_url, source_title, evidence_summary, note, status, observed_at,
			billing_period, commitment_months, price_unit, seat_count, quantity, currency,
			original_amount, original_price_text, tax_included, fingerprint, duplicate_of_lead_id,
			created_at, updated_at, version
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12, $13, $14,
			$15, $16, $17, $18, $19, $20,
			$21, $22, $23, $24, $25,
			$26, $27, $28
		)
	`, lead.ID, lead.SubmitterUserID, nullUUID(lead.ProductPlanID), lead.ProductText, nullText(lead.PlanText), lead.RegionCode, lead.Channel,
		lead.OpeningMethod, lead.SourceURL, nullText(lead.SourceTitle), lead.EvidenceSummary, nullText(lead.Note), lead.Status, lead.ObservedAt,
		lead.BillingPeriod, lead.CommitmentMonths, lead.PriceUnit, lead.SeatCount, lead.Quantity, lead.Currency,
		lead.OriginalAmount, lead.OriginalPriceText, lead.TaxIncluded, lead.Fingerprint, nullUUID(lead.DuplicateOfLeadID),
		lead.CreatedAt, lead.UpdatedAt, lead.Version)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func (s *Store) GetOfficialPriceLead(ctx context.Context, leadID string) (officialprice.Lead, *domain.AppError) {
	if s == nil || s.pool == nil {
		return officialprice.Lead{}, internalStoreError()
	}
	lead, err := s.getOfficialPriceLead(ctx, s.pool, leadID, false)
	if errors.Is(err, pgx.ErrNoRows) {
		return officialprice.Lead{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Lead not found", "低价线索不存在。")
	}
	if err != nil {
		return officialprice.Lead{}, internalStoreError()
	}
	return lead, nil
}

func (s *Store) ListOfficialPriceLeadsBySubmitter(ctx context.Context, submitterUserID string) ([]officialprice.Lead, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+officialPriceLeadColumns+`
		FROM official_price_leads
		WHERE submitter_user_id = $1
		ORDER BY created_at DESC
	`, submitterUserID)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return scanOfficialPriceLeads(rows)
}

func (s *Store) ListOfficialPriceLeads(ctx context.Context) ([]officialprice.Lead, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+officialPriceLeadColumns+`
		FROM official_price_leads
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return scanOfficialPriceLeads(rows)
}

func (s *Store) ApproveOfficialPriceLead(ctx context.Context, input officialprice.ApproveLeadInput, normalizedMonthlyCNY, offerKey string, now time.Time) (officialprice.Lead, officialprice.Record, *domain.AppError) {
	if s == nil || s.pool == nil {
		return officialprice.Lead{}, officialprice.Record{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return officialprice.Lead{}, officialprice.Record{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	lead, record, appErr := s.approveOfficialPriceLeadInTx(ctx, tx, input, normalizedMonthlyCNY, offerKey, now)
	if appErr != nil {
		return officialprice.Lead{}, officialprice.Record{}, appErr
	}

	if err := tx.Commit(ctx); err != nil {
		return officialprice.Lead{}, officialprice.Record{}, internalStoreError()
	}
	return lead, record, nil
}

func (s *Store) ApproveOfficialPriceLeadWithIdempotency(ctx context.Context, entry idempotency.Entry, input officialprice.ApproveLeadInput, normalizedMonthlyCNY, offerKey string, now time.Time, buildCompletion officialprice.ApprovalCompletionBuilder) (officialprice.Lead, officialprice.Record, idempotency.Completion, *domain.AppError) {
	if s == nil || s.pool == nil {
		return officialprice.Lead{}, officialprice.Record{}, idempotency.Completion{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return officialprice.Lead{}, officialprice.Record{}, idempotency.Completion{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	existing, appErr := lockProcessingIdempotencyInTx(ctx, tx, entry)
	if appErr != nil {
		return officialprice.Lead{}, officialprice.Record{}, idempotency.Completion{}, appErr
	}

	lead, record, appErr := s.approveOfficialPriceLeadInTx(ctx, tx, input, normalizedMonthlyCNY, offerKey, now)
	if appErr != nil {
		return officialprice.Lead{}, officialprice.Record{}, idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(lead, record)
	if appErr != nil {
		return officialprice.Lead{}, officialprice.Record{}, idempotency.Completion{}, appErr
	}
	if appErr := completeIdempotencyInTx(ctx, tx, existing, completion, now); appErr != nil {
		return officialprice.Lead{}, officialprice.Record{}, idempotency.Completion{}, appErr
	}

	if err := tx.Commit(ctx); err != nil {
		return officialprice.Lead{}, officialprice.Record{}, idempotency.Completion{}, internalStoreError()
	}
	return lead, record, completion, nil
}

func (s *Store) approveOfficialPriceLeadInTx(ctx context.Context, tx pgx.Tx, input officialprice.ApproveLeadInput, normalizedMonthlyCNY, offerKey string, now time.Time) (officialprice.Lead, officialprice.Record, *domain.AppError) {
	lead, err := s.getOfficialPriceLead(ctx, tx, input.LeadID, true)
	if errors.Is(err, pgx.ErrNoRows) {
		return officialprice.Lead{}, officialprice.Record{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Lead not found", "低价线索不存在。")
	}
	if err != nil {
		return officialprice.Lead{}, officialprice.Record{}, internalStoreError()
	}
	if input.ExpectedVersion > 0 && lead.Version != input.ExpectedVersion {
		return officialprice.Lead{}, officialprice.Record{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if lead.Status != officialprice.LeadStatusPending && lead.Status != officialprice.LeadStatusChangesRequested {
		return officialprice.Lead{}, officialprice.Record{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前线索状态不能标记通过。")
	}
	beforeJSON, err := json.Marshal(lead)
	if err != nil {
		return officialprice.Lead{}, officialprice.Record{}, internalStoreError()
	}

	var existingRecordID string
	err = tx.QueryRow(ctx, `SELECT id::text FROM official_price_records WHERE lead_id = $1`, lead.ID).Scan(&existingRecordID)
	if err == nil {
		return officialprice.Lead{}, officialprice.Record{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前线索已有价格记录，不能重复标记通过。")
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return officialprice.Lead{}, officialprice.Record{}, internalStoreError()
	}

	_, err = tx.Exec(ctx, `
		UPDATE official_price_records
		SET status = 'superseded', valid_to = $2, version = version + 1
		WHERE offer_key = $1 AND status = 'active'
	`, offerKey, now)
	if err != nil {
		return officialprice.Lead{}, officialprice.Record{}, internalStoreError()
	}

	lead.ProductPlanID = input.ResolvedProductPlanID
	lead.Status = officialprice.LeadStatusApproved
	lead.ReviewedByAdminID = input.AdminUserID
	lead.ReviewedAt = &now
	lead.ReviewReason = strings.TrimSpace(input.Reason)
	lead.CommitmentMonths = nil
	lead.PriceUnit = "per_account"
	lead.SeatCount = nil
	lead.Quantity = 1
	lead.NormalizedMonthlyCNY = normalizedMonthlyCNY
	lead.FXRate = input.FXRateToCNY
	lead.FXSource = strings.TrimSpace(input.FXSource)
	lead.FXObservedAt = &input.FXObservedAt
	lead.ConversionMode = "monthly_normalized"
	lead.RoundingRule = "round_half_up_2"
	lead.OfferKey = offerKey
	lead.UpdatedAt = now
	lead.Version++

	_, err = tx.Exec(ctx, `
		UPDATE official_price_leads
		SET product_plan_id = $2,
		    status = $3,
		    reviewed_by_admin_id = $4,
		    reviewed_at = $5,
		    review_reason = $6,
		    normalized_monthly_cny = $7,
		    fx_rate = $8,
		    fx_source = $9,
		    fx_observed_at = $10,
		    conversion_mode = $11,
		    rounding_rule = $12,
		    offer_key = $13,
		    commitment_months = $14,
		    price_unit = $15,
		    seat_count = $16,
		    quantity = $17,
		    updated_at = $18,
		    version = $19
		WHERE id = $1
	`, lead.ID, lead.ProductPlanID, lead.Status, lead.ReviewedByAdminID, lead.ReviewedAt, lead.ReviewReason,
		lead.NormalizedMonthlyCNY, lead.FXRate, lead.FXSource, lead.FXObservedAt, lead.ConversionMode,
		lead.RoundingRule, lead.OfferKey, lead.CommitmentMonths, lead.PriceUnit, lead.SeatCount, lead.Quantity,
		lead.UpdatedAt, lead.Version)
	if err != nil {
		return officialprice.Lead{}, officialprice.Record{}, internalStoreError()
	}

	record := officialprice.Record{
		ID:                   uuid.NewString(),
		LeadID:               lead.ID,
		ProductPlanID:        input.ResolvedProductPlanID,
		RegionCode:           lead.RegionCode,
		Channel:              lead.Channel,
		OpeningMethod:        lead.OpeningMethod,
		SourceURL:            lead.SourceURL,
		ApprovedByAdminID:    input.AdminUserID,
		ApprovedAt:           now,
		ValidFrom:            input.ValidFrom,
		Status:               officialprice.RecordStatusActive,
		ObservedAt:           lead.ObservedAt,
		BillingPeriod:        lead.BillingPeriod,
		CommitmentMonths:     lead.CommitmentMonths,
		PriceUnit:            lead.PriceUnit,
		SeatCount:            nil,
		Quantity:             1,
		Currency:             lead.Currency,
		OriginalAmount:       lead.OriginalAmount,
		TaxIncluded:          lead.TaxIncluded,
		NormalizedMonthlyCNY: normalizedMonthlyCNY,
		FXRate:               input.FXRateToCNY,
		FXSource:             strings.TrimSpace(input.FXSource),
		FXObservedAt:         input.FXObservedAt,
		ConversionMode:       "monthly_normalized",
		RoundingRule:         "round_half_up_2",
		Fingerprint:          lead.Fingerprint,
		OfferKey:             offerKey,
		CreatedAt:            now,
		Version:              1,
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO official_price_records (
			id, lead_id, product_plan_id, region_code, channel, opening_method, source_url,
			approved_by_admin_id, approved_at, valid_from, status, observed_at, billing_period,
			commitment_months, price_unit, seat_count, quantity, currency, original_amount,
			tax_included, normalized_monthly_cny, fx_rate, fx_source, fx_observed_at,
			conversion_mode, rounding_rule, fingerprint, offer_key, created_at, version
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12, $13,
			$14, $15, $16, $17, $18, $19,
			$20, $21, $22, $23, $24,
			$25, $26, $27, $28, $29, $30
		)
	`, record.ID, record.LeadID, record.ProductPlanID, record.RegionCode, record.Channel, record.OpeningMethod, record.SourceURL,
		record.ApprovedByAdminID, record.ApprovedAt, record.ValidFrom, record.Status, record.ObservedAt, record.BillingPeriod,
		record.CommitmentMonths, record.PriceUnit, record.SeatCount, record.Quantity, record.Currency, record.OriginalAmount,
		record.TaxIncluded, record.NormalizedMonthlyCNY, record.FXRate, record.FXSource, record.FXObservedAt,
		record.ConversionMode, record.RoundingRule, record.Fingerprint, record.OfferKey, record.CreatedAt, record.Version)
	if err != nil {
		if isUniqueViolation(err) {
			return officialprice.Lead{}, officialprice.Record{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前线索已有价格记录或活跃价格冲突。")
		}
		return officialprice.Lead{}, officialprice.Record{}, internalStoreError()
	}

	afterJSON, err := json.Marshal(lead)
	if err != nil {
		return officialprice.Lead{}, officialprice.Record{}, internalStoreError()
	}
	eventID := uuid.NewString()
	requestID := strings.TrimSpace(input.RequestID)
	if requestID == "" {
		requestID = "unknown"
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO domain_events (
			id, aggregate_type, aggregate_id, event_type, actor_user_id, actor_kind,
			aggregate_version, request_id, metadata_json, created_at
		)
		VALUES (
			$1, 'official_price_lead', $2, 'official_price_lead.approved', $3, 'admin',
			$4, $5, $6, $7
		)
	`, eventID, lead.ID, input.AdminUserID, lead.Version, requestID, json.RawMessage(`{"recordCreated":true}`), now)
	if err != nil {
		return officialprice.Lead{}, officialprice.Record{}, internalStoreError()
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO admin_audit_logs (
			admin_user_id, action, target_type, target_id, reason, before_json,
			after_json, request_id, created_at
		)
		VALUES ($1, 'official_price_lead.approve', 'official_price_lead', $2, $3, $4, $5, $6, $7)
	`, input.AdminUserID, lead.ID, lead.ReviewReason, beforeJSON, afterJSON, requestID, now)
	if err != nil {
		return officialprice.Lead{}, officialprice.Record{}, internalStoreError()
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO notifications (
			user_id, type, title, body, target_type, target_id, target_url,
			source_event_type, source_event_id, dedupe_key, created_at
		)
		VALUES (
			$1, 'official_price_lead_approved', '低价线索已通过',
			'你提交的低价线索已审核通过，并生成公开价格记录。',
			'official_price_lead', $2, $3,
			'official_price_lead.approved', $4, $5, $6
		)
		ON CONFLICT (user_id, dedupe_key) WHERE dedupe_key IS NOT NULL DO NOTHING
	`, lead.SubmitterUserID, lead.ID, "/official-prices/"+record.ID, eventID, "official_price_lead:"+lead.ID+":approved", now)
	if err != nil {
		return officialprice.Lead{}, officialprice.Record{}, internalStoreError()
	}

	return lead, record, nil
}

func (s *Store) UpdateLeadReviewStatus(ctx context.Context, user auth.User, leadID, status, reason string, ifMatchVersion int64, now time.Time) (officialprice.Lead, *domain.AppError) {
	if !user.IsAdmin {
		return officialprice.Lead{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return officialprice.Lead{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	lead, err := s.getOfficialPriceLead(ctx, tx, leadID, true)
	if errors.Is(err, pgx.ErrNoRows) {
		return officialprice.Lead{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Lead not found", "低价线索不存在。")
	}
	if err != nil {
		return officialprice.Lead{}, internalStoreError()
	}
	if ifMatchVersion > 0 && lead.Version != ifMatchVersion {
		return officialprice.Lead{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if !canUpdateLeadReviewStatus(lead.Status, status) {
		return officialprice.Lead{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前线索状态不能执行该审核动作。")
	}
	lead.Status = status
	lead.ReviewedByAdminID = user.ID
	lead.ReviewedAt = &now
	lead.ReviewReason = strings.TrimSpace(reason)
	lead.UpdatedAt = now
	lead.Version++

	_, err = tx.Exec(ctx, `
		UPDATE official_price_leads
		SET status = $2,
		    reviewed_by_admin_id = $3,
		    reviewed_at = $4,
		    review_reason = $5,
		    updated_at = $6,
		    version = $7
		WHERE id = $1
	`, lead.ID, lead.Status, lead.ReviewedByAdminID, lead.ReviewedAt, lead.ReviewReason, lead.UpdatedAt, lead.Version)
	if err != nil {
		return officialprice.Lead{}, internalStoreError()
	}
	if err := tx.Commit(ctx); err != nil {
		return officialprice.Lead{}, internalStoreError()
	}
	return lead, nil
}

func (s *Store) ListOfficialPriceRecords(ctx context.Context) ([]officialprice.Record, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+officialPriceRecordColumns+`
		FROM official_price_records
		WHERE status = 'active'
		ORDER BY normalized_monthly_cny ASC, valid_from DESC, id ASC
	`)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return scanOfficialPriceRecords(rows)
}

func (s *Store) GetOfficialPriceRecord(ctx context.Context, recordID string) (officialprice.Record, *domain.AppError) {
	if s == nil || s.pool == nil {
		return officialprice.Record{}, internalStoreError()
	}
	var record officialprice.Record
	err := scanOfficialPriceRecord(s.pool.QueryRow(ctx, `
		SELECT `+officialPriceRecordColumns+`
		FROM official_price_records
		WHERE id = $1 AND status = 'active'
	`, recordID), &record)
	if errors.Is(err, pgx.ErrNoRows) {
		return officialprice.Record{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Price record not found", "价格记录不存在。")
	}
	if err != nil {
		return officialprice.Record{}, internalStoreError()
	}
	return record, nil
}

func (s *Store) ListAdminOfficialPriceRecords(ctx context.Context) ([]officialprice.Record, *domain.AppError) {
	if s == nil || s.pool == nil {
		return nil, internalStoreError()
	}
	rows, err := s.pool.Query(ctx, `
		SELECT `+officialPriceRecordColumns+`
		FROM official_price_records
		ORDER BY created_at DESC, id ASC
	`)
	if err != nil {
		return nil, internalStoreError()
	}
	defer rows.Close()
	return scanOfficialPriceRecords(rows)
}

func (s *Store) GetAdminOfficialPriceRecord(ctx context.Context, recordID string) (officialprice.Record, *domain.AppError) {
	if s == nil || s.pool == nil {
		return officialprice.Record{}, internalStoreError()
	}
	record, err := s.getOfficialPriceRecord(ctx, s.pool, recordID, false)
	if errors.Is(err, pgx.ErrNoRows) {
		return officialprice.Record{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Price record not found", "价格记录不存在。")
	}
	if err != nil {
		return officialprice.Record{}, internalStoreError()
	}
	return record, nil
}

func (s *Store) CreateAdminOfficialPriceRecord(ctx context.Context, input officialprice.AdminRecordInput, normalizedMonthlyCNY, offerKey, fingerprint string, now time.Time) (officialprice.Record, *domain.AppError) {
	if s == nil || s.pool == nil {
		return officialprice.Record{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return officialprice.Record{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	_, err = tx.Exec(ctx, `
		UPDATE official_price_records
		SET status = 'superseded', valid_to = $2, version = version + 1
		WHERE offer_key = $1 AND status = 'active'
	`, offerKey, now)
	if err != nil {
		return officialprice.Record{}, internalStoreError()
	}

	lead, record, appErr := s.insertAdminOfficialPriceRecordInTx(ctx, tx, input, normalizedMonthlyCNY, offerKey, fingerprint, now)
	if appErr != nil {
		return officialprice.Record{}, appErr
	}
	if appErr := s.writeOfficialPriceRecordAuditInTx(ctx, tx, input.AdminUserID, "official_price_record.created", "official_price_record.create", input.Reason, input.RequestID, record.ID, record.Version, nil, record, map[string]string{"leadId": lead.ID}, now); appErr != nil {
		return officialprice.Record{}, appErr
	}

	if err := tx.Commit(ctx); err != nil {
		return officialprice.Record{}, internalStoreError()
	}
	return record, nil
}

func (s *Store) UpdateAdminOfficialPriceRecord(ctx context.Context, input officialprice.AdminRecordInput, normalizedMonthlyCNY, offerKey, fingerprint string, now time.Time) (officialprice.Record, *domain.AppError) {
	if s == nil || s.pool == nil {
		return officialprice.Record{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return officialprice.Record{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	oldRecord, err := s.getOfficialPriceRecord(ctx, tx, input.RecordID, true)
	if errors.Is(err, pgx.ErrNoRows) {
		return officialprice.Record{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Price record not found", "价格记录不存在。")
	}
	if err != nil {
		return officialprice.Record{}, internalStoreError()
	}
	if oldRecord.Status != officialprice.RecordStatusActive {
		return officialprice.Record{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "只有生效中的价格记录可以编辑。")
	}
	if input.ExpectedVersion > 0 && oldRecord.Version != input.ExpectedVersion {
		return officialprice.Record{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	beforeRecord := oldRecord
	oldRecord.Status = officialprice.RecordStatusSuperseded
	validTo := now
	oldRecord.ValidTo = &validTo
	oldRecord.Version++
	_, err = tx.Exec(ctx, `
		UPDATE official_price_records
		SET status = $2, valid_to = $3, version = $4
		WHERE id = $1
	`, oldRecord.ID, oldRecord.Status, oldRecord.ValidTo, oldRecord.Version)
	if err != nil {
		return officialprice.Record{}, internalStoreError()
	}

	_, err = tx.Exec(ctx, `
		UPDATE official_price_records
		SET status = 'superseded', valid_to = $2, version = version + 1
		WHERE offer_key = $1 AND status = 'active' AND id <> $3
	`, offerKey, now, oldRecord.ID)
	if err != nil {
		return officialprice.Record{}, internalStoreError()
	}

	lead, record, appErr := s.insertAdminOfficialPriceRecordInTx(ctx, tx, input, normalizedMonthlyCNY, offerKey, fingerprint, now)
	if appErr != nil {
		return officialprice.Record{}, appErr
	}
	if appErr := s.writeOfficialPriceRecordAuditInTx(ctx, tx, input.AdminUserID, "official_price_record.updated", "official_price_record.update", input.Reason, input.RequestID, record.ID, record.Version, beforeRecord, record, map[string]string{"leadId": lead.ID, "supersededRecordId": oldRecord.ID}, now); appErr != nil {
		return officialprice.Record{}, appErr
	}

	if err := tx.Commit(ctx); err != nil {
		return officialprice.Record{}, internalStoreError()
	}
	return record, nil
}

func (s *Store) TakeDownOfficialPriceRecord(ctx context.Context, input officialprice.AdminRecordActionInput, now time.Time) (officialprice.Record, *domain.AppError) {
	if s == nil || s.pool == nil {
		return officialprice.Record{}, internalStoreError()
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return officialprice.Record{}, internalStoreError()
	}
	defer rollback(ctx, tx)

	record, err := s.getOfficialPriceRecord(ctx, tx, input.RecordID, true)
	if errors.Is(err, pgx.ErrNoRows) {
		return officialprice.Record{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Price record not found", "价格记录不存在。")
	}
	if err != nil {
		return officialprice.Record{}, internalStoreError()
	}
	if record.Status != officialprice.RecordStatusActive {
		return officialprice.Record{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "只有生效中的价格记录可以下架。")
	}
	if input.ExpectedVersion > 0 && record.Version != input.ExpectedVersion {
		return officialprice.Record{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	beforeRecord := record
	validTo := now
	record.Status = officialprice.RecordStatusTakenDown
	record.ValidTo = &validTo
	record.Version++
	_, err = tx.Exec(ctx, `
		UPDATE official_price_records
		SET status = $2, valid_to = $3, version = $4
		WHERE id = $1
	`, record.ID, record.Status, record.ValidTo, record.Version)
	if err != nil {
		return officialprice.Record{}, internalStoreError()
	}
	if appErr := s.writeOfficialPriceRecordAuditInTx(ctx, tx, input.AdminUserID, "official_price_record.taken_down", "official_price_record.take_down", input.Reason, input.RequestID, record.ID, record.Version, beforeRecord, record, nil, now); appErr != nil {
		return officialprice.Record{}, appErr
	}

	if err := tx.Commit(ctx); err != nil {
		return officialprice.Record{}, internalStoreError()
	}
	return record, nil
}

const officialPriceLeadColumns = `
	id::text, submitter_user_id::text, COALESCE(product_plan_id::text, ''), product_text,
	COALESCE(plan_text, ''), region_code, channel, opening_method, source_url,
	COALESCE(source_title, ''), evidence_summary, COALESCE(note, ''), status,
	COALESCE(reviewed_by_admin_id::text, ''), reviewed_at, COALESCE(review_reason, ''),
	observed_at, billing_period, commitment_months, price_unit, seat_count, quantity,
	currency::text, original_amount::text, original_price_text, tax_included,
	COALESCE(normalized_monthly_cny::text, ''), COALESCE(fx_rate::text, ''),
	COALESCE(fx_source, ''), fx_observed_at, COALESCE(conversion_mode, ''),
	COALESCE(rounding_rule, ''), COALESCE(fingerprint, ''), COALESCE(offer_key, ''),
	COALESCE(duplicate_of_lead_id::text, ''), created_at, updated_at, version
`

const officialPriceRecordColumns = `
	id::text, lead_id::text, product_plan_id::text, region_code, channel, opening_method,
	source_url, approved_by_admin_id::text, approved_at, valid_from, valid_to, status,
	observed_at, billing_period, commitment_months, price_unit, seat_count, quantity,
	currency::text, original_amount::text, tax_included, normalized_monthly_cny::text,
	fx_rate::text, fx_source, fx_observed_at, conversion_mode, rounding_rule, fingerprint,
	offer_key, created_at, version
`

func (s *Store) getOfficialPriceLead(ctx context.Context, q queryer, leadID string, forUpdate bool) (officialprice.Lead, error) {
	query := `SELECT ` + officialPriceLeadColumns + ` FROM official_price_leads WHERE id = $1`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	var lead officialprice.Lead
	err := scanOfficialPriceLead(q.QueryRow(ctx, query, leadID), &lead)
	return lead, err
}

func (s *Store) getOfficialPriceRecord(ctx context.Context, q queryer, recordID string, forUpdate bool) (officialprice.Record, error) {
	query := `SELECT ` + officialPriceRecordColumns + ` FROM official_price_records WHERE id = $1`
	if forUpdate {
		query += ` FOR UPDATE`
	}
	var record officialprice.Record
	err := scanOfficialPriceRecord(q.QueryRow(ctx, query, recordID), &record)
	return record, err
}

func (s *Store) insertAdminOfficialPriceRecordInTx(ctx context.Context, tx pgx.Tx, input officialprice.AdminRecordInput, normalizedMonthlyCNY, offerKey, fingerprint string, now time.Time) (officialprice.Lead, officialprice.Record, *domain.AppError) {
	fxObservedAt := input.FXObservedAt
	reviewedAt := now
	lead := officialprice.Lead{
		ID:                   uuid.NewString(),
		SubmitterUserID:      input.AdminUserID,
		ProductPlanID:        input.ProductPlanID,
		ProductText:          input.ProductText,
		PlanText:             input.PlanText,
		RegionCode:           input.RegionCode,
		Channel:              input.Channel,
		OpeningMethod:        input.OpeningMethod,
		SourceURL:            input.SourceURL,
		SourceTitle:          "管理员维护官网价格",
		EvidenceSummary:      input.Reason,
		Status:               officialprice.LeadStatusApproved,
		ReviewedByAdminID:    input.AdminUserID,
		ReviewedAt:           &reviewedAt,
		ReviewReason:         input.Reason,
		ObservedAt:           input.ObservedAt,
		BillingPeriod:        input.BillingPeriod,
		CommitmentMonths:     nil,
		PriceUnit:            "per_account",
		SeatCount:            nil,
		Quantity:             1,
		Currency:             input.Currency,
		OriginalAmount:       input.OriginalAmount,
		OriginalPriceText:    officialPriceOriginalText(input),
		TaxIncluded:          input.TaxIncluded,
		NormalizedMonthlyCNY: normalizedMonthlyCNY,
		FXRate:               input.FXRateToCNY,
		FXSource:             input.FXSource,
		FXObservedAt:         &fxObservedAt,
		ConversionMode:       "monthly_normalized",
		RoundingRule:         "round_half_up_2",
		Fingerprint:          fingerprint,
		OfferKey:             offerKey,
		CreatedAt:            now,
		UpdatedAt:            now,
		Version:              1,
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO official_price_leads (
			id, submitter_user_id, product_plan_id, product_text, plan_text, region_code, channel,
			opening_method, source_url, source_title, evidence_summary, note, status,
			reviewed_by_admin_id, reviewed_at, review_reason, observed_at, billing_period,
			commitment_months, price_unit, seat_count, quantity, currency, original_amount,
			original_price_text, tax_included, normalized_monthly_cny, fx_rate, fx_source,
			fx_observed_at, conversion_mode, rounding_rule, fingerprint, offer_key,
			created_at, updated_at, version
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12, $13,
			$14, $15, $16, $17, $18,
			$19, $20, $21, $22, $23, $24,
			$25, $26, $27, $28, $29,
			$30, $31, $32, $33, $34,
			$35, $36, $37
		)
	`, lead.ID, lead.SubmitterUserID, nullUUID(lead.ProductPlanID), lead.ProductText, nullText(lead.PlanText), lead.RegionCode, lead.Channel,
		lead.OpeningMethod, lead.SourceURL, nullText(lead.SourceTitle), lead.EvidenceSummary, nullText(lead.Note), lead.Status,
		lead.ReviewedByAdminID, lead.ReviewedAt, lead.ReviewReason, lead.ObservedAt, lead.BillingPeriod,
		lead.CommitmentMonths, lead.PriceUnit, lead.SeatCount, lead.Quantity, lead.Currency, lead.OriginalAmount,
		lead.OriginalPriceText, lead.TaxIncluded, lead.NormalizedMonthlyCNY, lead.FXRate, lead.FXSource,
		lead.FXObservedAt, lead.ConversionMode, lead.RoundingRule, lead.Fingerprint, lead.OfferKey,
		lead.CreatedAt, lead.UpdatedAt, lead.Version)
	if err != nil {
		return officialprice.Lead{}, officialprice.Record{}, internalStoreError()
	}

	record := officialprice.Record{
		ID:                   uuid.NewString(),
		LeadID:               lead.ID,
		ProductPlanID:        input.ProductPlanID,
		RegionCode:           input.RegionCode,
		Channel:              input.Channel,
		OpeningMethod:        input.OpeningMethod,
		SourceURL:            input.SourceURL,
		ApprovedByAdminID:    input.AdminUserID,
		ApprovedAt:           now,
		ValidFrom:            input.ValidFrom,
		Status:               officialprice.RecordStatusActive,
		ObservedAt:           input.ObservedAt,
		BillingPeriod:        input.BillingPeriod,
		CommitmentMonths:     nil,
		PriceUnit:            "per_account",
		SeatCount:            nil,
		Quantity:             1,
		Currency:             input.Currency,
		OriginalAmount:       input.OriginalAmount,
		TaxIncluded:          input.TaxIncluded,
		NormalizedMonthlyCNY: normalizedMonthlyCNY,
		FXRate:               input.FXRateToCNY,
		FXSource:             input.FXSource,
		FXObservedAt:         input.FXObservedAt,
		ConversionMode:       "monthly_normalized",
		RoundingRule:         "round_half_up_2",
		Fingerprint:          fingerprint,
		OfferKey:             offerKey,
		CreatedAt:            now,
		Version:              1,
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO official_price_records (
			id, lead_id, product_plan_id, region_code, channel, opening_method, source_url,
			approved_by_admin_id, approved_at, valid_from, status, observed_at, billing_period,
			commitment_months, price_unit, seat_count, quantity, currency, original_amount,
			tax_included, normalized_monthly_cny, fx_rate, fx_source, fx_observed_at,
			conversion_mode, rounding_rule, fingerprint, offer_key, created_at, version
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12, $13,
			$14, $15, $16, $17, $18, $19,
			$20, $21, $22, $23, $24,
			$25, $26, $27, $28, $29, $30
		)
	`, record.ID, record.LeadID, record.ProductPlanID, record.RegionCode, record.Channel, record.OpeningMethod, record.SourceURL,
		record.ApprovedByAdminID, record.ApprovedAt, record.ValidFrom, record.Status, record.ObservedAt, record.BillingPeriod,
		record.CommitmentMonths, record.PriceUnit, record.SeatCount, record.Quantity, record.Currency, record.OriginalAmount,
		record.TaxIncluded, record.NormalizedMonthlyCNY, record.FXRate, record.FXSource, record.FXObservedAt,
		record.ConversionMode, record.RoundingRule, record.Fingerprint, record.OfferKey, record.CreatedAt, record.Version)
	if err != nil {
		if isUniqueViolation(err) {
			return officialprice.Lead{}, officialprice.Record{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "活跃官网价格记录冲突，请刷新后重试。")
		}
		return officialprice.Lead{}, officialprice.Record{}, internalStoreError()
	}
	return lead, record, nil
}

func (s *Store) writeOfficialPriceRecordAuditInTx(ctx context.Context, tx pgx.Tx, adminUserID, eventType, auditAction, reason, requestID string, recordID string, version int64, before any, after any, metadata map[string]string, now time.Time) *domain.AppError {
	eventID := uuid.NewString()
	if strings.TrimSpace(requestID) == "" {
		requestID = "unknown"
	}
	if metadata == nil {
		metadata = map[string]string{}
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO domain_events (
			id, aggregate_type, aggregate_id, event_type, actor_user_id, actor_kind,
			aggregate_version, request_id, metadata_json, created_at
		)
		VALUES (
			$1, 'official_price_record', $2, $3, $4, 'admin',
			$5, $6, $7, $8
		)
	`, eventID, recordID, eventType, adminUserID, version, requestID, json.RawMessage(metadataJSON), now)
	if err != nil {
		return internalStoreError()
	}

	var beforeJSON any
	if before != nil {
		body, err := json.Marshal(before)
		if err != nil {
			return internalStoreError()
		}
		beforeJSON = json.RawMessage(body)
	}
	afterJSON, err := json.Marshal(after)
	if err != nil {
		return internalStoreError()
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO admin_audit_logs (
			admin_user_id, action, target_type, target_id, reason, before_json,
			after_json, request_id, created_at
		)
		VALUES ($1, $2, 'official_price_record', $3, $4, $5, $6, $7, $8)
	`, adminUserID, auditAction, recordID, strings.TrimSpace(reason), beforeJSON, json.RawMessage(afterJSON), requestID, now)
	if err != nil {
		return internalStoreError()
	}
	return nil
}

func officialPriceOriginalText(input officialprice.AdminRecordInput) string {
	return strings.TrimSpace(input.Currency + " " + input.OriginalAmount + " / month")
}

func scanOfficialPriceLeads(rows pgx.Rows) ([]officialprice.Lead, *domain.AppError) {
	var leads []officialprice.Lead
	for rows.Next() {
		var lead officialprice.Lead
		if err := scanOfficialPriceLead(rows, &lead); err != nil {
			return nil, internalStoreError()
		}
		leads = append(leads, lead)
	}
	if err := rows.Err(); err != nil {
		return nil, internalStoreError()
	}
	return leads, nil
}

func scanOfficialPriceLead(row scanner, lead *officialprice.Lead) error {
	return row.Scan(
		&lead.ID,
		&lead.SubmitterUserID,
		&lead.ProductPlanID,
		&lead.ProductText,
		&lead.PlanText,
		&lead.RegionCode,
		&lead.Channel,
		&lead.OpeningMethod,
		&lead.SourceURL,
		&lead.SourceTitle,
		&lead.EvidenceSummary,
		&lead.Note,
		&lead.Status,
		&lead.ReviewedByAdminID,
		&lead.ReviewedAt,
		&lead.ReviewReason,
		&lead.ObservedAt,
		&lead.BillingPeriod,
		&lead.CommitmentMonths,
		&lead.PriceUnit,
		&lead.SeatCount,
		&lead.Quantity,
		&lead.Currency,
		&lead.OriginalAmount,
		&lead.OriginalPriceText,
		&lead.TaxIncluded,
		&lead.NormalizedMonthlyCNY,
		&lead.FXRate,
		&lead.FXSource,
		&lead.FXObservedAt,
		&lead.ConversionMode,
		&lead.RoundingRule,
		&lead.Fingerprint,
		&lead.OfferKey,
		&lead.DuplicateOfLeadID,
		&lead.CreatedAt,
		&lead.UpdatedAt,
		&lead.Version,
	)
}

func scanOfficialPriceRecords(rows pgx.Rows) ([]officialprice.Record, *domain.AppError) {
	records := []officialprice.Record{}
	for rows.Next() {
		var record officialprice.Record
		if err := scanOfficialPriceRecord(rows, &record); err != nil {
			return nil, internalStoreError()
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, internalStoreError()
	}
	return records, nil
}

func scanOfficialPriceRecord(row scanner, record *officialprice.Record) error {
	return row.Scan(
		&record.ID,
		&record.LeadID,
		&record.ProductPlanID,
		&record.RegionCode,
		&record.Channel,
		&record.OpeningMethod,
		&record.SourceURL,
		&record.ApprovedByAdminID,
		&record.ApprovedAt,
		&record.ValidFrom,
		&record.ValidTo,
		&record.Status,
		&record.ObservedAt,
		&record.BillingPeriod,
		&record.CommitmentMonths,
		&record.PriceUnit,
		&record.SeatCount,
		&record.Quantity,
		&record.Currency,
		&record.OriginalAmount,
		&record.TaxIncluded,
		&record.NormalizedMonthlyCNY,
		&record.FXRate,
		&record.FXSource,
		&record.FXObservedAt,
		&record.ConversionMode,
		&record.RoundingRule,
		&record.Fingerprint,
		&record.OfferKey,
		&record.CreatedAt,
		&record.Version,
	)
}
func canUpdateLeadReviewStatus(currentStatus, nextStatus string) bool {
	switch nextStatus {
	case officialprice.LeadStatusRejected:
		return currentStatus == officialprice.LeadStatusPending || currentStatus == officialprice.LeadStatusChangesRequested
	case officialprice.LeadStatusChangesRequested:
		return currentStatus == officialprice.LeadStatusPending
	default:
		return false
	}
}
