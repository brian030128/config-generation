ALTER TABLE project_config_values ADD COLUMN secret_keys JSONB NOT NULL DEFAULT '[]';
ALTER TABLE global_values ADD COLUMN secret_keys JSONB NOT NULL DEFAULT '[]';
