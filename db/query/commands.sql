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
