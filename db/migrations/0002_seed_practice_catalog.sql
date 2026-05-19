INSERT INTO workspace_templates (
  id,
  template_key,
  name,
  description,
  mode_compatibility,
  source_ref,
  difficulty_level,
  is_active
)
VALUES (
  1,
  'standard',
  'Standard',
  'Default disposable sandbox repository template for Git practice sessions.',
  JSON_ARRAY('sandbox'),
  'builtin:standard',
  'beginner',
  1
)
ON DUPLICATE KEY UPDATE
  template_key = VALUES(template_key),
  name = VALUES(name),
  description = VALUES(description),
  mode_compatibility = VALUES(mode_compatibility),
  source_ref = VALUES(source_ref),
  difficulty_level = VALUES(difficulty_level),
  is_active = VALUES(is_active);

INSERT INTO scenarios (
  id,
  scenario_key,
  mode_type,
  template_id,
  name,
  description,
  rules_json,
  completion_rules_json,
  hint_policy_json,
  is_active
)
VALUES (
  1,
  'sandbox-standard',
  'sandbox',
  1,
  'Standard Sandbox',
  'Default free-practice Git sandbox backed by the standard repository template.',
  JSON_OBJECT(),
  NULL,
  JSON_OBJECT('mode', 'minimal'),
  1
)
ON DUPLICATE KEY UPDATE
  scenario_key = VALUES(scenario_key),
  mode_type = VALUES(mode_type),
  template_id = VALUES(template_id),
  name = VALUES(name),
  description = VALUES(description),
  rules_json = VALUES(rules_json),
  completion_rules_json = VALUES(completion_rules_json),
  hint_policy_json = VALUES(hint_policy_json),
  is_active = VALUES(is_active);
