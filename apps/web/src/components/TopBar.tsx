import { SessionStatusBadge } from "./SessionStatusBadge";

type TopBarAction = {
  label: string;
  onClick: () => void;
  disabled?: boolean;
};

type TopBarProps = {
  metaLabel: string;
  sessionLabel: string;
  tone?: "idle" | "active" | "pending" | "error";
  actions?: TopBarAction[];
};

export function TopBar({
  metaLabel,
  sessionLabel,
  tone = "idle",
  actions = [],
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
        {actions.length > 0 ? (
          <div className="top-bar-controls">
            {actions.map((action) => (
              <button
                key={action.label}
                className="top-bar-button"
                disabled={action.disabled}
                onClick={action.onClick}
                type="button"
              >
                {action.label}
              </button>
            ))}
          </div>
        ) : null}
        <SessionStatusBadge label={sessionLabel} tone={tone} />
      </div>
    </header>
  );
}
