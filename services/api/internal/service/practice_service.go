package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"gitgym/services/api/internal/domain"
	"gitgym/services/api/internal/runner"
)

const practiceSessionTTL = 2 * time.Hour

var (
	ErrInvalidPracticeSessionInput  = errors.New("invalid practice session input")
	ErrUnknownPracticeTemplate      = errors.New("unknown practice template")
	ErrPracticeServiceConfiguration = errors.New("practice service configuration error")
	ErrRunnerWorkspaceCreation      = errors.New("runner workspace creation failed")
	ErrRunnerWorkspaceReset         = errors.New("runner workspace reset failed")
	ErrPracticeSessionNotFound      = errors.New("practice session not found")
)

type PracticeTemplate struct {
	ID   uint64 `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

type CreatePracticeSessionInput struct {
	UserID     uint64
	ScenarioID uint64
	TemplateID uint64
}

type PracticeSessionStore interface {
	CreatePracticeSession(ctx context.Context, session domain.PracticeSession) (domain.PracticeSession, error)
	CurrentPracticeSession(ctx context.Context, userID uint64) (domain.PracticeSession, error)
	PracticeSessionByID(ctx context.Context, sessionID uint64) (domain.PracticeSession, error)
}

type PracticeService interface {
	ListTemplates(ctx context.Context) []PracticeTemplate
	CreatePracticeSession(ctx context.Context, input CreatePracticeSessionInput) (domain.PracticeSession, error)
	ResetPracticeSession(ctx context.Context, userID uint64, sessionID uint64) error
	CurrentPracticeSession(ctx context.Context, userID uint64) (domain.PracticeSession, error)
	PracticeSessionByID(ctx context.Context, userID uint64, sessionID uint64) (domain.PracticeSession, error)
}

type practiceService struct {
	store     PracticeSessionStore
	runner    runner.Client
	now       func() time.Time
	templates []PracticeTemplate
}

func NewPracticeService(store PracticeSessionStore, runnerClient runner.Client, now func() time.Time) PracticeService {
	if now == nil {
		now = time.Now
	}

	return &practiceService{
		store:  store,
		runner: runnerClient,
		now:    now,
		templates: []PracticeTemplate{
			{ID: 1, Key: "standard", Name: "Standard"},
		},
	}
}

func (s *practiceService) ListTemplates(_ context.Context) []PracticeTemplate {
	templates := make([]PracticeTemplate, len(s.templates))
	copy(templates, s.templates)
	return templates
}

func (s *practiceService) CreatePracticeSession(ctx context.Context, input CreatePracticeSessionInput) (domain.PracticeSession, error) {
	if input.UserID == 0 || input.ScenarioID == 0 || input.TemplateID == 0 {
		return domain.PracticeSession{}, fmt.Errorf("%w", ErrInvalidPracticeSessionInput)
	}

	template, ok := s.templateByID(input.TemplateID)
	if !ok {
		return domain.PracticeSession{}, fmt.Errorf("%w: %d", ErrUnknownPracticeTemplate, input.TemplateID)
	}
	if s.runner == nil {
		return domain.PracticeSession{}, fmt.Errorf("%w: runner client is not configured", ErrPracticeServiceConfiguration)
	}
	if s.store == nil {
		return domain.PracticeSession{}, fmt.Errorf("%w: practice session store is not configured", ErrPracticeServiceConfiguration)
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
		ScenarioID:       input.ScenarioID,
		TemplateID:       input.TemplateID,
		RunnerRef:        workspace.ID,
		WorkspacePathRef: workspace.Path,
		Status:           "active",
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
	return session, nil
}

func (s *practiceService) templateByID(templateID uint64) (PracticeTemplate, bool) {
	for _, template := range s.templates {
		if template.ID == templateID {
			return template, true
		}
	}
	return PracticeTemplate{}, false
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
