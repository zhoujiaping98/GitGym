import { useEffect, useState } from "react";
import { fetchCurrentUser } from "../lib/api";
import type { CurrentUser, CurrentUserState } from "../types";

export function useCurrentUser(): CurrentUserState {
  const [status, setStatus] = useState<CurrentUserState["status"]>("loading");
  const [user, setUser] = useState<CurrentUser | null>(null);
  const [error, setError] = useState<string | null>(null);

  async function load(signal?: AbortSignal): Promise<CurrentUser | null> {
    setError(null);

    try {
      const currentUser = await fetchCurrentUser(signal);
      setUser(currentUser);
      setStatus("ready");
      return currentUser;
    } catch (loadError) {
      if (signal?.aborted) {
        return null;
      }

      const nextError =
        loadError instanceof Error ? loadError.message : "Unable to load current user.";
      setUser(null);
      setStatus("error");
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
    user,
    error,
    refresh: async () => {
      setStatus("loading");
      return load();
    },
  };
}
