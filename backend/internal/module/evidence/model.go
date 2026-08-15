package evidence

import (
	"io"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

const (
	MaxFileBytes        int64 = 5 * 1024 * 1024
	MaxDimension              = 4096
	MaxFilesPerUpload         = 3
	MaxAssetsPerCase          = 20
	QuarantineRetention       = 7 * 24 * time.Hour

	KindPaymentResult       = "payment_result"
	KindRefundResult        = "refund_result"
	KindAPIError            = "api_error"
	KindQuotaInsufficient   = "quota_insufficient"
	KindExpiredEarly        = "expired_early"
	KindDescriptionMismatch = "description_mismatch"
	KindOtherRedactedFact   = "other_redacted_fact"

	VisibilityParticipantsAdmin = "participants_admin"
	VisibilitySubmitterAdmin    = "submitter_admin"
	VisibilityAppellantAdmin    = "appellant_admin"

	UsageDisputeInitial     = "dispute_initial"
	UsagePlatformEscalation = "platform_escalation"
	UsageMessage            = "message"
	UsageInfoSupplement     = "info_supplement"
	UsageRemedyClaim        = "remedy_claim"
	UsageRemedyContest      = "remedy_contest"
	UsageAppeal             = "appeal"

	SourceDisputeCase    = "dispute_case"
	SourceDisputeMessage = "dispute_message"
	SourceInfoSupplement = "info_supplement"
	SourceDisputeRemedy  = "dispute_remedy"
	SourceAppeal         = "appeal"
)

var allowedKinds = map[string]struct{}{
	KindPaymentResult: {}, KindRefundResult: {}, KindAPIError: {},
	KindQuotaInsufficient: {}, KindExpiredEarly: {},
	KindDescriptionMismatch: {}, KindOtherRedactedFact: {},
}

type Asset struct {
	ID               string
	APIOrderID       string
	UploaderUserID   string
	Kind             string
	ObjectKey        string
	OutputMIME       string
	ByteSize         int64
	Width            int
	Height           int
	SHA256           [32]byte
	Status           string
	CreatedAt        time.Time
	ReadyAt          *time.Time
	UnboundExpiresAt *time.Time
	DestroyedAt      *time.Time
	DestroyReason    string
	Version          int64
}

type PublicAsset struct {
	ID          string    `json:"id"`
	Kind        string    `json:"kind"`
	MIME        string    `json:"mime"`
	ByteSize    int64     `json:"byteSize"`
	Width       int       `json:"width"`
	Height      int       `json:"height"`
	CreatedAt   time.Time `json:"createdAt"`
	ContentPath string    `json:"contentPath"`
	Version     int64     `json:"version"`
}

type Reference struct {
	PublicAsset
	UploaderUserID string `json:"-"`
	Visibility     string `json:"visibility"`
	Usage          string `json:"usage"`
	SourceType     string `json:"sourceType"`
	SourceID       string `json:"sourceId"`
}

type UploadInput struct {
	APIOrderID         string
	UploaderUserID     string
	Kind               string
	Files              []io.Reader
	RedactionConfirmed bool
}

type AdminQuarantineInput struct {
	AssetID         string
	AdminUserID     string
	ExpectedVersion int64
	Reason          string
	RequestID       string
}

type AdminQuarantineResult struct {
	ID                   string    `json:"id"`
	Status               string    `json:"status"`
	QuarantinedExpiresAt time.Time `json:"quarantinedExpiresAt"`
	Version              int64     `json:"version"`
}

type AdminQuarantineCompletionBuilder func(AdminQuarantineResult) (idempotency.Completion, *domain.AppError)

type BindingInput struct {
	AssetIDs      []string
	APIOrderID    string
	DisputeCaseID string
	UploaderID    string
	Visibility    string
	Usage         string
	SourceType    string
	SourceID      string
}

type DestroyCandidate struct {
	ID        string
	ObjectKey string
}

type CleanupResult struct {
	Claimed   int
	Destroyed int
	Failed    int
}

type ProcessedImage struct {
	Bytes  []byte
	MIME   string
	Width  int
	Height int
	SHA256 [32]byte
}

type Object struct {
	Body        io.ReadCloser
	ContentType string
	Size        int64
}

func IsAllowedKind(value string) bool {
	_, ok := allowedKinds[value]
	return ok
}
