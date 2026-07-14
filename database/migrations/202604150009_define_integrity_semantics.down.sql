DROP INDEX IF EXISTS idx_console_casbin_rules_unique;
DROP INDEX IF EXISTS idx_casbin_rules_unique;

ALTER TABLE console_casbin_rules
    ALTER COLUMN ptype DROP NOT NULL,
    ALTER COLUMN ptype DROP DEFAULT,
    ALTER COLUMN v0 DROP NOT NULL,
    ALTER COLUMN v0 DROP DEFAULT,
    ALTER COLUMN v1 DROP NOT NULL,
    ALTER COLUMN v1 DROP DEFAULT,
    ALTER COLUMN v2 DROP NOT NULL,
    ALTER COLUMN v2 DROP DEFAULT,
    ALTER COLUMN v3 DROP NOT NULL,
    ALTER COLUMN v3 DROP DEFAULT,
    ALTER COLUMN v4 DROP NOT NULL,
    ALTER COLUMN v4 DROP DEFAULT,
    ALTER COLUMN v5 DROP NOT NULL,
    ALTER COLUMN v5 DROP DEFAULT;

ALTER TABLE casbin_rules
    ALTER COLUMN ptype DROP NOT NULL,
    ALTER COLUMN ptype DROP DEFAULT,
    ALTER COLUMN v0 DROP NOT NULL,
    ALTER COLUMN v0 DROP DEFAULT,
    ALTER COLUMN v1 DROP NOT NULL,
    ALTER COLUMN v1 DROP DEFAULT,
    ALTER COLUMN v2 DROP NOT NULL,
    ALTER COLUMN v2 DROP DEFAULT,
    ALTER COLUMN v3 DROP NOT NULL,
    ALTER COLUMN v3 DROP DEFAULT,
    ALTER COLUMN v4 DROP NOT NULL,
    ALTER COLUMN v4 DROP DEFAULT,
    ALTER COLUMN v5 DROP NOT NULL,
    ALTER COLUMN v5 DROP DEFAULT;

ALTER TABLE system_configs
    DROP CONSTRAINT IF EXISTS chk_system_configs_value_type;

ALTER TABLE console_roles
    DROP CONSTRAINT IF EXISTS chk_console_roles_status;

ALTER TABLE console_admins
    DROP CONSTRAINT IF EXISTS chk_console_admins_status,
    DROP CONSTRAINT IF EXISTS fk_console_admins_role;

DROP INDEX IF EXISTS idx_users_email_active;
DROP INDEX IF EXISTS idx_console_roles_code_active;
DROP INDEX IF EXISTS idx_console_admins_account_active;
DROP INDEX IF EXISTS idx_console_admins_email_active;
DROP INDEX IF EXISTS idx_console_admins_phone_active;
DROP INDEX IF EXISTS idx_system_configs_group_key;

ALTER TABLE users
    ADD CONSTRAINT users_email_key UNIQUE (email);

ALTER TABLE console_roles
    ADD CONSTRAINT console_roles_code_key UNIQUE (code);

ALTER TABLE console_admins
    ADD CONSTRAINT console_admins_account_key UNIQUE (account);

CREATE UNIQUE INDEX idx_console_admins_email_non_empty
    ON console_admins (email)
    WHERE email <> '';

CREATE UNIQUE INDEX idx_console_admins_phone_non_empty
    ON console_admins (phone)
    WHERE phone <> '';

CREATE UNIQUE INDEX idx_system_configs_group_key
    ON system_configs (config_group, config_key);

ALTER TABLE console_operation_logs
    ADD COLUMN deleted_at TIMESTAMPTZ NULL;

ALTER TABLE console_login_logs
    ADD COLUMN deleted_at TIMESTAMPTZ NULL;
