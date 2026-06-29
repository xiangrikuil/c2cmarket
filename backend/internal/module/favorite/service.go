package favorite

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/google/uuid"
)

type TargetResolver interface {
	FavoriteTargetSummary(ctx context.Context, targetType, targetID string) (TargetSummary, *domain.AppError)
}

type Service struct {
	mu          sync.Mutex
	now         func() time.Time
	repo        Repository
	idempotency *idempotency.Service
	resolver    TargetResolver
	items       map[string]Favorite
}

func NewService(repo Repository, idempotencyService *idempotency.Service, resolver TargetResolver, now func() time.Time) *Service {
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
		items:       make(map[string]Favorite),
	}
}

func (s *Service) List(ctx context.Context, userID string) ([]ListItem, *domain.AppError) {
	if strings.TrimSpace(userID) == "" {
		return nil, sessionRequired()
	}
	if s.repo != nil {
		return s.repo.ListFavorites(ctx, userID)
	}
	s.mu.Lock()
	items := make([]Favorite, 0, len(s.items))
	for _, item := range s.items {
		if item.UserID == userID {
			items = append(items, item)
		}
	}
	s.mu.Unlock()

	rows := make([]ListItem, 0, len(items))
	for _, item := range items {
		summary, appErr := s.targetSummary(ctx, item.TargetType, item.TargetID)
		if appErr != nil {
			continue
		}
		rows = append(rows, ListItem{
			Favorite: item,
			Title:    summary.Title,
			Subtitle: summary.Subtitle,
			Status:   summary.Status,
			To:       summary.To,
		})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].CreatedAt.After(rows[j].CreatedAt)
	})
	return rows, nil
}

func (s *Service) IsFavorite(ctx context.Context, userID, targetType, targetID string) (bool, *domain.AppError) {
	targetType, appErr := normalizeTargetType(targetType)
	if appErr != nil {
		return false, appErr
	}
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return false, validationError("targetId", "必须提供收藏目标。")
	}
	if strings.TrimSpace(userID) == "" {
		return false, sessionRequired()
	}
	if s.repo != nil {
		return s.repo.IsFavorite(ctx, userID, targetType, targetID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.items[favoriteKey(userID, targetType, targetID)]
	return ok, nil
}

func (s *Service) CreateWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash, targetType, targetID string, buildCompletion CompletionBuilder) (idempotency.Completion, *domain.AppError) {
	targetType, appErr := normalizeTargetType(targetType)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return idempotency.Completion{}, validationError("targetId", "必须提供收藏目标。")
	}
	if _, appErr := s.targetSummary(ctx, targetType, targetID); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	entry, appErr := s.idempotency.Begin(ctx, userID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}
	if s.repo != nil {
		_, completion, appErr := s.repo.CreateFavoriteWithIdempotency(ctx, *entry, userID, targetType, targetID, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return idempotency.Completion{}, appErr
		}
		return completion, nil
	}

	result, appErr := s.createMemory(ctx, userID, targetType, targetID)
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

func (s *Service) Delete(ctx context.Context, userID, targetType, targetID string) (MutationResult, *domain.AppError) {
	targetType, appErr := normalizeTargetType(targetType)
	if appErr != nil {
		return MutationResult{}, appErr
	}
	targetID = strings.TrimSpace(targetID)
	if targetID == "" {
		return MutationResult{}, validationError("targetId", "必须提供收藏目标。")
	}
	if strings.TrimSpace(userID) == "" {
		return MutationResult{}, sessionRequired()
	}
	if s.repo != nil {
		return s.repo.DeleteFavorite(ctx, userID, targetType, targetID)
	}
	s.mu.Lock()
	delete(s.items, favoriteKey(userID, targetType, targetID))
	s.mu.Unlock()
	return MutationResult{Favorited: false}, nil
}

func (s *Service) createMemory(ctx context.Context, userID, targetType, targetID string) (MutationResult, *domain.AppError) {
	if strings.TrimSpace(userID) == "" {
		return MutationResult{}, sessionRequired()
	}
	summary, appErr := s.targetSummary(ctx, targetType, targetID)
	if appErr != nil {
		return MutationResult{}, appErr
	}
	now := s.now()
	key := favoriteKey(userID, targetType, targetID)
	s.mu.Lock()
	item, ok := s.items[key]
	if !ok {
		item = Favorite{
			ID:         uuid.NewString(),
			UserID:     userID,
			TargetType: targetType,
			TargetID:   targetID,
			CreatedAt:  now,
		}
		s.items[key] = item
	}
	s.mu.Unlock()
	listItem := ListItem{
		Favorite: item,
		Title:    summary.Title,
		Subtitle: summary.Subtitle,
		Status:   summary.Status,
		To:       summary.To,
	}
	return MutationResult{Favorited: true, Favorite: &listItem}, nil
}

func (s *Service) targetSummary(ctx context.Context, targetType, targetID string) (TargetSummary, *domain.AppError) {
	if s.resolver == nil {
		return TargetSummary{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Favorite target resolver missing", "收藏目标解析器不可用。")
	}
	return s.resolver.FavoriteTargetSummary(ctx, targetType, targetID)
}

func normalizeTargetType(value string) (string, *domain.AppError) {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case TargetCarpool:
		return TargetCarpool, nil
	case TargetAPIService, "api-service":
		return TargetAPIService, nil
	default:
		return "", validationError("targetType", "收藏类型不支持。")
	}
}

func favoriteKey(userID, targetType, targetID string) string {
	return userID + ":" + targetType + ":" + targetID
}

func validationError(field, detail string) *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Favorite validation failed", detail, field, "invalid", detail)
}

func sessionRequired() *domain.AppError {
	return domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。")
}
