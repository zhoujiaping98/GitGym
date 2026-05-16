import { LoginScreen } from "./components/LoginScreen";
import { TopBar } from "./components/TopBar";
import { Workbench } from "./components/Workbench";
import { useCurrentSession } from "./hooks/useCurrentSession";
import { useTerminalSession } from "./hooks/useTerminalSession";

function templateLabel(templateId: number | null) {
  if (templateId === 1) {
    return "Template: Standard";
  }

  return templateId ? `Template #${templateId}` : "Template: Standard";
}

type AppStateShellProps = {
  eyebrow: string;
  title: string;
  body: string;
  detail?: string | null;
  actionLabel?: string;
  onAction?: () => void;
};

function AppStateShell({
  eyebrow,
  title,
  body,
  detail,
  actionLabel,
  onAction,
}: AppStateShellProps) {
  return (
    <main className="session-state-shell">
      <section className="session-state-card">
        <span className="preview-label">{eyebrow}</span>
        <h1>{title}</h1>
        <p>{body}</p>
        {detail ? <div className="session-state-detail">{detail}</div> : null}
        {actionLabel && onAction ? (
          <button className="primary-button session-state-action" onClick={onAction} type="button">
            {actionLabel}
          </button>
        ) : null}
      </section>
    </main>
  );
}

export default function App() {
  const currentSession = useCurrentSession();
  const terminalSession = useTerminalSession(currentSession.session);
  const hasActiveSession = currentSession.status === "ready" && currentSession.session;
  const sessionTone =
    currentSession.status === "loading"
      ? "pending"
      : currentSession.status === "error"
        ? "error"
        : hasActiveSession
          ? "active"
          : "idle";
  const sessionLabel =
    hasActiveSession
      ? "Session live"
      : currentSession.status === "loading"
        ? "Checking session"
        : currentSession.status === "error"
          ? "Session unavailable"
          : "Signed out";

  return (
    <div className="app-shell">
      <TopBar
        metaLabel={templateLabel(currentSession.session?.templateId ?? null)}
        sessionLabel={sessionLabel}
        tone={sessionTone}
      />
      {hasActiveSession ? (
        <main className="live-shell">
          <div className="live-shell-copy">
            <span className="preview-label">Active practice session</span>
            <h1>Real session state, same editorial shell.</h1>
            <p>
              The terminal is attached to your active workspace. Repository and
              command history panels stay visible without taking over the page.
            </p>
          </div>
          <Workbench session={currentSession.session} terminal={terminalSession} />
        </main>
      ) : currentSession.status === "loading" ? (
        <AppStateShell
          eyebrow="Restoring session"
          title="Checking session"
          body="Restoring your practice workbench."
        />
      ) : currentSession.status === "error" ? (
        <AppStateShell
          eyebrow="Session lookup"
          title="Session unavailable"
          body="We could not restore your current practice session."
          detail={currentSession.error}
          actionLabel="Try again"
          onAction={() => {
            void currentSession.refresh();
          }}
        />
      ) : (
        <LoginScreen preview={<Workbench preview />} />
      )}
    </div>
  );
}
