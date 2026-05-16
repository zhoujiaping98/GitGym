package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gitgym/services/runner/internal/engine"
	"github.com/go-chi/chi/v5"
)

type runCommandRequest struct {
	Command string `json:"command"`
}

type runCommandResponse struct {
	WorkspaceID string `json:"workspace_id"`
	Status      string `json:"status"`
	Command     string `json:"command"`
	Stdout      string `json:"stdout"`
	Stderr      string `json:"stderr"`
	ExitCode    int    `json:"exit_code"`
	DurationMS  int    `json:"duration_ms"`
}

func RunCommand(workRoot string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := chi.URLParam(r, "workspaceID")
		workspacePath, err := resolveWorkspacePath(workRoot, workspaceID)
		if err != nil {
			writeWorkspaceError(w, err)
			return
		}

		var req runCommandRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeWorkspaceJSON(w, http.StatusBadRequest, map[string]any{
				"error": "invalid JSON body",
			})
			return
		}

		result, err := engine.RunCommand(workspacePath, req.Command)
		if err != nil {
			writeWorkspaceJSON(w, http.StatusBadRequest, map[string]any{
				"error": err.Error(),
			})
			return
		}

		writeWorkspaceJSON(w, http.StatusOK, runCommandResponse{
			WorkspaceID: workspaceID,
			Status:      "completed",
			Command:     req.Command,
			Stdout:      result.Stdout,
			Stderr:      result.Stderr,
			ExitCode:    result.ExitCode,
			DurationMS:  result.DurationMS,
		})
	}
}

func resolveWorkspacePath(workRoot string, workspaceID string) (string, error) {
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

	info, err := os.Stat(workspaceAbs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", os.ErrNotExist
		}
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("workspace path is not a directory")
	}

	return workspaceAbs, nil
}

func validateWorkspaceID(workspaceID string) error {
	if strings.TrimSpace(workspaceID) == "" {
		return errors.New("workspace ID is required")
	}
	if workspaceID == "." || workspaceID == ".." {
		return errors.New("workspace ID is malformed")
	}
	if strings.Contains(workspaceID, "/") || strings.Contains(workspaceID, "\\") {
		return errors.New("workspace ID is malformed")
	}
	return nil
}

func writeWorkspaceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, os.ErrNotExist):
		writeWorkspaceJSON(w, http.StatusNotFound, map[string]any{
			"error": "workspace not found",
		})
	default:
		writeWorkspaceJSON(w, http.StatusBadRequest, map[string]any{
			"error": err.Error(),
		})
	}
}

func writeWorkspaceJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
