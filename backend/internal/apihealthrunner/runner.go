package apihealthrunner

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apihealth"
)

type Options struct {
	Enabled      bool
	ScanInterval time.Duration
	Timeout      time.Duration
	Concurrency  int
	BatchSize    int
}

type Repository interface {
	ClaimDueProbes(ctx context.Context, slotStartedAt, now time.Time, limit int, runningTimeout time.Duration) ([]apihealth.ProbeJob, *domain.AppError)
	FinalizeProbe(ctx context.Context, sampleID string, result apihealth.ProbeResult, finishedAt time.Time) (bool, *domain.AppError)
}

type RunResult struct {
	Claimed   int
	Succeeded int
	Failed    int
	Finalized int
}

type Stats struct {
	Enabled             bool
	RunSuccessTotal     uint64
	RunFailureTotal     uint64
	ProbeSuccessTotal   uint64
	ProbeFailureTotal   uint64
	Inflight            int64
	LastDurationSeconds float64
	LastSuccessAt       time.Time
	LastClaimed         int
	LastFinalized       int
}

type runnerStats struct {
	runSuccess    atomic.Uint64
	runFailure    atomic.Uint64
	probeSuccess  atomic.Uint64
	probeFailure  atomic.Uint64
	inflight      atomic.Int64
	lastDuration  atomic.Int64
	lastSuccess   atomic.Int64
	lastClaimed   atomic.Int64
	lastFinalized atomic.Int64
}

type Runner struct {
	repository Repository
	prober     apihealth.Prober
	options    Options
	now        func() time.Time
	logger     *log.Logger
	cancel     context.CancelFunc
	done       chan struct{}
	startOnce  sync.Once
	closeOnce  sync.Once
	stats      runnerStats
}

func New(repository Repository, prober apihealth.Prober, options Options, now func() time.Time, logger *log.Logger) *Runner {
	if options.ScanInterval <= 0 {
		options.ScanInterval = time.Minute
	}
	if options.Timeout <= 0 {
		options.Timeout = 10 * time.Second
	}
	if options.Concurrency < 1 {
		options.Concurrency = 4
	}
	if options.BatchSize < 1 {
		options.BatchSize = 50
	}
	if now == nil {
		now = time.Now
	}
	if logger == nil {
		logger = log.Default()
	}
	return &Runner{repository: repository, prober: prober, options: options, now: now, logger: logger, done: make(chan struct{})}
}

func (r *Runner) Start(parent context.Context) {
	if r == nil {
		return
	}
	if !r.options.Enabled {
		r.logger.Printf("API 真实模型探针任务未启动 enabled=false")
		return
	}
	r.startOnce.Do(func() {
		r.logger.Printf("API 真实模型探针任务启动 enabled=true scan_interval=%s concurrency=%d batch_size=%d", r.options.ScanInterval, r.options.Concurrency, r.options.BatchSize)
		ctx, cancel := context.WithCancel(parent)
		r.cancel = cancel
		go r.loop(ctx)
	})
}

func (r *Runner) loop(ctx context.Context) {
	defer close(r.done)
	r.execute(ctx)
	ticker := time.NewTicker(r.options.ScanInterval)
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
	r.stats.lastDuration.Store(r.now().Sub(startedAt).Nanoseconds())
	if appErr != nil {
		r.stats.runFailure.Add(1)
		r.logger.Printf("API 探针任务失败 error=%s", appErr.Code)
		return
	}
	r.stats.runSuccess.Add(1)
	r.stats.lastSuccess.Store(r.now().UnixNano())
	r.stats.lastClaimed.Store(int64(result.Claimed))
	r.stats.lastFinalized.Store(int64(result.Finalized))
	r.logger.Printf("API 探针任务完成 claimed=%d succeeded=%d failed=%d finalized=%d", result.Claimed, result.Succeeded, result.Failed, result.Finalized)
}

func (r *Runner) RunOnce(ctx context.Context) (RunResult, *domain.AppError) {
	if r == nil || r.repository == nil || r.prober == nil || !r.options.Enabled {
		return RunResult{}, nil
	}
	now := r.now().UTC()
	cycleTimeout := 2*r.options.Timeout + 5*time.Second
	jobs, appErr := r.repository.ClaimDueProbes(ctx, apihealth.SlotStart(now), now, r.options.BatchSize, cycleTimeout)
	if appErr != nil {
		return RunResult{}, appErr
	}
	result := RunResult{Claimed: len(jobs)}
	if len(jobs) == 0 {
		return result, nil
	}
	workers := r.options.Concurrency
	if workers > len(jobs) {
		workers = len(jobs)
	}
	queue := make(chan apihealth.ProbeJob)
	type probeExecution struct {
		result    apihealth.ProbeResult
		finalized bool
		appErr    *domain.AppError
	}
	results := make(chan probeExecution, len(jobs))
	var wait sync.WaitGroup
	for range workers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for job := range queue {
				r.stats.inflight.Add(1)
				probeResult := apihealth.ProbeResult{ErrorCode: apihealth.ErrorDecryptFailed}
				if !job.CredentialError {
					probeCtx, cancel := context.WithTimeout(ctx, cycleTimeout)
					probeResult = r.prober.Probe(probeCtx, job)
					cancel()
				}
				if probeResult.ErrorCode == "" {
					r.stats.probeSuccess.Add(1)
				} else {
					r.stats.probeFailure.Add(1)
				}
				finalizeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
				finalized, finalizeErr := r.repository.FinalizeProbe(finalizeCtx, job.Sample.ID, probeResult, r.now().UTC())
				cancel()
				r.stats.inflight.Add(-1)
				results <- probeExecution{result: probeResult, finalized: finalized, appErr: finalizeErr}
			}
		}()
	}
	go func() {
		for _, job := range jobs {
			queue <- job
		}
		close(queue)
		wait.Wait()
		close(results)
	}()
	var finalizeErr *domain.AppError
	for execution := range results {
		if execution.result.ErrorCode == "" {
			result.Succeeded++
		} else {
			result.Failed++
		}
		if execution.finalized {
			result.Finalized++
		}
		if finalizeErr == nil && execution.appErr != nil {
			finalizeErr = execution.appErr
		}
	}
	return result, finalizeErr
}

func (r *Runner) Close() {
	if r == nil || !r.options.Enabled {
		return
	}
	r.closeOnce.Do(func() {
		if r.cancel != nil {
			r.cancel()
			<-r.done
		}
	})
}

func (r *Runner) Stats() Stats {
	if r == nil {
		return Stats{}
	}
	lastSuccess := time.Time{}
	if value := r.stats.lastSuccess.Load(); value > 0 {
		lastSuccess = time.Unix(0, value).UTC()
	}
	return Stats{
		Enabled:         r.options.Enabled,
		RunSuccessTotal: r.stats.runSuccess.Load(), RunFailureTotal: r.stats.runFailure.Load(),
		ProbeSuccessTotal: r.stats.probeSuccess.Load(), ProbeFailureTotal: r.stats.probeFailure.Load(),
		Inflight: r.stats.inflight.Load(), LastDurationSeconds: time.Duration(r.stats.lastDuration.Load()).Seconds(),
		LastSuccessAt: lastSuccess, LastClaimed: int(r.stats.lastClaimed.Load()), LastFinalized: int(r.stats.lastFinalized.Load()),
	}
}

func (r *Runner) ProbeRunnerStatus() apihealth.RunnerStatus {
	if r == nil {
		return apihealth.RunnerStatus{}
	}
	stats := r.Stats()
	return apihealth.RunnerStatus{
		Enabled: stats.Enabled, LastSuccessfulScanAt: stats.LastSuccessAt, ScanInterval: r.options.ScanInterval,
	}
}
