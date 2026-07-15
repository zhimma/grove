CREATE TABLE IF NOT EXISTS console_roles (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(120) NOT NULL,
    code VARCHAR(120) NOT NULL UNIQUE,
    description VARCHAR(255) NOT NULL DEFAULT '',
    menu_keys JSON NOT NULL,
    is_super BOOLEAN NOT NULL DEFAULT FALSE,
    status INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    deleted_at DATETIME(6) NULL
);

CREATE TABLE IF NOT EXISTS console_admins (
    id VARCHAR(64) PRIMARY KEY,
    account VARCHAR(120) NOT NULL UNIQUE,
    username VARCHAR(120) NOT NULL DEFAULT '',
    email VARCHAR(160) NOT NULL DEFAULT '',
    password VARCHAR(255) NOT NULL,
    role_id VARCHAR(64) NOT NULL,
    status INTEGER NOT NULL DEFAULT 1,
    phone VARCHAR(32) NOT NULL DEFAULT '',
    last_login_at DATETIME(6) NULL,
    created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    deleted_at DATETIME(6) NULL
);

CREATE TABLE IF NOT EXISTS console_casbin_rules (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    ptype VARCHAR(100),
    v0 VARCHAR(100),
    v1 VARCHAR(100),
    v2 VARCHAR(100),
    v3 VARCHAR(100),
    v4 VARCHAR(100),
    v5 VARCHAR(100)
);

CREATE INDEX idx_console_casbin_rules_ptype ON console_casbin_rules (ptype);
CREATE INDEX idx_console_casbin_rules_v0 ON console_casbin_rules (v0);
CREATE INDEX idx_console_casbin_rules_v1 ON console_casbin_rules (v1);
