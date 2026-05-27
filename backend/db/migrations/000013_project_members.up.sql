-- Project membership. Being a member of a project grants read:project(name)
-- (synthesized in the permission loader) — the holder can read the project's
-- metadata and see it in their project list, and nothing more. Membership does
-- not unlock templates, environments, or env-values reads.
CREATE TABLE IF NOT EXISTS project_members (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES projects(id),
    user_id    BIGINT NOT NULL REFERENCES users(id),
    added_by   BIGINT NOT NULL REFERENCES users(id),
    added_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_project_members_user ON project_members (user_id);
CREATE INDEX IF NOT EXISTS idx_project_members_project ON project_members (project_id);
