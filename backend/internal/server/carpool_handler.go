package server

import (
	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/carpool"
	"c2c-market/backend/internal/module/idempotency"
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"net/http"
	"strconv"
	"time"
)

type riskAcknowledgementRequest struct {
	RiskNoticeCode string `json:"riskNoticeCode"`
	PolicyVersion  int64  `json:"policyVersion"`
}

type createCarpoolRequest struct {
	ProductPlanID                         string                      `json:"productPlanId"`
	OwnerContactMethodID                  string                      `json:"ownerContactMethodId"`
	CycleTerm                             carpoolCycleTermRequest     `json:"cycleTerm"`
	Title                                 string                      `json:"title"`
	Summary                               string                      `json:"summary"`
	AccessArrangement                     string                      `json:"accessArrangement"`
	DistributionMethod                    string                      `json:"distributionMethod"`
	DistributionMethodNote                string                      `json:"distributionMethodNote"`
	ProvidesAdminAccount                  bool                        `json:"providesAdminAccount"`
	RegionCode                            string                      `json:"regionCode"`
	RegionName                            string                      `json:"regionName"`
	SourceURL                             string                      `json:"sourceUrl"`
	PriceMonthlyCNY                       string                      `json:"priceMonthlyCny"`
	ServiceMultiplier                     string                      `json:"serviceMultiplier"`
	DailyQuotaAmount                      *string                     `json:"dailySpendLimitUsd"`
	WeeklyQuotaAmount                     *string                     `json:"weeklySpendLimitUsd"`
	FollowsOfficialQuotaReset             *bool                       `json:"followsOfficialQuotaReset"`
	VPSRegion                             string                      `json:"vpsRegion"`
	SupportsMainlandChinaDirectConnection *bool                       `json:"supportsMainlandChinaDirectConnection"`
	OpeningChannelCode                    string                      `json:"openingChannelCode"`
	CustomOpeningChannel                  string                      `json:"customOpeningChannel"`
	PaymentMethodCode                     string                      `json:"paymentMethodCode"`
	CustomPaymentMethod                   string                      `json:"customPaymentMethod"`
	BuyerSeatCapacity                     int                         `json:"buyerSeatCapacity"`
	OfflineOccupiedSeats                  int                         `json:"offlineOccupiedSeats"`
	RiskAcknowledgement                   *riskAcknowledgementRequest `json:"riskAcknowledgement"`
}

func optionalRequestString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
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
	ID                                    string                                `json:"id"`
	OwnerUserID                           string                                `json:"ownerUserId"`
	ProductPlanID                         string                                `json:"productPlanId"`
	OwnerContactMethodID                  string                                `json:"ownerContactMethodId,omitempty"`
	CycleTerm                             *carpoolCycleTermResponse             `json:"cycleTerm,omitempty"`
	Title                                 string                                `json:"title"`
	Summary                               string                                `json:"summary"`
	AccessArrangement                     string                                `json:"accessArrangement"`
	DistributionMethod                    string                                `json:"distributionMethod"`
	DistributionMethodNote                string                                `json:"distributionMethodNote"`
	ProvidesAdminAccount                  bool                                  `json:"providesAdminAccount"`
	RegionCode                            string                                `json:"regionCode"`
	RegionName                            string                                `json:"regionName"`
	SourceURL                             string                                `json:"sourceUrl,omitempty"`
	PriceMonthlyCNY                       string                                `json:"priceMonthlyCny"`
	ServiceMultiplier                     string                                `json:"serviceMultiplier"`
	DailyQuotaAmount                      *string                               `json:"dailySpendLimitUsd"`
	WeeklyQuotaAmount                     *string                               `json:"weeklySpendLimitUsd"`
	FollowsOfficialQuotaReset             *bool                                 `json:"followsOfficialQuotaReset"`
	VPSRegion                             *string                               `json:"vpsRegion"`
	SupportsMainlandChinaDirectConnection *bool                                 `json:"supportsMainlandChinaDirectConnection"`
	OpeningChannelCode                    *string                               `json:"openingChannelCode"`
	CustomOpeningChannel                  *string                               `json:"customOpeningChannel"`
	PaymentMethodCode                     *string                               `json:"paymentMethodCode"`
	CustomPaymentMethod                   *string                               `json:"customPaymentMethod"`
	QuotaLabel                            string                                `json:"quotaLabel"`
	QuotaUnit                             string                                `json:"quotaUnit"`
	QuotaPeriod                           string                                `json:"quotaPeriod"`
	BuyerSeatCapacity                     int                                   `json:"buyerSeatCapacity"`
	OfflineOccupiedSeats                  int                                   `json:"offlineOccupiedSeats"`
	ActiveBuyerMembers                    int                                   `json:"activeBuyerMembers"`
	AvailableSeats                        int                                   `json:"availableSeats"`
	Status                                string                                `json:"status"`
	GovernanceStatus                      string                                `json:"governanceStatus"`
	RecruitmentStopReason                 string                                `json:"recruitmentStopReason,omitempty"`
	ConditionsVersion                     int64                                 `json:"conditionsVersion"`
	ReviewReason                          *string                               `json:"reviewReason,omitempty"`
	ReviewedAt                            *string                               `json:"reviewedAt,omitempty"`
	PolicyVersion                         int64                                 `json:"policyVersion"`
	RiskNoticeCode                        string                                `json:"riskNoticeCode,omitempty"`
	RiskAckRequired                       bool                                  `json:"riskAckRequired"`
	Version                               int64                                 `json:"version"`
	CreatedAt                             string                                `json:"createdAt"`
	UpdatedAt                             string                                `json:"updatedAt"`
	ApplicationEligibility                carpoolApplicationEligibilityResponse `json:"applicationEligibility"`
	SellerReputation                      *reputationSummaryResponse            `json:"sellerReputation"`
	SourceAuthorVerification              sourceAuthorResourceSummaryResponse   `json:"sourceAuthorVerification"`
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
	ID                        string                            `json:"id"`
	CarpoolListingID          string                            `json:"carpoolListingId"`
	BuyerUserID               string                            `json:"buyerUserId"`
	OwnerUserID               string                            `json:"ownerUserId"`
	ProductPlanID             string                            `json:"productPlanId"`
	BuyerContactMethodID      string                            `json:"buyerContactMethodId"`
	Status                    string                            `json:"status"`
	SeatCount                 int                               `json:"seatCount"`
	ListingTitleSnapshot      string                            `json:"listingTitleSnapshot"`
	PriceMonthlyCNY           string                            `json:"priceMonthlyCny"`
	PolicyVersionSnapshot     int64                             `json:"policyVersionSnapshot"`
	RiskNoticeCode            string                            `json:"riskNoticeCode,omitempty"`
	ConditionsVersionSnapshot int64                             `json:"conditionsVersionSnapshot"`
	ConditionsSnapshot        carpool.ListingConditionsSnapshot `json:"conditionsSnapshot"`
	AcceptedConditionsVersion int64                             `json:"acceptedConditionsVersion"`
	ConditionsAcceptedAt      string                            `json:"conditionsAcceptedAt"`
	ContactSessionID          string                            `json:"contactSessionId,omitempty"`
	JoinedAt                  *string                           `json:"joinedAt,omitempty"`
	DecisionReason            *string                           `json:"decisionReason,omitempty"`
	DecidedAt                 *string                           `json:"decidedAt,omitempty"`
	Version                   int64                             `json:"version"`
	CreatedAt                 string                            `json:"createdAt"`
	UpdatedAt                 string                            `json:"updatedAt"`
	BuyerReputation           *reputationSummaryResponse        `json:"buyerReputation"`
}

type carpoolMembershipResponse struct {
	ID                        string                            `json:"id"`
	CarpoolListingID          string                            `json:"carpoolListingId"`
	CarpoolApplicationID      string                            `json:"carpoolApplicationId"`
	CycleTermID               string                            `json:"cycleTermId,omitempty"`
	BuyerUserID               string                            `json:"buyerUserId"`
	OwnerUserID               string                            `json:"ownerUserId"`
	ProductPlanID             string                            `json:"productPlanId"`
	Status                    string                            `json:"status"`
	SeatCount                 int                               `json:"seatCount"`
	PriceMonthlyCNY           string                            `json:"priceMonthlyCny"`
	PolicyVersionSnapshot     int64                             `json:"policyVersionSnapshot"`
	RiskNoticeCode            string                            `json:"riskNoticeCode,omitempty"`
	ConditionsVersionSnapshot int64                             `json:"conditionsVersionSnapshot"`
	ConditionsSnapshot        carpool.ListingConditionsSnapshot `json:"conditionsSnapshot"`
	JoinedAt                  string                            `json:"joinedAt"`
	EndedAt                   *string                           `json:"endedAt,omitempty"`
	EndedReason               string                            `json:"endedReason,omitempty"`
	EndedByUserID             string                            `json:"endedByUserId,omitempty"`
	Version                   int64                             `json:"version"`
	CreatedAt                 string                            `json:"createdAt"`
	UpdatedAt                 string                            `json:"updatedAt"`
}

func (s *Server) handleCreateCarpool(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if !requireCapability(w, r, user, auth.CapabilityCarpoolPublish) {
		return
	}
	body, req, appErr := decodeStrictJSON[createCarpoolRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}

	routeKey := "POST /api/v1/carpools"
	input := toAppCreateCarpoolInput(req)
	input.RequestID = requestIDFrom(r)
	completion, appErr := s.carpools.CreateCarpoolListingWithIdempotency(
		r.Context(), user, routeKey, r.Header.Get("Idempotency-Key"), requestHash(http.MethodPost, routeKey, body), input,
		carpoolListingCompletionBuilder(http.StatusCreated),
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	restoreCarpoolListingETag(&completion)
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handlePublishCarpool(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if !requireCapability(w, r, user, auth.CapabilityCarpoolPublish) {
		return
	}
	body, req, appErr := decodeStrictJSON[createCarpoolRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}

	routeKey := "POST /api/v1/carpools/publish"
	input := toAppCreateCarpoolInput(req)
	input.RequestID = requestIDFrom(r)
	completion, appErr := s.carpools.PublishCarpoolListingWithIdempotency(
		r.Context(), user, routeKey, r.Header.Get("Idempotency-Key"), requestHash(http.MethodPost, routeKey, body), input,
		carpoolListingCompletionBuilder(http.StatusCreated),
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	restoreCarpoolListingETag(&completion)
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleUpdateCarpool(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[createCarpoolRequest](r)
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
	routeKey := "PATCH /api/v1/carpools/{id}:" + listingID
	completion, appErr := s.carpools.UpdateCarpoolListingWithIdempotency(r.Context(), user, routeKey, r.Header.Get("Idempotency-Key"), requestHash(http.MethodPatch, routeKey, body), carpool.UpdateListingInput{
		ListingID:                             listingID,
		ProductPlanID:                         req.ProductPlanID,
		OwnerContactMethodID:                  req.OwnerContactMethodID,
		CycleTerm:                             toAppCarpoolCycleTerm(req.CycleTerm),
		Title:                                 req.Title,
		Summary:                               req.Summary,
		AccessArrangement:                     req.AccessArrangement,
		DistributionMethod:                    req.DistributionMethod,
		DistributionMethodNote:                req.DistributionMethodNote,
		ProvidesAdminAccount:                  req.ProvidesAdminAccount,
		RegionCode:                            req.RegionCode,
		RegionName:                            req.RegionName,
		SourceURL:                             req.SourceURL,
		PriceMonthlyCNY:                       req.PriceMonthlyCNY,
		ServiceMultiplier:                     req.ServiceMultiplier,
		DailyQuotaAmount:                      optionalRequestString(req.DailyQuotaAmount),
		WeeklyQuotaAmount:                     optionalRequestString(req.WeeklyQuotaAmount),
		FollowsOfficialQuotaReset:             req.FollowsOfficialQuotaReset,
		VPSRegion:                             req.VPSRegion,
		SupportsMainlandChinaDirectConnection: req.SupportsMainlandChinaDirectConnection,
		OpeningChannelCode:                    req.OpeningChannelCode,
		CustomOpeningChannel:                  req.CustomOpeningChannel,
		PaymentMethodCode:                     req.PaymentMethodCode,
		CustomPaymentMethod:                   req.CustomPaymentMethod,
		BuyerSeatCapacity:                     req.BuyerSeatCapacity,
		OfflineOccupiedSeats:                  req.OfflineOccupiedSeats,
		RiskAcknowledgement:                   toAppRiskAck(req.RiskAcknowledgement),
		ExpectedVersion:                       version,
		RequestID:                             requestIDFrom(r),
	}, carpoolListingCompletionBuilder(http.StatusOK))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	restoreCarpoolListingETag(&completion)
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleStopCarpoolRecruiting(w http.ResponseWriter, r *http.Request) {
	s.handleUpdateCarpoolRecruitment(w, r, carpool.ListingStatusStopped)
}

func (s *Server) handleResumeCarpoolRecruiting(w http.ResponseWriter, r *http.Request) {
	s.handleUpdateCarpoolRecruitment(w, r, carpool.ListingStatusActive)
}

func (s *Server) handleUpdateCarpoolRecruitment(w http.ResponseWriter, r *http.Request, targetStatus string) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if !requireCapability(w, r, user, auth.CapabilityCarpoolPublish) {
		return
	}
	if _, _, appErr := decodeStrictJSON[emptyRequest](r); appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	listing, appErr := s.carpools.UpdateRecruitment(r.Context(), user, carpool.RecruitmentInput{
		ListingID:       chi.URLParam(r, "id"),
		ExpectedVersion: version,
		RequestID:       requestIDFrom(r),
	}, targetStatus)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, listing.Version)
	writeJSON(w, http.StatusOK, toCarpoolListingResponse(listing))
}

func toAppCreateCarpoolInput(req createCarpoolRequest) carpool.CreateListingInput {
	return carpool.CreateListingInput{
		ProductPlanID:                         req.ProductPlanID,
		OwnerContactMethodID:                  req.OwnerContactMethodID,
		CycleTerm:                             toAppCarpoolCycleTerm(req.CycleTerm),
		Title:                                 req.Title,
		Summary:                               req.Summary,
		AccessArrangement:                     req.AccessArrangement,
		DistributionMethod:                    req.DistributionMethod,
		DistributionMethodNote:                req.DistributionMethodNote,
		ProvidesAdminAccount:                  req.ProvidesAdminAccount,
		RegionCode:                            req.RegionCode,
		RegionName:                            req.RegionName,
		SourceURL:                             req.SourceURL,
		PriceMonthlyCNY:                       req.PriceMonthlyCNY,
		ServiceMultiplier:                     req.ServiceMultiplier,
		DailyQuotaAmount:                      optionalRequestString(req.DailyQuotaAmount),
		WeeklyQuotaAmount:                     optionalRequestString(req.WeeklyQuotaAmount),
		FollowsOfficialQuotaReset:             req.FollowsOfficialQuotaReset,
		VPSRegion:                             req.VPSRegion,
		SupportsMainlandChinaDirectConnection: req.SupportsMainlandChinaDirectConnection,
		OpeningChannelCode:                    req.OpeningChannelCode,
		CustomOpeningChannel:                  req.CustomOpeningChannel,
		PaymentMethodCode:                     req.PaymentMethodCode,
		CustomPaymentMethod:                   req.CustomPaymentMethod,
		BuyerSeatCapacity:                     req.BuyerSeatCapacity,
		OfflineOccupiedSeats:                  req.OfflineOccupiedSeats,
		RiskAcknowledgement:                   toAppRiskAck(req.RiskAcknowledgement),
	}
}

func (s *Server) handleSubmitCarpoolReview(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if !requireCapability(w, r, user, auth.CapabilityCarpoolPublish) {
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
	completion, appErr := s.carpools.SubmitCarpoolListingForReviewWithIdempotency(
		r.Context(), user, routeKey, r.Header.Get("Idempotency-Key"), requestHash(http.MethodPost, routeKey, body),
		carpool.SubmitListingReviewInput{
			ListingID:       listingID,
			ExpectedVersion: version,
			RequestID:       requestIDFrom(r),
		}, carpoolListingCompletionBuilder(http.StatusOK),
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	restoreCarpoolListingETag(&completion)
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handlePublicCarpools(w http.ResponseWriter, r *http.Request) {
	pageRequest, appErr := parsePageRequest(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	listings, appErr := s.carpools.PublicCarpoolListings(r.Context(), carpoolListingFilter(r), pageRequest)
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
	user, _, appErr := s.requireSession(w, r)
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
	listings, appErr := s.carpools.MyCarpoolListings(r.Context(), user, r.URL.Query().Get("view"), pageRequest)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePageJSON(w, domain.Page[carpoolListingResponse]{
		Items:      toCarpoolListingResponses(listings.Items),
		NextCursor: listings.NextCursor,
	})
}

func (s *Server) handleMyCarpool(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	listing, appErr := s.carpools.MyCarpoolListing(r.Context(), user, chi.URLParam(r, "id"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, listing.Version)
	writeJSON(w, http.StatusOK, toCarpoolListingResponse(listing))
}

func (s *Server) handleAdminCarpools(w http.ResponseWriter, r *http.Request) {
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
	listings, appErr := s.carpools.AdminCarpoolListings(r.Context(), user, carpoolListingFilter(r), pageRequest)
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
	user, _, appErr := s.requireSession(w, r)
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
	user, _, appErr := s.requireSessionAndCSRF(w, r)
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
	completion, appErr := s.carpools.UpdateCarpoolListingReviewStatusWithIdempotency(
		r.Context(), user, routeKey, r.Header.Get("Idempotency-Key"), requestHash(http.MethodPost, routeKey, body),
		carpool.ReviewInput{
			ListingID:       listingID,
			Action:          action,
			Status:          status,
			Reason:          req.Reason,
			ExpectedVersion: version,
			RequestID:       requestIDFrom(r),
		}, carpoolListingCompletionBuilder(http.StatusOK),
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	restoreCarpoolListingETag(&completion)
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleCreateCarpoolApplication(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if !requireCapability(w, r, user, auth.CapabilityCarpoolApply) {
		return
	}
	body, req, appErr := decodeStrictJSON[createCarpoolApplicationRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	listingID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/carpools/{id}/applications:" + listingID
	completion, appErr := s.carpools.CreateCarpoolApplicationWithIdempotency(
		r.Context(), user, routeKey, r.Header.Get("Idempotency-Key"), requestHash(r.Method, routeKey, body),
		carpool.CreateApplicationInput{
			ListingID:            listingID,
			BuyerContactMethodID: req.BuyerContactMethodID,
			RiskAcknowledgement:  toAppRiskAck(req.RiskAcknowledgement),
			RequestID:            requestIDFrom(r),
		}, carpoolApplicationCompletionBuilder(http.StatusCreated),
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	restoreCarpoolApplicationETag(&completion)
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleMyCarpoolApplications(w http.ResponseWriter, r *http.Request) {
	actor, appErr := s.requireBusinessActor(r, true, false)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	applications, appErr := s.carpoolContinuity.CarpoolApplicationsForActor(r.Context(), actor, carpool.JoinActorBuyer)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	memberships, appErr := s.carpoolContinuity.CarpoolMembershipsForActor(r.Context(), actor, carpool.JoinActorBuyer)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toCarpoolApplicationResponses(filterCarpoolApplications(r, applications, memberships)))
}

func (s *Server) handleMyCarpoolApplication(w http.ResponseWriter, r *http.Request) {
	actor, appErr := s.requireBusinessActor(r, true, false)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	application, appErr := s.carpoolContinuity.CarpoolApplicationForActor(r.Context(), actor, chi.URLParam(r, "id"), carpool.JoinActorBuyer)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, application.Version)
	writeJSON(w, http.StatusOK, toCarpoolApplicationResponse(application))
}

func (s *Server) handleConfirmCarpoolApplicationConditions(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if !requireCapability(w, r, user, auth.CapabilityCarpoolApply) {
		return
	}
	_, _, appErr = decodeStrictJSON[emptyRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	application, appErr := s.carpools.ConfirmCarpoolApplicationConditions(r.Context(), user, carpool.ConfirmApplicationConditionsInput{
		ApplicationID:   chi.URLParam(r, "id"),
		ExpectedVersion: version,
		RequestID:       requestIDFrom(r),
	})
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, application.Version)
	writeJSON(w, http.StatusOK, toCarpoolApplicationResponse(application))
}

func (s *Server) handleCancelCarpoolApplication(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
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
	restoreCarpoolApplicationETag(&completion)
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleBuyerLeaveCarpoolMembership(w http.ResponseWriter, r *http.Request) {
	s.handleEndCarpoolMembership(w, r, carpool.JoinActorBuyer, carpool.MembershipStatusLeft)
}

func (s *Server) handleMyCarpoolMemberships(w http.ResponseWriter, r *http.Request) {
	actor, appErr := s.requireBusinessActor(r, true, false)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	memberships, appErr := s.carpoolContinuity.CarpoolMembershipsForActor(r.Context(), actor, carpool.JoinActorBuyer)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toCarpoolMembershipResponses(memberships))
}
func (s *Server) handleOwnerCarpoolApplications(w http.ResponseWriter, r *http.Request) {
	actor, appErr := s.requireBusinessActor(r, true, false)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	applications, appErr := s.carpoolContinuity.CarpoolApplicationsForActor(r.Context(), actor, carpool.JoinActorOwner)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	memberships, appErr := s.carpoolContinuity.CarpoolMembershipsForActor(r.Context(), actor, carpool.JoinActorOwner)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toCarpoolApplicationResponses(filterCarpoolApplications(r, applications, memberships)))
}

func (s *Server) handleOwnerCarpoolApplication(w http.ResponseWriter, r *http.Request) {
	actor, appErr := s.requireBusinessActor(r, true, false)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	application, appErr := s.carpoolContinuity.CarpoolApplicationForActor(r.Context(), actor, chi.URLParam(r, "id"), carpool.JoinActorOwner)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, application.Version)
	writeJSON(w, http.StatusOK, toCarpoolApplicationResponse(application))
}

func (s *Server) handleAcceptCarpoolApplication(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if !requireCapability(w, r, user, auth.CapabilityCarpoolPublish) {
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
		user,
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
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if !requireCapability(w, r, user, auth.CapabilityCarpoolPublish) {
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
	completion, appErr := s.carpools.RejectCarpoolApplicationWithIdempotency(
		r.Context(), user, routeKey, r.Header.Get("Idempotency-Key"), requestHash(r.Method, routeKey, body),
		carpool.RejectApplicationInput{
			ApplicationID:   applicationID,
			OwnerUserID:     user.ID,
			Reason:          req.Reason,
			ExpectedVersion: version,
			RequestID:       requestIDFrom(r),
		}, carpoolApplicationCompletionBuilder(http.StatusOK),
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleOwnerRemoveCarpoolMembership(w http.ResponseWriter, r *http.Request) {
	s.handleEndCarpoolMembership(w, r, carpool.JoinActorOwner, carpool.MembershipStatusRemoved)
}

func (s *Server) handleOwnerCarpoolMemberships(w http.ResponseWriter, r *http.Request) {
	actor, appErr := s.requireBusinessActor(r, true, false)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	memberships, appErr := s.carpoolContinuity.CarpoolMembershipsForActor(r.Context(), actor, carpool.JoinActorOwner)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toCarpoolMembershipResponses(memberships))
}
func (s *Server) handleEndCarpoolMembership(w http.ResponseWriter, r *http.Request, actorRole, targetStatus string) {
	actor, appErr := s.requireBusinessActor(r, true, true)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if actor.Audience == auth.SessionAudienceNormal && actorRole == carpool.JoinActorOwner && !requireActorCapability(w, r, actor, auth.CapabilityCarpoolPublish) {
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
	completion, appErr := s.carpoolContinuity.EndCarpoolMembershipForActorWithIdempotency(
		r.Context(),
		actor,
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

func carpoolListingCompletionBuilder(status int) carpool.ListingCompletionBuilder {
	return func(listing carpool.Listing) (idempotency.Completion, *domain.AppError) {
		body, err := json.Marshal(toCarpoolListingResponse(listing))
		if err != nil {
			return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "车源响应编码失败。")
		}
		return idempotency.Completion{
			Status: status, ContentType: "application/json; charset=utf-8", Body: body,
			ResourceType: "carpool_listing", ResourceID: listing.ID,
			Headers: map[string]string{"ETag": `"` + strconv.FormatInt(listing.Version, 10) + `"`},
		}, nil
	}
}

func carpoolApplicationCompletionBuilder(status int) carpool.ApplicationCompletionBuilder {
	return func(application carpool.Application) (idempotency.Completion, *domain.AppError) {
		body, err := json.Marshal(toCarpoolApplicationResponse(application))
		if err != nil {
			return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "拼车申请响应编码失败。")
		}
		return idempotency.Completion{
			Status: status, ContentType: "application/json; charset=utf-8", Body: body,
			ResourceType: "carpool_application", ResourceID: application.ID,
			Headers: map[string]string{"ETag": `"` + strconv.FormatInt(application.Version, 10) + `"`},
		}, nil
	}
}

func restoreCarpoolApplicationETag(completion *idempotency.Completion) {
	if completion == nil || len(completion.Body) == 0 || completion.Headers != nil && completion.Headers["ETag"] != "" {
		return
	}
	var payload struct {
		Version int64 `json:"version"`
	}
	if err := json.Unmarshal(completion.Body, &payload); err != nil || payload.Version <= 0 {
		return
	}
	if completion.Headers == nil {
		completion.Headers = make(map[string]string)
	}
	completion.Headers["ETag"] = `"` + strconv.FormatInt(payload.Version, 10) + `"`
}

func restoreCarpoolListingETag(completion *idempotency.Completion) {
	if completion == nil || len(completion.Body) == 0 || completion.Headers != nil && completion.Headers["ETag"] != "" {
		return
	}
	var payload struct {
		Version int64 `json:"version"`
	}
	if err := json.Unmarshal(completion.Body, &payload); err != nil || payload.Version <= 0 {
		return
	}
	if completion.Headers == nil {
		completion.Headers = make(map[string]string)
	}
	completion.Headers["ETag"] = `"` + strconv.FormatInt(payload.Version, 10) + `"`
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
		ID:                                    listing.ID,
		OwnerUserID:                           listing.OwnerUserID,
		ProductPlanID:                         listing.ProductPlanID,
		OwnerContactMethodID:                  listing.OwnerContactMethodID,
		CycleTerm:                             cycleTerm,
		Title:                                 listing.Title,
		Summary:                               listing.Summary,
		AccessArrangement:                     listing.AccessArrangement,
		DistributionMethod:                    listing.DistributionMethod,
		DistributionMethodNote:                listing.DistributionMethodNote,
		ProvidesAdminAccount:                  listing.ProvidesAdminAccount,
		RegionCode:                            listing.RegionCode,
		RegionName:                            listing.RegionName,
		SourceURL:                             listing.SourceURL,
		PriceMonthlyCNY:                       listing.PriceMonthlyCNY,
		ServiceMultiplier:                     listing.ServiceMultiplier,
		DailyQuotaAmount:                      listing.DailyQuotaAmount,
		WeeklyQuotaAmount:                     listing.WeeklyQuotaAmount,
		FollowsOfficialQuotaReset:             listing.FollowsOfficialQuotaReset,
		VPSRegion:                             listing.VPSRegion,
		SupportsMainlandChinaDirectConnection: listing.SupportsMainlandChinaDirectConnection,
		OpeningChannelCode:                    listing.OpeningChannelCode,
		CustomOpeningChannel:                  listing.CustomOpeningChannel,
		PaymentMethodCode:                     listing.PaymentMethodCode,
		CustomPaymentMethod:                   listing.CustomPaymentMethod,
		QuotaLabel:                            listing.QuotaLabel,
		QuotaUnit:                             listing.QuotaUnit,
		QuotaPeriod:                           listing.QuotaPeriod,
		BuyerSeatCapacity:                     listing.BuyerSeatCapacity,
		OfflineOccupiedSeats:                  listing.OfflineOccupiedSeats,
		ActiveBuyerMembers:                    listing.ActiveBuyerMembers,
		AvailableSeats:                        listing.AvailableSeats,
		Status:                                listing.Status,
		GovernanceStatus:                      listing.GovernanceStatus,
		RecruitmentStopReason:                 listing.RecruitmentStopReason,
		ConditionsVersion:                     listing.ConditionsVersion,
		ReviewReason:                          reviewReason,
		ReviewedAt:                            reviewedAt,
		PolicyVersion:                         listing.PolicyVersion,
		RiskNoticeCode:                        listing.RiskNoticeCode,
		RiskAckRequired:                       listing.RiskAckRequired,
		Version:                               listing.Version,
		CreatedAt:                             listing.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:                             listing.UpdatedAt.UTC().Format(time.RFC3339),
		ApplicationEligibility:                toCarpoolApplicationEligibilityResponse(carpool.EvaluatePublicListingEligibility(listing)),
		SellerReputation:                      toReputationSummary(listing.SellerReputation),
		SourceAuthorVerification:              toSourceAuthorResourceSummaryResponse(listing.SourceAuthorVerification),
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
	var joinedAt *string
	if application.JoinedAt != nil {
		formatted := application.JoinedAt.UTC().Format(time.RFC3339)
		joinedAt = &formatted
	}
	return carpoolApplicationResponse{
		ID:                        application.ID,
		CarpoolListingID:          application.CarpoolListingID,
		BuyerUserID:               application.BuyerUserID,
		OwnerUserID:               application.OwnerUserID,
		ProductPlanID:             application.ProductPlanID,
		BuyerContactMethodID:      application.BuyerContactMethodID,
		Status:                    application.Status,
		SeatCount:                 application.SeatCount,
		ListingTitleSnapshot:      application.ListingTitleSnapshot,
		PriceMonthlyCNY:           application.PriceMonthlyCNY,
		PolicyVersionSnapshot:     application.PolicyVersionSnapshot,
		RiskNoticeCode:            application.RiskNoticeCode,
		ConditionsVersionSnapshot: application.ConditionsVersionSnapshot,
		ConditionsSnapshot:        application.ConditionsSnapshot,
		AcceptedConditionsVersion: application.AcceptedConditionsVersion,
		ConditionsAcceptedAt:      application.ConditionsAcceptedAt.UTC().Format(time.RFC3339),
		ContactSessionID:          application.ContactSessionID,
		JoinedAt:                  joinedAt,
		DecisionReason:            decisionReason,
		DecidedAt:                 decidedAt,
		Version:                   application.Version,
		CreatedAt:                 application.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:                 application.UpdatedAt.UTC().Format(time.RFC3339),
		BuyerReputation:           toReputationSummary(application.BuyerReputation),
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
	var endedAt *string
	if membership.EndedAt != nil {
		formatted := membership.EndedAt.UTC().Format(time.RFC3339)
		endedAt = &formatted
	}
	return carpoolMembershipResponse{
		ID:                        membership.ID,
		CarpoolListingID:          membership.CarpoolListingID,
		CarpoolApplicationID:      membership.CarpoolApplicationID,
		CycleTermID:               membership.CycleTermID,
		BuyerUserID:               membership.BuyerUserID,
		OwnerUserID:               membership.OwnerUserID,
		ProductPlanID:             membership.ProductPlanID,
		Status:                    membership.Status,
		SeatCount:                 membership.SeatCount,
		PriceMonthlyCNY:           membership.PriceMonthlyCNY,
		PolicyVersionSnapshot:     membership.PolicyVersionSnapshot,
		RiskNoticeCode:            membership.RiskNoticeCode,
		ConditionsVersionSnapshot: membership.ConditionsVersionSnapshot,
		ConditionsSnapshot:        membership.ConditionsSnapshot,
		JoinedAt:                  membership.JoinedAt.UTC().Format(time.RFC3339),
		EndedAt:                   endedAt,
		EndedReason:               membership.EndedReason,
		EndedByUserID:             membership.EndedByUserID,
		Version:                   membership.Version,
		CreatedAt:                 membership.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:                 membership.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
