# Scenario Picker Design

## Goal

Add a shared scenario picker modal for practice-session creation so users explicitly choose which scenario to start instead of always launching the first catalog entry implicitly.

This is the next Phase 3 product slice after the catalog API and scenario-driven session creation contract have already landed.

## Scope

This design covers:

- a shared frontend scenario picker modal
- replacing direct `New Session` creation flows with modal-driven selection
- reusing the modal across all session-creation entry points
- scenario selection using the existing catalog payload
- unit and smoke coverage for the new selection flow

This design does not cover:

- backend catalog schema changes
- scenario descriptions or richer scenario metadata
- admin catalog management
- repo-state UX changes
- multi-step onboarding beyond choosing a scenario

## Product Decision

Every `New Session` flow should go through the same modal.

That includes:

- the top-bar `New Session` action while a session is already active
- the authenticated empty-state `New Session`
- the orphaned-workspace recovery `New Session`

The user selects a scenario first, then explicitly confirms with `Start Session`.

This keeps the interaction consistent across the app and removes the current split between implicit default creation and explicit selection.

## Recommended Approach

Use one shared modal component that stays presentation-only, while `App.tsx` continues to own orchestration and session creation.

Why this is the right fit:

- the existing session creation, optimistic update, and reconciliation logic already lives in `App.tsx`
- the picker itself is a bounded UI concern and does not need its own fetch or business logic
- reusing one modal across all entry points prevents state drift and duplicate behavior

## Alternatives Considered

### 1. Inline picker in empty states, modal in top bar

Rejected.

This creates two different mental models for the same action and doubles the testing surface for no product benefit.

### 2. Top-bar dropdown or native select

Rejected.

The picker needs to show `name`, `key`, and `template name`. That is cramped and harder to scan in a compact dropdown.

### 3. Click-to-create without confirmation

Rejected.

The user explicitly asked for selection followed by a separate `Start Session` confirmation. That also reduces accidental launches and leaves room for richer metadata later.

## UX Design

### Modal Entry Points

The following actions open the same modal:

- top-bar `New Session`
- empty-state `New Session`
- orphaned-workspace recovery `New Session`

The button labels stay the same. The only change is that they open the picker instead of creating a session immediately.

### Modal Content

Each scenario row shows:

- scenario `name`
- scenario `key`
- bound template `name`

No description field is shown in this slice.

### Selection Model

- single selection only
- the first available scenario is selected by default when the modal opens
- selecting another scenario updates the highlighted choice
- the user must click `Start Session` to continue

### Modal Actions

- `Cancel` closes the modal without side effects
- `Start Session` creates a session for the selected scenario

### Visual Direction

Keep the existing editorial shell and avoid introducing a separate design language.

The modal should feel like part of the current UI:

- same typography and button language
- clear active selection state
- enough spacing for `name + key + template name` to scan quickly

This slice should not trigger a larger redesign.

## Frontend Architecture

### New Component

Add a new presentation component:

- `ScenarioPickerModal`

Responsibilities:

- render modal chrome
- render the scenario list
- render active selection state
- render modal-local error text
- emit selection, confirm, and close callbacks

It does not:

- fetch catalog data
- create sessions
- own reconcile logic

### App-Level Ownership

`App.tsx` continues to own:

- catalog readiness
- modal open/close state
- selected scenario state
- create-session requests
- optimistic session handling
- reconciliation after create

This keeps orchestration in one place and avoids introducing a new hook with only one caller.

## State Design

### Scenario Picker State

Add a dedicated modal state in `App.tsx`:

- `closed`
- `open` with:
  - `source`: `topbar | empty | orphaned`
  - `selectedScenarioId`
  - `error`

### Open Behavior

When the modal opens:

- require catalog state to be `ready`
- require at least one scenario
- initialize `selectedScenarioId` to the first scenario in catalog order
- clear any previous modal-local error

For this slice, the selection resets each time the modal opens. That is simpler and more predictable than preserving stale modal state across different entry points.

### Confirm Behavior

On `Start Session`:

- reuse the existing `pendingAction = "new-session"`
- call `createPracticeSession({ scenarioId })`
- if creation succeeds:
  - close the modal
  - continue through the existing optimistic session + reconciliation flow
- if creation fails:
  - keep the modal open
  - show the error inside the modal

### Cancel Behavior

On `Cancel`:

- close the modal
- clear modal-local error
- leave outer session state untouched

## Error Handling

### Catalog Not Ready

If catalog is not ready:

- the modal cannot open
- `New Session` triggers should remain disabled

This avoids opening an empty or misleading picker while required data is still loading.

### Empty Catalog

If catalog is ready but has no scenarios:

- continue using the existing empty-catalog shell
- do not show the modal
- do not allow session creation

### Create Failure

If `createPracticeSession` fails from inside the modal:

- keep the modal open
- display the error inside the modal
- allow the user to retry or choose another scenario

This is intentionally different from reconcile failures.

### Reconcile Failure

If creation succeeds but refresh/reconcile fails afterward:

- the modal is already closed
- the error remains a session-level problem
- continue using the existing `actionError` behavior in the page shell

This keeps scenario-choice errors and session-state errors separated instead of mixing them in one surface.

## Data Mapping

The frontend already has:

- `PracticeCatalog.templates`
- `PracticeCatalog.scenarios`

The modal view model should derive template names by joining:

- `scenario.templateId`
- `template.id`

If a template name cannot be found for a scenario, the UI should fall back to a stable label such as `Template #<id>`.

That should be rare because the API already treats broken scenario/template references as server errors, but the UI should still fail safely.

## Testing

### App Tests

Update `App.test.tsx` to cover:

- top-bar `New Session` opens the modal instead of creating immediately
- the modal defaults to the first scenario
- selecting another scenario changes the pending `scenarioId`
- clicking `Start Session` sends the selected `scenarioId`
- create failure stays inside the modal and does not close it
- empty-state `New Session` opens the same modal
- orphaned recovery `New Session` opens the same modal
- catalog-not-ready state does not allow modal opening

### Smoke Tests

Update `apps/web/tests/e2e/smoke.spec.ts` to cover:

- a multi-scenario catalog response
- opening the scenario picker from `New Session`
- changing selection away from the default
- asserting `POST /api/v1/practice-sessions` uses the chosen `scenario_id`

### No Backend Test Expansion

No new backend testing is needed for this slice.

The catalog API and scenario-only creation contract are already covered. This feature is a frontend interaction layer on top of those existing contracts.

## Risks

### Modal Adds Another Layer to Session Creation

Accepted.

This is the intended product change, and the shared modal keeps the complexity contained to one component and one orchestration path.

### Current Catalog Fields Are Sparse

Accepted.

The user explicitly chose `name + key + template name` only. If scenario descriptions are needed later, the modal can expand without changing the overall architecture.

### Selection Reset on Reopen May Feel Basic

Accepted for this slice.

Always resetting to the first scenario is simpler and avoids stale state between entry points. Persistent last-used selection can be considered later if the catalog grows.

## Implementation Order

1. add the modal component and scenario-display view model
2. wire all `New Session` entry points to open the modal
3. move session creation confirmation into the modal flow
4. add unit coverage for the modal-driven selection path
5. update smoke coverage for multi-scenario selection
