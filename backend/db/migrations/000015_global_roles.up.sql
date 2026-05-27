-- Roles become a single global namespace: a role is identified by a globally
-- unique name, and its scope comes only from its permission atoms
-- (key_project/key_env/key_name). Carry the old scope into the name for
-- auto-created roles, then drop the binding columns.

-- Guard: only run the name-backfill if the scope columns still exist.
-- This makes the migration safe to re-run after an interrupted attempt.
DO $$ BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'roles' AND column_name = 'project_id'
    ) THEN
        UPDATE roles SET name = p.name || '_project_admin'
        FROM projects p
        WHERE roles.project_id = p.id AND roles.is_auto_created;

        UPDATE roles SET name = roles.global_values_name || '_gv_group_admin'
        WHERE roles.global_values_name IS NOT NULL AND roles.is_auto_created;
    END IF;
END $$;

ALTER TABLE roles DROP CONSTRAINT IF EXISTS roles_name_scope_key;
ALTER TABLE roles DROP COLUMN IF EXISTS project_id;
ALTER TABLE roles DROP COLUMN IF EXISTS global_values_name;
DO $$ BEGIN
    ALTER TABLE roles ADD CONSTRAINT roles_name_key UNIQUE (name);
EXCEPTION WHEN duplicate_object THEN
    NULL; -- constraint already exists, skip
END $$;
