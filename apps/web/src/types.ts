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
  error: string | null;
  refresh: () => Promise<void>;
};

export type CommandHistoryEntry = {
  id: string;
  command: string;
  executedAt?: string;
  exitCode?: number | null;
  summary?: string;
};

export type TerminalSessionState = {
  status: "idle" | "connecting" | "ready" | "unavailable" | "error";
  transcript: string[];
  history: CommandHistoryEntry[];
  terminalUrl: string | null;
  error: string | null;
  reconnect: () => void;
};
