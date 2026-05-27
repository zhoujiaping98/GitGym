# Workspace Cleanup Retry Policy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add bounded retry semantics to persistent workspace cleanup jobs so transient failures still retry, but exhausted failures stop being reclaimed automatically after five attempts.

**Architecture:** Reuse the existing cleanup-job schema and store interfaces, then tighten claim eligibility and worker failure branching in the API service layer. The MySQL store and in-memory store must agree on exhaustion semantics so tests and fallback mode match production behavior.

**Tech Stack:** Go, MySQL-backed store queries, in-memory store parity, Go tests

---

## File Structure

### Existing files to modify

- `services/api/internal/service/practice_service.go`
  - add retry-attempt constants and branch cleanup failure handling between retryable and exhausted outcomes
- `services/api/internal/store/mysql.go`
  - stop reclaiming failed or stale-running jobs once `attempt_count` reaches the retry ceiling
- `services/api/internal/test/practice_service_test.go`
  - cover exhausted-failure behavior in the worker and in-memory store parity
- `services/api/internal/test/workspace_cleanup_jobs_store_test.go`
  - cover MySQL claim behavior for exhausted failed and exhausted stale-running jobs

### Files explicitly out of scope

- `db/migrations/**`
  - no schema changes in this slice
- `services/runner/**`
  - runner delete behavior stays unchanged
- `apps/web/**`
  - no frontend changes

---

### Task 1: Add exhausted-claim rules to the stores under TDD

**Files:**
- Modify: `services/api/internal/store/mysql.go`
- Modify: `services/api/internal/service/practice_service.go`
- Test: `services/api/internal/test/workspace_cleanup_jobs_store_test.go`
- Test: `services/api/internal/test/practice_service_test.go`

- [ ] **Step 1: Write the failing MySQL store test for exhausted failed jobs**

Add this test to `services/api/internal/test/workspace_cleanup_jobs_store_test.go`:

```go
func TestWorkspaceCleanupJobStoreDoesNotClaimExhaustedFailedJobs(t *testing.T) {
	t.Parallel()

	store := newTestMySQLStore(t)
	sessionID := seedPracticeSession(t, store, seedPracticeSessionParams{
		userID:     11,
		scenarioID: 1,
		templateID: 1,
		runnerRef:  "ws-cleanup-exhausted-failed",
		workspace:  "/tmp/ws-cleanup-exhausted-failed",
		status:     "expired",
	})

	now := time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC)
	if err := store.UpsertWorkspaceCleanupJob(context.Background(), domain.WorkspaceCleanupJob{
		PracticeSessionID: sessionID,
		WorkspaceID:       "ws-cleanup-exhausted-failed",
		Reason:            service.PracticeSessionStatusExpired,
		ScheduledAt:       now.Add(-time.Minute),
		Status:            "pending",
	}); err != nil {
		t.Fatalf("seed cleanup job: %v", err)
	}

	claimed, err := store.ClaimDueWorkspaceCleanupJobs(context.Background(), now, 10)
	if err != nil {
		t.Fatalf("first claim cleanup jobs: %v", err)
	}
	if len(claimed) != 1 {
		t.Fatalf("expected one initially claimed job, got %d", len(claimed))
	}

	if err := store.MarkWorkspaceCleanupJobFailed(context.Background(), claimed[0].ID, now.Add(time.Minute), "attempt 1 failed"); err != nil {
		t.Fatalf("mark first failed cleanup attempt: %v", err)
	}

	for attempt := uint32(2); attempt <= 5; attempt++ {
		reclaimed, err := store.ClaimDueWorkspaceCleanupJobs(context.Background(), now.Add(time.Duration(attempt)*time.Minute), 10)
		if err != nil {
			t.Fatalf("claim cleanup jobs on attempt %d: %v", attempt, err)
		}
		if len(reclaimed) != 1 {
			t.Fatalf("expected one reclaimed job on attempt %d, got %d", attempt, len(reclaimed))
		}
		if reclaimed[0].AttemptCount != attempt {
			t.Fatalf("expected reclaimed attempt_count %d, got %d", attempt, reclaimed[0].AttemptCount)
		}
		if err := store.MarkWorkspaceCleanupJobFailed(context.Background(), reclaimed[0].ID, now, fmt.Sprintf("attempt %d failed", attempt)); err != nil {
			t.Fatalf("mark cleanup job failed on attempt %d: %v", attempt, err)
		}
	}

	exhaustedClaim, err := store.ClaimDueWorkspaceCleanupJobs(context.Background(), now.Add(10*time.Minute), 10)
	if err != nil {
		t.Fatalf("claim exhausted failed cleanup jobs: %v", err)
	}
	if len(exhaustedClaim) != 0 {
		t.Fatalf("expected no exhausted failed cleanup jobs to be claimed, got %d", len(exhaustedClaim))
	}
}
```

- [ ] **Step 2: Run the failing exhausted-failed store test**

Run:

```bash
go test ./services/api/internal/test -run "TestWorkspaceCleanupJobStoreDoesNotClaimExhaustedFailedJobs" -v
```

Expected:

```text
FAIL
```

- [ ] **Step 3: Write the failing parity tests for exhausted stale-running jobs**

Add one MySQL-backed test to `services/api/internal/test/workspace_cleanup_jobs_store_test.go`:

```go
func TestWorkspaceCleanupJobStoreDoesNotReclaimExhaustedRunningJobs(t *testing.T) {
	t.Parallel()

	store := newTestMySQLStore(t)
	sessionID := seedPracticeSession(t, store, seedPracticeSessionParams{
		userID:     12,
		scenarioID: 1,
		templateID: 1,
		runnerRef:  "ws-cleanup-exhausted-running",
		workspace:  "/tmp/ws-cleanup-exhausted-running",
		status:     "expired",
	})

	now := time.Date(2026, 5, 27, 11, 0, 0, 0, time.UTC)
	if err := store.UpsertWorkspaceCleanupJob(context.Background(), domain.WorkspaceCleanupJob{
		PracticeSessionID: sessionID,
		WorkspaceID:       "ws-cleanup-exhausted-running",
		Reason:            service.PracticeSessionStatusExpired,
		ScheduledAt:       now.Add(-time.Minute),
		Status:            "pending",
	}); err != nil {
		t.Fatalf("seed cleanup job: %v", err)
	}

	claimed, err := store.ClaimDueWorkspaceCleanupJobs(context.Background(), now, 10)
	if err != nil {
		t.Fatalf("claim cleanup jobs: %v", err)
	}
	if len(claimed) != 1 {
		t.Fatalf("expected one initially claimed job, got %d", len(claimed))
	}

	for attempt := uint32(1); attempt < 5; attempt++ {
		if err := store.MarkWorkspaceCleanupJobFailed(context.Background(), claimed[0].ID, now, fmt.Sprintf("attempt %d failed", attempt)); err != nil {
			t.Fatalf("mark cleanup job failed on attempt %d: %v", attempt, err)
		}
		claimed, err = store.ClaimDueWorkspaceCleanupJobs(context.Background(), now, 10)
		if err != nil {
			t.Fatalf("reclaim cleanup job on attempt %d: %v", attempt+1, err)
		}
		if len(claimed) != 1 {
			t.Fatalf("expected one claimed job on attempt %d, got %d", attempt+1, len(claimed))
		}
	}

	reclaimAt := now.Add(service.WorkspaceCleanupJobLeaseTimeout + time.Minute)
	reclaimed, err := store.ClaimDueWorkspaceCleanupJobs(context.Background(), reclaimAt, 10)
	if err != nil {
		t.Fatalf("claim exhausted running cleanup jobs: %v", err)
	}
	if len(reclaimed) != 0 {
		t.Fatalf("expected no exhausted running cleanup jobs to be reclaimed, got %d", len(reclaimed))
	}
}
```

Add one in-memory parity test to `services/api/internal/test/practice_service_test.go`:

```go
func TestInMemoryPracticeSessionStoreDoesNotReclaimExhaustedRunningCleanupJobs(t *testing.T) {
	t.Parallel()

	store := service.NewInMemoryPracticeSessionStore()
	now := time.Date(2026, 5, 27, 11, 30, 0, 0, time.UTC)

	session, err := store.CreatePracticeSession(context.Background(), domain.PracticeSession{
		UserID:           52,
		ScenarioID:       1,
		TemplateID:       1,
		RunnerRef:        "ws-in-memory-exhausted-running",
		WorkspacePathRef: "/tmp/ws-in-memory-exhausted-running",
		Status:           service.PracticeSessionStatusExpired,
		StartedAt:        now.Add(-4 * time.Hour),
		ExpiresAt:        now.Add(-2 * time.Hour),
		LastActivityAt:   now.Add(-2 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create practice session: %v", err)
	}

	if err := store.UpsertWorkspaceCleanupJob(context.Background(), domain.WorkspaceCleanupJob{
		PracticeSessionID: session.ID,
		WorkspaceID:       session.RunnerRef,
		Reason:            service.PracticeSessionStatusExpired,
		ScheduledAt:       now.Add(-time.Minute),
		Status:            "pending",
	}); err != nil {
		t.Fatalf("seed cleanup job: %v", err)
	}

	var claimed []domain.WorkspaceCleanupJob
	for attempt := 1; attempt <= 5; attempt++ {
		claimed, err = store.ClaimDueWorkspaceCleanupJobs(context.Background(), now, 10)
		if err != nil {
			t.Fatalf("claim cleanup jobs on attempt %d: %v", attempt, err)
		}
		if len(claimed) != 1 {
			t.Fatalf("expected one claimed cleanup job on attempt %d, got %d", attempt, len(claimed))
		}
		if attempt < 5 {
			if err := store.MarkWorkspaceCleanupJobFailed(context.Background(), claimed[0].ID, now, "transient failure"); err != nil {
				t.Fatalf("mark cleanup job failed on attempt %d: %v", attempt, err)
			}
		}
	}

	reclaimAt := now.Add(service.WorkspaceCleanupJobLeaseTimeout + time.Minute)
	reclaimed, err := store.ClaimDueWorkspaceCleanupJobs(context.Background(), reclaimAt, 10)
	if err != nil {
		t.Fatalf("claim exhausted running cleanup jobs: %v", err)
	}
	if len(reclaimed) != 0 {
		t.Fatalf("expected no exhausted running cleanup jobs to be reclaimed, got %d", len(reclaimed))
	}
}
```

- [ ] **Step 4: Run the failing exhausted-running tests**

Run:

```bash
go test ./services/api/internal/test -run "TestWorkspaceCleanupJobStoreDoesNotReclaimExhaustedRunningJobs|TestInMemoryPracticeSessionStoreDoesNotReclaimExhaustedRunningCleanupJobs" -v
```

Expected:

```text
FAIL
```

- [ ] **Step 5: Implement minimal store-side attempt gating**

In `services/api/internal/service/practice_service.go`, add:

```go
const workspaceCleanupMaxAttempts = 5
```

Update the in-memory claim loop to stop selecting exhausted jobs:

```go
		case "pending", "failed":
			if job.AttemptCount >= workspaceCleanupMaxAttempts {
				continue
			}
			if job.ScheduledAt.After(now) {
				continue
			}
		case "running":
			if job.AttemptCount >= workspaceCleanupMaxAttempts {
				continue
			}
			if job.UpdatedAt.After(staleRunningBefore) {
				continue
			}
```

In `services/api/internal/store/mysql.go`, tighten the claim query parameters so failed and stale-running jobs are only eligible when `attempt_count < workspaceCleanupMaxAttempts`.

- [ ] **Step 6: Run the store and parity tests until green**

Run:

```bash
go test ./services/api/internal/test -run "TestWorkspaceCleanupJobStoreDoesNotClaimExhaustedFailedJobs|TestWorkspaceCleanupJobStoreDoesNotReclaimExhaustedRunningJobs|TestInMemoryPracticeSessionStoreDoesNotReclaimExhaustedRunningCleanupJobs" -v
```

Expected:

```text
PASS
```

- [ ] **Step 7: Commit Task 1**

Run:

```bash
git add services/api/internal/store/mysql.go services/api/internal/service/practice_service.go services/api/internal/test/workspace_cleanup_jobs_store_test.go services/api/internal/test/practice_service_test.go
git commit -m "fix: stop reclaiming exhausted cleanup jobs"
```

### Task 2: Add worker-side exhausted failure behavior under TDD

**Files:**
- Modify: `services/api/internal/service/practice_service.go`
- Test: `services/api/internal/test/practice_service_test.go`

- [ ] **Step 1: Write the failing service test for exhausted cleanup failures**

Add this test to `services/api/internal/test/practice_service_test.go`:

```go
func TestPracticeServiceRunWorkspaceCleanupDueJobsStopsRetryingAfterMaxAttempts(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)
	store := &stubPracticeSessionStore{
		claimedCleanupJobs: []domain.WorkspaceCleanupJob{
			{
				ID:                17,
				PracticeSessionID: 71,
				WorkspaceID:       "ws-cleanup-exhausted",
				Reason:            service.PracticeSessionStatusExpired,
				ScheduledAt:       now.Add(-time.Minute),
				Status:            "running",
				AttemptCount:      5,
			},
		},
	}
	runnerClient := &stubRunnerClient{deleteWorkspaceErr: errors.New("runner unavailable")}
	svc := service.NewPracticeService(store, runnerClient, func() time.Time { return now })

	err := svc.RunWorkspaceCleanupDueJobs(context.Background(), 10)

	if err == nil {
		t.Fatal("expected exhausted cleanup failure error")
	}
	if runnerClient.deleteWorkspaceCalls != 1 {
		t.Fatalf("expected one delete workspace call, got %d", runnerClient.deleteWorkspaceCalls)
	}
	if len(store.markCleanupFailedCalls) != 1 {
		t.Fatalf("expected one failed cleanup mark, got %d", len(store.markCleanupFailedCalls))
	}
	failure := store.markCleanupFailedCalls[0]
	if failure.jobID != 17 {
		t.Fatalf("expected failed cleanup job id 17, got %d", failure.jobID)
	}
	if !failure.scheduledAt.Equal(now) {
		t.Fatalf("expected exhausted cleanup job scheduled_at %v, got %v", now, failure.scheduledAt)
	}
	if !strings.Contains(failure.lastErr, "runner unavailable") {
		t.Fatalf("expected exhausted cleanup last error to include runner failure, got %q", failure.lastErr)
	}
	if !strings.Contains(err.Error(), "exhausted") {
		t.Fatalf("expected exhausted cleanup error, got %v", err)
	}
}
```

- [ ] **Step 2: Run the failing exhausted worker test**

Run:

```bash
go test ./services/api/internal/test -run "TestPracticeServiceRunWorkspaceCleanupDueJobsStopsRetryingAfterMaxAttempts" -v
```

Expected:

```text
FAIL
```

- [ ] **Step 3: Write the failing service test for retryable cleanup failures**

Add this focused test to make the non-exhausted branch explicit:

```go
func TestPracticeServiceRunWorkspaceCleanupDueJobsReschedulesRetryableFailures(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 27, 12, 30, 0, 0, time.UTC)
	store := &stubPracticeSessionStore{
		claimedCleanupJobs: []domain.WorkspaceCleanupJob{
			{
				ID:                18,
				PracticeSessionID: 72,
				WorkspaceID:       "ws-cleanup-retryable",
				Reason:            service.PracticeSessionStatusExpired,
				ScheduledAt:       now.Add(-time.Minute),
				Status:            "running",
				AttemptCount:      2,
			},
		},
	}
	runnerClient := &stubRunnerClient{deleteWorkspaceErr: errors.New("runner timeout")}
	svc := service.NewPracticeService(store, runnerClient, func() time.Time { return now })

	err := svc.RunWorkspaceCleanupDueJobs(context.Background(), 10)

	if err == nil {
		t.Fatal("expected retryable cleanup failure error")
	}
	if len(store.markCleanupFailedCalls) != 1 {
		t.Fatalf("expected one failed cleanup mark, got %d", len(store.markCleanupFailedCalls))
	}
	failure := store.markCleanupFailedCalls[0]
	expectedRetryAt := now.Add(5 * time.Minute)
	if !failure.scheduledAt.Equal(expectedRetryAt) {
		t.Fatalf("expected retryable cleanup reschedule at %v, got %v", expectedRetryAt, failure.scheduledAt)
	}
	if strings.Contains(err.Error(), "exhausted") {
		t.Fatalf("expected retryable failure error, got %v", err)
	}
}
```

- [ ] **Step 4: Run the failing retryable worker test**

Run:

```bash
go test ./services/api/internal/test -run "TestPracticeServiceRunWorkspaceCleanupDueJobsReschedulesRetryableFailures" -v
```

Expected:

```text
FAIL
```

- [ ] **Step 5: Implement minimal worker-side failure branching**

In `services/api/internal/service/practice_service.go`, keep the existing backoff helper but add a retryability helper:

```go
func workspaceCleanupAttemptsExhausted(attempt uint32) bool {
	return attempt >= workspaceCleanupMaxAttempts
}
```

Update `RunWorkspaceCleanupDueJobs` so failure handling becomes:

```go
		runErrs = append(runErrs, fmt.Errorf("delete workspace for cleanup job %d: %w", job.ID, err))
		if workspaceCleanupAttemptsExhausted(job.AttemptCount) {
			if markErr := s.markWorkspaceCleanupJobFailed(ctx, job, now, err.Error()); markErr != nil {
				runErrs = append(runErrs, markErr)
			}
			runErrs = append(runErrs, fmt.Errorf("cleanup job %d exhausted retries after attempt %d", job.ID, job.AttemptCount))
			continue
		}

		nextRun := nextWorkspaceCleanupRetryAt(now, job.AttemptCount)
		if markErr := s.markWorkspaceCleanupJobFailed(ctx, job, nextRun, err.Error()); markErr != nil {
			runErrs = append(runErrs, markErr)
		}
```

Do not change success handling.

- [ ] **Step 6: Run the focused worker tests until green**

Run:

```bash
go test ./services/api/internal/test -run "TestPracticeServiceRunWorkspaceCleanupDueJobsStopsRetryingAfterMaxAttempts|TestPracticeServiceRunWorkspaceCleanupDueJobsReschedulesRetryableFailures" -v
```

Expected:

```text
PASS
```

- [ ] **Step 7: Run the broader API test suite**

Run:

```bash
go test ./services/api/... 
```

Expected:

```text
PASS
```

- [ ] **Step 8: Commit Task 2**

Run:

```bash
git add services/api/internal/service/practice_service.go services/api/internal/test/practice_service_test.go
git commit -m "fix: bound workspace cleanup retries"
```
