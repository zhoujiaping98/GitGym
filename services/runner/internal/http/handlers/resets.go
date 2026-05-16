package handlers

import (
	"net/http"
	"os"
	"path/filepath"

	"gitgym/services/runner/internal/engine"
	"github.com/go-chi/chi/v5"
)

func ResetWorkspace(workRoot string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := chi.URLParam(r, "workspaceID")
		workspacePath, err := resolveWorkspacePath(workRoot, workspaceID)
		if err != nil {
			writeWorkspaceError(w, err)
			return
		}

		if err := resetWorkspaceContents(workspacePath); err != nil {
			writeWorkspaceJSON(w, http.StatusInternalServerError, map[string]any{
				"error": err.Error(),
			})
			return
		}

		writeWorkspaceJSON(w, http.StatusAccepted, map[string]any{
			"workspace_id": workspaceID,
			"status":       "resetting",
		})
	}
}

func resetWorkspaceContents(workspacePath string) error {
	entries, err := os.ReadDir(workspacePath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(workspacePath, entry.Name())); err != nil {
			return err
		}
	}

	if err := engine.InitStandardTemplate(workspacePath); err != nil {
		return err
	}

	return engine.InitWorkspaceRepository(workspacePath)
}
