package reputation

import (
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
)

const (
	RoleBuyer  = "buyer"
	RoleSeller = "seller"
	RoleAll    = "all"

	ScopeCarpool = "carpool"
	ScopeAPI     = "api"
	ScopeOverall = "overall"

	TransactionCarpoolApplication = "carpool_application"
	TransactionCarpoolMembership  = "carpool_membership"
	TransactionAPIPurchaseIntent  = "api_purchase_intent"
	TransactionAPIOrder           = "api_order"

	ActionCarpoolPublish    = "carpool_publish"
	ActionCarpoolApply      = "carpool_apply"
	ActionCarpoolAccept     = "carpool_accept"
	ActionAPIServicePublish = "api_service_publish"
	ActionAPIOrderCreate    = "api_order_create"
	ActionContactView       = "contact_view"
	ActionReviewSubmit      = "review_submit"
	ActionAll               = "all"

	ResponsibilityResponsible    = "responsible"
	ResponsibilityShared         = "shared"
	ResponsibilityNotResponsible = "not_responsible"
	ResponsibilityUndetermined   = "undetermined"

	SeverityNone     = "none"
	SeverityLow      = "low"
	SeverityMedium   = "medium"
	SeverityHigh     = "high"
	SeverityCritical = "critical"

	OutcomeStatusActive   = "active"
	OutcomeStatusReversed = "reversed"

	TierInsufficient = "insufficient"
	TierNormal       = "normal"
	TierReliable     = "reliable"
	TierHighTrust    = "high_trust"

	StateActive     = "active"
	StateCaution    = "caution"
	StateRestricted = "restricted"

	ConfidenceLow    = "low"
	ConfidenceMedium = "medium"
	ConfidenceHigh   = "high"

	ProgressMet         = "met"
	ProgressNotMet      = "not_met"
	ProgressBlocked     = "blocked"
	ProgressUnavailable = "unavailable"

	SourceResourceCarpool    = "carpool"
	SourceResourceAPIService = "api_service"

	SourceVerificationNotSubmitted = "not_submitted"
	SourceVerificationPending      = "pending"
	SourceVerificationVerified     = "verified"
	SourceVerificationMismatch     = "mismatch"
	SourceVerificationExpired      = "expired"

	SourceAggregateNotApplicable = "not_applicable"
	SourceAggregateNoSources     = "no_sources"
	SourceAggregatePending       = "pending"
	SourceAggregatePartial       = "partial"
	SourceAggregateVerified      = "verified"
	SourceAggregateMismatch      = "mismatch"
)

type ScopeFacts struct {
	Aggregated                             bool
	CompletedCount                         int
	CompletedCountLast90Days               int
	RoleResponsibilityCancellationCount    int
	RoleResponsibilityCancellationCount90d int
	UnknownResponsibilityCancellationCount int
	UnresolvedDisputeCount                 int
	ConfirmedFaultDisputeCount365d         int
	ConfirmedMajorFaultDisputeCount365d    int
	ActiveRestrictionCount                 int
	VerifiedReviewCount                    int
	RatingSum                              int
	RatingDistribution                     RatingDistribution
	RecentReviewCount90d                   int
	CommonPositiveTags                     []ReputationTagCount
	CommonNegativeTags                     []ReputationTagCount
	PlatformReviewCount                    int
	PlatformAverageRating                  float64
	SourceAuthorMismatch                   bool
	SourceAuthorVerification               SourceAuthorAggregate
	SourceDataUpdatedAt                    *time.Time
	NextRecalculationAt                    *time.Time
}

type RoleFacts struct {
	Carpool ScopeFacts
	API     ScopeFacts
	Overall ScopeFacts
}

type RawFacts struct {
	UserID string
	Buyer  RoleFacts
	Seller RoleFacts
}

type RatingDistribution struct {
	One   int `json:"1"`
	Two   int `json:"2"`
	Three int `json:"3"`
	Four  int `json:"4"`
	Five  int `json:"5"`
}

type ReputationTagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

type SourceAuthorStatusCounts struct {
	Total        int `json:"total"`
	NotSubmitted int `json:"notSubmitted"`
	Pending      int `json:"pending"`
	Verified     int `json:"verified"`
	Mismatch     int `json:"mismatch"`
	Expired      int `json:"expired"`
}

type SourceAuthorAggregate struct {
	State  string                   `json:"state"`
	Counts SourceAuthorStatusCounts `json:"counts"`
}

type SourceAuthorResourceSummary struct {
	Status     string     `json:"status"`
	VerifiedAt *time.Time `json:"verifiedAt"`
	ExpiresAt  *time.Time `json:"expiresAt"`
}

type ReputationMetrics struct {
	CompletedCount                         int                   `json:"completedCount"`
	CompletedCountLast90Days               int                   `json:"completedCountLast90Days"`
	RoleResponsibilityCancellationCount    int                   `json:"roleResponsibilityCancellationCount"`
	UnknownResponsibilityCancellationCount int                   `json:"unknownResponsibilityCancellationCount"`
	RoleControllableTerminalCount          int                   `json:"roleControllableTerminalCount"`
	RoleCompletionRate                     *float64              `json:"roleCompletionRate"`
	RoleFaultCancelRate                    *float64              `json:"roleFaultCancelRate"`
	VerifiedReviewCount                    int                   `json:"verifiedReviewCount"`
	RawAverageRating                       *float64              `json:"rawAverageRating"`
	WeightedRating                         *float64              `json:"weightedRating"`
	RatingDistribution                     RatingDistribution    `json:"ratingDistribution"`
	RecentReviewCount90Days                int                   `json:"recentReviewCount90Days"`
	CommonPositiveTags                     []ReputationTagCount  `json:"commonPositiveTags"`
	CommonNegativeTags                     []ReputationTagCount  `json:"commonNegativeTags"`
	ConfirmedFaultDisputeCount365Days      int                   `json:"confirmedFaultDisputeCount365Days"`
	ConfirmedMajorFaultDisputeCount365Days int                   `json:"confirmedMajorFaultDisputeCount365Days"`
	UnresolvedDisputeCount                 int                   `json:"unresolvedDisputeCount"`
	ActiveRestrictionCount                 int                   `json:"activeRestrictionCount"`
	SourceAuthorVerification               SourceAuthorAggregate `json:"sourceAuthorVerification"`
}

type ReputationProgressItem struct {
	Code           string   `json:"code"`
	Label          string   `json:"label"`
	Status         string   `json:"status"`
	CurrentValue   *float64 `json:"currentValue"`
	RequiredValue  *float64 `json:"requiredValue"`
	RemainingValue *float64 `json:"remainingValue"`
	ActionLabel    *string  `json:"actionLabel"`
	ActionURL      *string  `json:"actionUrl"`
}

type SnapshotKey struct {
	UserID string
	Role   string
	Scope  string
}

type ReputationSnapshot struct {
	UserID              string                   `json:"userId"`
	Role                string                   `json:"role"`
	Scope               string                   `json:"scope"`
	Tier                string                   `json:"tier"`
	State               string                   `json:"state"`
	Confidence          string                   `json:"confidence"`
	RuleVersion         string                   `json:"ruleVersion"`
	Metrics             ReputationMetrics        `json:"metrics"`
	Warnings            []string                 `json:"warnings"`
	Badges              []string                 `json:"badges"`
	Progress            []ReputationProgressItem `json:"progress"`
	TierEnteredAt       time.Time                `json:"tierEnteredAt"`
	ReliableSince       *time.Time               `json:"reliableSince"`
	StateEnteredAt      time.Time                `json:"stateEnteredAt"`
	DirtyAt             *time.Time               `json:"-"`
	CalculatedAt        time.Time                `json:"calculatedAt"`
	SourceDataUpdatedAt *time.Time               `json:"sourceDataUpdatedAt"`
	NextRecalculationAt *time.Time               `json:"nextRecalculationAt"`
}

func (s ReputationSnapshot) Key() SnapshotKey {
	return SnapshotKey{UserID: s.UserID, Role: s.Role, Scope: s.Scope}
}

type ReputationHistory struct {
	ID             string    `json:"id"`
	UserID         string    `json:"userId"`
	Role           string    `json:"role"`
	Scope          string    `json:"scope"`
	FromTier       *string   `json:"fromTier"`
	ToTier         string    `json:"toTier"`
	FromState      *string   `json:"fromState"`
	ToState        string    `json:"toState"`
	RuleVersion    string    `json:"ruleVersion"`
	ReasonSnapshot any       `json:"reasonSnapshot"`
	CreatedAt      time.Time `json:"createdAt"`
}

type AdminReputationAudit struct {
	UserID                    string                          `json:"userId"`
	RuleVersion               string                          `json:"ruleVersion"`
	Items                     []ReputationSnapshot            `json:"items"`
	History                   []ReputationHistory             `json:"history"`
	Restrictions              []UserRestriction               `json:"-"`
	Outcomes                  []DisputeOutcome                `json:"-"`
	Appeals                   []ReputationAppeal              `json:"-"`
	SourceAuthorVerifications []SourceAuthorVerificationAudit `json:"sourceAuthorVerifications"`
}

type AdminReputationEvidence struct {
	Restrictions              []UserRestriction
	Outcomes                  []DisputeOutcome
	Appeals                   []ReputationAppeal
	SourceAuthorVerifications []SourceAuthorVerificationAudit
}

type ReputationAppeal struct {
	ID               string
	AppellantUserID  string
	ReportID         string
	DisputeID        string
	TargetType       string
	TargetID         string
	Title            string
	Statement        string
	Status           string
	AdminReason      string
	HandledByAdminID string
	HandledAt        *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Version          int64
}

type RecalculationResult struct {
	RequestedUsers int       `json:"requestedUsers"`
	RebuiltStates  int       `json:"rebuiltStates"`
	CompletedAt    time.Time `json:"completedAt"`
}

type SourceAuthorVerification struct {
	ID                     string     `json:"id,omitempty"`
	ResourceType           string     `json:"resourceType"`
	ResourceID             string     `json:"resourceId"`
	OwnerUserID            string     `json:"ownerUserId"`
	SourceURL              string     `json:"sourceUrl"`
	ExpectedExternalUserID string     `json:"expectedExternalUserId"`
	ActualExternalUserID   string     `json:"actualExternalUserId,omitempty"`
	Status                 string     `json:"status"`
	VerificationMethod     string     `json:"verificationMethod,omitempty"`
	VerifiedByAdminID      string     `json:"verifiedByAdminId,omitempty"`
	VerifiedAt             *time.Time `json:"verifiedAt,omitempty"`
	ExpiresAt              *time.Time `json:"expiresAt,omitempty"`
	FailureReason          string     `json:"failureReason,omitempty"`
	CreatedAt              *time.Time `json:"createdAt,omitempty"`
	UpdatedAt              *time.Time `json:"updatedAt,omitempty"`
	Version                int64      `json:"version"`
}

type SourceAuthorVerificationEvent struct {
	ID                     string     `json:"id"`
	VerificationID         string     `json:"verificationId"`
	ResourceType           string     `json:"resourceType"`
	ResourceID             string     `json:"resourceId"`
	Action                 string     `json:"action"`
	FromStatus             *string    `json:"fromStatus,omitempty"`
	ToStatus               string     `json:"toStatus"`
	SourceURL              string     `json:"sourceUrl"`
	ExpectedExternalUserID string     `json:"expectedExternalUserId"`
	ActualExternalUserID   string     `json:"actualExternalUserId,omitempty"`
	VerificationMethod     string     `json:"verificationMethod,omitempty"`
	VerifiedByAdminID      string     `json:"verifiedByAdminId"`
	VerifiedAt             *time.Time `json:"verifiedAt,omitempty"`
	ExpiresAt              *time.Time `json:"expiresAt,omitempty"`
	FailureReason          string     `json:"failureReason,omitempty"`
	Version                int64      `json:"version"`
	CreatedAt              time.Time  `json:"createdAt"`
}

type SourceAuthorVerificationAudit struct {
	Verification SourceAuthorVerification        `json:"verification"`
	Events       []SourceAuthorVerificationEvent `json:"events"`
}

type UpdateSourceAuthorVerificationInput struct {
	ResourceType         string
	ResourceID           string
	AdminUserID          string
	Status               string
	ActualExternalUserID string
	VerificationMethod   string
	ExpiresAt            *time.Time
	FailureReason        string
	ExpectedVersion      int64
}

type AdminActor struct {
	UserID  string
	IsAdmin bool
}

type ExcludeTransactionInput struct {
	TransactionType string
	TransactionID   string
	ReasonCode      string
	Reason          string
}

type RestoreTransactionInput struct {
	TransactionType string
	TransactionID   string
	ReasonCode      string
	Reason          string
}

type ExclusionMutation struct {
	TransactionType string
	TransactionID   string
	AdminUserID     string
	ReasonCode      string
	Reason          string
}

type TransactionExclusion struct {
	ID                string
	TransactionType   string
	TransactionID     string
	ExcludedAt        time.Time
	ExcludedByAdminID string
	ReasonCode        string
	Reason            string
	RestoredAt        *time.Time
	RestoredByAdminID string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type DisputeOutcome struct {
	ID                string
	DisputeCaseID     string
	SubjectUserID     string
	Responsibility    string
	Severity          string
	RoleScope         string
	Status            string
	ReasonCode        string
	PublicReason      string
	InternalReason    string
	DecidedByAdminID  string
	DecidedAt         time.Time
	ReversedAt        *time.Time
	ReversedByAdminID string
	ReversalAppealID  string
	ReversalReason    string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Version           int64
	DisputeVersion    int64
}

type UserRestriction struct {
	ID                     string
	UserID                 string
	RestrictionType        string
	RoleScope              string
	ActionCode             string
	ReasonCode             string
	PublicReason           string
	InternalReason         string
	StartsAt               time.Time
	EndsAt                 *time.Time
	SourceDisputeOutcomeID string
	CreatedByAdminID       string
	RevokedAt              *time.Time
	RevokedByAdminID       string
	RevocationReason       string
	CreatedAt              time.Time
	UpdatedAt              time.Time
	Version                int64
	UserVersion            int64
}

type CreateOutcomeInput struct {
	DisputeCaseID   string
	SubjectUserID   string
	Responsibility  string
	Severity        string
	RoleScope       string
	ReasonCode      string
	PublicReason    string
	InternalReason  string
	AdminUserID     string
	ExpectedVersion int64
	RequestID       string
}

type CreateRestrictionInput struct {
	UserID                 string
	RestrictionType        string
	RoleScope              string
	ActionCode             string
	ReasonCode             string
	PublicReason           string
	InternalReason         string
	StartsAt               time.Time
	EndsAt                 *time.Time
	SourceDisputeOutcomeID string
	AdminUserID            string
	ExpectedUserVersion    int64
	RequestID              string
}

type RevokeRestrictionInput struct {
	RestrictionID   string
	Reason          string
	AdminUserID     string
	ExpectedVersion int64
	RequestID       string
}

type GovernanceMutationResult struct {
	Outcome     *DisputeOutcome
	Restriction *UserRestriction
}

type GovernanceCompletionBuilder func(GovernanceMutationResult) (idempotency.Completion, *domain.AppError)
