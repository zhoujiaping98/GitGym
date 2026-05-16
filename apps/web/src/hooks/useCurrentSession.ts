import { useEffect, useState } from "react";
import { fetchCurrentSession } from "../lib/api";
import type { CurrentSessionState, PracticeSession } from "../types";

export function useCurrentSession(): CurrentSessionState {
  const [status, setStatus] = useState<CurrentSessionState["status"]>("loading");
  const [session, setSession] = useState<PracticeSession | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function load(signal?: AbortSignal) {
    setError(null);

    try {
      const currentSession = await fetchCurrentSession(signal);
      setSession(currentSession);
      setStatus("ready");
    } catch (loadError) {
      if (signal?.aborted) {
        return;
      }

      setSession(null);
      setStatus("error");
      setError(
        loadError instanceof Error
          ? loadError.message
          : "Unable to load current session.",
      );
    }
  }

  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal);
    return () => controller.abort();
  }, []);

  return {
    status,
    session,
    error,
    refresh: async () => {
      setStatus("loading");
      await load();
    },
  };
}
