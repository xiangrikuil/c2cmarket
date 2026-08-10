package server

import (
	"encoding/json"
	"net/http"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/catalog"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/go-chi/chi/v5"
)

type productCategoryResponse struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	DisplayName string `json:"displayName"`
	IconDataURL string `json:"iconDataUrl"`
	SortOrder   int    `json:"sortOrder"`
	Active      bool   `json:"active"`
}

type productCategoryRequest struct {
	Code        string `json:"code"`
	DisplayName string `json:"displayName"`
	IconDataURL string `json:"iconDataUrl"`
	SortOrder   int    `json:"sortOrder"`
	Active      bool   `json:"active"`
}

type productPlanResponse struct {
	ID                   string `json:"id"`
	CategoryID           string `json:"categoryId"`
	CategoryCode         string `json:"categoryCode"`
	ProviderCode         string `json:"providerCode"`
	Slug                 string `json:"slug"`
	DisplayName          string `json:"displayName"`
	Description          string `json:"description"`
	PublishPolicy        string `json:"publishPolicy"`
	AccessMode           string `json:"accessMode"`
	ProviderPolicyStatus string `json:"providerPolicyStatus"`
	RiskLevel            string `json:"riskLevel"`
	RiskAckRequired      bool   `json:"riskAckRequired"`
	RiskNoticeCode       string `json:"riskNoticeCode,omitempty"`
	PolicyVersion        int64  `json:"policyVersion"`
	PolicyNote           string `json:"policyNote"`
	QuotaLabel           string `json:"quotaLabel"`
	QuotaUnit            string `json:"quotaUnit"`
	QuotaPeriod          string `json:"quotaPeriod"`
	Active               bool   `json:"active"`
	AllowCustomVariant   bool   `json:"allowCustomVariant"`
	SortOrder            int    `json:"sortOrder"`
	CreatedAt            string `json:"createdAt"`
	UpdatedAt            string `json:"updatedAt"`
}

type productPlanRequest struct {
	CategoryID           string `json:"categoryId"`
	ProviderCode         string `json:"providerCode"`
	Slug                 string `json:"slug"`
	DisplayName          string `json:"displayName"`
	Description          string `json:"description"`
	PublishPolicy        string `json:"publishPolicy"`
	AccessMode           string `json:"accessMode"`
	ProviderPolicyStatus string `json:"providerPolicyStatus"`
	RiskLevel            string `json:"riskLevel"`
	RiskAckRequired      bool   `json:"riskAckRequired"`
	RiskNoticeCode       string `json:"riskNoticeCode"`
	PolicyNote           string `json:"policyNote"`
	QuotaLabel           string `json:"quotaLabel"`
	QuotaUnit            string `json:"quotaUnit"`
	QuotaPeriod          string `json:"quotaPeriod"`
	Active               bool   `json:"active"`
	AllowCustomVariant   bool   `json:"allowCustomVariant"`
	SortOrder            int    `json:"sortOrder"`
}

type apiModelResponse struct {
	ID                         string   `json:"id"`
	ProviderID                 string   `json:"providerId"`
	ProviderCategory           string   `json:"providerCategory"`
	ProviderCode               string   `json:"providerCode"`
	Provider                   string   `json:"provider"`
	ProviderActive             bool     `json:"providerActive"`
	ModelKey                   string   `json:"modelKey"`
	Capabilities               []string `json:"capabilities"`
	Active                     bool     `json:"active"`
	CurrentPriceVersionID      string   `json:"currentPriceVersionId,omitempty"`
	CurrentPriceSourceURL      string   `json:"currentPriceSourceUrl,omitempty"`
	CurrentPriceSourceVersion  string   `json:"currentPriceSourceVersion,omitempty"`
	CurrentPriceValidFrom      string   `json:"currentPriceValidFrom,omitempty"`
	InputPricePerMillion       string   `json:"inputPricePerMillion,omitempty"`
	CachedInputPricePerMillion string   `json:"cachedInputPricePerMillion,omitempty"`
	OutputPricePerMillion      string   `json:"outputPricePerMillion,omitempty"`
	SortOrder                  int      `json:"sortOrder"`
	CreatedAt                  string   `json:"createdAt"`
	UpdatedAt                  string   `json:"updatedAt"`
}

type apiModelRequest struct {
	ProviderID            string   `json:"providerId"`
	ModelKey              string   `json:"modelKey"`
	Capabilities          []string `json:"capabilities"`
	InputTokenPrice       string   `json:"inputTokenPrice"`
	CachedInputTokenPrice string   `json:"cachedInputTokenPrice"`
	OutputTokenPrice      string   `json:"outputTokenPrice"`
	SourceURL             string   `json:"sourceUrl"`
	SourceVersion         string   `json:"sourceVersion"`
	Active                bool     `json:"active"`
	SortOrder             int      `json:"sortOrder"`
}

type apiModelProviderResponse struct {
	ID               string `json:"id"`
	ProviderCategory string `json:"providerCategory"`
	Code             string `json:"code"`
	DisplayName      string `json:"displayName"`
	Active           bool   `json:"active"`
	SortOrder        int    `json:"sortOrder"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

type apiModelProviderRequest struct {
	ProviderCategory string `json:"providerCategory"`
	Code             string `json:"code"`
	DisplayName      string `json:"displayName"`
	Active           bool   `json:"active"`
	SortOrder        int    `json:"sortOrder"`
}

type apiModelSyncPreviewRequest struct {
	ProviderIDs []string `json:"providerIds"`
}

type apiModelSyncPreviewResponse struct {
	Fingerprint string                     `json:"fingerprint"`
	FetchedAt   string                     `json:"fetchedAt"`
	Counts      apiModelSyncCountsResponse `json:"counts"`
	Items       []apiModelSyncItemResponse `json:"items"`
}

type apiModelSyncCountsResponse struct {
	New           int `json:"new"`
	PriceChanged  int `json:"priceChanged"`
	Unchanged     int `json:"unchanged"`
	SourceMissing int `json:"sourceMissing"`
	Unavailable   int `json:"unavailable"`
}

type apiModelSyncItemResponse struct {
	CandidateKey                    string   `json:"candidateKey"`
	Fingerprint                     string   `json:"fingerprint"`
	Status                          string   `json:"status"`
	ReasonCode                      string   `json:"reasonCode,omitempty"`
	Reason                          string   `json:"reason,omitempty"`
	ProviderID                      string   `json:"providerId"`
	ProviderCode                    string   `json:"providerCode"`
	Provider                        string   `json:"provider"`
	ModelKey                        string   `json:"modelKey"`
	Capabilities                    []string `json:"capabilities"`
	SourceURL                       string   `json:"sourceUrl,omitempty"`
	SourceVersion                   string   `json:"sourceVersion,omitempty"`
	InputPricePerMillion            string   `json:"inputPricePerMillion,omitempty"`
	CachedInputPricePerMillion      string   `json:"cachedInputPricePerMillion,omitempty"`
	OutputPricePerMillion           string   `json:"outputPricePerMillion,omitempty"`
	LocalModelID                    string   `json:"localModelId,omitempty"`
	LocalPriceVersionID             string   `json:"localPriceVersionId,omitempty"`
	LocalInputPricePerMillion       string   `json:"localInputPricePerMillion,omitempty"`
	LocalCachedInputPricePerMillion string   `json:"localCachedInputPricePerMillion,omitempty"`
	LocalOutputPricePerMillion      string   `json:"localOutputPricePerMillion,omitempty"`
	LocalSourceURL                  string   `json:"localSourceUrl,omitempty"`
	LocalSourceVersion              string   `json:"localSourceVersion,omitempty"`
}

type apiModelSyncApplyRequest struct {
	Items []apiModelSyncSelectionRequest `json:"items"`
}

type apiModelSyncSelectionRequest struct {
	Fingerprint                string   `json:"fingerprint"`
	Status                     string   `json:"status"`
	ProviderID                 string   `json:"providerId"`
	ProviderCode               string   `json:"providerCode"`
	ModelKey                   string   `json:"modelKey"`
	Capabilities               []string `json:"capabilities"`
	SourceURL                  string   `json:"sourceUrl"`
	SourceVersion              string   `json:"sourceVersion"`
	InputPricePerMillion       string   `json:"inputPricePerMillion"`
	CachedInputPricePerMillion string   `json:"cachedInputPricePerMillion"`
	OutputPricePerMillion      string   `json:"outputPricePerMillion"`
	LocalModelID               string   `json:"localModelId"`
	LocalPriceVersionID        string   `json:"localPriceVersionId"`
	Active                     bool     `json:"active"`
}

type apiModelBulkStatusRequest struct {
	ModelIDs []string `json:"modelIds"`
	Active   bool     `json:"active"`
}

type apiModelBulkMutationResponse struct {
	Created int      `json:"created"`
	Updated int      `json:"updated"`
	Changed int      `json:"changed"`
	IDs     []string `json:"ids"`
}

func (s *Server) handleProductCategories(w http.ResponseWriter, r *http.Request) {
	categories, appErr := s.app.ProductCategories(r.Context())
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items := make([]productCategoryResponse, 0, len(categories))
	for _, category := range categories {
		items = append(items, toProductCategoryResponse(category))
	}
	writeJSON(w, http.StatusOK, listResponse[productCategoryResponse]{Items: items})
}

func (s *Server) handleAdminProductCategories(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	categories, appErr := s.app.AdminProductCategories(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items := make([]productCategoryResponse, 0, len(categories))
	for _, category := range categories {
		items = append(items, toProductCategoryResponse(category))
	}
	writeJSON(w, http.StatusOK, listResponse[productCategoryResponse]{Items: items})
}

func (s *Server) handleAdminProductCategory(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	category, appErr := s.app.AdminProductCategory(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toProductCategoryResponse(category))
}

func (s *Server) handleCreateProductCategory(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	req, appErr := decodeStrictJSONOnly[productCategoryRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	category, appErr := s.app.CreateProductCategory(r.Context(), user, productCategoryInputFromRequest(req))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusCreated, toProductCategoryResponse(category))
}

func (s *Server) handleUpdateProductCategory(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	req, appErr := decodeStrictJSONOnly[productCategoryRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	category, appErr := s.app.UpdateProductCategory(r.Context(), user, chi.URLParam(r, "id"), productCategoryInputFromRequest(req))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toProductCategoryResponse(category))
}

func (s *Server) handleActivateProductCategory(w http.ResponseWriter, r *http.Request) {
	s.handleSetProductCategoryActive(w, r, true)
}

func (s *Server) handleDeactivateProductCategory(w http.ResponseWriter, r *http.Request) {
	s.handleSetProductCategoryActive(w, r, false)
}

func (s *Server) handleSetProductCategoryActive(w http.ResponseWriter, r *http.Request, active bool) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	category, appErr := s.app.SetProductCategoryActive(r.Context(), user, chi.URLParam(r, "id"), active)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toProductCategoryResponse(category))
}

func (s *Server) handleProductPlans(w http.ResponseWriter, r *http.Request) {
	plans, appErr := s.app.ProductPlans(r.Context(), r.URL.Query().Get("category"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[productPlanResponse]{Items: toProductPlanResponses(plans)})
}

func (s *Server) handleProductPlan(w http.ResponseWriter, r *http.Request) {
	plan, appErr := s.app.ProductPlan(r.Context(), chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toProductPlanResponse(plan))
}

func (s *Server) handleAdminProductPlans(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	plans, appErr := s.app.AdminProductPlans(r.Context(), user, r.URL.Query().Get("category"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[productPlanResponse]{Items: toProductPlanResponses(plans)})
}

func (s *Server) handleAdminProductPlan(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	plan, appErr := s.app.AdminProductPlan(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toProductPlanResponse(plan))
}

func (s *Server) handleCreateProductPlan(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	req, appErr := decodeStrictJSONOnly[productPlanRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	plan, appErr := s.app.CreateProductPlan(r.Context(), user, productPlanInputFromRequest(req))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusCreated, toProductPlanResponse(plan))
}

func (s *Server) handleUpdateProductPlan(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	req, appErr := decodeStrictJSONOnly[productPlanRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	plan, appErr := s.app.UpdateProductPlan(r.Context(), user, chi.URLParam(r, "id"), productPlanInputFromRequest(req))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toProductPlanResponse(plan))
}

func (s *Server) handleActivateProductPlan(w http.ResponseWriter, r *http.Request) {
	s.handleSetProductPlanActive(w, r, true)
}

func (s *Server) handleDeactivateProductPlan(w http.ResponseWriter, r *http.Request) {
	s.handleSetProductPlanActive(w, r, false)
}

func (s *Server) handleSetProductPlanActive(w http.ResponseWriter, r *http.Request, active bool) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	plan, appErr := s.app.SetProductPlanActive(r.Context(), user, chi.URLParam(r, "id"), active)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toProductPlanResponse(plan))
}

func (s *Server) handleAPIModels(w http.ResponseWriter, r *http.Request) {
	models, appErr := s.app.APIModels(r.Context())
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[apiModelResponse]{Items: toAPIModelResponses(models)})
}

func (s *Server) handleAPIModel(w http.ResponseWriter, r *http.Request) {
	model, appErr := s.app.APIModel(r.Context(), chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toAPIModelResponse(model))
}

func (s *Server) handleAdminAPIModelProviders(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	providers, appErr := s.app.AdminAPIModelProviders(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[apiModelProviderResponse]{Items: toAPIModelProviderResponses(providers)})
}

func (s *Server) handleAdminAPIModelProvider(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	provider, appErr := s.app.AdminAPIModelProvider(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toAPIModelProviderResponse(provider))
}

func (s *Server) handleCreateAPIModelProvider(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	req, appErr := decodeStrictJSONOnly[apiModelProviderRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	provider, appErr := s.app.CreateAPIModelProvider(r.Context(), user, apiModelProviderInputFromRequest(req))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusCreated, toAPIModelProviderResponse(provider))
}

func (s *Server) handleUpdateAPIModelProvider(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	req, appErr := decodeStrictJSONOnly[apiModelProviderRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	provider, appErr := s.app.UpdateAPIModelProvider(r.Context(), user, chi.URLParam(r, "id"), apiModelProviderInputFromRequest(req))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toAPIModelProviderResponse(provider))
}

func (s *Server) handleActivateAPIModelProvider(w http.ResponseWriter, r *http.Request) {
	s.handleSetAPIModelProviderActive(w, r, true)
}

func (s *Server) handleDeactivateAPIModelProvider(w http.ResponseWriter, r *http.Request) {
	s.handleSetAPIModelProviderActive(w, r, false)
}

func (s *Server) handleSetAPIModelProviderActive(w http.ResponseWriter, r *http.Request, active bool) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	provider, appErr := s.app.SetAPIModelProviderActive(r.Context(), user, chi.URLParam(r, "id"), active)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toAPIModelProviderResponse(provider))
}

func (s *Server) handleAdminAPIModels(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	models, appErr := s.app.AdminAPIModels(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[apiModelResponse]{Items: toAPIModelResponses(models)})
}

func (s *Server) handleAdminAPIModel(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	model, appErr := s.app.AdminAPIModel(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toAPIModelResponse(model))
}

func (s *Server) handlePreviewAPIModelSync(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	request, appErr := decodeStrictJSONOnly[apiModelSyncPreviewRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	preview, appErr := s.app.PreviewAPIModelSync(r.Context(), user, catalog.APIModelSyncPreviewInput{ProviderIDs: request.ProviderIDs})
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toAPIModelSyncPreviewResponse(preview))
}

func (s *Server) handleApplyAPIModelSync(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, request, appErr := decodeStrictJSON[apiModelSyncApplyRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	routeKey := "POST /api/v1/admin/api-models/models-dev/apply"
	completion, appErr := s.app.ApplyAPIModelSyncWithIdempotency(
		r.Context(), user, routeKey, r.Header.Get("Idempotency-Key"),
		requestHash(r.Method, routeKey, body),
		catalog.APIModelSyncApplyInput{Items: apiModelSyncSelectionsFromRequest(request.Items)},
		apiModelBulkMutationCompletionBuilder,
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleBulkAPIModelStatus(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, request, appErr := decodeStrictJSON[apiModelBulkStatusRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	routeKey := "POST /api/v1/admin/api-models/bulk-status"
	completion, appErr := s.app.SetAPIModelsActiveWithIdempotency(
		r.Context(), user, routeKey, r.Header.Get("Idempotency-Key"),
		requestHash(r.Method, routeKey, body),
		catalog.APIModelBulkStatusInput{ModelIDs: request.ModelIDs, Active: request.Active},
		apiModelBulkMutationCompletionBuilder,
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleCreateAPIModel(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	req, appErr := decodeStrictJSONOnly[apiModelRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	model, appErr := s.app.CreateAPIModel(r.Context(), user, apiModelInputFromRequest(req))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusCreated, toAPIModelResponse(model))
}

func (s *Server) handleUpdateAPIModel(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	req, appErr := decodeStrictJSONOnly[apiModelRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	model, appErr := s.app.UpdateAPIModel(r.Context(), user, chi.URLParam(r, "id"), apiModelInputFromRequest(req))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toAPIModelResponse(model))
}

func (s *Server) handleActivateAPIModel(w http.ResponseWriter, r *http.Request) {
	s.handleSetAPIModelActive(w, r, true)
}

func (s *Server) handleDeactivateAPIModel(w http.ResponseWriter, r *http.Request) {
	s.handleSetAPIModelActive(w, r, false)
}

func (s *Server) handleSetAPIModelActive(w http.ResponseWriter, r *http.Request, active bool) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	model, appErr := s.app.SetAPIModelActive(r.Context(), user, chi.URLParam(r, "id"), active)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toAPIModelResponse(model))
}

func toProductCategoryResponse(category catalog.ProductCategory) productCategoryResponse {
	return productCategoryResponse{
		ID:          category.ID,
		Code:        category.Code,
		DisplayName: category.DisplayName,
		IconDataURL: category.IconDataURL,
		SortOrder:   category.SortOrder,
		Active:      category.Active,
	}
}

func productCategoryInputFromRequest(req productCategoryRequest) catalog.ProductCategoryInput {
	return catalog.ProductCategoryInput{
		Code:        req.Code,
		DisplayName: req.DisplayName,
		IconDataURL: req.IconDataURL,
		SortOrder:   req.SortOrder,
		Active:      req.Active,
	}
}

func toProductPlanResponses(plans []catalog.ProductPlan) []productPlanResponse {
	items := make([]productPlanResponse, 0, len(plans))
	for _, plan := range plans {
		items = append(items, toProductPlanResponse(plan))
	}
	return items
}

func toProductPlanResponse(plan catalog.ProductPlan) productPlanResponse {
	return productPlanResponse{
		ID:                   plan.ID,
		CategoryID:           plan.CategoryID,
		CategoryCode:         plan.CategoryCode,
		ProviderCode:         plan.ProviderCode,
		Slug:                 plan.Slug,
		DisplayName:          plan.DisplayName,
		Description:          plan.Description,
		PublishPolicy:        plan.PublishPolicy,
		AccessMode:           plan.AccessMode,
		ProviderPolicyStatus: plan.ProviderPolicyStatus,
		RiskLevel:            plan.RiskLevel,
		RiskAckRequired:      plan.RiskAckRequired,
		RiskNoticeCode:       plan.RiskNoticeCode,
		PolicyVersion:        plan.PolicyVersion,
		PolicyNote:           plan.PolicyNote,
		QuotaLabel:           plan.QuotaLabel,
		QuotaUnit:            plan.QuotaUnit,
		QuotaPeriod:          plan.QuotaPeriod,
		Active:               plan.Active,
		AllowCustomVariant:   plan.AllowCustomVariant,
		SortOrder:            plan.SortOrder,
		CreatedAt:            plan.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:            plan.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func productPlanInputFromRequest(req productPlanRequest) catalog.ProductPlanInput {
	return catalog.ProductPlanInput{
		CategoryID:           req.CategoryID,
		ProviderCode:         req.ProviderCode,
		Slug:                 req.Slug,
		DisplayName:          req.DisplayName,
		Description:          req.Description,
		PublishPolicy:        req.PublishPolicy,
		AccessMode:           req.AccessMode,
		ProviderPolicyStatus: req.ProviderPolicyStatus,
		RiskLevel:            req.RiskLevel,
		RiskAckRequired:      req.RiskAckRequired,
		RiskNoticeCode:       req.RiskNoticeCode,
		PolicyNote:           req.PolicyNote,
		QuotaLabel:           req.QuotaLabel,
		QuotaUnit:            req.QuotaUnit,
		QuotaPeriod:          req.QuotaPeriod,
		Active:               req.Active,
		AllowCustomVariant:   req.AllowCustomVariant,
		SortOrder:            req.SortOrder,
	}
}

func toAPIModelResponses(models []catalog.APIModelCatalog) []apiModelResponse {
	items := make([]apiModelResponse, 0, len(models))
	for _, model := range models {
		items = append(items, toAPIModelResponse(model))
	}
	return items
}

func toAPIModelSyncPreviewResponse(preview catalog.APIModelSyncPreview) apiModelSyncPreviewResponse {
	items := make([]apiModelSyncItemResponse, 0, len(preview.Items))
	for _, item := range preview.Items {
		items = append(items, apiModelSyncItemResponse{
			CandidateKey: item.CandidateKey, Fingerprint: item.Fingerprint, Status: item.Status,
			ReasonCode: item.ReasonCode, Reason: item.Reason, ProviderID: item.ProviderID,
			ProviderCode: item.ProviderCode, Provider: item.Provider, ModelKey: item.ModelKey,
			Capabilities: append([]string(nil), item.Capabilities...), SourceURL: item.SourceURL,
			SourceVersion: item.SourceVersion, InputPricePerMillion: item.InputPricePerMillion,
			CachedInputPricePerMillion: item.CachedInputPricePerMillion,
			OutputPricePerMillion:      item.OutputPricePerMillion, LocalModelID: item.LocalModelID,
			LocalPriceVersionID:             item.LocalPriceVersionID,
			LocalInputPricePerMillion:       item.LocalInputPricePerMillion,
			LocalCachedInputPricePerMillion: item.LocalCachedInputPricePerMillion,
			LocalOutputPricePerMillion:      item.LocalOutputPricePerMillion,
			LocalSourceURL:                  item.LocalSourceURL, LocalSourceVersion: item.LocalSourceVersion,
		})
	}
	return apiModelSyncPreviewResponse{
		Fingerprint: preview.Fingerprint,
		FetchedAt:   preview.FetchedAt.UTC().Format(time.RFC3339),
		Counts: apiModelSyncCountsResponse{
			New: preview.Counts.New, PriceChanged: preview.Counts.PriceChanged,
			Unchanged: preview.Counts.Unchanged, SourceMissing: preview.Counts.SourceMissing,
			Unavailable: preview.Counts.Unavailable,
		},
		Items: items,
	}
}

func apiModelSyncSelectionsFromRequest(items []apiModelSyncSelectionRequest) []catalog.APIModelSyncSelection {
	result := make([]catalog.APIModelSyncSelection, 0, len(items))
	for _, item := range items {
		result = append(result, catalog.APIModelSyncSelection{
			Fingerprint: item.Fingerprint, Status: item.Status, ProviderID: item.ProviderID,
			ProviderCode: item.ProviderCode, ModelKey: item.ModelKey,
			Capabilities: append([]string(nil), item.Capabilities...), SourceURL: item.SourceURL,
			SourceVersion: item.SourceVersion, InputPricePerMillion: item.InputPricePerMillion,
			CachedInputPricePerMillion: item.CachedInputPricePerMillion,
			OutputPricePerMillion:      item.OutputPricePerMillion, LocalModelID: item.LocalModelID,
			LocalPriceVersionID: item.LocalPriceVersionID, Active: item.Active,
		})
	}
	return result
}

func apiModelBulkMutationCompletionBuilder(result catalog.APIModelBulkMutationResult) (idempotency.Completion, *domain.AppError) {
	if len(result.IDs) == 0 {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "API 模型目录批量操作缺少关联资源。")
	}
	body, err := json.Marshal(apiModelBulkMutationResponse{
		Created: result.Created, Updated: result.Updated, Changed: result.Changed,
		IDs: append([]string(nil), result.IDs...),
	})
	if err != nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "API 模型目录响应编码失败。")
	}
	return idempotency.Completion{
		Status: http.StatusOK, ContentType: "application/json; charset=utf-8", Body: body,
		ResourceType: "api_model_catalog", ResourceID: result.IDs[0],
	}, nil
}

func toAPIModelResponse(model catalog.APIModelCatalog) apiModelResponse {
	validFrom := ""
	if model.CurrentPriceValidFrom != nil {
		validFrom = model.CurrentPriceValidFrom.UTC().Format(time.RFC3339)
	}
	return apiModelResponse{
		ID:                         model.ID,
		ProviderID:                 model.ProviderID,
		ProviderCategory:           model.ProviderCategory,
		ProviderCode:               model.ProviderCode,
		Provider:                   model.Provider,
		ProviderActive:             model.ProviderActive,
		ModelKey:                   model.ModelKey,
		Capabilities:               model.Capabilities,
		Active:                     model.Active,
		CurrentPriceVersionID:      model.CurrentPriceVersionID,
		CurrentPriceSourceURL:      model.CurrentPriceSourceURL,
		CurrentPriceSourceVersion:  model.CurrentPriceSourceVersion,
		CurrentPriceValidFrom:      validFrom,
		InputPricePerMillion:       model.InputPricePerMillion,
		CachedInputPricePerMillion: model.CachedInputPricePerMillion,
		OutputPricePerMillion:      model.OutputPricePerMillion,
		SortOrder:                  model.SortOrder,
		CreatedAt:                  model.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:                  model.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func apiModelInputFromRequest(req apiModelRequest) catalog.APIModelInput {
	return catalog.APIModelInput{
		ProviderID:            req.ProviderID,
		ModelKey:              req.ModelKey,
		Capabilities:          req.Capabilities,
		SourceURL:             req.SourceURL,
		SourceVersion:         req.SourceVersion,
		InputTokenPrice:       req.InputTokenPrice,
		CachedInputTokenPrice: req.CachedInputTokenPrice,
		OutputTokenPrice:      req.OutputTokenPrice,
		Active:                req.Active,
		SortOrder:             req.SortOrder,
	}
}

func toAPIModelProviderResponses(providers []catalog.APIModelProvider) []apiModelProviderResponse {
	items := make([]apiModelProviderResponse, 0, len(providers))
	for _, provider := range providers {
		items = append(items, toAPIModelProviderResponse(provider))
	}
	return items
}

func toAPIModelProviderResponse(provider catalog.APIModelProvider) apiModelProviderResponse {
	return apiModelProviderResponse{
		ID:               provider.ID,
		ProviderCategory: provider.ProviderCategory,
		Code:             provider.Code,
		DisplayName:      provider.DisplayName,
		Active:           provider.Active,
		SortOrder:        provider.SortOrder,
		CreatedAt:        provider.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:        provider.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func apiModelProviderInputFromRequest(req apiModelProviderRequest) catalog.APIModelProviderInput {
	return catalog.APIModelProviderInput{
		ProviderCategory: req.ProviderCategory,
		Code:             req.Code,
		DisplayName:      req.DisplayName,
		Active:           req.Active,
		SortOrder:        req.SortOrder,
	}
}
