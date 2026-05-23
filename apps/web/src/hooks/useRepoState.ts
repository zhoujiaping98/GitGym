import { useEffect, useState } from "react";
import { fetchPracticeRepoState } from "../lib/api";
import type { PracticeSession, RepoStateView } from "../types";

const idleState: RepoStateView = {
  status: "idle",
  snapshot: null,
  error: null,
};

export function useRepoState(session: PracticeSession | null): RepoStateView {
  const [state, setState] = useState<RepoStateView>(idleState);

  useEffect(() => {
    if (!session) {
      setState(idleState);
      return;
    }

    const controller = new AbortController();
    setState({
      status: "loading",
      snapshot: null,
      error: null,
    });

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

        setState({
          status: "error",
          snapshot: null,
          error:
            error instanceof Error ? error.message : "Unable to load repository state.",
        });
      });

    return () => controller.abort();
  }, [session]);

  return state;
}
