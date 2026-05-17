package test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httpx "gitgym/services/api/internal/http"
)

func TestGitHubLoginRedirectsToGitHub(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github/login", nil)
	rec := httptest.NewRecorder()

	httpx.NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307, got %d", rec.Code)
	}
	location := rec.Header().Get("Location")
	if !strings.Contains(location, "github.com/login/oauth/authorize") {
		t.Fatalf("expected GitHub authorize redirect, got %q", location)
	}
	if !strings.Contains(location, "client_id=") || !strings.Contains(location, "state=") {
		t.Fatalf("expected client_id and state in redirect, got %q", location)
	}
	if len(rec.Result().Cookies()) == 0 {
		t.Fatalf("expected oauth state cookie to be set")
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

func TestLogoutRequiresRealSession(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	rec := httptest.NewRecorder()

	httpx.NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
