package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"gitgym/services/api/internal/domain"
	"gitgym/services/api/internal/http/middleware"
	"gitgym/services/api/internal/service"
)

type createPracticeSessionRequest struct {
	ScenarioID uint64 `json:"scenario_id"`
	TemplateID uint64 `json:"template_id"`
}

type practiceSessionResponse struct {
	ID             uint64     `json:"id"`
	UserID         uint64     `json:"user_id"`
	ScenarioID     uint64     `json:"scenario_id"`
	TemplateID     uint64     `json:"template_id"`
	RunnerRef      string     `json:"runner_ref"`
	WorkspacePath  string     `json:"workspace_path"`
	Status         string     `json:"status"`
	StartedAt      time.Time  `json:"started_at"`
	EndedAt        *time.Time `json:"ended_at,omitempty"`
	ExpiresAt      time.Time  `json:"expires_at"`
	LastActivityAt time.Time  `json:"last_activity_at"`
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
		if req.ScenarioID == 0 || req.TemplateID == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": "scenario_id and template_id are required",
			})
			return
		}

		authenticatedSession, ok := middleware.AuthenticatedSessionFromContext(r.Context())
		if !ok || authenticatedSession.UserID == 0 {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": "authenticated session missing from request context",
			})
			return
		}

		session, err := practiceService.CreatePracticeSession(r.Context(), service.CreatePracticeSessionInput{
			UserID:     authenticatedSession.UserID,
			ScenarioID: req.ScenarioID,
			TemplateID: req.TemplateID,
		})
		if err != nil {
			writeJSON(w, statusForCreatePracticeSessionError(err), map[string]any{
				"error": err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"session": newPracticeSessionResponse(session),
		})
	}
}

func newPracticeSessionResponse(session domain.PracticeSession) practiceSessionResponse {
	return practiceSessionResponse{
		ID:             session.ID,
		UserID:         session.UserID,
		ScenarioID:     session.ScenarioID,
		TemplateID:     session.TemplateID,
		RunnerRef:      session.RunnerRef,
		WorkspacePath:  session.WorkspacePathRef,
		Status:         session.Status,
		StartedAt:      session.StartedAt,
		EndedAt:        session.EndedAt,
		ExpiresAt:      session.ExpiresAt,
		LastActivityAt: session.LastActivityAt,
	}
}

func statusForCreatePracticeSessionError(err error) int {
	switch {
	case errors.Is(err, service.ErrInvalidPracticeSessionInput), errors.Is(err, service.ErrUnknownPracticeTemplate):
		return http.StatusBadRequest
	case errors.Is(err, service.ErrPracticeServiceConfiguration):
		return http.StatusInternalServerError
	case errors.Is(err, service.ErrRunnerWorkspaceCreation):
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}
