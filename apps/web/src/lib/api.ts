import type {
  PracticeCatalog,
  PracticeScenario,
  PracticeSession,
  PracticeTemplate,
  SessionAbsenceReason,
} from "../types";

const API_BASE = "/api/v1";

type ApiResponse<T> = {
  status: number;
  data: T | null;
};

type CurrentSessionPayload = {
  session: SessionResponse;
};

type CreateSessionInput = {
  scenarioId: number;
};

type ResetSessionPayload = {
  status: string;
};

type CatalogResponse = {
  templates: Array<{ id: number; key: string; name: string }>;
  scenarios: Array<{
    id: number;
    key: string;
    name: string;
    template_id: number;
  }>;
};

type CurrentSessionLookup = {
  session: PracticeSession | null;
  absenceReason: SessionAbsenceReason | null;
  detail?: string | null;
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

function toPracticeTemplate(template: CatalogResponse["templates"][number]): PracticeTemplate {
  return {
    id: template.id,
    key: template.key,
    name: template.name,
  };
}

function toPracticeScenario(scenario: CatalogResponse["scenarios"][number]): PracticeScenario {
  return {
    id: scenario.id,
    key: scenario.key,
    name: scenario.name,
    templateId: scenario.template_id,
  };
}

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

  try {
    return {
      status: response.status,
      data: JSON.parse(text) as T,
    };
  } catch {
    return {
      status: response.status,
      data: null,
    };
  }
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
): Promise<CurrentSessionLookup> {
  const response = await fetch(`${API_BASE}/practice-sessions/current`, {
    credentials: "include",
    headers: {
      Accept: "application/json",
    },
    signal,
  });

  if (response.status === 401) {
    return {
      session: null,
      absenceReason: "unauthenticated",
    };
  }

  if (response.status === 404) {
    return {
      session: null,
      absenceReason: "missing",
    };
  }

  const payload = await readJson<CurrentSessionPayload | { error?: string }>(
    response,
  );

  if (response.status === 410) {
    const message =
      payload.data && "error" in payload.data && payload.data.error
        ? payload.data.error
        : "Current session workspace is unavailable.";
    return {
      session: null,
      absenceReason: "orphaned",
      detail: message,
    };
  }

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

  return {
    session: toPracticeSession(payload.data.session),
    absenceReason: null,
  };
}

export async function fetchPracticeCatalog(
  signal?: AbortSignal,
): Promise<PracticeCatalog> {
  const response = await fetch(`${API_BASE}/templates`, {
    credentials: "include",
    headers: {
      Accept: "application/json",
    },
    signal,
  });

  const payload = await readJson<CatalogResponse | { error?: string }>(response);
  if (!response.ok) {
    const message =
      payload.data && "error" in payload.data && payload.data.error
        ? payload.data.error
        : "Request failed";
    throw new ApiError(message, response.status);
  }
  if (!payload.data || !("templates" in payload.data) || !("scenarios" in payload.data)) {
    throw new ApiError("Catalog response was malformed", response.status);
  }

  return {
    templates: payload.data.templates.map(toPracticeTemplate),
    scenarios: payload.data.scenarios.map(toPracticeScenario),
  };
}

export async function createPracticeSession(
  input: CreateSessionInput,
): Promise<PracticeSession> {
  const response = await fetch(`${API_BASE}/practice-sessions`, {
    method: "POST",
    credentials: "include",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      scenario_id: input.scenarioId,
    }),
  });

  const payload = await readJson<CurrentSessionPayload | { error?: string }>(response);
  if (!response.ok) {
    const message =
      payload.data && "error" in payload.data && payload.data.error
        ? payload.data.error
        : "Request failed";
    throw new ApiError(message, response.status);
  }
  if (!payload.data || !("session" in payload.data)) {
    throw new ApiError("Create session response was malformed", response.status);
  }

  return toPracticeSession(payload.data.session);
}

export async function resetPracticeSession(sessionId: number): Promise<void> {
  const response = await fetch(`${API_BASE}/practice-sessions/${sessionId}/reset`, {
    method: "POST",
    credentials: "include",
    headers: {
      Accept: "application/json",
    },
  });

  const payload = await readJson<ResetSessionPayload | { error?: string }>(response);
  if (!response.ok) {
    const message =
      payload.data && "error" in payload.data && payload.data.error
        ? payload.data.error
        : "Request failed";
    throw new ApiError(message, response.status);
  }
}

export async function logout(): Promise<void> {
  const response = await fetch(`${API_BASE}/auth/logout`, {
    method: "POST",
    credentials: "include",
  });

  const payload = await readJson<{ error?: string }>(response);
  if (!response.ok) {
    const message =
      payload.data && "error" in payload.data && payload.data.error
        ? payload.data.error
        : "Request failed";
    throw new ApiError(message, response.status);
  }
}
