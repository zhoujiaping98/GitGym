# Session Reconciliation UX Design

Date: 2026-05-22

## Goal

Turn the live-workbench `Session reconciliation` feedback from ad-hoc inline error text into a more intentional result model with clear `Retry sync` semantics.

This slice is about the inline reconciliation UX that appears while the live workbench remains mounted. It does not change page-level recovery shells, backend session semantics, or the scenario picker flow itself.

## Problem

The current reconciliation feedback is functional but not yet fully productized:

- different create/reset reconciliation outcomes are surfaced as loosely related inline messages
- `Retry sync` appears for some outcomes but not others without a sufficiently explicit user-facing rule
- informational results and retryable reconciliation failures are not clearly separated in the UX model

For the user, the important questions are:

1. Did the create/reset action work, but the server is not yet aligned?
2. Is there a meaningful retry action here, or is this just a result notice?
3. Can I keep using the current workbench while the app explains what happened?

## Non-Goals

- No changes to page-level `Session unavailable` recovery UX
- No changes to `Workspace unavailable`, `Practice catalog unavailable`, or `Practice catalog empty`
- No backend API changes
- No full session state machine refactor
- No change to create/reset transport errors that fail before reconciliation begins

## Recommended Approach

Normalize the live-shell reconciliation feedback into two explicit classes:

1. informational reconciliation results
2. retryable reconciliation results

This keeps the user in the current live workbench while making the inline feedback easier to interpret and the `Retry sync` action more predictable.

## Reconciliation Model

### Informational Results

Use informational inline feedback when the app has a result to report, but there is no meaningful retry path that should be presented inside the live shell.

Examples:

- `Created a new session, but the server did not return it as current.`
- `Reset completed, but the server did not return a current session.`

Display rule:

- show the inline message
- do not show `Retry sync`
- keep the current live workbench visible if one is still mounted

These are result notices, not retry invitations.

### Retryable Results

Use retryable inline feedback when the app still has a plausible way to reconcile the current session by fetching current-session state again.

Examples:

- `Created session #X, but the server returned session #Y.`
- `Reset session #X, but the server returned session #Y.`
- `Created a new session, but refreshing it failed: ...`
- `Reset completed, but refreshing it failed: ...`
- `Expected session #X, but the server returned session #Y.`

Display rule:

- show the inline message
- show `Retry sync`
- keep the current live workbench visible while retry remains meaningful

These are alignment failures, not terminal outcomes.

## Retry Sync Semantics

`Retry sync` should appear only when all of the following are true:

- a live or optimistic workbench is still mounted
- the current reconciliation outcome is retryable rather than purely informational
- another current-session refresh could still plausibly align the UI with the expected session

`Retry sync` should not appear when:

- the server returned no current session
- the page has already transitioned into page-level recovery
- create/reset failed before reconciliation began
- the issue is a plain current-session lookup failure rather than a post-action reconciliation outcome

This gives the user a stable rule:

- inline message with no button means “this is the result”
- inline message with `Retry sync` means “the app can still try to realign state”

## Information Hierarchy

The live-shell reconciliation surface should keep three layers.

### 1. Workbench First

The live workbench remains primary whenever it is still usable.

This slice does not replace live-shell reconciliation outcomes with a page-level error shell.

### 2. Result Message Second

The inline message should describe the reconciliation outcome in result-oriented language:

- mismatch between expected and returned session
- refresh failure after create/reset
- no-current-session informational result

### 3. Recovery Action Last

Show `Retry sync` only for retryable results.

Do not pair informational no-current-session outcomes with `Retry sync`.

## Scope Boundary

This slice changes the inline reconciliation behavior in `App.tsx` and the associated tests.

Specifically:

- clarify which inline reconciliation outcomes are informational
- clarify which inline reconciliation outcomes are retryable
- preserve the live workbench while retry remains meaningful
- preserve the absence of `Retry sync` for non-retryable no-current-session results

It does not change:

- page-level recovery shell routing
- `useCurrentSession` API shape
- session create/reset API contracts
- scenario picker component behavior
- terminal-session lifecycle

## Testing

### Unit Coverage

Update frontend tests to verify:

1. mismatch outcomes still show `Retry sync`
2. refresh-failure outcomes still show `Retry sync`
3. no-current-session informational outcomes do not show `Retry sync`
4. create failures before reconciliation still do not show `Retry sync`
5. a successful `Retry sync` clears the inline reconciliation error

### End-to-End Coverage

Keep e2e focused on the existing reconciliation smoke flows:

- create succeeds but current session cannot be confirmed
- refresh failure keeps the current workbench visible

Only adjust assertions if the live-shell wording or button semantics change.

## Success Criteria

This slice is successful when:

1. live-shell reconciliation messages fall into clear informational vs retryable categories
2. `Retry sync` appears only when it is genuinely meaningful
3. informational no-current-session outcomes remain inline notices rather than misleading retry paths
4. the live workbench stays primary while reconciliation feedback is still actionable
