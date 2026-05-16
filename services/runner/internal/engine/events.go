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
		cloned[key] = copyPayloadValue(value)
	}

	return cloned
}

func copyPayloadValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return copyPayload(typed)
	case []any:
		cloned := make([]any, len(typed))
		for i, item := range typed {
			cloned[i] = copyPayloadValue(item)
		}
		return cloned
	case Snapshot:
		cloned := typed
		if typed.StatusSummary != nil {
			cloned.StatusSummary = append([]string(nil), typed.StatusSummary...)
		}
		return cloned
	default:
		return value
	}
}
