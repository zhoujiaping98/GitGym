import { useEffect, useRef, useState } from "react";
import { buildTerminalWebSocketUrl } from "../lib/ws";
import type {
  CommandHistoryEntry,
  PracticeSession,
  TerminalSessionState,
} from "../types";

const unavailableSummary =
  "Live terminal transport will arrive with Task 10. Session metadata is available now.";

function makeSeedTranscript(session: PracticeSession): string[] {
  return [
    `$ gitgym attach --session ${session.id}`,
    `runner=${session.runnerRef}`,
    `workspace=${session.workspacePath}`,
  ];
}

function makeSeedHistory(session: PracticeSession): CommandHistoryEntry[] {
  return [
    {
      id: `session-${session.id}-attach`,
      command: `gitgym attach --session ${session.id}`,
      executedAt: session.lastActivityAt,
      exitCode: 0,
      summary: "Workbench attached to active practice session",
    },
  ];
}

export function useTerminalSession(
  session: PracticeSession | null,
): TerminalSessionState {
  const socketRef = useRef<WebSocket | null>(null);
  const reconnectTokenRef = useRef(0);
  const [status, setStatus] = useState<TerminalSessionState["status"]>("idle");
  const [transcript, setTranscript] = useState<string[]>([]);
  const [history, setHistory] = useState<CommandHistoryEntry[]>([]);
  const [terminalUrl, setTerminalUrl] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [reconnectCount, setReconnectCount] = useState(0);

  useEffect(() => {
    reconnectTokenRef.current += 1;

    if (socketRef.current) {
      socketRef.current.close();
      socketRef.current = null;
    }

    if (!session) {
      setStatus("idle");
      setTranscript([]);
      setHistory([]);
      setTerminalUrl(null);
      setError(null);
      return;
    }

    const currentReconnectToken = reconnectTokenRef.current;
    const nextTerminalUrl = buildTerminalWebSocketUrl(session.id);

    setStatus("connecting");
    setTerminalUrl(nextTerminalUrl);
    setError(null);
    setTranscript(makeSeedTranscript(session));
    setHistory(makeSeedHistory(session));

    if (typeof WebSocket === "undefined") {
      setStatus("unavailable");
      setError("WebSocket is not available in this browser.");
      return;
    }

    const socket = new WebSocket(nextTerminalUrl);
    socketRef.current = socket;

    socket.addEventListener("open", () => {
      if (reconnectTokenRef.current !== currentReconnectToken) {
        return;
      }

      setStatus("ready");
      setTranscript((lines) => [...lines, "Terminal connection established."]);
    });

    socket.addEventListener("message", (event) => {
      if (reconnectTokenRef.current !== currentReconnectToken) {
        return;
      }

      const nextLine = typeof event.data === "string" ? event.data : "[binary]";
      setTranscript((lines) => [...lines, nextLine]);
    });

    socket.addEventListener("error", () => {
      if (reconnectTokenRef.current !== currentReconnectToken) {
        return;
      }

      setStatus("unavailable");
      setError(unavailableSummary);
    });

    socket.addEventListener("close", () => {
      if (reconnectTokenRef.current !== currentReconnectToken) {
        return;
      }

      setStatus((currentStatus) =>
        currentStatus === "ready" ? "idle" : "unavailable",
      );
      setTranscript((lines) =>
        lines.includes(unavailableSummary) ? lines : [...lines, unavailableSummary],
      );
    });

    return () => {
      if (socketRef.current === socket) {
        socketRef.current = null;
      }
      socket.close();
    };
  }, [session, reconnectCount]);

  return {
    status,
    transcript,
    history,
    terminalUrl,
    error,
    reconnect: () => {
      setReconnectCount((count) => count + 1);
    },
  };
}
