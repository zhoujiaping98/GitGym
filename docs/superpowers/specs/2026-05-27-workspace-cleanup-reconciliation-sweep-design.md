# Workspace Cleanup Reconciliation Sweep Design

Date: 2026-05-27

## Goal

Close the last major reliability gap in workspace cleanup by adding a periodic reconciliation sweep that:

- backfills missing cleanup jobs for sessions already known to need cleanup
- surfaces exhausted cleanup failures through minimal operator-visible signals

This slice builds on the cleanup job foundation already on `main`:

- durable `workspace_cleanup_jobs`
- background cleanup execution loop
- bounded retry policy with exhausted failure semantics

## Non-Goals

- No frontend or admin UI
- No new database tables or migration
- No runner-side changes
- No automatic reset of exhausted failed jobs
- No generic lifecycle reconciliation across unrelated session states
- No metrics system or external alerting integration

## Current Problem

Cleanup execution is now durable and bounded, but two gaps remain:

1. a session can already be `expired` or `orphaned` while still missing a cleanup job
2. exhausted failed jobs can sit silently in the database without any periodic signal

That means cleanup intent can still be absent for edge cases, and operators have no lightweight way to notice terminal failures short of manual inspection.

## Recommended Approach

Extend the existing workspace cleanup sweep so each tick performs two phases:

1. execute due cleanup jobs
2. reconcile cleanup state

The reconciliation phase should:

- find `expired` or `orphaned` sessions that have no cleanup job row
- create the missing cleanup jobs using existing job semantics
- count exhausted failed jobs and return a summary for logging

This keeps responsibility boundaries clear:

- session rows remain lifecycle truth
- cleanup jobs remain execution truth
- reconciliation only repairs missing intent and reports exhausted failures

## Alternatives Considered

### 1. Separate reconciliation loop

Rejected.

It would add another background loop and another cadence without any real benefit. Reconciliation belongs beside cleanup execution.

### 2. Automatically reset exhausted jobs back to `pending`

Rejected.

That would undo the bounded retry semantics we just established and blur the meaning of exhausted failure.

### 3. Build an operator UI first

Rejected.

Useful later, but the missing piece right now is backend correctness plus minimal visibility, not a dashboard.

## Store Boundary

Add two store queries:

- `ListPracticeSessionsMissingWorkspaceCleanupJob(ctx, limit)`
- `ListExhaustedWorkspaceCleanupJobs(ctx, limit)`

### Missing-job query

Return sessions where:

- `status IN ('expired', 'orphaned')`
- no `workspace_cleanup_jobs` row exists for `practice_session_id`

This should return enough session fields to build the cleanup job:

- `id`
- `runner_ref`
- `status`
- `ended_at`

The query should be ordered by session id ascending for deterministic tests.

### Exhausted-job query

Return cleanup jobs where:

- `status = 'failed'`
- `attempt_count >= WorkspaceCleanupJobMaxAttempts`

This is a reporting query only. It does not mutate any state.

It should be ordered by id ascending and capped by the provided limit.

Both queries must exist in:

- MySQL store
- in-memory practice session store

## Service Boundary

Add a reconciliation method to `PracticeService`:

- `ReconcileWorkspaceCleanupJobs(ctx context.Context, limit int) (WorkspaceCleanupReconciliationSummary, error)`

Add a matching store-facing implementation on `practiceService`.

## Reconciliation Summary

Add a small service-level summary type:

- `BackfilledJobs int`
- `ExhaustedFailedJobs int`

This keeps loop logging simple without introducing a metrics subsystem.

## Backfill Rules

### Expired session with no job

Create a `pending` cleanup job with:

- `reason = expired`
- `scheduled_at = now`

### Orphaned session with no job

Create a `pending` cleanup job with:

- `reason = orphaned`
- `scheduled_at = max(now, ended_at + practiceSessionOrphanCleanupGrace)`

This preserves the original orphan cleanup grace window when the session was just orphaned, but still cleans up immediately if the grace window has already passed.

If `ended_at` is missing on an orphaned session, fall back to `scheduled_at = now`.

### Anything else

Do not invent jobs for:

- `active` sessions
- sessions that already have any cleanup job row
- `succeeded` or `failed` jobs that already exist

This slice only repairs missing intent, not incorrect existing rows.

## Exhausted Failure Reporting

Reconciliation must not requeue exhausted jobs.

Instead it should:

- count exhausted failed jobs up to the configured limit
- return that count in the reconciliation summary

The loop logger should emit a summary log when either:

- one or more jobs were backfilled
- one or more exhausted failed jobs were observed

Recommended log shape:

- `workspace cleanup reconciliation backfilled <n> jobs`
- `workspace cleanup reconciliation found <n> exhausted failed jobs`

Plain `log.Printf` is sufficient for this slice.

## Loop Integration

Keep a single `StartWorkspaceCleanupLoop`.

Each sweep tick should:

1. run `RunWorkspaceCleanupDueJobs`
2. run `ReconcileWorkspaceCleanupJobs`
3. log any execution failure
4. log any reconciliation failure
5. log non-zero reconciliation summary values

Execution failure should not block reconciliation in the same tick.

## Testing

Add coverage at three levels.

### Store tests

Cover:

- MySQL query returns missing `expired` and `orphaned` sessions only
- MySQL query returns exhausted failed jobs only

These tests may skip locally when MySQL is unavailable, consistent with existing store integration tests.

### Service tests

Cover:

- backfill of a missing `expired` cleanup job schedules immediately
- backfill of a missing `orphaned` cleanup job preserves remaining grace when `ended_at + grace > now`
- exhausted failed jobs contribute to the reconciliation summary without being mutated

### Loop tests

Cover:

- `StartWorkspaceCleanupLoop` runs both execution and reconciliation immediately and on interval
- reconciliation summary logging does not require a failure path

## Expected Outcome

After this slice:

- sessions that need cleanup but somehow lack a cleanup job will be repaired automatically
- exhausted cleanup failures remain terminal, but they are no longer silent
- workspace cleanup has a practical backend closure loop without needing a new UI or schema
