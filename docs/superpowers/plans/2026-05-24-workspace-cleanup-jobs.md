# Workspace Cleanup Jobs Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace best-effort workspace cleanup scheduling with durable cleanup jobs that survive transient API/runner failures and can be retried by a background API worker.

**Architecture:** Add a new `workspace_cleanup_jobs` persistence layer in the API, create or update jobs when sessions transition to `expired` or `orphaned`, and execute those jobs from a background cleanup loop that calls the existing runner delete endpoint. Runner delete semantics stay unchanged; the API becomes the durable owner of cleanup intent and retry state.

**Tech Stack:** Go, MySQL migrations, store/query code, background worker loops, Go tests

---

## File Structure

### Existing files to modify

- `db/migrations/0001_initial.sql`
  - do not modify; use a new forward migration instead
- `services/api/internal/domain/practice_session.go`
  - add a cleanup-job domain type if the project keeps persistence models in `domain`
- `services/api/internal/store/mysql.go`
  - add cleanup-job queries and store methods
- `services/api/internal/service/practice_service.go`
  - replace in-memory best-effort cleanup scheduling with cleanup-job upserts and a worker-facing execution path
- `services/api/internal/service/practice_session_reaper.go`
  - start the cleanup-jobs loop alongside the existing expiry loop, or extend this file to host both loops cleanly
- `services/api/internal/runner/client.go`
  - reuse existing delete semantics; only touch if worker execution needs narrower helpers or comments
- `services/api/internal/test/practice_service_test.go`
  - replace old best-effort cleanup expectations with cleanup-job expectations and worker execution coverage
- `services/api/internal/test/practice_session_reaper_test.go`
  - cover startup/loop behavior for the cleanup worker if this file already hosts reaper-loop tests

### New files to create

- `db/migrations/0003_workspace_cleanup_jobs.sql`
  - create the durable cleanup-jobs table and indexes
- `services/api/internal/test/workspace_cleanup_jobs_store_test.go`
  - store-level tests for insert, upsert, claim, success, and failure-reschedule behavior

### Files explicitly out of scope

- `services/runner/**`
  - no new behavior in this slice
- `apps/web/**`
  - no frontend changes

---

### Task 1: Add the cleanup-jobs schema and store methods under TDD

**Files:**
- Create: `db/migrations/0003_workspace_cleanup_jobs.sql`
- Modify: `services/api/internal/store/mysql.go`
- Create: `services/api/internal/test/workspace_cleanup_jobs_store_test.go`
- Modify: `services/api/internal/domain/practice_session.go` only if a cleanup-job domain type is needed

- [ ] **Step 1: Write the failing store insert/upsert test**

Add a focused store test proving the API can create a cleanup job and then update it for the same session instead of creating a duplicate row.

```go
func TestWorkspaceCleanupJobStoreUpsertsByPracticeSession(t *testing.T) {
	t.Parallel()

	store := newTestMySQLStore(t)
	sessionID := seedPracticeSession(t, store, seedPracticeSessionParams{
		userID:      7,
		scenarioID:  1,
		templateID:  1,
		runnerRef:   "ws-cleanup-upsert",
		workspace:   "/tmp/ws-cleanup-upsert",
		status:      "active",
	})

	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	later := now.Add(10 * time.Minute)

	if err := store.UpsertWorkspaceCleanupJob(context.Background(), domain.WorkspaceCleanupJob{
		PracticeSessionID: sessionID,
		WorkspaceID:       "ws-cleanup-upsert",
		Reason:            "expired",
		ScheduledAt:       now,
		Status:            "pending",
	}); err != nil {
		t.Fatalf("upsert cleanup job: %v", err)
	}

	if err := store.UpsertWorkspaceCleanupJob(context.Background(), domain.WorkspaceCleanupJob{
		PracticeSessionID: sessionID,
		WorkspaceID:       "ws-cleanup-upsert",
		Reason:            "orphaned",
		ScheduledAt:       later,
		Status:            "pending",
	}); err != nil {
		t.Fatalf("upsert cleanup job again: %v", err)
	}

	jobs, err := store.ListWorkspaceCleanupJobsForSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("list cleanup jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected one cleanup job, got %d", len(jobs))
	}
	if jobs[0].Reason != "orphaned" {
		t.Fatalf("expected updated reason orphaned, got %q", jobs[0].Reason)
	}
	if !jobs[0].ScheduledAt.Equal(later) {
		t.Fatalf("expected updated scheduled_at %v, got %v", later, jobs[0].ScheduledAt)
	}
}
```

- [ ] **Step 2: Run the insert/upsert test to verify it fails**

Run:

```bash
go test ./services/api/internal/test -run "TestWorkspaceCleanupJobStoreUpsertsByPracticeSession" -v
```

Expected:

```text
FAIL
```

The failure should be because the migration, domain type, or store methods do not exist yet.

- [ ] **Step 3: Write the failing claim-and-complete test**

Add a second store test proving due jobs can be claimed, then marked `succeeded`.

```go
func TestWorkspaceCleanupJobStoreClaimsDueJobsAndMarksSuccess(t *testing.T) {
	t.Parallel()

	store := newTestMySQLStore(t)
	sessionID := seedPracticeSession(t, store, seedPracticeSessionParams{
		userID:      9,
		scenarioID:  1,
		templateID:  1,
		runnerRef:   "ws-cleanup-claim",
		workspace:   "/tmp/ws-cleanup-claim",
		status:      "expired",
	})

	now := time.Date(2026, 5, 24, 13, 0, 0, 0, time.UTC)
	if err := store.UpsertWorkspaceCleanupJob(context.Background(), domain.WorkspaceCleanupJob{
		PracticeSessionID: sessionID,
		WorkspaceID:       "ws-cleanup-claim",
		Reason:            "expired",
		ScheduledAt:       now.Add(-time.Minute),
		Status:            "pending",
	}); err != nil {
		t.Fatalf("seed cleanup job: %v", err)
	}

	jobs, err := store.ClaimDueWorkspaceCleanupJobs(context.Background(), now, 10)
	if err != nil {
		t.Fatalf("claim due cleanup jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected one claimed job, got %d", len(jobs))
	}
	if jobs[0].Status != "running" {
		t.Fatalf("expected claimed job status running, got %q", jobs[0].Status)
	}

	if err := store.MarkWorkspaceCleanupJobSucceeded(context.Background(), jobs[0].ID); err != nil {
		t.Fatalf("mark cleanup success: %v", err)
	}

	reloaded, err := store.ListWorkspaceCleanupJobsForSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("reload cleanup jobs: %v", err)
	}
	if reloaded[0].Status != "succeeded" {
		t.Fatalf("expected succeeded cleanup job, got %q", reloaded[0].Status)
	}
	if reloaded[0].LastError != "" {
		t.Fatalf("expected last_error to be cleared, got %q", reloaded[0].LastError)
	}
}
```

- [ ] **Step 4: Run the claim-and-complete test to verify it fails**

Run:

```bash
go test ./services/api/internal/test -run "TestWorkspaceCleanupJobStoreClaimsDueJobsAndMarksSuccess" -v
```

Expected:

```text
FAIL
```

- [ ] **Step 5: Add the migration and minimal domain/store surface**

Create `db/migrations/0003_workspace_cleanup_jobs.sql`:

```sql
CREATE TABLE workspace_cleanup_jobs (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  practice_session_id BIGINT UNSIGNED NOT NULL,
  workspace_id VARCHAR(255) NOT NULL,
  reason VARCHAR(32) NOT NULL,
  scheduled_at DATETIME(6) NOT NULL,
  status VARCHAR(32) NOT NULL,
  attempt_count INT UNSIGNED NOT NULL DEFAULT 0,
  last_error TEXT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uq_workspace_cleanup_jobs_session (practice_session_id),
  KEY idx_workspace_cleanup_jobs_status_schedule (status, scheduled_at),
  CONSTRAINT fk_workspace_cleanup_jobs_session
    FOREIGN KEY (practice_session_id) REFERENCES practice_sessions(id)
    ON DELETE CASCADE
);
```

Add the domain type if needed:

```go
type WorkspaceCleanupJob struct {
	ID                uint64
	PracticeSessionID uint64
	WorkspaceID       string
	Reason            string
	ScheduledAt       time.Time
	Status            string
	AttemptCount      uint32
	LastError         string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
```

Add store methods in `services/api/internal/store/mysql.go`:

```go
func (s *MySQLPracticeSessionStore) UpsertWorkspaceCleanupJob(ctx context.Context, job domain.WorkspaceCleanupJob) error
func (s *MySQLPracticeSessionStore) ClaimDueWorkspaceCleanupJobs(ctx context.Context, now time.Time, limit int) ([]domain.WorkspaceCleanupJob, error)
func (s *MySQLPracticeSessionStore) MarkWorkspaceCleanupJobSucceeded(ctx context.Context, jobID uint64) error
func (s *MySQLPracticeSessionStore) MarkWorkspaceCleanupJobFailed(ctx context.Context, jobID uint64, scheduledAt time.Time, lastErr string) error
func (s *MySQLPracticeSessionStore) ListWorkspaceCleanupJobsForSession(ctx context.Context, sessionID uint64) ([]domain.WorkspaceCleanupJob, error)
```

Use straightforward SQL:

```sql
INSERT INTO workspace_cleanup_jobs (
  practice_session_id, workspace_id, reason, scheduled_at, status, attempt_count, last_error
) VALUES (?, ?, ?, ?, ?, 0, NULL)
ON DUPLICATE KEY UPDATE
  workspace_id = VALUES(workspace_id),
  reason = VALUES(reason),
  scheduled_at = VALUES(scheduled_at),
  status = VALUES(status),
  last_error = NULL,
  updated_at = CURRENT_TIMESTAMP(6);
```

For claiming, use a transaction:

1. select due rows ordered by `scheduled_at`, limited by `limit`
2. update those ids to `running`
3. return the claimed jobs with `status = running`

Notes for the engineer:
- Keep claim logic single-process-friendly but defensive.
- Do not try to solve distributed locking in this slice.
- `last_error` should round-trip as empty string when null for easy tests.

- [ ] **Step 6: Run the focused store tests to verify they pass**

Run:

```bash
go test ./services/api/internal/test -run "TestWorkspaceCleanupJobStoreUpsertsByPracticeSession|TestWorkspaceCleanupJobStoreClaimsDueJobsAndMarksSuccess" -v
```

Expected:

```text
PASS
```

- [ ] **Step 7: Commit the schema and store layer**

```bash
git add db/migrations/0003_workspace_cleanup_jobs.sql services/api/internal/domain/practice_session.go services/api/internal/store/mysql.go services/api/internal/test/workspace_cleanup_jobs_store_test.go
git commit -m "feat: add workspace cleanup jobs store"
```

---

### Task 2: Upsert cleanup jobs from expired and orphaned session transitions

**Files:**
- Modify: `services/api/internal/service/practice_service.go`
- Modify: `services/api/internal/test/practice_service_test.go`

- [ ] **Step 1: Write the failing expired-transition cleanup-job test**

Add a service test proving the expiry sweep creates a durable cleanup job instead of relying only on direct runner deletion attempts.

```go
func TestPracticeServiceExpireSweepUpsertsCleanupJob(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 24, 15, 0, 0, 0, time.UTC)
	store := newStubPracticeSessionStore()
	store.expireResults = []domain.PracticeSession{
		{
			ID:               41,
			UserID:           7,
			ScenarioID:       1,
			TemplateID:       1,
			RunnerRef:        "ws-sweep-cleanup",
			WorkspacePathRef: "/tmp/ws-sweep-cleanup",
			Status:           "expired",
		},
	}

	svc := service.NewPracticeService(store, &stubRunnerClient{}, func() time.Time { return now })

	expiredCount, err := svc.ExpireStalePracticeSessions(context.Background())
	if err != nil {
		t.Fatalf("expire stale sessions: %v", err)
	}
	if expiredCount != 1 {
		t.Fatalf("expected one expired session, got %d", expiredCount)
	}
	if len(store.upsertCleanupJobCalls) != 1 {
		t.Fatalf("expected one cleanup job upsert, got %d", len(store.upsertCleanupJobCalls))
	}
	job := store.upsertCleanupJobCalls[0]
	if job.PracticeSessionID != 41 {
		t.Fatalf("expected cleanup job for session 41, got %d", job.PracticeSessionID)
	}
	if job.WorkspaceID != "ws-sweep-cleanup" {
		t.Fatalf("expected workspace id ws-sweep-cleanup, got %q", job.WorkspaceID)
	}
	if job.Reason != "expired" {
		t.Fatalf("expected cleanup reason expired, got %q", job.Reason)
	}
	if !job.ScheduledAt.Equal(now) {
		t.Fatalf("expected immediate cleanup scheduling at %v, got %v", now, job.ScheduledAt)
	}
}
```

- [ ] **Step 2: Run the expired-transition test to verify it fails**

Run:

```bash
go test ./services/api/internal/test -run "TestPracticeServiceExpireSweepUpsertsCleanupJob" -v
```

Expected:

```text
FAIL
```

- [ ] **Step 3: Write the failing orphan-transition cleanup-job test**

Add a service test proving a missing workspace transition to `orphaned` upserts a delayed cleanup job.

```go
func TestPracticeServiceOrphanTransitionUpsertsDelayedCleanupJob(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 24, 16, 0, 0, 0, time.UTC)
	store := newStubPracticeSessionStore()
	store.sessionByID = domain.PracticeSession{
		ID:               52,
		UserID:           7,
		ScenarioID:       1,
		TemplateID:       1,
		RunnerRef:        "ws-orphan-cleanup",
		WorkspacePathRef: "/tmp/ws-orphan-cleanup",
		Status:           "active",
	}
	runnerClient := &stubRunnerClient{connectTerminalErr: runner.ErrWorkspaceNotFound}

	svc := service.NewPracticeService(store, runnerClient, func() time.Time { return now })

	_, err := svc.ConnectTerminal(context.Background(), 7, 52)
	if !errors.Is(err, service.ErrPracticeSessionOrphaned) {
		t.Fatalf("expected orphaned session error, got %v", err)
	}
	if len(store.upsertCleanupJobCalls) != 1 {
		t.Fatalf("expected one cleanup job upsert, got %d", len(store.upsertCleanupJobCalls))
	}
	job := store.upsertCleanupJobCalls[0]
	if job.Reason != "orphaned" {
		t.Fatalf("expected orphaned cleanup job, got %q", job.Reason)
	}
	if !job.ScheduledAt.Equal(now.Add(10 * time.Minute)) {
		t.Fatalf("expected orphan cleanup at %v, got %v", now.Add(10*time.Minute), job.ScheduledAt)
	}
}
```

- [ ] **Step 4: Run the orphan-transition test to verify it fails**

Run:

```bash
go test ./services/api/internal/test -run "TestPracticeServiceOrphanTransitionUpsertsDelayedCleanupJob" -v
```

Expected:

```text
FAIL
```

- [ ] **Step 5: Replace in-memory cleanup scheduling with cleanup-job upserts**

Update the store interface in `services/api/internal/service/practice_service.go`:

```go
type PracticeSessionStore interface {
	CreatePracticeSession(ctx context.Context, session domain.PracticeSession) (domain.PracticeSession, error)
	CurrentPracticeSession(ctx context.Context, userID uint64) (domain.PracticeSession, error)
	PracticeSessionByID(ctx context.Context, sessionID uint64) (domain.PracticeSession, error)
	UpdatePracticeSession(ctx context.Context, session domain.PracticeSession) (domain.PracticeSession, error)
	ExpirePracticeSessions(ctx context.Context, before time.Time, endedAt time.Time) ([]domain.PracticeSession, error)
	UpsertWorkspaceCleanupJob(ctx context.Context, job domain.WorkspaceCleanupJob) error
}
```

Add a helper:

```go
func (s *practiceService) upsertWorkspaceCleanupJob(
	ctx context.Context,
	session domain.PracticeSession,
	reason string,
	scheduledAt time.Time,
) {
	if s.store == nil {
		return
	}

	job := domain.WorkspaceCleanupJob{
		PracticeSessionID: session.ID,
		WorkspaceID:       session.RunnerRef,
		Reason:            reason,
		ScheduledAt:       scheduledAt.UTC(),
		Status:            "pending",
	}
	if err := s.store.UpsertWorkspaceCleanupJob(ctx, job); err != nil {
		log.Printf("practice cleanup job upsert failed for session %d: %v", session.ID, err)
	}
}
```

Use it from:

- `ExpireStalePracticeSessions()` after sessions transition to `expired`
- `orphanSession()` after the session is updated to `orphaned`

Notes for the engineer:
- Do not remove runner delete support from the client yet; Task 3 will use it from the worker.
- Remove or stop using the old in-memory `cleanupMu` / `cleanup` retry state once cleanup intent is persisted.

- [ ] **Step 6: Run the focused service tests to verify they pass**

Run:

```bash
go test ./services/api/internal/test -run "TestPracticeServiceExpireSweepUpsertsCleanupJob|TestPracticeServiceOrphanTransitionUpsertsDelayedCleanupJob" -v
```

Expected:

```text
PASS
```

- [ ] **Step 7: Commit the lifecycle-to-job upsert behavior**

```bash
git add services/api/internal/service/practice_service.go services/api/internal/test/practice_service_test.go
git commit -m "feat: persist cleanup intent for session lifecycle"
```

---

### Task 3: Execute cleanup jobs from a background worker with retry scheduling

**Files:**
- Modify: `services/api/internal/service/practice_service.go`
- Modify: `services/api/internal/service/practice_session_reaper.go`
- Modify: `services/api/internal/test/practice_service_test.go`
- Modify: `services/api/internal/test/practice_session_reaper_test.go` if startup-loop coverage belongs there

- [ ] **Step 1: Write the failing worker success test**

Add a service-level test proving a due cleanup job calls runner delete and marks itself succeeded.

```go
func TestPracticeServiceRunWorkspaceCleanupDueJobsMarksSuccess(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 24, 17, 0, 0, 0, time.UTC)
	store := newStubPracticeSessionStore()
	store.claimedCleanupJobs = []domain.WorkspaceCleanupJob{
		{
			ID:                7,
			PracticeSessionID: 41,
			WorkspaceID:       "ws-cleanup-run",
			Reason:            "expired",
			ScheduledAt:       now.Add(-time.Minute),
			Status:            "running",
		},
	}
	runnerClient := &stubRunnerClient{}
	svc := service.NewPracticeService(store, runnerClient, func() time.Time { return now })

	if err := svc.RunWorkspaceCleanupDueJobs(context.Background(), 10); err != nil {
		t.Fatalf("run workspace cleanup jobs: %v", err)
	}
	if runnerClient.deleteWorkspaceCalls != 1 {
		t.Fatalf("expected one delete workspace call, got %d", runnerClient.deleteWorkspaceCalls)
	}
	if len(store.markCleanupSucceededCalls) != 1 || store.markCleanupSucceededCalls[0] != 7 {
		t.Fatalf("expected cleanup job 7 to be marked succeeded, got %v", store.markCleanupSucceededCalls)
	}
}
```

- [ ] **Step 2: Run the worker success test to verify it fails**

Run:

```bash
go test ./services/api/internal/test -run "TestPracticeServiceRunWorkspaceCleanupDueJobsMarksSuccess" -v
```

Expected:

```text
FAIL
```

- [ ] **Step 3: Write the failing worker retry test**

Add a second test proving runner failure marks the job failed and reschedules it.

```go
func TestPracticeServiceRunWorkspaceCleanupDueJobsReschedulesFailure(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 24, 18, 0, 0, 0, time.UTC)
	store := newStubPracticeSessionStore()
	store.claimedCleanupJobs = []domain.WorkspaceCleanupJob{
		{
			ID:                8,
			PracticeSessionID: 52,
			WorkspaceID:       "ws-cleanup-fail",
			Reason:            "orphaned",
			ScheduledAt:       now.Add(-time.Minute),
			Status:            "running",
			AttemptCount:      0,
		},
	}
	runnerClient := &stubRunnerClient{
		deleteWorkspaceErr: errors.New("runner unavailable"),
	}
	svc := service.NewPracticeService(store, runnerClient, func() time.Time { return now })

	if err := svc.RunWorkspaceCleanupDueJobs(context.Background(), 10); err != nil {
		t.Fatalf("run workspace cleanup jobs: %v", err)
	}
	if len(store.markCleanupFailedCalls) != 1 {
		t.Fatalf("expected one failed cleanup mark, got %d", len(store.markCleanupFailedCalls))
	}
	failure := store.markCleanupFailedCalls[0]
	if failure.jobID != 8 {
		t.Fatalf("expected failed cleanup mark for job 8, got %d", failure.jobID)
	}
	if failure.lastError == "" {
		t.Fatal("expected failed cleanup to record last error")
	}
	if !failure.scheduledAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("expected first retry at %v, got %v", now.Add(time.Minute), failure.scheduledAt)
	}
}
```

- [ ] **Step 4: Run the worker retry test to verify it fails**

Run:

```bash
go test ./services/api/internal/test -run "TestPracticeServiceRunWorkspaceCleanupDueJobsReschedulesFailure" -v
```

Expected:

```text
FAIL
```

- [ ] **Step 5: Implement the cleanup worker and retry helper**

Extend the store interface:

```go
type PracticeSessionStore interface {
	// existing methods...
	ClaimDueWorkspaceCleanupJobs(ctx context.Context, now time.Time, limit int) ([]domain.WorkspaceCleanupJob, error)
	MarkWorkspaceCleanupJobSucceeded(ctx context.Context, jobID uint64) error
	MarkWorkspaceCleanupJobFailed(ctx context.Context, jobID uint64, scheduledAt time.Time, lastErr string) error
}
```

Add worker methods in `services/api/internal/service/practice_service.go`:

```go
func (s *practiceService) RunWorkspaceCleanupDueJobs(ctx context.Context, limit int) error {
	if s.store == nil || s.runner == nil {
		return nil
	}
	if limit <= 0 {
		limit = 10
	}

	jobs, err := s.store.ClaimDueWorkspaceCleanupJobs(ctx, s.now().UTC(), limit)
	if err != nil {
		return fmt.Errorf("claim due cleanup jobs: %w", err)
	}

	for _, job := range jobs {
		err := s.runner.DeleteWorkspace(ctx, job.WorkspaceID, job.Reason, 0)
		if err == nil || errors.Is(err, runner.ErrWorkspaceNotFound) {
			if markErr := s.store.MarkWorkspaceCleanupJobSucceeded(ctx, job.ID); markErr != nil {
				log.Printf("mark cleanup job %d succeeded: %v", job.ID, markErr)
			}
			continue
		}

		nextRun := nextWorkspaceCleanupRetryAt(s.now().UTC(), job.AttemptCount+1)
		if markErr := s.store.MarkWorkspaceCleanupJobFailed(ctx, job.ID, nextRun, err.Error()); markErr != nil {
			log.Printf("mark cleanup job %d failed: %v", job.ID, markErr)
		}
	}

	return nil
}

func nextWorkspaceCleanupRetryAt(now time.Time, attempt uint32) time.Time {
	switch attempt {
	case 1:
		return now.Add(1 * time.Minute)
	case 2:
		return now.Add(5 * time.Minute)
	default:
		return now.Add(15 * time.Minute)
	}
}
```

Start the loop in `services/api/internal/service/practice_session_reaper.go`:

```go
func StartWorkspaceCleanupLoop(ctx context.Context, logger *log.Logger, svc PracticeService, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}

	runSweep := func() {
		if err := svc.RunWorkspaceCleanupDueJobs(ctx, 20); err != nil && logger != nil {
			logger.Printf("workspace cleanup sweep failed: %v", err)
		}
	}

	runSweep()

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runSweep()
			}
		}
	}()
}
```

Notes for the engineer:
- Keep this worker independent from expiry transitions; it consumes durable jobs only.
- Reuse the existing runner delete contract with `deleteAfter = 0` because scheduling now lives in the API job table.

- [ ] **Step 6: Run the focused cleanup-worker tests to verify they pass**

Run:

```bash
go test ./services/api/internal/test -run "TestPracticeServiceRunWorkspaceCleanupDueJobsMarksSuccess|TestPracticeServiceRunWorkspaceCleanupDueJobsReschedulesFailure" -v
```

Expected:

```text
PASS
```

- [ ] **Step 7: Commit the cleanup worker**

```bash
git add services/api/internal/service/practice_service.go services/api/internal/service/practice_session_reaper.go services/api/internal/test/practice_service_test.go services/api/internal/test/practice_session_reaper_test.go
git commit -m "feat: execute workspace cleanup jobs"
```

---

### Task 4: Tighten idempotency and `workspace not found` semantics

**Files:**
- Modify: `services/api/internal/test/practice_service_test.go`
- Modify: `services/api/internal/store/mysql.go`

- [ ] **Step 1: Write the failing `workspace not found` success test**

Add a test proving runner `ErrWorkspaceNotFound` marks the job `succeeded` instead of retrying.

```go
func TestPracticeServiceRunWorkspaceCleanupDueJobsTreatsWorkspaceNotFoundAsSuccess(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 24, 19, 0, 0, 0, time.UTC)
	store := newStubPracticeSessionStore()
	store.claimedCleanupJobs = []domain.WorkspaceCleanupJob{
		{
			ID:                9,
			PracticeSessionID: 61,
			WorkspaceID:       "ws-cleanup-missing",
			Reason:            "expired",
			ScheduledAt:       now.Add(-time.Minute),
			Status:            "running",
		},
	}
	runnerClient := &stubRunnerClient{
		deleteWorkspaceErr: runner.ErrWorkspaceNotFound,
	}
	svc := service.NewPracticeService(store, runnerClient, func() time.Time { return now })

	if err := svc.RunWorkspaceCleanupDueJobs(context.Background(), 10); err != nil {
		t.Fatalf("run workspace cleanup jobs: %v", err)
	}
	if len(store.markCleanupSucceededCalls) != 1 || store.markCleanupSucceededCalls[0] != 9 {
		t.Fatalf("expected cleanup job 9 to be marked succeeded, got %v", store.markCleanupSucceededCalls)
	}
	if len(store.markCleanupFailedCalls) != 0 {
		t.Fatalf("expected no failed cleanup marks, got %d", len(store.markCleanupFailedCalls))
	}
}
```

- [ ] **Step 2: Run the `workspace not found` test to verify it fails**

Run:

```bash
go test ./services/api/internal/test -run "TestPracticeServiceRunWorkspaceCleanupDueJobsTreatsWorkspaceNotFoundAsSuccess" -v
```

Expected:

```text
FAIL
```

- [ ] **Step 3: Write the failing claim-idempotency test**

Add a store test proving `running` jobs are not reclaimed on the next worker tick.

```go
func TestWorkspaceCleanupJobStoreDoesNotReclaimRunningJobs(t *testing.T) {
	t.Parallel()

	store := newTestMySQLStore(t)
	sessionID := seedPracticeSession(t, store, seedPracticeSessionParams{
		userID:      11,
		scenarioID:  1,
		templateID:  1,
		runnerRef:   "ws-running-job",
		workspace:   "/tmp/ws-running-job",
		status:      "orphaned",
	})
	now := time.Date(2026, 5, 24, 20, 0, 0, 0, time.UTC)

	if err := store.UpsertWorkspaceCleanupJob(context.Background(), domain.WorkspaceCleanupJob{
		PracticeSessionID: sessionID,
		WorkspaceID:       "ws-running-job",
		Reason:            "orphaned",
		ScheduledAt:       now.Add(-time.Minute),
		Status:            "pending",
	}); err != nil {
		t.Fatalf("seed cleanup job: %v", err)
	}

	jobs, err := store.ClaimDueWorkspaceCleanupJobs(context.Background(), now, 10)
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected first claim to return one job, got %d", len(jobs))
	}

	jobs, err = store.ClaimDueWorkspaceCleanupJobs(context.Background(), now, 10)
	if err != nil {
		t.Fatalf("second claim: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("expected running job not to be reclaimed, got %d jobs", len(jobs))
	}
}
```

- [ ] **Step 4: Run the idempotency test to verify it fails**

Run:

```bash
go test ./services/api/internal/test -run "TestWorkspaceCleanupJobStoreDoesNotReclaimRunningJobs" -v
```

Expected:

```text
FAIL
```

- [ ] **Step 5: Tighten success semantics and claim filtering**

Make sure:

- `ClaimDueWorkspaceCleanupJobs` only selects `pending` and retryable `failed` rows
- `running` rows are skipped
- worker success path explicitly treats `runner.ErrWorkspaceNotFound` as success

If needed, tighten the select query to:

```sql
SELECT id, practice_session_id, workspace_id, reason, scheduled_at, status, attempt_count, COALESCE(last_error, ''), created_at, updated_at
FROM workspace_cleanup_jobs
WHERE (status = 'pending' OR status = 'failed')
  AND scheduled_at <= ?
ORDER BY scheduled_at ASC, id ASC
LIMIT ?
```

- [ ] **Step 6: Run the focused idempotency semantics tests**

Run:

```bash
go test ./services/api/internal/test -run "TestPracticeServiceRunWorkspaceCleanupDueJobsTreatsWorkspaceNotFoundAsSuccess|TestWorkspaceCleanupJobStoreDoesNotReclaimRunningJobs" -v
```

Expected:

```text
PASS
```

- [ ] **Step 7: Commit the cleanup-job execution hardening**

```bash
git add services/api/internal/store/mysql.go services/api/internal/test/workspace_cleanup_jobs_store_test.go services/api/internal/test/practice_service_test.go
git commit -m "fix: harden cleanup job execution semantics"
```

---

### Task 5: Run the backend verification set and clean up old assumptions

**Files:**
- Modify: `services/api/internal/test/practice_service_test.go`
- Modify: `services/api/internal/test/practice_session_reaper_test.go` if needed
- Modify: any stale helper/stub definitions touched by the new store interface

- [ ] **Step 1: Update test doubles to match the new interfaces**

Bring the existing service/reaper test doubles up to date:

- add cleanup-job store call capture fields
- remove assertions that expect direct best-effort cleanup retries from repeated expiry/orphan checks
- keep `DeleteWorkspace` on the runner stub for worker execution tests

Example additions:

```go
type stubPracticeSessionStore struct {
	// existing fields...
	upsertCleanupJobCalls    []domain.WorkspaceCleanupJob
	claimedCleanupJobs       []domain.WorkspaceCleanupJob
	markCleanupSucceededCalls []uint64
	markCleanupFailedCalls   []cleanupFailureMark
}

type cleanupFailureMark struct {
	jobID       uint64
	scheduledAt time.Time
	lastError   string
}
```

- [ ] **Step 2: Run the full API test suite to expose remaining gaps**

Run:

```bash
go test ./services/api/... 
```

Expected:

```text
FAIL
```

Use the failures to remove stale best-effort assumptions and finish any missing interface updates.

- [ ] **Step 3: Make the minimal test and helper adjustments**

Adjust old tests so they now expect:

- lifecycle transitions upsert cleanup jobs
- cleanup worker executes retries
- repeated expiry/orphan checks do not need to re-drive direct runner delete inline

Do not rewrite unrelated tests.

- [ ] **Step 4: Re-run full backend verification**

Run:

```bash
go test ./services/api/...
go test ./services/runner/...
```

Expected:

```text
PASS
PASS
```

Notes for the engineer:
- runner tests should remain green without production changes in this slice.
- If runner tests fail, stop and verify whether the API plan accidentally leaked into runner assumptions.

- [ ] **Step 5: Commit the finished cleanup-jobs slice**

```bash
git add services/api/internal/test/practice_service_test.go services/api/internal/test/practice_session_reaper_test.go services/api/internal/test/workspace_cleanup_jobs_store_test.go services/api/internal/service/practice_service.go services/api/internal/service/practice_session_reaper.go services/api/internal/store/mysql.go db/migrations/0003_workspace_cleanup_jobs.sql
git commit -m "test: finish workspace cleanup jobs coverage"
```
