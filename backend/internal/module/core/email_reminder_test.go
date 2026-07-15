package core

import (
	"context"
	"net/http"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apimarket"
)

type emailReminderFakeSender struct {
	verificationCodes []string
	carpoolCreated    []sentCarpoolCreatedEmail
	carpoolAccepted   []sentCarpoolAcceptedEmail
	apiIntentCreated  []sentAPIPurchaseIntentEmail
	failCarpoolAccept bool
	failAPIIntent     bool
}

type sentCarpoolCreatedEmail struct {
	to            string
	listingTitle  string
	applicationID string
}

type sentCarpoolAcceptedEmail struct {
	to            string
	listingTitle  string
	applicationID string
	joinDeadline  *time.Time
}

type sentAPIPurchaseIntentEmail struct {
	to           string
	serviceTitle string
	intentID     string
	buyerNote    string
}

func (f *emailReminderFakeSender) SendVerificationCode(_ context.Context, toEmail, code string, _ time.Time) *domain.AppError {
	f.verificationCodes = append(f.verificationCodes, toEmail+":"+code)
	return nil
}

func (f *emailReminderFakeSender) SendRegistrationSuccess(context.Context, string, string, string, time.Time) *domain.AppError {
	return nil
}

func (f *emailReminderFakeSender) SendCarpoolApplicationCreated(_ context.Context, toEmail, listingTitle, applicationID string, _ time.Time) *domain.AppError {
	f.carpoolCreated = append(f.carpoolCreated, sentCarpoolCreatedEmail{
		to:            toEmail,
		listingTitle:  listingTitle,
		applicationID: applicationID,
	})
	return nil
}

func (f *emailReminderFakeSender) SendCarpoolApplicationAccepted(_ context.Context, toEmail, listingTitle, applicationID string, joinDeadline *time.Time) *domain.AppError {
	var deadlineCopy *time.Time
	if joinDeadline != nil {
		value := *joinDeadline
		deadlineCopy = &value
	}
	f.carpoolAccepted = append(f.carpoolAccepted, sentCarpoolAcceptedEmail{
		to:            toEmail,
		listingTitle:  listingTitle,
		applicationID: applicationID,
		joinDeadline:  deadlineCopy,
	})
	if f.failCarpoolAccept {
		return domain.NewError(http.StatusBadGateway, domain.CodeInternalError, "Email send failed", "邮件发送失败。")
	}
	return nil
}

func (f *emailReminderFakeSender) SendAPIPurchaseIntentCreated(_ context.Context, toEmail, serviceTitle, intentID, buyerNote string, _ time.Time) *domain.AppError {
	f.apiIntentCreated = append(f.apiIntentCreated, sentAPIPurchaseIntentEmail{
		to:           toEmail,
		serviceTitle: serviceTitle,
		intentID:     intentID,
		buyerNote:    buyerNote,
	})
	if f.failAPIIntent {
		return domain.NewError(http.StatusBadGateway, domain.CodeInternalError, "Email send failed", "邮件发送失败。")
	}
	return nil
}

func (f *emailReminderFakeSender) ExposeDevCode() bool {
	return true
}

func TestCarpoolAcceptanceEmailSentToVerifiedBuyerOnce(t *testing.T) {
	ctx := context.Background()
	service, sender := newEmailReminderTestService()
	owner := testBoundUser("owner-carpool-email", "owner-carpool-email")
	buyer := testUser("buyer-carpool-email", "buyer-carpool-email")
	verifyProfileEmail(t, service, buyer, "buyer-carpool@example.com")

	ownerContact := createTestContactMethod(t, service, owner.ID, "telegram", "Owner TG", "@owner_carpool_email")
	buyerContact := createTestContactMethod(t, service, buyer.ID, "telegram", "Buyer TG", "@buyer_carpool_email")
	application := createTestCarpoolApplication(t, service, owner, buyer, ownerContact.ID, buyerContact.ID)

	_, appErr := service.AcceptCarpoolApplicationWithIdempotency(ctx, owner.ID, "accept-carpool-email", "accept-key", "accept-hash", AcceptCarpoolApplicationInput{
		ApplicationID:   application.ID,
		ExpectedVersion: application.Version,
		RequestID:       "accept-request",
	}, testCarpoolApplicationCompletion)
	if appErr != nil {
		t.Fatalf("accept carpool application: %v", appErr)
	}
	if len(sender.carpoolAccepted) != 1 {
		t.Fatalf("expected one buyer acceptance email, got %+v", sender.carpoolAccepted)
	}
	sent := sender.carpoolAccepted[0]
	if sent.to != "buyer-carpool@example.com" || sent.listingTitle != application.ListingTitleSnapshot || sent.applicationID != application.ID || sent.joinDeadline == nil {
		t.Fatalf("unexpected buyer acceptance email: %+v", sent)
	}

	_, appErr = service.AcceptCarpoolApplicationWithIdempotency(ctx, owner.ID, "accept-carpool-email", "accept-key", "accept-hash", AcceptCarpoolApplicationInput{
		ApplicationID:   application.ID,
		ExpectedVersion: application.Version,
		RequestID:       "accept-request",
	}, testCarpoolApplicationCompletion)
	if appErr != nil {
		t.Fatalf("replay accept carpool application: %v", appErr)
	}
	if len(sender.carpoolAccepted) != 1 {
		t.Fatalf("idempotent replay must not send duplicate buyer email, got %+v", sender.carpoolAccepted)
	}
}

func TestCarpoolAcceptanceEmailSkipsUnverifiedBuyer(t *testing.T) {
	ctx := context.Background()
	service, sender := newEmailReminderTestService()
	owner := testBoundUser("owner-carpool-unverified", "owner-carpool-unverified")
	buyer := testUser("buyer-carpool-unverified", "buyer-carpool-unverified")

	ownerContact := createTestContactMethod(t, service, owner.ID, "telegram", "Owner TG", "@owner_carpool_unverified")
	buyerContact := createTestContactMethod(t, service, buyer.ID, "telegram", "Buyer TG", "@buyer_carpool_unverified")
	application := createTestCarpoolApplication(t, service, owner, buyer, ownerContact.ID, buyerContact.ID)

	_, appErr := service.AcceptCarpoolApplicationWithIdempotency(ctx, owner.ID, "accept-carpool-unverified", "accept-key", "accept-hash", AcceptCarpoolApplicationInput{
		ApplicationID:   application.ID,
		ExpectedVersion: application.Version,
		RequestID:       "accept-request",
	}, testCarpoolApplicationCompletion)
	if appErr != nil {
		t.Fatalf("accept carpool application: %v", appErr)
	}
	if len(sender.carpoolAccepted) != 0 {
		t.Fatalf("unverified buyer email must be skipped, got %+v", sender.carpoolAccepted)
	}
}

func TestAPIPurchaseIntentEmailSentToVerifiedMerchantOnce(t *testing.T) {
	ctx := context.Background()
	service, sender := newEmailReminderTestService()
	owner := testBoundUser("owner-api-email", "owner-api-email")
	buyer := testUser("buyer-api-email", "buyer-api-email")
	verifyProfileEmail(t, service, owner, "merchant-api@example.com")

	ownerContact := createTestContactMethod(t, service, owner.ID, "telegram", "Owner TG", "@owner_api_email")
	buyerContact := createTestContactMethod(t, service, buyer.ID, "telegram", "Buyer TG", "@buyer_api_email")
	apiService := createOrderableAPIService(t, service, owner, ownerContact.ID)

	_, appErr := service.CreateAPIPurchaseIntentWithIdempotency(ctx, buyer.ID, "api-intent-email", "intent-key", "intent-hash", CreateAPIPurchaseIntentInput{
		APIServiceID:          apiService.ID,
		BuyerContactMethodID:  buyerContact.ID,
		RequestedCNYAmount:    "16.00",
		RequestedUSDAllowance: "20.000000",
		SelectedAccessMode:    "buyer_dedicated_sub_key",
		BuyerNote:             "希望站外确认 20 美元额度。",
		RequestID:             "intent-request",
	}, testAPIPurchaseIntentCompletion)
	if appErr != nil {
		t.Fatalf("create API purchase intent: %v", appErr)
	}
	if len(sender.apiIntentCreated) != 1 {
		t.Fatalf("expected one merchant API intent email, got %+v", sender.apiIntentCreated)
	}
	sent := sender.apiIntentCreated[0]
	if sent.to != "merchant-api@example.com" || sent.serviceTitle != apiService.Title || sent.intentID == "" || sent.buyerNote != "希望站外确认 20 美元额度。" {
		t.Fatalf("unexpected merchant API intent email: %+v", sent)
	}

	_, appErr = service.CreateAPIPurchaseIntentWithIdempotency(ctx, buyer.ID, "api-intent-email", "intent-key", "intent-hash", CreateAPIPurchaseIntentInput{
		APIServiceID:          apiService.ID,
		BuyerContactMethodID:  buyerContact.ID,
		RequestedCNYAmount:    "16.00",
		RequestedUSDAllowance: "20.000000",
		SelectedAccessMode:    "buyer_dedicated_sub_key",
		BuyerNote:             "希望站外确认 20 美元额度。",
		RequestID:             "intent-request",
	}, testAPIPurchaseIntentCompletion)
	if appErr != nil {
		t.Fatalf("replay API purchase intent: %v", appErr)
	}
	if len(sender.apiIntentCreated) != 1 {
		t.Fatalf("idempotent replay must not send duplicate merchant email, got %+v", sender.apiIntentCreated)
	}
}

func TestAPIPurchaseIntentEmailSkipsUnverifiedMerchant(t *testing.T) {
	ctx := context.Background()
	service, sender := newEmailReminderTestService()
	owner := testBoundUser("owner-api-unverified", "owner-api-unverified")
	buyer := testUser("buyer-api-unverified", "buyer-api-unverified")

	ownerContact := createTestContactMethod(t, service, owner.ID, "telegram", "Owner TG", "@owner_api_unverified")
	buyerContact := createTestContactMethod(t, service, buyer.ID, "telegram", "Buyer TG", "@buyer_api_unverified")
	apiService := createOrderableAPIService(t, service, owner, ownerContact.ID)

	_, appErr := service.CreateAPIPurchaseIntentWithIdempotency(ctx, buyer.ID, "api-intent-unverified", "intent-key", "intent-hash", CreateAPIPurchaseIntentInput{
		APIServiceID:          apiService.ID,
		BuyerContactMethodID:  buyerContact.ID,
		RequestedCNYAmount:    "16.00",
		RequestedUSDAllowance: "20.000000",
		SelectedAccessMode:    "buyer_dedicated_sub_key",
		BuyerNote:             "希望站外确认 20 美元额度。",
		RequestID:             "intent-request",
	}, testAPIPurchaseIntentCompletion)
	if appErr != nil {
		t.Fatalf("create API purchase intent: %v", appErr)
	}
	if len(sender.apiIntentCreated) != 0 {
		t.Fatalf("unverified merchant email must be skipped, got %+v", sender.apiIntentCreated)
	}
}

func TestEmailReminderFailuresDoNotBlockBusinessOperations(t *testing.T) {
	ctx := context.Background()
	service, sender := newEmailReminderTestService()
	sender.failCarpoolAccept = true
	sender.failAPIIntent = true

	owner := testBoundUser("owner-email-failure", "owner-email-failure")
	buyer := testUser("buyer-email-failure", "buyer-email-failure")
	verifyProfileEmail(t, service, buyer, "buyer-failure@example.com")
	verifyProfileEmail(t, service, owner, "merchant-failure@example.com")

	ownerContact := createTestContactMethod(t, service, owner.ID, "telegram", "Owner TG", "@owner_email_failure")
	buyerContact := createTestContactMethod(t, service, buyer.ID, "telegram", "Buyer TG", "@buyer_email_failure")
	application := createTestCarpoolApplication(t, service, owner, buyer, ownerContact.ID, buyerContact.ID)
	if _, appErr := service.AcceptCarpoolApplicationWithIdempotency(ctx, owner.ID, "accept-email-failure", "accept-key", "accept-hash", AcceptCarpoolApplicationInput{
		ApplicationID:   application.ID,
		ExpectedVersion: application.Version,
		RequestID:       "accept-request",
	}, testCarpoolApplicationCompletion); appErr != nil {
		t.Fatalf("email failure must not block carpool acceptance: %v", appErr)
	}

	apiService := createOrderableAPIService(t, service, owner, ownerContact.ID)
	if _, appErr := service.CreateAPIPurchaseIntentWithIdempotency(ctx, buyer.ID, "api-intent-email-failure", "intent-key", "intent-hash", CreateAPIPurchaseIntentInput{
		APIServiceID:          apiService.ID,
		BuyerContactMethodID:  buyerContact.ID,
		RequestedCNYAmount:    "16.00",
		RequestedUSDAllowance: "20.000000",
		SelectedAccessMode:    "buyer_dedicated_sub_key",
		BuyerNote:             "希望站外确认 20 美元额度。",
		RequestID:             "intent-request",
	}, testAPIPurchaseIntentCompletion); appErr != nil {
		t.Fatalf("email failure must not block API purchase intent: %v", appErr)
	}
	if len(sender.carpoolAccepted) != 1 || len(sender.apiIntentCreated) != 1 {
		t.Fatalf("expected failed email attempts to be recorded, got carpool=%+v api=%+v", sender.carpoolAccepted, sender.apiIntentCreated)
	}
}

func newEmailReminderTestService() (*Service, *emailReminderFakeSender) {
	sender := &emailReminderFakeSender{}
	now := func() time.Time { return time.Date(2026, 7, 6, 10, 0, 0, 0, time.UTC) }
	return newServiceWithEmailSender(now, Repositories{}, sender), sender
}

func testUser(id, username string) User {
	return User{ID: id, Username: username, DisplayName: username, Status: "active"}
}

func testBoundUser(id, username string) User {
	user := testUser(id, username)
	user.LinuxDoBinding = &LinuxDoBinding{
		Bound:           true,
		LinuxDoUserID:   id,
		LinuxDoUsername: username,
		TrustLevel:      3,
		BoundAt:         time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC),
		LastSyncedAt:    time.Date(2026, 7, 6, 9, 0, 0, 0, time.UTC),
	}
	return user
}

func verifyProfileEmail(t *testing.T, service *Service, user User, email string) {
	t.Helper()
	challenge, appErr := service.StartEmailVerification(context.Background(), user, EmailVerificationStartInput{Email: email})
	if appErr != nil {
		t.Fatalf("start email verification: %v", appErr)
	}
	if challenge.DevCode == "" {
		t.Fatalf("expected development email code")
	}
	if _, appErr := service.ConfirmEmailVerification(context.Background(), user, EmailVerificationConfirmInput{Email: email, Code: challenge.DevCode}); appErr != nil {
		t.Fatalf("confirm email verification: %v", appErr)
	}
}

func createTestContactMethod(t *testing.T, service *Service, userID, methodType, label, value string) ContactMethod {
	t.Helper()
	method, appErr := service.CreateContactMethod(context.Background(), ContactMethodInput{
		UserID:    userID,
		Type:      methodType,
		Label:     label,
		Value:     value,
		IsDefault: true,
		Enabled:   true,
	})
	if appErr != nil {
		t.Fatalf("create contact method: %v", appErr)
	}
	return method
}

func createTestCarpoolApplication(t *testing.T, service *Service, owner, buyer User, ownerContactID, buyerContactID string) CarpoolApplication {
	t.Helper()
	listing, appErr := service.PublishCarpoolListing(context.Background(), owner, PublishCarpoolListingInput{
		ProductPlanID:        "00000000-0000-0000-0000-000000000401",
		OwnerContactMethodID: ownerContactID,
		CycleTerm: CarpoolCycleTermInput{
			BillingPeriod: "monthly",
			NoticeDays:    7,
			ExitPolicy:    "提前 7 天站外确认退出安排。",
			UsageRules:    "仅限买家本人使用，不共享凭据。",
		},
		Title:                  "Claude Pro 拼车",
		Summary:                "Claude Pro 社区拼车名额。",
		AccessArrangement:      "席位和使用安排站外确认。",
		DistributionMethod:     "sub2api",
		DistributionMethodNote: "Sub2API 托管管理，具体方式站外确认。",
		ProvidesAdminAccount:   true,
		RegionCode:             "us",
		RegionName:             "美国区",
		PriceMonthlyCNY:        "20.00",
		ServiceMultiplier:      "1.0000",
		MonthlyQuotaAmount:     "20.000000",
		BuyerSeatCapacity:      2,
	})
	if appErr != nil {
		t.Fatalf("publish carpool listing: %v", appErr)
	}
	application, appErr := service.CreateCarpoolApplication(context.Background(), buyer, CreateCarpoolApplicationInput{
		ListingID:            listing.ID,
		BuyerContactMethodID: buyerContactID,
		RiskAcknowledgement:  nil,
	})
	if appErr != nil {
		t.Fatalf("create carpool application: %v", appErr)
	}
	return application
}

func createOrderableAPIService(t *testing.T, service *Service, owner User, ownerContactID string) APIService {
	t.Helper()
	created, appErr := service.CreateAPIService(context.Background(), owner, CreateAPIServiceInput{
		MerchantIdentityMode:             "public_profile",
		OwnerContactMethodID:             ownerContactID,
		Title:                            "Sub2API 美元额度意向服务",
		ShortDescription:                 "商户声明美元额度售价，双方站外确认具体安排。",
		DistributionSystem:               apimarket.ServiceDistributionSub2API,
		BillingMode:                      apimarket.ServiceBillingModeMetered,
		DeclaredCNYPerUSDAllowance:       "0.8000",
		DeclaredMaxUSDAllowancePerIntent: "20.000000",
		QuotaExpiresAt:                   "2026-08-08T00:00:00Z",
		MinimumIntentCNY:                 "10.00",
		MaximumIntentCNY:                 "200.00",
		UsageVisibility:                  "merchant_reported",
		PublicAccessNote:                 "提交购买意向后直接查看商户联系方式，平台不保存任何调用凭据。",
		MerchantNote:                     "仅后台可见，不展示给公开访客。",
		MerchantSupportNote:              "仅支持买家专属的子级访问安排。",
		AccessModes: []APIServiceAccessModeInput{
			{AccessMode: "buyer_dedicated_sub_key", PublicNote: "站外确认买家专属的访问方式。"},
		},
		Models: []APIServiceModelInput{
			{ModelCatalogID: "00000000-0000-0000-0000-000000000a01", MerchantMultiplier: "1.0000", Enabled: true},
		},
	})
	if appErr != nil {
		t.Fatalf("create API service: %v", appErr)
	}
	submitted, appErr := service.SubmitAPIServiceForReview(context.Background(), owner, APIServiceOwnerActionInput{
		ServiceID:       created.ID,
		ExpectedVersion: created.Version,
		RequestID:       "submit-api-service",
	})
	if appErr != nil {
		t.Fatalf("submit API service: %v", appErr)
	}
	published, appErr := service.UpdateAPIServicePublication(context.Background(), owner, APIServiceOwnerActionInput{
		ServiceID:       submitted.ID,
		ExpectedVersion: submitted.Version,
		RequestID:       "publish-api-service",
	}, "publish")
	if appErr != nil {
		t.Fatalf("publish API service: %v", appErr)
	}
	orderable, appErr := service.UpdateAPIServiceOrderSettings(context.Background(), owner, apimarket.UpdateOrderSettingsInput{
		ServiceID:            published.ID,
		AcceptingOrders:      true,
		PaymentWindowMinutes: 10,
		PaymentOptions: []apimarket.PaymentOptionInput{
			{PaymentMethod: apimarket.PaymentMethodWechat, Enabled: true, PaymentInstructions: "微信收款信息站外确认，付款后填写付款摘要。", PaymentQRCodeDataURL: "data:image/png;base64,iVBORw0KGgo="},
		},
		ExpectedVersion: published.Version,
		RequestID:       "api-service-order-settings",
	})
	if appErr != nil {
		t.Fatalf("update API order settings: %v", appErr)
	}
	return orderable
}

func testCarpoolApplicationCompletion(application CarpoolApplication) (IdempotencyCompletion, *domain.AppError) {
	return IdempotencyCompletion{
		Status:       http.StatusOK,
		ContentType:  "application/json; charset=utf-8",
		Body:         []byte(application.ID),
		ResourceType: "carpool_application",
		ResourceID:   application.ID,
	}, nil
}

func testAPIPurchaseIntentCompletion(intent APIPurchaseIntent) (IdempotencyCompletion, *domain.AppError) {
	return IdempotencyCompletion{
		Status:       http.StatusCreated,
		ContentType:  "application/json; charset=utf-8",
		Body:         []byte(intent.ID),
		ResourceType: "api_purchase_intent",
		ResourceID:   intent.ID,
	}, nil
}
