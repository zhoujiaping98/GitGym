# Catalog Empty State Design

Date: 2026-05-21

## Goal

Turn the current `Practice catalog empty` shell into a clearer administrative empty state that explains why session creation is unavailable in this environment without implying a transient failure.

This slice is about configuration-empty messaging only. It does not change catalog loading, session creation, or retry semantics.

## Problem

The current empty-catalog shell is serviceable but still reads like a generic system state rather than a productized administrative empty state.

For the user, the important distinction is:

- this is not a failed request
- this environment simply does not have any published practice scenarios yet

The page should make that configuration reality obvious.

## Non-Goals

- No changes to catalog loading behavior
- No changes to session creation flow
- No changes to scenario picker behavior
- No retry action
- No backend API changes
- No new administrator workflow inside the app

## Recommended Approach

Represent `Practice catalog empty` as an administrative empty state with no primary CTA.

The shell should:

1. explain that no practice scenarios are currently published for this environment
2. give a clear administrator-oriented next step
3. avoid offering actions that imply the condition is transient or user-fixable

## Interaction Model

### No Primary CTA

This state should not render:

- `Try again`
- `New Session`
- a disabled session-creation button

Reason:

- empty catalog is not a transient load failure
- retry is not a meaningful user action here
- session creation should not appear available when the required catalog data is absent

## Information Hierarchy

The shell should have two clear layers.

### 1. Outcome First

Title remains:

- `Practice catalog empty`

Primary body becomes:

- `This environment doesn’t have any published practice scenarios yet.`

This frames the state as a missing configuration/content condition, not an error.

### 2. Administrative Guidance

Detail becomes:

- `Ask an administrator to publish at least one scenario before creating a session.`

This gives the user a concrete next step without pretending there is an in-product recovery action available.

## Scope Boundary

This slice changes only the `hasEmptyCatalogState` shell rendering in `App.tsx` and the associated tests.

Specifically:

- update the empty-state body copy
- preserve the page-level shell pattern
- keep the state CTA-free

It does not change:

- catalog fetch behavior
- `retryCatalogLoad()`
- `useCurrentSession`
- authenticated empty-state auto-create logic
- scenario picker logic
- session creation logic

## Testing

### Unit Coverage

Update frontend tests to verify:

1. empty catalog state renders the new administrative body copy
2. the administrator guidance detail remains visible
3. the shell does not render `Try again`
4. the shell does not render `New Session`

### End-to-End Coverage

Do not add a new empty-catalog smoke flow in this slice.

Only update existing assertions if any already depend on the old body/detail copy. If none do, e2e remains unchanged.

## Success Criteria

This slice is successful when:

1. users can clearly tell this is a catalog-content/configuration absence, not a failed request
2. the page does not suggest futile retry or creation actions
3. the next step is explicit: an administrator must publish at least one scenario
