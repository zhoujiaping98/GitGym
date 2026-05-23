package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"gitgym/services/api/internal/http/middleware"
	"gitgym/services/api/internal/runner"
	"gitgym/services/api/internal/service"
	"github.com/go-chi/chi/v5"
)

func GetPracticeSessionRepoState(practiceService service.PracticeService, runnerClient runner.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authenticatedSession, ok := middleware.AuthenticatedSessionFromContext(r.Context())
		if !ok || authenticatedSession.UserID == 0 {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": "authenticated session missing from request context",
			})
			return
		}

		sessionID, err := strconv.ParseUint(chi.URLParam(r, "sessionId"), 10, 64)
		if err != nil || sessionID == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": "invalid session id",
			})
			return
		}

		session, err := practiceService.PracticeSessionByID(r.Context(), authenticatedSession.UserID, sessionID)
		if err != nil {
			writeJSON(w, statusForPracticeSessionLookupError(err), map[string]any{
				"error": err.Error(),
			})
			return
		}

		repoState, err := runnerClient.GetRepoState(r.Context(), session.RunnerRef)
		if err != nil {
			switch {
			case errors.Is(err, runner.ErrWorkspaceNotFound):
				writeJSON(w, http.StatusGone, map[string]any{
					"error": "current session workspace is unavailable",
				})
			case errors.Is(err, runner.ErrClientNotConfigured):
				writeJSON(w, http.StatusInternalServerError, map[string]any{
					"error": err.Error(),
				})
			default:
				writeJSON(w, http.StatusBadGateway, map[string]any{
					"error": "unable to load repository state",
				})
			}
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"data": map[string]any{
				"branch":        repoState.BranchName,
				"head_commit":   repoState.HeadCommit,
				"dirty":         len(repoState.StatusSummary) > 0,
				"changed_files": repoState.StatusSummary,
				"captured_at":   repoState.CapturedAt.UTC(),
			},
		})
	}
}
