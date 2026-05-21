import { CommandHistoryPanel } from "./CommandHistoryPanel";
import { RepoPanel } from "./RepoPanel";
import { TerminalPanel } from "./TerminalPanel";
import type { PracticeSession, TerminalSessionState } from "../types";

type WorkbenchProps = {
  preview?: boolean;
  session?: PracticeSession | null;
  terminal?: TerminalSessionState;
  scenarioName?: string | null;
  templateName?: string | null;
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
  scenarioName = null,
  templateName = null,
}: WorkbenchProps) {
  const shellClassName = preview
    ? "workbench-shell workbench-shell-preview"
    : "workbench-shell";

  return (
    <section className={shellClassName}>
      <TerminalPanel
        key={preview ? "preview-terminal" : session?.id ?? "live-terminal"}
        preview={preview}
        sessionKey={preview ? "preview-terminal" : session?.id ?? null}
        terminal={terminal}
      />
      <RepoPanel
        preview={preview}
        session={session}
        scenarioName={scenarioName}
        templateName={templateName}
        terminalStatus={terminal.status}
      />
      <CommandHistoryPanel preview={preview} terminal={terminal} />
    </section>
  );
}
