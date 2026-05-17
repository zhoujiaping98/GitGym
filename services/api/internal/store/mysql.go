package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"gitgym/services/api/internal/domain"
	"gitgym/services/api/internal/service"
	_ "github.com/go-sql-driver/mysql"
)

const (
	upsertGitHubUserQuery = `
INSERT INTO users (github_user_id, github_login, display_name, avatar_url, email, last_login_at)
VALUES (?, ?, ?, ?, ?, UTC_TIMESTAMP(6))
ON DUPLICATE KEY UPDATE
  github_login = VALUES(github_login),
  display_name = VALUES(display_name),
  avatar_url = VALUES(avatar_url),
  email = VALUES(email),
  last_login_at = UTC_TIMESTAMP(6)
`
	createBrowserSessionQuery = `
INSERT INTO user_sessions (user_id, session_token_hash, user_agent, ip_address, expires_at)
VALUES (?, ?, ?, ?, DATE_ADD(UTC_TIMESTAMP(6), INTERVAL 30 DAY))
`
	browserSessionLookupQuery = `
SELECT id, user_id, session_token_hash, user_agent, ip_address, expires_at, revoked_at
FROM user_sessions
WHERE session_token_hash = ? AND revoked_at IS NULL AND expires_at > UTC_TIMESTAMP(6)
LIMIT 1
`
	revokeBrowserSessionQuery = `
UPDATE user_sessions
SET revoked_at = UTC_TIMESTAMP(6)
WHERE session_token_hash = ? AND revoked_at IS NULL
`
)

type MySQLStore struct {
	db *sql.DB
}

func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

func OpenMySQL(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	return db, nil
}

func (s *MySQLStore) UpsertGitHubUser(ctx context.Context, profile service.GitHubProfile) (uint64, error) {
	if _, err := s.db.ExecContext(ctx, upsertGitHubUserQuery, profile.ID, profile.Login, profile.Name, nullableString(profile.AvatarURL), nullableString(profile.Email)); err != nil {
		return 0, fmt.Errorf("upsert github user: %w", err)
	}

	user, err := s.GetUserByGitHubID(ctx, profile.ID)
	if err != nil {
		return 0, err
	}
	return user.ID, nil
}

func (s *MySQLStore) GetUserByGitHubID(ctx context.Context, githubUserID uint64) (domain.CurrentUser, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, github_user_id, github_login, display_name, avatar_url, email
FROM users
WHERE github_user_id = ?
LIMIT 1
`, githubUserID)
	return scanCurrentUser(row)
}

func (s *MySQLStore) GetUserByID(ctx context.Context, userID uint64) (domain.CurrentUser, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, github_user_id, github_login, display_name, avatar_url, email
FROM users
WHERE id = ?
LIMIT 1
`, userID)
	return scanCurrentUser(row)
}

func (s *MySQLStore) CreateBrowserSession(ctx context.Context, userID uint64, tokenHash string, userAgent string, ip string) error {
	if _, err := s.db.ExecContext(ctx, createBrowserSessionQuery, userID, tokenHash, nullableString(userAgent), nullableString(ip)); err != nil {
		return fmt.Errorf("create browser session: %w", err)
	}
	return nil
}

func (s *MySQLStore) GetBrowserSessionByTokenHash(ctx context.Context, tokenHash string) (domain.BrowserSession, error) {
	row := s.db.QueryRowContext(ctx, browserSessionLookupQuery, tokenHash)

	var (
		session   domain.BrowserSession
		userAgent sql.NullString
		ipAddress sql.NullString
		revokedAt sql.NullTime
	)
	if err := row.Scan(
		&session.ID,
		&session.UserID,
		&session.SessionTokenHash,
		&userAgent,
		&ipAddress,
		&session.ExpiresAt,
		&revokedAt,
	); err != nil {
		return domain.BrowserSession{}, fmt.Errorf("get browser session by token hash: %w", err)
	}

	session.UserAgent = nullStringPtr(userAgent)
	session.IPAddress = nullStringPtr(ipAddress)
	session.RevokedAt = nullTimePtr(revokedAt)
	return session, nil
}

func (s *MySQLStore) RevokeBrowserSession(ctx context.Context, tokenHash string) error {
	if _, err := s.db.ExecContext(ctx, revokeBrowserSessionQuery, tokenHash); err != nil {
		return fmt.Errorf("revoke browser session: %w", err)
	}
	return nil
}

func BrowserSessionLookupQueryForTest() string {
	return browserSessionLookupQuery
}

func scanCurrentUser(row *sql.Row) (domain.CurrentUser, error) {
	var (
		user      domain.CurrentUser
		avatarURL sql.NullString
		email     sql.NullString
	)
	if err := row.Scan(
		&user.ID,
		&user.GitHubID,
		&user.GitHubLogin,
		&user.DisplayName,
		&avatarURL,
		&email,
	); err != nil {
		return domain.CurrentUser{}, fmt.Errorf("scan current user: %w", err)
	}

	user.AvatarURL = nullStringPtr(avatarURL)
	user.Email = nullStringPtr(email)
	return user, nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	str := value.String
	return &str
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	t := value.Time
	return &t
}
