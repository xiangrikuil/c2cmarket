package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/idempotency"
	"c2c-market/backend/internal/module/review"

	"github.com/go-chi/chi/v5"
)

type reviewCenterRowDTO struct {
	ID                   string         `json:"id"`
	TransactionType      string         `json:"transactionType"`
	TransactionID        string         `json:"transactionId"`
	SourceType           string         `json:"sourceType"`
	SourceID             string         `json:"sourceId"`
	Direction            string         `json:"direction"`
	Target               string         `json:"target"`
	CounterpartyUsername string         `json:"counterpartyUsername"`
	CounterpartyName     string         `json:"counterpartyName"`
	ReviewerRole         string         `json:"reviewerRole"`
	RevieweeRole         string         `json:"revieweeRole"`
	Status               string         `json:"status"`
	Visibility           string         `json:"visibility"`
	AllowedTags          []reviewTagDTO `json:"allowedTags"`
	CanCreate            bool           `json:"canCreate"`
	CanEdit              bool           `json:"canEdit"`
	Rating               *int           `json:"rating"`
	Tags                 []string       `json:"tags"`
	Note                 *string        `json:"note"`
	CompletedAt          string         `json:"completedAt"`
	ReviewDeadlineAt     string         `json:"reviewDeadlineAt"`
	CommercialOutcome    string         `json:"commercialOutcome"`
	ReviewPaused         bool           `json:"reviewPaused"`
	SubmittedAt          *string        `json:"submittedAt"`
	VisibleAt            *string        `json:"visibleAt"`
	FrozenAt             *string        `json:"frozenAt"`
	CreatedAt            string         `json:"createdAt"`
	UpdatedAt            string         `json:"updatedAt"`
	Version              int64          `json:"version"`
}

type reviewCenterResponse struct {
	Items      []reviewCenterRowDTO `json:"items"`
	PresetTags []reviewTagDTO       `json:"presetTags"`
	NextCursor *string              `json:"nextCursor,omitempty"`
}

type reviewTagDTO struct {
	Code     string `json:"code"`
	Label    string `json:"label"`
	Polarity string `json:"polarity"`
}

type submitReviewRequest struct {
	Rating int      `json:"rating"`
	Tags   []string `json:"tags"`
	Note   string   `json:"note"`
}

type removeReviewRequest struct {
	Reason string `json:"reason"`
}

type publicReviewDTO struct {
	ID                string   `json:"id"`
	Username          string   `json:"username"`
	Date              string   `json:"date"`
	ServiceType       string   `json:"serviceType"`
	TransactionType   string   `json:"transactionType"`
	ReviewerRole      string   `json:"reviewerRole"`
	RevieweeRole      string   `json:"revieweeRole"`
	Rating            int      `json:"rating"`
	Tags              []string `json:"tags"`
	Note              string   `json:"note"`
	Verified          bool     `json:"verified"`
	CommercialOutcome string   `json:"commercialOutcome"`
}

func (s *Server) handleMyReviewCenter(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	rows, appErr := s.app.ListMyReviewCenterRows(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	page, appErr := paginateSlice(r, filterReviewCenterRows(r, rows))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, reviewCenterResponse{
		Items:      toReviewCenterRowDTOs(page.Items),
		PresetTags: toReviewTagDTOs(review.AllTags()),
		NextCursor: page.NextCursor,
	})
}

func (s *Server) handleCreateTransactionReview(w http.ResponseWriter, r *http.Request) {
	s.handleSaveTransactionReview(w, r, review.OperationCreate, http.StatusCreated)
}

func (s *Server) handleEditTransactionReview(w http.ResponseWriter, r *http.Request) {
	s.handleSaveTransactionReview(w, r, review.OperationEdit, http.StatusOK)
}

func (s *Server) handleSaveTransactionReview(w http.ResponseWriter, r *http.Request, operation string, responseStatus int) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[submitReviewRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	transactionType := chi.URLParam(r, "type")
	transactionID := chi.URLParam(r, "id")
	routeKey := r.Method + " /api/v1/me/transactions/{type}/{id}/review:" + transactionType + ":" + transactionID
	completion, appErr := s.app.SubmitTransactionReviewWithIdempotency(
		r.Context(),
		user.ID,
		routeKey,
		r.Header.Get("Idempotency-Key"),
		requestHash(r.Method, routeKey, body),
		review.SubmitReviewInput{
			TransactionType: transactionType,
			TransactionID:   transactionID,
			Operation:       operation,
			Rating:          req.Rating,
			Tags:            req.Tags,
			Note:            req.Note,
		},
		reviewMutationCompletionBuilder(responseStatus),
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleSubmitCarpoolMembershipReview(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
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
			TransactionType: review.TransactionCarpoolMembership,
			TransactionID:   membershipID,
			Operation:       review.OperationLegacyUpsert,
			Rating:          req.Rating,
			Tags:            req.Tags,
			Note:            req.Note,
		},
		reviewMutationCompletionBuilder(http.StatusOK),
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleRemoveTransactionReview(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, req, appErr := decodeStrictJSON[removeReviewRequest](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	version, appErr := requireIfMatchVersion(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	reviewID := chi.URLParam(r, "id")
	routeKey := "POST /api/v1/admin/reviews/{id}/remove:" + reviewID
	completion, appErr := s.app.AdminRemoveTransactionReviewWithIdempotency(
		r.Context(),
		user,
		routeKey,
		r.Header.Get("Idempotency-Key"),
		requestHash(r.Method, routeKey, body),
		review.RemoveReviewInput{
			ReviewID:        reviewID,
			ExpectedVersion: version,
			Reason:          req.Reason,
		},
		reviewMutationCompletionBuilder(http.StatusOK),
	)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	w.Header().Set("ETag", `"`+strconv.FormatInt(version+1, 10)+`"`)
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

func reviewMutationCompletionBuilder(status int) review.CompletionBuilder {
	return func(result review.MutationResult) (idempotency.Completion, *domain.AppError) {
		responseBody, err := json.Marshal(toReviewCenterRowDTO(result.Row))
		if err != nil {
			return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应编码失败。")
		}
		return idempotency.Completion{
			Status:       status,
			ContentType:  "application/json; charset=utf-8",
			Body:         responseBody,
			ResourceType: "review",
			ResourceID:   result.Row.ID,
		}, nil
	}
}

func toReviewCenterRowDTOs(rows []review.ReviewCenterRow) []reviewCenterRowDTO {
	items := make([]reviewCenterRowDTO, 0, len(rows))
	for _, row := range rows {
		items = append(items, toReviewCenterRowDTO(row))
	}
	return items
}

func toReviewCenterRowDTO(row review.ReviewCenterRow) reviewCenterRowDTO {
	item := reviewCenterRowDTO{
		ID:                   row.ID,
		TransactionType:      row.TransactionType,
		TransactionID:        row.TransactionID,
		SourceType:           row.TransactionType,
		SourceID:             row.TransactionID,
		Direction:            row.Direction,
		Target:               row.Target,
		CounterpartyUsername: row.CounterpartyUsername,
		CounterpartyName:     row.CounterpartyName,
		ReviewerRole:         row.ReviewerRole,
		RevieweeRole:         row.RevieweeRole,
		Status:               row.Status,
		Visibility:           row.Visibility,
		AllowedTags:          toReviewTagDTOs(review.AllowedTags(row.TransactionType, row.ReviewerRole, row.RevieweeRole)),
		CanCreate:            row.CanCreate,
		CanEdit:              row.CanEdit,
		Tags:                 []string{},
		CompletedAt:          formatReviewTime(row.CompletedAt),
		ReviewDeadlineAt:     formatReviewTime(row.ReviewDeadlineAt),
		CommercialOutcome:    row.CommercialOutcome,
		ReviewPaused:         row.ReviewPaused,
		SubmittedAt:          formatOptionalReviewTime(row.SubmittedAt),
		VisibleAt:            formatOptionalReviewTime(row.VisibleAt),
		FrozenAt:             formatOptionalReviewTime(row.FrozenAt),
		CreatedAt:            formatReviewTime(row.CreatedAt),
		UpdatedAt:            formatReviewTime(row.UpdatedAt),
		Version:              row.Version,
	}
	if row.ContentVisible {
		rating := row.Rating
		note := row.Note
		item.Rating = &rating
		item.Tags = append([]string{}, row.Tags...)
		item.Note = &note
	}
	return item
}

func toPublicReviewDTOs(items []review.PublicReview) []publicReviewDTO {
	result := make([]publicReviewDTO, 0, len(items))
	for _, item := range items {
		result = append(result, publicReviewDTO{
			ID:                item.ID,
			Username:          item.ReviewerUsername,
			Date:              item.Date.UTC().Format("2006-01-02"),
			ServiceType:       item.ServiceType,
			TransactionType:   item.TransactionType,
			ReviewerRole:      item.ReviewerRole,
			RevieweeRole:      item.RevieweeRole,
			Rating:            item.Rating,
			Tags:              review.DisplayTagLabels(item.Tags),
			Note:              item.Note,
			Verified:          item.Verified,
			CommercialOutcome: item.CommercialOutcome,
		})
	}
	return result
}

func toReviewTagDTOs(items []review.TagDefinition) []reviewTagDTO {
	result := make([]reviewTagDTO, 0, len(items))
	for _, item := range items {
		result = append(result, reviewTagDTO{Code: item.Code, Label: item.Label, Polarity: item.Polarity})
	}
	return result
}

func formatReviewTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func formatOptionalReviewTime(value *time.Time) *string {
	if value == nil || value.IsZero() {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}
