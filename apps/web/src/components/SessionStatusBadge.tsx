type SessionStatusBadgeProps = {
  label: string;
  tone?: "idle" | "active";
};

export function SessionStatusBadge({
  label,
  tone = "idle",
}: SessionStatusBadgeProps) {
  return (
    <span className={`session-status-badge session-status-${tone}`}>{label}</span>
  );
}
