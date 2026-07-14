package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apiorder"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/go-chi/chi/v5"
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

type apiOrderPaymentIssueRequest struct {
	Reason string `json:"reason"`
	Note   string `json:"note"`
}

type apiOrderResponse struct {
	ID                            string                              `json:"id"`
	APIPurchaseIntentID           string                              `json:"apiPurchaseIntentId"`
	APIServiceID                  string                              `json:"apiServiceId"`
	BuyerUserID                   string                              `json:"buyerUserId,omitempty"`
	SellerUserID                  string                              `json:"sellerUserId,omitempty"`
	Status                        string                              `json:"status"`
	DisputeStatus                 string                              `json:"disputeStatus"`
	DisputeCaseID                 string                              `json:"disputeCaseId,omitempty"`
	ServiceTitleSnapshot          string                              `json:"serviceTitleSnapshot"`
	ServiceVersionSnapshot        int64                               `json:"serviceVersionSnapshot"`
	BillingModeSnapshot           string                              `json:"billingModeSnapshot"`
	SelectedPackageID             string                              `json:"selectedPackageId,omitempty"`
	SelectedPackageSnapshot       string                              `json:"selectedPackageSnapshot,omitempty"`
	QuoteVersionSnapshot          int64                               `json:"quoteVersionSnapshot,omitempty"`
	RequestedUSDAllowanceSnapshot string                              `json:"requestedUsdAllowanceSnapshot,omitempty"`
	CNYPerUSDAllowanceSnapshot    string                              `json:"cnyPerUsdAllowanceSnapshot,omitempty"`
	PricingSnapshot               string                              `json:"pricingSnapshot"`
	Amount                        string                              `json:"amount"`
	Currency                      string                              `json:"currency"`
	SelectedPaymentMethod         string                              `json:"selectedPaymentMethod"`
	PaymentWindowMinutesSnapshot  int                                 `json:"paymentWindowMinutesSnapshot"`
	PaymentExpiresAt              string                              `json:"paymentExpiresAt"`
	PaymentSummary                string                              `json:"paymentSummary,omitempty"`
	PaymentSubmittedAt            *string                             `json:"paymentSubmittedAt,omitempty"`
	PaymentIssueReason            string                              `json:"paymentIssueReason,omitempty"`
	PaymentIssueNote              string                              `json:"paymentIssueNote,omitempty"`
	PaymentIssueReportedAt        *string                             `json:"paymentIssueReportedAt,omitempty"`
	PaidConfirmedAt               *string                             `json:"paidConfirmedAt,omitempty"`
	DeliveryNote                  string                              `json:"deliveryNote,omitempty"`
	DeliverySubmittedAt           *string                             `json:"deliverySubmittedAt,omitempty"`
	DeliveryCredential            *apiOrderDeliveryCredentialResponse `json:"deliveryCredential,omitempty"`
	CompletedAt                   *string                             `json:"completedAt,omitempty"`
	CancelledAt                   *string                             `json:"cancelledAt,omitempty"`
	CancelReason                  string                              `json:"cancelReason,omitempty"`
	Version                       int64                               `json:"version"`
	CreatedAt                     string                              `json:"createdAt"`
	UpdatedAt                     string                              `json:"updatedAt"`
}

type apiOrderPaymentInstructionsResponse struct {
	OrderID              string `json:"orderId"`
	PaymentMethod        string `json:"paymentMethod"`
	PaymentInstructions  string `json:"paymentInstructions"`
	PaymentQRCodeDataURL string `json:"paymentQrCodeDataUrl,omitempty"`
	PaymentExpiresAt     string `json:"paymentExpiresAt"`
}

type apiOrderDeliveryCredentialResponse struct {
	DeliveryKind  string `json:"deliveryKind"`
	APIBaseURL    string `json:"apiBaseUrl,omitempty"`
	APIKey        string `json:"apiKey,omitempty"`
	PanelLoginURL string `json:"panelLoginUrl,omitempty"`
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
	Instructions  string `json:"instructions,omitempty"`
	SubmittedAt   string `json:"submittedAt"`
}

func (s *Server) handleCreateAPIOrder(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
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
		user.ID,
		routeKey,
		r.Header.Get("Idempotency-Key"),
		requestHash(r.Method, routeKey+":"+intentID, body),
		apiorder.ActionInput{},
		apiorder.CreateInput{
			IntentID:      intentID,
			PaymentMethod: req.PaymentMethod,
			RequestID:     requestIDFrom(r),
		},
		func(order apiorder.Order) (idempotency.Completion, *domain.AppError) {
			responseBody, marshalErr := json.Marshal(toAPIOrderResponse(order, false, false))
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
		},
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeNoStoreIdempotencyCompletion(w, completion)
}

func (s *Server) handleMyAPIOrders(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	orders, appErr := s.app.MyAPIOrders(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toAPIOrderResponses(orders, false))
}

func (s *Server) handleAdminAPIOrders(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	orders, appErr := s.app.AdminAPIOrders(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toAPIOrderResponses(orders, false))
}

func (s *Server) handleMyAPIOrder(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	order, appErr := s.app.MyAPIOrder(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, order.Version)
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, http.StatusOK, toAPIOrderResponse(order, false, true))
}

func (s *Server) handleReadAPIOrderPaymentInstructions(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
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
	s.handleBuyerAPIOrderAction(w, r, "confirm-complete", func(ctx context.Context, user auth.User, routeKey, key string, body []byte, input apiorder.ActionInput) (idempotency.Completion, *domain.AppError) {
		return s.app.ConfirmAPIOrderCompleteWithIdempotency(ctx, user.ID, routeKey, key, requestHash(http.MethodPost, routeKey+":"+input.OrderID, body), input, apiOrderCompletionBuilder(false))
	})
}

func (s *Server) handleOpenAPIOrderDispute(w http.ResponseWriter, r *http.Request) {
	s.handleBuyerAPIOrderAction(w, r, "dispute", func(ctx context.Context, user auth.User, routeKey, key string, body []byte, input apiorder.ActionInput) (idempotency.Completion, *domain.AppError) {
		return s.app.OpenAPIOrderDisputeWithIdempotency(ctx, user.ID, routeKey, key, requestHash(http.MethodPost, routeKey+":"+input.OrderID, body), input, apiOrderCompletionBuilder(false))
	})
}

func (s *Server) handleBuyerAPIOrderAction(w http.ResponseWriter, r *http.Request, action string, run func(context.Context, auth.User, string, string, []byte, apiorder.ActionInput) (idempotency.Completion, *domain.AppError)) {
	user, _, appErr := s.requireSessionAndCSRF(r)
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
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	orders, appErr := s.app.OwnerAPIOrders(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toAPIOrderResponses(orders, true))
}

func (s *Server) handleOwnerAPIOrder(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	order, appErr := s.app.OwnerAPIOrder(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, order.Version)
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, http.StatusOK, toAPIOrderResponse(order, true, true))
}

func (s *Server) handleConfirmAPIOrderPayment(w http.ResponseWriter, r *http.Request) {
	s.handleOwnerAPIOrderAction(w, r, "confirm-payment", func(ctx context.Context, user auth.User, routeKey, key string, body []byte, input apiorder.ActionInput) (idempotency.Completion, *domain.AppError) {
		return s.app.ConfirmAPIOrderPaymentWithIdempotency(ctx, user.ID, routeKey, key, requestHash(http.MethodPost, routeKey+":"+input.OrderID, body), input, apiOrderCompletionBuilder(true))
	})
}

func (s *Server) handleReportAPIOrderPaymentIssue(w http.ResponseWriter, r *http.Request) {
	s.handleOwnerAPIOrderAction(w, r, "report-payment-issue", func(ctx context.Context, user auth.User, routeKey, key string, body []byte, input apiorder.ActionInput) (idempotency.Completion, *domain.AppError) {
		return s.app.ReportAPIOrderPaymentIssueWithIdempotency(ctx, user.ID, routeKey, key, requestHash(http.MethodPost, routeKey+":"+input.OrderID, body), input, apiOrderCompletionBuilder(true))
	})
}

func (s *Server) handleSubmitAPIOrderDelivery(w http.ResponseWriter, r *http.Request) {
	s.handleOwnerAPIOrderAction(w, r, "submit-delivery", func(ctx context.Context, user auth.User, routeKey, key string, body []byte, input apiorder.ActionInput) (idempotency.Completion, *domain.AppError) {
		return s.app.SubmitAPIOrderDeliveryWithIdempotency(ctx, user.ID, routeKey, key, requestHash(http.MethodPost, routeKey+":"+input.OrderID, body), input, apiOrderCompletionBuilder(true))
	})
}

func (s *Server) handleOwnerAPIOrderAction(w http.ResponseWriter, r *http.Request, action string, run func(context.Context, auth.User, string, string, []byte, apiorder.ActionInput) (idempotency.Completion, *domain.AppError)) {
	user, _, appErr := s.requireSessionAndCSRF(r)
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
	case "cancel", "dispute":
		body, req, appErr := decodeStrictJSON[apiOrderReasonRequest](r)
		return body, apiorder.ActionInput{Reason: req.Reason}, appErr
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

func toAPIOrderResponse(order apiorder.Order, ownerView bool, includeCredential bool) apiOrderResponse {
	response := apiOrderResponse{
		ID:                            order.ID,
		APIPurchaseIntentID:           order.APIPurchaseIntentID,
		APIServiceID:                  order.APIServiceID,
		Status:                        order.Status,
		DisputeStatus:                 order.DisputeStatus,
		DisputeCaseID:                 order.DisputeCaseID,
		ServiceTitleSnapshot:          order.ServiceTitleSnapshot,
		ServiceVersionSnapshot:        order.ServiceVersionSnapshot,
		BillingModeSnapshot:           order.BillingModeSnapshot,
		SelectedPackageID:             order.SelectedPackageID,
		SelectedPackageSnapshot:       order.SelectedPackageSnapshot,
		QuoteVersionSnapshot:          order.QuoteVersionSnapshot,
		RequestedUSDAllowanceSnapshot: order.RequestedUSDAllowanceSnapshot,
		CNYPerUSDAllowanceSnapshot:    order.CNYPerUSDAllowanceSnapshot,
		PricingSnapshot:               order.PricingSnapshot,
		Amount:                        order.Amount,
		Currency:                      order.Currency,
		SelectedPaymentMethod:         order.SelectedPaymentMethod,
		PaymentWindowMinutesSnapshot:  order.PaymentWindowMinutesSnapshot,
		PaymentExpiresAt:              order.PaymentExpiresAt.UTC().Format(time.RFC3339),
		PaymentSummary:                order.PaymentSummary,
		PaidConfirmedAt:               formatOptionalTime(order.PaidConfirmedAt),
		PaymentSubmittedAt:            formatOptionalTime(order.PaymentSubmittedAt),
		PaymentIssueReason:            order.PaymentIssueReason,
		PaymentIssueNote:              order.PaymentIssueNote,
		PaymentIssueReportedAt:        formatOptionalTime(order.PaymentIssueReportedAt),
		DeliveryNote:                  order.DeliveryNote,
		DeliverySubmittedAt:           formatOptionalTime(order.DeliverySubmittedAt),
		CompletedAt:                   formatOptionalTime(order.CompletedAt),
		CancelledAt:                   formatOptionalTime(order.CancelledAt),
		CancelReason:                  order.CancelReason,
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
	return response
}

func toAPIOrderDeliveryCredentialResponse(credential apiorder.DeliveryCredential) *apiOrderDeliveryCredentialResponse {
	return &apiOrderDeliveryCredentialResponse{
		DeliveryKind:  credential.DeliveryKind,
		APIBaseURL:    credential.APIBaseURL,
		APIKey:        credential.APIKey,
		PanelLoginURL: credential.PanelLoginURL,
		Username:      credential.Username,
		Password:      credential.Password,
		Instructions:  credential.Instructions,
		SubmittedAt:   credential.SubmittedAt.UTC().Format(time.RFC3339),
	}
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
