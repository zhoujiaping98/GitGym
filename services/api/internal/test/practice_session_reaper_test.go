package test

import (
	"context"
	"sync"
	"testing"
	"time"

	"gitgym/services/api/internal/domain"
	"gitgym/services/api/internal/runner"
	"gitgym/services/api/internal/service"
)

func TestStartPracticeSessionExpiryLoopSweepsImmediatelyAndOnInterval(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sweeps := make(chan struct{}, 4)
	svc := &stubPracticeService{
		expireStaleSessionsFunc: func(context.Context) (int, error) {
			sweeps <- struct{}{}
			return 1, nil
		},
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		service.StartPracticeSessionExpiryLoop(ctx, svc, 10*time.Millisecond, nil)
	}()

	select {
	case <-sweeps:
	case <-time.After(time.Second):
		t.Fatal("expected initial expiry sweep")
	}

	select {
	case <-sweeps:
	case <-time.After(time.Second):
		t.Fatal("expected interval expiry sweep")
	}

	cancel()
	wg.Wait()
}

func TestStartWorkspaceCleanupLoopSweepsImmediatelyAndOnInterval(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sweeps := make(chan struct{}, 4)
	svc := &stubCleanupLoopPracticeService{
		runWorkspaceCleanupDueJobsFunc: func(context.Context, int) error {
			sweeps <- struct{}{}
			return nil
		},
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		service.StartWorkspaceCleanupLoop(ctx, svc, 10*time.Millisecond, nil)
	}()

	select {
	case <-sweeps:
	case <-time.After(time.Second):
		t.Fatal("expected initial workspace cleanup sweep")
	}

	select {
	case <-sweeps:
	case <-time.After(time.Second):
		t.Fatal("expected interval workspace cleanup sweep")
	}

	cancel()
	wg.Wait()
}

func TestStartWorkspaceCleanupLoopRunsReconciliationImmediatelyAndOnInterval(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	executions := make(chan struct{}, 4)
	reconciliations := make(chan struct{}, 4)
	svc := &stubCleanupLoopPracticeService{
		runWorkspaceCleanupDueJobsFunc: func(context.Context, int) error {
			executions <- struct{}{}
			return nil
		},
		reconcileWorkspaceCleanupJobsFunc: func(context.Context, int) (service.WorkspaceCleanupReconciliationSummary, error) {
			reconciliations <- struct{}{}
			return service.WorkspaceCleanupReconciliationSummary{}, nil
		},
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		service.StartWorkspaceCleanupLoop(ctx, svc, 10*time.Millisecond, nil)
	}()

	select {
	case <-executions:
	case <-time.After(time.Second):
		t.Fatal("expected initial cleanup execution sweep")
	}
	select {
	case <-reconciliations:
	case <-time.After(time.Second):
		t.Fatal("expected initial cleanup reconciliation sweep")
	}
	select {
	case <-executions:
	case <-time.After(time.Second):
		t.Fatal("expected interval cleanup execution sweep")
	}
	select {
	case <-reconciliations:
	case <-time.After(time.Second):
		t.Fatal("expected interval cleanup reconciliation sweep")
	}

	cancel()
	wg.Wait()
}

type stubCleanupLoopPracticeService struct {
	stubPracticeService
	runWorkspaceCleanupDueJobsFunc    func(context.Context, int) error
	reconcileWorkspaceCleanupJobsFunc func(context.Context, int) (service.WorkspaceCleanupReconciliationSummary, error)
}

func (s *stubCleanupLoopPracticeService) RunWorkspaceCleanupDueJobs(ctx context.Context, limit int) error {
	if s.runWorkspaceCleanupDueJobsFunc != nil {
		return s.runWorkspaceCleanupDueJobsFunc(ctx, limit)
	}
	return nil
}

func (s *stubCleanupLoopPracticeService) ReconcileWorkspaceCleanupJobs(ctx context.Context, limit int) (service.WorkspaceCleanupReconciliationSummary, error) {
	if s.reconcileWorkspaceCleanupJobsFunc != nil {
		return s.reconcileWorkspaceCleanupJobsFunc(ctx, limit)
	}
	return service.WorkspaceCleanupReconciliationSummary{}, nil
}

var _ service.PracticeService = (*stubPracticeService)(nil)
var _ runner.TerminalConnection = (*stubTerminalConnection)(nil)
var _ = domain.PracticeSession{}
