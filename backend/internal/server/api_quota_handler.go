package server

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apihealth"
	"c2c-market/backend/internal/module/apiquota"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/go-chi/chi/v5"
)

type apiQuotaBatchRequest struct {
	SourceType                string `json:"sourceType"`
	SourceLabel               string `json:"sourceLabel"`
	DeclaredTotalUSDAllowance string `json:"declaredTotalUsdAllowance"`
	SaleCutoffAt              string `json:"saleCutoffAt"`
	ExpiresAt                 string `json:"expiresAt"`
	SourceConfirmedAt         string `json:"sourceConfirmedAt"`
}

type apiQuotaOfferRequest struct {
	Name               string                     `json:"name"`
	USDAllowance       string                     `json:"usdAllowance"`
	PriceCNY           string                     `json:"priceCny"`
	ModelMultiplier    string                     `json:"modelMultiplier"`
	QuotaUsagePolicy   apiQuotaUsagePolicyRequest `json:"quotaUsagePolicy"`
	DeliveryMode       string                     `json:"deliveryMode"`
	DeliveryETAMinutes int                        `json:"deliveryEtaMinutes"`
	SaleMode           string                     `json:"saleMode"`
	ContinuousCopies   int                        `json:"continuousCopies"`
	SortOrder          int                        `json:"sortOrder"`
}

type apiQuotaRushOfferRequest struct {
	SourceType         string                     `json:"sourceType"`
	SourceLabel        string                     `json:"sourceLabel"`
	Name               string                     `json:"name"`
	USDAllowance       string                     `json:"usdAllowance"`
	PriceCNY           string                     `json:"priceCny"`
	ModelMultiplier    string                     `json:"modelMultiplier"`
	QuotaUsagePolicy   apiQuotaUsagePolicyRequest `json:"quotaUsagePolicy"`
	Copies             int                        `json:"copies"`
	DeliveryMode       string                     `json:"deliveryMode"`
	DeliveryETAMinutes int                        `json:"deliveryEtaMinutes"`
	SlotKey            string                     `json:"slotKey"`
	ExpiresAt          string                     `json:"expiresAt"`
	SourceConfirmedAt  string                     `json:"sourceConfirmedAt"`
	DeliveryKind       string                     `json:"deliveryKind"`
}

type apiQuotaRoundRequest struct {
	Name     string                      `json:"name"`
	StartsAt string                      `json:"startsAt"`
	EndsAt   string                      `json:"endsAt"`
	Offers   []apiQuotaRoundOfferRequest `json:"offers"`
}

type apiQuotaRoundOfferRequest struct {
	OfferID string `json:"offerId"`
	Copies  int    `json:"copies"`
}

type createAPIQuotaOrderRequest struct {
	SaleRoundID          string `json:"saleRoundId"`
	BuyerContactMethodID string `json:"buyerContactMethodId"`
	SelectedAccessMode   string `json:"selectedAccessMode"`
	PaymentMethod        string `json:"paymentMethod"`
	BuyerNote            string `json:"buyerNote"`
}

type apiQuotaBatchResponse struct {
	ID                        string  `json:"id"`
	APIServiceID              string  `json:"apiServiceId"`
	SourceType                string  `json:"sourceType"`
	SourceLabel               string  `json:"sourceLabel,omitempty"`
	Status                    string  `json:"status"`
	DeclaredTotalUSDAllowance string  `json:"declaredTotalUsdAllowance"`
	UnallocatedUSDAllowance   string  `json:"unallocatedUsdAllowance"`
	SaleCutoffAt              string  `json:"saleCutoffAt"`
	ExpiresAt                 string  `json:"expiresAt"`
	SourceConfirmedAt         string  `json:"sourceConfirmedAt"`
	PublishedAt               *string `json:"publishedAt,omitempty"`
	Version                   int64   `json:"version"`
}

type apiQuotaOfferResponse struct {
	ID                 string                      `json:"id"`
	BatchID            string                      `json:"batchId"`
	APIServiceID       string                      `json:"apiServiceId"`
	DistributionSystem string                      `json:"distributionSystem"`
	Name               string                      `json:"name"`
	USDAllowance       string                      `json:"usdAllowance"`
	PriceCNY           string                      `json:"priceCny"`
	CNYPerUSD          string                      `json:"cnyPerUsd"`
	ModelMultiplier    string                      `json:"modelMultiplier"`
	QuotaUsagePolicy   apiQuotaUsagePolicyResponse `json:"quotaUsagePolicy"`
	DeliveryMode       string                      `json:"deliveryMode"`
	DeliveryETAMinutes int                         `json:"deliveryEtaMinutes"`
	SaleMode           string                      `json:"saleMode"`
	Status             string                      `json:"status"`
	SortOrder          int                         `json:"sortOrder"`
	PublishedAt        *string                     `json:"publishedAt,omitempty"`
	Version            int64                       `json:"version"`
}

type apiQuotaAllocationResponse struct {
	ID                    string `json:"id"`
	OfferID               string `json:"offerId"`
	SaleRoundID           string `json:"saleRoundId,omitempty"`
	SaleMode              string `json:"saleMode"`
	CopyLimit             int    `json:"copyLimit"`
	AvailableCopies       int    `json:"availableCopies"`
	ReservedCopies        int    `json:"reservedCopies"`
	ConsumedCopies        int    `json:"consumedCopies"`
	AllocatedUSDAllowance string `json:"allocatedUsdAllowance"`
	ReturnedUSDAllowance  string `json:"returnedUsdAllowance"`
	Status                string `json:"status"`
}

type apiQuotaRoundResponse struct {
	ID            string                       `json:"id"`
	BatchID       string                       `json:"batchId"`
	SystemSlotKey string                       `json:"systemSlotKey,omitempty"`
	Name          string                       `json:"name"`
	StartsAt      string                       `json:"startsAt"`
	EndsAt        string                       `json:"endsAt"`
	Status        string                       `json:"status"`
	Allocations   []apiQuotaAllocationResponse `json:"allocations"`
	Version       int64                        `json:"version"`
}

type apiQuotaSystemSaleSlotResponse struct {
	Key                  string `json:"key"`
	StartsAt             string `json:"startsAt"`
	EndsAt               string `json:"endsAt"`
	RegistrationClosesAt string `json:"registrationClosesAt"`
	State                string `json:"state"`
}

type apiQuotaSystemSaleSlotListResponse struct {
	ServerNow string                           `json:"serverNow"`
	Items     []apiQuotaSystemSaleSlotResponse `json:"items"`
}

type publicAPIQuotaOfferResponse struct {
	apiQuotaOfferResponse
	BatchStatus               string                          `json:"batchStatus"`
	ServiceTitle              string                          `json:"serviceTitle"`
	SellerDisplayName         string                          `json:"sellerDisplayName"`
	SellerIdentityType        string                          `json:"sellerIdentityType"`
	SellerLinuxDOBound        bool                            `json:"sellerLinuxDoBound"`
	DeclaredMaxConcurrency    int                             `json:"declaredMaxConcurrency"`
	HealthSummary             apiServiceHealthSummaryResponse `json:"healthSummary"`
	SaleCutoffAt              string                          `json:"saleCutoffAt"`
	ExpiresAt                 string                          `json:"expiresAt"`
	CurrentRound              *apiQuotaRoundResponse          `json:"currentRound,omitempty"`
	NextRound                 *apiQuotaRoundResponse          `json:"nextRound,omitempty"`
	AvailableCopies           int                             `json:"availableCopies"`
	CredentialAvailableCopies int                             `json:"credentialAvailableCopies"`
	IsOrderable               bool                            `json:"isOrderable"`
	OrderabilityCode          string                          `json:"orderabilityCode"`
	OrderabilityReason        string                          `json:"orderabilityReason"`
}

type apiQuotaCredentialSummaryResponse struct {
	OfferID   string `json:"offerId"`
	Available int    `json:"available"`
	Reserved  int    `json:"reserved"`
	Delivered int    `json:"delivered"`
	Retired   int    `json:"retired"`
}

type apiQuotaCredentialImportResponse struct {
	Imported int                               `json:"imported"`
	Summary  apiQuotaCredentialSummaryResponse `json:"summary"`
}

type apiQuotaRushOfferResponse struct {
	Batch              apiQuotaBatchResponse             `json:"batch"`
	Offer              apiQuotaOfferResponse             `json:"offer"`
	Round              apiQuotaRoundResponse             `json:"round"`
	CredentialImported int                               `json:"credentialImported"`
	CredentialSummary  apiQuotaCredentialSummaryResponse `json:"credentialSummary"`
}

func (s *Server) handlePublicAPIQuotaOffers(w http.ResponseWriter, r *http.Request) {
	page, appErr := parsePageRequest(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	oneMultiplier, appErr := parseOptionalQueryBool(r, "oneMultiplier")
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	onlyOrderable, appErr := parseOptionalQueryBool(r, "onlyOrderable")
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	result, appErr := s.apiQuotas.PublicAPIQuotaOffers(r.Context(), apiquota.PublicOfferFilter{
		DistributionSystem: r.URL.Query().Get("distributionSystem"),
		OnlyOneMultiplier:  oneMultiplier,
		OnlyOrderable:      onlyOrderable,
		SystemSlotKey:      r.URL.Query().Get("slotKey"),
	}, page)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items := make([]publicAPIQuotaOfferResponse, 0, len(result.Items))
	serviceIDs := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		serviceIDs = append(serviceIDs, item.APIServiceID)
	}
	summaries := s.loadAPIHealthSummaries(r.Context(), serviceIDs)
	for _, item := range result.Items {
		items = append(items, toPublicAPIQuotaOfferResponseWithHealth(item, summaries[item.APIServiceID]))
	}
	writePageJSON(w, domain.Page[publicAPIQuotaOfferResponse]{Items: items, NextCursor: result.NextCursor})
}

func (s *Server) handleAPIQuotaSystemSaleSlots(w http.ResponseWriter, _ *http.Request) {
	slots := s.apiQuotas.APIQuotaSystemSaleSlots()
	items := make([]apiQuotaSystemSaleSlotResponse, 0, len(slots))
	serverNow := time.Time{}
	for _, slot := range slots {
		serverNow = slot.ServerNow
		items = append(items, apiQuotaSystemSaleSlotResponse{
			Key:                  slot.Key,
			StartsAt:             slot.StartsAt.Format(time.RFC3339Nano),
			EndsAt:               slot.EndsAt.Format(time.RFC3339Nano),
			RegistrationClosesAt: slot.RegistrationClosesAt.Format(time.RFC3339Nano),
			State:                slot.State,
		})
	}
	writeJSON(w, http.StatusOK, apiQuotaSystemSaleSlotListResponse{
		ServerNow: serverNow.Format(time.RFC3339Nano),
		Items:     items,
	})
}

func (s *Server) handlePublicAPIQuotaOffer(w http.ResponseWriter, r *http.Request) {
	item, appErr := s.apiQuotas.PublicAPIQuotaOffer(r.Context(), chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, item.Version)
	summaries := s.loadAPIHealthSummaries(r.Context(), []string{item.APIServiceID})
	writeJSON(w, http.StatusOK, toPublicAPIQuotaOfferResponseWithHealth(item, summaries[item.APIServiceID]))
}

func (s *Server) handleCreateAPIQuotaOrder(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[createAPIQuotaOrderRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	offerID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/api-quota-offers/{id}/orders"
	completion, appErr := s.apiQuotas.CreateAPIQuotaOrderWithIdempotency(r.Context(), user.ID, routeKey, r.Header.Get("Idempotency-Key"), requestHash(http.MethodPost, routeKey+":"+offerID, body), apiquota.CreateOrderInput{
		OfferID: offerID, SaleRoundID: req.SaleRoundID, BuyerContactMethodID: req.BuyerContactMethodID,
		SelectedAccessMode: req.SelectedAccessMode, PaymentMethod: req.PaymentMethod,
		BuyerNote: req.BuyerNote, RequestID: requestIDFrom(r),
	}, apiOrderCreateCompletionBuilder(false))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleOwnerAPIQuotaBatches(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	page, appErr := parsePageRequest(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	result, appErr := s.apiQuotas.OwnerAPIQuotaBatches(r.Context(), user, chi.URLParam(r, "id"), page)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items := make([]apiQuotaBatchResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, toAPIQuotaBatchResponse(item))
	}
	writePageJSON(w, domain.Page[apiQuotaBatchResponse]{Items: items, NextCursor: result.NextCursor})
}

func (s *Server) handleCreateAPIQuotaBatch(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[apiQuotaBatchRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	saleCutoffAt, appErr := parseAPIQuotaTime("saleCutoffAt", req.SaleCutoffAt)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	expiresAt, appErr := parseAPIQuotaTime("expiresAt", req.ExpiresAt)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	confirmedAt, appErr := parseAPIQuotaTime("sourceConfirmedAt", req.SourceConfirmedAt)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	serviceID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/owner/api-services/{id}/quota-batches:" + serviceID
	s.withIdempotency(w, r, user.ID, routeKey, body, func() (int, any, string, string, *domain.AppError) {
		item, runErr := s.apiQuotas.CreateAPIQuotaBatch(r.Context(), user, apiquota.CreateBatchInput{
			APIServiceID: serviceID, SourceType: req.SourceType, SourceLabel: req.SourceLabel,
			DeclaredTotalUSDAllowance: req.DeclaredTotalUSDAllowance,
			SaleCutoffAt:              saleCutoffAt, ExpiresAt: expiresAt, SourceConfirmedAt: confirmedAt,
		})
		return http.StatusCreated, toAPIQuotaBatchResponse(item), "api_quota_batch", item.ID, runErr
	})
}

func (s *Server) handleCreateAPIQuotaRushOffer(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, file, appErr := decodeRushOfferMultipart(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	expiresAt, appErr := parseAPIQuotaTime("expiresAt", req.ExpiresAt)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	sourceConfirmedAt, appErr := parseAPIQuotaTime("sourceConfirmedAt", req.SourceConfirmedAt)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	credentialRows := []apiquota.CredentialImportRow(nil)
	if req.DeliveryMode == apiquota.DeliveryModePreimported {
		if len(file) == 0 {
			writeProblem(w, r, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Credential CSV required", "预导入交付必须上传凭据 CSV。", "file", "required", "请上传凭据 CSV。"))
			return
		}
		credentialRows, appErr = apiquota.ParseCredentialCSV(bytes.NewReader(file), req.DeliveryKind)
		if appErr != nil {
			writeProblem(w, r, appErr)
			return
		}
	} else if len(file) > 0 {
		writeProblem(w, r, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Credential CSV unexpected", "卖家手工交付不需要上传凭据 CSV。", "file", "unexpected", "请移除凭据 CSV。"))
		return
	}

	serviceID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/owner/api-services/{id}/quota-rush-offers"
	completion, appErr := s.apiQuotas.CreateAPIQuotaRushOfferWithIdempotency(
		r.Context(),
		user,
		routeKey,
		r.Header.Get("Idempotency-Key"),
		requestHash(http.MethodPost, routeKey+":"+serviceID, body),
		apiquota.CreateRushOfferInput{
			APIServiceID: serviceID, SourceType: req.SourceType, SourceLabel: req.SourceLabel,
			Name: req.Name, USDAllowance: req.USDAllowance, PriceCNY: req.PriceCNY,
			ModelMultiplier: req.ModelMultiplier, QuotaUsagePolicy: toAPIQuotaUsagePolicy(req.QuotaUsagePolicy), Copies: req.Copies,
			DeliveryMode: req.DeliveryMode, DeliveryETAMinutes: req.DeliveryETAMinutes,
			SlotKey: req.SlotKey, ExpiresAt: expiresAt, SourceConfirmedAt: sourceConfirmedAt,
			DeliveryKind: req.DeliveryKind, CredentialRows: credentialRows,
		},
		apiQuotaRushOfferCompletionBuilder,
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleOwnerAPIQuotaOffers(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items, appErr := s.apiQuotas.OwnerAPIQuotaOffers(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	responses := make([]apiQuotaOfferResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, toAPIQuotaOfferResponse(item))
	}
	writeJSON(w, http.StatusOK, responses)
}

func (s *Server) handleCreateAPIQuotaOffer(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[apiQuotaOfferRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	batchID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/owner/api-quota-batches/{id}/offers:" + batchID
	s.withIdempotency(w, r, user.ID, routeKey, body, func() (int, any, string, string, *domain.AppError) {
		item, runErr := s.apiQuotas.CreateAPIQuotaOffer(r.Context(), user, apiquota.CreateOfferInput{
			BatchID: batchID, Name: req.Name, USDAllowance: req.USDAllowance, PriceCNY: req.PriceCNY,
			ModelMultiplier: req.ModelMultiplier, QuotaUsagePolicy: toAPIQuotaUsagePolicy(req.QuotaUsagePolicy), DeliveryMode: req.DeliveryMode,
			DeliveryETAMinutes: req.DeliveryETAMinutes, SaleMode: req.SaleMode,
			ContinuousCopies: req.ContinuousCopies, SortOrder: req.SortOrder,
		})
		return http.StatusCreated, toAPIQuotaOfferResponse(item), "api_quota_offer", item.ID, runErr
	})
}

func (s *Server) handleOwnerAPIQuotaRounds(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items, appErr := s.apiQuotas.OwnerAPIQuotaRounds(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	responses := make([]apiQuotaRoundResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, toAPIQuotaRoundResponse(item))
	}
	writeJSON(w, http.StatusOK, responses)
}

func (s *Server) handleCreateAPIQuotaRound(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[apiQuotaRoundRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	startsAt, appErr := parseAPIQuotaTime("startsAt", req.StartsAt)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	endsAt, appErr := parseAPIQuotaTime("endsAt", req.EndsAt)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	offers := make([]apiquota.RoundOfferInput, 0, len(req.Offers))
	for _, item := range req.Offers {
		offers = append(offers, apiquota.RoundOfferInput{OfferID: item.OfferID, Copies: item.Copies})
	}
	batchID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/owner/api-quota-batches/{id}/rounds:" + batchID
	s.withIdempotency(w, r, user.ID, routeKey, body, func() (int, any, string, string, *domain.AppError) {
		item, runErr := s.apiQuotas.CreateAPIQuotaRound(r.Context(), user, apiquota.CreateRoundInput{
			BatchID: batchID, Name: req.Name, StartsAt: startsAt, EndsAt: endsAt, Offers: offers,
		})
		return http.StatusCreated, toAPIQuotaRoundResponse(item), "api_quota_sale_round", item.ID, runErr
	})
}

func (s *Server) handleAPIQuotaBatchAction(w http.ResponseWriter, r *http.Request, action string) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
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
	batchID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/owner/api-quota-batches/{id}/" + action + ":" + batchID
	s.withIdempotency(w, r, user.ID, routeKey, body, func() (int, any, string, string, *domain.AppError) {
		input := apiquota.BatchActionInput{BatchID: batchID, ExpectedVersion: version, RequestID: requestIDFrom(r)}
		var item apiquota.Batch
		var runErr *domain.AppError
		if action == "publish" {
			item, runErr = s.apiQuotas.PublishAPIQuotaBatch(r.Context(), user, input)
		} else {
			item, runErr = s.apiQuotas.UpdateAPIQuotaBatchStatus(r.Context(), user, input, action)
		}
		return http.StatusOK, toAPIQuotaBatchResponse(item), "api_quota_batch", item.ID, runErr
	})
}

func (s *Server) handleImportAPIQuotaCredentials(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, deliveryKind, file, appErr := decodeCredentialMultipart(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	offerID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/owner/api-quota-offers/{id}/credentials/import:" + offerID
	w.Header().Set("Cache-Control", "private, no-store")
	s.withIdempotency(w, r, user.ID, routeKey, body, func() (int, any, string, string, *domain.AppError) {
		result, runErr := s.apiQuotas.ImportAPIQuotaCredentials(r.Context(), user, apiquota.CredentialImportInput{
			OfferID: offerID, DeliveryKind: deliveryKind, CSV: bytes.NewReader(file),
		})
		response := apiQuotaCredentialImportResponse{Imported: result.Imported, Summary: toAPIQuotaCredentialSummaryResponse(result.Summary)}
		return http.StatusCreated, response, "api_quota_offer", offerID, runErr
	})
}

func (s *Server) handleAPIQuotaCredentialSummary(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	item, appErr := s.apiQuotas.APIQuotaCredentialSummary(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	writeJSON(w, http.StatusOK, toAPIQuotaCredentialSummaryResponse(item))
}

func parseOptionalQueryBool(r *http.Request, name string) (bool, *domain.AppError) {
	value := strings.TrimSpace(r.URL.Query().Get(name))
	if value == "" {
		return false, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Query invalid", "筛选参数格式不正确。", name, "invalid", "必须使用 true 或 false。")
	}
	return parsed, nil
}

func parseAPIQuotaTime(field, value string) (time.Time, *domain.AppError) {
	parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Time invalid", "时间格式不正确。", field, "invalid", "必须使用 RFC3339 时间。")
	}
	return parsed.UTC(), nil
}

func decodeCredentialMultipart(w http.ResponseWriter, r *http.Request) ([]byte, string, []byte, *domain.AppError) {
	body, reader, appErr := readAPIQuotaMultipart(w, r)
	if appErr != nil {
		return nil, "", nil, appErr
	}
	deliveryKind := ""
	var file []byte
	for {
		part, partErr := reader.NextPart()
		if partErr == io.EOF {
			break
		}
		if partErr != nil {
			return nil, "", nil, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Multipart invalid", "上传内容格式不正确。", "file", "invalid", "上传内容格式不正确。")
		}
		data, readErr := io.ReadAll(io.LimitReader(part, 5*1024*1024+1))
		_ = part.Close()
		if readErr != nil {
			return nil, "", nil, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Upload invalid", "读取上传内容失败。", "file", "invalid", "读取上传内容失败。")
		}
		switch part.FormName() {
		case "deliveryKind":
			deliveryKind = strings.TrimSpace(string(data))
		case "file":
			if len(data) > 5*1024*1024 {
				return nil, "", nil, domain.NewFieldError(http.StatusRequestEntityTooLarge, domain.CodeValidationFailed, "CSV too large", "CSV 文件不能超过 5 MiB。", "file", "too_large", "CSV 文件不能超过 5 MiB。")
			}
			file = append([]byte(nil), data...)
		}
	}
	if deliveryKind == "" || len(file) == 0 {
		return nil, "", nil, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Upload incomplete", "必须同时提交交付类型和 CSV 文件。", "file", "required", "必须同时提交交付类型和 CSV 文件。")
	}
	return body, deliveryKind, file, nil
}

func decodeRushOfferMultipart(w http.ResponseWriter, r *http.Request) ([]byte, apiQuotaRushOfferRequest, []byte, *domain.AppError) {
	var zero apiQuotaRushOfferRequest
	body, reader, appErr := readAPIQuotaMultipart(w, r)
	if appErr != nil {
		return nil, zero, nil, appErr
	}
	var payload, file []byte
	for {
		part, partErr := reader.NextPart()
		if partErr == io.EOF {
			break
		}
		if partErr != nil {
			return nil, zero, nil, invalidRushOfferMultipart("上传内容格式不正确。")
		}
		data, readErr := io.ReadAll(io.LimitReader(part, 5*1024*1024+1))
		name := part.FormName()
		_ = part.Close()
		if readErr != nil || len(data) > 5*1024*1024 {
			return nil, zero, nil, invalidRushOfferMultipart("上传内容无效或超过 5 MiB。")
		}
		switch name {
		case "payload":
			if payload != nil {
				return nil, zero, nil, invalidRushOfferMultipart("payload 只能提交一次。")
			}
			payload = append([]byte(nil), data...)
		case "file":
			if file != nil {
				return nil, zero, nil, invalidRushOfferMultipart("凭据 CSV 只能上传一个文件。")
			}
			file = append([]byte(nil), data...)
		default:
			return nil, zero, nil, invalidRushOfferMultipart("上传内容包含未知字段。")
		}
	}
	if len(bytes.TrimSpace(payload)) == 0 {
		return nil, zero, nil, invalidRushOfferMultipart("必须提交限时包 payload。")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	var req apiQuotaRushOfferRequest
	if err := decoder.Decode(&req); err != nil {
		return nil, zero, nil, invalidRushOfferMultipart("payload JSON 格式不正确或包含未知字段。")
	}
	var trailing struct{}
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, zero, nil, invalidRushOfferMultipart("payload JSON 只能包含一个对象。")
	}
	return body, req, file, nil
}

func readAPIQuotaMultipart(w http.ResponseWriter, r *http.Request) ([]byte, *multipart.Reader, *domain.AppError) {
	const maxMultipartBytes = 6 * 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxMultipartBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, nil, domain.NewFieldError(http.StatusRequestEntityTooLarge, domain.CodeValidationFailed, "Upload too large", "上传内容不能超过 6 MiB。", "file", "too_large", "上传文件过大。")
	}
	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" || params["boundary"] == "" {
		return nil, nil, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Multipart required", "必须使用 multipart/form-data 提交。", "payload", "invalid", "请求格式不正确。")
	}
	return body, multipart.NewReader(bytes.NewReader(body), params["boundary"]), nil
}

func invalidRushOfferMultipart(message string) *domain.AppError {
	return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Rush offer upload invalid", message, "payload", "invalid", message)
}

func apiQuotaRushOfferCompletionBuilder(publication apiquota.RushOfferPublication) (idempotency.Completion, *domain.AppError) {
	body, err := json.Marshal(apiQuotaRushOfferResponse{
		Batch:              toAPIQuotaBatchResponse(publication.Batch),
		Offer:              toAPIQuotaOfferResponse(publication.Offer),
		Round:              toAPIQuotaRoundResponse(publication.Round),
		CredentialImported: publication.CredentialImported,
		CredentialSummary:  toAPIQuotaCredentialSummaryResponse(publication.CredentialSummary),
	})
	if err != nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
	}
	return idempotency.Completion{
		Status: http.StatusCreated, ContentType: "application/json; charset=utf-8", Body: body,
		ResourceType: "api_quota_offer", ResourceID: publication.Offer.ID,
	}, nil
}

func toAPIQuotaBatchResponse(item apiquota.Batch) apiQuotaBatchResponse {
	return apiQuotaBatchResponse{
		ID: item.ID, APIServiceID: item.APIServiceID, SourceType: item.SourceType, SourceLabel: item.SourceLabel,
		Status: item.Status, DeclaredTotalUSDAllowance: item.DeclaredTotalUSDAllowance,
		UnallocatedUSDAllowance: item.UnallocatedUSDAllowance, SaleCutoffAt: item.SaleCutoffAt.UTC().Format(time.RFC3339),
		ExpiresAt: item.ExpiresAt.UTC().Format(time.RFC3339), SourceConfirmedAt: item.SourceConfirmedAt.UTC().Format(time.RFC3339),
		PublishedAt: formatOptionalTime(item.PublishedAt), Version: item.Version,
	}
}

func toAPIQuotaOfferResponse(item apiquota.Offer) apiQuotaOfferResponse {
	return apiQuotaOfferResponse{
		ID: item.ID, BatchID: item.BatchID, APIServiceID: item.APIServiceID, DistributionSystem: item.DistributionSystem,
		Name: item.Name, USDAllowance: item.USDAllowance, PriceCNY: item.PriceCNY, CNYPerUSD: item.CNYPerUSD,
		ModelMultiplier: item.ModelMultiplier, QuotaUsagePolicy: toAPIQuotaUsagePolicyResponse(item.QuotaUsagePolicy), DeliveryMode: item.DeliveryMode, DeliveryETAMinutes: item.DeliveryETAMinutes,
		SaleMode: item.SaleMode, Status: item.Status, SortOrder: item.SortOrder,
		PublishedAt: formatOptionalTime(item.PublishedAt), Version: item.Version,
	}
}

func toAPIQuotaRoundResponse(item apiquota.SaleRound) apiQuotaRoundResponse {
	allocations := make([]apiQuotaAllocationResponse, 0, len(item.Allocations))
	for _, allocation := range item.Allocations {
		allocations = append(allocations, apiQuotaAllocationResponse{
			ID: allocation.ID, OfferID: allocation.OfferID, SaleRoundID: allocation.SaleRoundID,
			SaleMode: allocation.SaleMode, CopyLimit: allocation.CopyLimit, AvailableCopies: allocation.AvailableCopies,
			ReservedCopies: allocation.ReservedCopies, ConsumedCopies: allocation.ConsumedCopies,
			AllocatedUSDAllowance: allocation.AllocatedUSDAllowance, ReturnedUSDAllowance: allocation.ReturnedUSDAllowance,
			Status: allocation.Status,
		})
	}
	return apiQuotaRoundResponse{
		ID: item.ID, BatchID: item.BatchID, SystemSlotKey: item.SystemSlotKey, Name: item.Name,
		StartsAt: item.StartsAt.UTC().Format(time.RFC3339), EndsAt: item.EndsAt.UTC().Format(time.RFC3339),
		Status: item.Status, Allocations: allocations, Version: item.Version,
	}
}

func toPublicAPIQuotaOfferResponse(item apiquota.OfferCard) publicAPIQuotaOfferResponse {
	return toPublicAPIQuotaOfferResponseWithHealth(item, apihealth.BuildSummary(nil, nil, time.Now().UTC()))
}

func toPublicAPIQuotaOfferResponseWithHealth(item apiquota.OfferCard, health apihealth.Summary) publicAPIQuotaOfferResponse {
	var currentRound, nextRound *apiQuotaRoundResponse
	if item.CurrentRound != nil {
		value := toAPIQuotaRoundResponse(*item.CurrentRound)
		currentRound = &value
	}
	if item.NextRound != nil {
		value := toAPIQuotaRoundResponse(*item.NextRound)
		nextRound = &value
	}
	return publicAPIQuotaOfferResponse{
		apiQuotaOfferResponse: toAPIQuotaOfferResponse(item.Offer),
		BatchStatus:           item.BatchStatus, ServiceTitle: item.ServiceTitle,
		SellerDisplayName: item.SellerDisplayName, SellerIdentityType: item.SellerIdentityType,
		SellerLinuxDOBound:     item.SellerLinuxDOBound,
		DeclaredMaxConcurrency: item.DeclaredMaxConcurrency, HealthSummary: toAPIServiceHealthSummaryResponse(health),
		SaleCutoffAt: item.SaleCutoffAt.UTC().Format(time.RFC3339),
		ExpiresAt:    item.ExpiresAt.UTC().Format(time.RFC3339), CurrentRound: currentRound, NextRound: nextRound,
		AvailableCopies: item.AvailableCopies, CredentialAvailableCopies: item.CredentialAvailableCopies,
		IsOrderable: item.IsOrderable, OrderabilityCode: item.OrderabilityCode, OrderabilityReason: item.OrderabilityReason,
	}
}

func toAPIQuotaCredentialSummaryResponse(item apiquota.CredentialSummary) apiQuotaCredentialSummaryResponse {
	return apiQuotaCredentialSummaryResponse{
		OfferID: item.OfferID, Available: item.Available, Reserved: item.Reserved,
		Delivered: item.Delivered, Retired: item.Retired,
	}
}
