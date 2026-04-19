CREATE TABLE project_config_templates (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES projects(id),
    template_name TEXT NOT NULL,
    version_id INTEGER NOT NULL,
    body TEXT NOT NULL,
    commit_message TEXT,
    created_by BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, template_name, version_id)
);

CREATE INDEX idx_templates_latest ON project_config_templates (project_id, template_name, version_id DESC);

CREATE TABLE project_config_values (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    project_id BIGINT NOT NULL REFERENCES projects(id),
    environment_id BIGINT NOT NULL REFERENCES environments(id),
    version_id INTEGER NOT NULL,
    payload JSONB NOT NULL,
    commit_message TEXT,
    created_by BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, environment_id, version_id)
);

CREATE INDEX idx_values_latest ON project_config_values (project_id, environment_id, version_id DESC);

CREATE TABLE global_values (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL,
    version_id INTEGER NOT NULL,
    payload JSONB NOT NULL,
    commit_message TEXT,
    created_by BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (name, version_id)
);

CREATE INDEX idx_global_values_latest ON global_values (name, version_id DESC);
