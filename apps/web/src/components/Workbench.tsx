import { CommandHistoryPanel } from "./CommandHistoryPanel";
import { RepoPanel } from "./RepoPanel";
import { TerminalPanel } from "./TerminalPanel";
import type {
  PracticeSession,
  RepoAttribution,
  RepoStateView,
  TerminalSessionState,
} from "../types";

type WorkbenchProps = {
  preview?: boolean;
  session?: PracticeSession | null;
  terminal?: TerminalSessionState;
  scenarioName?: string | null;
  templateName?: string | null;
  repoState?: RepoStateView;
  repoAttribution?: RepoAttribution | null;
  repoOutcome?: string | null;
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
  repoState = { status: "idle", snapshot: null, error: null },
  repoAttribution = null,
  repoOutcome = null,
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
        repoState={repoState}
        repoAttribution={repoAttribution}
        repoOutcome={repoOutcome}
      />
      <CommandHistoryPanel preview={preview} terminal={terminal} />
    </section>
  );
}
