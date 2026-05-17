package test

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"

	"gitgym/services/runner/internal/engine"
)

func TestTerminalManagerStartsShellForWorkspace(t *testing.T) {
	workspace := createGitWorkspace(t)

	manager := engine.NewTerminalManager()
	session, err := manager.Acquire(context.Background(), workspace.Path, workspace.ID)
	if err != nil {
		t.Fatalf("acquire terminal session: %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Release(workspace.ID); err != nil {
			t.Fatalf("release terminal session: %v", err)
		}
	})

	output := readTerminalUntil(t, session, `Write-Output (Get-Location).Path`+"\r\n", workspace.Path)
	if !strings.Contains(output, workspace.Path) {
		t.Fatalf("expected terminal output to include workspace path %q, got %q", workspace.Path, output)
	}

	reused, err := manager.Acquire(context.Background(), workspace.Path, workspace.ID)
	if err != nil {
		t.Fatalf("re-acquire terminal session: %v", err)
	}
	if reused != session {
		t.Fatal("expected terminal manager to reuse the existing session for the workspace")
	}
}

func TestTerminalManagerRejectsMissingWorkspace(t *testing.T) {
	manager := engine.NewTerminalManager()

	if _, err := manager.Acquire(context.Background(), t.TempDir()+"\\missing", "missing-workspace"); err == nil {
		t.Fatal("expected error for missing workspace path")
	}
}

func TestTerminalManagerWritesInputToPTY(t *testing.T) {
	workspace := createGitWorkspace(t)

	manager := engine.NewTerminalManager()
	session, err := manager.Acquire(context.Background(), workspace.Path, workspace.ID)
	if err != nil {
		t.Fatalf("acquire terminal session: %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Release(workspace.ID); err != nil {
			t.Fatalf("release terminal session: %v", err)
		}
	})

	output := readTerminalUntil(t, session, `Write-Output "__GITGYM_WRITE_INPUT__"`+"\r\n", "__GITGYM_WRITE_INPUT__")
	if !strings.Contains(output, "__GITGYM_WRITE_INPUT__") {
		t.Fatalf("expected terminal output to include echoed marker, got %q", output)
	}
}

func TestTerminalManagerResizesPTY(t *testing.T) {
	workspace := createGitWorkspace(t)

	manager := engine.NewTerminalManager()
	session, err := manager.Acquire(context.Background(), workspace.Path, workspace.ID)
	if err != nil {
		t.Fatalf("acquire terminal session: %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Release(workspace.ID); err != nil {
			t.Fatalf("release terminal session: %v", err)
		}
	})

	if err := session.Resize(120, 40); err != nil {
		t.Fatalf("resize terminal session: %v", err)
	}
}

func TestTerminalManagerClosesShellOnRelease(t *testing.T) {
	workspace := createGitWorkspace(t)

	manager := engine.NewTerminalManager()
	session, err := manager.Acquire(context.Background(), workspace.Path, workspace.ID)
	if err != nil {
		t.Fatalf("acquire terminal session: %v", err)
	}

	if err := manager.Release(workspace.ID); err != nil {
		t.Fatalf("release terminal session: %v", err)
	}

	if err := session.WriteInput("Write-Output \"after-release\"\r\n"); err == nil {
		t.Fatal("expected writes to fail after terminal release")
	}

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- session.Cmd.Wait()
	}()

	select {
	case err := <-waitDone:
		var exitErr *exec.ExitError
		if err != nil && !errors.As(err, &exitErr) {
			t.Fatalf("expected released shell to exit, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for shell process to exit after release")
	}
}

func readTerminalUntil(t *testing.T, session *engine.TerminalSession, input string, want string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var builder strings.Builder
	readDone := make(chan error, 1)
	go func() {
		readDone <- session.ReadLoop(ctx, func(chunk []byte) error {
			builder.Write(chunk)
			if strings.Contains(builder.String(), want) {
				cancel()
			}
			return nil
		})
	}()

	if err := session.WriteInput(input); err != nil {
		t.Fatalf("write terminal input: %v", err)
	}

	err := <-readDone
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("read terminal output: %v", err)
	}

	output := builder.String()
	if !strings.Contains(output, want) {
		t.Fatalf("expected terminal output to include %q, got %q", want, output)
	}

	return output
}
