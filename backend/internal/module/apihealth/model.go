package apihealth

import "time"

const (
	VerificationUnverified = "unverified"
	VerificationVerified   = "verified"
	VerificationFailed     = "failed"

	SampleStatusRunning   = "running"
	SampleStatusSucceeded = "succeeded"
	SampleStatusFailed    = "failed"

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

type ServiceReference struct {
	ID    string
	Title string
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
	Enabled                 bool
	AcknowledgeInsecureHTTP bool
}

type ProbeJob struct {
	Sample          Sample
	Connection      Connection
	Credential      string
	CredentialError bool
}

type ProbeResult struct {
	TotalDurationMS int
	HTTPStatusClass int
	ErrorCode       string
}

type SummaryInput struct {
	Connection *Connection
	Samples    []Sample
}

type Sample struct {
	ID                 string
	ConnectionID       string
	MeasurementVersion int64
	SlotStartedAt      time.Time
	Status             string
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
	TransportSecurity  string
	SuccessRatePercent *string
	SuccessfulSamples  int
	TotalSamples       int
	LastSampledAt      *time.Time
	Samples            []HealthSlot
}

func TemporarilyUnavailableSummary(now time.Time) Summary {
	return noSampleSummary(AvailabilityTemporarilyUnavailable, TransportSecurityUnknown, now)
}
