# Workspace Unavailable Recovery UX Design

Date: 2026-05-21

## Goal

Turn the current `Workspace unavailable` page-level shell from an engineering-flavored error state into a clear recovery page that helps the user understand what happened and start a fresh session.

This slice is about communication and recovery UX only. It does not change lifecycle detection or session-creation mechanics.

## Problem

The current orphaned-session shell is functionally correct but too internal:

- it reads like a system failure page
- it emphasizes the broken workspace more than the recovery path
- it exposes diagnostic phrasing too early in the hierarchy

For the user, the important answer is not “why did attachment fail internally?” but:

1. Can I continue?
2. What should I do next?

## Non-Goals

- No changes to orphaned-session detection
- No changes to terminal attach behavior
- No changes to the scenario picker flow
- No new backend APIs
- No “retry attach” or “repair existing workspace” behavior
- No automatic recovery actions

## Recommended Approach

Keep `Workspace unavailable` as a page-level shell, but rewrite it as a recovery-first state:

1. lead with the outcome in plain language
2. clearly offer a fresh-start path
3. keep technical diagnostics available but visually secondary

This preserves the current control flow while making the state feel intentional and productized.

## Interaction Model

### Recovery Style

Use a conservative recovery flow:

- the user stays on the `Workspace unavailable` shell
- the primary action remains `New Session`
- clicking `New Session` opens the existing scenario picker

This avoids surprising automatic behavior and keeps recovery aligned with the rest of session creation.

### What We Are Not Doing

- not auto-opening the picker on entry
- not auto-creating a default session
- not attempting to reattach to the old sandbox
- not adding a secondary `Retry attach` action

The recovery path is explicitly “start fresh,” not “repair old state.”

## Information Hierarchy

The shell should have three layers.

### 1. Outcome First

Title remains:

- `Workspace unavailable`

Primary body becomes result-oriented:

- `Your previous sandbox can no longer be reopened. Start a fresh session to keep practicing.`

This phrasing keeps the focus on what the user experiences, not on internal failure terminology such as `orphaned`, `attach failed`, or `runner missing`.

### 2. Recovery Path Second

Primary CTA remains:

- `New Session`

Behavior remains unchanged:

- opening the existing scenario picker

This is important: the page communicates a clear next step without inventing a new interaction model.

### 3. Diagnostics Last

The existing low-level error detail remains available in the shell’s detail area.

Examples:

- missing workspace text
- runner cleanup/orphan detail
- transport-related explanatory text already surfaced by the app

Display rule:

- diagnostics remain visible
- diagnostics are visually secondary to title and CTA

This keeps the page useful for debugging without making the internal failure message the primary UX.

## Scope Boundary

This slice changes only the page-level `Workspace unavailable` shell rendering in `App.tsx`.

Specifically:

- update headline/body copy
- preserve the `New Session` action
- preserve the existing detail source
- keep scenario-picker invocation unchanged

It does not change:

- lifecycle transitions
- `useCurrentSession`
- `useTerminalSession`
- `ScenarioPickerModal`
- catalog loading behavior

## Testing

### Unit Coverage

Update frontend tests to verify:

1. orphaned-session state renders the new recovery-first body copy
2. the low-level detail text still appears
3. clicking `New Session` from the recovery shell still opens the existing scenario picker

### End-to-End Coverage

Only update existing assertions if needed.

No new backend setup is required.

If there is already a flow that lands on the unavailable shell and then opens the picker, keep it and align text assertions with the new copy.

## Success Criteria

This slice is successful when:

1. the unavailable shell explains the outcome in user-facing language
2. the recovery path is obvious and unchanged
3. diagnostics remain available without dominating the page
4. the user is guided toward a fresh session rather than a vague failure state
