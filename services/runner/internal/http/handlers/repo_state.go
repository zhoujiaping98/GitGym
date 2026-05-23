package handlers

import (
	"net/http"
	"time"

	"gitgym/services/runner/internal/engine"
	"github.com/go-chi/chi/v5"
)

type repoStateResponse struct {
	BranchName    string    `json:"branch_name"`
	HeadCommit    string    `json:"head_commit"`
	StatusSummary []string  `json:"status_summary"`
	CapturedAt    time.Time `json:"captured_at"`
}

func RepoState(workRoot string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := chi.URLParam(r, "workspaceID")
		workspacePath, err := resolveWorkspacePath(workRoot, workspaceID)
		if err != nil {
			writeWorkspaceError(w, err)
			return
		}

		snapshot, err := engine.CaptureSnapshot(workspacePath)
		if err != nil {
			writeWorkspaceJSON(w, http.StatusInternalServerError, map[string]any{
				"error": err.Error(),
			})
			return
		}

		writeWorkspaceJSON(w, http.StatusOK, repoStateResponse{
			BranchName:    snapshot.BranchName,
			HeadCommit:    snapshot.HeadCommit,
			StatusSummary: snapshot.StatusSummary,
			CapturedAt:    snapshot.CapturedAt,
		})
	}
}
