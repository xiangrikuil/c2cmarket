package evidence

import (
	"testing"
	"time"
)

func TestCleanupRunnerLifecycleStartsOnceAndDoesNotRestartAfterClose(t *testing.T) {
	repo := &repositoryStub{claimNotify: make(chan struct{}, 4)}
	runner := NewCleanupRunner(NewService(repo, &objectStoreStub{}, time.Now), time.Hour, 10)
	if runner == nil {
		t.Fatal("expected cleanup runner")
	}
	runner.Start()
	runner.Start()
	select {
	case <-repo.claimNotify:
	case <-time.After(time.Second):
		t.Fatal("runner did not execute immediately")
	}
	runner.Close()
	runner.Close()

	repo.mu.Lock()
	before := repo.claimCalls
	repo.mu.Unlock()
	runner.Start()
	time.Sleep(20 * time.Millisecond)
	repo.mu.Lock()
	after := repo.claimCalls
	repo.mu.Unlock()
	if before != 1 || after != before {
		t.Fatalf("runner lifecycle calls before=%d after=%d", before, after)
	}
}

func TestNewCleanupRunnerRejectsDisabledConfiguration(t *testing.T) {
	if NewCleanupRunner(nil, time.Minute, 1) != nil ||
		NewCleanupRunner(NewService(&repositoryStub{}, &objectStoreStub{}, nil), 0, 1) != nil ||
		NewCleanupRunner(NewService(&repositoryStub{}, &objectStoreStub{}, nil), time.Minute, 0) != nil {
		t.Fatal("invalid cleanup configuration must not create a runner")
	}
}
