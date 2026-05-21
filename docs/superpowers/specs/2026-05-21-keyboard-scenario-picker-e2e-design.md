# Keyboard Scenario Picker E2E Design

## Goal

Add one browser-level smoke test that proves the shared scenario picker works correctly for keyboard users:

- the user can move selection from the default scenario to a non-default scenario with the keyboard
- focus stays trapped inside the modal while it is open
- confirming the modal submits the selected non-default `scenario_id`

## Scope

This spec only covers Playwright smoke coverage in `apps/web/tests/e2e/smoke.spec.ts`.

It does not change production code unless the new browser test exposes a real runtime bug.

## Why This Exists

The current scenario picker is already covered by:

- RTL unit tests for keyboard selection and focus trapping
- Playwright smoke tests for mouse-driven selection

That still leaves one gap: real browser coverage does not yet verify the keyboard-only path. This spec closes that gap with one targeted smoke test.

## Recommended Approach

Add a new standalone Playwright smoke test in `smoke.spec.ts` rather than rewriting the existing happy-path test.

This keeps the current mouse-based smoke path intact while making the keyboard-specific behavior explicit and easy to diagnose when it fails.

## Test Design

### Setup

Reuse the existing suite-level test setup:

- terminal websocket stub
- `/api/v1/templates` route mock
- two-scenario catalog payload

Start from an active session state so the test can open the picker from the top-bar `New Session` button. This avoids mixing keyboard-picker assertions with authenticated empty-state auto-open behavior.

### Interaction Flow

The new smoke test should:

1. Open the page with an active current session
2. Click `New Session`
3. Assert the `Choose a practice scenario` dialog is visible
4. Verify the default scenario is initially focused/selected
5. Use the keyboard to move selection to the second scenario
6. Verify the second scenario becomes selected
7. Exercise `Shift+Tab` and `Tab` while the modal is open
8. Verify focus remains inside the dialog and does not escape to background controls such as top-bar actions
9. Confirm the modal with the keyboard path
10. Assert the create-session request body is `{ scenario_id: 2 }`
11. Assert the app returns to the live workbench after reconciliation

## Assertions

### Dialog assertions

- the picker dialog is visible
- the second scenario option becomes the selected option after keyboard navigation

### Focus assertions

- `Shift+Tab` from the first selectable point wraps to the last focusable control in the dialog
- `Tab` continues to cycle within dialog controls
- focus never lands on background controls while the dialog remains open

### Network assertions

- the `POST /api/v1/practice-sessions` request body equals `{ scenario_id: 2 }`

### Post-confirmation assertions

- the live workbench is visible again
- reconciliation reflects the second scenario path, consistent with existing multi-scenario smoke behavior

## Test Boundaries

### In scope

- keyboard scenario selection
- modal focus trap behavior
- selected `scenario_id` submission

### Out of scope

- terminal command-entry behavior
- empty-state auto-open behavior
- rewriting existing mouse-based smoke coverage
- adding extra accessibility product changes beyond what the browser test requires

## Risks And Mitigations

### Risk: brittle focus assertions

Focus assertions can become noisy if they depend on incidental markup order.

Mitigation:

- assert against known in-dialog controls and against the absence of focus on specific background controls
- keep the flow targeted to the current modal contract

### Risk: duplicate setup logic

Keyboard flow could duplicate the mouse-driven scenario-picker setup already in the file.

Mitigation:

- extract or reuse a small local helper only if it improves readability without hiding the core interaction

## Success Criteria

This work is complete when:

- `smoke.spec.ts` includes one keyboard-driven scenario-picker smoke test
- the test proves non-default keyboard selection and focus trapping in a real browser
- `pnpm --dir apps/web run test:e2e` passes
