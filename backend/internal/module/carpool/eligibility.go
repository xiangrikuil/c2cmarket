package carpool

import (
	"net/http"
	"strings"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/catalog"
)

const (
	EligibilityEligible            = "eligible"
	EligibilitySoldOut             = "sold_out"
	EligibilityPaused              = "paused"
	EligibilityCredentialRisk      = "credential_risk"
	EligibilityOwnerActionRequired = "owner_action_required"
	EligibilityAlreadyApplied      = "already_applied"
	EligibilityAlreadyMember       = "already_member"
	EligibilitySelfOwned           = "self_owned"
)

type ApplicationEligibility struct {
	Code             string
	CanApply         bool
	Reason           string
	ResolutionAction string
}

type EligibilityContext struct {
	Listing               Listing
	Plan                  catalog.ProductPlan
	CurrentUserID         string
	HasOngoingApplication bool
	HasActiveMembership   bool
}

func EvaluateApplicationEligibility(input EligibilityContext) ApplicationEligibility {
	listing := input.Listing
	if domain.LooksLikeSecretContent(listing.AccessArrangement) || domain.LooksLikeSecretContent(listing.DistributionMethodNote) {
		return blockedEligibility(EligibilityCredentialRisk, "访问安排包含共享凭据风险，当前不能申请。", "wait_for_owner_correction")
	}
	if strings.TrimSpace(listing.AccessArrangement) == "" || input.Plan.PublishPolicy != "allowed" || (listing.RiskAckRequired && strings.TrimSpace(listing.RiskNoticeCode) == "") {
		return blockedEligibility(EligibilityOwnerActionRequired, "车源资料或风险声明需要车主修正。", "wait_for_owner_correction")
	}
	if listing.Status != ListingStatusActive {
		return blockedEligibility(EligibilityPaused, "车源当前暂停或尚未公开。", "browse_other_listings")
	}
	if strings.TrimSpace(input.CurrentUserID) != "" && input.CurrentUserID == listing.OwnerUserID {
		return blockedEligibility(EligibilitySelfOwned, "不能申请自己的车源。", "manage_own_listing")
	}
	if input.HasActiveMembership {
		return blockedEligibility(EligibilityAlreadyMember, "你已是该车源的成员。", "view_membership")
	}
	if input.HasOngoingApplication {
		return blockedEligibility(EligibilityAlreadyApplied, "你已有该车源的进行中申请。", "view_application")
	}
	if listing.AvailableSeats < 1 {
		return blockedEligibility(EligibilitySoldOut, "当前车源没有可申请名额。", "browse_other_listings")
	}
	return ApplicationEligibility{Code: EligibilityEligible, CanApply: true, Reason: "当前可申请上车。", ResolutionAction: "apply"}
}

func EvaluatePublicListingEligibility(listing Listing) ApplicationEligibility {
	return EvaluateApplicationEligibility(EligibilityContext{
		Listing: listing,
		Plan:    catalog.ProductPlan{PublishPolicy: "allowed"},
	})
}

func blockedEligibility(code, reason, resolutionAction string) ApplicationEligibility {
	return ApplicationEligibility{Code: code, CanApply: false, Reason: reason, ResolutionAction: resolutionAction}
}

func eligibilityError(value ApplicationEligibility) *domain.AppError {
	switch value.Code {
	case EligibilitySelfOwned:
		return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Cannot apply to own carpool", value.Reason)
	case EligibilityAlreadyMember:
		return domain.NewError(http.StatusConflict, domain.CodeActiveMembershipExists, "Active membership exists", value.Reason)
	case EligibilityAlreadyApplied:
		return domain.NewError(http.StatusConflict, domain.CodeActiveApplicationExists, "Active application exists", value.Reason)
	case EligibilitySoldOut:
		return domain.NewError(http.StatusConflict, domain.CodeSeatUnavailable, "Seat unavailable", value.Reason)
	default:
		return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Carpool application unavailable", value.Reason)
	}
}
