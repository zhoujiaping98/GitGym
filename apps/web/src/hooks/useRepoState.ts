import { useEffect, useMemo, useRef, useState } from "react";
import { fetchPracticeRepoState } from "../lib/api";
import type {
  CommandHistoryEntry,
  PracticeSession,
  RepoAttribution,
  RepoRefreshTrigger,
  RepoStateView,
} from "../types";

const idleState: RepoStateView = {
  status: "idle",
  snapshot: null,
  error: null,
};

export type RepoRefreshContext = {
  trigger: RepoRefreshTrigger;
  commandId?: string;
  commandText?: string;
};

type UseRepoStateOptions = {
  session: PracticeSession | null;
  commandHistory: CommandHistoryEntry[];
  refreshContext: RepoRefreshContext;
};

type UseRepoStateResult = {
  repoState: RepoStateView;
  repoAttribution: RepoAttribution | null;
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
  const [refreshToken, setRefreshToken] = useState(0);
  const lastCompletedCommandKeyRef = useRef<string | null>(null);
  const lastSessionIdRef = useRef<number | null>(null);
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
      previousCompletedCountRef.current = 0;
      setState(idleState);
      setAttribution(null);
      return;
    }

    const controller = new AbortController();
    const isSameSession = lastSessionIdRef.current === session.id;
    lastSessionIdRef.current = session.id;
    if (!isSameSession) {
      setAttribution(null);
    }
    setState((current) =>
      isSameSession && current.snapshot
        ? {
            status: "stale",
            snapshot: current.snapshot,
            error: null,
          }
        : {
            status: "loading",
            snapshot: null,
            error: null,
          },
    );

    void fetchPracticeRepoState(session.id, controller.signal)
      .then((snapshot) => {
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
      })
      .catch((error: unknown) => {
        if (controller.signal.aborted) {
          return;
        }

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

  return {
    repoState: state,
    repoAttribution: attribution,
  };
}
