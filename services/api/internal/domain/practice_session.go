package domain

import "time"

type WorkspaceCleanupJob struct {
	ID                uint64
	PracticeSessionID uint64
	WorkspaceID       string
	Reason            string
	ScheduledAt       time.Time
	Status            string
	AttemptCount      uint32
	LastError         string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
