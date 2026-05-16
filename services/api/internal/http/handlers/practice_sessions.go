package handlers

import (
	"encoding/json"
	"net/http"

	"gitgym/services/api/internal/service"
)

type createPracticeSessionRequest struct {
	UserID     uint64 `json:"user_id"`
	ScenarioID uint64 `json:"scenario_id"`
	TemplateID uint64 `json:"template_id"`
}

func CreatePracticeSession(practiceService service.PracticeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createPracticeSessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": "invalid JSON body",
			})
			return
		}
		if req.UserID == 0 || req.ScenarioID == 0 || req.TemplateID == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": "user_id, scenario_id, and template_id are required",
			})
			return
		}

		session, err := practiceService.CreatePracticeSession(r.Context(), service.CreatePracticeSessionInput{
			UserID:     req.UserID,
			ScenarioID: req.ScenarioID,
			TemplateID: req.TemplateID,
		})
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{
				"error": err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"session": session,
		})
	}
}
