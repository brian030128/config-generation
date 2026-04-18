-- Add governance columns to global_values (tracked on every version row).
-- Every entry gets an approval_condition and a created_by already exists.
ALTER TABLE global_values ADD COLUMN approval_condition TEXT NOT NULL DEFAULT '1 x gv_group_admin';

-- Add global_values_name to pull_requests so a GV PR is scoped to one entry.
ALTER TABLE pull_requests ADD COLUMN global_values_name TEXT;

-- Backfill existing global values PRs: set global_values_name from pr_changes.
UPDATE pull_requests p
SET global_values_name = (
    SELECT c.global_values_name
    FROM pr_changes c
    WHERE c.pr_id = p.id AND c.object_type = 'global_values'
    LIMIT 1
)
WHERE p.project_id IS NULL;

-- Exactly one of project_id or global_values_name must be set.
ALTER TABLE pull_requests ADD CONSTRAINT chk_pr_scope
    CHECK (
        (project_id IS NOT NULL AND global_values_name IS NULL) OR
        (project_id IS NULL AND global_values_name IS NOT NULL)
    );

CREATE INDEX idx_pull_requests_gv_name_status ON pull_requests (global_values_name, status)
    WHERE global_values_name IS NOT NULL;

-- Allow roles to be scoped to a global values entry.
ALTER TABLE roles ADD COLUMN global_values_name TEXT;

-- Drop the old unique constraint and recreate to include global_values_name.
ALTER TABLE roles DROP CONSTRAINT roles_name_project_id_key;
ALTER TABLE roles ADD CONSTRAINT roles_name_scope_key
    UNIQUE (name, project_id, global_values_name);
