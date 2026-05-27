-- Each staged change now records its operation. Existing rows were all
-- create-or-update style edits, so they default to 'update'.
-- IF NOT EXISTS makes this safe to re-run if the migration was interrupted.
ALTER TABLE pr_changes
    ADD COLUMN IF NOT EXISTS operation TEXT NOT NULL DEFAULT 'update'
        CHECK (operation IN ('create', 'update', 'delete'));
