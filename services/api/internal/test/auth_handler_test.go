package test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	httpx "gitgym/services/api/internal/http"
)

func TestGitHubLoginRouteIsMountedAsStub(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github/login", nil)
	rec := httptest.NewRecorder()

	httpx.NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", rec.Code)
	}
}

func TestGitHubCallbackRouteIsMountedAsStub(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github/callback", nil)
	rec := httptest.NewRecorder()

	httpx.NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", rec.Code)
	}
}

func TestAuthMeRequiresRealSession(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	rec := httptest.NewRecorder()

	httpx.NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuthMeReturnsPlaceholderBodyWithSessionCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "session-token"})
	rec := httptest.NewRecorder()

	httpx.NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "session ok" {
		t.Fatalf("expected placeholder body, got %q", rec.Body.String())
	}
}

func TestLogoutRequiresRealSession(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	rec := httptest.NewRecorder()

	httpx.NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestLogoutReturnsStubResponseWithSessionCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "session-token"})
	rec := httptest.NewRecorder()

	httpx.NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", rec.Code)
	}
}
