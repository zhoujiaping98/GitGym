import React from "react";
import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import App from "../App";
import * as api from "../lib/api";
import type { PracticeCatalog, TerminalSessionState } from "../types";

const mockUseCurrentSession = vi.fn();
const mockUseTerminalSession = vi.fn();
const mockCreatePracticeSession = vi.spyOn(api, "createPracticeSession");
const mockResetPracticeSession = vi.spyOn(api, "resetPracticeSession");
const mockLogout = vi.spyOn(api, "logout");
const mockFetch = vi.fn();
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
const mockRequestAnimationFrame = vi.fn();
const mockCancelAnimationFrame = vi.fn();

let currentTerminalDataHandler: ((data: string) => void) | null = null;
let currentTerminalResizeHandler:
  | ((payload: { cols: number; rows: number }) => void)
  | null = null;
let currentResizeObserverCallback: (() => void) | null = null;
let scheduledAnimationFrameCallback: FrameRequestCallback | null = null;
let nextAnimationFrameHandle = 1;

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

const defaultCatalog: PracticeCatalog = {
  templates: [{ id: 1, key: "standard", name: "Standard" }],
  scenarios: [
    {
      id: 1,
      key: "sandbox-standard",
      name: "Standard Sandbox",
      templateId: 1,
    },
  ],
};

const defaultRepoStatePayload = {
  data: {
    branch: "main",
    head_commit: "6f9bc9e2f9e3f4f24b88a1d8d76d8ef0f1b1c6a0",
    dirty: false,
    changed_files: [],
    captured_at: "2026-05-23T04:00:00.000Z",
  },
} as const;

const reconciledSession = {
  id: 42,
  userId: 7,
  scenarioId: 1,
  templateId: 1,
  runnerRef: "runner-42",
  workspacePath: "/tmp/gitgym/session-42",
  status: "active",
  startedAt: "2026-05-16T10:15:00.000Z",
  expiresAt: "2026-05-16T12:15:00.000Z",
  lastActivityAt: "2026-05-16T10:15:00.000Z",
} as const;

function createCatalogResponse(
  catalog: {
    templates: Array<{ id: number; key: string; name: string }>;
    scenarios: Array<{
      id: number;
      key: string;
      name: string;
      template_id: number;
    }>;
  } = {
    templates: defaultCatalog.templates,
    scenarios: defaultCatalog.scenarios.map((scenario) => ({
      id: scenario.id,
      key: scenario.key,
      name: scenario.name,
      template_id: scenario.templateId,
    })),
  },
) {
  return Promise.resolve(
    new Response(JSON.stringify(catalog), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

function createJsonResponse(payload: unknown, status = 200) {
  return Promise.resolve(
    new Response(JSON.stringify(payload), {
      status,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

function createErrorResponse(status: number, message: string) {
  return createJsonResponse({ error: message }, status);
}

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

function flushAnimationFrame(timestamp = 16) {
  const callback = scheduledAnimationFrameCallback;
  scheduledAnimationFrameCallback = null;
  callback?.(timestamp);
}

async function waitForNewSessionAction() {
  await waitFor(() => {
    expect(screen.getByRole("button", { name: "New Session" })).toBeEnabled();
  });
}

async function waitForScenarioPicker() {
  await waitFor(() => {
    expect(
      screen.getByRole("dialog", { name: "Choose a practice scenario" }),
    ).toBeInTheDocument();
  });
}

async function confirmScenarioPicker() {
  await waitForScenarioPicker();
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: "Start Session" }));
  });
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
  mockRequestAnimationFrame.mockReset();
  mockCancelAnimationFrame.mockReset();
  mockTerminalInstances.length = 0;
  currentTerminalDataHandler = null;
  currentTerminalResizeHandler = null;
  currentResizeObserverCallback = null;
  scheduledAnimationFrameCallback = null;
  nextAnimationFrameHandle = 1;

  class MockResizeObserver {
    constructor(callback: () => void) {
      currentResizeObserverCallback = callback;
    }

    observe = mockResizeObserverObserve;
    disconnect = mockResizeObserverDisconnect;
  }

  vi.stubGlobal("ResizeObserver", MockResizeObserver);
  vi.stubGlobal(
    "requestAnimationFrame",
    mockRequestAnimationFrame.mockImplementation((callback: FrameRequestCallback) => {
      scheduledAnimationFrameCallback = callback;
      return nextAnimationFrameHandle++;
    }),
  );
  vi.stubGlobal("cancelAnimationFrame", mockCancelAnimationFrame);
  vi.stubGlobal("fetch", mockFetch);
  mockFetch.mockReset();
  mockFetch.mockImplementation((input: RequestInfo | URL) => {
    const url = String(input);

    if (url.endsWith("/api/v1/templates")) {
      return createCatalogResponse();
    }

    if (/\/api\/v1\/practice-sessions\/\d+\/repo-state$/.test(url)) {
      return createJsonResponse(defaultRepoStatePayload);
    }

    throw new Error(`Unexpected fetch request: ${url}`);
  });

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
    expect(screen.getByText("Repository")).toBeInTheDocument();
    expect(screen.getByText("Preview")).toBeInTheDocument();
    expect(screen.getByText("Sandbox status")).toBeInTheDocument();
    expect(
      screen.getByText("Operational details appear after a live session is attached."),
    ).toBeInTheDocument();
    expect(screen.queryByText("runner-42")).not.toBeInTheDocument();
    expect(screen.getByText("Signed out")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "New Session" }),
    ).not.toBeInTheDocument();
  });

  it("opens the shared scenario picker from the authenticated empty state", async () => {
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
      screen.queryByRole("link", { name: "Continue with GitHub" }),
    ).not.toBeInTheDocument();

    await waitForScenarioPicker();
    expect(mockCreatePracticeSession).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Start Session" })).toBeEnabled();
  });

  it("uses the selected scenario when the modal confirms a new session", async () => {
    const refresh = vi.fn().mockResolvedValue({
      ...nextSession,
      scenarioId: 2,
    });

    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: null,
      absenceReason: "missing",
      error: null,
      refresh,
    });

    mockFetch.mockImplementation((input: RequestInfo | URL) => {
      const url = String(input);

      if (url.endsWith("/api/v1/templates")) {
        return createCatalogResponse({
          templates: defaultCatalog.templates,
          scenarios: [
            {
              id: 1,
              key: "sandbox-standard",
              name: "Standard Sandbox",
              template_id: 1,
            },
            {
              id: 2,
              key: "sandbox-advanced",
              name: "Advanced Sandbox",
              template_id: 1,
            },
          ],
        });
      }

      if (url.endsWith("/api/v1/practice-sessions/42/repo-state")) {
        return createJsonResponse(defaultRepoStatePayload);
      }

      throw new Error(`Unexpected fetch request: ${url}`);
    });

    render(<App />);

    await waitForScenarioPicker();
    fireEvent.click(screen.getByRole("option", { name: /Advanced Sandbox/i }));
    fireEvent.click(screen.getByRole("button", { name: "Start Session" }));

    await waitFor(() => {
      expect(mockCreatePracticeSession).toHaveBeenCalledWith({
        scenarioId: 2,
      });
    });

    await waitFor(() => {
      expect(screen.getByText("Session live")).toBeInTheDocument();
      expect(screen.getByText("runner-43")).toBeInTheDocument();
    });

    expect(refresh).toHaveBeenCalledTimes(1);
  });

  it("renders the operational session card for an active session", async () => {
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

    mockFetch.mockImplementationOnce(() => createCatalogResponse());

    render(<App />);

    const sessionCard = screen.getByLabelText("Operational session card");

    expect(await within(sessionCard).findByText("Live")).toBeInTheDocument();
    expect(within(sessionCard).getByText("Standard Sandbox")).toBeInTheDocument();
    expect(within(sessionCard).getByText("Template: Standard")).toBeInTheDocument();
    expect(within(sessionCard).getByText("Runner")).toBeInTheDocument();
    expect(within(sessionCard).getByText("runner-42")).toBeInTheDocument();
    expect(within(sessionCard).getByText("Workspace")).toBeInTheDocument();
    expect(within(sessionCard).getByText("/tmp/gitgym/session-42")).toBeInTheDocument();
    expect(within(sessionCard).getByText("Session ID")).toBeInTheDocument();
    expect(within(sessionCard).getByText("42")).toBeInTheDocument();
  });

  it("renders live repo snapshot facts for the active session", async () => {
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

    mockFetch.mockImplementation((input: RequestInfo | URL) => {
      const url = String(input);

      if (url.endsWith("/api/v1/templates")) {
        return createCatalogResponse();
      }

      if (url.endsWith("/api/v1/practice-sessions/42/repo-state")) {
        return createJsonResponse(defaultRepoStatePayload);
      }

      throw new Error(`Unexpected fetch request: ${url}`);
    });

    render(<App />);

    const sessionCard = await screen.findByLabelText("Operational session card");

    expect(await within(sessionCard).findByText("Branch")).toBeInTheDocument();
    expect(within(sessionCard).getByText("main")).toBeInTheDocument();
    expect(within(sessionCard).getByText("HEAD")).toBeInTheDocument();
    expect(within(sessionCard).getByText("6f9bc9e")).toBeInTheDocument();
    expect(within(sessionCard).getByText("Working tree")).toBeInTheDocument();
    expect(within(sessionCard).getByText("Clean")).toBeInTheDocument();
  });

  it("renders neutral attribution for the initial repo snapshot load", async () => {
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
        history: [],
      }),
    );

    mockFetch.mockImplementation((input: RequestInfo | URL) => {
      const url = String(input);

      if (url.endsWith("/api/v1/templates")) {
        return createCatalogResponse();
      }

      if (url.endsWith("/api/v1/practice-sessions/42/repo-state")) {
        return createJsonResponse(defaultRepoStatePayload);
      }

      throw new Error(`Unexpected fetch request: ${url}`);
    });

    render(<App />);

    const sessionCard = await screen.findByLabelText("Operational session card");
    expect(await within(sessionCard).findByText("Snapshot loaded")).toBeInTheDocument();
    expect(within(sessionCard).getByText("main")).toBeInTheDocument();
  });

  it("refreshes repo snapshot when the same session id is reconciled with a new session object", async () => {
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

    let repoStateRequestCount = 0;
    mockFetch.mockImplementation((input: RequestInfo | URL) => {
      const url = String(input);

      if (url.endsWith("/api/v1/templates")) {
        return createCatalogResponse();
      }

      if (url.endsWith("/api/v1/practice-sessions/42/repo-state")) {
        repoStateRequestCount += 1;
        return createJsonResponse(
          repoStateRequestCount === 1
            ? defaultRepoStatePayload
            : {
                data: {
                  branch: "reset/main",
                  head_commit: "bbbbbbb2f9e3f4f24b88a1d8d76d8ef0f1b1c6a0",
                  dirty: true,
                  changed_files: ["M notes.txt"],
                  captured_at: "2026-05-23T04:02:00.000Z",
                },
              },
        );
      }

      throw new Error(`Unexpected fetch request: ${url}`);
    });

    const { rerender } = render(<App />);

    const sessionCard = await screen.findByLabelText("Operational session card");
    expect(await within(sessionCard).findByText("main")).toBeInTheDocument();
    expect(within(sessionCard).getByText("Clean")).toBeInTheDocument();

    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: reconciledSession,
      absenceReason: null,
      error: null,
      refresh: vi.fn().mockResolvedValue(reconciledSession),
    });

    rerender(<App />);

    expect(await within(sessionCard).findByText("reset/main")).toBeInTheDocument();
    expect(within(sessionCard).getByText("Dirty")).toBeInTheDocument();
    expect(within(sessionCard).getByText("M notes.txt")).toBeInTheDocument();
  });

  it("renders an inline unavailable repo state when the snapshot cannot be loaded", async () => {
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

    mockFetch.mockImplementation((input: RequestInfo | URL) => {
      const url = String(input);

      if (url.endsWith("/api/v1/templates")) {
        return createCatalogResponse();
      }

      if (url.endsWith("/api/v1/practice-sessions/42/repo-state")) {
        return createErrorResponse(502, "Unable to load repository state.");
      }

      throw new Error(`Unexpected fetch request: ${url}`);
    });

    render(<App />);

    const sessionCard = await screen.findByLabelText("Operational session card");

    expect(
      await within(sessionCard).findByText("Repository state unavailable."),
    ).toBeInTheDocument();
    expect(within(sessionCard).queryByText("Branch")).not.toBeInTheDocument();
  });

  it("refreshes repo state after a terminal command finishes", async () => {
    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: activeSession,
      absenceReason: null,
      error: null,
      refresh: vi.fn().mockResolvedValue(activeSession),
    });

    const initialTerminalState = createTerminalState({
      status: "ready",
      terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      history: [],
    });
    const completedCommandTerminalState = createTerminalState({
      status: "ready",
      terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      history: [
        {
          id: "cmd-1",
          command: "git status",
          phase: "stopped",
          summary: "Command finished successfully",
        },
      ],
    });

    mockUseTerminalSession.mockReturnValue(initialTerminalState);

    mockFetch.mockImplementation((input: RequestInfo | URL) => {
      const url = String(input);

      if (url.endsWith("/api/v1/templates")) {
        return createCatalogResponse();
      }

      if (url.endsWith("/api/v1/practice-sessions/42/repo-state")) {
        return createJsonResponse(
          mockFetch.mock.calls.filter(
            ([request]) => String(request).endsWith("/api/v1/practice-sessions/42/repo-state"),
          ).length === 1
            ? defaultRepoStatePayload
            : {
                data: {
                  branch: "feature/repo-panel",
                  head_commit: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
                  dirty: true,
                  changed_files: ["M notes.txt"],
                  captured_at: "2026-05-23T04:01:00.000Z",
                },
              },
        );
      }

      throw new Error(`Unexpected fetch request: ${url}`);
    });

    const { rerender } = render(<App />);

    expect(await screen.findByText("Clean")).toBeInTheDocument();

    mockUseTerminalSession.mockReturnValue(completedCommandTerminalState);
    rerender(<App />);

    await waitFor(() => expect(screen.getByText("Dirty")).toBeInTheDocument());
    expect(screen.getByText("M notes.txt")).toBeInTheDocument();
  });

  it("keeps the last snapshot visible and marks it stale when refresh fails", async () => {
    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: activeSession,
      absenceReason: null,
      error: null,
      refresh: vi.fn().mockResolvedValue(activeSession),
    });

    const initialTerminalState = createTerminalState({
      status: "ready",
      terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      history: [],
    });
    const completedCommandTerminalState = createTerminalState({
      status: "ready",
      terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      history: [
        {
          id: "cmd-2",
          command: "git status",
          phase: "stopped",
          summary: "Command finished successfully",
        },
      ],
    });

    mockUseTerminalSession.mockReturnValue(initialTerminalState);

    let repoStateRequestCount = 0;
    mockFetch.mockImplementation((input: RequestInfo | URL) => {
      const url = String(input);

      if (url.endsWith("/api/v1/templates")) {
        return createCatalogResponse();
      }

      if (url.endsWith("/api/v1/practice-sessions/42/repo-state")) {
        repoStateRequestCount += 1;
        return repoStateRequestCount === 1
          ? createJsonResponse(defaultRepoStatePayload)
          : createErrorResponse(502, "Unable to load repository state.");
      }

      throw new Error(`Unexpected fetch request: ${url}`);
    });

    const { rerender } = render(<App />);

    expect(await screen.findByText("main")).toBeInTheDocument();

    mockUseTerminalSession.mockReturnValue(completedCommandTerminalState);
    rerender(<App />);

    await waitFor(() =>
      expect(
        screen.getByText("Repository state may be out of date."),
      ).toBeInTheDocument(),
    );
    expect(screen.getByText("Clean")).toBeInTheDocument();
    expect(screen.getByText("main")).toBeInTheDocument();
  });

  it("refreshes repo state after reconnect when the same command id appears after history reset", async () => {
    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: activeSession,
      absenceReason: null,
      error: null,
      refresh: vi.fn().mockResolvedValue(activeSession),
    });

    const initialTerminalState = createTerminalState({
      status: "ready",
      terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      history: [],
    });
    const firstCompletedCommandState = createTerminalState({
      status: "ready",
      terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      history: [
        {
          id: "42-0",
          command: "git status",
          phase: "stopped",
          summary: "Command finished successfully",
        },
      ],
    });
    const resetHistoryState = createTerminalState({
      status: "ready",
      terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      history: [],
    });
    const repeatedCommandIdState = createTerminalState({
      status: "ready",
      terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      history: [
        {
          id: "42-0",
          command: "git status",
          phase: "stopped",
          summary: "Command finished successfully",
        },
      ],
    });

    let repoStateRequestCount = 0;
    mockFetch.mockImplementation((input: RequestInfo | URL) => {
      const url = String(input);

      if (url.endsWith("/api/v1/templates")) {
        return createCatalogResponse();
      }

      if (url.endsWith("/api/v1/practice-sessions/42/repo-state")) {
        repoStateRequestCount += 1;
        return createJsonResponse({
          data: {
            branch: repoStateRequestCount >= 3 ? "reconnected/main" : "main",
            head_commit:
              repoStateRequestCount >= 3
                ? "cccccccccccccccccccccccccccccccccccccccc"
                : "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
            dirty: repoStateRequestCount >= 3,
            changed_files: repoStateRequestCount >= 3 ? ["M notes.txt"] : [],
            captured_at: `2026-05-23T04:0${repoStateRequestCount}:00.000Z`,
          },
        });
      }

      throw new Error(`Unexpected fetch request: ${url}`);
    });

    mockUseTerminalSession.mockReturnValue(initialTerminalState);

    const { rerender } = render(<App />);

    expect(await screen.findByText("main")).toBeInTheDocument();

    mockUseTerminalSession.mockReturnValue(firstCompletedCommandState);
    rerender(<App />);

    await waitFor(() => expect(repoStateRequestCount).toBe(2));

    mockUseTerminalSession.mockReturnValue(resetHistoryState);
    rerender(<App />);

    mockUseTerminalSession.mockReturnValue(repeatedCommandIdState);
    rerender(<App />);

    await waitFor(() => expect(screen.getByText("reconnected/main")).toBeInTheDocument());
    expect(screen.getByText("Dirty")).toBeInTheDocument();
    expect(repoStateRequestCount).toBe(3);
  });

  it("updates command attribution after history reset when the same command id is reused", async () => {
    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: activeSession,
      absenceReason: null,
      error: null,
      refresh: vi.fn().mockResolvedValue(activeSession),
    });

    const initialTerminalState = createTerminalState({
      status: "ready",
      terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      history: [],
    });
    const firstCompletedCommandState = createTerminalState({
      status: "ready",
      terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      history: [
        {
          id: "42-0",
          command: "git status",
          phase: "stopped",
          summary: "Command finished successfully",
        },
      ],
    });
    const resetHistoryState = createTerminalState({
      status: "ready",
      terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      history: [],
    });
    const repeatedCommandIdState = createTerminalState({
      status: "ready",
      terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      history: [
        {
          id: "42-0",
          command: "git add .",
          phase: "stopped",
          summary: "Command finished successfully",
        },
      ],
    });

    let repoStateRequestCount = 0;
    mockFetch.mockImplementation((input: RequestInfo | URL) => {
      const url = String(input);

      if (url.endsWith("/api/v1/templates")) {
        return createCatalogResponse();
      }

      if (url.endsWith("/api/v1/practice-sessions/42/repo-state")) {
        repoStateRequestCount += 1;
        return createJsonResponse({
          data: {
            branch: repoStateRequestCount >= 3 ? "reconnected/main" : "main",
            head_commit:
              repoStateRequestCount >= 3
                ? "cccccccccccccccccccccccccccccccccccccccc"
                : "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
            dirty: repoStateRequestCount >= 3,
            changed_files: repoStateRequestCount >= 3 ? ["M notes.txt"] : [],
            captured_at: `2026-05-23T04:0${repoStateRequestCount}:00.000Z`,
          },
        });
      }

      throw new Error(`Unexpected fetch request: ${url}`);
    });

    mockUseTerminalSession.mockReturnValue(initialTerminalState);

    const { rerender } = render(<App />);

    expect(await screen.findByText("Snapshot loaded")).toBeInTheDocument();

    mockUseTerminalSession.mockReturnValue(firstCompletedCommandState);
    rerender(<App />);

    await waitFor(() =>
      expect(screen.getByText("Updated after git status")).toBeInTheDocument(),
    );

    mockUseTerminalSession.mockReturnValue(resetHistoryState);
    rerender(<App />);

    mockUseTerminalSession.mockReturnValue(repeatedCommandIdState);
    rerender(<App />);

    await waitFor(() =>
      expect(screen.getByText("Updated after git add .")).toBeInTheDocument(),
    );
    expect(screen.getByText("reconnected/main")).toBeInTheDocument();
    expect(repoStateRequestCount).toBe(3);
  });

  it("does not refresh repo state for a running command entry but refreshes when that command completes", async () => {
    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: activeSession,
      absenceReason: null,
      error: null,
      refresh: vi.fn().mockResolvedValue(activeSession),
    });

    const initialTerminalState = createTerminalState({
      status: "ready",
      terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      history: [
        {
          id: "42-0",
          command: "git status",
          phase: "stopped",
          summary: "Command finished successfully",
        },
      ],
    });
    const runningCommandState = createTerminalState({
      status: "ready",
      terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      history: [
        {
          id: "42-0",
          command: "git status",
          phase: "stopped",
          summary: "Command finished successfully",
        },
        {
          id: "42-1",
          command: "touch notes.txt",
          phase: "running",
          summary: "Command running",
        },
      ],
    });
    const completedCommandState = createTerminalState({
      status: "ready",
      terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      history: [
        {
          id: "42-0",
          command: "git status",
          phase: "stopped",
          summary: "Command finished successfully",
        },
        {
          id: "42-1",
          command: "touch notes.txt",
          phase: "stopped",
          summary: "Command finished successfully",
        },
      ],
    });

    let repoStateRequestCount = 0;
    mockFetch.mockImplementation((input: RequestInfo | URL) => {
      const url = String(input);

      if (url.endsWith("/api/v1/templates")) {
        return createCatalogResponse();
      }

      if (url.endsWith("/api/v1/practice-sessions/42/repo-state")) {
        repoStateRequestCount += 1;
        return createJsonResponse({
          data: {
            branch: repoStateRequestCount >= 2 ? "completed/main" : "main",
            head_commit:
              repoStateRequestCount >= 2
                ? "dddddddddddddddddddddddddddddddddddddddd"
                : "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
            dirty: repoStateRequestCount >= 2,
            changed_files: repoStateRequestCount >= 2 ? ["M notes.txt"] : [],
            captured_at: `2026-05-23T04:1${repoStateRequestCount}:00.000Z`,
          },
        });
      }

      throw new Error(`Unexpected fetch request: ${url}`);
    });

    mockUseTerminalSession.mockReturnValue(initialTerminalState);

    const { rerender } = render(<App />);

    expect(await screen.findByText("main")).toBeInTheDocument();
    expect(repoStateRequestCount).toBe(1);

    mockUseTerminalSession.mockReturnValue(runningCommandState);
    rerender(<App />);

    await waitFor(() => expect(screen.getByText("main")).toBeInTheDocument());
    expect(repoStateRequestCount).toBe(1);

    mockUseTerminalSession.mockReturnValue(completedCommandState);
    rerender(<App />);

    await waitFor(() => expect(screen.getByText("completed/main")).toBeInTheDocument());
    expect(repoStateRequestCount).toBe(2);
    expect(screen.getByText("Dirty")).toBeInTheDocument();
  });

  it("renders command attribution after repo state refreshes for a completed command", async () => {
    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: activeSession,
      absenceReason: null,
      error: null,
      refresh: vi.fn().mockResolvedValue(activeSession),
    });

    const initialTerminalState = createTerminalState({
      status: "ready",
      terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      history: [],
    });
    const completedCommandTerminalState = createTerminalState({
      status: "ready",
      terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      history: [
        {
          id: "cmd-1",
          command: "git add .",
          phase: "stopped",
          summary: "Command finished successfully",
        },
      ],
    });

    mockUseTerminalSession.mockReturnValue(initialTerminalState);

    mockFetch.mockImplementation((input: RequestInfo | URL) => {
      const url = String(input);

      if (url.endsWith("/api/v1/templates")) {
        return createCatalogResponse();
      }

      if (url.endsWith("/api/v1/practice-sessions/42/repo-state")) {
        return createJsonResponse(
          mockFetch.mock.calls.filter(
            ([request]) => String(request).endsWith("/api/v1/practice-sessions/42/repo-state"),
          ).length === 1
            ? defaultRepoStatePayload
            : {
                data: {
                  branch: "feature/repo-panel",
                  head_commit: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
                  dirty: true,
                  changed_files: ["M notes.txt"],
                  captured_at: "2026-05-24T04:01:00.000Z",
                },
              },
        );
      }

      throw new Error(`Unexpected fetch request: ${url}`);
    });

    const { rerender } = render(<App />);

    expect(await screen.findByText("Snapshot loaded")).toBeInTheDocument();

    mockUseTerminalSession.mockReturnValue(completedCommandTerminalState);
    rerender(<App />);

    await waitFor(() =>
      expect(screen.getByText("Updated after git add .")).toBeInTheDocument(),
    );
    expect(screen.getByText("M notes.txt")).toBeInTheDocument();
  });

  it("preserves the last successful attribution when a command-triggered refresh fails", async () => {
    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: activeSession,
      absenceReason: null,
      error: null,
      refresh: vi.fn().mockResolvedValue(activeSession),
    });

    const initialTerminalState = createTerminalState({
      status: "ready",
      terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      history: [],
    });
    const failedRefreshState = createTerminalState({
      status: "ready",
      terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      history: [
        {
          id: "cmd-2",
          command: "git status",
          phase: "stopped",
          summary: "Command finished successfully",
        },
      ],
    });

    mockUseTerminalSession.mockReturnValue(initialTerminalState);

    let repoStateRequestCount = 0;
    mockFetch.mockImplementation((input: RequestInfo | URL) => {
      const url = String(input);

      if (url.endsWith("/api/v1/templates")) {
        return createCatalogResponse();
      }

      if (url.endsWith("/api/v1/practice-sessions/42/repo-state")) {
        repoStateRequestCount += 1;
        return repoStateRequestCount === 1
          ? createJsonResponse(defaultRepoStatePayload)
          : createErrorResponse(502, "Unable to load repository state.");
      }

      throw new Error(`Unexpected fetch request: ${url}`);
    });

    const { rerender } = render(<App />);

    expect(await screen.findByText("Snapshot loaded")).toBeInTheDocument();

    mockUseTerminalSession.mockReturnValue(failedRefreshState);
    rerender(<App />);

    await waitFor(() =>
      expect(screen.getByText("Repository state may be out of date.")).toBeInTheDocument(),
    );
    expect(screen.getByText("Snapshot loaded")).toBeInTheDocument();
    expect(screen.queryByText("Updated after git status")).not.toBeInTheDocument();
  });

  it("keeps the workbench visible and marks the card recovering when the terminal degrades", async () => {
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
        error: "Terminal transport is unavailable for this session.",
      }),
    );

    mockFetch.mockImplementationOnce(() => createCatalogResponse());

    render(<App />);

    const sessionCard = screen.getByLabelText("Operational session card");

    expect(await within(sessionCard).findByText("Recovering")).toBeInTheDocument();
    expect(within(sessionCard).getByText("Terminal")).toBeInTheDocument();
    expect(within(sessionCard).getByText("unavailable")).toBeInTheDocument();
    expect(screen.getByText("History")).toBeInTheDocument();
  });

  it("marks an active session as live while terminal attachment is connecting", async () => {
    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: activeSession,
      absenceReason: null,
      error: null,
      refresh: vi.fn().mockResolvedValue(activeSession),
    });

    mockUseTerminalSession.mockReturnValue(
      createTerminalState({
        status: "connecting",
        terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      }),
    );

    mockFetch.mockImplementationOnce(() => createCatalogResponse());

    render(<App />);

    const sessionCard = screen.getByLabelText("Operational session card");

    expect(await within(sessionCard).findByText("Live")).toBeInTheDocument();
    expect(within(sessionCard).queryByText("Connecting")).not.toBeInTheDocument();
    expect(within(sessionCard).queryByText("Standby")).not.toBeInTheDocument();
  });

  it("supports keyboard scenario selection before confirming a new session", async () => {
    const refresh = vi.fn().mockResolvedValue({
      ...nextSession,
      scenarioId: 2,
    });

    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: activeSession,
      absenceReason: null,
      error: null,
      refresh,
    });

    mockFetch.mockImplementation((input: RequestInfo | URL) => {
      const url = String(input);

      if (url.endsWith("/api/v1/templates")) {
        return createCatalogResponse({
          templates: defaultCatalog.templates,
          scenarios: [
            {
              id: 1,
              key: "sandbox-standard",
              name: "Standard Sandbox",
              template_id: 1,
            },
            {
              id: 2,
              key: "sandbox-advanced",
              name: "Advanced Sandbox",
              template_id: 1,
            },
          ],
        });
      }

      if (url.endsWith("/api/v1/practice-sessions/42/repo-state")) {
        return createJsonResponse(defaultRepoStatePayload);
      }

      throw new Error(`Unexpected fetch request: ${url}`);
    });

    render(<App />);

    await waitForNewSessionAction();
    fireEvent.click(screen.getByRole("button", { name: "New Session" }));

    await waitForScenarioPicker();

    const firstOption = screen.getByRole("option", { name: /Standard Sandbox/i });
    const secondOption = screen.getByRole("option", { name: /Advanced Sandbox/i });

    firstOption.focus();
    expect(firstOption).toHaveFocus();

    fireEvent.keyDown(firstOption, { key: "ArrowDown" });

    await waitFor(() => {
      expect(secondOption).toHaveFocus();
      expect(secondOption).toHaveAttribute("aria-selected", "true");
    });
    expect(firstOption).toHaveAttribute("aria-selected", "false");

    fireEvent.click(screen.getByRole("button", { name: "Start Session" }));

    await waitFor(() => {
      expect(mockCreatePracticeSession).toHaveBeenCalledWith({
        scenarioId: 2,
      });
    });
  });

  it("traps focus inside the scenario picker while it is open", async () => {
    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: activeSession,
      absenceReason: null,
      error: null,
      refresh: vi.fn().mockResolvedValue(activeSession),
    });

    mockFetch.mockImplementationOnce(() =>
      createCatalogResponse({
        templates: defaultCatalog.templates,
        scenarios: [
          {
            id: 1,
            key: "sandbox-standard",
            name: "Standard Sandbox",
            template_id: 1,
          },
          {
            id: 2,
            key: "sandbox-advanced",
            name: "Advanced Sandbox",
            template_id: 1,
          },
        ],
      }),
    );

    render(<App />);

    await waitForNewSessionAction();

    const backgroundNewSessionButton = screen.getByRole("button", { name: "New Session" });
    fireEvent.click(backgroundNewSessionButton);

    await waitForScenarioPicker();

    const firstOption = screen.getByRole("option", { name: /Standard Sandbox/i });
    const startSessionButton = screen.getByRole("button", { name: "Start Session" });

    firstOption.focus();
    expect(firstOption).toHaveFocus();

    fireEvent.keyDown(firstOption, { key: "Tab", shiftKey: true });

    await waitFor(() => {
      expect(startSessionButton).toHaveFocus();
    });
    expect(backgroundNewSessionButton).not.toHaveFocus();

    fireEvent.keyDown(startSessionButton, { key: "Tab" });

    await waitFor(() => {
      expect(firstOption).toHaveFocus();
    });
    expect(backgroundNewSessionButton).not.toHaveFocus();
  });

  it("keeps create-session failures inside the scenario picker", async () => {
    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: null,
      absenceReason: "missing",
      error: null,
      refresh: vi.fn().mockResolvedValue(null),
    });

    mockCreatePracticeSession.mockRejectedValueOnce(new Error("create failed"));

    render(<App />);

    await confirmScenarioPicker();

    await waitFor(() => {
      expect(screen.getByText("create failed")).toBeInTheDocument();
    });
    expect(
      screen.getByRole("dialog", { name: "Choose a practice scenario" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Retry sync" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Session unavailable" }),
    ).not.toBeInTheDocument();
  });

  it("shows a recoverable manual create state after dismissing the auto-opened picker", async () => {
    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: null,
      absenceReason: "missing",
      error: null,
      refresh: vi.fn().mockResolvedValue(null),
    });

    render(<App />);

    await waitFor(() => {
      expect(
        screen.getByRole("dialog", { name: "Choose a practice scenario" }),
      ).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "Cancel" }));

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: "Create your first practice session" }),
      ).toBeInTheDocument();
    });

    expect(screen.getByRole("button", { name: "New Session" })).toBeVisible();
    expect(
      screen.queryByRole("heading", { name: "Preparing your workspace" }),
    ).not.toBeInTheDocument();
  });

  it("waits for catalog before auto-opening the scenario picker", async () => {
    let resolveCatalog: ((value: Response) => void) | null = null;

    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: null,
      absenceReason: "missing",
      error: null,
      refresh: vi.fn().mockResolvedValue(nextSession),
    });

    mockFetch.mockImplementationOnce(
      () =>
        new Promise<Response>((resolve) => {
          resolveCatalog = resolve;
        }),
    );

    render(<App />);

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledWith(
        "/api/v1/templates",
        expect.objectContaining({
          credentials: "include",
          headers: { Accept: "application/json" },
        }),
      );
    });
    expect(mockCreatePracticeSession).not.toHaveBeenCalled();
    expect(
      screen.queryByRole("dialog", { name: "Choose a practice scenario" }),
    ).not.toBeInTheDocument();

    resolveCatalog?.(
      new Response(
        JSON.stringify({
          templates: [{ id: 1, key: "standard", name: "Standard" }],
          scenarios: [
            {
              id: 1,
              key: "sandbox-standard",
              name: "Standard Sandbox",
              template_id: 1,
            },
          ],
        }),
        {
          status: 200,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );

    await waitFor(() => {
      expect(
        screen.getByRole("dialog", { name: "Choose a practice scenario" }),
      ).toBeInTheDocument();
    });
  });

  it("retries only the catalog request from the catalog unavailable shell", async () => {
    const refresh = vi.fn().mockResolvedValue(null);

    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: null,
      absenceReason: "missing",
      error: null,
      refresh,
    });

    mockFetch
      .mockImplementationOnce(() =>
        Promise.resolve(
          new Response(JSON.stringify({ error: "api offline" }), {
            status: 503,
            headers: { "Content-Type": "application/json" },
          }),
        ),
      )
      .mockResolvedValueOnce(createCatalogResponse());

    render(<App />);

    expect(
      await screen.findByRole("heading", { name: "Practice catalog unavailable" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("We couldn’t load the available practice scenarios for this environment."),
    ).toBeInTheDocument();
    expect(screen.getByText("api offline")).toBeInTheDocument();
    expect(mockCreatePracticeSession).not.toHaveBeenCalled();
    expect(
      screen.queryByRole("heading", { name: "Session unavailable" }),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Try again" }));

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledTimes(2);
    });
    expect(refresh).not.toHaveBeenCalled();
  });

  it("renders an administrative empty state when the catalog has no scenarios", async () => {
    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: null,
      absenceReason: "missing",
      error: null,
      refresh: vi.fn().mockResolvedValue(null),
    });

    mockFetch.mockImplementationOnce(() =>
      createCatalogResponse({
        templates: [{ id: 1, key: "standard", name: "Standard" }],
        scenarios: [],
      }),
    );

    render(<App />);

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: "Practice catalog empty" }),
      ).toBeInTheDocument();
    });

    expect(
      screen.getByText("This environment doesn’t have any published practice scenarios yet."),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Ask an administrator to publish at least one scenario before creating a session.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Try again" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "New Session" }),
    ).not.toBeInTheDocument();
    expect(mockCreatePracticeSession).not.toHaveBeenCalled();
  });

  it("renders a recovery-first workspace unavailable shell for orphaned sessions", async () => {
    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: null,
      absenceReason: "orphaned",
      error: "workspace path is no longer available",
      refresh: vi.fn().mockResolvedValue(null),
    });

    render(<App />);

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: "Workspace unavailable" }),
      ).toBeInTheDocument();
    });
    expect(
      screen.getByText(
        "Your previous sandbox can no longer be reopened. Start a fresh session to keep practicing.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("workspace path is no longer available")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "New Session" })).toBeInTheDocument();
  });

  it("opens the existing scenario picker from the workspace unavailable shell", async () => {
    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: null,
      absenceReason: "orphaned",
      error: "workspace path is no longer available",
      refresh: vi.fn().mockResolvedValue(null),
    });

    render(<App />);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "New Session" })).toBeInTheDocument();
    });
    fireEvent.click(screen.getByRole("button", { name: "New Session" }));

    expect(
      await screen.findByRole("dialog", { name: "Choose a practice scenario" }),
    ).toBeInTheDocument();
    expect(mockCreatePracticeSession).not.toHaveBeenCalled();
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

  it("renders a recovery-first session unavailable shell when no current session can be restored", async () => {
    const currentSessionState = {
      status: "ready",
      session: activeSession,
      absenceReason: null as "missing" | null,
      error: null,
      refresh: vi.fn(async () => {
        if (currentSessionState.session?.id === activeSession.id) {
          currentSessionState.session = mismatchedSession;
          return mismatchedSession;
        }

        currentSessionState.session = null;
        currentSessionState.absenceReason = "missing";
        return null;
      }),
    };

    mockUseCurrentSession.mockImplementation(() => currentSessionState);

    mockUseTerminalSession.mockReturnValue(
      createTerminalState({
        status: "ready",
        terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      }),
    );

    render(<App />);

    await waitForNewSessionAction();
    fireEvent.click(screen.getByRole("button", { name: "New Session" }));
    await confirmScenarioPicker();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Retry sync" })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "Retry sync" }));

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Session unavailable" })).toBeInTheDocument();
    });

    expect(screen.getByText("Session recovery")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Your previous practice session is no longer available. Start a fresh session to keep practicing.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Signed out")).not.toBeInTheDocument();
    expect(
      screen.getByText("Session unavailable", { selector: ".session-status-badge" }),
    ).toBeInTheDocument();
    expect(screen.getByText("The server did not return a current session.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "New Session" })).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Create your first practice session" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Try again" })).not.toBeInTheDocument();
  });

  it("opens the existing scenario picker after clicking new session during recovery while the catalog is still loading", async () => {
    let resolveCatalogRequest: ((response: Response) => void) | null = null;
    const currentSessionState = {
      status: "ready",
      session: activeSession,
      absenceReason: null as "missing" | null,
      error: null,
      refresh: vi.fn(async () => {
        if (currentSessionState.session?.id === activeSession.id) {
          currentSessionState.session = mismatchedSession;
          return mismatchedSession;
        }

        currentSessionState.session = null;
        currentSessionState.absenceReason = "missing";
        return null;
      }),
    };

    mockFetch.mockImplementation((input: RequestInfo | URL) => {
      if (String(input).endsWith("/api/v1/templates")) {
        return new Promise<Response>((resolve) => {
          resolveCatalogRequest = resolve;
        });
      }

      throw new Error(`Unexpected fetch request: ${String(input)}`);
    });

    mockUseCurrentSession.mockImplementation(() => currentSessionState);
    mockUseTerminalSession.mockReturnValue(
      createTerminalState({
        status: "ready",
        terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      }),
    );

    render(<App />);

    fireEvent.click(screen.getByRole("button", { name: "Reset" }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Retry sync" })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "Retry sync" }));

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Session unavailable" })).toBeInTheDocument();
    });

    expect(screen.getByText("Session recovery")).toBeInTheDocument();
    expect(screen.queryByText("Loading practice catalog")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "New Session" }));
    expect(
      screen.queryByRole("dialog", { name: "Choose a practice scenario" }),
    ).not.toBeInTheDocument();

    resolveCatalogRequest?.(
      new Response(JSON.stringify({
        templates: defaultCatalog.templates,
        scenarios: defaultCatalog.scenarios.map((scenario) => ({
          id: scenario.id,
          key: scenario.key,
          name: scenario.name,
          template_id: scenario.templateId,
        })),
      }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    await waitFor(() => {
      expect(
        screen.getByRole("dialog", { name: "Choose a practice scenario" }),
      ).toBeInTheDocument();
    });
  });

  it("shows the retryable catalog-unavailable shell when catalog loading fails during recovery", async () => {
    let rejectCatalogRequest: ((error: Error) => void) | null = null;
    const currentSessionState = {
      status: "ready",
      session: activeSession,
      absenceReason: null as "missing" | null,
      error: null,
      refresh: vi.fn(async () => {
        if (currentSessionState.session?.id === activeSession.id) {
          currentSessionState.session = mismatchedSession;
          return mismatchedSession;
        }

        currentSessionState.session = null;
        currentSessionState.absenceReason = "missing";
        return null;
      }),
    };

    mockFetch.mockImplementation((input: RequestInfo | URL) => {
      if (String(input).endsWith("/api/v1/templates")) {
        return new Promise<Response>((_resolve, reject) => {
          rejectCatalogRequest = reject;
        });
      }

      throw new Error(`Unexpected fetch request: ${String(input)}`);
    });

    mockUseCurrentSession.mockImplementation(() => currentSessionState);
    mockUseTerminalSession.mockReturnValue(
      createTerminalState({
        status: "ready",
        terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      }),
    );

    render(<App />);

    fireEvent.click(screen.getByRole("button", { name: "Reset" }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Retry sync" })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "Retry sync" }));

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Session unavailable" })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "New Session" }));

    await act(async () => {
      rejectCatalogRequest?.(new Error("catalog offline"));
    });

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: "Practice catalog unavailable" }),
      ).toBeInTheDocument();
    });

    expect(screen.getByText("catalog offline")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Try again" })).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Session unavailable" }),
    ).not.toBeInTheDocument();
  });

  it("returns to the signed-out login experience when retry sync ends unauthenticated", async () => {
    const currentSessionState = {
      status: "ready",
      session: activeSession,
      absenceReason: null as "unauthenticated" | null,
      error: null,
      refresh: vi.fn(async () => {
        if (currentSessionState.session?.id === activeSession.id) {
          currentSessionState.session = mismatchedSession;
          return mismatchedSession;
        }

        currentSessionState.session = null;
        currentSessionState.absenceReason = "unauthenticated";
        return null;
      }),
    };

    mockUseCurrentSession.mockImplementation(() => currentSessionState);
    mockUseTerminalSession.mockReturnValue(
      createTerminalState({
        status: "ready",
        terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      }),
    );

    render(<App />);

    await waitForNewSessionAction();
    fireEvent.click(screen.getByRole("button", { name: "New Session" }));
    await confirmScenarioPicker();

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Retry sync" })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "Retry sync" }));

    await waitFor(() => {
      expect(screen.getByRole("link", { name: "Continue with GitHub" })).toBeInTheDocument();
    });

    expect(screen.getByText("Signed out")).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Session unavailable" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "New Session" })).not.toBeInTheDocument();
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
    expect(
      screen.getByText("Terminal", { selector: ".workbench-main .panel-header span" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Repository")).toBeInTheDocument();
    expect(screen.getByText("History")).toBeInTheDocument();
    expect(screen.getByText("runner-42")).toBeInTheDocument();
    expect(screen.getByText("git status")).toBeInTheDocument();

    await waitForNewSessionAction();
    fireEvent.click(screen.getByRole("button", { name: "New Session" }));
    await confirmScenarioPicker();

    await waitFor(() => {
      expect(mockCreatePracticeSession).toHaveBeenCalledWith({
        scenarioId: 1,
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

  it("opens the scenario picker before creating a new session from the top bar", async () => {
    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: activeSession,
      absenceReason: null,
      error: null,
      refresh: vi.fn().mockResolvedValue(activeSession),
    });

    render(<App />);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "New Session" })).toBeEnabled();
    });

    fireEvent.click(screen.getByRole("button", { name: "New Session" }));

    expect(
      screen.getByRole("dialog", { name: "Choose a practice scenario" }),
    ).toBeInTheDocument();
    expect(mockCreatePracticeSession).not.toHaveBeenCalled();
  });

  it("closes the scenario picker when the app becomes unauthenticated", async () => {
    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: activeSession,
      absenceReason: null,
      error: null,
      refresh: vi.fn().mockResolvedValue(activeSession),
    });

    const { rerender } = render(<App />);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "New Session" })).toBeEnabled();
    });

    fireEvent.click(screen.getByRole("button", { name: "New Session" }));

    expect(
      screen.getByRole("dialog", { name: "Choose a practice scenario" }),
    ).toBeInTheDocument();

    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: null,
      absenceReason: "unauthenticated",
      error: null,
      refresh: vi.fn().mockResolvedValue(null),
    });

    rerender(<App />);

    await waitFor(() => {
      expect(
        screen.queryByRole("dialog", { name: "Choose a practice scenario" }),
      ).not.toBeInTheDocument();
    });

    expect(mockCreatePracticeSession).not.toHaveBeenCalled();
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

    await waitFor(() => {
      expect(mockTerminalOpen).toHaveBeenCalledTimes(1);
    });

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
    await waitFor(() => {
      expect(mockTerminalDispose).toHaveBeenCalledTimes(1);
    });
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

  it("refreshes the current session once when the live terminal becomes unavailable", async () => {
    const refresh = vi.fn().mockResolvedValue(activeSession);

    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: activeSession,
      absenceReason: null,
      error: null,
      refresh,
    });

    mockUseTerminalSession.mockReturnValue(
      createTerminalState({
        status: "unavailable",
        terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
        error: "Terminal transport is unavailable for this session.",
      }),
    );

    const { rerender } = render(<App />);

    await waitFor(() => {
      expect(refresh).toHaveBeenCalledTimes(1);
    });

    rerender(<App />);

    expect(refresh).toHaveBeenCalledTimes(1);

    mockUseTerminalSession.mockReturnValue(
      createTerminalState({
        status: "ready",
        terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      }),
    );
    rerender(<App />);

    mockUseTerminalSession.mockReturnValue(
      createTerminalState({
        status: "unavailable",
        terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
        error: "Terminal transport is unavailable for this session.",
      }),
    );
    rerender(<App />);

    await waitFor(() => {
      expect(refresh).toHaveBeenCalledTimes(2);
    });
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

    await waitForNewSessionAction();
    fireEvent.click(screen.getByRole("button", { name: "New Session" }));
    await confirmScenarioPicker();

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

    await waitForNewSessionAction();
    fireEvent.click(screen.getByRole("button", { name: "New Session" }));
    await confirmScenarioPicker();

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

    await waitForNewSessionAction();
    fireEvent.click(screen.getByRole("button", { name: "New Session" }));
    await confirmScenarioPicker();

    await waitFor(() => {
      expect(screen.getByText("create failed")).toBeInTheDocument();
    });

    expect(screen.queryByRole("button", { name: "Retry sync" })).not.toBeInTheDocument();
    expect(
      screen.getByRole("dialog", { name: "Choose a practice scenario" }),
    ).toBeInTheDocument();
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

  it("does not show retry sync for informational reset reconciliation results", async () => {
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
    expect(screen.queryByRole("button", { name: "Retry sync" })).not.toBeInTheDocument();
  });

  it("clears the inline reconciliation message when retry sync succeeds", async () => {
    const refresh = vi
      .fn()
      .mockResolvedValueOnce(mismatchedSession)
      .mockResolvedValueOnce(nextSession);

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

    await waitForNewSessionAction();
    fireEvent.click(screen.getByRole("button", { name: "New Session" }));
    await confirmScenarioPicker();

    await waitFor(() => {
      expect(
        screen.getByText("Created session #43, but the server returned session #99."),
      ).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Retry sync" })).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "Retry sync" }));

    await waitFor(() => {
      expect(screen.getByText("runner-43")).toBeInTheDocument();
    });

    expect(
      screen.queryByText("Created session #43, but the server returned session #99."),
    ).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Retry sync" })).not.toBeInTheDocument();
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

    expect(mockFitAddonFit).toHaveBeenCalledTimes(initialFitCalls);
    expect(mockRequestAnimationFrame).toHaveBeenCalledTimes(1);

    flushAnimationFrame();

    expect(mockFitAddonFit).toHaveBeenCalledTimes(initialFitCalls + 1);
    expect(resize).not.toHaveBeenCalled();

    emitTerminalResize(120, 40);

    expect(resize).toHaveBeenCalledTimes(1);
    expect(resize).toHaveBeenCalledWith(120, 40);
  });

  it("retries terminal fitting on the next animation frame when xterm dimensions are not ready", async () => {
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

    mockFitAddonFit
      .mockImplementationOnce(() => {
        throw new TypeError("Cannot read properties of undefined (reading 'dimensions')");
      })
      .mockImplementation(() => undefined);

    render(<App />);

    await waitFor(() => {
      expect(mockRequestAnimationFrame).toHaveBeenCalledTimes(1);
    });

    flushAnimationFrame();

    expect(mockFitAddonFit).toHaveBeenCalledTimes(2);
    expect(mockTerminalOpen).toHaveBeenCalledTimes(1);
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

    await waitForNewSessionAction();
    fireEvent.click(screen.getByRole("button", { name: "New Session" }));
    await confirmScenarioPicker();

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

  it("defers xterm initialization until the strict-mode effect replay settles", async () => {
    vi.useFakeTimers();

    try {
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

      render(
        <React.StrictMode>
          <App />
        </React.StrictMode>,
      );

      expect(mockTerminalInstances).toHaveLength(0);

      await act(async () => {
        vi.runOnlyPendingTimers();
        await Promise.resolve();
      });

      expect(mockTerminalInstances).toHaveLength(1);
    } finally {
      vi.useRealTimers();
    }
  });
});
