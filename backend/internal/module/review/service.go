package review

import (
	"context"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/google/uuid"
)

type TransactionResolver interface {
	ReviewTransactionsByUserID(ctx context.Context, userID string) ([]Transaction, *domain.AppError)
	ResolveReviewTransaction(ctx context.Context, transactionType, transactionID, userID string) (Transaction, *domain.AppError)
}

type ActionAuthorizer interface {
	CheckActionAllowed(ctx context.Context, userID, role, action string) *domain.AppError
}

type Service struct {
	mu          sync.Mutex
	now         func() time.Time
	repo        Repository
	idempotency *idempotency.Service
	resolver    TransactionResolver
	authorizer  ActionAuthorizer
	reviews     map[string]Review
}

func NewService(repo Repository, idempotencyService *idempotency.Service, resolver TransactionResolver, authorizer ActionAuthorizer, now func() time.Time) *Service {
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
		resolver:    resolver,
		authorizer:  authorizer,
		reviews:     make(map[string]Review),
	}
}

func (s *Service) ListMine(ctx context.Context, userID string) ([]ReviewCenterRow, *domain.AppError) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, sessionRequired()
	}
	now := s.now()
	if s.repo != nil {
		rows, appErr := s.repo.ListMyReviewCenterRows(ctx, userID, now)
		if appErr != nil {
			return nil, appErr
		}
		return decorateReviewCenterRows(rows), nil
	}
	if s.resolver == nil {
		return nil, internalReviewError("评价交易解析器不可用。")
	}
	transactions, appErr := s.resolver.ReviewTransactionsByUserID(ctx, userID)
	if appErr != nil {
		return nil, appErr
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, transaction := range transactions {
		s.refreshMutableTransactionReviewsLocked(transaction)
	}
	s.publishExpiredForTransactionsLocked(now, transactions)
	rows := make([]ReviewCenterRow, 0, len(transactions)*2)
	for _, transaction := range transactions {
		rows = append(rows, s.rowsForTransactionLocked(transaction, userID, now)...)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].CompletedAt.Equal(rows[j].CompletedAt) {
			return directionOrder(rows[i].Direction) < directionOrder(rows[j].Direction)
		}
		return rows[i].CompletedAt.After(rows[j].CompletedAt)
	})
	return decorateReviewCenterRows(rows), nil
}

func (s *Service) SubmitWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input SubmitReviewInput, buildCompletion CompletionBuilder) (idempotency.Completion, *domain.AppError) {
	userID = strings.TrimSpace(userID)
	input.ReviewerUserID = userID
	input.TransactionType = strings.TrimSpace(input.TransactionType)
	input.TransactionID = strings.TrimSpace(input.TransactionID)
	input.Operation = strings.TrimSpace(input.Operation)
	if appErr := ValidateSubmitInput(input); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if buildCompletion == nil {
		return idempotency.Completion{}, internalReviewError("响应编码失败。")
	}

	key = strings.TrimSpace(key)
	if appErr := idempotency.ValidateKey(key); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	entry, appErr := s.idempotency.Begin(ctx, userID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}

	transaction, appErr := s.resolveTransaction(ctx, input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	reviewerRole, revieweeRole, ok := transactionRoles(transaction, userID)
	if !ok {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, reviewTransactionNotFound()
	}
	now := s.now()
	if transaction.ReviewPaused {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, reviewPaused()
	}
	if !now.Before(transaction.ReviewDeadlineAt) {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, reviewWindowClosed()
	}
	if !ValidateTagsForScenario(input.Tags, transaction.Type, reviewerRole, revieweeRole) {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, validationError("tags", "所选体验标签不适用于当前交易评价。")
	}
	input.Tags = NormalizeTagCodes(input.Tags)
	if s.authorizer != nil {
		if appErr := s.authorizer.CheckActionAllowed(ctx, userID, reviewerRole, "review_submit"); appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
	}

	if s.repo != nil {
		_, completion, appErr := s.repo.SaveTransactionReviewWithIdempotency(ctx, *entry, transaction, input, now, buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}

	result, appErr := s.saveMemory(transaction, input, now)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(result)
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

func (s *Service) RemoveWithIdempotency(ctx context.Context, adminUserID string, isAdmin bool, routeKey, key, requestHash string, input RemoveReviewInput, buildCompletion CompletionBuilder) (idempotency.Completion, *domain.AppError) {
	if !isAdmin {
		return idempotency.Completion{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	input.AdminUserID = strings.TrimSpace(adminUserID)
	input.ReviewID = strings.TrimSpace(input.ReviewID)
	input.Reason = strings.TrimSpace(input.Reason)
	if _, err := uuid.Parse(input.ReviewID); err != nil {
		return idempotency.Completion{}, validationError("reviewId", "评价 ID 格式不正确。")
	}
	if input.ExpectedVersion < 1 {
		return idempotency.Completion{}, validationError("version", "必须提供有效评价版本。")
	}
	if input.Reason == "" {
		return idempotency.Completion{}, validationError("reason", "移除评价必须填写原因。")
	}
	if utf8.RuneCountInString(input.Reason) > 500 {
		return idempotency.Completion{}, validationError("reason", "移除原因最多 500 字。")
	}
	if buildCompletion == nil {
		return idempotency.Completion{}, internalReviewError("响应编码失败。")
	}
	if appErr := idempotency.ValidateKey(strings.TrimSpace(key)); appErr != nil {
		return idempotency.Completion{}, appErr
	}

	entry, appErr := s.idempotency.Begin(ctx, input.AdminUserID, routeKey, strings.TrimSpace(key), requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}
	if s.repo != nil {
		_, completion, appErr := s.repo.RemoveTransactionReviewWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}

	result, appErr := s.removeMemory(input, s.now())
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(result)
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

func (s *Service) PublicForUser(ctx context.Context, username string) ([]PublicReview, *domain.AppError) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Profile not found", "公开主页不存在。")
	}
	if s.repo != nil {
		return s.repo.ListPublicUserReviews(ctx, username, s.now())
	}
	return []PublicReview{}, nil
}

func (s *Service) resolveTransaction(ctx context.Context, input SubmitReviewInput) (Transaction, *domain.AppError) {
	if s.repo != nil {
		return s.repo.ResolveTransactionForReview(ctx, input.TransactionType, input.TransactionID, input.ReviewerUserID)
	}
	if s.resolver == nil {
		return Transaction{}, internalReviewError("评价交易解析器不可用。")
	}
	return s.resolver.ResolveReviewTransaction(ctx, input.TransactionType, input.TransactionID, input.ReviewerUserID)
}

func (s *Service) saveMemory(transaction Transaction, input SubmitReviewInput, now time.Time) (MutationResult, *domain.AppError) {
	reviewerRole, revieweeRole, ok := transactionRoles(transaction, input.ReviewerUserID)
	if !ok {
		return MutationResult{}, reviewTransactionNotFound()
	}
	revieweeUserID := transaction.SellerUserID
	if reviewerRole == RoleSeller {
		revieweeUserID = transaction.BuyerUserID
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.refreshMutableTransactionReviewsLocked(transaction)
	s.publishExpiredForTransactionLocked(now, transaction)
	key := reviewKey(input.TransactionType, input.TransactionID, input.ReviewerUserID)
	item, exists := s.reviews[key]
	switch input.Operation {
	case OperationCreate:
		if exists {
			return MutationResult{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Review already exists", "该交易已提交评价；公开前请使用修改操作。")
		}
	case OperationEdit:
		if !exists {
			return MutationResult{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Review not found", "待修改评价不存在。")
		}
	case OperationLegacyUpsert:
	default:
		return MutationResult{}, validationError("operation", "评价操作不受支持。")
	}
	if exists && (item.Status != StatusSealed || item.FrozenAt != nil || !now.Before(item.ReviewDeadlineAt)) {
		return MutationResult{}, reviewFrozen()
	}

	if !exists {
		item = Review{
			ID:                uuid.NewString(),
			TransactionType:   transaction.Type,
			ReviewerUserID:    input.ReviewerUserID,
			RevieweeUserID:    revieweeUserID,
			ReviewerRole:      reviewerRole,
			RevieweeRole:      revieweeRole,
			Status:            StatusSealed,
			ReviewDeadlineAt:  transaction.ReviewDeadlineAt,
			CommercialOutcome: transaction.CommercialOutcome,
			CreatedAt:         now,
			Version:           1,
		}
		if transaction.Type == TransactionCarpoolMembership {
			item.CarpoolMembershipID = transaction.ID
		} else {
			item.APIOrderID = transaction.ID
		}
	} else {
		item.Version++
	}
	item.Rating = input.Rating
	item.Tags = NormalizeTagCodes(input.Tags)
	item.Note = strings.TrimSpace(input.Note)
	item.UpdatedAt = now
	s.reviews[key] = item

	counterpartyKey := reviewKey(transaction.Type, transaction.ID, revieweeUserID)
	if counterparty, found := s.reviews[counterpartyKey]; found && counterparty.Status == StatusSealed {
		visibleAt := now
		item.Status = StatusPublished
		item.VisibleAt = &visibleAt
		item.FrozenAt = &visibleAt
		item.Version++
		counterparty.Status = StatusPublished
		counterparty.VisibleAt = &visibleAt
		counterparty.FrozenAt = &visibleAt
		counterparty.Version++
		s.reviews[key] = item
		s.reviews[counterpartyKey] = counterparty
	}
	return MutationResult{Row: reviewRow(transaction, item, DirectionSent, true, revieweeUserID, now)}, nil
}

func (s *Service) removeMemory(input RemoveReviewInput, now time.Time) (MutationResult, *domain.AppError) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, item := range s.reviews {
		if item.ID != input.ReviewID {
			continue
		}
		if item.Version != input.ExpectedVersion {
			return MutationResult{}, domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "评价已更新，请刷新后重试。")
		}
		if item.Status != StatusPublished || item.FrozenAt == nil {
			return MutationResult{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Invalid state transition", "只能移除已公开评价。")
		}
		item.Status = StatusRemoved
		item.RemovedAt = &now
		item.RemovedByAdminID = input.AdminUserID
		item.RemovalReason = input.Reason
		item.UpdatedAt = now
		item.Version++
		s.reviews[key] = item
		return MutationResult{Row: rowFromRemovedReview(item)}, nil
	}
	return MutationResult{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Review not found", "评价不存在。")
}

func (s *Service) publishExpiredForTransactionsLocked(now time.Time, transactions []Transaction) {
	for _, transaction := range transactions {
		s.publishExpiredForTransactionLocked(now, transaction)
	}
}

func (s *Service) refreshMutableTransactionReviewsLocked(transaction Transaction) {
	if transaction.Type != TransactionAPIOrder || transaction.ReviewPaused || !IsReviewableAPIOrderOutcome(transaction.CommercialOutcome) {
		return
	}
	for key, item := range s.reviews {
		if item.APIOrderID != transaction.ID || item.Status != StatusSealed || item.FrozenAt != nil {
			continue
		}
		if item.CommercialOutcome == transaction.CommercialOutcome && item.ReviewDeadlineAt.Equal(transaction.ReviewDeadlineAt) {
			continue
		}
		item.CommercialOutcome = transaction.CommercialOutcome
		item.ReviewDeadlineAt = transaction.ReviewDeadlineAt
		item.UpdatedAt = transaction.CompletedAt
		item.Version++
		s.reviews[key] = item
	}
}

func (s *Service) publishExpiredForTransactionLocked(now time.Time, transaction Transaction) {
	if transaction.ReviewPaused {
		return
	}
	for key, item := range s.reviews {
		transactionID := item.CarpoolMembershipID
		if item.TransactionType == TransactionAPIOrder {
			transactionID = item.APIOrderID
		}
		if item.TransactionType != transaction.Type || transactionID != transaction.ID || item.Status != StatusSealed || now.Before(item.ReviewDeadlineAt) {
			continue
		}
		visibleAt := item.ReviewDeadlineAt
		item.Status = StatusPublished
		item.VisibleAt = &visibleAt
		item.FrozenAt = &visibleAt
		item.Version++
		s.reviews[key] = item
	}
}

func (s *Service) rowsForTransactionLocked(transaction Transaction, userID string, now time.Time) []ReviewCenterRow {
	reviewerRole, revieweeRole, ok := transactionRoles(transaction, userID)
	if !ok {
		return nil
	}
	counterpartyUserID := transaction.SellerUserID
	if reviewerRole == RoleSeller {
		counterpartyUserID = transaction.BuyerUserID
	}
	current, currentExists := s.reviews[reviewKey(transaction.Type, transaction.ID, userID)]
	counterparty, counterpartyExists := s.reviews[reviewKey(transaction.Type, transaction.ID, counterpartyUserID)]

	rows := make([]ReviewCenterRow, 0, 2)
	if !currentExists {
		status := CenterStatusReviewable
		canCreate := !transaction.ReviewPaused && now.Before(transaction.ReviewDeadlineAt)
		if transaction.ReviewPaused {
			status = CenterStatusPaused
		} else if !canCreate {
			status = CenterStatusExpired
		}
		rows = append(rows, ReviewCenterRow{
			ID:                    "reviewable-" + transaction.Type + "-" + transaction.ID,
			TransactionType:       transaction.Type,
			TransactionID:         transaction.ID,
			Direction:             DirectionPending,
			Target:                transaction.Target,
			CounterpartyUsername:  counterpartyUsername(transaction, userID),
			CounterpartyName:      counterpartyName(transaction, userID),
			ReviewerRole:          reviewerRole,
			RevieweeRole:          revieweeRole,
			Status:                status,
			Visibility:            VisibilityNone,
			CounterpartySubmitted: false,
			CanCreate:             canCreate,
			CompletedAt:           transaction.CompletedAt,
			ReviewDeadlineAt:      transaction.ReviewDeadlineAt,
			CommercialOutcome:     transaction.CommercialOutcome,
			ReviewPaused:          transaction.ReviewPaused,
			CreatedAt:             transaction.CompletedAt,
			UpdatedAt:             transaction.CompletedAt,
		})
	} else {
		rows = append(rows, reviewRow(transaction, current, DirectionSent, current.Status != StatusRemoved, counterpartyUserID, now))
	}
	if counterpartyExists && counterparty.Status != StatusSealed {
		rows = append(rows, reviewRow(transaction, counterparty, DirectionReceived, counterparty.Status == StatusPublished, counterparty.ReviewerUserID, now))
	}
	return rows
}

func reviewRow(transaction Transaction, item Review, direction string, contentVisible bool, counterpartyUserID string, now time.Time) ReviewCenterRow {
	submittedAt := item.CreatedAt
	row := ReviewCenterRow{
		ID:                    item.ID,
		TransactionType:       transaction.Type,
		TransactionID:         transaction.ID,
		Direction:             direction,
		Target:                transaction.Target,
		CounterpartyUsername:  usernameForUser(transaction, counterpartyUserID),
		CounterpartyName:      displayNameForUser(transaction, counterpartyUserID),
		ReviewerRole:          item.ReviewerRole,
		RevieweeRole:          item.RevieweeRole,
		Status:                item.Status,
		Visibility:            visibilityForStatus(item.Status),
		CounterpartySubmitted: item.Status != StatusSealed,
		CanEdit:               direction == DirectionSent && item.Status == StatusSealed && !transaction.ReviewPaused && now.Before(item.ReviewDeadlineAt),
		ContentVisible:        contentVisible,
		CompletedAt:           transaction.CompletedAt,
		ReviewDeadlineAt:      item.ReviewDeadlineAt,
		CommercialOutcome:     item.CommercialOutcome,
		ReviewPaused:          transaction.ReviewPaused,
		SubmittedAt:           &submittedAt,
		VisibleAt:             item.VisibleAt,
		FrozenAt:              item.FrozenAt,
		CreatedAt:             item.CreatedAt,
		UpdatedAt:             item.UpdatedAt,
		Version:               item.Version,
	}
	if contentVisible {
		row.Rating = item.Rating
		row.Tags = append([]string{}, item.Tags...)
		row.Note = item.Note
	}
	return row
}

func rowFromRemovedReview(item Review) ReviewCenterRow {
	transactionID := item.CarpoolMembershipID
	if item.TransactionType == TransactionAPIOrder {
		transactionID = item.APIOrderID
	}
	submittedAt := item.CreatedAt
	return ReviewCenterRow{
		ID:               item.ID,
		TransactionType:  item.TransactionType,
		TransactionID:    transactionID,
		Direction:        DirectionSent,
		Status:           StatusRemoved,
		Visibility:       VisibilityRemoved,
		ReviewerRole:     item.ReviewerRole,
		RevieweeRole:     item.RevieweeRole,
		CompletedAt:      item.CreatedAt,
		ReviewDeadlineAt: item.ReviewDeadlineAt,
		SubmittedAt:      &submittedAt,
		VisibleAt:        item.VisibleAt,
		FrozenAt:         item.FrozenAt,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
		Version:          item.Version,
	}
}

func ValidateSubmitInput(input SubmitReviewInput) *domain.AppError {
	if strings.TrimSpace(input.ReviewerUserID) == "" {
		return sessionRequired()
	}
	if input.TransactionType != TransactionCarpoolMembership && input.TransactionType != TransactionAPIOrder {
		return validationError("type", "交易类型必须是 carpool_membership 或 api_order。")
	}
	if _, err := uuid.Parse(strings.TrimSpace(input.TransactionID)); err != nil {
		return validationError("id", "交易 ID 格式不正确。")
	}
	if input.Operation != OperationCreate && input.Operation != OperationEdit && input.Operation != OperationLegacyUpsert {
		return validationError("operation", "评价操作不受支持。")
	}
	if input.Rating < 1 || input.Rating > 5 {
		return validationError("rating", "评分必须在 1-5 分之间。")
	}
	tags := NormalizeTagCodes(input.Tags)
	if len(tags) > 5 {
		return validationError("tags", "体验标签最多 5 个。")
	}
	for _, tag := range tags {
		if utf8.RuneCountInString(tag) > 16 {
			return validationError("tags", "单个体验标签最多 16 字。")
		}
		if !IsKnownTag(tag) {
			return validationError("tags", "只能选择平台预设的体验标签。")
		}
	}
	note := strings.TrimSpace(input.Note)
	if note == "" && len(tags) == 0 {
		return validationError("content", "请至少选择一个体验标签或填写评价说明。")
	}
	if utf8.RuneCountInString(note) > 600 {
		return validationError("note", "评价说明最多 600 字。")
	}
	if domain.LooksLikeSecretContent(note) || looksLikeContactContent(note) {
		return sensitiveContentError("note")
	}
	return nil
}

func NormalizeTags(tags []string) []string {
	return NormalizeTagCodes(tags)
}

func IsPresetTag(tag string) bool {
	return IsKnownTag(tag)
}

func decorateReviewCenterRows(rows []ReviewCenterRow) []ReviewCenterRow {
	visibleRows := make([]ReviewCenterRow, 0, len(rows))
	for index := range rows {
		if rows[index].Direction == DirectionReceived && rows[index].Visibility == VisibilitySealed {
			continue
		}
		rows[index].AllowedTags = AllowedTags(rows[index].TransactionType, rows[index].ReviewerRole, rows[index].RevieweeRole)
		visibleRows = append(visibleRows, rows[index])
	}
	return visibleRows
}

func transactionRoles(transaction Transaction, userID string) (string, string, bool) {
	switch userID {
	case transaction.BuyerUserID:
		return RoleBuyer, RoleSeller, true
	case transaction.SellerUserID:
		return RoleSeller, RoleBuyer, true
	default:
		return "", "", false
	}
}

func counterpartyUsername(transaction Transaction, userID string) string {
	if userID == transaction.BuyerUserID {
		return transaction.SellerUsername
	}
	return transaction.BuyerUsername
}

func counterpartyName(transaction Transaction, userID string) string {
	if userID == transaction.BuyerUserID {
		return strings.TrimSpace(transaction.SellerDisplayName)
	}
	return strings.TrimSpace(transaction.BuyerDisplayName)
}

func usernameForUser(transaction Transaction, userID string) string {
	if userID == transaction.BuyerUserID {
		return transaction.BuyerUsername
	}
	return transaction.SellerUsername
}

func displayNameForUser(transaction Transaction, userID string) string {
	if userID == transaction.BuyerUserID {
		return strings.TrimSpace(transaction.BuyerDisplayName)
	}
	return strings.TrimSpace(transaction.SellerDisplayName)
}

func reviewKey(transactionType, transactionID, reviewerUserID string) string {
	return transactionType + ":" + transactionID + ":" + reviewerUserID
}

func visibilityForStatus(status string) string {
	switch status {
	case StatusSealed:
		return VisibilitySealed
	case StatusPublished:
		return VisibilityPublished
	case StatusRemoved:
		return VisibilityRemoved
	default:
		return VisibilityNone
	}
}

func directionOrder(direction string) int {
	switch direction {
	case DirectionPending:
		return 0
	case DirectionSent:
		return 1
	default:
		return 2
	}
}

func validationError(field, detail string) *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Review validation failed", detail, field, "invalid", detail)
}

func sensitiveContentError(field string) *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeSecretContentDetected, "Sensitive content detected", "不能在评价中填写联系方式或任何凭据。", field, "sensitive_content", "不能包含联系方式、密码、API Key、Token、Session、Cookie 或恢复码。")
}

func sessionRequired() *domain.AppError {
	return domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。")
}

func reviewTransactionNotFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Transaction not found", "可评价交易不存在。")
}

func reviewWindowClosed() *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Review window closed", "评价窗口已截止。")
}

func reviewPaused() *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Review paused", "活跃纠纷期间暂停创建、修改和公开评价。")
}

func reviewFrozen() *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Review is frozen", "评价已公开或已冻结，不能再修改。")
}

func internalReviewError(detail string) *domain.AppError {
	return domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", detail)
}

var (
	reviewEmailPattern   = regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`)
	reviewPhonePattern   = regexp.MustCompile(`(?:\+?\d[\d -]{7,}\d)`)
	reviewContactPattern = regexp.MustCompile(`(?i)(微信号|QQ号|telegram\s*[:：]|(?:wx|vx|qq|tg)\s*[:：])`)
)

func looksLikeContactContent(value string) bool {
	return reviewEmailPattern.MatchString(value) ||
		reviewPhonePattern.MatchString(value) ||
		reviewContactPattern.MatchString(value)
}
