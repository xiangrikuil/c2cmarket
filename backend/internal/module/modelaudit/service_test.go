package modelaudit

import (
	"context"
	"testing"
	"time"

	"c2c-market/backend/internal/module/auth"
)

type recordingChatAdapter struct {
	responseText string
	requests     []AuditChatRequest
}

func (a *recordingChatAdapter) Chat(_ context.Context, request AuditChatRequest) (AuditChatResponse, error) {
	a.requests = append(a.requests, request)
	text := a.responseText
	if text == "" {
		text = "7"
	}
	return AuditChatResponse{
		Text:      text,
		Model:     request.Model,
		LatencyMS: 12,
		Usage:     &AuditUsage{PromptTokens: 8, CompletionTokens: 1, TotalTokens: 9},
	}, nil
}

func TestCreateRunUsesClaimedModelOverrideForProbeRequest(t *testing.T) {
	service := newTestModelAuditService(t, &recordingChatAdapter{responseText: "7"})
	target := createTestModelAuditTarget(t, service, "target-default-model")

	adapter := &recordingChatAdapter{responseText: "7"}
	service.SetAdapterFactory(func(target Target, apiKey string) ProviderAdapter {
		if target.ClaimedModel != "run-override-model" {
			t.Fatalf("adapter factory received model %q, want run override", target.ClaimedModel)
		}
		return adapter
	})

	run, appErr := service.CreateRun(context.Background(), testAdminUser(), RunInput{
		TargetID:     target.ID,
		ClaimedModel: "run-override-model",
		Mode:         AuditModeQuick,
	})
	if appErr != nil {
		t.Fatalf("CreateRun returned error: %+v", appErr)
	}
	if run.ClaimedModel != "run-override-model" {
		t.Fatalf("run kept claimed model %q, want override", run.ClaimedModel)
	}
	if len(adapter.requests) != 1 {
		t.Fatalf("expected one low-sample request without baseline, got %d", len(adapter.requests))
	}
	if got := adapter.requests[0].Model; got != "run-override-model" {
		t.Fatalf("probe request used model %q, want run override", got)
	}
}

func TestCreateRunComparesRandomFingerprintAgainstSelectedBaseline(t *testing.T) {
	adapter := &recordingChatAdapter{responseText: "4"}
	service := newTestModelAuditService(t, adapter)
	target := createTestModelAuditTarget(t, service, "gpt-example")
	baseline, appErr := service.CreateBaseline(context.Background(), testAdminUser(), BaselineInput{
		BaselineName:    "gpt-example official",
		Model:           "gpt-example",
		SourceType:      "official_api",
		ProbeSetVersion: "2026-07-v1",
		ParamsJSON:      map[string]any{"temperature": 0.7},
		FeatureJSON: map[string]any{
			randomFingerprintFeatureJSONKey: RandomFingerprintFeatureSet{Categorical: []CategoricalFingerprintFeature{{
				PromptID: "rand_digit_1_10_v1",
				N:        80,
				Counts:   map[string]int{"7": 72, "4": 8},
				Values:   []string{"4", "7"},
			}}},
		},
		SampleCount: 80,
	})
	if appErr != nil {
		t.Fatalf("CreateBaseline returned error: %+v", appErr)
	}

	run, appErr := service.CreateRun(context.Background(), testAdminUser(), RunInput{
		TargetID:   target.ID,
		BaselineID: baseline.ID,
		Mode:       AuditModeQuick,
	})
	if appErr != nil {
		t.Fatalf("CreateRun returned error: %+v", appErr)
	}
	if len(adapter.requests) != randomFingerprintMinSamples {
		t.Fatalf("expected %d random fingerprint requests, got %d", randomFingerprintMinSamples, len(adapter.requests))
	}
	score := probeScoreForTest(t, run, ProbeRandomFingerprint)
	if score.Risk != RiskHigh {
		t.Fatalf("expected high random fingerprint risk against selected baseline, got %+v", score)
	}
	if score.Evidence["baseline_id"] != baseline.ID {
		t.Fatalf("score evidence did not keep selected baseline id: %+v", score.Evidence)
	}
}

func newTestModelAuditService(t *testing.T, adapter ProviderAdapter) *Service {
	t.Helper()
	now := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	service := NewService(nil, func() time.Time { return now })
	service.SetAdapterFactory(func(Target, string) ProviderAdapter {
		return adapter
	})
	return service
}

func createTestModelAuditTarget(t *testing.T, service *Service, claimedModel string) Target {
	t.Helper()
	target, appErr := service.CreateTarget(context.Background(), testAdminUser(), TargetInput{
		Name:         "Test target",
		BaseURL:      "https://relay.example.test/v1",
		ProviderType: "openai_compatible",
		ClaimedModel: claimedModel,
		APIKey:       "sk-test",
		Enabled:      true,
	})
	if appErr != nil {
		t.Fatalf("CreateTarget returned error: %+v", appErr)
	}
	return target
}

func testAdminUser() auth.User {
	return auth.User{ID: "admin", IsAdmin: true}
}

func probeScoreForTest(t *testing.T, run Run, probe ProbeName) ProbeScore {
	t.Helper()
	for _, score := range run.ProbeScores {
		if score.Probe == probe {
			return score
		}
	}
	t.Fatalf("missing probe score %s in %+v", probe, run.ProbeScores)
	return ProbeScore{}
}
