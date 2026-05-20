import { useEffect, useState } from "react";
import { fetchCurrentSession } from "../lib/api";
import type {
  CurrentSessionState,
  PracticeSession,
  SessionAbsenceReason,
} from "../types";

export function useCurrentSession(): CurrentSessionState {
  const [status, setStatus] = useState<CurrentSessionState["status"]>("loading");
  const [session, setSession] = useState<PracticeSession | null>(null);
  const [absenceReason, setAbsenceReason] = useState<SessionAbsenceReason | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function load(
    signal?: AbortSignal,
    options?: { preserveSessionOnError?: boolean },
  ): Promise<PracticeSession | null> {
    setError(null);

    try {
      const currentSession = await fetchCurrentSession(signal);
      setSession(currentSession.session);
      setAbsenceReason(currentSession.absenceReason);
      setError(currentSession.detail ?? null);
      setStatus("ready");
      return currentSession.session;
    } catch (loadError) {
      if (signal?.aborted) {
        return null;
      }

      const nextError =
        loadError instanceof Error
          ? loadError.message
          : "Unable to load current session.";
      if (!options?.preserveSessionOnError) {
        setSession(null);
        setAbsenceReason(null);
        setStatus("error");
      } else {
        setStatus("ready");
      }
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
    absenceReason,
    error,
    refresh: async () => {
      setStatus("loading");
      return load(undefined, { preserveSessionOnError: session !== null });
    },
  };
}
