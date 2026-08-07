package apihealth

import (
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestVersionConflictUsesPreconditionFailed(t *testing.T) {
	t.Parallel()
	appErr := versionConflict()
	if appErr.Status != http.StatusPreconditionFailed || appErr.Code != "VERSION_CONFLICT" {
		t.Fatalf("unexpected version conflict: %+v", appErr)
	}
}

func TestBuildConfigMutationSeparatesConfigAndMeasurementVersions(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	credential := "probe-secret"
	created, err := BuildConfigMutation(nil, "service", "owner", ConfigInput{
		BaseURL: "https://api.example.com/v1", Model: "gpt-5", Credential: &credential, Enabled: true,
	}, now)
	if err != nil {
		t.Fatalf("create config: %v", err)
	}
	if created.Config.Version != 1 || created.Config.MeasurementVersion != 1 || created.Config.AuthorizationStatus != AuthorizationPending {
		t.Fatalf("unexpected created config: %+v", created.Config)
	}

	verifiedAt := now
	existing := created.Config
	existing.ID = "config"
	existing.AuthorizationStatus = AuthorizationVerified
	existing.AuthorizationMethod = AuthorizationMethodDNSTXT
	existing.VerifiedOrigin = existing.NormalizedOrigin
	existing.VerifiedAt = &verifiedAt

	metadataOnly, err := BuildConfigMutation(&existing, "service", "owner", ConfigInput{
		BaseURL: existing.BaseURL, Model: existing.Model, Enabled: false,
	}, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("update metadata: %v", err)
	}
	if metadataOnly.Config.Version != 2 || metadataOnly.Config.MeasurementVersion != 1 || metadataOnly.MeasurementInvalidated || !IsAuthorized(metadataOnly.Config) {
		t.Fatalf("metadata update changed measurement identity: %+v", metadataOnly)
	}

	modelChanged, err := BuildConfigMutation(&metadataOnly.Config, "service", "owner", ConfigInput{
		BaseURL: existing.BaseURL, Model: "gpt-5.1", Enabled: false,
	}, now.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("change model: %v", err)
	}
	if modelChanged.Config.Version != 3 || modelChanged.Config.MeasurementVersion != 2 ||
		!modelChanged.MeasurementInvalidated || modelChanged.AuthorizationInvalidated || !IsAuthorized(modelChanged.Config) {
		t.Fatalf("model update did not preserve same-origin authorization: %+v", modelChanged)
	}

	pathChanged, err := BuildConfigMutation(&modelChanged.Config, "service", "owner", ConfigInput{
		BaseURL: "https://api.example.com/openai/v1", Model: modelChanged.Config.Model, Enabled: false,
	}, now.Add(3*time.Minute))
	if err != nil {
		t.Fatalf("change base path: %v", err)
	}
	if pathChanged.Config.MeasurementVersion != 3 || !pathChanged.MeasurementInvalidated ||
		pathChanged.AuthorizationInvalidated || !IsAuthorized(pathChanged.Config) {
		t.Fatalf("base-path update did not preserve same-origin authorization: %+v", pathChanged)
	}

	originChanged, err := BuildConfigMutation(&pathChanged.Config, "service", "owner", ConfigInput{
		BaseURL: "https://other.example.com/v1", Model: pathChanged.Config.Model, Enabled: false,
	}, now.Add(4*time.Minute))
	if err != nil {
		t.Fatalf("change origin: %v", err)
	}
	if originChanged.Config.MeasurementVersion != 4 || !originChanged.MeasurementInvalidated ||
		!originChanged.AuthorizationInvalidated || originChanged.Config.AuthorizationStatus != AuthorizationPending {
		t.Fatalf("origin update retained authorization: %+v", originChanged)
	}
}

func TestBuildConfigMutationPreservesPendingChallengeWhenMeasurementChangesOnSameOrigin(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(10 * time.Minute)
	existing := Config{
		ID: "config", APIServiceID: "service", OwnerUserID: "owner",
		Protocol: ProtocolOpenAIChatCompletionsV1, BaseURL: "https://api.example.com/v1",
		NormalizedOrigin: "https://api.example.com:443", Model: "gpt-5",
		CredentialConfigured: true, AuthorizationStatus: AuthorizationPending,
		AuthorizationMethod: AuthorizationMethodDNSTXT, ChallengeExpiresAt: &expiresAt,
		MeasurementVersion: 1, Version: 1, CreatedAt: now, UpdatedAt: now,
	}

	mutation, err := BuildConfigMutation(&existing, "service", "owner", ConfigInput{
		BaseURL: existing.BaseURL, Model: "gpt-5.1", Enabled: false,
	}, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("change pending config model: %v", err)
	}
	if !mutation.MeasurementInvalidated || mutation.AuthorizationInvalidated {
		t.Fatalf("same-origin measurement change invalidated pending authorization: %+v", mutation)
	}
	if mutation.Config.ChallengeExpiresAt != existing.ChallengeExpiresAt || mutation.Config.AuthorizationMethod != AuthorizationMethodDNSTXT {
		t.Fatalf("same-origin measurement change cleared pending challenge: %+v", mutation.Config)
	}
}

func TestBuildConfigMutationRequiresCredentialBeforeEnable(t *testing.T) {
	t.Parallel()
	_, err := BuildConfigMutation(nil, "service", "owner", ConfigInput{
		BaseURL: "https://api.example.com/v1", Model: "gpt-5", Enabled: true,
	}, time.Now())
	if !errors.Is(err, ErrCredentialRequired) {
		t.Fatalf("expected credential error, got %v", err)
	}
}

func TestBuildConfigMutationRequiresExplicitInsecureHTTPAcknowledgement(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	credential := "low-quota-probe-key"
	_, err := BuildConfigMutation(nil, "service", "owner", ConfigInput{
		BaseURL: "http://api.example.com", Model: "gpt-5-mini", Credential: &credential,
	}, now)
	if !errors.Is(err, ErrInsecureHTTPNotAcknowledged) {
		t.Fatalf("expected acknowledgement error, got %v", err)
	}

	mutation, err := BuildConfigMutation(nil, "service", "owner", ConfigInput{
		BaseURL: "http://api.example.com", Model: "gpt-5-mini", Credential: &credential,
		AcknowledgeInsecureHTTP: true,
	}, now)
	if err != nil {
		t.Fatalf("build acknowledged HTTP config: %v", err)
	}
	if mutation.Config.BaseURL != "http://api.example.com/v1" || mutation.Config.NormalizedOrigin != "http://api.example.com:80" {
		t.Fatalf("unexpected acknowledged HTTP config: %+v", mutation.Config)
	}
}

func TestBuildConfigMutationInvalidatesAuthorizationWhenSchemeChanges(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	verifiedAt := now.Add(-time.Hour)
	existing := Config{
		ID: "config", APIServiceID: "service", OwnerUserID: "owner",
		Protocol: ProtocolOpenAIChatCompletionsV1, BaseURL: "https://api.example.com/v1",
		NormalizedOrigin: "https://api.example.com:443", Model: "gpt-5-mini",
		CredentialConfigured: true, Enabled: true,
		AuthorizationStatus: AuthorizationVerified, AuthorizationMethod: AuthorizationMethodDNSTXT,
		VerifiedOrigin: "https://api.example.com:443", VerifiedAt: &verifiedAt,
		MeasurementVersion: 2, Version: 3, CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: verifiedAt,
	}
	mutation, err := BuildConfigMutation(&existing, "service", "owner", ConfigInput{
		BaseURL: "http://api.example.com/v1", Model: existing.Model, Enabled: true,
		AcknowledgeInsecureHTTP: true,
	}, now)
	if err != nil {
		t.Fatalf("change target scheme: %v", err)
	}
	if !mutation.AuthorizationInvalidated || !mutation.MeasurementInvalidated ||
		mutation.Config.AuthorizationStatus != AuthorizationPending || mutation.Config.VerifiedOrigin != "" ||
		mutation.Config.MeasurementVersion != 3 {
		t.Fatalf("scheme change retained authorization: %+v", mutation)
	}
}
