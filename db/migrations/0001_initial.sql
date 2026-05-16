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
