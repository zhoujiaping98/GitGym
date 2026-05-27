# Structured Changed Files UX Design

Date: 2026-05-27

## Goal

Upgrade the repository snapshot card from a raw `git status --short` text list into a structured changed-files view that is easier to scan during practice sessions.

This slice builds on repo snapshot and command attribution work already on `main`:

- repo snapshot already refreshes on lifecycle events and command completion
- the right-side card already shows `branch`, `HEAD`, `dirty/clean`, and attribution
- the API already returns `changed_files` as raw status lines

## Non-Goals

- No runner contract changes
- No API response shape changes
- No diff viewer
- No file-click actions
- No command history redesign
- No polling or refresh-model changes

## Current Problem

The repo card currently renders dirty files as a flat raw list:

- `M notes.txt`
- `?? draft.md`
- `R old.txt -> new.txt`

That is technically correct but weak product UX:

- staged vs unstaged changes are mixed together
- untracked files do not stand out
- rename and conflict rows are not visually distinguished
- users have to mentally parse porcelain status codes every time

## Recommended Approach

Keep the backend payload exactly as-is and productize the list in the web app.

The web layer should:

- parse each `git status --short` line into one or more structured change entries
- group those entries into:
  - `Staged`
  - `Unstaged`
  - `Untracked`
- render compact status badges plus file path text
- fall back to the raw line text if a row cannot be parsed safely

This is the highest-leverage approach because it materially improves readability without widening the runner or API contract.

## Alternatives Considered

### 1. Make runner return structured change objects

Rejected for this slice.

That would touch runner snapshot capture, API response types, and web consumers. The current data is already sufficient to materially improve the UI.

### 2. Keep the raw list and only add a legend

Rejected.

A legend helps a little, but it still leaves users doing manual parsing and does not solve the staged/unstaged grouping problem.

### 3. Add a full diff or file drawer

Rejected.

That is a larger product slice. This card should first become readable before it becomes interactive.

## Parsing Model

Use the current `git status --short` semantics.

Each raw line should be interpreted using the first two status columns plus the path text.

Supported codes for this slice:

- staged or unstaged: `M`, `A`, `D`, `R`, `C`, `U`
- untracked: `??`

### Entry Rules

#### Ordinary tracked file

Example:

- `M notes.txt`
- `MM notes.txt`
- `A  notes.txt`

Interpretation:

- first column -> staged status
- second column -> unstaged status

If both columns carry meaningful states, emit two separate display entries for the same path:

- one in `Staged`
- one in `Unstaged`

This preserves the real git state instead of collapsing it into one ambiguous row.

#### Untracked file

Example:

- `?? draft.md`

Interpretation:

- emit one `Untracked` entry

#### Rename or copy

Examples:

- `R  old.txt -> new.txt`
- ` C src/a.ts -> src/b.ts`

Interpretation:

- keep the `old -> new` text intact as the display path
- badge uses `Renamed` or `Copied`

#### Unmerged/conflict rows

Examples:

- `UU notes.txt`
- `AA notes.txt`

Interpretation:

- emit staged and/or unstaged entries according to the same two-column rule
- badge text should use a user-facing conflict label such as `Conflicted`

## Fallback Behavior

If a line cannot be parsed confidently:

- keep it visible
- place it in a generic fallback list under the changed-files section
- render the raw text unchanged

This slice must never hide dirty-state information just because parsing failed.

## UI Structure

The repo snapshot card keeps the existing top facts:

- `Branch`
- `HEAD`
- `Working tree`

When `Working tree = Clean`:

- do not render the changed-files section

When `Working tree = Dirty`:

- render a `Changed files` block below the snapshot facts
- within that block, render up to three sections in this order:
  - `Staged`
  - `Unstaged`
  - `Untracked`
- only show a section if it has at least one entry
- fallback raw rows, if any, render after the structured groups

Each entry should show:

- a small status pill
- the file path text

The card should stay compact and scannable, not turn into a table.

## Badge Copy

Recommended badge labels:

- `Modified`
- `Added`
- `Deleted`
- `Renamed`
- `Copied`
- `Conflicted`
- `Untracked`

These labels replace raw porcelain codes in the primary display.

## Type Boundary

Keep the API snapshot type unchanged:

- `changedFiles: string[]`

Add web-only derived types for:

- parsed change entry
- change group buckets

These belong near the web repo-state presentation layer, not in shared backend contracts.

## Component Boundary

Prefer a small parsing helper and keep `RepoPanel` mostly presentational.

Recommended split:

- a parsing helper under `apps/web/src/lib/` or `apps/web/src/components/`
- `RepoPanel` consumes grouped entries and renders them

Do not move parsing into `useRepoState`; refresh logic and presentation shaping should remain separate concerns.

## Testing

Add focused RTL coverage for:

- clean snapshot still shows no changed-files block
- dirty snapshot with staged-only file
- dirty snapshot with unstaged-only file
- dirty snapshot with mixed `MM` state appearing in both groups
- dirty snapshot with `??` untracked file
- dirty snapshot with rename row preserving `old -> new`
- unparseable row falling back to raw text

Existing attribution and refresh tests should remain intact.

## Expected Outcome

After this slice:

- the repo card becomes much easier to scan during practice
- users can immediately tell what is staged, unstaged, or untracked
- the system keeps its current backend contract and refresh model
- later repo-insight slices can build on a stronger presentation foundation
