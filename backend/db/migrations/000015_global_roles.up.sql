-- Roles become a single global namespace: a role is identified by a globally
-- unique name, and its scope comes only from its permission atoms
-- (key_project/key_env/key_name). Carry the old scope into the name for
-- auto-created roles, then drop the binding columns.

UPDATE roles SET name = p.name || '_project_admin'
FROM projects p
WHERE roles.project_id = p.id AND roles.is_auto_created;

UPDATE roles SET name = roles.global_values_name || '_gv_group_admin'
WHERE roles.global_values_name IS NOT NULL AND roles.is_auto_created;

ALTER TABLE roles DROP CONSTRAINT roles_name_scope_key;
ALTER TABLE roles DROP COLUMN project_id;
ALTER TABLE roles DROP COLUMN global_values_name;
ALTER TABLE roles ADD CONSTRAINT roles_name_key UNIQUE (name);
