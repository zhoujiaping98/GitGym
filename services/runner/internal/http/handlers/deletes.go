package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitgym/services/runner/internal/engine"
	"github.com/go-chi/chi/v5"
)

type deleteWorkspaceRequest struct {
	Reason             string `json:"reason"`
	DeleteAfterSeconds int    `json:"delete_after_seconds"`
}

func DeleteWorkspace(workRoot string, terminalManager *engine.TerminalManager, cleanup *engine.WorkspaceCleanupManager) http.HandlerFunc {
	if cleanup == nil {
		cleanup = engine.NewWorkspaceCleanupManager(terminalManager)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := chi.URLParam(r, "workspaceID")
		workspacePath, err := resolveWorkspacePath(workRoot, workspaceID)
		missingWorkspace := false
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				writeWorkspaceError(w, err)
				return
			}
			missingWorkspace = true
			workspacePath, err = resolveWorkspaceCleanupPath(workRoot, workspaceID)
			if err != nil {
				writeWorkspaceError(w, err)
				return
			}
		}

		var req deleteWorkspaceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeWorkspaceJSON(w, http.StatusBadRequest, map[string]any{
				"error": "invalid JSON body",
			})
			return
		}
		if req.DeleteAfterSeconds < 0 {
			writeWorkspaceJSON(w, http.StatusBadRequest, map[string]any{
				"error": "delete_after_seconds must be non-negative",
			})
			return
		}

		if err := cleanup.Schedule(r.Context(), engine.WorkspaceCleanupRequest{
			WorkspaceID: workspaceID,
			Path:        workspacePath,
			Reason:      req.Reason,
			DeleteAfter: time.Duration(req.DeleteAfterSeconds) * time.Second,
		}); err != nil {
			writeWorkspaceJSON(w, http.StatusInternalServerError, map[string]any{
				"error": err.Error(),
			})
			return
		}

		if missingWorkspace {
			writeWorkspaceJSON(w, http.StatusNotFound, map[string]any{
				"error": "workspace not found",
			})
			return
		}
		if req.DeleteAfterSeconds <= 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeWorkspaceJSON(w, http.StatusAccepted, map[string]any{
			"workspace_id": workspaceID,
			"status":       "cleanup_scheduled",
		})
	}
}

func resolveWorkspaceCleanupPath(workRoot string, workspaceID string) (string, error) {
	if err := validateWorkspaceID(workspaceID); err != nil {
		return "", err
	}

	rootAbs, err := filepath.Abs(workRoot)
	if err != nil {
		return "", err
	}

	workspacePath := filepath.Join(rootAbs, workspaceID)
	workspaceAbs, err := filepath.Abs(workspacePath)
	if err != nil {
		return "", err
	}

	relativePath, err := filepath.Rel(rootAbs, workspaceAbs)
	if err != nil {
		return "", err
	}
	if relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(os.PathSeparator)) {
		return "", errors.New("workspace path escapes root")
	}

	return workspaceAbs, nil
}
