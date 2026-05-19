# Workspace Cleanup Design

## Goal

Add real abandoned-workspace cleanup after practice sessions become `expired` or `orphaned`, while preserving a short recovery/debugging grace period.

This slice builds on the existing lifecycle hardening work:

- API already transitions stale sessions to `expired`
- API already transitions missing-workspace sessions to `orphaned`
- API already runs a background expiry sweep

The missing piece is physical workspace deletion.

## Scope

This design covers:

- runner-owned workspace deletion
- API-triggered cleanup scheduling for `expired` and `orphaned` sessions
- grace-period behavior
- idempotent cleanup semantics
- tests for delete, delayed delete, and API scheduling

This design does not cover:

- user-facing cleanup history UI
- configurable cleanup policy in admin settings
- database schema changes for cleanup audit trails

## Recommended Approach

Use API-managed lifecycle state with runner-managed workspace destruction.

The API remains the source of truth for session lifecycle. The runner remains the owner of the workspace filesystem. When a session becomes unrecoverable, the API tells the runner to delete the workspace immediately or after a short delay.

This keeps responsibilities clean:

- API decides when a session is no longer usable
- runner decides how to safely tear down its own workspace and terminal state

## Alternatives Considered

### 1. API deletes workspace directories directly

Rejected.

This breaks the service boundary and assumes the API process can see the runner filesystem. That is brittle now and incorrect if runner becomes independently deployed.

### 2. Runner scans workspace directories on its own

Rejected for the first slice.

Runner-local scanning would not know which workspaces are intentionally active, expired, or orphaned unless the API truth is duplicated into runner state.

### 3. API marks sessions only and never deletes workspaces

Rejected as incomplete.

This improves session semantics but leaks disk indefinitely.

## Grace-Period Policy

Use a fixed grace-period policy in code for the first slice:

- `expired` sessions: delete immediately
- `orphaned` sessions: delete after 10 minutes

Rationale:

- expired sessions are expected and do not represent a surprising infrastructure failure
- orphaned sessions are more useful to preserve briefly for debugging and operator inspection

If the runner reports the workspace is already gone, cleanup is treated as effectively complete.

## API Design

### Runner HTTP Contract

Add a runner endpoint:

- `DELETE /internal/workspaces/{workspaceID}`

Request body:

```json
{
  "reason": "expired",
  "delete_after_seconds": 0
}
```

Supported reasons for the first slice:

- `expired`
- `orphaned`

Response semantics:

- `202 Accepted` when deletion is scheduled
- `204 No Content` when deletion happens immediately and completes synchronously
- `404 Not Found` when the workspace does not exist

For API callers, `404` is acceptable and should be treated as an idempotent cleanup result.

## Runner Design

### Workspace Delete Handler

Add a delete handler under `services/runner/internal/http/handlers`.

Responsibilities:

- validate `workspaceID`
- decode optional cleanup request payload
- resolve workspace path through the existing safe workspace resolver
- schedule or execute deletion

### Cleanup Manager

Add a small in-memory cleanup manager in runner.

Responsibilities:

- accept cleanup requests for a workspace ID and path
- deduplicate pending cleanup for the same workspace
- run immediate deletion when delay is zero
- run delayed deletion when delay is positive
- stop any active terminal session for that workspace before deleting

For this first slice, cleanup scheduling only needs to survive within the runner process lifetime. Persistence across runner restarts is out of scope.

### Delete Execution

Delete execution should:

1. release or close any active terminal session for the workspace
2. remove the workspace directory recursively
3. treat missing directories as success

This keeps deletion idempotent and safe against retry.

## API Service Design

### Runner Client

Extend the API runner client with:

- `DeleteWorkspace(ctx, workspaceID string, reason string, deleteAfter time.Duration) error`

It should map:

- `404` to `runner.ErrWorkspaceNotFound`
- other non-success responses to a cleanup-specific error

### Cleanup Trigger Points

Trigger runner cleanup in two places:

1. expiry sweep
   - after a session is transitioned to `expired`
   - request immediate deletion

2. orphan transition
   - after a session is transitioned to `orphaned`
   - request delayed deletion with a 10-minute grace period

If runner cleanup scheduling fails, the API must not roll back the session lifecycle transition. The session state is still correct even if physical cleanup needs retry.

### Retry Behavior

Cleanup scheduling remains best-effort in this slice.

Practical behavior:

- log cleanup failures
- rely on future lifecycle sweeps or repeated orphan checks to retry scheduling

If we later need stronger guarantees, we can add explicit cleanup jobs in the database.

## Data Flow

### Expired Session

1. API expiry loop finds stale `active` session
2. API store transitions it to `expired`
3. API asks runner to delete workspace immediately
4. runner closes terminal if needed and removes workspace

### Orphaned Session

1. terminal attach or reset discovers missing/broken workspace state
2. API transitions session to `orphaned`
3. API asks runner to delete workspace after 10 minutes
4. runner deletes when grace period expires

## Error Handling

- missing workspace during delete is success-like and should not block lifecycle completion
- runner delete scheduling failure is logged and surfaced for observability, but it must not revert session status
- duplicate cleanup requests for the same workspace should collapse into one scheduled delete where possible

## Testing

### Runner Tests

- delete endpoint removes a real temp workspace
- delayed delete waits for the grace period and then removes the directory
- deleting a workspace with an active terminal first closes the terminal session
- repeated delete requests remain idempotent

### API Tests

- expiry sweep schedules immediate runner cleanup for expired sessions
- orphan transition schedules delayed cleanup for orphaned sessions
- runner cleanup failure does not prevent session state transition
- runner `workspace not found` during delete is tolerated

## Risks

### In-memory delayed cleanup is not restart-safe

Accepted for this slice.

The existing project is still local-beta quality. Restart-safe cleanup can come later with persistent cleanup jobs.

### Duplicate cleanup scheduling

Acceptable if the runner implementation is idempotent and deduplicates by workspace ID.

## Implementation Order

1. Add runner delete endpoint and engine cleanup path
2. Add runner cleanup manager and delayed deletion behavior
3. Extend API runner client with delete support
4. Trigger cleanup from expiry sweep and orphan transitions
5. Add tests for runner delete and API scheduling
