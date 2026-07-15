CREATE TABLE IF NOT EXISTS console_roles (
    id VARCHAR(64) PRIMARY KEY,
    name VARCHAR(120) NOT NULL,
    code VARCHAR(120) NOT NULL UNIQUE,
    display_name VARCHAR(120) NOT NULL DEFAULT '',
    description VARCHAR(255) NOT NULL DEFAULT '',
    menu_keys JSONB NOT NULL DEFAULT '[]'::jsonb,
    is_super BOOLEAN NOT NULL DEFAULT FALSE,
    status INTEGER NOT NULL DEFAULT 1,
    sort INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
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
    real_name VARCHAR(120) NOT NULL DEFAULT '',
    display_name VARCHAR(120) NOT NULL DEFAULT '',
    avatar VARCHAR(255) NOT NULL DEFAULT '',
    email_verified BOOLEAN NOT NULL DEFAULT FALSE,
    phone_verified BOOLEAN NOT NULL DEFAULT FALSE,
    last_login_at TIMESTAMPTZ NULL,
    last_login_ip VARCHAR(64) NOT NULL DEFAULT '',
    login_count INTEGER NOT NULL DEFAULT 0,
    remark VARCHAR(500) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS console_casbin_rules (
    id BIGSERIAL PRIMARY KEY,
    ptype VARCHAR(100),
    v0 VARCHAR(100),
    v1 VARCHAR(100),
    v2 VARCHAR(100),
    v3 VARCHAR(100),
    v4 VARCHAR(100),
    v5 VARCHAR(100)
);

CREATE INDEX IF NOT EXISTS idx_console_casbin_rules_ptype ON console_casbin_rules (ptype);
CREATE INDEX IF NOT EXISTS idx_console_casbin_rules_v0 ON console_casbin_rules (v0);
CREATE INDEX IF NOT EXISTS idx_console_casbin_rules_v1 ON console_casbin_rules (v1);
CREATE UNIQUE INDEX IF NOT EXISTS idx_console_admins_email_non_empty ON console_admins (email) WHERE email <> '';
CREATE UNIQUE INDEX IF NOT EXISTS idx_console_admins_phone_non_empty ON console_admins (phone) WHERE phone <> '';
