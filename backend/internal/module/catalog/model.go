package catalog

import (
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

const (
	StatusActive     = "active"
	StatusDeprecated = "deprecated"
	StatusBlocked    = "blocked"

	EffectiveStatusSourceSelf   = "self"
	EffectiveStatusSourceParent = "parent"

	ResourceProductCategory = "product_category"
	ResourceProductPlan     = "product_plan"
	ResourceAPIProvider     = "api_model_provider"
	ResourceAPIModel        = "api_model_catalog"

	LifecycleActionDeprecate  = "deprecate"
	LifecycleActionBlock      = "block"
	LifecycleActionReactivate = "reactivate"
	LifecycleActionUnblock    = "unblock"
)

type Lifecycle struct {
	CoreKey               string
	Status                string
	EffectiveStatus       string
	EffectiveStatusSource string
	StatusChangedAt       time.Time
	StatusReason          string
	StatusChangedBy       string
	Version               int64
	IdentityLocked        bool
	IdentityLockReason    string
}

func (l Lifecycle) IsEffectiveActive() bool {
	return l.EffectiveStatus == StatusActive
}

type LifecycleActionInput struct {
	ResourceType    string
	ResourceID      string
	Action          string
	Reason          string
	TargetStatus    string
	OperatorID      string
	ExpectedVersion int64
	RequestID       string
}

type LifecycleMutationResult struct {
	ResourceType string
	Category     *ProductCategory
	Plan         *ProductPlan
	Provider     *APIModelProvider
	Model        *APIModelCatalog
}

func (r LifecycleMutationResult) ResourceID() string {
	switch {
	case r.Category != nil:
		return r.Category.ID
	case r.Plan != nil:
		return r.Plan.ID
	case r.Provider != nil:
		return r.Provider.ID
	case r.Model != nil:
		return r.Model.ID
	default:
		return ""
	}
}

type LifecycleCompletionBuilder func(LifecycleMutationResult) (idempotency.Completion, *domain.AppError)

type ProductCategory struct {
	Lifecycle
	ID          string
	Code        string
	DisplayName string
	IconDataURL string
	SortOrder   int
	// Active is a read-only compatibility projection of EffectiveStatus.
	Active bool
}

type ProductCategoryInput struct {
	Code        string
	DisplayName string
	IconDataURL string
	SortOrder   int
}

type ProductCategoryMutationInput struct {
	ID         string
	OperatorID string
	Form       ProductCategoryInput
}

type ProductPlan struct {
	Lifecycle
	ID                   string
	CategoryID           string
	CategoryCode         string
	ProviderCode         string
	Slug                 string
	DisplayName          string
	Description          string
	PublishPolicy        string
	AccessMode           string
	ProviderPolicyStatus string
	RiskLevel            string
	RiskAckRequired      bool
	RiskNoticeCode       string
	PolicyVersion        int64
	PolicyNote           string
	QuotaLabel           string
	QuotaUnit            string
	QuotaPeriod          string
	AllowCustomVariant   bool
	SortOrder            int
	CreatedAt            time.Time
	UpdatedAt            time.Time
	// Active is a read-only compatibility projection of EffectiveStatus.
	Active bool
}

type ProductPlanInput struct {
	CategoryID           string
	ProviderCode         string
	Slug                 string
	DisplayName          string
	Description          string
	PublishPolicy        string
	AccessMode           string
	ProviderPolicyStatus string
	RiskLevel            string
	RiskAckRequired      bool
	RiskNoticeCode       string
	PolicyNote           string
	QuotaLabel           string
	QuotaUnit            string
	QuotaPeriod          string
	AllowCustomVariant   bool
	SortOrder            int
}

type ProductPlanMutationInput struct {
	ID         string
	OperatorID string
	Form       ProductPlanInput
}

type APIModelCatalog struct {
	Lifecycle
	ID                         string
	ProviderID                 string
	ProviderCategory           string
	ProviderCode               string
	Provider                   string
	ProviderStatus             string
	ProviderActive             bool
	ModelKey                   string
	Capabilities               []string
	SortOrder                  int
	CurrentPriceVersionID      string
	CurrentPriceSourceURL      string
	CurrentPriceSourceVersion  string
	CurrentPriceValidFrom      *time.Time
	InputPricePerMillion       string
	CachedInputPricePerMillion string
	OutputPricePerMillion      string
	CreatedAt                  time.Time
	UpdatedAt                  time.Time
	// Active is a read-only compatibility projection of EffectiveStatus.
	Active bool
}

type APIModelProvider struct {
	Lifecycle
	ID               string
	ProviderCategory string
	Code             string
	DisplayName      string
	SortOrder        int
	CreatedAt        time.Time
	UpdatedAt        time.Time
	// Active is a read-only compatibility projection of EffectiveStatus.
	Active bool
}

type APIModelProviderInput struct {
	ProviderCategory string
	Code             string
	DisplayName      string
	SortOrder        int
}

type APIModelProviderMutationInput struct {
	ID         string
	OperatorID string
	Form       APIModelProviderInput
}

type APIModelInput struct {
	ProviderID            string
	ModelKey              string
	Capabilities          []string
	SourceURL             string
	SourceVersion         string
	InputTokenPrice       string
	CachedInputTokenPrice string
	OutputTokenPrice      string
	SortOrder             int
}

type APIModelMutationInput struct {
	ID         string
	OperatorID string
	Form       APIModelInput
}

const (
	APIModelSyncStatusNew           = "new"
	APIModelSyncStatusPriceChanged  = "price_changed"
	APIModelSyncStatusUnchanged     = "unchanged"
	APIModelSyncStatusSourceMissing = "source_missing"
	APIModelSyncStatusUnavailable   = "unavailable"
)

type APIModelSyncPreviewInput struct {
	ProviderIDs []string
}

type APIModelSyncPreview struct {
	Fingerprint string
	FetchedAt   time.Time
	Counts      APIModelSyncCounts
	Items       []APIModelSyncItem
}

type APIModelSyncCounts struct {
	New           int
	PriceChanged  int
	Unchanged     int
	SourceMissing int
	Unavailable   int
}

type APIModelSyncItem struct {
	CandidateKey                    string
	Fingerprint                     string
	Status                          string
	ReasonCode                      string
	Reason                          string
	ProviderID                      string
	ProviderCode                    string
	Provider                        string
	ModelKey                        string
	Capabilities                    []string
	SourceURL                       string
	SourceVersion                   string
	InputPricePerMillion            string
	CachedInputPricePerMillion      string
	OutputPricePerMillion           string
	LocalModelID                    string
	LocalPriceVersionID             string
	LocalInputPricePerMillion       string
	LocalCachedInputPricePerMillion string
	LocalOutputPricePerMillion      string
	LocalSourceURL                  string
	LocalSourceVersion              string
}

type APIModelSyncSelection struct {
	Fingerprint                string
	Active                     bool
	ProviderID                 string
	ProviderCode               string
	ModelKey                   string
	Capabilities               []string
	SourceURL                  string
	SourceVersion              string
	InputPricePerMillion       string
	CachedInputPricePerMillion string
	OutputPricePerMillion      string
	LocalModelID               string
	LocalPriceVersionID        string
	Status                     string
}

type APIModelSyncApplyInput struct {
	Items []APIModelSyncSelection
}

type APIModelSyncMutationInput struct {
	OperatorID string
	Items      []APIModelSyncSelection
}

type APIModelBulkMutationResult struct {
	Created int
	Updated int
	Changed int
	IDs     []string
}

type APIModelSyncCompletionBuilder func(APIModelBulkMutationResult) (idempotency.Completion, *domain.AppError)
