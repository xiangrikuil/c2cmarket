package notification

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"c2c-market/backend/internal/domain"

	"github.com/google/uuid"
)

type Service struct {
	mu    sync.Mutex
	now   func() time.Time
	repo  Repository
	items map[string]Notification
}

func NewService(repo Repository, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{
		now:   now,
		repo:  repo,
		items: make(map[string]Notification),
	}
}

func (s *Service) List(ctx context.Context, userID string, page domain.PageRequest) (domain.Page[Notification], *domain.AppError) {
	if appErr := validateUserID(userID); appErr != nil {
		return domain.Page[Notification]{}, appErr
	}
	if s.repo != nil {
		return s.repo.ListNotifications(ctx, userID, page)
	}
	s.mu.Lock()
	items := make([]Notification, 0, len(s.items))
	for _, item := range s.items {
		if item.UserID == userID {
			items = append(items, item)
		}
	}
	s.mu.Unlock()
	sortNotifications(items)
	return domain.PageItems(items, page), nil
}

func (s *Service) UnreadCount(ctx context.Context, userID string) (int, *domain.AppError) {
	if appErr := validateUserID(userID); appErr != nil {
		return 0, appErr
	}
	if s.repo != nil {
		return s.repo.UnreadNotificationCount(ctx, userID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, item := range s.items {
		if item.UserID == userID && item.ReadAt == nil {
			count++
		}
	}
	return count, nil
}

func (s *Service) MarkRead(ctx context.Context, userID, notificationID string) (Notification, *domain.AppError) {
	if appErr := validateUserID(userID); appErr != nil {
		return Notification{}, appErr
	}
	notificationID = strings.TrimSpace(notificationID)
	if notificationID == "" {
		return Notification{}, validationError("id", "必须提供通知 ID。")
	}
	if s.repo != nil {
		return s.repo.MarkNotificationRead(ctx, userID, notificationID, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[notificationID]
	if !ok || item.UserID != userID {
		return Notification{}, notFound()
	}
	if item.ReadAt == nil {
		now := s.now()
		item.ReadAt = &now
		s.items[item.ID] = item
	}
	return item, nil
}

func (s *Service) MarkAllRead(ctx context.Context, userID string) (ReadAllResult, *domain.AppError) {
	if appErr := validateUserID(userID); appErr != nil {
		return ReadAllResult{}, appErr
	}
	if s.repo != nil {
		return s.repo.MarkAllNotificationsRead(ctx, userID, s.now())
	}
	now := s.now()
	count := 0
	s.mu.Lock()
	for id, item := range s.items {
		if item.UserID == userID && item.ReadAt == nil {
			item.ReadAt = &now
			s.items[id] = item
			count++
		}
	}
	items := make([]Notification, 0, len(s.items))
	for _, item := range s.items {
		if item.UserID == userID {
			items = append(items, item)
		}
	}
	s.mu.Unlock()
	sortNotifications(items)
	return ReadAllResult{Count: count, Items: items}, nil
}

func (s *Service) Add(item Notification) Notification {
	now := s.now()
	if item.ID == "" {
		item.ID = uuid.NewString()
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	s.mu.Lock()
	s.items[item.ID] = item
	s.mu.Unlock()
	return item
}

func sortNotifications(items []Notification) {
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
}

func validateUserID(userID string) *domain.AppError {
	if strings.TrimSpace(userID) == "" {
		return domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。")
	}
	return nil
}

func validationError(field, detail string) *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Notification validation failed", detail, field, "invalid", detail)
}

func notFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Notification not found", "通知不存在。")
}
