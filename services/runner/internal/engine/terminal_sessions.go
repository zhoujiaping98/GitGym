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

	"github.com/creack/pty"
)

type TerminalManager struct {
	mu       sync.Mutex
	sessions map[string]*TerminalSession
}

type TerminalSession struct {
	WorkspaceID   string
	WorkspacePath string
	Cmd           *exec.Cmd
	PTY           *os.File

	mu          sync.Mutex
	stdin       io.WriteCloser
	subscribers map[int]chan []byte
	nextID      int
	closed      bool
	closeErr    error
	done        chan struct{}
	doneOnce    sync.Once
}

func NewTerminalManager() *TerminalManager {
	return &TerminalManager{
		sessions: make(map[string]*TerminalSession),
	}
}

func (m *TerminalManager) Acquire(ctx context.Context, workspacePath string, workspaceID string) (*TerminalSession, error) {
	if err := ensureWorkspacePath(workspacePath); err != nil {
		return nil, err
	}

	m.mu.Lock()
	if m.sessions == nil {
		m.sessions = make(map[string]*TerminalSession)
	}
	if session, ok := m.sessions[workspaceID]; ok {
		m.mu.Unlock()
		return session, nil
	}
	m.mu.Unlock()

	session, err := startTerminalSession(ctx, workspacePath, workspaceID)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.sessions[workspaceID]; ok {
		_ = session.close()
		return existing, nil
	}

	m.sessions[workspaceID] = session
	return session, nil
}

func (m *TerminalManager) Release(workspaceID string) error {
	m.mu.Lock()
	session := m.sessions[workspaceID]
	delete(m.sessions, workspaceID)
	m.mu.Unlock()

	if session == nil {
		return nil
	}

	return session.close()
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
	writer := s.stdin
	s.mu.Unlock()

	if writer == nil {
		return errors.New("terminal session does not accept input")
	}

	_, err := io.WriteString(writer, data)
	return err
}

func (s *TerminalSession) Resize(cols uint16, rows uint16) error {
	if s.PTY == nil {
		return nil
	}

	return pty.Setsize(s.PTY, &pty.Winsize{Cols: cols, Rows: rows})
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
	cmd := shellCommand(ctx)
	cmd.Dir = workspacePath

	session := &TerminalSession{
		WorkspaceID:   workspaceID,
		WorkspacePath: workspacePath,
		Cmd:           cmd,
		subscribers:   make(map[int]chan []byte),
		done:          make(chan struct{}),
	}

	if runtime.GOOS == "windows" {
		if err := startWindowsShell(session); err != nil {
			return nil, err
		}
		return session, nil
	}

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}

	session.PTY = ptmx
	session.stdin = ptmx
	go session.pump(ptmx)
	return session, nil
}

func shellCommand(ctx context.Context) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile")
	}
	return exec.CommandContext(ctx, "sh", "-l")
}

func startWindowsShell(session *TerminalSession) error {
	stdin, err := session.Cmd.StdinPipe()
	if err != nil {
		return err
	}

	stdout, err := session.Cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return err
	}

	if err := session.Cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return err
	}

	session.stdin = stdin
	go session.pump(stdout)
	return nil
}

func (s *TerminalSession) pump(reader io.ReadCloser) {
	defer func() {
		_ = reader.Close()
	}()

	buffer := make([]byte, 4096)
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			s.broadcast(append([]byte(nil), buffer[:n]...))
		}
		if err != nil {
			s.finish(err)
			return
		}
	}
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

	var releaseErr error
	if s.stdin != nil {
		releaseErr = errors.Join(releaseErr, s.stdin.Close())
	}
	if s.PTY != nil {
		releaseErr = errors.Join(releaseErr, s.PTY.Close())
	}
	if s.Cmd != nil && s.Cmd.Process != nil {
		if err := s.Cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			releaseErr = errors.Join(releaseErr, err)
		}
	}

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
