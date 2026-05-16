package engine

import "testing"

func TestEventRecorderCopiesPayloadOnRecord(t *testing.T) {
	recorder := NewEventRecorder()
	payload := map[string]any{"raw": "git status"}

	recorded := recorder.Record("command_started", "ws-1", payload)
	payload["raw"] = "mutated"
	recorded.Payload["raw"] = "also mutated"

	events := recorder.Events()
	if got := events[0].Payload["raw"]; got != "git status" {
		t.Fatalf("expected stored payload to be immutable copy, got %#v", got)
	}
}

func TestEventRecorderCopiesPayloadOnEvents(t *testing.T) {
	recorder := NewEventRecorder()
	recorder.Record("command_started", "ws-1", map[string]any{"raw": "git status"})

	events := recorder.Events()
	events[0].Payload["raw"] = "mutated"

	fresh := recorder.Events()
	if got := fresh[0].Payload["raw"]; got != "git status" {
		t.Fatalf("expected returned payload copy, got %#v", got)
	}
}
