package test

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
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
		releaseTerminalSession(t, manager, session, workspace.ID)
	})

	marker := terminalMarker("cwd")
	pattern := terminalLinePattern(marker, `([^\r\n]+)`)
	output := readTerminalUntilMatch(t, session, shellPrintWorkingDirectory(marker), pattern)
	match := pattern.FindStringSubmatch(output)
	if len(match) != 2 {
		t.Fatalf("expected terminal output to include working directory marker %q, got %q", marker, output)
	}
	if !samePath(match[1], workspace.Path) {
		t.Fatalf("expected shell working directory %q, got %q", workspace.Path, match[1])
	}
	assertCleanShellStartup(t, output)

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
		releaseTerminalSession(t, manager, session, workspace.ID)
	})

	marker := terminalMarker("echo")
	pattern := terminalLinePattern(marker, `__GITGYM_WRITE_INPUT__`)
	output := readTerminalUntilMatch(t, session, shellPrintLine(marker, "__GITGYM_WRITE_INPUT__"), pattern)
	if !pattern.MatchString(output) {
		t.Fatalf("expected terminal output to include echoed marker, got %q", output)
	}
}

func TestTerminalManagerKeepsShellAliveAfterAcquireContextCancellation(t *testing.T) {
	workspace := createGitWorkspace(t)

	acquireCtx, cancelAcquire := context.WithCancel(context.Background())
	manager := engine.NewTerminalManager()
	session, err := manager.Acquire(acquireCtx, workspace.Path, workspace.ID)
	if err != nil {
		t.Fatalf("acquire terminal session: %v", err)
	}
	t.Cleanup(func() {
		releaseTerminalSession(t, manager, session, workspace.ID)
	})

	cancelAcquire()

	marker := terminalMarker("context")
	pattern := terminalLinePattern(marker, `__GITGYM_CONTEXT_ALIVE__`)
	output := readTerminalUntilMatch(t, session, shellPrintLine(marker, "__GITGYM_CONTEXT_ALIVE__"), pattern)
	if !pattern.MatchString(output) {
		t.Fatalf("expected terminal output after acquire context cancellation, got %q", output)
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
		releaseTerminalSession(t, manager, session, workspace.ID)
	})

	initialCols, initialRows := readTerminalSize(t, session)
	targetCols, targetRows := resizedDimensions(initialCols, initialRows)

	if err := session.Resize(targetCols, targetRows); err != nil {
		t.Fatalf("resize terminal session: %v", err)
	}

	resizedCols, resizedRows := readTerminalSize(t, session)
	if resizedCols != targetCols || resizedRows != targetRows {
		t.Fatalf("expected terminal size %dx%d after resize, got %dx%d (initial %dx%d)", targetCols, targetRows, resizedCols, resizedRows, initialCols, initialRows)
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
		waitDone <- session.Wait()
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

func TestShellPrintWorkingDirectoryUsesValidUnixCommand(t *testing.T) {
	got := shellPrintWorkingDirectoryForOS("linux", "__MARKER__")
	want := "printf '__MARKER__:%s\\n' \"$PWD\"\n"
	if got != want {
		t.Fatalf("expected unix cwd helper %q, got %q", want, got)
	}
}

func TestShellPrintSizeUsesValidUnixCommand(t *testing.T) {
	got := shellPrintSizeForOS("linux", "__MARKER__")
	want := "set -- $(stty size); printf '__MARKER__:%sx%s\\n' \"$2\" \"$1\"\n"
	if got != want {
		t.Fatalf("expected unix size helper %q, got %q", want, got)
	}
}

func readTerminalUntilMatch(t *testing.T, session *engine.TerminalSession, input string, want *regexp.Regexp) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var builder strings.Builder
	readDone := make(chan error, 1)
	go func() {
		readDone <- session.ReadLoop(ctx, func(chunk []byte) error {
			builder.Write(chunk)
			if want.MatchString(builder.String()) {
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
	if !want.MatchString(output) {
		t.Fatalf("expected terminal output to match %q, got %q", want.String(), output)
	}

	return output
}

func readTerminalSize(t *testing.T, session *engine.TerminalSession) (uint16, uint16) {
	t.Helper()

	marker := terminalMarker("size")
	pattern := terminalLinePattern(marker, `(\d+)x(\d+)`)
	output := readTerminalUntilMatch(t, session, shellPrintSize(marker), pattern)
	match := pattern.FindStringSubmatch(output)
	if len(match) != 3 {
		t.Fatalf("expected terminal output to include size marker %q, got %q", marker, output)
	}

	cols, err := strconv.ParseUint(match[1], 10, 16)
	if err != nil {
		t.Fatalf("parse terminal cols from %q: %v", match[1], err)
	}
	rows, err := strconv.ParseUint(match[2], 10, 16)
	if err != nil {
		t.Fatalf("parse terminal rows from %q: %v", match[2], err)
	}

	return uint16(cols), uint16(rows)
}

func releaseTerminalSession(t *testing.T, manager *engine.TerminalManager, session *engine.TerminalSession, workspaceID string) {
	t.Helper()

	if err := manager.Release(workspaceID); err != nil {
		t.Fatalf("release terminal session: %v", err)
	}

	waitDone := make(chan error, 1)
	go func() {
		waitDone <- session.Wait()
	}()

	select {
	case err := <-waitDone:
		var exitErr *exec.ExitError
		if err != nil && !errors.As(err, &exitErr) {
			t.Fatalf("wait for terminal session exit: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for terminal session to exit")
	}
}

func resizedDimensions(cols uint16, rows uint16) (uint16, uint16) {
	targetCols := cols + 12
	targetRows := rows + 6
	if targetCols == cols {
		targetCols++
	}
	if targetRows == rows {
		targetRows++
	}
	return targetCols, targetRows
}

func terminalMarker(prefix string) string {
	return fmt.Sprintf("__GITGYM_%s_%d__", strings.ToUpper(prefix), time.Now().UnixNano())
}

func terminalLinePattern(marker string, valuePattern string) *regexp.Regexp {
	return regexp.MustCompile(`(?:^|[\r\n])(?:\x1b\[[0-9;?]*[A-Za-z])*` + regexp.QuoteMeta(marker) + `:` + valuePattern + `(?:\x1b\[[0-9;?]*[A-Za-z])*(?:[\r\n]|$)`)
}

func shellPrintWorkingDirectory(marker string) string {
	return shellPrintWorkingDirectoryForOS(runtime.GOOS, marker)
}

func shellPrintLine(marker string, value string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("Write-Output \"%s:%s\"\r\n", marker, value)
	}

	return fmt.Sprintf("printf '%s:%s\\n'\n", marker, value)
}

func shellPrintSize(marker string) string {
	return shellPrintSizeForOS(runtime.GOOS, marker)
}

func shellExit() string {
	if runtime.GOOS == "windows" {
		return "exit\r\n"
	}

	return "exit\n"
}

func samePath(got string, want string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(got, want)
	}

	return got == want
}

func shellPrintWorkingDirectoryForOS(goos string, marker string) string {
	if goos == "windows" {
		return fmt.Sprintf("Write-Output \"%s:$((Get-Location).Path)\"\r\n", marker)
	}

	return fmt.Sprintf("printf '%s:%%s\\n' \"$PWD\"\n", marker)
}

func shellPrintSizeForOS(goos string, marker string) string {
	if goos == "windows" {
		return fmt.Sprintf("Write-Output \"%s:$($Host.UI.RawUI.WindowSize.Width)x$($Host.UI.RawUI.WindowSize.Height)\"\r\n", marker)
	}

	return fmt.Sprintf("set -- $(stty size); printf '%s:%%sx%%s\\n' \"$2\" \"$1\"\n", marker)
}

func assertCleanShellStartup(t *testing.T, output string) {
	t.Helper()

	if runtime.GOOS != "windows" {
		return
	}

	for _, fragment := range []string{
		"PSSecurityException",
		"PowerShell_profile.ps1",
		"Microsoft.PowerShell_profile.ps1",
		"PSReadLine",
	} {
		if strings.Contains(output, fragment) {
			t.Fatalf("expected clean shell startup without %q noise, got %q", fragment, output)
		}
	}
}
