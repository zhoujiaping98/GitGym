# Workspace Cleanup Retry Policy Design

Date: 2026-05-27

## Goal

Make workspace cleanup execution semantics explicit after persistent cleanup jobs already exist:

- retry transient cleanup failures a bounded number of times
- stop retrying once a job has exhausted automatic recovery
- keep that distinction observable from the existing job row without expanding the schema

This slice builds directly on the cleanup-jobs foundation already on `main`.

## Non-Goals

- No frontend or operator UI
- No new cleanup table or migration
- No runner protocol changes
- No manual replay endpoint
- No reconciliation sweep for missing or inconsistent jobs
- No broader session lifecycle refactor

## Current Problem

Cleanup jobs are now durable, but failure semantics are still too loose:

- the worker always computes another retry delay on failure
- `failed` currently means both "will retry later" and "has effectively stopped being useful"
- store claim logic does not have an explicit retry ceiling

That leaves two practical gaps:

- a permanently broken cleanup target can retry forever
- later reconciliation work cannot cleanly distinguish retryable failures from exhausted ones

## Recommended Approach

Keep the existing job shape and status vocabulary:

- `pending`
- `running`
- `succeeded`
- `failed`

Then add code-level retry policy:

- `workspaceCleanupMaxAttempts = 5`
- jobs are only claimable while `attempt_count < workspaceCleanupMaxAttempts`
- failed jobs with `attempt_count >= workspaceCleanupMaxAttempts` become exhausted terminal failures

This preserves the current schema and keeps the exhausted signal simple:

- `status = failed`
- `attempt_count >= max attempts`

## Alternatives Considered

### 1. Add new statuses such as `retryable_failed` and `exhausted`

Rejected for this slice.

It is more explicit, but it would widen store queries, tests, and state transitions for limited additional value right now.

### 2. Add a schema field such as `terminal_failed_at`

Rejected.

Useful later for operations, but unnecessary before there is any operator-facing visibility or reconciliation sweep.

### 3. Keep infinite retries

Rejected.

This leaves the system vulnerable to permanent background noise and hides the difference between a transient failure and a dead cleanup path.

## Execution Semantics

Add two new constants in the practice service layer:

- `WorkspaceCleanupJobLeaseTimeout = 15 minutes`
- `workspaceCleanupMaxAttempts = 5`

Claim semantics become:

- `pending` jobs are claimable when `scheduled_at <= now`
- `failed` jobs are claimable when `scheduled_at <= now` and `attempt_count < workspaceCleanupMaxAttempts`
- `running` jobs are reclaimable only when their lease is stale and `attempt_count < workspaceCleanupMaxAttempts`
- exhausted failed jobs are never claimed again

Because claiming already increments `attempt_count`, the fifth claim is the final automatic attempt.

## Failure Handling

When runner delete fails on attempts one through four:

- mark the job `failed`
- keep the failure in `last_error`
- schedule the next retry using the existing backoff ladder:
  - attempt 1 failure -> `+1 minute`
  - attempt 2 failure -> `+5 minutes`
  - attempt 3 failure -> `+15 minutes`
  - attempt 4 failure -> `+15 minutes`

When runner delete fails on attempt five:

- mark the job `failed`
- keep the failure in `last_error`
- do not schedule a meaningful next retry
- leave the row in terminal exhausted state via `attempt_count = 5`

For this slice, setting `scheduled_at = now` on exhausted failure is acceptable because claim gating is driven by `attempt_count < workspaceCleanupMaxAttempts`.

## Success Handling

Success semantics do not change:

- runner delete success marks the job `succeeded`
- `runner.ErrWorkspaceNotFound` also marks the job `succeeded`
- success clears `last_error`

## Store Boundary

No new store methods are needed.

Existing methods keep the same signatures:

- `ClaimDueWorkspaceCleanupJobs`
- `MarkWorkspaceCleanupJobSucceeded`
- `MarkWorkspaceCleanupJobFailed`
- `UpsertWorkspaceCleanupJob`

Only their selection rules and service-side scheduling decisions change.

Both store implementations must align:

- MySQL store
- in-memory fallback store used by tests and no-DB modes

## Worker Boundary

`RunWorkspaceCleanupDueJobs` remains the single execution path.

It should:

1. claim due jobs under the new eligibility rules
2. call runner delete
3. on failure, branch between:
   - retryable failure
   - exhausted terminal failure
4. continue returning aggregated execution errors so the loop logger still surfaces failures

Error text should make exhaustion obvious, for example by including the attempt count and that retries are exhausted.

## Testing

Add or update tests at three levels.

### Service tests

Cover:

- retryable failure still reschedules on early attempts
- fifth-attempt failure is marked failed without another retry window
- exhausted failed jobs are not silently treated as success

### Store tests

Cover:

- failed jobs below max attempts are still reclaimable when due
- failed jobs at max attempts are not reclaimable
- stale running jobs below max attempts are reclaimable
- stale running jobs at max attempts are not reclaimable

### In-memory parity tests

Cover the same exhausted-claim rules in the in-memory store path so fallback behavior matches MySQL semantics.

## Expected Outcome

After this slice:

- cleanup jobs still recover from transient failures automatically
- permanently failing jobs stop retrying after five attempts
- exhausted failures are visible from existing job fields
- later reconciliation or operator slices can build on that stable meaning without revisiting worker semantics
