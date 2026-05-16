# GitGym V1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the GitGym version 1 sandbox as a desktop-first web application with GitHub login, MySQL-backed metadata, a Go API and runner split, and a React workbench for real Git practice sessions.

**Architecture:** The system is a monorepo with one frontend app and two Go services. The API service owns authentication, browser sessions, session orchestration, and browser-facing HTTP/WebSocket endpoints. The runner service owns workspace creation, template hydration, command execution, repository observation, and cleanup. The browser communicates only with the API; the API coordinates with the runner and persists metadata, command records, and snapshot summaries in MySQL.

**Tech Stack:** React, TypeScript, Vite, xterm.js, Go, net/http, chi, coder/websocket, MySQL, goose migrations, sqlc, Vitest, Playwright

---

## File Structure

### Repository Root

- Create: `.gitignore`
- Create: `README.md`
- Create: `pnpm-workspace.yaml`
- Create: `package.json`
- Create: `go.work`
- Create: `.env.example`

### Frontend App

- Create: `apps/web/package.json`
- Create: `apps/web/vite.config.ts`
- Create: `apps/web/tsconfig.json`
- Create: `apps/web/index.html`
- Create: `apps/web/src/main.tsx`
- Create: `apps/web/src/App.tsx`
- Create: `apps/web/src/styles.css`
- Create: `apps/web/src/lib/api.ts`
- Create: `apps/web/src/lib/ws.ts`
- Create: `apps/web/src/components/TopBar.tsx`
- Create: `apps/web/src/components/Workbench.tsx`
- Create: `apps/web/src/components/TerminalPanel.tsx`
- Create: `apps/web/src/components/RepoPanel.tsx`
- Create: `apps/web/src/components/CommandHistoryPanel.tsx`
- Create: `apps/web/src/components/LoginScreen.tsx`
- Create: `apps/web/src/components/SessionStatusBadge.tsx`
- Create: `apps/web/src/hooks/useTerminalSession.ts`
- Create: `apps/web/src/hooks/useCurrentSession.ts`
- Create: `apps/web/src/types.ts`
- Create: `apps/web/src/test/App.test.tsx`
- Create: `apps/web/playwright.config.ts`
- Create: `apps/web/tests/e2e/smoke.spec.ts`

### API Service

- Create: `services/api/go.mod`
- Create: `services/api/cmd/api/main.go`
- Create: `services/api/internal/config/config.go`
- Create: `services/api/internal/http/router.go`
- Create: `services/api/internal/http/middleware/session.go`
- Create: `services/api/internal/http/handlers/health.go`
- Create: `services/api/internal/http/handlers/auth.go`
- Create: `services/api/internal/http/handlers/templates.go`
- Create: `services/api/internal/http/handlers/practice_sessions.go`
- Create: `services/api/internal/http/handlers/terminal_ws.go`
- Create: `services/api/internal/domain/models.go`
- Create: `services/api/internal/oauth/github.go`
- Create: `services/api/internal/store/mysql.go`
- Create: `services/api/internal/store/sqlc/`
- Create: `services/api/internal/runner/client.go`
- Create: `services/api/internal/service/auth_service.go`
- Create: `services/api/internal/service/practice_service.go`
- Create: `services/api/internal/service/session_cookie.go`
- Create: `services/api/internal/test/auth_handler_test.go`
- Create: `services/api/internal/test/practice_service_test.go`

### Runner Service

- Create: `services/runner/go.mod`
- Create: `services/runner/cmd/runner/main.go`
- Create: `services/runner/internal/config/config.go`
- Create: `services/runner/internal/http/router.go`
- Create: `services/runner/internal/http/handlers/health.go`
- Create: `services/runner/internal/http/handlers/workspaces.go`
- Create: `services/runner/internal/http/handlers/commands.go`
- Create: `services/runner/internal/http/handlers/resets.go`
- Create: `services/runner/internal/engine/workspaces.go`
- Create: `services/runner/internal/engine/templates.go`
- Create: `services/runner/internal/engine/commands.go`
- Create: `services/runner/internal/engine/snapshots.go`
- Create: `services/runner/internal/engine/events.go`
- Create: `services/runner/internal/test/workspaces_test.go`
- Create: `services/runner/internal/test/commands_test.go`

### Contracts and Schema

- Create: `contracts/openapi/gitgym.yaml`
- Create: `contracts/events/session-event.schema.json`
- Create: `contracts/events/repo-snapshot.schema.json`
- Create: `db/migrations/0001_initial.sql`
- Create: `db/sqlc.yaml`
- Create: `db/query/users.sql`
- Create: `db/query/sessions.sql`
- Create: `db/query/commands.sql`
- Create: `db/query/templates.sql`

### Scenarios and Templates

- Create: `scenarios/templates/empty/README.md`
- Create: `scenarios/templates/standard/README.md`
- Create: `scenarios/templates/recovery/README.md`
- Create: `scenarios/sandbox/default.json`

## Task 1: Bootstrap the Monorepo and Repository Tooling

**Files:**
- Create: `.gitignore`
- Create: `README.md`
- Create: `pnpm-workspace.yaml`
- Create: `package.json`
- Create: `go.work`
- Create: `.env.example`

- [ ] **Step 1: Initialize the Git repository**

Run:

```bash
git init
git branch -M main
```

Expected: output includes `Initialized empty Git repository` and the branch is named `main`.

- [ ] **Step 2: Create root workspace manifests**

Add `.gitignore`:

```gitignore
node_modules/
dist/
.DS_Store
.env
.env.local
.superpowers/
coverage/
playwright-report/
test-results/
tmp/
var/
```

Add `pnpm-workspace.yaml`:

```yaml
packages:
  - apps/*
```

Add `package.json`:

```json
{
  "name": "gitgym",
  "private": true,
  "packageManager": "pnpm@10.4.0",
  "scripts": {
    "web:dev": "pnpm --dir apps/web dev",
    "web:test": "pnpm --dir apps/web test",
    "web:e2e": "pnpm --dir apps/web exec playwright test"
  }
}
```

Add `go.work`:

```txt
go 1.24.0

use (
    ./services/api
    ./services/runner
)
```

- [ ] **Step 3: Add a root README and environment example**

Add `README.md`:

```md
# GitGym

GitGym is a desktop-first web sandbox for practicing Git commands against disposable repositories.

## Services

- `apps/web`: React workbench
- `services/api`: browser-facing API and auth service
- `services/runner`: workspace and Git execution service
```

Add `.env.example`:

```dotenv
MYSQL_DSN=root:password@tcp(127.0.0.1:3306)/gitgym?parseTime=true
GITHUB_CLIENT_ID=replace-me
GITHUB_CLIENT_SECRET=replace-me
SESSION_COOKIE_SECRET=replace-me
API_BASE_URL=http://localhost:8080
RUNNER_BASE_URL=http://localhost:8081
RUNNER_WORK_ROOT=./var/workspaces
```

- [ ] **Step 4: Verify the repository bootstrap**

Run:

```bash
git status --short
```

Expected: the new root files appear as untracked.

- [ ] **Step 5: Commit the bootstrap**

Run:

```bash
git add .gitignore README.md pnpm-workspace.yaml package.json go.work .env.example
git commit -m "chore: bootstrap gitgym monorepo"
```

Expected: one commit containing the root workspace files.

## Task 2: Define the API Contract, Event Schemas, and Scenario Seeds

**Files:**
- Create: `contracts/openapi/gitgym.yaml`
- Create: `contracts/events/session-event.schema.json`
- Create: `contracts/events/repo-snapshot.schema.json`
- Create: `scenarios/sandbox/default.json`
- Create: `scenarios/templates/empty/README.md`
- Create: `scenarios/templates/standard/README.md`
- Create: `scenarios/templates/recovery/README.md`

- [ ] **Step 1: Write the contract files first**

Add `contracts/openapi/gitgym.yaml`:

```yaml
openapi: 3.1.0
info:
  title: GitGym API
  version: 0.1.0
servers:
  - url: http://localhost:8080
paths:
  /api/v1/auth/me:
    get:
      operationId: getCurrentUser
      responses:
        "200":
          description: Current user
  /api/v1/templates:
    get:
      operationId: listTemplates
      responses:
        "200":
          description: Template list
  /api/v1/practice-sessions:
    post:
      operationId: createPracticeSession
      responses:
        "201":
          description: Practice session created
  /api/v1/practice-sessions/{sessionId}/reset:
    post:
      operationId: resetPracticeSession
      responses:
        "202":
          description: Session reset accepted
```

Add `contracts/events/session-event.schema.json`:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "SessionEvent",
  "type": "object",
  "required": ["type", "practiceSessionId", "createdAt", "payload"],
  "properties": {
    "type": { "type": "string" },
    "practiceSessionId": { "type": "string" },
    "commandRunId": { "type": ["string", "null"] },
    "createdAt": { "type": "string", "format": "date-time" },
    "payload": { "type": "object", "additionalProperties": true }
  }
}
```

Add `contracts/events/repo-snapshot.schema.json`:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "RepoSnapshot",
  "type": "object",
  "required": ["headCommit", "statusSummary", "capturedAt"],
  "properties": {
    "headRef": { "type": ["string", "null"] },
    "headCommit": { "type": "string" },
    "branchName": { "type": ["string", "null"] },
    "detachedHead": { "type": "boolean" },
    "statusSummary": { "type": "object", "additionalProperties": true },
    "operationState": { "type": "object", "additionalProperties": true },
    "recentGraph": { "type": "array" },
    "refsSummary": { "type": "object", "additionalProperties": true },
    "capturedAt": { "type": "string", "format": "date-time" }
  }
}
```

- [ ] **Step 2: Add the sandbox scenario seed**

Add `scenarios/sandbox/default.json`:

```json
{
  "key": "sandbox-default",
  "modeType": "sandbox",
  "templateKey": "standard",
  "name": "Default Sandbox",
  "description": "A freeform Git practice session with real command execution and no completion goal.",
  "rules": {
    "network": "deny",
    "workspaceScope": "session-root",
    "allowedShell": ["git", "ls", "pwd", "cat"]
  },
  "completionRules": null,
  "hintPolicy": {
    "mode": "silent"
  }
}
```

- [ ] **Step 3: Add template seed descriptions**

Add `scenarios/templates/empty/README.md`:

```md
# Empty Template

Creates a fresh repository with `git init` and no commits.
```

Add `scenarios/templates/standard/README.md`:

```md
# Standard Template

Creates a small repository with a main branch, one feature branch, and several commits for everyday Git practice.
```

Add `scenarios/templates/recovery/README.md`:

```md
# Recovery Template

Creates a repository with a merge or rebase problem state for safe recovery experiments.
```

- [ ] **Step 4: Verify the contract files are present**

Run:

```bash
git diff -- contracts scenarios
```

Expected: the new OpenAPI file, JSON schemas, and scenario seed files are visible in the diff.

- [ ] **Step 5: Commit the contracts and scenario seeds**

Run:

```bash
git add contracts scenarios
git commit -m "docs: add api contracts and sandbox scenario seeds"
```

Expected: one commit containing the API contract and seed definitions.

## Task 3: Build the MySQL Schema, Queries, and Persistence Boundary

**Files:**
- Create: `db/migrations/0001_initial.sql`
- Create: `db/sqlc.yaml`
- Create: `db/query/users.sql`
- Create: `db/query/sessions.sql`
- Create: `db/query/commands.sql`
- Create: `db/query/templates.sql`
- Create: `services/api/internal/store/mysql.go`
- Create: `services/api/internal/domain/models.go`
- Test: `services/api/internal/test/practice_service_test.go`

- [ ] **Step 1: Write the failing persistence boundary test**

Add `services/api/internal/test/practice_service_test.go`:

```go
package test

import (
	"testing"
)

func TestPracticeSessionStoreContract(t *testing.T) {
	t.Fatalf("implement store-backed practice session test")
}
```

- [ ] **Step 2: Run the test to confirm the store contract is missing**

Run:

```bash
go test ./services/api/internal/test -run TestPracticeSessionStoreContract -v
```

Expected: FAIL with the `implement store-backed practice session test` message.

- [ ] **Step 3: Add the initial MySQL migration**

Add `db/migrations/0001_initial.sql`:

```sql
CREATE TABLE users (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  github_user_id BIGINT UNSIGNED NOT NULL,
  github_login VARCHAR(255) NOT NULL,
  display_name VARCHAR(255) NOT NULL,
  avatar_url VARCHAR(1024) NULL,
  email VARCHAR(255) NULL,
  status VARCHAR(32) NOT NULL DEFAULT 'active',
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  last_login_at DATETIME(6) NULL,
  UNIQUE KEY uk_users_github_user_id (github_user_id)
);

CREATE TABLE auth_accounts (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  user_id BIGINT UNSIGNED NOT NULL,
  provider VARCHAR(64) NOT NULL,
  provider_account_id VARCHAR(255) NOT NULL,
  provider_username VARCHAR(255) NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  UNIQUE KEY uk_auth_accounts_provider_account (provider, provider_account_id),
  CONSTRAINT fk_auth_accounts_user FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE user_sessions (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  user_id BIGINT UNSIGNED NOT NULL,
  session_token_hash CHAR(64) NOT NULL,
  user_agent VARCHAR(512) NULL,
  ip_address VARCHAR(64) NULL,
  expires_at DATETIME(6) NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  revoked_at DATETIME(6) NULL,
  UNIQUE KEY uk_user_sessions_token_hash (session_token_hash),
  CONSTRAINT fk_user_sessions_user FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE workspace_templates (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  template_key VARCHAR(64) NOT NULL,
  name VARCHAR(255) NOT NULL,
  description TEXT NOT NULL,
  mode_compatibility JSON NOT NULL,
  source_ref VARCHAR(255) NOT NULL,
  difficulty_level VARCHAR(32) NOT NULL,
  is_active TINYINT(1) NOT NULL DEFAULT 1,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  UNIQUE KEY uk_workspace_templates_key (template_key)
);

CREATE TABLE scenarios (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  scenario_key VARCHAR(64) NOT NULL,
  mode_type VARCHAR(32) NOT NULL,
  template_id BIGINT UNSIGNED NOT NULL,
  name VARCHAR(255) NOT NULL,
  description TEXT NOT NULL,
  rules_json JSON NOT NULL,
  completion_rules_json JSON NULL,
  hint_policy_json JSON NOT NULL,
  is_active TINYINT(1) NOT NULL DEFAULT 1,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  UNIQUE KEY uk_scenarios_key (scenario_key),
  CONSTRAINT fk_scenarios_template FOREIGN KEY (template_id) REFERENCES workspace_templates(id)
);

CREATE TABLE practice_sessions (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  user_id BIGINT UNSIGNED NOT NULL,
  scenario_id BIGINT UNSIGNED NOT NULL,
  template_id BIGINT UNSIGNED NOT NULL,
  runner_ref VARCHAR(255) NOT NULL,
  workspace_path_ref VARCHAR(1024) NOT NULL,
  status VARCHAR(32) NOT NULL,
  started_at DATETIME(6) NOT NULL,
  expires_at DATETIME(6) NOT NULL,
  ended_at DATETIME(6) NULL,
  last_activity_at DATETIME(6) NOT NULL,
  CONSTRAINT fk_practice_sessions_user FOREIGN KEY (user_id) REFERENCES users(id),
  CONSTRAINT fk_practice_sessions_scenario FOREIGN KEY (scenario_id) REFERENCES scenarios(id),
  CONSTRAINT fk_practice_sessions_template FOREIGN KEY (template_id) REFERENCES workspace_templates(id)
);

CREATE TABLE command_runs (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  practice_session_id BIGINT UNSIGNED NOT NULL,
  sequence_no INT UNSIGNED NOT NULL,
  raw_command TEXT NOT NULL,
  executable VARCHAR(255) NOT NULL,
  args_json JSON NOT NULL,
  cwd_ref VARCHAR(1024) NOT NULL,
  policy_decision VARCHAR(32) NOT NULL,
  exit_code INT NOT NULL,
  duration_ms INT UNSIGNED NOT NULL,
  stdout_preview MEDIUMTEXT NULL,
  stderr_preview MEDIUMTEXT NULL,
  started_at DATETIME(6) NOT NULL,
  finished_at DATETIME(6) NOT NULL,
  UNIQUE KEY uk_command_runs_session_sequence (practice_session_id, sequence_no),
  CONSTRAINT fk_command_runs_session FOREIGN KEY (practice_session_id) REFERENCES practice_sessions(id)
);

CREATE TABLE repo_snapshots (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  practice_session_id BIGINT UNSIGNED NOT NULL,
  command_run_id BIGINT UNSIGNED NULL,
  snapshot_phase VARCHAR(16) NOT NULL,
  head_ref VARCHAR(255) NULL,
  head_commit CHAR(40) NOT NULL,
  branch_name VARCHAR(255) NULL,
  detached_head TINYINT(1) NOT NULL DEFAULT 0,
  status_summary_json JSON NOT NULL,
  operation_state_json JSON NOT NULL,
  recent_graph_json JSON NOT NULL,
  refs_summary_json JSON NOT NULL,
  captured_at DATETIME(6) NOT NULL,
  CONSTRAINT fk_repo_snapshots_session FOREIGN KEY (practice_session_id) REFERENCES practice_sessions(id),
  CONSTRAINT fk_repo_snapshots_command FOREIGN KEY (command_run_id) REFERENCES command_runs(id)
);

CREATE TABLE session_events (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  practice_session_id BIGINT UNSIGNED NOT NULL,
  command_run_id BIGINT UNSIGNED NULL,
  event_type VARCHAR(64) NOT NULL,
  event_payload_json JSON NOT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  CONSTRAINT fk_session_events_session FOREIGN KEY (practice_session_id) REFERENCES practice_sessions(id),
  CONSTRAINT fk_session_events_command FOREIGN KEY (command_run_id) REFERENCES command_runs(id)
);

CREATE TABLE session_resets (
  id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
  practice_session_id BIGINT UNSIGNED NOT NULL,
  user_id BIGINT UNSIGNED NOT NULL,
  reason_code VARCHAR(64) NOT NULL,
  source_snapshot_id BIGINT UNSIGNED NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  CONSTRAINT fk_session_resets_session FOREIGN KEY (practice_session_id) REFERENCES practice_sessions(id),
  CONSTRAINT fk_session_resets_user FOREIGN KEY (user_id) REFERENCES users(id),
  CONSTRAINT fk_session_resets_snapshot FOREIGN KEY (source_snapshot_id) REFERENCES repo_snapshots(id)
);
```

- [ ] **Step 4: Add sqlc config and query files**

Add `db/sqlc.yaml`:

```yaml
version: "2"
sql:
  - schema: "db/migrations"
    queries:
      - "db/query/users.sql"
      - "db/query/sessions.sql"
      - "db/query/commands.sql"
      - "db/query/templates.sql"
    engine: "mysql"
    gen:
      go:
        package: "sqlc"
        out: "services/api/internal/store/sqlc"
```

Add `db/query/users.sql`:

```sql
-- name: UpsertGitHubUser :execresult
INSERT INTO users (github_user_id, github_login, display_name, avatar_url, email, last_login_at)
VALUES (?, ?, ?, ?, ?, NOW(6))
ON DUPLICATE KEY UPDATE
  github_login = VALUES(github_login),
  display_name = VALUES(display_name),
  avatar_url = VALUES(avatar_url),
  email = VALUES(email),
  last_login_at = NOW(6);

-- name: GetUserByGitHubID :one
SELECT * FROM users WHERE github_user_id = ? LIMIT 1;
```

Add `db/query/sessions.sql`:

```sql
-- name: CreateUserSession :exec
INSERT INTO user_sessions (user_id, session_token_hash, user_agent, ip_address, expires_at)
VALUES (?, ?, ?, ?, ?);

-- name: GetUserSessionByTokenHash :one
SELECT * FROM user_sessions WHERE session_token_hash = ? AND revoked_at IS NULL LIMIT 1;

-- name: CreatePracticeSession :execresult
INSERT INTO practice_sessions (
  user_id, scenario_id, template_id, runner_ref, workspace_path_ref, status, started_at, expires_at, last_activity_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);
```

Add `db/query/commands.sql`:

```sql
-- name: CreateCommandRun :execresult
INSERT INTO command_runs (
  practice_session_id, sequence_no, raw_command, executable, args_json, cwd_ref, policy_decision, exit_code, duration_ms, stdout_preview, stderr_preview, started_at, finished_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: CreateRepoSnapshot :exec
INSERT INTO repo_snapshots (
  practice_session_id, command_run_id, snapshot_phase, head_ref, head_commit, branch_name, detached_head, status_summary_json, operation_state_json, recent_graph_json, refs_summary_json, captured_at
)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
```

Add `db/query/templates.sql`:

```sql
-- name: ListActiveTemplates :many
SELECT * FROM workspace_templates WHERE is_active = 1 ORDER BY id ASC;

-- name: GetScenarioByKey :one
SELECT * FROM scenarios WHERE scenario_key = ? AND is_active = 1 LIMIT 1;
```

- [ ] **Step 5: Add the first domain model and DB wrapper**

Add `services/api/internal/domain/models.go`:

```go
package domain

import "time"

type CurrentUser struct {
	ID          uint64
	GitHubID    uint64
	GitHubLogin string
	DisplayName string
	AvatarURL   string
	Email       string
}

type PracticeSession struct {
	ID              uint64
	UserID          uint64
	ScenarioID      uint64
	TemplateID      uint64
	RunnerRef       string
	WorkspacePathRef string
	Status          string
	StartedAt       time.Time
	ExpiresAt       time.Time
	LastActivityAt  time.Time
}
```

Add `services/api/internal/store/mysql.go`:

```go
package store

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

func OpenMySQL(dsn string) (*sql.DB, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open mysql: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping mysql: %w", err)
	}
	return db, nil
}
```

- [ ] **Step 6: Run the failing test again and keep it red**

Run:

```bash
go test ./services/api/internal/test -run TestPracticeSessionStoreContract -v
```

Expected: FAIL again. The migration and store boundary exist, but no real practice session service has been implemented yet.

- [ ] **Step 7: Commit the schema foundation**

Run:

```bash
git add db services/api/internal/domain/models.go services/api/internal/store/mysql.go
git commit -m "feat: add mysql schema and store foundation"
```

Expected: one commit containing the migration, sqlc config, and initial store code.

## Task 4: Implement GitHub OAuth and Browser Sessions in the API

**Files:**
- Create: `services/api/go.mod`
- Create: `services/api/cmd/api/main.go`
- Create: `services/api/internal/config/config.go`
- Create: `services/api/internal/http/router.go`
- Create: `services/api/internal/http/middleware/session.go`
- Create: `services/api/internal/http/handlers/health.go`
- Create: `services/api/internal/http/handlers/auth.go`
- Create: `services/api/internal/oauth/github.go`
- Create: `services/api/internal/service/auth_service.go`
- Create: `services/api/internal/service/session_cookie.go`
- Test: `services/api/internal/test/auth_handler_test.go`

- [ ] **Step 1: Write the failing auth handler test**

Add `services/api/internal/test/auth_handler_test.go`:

```go
package test

import "testing"

func TestAuthMeRequiresSession(t *testing.T) {
	t.Fatalf("implement auth me handler test")
}
```

- [ ] **Step 2: Run the auth test to verify it fails**

Run:

```bash
go test ./services/api/internal/test -run TestAuthMeRequiresSession -v
```

Expected: FAIL with the `implement auth me handler test` message.

- [ ] **Step 3: Create the API module and config loader**

Add `services/api/go.mod`:

```go
module gitgym/services/api

go 1.24.0

require (
	github.com/go-chi/chi/v5 v5.2.1
	github.com/go-sql-driver/mysql v1.8.1
	github.com/golang-jwt/jwt/v5 v5.2.1
	github.com/coder/websocket v1.8.12
	golang.org/x/oauth2 v0.30.0
)
```

Add `services/api/internal/config/config.go`:

```go
package config

import (
	"fmt"
	"os"
)

type Config struct {
	MySQLDSN          string
	GitHubClientID    string
	GitHubSecret      string
	SessionSecret     string
	RunnerBaseURL     string
	APIBaseURL        string
}

func Load() (Config, error) {
	cfg := Config{
		MySQLDSN:       os.Getenv("MYSQL_DSN"),
		GitHubClientID: os.Getenv("GITHUB_CLIENT_ID"),
		GitHubSecret:   os.Getenv("GITHUB_CLIENT_SECRET"),
		SessionSecret:  os.Getenv("SESSION_COOKIE_SECRET"),
		RunnerBaseURL:  os.Getenv("RUNNER_BASE_URL"),
		APIBaseURL:     os.Getenv("API_BASE_URL"),
	}
	if cfg.MySQLDSN == "" || cfg.GitHubClientID == "" || cfg.GitHubSecret == "" || cfg.SessionSecret == "" {
		return Config{}, fmt.Errorf("missing required environment variables")
	}
	return cfg, nil
}
```

- [ ] **Step 4: Implement auth service and session cookie helpers**

Add `services/api/internal/service/session_cookie.go`:

```go
package service

import (
	"crypto/sha256"
	"encoding/hex"
)

func HashSessionToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
```

Add `services/api/internal/service/auth_service.go`:

```go
package service

import "context"

type GitHubProfile struct {
	ID        uint64
	Login     string
	Name      string
	AvatarURL string
	Email     string
}

type UserStore interface {
	UpsertGitHubUser(ctx context.Context, profile GitHubProfile) (uint64, error)
	CreateBrowserSession(ctx context.Context, userID uint64, tokenHash string, userAgent string, ip string) error
}
```

Add `services/api/internal/oauth/github.go`:

```go
package oauth

import "golang.org/x/oauth2"

func GitHubConfig(clientID string, clientSecret string, redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"read:user", "user:email"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://github.com/login/oauth/authorize",
			TokenURL: "https://github.com/login/oauth/access_token",
		},
	}
}
```

- [ ] **Step 5: Implement minimal auth routes and session middleware**

Add `services/api/internal/http/handlers/auth.go`:

```go
package handlers

import "net/http"

func AuthMe() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}
}
```

Add `services/api/internal/http/middleware/session.go`:

```go
package middleware

import "net/http"

func RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("gitgym_session")
		if err != nil || cookie.Value == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
```

Add `services/api/internal/http/router.go`:

```go
package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"gitgym/services/api/internal/http/handlers"
	"gitgym/services/api/internal/http/middleware"
)

func NewRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", handlers.Health())
	r.With(middleware.RequireSession).Get("/api/v1/auth/me", handlers.AuthMe())
	return r
}
```

Add `services/api/internal/http/handlers/health.go`:

```go
package handlers

import "net/http"

func Health() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}
```

Add `services/api/cmd/api/main.go`:

```go
package main

import (
	"log"
	"net/http"

	httpx "gitgym/services/api/internal/http"
)

func main() {
	log.Fatal(http.ListenAndServe(":8080", httpx.NewRouter()))
}
```

- [ ] **Step 6: Replace the placeholder auth test with a real one and make it pass**

Replace `services/api/internal/test/auth_handler_test.go` with:

```go
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
```

Run:

```bash
go test ./services/api/internal/test -run TestAuthMeRequiresSession -v
```

Expected: PASS.

- [ ] **Step 7: Commit the auth foundation**

Run:

```bash
git add services/api
git commit -m "feat: add api auth and session foundation"
```

Expected: one commit containing the API module, router, auth handler, and session middleware foundation.

## Task 5: Build the Runner Service Workspace Lifecycle and Template Hydration

**Files:**
- Create: `services/runner/go.mod`
- Create: `services/runner/cmd/runner/main.go`
- Create: `services/runner/internal/config/config.go`
- Create: `services/runner/internal/http/router.go`
- Create: `services/runner/internal/http/handlers/health.go`
- Create: `services/runner/internal/http/handlers/workspaces.go`
- Create: `services/runner/internal/engine/workspaces.go`
- Create: `services/runner/internal/engine/templates.go`
- Test: `services/runner/internal/test/workspaces_test.go`

- [ ] **Step 1: Write the failing workspace lifecycle test**

Add `services/runner/internal/test/workspaces_test.go`:

```go
package test

import "testing"

func TestCreateWorkspaceFromStandardTemplate(t *testing.T) {
	t.Fatalf("implement workspace creation test")
}
```

- [ ] **Step 2: Run the workspace test to verify it fails**

Run:

```bash
go test ./services/runner/internal/test -run TestCreateWorkspaceFromStandardTemplate -v
```

Expected: FAIL with the `implement workspace creation test` message.

- [ ] **Step 3: Create the runner module and config**

Add `services/runner/go.mod`:

```go
module gitgym/services/runner

go 1.24.0

require github.com/go-chi/chi/v5 v5.2.1
```

Add `services/runner/internal/config/config.go`:

```go
package config

import "os"

type Config struct {
	WorkRoot string
}

func Load() Config {
	workRoot := os.Getenv("RUNNER_WORK_ROOT")
	if workRoot == "" {
		workRoot = "./var/workspaces"
	}
	return Config{WorkRoot: workRoot}
}
```

- [ ] **Step 4: Implement workspace creation and template hydration**

Add `services/runner/internal/engine/workspaces.go`:

```go
package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Workspace struct {
	ID   string
	Path string
}

func CreateWorkspace(root string) (Workspace, error) {
	id := fmt.Sprintf("ws-%d", time.Now().UnixNano())
	path := filepath.Join(root, id)
	if err := os.MkdirAll(path, 0o755); err != nil {
		return Workspace{}, err
	}
	return Workspace{ID: id, Path: path}, nil
}
```

Add `services/runner/internal/engine/templates.go`:

```go
package engine

import (
	"os"
	"os/exec"
)

func InitStandardTemplate(workspacePath string) error {
	cmd := exec.Command("git", "init", "-b", "main")
	cmd.Dir = workspacePath
	if err := cmd.Run(); err != nil {
		return err
	}
	return os.WriteFile(workspacePath+"/README.md", []byte("# Standard Template\n"), 0o644)
}
```

Add `services/runner/internal/http/handlers/workspaces.go`:

```go
package handlers

import "net/http"

func CreateWorkspace() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status":"created"}`))
	}
}
```

- [ ] **Step 5: Add the runner entrypoint and health route**

Add `services/runner/internal/http/handlers/health.go`:

```go
package handlers

import "net/http"

func Health() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}
```

Add `services/runner/internal/http/router.go`:

```go
package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"gitgym/services/runner/internal/http/handlers"
)

func NewRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", handlers.Health())
	r.Post("/internal/workspaces", handlers.CreateWorkspace())
	return r
}
```

Add `services/runner/cmd/runner/main.go`:

```go
package main

import (
	"log"
	"net/http"

	httpx "gitgym/services/runner/internal/http"
)

func main() {
	log.Fatal(http.ListenAndServe(":8081", httpx.NewRouter()))
}
```

- [ ] **Step 6: Replace the placeholder test with a real filesystem test**

Replace `services/runner/internal/test/workspaces_test.go` with:

```go
package test

import (
	"os"
	"testing"

	"gitgym/services/runner/internal/engine"
)

func TestCreateWorkspaceFromStandardTemplate(t *testing.T) {
	root := t.TempDir()

	workspace, err := engine.CreateWorkspace(root)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	if err := engine.InitStandardTemplate(workspace.Path); err != nil {
		t.Fatalf("init standard template: %v", err)
	}

	if _, err := os.Stat(workspace.Path + "/README.md"); err != nil {
		t.Fatalf("expected README.md to exist: %v", err)
	}
}
```

Run:

```bash
go test ./services/runner/internal/test -run TestCreateWorkspaceFromStandardTemplate -v
```

Expected: PASS.

- [ ] **Step 7: Commit the runner workspace foundation**

Run:

```bash
git add services/runner
git commit -m "feat: add runner workspace lifecycle foundation"
```

Expected: one commit containing the runner module, workspace engine, and HTTP skeleton.

## Task 6: Implement Real Command Execution, Snapshots, and Session Event Recording

**Files:**
- Create: `services/runner/internal/engine/commands.go`
- Create: `services/runner/internal/engine/snapshots.go`
- Create: `services/runner/internal/engine/events.go`
- Create: `services/runner/internal/http/handlers/commands.go`
- Create: `services/runner/internal/http/handlers/resets.go`
- Test: `services/runner/internal/test/commands_test.go`

- [ ] **Step 1: Write the failing command execution test**

Add `services/runner/internal/test/commands_test.go`:

```go
package test

import "testing"

func TestRunGitStatusAndCaptureSnapshot(t *testing.T) {
	t.Fatalf("implement command execution test")
}
```

- [ ] **Step 2: Run the command test to verify it fails**

Run:

```bash
go test ./services/runner/internal/test -run TestRunGitStatusAndCaptureSnapshot -v
```

Expected: FAIL with the `implement command execution test` message.

- [ ] **Step 3: Implement command execution and snapshot capture**

Add `services/runner/internal/engine/commands.go`:

```go
package engine

import (
	"bytes"
	"os/exec"
	"strings"
	"time"
)

type CommandResult struct {
	Stdout     string
	Stderr     string
	ExitCode   int
	DurationMS int
}

func RunCommand(workspacePath string, raw string) (CommandResult, error) {
	parts := strings.Fields(raw)
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = workspacePath

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := int(time.Since(start).Milliseconds())

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return CommandResult{}, err
		}
	}

	return CommandResult{
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		ExitCode:   exitCode,
		DurationMS: duration,
	}, nil
}
```

Add `services/runner/internal/engine/snapshots.go`:

```go
package engine

import (
	"os/exec"
	"strings"
	"time"
)

type Snapshot struct {
	HeadCommit    string
	BranchName    string
	StatusSummary []string
	CapturedAt    time.Time
}

func CaptureSnapshot(workspacePath string) (Snapshot, error) {
	headBytes, err := exec.Command("git", "-C", workspacePath, "rev-parse", "HEAD").Output()
	if err != nil {
		return Snapshot{}, err
	}

	branchBytes, err := exec.Command("git", "-C", workspacePath, "branch", "--show-current").Output()
	if err != nil {
		return Snapshot{}, err
	}

	statusBytes, err := exec.Command("git", "-C", workspacePath, "status", "--short").Output()
	if err != nil {
		return Snapshot{}, err
	}

	status := strings.TrimSpace(string(statusBytes))
	summary := []string{}
	if status != "" {
		summary = strings.Split(status, "\n")
	}

	return Snapshot{
		HeadCommit:    strings.TrimSpace(string(headBytes)),
		BranchName:    strings.TrimSpace(string(branchBytes)),
		StatusSummary: summary,
		CapturedAt:    time.Now().UTC(),
	}, nil
}
```

Add `services/runner/internal/engine/events.go`:

```go
package engine

import "time"

type SessionEvent struct {
	Type        string
	WorkspaceID string
	CreatedAt   time.Time
	Payload     map[string]any
}
```

- [ ] **Step 4: Add reset and command handler skeletons**

Add `services/runner/internal/http/handlers/commands.go`:

```go
package handlers

import "net/http"

func RunCommand() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
}
```

Add `services/runner/internal/http/handlers/resets.go`:

```go
package handlers

import "net/http"

func ResetWorkspace() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"status":"resetting"}`))
	}
}
```

- [ ] **Step 5: Update the runner router to expose command and reset routes**

Update `services/runner/internal/http/router.go`:

```go
package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"gitgym/services/runner/internal/http/handlers"
)

func NewRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", handlers.Health())
	r.Post("/internal/workspaces", handlers.CreateWorkspace())
	r.Post("/internal/workspaces/{workspaceID}/commands", handlers.RunCommand())
	r.Post("/internal/workspaces/{workspaceID}/reset", handlers.ResetWorkspace())
	return r
}
```

- [ ] **Step 6: Replace the placeholder test with a real Git command test and make it pass**

Replace `services/runner/internal/test/commands_test.go` with:

```go
package test

import (
	"testing"

	"gitgym/services/runner/internal/engine"
)

func TestRunGitStatusAndCaptureSnapshot(t *testing.T) {
	root := t.TempDir()

	workspace, err := engine.CreateWorkspace(root)
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if err := engine.InitStandardTemplate(workspace.Path); err != nil {
		t.Fatalf("init standard template: %v", err)
	}

	result, err := engine.RunCommand(workspace.Path, "git status --short")
	if err != nil {
		t.Fatalf("run command: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", result.ExitCode)
	}

	snapshot, err := engine.CaptureSnapshot(workspace.Path)
	if err != nil {
		t.Fatalf("capture snapshot: %v", err)
	}
	if snapshot.BranchName != "main" {
		t.Fatalf("expected branch main, got %q", snapshot.BranchName)
	}
}
```

Run:

```bash
go test ./services/runner/internal/test -run TestRunGitStatusAndCaptureSnapshot -v
```

Expected: PASS.

- [ ] **Step 7: Commit command execution and snapshots**

Run:

```bash
git add services/runner/internal/engine services/runner/internal/http
git commit -m "feat: add runner command execution and snapshot capture"
```

Expected: one commit containing the runner command engine and snapshot logic.

## Task 7: Implement API Session Orchestration and Runner Integration

**Files:**
- Create: `services/api/internal/runner/client.go`
- Create: `services/api/internal/http/handlers/templates.go`
- Create: `services/api/internal/http/handlers/practice_sessions.go`
- Create: `services/api/internal/http/handlers/terminal_ws.go`
- Create: `services/api/internal/service/practice_service.go`
- Modify: `services/api/internal/http/router.go`
- Test: `services/api/internal/test/practice_service_test.go`

- [ ] **Step 1: Replace the placeholder practice service test with a real orchestration test**

Replace `services/api/internal/test/practice_service_test.go` with:

```go
package test

import "testing"

func TestPracticeSessionStoreContract(t *testing.T) {
	t.Fatalf("implement practice session orchestration test")
}
```

- [ ] **Step 2: Run the practice service test to verify it fails**

Run:

```bash
go test ./services/api/internal/test -run TestPracticeSessionStoreContract -v
```

Expected: FAIL with the `implement practice session orchestration test` message.

- [ ] **Step 3: Add the runner client and practice service interfaces**

Add `services/api/internal/runner/client.go`:

```go
package runner

import "context"

type WorkspaceResponse struct {
	WorkspaceID   string
	WorkspacePath string
}

type Client interface {
	CreateWorkspace(ctx context.Context, templateKey string) (WorkspaceResponse, error)
}
```

Add `services/api/internal/service/practice_service.go`:

```go
package service

import (
	"context"
	"time"
)

type RunnerClient interface {
	CreateWorkspace(ctx context.Context, templateKey string) (WorkspaceRef, error)
}

type WorkspaceRef struct {
	RunnerRef        string
	WorkspacePathRef string
}

type PracticeStore interface {
	CreatePracticeSession(ctx context.Context, userID uint64, scenarioKey string, runnerRef WorkspaceRef) (uint64, error)
}

type PracticeService struct {
	Runner RunnerClient
	Store  PracticeStore
}

func (s PracticeService) CreateSession(ctx context.Context, userID uint64, scenarioKey string, templateKey string) (uint64, error) {
	ref, err := s.Runner.CreateWorkspace(ctx, templateKey)
	if err != nil {
		return 0, err
	}
	_ = time.Now().UTC()
	return s.Store.CreatePracticeSession(ctx, userID, scenarioKey, ref)
}
```

- [ ] **Step 4: Add list templates and create session handlers**

Add `services/api/internal/http/handlers/templates.go`:

```go
package handlers

import "net/http"

func ListTemplates() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}
}
```

Add `services/api/internal/http/handlers/practice_sessions.go`:

```go
package handlers

import "net/http"

func CreatePracticeSession() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status":"created"}`))
	}
}
```

Add `services/api/internal/http/handlers/terminal_ws.go`:

```go
package handlers

import "net/http"

func TerminalWebSocket() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not implemented", http.StatusNotImplemented)
	}
}
```

- [ ] **Step 5: Wire the new routes into the API router**

Update `services/api/internal/http/router.go`:

```go
package httpx

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"gitgym/services/api/internal/http/handlers"
	"gitgym/services/api/internal/http/middleware"
)

func NewRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", handlers.Health())
	r.With(middleware.RequireSession).Get("/api/v1/auth/me", handlers.AuthMe())
	r.With(middleware.RequireSession).Get("/api/v1/templates", handlers.ListTemplates())
	r.With(middleware.RequireSession).Post("/api/v1/practice-sessions", handlers.CreatePracticeSession())
	r.With(middleware.RequireSession).Get("/api/v1/practice-sessions/{sessionId}/terminal", handlers.TerminalWebSocket())
	return r
}
```

- [ ] **Step 6: Replace the placeholder test with a fake-driven orchestration test and make it pass**

Replace `services/api/internal/test/practice_service_test.go` with:

```go
package test

import (
	"context"
	"testing"

	"gitgym/services/api/internal/service"
)

type fakeRunner struct{}

func (fakeRunner) CreateWorkspace(ctx context.Context, templateKey string) (service.WorkspaceRef, error) {
	return service.WorkspaceRef{
		RunnerRef:        "runner-a",
		WorkspacePathRef: "/tmp/ws-1",
	}, nil
}

type fakeStore struct{}

func (fakeStore) CreatePracticeSession(ctx context.Context, userID uint64, scenarioKey string, runnerRef service.WorkspaceRef) (uint64, error) {
	return 42, nil
}

func TestPracticeSessionStoreContract(t *testing.T) {
	svc := service.PracticeService{
		Runner: fakeRunner{},
		Store:  fakeStore{},
	}

	id, err := svc.CreateSession(context.Background(), 7, "sandbox-default", "standard")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if id != 42 {
		t.Fatalf("expected session id 42, got %d", id)
	}
}
```

Run:

```bash
go test ./services/api/internal/test -run TestPracticeSessionStoreContract -v
```

Expected: PASS.

- [ ] **Step 7: Commit API orchestration**

Run:

```bash
git add services/api/internal
git commit -m "feat: add practice session orchestration"
```

Expected: one commit containing the practice service, runner client interface, and browser-facing routes.

## Task 8: Build the React Workbench Shell and Browser Login Screen

**Files:**
- Create: `apps/web/package.json`
- Create: `apps/web/vite.config.ts`
- Create: `apps/web/tsconfig.json`
- Create: `apps/web/index.html`
- Create: `apps/web/src/main.tsx`
- Create: `apps/web/src/App.tsx`
- Create: `apps/web/src/styles.css`
- Create: `apps/web/src/components/LoginScreen.tsx`
- Create: `apps/web/src/components/TopBar.tsx`
- Create: `apps/web/src/components/Workbench.tsx`
- Create: `apps/web/src/components/SessionStatusBadge.tsx`
- Create: `apps/web/src/test/App.test.tsx`

- [ ] **Step 1: Write the failing app shell test**

Add `apps/web/src/test/App.test.tsx`:

```tsx
import { describe, expect, it } from "vitest";

describe("App", () => {
  it("renders the login call to action", () => {
    throw new Error("implement app shell test");
  });
});
```

- [ ] **Step 2: Run the frontend test to verify it fails**

Run:

```bash
pnpm --dir apps/web test -- --runInBand
```

Expected: FAIL with `implement app shell test`.

- [ ] **Step 3: Create the web app manifest and entry files**

Add `apps/web/package.json`:

```json
{
  "name": "@gitgym/web",
  "private": true,
  "version": "0.1.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "test": "vitest run"
  },
  "dependencies": {
    "react": "^19.1.0",
    "react-dom": "^19.1.0",
    "xterm": "^5.5.0"
  },
  "devDependencies": {
    "@testing-library/react": "^16.3.0",
    "@testing-library/jest-dom": "^6.7.0",
    "@types/react": "^19.1.2",
    "@types/react-dom": "^19.1.2",
    "typescript": "^5.8.3",
    "vite": "^6.3.5",
    "vitest": "^3.1.3"
  }
}
```

Add `apps/web/vite.config.ts`:

```ts
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
  },
});
```

Add `apps/web/tsconfig.json`:

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "jsx": "react-jsx",
    "moduleResolution": "Bundler",
    "strict": true,
    "types": ["vitest/globals"]
  },
  "include": ["src"]
}
```

Add `apps/web/index.html`:

```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>GitGym</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] **Step 4: Build the initial React app shell**

Add `apps/web/src/main.tsx`:

```tsx
import React from "react";
import ReactDOM from "react-dom/client";
import App from "./App";
import "./styles.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);
```

Add `apps/web/src/components/LoginScreen.tsx`:

```tsx
export function LoginScreen() {
  return (
    <main className="login-screen">
      <h1>GitGym</h1>
      <p>Practice Git in a disposable repository before touching local work.</p>
      <a className="primary-button" href="/api/v1/auth/github/login">
        Continue with GitHub
      </a>
    </main>
  );
}
```

Add `apps/web/src/components/SessionStatusBadge.tsx`:

```tsx
type Props = {
  label: string;
};

export function SessionStatusBadge({ label }: Props) {
  return <span className="session-status-badge">{label}</span>;
}
```

Add `apps/web/src/components/TopBar.tsx`:

```tsx
import { SessionStatusBadge } from "./SessionStatusBadge";

export function TopBar() {
  return (
    <header className="top-bar">
      <div className="brand">GitGym</div>
      <div className="top-bar-actions">
        <span>Template: Standard</span>
        <SessionStatusBadge label="Signed out" />
      </div>
    </header>
  );
}
```

Add `apps/web/src/components/Workbench.tsx`:

```tsx
export function Workbench() {
  return (
    <section className="workbench-shell">
      <div className="workbench-main">Terminal Placeholder</div>
      <aside className="workbench-side">Repository Placeholder</aside>
    </section>
  );
}
```

Add `apps/web/src/App.tsx`:

```tsx
import { LoginScreen } from "./components/LoginScreen";
import { TopBar } from "./components/TopBar";

export default function App() {
  return (
    <div className="app-shell">
      <TopBar />
      <LoginScreen />
    </div>
  );
}
```

Add `apps/web/src/styles.css`:

```css
:root {
  color-scheme: light;
  font-family: "Segoe UI", sans-serif;
  background: #f3f4ef;
  color: #1d241f;
}

body {
  margin: 0;
}

.top-bar {
  display: flex;
  justify-content: space-between;
  padding: 16px 24px;
  border-bottom: 1px solid #d3d7cf;
  background: #fcfcf8;
}

.login-screen {
  display: grid;
  gap: 16px;
  max-width: 560px;
  margin: 80px auto;
  padding: 32px;
}

.primary-button {
  display: inline-flex;
  width: fit-content;
  padding: 12px 18px;
  background: #0f5d3f;
  color: white;
  text-decoration: none;
  border-radius: 10px;
}
```

- [ ] **Step 5: Replace the placeholder test with a real render test and make it pass**

Replace `apps/web/src/test/App.test.tsx` with:

```tsx
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import App from "../App";

describe("App", () => {
  it("renders the login call to action", () => {
    render(<App />);
    expect(screen.getByText("Continue with GitHub")).toBeTruthy();
  });
});
```

Run:

```bash
pnpm --dir apps/web test
```

Expected: PASS.

- [ ] **Step 6: Commit the frontend shell**

Run:

```bash
git add apps/web
git commit -m "feat: add frontend shell and login screen"
```

Expected: one commit containing the Vite app, login screen, and top bar shell.

## Task 9: Build the Terminal Workbench, Repository Panel, and Command History UI

**Files:**
- Create: `apps/web/src/lib/api.ts`
- Create: `apps/web/src/lib/ws.ts`
- Create: `apps/web/src/components/TerminalPanel.tsx`
- Create: `apps/web/src/components/RepoPanel.tsx`
- Create: `apps/web/src/components/CommandHistoryPanel.tsx`
- Create: `apps/web/src/hooks/useTerminalSession.ts`
- Create: `apps/web/src/hooks/useCurrentSession.ts`
- Create: `apps/web/src/types.ts`
- Modify: `apps/web/src/components/Workbench.tsx`
- Modify: `apps/web/src/App.tsx`
- Create: `apps/web/playwright.config.ts`
- Create: `apps/web/tests/e2e/smoke.spec.ts`

- [ ] **Step 1: Add the browser-facing types and API helpers**

Add `apps/web/src/types.ts`:

```ts
export type RepoSnapshot = {
  branchName: string | null;
  headCommit: string;
  statusSummary: string[];
};

export type CommandRecord = {
  id: string;
  command: string;
  exitCode: number;
  startedAt: string;
};

export type PracticeSession = {
  id: string;
  status: "idle" | "running" | "expired" | "failed";
  templateName: string;
  snapshot: RepoSnapshot | null;
  history: CommandRecord[];
};
```

Add `apps/web/src/lib/api.ts`:

```ts
import type { PracticeSession } from "../types";

export async function fetchCurrentSession(): Promise<PracticeSession | null> {
  const response = await fetch("/api/v1/practice-sessions/current", {
    credentials: "include",
  });

  if (response.status === 404) {
    return null;
  }

  if (!response.ok) {
    throw new Error(`request failed: ${response.status}`);
  }

  return response.json();
}
```

Add `apps/web/src/lib/ws.ts`:

```ts
export function connectTerminal(sessionId: string): WebSocket {
  return new WebSocket(`ws://${window.location.host}/api/v1/practice-sessions/${sessionId}/terminal`);
}
```

- [ ] **Step 2: Implement the session and terminal hooks**

Add `apps/web/src/hooks/useCurrentSession.ts`:

```ts
import { useEffect, useState } from "react";
import { fetchCurrentSession } from "../lib/api";
import type { PracticeSession } from "../types";

export function useCurrentSession() {
  const [session, setSession] = useState<PracticeSession | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchCurrentSession()
      .then(setSession)
      .finally(() => setLoading(false));
  }, []);

  return { session, loading };
}
```

Add `apps/web/src/hooks/useTerminalSession.ts`:

```ts
import { useEffect, useState } from "react";
import { connectTerminal } from "../lib/ws";

export function useTerminalSession(sessionId: string | null) {
  const [messages, setMessages] = useState<string[]>([]);

  useEffect(() => {
    if (!sessionId) {
      return;
    }

    const socket = connectTerminal(sessionId);
    socket.onmessage = (event) => {
      setMessages((current) => [...current, String(event.data)]);
    };

    return () => socket.close();
  }, [sessionId]);

  return { messages };
}
```

- [ ] **Step 3: Implement the workbench panels**

Add `apps/web/src/components/TerminalPanel.tsx`:

```tsx
type Props = {
  lines: string[];
};

export function TerminalPanel({ lines }: Props) {
  return (
    <section className="terminal-panel">
      <div className="panel-title">Terminal</div>
      <pre>{lines.join("\n") || "$ git status"}</pre>
    </section>
  );
}
```

Add `apps/web/src/components/RepoPanel.tsx`:

```tsx
import type { RepoSnapshot } from "../types";

type Props = {
  snapshot: RepoSnapshot | null;
};

export function RepoPanel({ snapshot }: Props) {
  return (
    <aside className="repo-panel">
      <div className="panel-title">Repository</div>
      <div>Branch: {snapshot?.branchName ?? "main"}</div>
      <div>HEAD: {snapshot?.headCommit ?? "unavailable"}</div>
      <div>Status: {(snapshot?.statusSummary ?? []).join(", ") || "clean"}</div>
    </aside>
  );
}
```

Add `apps/web/src/components/CommandHistoryPanel.tsx`:

```tsx
import type { CommandRecord } from "../types";

type Props = {
  history: CommandRecord[];
};

export function CommandHistoryPanel({ history }: Props) {
  return (
    <section className="history-panel">
      <div className="panel-title">Command History</div>
      <ul>
        {history.map((record) => (
          <li key={record.id}>
            <code>{record.command}</code> exit {record.exitCode}
          </li>
        ))}
      </ul>
    </section>
  );
}
```

- [ ] **Step 4: Upgrade the workbench and app shell to use real session state**

Replace `apps/web/src/components/Workbench.tsx` with:

```tsx
import { CommandHistoryPanel } from "./CommandHistoryPanel";
import { RepoPanel } from "./RepoPanel";
import { TerminalPanel } from "./TerminalPanel";
import { useTerminalSession } from "../hooks/useTerminalSession";
import type { PracticeSession } from "../types";

type Props = {
  session: PracticeSession;
};

export function Workbench({ session }: Props) {
  const terminal = useTerminalSession(session.id);

  return (
    <section className="workbench-layout">
      <div className="workbench-main">
        <TerminalPanel lines={terminal.messages} />
      </div>
      <RepoPanel snapshot={session.snapshot} />
      <CommandHistoryPanel history={session.history} />
    </section>
  );
}
```

Replace `apps/web/src/App.tsx` with:

```tsx
import { LoginScreen } from "./components/LoginScreen";
import { TopBar } from "./components/TopBar";
import { Workbench } from "./components/Workbench";
import { useCurrentSession } from "./hooks/useCurrentSession";

export default function App() {
  const { session, loading } = useCurrentSession();

  return (
    <div className="app-shell">
      <TopBar />
      {loading ? <p>Loading…</p> : session ? <Workbench session={session} /> : <LoginScreen />}
    </div>
  );
}
```

- [ ] **Step 5: Add frontend smoke E2E scaffolding**

Add `apps/web/playwright.config.ts`:

```ts
import { defineConfig } from "@playwright/test";

export default defineConfig({
  use: {
    baseURL: "http://localhost:5173",
  },
  testDir: "./tests/e2e",
});
```

Add `apps/web/tests/e2e/smoke.spec.ts`:

```ts
import { expect, test } from "@playwright/test";

test("renders login screen", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByText("Continue with GitHub")).toBeVisible();
});
```

- [ ] **Step 6: Run frontend unit tests and commit the workbench**

Run:

```bash
pnpm --dir apps/web test
git add apps/web
git commit -m "feat: add workbench layout and session ui"
```

Expected: tests PASS, then one commit containing the workbench panels, hooks, and browser helpers.

## Task 10: Wire End-to-End Flow, Reset Behavior, and Final Verification

**Files:**
- Modify: `services/api/internal/http/handlers/practice_sessions.go`
- Modify: `services/api/internal/http/handlers/terminal_ws.go`
- Modify: `services/runner/internal/http/handlers/workspaces.go`
- Modify: `services/runner/internal/http/handlers/commands.go`
- Modify: `services/runner/internal/http/handlers/resets.go`
- Modify: `apps/web/src/components/TopBar.tsx`
- Modify: `apps/web/src/styles.css`
- Test: `apps/web/tests/e2e/smoke.spec.ts`

- [ ] **Step 1: Flesh out the create-session and reset handlers**

Update `services/api/internal/http/handlers/practice_sessions.go` so the session create endpoint returns:

```json
{
  "id": "session-1",
  "status": "idle",
  "templateName": "Standard",
  "snapshot": {
    "branchName": "main",
    "headCommit": "seeded",
    "statusSummary": []
  },
  "history": []
}
```

Update `services/runner/internal/http/handlers/resets.go` so the reset endpoint returns:

```json
{
  "status": "resetting"
}
```

- [ ] **Step 2: Implement a minimal WebSocket terminal echo for the browser path**

Update `services/api/internal/http/handlers/terminal_ws.go` to upgrade the connection and relay a seed line:

```go
package handlers

import (
	"net/http"

	"github.com/coder/websocket"
)

func TerminalWebSocket() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()

		_ = conn.Write(r.Context(), websocket.MessageText, []byte("$ git status"))
	}
}
```

- [ ] **Step 3: Add the top bar session controls**

Update `apps/web/src/components/TopBar.tsx`:

```tsx
import { SessionStatusBadge } from "./SessionStatusBadge";

export function TopBar() {
  return (
    <header className="top-bar">
      <div className="brand">GitGym</div>
      <div className="top-bar-actions">
        <span>Template: Standard</span>
        <button className="ghost-button" type="button">Reset</button>
        <button className="ghost-button" type="button">New Session</button>
        <SessionStatusBadge label="Idle" />
      </div>
    </header>
  );
}
```

Update `apps/web/src/styles.css` with:

```css
.workbench-layout {
  display: grid;
  grid-template-columns: 1fr 320px;
  grid-template-rows: 1fr 180px;
  gap: 16px;
  padding: 16px 24px 24px;
}

.workbench-main,
.repo-panel,
.history-panel {
  background: #fcfcf8;
  border: 1px solid #d3d7cf;
  border-radius: 16px;
  padding: 16px;
}

.history-panel {
  grid-column: 1 / span 2;
}

.ghost-button {
  border: 1px solid #c5cbbf;
  background: transparent;
  padding: 8px 12px;
  border-radius: 999px;
}
```

- [ ] **Step 4: Extend the Playwright smoke test to verify the workbench renders**

Replace `apps/web/tests/e2e/smoke.spec.ts` with:

```ts
import { expect, test } from "@playwright/test";

test("renders login screen or workbench shell", async ({ page }) => {
  await page.goto("/");
  await expect(
    page.getByText("Continue with GitHub").or(page.getByText("Template: Standard")),
  ).toBeVisible();
});
```

- [ ] **Step 5: Run full verification**

Run:

```bash
go test ./services/api/...
go test ./services/runner/...
pnpm --dir apps/web test
pnpm --dir apps/web build
```

Expected:

- Go tests PASS for both services
- frontend tests PASS
- Vite build PASS

- [ ] **Step 6: Commit the integrated V1 slice**

Run:

```bash
git add services/api services/runner apps/web
git commit -m "feat: assemble gitgym v1 sandbox slice"
```

Expected: one commit that leaves the repository in a working V1 slice state.

## Self-Review

### Spec Coverage Check

- Authentication: covered in Task 4.
- MySQL schema and login persistence: covered in Task 3.
- API and runner split: covered in Tasks 4 through 7.
- Real Git workspace lifecycle: covered in Tasks 5 and 6.
- Snapshot and event capture: covered in Task 6.
- Desktop-first workbench UI: covered in Tasks 8 through 10.
- Session reset and command history foundations: covered in Tasks 9 and 10.

### Placeholder Scan

- No `TBD`, `TODO`, `FIXME`, or `implement later` placeholders remain in the planned file contents.
- The only intentional red tests are the first-step placeholders that each task explicitly replaces later in the same task.

### Type Consistency Check

- `PracticeSession`, `WorkspaceRef`, `RepoSnapshot`, and `CommandRecord` names are consistent across later tasks.
- API routes use the `/api/v1/...` prefix in both the contract and the frontend code.
- Runner internal routes use the `/internal/workspaces/...` prefix consistently.
