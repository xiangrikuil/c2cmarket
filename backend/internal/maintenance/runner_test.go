package maintenance

import (
	"context"
	"io"
	"log"
	"sync"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
)

func TestRunnerRunsImmediatelyAndPeriodically(t *testing.T) {
	repo := &fakeRepository{calls: make(chan struct{}, 4)}
	runner := newTestRunner(t, repo, 10*time.Millisecond)
	runner.Start()
	t.Cleanup(runner.Close)

	for call := 0; call < 2; call++ {
		select {
		case <-repo.calls:
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for maintenance call %d", call+1)
		}
	}
}

func TestRunnerContinuesAfterFailure(t *testing.T) {
	repo := &fakeRepository{
		calls:     make(chan struct{}, 4),
		failFirst: true,
	}
	runner := newTestRunner(t, repo, 10*time.Millisecond)
	runner.Start()
	t.Cleanup(runner.Close)

	for call := 0; call < 2; call++ {
		select {
		case <-repo.calls:
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for maintenance retry %d", call+1)
		}
	}
}

func TestRunnerCloseCancelsActiveRun(t *testing.T) {
	repo := &fakeRepository{
		calls: make(chan struct{}, 1),
		block: true,
	}
	runner := newTestRunner(t, repo, time.Hour)
	runner.Start()
	select {
	case <-repo.calls:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for active maintenance run")
	}

	closed := make(chan struct{})
	go func() {
		runner.Close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("runner close did not cancel the active run")
	}
}

func TestRunnerRejectsInvalidPolicy(t *testing.T) {
	_, err := NewRunner(&fakeRepository{}, Config{
		Interval:  time.Minute,
		BatchSize: 1,
		Policy: Policy{
			SessionRetention:            time.Hour,
			EmailVerificationRetention:  time.Hour,
			ReadNotificationRetention:   2 * time.Hour,
			UnreadNotificationRetention: time.Hour,
			DomainEventRetention:        time.Hour,
		},
	}, time.Now, log.New(io.Discard, "", 0))
	if err == nil {
		t.Fatal("expected invalid notification retention policy to fail")
	}
}

func TestRunnerStatsTrackRunOutcomes(t *testing.T) {
	repo := &fakeRepository{failFirst: true}
	runner := newTestRunner(t, repo, time.Hour)

	runner.execute(context.Background())
	runner.execute(context.Background())
	repo.mu.Lock()
	repo.skip = true
	repo.mu.Unlock()
	runner.execute(context.Background())

	stats := runner.Stats()
	if stats.FailureTotal != 1 || stats.SuccessTotal != 1 || stats.SkippedTotal != 1 {
		t.Fatalf("unexpected maintenance stats: %+v", stats)
	}
	if stats.LastDurationSeconds < 0 {
		t.Fatalf("invalid maintenance duration: %+v", stats)
	}
}

func newTestRunner(t *testing.T, repo Repository, interval time.Duration) *Runner {
	t.Helper()
	runner, err := NewRunner(repo, Config{
		Interval:  interval,
		BatchSize: 10,
		Policy: Policy{
			SessionRetention:            time.Hour,
			EmailVerificationRetention:  time.Hour,
			ReadNotificationRetention:   time.Hour,
			UnreadNotificationRetention: 2 * time.Hour,
			DomainEventRetention:        time.Hour,
		},
	}, time.Now, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("new runner: %v", err)
	}
	return runner
}

type fakeRepository struct {
	mu        sync.Mutex
	calls     chan struct{}
	failFirst bool
	block     bool
	skip      bool
	count     int
}

func (f *fakeRepository) RunDataLifecycle(ctx context.Context, _ time.Time, _ int, _ Policy) (Result, *domain.AppError) {
	f.mu.Lock()
	f.count++
	count := f.count
	skip := f.skip
	f.mu.Unlock()
	if f.calls != nil {
		select {
		case f.calls <- struct{}{}:
		default:
		}
	}
	if f.block {
		<-ctx.Done()
		return Result{}, domain.NewError(500, domain.CodeInternalError, "Internal error", "维护任务已取消。")
	}
	if f.failFirst && count == 1 {
		return Result{}, domain.NewError(500, domain.CodeInternalError, "Internal error", "维护任务失败。")
	}
	if skip {
		return Result{LockAcquired: false}, nil
	}
	return Result{LockAcquired: true}, nil
}
