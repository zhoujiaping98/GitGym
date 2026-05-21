# Catalog Unavailable Recovery UX Design

Date: 2026-05-21

## Goal

Turn the current `Practice catalog unavailable` page-level shell into a clearer recovery state that explains the outcome in product language, offers a focused retry action, and keeps low-level error detail secondary.

This slice is about catalog-error communication only. It does not change session loading, picker behavior, or retry semantics outside the catalog request.

## Problem

The current catalog error shell is functional but still reads like an implementation failure state:

- it emphasizes the failed load more than the user-facing consequence
- it does not clearly separate the main outcome from the technical detail
- it risks feeling like a generic app error instead of an intentional recovery surface

For the user, the important questions are:

1. What is unavailable right now?
2. Can I retry?
3. Is the environment broken, or just the catalog lookup?

## Non-Goals

- No changes to `useCurrentSession`
- No changes to session refresh behavior
- No changes to scenario picker behavior
- No changes to `retryCatalogLoad()` semantics
- No automatic retries
- No backend API changes

## Recommended Approach

Keep the catalog failure as a page-level shell, but rewrite it as a catalog-specific recovery surface:

1. explain that the list of available practice scenarios could not be loaded
2. keep `Try again` as the single recovery action
3. ensure `Try again` only retries catalog loading
4. keep low-level diagnostics visible but secondary

This preserves the existing control flow while making the failure mode feel intentional and scoped.

## Recovery Model

### Retry Semantics

`Try again` must remain catalog-only.

It should:

- re-trigger the catalog load

It should not:

- refresh the current session
- reopen the scenario picker
- auto-create a session
- retry unrelated app state

This is important because the user is explicitly retrying the practice catalog, not the whole workbench bootstrap.

## Information Hierarchy

The shell should have three layers.

### 1. Outcome First

Title remains:

- `Practice catalog unavailable`

Primary body becomes:

- `We couldn’t load the available practice scenarios for this environment.`

This keeps the message user-facing and specific to the missing catalog, not to internal API mechanics.

### 2. Recovery Action Second

Primary CTA remains:

- `Try again`

The button should keep its existing behavior and continue to call the current catalog reload path only.

### 3. Diagnostics Last

The existing low-level error text remains in the detail area.

Examples:

- `api offline`
- transport/proxy/load failures already surfaced by the app

Display rule:

- diagnostics remain visible
- diagnostics are visually secondary to title and CTA

## Scope Boundary

This slice changes only the catalog-error shell rendering in `App.tsx` and the associated tests.

Specifically:

- update the catalog-error body copy
- preserve the existing title
- preserve the `Try again` action
- preserve the existing detail source

It does not change:

- `retryCatalogLoad()`
- `useCurrentSession`
- authenticated empty-state behavior
- scenario picker logic
- session creation logic

## Testing

### Unit Coverage

Update frontend tests to verify:

1. catalog error state renders the new result-oriented body copy
2. the low-level detail text still appears
3. clicking `Try again` triggers catalog reload behavior only
4. `Try again` does not trigger `currentSession.refresh()`

### End-to-End Coverage

Do not add a new catalog-error smoke flow in this slice.

Only update existing e2e assertions if they already depend on the old wording. If no such assertions exist, e2e remains unchanged.

## Success Criteria

This slice is successful when:

1. the catalog error page clearly explains what is unavailable
2. the retry action remains single-purpose and predictable
3. diagnostics remain visible without dominating the shell
4. users are not led to think the entire workbench bootstrap is being retried
