ALTER TABLE roles DROP CONSTRAINT roles_name_scope_key;
ALTER TABLE roles ADD CONSTRAINT roles_name_project_id_key UNIQUE (name, project_id);
ALTER TABLE roles DROP COLUMN global_values_name;

DROP INDEX IF EXISTS idx_pull_requests_gv_name_status;
ALTER TABLE pull_requests DROP CONSTRAINT chk_pr_scope;
ALTER TABLE pull_requests DROP COLUMN global_values_name;

ALTER TABLE global_values DROP COLUMN approval_condition;
