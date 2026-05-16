-- name: UpsertGitHubUser :execresult
INSERT INTO users (github_user_id, github_login, display_name, avatar_url, email, last_login_at)
VALUES (?, ?, ?, ?, ?, NOW(6))
ON DUPLICATE KEY UPDATE
  github_login = VALUES(github_login),
  display_name = VALUES(display_name),
  avatar_url = VALUES(avatar_url),
  email = VALUES(email),
  last_login_at = NOW(6);

-- name: GetUserByGitHubID :one
SELECT * FROM users WHERE github_user_id = ? LIMIT 1;
