import { useEffect, useMemo, useRef, useState } from "react";
import { fetchPracticeRepoState } from "../lib/api";
import type { CommandHistoryEntry, PracticeSession, RepoStateView } from "../types";

const idleState: RepoStateView = {
  status: "idle",
  snapshot: null,
  error: null,
};

type UseRepoStateOptions = {
  session: PracticeSession | null;
  commandHistory: CommandHistoryEntry[];
};

function getRepoStateError(error: unknown) {
  return error instanceof Error ? error.message : "Unable to load repository state.";
}

export function useRepoState({
  session,
  commandHistory,
}: UseRepoStateOptions): RepoStateView {
  const [state, setState] = useState<RepoStateView>(idleState);
  const [refreshToken, setRefreshToken] = useState(0);
  const lastCompletedCommandIdRef = useRef<string | null>(null);
  const lastSessionIdRef = useRef<number | null>(null);
  const latestCompletedCommandId = useMemo(
    () =>
      [...commandHistory].reverse().find((entry) => entry.phase === "stopped")?.id ?? null,
    [commandHistory],
  );

  useEffect(() => {
    if (!session) {
      lastCompletedCommandIdRef.current = null;
      setState(idleState);
      return;
    }

    const controller = new AbortController();
    const isSameSession = lastSessionIdRef.current === session.id;
    lastSessionIdRef.current = session.id;
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
  }, [refreshToken, session]);

  useEffect(() => {
    if (!session) {
      lastSessionIdRef.current = null;
      return;
    }

    lastCompletedCommandIdRef.current = latestCompletedCommandId;
  }, [session?.id]);

  useEffect(() => {
    if (!session || !latestCompletedCommandId) {
      return;
    }

    if (lastCompletedCommandIdRef.current === latestCompletedCommandId) {
      return;
    }

    lastCompletedCommandIdRef.current = latestCompletedCommandId;
    setRefreshToken((value) => value + 1);
  }, [latestCompletedCommandId, session]);

  return state;
}
