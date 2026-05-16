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

export default function App() {
  const currentSession = useCurrentSession();
  const terminalSession = useTerminalSession(currentSession.session);
  const hasActiveSession = currentSession.status === "ready" && currentSession.session;

  return (
    <div className="app-shell">
      <TopBar
        metaLabel={templateLabel(currentSession.session?.templateId ?? null)}
        sessionLabel={
          hasActiveSession
            ? "Session live"
            : currentSession.status === "loading"
              ? "Checking session"
              : "Signed out"
        }
        tone={hasActiveSession ? "active" : "idle"}
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
      ) : (
        <LoginScreen preview={<Workbench preview />} />
      )}
    </div>
  );
}
