package contact

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"

	"c2c-market/backend/internal/domain"

	"github.com/google/uuid"
)

type Service struct {
	mu               sync.Mutex
	now              func() time.Time
	repo             Repository
	methods          map[string]ContactMethod
	versions         map[string]ContactMethodVersion
	sessions         map[string]ContactSession
	accessLogs       map[string]ContactAccessLog
	methodsByUserKey map[string]string
}

func NewService(repo Repository, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{
		now:              now,
		repo:             repo,
		methods:          make(map[string]ContactMethod),
		versions:         make(map[string]ContactMethodVersion),
		sessions:         make(map[string]ContactSession),
		accessLogs:       make(map[string]ContactAccessLog),
		methodsByUserKey: make(map[string]string),
	}
}

func (s *Service) CreateMethod(ctx context.Context, input ContactMethodInput) (ContactMethod, *domain.AppError) {
	if appErr := validateMethodInput(input.Type, input.Value); appErr != nil {
		return ContactMethod{}, appErr
	}

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

	s.methods[method.ID] = method
	s.versions[version.ID] = version
	s.methodsByUserKey[methodKey(method.UserID, method.ID)] = method.ID
	return method, nil
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
			methods = append(methods, method)
		}
	}
	return methods, nil
}

func (s *Service) UpdateMethod(ctx context.Context, input UpdateContactMethodInput) (ContactMethod, *domain.AppError) {
	if appErr := validateMethodInput(input.Type, input.Value); appErr != nil {
		return ContactMethod{}, appErr
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
	if input.IsDefault {
		for id, item := range s.methods {
			if item.UserID == input.UserID {
				item.IsDefault = false
				s.methods[id] = item
			}
		}
	}
	method.ID = current.ID
	method.UserID = current.UserID
	method.CreatedAt = current.CreatedAt
	method.Version = current.Version + 1
	version.ContactMethodID = method.ID
	method.CurrentVersionID = version.ID
	s.methods[method.ID] = method
	s.versions[version.ID] = version
	return method, nil
}

func (s *Service) DeleteMethod(ctx context.Context, userID, methodID string) (ContactMethod, *domain.AppError) {
	if s.repo != nil {
		return s.repo.DeleteContactMethod(ctx, userID, methodID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	method, ok := s.methods[methodID]
	if !ok || method.UserID != userID {
		return ContactMethod{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Contact method not found", "联系方式不存在。")
	}
	if method.Type == "linuxdo" {
		return ContactMethod{}, domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Contact method protected", "linux.do 绑定联系方式不能删除。")
	}
	method.Enabled = false
	method.UpdatedAt = s.now()
	method.Version++
	s.methods[method.ID] = method
	return method, nil
}

func (s *Service) SetDefaultMethod(ctx context.Context, userID, methodID string) (ContactMethod, *domain.AppError) {
	if s.repo != nil {
		return s.repo.SetDefaultContactMethod(ctx, userID, methodID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	method, ok := s.methods[methodID]
	if !ok || method.UserID != userID || !method.Enabled {
		return ContactMethod{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Contact method not found", "联系方式不存在。")
	}
	now := s.now()
	for id, item := range s.methods {
		if item.UserID == userID {
			item.IsDefault = item.ID == methodID
			if item.ID == methodID {
				item.UpdatedAt = now
				item.Version++
			}
			s.methods[id] = item
		}
	}
	return s.methods[methodID], nil
}

func (s *Service) VerifyMethod(ctx context.Context, userID, methodID string) (ContactMethod, *domain.AppError) {
	now := s.now()
	if s.repo != nil {
		return s.repo.VerifyContactMethod(ctx, userID, methodID, now)
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	method, ok := s.methods[methodID]
	if !ok || method.UserID != userID || !method.Enabled {
		return ContactMethod{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Contact method not found", "联系方式不存在。")
	}
	method.VerifiedAt = &now
	method.UpdatedAt = now
	method.Version++
	s.methods[method.ID] = method
	return method, nil
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

	buyerMethod, buyerVersion, ok := s.VersionForOwnerLocked(input.BuyerContactMethodID, input.BuyerUserID)
	if !ok || !buyerMethod.Enabled {
		return ContactSession{}, domain.NewError(http.StatusUnprocessableEntity, domain.CodeContactMethodNotOwned, "Contact method not owned", "买家联系方式不可用或不属于当前用户。")
	}
	sellerMethod, sellerVersion, ok := s.VersionForOwnerLocked(input.SellerContactMethodID, input.SellerUserID)
	if !ok || !sellerMethod.Enabled {
		return ContactSession{}, domain.NewError(http.StatusUnprocessableEntity, domain.CodeContactMethodNotOwned, "Contact method not owned", "商户联系方式不可用或归属不正确。")
	}

	session.BuyerVersionID = buyerVersion.ID
	session.SellerVersionID = sellerVersion.ID
	s.sessions[session.ID] = session
	return session, nil
}

func (s *Service) ReadSession(ctx context.Context, sessionID, viewerUserID, requestID string) (ContactSessionView, *domain.AppError) {
	if s.repo != nil {
		return s.repo.ReadContactSession(ctx, sessionID, viewerUserID, requestID, s.now())
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return ContactSessionView{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Contact session not found", "联系窗口不存在。")
	}
	if session.Revoked || !s.now().Before(session.EndsAt) {
		return ContactSessionView{}, domain.NewError(http.StatusConflict, domain.CodeContactWindowExpired, "Contact window expired", "联系窗口已过期。")
	}
	if viewerUserID != session.BuyerUserID && viewerUserID != session.SellerUserID {
		return ContactSessionView{}, domain.NewError(http.StatusForbidden, domain.CodeContactAccessForbidden, "Contact access forbidden", "你不是该联系窗口参与方。")
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

	return ContactSessionView{
		SessionID: session.ID,
		EndsAt:    session.EndsAt,
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

func (s *Service) VersionForOwner(methodID, ownerID string) (ContactMethod, ContactMethodVersion, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.VersionForOwnerLocked(methodID, ownerID)
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
	if now.Before(session.EndsAt) {
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
	return nil
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

func methodKey(userID, methodID string) string {
	return userID + "|" + methodID
}
