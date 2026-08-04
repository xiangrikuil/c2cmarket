package apihealth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/platform/outboundhttp"
)

const (
	challengeTokenBytes = 32
	httpChallengePath   = "/.well-known/c2cmarket-probe-verification"
	httpChallengeLimit  = 4096
)

type TXTResolver interface {
	LookupTXT(ctx context.Context, name string) ([]string, error)
}

type URLValidator interface {
	ValidateURL(ctx context.Context, raw string) (string, error)
}

type Service struct {
	repository    Repository
	urlValidator  URLValidator
	dnsResolver   TXTResolver
	clientFactory HTTPClientFactory
	now           func() time.Time
	random        io.Reader
	challengeTTL  time.Duration
}

func NewService(repository Repository, validator URLValidator, resolver TXTResolver, clientFactory HTTPClientFactory, now func() time.Time, challengeTTL time.Duration) *Service {
	if now == nil {
		now = time.Now
	}
	if challengeTTL <= 0 {
		challengeTTL = 15 * time.Minute
	}
	return &Service{
		repository: repository, urlValidator: validator, dnsResolver: resolver,
		clientFactory: clientFactory, now: now, random: rand.Reader, challengeTTL: challengeTTL,
	}
}

func (s *Service) OwnerConfig(ctx context.Context, user auth.User, serviceID string) (Config, bool, *domain.AppError) {
	if s == nil || s.repository == nil {
		return Config{}, false, internalError()
	}
	return s.repository.GetOwnerProbeConfig(ctx, user.ID, strings.TrimSpace(serviceID))
}

func (s *Service) PutOwnerConfig(ctx context.Context, user auth.User, serviceID string, input ConfigInput, expectedVersion int64) (Config, *domain.AppError) {
	if s == nil || s.repository == nil || s.urlValidator == nil {
		return Config{}, internalError()
	}
	serviceID = strings.TrimSpace(serviceID)
	if _, err := s.urlValidator.ValidateURL(ctx, input.BaseURL); err != nil {
		return Config{}, targetValidationError(err)
	}
	existing, found, appErr := s.repository.GetOwnerProbeConfig(ctx, user.ID, serviceID)
	if appErr != nil {
		return Config{}, appErr
	}
	if (!found && expectedVersion != 0) || (found && expectedVersion != existing.Version) {
		return Config{}, versionConflict()
	}
	var current *Config
	if found {
		current = &existing
	}
	mutation, err := BuildConfigMutation(current, serviceID, user.ID, input, s.now().UTC())
	if err != nil {
		return Config{}, configValidationError(err)
	}
	return s.repository.UpsertOwnerProbeConfig(ctx, mutation, input.Credential, expectedVersion)
}

func (s *Service) DeleteOwnerConfig(ctx context.Context, user auth.User, serviceID string, expectedVersion int64) *domain.AppError {
	if s == nil || s.repository == nil {
		return internalError()
	}
	return s.repository.DeleteOwnerProbeConfig(ctx, user.ID, strings.TrimSpace(serviceID), expectedVersion, s.now().UTC())
}

func (s *Service) CreateChallenge(ctx context.Context, user auth.User, serviceID, method string, expectedVersion int64) (Challenge, *domain.AppError) {
	if s == nil || s.repository == nil || s.random == nil {
		return Challenge{}, internalError()
	}
	config, found, appErr := s.repository.GetOwnerProbeConfig(ctx, user.ID, strings.TrimSpace(serviceID))
	if appErr != nil {
		return Challenge{}, appErr
	}
	if !found {
		return Challenge{}, notFound()
	}
	if config.Version != expectedVersion {
		return Challenge{}, versionConflict()
	}
	method = strings.TrimSpace(method)
	if method != AuthorizationMethodDNSTXT && method != AuthorizationMethodHTTPChallenge {
		return Challenge{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Verification method invalid", "验证方式不正确。", "method", "invalid", "验证方式不正确。")
	}
	parsedOrigin, err := url.Parse(config.NormalizedOrigin)
	if err != nil {
		return Challenge{}, internalError()
	}
	if method == AuthorizationMethodDNSTXT && parsedOrigin.Port() != "443" {
		return Challenge{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "DNS verification unavailable", "非 443 端口不能使用 DNS 验证。", "method", "port_not_supported", "请改用 HTTP 验证或管理员审核。")
	}
	raw := make([]byte, challengeTokenBytes)
	if _, err := io.ReadFull(s.random, raw); err != nil {
		return Challenge{}, internalError()
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	expiresAt := s.now().UTC().Add(s.challengeTTL)
	updated, appErr := s.repository.CreateProbeChallenge(ctx, user.ID, config.APIServiceID, method, hash[:], expiresAt, expectedVersion, s.now().UTC())
	if appErr != nil {
		return Challenge{}, appErr
	}
	challenge := Challenge{Token: token, Method: method, ExpiresAt: expiresAt, ConfigVersion: updated.Version}
	if method == AuthorizationMethodDNSTXT {
		challenge.DNSRecordName = "_c2cmarket-probe." + parsedOrigin.Hostname()
	} else {
		challenge.HTTPURL = strings.TrimRight(config.NormalizedOrigin, "/") + httpChallengePath
	}
	return challenge, nil
}

func (s *Service) VerifyChallenge(ctx context.Context, user auth.User, serviceID string, expectedVersion int64) (Config, *domain.AppError) {
	if s == nil || s.repository == nil {
		return Config{}, internalError()
	}
	challenge, appErr := s.repository.GetProbeChallenge(ctx, user.ID, strings.TrimSpace(serviceID))
	if appErr != nil {
		return Config{}, appErr
	}
	if challenge.Config.Version != expectedVersion {
		return Config{}, versionConflict()
	}
	now := s.now().UTC()
	succeeded := false
	reason := "challenge_mismatch"
	if !now.Before(challenge.ExpiresAt) {
		reason = "challenge_expired"
	} else {
		switch challenge.Method {
		case AuthorizationMethodDNSTXT:
			succeeded, reason = s.verifyDNS(ctx, challenge)
		case AuthorizationMethodHTTPChallenge:
			succeeded, reason = s.verifyHTTP(ctx, challenge)
		default:
			return Config{}, internalError()
		}
	}
	return s.repository.CompleteProbeVerification(ctx, user.ID, challenge.Config.APIServiceID, challenge.Method, expectedVersion, succeeded, reason, now)
}

func (s *Service) AdminConfigs(ctx context.Context, user auth.User, status string, page domain.PageRequest) (domain.Page[Config], *domain.AppError) {
	if !user.IsAdmin {
		return domain.Page[Config]{}, forbidden()
	}
	return s.repository.ListAdminProbeConfigs(ctx, strings.TrimSpace(status), page)
}

func (s *Service) AdminDecision(ctx context.Context, user auth.User, configID string, expectedVersion int64, approve bool, reason string) (Config, *domain.AppError) {
	if !user.IsAdmin {
		return Config{}, forbidden()
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return Config{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Review reason required", "管理员审核必须填写理由。", "reason", "required", "请填写审核理由。")
	}
	return s.repository.AdminDecideProbeConfig(ctx, user.ID, strings.TrimSpace(configID), expectedVersion, approve, reason, s.now().UTC())
}

func (s *Service) Summaries(ctx context.Context, serviceIDs []string) (map[string]Summary, *domain.AppError) {
	now := s.now().UTC()
	inputs, appErr := s.repository.LoadProbeSummaryInputs(ctx, serviceIDs, SlotStart(now).Add(-(SummarySlotCount-1)*ProbeSlotDuration))
	if appErr != nil {
		return nil, appErr
	}
	result := make(map[string]Summary, len(serviceIDs))
	for _, serviceID := range serviceIDs {
		input := inputs[serviceID]
		result[serviceID] = BuildSummary(input.Config, input.Samples, now)
	}
	return result, nil
}

func (s *Service) verifyDNS(ctx context.Context, challenge StoredChallenge) (bool, string) {
	if s.dnsResolver == nil {
		return false, "dns_resolution_failed"
	}
	origin, err := url.Parse(challenge.Config.NormalizedOrigin)
	if err != nil {
		return false, "invalid_origin"
	}
	values, err := s.dnsResolver.LookupTXT(ctx, "_c2cmarket-probe."+origin.Hostname())
	if err != nil {
		return false, "dns_resolution_failed"
	}
	for _, value := range values {
		if tokenHashMatches([]byte(value), challenge.TokenHash) {
			return true, ""
		}
	}
	return false, "challenge_mismatch"
}

func (s *Service) verifyHTTP(ctx context.Context, challenge StoredChallenge) (bool, string) {
	if s.clientFactory == nil {
		return false, "http_request_failed"
	}
	client, err := s.clientFactory.ClientFor(challenge.Config)
	if err != nil {
		return false, "target_blocked"
	}
	target := strings.TrimRight(challenge.Config.NormalizedOrigin, "/") + httpChallengePath
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return false, "invalid_origin"
	}
	response, err := client.Do(request)
	if err != nil {
		return false, "http_request_failed"
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return false, "http_status"
	}
	body, err := outboundhttp.ReadBody(response.Body, httpChallengeLimit)
	if err != nil {
		return false, "http_response_invalid"
	}
	if tokenHashMatches(body, challenge.TokenHash) {
		return true, ""
	}
	return false, "challenge_mismatch"
}

func tokenHashMatches(value, expected []byte) bool {
	hash := sha256.Sum256(value)
	return len(expected) == sha256.Size && subtle.ConstantTimeCompare(hash[:], expected) == 1
}

func internalError() *domain.AppError {
	return domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "服务暂时不可用。")
}

func notFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Probe config not found", "探针配置不存在。")
}

func forbidden() *domain.AppError {
	return domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Forbidden", "没有权限执行此操作。")
}

func versionConflict() *domain.AppError {
	return domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "探针配置已更新，请刷新后重试。")
}

func targetValidationError(err error) *domain.AppError {
	code := "invalid"
	message := "探针地址必须是可访问的公网 HTTPS 地址。"
	if errors.Is(err, outboundhttp.ErrUnsafeAddress) {
		code = "unsafe_address"
	}
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Probe target invalid", message, "baseUrl", code, message)
}

func configValidationError(err error) *domain.AppError {
	field := "baseUrl"
	message := "探针配置不正确。"
	if errors.Is(err, ErrInvalidModel) {
		field, message = "model", "必须填写探测模型。"
	} else if errors.Is(err, ErrCredentialRequired) {
		field, message = "credential", "启用探针前必须配置专用凭据。"
	} else if errors.Is(err, ErrCredentialInvalid) {
		field, message = "credential", "探针凭据不能为空。"
	}
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Probe config invalid", message, field, "invalid", message)
}
