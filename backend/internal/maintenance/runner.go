package maintenance

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"c2c-market/backend/internal/domain"
)

type Config struct {
	Interval  time.Duration
	BatchSize int
	Policy    Policy
}

type Logger interface {
	Printf(format string, values ...any)
}

type Runner struct {
	repo      Repository
	config    Config
	now       func() time.Time
	logger    Logger
	startOnce sync.Once
	closeOnce sync.Once
	cancel    context.CancelFunc
	wait      sync.WaitGroup
	stats     runnerStats
}

type Stats struct {
	SuccessTotal        uint64
	FailureTotal        uint64
	SkippedTotal        uint64
	LastDurationSeconds float64
}

type runnerStats struct {
	successTotal atomic.Uint64
	failureTotal atomic.Uint64
	skippedTotal atomic.Uint64
	lastDuration atomic.Int64
}

func NewRunner(repo Repository, config Config, now func() time.Time, logger Logger) (*Runner, error) {
	if repo == nil {
		return nil, fmt.Errorf("maintenance repository is required")
	}
	if config.Interval <= 0 {
		return nil, fmt.Errorf("maintenance interval must be positive")
	}
	if config.BatchSize <= 0 {
		return nil, fmt.Errorf("maintenance batch size must be positive")
	}
	if config.Policy.SessionRetention <= 0 ||
		config.Policy.EmailVerificationRetention <= 0 ||
		config.Policy.ReadNotificationRetention <= 0 ||
		config.Policy.UnreadNotificationRetention <= 0 ||
		config.Policy.DomainEventRetention <= 0 ||
		config.Policy.APIDeliveryCredentialRetention <= 0 {
		return nil, fmt.Errorf("maintenance retention values must be positive")
	}
	if config.Policy.UnreadNotificationRetention < config.Policy.ReadNotificationRetention {
		return nil, fmt.Errorf("unread notification retention must not be shorter than read retention")
	}
	if now == nil {
		now = time.Now
	}
	if logger == nil {
		logger = log.Default()
	}
	return &Runner{
		repo:   repo,
		config: config,
		now:    now,
		logger: logger,
	}, nil
}

func (r *Runner) Start() {
	if r == nil {
		return
	}
	r.startOnce.Do(func() {
		runCtx, cancel := context.WithCancel(context.Background())
		r.cancel = cancel
		r.wait.Add(1)
		go r.loop(runCtx)
	})
}

func (r *Runner) Close() {
	if r == nil {
		return
	}
	r.closeOnce.Do(func() {
		if r.cancel != nil {
			r.cancel()
		}
		r.wait.Wait()
	})
}

func (r *Runner) RunOnce(ctx context.Context) (Result, *domain.AppError) {
	if r == nil {
		return Result{}, nil
	}
	runTimeout := r.config.Interval
	if runTimeout > 5*time.Minute {
		runTimeout = 5 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, runTimeout)
	defer cancel()
	return r.repo.RunDataLifecycle(runCtx, r.now(), r.config.BatchSize, r.config.Policy)
}

func (r *Runner) loop(ctx context.Context) {
	defer r.wait.Done()
	r.execute(ctx)

	ticker := time.NewTicker(r.config.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.execute(ctx)
		}
	}
}

func (r *Runner) execute(ctx context.Context) {
	startedAt := r.now()
	result, appErr := r.RunOnce(ctx)
	duration := r.now().Sub(startedAt)
	r.stats.lastDuration.Store(duration.Nanoseconds())
	if appErr != nil {
		r.stats.failureTotal.Add(1)
		r.logger.Printf("数据维护任务失败 duration=%s error=%s", duration, appErr.Code)
		return
	}
	if !result.LockAcquired {
		r.stats.skippedTotal.Add(1)
		return
	}
	r.stats.successTotal.Add(1)
	r.logger.Printf(
		"数据维护任务完成 duration=%s sessions=%d account_appeal_sessions=%d verification_codes=%d idempotency=%d contact_sessions=%d api_order_payment_expired=%d api_order_review_reminders=%d api_orders_auto_completed=%d api_order_credentials_destroyed=%d api_quota_credentials_destroyed=%d notifications=%d domain_events=%d",
		duration,
		result.SessionsDeleted,
		result.AccountAppealSessionsDeleted,
		result.VerificationCodesDeleted,
		result.IdempotencyEntriesDeleted,
		result.ContactSessionsExpired,
		result.APIOrdersPaymentExpired,
		result.APIOrderReviewReminders,
		result.APIOrdersAutoCompleted,
		result.APIOrderCredentialsDestroyed,
		result.APIQuotaCredentialsDestroyed,
		result.NotificationsDeleted,
		result.DomainEventsDeleted,
	)
}

func (r *Runner) Stats() Stats {
	if r == nil {
		return Stats{}
	}
	return Stats{
		SuccessTotal:        r.stats.successTotal.Load(),
		FailureTotal:        r.stats.failureTotal.Load(),
		SkippedTotal:        r.stats.skippedTotal.Load(),
		LastDurationSeconds: time.Duration(r.stats.lastDuration.Load()).Seconds(),
	}
}
