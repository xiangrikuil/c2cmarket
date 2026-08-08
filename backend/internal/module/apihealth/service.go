package apihealth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/platform/openaiapi"
	"c2c-market/backend/internal/platform/outboundhttp"
)

type Verifier interface {
	Verify(ctx context.Context, baseURL, credential string, allowInsecureHTTP bool) ProbeResult
}

type Service struct {
	repository Repository
	verifier   Verifier
	now        func() time.Time
}

func NewService(repository Repository, verifier Verifier, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repository: repository, verifier: verifier, now: now}
}

func (service *Service) OwnerConnections(ctx context.Context, user auth.User) ([]Connection, *domain.AppError) {
	if service == nil || service.repository == nil {
		return nil, internalError()
	}
	connections, appErr := service.repository.ListOwnerProbeConnections(ctx, user.ID)
	if appErr != nil {
		return nil, appErr
	}
	if appErr := service.attachOwnerHealthSummaries(ctx, user.ID, connections); appErr != nil {
		return nil, appErr
	}
	return connections, nil
}

func (service *Service) OwnerConnection(ctx context.Context, user auth.User, connectionID string) (Connection, bool, *domain.AppError) {
	if service == nil || service.repository == nil {
		return Connection{}, false, internalError()
	}
	connection, found, appErr := service.repository.GetOwnerProbeConnection(ctx, user.ID, strings.TrimSpace(connectionID))
	if appErr != nil || !found {
		return connection, found, appErr
	}
	connections := []Connection{connection}
	if appErr := service.attachOwnerHealthSummaries(ctx, user.ID, connections); appErr != nil {
		return Connection{}, false, appErr
	}
	return connections[0], true, nil
}

func (service *Service) CreateOwnerConnection(ctx context.Context, user auth.User, input ConnectionInput) (Connection, *domain.AppError) {
	if service == nil || service.repository == nil || service.verifier == nil {
		return Connection{}, internalError()
	}
	target, appErr := validateTarget(input)
	if appErr != nil {
		return Connection{}, appErr
	}
	if input.Credential == nil || strings.TrimSpace(*input.Credential) == "" {
		return Connection{}, configValidationError(ErrCredentialRequired)
	}
	credential := strings.TrimSpace(*input.Credential)
	now := service.now().UTC()
	result := service.verifier.Verify(ctx, target.Raw, credential, UsesInsecureHTTP(target.Raw))
	connection, err := NewConnection(user.ID, input, target, result, now)
	if err != nil {
		return Connection{}, configValidationError(err)
	}
	connection, appErr = service.repository.CreateOwnerProbeConnection(ctx, connection, credential)
	if appErr == nil {
		connection.HealthSummary = BuildSummary(&connection, nil, now)
	}
	return connection, appErr
}

func (service *Service) UpdateOwnerConnection(ctx context.Context, user auth.User, connectionID string, input ConnectionInput, expectedVersion int64) (Connection, *domain.AppError) {
	if service == nil || service.repository == nil || service.verifier == nil {
		return Connection{}, internalError()
	}
	existing, credential, found, appErr := service.repository.GetOwnerProbeConnectionCredential(ctx, user.ID, strings.TrimSpace(connectionID))
	if appErr != nil {
		return Connection{}, appErr
	}
	if !found {
		return Connection{}, notFound()
	}
	if existing.Version != expectedVersion {
		return Connection{}, versionConflict()
	}
	target, appErr := validateTarget(input)
	if appErr != nil {
		return Connection{}, appErr
	}
	providedCredential := input.Credential
	if providedCredential != nil {
		credential = strings.TrimSpace(*providedCredential)
		if credential == "" {
			return Connection{}, configValidationError(ErrCredentialInvalid)
		}
	}
	mustVerify := target.Canonical != existing.NormalizedBaseURL || providedCredential != nil || (input.Enabled && !existing.Enabled)
	var result *ProbeResult
	if mustVerify {
		verified := service.verifier.Verify(ctx, target.Raw, credential, UsesInsecureHTTP(target.Raw))
		result = &verified
	}
	updated, err := UpdateConnection(existing, input, target, result, service.now().UTC())
	if err != nil {
		return Connection{}, configValidationError(err)
	}
	updated, appErr = service.repository.UpdateOwnerProbeConnection(ctx, updated, providedCredential, expectedVersion)
	if appErr == nil {
		updated.HealthSummary = BuildSummary(&updated, nil, service.now().UTC())
	}
	return updated, appErr
}

func (service *Service) VerifyOwnerConnection(ctx context.Context, user auth.User, connectionID string, expectedVersion int64) (Connection, *domain.AppError) {
	if service == nil || service.repository == nil || service.verifier == nil {
		return Connection{}, internalError()
	}
	existing, credential, found, appErr := service.repository.GetOwnerProbeConnectionCredential(ctx, user.ID, strings.TrimSpace(connectionID))
	if appErr != nil {
		return Connection{}, appErr
	}
	if !found {
		return Connection{}, notFound()
	}
	if existing.Version != expectedVersion {
		return Connection{}, versionConflict()
	}
	result := service.verifier.Verify(ctx, existing.BaseURL, credential, UsesInsecureHTTP(existing.BaseURL))
	input := ConnectionInput{Name: existing.Name, BaseURL: existing.BaseURL, Enabled: existing.Enabled, AcknowledgeInsecureHTTP: UsesInsecureHTTP(existing.BaseURL)}
	updated, err := UpdateConnection(existing, input, openaiapi.BaseURL{Raw: existing.BaseURL, Canonical: existing.NormalizedBaseURL}, &result, service.now().UTC())
	if err != nil {
		return Connection{}, configValidationError(err)
	}
	updated, appErr = service.repository.UpdateOwnerProbeConnection(ctx, updated, nil, expectedVersion)
	if appErr == nil {
		updated.HealthSummary = BuildSummary(&updated, nil, service.now().UTC())
	}
	return updated, appErr
}

func (service *Service) attachOwnerHealthSummaries(ctx context.Context, ownerUserID string, connections []Connection) *domain.AppError {
	if len(connections) == 0 {
		return nil
	}
	now := service.now().UTC()
	connectionIDs := make([]string, 0, len(connections))
	for _, connection := range connections {
		connectionIDs = append(connectionIDs, connection.ID)
	}
	samples, appErr := service.repository.LoadOwnerProbeConnectionSamples(
		ctx,
		ownerUserID,
		connectionIDs,
		SlotStart(now).Add(-(SummarySlotCount-1)*ProbeSlotDuration),
	)
	if appErr != nil {
		return appErr
	}
	for index := range connections {
		connections[index].HealthSummary = BuildSummary(&connections[index], samples[connections[index].ID], now)
	}
	return nil
}

func (service *Service) DeleteOwnerConnection(ctx context.Context, user auth.User, connectionID string, expectedVersion int64) *domain.AppError {
	if service == nil || service.repository == nil {
		return internalError()
	}
	return service.repository.DeleteOwnerProbeConnection(ctx, user.ID, strings.TrimSpace(connectionID), expectedVersion)
}

func (service *Service) Summaries(ctx context.Context, serviceIDs []string) (map[string]Summary, *domain.AppError) {
	now := service.now().UTC()
	inputs, appErr := service.repository.LoadProbeSummaryInputs(ctx, serviceIDs, SlotStart(now).Add(-(SummarySlotCount-1)*ProbeSlotDuration))
	if appErr != nil {
		return nil, appErr
	}
	result := make(map[string]Summary, len(serviceIDs))
	for _, serviceID := range serviceIDs {
		input := inputs[serviceID]
		result[serviceID] = BuildSummary(input.Connection, input.Samples, now)
	}
	return result, nil
}

func validateTarget(input ConnectionInput) (openaiapi.BaseURL, *domain.AppError) {
	if UsesInsecureHTTP(input.BaseURL) && !input.AcknowledgeInsecureHTTP {
		return openaiapi.BaseURL{}, configValidationError(ErrInsecureHTTPNotAcknowledged)
	}
	target, err := openaiapi.NormalizeBaseURL(input.BaseURL, input.AcknowledgeInsecureHTTP)
	if err != nil {
		return openaiapi.BaseURL{}, targetValidationError(err)
	}
	return target, nil
}

func internalError() *domain.AppError {
	return domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "服务暂时不可用。")
}

func notFound() *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Probe connection not found", "探针连接不存在。")
}

func versionConflict() *domain.AppError {
	return domain.NewError(http.StatusPreconditionFailed, domain.CodeVersionConflict, "Version conflict", "探针连接已更新，请刷新后重试。")
}

func targetValidationError(err error) *domain.AppError {
	code := "invalid"
	message := "探针地址必须是格式正确的公网 HTTP 或 HTTPS 地址。"
	if errors.Is(err, outboundhttp.ErrUnsafeAddress) {
		code = "unsafe_address"
	}
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Probe target invalid", message, "baseUrl", code, message)
}

func configValidationError(err error) *domain.AppError {
	field := "baseUrl"
	message := "探针连接配置不正确。"
	switch {
	case errors.Is(err, ErrInvalidName):
		field, message = "name", "连接名称不能为空且最多 80 个字符。"
	case errors.Is(err, ErrCredentialRequired):
		field, message = "credential", "必须填写探针专用 API Key。"
	case errors.Is(err, ErrCredentialInvalid):
		field, message = "credential", "探针专用 API Key 不能为空。"
	case errors.Is(err, ErrInsecureHTTPNotAcknowledged):
		field, message = "acknowledgeInsecureHttp", "使用 HTTP 探测前必须确认未加密传输风险。"
	}
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Probe connection invalid", message, field, "invalid", message)
}
