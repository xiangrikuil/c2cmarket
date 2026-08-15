package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/report"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type createAPIOrderRequest struct {
	PaymentMethod string `json:"paymentMethod"`
}

type apiOrderPaymentRequest struct {
	PaymentSummary string `json:"paymentSummary"`
}

type apiOrderDeliveryRequest struct {
	DeliveryKind  string `json:"deliveryKind"`
	APIBaseURL    string `json:"apiBaseUrl"`
	APIKey        string `json:"apiKey"`
	PanelLoginURL string `json:"panelLoginUrl"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	Instructions  string `json:"instructions"`
}

type apiOrderReasonRequest struct {
	Reason string `json:"reason"`
}

type apiOrderDisputeRequest struct {
	IssueCode           string   `json:"issueCode"`
	RequestedResolution string   `json:"requestedResolution"`
	RequestedAmountCNY  string   `json:"requestedAmountCny"`
	IssueOccurredAt     string   `json:"issueOccurredAt"`
	Reason              string   `json:"reason"`
	EvidenceAssetIDs    []string `json:"evidenceAssetIds"`
}

type apiOrderPaymentIssueRequest struct {
	Reason string `json:"reason"`
	Note   string `json:"note"`
}

type apiOrderLatePaymentRequest struct {
	Status string `json:"status"`
	Note   string `json:"note"`
}

type apiOrderResponse struct {
	ID                            string                              `json:"id"`
	OrderNo                       string                              `json:"orderNo"`
	PurchaseKind                  string                              `json:"purchaseKind"`
	APIPurchaseIntentID           string                              `json:"apiPurchaseIntentId"`
	APIServiceID                  string                              `json:"apiServiceId"`
	BuyerUserID                   string                              `json:"buyerUserId,omitempty"`
	SellerUserID                  string                              `json:"sellerUserId,omitempty"`
	BuyerReputation               *reputationSummaryResponse          `json:"buyerReputation"`
	SellerReputation              *reputationSummaryResponse          `json:"sellerReputation"`
	Status                        string                              `json:"status"`
	DisputeStatus                 string                              `json:"disputeStatus"`
	DisputeCaseID                 string                              `json:"disputeCaseId,omitempty"`
	LatestDisputeCaseID           string                              `json:"latestDisputeCaseId,omitempty"`
	HasDisputeHistory             bool                                `json:"hasDisputeHistory"`
	DisputeNextActor              string                              `json:"disputeNextActor"`
	DisputeDueAt                  *string                             `json:"disputeDueAt,omitempty"`
	DisputeNeedsAction            bool                                `json:"disputeNeedsAction"`
	DisputeResponseOverdue        bool                                `json:"disputeResponseOverdue"`
	DisputeAvailableActions       []string                            `json:"disputeAvailableActions"`
	ActiveRemedyAction            string                              `json:"activeRemedyAction,omitempty"`
	ActiveRemedySource            string                              `json:"activeRemedySource,omitempty"`
	ServiceTitleSnapshot          string                              `json:"serviceTitleSnapshot"`
	ServiceVersionSnapshot        int64                               `json:"serviceVersionSnapshot"`
	BillingModeSnapshot           string                              `json:"billingModeSnapshot"`
	SelectedPackageID             string                              `json:"selectedPackageId,omitempty"`
	SelectedPackageSnapshot       string                              `json:"selectedPackageSnapshot,omitempty"`
	QuoteVersionSnapshot          int64                               `json:"quoteVersionSnapshot,omitempty"`
	RequestedUSDAllowanceSnapshot string                              `json:"requestedUsdAllowanceSnapshot,omitempty"`
	CNYPerUSDAllowanceSnapshot    string                              `json:"cnyPerUsdAllowanceSnapshot,omitempty"`
	PricingSnapshot               string                              `json:"pricingSnapshot"`
	ProbeConnectionIDSnapshot     string                              `json:"probeConnectionIdSnapshot,omitempty"`
	APIBaseURLSnapshot            string                              `json:"apiBaseUrlSnapshot,omitempty"`
	QuotaUsagePolicySnapshot      apiQuotaUsagePolicyResponse         `json:"quotaUsagePolicySnapshot"`
	PromptAuditEnabledSnapshot    *bool                               `json:"promptAuditEnabledSnapshot"`
	PackageStockReserved          bool                                `json:"packageStockReserved"`
	PackageExpiresAt              *string                             `json:"packageExpiresAt,omitempty"`
	APIQuotaBatchID               string                              `json:"apiQuotaBatchId,omitempty"`
	APIQuotaOfferID               string                              `json:"apiQuotaOfferId,omitempty"`
	APIQuotaSaleRoundID           string                              `json:"apiQuotaSaleRoundId,omitempty"`
	QuotaOfferNameSnapshot        string                              `json:"quotaOfferNameSnapshot,omitempty"`
	QuotaUSDAllowanceSnapshot     string                              `json:"quotaUsdAllowanceSnapshot,omitempty"`
	QuotaPriceCNYSnapshot         string                              `json:"quotaPriceCnySnapshot,omitempty"`
	QuotaCNYPerUSDSnapshot        string                              `json:"quotaCnyPerUsdSnapshot,omitempty"`
	QuotaModelMultiplierSnapshot  string                              `json:"quotaModelMultiplierSnapshot,omitempty"`
	QuotaSaleCutoffAtSnapshot     *string                             `json:"quotaSaleCutoffAtSnapshot,omitempty"`
	QuotaExpiresAtSnapshot        *string                             `json:"quotaExpiresAtSnapshot,omitempty"`
	QuotaSaleModeSnapshot         string                              `json:"quotaSaleModeSnapshot,omitempty"`
	QuotaRoundStartsAtSnapshot    *string                             `json:"quotaRoundStartsAtSnapshot,omitempty"`
	QuotaRoundEndsAtSnapshot      *string                             `json:"quotaRoundEndsAtSnapshot,omitempty"`
	QuotaDistributionSnapshot     string                              `json:"quotaDistributionSystemSnapshot,omitempty"`
	QuotaTTFTBandSnapshot         string                              `json:"quotaTtftBandSnapshot,omitempty"`
	QuotaDeclaredMaxConcurrency   int                                 `json:"quotaDeclaredMaxConcurrencySnapshot,omitempty"`
	QuotaPerformanceConfirmedAt   *string                             `json:"quotaPerformanceConfirmedAtSnapshot,omitempty"`
	QuotaPerformanceUnverified    bool                                `json:"quotaPerformanceUnverifiedSnapshot,omitempty"`
	QuotaDeliveryETAMinutes       int                                 `json:"quotaDeliveryEtaMinutesSnapshot,omitempty"`
	QuotaDeliveryMode             string                              `json:"quotaDeliveryModeSnapshot,omitempty"`
	Amount                        string                              `json:"amount"`
	Currency                      string                              `json:"currency"`
	SelectedPaymentMethod         string                              `json:"selectedPaymentMethod"`
	PaymentWindowMinutesSnapshot  int                                 `json:"paymentWindowMinutesSnapshot"`
	PaymentExpiresAt              string                              `json:"paymentExpiresAt"`
	PaymentSummary                string                              `json:"paymentSummary,omitempty"`
	PaymentSubmittedAt            *string                             `json:"paymentSubmittedAt,omitempty"`
	MerchantConfirmDueAt          *string                             `json:"merchantConfirmDueAt,omitempty"`
	MerchantConfirmOverdue        bool                                `json:"merchantConfirmOverdue"`
	MerchantConfirmOverdueAt      *string                             `json:"merchantConfirmOverdueAt,omitempty"`
	PaymentIssueReason            string                              `json:"paymentIssueReason,omitempty"`
	PaymentIssueNote              string                              `json:"paymentIssueNote,omitempty"`
	PaymentIssueReportedAt        *string                             `json:"paymentIssueReportedAt,omitempty"`
	PaidConfirmedAt               *string                             `json:"paidConfirmedAt,omitempty"`
	DeliveryDueAt                 *string                             `json:"deliveryDueAt,omitempty"`
	DeliveryOverdue               bool                                `json:"deliveryOverdue"`
	DeliveryOverdueAt             *string                             `json:"deliveryOverdueAt,omitempty"`
	DeliveryDueRemindedAt         *string                             `json:"deliveryDueRemindedAt,omitempty"`
	DeliveryNote                  string                              `json:"deliveryNote,omitempty"`
	DeliverySubmittedAt           *string                             `json:"deliverySubmittedAt,omitempty"`
	DeliveryReviewExpiresAt       *string                             `json:"deliveryReviewExpiresAt,omitempty"`
	DeliveryCredential            *apiOrderDeliveryCredentialResponse `json:"deliveryCredential,omitempty"`
	CommercialOutcome             string                              `json:"commercialOutcome"`
	CommercialOutcomeUpdatedAt    *string                             `json:"commercialOutcomeUpdatedAt,omitempty"`
	QuotaValidityIssueAt          *string                             `json:"quotaValidityIssueAt,omitempty"`
	QuotaValidityIssueReason      string                              `json:"quotaValidityIssueReason,omitempty"`
	CompletionSource              string                              `json:"completionSource,omitempty"`
	CompletedAt                   *string                             `json:"completedAt,omitempty"`
	CancelledAt                   *string                             `json:"cancelledAt,omitempty"`
	CancelReason                  string                              `json:"cancelReason,omitempty"`
	LatePaymentStatus             string                              `json:"latePaymentStatus,omitempty"`
	LatePaymentReportedAt         *string                             `json:"latePaymentReportedAt,omitempty"`
	LatePaymentNote               string                              `json:"latePaymentNote,omitempty"`
	LatePaymentResolvedAt         *string                             `json:"latePaymentResolvedAt,omitempty"`
	CanReportLatePayment          bool                                `json:"canReportLatePayment"`
	AfterSalesExpiresAt           *string                             `json:"afterSalesExpiresAt,omitempty"`
	CanOpenDispute                bool                                `json:"canOpenDispute"`
	DisputeEligibilityReason      string                              `json:"disputeEligibilityReason"`
	Version                       int64                               `json:"version"`
	CreatedAt                     string                              `json:"createdAt"`
	UpdatedAt                     string                              `json:"updatedAt"`
	CatalogRiskHold               *apiOrderCatalogRiskHoldResponse    `json:"catalogRiskHold,omitempty"`
}

type apiOrderCatalogRiskHoldResponse struct {
	ID             string  `json:"id"`
	SourceType     string  `json:"sourceType"`
	SourceID       string  `json:"sourceId"`
	Status         string  `json:"status"`
	Reason         string  `json:"reason"`
	CreatedAt      string  `json:"createdAt"`
	ResolvedBy     string  `json:"resolvedBy,omitempty"`
	ResolvedAt     *string `json:"resolvedAt,omitempty"`
	ResolutionNote string  `json:"resolutionNote,omitempty"`
	Version        int64   `json:"version"`
}

type apiOrderCatalogRiskHoldRequest struct {
	ResolutionNote string `json:"resolutionNote"`
}

type apiOrderPaymentInstructionsResponse struct {
	OrderID              string `json:"orderId"`
	PaymentMethod        string `json:"paymentMethod"`
	PaymentInstructions  string `json:"paymentInstructions"`
	PaymentQRCodeDataURL string `json:"paymentQrCodeDataUrl,omitempty"`
	PaymentExpiresAt     string `json:"paymentExpiresAt"`
}

type apiOrderDeliveryCredentialResponse struct {
	DeliveryKind  string  `json:"deliveryKind"`
	APIBaseURL    string  `json:"apiBaseUrl,omitempty"`
	APIKey        string  `json:"apiKey,omitempty"`
	PanelLoginURL string  `json:"panelLoginUrl,omitempty"`
	Username      string  `json:"username,omitempty"`
	Password      string  `json:"password,omitempty"`
	Instructions  string  `json:"instructions,omitempty"`
	SubmittedAt   string  `json:"submittedAt"`
	DestroyedAt   *string `json:"destroyedAt,omitempty"`
	DestroyReason string  `json:"destroyReason,omitempty"`
}

func (s *Server) handleCreateAPIOrder(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if !requireCapability(w, r, user, auth.CapabilityAPIOrderCreate) {
		return
	}
	body, req, appErr := decodeStrictJSON[createAPIOrderRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	intentID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/me/api-purchase-intents/{id}/orders"
	completion, appErr := s.app.CreateAPIOrderWithIdempotency(
		r.Context(),
		user,
		routeKey,
		r.Header.Get("Idempotency-Key"),
		requestHash(r.Method, routeKey+":"+intentID, body),
		apiorder.ActionInput{},
		apiorder.CreateInput{
			IntentID:      intentID,
			PaymentMethod: req.PaymentMethod,
			RequestID:     requestIDFrom(r),
		},
		apiOrderCreateCompletionBuilder(false),
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeNoStoreIdempotencyCompletion(w, completion)
}

func (s *Server) handleMyAPIOrders(w http.ResponseWriter, r *http.Request) {
	actor, appErr := s.requireBusinessActor(r, true, false)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if appErr := validateAPIOrderListQuery(r); appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	orders, appErr := s.apiOrderContinuity.APIOrdersForActor(r.Context(), actor, "buyer")
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toAPIOrderResponses(filterAPIOrders(r, orders), false))
}

func (s *Server) handleAdminAPIOrder(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	order, appErr := s.app.AdminAPIOrder(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, order.Version)
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, http.StatusOK, toAdminAPIOrderResponse(order))
}

func (s *Server) handleAdminAPIOrders(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	pageRequest, appErr := parsePageRequest(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	filter, appErr := parseAdminAPIOrderFilter(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	page, appErr := s.app.AdminAPIOrders(r.Context(), user, filter, pageRequest)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePageJSON(w, domain.Page[apiOrderResponse]{
		Items:      toAdminAPIOrderResponses(page.Items),
		NextCursor: page.NextCursor,
	})
}

func (s *Server) handleResolveAPIOrderCatalogRiskHold(w http.ResponseWriter, r *http.Request, resolution string) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, request, appErr := decodeStrictJSON[apiOrderCatalogRiskHoldRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	orderID := chi.URLParam(r, "id")
	routeKey := "POST " + chi.RouteContext(r.Context()).RoutePattern() + ":" + orderID
	completion, appErr := s.app.ResolveAPIOrderCatalogRiskHoldWithIdempotency(
		r.Context(), user, routeKey, r.Header.Get("Idempotency-Key"), requestHash(r.Method, routeKey, body),
		apiorder.CatalogRiskHoldActionInput{OrderID: orderID, Resolution: resolution, ResolutionNote: request.ResolutionNote,
			ExpectedVersion: version, RequestID: requestIDFrom(r)}, apiOrderCompletionBuilder(false),
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func parseAdminAPIOrderFilter(r *http.Request) (apiorder.AdminOrderFilter, *domain.AppError) {
	values := r.URL.Query()
	filter := apiorder.AdminOrderFilter{
		Query:        strings.TrimSpace(values.Get("q")),
		Statuses:     queryValues(r, "statuses"),
		DateRange:    strings.TrimSpace(values.Get("dateRange")),
		BuyerUserID:  strings.TrimSpace(values.Get("buyerId")),
		SellerUserID: strings.TrimSpace(values.Get("sellerId")),
		APIServiceID: strings.TrimSpace(values.Get("serviceId")),
		Dispute:      strings.TrimSpace(values.Get("dispute")),
		Sort:         strings.TrimSpace(values.Get("sort")),
	}
	for _, status := range filter.Statuses {
		if !apiorder.IsStatus(status) {
			return apiorder.AdminOrderFilter{}, invalidAdminAPIOrderQuery("statuses", "订单状态筛选无效。")
		}
	}
	if !apiorder.IsAdminOrderDateRange(filter.DateRange) {
		return apiorder.AdminOrderFilter{}, invalidAdminAPIOrderQuery("dateRange", "创建时间筛选无效。")
	}
	if !apiorder.IsAdminOrderDispute(filter.Dispute) {
		return apiorder.AdminOrderFilter{}, invalidAdminAPIOrderQuery("dispute", "纠纷筛选无效。")
	}
	if !apiorder.IsAdminOrderSort(filter.Sort) {
		return apiorder.AdminOrderFilter{}, invalidAdminAPIOrderQuery("sort", "订单排序无效。")
	}
	for _, item := range []struct {
		field string
		value string
	}{
		{field: "buyerId", value: filter.BuyerUserID},
		{field: "sellerId", value: filter.SellerUserID},
		{field: "serviceId", value: filter.APIServiceID},
	} {
		if item.value == "" {
			continue
		}
		if _, err := uuid.Parse(item.value); err != nil {
			return apiorder.AdminOrderFilter{}, invalidAdminAPIOrderQuery(item.field, item.field+" 必须是有效 UUID。")
		}
	}
	var ok bool
	filter.MinAmount, ok = apiorder.NormalizeAdminOrderAmount(values.Get("minAmount"))
	if !ok {
		return apiorder.AdminOrderFilter{}, invalidAdminAPIOrderQuery("minAmount", "最小金额必须是非负十进制数。")
	}
	filter.MaxAmount, ok = apiorder.NormalizeAdminOrderAmount(values.Get("maxAmount"))
	if !ok {
		return apiorder.AdminOrderFilter{}, invalidAdminAPIOrderQuery("maxAmount", "最大金额必须是非负十进制数。")
	}
	if filter.MinAmount != "" && filter.MaxAmount != "" && apiorder.CompareAdminOrderAmounts(filter.MinAmount, filter.MaxAmount) > 0 {
		return apiorder.AdminOrderFilter{}, invalidAdminAPIOrderQuery("maxAmount", "最大金额不能小于最小金额。")
	}
	return filter, nil
}

func invalidAdminAPIOrderQuery(field, detail string) *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "API order query invalid", detail, field, "invalid", detail)
}

func (s *Server) handleMyAPIOrder(w http.ResponseWriter, r *http.Request) {
	actor, appErr := s.requireBusinessActor(r, true, false)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	order, appErr := s.apiOrderContinuity.APIOrderForActor(r.Context(), actor, chi.URLParam(r, "id"), "buyer")
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, order.Version)
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, http.StatusOK, toAPIOrderResponse(order, false, true))
}

func (s *Server) handleReadAPIOrderPaymentInstructions(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if _, appErr := decodeStrictJSONOnly[emptyRequest](r); appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	view, appErr := s.app.ReadAPIOrderPaymentInstructions(r.Context(), user, chi.URLParam(r, "id"), requestIDFrom(r))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, http.StatusOK, apiOrderPaymentInstructionsResponse{
		OrderID:              view.OrderID,
		PaymentMethod:        view.PaymentMethod,
		PaymentInstructions:  view.PaymentInstructions,
		PaymentQRCodeDataURL: view.PaymentQRCodeDataURL,
		PaymentExpiresAt:     view.PaymentExpiresAt.UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleSubmitAPIOrderPayment(w http.ResponseWriter, r *http.Request) {
	s.handleBuyerAPIOrderAction(w, r, "submit-payment", func(ctx context.Context, user auth.User, routeKey, key string, body []byte, input apiorder.ActionInput) (idempotency.Completion, *domain.AppError) {
		return s.app.SubmitAPIOrderPaymentWithIdempotency(ctx, user.ID, routeKey, key, requestHash(http.MethodPost, routeKey+":"+input.OrderID, body), input, apiOrderCompletionBuilder(false))
	})
}

func (s *Server) handleCancelAPIOrder(w http.ResponseWriter, r *http.Request) {
	s.handleBuyerAPIOrderAction(w, r, "cancel", func(ctx context.Context, user auth.User, routeKey, key string, body []byte, input apiorder.ActionInput) (idempotency.Completion, *domain.AppError) {
		return s.app.CancelAPIOrderWithIdempotency(ctx, user.ID, routeKey, key, requestHash(http.MethodPost, routeKey+":"+input.OrderID, body), input, apiOrderCompletionBuilder(false))
	})
}

func (s *Server) handleConfirmAPIOrderComplete(w http.ResponseWriter, r *http.Request) {
	s.handleContinuousAPIOrderAction(w, r, "buyer", "confirm-complete", func(ctx context.Context, actor auth.BusinessActor, routeKey, key string, body []byte, input apiorder.ActionInput) (idempotency.Completion, *domain.AppError) {
		return s.apiOrderContinuity.ConfirmAPIOrderCompleteForActorWithIdempotency(ctx, actor, routeKey, key, requestHash(http.MethodPost, routeKey+":"+input.OrderID, body), input, apiOrderCompletionBuilder(false))
	})
}

func (s *Server) handleOpenAPIOrderDispute(w http.ResponseWriter, r *http.Request) {
	s.handleContinuousAPIOrderAction(w, r, "buyer", "dispute", func(ctx context.Context, actor auth.BusinessActor, routeKey, key string, body []byte, input apiorder.ActionInput) (idempotency.Completion, *domain.AppError) {
		return s.apiOrderContinuity.OpenAPIOrderDisputeForActorWithIdempotency(ctx, actor, routeKey, key, requestHash(http.MethodPost, routeKey+":"+input.OrderID, body), input, apiOrderCompletionBuilder(false))
	})
}

func (s *Server) handleReportLateAPIOrderPayment(w http.ResponseWriter, r *http.Request) {
	s.handleBuyerAPIOrderAction(w, r, "report-late-payment", func(ctx context.Context, user auth.User, routeKey, key string, body []byte, input apiorder.ActionInput) (idempotency.Completion, *domain.AppError) {
		return s.app.ReportLateAPIOrderPaymentWithIdempotency(ctx, user.ID, routeKey, key, requestHash(http.MethodPost, routeKey+":"+input.OrderID, body), input, apiOrderCompletionBuilder(false))
	})
}

func (s *Server) handleContinuousAPIOrderAction(w http.ResponseWriter, r *http.Request, participantRole, action string, run func(context.Context, auth.BusinessActor, string, string, []byte, apiorder.ActionInput) (idempotency.Completion, *domain.AppError)) {
	actor, appErr := s.requireBusinessActor(r, true, true)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, input, appErr := s.decodeAPIOrderAction(r, action)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	input.OrderID = chi.URLParam(r, "id")
	input.ParticipantRole = participantRole
	input.ExpectedVersion = version
	input.RequestID = requestIDFrom(r)
	prefix := "/me/"
	if participantRole == "seller" {
		prefix = "/owner/"
	}
	routeKey := "POST /api/v1" + prefix + "api-orders/{id}/" + action
	completion, appErr := run(r.Context(), actor, routeKey, r.Header.Get("Idempotency-Key"), body, input)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeNoStoreIdempotencyCompletion(w, completion)
}

func (s *Server) handleBuyerAPIOrderAction(w http.ResponseWriter, r *http.Request, action string, run func(context.Context, auth.User, string, string, []byte, apiorder.ActionInput) (idempotency.Completion, *domain.AppError)) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, input, appErr := s.decodeAPIOrderAction(r, action)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	input.OrderID = chi.URLParam(r, "id")
	input.ExpectedVersion = version
	input.RequestID = requestIDFrom(r)
	routeKey := "POST /api/v1/me/api-orders/{id}/" + action
	completion, appErr := run(r.Context(), user, routeKey, r.Header.Get("Idempotency-Key"), body, input)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeNoStoreIdempotencyCompletion(w, completion)
}

func (s *Server) handleOwnerAPIOrders(w http.ResponseWriter, r *http.Request) {
	actor, appErr := s.requireBusinessActor(r, true, false)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if actor.Audience == auth.SessionAudienceNormal && !requireActorCapability(w, r, actor, auth.CapabilityAPIServicePublish) {
		return
	}
	if appErr := validateAPIOrderListQuery(r); appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	orders, appErr := s.apiOrderContinuity.APIOrdersForActor(r.Context(), actor, "seller")
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toAPIOrderResponses(filterAPIOrders(r, orders), true))
}

func (s *Server) handleOwnerAPIOrder(w http.ResponseWriter, r *http.Request) {
	actor, appErr := s.requireBusinessActor(r, true, false)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if actor.Audience == auth.SessionAudienceNormal && !requireActorCapability(w, r, actor, auth.CapabilityAPIServicePublish) {
		return
	}
	order, appErr := s.apiOrderContinuity.APIOrderForActor(r.Context(), actor, chi.URLParam(r, "id"), "seller")
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, order.Version)
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, http.StatusOK, toAPIOrderResponse(order, true, true))
}

func (s *Server) handleConfirmAPIOrderPayment(w http.ResponseWriter, r *http.Request) {
	s.handleContinuousAPIOrderAction(w, r, "seller", "confirm-payment", func(ctx context.Context, actor auth.BusinessActor, routeKey, key string, body []byte, input apiorder.ActionInput) (idempotency.Completion, *domain.AppError) {
		return s.apiOrderContinuity.ConfirmAPIOrderPaymentForActorWithIdempotency(ctx, actor, routeKey, key, requestHash(http.MethodPost, routeKey+":"+input.OrderID, body), input, apiOrderCompletionBuilder(true))
	})
}

func (s *Server) handleReportAPIOrderPaymentIssue(w http.ResponseWriter, r *http.Request) {
	s.handleContinuousAPIOrderAction(w, r, "seller", "report-payment-issue", func(ctx context.Context, actor auth.BusinessActor, routeKey, key string, body []byte, input apiorder.ActionInput) (idempotency.Completion, *domain.AppError) {
		return s.apiOrderContinuity.ReportAPIOrderPaymentIssueForActorWithIdempotency(ctx, actor, routeKey, key, requestHash(http.MethodPost, routeKey+":"+input.OrderID, body), input, apiOrderCompletionBuilder(true))
	})
}

func (s *Server) handleSubmitAPIOrderDelivery(w http.ResponseWriter, r *http.Request) {
	s.handleContinuousAPIOrderAction(w, r, "seller", "submit-delivery", func(ctx context.Context, actor auth.BusinessActor, routeKey, key string, body []byte, input apiorder.ActionInput) (idempotency.Completion, *domain.AppError) {
		return s.apiOrderContinuity.SubmitAPIOrderDeliveryForActorWithIdempotency(ctx, actor, routeKey, key, requestHash(http.MethodPost, routeKey+":"+input.OrderID, body), input, apiOrderCompletionBuilder(true))
	})
}

func (s *Server) handleResolveLateAPIOrderPayment(w http.ResponseWriter, r *http.Request) {
	s.handleOwnerAPIOrderAction(w, r, "resolve-late-payment", func(ctx context.Context, user auth.User, routeKey, key string, body []byte, input apiorder.ActionInput) (idempotency.Completion, *domain.AppError) {
		return s.app.ResolveLateAPIOrderPaymentWithIdempotency(ctx, user, routeKey, key, requestHash(http.MethodPost, routeKey+":"+input.OrderID, body), input, apiOrderCompletionBuilder(true))
	})
}

func (s *Server) handleOwnerAPIOrderAction(w http.ResponseWriter, r *http.Request, action string, run func(context.Context, auth.User, string, string, []byte, apiorder.ActionInput) (idempotency.Completion, *domain.AppError)) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if !requireCapability(w, r, user, auth.CapabilityAPIServicePublish) {
		return
	}
	body, input, appErr := s.decodeAPIOrderAction(r, action)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	input.OrderID = chi.URLParam(r, "id")
	input.ExpectedVersion = version
	input.RequestID = requestIDFrom(r)
	routeKey := "POST /api/v1/owner/api-orders/{id}/" + action
	completion, appErr := run(r.Context(), user, routeKey, r.Header.Get("Idempotency-Key"), body, input)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeNoStoreIdempotencyCompletion(w, completion)
}

func (s *Server) decodeAPIOrderAction(r *http.Request, action string) ([]byte, apiorder.ActionInput, *domain.AppError) {
	switch action {
	case "submit-payment":
		body, req, appErr := decodeStrictJSON[apiOrderPaymentRequest](r)
		return body, apiorder.ActionInput{PaymentSummary: req.PaymentSummary}, appErr
	case "submit-delivery":
		body, req, appErr := decodeStrictJSON[apiOrderDeliveryRequest](r)
		return body, apiorder.ActionInput{DeliveryCredential: apiorder.DeliveryCredentialInput{
			DeliveryKind:  req.DeliveryKind,
			APIBaseURL:    req.APIBaseURL,
			APIKey:        req.APIKey,
			PanelLoginURL: req.PanelLoginURL,
			Username:      req.Username,
			Password:      req.Password,
			Instructions:  req.Instructions,
		}}, appErr
	case "report-payment-issue":
		body, req, appErr := decodeStrictJSON[apiOrderPaymentIssueRequest](r)
		return body, apiorder.ActionInput{PaymentIssueReason: req.Reason, PaymentIssueNote: req.Note}, appErr
	case "report-late-payment":
		body, req, appErr := decodeStrictJSON[apiOrderLatePaymentRequest](r)
		return body, apiorder.ActionInput{LatePaymentNote: req.Note}, appErr
	case "resolve-late-payment":
		body, req, appErr := decodeStrictJSON[apiOrderLatePaymentRequest](r)
		return body, apiorder.ActionInput{LatePaymentStatus: req.Status, LatePaymentNote: req.Note}, appErr
	case "cancel":
		body, req, appErr := decodeStrictJSON[apiOrderReasonRequest](r)
		return body, apiorder.ActionInput{Reason: req.Reason}, appErr
	case "dispute":
		body, req, appErr := decodeStrictJSON[apiOrderDisputeRequest](r)
		return body, apiorder.ActionInput{
			IssueCode:           req.IssueCode,
			RequestedResolution: req.RequestedResolution,
			RequestedAmountCNY:  req.RequestedAmountCNY,
			IssueOccurredAt:     req.IssueOccurredAt,
			Reason:              req.Reason,
			EvidenceAssetIDs:    append([]string(nil), req.EvidenceAssetIDs...),
		}, appErr
	default:
		body, _, appErr := decodeStrictJSON[emptyRequest](r)
		return body, apiorder.ActionInput{}, appErr
	}
}

func toAPIOrderResponses(orders []apiorder.Order, ownerView bool) []apiOrderResponse {
	items := make([]apiOrderResponse, 0, len(orders))
	for _, order := range orders {
		items = append(items, toAPIOrderResponse(order, ownerView, false))
	}
	return items
}

func toAdminAPIOrderResponses(orders []apiorder.Order) []apiOrderResponse {
	items := make([]apiOrderResponse, 0, len(orders))
	for _, order := range orders {
		items = append(items, toAdminAPIOrderResponse(order))
	}
	return items
}

func toAdminAPIOrderResponse(order apiorder.Order) apiOrderResponse {
	response := toAPIOrderResponse(order, false, false)
	response.BuyerUserID = order.BuyerUserID
	response.SellerUserID = order.SellerUserID
	response.DeliveryCredential = nil
	response.DisputeNeedsAction = false
	response.DisputeAvailableActions = []string{}
	return response
}

func toAPIOrderResponse(order apiorder.Order, ownerView bool, includeCredential bool) apiOrderResponse {
	viewerUserID := order.BuyerUserID
	if ownerView {
		viewerUserID = order.SellerUserID
	}
	disputeActions := apiOrderAvailableActions(order, viewerUserID, time.Now())
	response := apiOrderResponse{
		ID:                            order.ID,
		OrderNo:                       order.OrderNo,
		PurchaseKind:                  order.PurchaseKind,
		APIPurchaseIntentID:           order.APIPurchaseIntentID,
		APIServiceID:                  order.APIServiceID,
		BuyerReputation:               toReputationSummary(order.BuyerReputation),
		SellerReputation:              toReputationSummary(order.SellerReputation),
		Status:                        order.Status,
		DisputeStatus:                 order.DisputeStatus,
		DisputeCaseID:                 order.DisputeCaseID,
		LatestDisputeCaseID:           order.LatestDisputeCaseID,
		HasDisputeHistory:             order.HasDisputeHistory,
		DisputeNextActor:              order.DisputeNextActor,
		DisputeDueAt:                  formatOptionalTime(order.DisputeDueAt),
		DisputeNeedsAction:            len(disputeActions) > 0,
		DisputeResponseOverdue:        order.DisputeStatus == apiorder.DisputeStatusPendingSellerResponse && order.DisputeDueAt != nil && !time.Now().Before(*order.DisputeDueAt),
		DisputeAvailableActions:       disputeActions,
		ActiveRemedyAction:            order.ActiveRemedyAction,
		ActiveRemedySource:            order.ActiveRemedySource,
		ServiceTitleSnapshot:          order.ServiceTitleSnapshot,
		ServiceVersionSnapshot:        order.ServiceVersionSnapshot,
		BillingModeSnapshot:           order.BillingModeSnapshot,
		SelectedPackageID:             order.SelectedPackageID,
		SelectedPackageSnapshot:       order.SelectedPackageSnapshot,
		QuoteVersionSnapshot:          order.QuoteVersionSnapshot,
		RequestedUSDAllowanceSnapshot: order.RequestedUSDAllowanceSnapshot,
		CNYPerUSDAllowanceSnapshot:    order.CNYPerUSDAllowanceSnapshot,
		PricingSnapshot:               order.PricingSnapshot,
		ProbeConnectionIDSnapshot:     order.ProbeConnectionIDSnapshot,
		APIBaseURLSnapshot:            order.APIBaseURLSnapshot,
		QuotaUsagePolicySnapshot:      toAPIQuotaUsagePolicyResponse(order.QuotaUsagePolicySnapshot),
		PromptAuditEnabledSnapshot:    order.PromptAuditEnabledSnapshot,
		PackageStockReserved:          order.PackageStockReserved,
		PackageExpiresAt:              formatOptionalTime(order.PackageExpiresAt),
		APIQuotaBatchID:               order.APIQuotaBatchID,
		APIQuotaOfferID:               order.APIQuotaOfferID,
		APIQuotaSaleRoundID:           order.APIQuotaSaleRoundID,
		QuotaOfferNameSnapshot:        order.QuotaOfferNameSnapshot,
		QuotaUSDAllowanceSnapshot:     order.QuotaUSDAllowanceSnapshot,
		QuotaPriceCNYSnapshot:         order.QuotaPriceCNYSnapshot,
		QuotaCNYPerUSDSnapshot:        order.QuotaCNYPerUSDSnapshot,
		QuotaModelMultiplierSnapshot:  order.QuotaModelMultiplierSnapshot,
		QuotaSaleCutoffAtSnapshot:     formatOptionalTime(order.QuotaSaleCutoffAtSnapshot),
		QuotaExpiresAtSnapshot:        formatOptionalTime(order.QuotaExpiresAtSnapshot),
		QuotaSaleModeSnapshot:         order.QuotaSaleModeSnapshot,
		QuotaRoundStartsAtSnapshot:    formatOptionalTime(order.QuotaRoundStartsAtSnapshot),
		QuotaRoundEndsAtSnapshot:      formatOptionalTime(order.QuotaRoundEndsAtSnapshot),
		QuotaDistributionSnapshot:     order.QuotaDistributionSnapshot,
		QuotaTTFTBandSnapshot:         order.QuotaTTFTBandSnapshot,
		QuotaDeclaredMaxConcurrency:   order.QuotaDeclaredMaxConcurrency,
		QuotaPerformanceConfirmedAt:   formatOptionalTime(order.QuotaPerformanceConfirmedAt),
		QuotaPerformanceUnverified:    order.QuotaPerformanceUnverified,
		QuotaDeliveryETAMinutes:       order.QuotaDeliveryETAMinutes,
		QuotaDeliveryMode:             order.QuotaDeliveryMode,
		Amount:                        order.Amount,
		Currency:                      order.Currency,
		SelectedPaymentMethod:         order.SelectedPaymentMethod,
		PaymentWindowMinutesSnapshot:  order.PaymentWindowMinutesSnapshot,
		PaymentExpiresAt:              order.PaymentExpiresAt.UTC().Format(time.RFC3339),
		PaymentSummary:                order.PaymentSummary,
		PaidConfirmedAt:               formatOptionalTime(order.PaidConfirmedAt),
		PaymentSubmittedAt:            formatOptionalTime(order.PaymentSubmittedAt),
		MerchantConfirmDueAt:          formatOptionalTime(order.MerchantConfirmDueAt),
		MerchantConfirmOverdue:        order.MerchantConfirmOverdue,
		MerchantConfirmOverdueAt:      formatOptionalTime(order.MerchantConfirmOverdueAt),
		PaymentIssueReason:            order.PaymentIssueReason,
		PaymentIssueNote:              order.PaymentIssueNote,
		PaymentIssueReportedAt:        formatOptionalTime(order.PaymentIssueReportedAt),
		DeliveryDueAt:                 formatOptionalTime(order.DeliveryDueAt),
		DeliveryOverdue:               order.DeliveryOverdue,
		DeliveryOverdueAt:             formatOptionalTime(order.DeliveryOverdueAt),
		DeliveryDueRemindedAt:         formatOptionalTime(order.DeliveryDueRemindedAt),
		DeliveryNote:                  order.DeliveryNote,
		DeliverySubmittedAt:           formatOptionalTime(order.DeliverySubmittedAt),
		DeliveryReviewExpiresAt:       formatOptionalTime(order.DeliveryReviewExpiresAt),
		CommercialOutcome:             order.CommercialOutcome,
		CommercialOutcomeUpdatedAt:    formatOptionalTime(order.CommercialOutcomeUpdatedAt),
		QuotaValidityIssueAt:          formatOptionalTime(order.QuotaValidityIssueAt),
		QuotaValidityIssueReason:      order.QuotaValidityIssueReason,
		CompletionSource:              order.CompletionSource,
		CompletedAt:                   formatOptionalTime(order.CompletedAt),
		CancelledAt:                   formatOptionalTime(order.CancelledAt),
		CancelReason:                  order.CancelReason,
		LatePaymentStatus:             order.LatePaymentStatus,
		LatePaymentReportedAt:         formatOptionalTime(order.LatePaymentReportedAt),
		LatePaymentNote:               order.LatePaymentNote,
		LatePaymentResolvedAt:         formatOptionalTime(order.LatePaymentResolvedAt),
		CanReportLatePayment:          order.CanReportLatePayment,
		AfterSalesExpiresAt:           formatOptionalTime(order.AfterSalesExpiresAt),
		CanOpenDispute:                order.CanOpenDispute,
		DisputeEligibilityReason:      order.DisputeEligibilityReason,
		Version:                       order.Version,
		CreatedAt:                     order.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:                     order.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if ownerView {
		response.BuyerUserID = order.BuyerUserID
	} else {
		response.SellerUserID = order.SellerUserID
	}
	if includeCredential && order.DeliveryCredential != nil {
		response.DeliveryCredential = toAPIOrderDeliveryCredentialResponse(*order.DeliveryCredential)
	}
	if order.CatalogRiskHold != nil {
		response.CatalogRiskHold = &apiOrderCatalogRiskHoldResponse{
			ID: order.CatalogRiskHold.ID, SourceType: order.CatalogRiskHold.SourceType, SourceID: order.CatalogRiskHold.SourceID,
			Status: order.CatalogRiskHold.Status, Reason: order.CatalogRiskHold.Reason,
			CreatedAt: order.CatalogRiskHold.CreatedAt.UTC().Format(time.RFC3339), ResolvedBy: order.CatalogRiskHold.ResolvedBy,
			ResolvedAt: formatOptionalTime(order.CatalogRiskHold.ResolvedAt), ResolutionNote: order.CatalogRiskHold.ResolutionNote,
			Version: order.CatalogRiskHold.Version,
		}
	}
	return response
}

func apiOrderAvailableActions(order apiorder.Order, viewerUserID string, now time.Time) []string {
	actions := make([]string, 0, 3)
	isBuyer := viewerUserID == order.BuyerUserID
	isSeller := viewerUserID == order.SellerUserID
	switch order.DisputeStatus {
	case apiorder.DisputeStatusPendingSellerResponse:
		if isSeller {
			actions = append(actions, report.DisputeActionSellerDecision)
		}
		if isBuyer {
			actions = append(actions, report.DisputeActionWithdraw)
			if order.DisputeDueAt != nil && !now.Before(*order.DisputeDueAt) {
				actions = append(actions, report.DisputeActionRequestPlatformIntervention)
			}
		}
	case apiorder.DisputeStatusPendingApplicantDecision:
		if isBuyer {
			actions = append(actions, report.DisputeActionWithdraw, report.DisputeActionRequestPlatformIntervention)
		}
	case apiorder.DisputeStatusAwaitingFulfillment:
		if order.DisputeNextUserID == viewerUserID {
			actions = append(actions, report.DisputeRemedyActionClaim)
		}
		if isBuyer && order.ActiveRemedySource == report.RemedySourceSellerAcceptance && order.DisputeDueAt != nil && !now.Before(*order.DisputeDueAt) {
			actions = append(actions, report.DisputeActionRequestPlatformIntervention)
		}
	case apiorder.DisputeStatusFulfillmentConfirmation:
		if order.DisputeNextUserID == viewerUserID {
			actions = append(actions, report.DisputeRemedyActionConfirm)
			if order.ActiveRemedySource == report.RemedySourceSellerAcceptance {
				actions = append(actions, report.DisputeActionRequestPlatformIntervention)
			} else {
				actions = append(actions, report.DisputeRemedyActionContest)
			}
		}
	}
	return actions
}

func toAPIOrderDeliveryCredentialResponse(credential apiorder.DeliveryCredential) *apiOrderDeliveryCredentialResponse {
	response := &apiOrderDeliveryCredentialResponse{
		DeliveryKind:  credential.DeliveryKind,
		SubmittedAt:   credential.SubmittedAt.UTC().Format(time.RFC3339),
		DestroyedAt:   formatOptionalTime(credential.DestroyedAt),
		DestroyReason: credential.DestroyReason,
	}
	if credential.DestroyedAt != nil {
		return response
	}
	response.APIBaseURL = credential.APIBaseURL
	response.APIKey = credential.APIKey
	response.PanelLoginURL = credential.PanelLoginURL
	response.Username = credential.Username
	response.Password = credential.Password
	response.Instructions = credential.Instructions
	return response
}

func apiOrderCompletionBuilder(ownerView bool) apiorder.CompletionBuilder {
	return func(order apiorder.Order) (idempotency.Completion, *domain.AppError) {
		responseBody, marshalErr := json.Marshal(toAPIOrderResponse(order, ownerView, true))
		if marshalErr != nil {
			return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
		}
		return idempotency.Completion{
			Status:        http.StatusOK,
			ContentType:   "application/json; charset=utf-8",
			Body:          responseBody,
			SkipBodyCache: true,
			ResourceType:  "api_order",
			ResourceID:    order.ID,
			Headers: map[string]string{
				"ETag": `"` + strconv.FormatInt(order.Version, 10) + `"`,
			},
		}, nil
	}
}

func apiOrderCreateCompletionBuilder(ownerView bool) apiorder.CompletionBuilder {
	return func(order apiorder.Order) (idempotency.Completion, *domain.AppError) {
		responseBody, marshalErr := json.Marshal(toAPIOrderResponse(order, ownerView, true))
		if marshalErr != nil {
			return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
		}
		return idempotency.Completion{
			Status:        http.StatusCreated,
			ContentType:   "application/json; charset=utf-8",
			Body:          responseBody,
			SkipBodyCache: true,
			ResourceType:  "api_order",
			ResourceID:    order.ID,
			Headers: map[string]string{
				"ETag":     `"` + strconv.FormatInt(order.Version, 10) + `"`,
				"Location": "/api/v1/me/api-orders/" + order.ID,
			},
		}, nil
	}
}
