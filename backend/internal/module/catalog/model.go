package catalog

import (
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

type ProductCategory struct {
	ID          string
	Code        string
	DisplayName string
	IconDataURL string
	SortOrder   int
	Active      bool
}

type ProductCategoryInput struct {
	Code        string
	DisplayName string
	IconDataURL string
	SortOrder   int
	Active      bool
}

type ProductCategoryMutationInput struct {
	ID         string
	OperatorID string
	Form       ProductCategoryInput
}

type ProductPlan struct {
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
	Active               bool
	AllowCustomVariant   bool
	SortOrder            int
	CreatedAt            time.Time
	UpdatedAt            time.Time
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
	Active               bool
	AllowCustomVariant   bool
	SortOrder            int
}

type ProductPlanMutationInput struct {
	ID         string
	OperatorID string
	Form       ProductPlanInput
}

type APIModelCatalog struct {
	ID                         string
	ProviderID                 string
	ProviderCategory           string
	ProviderCode               string
	Provider                   string
	ProviderActive             bool
	ModelKey                   string
	Capabilities               []string
	Active                     bool
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
}

type APIModelProvider struct {
	ID               string
	ProviderCategory string
	Code             string
	DisplayName      string
	Active           bool
	SortOrder        int
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type APIModelProviderInput struct {
	ProviderCategory string
	Code             string
	DisplayName      string
	Active           bool
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
	Active                bool
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
	Status                     string
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
	Active                     bool
}

type APIModelSyncApplyInput struct {
	Items []APIModelSyncSelection
}

type APIModelSyncMutationInput struct {
	OperatorID string
	Items      []APIModelSyncSelection
}

type APIModelBulkStatusInput struct {
	ModelIDs []string
	Active   bool
}

type APIModelBulkStatusMutationInput struct {
	OperatorID string
	ModelIDs   []string
	Active     bool
}

type APIModelBulkMutationResult struct {
	Created int
	Updated int
	Changed int
	IDs     []string
}

type APIModelSyncCompletionBuilder func(APIModelBulkMutationResult) (idempotency.Completion, *domain.AppError)
