# GitGym Design Spec

Date: 2026-05-16
Topic: GitGym sandbox-first product design
Status: Draft for review

## 1. Summary

GitGym is a web product for practicing Git commands in an isolated, disposable environment. The first release targets developers who want a realistic terminal and repository they can safely experiment in without risking local work.

The product will use real Git repositories for command execution, with platform-owned orchestration, observation, and mode logic around that core. Version 1 will focus on a sandbox-style practice mode, while the architecture will reserve clean extension points for future challenge and incident-simulation modes.

## 2. Product Goal

GitGym should let users:

- create a disposable practice session in the browser
- run real Git commands in an isolated repository
- inspect repository state before and after commands
- reset a broken or messy practice session instantly
- build intuition for Git behavior without touching local work

GitGym should not try to be a full IDE, a complete Git course, or an AI-heavy tutor in version 1.

## 3. Version 1 Scope

Version 1 is a developer-tool-style sandbox, not a guided training system.

Included:

- isolated practice sessions
- realistic terminal interaction
- real Git execution inside a controlled workspace
- a small set of repository templates
- a repository state panel
- command history and basic replayable state records
- one-click session reset

Excluded:

- scoring and grading
- multi-user collaboration
- rich AI tutoring
- custom environment images
- complex challenge authoring
- full incident scripting

## 4. Core Design Decision

GitGym will use a hybrid architecture:

- the execution layer always runs real Git commands in a real repository
- the platform layers manage lifecycle, safety, observation, and product modes

This avoids the inaccuracy of a pure command simulator while preserving room for guided scenarios, validation, replay, and incident recovery flows later.

## 5. System Architecture

The system should be structured as four logical layers.

### 5.1 Session Control

Responsibilities:

- create and destroy practice sessions
- assign repository templates
- manage session timeout and cleanup
- manage mode selection
- apply quotas and access rules

This layer owns the lifecycle of a user session but does not execute commands directly.

### 5.2 Git Execution Engine

Responsibilities:

- create an isolated working directory
- initialize or restore the repository instance from a template
- execute Git and approved shell commands
- return stdout, stderr, exit code, duration, and working directory

This layer is the only layer allowed to interact with the real repository and shell process.

### 5.3 Observation Layer

Responsibilities:

- record every command invocation
- capture command results
- collect repository state before and after execution
- summarize changes to refs, branch state, and working tree state
- persist enough information for replay and later scenario evaluation

This layer must be independent from product modes so the same telemetry can support sandbox, challenge, and incident flows.

### 5.4 Mode Layer

Responsibilities:

- define scenario-specific behavior
- attach completion checks, hints, and restrictions
- interpret observed state without changing the execution model

Version 1 will only ship with sandbox mode, but the layer exists from the start.

## 6. Core Domain Objects

### 6.1 WorkspaceTemplate

Defines the initial repository state for a session.

Examples:

- empty repository
- small repository with branches and commits
- repository containing a merge or rebase problem state

### 6.2 PracticeSession

Represents one user session.

Fields should include:

- session id
- user id or anonymous key
- selected mode
- selected template
- repository path or repository handle
- status: active, expired, resetting, failed, closed
- created time and expiry time

### 6.3 CommandRun

Represents one executed command.

Fields should include:

- command id
- session id
- raw input
- normalized executable and args if parsed
- start and finish time
- stdout
- stderr
- exit code
- duration
- policy decision metadata

### 6.4 RepoSnapshot

Represents a structured summary of repository state at a point in time.

Fields should include:

- HEAD commit and ref
- current branch or detached state
- status summary
- in-progress operation markers such as merge or rebase
- recent commit graph summary
- selected ref summary

Version 1 does not need full filesystem snapshots for every command, but it must capture enough Git state to explain and compare changes.

### 6.5 Scenario

Defines behavior for a mode-specific experience.

Fields should include:

- scenario id
- mode type
- template reference
- rules and limits
- optional completion checks
- optional hints
- optional failure conditions

Sandbox is modeled as a Scenario with minimal rules and no completion objective.

## 7. Command Execution Flow

Every command should follow the same pipeline.

1. User enters a command in the browser terminal.
2. Session Control validates that the session is active.
3. A policy gate determines whether the command is permitted in the sandbox.
4. Observation captures a pre-run repository snapshot.
5. Git Execution Engine runs the command in the isolated workspace.
6. Observation captures stdout, stderr, exit code, duration, and a post-run repository snapshot.
7. The UI renders command output and updated repository state.
8. Mode hooks may evaluate the result and add scenario-specific feedback.

The execution result must always come from real command execution, not simulated Git output.

## 8. Safety Model

The sandbox should be realistic for Git work but strict about platform boundaries.

Allowed by design:

- destructive Git commands inside the isolated repository
- resetting branches
- rebasing, cherry-picking, and creating conflicts
- editing tracked files inside the workspace

Restricted by platform policy:

- access outside the assigned workspace
- unrestricted system commands
- network access
- commands that can affect the host environment or other sessions

The product promise is not "prevent user mistakes." The promise is "mistakes are confined to a disposable workspace."

## 9. User Interface Shape

Version 1 UI should remain focused and tool-like.

Primary areas:

- terminal panel for command entry and output
- repository state panel showing branch, HEAD, status, and recent graph
- session controls for reset, template selection, and restart
- command history view

The interface should feel closer to a lightweight developer utility than a gamified learning product.

## 10. Repository Templates

Version 1 should ship with a narrow set of templates:

- Empty: fresh repository for free exploration
- Standard: a repository with a small history and branch structure
- Recovery: a repository in a broken or tricky state suitable for experimentation

Templates should be declarative and reusable so future scenarios can depend on the same seed states.

## 11. Future Expansion Path

The architecture should support two future modes without replacing the execution core.

### 11.1 Challenge Mode

Adds:

- explicit objective
- completion evaluation
- optional hints
- optional command or time restrictions

Challenge mode reuses:

- the same session model
- the same real Git execution engine
- the same observation records

### 11.2 Incident Mode

Adds:

- failure narrative
- recovery objective
- error-escalation detection
- recovery validation
- post-run explanation or replay

Incident mode also reuses the same execution and observation core.

## 12. Technical Principles

- Real Git is the source of truth for repository behavior.
- Product logic must not fork Git semantics.
- Mode logic must consume observations, not control shell internals directly.
- Templates and scenarios must be separate objects.
- Session reset must be first-class, fast, and reliable.
- Observability is a product feature, not only an engineering concern.

## 13. Technical Stack

Version 1 should use a monorepo, but not a frontend-backend code dump. The codebase should be split by responsibility.

### 13.1 Frontend

- React
- TypeScript
- Vite
- xterm.js for terminal rendering

The frontend is responsible for:

- terminal interaction
- repository state display
- session controls
- command history views

Version 1 frontend target:

- desktop-first web application
- no native Windows application in version 1
- mobile is out of scope beyond basic accessibility

### 13.1.1 Frontend Workspace Layout

Version 1 should use a single-page workbench layout rather than an IDE-style multi-navigation shell.

Recommended layout:

```text
Top Bar
Main Workspace + Right Repository Panel
Collapsible Bottom Command History Panel
```

Detailed structure:

- Top Bar
  - product identity
  - selected repository template
  - session state
  - reset session
  - create new session

- Main Workspace
  - terminal is the primary interaction surface
  - command input and output stay in the main visual focus
  - output must stream in real time

- Right Repository Panel
  - state summary
  - mini commit graph

- Bottom Collapsible Panel
  - command history
  - execution record summaries

### 13.1.2 Repository Panel Content

The right-side repository panel should use a mixed model: state summary plus a small commit graph.

State summary should include:

- current branch
- HEAD reference
- working tree status summary
- in-progress operation markers such as merge, rebase, or cherry-pick

Mini commit graph should include:

- recent commits only
- branch pointer markers
- a compact visual summary rather than a full graph explorer

### 13.1.3 Interaction Model

The screen should reflect three distinct product roles:

- terminal: where the user acts
- repository panel: where the user sees what changed
- command history panel: where the user reviews what happened

This separation is intentional:

- the terminal stays primary
- the repository panel helps interpret Git state
- the history panel supports replay and review without competing with the terminal

### 13.1.4 UI States

The workbench should explicitly support these states:

- Idle
  - terminal accepts input
  - repository panel shows current state

- Running
  - command output streams live
  - terminal input is locked or queued
  - repository panel indicates active execution

- Git Operation In Progress
  - repository panel highlights special states such as merge conflict or rebase in progress

- Session Expired or Failed
  - terminal input is disabled
  - top bar offers reset or new session actions

### 13.1.5 Expansion Readiness

The layout must remain stable as future modes are added.

- Sandbox mode uses the layout with minimal extra UI
- Challenge mode can attach objective cards to the right-side panel
- Incident mode can attach failure context and recovery goals to the same side panel
- Replay and explanation features can extend the bottom panel

This preserves one durable workspace model across all future product modes.

### 13.2 Backend

- Go
- `net/http` as the base HTTP layer
- `chi` as the router and middleware composition layer
- `github.com/coder/websocket` for WebSocket transport
- `os/exec` plus `context` for process execution, timeout, and cancellation
- MySQL as the primary relational database

The backend is selected for long-term service stability, concurrency handling, and cleaner separation of control-plane and execution-plane concerns.

### 13.3 Repository Layout

Recommended structure:

```text
GitGym/
  apps/
    web/
  services/
    api/
    runner/
  contracts/
    openapi/
    events/
  scenarios/
    templates/
    sandbox/
    challenge/
    incident/
  docs/
    superpowers/
      specs/
```

### 13.4 Service Boundaries

`services/api` is the control plane. It should own:

- session lifecycle
- authentication and quotas
- template and scenario selection
- browser-facing HTTP and WebSocket endpoints
- orchestration of runner instances

`services/runner` is the execution plane. It should own:

- workspace creation
- repository initialization and reset
- command execution
- repository observation and snapshot collection
- cleanup and timeout enforcement

The code should preserve this boundary from the start even if both services are initially deployed together.

### 13.5 Contract Strategy

Because the frontend and backend use different languages, shared TypeScript types are not the right integration model.

The recommended contract strategy is:

- OpenAPI for browser-facing API contracts
- explicit event schemas for command output, session state, and replay records
- generated frontend client types from the published API contract

This keeps the system language-agnostic and makes future service extraction easier.

### 13.6 Authentication

Version 1 includes login.

Recommended version 1 authentication model:

- GitHub OAuth as the primary sign-in method
- application-managed user session after successful OAuth callback

Why this model fits:

- the primary audience is developers
- it avoids building a password system in version 1
- it aligns well with future training, history, and account-bound personalization

Version 1 should not include:

- local username and password accounts
- multi-provider identity federation
- team or organization role models beyond simple user ownership

### 13.7 Persistence Model

The database stores product metadata and execution records, not full Git repositories.

Store in MySQL:

- users and login linkage
- browser session records
- workspace template definitions
- scenario definitions
- practice session metadata
- command execution records
- repository snapshot summaries
- replay or audit event records
- reset history

Do not store in MySQL:

- full repository contents
- live working directories
- raw repository templates as unpacked worktrees

Those belong in runner-managed storage on disk or in a later object-storage layer.

### 13.8 Core Tables

#### users

Purpose:

- one row per product user

Suggested fields:

- id
- github_user_id
- github_login
- display_name
- avatar_url
- email if available
- created_at
- updated_at
- last_login_at
- status

#### auth_accounts

Purpose:

- identity provider linkage

Suggested fields:

- id
- user_id
- provider
- provider_account_id
- provider_username
- access_token metadata if retained
- refresh_token metadata if retained
- created_at
- updated_at

Version 1 may only populate GitHub rows, but the table should still model provider linkage cleanly.

#### user_sessions

Purpose:

- browser or app login session tracking

Suggested fields:

- id
- user_id
- session_token_hash
- user_agent
- ip_address summary
- expires_at
- created_at
- revoked_at

#### workspace_templates

Purpose:

- defines reusable starting repository templates

Suggested fields:

- id
- key
- name
- description
- mode_compatibility
- source_ref
- difficulty_level
- is_active
- created_at
- updated_at

#### scenarios

Purpose:

- shared abstraction for sandbox, challenge, and incident modes

Suggested fields:

- id
- key
- mode_type
- template_id
- name
- description
- rules_json
- completion_rules_json
- hint_policy_json
- is_active
- created_at
- updated_at

Sandbox scenarios can use minimal rule payloads.

#### practice_sessions

Purpose:

- one user practice session bound to one scenario and one live repository instance

Suggested fields:

- id
- user_id
- scenario_id
- template_id
- runner_id or runner_ref
- workspace_path_ref
- status
- started_at
- expires_at
- ended_at
- last_activity_at

#### command_runs

Purpose:

- one row per executed terminal command

Suggested fields:

- id
- practice_session_id
- sequence_no
- raw_command
- executable
- args_json
- cwd_ref
- policy_decision
- exit_code
- duration_ms
- stdout_preview
- stderr_preview
- started_at
- finished_at

Large raw output can be truncated in-table and stored elsewhere later if needed.

#### repo_snapshots

Purpose:

- structured Git state summaries associated with command execution

Suggested fields:

- id
- practice_session_id
- command_run_id
- snapshot_phase
- head_ref
- head_commit
- branch_name
- detached_head
- status_summary_json
- operation_state_json
- recent_graph_json
- refs_summary_json
- captured_at

The expected `snapshot_phase` values are typically pre-run and post-run.

#### session_events

Purpose:

- audit and replay oriented event stream

Suggested fields:

- id
- practice_session_id
- command_run_id nullable
- event_type
- event_payload_json
- created_at

Examples include:

- session_created
- session_reset
- command_started
- command_finished
- session_expired
- runner_reassigned

#### session_resets

Purpose:

- tracks explicit reset actions for audit and product analysis

Suggested fields:

- id
- practice_session_id
- user_id
- reason_code
- source_snapshot_id nullable
- created_at

### 13.9 Persistence Principles

- Live repository state stays outside MySQL.
- MySQL stores metadata, summaries, and audit history.
- Session-to-user relationships must be explicit.
- Scenario and template definitions must be reusable across many sessions.
- Snapshot records should stay structured enough for query and replay use.

## 14. Testing Strategy

Version 1 needs testing at three levels.

### 14.1 Unit Tests

- policy gate behavior
- scenario evaluation hooks
- repository snapshot parsers
- session lifecycle logic

### 14.2 Integration Tests

- template creation and reset
- command execution pipeline
- observation before and after command runs
- cleanup and expiry behavior

### 14.3 End-to-End Tests

- start a session from the web UI
- run commands and inspect output
- view repository state updates
- reset and confirm repository restoration

The most important test priority is ensuring the platform remains consistent while running real Git workflows repeatedly.

## 15. Risks and Mitigations

### 15.1 Over-designing for future modes

Risk:
Version 1 slows down by trying to fully build challenge and incident infrastructure.

Mitigation:
Ship only sandbox behavior in version 1, but keep Scenario as the shared abstraction.

### 15.2 Weak isolation

Risk:
Terminal realism exposes the host environment.

Mitigation:
Constrain the execution environment strictly to the workspace and a narrow approved command surface.

### 15.3 Poor repository observability

Risk:
The product feels like a raw terminal with little added value.

Mitigation:
Invest early in command recording and repository state summaries.

### 15.4 Template sprawl

Risk:
Too many templates create maintenance cost before user value is proven.

Mitigation:
Start with three templates only and expand based on usage.

## 16. Recommended Build Order

1. Session lifecycle and workspace creation
2. Real command execution in isolated repositories
3. Repository snapshot collection
4. Minimal UI terminal plus repository state panel
5. Command history and reset
6. Template system cleanup and scenario abstraction hardening

## 17. Decision Record

Confirmed decisions from brainstorming:

- Version 1 target experience: developer tool
- Execution model: hybrid architecture with real Git underneath
- First mode to ship: sandbox practice
- Long-term expansion target: challenge mode and incident simulation
- Frontend stack: React + TypeScript + Vite
- Backend stack: Go + net/http + chi + coder/websocket + MySQL
- Backend boundary: separate api control plane and runner execution plane
- Contract approach: API and event schemas, not shared language-specific types
- Authentication model: GitHub OAuth for version 1

## 18. Open Items Deferred Beyond This Spec

These are intentionally left for implementation planning:

- storage backend choice
- containerization strategy versus process sandboxing
- authentication and billing model

Those decisions matter, but they should be resolved in the implementation plan rather than inside the product design spec.
