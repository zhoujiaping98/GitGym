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

func TestStartPracticeSessionExpiryLoopAlsoRunsWorkspaceCleanupSweep(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	expirySweeps := make(chan struct{}, 2)
	cleanupSweeps := make(chan struct{}, 2)
	svc := &stubCleanupLoopPracticeService{
		stubPracticeService: stubPracticeService{
			expireStaleSessionsFunc: func(context.Context) (int, error) {
				expirySweeps <- struct{}{}
				return 1, nil
			},
		},
		runWorkspaceCleanupDueJobsFunc: func(context.Context, int) error {
			cleanupSweeps <- struct{}{}
			return nil
		},
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		service.StartPracticeSessionExpiryLoop(ctx, svc, 10*time.Millisecond, nil)
	}()

	select {
	case <-expirySweeps:
	case <-time.After(time.Second):
		t.Fatal("expected initial expiry sweep")
	}

	select {
	case <-cleanupSweeps:
	case <-time.After(time.Second):
		t.Fatal("expected initial workspace cleanup sweep")
	}

	cancel()
	wg.Wait()
}

type stubCleanupLoopPracticeService struct {
	stubPracticeService
	runWorkspaceCleanupDueJobsFunc func(context.Context, int) error
}

func (s *stubCleanupLoopPracticeService) RunWorkspaceCleanupDueJobs(ctx context.Context, limit int) error {
	if s.runWorkspaceCleanupDueJobsFunc != nil {
		return s.runWorkspaceCleanupDueJobsFunc(ctx, limit)
	}
	return nil
}

var _ service.PracticeService = (*stubPracticeService)(nil)
var _ runner.TerminalConnection = (*stubTerminalConnection)(nil)
var _ = domain.PracticeSession{}
