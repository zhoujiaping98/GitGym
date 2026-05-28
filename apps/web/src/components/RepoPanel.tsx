import type {
  PracticeSession,
  RepoAttribution,
  RepoStateView,
  TerminalSessionState,
} from "../types";
import { groupRepoChanges } from "../lib/repoChanges";
import {
  summarizeRepoChanges,
  type SummarizedRepoChangeGroup,
  type SummarizedRepoChanges,
} from "../lib/repoChangeSummary";

type RepoPanelProps = {
  preview?: boolean;
  session: PracticeSession | null;
  scenarioName?: string | null;
  templateName?: string | null;
  terminalStatus?: TerminalSessionState["status"];
  repoState?: RepoStateView;
  repoAttribution?: RepoAttribution | null;
  repoOutcome?: string | null;
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

function repoAttributionCopy(attribution: RepoAttribution | null) {
  if (!attribution) {
    return null;
  }

  if (attribution.trigger === "session_load") {
    return "Snapshot loaded";
  }

  if (attribution.trigger === "session_create") {
    return "Snapshot refreshed after new session";
  }

  if (attribution.trigger === "session_reset") {
    return "Snapshot refreshed after reset";
  }

  if (attribution.trigger === "session_sync") {
    return "Snapshot refreshed after sync";
  }

  if (attribution.trigger === "command_complete" && attribution.commandText) {
    return `Updated after ${attribution.commandText}`;
  }

  return null;
}

function repoFreshnessCopy(capturedAt: string | null) {
  if (!capturedAt) {
    return null;
  }

  return `Captured ${formatDate(capturedAt)}`;
}

function renderChangeGroup(group: SummarizedRepoChangeGroup) {
  return (
    <section className="repo-state-change-group" aria-label={group.title} key={group.title}>
      <strong>{`${group.title} (${group.count})`}</strong>
      <ul className="repo-state-change-list">
        {group.visible.map((change) => (
          <li key={change.key}>
            <span className="repo-state-change-pill">{change.label}</span>
            <span>{change.path}</span>
          </li>
        ))}
        {group.hiddenCount > 0 ? (
          <li className="repo-state-change-more">{`+${group.hiddenCount} more`}</li>
        ) : null}
      </ul>
    </section>
  );
}

export function RepoPanel({
  preview = false,
  session,
  scenarioName = null,
  templateName = null,
  terminalStatus = "idle",
  repoState = { status: "idle", snapshot: null, error: null },
  repoAttribution = null,
  repoOutcome = null,
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
  const attributionCopy = repoAttributionCopy(repoAttribution);
  const freshnessCopy =
    repoState.status === "ready" || repoState.status === "stale"
      ? repoFreshnessCopy(repoState.snapshot.capturedAt)
      : null;
  const lifecycleFacts = [
    { label: "Started", value: formatDate(session.startedAt) },
    { label: "Last activity", value: formatDate(session.lastActivityAt) },
    { label: "Expires", value: formatDate(session.expiresAt) },
    { label: "Terminal", value: terminalStatus },
  ];
  const summarizedChanges: SummarizedRepoChanges | null =
    repoState.status === "ready" || repoState.status === "stale"
      ? summarizeRepoChanges(groupRepoChanges(repoState.snapshot.changedFiles))
      : null;

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
            {attributionCopy ? (
              <span className="repo-state-inline-note">{attributionCopy}</span>
            ) : null}
            {freshnessCopy ? <span className="repo-state-inline-note">{freshnessCopy}</span> : null}
            {repoOutcome ? <span className="repo-state-inline-note">{repoOutcome}</span> : null}
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
              {repoState.snapshot.dirty && summarizedChanges ? (
                <section className="repo-state-changes" aria-label="Changed files">
                  {summarizedChanges.groups.map((group) => renderChangeGroup(group))}
                  {summarizedChanges.fallback.visible.length > 0 ||
                  summarizedChanges.fallback.hiddenCount > 0 ? (
                    <ul className="repo-state-change-list repo-state-change-fallback">
                      {summarizedChanges.fallback.visible.map((line) => (
                        <li key={line}>{line}</li>
                      ))}
                      {summarizedChanges.fallback.hiddenCount > 0 ? (
                        <li className="repo-state-change-more">
                          {`+${summarizedChanges.fallback.hiddenCount} more`}
                        </li>
                      ) : null}
                    </ul>
                  ) : null}
                </section>
              ) : null}
            </>
          ) : null}
        </section>
      </section>
    </aside>
  );
}
