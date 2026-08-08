package apihealth

import (
	"context"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/platform/openaiapi"
)

func TestCreateConnectionVerifiesAndEnablesSuccessfulTarget(t *testing.T) {
	now := time.Date(2026, 8, 8, 10, 0, 0, 0, time.UTC)
	repository := &connectionRepository{}
	verifier := &connectionVerifier{result: successfulVerification()}
	service := NewService(repository, verifier, func() time.Time { return now })
	credential := " probe-key "
	input := ConnectionInput{
		Name: " 主 Sub2API ", BaseURL: "https://API.example.com/v1/", Credential: &credential, Enabled: true,
		ProbeModel: DefaultGPTProbeModel,
	}
	preflight, appErr := service.PreflightOwnerConnection(context.Background(), auth.User{ID: "owner-1"}, input)
	if appErr != nil || preflight.PreflightToken == "" {
		t.Fatalf("PreflightOwnerConnection() result=%+v error=%v", preflight, appErr)
	}
	input.PreflightToken = preflight.PreflightToken
	connection, appErr := service.CreateOwnerConnection(context.Background(), auth.User{ID: "owner-1"}, input)
	if appErr != nil {
		t.Fatalf("CreateOwnerConnection() error: %v", appErr)
	}
	if connection.Name != "主 Sub2API" || connection.BaseURL != "https://API.example.com/v1/" || connection.NormalizedBaseURL != "https://api.example.com/v1" {
		t.Fatalf("unexpected connection: %+v", connection)
	}
	if !connection.Enabled || connection.VerificationStatus != VerificationVerified || connection.VerifiedAt == nil {
		t.Fatalf("connection was not verified and enabled: %+v", connection)
	}
	if verifier.calls != 1 || verifier.baseURL != connection.BaseURL || verifier.credential != "probe-key" || repository.credential != "probe-key" {
		t.Fatalf("verification or persistence did not receive normalized secret input")
	}
}

func TestCreateConnectionRequiresSuccessfulPreflight(t *testing.T) {
	repository := &connectionRepository{}
	service := NewService(repository, &connectionVerifier{result: VerificationResult{ErrorCode: ErrorAuthorizationInvalid}}, time.Now)
	credential := "bad-key"
	input := ConnectionInput{
		Name: "低额度探针", BaseURL: "https://api.example.com/v1", Credential: &credential,
		ProbeModel: DefaultGPTProbeModel, Enabled: true,
	}
	preflight, appErr := service.PreflightOwnerConnection(context.Background(), auth.User{ID: "owner-1"}, input)
	if appErr != nil || preflight.Verification.ErrorCode != ErrorAuthorizationInvalid || preflight.PreflightToken != "" {
		t.Fatalf("unexpected failed preflight: result=%+v error=%v", preflight, appErr)
	}
	_, appErr = service.CreateOwnerConnection(context.Background(), auth.User{ID: "owner-1"}, input)
	if appErr == nil || len(appErr.FieldErrors) == 0 || appErr.FieldErrors[0].Field != "preflightToken" || repository.found {
		t.Fatalf("unexpected create result: error=%+v persisted=%t", appErr, repository.found)
	}
}

func TestUpdateConnectionOnlyReverifiesMaterialChangesOrEnable(t *testing.T) {
	now := time.Date(2026, 8, 8, 10, 0, 0, 0, time.UTC)
	existing := verifiedConnection(now)
	repository := &connectionRepository{connection: existing, found: true, credential: "secret"}
	verifier := &connectionVerifier{result: successfulVerification()}
	service := NewService(repository, verifier, func() time.Time { return now.Add(time.Minute) })

	updated, appErr := service.UpdateOwnerConnection(context.Background(), auth.User{ID: existing.OwnerUserID}, existing.ID, ConnectionInput{
		Name: "新名称", BaseURL: existing.BaseURL, Enabled: true,
	}, existing.Version)
	if appErr != nil {
		t.Fatalf("name update error: %v", appErr)
	}
	if verifier.calls != 0 || updated.MeasurementVersion != existing.MeasurementVersion {
		t.Fatalf("name-only update unexpectedly reverified: calls=%d connection=%+v", verifier.calls, updated)
	}

	repository.connection = updated
	repository.credential = "secret"
	repository.found = true
	newCredential := "replacement"
	credentialUpdate := ConnectionInput{
		Name: updated.Name, BaseURL: updated.BaseURL, Credential: &newCredential, Enabled: true,
		ProbeModel: updated.ProbeModel,
	}
	preflight, appErr := service.PreflightExistingOwnerConnection(context.Background(), auth.User{ID: existing.OwnerUserID}, existing.ID, credentialUpdate, updated.Version)
	if appErr != nil || preflight.PreflightToken == "" {
		t.Fatalf("credential preflight result=%+v error=%v", preflight, appErr)
	}
	credentialUpdate.PreflightToken = preflight.PreflightToken
	updated, appErr = service.UpdateOwnerConnection(context.Background(), auth.User{ID: existing.OwnerUserID}, existing.ID, credentialUpdate, updated.Version)
	if appErr != nil {
		t.Fatalf("credential update error: %v", appErr)
	}
	if verifier.calls != 1 || updated.MeasurementVersion != existing.MeasurementVersion+1 {
		t.Fatalf("credential update did not reverify: calls=%d connection=%+v", verifier.calls, updated)
	}
}

func TestPreflightTokenIsBoundOneTimeAndExpires(t *testing.T) {
	now := time.Date(2026, 8, 8, 10, 0, 0, 0, time.UTC)
	repository := &connectionRepository{}
	service := NewService(repository, &connectionVerifier{result: successfulVerification()}, func() time.Time { return now })
	credential := "probe-key"
	input := ConnectionInput{
		Name: "主连接", BaseURL: "https://api.example.com/v1", Credential: &credential,
		ProbeModel: DefaultGPTProbeModel, Enabled: true,
	}
	preflight, appErr := service.PreflightOwnerConnection(context.Background(), auth.User{ID: "owner-1"}, input)
	if appErr != nil || preflight.PreflightToken == "" {
		t.Fatalf("preflight result=%+v error=%v", preflight, appErr)
	}
	input.PreflightToken = preflight.PreflightToken
	if _, appErr = service.CreateOwnerConnection(context.Background(), auth.User{ID: "owner-2"}, input); appErr == nil {
		t.Fatal("token was accepted for another owner")
	}
	if _, appErr = service.CreateOwnerConnection(context.Background(), auth.User{ID: "owner-1"}, input); appErr == nil {
		t.Fatal("consumed token was accepted a second time")
	}

	preflight, appErr = service.PreflightOwnerConnection(context.Background(), auth.User{ID: "owner-1"}, input)
	if appErr != nil || preflight.PreflightToken == "" {
		t.Fatalf("second preflight result=%+v error=%v", preflight, appErr)
	}
	input.PreflightToken = preflight.PreflightToken
	now = now.Add(preflightTokenTTL + time.Second)
	if _, appErr = service.CreateOwnerConnection(context.Background(), auth.User{ID: "owner-1"}, input); appErr == nil {
		t.Fatal("expired token was accepted")
	}
}

func TestPreflightTokenBindsCanonicalTargetCredentialModelAndProtocol(t *testing.T) {
	now := time.Date(2026, 8, 8, 10, 0, 0, 0, time.UTC)
	service := NewService(&connectionRepository{}, &connectionVerifier{result: successfulVerification()}, func() time.Time { return now })
	owner := auth.User{ID: "owner-1"}
	credential := " probe-key "
	input := ConnectionInput{
		Name: "主连接", BaseURL: "https://API.example.com/v1/", Credential: &credential,
		ProbeModel: DefaultGPTProbeModel, Enabled: true,
	}

	preflight, appErr := service.PreflightOwnerConnection(context.Background(), owner, input)
	if appErr != nil {
		t.Fatalf("canonical preflight: %v", appErr)
	}
	canonicalCredential := "probe-key"
	canonicalInput := input
	canonicalInput.BaseURL = "https://api.example.com/v1"
	canonicalInput.Credential = &canonicalCredential
	canonicalInput.PreflightToken = preflight.PreflightToken
	if _, appErr = service.CreateOwnerConnection(context.Background(), owner, canonicalInput); appErr != nil {
		t.Fatalf("canonically equivalent binding was rejected: %v", appErr)
	}

	tests := []struct {
		name   string
		mutate func(*ConnectionInput)
	}{
		{name: "base URL", mutate: func(value *ConnectionInput) { value.BaseURL = "https://api.example.com/v2" }},
		{name: "credential", mutate: func(value *ConnectionInput) { changed := "other-key"; value.Credential = &changed }},
		{name: "model", mutate: func(value *ConnectionInput) { value.ProbeModel = "gpt-5.6-sol" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			preflight, appErr := service.PreflightOwnerConnection(context.Background(), owner, input)
			if appErr != nil {
				t.Fatalf("preflight: %v", appErr)
			}
			changed := input
			changed.PreflightToken = preflight.PreflightToken
			test.mutate(&changed)
			if _, appErr := service.CreateOwnerConnection(context.Background(), owner, changed); appErr == nil {
				t.Fatal("mismatched token binding was accepted")
			}
		})
	}

	preflight, appErr = service.PreflightOwnerConnection(context.Background(), owner, input)
	if appErr != nil {
		t.Fatalf("protocol preflight: %v", appErr)
	}
	service.preflights.mu.Lock()
	grant := service.preflights.grants[preflight.PreflightToken]
	grant.Result.Verification.ProbeProtocol = ProtocolChatCompletionsV1
	service.preflights.grants[preflight.PreflightToken] = grant
	service.preflights.mu.Unlock()
	input.PreflightToken = preflight.PreflightToken
	if _, appErr = service.CreateOwnerConnection(context.Background(), owner, input); appErr == nil {
		t.Fatal("protocol-mismatched token was accepted")
	}
}

func TestUpdateConnectionEnableAndManualVerificationKeepMeasurementVersionWhenConfigurationIsUnchanged(t *testing.T) {
	now := time.Date(2026, 8, 8, 10, 0, 0, 0, time.UTC)
	existing := verifiedConnection(now)
	existing.Enabled = false
	updated, err := UpdateConnection(existing, ConnectionInput{
		Name: existing.Name, BaseURL: existing.BaseURL, ProbeModel: existing.ProbeModel, Enabled: true,
	}, openaiapi.BaseURL{Raw: existing.BaseURL, Canonical: existing.NormalizedBaseURL}, pointerVerification(successfulVerification()), nil, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("enable unchanged connection: %v", err)
	}
	if updated.MeasurementVersion != existing.MeasurementVersion {
		t.Fatalf("enable-only verification changed measurement version: before=%d after=%d", existing.MeasurementVersion, updated.MeasurementVersion)
	}
}

func TestHTTPConnectionRequiresExplicitAcknowledgement(t *testing.T) {
	credential := "key"
	service := NewService(&connectionRepository{}, &connectionVerifier{}, time.Now)
	_, appErr := service.CreateOwnerConnection(context.Background(), auth.User{ID: "owner"}, ConnectionInput{
		Name: "HTTP", BaseURL: "http://155.103.116.134:31238/", Credential: &credential,
	})
	if appErr == nil || len(appErr.FieldErrors) == 0 || appErr.FieldErrors[0].Field != "acknowledgeInsecureHttp" {
		t.Fatalf("unexpected error: %+v", appErr)
	}
}

func TestOwnerConnectionsAttachConnectionLevelHealthSummary(t *testing.T) {
	now := time.Date(2026, 8, 8, 10, 4, 0, 0, time.UTC)
	connection := verifiedConnection(now.Add(-time.Hour))
	repository := &connectionRepository{
		connection: connection,
		found:      true,
		samples: map[string][]Sample{
			connection.ID: {
				finalSample(connection, SlotStart(now).Add(-10*time.Minute), SampleStatusSucceeded),
				finalSample(connection, SlotStart(now).Add(-5*time.Minute), SampleStatusSucceeded),
				finalSample(connection, SlotStart(now), SampleStatusSucceeded),
			},
		},
	}
	service := NewService(repository, &connectionVerifier{}, func() time.Time { return now })

	connections, appErr := service.OwnerConnections(context.Background(), auth.User{ID: connection.OwnerUserID})
	if appErr != nil {
		t.Fatalf("OwnerConnections() error: %v", appErr)
	}
	if len(connections) != 1 || connections[0].HealthSummary.State != HealthStateNormal ||
		connections[0].HealthSummary.SuccessRatePercent == nil || *connections[0].HealthSummary.SuccessRatePercent != "100.0" {
		t.Fatalf("unexpected owner connection health summary: %+v", connections)
	}
}

func TestHealthSummariesExposeDisabledAndStaleRunner(t *testing.T) {
	now := time.Date(2026, 8, 8, 10, 4, 0, 0, time.UTC)
	connection := verifiedConnection(now.Add(-time.Hour))
	repository := &connectionRepository{
		connection: connection,
		found:      true,
		summaryInputs: map[string]SummaryInput{
			"service-1": {Connection: &connection},
		},
	}
	service := NewService(repository, &connectionVerifier{}, func() time.Time { return now })
	status := &staticRunnerStatus{status: RunnerStatus{Enabled: false, ScanInterval: time.Minute}}
	service.SetRunnerStatusProvider(status)

	connections, appErr := service.OwnerConnections(context.Background(), auth.User{ID: connection.OwnerUserID})
	if appErr != nil || len(connections) != 1 || connections[0].HealthSummary.AvailabilityReason != AvailabilityRunnerDisabled {
		t.Fatalf("disabled runner owner summary=%+v error=%v", connections, appErr)
	}
	summaries, appErr := service.Summaries(context.Background(), []string{"service-1"})
	if appErr != nil || summaries["service-1"].AvailabilityReason != AvailabilityRunnerDisabled {
		t.Fatalf("disabled runner public summary=%+v error=%v", summaries, appErr)
	}

	status.status = RunnerStatus{Enabled: true, LastSuccessfulScanAt: now.Add(-11 * time.Minute), ScanInterval: time.Minute}
	connections, appErr = service.OwnerConnections(context.Background(), auth.User{ID: connection.OwnerUserID})
	if appErr != nil || connections[0].HealthSummary.AvailabilityReason != AvailabilityStale {
		t.Fatalf("stale runner owner summary=%+v error=%v", connections, appErr)
	}
}

func TestLatencyRuleValidationRejectsInvalidDimensionsAndIncompleteCalibration(t *testing.T) {
	repository := &calibrationConnectionRepository{
		connectionRepository: &connectionRepository{},
		calibration: Calibration{
			Model: DefaultGPTProbeModel, Protocol: ProtocolResponsesV1,
			Environment: ProbeEnvironmentUSWestV1, CompleteCalendarDays: 6, ConnectionCount: 5,
		},
	}
	service := NewService(repository, &connectionVerifier{}, time.Now)

	if _, appErr := service.PreviewLatencyRule(context.Background(), DefaultGPTProbeModel, "invalid", ProbeEnvironmentUSWestV1, 5000, 10000); appErr == nil || appErr.Status != 422 {
		t.Fatalf("invalid protocol error=%+v", appErr)
	}
	if _, appErr := service.PublishLatencyRule(context.Background(), auth.User{ID: "admin-1"}, DefaultGPTProbeModel, ProtocolResponsesV1, ProbeEnvironmentUSWestV1, 5000, 10000); appErr == nil || appErr.Status != 422 {
		t.Fatalf("incomplete calibration error=%+v", appErr)
	}
	if repository.previewCalls != 0 || repository.publishCalls != 0 {
		t.Fatalf("incomplete calibration reached persistence: preview=%d publish=%d", repository.previewCalls, repository.publishCalls)
	}
}

func TestPublishLatencyRulePersistsReadyCalibrationSnapshot(t *testing.T) {
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	repository := &calibrationConnectionRepository{
		connectionRepository: &connectionRepository{},
		calibration: Calibration{
			Model: DefaultGPTProbeModel, Protocol: ProtocolResponsesV1, Environment: ProbeEnvironmentUSWestV1,
			CompleteCalendarDays: 7, ConnectionCount: 5, SampleCount: 9000, Ready: true,
		},
	}
	service := NewService(repository, &connectionVerifier{}, func() time.Time { return now })

	rule, appErr := service.PublishLatencyRule(context.Background(), auth.User{ID: "admin-1"}, DefaultGPTProbeModel, ProtocolResponsesV1, ProbeEnvironmentUSWestV1, 5000, 10000)

	if appErr != nil || repository.previewCalls != 1 || repository.publishCalls != 1 {
		t.Fatalf("publish error=%v preview=%d publish=%d", appErr, repository.previewCalls, repository.publishCalls)
	}
	if rule.PublishedByAdminID != "admin-1" || !rule.PublishedAt.Equal(now) || rule.SampleCount != 9000 {
		t.Fatalf("unexpected rule snapshot: %+v", rule)
	}
}

type connectionVerifier struct {
	result     VerificationResult
	calls      int
	baseURL    string
	credential string
}

func (verifier *connectionVerifier) Verify(_ context.Context, baseURL, credential, _ string, _ bool) VerificationResult {
	verifier.calls++
	verifier.baseURL = baseURL
	verifier.credential = credential
	return verifier.result
}

type connectionRepository struct {
	connection    Connection
	found         bool
	credential    string
	samples       map[string][]Sample
	summaryInputs map[string]SummaryInput
}

type staticRunnerStatus struct{ status RunnerStatus }

func (provider *staticRunnerStatus) ProbeRunnerStatus() RunnerStatus { return provider.status }

type calibrationConnectionRepository struct {
	*connectionRepository
	calibration  Calibration
	previewCalls int
	publishCalls int
}

func (repository *calibrationConnectionRepository) LoadProbeCalibration(context.Context, string, string, string, time.Time) (Calibration, *domain.AppError) {
	return repository.calibration, nil
}

func (repository *calibrationConnectionRepository) PreviewProbeLatencyRule(_ context.Context, calibration Calibration, slowTTFTMS, hardTimeoutMS int) (LatencyRulePreview, *domain.AppError) {
	repository.previewCalls++
	return LatencyRulePreview{Calibration: calibration, SlowTTFTMS: slowTTFTMS, HardTimeoutMS: hardTimeoutMS}, nil
}

func (repository *calibrationConnectionRepository) PublishProbeLatencyRule(_ context.Context, rule LatencyRule) (LatencyRule, *domain.AppError) {
	repository.publishCalls++
	return rule, nil
}

func (repository *calibrationConnectionRepository) ListProbeLatencyRules(context.Context) ([]LatencyRule, *domain.AppError) {
	return nil, nil
}

func (repository *connectionRepository) ListOwnerProbeConnections(context.Context, string) ([]Connection, *domain.AppError) {
	return []Connection{repository.connection}, nil
}
func (repository *connectionRepository) GetOwnerProbeConnection(context.Context, string, string) (Connection, bool, *domain.AppError) {
	return repository.connection, repository.found, nil
}
func (repository *connectionRepository) GetOwnerProbeConnectionCredential(context.Context, string, string) (Connection, string, bool, *domain.AppError) {
	return repository.connection, repository.credential, repository.found, nil
}
func (repository *connectionRepository) CreateOwnerProbeConnection(_ context.Context, connection Connection, credential string) (Connection, *domain.AppError) {
	connection.ID = "connection-1"
	repository.connection = connection
	repository.credential = credential
	repository.found = true
	return connection, nil
}
func (repository *connectionRepository) UpdateOwnerProbeConnection(_ context.Context, connection Connection, credential *string, _ int64) (Connection, *domain.AppError) {
	repository.connection = connection
	if credential != nil {
		repository.credential = *credential
	}
	return connection, nil
}
func (repository *connectionRepository) DeleteOwnerProbeConnection(context.Context, string, string, int64) *domain.AppError {
	return nil
}
func (repository *connectionRepository) LookupProbeModelPrice(context.Context, string) (PriceSnapshot, bool, *domain.AppError) {
	return PriceSnapshot{}, false, nil
}
func (repository *connectionRepository) LoadOwnerProbeConnectionSamples(context.Context, string, []string, time.Time) (map[string][]Sample, *domain.AppError) {
	if repository.samples == nil {
		return map[string][]Sample{}, nil
	}
	return repository.samples, nil
}
func (repository *connectionRepository) LoadProbeSummaryInputs(context.Context, []string, time.Time) (map[string]SummaryInput, *domain.AppError) {
	if repository.summaryInputs != nil {
		return repository.summaryInputs, nil
	}
	return map[string]SummaryInput{}, nil
}
func (repository *connectionRepository) ClaimDueProbes(context.Context, time.Time, time.Time, int, time.Duration) ([]ProbeJob, *domain.AppError) {
	return nil, nil
}
func (repository *connectionRepository) FinalizeProbe(context.Context, string, ProbeResult, time.Time) (bool, *domain.AppError) {
	return true, nil
}
func (repository *connectionRepository) DeleteFinalProbeSamplesBefore(context.Context, time.Time, int) (int, *domain.AppError) {
	return 0, nil
}

func verifiedConnection(now time.Time) Connection {
	return Connection{
		ID: "connection-1", OwnerUserID: "owner-1", Name: "主连接",
		BaseURL: "https://api.example.com/v1", NormalizedBaseURL: "https://api.example.com/v1",
		CredentialConfigured: true, Enabled: true, VerificationStatus: VerificationVerified,
		VerifiedAt: timePointer(now), ProbeModel: DefaultGPTProbeModel, ProbeProtocol: ProtocolResponsesV1,
		AvailableModels: []string{DefaultGPTProbeModel}, ProbeEnvironment: ProbeEnvironmentUSWestV1,
		MeasurementVersion: 4, Version: 2, CreatedAt: now, UpdatedAt: now,
	}
}

func successfulVerification() VerificationResult {
	return VerificationResult{
		HTTPStatus: 200, AvailableModels: []string{DefaultGPTProbeModel},
		ProbeModel: DefaultGPTProbeModel, ProbeProtocol: ProtocolResponsesV1,
	}
}

func pointerVerification(value VerificationResult) *VerificationResult { return &value }
