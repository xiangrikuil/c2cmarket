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
	Verify(ctx context.Context, baseURL, credential, requestedModel string, allowInsecureHTTP bool) VerificationResult
}

type PreflightResult struct {
	Verification               VerificationResult
	Price                      PriceSnapshot
	DailyBaseCostUpperBoundUSD string
	PriceUnavailable           bool
	PreflightToken             string
}

type Service struct {
	repository            Repository
	calibrationRepository CalibrationRepository
	verifier              Verifier
	now                   func() time.Time
	preflights            preflightStore
	runnerStatus          RunnerStatusProvider
}

func (service *Service) SetRunnerStatusProvider(provider RunnerStatusProvider) {
	if service != nil {
		service.runnerStatus = provider
	}
}

func NewService(repository Repository, verifier Verifier, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	calibrationRepository, _ := repository.(CalibrationRepository)
	return &Service{repository: repository, calibrationRepository: calibrationRepository, verifier: verifier, now: now}
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

func (service *Service) PreflightOwnerConnection(ctx context.Context, user auth.User, input ConnectionInput) (PreflightResult, *domain.AppError) {
	if service == nil || service.repository == nil || service.verifier == nil {
		return PreflightResult{}, internalError()
	}
	target, appErr := validateTarget(input)
	if appErr != nil {
		return PreflightResult{}, appErr
	}
	if input.Credential == nil || strings.TrimSpace(*input.Credential) == "" {
		return PreflightResult{}, configValidationError(ErrCredentialRequired)
	}
	return service.preflight(ctx, user.ID, "", 0, target, strings.TrimSpace(*input.Credential), input.ProbeModel)
}

func (service *Service) PreflightExistingOwnerConnection(ctx context.Context, user auth.User, connectionID string, input ConnectionInput, expectedVersion int64) (PreflightResult, *domain.AppError) {
	if service == nil || service.repository == nil || service.verifier == nil {
		return PreflightResult{}, internalError()
	}
	existing, credential, found, appErr := service.repository.GetOwnerProbeConnectionCredential(ctx, user.ID, strings.TrimSpace(connectionID))
	if appErr != nil {
		return PreflightResult{}, appErr
	}
	if !found {
		return PreflightResult{}, notFound()
	}
	if existing.Version != expectedVersion {
		return PreflightResult{}, versionConflict()
	}
	if input.Credential != nil {
		credential = strings.TrimSpace(*input.Credential)
	}
	if credential == "" {
		return PreflightResult{}, configValidationError(ErrCredentialRequired)
	}
	if strings.TrimSpace(input.BaseURL) == "" {
		input.BaseURL = existing.BaseURL
	}
	if strings.TrimSpace(input.ProbeModel) == "" {
		input.ProbeModel = existing.ProbeModel
	}
	target, appErr := validateTarget(input)
	if appErr != nil {
		return PreflightResult{}, appErr
	}
	return service.preflight(ctx, user.ID, existing.ID, expectedVersion, target, credential, input.ProbeModel)
}

func (service *Service) runPreflight(ctx context.Context, baseURL, credential, requestedModel string) PreflightResult {
	verification := service.verifier.Verify(ctx, baseURL, credential, requestedModel, UsesInsecureHTTP(baseURL))
	result := PreflightResult{Verification: verification, PriceUnavailable: true}
	if verification.ProbeModel == "" {
		return result
	}
	price, found, appErr := service.repository.LookupProbeModelPrice(ctx, verification.ProbeModel)
	if appErr != nil || !found {
		return result
	}
	result.Price = price
	result.DailyBaseCostUpperBoundUSD = DailyBaseCostUpperBoundUSD(price)
	result.PriceUnavailable = result.DailyBaseCostUpperBoundUSD == ""
	return result
}

func (service *Service) preflight(
	ctx context.Context,
	ownerUserID string,
	connectionID string,
	expectedVersion int64,
	target openaiapi.BaseURL,
	credential string,
	requestedModel string,
) (PreflightResult, *domain.AppError) {
	result := service.runPreflight(ctx, target.Raw, credential, requestedModel)
	if result.Verification.ErrorCode != "" || result.Verification.ProbeModel == "" || result.Verification.ProbeProtocol == "" {
		return result, nil
	}
	now := service.now().UTC()
	token, err := service.preflights.issue(preflightGrant{
		OwnerUserID: ownerUserID, ConnectionID: connectionID, ExpectedVersion: expectedVersion,
		CanonicalBaseURL: target.Canonical, CredentialFingerprint: credentialFingerprint(credential),
		ProbeModel: result.Verification.ProbeModel, ProbeProtocol: result.Verification.ProbeProtocol,
		Result: result,
	}, now)
	if err != nil {
		return PreflightResult{}, internalError()
	}
	result.PreflightToken = token
	return result, nil
}

func (service *Service) consumePreflight(
	ownerUserID string,
	connectionID string,
	expectedVersion int64,
	target openaiapi.BaseURL,
	credential string,
	requestedModel string,
	token string,
) (PreflightResult, *domain.AppError) {
	if strings.TrimSpace(token) == "" {
		return PreflightResult{}, configValidationError(ErrPreflightRequired)
	}
	grant, found := service.preflights.consume(token, service.now().UTC())
	normalizedRequestedModel := strings.TrimSpace(requestedModel)
	if normalizedRequestedModel == "" {
		normalizedRequestedModel = grant.ProbeModel
	}
	if !found || grant.OwnerUserID != ownerUserID || grant.ConnectionID != connectionID ||
		grant.ExpectedVersion != expectedVersion || grant.CanonicalBaseURL != target.Canonical ||
		grant.CredentialFingerprint != credentialFingerprint(credential) || grant.ProbeModel != normalizedRequestedModel ||
		grant.ProbeProtocol == "" || grant.Result.Verification.ProbeProtocol != grant.ProbeProtocol {
		return PreflightResult{}, configValidationError(ErrPreflightInvalid)
	}
	return grant.Result, nil
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
	preflight, appErr := service.consumePreflight(user.ID, "", 0, target, credential, input.ProbeModel, input.PreflightToken)
	if appErr != nil {
		return Connection{}, appErr
	}
	connection, err := NewConnection(user.ID, input, target, preflight.Verification, preflight.Price, now)
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
	requestedModel := strings.TrimSpace(input.ProbeModel)
	if requestedModel == "" {
		requestedModel = existing.ProbeModel
	}
	mustVerify := target.Canonical != existing.NormalizedBaseURL || providedCredential != nil || requestedModel != existing.ProbeModel || (input.Enabled && !existing.Enabled)
	var result *VerificationResult
	var price *PriceSnapshot
	if mustVerify {
		checked, consumeErr := service.consumePreflight(user.ID, existing.ID, expectedVersion, target, credential, requestedModel, input.PreflightToken)
		if consumeErr != nil {
			return Connection{}, consumeErr
		}
		result = &checked.Verification
		price = &checked.Price
	}
	updated, err := UpdateConnection(existing, input, target, result, price, service.now().UTC())
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
	checked := service.runPreflight(ctx, existing.BaseURL, credential, existing.ProbeModel)
	input := ConnectionInput{Name: existing.Name, BaseURL: existing.BaseURL, ProbeModel: existing.ProbeModel, Enabled: existing.Enabled, AcknowledgeInsecureHTTP: UsesInsecureHTTP(existing.BaseURL)}
	updated, err := UpdateConnection(existing, input, openaiapi.BaseURL{Raw: existing.BaseURL, Canonical: existing.NormalizedBaseURL}, &checked.Verification, &checked.Price, service.now().UTC())
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
	samples, appErr := service.repository.LoadOwnerProbeConnectionSamples(ctx, ownerUserID, connectionIDs, SummaryStart(now))
	if appErr != nil {
		return appErr
	}
	for index := range connections {
		connections[index].HealthSummary = BuildSummary(&connections[index], samples[connections[index].ID], now)
		service.applyRunnerStatus(&connections[index].HealthSummary, &connections[index], now)
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
	inputs, appErr := service.repository.LoadProbeSummaryInputs(ctx, serviceIDs, SummaryStart(now))
	if appErr != nil {
		return nil, appErr
	}
	result := make(map[string]Summary, len(serviceIDs))
	for _, serviceID := range serviceIDs {
		input := inputs[serviceID]
		summary := BuildSummary(input.Connection, input.Samples, now)
		service.applyRunnerStatus(&summary, input.Connection, now)
		result[serviceID] = summary
	}
	return result, nil
}

func (service *Service) applyRunnerStatus(summary *Summary, connection *Connection, now time.Time) {
	if service == nil || service.runnerStatus == nil || summary == nil || connection == nil ||
		!connection.Enabled || connection.VerificationStatus != VerificationVerified {
		return
	}
	status := service.runnerStatus.ProbeRunnerStatus()
	if !status.Enabled {
		summary.State = HealthStateNoSample
		summary.AvailabilityReason = AvailabilityRunnerDisabled
		return
	}
	staleAfter := 3 * status.ScanInterval
	if staleAfter < SummaryStaleAfter {
		staleAfter = SummaryStaleAfter
	}
	scanStale := status.LastSuccessfulScanAt.IsZero() || now.Sub(status.LastSuccessfulScanAt) > staleAfter
	sampleFresh := summary.LastSampledAt != nil && now.Sub(*summary.LastSampledAt) <= SummaryStaleAfter
	if scanStale && !sampleFresh {
		summary.State = HealthStateNoSample
		summary.AvailabilityReason = AvailabilityStale
	}
}

func (service *Service) ProbeCalibration(ctx context.Context, model, protocol, environment string) (Calibration, *domain.AppError) {
	if service == nil || service.calibrationRepository == nil {
		return Calibration{}, internalError()
	}
	model, protocol, environment, appErr := validateLatencyDimension(model, protocol, environment)
	if appErr != nil {
		return Calibration{}, appErr
	}
	return service.calibrationRepository.LoadProbeCalibration(ctx, model, protocol, environment, service.now().UTC())
}

func (service *Service) PreviewLatencyRule(ctx context.Context, model, protocol, environment string, slowTTFTMS, hardTimeoutMS int) (LatencyRulePreview, *domain.AppError) {
	if appErr := validateLatencyThresholds(slowTTFTMS, hardTimeoutMS); appErr != nil {
		return LatencyRulePreview{}, appErr
	}
	calibration, appErr := service.ProbeCalibration(ctx, model, protocol, environment)
	if appErr != nil {
		return LatencyRulePreview{}, appErr
	}
	return service.calibrationRepository.PreviewProbeLatencyRule(ctx, calibration, slowTTFTMS, hardTimeoutMS)
}

func (service *Service) PublishLatencyRule(ctx context.Context, admin auth.User, model, protocol, environment string, slowTTFTMS, hardTimeoutMS int) (LatencyRule, *domain.AppError) {
	if appErr := validateLatencyThresholds(slowTTFTMS, hardTimeoutMS); appErr != nil {
		return LatencyRule{}, appErr
	}
	calibration, appErr := service.ProbeCalibration(ctx, model, protocol, environment)
	if appErr != nil {
		return LatencyRule{}, appErr
	}
	if !calibration.Ready {
		return LatencyRule{}, latencyValidationError("model", "calibration_incomplete", "校准至少需要 7 个完整自然日和 5 个独立连接。")
	}
	if _, appErr = service.calibrationRepository.PreviewProbeLatencyRule(ctx, calibration, slowTTFTMS, hardTimeoutMS); appErr != nil {
		return LatencyRule{}, appErr
	}
	now := service.now().UTC()
	return service.calibrationRepository.PublishProbeLatencyRule(ctx, LatencyRule{
		Model: model, Protocol: protocol, Environment: environment,
		SlowTTFTMS: slowTTFTMS, HardTimeoutMS: hardTimeoutMS,
		ObservationStartedAt: calibration.ObservationStartedAt, ObservationEndedAt: calibration.ObservationEndedAt,
		CompleteCalendarDays: calibration.CompleteCalendarDays, ConnectionCount: calibration.ConnectionCount,
		SampleCount: calibration.SampleCount, P50TTFTMS: calibration.P50TTFTMS, P90TTFTMS: calibration.P90TTFTMS,
		P95TTFTMS: calibration.P95TTFTMS, P99TTFTMS: calibration.P99TTFTMS,
		PublishedByAdminID: admin.ID, PublishedAt: now,
	})
}

func (service *Service) LatencyRules(ctx context.Context) ([]LatencyRule, *domain.AppError) {
	if service == nil || service.calibrationRepository == nil {
		return nil, internalError()
	}
	return service.calibrationRepository.ListProbeLatencyRules(ctx)
}

func validateLatencyDimension(model, protocol, environment string) (string, string, string, *domain.AppError) {
	model = strings.TrimSpace(model)
	protocol = strings.TrimSpace(protocol)
	environment = strings.TrimSpace(environment)
	if model == "" {
		return "", "", "", latencyValidationError("model", "required", "必须提供探针模型。")
	}
	if protocol != ProtocolResponsesV1 && protocol != ProtocolChatCompletionsV1 {
		return "", "", "", latencyValidationError("protocol", "invalid", "探针协议不受支持。")
	}
	if environment != ProbeEnvironmentUSWestV1 {
		return "", "", "", latencyValidationError("environment", "invalid", "探针测量环境不受支持。")
	}
	return model, protocol, environment, nil
}

func validateLatencyThresholds(slowTTFTMS, hardTimeoutMS int) *domain.AppError {
	if slowTTFTMS <= 0 || hardTimeoutMS <= slowTTFTMS || hardTimeoutMS > 30000 {
		return latencyValidationError("slowTtftMs", "invalid_range", "阈值必须满足 0 < X < Y <= 30000 毫秒。")
	}
	return nil
}

func latencyValidationError(field, code, detail string) *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Probe latency rule invalid", detail, field, code, detail)
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
	case errors.Is(err, ErrProbeModelRequired):
		field, message = "probeModel", "请选择用于真实请求的探针模型。"
	case errors.Is(err, ErrProbeModelUnavailable):
		field, message = "probeModel", "当前 Key 的 /models 中不存在该模型。"
	case errors.Is(err, ErrPreflightRequired):
		field, message = "preflightToken", "请先完成真实模型验证再保存。"
	case errors.Is(err, ErrPreflightInvalid):
		field, message = "preflightToken", "验证结果已过期、已使用或与当前配置不一致，请重新验证。"
	case errors.Is(err, ErrInsecureHTTPNotAcknowledged):
		field, message = "acknowledgeInsecureHttp", "使用 HTTP 探测前必须确认未加密传输风险。"
	}
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Probe connection invalid", message, field, "invalid", message)
}
