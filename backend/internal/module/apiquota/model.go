package apiquota

import (
	"io"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/idempotency"
)

const (
	BatchStatusDraft     = "draft"
	BatchStatusPublished = "published"
	BatchStatusPaused    = "paused"
	BatchStatusArchived  = "archived"

	OfferStatusDraft     = "draft"
	OfferStatusPublished = "published"
	OfferStatusPaused    = "paused"
	OfferStatusArchived  = "archived"

	SaleModeContinuous = "continuous"
	SaleModeScheduled  = "scheduled"

	DeliveryModeManual      = "manual"
	DeliveryModePreimported = "preimported"

	SourceTypeSub2API     = "sub2api"
	SourceTypeNewAPIProxy = "new_api_proxy"
	SourceTypeSelfHosted  = "self_hosted"
	SourceTypeOther       = "other"

	DistributionSub2API = "sub2api"

	RoundStatusScheduled = "scheduled"
	RoundStatusClosed    = "closed"
	RoundStatusCancelled = "cancelled"

	SystemSlotStateRegistrationOpen   = "registration_open"
	SystemSlotStateRegistrationClosed = "registration_closed"
	SystemSlotStateActive             = "active"
	SystemSlotStateEnded              = "ended"

	OrderabilityOrderable          = "orderable"
	OrderabilityServiceUnavailable = "service_unavailable"
	OrderabilityBatchPaused        = "batch_paused"
	OrderabilityOfferPaused        = "offer_paused"
	OrderabilityNotStarted         = "not_started"
	OrderabilityRoundEnded         = "round_ended"
	OrderabilitySoldOut            = "sold_out"
	OrderabilityBatchExpired       = "batch_expired"
	OrderabilityCredentialShortage = "credential_unavailable"
)

type Batch struct {
	ID                        string
	APIServiceID              string
	OwnerUserID               string
	ServiceTitle              string
	DistributionSystem        string
	ServiceOrderable          bool
	DeclaredTTFTBand          string
	DeclaredMaxConcurrency    int
	PerformanceConfirmedAt    *time.Time
	PromptAuditEnabled        *bool
	SourceType                string
	SourceLabel               string
	Status                    string
	DeclaredTotalUSDAllowance string
	UnallocatedUSDAllowance   string
	SaleCutoffAt              time.Time
	ExpiresAt                 time.Time
	SourceConfirmedAt         time.Time
	PublishedAt               *time.Time
	CreatedAt                 time.Time
	UpdatedAt                 time.Time
	Version                   int64
}

type Offer struct {
	ID                 string
	BatchID            string
	APIServiceID       string
	OwnerUserID        string
	PreviousVersionID  string
	DistributionSystem string
	Name               string
	USDAllowance       string
	PriceCNY           string
	CNYPerUSD          string
	ModelMultiplier    string
	QuotaUsagePolicy   apimarket.QuotaUsagePolicy
	DeliveryMode       string
	DeliveryETAMinutes int
	SaleMode           string
	Status             string
	SortOrder          int
	PublishedAt        *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
	Version            int64
}

type SaleRound struct {
	ID            string
	BatchID       string
	APIServiceID  string
	OwnerUserID   string
	SystemSlotKey string
	Name          string
	StartsAt      time.Time
	EndsAt        time.Time
	Status        string
	Allocations   []Allocation
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Version       int64
}

type SystemSaleSlot struct {
	Key                  string
	StartsAt             time.Time
	EndsAt               time.Time
	RegistrationClosesAt time.Time
	State                string
	ServerNow            time.Time
}

type Allocation struct {
	ID                    string
	BatchID               string
	OfferID               string
	APIServiceID          string
	OwnerUserID           string
	SaleRoundID           string
	SaleMode              string
	CopyLimit             int
	AvailableCopies       int
	ReservedCopies        int
	ConsumedCopies        int
	AllocatedUSDAllowance string
	ReturnedUSDAllowance  string
	Status                string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type OfferCard struct {
	Offer
	BatchStatus               string
	ServiceTitle              string
	ServiceOrderable          bool
	SellerDisplayName         string
	SellerIdentityType        string
	SellerLinuxDOBound        bool
	DeclaredTTFTBand          string
	DeclaredMaxConcurrency    int
	PerformanceConfirmedAt    *time.Time
	PromptAuditEnabled        *bool
	PerformanceDisclaimer     string
	SaleCutoffAt              time.Time
	ExpiresAt                 time.Time
	CurrentRound              *SaleRound
	NextRound                 *SaleRound
	AvailableCopies           int
	CredentialAvailableCopies int
	IsOrderable               bool
	OrderabilityCode          string
	OrderabilityReason        string
}

type CreateBatchInput struct {
	APIServiceID              string
	OwnerUserID               string
	SourceType                string
	SourceLabel               string
	DeclaredTotalUSDAllowance string
	SaleCutoffAt              time.Time
	ExpiresAt                 time.Time
	SourceConfirmedAt         time.Time
}

type CreateOfferInput struct {
	BatchID            string
	OwnerUserID        string
	Name               string
	USDAllowance       string
	PriceCNY           string
	ModelMultiplier    string
	QuotaUsagePolicy   apimarket.QuotaUsagePolicy
	DeliveryMode       string
	DeliveryETAMinutes int
	SaleMode           string
	ContinuousCopies   int
	SortOrder          int
}

type RoundOfferInput struct {
	OfferID string
	Copies  int
}

type CreateRoundInput struct {
	BatchID     string
	OwnerUserID string
	Name        string
	StartsAt    time.Time
	EndsAt      time.Time
	Offers      []RoundOfferInput
}

type CreateOrderInput struct {
	OfferID              string
	SaleRoundID          string
	BuyerUserID          string
	BuyerContactMethodID string
	SelectedAccessMode   string
	PaymentMethod        string
	BuyerNote            string
	RequestID            string
}

type BatchActionInput struct {
	BatchID         string
	OwnerUserID     string
	ExpectedVersion int64
	RequestID       string
}

type PublicOfferFilter struct {
	DistributionSystem string
	OnlyOneMultiplier  bool
	OnlyOrderable      bool
	SystemSlotKey      string
	Search             string
	ExcludeSystemSlots bool
}

type CredentialSummary struct {
	OfferID   string
	Available int
	Reserved  int
	Delivered int
	Retired   int
}

type CredentialImportInput struct {
	OfferID      string
	DeliveryKind string
	CSV          io.Reader
}

type CredentialImportRow struct {
	DeliveryKind  string
	APIBaseURL    string
	APIKey        string
	PanelLoginURL string
	Username      string
	Password      string
	Instructions  string
}

type CredentialImportResult struct {
	Imported int
	Summary  CredentialSummary
}

type CreateRushOfferInput struct {
	APIServiceID       string
	SourceType         string
	SourceLabel        string
	Name               string
	USDAllowance       string
	PriceCNY           string
	ModelMultiplier    string
	QuotaUsagePolicy   apimarket.QuotaUsagePolicy
	Copies             int
	DeliveryMode       string
	DeliveryETAMinutes int
	SlotKey            string
	ExpiresAt          time.Time
	SourceConfirmedAt  time.Time
	DeliveryKind       string
	CredentialRows     []CredentialImportRow
}

type RushOfferPublication struct {
	Batch              Batch
	Offer              Offer
	Round              SaleRound
	CredentialSummary  CredentialSummary
	CredentialImported int
}

type RushOfferCompletionBuilder func(RushOfferPublication) (idempotency.Completion, *domain.AppError)
