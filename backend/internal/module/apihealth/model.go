package apihealth

import "time"

const (
	ProtocolOpenAIChatCompletionsV1 = "openai_chat_completions_v1"

	AuthorizationPending  = "pending"
	AuthorizationVerified = "verified"
	AuthorizationApproved = "approved"
	AuthorizationRejected = "rejected"

	AuthorizationMethodDNSTXT        = "dns_txt"
	AuthorizationMethodHTTPChallenge = "http_challenge"
	AuthorizationMethodAdminApproval = "admin_approval"

	AuthorizationActionChallengeCreated      = "challenge_created"
	AuthorizationActionVerificationSucceeded = "verification_succeeded"
	AuthorizationActionVerificationFailed    = "verification_failed"
	AuthorizationActionAdminApproved         = "admin_approved"
	AuthorizationActionAdminRejected         = "admin_rejected"
	AuthorizationActionOriginInvalidated     = "origin_invalidated"
	AuthorizationActionConfigDeleted         = "config_deleted"
	AuthorizationReasonMeasurementChanged    = "measurement_identity_changed"

	SampleStatusRunning   = "running"
	SampleStatusSucceeded = "succeeded"
	SampleStatusFailed    = "failed"

	HealthStateNormal      = "normal"
	HealthStateFluctuating = "fluctuating"
	HealthStateAbnormal    = "abnormal"
	HealthStateNoSample    = "no_sample"

	AvailabilityUnconfigured           = "unconfigured"
	AvailabilityDisabled               = "disabled"
	AvailabilityUnauthorized           = "unauthorized"
	AvailabilityInsufficient           = "insufficient"
	AvailabilityStale                  = "stale"
	AvailabilityTemporarilyUnavailable = "temporarily_unavailable"

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
	ErrorResponseTooLarge     = "response_too_large"
	ErrorInvalidStream        = "invalid_stream"
	ErrorEmptyResponse        = "empty_response"
	ErrorDecryptFailed        = "decrypt_failed"
	ErrorInternal             = "internal"
	ErrorInternalTimeout      = "internal_timeout"
)

const (
	ProbeSlotDuration   = 5 * time.Minute
	SummaryWindow       = time.Hour
	SummarySlotCount    = 12
	MinimumFinalSamples = 3
	SummaryStaleAfter   = 10 * time.Minute
)

type Config struct {
	ID                   string
	APIServiceID         string
	OwnerUserID          string
	ServiceTitle         string
	OwnerUsername        string
	OwnerDisplayName     string
	Protocol             string
	BaseURL              string
	NormalizedOrigin     string
	Model                string
	CredentialConfigured bool
	Enabled              bool
	AuthorizationStatus  string
	AuthorizationMethod  string
	VerifiedOrigin       string
	VerifiedAt           *time.Time
	ApprovedByAdminID    string
	ApprovedAt           *time.Time
	RejectionReason      string
	ChallengeExpiresAt   *time.Time
	MeasurementVersion   int64
	LastConfigErrorCode  string
	Version              int64
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type ConfigInput struct {
	BaseURL    string
	Model      string
	Credential *string
	Enabled    bool
}

type Challenge struct {
	Token         string
	Method        string
	DNSRecordName string
	HTTPURL       string
	ExpiresAt     time.Time
	ConfigVersion int64
}

type StoredChallenge struct {
	Config    Config
	Method    string
	TokenHash []byte
	ExpiresAt time.Time
}

type AuthorizationEvent struct {
	ID             string
	ProbeConfigID  string
	APIServiceID   string
	ActorUserID    string
	Action         string
	Method         string
	OriginSnapshot string
	Reason         string
	CreatedAt      time.Time
}

type ProbeJob struct {
	Sample          Sample
	Config          Config
	Credential      string
	CredentialError bool
}

type ProbeResult struct {
	TTFTMS          int
	TotalDurationMS int
	HTTPStatusClass int
	ErrorCode       string
}

type SummaryInput struct {
	Config  *Config
	Samples []Sample
}

type Sample struct {
	ID                 string
	APIServiceID       string
	ProbeConfigID      string
	MeasurementVersion int64
	ProbeModelSnapshot string
	SlotStartedAt      time.Time
	Status             string
	TTFTMS             *int
	TotalDurationMS    *int
	HTTPStatusClass    *int
	ErrorCode          string
	StartedAt          time.Time
	FinishedAt         *time.Time
	CreatedAt          time.Time
}

type HealthSlot struct {
	SlotStartedAt time.Time
	State         string
}

type Summary struct {
	State              string
	AvailabilityReason string
	SuccessRatePercent *string
	SuccessfulSamples  int
	TotalSamples       int
	MedianTTFTMS       *int
	ProbeModel         *string
	LastSampledAt      *time.Time
	Samples            []HealthSlot
}

func TemporarilyUnavailableSummary(now time.Time) Summary {
	return noSampleSummary(AvailabilityTemporarilyUnavailable, nil, now)
}
