# Workspace Cleanup Reconciliation Sweep Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a reconciliation sweep that backfills missing workspace cleanup jobs for `expired` and `orphaned` sessions and reports exhausted failed jobs without mutating them.

**Architecture:** Extend the existing cleanup service/store boundary with two read-side reconciliation queries and one service reconciliation method that returns a small summary. Then wire that reconciliation step into the existing workspace cleanup loop so execution and repair happen in the same tick.

**Tech Stack:** Go, MySQL store queries, in-memory store parity, background loop tests

---

## File Structure

### Existing files to modify

- `services/api/internal/service/practice_service.go`
  - add reconciliation summary type, service method, and in-memory query implementations
- `services/api/internal/store/mysql.go`
  - add MySQL reconciliation queries and store methods
- `services/api/internal/service/practice_session_reaper.go`
  - run reconciliation after cleanup execution and log summary values
- `services/api/internal/test/practice_service_test.go`
  - add reconciliation behavior tests
- `services/api/internal/test/practice_session_reaper_test.go`
  - add loop coverage for reconciliation
- `services/api/internal/test/workspace_cleanup_jobs_store_test.go`
  - add store integration tests for missing-job and exhausted-job queries
- `services/api/internal/test/practice_routes_test.go`
  - extend the stub service if the widened `PracticeService` interface requires it

### Files explicitly out of scope

- `db/migrations/**`
  - no schema changes
- `services/runner/**`
  - no runner changes
- `apps/web/**`
  - no frontend work

---

### Task 1: Add reconciliation store queries and service behavior under TDD

**Files:**
- Modify: `services/api/internal/service/practice_service.go`
- Modify: `services/api/internal/store/mysql.go`
- Modify: `services/api/internal/test/practice_service_test.go`
- Modify: `services/api/internal/test/workspace_cleanup_jobs_store_test.go`

- [ ] **Step 1: Write the failing service test for missing expired cleanup jobs**

Add this test to `services/api/internal/test/practice_service_test.go`:

```go
func TestPracticeServiceReconcileWorkspaceCleanupJobsBackfillsMissingExpiredJobs(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 27, 14, 0, 0, 0, time.UTC)
	store := &stubPracticeSessionStore{
		missingCleanupJobSessions: []domain.PracticeSession{
			{
				ID:         81,
				RunnerRef:  "ws-missing-expired",
				Status:     service.PracticeSessionStatusExpired,
				EndedAt:    timePtr(now.Add(-30 * time.Minute)),
			},
		},
	}
	svc := service.NewPracticeService(store, &stubRunnerClient{}, func() time.Time { return now })

	summary, err := svc.ReconcileWorkspaceCleanupJobs(context.Background(), 10)

	if err != nil {
		t.Fatalf("reconcile workspace cleanup jobs: %v", err)
	}
	if summary.BackfilledJobs != 1 {
		t.Fatalf("expected one backfilled cleanup job, got %d", summary.BackfilledJobs)
	}
	if summary.ExhaustedFailedJobs != 0 {
		t.Fatalf("expected zero exhausted failed jobs, got %d", summary.ExhaustedFailedJobs)
	}
	if len(store.upsertCleanupJobCalls) != 1 {
		t.Fatalf("expected one cleanup job upsert, got %d", len(store.upsertCleanupJobCalls))
	}
	job := store.upsertCleanupJobCalls[0]
	if job.PracticeSessionID != 81 {
		t.Fatalf("expected cleanup job for session 81, got %d", job.PracticeSessionID)
	}
	if job.Reason != service.PracticeSessionStatusExpired {
		t.Fatalf("expected cleanup reason %q, got %q", service.PracticeSessionStatusExpired, job.Reason)
	}
	if !job.ScheduledAt.Equal(now) {
		t.Fatalf("expected immediate cleanup schedule %v, got %v", now, job.ScheduledAt)
	}
}
```

- [ ] **Step 2: Run the missing-expired service test to verify it fails**

Run:

```bash
go test ./services/api/internal/test -run "TestPracticeServiceReconcileWorkspaceCleanupJobsBackfillsMissingExpiredJobs" -v
```

Expected:

```text
FAIL
```

- [ ] **Step 3: Write the failing service tests for orphan grace and exhausted summary**

Add these tests to `services/api/internal/test/practice_service_test.go`:

```go
func TestPracticeServiceReconcileWorkspaceCleanupJobsPreservesOrphanGrace(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 27, 14, 30, 0, 0, time.UTC)
	endedAt := now.Add(-3 * time.Minute)
	store := &stubPracticeSessionStore{
		missingCleanupJobSessions: []domain.PracticeSession{
			{
				ID:         82,
				RunnerRef:  "ws-missing-orphaned",
				Status:     service.PracticeSessionStatusOrphaned,
				EndedAt:    timePtr(endedAt),
			},
		},
	}
	svc := service.NewPracticeService(store, &stubRunnerClient{}, func() time.Time { return now })

	summary, err := svc.ReconcileWorkspaceCleanupJobs(context.Background(), 10)

	if err != nil {
		t.Fatalf("reconcile workspace cleanup jobs: %v", err)
	}
	if summary.BackfilledJobs != 1 {
		t.Fatalf("expected one backfilled cleanup job, got %d", summary.BackfilledJobs)
	}
	if len(store.upsertCleanupJobCalls) != 1 {
		t.Fatalf("expected one cleanup job upsert, got %d", len(store.upsertCleanupJobCalls))
	}
	expectedSchedule := endedAt.Add(10 * time.Minute)
	if !store.upsertCleanupJobCalls[0].ScheduledAt.Equal(expectedSchedule) {
		t.Fatalf("expected orphan cleanup schedule %v, got %v", expectedSchedule, store.upsertCleanupJobCalls[0].ScheduledAt)
	}
}

func TestPracticeServiceReconcileWorkspaceCleanupJobsCountsExhaustedFailures(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 27, 15, 0, 0, 0, time.UTC)
	store := &stubPracticeSessionStore{
		exhaustedCleanupJobs: []domain.WorkspaceCleanupJob{
			{ID: 91, PracticeSessionID: 51, Status: "failed", AttemptCount: service.WorkspaceCleanupJobMaxAttempts},
			{ID: 92, PracticeSessionID: 52, Status: "failed", AttemptCount: service.WorkspaceCleanupJobMaxAttempts},
		},
	}
	svc := service.NewPracticeService(store, &stubRunnerClient{}, func() time.Time { return now })

	summary, err := svc.ReconcileWorkspaceCleanupJobs(context.Background(), 10)

	if err != nil {
		t.Fatalf("reconcile workspace cleanup jobs: %v", err)
	}
	if summary.BackfilledJobs != 0 {
		t.Fatalf("expected zero backfilled jobs, got %d", summary.BackfilledJobs)
	}
	if summary.ExhaustedFailedJobs != 2 {
		t.Fatalf("expected two exhausted failed jobs, got %d", summary.ExhaustedFailedJobs)
	}
	if len(store.upsertCleanupJobCalls) != 0 {
		t.Fatalf("expected no cleanup job upserts, got %d", len(store.upsertCleanupJobCalls))
	}
}
```

- [ ] **Step 4: Run the new service tests to verify they fail**

Run:

```bash
go test ./services/api/internal/test -run "TestPracticeServiceReconcileWorkspaceCleanupJobsPreservesOrphanGrace|TestPracticeServiceReconcileWorkspaceCleanupJobsCountsExhaustedFailures" -v
```

Expected:

```text
FAIL
```

- [ ] **Step 5: Write the failing MySQL store tests for reconciliation queries**

Add these tests to `services/api/internal/test/workspace_cleanup_jobs_store_test.go`:

```go
func TestWorkspaceCleanupJobStoreListsSessionsMissingCleanupJobs(t *testing.T) {
	t.Parallel()

	store := newTestMySQLStore(t)
	expiredSessionID := seedPracticeSession(t, store, seedPracticeSessionParams{
		userID: 16, scenarioID: 1, templateID: 1, runnerRef: "ws-missing-a", workspace: "/tmp/ws-missing-a", status: "expired",
	})
	orphanedSessionID := seedPracticeSession(t, store, seedPracticeSessionParams{
		userID: 17, scenarioID: 1, templateID: 1, runnerRef: "ws-missing-b", workspace: "/tmp/ws-missing-b", status: "orphaned",
	})
	coveredSessionID := seedPracticeSession(t, store, seedPracticeSessionParams{
		userID: 18, scenarioID: 1, templateID: 1, runnerRef: "ws-covered", workspace: "/tmp/ws-covered", status: "expired",
	})
	if err := store.UpsertWorkspaceCleanupJob(context.Background(), domain.WorkspaceCleanupJob{
		PracticeSessionID: coveredSessionID,
		WorkspaceID:       "ws-covered",
		Reason:            service.PracticeSessionStatusExpired,
		ScheduledAt:       time.Date(2026, 5, 27, 15, 30, 0, 0, time.UTC),
		Status:            "pending",
	}); err != nil {
		t.Fatalf("seed covered cleanup job: %v", err)
	}

	sessions, err := store.ListPracticeSessionsMissingWorkspaceCleanupJob(context.Background(), 10)

	if err != nil {
		t.Fatalf("list sessions missing cleanup jobs: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected two sessions missing cleanup jobs, got %d", len(sessions))
	}
	if sessions[0].ID != expiredSessionID || sessions[1].ID != orphanedSessionID {
		t.Fatalf("expected missing cleanup sessions %d and %d, got %d and %d", expiredSessionID, orphanedSessionID, sessions[0].ID, sessions[1].ID)
	}
}

func TestWorkspaceCleanupJobStoreListsExhaustedFailedJobs(t *testing.T) {
	t.Parallel()

	store := newTestMySQLStore(t)
	sessionID := seedPracticeSession(t, store, seedPracticeSessionParams{
		userID: 19, scenarioID: 1, templateID: 1, runnerRef: "ws-exhausted", workspace: "/tmp/ws-exhausted", status: "expired",
	})
	now := time.Date(2026, 5, 27, 16, 0, 0, 0, time.UTC)
	if err := store.UpsertWorkspaceCleanupJob(context.Background(), domain.WorkspaceCleanupJob{
		PracticeSessionID: sessionID,
		WorkspaceID:       "ws-exhausted",
		Reason:            service.PracticeSessionStatusExpired,
		ScheduledAt:       now,
		Status:            "pending",
	}); err != nil {
		t.Fatalf("seed cleanup job: %v", err)
	}
	for attempt := uint32(1); attempt <= service.WorkspaceCleanupJobMaxAttempts; attempt++ {
		claimed, err := store.ClaimDueWorkspaceCleanupJobs(context.Background(), now, 10)
		if err != nil {
			t.Fatalf("claim cleanup jobs on attempt %d: %v", attempt, err)
		}
		if len(claimed) != 1 {
			t.Fatalf("expected one claimed job on attempt %d, got %d", attempt, len(claimed))
		}
		if err := store.MarkWorkspaceCleanupJobFailed(context.Background(), claimed[0].ID, now, "terminal failure"); err != nil {
			t.Fatalf("mark cleanup job failed on attempt %d: %v", attempt, err)
		}
	}

	jobs, err := store.ListExhaustedWorkspaceCleanupJobs(context.Background(), 10)

	if err != nil {
		t.Fatalf("list exhausted cleanup jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected one exhausted cleanup job, got %d", len(jobs))
	}
	if jobs[0].PracticeSessionID != sessionID {
		t.Fatalf("expected exhausted cleanup job for session %d, got %d", sessionID, jobs[0].PracticeSessionID)
	}
}
```

- [ ] **Step 6: Run the store tests to verify they fail**

Run:

```bash
go test ./services/api/internal/test -run "TestWorkspaceCleanupJobStoreListsSessionsMissingCleanupJobs|TestWorkspaceCleanupJobStoreListsExhaustedFailedJobs" -v
```

Expected:

```text
FAIL
```

- [ ] **Step 7: Implement the minimal reconciliation store and service surface**

In `services/api/internal/service/practice_service.go`, extend the store and service interfaces:

```go
	ListPracticeSessionsMissingWorkspaceCleanupJob(ctx context.Context, limit int) ([]domain.PracticeSession, error)
	ListExhaustedWorkspaceCleanupJobs(ctx context.Context, limit int) ([]domain.WorkspaceCleanupJob, error)
```

```go
type WorkspaceCleanupReconciliationSummary struct {
	BackfilledJobs       int
	ExhaustedFailedJobs  int
}
```

```go
	ReconcileWorkspaceCleanupJobs(ctx context.Context, limit int) (WorkspaceCleanupReconciliationSummary, error)
```

Then add a minimal implementation:

```go
func (s *practiceService) ReconcileWorkspaceCleanupJobs(ctx context.Context, limit int) (WorkspaceCleanupReconciliationSummary, error) {
	if s.store == nil {
		return WorkspaceCleanupReconciliationSummary{}, nil
	}
	if limit <= 0 {
		limit = 10
	}

	now := s.now().UTC()
	sessions, err := s.store.ListPracticeSessionsMissingWorkspaceCleanupJob(ctx, limit)
	if err != nil {
		return WorkspaceCleanupReconciliationSummary{}, fmt.Errorf("list practice sessions missing cleanup jobs: %w", err)
	}

	summary := WorkspaceCleanupReconciliationSummary{}
	for _, session := range sessions {
		scheduledAt := now
		if session.Status == PracticeSessionStatusOrphaned && session.EndedAt != nil {
			graceSchedule := session.EndedAt.UTC().Add(practiceSessionOrphanCleanupGrace)
			if graceSchedule.After(scheduledAt) {
				scheduledAt = graceSchedule
			}
		}
		if session.Status != PracticeSessionStatusExpired && session.Status != PracticeSessionStatusOrphaned {
			continue
		}
		if err := s.store.UpsertWorkspaceCleanupJob(ctx, domain.WorkspaceCleanupJob{
			PracticeSessionID: session.ID,
			WorkspaceID:       session.RunnerRef,
			Reason:            session.Status,
			ScheduledAt:       scheduledAt,
			Status:            "pending",
		}); err != nil {
			return WorkspaceCleanupReconciliationSummary{}, fmt.Errorf("backfill cleanup job for session %d: %w", session.ID, err)
		}
		summary.BackfilledJobs++
	}

	exhaustedJobs, err := s.store.ListExhaustedWorkspaceCleanupJobs(ctx, limit)
	if err != nil {
		return WorkspaceCleanupReconciliationSummary{}, fmt.Errorf("list exhausted cleanup jobs: %w", err)
	}
	summary.ExhaustedFailedJobs = len(exhaustedJobs)
	return summary, nil
}
```

Add matching MySQL and in-memory query implementations.

- [ ] **Step 8: Run the focused reconciliation tests until green**

Run:

```bash
go test ./services/api/internal/test -run "TestPracticeServiceReconcileWorkspaceCleanupJobsBackfillsMissingExpiredJobs|TestPracticeServiceReconcileWorkspaceCleanupJobsPreservesOrphanGrace|TestPracticeServiceReconcileWorkspaceCleanupJobsCountsExhaustedFailures|TestWorkspaceCleanupJobStoreListsSessionsMissingCleanupJobs|TestWorkspaceCleanupJobStoreListsExhaustedFailedJobs" -v
```

Expected:

```text
PASS
```

- [ ] **Step 9: Commit Task 1**

Run:

```bash
git add services/api/internal/service/practice_service.go services/api/internal/store/mysql.go services/api/internal/test/practice_service_test.go services/api/internal/test/workspace_cleanup_jobs_store_test.go services/api/internal/test/practice_routes_test.go
git commit -m "feat: reconcile workspace cleanup state"
```

### Task 2: Wire reconciliation into the cleanup loop under TDD

**Files:**
- Modify: `services/api/internal/service/practice_session_reaper.go`
- Modify: `services/api/internal/test/practice_session_reaper_test.go`
- Modify: `services/api/internal/test/practice_routes_test.go` only if interface stubs need updating

- [ ] **Step 1: Write the failing loop test for reconciliation execution**

Add this test to `services/api/internal/test/practice_session_reaper_test.go`:

```go
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

	go service.StartWorkspaceCleanupLoop(ctx, svc, 10*time.Millisecond, nil)

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
}
```

- [ ] **Step 2: Run the loop test to verify it fails**

Run:

```bash
go test ./services/api/internal/test -run "TestStartWorkspaceCleanupLoopRunsReconciliationImmediatelyAndOnInterval" -v
```

Expected:

```text
FAIL
```

- [ ] **Step 3: Implement minimal loop wiring and stub support**

In `services/api/internal/service/practice_session_reaper.go`, extend the workspace cleanup loop:

```go
		if err := practiceService.RunWorkspaceCleanupDueJobs(ctx, defaultWorkspaceCleanupSweepLimit); err != nil && logger != nil {
			logger.Printf("workspace cleanup sweep failed: %v", err)
		}
		summary, err := practiceService.ReconcileWorkspaceCleanupJobs(ctx, defaultWorkspaceCleanupSweepLimit)
		if err != nil {
			if logger != nil {
				logger.Printf("workspace cleanup reconciliation failed: %v", err)
			}
			return
		}
		if logger != nil && summary.BackfilledJobs > 0 {
			logger.Printf("workspace cleanup reconciliation backfilled %d jobs", summary.BackfilledJobs)
		}
		if logger != nil && summary.ExhaustedFailedJobs > 0 {
			logger.Printf("workspace cleanup reconciliation found %d exhausted failed jobs", summary.ExhaustedFailedJobs)
		}
```

Update test stubs so the widened `PracticeService` interface compiles.

- [ ] **Step 4: Run the loop test until green**

Run:

```bash
go test ./services/api/internal/test -run "TestStartWorkspaceCleanupLoopRunsReconciliationImmediatelyAndOnInterval" -v
```

Expected:

```text
PASS
```

- [ ] **Step 5: Run the broader API suite**

Run:

```bash
go test ./services/api/...
```

Expected:

```text
PASS
```

- [ ] **Step 6: Commit Task 2**

Run:

```bash
git add services/api/internal/service/practice_session_reaper.go services/api/internal/test/practice_session_reaper_test.go services/api/internal/test/practice_routes_test.go
git commit -m "feat: add workspace cleanup reconciliation sweep"
```
