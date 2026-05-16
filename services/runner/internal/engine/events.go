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
	storedPayload := copyPayload(payload)
	event := SessionEvent{
		Type:        eventType,
		WorkspaceID: workspaceID,
		CreatedAt:   time.Now().UTC(),
		Payload:     storedPayload,
	}

	r.mu.Lock()
	r.events = append(r.events, event)
	r.mu.Unlock()

	event.Payload = copyPayload(storedPayload)
	return event
}

func (r *EventRecorder) Events() []SessionEvent {
	r.mu.Lock()
	defer r.mu.Unlock()

	events := make([]SessionEvent, len(r.events))
	for i, event := range r.events {
		events[i] = SessionEvent{
			Type:        event.Type,
			WorkspaceID: event.WorkspaceID,
			CreatedAt:   event.CreatedAt,
			Payload:     copyPayload(event.Payload),
		}
	}
	return events
}

func copyPayload(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}

	cloned := make(map[string]any, len(payload))
	for key, value := range payload {
		cloned[key] = value
	}

	return cloned
}
