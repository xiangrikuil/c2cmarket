package navigationbadge

import (
	"context"
	"net/http"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
)

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, now: now}
}

func (s *Service) Get(ctx context.Context, user auth.User) (Summary, *domain.AppError) {
	if strings.TrimSpace(user.ID) == "" {
		return Summary{}, domain.NewError(http.StatusUnauthorized, domain.CodeSessionExpired, "Session required", "请先登录。")
	}

	now := s.now().UTC()
	if s.repo == nil {
		return Summary{}, domain.NewError(http.StatusServiceUnavailable, domain.CodeInternalError, "Navigation badge repository unavailable", "导航徽标统计暂时不可用。")
	}

	result, appErr := s.repo.NavigationBadgeSummary(ctx, user.ID, user.IsAdmin, now)
	if appErr != nil {
		return Summary{}, appErr
	}
	result.GeneratedAt = now
	if !user.IsAdmin {
		result.Admin = nil
		return result, nil
	}
	if result.Admin == nil {
		return Summary{}, domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Navigation badge summary incomplete", "导航徽标统计暂时不可用。")
	}
	result.Admin.Total = result.Admin.ActionableTotal()
	return result, nil
}
