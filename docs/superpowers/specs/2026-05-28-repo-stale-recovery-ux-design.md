# Repo Stale Recovery UX Design

Date: 2026-05-28

## Goal

Add a small, explicit recovery path to the repository snapshot card so users can retry repo-state loading without leaving the live workbench.

This slice builds on what already exists on `main`:

- repo-state fetches already happen on session load and command completion
- stale fallback already preserves the last successful snapshot
- attribution, freshness, and command outcome already explain what the visible snapshot represents

## Non-Goals

- No API or runner contract changes
- No page-level shell changes
- No polling
- No diff viewer or changed-files redesign
- No terminal websocket changes

## Current Problem

The repo card currently degrades correctly, but not completely:

- if the first repo-state fetch fails, the card says `Repository state unavailable.`
- if a later refresh fails, the card keeps the last snapshot and says `Repository state may be out of date.`

That preserves context, but it still leaves the user without a direct recovery action inside the card. The only way forward is to wait for another lifecycle event or another terminal command to trigger a fetch.

## Recommended Approach

Keep the existing stale/error state model and add a card-local retry action driven by the existing web fetch path.

That means:

- `useRepoState` exposes a manual `retry` action
- `useRepoState` also exposes whether a fetch is currently in flight
- `RepoPanel` renders a small `Retry` button when repo-state is in `error` or `stale`
- clicking `Retry` reuses the same repo-state endpoint for the current live session

This is the highest-leverage approach because it finishes the current degradation model without reopening backend or session-lifecycle design.

## Alternatives Considered

### 1. Add automatic retry with backoff inside the hook

Rejected.

That would hide fetch churn in the background and make the card feel less predictable. The missing piece is an explicit user recovery path, not silent retry policy.

### 2. Reuse a page-level `Try again` shell

Rejected.

Repo-state failure is intentionally scoped below the page shell. Escalating it would undo the separation already established between session-level failure and repo-card degradation.

### 3. Add a new repo-refresh trigger type such as `manual_retry`

Rejected for now.

Manual retry can reuse the existing trigger context. If the retry succeeds, the card can continue to describe the current visible snapshot using the latest successful attribution model already in place.

## Interaction Model

### Error without a prior snapshot

When `repoState.status === "error"`:

- keep `Repository state unavailable.`
- render a card-local `Retry` action

If retry succeeds:

- render the snapshot normally
- keep the existing attribution rules

### Stale snapshot after a failed refresh

When `repoState.status === "stale"` and `repoState.error` exists:

- keep `Repository state may be out of date.`
- keep the preserved snapshot visible
- render a card-local `Retry` action next to that stale state

If retry succeeds:

- remove the stale warning
- keep rendering the newly loaded snapshot with updated attribution/freshness/outcome

## Refreshing Behavior

Expose an explicit loading signal from `useRepoState`.

When a manual retry is in flight:

- keep the existing visible snapshot on screen if one exists
- disable the retry button
- render `Refreshing repository state...` as a compact inline note

Important boundary:

- do not blank the card while retry is in flight
- do not collapse the stale fallback into a generic loading state

## State Boundary

Keep responsibility split this way:

- `useRepoState`
  - owns fetch lifecycle
  - owns retry action
  - owns in-flight state
  - preserves stale fallback and last successful attribution/outcome behavior

- `RepoPanel`
  - remains presentational
  - renders `Retry` only when the hook says retry is available
  - renders disabled/loading treatment from supplied props

No changes are needed in backend contracts.

## Failure Behavior

If a manual retry fails:

- preserve the current degraded state
- preserve the last successful snapshot if one exists
- preserve prior attribution and prior command outcome
- keep the retry action available for another attempt

This matches the existing repo insight rule: never invent new state from a failed refresh.

## Testing

Add focused RTL coverage for:

- first-load repo-state error renders a retry action and can recover into a visible snapshot
- stale repo-state warning renders a retry action and can recover without collapsing the workbench
- retry stays available after a failed manual retry
- retry shows a temporary `Refreshing repository state...` state while the request is pending

No E2E expansion is required for this slice because the behavior is entirely inside the existing web fetch/render loop.

## Expected Outcome

After this slice:

- repo-state degradation will have a direct in-card recovery path
- the live workbench will stay primary while repo-state retries remain local and explicit
- the current stale fallback model will become product-complete without widening scope into backend changes
