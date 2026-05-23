# Repo Snapshot Data Chain Design

Date: 2026-05-23

## Goal

Upgrade the right-side operational session card from session metadata only to real repository insight backed by live workspace snapshots.

This slice adds a data chain for:

- current branch
- current `HEAD` commit
- working tree cleanliness
- changed file summary

The card should stay current on initial load, after lifecycle actions, and after terminal commands complete.

## Non-Goals

- No polling
- No page-level recovery shell changes
- No session lifecycle redesign
- No persistence of repo snapshots in MySQL
- No full commit history, diff viewer, or staged/unstaged split
- No terminal websocket protocol redesign in this slice

## Current Problem

The existing right-side card is operationally clearer than the old metadata dump, but it still does not answer the repository questions users actually care about:

- which branch am I on?
- what commit am I at?
- is the workspace clean?
- what changed?

The runner already captures git snapshots around command execution, but the product does not expose that information as a stable UI data source.

## Recommended Approach

Add a dedicated repo-state API for a practice session and refresh it from the web app whenever the active workspace is likely to have changed.

This approach keeps repository insight as a focused operational resource:

- page load and lifecycle refreshes use normal HTTP fetches
- terminal command completion acts only as a refresh signal
- the repo panel renders last-known snapshot data and degrades inline if refresh fails

This avoids coupling repository state delivery to terminal websocket payloads while still keeping the panel fresh enough for interactive terminal work.

## Data Contract

Add an authenticated route:

- `GET /api/v1/practice-sessions/{sessionId}/repo-state`

Response shape:

```json
{
  "data": {
    "branch": "main",
    "head_commit": "6f9bc9e2f9e3f4f24b88a1d8d76d8ef0f1b1c6a0",
    "dirty": true,
    "changed_files": [
      "M notes.txt",
      "?? scratch.md"
    ],
    "captured_at": "2026-05-23T04:00:00Z"
  }
}
```

Field semantics:

- `branch`: result of the current workspace branch lookup
- `head_commit`: full current `HEAD` SHA
- `dirty`: `true` when `changed_files` is non-empty, otherwise `false`
- `changed_files`: ordered `git status --short` lines
- `captured_at`: UTC timestamp of the snapshot capture

This route returns live operational state only. It is not a lifecycle record and should not be stored in the practice session row.

## Backend Design

### Runner

Reuse the existing snapshot capture foundation in `services/runner/internal/engine/snapshots.go`.

Current runner snapshot data already includes:

- `HeadCommit`
- `BranchName`
- `StatusSummary`
- `CapturedAt`

This slice formalizes that data as a runner-facing repo-state response rather than creating a second git-inspection path.

### API

Add a small practice-session-scoped handler that:

1. authenticates the user
2. resolves the requested practice session using existing ownership and lifecycle checks
3. rejects unavailable/orphaned/missing workspaces through the existing session/workspace error family
4. asks the runner for the current snapshot for the session workspace
5. maps it into the repo-state JSON contract

The API remains stateless for repo snapshots. Each request captures or retrieves the current live workspace state from runner-backed execution.

## Frontend Design

### State Model

Add repo-state client state alongside the existing current-session, catalog, and terminal state:

- `idle` when there is no displayed live session
- `loading` when a snapshot fetch is in flight and no prior snapshot exists
- `ready` with the latest snapshot
- `stale` when a refresh failed but a previous snapshot still exists
- `error` when the snapshot could not be loaded and there is no previous snapshot to display

This state remains local to the web app and does not change the session lifecycle model.

### Refresh Triggers

Fetch repo state when:

- a live session is first rendered
- a new session becomes active after create
- a live session remains active after reset
- retry-sync or current-session refresh resolves to a live session
- terminal history records a command completion for the active session

Do not fetch repo state on a timer.

### Repo Panel Behavior

Keep the current operational session card and extend it with repository facts:

- `Branch`
- `HEAD`
- `Working tree`
- changed files list when dirty

Display rules:

- `Working tree` shows `Clean` or `Dirty`
- `HEAD` may show a shortened SHA in the visible value, but the full SHA should remain available in the DOM or title attribute
- changed files should stay compact and summary-oriented; this is not a diff viewer
- repo-state loading should use a small inline placeholder inside the card
- repo-state failure should degrade only the repo section, not collapse the workbench

### Stale Snapshot Behavior

If a repo-state refresh fails after at least one successful snapshot:

- keep rendering the last known snapshot
- mark the repo section as stale
- show a compact inline hint such as `Repository state may be out of date.`

If there is no prior snapshot and the fetch fails:

- show an inline unavailable state such as `Repository state unavailable.`

This keeps the workbench usable while making freshness explicit.

## Error Handling

### Session-Level Failures

If the active session is no longer restorable, existing page-level session/workspace flows still own the UI.

Repo-state fetch logic must not override:

- `Session unavailable`
- `Workspace unavailable`
- `Catalog unavailable`
- `Catalog empty`

### Repo-State-Only Failures

If the session remains live but repo-state fetch fails:

- keep the terminal and history visible
- keep the rest of the operational card visible
- degrade only the repository snapshot portion inline

### Command-Driven Refresh Failures

If a terminal command completes but the follow-up repo-state refresh fails:

- command history should still finalize normally
- terminal transcript should remain untouched
- repo-state should either remain stale or show unavailable if no prior snapshot exists

## Testing

### API Tests

Add coverage for:

- successful repo-state response for a valid live session
- unauthorized or foreign-session access rejection
- missing/orphaned/unavailable workspace rejection
- dirty workspace response including changed file lines

### Frontend Unit Tests

Add or update coverage for:

- live session loads and renders branch, head, clean/dirty state
- dirty repo renders changed file summary
- terminal command completion triggers repo-state refresh
- refresh failure after a prior snapshot keeps last-known data and marks it stale
- initial fetch failure without prior snapshot shows inline unavailable state

### End-to-End

Add a focused smoke flow that:

1. starts from a live session
2. confirms initial clean repo state in the card
3. runs a mutating git command through the terminal
4. waits for command completion
5. verifies the repo panel updates to dirty and shows the changed file entry

One focused E2E is enough for this slice; full matrix coverage is unnecessary.

## Implementation Notes

- Prefer a dedicated repo-state API client helper in the web app rather than embedding fetch logic directly in `RepoPanel`
- Keep `RepoPanel` presentational
- Reuse the existing runner snapshot model where practical instead of inventing a parallel repo-state representation
- Keep this slice additive to the operational session card, not a visual redesign

## Success Criteria

This slice is successful when:

1. the right-side card shows real branch and `HEAD` data for live sessions
2. users can tell whether the workspace is clean or dirty without using the terminal
3. changed files appear after mutating terminal commands complete
4. repo-state refresh failures degrade inline without collapsing the workbench
5. no polling or websocket protocol expansion is required to ship the feature
