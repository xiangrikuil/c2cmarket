package maintenance

import (
	"context"
	"fmt"
	"log"
	"sync"
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
		config.Policy.DomainEventRetention <= 0 {
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
	if appErr != nil {
		r.logger.Printf("数据维护任务失败 duration=%s error=%s", r.now().Sub(startedAt), appErr.Code)
		return
	}
	if !result.LockAcquired {
		return
	}
	r.logger.Printf(
		"数据维护任务完成 duration=%s sessions=%d verification_codes=%d idempotency=%d contact_sessions=%d notifications=%d domain_events=%d",
		r.now().Sub(startedAt),
		result.SessionsDeleted,
		result.VerificationCodesDeleted,
		result.IdempotencyEntriesDeleted,
		result.ContactSessionsExpired,
		result.NotificationsDeleted,
		result.DomainEventsDeleted,
	)
}
