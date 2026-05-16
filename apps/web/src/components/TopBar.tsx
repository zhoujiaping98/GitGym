import { SessionStatusBadge } from "./SessionStatusBadge";

type TopBarProps = {
  metaLabel: string;
  sessionLabel: string;
  tone?: "idle" | "active" | "pending" | "error";
};

export function TopBar({
  metaLabel,
  sessionLabel,
  tone = "idle",
}: TopBarProps) {
  return (
    <header className="top-bar">
      <div className="brand-lockup">
        <span className="brand-mark" aria-hidden="true">
          GG
        </span>
        <div>
          <div className="brand-name">GitGym</div>
          <div className="brand-subtitle">Resettable practice workbench</div>
        </div>
      </div>
      <div className="top-bar-actions">
        <span className="top-bar-meta">{metaLabel}</span>
        <SessionStatusBadge label={sessionLabel} tone={tone} />
      </div>
    </header>
  );
}
