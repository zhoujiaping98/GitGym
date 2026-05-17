package engine

import (
	"context"
	"testing"
	"time"
)

func TestTerminalManagerAcquireWaitsForReleasingWorkspaceSlot(t *testing.T) {
	manager := NewTerminalManager()
	released := make(chan struct{})
	manager.sessions = map[string]*terminalSessionSlot{
		"workspace-1": {
			state:    slotStateReleasing,
			released: released,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if _, err := manager.Acquire(ctx, t.TempDir(), "workspace-1"); err == nil {
		t.Fatal("expected acquire to wait for releasing slot and honor context timeout")
	}

	close(released)
}
