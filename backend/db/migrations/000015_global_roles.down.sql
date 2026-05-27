-- Revert roles to a scoped namespace. The original binding (which project/GV a
-- role belonged to) cannot be recovered; columns are re-added empty.
ALTER TABLE roles DROP CONSTRAINT roles_name_key;
ALTER TABLE roles ADD COLUMN project_id BIGINT REFERENCES projects(id);
ALTER TABLE roles ADD COLUMN global_values_name TEXT;
ALTER TABLE roles ADD CONSTRAINT roles_name_scope_key
    UNIQUE (name, project_id, global_values_name);
