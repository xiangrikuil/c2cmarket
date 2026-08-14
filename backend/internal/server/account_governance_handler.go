package server

import (
	"net/http"
	"time"

	"c2c-market/backend/internal/module/accountgovernance"
)

type accountGovernanceCurrentActionDTO struct {
	ActionType        string  `json:"actionType"`
	ReasonCode        string  `json:"reasonCode"`
	PublicReason      string  `json:"publicReason"`
	EffectiveAt       string  `json:"effectiveAt"`
	ExpiresAt         *string `json:"expiresAt"`
	Indefinite        bool    `json:"indefinite"`
	GovernanceVersion int64   `json:"governanceVersion"`
}

type accountGovernanceDispositionDTO struct {
	ID                     string   `json:"id"`
	ResourceType           string   `json:"resourceType"`
	ResourceID             string   `json:"resourceId"`
	ResourceLabel          string   `json:"resourceLabel"`
	ParticipantRole        string   `json:"participantRole"`
	Result                 string   `json:"result"`
	ReasonCode             string   `json:"reasonCode"`
	TriggerRoles           []string `json:"triggerRoles"`
	BeforeStatus           string   `json:"beforeStatus"`
	AfterStatus            string   `json:"afterStatus"`
	ReleasedResourceType   string   `json:"releasedResourceType,omitempty"`
	ReleasedQuantity       string   `json:"releasedQuantity,omitempty"`
	GovernanceEffectiveAt  string   `json:"governanceEffectiveAt"`
	PaymentClaimEligible   bool     `json:"paymentClaimEligible"`
	PaymentClaimDeadlineAt *string  `json:"paymentClaimDeadlineAt"`
	ActionCodes            []string `json:"actionCodes"`
	TargetURL              string   `json:"targetUrl"`
	UpdatedAt              string   `json:"updatedAt"`
}

type accountGovernanceBusinessCenterDTO struct {
	GeneratedAt      string                             `json:"generatedAt"`
	AccountStatus    string                             `json:"accountStatus"`
	ProcessingStatus string                             `json:"processingStatus"`
	CurrentAction    *accountGovernanceCurrentActionDTO `json:"currentAction"`
	Items            []accountGovernanceDispositionDTO  `json:"items"`
}

func (s *Server) handleAccountGovernanceBusinessCenter(w http.ResponseWriter, r *http.Request) {
	actor, appErr := s.requireBusinessActor(r, true, false)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	center, appErr := s.accountGovernance.AccountGovernanceBusinessCenter(r.Context(), actor)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toAccountGovernanceBusinessCenterDTO(center))
}

func toAccountGovernanceBusinessCenterDTO(center accountgovernance.Center) accountGovernanceBusinessCenterDTO {
	result := accountGovernanceBusinessCenterDTO{
		GeneratedAt: center.GeneratedAt.UTC().Format(time.RFC3339), AccountStatus: center.AccountStatus,
		ProcessingStatus: center.ProcessingStatus, Items: make([]accountGovernanceDispositionDTO, 0, len(center.Items)),
	}
	if center.CurrentAction != nil {
		action := center.CurrentAction
		result.CurrentAction = &accountGovernanceCurrentActionDTO{
			ActionType: action.ActionType, ReasonCode: action.ReasonCode, PublicReason: action.PublicReason,
			EffectiveAt: action.EffectiveAt.UTC().Format(time.RFC3339), ExpiresAt: timeStringPointer(action.ExpiresAt),
			Indefinite: action.Indefinite, GovernanceVersion: action.GovernanceVersion,
		}
	}
	for _, item := range center.Items {
		result.Items = append(result.Items, accountGovernanceDispositionDTO{
			ID: item.ID, ResourceType: item.ResourceType, ResourceID: item.ResourceID,
			ResourceLabel: item.ResourceLabel, ParticipantRole: item.ParticipantRole, Result: item.Result,
			ReasonCode: item.ReasonCode, TriggerRoles: item.TriggerRoles, BeforeStatus: item.BeforeStatus,
			AfterStatus: item.AfterStatus, ReleasedResourceType: item.ReleasedResourceType,
			ReleasedQuantity: item.ReleasedQuantity, GovernanceEffectiveAt: item.GovernanceEffectiveAt.UTC().Format(time.RFC3339),
			PaymentClaimEligible: item.PaymentClaimEligible, PaymentClaimDeadlineAt: timeStringPointer(item.PaymentClaimDeadlineAt),
			ActionCodes: item.ActionCodes, TargetURL: item.TargetURL, UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}
	return result
}

func timeStringPointer(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}
