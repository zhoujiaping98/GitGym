package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"gitgym/services/api/internal/domain"
	"gitgym/services/api/internal/service"
	mysql "github.com/go-sql-driver/mysql"
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
	createPracticeSessionQuery = `
INSERT INTO practice_sessions (
  user_id,
  scenario_id,
  template_id,
  runner_ref,
  workspace_path_ref,
  status,
  started_at,
  expires_at,
  ended_at,
  last_activity_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`
	currentPracticeSessionQuery = `
SELECT
  id,
  user_id,
  scenario_id,
  template_id,
  runner_ref,
  workspace_path_ref,
  status,
  started_at,
  ended_at,
  expires_at,
  last_activity_at
FROM practice_sessions
WHERE user_id = ?
ORDER BY started_at DESC, id DESC
LIMIT 1
`
	practiceSessionByIDQuery = `
SELECT
  id,
  user_id,
  scenario_id,
  template_id,
  runner_ref,
  workspace_path_ref,
  status,
  started_at,
  ended_at,
  expires_at,
  last_activity_at
FROM practice_sessions
WHERE id = ?
LIMIT 1
`
	updatePracticeSessionQuery = `
UPDATE practice_sessions
SET status = ?, ended_at = ?, last_activity_at = ?
WHERE id = ?
`
	expirePracticeSessionsQuery = `
UPDATE practice_sessions
SET status = ?, ended_at = COALESCE(ended_at, ?), last_activity_at = ?
WHERE status = ? AND expires_at <= ?
`
	expiredPracticeSessionsLookupQuery = `
SELECT
  id,
  user_id,
  scenario_id,
  template_id,
  runner_ref,
  workspace_path_ref,
  status,
  started_at,
  ended_at,
  expires_at,
  last_activity_at
FROM practice_sessions
WHERE status = ? AND ended_at = ? AND last_activity_at = ?
`
	listPracticeTemplatesQuery = `
SELECT id, template_key, name
FROM workspace_templates
WHERE is_active = 1
ORDER BY id ASC
`
	listPracticeScenariosQuery = `
SELECT id, scenario_key, name, template_id
FROM scenarios
WHERE is_active = 1
ORDER BY id ASC
`
	upsertWorkspaceCleanupJobQuery = `
INSERT INTO workspace_cleanup_jobs (
  practice_session_id, workspace_id, reason, scheduled_at, status, attempt_count, last_error
) VALUES (?, ?, ?, ?, ?, 0, NULL)
ON DUPLICATE KEY UPDATE
  workspace_id = VALUES(workspace_id),
  reason = VALUES(reason),
  scheduled_at = VALUES(scheduled_at),
  status = VALUES(status),
  last_error = NULL,
  updated_at = CURRENT_TIMESTAMP(6)
`
	claimDueWorkspaceCleanupJobsQuery = `
SELECT
  id,
  practice_session_id,
  workspace_id,
  reason,
  scheduled_at,
  status,
  attempt_count,
  last_error,
  created_at,
  updated_at
FROM workspace_cleanup_jobs
WHERE (
  (status IN (?, ?) AND scheduled_at <= ? AND attempt_count < ?)
  OR (status = ? AND updated_at <= ? AND attempt_count < ?)
)
ORDER BY scheduled_at ASC, id ASC
LIMIT ?
FOR UPDATE
`
	listWorkspaceCleanupJobsForSessionQuery = `
SELECT
  id,
  practice_session_id,
  workspace_id,
  reason,
  scheduled_at,
  status,
  attempt_count,
  last_error,
  created_at,
  updated_at
FROM workspace_cleanup_jobs
WHERE practice_session_id = ?
ORDER BY scheduled_at ASC, id ASC
`
	listPracticeSessionsMissingWorkspaceCleanupJobQuery = `
SELECT
  ps.id,
  ps.user_id,
  ps.scenario_id,
  ps.template_id,
  ps.runner_ref,
  ps.workspace_path_ref,
  ps.status,
  ps.started_at,
  ps.ended_at,
  ps.expires_at,
  ps.last_activity_at
FROM practice_sessions ps
LEFT JOIN workspace_cleanup_jobs wcj
  ON wcj.practice_session_id = ps.id
WHERE ps.status IN (?, ?)
  AND wcj.id IS NULL
ORDER BY ps.id ASC
LIMIT ?
`
	listExhaustedWorkspaceCleanupJobsQuery = `
SELECT
  id,
  practice_session_id,
  workspace_id,
  reason,
  scheduled_at,
  status,
  attempt_count,
  last_error,
  created_at,
  updated_at
FROM workspace_cleanup_jobs
WHERE status = ?
  AND attempt_count >= ?
ORDER BY id ASC
LIMIT ?
`
	workspaceCleanupJobByIDQuery = `
SELECT
  id,
  practice_session_id,
  workspace_id,
  reason,
  scheduled_at,
  status,
  attempt_count,
  last_error,
  created_at,
  updated_at
FROM workspace_cleanup_jobs
WHERE id = ?
LIMIT 1
`
)

type MySQLStore struct {
	db *sql.DB
}

func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

func OpenMySQL(dsn string) (*sql.DB, error) {
	normalizedDSN, err := NormalizeMySQLDSN(dsn)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("mysql", normalizedDSN)
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
		return domain.BrowserSession{}, mapBrowserSessionLookupError(err)
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

func (s *MySQLStore) CreatePracticeSession(ctx context.Context, session domain.PracticeSession) (domain.PracticeSession, error) {
	result, err := s.db.ExecContext(
		ctx,
		createPracticeSessionQuery,
		session.UserID,
		session.ScenarioID,
		session.TemplateID,
		session.RunnerRef,
		session.WorkspacePathRef,
		session.Status,
		session.StartedAt,
		session.ExpiresAt,
		session.EndedAt,
		session.LastActivityAt,
	)
	if err != nil {
		return domain.PracticeSession{}, fmt.Errorf("create practice session: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return domain.PracticeSession{}, fmt.Errorf("read practice session id: %w", err)
	}

	session.ID = uint64(id)
	return session, nil
}

func (s *MySQLStore) CurrentPracticeSession(ctx context.Context, userID uint64) (domain.PracticeSession, error) {
	row := s.db.QueryRowContext(ctx, currentPracticeSessionQuery, userID)
	return scanPracticeSession(row)
}

func (s *MySQLStore) PracticeSessionByID(ctx context.Context, sessionID uint64) (domain.PracticeSession, error) {
	row := s.db.QueryRowContext(ctx, practiceSessionByIDQuery, sessionID)
	return scanPracticeSession(row)
}

func (s *MySQLStore) UpdatePracticeSession(ctx context.Context, session domain.PracticeSession) (domain.PracticeSession, error) {
	result, err := s.db.ExecContext(
		ctx,
		updatePracticeSessionQuery,
		session.Status,
		session.EndedAt,
		session.LastActivityAt,
		session.ID,
	)
	if err != nil {
		return domain.PracticeSession{}, fmt.Errorf("update practice session: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return domain.PracticeSession{}, fmt.Errorf("update practice session rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return domain.PracticeSession{}, service.ErrPracticeSessionNotFound
	}

	return session, nil
}

func (s *MySQLStore) ExpirePracticeSessions(ctx context.Context, before time.Time, endedAt time.Time) ([]domain.PracticeSession, error) {
	if _, err := s.db.ExecContext(
		ctx,
		expirePracticeSessionsQuery,
		service.PracticeSessionStatusExpired,
		endedAt,
		endedAt,
		service.PracticeSessionStatusActive,
		before,
	); err != nil {
		return nil, fmt.Errorf("expire practice sessions: %w", err)
	}

	rows, err := s.db.QueryContext(
		ctx,
		expiredPracticeSessionsLookupQuery,
		service.PracticeSessionStatusExpired,
		endedAt,
		endedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("query expired practice sessions: %w", err)
	}
	defer rows.Close()

	var sessions []domain.PracticeSession
	for rows.Next() {
		session, err := scanPracticeSessionRows(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate expired practice sessions: %w", err)
	}

	return sessions, nil
}

func (s *MySQLStore) ListPracticeTemplates(ctx context.Context) ([]service.PracticeTemplate, error) {
	rows, err := s.db.QueryContext(ctx, listPracticeTemplatesQuery)
	if err != nil {
		return nil, fmt.Errorf("list practice templates: %w", err)
	}
	defer rows.Close()

	var templates []service.PracticeTemplate
	for rows.Next() {
		var template service.PracticeTemplate
		if err := rows.Scan(&template.ID, &template.Key, &template.Name); err != nil {
			return nil, fmt.Errorf("scan practice template: %w", err)
		}
		templates = append(templates, template)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate practice templates: %w", err)
	}

	return templates, nil
}

func (s *MySQLStore) ListPracticeScenarios(ctx context.Context) ([]service.PracticeScenario, error) {
	rows, err := s.db.QueryContext(ctx, listPracticeScenariosQuery)
	if err != nil {
		return nil, fmt.Errorf("list practice scenarios: %w", err)
	}
	defer rows.Close()

	var scenarios []service.PracticeScenario
	for rows.Next() {
		var scenario service.PracticeScenario
		if err := rows.Scan(&scenario.ID, &scenario.Key, &scenario.Name, &scenario.TemplateID); err != nil {
			return nil, fmt.Errorf("scan practice scenario: %w", err)
		}
		scenarios = append(scenarios, scenario)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate practice scenarios: %w", err)
	}

	return scenarios, nil
}

func (s *MySQLStore) UpsertWorkspaceCleanupJob(ctx context.Context, job domain.WorkspaceCleanupJob) error {
	if _, err := s.db.ExecContext(
		ctx,
		upsertWorkspaceCleanupJobQuery,
		job.PracticeSessionID,
		job.WorkspaceID,
		job.Reason,
		job.ScheduledAt,
		job.Status,
	); err != nil {
		return fmt.Errorf("upsert workspace cleanup job: %w", err)
	}

	return nil
}

func (s *MySQLStore) ClaimDueWorkspaceCleanupJobs(ctx context.Context, now time.Time, limit int) ([]domain.WorkspaceCleanupJob, error) {
	if limit <= 0 {
		return []domain.WorkspaceCleanupJob{}, nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin claim workspace cleanup jobs tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	staleRunningBefore := now.UTC().Add(-service.WorkspaceCleanupJobLeaseTimeout)
	rows, err := tx.QueryContext(
		ctx,
		claimDueWorkspaceCleanupJobsQuery,
		"pending",
		"failed",
		now,
		service.WorkspaceCleanupJobMaxAttempts,
		"running",
		staleRunningBefore,
		service.WorkspaceCleanupJobMaxAttempts,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query due workspace cleanup jobs: %w", err)
	}

	var jobs []domain.WorkspaceCleanupJob
	for rows.Next() {
		job, err := scanWorkspaceCleanupJobRows(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterate due workspace cleanup jobs: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close due workspace cleanup jobs rows: %w", err)
	}

	if len(jobs) == 0 {
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("commit empty workspace cleanup jobs claim: %w", err)
		}
		return []domain.WorkspaceCleanupJob{}, nil
	}

	claimedAt := now.UTC()
	args := make([]any, 0, len(jobs)+3)
	args = append(args, "running")
	args = append(args, claimedAt)
	placeholders := make([]string, 0, len(jobs))
	for _, job := range jobs {
		placeholders = append(placeholders, "?")
		args = append(args, job.ID)
	}
	updateQuery := fmt.Sprintf(`
UPDATE workspace_cleanup_jobs
SET status = ?, attempt_count = attempt_count + 1, last_error = NULL, updated_at = ?
WHERE id IN (%s)
`, strings.Join(placeholders, ", "))
	if _, err := tx.ExecContext(ctx, updateQuery, args...); err != nil {
		return nil, fmt.Errorf("mark workspace cleanup jobs running: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit workspace cleanup jobs claim: %w", err)
	}

	for i := range jobs {
		jobs[i].Status = "running"
		jobs[i].AttemptCount++
		jobs[i].LastError = ""
		jobs[i].UpdatedAt = claimedAt
	}

	return jobs, nil
}

func (s *MySQLStore) MarkWorkspaceCleanupJobSucceeded(ctx context.Context, jobID uint64) error {
	if _, err := s.db.ExecContext(ctx, `
UPDATE workspace_cleanup_jobs
SET status = ?, last_error = NULL, updated_at = CURRENT_TIMESTAMP(6)
WHERE id = ?
`, "succeeded", jobID); err != nil {
		return fmt.Errorf("mark workspace cleanup job succeeded: %w", err)
	}

	return nil
}

func (s *MySQLStore) MarkWorkspaceCleanupJobFailed(ctx context.Context, jobID uint64, scheduledAt time.Time, lastErr string) error {
	if _, err := s.db.ExecContext(ctx, `
UPDATE workspace_cleanup_jobs
SET status = ?, scheduled_at = ?, last_error = ?, updated_at = CURRENT_TIMESTAMP(6)
WHERE id = ?
`, "failed", scheduledAt, nullableString(lastErr), jobID); err != nil {
		return fmt.Errorf("mark workspace cleanup job failed: %w", err)
	}

	return nil
}

func (s *MySQLStore) ListWorkspaceCleanupJobsForSession(ctx context.Context, sessionID uint64) ([]domain.WorkspaceCleanupJob, error) {
	rows, err := s.db.QueryContext(ctx, listWorkspaceCleanupJobsForSessionQuery, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list workspace cleanup jobs for session: %w", err)
	}
	defer rows.Close()

	var jobs []domain.WorkspaceCleanupJob
	for rows.Next() {
		job, err := scanWorkspaceCleanupJobRows(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate workspace cleanup jobs for session: %w", err)
	}

	return jobs, nil
}

func (s *MySQLStore) ListPracticeSessionsMissingWorkspaceCleanupJob(ctx context.Context, limit int) ([]domain.PracticeSession, error) {
	if limit <= 0 {
		return []domain.PracticeSession{}, nil
	}

	rows, err := s.db.QueryContext(
		ctx,
		listPracticeSessionsMissingWorkspaceCleanupJobQuery,
		service.PracticeSessionStatusExpired,
		service.PracticeSessionStatusOrphaned,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list practice sessions missing cleanup jobs: %w", err)
	}
	defer rows.Close()

	var sessions []domain.PracticeSession
	for rows.Next() {
		session, err := scanPracticeSessionRows(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate practice sessions missing cleanup jobs: %w", err)
	}

	return sessions, nil
}

func (s *MySQLStore) ListExhaustedWorkspaceCleanupJobs(ctx context.Context, limit int) ([]domain.WorkspaceCleanupJob, error) {
	if limit <= 0 {
		return []domain.WorkspaceCleanupJob{}, nil
	}

	rows, err := s.db.QueryContext(
		ctx,
		listExhaustedWorkspaceCleanupJobsQuery,
		"failed",
		service.WorkspaceCleanupJobMaxAttempts,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list exhausted workspace cleanup jobs: %w", err)
	}
	defer rows.Close()

	var jobs []domain.WorkspaceCleanupJob
	for rows.Next() {
		job, err := scanWorkspaceCleanupJobRows(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate exhausted workspace cleanup jobs: %w", err)
	}

	return jobs, nil
}

func (s *MySQLStore) WorkspaceCleanupJobByID(ctx context.Context, jobID uint64) (domain.WorkspaceCleanupJob, error) {
	rows, err := s.db.QueryContext(ctx, workspaceCleanupJobByIDQuery, jobID)
	if err != nil {
		return domain.WorkspaceCleanupJob{}, fmt.Errorf("get workspace cleanup job by id: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return domain.WorkspaceCleanupJob{}, fmt.Errorf("iterate workspace cleanup job by id: %w", err)
		}
		return domain.WorkspaceCleanupJob{}, service.ErrWorkspaceCleanupJobNotFound
	}

	job, err := scanWorkspaceCleanupJobRows(rows)
	if err != nil {
		return domain.WorkspaceCleanupJob{}, err
	}
	if err := rows.Err(); err != nil {
		return domain.WorkspaceCleanupJob{}, fmt.Errorf("iterate workspace cleanup job by id: %w", err)
	}

	return job, nil
}

func (s *MySQLStore) RequeueWorkspaceCleanupJob(ctx context.Context, jobID uint64, scheduledAt time.Time) error {
	result, err := s.db.ExecContext(ctx, `
UPDATE workspace_cleanup_jobs
SET status = ?, attempt_count = 0, scheduled_at = ?, last_error = NULL, updated_at = CURRENT_TIMESTAMP(6)
WHERE id = ?
`, "pending", scheduledAt.UTC(), jobID)
	if err != nil {
		return fmt.Errorf("requeue workspace cleanup job: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected for requeue workspace cleanup job: %w", err)
	}
	if rowsAffected == 0 {
		return service.ErrWorkspaceCleanupJobNotFound
	}

	return nil
}

func NormalizeMySQLDSN(dsn string) (string, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return "", fmt.Errorf("parse mysql dsn: %w", err)
	}

	cfg.ParseTime = true
	cfg.Loc = time.UTC
	return cfg.FormatDSN(), nil
}

func BrowserSessionLookupQuery() string {
	return browserSessionLookupQuery
}

func MapBrowserSessionLookupError(err error) error {
	return mapBrowserSessionLookupError(err)
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

func mapBrowserSessionLookupError(err error) error {
	if err == nil {
		return nil
	}
	if err == sql.ErrNoRows {
		return service.ErrBrowserSessionNotFound
	}
	return fmt.Errorf("get browser session by token hash: %w", err)
}

func scanPracticeSession(row *sql.Row) (domain.PracticeSession, error) {
	if row == nil {
		return domain.PracticeSession{}, fmt.Errorf("scan practice session: nil row")
	}

	session, err := scanPracticeSessionScanner(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return domain.PracticeSession{}, service.ErrPracticeSessionNotFound
		}
		return domain.PracticeSession{}, fmt.Errorf("scan practice session: %w", err)
	}

	return session, nil
}

type practiceSessionScanner interface {
	Scan(dest ...any) error
}

func scanPracticeSessionRows(rows *sql.Rows) (domain.PracticeSession, error) {
	session, err := scanPracticeSessionScanner(rows)
	if err != nil {
		return domain.PracticeSession{}, fmt.Errorf("scan practice session rows: %w", err)
	}
	return session, nil
}

func scanPracticeSessionScanner(scanner practiceSessionScanner) (domain.PracticeSession, error) {
	var (
		session domain.PracticeSession
		endedAt sql.NullTime
	)

	if err := scanner.Scan(
		&session.ID,
		&session.UserID,
		&session.ScenarioID,
		&session.TemplateID,
		&session.RunnerRef,
		&session.WorkspacePathRef,
		&session.Status,
		&session.StartedAt,
		&endedAt,
		&session.ExpiresAt,
		&session.LastActivityAt,
	); err != nil {
		return domain.PracticeSession{}, err
	}

	session.EndedAt = nullTimePtr(endedAt)
	return session, nil
}

func scanWorkspaceCleanupJobRows(rows *sql.Rows) (domain.WorkspaceCleanupJob, error) {
	var (
		job       domain.WorkspaceCleanupJob
		lastError sql.NullString
	)

	if err := rows.Scan(
		&job.ID,
		&job.PracticeSessionID,
		&job.WorkspaceID,
		&job.Reason,
		&job.ScheduledAt,
		&job.Status,
		&job.AttemptCount,
		&lastError,
		&job.CreatedAt,
		&job.UpdatedAt,
	); err != nil {
		return domain.WorkspaceCleanupJob{}, fmt.Errorf("scan workspace cleanup job rows: %w", err)
	}

	if lastError.Valid {
		job.LastError = lastError.String
	}

	return job, nil
}
