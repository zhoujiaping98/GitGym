import type { PracticeSession } from "../types";

const API_BASE = "/api/v1";

type ApiResponse<T> = {
  status: number;
  data: T | null;
};

type CurrentSessionPayload = {
  session: SessionResponse;
};

type SessionResponse = {
  id: number;
  user_id: number;
  scenario_id: number;
  template_id: number;
  runner_ref: string;
  workspace_path: string;
  status: string;
  started_at: string;
  ended_at?: string | null;
  expires_at: string;
  last_activity_at: string;
};

export class ApiError extends Error {
  status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

async function readJson<T>(response: Response): Promise<ApiResponse<T>> {
  if (response.status === 204) {
    return { status: response.status, data: null };
  }

  const text = await response.text();
  if (!text) {
    return { status: response.status, data: null };
  }

  return {
    status: response.status,
    data: JSON.parse(text) as T,
  };
}

function toPracticeSession(session: SessionResponse): PracticeSession {
  return {
    id: session.id,
    userId: session.user_id,
    scenarioId: session.scenario_id,
    templateId: session.template_id,
    runnerRef: session.runner_ref,
    workspacePath: session.workspace_path,
    status: session.status,
    startedAt: session.started_at,
    endedAt: session.ended_at ?? null,
    expiresAt: session.expires_at,
    lastActivityAt: session.last_activity_at,
  };
}

export async function fetchCurrentSession(
  signal?: AbortSignal,
): Promise<PracticeSession | null> {
  const response = await fetch(`${API_BASE}/practice-sessions/current`, {
    credentials: "include",
    headers: {
      Accept: "application/json",
    },
    signal,
  });

  if (response.status === 401 || response.status === 404) {
    return null;
  }

  const payload = await readJson<CurrentSessionPayload | { error?: string }>(
    response,
  );

  if (!response.ok) {
    const message =
      payload.data && "error" in payload.data && payload.data.error
        ? payload.data.error
        : "Request failed";
    throw new ApiError(message, response.status);
  }

  if (!payload.data || !("session" in payload.data)) {
    throw new ApiError("Current session response was malformed", response.status);
  }

  return toPracticeSession(payload.data.session);
}
