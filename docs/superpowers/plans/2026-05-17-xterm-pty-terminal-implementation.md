# Xterm PTY Terminal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the current read-only terminal placeholder with a real `xterm.js` terminal backed by a long-lived shell/PTy per workspace, so users can type and execute Git and shell commands inside their disposable practice repository.

**Architecture:** The browser owns terminal rendering through `xterm.js` and communicates over a persistent WebSocket to the API. The API becomes a WebSocket bridge and authorization gate only; it forwards terminal traffic to the runner. The runner owns the real interactive shell lifecycle per workspace, including PTY creation, input/output streaming, resize, exit handling, and cleanup.

**Tech Stack:** React, TypeScript, Vite, `@xterm/xterm`, `@xterm/addon-fit`, Go, `github.com/coder/websocket`, `github.com/creack/pty`, Windows PowerShell, Vitest, Go test.

---

## File Structure

### Frontend

- Modify: `apps/web/package.json`
- Modify: `apps/web/src/types.ts`
- Modify: `apps/web/src/hooks/useTerminalSession.ts`
- Modify: `apps/web/src/components/TerminalPanel.tsx`
- Modify: `apps/web/src/components/Workbench.tsx`
- Modify: `apps/web/src/test/useTerminalSession.test.tsx`
- Modify: `apps/web/src/test/App.test.tsx`
- Create: `apps/web/src/lib/terminal-protocol.ts`

### API

- Modify: `services/api/internal/runner/client.go`
- Modify: `services/api/internal/http/handlers/terminal_ws.go`
- Modify: `services/api/internal/http/router.go`
- Modify: `services/api/internal/service/practice_service.go`
- Create: `services/api/internal/test/terminal_ws_test.go`

### Runner

- Modify: `services/runner/go.mod`
- Modify: `services/runner/internal/http/router.go`
- Modify: `services/runner/internal/http/handlers/commands.go`
- Create: `services/runner/internal/http/handlers/terminal_ws.go`
- Create: `services/runner/internal/engine/terminal_sessions.go`
- Create: `services/runner/internal/engine/terminal_protocol.go`
- Create: `services/runner/internal/test/terminal_ws_test.go`
- Create: `services/runner/internal/test/terminal_sessions_test.go`

### Docs

- Modify: `README.md`

---

## Task 1: Define the terminal event protocol and browser state model

**Files:**
- Create: `apps/web/src/lib/terminal-protocol.ts`
- Modify: `apps/web/src/types.ts`
- Modify: `apps/web/src/test/useTerminalSession.test.tsx`

- [ ] **Step 1: Write the failing frontend protocol tests**

Add tests in `apps/web/src/test/useTerminalSession.test.tsx` for these behaviors:

```ts
it("records streamed terminal output frames", async () => {
  // websocket sends {type:"output", data:"$ git status\r\n"}
  // hook exposes transcript containing the streamed line
});

it("exposes writable terminal state when the websocket is ready", async () => {
  // hook returns sendInput and resize callbacks once terminal is connected
});

it("marks terminal unavailable when a transport close arrives before ready", async () => {
  // websocket close without open -> status unavailable
});
```

- [ ] **Step 2: Run the focused test to verify the protocol is missing**

Run:

```bash
pnpm --dir apps/web test -- --run src/test/useTerminalSession.test.tsx
```

Expected: FAIL because `sendInput`, `resize`, protocol parsing, and terminal event handling do not exist yet.

- [ ] **Step 3: Add the shared browser-side protocol types**

Create `apps/web/src/lib/terminal-protocol.ts`:

```ts
export type TerminalClientMessage =
  | { type: "input"; data: string }
  | { type: "resize"; cols: number; rows: number }
  | { type: "ping" };

export type TerminalServerMessage =
  | { type: "ready"; cols: number; rows: number }
  | { type: "output"; data: string }
  | { type: "status"; phase: "starting" | "running" | "stopped"; detail?: string }
  | { type: "exit"; exitCode: number | null }
  | { type: "error"; message: string };
```

Update `apps/web/src/types.ts` so `TerminalSessionState` includes:

```ts
export type TerminalSessionState = {
  status: "idle" | "connecting" | "ready" | "unavailable" | "error";
  transcript: string[];
  history: CommandHistoryEntry[];
  terminalUrl: string | null;
  error: string | null;
  reconnect: () => void;
  sendInput: (data: string) => void;
  resize: (cols: number, rows: number) => void;
};
```

- [ ] **Step 4: Update the terminal hook tests to target the new interface**

Replace the old placeholder assertions with explicit protocol-driven assertions such as:

```ts
expect(result.current.sendInput).toBeTypeOf("function");
expect(result.current.resize).toBeTypeOf("function");
expect(result.current.status).toBe("ready");
```

- [ ] **Step 5: Commit the protocol contract**

```bash
git add apps/web/src/lib/terminal-protocol.ts apps/web/src/types.ts apps/web/src/test/useTerminalSession.test.tsx
git commit -m "test: define terminal protocol contract"
```

Expected: one commit with only the browser-side terminal protocol contract and failing tests.

## Task 2: Build runner-side PTY session management

**Files:**
- Modify: `services/runner/go.mod`
- Create: `services/runner/internal/engine/terminal_sessions.go`
- Create: `services/runner/internal/engine/terminal_protocol.go`
- Create: `services/runner/internal/test/terminal_sessions_test.go`

- [ ] **Step 1: Write the failing runner terminal session tests**

Create `services/runner/internal/test/terminal_sessions_test.go` covering:

```go
func TestTerminalManagerStartsShellForWorkspace(t *testing.T) {}
func TestTerminalManagerRejectsMissingWorkspace(t *testing.T) {}
func TestTerminalManagerWritesInputToPTY(t *testing.T) {}
func TestTerminalManagerResizesPTY(t *testing.T) {}
func TestTerminalManagerClosesShellOnRelease(t *testing.T) {}
```

- [ ] **Step 2: Run the focused runner test to verify the PTY manager does not exist**

Run:

```bash
go test ./services/runner/internal/test -run 'TestTerminalManager' -v
```

Expected: FAIL because `TerminalManager` and its dependencies are not implemented.

- [ ] **Step 3: Add the PTY dependency and protocol helpers**

Update `services/runner/go.mod` to add:

```txt
require github.com/creack/pty v1.1.24
```

Create `services/runner/internal/engine/terminal_protocol.go`:

```go
package engine

type TerminalClientMessage struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	Cols uint16 `json:"cols,omitempty"`
	Rows uint16 `json:"rows,omitempty"`
}

type TerminalServerMessage struct {
	Type     string `json:"type"`
	Data     string `json:"data,omitempty"`
	Phase    string `json:"phase,omitempty"`
	Detail   string `json:"detail,omitempty"`
	Cols     uint16 `json:"cols,omitempty"`
	Rows     uint16 `json:"rows,omitempty"`
	ExitCode *int   `json:"exitCode,omitempty"`
	Message  string `json:"message,omitempty"`
}
```

- [ ] **Step 4: Implement the terminal session manager**

Create `services/runner/internal/engine/terminal_sessions.go` with these responsibilities:

```go
type TerminalManager struct {
	mu       sync.Mutex
	sessions map[string]*TerminalSession
}

type TerminalSession struct {
	WorkspaceID string
	WorkspacePath string
	Cmd *exec.Cmd
	PTY *os.File
}

func (m *TerminalManager) Acquire(ctx context.Context, workspacePath string, workspaceID string) (*TerminalSession, error)
func (m *TerminalManager) Release(workspaceID string) error
func (s *TerminalSession) WriteInput(data string) error
func (s *TerminalSession) Resize(cols uint16, rows uint16) error
func (s *TerminalSession) ReadLoop(ctx context.Context, onData func([]byte) error) error
```

Implementation requirements:
- On Windows, launch `powershell.exe -NoLogo`.
- Set `cmd.Dir = workspacePath`.
- Reuse one PTY session per workspace ID.
- Return an error for unknown workspace paths.
- Close the PTY and kill the process on release.

- [ ] **Step 5: Re-run the runner terminal tests and make them pass**

Run:

```bash
go test ./services/runner/internal/test -run 'TestTerminalManager' -v
```

Expected: PASS.

- [ ] **Step 6: Commit the runner PTY manager**

```bash
git add services/runner/go.mod services/runner/internal/engine/terminal_protocol.go services/runner/internal/engine/terminal_sessions.go services/runner/internal/test/terminal_sessions_test.go
git commit -m "feat: add runner pty terminal manager"
```

Expected: one commit containing only the PTY session engine and its tests.

## Task 3: Expose a runner WebSocket terminal endpoint

**Files:**
- Modify: `services/runner/internal/http/router.go`
- Create: `services/runner/internal/http/handlers/terminal_ws.go`
- Create: `services/runner/internal/test/terminal_ws_test.go`

- [ ] **Step 1: Write the failing runner WebSocket handler tests**

Create `services/runner/internal/test/terminal_ws_test.go` covering:

```go
func TestTerminalWebSocketRejectsMalformedWorkspaceID(t *testing.T) {}
func TestTerminalWebSocketStreamsOutputFromPTY(t *testing.T) {}
func TestTerminalWebSocketForwardsInputMessagesToPTY(t *testing.T) {}
func TestTerminalWebSocketHandlesResizeMessages(t *testing.T) {}
```

- [ ] **Step 2: Run the handler test to verify the endpoint is missing**

Run:

```bash
go test ./services/runner/internal/test -run 'TestTerminalWebSocket' -v
```

Expected: FAIL because `/internal/workspaces/{workspaceID}/terminal` does not exist.

- [ ] **Step 3: Add the runner terminal WebSocket handler**

Create `services/runner/internal/http/handlers/terminal_ws.go`:

```go
func TerminalWebSocket(workRoot string, manager *engine.TerminalManager) http.HandlerFunc
```

Handler behavior:
- Validate `workspaceID` with the existing allowlist.
- Resolve the workspace path with `resolveWorkspacePath`.
- Upgrade with `coder/websocket`.
- Acquire the PTY session from the manager.
- Emit `{"type":"ready","cols":120,"rows":30}` after connect.
- Start one goroutine reading PTY bytes and writing `{"type":"output","data":"..."}` frames.
- Read client messages:
  - `input` -> `session.WriteInput`
  - `resize` -> `session.Resize`
  - `ping` -> ignore
- On disconnect, release only the browser socket, not the PTY process unless the session itself is being torn down.

- [ ] **Step 4: Wire the new terminal route**

Update `services/runner/internal/http/router.go`:

```go
r.Get("/internal/workspaces/{workspaceID}/terminal", handlers.TerminalWebSocket(workRoot, terminalManager))
```

Initialize one shared `terminalManager` in the router constructor or a sibling setup function.

- [ ] **Step 5: Re-run the runner WebSocket tests and make them pass**

Run:

```bash
go test ./services/runner/internal/test -run 'TestTerminalWebSocket' -v
```

Expected: PASS.

- [ ] **Step 6: Commit the runner WebSocket endpoint**

```bash
git add services/runner/internal/http/router.go services/runner/internal/http/handlers/terminal_ws.go services/runner/internal/test/terminal_ws_test.go
git commit -m "feat: expose runner terminal websocket"
```

Expected: one commit containing the runner-side terminal WebSocket.

## Task 4: Bridge API terminal WebSockets to the runner

**Files:**
- Modify: `services/api/internal/runner/client.go`
- Modify: `services/api/internal/http/handlers/terminal_ws.go`
- Create: `services/api/internal/test/terminal_ws_test.go`

- [ ] **Step 1: Write the failing API terminal bridge tests**

Create `services/api/internal/test/terminal_ws_test.go` covering:

```go
func TestPracticeTerminalWebSocketRejectsForeignSession(t *testing.T) {}
func TestPracticeTerminalWebSocketBridgesRunnerOutput(t *testing.T) {}
func TestPracticeTerminalWebSocketForwardsBrowserInput(t *testing.T) {}
```

- [ ] **Step 2: Run the focused API terminal test to confirm the bridge is incomplete**

Run:

```bash
go test ./services/api/internal/test -run 'TestPracticeTerminalWebSocket' -v
```

Expected: FAIL because the current handler only echoes frames locally and the runner client has no terminal connector.

- [ ] **Step 3: Extend the runner client with a terminal dialer**

Update `services/api/internal/runner/client.go` to add:

```go
type TerminalConnection interface {
	Read(ctx context.Context) (int, []byte, error)
	Write(ctx context.Context, messageType int, payload []byte) error
	Close(status websocket.StatusCode, reason string) error
}

type Client interface {
	CreateWorkspace(ctx context.Context, template string) (Workspace, error)
	ResetWorkspace(ctx context.Context, workspaceID string) error
	ConnectTerminal(ctx context.Context, workspaceID string) (TerminalConnection, error)
}
```

Add `ConnectTerminal` on `HTTPClient` so it dials:

```txt
ws://runner-base/internal/workspaces/{workspaceID}/terminal
```

using `coder/websocket.Dial`.

- [ ] **Step 4: Replace the API echo handler with a true bridge**

Update `services/api/internal/http/handlers/terminal_ws.go` so it:
- authorizes the current user against `PracticeSessionByID`
- upgrades the browser connection
- dials the runner terminal WebSocket using `session.RunnerRef`
- pumps browser -> runner frames
- pumps runner -> browser frames
- closes both sides on any terminal or context error

Target structure:

```go
func PracticeTerminalWebsocket(practiceService service.PracticeService, runnerClient runner.Client) http.HandlerFunc
```

- [ ] **Step 5: Re-run API terminal tests and make them pass**

Run:

```bash
go test ./services/api/internal/test -run 'TestPracticeTerminalWebSocket' -v
```

Expected: PASS.

- [ ] **Step 6: Commit the API bridge**

```bash
git add services/api/internal/runner/client.go services/api/internal/http/handlers/terminal_ws.go services/api/internal/test/terminal_ws_test.go
git commit -m "feat: bridge api terminal websocket to runner"
```

Expected: one commit containing the API-to-runner terminal bridge.

## Task 5: Upgrade the browser workbench to xterm.js

**Files:**
- Modify: `apps/web/package.json`
- Modify: `apps/web/src/hooks/useTerminalSession.ts`
- Modify: `apps/web/src/components/TerminalPanel.tsx`
- Modify: `apps/web/src/components/Workbench.tsx`
- Modify: `apps/web/src/test/useTerminalSession.test.tsx`
- Modify: `apps/web/src/test/App.test.tsx`

- [ ] **Step 1: Write the failing xterm integration tests**

Add or extend tests in:
- `apps/web/src/test/useTerminalSession.test.tsx`
- `apps/web/src/test/App.test.tsx`

Required assertions:

```ts
it("creates an xterm session and streams output into it", async () => {})
it("sends terminal keystrokes over the websocket", async () => {})
it("shows reconnect only when the live terminal transport is unavailable", async () => {})
it("keeps the terminal mounted while a session is active", async () => {})
```

- [ ] **Step 2: Run the focused web tests and confirm they fail**

Run:

```bash
pnpm --dir apps/web test -- --run src/test/useTerminalSession.test.tsx src/test/App.test.tsx
```

Expected: FAIL because `xterm.js` wiring, keyboard input, and resize support do not exist.

- [ ] **Step 3: Add xterm.js dependencies**

Update `apps/web/package.json` dependencies:

```json
{
  "@xterm/addon-fit": "^0.10.0",
  "@xterm/xterm": "^5.5.0"
}
```

- [ ] **Step 4: Rewrite the terminal hook for full-duplex transport**

Update `apps/web/src/hooks/useTerminalSession.ts` so it:
- parses JSON server frames from `TerminalServerMessage`
- exposes `sendInput(data)` and `resize(cols, rows)`
- stores transcript for history/debug fallback
- moves to `ready` only after a `ready` frame or successful socket open
- moves to `unavailable` when the transport drops

Minimal target surface:

```ts
return {
  status,
  transcript,
  history,
  terminalUrl,
  error,
  reconnect,
  sendInput,
  resize,
};
```

- [ ] **Step 5: Replace the placeholder terminal panel with xterm.js**

Update `apps/web/src/components/TerminalPanel.tsx` to:
- mount an `xterm.js` instance into a `div`
- load `FitAddon`
- on terminal data, call `terminal.sendInput(data)`
- on terminal transcript updates, write only new chunks into `xterm`
- on container resize, call `fitAddon.fit()` then `terminal.resize(cols, rows)`
- keep the existing reconnect affordance outside the xterm viewport

Required component shape:

```tsx
export function TerminalPanel({ preview = false, terminal }: TerminalPanelProps) {
  // preview path keeps the existing static preview
  // live path mounts xterm.js into a ref container
}
```

- [ ] **Step 6: Re-run focused browser tests and make them pass**

Run:

```bash
pnpm --dir apps/web test -- --run src/test/useTerminalSession.test.tsx src/test/App.test.tsx
```

Expected: PASS.

- [ ] **Step 7: Commit the xterm frontend**

```bash
git add apps/web/package.json apps/web/src/hooks/useTerminalSession.ts apps/web/src/components/TerminalPanel.tsx apps/web/src/components/Workbench.tsx apps/web/src/test/useTerminalSession.test.tsx apps/web/src/test/App.test.tsx
git commit -m "feat: add xterm terminal workbench"
```

Expected: one commit containing only the browser-side xterm workbench work.

## Task 6: Preserve command history and session usability around the terminal

**Files:**
- Modify: `apps/web/src/types.ts`
- Modify: `apps/web/src/components/CommandHistoryPanel.tsx`
- Modify: `apps/web/src/App.tsx`
- Modify: `services/api/internal/http/handlers/terminal_ws.go`
- Modify: `services/runner/internal/http/handlers/terminal_ws.go`

- [ ] **Step 1: Write failing tests for command logging and logout/session interaction**

Add tests that cover:

```ts
it("appends entered commands to command history after submit", async () => {})
it("does not destroy the xterm mount during logout or refresh boundaries", async () => {})
```

- [ ] **Step 2: Run the targeted tests to confirm history integration is missing**

Run:

```bash
pnpm --dir apps/web test -- --run src/test/App.test.tsx
```

Expected: FAIL because command metadata is not yet emitted into the browser model.

- [ ] **Step 3: Extend the terminal protocol with command metadata frames**

Update runner/API terminal messages to optionally emit:

```json
{ "type": "status", "phase": "running", "detail": "git status" }
{ "type": "exit", "exitCode": 0 }
```

Use those frames in the browser to append `CommandHistoryEntry` records.

- [ ] **Step 4: Keep the session lifecycle coherent**

Update `apps/web/src/App.tsx` so:
- logout tears down the active terminal cleanly
- auto-created first sessions still land directly in the xterm workbench
- reconnect only targets transport failures, not session reconciliation failures

- [ ] **Step 5: Re-run the targeted web tests and make them pass**

Run:

```bash
pnpm --dir apps/web test -- --run src/test/App.test.tsx
```

Expected: PASS.

- [ ] **Step 6: Commit the session/history integration**

```bash
git add apps/web/src/types.ts apps/web/src/components/CommandHistoryPanel.tsx apps/web/src/App.tsx services/api/internal/http/handlers/terminal_ws.go services/runner/internal/http/handlers/terminal_ws.go
git commit -m "feat: integrate terminal history and session lifecycle"
```

Expected: one commit containing command-history and session-lifecycle integration around the terminal.

## Task 7: Full verification and developer documentation

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Document terminal prerequisites and local dev flow**

Update `README.md` with:
- `xterm.js` browser terminal behavior
- runner PTY dependency note
- Windows shell choice (`powershell.exe`)
- exact startup order:

```bash
npm run db:migrate
npm run runner:dev
npm run api:dev
npm run web:dev
```

- [ ] **Step 2: Run backend verification**

Run:

```bash
go test ./services/runner/...
go test ./services/api/...
```

Expected: PASS for both services.

- [ ] **Step 3: Run frontend verification**

Run:

```bash
pnpm --dir apps/web test
pnpm --dir apps/web build
```

Expected: PASS for both commands.

- [ ] **Step 4: Run a manual smoke test**

Manual path:
1. Visit `http://127.0.0.1:5173`
2. Log in with GitHub
3. Land directly in the workbench
4. Observe `xterm.js` prompt
5. Type `git status`
6. See command output in the terminal
7. Click `Reset` and verify the terminal remains attached
8. Click `Logout` and verify the app returns to the login page

Expected: full interactive shell flow works end-to-end.

- [ ] **Step 5: Commit the finished terminal slice**

```bash
git add README.md
git commit -m "docs: finalize xterm terminal rollout"
```

Expected: one documentation/verification commit after all terminal work is green.

---

## Self-Review

### Spec coverage

- Browser terminal rendering: covered by Task 5.
- Real interactive shell/PTy: covered by Tasks 2 and 3.
- API authorization and relay: covered by Task 4.
- Command history and session lifecycle: covered by Task 6.
- Verification and local operation: covered by Task 7.

### Placeholder scan

- No `TODO`, `TBD`, or “fill this in later” placeholders remain.
- Each task names exact files, commands, and the behavioral target.

### Type consistency

- Browser protocol names are consistent across Tasks 1, 4, 5, and 6.
- `sendInput` and `resize` are defined once in `TerminalSessionState` and used consistently after that.
- Runner terminal route uses `/internal/workspaces/{workspaceID}/terminal` consistently in Tasks 3 and 4.
