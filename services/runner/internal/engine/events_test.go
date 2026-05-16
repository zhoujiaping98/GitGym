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

func TestEventRecorderDeepCopiesNestedJSONLikePayloads(t *testing.T) {
	recorder := NewEventRecorder()
	payload := map[string]any{
		"meta": map[string]any{
			"args": []any{
				"git",
				map[string]any{"message": "hello"},
			},
		},
	}

	recorded := recorder.Record("command_started", "ws-1", payload)

	payload["meta"].(map[string]any)["args"].([]any)[1].(map[string]any)["message"] = "mutated source"
	recorded.Payload["meta"].(map[string]any)["args"].([]any)[1].(map[string]any)["message"] = "mutated return"

	fresh := recorder.Events()
	got := fresh[0].Payload["meta"].(map[string]any)["args"].([]any)[1].(map[string]any)["message"]
	if got != "hello" {
		t.Fatalf("expected nested payload to remain unchanged, got %#v", got)
	}
}

func TestEventRecorderDeepCopiesSnapshotPayloadsOnEvents(t *testing.T) {
	recorder := NewEventRecorder()
	recorder.Record("command_started", "ws-1", map[string]any{
		"pre_snapshot": Snapshot{
			HeadCommit:    "abc123",
			BranchName:    "main",
			StatusSummary: []string{"M events.go"},
		},
	})

	events := recorder.Events()
	snapshot := events[0].Payload["pre_snapshot"].(Snapshot)
	snapshot.StatusSummary[0] = "mutated"
	events[0].Payload["pre_snapshot"] = snapshot

	fresh := recorder.Events()
	got := fresh[0].Payload["pre_snapshot"].(Snapshot).StatusSummary[0]
	if got != "M events.go" {
		t.Fatalf("expected snapshot payload to remain unchanged, got %#v", got)
	}
}
