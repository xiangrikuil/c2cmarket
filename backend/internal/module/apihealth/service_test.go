package apihealth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"net/http"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
)

func TestServicePutOwnerConfigPreservesAuthorizationForSameOriginModelChange(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 4, 12, 0, 0, 0, time.UTC)
	verifiedAt := now.Add(-time.Hour)
	repository := &probeServiceRepository{config: Config{
		ID: "config", APIServiceID: "service", OwnerUserID: "owner",
		Protocol: ProtocolOpenAIChatCompletionsV1, BaseURL: "https://api.example.com/v1",
		NormalizedOrigin: "https://api.example.com:443", Model: "gpt-5",
		CredentialConfigured: true, Enabled: true,
		AuthorizationStatus: AuthorizationVerified, AuthorizationMethod: AuthorizationMethodDNSTXT,
		VerifiedOrigin: "https://api.example.com:443", VerifiedAt: &verifiedAt,
		MeasurementVersion: 4, Version: 6, CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: verifiedAt,
	}, found: true}
	service := NewService(repository, urlValidatorFunc(func(_ context.Context, raw string) (string, error) {
		return raw, nil
	}), nil, nil, func() time.Time { return now }, time.Minute)

	config, appErr := service.PutOwnerConfig(context.Background(), auth.User{ID: "owner"}, " service ", ConfigInput{
		BaseURL: "https://api.example.com/v1", Model: "gpt-5.1", Enabled: true,
	}, 6)
	if appErr != nil {
		t.Fatalf("update owner config: %v", appErr)
	}
	mutation := repository.upsertMutation
	if !mutation.MeasurementInvalidated || mutation.AuthorizationInvalidated {
		t.Fatalf("same-origin model update passed the wrong invalidation flags: %+v", mutation)
	}
	if mutation.Config.Model != "gpt-5.1" || !IsAuthorized(mutation.Config) || mutation.Config.Version != 7 {
		t.Fatalf("unexpected persisted mutation config: %+v", mutation.Config)
	}
	if repository.upsertExpectedVersion != 6 || repository.upsertCredential != nil || config != mutation.Config {
		t.Fatalf("unexpected repository upsert inputs or result: repository=%+v config=%+v", repository, config)
	}
}

func TestConfigValidationErrorUsesProbeAPIKeyWording(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		err     error
		message string
	}{
		{name: "required", err: ErrCredentialRequired, message: "启用探针前必须配置探针专用 API Key。"},
		{name: "invalid", err: ErrCredentialInvalid, message: "探针专用 API Key 不能为空。"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			appErr := configValidationError(test.err)
			if appErr.Detail != test.message || len(appErr.FieldErrors) != 1 ||
				appErr.FieldErrors[0].Field != "credential" || appErr.FieldErrors[0].Message != test.message {
				t.Fatalf("unexpected API Key validation error: %+v", appErr)
			}
		})
	}
}

func TestServicePutOwnerConfigRequiresHTTPAcknowledgementBeforeValidation(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 5, 12, 0, 0, 0, time.UTC)
	repository := &probeServiceRepository{}
	validationCalls := 0
	service := NewService(repository, urlValidatorFunc(func(_ context.Context, raw string) (string, error) {
		validationCalls++
		return raw, nil
	}), nil, nil, func() time.Time { return now }, time.Minute)
	credential := "low-quota-probe-key"

	_, appErr := service.PutOwnerConfig(context.Background(), auth.User{ID: "owner"}, "service", ConfigInput{
		BaseURL: "http://api.example.com", Model: "gpt-5-mini", Credential: &credential,
	}, 0)
	if appErr == nil || appErr.Status != http.StatusUnprocessableEntity || len(appErr.FieldErrors) != 1 ||
		appErr.FieldErrors[0].Field != "acknowledgeInsecureHttp" || validationCalls != 0 {
		t.Fatalf("unexpected missing acknowledgement result: err=%+v validationCalls=%d", appErr, validationCalls)
	}

	config, appErr := service.PutOwnerConfig(context.Background(), auth.User{ID: "owner"}, "service", ConfigInput{
		BaseURL: "http://api.example.com", Model: "gpt-5-mini", Credential: &credential,
		AcknowledgeInsecureHTTP: true,
	}, 0)
	if appErr != nil {
		t.Fatalf("put acknowledged HTTP config: %v", appErr)
	}
	if validationCalls != 1 || config.BaseURL != "http://api.example.com/v1" || config.NormalizedOrigin != "http://api.example.com:80" {
		t.Fatalf("unexpected acknowledged HTTP config=%+v validationCalls=%d", config, validationCalls)
	}
}

func TestServiceCreateChallengeBindsExactTarget(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 4, 12, 0, 0, 0, time.UTC)
	repository := &probeServiceRepository{config: Config{
		ID: "config", APIServiceID: "service", OwnerUserID: "owner",
		NormalizedOrigin: "https://api.example.com:443", Version: 3,
	}, found: true}
	service := NewService(repository, nil, nil, nil, func() time.Time { return now }, 30*time.Minute)
	service.random = bytes.NewReader(make([]byte, challengeTokenBytes))

	challenge, appErr := service.CreateChallenge(context.Background(), auth.User{ID: "owner"}, " service ", AuthorizationMethodDNSTXT, 3)
	if appErr != nil {
		t.Fatalf("create DNS challenge: %v", appErr)
	}
	if challenge.Token == "" || challenge.DNSRecordName != "_c2cmarket-probe.api.example.com" || challenge.HTTPURL != "" {
		t.Fatalf("unexpected DNS challenge: %+v", challenge)
	}
	if challenge.ExpiresAt != now.Add(30*time.Minute) || challenge.ConfigVersion != 4 {
		t.Fatalf("unexpected challenge version or expiry: %+v", challenge)
	}
	expectedHash := sha256.Sum256([]byte(challenge.Token))
	if repository.createdMethod != AuthorizationMethodDNSTXT || !bytes.Equal(repository.createdTokenHash, expectedHash[:]) {
		t.Fatalf("challenge hash or method was not persisted")
	}

	repository.config.NormalizedOrigin = "https://api.example.com:8443"
	_, appErr = service.CreateChallenge(context.Background(), auth.User{ID: "owner"}, "service", AuthorizationMethodDNSTXT, 3)
	if appErr == nil || appErr.Status != http.StatusUnprocessableEntity || len(appErr.FieldErrors) != 1 || appErr.FieldErrors[0].Field != "method" {
		t.Fatalf("expected non-443 DNS rejection, got %v", appErr)
	}
}

func TestServiceVerifyDNSChallenge(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 4, 12, 0, 0, 0, time.UTC)
	token := "dns-control-token"
	repository := &probeServiceRepository{challenge: storedProbeChallenge(token, AuthorizationMethodDNSTXT, now.Add(time.Minute))}
	resolver := txtResolverFunc(func(_ context.Context, name string) ([]string, error) {
		if name != "_c2cmarket-probe.api.example.com" {
			t.Fatalf("unexpected TXT name: %s", name)
		}
		return []string{"unrelated", token}, nil
	})
	service := NewService(repository, nil, resolver, nil, func() time.Time { return now }, time.Minute)

	config, appErr := service.VerifyChallenge(context.Background(), auth.User{ID: "owner"}, "service", 7)
	if appErr != nil {
		t.Fatalf("verify DNS challenge: %v", appErr)
	}
	if !repository.completedSucceeded || repository.completedReason != "" || config.AuthorizationStatus != AuthorizationVerified {
		t.Fatalf("unexpected verification completion: config=%+v repository=%+v", config, repository)
	}
}

func TestServiceVerifyHTTPChallengeDoesNotSendCredential(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 4, 12, 0, 0, 0, time.UTC)
	token := "http-control-token"
	repository := &probeServiceRepository{challenge: storedProbeChallenge(token, AuthorizationMethodHTTPChallenge, now.Add(time.Minute))}
	var captured *http.Request
	factory := httpClientFactoryFunc(func(Config) (*http.Client, error) {
		return &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			captured = request
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewBufferString(token)),
			}, nil
		})}, nil
	})
	service := NewService(repository, nil, nil, factory, func() time.Time { return now }, time.Minute)

	_, appErr := service.VerifyChallenge(context.Background(), auth.User{ID: "owner"}, "service", 7)
	if appErr != nil {
		t.Fatalf("verify HTTP challenge: %v", appErr)
	}
	if captured == nil || captured.Method != http.MethodGet || captured.URL.String() != "https://api.example.com:443/.well-known/c2cmarket-probe-verification" {
		t.Fatalf("unexpected verification request: %v", captured)
	}
	if captured.Header.Get("Authorization") != "" {
		t.Fatalf("HTTP ownership challenge leaked an authorization header")
	}
	if !repository.completedSucceeded || repository.completedReason != "" {
		t.Fatalf("unexpected verification completion: %+v", repository)
	}
}

func TestServiceVerifyInsecureHTTPChallengeDoesNotSendCredential(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 5, 12, 0, 0, 0, time.UTC)
	token := "http-control-token"
	challenge := storedProbeChallenge(token, AuthorizationMethodHTTPChallenge, now.Add(time.Minute))
	challenge.Config.BaseURL = "http://api.example.com/v1"
	challenge.Config.NormalizedOrigin = "http://api.example.com:80"
	repository := &probeServiceRepository{challenge: challenge}
	var captured *http.Request
	factory := httpClientFactoryFunc(func(Config) (*http.Client, error) {
		return &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			captured = request
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewBufferString(token)),
			}, nil
		})}, nil
	})
	service := NewService(repository, nil, nil, factory, func() time.Time { return now }, time.Minute)

	_, appErr := service.VerifyChallenge(context.Background(), auth.User{ID: "owner"}, "service", 7)
	if appErr != nil {
		t.Fatalf("verify insecure HTTP challenge: %v", appErr)
	}
	if captured == nil || captured.URL.String() != "http://api.example.com:80/.well-known/c2cmarket-probe-verification" {
		t.Fatalf("unexpected HTTP challenge target: %v", captured)
	}
	if captured.Header.Get("Authorization") != "" {
		t.Fatalf("HTTP challenge sent credential header: %v", captured.Header)
	}
}

func TestServiceRejectsExpiredChallengeWithoutNetworkAccess(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, time.August, 4, 12, 0, 0, 0, time.UTC)
	repository := &probeServiceRepository{challenge: storedProbeChallenge("expired-token", AuthorizationMethodDNSTXT, now)}
	resolverCalled := false
	service := NewService(repository, nil, txtResolverFunc(func(context.Context, string) ([]string, error) {
		resolverCalled = true
		return nil, nil
	}), nil, func() time.Time { return now }, time.Minute)

	_, appErr := service.VerifyChallenge(context.Background(), auth.User{ID: "owner"}, "service", 7)
	if appErr != nil {
		t.Fatalf("complete expired challenge: %v", appErr)
	}
	if resolverCalled || repository.completedSucceeded || repository.completedReason != "challenge_expired" {
		t.Fatalf("expired challenge was not rejected locally: %+v", repository)
	}
}

func storedProbeChallenge(token, method string, expiresAt time.Time) StoredChallenge {
	hash := sha256.Sum256([]byte(token))
	return StoredChallenge{
		Config: Config{
			ID: "config", APIServiceID: "service", OwnerUserID: "owner",
			NormalizedOrigin: "https://api.example.com:443", Version: 7,
		},
		Method: method, TokenHash: hash[:], ExpiresAt: expiresAt,
	}
}

type txtResolverFunc func(context.Context, string) ([]string, error)

func (function txtResolverFunc) LookupTXT(ctx context.Context, name string) ([]string, error) {
	return function(ctx, name)
}

type httpClientFactoryFunc func(Config) (*http.Client, error)

func (function httpClientFactoryFunc) ClientFor(config Config) (*http.Client, error) {
	return function(config)
}

type urlValidatorFunc func(context.Context, string) (string, error)

func (function urlValidatorFunc) ValidateURL(ctx context.Context, raw string) (string, error) {
	return function(ctx, raw)
}

type probeServiceRepository struct {
	config                Config
	found                 bool
	upsertMutation        ConfigMutation
	upsertCredential      *string
	upsertExpectedVersion int64
	challenge             StoredChallenge
	createdMethod         string
	createdTokenHash      []byte
	completedSucceeded    bool
	completedReason       string
}

func (repository *probeServiceRepository) GetOwnerProbeConfig(context.Context, string, string) (Config, bool, *domain.AppError) {
	return repository.config, repository.found, nil
}

func (repository *probeServiceRepository) UpsertOwnerProbeConfig(_ context.Context, mutation ConfigMutation, credential *string, expectedVersion int64) (Config, *domain.AppError) {
	repository.upsertMutation = mutation
	repository.upsertCredential = credential
	repository.upsertExpectedVersion = expectedVersion
	return mutation.Config, nil
}

func (repository *probeServiceRepository) DeleteOwnerProbeConfig(context.Context, string, string, int64, time.Time) *domain.AppError {
	return nil
}

func (repository *probeServiceRepository) CreateProbeChallenge(_ context.Context, _, _ string, method string, tokenHash []byte, _ time.Time, _ int64, _ time.Time) (Config, *domain.AppError) {
	repository.createdMethod = method
	repository.createdTokenHash = append([]byte(nil), tokenHash...)
	updated := repository.config
	updated.Version++
	return updated, nil
}

func (repository *probeServiceRepository) GetProbeChallenge(context.Context, string, string) (StoredChallenge, *domain.AppError) {
	return repository.challenge, nil
}

func (repository *probeServiceRepository) CompleteProbeVerification(_ context.Context, _, _ string, _ string, _ int64, succeeded bool, reason string, now time.Time) (Config, *domain.AppError) {
	repository.completedSucceeded = succeeded
	repository.completedReason = reason
	config := repository.challenge.Config
	config.Version++
	if succeeded {
		config.AuthorizationStatus = AuthorizationVerified
		config.AuthorizationMethod = repository.challenge.Method
		config.VerifiedOrigin = config.NormalizedOrigin
		config.VerifiedAt = &now
	}
	return config, nil
}

func (repository *probeServiceRepository) ListAdminProbeConfigs(context.Context, string, domain.PageRequest) (domain.Page[Config], *domain.AppError) {
	return domain.Page[Config]{}, nil
}

func (repository *probeServiceRepository) AdminDecideProbeConfig(context.Context, string, string, int64, bool, string, time.Time) (Config, *domain.AppError) {
	return Config{}, nil
}

func (repository *probeServiceRepository) LoadProbeSummaryInputs(context.Context, []string, time.Time) (map[string]SummaryInput, *domain.AppError) {
	return map[string]SummaryInput{}, nil
}

func (repository *probeServiceRepository) ClaimDueProbes(context.Context, time.Time, time.Time, int, time.Duration) ([]ProbeJob, *domain.AppError) {
	return nil, nil
}

func (repository *probeServiceRepository) FinalizeProbe(context.Context, string, ProbeResult, time.Time) (bool, *domain.AppError) {
	return false, nil
}

func (repository *probeServiceRepository) DeleteFinalProbeSamplesBefore(context.Context, time.Time, int) (int, *domain.AppError) {
	return 0, nil
}
