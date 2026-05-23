package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"gitgym/services/api/internal/http/middleware"
	"gitgym/services/api/internal/service"
	"github.com/go-chi/chi/v5"
)

func GetPracticeSessionRepoState(practiceService service.PracticeService) http.HandlerFunc {
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

		repoState, err := practiceService.PracticeSessionRepoState(r.Context(), authenticatedSession.UserID, sessionID)
		if err != nil {
			if errors.Is(err, service.ErrRunnerRepoStateUnavailable) {
				writeJSON(w, http.StatusBadGateway, map[string]any{
					"error": err.Error(),
				})
				return
			}
			writeJSON(w, statusForPracticeSessionLookupError(err), map[string]any{
				"error": err.Error(),
			})
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
