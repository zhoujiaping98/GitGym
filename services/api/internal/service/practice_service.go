package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"gitgym/services/api/internal/domain"
	"gitgym/services/api/internal/runner"
)

const (
	practiceSessionTTL                = 2 * time.Hour
	practiceSessionOrphanCleanupGrace = 10 * time.Minute
)

const (
	PracticeSessionStatusActive   = "active"
	PracticeSessionStatusExpired  = "expired"
	PracticeSessionStatusOrphaned = "orphaned"
)

var (
	ErrInvalidPracticeSessionInput  = errors.New("invalid practice session input")
	ErrUnknownPracticeScenario      = errors.New("unknown practice scenario")
	ErrUnknownPracticeTemplate      = errors.New("unknown practice template")
	ErrPracticeServiceConfiguration = errors.New("practice service configuration error")
	ErrRunnerWorkspaceCreation      = errors.New("runner workspace creation failed")
	ErrRunnerWorkspaceReset         = errors.New("runner workspace reset failed")
	ErrRunnerRepoStateUnavailable   = errors.New("runner repository state unavailable")
	ErrRunnerTerminalUnavailable    = errors.New("runner terminal unavailable")
	ErrPracticeSessionNotFound      = errors.New("practice session not found")
	ErrPracticeSessionExpired       = errors.New("practice session expired")
	ErrPracticeSessionOrphaned      = errors.New("practice session workspace is unavailable")
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

type WorkspaceCleanupJob struct {
	PracticeSessionID uint64
	WorkspaceID       string
	Reason            string
	ScheduledAt       time.Time
	Status            string
}

type PracticeSessionStore interface {
	CreatePracticeSession(ctx context.Context, session domain.PracticeSession) (domain.PracticeSession, error)
	CurrentPracticeSession(ctx context.Context, userID uint64) (domain.PracticeSession, error)
	PracticeSessionByID(ctx context.Context, sessionID uint64) (domain.PracticeSession, error)
	UpdatePracticeSession(ctx context.Context, session domain.PracticeSession) (domain.PracticeSession, error)
	ExpirePracticeSessions(ctx context.Context, before time.Time, endedAt time.Time) ([]domain.PracticeSession, error)
	UpsertWorkspaceCleanupJob(ctx context.Context, job WorkspaceCleanupJob) error
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

	job := WorkspaceCleanupJob{
		PracticeSessionID: session.ID,
		WorkspaceID:       session.RunnerRef,
		Reason:            reason,
		ScheduledAt:       scheduledAt.UTC(),
		Status:            "pending",
	}
	if err := s.store.UpsertWorkspaceCleanupJob(ctx, job); err != nil {
		log.Printf("practice cleanup job upsert failed for session %d: %v", session.ID, err)
	}
}

type InMemoryPracticeSessionStore struct {
	mu            sync.Mutex
	nextID        uint64
	sessions      map[uint64]domain.PracticeSession
	currentByUser map[uint64]uint64
}

func NewInMemoryPracticeSessionStore() *InMemoryPracticeSessionStore {
	return &InMemoryPracticeSessionStore{
		nextID:        1,
		sessions:      make(map[uint64]domain.PracticeSession),
		currentByUser: make(map[uint64]uint64),
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

func (s *InMemoryPracticeSessionStore) UpsertWorkspaceCleanupJob(_ context.Context, _ WorkspaceCleanupJob) error {
	return nil
}
