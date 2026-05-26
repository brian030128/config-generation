-- Each staged change now records its operation. Existing rows were all
-- create-or-update style edits, so they default to 'update'.
ALTER TABLE pr_changes
    ADD COLUMN operation TEXT NOT NULL DEFAULT 'update'
        CHECK (operation IN ('create', 'update', 'delete'));
