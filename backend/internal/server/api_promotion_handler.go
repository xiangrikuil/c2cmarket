package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apipromotion"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/go-chi/chi/v5"
)

type createAPIPromotionRequest struct {
	APIServiceID string `json:"apiServiceId"`
	Placement    string `json:"placement"`
	StartsAt     string `json:"startsAt"`
	EndsAt       string `json:"endsAt"`
	Reason       string `json:"reason"`
}

type apiPromotionEligibilityResponse struct {
	Configurable       bool     `json:"configurable"`
	Displayable        bool     `json:"displayable"`
	HardBlockReasons   []string `json:"hardBlockReasons"`
	WarningReasons     []string `json:"warningReasons"`
	SuppressionReasons []string `json:"suppressionReasons"`
}

type apiPromotionAvailabilityResponse struct {
	Eligibility          apiPromotionEligibilityResponse `json:"eligibility"`
	OverlappingCampaigns int                             `json:"overlappingCampaigns"`
	Capacity             int                             `json:"capacity"`
	RemainingCapacity    int                             `json:"remainingCapacity"`
	SameServiceOverlap   bool                            `json:"sameServiceOverlap"`
}

type publicAPIPromotionResponse struct {
	PromotionID string                   `json:"promotionId"`
	Kind        string                   `json:"kind"`
	Placement   string                   `json:"placement"`
	Label       string                   `json:"label"`
	StartsAt    string                   `json:"startsAt"`
	EndsAt      string                   `json:"endsAt"`
	Service     publicAPIServiceResponse `json:"service"`
}

type adminAPIPromotionResponse struct {
	ID                   string                          `json:"id"`
	APIServiceID         string                          `json:"apiServiceId"`
	Placement            string                          `json:"placement"`
	StartsAt             string                          `json:"startsAt"`
	EndsAt               string                          `json:"endsAt"`
	Status               string                          `json:"status"`
	CreatedReason        string                          `json:"createdReason"`
	CreatedByAdminID     string                          `json:"createdByAdminId"`
	StoppedAt            *string                         `json:"stoppedAt"`
	StoppedByAdminID     string                          `json:"stoppedByAdminId,omitempty"`
	StoppedReason        string                          `json:"stoppedReason,omitempty"`
	Eligibility          apiPromotionEligibilityResponse `json:"eligibility"`
	OverlappingCampaigns int                             `json:"overlappingCampaigns"`
	Capacity             int                             `json:"capacity"`
	Service              apiServiceResponse              `json:"service"`
	CreatedAt            string                          `json:"createdAt"`
	UpdatedAt            string                          `json:"updatedAt"`
	Version              int64                           `json:"version"`
}

func (s *Server) handlePublicAPIPromotions(w http.ResponseWriter, r *http.Request) {
	items, appErr := s.apiPromotions.PublicAPIPromotions(r.Context(), r.URL.Query().Get("placement"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[publicAPIPromotionResponse]{
		Items: toPublicAPIPromotionResponses(items),
	})
}

func (s *Server) handleAdminAPIPromotions(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items, appErr := s.apiPromotions.AdminAPIPromotions(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toAdminAPIPromotionResponses(items))
}

func (s *Server) handleAPIPromotionAvailability(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	startsAt, appErr := parseRequiredTime(r.URL.Query().Get("startsAt"), "startsAt")
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	endsAt, appErr := parseRequiredTime(r.URL.Query().Get("endsAt"), "endsAt")
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	availability, appErr := s.apiPromotions.APIPromotionAvailability(r.Context(), user, apipromotion.AvailabilityInput{
		APIServiceID: r.URL.Query().Get("apiServiceId"),
		Placement:    r.URL.Query().Get("placement"),
		StartsAt:     startsAt,
		EndsAt:       endsAt,
	})
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toAPIPromotionAvailabilityResponse(availability))
}

func (s *Server) handleCreateAPIPromotion(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, request, appErr := decodeStrictJSON[createAPIPromotionRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	startsAt, appErr := parseRequiredTime(request.StartsAt, "startsAt")
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	endsAt, appErr := parseRequiredTime(request.EndsAt, "endsAt")
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	const routeKey = "POST /api/v1/admin/api-service-promotions"
	completion, appErr := s.apiPromotions.CreateAPIPromotionWithIdempotency(
		r.Context(),
		user,
		routeKey,
		r.Header.Get("Idempotency-Key"),
		requestHash(r.Method, routeKey, body),
		apipromotion.CreateInput{
			APIServiceID: request.APIServiceID,
			Placement:    request.Placement,
			StartsAt:     startsAt,
			EndsAt:       endsAt,
			Reason:       request.Reason,
			RequestID:    requestIDFrom(r),
		},
		apiPromotionCompletionBuilder(http.StatusCreated),
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	restoreAPIPromotionETag(&completion)
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleStopAPIPromotion(w http.ResponseWriter, r *http.Request) {
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
	promotionID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/admin/api-service-promotions/{id}/stop:" + promotionID
	completion, appErr := s.apiPromotions.StopAPIPromotionWithIdempotency(
		r.Context(),
		user,
		routeKey,
		r.Header.Get("Idempotency-Key"),
		requestHash(r.Method, routeKey, body),
		apipromotion.StopInput{
			PromotionID:     promotionID,
			Reason:          request.Reason,
			ExpectedVersion: version,
			RequestID:       requestIDFrom(r),
		},
		apiPromotionCompletionBuilder(http.StatusOK),
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	restoreAPIPromotionETag(&completion)
	writeIdempotencyCompletion(w, completion)
}

func apiPromotionCompletionBuilder(status int) apipromotion.CompletionBuilder {
	return func(item apipromotion.Promotion) (idempotency.Completion, *domain.AppError) {
		body, err := json.Marshal(toAdminAPIPromotionResponse(item))
		if err != nil {
			return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "推广响应编码失败。")
		}
		return idempotency.Completion{
			Status:       status,
			ContentType:  "application/json; charset=utf-8",
			Body:         body,
			ResourceType: "api_service_promotion",
			ResourceID:   item.ID,
			Headers: map[string]string{
				"ETag": `"` + strconv.FormatInt(item.Version, 10) + `"`,
			},
		}, nil
	}
}

func restoreAPIPromotionETag(completion *idempotency.Completion) {
	if completion == nil || len(completion.Body) == 0 || completion.Headers != nil && completion.Headers["ETag"] != "" {
		return
	}
	var payload adminAPIPromotionResponse
	if err := json.Unmarshal(completion.Body, &payload); err != nil || payload.Version <= 0 {
		return
	}
	if completion.Headers == nil {
		completion.Headers = make(map[string]string)
	}
	completion.Headers["ETag"] = `"` + strconv.FormatInt(payload.Version, 10) + `"`
}

func toPublicAPIPromotionResponses(items []apipromotion.Promotion) []publicAPIPromotionResponse {
	responses := make([]publicAPIPromotionResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, publicAPIPromotionResponse{
			PromotionID: item.ID,
			Kind:        item.Kind,
			Placement:   item.Placement,
			Label:       "推广",
			StartsAt:    item.StartsAt.UTC().Format(time.RFC3339),
			EndsAt:      item.EndsAt.UTC().Format(time.RFC3339),
			Service:     toPublicAPIServiceResponse(item.Service),
		})
	}
	return responses
}

func toAdminAPIPromotionResponses(items []apipromotion.Promotion) []adminAPIPromotionResponse {
	responses := make([]adminAPIPromotionResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, toAdminAPIPromotionResponse(item))
	}
	return responses
}

func toAPIPromotionAvailabilityResponse(value apipromotion.Availability) apiPromotionAvailabilityResponse {
	return apiPromotionAvailabilityResponse{
		Eligibility: apiPromotionEligibilityResponse{
			Configurable:       value.Eligibility.Configurable,
			Displayable:        value.Eligibility.Displayable,
			HardBlockReasons:   nonNilStrings(value.Eligibility.HardBlockReasons),
			WarningReasons:     nonNilStrings(value.Eligibility.WarningReasons),
			SuppressionReasons: nonNilStrings(value.Eligibility.SuppressionReasons),
		},
		OverlappingCampaigns: value.OverlappingCampaigns,
		Capacity:             value.Capacity,
		RemainingCapacity:    value.RemainingCapacity,
		SameServiceOverlap:   value.SameServiceOverlap,
	}
}

func toAdminAPIPromotionResponse(item apipromotion.Promotion) adminAPIPromotionResponse {
	return adminAPIPromotionResponse{
		ID:               item.ID,
		APIServiceID:     item.APIServiceID,
		Placement:        item.Placement,
		StartsAt:         item.StartsAt.UTC().Format(time.RFC3339),
		EndsAt:           item.EndsAt.UTC().Format(time.RFC3339),
		Status:           item.Status,
		CreatedReason:    item.CreatedReason,
		CreatedByAdminID: item.CreatedByAdminID,
		StoppedAt:        formatOptionalTime(item.StoppedAt),
		StoppedByAdminID: item.StoppedByAdminID,
		StoppedReason:    item.StoppedReason,
		Eligibility: apiPromotionEligibilityResponse{
			Configurable:       item.Eligibility.Configurable,
			Displayable:        item.Eligibility.Displayable,
			HardBlockReasons:   nonNilStrings(item.Eligibility.HardBlockReasons),
			WarningReasons:     nonNilStrings(item.Eligibility.WarningReasons),
			SuppressionReasons: nonNilStrings(item.Eligibility.SuppressionReasons),
		},
		OverlappingCampaigns: item.OverlappingCampaigns,
		Capacity:             item.Capacity,
		Service:              toAPIServiceResponse(item.Service),
		CreatedAt:            item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:            item.UpdatedAt.UTC().Format(time.RFC3339),
		Version:              item.Version,
	}
}

func nonNilStrings(items []string) []string {
	if items == nil {
		return []string{}
	}
	return items
}
