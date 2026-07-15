CREATE TABLE IF NOT EXISTS system_configs (
    id VARCHAR(26) PRIMARY KEY,
    config_group VARCHAR(64) NOT NULL,
    config_key VARCHAR(120) NOT NULL,
    name VARCHAR(120) NOT NULL,
    description VARCHAR(255) NOT NULL DEFAULT '',
    value_type VARCHAR(20) NOT NULL DEFAULT 'string',
    value TEXT NOT NULL,
    default_value TEXT NOT NULL,
    is_editable BOOLEAN NOT NULL DEFAULT TRUE,
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    deleted_at DATETIME(6) NULL,
    UNIQUE KEY idx_system_configs_group_key (config_group, config_key)
);

CREATE INDEX idx_system_configs_deleted_at ON system_configs (deleted_at);
