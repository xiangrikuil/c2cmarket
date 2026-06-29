package server

import (
	"net/http"

	"c2c-market/backend/internal/module/search"
)

type searchResultDTO struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Badge    string `json:"badge"`
	To       string `json:"to"`
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	items, appErr := s.app.SearchMarket(r.Context(), r.URL.Query().Get("q"))
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writePaginatedJSON(w, r, toSearchResultDTOs(items))
}

func toSearchResultDTOs(items []search.Result) []searchResultDTO {
	result := make([]searchResultDTO, 0, len(items))
	for _, item := range items {
		result = append(result, searchResultDTO{
			ID:       item.ID,
			Type:     item.Type,
			Title:    item.Title,
			Subtitle: item.Subtitle,
			Badge:    item.Badge,
			To:       item.To,
		})
	}
	return result
}
