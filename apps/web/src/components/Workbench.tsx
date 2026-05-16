type WorkbenchProps = {
  preview?: boolean;
};

const terminalLines = [
  "$ git switch feat/recovery-drill",
  "Switched to branch 'feat/recovery-drill'",
  "$ git rebase main",
  "CONFLICT (content): Merge conflict in src/session.ts",
  "hint: resolve conflicts, then run git rebase --continue",
];

const commits = ["a91d7f2", "f6d23c9", "2c8e14a"];

export function Workbench({ preview = false }: WorkbenchProps) {
  const shellClassName = preview
    ? "workbench-shell workbench-shell-preview"
    : "workbench-shell";

  return (
    <section className={shellClassName}>
      <div className="workbench-main">
        <div className="panel-header">
          <span>Terminal</span>
          <span className="panel-kicker">live shell preview</span>
        </div>
        <div className="terminal-window">
          {terminalLines.map((line) => (
            <div key={line} className="terminal-line">
              {line}
            </div>
          ))}
        </div>
      </div>
      <aside className="workbench-side">
        <div className="panel-header">
          <span>Repository</span>
          <span className="panel-kicker">state summary</span>
        </div>
        <dl className="repo-summary">
          <div>
            <dt>Branch</dt>
            <dd>feat/recovery-drill</dd>
          </div>
          <div>
            <dt>HEAD</dt>
            <dd>f6d23c9</dd>
          </div>
          <div>
            <dt>Status</dt>
            <dd>1 conflict, rebase in progress</dd>
          </div>
        </dl>
        <div className="commit-rail" aria-label="Recent commits">
          {commits.map((commit, index) => (
            <div key={commit} className="commit-node">
              <span className="commit-dot" />
              <span>{index === 1 ? `${commit} HEAD` : commit}</span>
            </div>
          ))}
        </div>
      </aside>
      <div className="history-strip">
        <span>History</span>
        <span>3 commands captured</span>
      </div>
    </section>
  );
}
