package handlers

import (
	"net/http"
	"strconv"

	"gitgym/services/api/internal/service"
)

const defaultWorkspaceCleanupOperatorLimit = 20

func ListExhaustedWorkspaceCleanupJobs(practiceService service.PracticeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		limit := defaultWorkspaceCleanupOperatorLimit
		if rawLimit := r.URL.Query().Get("limit"); rawLimit != "" {
			parsedLimit, err := strconv.Atoi(rawLimit)
			if err != nil || parsedLimit <= 0 {
				writeJSON(w, http.StatusBadRequest, map[string]any{
					"error": "invalid limit",
				})
				return
			}
			limit = parsedLimit
		}

		jobs, err := practiceService.ListExhaustedWorkspaceCleanupJobs(r.Context(), limit)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": err.Error(),
			})
			return
		}

		data := make([]map[string]any, 0, len(jobs))
		for _, job := range jobs {
			data = append(data, map[string]any{
				"id":                  job.ID,
				"practice_session_id": job.PracticeSessionID,
				"workspace_id":        job.WorkspaceID,
				"reason":              job.Reason,
				"status":              job.Status,
				"attempt_count":       job.AttemptCount,
				"last_error":          job.LastError,
				"scheduled_at":        job.ScheduledAt.UTC(),
				"created_at":          job.CreatedAt.UTC(),
				"updated_at":          job.UpdatedAt.UTC(),
			})
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"data": data,
		})
	}
}
