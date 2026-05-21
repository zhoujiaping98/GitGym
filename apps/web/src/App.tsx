import { useEffect, useRef, useState } from "react";
import { LoginScreen } from "./components/LoginScreen";
import { ScenarioPickerModal } from "./components/ScenarioPickerModal";
import { TopBar } from "./components/TopBar";
import { Workbench } from "./components/Workbench";
import { useCurrentSession } from "./hooks/useCurrentSession";
import { useTerminalSession } from "./hooks/useTerminalSession";
import {
  createPracticeSession,
  fetchPracticeCatalog,
  logout,
  resetPracticeSession,
} from "./lib/api";
import type { PracticeCatalog, PracticeSession } from "./types";

function templateLabel(templateId: number | null, catalog: PracticeCatalog | null) {
  const template = catalog?.templates.find((entry) => entry.id === templateId);
  if (template) {
    return `Template: ${template.name}`;
  }

  if (templateId === 1) {
    return "Template: Standard";
  }

  return templateId ? `Template #${templateId}` : "Template: Standard";
}

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

type CatalogState =
  | { status: "idle"; catalog: null; error: null }
  | { status: "loading"; catalog: null; error: null }
  | { status: "ready"; catalog: PracticeCatalog; error: null }
  | { status: "error"; catalog: null; error: string };

type ScenarioPickerSource = "topbar" | "empty" | "orphaned";

type ScenarioPickerState =
  | { status: "closed" }
  | {
      status: "open";
      source: ScenarioPickerSource;
      selectedScenarioId: number | null;
      error: string | null;
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
  const [catalogRequestKey, setCatalogRequestKey] = useState(0);
  const [catalogState, setCatalogState] = useState<CatalogState>({
    status: "idle",
    catalog: null,
    error: null,
  });
  const [scenarioPickerState, setScenarioPickerState] = useState<ScenarioPickerState>({
    status: "closed",
  });
  const unavailableRefreshSessionIdRef = useRef<number | null>(null);
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
  const hasOrphanedSessionState =
    !hasActiveSession &&
    !signedOutOverride &&
    currentSession.status === "ready" &&
    currentSession.absenceReason === "orphaned";
  const shouldShowCatalogState =
    !signedOutOverride &&
    currentSession.status === "ready" &&
    currentSession.absenceReason !== "unauthenticated";
  const catalog = catalogState.status === "ready" ? catalogState.catalog : null;
  const defaultScenario = catalog?.scenarios[0] ?? null;
  const scenarioOptions =
    catalog?.scenarios.map((scenario) => {
      const template = catalog.templates.find((entry) => entry.id === scenario.templateId);
      return {
        id: scenario.id,
        name: scenario.name,
        key: scenario.key,
        templateName: template?.name ?? `Template #${scenario.templateId}`,
      };
    }) ?? [];
  const hasEmptyCatalogState =
    !hasActiveSession &&
    shouldShowCatalogState &&
    catalogState.status === "ready" &&
    defaultScenario === null;
  const canUseScenarioPicker =
    !signedOutOverride &&
    currentSession.status === "ready" &&
    currentSession.absenceReason !== "unauthenticated" &&
    catalogState.status === "ready" &&
    defaultScenario !== null;
  const shouldShowPassiveEmptyState =
    hasAuthenticatedEmptyState &&
    !actionError &&
    !(hasAttemptedAutoCreate && scenarioPickerState.status === "closed");
  const displayedScenario =
    displayedSession && catalog
      ? catalog.scenarios.find((entry) => entry.id === displayedSession.scenarioId) ?? null
      : null;
  const displayedTemplate =
    displayedSession && catalog
      ? catalog.templates.find((entry) => entry.id === displayedSession.templateId) ?? null
      : null;

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
    if (
      signedOutOverride ||
      (currentSession.status === "ready" && currentSession.absenceReason === "unauthenticated")
    ) {
      setCatalogState((previous) =>
        previous.status === "idle" ? previous : { status: "idle", catalog: null, error: null },
      );
      return;
    }

    const controller = new AbortController();
    setCatalogState({ status: "loading", catalog: null, error: null });

    void fetchPracticeCatalog(controller.signal)
      .then((nextCatalog) => {
        setCatalogState({ status: "ready", catalog: nextCatalog, error: null });
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted) {
          return;
        }

        setCatalogState({
          status: "error",
          catalog: null,
          error:
            error instanceof Error ? error.message : "Unable to load the practice catalog.",
        });
      });

    return () => controller.abort();
  }, [catalogRequestKey, currentSession.absenceReason, currentSession.status, signedOutOverride]);

  useEffect(() => {
    if (!hasAuthenticatedEmptyState) {
      setHasAttemptedAutoCreate(false);
      return;
    }

    if (catalogState.status !== "ready" || !defaultScenario) {
      return;
    }

    if (hasAttemptedAutoCreate || actionError || pendingAction === "new-session") {
      return;
    }

    setHasAttemptedAutoCreate(true);
    openScenarioPicker("empty");
  }, [
    actionError,
    catalogState.status,
    defaultScenario,
    hasAttemptedAutoCreate,
    hasAuthenticatedEmptyState,
    pendingAction,
  ]);

  useEffect(() => {
    if (!displayedSession || terminalSession.status !== "unavailable") {
      unavailableRefreshSessionIdRef.current = null;
      return;
    }

    if (unavailableRefreshSessionIdRef.current === displayedSession.id) {
      return;
    }

    unavailableRefreshSessionIdRef.current = displayedSession.id;
    void currentSession.refresh().catch(() => undefined);
  }, [currentSession, displayedSession, terminalSession.status]);

  useEffect(() => {
    if (scenarioPickerState.status !== "open" || canUseScenarioPicker) {
      return;
    }

    setScenarioPickerState({ status: "closed" });
  }, [canUseScenarioPicker, scenarioPickerState.status]);

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

  function retryCatalogLoad() {
    if (signedOutOverride) {
      return;
    }

    setCatalogRequestKey((value) => value + 1);
  }

  function openScenarioPicker(source: ScenarioPickerSource) {
    if (!canUseScenarioPicker) {
      return;
    }

    setActionError(null);
    setScenarioPickerState({
      status: "open",
      source,
      selectedScenarioId: defaultScenario.id,
      error: null,
    });
  }

  function closeScenarioPicker() {
    if (pendingAction === "new-session") {
      return;
    }

    setScenarioPickerState({ status: "closed" });
  }

  function selectScenario(scenarioId: number) {
    setScenarioPickerState((previous) =>
      previous.status !== "open"
        ? previous
        : { ...previous, selectedScenarioId: scenarioId, error: null },
    );
  }

  function confirmScenarioPicker() {
    if (
      scenarioPickerState.status !== "open" ||
      scenarioPickerState.selectedScenarioId === null ||
      !canUseScenarioPicker ||
      !scenarioOptions.some((scenario) => scenario.id === scenarioPickerState.selectedScenarioId)
    ) {
      if (scenarioPickerState.status === "open" && !canUseScenarioPicker) {
        setScenarioPickerState({ status: "closed" });
      }
      return;
    }

    const selectedScenarioId = scenarioPickerState.selectedScenarioId;
    const fallbackSession = displayedSession;

    setPendingAction("new-session");
    void createPracticeSession({ scenarioId: selectedScenarioId })
      .then((nextSession) => {
        setScenarioPickerState({ status: "closed" });
        if (!fallbackSession) {
          setSessionOverride(nextSession);
        }
        return reconcileSessionAction("new-session", nextSession.id, {
          fallbackSession,
          optimisticSession: nextSession,
        });
      })
      .catch((error: unknown) => {
        setScenarioPickerState((previous) =>
          previous.status !== "open"
            ? previous
            : {
                ...previous,
                error:
                  error instanceof Error ? error.message : "Unable to create a new session.",
              },
        );
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
            openScenarioPicker("topbar");
          },
          disabled: pendingAction !== null || catalogState.status !== "ready" || !defaultScenario,
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
                void currentSession.refresh().catch(() => undefined);
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
      : hasOrphanedSessionState
      ? "error"
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
      : hasOrphanedSessionState
        ? "Workspace unavailable"
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
        metaLabel={templateLabel(displayedSession?.templateId ?? null, catalog)}
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
          <Workbench
            session={displayedSession}
            terminal={terminalSession}
            scenarioName={displayedScenario?.name ?? null}
            templateName={displayedTemplate?.name ?? null}
          />
        </main>
      ) : currentSession.status === "loading" ? (
        <AppStateShell
          eyebrow="Restoring session"
          title="Checking session"
          body="Restoring your practice workbench."
        />
      ) : catalogState.status === "loading" && shouldShowCatalogState ? (
        <AppStateShell
          eyebrow="Catalog readying"
          title="Loading practice catalog"
          body="Checking which practice scenarios are available for this workspace."
        />
      ) : catalogState.status === "error" && shouldShowCatalogState ? (
        <AppStateShell
          eyebrow="Catalog unavailable"
          title="Practice catalog unavailable"
          body="We couldn’t load the available practice scenarios for this environment."
          detail={catalogState.error}
          actionLabel="Try again"
          onAction={retryCatalogLoad}
        />
      ) : hasEmptyCatalogState ? (
        <AppStateShell
          eyebrow="Catalog empty"
          title="Practice catalog empty"
          body="There are no practice scenarios available for this environment."
          detail="Ask an administrator to publish at least one scenario before creating a session."
        />
      ) : shouldShowPassiveEmptyState ? (
        <AppStateShell
          eyebrow="Workspace ready"
          title="Preparing your workspace"
          body="Creating a disposable sandbox so you can land directly in the terminal."
          detail="We will start you in the first available practice scenario."
        />
      ) : hasAuthenticatedEmptyState ? (
        <AppStateShell
          eyebrow="Workspace ready"
          title="Create your first practice session"
          body="Your GitHub login is active. Start a disposable sandbox before you try the command sequence."
          detail={actionError?.message ?? null}
          actionLabel="New Session"
          onAction={() => {
            setHasAttemptedAutoCreate(true);
            openScenarioPicker("empty");
          }}
        />
      ) : hasOrphanedSessionState ? (
        <AppStateShell
          eyebrow="Workspace recovery"
          title="Workspace unavailable"
          body="Your previous sandbox can no longer be reopened. Start a fresh session to keep practicing."
          detail={actionError?.message ?? currentSession.error}
          actionLabel="New Session"
          onAction={() => {
            setHasAttemptedAutoCreate(true);
            openScenarioPicker("orphaned");
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
      <ScenarioPickerModal
        body="Pick a sandbox before creating the next session."
        confirmLabel="Start Session"
        error={scenarioPickerState.status === "open" ? scenarioPickerState.error : null}
        onClose={closeScenarioPicker}
        onConfirm={confirmScenarioPicker}
        onSelect={selectScenario}
        open={scenarioPickerState.status === "open"}
        pending={pendingAction === "new-session"}
        scenarios={scenarioOptions}
        selectedScenarioId={
          scenarioPickerState.status === "open" ? scenarioPickerState.selectedScenarioId : null
        }
        title="Choose a practice scenario"
      />
    </div>
  );
}
