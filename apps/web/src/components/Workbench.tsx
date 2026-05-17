import { CommandHistoryPanel } from "./CommandHistoryPanel";
import { RepoPanel } from "./RepoPanel";
import { TerminalPanel } from "./TerminalPanel";
import type { PracticeSession, TerminalSessionState } from "../types";

type WorkbenchProps = {
  preview?: boolean;
  session?: PracticeSession | null;
  terminal?: TerminalSessionState;
};

const previewTerminal: TerminalSessionState = {
  status: "idle",
  transcript: [],
  history: [],
  terminalUrl: null,
  error: null,
  reconnect: () => undefined,
  sendInput: () => undefined,
  resize: () => undefined,
};

export function Workbench({
  preview = false,
  session = null,
  terminal = previewTerminal,
}: WorkbenchProps) {
  const shellClassName = preview
    ? "workbench-shell workbench-shell-preview"
    : "workbench-shell";

  return (
    <section className={shellClassName}>
      <TerminalPanel preview={preview} terminal={terminal} />
      <RepoPanel preview={preview} session={session} />
      <CommandHistoryPanel preview={preview} terminal={terminal} />
    </section>
  );
}
