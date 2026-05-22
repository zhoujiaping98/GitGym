# Session Unavailable Recovery UX Design

Date: 2026-05-22

## Goal

Turn the page-level `Session unavailable` shell into an intentional recovery surface that distinguishes between retryable lookup failures and unrecoverable no-current-session outcomes.

This slice is about recovery UX and state presentation only. It does not change backend session semantics, catalog APIs, or the existing live-workbench reconciliation behavior.

## Problem

The current `Session unavailable` shell is too generic:

- it treats retryable session lookup failures and recovery-first session loss as the same UX state
- it defaults to `Try again` even when there is no current session left to restore
- it still feels like an engineering error surface instead of a productized recovery page

For the user, the important questions are:

1. Is this a temporary read failure or is my previous session gone?
2. Should I retry the lookup or start a new session?
3. If technical detail exists, where can I still see it?

## Non-Goals

- No changes to backend current-session lookup semantics
- No changes to reconciliation behavior while a live workbench is still visible
- No new APIs
- No changes to scenario picker mechanics
- No changes to `Workspace unavailable`, `Practice catalog unavailable`, or `Practice catalog empty`
- No attempt to fully productize all reconciliation mismatch states in this slice

## Recommended Approach

Split the page-level `Session unavailable` experience by recovery semantics rather than by raw implementation source:

1. keep lookup failures as a retryable shell
2. convert page-level no-current-session reconciliation outcomes into a recovery-first shell
3. keep low-level diagnostics available in the detail area in both cases

This preserves the current control flow while giving each state the correct user-facing next step.

## Recovery Model

### Retryable Session Lookup

When the app fails to read the current session and there is no stronger recovery semantic available, keep the existing lookup shell behavior:

- eyebrow: `Session lookup`
- title: `Session unavailable`
- body: `We could not restore your current practice session.`
- primary action: `Try again`
- detail: preserve the current lookup error text

This remains the transient-failure branch. The app is saying the read failed, not that the session is definitively gone.

### Recovery-First Session Unavailable

When the app has no live workbench to keep on screen and the effective outcome is that the previous current session is no longer available, render a recovery-first shell:

- eyebrow: `Session recovery`
- title: `Session unavailable`
- body: `Your previous practice session is no longer available. Start a fresh session to keep practicing.`
- primary action: `New Session`
- detail: preserve the reconciliation or backend error detail

Clicking `New Session` must open the existing scenario picker flow.

This is the result-oriented branch. The app is no longer asking the user to keep retrying a state that is not meaningfully recoverable in-page.

### Live Workbench Reconciliation Stays Inline

This slice does not change the behavior of reconciliation failures that happen while a usable workbench is still mounted.

If the page can still show a live or optimistic workbench:

- keep the user in the live shell
- keep reconciliation feedback inline
- do not replace the page with the recovery-first `Session unavailable` shell

This avoids collapsing usable state into a full-page error.

## State Boundary

The page-level branching rule should be:

- `currentSession.status === "error"` renders the retryable lookup shell
- `actionError` with no displayed session renders the recovery-first shell
- orphaned-session absence remains handled by the separate `Workspace unavailable` shell
- action errors while a workbench is still displayed remain inline in the live shell

This keeps the boundary user-facing:

- retry when the app cannot yet determine current session state
- start fresh when there is no current session left to restore on the page

## Information Hierarchy

Both `Session unavailable` shells should keep the same hierarchy:

### 1. Outcome First

Lead with the result in user-facing language:

- lookup branch explains that the session could not be restored
- recovery branch explains that the previous session is no longer available

### 2. Recovery Action Second

Use a single primary action per branch:

- `Try again` for retryable lookup failure
- `New Session` for page-level recovery-first session loss

Do not combine both actions in the same shell in this slice.

### 3. Diagnostics Last

Keep low-level error text in the detail area:

- lookup transport or API failures
- reconciliation messages already surfaced by the app
- backend detail describing why the current session could not be restored

Diagnostics remain visible but visually secondary to the outcome and CTA.

## Scope Boundary

This slice changes only the page-level `Session unavailable` rendering in `App.tsx` and the associated tests.

Specifically:

- introduce a recovery-first branch for page-level `actionError` with no displayed session
- preserve the retryable lookup shell for `currentSession.status === "error"`
- preserve the existing live-shell inline reconciliation behavior
- route the recovery-first shell through the existing scenario picker

It does not change:

- `useCurrentSession` API shape
- terminal-session lifecycle
- session reset or create API contracts
- catalog loading behavior
- scenario picker component behavior

## Testing

### Unit Coverage

Update frontend tests to verify:

1. retryable current-session lookup failure still renders `Session unavailable` with `Try again`
2. page-level `actionError` with no displayed session renders recovery-first `Session unavailable` with `New Session`
3. clicking `New Session` from the recovery-first shell opens the existing scenario picker
4. orphaned-session recovery remains unchanged
5. live-shell reconciliation errors remain inline and do not collapse into the page-level recovery shell

### End-to-End Coverage

Only update or add the minimal smoke coverage needed to prove the new page-level recovery branch if an obvious route already exists.

This slice does not require expanding Playwright coverage for every recovery permutation.

## Success Criteria

This slice is successful when:

1. retryable lookup failures still behave like retryable failures
2. page-level no-current-session outcomes now guide the user to start fresh
3. the recovery path reuses the existing scenario picker flow
4. diagnostics remain available without dominating the page
5. the `Session unavailable` shell no longer feels like a generic engineering error page
