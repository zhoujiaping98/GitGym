package test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"gitgym/services/api/internal/domain"
	"gitgym/services/api/internal/service"
	"gitgym/services/api/internal/store"
	mysqlcfg "github.com/go-sql-driver/mysql"
)

func TestWorkspaceCleanupJobStoreUpsertsByPracticeSession(t *testing.T) {
	t.Parallel()

	store := newTestMySQLStore(t)
	sessionID := seedPracticeSession(t, store, seedPracticeSessionParams{
		userID:     7,
		scenarioID: 1,
		templateID: 1,
		runnerRef:  "ws-cleanup-upsert",
		workspace:  "/tmp/ws-cleanup-upsert",
		status:     "active",
	})

	now := time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC)
	later := now.Add(10 * time.Minute)

	if err := store.UpsertWorkspaceCleanupJob(context.Background(), domain.WorkspaceCleanupJob{
		PracticeSessionID: sessionID,
		WorkspaceID:       "ws-cleanup-upsert",
		Reason:            "expired",
		ScheduledAt:       now,
		Status:            "pending",
	}); err != nil {
		t.Fatalf("upsert cleanup job: %v", err)
	}

	if err := store.UpsertWorkspaceCleanupJob(context.Background(), domain.WorkspaceCleanupJob{
		PracticeSessionID: sessionID,
		WorkspaceID:       "ws-cleanup-upsert",
		Reason:            "orphaned",
		ScheduledAt:       later,
		Status:            "pending",
	}); err != nil {
		t.Fatalf("upsert cleanup job again: %v", err)
	}

	jobs, err := store.ListWorkspaceCleanupJobsForSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("list cleanup jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected one cleanup job, got %d", len(jobs))
	}
	if jobs[0].Reason != "orphaned" {
		t.Fatalf("expected updated reason orphaned, got %q", jobs[0].Reason)
	}
	if !jobs[0].ScheduledAt.Equal(later) {
		t.Fatalf("expected updated scheduled_at %v, got %v", later, jobs[0].ScheduledAt)
	}
}

func TestWorkspaceCleanupJobStoreClaimsDueJobsAndMarksSuccess(t *testing.T) {
	t.Parallel()

	store := newTestMySQLStore(t)
	sessionID := seedPracticeSession(t, store, seedPracticeSessionParams{
		userID:     9,
		scenarioID: 1,
		templateID: 1,
		runnerRef:  "ws-cleanup-claim",
		workspace:  "/tmp/ws-cleanup-claim",
		status:     "expired",
	})

	now := time.Date(2026, 5, 24, 13, 0, 0, 0, time.UTC)
	if err := store.UpsertWorkspaceCleanupJob(context.Background(), domain.WorkspaceCleanupJob{
		PracticeSessionID: sessionID,
		WorkspaceID:       "ws-cleanup-claim",
		Reason:            "expired",
		ScheduledAt:       now.Add(-time.Minute),
		Status:            "pending",
	}); err != nil {
		t.Fatalf("seed cleanup job: %v", err)
	}

	jobs, err := store.ClaimDueWorkspaceCleanupJobs(context.Background(), now, 10)
	if err != nil {
		t.Fatalf("claim due cleanup jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected one claimed job, got %d", len(jobs))
	}
	if jobs[0].Status != "running" {
		t.Fatalf("expected claimed job status running, got %q", jobs[0].Status)
	}

	if err := store.MarkWorkspaceCleanupJobSucceeded(context.Background(), jobs[0].ID); err != nil {
		t.Fatalf("mark cleanup success: %v", err)
	}

	reloaded, err := store.ListWorkspaceCleanupJobsForSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("reload cleanup jobs: %v", err)
	}
	if reloaded[0].Status != "succeeded" {
		t.Fatalf("expected succeeded cleanup job, got %q", reloaded[0].Status)
	}
	if reloaded[0].LastError != "" {
		t.Fatalf("expected last_error to be cleared, got %q", reloaded[0].LastError)
	}
}

type seedPracticeSessionParams struct {
	userID     uint64
	scenarioID uint64
	templateID uint64
	runnerRef  string
	workspace  string
	status     string
}

func newTestMySQLStore(t *testing.T) *store.MySQLStore {
	t.Helper()

	rootDSN := strings.TrimSpace(os.Getenv("MYSQL_DSN"))
	if rootDSN == "" {
		rootDSN = "root:password@tcp(127.0.0.1:3306)/gitgym"
	}

	cfg, err := mysqlcfg.ParseDSN(rootDSN)
	if err != nil {
		t.Fatalf("parse mysql dsn: %v", err)
	}
	adminCfg := *cfg
	adminCfg.DBName = ""
	adminCfg.ParseTime = true
	adminCfg.Loc = time.UTC
	adminCfg.MultiStatements = true

	adminDB, err := sql.Open("mysql", adminCfg.FormatDSN())
	if err != nil {
		t.Fatalf("open admin mysql db: %v", err)
	}
	t.Cleanup(func() {
		_ = adminDB.Close()
	})

	if err := adminDB.Ping(); err != nil {
		t.Skipf("mysql unavailable for store integration test: %v", err)
	}

	dbName := fmt.Sprintf("gitgym_cleanup_jobs_%d", time.Now().UnixNano())
	if _, err := adminDB.Exec("CREATE DATABASE `" + dbName + "`"); err != nil {
		t.Fatalf("create test database: %v", err)
	}
	t.Cleanup(func() {
		_, _ = adminDB.Exec("DROP DATABASE `" + dbName + "`")
	})

	testCfg := *cfg
	testCfg.DBName = dbName
	testCfg.ParseTime = true
	testCfg.Loc = time.UTC
	testCfg.MultiStatements = true

	db, err := sql.Open("mysql", testCfg.FormatDSN())
	if err != nil {
		t.Fatalf("open test mysql db: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	applyMySQLMigrations(t, db)
	return store.NewMySQLStore(db)
}

func seedPracticeSession(t *testing.T, store *store.MySQLStore, params seedPracticeSessionParams) uint64 {
	t.Helper()

	userID, err := store.UpsertGitHubUser(context.Background(), service.GitHubProfile{
		ID:    params.userID + 1000,
		Login: fmt.Sprintf("user-%d", params.userID),
		Name:  fmt.Sprintf("User %d", params.userID),
	})
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	now := time.Date(2026, 5, 24, 11, 0, 0, 0, time.UTC)
	session, err := store.CreatePracticeSession(context.Background(), domain.PracticeSession{
		UserID:           userID,
		ScenarioID:       params.scenarioID,
		TemplateID:       params.templateID,
		RunnerRef:        params.runnerRef,
		WorkspacePathRef: params.workspace,
		Status:           params.status,
		StartedAt:        now,
		ExpiresAt:        now.Add(time.Hour),
		LastActivityAt:   now,
	})
	if err != nil {
		t.Fatalf("seed practice session: %v", err)
	}

	return session.ID
}

func applyMySQLMigrations(t *testing.T, db *sql.DB) {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path")
	}

	matches, err := filepath.Glob(filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..", "db", "migrations", "*.sql"))
	if err != nil {
		t.Fatalf("list migration files: %v", err)
	}
	sort.Strings(matches)

	for _, migrationPath := range matches {
		contents, err := os.ReadFile(migrationPath)
		if err != nil {
			t.Fatalf("read migration %s: %v", filepath.Base(migrationPath), err)
		}
		query := strings.TrimSpace(string(contents))
		if query == "" {
			continue
		}
		if _, err := db.Exec(query); err != nil {
			t.Fatalf("apply migration %s: %v", filepath.Base(migrationPath), err)
		}
	}
}
