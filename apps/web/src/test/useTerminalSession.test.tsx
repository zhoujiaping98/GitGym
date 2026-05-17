import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useTerminalSession } from "../hooks/useTerminalSession";
import type { PracticeSession } from "../types";

const activeSession: PracticeSession = {
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
};

type ListenerMap = Record<string, Array<(event?: MessageEvent) => void>>;

class MockWebSocket {
  static instances: MockWebSocket[] = [];

  url: string;
  listeners: ListenerMap = {};
  sent: string[] = [];

  constructor(url: string) {
    this.url = url;
    MockWebSocket.instances.push(this);
  }

  addEventListener(type: string, listener: (event?: MessageEvent) => void) {
    this.listeners[type] ??= [];
    this.listeners[type].push(listener);
  }

  close() {
    return undefined;
  }

  send(data: string) {
    this.sent.push(data);
  }

  emit(type: string, event?: MessageEvent) {
    for (const listener of this.listeners[type] ?? []) {
      listener(event);
    }
  }
}

afterEach(() => {
  MockWebSocket.instances = [];
  vi.unstubAllGlobals();
});

describe("useTerminalSession", () => {
  it("starts empty while attempting a live terminal connection", async () => {
    vi.stubGlobal("WebSocket", MockWebSocket);

    const { result } = renderHook(() => useTerminalSession(activeSession));

    await waitFor(() => {
      expect(result.current.status).toBe("connecting");
    });

    expect(result.current.transcript).toEqual([]);
    expect(result.current.history).toEqual([]);
    expect(result.current.error).toBeNull();
    expect(result.current.terminalUrl).toContain("/practice-sessions/42/terminal");
  });

  it("records streamed terminal output frames", async () => {
    vi.stubGlobal("WebSocket", MockWebSocket);

    const { result } = renderHook(() => useTerminalSession(activeSession));

    await waitFor(() => {
      expect(MockWebSocket.instances).toHaveLength(1);
    });

    act(() => {
      MockWebSocket.instances[0].emit("open");
      MockWebSocket.instances[0].emit("message", {
        data: JSON.stringify({ type: "output", data: "$ git status\r\n" }),
      } as MessageEvent);
    });

    await waitFor(() => {
      expect(result.current.transcript).toContain("$ git status\r\n");
    });
  });

  it("exposes writable terminal state when the websocket is ready", async () => {
    vi.stubGlobal("WebSocket", MockWebSocket);

    const { result } = renderHook(() => useTerminalSession(activeSession));

    await waitFor(() => {
      expect(MockWebSocket.instances).toHaveLength(1);
    });

    act(() => {
      MockWebSocket.instances[0].emit("open");
      MockWebSocket.instances[0].emit("message", {
        data: JSON.stringify({ type: "ready", cols: 120, rows: 40 }),
      } as MessageEvent);
    });

    await waitFor(() => {
      expect(result.current.status).toBe("ready");
    });

    expect(result.current.sendInput).toBeTypeOf("function");
    expect(result.current.resize).toBeTypeOf("function");
  });

  it("marks terminal unavailable when a transport close arrives before ready", async () => {
    vi.stubGlobal("WebSocket", MockWebSocket);

    const { result } = renderHook(() => useTerminalSession(activeSession));

    await waitFor(() => {
      expect(MockWebSocket.instances).toHaveLength(1);
    });

    act(() => {
      MockWebSocket.instances[0].emit("close");
    });

    await waitFor(() => {
      expect(result.current.status).toBe("unavailable");
    });
  });

  it("marks the terminal unavailable without inventing transcript or history entries", async () => {
    vi.stubGlobal("WebSocket", MockWebSocket);

    const { result } = renderHook(() => useTerminalSession(activeSession));

    await waitFor(() => {
      expect(MockWebSocket.instances).toHaveLength(1);
    });

    act(() => {
      MockWebSocket.instances[0].emit("error");
    });

    await waitFor(() => {
      expect(result.current.status).toBe("unavailable");
    });

    expect(result.current.transcript).toEqual([]);
    expect(result.current.history).toEqual([]);
    expect(result.current.error).toBe("Terminal transport is unavailable for this session.");
  });

  it("preserves terminal state when the session object changes but the id stays the same", async () => {
    vi.stubGlobal("WebSocket", MockWebSocket);

    const { result, rerender } = renderHook(
      ({ session }: { session: PracticeSession | null }) => useTerminalSession(session),
      {
        initialProps: {
          session: activeSession,
        },
      },
    );

    await waitFor(() => {
      expect(MockWebSocket.instances).toHaveLength(1);
    });

    act(() => {
      MockWebSocket.instances[0].emit("open");
      MockWebSocket.instances[0].emit("message", {
        data: "$ git status",
      } as MessageEvent);
    });

    await waitFor(() => {
      expect(result.current.status).toBe("ready");
      expect(result.current.transcript).toEqual(["$ git status"]);
    });

    act(() => {
      rerender({
        session: {
          ...activeSession,
          runnerRef: "runner-42-updated",
        },
      });
    });

    await waitFor(() => {
      expect(result.current.transcript).toEqual(["$ git status"]);
    });

    expect(MockWebSocket.instances).toHaveLength(1);
    expect(result.current.status).toBe("ready");
  });
});
