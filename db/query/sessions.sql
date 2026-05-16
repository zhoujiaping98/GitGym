-- name: CreateUserSession :exec
INSERT INTO user_sessions (user_id, session_token_hash, user_agent, ip_address, expires_at)
VALUES (?, ?, ?, ?, ?);

-- name: GetUserSessionByTokenHash :one
SELECT * FROM user_sessions WHERE session_token_hash = ? AND revoked_at IS NULL LIMIT 1;

-- name: CreatePracticeSession :execresult
INSERT INTO practice_sessions (
  user_id, scenario_id, template_id, runner_ref, workspace_path_ref, status, started_at, expires_at, last_activity_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);
