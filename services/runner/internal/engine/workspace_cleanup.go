package engine

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"time"
)

var ErrWorkspaceCleanupSuperseded = errors.New("workspace cleanup superseded")

type WorkspaceCleanupRequest struct {
	WorkspaceID string
	Path        string
	Reason      string
	DeleteAfter time.Duration
}

type WorkspaceCleanupManager struct {
	mu       sync.Mutex
	terminal *TerminalManager
	remove   func(string) error
	after    func(time.Duration) <-chan time.Time
	complete func(WorkspaceCleanupRequest, error)
	pending  map[string]*workspaceCleanupPending
	failures map[string]error
}

type workspaceCleanupPending struct {
	cancel context.CancelFunc
}

func NewWorkspaceCleanupManager(terminal *TerminalManager) *WorkspaceCleanupManager {
	return NewWorkspaceCleanupManagerWithRemover(terminal, os.RemoveAll)
}

func NewWorkspaceCleanupManagerWithRemover(terminal *TerminalManager, remove func(string) error) *WorkspaceCleanupManager {
	return NewWorkspaceCleanupManagerWithHooks(terminal, remove, time.After, nil)
}

func NewWorkspaceCleanupManagerWithHooks(terminal *TerminalManager, remove func(string) error, after func(time.Duration) <-chan time.Time, complete func(WorkspaceCleanupRequest, error)) *WorkspaceCleanupManager {
	if remove == nil {
		remove = os.RemoveAll
	}
	if after == nil {
		after = time.After
	}

	return &WorkspaceCleanupManager{
		terminal: terminal,
		remove:   remove,
		after:    after,
		complete: complete,
		pending:  make(map[string]*workspaceCleanupPending),
		failures: make(map[string]error),
	}
}

func (m *WorkspaceCleanupManager) Schedule(ctx context.Context, req WorkspaceCleanupRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(req.WorkspaceID) == "" {
		return errors.New("workspace cleanup requires workspace ID")
	}
	if strings.TrimSpace(req.Path) == "" {
		return errors.New("workspace cleanup requires workspace path")
	}

	if req.DeleteAfter <= 0 {
		m.cancelPending(req.WorkspaceID)
		err := m.deleteNow(req)
		m.recordCleanupResult(req.WorkspaceID, err)
		m.completeAsync(req, err)
		return err
	}

	timerCtx, cancel := context.WithCancel(context.Background())
	entry := &workspaceCleanupPending{cancel: cancel}

	m.mu.Lock()
	if existing, ok := m.pending[req.WorkspaceID]; ok {
		existing.cancel()
	}
	m.pending[req.WorkspaceID] = entry
	m.mu.Unlock()

	go func() {
		select {
		case <-timerCtx.Done():
			m.clearPending(req.WorkspaceID, entry)
			m.completeAsync(req, ErrWorkspaceCleanupSuperseded)
			return
		case <-m.after(req.DeleteAfter):
			if !m.beginPendingDelete(req.WorkspaceID, entry) {
				m.completeAsync(req, ErrWorkspaceCleanupSuperseded)
				return
			}
			err := m.deleteNow(req)
			m.recordCleanupResult(req.WorkspaceID, err)
			m.completeAsync(req, err)
		}
	}()

	return nil
}

func (m *WorkspaceCleanupManager) deleteNow(req WorkspaceCleanupRequest) error {
	if m.terminal != nil {
		if err := m.terminal.Release(req.WorkspaceID); err != nil {
			return err
		}
	}

	if err := m.remove(req.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return nil
}

func (m *WorkspaceCleanupManager) LastCleanupError(workspaceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.failures[workspaceID]
}

func (m *WorkspaceCleanupManager) cancelPending(workspaceID string) {
	m.mu.Lock()
	entry := m.pending[workspaceID]
	delete(m.pending, workspaceID)
	m.mu.Unlock()

	if entry != nil {
		entry.cancel()
	}
}

func (m *WorkspaceCleanupManager) beginPendingDelete(workspaceID string, entry *workspaceCleanupPending) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if current := m.pending[workspaceID]; current != entry {
		return false
	}

	delete(m.pending, workspaceID)
	return true
}

func (m *WorkspaceCleanupManager) clearPending(workspaceID string, entry *workspaceCleanupPending) {
	m.mu.Lock()
	if current := m.pending[workspaceID]; current == entry {
		delete(m.pending, workspaceID)
	}
	m.mu.Unlock()
}

func (m *WorkspaceCleanupManager) recordCleanupResult(workspaceID string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err == nil {
		delete(m.failures, workspaceID)
		return
	}
	m.failures[workspaceID] = err
}

func (m *WorkspaceCleanupManager) completeAsync(req WorkspaceCleanupRequest, err error) {
	if m.complete != nil {
		m.complete(req, err)
	}
}
