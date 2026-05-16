const API_BASE = "/api/v1";

export function buildTerminalWebSocketUrl(
  sessionId: number,
  locationLike: Pick<Location, "protocol" | "host"> = window.location,
): string {
  const protocol = locationLike.protocol === "https:" ? "wss:" : "ws:";
  return `${protocol}//${locationLike.host}${API_BASE}/practice-sessions/${sessionId}/terminal`;
}
