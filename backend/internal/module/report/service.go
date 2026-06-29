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

	"github.com/google/uuid"
)

type Service struct {
	mu          sync.Mutex
	now         func() time.Time
	repo        Repository
	idempotency *idempotency.Service
	reports     map[string]Report
	disputes    map[string]DisputeCase
	appeals     map[string]Appeal
}

func NewService(repo Repository, idempotencyService *idempotency.Service, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	if idempotencyService == nil {
		idempotencyService = idempotency.NewService(nil, now)
	}
	return &Service{
		now:         now,
		repo:        repo,
		idempotency: idempotencyService,
		reports:     make(map[string]Report),
		disputes:    make(map[string]DisputeCase),
		appeals:     make(map[string]Appeal),
	}
}

func (s *Service) CreateReportWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input CreateReportInput, buildCompletion ReportCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	input.ReporterUserID = user.ID
	input.ReporterUsername = user.Username
	input.ReporterName = displayName(user)
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
	item := s.createReportMemory(input)
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

func (s *Service) AdminReports(ctx context.Context, user auth.User) ([]Report, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return nil, appErr
	}
	if s.repo != nil {
		return s.repo.ListAdminReports(ctx)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]Report, 0, len(s.reports))
	for _, item := range s.reports {
		items = append(items, item)
	}
	sortReports(items)
	return items, nil
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
	return item, nil
}

func (s *Service) AdminReportActionWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input AdminActionInput, buildCompletion AdminCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	input.AdminUserID = user.ID
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

func (s *Service) AdminDispute(ctx context.Context, user auth.User, id string) (DisputeCase, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return DisputeCase{}, appErr
	}
	if s.repo != nil {
		return s.repo.GetAdminDispute(ctx, id)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.disputes[id]
	if !ok {
		return DisputeCase{}, disputeNotFound()
	}
	return item, nil
}

func (s *Service) AdminDisputeActionWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input AdminActionInput, buildCompletion AdminCompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	input.AdminUserID = user.ID
	if appErr := validateDisputeAction(input); appErr != nil {
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
		_, completion, appErr := s.repo.UpdateDisputeAdminWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}
	result, appErr := s.updateDisputeAdminMemory(input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
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
		ID:                 uuid.NewString(),
		TargetType:         TargetAPIOrder,
		TargetID:           strings.TrimSpace(input.OrderID),
		TargetLabel:        nonEmpty(input.ServiceTitle, "API 订单"),
		PrimaryUserID:      strings.TrimSpace(input.ActorUserID),
		CounterpartyUserID: strings.TrimSpace(counterpartyID),
		Status:             DisputeStatusOpen,
		PublicSummary:      "API 订单纠纷",
		PublicResult:       "已进入人工处理中",
		AdminReason:        strings.TrimSpace(input.Reason),
		OpenedByAdminID:    strings.TrimSpace(input.ActorUserID),
		OpenedAt:           now,
		CreatedAt:          now,
		UpdatedAt:          now,
		Version:            1,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.disputes[item.ID] = item
	return item.ID, nil
}

func (s *Service) createReportMemory(input CreateReportInput) Report {
	now := s.now()
	item := Report{
		ID:               uuid.NewString(),
		ReporterUserID:   input.ReporterUserID,
		ReporterUsername: input.ReporterUsername,
		ReporterName:     input.ReporterName,
		TargetType:       strings.TrimSpace(input.TargetType),
		TargetID:         strings.TrimSpace(input.TargetID),
		TargetLabel:      strings.TrimSpace(input.TargetLabel),
		ReportedUsername: normalizeUsername(input.ReportedUsername),
		ReasonCode:       normalizeReason(input.ReasonCode),
		Title:            strings.TrimSpace(input.Title),
		Description:      strings.TrimSpace(input.Description),
		Status:           ReportStatusSubmitted,
		CreatedAt:        now,
		UpdatedAt:        now,
		Version:          1,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reports[item.ID] = item
	return item
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
	case "reject":
		if item.Status == ReportStatusRejected || item.Status == ReportStatusDisputeOpened {
			return MutationResult{}, invalidState("当前举报不能拒绝。")
		}
		item.Status = ReportStatusRejected
	case "open_dispute":
		if item.Status == ReportStatusDisputeOpened || item.Status == ReportStatusRejected {
			return MutationResult{}, invalidState("当前举报不能打开纠纷。")
		}
		dispute := DisputeCase{
			ID:                   uuid.NewString(),
			ReportID:             item.ID,
			TargetType:           item.TargetType,
			TargetID:             item.TargetID,
			TargetLabel:          nonEmpty(input.PublicSummary, item.TargetLabel, item.Title),
			PrimaryUserID:        item.ReporterUserID,
			PrimaryUsername:      item.ReporterUsername,
			PrimaryDisplayName:   item.ReporterName,
			CounterpartyUsername: item.ReportedUsername,
			Status:               DisputeStatusOpen,
			PublicSummary:        nonEmpty(input.PublicSummary, item.Title),
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
	s.reports[item.ID] = item
	return MutationResult{Report: &item}, nil
}

func (s *Service) updateDisputeAdminMemory(input AdminActionInput) (MutationResult, *domain.AppError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.disputes[input.ID]
	if !ok {
		return MutationResult{}, disputeNotFound()
	}
	if input.ExpectedVersion > 0 && item.Version != input.ExpectedVersion {
		return MutationResult{}, versionConflict()
	}
	now := s.now()
	switch input.Action {
	case "request_info":
		if item.Status != DisputeStatusOpen {
			return MutationResult{}, invalidState("只有打开中的纠纷可以要求补充信息。")
		}
		item.Status = DisputeStatusWaitingInfo
	case "resolve":
		if item.Status != DisputeStatusOpen && item.Status != DisputeStatusWaitingInfo {
			return MutationResult{}, invalidState("当前纠纷不能标记处理完成。")
		}
		item.Status = DisputeStatusResolved
		item.ResolvedAt = &now
	case "close":
		if item.Status == DisputeStatusClosed {
			return MutationResult{}, invalidState("纠纷已关闭。")
		}
		item.Status = DisputeStatusClosed
		item.ClosedAt = &now
	default:
		return MutationResult{}, invalidState("纠纷处理动作不支持。")
	}
	item.AdminReason = strings.TrimSpace(input.Reason)
	item.PublicSummary = nonEmpty(input.PublicSummary, item.PublicSummary)
	item.PublicResult = nonEmpty(input.PublicResult, item.PublicResult)
	item.UpdatedAt = now
	item.Version++
	s.disputes[item.ID] = item
	return MutationResult{Dispute: &item}, nil
}

func (s *Service) createAppealMemory(input CreateAppealInput) (Appeal, *domain.AppError) {
	now := s.now()
	item := Appeal{
		ID:                uuid.NewString(),
		AppellantUserID:   input.AppellantUserID,
		AppellantUsername: input.AppellantUsername,
		AppellantName:     input.AppellantName,
		ReportID:          strings.TrimSpace(input.ReportID),
		DisputeID:         strings.TrimSpace(input.DisputeID),
		TargetType:        strings.TrimSpace(input.TargetType),
		TargetID:          strings.TrimSpace(input.TargetID),
		Title:             strings.TrimSpace(input.Title),
		Statement:         strings.TrimSpace(input.Statement),
		Status:            AppealStatusSubmitted,
		CreatedAt:         now,
		UpdatedAt:         now,
		Version:           1,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if item.DisputeID != "" {
		if _, ok := s.disputes[item.DisputeID]; !ok {
			return Appeal{}, disputeNotFound()
		}
	}
	if item.ReportID != "" {
		if _, ok := s.reports[item.ReportID]; !ok {
			return Appeal{}, reportNotFound()
		}
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
	if strings.TrimSpace(input.TargetType) != "" && !validTargets[normalize(input.TargetType)] {
		return fieldError("targetType", "申诉目标类型不支持。")
	}
	return nil
}

func validateReportAction(input AdminActionInput) *domain.AppError {
	switch input.Action {
	case "triage", "reject", "open_dispute":
	default:
		return invalidState("举报处理动作不支持。")
	}
	if strings.TrimSpace(input.Reason) == "" {
		return fieldError("reason", "必须填写处理原因。")
	}
	if input.Action == "open_dispute" {
		if appErr := validateText("publicSummary", input.PublicSummary, 2, 120, "公开纠纷摘要需为 2 至 120 个字符。"); appErr != nil {
			return appErr
		}
		if appErr := validateText("publicResult", nonEmpty(input.PublicResult, "已进入人工处理中"), 2, 120, "公开处理结果需为 2 至 120 个字符。"); appErr != nil {
			return appErr
		}
	}
	return validateText("reason", input.Reason, 2, 800, "处理原因需为 2 至 800 个字符。")
}

func validateDisputeAction(input AdminActionInput) *domain.AppError {
	switch input.Action {
	case "request_info", "resolve", "close":
	default:
		return invalidState("纠纷处理动作不支持。")
	}
	if strings.TrimSpace(input.Reason) == "" {
		return fieldError("reason", "必须填写处理原因。")
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
	return nil
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
		Unresolved: item.Status == DisputeStatusOpen || item.Status == DisputeStatusWaitingInfo,
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
	lower := strings.ToLower(value)
	needles := []string{
		"password", "密码", "api key", "api_key", "apikey", "sub2api key",
		"token", "session", "cookie", "secret", "bearer ", "access_token",
		"refresh_token", "恢复码", "mfa", "验证码",
	}
	for _, needle := range needles {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
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

func invalidState(detail string) *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid report state", detail)
}

func versionConflict() *domain.AppError {
	return domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "资源版本已变化，请刷新后重试。")
}

func sessionRequired() *domain.AppError {
	return domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。")
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

func publicProfileNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Profile not found", "公开主页不存在。")
}

var validTargets = map[string]bool{
	TargetContactSnapshot:   true,
	TargetPublicUser:        true,
	TargetCarpoolMembership: true,
	TargetAPIPurchaseIntent: true,
	TargetAPIOrder:          true,
}

var validReasons = map[string]bool{
	ReportReasonInvalid:       true,
	ReportReasonUnreachable:   true,
	ReportReasonImpersonation: true,
	ReportReasonOther:         true,
}
