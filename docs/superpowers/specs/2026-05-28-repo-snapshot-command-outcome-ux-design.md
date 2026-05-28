# Repo Snapshot Command Outcome UX Design

Date: 2026-05-28

## Goal

Add a concise outcome line to the repository snapshot card so users can see what the last completed terminal command changed in repo state, not just that a refresh happened.

This slice builds on the repo insight work already on `main`:

- repo snapshots already refresh after completed terminal commands
- attribution already says what triggered the current snapshot
- freshness already says when the snapshot was captured
- changed files are already grouped and summarized

## Non-Goals

- No runner or API contract changes
- No diff viewer or file-level before/after comparison
- No lifecycle-refresh outcome copy
- No terminal history redesign
- No polling or new refresh triggers

## Current Problem

The card currently answers two useful questions:

- what triggered this snapshot
- when was it captured

But it still does not answer the next product question:

- what actually changed because of that command

Examples:

- after `git add .`, users can see `Updated after git add .`
- they can see the new grouped changed-files state
- but they cannot quickly tell whether the command dirtied the tree, cleaned it, changed branches, or simply changed the number of tracked modifications

That forces users to manually compare the visible snapshot with what they remember from the prior one.

## Recommended Approach

Compare the previous successful repo snapshot with the current successful repo snapshot when the refresh trigger is `command_complete`, then render one concise outcome line in the card.

This is the highest-leverage approach because:

- it uses data already present in the web app
- it keeps comparison logic out of the API
- it complements, rather than replaces, attribution and freshness

## Alternatives Considered

### 1. Put before/after detail into the changed-files list

Rejected.

That would make the right-side card much noisier and push a small summary problem into a much larger comparison UI.

### 2. Add outcome copy to terminal history rows instead of the card

Rejected.

That would scatter repo insight across two surfaces. The repo card should remain the main place to understand current repository state.

### 3. Only show clean/dirty transitions

Rejected.

That would miss common useful cases such as `Changed files: 1 -> 4` when the tree stays dirty throughout.

## Outcome Rules

Only compute outcome copy for successful snapshots whose attribution trigger is `command_complete`.

Use this priority order:

1. if `previous.dirty === false` and `current.dirty === true`
   - `Working tree became dirty.`

2. if `previous.dirty === true` and `current.dirty === false`
   - `Working tree is now clean.`

3. if `previous.branch !== current.branch`
   - `Branch changed: <old> -> <new>.`

4. if `previous.changedFiles.length !== current.changedFiles.length`
   - `Changed files: <old> -> <new>.`

5. if `previous.headCommit !== current.headCommit`
   - `HEAD changed.`

6. otherwise
   - no outcome line

This keeps the copy result-oriented and avoids spamming the card with redundant text.

## Rendering Rules

When command-driven outcome copy exists:

- render it as its own inline note inside the repo snapshot header area
- keep it separate from:
  - attribution copy
  - freshness copy
  - stale warning

Recommended order:

1. loading or unavailable status note, if any
2. attribution copy
3. freshness copy
4. outcome copy
5. stale warning below, as it exists today

Do not fuse these into one sentence.

## State Boundary

Keep comparison logic out of `RepoPanel`.

Recommended split:

- `useRepoState` or a nearby web helper compares snapshots
- the hook returns `repoOutcome` or equivalent derived copy
- `RepoPanel` remains mostly presentational and only renders the supplied string

This follows the current architecture where the hook owns repo-state derivation and the panel owns rendering.

## Failure Behavior

If a command-triggered refresh fails and the card falls back to stale state:

- do not compute a new outcome from the failed refresh
- preserve the last successful outcome copy, if any

This matches the current attribution preservation rule and avoids inventing outcomes for snapshots that never actually loaded.

## Testing

Add focused RTL coverage for:

- clean -> dirty command refresh
- dirty -> clean command refresh
- changed-file count delta when dirty state does not flip
- failed command-triggered refresh preserving the prior successful outcome

No E2E expansion is required for this slice because the behavior is derived from existing frontend refresh flows.

## Expected Outcome

After this slice:

- the repo card will tell users what the last completed command changed
- attribution, freshness, and outcome will each answer a distinct question
- no backend contract changes will be required
