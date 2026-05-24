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

func TestWorkspaceCleanupJobStoreDoesNotReclaimRunningJobImmediately(t *testing.T) {
	t.Parallel()

	store := newTestMySQLStore(t)
	sessionID := seedPracticeSession(t, store, seedPracticeSessionParams{
		userID:     10,
		scenarioID: 1,
		templateID: 1,
		runnerRef:  "ws-cleanup-running",
		workspace:  "/tmp/ws-cleanup-running",
		status:     "expired",
	})

	now := time.Date(2026, 5, 24, 13, 30, 0, 0, time.UTC)
	if err := store.UpsertWorkspaceCleanupJob(context.Background(), domain.WorkspaceCleanupJob{
		PracticeSessionID: sessionID,
		WorkspaceID:       "ws-cleanup-running",
		Reason:            "expired",
		ScheduledAt:       now.Add(-time.Minute),
		Status:            "pending",
	}); err != nil {
		t.Fatalf("seed cleanup job: %v", err)
	}

	firstClaim, err := store.ClaimDueWorkspaceCleanupJobs(context.Background(), now, 10)
	if err != nil {
		t.Fatalf("first claim due cleanup jobs: %v", err)
	}
	if len(firstClaim) != 1 {
		t.Fatalf("expected one initially claimed job, got %d", len(firstClaim))
	}
	if firstClaim[0].Status != "running" {
		t.Fatalf("expected initially claimed job status running, got %q", firstClaim[0].Status)
	}

	secondClaim, err := store.ClaimDueWorkspaceCleanupJobs(context.Background(), now, 10)
	if err != nil {
		t.Fatalf("second claim due cleanup jobs: %v", err)
	}
	if len(secondClaim) != 0 {
		t.Fatalf("expected no reclaimed running jobs, got %d", len(secondClaim))
	}
}

func TestWorkspaceCleanupJobStoreMarksFailureAndReschedules(t *testing.T) {
	t.Parallel()

	store := newTestMySQLStore(t)
	sessionID := seedPracticeSession(t, store, seedPracticeSessionParams{
		userID:     11,
		scenarioID: 1,
		templateID: 1,
		runnerRef:  "ws-cleanup-failed",
		workspace:  "/tmp/ws-cleanup-failed",
		status:     "expired",
	})

	now := time.Date(2026, 5, 24, 14, 0, 0, 0, time.UTC)
	rescheduledAt := now.Add(15 * time.Minute)
	if err := store.UpsertWorkspaceCleanupJob(context.Background(), domain.WorkspaceCleanupJob{
		PracticeSessionID: sessionID,
		WorkspaceID:       "ws-cleanup-failed",
		Reason:            "expired",
		ScheduledAt:       now,
		Status:            "pending",
	}); err != nil {
		t.Fatalf("seed cleanup job: %v", err)
	}

	seededJobs, err := store.ListWorkspaceCleanupJobsForSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("list seeded cleanup jobs: %v", err)
	}
	if len(seededJobs) != 1 {
		t.Fatalf("expected one seeded cleanup job, got %d", len(seededJobs))
	}

	if err := store.MarkWorkspaceCleanupJobFailed(context.Background(), seededJobs[0].ID, rescheduledAt, "runner timeout"); err != nil {
		t.Fatalf("mark cleanup failure: %v", err)
	}

	reloaded, err := store.ListWorkspaceCleanupJobsForSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("reload cleanup jobs: %v", err)
	}
	if len(reloaded) != 1 {
		t.Fatalf("expected one cleanup job, got %d", len(reloaded))
	}
	if reloaded[0].Status != "failed" {
		t.Fatalf("expected failed cleanup job, got %q", reloaded[0].Status)
	}
	if !reloaded[0].ScheduledAt.Equal(rescheduledAt) {
		t.Fatalf("expected rescheduled_at %v, got %v", rescheduledAt, reloaded[0].ScheduledAt)
	}
	if reloaded[0].LastError != "runner timeout" {
		t.Fatalf("expected last_error to round-trip, got %q", reloaded[0].LastError)
	}
}

func TestWorkspaceCleanupJobStoreReclaimsFailedDueJobs(t *testing.T) {
	t.Parallel()

	store := newTestMySQLStore(t)
	sessionID := seedPracticeSession(t, store, seedPracticeSessionParams{
		userID:     13,
		scenarioID: 1,
		templateID: 1,
		runnerRef:  "ws-cleanup-reclaim-failed",
		workspace:  "/tmp/ws-cleanup-reclaim-failed",
		status:     "expired",
	})

	now := time.Date(2026, 5, 24, 15, 0, 0, 0, time.UTC)
	if err := store.UpsertWorkspaceCleanupJob(context.Background(), domain.WorkspaceCleanupJob{
		PracticeSessionID: sessionID,
		WorkspaceID:       "ws-cleanup-reclaim-failed",
		Reason:            "expired",
		ScheduledAt:       now.Add(-5 * time.Minute),
		Status:            "pending",
	}); err != nil {
		t.Fatalf("seed cleanup job: %v", err)
	}

	seededJobs, err := store.ListWorkspaceCleanupJobsForSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("list seeded cleanup jobs: %v", err)
	}
	if len(seededJobs) != 1 {
		t.Fatalf("expected one seeded cleanup job, got %d", len(seededJobs))
	}

	firstClaim, err := store.ClaimDueWorkspaceCleanupJobs(context.Background(), now, 10)
	if err != nil {
		t.Fatalf("first claim due cleanup jobs: %v", err)
	}
	if len(firstClaim) != 1 {
		t.Fatalf("expected one initially claimed cleanup job, got %d", len(firstClaim))
	}
	if firstClaim[0].ID != seededJobs[0].ID {
		t.Fatalf("expected initially claimed cleanup job id %d, got %d", seededJobs[0].ID, firstClaim[0].ID)
	}
	if firstClaim[0].Status != "running" {
		t.Fatalf("expected initially claimed cleanup job status running, got %q", firstClaim[0].Status)
	}
	if firstClaim[0].AttemptCount != 1 {
		t.Fatalf("expected initially claimed cleanup job attempt_count 1, got %d", firstClaim[0].AttemptCount)
	}

	rescheduledAt := now.Add(-time.Minute)
	if err := store.MarkWorkspaceCleanupJobFailed(context.Background(), seededJobs[0].ID, rescheduledAt, "transient runner timeout"); err != nil {
		t.Fatalf("mark cleanup failure: %v", err)
	}

	failedJobs, err := store.ListWorkspaceCleanupJobsForSession(context.Background(), sessionID)
	if err != nil {
		t.Fatalf("list failed cleanup jobs: %v", err)
	}
	if len(failedJobs) != 1 {
		t.Fatalf("expected one failed cleanup job, got %d", len(failedJobs))
	}
	if failedJobs[0].Status != "failed" {
		t.Fatalf("expected failed cleanup job status failed, got %q", failedJobs[0].Status)
	}
	if failedJobs[0].AttemptCount != 1 {
		t.Fatalf("expected failed cleanup job attempt_count 1, got %d", failedJobs[0].AttemptCount)
	}
	if !failedJobs[0].ScheduledAt.Equal(rescheduledAt) {
		t.Fatalf("expected failed cleanup job scheduled_at %v, got %v", rescheduledAt, failedJobs[0].ScheduledAt)
	}
	if failedJobs[0].LastError != "transient runner timeout" {
		t.Fatalf("expected failed cleanup job last_error to round-trip, got %q", failedJobs[0].LastError)
	}

	claimedJobs, err := store.ClaimDueWorkspaceCleanupJobs(context.Background(), now, 10)
	if err != nil {
		t.Fatalf("reclaim due cleanup jobs: %v", err)
	}
	if len(claimedJobs) != 1 {
		t.Fatalf("expected one reclaimed cleanup job, got %d", len(claimedJobs))
	}
	if claimedJobs[0].ID != seededJobs[0].ID {
		t.Fatalf("expected reclaimed cleanup job id %d, got %d", seededJobs[0].ID, claimedJobs[0].ID)
	}
	if claimedJobs[0].Status != "running" {
		t.Fatalf("expected reclaimed cleanup job status running, got %q", claimedJobs[0].Status)
	}
	if claimedJobs[0].AttemptCount != 2 {
		t.Fatalf("expected reclaimed cleanup job attempt_count 2, got %d", claimedJobs[0].AttemptCount)
	}
	if claimedJobs[0].LastError != "" {
		t.Fatalf("expected reclaimed cleanup job last_error to be cleared, got %q", claimedJobs[0].LastError)
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
