# Session Lifecycle Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make expired or missing-workspace practice sessions fail explicitly instead of surfacing as generic terminal/session errors.

**Architecture:** Keep the first slice backend-only. Centralize lifecycle checks in `PracticeService`, teach the runner client to classify missing workspace responses, and persist lifecycle status transitions in the practice session store so `current session`, `reset`, and terminal attach can make consistent decisions.

**Tech Stack:** Go, Chi HTTP handlers, MySQL store, in-memory test store, coder/websocket, Go test.

---

### Task 1: Define lifecycle errors and persistence hooks

**Files:**
- Modify: `services/api/internal/service/practice_service.go`
- Modify: `services/api/internal/store/mysql.go`
- Modify: `services/api/internal/domain/models.go`

- [ ] Add explicit lifecycle errors and status constants for `active`, `expired`, and `orphaned`.
- [ ] Extend the practice session store contract with a status update method that can persist `status`, `ended_at`, and `last_activity_at`.
- [ ] Implement the new update method in both the MySQL store and in-memory test store.

### Task 2: Enforce expiry/orphan transitions in the service layer

**Files:**
- Modify: `services/api/internal/service/practice_service.go`
- Modify: `services/api/internal/test/practice_service_test.go`

- [ ] Write failing tests proving `CurrentPracticeSession` expires stale active sessions and stops returning them as current.
- [ ] Write failing tests proving `ResetPracticeSession` marks the session `orphaned` when the runner reports a missing workspace.
- [ ] Implement lifecycle guards in `CurrentPracticeSession`, `PracticeSessionByID`, and `ResetPracticeSession`.

### Task 3: Classify missing-workspace runner errors and wire terminal attach through the service

**Files:**
- Modify: `services/api/internal/runner/client.go`
- Modify: `services/api/internal/service/practice_service.go`
- Modify: `services/api/internal/http/handlers/terminal_ws.go`
- Modify: `services/api/internal/test/terminal_ws_test.go`

- [ ] Add a runner client error for `workspace not found` and map runner HTTP/WS `404` responses to it.
- [ ] Add a `ConnectTerminal` service method so lifecycle checks and orphaning happen before the websocket bridge starts.
- [ ] Write failing tests proving terminal attach returns an explicit failure when the runner workspace is gone.
- [ ] Update the terminal websocket handler to use the new service entrypoint.

### Task 4: Map lifecycle errors to stable HTTP semantics

**Files:**
- Modify: `services/api/internal/http/handlers/practice_sessions.go`
- Modify: `services/api/internal/test/practice_routes_test.go`

- [ ] Write failing route tests for expired/orphaned session mutations returning `410 Gone`.
- [ ] Keep `GET /practice-sessions/current` behavior user-friendly by treating expired current sessions as absent current sessions.
- [ ] Update the route error mappers so lifecycle failures are distinct from `404 not found` and generic `502/500`.

### Task 5: Verify the backend slice end-to-end

**Files:**
- None

- [ ] Run `go test ./...` in `services/api`.
- [ ] Report any remaining frontend follow-up as a separate next slice instead of bundling it into this change.
