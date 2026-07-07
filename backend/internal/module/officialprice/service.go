package officialprice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/google/uuid"
)

type Service struct {
	mu          sync.Mutex
	now         func() time.Time
	repo        Repository
	idempotency *idempotency.Service

	leads               map[string]Lead
	leadOrder           []string
	records             map[string]Record
	recordByLeadID      map[string]string
	activeRecordByOffer map[string]string
}

func NewService(repo Repository, idempotencyService *idempotency.Service, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	if idempotencyService == nil {
		idempotencyService = idempotency.NewService(nil, now)
	}
	return &Service{
		now:                 now,
		repo:                repo,
		idempotency:         idempotencyService,
		leads:               make(map[string]Lead),
		records:             make(map[string]Record),
		recordByLeadID:      make(map[string]string),
		activeRecordByOffer: make(map[string]string),
	}
}

func (s *Service) SubmitLead(ctx context.Context, user auth.User, input SubmitLeadInput) (Lead, *domain.AppError) {
	return Lead{}, domain.NewError(http.StatusForbidden, domain.CodeOfficialPriceUserSubmitDisabled, "Official price user submit disabled", "官网价格已改为管理员维护，用户不再提交低价线索。")
}

func (s *Service) submitLeadLegacy(ctx context.Context, user auth.User, input SubmitLeadInput) (Lead, *domain.AppError) {
	if err := validateSubmitLeadInput(input); err != nil {
		return Lead{}, err
	}
	if s.repo != nil {
		return s.submitPersistentLead(ctx, user, input)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	input = normalizeSingleAccountLeadInput(input)
	fingerprint := computeLeadFingerprint(input)
	duplicateOf := ""
	for _, existing := range s.leads {
		if existing.Fingerprint == fingerprint {
			duplicateOf = existing.ID
			break
		}
	}

	lead := Lead{
		ID:                uuid.NewString(),
		SubmitterUserID:   user.ID,
		ProductPlanID:     strings.TrimSpace(input.ProductPlanID),
		ProductText:       strings.TrimSpace(input.ProductText),
		PlanText:          strings.TrimSpace(input.PlanText),
		RegionCode:        strings.ToLower(strings.TrimSpace(input.RegionCode)),
		Channel:           strings.TrimSpace(input.Channel),
		OpeningMethod:     strings.TrimSpace(input.OpeningMethod),
		SourceURL:         strings.TrimSpace(input.SourceURL),
		SourceTitle:       strings.TrimSpace(input.SourceTitle),
		EvidenceSummary:   strings.TrimSpace(input.EvidenceSummary),
		Note:              strings.TrimSpace(input.Note),
		Status:            LeadStatusPending,
		ObservedAt:        input.ObservedAt,
		BillingPeriod:     input.BillingPeriod,
		CommitmentMonths:  input.CommitmentMonths,
		PriceUnit:         input.PriceUnit,
		SeatCount:         input.SeatCount,
		Quantity:          input.Quantity,
		Currency:          strings.ToUpper(strings.TrimSpace(input.Currency)),
		OriginalAmount:    strings.TrimSpace(input.OriginalAmount),
		OriginalPriceText: strings.TrimSpace(input.OriginalPriceText),
		TaxIncluded:       input.TaxIncluded,
		Fingerprint:       fingerprint,
		DuplicateOfLeadID: duplicateOf,
		CreatedAt:         now,
		UpdatedAt:         now,
		Version:           1,
	}
	s.leads[lead.ID] = lead
	s.leadOrder = append(s.leadOrder, lead.ID)
	return lead, nil
}

func (s *Service) MyLeads(ctx context.Context, user auth.User) ([]Lead, *domain.AppError) {
	if s.repo != nil {
		leads, err := s.repo.ListOfficialPriceLeadsBySubmitter(ctx, user.ID)
		if err != nil {
			return nil, err
		}
		return leads, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var leads []Lead
	for _, id := range s.leadOrder {
		lead := s.leads[id]
		if lead.SubmitterUserID == user.ID {
			leads = append(leads, lead)
		}
	}
	return leads, nil
}

func (s *Service) MyLead(ctx context.Context, user auth.User, leadID string) (Lead, *domain.AppError) {
	var lead Lead
	var appErr *domain.AppError
	if s.repo != nil {
		lead, appErr = s.repo.GetOfficialPriceLead(ctx, leadID)
	} else {
		s.mu.Lock()
		lead = s.leads[leadID]
		s.mu.Unlock()
	}
	if appErr != nil {
		return Lead{}, appErr
	}
	if lead.ID == "" || lead.SubmitterUserID != user.ID {
		return Lead{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Lead not found", "低价线索不存在。")
	}
	return lead, nil
}

func (s *Service) AdminLeads(ctx context.Context, user auth.User) ([]Lead, *domain.AppError) {
	if !user.IsAdmin {
		return nil, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	if s.repo != nil {
		return s.repo.ListOfficialPriceLeads(ctx)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	leads := make([]Lead, 0, len(s.leadOrder))
	for _, id := range s.leadOrder {
		leads = append(leads, s.leads[id])
	}
	return leads, nil
}

func (s *Service) AdminLead(ctx context.Context, user auth.User, leadID string) (Lead, *domain.AppError) {
	if !user.IsAdmin {
		return Lead{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	if s.repo != nil {
		return s.repo.GetOfficialPriceLead(ctx, leadID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	lead, ok := s.leads[leadID]
	if !ok {
		return Lead{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Lead not found", "低价线索不存在。")
	}
	return lead, nil
}

func (s *Service) AdminRecords(ctx context.Context, user auth.User) ([]Record, *domain.AppError) {
	if !user.IsAdmin {
		return nil, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	if s.repo != nil {
		records, appErr := s.repo.ListAdminOfficialPriceRecords(ctx)
		if appErr != nil {
			return nil, appErr
		}
		markLowestReferences(records)
		return records, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	records := make([]Record, 0, len(s.records))
	for _, record := range s.records {
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool {
		if !records[i].CreatedAt.Equal(records[j].CreatedAt) {
			return records[i].CreatedAt.After(records[j].CreatedAt)
		}
		return records[i].ID < records[j].ID
	})
	markLowestReferences(records)
	return records, nil
}

func (s *Service) AdminRecord(ctx context.Context, user auth.User, recordID string) (Record, *domain.AppError) {
	if !user.IsAdmin {
		return Record{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	if s.repo != nil {
		return s.repo.GetAdminOfficialPriceRecord(ctx, recordID)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.records[recordID]
	if !ok {
		return Record{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Price record not found", "价格记录不存在。")
	}
	return record, nil
}

func (s *Service) AdminCreateRecord(ctx context.Context, user auth.User, input AdminRecordInput) (Record, *domain.AppError) {
	if !user.IsAdmin {
		return Record{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	input.AdminUserID = user.ID
	prepared, normalized, offerKey, fingerprint, appErr := prepareAdminRecordInput(input)
	if appErr != nil {
		return Record{}, appErr
	}
	now := s.now()
	if s.repo != nil {
		return s.repo.CreateAdminOfficialPriceRecord(ctx, prepared, normalized, offerKey, fingerprint, now)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if activeID := s.activeRecordByOffer[offerKey]; activeID != "" {
		active := s.records[activeID]
		if active.Status == RecordStatusActive {
			validTo := now
			active.Status = RecordStatusSuperseded
			active.ValidTo = &validTo
			active.Version++
			s.records[activeID] = active
		}
		delete(s.activeRecordByOffer, offerKey)
	}

	lead := buildAdminRecordLead(prepared, normalized, offerKey, fingerprint, now)
	record := buildAdminRecord(prepared, lead, normalized, offerKey, fingerprint, now)
	s.leads[lead.ID] = lead
	s.leadOrder = append(s.leadOrder, lead.ID)
	s.records[record.ID] = record
	s.recordByLeadID[lead.ID] = record.ID
	s.activeRecordByOffer[offerKey] = record.ID
	return record, nil
}

func (s *Service) AdminUpdateRecord(ctx context.Context, user auth.User, input AdminRecordInput) (Record, *domain.AppError) {
	if !user.IsAdmin {
		return Record{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	input.AdminUserID = user.ID
	prepared, normalized, offerKey, fingerprint, appErr := prepareAdminRecordInput(input)
	if appErr != nil {
		return Record{}, appErr
	}
	if strings.TrimSpace(prepared.RecordID) == "" {
		return Record{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Record ID required", "必须提供价格记录 ID。", "recordId", "required", "必须提供价格记录 ID。")
	}
	now := s.now()
	if s.repo != nil {
		return s.repo.UpdateAdminOfficialPriceRecord(ctx, prepared, normalized, offerKey, fingerprint, now)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	oldRecord, ok := s.records[prepared.RecordID]
	if !ok {
		return Record{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Price record not found", "价格记录不存在。")
	}
	if oldRecord.Status != RecordStatusActive {
		return Record{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "只有生效中的价格记录可以编辑。")
	}
	if prepared.ExpectedVersion > 0 && oldRecord.Version != prepared.ExpectedVersion {
		return Record{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}

	validTo := now
	oldRecord.Status = RecordStatusSuperseded
	oldRecord.ValidTo = &validTo
	oldRecord.Version++
	s.records[oldRecord.ID] = oldRecord
	if s.activeRecordByOffer[oldRecord.OfferKey] == oldRecord.ID {
		delete(s.activeRecordByOffer, oldRecord.OfferKey)
	}
	if activeID := s.activeRecordByOffer[offerKey]; activeID != "" {
		active := s.records[activeID]
		if active.Status == RecordStatusActive {
			validTo := now
			active.Status = RecordStatusSuperseded
			active.ValidTo = &validTo
			active.Version++
			s.records[activeID] = active
		}
		delete(s.activeRecordByOffer, offerKey)
	}

	lead := buildAdminRecordLead(prepared, normalized, offerKey, fingerprint, now)
	record := buildAdminRecord(prepared, lead, normalized, offerKey, fingerprint, now)
	s.leads[lead.ID] = lead
	s.leadOrder = append(s.leadOrder, lead.ID)
	s.records[record.ID] = record
	s.recordByLeadID[lead.ID] = record.ID
	s.activeRecordByOffer[offerKey] = record.ID
	return record, nil
}

func (s *Service) AdminTakeDownRecord(ctx context.Context, user auth.User, input AdminRecordActionInput) (Record, *domain.AppError) {
	if !user.IsAdmin {
		return Record{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	input.AdminUserID = user.ID
	if strings.TrimSpace(input.RecordID) == "" {
		return Record{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Record ID required", "必须提供价格记录 ID。", "recordId", "required", "必须提供价格记录 ID。")
	}
	if strings.TrimSpace(input.Reason) == "" {
		return Record{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Action reason required", "下架价格记录必须填写原因。", "reason", "required", "必须填写下架原因。")
	}
	now := s.now()
	if s.repo != nil {
		return s.repo.TakeDownOfficialPriceRecord(ctx, input, now)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.records[input.RecordID]
	if !ok {
		return Record{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Price record not found", "价格记录不存在。")
	}
	if record.Status != RecordStatusActive {
		return Record{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "只有生效中的价格记录可以下架。")
	}
	if input.ExpectedVersion > 0 && record.Version != input.ExpectedVersion {
		return Record{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	validTo := now
	record.Status = RecordStatusTakenDown
	record.ValidTo = &validTo
	record.Version++
	s.records[record.ID] = record
	if s.activeRecordByOffer[record.OfferKey] == record.ID {
		delete(s.activeRecordByOffer, record.OfferKey)
	}
	return record, nil
}

func (s *Service) ApproveLead(ctx context.Context, input ApproveLeadInput) (Lead, Record, *domain.AppError) {
	if err := validateApproveLeadInput(input); err != nil {
		return Lead{}, Record{}, err
	}
	if s.repo != nil {
		return s.approvePersistentLead(ctx, input)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	lead, ok := s.leads[input.LeadID]
	if !ok {
		return Lead{}, Record{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Lead not found", "低价线索不存在。")
	}
	if input.ExpectedVersion > 0 && lead.Version != input.ExpectedVersion {
		return Lead{}, Record{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if lead.Status != LeadStatusPending && lead.Status != LeadStatusChangesRequested {
		return Lead{}, Record{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前线索状态不能标记通过。")
	}
	if recordID := s.recordByLeadID[lead.ID]; recordID != "" {
		return Lead{}, Record{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前线索已有价格记录，不能重复标记通过。")
	}
	lead = normalizeSingleAccountLead(lead)

	normalized, err := normalizeMonthlyCNY(lead.OriginalAmount, input.FXRateToCNY, lead.BillingPeriod)
	if err != nil {
		return Lead{}, Record{}, domain.NewError(http.StatusUnprocessableEntity, domain.CodePriceNormalizationRequired, "Price normalization required", "价格归一化失败。")
	}

	now := s.now()
	offerKey := computeOfferKey(lead, input.ResolvedProductPlanID)
	if activeID := s.activeRecordByOffer[offerKey]; activeID != "" {
		active := s.records[activeID]
		active.Status = RecordStatusSuperseded
		active.ValidTo = &now
		active.Version++
		s.records[activeID] = active
	}

	lead.ProductPlanID = input.ResolvedProductPlanID
	lead.Status = LeadStatusApproved
	lead.ReviewedByAdminID = input.AdminUserID
	lead.ReviewedAt = &now
	lead.ReviewReason = strings.TrimSpace(input.Reason)
	lead.CommitmentMonths = nil
	lead.PriceUnit = "per_account"
	lead.SeatCount = nil
	lead.Quantity = 1
	lead.NormalizedMonthlyCNY = normalized
	lead.FXRate = input.FXRateToCNY
	lead.FXSource = strings.TrimSpace(input.FXSource)
	lead.FXObservedAt = &input.FXObservedAt
	lead.ConversionMode = "monthly_normalized"
	lead.RoundingRule = "round_half_up_2"
	lead.OfferKey = offerKey
	lead.UpdatedAt = now
	lead.Version++
	s.leads[lead.ID] = lead

	record := Record{
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
		Status:               RecordStatusActive,
		ObservedAt:           lead.ObservedAt,
		BillingPeriod:        lead.BillingPeriod,
		CommitmentMonths:     lead.CommitmentMonths,
		PriceUnit:            lead.PriceUnit,
		SeatCount:            nil,
		Quantity:             1,
		Currency:             lead.Currency,
		OriginalAmount:       lead.OriginalAmount,
		TaxIncluded:          lead.TaxIncluded,
		NormalizedMonthlyCNY: normalized,
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
	s.records[record.ID] = record
	s.recordByLeadID[lead.ID] = record.ID
	s.activeRecordByOffer[offerKey] = record.ID
	return lead, record, nil
}

func (s *Service) ApproveLeadWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input ApproveLeadInput, buildCompletion ApprovalCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	key = strings.TrimSpace(key)
	if err := idempotency.ValidateKey(key); err != nil {
		return idempotency.Completion{}, err
	}
	if buildCompletion == nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}

	if s.repo == nil {
		entry, appErr := s.idempotency.Begin(ctx, userID, routeKey, key, requestHash)
		if appErr != nil {
			return idempotency.Completion{}, appErr
		}
		if entry.State == "completed" {
			return idempotency.CompletionFromEntry(entry), nil
		}
		lead, record, appErr := s.ApproveLead(ctx, input)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		completion, appErr := buildCompletion(lead, record)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		if appErr := s.idempotency.Complete(ctx, entry, completion.Status, completion.ContentType, completion.Body, completion.ResourceType, completion.ResourceID); appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}

	if err := validateApproveLeadInput(input); err != nil {
		return idempotency.Completion{}, err
	}
	entry, appErr := s.idempotency.Begin(ctx, userID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}

	lead, appErr := s.repo.GetOfficialPriceLead(ctx, input.LeadID)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	lead = normalizeSingleAccountLead(lead)
	normalized, err := normalizeMonthlyCNY(lead.OriginalAmount, input.FXRateToCNY, lead.BillingPeriod)
	if err != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, domain.NewError(http.StatusUnprocessableEntity, domain.CodePriceNormalizationRequired, "Price normalization required", "价格归一化失败。")
	}
	offerKey := computeOfferKey(lead, input.ResolvedProductPlanID)
	_, _, completion, appErr := s.repo.ApproveOfficialPriceLeadWithIdempotency(ctx, *entry, input, normalized, offerKey, s.now(), buildCompletion)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	return completion, nil
}

func (s *Service) UpdateLeadReviewStatus(ctx context.Context, user auth.User, leadID, status, reason string, ifMatchVersion int64) (Lead, *domain.AppError) {
	if !user.IsAdmin {
		return Lead{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	if strings.TrimSpace(reason) == "" {
		return Lead{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Review reason required", "审核动作必须填写原因。", "reason", "required", "必须填写审核原因。")
	}
	if status != LeadStatusRejected && status != LeadStatusChangesRequested {
		return Lead{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "不支持的审核状态。")
	}
	if s.repo != nil {
		return s.repo.UpdateLeadReviewStatus(ctx, user, leadID, status, reason, ifMatchVersion, s.now())
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	lead, ok := s.leads[leadID]
	if !ok {
		return Lead{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Lead not found", "低价线索不存在。")
	}
	if ifMatchVersion > 0 && lead.Version != ifMatchVersion {
		return Lead{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
	}
	if !canUpdateLeadReviewStatus(lead.Status, status) {
		return Lead{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "当前线索状态不能执行该审核动作。")
	}
	lead.Status = status
	lead.ReviewedByAdminID = user.ID
	now := s.now()
	lead.ReviewedAt = &now
	lead.ReviewReason = strings.TrimSpace(reason)
	lead.UpdatedAt = now
	lead.Version++
	s.leads[lead.ID] = lead
	return lead, nil
}

func (s *Service) PublicRecords(ctx context.Context) ([]Record, *domain.AppError) {
	if s.repo != nil {
		records, appErr := s.repo.ListOfficialPriceRecords(ctx)
		if appErr != nil {
			return nil, appErr
		}
		markLowestReferences(records)
		return records, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	records := make([]Record, 0, len(s.records))
	for _, record := range s.records {
		if record.Status == RecordStatusActive {
			records = append(records, record)
		}
	}
	sort.Slice(records, func(i, j int) bool {
		cmp := compareDecimalStrings(records[i].NormalizedMonthlyCNY, records[j].NormalizedMonthlyCNY)
		if cmp != 0 {
			return cmp < 0
		}
		if !records[i].ValidFrom.Equal(records[j].ValidFrom) {
			return records[i].ValidFrom.After(records[j].ValidFrom)
		}
		return records[i].ID < records[j].ID
	})
	markLowestReferences(records)
	return records, nil
}

func (s *Service) PublicRecord(ctx context.Context, recordID string) (Record, *domain.AppError) {
	if s.repo != nil {
		record, appErr := s.repo.GetOfficialPriceRecord(ctx, recordID)
		if appErr != nil {
			return Record{}, appErr
		}
		records, listErr := s.repo.ListOfficialPriceRecords(ctx)
		if listErr != nil {
			return Record{}, listErr
		}
		markLowestReferences(records)
		for _, item := range records {
			if item.ID == record.ID {
				record.IsLowestReference = item.IsLowestReference
				break
			}
		}
		return record, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	record, ok := s.records[recordID]
	if !ok || record.Status != RecordStatusActive {
		return Record{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Price record not found", "价格记录不存在。")
	}
	records := make([]Record, 0, len(s.records))
	for _, item := range s.records {
		if item.Status == RecordStatusActive {
			records = append(records, item)
		}
	}
	markLowestReferences(records)
	for _, item := range records {
		if item.ID == record.ID {
			record.IsLowestReference = item.IsLowestReference
			break
		}
	}
	return record, nil
}

func (s *Service) submitPersistentLead(ctx context.Context, user auth.User, input SubmitLeadInput) (Lead, *domain.AppError) {
	now := s.now()
	input = normalizeSingleAccountLeadInput(input)
	fingerprint := computeLeadFingerprint(input)
	duplicateOf, appErr := s.repo.FindDuplicateOfficialPriceLeadID(ctx, fingerprint)
	if appErr != nil {
		return Lead{}, appErr
	}
	lead := Lead{
		ID:                uuid.NewString(),
		SubmitterUserID:   user.ID,
		ProductPlanID:     strings.TrimSpace(input.ProductPlanID),
		ProductText:       strings.TrimSpace(input.ProductText),
		PlanText:          strings.TrimSpace(input.PlanText),
		RegionCode:        strings.ToLower(strings.TrimSpace(input.RegionCode)),
		Channel:           strings.TrimSpace(input.Channel),
		OpeningMethod:     strings.TrimSpace(input.OpeningMethod),
		SourceURL:         strings.TrimSpace(input.SourceURL),
		SourceTitle:       strings.TrimSpace(input.SourceTitle),
		EvidenceSummary:   strings.TrimSpace(input.EvidenceSummary),
		Note:              strings.TrimSpace(input.Note),
		Status:            LeadStatusPending,
		ObservedAt:        input.ObservedAt,
		BillingPeriod:     input.BillingPeriod,
		CommitmentMonths:  input.CommitmentMonths,
		PriceUnit:         input.PriceUnit,
		SeatCount:         input.SeatCount,
		Quantity:          input.Quantity,
		Currency:          strings.ToUpper(strings.TrimSpace(input.Currency)),
		OriginalAmount:    strings.TrimSpace(input.OriginalAmount),
		OriginalPriceText: strings.TrimSpace(input.OriginalPriceText),
		TaxIncluded:       input.TaxIncluded,
		Fingerprint:       fingerprint,
		DuplicateOfLeadID: duplicateOf,
		CreatedAt:         now,
		UpdatedAt:         now,
		Version:           1,
	}
	if appErr := s.repo.CreateOfficialPriceLead(ctx, lead); appErr != nil {
		return Lead{}, appErr
	}
	return lead, nil
}

func (s *Service) approvePersistentLead(ctx context.Context, input ApproveLeadInput) (Lead, Record, *domain.AppError) {
	lead, appErr := s.repo.GetOfficialPriceLead(ctx, input.LeadID)
	if appErr != nil {
		return Lead{}, Record{}, appErr
	}
	lead = normalizeSingleAccountLead(lead)
	normalized, err := normalizeMonthlyCNY(lead.OriginalAmount, input.FXRateToCNY, lead.BillingPeriod)
	if err != nil {
		return Lead{}, Record{}, domain.NewError(http.StatusUnprocessableEntity, domain.CodePriceNormalizationRequired, "Price normalization required", "价格归一化失败。")
	}
	offerKey := computeOfferKey(lead, input.ResolvedProductPlanID)
	return s.repo.ApproveOfficialPriceLead(ctx, input, normalized, offerKey, s.now())
}

func prepareAdminRecordInput(input AdminRecordInput) (AdminRecordInput, string, string, string, *domain.AppError) {
	input = normalizeAdminRecordInput(input)
	if err := validateAdminRecordInput(input); err != nil {
		return AdminRecordInput{}, "", "", "", err
	}
	normalized, err := normalizeMonthlyCNY(input.OriginalAmount, input.FXRateToCNY, input.BillingPeriod)
	if err != nil {
		return AdminRecordInput{}, "", "", "", domain.NewError(http.StatusUnprocessableEntity, domain.CodePriceNormalizationRequired, "Price normalization required", "价格归一化失败。")
	}
	fingerprint := computeLeadFingerprint(adminRecordSubmitInput(input))
	lead := Lead{
		ProductPlanID:    input.ProductPlanID,
		RegionCode:       input.RegionCode,
		Channel:          input.Channel,
		OpeningMethod:    input.OpeningMethod,
		BillingPeriod:    input.BillingPeriod,
		CommitmentMonths: nil,
		PriceUnit:        "per_account",
		SeatCount:        nil,
		Quantity:         1,
		TaxIncluded:      input.TaxIncluded,
	}
	offerKey := computeOfferKey(lead, input.ProductPlanID)
	return input, normalized, offerKey, fingerprint, nil
}

func normalizeAdminRecordInput(input AdminRecordInput) AdminRecordInput {
	input.RecordID = strings.TrimSpace(input.RecordID)
	input.AdminUserID = strings.TrimSpace(input.AdminUserID)
	input.RequestID = strings.TrimSpace(input.RequestID)
	input.ProductPlanID = strings.TrimSpace(input.ProductPlanID)
	input.ProductText = strings.TrimSpace(input.ProductText)
	input.PlanText = strings.TrimSpace(input.PlanText)
	input.RegionCode = strings.ToLower(strings.TrimSpace(input.RegionCode))
	input.Channel = strings.TrimSpace(input.Channel)
	input.OpeningMethod = strings.TrimSpace(input.OpeningMethod)
	input.SourceURL = strings.TrimSpace(input.SourceURL)
	input.BillingPeriod = strings.TrimSpace(input.BillingPeriod)
	input.Currency = strings.ToUpper(strings.TrimSpace(input.Currency))
	input.OriginalAmount = strings.TrimSpace(input.OriginalAmount)
	input.FXRateToCNY = strings.TrimSpace(input.FXRateToCNY)
	input.FXSource = strings.TrimSpace(input.FXSource)
	input.Reason = strings.TrimSpace(input.Reason)
	return input
}

func validateAdminRecordInput(input AdminRecordInput) *domain.AppError {
	if strings.TrimSpace(input.ProductPlanID) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeProductPlanResolutionRequired, "Product plan required", "管理员维护价格记录必须选择产品套餐。", "productPlanId", "required", "必须选择产品套餐。")
	}
	if strings.TrimSpace(input.RegionCode) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Region required", "必须填写地区代码。", "regionCode", "required", "必须填写地区代码。")
	}
	if strings.TrimSpace(input.Channel) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Channel required", "必须填写价格渠道。", "channel", "required", "必须填写价格渠道。")
	}
	if strings.TrimSpace(input.OpeningMethod) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Opening method required", "必须填写开通方式。", "openingMethod", "required", "必须填写开通方式。")
	}
	if err := validateEvidenceURL(input.SourceURL); err != nil {
		return err
	}
	if input.ObservedAt.IsZero() {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Observed time required", "必须填写价格观察时间。", "observedAt", "required", "必须填写观察时间。")
	}
	if input.BillingPeriod != "monthly" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodePriceNormalizationRequired, "Billing period not supported", "官网价格当前只支持单账号月付价格。", "billingPeriod", "unsupported", "当前仅支持 monthly。")
	}
	if amount, ok := parsePositiveDecimal(input.OriginalAmount); !ok || amount.Sign() <= 0 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Amount invalid", "原始金额格式不正确。", "originalAmount", "invalid", "金额必须为正数。")
	}
	if len(input.Currency) != 3 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Currency invalid", "币种必须是三位代码。", "currency", "invalid", "币种必须是三位代码。")
	}
	if _, ok := parsePositiveDecimal(input.FXRateToCNY); !ok {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodePriceNormalizationRequired, "FX rate required", "必须提供有效汇率。", "fxRateToCny", "invalid", "汇率必须为正数。")
	}
	if strings.TrimSpace(input.FXSource) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "FX source required", "必须填写汇率来源。", "fxSource", "required", "必须填写汇率来源。")
	}
	if input.FXObservedAt.IsZero() {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "FX observed time required", "必须填写汇率观察时间。", "fxObservedAt", "required", "必须填写汇率观察时间。")
	}
	if input.ValidFrom.IsZero() {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Valid from required", "必须填写价格生效时间。", "validFrom", "required", "必须填写生效时间。")
	}
	if strings.TrimSpace(input.Reason) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Reason required", "维护价格记录必须填写原因。", "reason", "required", "必须填写维护原因。")
	}
	return nil
}

func adminRecordSubmitInput(input AdminRecordInput) SubmitLeadInput {
	return SubmitLeadInput{
		ProductPlanID:     input.ProductPlanID,
		ProductText:       input.ProductText,
		PlanText:          input.PlanText,
		RegionCode:        input.RegionCode,
		Channel:           input.Channel,
		OpeningMethod:     input.OpeningMethod,
		SourceURL:         input.SourceURL,
		SourceTitle:       "管理员维护官网价格",
		EvidenceSummary:   input.Reason,
		ObservedAt:        input.ObservedAt,
		BillingPeriod:     input.BillingPeriod,
		CommitmentMonths:  nil,
		PriceUnit:         "per_account",
		SeatCount:         nil,
		Quantity:          1,
		Currency:          input.Currency,
		OriginalAmount:    input.OriginalAmount,
		OriginalPriceText: originalPriceText(input),
		TaxIncluded:       input.TaxIncluded,
	}
}

func buildAdminRecordLead(input AdminRecordInput, normalizedMonthlyCNY, offerKey, fingerprint string, now time.Time) Lead {
	fxObservedAt := input.FXObservedAt
	reviewedAt := now
	return Lead{
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
		Status:               LeadStatusApproved,
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
		OriginalPriceText:    originalPriceText(input),
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
}

func buildAdminRecord(input AdminRecordInput, lead Lead, normalizedMonthlyCNY, offerKey, fingerprint string, now time.Time) Record {
	return Record{
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
		Status:               RecordStatusActive,
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
}

func originalPriceText(input AdminRecordInput) string {
	return strings.TrimSpace(input.Currency + " " + input.OriginalAmount + " / month")
}

func validateSubmitLeadInput(input SubmitLeadInput) *domain.AppError {
	input = normalizeSingleAccountLeadInput(input)
	if strings.TrimSpace(input.ProductText) == "" && strings.TrimSpace(input.ProductPlanID) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Product required", "必须提供产品信息。", "productText", "required", "必须提供产品信息。")
	}
	if err := validateEvidenceURL(input.SourceURL); err != nil {
		return err
	}
	if input.BillingPeriod != "monthly" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodePriceNormalizationRequired, "Billing period not supported", "官方价格当前只支持单账号月付价格。", "billingPeriod", "unsupported", "当前仅支持 monthly。")
	}
	if amount, ok := parsePositiveDecimal(input.OriginalAmount); !ok || amount.Sign() <= 0 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Amount invalid", "原始金额格式不正确。", "originalAmount", "invalid", "金额必须为正数。")
	}
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if len(currency) != 3 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Currency invalid", "币种必须是三位代码。", "currency", "invalid", "币种必须是三位代码。")
	}
	return nil
}

func normalizeSingleAccountLeadInput(input SubmitLeadInput) SubmitLeadInput {
	input.CommitmentMonths = nil
	input.PriceUnit = "per_account"
	input.SeatCount = nil
	input.Quantity = 1
	return input
}

func normalizeSingleAccountLead(lead Lead) Lead {
	lead.CommitmentMonths = nil
	lead.PriceUnit = "per_account"
	lead.SeatCount = nil
	lead.Quantity = 1
	return lead
}

func validateApproveLeadInput(input ApproveLeadInput) *domain.AppError {
	if strings.TrimSpace(input.ResolvedProductPlanID) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeProductPlanResolutionRequired, "Product plan resolution required", "审核通过前必须解析产品套餐。", "resolvedProductPlanId", "required", "必须提供解析后的产品套餐。")
	}
	if strings.TrimSpace(input.Reason) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Review reason required", "审核动作必须填写原因。", "reason", "required", "必须填写审核原因。")
	}
	if _, ok := parsePositiveDecimal(input.FXRateToCNY); !ok {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodePriceNormalizationRequired, "FX snapshot required", "审核通过前必须提供有效汇率快照。", "fxSnapshot.rateToCny", "invalid", "汇率必须为正数。")
	}
	return nil
}

func canUpdateLeadReviewStatus(currentStatus, nextStatus string) bool {
	switch nextStatus {
	case LeadStatusRejected:
		return currentStatus == LeadStatusPending || currentStatus == LeadStatusChangesRequested
	case LeadStatusChangesRequested:
		return currentStatus == LeadStatusPending
	default:
		return false
	}
}

func validateEvidenceURL(raw string) *domain.AppError {
	if len(raw) > 2048 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeURLNotAllowed, "URL not allowed", "来源 URL 过长。", "sourceUrl", "too_long", "来源 URL 过长。")
	}
	if strings.ContainsAny(raw, "\x00\r\n\t") {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeURLNotAllowed, "URL not allowed", "来源 URL 包含控制字符。", "sourceUrl", "control_character", "来源 URL 包含控制字符。")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeURLNotAllowed, "URL not allowed", "来源 URL 必须是 https。", "sourceUrl", "https_required", "来源 URL 必须是 https。")
	}
	if parsed.User != nil {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeURLNotAllowed, "URL not allowed", "来源 URL 不能包含 userinfo。", "sourceUrl", "userinfo_forbidden", "来源 URL 不能包含 userinfo。")
	}
	if parsed.Fragment != "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeURLNotAllowed, "URL not allowed", "来源 URL 不能包含 fragment。", "sourceUrl", "fragment_forbidden", "来源 URL 不能包含 fragment。")
	}
	for key := range parsed.Query() {
		normalized := strings.ToLower(key)
		switch normalized {
		case "key", "token", "apikey", "api_key", "access_token", "refresh_token", "session", "cookie", "password":
			return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeSecretContentDetected, "Secret content detected", "来源 URL 不能包含认证参数。", "sourceUrl", "secret_query", "来源 URL 不能包含认证参数。")
		}
	}
	decoded, _ := url.QueryUnescape(parsed.EscapedPath() + "?" + parsed.RawQuery)
	if looksLikeSecret(decoded) {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeSecretContentDetected, "Secret content detected", "来源 URL 看起来包含认证秘密。", "sourceUrl", "secret_content", "来源 URL 看起来包含认证秘密。")
	}
	return nil
}

func normalizeMonthlyCNY(amountText, rateText, billingPeriod string) (string, error) {
	amount, ok := parseNonNegativeDecimal(amountText)
	if !ok {
		return "", fmt.Errorf("invalid amount")
	}
	rate, ok := parsePositiveDecimal(rateText)
	if !ok {
		return "", fmt.Errorf("invalid rate")
	}
	value := new(big.Rat).Mul(amount, rate)
	if billingPeriod == "annual" {
		value.Quo(value, big.NewRat(12, 1))
	}
	return decimalString(value, 2), nil
}

func parseNonNegativeDecimal(value string) (*big.Rat, bool) {
	rat, ok := new(big.Rat).SetString(strings.TrimSpace(value))
	if !ok || rat.Sign() < 0 {
		return nil, false
	}
	return rat, true
}

func parsePositiveDecimal(value string) (*big.Rat, bool) {
	rat, ok := new(big.Rat).SetString(strings.TrimSpace(value))
	if !ok || rat.Sign() <= 0 {
		return nil, false
	}
	return rat, true
}

func decimalString(value *big.Rat, places int) string {
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(places)), nil)
	scaled := new(big.Rat).Mul(value, new(big.Rat).SetInt(scale))
	rounded := roundRatHalfUp(scaled)
	intPart := new(big.Int).Quo(rounded, scale)
	frac := new(big.Int).Mod(rounded, scale)
	fracText := frac.String()
	for len(fracText) < places {
		fracText = "0" + fracText
	}
	return fmt.Sprintf("%s.%s", intPart.String(), fracText)
}

func roundRatHalfUp(value *big.Rat) *big.Int {
	num := new(big.Int).Set(value.Num())
	den := new(big.Int).Set(value.Denom())
	quotient, remainder := new(big.Int).QuoRem(num, den, new(big.Int))
	twice := new(big.Int).Mul(remainder, big.NewInt(2))
	if twice.Cmp(den) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	return quotient
}

func computeLeadFingerprint(input SubmitLeadInput) string {
	parts := []string{
		"source=" + normalizeURLForKey(input.SourceURL),
		"amount=" + strings.TrimSpace(input.OriginalAmount),
		"currency=" + strings.ToUpper(strings.TrimSpace(input.Currency)),
		"observed=" + input.ObservedAt.UTC().Format("2006-01-02T15"),
		"period=" + input.BillingPeriod,
		"unit=" + input.PriceUnit,
		"quantity=" + fmt.Sprint(input.Quantity),
	}
	return sha256Hex(strings.Join(parts, "|"))
}

func computeOfferKey(lead Lead, productPlanID string) string {
	parts := []string{
		"product=" + productPlanID,
		"region=" + lead.RegionCode,
		"channel=" + lead.Channel,
		"opening=" + lead.OpeningMethod,
		"period=" + lead.BillingPeriod,
		"commitment=" + intPtrString(lead.CommitmentMonths),
		"unit=" + lead.PriceUnit,
		"seat=" + intPtrString(lead.SeatCount),
		"quantity=" + fmt.Sprint(lead.Quantity),
		"tax=" + fmt.Sprint(lead.TaxIncluded),
	}
	return sha256Hex(strings.Join(parts, "|"))
}

func normalizeURLForKey(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return strings.TrimSpace(raw)
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""
	query := parsed.Query()
	keys := make([]string, 0, len(query))
	for key := range query {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	ordered := url.Values{}
	for _, key := range keys {
		values := append([]string(nil), query[key]...)
		sort.Strings(values)
		for _, value := range values {
			ordered.Add(key, value)
		}
	}
	parsed.RawQuery = ordered.Encode()
	return parsed.String()
}

func intPtrString(value *int) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(*value)
}

func markLowestReferences(records []Record) {
	type bestRecord struct {
		index int
		price string
	}
	bestByGroup := map[string]bestRecord{}
	for index := range records {
		records[index].IsLowestReference = false
		if records[index].Status != RecordStatusActive || strings.TrimSpace(records[index].NormalizedMonthlyCNY) == "" {
			continue
		}
		group := lowestReferenceGroupKey(records[index])
		current := records[index].NormalizedMonthlyCNY
		best, ok := bestByGroup[group]
		if !ok || compareDecimalStrings(current, best.price) < 0 || (compareDecimalStrings(current, best.price) == 0 && records[index].ID < records[best.index].ID) {
			bestByGroup[group] = bestRecord{index: index, price: current}
		}
	}
	for _, best := range bestByGroup {
		records[best.index].IsLowestReference = true
	}
}

func lowestReferenceGroupKey(record Record) string {
	parts := []string{
		"product=" + record.ProductPlanID,
		"region=" + record.RegionCode,
		"channel=" + record.Channel,
		"opening=" + record.OpeningMethod,
		"period=" + record.BillingPeriod,
		"unit=" + record.PriceUnit,
		"tax=" + fmt.Sprint(record.TaxIncluded),
	}
	return strings.Join(parts, "|")
}

func compareDecimalStrings(left, right string) int {
	leftDecimal, leftOK := parseNonNegativeDecimal(left)
	rightDecimal, rightOK := parseNonNegativeDecimal(right)
	if !leftOK && !rightOK {
		return strings.Compare(left, right)
	}
	if !leftOK {
		return 1
	}
	if !rightOK {
		return -1
	}
	return leftDecimal.Cmp(rightDecimal)
}

func looksLikeSecret(value string) bool {
	lower := strings.ToLower(value)
	needles := []string{"bearer ", "api_key=", "apikey=", "access_token=", "refresh_token=", "session=", "cookie=", "password=", "api key", "sub2api key", "secret=", "token="}
	for _, needle := range needles {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
