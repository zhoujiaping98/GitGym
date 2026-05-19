import { useEffect, useRef } from "react";
import { FitAddon } from "@xterm/addon-fit";
import { Terminal } from "@xterm/xterm";
import "@xterm/xterm/css/xterm.css";
import type { TerminalSessionState } from "../types";

type TerminalPanelProps = {
  preview?: boolean;
  sessionKey?: number | string | null;
  terminal: TerminalSessionState;
};

const previewLines = [
  "$ git switch feat/recovery-drill",
  "Switched to branch 'feat/recovery-drill'",
  "$ git rebase main",
  "CONFLICT (content): Merge conflict in src/session.ts",
  "hint: resolve conflicts, then run git rebase --continue",
];

function statusLabel(status: TerminalSessionState["status"]) {
  switch (status) {
    case "connecting":
      return "connecting";
    case "ready":
      return "interactive";
    case "unavailable":
      return "unavailable";
    case "error":
      return "error";
    default:
      return "standby";
  }
}

function isTransientFitDimensionsError(error: unknown) {
  return (
    error instanceof TypeError &&
    error.message.includes("undefined") &&
    error.message.includes("dimensions")
  );
}

function writePendingTranscript(
  terminalInstance: Terminal,
  transcript: string[],
  lastWrittenIndexRef: { current: number },
) {
  if (transcript.length < lastWrittenIndexRef.current) {
    (terminalInstance as Terminal & { reset?: () => void }).reset?.();
    lastWrittenIndexRef.current = 0;
  }

  const nextChunks = transcript.slice(lastWrittenIndexRef.current);
  for (const chunk of nextChunks) {
    terminalInstance.write(chunk);
  }
  lastWrittenIndexRef.current = transcript.length;
}

export function TerminalPanel({
  preview = false,
  sessionKey = null,
  terminal,
}: TerminalPanelProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const terminalRef = useRef<Terminal | null>(null);
  const lastWrittenIndexRef = useRef(0);
  const sendInputRef = useRef(terminal.sendInput);
  const resizeHandlerRef = useRef(terminal.resize);

  useEffect(() => {
    sendInputRef.current = terminal.sendInput;
    resizeHandlerRef.current = terminal.resize;
  }, [terminal.resize, terminal.sendInput]);

  useEffect(() => {
    lastWrittenIndexRef.current = 0;
  }, [preview, sessionKey]);

  useEffect(() => {
    if (preview) {
      return;
    }

    let fitFrame: number | null = null;
    let initTimer: ReturnType<typeof setTimeout> | null = null;
    let resizeObserver: ResizeObserver | null = null;
    let terminalInstance: Terminal | null = null;
    let dataSubscription: { dispose: () => void } | null = null;
    let resizeSubscription: { dispose: () => void } | null = null;

    const scheduleFit = (fitAddon: FitAddon) => {
      if (typeof requestAnimationFrame !== "function") {
        fitTerminal(fitAddon);
        return;
      }

      if (fitFrame !== null) {
        return;
      }

      fitFrame = requestAnimationFrame(() => {
        fitFrame = null;
        fitTerminal(fitAddon);
      });
    };

    const fitTerminal = (fitAddon: FitAddon) => {
      try {
        fitAddon.fit();
      } catch (error) {
        if (isTransientFitDimensionsError(error)) {
          scheduleFit(fitAddon);
          return;
        }
        throw error;
      }
    };

    const initializeTerminal = () => {
      initTimer = null;

      const container = containerRef.current;
      if (!container) {
        return;
      }

      terminalInstance = new Terminal({
        convertEol: true,
        cursorBlink: true,
        fontFamily: '"Consolas", "SFMono-Regular", "Liberation Mono", monospace',
        fontSize: 13,
        theme: {
          background: "#050e19",
          foreground: "#bbffd6",
          cursor: "#79ffb1",
        },
      });
      const fitAddon = new FitAddon();

      terminalRef.current = terminalInstance;
      terminalInstance.loadAddon(fitAddon);
      terminalInstance.open(container);
      terminalInstance.focus();

      dataSubscription = terminalInstance.onData((data) => {
        sendInputRef.current(data);
      });
      resizeSubscription = terminalInstance.onResize(({ cols, rows }) => {
        resizeHandlerRef.current(cols, rows);
      });

      fitTerminal(fitAddon);

      resizeObserver =
        typeof ResizeObserver === "undefined"
          ? null
          : new ResizeObserver(() => {
              scheduleFit(fitAddon);
            });

      resizeObserver?.observe(container);
      writePendingTranscript(terminalInstance, terminal.transcript, lastWrittenIndexRef);
    };

    initTimer = setTimeout(initializeTerminal, 0);

    return () => {
      if (initTimer !== null) {
        clearTimeout(initTimer);
      }
      if (fitFrame !== null && typeof cancelAnimationFrame === "function") {
        cancelAnimationFrame(fitFrame);
      }
      resizeObserver?.disconnect();
      dataSubscription?.dispose();
      resizeSubscription?.dispose();
      lastWrittenIndexRef.current = 0;
      if (terminalRef.current === terminalInstance) {
        terminalRef.current = null;
      }
      terminalInstance?.dispose();
    };
  }, [preview, sessionKey]);

  useEffect(() => {
    if (preview || terminal.status !== "ready") {
      return;
    }

    terminalRef.current?.focus();
  }, [preview, terminal.status]);

  useEffect(() => {
    if (preview) {
      return;
    }

    const terminalInstance = terminalRef.current;
    if (!terminalInstance) {
      return;
    }
    writePendingTranscript(terminalInstance, terminal.transcript, lastWrittenIndexRef);
  }, [preview, terminal.transcript]);

  const showReconnect =
    !preview && terminal.terminalUrl && terminal.status === "unavailable";
  const showEmptyState =
    !preview && terminal.transcript.length === 0 && terminal.status !== "ready";
  const emptyMessage = terminal.error
    ? terminal.error
    : terminal.status === "connecting"
      ? "Opening terminal transport..."
      : "Terminal output has not arrived yet.";

  return (
    <section className="workbench-main">
      <div className="panel-header">
        <span>Terminal</span>
        <span className="panel-kicker">
          {preview ? "live shell preview" : statusLabel(terminal.status)}
        </span>
      </div>
      <div className="terminal-window" data-terminal-status={terminal.status}>
        {preview ? (
          previewLines.map((line, index) => (
            <div key={`${index}-${line}`} className="terminal-line">
              {line}
            </div>
          ))
        ) : (
          <div
            data-testid="live-terminal"
            className="terminal-host"
            onMouseDown={() => {
              terminalRef.current?.focus();
            }}
            ref={containerRef}
            style={{ minHeight: "100%", width: "100%" }}
          />
        )}
        {showEmptyState ? (
          <p className="terminal-empty">{emptyMessage}</p>
        ) : null}
      </div>
      {!preview && terminal.terminalUrl ? (
        <div className="panel-footnote">
          <span>Transport</span>
          <code>{terminal.terminalUrl}</code>
          {showReconnect ? (
            <button className="top-bar-button" onClick={terminal.reconnect} type="button">
              Reconnect
            </button>
          ) : null}
        </div>
      ) : null}
    </section>
  );
}
