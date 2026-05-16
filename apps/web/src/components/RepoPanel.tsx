import type { PracticeSession } from "../types";

type RepoPanelProps = {
  preview?: boolean;
  session: PracticeSession | null;
};

const previewCommits = ["a91d7f2", "f6d23c9", "2c8e14a"];

function formatDate(value: string) {
  return new Intl.DateTimeFormat("en", {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  }).format(new Date(value));
}

export function RepoPanel({ preview = false, session }: RepoPanelProps) {
  if (preview || !session) {
    return (
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
          {previewCommits.map((commit, index) => (
            <div key={commit} className="commit-node">
              <span className="commit-dot" />
              <span>{index === 1 ? `${commit} HEAD` : commit}</span>
            </div>
          ))}
        </div>
      </aside>
    );
  }

  return (
    <aside className="workbench-side">
      <div className="panel-header">
        <span>Repository</span>
        <span className="panel-kicker">live session metadata</span>
      </div>
      <dl className="repo-summary">
        <div>
          <dt>Runner</dt>
          <dd>{session.runnerRef}</dd>
        </div>
        <div>
          <dt>Workspace</dt>
          <dd className="repo-summary-break">{session.workspacePath}</dd>
        </div>
        <div>
          <dt>Status</dt>
          <dd>{session.status}</dd>
        </div>
        <div>
          <dt>Started</dt>
          <dd>{formatDate(session.startedAt)}</dd>
        </div>
        <div>
          <dt>Expires</dt>
          <dd>{formatDate(session.expiresAt)}</dd>
        </div>
      </dl>
      <div className="commit-rail" aria-label="Repository session details">
        <div className="commit-node">
          <span className="commit-dot" />
          <span>session #{session.id}</span>
        </div>
        <div className="commit-node">
          <span className="commit-dot commit-dot-muted" />
          <span>scenario #{session.scenarioId}</span>
        </div>
        <div className="commit-node">
          <span className="commit-dot commit-dot-muted" />
          <span>template #{session.templateId}</span>
        </div>
      </div>
    </aside>
  );
}
