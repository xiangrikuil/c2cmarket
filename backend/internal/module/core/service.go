package core

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/announcement"
	"c2c-market/backend/internal/module/apiintent"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/apiorder"
	authmodule "c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/carpool"
	"c2c-market/backend/internal/module/catalog"
	contactmodule "c2c-market/backend/internal/module/contact"
	"c2c-market/backend/internal/module/demand"
	"c2c-market/backend/internal/module/favorite"
	"c2c-market/backend/internal/module/feedback"
	idempotencymodule "c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/modelaudit"
	"c2c-market/backend/internal/module/notification"
	"c2c-market/backend/internal/module/officialprice"
	"c2c-market/backend/internal/module/profile"
	"c2c-market/backend/internal/module/report"
	"c2c-market/backend/internal/module/review"
	"c2c-market/backend/internal/module/search"
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

	DemandStatusPendingReview    = demand.StatusPendingReview
	DemandStatusActive           = demand.StatusActive
	DemandStatusChangesRequested = demand.StatusChangesRequested
	DemandStatusRejected         = demand.StatusRejected
	DemandStatusClosed           = demand.StatusClosed
	DemandStatusTakenDown        = demand.StatusTakenDown
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
	announcement       *announcement.Service
	notification       *notification.Service
	contactService     *contactmodule.Service
	profileService     *profile.Service
	emailSender        profile.EmailSender
	demandService      *demand.Service
	feedbackService    *feedback.Service
	favoriteService    *favorite.Service
	reviewService      *review.Service
	searchService      *search.Service
	reportService      *report.Service
	modelAudit         *modelaudit.Service
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

func NewServiceWithClock(now func() time.Time) *Service {
	return newService(now, Repositories{})
}

func newService(now func() time.Time, repositories Repositories) *Service {
	return newServiceWithEmailSender(now, repositories, profile.NewDevelopmentEmailSender())
}

func newServiceWithEmailSender(now func() time.Time, repositories Repositories, emailSender profile.EmailSender) *Service {
	s := &Service{
		authService:        authmodule.NewServiceWithRegistrationEmailSender(repositories.Auth, now, emailSender),
		idempotencyService: idempotencymodule.NewService(repositories.Idempotency, now),
		catalogService:     catalog.NewService(repositories.Catalog, now),
		announcement:       announcement.NewService(repositories.Announcement, now),
		notification:       notification.NewService(repositories.Notification, now),
		contactService:     contactmodule.NewService(repositories.Contact, now),
		profileService:     profile.NewServiceWithEmailSender(repositories.Profile, now, emailSender),
		emailSender:        emailSender,
		now:                now,
	}
	s.officialPrice = officialprice.NewService(repositories.OfficialPrice, s.idempotencyService, now)
	s.carpoolService = carpool.NewService(repositories.Carpool, s.catalogService, s.contactService, s.idempotencyService, now)
	s.apiMarket = apimarket.NewManager(repositories.APIService, s.catalogService, s.contactService, now)
	s.apiIntent = apiintent.NewManager(repositories.APIPurchaseIntent, s.apiMarket, s.contactService, s.idempotencyService, now)
	s.reportService = report.NewService(repositories.Report, s.idempotencyService, now)
	s.apiOrder = apiorder.NewService(repositories.APIOrder, s.apiIntent, s.apiMarket, s.reportService, s.idempotencyService, now)
	s.apiIntent.SetOrderExistenceChecker(s.apiOrder)
	s.demandService = demand.NewService(repositories.Demand, s.idempotencyService, now)
	s.feedbackService = feedback.NewService(repositories.Feedback, s.notification, s.idempotencyService, now)
	s.favoriteService = favorite.NewService(repositories.Favorite, s.idempotencyService, s, now)
	s.reviewService = review.NewService(repositories.Review, s.idempotencyService, s, now)
	s.searchService = search.NewService(repositories.Search, s)
	s.modelAudit = modelaudit.NewService(repositories.ModelAudit, now)
	return s
}

func (s *Service) CreateDevSession(ctx context.Context, username string, isAdmin bool) (User, Session, *domain.AppError) {
	return s.authService.CreateDevSession(ctx, username, isAdmin)
}

func (s *Service) LoginWithOAuthProfile(ctx context.Context, profile OAuthProfile) (User, Session, *domain.AppError) {
	return s.authService.LoginWithOAuthProfile(ctx, profile)
}

func (s *Service) LoginWithPassword(ctx context.Context, username, password string) (User, Session, *domain.AppError) {
	return s.authService.LoginWithPassword(ctx, username, password)
}

func (s *Service) BootstrapAdmin(ctx context.Context, input BootstrapAdminInput) (BootstrapAdminResult, *domain.AppError) {
	return s.authService.BootstrapAdmin(ctx, input)
}

func (s *Service) StartEmailRegistration(ctx context.Context, input EmailRegistrationStartInput) (EmailRegistrationChallenge, *domain.AppError) {
	return s.authService.StartEmailRegistration(ctx, input)
}

func (s *Service) ConfirmEmailRegistration(ctx context.Context, input EmailRegistrationConfirmInput) (User, Session, *domain.AppError) {
	return s.authService.ConfirmEmailRegistration(ctx, input)
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

func (s *Service) AdminUsers(ctx context.Context, user User) ([]authmodule.AdminUser, *domain.AppError) {
	return s.authService.AdminUsers(ctx, user)
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

func (s *Service) SetProductCategoryActive(ctx context.Context, user User, categoryID string, active bool) (ProductCategory, *domain.AppError) {
	return s.catalogService.SetProductCategoryActive(ctx, user, categoryID, active)
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

func (s *Service) SetProductPlanActive(ctx context.Context, user User, planID string, active bool) (ProductPlan, *domain.AppError) {
	return s.catalogService.SetProductPlanActive(ctx, user, planID, active)
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

func (s *Service) SetAPIModelProviderActive(ctx context.Context, user User, providerID string, active bool) (APIModelProvider, *domain.AppError) {
	return s.catalogService.SetAPIModelProviderActive(ctx, user, providerID, active)
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

func (s *Service) SetAPIModelActive(ctx context.Context, user User, modelID string, active bool) (APIModelCatalog, *domain.AppError) {
	return s.catalogService.SetAPIModelActive(ctx, user, modelID, active)
}

func (s *Service) CreateAPIService(ctx context.Context, user User, input CreateAPIServiceInput) (APIService, *domain.AppError) {
	service, appErr := s.apiMarket.Create(ctx, user, input)
	if appErr != nil {
		return APIService{}, appErr
	}
	return s.withAPIMerchantProfile(ctx, service)
}

func (s *Service) UpdateAPIService(ctx context.Context, user User, input UpdateAPIServiceInput) (APIService, *domain.AppError) {
	service, appErr := s.apiMarket.Update(ctx, user, input)
	if appErr != nil {
		return APIService{}, appErr
	}
	return s.withAPIMerchantProfile(ctx, service)
}

func (s *Service) PublicAPIServices(ctx context.Context, filter apimarket.PublicServiceFilter) ([]APIService, *domain.AppError) {
	services, appErr := s.apiMarket.PublicServices(ctx, filter)
	if appErr != nil {
		return nil, appErr
	}
	return s.withAPIMerchantProfiles(ctx, services)
}

func (s *Service) PublicAPIService(ctx context.Context, serviceID string) (APIService, *domain.AppError) {
	service, appErr := s.apiMarket.PublicService(ctx, serviceID)
	if appErr != nil {
		return APIService{}, appErr
	}
	return s.withAPIMerchantProfile(ctx, service)
}

func (s *Service) OwnerAPIServices(ctx context.Context, user User, page domain.PageRequest) (domain.Page[APIService], *domain.AppError) {
	services, appErr := s.apiMarket.OwnerServices(ctx, user, page)
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
	service, appErr := s.apiMarket.OwnerService(ctx, user, serviceID)
	if appErr != nil {
		return APIService{}, appErr
	}
	return s.withAPIMerchantProfile(ctx, service)
}

func (s *Service) AdminAPIServices(ctx context.Context, user User, page domain.PageRequest) (domain.Page[APIService], *domain.AppError) {
	services, appErr := s.apiMarket.AdminServices(ctx, user, page)
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
	service, appErr := s.apiMarket.SubmitForReview(ctx, user, input)
	if appErr != nil {
		return APIService{}, appErr
	}
	return s.withAPIMerchantProfile(ctx, service)
}

func (s *Service) UpdateAPIServicePublication(ctx context.Context, user User, input APIServiceOwnerActionInput, action string) (APIService, *domain.AppError) {
	service, appErr := s.apiMarket.UpdatePublication(ctx, user, input, action)
	if appErr != nil {
		return APIService{}, appErr
	}
	return s.withAPIMerchantProfile(ctx, service)
}

func (s *Service) UpdateAPIServiceAdminStatus(ctx context.Context, user User, input APIServiceAdminActionInput) (APIService, *domain.AppError) {
	service, appErr := s.apiMarket.UpdateAdminStatus(ctx, user, input)
	if appErr != nil {
		return APIService{}, appErr
	}
	return s.withAPIMerchantProfile(ctx, service)
}

func (s *Service) UpdateAPIServiceOrderSettings(ctx context.Context, user User, input apimarket.UpdateOrderSettingsInput) (APIService, *domain.AppError) {
	service, appErr := s.apiMarket.UpdateOrderSettings(ctx, user, input)
	if appErr != nil {
		return APIService{}, appErr
	}
	return s.withAPIMerchantProfile(ctx, service)
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

func (s *Service) CreateAPIPurchaseIntentWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input CreateAPIPurchaseIntentInput, buildCompletion APIPurchaseIntentCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	_, completion, _, appErr := s.apiIntent.CreateWithIdempotency(ctx, userID, routeKey, key, requestHash, input, buildCompletion)
	if appErr != nil {
		return IdempotencyCompletion{}, appErr
	}
	return completion, nil
}

func (s *Service) MyAPIPurchaseIntents(ctx context.Context, user User) ([]APIPurchaseIntent, *domain.AppError) {
	return s.apiIntent.BuyerIntents(ctx, user)
}

func (s *Service) MyAPIPurchaseIntent(ctx context.Context, user User, intentID, requestID string) (APIPurchaseIntent, *domain.AppError) {
	return s.apiIntent.BuyerIntent(ctx, user, intentID, requestID)
}

func (s *Service) OwnerAPIPurchaseIntents(ctx context.Context, user User) ([]APIPurchaseIntent, *domain.AppError) {
	return s.apiIntent.OwnerIntents(ctx, user)
}

func (s *Service) OwnerAPIPurchaseIntent(ctx context.Context, user User, intentID, requestID string) (APIPurchaseIntent, *domain.AppError) {
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

func (s *Service) MarkAPIPurchaseIntentContactedWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input APIPurchaseIntentActionInput, buildCompletion APIPurchaseIntentCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.apiIntent.MarkContactedWithIdempotency(ctx, userID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) CloseAPIPurchaseIntentWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input APIPurchaseIntentActionInput, buildCompletion APIPurchaseIntentCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.apiIntent.CloseWithIdempotency(ctx, userID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) CreateAPIOrderWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input APIOrderActionInput, createInput CreateAPIOrderInput, buildCompletion APIOrderCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	_ = input
	order, completion, created, appErr := s.apiOrder.CreateWithIdempotencyResult(ctx, userID, routeKey, key, requestHash, createInput, buildCompletion)
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
	return s.apiOrder.BuyerOrders(ctx, user)
}

func (s *Service) MyAPIOrder(ctx context.Context, user User, orderID string) (APIOrder, *domain.AppError) {
	return s.apiOrder.BuyerOrder(ctx, user, orderID)
}

func (s *Service) ReadAPIOrderPaymentInstructions(ctx context.Context, user User, orderID, requestID string) (APIOrderPaymentInstructionsView, *domain.AppError) {
	return s.apiOrder.ReadPaymentInstructions(ctx, user, orderID, requestID)
}

func (s *Service) OwnerAPIOrders(ctx context.Context, user User) ([]APIOrder, *domain.AppError) {
	return s.apiOrder.SellerOrders(ctx, user)
}

func (s *Service) AdminAPIOrders(ctx context.Context, user User) ([]APIOrder, *domain.AppError) {
	return s.apiOrder.AdminOrders(ctx, user)
}

func (s *Service) OwnerAPIOrder(ctx context.Context, user User, orderID string) (APIOrder, *domain.AppError) {
	return s.apiOrder.SellerOrder(ctx, user, orderID)
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

func (s *Service) OpenAPIOrderDisputeWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input APIOrderActionInput, buildCompletion APIOrderCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.apiOrder.OpenDisputeWithIdempotency(ctx, userID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) ConfirmAPIOrderPaymentWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input APIOrderActionInput, buildCompletion APIOrderCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.apiOrder.ConfirmPaymentWithIdempotency(ctx, userID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) ReportAPIOrderPaymentIssueWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input APIOrderActionInput, buildCompletion APIOrderCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.apiOrder.ReportPaymentIssueWithIdempotency(ctx, userID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) SubmitAPIOrderDeliveryWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input APIOrderActionInput, buildCompletion APIOrderCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.apiOrder.SubmitDeliveryWithIdempotency(ctx, userID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) CreateCarpoolListing(ctx context.Context, user User, input CreateCarpoolListingInput) (CarpoolListing, *domain.AppError) {
	return s.carpoolService.CreateListing(ctx, user, input)
}

func (s *Service) PublishCarpoolListing(ctx context.Context, user User, input PublishCarpoolListingInput) (CarpoolListing, *domain.AppError) {
	return s.carpoolService.PublishListing(ctx, user, input)
}

func (s *Service) UpdateCarpoolListing(ctx context.Context, user User, input UpdateCarpoolListingInput) (CarpoolListing, *domain.AppError) {
	return s.carpoolService.UpdateListing(ctx, user, input)
}

func (s *Service) SubmitCarpoolListingForReview(ctx context.Context, user User, input SubmitCarpoolListingReviewInput) (CarpoolListing, *domain.AppError) {
	return s.carpoolService.SubmitListingForReview(ctx, user, input)
}

func (s *Service) PublicCarpoolListings(ctx context.Context, page domain.PageRequest) (domain.Page[CarpoolListing], *domain.AppError) {
	return s.carpoolService.PublicListings(ctx, page)
}

func (s *Service) PublicCarpoolListing(ctx context.Context, listingID string) (CarpoolListing, *domain.AppError) {
	return s.carpoolService.PublicListing(ctx, listingID)
}

func (s *Service) CarpoolApplicationEligibility(ctx context.Context, user User, listingID string) (carpool.ApplicationEligibility, *domain.AppError) {
	return s.carpoolService.ApplicationEligibility(ctx, user, listingID)
}

func (s *Service) MyCarpoolListings(ctx context.Context, user User) ([]CarpoolListing, *domain.AppError) {
	return s.carpoolService.MyListings(ctx, user)
}

func (s *Service) AdminCarpoolListings(ctx context.Context, user User, page domain.PageRequest) (domain.Page[CarpoolListing], *domain.AppError) {
	return s.carpoolService.AdminListings(ctx, user, page)
}

func (s *Service) AdminCarpoolListing(ctx context.Context, user User, listingID string) (CarpoolListing, *domain.AppError) {
	return s.carpoolService.AdminListing(ctx, user, listingID)
}

func (s *Service) UpdateCarpoolListingReviewStatus(ctx context.Context, user User, input CarpoolReviewInput) (CarpoolListing, *domain.AppError) {
	return s.carpoolService.UpdateListingReviewStatus(ctx, user, input)
}

func (s *Service) CreateCarpoolApplication(ctx context.Context, user User, input CreateCarpoolApplicationInput) (CarpoolApplication, *domain.AppError) {
	application, appErr := s.carpoolService.CreateApplication(ctx, user, input)
	if appErr != nil {
		return CarpoolApplication{}, appErr
	}
	s.sendCarpoolApplicationEmailIfNeeded(ctx, application)
	return application, nil
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

func (s *Service) MyCarpoolApplication(ctx context.Context, user User, applicationID string) (CarpoolApplication, *domain.AppError) {
	return s.carpoolService.MyApplication(ctx, user, applicationID)
}

func (s *Service) OwnerCarpoolApplications(ctx context.Context, user User) ([]CarpoolApplication, *domain.AppError) {
	return s.carpoolService.OwnerApplications(ctx, user)
}

func (s *Service) OwnerCarpoolApplication(ctx context.Context, user User, applicationID string) (CarpoolApplication, *domain.AppError) {
	return s.carpoolService.OwnerApplication(ctx, user, applicationID)
}

func (s *Service) AcceptCarpoolApplicationWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input AcceptCarpoolApplicationInput, buildCompletion CarpoolApplicationCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	application, completion, accepted, appErr := s.carpoolService.AcceptApplicationWithIdempotency(ctx, userID, routeKey, key, requestHash, input, buildCompletion)
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

func (s *Service) RejectCarpoolApplication(ctx context.Context, input RejectCarpoolApplicationInput) (CarpoolApplication, *domain.AppError) {
	return s.carpoolService.RejectApplication(ctx, input)
}

func (s *Service) CancelCarpoolApplicationWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input CancelCarpoolApplicationInput, buildCompletion CarpoolApplicationCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.carpoolService.CancelApplicationWithIdempotency(ctx, userID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) WithdrawCarpoolAcceptanceWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input WithdrawCarpoolAcceptanceInput, buildCompletion CarpoolApplicationCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.carpoolService.WithdrawAcceptanceWithIdempotency(ctx, userID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) ConfirmCarpoolApplicationJoinWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input ConfirmCarpoolApplicationJoinInput, buildCompletion CarpoolApplicationCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.carpoolService.ConfirmApplicationJoinWithIdempotency(ctx, userID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) MyCarpoolMemberships(ctx context.Context, user User) ([]CarpoolMembership, *domain.AppError) {
	return s.carpoolService.MyMemberships(ctx, user)
}

func (s *Service) MyCarpoolMembershipsByUserID(ctx context.Context, userID string) ([]CarpoolMembership, *domain.AppError) {
	return s.carpoolService.MyMemberships(ctx, User{ID: userID})
}

func (s *Service) OwnerCarpoolMemberships(ctx context.Context, user User) ([]CarpoolMembership, *domain.AppError) {
	return s.carpoolService.OwnerMemberships(ctx, user)
}

func (s *Service) ConfirmCarpoolMembershipCompleteWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input ConfirmCarpoolMembershipCompleteInput, buildCompletion CarpoolMembershipCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.carpoolService.ConfirmMembershipCompleteWithIdempotency(ctx, userID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) EndCarpoolMembershipWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input EndCarpoolMembershipInput, buildCompletion CarpoolMembershipCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.carpoolService.EndMembershipWithIdempotency(ctx, userID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) CreateContactMethod(ctx context.Context, input ContactMethodInput) (ContactMethod, *domain.AppError) {
	return s.contactService.CreateMethod(ctx, input)
}

func (s *Service) ListContactMethods(ctx context.Context, userID string) ([]ContactMethod, *domain.AppError) {
	return s.contactService.ListMethods(ctx, userID)
}

func (s *Service) UpdateContactMethod(ctx context.Context, input contactmodule.UpdateContactMethodInput) (ContactMethod, *domain.AppError) {
	return s.contactService.UpdateMethod(ctx, input)
}

func (s *Service) DeleteContactMethod(ctx context.Context, userID, methodID string) (ContactMethod, *domain.AppError) {
	return s.contactService.DeleteMethod(ctx, userID, methodID)
}

func (s *Service) SetDefaultContactMethod(ctx context.Context, userID, methodID string) (ContactMethod, *domain.AppError) {
	return s.contactService.SetDefaultMethod(ctx, userID, methodID)
}

func (s *Service) VerifyContactMethod(ctx context.Context, userID, methodID string) (ContactMethod, *domain.AppError) {
	return s.contactService.VerifyMethod(ctx, userID, methodID)
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
	stats, appErr := s.reportService.PublicUserDisputeStats(ctx, username)
	if appErr != nil {
		return PublicUserProfile{}, appErr
	}
	publicProfile.Stats.UnresolvedDisputeCount = stats.UnresolvedCount
	resolved := stats.ResolvedLast90Days
	if !publicProfile.Privacy.ShowResolvedDisputeSummary {
		publicProfile.Stats.ResolvedDisputeCountLast90Days = nil
	} else {
		publicProfile.Stats.ResolvedDisputeCountLast90Days = &resolved
	}
	return publicProfile, nil
}

func (s *Service) MyMerchantProfile(ctx context.Context, user User) (MerchantProfile, *domain.AppError) {
	return s.profileService.MyMerchantProfile(ctx, user)
}

func (s *Service) UpsertMyMerchantProfile(ctx context.Context, user User, input UpsertMerchantProfileInput) (MerchantProfile, *domain.AppError) {
	return s.profileService.UpsertMyMerchantProfile(ctx, user, input)
}

func (s *Service) PublicMerchantProfile(ctx context.Context, slug string) (PublicMerchantProfile, *domain.AppError) {
	return s.profileService.PublicMerchantProfile(ctx, slug)
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

func (s *Service) CreateDemand(ctx context.Context, user User, input CreateDemandInput) (Demand, *domain.AppError) {
	return s.demandService.Create(ctx, user, input)
}

func (s *Service) PublicDemands(ctx context.Context) ([]Demand, *domain.AppError) {
	return s.demandService.PublicDemands(ctx)
}

func (s *Service) SearchMarket(ctx context.Context, keyword string) ([]search.Result, *domain.AppError) {
	return s.searchService.Search(ctx, keyword)
}

func (s *Service) PublicDemand(ctx context.Context, demandID string) (Demand, *domain.AppError) {
	return s.demandService.PublicDemand(ctx, demandID)
}

func (s *Service) MyDemands(ctx context.Context, user User) ([]Demand, *domain.AppError) {
	return s.demandService.MyDemands(ctx, user)
}

func (s *Service) MyDemand(ctx context.Context, user User, demandID string) (Demand, *domain.AppError) {
	return s.demandService.MyDemand(ctx, user, demandID)
}

func (s *Service) AdminDemands(ctx context.Context, user User) ([]Demand, *domain.AppError) {
	return s.demandService.AdminDemands(ctx, user)
}

func (s *Service) AdminDemand(ctx context.Context, user User, demandID string) (Demand, *domain.AppError) {
	return s.demandService.AdminDemand(ctx, user, demandID)
}

func (s *Service) CloseDemandWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input DemandOwnerActionInput, buildCompletion DemandCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.demandService.CloseWithIdempotency(ctx, userID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) ReopenDemandWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input DemandOwnerActionInput, buildCompletion DemandCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.demandService.ReopenWithIdempotency(ctx, userID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) AdminDemandActionWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input DemandAdminActionInput, buildCompletion DemandCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.demandService.AdminActionWithIdempotency(ctx, userID, routeKey, key, requestHash, input, buildCompletion)
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

func (s *Service) SubmitCarpoolMembershipReviewWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input review.SubmitReviewInput, buildCompletion review.CompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.reviewService.SubmitWithIdempotency(ctx, userID, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) PublicUserReviews(ctx context.Context, username string) ([]review.PublicReview, *domain.AppError) {
	return s.reviewService.PublicForUser(ctx, username)
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

func (s *Service) AdminDisputes(ctx context.Context, user User) ([]report.DisputeCase, *domain.AppError) {
	return s.reportService.AdminDisputes(ctx, user)
}

func (s *Service) AdminDispute(ctx context.Context, user User, id string) (report.DisputeCase, *domain.AppError) {
	return s.reportService.AdminDispute(ctx, user, id)
}

func (s *Service) AdminDisputeActionWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input report.AdminActionInput, buildCompletion report.AdminCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.reportService.AdminDisputeActionWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
}

func (s *Service) CreateAppealWithIdempotency(ctx context.Context, user User, routeKey, key, requestHash string, input report.CreateAppealInput, buildCompletion report.AppealCompletionBuilder) (IdempotencyCompletion, *domain.AppError) {
	return s.reportService.CreateAppealWithIdempotency(ctx, user, routeKey, key, requestHash, input, buildCompletion)
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
