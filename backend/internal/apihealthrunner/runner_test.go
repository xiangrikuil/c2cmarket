package apihealthrunner

import (
	"context"
	"io"
	"log"
	"net/http"
	"sync"
	"testing"
	"time"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apihealth"
)

func TestRunOnceReturnsPerRunCountsAndCumulativeStats(t *testing.T) {
	t.Parallel()
	repository := &runnerRepository{batches: [][]apihealth.ProbeJob{
		{{Sample: apihealth.Sample{ID: "success-1"}}, {Sample: apihealth.Sample{ID: "failure-1"}}},
		{{Sample: apihealth.Sample{ID: "success-2"}}},
	}}
	prober := &runnerProber{results: map[string]apihealth.ProbeResult{
		"success-1": {},
		"failure-1": {ErrorCode: apihealth.ErrorHTTP5xx},
		"success-2": {},
	}}
	runner := newEnabledRunner(repository, prober)

	first, appErr := runner.RunOnce(context.Background())
	if appErr != nil {
		t.Fatalf("first run: %v", appErr)
	}
	if first != (RunResult{Claimed: 2, Succeeded: 1, Failed: 1, Finalized: 2}) {
		t.Fatalf("unexpected first result: %+v", first)
	}
	second, appErr := runner.RunOnce(context.Background())
	if appErr != nil {
		t.Fatalf("second run: %v", appErr)
	}
	if second != (RunResult{Claimed: 1, Succeeded: 1, Finalized: 1}) {
		t.Fatalf("unexpected second result: %+v", second)
	}
	stats := runner.Stats()
	if stats.ProbeSuccessTotal != 2 || stats.ProbeFailureTotal != 1 || stats.Inflight != 0 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestRunOnceFinalizesCredentialDecryptionFailureWithoutCallingProber(t *testing.T) {
	t.Parallel()
	repository := &runnerRepository{batches: [][]apihealth.ProbeJob{{{
		Sample: apihealth.Sample{ID: "decrypt-failure"}, CredentialError: true,
	}}}}
	prober := &runnerProber{}
	runner := newEnabledRunner(repository, prober)

	result, appErr := runner.RunOnce(context.Background())

	if appErr != nil {
		t.Fatalf("run: %v", appErr)
	}
	if result != (RunResult{Claimed: 1, Failed: 1, Finalized: 1}) {
		t.Fatalf("unexpected result: %+v", result)
	}
	if prober.CallCount() != 0 {
		t.Fatalf("prober called %d times", prober.CallCount())
	}
	if got := repository.FinalizedResult("decrypt-failure").ErrorCode; got != apihealth.ErrorDecryptFailed {
		t.Fatalf("unexpected error code: %q", got)
	}
}

func TestRunOnceSurfacesFinalizeFailureAfterCompletingBatch(t *testing.T) {
	t.Parallel()
	repository := &runnerRepository{
		batches:     [][]apihealth.ProbeJob{{{Sample: apihealth.Sample{ID: "sample-1"}}}},
		finalizeErr: domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "服务暂时不可用。"),
	}
	runner := newEnabledRunner(repository, &runnerProber{})

	result, appErr := runner.RunOnce(context.Background())

	if appErr == nil || appErr.Code != domain.CodeInternalError {
		t.Fatalf("expected finalize error, got %v", appErr)
	}
	if result != (RunResult{Claimed: 1, Succeeded: 1}) {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestCloseCancelsInflightProbeAndWaitsForFinalize(t *testing.T) {
	t.Parallel()
	repository := &runnerRepository{batches: [][]apihealth.ProbeJob{{{Sample: apihealth.Sample{ID: "sample-1"}}}}}
	prober := &runnerProber{started: make(chan struct{}), blockUntilCanceled: true}
	runner := newEnabledRunner(repository, prober)
	runner.Start(context.Background())

	select {
	case <-prober.started:
	case <-time.After(time.Second):
		t.Fatal("probe did not start")
	}
	closed := make(chan struct{})
	go func() {
		runner.Close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("runner did not close")
	}
	if got := repository.FinalizedResult("sample-1").ErrorCode; got != apihealth.ErrorTimeout {
		t.Fatalf("unexpected finalized result: %q", got)
	}
}

func newEnabledRunner(repository Repository, prober apihealth.Prober) *Runner {
	return New(repository, prober, Options{
		Enabled: true, ScanInterval: time.Hour, Timeout: time.Second, Concurrency: 2, BatchSize: 50,
	}, func() time.Time { return time.Date(2026, time.August, 4, 12, 0, 0, 0, time.UTC) }, log.New(io.Discard, "", 0))
}

type runnerRepository struct {
	mu          sync.Mutex
	batches     [][]apihealth.ProbeJob
	finalized   map[string]apihealth.ProbeResult
	finalizeErr *domain.AppError
}

func (repository *runnerRepository) ClaimDueProbes(_ context.Context, _ time.Time, _ time.Time, _ int, _ time.Duration) ([]apihealth.ProbeJob, *domain.AppError) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if len(repository.batches) == 0 {
		return []apihealth.ProbeJob{}, nil
	}
	batch := repository.batches[0]
	repository.batches = repository.batches[1:]
	return batch, nil
}

func (repository *runnerRepository) FinalizeProbe(_ context.Context, sampleID string, result apihealth.ProbeResult, _ time.Time) (bool, *domain.AppError) {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	if repository.finalizeErr != nil {
		return false, repository.finalizeErr
	}
	if repository.finalized == nil {
		repository.finalized = make(map[string]apihealth.ProbeResult)
	}
	repository.finalized[sampleID] = result
	return true, nil
}

func (repository *runnerRepository) FinalizedResult(sampleID string) apihealth.ProbeResult {
	repository.mu.Lock()
	defer repository.mu.Unlock()
	return repository.finalized[sampleID]
}

type runnerProber struct {
	mu                 sync.Mutex
	results            map[string]apihealth.ProbeResult
	calls              int
	started            chan struct{}
	startOnce          sync.Once
	blockUntilCanceled bool
}

func (prober *runnerProber) Probe(ctx context.Context, job apihealth.ProbeJob) apihealth.ProbeResult {
	prober.mu.Lock()
	prober.calls++
	prober.mu.Unlock()
	if prober.started != nil {
		prober.startOnce.Do(func() { close(prober.started) })
	}
	if prober.blockUntilCanceled {
		<-ctx.Done()
		return apihealth.ProbeResult{ErrorCode: apihealth.ErrorTimeout}
	}
	return prober.results[job.Sample.ID]
}

func (prober *runnerProber) CallCount() int {
	prober.mu.Lock()
	defer prober.mu.Unlock()
	return prober.calls
}
