CREATE TABLE deployments (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES projects(id),
    environment_id BIGINT NOT NULL REFERENCES environments(id),
    status TEXT NOT NULL CHECK (status IN ('pending', 'succeeded', 'failed', 'rolled_back')),
    rolled_back_from BIGINT REFERENCES deployments(id),
    commit_message TEXT,
    created_by BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_deployments_lookup ON deployments (project_id, environment_id, status, created_at DESC);

CREATE TABLE deployment_entries (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    deployment_id BIGINT NOT NULL REFERENCES deployments(id),
    template_name TEXT NOT NULL,
    template_version_id INTEGER NOT NULL,
    values_version_id INTEGER NOT NULL,
    rendered_output TEXT NOT NULL,
    UNIQUE (deployment_id, template_name)
);

CREATE TABLE deployment_entry_global_refs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    deployment_entry_id BIGINT NOT NULL REFERENCES deployment_entries(id),
    global_values_name TEXT NOT NULL,
    global_values_version_id INTEGER NOT NULL,
    UNIQUE (deployment_entry_id, global_values_name)
);
