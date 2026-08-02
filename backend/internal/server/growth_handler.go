package server

import (
	"net/http"
	"strconv"
	"strings"

	"c2c-market/backend/internal/domain"
)

func (s *Server) handleAdminGrowthOverview(w http.ResponseWriter, r *http.Request) {
	user, _, appErr := s.requireSession(w, r)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}

	days := 0
	if value := strings.TrimSpace(r.URL.Query().Get("days")); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			writeProblem(w, r, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid growth window", "统计周期仅支持 7、30 或 90 天。", "days", "invalid", "统计周期仅支持 7、30 或 90 天。"))
			return
		}
		days = parsed
	}

	overview, appErr := s.growth.AdminGrowthOverview(r.Context(), user, days)
	if appErr != nil {
		writeProblem(w, r, appErr)
		return
	}
	writeJSON(w, http.StatusOK, overview)
}
