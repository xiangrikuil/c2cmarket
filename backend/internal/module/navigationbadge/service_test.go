package navigationbadge

import (
	"context"
	"net/http"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
)

type fakeRepository struct {
	result      Summary
	appErr      *domain.AppError
	userID      string
	isAdmin     bool
	requestedAt time.Time
}

func (f *fakeRepository) NavigationBadgeSummary(_ context.Context, userID string, isAdmin bool, now time.Time) (Summary, *domain.AppError) {
	f.userID = userID
	f.isAdmin = isAdmin
	f.requestedAt = now
	return f.result, f.appErr
}

func TestGetHidesAdminCountsFromNonAdmin(t *testing.T) {
	now := time.Date(2026, 7, 11, 8, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	repo := &fakeRepository{result: Summary{
		NotificationUnread: 3,
		Admin:              &AdminCounts{OfficialPrices: 9},
	}}
	service := NewService(repo, func() time.Time { return now })

	result, appErr := service.Get(context.Background(), auth.User{ID: "user-1"})
	if appErr != nil {
		t.Fatalf("get navigation badges: %v", appErr)
	}
	if result.Admin != nil {
		t.Fatalf("non-admin response exposed admin counts: %#v", result.Admin)
	}
	if result.NotificationUnread != 3 {
		t.Fatalf("notification unread = %d, want 3", result.NotificationUnread)
	}
	if !result.GeneratedAt.Equal(now.UTC()) || !repo.requestedAt.Equal(now.UTC()) {
		t.Fatalf("generated time mismatch: result=%s repo=%s", result.GeneratedAt, repo.requestedAt)
	}
	if repo.userID != "user-1" || repo.isAdmin {
		t.Fatalf("unexpected repository request: user=%q admin=%v", repo.userID, repo.isAdmin)
	}
}

func TestGetReturnsAdminCountsAndRecomputesTotal(t *testing.T) {
	repo := &fakeRepository{result: Summary{Admin: &AdminCounts{
		Total:           99,
		OfficialPrices:  1,
		Carpools:        2,
		APIServices:     3,
		FeedbackTickets: 4,
		Reports:         5,
	}}}
	service := NewService(repo, time.Now)

	result, appErr := service.Get(context.Background(), auth.User{ID: "admin-1", IsAdmin: true})
	if appErr != nil {
		t.Fatalf("get admin navigation badges: %v", appErr)
	}
	if result.Admin == nil || result.Admin.Total != 15 {
		t.Fatalf("admin counts = %#v, want total 15", result.Admin)
	}
	if !repo.isAdmin {
		t.Fatal("repository did not receive administrator scope")
	}
}

func TestGetRejectsMissingSessionUser(t *testing.T) {
	service := NewService(&fakeRepository{}, time.Now)
	_, appErr := service.Get(context.Background(), auth.User{})
	if appErr == nil || appErr.Code != domain.CodeSessionExpired {
		t.Fatalf("expected session error, got %v", appErr)
	}
}

func TestGetFailsWhenAdminCountsAreMissing(t *testing.T) {
	service := NewService(&fakeRepository{}, time.Now)
	_, appErr := service.Get(context.Background(), auth.User{ID: "admin-1", IsAdmin: true})
	if appErr == nil || appErr.Code != domain.CodeInternalError {
		t.Fatalf("expected internal error, got %v", appErr)
	}
}

func TestGetFailsWhenRepositoryIsUnavailable(t *testing.T) {
	service := NewService(nil, time.Now)
	_, appErr := service.Get(context.Background(), auth.User{ID: "user-1"})
	if appErr == nil || appErr.Status != http.StatusServiceUnavailable {
		t.Fatalf("expected service unavailable error, got %v", appErr)
	}
}
