package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"gitgym/services/api/internal/domain"
	"gitgym/services/api/internal/runner"
)

const (
	practiceSessionTTL                = 2 * time.Hour
	practiceSessionOrphanCleanupGrace = 10 * time.Minute
	workspaceCleanupWriteTimeout      = 5 * time.Second
	WorkspaceCleanupJobMaxAttempts    = 5
	WorkspaceCleanupJobLeaseTimeout   = 15 * time.Minute
)

const (
	PracticeSessionStatusActive   = "active"
	PracticeSessionStatusExpired  = "expired"
	PracticeSessionStatusOrphaned = "orphaned"
)

var (
	ErrInvalidPracticeSessionInput     = errors.New("invalid practice session input")
	ErrUnknownPracticeScenario         = errors.New("unknown practice scenario")
	ErrUnknownPracticeTemplate         = errors.New("unknown practice template")
	ErrPracticeServiceConfiguration    = errors.New("practice service configuration error")
	ErrRunnerWorkspaceCreation         = errors.New("runner workspace creation failed")
	ErrRunnerWorkspaceReset            = errors.New("runner workspace reset failed")
	ErrRunnerRepoStateUnavailable      = errors.New("runner repository state unavailable")
	ErrRunnerTerminalUnavailable       = errors.New("runner terminal unavailable")
	ErrPracticeSessionNotFound         = errors.New("practice session not found")
	ErrPracticeSessionExpired          = errors.New("practice session expired")
	ErrPracticeSessionOrphaned         = errors.New("practice session workspace is unavailable")
	ErrWorkspaceCleanupJobNotFound     = errors.New("workspace cleanup job not found")
	ErrWorkspaceCleanupJobNotExhausted = errors.New("workspace cleanup job is not exhausted")
)

type PracticeTemplate struct {
	ID   uint64 `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

type PracticeScenario struct {
	ID         uint64 `json:"id"`
	Key        string `json:"key"`
	Name       string `json:"name"`
	TemplateID uint64 `json:"template_id"`
}

type CreatePracticeSessionInput struct {
	UserID     uint64
	ScenarioID uint64
	TemplateID uint64
}

type WorkspaceCleanupReconciliationSummary struct {
	BackfilledJobs      int
	ExhaustedFailedJobs int
}

type PracticeSessionStore interface {
	CreatePracticeSession(ctx context.Context, session domain.PracticeSession) (domain.PracticeSession, error)
	CurrentPracticeSession(ctx context.Context, userID uint64) (domain.PracticeSession, error)
	PracticeSessionByID(ctx context.Context, sessionID uint64) (domain.PracticeSession, error)
	UpdatePracticeSession(ctx context.Context, session domain.PracticeSession) (domain.PracticeSession, error)
	ExpirePracticeSessions(ctx context.Context, before time.Time, endedAt time.Time) ([]domain.PracticeSession, error)
	UpsertWorkspaceCleanupJob(ctx context.Context, job domain.WorkspaceCleanupJob) error
	ClaimDueWorkspaceCleanupJobs(ctx context.Context, now time.Time, limit int) ([]domain.WorkspaceCleanupJob, error)
	MarkWorkspaceCleanupJobSucceeded(ctx context.Context, jobID uint64) error
	MarkWorkspaceCleanupJobFailed(ctx context.Context, jobID uint64, scheduledAt time.Time, lastErr string) error
	WorkspaceCleanupJobByID(ctx context.Context, jobID uint64) (domain.WorkspaceCleanupJob, error)
	RequeueWorkspaceCleanupJob(ctx context.Context, jobID uint64, scheduledAt time.Time) error
	ListPracticeSessionsMissingWorkspaceCleanupJob(ctx context.Context, limit int) ([]domain.PracticeSession, error)
	ListExhaustedWorkspaceCleanupJobs(ctx context.Context, limit int) ([]domain.WorkspaceCleanupJob, error)
}

type PracticeService interface {
	ListTemplates(ctx context.Context) []PracticeTemplate
	ListScenarios(ctx context.Context) []PracticeScenario
	ListTemplatesWithError(ctx context.Context) ([]PracticeTemplate, error)
	ListScenariosWithError(ctx context.Context) ([]PracticeScenario, error)
	CreatePracticeSession(ctx context.Context, input CreatePracticeSessionInput) (domain.PracticeSession, error)
	ResetPracticeSession(ctx context.Context, userID uint64, sessionID uint64) error
	CurrentPracticeSession(ctx context.Context, userID uint64) (domain.PracticeSession, error)
	PracticeSessionByID(ctx context.Context, userID uint64, sessionID uint64) (domain.PracticeSession, error)
	PracticeSessionRepoState(ctx context.Context, userID uint64, sessionID uint64) (runner.RepoState, error)
	ConnectTerminal(ctx context.Context, userID uint64, sessionID uint64) (runner.TerminalConnection, error)
	ExpireStalePracticeSessions(ctx context.Context) (int, error)
	RunWorkspaceCleanupDueJobs(ctx context.Context, limit int) error
	ReconcileWorkspaceCleanupJobs(ctx context.Context, limit int) (WorkspaceCleanupReconciliationSummary, error)
	ListExhaustedWorkspaceCleanupJobs(ctx context.Context, limit int) ([]domain.WorkspaceCleanupJob, error)
	RequeueExhaustedWorkspaceCleanupJob(ctx context.Context, jobID uint64) error
}

type practiceService struct {
	store   PracticeSessionStore
	runner  runner.Client
	catalog PracticeCatalog
	now     func() time.Time
}

func NewPracticeService(store PracticeSessionStore, runnerClient runner.Client, options ...any) PracticeService {
	var (
		catalog PracticeCatalog
		now     func() time.Time
	)

	for _, option := range options {
		switch value := option.(type) {
		case PracticeCatalog:
			catalog = value
		case func() time.Time:
			now = value
		}
	}

	if now == nil {
		now = time.Now
	}
	if catalog == nil {
		catalog = NewFallbackPracticeCatalog()
	}

	return &practiceService{
		store:   store,
		runner:  runnerClient,
		catalog: catalog,
		now:     now,
	}
}

func (s *practiceService) ListTemplates(ctx context.Context) []PracticeTemplate {
	templates, err := s.ListTemplatesWithError(ctx)
	if err != nil {
		return nil
	}

	return templates
}

func (s *practiceService) ListScenarios(ctx context.Context) []PracticeScenario {
	scenarios, err := s.ListScenariosWithError(ctx)
	if err != nil {
		return nil
	}

	return scenarios
}

func (s *practiceService) ListTemplatesWithError(ctx context.Context) ([]PracticeTemplate, error) {
	if s.catalog == nil {
		return nil, fmt.Errorf("%w: practice catalog is not configured", ErrPracticeServiceConfiguration)
	}

	return s.catalog.ListTemplates(ctx)
}

func (s *practiceService) ListScenariosWithError(ctx context.Context) ([]PracticeScenario, error) {
	if s.catalog == nil {
		return nil, fmt.Errorf("%w: practice catalog is not configured", ErrPracticeServiceConfiguration)
	}

	return s.catalog.ListScenarios(ctx)
}

func (s *practiceService) CreatePracticeSession(ctx context.Context, input CreatePracticeSessionInput) (domain.PracticeSession, error) {
	if input.UserID == 0 || input.ScenarioID == 0 {
		return domain.PracticeSession{}, fmt.Errorf("%w", ErrInvalidPracticeSessionInput)
	}
	if s.catalog == nil {
		return domain.PracticeSession{}, fmt.Errorf("%w: practice catalog is not configured", ErrPracticeServiceConfiguration)
	}
	if s.runner == nil {
		return domain.PracticeSession{}, fmt.Errorf("%w: runner client is not configured", ErrPracticeServiceConfiguration)
	}
	if s.store == nil {
		return domain.PracticeSession{}, fmt.Errorf("%w: practice session store is not configured", ErrPracticeServiceConfiguration)
	}

	scenario, err := s.catalog.ScenarioByID(ctx, input.ScenarioID)
	if err != nil {
		return domain.PracticeSession{}, err
	}

	template, err := s.catalog.TemplateByID(ctx, scenario.TemplateID)
	if err != nil {
		return domain.PracticeSession{}, fmt.Errorf(
			"%w: scenario %d references template %d",
			ErrPracticeServiceConfiguration,
			scenario.ID,
			scenario.TemplateID,
		)
	}
	if input.TemplateID != 0 && input.TemplateID != template.ID {
		return domain.PracticeSession{}, fmt.Errorf(
			"%w: scenario %d resolves to template %d, got %d",
			ErrUnknownPracticeTemplate,
			scenario.ID,
			template.ID,
			input.TemplateID,
		)
	}

	workspace, err := s.runner.CreateWorkspace(ctx, template.Key)
	if err != nil {
		if errors.Is(err, runner.ErrClientNotConfigured) {
			return domain.PracticeSession{}, fmt.Errorf("%w: %v", ErrPracticeServiceConfiguration, err)
		}
		return domain.PracticeSession{}, fmt.Errorf("%w: %v", ErrRunnerWorkspaceCreation, err)
	}

	now := s.now().UTC()
	session := domain.PracticeSession{
		UserID:           input.UserID,
		ScenarioID:       scenario.ID,
		TemplateID:       template.ID,
		RunnerRef:        workspace.ID,
		WorkspacePathRef: workspace.Path,
		Status:           PracticeSessionStatusActive,
		StartedAt:        now,
		ExpiresAt:        now.Add(practiceSessionTTL),
		LastActivityAt:   now,
	}

	created, err := s.store.CreatePracticeSession(ctx, session)
	if err != nil {
		return domain.PracticeSession{}, fmt.Errorf("create practice session: %w", err)
	}

	return created, nil
}

func (s *practiceService) CurrentPracticeSession(ctx context.Context, userID uint64) (domain.PracticeSession, error) {
	if userID == 0 {
		return domain.PracticeSession{}, fmt.Errorf("%w", ErrInvalidPracticeSessionInput)
	}
	if s.store == nil {
		return domain.PracticeSession{}, fmt.Errorf("%w: practice session store is not configured", ErrPracticeServiceConfiguration)
	}

	session, err := s.store.CurrentPracticeSession(ctx, userID)
	if err != nil {
		return domain.PracticeSession{}, err
	}
	if session.UserID != userID {
		return domain.PracticeSession{}, fmt.Errorf("%w", ErrPracticeSessionNotFound)
	}
	session, err = s.ensureSessionAvailable(ctx, session)
	if err != nil {
		if errors.Is(err, ErrPracticeSessionExpired) {
			return domain.PracticeSession{}, fmt.Errorf("%w", ErrPracticeSessionNotFound)
		}
		return domain.PracticeSession{}, err
	}
	return session, nil
}

func (s *practiceService) ResetPracticeSession(ctx context.Context, userID uint64, sessionID uint64) error {
	if userID == 0 || sessionID == 0 {
		return fmt.Errorf("%w", ErrInvalidPracticeSessionInput)
	}
	if s.runner == nil {
		return fmt.Errorf("%w: runner client is not configured", ErrPracticeServiceConfiguration)
	}

	session, err := s.PracticeSessionByID(ctx, userID, sessionID)
	if err != nil {
		return err
	}

	if err := s.runner.ResetWorkspace(ctx, session.RunnerRef); err != nil {
		if errors.Is(err, runner.ErrWorkspaceNotFound) {
			return s.orphanSession(ctx, session)
		}
		if errors.Is(err, runner.ErrClientNotConfigured) {
			return fmt.Errorf("%w: %v", ErrPracticeServiceConfiguration, err)
		}
		return fmt.Errorf("%w: %v", ErrRunnerWorkspaceReset, err)
	}

	return nil
}

func (s *practiceService) PracticeSessionByID(ctx context.Context, userID uint64, sessionID uint64) (domain.PracticeSession, error) {
	if userID == 0 || sessionID == 0 {
		return domain.PracticeSession{}, fmt.Errorf("%w", ErrInvalidPracticeSessionInput)
	}
	if s.store == nil {
		return domain.PracticeSession{}, fmt.Errorf("%w: practice session store is not configured", ErrPracticeServiceConfiguration)
	}

	session, err := s.store.PracticeSessionByID(ctx, sessionID)
	if err != nil {
		return domain.PracticeSession{}, err
	}
	if session.UserID != userID {
		return domain.PracticeSession{}, fmt.Errorf("%w", ErrPracticeSessionNotFound)
	}
	return s.ensureSessionAvailable(ctx, session)
}

func (s *practiceService) PracticeSessionRepoState(ctx context.Context, userID uint64, sessionID uint64) (runner.RepoState, error) {
	if s.runner == nil {
		return runner.RepoState{}, fmt.Errorf("%w: runner client is not configured", ErrPracticeServiceConfiguration)
	}

	session, err := s.PracticeSessionByID(ctx, userID, sessionID)
	if err != nil {
		return runner.RepoState{}, err
	}

	repoState, err := s.runner.GetRepoState(ctx, session.RunnerRef)
	if err != nil {
		if errors.Is(err, runner.ErrWorkspaceNotFound) {
			return runner.RepoState{}, s.orphanSession(ctx, session)
		}
		if errors.Is(err, runner.ErrClientNotConfigured) {
			return runner.RepoState{}, fmt.Errorf("%w: %v", ErrPracticeServiceConfiguration, err)
		}
		return runner.RepoState{}, fmt.Errorf("%w: %v", ErrRunnerRepoStateUnavailable, err)
	}

	return repoState, nil
}

func (s *practiceService) ConnectTerminal(ctx context.Context, userID uint64, sessionID uint64) (runner.TerminalConnection, error) {
	if s.runner == nil {
		return nil, fmt.Errorf("%w: runner client is not configured", ErrPracticeServiceConfiguration)
	}

	session, err := s.PracticeSessionByID(ctx, userID, sessionID)
	if err != nil {
		return nil, err
	}

	conn, err := s.runner.ConnectTerminal(ctx, session.RunnerRef)
	if err != nil {
		if errors.Is(err, runner.ErrWorkspaceNotFound) {
			return nil, s.orphanSession(ctx, session)
		}
		if errors.Is(err, runner.ErrClientNotConfigured) {
			return nil, fmt.Errorf("%w: %v", ErrPracticeServiceConfiguration, err)
		}
		return nil, fmt.Errorf("%w: %v", ErrRunnerTerminalUnavailable, err)
	}

	return conn, nil
}

func (s *practiceService) ExpireStalePracticeSessions(ctx context.Context) (int, error) {
	if s.store == nil {
		return 0, fmt.Errorf("%w: practice session store is not configured", ErrPracticeServiceConfiguration)
	}

	now := s.now().UTC()
	expiredSessions, err := s.store.ExpirePracticeSessions(ctx, now, now)
	if err != nil {
		return 0, fmt.Errorf("expire stale practice sessions: %w", err)
	}

	for _, session := range expiredSessions {
		s.upsertWorkspaceCleanupJob(ctx, session, PracticeSessionStatusExpired, now)
	}

	return len(expiredSessions), nil
}

func (s *practiceService) RunWorkspaceCleanupDueJobs(ctx context.Context, limit int) error {
	if s.store == nil || s.runner == nil {
		return nil
	}
	if limit <= 0 {
		limit = 10
	}

	now := s.now().UTC()
	jobs, err := s.store.ClaimDueWorkspaceCleanupJobs(ctx, now, limit)
	if err != nil {
		return fmt.Errorf("claim due cleanup jobs: %w", err)
	}

	var runErrs []error
	for _, job := range jobs {
		err := s.runner.DeleteWorkspace(ctx, job.WorkspaceID, job.Reason, 0)
		if err == nil || errors.Is(err, runner.ErrWorkspaceNotFound) {
			if markErr := s.markWorkspaceCleanupJobSucceeded(ctx, job, now); markErr != nil {
				runErrs = append(runErrs, markErr)
			}
			continue
		}

		if workspaceCleanupAttemptsExhausted(job.AttemptCount) {
			runErrs = append(runErrs, fmt.Errorf("cleanup job %d exhausted retries after attempt %d: %w", job.ID, job.AttemptCount, err))
			if markErr := s.markWorkspaceCleanupJobFailed(ctx, job, now, err.Error()); markErr != nil {
				runErrs = append(runErrs, markErr)
			}
			continue
		}

		runErrs = append(runErrs, fmt.Errorf("delete workspace for cleanup job %d: %w", job.ID, err))
		nextRun := nextWorkspaceCleanupRetryAt(now, job.AttemptCount)
		if markErr := s.markWorkspaceCleanupJobFailed(ctx, job, nextRun, err.Error()); markErr != nil {
			runErrs = append(runErrs, markErr)
		}
	}

	return errors.Join(runErrs...)
}

func (s *practiceService) ReconcileWorkspaceCleanupJobs(ctx context.Context, limit int) (WorkspaceCleanupReconciliationSummary, error) {
	if s.store == nil {
		return WorkspaceCleanupReconciliationSummary{}, nil
	}
	if limit <= 0 {
		limit = 10
	}

	now := s.now().UTC()
	sessions, err := s.store.ListPracticeSessionsMissingWorkspaceCleanupJob(ctx, limit)
	if err != nil {
		return WorkspaceCleanupReconciliationSummary{}, fmt.Errorf("list practice sessions missing cleanup jobs: %w", err)
	}

	summary := WorkspaceCleanupReconciliationSummary{}
	for _, session := range sessions {
		if session.Status != PracticeSessionStatusExpired && session.Status != PracticeSessionStatusOrphaned {
			continue
		}

		scheduledAt := now
		if session.Status == PracticeSessionStatusOrphaned && session.EndedAt != nil {
			graceSchedule := session.EndedAt.UTC().Add(practiceSessionOrphanCleanupGrace)
			if graceSchedule.After(scheduledAt) {
				scheduledAt = graceSchedule
			}
		}

		if err := s.store.UpsertWorkspaceCleanupJob(ctx, domain.WorkspaceCleanupJob{
			PracticeSessionID: session.ID,
			WorkspaceID:       session.RunnerRef,
			Reason:            session.Status,
			ScheduledAt:       scheduledAt,
			Status:            "pending",
		}); err != nil {
			return WorkspaceCleanupReconciliationSummary{}, fmt.Errorf("backfill cleanup job for session %d: %w", session.ID, err)
		}
		summary.BackfilledJobs++
	}

	exhaustedJobs, err := s.store.ListExhaustedWorkspaceCleanupJobs(ctx, limit)
	if err != nil {
		return WorkspaceCleanupReconciliationSummary{}, fmt.Errorf("list exhausted cleanup jobs: %w", err)
	}
	summary.ExhaustedFailedJobs = len(exhaustedJobs)

	return summary, nil
}

func (s *practiceService) ListExhaustedWorkspaceCleanupJobs(ctx context.Context, limit int) ([]domain.WorkspaceCleanupJob, error) {
	if s.store == nil {
		return []domain.WorkspaceCleanupJob{}, nil
	}
	if limit <= 0 {
		limit = 20
	}

	jobs, err := s.store.ListExhaustedWorkspaceCleanupJobs(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("list exhausted cleanup jobs: %w", err)
	}

	return jobs, nil
}

func (s *practiceService) RequeueExhaustedWorkspaceCleanupJob(ctx context.Context, jobID uint64) error {
	if s.store == nil {
		return ErrWorkspaceCleanupJobNotFound
	}
	if jobID == 0 {
		return ErrWorkspaceCleanupJobNotFound
	}

	job, err := s.store.WorkspaceCleanupJobByID(ctx, jobID)
	if err != nil {
		return err
	}
	if job.Status != "failed" || !workspaceCleanupAttemptsExhausted(job.AttemptCount) {
		return ErrWorkspaceCleanupJobNotExhausted
	}

	if err := s.store.RequeueWorkspaceCleanupJob(ctx, jobID, s.now().UTC()); err != nil {
		return err
	}

	return nil
}

func (s *practiceService) markWorkspaceCleanupJobSucceeded(
	ctx context.Context,
	job domain.WorkspaceCleanupJob,
	recoveryAt time.Time,
) error {
	writeCtx, cancel := workspaceCleanupWriteContext(ctx)
	defer cancel()

	if err := s.store.MarkWorkspaceCleanupJobSucceeded(writeCtx, job.ID); err != nil {
		return s.recoverClaimedWorkspaceCleanupJob(ctx, job, recoveryAt, fmt.Errorf("mark cleanup job %d succeeded: %w", job.ID, err))
	}

	return nil
}

func (s *practiceService) markWorkspaceCleanupJobFailed(
	ctx context.Context,
	job domain.WorkspaceCleanupJob,
	scheduledAt time.Time,
	lastErr string,
) error {
	writeCtx, cancel := workspaceCleanupWriteContext(ctx)
	defer cancel()

	if err := s.store.MarkWorkspaceCleanupJobFailed(writeCtx, job.ID, scheduledAt, lastErr); err != nil {
		return s.recoverClaimedWorkspaceCleanupJob(ctx, job, scheduledAt, fmt.Errorf("mark cleanup job %d failed: %w", job.ID, err))
	}

	return nil
}

func (s *practiceService) recoverClaimedWorkspaceCleanupJob(
	ctx context.Context,
	job domain.WorkspaceCleanupJob,
	scheduledAt time.Time,
	cause error,
) error {
	writeCtx, cancel := workspaceCleanupWriteContext(ctx)
	defer cancel()

	recoveryJob := domain.WorkspaceCleanupJob{
		PracticeSessionID: job.PracticeSessionID,
		WorkspaceID:       job.WorkspaceID,
		Reason:            job.Reason,
		ScheduledAt:       scheduledAt.UTC(),
		Status:            "pending",
	}

	if err := s.store.UpsertWorkspaceCleanupJob(writeCtx, recoveryJob); err != nil {
		return errors.Join(
			cause,
			fmt.Errorf("recover cleanup job %d with re-enqueue: %w", job.ID, err),
		)
	}

	return cause
}

func workspaceCleanupWriteContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		return context.WithTimeout(context.Background(), workspaceCleanupWriteTimeout)
	}

	return context.WithTimeout(context.WithoutCancel(ctx), workspaceCleanupWriteTimeout)
}

func (s *practiceService) ensureSessionAvailable(ctx context.Context, session domain.PracticeSession) (domain.PracticeSession, error) {
	switch session.Status {
	case PracticeSessionStatusExpired:
		return domain.PracticeSession{}, fmt.Errorf("%w", ErrPracticeSessionExpired)
	case PracticeSessionStatusOrphaned:
		return domain.PracticeSession{}, fmt.Errorf("%w", ErrPracticeSessionOrphaned)
	}

	if !session.ExpiresAt.After(s.now().UTC()) {
		updated, err := s.transitionSession(ctx, session, PracticeSessionStatusExpired)
		if err != nil {
			return domain.PracticeSession{}, err
		}
		s.upsertWorkspaceCleanupJob(ctx, updated, PracticeSessionStatusExpired, s.now().UTC())
		return domain.PracticeSession{}, fmt.Errorf("%w: %d", ErrPracticeSessionExpired, updated.ID)
	}

	return session, nil
}

func (s *practiceService) transitionSession(ctx context.Context, session domain.PracticeSession, status string) (domain.PracticeSession, error) {
	if s.store == nil {
		return domain.PracticeSession{}, fmt.Errorf("%w: practice session store is not configured", ErrPracticeServiceConfiguration)
	}

	now := s.now().UTC()
	session.Status = status
	session.LastActivityAt = now
	if session.EndedAt == nil {
		session.EndedAt = &now
	}

	updated, err := s.store.UpdatePracticeSession(ctx, session)
	if err != nil {
		return domain.PracticeSession{}, fmt.Errorf("update practice session lifecycle: %w", err)
	}

	return updated, nil
}

func (s *practiceService) orphanSession(ctx context.Context, session domain.PracticeSession) error {
	updated, err := s.transitionSession(ctx, session, PracticeSessionStatusOrphaned)
	if err != nil {
		return err
	}
	s.upsertWorkspaceCleanupJob(ctx, updated, PracticeSessionStatusOrphaned, s.now().UTC().Add(practiceSessionOrphanCleanupGrace))
	return fmt.Errorf("%w", ErrPracticeSessionOrphaned)
}

func (s *practiceService) upsertWorkspaceCleanupJob(
	ctx context.Context,
	session domain.PracticeSession,
	reason string,
	scheduledAt time.Time,
) {
	if s.store == nil {
		return
	}

	job := domain.WorkspaceCleanupJob{
		PracticeSessionID: session.ID,
		WorkspaceID:       session.RunnerRef,
		Reason:            reason,
		ScheduledAt:       scheduledAt.UTC(),
		Status:            "pending",
	}
	if err := s.store.UpsertWorkspaceCleanupJob(ctx, job); err != nil {
		log.Printf("practice cleanup job upsert failed for session %d: %v", session.ID, err)
		s.fallbackWorkspaceCleanup(ctx, session, reason, scheduledAt)
	}
}

func (s *practiceService) fallbackWorkspaceCleanup(
	ctx context.Context,
	session domain.PracticeSession,
	reason string,
	scheduledAt time.Time,
) {
	if s.runner == nil || session.RunnerRef == "" {
		return
	}

	deleteAfter := scheduledAt.UTC().Sub(s.now().UTC())
	if deleteAfter < 0 {
		deleteAfter = 0
	}

	writeCtx, cancel := workspaceCleanupWriteContext(ctx)
	defer cancel()

	if err := s.runner.DeleteWorkspace(writeCtx, session.RunnerRef, reason, deleteAfter); err != nil && !errors.Is(err, runner.ErrWorkspaceNotFound) {
		log.Printf("practice cleanup fallback delete failed for session %d: %v", session.ID, err)
	}
}

func nextWorkspaceCleanupRetryAt(now time.Time, attempt uint32) time.Time {
	switch attempt {
	case 1:
		return now.Add(time.Minute)
	case 2:
		return now.Add(5 * time.Minute)
	default:
		return now.Add(15 * time.Minute)
	}
}

func workspaceCleanupAttemptsExhausted(attempt uint32) bool {
	return attempt >= WorkspaceCleanupJobMaxAttempts
}

type InMemoryPracticeSessionStore struct {
	mu                  sync.Mutex
	nextID              uint64
	nextCleanupJobID    uint64
	sessions            map[uint64]domain.PracticeSession
	currentByUser       map[uint64]uint64
	cleanupJobs         map[uint64]domain.WorkspaceCleanupJob
	cleanupJobBySession map[uint64]uint64
}

func NewInMemoryPracticeSessionStore() *InMemoryPracticeSessionStore {
	return &InMemoryPracticeSessionStore{
		nextID:              1,
		nextCleanupJobID:    1,
		sessions:            make(map[uint64]domain.PracticeSession),
		currentByUser:       make(map[uint64]uint64),
		cleanupJobs:         make(map[uint64]domain.WorkspaceCleanupJob),
		cleanupJobBySession: make(map[uint64]uint64),
	}
}

func (s *InMemoryPracticeSessionStore) CreatePracticeSession(_ context.Context, session domain.PracticeSession) (domain.PracticeSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session.ID = s.nextID
	s.nextID++
	s.sessions[session.ID] = session
	s.currentByUser[session.UserID] = session.ID
	return session, nil
}

func (s *InMemoryPracticeSessionStore) CurrentPracticeSession(_ context.Context, userID uint64) (domain.PracticeSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sessionID, ok := s.currentByUser[userID]
	if !ok {
		return domain.PracticeSession{}, fmt.Errorf("%w", ErrPracticeSessionNotFound)
	}

	session, ok := s.sessions[sessionID]
	if !ok {
		return domain.PracticeSession{}, fmt.Errorf("%w", ErrPracticeSessionNotFound)
	}
	return session, nil
}

func (s *InMemoryPracticeSessionStore) PracticeSessionByID(_ context.Context, sessionID uint64) (domain.PracticeSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return domain.PracticeSession{}, fmt.Errorf("%w", ErrPracticeSessionNotFound)
	}
	return session, nil
}

func (s *InMemoryPracticeSessionStore) UpdatePracticeSession(_ context.Context, session domain.PracticeSession) (domain.PracticeSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.sessions[session.ID]; !ok {
		return domain.PracticeSession{}, fmt.Errorf("%w", ErrPracticeSessionNotFound)
	}

	s.sessions[session.ID] = session
	if currentID, ok := s.currentByUser[session.UserID]; !ok || currentID == session.ID || session.Status == PracticeSessionStatusActive {
		s.currentByUser[session.UserID] = session.ID
	}

	return session, nil
}

func (s *InMemoryPracticeSessionStore) ExpirePracticeSessions(_ context.Context, before time.Time, endedAt time.Time) ([]domain.PracticeSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var expired []domain.PracticeSession
	for id, session := range s.sessions {
		if session.Status != PracticeSessionStatusActive || session.ExpiresAt.After(before) {
			continue
		}

		session.Status = PracticeSessionStatusExpired
		session.LastActivityAt = endedAt
		if session.EndedAt == nil {
			session.EndedAt = &endedAt
		}
		s.sessions[id] = session
		expired = append(expired, session)
	}

	return expired, nil
}

func (s *InMemoryPracticeSessionStore) UpsertWorkspaceCleanupJob(_ context.Context, job domain.WorkspaceCleanupJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	if existingID, ok := s.cleanupJobBySession[job.PracticeSessionID]; ok {
		existing := s.cleanupJobs[existingID]
		existing.WorkspaceID = job.WorkspaceID
		existing.Reason = job.Reason
		existing.ScheduledAt = job.ScheduledAt.UTC()
		existing.Status = job.Status
		existing.LastError = ""
		existing.UpdatedAt = now
		s.cleanupJobs[existingID] = existing
		return nil
	}

	job.ID = s.nextCleanupJobID
	s.nextCleanupJobID++
	job.ScheduledAt = job.ScheduledAt.UTC()
	job.CreatedAt = now
	job.UpdatedAt = now
	s.cleanupJobs[job.ID] = job
	s.cleanupJobBySession[job.PracticeSessionID] = job.ID
	return nil
}

func (s *InMemoryPracticeSessionStore) ClaimDueWorkspaceCleanupJobs(_ context.Context, now time.Time, limit int) ([]domain.WorkspaceCleanupJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 {
		return []domain.WorkspaceCleanupJob{}, nil
	}

	dueJobs := make([]domain.WorkspaceCleanupJob, 0, len(s.cleanupJobs))
	staleRunningBefore := now.UTC().Add(-WorkspaceCleanupJobLeaseTimeout)
	for _, job := range s.cleanupJobs {
		switch job.Status {
		case "pending", "failed":
			if workspaceCleanupAttemptsExhausted(job.AttemptCount) {
				continue
			}
			if job.ScheduledAt.After(now) {
				continue
			}
		case "running":
			if workspaceCleanupAttemptsExhausted(job.AttemptCount) {
				continue
			}
			if job.UpdatedAt.After(staleRunningBefore) {
				continue
			}
		default:
			continue
		}
		dueJobs = append(dueJobs, job)
	}

	sort.Slice(dueJobs, func(i, j int) bool {
		if dueJobs[i].ScheduledAt.Equal(dueJobs[j].ScheduledAt) {
			return dueJobs[i].ID < dueJobs[j].ID
		}
		return dueJobs[i].ScheduledAt.Before(dueJobs[j].ScheduledAt)
	})
	if len(dueJobs) > limit {
		dueJobs = dueJobs[:limit]
	}

	claimedAt := now.UTC()
	for i := range dueJobs {
		job := dueJobs[i]
		job.Status = "running"
		job.AttemptCount++
		job.LastError = ""
		job.UpdatedAt = claimedAt
		s.cleanupJobs[job.ID] = job
		dueJobs[i] = job
	}

	return dueJobs, nil
}

func (s *InMemoryPracticeSessionStore) MarkWorkspaceCleanupJobSucceeded(_ context.Context, jobID uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.cleanupJobs[jobID]
	if !ok {
		return fmt.Errorf("workspace cleanup job not found")
	}

	job.Status = "succeeded"
	job.LastError = ""
	job.UpdatedAt = time.Now().UTC()
	s.cleanupJobs[jobID] = job
	return nil
}

func (s *InMemoryPracticeSessionStore) MarkWorkspaceCleanupJobFailed(_ context.Context, jobID uint64, scheduledAt time.Time, lastErr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.cleanupJobs[jobID]
	if !ok {
		return fmt.Errorf("workspace cleanup job not found")
	}

	job.Status = "failed"
	job.ScheduledAt = scheduledAt.UTC()
	job.LastError = lastErr
	job.UpdatedAt = time.Now().UTC()
	s.cleanupJobs[jobID] = job
	return nil
}

func (s *InMemoryPracticeSessionStore) WorkspaceCleanupJobByID(_ context.Context, jobID uint64) (domain.WorkspaceCleanupJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.cleanupJobs[jobID]
	if !ok {
		return domain.WorkspaceCleanupJob{}, ErrWorkspaceCleanupJobNotFound
	}

	return job, nil
}

func (s *InMemoryPracticeSessionStore) RequeueWorkspaceCleanupJob(_ context.Context, jobID uint64, scheduledAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, ok := s.cleanupJobs[jobID]
	if !ok {
		return ErrWorkspaceCleanupJobNotFound
	}

	job.Status = "pending"
	job.AttemptCount = 0
	job.LastError = ""
	job.ScheduledAt = scheduledAt.UTC()
	job.UpdatedAt = time.Now().UTC()
	s.cleanupJobs[jobID] = job
	return nil
}

func (s *InMemoryPracticeSessionStore) ListPracticeSessionsMissingWorkspaceCleanupJob(_ context.Context, limit int) ([]domain.PracticeSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 {
		return []domain.PracticeSession{}, nil
	}

	sessionIDs := make([]uint64, 0, len(s.sessions))
	for id := range s.sessions {
		sessionIDs = append(sessionIDs, id)
	}
	sort.Slice(sessionIDs, func(i, j int) bool { return sessionIDs[i] < sessionIDs[j] })

	sessions := make([]domain.PracticeSession, 0, len(sessionIDs))
	for _, id := range sessionIDs {
		session := s.sessions[id]
		if session.Status != PracticeSessionStatusExpired && session.Status != PracticeSessionStatusOrphaned {
			continue
		}
		if _, ok := s.cleanupJobBySession[session.ID]; ok {
			continue
		}
		sessions = append(sessions, session)
		if len(sessions) == limit {
			break
		}
	}

	return sessions, nil
}

func (s *InMemoryPracticeSessionStore) ListExhaustedWorkspaceCleanupJobs(_ context.Context, limit int) ([]domain.WorkspaceCleanupJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 {
		return []domain.WorkspaceCleanupJob{}, nil
	}

	jobIDs := make([]uint64, 0, len(s.cleanupJobs))
	for id := range s.cleanupJobs {
		jobIDs = append(jobIDs, id)
	}
	sort.Slice(jobIDs, func(i, j int) bool { return jobIDs[i] < jobIDs[j] })

	jobs := make([]domain.WorkspaceCleanupJob, 0, len(jobIDs))
	for _, id := range jobIDs {
		job := s.cleanupJobs[id]
		if job.Status != "failed" || !workspaceCleanupAttemptsExhausted(job.AttemptCount) {
			continue
		}
		jobs = append(jobs, job)
		if len(jobs) == limit {
			break
		}
	}

	return jobs, nil
}
