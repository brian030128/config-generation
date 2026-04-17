CREATE TABLE roles (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name TEXT NOT NULL,
    project_id BIGINT REFERENCES projects(id),
    is_auto_created BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (name, project_id)
);

CREATE TABLE role_permissions (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    role_id BIGINT NOT NULL REFERENCES roles(id),
    action TEXT NOT NULL,
    resource TEXT NOT NULL,
    key_project TEXT,
    key_env TEXT,
    key_name TEXT,
    UNIQUE (role_id, action, resource, key_project, key_env, key_name)
);

CREATE INDEX idx_role_permissions_role ON role_permissions (role_id);

CREATE TABLE user_roles (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    role_id BIGINT NOT NULL REFERENCES roles(id),
    granted_by BIGINT NOT NULL REFERENCES users(id),
    granted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, role_id)
);

CREATE INDEX idx_user_roles_user ON user_roles (user_id);
