import { useEffect, useState } from "react";
import { LoginScreen } from "./components/LoginScreen";
import { TopBar } from "./components/TopBar";
import { Workbench } from "./components/Workbench";
import { useCurrentSession } from "./hooks/useCurrentSession";
import { useTerminalSession } from "./hooks/useTerminalSession";
import { createPracticeSession, logout, resetPracticeSession } from "./lib/api";
import type { PracticeSession } from "./types";

function templateLabel(templateId: number | null) {
  if (templateId === 1) {
    return "Template: Standard";
  }

  return templateId ? `Template #${templateId}` : "Template: Standard";
}

const defaultSandboxScenarioId = 9;
const defaultSandboxTemplateId = 1;

type ActionErrorState = {
  message: string;
  retryExpectedSessionId?: number;
};

type SessionReconcileOptions = {
  fallbackSession?: PracticeSession | null;
  optimisticSession?: PracticeSession | null;
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
  const [pendingAction, setPendingAction] = useState<"reset" | "new-session" | "logout" | null>(null);
  const [hasAttemptedAutoCreate, setHasAttemptedAutoCreate] = useState(false);
  const [signedOutOverride, setSignedOutOverride] = useState(false);
  const effectiveSession = signedOutOverride ? null : currentSession.session;
  const displayedSession = sessionOverride ?? effectiveSession;
  const terminalSession = useTerminalSession(displayedSession);
  const hasActiveSession =
    displayedSession !== null &&
    ((currentSession.status === "ready" && !signedOutOverride) || sessionOverride !== null);
  const hasAuthenticatedEmptyState =
    !hasActiveSession &&
    !signedOutOverride &&
    currentSession.status === "ready" &&
    currentSession.absenceReason === "missing";

  useEffect(() => {
    if (
      sessionOverride &&
      currentSession.status === "ready" &&
      effectiveSession?.id === sessionOverride.id
    ) {
      setSessionOverride(null);
    }
  }, [currentSession.status, effectiveSession, sessionOverride]);

  useEffect(() => {
    if (!hasAuthenticatedEmptyState) {
      setHasAttemptedAutoCreate(false);
      return;
    }

    if (hasAttemptedAutoCreate || actionError || pendingAction === "new-session") {
      return;
    }

    setHasAttemptedAutoCreate(true);
    startNewSession(defaultSandboxScenarioId, defaultSandboxTemplateId);
  }, [actionError, hasAttemptedAutoCreate, hasAuthenticatedEmptyState, pendingAction]);

  async function reconcileSessionAction(
    action: "reset" | "new-session",
    expectedSessionId: number,
    options: SessionReconcileOptions = {},
  ) {
    const fallbackSession = options.fallbackSession ?? null;
    const optimisticSession = options.optimisticSession ?? null;

    try {
      const refreshedSession = await currentSession.refresh();

      if (!refreshedSession) {
        setSessionOverride(fallbackSession ?? optimisticSession);
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

      setSessionOverride(refreshedSession);
      setActionError(null);
    } catch (error) {
      setSessionOverride(fallbackSession ?? optimisticSession);
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

  function startNewSession(scenarioId: number, templateId: number) {
    const fallbackSession = displayedSession;

    setActionError(null);
    setPendingAction("new-session");
    void createPracticeSession({
      scenarioId,
      templateId,
    })
      .then((nextSession) => {
        if (!fallbackSession) {
          setSessionOverride(nextSession);
        }
        return reconcileSessionAction("new-session", nextSession.id, {
          fallbackSession,
          optimisticSession: nextSession,
        });
      })
      .catch((error: unknown) => {
        setActionError({
          message: error instanceof Error ? error.message : "Unable to create a new session.",
        });
      })
      .finally(() => {
        setPendingAction(null);
      });
  }

  const topBarActions = hasActiveSession
    ? [
        {
          label: "New Session",
          onClick: () => {
            if (displayedSession) {
              startNewSession(displayedSession.scenarioId, displayedSession.templateId);
              return;
            }

            startNewSession(defaultSandboxScenarioId, defaultSandboxTemplateId);
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
                return reconcileSessionAction("reset", session.id, {
                  fallbackSession: session,
                  optimisticSession: session,
                });
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
        {
          label: "Logout",
          onClick: () => {
            setActionError(null);
            setPendingAction("logout");
            void logout()
              .then(() => {
                setSessionOverride(null);
                setSignedOutOverride(true);
                return currentSession.refresh();
              })
              .catch((error: unknown) => {
                setActionError({
                  message: error instanceof Error ? error.message : "Unable to log out.",
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
      : signedOutOverride
        ? "Signed out"
      : hasAuthenticatedEmptyState
        ? "Signed in"
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
      ) : hasAuthenticatedEmptyState && !actionError ? (
        <AppStateShell
          eyebrow="Workspace ready"
          title="Preparing your workspace"
          body="Creating a disposable sandbox so you can land directly in the terminal."
          detail="We will start you in the standard sandbox template."
        />
      ) : hasAuthenticatedEmptyState ? (
        <AppStateShell
          eyebrow="Workspace ready"
          title="Create your first practice session"
          body="Your GitHub login is active. Start a disposable sandbox before you try the command sequence."
          detail={actionError.message}
          actionLabel="New Session"
          onAction={() => {
            setHasAttemptedAutoCreate(true);
            startNewSession(defaultSandboxScenarioId, defaultSandboxTemplateId);
          }}
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
