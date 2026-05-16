package engine

import "time"

type SessionEvent struct {
	Type        string
	WorkspaceID string
	CreatedAt   time.Time
	Payload     map[string]any
}
