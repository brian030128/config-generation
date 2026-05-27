-- Environment administrators. Being an env-admin of an environment synthesizes
-- (in the permission loader) read:project(p), create:env_values(p, env) and
-- delete:project_values(p, env) for that environment — full control of the
-- environment's value sets plus the ability to delete the environment. An
-- env-admin can also grant env-admin to other users (self-propagating). The
-- environment's creator and the project's admins are env-admins implicitly.
CREATE TABLE env_admins (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    environment_id BIGINT NOT NULL REFERENCES environments(id),
    user_id        BIGINT NOT NULL REFERENCES users(id),
    granted_by     BIGINT NOT NULL REFERENCES users(id),
    granted_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (environment_id, user_id)
);

CREATE INDEX idx_env_admins_user ON env_admins (user_id);
CREATE INDEX idx_env_admins_env  ON env_admins (environment_id);
