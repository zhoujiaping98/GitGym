CREATE TABLE workspace_cleanup_jobs (
  id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  practice_session_id BIGINT UNSIGNED NOT NULL,
  workspace_id VARCHAR(255) NOT NULL,
  reason VARCHAR(32) NOT NULL,
  scheduled_at DATETIME(6) NOT NULL,
  status VARCHAR(32) NOT NULL,
  attempt_count INT UNSIGNED NOT NULL DEFAULT 0,
  last_error TEXT NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uq_workspace_cleanup_jobs_session (practice_session_id),
  KEY idx_workspace_cleanup_jobs_status_schedule (status, scheduled_at),
  CONSTRAINT fk_workspace_cleanup_jobs_session
    FOREIGN KEY (practice_session_id) REFERENCES practice_sessions(id)
    ON DELETE CASCADE
);
