package apihealth

import (
	"context"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
)

func TestCreateConnectionVerifiesAndEnablesSuccessfulTarget(t *testing.T) {
	now := time.Date(2026, 8, 8, 10, 0, 0, 0, time.UTC)
	repository := &connectionRepository{}
	verifier := &connectionVerifier{result: ProbeResult{TotalDurationMS: 12, HTTPStatusClass: 2}}
	service := NewService(repository, verifier, func() time.Time { return now })
	credential := " probe-key "

	connection, appErr := service.CreateOwnerConnection(context.Background(), auth.User{ID: "owner-1"}, ConnectionInput{
		Name: " 主 Sub2API ", BaseURL: "https://API.example.com/v1/", Credential: &credential, Enabled: true,
	})
	if appErr != nil {
		t.Fatalf("CreateOwnerConnection() error: %v", appErr)
	}
	if connection.Name != "主 Sub2API" || connection.BaseURL != "https://API.example.com/v1/" || connection.NormalizedBaseURL != "https://api.example.com/v1" {
		t.Fatalf("unexpected connection: %+v", connection)
	}
	if !connection.Enabled || connection.VerificationStatus != VerificationVerified || connection.VerifiedAt == nil {
		t.Fatalf("connection was not verified and enabled: %+v", connection)
	}
	if verifier.baseURL != connection.BaseURL || verifier.credential != "probe-key" || repository.credential != "probe-key" {
		t.Fatalf("verification or persistence did not receive normalized secret input")
	}
}

func TestCreateConnectionPersistsFailedVerificationDisabled(t *testing.T) {
	repository := &connectionRepository{}
	service := NewService(repository, &connectionVerifier{result: ProbeResult{ErrorCode: ErrorAuthorizationInvalid}}, time.Now)
	credential := "bad-key"
	connection, appErr := service.CreateOwnerConnection(context.Background(), auth.User{ID: "owner-1"}, ConnectionInput{
		Name: "低额度探针", BaseURL: "https://api.example.com/v1", Credential: &credential, Enabled: true,
	})
	if appErr != nil {
		t.Fatalf("CreateOwnerConnection() error: %v", appErr)
	}
	if connection.Enabled || connection.VerificationStatus != VerificationFailed || connection.LastVerificationErrorCode != ErrorAuthorizationInvalid {
		t.Fatalf("unexpected failed connection: %+v", connection)
	}
}

func TestUpdateConnectionOnlyReverifiesMaterialChangesOrEnable(t *testing.T) {
	now := time.Date(2026, 8, 8, 10, 0, 0, 0, time.UTC)
	existing := verifiedConnection(now)
	repository := &connectionRepository{connection: existing, found: true, credential: "secret"}
	verifier := &connectionVerifier{}
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
	updated, appErr = service.UpdateOwnerConnection(context.Background(), auth.User{ID: existing.OwnerUserID}, existing.ID, ConnectionInput{
		Name: updated.Name, BaseURL: updated.BaseURL, Credential: &newCredential, Enabled: true,
	}, updated.Version)
	if appErr != nil {
		t.Fatalf("credential update error: %v", appErr)
	}
	if verifier.calls != 1 || updated.MeasurementVersion != existing.MeasurementVersion+1 {
		t.Fatalf("credential update did not reverify: calls=%d connection=%+v", verifier.calls, updated)
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

type connectionVerifier struct {
	result     ProbeResult
	calls      int
	baseURL    string
	credential string
}

func (verifier *connectionVerifier) Verify(_ context.Context, baseURL, credential string, _ bool) ProbeResult {
	verifier.calls++
	verifier.baseURL = baseURL
	verifier.credential = credential
	return verifier.result
}

type connectionRepository struct {
	connection Connection
	found      bool
	credential string
	samples    map[string][]Sample
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
func (repository *connectionRepository) LoadOwnerProbeConnectionSamples(context.Context, string, []string, time.Time) (map[string][]Sample, *domain.AppError) {
	if repository.samples == nil {
		return map[string][]Sample{}, nil
	}
	return repository.samples, nil
}
func (repository *connectionRepository) LoadProbeSummaryInputs(context.Context, []string, time.Time) (map[string]SummaryInput, *domain.AppError) {
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
		VerifiedAt: timePointer(now), MeasurementVersion: 4, Version: 2, CreatedAt: now, UpdatedAt: now,
	}
}
