package handlers

import (
	"encoding/json"
	"net/http"

	"gitgym/services/api/internal/service"
)

type practiceCatalogResponse struct {
	Templates []service.PracticeTemplate `json:"templates"`
	Scenarios []service.PracticeScenario `json:"scenarios"`
}

func ListPracticeTemplates(practiceService service.PracticeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		templates, err := practiceService.ListTemplatesWithError(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": err.Error(),
			})
			return
		}

		scenarios, err := practiceService.ListScenariosWithError(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusOK, practiceCatalogResponse{
			Templates: templates,
			Scenarios: scenarios,
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
