import { SessionStatusBadge } from "./SessionStatusBadge";
import type { CurrentUser } from "../types";

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
  currentUser?: CurrentUser | null;
};

function getCurrentUserLabels(currentUser: CurrentUser) {
  const loginLabel = `@${currentUser.githubLogin}`;
  const displayName = currentUser.displayName.trim();

  if (displayName) {
    return {
      primary: displayName,
      secondary: loginLabel,
    };
  }

  return {
    primary: loginLabel,
    secondary: null,
  };
}

export function TopBar({
  metaLabel,
  sessionLabel,
  tone = "idle",
  actions = [],
  currentUser = null,
}: TopBarProps) {
  const currentUserLabels = currentUser ? getCurrentUserLabels(currentUser) : null;

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
        {currentUser && currentUserLabels ? (
          <div className="top-bar-user">
            {currentUser.avatarUrl ? (
              <img
                alt={`${currentUserLabels.primary} avatar`}
                className="top-bar-user-avatar"
                src={currentUser.avatarUrl}
              />
            ) : null}
            <div className="top-bar-user-copy">
              <span className="top-bar-user-name">{currentUserLabels.primary}</span>
              {currentUserLabels.secondary ? (
                <span className="top-bar-user-login">{currentUserLabels.secondary}</span>
              ) : null}
            </div>
          </div>
        ) : null}
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
