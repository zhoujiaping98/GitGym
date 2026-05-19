import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import App from "../App";
import * as api from "../lib/api";
import type { TerminalSessionState } from "../types";

const mockUseCurrentSession = vi.fn();
const mockUseTerminalSession = vi.fn();
const mockCreatePracticeSession = vi.spyOn(api, "createPracticeSession");
const mockResetPracticeSession = vi.spyOn(api, "resetPracticeSession");
const mockLogout = vi.spyOn(api, "logout");
const mockFitAddonFit = vi.fn();
const mockTerminalDispose = vi.fn();
const mockTerminalFocus = vi.fn();
const mockTerminalLoadAddon = vi.fn();
const mockTerminalOpen = vi.fn();
const mockTerminalReset = vi.fn();
const mockTerminalResize = vi.fn();
const mockTerminalWrite = vi.fn();
const mockTerminalOnData = vi.fn();
const mockTerminalOnResize = vi.fn();
const mockTerminalInstances: Array<{ options?: unknown }> = [];
const mockResizeObserverObserve = vi.fn();
const mockResizeObserverDisconnect = vi.fn();

let currentTerminalDataHandler: ((data: string) => void) | null = null;
let currentTerminalResizeHandler:
  | ((payload: { cols: number; rows: number }) => void)
  | null = null;
let currentResizeObserverCallback: (() => void) | null = null;

vi.mock("../hooks/useCurrentSession", () => ({
  useCurrentSession: () => mockUseCurrentSession(),
}));

vi.mock("../hooks/useTerminalSession", () => ({
  useTerminalSession: (session: unknown) => mockUseTerminalSession(session),
}));

vi.mock("@xterm/addon-fit", () => ({
  FitAddon: class MockFitAddon {
    fit = mockFitAddonFit;
  },
}));

vi.mock("@xterm/xterm", () => ({
  Terminal: class MockTerminal {
    cols = 80;
    rows = 24;

    constructor(options?: unknown) {
      mockTerminalInstances.push({ options });
    }

    dispose = mockTerminalDispose;
    focus = mockTerminalFocus;
    loadAddon = mockTerminalLoadAddon;
    open = mockTerminalOpen;
    reset = mockTerminalReset;
    resize = (cols: number, rows: number) => {
      this.cols = cols;
      this.rows = rows;
      mockTerminalResize(cols, rows);
    };
    write = mockTerminalWrite;

    onData = (handler: (data: string) => void) => {
      currentTerminalDataHandler = handler;
      mockTerminalOnData(handler);
      return {
        dispose: vi.fn(),
      };
    };

    onResize = (handler: (payload: { cols: number; rows: number }) => void) => {
      currentTerminalResizeHandler = handler;
      mockTerminalOnResize(handler);
      return {
        dispose: vi.fn(),
      };
    };
  },
}));

const activeSession = {
  id: 42,
  userId: 7,
  scenarioId: 1,
  templateId: 1,
  runnerRef: "runner-42",
  workspacePath: "/tmp/gitgym/session-42",
  status: "active",
  startedAt: "2026-05-16T10:00:00.000Z",
  expiresAt: "2026-05-16T12:00:00.000Z",
  lastActivityAt: "2026-05-16T10:05:00.000Z",
} as const;

const nextSession = {
  id: 43,
  userId: 7,
  scenarioId: 1,
  templateId: 1,
  runnerRef: "runner-43",
  workspacePath: "/tmp/gitgym/session-43",
  status: "active",
  startedAt: "2026-05-16T10:10:00.000Z",
  expiresAt: "2026-05-16T12:10:00.000Z",
  lastActivityAt: "2026-05-16T10:10:00.000Z",
} as const;

const mismatchedSession = {
  id: 99,
  userId: 7,
  scenarioId: 1,
  templateId: 1,
  runnerRef: "runner-99",
  workspacePath: "/tmp/gitgym/session-99",
  status: "active",
  startedAt: "2026-05-16T10:20:00.000Z",
  expiresAt: "2026-05-16T12:20:00.000Z",
  lastActivityAt: "2026-05-16T10:20:00.000Z",
} as const;

function createTerminalState(
  overrides: Partial<TerminalSessionState> = {},
): TerminalSessionState {
  return {
    status: "idle",
    transcript: [],
    history: [],
    terminalUrl: null,
    error: null,
    reconnect: vi.fn(),
    sendInput: vi.fn(),
    resize: vi.fn(),
    ...overrides,
  };
}

function emitTerminalData(data: string) {
  currentTerminalDataHandler?.(data);
}

function emitTerminalResize(cols: number, rows: number) {
  currentTerminalResizeHandler?.({ cols, rows });
}

function triggerTerminalContainerResize() {
  currentResizeObserverCallback?.();
}

beforeEach(() => {
  mockUseCurrentSession.mockReset();
  mockUseTerminalSession.mockReset();
  mockFitAddonFit.mockReset();
  mockTerminalDispose.mockReset();
  mockTerminalFocus.mockReset();
  mockTerminalLoadAddon.mockReset();
  mockTerminalOpen.mockReset();
  mockTerminalReset.mockReset();
  mockTerminalResize.mockReset();
  mockTerminalWrite.mockReset();
  mockTerminalOnData.mockReset();
  mockTerminalOnResize.mockReset();
  mockResizeObserverObserve.mockReset();
  mockResizeObserverDisconnect.mockReset();
  mockTerminalInstances.length = 0;
  currentTerminalDataHandler = null;
  currentTerminalResizeHandler = null;
  currentResizeObserverCallback = null;

  class MockResizeObserver {
    constructor(callback: () => void) {
      currentResizeObserverCallback = callback;
    }

    observe = mockResizeObserverObserve;
    disconnect = mockResizeObserverDisconnect;
  }

  vi.stubGlobal("ResizeObserver", MockResizeObserver);

  mockUseCurrentSession.mockReturnValue({
    status: "ready",
    session: null,
    absenceReason: "unauthenticated",
    error: null,
    refresh: vi.fn().mockResolvedValue(null),
  });

  mockUseTerminalSession.mockReturnValue(createTerminalState());

  mockCreatePracticeSession.mockReset();
  mockCreatePracticeSession.mockResolvedValue(nextSession);
  mockResetPracticeSession.mockReset();
  mockResetPracticeSession.mockResolvedValue(undefined);
  mockLogout.mockReset();
  mockLogout.mockResolvedValue(undefined);
});

describe("App", () => {
  it("renders the GitHub login link", () => {
    render(<App />);

    const loginLink = screen.getByRole("link", {
      name: "Continue with GitHub",
    });

    expect(loginLink).toBeInTheDocument();
    expect(loginLink).toHaveAccessibleName("Continue with GitHub");
    expect(loginLink).toHaveAttribute("href", "/api/v1/auth/github/login");
    expect(
      screen.getByText(/safe trial repository, real git behavior, and a resettable environment/i),
    ).toBeInTheDocument();
    expect(screen.getByText("Template: Standard")).toBeInTheDocument();
    expect(screen.getByText("Signed out")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "New Session" }),
    ).not.toBeInTheDocument();
  });

  it("automatically creates a first session for authenticated users without a current session", async () => {
    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: null,
      absenceReason: "missing",
      error: null,
      refresh: vi.fn().mockResolvedValue(nextSession),
    });

    render(<App />);

    expect(screen.getByText("Signed in")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Preparing your workspace" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Continue with GitHub" }),
    ).not.toBeInTheDocument();

    await waitFor(() => {
      expect(mockCreatePracticeSession).toHaveBeenCalledWith({
        scenarioId: 1,
        templateId: 1,
      });
    });

    await waitFor(() => {
      expect(screen.getByText("Session live")).toBeInTheDocument();
      expect(screen.getByText("runner-43")).toBeInTheDocument();
    });
  });

  it("shows a manual recovery state when automatic first-session creation fails", async () => {
    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: null,
      absenceReason: "missing",
      error: null,
      refresh: vi.fn().mockResolvedValue(null),
    });

    mockCreatePracticeSession.mockRejectedValueOnce(new Error("create failed"));

    render(<App />);

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: "Create your first practice session" }),
      ).toBeInTheDocument();
    });

    expect(screen.getByText("create failed")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "New Session" })).toBeInTheDocument();
  });

  it("renders a loading shell while checking for a current session", () => {
    mockUseCurrentSession.mockReturnValue({
      status: "loading",
      session: null,
      absenceReason: null,
      error: null,
      refresh: vi.fn().mockResolvedValue(null),
    });

    render(<App />);

    expect(
      screen.getByRole("heading", { name: "Checking session" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Restoring your practice workbench.")).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Continue with GitHub" }),
    ).not.toBeInTheDocument();
  });

  it("renders a retryable error shell when current session lookup fails", () => {
    const refresh = vi.fn().mockResolvedValue(null);

    mockUseCurrentSession.mockReturnValue({
      status: "error",
      session: null,
      absenceReason: null,
      error: "api offline",
      refresh,
    });

    render(<App />);

    expect(
      screen.getByRole("heading", { name: "Session unavailable" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("We could not restore your current practice session."),
    ).toBeInTheDocument();
    expect(screen.getByText("api offline")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Try again" })).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Continue with GitHub" }),
    ).not.toBeInTheDocument();
  });

  it("renders the live workbench when there is an active session", async () => {
    const refresh = vi.fn().mockResolvedValue(nextSession);
    const reconnect = vi.fn();

    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: activeSession,
      absenceReason: null,
      error: null,
      refresh,
    });

    mockUseTerminalSession.mockReturnValue(
      createTerminalState({
        status: "ready",
        transcript: [
          "$ git status",
          "On branch main",
          "nothing to commit, working tree clean",
        ],
        history: [
          {
            id: "cmd-1",
            command: "git status",
            executedAt: "2026-05-16T10:04:00.000Z",
            exitCode: 0,
            summary: "working tree clean",
          },
        ],
        terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
        reconnect,
      }),
    );

    render(<App />);

    expect(
      screen.queryByRole("link", { name: "Continue with GitHub" }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Session live")).toBeInTheDocument();
    expect(screen.getByText("Terminal")).toBeInTheDocument();
    expect(screen.getByText("Repository")).toBeInTheDocument();
    expect(screen.getByText("History")).toBeInTheDocument();
    expect(screen.getByText("runner-42")).toBeInTheDocument();
    expect(screen.getByText("git status")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "New Session" }));

    await waitFor(() => {
      expect(mockCreatePracticeSession).toHaveBeenCalledWith({
        scenarioId: 1,
        templateId: 1,
      });
    });
    await waitFor(() => {
      expect(mockUseTerminalSession).toHaveBeenLastCalledWith(nextSession);
    });
    expect(refresh).toHaveBeenCalledTimes(1);
    expect(screen.getByText("runner-43")).toBeInTheDocument();
    expect(screen.queryByText("runner-42")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Reset" }));

    await waitFor(() => {
      expect(mockResetPracticeSession).toHaveBeenCalledWith(43);
      expect(refresh).toHaveBeenCalledTimes(2);
    });
    expect(reconnect).not.toHaveBeenCalled();
  });

  it("shows a logout action for authenticated users and returns to the login screen after logout", async () => {
    let resolveLogout: (() => void) | null = null;
    const refresh = vi
      .fn()
      .mockResolvedValueOnce(activeSession)
      .mockResolvedValueOnce(null);
    mockLogout.mockReturnValue(
      new Promise<void>((resolve) => {
        resolveLogout = resolve;
      }),
    );

    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: activeSession,
      absenceReason: null,
      error: null,
      refresh,
    });

    mockUseTerminalSession.mockReturnValue(
      createTerminalState({
        status: "ready",
        terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      }),
    );

    render(<App />);

    fireEvent.click(screen.getByRole("button", { name: "Logout" }));

    expect(mockLogout).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId("live-terminal")).toBeInTheDocument();
    expect(mockTerminalDispose).not.toHaveBeenCalled();

    resolveLogout?.();

    await waitFor(() => {
      expect(refresh).toHaveBeenCalledTimes(1);
    });

    await waitFor(() => {
      expect(
        screen.getByRole("link", { name: "Continue with GitHub" }),
      ).toBeInTheDocument();
    });
    expect(mockTerminalDispose).toHaveBeenCalledTimes(1);
    expect(screen.getByText("Signed out")).toBeInTheDocument();
    expect(screen.queryByText("runner-42")).not.toBeInTheDocument();
  });

  it("renders a reconnect action for unavailable terminals without resetting the session", () => {
    const reconnect = vi.fn();

    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: activeSession,
      absenceReason: null,
      error: null,
      refresh: vi.fn().mockResolvedValue(activeSession),
    });

    mockUseTerminalSession.mockReturnValue(
      createTerminalState({
        status: "unavailable",
        terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
        error: "Terminal transport is unavailable for this session.",
        reconnect,
      }),
    );

    render(<App />);

    fireEvent.click(screen.getByRole("button", { name: "Reconnect" }));

    expect(reconnect).toHaveBeenCalledTimes(1);
    expect(mockResetPracticeSession).not.toHaveBeenCalled();
  });

  it("does not render the terminal empty-state copy while the live shell is ready", () => {
    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: activeSession,
      absenceReason: null,
      error: null,
      refresh: vi.fn().mockResolvedValue(activeSession),
    });

    mockUseTerminalSession.mockReturnValue(
      createTerminalState({
        status: "ready",
        terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
        transcript: [],
      }),
    );

    render(<App />);

    expect(
      screen.queryByText("Terminal output has not arrived yet."),
    ).not.toBeInTheDocument();
  });

  it("reverts the optimistic new session and shows an error when refresh fails", async () => {
    const refresh = vi.fn().mockRejectedValue(new Error("api offline"));

    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: activeSession,
      absenceReason: null,
      error: null,
      refresh,
    });

    mockUseTerminalSession.mockReturnValue(
      createTerminalState({
        status: "ready",
        terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      }),
    );

    render(<App />);

    fireEvent.click(screen.getByRole("button", { name: "New Session" }));

    await waitFor(() => {
      expect(mockCreatePracticeSession).toHaveBeenCalledTimes(1);
      expect(refresh).toHaveBeenCalledTimes(1);
    });

    await waitFor(() => {
      expect(screen.getByText("runner-42")).toBeInTheDocument();
    });

    expect(screen.queryByText("runner-43")).not.toBeInTheDocument();
    expect(
      screen.getByText("Created a new session, but refreshing it failed: api offline"),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Retry sync" })).toBeInTheDocument();
  });

  it("replaces the optimistic new session when refresh returns a different current session", async () => {
    const refresh = vi.fn().mockResolvedValue(mismatchedSession);

    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: activeSession,
      absenceReason: null,
      error: null,
      refresh,
    });

    mockUseTerminalSession.mockReturnValue(
      createTerminalState({
        status: "ready",
        terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      }),
    );

    render(<App />);

    fireEvent.click(screen.getByRole("button", { name: "New Session" }));

    await waitFor(() => {
      expect(refresh).toHaveBeenCalledTimes(1);
      expect(screen.getByText("runner-99")).toBeInTheDocument();
    });

    expect(screen.queryByText("runner-43")).not.toBeInTheDocument();
    expect(
      screen.getByText("Created session #43, but the server returned session #99."),
    ).toBeInTheDocument();
  });

  it("does not show retry sync when creating a new session fails before reconciliation", async () => {
    mockCreatePracticeSession.mockRejectedValueOnce(new Error("create failed"));

    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: activeSession,
      absenceReason: null,
      error: null,
      refresh: vi.fn().mockResolvedValue(activeSession),
    });

    mockUseTerminalSession.mockReturnValue(
      createTerminalState({
        status: "ready",
        terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      }),
    );

    render(<App />);

    fireEvent.click(screen.getByRole("button", { name: "New Session" }));

    await waitFor(() => {
      expect(screen.getByText("create failed")).toBeInTheDocument();
    });

    expect(screen.queryByRole("button", { name: "Retry sync" })).not.toBeInTheDocument();
  });

  it("surfaces a reset reconciliation error when refresh returns no current session", async () => {
    const refresh = vi.fn().mockResolvedValue(null);

    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: activeSession,
      absenceReason: null,
      error: null,
      refresh,
    });

    mockUseTerminalSession.mockReturnValue(
      createTerminalState({
        status: "ready",
        terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      }),
    );

    render(<App />);

    fireEvent.click(screen.getByRole("button", { name: "Reset" }));

    await waitFor(() => {
      expect(mockResetPracticeSession).toHaveBeenCalledWith(42);
      expect(refresh).toHaveBeenCalledTimes(1);
    });

    expect(
      screen.getByText("Reset completed, but the server did not return a current session."),
    ).toBeInTheDocument();
  });

  it("creates an xterm session and streams output into it", async () => {
    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: activeSession,
      absenceReason: null,
      error: null,
      refresh: vi.fn().mockResolvedValue(activeSession),
    });

    mockUseTerminalSession.mockReturnValue(
      createTerminalState({
        status: "ready",
        transcript: ["$ git status\r\n", "On branch main\r\n"],
        terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      }),
    );

    render(<App />);

    await waitFor(() => {
      expect(mockTerminalInstances).toHaveLength(1);
      expect(mockTerminalOpen).toHaveBeenCalledTimes(1);
    });

    await waitFor(() => {
      expect(mockTerminalWrite).toHaveBeenCalledWith("$ git status\r\n");
      expect(mockTerminalWrite).toHaveBeenCalledWith("On branch main\r\n");
    });
  });

  it("sends terminal keystrokes over the websocket", async () => {
    const sendInput = vi.fn();
    const resize = vi.fn();

    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: activeSession,
      absenceReason: null,
      error: null,
      refresh: vi.fn().mockResolvedValue(activeSession),
    });

    mockUseTerminalSession.mockReturnValue(
      createTerminalState({
        status: "ready",
        terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
        sendInput,
        resize,
      }),
    );

    render(<App />);

    await waitFor(() => {
      expect(mockTerminalOnData).toHaveBeenCalledTimes(1);
      expect(mockTerminalOnResize).toHaveBeenCalledTimes(1);
    });

    emitTerminalData("git status\r");
    emitTerminalResize(120, 40);

    expect(sendInput).toHaveBeenCalledWith("git status\r");
    expect(resize).toHaveBeenCalledWith(120, 40);
  });

  it("fits on container resize without sending duplicate resize frames", async () => {
    const resize = vi.fn();

    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: activeSession,
      absenceReason: null,
      error: null,
      refresh: vi.fn().mockResolvedValue(activeSession),
    });

    mockUseTerminalSession.mockReturnValue(
      createTerminalState({
        status: "ready",
        terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
        resize,
      }),
    );

    render(<App />);

    await waitFor(() => {
      expect(mockResizeObserverObserve).toHaveBeenCalledTimes(1);
    });

    const initialFitCalls = mockFitAddonFit.mock.calls.length;

    triggerTerminalContainerResize();

    expect(mockFitAddonFit).toHaveBeenCalledTimes(initialFitCalls + 1);
    expect(resize).not.toHaveBeenCalled();

    emitTerminalResize(120, 40);

    expect(resize).toHaveBeenCalledTimes(1);
    expect(resize).toHaveBeenCalledWith(120, 40);
  });

  it("shows reconnect only when the live terminal transport is unavailable", () => {
    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: activeSession,
      absenceReason: null,
      error: null,
      refresh: vi.fn().mockResolvedValue(activeSession),
    });

    const { rerender } = render(<App />);

    expect(screen.queryByRole("button", { name: "Reconnect" })).not.toBeInTheDocument();

    mockUseTerminalSession.mockReturnValue(
      createTerminalState({
        status: "ready",
        terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      }),
    );
    rerender(<App />);
    expect(screen.queryByRole("button", { name: "Reconnect" })).not.toBeInTheDocument();

    mockUseTerminalSession.mockReturnValue(
      createTerminalState({
        status: "unavailable",
        terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
        error: "Terminal transport is unavailable for this session.",
      }),
    );
    rerender(<App />);
    expect(screen.getByRole("button", { name: "Reconnect" })).toBeInTheDocument();
  });

  it("keeps the terminal mounted while a session is active", async () => {
    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: activeSession,
      absenceReason: null,
      error: null,
      refresh: vi.fn().mockResolvedValue(activeSession),
    });

    mockUseTerminalSession
      .mockReturnValueOnce(
        createTerminalState({
          status: "connecting",
          terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
        }),
      )
      .mockReturnValueOnce(
        createTerminalState({
          status: "ready",
          transcript: ["$ pwd\r\n"],
          terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
        }),
      );

    const { rerender } = render(<App />);

    const firstMount = await screen.findByTestId("live-terminal");

    rerender(<App />);

    const secondMount = await screen.findByTestId("live-terminal");

    expect(firstMount).toBe(secondMount);
    expect(mockTerminalInstances).toHaveLength(1);
  });

  it("does not destroy the xterm mount during refresh boundaries", async () => {
    const refresh = vi.fn().mockRejectedValue(new Error("api offline"));

    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: activeSession,
      absenceReason: null,
      error: null,
      refresh,
    });

    mockUseTerminalSession.mockImplementation((session: typeof activeSession | null) =>
      createTerminalState({
        status: "ready",
        terminalUrl: session
          ? `ws://localhost:3000/api/v1/practice-sessions/${session.id}/terminal`
          : null,
      }),
    );

    render(<App />);

    const firstMount = await screen.findByTestId("live-terminal");

    fireEvent.click(screen.getByRole("button", { name: "New Session" }));

    await waitFor(() => {
      expect(refresh).toHaveBeenCalledTimes(1);
      expect(
        screen.getByText("Created a new session, but refreshing it failed: api offline"),
      ).toBeInTheDocument();
    });

    const secondMount = await screen.findByTestId("live-terminal");

    expect(firstMount).toBe(secondMount);
    expect(mockTerminalInstances).toHaveLength(1);
  });
});
