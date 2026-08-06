package growth

import (
	"context"
	"net/http"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
)

type Service struct {
	repo Repository
	now  func() time.Time
	zone *time.Location
}

func NewService(repo Repository, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	zone, err := time.LoadLocation(BusinessTimezone)
	if err != nil {
		zone = time.FixedZone(BusinessTimezone, 8*60*60)
	}
	return &Service{repo: repo, now: now, zone: zone}
}

func (s *Service) AdminOverview(ctx context.Context, user auth.User, windowDays int) (Overview, *domain.AppError) {
	if !user.IsAdmin {
		return Overview{}, domain.NewError(http.StatusForbidden, domain.CodePermissionDenied, "Permission denied", "需要管理员权限。")
	}
	if windowDays == 0 {
		windowDays = DefaultWindowDays
	}
	if !isSupportedWindow(windowDays) {
		return Overview{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Invalid growth window", "统计周期仅支持 7、30 或 90 天。", "days", "invalid", "统计周期仅支持 7、30 或 90 天。")
	}
	asOf := s.now().UTC()
	if s == nil || s.repo == nil {
		return emptyOverview(asOf, windowDays, s.zone), nil
	}
	return s.repo.GrowthOverview(ctx, asOf, windowDays)
}

func (s *Service) RecordActivity(ctx context.Context, userID string) *domain.AppError {
	if s == nil || s.repo == nil || userID == "" {
		return nil
	}
	seenAt := s.now().UTC()
	activityDate := seenAt.In(s.zone).Format(time.DateOnly)
	return s.repo.RecordActivity(ctx, userID, activityDate, seenAt)
}

func isSupportedWindow(value int) bool {
	for _, candidate := range SupportedWindowDays {
		if value == candidate {
			return true
		}
	}
	return false
}

func emptyOverview(asOf time.Time, windowDays int, zone *time.Location) Overview {
	if zone == nil {
		zone = time.FixedZone(BusinessTimezone, 8*60*60)
	}
	endDate := asOf.In(zone)
	registration := make([]RegistrationTrendPoint, 0, windowDays)
	activity := make([]ActivityTrendPoint, 0, windowDays)
	retention := make([]RetentionCohort, 0, windowDays)
	for offset := windowDays - 1; offset >= 0; offset-- {
		date := endDate.AddDate(0, 0, -offset).Format(time.DateOnly)
		registration = append(registration, RegistrationTrendPoint{Date: date})
		activity = append(activity, ActivityTrendPoint{Date: date})
		retention = append(retention, RetentionCohort{CohortDate: date})
	}
	return Overview{
		GeneratedAt:       asOf,
		Timezone:          BusinessTimezone,
		WindowDays:        windowDays,
		RegistrationTrend: registration,
		ActivityTrend:     activity,
		Attribution:       []AttributionGroup{},
		RetentionCohorts:  retention,
	}
}
