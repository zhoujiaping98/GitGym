# Repo State UX Design

Date: 2026-05-21

## Goal

Turn the current right-side `Repository` panel from a raw metadata dump into an operational session status card that answers three questions quickly:

1. What sandbox am I in?
2. Is it healthy right now?
3. What runner/workspace/session is this tied to?

This phase does not add new backend data. It only reorganizes and presents data already available in the web app from:

- `PracticeSession`
- terminal session state
- practice catalog names for scenarios/templates

## Non-Goals

- No new API routes
- No repo snapshot backend work (`branch`, `dirty`, `recent changes`)
- No command history redesign in this slice
- No page-level lifecycle recovery flow changes
- No changes to `orphaned` / `missing` empty-state shells outside the existing app state flow

## Current Problem

The current `Repository` side panel is technically accurate but product-weak:

- it foregrounds IDs instead of meaning
- it mixes operational health and metadata without hierarchy
- it reads like debug output rather than a status card
- it does not clearly separate current health from chronological facts

Users need an at-a-glance operational summary, not a list of raw fields.

## Recommended Approach

Implement a single operational session status card that replaces the current repository metadata layout while keeping the surrounding workbench structure intact.

Key principles:

1. `Name first, ids second`
   Show scenario/template human-readable names first. Keep `session #id` and runner IDs secondary.

2. `Health first, chronology second`
   Lead with session health and terminal transport state. Put timestamps later.

3. `Active session only`
   This card is responsible for expressing session state only when a session exists. Page-level unavailable/orphaned states remain page-level shells.

## Information Architecture

The card is split into three layers.

### 1. Status Header

Purpose: answer "is this workspace usable right now?"

Content:

- primary status badge:
  - `Live`
  - `Recovering`
  - `Unavailable`
- primary title:
  - scenario display name
- secondary subtitle:
  - template display name

Status mapping:

- `Live`
  - session exists
  - terminal state is healthy/interactive/connecting enough to treat the workspace as attached
- `Recovering`
  - session exists
  - terminal state is temporarily unavailable or reattaching
- `Unavailable`
  - reserved for active-session renders where terminal state cannot currently attach and should read as degraded

This slice should not invent new session-level lifecycle states beyond what the app already knows.

### 2. Operational Facts

Purpose: answer "what exact environment is this?"

Facts shown:

- template name
- runner ref
- workspace path
- session id

Display rules:

- Use readable labels, not inline debug text
- Keep IDs secondary in visual hierarchy
- Workspace path should remain visible because it is operationally useful for this product

### 3. Lifecycle Strip

Purpose: answer "when was this created / last active / due to expire?"

Fields:

- started
- last activity
- expires
- terminal state

Display rules:

- chronology is presented after health and facts
- terminal state is included here as a transport-level status detail
- if timestamps are unavailable, render a clear fallback such as `Unavailable`

## Data Sources

The UI continues to derive values from existing client state:

- scenario name:
  - from catalog lookup by `session.scenarioId`
- template name:
  - from catalog lookup by `session.templateId`
- session id:
  - from `PracticeSession.id`
- runner ref:
  - from `PracticeSession.runnerRef`
- workspace path:
  - from `PracticeSession.workspacePath`
- started:
  - from `PracticeSession.startedAt`
- last activity:
  - from `PracticeSession.lastActivityAt`
- expires:
  - from `PracticeSession.expiresAt`
- terminal state:
  - from `useTerminalSession`

No additional fetching is introduced.

## State Behavior

### Active Session

Show the full operational card.

### Terminal Degraded While Session Exists

Do not replace the whole workbench with a special panel.

Instead:

- keep the card visible
- downgrade the status header to `Recovering` or `Unavailable`
- visually emphasize the terminal state row

This preserves context while making the degradation legible.

### Preview Mode

Preview mode keeps a simplified shell, not fake operational details.

Rules:

- preserve the right-rail structure and general layout
- do not fabricate runner refs, workspace paths, or timestamps
- render a lightweight placeholder version of the card so the preview remains visually aligned with the live workbench

### Page-Level Missing / Orphaned / Signed-Out States

No change.

Those remain page-level shells in `App.tsx`, outside this component's responsibility.

## Component Boundary

The existing right-side repository panel component should be reshaped into a focused operational card component rather than absorbing broader lifecycle logic.

Responsibilities:

- render current session operational information
- map terminal/session/catalog data into readable display fields
- handle preview rendering

Non-responsibilities:

- fetch or refresh session state
- own recovery actions
- decide page-level unavailable shells
- redesign command history

## Visual Direction

The card should feel operational, not decorative:

- strong status indicator at top
- clean fact rows with disciplined label/value rhythm
- clear grouping between health, environment facts, and lifecycle
- maintain compatibility with current workbench visual language

This is a productization pass, not a full visual redesign of the workbench.

## Testing

### Unit / Component Coverage

Update frontend tests to verify:

- active session renders readable scenario/template names
- operational facts render runner/workspace/session fields
- lifecycle strip renders time fields and terminal state
- degraded terminal state changes card status without removing workbench context
- preview mode renders placeholder shell without fake operational values

### E2E Expectations

No new backend dependencies are required for e2e.

Existing smoke coverage should continue to pass after the UI update, with targeted assertion updates only if text structure changes.

## Implementation Notes

- Follow existing app structure and keep this slice mostly within the web app
- Prefer targeted refactoring over broad workbench redesign
- If current repository panel code is doing too many jobs, split presentation helpers/components rather than expanding one file further

## Open Decisions Resolved

Resolved for this slice:

- prioritize session context over command history
- operational tone over instructional tone
- show names first and IDs second
- keep page-level recovery flows outside the card
- use existing data only

## Success Criteria

This slice is successful when:

1. the right-side session card immediately communicates workspace health
2. users can identify the current scenario/template without reading raw IDs
3. runner/workspace/session details remain available for operational debugging
4. degraded terminal attachment is visible without collapsing the whole workbench
5. no backend work is required to ship the improvement
