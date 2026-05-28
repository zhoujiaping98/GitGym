# Repo Snapshot Freshness UX Design

Date: 2026-05-28

## Goal

Make the repository snapshot card clearly answer when the currently visible snapshot was captured, using the existing `capturedAt` timestamp that already comes from the repo-state API.

This slice builds on the repo insight work already on `main`:

- live repo snapshots already render in the right-side card
- command and lifecycle attribution already tell users what triggered the latest successful refresh
- changed files are already grouped and summarized
- stale snapshot fallback already preserves the last good snapshot when refresh fails

## Non-Goals

- No runner or API contract changes
- No polling or live ticking relative-time UI
- No new stale retry behavior
- No command history redesign
- No timezone preferences or localization work beyond the existing browser formatting

## Current Problem

The card already tells users:

- what branch and HEAD are visible
- whether the working tree is dirty
- what action last refreshed the snapshot

But it still does not clearly say when that snapshot was captured.

That leaves a practical gap:

- users cannot tell whether the panel is seconds old or minutes old
- stale-state copy says the snapshot may be out of date, but not how old the preserved snapshot is
- attribution alone is not enough because “Updated after git add .” does not tell you when it happened

## Recommended Approach

Keep attribution semantics as they are and add a separate freshness line driven by `capturedAt`.

The card should:

- keep the existing attribution line unchanged in meaning
- render a compact freshness line for successful snapshots:
  - `Captured May 23, 4:00 PM`
- when the repo state is stale, keep the stale warning and add the same freshness line so users know how old the preserved snapshot is

This is the highest-leverage approach because it clarifies freshness without rewriting attribution copy or widening backend data.

## Alternatives Considered

### 1. Append timestamps directly to attribution copy

Rejected.

That overloads one line with two concerns: cause and freshness. `Updated after git add . at 4:00 PM` is readable, but less scannable and harder to vary for stale states.

### 2. Add `Captured` as a fourth field inside the repo snapshot facts grid

Rejected.

That makes the card denser and mixes low-frequency metadata into the branch/HEAD/dirty facts. Freshness reads better as inline status metadata near attribution and stale warnings.

### 3. Show relative time like `Captured 2 minutes ago`

Rejected for this slice.

Relative time would require ticking or scheduled rerenders to stay truthful. Absolute timestamp copy is simpler and stable.

## UI Rules

When `repoState.status === "ready"`:

- show attribution copy if available
- show one freshness line:
  - `Captured <formatted timestamp>`

When `repoState.status === "stale"`:

- keep the existing stale warning:
  - `Repository state may be out of date.`
- also show the freshness line based on the preserved snapshot:
  - `Captured <formatted timestamp>`

When `repoState.status === "loading"` or `error` with no snapshot:

- do not render a freshness line

## Formatting Rules

Use the same date formatting family already used elsewhere in `RepoPanel`:

- month short
- day numeric
- hour numeric
- minute two-digit

Example:

- `Captured May 23, 4:00 PM`

This slice should keep formatting consistent with the session lifecycle facts instead of introducing a second date style.

## Component Boundary

Keep this entirely in the web app.

Recommended split:

- a small presentation helper in `RepoPanel.tsx` or a nearby repo-panel helper formats the freshness copy from `capturedAt`
- `useRepoState` remains responsible only for state, attribution, and stale preservation

Do not move freshness derivation into API types or fetch hooks.

## Testing

Add focused RTL coverage for:

- initial ready snapshot renders `Captured ...`
- command-driven snapshot refresh updates freshness text when a newer snapshot arrives
- stale fallback keeps rendering the last successful `Captured ...` value alongside the stale warning
- loading and unavailable states do not render a bogus freshness line

No E2E expansion is required for this slice because it is presentational and existing repo snapshot flows are already exercised.

## Expected Outcome

After this slice:

- the repo card clearly shows both why the snapshot changed and when it was captured
- stale snapshot fallback becomes easier to interpret
- no backend contract or refresh behavior changes are needed
