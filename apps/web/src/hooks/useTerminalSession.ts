import { useCallback, useEffect, useRef, useState } from "react";
import type {
  TerminalClientMessage,
  TerminalServerMessage,
} from "../lib/terminal-protocol";
import { buildTerminalWebSocketUrl } from "../lib/ws";
import type {
  CommandHistoryEntry,
  PracticeSession,
  TerminalSessionState,
} from "../types";

const unavailableSummary = "Terminal transport is unavailable for this session.";

function sendTerminalMessage(
  message: TerminalClientMessage,
  socket: WebSocket | null,
  protocolReady: boolean,
) {
  if (!protocolReady || !socket || socket.readyState !== 1) {
    return;
  }

  socket.send(JSON.stringify(message));
}

function parseTerminalFrame(payload: string): TerminalServerMessage | null {
  try {
    return JSON.parse(payload) as TerminalServerMessage;
  } catch {
    return null;
  }
}

export function useTerminalSession(
  session: PracticeSession | null,
): TerminalSessionState {
  const socketRef = useRef<WebSocket | null>(null);
  const protocolReadyRef = useRef(false);
  const reconnectTokenRef = useRef(0);
  const historyEntryCountRef = useRef(0);
  const lastCommandEntryIdRef = useRef<string | null>(null);
  const sessionId = session?.id ?? null;
  const [status, setStatus] = useState<TerminalSessionState["status"]>("idle");
  const [transcript, setTranscript] = useState<string[]>([]);
  const [history, setHistory] = useState<CommandHistoryEntry[]>([]);
  const [terminalUrl, setTerminalUrl] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [reconnectCount, setReconnectCount] = useState(0);

  useEffect(() => {
    reconnectTokenRef.current += 1;
    protocolReadyRef.current = false;
    historyEntryCountRef.current = 0;
    lastCommandEntryIdRef.current = null;

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

      protocolReadyRef.current = true;
      setStatus("ready");
      setError(null);
    });

    socket.addEventListener("message", (event) => {
      if (reconnectTokenRef.current !== currentReconnectToken) {
        return;
      }

      if (typeof event.data !== "string") {
        setTranscript((lines) => [...lines, "[binary]"]);
        return;
      }

      const frame = parseTerminalFrame(event.data);
      if (!frame) {
        setTranscript((lines) => [...lines, event.data]);
        return;
      }

      switch (frame.type) {
        case "ready":
          protocolReadyRef.current = true;
          setStatus("ready");
          setError(null);
          return;
        case "output":
          setTranscript((lines) => [...lines, frame.data]);
          return;
        case "status": {
          const command = frame.detail?.trim();
          if (frame.phase !== "running" || !command) {
            return;
          }

          const entryId = `${sessionId ?? "terminal"}-${historyEntryCountRef.current}`;
          historyEntryCountRef.current += 1;
          lastCommandEntryIdRef.current = entryId;
          setHistory((entries) => [
            ...entries,
            {
              id: entryId,
              command,
              executedAt: new Date().toISOString(),
              phase: "running",
              summary: "Command running",
            },
          ]);
          return;
        }
        case "exit": {
          const latestEntryId = lastCommandEntryIdRef.current;
          if (!latestEntryId) {
            return;
          }

          setHistory((entries) =>
            entries.map((entry) =>
              entry.id === latestEntryId
                ? {
                    ...entry,
                    exitCode: frame.exitCode,
                    phase: "stopped",
                    summary:
                      frame.exitCode == null
                        ? "Command finished"
                        : frame.exitCode === 0
                          ? "Command finished successfully"
                          : "Command finished with errors",
                  }
                : entry,
            ),
          );
          lastCommandEntryIdRef.current = null;
          return;
        }
        case "error":
          setStatus("error");
          setError(frame.message);
          return;
        default:
          return;
      }
    });

    socket.addEventListener("error", () => {
      if (reconnectTokenRef.current !== currentReconnectToken) {
        return;
      }

      protocolReadyRef.current = false;
      setStatus("unavailable");
      setError(unavailableSummary);
    });

    socket.addEventListener("close", () => {
      if (reconnectTokenRef.current !== currentReconnectToken) {
        return;
      }

      protocolReadyRef.current = false;
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
  }, [reconnectCount, sessionId]);

  const reconnect = useCallback(() => {
    setReconnectCount((count) => count + 1);
  }, []);

  const sendInput = useCallback((data: string) => {
    sendTerminalMessage(
      { type: "input", data },
      socketRef.current,
      protocolReadyRef.current,
    );
  }, []);

  const resize = useCallback((cols: number, rows: number) => {
    sendTerminalMessage(
      { type: "resize", cols, rows },
      socketRef.current,
      protocolReadyRef.current,
    );
  }, []);

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
}
