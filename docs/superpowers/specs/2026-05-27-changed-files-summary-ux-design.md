# Changed Files Summary UX Design

Date: 2026-05-27

## Goal

Keep the repository snapshot card compact when a workspace has many changed files by adding per-group counts and truncating long changed-file sections into a readable summary.

This slice builds on the repo snapshot work already on `main`:

- the card already shows `branch`, `HEAD`, `dirty/clean`, and attribution
- changed files are already parsed into `Staged`, `Unstaged`, `Untracked`, and fallback raw rows
- command completion already refreshes the snapshot

## Non-Goals

- No runner or API contract changes
- No new repo-state fields
- No expand/collapse interaction
- No diff viewer
- No click actions for files
- No changes to attribution or refresh timing

## Current Problem

The card is now structurally readable, but it still scales poorly when the working tree is noisy.

If a session has many changed files:

- each group can grow arbitrarily tall
- the right-side card can dominate the workbench
- users lose the quick-scan value of the panel

The current grouped rendering is better than raw `git status --short`, but it still behaves like an unbounded list instead of an operational summary.

## Recommended Approach

Keep the current grouped rendering model and add a small summary layer in the web app:

- each visible group header shows a count
- each group renders only the first `3` entries
- any hidden remainder is represented by a compact `+N more` row
- fallback raw rows use the same truncation rule

This keeps the panel useful for both small and large dirty states without widening the backend contract or adding new interaction state.

## Alternatives Considered

### 1. Add expand/collapse controls

Rejected for this slice.

That adds local UI state, more accessibility surface, and more tests. The first problem to solve is card height, not exploration depth.

### 2. Add a dedicated drawer or modal for file changes

Rejected.

That creates a second repo surface and starts turning a side card into a larger workflow. The current slice should stay inside the existing panel.

### 3. Only add counts and keep full lists visible

Rejected.

Counts help orientation, but they do not solve the layout problem when a group contains many files.

## Display Rules

Use a fixed summary rule for every changed-files section:

- `max visible entries per group = 3`

For each structured group:

- if the group is empty, do not render it
- if the group has `1-3` entries, render them all
- if the group has more than `3` entries, render the first `3` and then a summary row:
  - `+1 more`
  - `+4 more`
  - and so on

The same rule applies to fallback raw rows:

- show at most the first `3`
- if more remain, show one final summary row with the remainder count

## Group Header Copy

The section headings should include the count:

- `Staged (2)`
- `Unstaged (5)`
- `Untracked (1)`

Fallback rows should not get a new labeled section. They remain visually last and only use the summary row when truncated.

## Summary Row Behavior

The summary row is informational only.

It should:

- be visually lighter than a normal file row
- not use a status pill
- not imply interactivity

Recommended copy:

- `+2 more`
- `+7 more`

Do not mention hidden filenames in the summary row.

## Component Boundary

Keep the change parsing helper focused on parsing only. Do not move truncation into `groupRepoChanges`.

Recommended split:

- `groupRepoChanges(lines)` continues to produce the full parsed model
- a small UI-oriented helper derives visible entries plus hidden counts for rendering
- `RepoPanel` stays mostly presentational and renders already-shaped sections

This keeps parsing semantics separate from display summarization.

## Type Boundary

Keep existing repo snapshot and parsed change types intact where possible.

Add web-only display types only if they reduce repeated conditional logic in `RepoPanel`, for example:

- summarized change group
- visible fallback rows with hidden remainder count

Do not change shared API-facing types.

## Testing

Add focused RTL coverage for:

- group headers showing counts
- a group with exactly `3` entries rendering no summary row
- a group with `4+` entries rendering only the first `3` plus `+N more`
- fallback raw rows truncating with the same summary behavior
- existing small dirty-state tests continuing to show full entries unchanged

No new E2E coverage is required for this slice because the change is presentational and the existing repo snapshot refresh path is already covered.

## Expected Outcome

After this slice:

- the repo card stays compact even for noisy working trees
- users can quickly understand both the category and volume of changes
- the current snapshot API and refresh model remain unchanged
- later interactive repo slices can build on a stronger summary-first presentation
