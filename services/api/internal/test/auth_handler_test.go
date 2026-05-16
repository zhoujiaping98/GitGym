package test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	httpx "gitgym/services/api/internal/http"
)

func TestAuthMeRequiresSessionCookie(t *testing.T) {
	t.Run("returns unauthorized without session cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		rec := httptest.NewRecorder()

		httpx.NewRouter().ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("returns success with non-empty session cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
		req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "session-token"})
		rec := httptest.NewRecorder()

		httpx.NewRouter().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if rec.Body.String() != "session ok" {
			t.Fatalf("expected handler success body, got %q", rec.Body.String())
		}
	})
}
