package evidence

import (
	"context"
	"log"
	"sync"
	"time"

	"c2c-market/backend/internal/domain"
)

type CleanupRunner struct {
	service   *Service
	interval  time.Duration
	batchSize int
	logger    interface{ Printf(string, ...any) }
	mu        sync.Mutex
	cancel    context.CancelFunc
	wait      sync.WaitGroup
	started   bool
	closed    bool
}

func NewCleanupRunner(service *Service, interval time.Duration, batchSize int) *CleanupRunner {
	if service == nil || !service.Enabled() || interval <= 0 || batchSize <= 0 {
		return nil
	}
	return &CleanupRunner{service: service, interval: interval, batchSize: batchSize, logger: log.Default()}
}

func (r *CleanupRunner) Start() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.started || r.closed {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.started = true
	r.wait.Add(1)
	go r.loop(ctx)
}

func (r *CleanupRunner) Close() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.closed = true
	cancel := r.cancel
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	r.wait.Wait()
}

func (r *CleanupRunner) RunOnce(ctx context.Context) (CleanupResult, *domain.AppError) {
	if r == nil {
		return CleanupResult{}, nil
	}
	return r.service.Cleanup(ctx, r.batchSize)
}

func (r *CleanupRunner) loop(ctx context.Context) {
	defer r.wait.Done()
	r.execute(ctx)
	ticker := time.NewTicker(r.interval)
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

func (r *CleanupRunner) execute(ctx context.Context) {
	result, appErr := r.RunOnce(ctx)
	if appErr != nil {
		r.logger.Printf("图片证据清理失败 code=%s", appErr.Code)
		return
	}
	if result.Claimed > 0 {
		r.logger.Printf("图片证据清理完成 claimed=%d destroyed=%d failed=%d", result.Claimed, result.Destroyed, result.Failed)
	}
}
