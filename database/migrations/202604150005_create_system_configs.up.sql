CREATE TABLE IF NOT EXISTS system_configs (
    id VARCHAR(26) PRIMARY KEY,
    config_group VARCHAR(64) NOT NULL,
    config_key VARCHAR(120) NOT NULL,
    name VARCHAR(120) NOT NULL,
    description VARCHAR(255) NOT NULL DEFAULT '',
    value_type VARCHAR(20) NOT NULL DEFAULT 'string',
    value TEXT NOT NULL DEFAULT '',
    default_value TEXT NOT NULL DEFAULT '',
    is_editable BOOLEAN NOT NULL DEFAULT TRUE,
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_system_configs_group_key
    ON system_configs (config_group, config_key);

CREATE INDEX IF NOT EXISTS idx_system_configs_deleted_at
    ON system_configs (deleted_at);
