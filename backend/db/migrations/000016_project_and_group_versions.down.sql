-- Reverse 000016. Per-item version columns are recreated empty (set to 1 for
-- existing content rows) since the original per-template / per-env / per-name
-- version sequences cannot be reconstructed from the project-version snapshot.

ALTER TABLE global_values ADD COLUMN approval_condition TEXT NOT NULL DEFAULT '1 x gv_group_admin';
UPDATE global_values gv SET approval_condition = g.approval_condition
FROM global_values_groups g WHERE g.id = gv.group_id;

ALTER TABLE deployment_entries ADD COLUMN template_version_id INTEGER NOT NULL DEFAULT 1;
ALTER TABLE deployment_entries ADD COLUMN values_version_id   INTEGER NOT NULL DEFAULT 1;

ALTER TABLE pr_changes ADD COLUMN base_version_id INTEGER NOT NULL DEFAULT 1;

CREATE TABLE deployment_entry_global_refs (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    deployment_entry_id BIGINT NOT NULL REFERENCES deployment_entries(id),
    global_values_name TEXT NOT NULL,
    global_values_version_id INTEGER NOT NULL,
    UNIQUE (deployment_entry_id, global_values_name)
);

ALTER TABLE pull_requests       DROP COLUMN base_project_version_id;
ALTER TABLE pull_requests       DROP COLUMN base_group_version_id;
ALTER TABLE deployments         DROP COLUMN project_version_id;
ALTER TABLE global_values       DROP COLUMN group_id;

DROP TABLE deployment_group_refs;
DROP TABLE global_values_group_version_entries;
DROP TABLE global_values_group_versions;
DROP TABLE global_values_groups;
DROP TABLE project_version_values;
DROP TABLE project_version_templates;
DROP TABLE project_versions;
