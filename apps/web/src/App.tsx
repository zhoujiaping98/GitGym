import { useEffect, useState } from "react";
import { LoginScreen } from "./components/LoginScreen";
import { TopBar } from "./components/TopBar";
import { Workbench } from "./components/Workbench";
import { useCurrentSession } from "./hooks/useCurrentSession";
import { useTerminalSession } from "./hooks/useTerminalSession";
import { createPracticeSession, resetPracticeSession } from "./lib/api";
import type { PracticeSession } from "./types";

function templateLabel(templateId: number | null) {
  if (templateId === 1) {
    return "Template: Standard";
  }

  return templateId ? `Template #${templateId}` : "Template: Standard";
}

type ActionErrorState = {
  message: string;
  retryExpectedSessionId?: number;
};

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
  const [sessionOverride, setSessionOverride] = useState<PracticeSession | null>(null);
  const [actionError, setActionError] = useState<ActionErrorState | null>(null);
  const [pendingAction, setPendingAction] = useState<"reset" | "new-session" | null>(null);
  const displayedSession = sessionOverride ?? currentSession.session;
  const terminalSession = useTerminalSession(displayedSession);
  const hasActiveSession =
    displayedSession !== null &&
    (currentSession.status === "ready" || sessionOverride !== null);

  useEffect(() => {
    if (
      sessionOverride &&
      currentSession.status === "ready" &&
      currentSession.session?.id === sessionOverride.id
    ) {
      setSessionOverride(null);
    }
  }, [currentSession.session, currentSession.status, sessionOverride]);

  async function reconcileSessionAction(
    action: "reset" | "new-session",
    expectedSessionId: number,
  ) {
    try {
      const refreshedSession = await currentSession.refresh();

      if (!refreshedSession) {
        setSessionOverride(null);
        setActionError({
          message:
            action === "new-session"
              ? "Created a new session, but the server did not return it as current."
              : "Reset completed, but the server did not return a current session.",
        });
        return;
      }

      if (refreshedSession.id !== expectedSessionId) {
        setSessionOverride(refreshedSession);
        setActionError({
          message:
            action === "new-session"
              ? `Created session #${expectedSessionId}, but the server returned session #${refreshedSession.id}.`
              : `Reset session #${expectedSessionId}, but the server returned session #${refreshedSession.id}.`,
          retryExpectedSessionId: expectedSessionId,
        });
        return;
      }

      setActionError(null);
    } catch (error) {
      setSessionOverride(null);
      setActionError({
        message: `${
          action === "new-session"
            ? "Created a new session"
            : "Reset completed"
        }, but refreshing it failed: ${
          error instanceof Error ? error.message : "Unable to refresh the current session."
        }`,
        retryExpectedSessionId: expectedSessionId,
      });
    }
  }

  async function retrySessionRefresh() {
    try {
      const refreshedSession = await currentSession.refresh();
      if (!actionError?.retryExpectedSessionId) {
        setActionError(null);
        return;
      }
      if (!refreshedSession) {
        setSessionOverride(null);
        setActionError({
          message: "The server did not return a current session.",
          retryExpectedSessionId: actionError.retryExpectedSessionId,
        });
        return;
      }
      if (refreshedSession.id !== actionError.retryExpectedSessionId) {
        setSessionOverride(refreshedSession);
        setActionError({
          message: `Expected session #${actionError.retryExpectedSessionId}, but the server returned session #${refreshedSession.id}.`,
          retryExpectedSessionId: actionError.retryExpectedSessionId,
        });
        return;
      }

      setSessionOverride(refreshedSession);
      setActionError(null);
    } catch (error) {
      setActionError({
        message: error instanceof Error ? error.message : "Unable to refresh the current session.",
        retryExpectedSessionId: actionError?.retryExpectedSessionId,
      });
    }
  }

  const topBarActions = hasActiveSession
    ? [
        {
          label: "New Session",
          onClick: () => {
            const session = displayedSession;
            if (!session) {
              return;
            }

            setActionError(null);
            setPendingAction("new-session");
            void createPracticeSession({
              scenarioId: session.scenarioId,
              templateId: session.templateId,
            })
              .then((nextSession) => {
                setSessionOverride(nextSession);
                return reconcileSessionAction("new-session", nextSession.id);
              })
              .catch((error: unknown) => {
                setActionError({
                  message: error instanceof Error ? error.message : "Unable to create a new session.",
                });
              })
              .finally(() => {
                setPendingAction(null);
              });
          },
          disabled: pendingAction !== null,
        },
        {
          label: "Reset",
          onClick: () => {
            const session = displayedSession;
            if (!session) {
              return;
            }

            setActionError(null);
            setPendingAction("reset");
            void resetPracticeSession(session.id)
              .then(() => {
                return reconcileSessionAction("reset", session.id);
              })
              .catch((error: unknown) => {
                setActionError({
                  message: error instanceof Error ? error.message : "Unable to reset this session.",
                });
              })
              .finally(() => {
                setPendingAction(null);
              });
          },
          disabled: pendingAction !== null,
        },
      ]
    : [];
  const sessionTone =
    hasActiveSession
      ? "active"
      : currentSession.status === "loading"
      ? "pending"
      : currentSession.status === "error"
      ? "error"
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
        actions={topBarActions}
        metaLabel={templateLabel(displayedSession?.templateId ?? null)}
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
            {actionError ? (
              <div className="session-state-detail">
                <div>{actionError.message}</div>
                {actionError.retryExpectedSessionId ? (
                  <button className="top-bar-button" onClick={() => void retrySessionRefresh()} type="button">
                    Retry sync
                  </button>
                ) : null}
              </div>
            ) : null}
          </div>
          <Workbench session={displayedSession} terminal={terminalSession} />
        </main>
      ) : currentSession.status === "loading" ? (
        <AppStateShell
          eyebrow="Restoring session"
          title="Checking session"
          body="Restoring your practice workbench."
        />
      ) : currentSession.status === "error" || actionError ? (
        <AppStateShell
          eyebrow={actionError ? "Session reconciliation" : "Session lookup"}
          title="Session unavailable"
          body={
            actionError
              ? "We could not reconcile your current practice session."
              : "We could not restore your current practice session."
          }
          detail={actionError?.message ?? currentSession.error}
          actionLabel="Try again"
          onAction={() => {
            setActionError(null);
            void currentSession.refresh().catch(() => undefined);
          }}
        />
      ) : (
        <LoginScreen preview={<Workbench preview />} />
      )}
    </div>
  );
}
