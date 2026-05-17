import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import App from "../App";
import * as api from "../lib/api";

const mockUseCurrentSession = vi.fn();
const mockUseTerminalSession = vi.fn();
const mockCreatePracticeSession = vi.spyOn(api, "createPracticeSession");
const mockResetPracticeSession = vi.spyOn(api, "resetPracticeSession");
const mockLogout = vi.spyOn(api, "logout");

vi.mock("../hooks/useCurrentSession", () => ({
  useCurrentSession: () => mockUseCurrentSession(),
}));

vi.mock("../hooks/useTerminalSession", () => ({
  useTerminalSession: (session: unknown) => mockUseTerminalSession(session),
}));

const activeSession = {
  id: 42,
  userId: 7,
  scenarioId: 9,
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
  scenarioId: 9,
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
  scenarioId: 9,
  templateId: 1,
  runnerRef: "runner-99",
  workspacePath: "/tmp/gitgym/session-99",
  status: "active",
  startedAt: "2026-05-16T10:20:00.000Z",
  expiresAt: "2026-05-16T12:20:00.000Z",
  lastActivityAt: "2026-05-16T10:20:00.000Z",
} as const;

beforeEach(() => {
  mockUseCurrentSession.mockReset();
  mockUseTerminalSession.mockReset();

  mockUseCurrentSession.mockReturnValue({
    status: "ready",
    session: null,
    absenceReason: "unauthenticated",
    error: null,
    refresh: vi.fn().mockResolvedValue(null),
  });

  mockUseTerminalSession.mockReturnValue({
    status: "idle",
    transcript: [],
    history: [],
    terminalUrl: null,
    error: null,
    reconnect: vi.fn(),
  });

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

    mockUseTerminalSession.mockReturnValue({
      status: "idle",
      transcript: [],
      history: [],
      terminalUrl: null,
      error: null,
      reconnect: vi.fn(),
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
        scenarioId: 9,
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
      expect(screen.getByRole("heading", { name: "Create your first practice session" })).toBeInTheDocument();
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
    expect(screen.getByText("We could not restore your current practice session.")).toBeInTheDocument();
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

    mockUseTerminalSession.mockReturnValue({
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
      error: null,
      reconnect,
    });

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
        scenarioId: 9,
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
    const refresh = vi
      .fn()
      .mockResolvedValueOnce(activeSession)
      .mockResolvedValueOnce(null);

    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: activeSession,
      absenceReason: null,
      error: null,
      refresh,
    });

    mockUseTerminalSession.mockReturnValue({
      status: "ready",
      transcript: [],
      history: [],
      terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      error: null,
      reconnect: vi.fn(),
    });

    render(<App />);

    fireEvent.click(screen.getByRole("button", { name: "Logout" }));

    await waitFor(() => {
      expect(mockLogout).toHaveBeenCalledTimes(1);
      expect(refresh).toHaveBeenCalledTimes(1);
    });

    await waitFor(() => {
      expect(
        screen.getByRole("link", { name: "Continue with GitHub" }),
      ).toBeInTheDocument();
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

    mockUseTerminalSession.mockReturnValue({
      status: "unavailable",
      transcript: [],
      history: [],
      terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      error: "Terminal transport is unavailable for this session.",
      reconnect,
    });

    render(<App />);

    fireEvent.click(screen.getByRole("button", { name: "Reconnect" }));

    expect(reconnect).toHaveBeenCalledTimes(1);
    expect(mockResetPracticeSession).not.toHaveBeenCalled();
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

    mockUseTerminalSession.mockReturnValue({
      status: "ready",
      transcript: [],
      history: [],
      terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      error: null,
      reconnect: vi.fn(),
    });

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
    expect(screen.getByText("Created a new session, but refreshing it failed: api offline")).toBeInTheDocument();
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

    mockUseTerminalSession.mockReturnValue({
      status: "ready",
      transcript: [],
      history: [],
      terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      error: null,
      reconnect: vi.fn(),
    });

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

    mockUseTerminalSession.mockReturnValue({
      status: "ready",
      transcript: [],
      history: [],
      terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      error: null,
      reconnect: vi.fn(),
    });

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

    mockUseTerminalSession.mockReturnValue({
      status: "ready",
      transcript: [],
      history: [],
      terminalUrl: "ws://localhost:3000/api/v1/practice-sessions/42/terminal",
      error: null,
      reconnect: vi.fn(),
    });

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
});
