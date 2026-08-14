package accountgovernance

import (
	"context"
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

func (s *Service) BusinessCenter(ctx context.Context, actor auth.BusinessActor) (Center, *domain.AppError) {
	now := s.now().UTC()
	if s == nil || s.repo == nil {
		return Center{GeneratedAt: now, AccountStatus: actor.AccountStatus, ProcessingStatus: "completed", Items: []Disposition{}}, nil
	}
	return s.repo.BusinessCenter(ctx, actor.UserID, now)
}
