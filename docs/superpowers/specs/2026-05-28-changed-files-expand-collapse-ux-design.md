# Changed Files Expand/Collapse UX Design

Date: 2026-05-28

## Goal

Make long changed-file lists in the repository snapshot card explorable without abandoning the current summary-first layout.

This slice builds on the repo-card work already on `main`:

- changed files are already grouped into `Staged`, `Unstaged`, `Untracked`, and fallback raw rows
- each group already shows counts
- long groups are already truncated to three visible rows

## Non-Goals

- No API or runner changes
- No diff viewer
- No file click actions
- No global `Expand all`
- No persistence of expanded state across snapshots or sessions

## Current Problem

The current summary behavior keeps the card compact, but it is not enough once users want to inspect the hidden remainder:

- the card tells users `+N more`
- but there is no way to reveal those hidden rows in place

That leaves the card in an awkward middle state: it is no longer overloaded, but it still withholds useful detail when the dirty tree grows beyond three entries.

## Recommended Approach

Add local expand/collapse controls for each truncated section while preserving the summary-first default.

That means:

- each grouped section remains collapsed by default
- collapsed sections render the first three rows and a control such as `Show 2 more`
- expanded sections render all rows and a `Show less` control
- each section expands independently

This is the best tradeoff because it restores access to detail without turning the repo card into a fully interactive file browser.

## Alternatives Considered

### 1. Global `Expand all` for the whole card

Rejected.

That would create unnecessary state coupling between unrelated groups and make the card jump more aggressively on large dirty trees.

### 2. Replace the repo card with a drawer or modal

Rejected.

That is a larger surface-area change and starts drifting toward a dedicated repo workspace instead of finishing the current card.

### 3. Keep the existing `+N more` summary row only

Rejected.

That preserves compactness, but not usability. Users still cannot inspect the hidden remainder.

## Interaction Model

### Grouped sections

For `Staged`, `Unstaged`, and `Untracked` groups:

- if `hiddenCount === 0`
  - render exactly as today
- if `hiddenCount > 0` and the section is collapsed
  - render the first three rows
  - render a small button:
    - `Show 1 more`
    - `Show 4 more`
- if the section is expanded
  - render all rows
  - render `Show less`

### Fallback raw rows

Fallback rows follow the same rule:

- collapsed: first three raw rows plus `Show N more`
- expanded: all raw rows plus `Show less`

Fallback rows remain visually last. This slice does not add a new titled section just for fallback rows.

## State Model

Expansion is local presentation state owned by `RepoPanel`.

Recommended shape:

- one local key per visible section
- grouped sections use stable ids such as `staged`, `unstaged`, `untracked`
- fallback uses `fallback`

Expansion state should reset when the visible repo snapshot changes.

Recommended reset boundary:

- when `repoState.snapshot.capturedAt` changes for a newly loaded successful snapshot

This keeps behavior predictable:

- users can inspect a long list in the current snapshot
- but the card returns to summary mode when a newer snapshot arrives

## Rendering Rules

Preserve the current visual hierarchy:

- group heading
- changed-file rows
- compact toggle action below the rows when needed

Do not render both:

- `+N more`
- and a separate `Show all`

That is redundant. The collapsed control should carry the remainder count directly:

- `Show 1 more`
- `Show 3 more`

Expanded sections should use:

- `Show less`

## File Boundaries

Keep this slice inside the web presentation layer.

Recommended split:

- `repoChangeSummary.ts`
  - can expose enough data for both collapsed and expanded rendering
- `RepoPanel.tsx`
  - owns expand/collapse state
  - decides whether to render truncated or full rows

No new backend types or contracts are needed.

## Failure Behavior

Stale repo snapshots and repo retry state should continue to work exactly as they do now.

Important boundary:

- expand/collapse is purely presentation state
- a failed repo refresh must not mutate the currently expanded list contents
- a successful newer snapshot may reset expanded sections back to collapsed

## Testing

Add focused RTL coverage for:

- grouped changed files expanding from `Show N more` to the hidden file rows
- grouped changed files collapsing back to summary mode via `Show less`
- fallback raw rows supporting the same expand/collapse behavior
- expansion state resetting when a newer repo snapshot replaces the current one

No E2E expansion is required for this slice because the behavior is local card presentation.

## Expected Outcome

After this slice:

- the repo card stays compact by default
- users can inspect long changed-file sections in place
- expansion remains local, predictable, and easy to reset when repo state changes
