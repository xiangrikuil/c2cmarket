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
			ExpiresAt:   now.Add(ProcessingLifetime),
		})
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	mapKey := entryMapKey(userID, routeKey, key)
	entry, ok := s.entries[mapKey]
	if ok {
		now := s.now()
		if !now.Before(entry.ExpiresAt) {
			entry = newProcessingEntry(userID, routeKey, key, requestHash, now)
			s.entries[mapKey] = entry
			return &entry, nil
		}
		if entry.RequestHash != requestHash {
			return nil, domain.NewError(http.StatusConflict, domain.CodeIdempotencyKeyReused, "Idempotency key reused", "同一个 Idempotency-Key 不能用于不同请求。")
		}
		switch entry.State {
		case "completed":
			return &entry, nil
		case "failed":
			entry = newProcessingEntry(userID, routeKey, key, requestHash, now)
			s.entries[mapKey] = entry
			return &entry, nil
		case "processing":
			return nil, domain.NewError(http.StatusConflict, domain.CodeIdempotencyInProgress, "Idempotency request in progress", "相同幂等请求仍在处理中。")
		default:
			return nil, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "幂等记录状态无效。")
		}
	}

	now := s.now()
	entry = newProcessingEntry(userID, routeKey, key, requestHash, now)
	s.entries[mapKey] = entry
	return &entry, nil
}

func (s *Service) Complete(ctx context.Context, entry *Entry, status int, contentType string, body []byte, resourceType, resourceID string) *domain.AppError {
	if entry == nil {
		return nil
	}
	completion := boundedCompletion(status, contentType, body, resourceType, resourceID)
	if s.repo != nil {
		return s.repo.CompleteIdempotency(ctx, entry, completion, s.now())
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	mapKey := entryMapKey(entry.UserID, entry.RouteKey, entry.Key)
	current, ok := s.entries[mapKey]
	if !ok {
		return nil
	}
	if !sameGeneration(current, *entry) {
		return domain.NewError(http.StatusConflict, domain.CodeIdempotencyInProgress, "Idempotency request superseded", "该幂等请求已被新的执行接管。")
	}
	now := s.now()
	current.State = "completed"
	current.Status = completion.Status
	current.ContentType = completion.ContentType
	current.Body = append([]byte(nil), completion.Body...)
	current.BodyCacheAllowed = !completion.SkipBodyCache
	current.ResourceType = completion.ResourceType
	current.ResourceID = completion.ResourceID
	current.CompletedAt = &now
	current.ExpiresAt = now.Add(CompletedRetention)
	s.entries[mapKey] = current
	return nil
}

func (s *Service) Cancel(ctx context.Context, entry *Entry) {
	if entry == nil {
		return
	}
	failedAt := s.now()
	if s.repo != nil {
		_ = s.repo.CancelIdempotency(ctx, entry, failedAt)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	mapKey := entryMapKey(entry.UserID, entry.RouteKey, entry.Key)
	current, ok := s.entries[mapKey]
	if !ok || current.State != "processing" || !sameGeneration(current, *entry) {
		return
	}
	current.State = "failed"
	current.Status = 0
	current.ContentType = ""
	current.Body = nil
	current.BodyCacheAllowed = false
	current.ResourceType = ""
	current.ResourceID = ""
	current.CompletedAt = &failedAt
	current.ExpiresAt = failedAt.Add(FailedRetention)
	s.entries[mapKey] = current
}

func CompletionFromEntry(entry *Entry) Completion {
	if entry == nil {
		return Completion{}
	}
	if entry.State == "completed" && !entry.BodyCacheAllowed {
		return Completion{
			Status:       http.StatusConflict,
			ContentType:  "application/problem+json",
			Body:         []byte(`{"type":"about:blank","title":"Idempotency result not replayable","status":409,"code":"IDEMPOTENCY_RESULT_NOT_REPLAYABLE","detail":"该请求已处理，但原响应未缓存；请重新读取目标资源。"}`),
			ResourceType: entry.ResourceType,
			ResourceID:   entry.ResourceID,
		}
	}
	return Completion{
		Status:       entry.Status,
		ContentType:  entry.ContentType,
		Body:         append([]byte(nil), entry.Body...),
		ResourceType: entry.ResourceType,
		ResourceID:   entry.ResourceID,
	}
}

func newProcessingEntry(userID, routeKey, key, requestHash string, now time.Time) Entry {
	return Entry{
		UserID:      userID,
		RouteKey:    routeKey,
		Key:         key,
		RequestHash: requestHash,
		State:       "processing",
		CreatedAt:   now,
		ExpiresAt:   now.Add(ProcessingLifetime),
	}
}

func sameGeneration(current, entry Entry) bool {
	return current.State == "processing" &&
		current.RequestHash == entry.RequestHash &&
		current.CreatedAt.Equal(entry.CreatedAt)
}

func boundedCompletion(status int, contentType string, body []byte, resourceType, resourceID string) Completion {
	completion := Completion{
		Status:       status,
		ContentType:  contentType,
		ResourceType: resourceType,
		ResourceID:   resourceID,
	}
	if len(body) > MaxCachedResponseBodySize {
		completion.SkipBodyCache = true
		return completion
	}
	completion.Body = append([]byte(nil), body...)
	return completion
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
