package modelaudit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"

	"github.com/google/uuid"
)

type ChatAdapterFactory func(target Target, apiKey string) ProviderAdapter

type Service struct {
	mu             sync.Mutex
	now            func() time.Time
	repo           Repository
	adapterFactory ChatAdapterFactory
	targets        map[string]Target
	targetSecrets  map[string]string
	targetOrder    []string
	baselines      map[string]Baseline
	baselineOrder  []string
	runs           map[string]Run
	runOrder       []string
	samples        map[string][]Sample
	probeScores    map[string][]ProbeScore
	monitors       map[string]Monitor
	monitorOrder   []string
}

func NewService(repo Repository, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	service := &Service{
		now:           now,
		repo:          repo,
		targets:       map[string]Target{},
		targetSecrets: map[string]string{},
		baselines:     map[string]Baseline{},
		runs:          map[string]Run{},
		samples:       map[string][]Sample{},
		probeScores:   map[string][]ProbeScore{},
		monitors:      map[string]Monitor{},
	}
	service.adapterFactory = func(target Target, apiKey string) ProviderAdapter {
		return NewOpenAICompatibleAdapter(target.BaseURL, apiKey)
	}
	return service
}

func (s *Service) SetAdapterFactory(factory ChatAdapterFactory) {
	if factory == nil {
		return
	}
	s.adapterFactory = factory
}

func (s *Service) AdminTargets(ctx context.Context, user auth.User) ([]Target, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return nil, appErr
	}
	if s.repo != nil {
		return s.repo.ListModelAuditTargets(ctx)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	targets := make([]Target, 0, len(s.targetOrder))
	for _, id := range s.targetOrder {
		targets = append(targets, s.targets[id])
	}
	return targets, nil
}

func (s *Service) AdminTarget(ctx context.Context, user auth.User, targetID string) (Target, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return Target{}, appErr
	}
	if s.repo != nil {
		return s.repo.GetModelAuditTarget(ctx, targetID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	target, ok := s.targets[targetID]
	if !ok {
		return Target{}, notFound("审计目标不存在。")
	}
	return target, nil
}

func (s *Service) CreateTarget(ctx context.Context, user auth.User, input TargetInput) (Target, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return Target{}, appErr
	}
	target, appErr := s.buildTarget(Target{}, input, true)
	if appErr != nil {
		return Target{}, appErr
	}
	if s.repo != nil {
		return s.repo.CreateModelAuditTarget(ctx, target, strings.TrimSpace(input.APIKey))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.targets[target.ID] = target
	s.targetSecrets[target.ID] = strings.TrimSpace(input.APIKey)
	s.targetOrder = append(s.targetOrder, target.ID)
	return target, nil
}

func (s *Service) UpdateTarget(ctx context.Context, user auth.User, targetID string, input TargetInput) (Target, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return Target{}, appErr
	}
	current, appErr := s.AdminTarget(ctx, user, targetID)
	if appErr != nil {
		return Target{}, appErr
	}
	target, appErr := s.buildTarget(current, input, false)
	if appErr != nil {
		return Target{}, appErr
	}
	var apiKey *string
	if strings.TrimSpace(input.APIKey) != "" {
		value := strings.TrimSpace(input.APIKey)
		apiKey = &value
	}
	if s.repo != nil {
		return s.repo.UpdateModelAuditTarget(ctx, target, apiKey)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.targets[target.ID] = target
	if apiKey != nil {
		s.targetSecrets[target.ID] = *apiKey
	}
	return target, nil
}

func (s *Service) DeleteTarget(ctx context.Context, user auth.User, targetID string) *domain.AppError {
	if appErr := requireAdmin(user); appErr != nil {
		return appErr
	}
	if s.repo != nil {
		return s.repo.DeleteModelAuditTarget(ctx, targetID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.targets[targetID]; !ok {
		return notFound("审计目标不存在。")
	}
	delete(s.targets, targetID)
	delete(s.targetSecrets, targetID)
	return nil
}

func (s *Service) AdminBaselines(ctx context.Context, user auth.User) ([]Baseline, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return nil, appErr
	}
	if s.repo != nil {
		return s.repo.ListModelAuditBaselines(ctx)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	baselines := make([]Baseline, 0, len(s.baselineOrder))
	for _, id := range s.baselineOrder {
		baselines = append(baselines, s.baselines[id])
	}
	return baselines, nil
}

func (s *Service) AdminBaseline(ctx context.Context, user auth.User, baselineID string) (Baseline, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return Baseline{}, appErr
	}
	if s.repo != nil {
		return s.repo.GetModelAuditBaseline(ctx, baselineID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	baseline, ok := s.baselines[baselineID]
	if !ok {
		return Baseline{}, notFound("基线不存在。")
	}
	return baseline, nil
}

func (s *Service) CreateBaseline(ctx context.Context, user auth.User, input BaselineInput) (Baseline, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return Baseline{}, appErr
	}
	baseline, appErr := s.buildBaseline(input)
	if appErr != nil {
		return Baseline{}, appErr
	}
	if s.repo != nil {
		return s.repo.CreateModelAuditBaseline(ctx, baseline)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.baselines[baseline.ID] = baseline
	s.baselineOrder = append(s.baselineOrder, baseline.ID)
	return baseline, nil
}

func (s *Service) AdminRuns(ctx context.Context, user auth.User) ([]Run, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return nil, appErr
	}
	if s.repo != nil {
		return s.repo.ListModelAuditRuns(ctx)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	runs := make([]Run, 0, len(s.runOrder))
	for _, id := range s.runOrder {
		runs = append(runs, s.runs[id])
	}
	return runs, nil
}

func (s *Service) AdminRun(ctx context.Context, user auth.User, runID string) (Run, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return Run{}, appErr
	}
	run, appErr := s.getRun(ctx, runID)
	if appErr != nil {
		return Run{}, appErr
	}
	scores, _ := s.listProbeScores(ctx, runID)
	run.ProbeScores = scores
	return run, nil
}

func (s *Service) CreateRun(ctx context.Context, user auth.User, input RunInput) (Run, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return Run{}, appErr
	}
	if input.Mode == "" {
		input.Mode = AuditModeQuick
	}
	if !validAuditMode(input.Mode) {
		return Run{}, validation("mode", "invalid", "审计模式不支持。")
	}
	target, apiKey, appErr := s.targetWithSecret(ctx, input.TargetID)
	if appErr != nil {
		return Run{}, appErr
	}
	if !target.Enabled {
		return Run{}, validation("targetId", "disabled", "审计目标已禁用。")
	}
	claimedModel := strings.TrimSpace(input.ClaimedModel)
	if claimedModel == "" {
		claimedModel = target.ClaimedModel
	}
	baselineID := strings.TrimSpace(input.BaselineID)
	var baseline *Baseline
	if baselineID != "" {
		loaded, appErr := s.getBaseline(ctx, baselineID)
		if appErr != nil {
			return Run{}, appErr
		}
		baseline = &loaded
	}
	now := s.now().UTC()
	run := Run{
		ID:           uuid.NewString(),
		TargetID:     target.ID,
		TargetName:   target.Name,
		ClaimedModel: claimedModel,
		BaselineID:   baselineID,
		Status:       RunStatusRunning,
		Mode:         input.Mode,
		StartedAt:    &now,
		CreatedAt:    now,
	}
	created, appErr := s.createRun(ctx, run)
	if appErr != nil {
		return Run{}, appErr
	}

	target.ClaimedModel = claimedModel
	completed := s.executeRun(ctx, created, target, baseline, apiKey, input)
	return s.updateRun(ctx, completed)
}

func (s *Service) CancelRun(ctx context.Context, user auth.User, runID string) (Run, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return Run{}, appErr
	}
	if s.repo != nil {
		return s.repo.CancelModelAuditRun(ctx, runID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[runID]
	if !ok {
		return Run{}, notFound("审计运行不存在。")
	}
	if run.Status == RunStatusCompleted || run.Status == RunStatusFailed {
		return run, nil
	}
	now := s.now().UTC()
	run.Status = RunStatusCancelled
	run.FinishedAt = &now
	s.runs[runID] = run
	return run, nil
}

func (s *Service) AdminReport(ctx context.Context, user auth.User, runID string) (AuditReport, *domain.AppError) {
	run, appErr := s.AdminRun(ctx, user, runID)
	if appErr != nil {
		return AuditReport{}, appErr
	}
	if len(run.ReportJSON) > 0 {
		return BuildAuditReport(AuditReportInput{
			RunID:             run.ID,
			TargetID:          run.TargetID,
			TargetName:        run.TargetName,
			ClaimedModel:      run.ClaimedModel,
			BaselineID:        run.BaselineID,
			Mode:              run.Mode,
			OverallRisk:       run.RiskLevel,
			OverallScore:      run.OverallScore,
			OverallConfidence: run.Confidence,
			ProbeScores:       run.ProbeScores,
			CreatedAt:         run.CreatedAt,
		}), nil
	}
	return BuildAuditReport(AuditReportInput{
		RunID:             run.ID,
		TargetID:          run.TargetID,
		TargetName:        run.TargetName,
		ClaimedModel:      run.ClaimedModel,
		BaselineID:        run.BaselineID,
		Mode:              run.Mode,
		OverallRisk:       run.RiskLevel,
		OverallScore:      run.OverallScore,
		OverallConfidence: run.Confidence,
		ProbeScores:       run.ProbeScores,
		CreatedAt:         run.CreatedAt,
	}), nil
}

func (s *Service) AdminMonitors(ctx context.Context, user auth.User) ([]Monitor, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return nil, appErr
	}
	if s.repo != nil {
		return s.repo.ListModelAuditMonitors(ctx)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	monitors := make([]Monitor, 0, len(s.monitorOrder))
	for _, id := range s.monitorOrder {
		monitors = append(monitors, s.monitors[id])
	}
	return monitors, nil
}

func (s *Service) CreateMonitor(ctx context.Context, user auth.User, input MonitorInput) (Monitor, *domain.AppError) {
	if appErr := requireAdmin(user); appErr != nil {
		return Monitor{}, appErr
	}
	if strings.TrimSpace(input.TargetID) == "" {
		return Monitor{}, validation("targetId", "required", "必须选择审计目标。")
	}
	if input.Mode == "" {
		input.Mode = AuditModeScheduled
	}
	if !validAuditMode(input.Mode) {
		return Monitor{}, validation("mode", "invalid", "巡检模式不支持。")
	}
	now := s.now().UTC()
	monitor := Monitor{
		ID:         uuid.NewString(),
		TargetID:   strings.TrimSpace(input.TargetID),
		BaselineID: strings.TrimSpace(input.BaselineID),
		Mode:       input.Mode,
		Enabled:    input.Enabled,
		CronSpec:   strings.TrimSpace(input.CronSpec),
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if s.repo != nil {
		return s.repo.CreateModelAuditMonitor(ctx, monitor)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.monitors[monitor.ID] = monitor
	s.monitorOrder = append(s.monitorOrder, monitor.ID)
	return monitor, nil
}

func (s *Service) executeRun(ctx context.Context, run Run, target Target, baseline *Baseline, apiKey string, input RunInput) Run {
	adapter := s.adapterFactory(target, apiKey)
	scores := []ProbeScore{}
	samples := []Sample{}

	randomScore, randomSamples := s.runRandomProbe(ctx, run, target, baseline, adapter, input)
	scores = append(scores, randomScore)
	samples = append(samples, randomSamples...)

	billingScore := s.runBillingLatencyProbe(ctx, run, adapter, samples)
	scores = append(scores, billingScore)

	if input.Mode != AuditModeQuick {
		scores = append(scores, RunActiveFingerprintProbe(nil, nil))
		scores = append(scores, RunKBFProbe(nil, nil))
	}
	if input.Mode == AuditModeStrict || input.EnableModelEquality {
		scores = append(scores, RunModelEqualityProbe(nil, nil))
	}
	if input.Mode == AuditModeStrict || input.Mode == AuditModeScheduled {
		scores = append(scores, RunLogprobTrackingProbe(adapter))
		scores = append(scores, RunBorderInputChangeProbe(nil, nil))
	}

	for _, sample := range samples {
		_, _ = s.createSample(ctx, sample)
	}
	for _, score := range scores {
		_ = s.createProbeScore(ctx, run.ID, score)
	}
	aggregated := DefaultRiskAggregator().Aggregate(scores)
	now := s.now().UTC()
	report := BuildAuditReport(AuditReportInput{
		RunID:             run.ID,
		TargetID:          run.TargetID,
		TargetName:        run.TargetName,
		ClaimedModel:      run.ClaimedModel,
		BaselineID:        run.BaselineID,
		Mode:              run.Mode,
		OverallRisk:       aggregated.Risk,
		OverallScore:      aggregated.Score,
		OverallConfidence: aggregated.Confidence,
		ProbeScores:       scores,
		CreatedAt:         run.CreatedAt,
	})
	run.Status = RunStatusCompleted
	run.RiskLevel = aggregated.Risk
	run.Confidence = aggregated.Confidence
	run.OverallScore = aggregated.Score
	run.ProbeScores = scores
	run.ScoreJSON = map[string]any{"overallRiskScore": aggregated.Score, "confidence": aggregated.Confidence}
	run.ReportJSON = report.JSONMap()
	run.ReportMarkdown = report.Markdown
	run.FinishedAt = &now
	return run
}

func (s *Service) runRandomProbe(ctx context.Context, run Run, target Target, baseline *Baseline, adapter ProviderAdapter, input RunInput) (ProbeScore, []Sample) {
	baselineFeatures, hasBaselineFeatures := randomFingerprintBaselineFeatures(baseline)
	prompt := selectRandomFingerprintPrompt(baselineFeatures)
	sampleTarget := 1
	if hasBaselineFeatures {
		sampleTarget = randomFingerprintMinSamples
	}
	counts := map[string]int{}
	invalidCount := 0
	samples := make([]Sample, 0, sampleTarget)
	request := AuditChatRequest{
		Model: run.ClaimedModel,
		Messages: []AuditChatMessage{{
			Role:    "user",
			Content: prompt.Instruction,
		}},
		Temperature: 0.7,
		TopP:        1,
		MaxTokens:   8,
		Timeout:     15 * time.Second,
	}
	for i := 0; i < sampleTarget; i++ {
		response, err := adapter.Chat(ctx, request)
		sample := Sample{
			ID:         uuid.NewString(),
			RunID:      run.ID,
			TargetID:   target.ID,
			ProbeType:  ProbeRandomFingerprint,
			PromptID:   prompt.ID,
			PromptHash: hashText(prompt.Instruction),
			RequestParamsJSON: map[string]any{
				"model":       request.Model,
				"temperature": request.Temperature,
				"top_p":       request.TopP,
				"max_tokens":  request.MaxTokens,
			},
			CreatedAt: s.now().UTC(),
		}
		if input.StorePromptText {
			sample.PromptText = prompt.Instruction
		}
		if err != nil {
			sample.ErrorMessage = err.Error()
			samples = append(samples, sample)
			return ProbeScore{
				Probe:      ProbeRandomFingerprint,
				Risk:       RiskInsufficientData,
				Confidence: 0.1,
				Score:      0,
				Evidence:   map[string]any{"error": "target_call_failed", "sample_count": len(samples), "baseline_id": run.BaselineID, "claimed_model": run.ClaimedModel},
			}, samples
		}
		parsed := strings.TrimSpace(response.Text)
		sample.ParsedValue = parsed
		sample.ResponseHash = hashText(response.Text)
		sample.LatencyMS = response.LatencyMS
		sample.FirstTokenLatencyMS = response.FirstTokenLatencyMS
		if response.Usage != nil {
			sample.UsagePromptTokens = response.Usage.PromptTokens
			sample.UsageCompletionTokens = response.Usage.CompletionTokens
		}
		if input.StoreResponseText {
			sample.ResponseText = response.Text
		}
		if !randomPromptAllowsValue(prompt, parsed) {
			invalidCount++
		}
		counts[parsed]++
		samples = append(samples, sample)
	}
	targetFeatures := RandomFingerprintFeatureSet{Categorical: []CategoricalFingerprintFeature{{
		PromptID:    prompt.ID,
		N:           len(samples),
		Counts:      counts,
		Values:      prompt.AllowedValues,
		InvalidRate: float64(invalidCount) / float64(len(samples)),
	}}}
	if !hasBaselineFeatures {
		return ProbeScore{
			Probe:      ProbeRandomFingerprint,
			Risk:       RiskInsufficientData,
			Confidence: clamp01(float64(len(samples)) / randomFingerprintMinSamples),
			Score:      0,
			Evidence: map[string]any{
				"reason":        "missing_random_fingerprint_baseline",
				"sample_count":  len(samples),
				"baseline_id":   run.BaselineID,
				"claimed_model": run.ClaimedModel,
			},
		}, samples
	}
	score := ScoreRandomFingerprint(baselineFeatures, targetFeatures)
	if score.Evidence == nil {
		score.Evidence = map[string]any{}
	}
	score.Evidence["baseline_id"] = run.BaselineID
	score.Evidence["claimed_model"] = run.ClaimedModel
	return score, samples
}

func (s *Service) runBillingLatencyProbe(ctx context.Context, run Run, adapter ProviderAdapter, samples []Sample) ProbeScore {
	evidence := map[string]any{"claimed_model": run.ClaimedModel}
	if lister, ok := adapter.(interface {
		ListModels(context.Context) ([]string, error)
	}); ok {
		models, err := lister.ListModels(ctx)
		if err == nil {
			contains := false
			for _, model := range models {
				if model == run.ClaimedModel {
					contains = true
					break
				}
			}
			evidence["models_contains_claimed"] = contains
			if !contains && len(models) > 0 {
				return ProbeScore{Probe: ProbeBillingLatency, Risk: RiskSuspicious, Confidence: 0.5, Score: 0.35, Evidence: evidence}
			}
		}
	}
	latencies := []float64{}
	for _, sample := range samples {
		if sample.LatencyMS > 0 {
			latencies = append(latencies, float64(sample.LatencyMS))
		}
	}
	sort.Float64s(latencies)
	if len(latencies) > 0 {
		evidence["latency_p50_ms"] = percentile(latencies, 0.50)
		evidence["latency_p95_ms"] = percentile(latencies, 0.95)
	}
	return ProbeScore{Probe: ProbeBillingLatency, Risk: RiskConsistent, Confidence: 0.4, Score: 0.05, Evidence: evidence}
}

func (s *Service) buildTarget(current Target, input TargetInput, requireAPIKey bool) (Target, *domain.AppError) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Target{}, validation("name", "required", "必须填写审计目标名称。")
	}
	baseURL := strings.TrimSpace(input.BaseURL)
	parsedURL, err := url.Parse(baseURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return Target{}, validation("baseUrl", "invalid", "必须填写有效 API Base URL。")
	}
	claimedModel := strings.TrimSpace(input.ClaimedModel)
	if claimedModel == "" {
		return Target{}, validation("claimedModel", "required", "必须填写声称模型。")
	}
	if requireAPIKey && strings.TrimSpace(input.APIKey) == "" {
		return Target{}, validation("apiKey", "required", "必须填写 API Key。")
	}
	now := s.now().UTC()
	target := current
	if target.ID == "" {
		target.ID = uuid.NewString()
		target.CreatedAt = now
	}
	target.Name = name
	target.BaseURL = strings.TrimRight(baseURL, "/")
	target.ProviderType = strings.TrimSpace(input.ProviderType)
	if target.ProviderType == "" {
		target.ProviderType = "openai_compatible"
	}
	target.ClaimedModel = claimedModel
	target.Enabled = input.Enabled
	target.APIServiceID = strings.TrimSpace(input.APIServiceID)
	target.APIServiceModelID = strings.TrimSpace(input.APIServiceModelID)
	target.UpdatedAt = now
	return target, nil
}

func (s *Service) buildBaseline(input BaselineInput) (Baseline, *domain.AppError) {
	if strings.TrimSpace(input.BaselineName) == "" {
		return Baseline{}, validation("baselineName", "required", "必须填写基线名称。")
	}
	if strings.TrimSpace(input.Model) == "" {
		return Baseline{}, validation("model", "required", "必须填写模型。")
	}
	if strings.TrimSpace(input.ProbeSetVersion) == "" {
		return Baseline{}, validation("probeSetVersion", "required", "必须填写探针版本。")
	}
	if input.ParamsJSON == nil {
		input.ParamsJSON = map[string]any{}
	}
	if input.FeatureJSON == nil {
		input.FeatureJSON = map[string]any{}
	}
	now := s.now().UTC()
	return Baseline{
		ID:              uuid.NewString(),
		BaselineName:    strings.TrimSpace(input.BaselineName),
		SourceTargetID:  strings.TrimSpace(input.SourceTargetID),
		Model:           strings.TrimSpace(input.Model),
		SourceType:      strings.TrimSpace(input.SourceType),
		ProbeSetVersion: strings.TrimSpace(input.ProbeSetVersion),
		ParamsJSON:      input.ParamsJSON,
		FeatureJSON:     input.FeatureJSON,
		SampleCount:     input.SampleCount,
		ValidFrom:       now,
		CreatedAt:       now,
	}, nil
}

func (s *Service) targetWithSecret(ctx context.Context, targetID string) (Target, string, *domain.AppError) {
	if strings.TrimSpace(targetID) == "" {
		return Target{}, "", validation("targetId", "required", "必须选择审计目标。")
	}
	if s.repo != nil {
		return s.repo.GetModelAuditTargetSecret(ctx, targetID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	target, ok := s.targets[targetID]
	if !ok {
		return Target{}, "", notFound("审计目标不存在。")
	}
	return target, s.targetSecrets[targetID], nil
}

func (s *Service) getBaseline(ctx context.Context, baselineID string) (Baseline, *domain.AppError) {
	if strings.TrimSpace(baselineID) == "" {
		return Baseline{}, validation("baselineId", "required", "必须选择基线。")
	}
	if s.repo != nil {
		return s.repo.GetModelAuditBaseline(ctx, baselineID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	baseline, ok := s.baselines[baselineID]
	if !ok {
		return Baseline{}, notFound("基线不存在。")
	}
	return baseline, nil
}

func (s *Service) createRun(ctx context.Context, run Run) (Run, *domain.AppError) {
	if s.repo != nil {
		return s.repo.CreateModelAuditRun(ctx, run)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[run.ID] = run
	s.runOrder = append(s.runOrder, run.ID)
	return run, nil
}

func (s *Service) getRun(ctx context.Context, runID string) (Run, *domain.AppError) {
	if s.repo != nil {
		return s.repo.GetModelAuditRun(ctx, runID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[runID]
	if !ok {
		return Run{}, notFound("审计运行不存在。")
	}
	return run, nil
}

func (s *Service) updateRun(ctx context.Context, run Run) (Run, *domain.AppError) {
	if s.repo != nil {
		return s.repo.UpdateModelAuditRun(ctx, run)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[run.ID] = run
	return run, nil
}

func (s *Service) createSample(ctx context.Context, sample Sample) (Sample, *domain.AppError) {
	if s.repo != nil {
		return s.repo.CreateModelAuditSample(ctx, sample)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.samples[sample.RunID] = append(s.samples[sample.RunID], sample)
	return sample, nil
}

func (s *Service) createProbeScore(ctx context.Context, runID string, score ProbeScore) *domain.AppError {
	if s.repo != nil {
		return s.repo.CreateModelAuditProbeScore(ctx, runID, score)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.probeScores[runID] = append(s.probeScores[runID], score)
	return nil
}

func (s *Service) listProbeScores(ctx context.Context, runID string) ([]ProbeScore, *domain.AppError) {
	if s.repo != nil {
		return s.repo.ListModelAuditProbeScores(ctx, runID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]ProbeScore{}, s.probeScores[runID]...), nil
}

func requireAdmin(user auth.User) *domain.AppError {
	if !user.IsAdmin {
		return domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	return nil
}

func validation(field, code, message string) *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Validation failed", "请求字段不合法。", field, code, message)
}

func notFound(detail string) *domain.AppError {
	return domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Object not found", detail)
}

func validAuditMode(mode AuditMode) bool {
	switch mode {
	case AuditModeQuick, AuditModeStandard, AuditModeStrict, AuditModeScheduled:
		return true
	default:
		return false
	}
}

func randomFingerprintBaselineFeatures(baseline *Baseline) (RandomFingerprintFeatureSet, bool) {
	if baseline == nil {
		return RandomFingerprintFeatureSet{}, false
	}
	return DecodeRandomFingerprintFeatureSet(baseline.FeatureJSON)
}

func selectRandomFingerprintPrompt(baselineFeatures RandomFingerprintFeatureSet) RandomPrompt {
	prompts := RandomPromptBank()
	if len(prompts) == 0 {
		return RandomPrompt{}
	}
	byID := map[string]RandomPrompt{}
	for _, prompt := range prompts {
		byID[prompt.ID] = prompt
	}
	for _, feature := range baselineFeatures.Categorical {
		if prompt, ok := byID[feature.PromptID]; ok {
			return prompt
		}
	}
	for _, feature := range baselineFeatures.Binary {
		if prompt, ok := byID[feature.PromptID]; ok {
			return prompt
		}
	}
	return prompts[0]
}

func randomPromptAllowsValue(prompt RandomPrompt, value string) bool {
	for _, allowed := range prompt.AllowedValues {
		if value == allowed {
			return true
		}
	}
	return false
}

func hashText(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func percentile(values []float64, p float64) float64 {
	if len(values) == 0 {
		return 0
	}
	index := int(float64(len(values)-1) * p)
	if index < 0 {
		index = 0
	}
	if index >= len(values) {
		index = len(values) - 1
	}
	return values[index]
}

func MaskAPIKey(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) <= 8 {
		return "********"
	}
	return fmt.Sprintf("%s...%s", trimmed[:4], trimmed[len(trimmed)-4:])
}
