import { useEffect, useMemo, useRef, useState } from "react";
import { fetchPracticeRepoState } from "../lib/api";
import { repoOutcomeCopy } from "../lib/repoOutcome";
import type {
  CommandHistoryEntry,
  PracticeSession,
  RepoAttribution,
  RepoRefreshContext,
  RepoStateSnapshot,
  RepoStateView,
} from "../types";

const idleState: RepoStateView = {
  status: "idle",
  snapshot: null,
  error: null,
};

type UseRepoStateOptions = {
  session: PracticeSession | null;
  commandHistory: CommandHistoryEntry[];
  refreshContext: RepoRefreshContext;
};

type UseRepoStateResult = {
  repoState: RepoStateView;
  repoAttribution: RepoAttribution | null;
  repoOutcome: string | null;
  retryRepoState: (() => void) | null;
  isRefreshingRepoState: boolean;
};

function getRepoStateError(error: unknown) {
  return error instanceof Error ? error.message : "Unable to load repository state.";
}

export function useRepoState({
  session,
  commandHistory,
  refreshContext,
}: UseRepoStateOptions): UseRepoStateResult {
  const [state, setState] = useState<RepoStateView>(idleState);
  const [attribution, setAttribution] = useState<RepoAttribution | null>(null);
  const [outcome, setOutcome] = useState<string | null>(null);
  const [isRefreshingRepoState, setIsRefreshingRepoState] = useState(false);
  const [refreshToken, setRefreshToken] = useState(0);
  const lastCompletedCommandKeyRef = useRef<string | null>(null);
  const lastSessionIdRef = useRef<number | null>(null);
  const lastSuccessfulSnapshotRef = useRef<RepoStateSnapshot | null>(null);
  const previousCompletedCountRef = useRef(0);
  const completedCommands = useMemo(
    () => commandHistory.filter((entry) => entry.phase === "stopped"),
    [commandHistory],
  );
  const latestCompletedCommandId = completedCommands.at(-1)?.id ?? null;
  const completedCommandCount = completedCommands.length;
  const latestCompletedCommandKey =
    latestCompletedCommandId === null ? null : `${completedCommandCount}:${latestCompletedCommandId}`;

  useEffect(() => {
    if (!session) {
      lastCompletedCommandKeyRef.current = null;
      lastSuccessfulSnapshotRef.current = null;
      previousCompletedCountRef.current = 0;
      setState(idleState);
      setAttribution(null);
      setOutcome(null);
      setIsRefreshingRepoState(false);
      return;
    }

    const controller = new AbortController();
    const isSameSession = lastSessionIdRef.current === session.id;
    lastSessionIdRef.current = session.id;
    if (!isSameSession) {
      lastSuccessfulSnapshotRef.current = null;
      setAttribution(null);
      setOutcome(null);
      setIsRefreshingRepoState(false);
    }
    setState((current) =>
      isSameSession && current.snapshot
        ? {
            status: "stale",
            snapshot: current.snapshot,
            error: current.error,
          }
        : {
            status: "loading",
            snapshot: null,
            error: null,
          },
    );

    void fetchPracticeRepoState(session.id, controller.signal)
      .then((snapshot) => {
        setIsRefreshingRepoState(false);
        const previousSnapshot = lastSuccessfulSnapshotRef.current;
        lastSuccessfulSnapshotRef.current = snapshot;
        setState({
          status: "ready",
          snapshot,
          error: null,
        });
        setAttribution({
          trigger: refreshContext.trigger,
          capturedAt: snapshot.capturedAt,
          commandId: refreshContext.commandId,
          commandText: refreshContext.commandText,
        });
        if (refreshContext.trigger === "command_complete" && previousSnapshot) {
          setOutcome(repoOutcomeCopy(previousSnapshot, snapshot));
          return;
        }

        setOutcome(null);
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted) {
          return;
        }

        setIsRefreshingRepoState(false);
        const message = getRepoStateError(error);
        setState((current) =>
          current.snapshot
            ? {
                status: "stale",
                snapshot: current.snapshot,
                error: message,
              }
            : {
                status: "error",
                snapshot: null,
                error: message,
              },
        );
      });

    return () => controller.abort();
  }, [refreshContext, refreshToken, session]);

  useEffect(() => {
    if (!session) {
      lastSessionIdRef.current = null;
      return;
    }

    lastCompletedCommandKeyRef.current = latestCompletedCommandKey;
    previousCompletedCountRef.current = completedCommandCount;
  }, [session?.id]);

  useEffect(() => {
    if (!session) {
      return;
    }

    if (completedCommandCount < previousCompletedCountRef.current || commandHistory.length === 0) {
      lastCompletedCommandKeyRef.current = null;
    }
    previousCompletedCountRef.current = completedCommandCount;

    if (!latestCompletedCommandKey) {
      return;
    }

    if (lastCompletedCommandKeyRef.current === latestCompletedCommandKey) {
      return;
    }

    lastCompletedCommandKeyRef.current = latestCompletedCommandKey;
    setRefreshToken((value) => value + 1);
  }, [commandHistory.length, completedCommandCount, latestCompletedCommandKey, session]);

  const retryRepoState = session
    ? () => {
        setIsRefreshingRepoState(true);
        setRefreshToken((value) => value + 1);
      }
    : null;

  return {
    repoState: state,
    repoAttribution: attribution,
    repoOutcome: outcome,
    retryRepoState,
    isRefreshingRepoState,
  };
}
