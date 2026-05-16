package engine

import (
	"sync"
	"time"
)

type SessionEvent struct {
	Type        string
	WorkspaceID string
	CreatedAt   time.Time
	Payload     map[string]any
}

type EventRecorder struct {
	mu     sync.Mutex
	events []SessionEvent
}

func NewEventRecorder() *EventRecorder {
	return &EventRecorder{}
}

func (r *EventRecorder) Record(eventType string, workspaceID string, payload map[string]any) SessionEvent {
	event := SessionEvent{
		Type:        eventType,
		WorkspaceID: workspaceID,
		CreatedAt:   time.Now().UTC(),
		Payload:     payload,
	}

	r.mu.Lock()
	r.events = append(r.events, event)
	r.mu.Unlock()

	return event
}

func (r *EventRecorder) Events() []SessionEvent {
	r.mu.Lock()
	defer r.mu.Unlock()

	events := make([]SessionEvent, len(r.events))
	copy(events, r.events)
	return events
}
