package test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	httpx "gitgym/services/api/internal/http"
)

func TestAuthMeRequiresSession(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	rec := httptest.NewRecorder()

	httpx.NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
