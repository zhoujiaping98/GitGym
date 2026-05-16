-- name: ListActiveTemplates :many
SELECT * FROM workspace_templates WHERE is_active = 1 ORDER BY id ASC;

-- name: GetScenarioByKey :one
SELECT * FROM scenarios WHERE scenario_key = ? AND is_active = 1 LIMIT 1;
