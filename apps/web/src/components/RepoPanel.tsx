import type { PracticeSession, RepoStateView, TerminalSessionState } from "../types";

type RepoPanelProps = {
  preview?: boolean;
  session: PracticeSession | null;
  scenarioName?: string | null;
  templateName?: string | null;
  terminalStatus?: TerminalSessionState["status"];
  repoState?: RepoStateView;
};

function formatDate(value: string) {
  return new Intl.DateTimeFormat("en", {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  }).format(new Date(value));
}

function getHealthLabel(status: TerminalSessionState["status"], sessionStatus: string) {
  if (status === "unavailable" || status === "error") {
    return "Recovering";
  }

  if ((status === "ready" || status === "connecting") && sessionStatus === "active") {
    return "Live";
  }

  return "Recovering";
}

function getHealthTone(status: TerminalSessionState["status"], sessionStatus: string) {
  if (status === "unavailable" || status === "error") {
    return "degraded";
  }

  if ((status === "ready" || status === "connecting") && sessionStatus === "active") {
    return "live";
  }

  return "degraded";
}

function formatScenarioName(scenarioName: string | null, scenarioId?: number) {
  if (scenarioName) {
    return scenarioName;
  }

  return scenarioId ? `Scenario #${scenarioId}` : "Scenario unavailable";
}

function formatTemplateName(templateName: string | null, templateId?: number) {
  if (templateName) {
    return `Template: ${templateName}`;
  }

  return templateId ? `Template #${templateId}` : "Template unavailable";
}

function shortHead(headCommit: string) {
  return headCommit.slice(0, 7);
}

export function RepoPanel({
  preview = false,
  session,
  scenarioName = null,
  templateName = null,
  terminalStatus = "idle",
  repoState = { status: "idle", snapshot: null, error: null },
}: RepoPanelProps) {
  if (preview || !session) {
    return (
      <aside className="workbench-side">
        <div className="panel-header">
          <span>Repository</span>
          <span className="panel-kicker">operational shell</span>
        </div>
        <section className="repo-state-card repo-state-card-shell" aria-label="Operational session card">
          <div className="repo-state-header">
            <span className="repo-state-health repo-state-health-idle">Preview</span>
            <div className="repo-state-heading">
              <strong>Sandbox status</strong>
              <p>Operational details appear after a live session is attached.</p>
            </div>
          </div>
        </section>
      </aside>
    );
  }

  const healthLabel = getHealthLabel(terminalStatus, session.status);
  const healthTone = getHealthTone(terminalStatus, session.status);
  const lifecycleFacts = [
    { label: "Started", value: formatDate(session.startedAt) },
    { label: "Last activity", value: formatDate(session.lastActivityAt) },
    { label: "Expires", value: formatDate(session.expiresAt) },
    { label: "Terminal", value: terminalStatus },
  ];

  return (
    <aside className="workbench-side">
      <div className="panel-header">
        <span>Repository</span>
        <span className="panel-kicker">operational status</span>
      </div>
      <section className="repo-state-card" aria-label="Operational session card">
        <div className="repo-state-header">
          <span className={`repo-state-health repo-state-health-${healthTone}`}>{healthLabel}</span>
          <div className="repo-state-heading">
            <strong>{formatScenarioName(scenarioName, session.scenarioId)}</strong>
            <span>{formatTemplateName(templateName, session.templateId)}</span>
          </div>
        </div>
        <dl className="repo-state-facts">
          <div>
            <dt>Runner</dt>
            <dd>{session.runnerRef}</dd>
          </div>
          <div>
            <dt>Workspace</dt>
            <dd className="repo-state-break">{session.workspacePath}</dd>
          </div>
          <div>
            <dt>Session ID</dt>
            <dd>{session.id}</dd>
          </div>
        </dl>
        <dl className="repo-state-lifecycle">
          {lifecycleFacts.map((fact) => (
            <div key={fact.label}>
              <dt>{fact.label}</dt>
              <dd>{fact.value}</dd>
            </div>
          ))}
        </dl>
        <section className="repo-state-snapshot-shell" aria-label="Repository snapshot">
          <div className="repo-state-snapshot-header">
            <strong>Repository snapshot</strong>
            {repoState.status === "loading" ? (
              <span className="repo-state-inline-note">Loading repository state...</span>
            ) : null}
            {repoState.status === "error" ? (
              <span className="repo-state-inline-note">Repository state unavailable.</span>
            ) : null}
          </div>
          {repoState.status === "ready" || repoState.status === "stale" ? (
            <>
              {repoState.status === "stale" && repoState.error ? (
                <span className="repo-state-inline-note">Repository state may be out of date.</span>
              ) : null}
              <dl className="repo-state-snapshot">
                <div>
                  <dt>Branch</dt>
                  <dd>{repoState.snapshot.branch}</dd>
                </div>
                <div>
                  <dt>HEAD</dt>
                  <dd title={repoState.snapshot.headCommit}>
                    {shortHead(repoState.snapshot.headCommit)}
                  </dd>
                </div>
                <div>
                  <dt>Working tree</dt>
                  <dd>{repoState.snapshot.dirty ? "Dirty" : "Clean"}</dd>
                </div>
              </dl>
              {repoState.snapshot.dirty ? (
                <ul className="repo-state-changes" aria-label="Changed files">
                  {repoState.snapshot.changedFiles.map((entry) => (
                    <li key={entry}>{entry}</li>
                  ))}
                </ul>
              ) : null}
            </>
          ) : null}
        </section>
      </section>
    </aside>
  );
}
