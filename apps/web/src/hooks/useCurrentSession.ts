import { useEffect, useState } from "react";
import { fetchCurrentSession } from "../lib/api";
import type { CurrentSessionState, PracticeSession } from "../types";

export function useCurrentSession(): CurrentSessionState {
  const [status, setStatus] = useState<CurrentSessionState["status"]>("loading");
  const [session, setSession] = useState<PracticeSession | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function load(signal?: AbortSignal): Promise<PracticeSession | null> {
    setError(null);

    try {
      const currentSession = await fetchCurrentSession(signal);
      setSession(currentSession);
      setStatus("ready");
      return currentSession;
    } catch (loadError) {
      if (signal?.aborted) {
        return null;
      }

      setSession(null);
      setStatus("error");
      const nextError =
        loadError instanceof Error
          ? loadError.message
          : "Unable to load current session.";
      setError(nextError);
      throw loadError instanceof Error ? loadError : new Error(nextError);
    }
  }

  useEffect(() => {
    const controller = new AbortController();
    void load(controller.signal).catch(() => undefined);
    return () => controller.abort();
  }, []);

  return {
    status,
    session,
    error,
    refresh: async () => {
      setStatus("loading");
      return load();
    },
  };
}
