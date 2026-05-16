import type { TerminalSessionState } from "../types";

type CommandHistoryPanelProps = {
  preview?: boolean;
  terminal: TerminalSessionState;
};

export function CommandHistoryPanel({
  preview = false,
  terminal,
}: CommandHistoryPanelProps) {
  if (preview) {
    return (
      <section className="history-panel history-panel-preview">
        <div className="history-strip">
          <span>History</span>
          <span>3 commands captured</span>
        </div>
      </section>
    );
  }

  return (
    <section className="history-panel">
      <div className="history-strip">
        <span>History</span>
        <span>{terminal.history.length} commands captured</span>
      </div>
      {terminal.history.length > 0 ? (
        <div className="history-list" aria-label="Command history">
          {terminal.history.map((entry) => (
            <article key={entry.id} className="history-entry">
              <div className="history-entry-command">{entry.command}</div>
              <div className="history-entry-meta">
                <span>{entry.summary ?? "Command captured"}</span>
                {entry.exitCode != null ? <span>exit {entry.exitCode}</span> : null}
              </div>
            </article>
          ))}
        </div>
      ) : (
        <p className="history-empty">
          No command history has been received for this session.
        </p>
      )}
    </section>
  );
}
