# Catalog API Design

## Goal

Replace hardcoded practice templates and scenarios with a real catalog source in the API, while keeping local development working through a built-in fallback catalog. Update the frontend to load catalog data and create sessions by selecting a scenario only.

This slice is the first Phase 3 productization step after terminal stability and session lifecycle hardening.

## Scope

This design covers:

- API-side catalog loading for practice templates and scenarios
- MySQL-backed catalog lookup with built-in fallback when no database-backed catalog store is available
- browser-facing catalog response shape
- session creation driven by `scenario_id` only
- frontend catalog loading and scenario-driven session creation
- regression coverage for API, frontend, and smoke flows

This design does not cover:

- catalog authoring or admin CRUD
- multiple scenario groups or pagination
- user-facing scenario descriptions beyond the minimum fields already needed by the UI
- repo-state UX beyond existing session metadata

## Product Decision

The user selects a scenario, not a template.

Templates remain a reusable internal concept because scenarios still bind to templates, and practice sessions still persist both `scenario_id` and `template_id`. But the browser no longer sends `template_id` during session creation, and the UI no longer presents template selection as a user choice.

This keeps the model aligned with the product:

- users start a named scenario
- the service decides which template that scenario uses
- future scenario-specific behavior can expand without reopening the session creation contract

## Recommended Approach

Use a catalog provider abstraction inside the API service:

- if a MySQL-backed catalog store is available, it is the source of truth
- otherwise, the API falls back to a small built-in catalog matching current local behavior

The API service continues to own browser-facing session orchestration. The runner still only receives the resolved template key when a workspace is created.

## Alternatives Considered

### 1. MySQL only, no fallback

Rejected.

This would make local development and test setup more brittle than the project currently needs. The current repo already supports in-memory/local paths for development, and removing that would slow iteration without product benefit.

### 2. Keep scenario and template as separate browser inputs

Rejected.

That preserves a false choice in the UI and allows invalid combinations that the product model does not want. It also keeps the API contract coupled to implementation details instead of user intent.

### 3. File-seeded catalog instead of MySQL-backed catalog

Rejected for this slice.

The project already has MySQL-backed product state and planned catalog tables. Phase 3 should move toward real persisted catalog reads, not add another temporary runtime source.

## Data Model

The API works with two catalog objects:

### Practice Template

- `id`
- `key`
- `name`

### Practice Scenario

- `id`
- `key`
- `name`
- `template_id`

For this slice, those fields are enough for:

- validating session creation input
- resolving a scenario to its template
- rendering a scenario picker in the browser

If scenario descriptions or badges are needed later, they can be added without changing the core approach.

## API Service Design

### Catalog Provider Boundary

Introduce a catalog-reading boundary in the API service layer instead of embedding template/scenario lists directly in `practiceService`.

Responsibilities:

- list templates
- list scenarios
- look up template by ID
- look up scenario by ID

The practice service should depend on this boundary for validation and resolution. It should no longer own hardcoded `templates` and `scenarios` slices.

### Built-in Fallback Catalog

Provide a built-in fallback catalog that preserves current behavior:

- template:
  - `id=1`
  - `key=standard`
  - `name=Standard`
- scenario:
  - `id=1`
  - `key=sandbox-standard`
  - `name=Standard Sandbox`
  - `template_id=1`

This fallback is used when the dependencies available to the router do not expose a real catalog store.

### MySQL Catalog Source

When MySQL is configured and the store supports catalog reads, the API should read:

- active rows from `workspace_templates`
- active rows from `scenarios`

Expected behavior:

- templates ordered deterministically, preferably by `id`
- scenarios ordered deterministically, preferably by `id`
- inactive rows excluded
- scenarios whose `template_id` does not resolve to an active template treated as invalid data and excluded or surfaced as a store error

For this slice, the safer behavior is to treat broken catalog joins as a store error so the inconsistency is visible instead of silently serving partial catalog data.

### Practice Service Behavior

Change practice session creation so it accepts only:

- `user_id`
- `scenario_id`

Flow:

1. validate input
2. load scenario from catalog provider
3. load template referenced by the scenario
4. call runner `CreateWorkspace` with the template key
5. persist practice session with both `scenario_id` and resolved `template_id`

Error cases:

- unknown scenario -> `ErrUnknownPracticeScenario`
- scenario references missing template -> configuration/store error
- catalog provider unavailable -> `ErrPracticeServiceConfiguration`

`ErrUnknownPracticeTemplate` can remain for internal service validation or route compatibility, but browser-driven session creation should no longer rely on user-supplied template IDs.

## Browser API Design

### GET `/api/v1/templates`

Keep the route for compatibility, but expand the response into a catalog payload:

```json
{
  "templates": [
    { "id": 1, "key": "standard", "name": "Standard" }
  ],
  "scenarios": [
    { "id": 1, "key": "sandbox-standard", "name": "Standard Sandbox", "template_id": 1 }
  ]
}
```

This avoids a route rename while giving the browser the scenario list it actually needs.

### POST `/api/v1/practice-sessions`

Change the request body to:

```json
{
  "scenario_id": 1
}
```

The response shape remains unchanged and still includes:

- `scenario_id`
- `template_id`

That preserves current session rendering paths while removing template choice from the request.

### Compatibility Policy

Do not keep a long-lived dual-input contract.

This repo controls both frontend and backend together, and the user explicitly wants the frontend switched to scenario-driven creation. The API and frontend should move in one slice with test coverage instead of carrying compatibility complexity forward.

## Frontend Design

### Catalog Loading

The web app should load catalog data during authenticated app bootstrap.

State requirements:

- catalog loading
- catalog ready
- catalog load error

The app should not auto-create a practice session until catalog data is ready.

### Auto-Create Behavior

Current behavior guesses `scenario_id=1` and `template_id=1`.

Replace that with:

- wait for catalog load
- choose the default scenario from the loaded scenario list
- create a session using that scenario ID only

For this slice, the default scenario can be the first scenario in deterministic catalog order.

### New Session Flow

`New Session` should create a new session using the selected scenario.

For the first slice, a minimal UI is sufficient:

- if only one scenario exists, use it directly
- if multiple scenarios exist, expose a small scenario picker in the existing shell before or during new-session creation

Given the current repo state and the goal to avoid unnecessary UI churn, the recommended first version is:

- use the first scenario automatically
- surface the scenario name in the shell
- defer richer picker UX to a later product slice

This keeps the browser aligned with scenario-driven creation without forcing a bigger interface redesign in the same change.

### Error Handling

If catalog loading fails:

- do not treat it as a missing session
- show a dedicated catalog/load failure state
- allow retry

If catalog loads but contains no scenarios:

- show a dedicated empty-catalog state
- block session creation

## Router and Dependency Wiring

The router should assemble the practice service with:

- a real catalog-backed store when available from MySQL dependencies
- otherwise a fallback catalog provider

The fallback decision belongs at dependency assembly time, not scattered inside route handlers.

This keeps the service logic deterministic and testable.

## Testing

### API Tests

Add or update coverage for:

- list catalog returns fallback templates and scenarios when only fallback is available
- list catalog returns MySQL-backed templates and scenarios when store supports catalog reads
- create session route accepts `scenario_id` only
- missing `scenario_id` returns `400`
- scenario lookup resolves template key correctly before runner workspace creation
- unknown scenario returns `400`
- broken scenario/template catalog data returns configuration or server error

### Frontend Tests

Add or update coverage for:

- catalog loads before auto-create runs
- auto-create uses loaded default scenario and does not require template input
- new session action uses scenario-driven request payload
- catalog load failure renders a dedicated error state
- empty catalog renders a blocked state instead of a misleading session error

### Smoke Tests

Update smoke coverage so the mocked create-session payload matches the new contract:

- create session with `scenario_id`
- current session still restores normally
- live terminal flows remain unaffected

## Risks

### Catalog route name is slightly misleading

Accepted for this slice.

Keeping `GET /api/v1/templates` avoids route churn while the response expands to include scenarios. If needed later, a clearer route such as `/api/v1/catalog` can be added with explicit migration.

### Fallback catalog can diverge from MySQL seed data

Accepted for this slice.

The fallback catalog is intentionally tiny and should match the seeded default path. If divergence becomes a problem, fallback data can later be generated from shared seed definitions.

### Frontend default-scenario selection is simplistic

Accepted for this slice.

Choosing the first scenario is enough while there is effectively one default sandbox. A richer picker can come later if more scenarios are added.

## Implementation Order

1. add catalog provider boundary and fallback catalog in the API service layer
2. add MySQL-backed catalog reads
3. update list-catalog and create-session routes to the new contract
4. update frontend catalog loading and scenario-driven session creation
5. update unit and smoke coverage
