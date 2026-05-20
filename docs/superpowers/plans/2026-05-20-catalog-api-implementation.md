# Catalog API Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace hardcoded practice catalog data with a MySQL-backed catalog plus built-in fallback, then switch the browser to scenario-driven session creation.

**Architecture:** The API service gets an explicit catalog provider boundary. `practiceService` resolves scenarios and templates through that provider instead of owning embedded slices. Router assembly selects either a MySQL-backed provider or a built-in fallback provider. The browser loads catalog data during authenticated bootstrap, waits for catalog readiness before auto-creating a session, and creates sessions by sending `scenario_id` only.

**Tech Stack:** Go, Chi HTTP handlers, MySQL store, React, TypeScript, Vitest, Playwright.

---

## File Map

- Create: `services/api/internal/service/practice_catalog.go`
  - catalog types, provider interface, built-in fallback provider
- Modify: `services/api/internal/service/practice_service.go`
  - remove embedded hardcoded template/scenario slices and resolve session creation through the catalog provider
- Modify: `services/api/internal/store/mysql.go`
  - add MySQL catalog read queries and methods
- Modify: `services/api/internal/http/router.go`
  - choose MySQL-backed catalog provider when available, otherwise fallback provider
- Modify: `services/api/internal/http/handlers/templates.go`
  - return catalog payload with `templates` and `scenarios`
- Modify: `services/api/internal/http/handlers/practice_sessions.go`
  - accept `scenario_id` only on session creation
- Modify: `services/api/internal/test/practice_service_test.go`
  - service-level catalog resolution tests
- Modify: `services/api/internal/test/practice_routes_test.go`
  - route and payload contract tests for catalog list + create session
- Modify: `apps/web/src/lib/api.ts`
  - add catalog fetcher and change create-session payload to `scenario_id` only
- Modify: `apps/web/src/types.ts`
  - add catalog/scenario/template types
- Modify: `apps/web/src/App.tsx`
  - load catalog, block auto-create until catalog is ready, use default scenario
- Modify: `apps/web/src/test/App.test.tsx`
  - app-level catalog loading and error-state tests
- Modify: `apps/web/tests/e2e/smoke.spec.ts`
  - update create-session mocks and smoke expectations for scenario-driven creation

## Task 1: Add the catalog provider boundary and fallback catalog

**Files:**
- Create: `services/api/internal/service/practice_catalog.go`
- Modify: `services/api/internal/service/practice_service.go`
- Test: `services/api/internal/test/practice_service_test.go`

- [ ] **Step 1: Write the failing service test for fallback catalog listing**

Add this test to `services/api/internal/test/practice_service_test.go`:

```go
func TestPracticeServiceListsFallbackCatalog(t *testing.T) {
	t.Parallel()

	svc := service.NewPracticeService(
		service.NewInMemoryPracticeSessionStore(),
		&stubRunnerClient{},
		service.NewFallbackPracticeCatalog(),
		time.Now,
	)

	templates := svc.ListTemplates(context.Background())
	scenarios := svc.ListScenarios(context.Background())

	if len(templates) != 1 {
		t.Fatalf("expected one fallback template, got %d", len(templates))
	}
	if templates[0].Key != "standard" {
		t.Fatalf("expected fallback template key %q, got %q", "standard", templates[0].Key)
	}
	if len(scenarios) != 1 {
		t.Fatalf("expected one fallback scenario, got %d", len(scenarios))
	}
	if scenarios[0].Key != "sandbox-standard" {
		t.Fatalf("expected fallback scenario key %q, got %q", "sandbox-standard", scenarios[0].Key)
	}
	if scenarios[0].TemplateID != templates[0].ID {
		t.Fatalf("expected fallback scenario to reference template %d, got %d", templates[0].ID, scenarios[0].TemplateID)
	}
}
```

- [ ] **Step 2: Run the focused service test to verify it fails**

Run: `go test ./internal/test -run "TestPracticeServiceListsFallbackCatalog" -v`

Workdir: `services/api`

Expected: FAIL because `NewFallbackPracticeCatalog`, `ListScenarios`, or the expanded `NewPracticeService` signature do not exist yet.

- [ ] **Step 3: Write the failing session-creation resolution test**

Add this test to `services/api/internal/test/practice_service_test.go`:

```go
func TestPracticeServiceResolvesTemplateFromScenarioCatalog(t *testing.T) {
	t.Parallel()

	store := &stubPracticeSessionStore{}
	runnerClient := &stubRunnerClient{
		workspace: runner.Workspace{
			ID:       "ws-catalog-create",
			Path:     "/tmp/ws-catalog-create",
			Template: "standard",
		},
	}
	catalog := service.NewStaticPracticeCatalog(
		[]service.PracticeTemplate{
			{ID: 7, Key: "standard", Name: "Standard"},
		},
		[]service.PracticeScenario{
			{ID: 11, Key: "sandbox-standard", Name: "Standard Sandbox", TemplateID: 7},
		},
	)
	svc := service.NewPracticeService(store, runnerClient, catalog, time.Now)

	session, err := svc.CreatePracticeSession(context.Background(), service.CreatePracticeSessionInput{
		UserID:     42,
		ScenarioID: 11,
	})
	if err != nil {
		t.Fatalf("create practice session: %v", err)
	}

	if runnerClient.lastTemplate != "standard" {
		t.Fatalf("expected runner template %q, got %q", "standard", runnerClient.lastTemplate)
	}
	if session.TemplateID != 7 {
		t.Fatalf("expected resolved template ID %d, got %d", 7, session.TemplateID)
	}
	if session.ScenarioID != 11 {
		t.Fatalf("expected scenario ID %d, got %d", 11, session.ScenarioID)
	}
}
```

- [ ] **Step 4: Run the focused resolution test to verify it fails**

Run: `go test ./internal/test -run "TestPracticeServiceResolvesTemplateFromScenarioCatalog" -v`

Workdir: `services/api`

Expected: FAIL because `CreatePracticeSessionInput` still requires `TemplateID`, and the service still resolves templates from embedded slices.

- [ ] **Step 5: Implement the catalog provider boundary and fallback catalog**

Add `services/api/internal/service/practice_catalog.go` with:

```go
package service

import (
	"context"
	"fmt"
)

type PracticeCatalog interface {
	ListTemplates(ctx context.Context) ([]PracticeTemplate, error)
	ListScenarios(ctx context.Context) ([]PracticeScenario, error)
	TemplateByID(ctx context.Context, templateID uint64) (PracticeTemplate, error)
	ScenarioByID(ctx context.Context, scenarioID uint64) (PracticeScenario, error)
}

type staticPracticeCatalog struct {
	templates []PracticeTemplate
	scenarios []PracticeScenario
}

func NewFallbackPracticeCatalog() PracticeCatalog {
	return NewStaticPracticeCatalog(
		[]PracticeTemplate{
			{ID: 1, Key: "standard", Name: "Standard"},
		},
		[]PracticeScenario{
			{ID: 1, Key: "sandbox-standard", Name: "Standard Sandbox", TemplateID: 1},
		},
	)
}

func NewStaticPracticeCatalog(templates []PracticeTemplate, scenarios []PracticeScenario) PracticeCatalog {
	templateCopy := append([]PracticeTemplate(nil), templates...)
	scenarioCopy := append([]PracticeScenario(nil), scenarios...)
	return staticPracticeCatalog{
		templates: templateCopy,
		scenarios: scenarioCopy,
	}
}

func (c staticPracticeCatalog) ListTemplates(context.Context) ([]PracticeTemplate, error) {
	return append([]PracticeTemplate(nil), c.templates...), nil
}

func (c staticPracticeCatalog) ListScenarios(context.Context) ([]PracticeScenario, error) {
	return append([]PracticeScenario(nil), c.scenarios...), nil
}

func (c staticPracticeCatalog) TemplateByID(_ context.Context, templateID uint64) (PracticeTemplate, error) {
	for _, template := range c.templates {
		if template.ID == templateID {
			return template, nil
		}
	}
	return PracticeTemplate{}, fmt.Errorf("%w: %d", ErrUnknownPracticeTemplate, templateID)
}

func (c staticPracticeCatalog) ScenarioByID(_ context.Context, scenarioID uint64) (PracticeScenario, error) {
	for _, scenario := range c.scenarios {
		if scenario.ID == scenarioID {
			return scenario, nil
		}
	}
	return PracticeScenario{}, fmt.Errorf("%w: %d", ErrUnknownPracticeScenario, scenarioID)
}
```

Update `services/api/internal/service/practice_service.go` so the service is constructed as:

```go
type PracticeService interface {
	ListTemplates(ctx context.Context) []PracticeTemplate
	ListScenarios(ctx context.Context) []PracticeScenario
	CreatePracticeSession(ctx context.Context, input CreatePracticeSessionInput) (domain.PracticeSession, error)
	// existing methods unchanged...
}

type practiceService struct {
	store   PracticeSessionStore
	runner  runner.Client
	catalog PracticeCatalog
	now     func() time.Time
	// existing lifecycle fields unchanged...
}

func NewPracticeService(
	store PracticeSessionStore,
	runnerClient runner.Client,
	catalog PracticeCatalog,
	now func() time.Time,
) PracticeService {
	if now == nil {
		now = time.Now
	}
	if catalog == nil {
		catalog = NewFallbackPracticeCatalog()
	}
	return &practiceService{
		store:   store,
		runner:  runnerClient,
		catalog: catalog,
		now:     now,
		cleanup: make(map[string]workspaceCleanupSchedule),
	}
}
```

Change `CreatePracticeSessionInput` to:

```go
type CreatePracticeSessionInput struct {
	UserID     uint64
	ScenarioID uint64
}
```

Change session creation logic to:

```go
func (s *practiceService) ListTemplates(ctx context.Context) []PracticeTemplate {
	templates, err := s.catalog.ListTemplates(ctx)
	if err != nil {
		return nil
	}
	return templates
}

func (s *practiceService) ListScenarios(ctx context.Context) []PracticeScenario {
	scenarios, err := s.catalog.ListScenarios(ctx)
	if err != nil {
		return nil
	}
	return scenarios
}

func (s *practiceService) CreatePracticeSession(ctx context.Context, input CreatePracticeSessionInput) (domain.PracticeSession, error) {
	if input.UserID == 0 || input.ScenarioID == 0 {
		return domain.PracticeSession{}, fmt.Errorf("%w", ErrInvalidPracticeSessionInput)
	}
	if s.catalog == nil {
		return domain.PracticeSession{}, fmt.Errorf("%w: practice catalog is not configured", ErrPracticeServiceConfiguration)
	}
	if s.runner == nil {
		return domain.PracticeSession{}, fmt.Errorf("%w: runner client is not configured", ErrPracticeServiceConfiguration)
	}
	if s.store == nil {
		return domain.PracticeSession{}, fmt.Errorf("%w: practice session store is not configured", ErrPracticeServiceConfiguration)
	}

	scenario, err := s.catalog.ScenarioByID(ctx, input.ScenarioID)
	if err != nil {
		return domain.PracticeSession{}, err
	}
	template, err := s.catalog.TemplateByID(ctx, scenario.TemplateID)
	if err != nil {
		return domain.PracticeSession{}, fmt.Errorf("%w: scenario %d references template %d", ErrPracticeServiceConfiguration, scenario.ID, scenario.TemplateID)
	}

	workspace, err := s.runner.CreateWorkspace(ctx, template.Key)
	if err != nil {
		if errors.Is(err, runner.ErrClientNotConfigured) {
			return domain.PracticeSession{}, fmt.Errorf("%w: %v", ErrPracticeServiceConfiguration, err)
		}
		return domain.PracticeSession{}, fmt.Errorf("%w: %v", ErrRunnerWorkspaceCreation, err)
	}

	now := s.now().UTC()
	session := domain.PracticeSession{
		UserID:           input.UserID,
		ScenarioID:       scenario.ID,
		TemplateID:       template.ID,
		RunnerRef:        workspace.ID,
		WorkspacePathRef: workspace.Path,
		Status:           PracticeSessionStatusActive,
		StartedAt:        now,
		ExpiresAt:        now.Add(practiceSessionTTL),
		LastActivityAt:   now,
	}

	created, err := s.store.CreatePracticeSession(ctx, session)
	if err != nil {
		return domain.PracticeSession{}, fmt.Errorf("create practice session: %w", err)
	}
	return created, nil
}
```

- [ ] **Step 6: Run the focused service tests and verify they pass**

Run: `go test ./internal/test -run "TestPracticeServiceListsFallbackCatalog|TestPracticeServiceResolvesTemplateFromScenarioCatalog" -v`

Workdir: `services/api`

Expected: PASS

- [ ] **Step 7: Commit the catalog-boundary slice**

```bash
git add services/api/internal/service/practice_catalog.go services/api/internal/service/practice_service.go services/api/internal/test/practice_service_test.go
git commit -m "feat: add practice catalog provider boundary"
```

## Task 2: Add MySQL-backed catalog reads

**Files:**
- Modify: `services/api/internal/store/mysql.go`
- Test: `services/api/internal/test/practice_routes_test.go`

- [ ] **Step 1: Write the failing route test for MySQL-backed catalog exposure**

Add this test to `services/api/internal/test/practice_routes_test.go`:

```go
func TestListPracticeCatalogReturnsMySQLBackedTemplatesAndScenarios(t *testing.T) {
	t.Parallel()

	router := httpx.NewRouter(httpx.Dependencies{
		PracticeService: service.NewPracticeService(
			service.NewInMemoryPracticeSessionStore(),
			&stubRunnerClient{},
			service.NewStaticPracticeCatalog(
				[]service.PracticeTemplate{
					{ID: 2, Key: "recovery", Name: "Recovery"},
				},
				[]service.PracticeScenario{
					{ID: 8, Key: "recover-branch", Name: "Recover Branch", TemplateID: 2},
				},
			),
			time.Now,
		),
		AuthStore: authStoreWithSession("catalog-token", 42),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/templates", nil)
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "catalog-token"})
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Templates []struct {
			ID  uint64 `json:"id"`
			Key string `json:"key"`
		} `json:"templates"`
		Scenarios []struct {
			ID         uint64 `json:"id"`
			Key        string `json:"key"`
			TemplateID uint64 `json:"template_id"`
		} `json:"scenarios"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal catalog payload: %v", err)
	}
	if len(payload.Templates) != 1 || payload.Templates[0].Key != "recovery" {
		t.Fatalf("unexpected templates payload: %+v", payload.Templates)
	}
	if len(payload.Scenarios) != 1 || payload.Scenarios[0].Key != "recover-branch" {
		t.Fatalf("unexpected scenarios payload: %+v", payload.Scenarios)
	}
}
```

- [ ] **Step 2: Run the focused route test to verify it fails**

Run: `go test ./internal/test -run "TestListPracticeCatalogReturnsMySQLBackedTemplatesAndScenarios" -v`

Workdir: `services/api`

Expected: FAIL because `/api/v1/templates` currently returns only `templates`.

- [ ] **Step 3: Add MySQL catalog read methods**

In `services/api/internal/store/mysql.go`, add:

```go
const (
	listPracticeTemplatesQuery = `
SELECT id, template_key, display_name
FROM workspace_templates
WHERE is_active = 1
ORDER BY id ASC
`
	listPracticeScenariosQuery = `
SELECT id, scenario_key, display_name, template_id
FROM scenarios
WHERE is_active = 1
ORDER BY id ASC
`
)
```

Add methods:

```go
func (s *MySQLStore) ListPracticeTemplates(ctx context.Context) ([]service.PracticeTemplate, error) {
	rows, err := s.db.QueryContext(ctx, listPracticeTemplatesQuery)
	if err != nil {
		return nil, fmt.Errorf("list practice templates: %w", err)
	}
	defer rows.Close()

	var templates []service.PracticeTemplate
	for rows.Next() {
		var template service.PracticeTemplate
		if err := rows.Scan(&template.ID, &template.Key, &template.Name); err != nil {
			return nil, fmt.Errorf("scan practice template: %w", err)
		}
		templates = append(templates, template)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate practice templates: %w", err)
	}
	return templates, nil
}

func (s *MySQLStore) ListPracticeScenarios(ctx context.Context) ([]service.PracticeScenario, error) {
	rows, err := s.db.QueryContext(ctx, listPracticeScenariosQuery)
	if err != nil {
		return nil, fmt.Errorf("list practice scenarios: %w", err)
	}
	defer rows.Close()

	var scenarios []service.PracticeScenario
	for rows.Next() {
		var scenario service.PracticeScenario
		if err := rows.Scan(&scenario.ID, &scenario.Key, &scenario.Name, &scenario.TemplateID); err != nil {
			return nil, fmt.Errorf("scan practice scenario: %w", err)
		}
		scenarios = append(scenarios, scenario)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate practice scenarios: %w", err)
	}
	return scenarios, nil
}
```

- [ ] **Step 4: Run the existing store-backed API tests to verify no regressions**

Run: `go test ./internal/test -run "TestCurrentPracticeSessionSurvivesRouterRebuildWhenStoreIsPersistent|TestPracticeRoutesMatchPlanSurface" -v`

Workdir: `services/api`

Expected: PASS

- [ ] **Step 5: Commit the MySQL catalog-read slice**

```bash
git add services/api/internal/store/mysql.go services/api/internal/test/practice_routes_test.go
git commit -m "feat: add mysql practice catalog reads"
```

## Task 3: Update API routes to expose catalog and accept `scenario_id` only

**Files:**
- Modify: `services/api/internal/http/handlers/templates.go`
- Modify: `services/api/internal/http/handlers/practice_sessions.go`
- Modify: `services/api/internal/http/router.go`
- Modify: `services/api/internal/test/practice_routes_test.go`

- [ ] **Step 1: Write the failing route test for scenario-only session creation**

Add this test to `services/api/internal/test/practice_routes_test.go`:

```go
func TestCreatePracticeSessionAcceptsScenarioIDOnly(t *testing.T) {
	t.Parallel()

	recordingService := &stubPracticeService{}
	router := httpx.NewRouter(httpx.Dependencies{
		PracticeService: recordingService,
		AuthStore:       authStoreWithSession("scenario-only-token", 42),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/practice-sessions", strings.NewReader(`{"scenario_id":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "scenario-only-token"})
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", rec.Code, rec.Body.String())
	}
	if recordingService.lastCreateInput.ScenarioID != 1 {
		t.Fatalf("expected scenario ID 1, got %d", recordingService.lastCreateInput.ScenarioID)
	}
}
```

- [ ] **Step 2: Run the focused route test to verify it fails**

Run: `go test ./internal/test -run "TestCreatePracticeSessionAcceptsScenarioIDOnly" -v`

Workdir: `services/api`

Expected: FAIL because the handler still requires `template_id`.

- [ ] **Step 3: Change catalog and create-session handlers**

Update `services/api/internal/http/handlers/templates.go`:

```go
type practiceCatalogResponse struct {
	Templates []service.PracticeTemplate `json:"templates"`
	Scenarios []service.PracticeScenario `json:"scenarios"`
}

func ListPracticeTemplates(practiceService service.PracticeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, practiceCatalogResponse{
			Templates: practiceService.ListTemplates(r.Context()),
			Scenarios: practiceService.ListScenarios(r.Context()),
		})
	}
}
```

Update `services/api/internal/http/handlers/practice_sessions.go`:

```go
type createPracticeSessionRequest struct {
	ScenarioID uint64 `json:"scenario_id"`
}

func CreatePracticeSession(practiceService service.PracticeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createPracticeSessionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": "invalid JSON body",
			})
			return
		}
		if req.ScenarioID == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"error": "scenario_id is required",
			})
			return
		}

		authenticatedSession, ok := middleware.AuthenticatedSessionFromContext(r.Context())
		if !ok || authenticatedSession.UserID == 0 {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error": "authenticated session missing from request context",
			})
			return
		}

		session, err := practiceService.CreatePracticeSession(r.Context(), service.CreatePracticeSessionInput{
			UserID:     authenticatedSession.UserID,
			ScenarioID: req.ScenarioID,
		})
		if err != nil {
			writeJSON(w, statusForCreatePracticeSessionError(err), map[string]any{
				"error": err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"session": newPracticeSessionResponse(session),
		})
	}
}
```

Update the stub service in `practice_routes_test.go` so its default `CreatePracticeSession` response sets `ScenarioID` and `TemplateID`.

- [ ] **Step 4: Run the focused route tests and verify they pass**

Run: `go test ./internal/test -run "TestCreatePracticeSessionAcceptsScenarioIDOnly|TestListPracticeCatalogReturnsMySQLBackedTemplatesAndScenarios|TestCreatePracticeSessionUsesAuthenticatedUserAndReturnsStableJSON" -v`

Workdir: `services/api`

Expected: PASS

- [ ] **Step 5: Commit the API-contract slice**

```bash
git add services/api/internal/http/handlers/templates.go services/api/internal/http/handlers/practice_sessions.go services/api/internal/http/router.go services/api/internal/test/practice_routes_test.go
git commit -m "feat: expose practice catalog and scenario-only session creation"
```

## Task 4: Wire router fallback and MySQL-backed catalog provider selection

**Files:**
- Modify: `services/api/internal/http/router.go`
- Modify: `services/api/internal/store/mysql.go`
- Test: `services/api/internal/test/practice_routes_test.go`

- [ ] **Step 1: Write the failing fallback-catalog router test**

Add this test to `services/api/internal/test/practice_routes_test.go`:

```go
func TestRouterUsesFallbackCatalogWhenNoCatalogStoreIsAvailable(t *testing.T) {
	t.Parallel()

	router := httpx.NewRouter(httpx.Dependencies{
		AuthStore: authStoreWithSession("fallback-catalog-token", 42),
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/templates", nil)
	req.AddCookie(&http.Cookie{Name: "gitgym_session", Value: "fallback-catalog-token"})
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Templates []struct {
			Key string `json:"key"`
		} `json:"templates"`
		Scenarios []struct {
			Key string `json:"key"`
		} `json:"scenarios"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal fallback catalog payload: %v", err)
	}
	if len(payload.Templates) != 1 || payload.Templates[0].Key != "standard" {
		t.Fatalf("unexpected fallback templates: %+v", payload.Templates)
	}
	if len(payload.Scenarios) != 1 || payload.Scenarios[0].Key != "sandbox-standard" {
		t.Fatalf("unexpected fallback scenarios: %+v", payload.Scenarios)
	}
}
```

- [ ] **Step 2: Run the fallback router test to verify it fails**

Run: `go test ./internal/test -run "TestRouterUsesFallbackCatalogWhenNoCatalogStoreIsAvailable" -v`

Workdir: `services/api`

Expected: FAIL because router assembly does not yet explicitly choose between store-backed and fallback catalogs.

- [ ] **Step 3: Add provider selection in router assembly**

In `services/api/internal/http/router.go`, add:

```go
type practiceCatalogStore interface {
	ListPracticeTemplates(ctx context.Context) ([]service.PracticeTemplate, error)
	ListPracticeScenarios(ctx context.Context) ([]service.PracticeScenario, error)
}

type storeBackedPracticeCatalog struct {
	store practiceCatalogStore
}

func (c storeBackedPracticeCatalog) ListTemplates(ctx context.Context) ([]service.PracticeTemplate, error) {
	return c.store.ListPracticeTemplates(ctx)
}

func (c storeBackedPracticeCatalog) ListScenarios(ctx context.Context) ([]service.PracticeScenario, error) {
	return c.store.ListPracticeScenarios(ctx)
}

func (c storeBackedPracticeCatalog) TemplateByID(ctx context.Context, templateID uint64) (service.PracticeTemplate, error) {
	templates, err := c.store.ListPracticeTemplates(ctx)
	if err != nil {
		return service.PracticeTemplate{}, err
	}
	for _, template := range templates {
		if template.ID == templateID {
			return template, nil
		}
	}
	return service.PracticeTemplate{}, fmt.Errorf("%w: %d", service.ErrUnknownPracticeTemplate, templateID)
}

func (c storeBackedPracticeCatalog) ScenarioByID(ctx context.Context, scenarioID uint64) (service.PracticeScenario, error) {
	scenarios, err := c.store.ListPracticeScenarios(ctx)
	if err != nil {
		return service.PracticeScenario{}, err
	}
	for _, scenario := range scenarios {
		if scenario.ID == scenarioID {
			return scenario, nil
		}
	}
	return service.PracticeScenario{}, fmt.Errorf("%w: %d", service.ErrUnknownPracticeScenario, scenarioID)
}
```

Update practice-service assembly:

```go
func practiceCatalogFromDependencies(dependencies Dependencies) service.PracticeCatalog {
	if dependencies.AuthStore != nil {
		if catalogStore, ok := dependencies.AuthStore.(practiceCatalogStore); ok {
			return storeBackedPracticeCatalog{store: catalogStore}
		}
	}
	return service.NewFallbackPracticeCatalog()
}
```

And pass it into `service.NewPracticeService(...)`.

- [ ] **Step 4: Run the fallback router test and verification routes**

Run: `go test ./internal/test -run "TestRouterUsesFallbackCatalogWhenNoCatalogStoreIsAvailable|TestPracticeRoutesMatchPlanSurface|TestCurrentPracticeSessionSurvivesRouterRebuildWhenStoreIsPersistent" -v`

Workdir: `services/api`

Expected: PASS

- [ ] **Step 5: Commit the router-wiring slice**

```bash
git add services/api/internal/http/router.go services/api/internal/store/mysql.go services/api/internal/test/practice_routes_test.go
git commit -m "feat: wire mysql and fallback practice catalogs"
```

## Task 5: Update frontend API client and app bootstrap for catalog loading

**Files:**
- Modify: `apps/web/src/types.ts`
- Modify: `apps/web/src/lib/api.ts`
- Modify: `apps/web/src/App.tsx`
- Test: `apps/web/src/test/App.test.tsx`

- [ ] **Step 1: Write the failing frontend test for catalog-gated auto-create**

Add this test to `apps/web/src/test/App.test.tsx`:

```tsx
it("waits for catalog before auto-creating a session", async () => {
  const fetchMock = vi.fn()
    .mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          templates: [{ id: 1, key: "standard", name: "Standard" }],
          scenarios: [{ id: 1, key: "sandbox-standard", name: "Standard Sandbox", template_id: 1 }],
        }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    )
    .mockResolvedValueOnce(new Response("", { status: 404 }))
    .mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          session: {
            id: 99,
            user_id: 42,
            scenario_id: 1,
            template_id: 1,
            runner_ref: "ws-99",
            workspace_path: "/tmp/ws-99",
            status: "active",
            started_at: "2026-05-20T12:00:00Z",
            expires_at: "2026-05-20T14:00:00Z",
            last_activity_at: "2026-05-20T12:00:00Z",
          },
        }),
        { status: 201, headers: { "Content-Type": "application/json" } },
      ),
    );

  vi.stubGlobal("fetch", fetchMock);

  render(<App />);

  await waitFor(() => {
    expect(fetchMock).toHaveBeenCalledWith("/api/v1/templates", expect.anything());
  });
});
```

- [ ] **Step 2: Run the focused app test to verify it fails**

Run: `pnpm --dir apps/web test -- --run src/test/App.test.tsx`

Expected: FAIL because the app does not load catalog data yet.

- [ ] **Step 3: Add catalog client types and fetcher**

Update `apps/web/src/types.ts`:

```ts
export type PracticeTemplate = {
  id: number;
  key: string;
  name: string;
};

export type PracticeScenario = {
  id: number;
  key: string;
  name: string;
  templateId: number;
};
```

Update `apps/web/src/lib/api.ts`:

```ts
type CatalogResponse = {
  templates: Array<{ id: number; key: string; name: string }>;
  scenarios: Array<{ id: number; key: string; name: string; template_id: number }>;
};

export type PracticeCatalog = {
  templates: PracticeTemplate[];
  scenarios: PracticeScenario[];
};

export async function fetchPracticeCatalog(signal?: AbortSignal): Promise<PracticeCatalog> {
  const response = await fetch(`${API_BASE}/templates`, {
    credentials: "include",
    headers: {
      Accept: "application/json",
    },
    signal,
  });

  const payload = await readJson<CatalogResponse | { error?: string }>(response);
  if (!response.ok) {
    const message =
      payload.data && "error" in payload.data && payload.data.error
        ? payload.data.error
        : "Request failed";
    throw new ApiError(message, response.status);
  }
  if (!payload.data || !("templates" in payload.data) || !("scenarios" in payload.data)) {
    throw new ApiError("Catalog response was malformed", response.status);
  }

  return {
    templates: payload.data.templates,
    scenarios: payload.data.scenarios.map((scenario) => ({
      id: scenario.id,
      key: scenario.key,
      name: scenario.name,
      templateId: scenario.template_id,
    })),
  };
}
```

Change create-session input and request:

```ts
type CreateSessionInput = {
  scenarioId: number;
};

export async function createPracticeSession(input: CreateSessionInput): Promise<PracticeSession> {
  const response = await fetch(`${API_BASE}/practice-sessions`, {
    method: "POST",
    credentials: "include",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
    },
    body: JSON.stringify({
      scenario_id: input.scenarioId,
    }),
  });
  // existing response handling unchanged...
}
```

- [ ] **Step 4: Update app bootstrap to wait for catalog**

In `apps/web/src/App.tsx`, add state like:

```tsx
type CatalogState =
  | { status: "loading"; catalog: null; error: null }
  | { status: "ready"; catalog: PracticeCatalog; error: null }
  | { status: "error"; catalog: null; error: string };
```

Load catalog on mount:

```tsx
const [catalogState, setCatalogState] = useState<CatalogState>({
  status: "loading",
  catalog: null,
  error: null,
});

useEffect(() => {
  const controller = new AbortController();
  void fetchPracticeCatalog(controller.signal)
    .then((catalog) => {
      setCatalogState({ status: "ready", catalog, error: null });
    })
    .catch((error) => {
      setCatalogState({
        status: "error",
        catalog: null,
        error: error instanceof Error ? error.message : "Unable to load practice catalog.",
      });
    });
  return () => controller.abort();
}, []);
```

Change auto-create gating:

```tsx
const defaultScenario = catalogState.status === "ready" ? catalogState.catalog.scenarios[0] ?? null : null;

useEffect(() => {
  if (!hasAuthenticatedEmptyState) {
    setHasAttemptedAutoCreate(false);
    return;
  }
  if (catalogState.status !== "ready" || !defaultScenario) {
    return;
  }
  if (hasAttemptedAutoCreate || actionError || pendingAction === "new-session") {
    return;
  }

  setHasAttemptedAutoCreate(true);
  startNewSession(defaultScenario.id);
}, [actionError, catalogState.status, defaultScenario, hasAttemptedAutoCreate, hasAuthenticatedEmptyState, pendingAction]);
```

Change `startNewSession` signature:

```tsx
function startNewSession(scenarioId: number) {
  const fallbackSession = displayedSession;
  setActionError(null);
  setPendingAction("new-session");
  void createPracticeSession({ scenarioId })
    .then((nextSession) => {
      if (!fallbackSession) {
        setSessionOverride(nextSession);
      }
      return reconcileSessionAction("new-session", nextSession.id, {
        fallbackSession,
        optimisticSession: nextSession,
      });
    })
    // existing catch/finally unchanged...
}
```

Add catalog error shell:

```tsx
if (catalogState.status === "error" && !hasActiveSession && !signedOutOverride) {
  return (
    <AppStateShell
      eyebrow="Catalog unavailable"
      title="Practice catalog unavailable"
      body="We could not load the available practice scenarios."
      detail={catalogState.error}
      actionLabel="Try again"
      onAction={() => window.location.reload()}
    />
  );
}
```

- [ ] **Step 5: Run the focused frontend tests and verify they pass**

Run: `pnpm --dir apps/web test -- --run src/test/App.test.tsx`

Expected: PASS

- [ ] **Step 6: Commit the frontend catalog-loading slice**

```bash
git add apps/web/src/types.ts apps/web/src/lib/api.ts apps/web/src/App.tsx apps/web/src/test/App.test.tsx
git commit -m "feat: load practice catalog before session creation"
```

## Task 6: Update smoke coverage and finish verification

**Files:**
- Modify: `apps/web/tests/e2e/smoke.spec.ts`
- Test: `services/api`, `services/runner`, `apps/web`

- [ ] **Step 1: Write the failing smoke adjustment**

Update create-session request expectations in `apps/web/tests/e2e/smoke.spec.ts` from:

```ts
expect(body).toEqual({
  scenario_id: 1,
  template_id: 1,
});
```

to:

```ts
expect(body).toEqual({
  scenario_id: 1,
});
```

Add catalog mock responses anywhere the app now needs `/api/v1/templates`.

- [ ] **Step 2: Run the smoke tests to verify they fail for the expected reason**

Run: `pnpm --dir apps/web run test:e2e`

Expected: FAIL because the app or mocks still assume the old create-session payload and no catalog bootstrap.

- [ ] **Step 3: Finish smoke wiring for catalog bootstrap**

In `apps/web/tests/e2e/smoke.spec.ts`, make sure the test server responds to:

```ts
if (request.url().endsWith("/api/v1/templates")) {
  return route.fulfill({
    status: 200,
    contentType: "application/json",
    body: JSON.stringify({
      templates: [{ id: 1, key: "standard", name: "Standard" }],
      scenarios: [{ id: 1, key: "sandbox-standard", name: "Standard Sandbox", template_id: 1 }],
    }),
  });
}
```

And keep create-session assertions scenario-only:

```ts
const body = JSON.parse(request.postData() ?? "{}");
expect(body).toEqual({ scenario_id: 1 });
```

- [ ] **Step 4: Run full verification**

Run: `go test ./...`

Workdir: `services/api`

Expected: PASS

Run: `go test ./...`

Workdir: `services/runner`

Expected: PASS

Run: `pnpm --dir apps/web test`

Expected: PASS

Run: `pnpm --dir apps/web run test:e2e`

Expected: PASS

- [ ] **Step 5: Report the remaining intended limitations**

Close-out checklist:

```text
- GET /api/v1/templates still carries the catalog payload for compatibility
- fallback catalog still contains only the built-in default template and scenario
- scenario selection UI still defaults to the first scenario rather than a richer picker
```

- [ ] **Step 6: Commit the smoke-and-verification slice**

```bash
git add apps/web/tests/e2e/smoke.spec.ts
git commit -m "test: cover scenario-driven catalog session flows"
```
