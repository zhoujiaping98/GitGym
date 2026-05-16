package domain

import "time"

type CurrentUser struct {
	ID          uint64
	GitHubID    uint64
	GitHubLogin string
	DisplayName string
	AvatarURL   string
	Email       string
}

type PracticeSession struct {
	ID               uint64
	UserID           uint64
	ScenarioID       uint64
	TemplateID       uint64
	RunnerRef        string
	WorkspacePathRef string
	Status           string
	StartedAt        time.Time
	ExpiresAt        time.Time
	LastActivityAt   time.Time
}
