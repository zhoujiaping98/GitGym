package test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"gitgym/services/runner/internal/engine"
)

func TestCleanupManagerDeletesWorkspaceImmediately(t *testing.T) {
	t.Parallel()

	workRoot := t.TempDir()
	workspace, err := engine.CreateWorkspace(workRoot)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	completions := make(chan cleanupCompletion, 1)
	cleanup := engine.NewWorkspaceCleanupManagerWithHooks(engine.NewTerminalManager(), os.RemoveAll, nil, func(req engine.WorkspaceCleanupRequest, err error) {
		completions <- cleanupCompletion{workspaceID: req.WorkspaceID, err: err}
	})

	if err := cleanup.Schedule(context.Background(), engine.WorkspaceCleanupRequest{
		WorkspaceID: workspace.ID,
		Path:        workspace.Path,
		Reason:      "expired",
		DeleteAfter: 0,
	}); err != nil {
		t.Fatalf("schedule cleanup: %v", err)
	}

	if _, err := os.Stat(workspace.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected workspace to be deleted, got %v", err)
	}
	requireCleanupCompletion(t, completions, workspace.ID, nil)
}

func TestCleanupManagerDeletesWorkspaceAfterDelay(t *testing.T) {
	t.Parallel()

	workRoot := t.TempDir()
	workspace, err := engine.CreateWorkspace(workRoot)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	trigger := make(chan time.Time, 1)
	removed := make(chan string, 1)
	cleanup := engine.NewWorkspaceCleanupManagerWithHooks(engine.NewTerminalManager(), func(path string) error {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
		removed <- path
		return nil
	}, func(delay time.Duration) <-chan time.Time {
		if delay != 20*time.Millisecond {
			t.Fatalf("expected delay 20ms, got %v", delay)
		}
		return trigger
	}, nil)

	if err := cleanup.Schedule(context.Background(), engine.WorkspaceCleanupRequest{
		WorkspaceID: workspace.ID,
		Path:        workspace.Path,
		Reason:      "orphaned",
		DeleteAfter: 20 * time.Millisecond,
	}); err != nil {
		t.Fatalf("schedule delayed cleanup: %v", err)
	}

	if _, err := os.Stat(workspace.Path); err != nil {
		t.Fatalf("expected workspace to still exist before grace period, got %v", err)
	}

	trigger <- time.Now()
	requireRemovedPath(t, removed, workspace.Path)

	if _, err := os.Stat(workspace.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected workspace to be deleted after grace period, got %v", err)
	}
}

func TestCleanupManagerReleasesTerminalBeforeDeletingWorkspace(t *testing.T) {
	t.Parallel()

	workspace := createGitWorkspace(t)
	terminalManager := engine.NewTerminalManager()

	session, err := terminalManager.Acquire(context.Background(), workspace.Path, workspace.ID)
	if err != nil {
		t.Fatalf("acquire terminal session: %v", err)
	}
	t.Cleanup(func() {
		_ = session.Wait()
	})

	removeCalled := false
	cleanup := engine.NewWorkspaceCleanupManagerWithRemover(terminalManager, func(path string) error {
		removeCalled = true
		if path != workspace.Path {
			t.Fatalf("expected cleanup path %q, got %q", workspace.Path, path)
		}
		if err := session.WriteInput(shellPrintLine(terminalMarker("cleanup"), "still-open")); !errors.Is(err, os.ErrClosed) {
			t.Fatalf("expected terminal session to be closed before deletion, got %v", err)
		}
		return os.RemoveAll(path)
	})
	if err := cleanup.Schedule(context.Background(), engine.WorkspaceCleanupRequest{
		WorkspaceID: workspace.ID,
		Path:        workspace.Path,
		Reason:      "expired",
		DeleteAfter: 0,
	}); err != nil {
		t.Fatalf("schedule cleanup: %v", err)
	}

	if !removeCalled {
		t.Fatal("expected workspace deletion to be invoked")
	}

	if _, err := os.Stat(workspace.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected workspace to be deleted, got %v", err)
	}
}

func TestCleanupManagerDelayedCleanupSurvivesContextCancellation(t *testing.T) {
	t.Parallel()

	workRoot := t.TempDir()
	workspace, err := engine.CreateWorkspace(workRoot)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	trigger := make(chan time.Time, 1)
	removed := make(chan string, 1)
	cleanup := engine.NewWorkspaceCleanupManagerWithHooks(engine.NewTerminalManager(), func(path string) error {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
		removed <- path
		return nil
	}, func(delay time.Duration) <-chan time.Time {
		if delay != 20*time.Millisecond {
			t.Fatalf("expected delay 20ms, got %v", delay)
		}
		return trigger
	}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	if err := cleanup.Schedule(ctx, engine.WorkspaceCleanupRequest{
		WorkspaceID: workspace.ID,
		Path:        workspace.Path,
		Reason:      "orphaned",
		DeleteAfter: 20 * time.Millisecond,
	}); err != nil {
		t.Fatalf("schedule delayed cleanup: %v", err)
	}
	cancel()

	trigger <- time.Now()
	requireRemovedPath(t, removed, workspace.Path)

	if _, err := os.Stat(workspace.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected workspace to be deleted, got %v", err)
	}
}

func TestCleanupManagerReschedulesDelayedCleanupForSameWorkspace(t *testing.T) {
	t.Parallel()

	workRoot := t.TempDir()
	workspace, err := engine.CreateWorkspace(workRoot)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	var (
		mu          sync.Mutex
		removeCalls []string
		triggers    = make(map[time.Duration]chan time.Time)
		completions = make(chan cleanupCompletion, 2)
	)

	cleanup := engine.NewWorkspaceCleanupManagerWithHooks(engine.NewTerminalManager(), func(path string) error {
		mu.Lock()
		removeCalls = append(removeCalls, path)
		mu.Unlock()
		return os.RemoveAll(path)
	}, func(delay time.Duration) <-chan time.Time {
		ch := make(chan time.Time, 1)
		mu.Lock()
		triggers[delay] = ch
		mu.Unlock()
		return ch
	}, func(req engine.WorkspaceCleanupRequest, err error) {
		completions <- cleanupCompletion{workspaceID: req.WorkspaceID, err: err}
	})

	if err := cleanup.Schedule(context.Background(), engine.WorkspaceCleanupRequest{
		WorkspaceID: workspace.ID,
		Path:        workspace.Path,
		Reason:      "orphaned",
		DeleteAfter: 30 * time.Millisecond,
	}); err != nil {
		t.Fatalf("schedule first delayed cleanup: %v", err)
	}

	if err := cleanup.Schedule(context.Background(), engine.WorkspaceCleanupRequest{
		WorkspaceID: workspace.ID,
		Path:        workspace.Path,
		Reason:      "expired",
		DeleteAfter: 10 * time.Millisecond,
	}); err != nil {
		t.Fatalf("schedule second delayed cleanup: %v", err)
	}

	secondTrigger := requireScheduledTrigger(t, &mu, triggers, 10*time.Millisecond)
	requireCleanupCompletion(t, completions, workspace.ID, engine.ErrWorkspaceCleanupSuperseded)
	secondTrigger <- time.Now()
	requireCleanupCompletion(t, completions, workspace.ID, nil)

	mu.Lock()
	defer mu.Unlock()
	if len(removeCalls) != 1 {
		t.Fatalf("expected only replacement cleanup to delete workspace once, got %d calls", len(removeCalls))
	}
	if removeCalls[0] != workspace.Path {
		t.Fatalf("expected cleanup path %q, got %q", workspace.Path, removeCalls[0])
	}
}

func TestCleanupManagerRetainsAsyncDeleteFailure(t *testing.T) {
	t.Parallel()

	workRoot := t.TempDir()
	workspace, err := engine.CreateWorkspace(workRoot)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	trigger := make(chan time.Time, 1)
	wantErr := errors.New("remove failed")
	completions := make(chan cleanupCompletion, 1)
	cleanup := engine.NewWorkspaceCleanupManagerWithHooks(engine.NewTerminalManager(), func(string) error {
		return wantErr
	}, func(time.Duration) <-chan time.Time {
		return trigger
	}, func(req engine.WorkspaceCleanupRequest, err error) {
		completions <- cleanupCompletion{workspaceID: req.WorkspaceID, err: err}
	})

	if err := cleanup.Schedule(context.Background(), engine.WorkspaceCleanupRequest{
		WorkspaceID: workspace.ID,
		Path:        workspace.Path,
		Reason:      "orphaned",
		DeleteAfter: 15 * time.Millisecond,
	}); err != nil {
		t.Fatalf("schedule delayed cleanup: %v", err)
	}

	trigger <- time.Now()
	requireCleanupCompletion(t, completions, workspace.ID, wantErr)

	if err := cleanup.LastCleanupError(workspace.ID); !errors.Is(err, wantErr) {
		t.Fatalf("expected retained cleanup error %v, got %v", wantErr, err)
	}
}

func requireRemovedPath(t *testing.T, removed <-chan string, want string) {
	t.Helper()

	select {
	case got := <-removed:
		if got != want {
			t.Fatalf("expected removed path %q, got %q", want, got)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatalf("timed out waiting for removal of %q", want)
	}
}

func requireScheduledTrigger(t *testing.T, mu *sync.Mutex, triggers map[time.Duration]chan time.Time, want time.Duration) chan time.Time {
	t.Helper()

	deadline := time.Now().Add(250 * time.Millisecond)
	for {
		mu.Lock()
		if trigger := triggers[want]; trigger != nil {
			mu.Unlock()
			return trigger
		}
		mu.Unlock()

		if time.Now().After(deadline) {
			t.Fatalf("expected scheduled cleanup trigger for %v", want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func requireCleanupCompletion(t *testing.T, completions <-chan cleanupCompletion, workspaceID string, wantErr error) {
	t.Helper()

	select {
	case completion := <-completions:
		if completion.workspaceID != workspaceID {
			t.Fatalf("expected cleanup completion for %q, got %q", workspaceID, completion.workspaceID)
		}
		if !errors.Is(completion.err, wantErr) {
			t.Fatalf("expected cleanup completion error %v, got %v", wantErr, completion.err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatalf("timed out waiting for cleanup completion for %q", workspaceID)
	}
}

type cleanupCompletion struct {
	workspaceID string
	err         error
}
