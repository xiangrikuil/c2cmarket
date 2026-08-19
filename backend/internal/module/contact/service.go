package contact

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/reputation"

	"github.com/google/uuid"
)

type ActionChecker interface {
	CheckActionAllowed(ctx context.Context, userID, role, action string) *domain.AppError
}

type EmailSender interface {
	SendVerificationCode(ctx context.Context, toEmail, code string, expiresAt time.Time) *domain.AppError
	ExposeDevCode() bool
}

type developmentEmailSender struct{}

func (developmentEmailSender) SendVerificationCode(context.Context, string, string, time.Time) *domain.AppError {
	return nil
}

func (developmentEmailSender) ExposeDevCode() bool { return true }

type ServiceOptions struct {
	EmailVerificationPepper string
	EmailSender             EmailSender
}

const (
	ContactEmailVerificationLifetime    = 15 * time.Minute
	ContactEmailVerificationMaxAttempts = 5
	localEmailVerificationPepper        = "c2cmarket-local-email-verification-pepper-v1"
)

type Service struct {
	mu                sync.Mutex
	now               func() time.Time
	repo              Repository
	idempotency       *idempotency.Service
	actionChecker     ActionChecker
	methods           map[string]ContactMethod
	versions          map[string]ContactMethodVersion
	sessions          map[string]ContactSession
	accessLogs        map[string]ContactAccessLog
	methodsByUserKey  map[string]string
	methodAuditEvents []ContactMethodAuditEvent
	emailChallenges   map[string]ContactEmailVerificationCode
	emailSender       EmailSender
	emailPepper       []byte
}

func NewService(repo Repository, now func() time.Time) *Service {
	return NewServiceWithOptions(repo, now, ServiceOptions{})
}

func NewServiceWithOptions(repo Repository, now func() time.Time, options ServiceOptions) *Service {
	if now == nil {
		now = time.Now
	}
	pepper := strings.TrimSpace(options.EmailVerificationPepper)
	if pepper == "" {
		pepper = localEmailVerificationPepper
	}
	if options.EmailSender == nil {
		options.EmailSender = developmentEmailSender{}
	}
	var idempotencyRepo idempotency.Repository
	if candidate, ok := repo.(idempotency.Repository); ok {
		idempotencyRepo = candidate
	}
	return &Service{
		now:              now,
		repo:             repo,
		idempotency:      idempotency.NewService(idempotencyRepo, now),
		methods:          make(map[string]ContactMethod),
		versions:         make(map[string]ContactMethodVersion),
		sessions:         make(map[string]ContactSession),
		accessLogs:       make(map[string]ContactAccessLog),
		methodsByUserKey: make(map[string]string),
		emailChallenges:  make(map[string]ContactEmailVerificationCode),
		emailSender:      options.EmailSender,
		emailPepper:      []byte(pepper),
	}
}

func (s *Service) CreateMethodWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input ContactMethodInput, buildCompletion MethodCompletionBuilder) (ContactMethod, idempotency.Completion, bool, *domain.AppError) {
	if err := idempotency.ValidateKey(strings.TrimSpace(key)); err != nil {
		return ContactMethod{}, idempotency.Completion{}, false, err
	}
	if buildCompletion == nil {
		return ContactMethod{}, idempotency.Completion{}, false, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	input.UserID = userID
	input.Type, input.Value = normalizeMethodTypeAndValue(input.Type, input.Value)
	if appErr := validateMethodInput(input.Type, input.Value); appErr != nil {
		return ContactMethod{}, idempotency.Completion{}, false, appErr
	}
	usageScopes, appErr := normalizedUsageScopesForMethod(input.Type, input.UsageScopes)
	if appErr != nil {
		return ContactMethod{}, idempotency.Completion{}, false, appErr
	}
	input.UsageScopes = usageScopes
	now := s.now()
	method, version := NewMethodVersion(input, now)
	entry, appErr := s.idempotency.Begin(ctx, userID, routeKey, strings.TrimSpace(key), requestHash)
	if appErr != nil {
		return ContactMethod{}, idempotency.Completion{}, false, appErr
	}
	if entry.State == "completed" {
		return ContactMethod{}, idempotency.CompletionFromEntry(entry), false, nil
	}
	if s.repo != nil {
		method, completion, appErr := s.repo.CreateContactMethodWithIdempotency(ctx, *entry, input, method, version, buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return ContactMethod{}, idempotency.Completion{}, false, appErr
		}
		return method, completion, true, nil
	}

	s.mu.Lock()
	if method.Enabled && method.Type == MethodTypeWechat && s.hasOtherEnabledWechatLocked(method.UserID, "") {
		s.mu.Unlock()
		s.idempotency.Cancel(ctx, entry)
		return ContactMethod{}, idempotency.Completion{}, false, DuplicateEnabledWechatError()
	}
	if method.IsDefault {
		for id, item := range s.methods {
			if item.UserID == method.UserID && item.IsDefault {
				item.IsDefault = false
				item.UpdatedAt = now
				item.Version++
				s.methods[id] = cloneContactMethod(item)
				s.appendMethodAuditEventLocked(item, "contact_method.default_changed", input.RequestID, []string{"isDefault"})
			}
		}
	}
	s.methods[method.ID] = cloneContactMethod(method)
	s.versions[version.ID] = version
	s.methodsByUserKey[methodKey(method.UserID, method.ID)] = method.ID
	s.appendMethodAuditEventLocked(method, "contact_method.created", input.RequestID, []string{"type", "label", "value", "usageScopes", "isDefault", "enabled"})
	s.mu.Unlock()

	completion, appErr := buildCompletion(method)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return ContactMethod{}, idempotency.Completion{}, false, appErr
	}
	if appErr := s.idempotency.Complete(ctx, entry, completion.Status, completion.ContentType, completion.Body, completion.ResourceType, completion.ResourceID); appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return ContactMethod{}, idempotency.Completion{}, false, appErr
	}
	return method, completion, true, nil
}

func (s *Service) SetActionChecker(checker ActionChecker) {
	s.actionChecker = checker
}

func (s *Service) CreateMethod(ctx context.Context, input ContactMethodInput) (ContactMethod, *domain.AppError) {
	input.Type, input.Value = normalizeMethodTypeAndValue(input.Type, input.Value)
	if appErr := validateMethodInput(input.Type, input.Value); appErr != nil {
		return ContactMethod{}, appErr
	}
	usageScopes, appErr := normalizedUsageScopesForMethod(input.Type, input.UsageScopes)
	if appErr != nil {
		return ContactMethod{}, appErr
	}
	input.UsageScopes = usageScopes

	now := s.now()
	method, version := NewMethodVersion(input, now)
	if s.repo != nil {
		if appErr := s.repo.CreateContactMethod(ctx, input, method, version); appErr != nil {
			return ContactMethod{}, appErr
		}
		return method, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if method.Enabled && method.Type == MethodTypeWechat && s.hasOtherEnabledWechatLocked(method.UserID, "") {
		return ContactMethod{}, DuplicateEnabledWechatError()
	}

	if method.IsDefault {
		for id, item := range s.methods {
			if item.UserID == method.UserID && item.IsDefault {
				item.IsDefault = false
				item.UpdatedAt = now
				item.Version++
				s.methods[id] = cloneContactMethod(item)
				s.appendMethodAuditEventLocked(item, "contact_method.default_changed", input.RequestID, []string{"isDefault"})
			}
		}
	}
	s.methods[method.ID] = cloneContactMethod(method)
	s.versions[version.ID] = version
	s.methodsByUserKey[methodKey(method.UserID, method.ID)] = method.ID
	s.appendMethodAuditEventLocked(method, "contact_method.created", input.RequestID, []string{"type", "label", "value", "usageScopes", "isDefault", "enabled"})
	return cloneContactMethod(method), nil
}

func (s *Service) ListMethods(ctx context.Context, userID string) ([]ContactMethod, *domain.AppError) {
	if s.repo != nil {
		return s.repo.ListContactMethods(ctx, userID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	methods := make([]ContactMethod, 0)
	for _, method := range s.methods {
		if method.UserID == userID {
			methods = append(methods, cloneContactMethod(method))
		}
	}
	return methods, nil
}

// EnsureLinuxDoMethod 将权威 linux.do 身份绑定投影为交易快照使用的版本化联系方式。
func (s *Service) EnsureLinuxDoMethod(ctx context.Context, userID, username string) (ContactMethod, *domain.AppError) {
	userID = strings.TrimSpace(userID)
	username = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(username), "@"))
	if userID == "" || username == "" {
		return ContactMethod{}, domain.NewError(http.StatusConflict, domain.CodeLinuxDoBindingRequired, "linux.do binding required", "当前账号尚未完成 linux.do 身份绑定。")
	}

	methods, appErr := s.ListMethods(ctx, userID)
	if appErr != nil {
		return ContactMethod{}, appErr
	}
	var current *ContactMethod
	hasDefault := false
	for index := range methods {
		method := methods[index]
		if !method.Enabled {
			continue
		}
		hasDefault = hasDefault || method.IsDefault
		if method.Type == "linuxdo" && (current == nil || (!current.IsDefault && method.IsDefault)) {
			copy := method
			current = &copy
		}
	}
	expectedValue := "@" + username
	expectedUsageScopes := AllUsageScopes()
	if current != nil {
		if strings.EqualFold(strings.TrimSpace(current.DisplayValue), expectedValue) &&
			strings.TrimSpace(current.Label) == "linux.do 私信" &&
			equalUsageScopes(current.UsageScopes, expectedUsageScopes) {
			return *current, nil
		}
		return s.UpdateMethod(ctx, UpdateContactMethodInput{
			UserID: userID, MethodID: current.ID, Type: "linuxdo", Label: "linux.do 私信",
			Value: expectedValue, UsageScopes: expectedUsageScopes, IsDefault: current.IsDefault, Enabled: true,
		})
	}

	created, appErr := s.CreateMethod(ctx, ContactMethodInput{
		UserID: userID, Type: "linuxdo", Label: "linux.do 私信", Value: expectedValue,
		UsageScopes: expectedUsageScopes, IsDefault: !hasDefault, Enabled: true,
	})
	if appErr == nil {
		return created, nil
	}
	// 数据库唯一索引负责并发收敛；竞争失败后读取胜出的映射。
	methods, retryErr := s.ListMethods(ctx, userID)
	if retryErr == nil {
		for _, method := range methods {
			if method.Enabled && method.Type == "linuxdo" {
				if strings.EqualFold(strings.TrimSpace(method.DisplayValue), expectedValue) &&
					strings.TrimSpace(method.Label) == "linux.do 私信" &&
					equalUsageScopes(method.UsageScopes, expectedUsageScopes) {
					return method, nil
				}
				return s.UpdateMethod(ctx, UpdateContactMethodInput{
					UserID: userID, MethodID: method.ID, Type: "linuxdo", Label: "linux.do 私信",
					Value: expectedValue, UsageScopes: expectedUsageScopes, IsDefault: method.IsDefault, Enabled: true,
				})
			}
		}
	}
	return ContactMethod{}, appErr
}

func (s *Service) UpdateMethod(ctx context.Context, input UpdateContactMethodInput) (ContactMethod, *domain.AppError) {
	input.Type, input.Value = normalizeMethodTypeAndValue(input.Type, input.Value)
	if appErr := validateMethodInput(input.Type, input.Value); appErr != nil {
		return ContactMethod{}, appErr
	}
	if input.Type == MethodTypeWechat {
		input.UsageScopes = AllUsageScopes()
	} else if input.UsageScopes != nil {
		usageScopes, appErr := normalizeUsageScopes(input.UsageScopes)
		if appErr != nil {
			return ContactMethod{}, appErr
		}
		input.UsageScopes = usageScopes
	}
	now := s.now()
	method, version := NewUpdatedMethodVersion(input, now)
	if s.repo != nil {
		return s.repo.UpdateContactMethod(ctx, input, method, version)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	current, ok := s.methods[input.MethodID]
	if !ok || current.UserID != input.UserID {
		return ContactMethod{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Contact method not found", "联系方式不存在。")
	}
	if appErr := validateRequiredWechatUpdate(current, input); appErr != nil {
		return ContactMethod{}, appErr
	}
	if method.Enabled && method.Type == MethodTypeWechat && s.hasOtherEnabledWechatLocked(input.UserID, input.MethodID) {
		return ContactMethod{}, DuplicateEnabledWechatError()
	}
	if input.IsDefault {
		for id, item := range s.methods {
			if item.UserID == input.UserID && item.ID != input.MethodID && item.IsDefault {
				item.IsDefault = false
				item.UpdatedAt = now
				item.Version++
				s.methods[id] = cloneContactMethod(item)
				s.appendMethodAuditEventLocked(item, "contact_method.default_changed", input.RequestID, []string{"isDefault"})
			}
		}
	}
	method.ID = current.ID
	method.UserID = current.UserID
	method.CreatedAt = current.CreatedAt
	if input.UsageScopes == nil {
		method.UsageScopes = append([]string(nil), current.UsageScopes...)
	}
	method.Version = current.Version + 1
	valueChanged := current.Type != method.Type || current.DisplayValue != method.DisplayValue
	if valueChanged {
		version.ContactMethodID = method.ID
		method.CurrentVersionID = version.ID
		method.VerifiedAt = nil
		s.versions[version.ID] = version
	} else {
		method.CurrentVersionID = current.CurrentVersionID
		method.VerifiedAt = current.VerifiedAt
		method.MaskedValue = current.MaskedValue
		method.DisplayValue = current.DisplayValue
	}
	s.methods[method.ID] = cloneContactMethod(method)
	eventType := "contact_method.updated"
	if current.Enabled && !method.Enabled {
		eventType = "contact_method.disabled"
	}
	s.appendMethodAuditEventLocked(method, eventType, input.RequestID, []string{"type", "label", "value", "usageScopes", "isDefault", "enabled"})
	return cloneContactMethod(method), nil
}

func (s *Service) UpdateMethodWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input UpdateContactMethodInput, buildCompletion MethodCompletionBuilder) (ContactMethod, idempotency.Completion, bool, *domain.AppError) {
	input.UserID = userID
	input.Type, input.Value = normalizeMethodTypeAndValue(input.Type, input.Value)
	if appErr := validateMethodInput(input.Type, input.Value); appErr != nil {
		return ContactMethod{}, idempotency.Completion{}, false, appErr
	}
	if input.Type == MethodTypeWechat {
		input.UsageScopes = AllUsageScopes()
	} else if input.UsageScopes != nil {
		usageScopes, appErr := normalizeUsageScopes(input.UsageScopes)
		if appErr != nil {
			return ContactMethod{}, idempotency.Completion{}, false, appErr
		}
		input.UsageScopes = usageScopes
	}
	entry, completion, replay, appErr := s.beginMethodIdempotency(ctx, userID, routeKey, key, requestHash, buildCompletion)
	if appErr != nil || replay {
		return ContactMethod{}, completion, false, appErr
	}
	now := s.now()
	method, version := NewUpdatedMethodVersion(input, now)
	if s.repo != nil {
		method, completion, appErr = s.repo.UpdateContactMethodWithIdempotency(ctx, *entry, input, method, version, buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return ContactMethod{}, idempotency.Completion{}, false, appErr
		}
		return method, completion, true, nil
	}
	method, appErr = s.UpdateMethod(ctx, input)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return ContactMethod{}, idempotency.Completion{}, false, appErr
	}
	return s.completeMemoryMethodMutation(ctx, entry, method, buildCompletion)
}

func (s *Service) DeleteMethod(ctx context.Context, userID, methodID string) (ContactMethod, *domain.AppError) {
	return s.DeleteMethodWithRequestID(ctx, userID, methodID, "unknown")
}

func (s *Service) DeleteMethodWithRequestID(ctx context.Context, userID, methodID, requestID string) (ContactMethod, *domain.AppError) {
	now := s.now()
	if s.repo != nil {
		return s.repo.DeleteContactMethod(ctx, userID, methodID, requestID, now)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	method, ok := s.methods[methodID]
	if !ok || method.UserID != userID {
		return ContactMethod{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Contact method not found", "联系方式不存在。")
	}
	if method.Type == "linuxdo" || method.Type == "wechat" {
		return ContactMethod{}, protectedContactDeleteError(method.Type)
	}
	method.Enabled = false
	method.UpdatedAt = now
	method.Version++
	s.methods[method.ID] = cloneContactMethod(method)
	s.appendMethodAuditEventLocked(method, "contact_method.disabled", requestID, []string{"enabled", "isDefault"})
	return cloneContactMethod(method), nil
}

func (s *Service) DeleteMethodWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash, methodID, requestID string, buildCompletion MethodCompletionBuilder) (ContactMethod, idempotency.Completion, bool, *domain.AppError) {
	entry, completion, replay, appErr := s.beginMethodIdempotency(ctx, userID, routeKey, key, requestHash, buildCompletion)
	if appErr != nil || replay {
		return ContactMethod{}, completion, false, appErr
	}
	if s.repo != nil {
		method, completion, appErr := s.repo.DeleteContactMethodWithIdempotency(ctx, *entry, userID, methodID, requestID, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return ContactMethod{}, idempotency.Completion{}, false, appErr
		}
		return method, completion, true, nil
	}
	method, appErr := s.DeleteMethodWithRequestID(ctx, userID, methodID, requestID)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return ContactMethod{}, idempotency.Completion{}, false, appErr
	}
	return s.completeMemoryMethodMutation(ctx, entry, method, buildCompletion)
}

func (s *Service) SetDefaultMethod(ctx context.Context, userID, methodID string) (ContactMethod, *domain.AppError) {
	return s.SetDefaultMethodWithRequestID(ctx, userID, methodID, "unknown")
}

func (s *Service) SetDefaultMethodWithRequestID(ctx context.Context, userID, methodID, requestID string) (ContactMethod, *domain.AppError) {
	now := s.now()
	if s.repo != nil {
		return s.repo.SetDefaultContactMethod(ctx, userID, methodID, requestID, now)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	method, ok := s.methods[methodID]
	if !ok || method.UserID != userID || !method.Enabled {
		return ContactMethod{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Contact method not found", "联系方式不存在。")
	}
	for id, item := range s.methods {
		if item.UserID == userID {
			wasDefault := item.IsDefault
			item.IsDefault = item.ID == methodID
			if wasDefault != item.IsDefault || item.ID == methodID {
				item.UpdatedAt = now
				item.Version++
				s.appendMethodAuditEventLocked(item, "contact_method.default_changed", requestID, []string{"isDefault"})
			}
			s.methods[id] = item
		}
	}
	return cloneContactMethod(s.methods[methodID]), nil
}

func (s *Service) SetDefaultMethodWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash, methodID, requestID string, buildCompletion MethodCompletionBuilder) (ContactMethod, idempotency.Completion, bool, *domain.AppError) {
	entry, completion, replay, appErr := s.beginMethodIdempotency(ctx, userID, routeKey, key, requestHash, buildCompletion)
	if appErr != nil || replay {
		return ContactMethod{}, completion, false, appErr
	}
	if s.repo != nil {
		method, completion, appErr := s.repo.SetDefaultContactMethodWithIdempotency(ctx, *entry, userID, methodID, requestID, s.now(), buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return ContactMethod{}, idempotency.Completion{}, false, appErr
		}
		return method, completion, true, nil
	}
	method, appErr := s.SetDefaultMethodWithRequestID(ctx, userID, methodID, requestID)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return ContactMethod{}, idempotency.Completion{}, false, appErr
	}
	return s.completeMemoryMethodMutation(ctx, entry, method, buildCompletion)
}

func (s *Service) StartEmailVerification(ctx context.Context, userID, methodID string) (ContactEmailVerificationChallenge, *domain.AppError) {
	method, appErr := s.emailMethod(ctx, userID, methodID)
	if appErr != nil {
		return ContactEmailVerificationChallenge{}, appErr
	}
	now := s.now()
	code := newContactEmailVerificationCode()
	expiresAt := now.Add(ContactEmailVerificationLifetime)
	email := normalizeContactEmail(method.DisplayValue)
	challenge := ContactEmailVerificationCode{
		UserID:                 userID,
		ContactMethodID:        method.ID,
		ContactMethodVersionID: method.CurrentVersionID,
		Email:                  email,
		CodeHash:               contactEmailCodeHash(s.emailPepper, userID, method.ID, method.CurrentVersionID, email, code),
		ExpiresAt:              expiresAt,
		CreatedAt:              now,
	}
	if s.repo != nil {
		if appErr := s.repo.CreateContactEmailVerificationCode(ctx, challenge); appErr != nil {
			return ContactEmailVerificationChallenge{}, appErr
		}
	} else {
		s.mu.Lock()
		current, ok := s.methods[method.ID]
		if !ok || current.UserID != userID || !current.Enabled || current.Type != "email" || current.CurrentVersionID != method.CurrentVersionID {
			s.mu.Unlock()
			return ContactEmailVerificationChallenge{}, contactEmailVerificationInvalidStateError()
		}
		s.emailChallenges[method.ID] = challenge
		s.mu.Unlock()
	}
	if appErr := s.emailSender.SendVerificationCode(ctx, email, code, expiresAt); appErr != nil {
		return ContactEmailVerificationChallenge{}, appErr
	}
	result := ContactEmailVerificationChallenge{
		ContactMethodID:        method.ID,
		ContactMethodVersionID: method.CurrentVersionID,
		Email:                  email,
		ExpiresAt:              expiresAt,
	}
	if s.emailSender.ExposeDevCode() {
		result.DevCode = code
	}
	return result, nil
}

func (s *Service) ConfirmEmailVerificationWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash, methodID, code, requestID string, buildCompletion MethodCompletionBuilder) (ContactMethod, idempotency.Completion, bool, *domain.AppError) {
	code = strings.TrimSpace(code)
	if !contactEmailCodePattern.MatchString(code) {
		return ContactMethod{}, idempotency.Completion{}, false, invalidContactEmailVerificationCodeError()
	}
	method, appErr := s.emailMethod(ctx, userID, methodID)
	if appErr != nil {
		return ContactMethod{}, idempotency.Completion{}, false, appErr
	}
	entry, completion, replay, appErr := s.beginMethodIdempotency(ctx, userID, routeKey, key, requestHash, buildCompletion)
	if appErr != nil || replay {
		return ContactMethod{}, completion, false, appErr
	}
	now := s.now()
	input := ConfirmContactEmailInput{
		UserID:                 userID,
		ContactMethodID:        method.ID,
		ContactMethodVersionID: method.CurrentVersionID,
		Email:                  normalizeContactEmail(method.DisplayValue),
		CodeHash:               contactEmailCodeHash(s.emailPepper, userID, method.ID, method.CurrentVersionID, method.DisplayValue, code),
		RequestID:              requestID,
		Now:                    now,
	}
	if s.repo != nil {
		method, completion, appErr = s.repo.ConfirmContactEmailVerificationWithIdempotency(ctx, *entry, input, buildCompletion)
		if appErr != nil {
			s.idempotency.Cancel(ctx, entry)
			return ContactMethod{}, idempotency.Completion{}, false, appErr
		}
		return method, completion, true, nil
	}

	s.mu.Lock()
	current, ok := s.methods[method.ID]
	challenge, challengeOK := s.emailChallenges[method.ID]
	if !ok || current.UserID != userID || !current.Enabled || current.Type != "email" ||
		!challengeOK || challenge.Consumed || challenge.ContactMethodVersionID != current.CurrentVersionID ||
		challenge.Email != normalizeContactEmail(current.DisplayValue) {
		s.mu.Unlock()
		s.idempotency.Cancel(ctx, entry)
		return ContactMethod{}, idempotency.Completion{}, false, invalidContactEmailVerificationCodeError()
	}
	if !now.Before(challenge.ExpiresAt) || challenge.AttemptCount >= ContactEmailVerificationMaxAttempts {
		challenge.Consumed = true
		s.emailChallenges[method.ID] = challenge
		s.mu.Unlock()
		s.idempotency.Cancel(ctx, entry)
		return ContactMethod{}, idempotency.Completion{}, false, invalidContactEmailVerificationCodeError()
	}
	if !hmac.Equal([]byte(challenge.CodeHash), []byte(input.CodeHash)) {
		challenge.AttemptCount++
		if challenge.AttemptCount >= ContactEmailVerificationMaxAttempts {
			challenge.Consumed = true
		}
		s.emailChallenges[method.ID] = challenge
		s.mu.Unlock()
		s.idempotency.Cancel(ctx, entry)
		return ContactMethod{}, idempotency.Completion{}, false, invalidContactEmailVerificationCodeError()
	}
	current.VerifiedAt = &now
	current.UpdatedAt = now
	current.Version++
	challenge.Consumed = true
	s.methods[current.ID] = cloneContactMethod(current)
	s.emailChallenges[current.ID] = challenge
	s.appendMethodAuditEventLocked(current, "contact_method.verified", requestID, []string{"verifiedAt"})
	s.mu.Unlock()
	return s.completeMemoryMethodMutation(ctx, entry, current, buildCompletion)
}

func (s *Service) emailMethod(ctx context.Context, userID, methodID string) (ContactMethod, *domain.AppError) {
	methods, appErr := s.ListMethods(ctx, userID)
	if appErr != nil {
		return ContactMethod{}, appErr
	}
	for _, method := range methods {
		if method.ID != methodID {
			continue
		}
		if !method.Enabled {
			break
		}
		if method.Type != "email" {
			return ContactMethod{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Email contact required", "当前联系方式不是邮箱。", "contactMethodId", "not_email", "请选择邮箱联系方式。")
		}
		email := normalizeContactEmail(method.DisplayValue)
		if appErr := validateContactEmail(email); appErr != nil {
			return ContactMethod{}, appErr
		}
		method.DisplayValue = email
		return method, nil
	}
	return ContactMethod{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Contact method not found", "联系方式不存在。")
}

func (s *Service) beginMethodIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, buildCompletion MethodCompletionBuilder) (*idempotency.Entry, idempotency.Completion, bool, *domain.AppError) {
	if appErr := idempotency.ValidateKey(strings.TrimSpace(key)); appErr != nil {
		return nil, idempotency.Completion{}, false, appErr
	}
	if buildCompletion == nil {
		return nil, idempotency.Completion{}, false, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	entry, appErr := s.idempotency.Begin(ctx, userID, routeKey, strings.TrimSpace(key), requestHash)
	if appErr != nil {
		return nil, idempotency.Completion{}, false, appErr
	}
	if entry.State == "completed" {
		return entry, idempotency.CompletionFromEntry(entry), true, nil
	}
	return entry, idempotency.Completion{}, false, nil
}

func (s *Service) completeMemoryMethodMutation(ctx context.Context, entry *idempotency.Entry, method ContactMethod, buildCompletion MethodCompletionBuilder) (ContactMethod, idempotency.Completion, bool, *domain.AppError) {
	completion, appErr := buildCompletion(method)
	if appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return ContactMethod{}, idempotency.Completion{}, false, appErr
	}
	if appErr := s.idempotency.Complete(ctx, entry, completion.Status, completion.ContentType, completion.Body, completion.ResourceType, completion.ResourceID); appErr != nil {
		s.idempotency.Cancel(ctx, entry)
		return ContactMethod{}, idempotency.Completion{}, false, appErr
	}
	return method, completion, true, nil
}

func (s *Service) CreateSession(ctx context.Context, input CreateContactSessionInput) (ContactSession, *domain.AppError) {
	if input.Duration <= 0 {
		input.Duration = 10 * time.Minute
	}

	now := s.now()
	session := ContactSession{
		ID:           uuid.NewString(),
		BuyerUserID:  input.BuyerUserID,
		SellerUserID: input.SellerUserID,
		OpensAt:      now,
		EndsAt:       now.Add(input.Duration),
	}
	if s.repo != nil {
		return s.repo.CreateContactSession(ctx, input, session, now)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	buyerMethod, buyerVersion, ok := s.VersionForOwnerAndScopeLocked(input.BuyerContactMethodID, input.BuyerUserID, UsageScopeBuyer)
	if !ok || !buyerMethod.Enabled {
		return ContactSession{}, domain.NewError(http.StatusUnprocessableEntity, domain.CodeContactMethodNotOwned, "Contact method not owned", "买家联系方式不可用、不属于当前用户或未允许买家用途。")
	}
	sellerMethod, sellerVersion, ok := s.VersionForOwnerAndScopeLocked(input.SellerContactMethodID, input.SellerUserID, UsageScopeCarpoolOwner)
	if !ok || !sellerMethod.Enabled {
		return ContactSession{}, domain.NewError(http.StatusUnprocessableEntity, domain.CodeContactMethodNotOwned, "Contact method not owned", "车主联系方式不可用、归属不正确或未允许拼车用途。")
	}

	session.BuyerVersionID = buyerVersion.ID
	session.SellerVersionID = sellerVersion.ID
	s.sessions[session.ID] = session
	return session, nil
}

func (s *Service) ReadSession(ctx context.Context, sessionID, viewerUserID, requestID string) (ContactSessionView, *domain.AppError) {
	if s.repo != nil {
		role, appErr := s.repo.ContactSessionViewerRole(ctx, sessionID, viewerUserID)
		if appErr != nil {
			return ContactSessionView{}, appErr
		}
		if appErr := s.checkContactViewAllowed(ctx, viewerUserID, role); appErr != nil {
			return ContactSessionView{}, appErr
		}
		return s.repo.ReadContactSession(ctx, sessionID, viewerUserID, requestID, s.now())
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return ContactSessionView{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Contact session not found", "联系窗口不存在。")
	}
	if session.Revoked || (!session.EndsAt.IsZero() && !s.now().Before(session.EndsAt)) {
		return ContactSessionView{}, domain.NewError(http.StatusConflict, domain.CodeContactWindowExpired, "Contact window expired", "联系窗口已过期。")
	}
	if viewerUserID != session.BuyerUserID && viewerUserID != session.SellerUserID {
		return ContactSessionView{}, domain.NewError(http.StatusForbidden, domain.CodeContactAccessForbidden, "Contact access forbidden", "你不是该联系窗口参与方。")
	}
	role := reputation.RoleSeller
	if viewerUserID == session.BuyerUserID {
		role = reputation.RoleBuyer
	}
	if appErr := s.checkContactViewAllowed(ctx, viewerUserID, role); appErr != nil {
		return ContactSessionView{}, appErr
	}

	buyerVersion := s.versions[session.BuyerVersionID]
	sellerVersion := s.versions[session.SellerVersionID]
	buyerMethod := s.methods[buyerVersion.ContactMethodID]
	sellerMethod := s.methods[sellerVersion.ContactMethodID]

	logEntry := ContactAccessLog{
		ID:               uuid.NewString(),
		ContactSessionID: session.ID,
		ViewerUserID:     viewerUserID,
		AccessedAt:       s.now(),
		RequestID:        requestID,
	}
	s.accessLogs[logEntry.ID] = logEntry
	session.ContactAccessLogIDs = append(session.ContactAccessLogIDs, logEntry.ID)
	s.sessions[session.ID] = session

	var endsAt *time.Time
	if !session.EndsAt.IsZero() {
		value := session.EndsAt
		endsAt = &value
	}
	return ContactSessionView{
		SessionID: session.ID,
		EndsAt:    endsAt,
		Items: []ContactItemView{
			{
				Side:        "buyer",
				SubjectID:   session.BuyerUserID,
				Type:        buyerMethod.Type,
				Label:       buyerMethod.Label,
				Value:       buyerVersion.Value,
				MaskedValue: buyerVersion.MaskedValue,
			},
			{
				Side:        "seller",
				SubjectID:   session.SellerUserID,
				Type:        sellerMethod.Type,
				Label:       sellerMethod.Label,
				Value:       sellerVersion.Value,
				MaskedValue: sellerVersion.MaskedValue,
			},
		},
	}, nil
}

func (s *Service) checkContactViewAllowed(ctx context.Context, userID, role string) *domain.AppError {
	if s.actionChecker == nil {
		return nil
	}
	return s.actionChecker.CheckActionAllowed(ctx, userID, role, reputation.ActionContactView)
}

func (s *Service) AccessLogCount(ctx context.Context, sessionID string) int {
	if s.repo != nil {
		count, appErr := s.repo.ContactAccessLogCount(ctx, sessionID)
		if appErr != nil {
			return 0
		}
		return count
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sessions[sessionID].ContactAccessLogIDs)
}

func (s *Service) appendMethodAuditEventLocked(method ContactMethod, eventType, requestID string, changedFields []string) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		requestID = "unknown"
	}
	s.methodAuditEvents = append(s.methodAuditEvents, ContactMethodAuditEvent{
		MethodID: method.ID, EventType: eventType, ActorUserID: method.UserID,
		AggregateVersion: method.Version, RequestID: requestID,
		ChangedFields: append([]string(nil), changedFields...), CreatedAt: method.UpdatedAt,
	})
}

// MethodAuditEvents 返回内存模式的安全事件副本，仅用于一致性测试。
func (s *Service) MethodAuditEvents() []ContactMethodAuditEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]ContactMethodAuditEvent, len(s.methodAuditEvents))
	for index, event := range s.methodAuditEvents {
		result[index] = event
		result[index].ChangedFields = append([]string(nil), event.ChangedFields...)
	}
	return result
}

func (s *Service) VersionForOwner(methodID, ownerID string) (ContactMethod, ContactMethodVersion, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.VersionForOwnerLocked(methodID, ownerID)
}

// VersionForOwnerAndScope 只返回归属正确、已启用且明确允许指定业务用途的当前版本。
func (s *Service) VersionForOwnerAndScope(methodID, ownerID, requiredScope string) (ContactMethod, ContactMethodVersion, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.VersionForOwnerAndScopeLocked(methodID, ownerID, requiredScope)
}

// WechatVersionForOwnerAndScope only returns the actor's enabled WeChat version for a transaction scope.
func (s *Service) WechatVersionForOwnerAndScope(methodID, ownerID, requiredScope string) (ContactMethod, ContactMethodVersion, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	method, version, ok := s.VersionForOwnerAndScopeLocked(methodID, ownerID, requiredScope)
	if !ok || method.Type != MethodTypeWechat {
		return ContactMethod{}, ContactMethodVersion{}, false
	}
	return method, version, true
}

func (s *Service) VersionForOwnerLocked(methodID, ownerID string) (ContactMethod, ContactMethodVersion, bool) {
	method, ok := s.methods[methodID]
	if !ok || method.UserID != ownerID || method.CurrentVersionID == "" {
		return ContactMethod{}, ContactMethodVersion{}, false
	}
	version, ok := s.versions[method.CurrentVersionID]
	if !ok || version.OwnerUserID != ownerID {
		return ContactMethod{}, ContactMethodVersion{}, false
	}
	return method, version, true
}

func (s *Service) VersionForOwnerAndScopeLocked(methodID, ownerID, requiredScope string) (ContactMethod, ContactMethodVersion, bool) {
	method, version, ok := s.VersionForOwnerLocked(methodID, ownerID)
	if !ok || !method.Enabled || !HasUsageScope(method.UsageScopes, requiredScope) {
		return ContactMethod{}, ContactMethodVersion{}, false
	}
	return method, version, true
}

func (s *Service) AddSession(session ContactSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[session.ID] = session
}

func (s *Service) RevokeSession(sessionID string, now time.Time) {
	if strings.TrimSpace(sessionID) == "" {
		return
	}
	if now.IsZero() {
		now = s.now()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		return
	}
	session.Revoked = true
	if session.EndsAt.IsZero() || now.Before(session.EndsAt) {
		session.EndsAt = now
	}
	s.sessions[session.ID] = session
}

func (s *Service) Version(versionID string) (ContactMethodVersion, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	version, ok := s.versions[versionID]
	return version, ok
}

func NewMethodVersion(input ContactMethodInput, now time.Time) (ContactMethod, ContactMethodVersion) {
	method := ContactMethod{
		ID:           uuid.NewString(),
		UserID:       input.UserID,
		Type:         strings.TrimSpace(input.Type),
		Label:        strings.TrimSpace(input.Label),
		MaskedValue:  MaskValue(input.Value),
		DisplayValue: strings.TrimSpace(input.Value),
		UsageScopes:  append([]string(nil), input.UsageScopes...),
		Enabled:      input.Enabled,
		IsDefault:    input.IsDefault,
		CreatedAt:    now,
		UpdatedAt:    now,
		Version:      1,
	}
	version := ContactMethodVersion{
		ID:              uuid.NewString(),
		ContactMethodID: method.ID,
		OwnerUserID:     input.UserID,
		Value:           input.Value,
		MaskedValue:     method.MaskedValue,
		Fingerprint:     Fingerprint(input.Value),
		CreatedAt:       now,
	}
	method.CurrentVersionID = version.ID
	return method, version
}

func NewUpdatedMethodVersion(input UpdateContactMethodInput, now time.Time) (ContactMethod, ContactMethodVersion) {
	method := ContactMethod{
		UserID:       input.UserID,
		Type:         strings.TrimSpace(input.Type),
		Label:        strings.TrimSpace(input.Label),
		MaskedValue:  MaskValue(input.Value),
		DisplayValue: strings.TrimSpace(input.Value),
		UsageScopes:  append([]string(nil), input.UsageScopes...),
		Enabled:      input.Enabled,
		IsDefault:    input.IsDefault,
		UpdatedAt:    now,
	}
	version := ContactMethodVersion{
		ID:          uuid.NewString(),
		OwnerUserID: input.UserID,
		Value:       input.Value,
		MaskedValue: method.MaskedValue,
		Fingerprint: Fingerprint(input.Value),
		CreatedAt:   now,
	}
	return method, version
}

func validateMethodInput(methodType, value string) *domain.AppError {
	if strings.TrimSpace(value) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeContactMethodRequired, "Contact method required", "必须填写联系方式。", "value", "required", "必须填写联系方式。")
	}
	if strings.TrimSpace(methodType) == "" {
		methodType = "other"
	}
	methodType = strings.TrimSpace(methodType)
	if !allowedMethodType(methodType) {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Contact type invalid", "联系方式类型不支持。", "type", "unsupported", "联系方式类型不支持。")
	}
	if methodType == "email" {
		return validateContactEmail(normalizeContactEmail(value))
	}
	return nil
}

func normalizeMethodTypeAndValue(methodType, value string) (string, string) {
	methodType = strings.TrimSpace(methodType)
	value = strings.TrimSpace(value)
	if methodType == "email" {
		value = normalizeContactEmail(value)
	}
	return methodType, value
}

func normalizeContactEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func validateContactEmail(value string) *domain.AppError {
	if len(value) > 254 || !contactEmailPattern.MatchString(value) {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Email invalid", "邮箱格式不正确。", "value", "invalid", "邮箱格式不正确。")
	}
	return nil
}

func newContactEmailVerificationCode() string {
	var buffer [4]byte
	if _, err := rand.Read(buffer[:]); err != nil {
		panic(err)
	}
	number := int(buffer[0])<<24 | int(buffer[1])<<16 | int(buffer[2])<<8 | int(buffer[3])
	if number < 0 {
		number = -number
	}
	code := strconv.Itoa(number % 1000000)
	return strings.Repeat("0", 6-len(code)) + code
}

func contactEmailCodeHash(pepper []byte, userID, methodID, versionID, email, code string) string {
	subject := strings.Join([]string{strings.TrimSpace(userID), strings.TrimSpace(methodID), strings.TrimSpace(versionID)}, ":")
	return auth.VerificationCodeHash(pepper, "contact_email", subject, normalizeContactEmail(email), strings.TrimSpace(code))
}

func invalidContactEmailVerificationCodeError() *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeVerificationCodeInvalid, "Verification code invalid", "验证码无效或已过期。", "code", "invalid", "验证码无效或已过期。")
}

func contactEmailVerificationInvalidStateError() *domain.AppError {
	return domain.NewError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Contact email changed", "邮箱联系方式已变化，请重新获取验证码。")
}

func Fingerprint(value string) string {
	mac := hmac.New(sha256.New, []byte("c2cmarket-local-contact-fingerprint"))
	mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

func MaskValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 4 {
		return "****"
	}
	return value[:2] + "****" + value[len(value)-2:]
}

func allowedMethodType(value string) bool {
	switch value {
	case "linuxdo", "telegram", "wechat", "email", "other":
		return true
	default:
		return false
	}
}

var (
	contactEmailPattern     = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
	contactEmailCodePattern = regexp.MustCompile(`^[0-9]{6}$`)
)

// DefaultUsageScopes 返回新建联系方式省略使用范围时的最小默认集合。
func DefaultUsageScopes() []string {
	return []string{UsageScopeBuyer, UsageScopeDispute}
}

// HasUsageScope 使用规范化后的精确值判断联系方式能否进入对应业务快照。
func HasUsageScope(scopes []string, requiredScope string) bool {
	requiredScope = strings.TrimSpace(requiredScope)
	if requiredScope == "" {
		return true
	}
	for _, scope := range scopes {
		if strings.TrimSpace(scope) == requiredScope {
			return true
		}
	}
	return false
}

// AllUsageScopes 返回账号级交易联系方式使用范围全集。
func AllUsageScopes() []string {
	return []string{
		UsageScopeCarpoolOwner,
		UsageScopeAPIMerchant,
		UsageScopeBuyer,
		UsageScopeDispute,
	}
}

func normalizedUsageScopesForMethod(methodType string, input []string) ([]string, *domain.AppError) {
	if methodType == MethodTypeWechat {
		return AllUsageScopes(), nil
	}
	if input == nil {
		return DefaultUsageScopes(), nil
	}
	return normalizeUsageScopes(input)
}

func (s *Service) hasOtherEnabledWechatLocked(userID, excludedMethodID string) bool {
	for _, method := range s.methods {
		if method.UserID == userID && method.ID != excludedMethodID && method.Enabled && method.Type == MethodTypeWechat {
			return true
		}
	}
	return false
}

func DuplicateEnabledWechatError() *domain.AppError {
	return domain.NewFieldError(
		http.StatusConflict,
		domain.CodeInvalidStateTransition,
		"WeChat contact already configured",
		"每个账号只能配置一个启用的微信联系方式，请直接更新现有微信。",
		"type",
		"duplicate",
		"微信联系方式已配置。",
	)
}

func WechatRequiredError(field, detail string) *domain.AppError {
	if strings.TrimSpace(detail) == "" {
		detail = "请先在个人中心配置微信联系方式。"
	}
	return domain.NewFieldError(
		http.StatusUnprocessableEntity,
		domain.CodeContactMethodRequired,
		"WeChat contact required",
		detail,
		field,
		"wechat_required",
		"必须使用当前账号已启用的微信联系方式。",
	)
}

func normalizeUsageScopes(input []string) ([]string, *domain.AppError) {
	if len(input) == 0 {
		return nil, domain.NewFieldError(
			http.StatusUnprocessableEntity,
			domain.CodeValidationFailed,
			"Contact usage scopes required",
			"联系方式至少需要一个使用范围。",
			"usageScopes",
			"required",
			"联系方式至少需要一个使用范围。",
		)
	}

	seen := make(map[string]struct{}, len(input))
	for _, scope := range input {
		switch scope {
		case UsageScopeCarpoolOwner, UsageScopeAPIMerchant, UsageScopeBuyer, UsageScopeDispute:
			seen[scope] = struct{}{}
		default:
			return nil, domain.NewFieldError(
				http.StatusUnprocessableEntity,
				domain.CodeValidationFailed,
				"Contact usage scope invalid",
				"联系方式使用范围不支持。",
				"usageScopes",
				"unsupported",
				scope,
			)
		}
	}

	canonical := AllUsageScopes()
	result := make([]string, 0, len(seen))
	for _, scope := range canonical {
		if _, ok := seen[scope]; ok {
			result = append(result, scope)
		}
	}
	return result, nil
}

func validateRequiredWechatUpdate(current ContactMethod, input UpdateContactMethodInput) *domain.AppError {
	if current.Type != "wechat" {
		return nil
	}
	if strings.TrimSpace(input.Type) != "wechat" || !input.Enabled {
		return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "WeChat contact required", "微信是必填联系方式，只能修改微信号，不能停用或改为其他联系方式。")
	}
	return nil
}

func protectedContactDeleteError(methodType string) *domain.AppError {
	if methodType == "wechat" {
		return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "WeChat contact required", "微信是必填联系方式，只能修改微信号，不能解除绑定。")
	}
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Contact method protected", "linux.do 绑定联系方式不能删除。")
}

func equalUsageScopes(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func cloneContactMethod(method ContactMethod) ContactMethod {
	method.UsageScopes = append([]string(nil), method.UsageScopes...)
	return method
}

func methodKey(userID, methodID string) string {
	return userID + "|" + methodID
}
