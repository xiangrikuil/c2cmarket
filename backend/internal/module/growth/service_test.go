package growth

import (
	"context"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
)

type fakeRepository struct {
	overview     Overview
	activityDate string
	activityUser string
}

func (f *fakeRepository) RecordActivity(_ context.Context, userID, activityDate string, _ time.Time) *domain.AppError {
	f.activityUser = userID
	f.activityDate = activityDate
	return nil
}

func (f *fakeRepository) GrowthOverview(_ context.Context, asOf time.Time, windowDays int) (Overview, *domain.AppError) {
	result := f.overview
	result.GeneratedAt = asOf
	result.WindowDays = windowDays
	return result, nil
}

func TestAdminOverviewRequiresAdminAndSupportedWindow(t *testing.T) {
	service := NewService(&fakeRepository{}, func() time.Time { return time.Date(2026, 8, 2, 4, 0, 0, 0, time.UTC) })
	if _, appErr := service.AdminOverview(context.Background(), auth.User{}, 30); appErr == nil || appErr.Code != domain.CodePermissionDenied {
		t.Fatalf("expected permission error, got %#v", appErr)
	}
	if _, appErr := service.AdminOverview(context.Background(), auth.User{IsAdmin: true}, 14); appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("expected window validation error, got %#v", appErr)
	}
	result, appErr := service.AdminOverview(context.Background(), auth.User{IsAdmin: true}, 0)
	if appErr != nil || result.WindowDays != DefaultWindowDays {
		t.Fatalf("default overview = %+v, err=%v", result, appErr)
	}
}

func TestRecordActivityUsesShanghaiBusinessDate(t *testing.T) {
	repo := &fakeRepository{}
	service := NewService(repo, func() time.Time { return time.Date(2026, 8, 1, 16, 30, 0, 0, time.UTC) })
	if appErr := service.RecordActivity(context.Background(), "user-1"); appErr != nil {
		t.Fatalf("record activity: %v", appErr)
	}
	if repo.activityUser != "user-1" || repo.activityDate != "2026-08-02" {
		t.Fatalf("unexpected activity user=%q date=%q", repo.activityUser, repo.activityDate)
	}
}

func TestNilRepositoryReturnsZeroFilledWindow(t *testing.T) {
	now := time.Date(2026, 8, 2, 4, 0, 0, 0, time.UTC)
	service := NewService(nil, func() time.Time { return now })
	result, appErr := service.AdminOverview(context.Background(), auth.User{IsAdmin: true}, 7)
	if appErr != nil {
		t.Fatalf("empty overview: %v", appErr)
	}
	if len(result.RegistrationTrend) != 7 || len(result.ActivityTrend) != 7 || len(result.RetentionCohorts) != 7 {
		t.Fatalf("empty series not zero-filled: %+v", result)
	}
	if result.RegistrationTrend[0].Date != "2026-07-27" || result.RegistrationTrend[6].Date != "2026-08-02" {
		t.Fatalf("unexpected date range: %+v", result.RegistrationTrend)
	}
}
