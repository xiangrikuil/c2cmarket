package review

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/carpool"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/google/uuid"
)

type MembershipResolver interface {
	MyCarpoolMembershipsByUserID(ctx context.Context, userID string) ([]carpool.Membership, *domain.AppError)
}

type Service struct {
	mu          sync.Mutex
	now         func() time.Time
	repo        Repository
	idempotency *idempotency.Service
	resolver    MembershipResolver
	reviews     map[string]Review
}

func NewService(repo Repository, idempotencyService *idempotency.Service, resolver MembershipResolver, now func() time.Time) *Service {
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
		reviews:     make(map[string]Review),
	}
}

func (s *Service) ListMine(ctx context.Context, userID string) ([]ReviewCenterRow, *domain.AppError) {
	if strings.TrimSpace(userID) == "" {
		return nil, sessionRequired()
	}
	if s.repo != nil {
		return s.repo.ListMyReviewCenterRows(ctx, userID)
	}
	memberships, appErr := s.completedBuyerMemberships(ctx, userID)
	if appErr != nil {
		return nil, appErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := make([]ReviewCenterRow, 0, len(memberships))
	for _, membership := range memberships {
		row := s.rowFromMembershipLocked(membership)
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].UpdatedAt.After(rows[j].UpdatedAt)
	})
	return rows, nil
}

func (s *Service) SubmitWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input SubmitReviewInput, buildCompletion CompletionBuilder) (idempotency.Completion, *domain.AppError) {
	key = strings.TrimSpace(key)
	if err := idempotency.ValidateKey(key); err != nil {
		return idempotency.Completion{}, err
	}
	input.ReviewerUserID = userID
	input.SourceType = SourceCarpoolMembership
	if appErr := validateSubmitInput(input); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if buildCompletion == nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}

	entry, appErr := s.idempotency.Begin(ctx, userID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}
	if s.repo != nil {
		_, completion, appErr := s.repo.UpsertCarpoolReviewWithIdempotency(ctx, *entry, input, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}

	result, appErr := s.submitMemory(ctx, input)
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
	if strings.TrimSpace(username) == "" {
		return nil, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Profile not found", "公开主页不存在。")
	}
	if s.repo != nil {
		return s.repo.ListPublicUserReviews(ctx, username)
	}
	return []PublicReview{}, nil
}

func (s *Service) submitMemory(ctx context.Context, input SubmitReviewInput) (MutationResult, *domain.AppError) {
	memberships, appErr := s.completedBuyerMemberships(ctx, input.ReviewerUserID)
	if appErr != nil {
		return MutationResult{}, appErr
	}
	var membership carpool.Membership
	for _, item := range memberships {
		if item.ID == input.SourceID {
			membership = item
			break
		}
	}
	if membership.ID == "" {
		return MutationResult{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Carpool membership not found", "已完成拼车成员关系不存在。")
	}

	now := s.now()
	key := reviewKey(SourceCarpoolMembership, membership.ID, input.ReviewerUserID)
	s.mu.Lock()
	review, ok := s.reviews[key]
	if !ok {
		review = Review{
			ID:             uuid.NewString(),
			SourceType:     SourceCarpoolMembership,
			SourceID:       membership.ID,
			ReviewerUserID: input.ReviewerUserID,
			RevieweeUserID: membership.OwnerUserID,
			ReviewerRole:   ReviewerRoleBuyer,
			RevieweeRole:   RevieweeRoleOwner,
			CreatedAt:      now,
		}
	}
	review.Rating = input.Rating
	review.Tags = normalizeTags(input.Tags)
	review.Note = strings.TrimSpace(input.Note)
	review.UpdatedAt = now
	s.reviews[key] = review
	row := s.rowFromMembershipLocked(membership)
	s.mu.Unlock()
	return MutationResult{Row: row}, nil
}

func (s *Service) completedBuyerMemberships(ctx context.Context, userID string) ([]carpool.Membership, *domain.AppError) {
	if s.resolver == nil {
		return nil, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "评价来源解析器不可用。")
	}
	items, appErr := s.resolver.MyCarpoolMembershipsByUserID(ctx, userID)
	if appErr != nil {
		return nil, appErr
	}
	completed := make([]carpool.Membership, 0, len(items))
	for _, item := range items {
		if item.BuyerUserID == userID && item.Status == carpool.MembershipStatusCompleted {
			completed = append(completed, item)
		}
	}
	return completed, nil
}

func (s *Service) rowFromMembershipLocked(membership carpool.Membership) ReviewCenterRow {
	row := ReviewCenterRow{
		ID:                   "review-carpool-membership-" + membership.ID,
		SourceType:           SourceCarpoolMembership,
		SourceID:             membership.ID,
		Target:               "拼车车源",
		CounterpartyUsername: membership.OwnerUserID,
		CounterpartyName:     membership.OwnerUserID,
		Status:               "可评价",
		CreatedAt:            membershipReviewTime(membership),
		UpdatedAt:            membershipReviewTime(membership),
	}
	if review, ok := s.reviews[reviewKey(SourceCarpoolMembership, membership.ID, membership.BuyerUserID)]; ok {
		row.ID = review.ID
		row.Status = "已评价"
		row.Rating = review.Rating
		row.Tags = append([]string{}, review.Tags...)
		row.Note = review.Note
		row.CreatedAt = review.CreatedAt
		row.UpdatedAt = review.UpdatedAt
	}
	return row
}

func membershipReviewTime(membership carpool.Membership) time.Time {
	if membership.EndedAt != nil {
		return *membership.EndedAt
	}
	if membership.CompletedAt != nil {
		return *membership.CompletedAt
	}
	return membership.UpdatedAt
}

func validateSubmitInput(input SubmitReviewInput) *domain.AppError {
	if strings.TrimSpace(input.SourceID) == "" {
		return validationError("sourceId", "必须提供评价来源。")
	}
	if strings.TrimSpace(input.ReviewerUserID) == "" {
		return sessionRequired()
	}
	if input.Rating < 1 || input.Rating > 5 {
		return validationError("rating", "评分必须在 1-5 分之间。")
	}
	tags := normalizeTags(input.Tags)
	if len(tags) > 5 {
		return validationError("tags", "体验标签最多 5 个。")
	}
	for _, tag := range tags {
		if utf8.RuneCountInString(tag) > 16 {
			return validationError("tags", "单个体验标签最多 16 字。")
		}
		if looksLikeSecret(tag) {
			return secretError("tags")
		}
	}
	note := strings.TrimSpace(input.Note)
	if note == "" {
		return validationError("note", "评价说明不能为空。")
	}
	if utf8.RuneCountInString(note) > 600 {
		return validationError("note", "评价说明最多 600 字。")
	}
	if looksLikeSecret(note) {
		return secretError("note")
	}
	return nil
}

func normalizeTags(tags []string) []string {
	result := make([]string, 0, len(tags))
	seen := map[string]struct{}{}
	for _, tag := range tags {
		value := strings.TrimSpace(tag)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func reviewKey(sourceType, sourceID, reviewerUserID string) string {
	return sourceType + ":" + sourceID + ":" + reviewerUserID
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

func validationError(field, detail string) *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Review validation failed", detail, field, "invalid", detail)
}

func secretError(field string) *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeSecretContentDetected, "Secret content detected", "不能在评价中填写、粘贴或上传任何凭据。", field, "secret_content", "不能包含密码、API Key、Token、Session、Cookie 或恢复码。")
}

func sessionRequired() *domain.AppError {
	return domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。")
}
