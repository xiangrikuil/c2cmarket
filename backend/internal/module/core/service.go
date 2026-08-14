package core

import (
	"context"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/accountgovernance"
	"c2c-market/backend/internal/module/announcement"
	"c2c-market/backend/internal/module/apiintent"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/apipromotion"
	"c2c-market/backend/internal/module/apiquota"
	authmodule "c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/carpool"
	"c2c-market/backend/internal/module/catalog"
	contactmodule "c2c-market/backend/internal/module/contact"
	"c2c-market/backend/internal/module/devpersona"
	"c2c-market/backend/internal/module/favorite"
	"c2c-market/backend/internal/module/feedback"
	"c2c-market/backend/internal/module/growth"
	idempotencymodule "c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/modelaudit"
	"c2c-market/backend/internal/module/notification"
	"c2c-market/backend/internal/module/officialprice"
	"c2c-market/backend/internal/module/operationaudit"
	"c2c-market/backend/internal/module/profile"
	"c2c-market/backend/internal/module/promotionreward"
	"c2c-market/backend/internal/module/report"
	"c2c-market/backend/internal/module/reputation"
	"c2c-market/backend/internal/module/review"
	"c2c-market/backend/internal/module/search"
	"c2c-market/backend/internal/platform/modelsdev"
	"c2c-market/backend/internal/platform/outboundhttp"
)

const (
	LeadStatusPending          = officialprice.LeadStatusPending
	LeadStatusChangesRequested = officialprice.LeadStatusChangesRequested
	LeadStatusApproved         = officialprice.LeadStatusApproved
	LeadStatusRejected         = officialprice.LeadStatusRejected

	RecordStatusActive     = officialprice.RecordStatusActive
	RecordStatusSuperseded = officialprice.RecordStatusSuperseded

	CarpoolListingStatusDraft            = carpool.ListingStatusDraft
	CarpoolListingStatusPendingReview    = carpool.ListingStatusPendingReview
	CarpoolListingStatusChangesRequested = carpool.ListingStatusChangesRequested
	CarpoolListingStatusActive           = carpool.ListingStatusActive
	CarpoolListingStatusPaused           = carpool.ListingStatusPaused
	CarpoolListingStatusRejected         = carpool.ListingStatusRejected
	CarpoolListingStatusRemoved          = carpool.ListingStatusRemoved

	CarpoolApplicationStatusPendingOwner     = carpool.ApplicationStatusPendingOwner
	CarpoolApplicationStatusAcceptedReserved = carpool.ApplicationStatusAcceptedReserved
	CarpoolApplicationStatusJoined           = carpool.ApplicationStatusJoined
	CarpoolApplicationStatusRejected         = carpool.ApplicationStatusRejected
	CarpoolApplicationStatusCancelledByBuyer = carpool.ApplicationStatusCancelledByBuyer
	CarpoolApplicationStatusCancelledByOwner = carpool.ApplicationStatusCancelledByOwner
	CarpoolApplicationStatusExpired          = carpool.ApplicationStatusExpired

	CarpoolJoinActorBuyer = carpool.JoinActorBuyer
	CarpoolJoinActorOwner = carpool.JoinActorOwner

	CarpoolMembershipStatusActive    = carpool.MembershipStatusActive
	CarpoolMembershipStatusCompleted = carpool.MembershipStatusCompleted
	CarpoolMembershipStatusLeft      = carpool.MembershipStatusLeft
	CarpoolMembershipStatusRemoved   = carpool.MembershipStatusRemoved

	APIServiceReviewStatusDraft            = apimarket.ServiceReviewStatusDraft
	APIServiceReviewStatusPendingReview    = apimarket.ServiceReviewStatusPendingReview
	APIServiceReviewStatusChangesRequested = apimarket.ServiceReviewStatusChangesRequested
	APIServiceReviewStatusApproved         = apimarket.ServiceReviewStatusApproved
	APIServiceReviewStatusRejected         = apimarket.ServiceReviewStatusRejected

	APIServicePublicationStatusOffline     = apimarket.ServicePublicationStatusOffline
	APIServicePublicationStatusOnline      = apimarket.ServicePublicationStatusOnline
	APIServicePublicationStatusOwnerPaused = apimarket.ServicePublicationStatusOwnerPaused
	APIServicePublicationStatusArchived    = apimarket.ServicePublicationStatusArchived

	APIServiceModerationStatusClear          = apimarket.ServiceModerationStatusClear
	APIServiceModerationStatusAdminSuspended = apimarket.ServiceModerationStatusAdminSuspended
	APIServiceModerationStatusRemoved        = apimarket.ServiceModerationStatusRemoved

	APIServiceDistributionSub2API     = apimarket.ServiceDistributionSub2API
	APIServiceBillingModeMetered      = apimarket.ServiceBillingModeMetered
	APIServiceBillingModeManual       = apimarket.ServiceBillingModeManual
	APIServiceBillingModeFixedPackage = apimarket.ServiceBillingModeFixedPackage

	APIPurchaseIntentStatusOpen           = apiintent.StatusOpen
	APIPurchaseIntentStatusContacted      = apiintent.StatusContacted
	APIPurchaseIntentStatusOrdered        = apiintent.StatusOrdered
	APIPurchaseIntentStatusBuyerCancelled = apiintent.StatusBuyerCancelled
	APIPurchaseIntentStatusOwnerClosed    = apiintent.StatusOwnerClosed
)

// Service is a legacy compatibility facade that wires domain services together
// for existing app/server construction. New behavior should prefer a
// domain-specific service boundary instead of adding more facade methods here.
type Service struct {
	now                func() time.Time
	authService        *authmodule.Service
	idempotencyService *idempotencymodule.Service
	officialPrice      *officialprice.Service
	catalogService     *catalog.Service
	carpoolService     *carpool.Service
	apiMarket          *apimarket.Manager
	apiIntent          *apiintent.Manager
	apiOrder           *apiorder.Service
	apiPromotion       *apipromotion.Service
	apiQuota           *apiquota.Manager
	announcement       *announcement.Service
	notification       *notification.Service
	contactService     *contactmodule.Service
	devPersonaService  *devpersona.Service
	profileService     *profile.Service
	emailSender        profile.EmailSender
	feedbackService    *feedback.Service
	favoriteService    *favorite.Service
	reviewService      *review.Service
	searchService      *search.Service
	reportService      *report.Service
	reputationService  *reputation.Service
	sellerPublishCheck *sellerPublishActionChecker
	modelAudit         *modelaudit.Service
	growthService      *growth.Service
	promotionRewards   *promotionreward.Service
	operationAudit     *operationaudit.Service
	accountGovernance  *accountgovernance.Service
}

type ServiceOptions struct {
	EmailVerificationPepper string
}

func NewService() *Service {
	return NewServiceWithClock(time.Now)
}

func NewServiceWithPersistence(persistence Persistence) *Service {
	return NewServiceWithRepositories(RepositoriesFromPersistence(persistence))
}

func NewServiceWithRepositories(repositories Repositories) *Service {
	return newService(time.Now, repositories)
}

func NewServiceWithRepositoriesAndEmailSender(repositories Repositories, emailSender profile.EmailSender) *Service {
	return newServiceWithEmailSender(time.Now, repositories, emailSender)
}

func NewServiceWithRepositoriesEmailSenderAndOptions(repositories Repositories, emailSender profile.EmailSender, options ServiceOptions) *Service {
	return newServiceWithOptions(time.Now, repositories, emailSender, options)
}

func NewServiceWithClock(now func() time.Time) *Service {
	return newService(now, Repositories{})
}

func newService(now func() time.Time, repositories Repositories) *Service {
	return newServiceWithEmailSender(now, repositories, profile.NewDevelopmentEmailSender())
}

func newServiceWithEmailSender(now func() time.Time, repositories Repositories, emailSender profile.EmailSender) *Service {
	return newServiceWithOptions(now, repositories, emailSender, ServiceOptions{})
}

func newServiceWithOptions(now func() time.Time, repositories Repositories, emailSender profile.EmailSender, options ServiceOptions) *Service {
	idempotencyService := idempotencymodule.NewService(repositories.Idempotency, now)
	s := &Service{
		authService:        authmodule.NewServiceWithRegistrationEmailSenderAndIdempotency(repositories.Auth, now, emailSender, idempotencyService),
		idempotencyService: idempotencyService,
		catalogService:     catalog.NewService(repositories.Catalog, idempotencyService, modelsdev.NewClient(15*time.Second), now),
		announcement:       announcement.NewService(repositories.Announcement, now),
		notification:       notification.NewService(repositories.Notification, now),
		contactService:     contactmodule.NewService(repositories.Contact, now),
		growthService:      growth.NewService(repositories.Growth, now),
		accountGovernance:  accountgovernance.NewService(repositories.AccountGovernance, now),
		promotionRewards:   promotionreward.NewService(repositories.PromotionReward, idempotencyService, now),
		profileService: profile.NewServiceWithOptions(repositories.Profile, now, emailSender, profile.ServiceOptions{
			EmailVerificationPepper: options.EmailVerificationPepper,
		}),
		emailSender: emailSender,
		now:         now,
	}
	s.authService.SetEmailVerificationPepper(options.EmailVerificationPepper)
	s.operationAudit = operationaudit.NewService(repositories.OperationAudit, s.authService, now)
	s.reputationService = reputation.NewService(repositories.Reputation, now, s.idempotencyService)
	s.contactService.SetActionChecker(s.reputationService)
	s.officialPrice = officialprice.NewService(repositories.OfficialPrice, s.idempotencyService, now)
	s.carpoolService = carpool.NewService(repositories.Carpool, s.catalogService, s.contactService, s.idempotencyService, now)
	s.carpoolService.SetApplicationCreateGuard(func(ctx context.Context, user authmodule.User) *domain.AppError {
		return s.reputationService.CheckActionAllowed(ctx, user.ID, reputation.RoleBuyer, reputation.ActionCarpoolApply)
	})
	s.apiMarket = apimarket.NewManager(repositories.APIService, s.catalogService, s.contactService, now)
	s.devPersonaService = devpersona.NewService(s.authService, s.profileService, s.contactService, s.apiMarket)
	s.apiIntent = apiintent.NewManager(repositories.APIPurchaseIntent, s.apiMarket, s.contactService, s.idempotencyService, now)
	s.reportService = report.NewServiceWithNotifications(repositories.Report, s.idempotencyService, s.notification, now)
	s.apiOrder = apiorder.NewService(repositories.APIOrder, s.apiIntent, s.apiMarket, s.reportService, s.idempotencyService, now)
	s.sellerPublishCheck = &sellerPublishActionChecker{reputation: s.reputationService, orders: s.apiOrder}
	s.apiIntent.SetCreateGuard(func(ctx context.Context, buyerUserID string, service apimarket.Service) *domain.AppError {
		if appErr := s.reputationService.CheckActionAllowed(ctx, buyerUserID, reputation.RoleBuyer, reputation.ActionContactView); appErr != nil {
			return appErr
		}
		return s.sellerPublishCheck.CheckActionAllowed(ctx, service.OwnerUserID, reputation.RoleSeller, reputation.ActionAPIServicePublish)
	})
	s.apiOrder.SetActionChecker(s.sellerPublishCheck)
	s.reportService.SetDisputeProjectionCloser(s.apiOrder)
	s.apiPromotion = apipromotion.NewService(repositories.APIPromotion, s.idempotencyService, now)
	s.apiQuota = apiquota.NewManager(repositories.APIQuota, now)
	s.apiQuota.SetActionChecker(s.sellerPublishCheck)
	s.apiIntent.SetOrderExistenceChecker(s.apiOrder)
	s.feedbackService = feedback.NewService(repositories.Feedback, s.notification, s.idempotencyService, now)
	s.favoriteService = favorite.NewService(repositories.Favorite, s.idempotencyService, s, now)
	s.reviewService = review.NewService(repositories.Review, s.idempotencyService, s, s.reputationService, now)
	s.searchService = search.NewService(repositories.Search, s)
	s.modelAudit = modelaudit.NewService(repositories.ModelAudit, now)
	return s
}

func (s *Service) AccountGovernanceBusinessCenter(ctx context.Context, actor authmodule.BusinessActor) (accountgovernance.Center, *domain.AppError) {
	return s.accountGovernance.BusinessCenter(ctx, actor)
}

func (s *Service) ConfigureModelAuditOutbound(policy *outboundhttp.Policy) {
	if s == nil || s.modelAudit == nil {
		return
	}
	s.modelAudit.SetOutboundPolicy(policy)
}

func (s *Service) ConfigureAPIOrderDeliveryVerifier(timeout time.Duration) {
	if s == nil || s.apiOrder == nil {
		return
	}
	s.apiOrder.SetDeliveryCredentialVerifier(apiorder.NewOpenAIDeliveryCredentialVerifier(timeout))
}

func (s *Service) ConfigurePasswordResetDeliveryRecorder(recorder authmodule.PasswordResetDeliveryRecorder) {
	if s == nil || s.authService == nil {
		return
	}
	s.authService.SetPasswordResetDeliveryRecorder(recorder)
}

func (s *Service) CreateDevSession(ctx context.Context, username string, isAdmin bool) (User, Session, *domain.AppError) {
	user, session, appErr := s.authService.CreateDevSession(ctx, username, isAdmin)
	s.recordAuthenticatedActivity(ctx, user, appErr)
	return user, session, appErr
}

func (s *Service) PrepareDevPersonaSession(ctx context.Context, persona string) (devpersona.Result, *domain.AppError) {
	result, appErr := s.devPersonaService.PrepareSession(ctx, persona)
	s.recordAuthenticatedActivity(ctx, result.User, appErr)
	return result, appErr
}

func (s *Service) LoginWithOAuthProfile(ctx context.Context, profile OAuthProfile) (User, Session, *domain.AppError) {
	user, session, appErr := s.authService.LoginWithOAuthProfile(ctx, profile)
	s.recordAuthenticatedActivity(ctx, user, appErr)
	return user, session, appErr
}

func (s *Service) AuthenticateWithOAuthProfile(ctx context.Context, profile OAuthProfile) (authmodule.AuthenticationResult, *domain.AppError) {
	result, appErr := s.authService.AuthenticateWithOAuthProfile(ctx, profile)
	s.recordAuthenticatedActivity(ctx, result.User, appErr)
	return result, appErr
}

func (s *Service) StartRestrictedBusinessOAuth(ctx context.Context) (string, *domain.AppError) {
	return s.authService.StartRestrictedBusinessOAuth(ctx)
}

func (s *Service) CompleteRestrictedBusinessOAuth(ctx context.Context, state string, profile authmodule.OAuthProfile) (authmodule.AuthenticationResult, *domain.AppError) {
	return s.authService.CompleteRestrictedBusinessOAuth(ctx, state, profile)
}

func (s *Service) StartAccountAppealOAuth(ctx context.Context) (string, *domain.AppError) {
	return s.authService.StartAccountAppealOAuth(ctx)
}

func (s *Service) CompleteAccountAppealOAuth(ctx context.Context, state string, profile authmodule.OAuthProfile) (authmodule.User, authmodule.AccountAppealSession, *domain.AppError) {
	return s.authService.CompleteAccountAppealOAuth(ctx, state, profile)
}

func (s *Service) StartAccountAppealSession(ctx context.Context, profile authmodule.OAuthProfile) (authmodule.User, authmodule.AccountAppealSession, *domain.AppError) {
	return s.authService.StartAccountAppealSession(ctx, profile)
}

func (s *Service) GetAccountAppealSession(ctx context.Context, sessionID string) (authmodule.User, authmodule.AccountAppealSession, *domain.AppError) {
	return s.authService.GetAccountAppealSession(ctx, sessionID)
}

func (s *Service) GetAccountAppealSessionWithCSRF(ctx context.Context, sessionID, csrfToken string) (authmodule.User, authmodule.AccountAppealSession, *domain.AppError) {
	return s.authService.GetAccountAppealSessionWithCSRF(ctx, sessionID, csrfToken)
}

func (s *Service) LoginWithPassword(ctx context.Context, username, password string) (User, Session, *domain.AppError) {
	user, session, appErr := s.authService.LoginWithPassword(ctx, username, password)
	s.recordAuthenticatedActivity(ctx, user, appErr)
	return user, session, appErr
}

func (s *Service) AuthenticateWithPassword(ctx context.Context, username, password string) (authmodule.AuthenticationResult, *domain.AppError) {
	result, appErr := s.authService.AuthenticateWithPassword(ctx, username, password)
	s.recordAuthenticatedActivity(ctx, result.User, appErr)
	return result, appErr
}

func (s *Service) StudentRegistrationConfig(ctx context.Context) (authmodule.StudentRegistrationConfig, *domain.AppError) {
	return s.authService.StudentRegistrationConfig(ctx)
}

func (s *Service) UsernameAvailable(ctx context.Context, username string) (bool, *domain.AppError) {
	return s.authService.UsernameAvailable(ctx, username)
}

func (s *Service) AdminStudentRegistration(ctx context.Context, user authmodule.User) (authmodule.StudentRegistrationConfig, *domain.AppError) {
	return s.authService.AdminStudentRegistration(ctx, user)
}

func (s *Service) UpdateAdminStudentRegistrationWithIdempotency(ctx context.Context, user authmodule.User, routeKey, key, requestHash string, input authmodule.StudentRegistrationSettingUpdate, build authmodule.StudentRegistrationCompletionBuilder) (idempotencymodule.Completion, *domain.AppError) {
	return s.authService.UpdateAdminStudentRegistrationWithIdempotency(ctx, user, routeKey, key, requestHash, input, build)
}

func (s *Service) AdminStudentInstitutionDomains(ctx context.Context, user authmodule.User) ([]authmodule.StudentInstitutionDomain, *domain.AppError) {
	return s.authService.AdminStudentInstitutionDomains(ctx, user)
}

func (s *Service) CreateStudentInstitutionDomainWithIdempotency(ctx context.Context, user authmodule.User, routeKey, key, requestHash string, input authmodule.StudentInstitutionDomainCreateInput, build authmodule.StudentInstitutionDomainCompletionBuilder) (idempotencymodule.Completion, *domain.AppError) {
	return s.authService.CreateStudentInstitutionDomainWithIdempotency(ctx, user, routeKey, key, requestHash, input, build)
}

func (s *Service) UpdateStudentInstitutionDomainWithIdempotency(ctx context.Context, user authmodule.User, routeKey, key, requestHash string, input authmodule.StudentInstitutionDomainUpdateInput, build authmodule.StudentInstitutionDomainCompletionBuilder) (idempotencymodule.Completion, *domain.AppError) {
	return s.authService.UpdateStudentInstitutionDomainWithIdempotency(ctx, user, routeKey, key, requestHash, input, build)
}

func (s *Service) ReauthenticatePassword(ctx context.Context, sessionID, csrfToken, password string) *domain.AppError {
	return s.authService.ReauthenticatePassword(ctx, sessionID, csrfToken, password)
}

func (s *Service) ReauthenticatePasswordForPurpose(ctx context.Context, sessionID, csrfToken, password, purpose string) *domain.AppError {
	return s.authService.ReauthenticatePasswordForPurpose(ctx, sessionID, csrfToken, password, purpose)
}

func (s *Service) StartAdminReauthenticationOAuth(ctx context.Context, sessionID string) (string, *domain.AppError) {
	return s.authService.StartAdminReauthenticationOAuth(ctx, sessionID)
}

func (s *Service) CompleteAdminReauthenticationOAuth(ctx context.Context, sessionID, state string, profile authmodule.OAuthProfile) *domain.AppError {
	return s.authService.CompleteAdminReauthenticationOAuth(ctx, sessionID, state, profile)
}

func (s *Service) StartLinuxDoLink(ctx context.Context, sessionID string) (string, *domain.AppError) {
	return s.authService.StartLinuxDoLink(ctx, sessionID)
}

func (s *Service) CompleteLinuxDoLink(ctx context.Context, sessionID, state string, profile authmodule.OAuthProfile) (User, Session, *domain.AppError) {
	return s.authService.CompleteLinuxDoLink(ctx, sessionID, state, profile)
}

func (s *Service) BootstrapAdmin(ctx context.Context, input BootstrapAdminInput) (BootstrapAdminResult, *domain.AppError) {
	return s.authService.BootstrapAdmin(ctx, input)
}

func (s *Service) StartEmailRegistration(ctx context.Context, input EmailRegistrationStartInput) (EmailRegistrationChallenge, *domain.AppError) {
	return s.authService.StartEmailRegistration(ctx, input)
}

func (s *Service) ConfirmEmailRegistration(ctx context.Context, input EmailRegistrationConfirmInput) (User, Session, *domain.AppError) {
	user, session, appErr := s.authService.ConfirmEmailRegistration(ctx, input)
	s.recordAuthenticatedActivity(ctx, user, appErr)
	return user, session, appErr
}

func (s *Service) StartPasswordReset(ctx context.Context, input authmodule.PasswordResetStartInput) (authmodule.PasswordResetStartResult, *domain.AppError) {
	return s.authService.StartPasswordReset(ctx, input)
}

func (s *Service) ConfirmPasswordReset(ctx context.Context, input authmodule.PasswordResetConfirmInput) *domain.AppError {
	return s.authService.ConfirmPasswordReset(ctx, input)
}

func (s *Service) SetPassword(ctx context.Context, input SetPasswordInput) *domain.AppError {
	return s.authService.SetPassword(ctx, input)
}

func (s *Service) GetSession(ctx context.Context, sessionID string) (User, Session, *domain.AppError) {
	return s.authService.GetSession(ctx, sessionID)
}

func (s *Service) GetSessionWithCSRF(ctx context.Context, sessionID, csrfToken string) (User, Session, *domain.AppError) {
	return s.authService.GetSessionWithCSRF(ctx, sessionID, csrfToken)
}

func (s *Service) GetRestrictedBusinessSession(ctx context.Context, sessionID string) (User, authmodule.RestrictedBusinessSession, *domain.AppError) {
	return s.authService.GetRestrictedBusinessSession(ctx, sessionID)
}

func (s *Service) GetRestrictedBusinessSessionWithCSRF(ctx context.Context, sessionID, csrfToken string) (User, authmodule.RestrictedBusinessSession, *domain.AppError) {
	return s.authService.GetRestrictedBusinessSessionWithCSRF(ctx, sessionID, csrfToken)
}

func (s *Service) RefreshRestrictedBusinessSessionCSRF(ctx context.Context, sessionID string) (string, *domain.AppError) {
	return s.authService.RefreshRestrictedBusinessSessionCSRF(ctx, sessionID)
}

func (s *Service) LogoutRestrictedBusinessSession(ctx context.Context, sessionID string) {
	s.authService.LogoutRestrictedBusinessSession(ctx, sessionID)
}

func (s *Service) recordAuthenticatedActivity(ctx context.Context, user User, authErr *domain.AppError) {
	if authErr != nil || user.ID == "" || s == nil || s.growthService == nil {
		return
	}
	if appErr := s.growthService.RecordActivity(ctx, user.ID); appErr != nil {
		log.Printf("growth_activity_record_failed user_id=%s code=%s", user.ID, appErr.Code)
	}
}

func (s *Service) AdminGrowthOverview(ctx context.Context, user User, windowDays int) (growth.Overview, *domain.AppError) {
	return s.growthService.AdminOverview(ctx, user, windowDays)
}

func (s *Service) RecordAuthenticatedActivity(ctx context.Context, userID string) *domain.AppError {
	return s.growthService.RecordActivity(ctx, userID)
}

func (s *Service) RenewSession(ctx context.Context, sessionID string) (Session, bool, *domain.AppError) {
	return s.authService.RenewSession(ctx, sessionID)
}

func (s *Service) AdminUsers(ctx context.Context, user User, query authmodule.AdminUserDirectoryQuery) (authmodule.AdminUserDirectory, *domain.AppError) {
	return s.authService.AdminUsers(ctx, user, query)
}

func (s *Service) AdminAuditLogs(ctx context.Context, user User, filter authmodule.AdminAuditLogFilter, page domain.PageRequest) (domain.Page[authmodule.AdminAuditLog], *domain.AppError) {
	return s.authService.AdminAuditLogs(ctx, user, filter, page)
}

func (s *Service) AdminUser(ctx context.Context, user User, userID string) (authmodule.AdminUserDetail, *domain.AppError) {
	return s.authService.AdminUser(ctx, user, userID)
}

func (s *Service) UpdateAdminUserStatusWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input authmodule.AdminUserStatusInput, buildCompletion authmodule.AdminUserCompletionBuilder) (idempotencymodule.Completion, *domain.AppError) {
	return s.authService.UpdateAdminUserStatusWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) UpdateAdminUserPermissionWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input authmodule.AdminUserPermissionInput, buildCompletion authmodule.AdminUserCompletionBuilder) (idempotencymodule.Completion, *domain.AppError) {
	return s.authService.UpdateAdminUserPermissionWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) RefreshSessionCSRF(ctx context.Context, sessionID string) (string, *domain.AppError) {
	return s.authService.RefreshSessionCSRF(ctx, sessionID)
}

func (s *Service) Logout(ctx context.Context, sessionID string) {
	s.authService.Logout(ctx, sessionID)
}

func (s *Service) BeginIdempotency(ctx context.Context, userID, routeKey, key, requestHash string) (*IdempotencyEntry, *domain.AppError) {
	return s.idempotencyService.Begin(ctx, userID, routeKey, key, requestHash)
}

func (s *Service) CompleteIdempotency(ctx context.Context, entry *IdempotencyEntry, status int, contentType string, body []byte, resourceType, resourceID string) *domain.AppError {
	return s.idempotencyService.Complete(ctx, entry, status, contentType, body, resourceType, resourceID)
}

func (s *Service) CancelIdempotency(ctx context.Context, entry *IdempotencyEntry) {
	s.idempotencyService.Cancel(ctx, entry)
}

func (s *Service) SubmitOfficialPriceLead(ctx context.Context, user User, input SubmitLeadInput) (OfficialPriceLead, *domain.AppError) {
	return s.officialPrice.SubmitLead(ctx, user, input)
}

func (s *Service) MyOfficialPriceLeads(ctx context.Context, user User) ([]OfficialPriceLead, *domain.AppError) {
	return s.officialPrice.MyLeads(ctx, user)
}

func (s *Service) MyOfficialPriceLead(ctx context.Context, user User, leadID string) (OfficialPriceLead, *domain.AppError) {
	return s.officialPrice.MyLead(ctx, user, leadID)
}

func (s *Service) AdminOfficialPriceLeads(ctx context.Context, user User) ([]OfficialPriceLead, *domain.AppError) {
	return s.officialPrice.AdminLeads(ctx, user)
}

func (s *Service) AdminOfficialPriceLead(ctx context.Context, user User, leadID string) (OfficialPriceLead, *domain.AppError) {
	return s.officialPrice.AdminLead(ctx, user, leadID)
}

func (s *Service) AdminOfficialPriceRecords(ctx context.Context, user User) ([]OfficialPriceRecord, *domain.AppError) {
	return s.officialPrice.AdminRecords(ctx, user)
}

func (s *Service) AdminOfficialPriceRecord(ctx context.Context, user User, recordID string) (OfficialPriceRecord, *domain.AppError) {
	return s.officialPrice.AdminRecord(ctx, user, recordID)
}

func (s *Service) CreateAdminOfficialPriceRecord(ctx context.Context, user User, input AdminOfficialPriceRecordInput) (OfficialPriceRecord, *domain.AppError) {
	return s.officialPrice.AdminCreateRecord(ctx, user, input)
}

func (s *Service) UpdateAdminOfficialPriceRecord(ctx context.Context, user User, input AdminOfficialPriceRecordInput) (OfficialPriceRecord, *domain.AppError) {
	return s.officialPrice.AdminUpdateRecord(ctx, user, input)
}

func (s *Service) TakeDownAdminOfficialPriceRecord(ctx context.Context, user User, input AdminOfficialPriceRecordActionInput) (OfficialPriceRecord, *domain.AppError) {
	return s.officialPrice.AdminTakeDownRecord(ctx, user, input)
}

func (s *Service) ApproveOfficialPriceLead(ctx context.Context, input ApproveLeadInput) (OfficialPriceLead, OfficialPriceRecord, *domain.AppError) {
	return s.officialPrice.ApproveLead(ctx, input)
}

func (s *Service) ApproveOfficialPriceLeadWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input ApproveLeadInput, buildCompletion OfficialPriceApprovalCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.officialPrice.ApproveLeadWithIdempotency(ctx, userID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) UpdateLeadReviewStatus(ctx context.Context, user User, leadID, status, reason string, ifMatchVersion int64) (OfficialPriceLead, *domain.AppError) {
	return s.officialPrice.UpdateLeadReviewStatus(ctx, user, leadID, status, reason, ifMatchVersion)
}

func (s *Service) PublicOfficialPriceRecords(ctx context.Context) ([]OfficialPriceRecord, *domain.AppError) {
	return s.officialPrice.PublicRecords(ctx)
}

func (s *Service) PublicOfficialPriceRecord(ctx context.Context, recordID string) (OfficialPriceRecord, *domain.AppError) {
	return s.officialPrice.PublicRecord(ctx, recordID)
}

func (s *Service) ProductCategories(ctx context.Context) ([]ProductCategory, *domain.AppError) {
	return s.catalogService.ProductCategories(ctx)
}

func (s *Service) AdminProductCategories(ctx context.Context, user User) ([]ProductCategory, *domain.AppError) {
	return s.catalogService.AdminProductCategories(ctx, user)
}

func (s *Service) AdminProductCategory(ctx context.Context, user User, categoryID string) (ProductCategory, *domain.AppError) {
	return s.catalogService.AdminProductCategory(ctx, user, categoryID)
}

func (s *Service) CreateProductCategory(ctx context.Context, user User, input ProductCategoryInput) (ProductCategory, *domain.AppError) {
	return s.catalogService.CreateProductCategory(ctx, user, input)
}

func (s *Service) UpdateProductCategory(ctx context.Context, user User, categoryID string, input ProductCategoryInput) (ProductCategory, *domain.AppError) {
	return s.catalogService.UpdateProductCategory(ctx, user, categoryID, input)
}

func (s *Service) ProductPlans(ctx context.Context, categoryCode string) ([]ProductPlan, *domain.AppError) {
	return s.catalogService.ProductPlans(ctx, categoryCode)
}

func (s *Service) ProductPlan(ctx context.Context, planID string) (ProductPlan, *domain.AppError) {
	return s.catalogService.ProductPlan(ctx, planID)
}

func (s *Service) AdminProductPlans(ctx context.Context, user User, categoryCode string) ([]ProductPlan, *domain.AppError) {
	return s.catalogService.AdminProductPlans(ctx, user, categoryCode)
}

func (s *Service) AdminProductPlan(ctx context.Context, user User, planID string) (ProductPlan, *domain.AppError) {
	return s.catalogService.AdminProductPlan(ctx, user, planID)
}

func (s *Service) CreateProductPlan(ctx context.Context, user User, input ProductPlanInput) (ProductPlan, *domain.AppError) {
	return s.catalogService.CreateProductPlan(ctx, user, input)
}

func (s *Service) UpdateProductPlan(ctx context.Context, user User, planID string, input ProductPlanInput) (ProductPlan, *domain.AppError) {
	return s.catalogService.UpdateProductPlan(ctx, user, planID, input)
}

func (s *Service) AdminAPIModelProviders(ctx context.Context, user User) ([]APIModelProvider, *domain.AppError) {
	return s.catalogService.AdminAPIModelProviders(ctx, user)
}

func (s *Service) AdminAPIModelProvider(ctx context.Context, user User, providerID string) (APIModelProvider, *domain.AppError) {
	return s.catalogService.AdminAPIModelProvider(ctx, user, providerID)
}

func (s *Service) CreateAPIModelProvider(ctx context.Context, user User, input APIModelProviderInput) (APIModelProvider, *domain.AppError) {
	return s.catalogService.CreateAPIModelProvider(ctx, user, input)
}

func (s *Service) UpdateAPIModelProvider(ctx context.Context, user User, providerID string, input APIModelProviderInput) (APIModelProvider, *domain.AppError) {
	return s.catalogService.UpdateAPIModelProvider(ctx, user, providerID, input)
}

func (s *Service) APIModels(ctx context.Context) ([]APIModelCatalog, *domain.AppError) {
	return s.catalogService.APIModels(ctx)
}

func (s *Service) APIModel(ctx context.Context, modelID string) (APIModelCatalog, *domain.AppError) {
	return s.catalogService.APIModel(ctx, modelID)
}

func (s *Service) AdminAPIModels(ctx context.Context, user User) ([]APIModelCatalog, *domain.AppError) {
	return s.catalogService.AdminAPIModels(ctx, user)
}

func (s *Service) AdminAPIModel(ctx context.Context, user User, modelID string) (APIModelCatalog, *domain.AppError) {
	return s.catalogService.AdminAPIModel(ctx, user, modelID)
}

func (s *Service) CreateAPIModel(ctx context.Context, user User, input APIModelInput) (APIModelCatalog, *domain.AppError) {
	return s.catalogService.CreateAPIModel(ctx, user, input)
}

func (s *Service) UpdateAPIModel(ctx context.Context, user User, modelID string, input APIModelInput) (APIModelCatalog, *domain.AppError) {
	return s.catalogService.UpdateAPIModel(ctx, user, modelID, input)
}

func (s *Service) ApplyCatalogLifecycleWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input catalog.LifecycleActionInput, buildCompletion catalog.LifecycleCompletionBuilder) (idempotencymodule.Completion, *domain.AppError) {
	return s.catalogService.ApplyCatalogLifecycleWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
}
func (s *Service) PreviewAPIModelSync(ctx context.Context, user User, input catalog.APIModelSyncPreviewInput) (catalog.APIModelSyncPreview, *domain.AppError) {
	return s.catalogService.PreviewAPIModelSync(ctx, user, input)
}

func (s *Service) ApplyAPIModelSyncWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input catalog.APIModelSyncApplyInput, buildCompletion catalog.APIModelSyncCompletionBuilder) (idempotencymodule.Completion, *domain.AppError) {
	return s.catalogService.ApplyAPIModelSyncWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) ConfigureModelsDevSource(source modelsdev.Source) {
	if s == nil || s.catalogService == nil {
		return
	}
	s.catalogService.SetModelsDevSource(source)
}

func (s *Service) CreateAPIService(ctx context.Context, user User, input CreateAPIServiceInput) (APIService, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIServicePublish); appErr != nil {
		return APIService{}, appErr
	}
	service, appErr := s.apiMarket.Create(ctx, user, input)
	if appErr != nil {
		return APIService{}, appErr
	}
	return s.withAPIMerchantProfile(ctx, service)
}

func (s *Service) CreateAPIServiceWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input CreateAPIServiceInput, buildCompletion apimarket.ServiceCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIServicePublish); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	return s.apiMarket.CreateWithIdempotency(ctx, user, routeKey, key, requestHash, input, s.apiServiceCompletionWithProfile(ctx, buildCompletion))
}

func (s *Service) UpdateAPIService(ctx context.Context, user User, input UpdateAPIServiceInput) (APIService, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIServicePublish); appErr != nil {
		return APIService{}, appErr
	}
	service, appErr := s.apiMarket.Update(ctx, user, input)
	if appErr != nil {
		return APIService{}, appErr
	}
	return s.withAPIMerchantProfile(ctx, service)
}

func (s *Service) UpdateAPIServiceWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input UpdateAPIServiceInput, buildCompletion apimarket.ServiceCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIServicePublish); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	return s.apiMarket.UpdateWithIdempotency(ctx, user, routeKey, key, requestHash, input, s.apiServiceCompletionWithProfile(ctx, buildCompletion))
}

func (s *Service) UpdateAPIServiceProbeConnection(ctx context.Context, user User, input apimarket.UpdateProbeConnectionInput) (APIService, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIServicePublish); appErr != nil {
		return APIService{}, appErr
	}
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIProbeManage); appErr != nil {
		return APIService{}, appErr
	}
	service, appErr := s.apiMarket.UpdateProbeConnection(ctx, user, input)
	if appErr != nil {
		return APIService{}, appErr
	}
	return s.withAPIMerchantProfile(ctx, service)
}

func (s *Service) UpdateAPIServiceProbeConnectionWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input apimarket.UpdateProbeConnectionInput, buildCompletion apimarket.ServiceCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIServicePublish); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIProbeManage); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	return s.apiMarket.UpdateProbeConnectionWithIdempotency(ctx, user, routeKey, key, requestHash, input, s.apiServiceCompletionWithProfile(ctx, buildCompletion))
}

func (s *Service) PublicAPIServices(ctx context.Context, filter apimarket.PublicServiceFilter, page domain.PageRequest) (domain.Page[APIService], *domain.AppError) {
	services, appErr := s.apiMarket.PublicServices(ctx, filter, page)
	if appErr != nil {
		return domain.Page[APIService]{}, appErr
	}
	items, appErr := s.withAPIMerchantProfiles(ctx, services.Items)
	if appErr != nil {
		return domain.Page[APIService]{}, appErr
	}
	items, appErr = s.withSellerReputation(ctx, items)
	if appErr != nil {
		return domain.Page[APIService]{}, appErr
	}
	return domain.Page[APIService]{Items: items, NextCursor: services.NextCursor}, nil
}

func (s *Service) PublicAPIService(ctx context.Context, serviceID string) (APIService, *domain.AppError) {
	service, appErr := s.apiMarket.PublicService(ctx, serviceID)
	if appErr != nil {
		return APIService{}, appErr
	}
	service, appErr = s.withAPIMerchantProfile(ctx, service)
	if appErr != nil {
		return APIService{}, appErr
	}
	items, appErr := s.withSellerReputation(ctx, []APIService{service})
	if appErr != nil {
		return APIService{}, appErr
	}
	return items[0], nil
}

func (s *Service) PublicAPIPromotions(ctx context.Context, placement string) ([]apipromotion.Promotion, *domain.AppError) {
	items, appErr := s.apiPromotion.Public(ctx, placement)
	if appErr != nil {
		return nil, appErr
	}
	return s.withAPIPromotionServiceContext(ctx, items, true)
}

func (s *Service) AdminAPIPromotions(ctx context.Context, user User) ([]apipromotion.Promotion, *domain.AppError) {
	items, appErr := s.apiPromotion.AdminList(ctx, user)
	if appErr != nil {
		return nil, appErr
	}
	return s.withAPIPromotionServiceContext(ctx, items, false)
}

func (s *Service) APIPromotionAvailability(ctx context.Context, user User, input apipromotion.AvailabilityInput) (apipromotion.Availability, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAdminAccess); appErr != nil {
		return apipromotion.Availability{}, appErr
	}
	return s.apiPromotion.Availability(ctx, user, input)
}

func (s *Service) CreateAPIPromotion(ctx context.Context, user User, input apipromotion.CreateInput) (apipromotion.Promotion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAdminAccess); appErr != nil {
		return apipromotion.Promotion{}, appErr
	}
	item, appErr := s.apiPromotion.Create(ctx, user, input)
	if appErr != nil {
		return apipromotion.Promotion{}, appErr
	}
	items, appErr := s.withAPIPromotionServiceContext(ctx, []apipromotion.Promotion{item}, false)
	if appErr != nil {
		return apipromotion.Promotion{}, appErr
	}
	return items[0], nil
}

func (s *Service) CreateAPIPromotionWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input apipromotion.CreateInput, buildCompletion apipromotion.CompletionBuilder) (idempotencymodule.Completion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAdminAccess); appErr != nil {
		return idempotencymodule.Completion{}, appErr
	}
	return s.apiPromotion.CreateWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) StopAPIPromotion(ctx context.Context, user User, input apipromotion.StopInput) (apipromotion.Promotion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAdminAccess); appErr != nil {
		return apipromotion.Promotion{}, appErr
	}
	item, appErr := s.apiPromotion.Stop(ctx, user, input)
	if appErr != nil {
		return apipromotion.Promotion{}, appErr
	}
	items, appErr := s.withAPIPromotionServiceContext(ctx, []apipromotion.Promotion{item}, false)
	if appErr != nil {
		return apipromotion.Promotion{}, appErr
	}
	return items[0], nil
}

func (s *Service) StopAPIPromotionWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input apipromotion.StopInput, buildCompletion apipromotion.CompletionBuilder) (idempotencymodule.Completion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAdminAccess); appErr != nil {
		return idempotencymodule.Completion{}, appErr
	}
	return s.apiPromotion.StopWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) PromotionRewardPublicConfig(ctx context.Context) (promotionreward.PublicConfig, *domain.AppError) {
	return s.promotionRewards.PublicConfig(ctx)
}

func (s *Service) MyReferralSummary(ctx context.Context, user User) (promotionreward.ReferralSummary, *domain.AppError) {
	return s.promotionRewards.MyReferral(ctx, user)
}

func (s *Service) MyPromotionCoupons(ctx context.Context, user User, query promotionreward.CouponQuery) (promotionreward.CouponPage, *domain.AppError) {
	return s.promotionRewards.MyCoupons(ctx, user, query)
}

func (s *Service) ApplyPromotionCouponWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input promotionreward.ApplyCouponInput, buildCompletion promotionreward.CouponCompletionBuilder) (idempotencymodule.Completion, *domain.AppError) {
	return s.promotionRewards.ApplyCouponWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) AdminPromotionRewardCampaign(ctx context.Context, user User) (promotionreward.Campaign, *domain.AppError) {
	return s.promotionRewards.AdminCampaign(ctx, user)
}

func (s *Service) UpdateAdminPromotionRewardCampaign(ctx context.Context, user User, input promotionreward.UpdateCampaignInput) (promotionreward.Campaign, *domain.AppError) {
	return s.promotionRewards.UpdateAdminCampaign(ctx, user, input)
}

func (s *Service) UpdateAdminPromotionRewardCampaignWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input promotionreward.UpdateCampaignInput, buildCompletion promotionreward.CampaignCompletionBuilder) (idempotencymodule.Completion, *domain.AppError) {
	return s.promotionRewards.UpdateAdminCampaignWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) AdminReferrals(ctx context.Context, user User, query promotionreward.ReferralQuery) (promotionreward.ReferralPage, *domain.AppError) {
	return s.promotionRewards.AdminReferrals(ctx, user, query)
}

func (s *Service) RevokeAdminReferral(ctx context.Context, user User, input promotionreward.RevokeReferralInput) (promotionreward.ReferralRecord, *domain.AppError) {
	return s.promotionRewards.RevokeAdminReferral(ctx, user, input)
}

func (s *Service) RevokeAdminReferralWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input promotionreward.RevokeReferralInput, buildCompletion promotionreward.ReferralCompletionBuilder) (idempotencymodule.Completion, *domain.AppError) {
	return s.promotionRewards.RevokeAdminReferralWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) AdminPromotionCoupons(ctx context.Context, user User, query promotionreward.CouponQuery) (promotionreward.CouponPage, *domain.AppError) {
	return s.promotionRewards.AdminCoupons(ctx, user, query)
}

func (s *Service) GrantAdminPromotionCoupon(ctx context.Context, user User, input promotionreward.GrantCouponInput) (promotionreward.Coupon, *domain.AppError) {
	return s.promotionRewards.GrantAdminCoupon(ctx, user, input)
}

func (s *Service) GrantAdminPromotionCouponWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input promotionreward.GrantCouponInput, buildCompletion promotionreward.CouponCompletionBuilder) (idempotencymodule.Completion, *domain.AppError) {
	return s.promotionRewards.GrantAdminCouponWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) RevokeAdminPromotionCoupon(ctx context.Context, user User, input promotionreward.RevokeCouponInput) (promotionreward.Coupon, *domain.AppError) {
	return s.promotionRewards.RevokeAdminCoupon(ctx, user, input)
}

func (s *Service) RevokeAdminPromotionCouponWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input promotionreward.RevokeCouponInput, buildCompletion promotionreward.CouponCompletionBuilder) (idempotencymodule.Completion, *domain.AppError) {
	return s.promotionRewards.RevokeAdminCouponWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) withAPIPromotionServiceContext(ctx context.Context, items []apipromotion.Promotion, includeReputation bool) ([]apipromotion.Promotion, *domain.AppError) {
	services := make([]APIService, 0, len(items))
	for _, item := range items {
		services = append(services, item.Service)
	}
	services, appErr := s.withAPIMerchantProfiles(ctx, services)
	if appErr != nil {
		return nil, appErr
	}
	if includeReputation {
		services, appErr = s.withSellerReputation(ctx, services)
		if appErr != nil {
			return nil, appErr
		}
	}
	for i := range items {
		items[i].Service = services[i]
	}
	return items, nil
}

func (s *Service) OwnerAPIServices(ctx context.Context, user User, filter apimarket.OwnerServiceFilter, page domain.PageRequest) (domain.Page[APIService], *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIServicePublish); appErr != nil {
		return domain.Page[APIService]{}, appErr
	}
	services, appErr := s.apiMarket.OwnerServices(ctx, user, filter, page)
	if appErr != nil {
		return domain.Page[APIService]{}, appErr
	}
	items, appErr := s.withAPIMerchantProfiles(ctx, services.Items)
	if appErr != nil {
		return domain.Page[APIService]{}, appErr
	}
	return domain.Page[APIService]{Items: items, NextCursor: services.NextCursor}, nil
}

func (s *Service) OwnerAPIService(ctx context.Context, user User, serviceID string) (APIService, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIServicePublish); appErr != nil {
		return APIService{}, appErr
	}
	service, appErr := s.apiMarket.OwnerService(ctx, user, serviceID)
	if appErr != nil {
		return APIService{}, appErr
	}
	return s.withAPIMerchantProfile(ctx, service)
}

func (s *Service) AdminAPIServices(ctx context.Context, user User, filter apimarket.AdminServiceFilter, page domain.PageRequest) (domain.Page[APIService], *domain.AppError) {
	services, appErr := s.apiMarket.AdminServices(ctx, user, filter, page)
	if appErr != nil {
		return domain.Page[APIService]{}, appErr
	}
	items, appErr := s.withAPIMerchantProfiles(ctx, services.Items)
	if appErr != nil {
		return domain.Page[APIService]{}, appErr
	}
	return domain.Page[APIService]{Items: items, NextCursor: services.NextCursor}, nil
}

func (s *Service) AdminAPIService(ctx context.Context, user User, serviceID string) (APIService, *domain.AppError) {
	service, appErr := s.apiMarket.AdminService(ctx, user, serviceID)
	if appErr != nil {
		return APIService{}, appErr
	}
	return s.withAPIMerchantProfile(ctx, service)
}

func (s *Service) SubmitAPIServiceForReview(ctx context.Context, user User, input APIServiceOwnerActionInput) (APIService, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIServicePublish); appErr != nil {
		return APIService{}, appErr
	}
	if appErr := s.sellerPublishCheck.CheckActionAllowed(ctx, user.ID, reputation.RoleSeller, reputation.ActionAPIServicePublish); appErr != nil {
		return APIService{}, appErr
	}
	service, appErr := s.apiMarket.SubmitForReview(ctx, user, input)
	if appErr != nil {
		return APIService{}, appErr
	}
	return s.withAPIMerchantProfile(ctx, service)
}

func (s *Service) SubmitAPIServiceForReviewWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input APIServiceOwnerActionInput, buildCompletion apimarket.ServiceCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIServicePublish); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	if appErr := s.sellerPublishCheck.CheckActionAllowed(ctx, user.ID, reputation.RoleSeller, reputation.ActionAPIServicePublish); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	return s.apiMarket.SubmitForReviewWithIdempotency(ctx, user, routeKey, key, requestHash, input, s.apiServiceCompletionWithProfile(ctx, buildCompletion))
}

func (s *Service) UpdateAPIServicePublication(ctx context.Context, user User, input APIServiceOwnerActionInput, action string) (APIService, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIServicePublish); appErr != nil {
		return APIService{}, appErr
	}
	if action == "publish" || action == "resume" {
		if appErr := s.sellerPublishCheck.CheckActionAllowed(ctx, user.ID, reputation.RoleSeller, reputation.ActionAPIServicePublish); appErr != nil {
			return APIService{}, appErr
		}
	}
	service, appErr := s.apiMarket.UpdatePublication(ctx, user, input, action)
	if appErr != nil {
		return APIService{}, appErr
	}
	return s.withAPIMerchantProfile(ctx, service)
}

func (s *Service) UpdateAPIServicePublicationWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input APIServiceOwnerActionInput, action string, buildCompletion apimarket.ServiceCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIServicePublish); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	if action == "publish" || action == "resume" {
		if appErr := s.sellerPublishCheck.CheckActionAllowed(ctx, user.ID, reputation.RoleSeller, reputation.ActionAPIServicePublish); appErr != nil {
			return IdempotencyCompletion{}, appErr
		}
	}
	return s.apiMarket.UpdatePublicationWithIdempotency(ctx, user, routeKey, key, requestHash, input, action, s.apiServiceCompletionWithProfile(ctx, buildCompletion))
}

func (s *Service) UpdateAPIServiceAdminStatus(ctx context.Context, user User, input APIServiceAdminActionInput) (APIService, *domain.AppError) {
	service, appErr := s.apiMarket.UpdateAdminStatus(ctx, user, input)
	if appErr != nil {
		return APIService{}, appErr
	}
	return s.withAPIMerchantProfile(ctx, service)
}

func (s *Service) UpdateAPIServiceAdminStatusWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input APIServiceAdminActionInput, buildCompletion apimarket.ServiceCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.apiMarket.UpdateAdminStatusWithIdempotency(ctx, user, routeKey, key, requestHash, input, s.apiServiceCompletionWithProfile(ctx, buildCompletion))
}

func (s *Service) UpdateAPIServiceOrderSettings(ctx context.Context, user User, input apimarket.UpdateOrderSettingsInput) (APIService, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIServicePublish); appErr != nil {
		return APIService{}, appErr
	}
	service, appErr := s.apiMarket.UpdateOrderSettings(ctx, user, input)
	if appErr != nil {
		return APIService{}, appErr
	}
	return s.withAPIMerchantProfile(ctx, service)
}

func (s *Service) UpdateAPIServiceOrderSettingsWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input apimarket.UpdateOrderSettingsInput, buildCompletion apimarket.ServiceCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIServicePublish); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	return s.apiMarket.UpdateOrderSettingsWithIdempotency(ctx, user, routeKey, key, requestHash, input, s.apiServiceCompletionWithProfile(ctx, buildCompletion))
}

func (s *Service) apiServiceCompletionWithProfile(ctx context.Context, buildCompletion apimarket.ServiceCompletionBuilder) apimarket.ServiceCompletionBuilder {
	if buildCompletion == nil {
		return nil
	}
	return func(service APIService) (IdempotencyCompletion, *domain.AppError) {
		service, appErr := s.withAPIMerchantProfile(ctx, service)
		if appErr != nil {
			return IdempotencyCompletion{}, appErr
		}
		return buildCompletion(service)
	}
}

func (s *Service) withAPIMerchantProfiles(ctx context.Context, services []APIService) ([]APIService, *domain.AppError) {
	enriched := make([]APIService, 0, len(services))
	for _, service := range services {
		value, appErr := s.withAPIMerchantProfile(ctx, service)
		if appErr != nil {
			return nil, appErr
		}
		enriched = append(enriched, value)
	}
	return enriched, nil
}

func (s *Service) withSellerReputation(ctx context.Context, services []APIService) ([]APIService, *domain.AppError) {
	if len(services) == 0 || !s.reputationService.EngineAvailable() {
		return services, nil
	}
	userIDs := make([]string, 0, len(services))
	for _, service := range services {
		userIDs = append(userIDs, service.OwnerUserID)
	}
	snapshots, appErr := s.reputationService.GetMany(ctx, userIDs, reputation.RoleSeller, reputation.ScopeAPI)
	if appErr != nil {
		return nil, appErr
	}
	for index := range services {
		if snapshot, exists := snapshots[services[index].OwnerUserID]; exists {
			copy := snapshot
			services[index].SellerReputation = &copy
		}
	}
	return services, nil
}

func (s *Service) withAPIMerchantProfile(ctx context.Context, service APIService) (APIService, *domain.AppError) {
	if service.MerchantDisplayName != "" && service.MerchantProfileSlug != "" {
		return service, nil
	}
	if service.MerchantIdentityMode == "store_alias" {
		if service.MerchantProfileID == "" {
			return service, nil
		}
		merchant, appErr := s.profileService.MyMerchantProfile(ctx, User{ID: service.OwnerUserID})
		if appErr != nil {
			return APIService{}, appErr
		}
		if merchant.ID != service.MerchantProfileID {
			return APIService{}, domain.NewError(http.StatusConflict, domain.CodeValidationFailed, "Merchant profile mismatch", "API 服务关联的商户资料不可用。")
		}
		service.MerchantDisplayName = merchant.DisplayName
		service.MerchantProfileSlug = merchant.Slug
		service.MerchantAvatarURL = merchant.AvatarURL
		return service, nil
	}
	owner, appErr := s.profileService.MyProfile(ctx, User{ID: service.OwnerUserID})
	if appErr != nil {
		return APIService{}, appErr
	}
	service.MerchantDisplayName = owner.DisplayName
	service.MerchantProfileSlug = owner.Username
	service.MerchantAvatarURL = owner.AvatarURL
	return service, nil
}

func (s *Service) withCarpoolSellerReputation(ctx context.Context, listings []CarpoolListing) ([]CarpoolListing, *domain.AppError) {
	if len(listings) == 0 || !s.reputationService.EngineAvailable() {
		return listings, nil
	}
	userIDs := make([]string, 0, len(listings))
	for _, listing := range listings {
		userIDs = append(userIDs, listing.OwnerUserID)
	}
	snapshots, appErr := s.reputationService.GetMany(ctx, userIDs, reputation.RoleSeller, reputation.ScopeCarpool)
	if appErr != nil {
		return nil, appErr
	}
	for index := range listings {
		if snapshot, exists := snapshots[listings[index].OwnerUserID]; exists {
			copy := snapshot
			listings[index].SellerReputation = &copy
		}
	}
	return listings, nil
}

func (s *Service) withCarpoolBuyerReputation(ctx context.Context, applications []CarpoolApplication) ([]CarpoolApplication, *domain.AppError) {
	if len(applications) == 0 || !s.reputationService.EngineAvailable() {
		return applications, nil
	}
	userIDs := make([]string, 0, len(applications))
	for _, application := range applications {
		userIDs = append(userIDs, application.BuyerUserID)
	}
	snapshots, appErr := s.reputationService.GetMany(ctx, userIDs, reputation.RoleBuyer, reputation.ScopeCarpool)
	if appErr != nil {
		return nil, appErr
	}
	for index := range applications {
		if snapshot, exists := snapshots[applications[index].BuyerUserID]; exists {
			copy := snapshot
			applications[index].BuyerReputation = &copy
		}
	}
	return applications, nil
}

func (s *Service) withAPIOrderReputations(ctx context.Context, orders []APIOrder) ([]APIOrder, *domain.AppError) {
	if len(orders) == 0 || !s.reputationService.EngineAvailable() {
		return orders, nil
	}
	buyerIDs := make([]string, 0, len(orders))
	sellerIDs := make([]string, 0, len(orders))
	for _, order := range orders {
		buyerIDs = append(buyerIDs, order.BuyerUserID)
		sellerIDs = append(sellerIDs, order.SellerUserID)
	}
	buyers, appErr := s.reputationService.GetMany(ctx, buyerIDs, reputation.RoleBuyer, reputation.ScopeAPI)
	if appErr != nil {
		return nil, appErr
	}
	sellers, appErr := s.reputationService.GetMany(ctx, sellerIDs, reputation.RoleSeller, reputation.ScopeAPI)
	if appErr != nil {
		return nil, appErr
	}
	for index := range orders {
		if snapshot, exists := buyers[orders[index].BuyerUserID]; exists {
			copy := snapshot
			orders[index].BuyerReputation = &copy
		}
		if snapshot, exists := sellers[orders[index].SellerUserID]; exists {
			copy := snapshot
			orders[index].SellerReputation = &copy
		}
	}
	return orders, nil
}

func (s *Service) PublicAPIQuotaOffers(ctx context.Context, filter apiquota.PublicOfferFilter, page domain.PageRequest) (domain.Page[apiquota.OfferCard], *domain.AppError) {
	return s.apiQuota.PublicOffers(ctx, filter, page)
}

func (s *Service) APIQuotaSystemSaleSlots() []apiquota.SystemSaleSlot {
	return s.apiQuota.SystemSaleSlots()
}

func (s *Service) PublicAPIQuotaOffer(ctx context.Context, offerID string) (apiquota.OfferCard, *domain.AppError) {
	return s.apiQuota.PublicOffer(ctx, offerID)
}

func (s *Service) OwnerAPIQuotaBatches(ctx context.Context, user User, apiServiceID string, page domain.PageRequest) (domain.Page[apiquota.Batch], *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIQuotaPublish); appErr != nil {
		return domain.Page[apiquota.Batch]{}, appErr
	}
	return s.apiQuota.OwnerBatches(ctx, user, apiServiceID, page)
}

func (s *Service) CreateAPIQuotaBatch(ctx context.Context, user User, input apiquota.CreateBatchInput) (apiquota.Batch, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIQuotaPublish); appErr != nil {
		return apiquota.Batch{}, appErr
	}
	return s.apiQuota.CreateBatch(ctx, user, input)
}

func (s *Service) CreateAPIQuotaBatchWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input apiquota.CreateBatchInput, buildCompletion apiquota.BatchCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIQuotaPublish); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	return s.apiQuota.CreateBatchWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) OwnerAPIQuotaOffers(ctx context.Context, user User, batchID string) ([]apiquota.Offer, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIQuotaPublish); appErr != nil {
		return nil, appErr
	}
	return s.apiQuota.OwnerOffers(ctx, user, batchID)
}

func (s *Service) CreateAPIQuotaOffer(ctx context.Context, user User, input apiquota.CreateOfferInput) (apiquota.Offer, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIQuotaPublish); appErr != nil {
		return apiquota.Offer{}, appErr
	}
	return s.apiQuota.CreateOffer(ctx, user, input)
}

func (s *Service) CreateAPIQuotaOfferWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input apiquota.CreateOfferInput, buildCompletion apiquota.OfferCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIQuotaPublish); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	return s.apiQuota.CreateOfferWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) OwnerAPIQuotaRounds(ctx context.Context, user User, batchID string) ([]apiquota.SaleRound, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIQuotaPublish); appErr != nil {
		return nil, appErr
	}
	return s.apiQuota.OwnerRounds(ctx, user, batchID)
}

func (s *Service) CreateAPIQuotaRound(ctx context.Context, user User, input apiquota.CreateRoundInput) (apiquota.SaleRound, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIQuotaPublish); appErr != nil {
		return apiquota.SaleRound{}, appErr
	}
	return s.apiQuota.CreateRound(ctx, user, input)
}

func (s *Service) CreateAPIQuotaRoundWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input apiquota.CreateRoundInput, buildCompletion apiquota.SaleRoundCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIQuotaPublish); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	return s.apiQuota.CreateRoundWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) ConfirmAPIQuotaRoundFulfillmentWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input apiquota.SaleRoundActionInput, buildCompletion apiquota.SaleRoundCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIQuotaPublish); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	return s.apiQuota.ConfirmRoundFulfillmentWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) PublishAPIQuotaBatch(ctx context.Context, user User, input apiquota.BatchActionInput) (apiquota.Batch, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIQuotaPublish); appErr != nil {
		return apiquota.Batch{}, appErr
	}
	return s.apiQuota.PublishBatch(ctx, user, input)
}

func (s *Service) PublishAPIQuotaBatchWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input apiquota.BatchActionInput, buildCompletion apiquota.BatchCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIQuotaPublish); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	return s.apiQuota.PublishBatchWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) UpdateAPIQuotaBatchStatus(ctx context.Context, user User, input apiquota.BatchActionInput, action string) (apiquota.Batch, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIQuotaPublish); appErr != nil {
		return apiquota.Batch{}, appErr
	}
	return s.apiQuota.UpdateBatchStatus(ctx, user, input, action)
}

func (s *Service) UpdateAPIQuotaBatchStatusWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input apiquota.BatchActionInput, action string, buildCompletion apiquota.BatchCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIQuotaPublish); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	return s.apiQuota.UpdateBatchStatusWithIdempotency(ctx, user, routeKey, key, requestHash, input, action, buildCompletion)
}

func (s *Service) CreateAPIQuotaOrderWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input apiquota.CreateOrderInput, buildCompletion apiorder.CompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIOrderCreate); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	return s.apiQuota.CreateOrderWithIdempotency(ctx, user.ID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) ImportAPIQuotaCredentials(ctx context.Context, user User, input apiquota.CredentialImportInput) (apiquota.CredentialImportResult, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIQuotaPublish); appErr != nil {
		return apiquota.CredentialImportResult{}, appErr
	}
	return s.apiQuota.ImportCredentials(ctx, user, input)
}

func (s *Service) ImportAPIQuotaCredentialsWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input apiquota.CredentialImportInput, buildCompletion apiquota.CredentialImportCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIQuotaPublish); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	return s.apiQuota.ImportCredentialsWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) APIQuotaCredentialSummary(ctx context.Context, user User, offerID string) (apiquota.CredentialSummary, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIQuotaPublish); appErr != nil {
		return apiquota.CredentialSummary{}, appErr
	}
	return s.apiQuota.CredentialSummary(ctx, user, offerID)
}

func (s *Service) CreateAPIQuotaRushOfferWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input apiquota.CreateRushOfferInput, buildCompletion apiquota.RushOfferCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIQuotaPublish); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	return s.apiQuota.CreateRushOfferWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) CreateAPIPurchaseIntentWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input CreateAPIPurchaseIntentInput, buildCompletion APIPurchaseIntentCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIOrderCreate); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	_, completion, _, appErr := s.apiIntent.CreateWithIdempotency(ctx, user.ID, routeKey, key, requestHash, input, buildCompletion)
	if appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	return completion, nil
}

func (s *Service) MyAPIPurchaseIntents(ctx context.Context, user User) ([]APIPurchaseIntent, *domain.AppError) {
	return s.apiIntent.BuyerIntents(ctx, user)
}

func (s *Service) MyAPIPurchaseIntent(ctx context.Context, user User, intentID, requestID string) (APIPurchaseIntent, *domain.AppError) {
	if appErr := s.reputationService.CheckActionAllowed(ctx, user.ID, reputation.RoleBuyer, reputation.ActionContactView); appErr != nil {
		return APIPurchaseIntent{}, appErr
	}
	return s.apiIntent.BuyerIntent(ctx, user, intentID, requestID)
}

func (s *Service) OwnerAPIPurchaseIntents(ctx context.Context, user User) ([]APIPurchaseIntent, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIServicePublish); appErr != nil {
		return nil, appErr
	}
	return s.apiIntent.OwnerIntents(ctx, user)
}

func (s *Service) OwnerAPIPurchaseIntent(ctx context.Context, user User, intentID, requestID string) (APIPurchaseIntent, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIServicePublish); appErr != nil {
		return APIPurchaseIntent{}, appErr
	}
	if appErr := s.reputationService.CheckActionAllowed(ctx, user.ID, reputation.RoleSeller, reputation.ActionContactView); appErr != nil {
		return APIPurchaseIntent{}, appErr
	}
	return s.apiIntent.OwnerIntent(ctx, user, intentID, requestID)
}

func (s *Service) AdminAPIPurchaseIntents(ctx context.Context, user User) ([]APIPurchaseIntent, *domain.AppError) {
	return s.apiIntent.AdminIntents(ctx, user)
}

func (s *Service) AdminAPIPurchaseIntent(ctx context.Context, user User, intentID string) (APIPurchaseIntent, *domain.AppError) {
	return s.apiIntent.AdminIntent(ctx, user, intentID)
}

func (s *Service) CancelAPIPurchaseIntentWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input APIPurchaseIntentActionInput, buildCompletion APIPurchaseIntentCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.apiIntent.CancelWithIdempotency(ctx, userID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) MarkAPIPurchaseIntentContactedWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input APIPurchaseIntentActionInput, buildCompletion APIPurchaseIntentCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIServicePublish); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	return s.apiIntent.MarkContactedWithIdempotency(ctx, user.ID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) CloseAPIPurchaseIntentWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input APIPurchaseIntentActionInput, buildCompletion APIPurchaseIntentCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIServicePublish); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	return s.apiIntent.CloseWithIdempotency(ctx, user.ID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) CreateAPIOrderWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input APIOrderActionInput, createInput CreateAPIOrderInput, buildCompletion APIOrderCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIOrderCreate); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	_ = input
	if appErr := s.reputationService.CheckActionAllowed(ctx, user.ID, reputation.RoleBuyer, reputation.ActionAPIOrderCreate); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	order, completion, created, appErr := s.apiOrder.CreateWithIdempotencyResult(ctx, user.ID, routeKey, key, requestHash, createInput, buildCompletion)
	if appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	if created {
		s.sendAPIOrderEmailIfNeeded(ctx, order)
	}
	return completion, nil
}

func (s *Service) sendAPIOrderEmailIfNeeded(ctx context.Context, order APIOrder) {
	if s == nil || s.emailSender == nil || s.profileService == nil {
		return
	}
	ownerProfile, appErr := s.profileService.MyProfile(ctx, User{ID: order.SellerUserID})
	if appErr != nil {
		log.Printf("API 订单邮件跳过：读取商户资料失败 order_id=%s seller_user_id=%s code=%s title=%s", order.ID, order.SellerUserID, appErr.Code, appErr.Title)
		return
	}
	if strings.TrimSpace(ownerProfile.Email) == "" || ownerProfile.EmailVerifiedAt == nil {
		return
	}
	if appErr := s.emailSender.SendAPIOrderCreated(ctx, ownerProfile.Email, order.ServiceTitleSnapshot, order.ID, order.Amount, order.Currency, order.PaymentExpiresAt, order.CreatedAt); appErr != nil {
		log.Printf("API 订单邮件发送失败 order_id=%s seller_user_id=%s code=%s title=%s", order.ID, order.SellerUserID, appErr.Code, appErr.Title)
	}
}

func (s *Service) MyAPIOrders(ctx context.Context, user User) ([]APIOrder, *domain.AppError) {
	orders, appErr := s.apiOrder.BuyerOrders(ctx, user)
	if appErr != nil {
		return nil, appErr
	}
	return s.withAPIOrderReputations(ctx, orders)
}

func (s *Service) APIOrdersForActor(ctx context.Context, actor authmodule.BusinessActor, participantRole string) ([]APIOrder, *domain.AppError) {
	if actor.Audience == authmodule.SessionAudienceNormal && participantRole == "seller" {
		if appErr := authmodule.RequireProjectedCapability(actor.Capabilities, authmodule.CapabilityAPIServicePublish); appErr != nil {
			return nil, appErr
		}
	}
	orders, appErr := s.apiOrder.OrdersForActor(ctx, actor, participantRole)
	if appErr != nil {
		return nil, appErr
	}
	return s.withAPIOrderReputations(ctx, orders)
}

func (s *Service) MyAPIOrder(ctx context.Context, user User, orderID string) (APIOrder, *domain.AppError) {
	order, appErr := s.apiOrder.BuyerOrder(ctx, user, orderID)
	if appErr != nil {
		return APIOrder{}, appErr
	}
	orders, appErr := s.withAPIOrderReputations(ctx, []APIOrder{order})
	if appErr != nil {
		return APIOrder{}, appErr
	}
	return orders[0], nil
}

func (s *Service) APIOrderForActor(ctx context.Context, actor authmodule.BusinessActor, orderID, participantRole string) (APIOrder, *domain.AppError) {
	if actor.Audience == authmodule.SessionAudienceNormal && participantRole == "seller" {
		if appErr := authmodule.RequireProjectedCapability(actor.Capabilities, authmodule.CapabilityAPIServicePublish); appErr != nil {
			return APIOrder{}, appErr
		}
	}
	order, appErr := s.apiOrder.OrderForActor(ctx, actor, orderID, participantRole)
	if appErr != nil {
		return APIOrder{}, appErr
	}
	orders, appErr := s.withAPIOrderReputations(ctx, []APIOrder{order})
	if appErr != nil {
		return APIOrder{}, appErr
	}
	return orders[0], nil
}

func (s *Service) ReadAPIOrderPaymentInstructions(ctx context.Context, user User, orderID, requestID string) (APIOrderPaymentInstructionsView, *domain.AppError) {
	return s.apiOrder.ReadPaymentInstructions(ctx, user, orderID, requestID)
}

func (s *Service) OwnerAPIOrders(ctx context.Context, user User) ([]APIOrder, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIServicePublish); appErr != nil {
		return nil, appErr
	}
	orders, appErr := s.apiOrder.SellerOrders(ctx, user)
	if appErr != nil {
		return nil, appErr
	}
	return s.withAPIOrderReputations(ctx, orders)
}

func (s *Service) AdminAPIOrders(ctx context.Context, user User, filter apiorder.AdminOrderFilter, pageRequest domain.PageRequest) (domain.Page[APIOrder], *domain.AppError) {
	page, appErr := s.apiOrder.AdminOrders(ctx, user, filter, pageRequest)
	if appErr != nil {
		return domain.Page[APIOrder]{}, appErr
	}
	orders, appErr := s.withAPIOrderReputations(ctx, page.Items)
	if appErr != nil {
		return domain.Page[APIOrder]{}, appErr
	}
	return domain.Page[APIOrder]{Items: orders, NextCursor: page.NextCursor}, nil
}

func (s *Service) AdminAPIOrder(ctx context.Context, user User, orderID string) (APIOrder, *domain.AppError) {
	order, appErr := s.apiOrder.AdminOrder(ctx, user, orderID)
	if appErr != nil {
		return APIOrder{}, appErr
	}
	orders, appErr := s.withAPIOrderReputations(ctx, []APIOrder{order})
	if appErr != nil {
		return APIOrder{}, appErr
	}
	return orders[0], nil
}

func (s *Service) ResolveAPIOrderCatalogRiskHoldWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input apiorder.CatalogRiskHoldActionInput, buildCompletion APIOrderCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.apiOrder.ResolveCatalogRiskHoldWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) OwnerAPIOrder(ctx context.Context, user User, orderID string) (APIOrder, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIServicePublish); appErr != nil {
		return APIOrder{}, appErr
	}
	order, appErr := s.apiOrder.SellerOrder(ctx, user, orderID)
	if appErr != nil {
		return APIOrder{}, appErr
	}
	orders, appErr := s.withAPIOrderReputations(ctx, []APIOrder{order})
	if appErr != nil {
		return APIOrder{}, appErr
	}
	return orders[0], nil
}

func (s *Service) SubmitAPIOrderPaymentWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input APIOrderActionInput, buildCompletion APIOrderCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.apiOrder.SubmitPaymentWithIdempotency(ctx, userID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) CancelAPIOrderWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input APIOrderActionInput, buildCompletion APIOrderCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.apiOrder.CancelWithIdempotency(ctx, userID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) ConfirmAPIOrderCompleteWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input APIOrderActionInput, buildCompletion APIOrderCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.apiOrder.ConfirmCompleteWithIdempotency(ctx, userID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) ConfirmAPIOrderCompleteForActorWithIdempotency(ctx context.Context, actor authmodule.BusinessActor, routeKey, key, requestHash string, input APIOrderActionInput, buildCompletion APIOrderCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.apiOrder.ConfirmCompleteForActorWithIdempotency(ctx, actor, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) OpenAPIOrderDisputeWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input APIOrderActionInput, buildCompletion APIOrderCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.apiOrder.OpenDisputeWithIdempotency(ctx, userID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) OpenAPIOrderDisputeForActorWithIdempotency(ctx context.Context, actor authmodule.BusinessActor, routeKey, key, requestHash string, input APIOrderActionInput, buildCompletion APIOrderCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if actor.Audience == authmodule.SessionAudienceNormal && input.ParticipantRole == "seller" {
		if appErr := authmodule.RequireProjectedCapability(actor.Capabilities, authmodule.CapabilityAPIServicePublish); appErr != nil {
			return IdempotencyCompletion{}, appErr
		}
	}
	return s.apiOrder.OpenDisputeForActorWithIdempotency(ctx, actor, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) ConfirmAPIOrderPaymentForActorWithIdempotency(ctx context.Context, actor authmodule.BusinessActor, routeKey, key, requestHash string, input APIOrderActionInput, buildCompletion APIOrderCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if actor.Audience == authmodule.SessionAudienceNormal {
		if appErr := authmodule.RequireProjectedCapability(actor.Capabilities, authmodule.CapabilityAPIServicePublish); appErr != nil {
			return IdempotencyCompletion{}, appErr
		}
	}
	return s.apiOrder.ConfirmPaymentForActorWithIdempotency(ctx, actor, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) ReportAPIOrderPaymentIssueForActorWithIdempotency(ctx context.Context, actor authmodule.BusinessActor, routeKey, key, requestHash string, input APIOrderActionInput, buildCompletion APIOrderCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if actor.Audience == authmodule.SessionAudienceNormal {
		if appErr := authmodule.RequireProjectedCapability(actor.Capabilities, authmodule.CapabilityAPIServicePublish); appErr != nil {
			return IdempotencyCompletion{}, appErr
		}
	}
	return s.apiOrder.ReportPaymentIssueForActorWithIdempotency(ctx, actor, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) SubmitAPIOrderDeliveryForActorWithIdempotency(ctx context.Context, actor authmodule.BusinessActor, routeKey, key, requestHash string, input APIOrderActionInput, buildCompletion APIOrderCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if actor.Audience == authmodule.SessionAudienceNormal {
		if appErr := authmodule.RequireProjectedCapability(actor.Capabilities, authmodule.CapabilityAPIServicePublish); appErr != nil {
			return IdempotencyCompletion{}, appErr
		}
	}
	return s.apiOrder.SubmitDeliveryForActorWithIdempotency(ctx, actor, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) OpenOwnerAPIOrderDisputeWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input APIOrderActionInput, buildCompletion APIOrderCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIServicePublish); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	return s.apiOrder.OpenDisputeWithIdempotency(ctx, user.ID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) ConfirmAPIOrderPaymentWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input APIOrderActionInput, buildCompletion APIOrderCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIServicePublish); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	return s.apiOrder.ConfirmPaymentWithIdempotency(ctx, user.ID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) ReportAPIOrderPaymentIssueWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input APIOrderActionInput, buildCompletion APIOrderCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIServicePublish); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	return s.apiOrder.ReportPaymentIssueWithIdempotency(ctx, user.ID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) SubmitAPIOrderDeliveryWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input APIOrderActionInput, buildCompletion APIOrderCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIServicePublish); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	return s.apiOrder.SubmitDeliveryWithIdempotency(ctx, user.ID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) ReportLateAPIOrderPaymentWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input APIOrderActionInput, buildCompletion APIOrderCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.apiOrder.ReportLatePaymentWithIdempotency(ctx, userID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) ResolveLateAPIOrderPaymentWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input APIOrderActionInput, buildCompletion APIOrderCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIServicePublish); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	return s.apiOrder.ResolveLatePaymentWithIdempotency(ctx, user.ID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) CreateCarpoolListing(ctx context.Context, user User, input CreateCarpoolListingInput) (CarpoolListing, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityCarpoolPublish); appErr != nil {
		return CarpoolListing{}, appErr
	}
	return s.carpoolService.CreateListing(ctx, user, input)
}

func (s *Service) CreateCarpoolListingWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input CreateCarpoolListingInput, buildCompletion carpool.ListingCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityCarpoolPublish); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	_, completion, _, appErr := s.carpoolService.CreateListingWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
	return completion, appErr
}

func (s *Service) PublishCarpoolListing(ctx context.Context, user User, input PublishCarpoolListingInput) (CarpoolListing, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityCarpoolPublish); appErr != nil {
		return CarpoolListing{}, appErr
	}
	if appErr := s.reputationService.CheckActionAllowed(ctx, user.ID, reputation.RoleSeller, reputation.ActionCarpoolPublish); appErr != nil {
		return CarpoolListing{}, appErr
	}
	return s.carpoolService.PublishListing(ctx, user, input)
}

func (s *Service) PublishCarpoolListingWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input PublishCarpoolListingInput, buildCompletion carpool.ListingCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityCarpoolPublish); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	if appErr := s.reputationService.CheckActionAllowed(ctx, user.ID, reputation.RoleSeller, reputation.ActionCarpoolPublish); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	_, completion, _, appErr := s.carpoolService.PublishListingWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
	return completion, appErr
}

func (s *Service) UpdateCarpoolListing(ctx context.Context, user User, input UpdateCarpoolListingInput) (CarpoolListing, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityCarpoolPublish); appErr != nil {
		return CarpoolListing{}, appErr
	}
	return s.carpoolService.UpdateListing(ctx, user, input)
}

func (s *Service) UpdateCarpoolListingWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input UpdateCarpoolListingInput, buildCompletion carpool.ListingCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityCarpoolPublish); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	_, completion, _, appErr := s.carpoolService.UpdateListingWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
	return completion, appErr
}

func (s *Service) SubmitCarpoolListingForReview(ctx context.Context, user User, input SubmitCarpoolListingReviewInput) (CarpoolListing, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityCarpoolPublish); appErr != nil {
		return CarpoolListing{}, appErr
	}
	if appErr := s.reputationService.CheckActionAllowed(ctx, user.ID, reputation.RoleSeller, reputation.ActionCarpoolPublish); appErr != nil {
		return CarpoolListing{}, appErr
	}
	return s.carpoolService.SubmitListingForReview(ctx, user, input)
}

func (s *Service) SubmitCarpoolListingForReviewWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input SubmitCarpoolListingReviewInput, buildCompletion carpool.ListingCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityCarpoolPublish); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	if appErr := s.reputationService.CheckActionAllowed(ctx, user.ID, reputation.RoleSeller, reputation.ActionCarpoolPublish); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	_, completion, _, appErr := s.carpoolService.SubmitListingForReviewWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
	return completion, appErr
}

func (s *Service) PublicCarpoolListings(ctx context.Context, filter carpool.ListingFilter, page domain.PageRequest) (domain.Page[CarpoolListing], *domain.AppError) {
	listings, appErr := s.carpoolService.PublicListings(ctx, filter, page)
	if appErr != nil {
		return domain.Page[CarpoolListing]{}, appErr
	}
	items, appErr := s.withCarpoolSellerReputation(ctx, listings.Items)
	if appErr != nil {
		return domain.Page[CarpoolListing]{}, appErr
	}
	return domain.Page[CarpoolListing]{Items: items, NextCursor: listings.NextCursor}, nil
}

func (s *Service) PublicCarpoolListing(ctx context.Context, listingID string) (CarpoolListing, *domain.AppError) {
	listing, appErr := s.carpoolService.PublicListing(ctx, listingID)
	if appErr != nil {
		return CarpoolListing{}, appErr
	}
	items, appErr := s.withCarpoolSellerReputation(ctx, []CarpoolListing{listing})
	if appErr != nil {
		return CarpoolListing{}, appErr
	}
	return items[0], nil
}

func (s *Service) CarpoolApplicationEligibility(ctx context.Context, user User, listingID string) (carpool.ApplicationEligibility, *domain.AppError) {
	return s.carpoolService.ApplicationEligibility(ctx, user, listingID)
}

func (s *Service) MyCarpoolListings(ctx context.Context, user User, view string, page domain.PageRequest) (domain.Page[CarpoolListing], *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityCarpoolPublish); appErr != nil {
		return domain.Page[CarpoolListing]{}, appErr
	}
	return s.carpoolService.MyListings(ctx, user, view, page)
}

func (s *Service) MyCarpoolListing(ctx context.Context, user User, listingID string) (CarpoolListing, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityCarpoolPublish); appErr != nil {
		return CarpoolListing{}, appErr
	}
	return s.carpoolService.MyListing(ctx, user, listingID)
}

func (s *Service) AdminCarpoolListings(ctx context.Context, user User, filter carpool.ListingFilter, page domain.PageRequest) (domain.Page[CarpoolListing], *domain.AppError) {
	return s.carpoolService.AdminListings(ctx, user, filter, page)
}

func (s *Service) AdminCarpoolListing(ctx context.Context, user User, listingID string) (CarpoolListing, *domain.AppError) {
	return s.carpoolService.AdminListing(ctx, user, listingID)
}

func (s *Service) UpdateCarpoolListingReviewStatus(ctx context.Context, user User, input CarpoolReviewInput) (CarpoolListing, *domain.AppError) {
	return s.carpoolService.UpdateListingReviewStatus(ctx, user, input)
}

func (s *Service) UpdateCarpoolListingReviewStatusWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input CarpoolReviewInput, buildCompletion carpool.ListingCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	_, completion, _, appErr := s.carpoolService.UpdateListingReviewStatusWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
	return completion, appErr
}

func (s *Service) CreateCarpoolApplication(ctx context.Context, user User, input CreateCarpoolApplicationInput) (CarpoolApplication, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityCarpoolApply); appErr != nil {
		return CarpoolApplication{}, appErr
	}
	if appErr := s.reputationService.CheckActionAllowed(ctx, user.ID, reputation.RoleBuyer, reputation.ActionCarpoolApply); appErr != nil {
		return CarpoolApplication{}, appErr
	}
	application, appErr := s.carpoolService.CreateApplication(ctx, user, input)
	if appErr != nil {
		return CarpoolApplication{}, appErr
	}
	s.sendCarpoolApplicationEmailIfNeeded(ctx, application)
	return application, nil
}

func (s *Service) CreateCarpoolApplicationWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input CreateCarpoolApplicationInput, buildCompletion CarpoolApplicationCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityCarpoolApply); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	application, completion, created, appErr := s.carpoolService.CreateApplicationWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
	if appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	if created {
		s.sendCarpoolApplicationEmailIfNeeded(ctx, application)
	}
	return completion, nil
}

func (s *Service) sendCarpoolApplicationEmailIfNeeded(ctx context.Context, application CarpoolApplication) {
	if s == nil || s.emailSender == nil || s.profileService == nil {
		return
	}
	ownerProfile, appErr := s.profileService.MyProfile(ctx, User{ID: application.OwnerUserID})
	if appErr != nil {
		log.Printf("上车申请邮件跳过：读取车主资料失败 application_id=%s owner_user_id=%s code=%s title=%s", application.ID, application.OwnerUserID, appErr.Code, appErr.Title)
		return
	}
	if strings.TrimSpace(ownerProfile.Email) == "" || ownerProfile.EmailVerifiedAt == nil {
		return
	}
	if appErr := s.emailSender.SendCarpoolApplicationCreated(ctx, ownerProfile.Email, application.ListingTitleSnapshot, application.ID, application.CreatedAt); appErr != nil {
		log.Printf("上车申请邮件发送失败 application_id=%s owner_user_id=%s code=%s title=%s", application.ID, application.OwnerUserID, appErr.Code, appErr.Title)
	}
}

func (s *Service) MyCarpoolApplications(ctx context.Context, user User) ([]CarpoolApplication, *domain.AppError) {
	return s.carpoolService.MyApplications(ctx, user)
}

func (s *Service) CarpoolApplicationsForActor(ctx context.Context, actor authmodule.BusinessActor, participantRole string) ([]CarpoolApplication, *domain.AppError) {
	if actor.Audience == authmodule.SessionAudienceNormal && participantRole == CarpoolJoinActorOwner {
		if appErr := authmodule.RequireProjectedCapability(actor.Capabilities, authmodule.CapabilityCarpoolPublish); appErr != nil {
			return nil, appErr
		}
	}
	applications, appErr := s.carpoolService.ApplicationsForActor(ctx, actor, participantRole)
	if appErr != nil || actor.Audience != authmodule.SessionAudienceNormal || participantRole != CarpoolJoinActorOwner {
		return applications, appErr
	}
	return s.withCarpoolBuyerReputation(ctx, applications)
}

func (s *Service) CarpoolApplicationForActor(ctx context.Context, actor authmodule.BusinessActor, applicationID, participantRole string) (CarpoolApplication, *domain.AppError) {
	if actor.Audience == authmodule.SessionAudienceNormal && participantRole == CarpoolJoinActorOwner {
		if appErr := authmodule.RequireProjectedCapability(actor.Capabilities, authmodule.CapabilityCarpoolPublish); appErr != nil {
			return CarpoolApplication{}, appErr
		}
	}
	application, appErr := s.carpoolService.ApplicationForActor(ctx, actor, applicationID, participantRole)
	if appErr != nil || actor.Audience != authmodule.SessionAudienceNormal || participantRole != CarpoolJoinActorOwner {
		return application, appErr
	}
	items, appErr := s.withCarpoolBuyerReputation(ctx, []CarpoolApplication{application})
	if appErr != nil {
		return CarpoolApplication{}, appErr
	}
	return items[0], nil
}

func (s *Service) MyCarpoolApplication(ctx context.Context, user User, applicationID string) (CarpoolApplication, *domain.AppError) {
	return s.carpoolService.MyApplication(ctx, user, applicationID)
}

func (s *Service) OwnerCarpoolApplications(ctx context.Context, user User) ([]CarpoolApplication, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityCarpoolPublish); appErr != nil {
		return nil, appErr
	}
	applications, appErr := s.carpoolService.OwnerApplications(ctx, user)
	if appErr != nil {
		return nil, appErr
	}
	return s.withCarpoolBuyerReputation(ctx, applications)
}

func (s *Service) OwnerCarpoolApplication(ctx context.Context, user User, applicationID string) (CarpoolApplication, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityCarpoolPublish); appErr != nil {
		return CarpoolApplication{}, appErr
	}
	application, appErr := s.carpoolService.OwnerApplication(ctx, user, applicationID)
	if appErr != nil {
		return CarpoolApplication{}, appErr
	}
	items, appErr := s.withCarpoolBuyerReputation(ctx, []CarpoolApplication{application})
	if appErr != nil {
		return CarpoolApplication{}, appErr
	}
	return items[0], nil
}

func (s *Service) AcceptCarpoolApplicationWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input AcceptCarpoolApplicationInput, buildCompletion CarpoolApplicationCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityCarpoolPublish); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	if appErr := s.reputationService.CheckActionAllowed(ctx, user.ID, reputation.RoleSeller, reputation.ActionCarpoolAccept); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	application, completion, accepted, appErr := s.carpoolService.AcceptApplicationWithIdempotency(ctx, user.ID, routeKey, key, requestHash, input, buildCompletion)
	if appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	if accepted {
		s.sendCarpoolApplicationAcceptedEmailIfNeeded(ctx, application)
	}
	return completion, nil
}

func (s *Service) sendCarpoolApplicationAcceptedEmailIfNeeded(ctx context.Context, application CarpoolApplication) {
	if s == nil || s.emailSender == nil || s.profileService == nil {
		return
	}
	buyerProfile, appErr := s.profileService.MyProfile(ctx, User{ID: application.BuyerUserID})
	if appErr != nil {
		log.Printf("上车申请接受邮件跳过：读取买家资料失败 application_id=%s buyer_user_id=%s code=%s title=%s", application.ID, application.BuyerUserID, appErr.Code, appErr.Title)
		return
	}
	if strings.TrimSpace(buyerProfile.Email) == "" || buyerProfile.EmailVerifiedAt == nil {
		return
	}
	if appErr := s.emailSender.SendCarpoolApplicationAccepted(ctx, buyerProfile.Email, application.ListingTitleSnapshot, application.ID, application.JoinConfirmationDeadline); appErr != nil {
		log.Printf("上车申请接受邮件发送失败 application_id=%s buyer_user_id=%s code=%s title=%s", application.ID, application.BuyerUserID, appErr.Code, appErr.Title)
	}
}

func (s *Service) RejectCarpoolApplication(ctx context.Context, user User, input RejectCarpoolApplicationInput) (CarpoolApplication, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityCarpoolPublish); appErr != nil {
		return CarpoolApplication{}, appErr
	}
	input.OwnerUserID = user.ID
	return s.carpoolService.RejectApplication(ctx, input)
}

func (s *Service) RejectCarpoolApplicationWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input RejectCarpoolApplicationInput, buildCompletion CarpoolApplicationCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityCarpoolPublish); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	_, completion, _, appErr := s.carpoolService.RejectApplicationWithIdempotency(ctx, user.ID, routeKey, key, requestHash, input, buildCompletion)
	return completion, appErr
}

func (s *Service) CancelCarpoolApplicationWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input CancelCarpoolApplicationInput, buildCompletion CarpoolApplicationCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.carpoolService.CancelApplicationWithIdempotency(ctx, userID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) WithdrawCarpoolAcceptanceWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input WithdrawCarpoolAcceptanceInput, buildCompletion CarpoolApplicationCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityCarpoolPublish); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	return s.carpoolService.WithdrawAcceptanceWithIdempotency(ctx, user.ID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) ConfirmCarpoolApplicationJoinWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input ConfirmCarpoolApplicationJoinInput, buildCompletion CarpoolApplicationCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if input.ActorRole == CarpoolJoinActorOwner {
		if appErr := authmodule.RequireCapability(user, authmodule.CapabilityCarpoolPublish); appErr != nil {
			return IdempotencyCompletion{}, appErr
		}
	}
	return s.carpoolService.ConfirmApplicationJoinWithIdempotency(ctx, user.ID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) MyCarpoolMemberships(ctx context.Context, user User) ([]CarpoolMembership, *domain.AppError) {
	return s.carpoolService.MyMemberships(ctx, user)
}

func (s *Service) CarpoolMembershipsForActor(ctx context.Context, actor authmodule.BusinessActor, participantRole string) ([]CarpoolMembership, *domain.AppError) {
	if actor.Audience == authmodule.SessionAudienceNormal && participantRole == CarpoolJoinActorOwner {
		if appErr := authmodule.RequireProjectedCapability(actor.Capabilities, authmodule.CapabilityCarpoolPublish); appErr != nil {
			return nil, appErr
		}
	}
	return s.carpoolService.MembershipsForActor(ctx, actor, participantRole)
}

func (s *Service) ConfirmCarpoolMembershipCompleteForActorWithIdempotency(ctx context.Context, actor authmodule.BusinessActor, routeKey, key, requestHash string, input ConfirmCarpoolMembershipCompleteInput, buildCompletion CarpoolMembershipCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if actor.Audience == authmodule.SessionAudienceNormal && input.ActorRole == CarpoolJoinActorOwner {
		if appErr := authmodule.RequireProjectedCapability(actor.Capabilities, authmodule.CapabilityCarpoolPublish); appErr != nil {
			return IdempotencyCompletion{}, appErr
		}
	}
	return s.carpoolService.ConfirmMembershipCompleteForActorWithIdempotency(ctx, actor, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) EndCarpoolMembershipForActorWithIdempotency(ctx context.Context, actor authmodule.BusinessActor, routeKey, key, requestHash string, input EndCarpoolMembershipInput, buildCompletion CarpoolMembershipCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if actor.Audience == authmodule.SessionAudienceNormal && input.ActorRole == CarpoolJoinActorOwner {
		if appErr := authmodule.RequireProjectedCapability(actor.Capabilities, authmodule.CapabilityCarpoolPublish); appErr != nil {
			return IdempotencyCompletion{}, appErr
		}
	}
	return s.carpoolService.EndMembershipForActorWithIdempotency(ctx, actor, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) MyCarpoolMembershipsByUserID(ctx context.Context, userID string) ([]CarpoolMembership, *domain.AppError) {
	return s.carpoolService.MyMemberships(ctx, User{ID: userID})
}

func (s *Service) OwnerCarpoolMemberships(ctx context.Context, user User) ([]CarpoolMembership, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityCarpoolPublish); appErr != nil {
		return nil, appErr
	}
	return s.carpoolService.OwnerMemberships(ctx, user)
}

func (s *Service) ConfirmCarpoolMembershipCompleteWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input ConfirmCarpoolMembershipCompleteInput, buildCompletion CarpoolMembershipCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if input.ActorRole == CarpoolJoinActorOwner {
		if appErr := authmodule.RequireCapability(user, authmodule.CapabilityCarpoolPublish); appErr != nil {
			return IdempotencyCompletion{}, appErr
		}
	}
	return s.carpoolService.ConfirmMembershipCompleteWithIdempotency(ctx, user.ID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) EndCarpoolMembershipWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input EndCarpoolMembershipInput, buildCompletion CarpoolMembershipCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	if input.ActorRole == CarpoolJoinActorOwner {
		if appErr := authmodule.RequireCapability(user, authmodule.CapabilityCarpoolPublish); appErr != nil {
			return IdempotencyCompletion{}, appErr
		}
	}
	return s.carpoolService.EndMembershipWithIdempotency(ctx, user.ID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) CreateContactMethod(ctx context.Context, input ContactMethodInput) (ContactMethod, *domain.AppError) {
	if strings.TrimSpace(input.Type) == "linuxdo" {
		return ContactMethod{}, identityManagedContactError()
	}
	if appErr := s.requireContactUsageScopeCapabilities(ctx, input.UserID, input.UsageScopes); appErr != nil {
		return ContactMethod{}, appErr
	}
	return s.contactService.CreateMethod(ctx, input)
}

func (s *Service) CreateContactMethodWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input ContactMethodInput, buildCompletion contactmodule.MethodCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	input.UserID = user.ID
	if strings.TrimSpace(input.Type) == "linuxdo" {
		return IdempotencyCompletion{}, identityManagedContactError()
	}
	if appErr := s.requireContactUsageScopeCapabilities(ctx, user.ID, input.UsageScopes); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	_, completion, _, appErr := s.contactService.CreateMethodWithIdempotency(ctx, user.ID, routeKey, key, requestHash, input, buildCompletion)
	return completion, appErr
}

func (s *Service) ListContactMethods(ctx context.Context, userID string) ([]ContactMethod, *domain.AppError) {
	user, appErr := s.authService.UserByID(ctx, userID)
	if appErr != nil {
		return nil, appErr
	}
	if user.LinuxDoBinding != nil && user.LinuxDoBinding.Bound {
		if _, appErr := s.contactService.EnsureLinuxDoMethod(ctx, user.ID, user.LinuxDoBinding.LinuxDoUsername); appErr != nil {
			return nil, appErr
		}
	}
	return s.contactService.ListMethods(ctx, userID)
}

func (s *Service) UpdateContactMethod(ctx context.Context, input contactmodule.UpdateContactMethodInput) (ContactMethod, *domain.AppError) {
	isLinuxDo, appErr := s.isLinuxDoContactMethod(ctx, input.UserID, input.MethodID)
	if appErr != nil {
		return ContactMethod{}, appErr
	}
	if strings.TrimSpace(input.Type) == "linuxdo" || isLinuxDo {
		return ContactMethod{}, identityManagedContactError()
	}
	effectiveScopes := input.UsageScopes
	if effectiveScopes == nil {
		methods, listErr := s.contactService.ListMethods(ctx, input.UserID)
		if listErr != nil {
			return ContactMethod{}, listErr
		}
		for _, method := range methods {
			if method.ID == input.MethodID {
				effectiveScopes = method.UsageScopes
				break
			}
		}
	}
	if appErr := s.requireContactUsageScopeCapabilities(ctx, input.UserID, effectiveScopes); appErr != nil {
		return ContactMethod{}, appErr
	}
	return s.contactService.UpdateMethod(ctx, input)
}

func (s *Service) UpdateContactMethodWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input contactmodule.UpdateContactMethodInput, buildCompletion contactmodule.MethodCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	input.UserID = user.ID
	isLinuxDo, appErr := s.isLinuxDoContactMethod(ctx, user.ID, input.MethodID)
	if appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	if strings.TrimSpace(input.Type) == "linuxdo" || isLinuxDo {
		return IdempotencyCompletion{}, identityManagedContactError()
	}
	effectiveScopes := input.UsageScopes
	if effectiveScopes == nil {
		methods, listErr := s.contactService.ListMethods(ctx, user.ID)
		if listErr != nil {
			return IdempotencyCompletion{}, listErr
		}
		for _, method := range methods {
			if method.ID == input.MethodID {
				effectiveScopes = method.UsageScopes
				break
			}
		}
	}
	if appErr := s.requireContactUsageScopeCapabilities(ctx, user.ID, effectiveScopes); appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	_, completion, _, appErr := s.contactService.UpdateMethodWithIdempotency(ctx, user.ID, routeKey, key, requestHash, input, buildCompletion)
	return completion, appErr
}

func (s *Service) requireContactUsageScopeCapabilities(ctx context.Context, userID string, usageScopes []string) *domain.AppError {
	requiresCarpoolPublish := false
	requiresAPIServicePublish := false
	for _, scope := range usageScopes {
		switch scope {
		case contactmodule.UsageScopeCarpoolOwner:
			requiresCarpoolPublish = true
		case contactmodule.UsageScopeAPIMerchant:
			requiresAPIServicePublish = true
		}
	}
	if !requiresCarpoolPublish && !requiresAPIServicePublish {
		return nil
	}
	user, appErr := s.authService.UserByID(ctx, userID)
	if appErr != nil {
		return appErr
	}
	if requiresCarpoolPublish {
		if appErr := authmodule.RequireCapability(user, authmodule.CapabilityCarpoolPublish); appErr != nil {
			return appErr
		}
	}
	if requiresAPIServicePublish {
		if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIServicePublish); appErr != nil {
			return appErr
		}
	}
	return nil
}

func (s *Service) DeleteContactMethod(ctx context.Context, userID, methodID string) (ContactMethod, *domain.AppError) {
	isLinuxDo, appErr := s.isLinuxDoContactMethod(ctx, userID, methodID)
	if appErr != nil {
		return ContactMethod{}, appErr
	}
	if isLinuxDo {
		return ContactMethod{}, identityManagedContactError()
	}
	return s.contactService.DeleteMethod(ctx, userID, methodID)
}

func (s *Service) DeleteContactMethodWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash, methodID, requestID string, buildCompletion contactmodule.MethodCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	isLinuxDo, appErr := s.isLinuxDoContactMethod(ctx, user.ID, methodID)
	if appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	if isLinuxDo {
		return IdempotencyCompletion{}, identityManagedContactError()
	}
	_, completion, _, appErr := s.contactService.DeleteMethodWithIdempotency(ctx, user.ID, routeKey, key, requestHash, methodID, requestID, buildCompletion)
	return completion, appErr
}

func (s *Service) isLinuxDoContactMethod(ctx context.Context, userID, methodID string) (bool, *domain.AppError) {
	methods, appErr := s.contactService.ListMethods(ctx, userID)
	if appErr != nil {
		return false, appErr
	}
	for _, method := range methods {
		if method.ID == methodID {
			return method.Type == "linuxdo", nil
		}
	}
	return false, nil
}

func identityManagedContactError() *domain.AppError {
	return domain.NewError(http.StatusConflict, domain.CodeInvalidStateTransition, "Identity-managed contact method", "linux.do 联系方式来自当前账号绑定，只能随身份绑定同步，不能手动新增、修改或删除。")
}

func (s *Service) SetDefaultContactMethod(ctx context.Context, userID, methodID string) (ContactMethod, *domain.AppError) {
	return s.contactService.SetDefaultMethod(ctx, userID, methodID)
}

func (s *Service) SetDefaultContactMethodWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash, methodID, requestID string, buildCompletion contactmodule.MethodCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	_, completion, _, appErr := s.contactService.SetDefaultMethodWithIdempotency(ctx, user.ID, routeKey, key, requestHash, methodID, requestID, buildCompletion)
	return completion, appErr
}

func (s *Service) VerifyContactMethod(ctx context.Context, userID, methodID string) (ContactMethod, *domain.AppError) {
	return s.contactService.VerifyMethod(ctx, userID, methodID)
}

func (s *Service) VerifyContactMethodWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash, methodID, requestID string, buildCompletion contactmodule.MethodCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	_, completion, _, appErr := s.contactService.VerifyMethodWithIdempotency(ctx, user.ID, routeKey, key, requestHash, methodID, requestID, buildCompletion)
	return completion, appErr
}

func (s *Service) CreateContactSession(ctx context.Context, input CreateContactSessionInput) (ContactSession, *domain.AppError) {
	return s.contactService.CreateSession(ctx, input)
}

func (s *Service) ReadContactSession(ctx context.Context, sessionID, viewerUserID, requestID string) (ContactSessionView, *domain.AppError) {
	return s.contactService.ReadSession(ctx, sessionID, viewerUserID, requestID)
}

func (s *Service) AccessLogCountForSession(ctx context.Context, sessionID string) int {
	return s.contactService.AccessLogCount(ctx, sessionID)
}

func (s *Service) MyProfile(ctx context.Context, user User) (UserProfile, *domain.AppError) {
	profile, appErr := s.profileService.MyProfile(ctx, user)
	if appErr != nil {
		return UserProfile{}, appErr
	}
	passwordConfigured, appErr := s.authService.PasswordConfigured(ctx, user.ID)
	if appErr != nil {
		return UserProfile{}, appErr
	}
	profile.PasswordConfigured = passwordConfigured
	return profile, nil
}

func (s *Service) UpdateMyProfile(ctx context.Context, user User, input UpdateUserProfileInput) (UserProfile, *domain.AppError) {
	return s.profileService.UpdateMyProfile(ctx, user, input)
}

func (s *Service) StartEmailVerification(ctx context.Context, user User, input EmailVerificationStartInput) (EmailVerificationChallenge, *domain.AppError) {
	return s.profileService.StartEmailVerification(ctx, user, input)
}

func (s *Service) ConfirmEmailVerification(ctx context.Context, user User, input EmailVerificationConfirmInput) (UserProfile, *domain.AppError) {
	profile, appErr := s.profileService.ConfirmEmailVerification(ctx, user, input)
	if appErr != nil {
		return UserProfile{}, appErr
	}
	passwordConfigured, appErr := s.authService.PasswordConfigured(ctx, user.ID)
	if appErr != nil {
		return UserProfile{}, appErr
	}
	profile.PasswordConfigured = passwordConfigured
	return profile, nil
}

func (s *Service) PublicUserProfile(ctx context.Context, username string) (PublicUserProfile, *domain.AppError) {
	publicProfile, appErr := s.profileService.PublicUserProfile(ctx, username)
	if appErr != nil {
		return PublicUserProfile{}, appErr
	}
	factsByUser, appErr := s.reputationService.AggregateFacts(ctx, []string{publicProfile.ID})
	if appErr != nil {
		return PublicUserProfile{}, appErr
	}
	if facts, ok := factsByUser[publicProfile.ID]; ok {
		applyPublicUserReputationFacts(&publicProfile, facts)
	}
	stats, appErr := s.reportService.PublicUserDisputeStats(ctx, username)
	if appErr != nil {
		return PublicUserProfile{}, appErr
	}
	resolved := stats.ResolvedLast90Days
	if !publicProfile.Privacy.ShowResolvedDisputeSummary {
		publicProfile.Stats.ResolvedDisputeCountLast90Days = nil
	} else {
		publicProfile.Stats.ResolvedDisputeCountLast90Days = &resolved
	}
	return publicProfile, nil
}

func (s *Service) PublicUserProfileBundle(ctx context.Context, username string) (profile.PublicUserProfileBundle, *domain.AppError) {
	publicProfile, appErr := s.PublicUserProfile(ctx, username)
	if appErr != nil {
		return profile.PublicUserProfileBundle{}, appErr
	}
	user := User{ID: publicProfile.ID}

	listingPage, appErr := s.carpoolService.MyListings(ctx, user, carpool.OwnerListingViewAll, domain.PageRequest{Limit: 100})
	if appErr != nil {
		return profile.PublicUserProfileBundle{}, appErr
	}
	services, appErr := s.apiMarket.OwnerServices(ctx, user, apimarket.OwnerServiceFilter{SalesView: apimarket.OwnerSalesViewAll}, domain.PageRequest{Limit: 100})
	if appErr != nil {
		return profile.PublicUserProfileBundle{}, appErr
	}
	buyerMemberships, appErr := s.carpoolService.MyMemberships(ctx, user)
	if appErr != nil {
		return profile.PublicUserProfileBundle{}, appErr
	}
	ownerMemberships, appErr := s.carpoolService.OwnerMemberships(ctx, user)
	if appErr != nil {
		return profile.PublicUserProfileBundle{}, appErr
	}
	buyerOrders, appErr := s.apiOrder.BuyerOrders(ctx, user)
	if appErr != nil {
		return profile.PublicUserProfileBundle{}, appErr
	}
	sellerOrders, appErr := s.apiOrder.SellerOrders(ctx, user)
	if appErr != nil {
		return profile.PublicUserProfileBundle{}, appErr
	}
	reviews, appErr := s.reviewService.PublicForUser(ctx, username)
	if appErr != nil {
		return profile.PublicUserProfileBundle{}, appErr
	}
	disputes, appErr := s.reportService.PublicUserDisputes(ctx, username)
	if appErr != nil {
		return profile.PublicUserProfileBundle{}, appErr
	}

	reputations := []reputation.ReputationSnapshot{}
	if s.reputationService.EngineAvailable() {
		reputations, appErr = s.reputationService.GetUserScope(ctx, publicProfile.ID, reputation.ScopeOverall)
		if appErr != nil {
			return profile.PublicUserProfileBundle{}, appErr
		}
	}
	return profile.PublicUserProfileBundle{
		Profile:     publicProfile,
		Reputations: reputations,
		Carpools:    publicProfileCarpools(listingPage.Items),
		Services:    publicProfileAPIServices(services.Items),
		Completions: publicProfileCompletions(publicProfile, buyerMemberships, ownerMemberships, buyerOrders, sellerOrders),
		Reviews:     publicProfileReviews(reviews),
		Disputes:    publicProfileDisputes(disputes),
	}, nil
}

func publicProfileCarpools(listings []carpool.Listing) []profile.PublicProfileCarpool {
	items := make([]profile.PublicProfileCarpool, 0, len(listings))
	for _, listing := range listings {
		if listing.Status != carpool.ListingStatusActive {
			continue
		}
		items = append(items, profile.PublicProfileCarpool{
			ID:              listing.ID,
			Title:           listing.Title,
			Summary:         listing.Summary,
			RegionName:      listing.RegionName,
			PriceMonthlyCNY: listing.PriceMonthlyCNY,
			AvailableSeats:  listing.AvailableSeats,
			UpdatedAt:       listing.UpdatedAt,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return limitSlice(items, 6)
}

func publicProfileAPIServices(services []apimarket.Service) []profile.PublicProfileAPIService {
	items := make([]profile.PublicProfileAPIService, 0, len(services))
	for _, service := range services {
		if !apimarket.IsPublicService(service) || service.MerchantIdentityMode != "public_profile" {
			continue
		}
		items = append(items, profile.PublicProfileAPIService{
			ID:                    service.ID,
			Title:                 service.Title,
			ShortDescription:      service.ShortDescription,
			BillingMode:           service.BillingMode,
			AvailableUSDAllowance: service.AvailableUSDAllowance,
			UsageVisibility:       service.UsageVisibility,
			RefundCommitment:      service.MerchantRefundCommitment,
			UpdatedAt:             service.UpdatedAt,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return limitSlice(items, 6)
}

func publicProfileCompletions(
	publicProfile profile.PublicUserProfile,
	buyerMemberships []carpool.Membership,
	ownerMemberships []carpool.Membership,
	buyerOrders []apiorder.Order,
	sellerOrders []apiorder.Order,
) []profile.PublicProfileCompletion {
	items := []profile.PublicProfileCompletion{}
	seen := make(map[string]struct{})
	if publicProfile.Privacy.ShowCompletedCarpoolCount {
		for _, membership := range append(buyerMemberships, ownerMemberships...) {
			if membership.Status != carpool.MembershipStatusCompleted || membership.CompletedAt == nil {
				continue
			}
			key := "carpool:" + membership.ID
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			role := "seller"
			if membership.BuyerUserID == publicProfile.ID {
				role = "buyer"
			}
			items = append(items, profile.PublicProfileCompletion{ID: membership.ID, Kind: "carpool", Title: "拼车成员关系", Role: role, CompletedAt: *membership.CompletedAt})
		}
	}
	if publicProfile.Privacy.ShowCompletedAPIIntentCount {
		for _, order := range append(buyerOrders, sellerOrders...) {
			if order.Status != apiorder.StatusCompleted || order.CompletedAt == nil {
				continue
			}
			key := "api_order:" + order.ID
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			role := "seller"
			if order.BuyerUserID == publicProfile.ID {
				role = "buyer"
			}
			items = append(items, profile.PublicProfileCompletion{ID: order.ID, Kind: "api_order", Title: order.ServiceTitleSnapshot, Role: role, CompletedAt: *order.CompletedAt})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CompletedAt.Equal(items[j].CompletedAt) {
			if items[i].Kind == items[j].Kind {
				return items[i].ID > items[j].ID
			}
			return items[i].Kind > items[j].Kind
		}
		return items[i].CompletedAt.After(items[j].CompletedAt)
	})
	return limitSlice(items, 10)
}

func publicProfileReviews(items []review.PublicReview) []review.PublicReview {
	result := append([]review.PublicReview(nil), items...)
	sort.Slice(result, func(i, j int) bool {
		if result[i].Date.Equal(result[j].Date) {
			return result[i].ID > result[j].ID
		}
		return result[i].Date.After(result[j].Date)
	})
	return limitSlice(result, 10)
}

func publicProfileDisputes(items []report.PublicDispute) []report.PublicDispute {
	result := append([]report.PublicDispute(nil), items...)
	sort.Slice(result, func(i, j int) bool {
		if result[i].HandledAt.Equal(result[j].HandledAt) {
			return result[i].ID > result[j].ID
		}
		return result[i].HandledAt.After(result[j].HandledAt)
	})
	return limitSlice(result, 10)
}

func limitSlice[T any](items []T, limit int) []T {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

func (s *Service) MyMerchantProfile(ctx context.Context, user User) (MerchantProfile, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIServicePublish); appErr != nil {
		return MerchantProfile{}, appErr
	}
	return s.profileService.MyMerchantProfile(ctx, user)
}

func (s *Service) UpsertMyMerchantProfile(ctx context.Context, user User, input UpsertMerchantProfileInput) (MerchantProfile, *domain.AppError) {
	if appErr := authmodule.RequireCapability(user, authmodule.CapabilityAPIServicePublish); appErr != nil {
		return MerchantProfile{}, appErr
	}
	return s.profileService.UpsertMyMerchantProfile(ctx, user, input)
}

func (s *Service) PublicMerchantProfile(ctx context.Context, slug string) (PublicMerchantProfile, *domain.AppError) {
	publicProfile, appErr := s.profileService.PublicMerchantProfile(ctx, slug)
	if appErr != nil {
		return PublicMerchantProfile{}, appErr
	}
	factsByUser, appErr := s.reputationService.AggregateFacts(ctx, []string{publicProfile.OwnerUserID})
	if appErr != nil {
		return PublicMerchantProfile{}, appErr
	}
	if facts, ok := factsByUser[publicProfile.OwnerUserID]; ok {
		sellerAPI := facts.Seller.API
		publicProfile.CompletedLast90Days = reputationCount(sellerAPI.CompletedCountLast90Days)
		publicProfile.MerchantResponsibleCancellations = reputationCount(sellerAPI.RoleResponsibilityCancellationCount)
		publicProfile.UnresolvedDisputes = reputationCount(sellerAPI.UnresolvedDisputeCount)
	}
	return publicProfile, nil
}

func applyPublicUserReputationFacts(publicProfile *PublicUserProfile, facts reputation.RawFacts) {
	if publicProfile.Privacy.ShowCompletedCarpoolCount {
		publicProfile.Stats.CompletedCarpools = reputationCount(facts.Buyer.Carpool.CompletedCount + facts.Seller.Carpool.CompletedCount)
		publicProfile.Stats.CompletedCarpoolsLast90Days = reputationCount(facts.Buyer.Carpool.CompletedCountLast90Days + facts.Seller.Carpool.CompletedCountLast90Days)
	}
	if publicProfile.Privacy.ShowCompletedAPIIntentCount {
		publicProfile.Stats.CompletedAPIOrders = reputationCount(facts.Buyer.API.CompletedCount + facts.Seller.API.CompletedCount)
		publicProfile.Stats.CompletedAPIOrdersLast90Days = reputationCount(facts.Buyer.API.CompletedCountLast90Days + facts.Seller.API.CompletedCountLast90Days)
	}
	publicProfile.Stats.BuyerResponsibilityCancellationCount = reputationCount(facts.Buyer.Overall.RoleResponsibilityCancellationCount)
	publicProfile.Stats.SellerResponsibilityCancellationCount = reputationCount(facts.Seller.Overall.RoleResponsibilityCancellationCount)
	publicProfile.Stats.UnknownResponsibilityCancellationCount = reputationCount(
		facts.Buyer.Overall.UnknownResponsibilityCancellationCount + facts.Seller.Overall.UnknownResponsibilityCancellationCount,
	)
	publicProfile.Stats.UnresolvedDisputeCount = reputationCount(
		facts.Buyer.Overall.UnresolvedDisputeCount + facts.Seller.Overall.UnresolvedDisputeCount,
	)
}

func reputationCount(value int) *int {
	result := value
	return &result
}

func (s *Service) UserAnnouncements(ctx context.Context, user User) ([]Announcement, *domain.AppError) {
	return s.announcement.UserAnnouncements(ctx, user)
}

func (s *Service) ActiveAnnouncements(ctx context.Context, user User, channel string) ([]Announcement, *domain.AppError) {
	return s.announcement.ActiveAnnouncements(ctx, user, channel)
}

func (s *Service) HomeAnnouncement(ctx context.Context, user User) (*Announcement, *domain.AppError) {
	return s.announcement.HomeAnnouncement(ctx, user)
}

func (s *Service) UserAnnouncementBySlug(ctx context.Context, user User, slug string) (Announcement, *domain.AppError) {
	return s.announcement.UserAnnouncementBySlug(ctx, user, slug)
}

func (s *Service) AnnouncementUnreadCount(ctx context.Context, user User, importantOnly bool) (int, *domain.AppError) {
	return s.announcement.AnnouncementUnreadCount(ctx, user, importantOnly)
}

func (s *Service) MarkAnnouncementSeen(ctx context.Context, user User, id string) (AnnouncementReceipt, *domain.AppError) {
	return s.announcement.MarkSeen(ctx, user, id)
}

func (s *Service) MarkAnnouncementRead(ctx context.Context, user User, id string) (AnnouncementReceipt, *domain.AppError) {
	return s.announcement.MarkRead(ctx, user, id)
}

func (s *Service) DismissAnnouncement(ctx context.Context, user User, id string) (AnnouncementReceipt, *domain.AppError) {
	return s.announcement.Dismiss(ctx, user, id)
}

func (s *Service) AdminAnnouncements(ctx context.Context, user User) ([]Announcement, *domain.AppError) {
	return s.announcement.AdminAnnouncements(ctx, user)
}

func (s *Service) AdminAnnouncement(ctx context.Context, user User, id string) (Announcement, *domain.AppError) {
	return s.announcement.AdminAnnouncement(ctx, user, id)
}

func (s *Service) CreateAnnouncement(ctx context.Context, user User, input AnnouncementFormInput) (Announcement, *domain.AppError) {
	return s.announcement.CreateAnnouncement(ctx, user, input)
}

func (s *Service) UpdateAnnouncement(ctx context.Context, user User, id string, input AnnouncementFormInput) (Announcement, *domain.AppError) {
	return s.announcement.UpdateAnnouncement(ctx, user, id, input)
}

func (s *Service) PublishAnnouncement(ctx context.Context, user User, id string) (Announcement, *domain.AppError) {
	return s.announcement.PublishAnnouncement(ctx, user, id)
}

func (s *Service) OfflineAnnouncement(ctx context.Context, user User, id, reason string) (Announcement, *domain.AppError) {
	return s.announcement.OfflineAnnouncement(ctx, user, id, reason)
}

func (s *Service) DuplicateAnnouncement(ctx context.Context, user User, id string) (Announcement, *domain.AppError) {
	return s.announcement.DuplicateAnnouncement(ctx, user, id)
}

func (s *Service) AnnouncementAuditLogs(ctx context.Context, user User) ([]AnnouncementAuditLog, *domain.AppError) {
	return s.announcement.AnnouncementAuditLogs(ctx, user)
}

func (s *Service) MyNotifications(ctx context.Context, user User, page domain.PageRequest) (domain.Page[notification.Notification], *domain.AppError) {
	return s.notification.List(ctx, user.ID, page)
}

func (s *Service) MyNotificationUnreadCount(ctx context.Context, user User) (int, *domain.AppError) {
	return s.notification.UnreadCount(ctx, user.ID)
}

func (s *Service) MarkNotificationRead(ctx context.Context, user User, id string) (notification.Notification, *domain.AppError) {
	return s.notification.MarkRead(ctx, user.ID, id)
}

func (s *Service) MarkAllNotificationsRead(ctx context.Context, user User) (notification.ReadAllResult, *domain.AppError) {
	return s.notification.MarkAllRead(ctx, user.ID)
}

func (s *Service) SearchMarket(ctx context.Context, keyword string) ([]search.Result, *domain.AppError) {
	return s.searchService.Search(ctx, keyword)
}

func (s *Service) CreateFeedbackTicketWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input CreateFeedbackInput, buildCompletion FeedbackCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.feedbackService.CreateWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) MyFeedbackTickets(ctx context.Context, user User, page domain.PageRequest) (domain.Page[FeedbackTicket], *domain.AppError) {
	return s.feedbackService.MyTickets(ctx, user, page)
}

func (s *Service) MyFeedbackTicket(ctx context.Context, user User, id string) (FeedbackTicket, *domain.AppError) {
	return s.feedbackService.MyTicket(ctx, user, id)
}

func (s *Service) AddFeedbackSupplementWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input FeedbackSupplementInput, buildCompletion FeedbackCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.feedbackService.AddSupplementWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) MarkFeedbackRead(ctx context.Context, user User, id string) (FeedbackTicket, *domain.AppError) {
	return s.feedbackService.MarkRead(ctx, user, id)
}

func (s *Service) MyFeedbackUnreadCount(ctx context.Context, user User) (int, *domain.AppError) {
	return s.feedbackService.UnreadCount(ctx, user)
}

func (s *Service) AdminFeedbackTickets(ctx context.Context, user User) ([]FeedbackTicket, *domain.AppError) {
	return s.feedbackService.AdminTickets(ctx, user)
}

func (s *Service) AdminFeedbackTicket(ctx context.Context, user User, id string) (FeedbackTicket, *domain.AppError) {
	return s.feedbackService.AdminTicket(ctx, user, id)
}

func (s *Service) AdminHandleFeedbackTicketWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input FeedbackAdminHandleInput, buildCompletion FeedbackCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.feedbackService.AdminHandleWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) FavoriteTargetSummary(ctx context.Context, targetType, targetID string) (favorite.TargetSummary, *domain.AppError) {
	switch targetType {
	case favorite.TargetCarpool:
		listing, appErr := s.PublicCarpoolListing(ctx, targetID)
		if appErr != nil {
			return favorite.TargetSummary{}, appErr
		}
		return favorite.TargetSummary{
			Title:    listing.Title,
			Subtitle: "车源 · 月费 ¥" + listing.PriceMonthlyCNY,
			Status:   listing.Status,
			To:       "/carpools/" + listing.ID,
		}, nil
	case favorite.TargetAPIService:
		service, appErr := s.PublicAPIService(ctx, targetID)
		if appErr != nil {
			return favorite.TargetSummary{}, appErr
		}
		return favorite.TargetSummary{
			Title:    service.Title,
			Subtitle: "API 服务 · " + service.MerchantDisplayName,
			Status:   service.PublicationStatus,
			To:       "/api-market/" + service.ID,
		}, nil
	default:
		return favorite.TargetSummary{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Favorite validation failed", "收藏类型不支持。", "targetType", "invalid", "收藏类型不支持。")
	}
}

func (s *Service) MyFavorites(ctx context.Context, user User) ([]favorite.ListItem, *domain.AppError) {
	return s.favoriteService.List(ctx, user.ID)
}

func (s *Service) IsFavorite(ctx context.Context, user User, targetType, targetID string) (bool, *domain.AppError) {
	return s.favoriteService.IsFavorite(ctx, user.ID, targetType, targetID)
}

func (s *Service) CreateFavoriteWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash, targetType, targetID string, buildCompletion favorite.CompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.favoriteService.CreateWithIdempotency(ctx, userID, routeKey, key, requestHash, targetType, targetID, buildCompletion)
}

func (s *Service) DeleteFavorite(ctx context.Context, user User, targetType, targetID string) (favorite.MutationResult, *domain.AppError) {
	return s.favoriteService.Delete(ctx, user.ID, targetType, targetID)
}

func (s *Service) ListMyReviewCenterRows(ctx context.Context, user User) ([]review.ReviewCenterRow, *domain.AppError) {
	return s.reviewService.ListMine(ctx, user.ID)
}

func (s *Service) SubmitTransactionReviewWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input review.SubmitReviewInput, buildCompletion review.CompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.reviewService.SubmitWithIdempotency(ctx, userID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) SubmitCarpoolMembershipReviewWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input review.SubmitReviewInput, buildCompletion review.CompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	input.TransactionType = review.TransactionCarpoolMembership
	input.Operation = review.OperationLegacyUpsert
	return s.reviewService.SubmitWithIdempotency(ctx, userID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) AdminRemoveTransactionReviewWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input review.RemoveReviewInput, buildCompletion review.CompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.reviewService.RemoveWithIdempotency(ctx, user.ID, user.IsAdmin, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) PublicUserReviews(ctx context.Context, username string) ([]review.PublicReview, *domain.AppError) {
	return s.reviewService.PublicForUser(ctx, username)
}

func (s *Service) ReviewTransactionsByUserID(ctx context.Context, userID string) ([]review.Transaction, *domain.AppError) {
	user := User{ID: userID}
	buyerMemberships, appErr := s.carpoolService.MyMemberships(ctx, user)
	if appErr != nil {
		return nil, appErr
	}
	sellerMemberships, appErr := s.carpoolService.OwnerMemberships(ctx, user)
	if appErr != nil {
		return nil, appErr
	}
	buyerOrders, appErr := s.apiOrder.BuyerOrders(ctx, user)
	if appErr != nil {
		return nil, appErr
	}
	sellerOrders, appErr := s.apiOrder.SellerOrders(ctx, user)
	if appErr != nil {
		return nil, appErr
	}

	items := make([]review.Transaction, 0, len(buyerMemberships)+len(sellerMemberships)+len(buyerOrders)+len(sellerOrders))
	seen := make(map[string]struct{})
	for _, membership := range append(buyerMemberships, sellerMemberships...) {
		if membership.Status != carpool.MembershipStatusCompleted {
			continue
		}
		transaction := reviewTransactionFromCarpoolMembership(membership)
		key := transaction.Type + ":" + transaction.ID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		items = append(items, transaction)
	}
	for _, order := range append(buyerOrders, sellerOrders...) {
		if order.Status != apiorder.StatusCompleted || order.CompletedAt == nil {
			continue
		}
		transaction := reviewTransactionFromAPIOrder(order)
		key := transaction.Type + ":" + transaction.ID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		items = append(items, transaction)
	}
	return items, nil
}

func (s *Service) ResolveReviewTransaction(ctx context.Context, transactionType, transactionID, userID string) (review.Transaction, *domain.AppError) {
	items, appErr := s.ReviewTransactionsByUserID(ctx, userID)
	if appErr != nil {
		return review.Transaction{}, appErr
	}
	for _, item := range items {
		if item.Type == transactionType && item.ID == transactionID {
			return item, nil
		}
	}
	return review.Transaction{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Transaction not found", "可评价交易不存在。")
}

func reviewTransactionFromCarpoolMembership(membership carpool.Membership) review.Transaction {
	completedAt := membership.UpdatedAt
	if membership.EndedAt != nil {
		completedAt = *membership.EndedAt
	} else if membership.CompletedAt != nil {
		completedAt = *membership.CompletedAt
	}
	return review.Transaction{
		Type:              review.TransactionCarpoolMembership,
		ID:                membership.ID,
		Target:            "拼车成员关系 " + membership.ID,
		BuyerUserID:       membership.BuyerUserID,
		BuyerUsername:     membership.BuyerUserID,
		BuyerDisplayName:  membership.BuyerUserID,
		SellerUserID:      membership.OwnerUserID,
		SellerUsername:    membership.OwnerUserID,
		SellerDisplayName: membership.OwnerUserID,
		CompletedAt:       completedAt,
		ReviewDeadlineAt:  completedAt.Add(review.ReviewWindow),
	}
}

func reviewTransactionFromAPIOrder(order apiorder.Order) review.Transaction {
	completedAt := order.UpdatedAt
	if order.CompletedAt != nil {
		completedAt = *order.CompletedAt
	}
	return review.Transaction{
		Type:              review.TransactionAPIOrder,
		ID:                order.ID,
		Target:            order.ServiceTitleSnapshot,
		BuyerUserID:       order.BuyerUserID,
		BuyerUsername:     order.BuyerUserID,
		BuyerDisplayName:  order.BuyerUserID,
		SellerUserID:      order.SellerUserID,
		SellerUsername:    order.SellerUserID,
		SellerDisplayName: order.SellerUserID,
		CompletedAt:       completedAt,
		ReviewDeadlineAt:  completedAt.Add(review.ReviewWindow),
	}
}

func (s *Service) CreateReportWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input report.CreateReportInput, buildCompletion report.ReportCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.reportService.CreateReportWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) MyReports(ctx context.Context, user User) ([]report.Report, *domain.AppError) {
	return s.reportService.MyReports(ctx, user)
}

func (s *Service) AdminReports(ctx context.Context, user User, page domain.PageRequest) (domain.Page[report.Report], *domain.AppError) {
	return s.reportService.AdminReports(ctx, user, page)
}

func (s *Service) AdminReport(ctx context.Context, user User, id string) (report.Report, *domain.AppError) {
	return s.reportService.AdminReport(ctx, user, id)
}

func (s *Service) AdminReportActionWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input report.AdminActionInput, buildCompletion report.AdminCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.reportService.AdminReportActionWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) SubmitInfoSupplementWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input report.SupplementInput, buildCompletion report.SupplementCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.reportService.SubmitInfoSupplementWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) MyDisputes(ctx context.Context, user User) ([]report.DisputeCase, *domain.AppError) {
	return s.reportService.MyDisputes(ctx, user)
}

func (s *Service) DisputesForActor(ctx context.Context, actor authmodule.BusinessActor) ([]report.DisputeCase, *domain.AppError) {
	return s.reportService.DisputesForActor(ctx, actor)
}

func (s *Service) MyDispute(ctx context.Context, user User, id string) (report.DisputeCase, *domain.AppError) {
	return s.reportService.MyDispute(ctx, user, id)
}

func (s *Service) DisputeForActor(ctx context.Context, actor authmodule.BusinessActor, id string) (report.DisputeCase, *domain.AppError) {
	return s.reportService.DisputeForActor(ctx, actor, id)
}

func (s *Service) DisputeParticipantActionWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input report.DisputeParticipantActionInput, buildCompletion report.DisputeParticipantCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.reportService.DisputeParticipantActionWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) DisputeParticipantActionForActorWithIdempotency(ctx context.Context, actor authmodule.BusinessActor, routeKey, key, requestHash string, input report.DisputeParticipantActionInput, buildCompletion report.DisputeParticipantCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.reportService.DisputeParticipantActionForActorWithIdempotency(ctx, actor, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) SubmitInfoSupplementForActorWithIdempotency(ctx context.Context, actor authmodule.BusinessActor, routeKey, key, requestHash string, input report.SupplementInput, buildCompletion report.SupplementCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.reportService.SubmitInfoSupplementForActorWithIdempotency(ctx, actor, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) AdminDisputes(ctx context.Context, user User) ([]report.DisputeCase, *domain.AppError) {
	return s.reportService.AdminDisputes(ctx, user)
}

func (s *Service) AdminDispute(ctx context.Context, user User, id string) (report.DisputeCase, *domain.AppError) {
	return s.reportService.AdminDispute(ctx, user, id)
}

func (s *Service) AdminDisputeActionWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input report.AdminActionInput, buildCompletion report.AdminCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	var overdueRemedy *report.DisputeRemedy
	var overdueSellerUserID string
	if input.Action == "mark_overdue" && s.reputationService.TracksAPIOrderRemedyFactsInMemory() {
		dispute, appErr := s.reportService.AdminDispute(ctx, user, input.ID)
		if appErr != nil {
			return IdempotencyCompletion{}, appErr
		}
		if dispute.TargetType == report.TargetAPIOrder && len(dispute.Remedies) > 0 {
			remedy := dispute.Remedies[0]
			overdueRemedy = &remedy
			order, orderErr := s.apiOrder.AdminOrder(ctx, user, dispute.TargetID)
			if orderErr != nil {
				return IdempotencyCompletion{}, orderErr
			}
			overdueSellerUserID = order.SellerUserID
		}
	}
	completion, appErr := s.reportService.AdminDisputeActionWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
	if appErr == nil && overdueRemedy != nil {
		overdueAt := s.now()
		if updated, readErr := s.reportService.AdminDispute(ctx, user, input.ID); readErr == nil {
			for _, remedy := range updated.Remedies {
				if remedy.ID == overdueRemedy.ID && remedy.OverdueAt != nil {
					overdueAt = *remedy.OverdueAt
					break
				}
			}
		}
		s.reputationService.RecordAPIOrderRemedyOverdueFact(
			overdueRemedy.DisputeCaseID,
			overdueRemedy.ID,
			overdueRemedy.ResponsibleUserID,
			overdueSellerUserID,
			overdueAt,
		)
	}
	return completion, appErr
}

func (s *Service) CreateAppealWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input report.CreateAppealInput, buildCompletion report.AppealCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.reportService.CreateAppealWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) CreateAccountGovernanceAppealWithIdempotency(ctx context.Context, appellantUserID, routeKey, key, requestHash string, input report.CreateAccountGovernanceAppealInput, buildCompletion report.AppealCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.reportService.CreateAccountGovernanceAppealWithIdempotency(ctx, appellantUserID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) MyAppeals(ctx context.Context, user User) ([]report.Appeal, *domain.AppError) {
	return s.reportService.MyAppeals(ctx, user)
}

func (s *Service) AdminAppeals(ctx context.Context, user User) ([]report.Appeal, *domain.AppError) {
	return s.reportService.AdminAppeals(ctx, user)
}

func (s *Service) AdminAppeal(ctx context.Context, user User, id string) (report.Appeal, *domain.AppError) {
	return s.reportService.AdminAppeal(ctx, user, id)
}

func (s *Service) AdminAppealActionWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input report.AdminActionInput, buildCompletion report.AdminCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.reportService.AdminAppealActionWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) PublicUserDisputes(ctx context.Context, username string) ([]report.PublicDispute, *domain.AppError) {
	return s.reportService.PublicUserDisputes(ctx, username)
}

func (s *Service) ReputationRules() reputation.RuleSet {
	return s.reputationService.Rules()
}

func (s *Service) ReputationAvailable() bool {
	return s.reputationService.EngineAvailable()
}

func (s *Service) PublicUserReputation(ctx context.Context, username, scope string) ([]reputation.ReputationSnapshot, *domain.AppError) {
	publicProfile, appErr := s.profileService.PublicUserProfile(ctx, username)
	if appErr != nil {
		return nil, appErr
	}
	return s.reputationService.GetUserScope(ctx, publicProfile.ID, scope)
}

func (s *Service) MyReputation(ctx context.Context, user User) ([]reputation.ReputationSnapshot, *domain.AppError) {
	if _, appErr := s.profileService.MyProfile(ctx, user); appErr != nil {
		return nil, appErr
	}
	return s.reputationService.GetUserReputation(ctx, user.ID)
}

func (s *Service) AdminUserReputation(ctx context.Context, user User, userID string, historyLimit int) (reputation.AdminReputationAudit, *domain.AppError) {
	if !user.IsAdmin {
		return reputation.AdminReputationAudit{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	if _, appErr := s.profileService.MyProfile(ctx, User{ID: userID}); appErr != nil {
		return reputation.AdminReputationAudit{}, appErr
	}
	items, appErr := s.reputationService.GetUserReputation(ctx, userID)
	if appErr != nil {
		return reputation.AdminReputationAudit{}, appErr
	}
	history, appErr := s.reputationService.History(ctx, userID, historyLimit)
	if appErr != nil {
		return reputation.AdminReputationAudit{}, appErr
	}
	evidence, appErr := s.reputationService.AdminEvidence(ctx, userID)
	if appErr != nil {
		return reputation.AdminReputationAudit{}, appErr
	}
	return reputation.AdminReputationAudit{
		UserID:                    userID,
		RuleVersion:               reputation.RuleVersion,
		Items:                     items,
		History:                   history,
		Restrictions:              evidence.Restrictions,
		Outcomes:                  evidence.Outcomes,
		Appeals:                   evidence.Appeals,
		SourceAuthorVerifications: evidence.SourceAuthorVerifications,
	}, nil
}

func (s *Service) AdminRecalculateUserReputation(ctx context.Context, user User, userID string) (reputation.RecalculationResult, *domain.AppError) {
	if !user.IsAdmin {
		return reputation.RecalculationResult{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	if _, appErr := s.profileService.MyProfile(ctx, User{ID: userID}); appErr != nil {
		return reputation.RecalculationResult{}, appErr
	}
	return s.reputationService.RecalculateUser(ctx, userID)
}

func (s *Service) AdminRecalculateAllReputation(ctx context.Context, user User) (reputation.RecalculationResult, *domain.AppError) {
	if !user.IsAdmin {
		return reputation.RecalculationResult{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	return s.reputationService.RecalculateAll(ctx)
}

func (s *Service) AdminSourceAuthorVerification(
	ctx context.Context,
	user User,
	resourceType string,
	resourceID string,
) (reputation.SourceAuthorVerificationAudit, *domain.AppError) {
	return s.reputationService.GetSourceAuthorVerificationAudit(
		ctx,
		reputation.AdminActor{UserID: user.ID, IsAdmin: user.IsAdmin},
		resourceType,
		resourceID,
	)
}

func (s *Service) AdminUpdateSourceAuthorVerification(
	ctx context.Context,
	user User,
	input reputation.UpdateSourceAuthorVerificationInput,
) (reputation.SourceAuthorVerificationAudit, *domain.AppError) {
	return s.reputationService.UpdateSourceAuthorVerification(
		ctx,
		reputation.AdminActor{UserID: user.ID, IsAdmin: user.IsAdmin},
		input,
	)
}

func (s *Service) AdminCreateDisputeOutcomeWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input reputation.CreateOutcomeInput, buildCompletion reputation.GovernanceCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	dispute, appErr := s.reportService.AdminDispute(ctx, user, input.DisputeCaseID)
	if appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	input.APIOrderDispute = dispute.TargetType == report.TargetAPIOrder
	input.RemedyOverdueFact = input.APIOrderDispute && len(dispute.Remedies) > 0 && dispute.Remedies[0].Status == report.RemedyStatusOverdue
	if input.RemedyOverdueFact {
		remedy := dispute.Remedies[0]
		order, orderErr := s.apiOrder.AdminOrder(ctx, user, dispute.TargetID)
		if orderErr != nil {
			return IdempotencyCompletion{}, orderErr
		}
		input.RemedyID = remedy.ID
		input.RemedyResponsible = remedy.ResponsibleUserID
		input.RemedyOverdueAt = remedy.OverdueAt
		input.APIOrderSellerID = order.SellerUserID
	}
	return s.reputationService.CreateDisputeOutcomeWithIdempotency(ctx, reputation.AdminActor{UserID: user.ID, IsAdmin: user.IsAdmin}, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) AdminAPIOrderSanctionRecommendation(ctx context.Context, user User, disputeCaseID string) (reputation.APIOrderSanctionRecommendation, *domain.AppError) {
	return s.reputationService.APIOrderSanctionRecommendation(ctx, reputation.AdminActor{UserID: user.ID, IsAdmin: user.IsAdmin}, disputeCaseID)
}

func (s *Service) AdminApplyAPIOrderSanctionWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input reputation.ApplyAPIOrderSanctionInput, buildCompletion reputation.GovernanceCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.reputationService.ApplyAPIOrderSanctionWithIdempotency(ctx, reputation.AdminActor{UserID: user.ID, IsAdmin: user.IsAdmin}, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) MyActiveReputationRestrictions(ctx context.Context, user User) ([]reputation.UserRestriction, *domain.AppError) {
	if _, appErr := s.profileService.MyProfile(ctx, user); appErr != nil {
		return nil, appErr
	}
	return s.reputationService.ActiveRestrictions(ctx, user.ID)
}

func (s *Service) AdminCreateUserRestrictionWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input reputation.CreateRestrictionInput, buildCompletion reputation.GovernanceCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.reputationService.CreateUserRestrictionWithIdempotency(ctx, reputation.AdminActor{UserID: user.ID, IsAdmin: user.IsAdmin}, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) AdminRevokeUserRestrictionWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input reputation.RevokeRestrictionInput, buildCompletion reputation.GovernanceCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.reputationService.RevokeUserRestrictionWithIdempotency(ctx, reputation.AdminActor{UserID: user.ID, IsAdmin: user.IsAdmin}, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) CheckReputationActionAllowed(ctx context.Context, userID, role, action string) *domain.AppError {
	return s.reputationService.CheckActionAllowed(ctx, userID, role, action)
}
