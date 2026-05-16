import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import App from "../App";

const mockUseCurrentSession = vi.fn();
const mockUseTerminalSession = vi.fn();

vi.mock("../hooks/useCurrentSession", () => ({
  useCurrentSession: () => mockUseCurrentSession(),
}));

vi.mock("../hooks/useTerminalSession", () => ({
  useTerminalSession: () => mockUseTerminalSession(),
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

beforeEach(() => {
  mockUseCurrentSession.mockReset();
  mockUseTerminalSession.mockReset();

  mockUseCurrentSession.mockReturnValue({
    status: "ready",
    session: null,
    error: null,
    refresh: vi.fn(),
  });

  mockUseTerminalSession.mockReturnValue({
    status: "idle",
    transcript: [],
    history: [],
    terminalUrl: null,
    error: null,
    reconnect: vi.fn(),
  });
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
  });

  it("renders the live workbench when there is an active session", () => {
    mockUseCurrentSession.mockReturnValue({
      status: "ready",
      session: activeSession,
      error: null,
      refresh: vi.fn(),
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
      reconnect: vi.fn(),
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
  });
});
