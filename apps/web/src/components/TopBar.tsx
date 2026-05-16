import { SessionStatusBadge } from "./SessionStatusBadge";

export function TopBar() {
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
        <span className="top-bar-meta">Template: Standard</span>
        <SessionStatusBadge label="Signed out" tone="idle" />
      </div>
    </header>
  );
}
