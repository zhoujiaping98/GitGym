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

## 13. Testing Strategy

Version 1 needs testing at three levels.

### 13.1 Unit Tests

- policy gate behavior
- scenario evaluation hooks
- repository snapshot parsers
- session lifecycle logic

### 13.2 Integration Tests

- template creation and reset
- command execution pipeline
- observation before and after command runs
- cleanup and expiry behavior

### 13.3 End-to-End Tests

- start a session from the web UI
- run commands and inspect output
- view repository state updates
- reset and confirm repository restoration

The most important test priority is ensuring the platform remains consistent while running real Git workflows repeatedly.

## 14. Risks and Mitigations

### 14.1 Over-designing for future modes

Risk:
Version 1 slows down by trying to fully build challenge and incident infrastructure.

Mitigation:
Ship only sandbox behavior in version 1, but keep Scenario as the shared abstraction.

### 14.2 Weak isolation

Risk:
Terminal realism exposes the host environment.

Mitigation:
Constrain the execution environment strictly to the workspace and a narrow approved command surface.

### 14.3 Poor repository observability

Risk:
The product feels like a raw terminal with little added value.

Mitigation:
Invest early in command recording and repository state summaries.

### 14.4 Template sprawl

Risk:
Too many templates create maintenance cost before user value is proven.

Mitigation:
Start with three templates only and expand based on usage.

## 15. Recommended Build Order

1. Session lifecycle and workspace creation
2. Real command execution in isolated repositories
3. Repository snapshot collection
4. Minimal UI terminal plus repository state panel
5. Command history and reset
6. Template system cleanup and scenario abstraction hardening

## 16. Decision Record

Confirmed decisions from brainstorming:

- Version 1 target experience: developer tool
- Execution model: hybrid architecture with real Git underneath
- First mode to ship: sandbox practice
- Long-term expansion target: challenge mode and incident simulation

## 17. Open Items Deferred Beyond This Spec

These are intentionally left for implementation planning:

- exact backend framework
- exact terminal transport mechanism
- storage backend choice
- containerization strategy versus process sandboxing
- authentication and billing model

Those decisions matter, but they should be resolved in the implementation plan rather than inside the product design spec.
