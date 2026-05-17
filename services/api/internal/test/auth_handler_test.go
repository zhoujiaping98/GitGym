package test

import (
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	httpx "gitgym/services/api/internal/http"
	"gitgym/services/api/internal/service"
	"gitgym/services/api/internal/store"
	mysql "github.com/go-sql-driver/mysql"
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

func TestNewSessionTokenReturnsHexAndStableHash(t *testing.T) {
	firstToken, err := service.NewSessionToken()
	if err != nil {
		t.Fatalf("expected token, got error: %v", err)
	}
	secondToken, err := service.NewSessionToken()
	if err != nil {
		t.Fatalf("expected token, got error: %v", err)
	}

	if len(firstToken) != 64 {
		t.Fatalf("expected 64-char token, got %d", len(firstToken))
	}
	if len(secondToken) != 64 {
		t.Fatalf("expected 64-char token, got %d", len(secondToken))
	}
	if firstToken == secondToken {
		t.Fatalf("expected unique tokens, got %q", firstToken)
	}

	firstHash := service.HashSessionToken(firstToken)
	secondHash := service.HashSessionToken(firstToken)
	if firstHash == "" {
		t.Fatalf("expected non-empty hash")
	}
	if firstHash != secondHash {
		t.Fatalf("expected deterministic hash, got %q and %q", firstHash, secondHash)
	}
}

func TestBrowserSessionLookupQueryRequiresUnrevokedAndUnexpiredSession(t *testing.T) {
	query := store.BrowserSessionLookupQuery()

	if !strings.Contains(query, "revoked_at IS NULL") {
		t.Fatalf("expected revoked guard in query, got %q", query)
	}
	if !strings.Contains(query, "expires_at > UTC_TIMESTAMP(6)") {
		t.Fatalf("expected expiry guard in query, got %q", query)
	}
}

func TestBrowserSessionLookupErrorMapsNoRowsToStableNotFound(t *testing.T) {
	err := store.MapBrowserSessionLookupError(sql.ErrNoRows)

	if !errors.Is(err, service.ErrBrowserSessionNotFound) {
		t.Fatalf("expected browser session not found, got %v", err)
	}
}

func TestNormalizeMySQLDSNEnablesParseTimeAndUTC(t *testing.T) {
	normalized, err := store.NormalizeMySQLDSN("user:pass@tcp(localhost:3306)/gitgym")
	if err != nil {
		t.Fatalf("expected normalized dsn, got error: %v", err)
	}

	cfg, err := mysql.ParseDSN(normalized)
	if err != nil {
		t.Fatalf("expected parseable dsn, got error: %v", err)
	}
	if !cfg.ParseTime {
		t.Fatalf("expected ParseTime=true in %q", normalized)
	}
	if cfg.Loc != time.UTC {
		t.Fatalf("expected UTC location, got %v", cfg.Loc)
	}
}
