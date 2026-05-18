export type SessionAbsenceReason = "unauthenticated" | "missing";

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
