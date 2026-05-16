import { useEffect, useRef, useState } from "react";
import { buildTerminalWebSocketUrl } from "../lib/ws";
import type {
  CommandHistoryEntry,
  PracticeSession,
  TerminalSessionState,
} from "../types";

const unavailableSummary = "Terminal transport is unavailable for this session.";

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
    setTranscript([]);
    setHistory([]);

    if (typeof WebSocket === "undefined") {
      setStatus("unavailable");
      setError("WebSocket is not available in this browser.");
      return;
    }

    let socket: WebSocket;
    try {
      socket = new WebSocket(nextTerminalUrl);
    } catch (connectError) {
      setStatus("error");
      setError(
        connectError instanceof Error
          ? connectError.message
          : "Unable to open terminal transport.",
      );
      return;
    }

    socketRef.current = socket;

    socket.addEventListener("open", () => {
      if (reconnectTokenRef.current !== currentReconnectToken) {
        return;
      }

      setStatus("ready");
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
        currentStatus === "connecting" || currentStatus === "ready"
          ? "unavailable"
          : currentStatus,
      );
      setError((currentError) => currentError ?? unavailableSummary);
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
