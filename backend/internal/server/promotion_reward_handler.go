package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/promotionreward"

	"github.com/go-chi/chi/v5"
)

type promotionRewardPublicConfigResponse struct {
	ProgramEnabled         bool    `json:"programEnabled"`
	WelcomeEnabled         bool    `json:"welcomeEnabled"`
	ReferralEnabled        bool    `json:"referralEnabled"`
	PromotionDurationHours int     `json:"promotionDurationHours"`
	CouponValidDays        int     `json:"couponValidDays"`
	RewardDelayHours       int     `json:"rewardDelayHours"`
	InviterMonthlyLimit    int     `json:"inviterMonthlyLimit"`
	RulesText              string  `json:"rulesText"`
	StartsAt               string  `json:"startsAt,omitempty"`
	EndsAt                 *string `json:"endsAt,omitempty"`
}

type referralStatisticsResponse struct {
	InvitedCount            int `json:"invitedCount"`
	QualifiedCount          int `json:"qualifiedCount"`
	RewardedCount           int `json:"rewardedCount"`
	PendingCount            int `json:"pendingCount"`
	InviterRewardsThisMonth int `json:"inviterRewardsThisMonth"`
	InviterRewardsRemaining int `json:"inviterRewardsRemaining"`
}

type referralRecordResponse struct {
	ID                    string   `json:"id"`
	InviterUserID         string   `json:"inviterUserId,omitempty"`
	InviteeUserID         string   `json:"inviteeUserId,omitempty"`
	InviterDisplayName    string   `json:"inviterDisplayName"`
	InviteeDisplayName    string   `json:"inviteeDisplayName"`
	Status                string   `json:"status"`
	BoundAt               string   `json:"boundAt"`
	QualifiedAt           *string  `json:"qualifiedAt,omitempty"`
	RewardedAt            *string  `json:"rewardedAt,omitempty"`
	QualifiedAPIServiceID string   `json:"qualifiedApiServiceId,omitempty"`
	RejectedAt            *string  `json:"rejectedAt,omitempty"`
	RejectedReason        string   `json:"rejectedReason,omitempty"`
	RevokedAt             *string  `json:"revokedAt,omitempty"`
	RevokedReason         string   `json:"revokedReason,omitempty"`
	RiskFlags             []string `json:"riskFlags,omitempty"`
	CreatedAt             string   `json:"createdAt"`
	UpdatedAt             string   `json:"updatedAt"`
	Version               int64    `json:"version"`
}

type referralSummaryResponse struct {
	Code       string                              `json:"code"`
	Statistics referralStatisticsResponse          `json:"statistics"`
	Records    []referralRecordResponse            `json:"records"`
	Campaign   promotionRewardPublicConfigResponse `json:"campaign"`
}

type promotionCouponResponse struct {
	ID                  string  `json:"id"`
	CampaignID          string  `json:"campaignId,omitempty"`
	UserID              string  `json:"userId,omitempty"`
	UserDisplayName     string  `json:"userDisplayName,omitempty"`
	SourceType          string  `json:"sourceType"`
	Status              string  `json:"status"`
	AvailableAt         string  `json:"availableAt"`
	ExpiresAt           string  `json:"expiresAt"`
	DurationHours       int     `json:"durationHours"`
	UsedAPIServiceID    string  `json:"usedApiServiceId,omitempty"`
	UsedAPIServiceTitle string  `json:"usedApiServiceTitle,omitempty"`
	ActivationID        string  `json:"activationId,omitempty"`
	PromotionStartsAt   *string `json:"promotionStartsAt,omitempty"`
	PromotionEndsAt     *string `json:"promotionEndsAt,omitempty"`
	UsedAt              *string `json:"usedAt,omitempty"`
	RevokedAt           *string `json:"revokedAt,omitempty"`
	RevokedReason       string  `json:"revokedReason,omitempty"`
	GrantReason         string  `json:"grantReason,omitempty"`
	CreatedAt           string  `json:"createdAt"`
	UpdatedAt           string  `json:"updatedAt"`
	Version             int64   `json:"version"`
}

type promotionRewardPaginationResponse struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	TotalItems int `json:"totalItems"`
	TotalPages int `json:"totalPages"`
}

type promotionCouponPageResponse struct {
	Items      []promotionCouponResponse         `json:"items"`
	Pagination promotionRewardPaginationResponse `json:"pagination"`
}

type referralPageResponse struct {
	Items      []referralRecordResponse          `json:"items"`
	Pagination promotionRewardPaginationResponse `json:"pagination"`
}

type promotionRewardCampaignResponse struct {
	ID                     string  `json:"id"`
	Code                   string  `json:"code"`
	ProgramEnabled         bool    `json:"programEnabled"`
	WelcomeEnabled         bool    `json:"welcomeEnabled"`
	ReferralEnabled        bool    `json:"referralEnabled"`
	StartsAt               string  `json:"startsAt"`
	EndsAt                 *string `json:"endsAt,omitempty"`
	PromotionDurationHours int     `json:"promotionDurationHours"`
	CouponValidDays        int     `json:"couponValidDays"`
	RewardDelayHours       int     `json:"rewardDelayHours"`
	InviterMonthlyLimit    int     `json:"inviterMonthlyLimit"`
	RulesText              string  `json:"rulesText"`
	CreatedAt              string  `json:"createdAt"`
	UpdatedAt              string  `json:"updatedAt"`
	Version                int64   `json:"version"`
}

type applyPromotionCouponRequest struct {
	APIServiceID string `json:"apiServiceId"`
}

type updatePromotionRewardCampaignRequest struct {
	ProgramEnabled         bool   `json:"programEnabled"`
	WelcomeEnabled         bool   `json:"welcomeEnabled"`
	ReferralEnabled        bool   `json:"referralEnabled"`
	StartsAt               string `json:"startsAt"`
	EndsAt                 string `json:"endsAt"`
	PromotionDurationHours int    `json:"promotionDurationHours"`
	CouponValidDays        int    `json:"couponValidDays"`
	RewardDelayHours       int    `json:"rewardDelayHours"`
	InviterMonthlyLimit    int    `json:"inviterMonthlyLimit"`
	RulesText              string `json:"rulesText"`
	Reason                 string `json:"reason"`
}

type grantPromotionCouponRequest struct {
	UserID        string `json:"userId"`
	DurationHours int    `json:"durationHours"`
	ValidDays     int    `json:"validDays"`
	Reason        string `json:"reason"`
}

func (s *Server) handlePromotionRewardPublicConfig(w http.ResponseWriter, r *http.Request) {
	config, appErr := s.promotionRewards.PromotionRewardPublicConfig(r.Context())
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toPromotionRewardPublicConfigResponse(config))
}

func (s *Server) handleMyReferralSummary(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	summary, appErr := s.promotionRewards.MyReferralSummary(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toReferralSummaryResponse(summary))
}

func (s *Server) handleMyPromotionCoupons(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	query, appErr := parsePromotionCouponQuery(r, false)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	page, appErr := s.promotionRewards.MyPromotionCoupons(r.Context(), user, query)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toPromotionCouponPageResponse(page, false))
}

func (s *Server) handleApplyPromotionCoupon(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, request, appErr := decodeStrictJSON[applyPromotionCouponRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	couponID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/me/promotion-coupons/{id}/apply:" + couponID
	completion, appErr := s.promotionRewards.ApplyPromotionCouponWithIdempotency(
		r.Context(), user, routeKey, r.Header.Get("Idempotency-Key"),
		requestHash(r.Method, routeKey, body), promotionreward.ApplyCouponInput{
			CouponID: couponID, APIServiceID: request.APIServiceID, RequestID: requestIDFrom(r),
		}, promotionCouponCompletionBuilder(http.StatusOK, false),
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeNoStoreIdempotencyCompletion(w, completion)
}

func (s *Server) handleAdminPromotionRewardCampaign(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	campaign, appErr := s.promotionRewards.AdminPromotionRewardCampaign(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	setETag(w, campaign.Version)
	writeJSON(w, http.StatusOK, toPromotionRewardCampaignResponse(campaign))
}

func (s *Server) handleUpdateAdminPromotionRewardCampaign(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, request, appErr := decodeStrictJSON[updatePromotionRewardCampaignRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	startsAt, appErr := parseRequiredTime(request.StartsAt, "startsAt")
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	var endsAt *time.Time
	if strings.TrimSpace(request.EndsAt) != "" {
		parsed, parseErr := parseRequiredTime(request.EndsAt, "endsAt")
		if parseErr != nil {
			writeProblem(w, r, parseErr)
			return
		}
		endsAt = &parsed
	}
	const routeKey = "PATCH /api/v1/admin/promotion-reward-campaign"
	completion, appErr := s.promotionRewards.UpdateAdminPromotionRewardCampaignWithIdempotency(r.Context(), user,
		routeKey, r.Header.Get("Idempotency-Key"), requestHash(r.Method, routeKey, body), promotionreward.UpdateCampaignInput{
			ProgramEnabled: request.ProgramEnabled, WelcomeEnabled: request.WelcomeEnabled,
			ReferralEnabled: request.ReferralEnabled, StartsAt: startsAt, EndsAt: endsAt,
			PromotionDurationHours: request.PromotionDurationHours, CouponValidDays: request.CouponValidDays,
			RewardDelayHours: request.RewardDelayHours, InviterMonthlyLimit: request.InviterMonthlyLimit,
			RulesText: request.RulesText, ExpectedVersion: version, Reason: request.Reason, RequestID: requestIDFrom(r),
		}, promotionRewardCampaignCompletionBuilder)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	restorePromotionRewardETag(&completion)
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleAdminReferrals(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	page, limit, appErr := parsePromotionRewardPage(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	result, appErr := s.promotionRewards.AdminReferrals(r.Context(), user, promotionreward.ReferralQuery{
		Page: page, Limit: limit, Status: r.URL.Query().Get("status"), Search: r.URL.Query().Get("search"),
	})
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toReferralPageResponse(result))
}

func (s *Server) handleRevokeAdminReferral(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, request, appErr := decodeStrictJSON[reviewActionRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	referralID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/admin/referrals/{id}/revoke:" + referralID
	completion, appErr := s.promotionRewards.RevokeAdminReferralWithIdempotency(r.Context(), user,
		routeKey, r.Header.Get("Idempotency-Key"), requestHash(r.Method, routeKey, body), promotionreward.RevokeReferralInput{
			ReferralID: referralID, ExpectedVersion: version,
			Reason: request.Reason, RequestID: requestIDFrom(r),
		}, promotionRewardReferralCompletionBuilder)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	restorePromotionRewardETag(&completion)
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleAdminPromotionCoupons(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	query, appErr := parsePromotionCouponQuery(r, true)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	page, appErr := s.promotionRewards.AdminPromotionCoupons(r.Context(), user, query)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toPromotionCouponPageResponse(page, true))
}

func (s *Server) handleGrantAdminPromotionCoupon(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, request, appErr := decodeStrictJSON[grantPromotionCouponRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	const routeKey = "POST /api/v1/admin/promotion-coupons/grant"
	completion, appErr := s.promotionRewards.GrantAdminPromotionCouponWithIdempotency(r.Context(), user,
		routeKey, r.Header.Get("Idempotency-Key"), requestHash(r.Method, routeKey, body), promotionreward.GrantCouponInput{
			UserID: request.UserID, DurationHours: request.DurationHours,
			ValidDays: request.ValidDays, Reason: request.Reason, RequestID: requestIDFrom(r),
		}, promotionCouponCompletionBuilder(http.StatusCreated, true))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	restorePromotionRewardETag(&completion)
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleRevokeAdminPromotionCoupon(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, request, appErr := decodeStrictJSON[reviewActionRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	couponID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/admin/promotion-coupons/{id}/revoke:" + couponID
	completion, appErr := s.promotionRewards.RevokeAdminPromotionCouponWithIdempotency(r.Context(), user,
		routeKey, r.Header.Get("Idempotency-Key"), requestHash(r.Method, routeKey, body), promotionreward.RevokeCouponInput{
			CouponID: couponID, ExpectedVersion: version,
			Reason: request.Reason, RequestID: requestIDFrom(r),
		}, promotionCouponCompletionBuilder(http.StatusOK, true))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	restorePromotionRewardETag(&completion)
	writeIdempotencyCompletion(w, completion)
}

func promotionRewardCampaignCompletionBuilder(item promotionreward.Campaign) (idempotency.Completion, *domain.AppError) {
	return promotionRewardCompletion(http.StatusOK, "promotion_reward_campaign", item.ID, item.Version, toPromotionRewardCampaignResponse(item))
}

func promotionRewardReferralCompletionBuilder(item promotionreward.ReferralRecord) (idempotency.Completion, *domain.AppError) {
	return promotionRewardCompletion(http.StatusOK, "referral_relation", item.ID, item.Version, toReferralRecordResponse(item, true))
}

func promotionCouponCompletionBuilder(status int, admin bool) promotionreward.CouponCompletionBuilder {
	return func(item promotionreward.Coupon) (idempotency.Completion, *domain.AppError) {
		return promotionRewardCompletion(status, "promotion_coupon", item.ID, item.Version, toPromotionCouponResponse(item, admin))
	}
}

func promotionRewardCompletion(status int, resourceType, resourceID string, version int64, payload any) (idempotency.Completion, *domain.AppError) {
	body, err := json.Marshal(payload)
	if err != nil {
		return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "推广权益响应编码失败。")
	}
	return idempotency.Completion{
		Status: status, ContentType: "application/json; charset=utf-8", Body: body,
		ResourceType: resourceType, ResourceID: resourceID,
		Headers: map[string]string{"ETag": `"` + strconv.FormatInt(version, 10) + `"`},
	}, nil
}

func restorePromotionRewardETag(completion *idempotency.Completion) {
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

func parsePromotionCouponQuery(r *http.Request, admin bool) (promotionreward.CouponQuery, *domain.AppError) {
	page, limit, appErr := parsePromotionRewardPage(r)
	if appErr != nil {
		return promotionreward.CouponQuery{}, appErr
	}
	query := promotionreward.CouponQuery{Page: page, Limit: limit, Status: r.URL.Query().Get("status")}
	if admin {
		query.SourceType = r.URL.Query().Get("source")
		query.Search = r.URL.Query().Get("search")
	}
	return query, nil
}

func parsePromotionRewardPage(r *http.Request) (int, int, *domain.AppError) {
	page := promotionreward.DefaultPage
	limit := promotionreward.DefaultPageLimit
	if raw := strings.TrimSpace(r.URL.Query().Get("page")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			return 0, 0, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid page", "分页 page 必须是正整数。", "page", "invalid", "page 必须是正整数。")
		}
		page = parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > promotionreward.MaximumPageLimit {
			return 0, 0, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid limit", "分页 limit 必须是 1 到 100 之间的整数。", "limit", "invalid", "limit 必须是 1 到 100 之间的整数。")
		}
		limit = parsed
	}
	return page, limit, nil
}

func toPromotionRewardPublicConfigResponse(item promotionreward.PublicConfig) promotionRewardPublicConfigResponse {
	return promotionRewardPublicConfigResponse{
		ProgramEnabled: item.ProgramEnabled, WelcomeEnabled: item.WelcomeEnabled,
		ReferralEnabled: item.ReferralEnabled, PromotionDurationHours: item.PromotionDurationHours,
		CouponValidDays: item.CouponValidDays, RewardDelayHours: item.RewardDelayHours,
		InviterMonthlyLimit: item.InviterMonthlyLimit, RulesText: item.RulesText,
		StartsAt: formatPromotionRewardTime(item.StartsAt), EndsAt: formatPromotionRewardTimePointer(item.EndsAt),
	}
}

func toPromotionRewardCampaignResponse(item promotionreward.Campaign) promotionRewardCampaignResponse {
	return promotionRewardCampaignResponse{
		ID: item.ID, Code: item.Code, ProgramEnabled: item.ProgramEnabled,
		WelcomeEnabled: item.WelcomeEnabled, ReferralEnabled: item.ReferralEnabled,
		StartsAt: formatPromotionRewardTime(item.StartsAt), EndsAt: formatPromotionRewardTimePointer(item.EndsAt),
		PromotionDurationHours: item.PromotionDurationHours, CouponValidDays: item.CouponValidDays,
		RewardDelayHours: item.RewardDelayHours, InviterMonthlyLimit: item.InviterMonthlyLimit,
		RulesText: item.RulesText, CreatedAt: formatPromotionRewardTime(item.CreatedAt),
		UpdatedAt: formatPromotionRewardTime(item.UpdatedAt), Version: item.Version,
	}
}

func toReferralSummaryResponse(item promotionreward.ReferralSummary) referralSummaryResponse {
	records := make([]referralRecordResponse, 0, len(item.Records))
	for _, record := range item.Records {
		records = append(records, toReferralRecordResponse(record, false))
	}
	return referralSummaryResponse{
		Code: item.Code,
		Statistics: referralStatisticsResponse{
			InvitedCount: item.Statistics.InvitedCount, QualifiedCount: item.Statistics.QualifiedCount,
			RewardedCount: item.Statistics.RewardedCount, PendingCount: item.Statistics.PendingCount,
			InviterRewardsThisMonth: item.Statistics.InviterRewardsThisMonth,
			InviterRewardsRemaining: item.Statistics.InviterRewardsRemaining,
		},
		Records: records, Campaign: toPromotionRewardPublicConfigResponse(item.Campaign),
	}
}

func toReferralRecordResponse(item promotionreward.ReferralRecord, admin bool) referralRecordResponse {
	response := referralRecordResponse{
		ID: item.ID, InviterDisplayName: item.InviterDisplayName, InviteeDisplayName: item.InviteeDisplayName,
		Status: item.Status, BoundAt: formatPromotionRewardTime(item.BoundAt),
		QualifiedAt: formatPromotionRewardTimePointer(item.QualifiedAt), RewardedAt: formatPromotionRewardTimePointer(item.RewardedAt),
		QualifiedAPIServiceID: item.QualifiedAPIServiceID, RejectedAt: formatPromotionRewardTimePointer(item.RejectedAt),
		RejectedReason: item.RejectedReason, RevokedAt: formatPromotionRewardTimePointer(item.RevokedAt),
		RevokedReason: item.RevokedReason, CreatedAt: formatPromotionRewardTime(item.CreatedAt),
		UpdatedAt: formatPromotionRewardTime(item.UpdatedAt), Version: item.Version,
	}
	if admin {
		response.InviterUserID = item.InviterUserID
		response.InviteeUserID = item.InviteeUserID
		response.RiskFlags = nonNilStrings(item.RiskFlags)
	}
	return response
}

func toPromotionCouponResponse(item promotionreward.Coupon, admin bool) promotionCouponResponse {
	response := promotionCouponResponse{
		ID: item.ID, SourceType: item.SourceType, Status: item.Status,
		AvailableAt: formatPromotionRewardTime(item.AvailableAt), ExpiresAt: formatPromotionRewardTime(item.ExpiresAt),
		DurationHours: item.DurationHours, UsedAPIServiceID: item.UsedAPIServiceID,
		UsedAPIServiceTitle: item.UsedAPIServiceTitle, ActivationID: item.ActivationID,
		PromotionStartsAt: formatPromotionRewardTimePointer(item.PromotionStartsAt),
		PromotionEndsAt:   formatPromotionRewardTimePointer(item.PromotionEndsAt), UsedAt: formatPromotionRewardTimePointer(item.UsedAt),
		RevokedAt: formatPromotionRewardTimePointer(item.RevokedAt), RevokedReason: item.RevokedReason,
		CreatedAt: formatPromotionRewardTime(item.CreatedAt), UpdatedAt: formatPromotionRewardTime(item.UpdatedAt), Version: item.Version,
	}
	if admin {
		response.CampaignID = item.CampaignID
		response.UserID = item.UserID
		response.UserDisplayName = item.UserDisplayName
		response.GrantReason = item.GrantReason
	}
	return response
}

func toPromotionCouponPageResponse(item promotionreward.CouponPage, admin bool) promotionCouponPageResponse {
	items := make([]promotionCouponResponse, 0, len(item.Items))
	for _, coupon := range item.Items {
		items = append(items, toPromotionCouponResponse(coupon, admin))
	}
	return promotionCouponPageResponse{Items: items, Pagination: toPromotionRewardPaginationResponse(item.Pagination)}
}

func toReferralPageResponse(item promotionreward.ReferralPage) referralPageResponse {
	items := make([]referralRecordResponse, 0, len(item.Items))
	for _, record := range item.Items {
		items = append(items, toReferralRecordResponse(record, true))
	}
	return referralPageResponse{Items: items, Pagination: toPromotionRewardPaginationResponse(item.Pagination)}
}

func toPromotionRewardPaginationResponse(item promotionreward.Pagination) promotionRewardPaginationResponse {
	return promotionRewardPaginationResponse{Page: item.Page, Limit: item.Limit, TotalItems: item.TotalItems, TotalPages: item.TotalPages}
}

func formatPromotionRewardTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func formatPromotionRewardTimePointer(value *time.Time) *string {
	if value == nil || value.IsZero() {
		return nil
	}
	formatted := formatPromotionRewardTime(*value)
	return &formatted
}
