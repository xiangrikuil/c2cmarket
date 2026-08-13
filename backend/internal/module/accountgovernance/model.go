package accountgovernance

import "time"

const (
	ActionViewResource = "view_resource"
	ActionPaymentClaim = "payment_claim"
)

type CurrentAction struct {
	ActionType        string     `json:"actionType"`
	ReasonCode        string     `json:"reasonCode"`
	PublicReason      string     `json:"publicReason"`
	EffectiveAt       time.Time  `json:"effectiveAt"`
	ExpiresAt         *time.Time `json:"expiresAt"`
	Indefinite        bool       `json:"indefinite"`
	GovernanceVersion int64      `json:"governanceVersion"`
}

type Disposition struct {
	ID                     string     `json:"id"`
	ResourceType           string     `json:"resourceType"`
	ResourceID             string     `json:"resourceId"`
	ResourceLabel          string     `json:"resourceLabel"`
	ParticipantRole        string     `json:"participantRole"`
	Result                 string     `json:"result"`
	ReasonCode             string     `json:"reasonCode"`
	TriggerRoles           []string   `json:"triggerRoles"`
	BeforeStatus           string     `json:"beforeStatus"`
	AfterStatus            string     `json:"afterStatus"`
	ReleasedResourceType   string     `json:"releasedResourceType,omitempty"`
	ReleasedQuantity       string     `json:"releasedQuantity,omitempty"`
	GovernanceEffectiveAt  time.Time  `json:"governanceEffectiveAt"`
	PaymentClaimEligible   bool       `json:"paymentClaimEligible"`
	PaymentClaimDeadlineAt *time.Time `json:"paymentClaimDeadlineAt"`
	ActionCodes            []string   `json:"actionCodes"`
	TargetURL              string     `json:"targetUrl"`
	UpdatedAt              time.Time  `json:"updatedAt"`
}

type Center struct {
	GeneratedAt      time.Time      `json:"generatedAt"`
	AccountStatus    string         `json:"accountStatus"`
	ProcessingStatus string         `json:"processingStatus"`
	CurrentAction    *CurrentAction `json:"currentAction"`
	Items            []Disposition  `json:"items"`
}
