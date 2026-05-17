export type TerminalClientMessage =
  | { type: "input"; data: string }
  | { type: "resize"; cols: number; rows: number }
  | { type: "ping" };

export type TerminalServerMessage =
  | { type: "ready"; cols: number; rows: number }
  | { type: "output"; data: string }
  | {
      type: "status";
      phase: "starting" | "running" | "stopped";
      detail?: string;
    }
  | { type: "exit"; exitCode: number | null }
  | { type: "error"; message: string };
