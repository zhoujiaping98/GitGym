package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"gitgym/services/api/internal/service"
	"github.com/go-chi/chi/v5"
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

func RequeueExhaustedWorkspaceCleanupJob(practiceService service.PracticeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jobID, err := strconv.ParseUint(chi.URLParam(r, "jobId"), 10, 64)
		if err != nil || jobID == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": "invalid job id",
			})
			return
		}

		if err := practiceService.RequeueExhaustedWorkspaceCleanupJob(r.Context(), jobID); err != nil {
			status := http.StatusInternalServerError
			switch {
			case errors.Is(err, service.ErrWorkspaceCleanupJobNotFound):
				status = http.StatusNotFound
			case errors.Is(err, service.ErrWorkspaceCleanupJobNotExhausted):
				status = http.StatusConflict
			}
			writeJSON(w, status, map[string]any{
				"error": err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusAccepted, map[string]any{
			"job_id": jobID,
			"status": "pending",
		})
	}
}
