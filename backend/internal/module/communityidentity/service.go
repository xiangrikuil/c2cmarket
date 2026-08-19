package communityidentity

import (
	"context"
	"net/http"
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
	cutoff      time.Time
	items       map[string]Identity
}

func NewService(repo Repository, idempotencyService *idempotency.Service, now func() time.Time, cutoff time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	if cutoff.IsZero() {
		cutoff, _ = time.Parse(time.RFC3339, DefaultFoundingCutoff)
	}
	return &Service{
		now: now, repo: repo, idempotency: idempotencyService, cutoff: cutoff,
		items: make(map[string]Identity),
	}
}

func (s *Service) GrantFounding(ctx context.Context, input GrantFoundingInput) (Identity, bool, *domain.AppError) {
	if err := validateUserID(input.UserID); err != nil {
		return Identity{}, false, err
	}
	if input.QualifiedAt.IsZero() || input.QualifiedAt.After(s.cutoff) {
		return Identity{}, false, nil
	}
	if input.Source == "" {
		input.Source = SourceAuto
	}
	if input.Source != SourceAuto && input.Source != SourceBackfill {
		return Identity{}, false, validationError("source", "自动发放来源无效。")
	}
	if s.repo != nil {
		return s.repo.GrantFounding(ctx, input, s.now())
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	key := identityKey(input.UserID, IdentityTypeFoundingUser)
	if existing, ok := s.items[key]; ok {
		return existing, false, nil
	}
	now := s.now()
	item := Identity{ID: newID(), UserID: input.UserID, Type: IdentityTypeFoundingUser, Source: input.Source, QualifiedAt: timePointer(input.QualifiedAt), GrantedAt: now, CreatedAt: now, UpdatedAt: now}
	s.items[key] = item
	return item, true, nil
}

// GrantFoundingForUser applies account eligibility before checking the cutoff.
func (s *Service) GrantFoundingForUser(ctx context.Context, user auth.User, qualifiedAt time.Time) (Identity, bool, *domain.AppError) {
	if strings.TrimSpace(user.ID) == "" {
		return Identity{}, false, domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。")
	}
	if user.IsAdmin || user.Status != auth.AccountStatusActive {
		return Identity{}, false, nil
	}
	return s.GrantFounding(ctx, GrantFoundingInput{UserID: user.ID, QualifiedAt: qualifiedAt, Source: SourceAuto})
}

func (s *Service) ListForUser(ctx context.Context, userID string, includeRevoked bool) ([]Identity, *domain.AppError) {
	if err := validateUserID(userID); err != nil {
		return nil, err
	}
	if s.repo != nil {
		return s.repo.ListForUser(ctx, userID, includeRevoked)
	}
	s.mu.Lock()
	items := make([]Identity, 0)
	for _, item := range s.items {
		if item.UserID == userID && (includeRevoked || item.RevokedAt == nil) {
			items = append(items, item)
		}
	}
	s.mu.Unlock()
	sortIdentities(items)
	return items, nil
}

func (s *Service) PublicForUser(ctx context.Context, userID string) ([]PublicIdentity, *domain.AppError) {
	items, appErr := s.ListForUser(ctx, userID, false)
	if appErr != nil {
		return nil, appErr
	}
	result := make([]PublicIdentity, 0, len(items))
	for _, item := range items {
		result = append(result, ToPublic(item))
	}
	return result, nil
}

func (s *Service) GrantAdmin(ctx context.Context, actor auth.User, input GrantAdminInput) (Identity, *domain.AppError) {
	if err := requireAdmin(actor); err != nil {
		return Identity{}, err
	}
	input.AdminUserID = actor.ID
	input.Type = IdentityType(strings.TrimSpace(string(input.Type)))
	input.Reason = strings.TrimSpace(input.Reason)
	if err := validateGrantAdminInput(input); err != nil {
		return Identity{}, err
	}
	if s.repo != nil {
		item, created, appErr := s.repo.GrantAdmin(ctx, input, s.now())
		if appErr != nil {
			return Identity{}, appErr
		}
		if !created {
			return Identity{}, duplicateError()
		}
		return item, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := identityKey(input.TargetUserID, input.Type)
	if _, ok := s.items[key]; ok {
		return Identity{}, duplicateError()
	}
	now := s.now()
	item := Identity{ID: newID(), UserID: input.TargetUserID, Type: input.Type, Source: SourceAdmin, GrantedAt: now, GrantedBy: input.AdminUserID, GrantReason: input.Reason, CreatedAt: now, UpdatedAt: now}
	s.items[key] = item
	return item, nil
}

func (s *Service) GrantAdminWithIdempotency(ctx context.Context, actor auth.User, routeKey, key, requestHash string, input GrantAdminInput, buildCompletion func(Identity) (idempotency.Completion, *domain.AppError)) (idempotency.Completion, *domain.AppError) {
	if err := requireAdmin(actor); err != nil {
		return idempotency.Completion{}, err
	}
	if s.idempotency == nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "社区身份幂等服务未配置。")
	}
	entry, appErr := s.idempotency.Begin(ctx, actor.ID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}
	item, appErr := s.GrantAdmin(ctx, actor, input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(item)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	if appErr := s.idempotency.Complete(ctx, entry, completion.Status, completion.ContentType, completion.Body, completion.ResourceType, completion.ResourceID); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	return completion, nil
}

func (s *Service) Revoke(ctx context.Context, actor auth.User, input RevokeInput) (Identity, *domain.AppError) {
	if err := requireAdmin(actor); err != nil {
		return Identity{}, err
	}
	input.AdminUserID = actor.ID
	input.Type = IdentityType(strings.TrimSpace(string(input.Type)))
	input.Reason = strings.TrimSpace(input.Reason)
	if err := validateRevokeInput(input); err != nil {
		return Identity{}, err
	}
	if s.repo != nil {
		return s.repo.Revoke(ctx, input, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := identityKey(input.TargetUserID, input.Type)
	item, ok := s.items[key]
	if !ok {
		return Identity{}, notFoundError()
	}
	if item.RevokedAt != nil {
		return Identity{}, duplicateError()
	}
	now := s.now()
	item.RevokedAt, item.RevokedBy, item.RevokeReason, item.UpdatedAt = timePointer(now), input.AdminUserID, input.Reason, now
	s.items[key] = item
	return item, nil
}

func (s *Service) RevokeWithIdempotency(ctx context.Context, actor auth.User, routeKey, key, requestHash string, input RevokeInput, buildCompletion func(Identity) (idempotency.Completion, *domain.AppError)) (idempotency.Completion, *domain.AppError) {
	if err := requireAdmin(actor); err != nil {
		return idempotency.Completion{}, err
	}
	if s.idempotency == nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "社区身份幂等服务未配置。")
	}
	entry, appErr := s.idempotency.Begin(ctx, actor.ID, routeKey, key, requestHash)
	if appErr != nil {
		return idempotency.Completion{}, appErr
	}
	if entry.State == "completed" {
		return idempotency.CompletionFromEntry(entry), nil
	}
	item, appErr := s.Revoke(ctx, actor, input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	completion, appErr := buildCompletion(item)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return idempotency.Completion{}, appErr
	}
	if appErr := s.idempotency.Complete(ctx, entry, completion.Status, completion.ContentType, completion.Body, completion.ResourceType, completion.ResourceID); appErr != nil {
		return idempotency.Completion{}, appErr
	}
	return completion, nil
}

func (s *Service) BackfillFounding(ctx context.Context) (int, *domain.AppError) {
	if s.repo != nil {
		return s.repo.BackfillFounding(ctx, s.cutoff, s.now())
	}
	return 0, nil
}

func sortIdentities(items []Identity) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Type != items[j].Type {
			return items[i].Type == IdentityTypeBetaContributor
		}
		return items[i].GrantedAt.Before(items[j].GrantedAt)
	})
}

func validateGrantAdminInput(input GrantAdminInput) *domain.AppError {
	if err := validateUserID(input.TargetUserID); err != nil {
		return err
	}
	if !IsKnownType(input.Type) {
		return validationError("identityType", "社区身份类型无效。")
	}
	if input.Type != IdentityTypeBetaContributor {
		return validationError("identityType", "管理员手动发放只支持内测共建者。")
	}
	if input.Reason == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Reason required", "发放内测共建者必须填写原因。", "reason", "required", "必须填写发放原因。")
	}
	return nil
}

func validateRevokeInput(input RevokeInput) *domain.AppError {
	if err := validateUserID(input.TargetUserID); err != nil {
		return err
	}
	if !IsKnownType(input.Type) {
		return validationError("identityType", "社区身份类型无效。")
	}
	if input.Reason == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Reason required", "撤销社区身份必须填写原因。", "reason", "required", "必须填写撤销原因。")
	}
	return nil
}

func requireAdmin(user auth.User) *domain.AppError {
	if !user.IsAdmin {
		return domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "只有管理员可以管理社区身份。")
	}
	return nil
}

func validateUserID(value string) *domain.AppError {
	if strings.TrimSpace(value) == "" {
		return domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。")
	}
	return nil
}

func validationError(field, detail string) *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Community identity validation failed", detail, field, "invalid", detail)
}

func duplicateError() *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Community identity already exists", "该用户已经拥有该社区身份。")
}

func notFoundError() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Community identity not found", "社区身份不存在。")
}

func identityKey(userID string, identityType IdentityType) string {
	return userID + ":" + string(identityType)
}
func timePointer(value time.Time) *time.Time { copy := value; return &copy }
func newID() string                          { return uuid.NewString() }
