package report

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/notification"

	"github.com/google/uuid"
)

type Service struct {
	mu                      sync.Mutex
	now                     func() time.Time
	repo                    Repository
	idempotency             *idempotency.Service
	notifications           *notification.Service
	reports                 map[string]Report
	disputes                map[string]DisputeCase
	appeals                 map[string]Appeal
	infoRequests            map[string]InfoRequest
	infoSupplements         map[string][]InfoSupplement
	disputeMessages         map[string][]DisputeMessage
	settlementProposals     map[string][]SettlementProposal
	disputeRemedies         map[string][]DisputeRemedy
	disputeProjectionCloser DisputeProjectionCloser
}

type DisputeProjectionCloser interface {
	CloseDisputeProjection(ctx context.Context, disputeCaseID, actorUserID, requestID string) *domain.AppError
	SetDisputeProjection(ctx context.Context, disputeCaseID, status, actorUserID, requestID string) *domain.AppError
	ValidateDisputeProposalAmount(ctx context.Context, disputeCaseID, resolution, amount string) *domain.AppError
}

func NewService(repo Repository, idempotencyService *idempotency.Service, now func() time.Time) *Service {
	return NewServiceWithNotifications(repo, idempotencyService, nil, now)
}

func (s *Service) SetDisputeProjectionCloser(closer DisputeProjectionCloser) {
	s.disputeProjectionCloser = closer
}

func NewServiceWithNotifications(repo Repository, idempotencyService *idempotency.Service, notifications *notification.Service, now func() time.Time) *Service {
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
		notifications:       notifications,
		reports:             make(map[string]Report),
		disputes:            make(map[string]DisputeCase),
		appeals:             make(map[string]Appeal),
		infoRequests:        make(map[string]InfoRequest),
		infoSupplements:     make(map[string][]InfoSupplement),
		disputeMessages:     make(map[string][]DisputeMessage),
		settlementProposals: make(map[string][]SettlementProposal),
		disputeRemedies:     make(map[string][]DisputeRemedy),
	}
}

func (s *Service) CreateReportWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input CreateReportInput, buildCompletion ReportCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	input.ReporterUserID = user.ID
	input.ReporterUsername = user.Username
	input.ReporterName = displayName(user)
	input.TargetType = normalize(input.TargetType)
	input.ReasonCode = normalizeReason(input.ReasonCode)
	input.ReportedUsername = normalizeUsername(input.ReportedUsername)
	if appErr := validateCreateReport(input); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	entry, appErr := s.begin(ctx, user.ID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}
	if s.repo != nil {
		_, completion, appErr := s.repo.CreateReportWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}
	item, appErr := s.createReportMemory(input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(item)
	return s.complete(ctx, entry, completion, appErr)
}

func (s *Service) MyReports(ctx context.Context, user auth.User) ([]Report, *domain.AppError) {
	if strings.TrimSpace(user.ID) == "" {
		return nil, sessionRequired()
	}
	if s.repo != nil {
		return s.repo.ListReportsByUser(ctx, user.ID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]Report, 0)
	for _, item := range s.reports {
		if item.ReporterUserID == user.ID {
			items = append(items, item)
		}
	}
	sortReports(items)
	return items, nil
}

func (s *Service) AdminReports(ctx context.Context, user auth.User, page domain.PageRequest) (domain.Page[Report], *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return domain.Page[Report]{}, appErr
	}
	if s.repo != nil {
		return s.repo.ListAdminReports(ctx, page)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]Report, 0, len(s.reports))
	for _, item := range s.reports {
		items = append(items, item)
	}
	sortReports(items)
	return domain.PageItems(items, page)
}

func (s *Service) AdminReport(ctx context.Context, user auth.User, id string) (Report, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return Report{}, appErr
	}
	if s.repo != nil {
		return s.repo.GetAdminReport(ctx, id)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.reports[id]
	if !ok {
		return Report{}, reportNotFound()
	}
	item.Supplements = append([]InfoSupplement(nil), s.infoSupplements[infoSupplementEntityKey(InfoRequestEntityReport, id)]...)
	return item, nil
}

func (s *Service) AdminReportActionWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input AdminActionInput, buildCompletion AdminCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	input.AdminUserID = user.ID
	input.PublicResultCode = normalizePublicResultCode(input.PublicResultCode)
	if appErr := validateReportAction(input); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	entry, appErr := s.begin(ctx, user.ID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}
	if s.repo != nil {
		_, completion, appErr := s.repo.UpdateReportAdminWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}
	result, appErr := s.updateReportAdminMemory(input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(result)
	return s.complete(ctx, entry, completion, appErr)
}

func (s *Service) AdminDisputes(ctx context.Context, user auth.User) ([]DisputeCase, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return nil, appErr
	}
	if s.repo != nil {
		return s.repo.ListAdminDisputes(ctx)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]DisputeCase, 0, len(s.disputes))
	for _, item := range s.disputes {
		items = append(items, item)
	}
	sortDisputes(items)
	return items, nil
}

func (s *Service) MyDisputes(ctx context.Context, user auth.User) ([]DisputeCase, *domain.AppError) {
	if strings.TrimSpace(user.ID) == "" {
		return nil, sessionRequired()
	}
	if s.repo != nil {
		return s.repo.ListDisputesByUser(ctx, user.ID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]DisputeCase, 0)
	for _, item := range s.disputes {
		if isDisputeParticipant(item, user.ID) {
			items = append(items, item)
		}
	}
	sortDisputes(items)
	return items, nil
}

func (s *Service) MyDispute(ctx context.Context, user auth.User, id string) (DisputeCase, *domain.AppError) {
	if strings.TrimSpace(user.ID) == "" {
		return DisputeCase{}, sessionRequired()
	}
	if s.repo != nil {
		return s.repo.GetDisputeForParticipant(ctx, strings.TrimSpace(id), user.ID)
	}
	id = strings.TrimSpace(id)
	s.mu.Lock()
	item, ok := s.disputes[id]
	authorized := ok && isDisputeParticipant(item, user.ID)
	s.mu.Unlock()
	if !authorized {
		return DisputeCase{}, disputeNotFound()
	}
	if appErr := s.normalizeExpiredDisputeRemedyMemory(ctx, id); appErr != nil {
		return DisputeCase{}, appErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok = s.disputes[id]
	if !ok || !isDisputeParticipant(item, user.ID) {
		return DisputeCase{}, disputeNotFound()
	}
	return s.disputeDetailMemory(item), nil
}

func (s *Service) DisputeParticipantActionWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input DisputeParticipantActionInput, buildCompletion DisputeParticipantCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if strings.TrimSpace(user.ID) == "" || (user.Status != "" && user.Status != auth.AccountStatusActive) {
		return idempotency.Completion{}, sessionRequired()
	}
	input.ActorUserID = user.ID
	input.DisputeID = strings.TrimSpace(input.DisputeID)
	input.Action = strings.TrimSpace(input.Action)
	input.Body = strings.TrimSpace(input.Body)
	input.Resolution = strings.TrimSpace(input.Resolution)
	input.AmountCNY = strings.TrimSpace(input.AmountCNY)
	input.Terms = strings.TrimSpace(input.Terms)
	input.ProposalID = strings.TrimSpace(input.ProposalID)
	input.Note = strings.TrimSpace(input.Note)
	input.Reason = strings.TrimSpace(input.Reason)
	if appErr := validateDisputeParticipantAction(input); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if input.Action == DisputeMessageActionPropose && s.repo == nil && s.disputeProjectionCloser != nil {
		if appErr := s.disputeProjectionCloser.ValidateDisputeProposalAmount(ctx, input.DisputeID, input.Resolution, input.AmountCNY); appErr != nil {
			return idempotency.Completion{}, appErr
		}
	}
	entry, appErr := s.begin(ctx, user.ID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}
	if s.repo != nil {
		_, completion, appErr := s.repo.UpdateDisputeParticipantWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}
	item, projectionStatus, rollbackMemory, appErr := s.updateDisputeParticipantMemory(input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	if projectionStatus != "" && s.disputeProjectionCloser != nil {
		if appErr := s.disputeProjectionCloser.SetDisputeProjection(ctx, item.ID, projectionStatus, input.ActorUserID, input.RequestID); appErr != nil {
			rollbackMemory()
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
	}
	completion, appErr := buildCompletion(item)
	completion, appErr = s.complete(ctx, entry, completion, appErr)
	if appErr == nil {
		s.notifyDisputeRemedyMemory(item, input.Action)
	}
	return completion, appErr
}

func (s *Service) updateDisputeParticipantMemory(input DisputeParticipantActionInput) (DisputeCase, string, func(), *domain.AppError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.disputes[input.DisputeID]
	if !ok || !isDisputeParticipant(item, input.ActorUserID) || item.TargetType != TargetAPIOrder {
		return DisputeCase{}, "", func() {}, disputeNotFound()
	}
	previousItem := item
	previousMessages := append([]DisputeMessage(nil), s.disputeMessages[item.ID]...)
	previousProposals := append([]SettlementProposal(nil), s.settlementProposals[item.ID]...)
	previousRemedies := append([]DisputeRemedy(nil), s.disputeRemedies[item.ID]...)
	rollback := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.disputes[item.ID] = previousItem
		s.disputeMessages[item.ID] = previousMessages
		s.settlementProposals[item.ID] = previousProposals
		s.disputeRemedies[item.ID] = previousRemedies
	}
	now := s.now()
	projectionStatus := ""
	switch input.Action {
	case DisputeMessageActionAppend:
		if item.Status != DisputeStatusNegotiating && item.Status != DisputeStatusOpen && item.Status != DisputeStatusWaitingInfo {
			return DisputeCase{}, "", func() {}, invalidState("当前纠纷状态不能继续留言。")
		}
		s.disputeMessages[item.ID] = append(s.disputeMessages[item.ID], DisputeMessage{
			ID: uuid.NewString(), DisputeCaseID: item.ID, SenderUserID: input.ActorUserID,
			Body: input.Body, CreatedAt: now,
		})
	case DisputeMessageActionPropose:
		if item.Status != DisputeStatusNegotiating {
			return DisputeCase{}, "", func() {}, invalidState("平台已介入或纠纷已结束，不能再提交协商方案。")
		}
		proposals := s.settlementProposals[item.ID]
		for index := range proposals {
			if proposals[index].Status == SettlementStatusPending {
				proposals[index].Status = SettlementStatusSuperseded
				proposals[index].UpdatedAt = now
				proposals[index].Version++
			}
		}
		proposals = append(proposals, SettlementProposal{
			ID: uuid.NewString(), DisputeCaseID: item.ID, ProposedByUserID: input.ActorUserID,
			Resolution: input.Resolution, AmountCNY: input.AmountCNY, Terms: input.Terms,
			Status: SettlementStatusPending, CreatedAt: now, UpdatedAt: now, Version: 1,
		})
		s.settlementProposals[item.ID] = proposals
	case DisputeMessageActionConfirm, DisputeMessageActionReject:
		if item.Status != DisputeStatusNegotiating {
			return DisputeCase{}, "", func() {}, invalidState("平台已介入或纠纷已结束，不能处理协商方案。")
		}
		proposals := s.settlementProposals[item.ID]
		index := -1
		for candidate := range proposals {
			if proposals[candidate].ID == input.ProposalID {
				index = candidate
				break
			}
		}
		if index < 0 {
			return DisputeCase{}, "", func() {}, disputeNotFound()
		}
		proposal := proposals[index]
		if proposal.Status != SettlementStatusPending || proposal.ProposedByUserID == input.ActorUserID {
			return DisputeCase{}, "", func() {}, invalidState("只能由另一方确认或拒绝当前待确认方案。")
		}
		proposal.UpdatedAt = now
		proposal.Version++
		if input.Action == DisputeMessageActionReject {
			proposal.Status = SettlementStatusRejected
			proposal.RejectedByUserID = input.ActorUserID
			proposal.RejectedAt = &now
		} else {
			proposal.Status = SettlementStatusAccepted
			proposal.AcceptedByUserID = input.ActorUserID
			proposal.AcceptedAt = &now
			item.Status = DisputeStatusClosed
			item.PublicResult = "双方已确认协商方案"
			item.ClosedAt = &now
			projectionStatus = apiorder.DisputeStatusClosed
		}
		proposals[index] = proposal
		s.settlementProposals[item.ID] = proposals
	case DisputeMessageActionEscalate:
		if item.Status != DisputeStatusNegotiating {
			return DisputeCase{}, "", func() {}, invalidState("当前纠纷已由平台处理或已经结案。")
		}
		proposals := s.settlementProposals[item.ID]
		for index := range proposals {
			if proposals[index].Status == SettlementStatusPending {
				proposals[index].Status = SettlementStatusSuperseded
				proposals[index].UpdatedAt = now
				proposals[index].Version++
			}
		}
		s.settlementProposals[item.ID] = proposals
		item.Status = DisputeStatusOpen
		item.PublicResult = "平台审核中"
		projectionStatus = apiorder.DisputeStatusOpen
	case DisputeRemedyActionClaim:
		index := currentRemedyIndex(s.disputeRemedies[item.ID])
		if item.Status != DisputeStatusResolved || index < 0 {
			return DisputeCase{}, "", func() {}, invalidState("当前纠纷没有待履行的整改要求。")
		}
		remedies := s.disputeRemedies[item.ID]
		remedy := remedies[index]
		if remedy.Status != RemedyStatusPending || remedy.ResponsibleUserID != input.ActorUserID {
			return DisputeCase{}, "", func() {}, invalidState("只有整改责任方可以声明已履行。")
		}
		confirmationDueAt := now.Add(RemedyConfirmationWindow)
		remedy.Status = RemedyStatusClaimedFulfilled
		remedy.ClaimNote = input.Note
		remedy.ClaimedAt = &now
		remedy.ConfirmationDueAt = &confirmationDueAt
		remedy.UpdatedAt = now
		remedy.Version++
		remedies[index] = remedy
		s.disputeRemedies[item.ID] = remedies
		item.PublicResult = "责任方已声明履行，等待对方确认"
		projectionStatus = apiorder.DisputeStatusFulfillmentConfirmation
	case DisputeRemedyActionConfirm, DisputeRemedyActionContest:
		index := currentRemedyIndex(s.disputeRemedies[item.ID])
		if item.Status != DisputeStatusResolved || index < 0 {
			return DisputeCase{}, "", func() {}, invalidState("当前纠纷没有待确认的履行声明。")
		}
		remedies := s.disputeRemedies[item.ID]
		remedy := remedies[index]
		if remedy.Status != RemedyStatusClaimedFulfilled || remedy.BeneficiaryUserID != input.ActorUserID {
			return DisputeCase{}, "", func() {}, invalidState("只有整改受益方可以确认或否认履行结果。")
		}
		if remedy.ConfirmationDueAt != nil && !now.Before(*remedy.ConfirmationDueAt) {
			remedy.Status = RemedyStatusConfirmationExpired
			remedy.ResponseNote = RemedyConfirmationExpiredNote
			remedy.ConfirmationExpiredAt = &now
			remedy.UpdatedAt = now
			remedy.Version++
			remedies[index] = remedy
			s.disputeRemedies[item.ID] = remedies
			item.Status = DisputeStatusClosed
			item.PublicResult = RemedyConfirmationExpiredPublicResult
			item.ClosedAt = &now
			projectionStatus = apiorder.DisputeStatusClosed
			break
		}
		remedy.ResponseNote = input.Reason
		remedy.UpdatedAt = now
		remedy.Version++
		if input.Action == DisputeRemedyActionContest {
			remedy.Status = RemedyStatusContested
			remedy.ContestedAt = &now
			item.Status = DisputeStatusOpen
			item.PublicResult = "履行结果有异议，平台重新审核中"
			item.ResolvedAt = nil
			projectionStatus = apiorder.DisputeStatusOpen
		} else {
			remedy.Status = RemedyStatusConfirmed
			remedy.ConfirmedAt = &now
			item.Status = DisputeStatusClosed
			item.PublicResult = "对方已确认整改履行完成"
			item.ClosedAt = &now
			projectionStatus = apiorder.DisputeStatusClosed
		}
		remedies[index] = remedy
		s.disputeRemedies[item.ID] = remedies
	default:
		return DisputeCase{}, "", func() {}, invalidState("纠纷协商动作不支持。")
	}
	item.UpdatedAt = now
	item.Version++
	s.disputes[item.ID] = item
	return s.disputeDetailMemory(item), projectionStatus, rollback, nil
}

func (s *Service) disputeDetailMemory(item DisputeCase) DisputeCase {
	item.Messages = append([]DisputeMessage(nil), s.disputeMessages[item.ID]...)
	proposals := s.settlementProposals[item.ID]
	item.SettlementProposals = make([]SettlementProposal, len(proposals))
	for index := range proposals {
		item.SettlementProposals[len(proposals)-1-index] = proposals[index]
	}
	remedies := s.disputeRemedies[item.ID]
	item.Remedies = make([]DisputeRemedy, len(remedies))
	for index := range remedies {
		item.Remedies[len(remedies)-1-index] = remedies[index]
	}
	return item
}

func currentRemedyIndex(items []DisputeRemedy) int {
	for index := len(items) - 1; index >= 0; index-- {
		if items[index].Status == RemedyStatusPending || items[index].Status == RemedyStatusClaimedFulfilled {
			return index
		}
	}
	return -1
}

func (s *Service) normalizeExpiredDisputeRemedyMemory(ctx context.Context, disputeID string) *domain.AppError {
	now := s.now()
	s.mu.Lock()
	item, ok := s.disputes[disputeID]
	if !ok {
		s.mu.Unlock()
		return nil
	}
	previousItem := item
	previousRemedies := append([]DisputeRemedy(nil), s.disputeRemedies[disputeID]...)
	remedies := s.disputeRemedies[disputeID]
	index := currentRemedyIndex(remedies)
	if item.Status != DisputeStatusResolved || index < 0 || remedies[index].Status != RemedyStatusClaimedFulfilled ||
		remedies[index].ConfirmationDueAt == nil || now.Before(*remedies[index].ConfirmationDueAt) {
		s.mu.Unlock()
		return nil
	}
	remedy := remedies[index]
	remedy.Status = RemedyStatusConfirmationExpired
	remedy.ResponseNote = RemedyConfirmationExpiredNote
	remedy.ConfirmationExpiredAt = &now
	remedy.UpdatedAt = now
	remedy.Version++
	remedies[index] = remedy
	s.disputeRemedies[disputeID] = remedies
	item.Status = DisputeStatusClosed
	item.PublicResult = RemedyConfirmationExpiredPublicResult
	item.ClosedAt = &now
	item.UpdatedAt = now
	item.Version++
	s.disputes[disputeID] = item
	expiredVersion := item.Version
	s.mu.Unlock()

	if s.disputeProjectionCloser == nil {
		item.Remedies = []DisputeRemedy{remedy}
		s.notifyDisputeRemedyMemory(item, "confirmation_expired")
		return nil
	}
	requestID := "remedy-confirmation-timeout:" + remedy.ID
	if appErr := s.disputeProjectionCloser.SetDisputeProjection(ctx, disputeID, apiorder.DisputeStatusClosed, "", requestID); appErr != nil {
		s.mu.Lock()
		if current, exists := s.disputes[disputeID]; exists && current.Version == expiredVersion {
			s.disputes[disputeID] = previousItem
			s.disputeRemedies[disputeID] = previousRemedies
		}
		s.mu.Unlock()
		return appErr
	}
	item.Remedies = []DisputeRemedy{remedy}
	s.notifyDisputeRemedyMemory(item, "confirmation_expired")
	return nil
}

func (s *Service) AdminDispute(ctx context.Context, user auth.User, id string) (DisputeCase, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return DisputeCase{}, appErr
	}
	if s.repo != nil {
		return s.repo.GetAdminDispute(ctx, id)
	}
	if appErr := s.normalizeExpiredDisputeRemedyMemory(ctx, id); appErr != nil {
		return DisputeCase{}, appErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.disputes[id]
	if !ok {
		return DisputeCase{}, disputeNotFound()
	}
	item.Supplements = append([]InfoSupplement(nil), s.infoSupplements[infoSupplementEntityKey(InfoRequestEntityDispute, id)]...)
	return s.disputeDetailMemory(item), nil
}

func (s *Service) AdminDisputeActionWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input AdminActionInput, buildCompletion AdminCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	input.AdminUserID = user.ID
	input.PublicResultCode = normalizePublicResultCode(input.PublicResultCode)
	if appErr := validateDisputeAction(input); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if input.Action == "resolve" && input.Remedy != nil && !input.Remedy.DueAt.After(s.now()) {
		return idempotency.Completion{}, fieldError("remedy.dueAt", "整改期限必须晚于当前时间。")
	}
	entry, appErr := s.begin(ctx, user.ID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}
	if s.repo != nil {
		_, completion, appErr := s.repo.UpdateDisputeAdminWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}
	if input.Action == "resolve" && input.Remedy != nil {
		if s.disputeProjectionCloser != nil {
			if appErr := s.disputeProjectionCloser.ValidateDisputeProposalAmount(ctx, input.ID, input.Remedy.Action, input.Remedy.AmountCNY); appErr != nil {
				return idempotency.Completion{}, appErr
			}
		}
	}
	result, rollbackMemory, appErr := s.updateDisputeAdminMemory(input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	if result.Dispute != nil && result.Dispute.TargetType == TargetAPIOrder && s.disputeProjectionCloser != nil {
		projectionStatus := ""
		switch input.Action {
		case "resolve":
			projectionStatus = apiorder.DisputeStatusClosed
			if input.Remedy != nil {
				projectionStatus = apiorder.DisputeStatusAwaitingFulfillment
			}
		case "close", "mark_overdue":
			projectionStatus = apiorder.DisputeStatusClosed
		}
		if projectionStatus != "" {
			if appErr := s.disputeProjectionCloser.SetDisputeProjection(ctx, result.Dispute.ID, projectionStatus, input.AdminUserID, input.RequestID); appErr != nil {
				rollbackMemory()
				s.idempotency.Cancel(ctx, entry)
				return idempotency.Completion{}, appErr
			}
		}
	}
	completion, appErr := buildCompletion(result)
	completion, appErr = s.complete(ctx, entry, completion, appErr)
	if appErr == nil && result.Dispute != nil {
		s.notifyDisputeRemedyMemory(*result.Dispute, input.Action)
	}
	return completion, appErr
}

func (s *Service) notifyDisputeRemedyMemory(item DisputeCase, action string) {
	if s.notifications == nil || len(item.Remedies) == 0 {
		return
	}
	remedy := item.Remedies[0]
	title := ""
	body := ""
	recipients := []string{}
	eventType := ""
	switch {
	case action == "resolve" && remedy.Status == RemedyStatusPending:
		eventType, title = "dispute.remedy_created", "平台已下达整改要求"
		body = "平台已作出裁决，请按整改要求和期限完成履行。"
		recipients = []string{remedy.ResponsibleUserID, remedy.BeneficiaryUserID}
	case action == DisputeRemedyActionClaim && remedy.Status == RemedyStatusClaimedFulfilled:
		eventType, title = "dispute.remedy_claimed", "整改履行声明待确认"
		body = "责任方已声明履行，请在 48 小时内确认是否收到或完成。"
		recipients = []string{remedy.BeneficiaryUserID}
	case remedy.Status == RemedyStatusConfirmationExpired:
		eventType, title = "dispute.remedy_confirmation_expired", "整改确认期已结束"
		body = "对方未在期限内反馈，流程已中性结案；平台未核验到账或履约事实。"
		recipients = []string{remedy.ResponsibleUserID, remedy.BeneficiaryUserID}
	case action == DisputeRemedyActionConfirm && remedy.Status == RemedyStatusConfirmed:
		eventType, title = "dispute.remedy_confirmed", "整改结果已由对方确认"
		body = "对方已确认整改履行完成，纠纷已结案。"
		recipients = []string{remedy.ResponsibleUserID}
	case action == DisputeRemedyActionContest && remedy.Status == RemedyStatusContested:
		eventType, title = "dispute.remedy_contested", "整改结果已申请平台复核"
		body = "对方反馈未收到或未完成，纠纷已重新进入平台审核。"
		recipients = []string{remedy.ResponsibleUserID}
	case action == "mark_overdue" && remedy.Status == RemedyStatusOverdue:
		eventType, title = "dispute.remedy_overdue", "平台已确认整改逾期"
		body = "平台已确认责任方未在裁决期限内履行，纠纷已结案。"
		recipients = []string{remedy.ResponsibleUserID, remedy.BeneficiaryUserID}
	default:
		return
	}
	seen := make(map[string]bool, len(recipients))
	for _, userID := range recipients {
		userID = strings.TrimSpace(userID)
		if userID == "" || seen[userID] {
			continue
		}
		seen[userID] = true
		s.notifications.Add(notification.Notification{
			UserID: userID, Type: eventType, Title: title, Body: body,
			TargetType: "dispute", TargetID: item.ID,
			TargetURL: "/my/reports/dispute/" + item.ID, SourceEventType: eventType,
		})
	}
}

func (s *Service) SubmitInfoSupplementWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input SupplementInput, buildCompletion SupplementCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if strings.TrimSpace(user.ID) == "" {
		return idempotency.Completion{}, sessionRequired()
	}
	if user.Status != "" && user.Status != auth.AccountStatusActive {
		return idempotency.Completion{}, sessionRequired()
	}
	input.SubmittingUserID = user.ID
	input.SubmittingUsername = user.Username
	input.SubmittingName = displayName(user)
	input.EntityType = normalize(input.EntityType)
	input.EntityID = strings.TrimSpace(input.EntityID)
	input.InfoRequestID = strings.TrimSpace(input.InfoRequestID)
	input.Body = strings.TrimSpace(input.Body)
	if appErr := validateSupplement(input); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	entry, appErr := s.begin(ctx, user.ID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}
	if s.repo != nil {
		_, completion, appErr := s.repo.SubmitInfoSupplementWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}
	result, requestedByAdminID, appErr := s.submitInfoSupplementMemory(input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	if s.notifications != nil {
		s.notifications.Add(notification.Notification{
			UserID:          requestedByAdminID,
			Type:            "案件补充材料",
			Title:           "用户已补充案件材料",
			Body:            "用户已提交脱敏补充说明，请重新查看案件。",
			TargetType:      input.EntityType,
			TargetID:        input.EntityID,
			TargetURL:       "/admin/reports",
			SourceEventType: "moderation.info_supplemented",
		})
	}
	completion, appErr := buildCompletion(result)
	return s.complete(ctx, entry, completion, appErr)
}

func (s *Service) CreateAppealWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input CreateAppealInput, buildCompletion AppealCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	input.AppellantUserID = user.ID
	input.AppellantUsername = user.Username
	input.AppellantName = displayName(user)
	if appErr := validateCreateAppeal(input); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	entry, appErr := s.begin(ctx, user.ID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}
	if s.repo != nil {
		_, completion, appErr := s.repo.CreateAppealWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}
	item, appErr := s.createAppealMemory(input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(item)
	return s.complete(ctx, entry, completion, appErr)
}

func (s *Service) CreateAccountGovernanceAppealWithIdempotency(ctx context.Context, appellantUserID, routeKey, key, requestHash string, input CreateAccountGovernanceAppealInput, buildCompletion AppealCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	input.AppellantUserID = strings.TrimSpace(appellantUserID)
	input.Statement = strings.TrimSpace(input.Statement)
	if appErr := validateCreateAccountGovernanceAppeal(input); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if s.repo == nil {
		return idempotency.Completion{}, accountGovernanceAppealUnavailable()
	}
	entry, appErr := s.begin(ctx, input.AppellantUserID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}
	_, completion, appErr := s.repo.CreateAccountGovernanceAppealWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	return completion, nil
}

func (s *Service) MyAppeals(ctx context.Context, user auth.User) ([]Appeal, *domain.AppError) {
	if strings.TrimSpace(user.ID) == "" {
		return nil, sessionRequired()
	}
	if s.repo != nil {
		return s.repo.ListAppealsByUser(ctx, user.ID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]Appeal, 0)
	for _, item := range s.appeals {
		if item.AppellantUserID == user.ID {
			items = append(items, item)
		}
	}
	sortAppeals(items)
	return items, nil
}

func (s *Service) AdminAppeals(ctx context.Context, user auth.User) ([]Appeal, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return nil, appErr
	}
	if s.repo != nil {
		return s.repo.ListAdminAppeals(ctx)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]Appeal, 0, len(s.appeals))
	for _, item := range s.appeals {
		items = append(items, item)
	}
	sortAppeals(items)
	return items, nil
}

func (s *Service) AdminAppeal(ctx context.Context, user auth.User, id string) (Appeal, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return Appeal{}, appErr
	}
	if s.repo != nil {
		return s.repo.GetAdminAppeal(ctx, id)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.appeals[id]
	if !ok {
		return Appeal{}, appealNotFound()
	}
	return item, nil
}

func (s *Service) AdminAppealActionWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input AdminActionInput, buildCompletion AdminCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	input.AdminUserID = user.ID
	if appErr := validateAppealAction(input); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	entry, appErr := s.begin(ctx, user.ID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}
	if s.repo != nil {
		_, completion, appErr := s.repo.UpdateAppealAdminWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}
	result, appErr := s.updateAppealAdminMemory(input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(result)
	return s.complete(ctx, entry, completion, appErr)
}

func (s *Service) PublicUserDisputes(ctx context.Context, username string) ([]PublicDispute, *domain.AppError) {
	if strings.TrimSpace(username) == "" {
		return nil, publicProfileNotFound()
	}
	if s.repo != nil {
		return s.repo.ListPublicUserDisputes(ctx, username)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]PublicDispute, 0)
	for _, dispute := range s.disputes {
		if !matchesUsername(dispute.PrimaryUsername, username) && !matchesUsername(dispute.CounterpartyUsername, username) {
			continue
		}
		items = append(items, publicDisputeFromCase(dispute, username))
	}
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].HandledAt.After(items[j].HandledAt)
	})
	return items, nil
}

func (s *Service) PublicUserDisputeStats(ctx context.Context, username string) (PublicStats, *domain.AppError) {
	if strings.TrimSpace(username) == "" {
		return PublicStats{}, publicProfileNotFound()
	}
	if s.repo != nil {
		return s.repo.PublicUserDisputeStats(ctx, username, s.now())
	}
	items, appErr := s.PublicUserDisputes(ctx, username)
	if appErr != nil {
		return PublicStats{}, appErr
	}
	stats := PublicStats{}
	cutoff := s.now().AddDate(0, 0, -90)
	for _, item := range items {
		if item.Unresolved {
			stats.UnresolvedCount++
			continue
		}
		if !item.HandledAt.Before(cutoff) {
			stats.ResolvedLast90Days++
		}
	}
	return stats, nil
}

func (s *Service) RegisterAPIOrderDispute(ctx context.Context, input apiorder.DisputeCaseInput) (string, *domain.AppError) {
	_ = ctx
	if strings.TrimSpace(input.OrderID) == "" {
		return "", fieldError("orderId", "必须提供订单。")
	}
	if strings.TrimSpace(input.ActorUserID) == "" {
		return "", sessionRequired()
	}
	now := input.Now
	if now.IsZero() {
		now = s.now()
	}
	counterpartyID := input.SellerUserID
	if input.ActorUserID == input.SellerUserID {
		counterpartyID = input.BuyerUserID
	}
	item := DisputeCase{
		ID:                  uuid.NewString(),
		TargetType:          TargetAPIOrder,
		TargetID:            strings.TrimSpace(input.OrderID),
		TargetLabel:         nonEmpty(input.ServiceTitle, "API 订单"),
		PrimaryUserID:       strings.TrimSpace(input.ActorUserID),
		CounterpartyUserID:  strings.TrimSpace(counterpartyID),
		SubjectUserID:       strings.TrimSpace(counterpartyID),
		Status:              DisputeStatusNegotiating,
		IssueCode:           strings.TrimSpace(input.IssueCode),
		RequestedResolution: strings.TrimSpace(input.RequestedResolution),
		RequestedAmountCNY:  strings.TrimSpace(input.RequestedAmountCNY),
		PublicSummary:       "API 订单纠纷",
		PublicResultCode:    PublicResultNoAction,
		PublicResult:        "双方协商中",
		OpenedByAdminID:     strings.TrimSpace(input.ActorUserID),
		OpenedAt:            now,
		CreatedAt:           now,
		UpdatedAt:           now,
		Version:             1,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.disputes[item.ID] = item
	s.disputeMessages[item.ID] = []DisputeMessage{{
		ID: uuid.NewString(), DisputeCaseID: item.ID, SenderUserID: input.ActorUserID,
		Body: strings.TrimSpace(input.Reason), CreatedAt: now,
	}}
	return item.ID, nil
}

func (s *Service) createReportMemory(input CreateReportInput) (Report, *domain.AppError) {
	now := s.now()
	reportedUsername := normalizeUsername(input.ReportedUsername)
	if input.TargetType == TargetPublicUser {
		if reportedUsername == "" {
			reportedUsername = normalizeUsername(input.TargetID)
		}
		if reportedUsername != "" && matchesUsername(reportedUsername, input.ReporterUsername) {
			return Report{}, selfReportForbidden()
		}
	}
	item := Report{
		ID:                  uuid.NewString(),
		ReporterUserID:      input.ReporterUserID,
		ReporterUsername:    input.ReporterUsername,
		ReporterName:        input.ReporterName,
		TargetType:          strings.TrimSpace(input.TargetType),
		TargetID:            strings.TrimSpace(input.TargetID),
		CanonicalTargetType: strings.TrimSpace(input.TargetType),
		CanonicalTargetID:   strings.TrimSpace(input.TargetID),
		TargetLabel:         strings.TrimSpace(input.TargetLabel),
		TargetSnapshotJSON:  "{}",
		ReportedUsername:    reportedUsername,
		ReasonCode:          normalizeReason(input.ReasonCode),
		Title:               strings.TrimSpace(input.Title),
		Description:         strings.TrimSpace(input.Description),
		Status:              ReportStatusSubmitted,
		CreatedAt:           now,
		UpdatedAt:           now,
		Version:             1,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.reports {
		if existing.ReporterUserID == item.ReporterUserID &&
			existing.CanonicalTargetType == item.CanonicalTargetType &&
			existing.CanonicalTargetID == item.CanonicalTargetID &&
			isActiveReportStatus(existing.Status) {
			return Report{}, activeReportExists()
		}
	}
	s.reports[item.ID] = item
	return item, nil
}

func (s *Service) updateReportAdminMemory(input AdminActionInput) (MutationResult, *domain.AppError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.reports[input.ID]
	if !ok {
		return MutationResult{}, reportNotFound()
	}
	if input.ExpectedVersion > 0 && item.Version != input.ExpectedVersion {
		return MutationResult{}, versionConflict()
	}
	now := s.now()
	switch input.Action {
	case "triage":
		if item.Status != ReportStatusSubmitted {
			return MutationResult{}, invalidState("只有新提交的举报可以标记分诊。")
		}
		item.Status = ReportStatusTriaged
	case "request_info":
		if item.Status != ReportStatusSubmitted && item.Status != ReportStatusTriaged {
			return MutationResult{}, invalidState("只有新提交或已分诊的举报可以要求补充信息。")
		}
		if input.RequestedFromID != item.ReporterUserID {
			return MutationResult{}, infoRequestPermissionDenied()
		}
		item.Status = ReportStatusNeedsInfo
	case "reject":
		if !canFinishReport(item.Status) {
			return MutationResult{}, invalidState("当前举报不能拒绝。")
		}
		item.Status = ReportStatusRejected
	case "close":
		if !canFinishReport(item.Status) {
			return MutationResult{}, invalidState("当前举报不能关闭。")
		}
		item.Status = ReportStatusClosed
	case "open_dispute":
		if !canOpenDisputeFromReport(item.Status) {
			return MutationResult{}, invalidState("当前举报不能打开纠纷。")
		}
		dispute := DisputeCase{
			ID:                   uuid.NewString(),
			ReportID:             item.ID,
			TargetType:           nonEmpty(item.CanonicalTargetType, item.TargetType),
			TargetID:             nonEmpty(item.CanonicalTargetID, item.TargetID),
			TargetLabel:          nonEmpty(input.PublicSummary, item.TargetLabel, item.Title),
			PrimaryUserID:        item.ReporterUserID,
			PrimaryUsername:      item.ReporterUsername,
			PrimaryDisplayName:   item.ReporterName,
			CounterpartyUsername: item.ReportedUsername,
			Status:               DisputeStatusOpen,
			PublicSummary:        nonEmpty(input.PublicSummary, item.Title),
			PublicResultCode:     nonEmpty(normalizePublicResultCode(input.PublicResultCode), PublicResultNoAction),
			PublicResult:         nonEmpty(input.PublicResult, "已进入人工处理中"),
			AdminReason:          strings.TrimSpace(input.Reason),
			OpenedByAdminID:      input.AdminUserID,
			OpenedAt:             now,
			CreatedAt:            now,
			UpdatedAt:            now,
			Version:              1,
		}
		s.disputes[dispute.ID] = dispute
		item.Status = ReportStatusDisputeOpened
		item.DisputeID = dispute.ID
		item.HandledByAdminID = input.AdminUserID
		item.HandledAt = &now
		item.AdminReason = strings.TrimSpace(input.Reason)
		item.UpdatedAt = now
		item.Version++
		s.cancelInfoRequestMemory(InfoRequestEntityReport, item.ID, now)
		item.OpenInfoRequestID = ""
		item.InfoRequestedFromID = ""
		s.reports[item.ID] = item
		return MutationResult{Report: &item, Dispute: &dispute}, nil
	default:
		return MutationResult{}, invalidState("举报处理动作不支持。")
	}
	item.AdminReason = strings.TrimSpace(input.Reason)
	item.HandledByAdminID = input.AdminUserID
	item.HandledAt = &now
	item.UpdatedAt = now
	item.Version++
	if input.Action == "request_info" {
		request := s.createInfoRequestMemory(InfoRequestEntityReport, item.ID, input.RequestedFromID, input.AdminUserID, input.Reason, now)
		item.OpenInfoRequestID = request.ID
		item.InfoRequestedFromID = request.RequestedFromID
		if s.notifications != nil {
			s.notifications.Add(notification.Notification{
				UserID: request.RequestedFromID, Type: "案件补充材料", Title: "平台需要你补充案件材料",
				Body: "请提交脱敏事实说明，不要包含联系方式或任何凭据。", TargetType: InfoRequestEntityReport,
				TargetID: item.ID, TargetURL: "/my/reports/report/" + item.ID, SourceEventType: "moderation.info_requested",
			})
		}
	} else {
		s.cancelInfoRequestMemory(InfoRequestEntityReport, item.ID, now)
		item.OpenInfoRequestID = ""
		item.InfoRequestedFromID = ""
	}
	s.reports[item.ID] = item
	return MutationResult{Report: &item}, nil
}

func (s *Service) updateDisputeAdminMemory(input AdminActionInput) (MutationResult, func(), *domain.AppError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.disputes[input.ID]
	if !ok {
		return MutationResult{}, func() {}, disputeNotFound()
	}
	if input.ExpectedVersion > 0 && item.Version != input.ExpectedVersion {
		return MutationResult{}, func() {}, versionConflict()
	}
	previousItem := item
	previousRemedies := append([]DisputeRemedy(nil), s.disputeRemedies[item.ID]...)
	rollback := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.disputes[item.ID] = previousItem
		s.disputeRemedies[item.ID] = previousRemedies
	}
	now := s.now()
	switch input.Action {
	case "request_info":
		if item.Status != DisputeStatusOpen {
			return MutationResult{}, func() {}, invalidState("只有打开中的纠纷可以要求补充信息。")
		}
		if !isDisputeParticipant(item, input.RequestedFromID) {
			return MutationResult{}, func() {}, infoRequestPermissionDenied()
		}
		item.Status = DisputeStatusWaitingInfo
	case "resolve":
		if item.Status != DisputeStatusOpen && item.Status != DisputeStatusWaitingInfo {
			return MutationResult{}, func() {}, invalidState("当前纠纷不能标记处理完成。")
		}
		item.Status = DisputeStatusResolved
		if item.TargetType == TargetAPIOrder && input.Remedy == nil {
			item.Status = DisputeStatusClosed
			item.ClosedAt = &now
		}
		item.ResolvedAt = &now
		if input.Remedy != nil {
			if item.TargetType != TargetAPIOrder || !isDisputeParticipant(item, input.Remedy.ResponsibleUserID) {
				return MutationResult{}, func() {}, invalidState("整改责任方必须是当前 API 订单纠纷参与者。")
			}
			beneficiaryID := item.PrimaryUserID
			if beneficiaryID == input.Remedy.ResponsibleUserID {
				beneficiaryID = item.CounterpartyUserID
			}
			if beneficiaryID == "" || beneficiaryID == input.Remedy.ResponsibleUserID {
				return MutationResult{}, func() {}, invalidState("整改要求缺少有效受益方。")
			}
			s.disputeRemedies[item.ID] = append(s.disputeRemedies[item.ID], DisputeRemedy{
				ID: uuid.NewString(), DisputeCaseID: item.ID, Action: input.Remedy.Action,
				AmountCNY: input.Remedy.AmountCNY, Currency: "CNY",
				ResponsibleUserID: input.Remedy.ResponsibleUserID, BeneficiaryUserID: beneficiaryID,
				Instructions: input.Remedy.Instructions, Status: RemedyStatusPending,
				DueAt: input.Remedy.DueAt, CreatedByAdminID: input.AdminUserID,
				CreatedAt: now, UpdatedAt: now, Version: 1,
			})
		}
	case "close":
		if item.Status == DisputeStatusClosed {
			return MutationResult{}, func() {}, invalidState("纠纷已关闭。")
		}
		if item.TargetType == TargetAPIOrder && currentRemedyIndex(s.disputeRemedies[item.ID]) >= 0 {
			return MutationResult{}, func() {}, invalidState("当前纠纷仍有进行中的整改要求，不能直接关闭。")
		}
		item.Status = DisputeStatusClosed
		item.ClosedAt = &now
	case "mark_overdue":
		index := currentRemedyIndex(s.disputeRemedies[item.ID])
		if item.Status != DisputeStatusResolved || index < 0 {
			return MutationResult{}, func() {}, invalidState("当前纠纷没有可确认逾期的整改要求。")
		}
		remedies := s.disputeRemedies[item.ID]
		remedy := remedies[index]
		if remedy.Status != RemedyStatusPending || now.Before(remedy.DueAt) {
			return MutationResult{}, func() {}, invalidState("整改尚未到期或已提交履行声明。")
		}
		remedy.Status = RemedyStatusOverdue
		remedy.ResponseNote = input.Reason
		remedy.OverdueAt = &now
		remedy.UpdatedAt = now
		remedy.Version++
		remedies[index] = remedy
		s.disputeRemedies[item.ID] = remedies
		item.Status = DisputeStatusClosed
		item.PublicResult = "责任方未在裁决期限内履行"
		item.ClosedAt = &now
	default:
		return MutationResult{}, func() {}, invalidState("纠纷处理动作不支持。")
	}
	item.AdminReason = strings.TrimSpace(input.Reason)
	item.PublicSummary = nonEmpty(input.PublicSummary, item.PublicSummary)
	item.PublicResultCode = nonEmpty(normalizePublicResultCode(input.PublicResultCode), item.PublicResultCode, PublicResultNoAction)
	item.PublicResult = nonEmpty(input.PublicResult, item.PublicResult)
	item.UpdatedAt = now
	item.Version++
	if input.Action == "request_info" {
		request := s.createInfoRequestMemory(InfoRequestEntityDispute, item.ID, input.RequestedFromID, input.AdminUserID, input.Reason, now)
		item.OpenInfoRequestID = request.ID
		item.InfoRequestedFromID = request.RequestedFromID
		if s.notifications != nil {
			s.notifications.Add(notification.Notification{
				UserID: request.RequestedFromID, Type: "案件补充材料", Title: "平台需要你补充案件材料",
				Body: "请提交脱敏事实说明，不要包含联系方式或任何凭据。", TargetType: InfoRequestEntityDispute,
				TargetID: item.ID, TargetURL: "/my/reports/dispute/" + item.ID, SourceEventType: "moderation.info_requested",
			})
		}
	} else {
		s.cancelInfoRequestMemory(InfoRequestEntityDispute, item.ID, now)
		item.OpenInfoRequestID = ""
		item.InfoRequestedFromID = ""
	}
	s.disputes[item.ID] = item
	detail := s.disputeDetailMemory(item)
	return MutationResult{Dispute: &detail}, rollback, nil
}

func (s *Service) createInfoRequestMemory(entityType, entityID, requestedFromID, adminID, reason string, now time.Time) InfoRequest {
	item := InfoRequest{
		ID: uuid.NewString(), EntityType: entityType, EntityID: entityID, RequestedFromID: requestedFromID,
		RequestedByAdminID: adminID, InternalReason: strings.TrimSpace(reason), Status: InfoRequestStatusOpen, RequestedAt: now,
	}
	s.infoRequests[item.ID] = item
	return item
}

func (s *Service) cancelInfoRequestMemory(entityType, entityID string, now time.Time) {
	for id, item := range s.infoRequests {
		if item.EntityType == entityType && item.EntityID == entityID && item.Status == InfoRequestStatusOpen {
			item.Status = InfoRequestStatusCanceled
			item.CancelledAt = &now
			s.infoRequests[id] = item
		}
	}
}

func (s *Service) submitInfoSupplementMemory(input SupplementInput) (MutationResult, string, *domain.AppError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	request, ok := s.infoRequests[input.InfoRequestID]
	if !ok || request.EntityType != input.EntityType || request.EntityID != input.EntityID || request.RequestedFromID != input.SubmittingUserID {
		return MutationResult{}, "", infoRequestNotFound()
	}
	if request.Status != InfoRequestStatusOpen {
		return MutationResult{}, "", invalidState("该补充请求已处理，不能重复提交。")
	}
	now := s.now()
	switch input.EntityType {
	case InfoRequestEntityReport:
		item, ok := s.reports[input.EntityID]
		if !ok || item.ReporterUserID != input.SubmittingUserID || item.Status != ReportStatusNeedsInfo {
			return MutationResult{}, "", infoRequestNotFound()
		}
		request.Status = InfoRequestStatusAnswered
		request.AnsweredAt = &now
		s.infoRequests[request.ID] = request
		item.OpenInfoRequestID = ""
		item.InfoRequestedFromID = ""
		item.UpdatedAt = now
		item.Version++
		s.reports[item.ID] = item
		s.recordInfoSupplementMemory(input, now)
		return MutationResult{Report: &item}, request.RequestedByAdminID, nil
	case InfoRequestEntityDispute:
		item, ok := s.disputes[input.EntityID]
		if !ok || !isDisputeParticipant(item, input.SubmittingUserID) || item.Status != DisputeStatusWaitingInfo {
			return MutationResult{}, "", infoRequestNotFound()
		}
		request.Status = InfoRequestStatusAnswered
		request.AnsweredAt = &now
		s.infoRequests[request.ID] = request
		item.OpenInfoRequestID = ""
		item.InfoRequestedFromID = ""
		item.UpdatedAt = now
		item.Version++
		s.disputes[item.ID] = item
		s.recordInfoSupplementMemory(input, now)
		return MutationResult{Dispute: &item}, request.RequestedByAdminID, nil
	default:
		return MutationResult{}, "", infoRequestNotFound()
	}
}

func infoSupplementEntityKey(entityType, entityID string) string {
	return entityType + ":" + entityID
}

func (s *Service) recordInfoSupplementMemory(input SupplementInput, now time.Time) {
	key := infoSupplementEntityKey(input.EntityType, input.EntityID)
	s.infoSupplements[key] = append(s.infoSupplements[key], InfoSupplement{
		ID:                  uuid.NewString(),
		InfoRequestID:       input.InfoRequestID,
		SubmittedByUserID:   input.SubmittingUserID,
		SubmittedByUsername: input.SubmittingUsername,
		SubmittedByName:     input.SubmittingName,
		Body:                strings.TrimSpace(input.Body),
		CreatedAt:           now,
	})
}

func (s *Service) createAppealMemory(input CreateAppealInput) (Appeal, *domain.AppError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var sourceReport *Report
	var sourceDispute *DisputeCase
	reportID := strings.TrimSpace(input.ReportID)
	disputeID := strings.TrimSpace(input.DisputeID)
	if reportID != "" {
		item, ok := s.reports[reportID]
		if !ok {
			return Appeal{}, appealSourceNotFound()
		}
		sourceReport = &item
	}
	if disputeID != "" {
		item, ok := s.disputes[disputeID]
		if !ok {
			return Appeal{}, appealSourceNotFound()
		}
		sourceDispute = &item
	}
	source, appErr := ResolveAppealSource(input.AppellantUserID, sourceReport, sourceDispute)
	if appErr != nil {
		return Appeal{}, appErr
	}
	for _, existing := range s.appeals {
		if existing.AppellantUserID != input.AppellantUserID || existing.Status != AppealStatusSubmitted {
			continue
		}
		sameSource := disputeID != "" && existing.DisputeID == disputeID
		if disputeID == "" {
			sameSource = existing.DisputeID == "" && existing.ReportID == reportID
		}
		if appErr := ValidateNoSubmittedAppeal(sameSource); appErr != nil {
			return Appeal{}, appErr
		}
	}
	now := s.now()
	item := Appeal{
		ID:                uuid.NewString(),
		AppellantUserID:   input.AppellantUserID,
		AppellantUsername: input.AppellantUsername,
		AppellantName:     input.AppellantName,
		ReportID:          reportID,
		DisputeID:         disputeID,
		TargetType:        source.TargetType,
		TargetID:          source.TargetID,
		Title:             strings.TrimSpace(input.Title),
		Statement:         strings.TrimSpace(input.Statement),
		Status:            AppealStatusSubmitted,
		CreatedAt:         now,
		UpdatedAt:         now,
		Version:           1,
	}
	s.appeals[item.ID] = item
	return item, nil
}

func (s *Service) updateAppealAdminMemory(input AdminActionInput) (MutationResult, *domain.AppError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.appeals[input.ID]
	if !ok {
		return MutationResult{}, appealNotFound()
	}
	if input.ExpectedVersion > 0 && item.Version != input.ExpectedVersion {
		return MutationResult{}, versionConflict()
	}
	if item.Status != AppealStatusSubmitted {
		return MutationResult{}, invalidState("只有待处理申诉可以审核。")
	}
	now := s.now()
	switch input.Action {
	case "approve":
		item.Status = AppealStatusApproved
	case "reject":
		item.Status = AppealStatusRejected
	default:
		return MutationResult{}, invalidState("申诉处理动作不支持。")
	}
	item.AdminReason = strings.TrimSpace(input.Reason)
	item.HandledByAdminID = input.AdminUserID
	item.HandledAt = &now
	item.UpdatedAt = now
	item.Version++
	s.appeals[item.ID] = item
	return MutationResult{Appeal: &item}, nil
}

func (s *Service) begin(ctx context.Context, userID, routeKey, key, requestHash string) (*idempotency.Entry, *domain.AppError) {
	if strings.TrimSpace(userID) == "" {
		return nil, sessionRequired()
	}
	if err := idempotency.ValidateKey(key); err != nil {
		return nil, err
	}
	return s.idempotency.Begin(ctx, userID, routeKey, key, requestHash)
}

func (s *Service) complete(ctx context.Context, entry *idempotency.Entry, completion idempotency.Completion, appErr *domain.AppError) (idempotency.Completion, *domain.AppError) {
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

func validateCreateReport(input CreateReportInput) *domain.AppError {
	if !validTargets[normalize(input.TargetType)] {
		return fieldError("targetType", "举报目标类型不支持。")
	}
	if strings.TrimSpace(input.TargetID) == "" {
		return fieldError("targetId", "必须提供举报目标。")
	}
	if !validReasons[normalizeReason(input.ReasonCode)] {
		return fieldError("reasonCode", "举报原因不支持。")
	}
	if input.TargetType == TargetPublicUser && normalizeUsername(input.ReportedUsername) == "" {
		return fieldError("reportedUsername", "公开主页举报必须提供被举报用户名。")
	}
	if appErr := validateText("title", input.Title, 2, 80, "举报标题需为 2 至 80 个字符。"); appErr != nil {
		return appErr
	}
	if appErr := validateText("description", input.Description, 4, 1000, "举报说明需为 4 至 1000 个字符。"); appErr != nil {
		return appErr
	}
	if appErr := validateOptionalText("targetLabel", input.TargetLabel, 120); appErr != nil {
		return appErr
	}
	return nil
}

func validateCreateAppeal(input CreateAppealInput) *domain.AppError {
	if strings.TrimSpace(input.ReportID) == "" && strings.TrimSpace(input.DisputeID) == "" {
		return fieldError("targetId", "申诉必须关联举报或纠纷。")
	}
	if appErr := validateText("title", input.Title, 2, 80, "申诉标题需为 2 至 80 个字符。"); appErr != nil {
		return appErr
	}
	if appErr := validateText("statement", input.Statement, 4, 1000, "申诉说明需为 4 至 1000 个字符。"); appErr != nil {
		return appErr
	}
	return nil
}

func validateCreateAccountGovernanceAppeal(input CreateAccountGovernanceAppealInput) *domain.AppError {
	if strings.TrimSpace(input.AppellantUserID) == "" {
		return sessionRequired()
	}
	return validateText("statement", input.Statement, 4, 1000, "申诉说明需为 4 至 1000 个字符。")
}

func validateReportAction(input AdminActionInput) *domain.AppError {
	switch input.Action {
	case "triage", "request_info", "reject", "open_dispute", "close":
	default:
		return invalidState("举报处理动作不支持。")
	}
	if strings.TrimSpace(input.Reason) == "" {
		return fieldError("reason", "必须填写处理原因。")
	}
	if input.Action == "request_info" && strings.TrimSpace(input.RequestedFromID) == "" {
		return fieldError("requestedFromUserId", "必须指定需要补充信息的用户。")
	}
	if input.Action == "open_dispute" {
		if appErr := validateText("publicSummary", input.PublicSummary, 2, 120, "公开纠纷摘要需为 2 至 120 个字符。"); appErr != nil {
			return appErr
		}
		if appErr := validateText("publicResult", nonEmpty(input.PublicResult, "已进入人工处理中"), 2, 120, "公开处理结果需为 2 至 120 个字符。"); appErr != nil {
			return appErr
		}
		if strings.TrimSpace(input.PublicResultCode) != "" && !validPublicResultCodes[normalizePublicResultCode(input.PublicResultCode)] {
			return fieldError("publicResultCode", "公开结果代码不支持。")
		}
	}
	return validateText("reason", input.Reason, 2, 800, "处理原因需为 2 至 800 个字符。")
}

func validateDisputeAction(input AdminActionInput) *domain.AppError {
	switch input.Action {
	case "request_info", "resolve", "close", "mark_overdue":
	default:
		return invalidState("纠纷处理动作不支持。")
	}
	if strings.TrimSpace(input.Reason) == "" {
		return fieldError("reason", "必须填写处理原因。")
	}
	if input.Action == "request_info" && strings.TrimSpace(input.RequestedFromID) == "" {
		return fieldError("requestedFromUserId", "必须指定需要补充信息的案件参与者。")
	}
	if appErr := validateText("reason", input.Reason, 2, 800, "处理原因需为 2 至 800 个字符。"); appErr != nil {
		return appErr
	}
	if input.PublicSummary != "" {
		if appErr := validateText("publicSummary", input.PublicSummary, 2, 120, "公开纠纷摘要需为 2 至 120 个字符。"); appErr != nil {
			return appErr
		}
	}
	if input.PublicResult != "" {
		if appErr := validateText("publicResult", input.PublicResult, 2, 120, "公开处理结果需为 2 至 120 个字符。"); appErr != nil {
			return appErr
		}
	}
	if input.Action == "resolve" && strings.TrimSpace(input.PublicResultCode) == "" {
		return fieldError("publicResultCode", "处理完成必须选择公开结果代码。")
	}
	if input.Remedy != nil {
		if input.Action != "resolve" {
			return fieldError("remedy", "只有平台裁决可以创建整改要求。")
		}
		input.Remedy.Action = strings.TrimSpace(input.Remedy.Action)
		input.Remedy.AmountCNY = strings.TrimSpace(input.Remedy.AmountCNY)
		input.Remedy.ResponsibleUserID = strings.TrimSpace(input.Remedy.ResponsibleUserID)
		input.Remedy.Instructions = strings.TrimSpace(input.Remedy.Instructions)
		if !apiorder.IsDisputeResolution(input.Remedy.Action) {
			return fieldError("remedy.action", "请选择有效的整改动作。")
		}
		if appErr := apiorder.ValidateRequestedDisputeAmount(input.Remedy.Action, input.Remedy.AmountCNY, ""); appErr != nil {
			return appErr
		}
		if input.Remedy.ResponsibleUserID == "" {
			return fieldError("remedy.responsibleUserId", "必须指定整改责任方。")
		}
		if input.Remedy.DueAt.IsZero() {
			return fieldError("remedy.dueAt", "必须填写整改期限。")
		}
		if appErr := validateDisputeParticipantText("remedy.instructions", input.Remedy.Instructions, 2, 2000, "整改说明需为 2 至 2000 个字符。"); appErr != nil {
			return appErr
		}
	}
	if strings.TrimSpace(input.PublicResultCode) != "" && !validPublicResultCodes[normalizePublicResultCode(input.PublicResultCode)] {
		return fieldError("publicResultCode", "公开结果代码不支持。")
	}
	return nil
}

func validateDisputeParticipantAction(input DisputeParticipantActionInput) *domain.AppError {
	if input.DisputeID == "" {
		return fieldError("disputeId", "必须提供纠纷记录。")
	}
	switch input.Action {
	case DisputeMessageActionAppend:
		return validateDisputeParticipantText("body", input.Body, 1, 2000, "留言需为 1 至 2000 个字符。")
	case DisputeMessageActionPropose:
		if !apiorder.IsDisputeResolution(input.Resolution) {
			return fieldError("resolution", "请选择有效的协商处理方案。")
		}
		if appErr := apiorder.ValidateRequestedDisputeAmount(input.Resolution, input.AmountCNY, ""); appErr != nil {
			return appErr
		}
		return validateDisputeParticipantText("terms", input.Terms, 1, 2000, "方案说明需为 1 至 2000 个字符。")
	case DisputeMessageActionConfirm:
		if input.ProposalID == "" {
			return fieldError("proposalId", "必须提供待确认方案。")
		}
		return nil
	case DisputeMessageActionReject:
		if input.ProposalID == "" {
			return fieldError("proposalId", "必须提供待拒绝方案。")
		}
		if input.Reason == "" {
			return nil
		}
		return validateDisputeParticipantText("reason", input.Reason, 1, 500, "拒绝说明不能超过 500 个字符。")
	case DisputeMessageActionEscalate:
		return validateDisputeParticipantText("reason", input.Reason, 2, 500, "平台介入原因需为 2 至 500 个字符。")
	case DisputeRemedyActionClaim:
		return validateDisputeParticipantText("note", input.Note, 2, 2000, "履行说明需为 2 至 2000 个字符。")
	case DisputeRemedyActionConfirm:
		if input.Reason == "" {
			return nil
		}
		return validateDisputeParticipantText("reason", input.Reason, 2, 500, "确认说明需为 2 至 500 个字符。")
	case DisputeRemedyActionContest:
		return validateDisputeParticipantText("reason", input.Reason, 2, 2000, "未收到或未履行说明需为 2 至 2000 个字符。")
	default:
		return invalidState("纠纷参与方动作不支持。")
	}
}

func validateDisputeParticipantText(field, value string, min, max int, detail string) *domain.AppError {
	count := utf8.RuneCountInString(strings.TrimSpace(value))
	if count < min || count > max {
		return fieldError(field, detail)
	}
	if strings.ContainsAny(value, "\x00") {
		return fieldError(field, "文本内容包含非法字符。")
	}
	if domain.LooksLikeSecretContent(value) {
		return secretError(field)
	}
	return nil
}

func validateSupplement(input SupplementInput) *domain.AppError {
	if input.EntityType != InfoRequestEntityReport && input.EntityType != InfoRequestEntityDispute {
		return fieldError("entityType", "补充材料类型不支持。")
	}
	if input.EntityID == "" {
		return fieldError("entityId", "必须提供案件记录。")
	}
	if input.InfoRequestID == "" {
		return fieldError("openInfoRequestId", "补充请求已失效，请刷新后重试。")
	}
	return validateText("body", input.Body, 4, 1200, "补充说明需为 4 至 1200 个字符。")
}

func validateAppealAction(input AdminActionInput) *domain.AppError {
	if input.Action != "approve" && input.Action != "reject" {
		return invalidState("申诉处理动作不支持。")
	}
	if strings.TrimSpace(input.Reason) == "" {
		return fieldError("reason", "必须填写处理原因。")
	}
	return validateText("reason", input.Reason, 2, 800, "处理原因需为 2 至 800 个字符。")
}

func validateText(field, value string, min, max int, detail string) *domain.AppError {
	value = strings.TrimSpace(value)
	count := utf8.RuneCountInString(value)
	if count < min || count > max {
		return fieldError(field, detail)
	}
	if strings.ContainsAny(value, "\x00") {
		return fieldError(field, "文本内容包含非法字符。")
	}
	if looksLikeContact(value) {
		return contactContentError(field)
	}
	if looksLikeSecret(value) {
		return secretError(field)
	}
	return nil
}

func validateOptionalText(field, value string, max int) *domain.AppError {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	count := utf8.RuneCountInString(value)
	if count > max {
		return fieldError(field, "文本内容过长。")
	}
	if looksLikeContact(value) {
		return contactContentError(field)
	}
	if looksLikeSecret(value) {
		return secretError(field)
	}
	return nil
}

func publicDisputeFromCase(item DisputeCase, username string) PublicDispute {
	handledAt := item.UpdatedAt
	if item.ResolvedAt != nil {
		handledAt = *item.ResolvedAt
	}
	if item.ClosedAt != nil {
		handledAt = *item.ClosedAt
	}
	return PublicDispute{
		ID:         item.ID,
		Username:   normalizeUsername(username),
		Type:       nonEmpty(item.PublicSummary, item.TargetLabel, "纠纷记录"),
		Result:     nonEmpty(item.PublicResult, statusLabel(item.Status)),
		HandledAt:  handledAt,
		Unresolved: isUnresolvedDisputeStatus(item.Status),
	}
}

func sortReports(items []Report) {
	sort.SliceStable(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
}

func sortDisputes(items []DisputeCase) {
	sort.SliceStable(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
}

func sortAppeals(items []Appeal) {
	sort.SliceStable(items, func(i, j int) bool { return items[i].UpdatedAt.After(items[j].UpdatedAt) })
}

func statusLabel(status string) string {
	switch status {
	case DisputeStatusNegotiating:
		return "协商中"
	case DisputeStatusOpen:
		return "人工处理中"
	case DisputeStatusWaitingInfo:
		return "等待补充信息"
	case DisputeStatusResolved:
		return "已处理"
	case DisputeStatusClosed:
		return "已关闭"
	default:
		return status
	}
}

func isUnresolvedDisputeStatus(status string) bool {
	switch status {
	case DisputeStatusNegotiating, DisputeStatusOpen, DisputeStatusWaitingInfo:
		return true
	default:
		return false
	}
}

func displayName(user auth.User) string {
	if strings.TrimSpace(user.DisplayName) != "" {
		return strings.TrimSpace(user.DisplayName)
	}
	return strings.TrimSpace(user.Username)
}

func normalize(value string) string {
	return strings.TrimSpace(strings.ToLower(value))
}

func normalizeReason(value string) string {
	value = normalize(value)
	if value == "" {
		return ReportReasonOther
	}
	return value
}

func normalizePublicResultCode(value string) string {
	return normalize(value)
}

func normalizeUsername(value string) string {
	return normalize(value)
}

func matchesUsername(a, b string) bool {
	return normalizeUsername(a) == normalizeUsername(b)
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

func looksLikeSecret(value string) bool {
	return domain.LooksLikeSecretContent(value)
}

func looksLikeContact(value string) bool {
	return domain.LooksLikeContactContent(value)
}

func requireAdmin(user auth.User) *domain.AppError {
	if !user.IsAdmin {
		return domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	return nil
}

func fieldError(field, detail string) *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Report validation failed", detail, field, "invalid", detail)
}

func secretError(field string) *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeSecretContentDetected, "Secret content detected", "不能在举报、纠纷或申诉中填写、粘贴或上传任何凭据。", field, "secret_content", "不能包含密码、API Key、Token、Session、Cookie 或恢复码。")
}

func contactContentError(field string) *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeContactContentDetected, "Contact content detected", "不能在举报、纠纷或申诉中填写完整联系方式。", field, "contact_content", "不能包含手机号、邮箱、微信号、QQ 或其他完整联系方式。")
}

func invalidState(detail string) *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid report state", detail)
}

func versionConflict() *domain.AppError {
	return domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
}

func sessionRequired() *domain.AppError {
	return domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。")
}

func appealSourcePermissionDenied() *domain.AppError {
	return domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "你不能申诉该举报或纠纷。")
}

func appealSourceNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Appeal source not found", "关联举报或纠纷不存在。")
}

func reportNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Report not found", "举报记录不存在。")
}

func disputeNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Dispute not found", "纠纷记录不存在。")
}

func infoRequestNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Information request not found", "补充请求不存在、已失效或不属于当前用户。")
}

func infoRequestPermissionDenied() *domain.AppError {
	return domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "只能指定该案件中的有效参与者补充信息。")
}

func appealNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Appeal not found", "申诉记录不存在。")
}

func accountGovernanceAppealUnavailable() *domain.AppError {
	return domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "账号治理申诉服务暂不可用。")
}

func publicProfileNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Profile not found", "公开主页不存在。")
}

func selfReportForbidden() *domain.AppError {
	return domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "不能举报自己。")
}

func activeReportExists() *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeActiveReportExists, "Active report exists", "你已对该对象提交过进行中的举报或人工介入申请。")
}

var validTargets = map[string]bool{
	TargetContactSnapshot:    true,
	TargetPublicUser:         true,
	TargetCarpoolApplication: true,
	TargetCarpoolMembership:  true,
	TargetAPIPurchaseIntent:  true,
	TargetAPIOrder:           true,
}

var validReasons = map[string]bool{
	ReportReasonUnreachable:          true,
	ReportReasonContactInvalid:       true,
	ReportReasonImpersonation:        true,
	ReportReasonDescriptionMismatch:  true,
	ReportReasonSeatRuleDispute:      true,
	ReportReasonAPIQuotaDispute:      true,
	ReportReasonOrderDeliveryDispute: true,
	ReportReasonOther:                true,
}

var validPublicResultCodes = map[string]bool{
	PublicResultNoAction:               true,
	PublicResultContactInvalid:         true,
	PublicResultImpersonationConfirmed: true,
	PublicResultDescriptionMismatch:    true,
	PublicResultRuleOrSeatIssue:        true,
	PublicResultAPIDeliveryIssue:       true,
	PublicResultOtherResolved:          true,
}

func canFinishReport(status string) bool {
	switch status {
	case ReportStatusSubmitted, ReportStatusTriaged, ReportStatusNeedsInfo:
		return true
	default:
		return false
	}
}

func canOpenDisputeFromReport(status string) bool {
	switch status {
	case ReportStatusSubmitted, ReportStatusTriaged, ReportStatusNeedsInfo:
		return true
	default:
		return false
	}
}

func isActiveReportStatus(status string) bool {
	switch status {
	case ReportStatusSubmitted, ReportStatusTriaged, ReportStatusNeedsInfo, ReportStatusDisputeOpened:
		return true
	default:
		return false
	}
}
