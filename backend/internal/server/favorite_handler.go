package server

import (
	"encoding/json"
	"net/http"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/favorite"
	"c2c-market/backend/internal/module/idempotency"

	"github.com/go-chi/chi/v5"
)

type favoriteDTO struct {
	ID         string `json:"id"`
	TargetType string `json:"targetType"`
	TargetID   string `json:"targetId"`
	Title      string `json:"title"`
	Subtitle   string `json:"subtitle"`
	Status     string `json:"status"`
	To         string `json:"to"`
	CreatedAt  string `json:"createdAt"`
}

type favoriteStatusDTO struct {
	Favorited bool         `json:"favorited"`
	Favorite  *favoriteDTO `json:"favorite,omitempty"`
}

func (s *Server) handleMyFavorites(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	items, appErr := s.app.MyFavorites(r.Context(), user)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toFavoriteDTOs(items))
}

func (s *Server) handleFavoriteStatus(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	favorited, appErr := s.app.IsFavorite(r.Context(), user, chi.URLParam(r, "targetType"), chi.URLParam(r, "targetId"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, favoriteStatusDTO{Favorited: favorited})
}

func (s *Server) handleCreateFavorite(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	body, _, appErr := decodeStrictJSON[struct{}](r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	targetType := chi.URLParam(r, "targetType")
	targetID := chi.URLParam(r, "targetId")
	routeKey := "PUT /api/v1/me/favorites/{targetType}/{targetId}:" + targetType + ":" + targetID
	completion, appErr := s.app.CreateFavoriteWithIdempotency(r.Context(), user.ID, routeKey, r.Header.Get("Idempotency-Key"), requestHash(r.Method, routeKey, body), targetType, targetID, favoriteCompletionBuilder())
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeIdempotencyCompletion(w, completion)
}

func (s *Server) handleDeleteFavorite(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSessionAndCSRF(r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	if _, appErr := decodeStrictJSONOnly[struct{}](r); appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	result, appErr := s.app.DeleteFavorite(r.Context(), user, chi.URLParam(r, "targetType"), chi.URLParam(r, "targetId"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, toFavoriteStatusDTO(result))
}

func favoriteCompletionBuilder() favorite.CompletionBuilder {
	return func(result favorite.MutationResult) (idempotency.Completion, *domain.AppError) {
		body, err := json.Marshal(toFavoriteStatusDTO(result))
		if err != nil {
			return idempotency.Completion{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "响应序列化失败。")
		}
		resourceID := ""
		if result.Favorite != nil {
			resourceID = result.Favorite.ID
		}
		return idempotency.Completion{
			Status:       http.StatusOK,
			ContentType:  "application/json; charset=utf-8",
			Body:         body,
			ResourceType: "favorite",
			ResourceID:   resourceID,
		}, nil
	}
}

func toFavoriteStatusDTO(result favorite.MutationResult) favoriteStatusDTO {
	var dto *favoriteDTO
	if result.Favorite != nil {
		value := toFavoriteDTO(*result.Favorite)
		dto = &value
	}
	return favoriteStatusDTO{Favorited: result.Favorited, Favorite: dto}
}

func toFavoriteDTOs(items []favorite.ListItem) []favoriteDTO {
	result := make([]favoriteDTO, 0, len(items))
	for _, item := range items {
		result = append(result, toFavoriteDTO(item))
	}
	return result
}

func toFavoriteDTO(item favorite.ListItem) favoriteDTO {
	return favoriteDTO{
		ID:         item.ID,
		TargetType: item.TargetType,
		TargetID:   item.TargetID,
		Title:      item.Title,
		Subtitle:   item.Subtitle,
		Status:     item.Status,
		To:         item.To,
		CreatedAt:  item.CreatedAt.UTC().Format(time.RFC3339),
	}
}
