package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"gitgym/services/runner/internal/engine"
)

type createWorkspaceRequest struct {
	Template string `json:"template"`
}

type createWorkspaceResponse struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Template string `json:"template"`
}

func CreateWorkspace(workRoot string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createWorkspaceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		if req.Template != "" && req.Template != "standard" {
			http.Error(w, "unknown template", http.StatusBadRequest)
			return
		}

		workspace, err := engine.CreateWorkspace(workRoot)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(createWorkspaceResponse{
			ID:       workspace.ID,
			Path:     workspace.Path,
			Template: workspace.Template,
		})
	}
}
