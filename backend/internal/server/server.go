package server

import (
	"context"
	"time"

	"c2c-market/backend/internal/config"
	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/health"
	"c2c-market/backend/internal/middleware"
	"c2c-market/backend/internal/module/announcement"
	"c2c-market/backend/internal/module/apiintent"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/carpool"
	"c2c-market/backend/internal/module/catalog"
	"c2c-market/backend/internal/module/contact"
	"c2c-market/backend/internal/module/demand"
	"c2c-market/backend/internal/module/favorite"
	"c2c-market/backend/internal/module/feedback"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/notification"
	"c2c-market/backend/internal/module/officialprice"
	"c2c-market/backend/internal/module/profile"
	"c2c-market/backend/internal/module/report"
	"c2c-market/backend/internal/module/review"
	"c2c-market/backend/internal/module/search"
	"github.com/go-chi/chi/v5"
	"net/http"
)

const (
	sessionCookieName = middleware.SessionCookieName
	csrfHeaderName    = middleware.CSRFHeaderName
	requestIDHeader   = middleware.RequestIDHeader
)

type ServerOptions struct {
	EnableDevAuth    bool
	ReadinessChecker health.Checker
	AppEnv           string
	AllowedOrigins   []string
	OAuth            OAuthOptions
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

type Service interface {
	CreateDevSession(ctx context.Context, username string, isAdmin bool) (auth.User, auth.Session, *domain.AppError)
	LoginWithOAuthProfile(ctx context.Context, profile auth.OAuthProfile) (auth.User, auth.Session, *domain.AppError)
	LoginWithPassword(ctx context.Context, username, password string) (auth.User, auth.Session, *domain.AppError)
	StartEmailRegistration(ctx context.Context, input auth.EmailRegistrationStartInput) (auth.EmailRegistrationChallenge, *domain.AppError)
	ConfirmEmailRegistration(ctx context.Context, input auth.EmailRegistrationConfirmInput) (auth.User, auth.Session, *domain.AppError)
	SetPassword(ctx context.Context, input auth.SetPasswordInput) *domain.AppError
	GetSession(ctx context.Context, sessionID string) (auth.User, auth.Session, *domain.AppError)
	GetSessionWithCSRF(ctx context.Context, sessionID, csrfToken string) (auth.User, auth.Session, *domain.AppError)
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
	SetProductCategoryActive(ctx context.Context, user auth.User, categoryID string, active bool) (catalog.ProductCategory, *domain.AppError)
	ProductPlans(ctx context.Context, categoryCode string) ([]catalog.ProductPlan, *domain.AppError)
	ProductPlan(ctx context.Context, planID string) (catalog.ProductPlan, *domain.AppError)
	AdminProductPlans(ctx context.Context, user auth.User, categoryCode string) ([]catalog.ProductPlan, *domain.AppError)
	AdminProductPlan(ctx context.Context, user auth.User, planID string) (catalog.ProductPlan, *domain.AppError)
	CreateProductPlan(ctx context.Context, user auth.User, input catalog.ProductPlanInput) (catalog.ProductPlan, *domain.AppError)
	UpdateProductPlan(ctx context.Context, user auth.User, planID string, input catalog.ProductPlanInput) (catalog.ProductPlan, *domain.AppError)
	SetProductPlanActive(ctx context.Context, user auth.User, planID string, active bool) (catalog.ProductPlan, *domain.AppError)
	AdminAPIModelProviders(ctx context.Context, user auth.User) ([]catalog.APIModelProvider, *domain.AppError)
	AdminAPIModelProvider(ctx context.Context, user auth.User, providerID string) (catalog.APIModelProvider, *domain.AppError)
	CreateAPIModelProvider(ctx context.Context, user auth.User, input catalog.APIModelProviderInput) (catalog.APIModelProvider, *domain.AppError)
	UpdateAPIModelProvider(ctx context.Context, user auth.User, providerID string, input catalog.APIModelProviderInput) (catalog.APIModelProvider, *domain.AppError)
	SetAPIModelProviderActive(ctx context.Context, user auth.User, providerID string, active bool) (catalog.APIModelProvider, *domain.AppError)
	APIModels(ctx context.Context) ([]catalog.APIModelCatalog, *domain.AppError)
	APIModel(ctx context.Context, modelID string) (catalog.APIModelCatalog, *domain.AppError)
	AdminAPIModels(ctx context.Context, user auth.User) ([]catalog.APIModelCatalog, *domain.AppError)
	AdminAPIModel(ctx context.Context, user auth.User, modelID string) (catalog.APIModelCatalog, *domain.AppError)
	CreateAPIModel(ctx context.Context, user auth.User, input catalog.APIModelInput) (catalog.APIModelCatalog, *domain.AppError)
	UpdateAPIModel(ctx context.Context, user auth.User, modelID string, input catalog.APIModelInput) (catalog.APIModelCatalog, *domain.AppError)
	SetAPIModelActive(ctx context.Context, user auth.User, modelID string, active bool) (catalog.APIModelCatalog, *domain.AppError)

	SubmitOfficialPriceLead(ctx context.Context, user auth.User, input officialprice.SubmitLeadInput) (officialprice.Lead, *domain.AppError)
	MyOfficialPriceLeads(ctx context.Context, user auth.User) ([]officialprice.Lead, *domain.AppError)
	MyOfficialPriceLead(ctx context.Context, user auth.User, leadID string) (officialprice.Lead, *domain.AppError)
	AdminOfficialPriceLeads(ctx context.Context, user auth.User) ([]officialprice.Lead, *domain.AppError)
	AdminOfficialPriceLead(ctx context.Context, user auth.User, leadID string) (officialprice.Lead, *domain.AppError)
	ApproveOfficialPriceLeadWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input officialprice.ApproveLeadInput, buildCompletion officialprice.ApprovalCompletionBuilder) (idempotency.Completion, *domain.AppError)
	UpdateLeadReviewStatus(ctx context.Context, user auth.User, leadID, status, reason string, ifMatchVersion int64) (officialprice.Lead, *domain.AppError)
	PublicOfficialPriceRecords(ctx context.Context) ([]officialprice.Record, *domain.AppError)
	PublicOfficialPriceRecord(ctx context.Context, recordID string) (officialprice.Record, *domain.AppError)

	CreateDemand(ctx context.Context, user auth.User, input demand.CreateInput) (demand.Demand, *domain.AppError)
	PublicDemands(ctx context.Context) ([]demand.Demand, *domain.AppError)
	PublicDemand(ctx context.Context, demandID string) (demand.Demand, *domain.AppError)
	SearchMarket(ctx context.Context, keyword string) ([]search.Result, *domain.AppError)
	MyDemands(ctx context.Context, user auth.User) ([]demand.Demand, *domain.AppError)
	MyDemand(ctx context.Context, user auth.User, demandID string) (demand.Demand, *domain.AppError)
	AdminDemands(ctx context.Context, user auth.User) ([]demand.Demand, *domain.AppError)
	AdminDemand(ctx context.Context, user auth.User, demandID string) (demand.Demand, *domain.AppError)
	CloseDemandWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input demand.OwnerActionInput, buildCompletion demand.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	ReopenDemandWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input demand.OwnerActionInput, buildCompletion demand.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	AdminDemandActionWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input demand.AdminActionInput, buildCompletion demand.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	CreateFeedbackTicketWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input feedback.CreateInput, buildCompletion feedback.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	MyFeedbackTickets(ctx context.Context, user auth.User) ([]feedback.Ticket, *domain.AppError)
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
	SubmitCarpoolMembershipReviewWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input review.SubmitReviewInput, buildCompletion review.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	PublicUserReviews(ctx context.Context, username string) ([]review.PublicReview, *domain.AppError)
	CreateReportWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input report.CreateReportInput, buildCompletion report.ReportCompletionBuilder) (idempotency.Completion, *domain.AppError)
	MyReports(ctx context.Context, user auth.User) ([]report.Report, *domain.AppError)
	AdminReports(ctx context.Context, user auth.User) ([]report.Report, *domain.AppError)
	AdminReport(ctx context.Context, user auth.User, id string) (report.Report, *domain.AppError)
	AdminReportActionWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input report.AdminActionInput, buildCompletion report.AdminCompletionBuilder) (idempotency.Completion, *domain.AppError)
	AdminDisputes(ctx context.Context, user auth.User) ([]report.DisputeCase, *domain.AppError)
	AdminDispute(ctx context.Context, user auth.User, id string) (report.DisputeCase, *domain.AppError)
	AdminDisputeActionWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input report.AdminActionInput, buildCompletion report.AdminCompletionBuilder) (idempotency.Completion, *domain.AppError)
	CreateAppealWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input report.CreateAppealInput, buildCompletion report.AppealCompletionBuilder) (idempotency.Completion, *domain.AppError)
	MyAppeals(ctx context.Context, user auth.User) ([]report.Appeal, *domain.AppError)
	AdminAppeals(ctx context.Context, user auth.User) ([]report.Appeal, *domain.AppError)
	AdminAppeal(ctx context.Context, user auth.User, id string) (report.Appeal, *domain.AppError)
	AdminAppealActionWithIdempotency(ctx context.Context, user auth.User, routeKey, key, requestHash string, input report.AdminActionInput, buildCompletion report.AdminCompletionBuilder) (idempotency.Completion, *domain.AppError)
	PublicUserDisputes(ctx context.Context, username string) ([]report.PublicDispute, *domain.AppError)

	CreateCarpoolListing(ctx context.Context, user auth.User, input carpool.CreateListingInput) (carpool.Listing, *domain.AppError)
	PublishCarpoolListing(ctx context.Context, user auth.User, input carpool.PublishListingInput) (carpool.Listing, *domain.AppError)
	UpdateCarpoolListing(ctx context.Context, user auth.User, input carpool.UpdateListingInput) (carpool.Listing, *domain.AppError)
	SubmitCarpoolListingForReview(ctx context.Context, user auth.User, input carpool.SubmitListingReviewInput) (carpool.Listing, *domain.AppError)
	PublicCarpoolListings(ctx context.Context) ([]carpool.Listing, *domain.AppError)
	PublicCarpoolListing(ctx context.Context, listingID string) (carpool.Listing, *domain.AppError)
	MyCarpoolListings(ctx context.Context, user auth.User) ([]carpool.Listing, *domain.AppError)
	AdminCarpoolListings(ctx context.Context, user auth.User) ([]carpool.Listing, *domain.AppError)
	AdminCarpoolListing(ctx context.Context, user auth.User, listingID string) (carpool.Listing, *domain.AppError)
	UpdateCarpoolListingReviewStatus(ctx context.Context, user auth.User, input carpool.ReviewInput) (carpool.Listing, *domain.AppError)
	CreateCarpoolApplication(ctx context.Context, user auth.User, input carpool.CreateApplicationInput) (carpool.Application, *domain.AppError)
	MyCarpoolApplications(ctx context.Context, user auth.User) ([]carpool.Application, *domain.AppError)
	MyCarpoolApplication(ctx context.Context, user auth.User, applicationID string) (carpool.Application, *domain.AppError)
	OwnerCarpoolApplications(ctx context.Context, user auth.User) ([]carpool.Application, *domain.AppError)
	OwnerCarpoolApplication(ctx context.Context, user auth.User, applicationID string) (carpool.Application, *domain.AppError)
	AcceptCarpoolApplicationWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input carpool.AcceptApplicationInput, buildCompletion carpool.ApplicationCompletionBuilder) (idempotency.Completion, *domain.AppError)
	RejectCarpoolApplication(ctx context.Context, input carpool.RejectApplicationInput) (carpool.Application, *domain.AppError)
	CancelCarpoolApplicationWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input carpool.CancelApplicationInput, buildCompletion carpool.ApplicationCompletionBuilder) (idempotency.Completion, *domain.AppError)
	WithdrawCarpoolAcceptanceWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input carpool.WithdrawAcceptanceInput, buildCompletion carpool.ApplicationCompletionBuilder) (idempotency.Completion, *domain.AppError)
	ConfirmCarpoolApplicationJoinWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input carpool.ConfirmApplicationJoinInput, buildCompletion carpool.ApplicationCompletionBuilder) (idempotency.Completion, *domain.AppError)
	MyCarpoolMemberships(ctx context.Context, user auth.User) ([]carpool.Membership, *domain.AppError)
	OwnerCarpoolMemberships(ctx context.Context, user auth.User) ([]carpool.Membership, *domain.AppError)
	ConfirmCarpoolMembershipCompleteWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input carpool.ConfirmMembershipCompleteInput, buildCompletion carpool.MembershipCompletionBuilder) (idempotency.Completion, *domain.AppError)
	EndCarpoolMembershipWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input carpool.EndMembershipInput, buildCompletion carpool.MembershipCompletionBuilder) (idempotency.Completion, *domain.AppError)

	CreateAPIService(ctx context.Context, user auth.User, input apimarket.CreateServiceInput) (apimarket.Service, *domain.AppError)
	UpdateAPIService(ctx context.Context, user auth.User, input apimarket.UpdateServiceInput) (apimarket.Service, *domain.AppError)
	PublicAPIServices(ctx context.Context, filter apimarket.PublicServiceFilter) ([]apimarket.Service, *domain.AppError)
	PublicAPIService(ctx context.Context, serviceID string) (apimarket.Service, *domain.AppError)
	OwnerAPIServices(ctx context.Context, user auth.User) ([]apimarket.Service, *domain.AppError)
	OwnerAPIService(ctx context.Context, user auth.User, serviceID string) (apimarket.Service, *domain.AppError)
	AdminAPIServices(ctx context.Context, user auth.User) ([]apimarket.Service, *domain.AppError)
	AdminAPIService(ctx context.Context, user auth.User, serviceID string) (apimarket.Service, *domain.AppError)
	SubmitAPIServiceForReview(ctx context.Context, user auth.User, input apimarket.ServiceOwnerActionInput) (apimarket.Service, *domain.AppError)
	UpdateAPIServicePublication(ctx context.Context, user auth.User, input apimarket.ServiceOwnerActionInput, action string) (apimarket.Service, *domain.AppError)
	UpdateAPIServiceAdminStatus(ctx context.Context, user auth.User, input apimarket.ServiceAdminActionInput) (apimarket.Service, *domain.AppError)
	UpdateAPIServiceOrderSettings(ctx context.Context, user auth.User, input apimarket.UpdateOrderSettingsInput) (apimarket.Service, *domain.AppError)

	CreateAPIPurchaseIntentWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input apiintent.CreateIntentInput, buildCompletion apiintent.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	MyAPIPurchaseIntents(ctx context.Context, user auth.User) ([]apiintent.Intent, *domain.AppError)
	MyAPIPurchaseIntent(ctx context.Context, user auth.User, intentID, requestID string) (apiintent.Intent, *domain.AppError)
	CancelAPIPurchaseIntentWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input apiintent.ActionInput, buildCompletion apiintent.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	OwnerAPIPurchaseIntents(ctx context.Context, user auth.User) ([]apiintent.Intent, *domain.AppError)
	OwnerAPIPurchaseIntent(ctx context.Context, user auth.User, intentID, requestID string) (apiintent.Intent, *domain.AppError)
	MarkAPIPurchaseIntentContactedWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input apiintent.ActionInput, buildCompletion apiintent.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	CloseAPIPurchaseIntentWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input apiintent.ActionInput, buildCompletion apiintent.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	AdminAPIPurchaseIntents(ctx context.Context, user auth.User) ([]apiintent.Intent, *domain.AppError)
	AdminAPIPurchaseIntent(ctx context.Context, user auth.User, intentID string) (apiintent.Intent, *domain.AppError)

	CreateAPIOrderWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input apiorder.ActionInput, createInput apiorder.CreateInput, buildCompletion apiorder.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	MyAPIOrders(ctx context.Context, user auth.User) ([]apiorder.Order, *domain.AppError)
	MyAPIOrder(ctx context.Context, user auth.User, orderID string) (apiorder.Order, *domain.AppError)
	ReadAPIOrderPaymentInstructions(ctx context.Context, user auth.User, orderID, requestID string) (apiorder.PaymentInstructionsView, *domain.AppError)
	SubmitAPIOrderPaymentWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input apiorder.ActionInput, buildCompletion apiorder.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	CancelAPIOrderWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input apiorder.ActionInput, buildCompletion apiorder.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	ConfirmAPIOrderCompleteWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input apiorder.ActionInput, buildCompletion apiorder.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	OpenAPIOrderDisputeWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input apiorder.ActionInput, buildCompletion apiorder.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	OwnerAPIOrders(ctx context.Context, user auth.User) ([]apiorder.Order, *domain.AppError)
	OwnerAPIOrder(ctx context.Context, user auth.User, orderID string) (apiorder.Order, *domain.AppError)
	ConfirmAPIOrderPaymentWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input apiorder.ActionInput, buildCompletion apiorder.CompletionBuilder) (idempotency.Completion, *domain.AppError)
	SubmitAPIOrderDeliveryWithIdempotency(ctx context.Context, userID, routeKey, key, requestHash string, input apiorder.ActionInput, buildCompletion apiorder.CompletionBuilder) (idempotency.Completion, *domain.AppError)

	CreateContactMethod(ctx context.Context, input contact.ContactMethodInput) (contact.ContactMethod, *domain.AppError)
	ListContactMethods(ctx context.Context, userID string) ([]contact.ContactMethod, *domain.AppError)
	UpdateContactMethod(ctx context.Context, input contact.UpdateContactMethodInput) (contact.ContactMethod, *domain.AppError)
	DeleteContactMethod(ctx context.Context, userID, methodID string) (contact.ContactMethod, *domain.AppError)
	SetDefaultContactMethod(ctx context.Context, userID, methodID string) (contact.ContactMethod, *domain.AppError)
	VerifyContactMethod(ctx context.Context, userID, methodID string) (contact.ContactMethod, *domain.AppError)
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

	MyNotifications(ctx context.Context, user auth.User) ([]notification.Notification, *domain.AppError)
	MyNotificationUnreadCount(ctx context.Context, user auth.User) (int, *domain.AppError)
	MarkNotificationRead(ctx context.Context, user auth.User, id string) (notification.Notification, *domain.AppError)
	MarkAllNotificationsRead(ctx context.Context, user auth.User) (notification.ReadAllResult, *domain.AppError)
}

type Server struct {
	app              Service
	mux              chi.Router
	enableDevAuth    bool
	readinessChecker health.Checker
	oauth            OAuthOptions
	cookieSecure     bool
	allowedOrigins   []string
	rateLimiter      *middleware.RateLimiter
	oauthHTTPClient  *http.Client
}

func NewServer(service Service, options ...ServerOptions) http.Handler {
	option := ServerOptions{EnableDevAuth: true, AppEnv: config.EnvDevelopment}
	if len(options) > 0 {
		option = options[0]
	}
	if option.AppEnv == "" {
		option.AppEnv = config.EnvDevelopment
	}
	server := &Server{
		app:              service,
		mux:              chi.NewRouter(),
		enableDevAuth:    option.EnableDevAuth,
		readinessChecker: option.ReadinessChecker,
		oauth:            option.OAuth,
		cookieSecure:     option.AppEnv == config.EnvProduction,
		allowedOrigins:   append([]string(nil), option.AllowedOrigins...),
		rateLimiter:      middleware.NewRateLimiter(time.Minute),
		oauthHTTPClient:  &http.Client{Timeout: 10 * time.Second},
	}
	server.routes()
	return middleware.WithRequestID(
		middleware.WithSecurityHeaders(
			middleware.WithCORSAndOrigin(server.mux, middleware.CORSOptions{
				AllowedOrigins: server.allowedOrigins,
				Production:     option.AppEnv == config.EnvProduction,
			}),
			middleware.SecurityHeadersOptions{HSTS: option.AppEnv == config.EnvProduction},
		),
	)
}
