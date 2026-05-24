# Repo Command Attribution UX Design

Date: 2026-05-24

## Goal

Upgrade the right-side repository card from a static snapshot into a causally legible status surface that also answers:

- what does the repo look like right now?
- what most recently made it look this way?

This slice keeps the existing repo snapshot facts:

- branch
- `HEAD` commit
- clean vs dirty
- changed files

It adds a lightweight attribution layer for the current visible snapshot, especially after terminal commands complete.

## Non-Goals

- No multi-snapshot history or timeline
- No backend persistence of repo snapshots or command metadata
- No diff viewer
- No staged vs unstaged split in this slice
- No terminal protocol redesign
- No page-level session or workspace shell changes

## Current Problem

The repo snapshot card now shows accurate state, but it still leaves one important question implicit:

- did my last command actually change anything here?

Users can see that the working tree is dirty and which files changed, but they still have to infer whether that state came from:

- the initial session load
- a reset or sync action
- the command they just ran

That makes the panel informative but not yet explanatory.

## Recommended Approach

Keep one current snapshot and add one current cause.

The web app already knows when repo-state refreshes are triggered by:

- session load
- create
- reset
- retry sync
- terminal command completion

This slice should preserve the current repo-state API and stale fallback behavior, while attaching the latest successful refresh trigger to the currently displayed snapshot.

That keeps the model simple:

- the repo-state API remains the source of truth for repository facts
- the web app remains responsible for refresh intent and command identity
- the repository card renders one snapshot plus one attribution line

## Data Contract

Reuse the existing session repo-state route:

- `GET /api/v1/practice-sessions/{sessionId}/repo-state`

No new route family is needed.

The existing response already carries `captured_at`, which is sufficient for this slice:

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
    "captured_at": "2026-05-24T02:00:00Z"
  }
}
```

Command identity should stay client-side. The browser already knows which terminal command completion triggered a repo refresh, so the backend does not need to echo command ids or command text in the repo-state response.

## Frontend Design

### State Model

Add a small attribution context alongside the existing repo-state view.

For each successful repo-state fetch, the app should retain:

- snapshot data
- `captured_at`
- the trigger that initiated the fetch
- command id and command text when the trigger was a completed terminal command

Trigger values:

- `session_load`
- `session_create`
- `session_reset`
- `session_sync`
- `command_complete`

This is attribution for the currently rendered snapshot only. It is not a historical ledger.

### Attribution Semantics

If the latest successful fetch came from:

- `session_load`
  - show a neutral freshness line such as `Snapshot loaded`
- `session_create`
  - show a lifecycle line such as `Snapshot refreshed after new session`
- `session_reset`
  - show a lifecycle line such as `Snapshot refreshed after reset`
- `session_sync`
  - show a lifecycle line such as `Snapshot refreshed after sync`
- `command_complete`
  - show a causal line such as `Updated after git add .`

If the snapshot is dirty, the attribution area may append a compact impact summary such as:

- `Updated after git add .`
- `3 changed files`

If the repo remains clean, the card should still attribute the refresh to the command when applicable. A completed command can be the cause of the latest snapshot even if it produced no repo change.

### Repo Panel Behavior

Keep the existing repository facts section intact:

- `Branch`
- `HEAD`
- `Working tree`
- changed files list when dirty

Add a small attribution/freshness line near the repo facts area.

Display rules:

- prefer command text for command-triggered refreshes
- keep lifecycle-triggered copy concise and non-debuggy
- do not show both stale/error warning and fresh attribution as equal peers; stale/error state should visually win
- preserve the current compact card layout rather than turning this into a command history panel

## Refresh And Error Handling

### Successful Refresh

When a repo-state fetch succeeds:

1. update the snapshot data
2. update `captured_at`
3. stamp the snapshot with the trigger context that initiated the fetch

For command-triggered refreshes, this means the visible snapshot becomes explicitly attributable to the completed command that caused the refresh.

### Failed Refresh

If a refresh fails:

- preserve the current snapshot and its previous attribution
- preserve the existing stale/unavailable behavior
- do not overwrite attribution with the failed trigger context

This avoids false claims such as implying a command updated the panel when the post-command snapshot fetch never actually succeeded.

### Session-Level Overrides

Session-level page shells still take precedence.

This slice must not alter behavior for:

- `Session unavailable`
- `Workspace unavailable`
- `Catalog unavailable`
- `Catalog empty`

The attribution line exists only when a live session workbench and repo card are already being rendered.

## Component Boundary

### `useRepoState`

Remains the source of truth for repository snapshot fetches and stale fallback behavior.

### App-Level Caller

Owns refresh trigger intent and associates it with the repo-state request being made.

This is the correct place to remember:

- why the fetch happened
- which completed command triggered it, if any

### `RepoPanel`

Remains presentational.

It should receive:

- the current repo-state view
- the current attribution metadata for the displayed snapshot

It should not infer command causality by inspecting terminal history on its own.

## Testing

### Frontend Unit Tests

Add or update coverage for:

- initial live-session load renders neutral attribution
- command-complete refresh renders command-based attribution text
- reset or sync refresh renders lifecycle attribution text
- failed command-triggered refresh preserves prior attribution instead of replacing it
- dirty repo snapshot can render changed-files summary alongside command attribution

### End-to-End

Add one focused flow that:

1. opens a live session
2. runs a mutating terminal command
3. waits for command completion
4. verifies the repo panel updates both:
   - repository facts or changed-files summary
   - attribution text tied to the command

This keeps the slice demonstrably user-visible without expanding test scope into a broader history feature.

## Success Criteria

This slice is successful when:

- users can tell whether the current repo snapshot came from session lifecycle recovery or the last completed terminal command
- the card still behaves correctly when repo refresh fails
- no new page-level state families or backend persistence are introduced
- the UX stays compact and operational rather than turning into a second command history view
