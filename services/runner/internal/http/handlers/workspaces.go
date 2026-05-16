package handlers

import (
	"encoding/json"
	"net/http"

	"gitgym/services/runner/internal/engine"
)

type createWorkspaceResponse struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Template string `json:"template"`
}

func CreateWorkspace(workRoot string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
