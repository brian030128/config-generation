CREATE TABLE pull_requests (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES projects(id),
    author_id BIGINT NOT NULL REFERENCES users(id),
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL CHECK (status IN ('draft', 'open', 'approved', 'merged', 'closed')),
    is_conflicted BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    merged_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ
);

CREATE INDEX idx_pull_requests_project_status ON pull_requests (project_id, status);

CREATE TABLE pr_changes (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    pr_id BIGINT NOT NULL REFERENCES pull_requests(id),
    object_type TEXT NOT NULL CHECK (object_type IN ('template', 'values', 'global_values', 'environment')),
    project_id BIGINT REFERENCES projects(id),
    template_name TEXT,
    environment_name TEXT,
    global_values_name TEXT,
    base_version_id INTEGER NOT NULL,
    proposed_payload TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE pr_approvals (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    pr_id BIGINT NOT NULL REFERENCES pull_requests(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    approved_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    withdrawn_at TIMESTAMPTZ,
    UNIQUE (pr_id, user_id)
);
