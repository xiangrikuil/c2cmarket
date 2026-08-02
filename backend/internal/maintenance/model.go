package maintenance

import (
	"context"
	"time"

	"c2c-market/backend/internal/domain"
)

type Policy struct {
	SessionRetention            time.Duration
	EmailVerificationRetention  time.Duration
	ReadNotificationRetention   time.Duration
	UnreadNotificationRetention time.Duration
	DomainEventRetention        time.Duration
}

type Result struct {
	LockAcquired              bool
	SessionsDeleted           int64
	VerificationCodesDeleted  int64
	IdempotencyEntriesDeleted int64
	ContactSessionsExpired    int64
	APIOrdersPaymentExpired   int64
	APIOrderReviewReminders   int64
	APIOrdersAutoCompleted    int64
	NotificationsDeleted      int64
	DomainEventsDeleted       int64
}

type Repository interface {
	RunDataLifecycle(ctx context.Context, now time.Time, batchSize int, policy Policy) (Result, *domain.AppError)
}
