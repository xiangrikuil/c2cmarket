package core

import (
	"c2c-market/backend/internal/module/announcement"
	"c2c-market/backend/internal/module/apiintent"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/carpool"
	"c2c-market/backend/internal/module/catalog"
	"c2c-market/backend/internal/module/contact"
	"c2c-market/backend/internal/module/demand"
	"c2c-market/backend/internal/module/feedback"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/officialprice"
	"c2c-market/backend/internal/module/profile"
	"c2c-market/backend/internal/module/report"
	"c2c-market/backend/internal/module/review"
)

type User = auth.User

type Session = auth.Session

type OAuthProfile = auth.OAuthProfile

type BootstrapAdminInput = auth.BootstrapAdminInput

type BootstrapAdminResult = auth.BootstrapAdminResult

type SetPasswordInput = auth.SetPasswordInput

type EmailRegistrationStartInput = auth.EmailRegistrationStartInput

type EmailRegistrationChallenge = auth.EmailRegistrationChallenge

type EmailRegistrationConfirmInput = auth.EmailRegistrationConfirmInput

type LinuxDoBinding = auth.LinuxDoBinding

type OfficialPriceLead = officialprice.Lead

type OfficialPriceRecord = officialprice.Record

type IdempotencyEntry = idempotency.Entry

type IdempotencyCompletion = idempotency.Completion

type SubmitLeadInput = officialprice.SubmitLeadInput

type ApproveLeadInput = officialprice.ApproveLeadInput

type OfficialPriceApprovalCompletionBuilder = officialprice.ApprovalCompletionBuilder

type ProductCategory = catalog.ProductCategory

type ProductCategoryInput = catalog.ProductCategoryInput

type ProductPlan = catalog.ProductPlan

type ProductPlanInput = catalog.ProductPlanInput

type APIModelProvider = catalog.APIModelProvider

type APIModelProviderInput = catalog.APIModelProviderInput

type APIModelCatalog = catalog.APIModelCatalog

type APIModelInput = catalog.APIModelInput

type APIService = apimarket.Service

type APIServiceAccessMode = apimarket.ServiceAccessMode

type APIServiceModel = apimarket.ServiceModel

type APIServicePackage = apimarket.ServicePackage

type CreateAPIServiceInput = apimarket.CreateServiceInput

type UpdateAPIServiceInput = apimarket.UpdateServiceInput

type APIServiceAccessModeInput = apimarket.ServiceAccessModeInput

type APIServiceModelInput = apimarket.ServiceModelInput

type APIServicePackageInput = apimarket.ServicePackageInput

type APIServiceOwnerActionInput = apimarket.ServiceOwnerActionInput

type APIServiceAdminActionInput = apimarket.ServiceAdminActionInput

type APIPurchaseIntent = apiintent.Intent

type CreateAPIPurchaseIntentInput = apiintent.CreateIntentInput

type APIPurchaseIntentActionInput = apiintent.ActionInput

type APIPurchaseIntentCompletionBuilder = apiintent.CompletionBuilder

type APIOrder = apiorder.Order

type APIOrderPaymentInstructionsView = apiorder.PaymentInstructionsView

type CreateAPIOrderInput = apiorder.CreateInput

type APIOrderActionInput = apiorder.ActionInput

type APIOrderCompletionBuilder = apiorder.CompletionBuilder

type RiskAcknowledgement = carpool.RiskAcknowledgement

type CarpoolListing = carpool.Listing

type CarpoolCycleTerm = carpool.CycleTerm

type CarpoolApplication = carpool.Application

type CarpoolMembership = carpool.Membership

type CreateCarpoolListingInput = carpool.CreateListingInput

type PublishCarpoolListingInput = carpool.PublishListingInput

type CarpoolCycleTermInput = carpool.CycleTermInput

type CarpoolReviewInput = carpool.ReviewInput

type UpdateCarpoolListingInput = carpool.UpdateListingInput

type SubmitCarpoolListingReviewInput = carpool.SubmitListingReviewInput

type CreateCarpoolApplicationInput = carpool.CreateApplicationInput

type AcceptCarpoolApplicationInput = carpool.AcceptApplicationInput

type RejectCarpoolApplicationInput = carpool.RejectApplicationInput

type CancelCarpoolApplicationInput = carpool.CancelApplicationInput

type WithdrawCarpoolAcceptanceInput = carpool.WithdrawAcceptanceInput

type ConfirmCarpoolApplicationJoinInput = carpool.ConfirmApplicationJoinInput

type CarpoolApplicationCompletionBuilder = carpool.ApplicationCompletionBuilder

type ConfirmCarpoolMembershipCompleteInput = carpool.ConfirmMembershipCompleteInput

type EndCarpoolMembershipInput = carpool.EndMembershipInput

type CarpoolMembershipCompletionBuilder = carpool.MembershipCompletionBuilder

type ContactMethod = contact.ContactMethod

type ContactMethodVersion = contact.ContactMethodVersion

type ContactSession = contact.ContactSession

type ContactAccessLog = contact.ContactAccessLog

type ContactMethodInput = contact.ContactMethodInput

type CreateContactSessionInput = contact.CreateContactSessionInput

type ContactSessionView = contact.ContactSessionView

type ContactItemView = contact.ContactItemView

type UserProfile = profile.UserProfile

type UpdateUserProfileInput = profile.UpdateUserProfileInput

type EmailVerificationStartInput = profile.EmailVerificationStartInput

type EmailVerificationConfirmInput = profile.EmailVerificationConfirmInput

type EmailVerificationChallenge = profile.EmailVerificationChallenge

type PublicUserProfile = profile.PublicUserProfile

type MerchantProfile = profile.MerchantProfile

type UpsertMerchantProfileInput = profile.UpsertMerchantProfileInput

type PublicMerchantProfile = profile.PublicMerchantProfile

type ReviewCenterRow = review.ReviewCenterRow

type SubmitReviewInput = review.SubmitReviewInput

type PublicReview = review.PublicReview

type ReviewCompletionBuilder = review.CompletionBuilder

type Report = report.Report

type DisputeCase = report.DisputeCase

type Appeal = report.Appeal

type PublicDispute = report.PublicDispute

type CreateReportInput = report.CreateReportInput

type CreateAppealInput = report.CreateAppealInput

type ReportAdminActionInput = report.AdminActionInput

type ReportCompletionBuilder = report.ReportCompletionBuilder

type AppealCompletionBuilder = report.AppealCompletionBuilder

type ReportAdminCompletionBuilder = report.AdminCompletionBuilder

type Announcement = announcement.Announcement

type AnnouncementFormInput = announcement.FormInput

type AnnouncementAuditLog = announcement.AuditLog

type AnnouncementReceipt = announcement.Receipt

type Demand = demand.Demand

type CreateDemandInput = demand.CreateInput

type DemandOwnerActionInput = demand.OwnerActionInput

type DemandAdminActionInput = demand.AdminActionInput

type DemandCompletionBuilder = demand.CompletionBuilder

type FeedbackTicket = feedback.Ticket

type FeedbackEvent = feedback.Event

type CreateFeedbackInput = feedback.CreateInput

type FeedbackSupplementInput = feedback.SupplementInput

type FeedbackAdminHandleInput = feedback.AdminHandleInput

type FeedbackCompletionBuilder = feedback.CompletionBuilder
