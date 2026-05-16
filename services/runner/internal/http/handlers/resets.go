package handlers

import "net/http"

func ResetWorkspace() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"resetting"}`))
	}
}
