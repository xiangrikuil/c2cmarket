package server

import (
	"context"
	"log"
	"strings"
	"time"

	"c2c-market/backend/internal/config"
	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/health"
	"c2c-market/backend/internal/middleware"
	"c2c-market/backend/internal/module/accountgovernance"
	"c2c-market/backend/internal/module/announcement"
	"c2c-market/backend/internal/module/apihealth"
	"c2c-market/backend/internal/module/apiintent"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/apimodeltest"
	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/apipromotion"
	"c2c-market/backend/internal/module/apiquota"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/carpool"
	"c2c-market/backend/internal/module/catalog"
	"c2c-market/backend/internal/module/contact"
	"c2c-market/backend/internal/module/devpersona"
	"c2c-market/backend/internal/module/favorite"
	"c2c-market/backend/internal/module/feedback"
	"c2c-market/backend/internal/module/growth"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/modelaudit"
	"c2c-market/backend/internal/module/navigationbadge"
	"c2c-market/backend/internal/module/notification"
	"c2c-market/backend/internal/module/officialprice"
	"c2c-market/backend/internal/module/operationaudit"
	"c2c-market/backend/internal/module/profile"
	"c2c-market/backend/internal/module/promotionreward"
	"c2c-market/backend/internal/module/report"
	"c2c-market/backend/internal/module/reputation"
	"c2c-market/backend/internal/module/review"
	"c2c-market/backend/internal/module/search"
	"c2c-market/backend/internal/observability"
	"c2c-market/backend/internal/platform/turnstile"
	"c2c-market/backend/internal/realtime"
	"github.com/go-chi/chi/v5"
	"net/http"
)

const (
	sessionCookieName = middleware.SessionCookieName
	csrfHeaderName    = middleware.CSRFHeaderName
	requestIDHeader   = middleware.RequestIDHeader
)

type ServerOptions struct {
	EnableDevAuth      bool
	ReadinessChecker   health.Checker
	APIHealth          APIHealthService
	AdminAPIHealth     AdminAPIHealthService
	APIModelTester     APIModelTesterService
	NavigationBadges   NavigationBadgeService
	RealtimeHub        *realtime.Hub
	AppEnv             string
	FrontendOrigin     string
	AllowedOrigins     []string
	OAuth              OAuthOptions
	TrustXForwardedFor bool
	TrustedProxies     []string
	RateLimiter        *middleware.RateLimiter
	Metrics            *observability.Metrics
	MetricsBearerToken string
	TurnstileVerifier  turnstile.Verifier
	SentryEnabled      bool
}

type OAuthOptions struct {
	ProviderMode string
	ClientID     string
	ClientSecret string
	AuthorizeURL string
	TokenURL     string
	UserInfoURL  string
	RedirectURL  string
	Scopes       string
}

type NavigationBadgeService interface {
	Get(ctx context.Context, user auth.User) (navigationbadge.Summary, *domain.AppError)
}

type DevPersonaSessionService interface {
	PrepareDevPersonaSession(ctx context.Context, persona string) (devpersona.Result, *domain.AppError)
}

type APIPaymentSettingsService interface {
	GetAPIAccountPaymentSettings(ctx context.Context, user auth.User) (apimarket.AccountPaymentSettings, *domain.AppError)
	UpdateAPIAccountPaymentSettings(ctx context.Context, user auth.User, input apimarket.UpdateAccountPaymentSettingsInput) (apimarket.AccountPaymentSettings, *domain.AppError)
}

type APIHealthService interface {
	OwnerConnections(ctx context.Context, user auth.User) ([]apihealth.Connection, *domain.AppError)
	OwnerConnection(ctx context.Context, user auth.User, connectionID string) (apihealth.Connection, bool, *domain.AppError)
	PreflightOwnerConnection(ctx context.Context, user auth.User, input apihealth.ConnectionInput) (apihealth.PreflightResult, *domain.AppError)
	PreflightExistingOwnerConnection(ctx context.Context, user auth.User, connectionID string, input apihealth.ConnectionInput, expectedVersion int64) (apihealth.PreflightResult, *domain.AppError)
	CreateOwnerConnectionWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input apihealth.ConnectionInput, requestID string, buildCompletion apihealth.MutationCompletionBuilder) (idempotency.Completion, *domain.AppError)
	UpdateOwnerConnectionWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash, connectionID string, input apihealth.ConnectionInput, expectedVersion int64, requestID string, buildCompletion apihealth.MutationCompletionBuilder) (idempotency.Completion, *domain.AppError)
	VerifyOwnerConnectionWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash, connectionID string, expectedVersion int64, requestID string, buildCompletion apihealth.MutationCompletionBuilder) (idempotency.Completion, *domain.AppError)
	DeleteOwnerConnectionWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash, connectionID string, expectedVersion int64, requestID string, buildCompletion apihealth.MutationCompletionBuilder) (idempotency.Completion, *domain.AppError)
	Summaries(ctx context.Context, serviceIDs []string) (map[string]apihealth.Summary, *domain.AppError)
}

type AdminAPIHealthService interface {
	ProbeCalibration(ctx context.Context, model, protocol, environment string) (apihealth.Calibration, *domain.AppError)
	PreviewLatencyRule(ctx context.Context, model, protocol, environment string, slowTTFTMS, hardTimeoutMS int) (apihealth.LatencyRulePreview, *domain.AppError)
	PublishLatencyRule(ctx context.Context, admin auth.User, model, protocol, environment string, slowTTFTMS, hardTimeoutMS int) (apihealth.LatencyRule, *domain.AppError)
	LatencyRules(ctx context.Context) ([]apihealth.LatencyRule, *domain.AppError)
}

type APIModelTesterService interface {
	OrderSources(ctx context.Context, user auth.User) ([]apimodeltest.OrderSource, *domain.AppError)
	Discover(ctx context.Context, user auth.User, source apimodeltest.CredentialSource) (apimodeltest.Discovery, *domain.AppError)
	Test(ctx context.Context, user auth.User, source apimodeltest.CredentialSource, model string) (apimodeltest.ModelTest, *domain.AppError)
}

type AdminUserService interface {
	AdminUsers(ctx context.Context, user auth.User, query auth.AdminUserDirectoryQuery) (auth.AdminUserDirectory, *domain.AppError)
	AdminAuditLogs(ctx context.Context, user auth.User, filter auth.AdminAuditLogFilter, page domain.PageRequest) (domain.Page[auth.AdminAuditLog], *domain.AppError)
	AdminUser(ctx context.Context, user auth.User, userID string) (auth.AdminUserDetail, *domain.AppError)
	UpdateAdminUserStatusWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input auth.AdminUserStatusInput, buildCompletion auth.AdminUserCompletionBuilder) (idempotency.Completion, *domain.AppError)
	UpdateAdminUserPermissionWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input auth.AdminUserPermissionInput, buildCompletion auth.AdminUserCompletionBuilder) (idempotency.Completion, *domain.AppError)
}

type OperationAuditService interface {
	AdminOperationAuditLogs(ctx context.Context, user auth.User, filter operationaudit.Filter) (domain.Page[operationaudit.Entry], *domain.AppError)
}

type APIPromotionService interface {
	PublicAPIPromotions(ctx context.Context, placement string) ([]apipromotion.Promotion, *domain.AppError)
	AdminAPIPromotions(ctx context.Context, user auth.User) ([]apipromotion.Promotion, *domain.AppError)
	APIPromotionAvailability(ctx context.Context, user auth.User, input apipromotion.AvailabilityInput) (apipromotion.Availability, *domain.AppError)
	CreateAPIPromotionWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input apipromotion.CreateInput, buildCompletion apipromotion.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	StopAPIPromotionWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input apipromotion.StopInput, buildCompletion apipromotion.CompletionBuilder) (idempotency.Completion, *domain.AppError)
}

type GrowthService interface {
	AdminGrowthOverview(ctx context.Context, user auth.User, windowDays int) (growth.Overview, *domain.AppError)
	RecordAuthenticatedActivity(ctx context.Context, userID string) *domain.AppError
}

type PublicProfileService interface {
	PublicUserProfileBundle(ctx context.Context, username string) (profile.PublicUserProfileBundle, *domain.AppError)
}

type AccountGovernanceService interface {
	AccountGovernanceBusinessCenter(ctx context.Context, actor auth.BusinessActor) (accountgovernance.Center, *domain.AppError)
}

type APIOrderContinuityService interface {
	APIOrdersForActor(ctx context.Context, actor auth.BusinessActor, participantRole string) ([]apiorder.Order, *domain.AppError)
	APIOrderForActor(ctx context.Context, actor auth.BusinessActor, orderID, participantRole string) (apiorder.Order, *domain.AppError)
	ConfirmAPIOrderCompleteForActorWithIdempotency(ctx context.Context, actor auth.BusinessActor, routeKey, key, requestHash string, input apiorder.ActionInput, buildCompletion apiorder.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	OpenAPIOrderDisputeForActorWithIdempotency(ctx context.Context, actor auth.BusinessActor, routeKey, key, requestHash string, input apiorder.ActionInput, buildCompletion apiorder.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	ConfirmAPIOrderPaymentForActorWithIdempotency(ctx context.Context, actor auth.BusinessActor, routeKey, key, requestHash string, input apiorder.ActionInput, buildCompletion apiorder.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	ReportAPIOrderPaymentIssueForActorWithIdempotency(ctx context.Context, actor auth.BusinessActor, routeKey, key, requestHash string, input apiorder.ActionInput, buildCompletion apiorder.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	SubmitAPIOrderDeliveryForActorWithIdempotency(ctx context.Context, actor auth.BusinessActor, routeKey, key, requestHash string, input apiorder.ActionInput, buildCompletion apiorder.CompletionBuilder) (idempotency.Completion, *domain.AppError)
}

type CarpoolContinuityService interface {
	CarpoolApplicationsForActor(ctx context.Context, actor auth.BusinessActor, participantRole string) ([]carpool.Application, *domain.AppError)
	CarpoolApplicationForActor(ctx context.Context, actor auth.BusinessActor, applicationID, participantRole string) (carpool.Application, *domain.AppError)
	CarpoolMembershipsForActor(ctx context.Context, actor auth.BusinessActor, participantRole string) ([]carpool.Membership, *domain.AppError)
	ConfirmCarpoolMembershipCompleteForActorWithIdempotency(ctx context.Context, actor auth.BusinessActor, routeKey, key, requestHash string, input carpool.ConfirmMembershipCompleteInput, buildCompletion carpool.MembershipCompletionBuilder) (idempotency.Completion, *domain.AppError)
	EndCarpoolMembershipForActorWithIdempotency(ctx context.Context, actor auth.BusinessActor, routeKey, key, requestHash string, input carpool.EndMembershipInput, buildCompletion carpool.MembershipCompletionBuilder) (idempotency.Completion, *domain.AppError)
}

type DisputeContinuityService interface {
	DisputesForActor(ctx context.Context, actor auth.BusinessActor) ([]report.DisputeCase, *domain.AppError)
	DisputeForActor(ctx context.Context, actor auth.BusinessActor, id string) (report.DisputeCase, *domain.AppError)
	DisputeParticipantActionForActorWithIdempotency(ctx context.Context, actor auth.BusinessActor, routeKey, key, requestHash string, input report.DisputeParticipantActionInput, buildCompletion report.DisputeParticipantCompletionBuilder) (idempotency.Completion, *domain.AppError)
	SubmitInfoSupplementForActorWithIdempotency(ctx context.Context, actor auth.BusinessActor, routeKey, key, requestHash string, input report.SupplementInput, buildCompletion report.SupplementCompletionBuilder) (idempotency.Completion, *domain.AppError)
}

type PromotionRewardService interface {
	PromotionRewardPublicConfig(ctx context.Context) (promotionreward.PublicConfig, *domain.AppError)
	MyReferralSummary(ctx context.Context, user auth.User) (promotionreward.ReferralSummary, *domain.AppError)
	MyPromotionCoupons(ctx context.Context, user auth.User, query promotionreward.CouponQuery) (promotionreward.CouponPage, *domain.AppError)
	ApplyPromotionCouponWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input promotionreward.ApplyCouponInput, buildCompletion promotionreward.CouponCompletionBuilder) (idempotency.Completion, *domain.AppError)
	AdminPromotionRewardCampaign(ctx context.Context, user auth.User) (promotionreward.Campaign, *domain.AppError)
	UpdateAdminPromotionRewardCampaignWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input promotionreward.UpdateCampaignInput, buildCompletion promotionreward.CampaignCompletionBuilder) (idempotency.Completion, *domain.AppError)
	AdminReferrals(ctx context.Context, user auth.User, query promotionreward.ReferralQuery) (promotionreward.ReferralPage, *domain.AppError)
	RevokeAdminReferralWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input promotionreward.RevokeReferralInput, buildCompletion promotionreward.ReferralCompletionBuilder) (idempotency.Completion, *domain.AppError)
	AdminPromotionCoupons(ctx context.Context, user auth.User, query promotionreward.CouponQuery) (promotionreward.CouponPage, *domain.AppError)
	GrantAdminPromotionCouponWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input promotionreward.GrantCouponInput, buildCompletion promotionreward.CouponCompletionBuilder) (idempotency.Completion, *domain.AppError)
	RevokeAdminPromotionCouponWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input promotionreward.RevokeCouponInput, buildCompletion promotionreward.CouponCompletionBuilder) (idempotency.Completion, *domain.AppError)
}

// Service is the legacy application facade for handlers that have not yet been
// moved to domain-specific server dependencies.
type Service interface {
	CreateDevSession(ctx context.Context, username string, isAdmin bool) (auth.User, auth.Session, *domain.AppError)
	LoginWithOAuthProfile(ctx context.Context, profile auth.OAuthProfile) (auth.User, auth.Session, *domain.AppError)
	AuthenticateWithOAuthProfile(ctx context.Context, profile auth.OAuthProfile) (auth.AuthenticationResult, *domain.AppError)
	StartRestrictedBusinessOAuth(ctx context.Context) (string, *domain.AppError)
	CompleteRestrictedBusinessOAuth(ctx context.Context, state string, profile auth.OAuthProfile) (auth.AuthenticationResult, *domain.AppError)
	StartAccountAppealOAuth(ctx context.Context) (string, *domain.AppError)
	CompleteAccountAppealOAuth(ctx context.Context, state string, profile auth.OAuthProfile) (auth.User, auth.AccountAppealSession, *domain.AppError)
	StartAccountAppealSession(ctx context.Context, profile auth.OAuthProfile) (auth.User, auth.AccountAppealSession, *domain.AppError)
	GetAccountAppealSession(ctx context.Context, sessionID string) (auth.User, auth.AccountAppealSession, *domain.AppError)
	GetAccountAppealSessionWithCSRF(ctx context.Context, sessionID, csrfToken string) (auth.User, auth.AccountAppealSession, *domain.AppError)
	LoginWithPassword(ctx context.Context, username, password string) (auth.User, auth.Session, *domain.AppError)
	AuthenticateWithPassword(ctx context.Context, username, password string) (auth.AuthenticationResult, *domain.AppError)
	StudentRegistrationConfig(ctx context.Context) (auth.StudentRegistrationConfig, *domain.AppError)
	UsernameAvailable(ctx context.Context, username string) (bool, *domain.AppError)
	AdminStudentRegistration(ctx context.Context, user auth.User) (auth.StudentRegistrationConfig, *domain.AppError)
	UpdateAdminStudentRegistrationWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input auth.StudentRegistrationSettingUpdate, build auth.StudentRegistrationCompletionBuilder) (idempotency.Completion, *domain.AppError)
	AdminStudentInstitutionDomains(ctx context.Context, user auth.User) ([]auth.StudentInstitutionDomain, *domain.AppError)
	CreateStudentInstitutionDomainWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input auth.StudentInstitutionDomainCreateInput, build auth.StudentInstitutionDomainCompletionBuilder) (idempotency.Completion, *domain.AppError)
	UpdateStudentInstitutionDomainWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input auth.StudentInstitutionDomainUpdateInput, build auth.StudentInstitutionDomainCompletionBuilder) (idempotency.Completion, *domain.AppError)
	StartEmailRegistration(ctx context.Context, input auth.EmailRegistrationStartInput) (auth.EmailRegistrationChallenge, *domain.AppError)
	ConfirmEmailRegistration(ctx context.Context, input auth.EmailRegistrationConfirmInput) (auth.User, auth.Session, *domain.AppError)
	StartPasswordReset(ctx context.Context, input auth.PasswordResetStartInput) (auth.PasswordResetStartResult, *domain.AppError)
	ConfirmPasswordReset(ctx context.Context, input auth.PasswordResetConfirmInput) *domain.AppError
	ReauthenticatePassword(ctx context.Context, sessionID, csrfToken, password string) *domain.AppError
	ReauthenticatePasswordForPurpose(ctx context.Context, sessionID, csrfToken, password, purpose string) *domain.AppError
	StartAdminReauthenticationOAuth(ctx context.Context, sessionID string) (string, *domain.AppError)
	CompleteAdminReauthenticationOAuth(ctx context.Context, sessionID, state string, profile auth.OAuthProfile) *domain.AppError
	StartLinuxDoLink(ctx context.Context, sessionID string) (string, *domain.AppError)
	CompleteLinuxDoLink(ctx context.Context, sessionID, state string, profile auth.OAuthProfile) (auth.User, auth.Session, *domain.AppError)
	SetPassword(ctx context.Context, input auth.SetPasswordInput) *domain.AppError
	GetSession(ctx context.Context, sessionID string) (auth.User, auth.Session, *domain.AppError)
	GetSessionWithCSRF(ctx context.Context, sessionID, csrfToken string) (auth.User, auth.Session, *domain.AppError)
	GetRestrictedBusinessSession(ctx context.Context, sessionID string) (auth.User, auth.RestrictedBusinessSession, *domain.AppError)
	GetRestrictedBusinessSessionWithCSRF(ctx context.Context, sessionID, csrfToken string) (auth.User, auth.RestrictedBusinessSession, *domain.AppError)
	RefreshRestrictedBusinessSessionCSRF(ctx context.Context, sessionID string) (string, *domain.AppError)
	LogoutRestrictedBusinessSession(ctx context.Context, sessionID string)
	RenewSession(ctx context.Context, sessionID string) (auth.Session, bool, *domain.AppError)
	RefreshSessionCSRF(ctx context.Context, sessionID string) (string, *domain.AppError)
	Logout(ctx context.Context, sessionID string)
	BeginIdempotency(ctx context.Context, userID, routeKey, key, requestHash string) (*idempotency.Entry, *domain.AppError)
	CompleteIdempotency(ctx context.Context, entry *idempotency.Entry, status int, contentType string, body []byte, resourceType, resourceID string) *domain.AppError
	CancelIdempotency(ctx context.Context, entry *idempotency.Entry)

	ProductCategories(ctx context.Context) ([]catalog.ProductCategory, *domain.AppError)
	AdminProductCategories(ctx context.Context, user auth.User) ([]catalog.ProductCategory, *domain.AppError)
	AdminProductCategory(ctx context.Context, user auth.User, categoryID string) (catalog.ProductCategory, *domain.AppError)
	CreateProductCategory(ctx context.Context, user auth.User, input catalog.ProductCategoryInput) (catalog.ProductCategory, *domain.AppError)
	UpdateProductCategory(ctx context.Context, user auth.User, categoryID string, input catalog.ProductCategoryInput) (catalog.ProductCategory, *domain.AppError)
	ProductPlans(ctx context.Context, categoryCode string) ([]catalog.ProductPlan, *domain.AppError)
	ProductPlan(ctx context.Context, planID string) (catalog.ProductPlan, *domain.AppError)
	AdminProductPlans(ctx context.Context, user auth.User, categoryCode string) ([]catalog.ProductPlan, *domain.AppError)
	AdminProductPlan(ctx context.Context, user auth.User, planID string) (catalog.ProductPlan, *domain.AppError)
	CreateProductPlan(ctx context.Context, user auth.User, input catalog.ProductPlanInput) (catalog.ProductPlan, *domain.AppError)
	UpdateProductPlan(ctx context.Context, user auth.User, planID string, input catalog.ProductPlanInput) (catalog.ProductPlan, *domain.AppError)
	AdminAPIModelProviders(ctx context.Context, user auth.User) ([]catalog.APIModelProvider, *domain.AppError)
	AdminAPIModelProvider(ctx context.Context, user auth.User, providerID string) (catalog.APIModelProvider, *domain.AppError)
	CreateAPIModelProvider(ctx context.Context, user auth.User, input catalog.APIModelProviderInput) (catalog.APIModelProvider, *domain.AppError)
	UpdateAPIModelProvider(ctx context.Context, user auth.User, providerID string, input catalog.APIModelProviderInput) (catalog.APIModelProvider, *domain.AppError)
	APIModels(ctx context.Context) ([]catalog.APIModelCatalog, *domain.AppError)
	APIModel(ctx context.Context, modelID string) (catalog.APIModelCatalog, *domain.AppError)
	AdminAPIModels(ctx context.Context, user auth.User) ([]catalog.APIModelCatalog, *domain.AppError)
	AdminAPIModel(ctx context.Context, user auth.User, modelID string) (catalog.APIModelCatalog, *domain.AppError)
	CreateAPIModel(ctx context.Context, user auth.User, input catalog.APIModelInput) (catalog.APIModelCatalog, *domain.AppError)
	UpdateAPIModel(ctx context.Context, user auth.User, modelID string, input catalog.APIModelInput) (catalog.APIModelCatalog, *domain.AppError)
	ApplyCatalogLifecycleWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input catalog.LifecycleActionInput, buildCompletion catalog.LifecycleCompletionBuilder) (idempotency.Completion, *domain.AppError)
	PreviewAPIModelSync(ctx context.Context, user auth.User, input catalog.APIModelSyncPreviewInput) (catalog.APIModelSyncPreview, *domain.AppError)
	ApplyAPIModelSyncWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input catalog.APIModelSyncApplyInput, buildCompletion catalog.APIModelSyncCompletionBuilder) (idempotency.Completion, *domain.AppError)

	AdminModelAuditTargets(ctx context.Context, user auth.User) ([]modelaudit.Target, *domain.AppError)
	AdminModelAuditTarget(ctx context.Context, user auth.User, targetID string) (modelaudit.Target, *domain.AppError)
	CreateModelAuditTarget(ctx context.Context, user auth.User, input modelaudit.TargetInput) (modelaudit.Target, *domain.AppError)
	UpdateModelAuditTarget(ctx context.Context, user auth.User, targetID string, input modelaudit.TargetInput) (modelaudit.Target, *domain.AppError)
	DeleteModelAuditTarget(ctx context.Context, user auth.User, targetID string) *domain.AppError
	AdminModelAuditBaselines(ctx context.Context, user auth.User) ([]modelaudit.Baseline, *domain.AppError)
	AdminModelAuditBaseline(ctx context.Context, user auth.User, baselineID string) (modelaudit.Baseline, *domain.AppError)
	CreateModelAuditBaseline(ctx context.Context, user auth.User, input modelaudit.BaselineInput) (modelaudit.Baseline, *domain.AppError)
	AdminModelAuditRuns(ctx context.Context, user auth.User) ([]modelaudit.Run, *domain.AppError)
	AdminModelAuditRun(ctx context.Context, user auth.User, runID string) (modelaudit.Run, *domain.AppError)
	CreateModelAuditRun(ctx context.Context, user auth.User, input modelaudit.RunInput) (modelaudit.Run, *domain.AppError)
	CancelModelAuditRun(ctx context.Context, user auth.User, runID string) (modelaudit.Run, *domain.AppError)
	AdminModelAuditReport(ctx context.Context, user auth.User, runID string) (modelaudit.AuditReport, *domain.AppError)
	AdminModelAuditMonitors(ctx context.Context, user auth.User) ([]modelaudit.Monitor, *domain.AppError)
	CreateModelAuditMonitor(ctx context.Context, user auth.User, input modelaudit.MonitorInput) (modelaudit.Monitor, *domain.AppError)

	SubmitOfficialPriceLead(ctx context.Context, user auth.User, input officialprice.SubmitLeadInput) (officialprice.Lead, *domain.AppError)
	MyOfficialPriceLeads(ctx context.Context, user auth.User) ([]officialprice.Lead, *domain.AppError)
	MyOfficialPriceLead(ctx context.Context, user auth.User, leadID string) (officialprice.Lead, *domain.AppError)
	AdminOfficialPriceLeads(ctx context.Context, user auth.User) ([]officialprice.Lead, *domain.AppError)
	AdminOfficialPriceLead(ctx context.Context, user auth.User, leadID string) (officialprice.Lead, *domain.AppError)
	AdminOfficialPriceRecords(ctx context.Context, user auth.User) ([]officialprice.Record, *domain.AppError)
	AdminOfficialPriceRecord(ctx context.Context, user auth.User, recordID string) (officialprice.Record, *domain.AppError)
	CreateAdminOfficialPriceRecord(ctx context.Context, user auth.User, input officialprice.AdminRecordInput) (officialprice.Record, *domain.AppError)
	UpdateAdminOfficialPriceRecord(ctx context.Context, user auth.User, input officialprice.AdminRecordInput) (officialprice.Record, *domain.AppError)
	TakeDownAdminOfficialPriceRecord(ctx context.Context, user auth.User, input officialprice.AdminRecordActionInput) (officialprice.Record, *domain.AppError)
	ApproveOfficialPriceLeadWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input officialprice.ApproveLeadInput, buildCompletion officialprice.ApprovalCompletionBuilder) (idempotency.Completion, *domain.AppError)
	UpdateLeadReviewStatus(ctx context.Context, user auth.User, leadID, status, reason string, ifMatchVersion int64) (officialprice.Lead, *domain.AppError)
	PublicOfficialPriceRecords(ctx context.Context) ([]officialprice.Record, *domain.AppError)
	PublicOfficialPriceRecord(ctx context.Context, recordID string) (officialprice.Record, *domain.AppError)

	SearchMarket(ctx context.Context, keyword string) ([]search.Result, *domain.AppError)
	CreateFeedbackTicketWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input feedback.CreateInput, buildCompletion feedback.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	MyFeedbackTickets(ctx context.Context, user auth.User, page domain.PageRequest) (domain.Page[feedback.Ticket], *domain.AppError)
	MyFeedbackTicket(ctx context.Context, user auth.User, id string) (feedback.Ticket, *domain.AppError)
	AddFeedbackSupplementWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input feedback.SupplementInput, buildCompletion feedback.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	MarkFeedbackRead(ctx context.Context, user auth.User, id string) (feedback.Ticket, *domain.AppError)
	MyFeedbackUnreadCount(ctx context.Context, user auth.User) (int, *domain.AppError)
	AdminFeedbackTickets(ctx context.Context, user auth.User) ([]feedback.Ticket, *domain.AppError)
	AdminFeedbackTicket(ctx context.Context, user auth.User, id string) (feedback.Ticket, *domain.AppError)
	AdminHandleFeedbackTicketWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input feedback.AdminHandleInput, buildCompletion feedback.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	MyFavorites(ctx context.Context, user auth.User) ([]favorite.ListItem, *domain.AppError)
	IsFavorite(ctx context.Context, user auth.User, targetType, targetID string) (bool, *domain.AppError)
	CreateFavoriteWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash, targetType, targetID string, buildCompletion favorite.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	DeleteFavorite(ctx context.Context, user auth.User, targetType, targetID string) (favorite.MutationResult, *domain.AppError)
	ListMyReviewCenterRows(ctx context.Context, user auth.User) ([]review.ReviewCenterRow, *domain.AppError)
	SubmitTransactionReviewWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input review.SubmitReviewInput, buildCompletion review.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	SubmitCarpoolMembershipReviewWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input review.SubmitReviewInput, buildCompletion review.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	AdminRemoveTransactionReviewWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input review.RemoveReviewInput, buildCompletion review.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	PublicUserReviews(ctx context.Context, username string) ([]review.PublicReview, *domain.AppError)
	CreateReportWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input report.CreateReportInput, buildCompletion report.ReportCompletionBuilder) (idempotency.Completion, *domain.AppError)
	MyReports(ctx context.Context, user auth.User) ([]report.Report, *domain.AppError)
	AdminReports(ctx context.Context, user auth.User, page domain.PageRequest) (domain.Page[report.Report], *domain.AppError)
	AdminReport(ctx context.Context, user auth.User, id string) (report.Report, *domain.AppError)
	AdminReportActionWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input report.AdminActionInput, buildCompletion report.AdminCompletionBuilder) (idempotency.Completion, *domain.AppError)
	SubmitInfoSupplementWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input report.SupplementInput, buildCompletion report.SupplementCompletionBuilder) (idempotency.Completion, *domain.AppError)
	MyDisputes(ctx context.Context, user auth.User) ([]report.DisputeCase, *domain.AppError)
	MyDispute(ctx context.Context, user auth.User, id string) (report.DisputeCase, *domain.AppError)
	DisputeParticipantActionWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input report.DisputeParticipantActionInput, buildCompletion report.DisputeParticipantCompletionBuilder) (idempotency.Completion, *domain.AppError)
	AdminDisputes(ctx context.Context, user auth.User) ([]report.DisputeCase, *domain.AppError)
	AdminDispute(ctx context.Context, user auth.User, id string) (report.DisputeCase, *domain.AppError)
	AdminDisputeActionWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input report.AdminActionInput, buildCompletion report.AdminCompletionBuilder) (idempotency.Completion, *domain.AppError)
	CreateAppealWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input report.CreateAppealInput, buildCompletion report.AppealCompletionBuilder) (idempotency.Completion, *domain.AppError)
	CreateAccountGovernanceAppealWithIdempotency(ctx context.Context, appellantUserID, routeKey, key, requestHash string, input report.CreateAccountGovernanceAppealInput, buildCompletion report.AppealCompletionBuilder) (idempotency.Completion, *domain.AppError)
	MyAppeals(ctx context.Context, user auth.User) ([]report.Appeal, *domain.AppError)
	AdminAppeals(ctx context.Context, user auth.User) ([]report.Appeal, *domain.AppError)
	AdminAppeal(ctx context.Context, user auth.User, id string) (report.Appeal, *domain.AppError)
	AdminAppealActionWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input report.AdminActionInput, buildCompletion report.AdminCompletionBuilder) (idempotency.Completion, *domain.AppError)
	PublicUserDisputes(ctx context.Context, username string) ([]report.PublicDispute, *domain.AppError)

	CreateAPIService(ctx context.Context, user auth.User, input apimarket.CreateServiceInput) (apimarket.Service, *domain.AppError)
	CreateAPIServiceWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input apimarket.CreateServiceInput, buildCompletion apimarket.ServiceCompletionBuilder) (idempotency.Completion, *domain.AppError)
	UpdateAPIService(ctx context.Context, user auth.User, input apimarket.UpdateServiceInput) (apimarket.Service, *domain.AppError)
	UpdateAPIServiceWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input apimarket.UpdateServiceInput, buildCompletion apimarket.ServiceCompletionBuilder) (idempotency.Completion, *domain.AppError)
	UpdateAPIServiceProbeConnection(ctx context.Context, user auth.User, input apimarket.UpdateProbeConnectionInput) (apimarket.Service, *domain.AppError)
	UpdateAPIServiceProbeConnectionWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input apimarket.UpdateProbeConnectionInput, buildCompletion apimarket.ServiceCompletionBuilder) (idempotency.Completion, *domain.AppError)
	PublicAPIServices(ctx context.Context, filter apimarket.PublicServiceFilter, page domain.PageRequest) (domain.Page[apimarket.Service], *domain.AppError)
	PublicAPIService(ctx context.Context, serviceID string) (apimarket.Service, *domain.AppError)
	OwnerAPIServices(ctx context.Context, user auth.User, filter apimarket.OwnerServiceFilter, page domain.PageRequest) (domain.Page[apimarket.Service], *domain.AppError)
	OwnerAPIService(ctx context.Context, user auth.User, serviceID string) (apimarket.Service, *domain.AppError)
	AdminAPIServices(ctx context.Context, user auth.User, filter apimarket.AdminServiceFilter, page domain.PageRequest) (domain.Page[apimarket.Service], *domain.AppError)
	AdminAPIService(ctx context.Context, user auth.User, serviceID string) (apimarket.Service, *domain.AppError)
	SubmitAPIServiceForReview(ctx context.Context, user auth.User, input apimarket.ServiceOwnerActionInput) (apimarket.Service, *domain.AppError)
	SubmitAPIServiceForReviewWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input apimarket.ServiceOwnerActionInput, buildCompletion apimarket.ServiceCompletionBuilder) (idempotency.Completion, *domain.AppError)
	UpdateAPIServicePublication(ctx context.Context, user auth.User, input apimarket.ServiceOwnerActionInput, action string) (apimarket.Service, *domain.AppError)
	UpdateAPIServicePublicationWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input apimarket.ServiceOwnerActionInput, action string, buildCompletion apimarket.ServiceCompletionBuilder) (idempotency.Completion, *domain.AppError)
	UpdateAPIServiceAdminStatus(ctx context.Context, user auth.User, input apimarket.ServiceAdminActionInput) (apimarket.Service, *domain.AppError)
	UpdateAPIServiceAdminStatusWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input apimarket.ServiceAdminActionInput, buildCompletion apimarket.ServiceCompletionBuilder) (idempotency.Completion, *domain.AppError)
	UpdateAPIServiceOrderSettings(ctx context.Context, user auth.User, input apimarket.UpdateOrderSettingsInput) (apimarket.Service, *domain.AppError)
	UpdateAPIServiceOrderSettingsWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input apimarket.UpdateOrderSettingsInput, buildCompletion apimarket.ServiceCompletionBuilder) (idempotency.Completion, *domain.AppError)

	CreateAPIPurchaseIntentWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input apiintent.CreateIntentInput, buildCompletion apiintent.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	MyAPIPurchaseIntents(ctx context.Context, user auth.User) ([]apiintent.Intent, *domain.AppError)
	MyAPIPurchaseIntent(ctx context.Context, user auth.User, intentID, requestID string) (apiintent.Intent, *domain.AppError)
	CancelAPIPurchaseIntentWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input apiintent.ActionInput, buildCompletion apiintent.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	OwnerAPIPurchaseIntents(ctx context.Context, user auth.User) ([]apiintent.Intent, *domain.AppError)
	OwnerAPIPurchaseIntent(ctx context.Context, user auth.User, intentID, requestID string) (apiintent.Intent, *domain.AppError)
	MarkAPIPurchaseIntentContactedWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input apiintent.ActionInput, buildCompletion apiintent.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	CloseAPIPurchaseIntentWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input apiintent.ActionInput, buildCompletion apiintent.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	AdminAPIPurchaseIntents(ctx context.Context, user auth.User) ([]apiintent.Intent, *domain.AppError)
	AdminAPIPurchaseIntent(ctx context.Context, user auth.User, intentID string) (apiintent.Intent, *domain.AppError)

	CreateAPIOrderWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input apiorder.ActionInput, createInput apiorder.CreateInput, buildCompletion apiorder.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	MyAPIOrders(ctx context.Context, user auth.User) ([]apiorder.Order, *domain.AppError)
	MyAPIOrder(ctx context.Context, user auth.User, orderID string) (apiorder.Order, *domain.AppError)
	ReadAPIOrderPaymentInstructions(ctx context.Context, user auth.User, orderID, requestID string) (apiorder.PaymentInstructionsView, *domain.AppError)
	SubmitAPIOrderPaymentWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input apiorder.ActionInput, buildCompletion apiorder.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	CancelAPIOrderWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input apiorder.ActionInput, buildCompletion apiorder.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	ConfirmAPIOrderCompleteWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input apiorder.ActionInput, buildCompletion apiorder.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	OpenAPIOrderDisputeWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input apiorder.ActionInput, buildCompletion apiorder.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	OpenOwnerAPIOrderDisputeWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input apiorder.ActionInput, buildCompletion apiorder.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	OwnerAPIOrders(ctx context.Context, user auth.User) ([]apiorder.Order, *domain.AppError)
	AdminAPIOrders(ctx context.Context, user auth.User, filter apiorder.AdminOrderFilter, page domain.PageRequest) (domain.Page[apiorder.Order], *domain.AppError)
	AdminAPIOrder(ctx context.Context, user auth.User, orderID string) (apiorder.Order, *domain.AppError)
	ResolveAPIOrderCatalogRiskHoldWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input apiorder.CatalogRiskHoldActionInput, buildCompletion apiorder.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	OwnerAPIOrder(ctx context.Context, user auth.User, orderID string) (apiorder.Order, *domain.AppError)
	ConfirmAPIOrderPaymentWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input apiorder.ActionInput, buildCompletion apiorder.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	ReportAPIOrderPaymentIssueWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input apiorder.ActionInput, buildCompletion apiorder.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	SubmitAPIOrderDeliveryWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input apiorder.ActionInput, buildCompletion apiorder.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	ReportLateAPIOrderPaymentWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input apiorder.ActionInput, buildCompletion apiorder.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	ResolveLateAPIOrderPaymentWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input apiorder.ActionInput, buildCompletion apiorder.CompletionBuilder) (idempotency.Completion, *domain.AppError)

	CreateContactMethod(ctx context.Context, input contact.ContactMethodInput) (contact.ContactMethod, *domain.AppError)
	CreateContactMethodWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input contact.ContactMethodInput, buildCompletion contact.MethodCompletionBuilder) (idempotency.Completion, *domain.AppError)
	ListContactMethods(ctx context.Context, userID string) ([]contact.ContactMethod, *domain.AppError)
	UpdateContactMethod(ctx context.Context, input contact.UpdateContactMethodInput) (contact.ContactMethod, *domain.AppError)
	UpdateContactMethodWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input contact.UpdateContactMethodInput, buildCompletion contact.MethodCompletionBuilder) (idempotency.Completion, *domain.AppError)
	DeleteContactMethod(ctx context.Context, userID, methodID string) (contact.ContactMethod, *domain.AppError)
	DeleteContactMethodWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash, methodID, requestID string, buildCompletion contact.MethodCompletionBuilder) (idempotency.Completion, *domain.AppError)
	SetDefaultContactMethod(ctx context.Context, userID, methodID string) (contact.ContactMethod, *domain.AppError)
	SetDefaultContactMethodWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash, methodID, requestID string, buildCompletion contact.MethodCompletionBuilder) (idempotency.Completion, *domain.AppError)
	VerifyContactMethod(ctx context.Context, userID, methodID string) (contact.ContactMethod, *domain.AppError)
	VerifyContactMethodWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash, methodID, requestID string, buildCompletion contact.MethodCompletionBuilder) (idempotency.Completion, *domain.AppError)
	CreateContactSession(ctx context.Context, input contact.CreateContactSessionInput) (contact.ContactSession, *domain.AppError)
	ReadContactSession(ctx context.Context, sessionID, viewerUserID, requestID string) (contact.ContactSessionView, *domain.AppError)

	MyProfile(ctx context.Context, user auth.User) (profile.UserProfile, *domain.AppError)
	UpdateMyProfile(ctx context.Context, user auth.User, input profile.UpdateUserProfileInput) (profile.UserProfile, *domain.AppError)
	StartEmailVerification(ctx context.Context, user auth.User, input profile.EmailVerificationStartInput) (profile.EmailVerificationChallenge, *domain.AppError)
	ConfirmEmailVerification(ctx context.Context, user auth.User, input profile.EmailVerificationConfirmInput) (profile.UserProfile, *domain.AppError)
	PublicUserProfile(ctx context.Context, username string) (profile.PublicUserProfile, *domain.AppError)
	MyMerchantProfile(ctx context.Context, user auth.User) (profile.MerchantProfile, *domain.AppError)
	UpsertMyMerchantProfile(ctx context.Context, user auth.User, input profile.UpsertMerchantProfileInput) (profile.MerchantProfile, *domain.AppError)
	PublicMerchantProfile(ctx context.Context, slug string) (profile.PublicMerchantProfile, *domain.AppError)

	UserAnnouncements(ctx context.Context, user auth.User) ([]announcement.Announcement, *domain.AppError)
	ActiveAnnouncements(ctx context.Context, user auth.User, channel string) ([]announcement.Announcement, *domain.AppError)
	HomeAnnouncement(ctx context.Context, user auth.User) (*announcement.Announcement, *domain.AppError)
	UserAnnouncementBySlug(ctx context.Context, user auth.User, slug string) (announcement.Announcement, *domain.AppError)
	AnnouncementUnreadCount(ctx context.Context, user auth.User, importantOnly bool) (int, *domain.AppError)
	MarkAnnouncementSeen(ctx context.Context, user auth.User, id string) (announcement.Receipt, *domain.AppError)
	MarkAnnouncementRead(ctx context.Context, user auth.User, id string) (announcement.Receipt, *domain.AppError)
	DismissAnnouncement(ctx context.Context, user auth.User, id string) (announcement.Receipt, *domain.AppError)
	AdminAnnouncements(ctx context.Context, user auth.User) ([]announcement.Announcement, *domain.AppError)
	AdminAnnouncement(ctx context.Context, user auth.User, id string) (announcement.Announcement, *domain.AppError)
	CreateAnnouncement(ctx context.Context, user auth.User, input announcement.FormInput) (announcement.Announcement, *domain.AppError)
	UpdateAnnouncement(ctx context.Context, user auth.User, id string, input announcement.FormInput) (announcement.Announcement, *domain.AppError)
	PublishAnnouncement(ctx context.Context, user auth.User, id string) (announcement.Announcement, *domain.AppError)
	OfflineAnnouncement(ctx context.Context, user auth.User, id, reason string) (announcement.Announcement, *domain.AppError)
	DuplicateAnnouncement(ctx context.Context, user auth.User, id string) (announcement.Announcement, *domain.AppError)
	AnnouncementAuditLogs(ctx context.Context, user auth.User) ([]announcement.AuditLog, *domain.AppError)

	MyNotifications(ctx context.Context, user auth.User, page domain.PageRequest) (domain.Page[notification.Notification], *domain.AppError)
	MyNotificationUnreadCount(ctx context.Context, user auth.User) (int, *domain.AppError)
	MarkNotificationRead(ctx context.Context, user auth.User, id string) (notification.Notification, *domain.AppError)
	MarkAllNotificationsRead(ctx context.Context, user auth.User) (notification.ReadAllResult, *domain.AppError)
}

// CarpoolService is the server transport boundary for carpool handlers.
type CarpoolService interface {
	CreateCarpoolListing(ctx context.Context, user auth.User, input carpool.CreateListingInput) (carpool.Listing, *domain.AppError)
	CreateCarpoolListingWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input carpool.CreateListingInput, buildCompletion carpool.ListingCompletionBuilder) (idempotency.Completion, *domain.AppError)
	PublishCarpoolListing(ctx context.Context, user auth.User, input carpool.PublishListingInput) (carpool.Listing, *domain.AppError)
	PublishCarpoolListingWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input carpool.PublishListingInput, buildCompletion carpool.ListingCompletionBuilder) (idempotency.Completion, *domain.AppError)
	UpdateCarpoolListing(ctx context.Context, user auth.User, input carpool.UpdateListingInput) (carpool.Listing, *domain.AppError)
	UpdateCarpoolListingWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input carpool.UpdateListingInput, buildCompletion carpool.ListingCompletionBuilder) (idempotency.Completion, *domain.AppError)
	SubmitCarpoolListingForReview(ctx context.Context, user auth.User, input carpool.SubmitListingReviewInput) (carpool.Listing, *domain.AppError)
	SubmitCarpoolListingForReviewWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input carpool.SubmitListingReviewInput, buildCompletion carpool.ListingCompletionBuilder) (idempotency.Completion, *domain.AppError)
	PublicCarpoolListings(ctx context.Context, filter carpool.ListingFilter, page domain.PageRequest) (domain.Page[carpool.Listing], *domain.AppError)
	PublicCarpoolListing(ctx context.Context, listingID string) (carpool.Listing, *domain.AppError)
	CarpoolApplicationEligibility(ctx context.Context, user auth.User, listingID string) (carpool.ApplicationEligibility, *domain.AppError)
	MyCarpoolListings(ctx context.Context, user auth.User, view string, page domain.PageRequest) (domain.Page[carpool.Listing], *domain.AppError)
	MyCarpoolListing(ctx context.Context, user auth.User, listingID string) (carpool.Listing, *domain.AppError)
	AdminCarpoolListings(ctx context.Context, user auth.User, filter carpool.ListingFilter, page domain.PageRequest) (domain.Page[carpool.Listing], *domain.AppError)
	AdminCarpoolListing(ctx context.Context, user auth.User, listingID string) (carpool.Listing, *domain.AppError)
	UpdateCarpoolListingReviewStatus(ctx context.Context, user auth.User, input carpool.ReviewInput) (carpool.Listing, *domain.AppError)
	UpdateCarpoolListingReviewStatusWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input carpool.ReviewInput, buildCompletion carpool.ListingCompletionBuilder) (idempotency.Completion, *domain.AppError)
	CreateCarpoolApplication(ctx context.Context, user auth.User, input carpool.CreateApplicationInput) (carpool.Application, *domain.AppError)
	CreateCarpoolApplicationWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input carpool.CreateApplicationInput, buildCompletion carpool.ApplicationCompletionBuilder) (idempotency.Completion, *domain.AppError)
	MyCarpoolApplications(ctx context.Context, user auth.User) ([]carpool.Application, *domain.AppError)
	MyCarpoolApplication(ctx context.Context, user auth.User, applicationID string) (carpool.Application, *domain.AppError)
	OwnerCarpoolApplications(ctx context.Context, user auth.User) ([]carpool.Application, *domain.AppError)
	OwnerCarpoolApplication(ctx context.Context, user auth.User, applicationID string) (carpool.Application, *domain.AppError)
	AcceptCarpoolApplicationWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input carpool.AcceptApplicationInput, buildCompletion carpool.ApplicationCompletionBuilder) (idempotency.Completion, *domain.AppError)
	RejectCarpoolApplication(ctx context.Context, user auth.User, input carpool.RejectApplicationInput) (carpool.Application, *domain.AppError)
	RejectCarpoolApplicationWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input carpool.RejectApplicationInput, buildCompletion carpool.ApplicationCompletionBuilder) (idempotency.Completion, *domain.AppError)
	CancelCarpoolApplicationWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input carpool.CancelApplicationInput, buildCompletion carpool.ApplicationCompletionBuilder) (idempotency.Completion, *domain.AppError)
	WithdrawCarpoolAcceptanceWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input carpool.WithdrawAcceptanceInput, buildCompletion carpool.ApplicationCompletionBuilder) (idempotency.Completion, *domain.AppError)
	ConfirmCarpoolApplicationJoinWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input carpool.ConfirmApplicationJoinInput, buildCompletion carpool.ApplicationCompletionBuilder) (idempotency.Completion, *domain.AppError)
	MyCarpoolMemberships(ctx context.Context, user auth.User) ([]carpool.Membership, *domain.AppError)
	OwnerCarpoolMemberships(ctx context.Context, user auth.User) ([]carpool.Membership, *domain.AppError)
	ConfirmCarpoolMembershipCompleteWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input carpool.ConfirmMembershipCompleteInput, buildCompletion carpool.MembershipCompletionBuilder) (idempotency.Completion, *domain.AppError)
	EndCarpoolMembershipWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input carpool.EndMembershipInput, buildCompletion carpool.MembershipCompletionBuilder) (idempotency.Completion, *domain.AppError)
}

type APIQuotaService interface {
	PublicAPIQuotaOffers(ctx context.Context, filter apiquota.PublicOfferFilter, page domain.PageRequest) (domain.Page[apiquota.OfferCard], *domain.AppError)
	APIQuotaSystemSaleSlots() []apiquota.SystemSaleSlot
	PublicAPIQuotaOffer(ctx context.Context, offerID string) (apiquota.OfferCard, *domain.AppError)
	OwnerAPIQuotaBatches(ctx context.Context, user auth.User, apiServiceID string, page domain.PageRequest) (domain.Page[apiquota.Batch], *domain.AppError)
	CreateAPIQuotaBatch(ctx context.Context, user auth.User, input apiquota.CreateBatchInput) (apiquota.Batch, *domain.AppError)
	CreateAPIQuotaBatchWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input apiquota.CreateBatchInput, buildCompletion apiquota.BatchCompletionBuilder) (idempotency.Completion, *domain.AppError)
	OwnerAPIQuotaOffers(ctx context.Context, user auth.User, batchID string) ([]apiquota.Offer, *domain.AppError)
	CreateAPIQuotaOffer(ctx context.Context, user auth.User, input apiquota.CreateOfferInput) (apiquota.Offer, *domain.AppError)
	CreateAPIQuotaOfferWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input apiquota.CreateOfferInput, buildCompletion apiquota.OfferCompletionBuilder) (idempotency.Completion, *domain.AppError)
	OwnerAPIQuotaRounds(ctx context.Context, user auth.User, batchID string) ([]apiquota.SaleRound, *domain.AppError)
	CreateAPIQuotaRound(ctx context.Context, user auth.User, input apiquota.CreateRoundInput) (apiquota.SaleRound, *domain.AppError)
	CreateAPIQuotaRoundWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input apiquota.CreateRoundInput, buildCompletion apiquota.SaleRoundCompletionBuilder) (idempotency.Completion, *domain.AppError)
	ConfirmAPIQuotaRoundFulfillmentWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input apiquota.SaleRoundActionInput, buildCompletion apiquota.SaleRoundCompletionBuilder) (idempotency.Completion, *domain.AppError)
	PublishAPIQuotaBatch(ctx context.Context, user auth.User, input apiquota.BatchActionInput) (apiquota.Batch, *domain.AppError)
	PublishAPIQuotaBatchWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input apiquota.BatchActionInput, buildCompletion apiquota.BatchCompletionBuilder) (idempotency.Completion, *domain.AppError)
	UpdateAPIQuotaBatchStatus(ctx context.Context, user auth.User, input apiquota.BatchActionInput, action string) (apiquota.Batch, *domain.AppError)
	UpdateAPIQuotaBatchStatusWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input apiquota.BatchActionInput, action string, buildCompletion apiquota.BatchCompletionBuilder) (idempotency.Completion, *domain.AppError)
	CreateAPIQuotaOrderWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input apiquota.CreateOrderInput, buildCompletion apiorder.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	ImportAPIQuotaCredentials(ctx context.Context, user auth.User, input apiquota.CredentialImportInput) (apiquota.CredentialImportResult, *domain.AppError)
	ImportAPIQuotaCredentialsWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input apiquota.CredentialImportInput, buildCompletion apiquota.CredentialImportCompletionBuilder) (idempotency.Completion, *domain.AppError)
	APIQuotaCredentialSummary(ctx context.Context, user auth.User, offerID string) (apiquota.CredentialSummary, *domain.AppError)
	CreateAPIQuotaRushOfferWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input apiquota.CreateRushOfferInput, buildCompletion apiquota.RushOfferCompletionBuilder) (idempotency.Completion, *domain.AppError)
}

type ReputationGovernanceService interface {
	ReputationAvailable() bool
	ReputationRules() reputation.RuleSet
	PublicUserReputation(ctx context.Context, username, scope string) ([]reputation.ReputationSnapshot, *domain.AppError)
	MyReputation(ctx context.Context, user auth.User) ([]reputation.ReputationSnapshot, *domain.AppError)
	AdminUserReputation(ctx context.Context, user auth.User, userID string, historyLimit int) (reputation.AdminReputationAudit, *domain.AppError)
	AdminRecalculateUserReputation(ctx context.Context, user auth.User, userID string) (reputation.RecalculationResult, *domain.AppError)
	AdminRecalculateAllReputation(ctx context.Context, user auth.User) (reputation.RecalculationResult, *domain.AppError)
	AdminSourceAuthorVerification(ctx context.Context, user auth.User, resourceType, resourceID string) (reputation.SourceAuthorVerificationAudit, *domain.AppError)
	AdminUpdateSourceAuthorVerification(ctx context.Context, user auth.User, input reputation.UpdateSourceAuthorVerificationInput) (reputation.SourceAuthorVerificationAudit, *domain.AppError)
	AdminCreateDisputeOutcomeWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input reputation.CreateOutcomeInput, buildCompletion reputation.GovernanceCompletionBuilder) (idempotency.Completion, *domain.AppError)
	AdminAPIOrderSanctionRecommendation(ctx context.Context, user auth.User, disputeCaseID string) (reputation.APIOrderSanctionRecommendation, *domain.AppError)
	AdminApplyAPIOrderSanctionWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input reputation.ApplyAPIOrderSanctionInput, buildCompletion reputation.GovernanceCompletionBuilder) (idempotency.Completion, *domain.AppError)
	MyActiveReputationRestrictions(ctx context.Context, user auth.User) ([]reputation.UserRestriction, *domain.AppError)
	AdminCreateUserRestrictionWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input reputation.CreateRestrictionInput, buildCompletion reputation.GovernanceCompletionBuilder) (idempotency.Completion, *domain.AppError)
	AdminRevokeUserRestrictionWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input reputation.RevokeRestrictionInput, buildCompletion reputation.GovernanceCompletionBuilder) (idempotency.Completion, *domain.AppError)
	CheckReputationActionAllowed(ctx context.Context, userID, role, action string) *domain.AppError
}

// ApplicationService is the constructor aggregate while handlers migrate from
// the legacy facade to domain-specific service boundaries.
type ApplicationService interface {
	Service
	DevPersonaSessionService
	CarpoolService
	APIQuotaService
	APIPaymentSettingsService
	AdminUserService
	OperationAuditService
	APIPromotionService
	GrowthService
	PromotionRewardService
	ReputationGovernanceService
	PublicProfileService
	AccountGovernanceService
	APIOrderContinuityService
	CarpoolContinuityService
	DisputeContinuityService
}

type Server struct {
	app                Service
	carpools           CarpoolService
	apiQuotas          APIQuotaService
	apiPayment         APIPaymentSettingsService
	apiHealth          APIHealthService
	adminAPIHealth     AdminAPIHealthService
	apiModelTester     APIModelTesterService
	adminUsers         AdminUserService
	operationAudit     OperationAuditService
	apiPromotions      APIPromotionService
	growth             GrowthService
	promotionRewards   PromotionRewardService
	reputation         ReputationGovernanceService
	publicProfiles     PublicProfileService
	accountGovernance  AccountGovernanceService
	apiOrderContinuity APIOrderContinuityService
	carpoolContinuity  CarpoolContinuityService
	disputeContinuity  DisputeContinuityService
	devPersonas        DevPersonaSessionService
	mux                chi.Router
	enableDevAuth      bool
	readinessChecker   health.Checker
	navigationBadges   NavigationBadgeService
	realtimeHub        *realtime.Hub
	oauth              OAuthOptions
	frontendOrigin     string
	cookieSecure       bool
	allowedOrigins     []string
	rateLimiter        *middleware.RateLimiter
	oauthHTTPClient    *http.Client
	clientIPResolver   middleware.ClientIPResolver
	metrics            *observability.Metrics
	metricsToken       string
	metricsAuth        bool
	turnstile          turnstile.Verifier
}

func NewServer(service ApplicationService, options ...ServerOptions) http.Handler {
	option := ServerOptions{EnableDevAuth: true, AppEnv: config.EnvDevelopment}
	if len(options) > 0 {
		option = options[0]
	}
	if option.AppEnv == "" {
		option.AppEnv = config.EnvDevelopment
	}
	navigationBadges := option.NavigationBadges
	if navigationBadges == nil {
		navigationBadges = navigationbadge.NewService(nil, time.Now)
	}
	apiModelTester := option.APIModelTester
	if apiModelTester == nil {
		apiModelTester = apimodeltest.NewService(nil, 15*time.Second, time.Now)
	}
	realtimeHub := option.RealtimeHub
	if realtimeHub == nil {
		realtimeHub = realtime.NewHub()
	}
	rateLimiter := option.RateLimiter
	if rateLimiter == nil {
		rateLimiter = middleware.NewRateLimiter(time.Minute)
	}
	metrics := option.Metrics
	if metrics == nil {
		metrics = observability.New(observability.Sources{RateLimiter: rateLimiter})
	}
	server := &Server{
		app:                service,
		carpools:           service,
		apiQuotas:          service,
		apiPayment:         service,
		apiHealth:          option.APIHealth,
		adminAPIHealth:     option.AdminAPIHealth,
		apiModelTester:     apiModelTester,
		adminUsers:         service,
		operationAudit:     service,
		apiPromotions:      service,
		growth:             service,
		promotionRewards:   service,
		reputation:         service,
		publicProfiles:     service,
		accountGovernance:  service,
		apiOrderContinuity: service,
		carpoolContinuity:  service,
		disputeContinuity:  service,
		devPersonas:        service,
		mux:                chi.NewRouter(),
		enableDevAuth:      option.EnableDevAuth,
		readinessChecker:   option.ReadinessChecker,
		navigationBadges:   navigationBadges,
		realtimeHub:        realtimeHub,
		oauth:              option.OAuth,
		frontendOrigin:     option.FrontendOrigin,
		cookieSecure:       option.AppEnv == config.EnvProduction,
		allowedOrigins:     append([]string(nil), option.AllowedOrigins...),
		rateLimiter:        rateLimiter,
		oauthHTTPClient:    &http.Client{Timeout: 10 * time.Second},
		clientIPResolver:   middleware.NewClientIPResolver(option.TrustXForwardedFor, option.TrustedProxies),
		metrics:            metrics,
		metricsToken:       strings.TrimSpace(option.MetricsBearerToken),
		metricsAuth:        option.AppEnv == config.EnvProduction || strings.TrimSpace(option.MetricsBearerToken) != "",
		turnstile:          option.TurnstileVerifier,
	}
	server.routes()
	handler := middleware.WithClientIP(
		server.clientIPResolver,
		middleware.WithRequestLogging(
			log.Default(),
			middleware.WithSecurityHeaders(
				middleware.WithCORSAndOrigin(server.mux, middleware.CORSOptions{
					AllowedOrigins: server.allowedOrigins,
					Production:     option.AppEnv == config.EnvProduction,
				}),
				middleware.SecurityHeadersOptions{HSTS: option.AppEnv == config.EnvProduction},
			),
		),
	)
	if option.SentryEnabled {
		handler = observability.WithSentry(handler)
	}
	return middleware.WithRequestID(handler)
}
