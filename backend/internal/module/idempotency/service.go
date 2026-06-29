package idempotency

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"c2c-market/backend/internal/domain"
)

type Service struct {
	mu      sync.Mutex
	now     func() time.Time
	repo    Repository
	entries map[string]Entry
}

func NewService(repo Repository, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{
		now:     now,
		repo:    repo,
		entries: make(map[string]Entry),
	}
}

func (s *Service) Begin(ctx context.Context, userID, routeKey, key, requestHash string) (*Entry, *domain.AppError) {
	key = strings.TrimSpace(key)
	if err := ValidateKey(key); err != nil {
		return nil, err
	}
	if s.repo != nil {
		now := s.now()
		return s.repo.BeginIdempotency(ctx, Entry{
			UserID:      userID,
			RouteKey:    routeKey,
			Key:         key,
			RequestHash: requestHash,
			State:       "processing",
			CreatedAt:   now,
			ExpiresAt:   now.Add(24 * time.Hour),
		})
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	mapKey := entryMapKey(userID, routeKey, key)
	entry, ok := s.entries[mapKey]
	if ok {
		if entry.RequestHash != requestHash {
			return nil, domain.NewError(http.StatusConflict, domain.CodeIdempotencyKeyReused, "Idempotency key reused", "同一个 Idempotency-Key 不能用于不同请求。")
		}
		if entry.State == "completed" {
			return &entry, nil
		}
		now := s.now()
		if now.After(entry.ExpiresAt) {
			entry.RequestHash = requestHash
			entry.State = "processing"
			entry.Status = 0
			entry.ContentType = ""
			entry.Body = nil
			entry.ResourceType = ""
			entry.ResourceID = ""
			entry.CompletedAt = nil
			entry.CreatedAt = now
			entry.ExpiresAt = now.Add(24 * time.Hour)
			s.entries[mapKey] = entry
			return &entry, nil
		}
		return nil, domain.NewError(http.StatusConflict, domain.CodeIdempotencyInProgress, "Idempotency request in progress", "相同幂等请求仍在处理中。")
	}

	now := s.now()
	entry = Entry{
		UserID:      userID,
		RouteKey:    routeKey,
		Key:         key,
		RequestHash: requestHash,
		State:       "processing",
		CreatedAt:   now,
		ExpiresAt:   now.Add(24 * time.Hour),
	}
	s.entries[mapKey] = entry
	return &entry, nil
}

func (s *Service) Complete(ctx context.Context, entry *Entry, status int, contentType string, body []byte, resourceType, resourceID string) *domain.AppError {
	if entry == nil {
		return nil
	}
	if s.repo != nil {
		return s.repo.CompleteIdempotency(ctx, entry, status, contentType, body, resourceType, resourceID, s.now())
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	mapKey := entryMapKey(entry.UserID, entry.RouteKey, entry.Key)
	current, ok := s.entries[mapKey]
	if !ok {
		return nil
	}
	now := s.now()
	current.State = "completed"
	current.Status = status
	current.ContentType = contentType
	current.Body = append([]byte(nil), body...)
	current.ResourceType = resourceType
	current.ResourceID = resourceID
	current.CompletedAt = &now
	s.entries[mapKey] = current
	return nil
}

func (s *Service) Cancel(ctx context.Context, entry *Entry) {
	if entry == nil {
		return
	}
	if s.repo != nil {
		_ = s.repo.CancelIdempotency(ctx, entry)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	mapKey := entryMapKey(entry.UserID, entry.RouteKey, entry.Key)
	current, ok := s.entries[mapKey]
	if !ok || current.State != "processing" {
		return
	}
	delete(s.entries, mapKey)
}

func CompletionFromEntry(entry *Entry) Completion {
	if entry == nil {
		return Completion{}
	}
	return Completion{
		Status:       entry.Status,
		ContentType:  entry.ContentType,
		Body:         append([]byte(nil), entry.Body...),
		ResourceType: entry.ResourceType,
		ResourceID:   entry.ResourceID,
	}
}

func ValidateKey(key string) *domain.AppError {
	if key == "" {
		return domain.NewFieldError(http.StatusBadRequest, domain.CodeValidationFailed, "Idempotency key required", "缺少 Idempotency-Key。", "Idempotency-Key", "required", "必须提供 Idempotency-Key。")
	}
	if len(key) > 128 {
		return domain.NewFieldError(http.StatusBadRequest, domain.CodeValidationFailed, "Idempotency key too long", "Idempotency-Key 过长。", "Idempotency-Key", "too_long", "Idempotency-Key 最多 128 个字符。")
	}
	return nil
}

func entryMapKey(userID, routeKey, key string) string {
	return userID + "|" + routeKey + "|" + key
}
