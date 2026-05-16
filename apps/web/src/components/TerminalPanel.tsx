import type { TerminalSessionState } from "../types";

type TerminalPanelProps = {
  preview?: boolean;
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
      return "transport pending";
    case "error":
      return "error";
    default:
      return "standby";
  }
}

export function TerminalPanel({
  preview = false,
  terminal,
}: TerminalPanelProps) {
  const lines = preview ? previewLines : terminal.transcript;

  return (
    <section className="workbench-main">
      <div className="panel-header">
        <span>Terminal</span>
        <span className="panel-kicker">
          {preview ? "live shell preview" : statusLabel(terminal.status)}
        </span>
      </div>
      <div className="terminal-window" data-terminal-status={terminal.status}>
        {lines.length > 0 ? (
          lines.map((line, index) => (
            <div key={`${index}-${line}`} className="terminal-line">
              {line}
            </div>
          ))
        ) : (
          <p className="terminal-empty">
            Connect to a practice session to activate the terminal workbench.
          </p>
        )}
      </div>
      {!preview && terminal.terminalUrl ? (
        <div className="panel-footnote">
          <span>Transport</span>
          <code>{terminal.terminalUrl}</code>
        </div>
      ) : null}
    </section>
  );
}
