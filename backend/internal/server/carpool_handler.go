package server

import (
	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/carpool"
	"c2c-market/backend/internal/module/idempotency"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"net/http"
	"time"
)

type riskAcknowledgementRequest struct {
	RiskNoticeCode string `json:"riskNoticeCode"`
	PolicyVersion  int64  `json:"policyVersion"`
}

type createCarpoolRequest struct {
	ProductPlanID          string                      `json:"productPlanId"`
	OwnerContactMethodID   string                      `json:"ownerContactMethodId"`
	CycleTerm              carpoolCycleTermRequest     `json:"cycleTerm"`
	Title                  string                      `json:"title"`
	Summary                string                      `json:"summary"`
	AccessArrangement      string                      `json:"accessArrangement"`
	DistributionMethod     string                      `json:"distributionMethod"`
	DistributionMethodNote string                      `json:"distributionMethodNote"`
	ProvidesAdminAccount   bool                        `json:"providesAdminAccount"`
	RegionCode             string                      `json:"regionCode"`
	RegionName             string                      `json:"regionName"`
	SourceURL              string                      `json:"sourceUrl"`
	PriceMonthlyCNY        string                      `json:"priceMonthlyCny"`
	ServiceMultiplier      string                      `json:"serviceMultiplier"`
	MonthlyQuotaAmount     string                      `json:"monthlyQuotaAmount"`
	BuyerSeatCapacity      int                         `json:"buyerSeatCapacity"`
	ActiveBuyerMembers     int                         `json:"activeBuyerMembers"`
	RiskAcknowledgement    *riskAcknowledgementRequest `json:"riskAcknowledgement"`
}

type carpoolCycleTermRequest struct {
	BillingPeriod string `json:"billingPeriod"`
	CycleStartDay *int   `json:"cycleStartDay"`
	NoticeDays    int    `json:"noticeDays"`
	ExitPolicy    string `json:"exitPolicy"`
	UsageRules    string `json:"usageRules"`
}

type carpoolCycleTermResponse struct {
	ID            string `json:"id"`
	BillingPeriod string `json:"billingPeriod"`
	CycleStartDay *int   `json:"cycleStartDay,omitempty"`
	NoticeDays    int    `json:"noticeDays"`
	ExitPolicy    string `json:"exitPolicy"`
	UsageRules    string `json:"usageRules"`
	Version       int64  `json:"version"`
	CreatedAt     string `json:"createdAt"`
	UpdatedAt     string `json:"updatedAt"`
}

type carpoolListingResponse struct {
	ID                     string                                `json:"id"`
	OwnerUserID            string                                `json:"ownerUserId"`
	ProductPlanID          string                                `json:"productPlanId"`
	OwnerContactMethodID   string                                `json:"ownerContactMethodId,omitempty"`
	CycleTerm              *carpoolCycleTermResponse             `json:"cycleTerm,omitempty"`
	Title                  string                                `json:"title"`
	Summary                string                                `json:"summary"`
	AccessArrangement      string                                `json:"accessArrangement"`
	DistributionMethod     string                                `json:"distributionMethod"`
	DistributionMethodNote string                                `json:"distributionMethodNote"`
	ProvidesAdminAccount   bool                                  `json:"providesAdminAccount"`
	RegionCode             string                                `json:"regionCode"`
	RegionName             string                                `json:"regionName"`
	SourceURL              string                                `json:"sourceUrl,omitempty"`
	PriceMonthlyCNY        string                                `json:"priceMonthlyCny"`
	ServiceMultiplier      string                                `json:"serviceMultiplier"`
	MonthlyQuotaAmount     string                                `json:"monthlyQuotaAmount"`
	QuotaLabel             string                                `json:"quotaLabel"`
	QuotaUnit              string                                `json:"quotaUnit"`
	QuotaPeriod            string                                `json:"quotaPeriod"`
	BuyerSeatCapacity      int                                   `json:"buyerSeatCapacity"`
	ActiveBuyerMembers     int                                   `json:"activeBuyerMembers"`
	ReservedSeats          int                                   `json:"reservedSeats"`
	AvailableSeats         int                                   `json:"availableSeats"`
	Status                 string                                `json:"status"`
	ReviewReason           *string                               `json:"reviewReason,omitempty"`
	ReviewedAt             *string                               `json:"reviewedAt,omitempty"`
	PolicyVersion          int64                                 `json:"policyVersion"`
	RiskNoticeCode         string                                `json:"riskNoticeCode,omitempty"`
	RiskAckRequired        bool                                  `json:"riskAckRequired"`
	Version                int64                                 `json:"version"`
	CreatedAt              string                                `json:"createdAt"`
	UpdatedAt              string                                `json:"updatedAt"`
	ApplicationEligibility carpoolApplicationEligibilityResponse `json:"applicationEligibility"`
}

type createCarpoolApplicationRequest struct {
	BuyerContactMethodID string                      `json:"buyerContactMethodId"`
	RiskAcknowledgement  *riskAcknowledgementRequest `json:"riskAcknowledgement"`
}

type carpoolApplicationEligibilityResponse struct {
	Code             string `json:"code"`
	CanApply         bool   `json:"canApply"`
	Reason           string `json:"reason"`
	ResolutionAction string `json:"resolutionAction"`
}

type carpoolApplicationResponse struct {
	ID                       string  `json:"id"`
	CarpoolListingID         string  `json:"carpoolListingId"`
	BuyerUserID              string  `json:"buyerUserId"`
	OwnerUserID              string  `json:"ownerUserId"`
	ProductPlanID            string  `json:"productPlanId"`
	BuyerContactMethodID     string  `json:"buyerContactMethodId"`
	Status                   string  `json:"status"`
	SeatCount                int     `json:"seatCount"`
	ListingTitleSnapshot     string  `json:"listingTitleSnapshot"`
	PriceMonthlyCNY          string  `json:"priceMonthlyCny"`
	PolicyVersionSnapshot    int64   `json:"policyVersionSnapshot"`
	RiskNoticeCode           string  `json:"riskNoticeCode,omitempty"`
	ContactSessionID         string  `json:"contactSessionId,omitempty"`
	ReservationExpiresAt     *string `json:"reservationExpiresAt,omitempty"`
	JoinConfirmationDeadline *string `json:"joinConfirmationDeadline,omitempty"`
	BuyerConfirmedAt         *string `json:"buyerConfirmedAt,omitempty"`
	OwnerConfirmedAt         *string `json:"ownerConfirmedAt,omitempty"`
	JoinedAt                 *string `json:"joinedAt,omitempty"`
	DecisionReason           *string `json:"decisionReason,omitempty"`
	DecidedAt                *string `json:"decidedAt,omitempty"`
	Version                  int64   `json:"version"`
	CreatedAt                string  `json:"createdAt"`
	UpdatedAt                string  `json:"updatedAt"`
}

type carpoolMembershipResponse struct {
	ID                    string  `json:"id"`
	CarpoolListingID      string  `json:"carpoolListingId"`
	CarpoolApplicationID  string  `json:"carpoolApplicationId"`
	CycleTermID           string  `json:"cycleTermId,omitempty"`
	BuyerUserID           string  `json:"buyerUserId"`
	OwnerUserID           string  `json:"ownerUserId"`
	ProductPlanID         string  `json:"productPlanId"`
	Status                string  `json:"status"`
	SeatCount             int     `json:"seatCount"`
	PriceMonthlyCNY       string  `json:"priceMonthlyCny"`
	PolicyVersionSnapshot int64   `json:"policyVersionSnapshot"`
	RiskNoticeCode        string  `json:"riskNoticeCode,omitempty"`
	JoinedAt              string  `json:"joinedAt"`
	BuyerCompletedAt      *string `json:"buyerCompletedAt,omitempty"`
	OwnerCompletedAt      *string `json:"ownerCompletedAt,omitempty"`
	CompletedAt           *string `json:"completedAt,omitempty"`
	EndedAt               *string `json:"endedAt,omitempty"`
	EndedReason           string  `json:"endedReason,omitempty"`
	EndedByUserID         string  `json:"endedByUserId,omitempty"`
	Version               int64   `json:"version"`
	CreatedAt             string  `json:"createdAt"`
	UpdatedAt             string  `json:"updatedAt"`
}

func (s *Server) handleCreateCarpool(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[createCarpoolRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}

	s.withIdempotency(w, r, user.ID, "POST /api/v1/carpools", body, func() (int, any, string, string, *domain.AppError) {
		listing, errApp := s.carpools.CreateCarpoolListing(r.Context(), user, toAppCreateCarpoolInput(req))
		if errApp != nil {
			return 0, nil, "", "", errApp
		}
		setETag(w, listing.Version)
		return http.StatusCreated, toCarpoolListingResponse(listing), "carpool_listing", listing.ID, nil
	})
}

func (s *Server) handlePublishCarpool(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[createCarpoolRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}

	s.withIdempotency(w, r, user.ID, "POST /api/v1/carpools/publish", body, func() (int, any, string, string, *domain.AppError) {
		listing, errApp := s.carpools.PublishCarpoolListing(r.Context(), user, toAppCreateCarpoolInput(req))
		if errApp != nil {
			return 0, nil, "", "", errApp
		}
		setETag(w, listing.Version)
		return http.StatusCreated, toCarpoolListingResponse(listing), "carpool_listing", listing.ID, nil
	})
}

func (s *Server) handleUpdateCarpool(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	req, appErr := decodeStrictJSONOnly[createCarpoolRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	listing, appErr := s.carpools.UpdateCarpoolListing(r.Context(), user, carpool.UpdateListingInput{
		ListingID:              chi.URLParam(r, "id"),
		ProductPlanID:          req.ProductPlanID,
		OwnerContactMethodID:   req.OwnerContactMethodID,
		CycleTerm:              toAppCarpoolCycleTerm(req.CycleTerm),
		Title:                  req.Title,
		Summary:                req.Summary,
		AccessArrangement:      req.AccessArrangement,
		DistributionMethod:     req.DistributionMethod,
		DistributionMethodNote: req.DistributionMethodNote,
		ProvidesAdminAccount:   req.ProvidesAdminAccount,
		RegionCode:             req.RegionCode,
		RegionName:             req.RegionName,
		SourceURL:              req.SourceURL,
		PriceMonthlyCNY:        req.PriceMonthlyCNY,
		ServiceMultiplier:      req.ServiceMultiplier,
		MonthlyQuotaAmount:     req.MonthlyQuotaAmount,
		BuyerSeatCapacity:      req.BuyerSeatCapacity,
		ActiveBuyerMembers:     req.ActiveBuyerMembers,
		RiskAcknowledgement:    toAppRiskAck(req.RiskAcknowledgement),
		ExpectedVersion:        version,
		RequestID:              requestIDFrom(r),
	})
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, listing.Version)
	writeJSON(w, http.StatusOK, toCarpoolListingResponse(listing))
}

func toAppCreateCarpoolInput(req createCarpoolRequest) carpool.CreateListingInput {
	return carpool.CreateListingInput{
		ProductPlanID:          req.ProductPlanID,
		OwnerContactMethodID:   req.OwnerContactMethodID,
		CycleTerm:              toAppCarpoolCycleTerm(req.CycleTerm),
		Title:                  req.Title,
		Summary:                req.Summary,
		AccessArrangement:      req.AccessArrangement,
		DistributionMethod:     req.DistributionMethod,
		DistributionMethodNote: req.DistributionMethodNote,
		ProvidesAdminAccount:   req.ProvidesAdminAccount,
		RegionCode:             req.RegionCode,
		RegionName:             req.RegionName,
		SourceURL:              req.SourceURL,
		PriceMonthlyCNY:        req.PriceMonthlyCNY,
		ServiceMultiplier:      req.ServiceMultiplier,
		MonthlyQuotaAmount:     req.MonthlyQuotaAmount,
		BuyerSeatCapacity:      req.BuyerSeatCapacity,
		ActiveBuyerMembers:     req.ActiveBuyerMembers,
		RiskAcknowledgement:    toAppRiskAck(req.RiskAcknowledgement),
	}
}

func (s *Server) handleSubmitCarpoolReview(w http.ResponseWriter, r *http.Request) {
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
	listingID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/carpools/{id}/submit-review:" + listingID
	s.withIdempotency(w, r, user.ID, routeKey, body, func() (int, any, string, string, *domain.AppError) {
		listing, errApp := s.carpools.SubmitCarpoolListingForReview(r.Context(), user, carpool.SubmitListingReviewInput{
			ListingID:       listingID,
			ExpectedVersion: version,
			RequestID:       requestIDFrom(r),
		})
		if errApp != nil {
			return 0, nil, "", "", errApp
		}
		setETag(w, listing.Version)
		return http.StatusOK, toCarpoolListingResponse(listing), "carpool_listing", listing.ID, nil
	})
}

func (s *Server) handlePublicCarpools(w http.ResponseWriter, r *http.Request) {
	pageRequest, appErr := parsePageRequest(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	listings, appErr := s.carpools.PublicCarpoolListings(r.Context(), pageRequest)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePageJSON(w, domain.Page[carpoolListingResponse]{
		Items:      toCarpoolListingResponses(listings.Items),
		NextCursor: listings.NextCursor,
	})
}

func (s *Server) handlePublicCarpool(w http.ResponseWriter, r *http.Request) {
	listing, appErr := s.carpools.PublicCarpoolListing(r.Context(), chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, listing.Version)
	writeJSON(w, http.StatusOK, toCarpoolListingResponse(listing))
}

func (s *Server) handleCarpoolApplicationEligibility(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	eligibility, appErr := s.carpools.CarpoolApplicationEligibility(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toCarpoolApplicationEligibilityResponse(eligibility))
}

func (s *Server) handleMyCarpools(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	listings, appErr := s.carpools.MyCarpoolListings(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toCarpoolListingResponses(listings))
}

func (s *Server) handleAdminCarpools(w http.ResponseWriter, r *http.Request) {
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
	listings, appErr := s.carpools.AdminCarpoolListings(r.Context(), user, pageRequest)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePageJSON(w, domain.Page[carpoolListingResponse]{
		Items:      toCarpoolListingResponses(listings.Items),
		NextCursor: listings.NextCursor,
	})
}

func (s *Server) handleAdminCarpool(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	listing, appErr := s.carpools.AdminCarpoolListing(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, listing.Version)
	writeJSON(w, http.StatusOK, toCarpoolListingResponse(listing))
}

func (s *Server) handleApproveCarpool(w http.ResponseWriter, r *http.Request) {
	s.handleCarpoolReviewStatus(w, r, "approve", carpool.ListingStatusActive)
}

func (s *Server) handleRejectCarpool(w http.ResponseWriter, r *http.Request) {
	s.handleCarpoolReviewStatus(w, r, "reject", carpool.ListingStatusRejected)
}

func (s *Server) handleRequestChangesCarpool(w http.ResponseWriter, r *http.Request) {
	s.handleCarpoolReviewStatus(w, r, "request_changes", carpool.ListingStatusChangesRequested)
}

func (s *Server) handlePauseCarpool(w http.ResponseWriter, r *http.Request) {
	s.handleCarpoolReviewStatus(w, r, "pause", carpool.ListingStatusPaused)
}

func (s *Server) handleRestoreCarpool(w http.ResponseWriter, r *http.Request) {
	s.handleCarpoolReviewStatus(w, r, "restore", carpool.ListingStatusActive)
}

func (s *Server) handleCarpoolReviewStatus(w http.ResponseWriter, r *http.Request, action, status string) {
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
	listingID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/admin/carpools/{id}/" + action + ":" + listingID
	s.withIdempotency(w, r, user.ID, routeKey, body, func() (int, any, string, string, *domain.AppError) {
		listing, errApp := s.carpools.UpdateCarpoolListingReviewStatus(r.Context(), user, carpool.ReviewInput{
			ListingID:       listingID,
			Action:          action,
			Status:          status,
			Reason:          req.Reason,
			ExpectedVersion: version,
			RequestID:       requestIDFrom(r),
		})
		if errApp != nil {
			return 0, nil, "", "", errApp
		}
		setETag(w, listing.Version)
		return http.StatusOK, toCarpoolListingResponse(listing), "carpool_listing", listing.ID, nil
	})
}

func (s *Server) handleCreateCarpoolApplication(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[createCarpoolApplicationRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	listingID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/carpools/{id}/applications:" + listingID
	s.withIdempotency(w, r, user.ID, routeKey, body, func() (int, any, string, string, *domain.AppError) {
		application, errApp := s.carpools.CreateCarpoolApplication(r.Context(), user, carpool.CreateApplicationInput{
			ListingID:            listingID,
			BuyerContactMethodID: req.BuyerContactMethodID,
			RiskAcknowledgement:  toAppRiskAck(req.RiskAcknowledgement),
		})
		if errApp != nil {
			return 0, nil, "", "", errApp
		}
		setETag(w, application.Version)
		return http.StatusCreated, toCarpoolApplicationResponse(application), "carpool_application", application.ID, nil
	})
}

func (s *Server) handleMyCarpoolApplications(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	applications, appErr := s.carpools.MyCarpoolApplications(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toCarpoolApplicationResponses(applications))
}

func (s *Server) handleMyCarpoolApplication(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	application, appErr := s.carpools.MyCarpoolApplication(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, application.Version)
	writeJSON(w, http.StatusOK, toCarpoolApplicationResponse(application))
}

func (s *Server) handleBuyerConfirmCarpoolJoin(w http.ResponseWriter, r *http.Request) {
	s.handleConfirmCarpoolJoin(w, r, carpool.JoinActorBuyer)
}

func (s *Server) handleCancelCarpoolApplication(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[membershipEndRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	applicationID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/me/carpool-applications/{id}/cancel:" + applicationID
	completion, appErr := s.carpools.CancelCarpoolApplicationWithIdempotency(
		r.Context(),
		user.ID,
		routeKey,
		r.Header.Get("Idempotency-Key"),
		requestHash(r.Method, routeKey, body),
		carpool.CancelApplicationInput{
			ApplicationID:   applicationID,
			Reason:          req.Reason,
			ExpectedVersion: version,
			RequestID:       requestIDFrom(r),
		},
		func(application carpool.Application) (idempotency.Completion, *domain.AppError) {
			responseBody, marshalErr := json.Marshal(toCarpoolApplicationResponse(application))
			if marshalErr != nil {
				return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
			}
			return idempotency.Completion{
				Status:       http.StatusOK,
				ContentType:  "application/json; charset=utf-8",
				Body:         responseBody,
				ResourceType: "carpool_application",
				ResourceID:   application.ID,
			}, nil
		},
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleBuyerConfirmCarpoolMembershipComplete(w http.ResponseWriter, r *http.Request) {
	s.handleConfirmCarpoolMembershipComplete(w, r, carpool.JoinActorBuyer)
}

func (s *Server) handleBuyerLeaveCarpoolMembership(w http.ResponseWriter, r *http.Request) {
	s.handleEndCarpoolMembership(w, r, carpool.JoinActorBuyer, carpool.MembershipStatusLeft)
}

func (s *Server) handleMyCarpoolMemberships(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	memberships, appErr := s.carpools.MyCarpoolMemberships(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toCarpoolMembershipResponses(memberships))
}
func (s *Server) handleOwnerCarpoolApplications(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	applications, appErr := s.carpools.OwnerCarpoolApplications(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toCarpoolApplicationResponses(applications))
}

func (s *Server) handleOwnerCarpoolApplication(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	application, appErr := s.carpools.OwnerCarpoolApplication(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, application.Version)
	writeJSON(w, http.StatusOK, toCarpoolApplicationResponse(application))
}

func (s *Server) handleAcceptCarpoolApplication(w http.ResponseWriter, r *http.Request) {
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
	applicationID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/owner/carpool-applications/{id}/accept:" + applicationID
	completion, appErr := s.carpools.AcceptCarpoolApplicationWithIdempotency(
		r.Context(),
		user.ID,
		routeKey,
		r.Header.Get("Idempotency-Key"),
		requestHash(r.Method, routeKey, body),
		carpool.AcceptApplicationInput{
			ApplicationID:   applicationID,
			ExpectedVersion: version,
			RequestID:       requestIDFrom(r),
		},
		func(application carpool.Application) (idempotency.Completion, *domain.AppError) {
			responseBody, marshalErr := json.Marshal(toCarpoolApplicationResponse(application))
			if marshalErr != nil {
				return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
			}
			return idempotency.Completion{
				Status:       http.StatusOK,
				ContentType:  "application/json; charset=utf-8",
				Body:         responseBody,
				ResourceType: "carpool_application",
				ResourceID:   application.ID,
			}, nil
		},
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleRejectCarpoolApplication(w http.ResponseWriter, r *http.Request) {
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
	applicationID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/owner/carpool-applications/{id}/reject:" + applicationID
	s.withIdempotency(w, r, user.ID, routeKey, body, func() (int, any, string, string, *domain.AppError) {
		application, errApp := s.carpools.RejectCarpoolApplication(r.Context(), carpool.RejectApplicationInput{
			ApplicationID:   applicationID,
			OwnerUserID:     user.ID,
			Reason:          req.Reason,
			ExpectedVersion: version,
			RequestID:       requestIDFrom(r),
		})
		if errApp != nil {
			return 0, nil, "", "", errApp
		}
		setETag(w, application.Version)
		return http.StatusOK, toCarpoolApplicationResponse(application), "carpool_application", application.ID, nil
	})
}

func (s *Server) handleWithdrawCarpoolAcceptance(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[membershipEndRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	applicationID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/owner/carpool-applications/{id}/withdraw-acceptance:" + applicationID
	completion, appErr := s.carpools.WithdrawCarpoolAcceptanceWithIdempotency(
		r.Context(),
		user.ID,
		routeKey,
		r.Header.Get("Idempotency-Key"),
		requestHash(r.Method, routeKey, body),
		carpool.WithdrawAcceptanceInput{
			ApplicationID:   applicationID,
			Reason:          req.Reason,
			ExpectedVersion: version,
			RequestID:       requestIDFrom(r),
		},
		func(application carpool.Application) (idempotency.Completion, *domain.AppError) {
			responseBody, marshalErr := json.Marshal(toCarpoolApplicationResponse(application))
			if marshalErr != nil {
				return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
			}
			return idempotency.Completion{
				Status:       http.StatusOK,
				ContentType:  "application/json; charset=utf-8",
				Body:         responseBody,
				ResourceType: "carpool_application",
				ResourceID:   application.ID,
			}, nil
		},
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleOwnerConfirmCarpoolJoin(w http.ResponseWriter, r *http.Request) {
	s.handleConfirmCarpoolJoin(w, r, carpool.JoinActorOwner)
}

func (s *Server) handleOwnerConfirmCarpoolMembershipComplete(w http.ResponseWriter, r *http.Request) {
	s.handleConfirmCarpoolMembershipComplete(w, r, carpool.JoinActorOwner)
}

func (s *Server) handleOwnerRemoveCarpoolMembership(w http.ResponseWriter, r *http.Request) {
	s.handleEndCarpoolMembership(w, r, carpool.JoinActorOwner, carpool.MembershipStatusRemoved)
}

func (s *Server) handleOwnerCarpoolMemberships(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	memberships, appErr := s.carpools.OwnerCarpoolMemberships(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toCarpoolMembershipResponses(memberships))
}
func (s *Server) handleConfirmCarpoolJoin(w http.ResponseWriter, r *http.Request, actorRole string) {
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
	applicationID := chi.URLParam(r, "id")
	routePrefix := "POST /api/v1/me/carpool-applications/{id}/confirm-join"
	if actorRole == carpool.JoinActorOwner {
		routePrefix = "POST /api/v1/owner/carpool-applications/{id}/confirm-join"
	}
	routeKey := routePrefix + ":" + applicationID
	completion, appErr := s.carpools.ConfirmCarpoolApplicationJoinWithIdempotency(
		r.Context(),
		user.ID,
		routeKey,
		r.Header.Get("Idempotency-Key"),
		requestHash(r.Method, routeKey, body),
		carpool.ConfirmApplicationJoinInput{
			ApplicationID:   applicationID,
			ActorRole:       actorRole,
			ExpectedVersion: version,
			RequestID:       requestIDFrom(r),
		},
		func(application carpool.Application) (idempotency.Completion, *domain.AppError) {
			responseBody, marshalErr := json.Marshal(toCarpoolApplicationResponse(application))
			if marshalErr != nil {
				return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
			}
			return idempotency.Completion{
				Status:       http.StatusOK,
				ContentType:  "application/json; charset=utf-8",
				Body:         responseBody,
				ResourceType: "carpool_application",
				ResourceID:   application.ID,
			}, nil
		},
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleConfirmCarpoolMembershipComplete(w http.ResponseWriter, r *http.Request, actorRole string) {
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
	membershipID := chi.URLParam(r, "id")
	routePrefix := "POST /api/v1/me/carpool-memberships/{id}/confirm-complete"
	if actorRole == carpool.JoinActorOwner {
		routePrefix = "POST /api/v1/owner/carpool-memberships/{id}/confirm-complete"
	}
	routeKey := routePrefix + ":" + membershipID
	completion, appErr := s.carpools.ConfirmCarpoolMembershipCompleteWithIdempotency(
		r.Context(),
		user.ID,
		routeKey,
		r.Header.Get("Idempotency-Key"),
		requestHash(r.Method, routeKey, body),
		carpool.ConfirmMembershipCompleteInput{
			MembershipID:    membershipID,
			ActorRole:       actorRole,
			ExpectedVersion: version,
			RequestID:       requestIDFrom(r),
		},
		func(membership carpool.Membership) (idempotency.Completion, *domain.AppError) {
			responseBody, marshalErr := json.Marshal(toCarpoolMembershipResponse(membership))
			if marshalErr != nil {
				return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
			}
			return idempotency.Completion{
				Status:       http.StatusOK,
				ContentType:  "application/json; charset=utf-8",
				Body:         responseBody,
				ResourceType: "carpool_membership",
				ResourceID:   membership.ID,
			}, nil
		},
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleEndCarpoolMembership(w http.ResponseWriter, r *http.Request, actorRole, targetStatus string) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[membershipEndRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	membershipID := chi.URLParam(r, "id")
	routePrefix := "POST /api/v1/me/carpool-memberships/{id}/leave"
	if actorRole == carpool.JoinActorOwner {
		routePrefix = "POST /api/v1/owner/carpool-memberships/{id}/remove"
	}
	routeKey := routePrefix + ":" + membershipID
	completion, appErr := s.carpools.EndCarpoolMembershipWithIdempotency(
		r.Context(),
		user.ID,
		routeKey,
		r.Header.Get("Idempotency-Key"),
		requestHash(r.Method, routeKey, body),
		carpool.EndMembershipInput{
			MembershipID:    membershipID,
			ActorRole:       actorRole,
			TargetStatus:    targetStatus,
			Reason:          req.Reason,
			ExpectedVersion: version,
			RequestID:       requestIDFrom(r),
		},
		func(membership carpool.Membership) (idempotency.Completion, *domain.AppError) {
			responseBody, marshalErr := json.Marshal(toCarpoolMembershipResponse(membership))
			if marshalErr != nil {
				return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
			}
			return idempotency.Completion{
				Status:       http.StatusOK,
				ContentType:  "application/json; charset=utf-8",
				Body:         responseBody,
				ResourceType: "carpool_membership",
				ResourceID:   membership.ID,
			}, nil
		},
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}
func toAppRiskAck(req *riskAcknowledgementRequest) *carpool.RiskAcknowledgement {
	if req == nil {
		return nil
	}
	return &carpool.RiskAcknowledgement{
		RiskNoticeCode: req.RiskNoticeCode,
		PolicyVersion:  req.PolicyVersion,
	}
}

func toAppCarpoolCycleTerm(req carpoolCycleTermRequest) carpool.CycleTermInput {
	return carpool.CycleTermInput{
		BillingPeriod: req.BillingPeriod,
		CycleStartDay: req.CycleStartDay,
		NoticeDays:    req.NoticeDays,
		ExitPolicy:    req.ExitPolicy,
		UsageRules:    req.UsageRules,
	}
}

func toCarpoolListingResponses(listings []carpool.Listing) []carpoolListingResponse {
	items := make([]carpoolListingResponse, 0, len(listings))
	for _, listing := range listings {
		items = append(items, toCarpoolListingResponse(listing))
	}
	return items
}

func toCarpoolListingResponse(listing carpool.Listing) carpoolListingResponse {
	var reviewReason *string
	if listing.ReviewReason != "" {
		reviewReason = &listing.ReviewReason
	}
	var reviewedAt *string
	if listing.ReviewedAt != nil {
		formatted := listing.ReviewedAt.UTC().Format(time.RFC3339)
		reviewedAt = &formatted
	}
	var cycleTerm *carpoolCycleTermResponse
	if listing.CycleTerm != nil {
		cycleTerm = &carpoolCycleTermResponse{
			ID:            listing.CycleTerm.ID,
			BillingPeriod: listing.CycleTerm.BillingPeriod,
			CycleStartDay: listing.CycleTerm.CycleStartDay,
			NoticeDays:    listing.CycleTerm.NoticeDays,
			ExitPolicy:    listing.CycleTerm.ExitPolicy,
			UsageRules:    listing.CycleTerm.UsageRules,
			Version:       listing.CycleTerm.Version,
			CreatedAt:     listing.CycleTerm.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt:     listing.CycleTerm.UpdatedAt.UTC().Format(time.RFC3339),
		}
	}
	return carpoolListingResponse{
		ID:                     listing.ID,
		OwnerUserID:            listing.OwnerUserID,
		ProductPlanID:          listing.ProductPlanID,
		OwnerContactMethodID:   listing.OwnerContactMethodID,
		CycleTerm:              cycleTerm,
		Title:                  listing.Title,
		Summary:                listing.Summary,
		AccessArrangement:      listing.AccessArrangement,
		DistributionMethod:     listing.DistributionMethod,
		DistributionMethodNote: listing.DistributionMethodNote,
		ProvidesAdminAccount:   listing.ProvidesAdminAccount,
		RegionCode:             listing.RegionCode,
		RegionName:             listing.RegionName,
		SourceURL:              listing.SourceURL,
		PriceMonthlyCNY:        listing.PriceMonthlyCNY,
		ServiceMultiplier:      listing.ServiceMultiplier,
		MonthlyQuotaAmount:     listing.MonthlyQuotaAmount,
		QuotaLabel:             listing.QuotaLabel,
		QuotaUnit:              listing.QuotaUnit,
		QuotaPeriod:            listing.QuotaPeriod,
		BuyerSeatCapacity:      listing.BuyerSeatCapacity,
		ActiveBuyerMembers:     listing.ActiveBuyerMembers,
		ReservedSeats:          listing.ReservedSeats,
		AvailableSeats:         listing.AvailableSeats,
		Status:                 listing.Status,
		ReviewReason:           reviewReason,
		ReviewedAt:             reviewedAt,
		PolicyVersion:          listing.PolicyVersion,
		RiskNoticeCode:         listing.RiskNoticeCode,
		RiskAckRequired:        listing.RiskAckRequired,
		Version:                listing.Version,
		CreatedAt:              listing.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:              listing.UpdatedAt.UTC().Format(time.RFC3339),
		ApplicationEligibility: toCarpoolApplicationEligibilityResponse(carpool.EvaluatePublicListingEligibility(listing)),
	}
}

func toCarpoolApplicationEligibilityResponse(eligibility carpool.ApplicationEligibility) carpoolApplicationEligibilityResponse {
	return carpoolApplicationEligibilityResponse{
		Code:             eligibility.Code,
		CanApply:         eligibility.CanApply,
		Reason:           eligibility.Reason,
		ResolutionAction: eligibility.ResolutionAction,
	}
}

func toCarpoolApplicationResponses(applications []carpool.Application) []carpoolApplicationResponse {
	items := make([]carpoolApplicationResponse, 0, len(applications))
	for _, application := range applications {
		items = append(items, toCarpoolApplicationResponse(application))
	}
	return items
}

func toCarpoolApplicationResponse(application carpool.Application) carpoolApplicationResponse {
	var decisionReason *string
	if application.DecisionReason != "" {
		decisionReason = &application.DecisionReason
	}
	var decidedAt *string
	if application.DecidedAt != nil {
		formatted := application.DecidedAt.UTC().Format(time.RFC3339)
		decidedAt = &formatted
	}
	var reservationExpiresAt *string
	if application.ReservationExpiresAt != nil {
		formatted := application.ReservationExpiresAt.UTC().Format(time.RFC3339)
		reservationExpiresAt = &formatted
	}
	var joinConfirmationDeadline *string
	if application.JoinConfirmationDeadline != nil {
		formatted := application.JoinConfirmationDeadline.UTC().Format(time.RFC3339)
		joinConfirmationDeadline = &formatted
	}
	var buyerConfirmedAt *string
	if application.BuyerConfirmedAt != nil {
		formatted := application.BuyerConfirmedAt.UTC().Format(time.RFC3339)
		buyerConfirmedAt = &formatted
	}
	var ownerConfirmedAt *string
	if application.OwnerConfirmedAt != nil {
		formatted := application.OwnerConfirmedAt.UTC().Format(time.RFC3339)
		ownerConfirmedAt = &formatted
	}
	var joinedAt *string
	if application.JoinedAt != nil {
		formatted := application.JoinedAt.UTC().Format(time.RFC3339)
		joinedAt = &formatted
	}
	return carpoolApplicationResponse{
		ID:                       application.ID,
		CarpoolListingID:         application.CarpoolListingID,
		BuyerUserID:              application.BuyerUserID,
		OwnerUserID:              application.OwnerUserID,
		ProductPlanID:            application.ProductPlanID,
		BuyerContactMethodID:     application.BuyerContactMethodID,
		Status:                   application.Status,
		SeatCount:                application.SeatCount,
		ListingTitleSnapshot:     application.ListingTitleSnapshot,
		PriceMonthlyCNY:          application.PriceMonthlyCNY,
		PolicyVersionSnapshot:    application.PolicyVersionSnapshot,
		RiskNoticeCode:           application.RiskNoticeCode,
		ContactSessionID:         application.ContactSessionID,
		ReservationExpiresAt:     reservationExpiresAt,
		JoinConfirmationDeadline: joinConfirmationDeadline,
		BuyerConfirmedAt:         buyerConfirmedAt,
		OwnerConfirmedAt:         ownerConfirmedAt,
		JoinedAt:                 joinedAt,
		DecisionReason:           decisionReason,
		DecidedAt:                decidedAt,
		Version:                  application.Version,
		CreatedAt:                application.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:                application.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toCarpoolMembershipResponses(memberships []carpool.Membership) []carpoolMembershipResponse {
	items := make([]carpoolMembershipResponse, 0, len(memberships))
	for _, membership := range memberships {
		items = append(items, toCarpoolMembershipResponse(membership))
	}
	return items
}

func toCarpoolMembershipResponse(membership carpool.Membership) carpoolMembershipResponse {
	var buyerCompletedAt *string
	if membership.BuyerCompletedAt != nil {
		formatted := membership.BuyerCompletedAt.UTC().Format(time.RFC3339)
		buyerCompletedAt = &formatted
	}
	var ownerCompletedAt *string
	if membership.OwnerCompletedAt != nil {
		formatted := membership.OwnerCompletedAt.UTC().Format(time.RFC3339)
		ownerCompletedAt = &formatted
	}
	var completedAt *string
	if membership.CompletedAt != nil {
		formatted := membership.CompletedAt.UTC().Format(time.RFC3339)
		completedAt = &formatted
	}
	var endedAt *string
	if membership.EndedAt != nil {
		formatted := membership.EndedAt.UTC().Format(time.RFC3339)
		endedAt = &formatted
	}
	return carpoolMembershipResponse{
		ID:                    membership.ID,
		CarpoolListingID:      membership.CarpoolListingID,
		CarpoolApplicationID:  membership.CarpoolApplicationID,
		CycleTermID:           membership.CycleTermID,
		BuyerUserID:           membership.BuyerUserID,
		OwnerUserID:           membership.OwnerUserID,
		ProductPlanID:         membership.ProductPlanID,
		Status:                membership.Status,
		SeatCount:             membership.SeatCount,
		PriceMonthlyCNY:       membership.PriceMonthlyCNY,
		PolicyVersionSnapshot: membership.PolicyVersionSnapshot,
		RiskNoticeCode:        membership.RiskNoticeCode,
		JoinedAt:              membership.JoinedAt.UTC().Format(time.RFC3339),
		BuyerCompletedAt:      buyerCompletedAt,
		OwnerCompletedAt:      ownerCompletedAt,
		CompletedAt:           completedAt,
		EndedAt:               endedAt,
		EndedReason:           membership.EndedReason,
		EndedByUserID:         membership.EndedByUserID,
		Version:               membership.Version,
		CreatedAt:             membership.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:             membership.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
