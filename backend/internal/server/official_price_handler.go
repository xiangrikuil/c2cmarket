package server

import (
	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/officialprice"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"net/http"
	"time"
)

type submitLeadRequest struct {
	ProductPlanID     string `json:"productPlanId"`
	ProductText       string `json:"productText"`
	PlanText          string `json:"planText"`
	RegionCode        string `json:"regionCode"`
	Channel           string `json:"channel"`
	OpeningMethod     string `json:"openingMethod"`
	SourceURL         string `json:"sourceUrl"`
	SourceTitle       string `json:"sourceTitle"`
	EvidenceSummary   string `json:"evidenceSummary"`
	Note              string `json:"note"`
	ObservedAt        string `json:"observedAt"`
	BillingPeriod     string `json:"billingPeriod"`
	Currency          string `json:"currency"`
	OriginalAmount    string `json:"originalAmount"`
	OriginalPriceText string `json:"originalPriceText"`
	TaxIncluded       bool   `json:"taxIncluded"`
}

type leadResponse struct {
	ID                   string  `json:"id"`
	Status               string  `json:"status"`
	ProductPlanID        string  `json:"productPlanId,omitempty"`
	ProductText          string  `json:"productText"`
	RegionCode           string  `json:"regionCode"`
	SourceURL            string  `json:"sourceUrl"`
	ObservedAt           string  `json:"observedAt"`
	BillingPeriod        string  `json:"billingPeriod"`
	Currency             string  `json:"currency"`
	OriginalAmount       string  `json:"originalAmount"`
	NormalizedMonthlyCNY string  `json:"normalizedMonthlyCny,omitempty"`
	NormalizationStatus  string  `json:"normalizationStatus,omitempty"`
	DuplicateOfLeadID    *string `json:"duplicateOfLeadId"`
	Version              int64   `json:"version"`
	CreatedAt            string  `json:"createdAt"`
}

type leadOwnerResponse struct {
	leadResponse
	ReviewReason *string `json:"reviewReason"`
	ReviewedAt   *string `json:"reviewedAt"`
}

type leadAdminResponse struct {
	leadOwnerResponse
	SubmitterUserID   string `json:"submitterUserId"`
	PlanText          string `json:"planText,omitempty"`
	Channel           string `json:"channel"`
	OpeningMethod     string `json:"openingMethod"`
	SourceTitle       string `json:"sourceTitle,omitempty"`
	EvidenceSummary   string `json:"evidenceSummary"`
	Note              string `json:"note,omitempty"`
	PriceUnit         string `json:"priceUnit"`
	OriginalPriceText string `json:"originalPriceText"`
	TaxIncluded       bool   `json:"taxIncluded"`
	FXRate            string `json:"fxRate,omitempty"`
	FXSource          string `json:"fxSource,omitempty"`
	FXObservedAt      string `json:"fxObservedAt,omitempty"`
	OfferKey          string `json:"offerKey,omitempty"`
	DuplicateOfLeadID string `json:"duplicateOfLeadId,omitempty"`
	ReviewedByAdminID string `json:"reviewedByAdminId,omitempty"`
	ConversionMode    string `json:"conversionMode,omitempty"`
	NormalizationRule string `json:"normalizationRule,omitempty"`
}

func (s *Server) handleSubmitOfficialPriceLead(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}

	body, req, appErr := decodeStrictJSON[submitLeadRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}

	s.withIdempotency(w, r, user.ID, "POST /api/v1/official-price-leads", body, func() (int, any, string, string, *domain.AppError) {
		observedAt, err := parseOptionalTime(req.ObservedAt)
		if err != nil {
			return 0, nil, "", "", domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Observed time invalid", "观察时间格式不正确。", "observedAt", "invalid", "观察时间必须是 ISO 8601。")
		}
		lead, errApp := s.app.SubmitOfficialPriceLead(r.Context(), user, officialprice.SubmitLeadInput{
			ProductPlanID:     req.ProductPlanID,
			ProductText:       req.ProductText,
			PlanText:          req.PlanText,
			RegionCode:        req.RegionCode,
			Channel:           req.Channel,
			OpeningMethod:     req.OpeningMethod,
			SourceURL:         req.SourceURL,
			SourceTitle:       req.SourceTitle,
			EvidenceSummary:   req.EvidenceSummary,
			Note:              req.Note,
			ObservedAt:        observedAt,
			BillingPeriod:     req.BillingPeriod,
			Currency:          req.Currency,
			OriginalAmount:    req.OriginalAmount,
			OriginalPriceText: req.OriginalPriceText,
			TaxIncluded:       req.TaxIncluded,
		})
		if errApp != nil {
			return 0, nil, "", "", errApp
		}
		return http.StatusCreated, toLeadResponse(lead), "official_price_lead", lead.ID, nil
	})
}

func (s *Server) handleMyOfficialPriceLeads(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	leads, appErr := s.app.MyOfficialPriceLeads(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[leadResponse]{Items: toLeadResponses(leads)})
}

func (s *Server) handleMyOfficialPriceLead(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	lead, appErr := s.app.MyOfficialPriceLead(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, lead.Version)
	writeJSON(w, http.StatusOK, toLeadOwnerResponse(lead))
}

func (s *Server) handleAdminOfficialPriceLeads(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	leads, appErr := s.app.AdminOfficialPriceLeads(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[leadResponse]{Items: toLeadResponses(leads)})
}

func (s *Server) handleAdminOfficialPriceLead(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	lead, appErr := s.app.AdminOfficialPriceLead(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, lead.Version)
	writeJSON(w, http.StatusOK, toLeadAdminResponse(lead))
}

type approveLeadRequest struct {
	Reason                string            `json:"reason"`
	ResolvedProductPlanID string            `json:"resolvedProductPlanId"`
	ValidFrom             string            `json:"validFrom"`
	FXSnapshot            fxSnapshotRequest `json:"fxSnapshot"`
}

type fxSnapshotRequest struct {
	RateToCNY  string `json:"rateToCny"`
	Source     string `json:"source"`
	ObservedAt string `json:"observedAt"`
}

type approveLeadResponse struct {
	Lead   leadResponse        `json:"lead"`
	Record priceRecordResponse `json:"record"`
}

type adminOfficialPriceRecordRequest struct {
	ProductPlanID  string `json:"productPlanId"`
	ProductText    string `json:"productText"`
	PlanText       string `json:"planText"`
	RegionCode     string `json:"regionCode"`
	Channel        string `json:"channel"`
	OpeningMethod  string `json:"openingMethod"`
	SourceURL      string `json:"sourceUrl"`
	ObservedAt     string `json:"observedAt"`
	BillingPeriod  string `json:"billingPeriod"`
	Currency       string `json:"currency"`
	OriginalAmount string `json:"originalAmount"`
	TaxIncluded    bool   `json:"taxIncluded"`
	FXRateToCNY    string `json:"fxRateToCny"`
	FXSource       string `json:"fxSource"`
	FXObservedAt   string `json:"fxObservedAt"`
	ValidFrom      string `json:"validFrom"`
	Reason         string `json:"reason"`
}

type adminOfficialPriceRecordActionRequest struct {
	Reason string `json:"reason"`
}

type priceRecordResponse struct {
	ID                   string  `json:"id"`
	LeadID               string  `json:"leadId"`
	ProductPlanID        string  `json:"productPlanId"`
	RegionCode           string  `json:"regionCode"`
	Channel              string  `json:"channel"`
	OpeningMethod        string  `json:"openingMethod"`
	SourceURL            string  `json:"sourceUrl"`
	Status               string  `json:"status"`
	ValidFrom            string  `json:"validFrom"`
	ValidTo              *string `json:"validTo"`
	ObservedAt           string  `json:"observedAt"`
	BillingPeriod        string  `json:"billingPeriod"`
	PriceUnit            string  `json:"priceUnit"`
	Currency             string  `json:"currency"`
	OriginalAmount       string  `json:"originalAmount"`
	TaxIncluded          bool    `json:"taxIncluded"`
	NormalizedMonthlyCNY string  `json:"normalizedMonthlyCny"`
	FXRate               string  `json:"fxRate"`
	FXSource             string  `json:"fxSource"`
	FXObservedAt         string  `json:"fxObservedAt"`
	OfferKey             string  `json:"offerKey"`
	IsLowestReference    bool    `json:"isLowestReference"`
	Version              int64   `json:"version"`
	CreatedAt            string  `json:"createdAt"`
}

func (s *Server) handleOfficialPrices(w http.ResponseWriter, r *http.Request) {
	records, appErr := s.app.PublicOfficialPriceRecords(r.Context())
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toPriceRecordResponses(records))
}

func (s *Server) handleOfficialPrice(w http.ResponseWriter, r *http.Request) {
	record, appErr := s.app.PublicOfficialPriceRecord(r.Context(), chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toPriceRecordResponse(record))
}

func (s *Server) handleAdminOfficialPriceRecords(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	records, appErr := s.app.AdminOfficialPriceRecords(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toPriceRecordResponses(records))
}

func (s *Server) handleAdminOfficialPriceRecord(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	record, appErr := s.app.AdminOfficialPriceRecord(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, record.Version)
	writeJSON(w, http.StatusOK, toPriceRecordResponse(record))
}

func (s *Server) handleCreateAdminOfficialPriceRecord(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[adminOfficialPriceRecordRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	input, appErr := toAdminOfficialPriceRecordInput(req, "", user.ID, 0, requestIDFrom(r))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	s.withIdempotency(w, r, user.ID, "POST /api/v1/admin/official-price-records", body, func() (int, any, string, string, *domain.AppError) {
		record, errApp := s.app.CreateAdminOfficialPriceRecord(r.Context(), user, input)
		if errApp != nil {
			return 0, nil, "", "", errApp
		}
		return http.StatusCreated, toPriceRecordResponse(record), "official_price_record", record.ID, nil
	})
}

func (s *Server) handleUpdateAdminOfficialPriceRecord(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[adminOfficialPriceRecordRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	recordID := chi.URLParam(r, "id")
	input, appErr := toAdminOfficialPriceRecordInput(req, recordID, user.ID, version, requestIDFrom(r))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	routeKey := "PUT /api/v1/admin/official-price-records/{id}:" + recordID
	s.withIdempotency(w, r, user.ID, routeKey, body, func() (int, any, string, string, *domain.AppError) {
		record, errApp := s.app.UpdateAdminOfficialPriceRecord(r.Context(), user, input)
		if errApp != nil {
			return 0, nil, "", "", errApp
		}
		return http.StatusOK, toPriceRecordResponse(record), "official_price_record", record.ID, nil
	})
}

func (s *Server) handleTakeDownAdminOfficialPriceRecord(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[adminOfficialPriceRecordActionRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	recordID := chi.URLParam(r, "id")
	input := officialprice.AdminRecordActionInput{
		RecordID:        recordID,
		AdminUserID:     user.ID,
		ExpectedVersion: version,
		RequestID:       requestIDFrom(r),
		Reason:          req.Reason,
	}
	routeKey := "POST /api/v1/admin/official-price-records/{id}/take-down:" + recordID
	s.withIdempotency(w, r, user.ID, routeKey, body, func() (int, any, string, string, *domain.AppError) {
		record, errApp := s.app.TakeDownAdminOfficialPriceRecord(r.Context(), user, input)
		if errApp != nil {
			return 0, nil, "", "", errApp
		}
		return http.StatusOK, toPriceRecordResponse(record), "official_price_record", record.ID, nil
	})
}

func (s *Server) handleApproveOfficialPriceLead(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if !user.IsAdmin {
		writeProblem(w, r, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。"))
		return
	}

	body, req, appErr := decodeStrictJSON[approveLeadRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	leadID := chi.URLParam(r, "id")
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}

	validFrom, err := parseRequiredTime(req.ValidFrom, "validFrom")
	if err != nil {
		writeProblem(w, r, err)
		return
	}
	fxObservedAt, err := parseRequiredTime(req.FXSnapshot.ObservedAt, "fxSnapshot.observedAt")
	if err != nil {
		writeProblem(w, r, err)
		return
	}

	routeKey := "POST /api/v1/admin/official-price-leads/{id}/approve:" + leadID
	hash := requestHash(r.Method, routeKey, body)
	completion, appErr := s.app.ApproveOfficialPriceLeadWithIdempotency(
		r.Context(),
		user.ID,
		routeKey,
		r.Header.Get("Idempotency-Key"),
		hash,
		officialprice.ApproveLeadInput{
			LeadID:                leadID,
			AdminUserID:           user.ID,
			ExpectedVersion:       version,
			RequestID:             requestIDFrom(r),
			Reason:                req.Reason,
			ResolvedProductPlanID: req.ResolvedProductPlanID,
			ValidFrom:             validFrom,
			FXRateToCNY:           req.FXSnapshot.RateToCNY,
			FXSource:              req.FXSnapshot.Source,
			FXObservedAt:          fxObservedAt,
		},
		func(lead officialprice.Lead, record officialprice.Record) (idempotency.Completion, *domain.AppError) {
			responseBody, marshalErr := json.Marshal(approveLeadResponse{
				Lead:   toLeadResponse(lead),
				Record: toPriceRecordResponse(record),
			})
			if marshalErr != nil {
				return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
			}
			return idempotency.Completion{
				Status:       http.StatusOK,
				ContentType:  "application/json; charset=utf-8",
				Body:         responseBody,
				ResourceType: "official_price_record",
				ResourceID:   record.ID,
			}, nil
		},
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}
func (s *Server) handleRejectOfficialPriceLead(w http.ResponseWriter, r *http.Request) {
	s.handleLeadReviewStatus(w, r, officialprice.LeadStatusRejected)
}

func (s *Server) handleRequestChangesOfficialPriceLead(w http.ResponseWriter, r *http.Request) {
	s.handleLeadReviewStatus(w, r, officialprice.LeadStatusChangesRequested)
}

func (s *Server) handleLeadReviewStatus(w http.ResponseWriter, r *http.Request, status string) {
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
	leadID := chi.URLParam(r, "id")
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	routeKey := "POST /api/v1/admin/official-price-leads/{id}/" + status + ":" + leadID
	s.withIdempotency(w, r, user.ID, routeKey, body, func() (int, any, string, string, *domain.AppError) {
		lead, errApp := s.app.UpdateLeadReviewStatus(r.Context(), user, leadID, status, req.Reason, version)
		if errApp != nil {
			return 0, nil, "", "", errApp
		}
		setETag(w, lead.Version)
		return http.StatusOK, toLeadResponse(lead), "official_price_lead", lead.ID, nil
	})
}

func toAdminOfficialPriceRecordInput(req adminOfficialPriceRecordRequest, recordID, adminUserID string, expectedVersion int64, requestID string) (officialprice.AdminRecordInput, *domain.AppError) {
	observedAt, appErr := parseRequiredTime(req.ObservedAt, "observedAt")
	if appErr != nil {
		return officialprice.AdminRecordInput{}, appErr
	}
	fxObservedAt, appErr := parseRequiredTime(req.FXObservedAt, "fxObservedAt")
	if appErr != nil {
		return officialprice.AdminRecordInput{}, appErr
	}
	validFrom, appErr := parseRequiredTime(req.ValidFrom, "validFrom")
	if appErr != nil {
		return officialprice.AdminRecordInput{}, appErr
	}
	return officialprice.AdminRecordInput{
		RecordID:        recordID,
		AdminUserID:     adminUserID,
		ExpectedVersion: expectedVersion,
		RequestID:       requestID,
		ProductPlanID:   req.ProductPlanID,
		ProductText:     req.ProductText,
		PlanText:        req.PlanText,
		RegionCode:      req.RegionCode,
		Channel:         req.Channel,
		OpeningMethod:   req.OpeningMethod,
		SourceURL:       req.SourceURL,
		ObservedAt:      observedAt,
		BillingPeriod:   req.BillingPeriod,
		Currency:        req.Currency,
		OriginalAmount:  req.OriginalAmount,
		TaxIncluded:     req.TaxIncluded,
		FXRateToCNY:     req.FXRateToCNY,
		FXSource:        req.FXSource,
		FXObservedAt:    fxObservedAt,
		ValidFrom:       validFrom,
		Reason:          req.Reason,
	}, nil
}

func toLeadResponses(leads []officialprice.Lead) []leadResponse {
	items := make([]leadResponse, 0, len(leads))
	for _, lead := range leads {
		items = append(items, toLeadResponse(lead))
	}
	return items
}

func toLeadResponse(lead officialprice.Lead) leadResponse {
	var duplicate *string
	if lead.DuplicateOfLeadID != "" {
		duplicate = &lead.DuplicateOfLeadID
	}
	normalizationStatus := ""
	if lead.NormalizedMonthlyCNY == "" {
		normalizationStatus = "pendingReview"
	}
	return leadResponse{
		ID:                   lead.ID,
		Status:               lead.Status,
		ProductPlanID:        lead.ProductPlanID,
		ProductText:          lead.ProductText,
		RegionCode:           lead.RegionCode,
		SourceURL:            lead.SourceURL,
		ObservedAt:           lead.ObservedAt.UTC().Format(time.RFC3339),
		BillingPeriod:        lead.BillingPeriod,
		Currency:             lead.Currency,
		OriginalAmount:       lead.OriginalAmount,
		NormalizedMonthlyCNY: lead.NormalizedMonthlyCNY,
		NormalizationStatus:  normalizationStatus,
		DuplicateOfLeadID:    duplicate,
		Version:              lead.Version,
		CreatedAt:            lead.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func toLeadOwnerResponse(lead officialprice.Lead) leadOwnerResponse {
	var reason *string
	if lead.ReviewReason != "" {
		reason = &lead.ReviewReason
	}
	var reviewedAt *string
	if lead.ReviewedAt != nil {
		formatted := lead.ReviewedAt.UTC().Format(time.RFC3339)
		reviewedAt = &formatted
	}
	return leadOwnerResponse{
		leadResponse: toLeadResponse(lead),
		ReviewReason: reason,
		ReviewedAt:   reviewedAt,
	}
}

func toLeadAdminResponse(lead officialprice.Lead) leadAdminResponse {
	owner := toLeadOwnerResponse(lead)
	fxObservedAt := ""
	if lead.FXObservedAt != nil {
		fxObservedAt = lead.FXObservedAt.UTC().Format(time.RFC3339)
	}
	return leadAdminResponse{
		leadOwnerResponse: owner,
		SubmitterUserID:   lead.SubmitterUserID,
		PlanText:          lead.PlanText,
		Channel:           lead.Channel,
		OpeningMethod:     lead.OpeningMethod,
		SourceTitle:       lead.SourceTitle,
		EvidenceSummary:   lead.EvidenceSummary,
		Note:              lead.Note,
		PriceUnit:         lead.PriceUnit,
		OriginalPriceText: lead.OriginalPriceText,
		TaxIncluded:       lead.TaxIncluded,
		FXRate:            lead.FXRate,
		FXSource:          lead.FXSource,
		FXObservedAt:      fxObservedAt,
		OfferKey:          lead.OfferKey,
		DuplicateOfLeadID: lead.DuplicateOfLeadID,
		ReviewedByAdminID: lead.ReviewedByAdminID,
		ConversionMode:    lead.ConversionMode,
		NormalizationRule: lead.RoundingRule,
	}
}

func toPriceRecordResponses(records []officialprice.Record) []priceRecordResponse {
	items := make([]priceRecordResponse, 0, len(records))
	for _, record := range records {
		items = append(items, toPriceRecordResponse(record))
	}
	return items
}

func toPriceRecordResponse(record officialprice.Record) priceRecordResponse {
	var validTo *string
	if record.ValidTo != nil {
		formatted := record.ValidTo.UTC().Format(time.RFC3339)
		validTo = &formatted
	}
	return priceRecordResponse{
		ID:                   record.ID,
		LeadID:               record.LeadID,
		ProductPlanID:        record.ProductPlanID,
		RegionCode:           record.RegionCode,
		Channel:              record.Channel,
		OpeningMethod:        record.OpeningMethod,
		SourceURL:            record.SourceURL,
		Status:               record.Status,
		ValidFrom:            record.ValidFrom.UTC().Format(time.RFC3339),
		ValidTo:              validTo,
		ObservedAt:           record.ObservedAt.UTC().Format(time.RFC3339),
		BillingPeriod:        record.BillingPeriod,
		PriceUnit:            record.PriceUnit,
		Currency:             record.Currency,
		OriginalAmount:       record.OriginalAmount,
		TaxIncluded:          record.TaxIncluded,
		NormalizedMonthlyCNY: record.NormalizedMonthlyCNY,
		FXRate:               record.FXRate,
		FXSource:             record.FXSource,
		FXObservedAt:         record.FXObservedAt.UTC().Format(time.RFC3339),
		OfferKey:             record.OfferKey,
		IsLowestReference:    record.IsLowestReference,
		Version:              record.Version,
		CreatedAt:            record.CreatedAt.UTC().Format(time.RFC3339),
	}
}
