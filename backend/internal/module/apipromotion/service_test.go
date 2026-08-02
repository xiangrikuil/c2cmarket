package apipromotion

import (
	"context"
	"strings"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
	"c2c-market/backend/internal/module/idempotency"
)

type fakeRepository struct {
	eligibility Eligibility
	created     Promotion
	stopped     Promotion
}

func (f *fakeRepository) ListPublicAPIPromotions(context.Context, string, time.Time) ([]Promotion, *domain.AppError) {
	return nil, nil
}
func (f *fakeRepository) ListAdminAPIPromotions(context.Context, time.Time) ([]Promotion, *domain.AppError) {
	return nil, nil
}
func (f *fakeRepository) GetAPIPromotionEligibility(context.Context, string, time.Time) (Eligibility, *domain.AppError) {
	return f.eligibility, nil
}
func (f *fakeRepository) GetAPIPromotionAvailability(context.Context, AvailabilityInput, time.Time) (Availability, *domain.AppError) {
	return Availability{Eligibility: f.eligibility, Capacity: APIMarketTopCapacity}, nil
}
func (f *fakeRepository) CreateAPIPromotion(context.Context, CreateInput, time.Time) (Promotion, *domain.AppError) {
	return f.created, nil
}
func (f *fakeRepository) CreateAPIPromotionWithIdempotency(_ context.Context, _ idempotency.Entry, _ CreateInput, _ time.Time, build CompletionBuilder) (Promotion, idempotency.Completion, *domain.AppError) {
	completion, appErr := build(f.created)
	return f.created, completion, appErr
}
func (f *fakeRepository) StopAPIPromotion(context.Context, StopInput, time.Time) (Promotion, *domain.AppError) {
	return f.stopped, nil
}
func (f *fakeRepository) StopAPIPromotionWithIdempotency(_ context.Context, _ idempotency.Entry, _ StopInput, _ time.Time, build CompletionBuilder) (Promotion, idempotency.Completion, *domain.AppError) {
	completion, appErr := build(f.stopped)
	return f.stopped, completion, appErr
}

func TestStatusAtBoundaries(t *testing.T) {
	now := time.Date(2026, 8, 2, 4, 0, 0, 0, time.UTC)
	base := Promotion{StartsAt: now, EndsAt: now.Add(time.Hour), Eligibility: Eligibility{Displayable: true}}
	if got := StatusAt(base, now); got != StatusServing {
		t.Fatalf("start boundary status = %q", got)
	}
	if got := StatusAt(base, base.EndsAt); got != StatusFinished {
		t.Fatalf("end boundary status = %q", got)
	}
	base.Eligibility.Displayable = false
	if got := StatusAt(base, now); got != StatusSuppressed {
		t.Fatalf("suppressed status = %q", got)
	}
	stoppedAt := now
	base.StoppedAt = &stoppedAt
	if got := StatusAt(base, now.Add(-time.Hour)); got != StatusStopped {
		t.Fatalf("stopped status = %q", got)
	}
}

func TestCreateRejectsIneligibleService(t *testing.T) {
	now := time.Date(2026, 8, 2, 4, 0, 0, 0, time.UTC)
	service := NewService(&fakeRepository{eligibility: Eligibility{
		Configurable:     false,
		HardBlockReasons: []string{"商户账号当前不可用。"},
	}}, nil, func() time.Time { return now })
	_, appErr := service.Create(context.Background(), auth.User{ID: "admin", IsAdmin: true}, CreateInput{
		APIServiceID: "10000000-0000-4000-8000-000000000001", StartsAt: now, EndsAt: now.Add(7 * 24 * time.Hour), Reason: "首页运营测试",
	})
	if appErr == nil || appErr.Detail != "商户账号当前不可用。" {
		t.Fatalf("unexpected error: %#v", appErr)
	}
}

func TestCreateRequiresAdminAndValidPeriod(t *testing.T) {
	now := time.Date(2026, 8, 2, 4, 0, 0, 0, time.UTC)
	service := NewService(&fakeRepository{eligibility: Eligibility{Configurable: true}}, nil, func() time.Time { return now })
	if _, appErr := service.Create(context.Background(), auth.User{}, CreateInput{}); appErr == nil || appErr.Code != domain.CodePermissionDenied {
		t.Fatalf("expected permission error, got %#v", appErr)
	}
	_, appErr := service.Create(context.Background(), auth.User{ID: "admin", IsAdmin: true}, CreateInput{
		APIServiceID: "10000000-0000-4000-8000-000000000001", StartsAt: now, EndsAt: now, Reason: "test",
	})
	if appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("expected validation error, got %#v", appErr)
	}
}

func TestAvailabilityValidatesServiceIDAndReturnsRepositoryFacts(t *testing.T) {
	now := time.Date(2026, 8, 2, 4, 0, 0, 0, time.UTC)
	repo := &fakeRepository{eligibility: Eligibility{
		Configurable:       true,
		Displayable:        false,
		WarningReasons:     []string{"该服务存在普通未解决纠纷，请人工复核后再配置。"},
		SuppressionReasons: []string{"服务当前暂停接单。"},
	}}
	service := NewService(repo, nil, func() time.Time { return now })
	admin := auth.User{ID: "20000000-0000-4000-8000-000000000001", IsAdmin: true}

	if _, appErr := service.Availability(context.Background(), admin, AvailabilityInput{
		APIServiceID: "not-a-uuid",
		StartsAt:     now,
		EndsAt:       now.Add(time.Hour),
	}); appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("expected invalid UUID validation error, got %#v", appErr)
	}

	result, appErr := service.Availability(context.Background(), admin, AvailabilityInput{
		APIServiceID: "10000000-0000-4000-8000-000000000001",
		StartsAt:     now,
		EndsAt:       now.Add(time.Hour),
	})
	if appErr != nil {
		t.Fatalf("availability failed: %#v", appErr)
	}
	if !result.Eligibility.Configurable || result.Eligibility.Displayable || len(result.Eligibility.WarningReasons) != 1 || len(result.Eligibility.SuppressionReasons) != 1 {
		t.Fatalf("availability lost eligibility facts: %#v", result)
	}
}

func TestPromotionReasonLengthBoundaries(t *testing.T) {
	now := time.Date(2026, 8, 2, 4, 0, 0, 0, time.UTC)
	repo := &fakeRepository{eligibility: Eligibility{Configurable: true}}
	service := NewService(repo, nil, func() time.Time { return now })
	admin := auth.User{ID: "20000000-0000-4000-8000-000000000001", IsAdmin: true}
	validServiceID := "10000000-0000-4000-8000-000000000001"

	if _, appErr := service.Create(context.Background(), admin, CreateInput{
		APIServiceID: validServiceID,
		StartsAt:     now,
		EndsAt:       now.Add(time.Hour),
		Reason:       strings.Repeat("推", 500),
	}); appErr != nil {
		t.Fatalf("500-rune create reason should be accepted: %#v", appErr)
	}
	if _, appErr := service.Create(context.Background(), admin, CreateInput{
		APIServiceID: validServiceID,
		StartsAt:     now,
		EndsAt:       now.Add(time.Hour),
		Reason:       strings.Repeat("推", 501),
	}); appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("expected 501-rune create reason rejection, got %#v", appErr)
	}
	if _, appErr := service.Stop(context.Background(), admin, StopInput{
		PromotionID:     "not-a-uuid",
		ExpectedVersion: 1,
		Reason:          "停止",
	}); appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("expected invalid promotion UUID rejection, got %#v", appErr)
	}
	if _, appErr := service.Stop(context.Background(), admin, StopInput{
		PromotionID:     "30000000-0000-4000-8000-000000000001",
		ExpectedVersion: 1,
		Reason:          strings.Repeat("停", 501),
	}); appErr == nil || appErr.Code != domain.CodeValidationFailed {
		t.Fatalf("expected 501-rune stop reason rejection, got %#v", appErr)
	}
}
