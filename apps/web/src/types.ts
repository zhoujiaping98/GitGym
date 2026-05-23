export type SessionAbsenceReason = "unauthenticated" | "missing" | "orphaned";

export type PracticeSession = {
  id: number;
  userId: number;
  scenarioId: number;
  templateId: number;
  runnerRef: string;
  workspacePath: string;
  status: string;
  startedAt: string;
  endedAt?: string | null;
  expiresAt: string;
  lastActivityAt: string;
};

export type PracticeTemplate = {
  id: number;
  key: string;
  name: string;
};

export type PracticeScenario = {
  id: number;
  key: string;
  name: string;
  templateId: number;
};

export type PracticeCatalog = {
  templates: PracticeTemplate[];
  scenarios: PracticeScenario[];
};

export type CurrentSessionState = {
  status: "loading" | "ready" | "error";
  session: PracticeSession | null;
  absenceReason: SessionAbsenceReason | null;
  error: string | null;
  refresh: () => Promise<PracticeSession | null>;
};

export type CommandHistoryEntry = {
  id: string;
  command: string;
  executedAt?: string;
  exitCode?: number | null;
  phase?: "running" | "stopped";
  summary?: string;
};

export type RepoStateSnapshot = {
  branch: string;
  headCommit: string;
  dirty: boolean;
  changedFiles: string[];
  capturedAt: string;
};

export type RepoStateView =
  | { status: "idle"; snapshot: null; error: null }
  | { status: "loading"; snapshot: null; error: null }
  | { status: "ready"; snapshot: RepoStateSnapshot; error: null }
  | { status: "error"; snapshot: null; error: string };

export type TerminalSessionState = {
  status: "idle" | "connecting" | "ready" | "unavailable" | "error";
  transcript: string[];
  history: CommandHistoryEntry[];
  terminalUrl: string | null;
  error: string | null;
  reconnect: () => void;
  sendInput: (data: string) => void;
  resize: (cols: number, rows: number) => void;
};
