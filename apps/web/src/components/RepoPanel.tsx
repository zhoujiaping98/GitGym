import type { PracticeSession, TerminalSessionState } from "../types";

type RepoPanelProps = {
  preview?: boolean;
  session: PracticeSession | null;
  scenarioName?: string | null;
  templateName?: string | null;
  terminalStatus?: TerminalSessionState["status"];
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
  if (status === "unavailable" || status === "error" || status === "connecting") {
    return "Recovering";
  }

  if (status === "ready" && sessionStatus === "active") {
    return "Live";
  }

  return "Recovering";
}

function getHealthTone(status: TerminalSessionState["status"], sessionStatus: string) {
  if (status === "unavailable" || status === "error" || status === "connecting") {
    return "degraded";
  }

  if (status === "ready" && sessionStatus === "active") {
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

export function RepoPanel({
  preview = false,
  session,
  scenarioName = null,
  templateName = null,
  terminalStatus = "idle",
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
              <strong>{formatScenarioName(scenarioName, session?.scenarioId)}</strong>
              <span>{formatTemplateName(templateName, session?.templateId)}</span>
            </div>
          </div>
          <p className="repo-state-shell-copy">
            Session infrastructure appears here once a live workspace is attached.
          </p>
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
      </section>
    </aside>
  );
}
