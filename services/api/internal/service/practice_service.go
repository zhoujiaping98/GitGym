package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"gitgym/services/api/internal/domain"
	"gitgym/services/api/internal/runner"
)

const practiceSessionTTL = 2 * time.Hour

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
}

type PracticeService interface {
	ListTemplates(ctx context.Context) []PracticeTemplate
	CreatePracticeSession(ctx context.Context, input CreatePracticeSessionInput) (domain.PracticeSession, error)
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
	template, ok := s.templateByID(input.TemplateID)
	if !ok {
		return domain.PracticeSession{}, fmt.Errorf("unknown template ID %d", input.TemplateID)
	}
	if s.runner == nil {
		return domain.PracticeSession{}, fmt.Errorf("runner client is not configured")
	}
	if s.store == nil {
		return domain.PracticeSession{}, fmt.Errorf("practice session store is not configured")
	}

	workspace, err := s.runner.CreateWorkspace(ctx, template.Key)
	if err != nil {
		return domain.PracticeSession{}, fmt.Errorf("create runner workspace: %w", err)
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

func (s *practiceService) templateByID(templateID uint64) (PracticeTemplate, bool) {
	for _, template := range s.templates {
		if template.ID == templateID {
			return template, true
		}
	}
	return PracticeTemplate{}, false
}

type InMemoryPracticeSessionStore struct {
	mu       sync.Mutex
	nextID   uint64
	sessions map[uint64]domain.PracticeSession
}

func NewInMemoryPracticeSessionStore() *InMemoryPracticeSessionStore {
	return &InMemoryPracticeSessionStore{
		nextID:   1,
		sessions: make(map[uint64]domain.PracticeSession),
	}
}

func (s *InMemoryPracticeSessionStore) CreatePracticeSession(_ context.Context, session domain.PracticeSession) (domain.PracticeSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session.ID = s.nextID
	s.nextID++
	s.sessions[session.ID] = session
	return session, nil
}
