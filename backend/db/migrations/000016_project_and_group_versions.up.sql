-- Project-level version snapshots and global-values groups.
-- Replaces per-template / per-(project,env) / per-name version sequences with
-- one version sequence per project and one per global-values group. Each
-- snapshot points at the immutable content rows that live in the existing
-- project_config_templates / project_config_values / global_values tables.

CREATE TABLE project_versions (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    project_id        BIGINT NOT NULL REFERENCES projects(id),
    version_id        INTEGER NOT NULL,
    parent_version_id BIGINT REFERENCES project_versions(id),
    commit_message    TEXT,
    is_anchor         BOOLEAN NOT NULL DEFAULT false,
    created_by        BIGINT NOT NULL REFERENCES users(id),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, version_id)
);
CREATE INDEX idx_project_versions_latest ON project_versions (project_id, version_id DESC);

CREATE TABLE project_version_templates (
    project_version_id BIGINT NOT NULL REFERENCES project_versions(id) ON DELETE CASCADE,
    template_name      TEXT   NOT NULL,
    template_row_id    BIGINT NOT NULL REFERENCES project_config_templates(id),
    PRIMARY KEY (project_version_id, template_name)
);

CREATE TABLE project_version_values (
    project_version_id BIGINT NOT NULL REFERENCES project_versions(id) ON DELETE CASCADE,
    environment_id     BIGINT NOT NULL REFERENCES environments(id),
    values_row_id      BIGINT NOT NULL REFERENCES project_config_values(id),
    PRIMARY KEY (project_version_id, environment_id)
);

CREATE TABLE global_values_groups (
    id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name               TEXT NOT NULL UNIQUE,
    approval_condition TEXT NOT NULL,
    created_by         BIGINT NOT NULL REFERENCES users(id),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE global_values_group_versions (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    group_id          BIGINT NOT NULL REFERENCES global_values_groups(id),
    version_id        INTEGER NOT NULL,
    parent_version_id BIGINT REFERENCES global_values_group_versions(id),
    commit_message    TEXT,
    created_by        BIGINT NOT NULL REFERENCES users(id),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (group_id, version_id)
);
CREATE INDEX idx_gvg_versions_latest ON global_values_group_versions (group_id, version_id DESC);

CREATE TABLE global_values_group_version_entries (
    group_version_id BIGINT NOT NULL REFERENCES global_values_group_versions(id) ON DELETE CASCADE,
    gv_name          TEXT   NOT NULL,
    gv_row_id        BIGINT NOT NULL REFERENCES global_values(id),
    PRIMARY KEY (group_version_id, gv_name)
);

ALTER TABLE global_values ADD COLUMN group_id BIGINT REFERENCES global_values_groups(id);

-- Pin a single project_version on each deployment instead of per-template /
-- per-values version maps spread across deployment_entries.
ALTER TABLE deployments ADD COLUMN project_version_id BIGINT REFERENCES project_versions(id);

CREATE TABLE deployment_group_refs (
    id                  BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    deployment_id       BIGINT NOT NULL REFERENCES deployments(id),
    group_id            BIGINT NOT NULL REFERENCES global_values_groups(id),
    group_version_id    BIGINT NOT NULL REFERENCES global_values_group_versions(id),
    UNIQUE (deployment_id, group_id)
);

-- PR base now pins a single project_version (or group_version), not a map.
ALTER TABLE pull_requests ADD COLUMN base_project_version_id BIGINT REFERENCES project_versions(id);
ALTER TABLE pull_requests ADD COLUMN base_group_version_id   BIGINT REFERENCES global_values_group_versions(id);

-- Each global_values name becomes a singleton group carrying its v1 approval
-- condition. group_id is backfilled on every existing global_values row.
INSERT INTO global_values_groups (name, approval_condition, created_by, created_at)
SELECT gv.name, gv.approval_condition, gv.created_by, gv.created_at
FROM global_values gv
WHERE gv.version_id = 1;

UPDATE global_values gv
SET group_id = g.id
FROM global_values_groups g
WHERE gv.name = g.name;

-- One group_version per existing (name, version_id), preserving the ordinal.
INSERT INTO global_values_group_versions (group_id, version_id, commit_message, created_by, created_at)
SELECT g.id, gv.version_id, gv.commit_message, gv.created_by, gv.created_at
FROM global_values gv
JOIN global_values_groups g ON g.name = gv.name;

INSERT INTO global_values_group_version_entries (group_version_id, gv_name, gv_row_id)
SELECT ggv.id, gv.name, gv.id
FROM global_values gv
JOIN global_values_groups g ON g.name = gv.name
JOIN global_values_group_versions ggv ON ggv.group_id = g.id AND ggv.version_id = gv.version_id;

-- Build project_versions by walking each project's template+values events in
-- temporal order. Each event emits one project_version, snapshotting the full
-- (name -> content_row) and (env -> content_row) maps cumulatively.
DO $$
DECLARE
    proj RECORD;
    evt RECORD;
    new_pv_id BIGINT;
    last_pv_id BIGINT;
    next_ord INT;
BEGIN
    FOR proj IN SELECT id FROM projects ORDER BY id LOOP
        last_pv_id := NULL;
        next_ord := 0;
        FOR evt IN
            SELECT 'template'::text AS kind, id AS row_id, template_name,
                   NULL::BIGINT AS environment_id, commit_message, created_by, created_at
            FROM project_config_templates WHERE project_id = proj.id
            UNION ALL
            SELECT 'values'::text, id, NULL::text,
                   environment_id, commit_message, created_by, created_at
            FROM project_config_values WHERE project_id = proj.id
            ORDER BY created_at ASC, row_id ASC
        LOOP
            next_ord := next_ord + 1;
            INSERT INTO project_versions (project_id, version_id, parent_version_id, commit_message, created_by, created_at)
            VALUES (proj.id, next_ord, last_pv_id, evt.commit_message, evt.created_by, evt.created_at)
            RETURNING id INTO new_pv_id;

            IF last_pv_id IS NOT NULL THEN
                INSERT INTO project_version_templates (project_version_id, template_name, template_row_id)
                SELECT new_pv_id, template_name, template_row_id
                FROM project_version_templates WHERE project_version_id = last_pv_id;

                INSERT INTO project_version_values (project_version_id, environment_id, values_row_id)
                SELECT new_pv_id, environment_id, values_row_id
                FROM project_version_values WHERE project_version_id = last_pv_id;
            END IF;

            IF evt.kind = 'template' THEN
                INSERT INTO project_version_templates (project_version_id, template_name, template_row_id)
                VALUES (new_pv_id, evt.template_name, evt.row_id)
                ON CONFLICT (project_version_id, template_name)
                DO UPDATE SET template_row_id = EXCLUDED.template_row_id;
            ELSE
                INSERT INTO project_version_values (project_version_id, environment_id, values_row_id)
                VALUES (new_pv_id, evt.environment_id, evt.row_id)
                ON CONFLICT (project_version_id, environment_id)
                DO UPDATE SET values_row_id = EXCLUDED.values_row_id;
            END IF;

            last_pv_id := new_pv_id;
        END LOOP;
    END LOOP;
END $$;

-- Backfill deployments.project_version_id. For each deployment, try to find a
-- project_version whose snapshot matches every pinned (template_name, version)
-- and (environment_id, values_version) in the deployment's entries. If none
-- matches exactly (e.g. cross-version pin sets that never co-existed in the
-- walk), synthesize an anchor project_version and snapshot the pinned rows.
DO $$
DECLARE
    d RECORD;
    matched_pv BIGINT;
    anchor_id BIGINT;
    next_ord INT;
BEGIN
    FOR d IN SELECT id, project_id, environment_id, commit_message, created_by, created_at FROM deployments LOOP
        SELECT pv.id INTO matched_pv
        FROM project_versions pv
        WHERE pv.project_id = d.project_id
          AND NOT EXISTS (
              SELECT 1
              FROM deployment_entries de
              JOIN project_config_templates t ON t.project_id = d.project_id
                                              AND t.template_name = de.template_name
                                              AND t.version_id = de.template_version_id
              LEFT JOIN project_version_templates pvt ON pvt.project_version_id = pv.id AND pvt.template_name = de.template_name
              WHERE de.deployment_id = d.id
                AND (pvt.template_row_id IS NULL OR pvt.template_row_id <> t.id)
          )
          AND NOT EXISTS (
              SELECT 1
              FROM deployment_entries de
              JOIN project_config_values v ON v.project_id = d.project_id
                                           AND v.environment_id = d.environment_id
                                           AND v.version_id = de.values_version_id
              LEFT JOIN project_version_values pvv ON pvv.project_version_id = pv.id AND pvv.environment_id = d.environment_id
              WHERE de.deployment_id = d.id
                AND (pvv.values_row_id IS NULL OR pvv.values_row_id <> v.id)
          )
        ORDER BY pv.version_id ASC
        LIMIT 1;

        IF matched_pv IS NULL THEN
            SELECT COALESCE(MAX(version_id), 0) + 1 INTO next_ord FROM project_versions WHERE project_id = d.project_id;
            INSERT INTO project_versions (project_id, version_id, commit_message, is_anchor, created_by, created_at)
            VALUES (d.project_id, next_ord, 'Migration anchor for deployment ' || d.id, true, d.created_by, d.created_at)
            RETURNING id INTO anchor_id;

            INSERT INTO project_version_templates (project_version_id, template_name, template_row_id)
            SELECT anchor_id, de.template_name, t.id
            FROM deployment_entries de
            JOIN project_config_templates t ON t.project_id = d.project_id
                                            AND t.template_name = de.template_name
                                            AND t.version_id = de.template_version_id
            WHERE de.deployment_id = d.id;

            INSERT INTO project_version_values (project_version_id, environment_id, values_row_id)
            SELECT DISTINCT anchor_id, d.environment_id, v.id
            FROM deployment_entries de
            JOIN project_config_values v ON v.project_id = d.project_id
                                         AND v.environment_id = d.environment_id
                                         AND v.version_id = de.values_version_id
            WHERE de.deployment_id = d.id;

            matched_pv := anchor_id;
        END IF;

        UPDATE deployments SET project_version_id = matched_pv WHERE id = d.id;
    END LOOP;
END $$;

-- Backfill deployment_group_refs from the old deployment_entry_global_refs.
INSERT INTO deployment_group_refs (deployment_id, group_id, group_version_id)
SELECT DISTINCT de.deployment_id, g.id, ggv.id
FROM deployment_entry_global_refs degr
JOIN deployment_entries de ON de.id = degr.deployment_entry_id
JOIN global_values_groups g ON g.name = degr.global_values_name
JOIN global_values_group_versions ggv ON ggv.group_id = g.id AND ggv.version_id = degr.global_values_version_id;

-- Backfill PR base_project_version_id / base_group_version_id.
-- For project PRs: latest project_version for that project as of PR created_at.
-- For GV PRs: latest group_version for the named group as of PR created_at.
UPDATE pull_requests pr
SET base_project_version_id = (
    SELECT pv.id FROM project_versions pv
    WHERE pv.project_id = pr.project_id AND pv.created_at <= pr.created_at
    ORDER BY pv.version_id DESC LIMIT 1
)
WHERE pr.project_id IS NOT NULL;

UPDATE pull_requests pr
SET base_group_version_id = (
    SELECT ggv.id FROM global_values_group_versions ggv
    JOIN global_values_groups g ON g.id = ggv.group_id
    WHERE g.name = pr.global_values_name AND ggv.created_at <= pr.created_at
    ORDER BY ggv.version_id DESC LIMIT 1
)
WHERE pr.global_values_name IS NOT NULL;

-- Constraints + cleanup of obsolete columns.
ALTER TABLE deployments      ALTER COLUMN project_version_id SET NOT NULL;
ALTER TABLE global_values    ALTER COLUMN group_id           SET NOT NULL;

ALTER TABLE pr_changes         DROP COLUMN base_version_id;
ALTER TABLE deployment_entries DROP COLUMN template_version_id;
ALTER TABLE deployment_entries DROP COLUMN values_version_id;
ALTER TABLE global_values      DROP COLUMN approval_condition;
-- global_values.version_id is no longer authoritative; the group's version
-- sequence lives on global_values_group_versions. Drop the column and the
-- (name, version_id) uniqueness so new content rows just need a unique id.
ALTER TABLE global_values      DROP CONSTRAINT global_values_name_version_id_key;
DROP INDEX IF EXISTS idx_global_values_latest;
ALTER TABLE global_values      DROP COLUMN version_id;
DROP TABLE deployment_entry_global_refs;
