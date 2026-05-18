package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sync"

	unixpty "github.com/creack/pty"
	winpty "github.com/threatexpert/go-winpty"
)

type TerminalManager struct {
	mu       sync.Mutex
	sessions map[string]*terminalSessionSlot
}

type TerminalSession struct {
	WorkspaceID   string
	WorkspacePath string

	mu          sync.Mutex
	backend     terminalBackend
	process     *os.Process
	terminate   func() error
	wait        func() error
	subscribers map[int]chan []byte
	nextID      int
	closed      bool
	closeErr    error
	backendErr  error
	waitErr     error
	done        chan struct{}
	doneOnce    sync.Once
	backendOnce sync.Once
	waitOnce    sync.Once
}

type terminalSessionSlotState int

const (
	slotStateStarting terminalSessionSlotState = iota
	slotStateActive
	slotStateReleasing
)

type terminalSessionSlot struct {
	session  *TerminalSession
	state    terminalSessionSlotState
	ready    chan struct{}
	released chan struct{}
}

type terminalBackend interface {
	io.ReadWriteCloser
	Resize(cols uint16, rows uint16) error
}

type unixTerminalBackend struct {
	file *os.File
}

func (b *unixTerminalBackend) Read(p []byte) (int, error) {
	return b.file.Read(p)
}

func (b *unixTerminalBackend) Write(p []byte) (int, error) {
	return b.file.Write(p)
}

func (b *unixTerminalBackend) Close() error {
	return b.file.Close()
}

func (b *unixTerminalBackend) Resize(cols uint16, rows uint16) error {
	return unixpty.Setsize(b.file, &unixpty.Winsize{Cols: cols, Rows: rows})
}

type windowsTerminalBackend struct {
	pty winpty.Pty
}

func (b *windowsTerminalBackend) Read(p []byte) (int, error) {
	return b.pty.Read(p)
}

func (b *windowsTerminalBackend) Write(p []byte) (int, error) {
	return b.pty.Write(p)
}

func (b *windowsTerminalBackend) Close() error {
	return b.pty.Close()
}

func (b *windowsTerminalBackend) Resize(cols uint16, rows uint16) error {
	return b.pty.Resize(int(cols), int(rows))
}

func NewTerminalManager() *TerminalManager {
	return &TerminalManager{
		sessions: make(map[string]*terminalSessionSlot),
	}
}

func (m *TerminalManager) Acquire(ctx context.Context, workspacePath string, workspaceID string) (*TerminalSession, error) {
	if err := ensureWorkspacePath(workspacePath); err != nil {
		return nil, err
	}

	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		m.mu.Lock()
		if m.sessions == nil {
			m.sessions = make(map[string]*terminalSessionSlot)
		}
		if slot, ok := m.sessions[workspaceID]; ok {
			switch slot.state {
			case slotStateActive:
				session := slot.session
				if session != nil && !session.isClosed() {
					m.mu.Unlock()
					return session, nil
				}
				delete(m.sessions, workspaceID)
			case slotStateStarting:
				ready := slot.ready
				m.mu.Unlock()
				if err := waitForSlot(ctx, ready); err != nil {
					return nil, err
				}
				continue
			case slotStateReleasing:
				released := slot.released
				m.mu.Unlock()
				if err := waitForSlot(ctx, released); err != nil {
					return nil, err
				}
				continue
			default:
				delete(m.sessions, workspaceID)
			}
		}

		slot := &terminalSessionSlot{
			state: slotStateStarting,
			ready: make(chan struct{}),
		}
		m.sessions[workspaceID] = slot
		m.mu.Unlock()

		session, err := startTerminalSession(ctx, workspacePath, workspaceID)

		m.mu.Lock()
		current := m.sessions[workspaceID]
		if current == slot {
			if err != nil {
				delete(m.sessions, workspaceID)
			} else {
				slot.session = session
				slot.state = slotStateActive
			}
			close(slot.ready)
		}
		m.mu.Unlock()

		if err != nil {
			return nil, err
		}

		return session, nil
	}
}

func (m *TerminalManager) Release(workspaceID string) error {
	for {
		m.mu.Lock()
		slot := m.sessions[workspaceID]
		if slot == nil {
			m.mu.Unlock()
			return nil
		}

		switch slot.state {
		case slotStateStarting:
			ready := slot.ready
			m.mu.Unlock()
			<-ready
			continue
		case slotStateReleasing:
			released := slot.released
			m.mu.Unlock()
			<-released
			return nil
		case slotStateActive:
			session := slot.session
			slot.state = slotStateReleasing
			slot.released = make(chan struct{})
			released := slot.released
			m.mu.Unlock()

			var err error
			if session != nil {
				err = session.close()
			}

			m.mu.Lock()
			if m.sessions[workspaceID] == slot {
				delete(m.sessions, workspaceID)
			}
			close(released)
			m.mu.Unlock()
			return err
		default:
			delete(m.sessions, workspaceID)
			m.mu.Unlock()
			return nil
		}
	}
}

func (s *TerminalSession) WriteInput(data string) error {
	s.mu.Lock()
	if s.closed {
		err := s.closeErr
		s.mu.Unlock()
		if err != nil {
			return err
		}
		return os.ErrClosed
	}
	backend := s.backend
	s.mu.Unlock()

	if backend == nil {
		return errors.New("terminal session does not accept input")
	}

	_, err := io.WriteString(backend, data)
	return err
}

func (s *TerminalSession) Resize(cols uint16, rows uint16) error {
	s.mu.Lock()
	if s.closed {
		err := s.closeErr
		s.mu.Unlock()
		if err != nil {
			return err
		}
		return os.ErrClosed
	}
	backend := s.backend
	s.mu.Unlock()

	if backend == nil {
		return errors.New("terminal session does not support resizing")
	}

	return backend.Resize(cols, rows)
}

func (s *TerminalSession) ReadLoop(ctx context.Context, onData func([]byte) error) error {
	if onData == nil {
		return errors.New("terminal read callback is required")
	}

	subscription, unsubscribe, err := s.subscribe()
	if err != nil {
		return err
	}
	defer unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case chunk, ok := <-subscription:
			if !ok {
				s.mu.Lock()
				err := s.closeErr
				s.mu.Unlock()
				if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, os.ErrClosed) {
					return err
				}
				return nil
			}
			if err := onData(chunk); err != nil {
				return err
			}
		}
	}
}

func (s *TerminalSession) Wait() error {
	return s.waitProcess()
}

func (s *TerminalSession) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.closed
}

func ensureWorkspacePath(workspacePath string) error {
	info, err := os.Stat(workspacePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("workspace path does not exist: %s", workspacePath)
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("workspace path is not a directory: %s", workspacePath)
	}
	return nil
}

func startTerminalSession(ctx context.Context, workspacePath string, workspaceID string) (*TerminalSession, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	session := &TerminalSession{
		WorkspaceID:   workspaceID,
		WorkspacePath: workspacePath,
		subscribers:   make(map[int]chan []byte),
		done:          make(chan struct{}),
	}

	if runtime.GOOS == "windows" {
		if err := startWindowsShell(ctx, session); err != nil {
			return nil, err
		}
		return session, nil
	}

	if err := startUnixShell(ctx, session); err != nil {
		return nil, err
	}

	return session, nil
}

func startUnixShell(ctx context.Context, session *TerminalSession) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	command, args, env, err := posixShellStartupSpec(exec.LookPath)
	if err != nil {
		return err
	}

	cmd := exec.Command(command, args...)
	cmd.Dir = session.WorkspacePath
	configureTerminalCommand(cmd)
	cmd.Env = append(os.Environ(), env...)

	ptmx, err := unixpty.Start(cmd)
	if err != nil {
		return err
	}

	session.mu.Lock()
	session.backend = &unixTerminalBackend{file: ptmx}
	session.process = cmd.Process
	session.terminate = func() error {
		return terminateTerminalProcessTree(cmd.Process)
	}
	session.wait = cmd.Wait
	session.mu.Unlock()

	go session.pump(session.backend)
	go session.monitorProcessExit()
	return nil
}

func startWindowsShell(ctx context.Context, session *TerminalSession) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	pty, err := winpty.New()
	if err != nil {
		return err
	}

	command, args, err := shellCommand()
	if err != nil {
		_ = pty.Close()
		return err
	}
	if err := ctx.Err(); err != nil {
		_ = pty.Close()
		return err
	}

	cmd := pty.Command(command, args...)
	cmd.Dir = session.WorkspacePath

	if err := cmd.Start(); err != nil {
		_ = pty.Close()
		return err
	}

	session.mu.Lock()
	session.backend = &windowsTerminalBackend{pty: pty}
	session.process = cmd.Process
	session.terminate = func() error {
		return terminateTerminalProcessTree(cmd.Process)
	}
	session.wait = cmd.Wait
	session.mu.Unlock()

	go session.pump(session.backend)
	go session.monitorProcessExit()
	return nil
}

func shellCommand() (string, []string, error) {
	if runtime.GOOS == "windows" {
		command, err := exec.LookPath("powershell.exe")
		if err != nil {
			return "", nil, err
		}
		return command, []string{
			"-NoLogo",
			"-NoProfile",
			"-NoExit",
			"-Command",
			powershellPromptBootstrap(),
		}, nil
	}

	command, args, _, err := posixShellStartupSpec(exec.LookPath)
	if err != nil {
		return "", nil, err
	}
	return command, args, nil
}

func posixShellStartupSpec(lookPath func(string) (string, error)) (string, []string, []string, error) {
	command, err := lookPath("bash")
	if err != nil {
		return "", nil, nil, errors.New("interactive terminal requires bash on non-windows hosts")
	}
	return command, []string{"--noprofile", "--norc", "-i"}, []string{
		"PS1=$ ",
		"HISTFILE=/dev/null",
		"HISTCONTROL=",
		"PROMPT_COMMAND=" + posixPromptCommandBootstrap(),
	}, nil
}

func powershellPromptBootstrap() string {
	return fmt.Sprintf(
		"$ErrorActionPreference='SilentlyContinue'; "+
			"$global:GitGymOriginalPrompt = $function:prompt; "+
			"$global:GitGymLastSubmittedCommand = $null; "+
			"$global:GitGymLastSubmittedSequence = 0; "+
			"$global:GitGymLastReportedSequence = 0; "+
			"function global:GitGymWriteCommandMarker([int]$gitgymExit, [string]$gitgymPrompt) { "+
			"if ($gitgymPrompt -match '^\\s*>>') { [Console]::Out.WriteLine('%s'); return }; "+
			"if ([int]$global:GitGymLastSubmittedSequence -le [int]$global:GitGymLastReportedSequence) { return }; "+
			"$global:GitGymLastReportedSequence = [int]$global:GitGymLastSubmittedSequence; "+
			"$gitgymCommand = [string]$global:GitGymLastSubmittedCommand; "+
			"if ([string]::IsNullOrWhiteSpace($gitgymCommand)) { return }; "+
			"$gitgymEncoded = [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($gitgymCommand)); "+
			"[Console]::Out.WriteLine('%s:' + $gitgymExit + ':' + $gitgymEncoded) }; "+
			"function global:prompt { "+
			"$gitgymSuccess = $?; "+
			"$gitgymExit = if ($gitgymSuccess) { 0 } elseif ($null -ne $global:LASTEXITCODE -and $global:LASTEXITCODE -is [int]) { [int]$global:LASTEXITCODE } else { 1 }; "+
			"$gitgymPrompt = if ($global:GitGymOriginalPrompt) { & $global:GitGymOriginalPrompt } else { 'PS ' + $executionContext.SessionState.Path.CurrentLocation + '> ' }; "+
			"GitGymWriteCommandMarker $gitgymExit $gitgymPrompt; "+
			"$gitgymPrompt }; "+
			"Import-Module PSReadLine -ErrorAction SilentlyContinue; "+
			"Set-PSReadLineOption -AddToHistoryHandler { param($line) $global:GitGymLastSubmittedCommand = $line; $global:GitGymLastSubmittedSequence = [int]$global:GitGymLastSubmittedSequence + 1; return $true } -ErrorAction SilentlyContinue; "+
			"Set-PSReadLineOption -HistorySaveStyle SaveNothing -ErrorAction SilentlyContinue",
		TerminalContinuationPromptMarker,
		TerminalCommandExitMarker,
	)
}

func posixPromptCommandBootstrap() string {
	return fmt.Sprintf(
		"shopt -s cmdhist lithist; __gitgym_exit=$?; __gitgym_history_id=${HISTCMD:-}; "+
			"if [ -n \"$__gitgym_history_id\" ] && [ \"$__gitgym_history_id\" != \"$__GITGYM_LAST_HISTORY_ID\" ]; then "+
			"__gitgym_command=$(fc -ln -1); "+
			"if [ -n \"$__gitgym_command\" ]; then "+
			"__gitgym_encoded=$(printf %%s \"$__gitgym_command\" | base64 | tr -d '\\n'); "+
			"printf '%s:%%s:%%s\\n' \"$__gitgym_exit\" \"$__gitgym_encoded\"; "+
			"__GITGYM_LAST_HISTORY_ID=\"$__gitgym_history_id\"; "+
			"fi; "+
			"fi",
		TerminalCommandExitMarker,
	)
}

func (s *TerminalSession) pump(reader io.ReadCloser) {
	buffer := make([]byte, 4096)
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			s.broadcast(append([]byte(nil), buffer[:n]...))
		}
		if err != nil {
			s.finish(err)
			_ = s.closeBackend()
			return
		}
	}
}

func (s *TerminalSession) monitorProcessExit() {
	_ = s.waitProcess()
}

func (s *TerminalSession) broadcast(chunk []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	for _, subscriber := range s.subscribers {
		subscriber <- chunk
	}
}

func (s *TerminalSession) subscribe() (<-chan []byte, func(), error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		if s.closeErr != nil {
			return nil, nil, s.closeErr
		}
		return nil, nil, os.ErrClosed
	}

	id := s.nextID
	s.nextID++

	ch := make(chan []byte, 32)
	s.subscribers[id] = ch

	unsubscribe := func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		subscriber, ok := s.subscribers[id]
		if !ok {
			return
		}

		delete(s.subscribers, id)
		close(subscriber)
	}

	return ch, unsubscribe, nil
}

func (s *TerminalSession) close() error {
	s.finish(os.ErrClosed)

	s.mu.Lock()
	backend := s.backend
	terminate := s.terminate
	s.mu.Unlock()

	var releaseErr error
	if terminate != nil {
		releaseErr = errors.Join(releaseErr, terminate())
	}
	if backend != nil {
		releaseErr = errors.Join(releaseErr, s.closeBackend())
	}
	releaseErr = errors.Join(releaseErr, s.reapProcess())

	return releaseErr
}

func (s *TerminalSession) finish(err error) {
	s.doneOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.closeErr = err
		for id, subscriber := range s.subscribers {
			delete(s.subscribers, id)
			close(subscriber)
		}
		s.mu.Unlock()

		close(s.done)
	})
}

func (s *TerminalSession) closeBackend() error {
	s.backendOnce.Do(func() {
		s.mu.Lock()
		backend := s.backend
		s.mu.Unlock()

		if backend != nil {
			s.backendErr = backend.Close()
		}
	})

	return s.backendErr
}

func (s *TerminalSession) waitProcess() error {
	s.waitOnce.Do(func() {
		s.mu.Lock()
		wait := s.wait
		s.mu.Unlock()

		if wait == nil {
			s.waitErr = errors.New("terminal session does not have a running process")
			return
		}

		s.waitErr = wait()
		s.finish(normalizeWaitCloseErr(s.waitErr))
	})

	return s.waitErr
}

func (s *TerminalSession) reapProcess() error {
	err := s.waitProcess()
	var exitErr *exec.ExitError
	if err == nil || errors.As(err, &exitErr) || errors.Is(err, os.ErrProcessDone) {
		return nil
	}
	return err
}

func normalizeWaitCloseErr(err error) error {
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &exitErr):
		return io.EOF
	default:
		return err
	}
}

func waitForSlot(ctx context.Context, ch <-chan struct{}) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ch:
		return nil
	}
}
