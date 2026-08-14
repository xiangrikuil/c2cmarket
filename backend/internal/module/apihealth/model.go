package apihealth

import (
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

const (
	VerificationUnverified = "unverified"
	VerificationVerified   = "verified"
	VerificationFailed     = "failed"

	SampleStatusRunning   = "running"
	SampleStatusSucceeded = "succeeded"
	SampleStatusFailed    = "failed"

	OutcomeFirstSuccess     = "first_success"
	OutcomeFirstSuccessSlow = "first_success_slow"
	OutcomeRetryRecovered   = "retry_recovered"
	OutcomeFinalFailure     = "final_failure"

	ProtocolResponsesV1       = "openai_responses_v1"
	ProtocolChatCompletionsV1 = "openai_chat_completions_v1"
	ProbeEnvironmentUSWestV1  = "us-west-v1"
	DefaultGPTProbeModel      = "gpt-5.6-luna"

	HealthStateNormal      = "normal"
	HealthStateFluctuating = "fluctuating"
	HealthStateAbnormal    = "abnormal"
	HealthStateNoSample    = "no_sample"

	AvailabilityUnconfigured           = "unconfigured"
	AvailabilityDisabled               = "disabled"
	AvailabilityUnverified             = "unverified"
	AvailabilityInsufficient           = "insufficient"
	AvailabilityStale                  = "stale"
	AvailabilityTemporarilyUnavailable = "temporarily_unavailable"
	AvailabilityRunnerDisabled         = "runner_disabled"

	TransportSecurityHTTPS   = "secure_https"
	TransportSecurityHTTP    = "insecure_http"
	TransportSecurityUnknown = "unknown"

	SlotStateSmooth      = "smooth"
	SlotStateFluctuating = "fluctuating"
	SlotStateAbnormal    = "abnormal"
	SlotStateNoSample    = "no_sample"

	ErrorBlockedTarget        = "blocked_target"
	ErrorAuthorizationInvalid = "authorization_invalid"
	ErrorDNSFailed            = "dns_failed"
	ErrorConnectFailed        = "connect_failed"
	ErrorTLSFailed            = "tls_failed"
	ErrorTimeout              = "timeout"
	ErrorHTTP4xx              = "http_4xx"
	ErrorHTTP5xx              = "http_5xx"
	ErrorRateLimited          = "rate_limited"
	ErrorResponseTooLarge     = "response_too_large"
	ErrorInvalidResponse      = "invalid_response"
	ErrorStreamInterrupted    = "stream_interrupted"
	ErrorModelUnavailable     = "model_unavailable"
	ErrorProtocolUnavailable  = "protocol_unavailable"
	ErrorDecryptFailed        = "decrypt_failed"
	ErrorInternal             = "internal"
	ErrorInternalTimeout      = "internal_timeout"
)

const (
	ProbeAuditCreated         = "created"
	ProbeAuditUpdated         = "updated"
	ProbeAuditModelChanged    = "model_changed"
	ProbeAuditVerifySucceeded = "verify_succeeded"
	ProbeAuditVerifyFailed    = "verify_failed"
	ProbeAuditEnabled         = "enabled"
	ProbeAuditDisabled        = "disabled"
	ProbeAuditDeleted         = "deleted"
)

const (
	ProbeSlotDuration         = 5 * time.Minute
	SummaryWindow             = 24 * time.Hour
	SummarySlotCount          = 24
	SummaryTheoreticalSamples = 288
	MinimumFinalSamples       = 3
	SummaryStaleAfter         = 10 * time.Minute
	ModelChangeNoticeDuration = 24 * time.Hour
	ProbeInputTokenUpperBound = 8
	ProbeOutputTokenLimit     = 32
)

type ServiceReference struct {
	ID    string
	Title string
}

type PriceSnapshot struct {
	VersionID                  string
	InputPricePerMillion       string
	CachedInputPricePerMillion string
	OutputPricePerMillion      string
	Currency                   string
}

type Connection struct {
	ID                        string
	OwnerUserID               string
	Name                      string
	BaseURL                   string
	NormalizedBaseURL         string
	CredentialConfigured      bool
	Enabled                   bool
	VerificationStatus        string
	VerifiedAt                *time.Time
	LastVerificationErrorCode string
	ProbeModel                string
	ProbeProtocol             string
	AvailableModels           []string
	ProbeEnvironment          string
	ProbeModelChangedAt       *time.Time
	Price                     PriceSnapshot
	MeasurementVersion        int64
	Version                   int64
	References                []ServiceReference
	HealthSummary             Summary
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
}

type ConnectionInput struct {
	Name                    string
	BaseURL                 string
	Credential              *string
	ProbeModel              string
	PreflightToken          string
	Enabled                 bool
	AcknowledgeInsecureHTTP bool
}

// ProbeAuditMutation 只承载安全审计上下文，不包含探针地址或凭据。
type ProbeAuditMutation struct {
	Action     string
	RequestID  string
	OccurredAt time.Time
}

// MutationCompletionBuilder 在数据库事务内根据最终连接状态生成可重放响应。
type MutationCompletionBuilder func(Connection) (idempotency.Completion, *domain.AppError)

type VerificationResult struct {
	TotalDurationMS int
	HTTPStatus      int
	ErrorCode       string
	AvailableModels []string
	ProbeModel      string
	ProbeProtocol   string
	Attempt         ProbeAttempt
}

type ProbeJob struct {
	Sample          Sample
	Connection      Connection
	Credential      string
	CredentialError bool
	LatencyRule     *LatencyRule
}

type TokenUsage struct {
	InputTokens       *int64
	CachedInputTokens *int64
	OutputTokens      *int64
	ReasoningTokens   *int64
}

func (usage TokenUsage) Complete() bool {
	return usage.InputTokens != nil && usage.OutputTokens != nil
}

type ProbeAttempt struct {
	AttemptNumber   int
	StartedAt       time.Time
	FirstTextAt     *time.Time
	FinishedAt      time.Time
	HTTPStatus      int
	TTFTMS          *int
	TotalDurationMS int
	Succeeded       bool
	Retryable       bool
	ErrorCode       string
	RetryAfterMS    int
	Usage           TokenUsage
	CostUSD         string
}

type ProbeResult struct {
	Outcome                     string
	Attempts                    []ProbeAttempt
	TotalDurationMS             int
	HTTPStatus                  int
	HTTPStatusClass             int
	ErrorCode                   string
	FirstAttemptTTFTMS          *int
	FirstAttemptTotalDurationMS *int
	RecoveryDurationMS          *int
	Usage                       TokenUsage
	UsageComplete               bool
	BaseCostUSD                 string
	RetryCostUSD                string
}

type SummaryInput struct {
	Connection *Connection
	Samples    []Sample
}

type RunnerStatus struct {
	Enabled              bool
	LastSuccessfulScanAt time.Time
	ScanInterval         time.Duration
}

type RunnerStatusProvider interface {
	ProbeRunnerStatus() RunnerStatus
}

type Sample struct {
	ID                          string
	ConnectionID                string
	MeasurementVersion          int64
	SlotStartedAt               time.Time
	Status                      string
	ProbeModel                  string
	ProbeProtocol               string
	ProbeEnvironment            string
	LatencyRuleVersionID        string
	Outcome                     string
	AttemptCount                int
	FirstAttemptTTFTMS          *int
	FirstAttemptTotalDurationMS *int
	RecoveryDurationMS          *int
	TotalDurationMS             *int
	HTTPStatusClass             *int
	FinalHTTPStatus             *int
	ErrorCode                   string
	Usage                       TokenUsage
	UsageComplete               bool
	BaseCostUSD                 string
	RetryCostUSD                string
	StartedAt                   time.Time
	FinishedAt                  *time.Time
	CreatedAt                   time.Time
}

type HourlyBucket struct {
	HourStartedAt         time.Time
	State                 string
	CompletedCycles       int
	FirstAttemptSuccesses int
	RetryRecoveries       int
	FinalFailures         int
	SlowSuccesses         int
	FinalSuccessPercent   *string
	AverageTTFTMS         *int
}

type CostSummary struct {
	KnownBaseCostUSD      string
	KnownRetryCostUSD     string
	ProjectedDailyCostUSD string
	HasUnknownUsage       bool
	KnownUsageSamples     int
}

type Summary struct {
	State                 string
	AvailabilityReason    string
	TransportSecurity     string
	StabilityPercent      *string
	FinalSuccessPercent   *string
	CoveragePercent       string
	CompletedCycles       int
	TheoreticalSlots      int
	FirstAttemptSuccesses int
	RetryRecoveries       int
	FinalFailures         int
	AverageTTFTMS         *int
	P50TTFTMS             *int
	P95TTFTMS             *int
	LastSampledAt         *time.Time
	ProbeModel            string
	ProbeProtocol         string
	ProbeEnvironment      string
	ProbeModelChangedAt   *time.Time
	AccumulatingSamples   bool
	HourlyBuckets         []HourlyBucket
	Cost                  CostSummary
	// Legacy aliases remain during the API response transition.
	SuccessRatePercent *string
	SuccessfulSamples  int
	TotalSamples       int
	Samples            []HealthSlot
}

type HealthSlot struct {
	SlotStartedAt time.Time
	State         string
}

type LatencyRule struct {
	ID                   string
	Model                string
	Protocol             string
	Environment          string
	Version              int64
	SlowTTFTMS           int
	HardTimeoutMS        int
	ObservationStartedAt time.Time
	ObservationEndedAt   time.Time
	CompleteCalendarDays int
	ConnectionCount      int
	SampleCount          int64
	P50TTFTMS            *int
	P90TTFTMS            *int
	P95TTFTMS            *int
	P99TTFTMS            *int
	Status               string
	PublishedByAdminID   string
	PublishedAt          time.Time
	SupersededAt         *time.Time
}

type Calibration struct {
	Model                string
	Protocol             string
	Environment          string
	ObservationStartedAt time.Time
	ObservationEndedAt   time.Time
	CompleteCalendarDays int
	ConnectionCount      int
	SampleCount          int64
	P50TTFTMS            *int
	P90TTFTMS            *int
	P95TTFTMS            *int
	P99TTFTMS            *int
	Ready                bool
}

type LatencyRulePreview struct {
	Calibration        Calibration
	SlowTTFTMS         int
	HardTimeoutMS      int
	SlowSampleCount    int64
	SlowPercent        string
	OverTimeoutCount   int64
	OverTimeoutPercent string
}

func TemporarilyUnavailableSummary(now time.Time) Summary {
	return noSampleSummary(AvailabilityTemporarilyUnavailable, TransportSecurityUnknown, now)
}
