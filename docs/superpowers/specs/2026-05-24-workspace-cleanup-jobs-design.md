# Workspace Cleanup Jobs Design

Date: 2026-05-24

## Goal

Make workspace cleanup intent durable and retryable so abandoned workspaces are still cleaned up even when the first API-to-runner delete attempt fails.

This slice builds on the existing cleanup foundation:

- runner already exposes workspace delete semantics
- API already knows when sessions become `expired` or `orphaned`
- API already has lifecycle sweeps and orphan transition paths

The missing piece is durable cleanup intent and execution state.

## Non-Goals

- No frontend or user-facing cleanup UI
- No runner-side durable scheduler
- No admin observability dashboard
- No configurable cleanup policy
- No cleanup audit history beyond the minimum job execution fields
- No redesign of runner delete behavior

## Current Problem

Current cleanup behavior is still best-effort:

- session lifecycle transitions are correct
- runner delete is idempotent
- but failed cleanup scheduling depends on later sweeps or repeated orphan checks happening to retry it

That leaves a reliability gap:

- transient API/runner failures can delay cleanup indefinitely
- cleanup intent is not stored independently of the lifecycle mutation that created it
- the system cannot clearly distinguish "session is orphaned" from "workspace cleanup is still pending"

## Recommended Approach

Add a persistent API-owned `workspace_cleanup_jobs` table and a background cleanup worker that claims due jobs and calls the existing runner delete endpoint.

This keeps boundaries clean:

- practice session rows remain the source of lifecycle truth
- cleanup jobs become the source of cleanup intent and execution progress
- runner remains responsible only for deleting workspaces when asked

The key design shift is:

- lifecycle transitions create or update cleanup jobs transactionally
- a dedicated cleanup loop executes those jobs later and retries failures

## Alternatives Considered

### 1. Store cleanup scheduling fields directly on `practice_sessions`

Rejected.

This mixes two different concerns into one row:

- lifecycle state of the practice session
- execution state of background workspace cleanup

That becomes awkward as soon as retries, attempt counts, and last error details are needed.

### 2. Make runner persist delayed cleanup jobs

Rejected for this slice.

Runner should stay operationally simple. Making it the durable owner of cleanup intent would reduce API visibility and introduce a new stateful subsystem before it is needed.

### 3. Keep retry best-effort and rely on repeated lifecycle sweeps

Rejected.

This is the current gap. It does not provide durable cleanup intent or a clear retry model.

## Data Model

Add a new table:

- `workspace_cleanup_jobs`

Minimum columns:

- `id`
- `practice_session_id`
- `workspace_id`
- `reason`
- `scheduled_at`
- `status`
- `attempt_count`
- `last_error`
- `created_at`
- `updated_at`

### Field Semantics

- `practice_session_id`
  - identifies the session whose workspace should be cleaned up
- `workspace_id`
  - runner workspace identifier used by the delete route
- `reason`
  - first slice supports:
    - `expired`
    - `orphaned`
- `scheduled_at`
  - when the next cleanup attempt should run
- `status`
  - one of:
    - `pending`
    - `running`
    - `succeeded`
    - `failed`
- `attempt_count`
  - increments each time a claimed job actually attempts runner deletion
- `last_error`
  - nullable summary string from the most recent failed attempt

### Uniqueness

There should be at most one active cleanup job per practice session.

Practical rule for this slice:

- upsert by `practice_session_id`

This keeps duplicate lifecycle events from producing competing cleanup jobs for the same workspace.

## Job Creation Rules

Create or update cleanup jobs in the same transactional unit as the lifecycle transition when possible.

### Expired Session

When a session transitions to `expired`:

- upsert cleanup job
- set:
  - `reason = expired`
  - `scheduled_at = now`
  - `status = pending`
  - `last_error = null`

### Orphaned Session

When a session transitions to `orphaned`:

- upsert cleanup job
- set:
  - `reason = orphaned`
  - `scheduled_at = now + 10 minutes`
  - `status = pending`
  - `last_error = null`

### Repeated Transitions

If a matching cleanup job already exists and is not yet `succeeded`:

- update it rather than inserting a second row
- preserve the latest intended reason and schedule

If the job is already `succeeded`, no new job is needed unless the system later supports a distinct recreated workspace lifecycle, which is out of scope here.

## Execution Model

Add a background API cleanup loop dedicated to workspace cleanup jobs.

### Claiming

The cleanup worker should:

1. ask the store for due jobs where:
   - `status = pending`
   - or `status = failed` and `scheduled_at <= now`
2. claim a batch by marking them `running`
3. process each claimed job by calling runner delete

For this slice, a single-process API worker assumption is acceptable, but the claim update should still be written defensively so repeated loop ticks do not double-claim the same row.

### Success

Treat these outcomes as success:

- runner delete succeeds
- runner returns `workspace not found`

On success:

- set `status = succeeded`
- keep `attempt_count`
- clear `last_error`
- update `updated_at`

### Failure

When runner delete fails for any other reason:

- increment `attempt_count`
- set `status = failed`
- store `last_error`
- reschedule by setting a later `scheduled_at`

## Retry Policy

Use fixed code-level retry delays for the first slice.

Recommended schedule:

- first retry: `+1 minute`
- second retry: `+5 minutes`
- third and later retries: `+15 minutes`

This avoids hot-loop retries while keeping the behavior simple.

No dead-letter queue is needed in this slice.

## API Boundary

### Practice Service

Practice session lifecycle methods stay responsible for deciding when a session is:

- `expired`
- `orphaned`

They are no longer responsible for cleanup success inline.

Their new responsibility is:

- create or update cleanup intent when the lifecycle transition is committed

### Store

Add store methods for:

- upserting cleanup jobs
- claiming due cleanup jobs
- marking cleanup success
- marking cleanup failure with retry scheduling

The store becomes the durable boundary for cleanup execution state.

### Runner Client

Reuse the existing runner delete contract.

No new runner API behavior is needed here beyond what cleanup already uses:

- delete workspace by id
- tolerate `workspace not found`

## Runner Boundary

Runner remains unchanged in product semantics:

- it deletes workspaces when asked
- it treats missing workspaces idempotently
- it does not become the durable owner of pending cleanup work

This preserves the current service separation.

## Data Flow

### Expiry Path

1. expiry loop finds stale `active` session
2. session store transitions it to `expired`
3. API upserts cleanup job scheduled for `now`
4. cleanup worker claims the due job
5. cleanup worker calls runner delete
6. job becomes `succeeded` or `failed`

### Orphan Path

1. terminal attach, repo lookup, or reset discovers missing workspace
2. session store transitions it to `orphaned`
3. API upserts cleanup job scheduled for `now + 10m`
4. cleanup worker claims the job after grace period
5. cleanup worker calls runner delete
6. job becomes `succeeded` or `failed`

## Error Handling

- session lifecycle transitions must remain committed even if later cleanup execution fails
- runner `workspace not found` is treated as cleanup success
- failed jobs keep retry metadata rather than disappearing
- duplicate transition events should collapse into one durable cleanup job via upsert

## Testing

### Migration / Store Tests

Add coverage for:

- inserting a new cleanup job
- upserting an existing cleanup job for the same session
- claiming due jobs
- marking success
- marking failure and rescheduling

### Service Tests

Add coverage for:

- `expired` transition creates or updates an immediate cleanup job
- `orphaned` transition creates or updates a delayed cleanup job
- repeated orphan/expiry checks do not create duplicate active jobs

### Cleanup Worker Tests

Add coverage for:

- due pending job calls runner delete and marks `succeeded`
- runner `workspace not found` marks cleanup `succeeded`
- runner failure marks cleanup `failed`, increments attempt count, and reschedules
- a later worker tick retries the failed job after `scheduled_at`

### No Frontend Tests

This slice is backend-only.

## Risks

### Single-process claim model

Accepted for this slice.

The claim flow should still be defensive, but we do not need full distributed worker coordination yet.

### Job table without deep observability

Accepted for this slice.

The table itself creates a usable foundation. Rich operator visibility can come in a later slice.

## Implementation Order

1. add `workspace_cleanup_jobs` migration and store methods
2. upsert cleanup jobs from `expired` and `orphaned` transitions
3. add background cleanup worker that claims due jobs and calls runner delete
4. add retry scheduling and failure recording
5. add migration, store, service, and worker tests

