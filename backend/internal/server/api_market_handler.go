package server

import (
	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apiintent"
	"c2c-market/backend/internal/module/apimarket"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/contact"
	"c2c-market/backend/internal/module/idempotency"
	"context"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"
	"time"
)

type apiServiceRequest struct {
	MerchantProfileID                string                        `json:"merchantProfileId"`
	MerchantIdentityMode             string                        `json:"merchantIdentityMode"`
	OwnerContactMethodID             string                        `json:"ownerContactMethodId"`
	Title                            string                        `json:"title"`
	ShortDescription                 string                        `json:"shortDescription"`
	SourceURL                        string                        `json:"sourceUrl"`
	DistributionSystem               string                        `json:"distributionSystem"`
	BillingMode                      string                        `json:"billingMode"`
	DeclaredCNYPerUSDAllowance       string                        `json:"declaredCnyPerUsdAllowance"`
	DeclaredMaxUSDAllowancePerIntent string                        `json:"declaredMaxUsdAllowancePerIntent"`
	AvailableUSDAllowance            string                        `json:"availableUsdAllowance"`
	QuotaExpiresAt                   string                        `json:"quotaExpiresAt"`
	MinimumIntentCNY                 string                        `json:"minimumIntentCny"`
	MaximumIntentCNY                 string                        `json:"maximumIntentCny"`
	UsageVisibility                  string                        `json:"usageVisibility"`
	PublicAccessNote                 string                        `json:"publicAccessNote"`
	MerchantNote                     string                        `json:"merchantNote"`
	MerchantSupportNote              string                        `json:"merchantSupportNote"`
	AccessModes                      []apiServiceAccessModeRequest `json:"accessModes"`
	Models                           []apiServiceModelRequest      `json:"models"`
	Packages                         []apiServicePackageRequest    `json:"packages"`
}

type apiServiceOrderSettingsRequest struct {
	AcceptingOrders      *bool                            `json:"acceptingOrders"`
	PaymentWindowMinutes int                              `json:"paymentWindowMinutes"`
	PaymentOptions       []apiServicePaymentOptionRequest `json:"paymentOptions"`
}

type apiServicePaymentOptionRequest struct {
	PaymentMethod        string `json:"paymentMethod"`
	Enabled              *bool  `json:"enabled"`
	PaymentInstructions  string `json:"paymentInstructions"`
	PaymentQRCodeDataURL string `json:"paymentQrCodeDataUrl"`
}

type apiServiceAccessModeRequest struct {
	AccessMode string `json:"accessMode"`
	PublicNote string `json:"publicNote"`
}

type apiServiceModelRequest struct {
	ModelCatalogID      string `json:"modelCatalogId"`
	ModelPriceVersionID string `json:"modelPriceVersionId"`
	MerchantMultiplier  string `json:"merchantMultiplier"`
	Enabled             *bool  `json:"enabled"`
}

type apiServicePackageRequest struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	PriceCNY        string   `json:"priceCny"`
	PanelAllowance  string   `json:"panelAllowance"`
	DurationDays    *int     `json:"durationDays"`
	StockTotal      int      `json:"stockTotal"`
	Description     string   `json:"description"`
	Enabled         *bool    `json:"enabled"`
	SortOrder       int      `json:"sortOrder"`
	ModelCatalogIDs []string `json:"modelCatalogIds"`
}

type apiServiceResponse struct {
	ID                               string                            `json:"id"`
	OwnerUserID                      string                            `json:"ownerUserId"`
	MerchantProfileID                string                            `json:"merchantProfileId,omitempty"`
	MerchantIdentityMode             string                            `json:"merchantIdentityMode"`
	MerchantDisplayName              string                            `json:"merchantDisplayName,omitempty"`
	MerchantProfileSlug              string                            `json:"merchantProfileSlug,omitempty"`
	MerchantAvatarURL                string                            `json:"merchantAvatarUrl,omitempty"`
	OwnerContactMethodID             string                            `json:"ownerContactMethodId,omitempty"`
	Title                            string                            `json:"title"`
	ShortDescription                 string                            `json:"shortDescription"`
	SourceURL                        string                            `json:"sourceUrl,omitempty"`
	DistributionSystem               string                            `json:"distributionSystem"`
	BillingMode                      string                            `json:"billingMode"`
	DeclaredCNYPerUSDAllowance       string                            `json:"declaredCnyPerUsdAllowance,omitempty"`
	DeclaredMaxUSDAllowancePerIntent string                            `json:"declaredMaxUsdAllowancePerIntent,omitempty"`
	AvailableUSDAllowance            string                            `json:"availableUsdAllowance,omitempty"`
	QuotaExpiresAt                   *string                           `json:"quotaExpiresAt,omitempty"`
	MinimumIntentCNY                 string                            `json:"minimumIntentCny"`
	MaximumIntentCNY                 string                            `json:"maximumIntentCny,omitempty"`
	UsageVisibility                  string                            `json:"usageVisibility"`
	PublicAccessNote                 string                            `json:"publicAccessNote,omitempty"`
	MerchantNote                     string                            `json:"merchantNote,omitempty"`
	MerchantSupportNote              string                            `json:"merchantSupportNote,omitempty"`
	AcceptingOrders                  bool                              `json:"acceptingOrders"`
	PaymentWindowMinutes             int                               `json:"paymentWindowMinutes"`
	AcceptedPaymentMethods           []string                          `json:"acceptedPaymentMethods"`
	PaymentOptions                   []apiServicePaymentOptionResponse `json:"paymentOptions,omitempty"`
	IsOrderable                      bool                              `json:"isOrderable"`
	OrderableReasons                 []string                          `json:"orderableReasons,omitempty"`
	ReviewStatus                     string                            `json:"reviewStatus"`
	PublicationStatus                string                            `json:"publicationStatus"`
	ModerationStatus                 string                            `json:"moderationStatus"`
	ApprovedByAdminID                string                            `json:"approvedByAdminId,omitempty"`
	ApprovedAt                       *string                           `json:"approvedAt,omitempty"`
	ModerationReason                 string                            `json:"moderationReason,omitempty"`
	AccessModes                      []apiServiceAccessModeResponse    `json:"accessModes"`
	Models                           []apiServiceModelResponse         `json:"models"`
	Packages                         []apiServicePackageResponse       `json:"packages"`
	Version                          int64                             `json:"version"`
	CreatedAt                        string                            `json:"createdAt"`
	UpdatedAt                        string                            `json:"updatedAt"`
}

type publicAPIServiceResponse struct {
	ID                               string                         `json:"id"`
	MerchantIdentityMode             string                         `json:"merchantIdentityMode"`
	MerchantDisplayName              string                         `json:"merchantDisplayName,omitempty"`
	MerchantProfileSlug              string                         `json:"merchantProfileSlug,omitempty"`
	MerchantAvatarURL                string                         `json:"merchantAvatarUrl,omitempty"`
	Title                            string                         `json:"title"`
	ShortDescription                 string                         `json:"shortDescription"`
	SourceURL                        string                         `json:"sourceUrl,omitempty"`
	DistributionSystem               string                         `json:"distributionSystem"`
	BillingMode                      string                         `json:"billingMode"`
	DeclaredCNYPerUSDAllowance       string                         `json:"declaredCnyPerUsdAllowance,omitempty"`
	DeclaredMaxUSDAllowancePerIntent string                         `json:"declaredMaxUsdAllowancePerIntent,omitempty"`
	AvailableUSDAllowance            string                         `json:"availableUsdAllowance,omitempty"`
	QuotaExpiresAt                   *string                        `json:"quotaExpiresAt,omitempty"`
	MinimumIntentCNY                 string                         `json:"minimumIntentCny"`
	MaximumIntentCNY                 string                         `json:"maximumIntentCny,omitempty"`
	UsageVisibility                  string                         `json:"usageVisibility"`
	PublicAccessNote                 string                         `json:"publicAccessNote,omitempty"`
	MerchantSupportNote              string                         `json:"merchantSupportNote,omitempty"`
	AcceptingOrders                  bool                           `json:"acceptingOrders"`
	PaymentWindowMinutes             int                            `json:"paymentWindowMinutes"`
	AcceptedPaymentMethods           []string                       `json:"acceptedPaymentMethods"`
	IsOrderable                      bool                           `json:"isOrderable"`
	OrderableReasons                 []string                       `json:"orderableReasons,omitempty"`
	AccessModes                      []apiServiceAccessModeResponse `json:"accessModes"`
	Models                           []apiServiceModelResponse      `json:"models"`
	Packages                         []apiServicePackageResponse    `json:"packages"`
	Completed30d                     int                            `json:"completed30d"`
	UnresolvedDisputes               int                            `json:"unresolvedDisputes"`
	ResponseMedianMinutes            *float64                       `json:"responseMedianMinutes"`
	Version                          int64                          `json:"version"`
	CreatedAt                        string                         `json:"createdAt"`
	UpdatedAt                        string                         `json:"updatedAt"`
}

type apiServiceAccessModeResponse struct {
	AccessMode string `json:"accessMode"`
	PublicNote string `json:"publicNote,omitempty"`
}

type apiServiceModelResponse struct {
	ID                                  string   `json:"id"`
	ModelCatalogID                      string   `json:"modelCatalogId"`
	ModelPriceVersionID                 string   `json:"modelPriceVersionId,omitempty"`
	ModelNameSnapshot                   string   `json:"modelNameSnapshot"`
	ProviderSnapshot                    string   `json:"providerSnapshot"`
	CapabilitiesSnapshot                []string `json:"capabilitiesSnapshot"`
	MerchantMultiplier                  string   `json:"merchantMultiplier"`
	EffectiveInputPricePerMillion       string   `json:"effectiveInputPricePerMillion,omitempty"`
	EffectiveCachedInputPricePerMillion string   `json:"effectiveCachedInputPricePerMillion,omitempty"`
	EffectiveOutputPricePerMillion      string   `json:"effectiveOutputPricePerMillion,omitempty"`
	Enabled                             bool     `json:"enabled"`
}

type apiServicePackageResponse struct {
	ID             string                           `json:"id"`
	Name           string                           `json:"name"`
	PriceCNY       string                           `json:"priceCny"`
	PanelAllowance string                           `json:"panelAllowance"`
	DurationDays   *int                             `json:"durationDays,omitempty"`
	StockTotal     int                              `json:"stockTotal"`
	StockAvailable int                              `json:"stockAvailable"`
	Description    string                           `json:"description"`
	Enabled        bool                             `json:"enabled"`
	SortOrder      int                              `json:"sortOrder"`
	Models         []apiServicePackageModelResponse `json:"models"`
}

type apiServicePackageModelResponse struct {
	ServiceModelID      string `json:"serviceModelId"`
	ModelCatalogID      string `json:"modelCatalogId"`
	ModelPriceVersionID string `json:"modelPriceVersionId,omitempty"`
	ModelNameSnapshot   string `json:"modelNameSnapshot"`
	ProviderSnapshot    string `json:"providerSnapshot"`
	MerchantMultiplier  string `json:"merchantMultiplier"`
}

type apiServicePaymentOptionResponse struct {
	ID                   string `json:"id,omitempty"`
	PaymentMethod        string `json:"paymentMethod"`
	Enabled              bool   `json:"enabled"`
	PaymentInstructions  string `json:"paymentInstructions,omitempty"`
	PaymentQRCodeDataURL string `json:"paymentQrCodeDataUrl,omitempty"`
	Version              int64  `json:"version,omitempty"`
}

type createAPIPurchaseIntentRequest struct {
	BuyerContactMethodID  string `json:"buyerContactMethodId"`
	RequestedCNYAmount    string `json:"requestedCnyAmount"`
	RequestedUSDAllowance string `json:"requestedUsdAllowance"`
	SelectedAccessMode    string `json:"selectedAccessMode"`
	SelectedPackageID     string `json:"selectedPackageId"`
	BuyerNote             string `json:"buyerNote"`
}

type apiPurchaseIntentActionRequest struct {
	Reason string `json:"reason"`
}

type apiPurchaseIntentCoreResponse struct {
	ID                                       string  `json:"id"`
	APIServiceID                             string  `json:"apiServiceId"`
	Status                                   string  `json:"status"`
	RequestedCNYAmount                       string  `json:"requestedCnyAmount"`
	RequestedUSDAllowance                    string  `json:"requestedUsdAllowance,omitempty"`
	SelectedAccessMode                       string  `json:"selectedAccessMode"`
	SelectedPackageID                        string  `json:"selectedPackageId,omitempty"`
	SelectedPackageSnapshot                  string  `json:"selectedPackageSnapshot,omitempty"`
	ServiceVersionSnapshot                   int64   `json:"serviceVersionSnapshot"`
	ServiceTitleSnapshot                     string  `json:"serviceTitleSnapshot"`
	DistributionSystemSnapshot               string  `json:"distributionSystemSnapshot"`
	BillingModeSnapshot                      string  `json:"billingModeSnapshot"`
	DeclaredCNYPerUSDAllowanceSnapshot       string  `json:"declaredCnyPerUsdAllowanceSnapshot,omitempty"`
	DeclaredMaxUSDAllowancePerIntentSnapshot string  `json:"declaredMaxUsdAllowancePerIntentSnapshot,omitempty"`
	MinimumIntentCNYSnapshot                 string  `json:"minimumIntentCnySnapshot"`
	MaximumIntentCNYSnapshot                 string  `json:"maximumIntentCnySnapshot,omitempty"`
	PricingSnapshot                          string  `json:"pricingSnapshot"`
	BuyerNote                                string  `json:"buyerNote,omitempty"`
	ContactedAt                              *string `json:"contactedAt,omitempty"`
	BuyerCancelledAt                         *string `json:"buyerCancelledAt,omitempty"`
	BuyerCancelReason                        string  `json:"buyerCancelReason,omitempty"`
	OwnerClosedAt                            *string `json:"ownerClosedAt,omitempty"`
	OwnerCloseReason                         string  `json:"ownerCloseReason,omitempty"`
	Version                                  int64   `json:"version"`
	CreatedAt                                string  `json:"createdAt"`
	UpdatedAt                                string  `json:"updatedAt"`
}

type apiPurchaseIntentListItemResponse struct {
	apiPurchaseIntentCoreResponse
}

type createAPIPurchaseIntentResponse struct {
	apiPurchaseIntentCoreResponse
	MerchantContact *contactDisclosureDTO `json:"merchantContact"`
}

type buyerAPIPurchaseIntentDetailResponse struct {
	apiPurchaseIntentCoreResponse
	MerchantContact *contactDisclosureDTO `json:"merchantContact"`
}

type ownerAPIPurchaseIntentDetailResponse struct {
	apiPurchaseIntentCoreResponse
	BuyerUserID          string                `json:"buyerUserId,omitempty"`
	BuyerContactMethodID string                `json:"buyerContactMethodId,omitempty"`
	BuyerContact         *contactDisclosureDTO `json:"buyerContact"`
}

type ownerAPIPurchaseIntentListItemResponse struct {
	apiPurchaseIntentCoreResponse
	BuyerUserID          string `json:"buyerUserId,omitempty"`
	BuyerContactMethodID string `json:"buyerContactMethodId,omitempty"`
}

type adminAPIPurchaseIntentDetailResponse struct {
	apiPurchaseIntentCoreResponse
	BuyerUserID          string `json:"buyerUserId,omitempty"`
	OwnerUserID          string `json:"ownerUserId,omitempty"`
	BuyerContactMethodID string `json:"buyerContactMethodId,omitempty"`
	OwnerContactMethodID string `json:"ownerContactMethodId,omitempty"`
}

type contactDisclosureDTO struct {
	Side        string `json:"side"`
	Type        string `json:"type"`
	Label       string `json:"label"`
	Value       string `json:"value"`
	MaskedValue string `json:"maskedValue"`
}

func (s *Server) handlePublicAPIServices(w http.ResponseWriter, r *http.Request) {
	services, appErr := s.app.PublicAPIServices(r.Context(), apimarket.PublicServiceFilter{
		PaymentMethod: r.URL.Query().Get("paymentMethod"),
	})
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toPublicAPIServiceResponses(services))
}

func (s *Server) handleUpdateAPIServiceOrderSettings(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	req, appErr := decodeStrictJSONOnly[apiServiceOrderSettingsRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	input := apimarket.UpdateOrderSettingsInput{
		ServiceID:            chi.URLParam(r, "id"),
		AcceptingOrders:      req.AcceptingOrders != nil && *req.AcceptingOrders,
		PaymentWindowMinutes: req.PaymentWindowMinutes,
		PaymentOptions:       toAppPaymentOptionInputs(req.PaymentOptions),
		ExpectedVersion:      version,
		RequestID:            requestIDFrom(r),
	}
	service, appErr := s.app.UpdateAPIServiceOrderSettings(r.Context(), user, input)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, service.Version)
	writeJSON(w, http.StatusOK, toAPIServiceResponse(service))
}

func (s *Server) handlePublicAPIService(w http.ResponseWriter, r *http.Request) {
	service, appErr := s.app.PublicAPIService(r.Context(), chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, service.Version)
	writeJSON(w, http.StatusOK, toPublicAPIServiceResponse(service))
}

func (s *Server) handleCreateAPIPurchaseIntent(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[createAPIPurchaseIntentRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	serviceID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/api-services/{id}/purchase-intents"
	completion, appErr := s.app.CreateAPIPurchaseIntentWithIdempotency(
		r.Context(),
		user.ID,
		routeKey,
		r.Header.Get("Idempotency-Key"),
		requestHash(r.Method, routeKey+":"+serviceID, body),
		apiintent.CreateIntentInput{
			APIServiceID:          serviceID,
			BuyerContactMethodID:  req.BuyerContactMethodID,
			RequestedCNYAmount:    req.RequestedCNYAmount,
			RequestedUSDAllowance: req.RequestedUSDAllowance,
			SelectedAccessMode:    req.SelectedAccessMode,
			SelectedPackageID:     req.SelectedPackageID,
			BuyerNote:             req.BuyerNote,
			RequestID:             requestIDFrom(r),
		},
		func(intent apiintent.Intent) (idempotency.Completion, *domain.AppError) {
			responseBody, marshalErr := json.Marshal(toCreateAPIPurchaseIntentResponse(intent))
			if marshalErr != nil {
				return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
			}
			return idempotency.Completion{
				Status:        http.StatusCreated,
				ContentType:   "application/json; charset=utf-8",
				Body:          responseBody,
				SkipBodyCache: true,
				ResourceType:  "api_purchase_intent",
				ResourceID:    intent.ID,
				Headers: map[string]string{
					"ETag":     `"` + strconv.FormatInt(intent.Version, 10) + `"`,
					"Location": "/api/v1/me/api-purchase-intents/" + intent.ID,
				},
			}, nil
		},
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeNoStoreIdempotencyCompletion(w, completion)
}
func (s *Server) handleMyAPIPurchaseIntents(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	intents, appErr := s.app.MyAPIPurchaseIntents(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toBuyerAPIPurchaseIntentListResponses(intents))
}

func (s *Server) handleMyAPIPurchaseIntent(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	intent, appErr := s.app.MyAPIPurchaseIntent(r.Context(), user, chi.URLParam(r, "id"), requestIDFrom(r))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, intent.Version)
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, http.StatusOK, toBuyerAPIPurchaseIntentDetailResponse(intent))
}

func (s *Server) handleCancelAPIPurchaseIntent(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[apiPurchaseIntentActionRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	intentID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/me/api-purchase-intents/{id}/cancel"
	completion, appErr := s.app.CancelAPIPurchaseIntentWithIdempotency(
		r.Context(),
		user.ID,
		routeKey,
		r.Header.Get("Idempotency-Key"),
		requestHash(r.Method, routeKey+":"+intentID, body),
		apiintent.ActionInput{
			IntentID:        intentID,
			Reason:          req.Reason,
			ExpectedVersion: version,
			RequestID:       requestIDFrom(r),
		},
		apiPurchaseIntentCompletionBuilder(toBuyerAPIPurchaseIntentListItemResponse),
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}
func (s *Server) handleOwnerAPIServices(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	pageRequest, appErr := parsePageRequest(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	services, appErr := s.app.OwnerAPIServices(r.Context(), user, pageRequest)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePageJSON(w, domain.Page[apiServiceResponse]{
		Items:      toAPIServiceResponses(services.Items),
		NextCursor: services.NextCursor,
	})
}

func (s *Server) handleOwnerAPIService(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	service, appErr := s.app.OwnerAPIService(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, service.Version)
	writeJSON(w, http.StatusOK, toAPIServiceResponse(service))
}

func (s *Server) handleCreateAPIService(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[apiServiceRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	s.withIdempotency(w, r, user.ID, "POST /api/v1/owner/api-services", body, func() (int, any, string, string, *domain.AppError) {
		service, errApp := s.app.CreateAPIService(r.Context(), user, toAppCreateAPIServiceInput(req))
		if errApp != nil {
			return 0, nil, "", "", errApp
		}
		setETag(w, service.Version)
		return http.StatusCreated, toAPIServiceResponse(service), "api_service", service.ID, nil
	})
}

func (s *Server) handleUpdateAPIService(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	req, appErr := decodeStrictJSONOnly[apiServiceRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	input := toAppUpdateAPIServiceInput(req)
	input.ServiceID = chi.URLParam(r, "id")
	input.ExpectedVersion = version
	input.RequestID = requestIDFrom(r)
	service, appErr := s.app.UpdateAPIService(r.Context(), user, input)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, service.Version)
	writeJSON(w, http.StatusOK, toAPIServiceResponse(service))
}

func (s *Server) handleSubmitAPIServiceReview(w http.ResponseWriter, r *http.Request) {
	s.handleOwnerAPIServiceAction(w, r, "submit-review", func(ctx context.Context, user auth.User, input apimarket.ServiceOwnerActionInput) (apimarket.Service, *domain.AppError) {
		return s.app.SubmitAPIServiceForReview(ctx, user, input)
	})
}

func (s *Server) handlePublishAPIService(w http.ResponseWriter, r *http.Request) {
	s.handleOwnerAPIServiceAction(w, r, "publish", func(ctx context.Context, user auth.User, input apimarket.ServiceOwnerActionInput) (apimarket.Service, *domain.AppError) {
		return s.app.UpdateAPIServicePublication(ctx, user, input, "publish")
	})
}

func (s *Server) handlePauseAPIService(w http.ResponseWriter, r *http.Request) {
	s.handleOwnerAPIServiceAction(w, r, "pause", func(ctx context.Context, user auth.User, input apimarket.ServiceOwnerActionInput) (apimarket.Service, *domain.AppError) {
		return s.app.UpdateAPIServicePublication(ctx, user, input, "pause")
	})
}

func (s *Server) handleResumeAPIService(w http.ResponseWriter, r *http.Request) {
	s.handleOwnerAPIServiceAction(w, r, "resume", func(ctx context.Context, user auth.User, input apimarket.ServiceOwnerActionInput) (apimarket.Service, *domain.AppError) {
		return s.app.UpdateAPIServicePublication(ctx, user, input, "resume")
	})
}

func (s *Server) handleStartAPIServiceRevision(w http.ResponseWriter, r *http.Request) {
	s.handleOwnerAPIServiceAction(w, r, "start-revision", func(ctx context.Context, user auth.User, input apimarket.ServiceOwnerActionInput) (apimarket.Service, *domain.AppError) {
		return s.app.UpdateAPIServicePublication(ctx, user, input, "start_revision")
	})
}

func (s *Server) handleOwnerAPIServiceAction(w http.ResponseWriter, r *http.Request, action string, run func(context.Context, auth.User, apimarket.ServiceOwnerActionInput) (apimarket.Service, *domain.AppError)) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, _, appErr := decodeStrictJSON[emptyRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	serviceID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/owner/api-services/{id}/" + action + ":" + serviceID
	s.withIdempotency(w, r, user.ID, routeKey, body, func() (int, any, string, string, *domain.AppError) {
		service, errApp := run(r.Context(), user, apimarket.ServiceOwnerActionInput{
			ServiceID:       serviceID,
			ExpectedVersion: version,
			RequestID:       requestIDFrom(r),
		})
		if errApp != nil {
			return 0, nil, "", "", errApp
		}
		setETag(w, service.Version)
		return http.StatusOK, toAPIServiceResponse(service), "api_service", service.ID, nil
	})
}

func (s *Server) handleOwnerAPIPurchaseIntents(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	intents, appErr := s.app.OwnerAPIPurchaseIntents(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toOwnerAPIPurchaseIntentListResponses(intents))
}

func (s *Server) handleOwnerAPIPurchaseIntent(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	intent, appErr := s.app.OwnerAPIPurchaseIntent(r.Context(), user, chi.URLParam(r, "id"), requestIDFrom(r))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, intent.Version)
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, http.StatusOK, toOwnerAPIPurchaseIntentDetailResponse(intent))
}

func (s *Server) handleMarkAPIPurchaseIntentContacted(w http.ResponseWriter, r *http.Request) {
	s.handleOwnerAPIPurchaseIntentAction(w, r, "mark-contacted", false, func(ctx context.Context, user auth.User, routeKey, idempotencyKey string, body []byte, input apiintent.ActionInput) (idempotency.Completion, *domain.AppError) {
		return s.app.MarkAPIPurchaseIntentContactedWithIdempotency(ctx, user.ID, routeKey, idempotencyKey, requestHash(http.MethodPost, routeKey+":"+input.IntentID, body), input, apiPurchaseIntentCompletionBuilder(toOwnerAPIPurchaseIntentListItemResponse))
	})
}

func (s *Server) handleCloseAPIPurchaseIntent(w http.ResponseWriter, r *http.Request) {
	s.handleOwnerAPIPurchaseIntentAction(w, r, "close", true, func(ctx context.Context, user auth.User, routeKey, idempotencyKey string, body []byte, input apiintent.ActionInput) (idempotency.Completion, *domain.AppError) {
		return s.app.CloseAPIPurchaseIntentWithIdempotency(ctx, user.ID, routeKey, idempotencyKey, requestHash(http.MethodPost, routeKey+":"+input.IntentID, body), input, apiPurchaseIntentCompletionBuilder(toOwnerAPIPurchaseIntentListItemResponse))
	})
}

func (s *Server) handleOwnerAPIPurchaseIntentAction(w http.ResponseWriter, r *http.Request, action string, decodeReason bool, run func(context.Context, auth.User, string, string, []byte, apiintent.ActionInput) (idempotency.Completion, *domain.AppError)) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	var body []byte
	var reason string
	if decodeReason {
		var req apiPurchaseIntentActionRequest
		body, req, appErr = decodeStrictJSON[apiPurchaseIntentActionRequest](r)
		reason = req.Reason
	} else {
		body, _, appErr = decodeStrictJSON[emptyRequest](r)
	}
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	intentID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/owner/api-purchase-intents/{id}/" + action
	completion, appErr := run(r.Context(), user, routeKey, r.Header.Get("Idempotency-Key"), body, apiintent.ActionInput{
		IntentID:        intentID,
		Reason:          reason,
		ExpectedVersion: version,
		RequestID:       requestIDFrom(r),
	})
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleAdminAPIServices(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	pageRequest, appErr := parsePageRequest(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	services, appErr := s.app.AdminAPIServices(r.Context(), user, pageRequest)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePageJSON(w, domain.Page[apiServiceResponse]{
		Items:      toAPIServiceResponses(services.Items),
		NextCursor: services.NextCursor,
	})
}

func (s *Server) handleAdminAPIService(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	service, appErr := s.app.AdminAPIService(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, service.Version)
	writeJSON(w, http.StatusOK, toAPIServiceResponse(service))
}

func (s *Server) handleApproveAPIService(w http.ResponseWriter, r *http.Request) {
	s.handleAdminAPIServiceAction(w, r, "approve")
}

func (s *Server) handleRequestChangesAPIService(w http.ResponseWriter, r *http.Request) {
	s.handleAdminAPIServiceAction(w, r, "request_changes")
}

func (s *Server) handleRejectAPIService(w http.ResponseWriter, r *http.Request) {
	s.handleAdminAPIServiceAction(w, r, "reject")
}

func (s *Server) handleSuspendAPIService(w http.ResponseWriter, r *http.Request) {
	s.handleAdminAPIServiceAction(w, r, "suspend")
}

func (s *Server) handleRestoreAPIService(w http.ResponseWriter, r *http.Request) {
	s.handleAdminAPIServiceAction(w, r, "restore")
}

func (s *Server) handleRemoveAPIService(w http.ResponseWriter, r *http.Request) {
	s.handleAdminAPIServiceAction(w, r, "remove")
}

func (s *Server) handleAdminAPIServiceAction(w http.ResponseWriter, r *http.Request, action string) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[reviewActionRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	serviceID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/admin/api-services/{id}/" + action + ":" + serviceID
	s.withIdempotency(w, r, user.ID, routeKey, body, func() (int, any, string, string, *domain.AppError) {
		service, errApp := s.app.UpdateAPIServiceAdminStatus(r.Context(), user, apimarket.ServiceAdminActionInput{
			ServiceID:       serviceID,
			Action:          action,
			Reason:          req.Reason,
			ExpectedVersion: version,
			RequestID:       requestIDFrom(r),
		})
		if errApp != nil {
			return 0, nil, "", "", errApp
		}
		setETag(w, service.Version)
		return http.StatusOK, toAPIServiceResponse(service), "api_service", service.ID, nil
	})
}

func (s *Server) handleAdminAPIPurchaseIntents(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	intents, appErr := s.app.AdminAPIPurchaseIntents(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toAdminAPIPurchaseIntentListResponses(intents))
}

func (s *Server) handleAdminAPIPurchaseIntent(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	intent, appErr := s.app.AdminAPIPurchaseIntent(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, intent.Version)
	writeJSON(w, http.StatusOK, toAdminAPIPurchaseIntentDetailResponse(intent))
}
func toAppCreateAPIServiceInput(req apiServiceRequest) apimarket.CreateServiceInput {
	accessModes := make([]apimarket.ServiceAccessModeInput, 0, len(req.AccessModes))
	for _, mode := range req.AccessModes {
		accessModes = append(accessModes, apimarket.ServiceAccessModeInput{
			AccessMode: mode.AccessMode,
			PublicNote: mode.PublicNote,
		})
	}
	models := make([]apimarket.ServiceModelInput, 0, len(req.Models))
	for _, model := range req.Models {
		enabled := true
		if model.Enabled != nil {
			enabled = *model.Enabled
		}
		models = append(models, apimarket.ServiceModelInput{
			ModelCatalogID:      model.ModelCatalogID,
			ModelPriceVersionID: model.ModelPriceVersionID,
			MerchantMultiplier:  model.MerchantMultiplier,
			Enabled:             enabled,
		})
	}
	packages := make([]apimarket.ServicePackageInput, 0, len(req.Packages))
	for _, pack := range req.Packages {
		enabled := true
		if pack.Enabled != nil {
			enabled = *pack.Enabled
		}
		packages = append(packages, apimarket.ServicePackageInput{
			ID:              pack.ID,
			Name:            pack.Name,
			PriceCNY:        pack.PriceCNY,
			PanelAllowance:  pack.PanelAllowance,
			DurationDays:    pack.DurationDays,
			StockTotal:      pack.StockTotal,
			Description:     pack.Description,
			Enabled:         enabled,
			SortOrder:       pack.SortOrder,
			ModelCatalogIDs: append([]string(nil), pack.ModelCatalogIDs...),
		})
	}
	return apimarket.CreateServiceInput{
		MerchantProfileID:                req.MerchantProfileID,
		MerchantIdentityMode:             req.MerchantIdentityMode,
		OwnerContactMethodID:             req.OwnerContactMethodID,
		Title:                            req.Title,
		ShortDescription:                 req.ShortDescription,
		SourceURL:                        req.SourceURL,
		DistributionSystem:               req.DistributionSystem,
		BillingMode:                      req.BillingMode,
		DeclaredCNYPerUSDAllowance:       req.DeclaredCNYPerUSDAllowance,
		DeclaredMaxUSDAllowancePerIntent: req.DeclaredMaxUSDAllowancePerIntent,
		AvailableUSDAllowance:            req.AvailableUSDAllowance,
		QuotaExpiresAt:                   req.QuotaExpiresAt,
		MinimumIntentCNY:                 req.MinimumIntentCNY,
		MaximumIntentCNY:                 req.MaximumIntentCNY,
		UsageVisibility:                  req.UsageVisibility,
		PublicAccessNote:                 req.PublicAccessNote,
		MerchantNote:                     req.MerchantNote,
		MerchantSupportNote:              req.MerchantSupportNote,
		AccessModes:                      accessModes,
		Models:                           models,
		Packages:                         packages,
	}
}

func toAppUpdateAPIServiceInput(req apiServiceRequest) apimarket.UpdateServiceInput {
	base := toAppCreateAPIServiceInput(req)
	return apimarket.UpdateServiceInput{
		MerchantProfileID:                base.MerchantProfileID,
		MerchantIdentityMode:             base.MerchantIdentityMode,
		OwnerContactMethodID:             base.OwnerContactMethodID,
		Title:                            base.Title,
		ShortDescription:                 base.ShortDescription,
		SourceURL:                        base.SourceURL,
		DistributionSystem:               base.DistributionSystem,
		BillingMode:                      base.BillingMode,
		DeclaredCNYPerUSDAllowance:       base.DeclaredCNYPerUSDAllowance,
		DeclaredMaxUSDAllowancePerIntent: base.DeclaredMaxUSDAllowancePerIntent,
		AvailableUSDAllowance:            base.AvailableUSDAllowance,
		QuotaExpiresAt:                   base.QuotaExpiresAt,
		MinimumIntentCNY:                 base.MinimumIntentCNY,
		MaximumIntentCNY:                 base.MaximumIntentCNY,
		UsageVisibility:                  base.UsageVisibility,
		PublicAccessNote:                 base.PublicAccessNote,
		MerchantNote:                     base.MerchantNote,
		MerchantSupportNote:              base.MerchantSupportNote,
		AccessModes:                      base.AccessModes,
		Models:                           base.Models,
		Packages:                         base.Packages,
	}
}

func toAppPaymentOptionInputs(requests []apiServicePaymentOptionRequest) []apimarket.PaymentOptionInput {
	items := make([]apimarket.PaymentOptionInput, 0, len(requests))
	for _, req := range requests {
		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}
		items = append(items, apimarket.PaymentOptionInput{
			PaymentMethod:        req.PaymentMethod,
			Enabled:              enabled,
			PaymentInstructions:  req.PaymentInstructions,
			PaymentQRCodeDataURL: req.PaymentQRCodeDataURL,
		})
	}
	return items
}

func toAPIServiceResponses(services []apimarket.Service) []apiServiceResponse {
	items := make([]apiServiceResponse, 0, len(services))
	for _, service := range services {
		items = append(items, toAPIServiceResponse(service))
	}
	return items
}

func toPublicAPIServiceResponses(services []apimarket.Service) []publicAPIServiceResponse {
	items := make([]publicAPIServiceResponse, 0, len(services))
	for _, service := range services {
		items = append(items, toPublicAPIServiceResponse(service))
	}
	return items
}

func toPublicAPIServiceResponse(service apimarket.Service) publicAPIServiceResponse {
	return publicAPIServiceResponse{
		ID:                               service.ID,
		MerchantIdentityMode:             service.MerchantIdentityMode,
		MerchantDisplayName:              service.MerchantDisplayName,
		MerchantProfileSlug:              service.MerchantProfileSlug,
		MerchantAvatarURL:                service.MerchantAvatarURL,
		Title:                            service.Title,
		ShortDescription:                 service.ShortDescription,
		SourceURL:                        service.SourceURL,
		DistributionSystem:               service.DistributionSystem,
		BillingMode:                      service.BillingMode,
		DeclaredCNYPerUSDAllowance:       service.DeclaredCNYPerUSDAllowance,
		DeclaredMaxUSDAllowancePerIntent: service.DeclaredMaxUSDAllowancePerIntent,
		AvailableUSDAllowance:            service.AvailableUSDAllowance,
		QuotaExpiresAt:                   formatOptionalTime(service.QuotaExpiresAt),
		MinimumIntentCNY:                 service.MinimumIntentCNY,
		MaximumIntentCNY:                 service.MaximumIntentCNY,
		UsageVisibility:                  service.UsageVisibility,
		PublicAccessNote:                 service.PublicAccessNote,
		MerchantSupportNote:              service.MerchantSupportNote,
		AcceptingOrders:                  service.AcceptingOrders,
		PaymentWindowMinutes:             service.PaymentWindowMinutes,
		AcceptedPaymentMethods:           enabledPaymentMethods(service.PaymentOptions),
		IsOrderable:                      service.IsOrderable,
		OrderableReasons:                 service.OrderableReasons,
		AccessModes:                      toAPIServiceAccessModeResponses(service.AccessModes),
		Models:                           toAPIServiceModelResponses(service.Models),
		Packages:                         toAPIServicePackageResponses(service.Packages),
		Completed30d:                     service.Completed30d,
		UnresolvedDisputes:               service.UnresolvedDisputes,
		ResponseMedianMinutes:            service.ResponseMedianMinutes,
		Version:                          service.Version,
		CreatedAt:                        service.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:                        service.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toAPIServiceResponse(service apimarket.Service) apiServiceResponse {
	var approvedAt *string
	if service.ApprovedAt != nil {
		formatted := service.ApprovedAt.UTC().Format(time.RFC3339)
		approvedAt = &formatted
	}
	return apiServiceResponse{
		ID:                               service.ID,
		OwnerUserID:                      service.OwnerUserID,
		MerchantProfileID:                service.MerchantProfileID,
		MerchantIdentityMode:             service.MerchantIdentityMode,
		MerchantDisplayName:              service.MerchantDisplayName,
		MerchantProfileSlug:              service.MerchantProfileSlug,
		MerchantAvatarURL:                service.MerchantAvatarURL,
		OwnerContactMethodID:             service.OwnerContactMethodID,
		Title:                            service.Title,
		ShortDescription:                 service.ShortDescription,
		SourceURL:                        service.SourceURL,
		DistributionSystem:               service.DistributionSystem,
		BillingMode:                      service.BillingMode,
		DeclaredCNYPerUSDAllowance:       service.DeclaredCNYPerUSDAllowance,
		DeclaredMaxUSDAllowancePerIntent: service.DeclaredMaxUSDAllowancePerIntent,
		AvailableUSDAllowance:            service.AvailableUSDAllowance,
		QuotaExpiresAt:                   formatOptionalTime(service.QuotaExpiresAt),
		MinimumIntentCNY:                 service.MinimumIntentCNY,
		MaximumIntentCNY:                 service.MaximumIntentCNY,
		UsageVisibility:                  service.UsageVisibility,
		PublicAccessNote:                 service.PublicAccessNote,
		MerchantNote:                     service.MerchantNote,
		MerchantSupportNote:              service.MerchantSupportNote,
		AcceptingOrders:                  service.AcceptingOrders,
		PaymentWindowMinutes:             service.PaymentWindowMinutes,
		AcceptedPaymentMethods:           enabledPaymentMethods(service.PaymentOptions),
		PaymentOptions:                   toAPIServicePaymentOptionResponses(service.PaymentOptions, true),
		IsOrderable:                      service.IsOrderable,
		OrderableReasons:                 service.OrderableReasons,
		ReviewStatus:                     service.ReviewStatus,
		PublicationStatus:                service.PublicationStatus,
		ModerationStatus:                 service.ModerationStatus,
		ApprovedByAdminID:                service.ApprovedByAdminID,
		ApprovedAt:                       approvedAt,
		ModerationReason:                 service.ModerationReason,
		AccessModes:                      toAPIServiceAccessModeResponses(service.AccessModes),
		Models:                           toAPIServiceModelResponses(service.Models),
		Packages:                         toAPIServicePackageResponses(service.Packages),
		Version:                          service.Version,
		CreatedAt:                        service.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:                        service.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toAPIServiceAccessModeResponses(modes []apimarket.ServiceAccessMode) []apiServiceAccessModeResponse {
	items := make([]apiServiceAccessModeResponse, 0, len(modes))
	for _, mode := range modes {
		items = append(items, apiServiceAccessModeResponse{
			AccessMode: mode.AccessMode,
			PublicNote: mode.PublicNote,
		})
	}
	return items
}

func toAPIServiceModelResponses(models []apimarket.ServiceModel) []apiServiceModelResponse {
	items := make([]apiServiceModelResponse, 0, len(models))
	for _, model := range models {
		items = append(items, apiServiceModelResponse{
			ID:                                  model.ID,
			ModelCatalogID:                      model.ModelCatalogID,
			ModelPriceVersionID:                 model.ModelPriceVersionID,
			ModelNameSnapshot:                   model.ModelNameSnapshot,
			ProviderSnapshot:                    model.ProviderSnapshot,
			CapabilitiesSnapshot:                model.CapabilitiesSnapshot,
			MerchantMultiplier:                  model.MerchantMultiplier,
			EffectiveInputPricePerMillion:       model.EffectiveInputPricePerMillion,
			EffectiveCachedInputPricePerMillion: model.EffectiveCachedInputPricePerMillion,
			EffectiveOutputPricePerMillion:      model.EffectiveOutputPricePerMillion,
			Enabled:                             model.Enabled,
		})
	}
	return items
}

func toAPIServicePackageResponses(packages []apimarket.ServicePackage) []apiServicePackageResponse {
	items := make([]apiServicePackageResponse, 0, len(packages))
	for _, pack := range packages {
		models := make([]apiServicePackageModelResponse, 0, len(pack.Models))
		for _, model := range pack.Models {
			models = append(models, apiServicePackageModelResponse{
				ServiceModelID:      model.ServiceModelID,
				ModelCatalogID:      model.ModelCatalogID,
				ModelPriceVersionID: model.ModelPriceVersionID,
				ModelNameSnapshot:   model.ModelNameSnapshot,
				ProviderSnapshot:    model.ProviderSnapshot,
				MerchantMultiplier:  model.MerchantMultiplier,
			})
		}
		items = append(items, apiServicePackageResponse{
			ID:             pack.ID,
			Name:           pack.Name,
			PriceCNY:       pack.PriceCNY,
			PanelAllowance: pack.PanelAllowance,
			DurationDays:   pack.DurationDays,
			StockTotal:     pack.StockTotal,
			StockAvailable: pack.StockAvailable,
			Description:    pack.Description,
			Enabled:        pack.Enabled,
			SortOrder:      pack.SortOrder,
			Models:         models,
		})
	}
	return items
}

func enabledPaymentMethods(options []apimarket.PaymentOption) []string {
	items := []string{}
	for _, option := range options {
		if option.Enabled && apimarket.IsSupportedPaymentMethod(option.PaymentMethod) {
			items = append(items, option.PaymentMethod)
		}
	}
	return items
}

func toAPIServicePaymentOptionResponses(options []apimarket.PaymentOption, includeInstructions bool) []apiServicePaymentOptionResponse {
	items := make([]apiServicePaymentOptionResponse, 0, len(options))
	for _, option := range options {
		if !apimarket.IsSupportedPaymentMethod(option.PaymentMethod) {
			continue
		}
		response := apiServicePaymentOptionResponse{
			ID:            option.ID,
			PaymentMethod: option.PaymentMethod,
			Enabled:       option.Enabled,
			Version:       option.Version,
		}
		if includeInstructions {
			response.PaymentInstructions = option.PaymentInstructions
			response.PaymentQRCodeDataURL = option.PaymentQRCodeDataURL
		}
		items = append(items, response)
	}
	return items
}

func toBuyerAPIPurchaseIntentListResponses(intents []apiintent.Intent) []apiPurchaseIntentListItemResponse {
	items := make([]apiPurchaseIntentListItemResponse, 0, len(intents))
	for _, intent := range intents {
		items = append(items, toBuyerAPIPurchaseIntentListItemResponse(intent))
	}
	return items
}

func toOwnerAPIPurchaseIntentListResponses(intents []apiintent.Intent) []ownerAPIPurchaseIntentListItemResponse {
	items := make([]ownerAPIPurchaseIntentListItemResponse, 0, len(intents))
	for _, intent := range intents {
		items = append(items, toOwnerAPIPurchaseIntentListItemResponse(intent))
	}
	return items
}

func toAdminAPIPurchaseIntentListResponses(intents []apiintent.Intent) []adminAPIPurchaseIntentDetailResponse {
	items := make([]adminAPIPurchaseIntentDetailResponse, 0, len(intents))
	for _, intent := range intents {
		items = append(items, toAdminAPIPurchaseIntentDetailResponse(intent))
	}
	return items
}

func toAPIPurchaseIntentCoreResponse(intent apiintent.Intent) apiPurchaseIntentCoreResponse {
	var contactedAt *string
	if intent.ContactedAt != nil {
		formatted := intent.ContactedAt.UTC().Format(time.RFC3339)
		contactedAt = &formatted
	}
	var buyerCancelledAt *string
	if intent.BuyerCancelledAt != nil {
		formatted := intent.BuyerCancelledAt.UTC().Format(time.RFC3339)
		buyerCancelledAt = &formatted
	}
	var ownerClosedAt *string
	if intent.OwnerClosedAt != nil {
		formatted := intent.OwnerClosedAt.UTC().Format(time.RFC3339)
		ownerClosedAt = &formatted
	}
	return apiPurchaseIntentCoreResponse{
		ID:                                       intent.ID,
		APIServiceID:                             intent.APIServiceID,
		Status:                                   intent.Status,
		RequestedCNYAmount:                       intent.RequestedCNYAmount,
		RequestedUSDAllowance:                    intent.RequestedUSDAllowance,
		SelectedAccessMode:                       intent.SelectedAccessMode,
		SelectedPackageID:                        intent.SelectedPackageID,
		SelectedPackageSnapshot:                  intent.SelectedPackageSnapshot,
		ServiceVersionSnapshot:                   intent.ServiceVersionSnapshot,
		ServiceTitleSnapshot:                     intent.ServiceTitleSnapshot,
		DistributionSystemSnapshot:               intent.DistributionSystemSnapshot,
		BillingModeSnapshot:                      intent.BillingModeSnapshot,
		DeclaredCNYPerUSDAllowanceSnapshot:       intent.DeclaredCNYPerUSDAllowanceSnapshot,
		DeclaredMaxUSDAllowancePerIntentSnapshot: intent.DeclaredMaxUSDAllowancePerIntentSnapshot,
		MinimumIntentCNYSnapshot:                 intent.MinimumIntentCNYSnapshot,
		MaximumIntentCNYSnapshot:                 intent.MaximumIntentCNYSnapshot,
		PricingSnapshot:                          intent.PricingSnapshot,
		BuyerNote:                                intent.BuyerNote,
		ContactedAt:                              contactedAt,
		BuyerCancelledAt:                         buyerCancelledAt,
		BuyerCancelReason:                        intent.BuyerCancelReason,
		OwnerClosedAt:                            ownerClosedAt,
		OwnerCloseReason:                         intent.OwnerCloseReason,
		Version:                                  intent.Version,
		CreatedAt:                                intent.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:                                intent.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toBuyerAPIPurchaseIntentListItemResponse(intent apiintent.Intent) apiPurchaseIntentListItemResponse {
	return apiPurchaseIntentListItemResponse{
		apiPurchaseIntentCoreResponse: toAPIPurchaseIntentCoreResponse(intent),
	}
}

func toOwnerAPIPurchaseIntentListItemResponse(intent apiintent.Intent) ownerAPIPurchaseIntentListItemResponse {
	return ownerAPIPurchaseIntentListItemResponse{
		apiPurchaseIntentCoreResponse: toAPIPurchaseIntentCoreResponse(intent),
		BuyerUserID:                   intent.BuyerUserID,
		BuyerContactMethodID:          intent.BuyerContactMethodID,
	}
}

func toAdminAPIPurchaseIntentDetailResponse(intent apiintent.Intent) adminAPIPurchaseIntentDetailResponse {
	return adminAPIPurchaseIntentDetailResponse{
		apiPurchaseIntentCoreResponse: toAPIPurchaseIntentCoreResponse(intent),
		BuyerUserID:                   intent.BuyerUserID,
		OwnerUserID:                   intent.OwnerUserID,
		BuyerContactMethodID:          intent.BuyerContactMethodID,
		OwnerContactMethodID:          intent.OwnerContactMethodID,
	}
}

func toCreateAPIPurchaseIntentResponse(intent apiintent.Intent) createAPIPurchaseIntentResponse {
	return createAPIPurchaseIntentResponse{
		apiPurchaseIntentCoreResponse: toAPIPurchaseIntentCoreResponse(intent),
		MerchantContact:               toContactDisclosureDTO(intent.MerchantContact),
	}
}

func toBuyerAPIPurchaseIntentDetailResponse(intent apiintent.Intent) buyerAPIPurchaseIntentDetailResponse {
	return buyerAPIPurchaseIntentDetailResponse{
		apiPurchaseIntentCoreResponse: toAPIPurchaseIntentCoreResponse(intent),
		MerchantContact:               toContactDisclosureDTO(intent.MerchantContact),
	}
}

func toOwnerAPIPurchaseIntentDetailResponse(intent apiintent.Intent) ownerAPIPurchaseIntentDetailResponse {
	return ownerAPIPurchaseIntentDetailResponse{
		apiPurchaseIntentCoreResponse: toAPIPurchaseIntentCoreResponse(intent),
		BuyerUserID:                   intent.BuyerUserID,
		BuyerContactMethodID:          intent.BuyerContactMethodID,
		BuyerContact:                  toContactDisclosureDTO(intent.BuyerContact),
	}
}

func apiPurchaseIntentCompletionBuilder[T any](mapper func(apiintent.Intent) T) apiintent.CompletionBuilder {
	return func(intent apiintent.Intent) (idempotency.Completion, *domain.AppError) {
		responseBody, marshalErr := json.Marshal(mapper(intent))
		if marshalErr != nil {
			return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
		}
		return idempotency.Completion{
			Status:       http.StatusOK,
			ContentType:  "application/json; charset=utf-8",
			Body:         responseBody,
			ResourceType: "api_purchase_intent",
			ResourceID:   intent.ID,
			Headers: map[string]string{
				"ETag": `"` + strconv.FormatInt(intent.Version, 10) + `"`,
			},
		}, nil
	}
}

func toContactDisclosureDTO(item *contact.ContactItemView) *contactDisclosureDTO {
	if item == nil {
		return nil
	}
	return &contactDisclosureDTO{
		Side:        item.Side,
		Type:        item.Type,
		Label:       item.Label,
		Value:       item.Value,
		MaskedValue: item.MaskedValue,
	}
}
