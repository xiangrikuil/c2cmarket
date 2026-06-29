package server

import (
	"encoding/json"
	"net/http"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/review"

	"github.com/go-chi/chi/v5"
)

type reviewCenterRowDTO struct {
	ID                   string   `json:"id"`
	SourceType           string   `json:"sourceType"`
	SourceID             string   `json:"sourceId"`
	Target               string   `json:"target"`
	CounterpartyUsername string   `json:"counterpartyUsername"`
	CounterpartyName     string   `json:"counterpartyName"`
	Status               string   `json:"status"`
	Rating               int      `json:"rating"`
	Tags                 []string `json:"tags"`
	Note                 string   `json:"note"`
	CreatedAt            string   `json:"createdAt"`
	UpdatedAt            string   `json:"updatedAt"`
}

type submitReviewRequest struct {
	Rating int      `json:"rating"`
	Tags   []string `json:"tags"`
	Note   string   `json:"note"`
}

type publicReviewDTO struct {
	ID          string   `json:"id"`
	Username    string   `json:"username"`
	Date        string   `json:"date"`
	ServiceType string   `json:"serviceType"`
	Rating      int      `json:"rating"`
	Tags        []string `json:"tags"`
	Note        string   `json:"note"`
	Verified    bool     `json:"verified"`
}

func (s *Server) handleMyReviewCenter(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	rows, appErr := s.app.ListMyReviewCenterRows(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[reviewCenterRowDTO]{Items: toReviewCenterRowDTOs(rows)})
}

func (s *Server) handleSubmitCarpoolMembershipReview(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[submitReviewRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	membershipID := chi.URLParam(r, "membershipId")
	routeKey := "PUT /api/v1/me/reviews/carpool-memberships/{membershipId}:" + membershipID
	completion, appErr := s.app.SubmitCarpoolMembershipReviewWithIdempotency(
		r.Context(),
		user.ID,
		routeKey,
		r.Header.Get("Idempotency-Key"),
		requestHash(r.Method, routeKey, body),
		review.SubmitReviewInput{
			SourceType: review.SourceCarpoolMembership,
			SourceID:   membershipID,
			Rating:     req.Rating,
			Tags:       req.Tags,
			Note:       req.Note,
		},
		func(result review.MutationResult) (idempotency.Completion, *domain.AppError) {
			responseBody, marshalErr := json.Marshal(toReviewCenterRowDTO(result.Row))
			if marshalErr != nil {
				return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
			}
			return idempotency.Completion{
				Status:       http.StatusOK,
				ContentType:  "application/json; charset=utf-8",
				Body:         responseBody,
				ResourceType: "review",
				ResourceID:   result.Row.ID,
			}, nil
		},
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handlePublicUserReviews(w http.ResponseWriter, r *http.Request) {
	items, appErr := s.app.PublicUserReviews(r.Context(), chi.URLParam(r, "username"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, listResponse[publicReviewDTO]{Items: toPublicReviewDTOs(items)})
}

func toReviewCenterRowDTOs(rows []review.ReviewCenterRow) []reviewCenterRowDTO {
	items := make([]reviewCenterRowDTO, 0, len(rows))
	for _, row := range rows {
		items = append(items, toReviewCenterRowDTO(row))
	}
	return items
}

func toReviewCenterRowDTO(row review.ReviewCenterRow) reviewCenterRowDTO {
	return reviewCenterRowDTO{
		ID:                   row.ID,
		SourceType:           row.SourceType,
		SourceID:             row.SourceID,
		Target:               row.Target,
		CounterpartyUsername: row.CounterpartyUsername,
		CounterpartyName:     row.CounterpartyName,
		Status:               row.Status,
		Rating:               row.Rating,
		Tags:                 append([]string{}, row.Tags...),
		Note:                 row.Note,
		CreatedAt:            row.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:            row.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toPublicReviewDTOs(items []review.PublicReview) []publicReviewDTO {
	result := make([]publicReviewDTO, 0, len(items))
	for _, item := range items {
		result = append(result, toPublicReviewDTO(item))
	}
	return result
}

func toPublicReviewDTO(item review.PublicReview) publicReviewDTO {
	return publicReviewDTO{
		ID:          item.ID,
		Username:    item.Username,
		Date:        item.Date.UTC().Format("2006-01-02"),
		ServiceType: item.ServiceType,
		Rating:      item.Rating,
		Tags:        append([]string{}, item.Tags...),
		Note:        item.Note,
		Verified:    item.Verified,
	}
}
