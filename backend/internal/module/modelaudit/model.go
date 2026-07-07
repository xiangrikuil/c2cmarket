package modelaudit

import "time"

const (
	RiskConsistent       RiskLevel = "consistent"
	RiskSuspicious      RiskLevel = "suspicious"
	RiskHigh            RiskLevel = "high_risk"
	RiskInsufficientData RiskLevel = "insufficient_data"
	RiskNotApplicable   RiskLevel = "not_applicable"

	AuditModeQuick     AuditMode = "quick"
	AuditModeStandard  AuditMode = "standard"
	AuditModeStrict    AuditMode = "strict"
	AuditModeScheduled AuditMode = "scheduled"

	RunStatusQueued    RunStatus = "queued"
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCancelled RunStatus = "cancelled"

	ProbeRandomFingerprint ProbeName = "random_fingerprint"
	ProbeBillingLatency    ProbeName = "billing_latency_protocol"
	ProbeActiveFingerprint ProbeName = "llmmap_active"
	ProbeKBF               ProbeName = "kbf_knowledge_boundary"
	ProbeModelEquality     ProbeName = "model_equality"
	ProbeLogprobTracking   ProbeName = "logprob_tracking"
	ProbeBorderInputChange ProbeName = "border_input_change"
)

type RiskLevel string

type AuditMode string

type RunStatus string

type ProbeName string

type Target struct {
	ID                string
	Name              string
	BaseURL           string
	ProviderType      string
	ClaimedModel      string
	Enabled           bool
	APIServiceID      string
	APIServiceModelID string
	LastRiskLevel     RiskLevel
	LastRunID         string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type TargetInput struct {
	Name              string
	BaseURL           string
	ProviderType      string
	ClaimedModel      string
	APIKey            string
	Enabled           bool
	APIServiceID      string
	APIServiceModelID string
}

type Baseline struct {
	ID              string
	BaselineName    string
	SourceTargetID  string
	Model           string
	SourceType      string
	ProbeSetVersion string
	ParamsJSON      map[string]any
	FeatureJSON     map[string]any
	SampleCount     int
	ValidFrom       time.Time
	ValidTo         *time.Time
	CreatedAt       time.Time
}

type BaselineInput struct {
	BaselineName    string
	SourceTargetID  string
	Model           string
	SourceType      string
	ProbeSetVersion string
	ParamsJSON      map[string]any
	FeatureJSON     map[string]any
	SampleCount     int
}

type Run struct {
	ID             string
	TargetID       string
	TargetName     string
	ClaimedModel   string
	BaselineID      string
	Status         RunStatus
	Mode           AuditMode
	RiskLevel      RiskLevel
	Confidence     float64
	OverallScore   float64
	ScoreJSON      map[string]any
	ReportJSON     map[string]any
	ReportMarkdown string
	ErrorMessage   string
	ProbeScores    []ProbeScore
	StartedAt      *time.Time
	FinishedAt     *time.Time
	CreatedAt      time.Time
}

type RunInput struct {
	TargetID              string
	BaselineID            string
	ClaimedModel          string
	Mode                  AuditMode
	EnableModelEquality   bool
	EnableLogprobs        string
	StorePromptText       bool
	StoreResponseText     bool
	ScheduledMonitorID    string
}

type Sample struct {
	ID                       string
	RunID                    string
	TargetID                 string
	ProbeType                ProbeName
	PromptID                 string
	PromptHash               string
	PromptText               string
	ResponseText             string
	ResponseHash             string
	ParsedValue              string
	RawJSON                  map[string]any
	RequestParamsJSON         map[string]any
	LatencyMS                int
	FirstTokenLatencyMS       int
	UsagePromptTokens         int
	UsageCompletionTokens     int
	EstimatedPromptTokens     int
	EstimatedCompletionTokens int
	ErrorMessage              string
	SessionID                 string
	CreatedAt                time.Time
}

type ProbeScore struct {
	Probe      ProbeName
	Risk       RiskLevel
	Confidence float64
	Score      float64
	Evidence   map[string]any
}

type AggregatedRisk struct {
	Risk       RiskLevel
	Score      float64
	Confidence float64
}

type Monitor struct {
	ID           string
	TargetID     string
	BaselineID   string
	Mode         AuditMode
	Enabled      bool
	CronSpec     string
	LastRunID    string
	LastRisk     RiskLevel
	LastRunAt    *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type MonitorInput struct {
	TargetID   string
	BaselineID string
	Mode       AuditMode
	Enabled    bool
	CronSpec   string
}
